// Package odraw reads the shapes and the pictures of Office Drawing
// (MS-ODRAW), which the binary Office formats share: the records, the
// properties of a shape, the store of pictures, the groups of shapes, and
// the figures of internal/drawingml that draw them.
//
// The format of a file gives what Office Drawing leaves to its client: the
// anchor of a shape on a slide, on a sheet or on a page, the colors of a
// scheme, and the text of a shape. msppt, msxls and msdoc read it, each
// with the anchors and the text of its own format (D51). The package draws
// the text of WordArt itself, from the properties of geometry text.
package odraw

import (
	"encoding/binary"
	"iter"
)

var le = binary.LittleEndian

// The types of the records of Office Drawing that the package reads
// (MS-ODRAW 2.2).
const (
	RecDggContainer    = 0xF000
	RecBStoreContainer = 0xF001
	RecDgContainer     = 0xF002
	RecSpgrContainer   = 0xF003
	RecSpContainer     = 0xF004
	RecFBSE            = 0xF007
	RecFSPGR           = 0xF009
	RecFSP             = 0xF00A
	RecFOPT            = 0xF00B
	RecClientTextbox   = 0xF00D
	RecChildAnchor     = 0xF00F
	RecClientAnchor    = 0xF010
	RecClientData      = 0xF011
	RecTertiaryFOPT    = 0xF122
	RecBlipEMF         = 0xF01A
	RecBlipWMF         = 0xF01B
	RecBlipPICT        = 0xF01C
	RecBlipJPEG        = 0xF01D
	RecBlipPNG         = 0xF01E
	RecBlipDIB         = 0xF01F
	RecBlipTIFF        = 0xF029
	RecBlipJPEGCMYK    = 0xF02A
)

// Header is the header of a record (MS-ODRAW 2.2.1). The binary Office
// formats give the same header to their own records.
type Header struct {
	Ver, Inst uint16
	Type      uint16
	Size      uint32
}

// ReadHeader reads the header of the record at the start of b.
func ReadHeader(b []byte) (Header, bool) {
	if len(b) < 8 {
		return Header{}, false
	}
	vi := le.Uint16(b)
	return Header{Ver: vi & 0xF, Inst: vi >> 4, Type: le.Uint16(b[2:]), Size: le.Uint32(b[4:])}, true
}

// Children iterates over the records in the body b of a container. It
// stops at a record that b cuts.
func Children(b []byte) iter.Seq2[Header, []byte] {
	return func(yield func(Header, []byte) bool) {
		for len(b) >= 8 {
			h, _ := ReadHeader(b)
			if uint64(h.Size) > uint64(len(b)-8) {
				return
			}
			if !yield(h, b[8:8+h.Size]) {
				return
			}
			b = b[8+h.Size:]
		}
	}
}

// Child returns the body of the first child of the type typ in the body b
// of a container, or nil.
func Child(b []byte, typ uint16) []byte {
	for h, body := range Children(b) {
		if h.Type == typ {
			return body
		}
	}
	return nil
}
