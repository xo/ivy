package msxls

import (
	"bytes"
	"compress/zlib"
	"strconv"

	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/internal/sheet"
)

// This file reads where the drawing objects of a sheet are: the charts,
// the pictures, the text boxes and the shapes. A worksheet holds them in
// the records MSODRAWING, as Office Drawing (MS-ODRAW), each followed by a
// record OBJ that gives its type (MS-XLS 2.4.181). The package shows a
// picture in PNG, JPEG, EMF or WMF, and a frame for each other object, with
// the text of a text box. It does not draw the shapes and the charts.

// The records of the drawings.
const (
	recMsoDrawingGroup = 0x00EB
	recMsoDrawing      = 0x00EC
	recObj             = 0x005D
	recTxo             = 0x01B6
)

// The records of Office Drawing that the package reads (MS-ODRAW 2.2).
const (
	odrawBStore       = 0xF001
	odrawFSP          = 0xF00A
	odrawOPT          = 0xF00B
	odrawFBSE         = 0xF007
	odrawClientAnchor = 0xF010
	odrawBlipEMF      = 0xF01A
	odrawBlipWMF      = 0xF01B
	odrawBlipJPEG     = 0xF01D
	odrawBlipPNG      = 0xF01E
	odrawBlipJPEGCMYK = 0xF02A
)

// The types of the objects of the record OBJ, in its structure FtCmo.
const (
	objGroup    = 0x00
	objChart    = 0x05
	objText     = 0x06
	objPicture  = 0x08
	objComboBox = 0x14
	objNote     = 0x19
)

// maxDrawing bounds the bytes of the drawing of one sheet and of the
// pictures of the workbook.
const maxDrawing = 64 << 20

// shape is a shape of the drawing of a sheet: where its anchor ends in the
// stream of the drawing, its anchor, and the index of its picture.
type shape struct {
	end    int
	anchor [8]int
	pib    int
	used   bool
}

// drawObj is a record OBJ of a sheet: the offset of the stream of the
// drawing when it comes, its type, and the text of its record TXO.
type drawObj struct {
	at   int
	ot   int
	text string
}

// odrawRecords iterates over the records of Office Drawing in b, from the
// offset base of b in its stream, and over the records in their
// containers. f gets the type, the instance, the offset after the header in
// the stream, and the data of each record.
func odrawRecords(b []byte, base int, f func(typ, inst uint16, at int, data []byte), depth int) {
	for off := 0; off+8 <= len(b) && depth < 16; {
		vi := le.Uint16(b[off:])
		typ := le.Uint16(b[off+2:])
		n := int(le.Uint32(b[off+4:]))
		at := off + 8
		end := at + n
		if n < 0 || end > len(b) || end < at {
			end = len(b)
		}
		if vi&0x0F == 0x0F {
			// a container: its records follow its header
			odrawRecords(b[at:end], base+at, f, depth+1)
		} else {
			f(typ, vi>>4, base+at, b[at:end])
		}
		off = end
	}
}

// blip is a picture of the store of a workbook: the file and its
// extension, such as ".png".
type blip struct {
	data []byte
	ext  string
}

// readBlips reads the pictures of the records MSODRAWINGGROUP, by their
// index from 1. A picture in a format that the package does not read has
// no data. A metafile stays within limit bytes.
func readBlips(b []byte, limit int64) []blip {
	var blips []blip
	odrawRecords(b, 0, func(typ, _ uint16, _ int, data []byte) {
		if typ != odrawFBSE {
			return
		}
		// OfficeArtFBSE (MS-ODRAW 2.2.32): 36 bytes, the name, and the
		// picture
		var bl blip
		if len(data) >= 36 {
			name := int(data[33])
			if rest := data[min(36+name, len(data)):]; len(rest) >= 8 {
				bl = blipData(le.Uint16(rest[2:]), le.Uint16(rest)>>4, rest[8:min(8+int(le.Uint32(rest[4:])), len(rest))], limit)
			}
		}
		if len(blips) < 4096 {
			blips = append(blips, bl)
		}
	}, 0)
	return blips
}

// blipData returns the file of a picture record of the type typ and the
// instance inst, without its header, or no data for a format that the
// package does not read, such as a DIB. The file follows one id of 16
// bytes, or two when the instance is odd. A JPEG or PNG image follows a
// tag of 1 byte (MS-ODRAW 2.2.27, 2.2.28). An EMF or WMF file follows an
// OfficeArtMetafileHeader of 34 bytes, compressed with DEFLATE unless the
// header says otherwise (MS-ODRAW 2.2.24, 2.2.25, 2.2.31).
func blipData(typ, inst uint16, b []byte, limit int64) blip {
	uids := 16
	if inst&1 == 1 {
		uids = 32
	}
	switch typ {
	case odrawBlipJPEG, odrawBlipJPEGCMYK, odrawBlipPNG:
		if len(b) <= uids+1 {
			return blip{}
		}
		data := b[uids+1:]
		if bytes.HasPrefix(data, []byte("\x89PNG")) {
			return blip{data: data, ext: ".png"}
		}
		return blip{data: data, ext: ".jpg"}
	case odrawBlipEMF, odrawBlipWMF:
		if len(b) < uids+34 {
			return blip{}
		}
		ext := ".emf"
		if typ == odrawBlipWMF {
			ext = ".wmf"
		}
		mh, data := b[uids:], b[uids+34:]
		if mh[32] == 0xFE {
			return blip{data: data, ext: ext}
		}
		size := int64(le.Uint32(mh))
		if size <= 0 || size > limit {
			return blip{}
		}
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return blip{}
		}
		out, err := raster.ReadAll(zr, size)
		if err != nil {
			return blip{}
		}
		return blip{data: out, ext: ext}
	}
	return blip{}
}

