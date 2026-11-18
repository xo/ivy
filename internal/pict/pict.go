// Package pict reads the pictures of QuickDraw, PICT version 1 and
// version 2, as Inside Macintosh: Imaging With QuickDraw, Appendix A, gives
// them. The store of pictures of Office Drawing holds such pictures, from
// Office for the Macintosh.
//
// The package draws the bitmaps of a picture and the images of QuickTime in
// it, such as JPEG, on a transparent image of the frame of the picture. It
// reads the other opcodes only to skip them: it does not draw the lines,
// the shapes, the regions or the text.
package pict

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
)

// be is the byte order of QuickDraw.
var be = binary.BigEndian

// headerSize is the size of the header of a PICT file, which a picture of
// a resource leaves out.
const headerSize = 512

// maxScale bounds the number of pixels of a bitmap for each point of the
// frame of the picture.
const maxScale = 16

// Decode returns the image of the picture in data. data can start with the
// header of 512 bytes of a PICT file. The image has the size of the frame
// of the picture, times the resolution of its finest bitmap. It returns an
// error that wraps ivy.ErrFormat for data that is not a picture or holds
// no bitmap and no image, and one that wraps ivy.ErrLimit for a picture
// past the limits.
func Decode(data []byte, lim ivy.Limits) (image.Image, error) {
	lim = lim.WithDefaults()
	img, err := decode(data, lim)
	if err != nil && len(data) > headerSize+10 {
		if img2, err2 := decode(data[headerSize:], lim); err2 == nil {
			return img2, nil
		}
	}
	return img, err
}

// rect is a rectangle of QuickDraw: top, left, bottom and right.
type rect struct {
	top, left, bottom, right int
}

// empty reports whether r has no area.
func (r rect) empty() bool {
	return r.bottom <= r.top || r.right <= r.left
}

// dx and dy return the size of r.
func (r rect) dx() int { return r.right - r.left }
func (r rect) dy() int { return r.bottom - r.top }

// bitmap is a bitmap of the picture, or an image of QuickTime, with the
// rectangle of its pixels that it draws and the rectangle of the frame
// that it draws them in.
type bitmap struct {
	img      image.Image
	src, dst rect
}

// reader reads the opcodes of a picture.
type reader struct {
	b   []byte
	pos int
	v2  bool
	lim ivy.Limits
	// count counts the pixels of the bitmaps that the picture holds.
	count   int64
	bitmaps []bitmap
}

// errShort is the error of a picture that ends in an opcode.
var errShort = errors.New("pict: picture cut short")

// decode decodes the picture at the start of data.
func decode(data []byte, lim ivy.Limits) (image.Image, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("pict of %d bytes: %w", len(data), ivy.ErrFormat)
	}
	r := &reader{b: data, pos: 2, lim: lim}
	frame := r.rect()
	if frame.empty() {
		return nil, fmt.Errorf("pict with an empty frame: %w", ivy.ErrFormat)
	}
	// the version opcode: 0x11 0x01 for version 1, and 0x0011 0x02FF for
	// version 2
	switch {
	case data[10] == 0x11 && data[11] == 0x01:
		r.pos = 12
	case len(data) >= 14 && be.Uint16(data[10:]) == 0x0011 && be.Uint16(data[12:]) == 0x02FF:
		r.pos, r.v2 = 14, true
	default:
		return nil, fmt.Errorf("pict without a version opcode: %w", ivy.ErrFormat)
	}
	if err := r.run(&frame); err != nil {
		return nil, err
	}
	if len(r.bitmaps) == 0 {
		return nil, fmt.Errorf("pict without a bitmap: %w", ivy.ErrFormat)
	}
	return r.compose(frame)
}

// run reads the opcodes up to the end of the picture. An extended header
// of version 2 sets the frame to its own rectangle, whose coordinates the
// opcodes use.
func (r *reader) run(frame *rect) error {
	for n := 0; ; n++ {
		if n >= r.lim.MaxObjects {
			return fmt.Errorf("pict of more than %d opcodes: %w", r.lim.MaxObjects, ivy.ErrLimit)
		}
		if r.v2 && r.pos%2 == 1 {
			r.pos++
		}
		var op int
		if r.v2 {
			op = r.u16()
		} else {
			op = r.u8()
		}
		if r.pos > len(r.b) {
			// a picture that ends without OpEndPic keeps its bitmaps
			return nil
		}
		if op == 0x00FF {
			return nil
		}
		if op == 0x0C00 && r.pos+24 <= len(r.b) {
			// HeaderOp: an extended header, version -2, gives the
			// rectangle of the coordinates at its resolution
			h := r.b[r.pos:]
			if int16(be.Uint16(h)) == -2 {
				src := rect{int(int16(be.Uint16(h[12:]))), int(int16(be.Uint16(h[14:]))), int(int16(be.Uint16(h[16:]))), int(int16(be.Uint16(h[18:])))}
				if !src.empty() {
					*frame = src
				}
			}
		}
		if err := r.op(op); err != nil {
			if errors.Is(err, errShort) {
				return nil
			}
			return err
		}
	}
}

