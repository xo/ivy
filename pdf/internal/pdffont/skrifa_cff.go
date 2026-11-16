// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports no file of hayro-font, and it does not port skrifa. It
// fills the gaps of go-text for the CFF fonts that hayro-font reads with
// skrifa in src/program/mod.rs.
//
// go-text parses a bare CFF font with cff.Parse of its package font/cff, and
// it gives the charstrings, the outline of a glyph (CFF.LoadGlyph) and the
// name of a glyph (CFF.GlyphName). This file uses it for that work. go-text
// v0.3.5 has three gaps:
//
//  1. cff.Parse skips the built-in encoding ("skip encoding" in
//     font/cff/parser.go). hayro-font maps the codes of an embedded font
//     with the encoding of skrifa (builtin_encoding_glyph).
//  2. The charset and the FDSelect of a CFF are unexported fields. Nothing
//     maps a CID to a glyph (glyph_by_cid), or tells whether the font is a
//     CIDFont (is_cid_keyed).
//  3. The FontMatrix of the Top DICT is a no-op in the parser. A bare CFF
//     font in a PDF file can use a matrix other than 0.001.
//
// This file reads the Top DICT, the charset and the encoding for these
// three items, about 300 lines. It leaves the charstrings to go-text.

package pdffont

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/go-text/typesetting/font/cff"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy"
)

// The kinds of the built-in encoding of a CFF font.
const (
	cffEncodingStandard = iota
	cffEncodingExpert
	cffEncodingCustom
)

// maxCFFDictOperands bounds the operands of one operator of a DICT.
const maxCFFDictOperands = 48

// cffProgram is a CFF font: the charstrings that go-text reads, and the data of
// the Top DICT that go-text does not give.
type cffProgram struct {
	ot        *cff.CFF
	numGlyphs int
	isCID     bool
	matrix    kurbo.Affine

	// charset holds the SID of each glyph, or its CID in a CIDFont. It is
	// nil for the predefined charsets Expert and ExpertSubset.
	charset []uint16

	encodingKind int
	// codes maps each code of a custom encoding to a glyph, or to -1.
	codes [256]int32

	namesOnce sync.Once
	names     map[string]GlyphID
	cidsOnce  sync.Once
	cids      map[uint16]GlyphID
}

// newCFFProgram reads a CFF font.
func newCFFProgram(data []byte) (*cffProgram, error) {
	var ot *cff.CFF
	if err := safely(func() error {
		var err error
		ot, err = cff.Parse(data)
		return goTextError(err)
	}); err != nil {
		return nil, fmt.Errorf("reading CFF font: %w", err)
	}
	f := &cffProgram{
		ot:        ot,
		numGlyphs: len(ot.Charstrings),
		matrix:    kurbo.NewAffine([6]float64{0.001, 0, 0, 0.001, 0, 0}),
	}
	for i := range f.codes {
		f.codes[i] = -1
	}
	if err := f.readTopDict(data); err != nil {
		return nil, err
	}
	return f, nil
}

// errCFF is the error of a malformed CFF font.
var errCFF = fmt.Errorf("malformed CFF font: %w", ivy.ErrFormat)

// readTopDict reads the font matrix, the charset and the encoding from the
// Top DICT.
func (f *cffProgram) readTopDict(data []byte) error {
	if len(data) < 4 {
		return errCFF
	}
	pos := int(data[2])
	// The Name INDEX, the Top DICT INDEX and the String INDEX follow the
	// header.
	_, pos, err := cffIndex(data, pos)
	if err != nil {
		return err
	}
	dicts, _, err := cffIndex(data, pos)
	if err != nil {
		return err
	}
	if len(dicts) == 0 {
		return errCFF
	}
	charsetOffset, encodingOffset := 0, 0
	err = cffDict(dicts[0], func(op int, args []float64) {
		switch op {
		case 15:
			if len(args) > 0 {
				charsetOffset = int(args[len(args)-1])
			}
		case 16:
			if len(args) > 0 {
				encodingOffset = int(args[len(args)-1])
			}
		case 1200 + 30:
			f.isCID = true
		case 1200 + 7:
			if len(args) == 6 {
				f.matrix = kurbo.NewAffine([6]float64(args))
			}
		}
	})
	if err != nil {
		return err
	}
	f.readCharset(data, charsetOffset)
	if !f.isCID {
		f.readEncoding(data, encodingOffset)
	}
	return nil
}

