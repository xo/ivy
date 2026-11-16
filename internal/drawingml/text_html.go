package drawingml

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage"
)

// The defaults of a text body, from ECMA-376 Part 1, 21.1.2.1.1 and
// 21.1.2.3.9.
const (
	// defaultInsetX and defaultInsetY are the insets of a text box, 0.1
	// and 0.05 inches.
	defaultInsetX = 7.2
	defaultInsetY = 3.6
	// defaultSize is the size of a run without a size.
	defaultSize = 18.0
	// lineFactor is the height of a line of single spacing, for a size of
	// 1. PowerPoint draws a line of 100 percent at about 1.2 times the size
	// of its text.
	lineFactor = 1.2
	// overflowLines is the height of the page of a text body, past its
	// box, in multiples of the height of the box, so that the text that
	// overflows the box still draws.
	overflowLines = 4
)

// TextBox is a text body that is ready for the layout: a document of HTML
// on a page, and the place of the page in the box of the text.
type TextBox struct {
	// HTML is the document.
	HTML string
	// Width and Height are the size of the page in points.
	Width, Height float64
	// Transform maps the page onto the box of the text.
	Transform okage.Transform
}

// Box returns the text body as a document of HTML for the box of the size
// w by h points. theme gives the fonts of the theme, and can be nil. It
// returns false for a body without text, or a box without area.
func (tb *TextBody) Box(theme *Theme, w, h float64) (TextBox, bool) {
	if tb.Empty() || !(w > 0 && h > 0) {
		return TextBox{}, false
	}
	b := tb.Body
	l, t := b.Insets[0].Or(defaultInsetX), b.Insets[1].Or(defaultInsetY)
	r, bt := b.Insets[2].Or(defaultInsetX), b.Insets[3].Or(defaultInsetY)
	iw, ih := max(w-l-r, 1), max(h-t-bt, 1)
	m := okage.TransformFromTranslate(float32(l), float32(t))
	pw, ph := iw, ih
	switch b.Vert.Or("horz") {
	case "vert", "eaVert", "wordArtVert", "mongolianVert", "wordArtVertRtl":
		// the lines go down, and the page turns clockwise
		pw, ph = ih, iw
		m = okage.TransformFromTranslate(float32(l+iw), float32(t)).PreRotate(90)
	case "vert270":
		pw, ph = ih, iw
		m = okage.TransformFromTranslate(float32(l), float32(t+ih)).PreRotate(-90)
	}
	if rot := b.Rotation.Or(0); rot != 0 && !math.IsNaN(rot) {
		m = okage.TransformFromRotateAt(float32(rot), float32(w/2), float32(h/2)).PreConcat(m)
	}
	page := max(ph*overflowLines, ph+720)
	doc := tb.html(theme, pw, ph, page)
	return TextBox{HTML: doc, Width: pw, Height: page, Transform: m}, true
}

// html writes the document of the text body on a page of the width w and
// the height page, with the text in the top h points.
func (tb *TextBody) html(theme *Theme, w, h, page float64) string {
	var sb strings.Builder
	b := tb.Body
	scale := b.FontScale.Or(1)
	reduction := b.LineReduction.Or(0)
	whiteSpace := "pre-wrap"
	if !b.Wrap.Or(true) {
		whiteSpace = "pre"
	}
	fmt.Fprintf(&sb, `<!DOCTYPE html><html><head><meta charset="utf-8"><style>
@page { size: %spt %spt; margin: 0 }
html, body { margin: 0; padding: 0 }
table { border-collapse: collapse; border-spacing: 0; table-layout: fixed; width: 100%% }
td { padding: 0; overflow-wrap: break-word }
p { margin: 0; white-space: %s; overflow-wrap: break-word; hyphens: none }
span.b { display: inline-block; text-indent: 0; white-space: pre }
</style></head><body>`, num(w), num(page), whiteSpace)
	align := "top"
	switch b.Anchor.Or("t") {
	case "ctr":
		align = "middle"
	case "b":
		align = "bottom"
	}
	fmt.Fprintf(&sb, `<table><tr style="height:%spt"><td style="vertical-align:%s">`, num(h), align)
	if cols := b.Columns.Or(1); cols > 1 {
		fmt.Fprintf(&sb, `<div style="column-count:%d;column-gap:%spt">`, cols, num(b.ColumnGap.Or(0)))
	} else {
		sb.WriteString("<div>")
	}
	for i := range tb.Paragraphs {
		writeParagraph(&sb, &tb.Paragraphs[i], theme, scale, reduction, i == 0)
	}
	sb.WriteString("</div></td></tr></table></body></html>")
	return sb.String()
}

