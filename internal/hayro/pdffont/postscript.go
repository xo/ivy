// This file ports no file of hayro-font. It gives the interpreter of
// PostScript in ps/internal the font programs of this package (D29): the
// Type 1 fonts that a PostScript program defines with its charstrings, the
// font programs that it embeds, such as TrueType in a Type 42 font, and the
// substitutes of the standard fonts.

package pdffont

import (
	"errors"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
)

// Program is a font program for the interpreter of PostScript. An outline
// is in the units of UnitsPerEm for a program that NewProgram or
// StandardProgram returns, and in the units of the charstrings for one
// that NewType1Charstrings returns.
type Program struct {
	p *fontProgram
}

// NewProgram reads a font program: Type 1, CFF, TrueType or OpenType. For a
// collection, index selects the font.
func NewProgram(data []byte, index uint32) (*Program, error) {
	p, err := newFontProgram(data, index)
	if err != nil {
		return nil, err
	}
	return &Program{p: p}, nil
}

// StandardProgram returns the substitute of a standard font.
func StandardProgram(s StandardFont) (*Program, error) {
	data, index := s.GetFontData()
	return NewProgram(data, index)
}

// NewType1Charstrings returns the program of a Type 1 font that a
// PostScript program defines: its charstrings by name, still encrypted, its
// subroutines and the value of lenIV. weights holds the weights of a
// multiple master font, or nil.
func NewType1Charstrings(names []string, charstrings [][]byte, subrs [][]byte, lenIV int, weights []float64) (*Program, error) {
	if len(names) != len(charstrings) {
		return nil, errors.New("type 1 charstrings without their names")
	}
	f := &type1Program{
		// the outlines stay in the units of the charstrings
		matrix:  kurbo.AffineScale(1 / float64(UnitsPerEm)),
		subrs:   subrs,
		lenIV:   lenIV,
		weights: weights,
		names:   make(map[string]GlyphID, len(names)),
	}
	for i, name := range names {
		if len(f.glyphs) >= maxType1Glyphs {
			break
		}
		if _, ok := f.names[name]; ok {
			continue
		}
		f.names[name] = GlyphID(len(f.glyphs))
		f.glyphs = append(f.glyphs, type1ProgramGlyph{name: name, charstring: charstrings[i]})
	}
	return &Program{p: &fontProgram{kind: programType1, type1: f}}, nil
}

// GlyphByName returns the glyph of a name.
func (p *Program) GlyphByName(name string) (GlyphID, bool) {
	return p.p.glyphByName(name)
}

// Outline returns the outline of a glyph.
func (p *Program) Outline(g GlyphID) kurboext.BezPath {
	return p.p.outline(g)
}

// Type1Glyph returns the outline of a glyph of a Type 1 program, and the
// advance that its charstring gives. ok is false when the charstring gives
// no advance.
func (p *Program) Type1Glyph(g GlyphID) (outline kurboext.BezPath, advance kurbo.Vec2, ok bool) {
	f := p.p.type1
	if f == nil || int(g) >= len(f.glyphs) {
		return kurboext.BezPath{}, kurbo.Vec2{}, false
	}
	pen := newOutlinePath()
	d := &type1Decoder{
		font:   f,
		pen:    pen,
		matrix: kurbo.AffineScale(float64(UnitsPerEm)).Mul(f.matrix),
	}
	_ = d.glyph(f.glyphs[g].charstring, 0, 0)
	if d.open {
		pen.closePath()
	}
	return pen.take(), d.width, d.hasWidth
}

// Advance returns the advance of a glyph of a TrueType or OpenType program,
// in the units of UnitsPerEm.
func (p *Program) Advance(g GlyphID) (float32, bool) {
	return p.p.advanceWidth(g)
}

// CodeName returns the name of a code in the encoding of a standard font:
// StandardEncoding, or the built-in encoding of Symbol or ZapfDingbats.
func (s StandardFont) CodeName(code uint8) (string, bool) {
	return s.codeToName(code)
}

// Width returns the width of a glyph of a standard font, in the units of
// UnitsPerEm.
func (s StandardFont) Width(name string) (float32, bool) {
	return s.getWidth(name)
}

// StandardEncodingName returns the name of a code in StandardEncoding.
func StandardEncodingName(code uint8) (string, bool) {
	return standardGet(code)
}

// SymbolEncodingName returns the name of a code in the built-in encoding of
// Symbol.
func SymbolEncodingName(code uint8) (string, bool) {
	return symbolGet(code)
}

// DingbatsEncodingName returns the name of a code in the built-in encoding
// of ZapfDingbats.
func DingbatsEncodingName(code uint8) (string, bool) {
	return zapfDingsGet(code)
}
