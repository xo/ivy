package drawingml

import (
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/fontdb"
)

// The limits of the text of a shape.
const (
	// maxParagraphs is the number of paragraphs of a text body that the
	// package reads.
	maxParagraphs = 4096
	// maxRuns is the number of runs of a paragraph that the package reads.
	maxRuns = 4096
	// maxTextBytes is the number of bytes of text of a text body that the
	// package reads.
	maxTextBytes = 1 << 20
)

// Opt is an optional value of a style: a value that the style sets, or that
// it leaves to the style that it inherits from.
type Opt[T any] struct {
	V  T
	OK bool
}

// Set sets the value.
func (o *Opt[T]) Set(v T) {
	o.V, o.OK = v, true
}

// Or returns the value, or def if it is not set.
func (o Opt[T]) Or(def T) T {
	if o.OK {
		return o.V
	}
	return def
}

// over sets o to p if p is set.
func (o *Opt[T]) over(p Opt[T]) {
	if p.OK {
		*o = p
	}
}

// RunStyle is the style of a run of text, from a:rPr, a:defRPr or
// a:endParaRPr.
type RunStyle struct {
	// Size is the size of the font in points.
	Size                            Opt[float64]
	Bold, Italic, Underline, Strike Opt[bool]
	// Caps is "none", "small" or "all".
	Caps Opt[string]
	// Baseline raises the text by a fraction of its size, for a superscript
	// or a subscript.
	Baseline Opt[float64]
	// Spacing is the spacing of the characters, in points.
	Spacing Opt[float64]
	// Kern is the smallest size of the font, in points, at which the run
	// kerns, from the attribute kern. 0 does not kern. PowerPoint kerns
	// the runs of 12 points and more of custom-geometry.pptx, which has
	// kern="1200", and not its runs with kern="0".
	Kern Opt[float64]
	// Latin, EastAsian, ComplexScript and Symbol are the typefaces. A
	// typeface of the theme starts with "+", such as "+mn-lt".
	Latin, EastAsian, ComplexScript, Symbol Opt[string]
	// Paint is the fill of the text.
	Paint Opt[Paint]
	// Highlight is the color behind the text.
	Highlight Opt[RGBA]
	// Link reports a hyperlink.
	Link Opt[bool]
	// Lang is the language of the East Asian text of the run, such as
	// ja-JP: the lang of the run when it is an East Asian language, or else
	// its altLang. The fallback of an East Asian character prefers a font
	// of the language.
	Lang Opt[string]
}

// Over sets each field of s that o sets.
func (s *RunStyle) Over(o *RunStyle) {
	s.Size.over(o.Size)
	s.Bold.over(o.Bold)
	s.Italic.over(o.Italic)
	s.Underline.over(o.Underline)
	s.Strike.over(o.Strike)
	s.Caps.over(o.Caps)
	s.Baseline.over(o.Baseline)
	s.Spacing.over(o.Spacing)
	s.Kern.over(o.Kern)
	s.Latin.over(o.Latin)
	s.EastAsian.over(o.EastAsian)
	s.ComplexScript.over(o.ComplexScript)
	s.Symbol.over(o.Symbol)
	s.Paint.over(o.Paint)
	s.Highlight.over(o.Highlight)
	s.Link.over(o.Link)
	s.Lang.over(o.Lang)
}

// Spacing is the spacing of lines or of paragraphs: a percentage of the
// size of the text, or a number of points.
type Spacing struct {
	// Pct is a fraction of a line, such as 1.5, when Points is false.
	Pct    float64
	Points bool
	// Pt is the spacing in points, when Points is true.
	Pt float64
}

// BulletKind is the kind of a bullet.
type BulletKind uint8

// The kinds of bullets.
const (
	BulletNone BulletKind = iota
	BulletChar
	BulletNumber
	// BulletPicture is a bullet of a picture, which the package draws as a
	// round bullet.
	BulletPicture
)

// Bullet is the bullet of a paragraph.
type Bullet struct {
	Kind BulletKind
	// Char is the character of BulletChar.
	Char string
	// Scheme is the scheme of the numbers of BulletNumber, such as
	// "arabicPeriod" or "romanUcParenR".
	Scheme string
	// Start is the first number of BulletNumber.
	Start int
}

// BulletSize is the size of a bullet: a fraction of the size of the text, or
// a number of points.
type BulletSize struct {
	Pct    float64
	Points bool
	Pt     float64
}

