// Package mathsvg gives the engine of LaTeX the part of the API of
// github.com/go-tex/math that it calls, on the layout of math of okage,
// texmath (okage D78). The engine is a copy, and its file math.go imports
// this package in place of go-tex/math, so that a new commit of the engine
// still applies (D62).
//
// A formula gives SVG text, as go-tex/math gives, because the engine puts
// the SVG of a formula in a diagram of xymatrix. The renderer also keeps the
// box of each formula, and the SVG names it with the attribute
// data-ivy-math, so that the driver of ivy draws the glyphs of the box as
// runs of glyphs with their text, and not as paths.
package mathsvg

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/okage"
	"github.com/xo/okage/texmath"
	"github.com/xo/okage/texmath/stixtwomath"
	gofont "github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
)

// Metrics is the box of a formula, in the unit of the size: its advance
// width, its height above the baseline and its depth below it.
type Metrics struct {
	Width, Height, Depth float64
}

// The limits of one formula. A document holds many formulas, so a formula
// takes less than the limits of texmath.
const (
	maxDepth      = 64
	maxNodes      = 20000
	maxExpansions = 20000
	maxBoxes      = 1 << 20
)

// Renderer lays out formulas with one font that has a MATH table. It is
// not safe for concurrent use, and the engine gives each run a renderer.
type Renderer struct {
	face *gofont.Face
	// draw is the face that the driver draws the glyphs with, and nil when
	// the driver must draw the outlines.
	draw  *font.Face
	boxes []*texmath.Box
}

// programs holds the parsed fonts of the data of each call of New, by the
// address of the data. A *gofont.Font is safe for concurrent use.
var programs sync.Map

// program is a parsed math font, with the face that draws it.
type program struct {
	font *gofont.Font
	draw *font.Face
}

// New returns a renderer for the math font in data. A nil data takes STIX
// Two Math of okage. It returns an error that wraps ivy.ErrFormat if the
// font has no MATH table.
func New(data []byte) (*Renderer, error) {
	if len(data) == 0 {
		data = stixtwomath.Data()
	}
	key := &data[0]
	if p, ok := programs.Load(key); ok {
		prog := p.(*program)
		return &Renderer{face: gofont.NewFace(prog.font), draw: prog.draw}, nil
	}
	prog, err := parse(data)
	if err != nil {
		return nil, err
	}
	programs.Store(key, prog)
	return &Renderer{face: gofont.NewFace(prog.font), draw: prog.draw}, nil
}

// parse reads a math font.
func parse(data []byte) (prog *program, err error) {
	defer func() {
		if r := recover(); r != nil {
			prog, err = nil, fmt.Errorf("reading math font: %v: %w", r, ivy.ErrFormat)
		}
	}()
	ld, err := ot.NewLoader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("reading math font: %w: %w", ivy.ErrFormat, err)
	}
	f, err := gofont.NewFont(ld)
	if err != nil {
		return nil, fmt.Errorf("reading math font: %w: %w", ivy.ErrFormat, err)
	}
	if !f.HasMath() {
		return nil, fmt.Errorf("math font without a MATH table: %w", ivy.ErrFormat)
	}
	// The driver draws with a face of its own. Without it, it draws the
	// outlines that the box gives.
	draw, _ := font.Load(data, 0)
	return &program{font: f, draw: draw}, nil
}

// RenderSVGMetrics lays out tex in the text style at the size, and returns
// its SVG and its box.
func (r *Renderer) RenderSVGMetrics(tex string, size int) (string, Metrics, error) {
	return r.render(tex, size, texmath.Text)
}

// RenderDisplaySVGMetrics lays out tex in the display style.
func (r *Renderer) RenderDisplaySVGMetrics(tex string, size int) (string, Metrics, error) {
	return r.render(tex, size, texmath.Display)
}