// readCharset reads the charset at offset. A malformed charset leaves the
// glyphs that it does not reach at SID 0.
func (f *cffProgram) readCharset(data []byte, offset int) {
	switch offset {
	case 0:
		// ISOAdobe: the SID of each of the first 229 glyphs is its index.
		f.charset = make([]uint16, f.numGlyphs)
		for i := range f.charset {
			if i < 229 {
				f.charset[i] = uint16(i)
			}
		}
		return
	case 1, 2:
		// Port: the tables of the predefined charsets Expert and
		// ExpertSubset are not in this package. Only the supplements of
		// a custom encoding need them.
		return
	}
	f.charset = make([]uint16, f.numGlyphs)
	if offset < 0 || offset >= len(data) {
		return
	}
	format := data[offset]
	pos := offset + 1
	gid := 1
	switch format {
	case 0:
		for ; gid < f.numGlyphs && pos+2 <= len(data); gid++ {
			f.charset[gid] = binary.BigEndian.Uint16(data[pos:])
			pos += 2
		}
	case 1, 2:
		size := 3
		if format == 2 {
			size = 4
		}
		for gid < f.numGlyphs && pos+size <= len(data) {
			first := int(binary.BigEndian.Uint16(data[pos:]))
			left := int(data[pos+2])
			if format == 2 {
				left = int(binary.BigEndian.Uint16(data[pos+2:]))
			}
			pos += size
			for j := 0; j <= left && gid < f.numGlyphs; j++ {
				f.charset[gid] = uint16(first + j)
				gid++
			}
		}
	}
}

// readEncoding reads the built-in encoding at offset.
func (f *cffProgram) readEncoding(data []byte, offset int) {
	switch offset {
	case 0:
		f.encodingKind = cffEncodingStandard
		return
	case 1:
		f.encodingKind = cffEncodingExpert
		return
	}
	f.encodingKind = cffEncodingCustom
	if offset < 0 || offset >= len(data) {
		return
	}
	format := data[offset]
	pos := offset + 1
	if pos >= len(data) {
		return
	}
	switch format & 0x7f {
	case 0:
		n := int(data[pos])
		pos++
		for i := 0; i < n && pos < len(data); i++ {
			if i+1 < f.numGlyphs {
				f.codes[data[pos]] = int32(i + 1)
			}
			pos++
		}
	case 1:
		n := int(data[pos])
		pos++
		gid := 1
		for i := 0; i < n && pos+2 <= len(data); i++ {
			first := int(data[pos])
			left := int(data[pos+1])
			pos += 2
			for j := 0; j <= left && gid < f.numGlyphs; j++ {
				if first+j < 256 {
					f.codes[first+j] = int32(gid)
				}
				gid++
			}
		}
	default:
		return
	}
	if format&0x80 == 0 || pos >= len(data) {
		return
	}
	// The supplements map a code to a glyph through its SID.
	n := int(data[pos])
	pos++
	for i := 0; i < n && pos+3 <= len(data); i++ {
		code := data[pos]
		sid := binary.BigEndian.Uint16(data[pos+1:])
		pos += 3
		for gid, s := range f.charset {
			if s == sid {
				f.codes[code] = int32(gid)
				break
			}
		}
	}
}

// glyphName returns the name of a glyph of a font that is not a CIDFont.
func (f *cffProgram) glyphName(gid GlyphID) (string, bool) {
	if f.isCID || int(gid) >= f.numGlyphs {
		return "", false
	}
	name := f.ot.GlyphName(ot.GID(gid))
	return name, name != ""
}

