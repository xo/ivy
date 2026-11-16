package chart

import (
	"image/color"
	"math"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
)

// This file measures and draws the text of a chart. The text is a run of
// glyphs of one face for each line, without kerning and without shaping,
// as the labels of a chart need no more. A character that the face of the
// text lacks takes the face of the sans serif family of the database.

// The metrics of a line of text, in ems: the height of a line, and the
// distance from its top to its baseline. They are near the metrics of
// Calibri, Carlito and Liberation Sans.
const (
	lineHeight = 1.2
	ascent     = 0.95
)

// faceCache keeps the faces that the charts loaded from the bytes of a
// database, so that each chart does not read its fonts again.
var faceCache = struct {
	sync.Mutex
	m map[faceSource]*font.Face
}{m: map[faceSource]*font.Face{}}

// faceSource names a face of the bytes of a database.
type faceSource struct {
	data  *byte
	size  int
	index uint32
}

// maxCachedFaces is the number of the faces that faceCache keeps.
const maxCachedFaces = 64

// loadFace returns the face id of db, or nil.
func loadFace(db *svg.FontDatabase, id svg.FontID) *font.Face {
	data, index, ok := db.FaceData(id)
	if !ok || len(data) == 0 {
		return nil
	}
	key := faceSource{data: &data[0], size: len(data), index: index}
	faceCache.Lock()
	f, ok := faceCache.m[key]
	faceCache.Unlock()
	if ok {
		return f
	}
	f, err := font.Load(data, int(index))
	if err != nil {
		f = nil
	}
	faceCache.Lock()
	if len(faceCache.m) >= maxCachedFaces {
		clear(faceCache.m)
	}
	faceCache.m[key] = f
	faceCache.Unlock()
	return f
}

// texter measures and draws the text of one drawing of a chart.
type texter struct {
	db *svg.FontDatabase
	// family is the family of the chart, which a text without a font of
	// its own takes.
	family string
	// families maps the name of a font to the family of db that draws it.
	families map[string]string
	faces    map[faceKey]*font.Face
}

// faceKey names a face of a texter.
type faceKey struct {
	family string
	bold   bool
}

// newTexter returns a texter of the fonts of db, for a chart whose font is
// the family name.
func newTexter(db *svg.FontDatabase, name string) *texter {
	t := &texter{db: db, families: map[string]string{}, faces: map[faceKey]*font.Face{}}
	t.family = t.resolve(name)
	return t
}

// resolve returns the family of the database that draws the font name: the
// font, else the font with the same metrics, else the sans serif family,
// else the first family.
func (t *texter) resolve(name string) string {
	if f, ok := t.families[name]; ok {
		return f
	}
	f := familyOf(t.db, name)
	if len(t.families) < 64 {
		t.families[name] = f
	}
	return f
}

// familyOf returns the family of db that draws the font name.
func familyOf(db *svg.FontDatabase, name string) string {
	has := func(name string) (string, bool) {
		if name == "" {
			return "", false
		}
		for face := range db.Faces() {
			for _, f := range face.Families {
				if strings.EqualFold(f.Name, name) {
					return f.Name, true
				}
			}
		}
		return "", false
	}
	if f, ok := has(name); ok {
		return f
	}
	if sub, ok := font.Substitute(name); ok {
		if f, ok := has(sub); ok {
			return f
		}
	}
	if f, ok := has(db.FamilyName(svg.FontFamily{Kind: svg.FontFamilySansSerif})); ok {
		return f
	}
	for face := range db.Faces() {
		if len(face.Families) > 0 {
			return face.Families[0].Name
		}
	}
	return ""
}

// face returns the face of a style, or nil when the database has none.
func (t *texter) face(st textStyle) *font.Face {
	family := t.family
	if st.font != "" {
		family = t.resolve(st.font)
	}
	return t.faceOf(family, st.bold)
}

// faceOf returns the face of a family, bold or regular.
func (t *texter) faceOf(family string, bold bool) *font.Face {
	key := faceKey{family: family, bold: bold}
	if f, ok := t.faces[key]; ok {
		return f
	}
	weight := uint16(400)
	if bold {
		weight = 700
	}
	var f *font.Face
	id, ok := t.db.Query(svg.FontQuery{
		Families: []svg.FontFamily{{Kind: svg.FontFamilyNamed, Name: family}, {Kind: svg.FontFamilySansSerif}},
		Weight:   weight, Stretch: svg.FontStretchNormal, Style: svg.FontStyleNormal,
	})
	if ok {
		f = loadFace(t.db, id)
	}
	if len(t.faces) < 64 {
		t.faces[key] = f
	}
	return f
}

