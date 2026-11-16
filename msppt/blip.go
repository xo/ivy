package msppt

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"image"
	"image/draw"
	"image/png"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
)

// blipEntry is an entry of the store of pictures: an OfficeArtFBSE (MS-ODRAW
// 2.2.32). The picture is the record in the entry, or the record at the
// offset foDelay of the stream "Pictures".
type blipEntry struct {
	embedded []byte
	foDelay  uint32
	size     uint32
	ok       bool
}

// fbseSize is the size of an OfficeArtFBSE without its name and its
// picture.
const fbseSize = 36

// readFBSE reads an OfficeArtFBSE. An entry of another type is empty, so
// that the indexes of the others stay.
func readFBSE(h header, b []byte) blipEntry {
	if h.typ != rtOfficeArtFBSE || len(b) < fbseSize {
		return blipEntry{}
	}
	e := blipEntry{size: le.Uint32(b[20:]), foDelay: le.Uint32(b[28:]), ok: true}
	if cbName := int(b[33]); fbseSize+cbName < len(b) {
		e.embedded = b[fbseSize+cbName:]
	}
	return e
}

// blipData returns the bytes of the picture of an OfficeArtBlip record at
// the start of b, as a file that internal/drawingml decodes: an EMF or a
// WMF file, decompressed, a JPEG, PNG or TIFF file, or a BMP file of a DIB
// (MS-ODRAW 2.2.23). It returns nil for a picture that it does not read,
// such as PICT.
func blipData(b []byte, maxMemory int64) []byte {
	h, ok := readHeader(b)
	if !ok || uint64(h.size) > uint64(len(b)-8) {
		return nil
	}
	body := b[8 : 8+h.size]
	// an odd instance has a second identifier of 16 bytes
	uids := 16
	if h.inst&1 == 1 {
		uids = 32
	}
	switch h.typ {
	case rtOfficeArtBlipEMF, rtOfficeArtBlipWMF:
		// the identifiers and an OfficeArtMetafileHeader of 34 bytes
		if len(body) < uids+34 {
			return nil
		}
		mh := body[uids:]
		size := int64(le.Uint32(mh))
		data := body[uids+34:]
		if mh[32] == 0xFE {
			return data
		}
		if size <= 0 || size > maxMemory {
			return nil
		}
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil
		}
		out, err := raster.ReadAll(zr, size)
		if err != nil {
			return nil
		}
		return out
	case rtOfficeArtBlipJPEG, rtOfficeArtBlipJPEG2, rtOfficeArtBlipPNG, rtOfficeArtBlipTIFF:
		// the identifiers and a tag of one byte
		if len(body) < uids+1 {
			return nil
		}
		return body[uids+1:]
	case rtOfficeArtBlipDIB:
		if len(body) < uids+1+40 {
			return nil
		}
		return bmpFile(body[uids+1:])
	}
	return nil
}

// bmpFile returns the DIB dib as a BMP file: a BITMAPFILEHEADER of 14 bytes
// before it, with the offset of its pixels.
func bmpFile(dib []byte) []byte {
	hdr := int(le.Uint32(dib))
	if hdr < 12 || hdr > len(dib) {
		return nil
	}
	colors := 0
	if hdr >= 40 {
		bits := int(le.Uint16(dib[14:]))
		colors = int(le.Uint32(dib[32:]))
		if colors == 0 && bits <= 8 {
			colors = 1 << bits
		}
		if le.Uint32(dib[16:]) == 3 && hdr == 40 {
			// the three masks of BI_BITFIELDS follow the header
			colors = 3
		}
	}
	off := 14 + hdr + 4*colors
	out := make([]byte, 14, 14+len(dib))
	out[0], out[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(out[2:], uint32(14+len(dib)))
	binary.LittleEndian.PutUint32(out[10:], uint32(off))
	return append(out, dib...)
}

// colorKey decodes the raster picture data, makes each pixel of the color
// r, g and b transparent, and returns the picture as a PNG file. It returns
// false for a picture that it cannot decode, such as a metafile.
func colorKey(ctx context.Context, data []byte, r, g, b uint8, lim ivy.Limits) ([]byte, bool) {
	if ctx == nil || len(data) == 0 || data[0] == 0x01 || data[0] == 0xD7 {
		return nil, false
	}
	img, err := raster.Decode(ctx, data, lim)
	if err != nil {
		return nil, false
	}
	bounds := img.Bounds()
	out := image.NewNRGBA(bounds)
	draw.Draw(out, bounds, img, bounds.Min, draw.Src)
	pix := out.Pix
	for i := 0; i+4 <= len(pix); i += 4 {
		if pix[i] == r && pix[i+1] == g && pix[i+2] == b {
			pix[i+3] = 0
		}
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, out); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}