// writeParagraph writes a paragraph as an element p. The space before the
// first paragraph of a body is left out, as PowerPoint does.
func writeParagraph(sb *strings.Builder, p *Paragraph, theme *Theme, scale, reduction float64, first bool) {
	s := &p.Style
	run := p.End
	for _, r := range p.Runs {
		if r.Text != "" {
			run = r.Style
			break
		}
	}
	size := run.Size.Or(defaultSize) * scale
	sb.WriteString(`<p style="`)
	fmt.Fprintf(sb, "font-size:%spt;font-family:%s;", num(size), fontFamily(run.Latin.Or(""), theme))
	if v := s.MarL.Or(0); v != 0 {
		fmt.Fprintf(sb, "padding-left:%spt;", num(max(v, 0)))
	}
	if v := s.MarR.Or(0); v > 0 {
		fmt.Fprintf(sb, "padding-right:%spt;", num(v))
	}
	if v := s.Indent.Or(0); v != 0 {
		fmt.Fprintf(sb, "text-indent:%spt;", num(v))
	}
	switch s.Align.Or("l") {
	case "ctr":
		sb.WriteString("text-align:center;")
	case "r":
		sb.WriteString("text-align:right;")
	case "just", "dist", "justLow", "thaiDist":
		sb.WriteString("text-align:justify;")
	}
	if s.RTL.Or(false) {
		sb.WriteString("direction:rtl;")
	}
	if v, ok := spacing(s.SpaceBefore, size); ok && !first {
		fmt.Fprintf(sb, "margin-top:%spt;", num(v))
	}
	if v, ok := spacing(s.SpaceAfter, size); ok {
		fmt.Fprintf(sb, "margin-bottom:%spt;", num(v))
	}
	switch ls := s.LineSpacing; {
	case ls.OK && ls.V.Points:
		fmt.Fprintf(sb, "line-height:%spt;", num(ls.V.Pt*scale))
	case ls.OK:
		fmt.Fprintf(sb, "line-height:%s;", num(max(ls.V.Pct-reduction, 0.1)*lineFactor))
	case reduction > 0:
		fmt.Fprintf(sb, "line-height:%s;", num(max(1-reduction, 0.1)*lineFactor))
	}
	sb.WriteString(`">`)
	if paragraphEmpty(p) {
		sb.WriteString("<br></p>")
		return
	}
	writeBullet(sb, p, &run, theme, scale)
	for _, r := range p.Runs {
		if r.Break {
			sb.WriteString("<br>")
			continue
		}
		if r.Text == "" {
			continue
		}
		sb.WriteString(`<span style="`)
		writeRunStyle(sb, &r.Style, theme, scale)
		sb.WriteString(`">`)
		sb.WriteString(escapeText(r.Text, &r.Style))
		sb.WriteString("</span>")
	}
	sb.WriteString("</p>")
}

// spacing returns the space before or after a paragraph in points. A
// percentage is of the size of the text, as LibreOffice reads it.
func spacing(o Opt[Spacing], size float64) (float64, bool) {
	if !o.OK {
		return 0, false
	}
	if o.V.Points {
		return o.V.Pt, true
	}
	return o.V.Pct * size, true
}