// fallback returns the face of the sans serif family, for the characters
// that the face of a text lacks.
func (t *texter) fallback(bold bool) *font.Face {
	return t.faceOf(t.db.FamilyName(svg.FontFamily{Kind: svg.FontFamilySansSerif}), bold)
}

// run is a part of a line of text in one face.
type run struct {
	face   *font.Face
	glyphs []display.Glyph
}

// shape returns the runs of a line of text at the size of st, from x = 0
// on the baseline, and the width of the line.
func (t *texter) shape(s string, st textStyle) ([]run, float64) {
	f := t.face(st)
	size := float32(st.size)
	if f == nil {
		return nil, float64(utf8.RuneCountInString(s)) * st.size * 0.5
	}
	var runs []run
	x := float32(0)
	for _, r := range s {
		if r == '\t' {
			r = ' '
		}
		face := f
		gid, ok := f.Glyph(r)
		if !ok && r > ' ' {
			if fb := t.fallback(st.bold); fb != nil {
				if g, ok := fb.Glyph(r); ok {
					face, gid = fb, g
				}
			}
		}
		if len(runs) == 0 || runs[len(runs)-1].face != face {
			runs = append(runs, run{face: face})
		}
		last := &runs[len(runs)-1]
		last.glyphs = append(last.glyphs, display.Glyph{ID: gid, X: x})
		x += face.Advance(gid) * size
	}
	return runs, float64(x)
}

// width returns the width of a line of text in points.
func (t *texter) width(s string, st textStyle) float64 {
	_, w := t.shape(s, st)
	return w
}

// block is a text of one or more lines and its style.
type block struct {
	lines []string
	st    textStyle
}

// size returns the width and the height of a block.
func (t *texter) size(b block) (w, h float64) {
	for _, l := range b.lines {
		w = max(w, t.width(l, b.st))
	}
	return w, float64(len(b.lines)) * b.st.size * lineHeight
}

// The alignments of a text to its point.
const (
	alignStart = iota
	alignCenter
	alignEnd
)

// draw draws a block of text. The point (x, y) is where its box aligns:
// ha and va align the box to the point, start for its left or its top
// side. angle turns the box about the point, in degrees counterclockwise.
func (t *texter) draw(dev display.Device, b block, x, y float64, ha, va int, angle float64) {
	if len(b.lines) == 0 || !(b.st.size > 0) || b.st.color.A == 0 {
		return
	}
	w, h := t.size(b)
	ox := -w * float64(ha) / 2
	oy := -h * float64(va) / 2
	turned := angle != 0 && !math.IsNaN(angle)
	if turned {
		dev.Save()
		defer dev.Restore()
		dev.Concat(okage.TransformFromTranslate(float32(x), float32(y)).PreConcat(okage.TransformFromRotate(float32(-angle))))
		x, y = 0, 0
	}
	shader := okage.SolidColor(rgba(b.st.color))
	for i, line := range b.lines {
		runs, lw := t.shape(line, b.st)
		lx := x + ox
		switch ha {
		case alignCenter:
			lx += (w - lw) / 2
		case alignEnd:
			lx += w - lw
		}
		base := y + oy + (float64(i)*lineHeight+ascent)*b.st.size
		for _, r := range runs {
			glyphs := make([]display.Glyph, len(r.glyphs))
			for j, g := range r.glyphs {
				glyphs[j] = display.Glyph{ID: g.ID, X: g.X + float32(lx), Y: float32(base)}
			}
			dev.FillGlyphs(display.GlyphRun{Face: r.face, Size: float32(b.st.size), Glyphs: glyphs,
				Transform: okage.IdentityTransform()}, shader)
		}
	}
}

// wrap breaks a text into lines no wider than maxW, at the spaces. A word
// wider than maxW keeps its own line.
func (t *texter) wrap(s string, st textStyle, maxW float64) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := words[0]
		for _, w := range words[1:] {
			if t.width(line+" "+w, st) <= maxW {
				line += " " + w
				continue
			}
			out = append(out, line)
			line = w
		}
		out = append(out, line)
		if len(out) > 64 {
			break
		}
	}
	return out
}

// rgba converts a color to a color of okage.
func rgba(c color.NRGBA) okage.Color {
	return okage.ColorFromRGBA8(c.R, c.G, c.B, c.A)
}
