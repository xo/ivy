// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file tests the reader of Type 1 fonts, which replaces skrifa.
// Upstream has no such test.

package pdffont

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
)

// cs builds a charstring of Type 1 from numbers and operators. An int is a
// number, and a string is an operator.
func cs(items ...any) []byte {
	ops := map[string][]byte{
		"hstem": {1}, "vstem": {3}, "vmoveto": {4}, "rlineto": {5}, "hlineto": {6},
		"vlineto": {7}, "rrcurveto": {8}, "closepath": {9}, "callsubr": {10},
		"return": {11}, "hsbw": {13}, "endchar": {14}, "rmoveto": {21},
		"hmoveto": {22}, "vhcurveto": {30}, "hvcurveto": {31},
		"seac": {12, 6}, "sbw": {12, 7}, "div": {12, 12}, "callothersubr": {12, 16},
		"pop": {12, 17}, "setcurrentpoint": {12, 33},
	}
	var out []byte
	for _, it := range items {
		switch v := it.(type) {
		case int:
			switch {
			case v >= -107 && v <= 107:
				out = append(out, byte(v+139))
			case v >= 108 && v <= 1131:
				out = append(out, byte((v-108)/256+247), byte((v-108)%256))
			case v >= -1131 && v <= -108:
				out = append(out, byte((-v-108)/256+251), byte((-v-108)%256))
			default:
				out = append(out, 255)
				out = binary.BigEndian.AppendUint32(out, uint32(int32(v)))
			}
		case string:
			op, ok := ops[v]
			if !ok {
				panic("unknown operator " + v)
			}
			out = append(out, op...)
		}
	}
	return out
}

// encrypt encrypts data with the key r, after skip bytes of zeros.
func encrypt(data []byte, r uint16, skip int) []byte {
	plain := append(make([]byte, skip), data...)
	out := make([]byte, len(plain))
	for i, p := range plain {
		c := p ^ byte(r>>8)
		out[i] = c
		r = (uint16(c)+r)*52845 + 22719
	}
	return out
}

// type1Fixture builds a Type 1 font with the subroutines and glyphs. The
// form is "binary", "hex" or "pfb".
func type1Fixture(subrs [][]byte, glyphs map[string][]byte, order []string, form string) []byte {
	var clearText bytes.Buffer
	clearText.WriteString("%!PS-AdobeFont-1.0: Test 001\n")
	clearText.WriteString("10 dict begin\n/FontName /Test def\n/FontMatrix [0.001 0 0 0.001 0 0] readonly def\n")
	clearText.WriteString("/Encoding 256 array\n0 1 255 {1 index exch /.notdef put} for\n")
	clearText.WriteString("dup 65 /A put\ndup 66 /B put\nreadonly def\ncurrentdict end\ncurrentfile eexec\n")

	var priv bytes.Buffer
	priv.WriteString("dup /Private 8 dict dup begin\n/RD{string currentfile exch readstring pop}executeonly def\n")
	priv.WriteString("/ND{noaccess def}executeonly def\n/NP{noaccess put}executeonly def\n/lenIV 4 def\n")
	fmt.Fprintf(&priv, "/Subrs %d array\n", len(subrs))
	for i, s := range subrs {
		e := encrypt(s, charstringKey, 4)
		fmt.Fprintf(&priv, "dup %d %d RD ", i, len(e))
		priv.Write(e)
		priv.WriteString(" NP\n")
	}
	priv.WriteString("ND\n2 index /CharStrings ")
	fmt.Fprintf(&priv, "%d dict dup begin\n", len(glyphs))
	for _, name := range order {
		e := encrypt(glyphs[name], charstringKey, 4)
		fmt.Fprintf(&priv, "/%s %d RD ", name, len(e))
		priv.Write(e)
		priv.WriteString(" ND\n")
	}
	priv.WriteString("end\nend\nreadonly put\nnoaccess put\ndup /FontName get exch definefont pop\nmark currentfile closefile\n")
	enc := encrypt(priv.Bytes(), eexecKey, 4)

	switch form {
	case "hex":
		return append(clearText.Bytes(), []byte(hex.EncodeToString(enc)+"\n")...)
	case "pfb":
		var out []byte
		seg := func(kind byte, data []byte) {
			out = append(out, 0x80, kind)
			out = binary.LittleEndian.AppendUint32(out, uint32(len(data)))
			out = append(out, data...)
		}
		seg(1, clearText.Bytes())
		seg(2, enc)
		out = append(out, 0x80, 3)
		return out
	}
	return append(clearText.Bytes(), enc...)
}

// flexSubrs are the subroutines 0 to 3 of flex and of the replacement of
// hints, as the Type 1 format defines them.
var flexSubrs = [][]byte{
	cs(3, 0, "callothersubr", "pop", "pop", "setcurrentpoint", "return"),
	cs(0, 1, "callothersubr", "return"),
	cs(0, 2, "callothersubr", "return"),
	cs("return"),
}