// op reads the data of the opcode op.
func (r *reader) op(op int) error {
	switch op {
	case 0x0090, 0x0091, 0x0098, 0x0099:
		return r.bits(op&1 == 1, op >= 0x0098)
	case 0x009A, 0x009B:
		return r.directBits(op == 0x009B)
	case 0x8200, 0x8201:
		n := r.u32()
		if n < 0 || r.pos+n > len(r.b) {
			return errShort
		}
		data := r.b[r.pos : r.pos+n]
		r.pos += n
		return r.quickTime(data)
	}
	n, ok := r.size(op)
	if !ok {
		return fmt.Errorf("pict opcode %#04x: %w", op, ivy.ErrFormat)
	}
	if n < 0 || r.pos+n > len(r.b) {
		return errShort
	}
	r.pos += n
	return nil
}

// size returns the size of the data of an opcode that the reader skips,
// after the bytes of a size that it reads.
func (r *reader) size(op int) (int, bool) {
	switch {
	case op == 0x0000, op >= 0x0017 && op <= 0x0019, op == 0x001C, op == 0x001E,
		op >= 0x0038 && op <= 0x003F, op >= 0x0048 && op <= 0x004F, op >= 0x0058 && op <= 0x005F,
		op >= 0x0078 && op <= 0x007F, op >= 0x0088 && op <= 0x008F, op >= 0x00B0 && op <= 0x00CF,
		op >= 0x8000 && op <= 0x80FF:
		return 0, true
	case op == 0x0001, op >= 0x0070 && op <= 0x0077, op >= 0x0080 && op <= 0x0087:
		// a region or a polygon: its size counts its own 2 bytes
		return r.u16() - 2, true
	case op == 0x0003, op == 0x0005, op == 0x0008, op == 0x000D, op == 0x0015, op == 0x0016, op == 0x0023, op == 0x00A0:
		return 2, true
	case op == 0x0004, op == 0x0011:
		return 1, true
	case op == 0x0006, op == 0x0007, op == 0x000B, op == 0x000C, op == 0x000E, op == 0x000F, op == 0x0021,
		op >= 0x0068 && op <= 0x006F:
		return 4, true
	case op == 0x0002, op == 0x0009, op == 0x000A, op == 0x0010, op == 0x0020,
		op >= 0x0030 && op <= 0x0037, op >= 0x0040 && op <= 0x0047, op >= 0x0050 && op <= 0x0057:
		return 8, true
	case op == 0x001A, op == 0x001B, op == 0x001D, op == 0x001F, op == 0x0022:
		return 6, true
	case op >= 0x0060 && op <= 0x0067:
		return 12, true
	case op == 0x0012, op == 0x0013, op == 0x0014:
		return r.pixPat()
	case op == 0x0028:
		r.pos += 4
		return r.u8(), true
	case op == 0x0029, op == 0x002A:
		r.pos++
		return r.u8(), true
	case op == 0x002B:
		r.pos += 2
		return r.u8(), true
	case op >= 0x0024 && op <= 0x0027, op >= 0x002C && op <= 0x002F, op >= 0x0092 && op <= 0x0097,
		op >= 0x009C && op <= 0x009F, op >= 0x00A2 && op <= 0x00AF:
		return r.u16(), true
	case op == 0x00A1:
		r.pos += 2
		return r.u16(), true
	case op >= 0x00D0 && op <= 0x00FE, op >= 0x8100:
		return r.u32(), true
	case op >= 0x0100 && op <= 0x7FFF:
		return 2 * (op >> 8), true
	}
	return 0, false
}

