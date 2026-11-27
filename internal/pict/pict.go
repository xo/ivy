// Package pict reads the pictures of QuickDraw, PICT version 1 and
// version 2, as Inside Macintosh: Imaging With QuickDraw, Appendix A, gives
// them. The store of pictures of Office Drawing holds such pictures, from
// Office for the Macintosh.
//
// The package draws the opcodes of a picture onto the display list of D10:
// the lines, the rectangles, the rounded rectangles, the ovals, the arcs,
// the polygons and the regions, with the pen, the patterns, the transfer
// modes and the colors, the text with the fonts of internal/fontdb, the
// bitmaps, and the images of QuickTime, such as JPEG. Decode draws the list
// on a transparent image of the frame of the picture.
package pict

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
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

// The resolution of the image of a picture with lines, shapes or text:
// vectorScale pixels for each point of the frame, with at most maxSide
// pixels on its longest side.
const (
	vectorScale = 4
	maxSide     = 2048
)

// Picture is a picture that Open read: the drawing of its opcodes, as a
// display list in the coordinates of its frame. It is safe for concurrent
// use.
type Picture struct {
	frame rect
	list  *display.List
	// scale is the number of the pixels of the finest bitmap for each unit
	// of the frame, and vector is set for a picture that draws lines,
	// shapes or text.
	scale  float64
	vector bool
	// area is the sum of the areas that the commands of the list cover, in
	// the units of the frame.
	area float64
}

// Open reads the picture in data, and draws its opcodes onto a display
// list. data can start with the header of 512 bytes of a PICT file. The
// text takes the fonts of cfg. It returns an error that wraps ivy.ErrFormat
// for data that is not a picture or draws nothing, and one that wraps
// ivy.ErrLimit for a picture past the limits.
func Open(data []byte, cfg ivy.Config) (*Picture, error) {
	lim := cfg.Limits.WithDefaults()
	p, err := open(data, cfg, lim)
	if err != nil && len(data) > headerSize+10 {
		if p2, err2 := open(data[headerSize:], cfg, lim); err2 == nil {
			return p2, nil
		}
	}
	return p, err
}

// Decode returns the image of the picture in data, as Open reads it, with
// the fonts that internal/fontdb gives without a font of the program. The
// image has the size of the frame of the picture, times the resolution of
// its finest bitmap, or times vectorScale for a picture with lines, shapes
// or text.
func Decode(data []byte, lim ivy.Limits) (image.Image, error) {
	p, err := Open(data, ivy.Config{Limits: lim})
	if err != nil {
		return nil, err
	}
	return p.Image(lim)
}

// Size returns the size of the frame of the picture, in its units: points
// for a picture of 72 dots per inch.
func (p *Picture) Size() (width, height float64) {
	return float64(p.frame.dx()), float64(p.frame.dy())
}