// ParaStyle is the style of a paragraph, from a:pPr or a level of a list
// style, such as a:lvl1pPr.
type ParaStyle struct {
	// MarL and Indent are the left margin and the indent of the first line,
	// in points.
	MarL, Indent Opt[float64]
	// MarR is the right margin, in points.
	MarR Opt[float64]
	// Align is "l", "ctr", "r", "just", "dist" or "justLow".
	Align                                Opt[string]
	LineSpacing, SpaceBefore, SpaceAfter Opt[Spacing]
	RTL                                  Opt[bool]
	Bullet                               Opt[Bullet]
	// BulletColor is the color of the bullet. An unset value or a nil one
	// takes the color of the text.
	BulletColor Opt[*RGBA]
	// BulletSize is the size of the bullet. An unset value or a nil one
	// takes the size of the text.
	BulletSize Opt[*BulletSize]
	// BulletFont is the typeface of the bullet. An unset value or an empty
	// one takes the typeface of the text.
	BulletFont Opt[string]
	// Run is the style of the runs of the paragraph (a:defRPr).
	Run RunStyle
}

// Over sets each field of s that o sets.
func (s *ParaStyle) Over(o *ParaStyle) {
	s.MarL.over(o.MarL)
	s.MarR.over(o.MarR)
	s.Indent.over(o.Indent)
	s.Align.over(o.Align)
	s.LineSpacing.over(o.LineSpacing)
	s.SpaceBefore.over(o.SpaceBefore)
	s.SpaceAfter.over(o.SpaceAfter)
	s.RTL.over(o.RTL)
	s.Bullet.over(o.Bullet)
	s.BulletColor.over(o.BulletColor)
	s.BulletSize.over(o.BulletSize)
	s.BulletFont.over(o.BulletFont)
	s.Run.Over(&o.Run)
}

// ListStyle is a list style of DrawingML, such as a:lstStyle or
// p:titleStyle: the style of the paragraphs of each level. Index 0 is
// a:defPPr, and 1 to 9 are a:lvl1pPr to a:lvl9pPr.
type ListStyle [10]ParaStyle

// Over sets each field of each level of s that o sets.
func (s *ListStyle) Over(o *ListStyle) {
	if o == nil {
		return
	}
	for i := range s {
		s[i].Over(&o[i])
	}
}

// Level returns the style of a paragraph of the level, from 0: a:defPPr
// with the style of the level over it.
func (s *ListStyle) Level(lvl int) ParaStyle {
	lvl = min(max(lvl, 0), 8)
	p := s[0]
	p.Over(&s[lvl+1])
	return p
}

// BodyStyle is the style of a text body, from a:bodyPr.
type BodyStyle struct {
	// Insets are the left, top, right and bottom insets, in points.
	Insets [4]Opt[float64]
	// Anchor is "t", "ctr", "b", "just" or "dist".
	Anchor Opt[string]
	// AnchorCenter centers the text across.
	AnchorCenter Opt[bool]
	// Wrap wraps the lines at the width of the box.
	Wrap Opt[bool]
	// Vert is the direction of the text, such as "horz", "vert" or
	// "vert270".
	Vert Opt[string]
	// Rotation rotates the text, in degrees.
	Rotation Opt[float64]
	// Upright keeps the text upright in a shape that rotates.
	Upright Opt[bool]
	// FontScale and LineReduction are the fractions of a:normAutofit that
	// shrink the text.
	FontScale, LineReduction Opt[float64]
	// Columns is the number of the columns, and ColumnGap the space
	// between them in points.
	Columns   Opt[int]
	ColumnGap Opt[float64]
}

// Over sets each field of s that o sets.
func (s *BodyStyle) Over(o *BodyStyle) {
	for i := range s.Insets {
		s.Insets[i].over(o.Insets[i])
	}
	s.Anchor.over(o.Anchor)
	s.AnchorCenter.over(o.AnchorCenter)
	s.Wrap.over(o.Wrap)
	s.Vert.over(o.Vert)
	s.Rotation.over(o.Rotation)
	s.Upright.over(o.Upright)
	s.FontScale.over(o.FontScale)
	s.LineReduction.over(o.LineReduction)
	s.Columns.over(o.Columns)
	s.ColumnGap.over(o.ColumnGap)
}

// TextBody is a body of text with the styles of its paragraphs and runs
// resolved.
type TextBody struct {
	Body       BodyStyle
	Paragraphs []Paragraph
	// NoKerning draws the text without the kerning of its fonts, as
	// PowerPoint draws its slides, except the runs that RunStyle.Kern
	// kerns.
	NoKerning bool
}

// Empty reports whether the body holds no text to draw.
func (tb *TextBody) Empty() bool {
	if tb == nil {
		return true
	}
	for _, p := range tb.Paragraphs {
		for _, r := range p.Runs {
			if r.Text != "" {
				return false
			}
		}
	}
	return true
}