// pixPat skips a pattern of pixels (PixPat), and returns 0: the size of
// the data after it.
func (r *reader) pixPat() (int, bool) {
	typ := r.u16()
	r.pos += 8 // the old pattern
	if typ != 1 {
		// a dithered RGB pattern
		r.pos += 6
		return 0, true
	}
	pm, ok := r.pixMap(false)
	if !ok {
		return 0, false
	}
	if _, ok := r.colorTable(); !ok {
		return 0, false
	}
	if _, ok := r.pixels(pm, pm.bounds); !ok {
		return 0, false
	}
	return 0, true
}

// pixMap is a PixMap, or a BitMap of one bit for each pixel.
type pixMap struct {
	rowBytes  int
	bounds    rect
	packType  int
	pixelSize int
	cmpCount  int
	// bitMap is set for a BitMap, which has no colors: 1 is black and 0
	// is white.
	bitMap bool
}

// pixMap reads a PixMap or a BitMap, from its rowBytes. direct is set for
// the pixels of DirectBitsRect.
func (r *reader) pixMap(direct bool) (pixMap, bool) {
	rb := r.u16()
	pm := pixMap{rowBytes: rb & 0x3FFF, bounds: r.rect(), pixelSize: 1, cmpCount: 1}
	if rb&0x8000 == 0 && !direct {
		pm.bitMap = true
		return pm, r.pos <= len(r.b)
	}
	if r.pos+36 > len(r.b) {
		return pm, false
	}
	h := r.b[r.pos:]
	pm.packType = int(be.Uint16(h[2:]))
	pm.pixelSize = int(be.Uint16(h[18:]))
	pm.cmpCount = int(be.Uint16(h[20:]))
	r.pos += 36
	switch pm.pixelSize {
	case 1, 2, 4, 8, 16, 32:
	default:
		return pm, false
	}
	if pm.packType == 0 {
		// the default packing of the size of the pixels
		switch pm.pixelSize {
		case 16:
			pm.packType = 3
		case 32:
			pm.packType = 4
		}
	}
	return pm, true
}

// colorTable reads a ColorTable. The colors take their places by their
// values, unless the flags say that the table is for a device, whose colors
// take their places by their order.
func (r *reader) colorTable() ([]color.NRGBA, bool) {
	if r.pos+8 > len(r.b) {
		return nil, false
	}
	flags := be.Uint16(r.b[r.pos+4:])
	n := int(be.Uint16(r.b[r.pos+6:])) + 1
	r.pos += 8
	if n > 256 || r.pos+8*n > len(r.b) {
		return nil, n == 0x10000
	}
	pal := make([]color.NRGBA, 256)
	for i := range pal {
		pal[i] = color.NRGBA{A: 255}
	}
	for i := range n {
		e := r.b[r.pos+8*i:]
		at := int(be.Uint16(e)) & 0xFF
		if flags&0x8000 != 0 {
			at = i
		}
		pal[at] = color.NRGBA{e[2], e[4], e[6], 255}
	}
	r.pos += 8 * n
	return pal, true
}

// bits reads a BitsRect, BitsRgn, PackBitsRect or PackBitsRgn opcode.
func (r *reader) bits(region, packed bool) error {
	pm, ok := r.pixMap(false)
	if !ok {
		return errShort
	}
	pal := []color.NRGBA{{255, 255, 255, 255}, {0, 0, 0, 255}}
	if !pm.bitMap {
		if pm.pixelSize > 8 {
			return fmt.Errorf("pict indexed pixmap of %d bits: %w", pm.pixelSize, ivy.ErrFormat)
		}
		if pal, ok = r.colorTable(); !ok {
			return errShort
		}
	}
	// BitsRect does not pack its rows
	return r.place(pm, pal, region, !packed)
}

// directBits reads a DirectBitsRect or DirectBitsRgn opcode.
func (r *reader) directBits(region bool) error {
	r.pos += 4 // baseAddr
	pm, ok := r.pixMap(true)
	if !ok {
		return errShort
	}
	if pm.pixelSize != 16 && pm.pixelSize != 32 {
		return fmt.Errorf("pict direct pixmap of %d bits: %w", pm.pixelSize, ivy.ErrFormat)
	}
	return r.place(pm, nil, region, false)
}

