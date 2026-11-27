package pict

import (
	"strings"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy/internal/charset"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/ivy/internal/fontdb"
)

// macFonts are the names of the fonts of the Macintosh by their numbers,
// for a picture that does not name its fonts with the opcode fontName.
var macFonts = map[int]string{
	0:  "Chicago",
	1:  "Geneva",
	2:  "New York",
	3:  "Geneva",
	4:  "Monaco",
	5:  "Venice",
	6:  "London",
	7:  "Athens",
	8:  "San Francisco",
	9:  "Toronto",
	11: "Cairo",
	12: "Los Angeles",
	20: "Times",
	21: "Helvetica",
	22: "Courier",
	23: "Symbol",
}

// The styles of TxFace.
const (
	faceBold      = 1
	faceItalic    = 2
	faceUnderline = 4
	faceCondense  = 32
	faceExtend    = 64
)

// The bounds of the fonts that a picture names and of the faces that it
// keeps.
const (
	maxFontNames = 256
	maxFaces     = 64
)

// faceKey identifies a face that the text of a picture takes.
type faceKey struct {
	name         string
	bold, italic bool
}

// fontName reads the opcode fontName: its size, the number of the font
// and its name, which the opcode TxFont then takes.
func (r *reader) fontName() error {
	if !r.has(2) {
		return errShort
	}
	start := r.pos
	n := r.u16()
	end := start + 2 + n
	if end > len(r.b) {
		return errShort
	}
	defer func() { r.pos = end }()
	if n < 3 {
		return nil
	}
	id := r.s16()
	size := r.u8()
	if r.pos+size > end {
		return nil
	}
	name, _ := charset.Decode(r.b[r.pos:r.pos+size], "macintosh")
	st := &r.st
	if st.fonts == nil {
		st.fonts = make(map[int]string)
	}
	if len(st.fonts) < maxFontNames {
		st.fonts[id] = name
	}
	return nil
}

// textOp reads and draws the opcode of the text op: LongText, which gives
// the location of the text, or DHText, DVText or DHDVText, which move it
// across, down or both.
func (r *reader) textOp(op int) error {
	st := &r.st
	switch op {
	case 0x0028:
		if !r.has(4) {
			return errShort
		}
		st.txLoc = r.point()
	case 0x0029:
		st.txLoc.h += r.u8()
	case 0x002A:
		st.txLoc.v += r.u8()
	default:
		st.txLoc.h += r.u8()
		st.txLoc.v += r.u8()
	}
	n := r.u8()
	if r.pos+n > len(r.b) {
		return errShort
	}
	text := r.b[r.pos : r.pos+n]
	r.pos += n
	r.text(text)
	return nil
}

// text draws the text raw, in the encoding Mac OS Roman, at the location of
// the text, with the font, the style, the size and the mode of the text.
// The styles outline and shadow draw as plain text.
func (r *reader) text(raw []byte) {
	st := &r.st
	if len(raw) == 0 {
		return
	}
	name, ok := st.fonts[st.txFont]
	if !ok {
		name = macFonts[st.txFont]
	}
	face := r.face(name, st.txFace&faceBold != 0, st.txFace&faceItalic != 0)
	if face == nil {
		return
	}
	size := float32(st.txSize)
	if size <= 0 {
		size = 12
	}
	sx, sy := st.ratio[0], st.ratio[1]
	var extra float32
	switch {
	case st.txFace&faceCondense != 0:
		extra = -1
	case st.txFace&faceExtend != 0:
		extra = 1
	}
	s, _ := charset.Decode(raw, "macintosh")
	x0, y := float32(st.txLoc.h), float32(st.txLoc.v)
	x := x0
	glyphs := make([]display.Glyph, 0, len(raw))
	for i, c := range s {
		if c < ' ' {
			continue
		}
		gid, _ := face.Glyph(c)
		glyphs = append(glyphs, display.Glyph{ID: gid, X: x, Y: y, Start: i, End: i + utf8.RuneLen(c)})
		x += face.Advance(gid)*size*sx + extra
		if c == ' ' {
			x += st.spExtra
		}
	}
	if len(glyphs) == 0 {
		return
	}
	b, ok := okage.RectFromLTRB(min(x0, x), y-size*sy, max(x0, x)+size*sx, y+size*sy/3)
	if !ok {
		return
	}
	run := display.GlyphRun{Face: face, Size: size, Glyphs: glyphs, Transform: okage.TransformFromScale(sx, sy), Text: s}
	var underline *okage.Path
	if st.txFace&faceUnderline != 0 {
		thick := max(size*sy/12, 0.5)
		if u, ok := okage.RectFromLTRB(x0, y+thick, x, y+2*thick); ok {
			underline = okage.PathFromRect(u)
		}
	}
	r.paintWith(b, black, st.txMode, func(sh okage.Shader) {
		r.list.FillGlyphs(run, sh)
		if underline != nil {
			r.list.FillPath(underline, okage.FillRuleWinding, sh)
		}
	})
}

// face returns the face of the font name in the style, or the face of its
// generic family, or nil when the database of the fonts has none. It loads
// the database once.
func (r *reader) face(name string, bold, italic bool) *font.Face {
	key := faceKey{name, bold, italic}
	if f, ok := r.faces[key]; ok {
		return f
	}
	if r.db == nil {
		r.db = fontdb.Of(r.cfg)
	}
	weight := uint16(400)
	if bold {
		weight = 700
	}
	id, ok := fontdb.Find(r.db, name, weight, italic)
	if !ok {
		q := svg.FontQuery{Weight: weight, Stretch: svg.FontStretchNormal, Style: svg.FontStyleNormal}
		if italic {
			q.Style = svg.FontStyleItalic
		}
		q.Families = []svg.FontFamily{{Kind: generic(name)}, {Kind: svg.FontFamilySansSerif}}
		id, ok = r.db.Query(q)
	}
	var f *font.Face
	if ok {
		f = fontdb.NewFace(r.db, id, nil)
	}
	if r.faces == nil {
		r.faces = make(map[faceKey]*font.Face)
	}
	if len(r.faces) < maxFaces {
		r.faces[key] = f
	}
	return f
}

// generic returns the generic family of a font of the Macintosh by its
// name.
func generic(name string) svg.FontFamilyKind {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "monaco"), strings.Contains(n, "courier"), strings.Contains(n, "mono"):
		return svg.FontFamilyMonospace
	case strings.Contains(n, "sans"):
		return svg.FontFamilySansSerif
	case strings.Contains(n, "times"), strings.Contains(n, "new york"), strings.Contains(n, "palatino"),
		strings.Contains(n, "toronto"), strings.Contains(n, "athens"), strings.Contains(n, "bookman"),
		strings.Contains(n, "century"), strings.Contains(n, "garamond"), strings.Contains(n, "serif"),
		strings.Contains(n, "roman"):
		return svg.FontFamilySerif
	}
	return svg.FontFamilySansSerif
}
