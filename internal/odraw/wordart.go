package odraw

import (
	"math"
	"strings"
	"unicode/utf16"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/ivy/internal/fontdb"
)

// This file draws the text of WordArt: a shape whose properties of
// geometry text (MS-ODRAW 2.3.22) give its text and its font. The outlines
// of the glyphs are the paths of the shape, with its fill and its line.

// The properties of geometry text that the package reads.
const (
	PropGtextUnicode = 0x00C0
	PropGtextFont    = 0x00C5
	PropGtextBools   = 0x00FF
)

// The bits of the boolean properties of geometry text, whose bits of use
// are 16 bits higher (MS-ODRAW 2.3.22.10).
const (
	gtextItalic = 1 << 4
	gtextBold   = 1 << 5
	gtextGtext  = 1 << 14
)

// maxWordArt bounds the characters of the text of a shape.
const maxWordArt = 4096

// WordArt reports whether the shape sh with the properties c is WordArt: a
// shape of a type of text effect, or a shape that fGtext turns on.
func WordArt(sh *Shape, c Chain) bool {
	return sh.Type >= SptTextEffectMin && sh.Type <= SptTextEffectMax ||
		c.Flag(PropGtextBools, gtextGtext, gtextGtext<<16, false)
}

// complexText returns the text of a complex property of UTF-16 characters,
// which a character 0 can end.
func complexText(b []byte) string {
	n := min(len(b)/2, maxWordArt)
	units := make([]uint16, 0, n)
	for i := range n {
		u := le.Uint16(b[2*i:])
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}

// wordArt returns the outlines of the glyphs of the text of the WordArt
// shape with the properties c, in its box of w by h points. Each line of
// the text fills its share of the height and the whole width, as the
// plain text of WordArt does. The shapes that bend the text, such as an
// arch, draw it plain. It returns false when s has no fonts, or when the
// text draws nothing.
func (s *Style) wordArt(c Chain, w, h float64) ([]drawingml.Path, bool) {
	text := complexText(c.Complex(PropGtextUnicode))
	if s.Fonts == nil || strings.TrimSpace(text) == "" || !(w > 0) || !(h > 0) {
		return nil, false
	}
	bold := c.Flag(PropGtextBools, gtextBold, gtextBold<<16, false)
	italic := c.Flag(PropGtextBools, gtextItalic, gtextItalic<<16, false)
	face := s.face(complexText(c.Complex(PropGtextFont)), bold, italic)
	if face == nil {
		return nil, false
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var paths []drawingml.Path
	lh := h / float64(len(lines))
	for i, line := range lines {
		p, ok := lineOutline(face, line)
		if !ok {
			continue
		}
		b, ok := p.ComputeTightBounds()
		if !ok || !(b.Width() > 0) || !(b.Height() > 0) {
			continue
		}
		sx, sy := float32(w)/b.Width(), float32(lh)/b.Height()
		t := okage.TransformFromRow(sx, 0, 0, sy, -b.Left()*sx, float32(float64(i)*lh)-b.Top()*sy)
		if q, ok := p.Transform(t); ok {
			paths = append(paths, drawingml.Path{Path: q, Stroke: true})
		}
	}
	return paths, len(paths) > 0
}

// lineOutline returns the outlines of the glyphs of a line of text in the
// face f, in ems, on one baseline. It returns false for a line that draws
// nothing.
func lineOutline(f *font.Face, line string) (*okage.Path, bool) {
	pb := okage.NewPathBuilder()
	x := float32(0)
	for _, r := range line {
		gid, ok := f.Glyph(r)
		if !ok {
			continue
		}
		if o := f.Outline(gid); o != nil {
			// Transform changes the path, and the face keeps its outlines
			if q, ok := o.Clone().Transform(okage.TransformFromTranslate(x, 0)); ok {
				pb.PushPath(q)
			}
		}
		x += f.Advance(gid)
		if math.IsInf(float64(x), 0) {
			break
		}
	}
	return pb.Finish()
}

// face returns the face of the font name of the fonts of s, or of the sans
// serif family, or nil.
func (s *Style) face(name string, bold, italic bool) *font.Face {
	weight := uint16(400)
	if bold {
		weight = 700
	}
	id, ok := fontdb.Find(s.Fonts, name, weight, italic)
	if !ok {
		q := svg.FontQuery{
			Families: []svg.FontFamily{{Kind: svg.FontFamilySansSerif}},
			Weight:   weight,
			Stretch:  svg.DefaultFontStretch(),
		}
		if italic {
			q.Style = svg.FontStyleItalic
		}
		if id, ok = s.Fonts.Query(q); !ok {
			return nil
		}
	}
	return newFace(s.Fonts, id)
}

// newFace returns the face id of db, or nil.
func newFace(db *svg.FontDatabase, id svg.FontID) (f *font.Face) {
	defer func() {
		// go-text can panic on a malformed font (D5, D18)
		if recover() != nil {
			f = nil
		}
	}()
	face, ok := db.NewFace(id, nil)
	if !ok {
		return nil
	}
	return font.FromFace(face)
}