// place reads the rectangles, the mode, the region and the pixels of a
// bitmap opcode, and keeps the bitmap. raw is set for the rows that are
// not packed.
func (r *reader) place(pm pixMap, pal []color.NRGBA, region, raw bool) error {
	src, dst := r.rect(), r.rect()
	r.pos += 2 // the mode
	if region {
		n := r.u16()
		if n < 2 {
			return errShort
		}
		r.pos += n - 2
	}
	if r.pos > len(r.b) {
		return errShort
	}
	w, h := pm.bounds.dx(), pm.bounds.dy()
	if w <= 0 || h <= 0 {
		return fmt.Errorf("pict pixmap of %dx%d: %w", w, h, ivy.ErrFormat)
	}
	r.count += int64(w) * int64(h)
	if err := raster.CheckPixels(image.Pt(w, h), r.lim); err != nil {
		return err
	}
	if r.count > r.lim.MaxPixels {
		return fmt.Errorf("pict of more than %d pixels: %w", r.lim.MaxPixels, ivy.ErrLimit)
	}
	if raw {
		pm.packType = 1
	}
	rows, ok := r.pixels(pm, pm.bounds)
	if !ok {
		return errShort
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		convert(img.Pix[y*img.Stride:(y+1)*img.Stride], rows[y], pm, pal)
	}
	if len(r.bitmaps) < r.lim.MaxObjects && !src.empty() && !dst.empty() {
		// src is in the coordinates of the bounds of the pixmap
		src.left -= pm.bounds.left
		src.right -= pm.bounds.left
		src.top -= pm.bounds.top
		src.bottom -= pm.bounds.top
		r.bitmaps = append(r.bitmaps, bitmap{img: img, src: src, dst: dst})
	}
	return nil
}

// pixels reads the rows of the pixels of pm in the rectangle bounds. Each
// row has the bytes of its pixels: for a pixmap of 32 bits of packType 4,
// the planes of its components one after the other.
func (r *reader) pixels(pm pixMap, bounds rect) ([][]byte, bool) {
	w, h := bounds.dx(), bounds.dy()
	if w <= 0 || h <= 0 {
		return nil, false
	}
	// width is the number of bytes of a row once unpacked, and stored the
	// number of bytes of a row that is not packed
	width := (w*pm.pixelSize + 7) / 8
	stored := pm.rowBytes
	switch {
	case pm.pixelSize == 32 && pm.packType == 4:
		width = w * max(3, min(pm.cmpCount, 4))
	case pm.pixelSize == 32 && pm.packType == 2:
		width, stored = 3*w, 3*w
	}
	width = max(width, pm.rowBytes)
	// each row takes a byte of the picture at least
	if int64(width)*int64(h) > r.lim.MaxMemory || int64(w)*int64(h) > r.lim.MaxPixels || h > len(r.b)-r.pos {
		return nil, false
	}
	plain := pm.rowBytes < 8 || pm.packType == 1 || pm.packType == 2
	unit := 1
	if pm.packType == 3 {
		unit = 2
	}
	rows := make([][]byte, h)
	for y := range h {
		row := make([]byte, width)
		if plain {
			if r.pos+stored > len(r.b) {
				return nil, false
			}
			copy(row, r.b[r.pos:r.pos+stored])
			r.pos += stored
		} else {
			var n int
			if pm.rowBytes > 250 {
				n = r.u16()
			} else {
				n = r.u8()
			}
			if r.pos+n > len(r.b) {
				return nil, false
			}
			unpack(row, r.b[r.pos:r.pos+n], unit)
			r.pos += n
		}
		rows[y] = row
	}
	return rows, true
}

// unpack decodes the bytes of PackBits in src into dst, in units of one or
// two bytes. It stops at the end of dst or of src.
func unpack(dst, src []byte, unit int) {
	o := 0
	for i := 0; i < len(src) && o < len(dst); {
		c := int(int8(src[i]))
		i++
		switch {
		case c >= 0:
			n := (c + 1) * unit
			n = min(n, len(src)-i)
			o += copy(dst[o:], src[i:i+n])
			i += n
		case c != -128:
			if i+unit > len(src) {
				return
			}
			for range 1 - c {
				o += copy(dst[o:], src[i:i+unit])
			}
			i += unit
		}
	}
}