// testGlyphs are the glyphs of the fixture.
func testGlyphs() (map[string][]byte, []string) {
	glyphs := map[string][]byte{
		// A triangle drawn through a subroutine.
		"A": cs(10, 600, "hsbw", 0, 0, "rmoveto", 4, "callsubr", 300, 600, "rlineto", "closepath", "endchar"),
		// A flex of two curves from (0, 0) to (300, 0).
		"B": cs(0, 600, "hsbw", 0, 0, "rmoveto",
			1, "callsubr",
			100, 0, "rmoveto", 2, "callsubr",
			-50, 20, "rmoveto", 2, "callsubr",
			50, 0, "rmoveto", 2, "callsubr",
			50, 0, "rmoveto", 2, "callsubr",
			50, 0, "rmoveto", 2, "callsubr",
			50, 0, "rmoveto", 2, "callsubr",
			50, -20, "rmoveto", 2, "callsubr",
			50, 300, 0, 0, "callsubr",
			0, 100, "rlineto", "closepath", "endchar"),
		// The replacement of hints: subroutine 3 through othersubr 3.
		"C": cs(0, 600, "hsbw", 3, 1, 3, "callothersubr", "pop", "callsubr",
			0, 0, "rmoveto", 100, 0, "rlineto", 0, 100, "rlineto", "closepath", "endchar"),
		// An accented glyph: A with C on top, shifted by 100, 50.
		"Aacute": cs(10, 600, "hsbw", 0, 100, 50, 65, 67, "seac"),
		// A subroutine that calls itself without end.
		"loop":    cs(0, 600, "hsbw", 5, "callsubr", "endchar"),
		".notdef": cs(0, 250, "hsbw", "endchar"),
	}
	return glyphs, []string{"A", "B", "C", "Aacute", "loop", ".notdef"}
}

// fixtureSubrs are the subroutines of the fixture.
func fixtureSubrs() [][]byte {
	return append(append([][]byte{}, flexSubrs...),
		cs(300, -600, "rlineto", "return"),
		cs(5, "callsubr", "return"),
	)
}

// elements describes the elements of a path, for a comparison.
func elements(p kurboext.BezPath) string {
	var b bytes.Buffer
	for _, el := range p.Elements() {
		switch el.Kind {
		case kurbo.PathElMoveTo:
			fmt.Fprintf(&b, "M%g,%g ", el.P[0].X, el.P[0].Y)
		case kurbo.PathElLineTo:
			fmt.Fprintf(&b, "L%g,%g ", el.P[0].X, el.P[0].Y)
		case kurbo.PathElQuadTo:
			fmt.Fprintf(&b, "Q%g,%g ", el.P[1].X, el.P[1].Y)
		case kurbo.PathElCurveTo:
			fmt.Fprintf(&b, "C%g,%g,%g,%g,%g,%g ", el.P[0].X, el.P[0].Y, el.P[1].X, el.P[1].Y, el.P[2].X, el.P[2].Y)
		case kurbo.PathElClosePath:
			b.WriteString("Z ")
		}
	}
	return b.String()
}

// TestType1Program reads the fixture in its three forms and draws its
// glyphs.
func TestType1Program(t *testing.T) {
	t.Parallel()
	glyphs, order := testGlyphs()
	for _, form := range []string{"binary", "hex", "pfb"} {
		t.Run(form, func(t *testing.T) {
			t.Parallel()
			p, err := newFontProgram(type1Fixture(fixtureSubrs(), glyphs, order, form), 0)
			if err != nil {
				t.Fatal(err)
			}
			if p.programKind() != programType1 {
				t.Fatalf("kind %d", p.programKind())
			}
			if got := p.type1.type1Names(); got != ".notdef B C Aacute loop A" {
				t.Errorf("glyphs %q", got)
			}
			outline := func(name string) string {
				g, ok := p.glyphByName(name)
				if !ok {
					t.Fatalf("no glyph %s", name)
				}
				return elements(p.outline(g))
			}
			for _, c := range []struct{ name, want string }{
				{"A", "M10,0 L310,-600 L610,0 Z "},
				{"B", "M0,0 C50,20,100,20,150,20 C200,20,250,20,300,0 L300,100 Z "},
				{"C", "M0,0 L100,0 L100,100 Z "},
				{"Aacute", "M10,0 L310,-600 L610,0 Z M100,50 L200,50 L200,150 Z "},
				{".notdef", ""},
			} {
				if got := outline(c.name); got != c.want {
					t.Errorf("%s: %s, want %s", c.name, got, c.want)
				}
			}
			// A subroutine that calls itself stops at the limit.
			outline("loop")
			if g, ok := p.builtinEncodingGlyph('B'); !ok || g != 1 {
				t.Errorf("code B maps to %d, %v", g, ok)
			}
			if name, ok := p.builtinEncodingGlyphName('A'); !ok || name != "A" {
				t.Errorf("code A has the name %q, %v", name, ok)
			}
		})
	}
}

// TestType1Blend tests the blend of a multiple master font.
func TestType1Blend(t *testing.T) {
	t.Parallel()
	f := &type1Program{weights: []float64{0.25, 0.75}, lenIV: -1}
	d := &type1Decoder{font: f}
	// Two masters and one result: 100 + 0.75 * 40.
	got := d.blend(14, []float64{100, 40})
	if len(got) != 1 || got[0] != 130 {
		t.Errorf("blend = %v, want [130]", got)
	}
}
