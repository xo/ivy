package xps

import (
	"bytes"
	"encoding/binary"
	"path"
	"strings"
	"sync"

	gofont "github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype"

	"github.com/xo/ivy/internal/font"
	"github.com/xo/okage"
)

// sidewaysTurn turns a glyph of a sideways run 90 degrees counterclockwise
// on the page, so that the top of the glyph points against the direction of
// the run (ECMA-388, the IsSideways attribute of Glyphs).
var sidewaysTurn = okage.TransformFromRow(0, -1, 1, 0, 0, 0)

// fallbackOrigin is the height of the vertical origin above the baseline, in
// ems, for a font whose vertical metrics ivy cannot read. It is a common
// ascender.
const fallbackOrigin = 0.88

// vertical gives the vertical metrics of a font for a sideways glyph run:
// the vertical advance of each glyph, and its vertical origin. The methods
// are safe for concurrent use.
type vertical struct {
	mu sync.Mutex
	// face is the font of go-text, or nil if the font cannot give its
	// vertical metrics.
	face *gofont.Face
	upem float32
}

// metrics returns the vertical advance of the glyph gid of f, and the
// vertical origin of the glyph: half of its horizontal advance, and the
// height above the baseline. Each value is in ems.
func (v *vertical) metrics(f *font.Face, gid font.GID) (advance, ox, oy float32) {
	ox = f.Advance(gid) / 2
	if v == nil || v.face == nil {
		return 1, ox, fallbackOrigin
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	// go-text can panic on a malformed font (D5)
	defer func() {
		if recover() != nil {
			advance, oy = 1, fallbackOrigin
		}
	}()
	// go-text gives the vertical advance with y up, so it is negative
	advance = -v.face.VerticalAdvance(opentype.GID(gid)) / v.upem
	_, y := v.face.GlyphVOrigin(opentype.GID(gid))
	if advance <= 0 {
		advance = 1
	}
	return advance, ox, y / v.upem
}

// vertical returns the vertical metrics of the font part name, which it
// reads once. It reads the font again, so that a document without a
// sideways run does not keep a second copy of each font.
func (d *Document) vertical(name string) *vertical {
	d.mu.Lock()
	defer d.mu.Unlock()
	if v, ok := d.verticals[name]; ok {
		return v
	}
	v := &vertical{}
	d.verticals[name] = v
	data, err := d.pkg.read(name)
	if err == nil && strings.EqualFold(path.Ext(name), ".odttf") {
		err = deobfuscate(data, name)
	}
	if err != nil || !tablesFit(data) {
		return v
	}
	v.face, v.upem = parseFace(data)
	return v
}

// parseFace parses the first font of data with go-text. It returns nil if
// data is not a font.
func parseFace(data []byte) (face *gofont.Face, upem float32) {
	// go-text can panic on a malformed font (D5)
	defer func() {
		if recover() != nil {
			face, upem = nil, 0
		}
	}()
	faces, err := gofont.ParseTTC(bytes.NewReader(data))
	if err != nil || len(faces) == 0 {
		return nil, 0
	}
	upem = float32(faces[0].Upem())
	if upem <= 0 {
		upem = 1000
	}
	return faces[0], upem
}

// tablesFit reports whether each table of each font of data ends within
// data. go-text reserves the length that the directory of a font gives for a
// table before it reads the table, so a table of a declared length of 4 GB
// in a small file makes it reserve 4 GB. font.Load cuts such a table, and
// the vertical metrics are left out.
func tablesFit(data []byte) bool {
	size := uint64(len(data))
	if size < 12 {
		return false
	}
	offsets := []uint64{0}
	if string(data[:4]) == "ttcf" {
		n := uint64(binary.BigEndian.Uint32(data[8:]))
		if 12+4*n > size {
			return false
		}
		offsets = offsets[:0]
		for i := range n {
			offsets = append(offsets, uint64(binary.BigEndian.Uint32(data[12+4*i:])))
		}
	}
	for _, off := range offsets {
		if off+12 > size {
			return false
		}
		n := uint64(binary.BigEndian.Uint16(data[off+4:]))
		if off+12+16*n > size {
			return false
		}
		for i := range n {
			rec := off + 12 + 16*i
			start := uint64(binary.BigEndian.Uint32(data[rec+8:]))
			length := uint64(binary.BigEndian.Uint32(data[rec+12:]))
			if start+length > size {
				return false
			}
		}
	}
	return true
}
