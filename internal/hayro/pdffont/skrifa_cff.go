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
// v0.3.5 has four gaps:
//
//  1. cff.Parse skips the built-in encoding ("skip encoding" in
//     font/cff/parser.go). hayro-font maps the codes of an embedded font
//     with the encoding of skrifa (builtin_encoding_glyph).
//  2. The charset and the FDSelect of a CFF are unexported fields. Nothing
//     maps a CID to a glyph (glyph_by_cid), or tells whether the font is a
//     CIDFont (is_cid_keyed).
//  3. The FontMatrix of the Top DICT and of the font dictionaries of a
//     CIDFont is a no-op in the parser. A bare CFF font in a PDF file can
//     use a matrix other than 0.001, and a CIDFont often has the identity
//     matrix in its Top DICT and the matrix 0.001 in each font dictionary.
//  4. The charstring handler of CFF.LoadGlyph ends a glyph at endchar, and
//     ignores the accented glyph that endchar with four operands builds
//     from two glyphs of StandardEncoding, the seac of Type 1.
//
// This file reads the Top DICT, the charset, the encoding, the font
// dictionaries, the FDSelect and the subroutines for these four items. It
// leaves the charstrings to go-text. For a glyph of seac, it finds the two
// glyphs with the interpreter of go-text, and draws them with
// CFF.LoadGlyph.

package pdffont

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/xo/okage/kurbo"
	"github.com/xo/okage/typesetting/font/cff"
	ps "github.com/xo/okage/typesetting/font/cff/interpreter"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"

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

// maxCFFFontDicts bounds the font dictionaries of a CIDFont. FDSelect gives
// the index of a font dictionary in one byte.
const maxCFFFontDicts = 256

// cffDefaultMatrix is the FontMatrix of a CFF font that gives none.
var cffDefaultMatrix = kurbo.NewAffine([6]float64{0.001, 0, 0, 0.001, 0, 0})

