package odraw

import (
	"bytes"
	"compress/zlib"
	"context"
	"image"
	"image/draw"
	"image/png"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
)

// Entry is an entry of the store of pictures: an OfficeArtFBSE (MS-ODRAW
// 2.2.32). The picture is the record in the entry, or the record at the
// offset Delay of the stream of the pictures of the file, such as the
// stream "Pictures" of a presentation.
type Entry struct {
	// Embedded is the picture record in the entry, or nil.
	Embedded []byte
	// Delay is the offset of the picture record in the stream of the
	// pictures, when Embedded is nil.
	Delay uint32
	// OK is false for an entry of another type, which keeps the indexes of
	// the others.
	OK bool
}

// fbseSize is the size of an OfficeArtFBSE without its name and its
// picture.
const fbseSize = 36

// ReadStore reads the entries of the store of pictures of the body dgg of
// an OfficeArtDggContainer, maxObjects of them at most. The index 1 of a
// shape is the first entry. A store that dgg cuts gives the entries before
// the cut, so that a file whose last picture is too large keeps the others.
func ReadStore(dgg []byte, maxObjects int) []Entry {
	var out []Entry
	for len(dgg) >= 8 {
		h, _ := ReadHeader(dgg)
		end := 8 + int(min(uint64(h.Size), uint64(len(dgg)-8)))
		if h.Type == RecBStoreContainer {
			out = append(out, ReadEntries(dgg[8:end], maxObjects-len(out))...)
			if len(out) >= maxObjects {
				return out
			}
		}
		dgg = dgg[end:]
	}
	return out
}

// ReadEntries reads the OfficeArtFBSE records in b, maxObjects of them at
// most, such as those after the shape of an inline picture of Word (MS-DOC
// 2.9.134). A record of another type is an entry that is not OK.
func ReadEntries(b []byte, maxObjects int) []Entry {
	var out []Entry
	for h, e := range Children(b) {
		if len(out) >= maxObjects {
			break
		}
		out = append(out, readFBSE(h, e))
	}
	return out
}

// readFBSE reads an OfficeArtFBSE.
func readFBSE(h Header, b []byte) Entry {
	if h.Type != RecFBSE || len(b) < fbseSize {
		return Entry{}
	}
	e := Entry{Delay: le.Uint32(b[28:]), OK: true}
	if cbName := int(b[33]); fbseSize+cbName < len(b) {
		e.Embedded = b[fbseSize+cbName:]
	}
	return e
}

// Record returns the picture record of the entry: the embedded one, or the
// one at its offset in the stream delay. It returns nil for an entry
// without a picture.
func (e Entry) Record(delay []byte) []byte {
	switch {
	case !e.OK:
		return nil
	case e.Embedded != nil:
		return e.Embedded
	case uint64(e.Delay) < uint64(len(delay)):
		return delay[e.Delay:]
	}
	return nil
}

// Data returns the bytes of the picture of an OfficeArtBlip record at the
// start of b, as a file that internal/drawingml decodes: an EMF or a WMF
// file, decompressed, a JPEG, PNG or TIFF file, or a BMP file of a DIB
// (MS-ODRAW 2.2.23). A metafile stays within maxMemory bytes. It returns
// nil for a picture that it does not read, such as PICT.
func Data(b []byte, maxMemory int64) []byte {
	h, ok := ReadHeader(b)
	if !ok || uint64(h.Size) > uint64(len(b)-8) {
		return nil
	}
	body := b[8 : 8+h.Size]
	// an odd instance has a second identifier of 16 bytes
	uids := 16
	if h.Inst&1 == 1 {
		uids = 32
	}
	switch h.Type {
	case RecBlipEMF, RecBlipWMF:
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
	case RecBlipJPEG, RecBlipJPEGCMYK, RecBlipPNG, RecBlipTIFF:
		// the identifiers and a tag of one byte
		if len(body) < uids+1 {
			return nil
		}
		return body[uids+1:]
	case RecBlipDIB:
		if len(body) < uids+1+40 {
			return nil
		}
		return BMPFile(body[uids+1:])
	}
	return nil
}

// BMPFile returns the DIB dib as a BMP file: a BITMAPFILEHEADER of 14 bytes
// before it, with the offset of its pixels. It returns nil for a DIB
// without a header.
func BMPFile(dib []byte) []byte {
	if len(dib) < 4 {
		return nil
	}
	hdr := int(le.Uint32(dib))
	if hdr < 12 || hdr > len(dib) || len(dib) > 1<<31-15 {
		return nil
	}
	// pal is the size of the color table in bytes
	colors, pal := 0, 0
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
		pal = 4 * min(max(colors, 0), 1<<16)
	} else if bits := int(le.Uint16(dib[10:])); bits <= 8 {
		// a BITMAPCOREHEADER has colors of 3 bytes
		pal = 3 << bits
	}
	off := 14 + hdr + pal
	out := make([]byte, 14, 14+len(dib))
	out[0], out[1] = 'B', 'M'
	le.PutUint32(out[2:], uint32(14+len(dib)))
	le.PutUint32(out[10:], uint32(off))
	return append(out, dib...)
}

// ColorKey decodes the raster picture data, makes each pixel of the color
// r, g and b transparent, and returns the picture as a PNG file. It returns
// false for a picture that it cannot decode, such as a metafile.
func ColorKey(ctx context.Context, data []byte, r, g, b uint8, lim ivy.Limits) ([]byte, bool) {
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