// Paragraph is a paragraph of a text body.
type Paragraph struct {
	// Style is the style of the paragraph, with the styles that it
	// inherits.
	Style ParaStyle
	// Level is the level of the paragraph in a list, from 0.
	Level int
	Runs  []Run
	// End is the style of the end of the paragraph, which gives the height
	// of an empty paragraph.
	End RunStyle
	// Number is the number of an automatic bullet, which the text body
	// counts.
	Number int
}

// Run is a run of text with one style, or a line break.
type Run struct {
	Text  string
	Break bool
	Style RunStyle
}

// Fields gives the text of the fields of a text body, such as a slide
// number.
type Fields struct {
	// SlideNumber is the number of the slide, from 1, or 0 to keep the text
	// of the field.
	SlideNumber int
}

// ParseRunStyle reads a:rPr, a:defRPr or a:endParaRPr.
func (c *Colors) ParseRunStyle(n roxmltree.Node) RunStyle {
	var s RunStyle
	if v, ok := number(attr(n, "sz", "")); ok && v > 0 {
		s.Size.Set(min(v/100, 4000))
	}
	if v, ok := boolAttr(n, "b"); ok {
		s.Bold.Set(v)
	}
	if v, ok := boolAttr(n, "i"); ok {
		s.Italic.Set(v)
	}
	if u := attr(n, "u", ""); u != "" {
		s.Underline.Set(u != "none")
	}
	if v := attr(n, "strike", ""); v != "" {
		s.Strike.Set(v != "noStrike")
	}
	if v := attr(n, "cap", ""); v != "" {
		s.Caps.Set(v)
	}
	if v := attr(n, "baseline", ""); v != "" {
		s.Baseline.Set(pct(numberAttr(n, "baseline", 0)))
	}
	if v, ok := number(attr(n, "spc", "")); ok {
		s.Spacing.Set(min(max(v/100, -100), 100))
	}
	if v, ok := number(attr(n, "kern", "")); ok && v >= 0 {
		s.Kern.Set(min(v/100, 4000))
	}

	for _, a := range [...]string{"lang", "altLang"} {
		if v := attr(n, a, ""); fontdb.ParseLang(v) != fontdb.NoLang {
			s.Lang.Set(v)
			break
		}
	}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		if pt, ok := c.PaintOf(e, nil); ok {
			s.Paint.Set(pt)
			continue
		}
		switch e.TagName().Name() {
		case "latin":
			s.Latin.Set(attr(e, "typeface", ""))
		case "ea":
			s.EastAsian.Set(attr(e, "typeface", ""))
		case "cs":
			s.ComplexScript.Set(attr(e, "typeface", ""))
		case "sym":
			s.Symbol.Set(attr(e, "typeface", ""))
		case "highlight":
			if v, ok := c.In(e); ok {
				s.Highlight.Set(v)
			}
		case "hlinkClick":
			s.Link.Set(true)
		}
	}
	if s.Link.Or(false) {
		// a hyperlink without a color of its own takes the color hlink of
		// the theme, and an underline, as PowerPoint draws it
		if v, ok := c.linkColor(); ok && !s.Paint.OK {
			s.Paint.Set(Solid(v))
		}
		if !s.Underline.OK {
			s.Underline.Set(true)
		}
	}
	return s
}

// boolAttr returns the attribute of n with the name as an xsd:boolean.
func boolAttr(n roxmltree.Node, name string) (bool, bool) {
	switch attr(n, name, "") {
	case "1", "true":
		return true, true
	case "0", "false":
		return false, true
	}
	return false, false
}