// writeBullet writes the bullet of a paragraph, in a box as wide as the
// hanging indent.
func writeBullet(sb *strings.Builder, p *Paragraph, first *RunStyle, theme *Theme, scale float64) {
	s := &p.Style
	if !s.Bullet.OK {
		return
	}
	b := s.Bullet.V
	var text string
	font := first.Latin.Or("")
	switch b.Kind {
	case BulletChar:
		var mapped bool
		text, mapped = bulletChar(b.Char, s.BulletFont.Or(""))
		if !mapped && s.BulletFont.Or("") != "" {
			font = s.BulletFont.V
		}
	case BulletNumber:
		text = numberText(b.Scheme, p.Number)
	case BulletPicture:
		text = "•"
	default:
		return
	}
	size := first.Size.Or(defaultSize) * scale
	if bs := s.BulletSize.V; s.BulletSize.OK && bs != nil {
		if bs.Points {
			size = bs.Pt * scale
		} else {
			size *= bs.Pct
		}
	}
	color := first.Paint.Or(Solid(Black)).Mean()
	if bc := s.BulletColor.V; s.BulletColor.OK && bc != nil {
		color = *bc
	}
	width := max(-s.Indent.Or(0), 0)
	fmt.Fprintf(sb, `<span class="b" style="min-width:%spt;font-size:%spt;font-family:%s;color:%s">%s </span>`,
		num(width), num(size), fontFamily(font, theme), color.CSS(), html.EscapeString(text))
}

// writeRunStyle writes the declarations of CSS of the style of a run.
func writeRunStyle(sb *strings.Builder, s *RunStyle, theme *Theme, scale float64) {
	size := s.Size.Or(defaultSize) * scale
	if b := s.Baseline.Or(0); b != 0 {
		fmt.Fprintf(sb, "vertical-align:%spt;", num(b*size))
		size *= 2.0 / 3
	}
	fmt.Fprintf(sb, "font-size:%spt;", num(size))
	font := s.Latin.Or("")
	if font == "" || strings.HasPrefix(font, "+") && theme == nil {
		font = s.EastAsian.Or(font)
	}
	fmt.Fprintf(sb, "font-family:%s;", fontFamily(font, theme))
	if s.Bold.Or(false) {
		sb.WriteString("font-weight:bold;")
	}
	if s.Italic.Or(false) {
		sb.WriteString("font-style:italic;")
	}
	var deco []string
	if s.Underline.Or(false) {
		deco = append(deco, "underline")
	}
	if s.Strike.Or(false) {
		deco = append(deco, "line-through")
	}
	if len(deco) > 0 {
		sb.WriteString("text-decoration:" + strings.Join(deco, " ") + ";")
	}
	if s.Caps.Or("none") == "small" {
		sb.WriteString("font-variant:small-caps;")
	}
	if v := s.Spacing.Or(0); v != 0 {
		fmt.Fprintf(sb, "letter-spacing:%spt;", num(v))
	}
	paint := s.Paint.Or(Solid(Black))
	color := paint.Mean()
	if paint.Kind == PaintNone {
		color = RGBA{}
	}
	sb.WriteString("color:" + color.CSS() + ";")
	if h := s.Highlight; h.OK {
		sb.WriteString("background-color:" + h.V.CSS() + ";")
	}
}

// escapeText escapes the text of a run for HTML, with the capitals of its
// style.
func escapeText(text string, s *RunStyle) string {
	if s.Caps.Or("none") == "all" {
		text = strings.ToUpper(text)
	}
	text = strings.Map(func(r rune) rune {
		switch {
		case r == '\v':
			return '\n'
		case r == utf8.RuneError, r < 0x20 && r != '\t' && r != '\n':
			return -1
		}
		return r
	}, text)
	text = html.EscapeString(text)
	return strings.ReplaceAll(text, "\n", "<br>")
}

// fontFamily returns the value of font-family for a typeface, with the
// typefaces of the theme resolved, such as "+mn-lt", and a generic family
// after it.
func fontFamily(face string, theme *Theme) string {
	face = ThemeFont(face, theme)
	if face == "" {
		return "sans-serif"
	}
	return cssString(face) + ", sans-serif"
}

