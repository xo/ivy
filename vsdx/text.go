package vsdx

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/ooxml"
)

// maxTextBytes bounds the text of a shape that the package reads.
const maxTextBytes = 1 << 20

// textRun is a run of the text of a shape, with the index of its row of the
// section Character.
type textRun struct {
	text string
	cp   string
	// brk reports a line break inside a paragraph.
	brk bool
}

// textPara is a paragraph of the text of a shape, with the index of its
// row of the section Paragraph.
type textPara struct {
	pp   string
	runs []textRun
}

// textOf returns the element Text of the shape, or of the master that it
// inherits it from.
func (b *builder) textOf(s *shape) (roxmltree.Node, bool) {
	for depth := 0; s != nil && depth <= b.lim.MaxDepth; depth++ {
		if s.hasText {
			return s.text, true
		}
		s = b.masterOf(s)
	}
	return roxmltree.Node{}, false
}

// splitText reads the element Text into paragraphs: its text, its marks of
// the rows of the sections Character (cp) and Paragraph (pp), and its
// fields (fld).
func (b *builder) splitText(s *shape, n roxmltree.Node) []textPara {
	paras := []textPara{{pp: "0"}}
	cp := "0"
	size := 0
	fields := b.chain(s, catOther)
	add := func(text string) {
		for text != "" && size < maxTextBytes {
			size += len(text)
			i := strings.IndexFunc(text, func(r rune) bool {
				return r == '\n' || r == '\r' || r == '\u2028' || r == '\u2029'
			})
			part := text
			if i >= 0 {
				part = text[:i]
			}
			p := &paras[len(paras)-1]
			if part != "" {
				p.runs = append(p.runs, textRun{text: part, cp: cp})
			}
			if i < 0 {
				return
			}
			r, n := utf8.DecodeRuneInString(text[i:])
			switch r {
			case '\u2028':
				p.runs = append(p.runs, textRun{brk: true, cp: cp})
			case '\r':
			default:
				paras = append(paras, textPara{pp: p.pp})
			}
			text = text[i+n:]
		}
	}
	for c := range n.Children() {
		if c.IsText() {
			t, _ := c.Text()
			add(t)
			continue
		}
		if !c.IsElement() {
			continue
		}
		switch c.TagName().Name() {
		case "cp":
			cp = ooxml.Attr(c, "IX")
		case "pp":
			paras[len(paras)-1].pp = ooxml.Attr(c, "IX")
		case "fld":
			add(b.fieldText(fields, c))
		}
	}
	// the text ends with a paragraph mark, which starts no paragraph
	if last := paras[len(paras)-1]; len(last.runs) == 0 && len(paras) > 1 {
		paras = paras[:len(paras)-1]
	}
	return paras
}

// fieldText returns the text of the field fld: its text, or the value of
// its row of the section Field, with the page number and the page count.
func (b *builder) fieldText(c chain, fld roxmltree.Node) string {
	var text strings.Builder
	for t := range fld.Children() {
		if t.IsText() {
			v, _ := t.Text()
			text.WriteString(v)
		}
	}
	ix := ooxml.Attr(fld, "IX")
	for _, r := range c.rows("Field", maxRows) {
		if r.key != ix {
			continue
		}
		v, _, _ := r.cell("Value")
		f := strings.ToUpper(v.f)
		switch {
		case strings.Contains(f, "PAGENUMBER"):
			return strconv.Itoa(b.pageNumber)
		case strings.Contains(f, "PAGECOUNT"):
			return strconv.Itoa(b.pageCount)
		case strings.Contains(f, "PAGENAME"):
			return b.pageName
		}
		if text.Len() == 0 {
			return v.v
		}
	}
	return text.String()
}

// rowCell returns the cell of the row r with the name, or of the row
// first when r does not give it.
func rowCell(r, first *rowView, name string) (cell, bool) {
	if r != nil {
		if c, _, ok := r.cell(name); ok {
			return c, true
		}
	}
	if first != nil {
		if c, _, ok := first.cell(name); ok {
			return c, true
		}
	}
	return cell{}, false
}