// ParseParaStyle reads a:pPr or a level of a list style, such as
// a:lvl1pPr.
func (c *Colors) ParseParaStyle(n roxmltree.Node) ParaStyle {
	var s ParaStyle
	if v, ok := number(attr(n, "marL", "")); ok {
		s.MarL.Set(clampPt(emu(v)))
	}
	if v, ok := number(attr(n, "marR", "")); ok {
		s.MarR.Set(clampPt(emu(v)))
	}
	if v, ok := number(attr(n, "indent", "")); ok {
		s.Indent.Set(clampPt(emu(v)))
	}
	if v := attr(n, "algn", ""); v != "" {
		s.Align.Set(v)
	}
	if v, ok := boolAttr(n, "rtl"); ok {
		s.RTL.Set(v)
	}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "lnSpc":
			if v, ok := parseSpacing(e); ok {
				s.LineSpacing.Set(v)
			}
		case "spcBef":
			if v, ok := parseSpacing(e); ok {
				s.SpaceBefore.Set(v)
			}
		case "spcAft":
			if v, ok := parseSpacing(e); ok {
				s.SpaceAfter.Set(v)
			}
		case "buNone":
			s.Bullet.Set(Bullet{Kind: BulletNone})
		case "buChar":
			s.Bullet.Set(Bullet{Kind: BulletChar, Char: attr(e, "char", "•")})
		case "buAutoNum":
			start := int(numberAttr(e, "startAt", 1))
			s.Bullet.Set(Bullet{Kind: BulletNumber, Scheme: attr(e, "type", "arabicPeriod"), Start: min(max(start, 1), 32767)})
		case "buBlip":
			s.Bullet.Set(Bullet{Kind: BulletPicture})
		case "buClrTx":
			s.BulletColor.Set(nil)
		case "buClr":
			if v, ok := c.In(e); ok {
				s.BulletColor.Set(&v)
			}
		case "buSzTx":
			s.BulletSize.Set(nil)
		case "buSzPct":
			s.BulletSize.Set(&BulletSize{Pct: min(max(pct(numberAttr(e, "val", 100000)), 0.25), 4)})
		case "buSzPts":
			s.BulletSize.Set(&BulletSize{Points: true, Pt: min(max(numberAttr(e, "val", 0)/100, 1), 4000)})
		case "buFontTx":
			s.BulletFont.Set("")
		case "buFont":
			s.BulletFont.Set(attr(e, "typeface", ""))
		case "defRPr":
			s.Run = c.ParseRunStyle(e)
		}
	}
	return s
}

// clampPt limits a length in points to a size that a page can hold.
func clampPt(v float64) float64 {
	return min(max(v, -10000), 10000)
}

// parseSpacing reads a:lnSpc, a:spcBef or a:spcAft.
func parseSpacing(n roxmltree.Node) (Spacing, bool) {
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "spcPct":
			return Spacing{Pct: min(max(pct(numberAttr(e, "val", 100000)), 0), 100)}, true
		case "spcPts":
			return Spacing{Points: true, Pt: min(max(numberAttr(e, "val", 0)/100, 0), 4000)}, true
		}
	}
	return Spacing{}, false
}

// ParseListStyle reads a list style, such as a:lstStyle, p:titleStyle or
// a:defaultTextStyle. A nil node gives the empty list style.
func (c *Colors) ParseListStyle(n roxmltree.Node) *ListStyle {
	var s ListStyle
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		name := e.TagName().Name()
		switch {
		case name == "defPPr":
			s[0] = c.ParseParaStyle(e)
		case strings.HasPrefix(name, "lvl") && strings.HasSuffix(name, "pPr"):
			i, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "lvl"), "pPr"))
			if err == nil && i >= 1 && i <= 9 {
				s[i] = c.ParseParaStyle(e)
			}
		}
	}
	return &s
}

// ParseBodyStyle reads a:bodyPr.
func ParseBodyStyle(n roxmltree.Node) BodyStyle {
	var s BodyStyle
	for i, name := range [...]string{"lIns", "tIns", "rIns", "bIns"} {
		if v, ok := number(attr(n, name, "")); ok {
			s.Insets[i].Set(min(max(emu(v), 0), 10000))
		}
	}
	if v := attr(n, "anchor", ""); v != "" {
		s.Anchor.Set(v)
	}
	if v, ok := boolAttr(n, "anchorCtr"); ok {
		s.AnchorCenter.Set(v)
	}
	if v := attr(n, "wrap", ""); v != "" {
		s.Wrap.Set(v != "none")
	}
	if v := attr(n, "vert", ""); v != "" {
		s.Vert.Set(v)
	}
	if v, ok := number(attr(n, "rot", "")); ok {
		s.Rotation.Set(v / 60000)
	}
	if v, ok := boolAttr(n, "upright"); ok {
		s.Upright.Set(v)
	}
	if v, ok := number(attr(n, "numCol", "")); ok && v >= 1 {
		s.Columns.Set(int(min(v, 16)))
	}
	if v, ok := number(attr(n, "spcCol", "")); ok {
		s.ColumnGap.Set(min(max(emu(v), 0), 10000))
	}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "normAutofit":
			s.FontScale.Set(min(max(pct(numberAttr(e, "fontScale", 100000)), 0.01), 1))
			s.LineReduction.Set(min(max(pct(numberAttr(e, "lnSpcReduction", 0)), 0), 0.9))
		case "noAutofit", "spAutoFit":
			s.FontScale.Set(1)
			s.LineReduction.Set(0)
		}
	}
	return s
}

