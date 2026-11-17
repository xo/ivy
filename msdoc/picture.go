package msdoc

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"strconv"

	"github.com/xo/ivy/internal/raster"
)

// This file reads the inline pictures of a document: the character 0x01
// whose properties give the offset of a PICFAndOfficeArtData in the stream
// Data (MS-DOC 2.9.192). The picture is an OfficeArtBlip record of Office
// Drawing (MS-ODRAW 2.2.23) in it, or in the stream WordDocument.

// The types of the records of Office Drawing that the package reads.
const (
	rtFBSE      = 0xF007
	rtBlipFirst = 0xF018
	rtBlipLast  = 0xF117
	rtBlipEMF   = 0xF01A
	rtBlipWMF   = 0xF01B
	rtBlipJPEG  = 0xF01D
	rtBlipPNG   = 0xF01E
	rtBlipDIB   = 0xF01F
	rtBlipTIFF  = 0xF029
	rtBlipJPEG2 = 0xF02A
)

// picfSize is the size of a PICF, the header of a picture.
const picfSize = 0x44

// mmShape is the mm of a PICF whose picture is a shape with a name.
const mmShape = 0x66

// picture is an inline picture: its file in the source of the layout and
// its size in points.
type picture struct {
	name          string
	width, height float64
}

// media is a file of a picture for the source of the layout.
type media struct {
	name string
	data []byte
}

// picture returns the picture of the character 0x01 with the properties c,
// or nil for a picture that the package does not read, such as a PICT.
func (p *parser) picture(c chp) *picture {
	if c.data || p.s.data == nil {
		return nil
	}
	if pic, ok := p.pictures[c.picLocation]; ok {
		return pic
	}
	p.pictures[c.picLocation] = nil
	if len(p.pictures) > p.lim.MaxObjects {
		return nil
	}
	data := p.s.data
	off := int(c.picLocation)
	if off < 0 || off+picfSize > len(data) {
		return nil
	}
	lcb := int(le.Uint32(data[off:]))
	cbHeader := int(le.Uint16(data[off+4:]))
	if cbHeader < picfSize || lcb < cbHeader || off+lcb > len(data) {
		return nil
	}
	picf := data[off : off+lcb]
	// the size that the picture shows, in twips, and its scale in tenths
	// of a percent
	dxa, dya := float64(le.Uint16(picf[28:])), float64(le.Uint16(picf[30:]))
	mx, my := float64(le.Uint16(picf[32:])), float64(le.Uint16(picf[34:]))
	if mx == 0 {
		mx = 1000
	}
	if my == 0 {
		my = 1000
	}
	rest := picf[cbHeader:]
	if le.Uint16(picf[6:]) == mmShape && len(rest) > 0 {
		rest = rest[min(1+int(rest[0]), len(rest)):]
	}
	blip, ext := p.findBlip(rest, 0)
	if blip == nil {
		return nil
	}
	name := "media/" + strconv.Itoa(len(p.media)+1) + ext
	p.media = append(p.media, media{name: name, data: blip})
	pic := &picture{name: name, width: dxa * mx / 1000 / 20, height: dya * my / 1000 / 20}
	if pic.width <= 0 || pic.height <= 0 || pic.width > 1584 || pic.height > 1584 {
		pic.width, pic.height = 0, 0
	}
	p.pictures[c.picLocation] = pic
	return pic
}

// findBlip returns the first picture of the records in b, as a file, and
// the extension of the file. It looks into the containers and the
// OfficeArtFBSE records, whose picture can be in the stream WordDocument.
func (p *parser) findBlip(b []byte, depth int) ([]byte, string) {
	if depth > 8 {
		return nil, ""
	}
	for len(b) >= 8 {
		vi, typ, size := le.Uint16(b), le.Uint16(b[2:]), le.Uint32(b[4:])
		if uint64(size) > uint64(len(b)-8) {
			return nil, ""
		}
		body := b[8 : 8+size]
		switch {
		case vi&0xF == 0xF:
			if data, ext := p.findBlip(body, depth+1); data != nil {
				return data, ext
			}
		case typ == rtFBSE && len(body) >= 36:
			cbName := int(body[33])
			if 36+cbName < len(body) {
				if data, ext := blipData(body[36+cbName:], p.lim.MaxMemory); data != nil {
					return data, ext
				}
			}
			if fo := uint64(le.Uint32(body[28:])); fo < uint64(len(p.s.word)) {
				if data, ext := blipData(p.s.word[fo:], p.lim.MaxMemory); data != nil {
					return data, ext
				}
			}
		case typ >= rtBlipFirst && typ <= rtBlipLast:
			if data, ext := blipData(b, p.lim.MaxMemory); data != nil {
				return data, ext
			}
		}
		b = b[8+size:]
	}
	return nil, ""
}

