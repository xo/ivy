package odraw

import (
	"math"
	"slices"
	"strings"
	"unicode"
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
	PropGtextAlign   = 0x00C2
	PropGtextSpacing = 0x00C4
	PropGtextFont    = 0x00C5
	PropGtextBools   = 0x00FF
)

// The bits of the boolean properties of geometry text, whose bits of use
// are 16 bits higher (MS-ODRAW 2.3.22.10).
const (
	gtextSmallcaps   = 1 << 1
	gtextItalic      = 1 << 4
	gtextBold        = 1 << 5
	gtextVertical    = 1 << 13
	gtextGtext       = 1 << 14
	gtextReverseRows = 1 << 15
)

// The values of gtextAlign. The default is the center.
const (
	alignStretch    = 0
	alignCenter     = 1
	alignLeft       = 2
	alignRight      = 3
	alignLetterJust = 4
	alignWordJust   = 5
)

const (
	// maxWordArt bounds the characters of the text of a shape.
	maxWordArt = 4096
	// defaultSpacing is the default of gtextSpacing, 1 in 16.16 bits, and
	// maxSpacing bounds it.
	defaultSpacing = 0x10000
	maxSpacing     = 10
	// smallcapsScale is the size of a small capital, as a share of a
	// capital.
	smallcapsScale = 0.7
)

// WordArt reports whether the shape sh with the properties c is WordArt: a
// shape of a type of text effect, or a shape that fGtext turns on.
func WordArt(sh *Shape, c Chain) bool {
	return sh.Type >= SptTextEffectMin && sh.Type <= SptTextEffectMax ||
		c.Flag(PropGtextBools, gtextGtext, gtextGtext<<16, false)
}

// complexText returns the text of a complex property of UTF-16 characters,
// which a character 0 can end. It keeps at most maxWordArt characters.
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

// gtext holds the properties of geometry text that lay out the glyphs.
type gtext struct {
	face      *font.Face
	spacing   float32
	smallcaps bool
	vertical  bool
}

// wordArt returns the outlines of the glyphs of the text of the WordArt
// shape sh with the properties c, in its box of w by h points. Each line
// of the text fills its share of the height. The lines fill the width as
// gtextAlign says: each line stretches to the width, or the longest line
// fills it and the other lines align with it. A type of text effect then
// bends the text onto its shape, such as an arch or a wave (warp.go). It
// returns false when s has no fonts, or when the text draws nothing.
func (s *Style) wordArt(sh *Shape, c Chain, w, h float64) ([]drawingml.Path, bool) {
	text := complexText(c.Complex(PropGtextUnicode))
	if s.Fonts == nil || strings.TrimSpace(text) == "" || !(w > 0) || !(h > 0) {
		return nil, false
	}
	bold := c.Flag(PropGtextBools, gtextBold, gtextBold<<16, false)
	italic := c.Flag(PropGtextBools, gtextItalic, gtextItalic<<16, false)
	g := gtext{
		face:      s.face(complexText(c.Complex(PropGtextFont)), bold, italic),
		spacing:   float32(min(max(Fixed(c.Value(PropGtextSpacing, defaultSpacing)), 0), maxSpacing)),
		smallcaps: c.Flag(PropGtextBools, gtextSmallcaps, gtextSmallcaps<<16, false),
		vertical:  c.Flag(PropGtextBools, gtextVertical, gtextVertical<<16, false),
	}
	if g.face == nil {
		return nil, false
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if c.Flag(PropGtextBools, gtextReverseRows, gtextReverseRows<<16, false) {
		slices.Reverse(lines)
	}
	type line struct {
		p *okage.Path
		b okage.Rect
	}
	outs := make([]line, len(lines))
	widest, total := float32(0), float32(0)
	for i, l := range lines {
		p, ok := g.lineOutline(l)
		if !ok {
			continue
		}
		b, ok := p.ComputeTightBounds()
		if !ok || !(b.Width() > 0) || !(b.Height() > 0) {
			continue
		}
		outs[i] = line{p, b}
		widest = max(widest, b.Width())
		total += b.Height()
	}
	if widest == 0 {
		return nil, false
	}
	align := c.Value(PropGtextAlign, alignCenter)
	lh := h / float64(len(lines))
	var paths []drawingml.Path
	for i, l := range outs {
		if l.p == nil {
			continue
		}
		b := l.b
		// each line fills its share of the height
		sy := float32(lh) / b.Height()
		sx := float32(w) / widest
		x := float32(0)
		switch align {
		case alignStretch, alignLetterJust, alignWordJust:
			sx = float32(w) / b.Width()
		case alignLeft:
		case alignRight:
			x = float32(w) - b.Width()*sx
		default:
			x = (float32(w) - b.Width()*sx) / 2
		}
		t := okage.TransformFromRow(sx, 0, 0, sy, x-b.Left()*sx, float32(float64(i)*lh)-b.Top()*sy)
		if q, ok := l.p.Transform(t); ok {
			paths = append(paths, drawingml.Path{Path: q, Stroke: true})
		}
	}
	if len(paths) == 0 {
		return nil, false
	}
	// the warp of an arch keeps the shape of the text along the arc
	if wp := warpOf(sh.Type, c, float64(total/widest), w, h); wp != nil {
		for i := range paths {
			if q, ok := warpPath(paths[i].Path, wp, w, h); ok {
				paths[i].Path = q
			}
		}
	}
	return paths, true
}

// lineOutline returns the outlines of the glyphs of a line of text, in ems,
// on one baseline, or in one column for vertical text. It returns false
// for a line that draws nothing.
func (g *gtext) lineOutline(line string) (*okage.Path, bool) {
	pb := okage.NewPathBuilder()
	pen := float32(0)
	for _, r := range line {
		scale := float32(1)
		if g.smallcaps {
			if u := unicode.ToUpper(r); u != r {
				r, scale = u, smallcapsScale
			}
		}
		gid, ok := g.face.Glyph(r)
		if !ok {
			continue
		}
		adv := g.face.Advance(gid)
		var t okage.Transform
		if g.vertical {
			// the glyphs stand one below the other, each centered on
			// the column
			vx, vy, ok := g.face.VerticalOrigin(gid)
			if !ok {
				vx, vy = adv/2, 0.8
			}
			t = okage.TransformFromRow(scale, 0, 0, scale, -vx*scale, pen+vy*scale)
			adv = g.face.VerticalAdvance(gid)
		} else {
			t = okage.TransformFromRow(scale, 0, 0, scale, pen, 0)
		}
		if o := g.face.Outline(gid); o != nil {
			// Transform changes the path, and the face keeps its outlines
			if q, ok := o.Clone().Transform(t); ok {
				pb.PushPath(q)
			}
		}
		pen += adv * scale * g.spacing
		if math.IsInf(float64(pen), 0) || math.IsNaN(float64(pen)) {
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
