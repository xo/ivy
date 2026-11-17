package ps

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"

	"golang.org/x/image/tiff"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
)

// The tags of TIFF that the reader of packed images reads.
const (
	tagImageWidth      = 256
	tagImageLength     = 257
	tagBitsPerSample   = 258
	tagPhotometric     = 262
	tagSamplesPerPixel = 277
	tagPredictor       = 317
	tagColorMap        = 320
	tagTileWidth       = 322
)

// The photometric interpretations of TIFF that the reader of packed images
// reads.
const (
	photoWhiteIsZero = 0
	photoBlackIsZero = 1
	photoPalette     = 3
)

// packedTIFF is a TIFF image of one sample of 2 or 4 bits for each pixel,
// gray or with a palette. golang.org/x/image/tiff refuses such an image, and
// ImageMagick writes the TIFF preview of an EPS file with a palette of 4
// bits.
type packedTIFF struct {
	order         binary.ByteOrder
	width, height int
	bits          int
	photometric   int
	palette       color.Palette
	// widthAt, bitsAt and photoAt are the offsets of the values of the
	// entries ImageWidth, BitsPerSample and PhotometricInterpretation, and
	// widthLong is true if the value of ImageWidth has 4 bytes.
	widthAt, bitsAt, photoAt int
	widthLong                bool
}

// tiffEntry is an entry of the first IFD of a TIFF file.
type tiffEntry struct {
	typ   uint16
	count uint32
	// at is the offset of the value of the entry, in the entry or
	// elsewhere in the file.
	at int
}

// readPackedTIFF reads the first IFD of the TIFF file buf. It returns false
// if the image is not a packed image that decodePackedTIFF decodes.
func readPackedTIFF(buf []byte) (*packedTIFF, bool) {
	if len(buf) < 8 {
		return nil, false
	}
	var order binary.ByteOrder
	switch string(buf[:4]) {
	case "II*\x00":
		order = binary.LittleEndian
	case "MM\x00*":
		order = binary.BigEndian
	default:
		return nil, false
	}
	ifd := int(order.Uint32(buf[4:]))
	if ifd < 8 || ifd > len(buf)-2 {
		return nil, false
	}
	n := int(order.Uint16(buf[ifd:]))
	if ifd+2+12*n > len(buf) {
		return nil, false
	}
	entries := make(map[uint16]tiffEntry, n)
	for i := range n {
		e := buf[ifd+2+12*i:]
		entry := tiffEntry{typ: order.Uint16(e[2:]), count: order.Uint32(e[4:]), at: ifd + 2 + 12*i + 8}
		size := uint64(entry.count)
		switch entry.typ {
		case 3:
			size *= 2
		case 4:
			size *= 4
		default:
			// the reader of packed images needs no other type
			continue
		}
		if size > 4 {
			entry.at = int(order.Uint32(e[8:]))
			if uint64(entry.at)+size > uint64(len(buf)) {
				return nil, false
			}
		}
		entries[order.Uint16(e)] = entry
	}
	// value returns the first value of the entry tag, or def if the file
	// has none
	value := func(tag uint16, def int) int {
		e, ok := entries[tag]
		switch {
		case !ok || e.count == 0:
			return def
		case e.typ == 3:
			return int(order.Uint16(buf[e.at:]))
		}
		return int(order.Uint32(buf[e.at:]))
	}
	p := &packedTIFF{
		order:       order,
		width:       value(tagImageWidth, 0),
		height:      value(tagImageLength, 0),
		bits:        value(tagBitsPerSample, 1),
		photometric: value(tagPhotometric, -1),
	}
	switch {
	case p.width <= 0 || p.height <= 0,
		p.bits != 2 && p.bits != 4,
		value(tagSamplesPerPixel, 1) != 1,
		value(tagPredictor, 1) != 1,
		value(tagTileWidth, 0) != 0:
		return nil, false
	}
	for tag, at := range map[uint16]*int{tagImageWidth: &p.widthAt, tagBitsPerSample: &p.bitsAt, tagPhotometric: &p.photoAt} {
		e := entries[tag]
		if e.count != 1 {
			return nil, false
		}
		*at = e.at
	}
	p.widthLong = entries[tagImageWidth].typ == 4
	switch p.photometric {
	case photoWhiteIsZero, photoBlackIsZero:
	case photoPalette:
		// the colors have 16 bits, the reds first, then the greens and the
		// blues
		e, ok := entries[tagColorMap]
		colors := 1 << p.bits
		if !ok || e.typ != 3 || int(e.count) != 3*colors {
			return nil, false
		}
		at := func(i int) uint8 {
			return uint8(order.Uint16(buf[e.at+2*i:]) >> 8)
		}
		for i := range colors {
			p.palette = append(p.palette, color.RGBA{at(i), at(colors + i), at(2*colors + i), 0xff})
		}
	default:
		return nil, false
	}
	return p, true
}