// runStyle returns the style of a run from the row r of the section
// Character, with the row first under it.
func (b *builder) runStyle(r, first *rowView, q quickStyle) drawingml.RunStyle {
	var st drawingml.RunStyle
	get := func(name string) string {
		c, _ := rowCell(r, first, name)
		return c.v
	}
	st.Size.Set(min(max(parseNum(get("Size"), 1.0/6)*72, 1), 4000))
	switch font := get("Font"); font {
	case themed, "":
		// Visio 2013 draws a themed font without a theme in Calibri
		st.Latin.Set("Calibri")
		if b.theme != nil {
			st.Latin.Set("+mn-lt")
		}
	default:
		if _, err := strconv.Atoi(font); err == nil {
			font = b.dr.faces[font]
		}
		st.Latin.Set(font)
	}
	themeColor, themeStyle, _ := b.themeFont(q)
	col := drawingml.Black
	switch v := get("Color"); v {
	case themed:
		col = themeColor
	default:
		if c, ok := b.color(v); ok {
			col = c
		}
	}
	if t := parseNum(get("ColorTrans"), 0); t > 0 {
		col = col.WithAlpha(1 - min(t, 1))
	}
	st.Paint.Set(drawingml.Solid(col))
	style := int(parseNum(get("Style"), 0))
	if get("Style") == themed {
		style = themeStyle
	}
	st.Bold.Set(style&1 != 0)
	st.Italic.Set(style&2 != 0)
	st.Underline.Set(style&4 != 0 || isTrue(get("DblUnderline")))
	if style&8 != 0 {
		st.Caps.Set("small")
	}
	if get("Case") == "1" {
		st.Caps.Set("all")
	}
	switch get("Pos") {
	case "1":
		st.Baseline.Set(0.33)
	case "2":
		st.Baseline.Set(-0.25)
	}
	st.Strike.Set(isTrue(get("Strikethru")) || isTrue(get("DoubleStrikethrough")))
	if v := parseNum(get("Letterspace"), 0); v != 0 {
		st.Spacing.Set(min(max(v*72, -100), 100))
	}
	return st
}

// paraStyle returns the style of a paragraph from the row r of the
// section Paragraph, with the row first under it.
func paraStyle(r, first *rowView) drawingml.ParaStyle {
	var st drawingml.ParaStyle
	get := func(name string) string {
		c, _ := rowCell(r, first, name)
		return c.v
	}
	pts := func(name string) float64 {
		return min(max(parseNum(get(name), 0)*72, -10000), 10000)
	}
	st.MarL.Set(pts("IndLeft"))
	st.MarR.Set(pts("IndRight"))
	st.Indent.Set(pts("IndFirst"))
	switch get("HorzAlign") {
	case "0":
		st.Align.Set("l")
	case "2":
		st.Align.Set("r")
	case "3":
		st.Align.Set("just")
	case "4":
		st.Align.Set("dist")
	default:
		st.Align.Set("ctr")
	}
	switch v := parseNum(get("SpLine"), -1.2); {
	case v < 0:
		st.LineSpacing.Set(drawingml.Spacing{Pct: min(-v/1.2, 100)})
	case v > 0:
		st.LineSpacing.Set(drawingml.Spacing{Points: true, Pt: min(v*72, 4000)})
	}
	if v := pts("SpBefore"); v > 0 {
		st.SpaceBefore.Set(drawingml.Spacing{Points: true, Pt: v})
	}
	if v := pts("SpAfter"); v > 0 {
		st.SpaceAfter.Set(drawingml.Spacing{Points: true, Pt: v})
	}
	if v := get("Bullet"); v != "" && v != "0" {
		char := get("BulletStr")
		if char == "" {
			char = "•"
		}
		st.Bullet.Set(drawingml.Bullet{Kind: drawingml.BulletChar, Char: char})
	}
	return st
}