// convert converts a row of pixels of pm into the row dst of an NRGBA
// image.
func convert(dst, row []byte, pm pixMap, pal []color.NRGBA) {
	w := len(dst) / 4
	for x := range w {
		var c color.NRGBA
		switch {
		case pm.pixelSize <= 8:
			bit := x * pm.pixelSize
			if bit/8 >= len(row) {
				return
			}
			v := int(row[bit/8]>>(8-pm.pixelSize-bit%8)) & (1<<pm.pixelSize - 1)
			if v < len(pal) {
				c = pal[v]
			}
		case pm.pixelSize == 16:
			if 2*x+1 >= len(row) {
				return
			}
			v := int(be.Uint16(row[2*x:]))
			c = color.NRGBA{five(v >> 10), five(v >> 5), five(v), 255}
		case pm.packType == 4:
			// the planes of the components: alpha, red, green and blue,
			// or red, green and blue. The alpha is often 0, and the
			// picture is opaque.
			o := max(3, min(pm.cmpCount, 4)) - 3
			if (o+2)*w+x >= len(row) {
				return
			}
			c = color.NRGBA{row[o*w+x], row[(o+1)*w+x], row[(o+2)*w+x], 255}
		case pm.packType == 2:
			if 3*x+2 >= len(row) {
				return
			}
			c = color.NRGBA{row[3*x], row[3*x+1], row[3*x+2], 255}
		default:
			// xRGB
			if 4*x+3 >= len(row) {
				return
			}
			c = color.NRGBA{row[4*x+1], row[4*x+2], row[4*x+3], 255}
		}
		dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = c.R, c.G, c.B, c.A
	}
}

// five returns the 8 bits of a component of 5 bits, the low bits of v.
func five(v int) uint8 {
	v &= 0x1F
	return uint8(v<<3 | v>>2)
}

// quickTime reads the image of a CompressedQuickTime or
// UncompressedQuickTime opcode: a JPEG or PNG image after the description
// of the image. The matrix of the opcode places the image in the frame. An
// image that it does not read draws nothing.
func (r *reader) quickTime(data []byte) error {
	img, dst, ok := r.quickTimeImage(data)
	if !ok {
		return nil
	}
	b := img.Bounds()
	r.count += int64(b.Dx()) * int64(b.Dy())
	if r.count > r.lim.MaxPixels {
		return fmt.Errorf("pict of more than %d pixels: %w", r.lim.MaxPixels, ivy.ErrLimit)
	}
	if len(r.bitmaps) < r.lim.MaxObjects {
		r.bitmaps = append(r.bitmaps, bitmap{img: img, src: rect{0, 0, b.Dy(), b.Dx()}, dst: dst})
	}
	return nil
}

// quickTimeImage decodes the image of the data of a QuickTime opcode, and
// returns the rectangle of the frame that it draws in.
func (r *reader) quickTimeImage(data []byte) (image.Image, rect, bool) {
	// the version, the matrix, the size of the matte, its rectangle, the
	// mode, the source rectangle, the accuracy and the size of the mask
	if len(data) < 68 {
		return nil, rect{}, false
	}
	// a matrix of 3 by 3 numbers, of 16.16 bits in its first two columns
	fix := func(i int) float64 { return float64(int32(be.Uint32(data[2+4*i:]))) / 65536 }
	sx, sy, tx, ty := fix(0), fix(4), fix(6), fix(7)
	matte, mask := int64(be.Uint32(data[38:])), int64(be.Uint32(data[64:]))
	at := 68 + matte + mask
	if at+86 > int64(len(data)) {
		return nil, rect{}, false
	}
	desc := data[at:]
	size := int64(be.Uint32(desc))
	w, h := int(be.Uint16(desc[32:])), int(be.Uint16(desc[34:]))
	if size < 86 || at+size > int64(len(data)) || w <= 0 || h <= 0 {
		return nil, rect{}, false
	}
	enc := data[at+size:]
	i := bytes.Index(enc, []byte{0xFF, 0xD8, 0xFF})
	if j := bytes.Index(enc, []byte("\x89PNG\r\n\x1a\n")); j >= 0 && (i < 0 || j < i) {
		i = j
	}
	if i < 0 {
		return nil, rect{}, false
	}
	img, ok := decodeImage(enc[i:], r.lim)
	if !ok {
		return nil, rect{}, false
	}
	dst := rect{top: int(math.Round(ty)), left: int(math.Round(tx))}
	dst.right = dst.left + int(math.Round(float64(w)*sx))
	dst.bottom = dst.top + int(math.Round(float64(h)*sy))
	return img, dst, !dst.empty()
}

// decodeImage decodes the JPEG or PNG image enc within the limits.
func decodeImage(enc []byte, lim ivy.Limits) (image.Image, bool) {
	size, err := raster.Config(enc)
	if err != nil || raster.CheckPixels(size, lim) != nil {
		return nil, false
	}
	img, _, err := image.Decode(bytes.NewReader(enc))
	return img, err == nil
}