// glyphByName returns the first glyph with the name.
func (f *cffProgram) glyphByName(name string) (GlyphID, bool) {
	f.namesOnce.Do(func() {
		f.names = make(map[string]GlyphID)
		if f.isCID {
			return
		}
		for gid := range f.numGlyphs {
			if n := f.ot.GlyphName(ot.GID(gid)); n != "" {
				if _, ok := f.names[n]; !ok {
					f.names[n] = GlyphID(gid)
				}
			}
		}
	})
	gid, ok := f.names[name]
	return gid, ok
}

// glyphByCID returns the glyph of a CID in a CIDFont.
func (f *cffProgram) glyphByCID(cid uint32) (GlyphID, bool) {
	if !f.isCID || cid > math.MaxUint16 {
		return 0, false
	}
	f.cidsOnce.Do(func() {
		f.cids = make(map[uint16]GlyphID, len(f.charset))
		for gid, c := range f.charset {
			if _, ok := f.cids[c]; !ok {
				f.cids[c] = GlyphID(gid)
			}
		}
	})
	gid, ok := f.cids[uint16(cid)]
	return gid, ok
}

// encodingGlyph returns the glyph of a code in the built-in encoding.
//
// Port: skrifa maps a code of the predefined encodings to a SID, and the SID
// to a glyph through the charset. This function maps the code to the name of
// the glyph in StandardEncoding or MacExpertEncoding, and finds the glyph by
// its name. For the standard encoding the result is the same. The expert
// encoding of CFF and MacExpertEncoding differ for some codes.
func (f *cffProgram) encodingGlyph(code uint8) (GlyphID, bool) {
	if f.isCID {
		return 0, false
	}
	switch f.encodingKind {
	case cffEncodingStandard:
		if name, ok := standardGet(code); ok {
			return f.glyphByName(name)
		}
		return 0, false
	case cffEncodingExpert:
		if name, ok := macExpertGet(code); ok {
			return f.glyphByName(name)
		}
		return 0, false
	}
	if g := f.codes[code]; g >= 0 {
		return GlyphID(g), true
	}
	return 0, false
}

// outline draws a glyph into the pen, scaled by the font matrix to
// UnitsPerEm.
func (f *cffProgram) outline(gid GlyphID, pen *outlinePath) {
	if int(gid) >= f.numGlyphs {
		return
	}
	var segs []ot.Segment
	_ = safely(func() error {
		var err error
		segs, _, err = f.ot.LoadGlyph(tables.GlyphID(gid))
		return goTextError(err)
	})
	m := kurbo.AffineScale(float64(UnitsPerEm)).Mul(f.matrix)
	drawSegments(segs, m, pen)
}

// cffIndex reads the INDEX at pos. It returns the items and the position
// after the INDEX.
func cffIndex(data []byte, pos int) ([][]byte, int, error) {
	if pos < 0 || pos+2 > len(data) {
		return nil, 0, errCFF
	}
	count := int(binary.BigEndian.Uint16(data[pos:]))
	pos += 2
	if count == 0 {
		return nil, pos, nil
	}
	if pos >= len(data) {
		return nil, 0, errCFF
	}
	offSize := int(data[pos])
	pos++
	if offSize < 1 || offSize > 4 {
		return nil, 0, errCFF
	}
	offsetsEnd := pos + (count+1)*offSize
	if offsetsEnd > len(data) {
		return nil, 0, errCFF
	}
	readOffset := func(i int) int {
		v := 0
		for _, b := range data[pos+i*offSize : pos+(i+1)*offSize] {
			v = v<<8 | int(b)
		}
		return v
	}
	base := offsetsEnd - 1
	items := make([][]byte, count)
	prev := readOffset(0)
	for i := range count {
		next := readOffset(i + 1)
		if prev < 1 || next < prev || base+next > len(data) {
			return nil, 0, errCFF
		}
		items[i] = data[base+prev : base+next]
		prev = next
	}
	return items, base + prev, nil
}

