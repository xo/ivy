package chart

import (
	"image/color"
	"math"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/ivy/internal/fontdb"
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

// faceCache keeps the faces that the charts took from a database, so that
// each chart does not make its faces again.
var faceCache = struct {
	sync.Mutex
	m map[faceSource]*font.Face
}{m: map[faceSource]*font.Face{}}

// faceSource names a face of a database by its information, which the
// copies of the database share.
type faceSource struct {
	info *svg.FaceInfo
}

// maxCachedFaces is the number of the faces that faceCache keeps.
const maxCachedFaces = 64

// loadFace returns the face id of db, or nil. The face shares the font that
// db parses once (okage D57).
func loadFace(db *svg.FontDatabase, id svg.FontID) *font.Face {
	info, ok := db.Face(id)
	if !ok {
		return nil
	}
	key := faceSource{info: info}
	faceCache.Lock()
	f, ok := faceCache.m[key]
	faceCache.Unlock()
	if ok {
		return f
	}
	f = newFace(db, id)
	faceCache.Lock()
	if len(faceCache.m) >= maxCachedFaces {
		clear(faceCache.m)
	}
	faceCache.m[key] = f
	faceCache.Unlock()
	return f
}

// newFace returns a new face of the font id of db, or nil.
func newFace(db *svg.FontDatabase, id svg.FontID) (f *font.Face) {
	defer func() {
		// go-text can panic on a malformed font (D5, D18).
		if recover() != nil {
			f = nil
		}
	}()
	return fontdb.NewFace(db, id, nil)
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

// run is a part of a line of text in one face. The range of each glyph is
// a range of the bytes of the line, and start and end are the bytes of the
// line that the run covers.
type run struct {
	face       *font.Face
	glyphs     []display.Glyph
	start, end int
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
	for i, r := range s {
		_, n := utf8.DecodeRuneInString(s[i:])
		next := i + n
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
			runs = append(runs, run{face: face, start: i})
		}
		last := &runs[len(runs)-1]
		last.glyphs = append(last.glyphs, display.Glyph{ID: gid, X: x, Start: i, End: next})
		last.end = next
		x += face.Advance(gid) * size
	}
	return runs, float64(x)
}

// width returns the width of a line of text in points.
func (t *texter) width(s string, st textStyle) float64 {
	_, w := t.shape(s, st)
	return w
}

// span is a part of a line of rich text, with the style of its run. The
// style of its block gives each field that the span does not set.
type span struct {
	text string
	st   textStyle
}

// block is a text of one or more lines and its style. spans, if not nil,
// holds the runs of each line of lines, for a text of rich text.
type block struct {
	lines []string
	st    textStyle
	spans [][]span
	// center centers each line in the box of the block, whatever the
	// alignment of the box to its point.
	center bool
}

// lineSpans returns the runs of the line i of b, each with its whole
// style.
func (b block) lineSpans(i int) []span {
	if b.spans == nil {
		return []span{{text: b.lines[i], st: b.st}}
	}
	out := make([]span, len(b.spans[i]))
	for j, sp := range b.spans[i] {
		out[j] = span{text: sp.text, st: sp.st.merge(b.st)}
	}
	return out
}

// lineSize returns the size of the text of the line i of b: the size of
// its largest run, or the size of b for a line without runs.
func (b block) lineSize(i int) float64 {
	size := 0.0
	if b.spans != nil {
		for _, sp := range b.lineSpans(i) {
			size = max(size, sp.st.size)
		}
	}
	if !(size > 0) {
		return b.st.size
	}
	return size
}

// spanWidth returns the width of the runs of a line of rich text, whose
// block has the style base.
func (t *texter) spanWidth(line []span, base textStyle) float64 {
	w := 0.0
	for _, sp := range line {
		w += t.width(sp.text, sp.st.merge(base))
	}
	return w
}

// size returns the width and the height of a block.
func (t *texter) size(b block) (w, h float64) {
	if b.spans == nil {
		for _, l := range b.lines {
			w = max(w, t.width(l, b.st))
		}
		return w, float64(len(b.lines)) * b.st.size * lineHeight
	}
	for i := range b.lines {
		w = max(w, t.spanWidth(b.spans[i], b.st))
		h += b.lineSize(i) * lineHeight
	}
	return w, h
}

// spanLines returns the text of each line of rich text.
func spanLines(spans [][]span) []string {
	lines := make([]string, len(spans))
	for i, line := range spans {
		var b strings.Builder
		for _, sp := range line {
			b.WriteString(sp.text)
		}
		lines[i] = b.String()
	}
	return lines
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
	if len(b.lines) == 0 || !(b.st.size > 0) || (b.spans == nil && b.st.color.A == 0) {
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
	// top is the top of the line, below the lines before it, in a block of
	// rich text
	top := 0.0
	for i := range b.lines {
		spans := b.lineSpans(i)
		shaped := make([][]run, len(spans))
		widths := make([]float64, len(spans))
		lw := 0.0
		for j, sp := range spans {
			shaped[j], widths[j] = t.shape(sp.text, sp.st)
			lw += widths[j]
		}
		lx := x + ox
		la := ha
		if b.center {
			la = alignCenter
		}
		switch la {
		case alignCenter:
			lx += (w - lw) / 2
		case alignEnd:
			lx += w - lw
		}
		base := y + oy + (float64(i)*lineHeight+ascent)*b.st.size
		if b.spans != nil {
			size := b.lineSize(i)
			base = y + oy + top + ascent*size
			top += size * lineHeight
		}
		for j, sp := range spans {
			if sp.st.size > 0 && sp.st.color.A != 0 {
				t.drawRuns(dev, shaped[j], sp, lx, base)
			}
			lx += widths[j]
		}
	}
}

// drawRuns draws the runs of the text of sp, from x on the baseline y.
func (t *texter) drawRuns(dev display.Device, runs []run, sp span, x, y float64) {
	shader := okage.SolidColor(rgba(sp.st.color))
	for _, r := range runs {
		glyphs := make([]display.Glyph, len(r.glyphs))
		for j, g := range r.glyphs {
			glyphs[j] = display.Glyph{ID: g.ID, X: g.X + float32(x), Y: float32(y),
				Start: g.Start - r.start, End: g.End - r.start}
		}
		// the run gives its text to a writer of PDF (D58)
		dev.FillGlyphs(display.GlyphRun{Face: r.face, Size: float32(sp.st.size), Glyphs: glyphs,
			Transform: okage.IdentityTransform(), Text: sp.text[r.start:r.end]}, shader)
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

// wrapSpans breaks the lines of rich text into lines no wider than maxW,
// at the spaces, as wrap does. base is the style of the block.
func (t *texter) wrapSpans(lines [][]span, base textStyle, maxW float64) [][]span {
	var out [][]span
	for _, para := range lines {
		words := spanWords(para)
		if len(words) == 0 {
			out = append(out, nil)
			continue
		}
		line := words[0]
		for _, w := range words[1:] {
			next := joinWords(line, w)
			if t.spanWidth(next, base) <= maxW {
				line = next
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

// spanWords returns the words of a line of rich text, as strings.Fields
// splits a text. A word that crosses runs has a part of each run.
func spanWords(line []span) [][]span {
	var words [][]span
	var word []span
	for _, sp := range line {
		start := -1
		for i, r := range sp.text {
			if !unicode.IsSpace(r) {
				if start < 0 {
					start = i
				}
				continue
			}
			if start >= 0 {
				word = append(word, span{text: sp.text[start:i], st: sp.st})
				start = -1
			}
			if word != nil {
				words = append(words, word)
				word = nil
			}
		}
		if start >= 0 {
			word = append(word, span{text: sp.text[start:], st: sp.st})
		}
	}
	if word != nil {
		words = append(words, word)
	}
	return words
}

// joinWords returns a new line of rich text: the line, a space in the
// style of its last run, and the word.
func joinWords(line, word []span) []span {
	out := make([]span, len(line), len(line)+len(word))
	copy(out, line)
	last := &out[len(out)-1]
	last.text += " "
	for _, sp := range word {
		if last := &out[len(out)-1]; last.st == sp.st {
			last.text += sp.text
			continue
		}
		out = append(out, sp)
	}
	return out
}

// rgba converts a color to a color of okage.
func rgba(c color.NRGBA) okage.Color {
	return okage.ColorFromRGBA8(c.R, c.G, c.B, c.A)
}