// render lays out tex and writes its SVG.
func (r *Renderer) render(tex string, size int, style texmath.Style) (string, Metrics, error) {
	if size <= 0 {
		size = 10
	}
	if len(r.boxes) >= maxBoxes {
		return "", Metrics{}, fmt.Errorf("math: too many formulas: %w", ivy.ErrLimit)
	}
	box, err := texmath.LayoutTeX(tex, r.face, float64(size), style,
		texmath.WithMaxDepth(maxDepth),
		texmath.WithMaxNodes(maxNodes),
		texmath.WithMaxExpansions(maxExpansions),
	)
	if err != nil {
		// The engine reads the name of an unknown command from the message,
		// in the words of go-tex/math.
		if se, ok := errors.AsType[*texmath.SyntaxError](err); ok && se.Command != "" && strings.HasPrefix(se.Command, `\`) {
			return "", Metrics{}, fmt.Errorf("texmath: unknown command %s: %w", se.Command, err)
		}
		return "", Metrics{}, err
	}
	id := len(r.boxes)
	r.boxes = append(r.boxes, box)
	m := Metrics{Width: box.Width, Height: box.Height, Depth: box.Depth}
	return r.svg(box, id), m, nil
}

// Box returns the box of the formula whose SVG names id, and false if the
// renderer has none.
func (r *Renderer) Box(id int) (*texmath.Box, bool) {
	if id < 0 || id >= len(r.boxes) {
		return nil, false
	}
	return r.boxes[id], true
}

// DrawFace returns the face that draws the glyphs of the math font, or nil
// when the driver must draw the outlines.
func (r *Renderer) DrawFace() *font.Face {
	return r.draw
}

// MathFace returns the face of okage that lays out the formulas, which the
// glyphs of each box name.
func (r *Renderer) MathFace() *gofont.Face {
	return r.face
}

// svg writes the box as go-tex/math does: an svg element with the box, and
// a group with the baseline at the height of the box. An item of the color
// black takes currentColor, which the engine replaces with the color of the
// text.
func (r *Renderer) svg(box *texmath.Box, id int) string {
	w, h := box.Width, box.Height+box.Depth
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" data-ivy-math="%d">`,
		ftoa(w), ftoa(h), ftoa(w), ftoa(h), id)
	fmt.Fprintf(&b, `<g fill="currentColor" transform="translate(0,%s)">`, ftoa(box.Height))
	for i := range box.Items {
		p := box.ItemPath(i)
		if p == nil {
			continue
		}
		b.WriteString(`<path d="`)
		writePath(&b, p)
		b.WriteByte('"')
		if c := box.Items[i].Color; c != okage.ColorBlack {
			fmt.Fprintf(&b, ` fill="%s"`, Hex(c))
		}
		b.WriteString(`/>`)
	}
	b.WriteString(`</g></svg>`)
	return b.String()
}

// Hex returns the color as #rrggbb, and its opacity is lost.
func Hex(c okage.Color) string {
	u := c.ToColorU8()
	return fmt.Sprintf("#%02x%02x%02x", u.Red(), u.Green(), u.Blue())
}

// writePath writes the data of an SVG path.
func writePath(b *strings.Builder, p *okage.Path) {
	it := p.Segments()
	for {
		seg, ok := it.Next()
		if !ok {
			return
		}
		var n int
		switch seg.Kind {
		case okage.PathSegmentMoveTo:
			b.WriteByte('M')
			n = 1
		case okage.PathSegmentLineTo:
			b.WriteByte('L')
			n = 1
		case okage.PathSegmentQuadTo:
			b.WriteByte('Q')
			n = 2
		case okage.PathSegmentCubicTo:
			b.WriteByte('C')
			n = 3
		case okage.PathSegmentClose:
			b.WriteByte('Z')
		}
		for i := range n {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(ftoa(float64(seg.Points[i].X)))
			b.WriteByte(' ')
			b.WriteString(ftoa(float64(seg.Points[i].Y)))
		}
	}
}

// ftoa formats a float with up to three decimals and no trailing zeros, as
// go-tex/math does.
func ftoa(v float64) string {
	if v == 0 {
		return "0"
	}
	s := strings.TrimRight(strconv.FormatFloat(v, 'f', 3, 64), "0")
	s = strings.TrimSuffix(s, ".")
	if s == "-0" {
		return "0"
	}
	return s
}