// blipData returns the picture of the OfficeArtBlip record at the start of
// b as a file, and its extension: an EMF or a WMF file, decompressed, a
// JPEG, PNG or TIFF file, or a BMP file of a DIB. It returns nil for a
// picture that it does not read, such as a PICT.
func blipData(b []byte, maxMemory int64) ([]byte, string) {
	if len(b) < 8 {
		return nil, ""
	}
	inst, typ, size := le.Uint16(b)>>4, le.Uint16(b[2:]), le.Uint32(b[4:])
	if uint64(size) > uint64(len(b)-8) {
		return nil, ""
	}
	body := b[8 : 8+size]
	// an odd instance has a second identifier of 16 bytes
	uids := 16
	if inst&1 == 1 {
		uids = 32
	}
	switch typ {
	case rtBlipEMF, rtBlipWMF:
		ext := ".emf"
		if typ == rtBlipWMF {
			ext = ".wmf"
		}
		// the identifiers and an OfficeArtMetafileHeader of 34 bytes
		if len(body) < uids+34 {
			return nil, ""
		}
		mh := body[uids:]
		data := body[uids+34:]
		if mh[32] == 0xFE {
			return data, ext
		}
		n := int64(le.Uint32(mh))
		if n <= 0 || n > maxMemory {
			return nil, ""
		}
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, ""
		}
		out, err := raster.ReadAll(zr, n)
		if err != nil {
			return nil, ""
		}
		return out, ext
	case rtBlipJPEG, rtBlipJPEG2, rtBlipPNG, rtBlipTIFF:
		// the identifiers and a tag of one byte
		if len(body) < uids+1 {
			return nil, ""
		}
		ext := map[uint16]string{rtBlipJPEG: ".jpg", rtBlipJPEG2: ".jpg", rtBlipPNG: ".png", rtBlipTIFF: ".tif"}[typ]
		return body[uids+1:], ext
	case rtBlipDIB:
		if len(body) < uids+1+40 {
			return nil, ""
		}
		if bmp := bmpFile(body[uids+1:]); bmp != nil {
			return bmp, ".bmp"
		}
	}
	return nil, ""
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
	out := make([]byte, 14, 14+len(dib))
	out[0], out[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(out[2:], uint32(14+len(dib)))
	binary.LittleEndian.PutUint32(out[10:], uint32(14+hdr+4*colors))
	return append(out, dib...)
}

// The records of Office Drawing of the shapes that float on the page.
const (
	rtDgContainer  = 0xF002
	rtBStore       = 0xF001
	rtFSP          = 0xF00A
	rtFOPT         = 0xF00B
	rtTertiaryFOPT = 0xF122
	// propPib is the property of a shape that names its picture in the
	// store, from 1 (MS-ODRAW 2.3.23.2).
	propPib = 0x0104
	// propPosRelH and propPosRelV give the origin of the place of a shape:
	// the margin, the page, the column or the paragraph (MS-ODRAW 2.3.4).
	propPosRelH = 0x0390
	propPosRelV = 0x0392
	// propGroupBools holds fBehindDocument in its bit 5, which its bit 21
	// makes valid (MS-ODRAW 2.3.4.44).
	propGroupBools = 0x03BF
	// fspaSize is the size of an FSPA (MS-DOC 2.9.94).
	fspaSize = 26
)

// floating is a picture that floats on the page: its picture, its place
// in twips, and how the text wraps around it.
type floating struct {
	pic *picture
	// left and top are the place of the picture from the origin of bx
	// and by: 0 the margin, 1 the page, and 2 the column or the paragraph.
	left, top int
	bx, by    int
	// wr is how the text wraps: 1 above and below, 3 not at all, and the
	// others around it.
	wr     int
	behind bool
}

// shapeProps holds the properties of a shape of Office Drawing that the
// package reads, with -1 for a property that the shape does not set.
type shapeProps struct {
	pib, relH, relV int
	behind          bool
}