// ParseTextBody reads the paragraphs of the text body n, such as a:txBody
// or p:txBody. list is the list style that the body inherits. run is the
// style of the text over it, such as the font of a:fontRef, or nil. The
// list style of n goes over both, and the styles of the paragraphs and the
// runs come from them.
func (c *Colors) ParseTextBody(n roxmltree.Node, list *ListStyle, run *RunStyle, body BodyStyle, fields Fields) *TextBody {
	tb := &TextBody{Body: body, NoKerning: true}
	styles := *list
	if run != nil {
		for i := range styles {
			styles[i].Run.Over(run)
		}
	}
	if e, ok := child(n, "lstStyle"); ok {
		styles.Over(c.ParseListStyle(e))
	}
	size := 0
	for p := range n.Children() {
		if !p.IsElement() || p.TagName().Name() != "p" {
			continue
		}
		if len(tb.Paragraphs) >= maxParagraphs || size > maxTextBytes {
			break
		}
		para := c.parseParagraph(p, &styles, fields, &size)
		tb.Paragraphs = append(tb.Paragraphs, para)
	}
	numberParagraphs(tb.Paragraphs)
	return tb
}

// parseParagraph reads a:p.
func (c *Colors) parseParagraph(n roxmltree.Node, list *ListStyle, fields Fields, size *int) Paragraph {
	var own ParaStyle
	lvl := 0
	if e, ok := child(n, "pPr"); ok {
		own = c.ParseParaStyle(e)
		lvl = int(min(max(numberAttr(e, "lvl", 0), 0), 8))
	}
	style := list.Level(lvl)
	style.Over(&own)
	para := Paragraph{Style: style, Level: lvl, End: style.Run}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		if len(para.Runs) >= maxRuns {
			break
		}
		var text string
		var rPr roxmltree.Node
		var hasPr bool
		switch e.TagName().Name() {
		case "r":
			rPr, hasPr = child(e, "rPr")
			if t, ok := child(e, "t"); ok {
				text = nodeText(t)
			}
		case "fld":
			rPr, hasPr = child(e, "rPr")
			if t, ok := child(e, "t"); ok {
				text = nodeText(t)
			}
			if attr(e, "type", "") == "slidenum" && fields.SlideNumber > 0 {
				text = strconv.Itoa(fields.SlideNumber)
			}
		case "br":
			rPr, hasPr = child(e, "rPr")
			r := Run{Break: true, Style: style.Run}
			if hasPr {
				own := c.ParseRunStyle(rPr)
				r.Style.Over(&own)
			}
			para.Runs = append(para.Runs, r)
			continue
		case "endParaRPr":
			own := c.ParseRunStyle(e)
			para.End.Over(&own)
			continue
		default:
			continue
		}
		*size += len(text)
		r := Run{Text: text, Style: style.Run}
		if hasPr {
			own := c.ParseRunStyle(rPr)
			r.Style.Over(&own)
		}
		para.Runs = append(para.Runs, r)
	}
	return para
}

// nodeText returns the text of the element n.
func nodeText(n roxmltree.Node) string {
	var b strings.Builder
	for c := range n.Children() {
		if c.IsText() {
			t, _ := c.Text()
			b.WriteString(t)
		}
	}
	return b.String()
}

// numberParagraphs counts the numbers of the paragraphs with automatic
// bullets. A number goes on while the paragraphs at the same level have
// the same scheme. A paragraph at a lower level ends the lists below it.
func numberParagraphs(paras []Paragraph) {
	var counts [9]int
	var schemes [9]string
	for i := range paras {
		p := &paras[i]
		lvl := min(max(p.Level, 0), 8)
		for j := lvl + 1; j < len(counts); j++ {
			counts[j], schemes[j] = 0, ""
		}
		b := p.Style.Bullet.V
		if !p.Style.Bullet.OK || b.Kind != BulletNumber {
			if !paragraphEmpty(p) {
				counts[lvl], schemes[lvl] = 0, ""
			}
			continue
		}
		if schemes[lvl] != b.Scheme {
			counts[lvl], schemes[lvl] = 0, b.Scheme
		}
		counts[lvl]++
		p.Number = b.Start + counts[lvl] - 1
	}
}

// paragraphEmpty reports whether a paragraph holds no text.
func paragraphEmpty(p *Paragraph) bool {
	for _, r := range p.Runs {
		if r.Text != "" {
			return false
		}
	}
	return true
}

// linkColor returns the color hlink of the theme, for a hyperlink.
func (c *Colors) linkColor() (RGBA, bool) {
	if c == nil || c.Theme == nil {
		return RGBA{}, false
	}
	return c.scheme("hlink")
}