// ThemeFont returns the typeface that face names: the face itself, or for
// a typeface of the theme such as "+mj-lt" or "+mn-ea", the font of the
// theme. An empty face is the minor Latin font of the theme.
func ThemeFont(face string, theme *Theme) string {
	if face != "" && !strings.HasPrefix(face, "+") {
		return face
	}
	if theme == nil {
		return ""
	}
	fonts := theme.Minor
	if strings.HasPrefix(face, "+mj") {
		fonts = theme.Major
	}
	switch {
	case strings.HasSuffix(face, "-ea"):
		if fonts.EastAsian != "" {
			return fonts.EastAsian
		}
	case strings.HasSuffix(face, "-cs"):
		if fonts.ComplexScript != "" {
			return fonts.ComplexScript
		}
	}
	return fonts.Latin
}

// cssString returns s as a string of CSS, in double quotes.
func cssString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, "\\%x ", r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return html.EscapeString(b.String())
}

// num formats a number for CSS.
func num(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		v = 0
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// symbolBullets maps the characters of the symbol fonts of Windows that
// bullets use to Unicode, by the code of the character in the font.
var symbolBullets = map[rune]string{
	0x6c: "●", 0x6e: "■", 0x71: "❑", 0x75: "◆", 0x76: "❖", 0x77: "⬥",
	0xa7: "▪", 0xa8: "◻", 0xb7: "•", 0xd8: "➢", 0xe0: "➢", 0xe8: "➔",
	0xf0: "⇨", 0xfc: "✔", 0xfb: "✘", 0x2d: "–", 0x6f: "□", 0x70: "◼",
	0x9f: "•", 0xa1: "○", 0xa2: "◉",
}

// bulletChar returns the character of a bullet in Unicode. A character of
// a symbol font, such as Wingdings, becomes the character of Unicode that
// looks the same, and the bullet then draws with the font of the text: it
// returns true for it.
func bulletChar(char, font string) (string, bool) {
	r, _ := utf8.DecodeRuneInString(char)
	if r == utf8.RuneError {
		return "•", true
	}
	symbol := false
	switch strings.ToLower(font) {
	case "wingdings", "wingdings 2", "wingdings 3", "symbol", "webdings", "monotype sorts", "zapfdingbats":
		symbol = true
	}
	if r >= 0xf000 && r <= 0xf0ff {
		r -= 0xf000
		symbol = true
	}
	if !symbol {
		return string(r), false
	}
	if s, ok := symbolBullets[r]; ok {
		return s, true
	}
	return "•", true
}

// numberText returns the text of the number n of an automatic bullet with
// the scheme, such as "3." for arabicPeriod or "(c)" for alphaLcParenBoth.
func numberText(scheme string, n int) string {
	var digits, rest string
	switch {
	case strings.HasPrefix(scheme, "alphaLc"):
		digits, rest = alpha(n, 'a'), scheme[len("alphaLc"):]
	case strings.HasPrefix(scheme, "alphaUc"):
		digits, rest = alpha(n, 'A'), scheme[len("alphaUc"):]
	case strings.HasPrefix(scheme, "romanLc"):
		digits, rest = strings.ToLower(roman(n)), scheme[len("romanLc"):]
	case strings.HasPrefix(scheme, "romanUc"):
		digits, rest = roman(n), scheme[len("romanUc"):]
	case strings.HasPrefix(scheme, "arabic"):
		digits, rest = strconv.Itoa(n), scheme[len("arabic"):]
	default:
		digits = strconv.Itoa(n)
		rest = "Period"
	}
	switch rest {
	case "ParenR":
		return digits + ")"
	case "ParenBoth":
		return "(" + digits + ")"
	case "Plain", "DbPlain":
		return digits
	case "Minus":
		return "- " + digits + " -"
	}
	return digits + "."
}

// alpha returns n in letters, as a, b, ..., z, aa, bb.
func alpha(n int, first rune) string {
	if n < 1 {
		return string(first)
	}
	n--
	return strings.Repeat(string(first+rune(n%26)), min(n/26+1, 32))
}

// roman returns n in Roman numerals.
func roman(n int) string {
	if n < 1 || n >= 4000 {
		return strconv.Itoa(n)
	}
	var b strings.Builder
	for _, d := range [...]struct {
		v int
		s string
	}{{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}} {
		for n >= d.v {
			b.WriteString(d.s)
			n -= d.v
		}
	}
	return b.String()
}
