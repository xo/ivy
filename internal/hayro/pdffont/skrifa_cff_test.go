package pdffont

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/xo/okage/kurbo"
)

// This file ports no upstream file. It tests the parts of a CFF font that
// skrifa_cff.go reads beside go-text.

// cffInt returns the encoding of an integer operand of a DICT or of a Type 2
// charstring.
func cffInt(v int) []byte {
	switch {
	case v >= -107 && v <= 107:
		return []byte{byte(v + 139)}
	case v >= 108 && v <= 1131:
		v -= 108
		return []byte{byte(v/256 + 247), byte(v % 256)}
	case v >= -1131 && v <= -108:
		v = -v - 108
		return []byte{byte(v/256 + 251), byte(v % 256)}
	}
	return []byte{28, byte(v >> 8), byte(v)}
}

// cffOffset returns the encoding of an integer operand of a DICT in 5 bytes,
// so that its size does not depend on its value.
func cffOffset(v int) []byte {
	return []byte{29, byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// cffOps returns the encoding of a sequence of operands and operators. An int
// is an operand, a float64 is a real operand of a DICT, and a string is an
// operator: its bytes.
func cffOps(items ...any) []byte {
	var b []byte
	for _, item := range items {
		switch v := item.(type) {
		case int:
			b = append(b, cffInt(v)...)
		case float64:
			// a real number of a DICT, as nibbles: 0.001 only
			if v != 0.001 {
				panic("cffOps: only the real 0.001")
			}
			b = append(b, 30, 0x0a, 0x00, 0x1f)
		case []byte:
			b = append(b, v...)
		case string:
			b = append(b, v...)
		}
	}
	return b
}

// cffINDEX returns an INDEX of the items, with offsets of 4 bytes.
func cffINDEX(items ...[]byte) []byte {
	b := binary.BigEndian.AppendUint16(nil, uint16(len(items)))
	if len(items) == 0 {
		return b
	}
	b = append(b, 4)
	off := 1
	b = binary.BigEndian.AppendUint32(b, uint32(off))
	for _, item := range items {
		off += len(item)
		b = binary.BigEndian.AppendUint32(b, uint32(off))
	}
	for _, item := range items {
		b = append(b, item...)
	}
	return b
}

// The operators of Type 2 charstrings and of DICTs.
const (
	opRMoveTo  = "\x15"
	opRLineTo  = "\x05"
	opEndChar  = "\x0e"
	opCallSubr = "\x0a"
	opReturn   = "\x0b"
	opHStem    = "\x01"
	opHintMask = "\x13"

	opCharset     = "\x0f"
	opCharStrings = "\x11"
	opPrivate     = "\x12"
	opSubrs       = "\x13"
	opROS         = "\x0c\x1e"
	opFontMatrix  = "\x0c\x07"
	opFDArray     = "\x0c\x24"
	opFDSelect    = "\x0c\x25"
)

// square returns a charstring of a square of side size at (x, y).
func square(x, y, size int) []byte {
	return cffOps(x, y, opRMoveTo, size, 0, opRLineTo, 0, size, opRLineTo, -size, 0, opRLineTo, opEndChar)
}

// cffFont returns a CFF font of the charstrings, with the SIDs of glyphs 1
// and on in a charset of format 0. top holds more operands and operators of
// the Top DICT, fdArray the font dictionaries of a CIDFont, and subrs the
// local subroutines.
func cffFont(charstrings [][]byte, sids []int, top []byte, fdArray [][]byte, subrs [][]byte) []byte {
	header := []byte{1, 0, 4, 4}
	name := cffINDEX([]byte("Test"))
	strings := cffINDEX([]byte("Adobe"), []byte("Identity"))
	global := cffINDEX()
	// The Private DICT has its subroutines after it.
	var private []byte
	if subrs != nil {
		private = cffOps(cffOffset(len(cffOps(cffOffset(0), opSubrs))), opSubrs)
	}
	// The Top DICT has a size that does not depend on the offsets, which
	// are of 5 bytes each.
	topDict := func(charset, chars, privateAt, fdArrayAt, fdSelect int) []byte {
		d := cffOps(top, cffOffset(charset), opCharset, cffOffset(chars), opCharStrings)
		if fdArray != nil {
			return cffOps(d, cffOffset(fdArrayAt), opFDArray, cffOffset(fdSelect), opFDSelect)
		}
		return cffOps(d, cffOffset(len(private)), cffOffset(privateAt), opPrivate)
	}
	start := len(header) + len(name) + len(cffINDEX(topDict(0, 0, 0, 0, 0))) + len(strings) + len(global)

	var tail bytes.Buffer
	charset := start + tail.Len()
	tail.WriteByte(0)
	for _, sid := range sids {
		tail.Write(binary.BigEndian.AppendUint16(nil, uint16(sid)))
	}
	chars := start + tail.Len()
	tail.Write(cffINDEX(charstrings...))
	privateAt := start + tail.Len()
	tail.Write(private)
	if subrs != nil {
		tail.Write(cffINDEX(subrs...))
	}
	fdArrayAt, fdSelect := 0, 0
	if fdArray != nil {
		dicts := make([][]byte, 0, len(fdArray))
		for _, d := range fdArray {
			dicts = append(dicts, cffOps(d, cffOffset(len(private)), cffOffset(privateAt), opPrivate))
		}
		fdArrayAt = start + tail.Len()
		tail.Write(cffINDEX(dicts...))
		fdSelect = start + tail.Len()
		// format 0: the font dictionaries in turn
		tail.WriteByte(0)
		for i := range charstrings {
			tail.WriteByte(byte(i % len(fdArray)))
		}
	}
	var b []byte
	b = append(b, header...)
	b = append(b, name...)
	b = append(b, cffINDEX(topDict(charset, chars, privateAt, fdArrayAt, fdSelect))...)
	b = append(b, strings...)
	b = append(b, global...)
	return append(b, tail.Bytes()...)
}

// bounds returns the bounds of the outline of a glyph, in UnitsPerEm.
func bounds(t *testing.T, f *cffProgram, gid GlyphID) kurbo.Rect {
	t.Helper()
	pen := newOutlinePath()
	f.outline(gid, pen)
	path := pen.take()
	if path.IsEmpty() {
		t.Fatalf("glyph %d: expected an outline", gid)
	}
	return path.BoundingBox()
}

// near reports whether the two rectangles are within 0.01 of each other.
func near(a, b kurbo.Rect) bool {
	return math.Abs(a.X0-b.X0) < 0.01 && math.Abs(a.Y0-b.Y0) < 0.01 &&
		math.Abs(a.X1-b.X1) < 0.01 && math.Abs(a.Y1-b.Y1) < 0.01
}

// TestCFFSeac checks that a glyph that endchar builds from two glyphs of
// StandardEncoding draws the base glyph and the accent at its offset, also
// when the operands come from a subroutine and after a hintmask.
func TestCFFSeac(t *testing.T) {
	t.Parallel()
	const (
		sidA     = 34  // A, the code 65 of StandardEncoding
		sidAcute = 125 // acute, the code 194 of StandardEncoding
		sidAring = 162 // Aring
	)
	charstrings := [][]byte{
		cffOps(opEndChar),
		square(0, 0, 100),
		square(10, 10, 20),
		// the width 500, a stem and a hintmask, and the operands of seac in
		// a subroutine; a font of fewer than 1240 subroutines has the bias
		// 107
		cffOps(500, 0, 50, opHStem, opHintMask, []byte{0x80}, 100, 200, -107, opCallSubr, opEndChar),
	}
	subrs := [][]byte{cffOps(65, 194, opReturn)}
	data := cffFont(charstrings, []int{sidA, sidAcute, sidAring}, nil, nil, subrs)
	f, err := newCFFProgram(data)
	if err != nil {
		t.Fatal(err)
	}
	// the scale of the default FontMatrix to UnitsPerEm: glyph 3 draws the
	// base glyph 1 and the accent 2 at (100, 200)
	s := float64(UnitsPerEm) * 0.001
	exp := kurbo.NewRect(0, 0, 130*s, 230*s)
	if got := bounds(t, f, 3); !near(got, exp) {
		t.Errorf("expected the bounds %v, got %v", exp, got)
	}
	if got, exp := bounds(t, f, 1), kurbo.NewRect(0, 0, 100*s, 100*s); !near(got, exp) {
		t.Errorf("expected the bounds %v of glyph 1, got %v", exp, got)
	}
}

// TestCFFCIDFontMatrix checks that a glyph of a CIDFont takes the FontMatrix
// of its font dictionary, after the FontMatrix of the Top DICT.
func TestCFFCIDFontMatrix(t *testing.T) {
	t.Parallel()
	charstrings := [][]byte{cffOps(opEndChar), square(0, 0, 500), square(0, 0, 500)}
	ros := cffOps(cffInt(391), cffInt(392), 0, opROS)
	scaled := cffOps(0.001, 0, 0, 0.001, 0, 0, opFontMatrix)
	doubled := cffOps(2, 0, 0, 2, 0, 0, opFontMatrix)
	s := float64(UnitsPerEm) * 0.001
	for _, c := range []struct {
		name string
		top  []byte
		fds  [][]byte
		// the size of glyph 1, of the font dictionary 1, and of glyph 2, of
		// the font dictionary 0
		size1, size2 float64
	}{
		// the identity in the Top DICT, and 0.001 in the font dictionaries
		{"identity", cffOps(ros, 1, 0, 0, 1, 0, 0, opFontMatrix), [][]byte{scaled, scaled}, 500 * s, 500 * s},
		// no matrix in the Top DICT
		{"none", ros, [][]byte{scaled, nil}, 500 * s, 500 * s},
		// the Top DICT doubles the matrix of the font dictionary
		{"doubled", cffOps(ros, doubled), [][]byte{scaled, nil}, 1000 * 1000 * s, 1000 * s},
	} {
		f, err := newCFFProgram(cffFont(charstrings, []int{1, 2}, c.top, c.fds, nil))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !f.isCID {
			t.Fatalf("%s: expected a CIDFont", c.name)
		}
		if got := bounds(t, f, 1); !near(got, kurbo.NewRect(0, 0, c.size1, c.size1)) {
			t.Errorf("%s: glyph 1: expected a square of %g, got %v", c.name, c.size1, got)
		}
		if got := bounds(t, f, 2); !near(got, kurbo.NewRect(0, 0, c.size2, c.size2)) {
			t.Errorf("%s: glyph 2: expected a square of %g, got %v", c.name, c.size2, got)
		}
	}
}

// TestCFFExpertCharset checks that a font with the predefined charset Expert
// maps a code of the supplements of its encoding through the SIDs of that
// charset.
func TestCFFExpertCharset(t *testing.T) {
	t.Parallel()
	f := &cffProgram{numGlyphs: 4}
	f.readCharset(nil, 1)
	if len(f.charset) != 4 || f.charset[2] != 229 || f.charset[3] != 230 {
		t.Fatalf("expected the SIDs of Expert, got %v", f.charset)
	}
	f = &cffProgram{numGlyphs: 200}
	f.readCharset(nil, 2)
	if len(f.charset) != 200 || f.charset[86] != 346 || f.charset[87] != 0 {
		t.Fatalf("expected the SIDs of ExpertSubset, got %v", f.charset[80:90])
	}
	// at the offset 2, an encoding of format 0 with no codes, and a
	// supplement that maps the code 0x41 to the SID 230, glyph 3 of Expert
	f = &cffProgram{numGlyphs: 10}
	for i := range f.codes {
		f.codes[i] = -1
	}
	f.readCharset(nil, 1)
	f.readEncoding([]byte{0, 0, 0x80, 0, 1, 0x41, 0, 230}, 2)
	if g, ok := f.encodingGlyph(0x41); !ok || g != 3 {
		t.Errorf("expected glyph 3 for the code 0x41, got %d (%v)", g, ok)
	}
}