// compose draws the bitmaps on a transparent image of the frame, at the
// resolution of the finest bitmap.
func (r *reader) compose(frame rect) (image.Image, error) {
	scale := 1.0
	for _, bm := range r.bitmaps {
		scale = max(scale, float64(bm.src.dx())/float64(bm.dst.dx()), float64(bm.src.dy())/float64(bm.dst.dy()))
	}
	scale = min(scale, maxScale)
	w := int(math.Ceil(float64(frame.dx()) * scale))
	h := int(math.Ceil(float64(frame.dy()) * scale))
	for int64(w)*int64(h) > r.lim.MaxPixels && scale > 1 {
		scale = max(1, scale/2)
		w = int(math.Ceil(float64(frame.dx()) * scale))
		h = int(math.Ceil(float64(frame.dy()) * scale))
	}
	if err := raster.CheckPixels(image.Pt(w, h), r.lim); err != nil {
		return nil, err
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	if len(r.bitmaps) == 1 {
		bm := r.bitmaps[0]
		b := bm.img.Bounds()
		if bm.dst == frame && bm.src == (rect{0, 0, b.Dy(), b.Dx()}) && b.Dx() == w && b.Dy() == h {
			// one bitmap that fills the frame
			draw.Draw(out, out.Rect, bm.img, b.Min, draw.Src)
			return out, nil
		}
	}
	// each bitmap can cover the whole image, so the pixels that the
	// bitmaps draw have a bound too
	budget := 4 * r.lim.MaxPixels
	for _, bm := range r.bitmaps {
		if budget -= stretch(out, bm, frame, scale); budget < 0 {
			return nil, fmt.Errorf("pict that draws more than %d pixels: %w", 4*r.lim.MaxPixels, ivy.ErrLimit)
		}
	}
	return out, nil
}

// stretch draws the bitmap bm on out, which shows the frame at the scale,
// with the nearest pixels of its source rectangle. It returns the number of
// the pixels of out that it covers.
func stretch(out *image.NRGBA, bm bitmap, frame rect, scale float64) int64 {
	b := bm.img.Bounds()
	x0 := int(math.Round(float64(bm.dst.left-frame.left) * scale))
	y0 := int(math.Round(float64(bm.dst.top-frame.top) * scale))
	x1 := int(math.Round(float64(bm.dst.right-frame.left) * scale))
	y1 := int(math.Round(float64(bm.dst.bottom-frame.top) * scale))
	xa, xb := max(0, x0), min(x1, out.Rect.Dx())
	ya, yb := max(0, y0), min(y1, out.Rect.Dy())
	if xb <= xa || yb <= ya {
		return 0
	}
	// the products of the coordinates need 64 bits on a computer of 32
	// bits
	at := func(v, v0, v1, s0, n int) int {
		return s0 + int(int64(v-v0)*int64(n)/int64(v1-v0))
	}
	for y := ya; y < yb; y++ {
		sy := at(y, y0, y1, bm.src.top, bm.src.dy())
		if sy < 0 || sy >= b.Dy() {
			continue
		}
		for x := xa; x < xb; x++ {
			sx := at(x, x0, x1, bm.src.left, bm.src.dx())
			if sx < 0 || sx >= b.Dx() {
				continue
			}
			c := color.NRGBAModel.Convert(bm.img.At(b.Min.X+sx, b.Min.Y+sy)).(color.NRGBA)
			i := y*out.Stride + 4*x
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = c.R, c.G, c.B, c.A
		}
	}
	return int64(xb-xa) * int64(yb-ya)
}

// u8, u16 and u32 read a number, and move past it. At the end of the
// picture, they return 0 and move past the end.
func (r *reader) u8() int {
	r.pos++
	if r.pos > len(r.b) {
		return 0
	}
	return int(r.b[r.pos-1])
}

func (r *reader) u16() int {
	r.pos += 2
	if r.pos > len(r.b) {
		return 0
	}
	return int(be.Uint16(r.b[r.pos-2:]))
}

func (r *reader) u32() int {
	r.pos += 4
	if r.pos > len(r.b) {
		return 0
	}
	return int(int32(be.Uint32(r.b[r.pos-4:])))
}

// rect reads a rectangle.
func (r *reader) rect() rect {
	top, left := int(int16(r.u16())), int(int16(r.u16()))
	bottom, right := int(int16(r.u16())), int(int16(r.u16()))
	return rect{top, left, bottom, right}
}