// decode decodes the image of buf, whose first IFD is p. It gives
// golang.org/x/image/tiff a copy of buf that holds a gray image of 8 bits,
// with one pixel for each byte of a row. The decoder then undoes the
// compression of the strips, and decode reads the samples from the bytes of
// each row.
func (p *packedTIFF) decode(buf []byte) (image.Image, error) {
	rowBytes := (p.width*p.bits + 7) / 8
	alt := bytes.Clone(buf)
	if p.widthLong {
		p.order.PutUint32(alt[p.widthAt:], uint32(rowBytes))
	} else {
		p.order.PutUint16(alt[p.widthAt:], uint16(rowBytes))
	}
	p.order.PutUint16(alt[p.bitsAt:], 8)
	p.order.PutUint16(alt[p.photoAt:], photoBlackIsZero)
	img, err := tiff.Decode(bytes.NewReader(alt))
	if err != nil {
		return nil, tiffError(err)
	}
	gray, ok := img.(*image.Gray)
	if !ok || gray.Rect.Dx() != rowBytes || gray.Rect.Dy() != p.height {
		return nil, fmt.Errorf("decoding tiff preview: %w", ivy.ErrFormat)
	}
	palette := p.palette
	if palette == nil {
		levels := 1 << p.bits
		for i := range levels {
			v := uint8(i * 255 / (levels - 1))
			if p.photometric == photoWhiteIsZero {
				v = 255 - v
			}
			palette = append(palette, color.Gray{Y: v})
		}
	}
	out := image.NewPaletted(image.Rect(0, 0, p.width, p.height), palette)
	mask := byte(1<<p.bits - 1)
	for y := range p.height {
		row := gray.Pix[y*gray.Stride:]
		for x := range p.width {
			bit := x * p.bits
			shift := 8 - p.bits - bit%8
			out.Pix[y*out.Stride+x] = row[bit/8] >> shift & mask
		}
	}
	return out, nil
}

// tiffConfig returns the size of the TIFF preview r, and the packed image of
// the preview if golang.org/x/image/tiff does not support it.
func tiffConfig(r *io.SectionReader, lim ivy.Limits) (image.Point, *packedTIFF, []byte, error) {
	cfg, err := tiff.DecodeConfig(io.NewSectionReader(r, 0, r.Size()))
	if err == nil {
		return image.Pt(cfg.Width, cfg.Height), nil, nil, nil
	}
	if _, ok := errors.AsType[tiff.UnsupportedError](err); !ok {
		return image.Point{}, nil, nil, tiffError(err)
	}
	buf, rerr := raster.ReadAll(io.NewSectionReader(r, 0, r.Size()), lim.WithDefaults().MaxMemory)
	if rerr != nil {
		return image.Point{}, nil, nil, fmt.Errorf("decoding tiff preview: %w", rerr)
	}
	p, ok := readPackedTIFF(buf)
	if !ok {
		return image.Point{}, nil, nil, tiffError(err)
	}
	return image.Pt(p.width, p.height), p, buf, nil
}

// tiffPreview returns the TIFF preview of a DOS EPS file.
func (d *Document) tiffPreview() ivy.Preview {
	size, _, _, _ := tiffConfig(d.tiff, d.lim)
	return ivy.Preview{
		Kind: ivy.Thumbnail,
		Size: size,
		Decode: func(ctx context.Context) (image.Image, error) {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("decoding tiff preview: %w", err)
			}
			size, packed, buf, err := tiffConfig(d.tiff, d.lim)
			if err != nil {
				return nil, err
			}
			if err := raster.CheckPixels(size, d.lim); err != nil {
				return nil, err
			}
			if packed != nil {
				return packed.decode(buf)
			}
			img, err := tiff.Decode(io.NewSectionReader(d.tiff, 0, d.tiff.Size()))
			if err != nil {
				return nil, tiffError(err)
			}
			return img, nil
		},
	}
}

// tiffError wraps an error of the TIFF decoder with ivy.ErrUnsupported for a
// feature that the decoder does not support, and with ivy.ErrFormat for any
// other.
func tiffError(err error) error {
	if _, ok := errors.AsType[tiff.UnsupportedError](err); ok {
		return fmt.Errorf("decoding tiff preview: %w: %w", ivy.ErrUnsupported, err)
	}
	return fmt.Errorf("decoding tiff preview: %w: %w", ivy.ErrFormat, err)
}