// readShapes reads the shapes of the drawing of a sheet: each shape with a
// cell anchor, the structure OfficeArtClientAnchorSheet of MS-XLS, with the
// index of its picture.
func readShapes(b []byte) []shape {
	var shapes []shape
	pib := 0
	odrawRecords(b, 0, func(typ, inst uint16, at int, data []byte) {
		switch typ {
		case odrawFSP:
			pib = 0
		case odrawOPT:
			// the properties: inst of them, of 6 bytes each, and the
			// property pib (0x0104) is the index of the picture
			for i := 0; i < int(inst) && 6*i+6 <= len(data); i++ {
				if le.Uint16(data[6*i:])&0x3FFF == 0x0104 {
					pib = int(le.Uint32(data[6*i+2:]))
				}
			}
		case odrawClientAnchor:
			if len(data) < 18 || len(shapes) >= 4096 {
				return
			}
			s := shape{end: at + len(data), pib: pib}
			for i := range 8 {
				s.anchor[i] = int(le.Uint16(data[2+2*i:]))
			}
			shapes = append(shapes, s)
		}
	}, 0)
	return shapes
}

// images converts the drawing objects of raw into images of sh. blips are
// the pictures of the workbook, and files gets the pictures that the
// images show.
func images(sh *sheet.Sheet, raw *rawSheet, blips []blip, files map[string][]byte) {
	if len(raw.objs) == 0 {
		return
	}
	shapes := readShapes(raw.drawing)
	for _, o := range raw.objs {
		// the object takes the last shape before it that no other object
		// took: the shapes in a group have no cell anchor
		var s *shape
		for i := len(shapes) - 1; i >= 0; i-- {
			if shapes[i].end <= o.at {
				if !shapes[i].used {
					s = &shapes[i]
				}
				break
			}
		}
		if s == nil {
			continue
		}
		s.used = true
		img, ok := anchorImage(sh, s.anchor)
		if !ok {
			continue
		}
		switch o.ot {
		case objNote, objComboBox:
			// a comment shows only when a person points at its cell, and
			// a combo box is the button of a filter
			continue
		case objChart:
			img.Label = "Chart"
		case objPicture:
			if s.pib > 0 && s.pib <= len(blips) && blips[s.pib-1].data != nil {
				bl := blips[s.pib-1]
				img.Path = "media/image" + strconv.Itoa(s.pib) + bl.ext
				files[img.Path] = bl.data
			} else {
				img.Label = "Picture"
			}
		default:
			img.Label = o.text
		}
		sh.Images = append(sh.Images, img)
	}
}

// anchorImage returns the image of a cell anchor: the column and the
// offset in 1/1024 of its width, and the row and the offset in 1/256 of
// its height, of the top left corner and of the bottom right corner.
func anchorImage(sh *sheet.Sheet, a [8]int) (sheet.Image, bool) {
	col0, dx0, row0, dy0 := a[0], a[1], a[2], a[3]
	col1, dx1, row1, dy1 := a[4], a[5], a[6], a[7]
	if col0 > maxCol || col1 > maxCol || row0 > maxRow || row1 > maxRow || col1 < col0 || row1 < row0 || row1-row0 > 4096 {
		return sheet.Image{}, false
	}
	img := sheet.Image{Row: row0, Col: col0}
	img.DX = float64(min(dx0, 1024)) / 1024 * sh.ColWidth(col0)
	img.DY = float64(min(dy0, 256)) / 256 * sh.RowHeight(row0)
	x1 := float64(min(dx1, 1024)) / 1024 * sh.ColWidth(col1)
	y1 := float64(min(dy1, 256)) / 256 * sh.RowHeight(row1)
	for c := col0; c < col1; c++ {
		x1 += sh.ColWidth(c)
	}
	for r := row0; r < row1; r++ {
		y1 += sh.RowHeight(r)
	}
	img.Width, img.Height = x1-img.DX, y1-img.DY
	if !(img.Width >= 1) || !(img.Height >= 1) || img.Width > 14400 || img.Height > 14400 {
		return sheet.Image{}, false
	}
	return img, true
}
