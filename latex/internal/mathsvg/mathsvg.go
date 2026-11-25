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
	if f.Describe().Family == "Latin Modern Math" {
		subscriptsOfTeX(f)
	}
	// The driver draws with a face of its own. Without it, it draws the
	// outlines that the box gives.
	draw, _ := font.Load(data, 0)
	return &program{font: f, draw: draw}, nil
}

// The shifts of a subscript of pdfTeX with Computer Modern, in thousandths
// of an em of the size of the nucleus.
const (
	// texSub1 is sub1, \fontdimen16 of cmsy10: the shift of a subscript
	// without a superscript.
	texSub1 = 150
	// texSubDrop is sub_drop, \fontdimen19 of cmsy7, at the size of the
	// nucleus of the text style: 0.5pt at 10pt.
	texSubDrop = 50
)

// subscriptsOfTeX gives Latin Modern Math the shifts of a subscript of
// pdfTeX. Latin Modern Math gives SubscriptShiftDown the value of sub2 of
// cmsy10, 0.247em, which TeX takes only when a superscript is also there.
// TeX takes sub1, 0.15em, for a subscript alone, so K_n of pdfLaTeX has its
// n 1pt higher. The table MATH has one shift for both, and texmath takes it
// for both. A subscript with a superscript still goes down to 0.247em or
// more by the gap of SubSuperscriptGapMin. Latin Modern Math also gives
// SubscriptBaselineDropMin 0.2em, where TeX drops a subscript 0.5pt below
// the depth of a nucleus that is not one character, such as \hat y.
func subscriptsOfTeX(f *gofont.Font) {
	rec := &f.MATH.MathConstants.Records
	rec[gofont.SubscriptShiftDown-gofont.MathLeading].Value = texSub1
	rec[gofont.SubscriptBaselineDropMin-gofont.MathLeading].Value = texSubDrop
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
	box, err := texmath.LayoutTeX(displayCells(mathDots(basePrimes(tex))), r.face, float64(size), style,
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

// textCommands are the commands whose argument is text, where an
// apostrophe is a character and not a prime.
var textCommands = map[string]bool{
	"text": true, "textrm": true, "textit": true, "textbf": true, "textsf": true,
	"texttt": true, "textnormal": true, "textup": true, "textsl": true, "mbox": true,
	"hbox": true, "intertext": true,
}

// basePrimes returns tex with each prime ' as \prime, the glyph prime at
// the size of the formula. texmath sets x' as the glyph prime in a
// superscript, and the glyph of a math font is already high and small, so
// the prime came out half the size of the prime of pdfLaTeX and too high.
// The apostrophes of the argument of a command of text stay.
func basePrimes(tex string) string {
	// x^\prime and x^{\prime} are x' written out
	if strings.Contains(tex, `^`) {
		tex = primeScripts.Replace(tex)
	}
	if !strings.Contains(tex, "'") {
		return tex
	}
	var b strings.Builder
	b.Grow(len(tex) + 16)
	for i := 0; i < len(tex); {
		switch c := tex[i]; c {
		case '\\':
			j := i + 1
			for j < len(tex) && isLetter(tex[j]) {
				j++
			}
			if j == i+1 && j < len(tex) {
				// a control symbol, such as \' or \\
				j++
			}
			name := tex[i+1 : j]
			b.WriteString(tex[i:j])
			i = j
			if textCommands[name] {
				for i < len(tex) && tex[i] == ' ' {
					b.WriteByte(tex[i])
					i++
				}
				end := groupEnd(tex, i)
				b.WriteString(tex[i:end])
				i = end
			}
		case '\'':
			b.WriteString(`\prime `)
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// The dots of plain TeX: \ldots is \mathinner{\ldotp\ldotp\ldotp} and \cdots
// is \mathinner{\cdotp\cdotp\cdotp}, where \ldotp and \cdotp are of the
// class Punct. The class Punct puts a thin space after each dot in the text
// and the display styles.
const (
	lowDots    = `\mathinner{\mathpunct{.}\mathpunct{.}\mathpunct{.}}`
	centerDots = `\mathinner{\mathpunct{\cdot}\mathpunct{\cdot}\mathpunct{\cdot}}`
)

// dotCommands gives the dots that each command of dots of amsmath sets. An
// empty value is \dots, which looks at what follows it.
var dotCommands = map[string]string{
	"dots": "", "ldots": lowDots, "dotsc": lowDots, "dotso": lowDots, "mathellipsis": lowDots,
	"cdots": centerDots, "dotsb": centerDots, "dotsm": centerDots, "dotsi": centerDots,
}

// binaryCommands are the commands of a binary operator, a relation, or a
// large operator that amsmath marks with \DOTSB or \DOTSI. \dots before one
// of them sets the dots at the center, as \dotsb.
var binaryCommands = map[string]bool{
	"cdot": true, "times": true, "pm": true, "mp": true, "div": true, "ast": true,
	"oplus": true, "ominus": true, "otimes": true, "odot": true, "circ": true,
	"cup": true, "cap": true, "vee": true, "wedge": true, "setminus": true,
	"le": true, "leq": true, "ge": true, "geq": true, "ne": true, "neq": true,
	"equiv": true, "sim": true, "simeq": true, "approx": true, "cong": true,
	"subset": true, "subseteq": true, "supset": true, "supseteq": true,
	"to": true, "rightarrow": true, "leftarrow": true, "Rightarrow": true,
	"Leftarrow": true, "iff": true, "implies": true, "mapsto": true,
	"sum": true, "prod": true, "coprod": true, "bigcup": true, "bigcap": true,
	"bigoplus": true, "bigotimes": true, "int": true, "iint": true, "oint": true,
	"lor": true, "land": true, "in": true,
}

// mathDots returns tex with each command of dots as the dots of TeX. The
// glyphs U+2026 and U+22EF that texmath sets for \ldots and \cdots are
// narrower than the three dots and two thin spaces of TeX: 8.4pt and 7.8pt
// where pdfLaTeX sets 11.7pt. \dots takes the dots at the center before a
// binary operator or a relation, and the low dots elsewhere, as amsmath
// does. The arguments of a command of text stay.
func mathDots(tex string) string {
	if !strings.Contains(tex, "dots") && !strings.Contains(tex, "mathellipsis") {
		return tex
	}
	var b strings.Builder
	b.Grow(len(tex) + 64)
	for i := 0; i < len(tex); {
		if tex[i] != '\\' {
			b.WriteByte(tex[i])
			i++
			continue
		}
		j := i + 1
		for j < len(tex) && isLetter(tex[j]) {
			j++
		}
		if j == i+1 && j < len(tex) {
			j++
		}
		name := tex[i+1 : j]
		dots, ok := dotCommands[name]
		switch {
		case ok:
			if dots == "" {
				dots = lowDots
				if binaryFollows(tex[j:]) {
					dots = centerDots
				}
			}
			b.WriteString(dots)
			i = j
		case textCommands[name]:
			for j < len(tex) && tex[j] == ' ' {
				j++
			}
			end := groupEnd(tex, j)
			b.WriteString(tex[i:end])
			i = end
		default:
			b.WriteString(tex[i:j])
			i = j
		}
	}
	return b.String()
}

// binaryFollows reports whether tex starts, after its spaces, with a binary
// operator, a relation, or a command of binaryCommands.
func binaryFollows(tex string) bool {
	tex = strings.TrimLeft(tex, " \t\n")
	if tex == "" {
		return false
	}
	switch tex[0] {
	case '+', '-', '=', '<', '>', '*':
		return true
	case '\\':
		j := 1
		for j < len(tex) && isLetter(tex[j]) {
			j++
		}
		return binaryCommands[tex[1:j]]
	}
	return false
}

// displayEnvs are the environments of amsmath whose cells are in the
// display style: the templates of aligned, alignedat, split and gathered
// start each cell with \displaystyle.
var displayEnvs = map[string]bool{
	"aligned": true, "alignedat": true, "split": true, "gathered": true,
}

// displayCells returns tex with \displaystyle at the start of each cell of
// the environments of displayEnvs. texmath sets their cells in the text
// style, so an integral or a fraction in a split was small, where amsmath
// sets it at the size of a display. A cell of an environment inside them,
// such as a matrix, keeps its style.
func displayCells(tex string) string {
	if !strings.Contains(tex, `\begin`) {
		return tex
	}
	// cells holds, for each open environment and group, whether its cells
	// are in the display style.
	var cells []bool
	in := func() bool { return len(cells) > 0 && cells[len(cells)-1] }
	var b strings.Builder
	b.Grow(len(tex) + 64)
	for i := 0; i < len(tex); {
		switch c := tex[i]; c {
		case '{':
			cells = append(cells, false)
			b.WriteByte(c)
			i++
		case '}':
			if len(cells) > 0 {
				cells = cells[:len(cells)-1]
			}
			b.WriteByte(c)
			i++
		case '&':
			b.WriteByte(c)
			i++
			if in() {
				b.WriteString(`\displaystyle `)
			}
		case '\\':
			j := i + 1
			for j < len(tex) && isLetter(tex[j]) {
				j++
			}
			if j == i+1 && j < len(tex) {
				j++
			}
			name := tex[i+1 : j]
			switch {
			case name == "begin" || name == "end":
				end := groupEnd(tex, j)
				env := strings.TrimSuffix(strings.Trim(tex[j:end], "{}"), "*")
				b.WriteString(tex[i:end])
				i = end
				if name == "end" {
					if len(cells) > 0 {
						cells = cells[:len(cells)-1]
					}
					continue
				}
				cells = append(cells, displayEnvs[env])
				if !displayEnvs[env] {
					continue
				}
				// the arguments of alignedat and the position of aligned
				for i < len(tex) && (tex[i] == '{' || tex[i] == '[') {
					end := groupEnd(tex, i)
					if tex[i] == '[' {
						end = strings.IndexByte(tex[i:], ']') + i + 1
						if end <= i {
							end = len(tex)
						}
					}
					b.WriteString(tex[i:end])
					i = end
				}
				b.WriteString(`\displaystyle `)
			case name == `\` && in():
				b.WriteString(tex[i:j])
				i = j
				if i < len(tex) && tex[i] == '*' {
					b.WriteByte('*')
					i++
				}
				if k := skipSpaces(tex, i); k < len(tex) && tex[k] == '[' {
					if end := strings.IndexByte(tex[k:], ']'); end >= 0 {
						b.WriteString(tex[i : k+end+1])
						i = k + end + 1
					}
				}
				if !strings.HasPrefix(tex[skipSpaces(tex, i):], `\end`) {
					b.WriteString(`\displaystyle `)
				}
			case textCommands[name]:
				k := skipSpaces(tex, j)
				end := groupEnd(tex, k)
				b.WriteString(tex[i:end])
				i = end
			default:
				b.WriteString(tex[i:j])
				i = j
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// skipSpaces returns the index of the first byte of tex at or after i that
// is not a space.
func skipSpaces(tex string, i int) int {
	for i < len(tex) && (tex[i] == ' ' || tex[i] == '\t' || tex[i] == '\n') {
		i++
	}
	return i
}

// primeScripts replaces a superscript that holds only primes with the
// primes.
var primeScripts = strings.NewReplacer(
	`^{\prime\prime}`, `\prime\prime `,
	`^{\prime}`, `\prime `,
	`^\prime`, `\prime`,
)

// isLetter reports whether c is a letter of the name of a command.
func isLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// groupEnd returns the end of the braced group that starts at i, or i when
// no group starts there. A group without its closing brace ends with tex.
func groupEnd(tex string, i int) int {
	if i >= len(tex) || tex[i] != '{' {
		return i
	}
	depth := 0
	for j := i; j < len(tex); j++ {
		switch tex[j] {
		case '\\':
			j++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return len(tex)
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