// cffProgram is a CFF font: the charstrings that go-text reads, and the data of
// the Top DICT that go-text does not give.
type cffProgram struct {
	ot        *cff.CFF
	numGlyphs int
	isCID     bool

	// matrix is the FontMatrix of the Top DICT, or cffDefaultMatrix.
	// hasMatrix is false if the Top DICT gives no FontMatrix.
	matrix    kurbo.Affine
	hasMatrix bool
	// fdMatrices holds the matrix of each font dictionary of a CIDFont,
	// with the matrix of the Top DICT, and fdHasMatrix whether the Top DICT
	// or the font dictionary gives it. fdSelect holds the index of the font
	// dictionary of each glyph. They are nil for a font that is not a
	// CIDFont.
	fdMatrices  []kurbo.Affine
	fdHasMatrix []bool
	fdSelect    []uint8

	// globalSubrs and localSubrs are the subroutines of a font that is not
	// a CIDFont, for the glyphs of seac.
	globalSubrs, localSubrs [][]byte

	// charset holds the SID of each glyph, or its CID in a CIDFont.
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
		matrix:    cffDefaultMatrix,
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

// readTopDict reads the font matrix, the charset, the encoding, the font
// dictionaries, the FDSelect and the subroutines from the Top DICT.
func (f *cffProgram) readTopDict(data []byte) error {
	if len(data) < 4 {
		return errCFF
	}
	pos := int(data[2])
	// The Name INDEX, the Top DICT INDEX, the String INDEX and the Global
	// Subr INDEX follow the header.
	_, pos, err := cffIndex(data, pos)
	if err != nil {
		return err
	}
	dicts, pos, err := cffIndex(data, pos)
	if err != nil {
		return err
	}
	if len(dicts) == 0 {
		return errCFF
	}
	charsetOffset, encodingOffset, fdArrayOffset, fdSelectOffset := 0, 0, -1, -1
	var private []float64
	err = cffDict(dicts[0], func(op int, args []float64) {
		last := -1
		if len(args) > 0 {
			last = int(args[len(args)-1])
		}
		switch op {
		case 15:
			charsetOffset = max(last, 0)
		case 16:
			encodingOffset = max(last, 0)
		case 18:
			private = append(private[:0], args...)
		case 1200 + 30:
			f.isCID = true
		case 1200 + 36:
			fdArrayOffset = last
		case 1200 + 37:
			fdSelectOffset = last
		case 1200 + 7:
			if len(args) == 6 {
				f.matrix, f.hasMatrix = kurbo.NewAffine([6]float64(args)), true
			}
		}
	})
	if err != nil {
		return err
	}
	f.readCharset(data, charsetOffset)
	if f.isCID {
		f.readFontDicts(data, fdArrayOffset, fdSelectOffset)
		return nil
	}
	f.readEncoding(data, encodingOffset)
	// The subroutines are only for the glyphs of seac. A font without them
	// draws its glyphs.
	if _, pos, err = cffIndex(data, pos); err == nil {
		f.globalSubrs, _, _ = cffIndex(data, pos)
	}
	f.localSubrs = cffLocalSubrs(data, private)
	return nil
}

// cffLocalSubrs returns the local subroutines of the Private DICT that the
// operands private of the operator Private give, or nil.
func cffLocalSubrs(data []byte, private []float64) [][]byte {
	if len(private) != 2 {
		return nil
	}
	size, offset := int(private[0]), int(private[1])
	if size <= 0 || offset < 0 || offset > len(data) || size > len(data)-offset {
		return nil
	}
	subrs := -1
	_ = cffDict(data[offset:offset+size], func(op int, args []float64) {
		if op == 19 && len(args) > 0 {
			subrs = int(args[len(args)-1])
		}
	})
	if subrs <= 0 || subrs > len(data)-offset {
		return nil
	}
	items, _, _ := cffIndex(data, offset+subrs)
	return items
}

// readFontDicts reads the FontMatrix of each font dictionary of the
// FDArray at fdArrayOffset, and the FDSelect at fdSelectOffset, of a
// CIDFont. A malformed FDArray or FDSelect leaves the matrix of the Top
// DICT to each glyph.
//
// Port: skrifa combines the two matrices as FreeType does. The matrix of
// the Top DICT applies after that of the font dictionary, without its
// translation. Without a matrix in the Top DICT, the matrix of the font
// dictionary applies alone.
func (f *cffProgram) readFontDicts(data []byte, fdArrayOffset, fdSelectOffset int) {
	if fdArrayOffset <= 0 || fdSelectOffset <= 0 || fdSelectOffset >= len(data) {
		return
	}
	dicts, _, err := cffIndex(data, fdArrayOffset)
	if err != nil || len(dicts) == 0 || len(dicts) > maxCFFFontDicts {
		return
	}
	top := f.matrix
	top[4], top[5] = 0, 0
	matrices := make([]kurbo.Affine, len(dicts))
	has := make([]bool, len(dicts))
	for i, d := range dicts {
		matrices[i], has[i] = f.matrix, f.hasMatrix
		_ = cffDict(d, func(op int, args []float64) {
			if op != 1200+7 || len(args) != 6 {
				return
			}
			m := kurbo.NewAffine([6]float64(args))
			if f.hasMatrix {
				m = top.Mul(m)
			}
			matrices[i], has[i] = m, true
		})
	}
	selection, ok := cffFDSelect(data[fdSelectOffset:], f.numGlyphs, len(dicts))
	if !ok {
		return
	}
	f.fdMatrices, f.fdHasMatrix, f.fdSelect = matrices, has, selection
}

// cffFDSelect reads an FDSelect of the format 0 or 3. It returns the index
// of the font dictionary of each of numGlyphs glyphs, and false if the
// FDSelect is malformed or names a font dictionary past count.
func cffFDSelect(data []byte, numGlyphs, count int) ([]uint8, bool) {
	if len(data) < 1 {
		return nil, false
	}
	selection := make([]uint8, numGlyphs)
	switch data[0] {
	case 0:
		if len(data) < 1+numGlyphs {
			return nil, false
		}
		copy(selection, data[1:1+numGlyphs])
	case 3:
		if len(data) < 3 {
			return nil, false
		}
		n := int(binary.BigEndian.Uint16(data[1:]))
		if len(data) < 3+3*n+2 {
			return nil, false
		}
		// The ranges ascend. A range that goes back starts after the end
		// of the ranges before it, so that each glyph is set once.
		next := 0
		for i := range n {
			r := data[3+3*i:]
			first, fd := int(binary.BigEndian.Uint16(r)), r[2]
			end := min(int(binary.BigEndian.Uint16(r[3:])), numGlyphs)
			for gid := max(first, next); gid < end; gid++ {
				selection[gid] = fd
			}
			next = max(next, end)
		}
	default:
		return nil, false
	}
	for _, fd := range selection {
		if int(fd) >= count {
			return nil, false
		}
	}
	return selection, true
}

// matrixOf returns the FontMatrix of a glyph, and false if the font gives
// none for it. The matrix is then cffDefaultMatrix.
func (f *cffProgram) matrixOf(gid GlyphID) (kurbo.Affine, bool) {
	if int(gid) < len(f.fdSelect) {
		fd := f.fdSelect[gid]
		return f.fdMatrices[fd], f.fdHasMatrix[fd]
	}
	return f.matrix, f.hasMatrix
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
		// Expert and ExpertSubset: the SID of each glyph is in a table.
		table := cffExpertCharset[:]
		if offset == 2 {
			table = cffExpertSubsetCharset[:]
		}
		f.charset = make([]uint16, f.numGlyphs)
		copy(f.charset, table)
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
	m, _ := f.matrixOf(gid)
	f.outlineWith(gid, m, pen)
}

// outlineWith draws a glyph into the pen, scaled by matrix to UnitsPerEm. A
// glyph of seac draws its accent and its base glyph.
func (f *cffProgram) outlineWith(gid GlyphID, matrix kurbo.Affine, pen *outlinePath) {
	if int(gid) >= f.numGlyphs {
		return
	}
	m := kurbo.AffineScale(float64(UnitsPerEm)).Mul(matrix)
	if accent, base, dx, dy, ok := f.seac(gid); ok {
		// skrifa draws the accent first, at its offset, and then the base
		// glyph.
		drawSegments(f.segments(accent), m.Mul(kurbo.AffineTranslate(kurbo.NewVec2(dx, dy))), pen)
		drawSegments(f.segments(base), m, pen)
	}
	drawSegments(f.segments(gid), m, pen)
}

// segments returns the outline of a glyph from go-text, in the units of
// the glyph.
func (f *cffProgram) segments(gid GlyphID) []ot.Segment {
	var segs []ot.Segment
	_ = safely(func() error {
		var err error
		segs, _, err = f.ot.LoadGlyph(tables.GlyphID(gid))
		return goTextError(err)
	})
	return segs
}

// seac returns the accent and the base glyph of a glyph that endchar
// builds from two glyphs of StandardEncoding, and the offset of the accent.
// It returns false for another glyph, for a CIDFont, and for a code of
// StandardEncoding that the font does not draw.
//
// Port: skrifa finds the glyph of a code of StandardEncoding through its SID
// and the charset. Go finds it through the name of the code, which is the
// standard string of the SID.
func (f *cffProgram) seac(gid GlyphID) (accent, base GlyphID, dx, dy float64, ok bool) {
	if f.isCID || int(gid) >= f.numGlyphs {
		return 0, 0, 0, 0, false
	}
	var (
		m      ps.Machine
		finder seacFinder
	)
	err := safely(func() error {
		return m.Run(f.ot.Charstrings[gid], f.localSubrs, f.globalSubrs, &finder)
	})
	if err != nil || !finder.found {
		return 0, 0, 0, 0, false
	}
	glyph := func(code float64) (GlyphID, bool) {
		if code < 0 || code > 255 {
			return 0, false
		}
		name, ok := standardGet(uint8(code))
		if !ok {
			return 0, false
		}
		return f.glyphByName(name)
	}
	a := finder.args
	accent, ok1 := glyph(a[3])
	base, ok2 := glyph(a[2])
	if !ok1 || !ok2 {
		return 0, 0, 0, 0, false
	}
	return accent, base, a[0], a[1], true
}

// seacFinder is a handler of Type 2 charstrings that finds the operands of
// an endchar with four operands, adx ady bchar achar, the seac of Type 1. It
// counts the stem hints so that it skips the bytes of a hintmask, and it
// ignores the other operators.
type seacFinder struct {
	stems ps.CharstringReader
	found bool
	args  [4]float64
}

// Context satisfies the ps.OperatorHandler interface.
func (*seacFinder) Context() ps.Context { return ps.Type2Charstring }

// Apply satisfies the ps.OperatorHandler interface.
func (s *seacFinder) Apply(state *ps.Machine, op ps.Operator) error {
	if !op.IsEscaped {
		switch op.Operator {
		case 11: // return
			return state.Return()
		case 10: // callsubr
			return ps.LocalSubr(state)
		case 29: // callgsubr
			return ps.GlobalSubr(state)
		case 14: // endchar
			// An optional width comes before the four operands.
			if n := state.ArgStack.Top; n == 4 || n == 5 {
				s.found = true
				copy(s.args[:], state.ArgStack.Vals[n-4:n])
			}
			return ps.ErrInterrupt
		case 1, 18: // hstem, hstemhm
			s.stems.Hstem(state)
		case 3, 23: // vstem, vstemhm
			s.stems.Vstem(state)
		case 19, 20: // hintmask, cntrmask
			s.stems.Hintmask(state)
			return nil
		}
	}
	state.ArgStack.Clear()
	return nil
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

// cffExpertCharset and cffExpertSubsetCharset are the SIDs of the glyphs of
// the predefined charsets Expert and ExpertSubset (Adobe Technical Note
// 5176, The Compact Font Format Specification, Appendix C).
var (
	cffExpertCharset = [166]uint16{
		0, 1, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 13, 14, 15, 99,
		239, 240, 241, 242, 243, 244, 245, 246, 247, 248, 27, 28, 249, 250, 251, 252,
		253, 254, 255, 256, 257, 258, 259, 260, 261, 262, 263, 264, 265, 266, 109, 110,
		267, 268, 269, 270, 271, 272, 273, 274, 275, 276, 277, 278, 279, 280, 281, 282,
		283, 284, 285, 286, 287, 288, 289, 290, 291, 292, 293, 294, 295, 296, 297, 298,
		299, 300, 301, 302, 303, 304, 305, 306, 307, 308, 309, 310, 311, 312, 313, 314,
		315, 316, 317, 318, 158, 155, 163, 319, 320, 321, 322, 323, 324, 325, 326, 150,
		164, 169, 327, 328, 329, 330, 331, 332, 333, 334, 335, 336, 337, 338, 339, 340,
		341, 342, 343, 344, 345, 346, 347, 348, 349, 350, 351, 352, 353, 354, 355, 356,
		357, 358, 359, 360, 361, 362, 363, 364, 365, 366, 367, 368, 369, 370, 371, 372,
		373, 374, 375, 376, 377, 378,
	}
	cffExpertSubsetCharset = [87]uint16{
		0, 1, 231, 232, 235, 236, 237, 238, 13, 14, 15, 99, 239, 240, 241, 242,
		243, 244, 245, 246, 247, 248, 27, 28, 249, 250, 251, 253, 254, 255, 256, 257,
		258, 259, 260, 261, 262, 263, 264, 265, 266, 109, 110, 267, 268, 269, 270, 272,
		300, 301, 302, 305, 314, 315, 158, 155, 163, 320, 321, 322, 323, 324, 325, 326,
		150, 164, 169, 327, 328, 329, 330, 331, 332, 333, 334, 335, 336, 337, 338, 339,
		340, 341, 342, 343, 344, 345, 346,
	}
)