// readShapes reads the pictures of the shapes of the main document that
// float on the page, by the character position of their anchor: the FSPAs
// of the PlcSpaMom, and the pictures of the store of Office Drawing that
// the property pib of each shape names (MS-DOC 2.8.25 and 2.9.94).
func (p *parser) readShapes() map[uint32]floating {
	out := map[uint32]floating{}
	if p.s.word6 || len(p.s.fib.fcLcb) <= fcDggInfo {
		return out
	}
	dgg := p.s.part(fcDggInfo)
	plc := p.s.part(fcPlcSpaMom)
	if dgg == nil || len(plc) < 4+fspaSize {
		return out
	}
	var store [][]byte
	shapes := map[uint32]*shapeProps{}
	p.walkShapes(dgg, 0, &store, shapes)
	n := (len(plc) - 4) / (4 + fspaSize)
	for i := range min(n, p.lim.MaxObjects) {
		cp := u32(plc, 4*i)
		fspa := plc[4*(n+1)+fspaSize*i:]
		sp, ok := shapes[u32(fspa, 0)]
		if !ok || sp.pib < 1 || sp.pib > len(store) {
			continue
		}
		data, ext := blipFromEntry(store[sp.pib-1], p.s.word, p.lim.MaxMemory)
		if data == nil {
			continue
		}
		left, top := int(int32(u32(fspa, 4))), int(int32(u32(fspa, 8)))
		right, bottom := int(int32(u32(fspa, 12))), int(int32(u32(fspa, 16)))
		w, h := right-left, bottom-top
		if w <= 0 || h <= 0 || w > 31680 || h > 31680 {
			continue
		}
		flags := u16(fspa, 20)
		name := "media/" + strconv.Itoa(len(p.media)+1) + ext
		p.media = append(p.media, media{name: name, data: data})
		f := floating{
			pic:  &picture{name: name, width: float64(w) / 20, height: float64(h) / 20},
			left: left, top: top,
			bx: int(flags >> 1 & 3), by: int(flags >> 3 & 3), wr: int(flags >> 5 & 0xF),
			behind: flags&0x4000 != 0 || sp.behind,
		}
		// the origins of the shape, which Word 2000 and later write in the
		// shape and not in the FSPA: the margin, the page, the column and
		// the character, or the margin, the page, the paragraph and the
		// line
		if sp.relH >= 0 && sp.relH <= 3 {
			f.bx = [...]int{0, 1, 2, 2}[sp.relH]
		}
		if sp.relV >= 0 && sp.relV <= 3 {
			f.by = [...]int{0, 1, 2, 2}[sp.relV]
		}
		out[cp] = f
	}
	return out
}

// walkShapes reads the records of Office Drawing in b: the entries of the
// store of pictures into store, and the properties of each shape into
// shapes, by the id of the shape.
func (p *parser) walkShapes(b []byte, depth int, store *[][]byte, shapes map[uint32]*shapeProps) {
	if depth > 8 {
		return
	}
	var sp *shapeProps
	for len(b) >= 8 {
		if depth == 0 && len(b) >= 9 && le.Uint16(b[3:]) == rtDgContainer {
			// a drawing of the OfficeArtContent of Word starts with a byte,
			// dgglbl, before its OfficeArtDgContainer (MS-DOC 2.9.172)
			b = b[1:]
		}
		vi, typ, size := le.Uint16(b), le.Uint16(b[2:]), le.Uint32(b[4:])
		if uint64(size) > uint64(len(b)-8) {
			return
		}
		body := b[8 : 8+size]
		switch {
		case typ == rtBStore:
			for len(body) >= 8 && len(*store) < p.lim.MaxObjects {
				n := le.Uint32(body[4:])
				if uint64(n) > uint64(len(body)-8) {
					break
				}
				*store = append(*store, body[:8+n])
				body = body[8+n:]
			}
		case typ == rtFSP && len(body) >= 4:
			if len(shapes) < p.lim.MaxObjects {
				sp = &shapeProps{pib: -1, relH: -1, relV: -1}
				shapes[le.Uint32(body)] = sp
			}
		case (typ == rtFOPT || typ == rtTertiaryFOPT) && sp != nil:
			count := int(vi >> 4)
			for k := 0; k < count && 6*k+6 <= len(body); k++ {
				v := le.Uint32(body[6*k+2:])
				switch le.Uint16(body[6*k:]) & 0x3FFF {
				case propPib:
					sp.pib = int(v)
				case propPosRelH:
					sp.relH = int(v)
				case propPosRelV:
					sp.relV = int(v)
				case propGroupBools:
					if v&(1<<21) != 0 {
						sp.behind = v&(1<<5) != 0
					}
				}
			}
		case vi&0xF == 0xF:
			p.walkShapes(body, depth+1, store, shapes)
		}
		b = b[8+size:]
	}
}

// blipFromEntry returns the picture of an OfficeArtFBSE record of the
// store, which holds the picture or names its offset in the stream
// WordDocument.
func blipFromEntry(rec, word []byte, maxMemory int64) ([]byte, string) {
	if len(rec) < 8+36 || le.Uint16(rec[2:]) != rtFBSE {
		return nil, ""
	}
	body := rec[8:]
	cbName := int(body[33])
	if 36+cbName < len(body) {
		if data, ext := blipData(body[36+cbName:], maxMemory); data != nil {
			return data, ext
		}
	}
	if fo := uint64(le.Uint32(body[28:])); fo < uint64(len(word)) {
		return blipData(word[fo:], maxMemory)
	}
	return nil, ""
}