// cffDict reads a DICT and calls fn for each operator. An escaped operator
// is 1200 plus its second byte.
func cffDict(data []byte, fn func(op int, args []float64)) error {
	var args []float64
	push := func(v float64) {
		if len(args) < maxCFFDictOperands {
			args = append(args, v)
		}
	}
	for i := 0; i < len(data); {
		b := int(data[i])
		switch {
		case b <= 21:
			op := b
			i++
			if b == 12 {
				if i >= len(data) {
					return errCFF
				}
				op = 1200 + int(data[i])
				i++
			}
			fn(op, args)
			args = args[:0]
		case b == 28:
			if i+3 > len(data) {
				return errCFF
			}
			push(float64(int16(binary.BigEndian.Uint16(data[i+1:]))))
			i += 3
		case b == 29:
			if i+5 > len(data) {
				return errCFF
			}
			push(float64(int32(binary.BigEndian.Uint32(data[i+1:]))))
			i += 5
		case b == 30:
			v, n := cffReal(data[i+1:])
			push(v)
			i += 1 + n
		case b >= 32 && b <= 246:
			push(float64(b - 139))
			i++
		case b >= 247 && b <= 250:
			if i+2 > len(data) {
				return errCFF
			}
			push(float64((b-247)*256 + int(data[i+1]) + 108))
			i += 2
		case b >= 251 && b <= 254:
			if i+2 > len(data) {
				return errCFF
			}
			push(float64(-(b-251)*256 - int(data[i+1]) - 108))
			i += 2
		default:
			// A reserved byte.
			i++
		}
	}
	return nil
}

// cffReal reads a real number of a DICT. It returns the number and the count
// of the bytes that it read.
func cffReal(data []byte) (float64, int) {
	const digits = "0123456789.EE?-?"
	var s []byte
	for i, b := range data {
		for _, nib := range [2]byte{b >> 4, b & 0xf} {
			switch nib {
			case 0xf:
				v, err := strconv.ParseFloat(string(s), 64)
				if err != nil {
					v = 0
				}
				return v, i + 1
			case 0xc:
				s = append(s, 'E', '-')
			case 0xd:
			default:
				s = append(s, digits[nib])
			}
		}
		if len(s) > 64 {
			break
		}
	}
	return 0, len(data)
}

// drawSegments draws the segments of go-text into the pen through m. It
// closes each contour before the next one starts, and the last one at the
// end, as skrifa does.
func drawSegments(segs []ot.Segment, m kurbo.Affine, pen *outlinePath) {
	p := func(s ot.SegmentPoint) (float32, float32) {
		q := m.MulPoint(kurbo.NewPoint(float64(s.X), float64(s.Y)))
		return float32(q.X), float32(q.Y)
	}
	open := false
	for _, s := range segs {
		switch s.Op {
		case ot.SegmentOpMoveTo:
			if open {
				pen.closePath()
			}
			pen.moveTo(p(s.Args[0]))
			open = true
		case ot.SegmentOpLineTo:
			pen.lineTo(p(s.Args[0]))
		case ot.SegmentOpQuadTo:
			x1, y1 := p(s.Args[0])
			x, y := p(s.Args[1])
			pen.quadTo(x1, y1, x, y)
		case ot.SegmentOpCubeTo:
			x1, y1 := p(s.Args[0])
			x2, y2 := p(s.Args[1])
			x, y := p(s.Args[2])
			pen.curveTo(x1, y1, x2, y2, x, y)
		}
	}
	if open {
		pen.closePath()
	}
}

// goTextError wraps an error of go-text in ivy.ErrFormat. It returns nil
// for nil.
func goTextError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("go-text: %w: %w", ivy.ErrFormat, err)
}

// safely runs fn and turns a panic of a reader of go-text into an error, so
// that a malformed font cannot crash the program.
func safely(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("reading font: %w: %v", ivy.ErrFormat, r)
		}
	}()
	return fn()
}