// textBody returns the text of the shape as a text body of DrawingML, or
// nil for a shape without text.
func (b *builder) textBody(s *shape, q quickStyle) *drawingml.TextBody {
	if v, _ := b.value(s, "HideText"); isTrue(v) {
		return nil
	}
	n, ok := b.textOf(s)
	if !ok {
		return nil
	}
	paras := b.splitText(s, n)
	c := b.chain(s, catText)
	chars := rowsByKey(c.rows("Character", maxRows))
	pps := rowsByKey(c.rows("Paragraph", maxRows))
	tb := &drawingml.TextBody{}
	for i, name := range [...]string{"LeftMargin", "TopMargin", "RightMargin", "BottomMargin"} {
		tb.Body.Insets[i].Set(min(max(b.num(s, name, 0)*72, 0), 1000))
	}
	switch b.num(s, "VerticalAlign", 1) {
	case 0:
		tb.Body.Anchor.Set("t")
	case 2:
		tb.Body.Anchor.Set("b")
	default:
		tb.Body.Anchor.Set("ctr")
	}
	if b.num(s, "TextDirection", 0) == 1 {
		tb.Body.Vert.Set("vert")
	}
	if c, _, ok := b.chain(s, catOther).cell("TxtWidth"); ok && strings.Contains(strings.ToUpper(c.f), "TEXTWIDTH") {
		// the block is as wide as its text in the fonts of Visio, and the
		// text keeps its lines in other fonts
		tb.Body.Wrap.Set(false)
	}
	bkgnd, hasBkgnd := b.textBackground(s)
	for _, p := range paras {
		para := drawingml.Paragraph{Style: paraStyle(pps[p.pp], pps["0"])}
		end := b.runStyle(nil, chars["0"], q)
		para.End = end
		for _, r := range p.runs {
			st := b.runStyle(chars[r.cp], chars["0"], q)
			if hasBkgnd {
				st.Highlight.Set(bkgnd)
			}
			if len(para.Runs) == 0 {
				para.End = st
			}
			para.Runs = append(para.Runs, drawingml.Run{Text: r.text, Break: r.brk, Style: st})
		}
		para.Style.Run = para.End
		tb.Paragraphs = append(tb.Paragraphs, para)
	}
	return tb
}

// textBackground returns the color behind the text of the shape, from the
// cells TextBkgnd and TextBkgndTrans. The color is behind the lines of the
// text, not behind the whole block. A value of 0 is no color, and an index
// n is the color n-1 of the color table.
func (b *builder) textBackground(s *shape) (drawingml.RGBA, bool) {
	v, _ := b.value(s, "TextBkgnd")
	if v == "" || v == "0" || v == themed {
		return drawingml.RGBA{}, false
	}
	if n, err := strconv.Atoi(v); err == nil {
		v = strconv.Itoa(n - 1)
	}
	c, ok := b.color(v)
	if !ok {
		return drawingml.RGBA{}, false
	}
	if t := b.num(s, "TextBkgndTrans", 0); t > 0 {
		c = c.WithAlpha(1 - min(t, 1))
	}
	return c, c.A > 0
}

// rowsByKey maps the key of each row to the row.
func rowsByKey(rows []*rowView) map[string]*rowView {
	m := make(map[string]*rowView, len(rows))
	for _, r := range rows {
		m[r.key] = r
	}
	return m
}

// textFrame returns the transform of the text block of the shape, from the
// cells TxtPinX, TxtPinY, TxtWidth, TxtHeight, TxtLocPinX, TxtLocPinY and
// TxtAngle, and its box in points. m maps the coordinates of the shape onto
// the page. The text does not mirror with the shape.
func (b *builder) textFrame(s *shape, m affine, w, h float64) (okage.Transform, okage.Rect, bool) {
	tw := b.num(s, "TxtWidth", w)
	th := b.num(s, "TxtHeight", h)
	if !(tw > 0) {
		tw = w
	}
	if !(th > 0) {
		// the text of a block without height overflows it
		th = 1e-3
	}
	px, py := b.num(s, "TxtPinX", w/2), b.num(s, "TxtPinY", h/2)
	lx, ly := b.num(s, "TxtLocPinX", tw/2), b.num(s, "TxtLocPinY", th/2)
	angle := b.num(s, "TxtAngle", 0)
	k := b.k
	t := m.mul(translate(px, py)).mul(rotate(angle)).mul(translate(-lx, -ly)).mul(affine{a: 1 / k, d: -1 / k, f: th})
	if t.det() < 0 {
		t = t.mul(affine{a: -1, d: 1, e: tw * k})
	}
	if !t.finite() || !(tw*k > 0) || !(th*k > 0) || math.IsInf(tw*k, 0) || math.IsInf(th*k, 0) {
		return okage.Transform{}, okage.Rect{}, false
	}
	r, ok := okage.RectFromXYWH(0, 0, float32(min(tw*k, 1e6)), float32(min(th*k, 1e6)))
	return t.okage(), r, ok
}