// Draw replays the drawing of the picture onto d, into the rectangle dst of
// the user space of d. It changes no state of d that outlives the call.
func (p *Picture) Draw(d display.Device, dst okage.Rect) {
	d.Save()
	defer d.Restore()
	d.ClipPath(okage.PathFromRect(dst), okage.FillRuleWinding)
	t := okage.TransformFromTranslate(dst.X(), dst.Y()).
		PreScale(dst.Width()/float32(p.frame.dx()), dst.Height()/float32(p.frame.dy())).
		PreTranslate(-float32(p.frame.left), -float32(p.frame.top))
	d.Concat(t)
	p.list.Replay(d)
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

// reader reads the opcodes of a picture, and draws them onto list.
type reader struct {
	b   []byte
	pos int
	v2  bool
	cfg ivy.Config
	lim ivy.Limits
	// count counts the pixels of the bitmaps that the picture holds.
	count int64

	list  *display.List
	frame rect
	st    state
	// drew is set once the picture draws a command, and vector once it
	// draws a line, a shape or text. scale and area are those of Picture.
	drew, vector bool
	scale        float64
	area         float64

	// db is the database of the fonts, which the first text loads, faces
	// the faces that the text took, and tiles the shaders of the patterns.
	db    *svg.FontDatabase
	faces map[faceKey]*font.Face
	tiles map[tileKey]okage.Shader
}

// errShort is the error of a picture that ends in an opcode.
var errShort = errors.New("pict: picture cut short")

// open reads the picture at the start of data.
func open(data []byte, cfg ivy.Config, lim ivy.Limits) (p *Picture, err error) {
	defer func() {
		if r := recover(); r != nil {
			p, err = nil, fmt.Errorf("reading pict: panic: %v: %w", r, ivy.ErrFormat)
		}
	}()
	if len(data) < 12 {
		return nil, fmt.Errorf("pict of %d bytes: %w", len(data), ivy.ErrFormat)
	}
	r := &reader{b: data, pos: 2, cfg: cfg, lim: lim, list: display.NewRecording(), st: newState(), scale: 1}
	r.frame = r.rect()
	if r.frame.empty() {
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
	if err := r.run(); err != nil {
		return nil, err
	}
	if !r.drew {
		return nil, fmt.Errorf("pict that draws nothing: %w", ivy.ErrFormat)
	}
	return &Picture{frame: r.frame, list: r.list, scale: r.scale, vector: r.vector, area: r.area}, nil
}

// run reads the opcodes up to the end of the picture. An extended header
// of version 2 sets the frame to its own rectangle, whose coordinates the
// opcodes use.
func (r *reader) run() error {
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
			// a picture that ends without OpEndPic keeps its drawing
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
					r.frame = src
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

// op reads the data of the opcode op, and draws it.
func (r *reader) op(op int) error {
	if op >= 0x0020 && op <= 0x00FE && op != 0x00A0 && op != 0x00A1 {
		// the opcodes of the lines, the text, the shapes and the bitmaps
		return r.drawOp(op)
	}
	st := &r.st
	switch op {
	case 0x0001:
		g, ok := r.region()
		if !ok {
			return errShort
		}
		r.setClip(g)
		return nil
	case 0x0002, 0x0009, 0x000A:
		if !r.has(8) {
			return errShort
		}
		pat := r.pattern()
		switch op {
		case 0x0002:
			st.bkPat = pat
		case 0x0009:
			st.penPat = pat
		default:
			st.fillPat = pat
		}
		return nil
	case 0x0012, 0x0013, 0x0014:
		pat, ok := r.pixPat()
		if !ok {
			return errShort
		}
		switch op {
		case 0x0012:
			st.bkPat = pat
		case 0x0013:
			st.penPat = pat
		default:
			st.fillPat = pat
		}
		return nil
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
	at := r.pos
	r.state(op)
	r.pos = at + n
	return nil
}

// state reads the data of an opcode of the state of the drawing, of a size
// that the caller checked.
func (r *reader) state(op int) {
	st := &r.st
	switch op {
	case 0x0003:
		st.txFont = r.s16()
	case 0x0004:
		st.txFace = r.u8()
	case 0x0005:
		st.txMode = r.u16()
	case 0x0006:
		st.spExtra = float32(r.u32()) / 65536
	case 0x0007:
		st.penSize = r.size2()
	case 0x0008:
		st.penMode = r.u16()
	case 0x000B:
		st.ovSize = r.size2()
	case 0x000C:
		dh, dv := r.s16(), r.s16()
		st.origin.h += dh
		st.origin.v += dv
	case 0x000D:
		st.txSize = r.s16()
	case 0x000E:
		st.fg = oldColor(r.u32())
	case 0x000F:
		st.bg = oldColor(r.u32())
	case 0x0010:
		numer, denom := r.size2(), r.size2()
		st.ratio = [2]float32{ratio(numer.h, denom.h), ratio(numer.v, denom.v)}
	case 0x001A:
		st.fg = r.rgb()
	case 0x001B:
		st.bg = r.rgb()
	}
}

// drawOp reads and draws an opcode from 0x0020 to 0x00FE.
func (r *reader) drawOp(op int) error {
	st := &r.st
	switch {
	case op >= 0x0020 && op <= 0x0023:
		sizes := [4]int{8, 4, 6, 2}
		if !r.has(sizes[op-0x0020]) {
			return errShort
		}
		var from, to point
		switch op {
		case 0x0020:
			from = r.point()
			to = r.point()
		case 0x0021:
			from, to = st.pen, r.point()
		case 0x0022:
			from = r.point()
			to = point{from.h + int(int8(r.u8())), from.v + int(int8(r.u8()))}
		default:
			from = st.pen
			to = point{from.h + int(int8(r.u8())), from.v + int(int8(r.u8()))}
		}
		r.line(from, to)
		return nil
	case op >= 0x0028 && op <= 0x002B:
		return r.textOp(op)
	case op == 0x002C:
		return r.fontName()
	case op >= 0x0030 && op <= 0x0034, op >= 0x0040 && op <= 0x0044, op >= 0x0050 && op <= 0x0054:
		if !r.has(8) {
			return errShort
		}
		st.last = r.prect()
		r.shape(op, 0, 0)
		return nil
	case op >= 0x0038 && op <= 0x003C, op >= 0x0048 && op <= 0x004C, op >= 0x0058 && op <= 0x005C:
		r.shape(op, 0, 0)
		return nil
	case op >= 0x0060 && op <= 0x0064:
		if !r.has(12) {
			return errShort
		}
		st.last = r.prect()
		start, arc := r.s16(), r.s16()
		r.shape(op, start, arc)
		return nil
	case op >= 0x0068 && op <= 0x006C:
		if !r.has(4) {
			return errShort
		}
		start, arc := r.s16(), r.s16()
		r.shape(op, start, arc)
		return nil
	case op >= 0x0070 && op <= 0x0074:
		pts, ok := r.polygon()
		if !ok {
			return errShort
		}
		st.lastPoly = pts
		r.poly(op&7, pts)
		return nil
	case op >= 0x0078 && op <= 0x007C:
		r.poly(op&7, st.lastPoly)
		return nil
	case op >= 0x0080 && op <= 0x0084:
		g, ok := r.region()
		if !ok {
			return errShort
		}
		st.lastRgn = g
		r.regionOp(op&7, g)
		return nil
	case op >= 0x0088 && op <= 0x008C:
		r.regionOp(op&7, st.lastRgn)
		return nil
	case op == 0x0090, op == 0x0091, op == 0x0098, op == 0x0099:
		return r.bits(op&1 == 1, op >= 0x0098)
	case op == 0x009A, op == 0x009B:
		return r.directBits(op == 0x009B)
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

// pixPat reads a pattern of pixels (PixPat): an old pattern, then a color
// for a dithered pattern, or a PixMap with its colors and its pixels. A
// PixMap of more than maxTile pixels on a side keeps its old pattern.
func (r *reader) pixPat() (pattern, bool) {
	if !r.has(10) {
		return pattern{}, false
	}
	typ := r.u16()
	pat := r.pattern()
	if typ != 1 {
		// a dithered RGB pattern
		if !r.has(6) {
			return pattern{}, false
		}
		return pattern{rgb: true, c: r.rgb()}, true
	}
	pm, ok := r.pixMap(false)
	if !ok {
		return pattern{}, false
	}
	pal, ok := r.colorTable()
	if !ok {
		return pattern{}, false
	}
	rows, ok := r.pixels(pm, pm.bounds)
	if !ok {
		return pattern{}, false
	}
	w, h := pm.bounds.dx(), pm.bounds.dy()
	if w > maxTile || h > maxTile || (pm.pixelSize <= 8 && pal == nil) {
		return pat, true
	}
	tile := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		convert(tile.Pix[y*tile.Stride:(y+1)*tile.Stride], rows[y], pm, pal)
	}
	pat.tile = tile
	return pat, true
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

// bits reads a BitsRect, BitsRgn, PackBitsRect or PackBitsRgn opcode. A
// BitMap takes the colors of the foreground and the background.
func (r *reader) bits(masked, packed bool) error {
	pm, ok := r.pixMap(false)
	if !ok {
		return errShort
	}
	pal := []color.NRGBA{r.st.bg, r.st.fg}
	if !pm.bitMap {
		if pm.pixelSize > 8 {
			return fmt.Errorf("pict indexed pixmap of %d bits: %w", pm.pixelSize, ivy.ErrFormat)
		}
		if pal, ok = r.colorTable(); !ok {
			return errShort
		}
	}
	// BitsRect does not pack its rows
	return r.place(pm, pal, masked, !packed)
}

// directBits reads a DirectBitsRect or DirectBitsRgn opcode.
func (r *reader) directBits(masked bool) error {
	r.pos += 4 // baseAddr
	pm, ok := r.pixMap(true)
	if !ok {
		return errShort
	}
	if pm.pixelSize != 16 && pm.pixelSize != 32 {
		return fmt.Errorf("pict direct pixmap of %d bits: %w", pm.pixelSize, ivy.ErrFormat)
	}
	return r.place(pm, nil, masked, false)
}

// place reads the rectangles, the mode, the region and the pixels of a
// bitmap opcode, and draws the bitmap. masked is set for an opcode with a
// region, and raw for the rows that are not packed.
func (r *reader) place(pm pixMap, pal []color.NRGBA, masked, raw bool) error {
	if !r.has(18) {
		return errShort
	}
	src, dst := r.rect(), r.prect()
	mode := r.u16()
	var clip *region
	if masked {
		g, ok := r.region()
		if !ok {
			return errShort
		}
		clip = &g
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
	if pm.bitMap {
		pal = bitMapColors(pal, mode)
	}
	rows, ok := r.pixels(pm, pm.bounds)
	if !ok {
		return errShort
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		convert(img.Pix[y*img.Stride:(y+1)*img.Stride], rows[y], pm, pal)
	}
	if src.empty() || dst.empty() {
		return nil
	}
	// src is in the coordinates of the bounds of the pixmap
	src.left -= pm.bounds.left
	src.right -= pm.bounds.left
	src.top -= pm.bounds.top
	src.bottom -= pm.bounds.top
	r.image(img, src, dst, clip)
	return nil
}

// bitMapColors returns the colors of the bits 0 and 1 of a BitMap in the
// transfer mode: pal for srcCopy, and no color for the bits that srcOr,
// srcBic and transparent leave as they are.
func bitMapColors(pal []color.NRGBA, mode int) []color.NRGBA {
	bg, fg := pal[0], pal[1]
	switch mode &^ 64 {
	case 1, 36:
		return []color.NRGBA{{}, fg}
	case 3:
		return []color.NRGBA{{}, bg}
	case 4:
		return []color.NRGBA{fg, bg}
	case 5:
		return []color.NRGBA{fg, {}}
	case 7:
		return []color.NRGBA{bg, {}}
	}
	return pal
}

// image draws the pixels src of img into the rectangle dst of the
// picture, within the region clip when it is not nil.
func (r *reader) image(img image.Image, src, dst rect, clip *region) {
	d, ok := dst.okage()
	if !ok || !r.cover(d) {
		return
	}
	r.scale = max(r.scale, float64(src.dx())/float64(dst.dx()), float64(src.dy())/float64(dst.dy()))
	t := okage.TransformFromTranslate(float32(dst.left), float32(dst.top)).
		PreScale(float32(dst.dx())/float32(src.dx()), float32(dst.dy())/float32(src.dy())).
		PreTranslate(-float32(src.left), -float32(src.top))
	r.begin()
	r.list.Save()
	r.list.ClipPath(okage.PathFromRect(d), okage.FillRuleWinding)
	if clip != nil {
		if p := clip.path(); p != nil {
			r.list.ClipPath(p, okage.FillRuleWinding)
		}
	}
	r.list.DrawImage(img, t, display.ImageOptions{Opacity: 1})
	r.list.Restore()
	r.end()
	r.drew = true
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
	r.image(img, rect{b.Min.Y, b.Min.X, b.Max.Y, b.Max.X}, dst, nil)
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

// Image draws the picture on a transparent image of its frame, at the
// resolution of its finest bitmap, or at vectorScale for a picture with
// lines, shapes or text. The image has fewer pixels than lim.MaxPixels, and
// the commands draw at most 4 times as many pixels.
func (p *Picture) Image(lim ivy.Limits) (img image.Image, err error) {
	lim = lim.WithDefaults()
	dx, dy := float64(p.frame.dx()), float64(p.frame.dy())
	scale := min(p.scale, maxScale)
	if p.vector {
		scale = max(scale, min(vectorScale, maxSide/max(dx, dy)))
	}
	budget := 4 * float64(lim.MaxPixels)
	size := func() (int, int) {
		return int(math.Ceil(dx * scale)), int(math.Ceil(dy * scale))
	}
	w, h := size()
	for (int64(w)*int64(h) > lim.MaxPixels || p.area*scale*scale > budget) && scale > 1 {
		scale = max(1, scale/2)
		w, h = size()
	}
	if err := raster.CheckPixels(image.Pt(w, h), lim); err != nil {
		return nil, err
	}
	if p.area*scale*scale > budget {
		return nil, fmt.Errorf("pict that draws more than %d pixels: %w", 4*lim.MaxPixels, ivy.ErrLimit)
	}
	pm, ok := okage.NewPixmap(uint32(w), uint32(h))
	if !ok {
		return nil, fmt.Errorf("pict image of %dx%d: %w", w, h, ivy.ErrLimit)
	}
	dst, ok := okage.RectFromXYWH(0, 0, float32(w), float32(h))
	if !ok {
		return nil, fmt.Errorf("pict image of %dx%d: %w", w, h, ivy.ErrFormat)
	}
	defer func() {
		if r := recover(); r != nil {
			img, err = nil, fmt.Errorf("drawing pict: panic: %v: %w", r, ivy.ErrFormat)
		}
	}()
	p.Draw(display.NewOkage(pm, okage.IdentityTransform()), dst)
	return pm, nil
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

// s16 reads a signed number of 16 bits.
func (r *reader) s16() int {
	return int(int16(r.u16()))
}

// has reports whether the picture holds n more bytes.
func (r *reader) has(n int) bool {
	return r.pos+n <= len(r.b)
}

// rect reads a rectangle.
func (r *reader) rect() rect {
	top, left := r.s16(), r.s16()
	bottom, right := r.s16(), r.s16()
	return rect{top, left, bottom, right}
}

// prect reads a rectangle of the picture, in the coordinates of the frame.
func (r *reader) prect() rect {
	o := r.st.origin
	b := r.rect()
	return rect{b.top - o.v, b.left - o.h, b.bottom - o.v, b.right - o.h}
}

// point reads a point of the picture, in the coordinates of the frame.
func (r *reader) point() point {
	v, h := r.s16(), r.s16()
	return point{h - r.st.origin.h, v - r.st.origin.v}
}

// size2 reads a point that is a size, such as the size of the pen.
func (r *reader) size2() point {
	v, h := r.s16(), r.s16()
	return point{h, v}
}

// rgb reads an RGBColor.
func (r *reader) rgb() color.NRGBA {
	red, green, blue := r.u16(), r.u16(), r.u16()
	return color.NRGBA{uint8(red >> 8), uint8(green >> 8), uint8(blue >> 8), 255}
}
