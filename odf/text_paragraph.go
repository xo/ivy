package odf

import (
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/chart"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/odf/internal/drawpage"
)

// odtDecl is a declaration of CSS.
type odtDecl struct {
	name, value string
}

// odtColor returns a color of OpenDocument, "#rrggbb", or "" for another
// value, such as "transparent".
func odtColor(s string) string {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return ""
	}
	if _, err := strconv.ParseUint(s[1:], 16, 32); err != nil {
		return ""
	}
	return s
}

// odtGenerics maps the generic families of OpenDocument to those of CSS.
var odtGenerics = map[string]string{
	"roman": "serif", "swiss": "sans-serif", "modern": "monospace",
	"decorative": "fantasy", "script": "cursive", "system": "sans-serif",
}

// family returns the value of font-family of the properties, or "". The
// East Asian font comes after the font of the Latin text, as in docx, so
// that it draws the East Asian characters that the first font does not
// hold.
func (c *odtConverter) family(p odtProps) string {
	family, generic := c.st.font(p)
	asian := c.st.asianFont(p)
	if family == "" {
		if asian == "" {
			return ""
		}
		return odtQuote(asian) + ", serif"
	}
	if asian != "" && !strings.EqualFold(asian, family) {
		return odtQuote(family) + ", " + odtQuote(asian) + ", " + odtGeneric(family, generic)
	}
	return odtQuote(family) + ", " + odtGeneric(family, generic)
}

// odtGeneric returns the generic family of CSS of the font family, from its
// generic family of OpenDocument or from its name.
func odtGeneric(family, generic string) string {
	g := odtGenerics[generic]
	if g == "" {
		g = "serif"
		lower := strings.ToLower(family)
		for _, k := range [...]string{"sans", "arial", "calibri", "carlito", "helvetica", "verdana"} {
			if strings.Contains(lower, k) {
				g = "sans-serif"
			}
		}
		if strings.Contains(lower, "mono") || strings.Contains(lower, "courier") {
			g = "monospace"
		}
	}
	return g
}

// odtTextPosition returns the vertical align and the scale of the size of
// style:text-position, such as "super 58%".
func odtTextPosition(v string) (string, float64) {
	f := strings.Fields(v)
	if len(f) == 0 {
		return "", 1
	}
	scale := 1.0
	if len(f) > 1 {
		if pct, err := strconv.ParseFloat(strings.TrimSuffix(f[1], "%"), 64); err == nil && pct > 0 && pct <= 100 {
			scale = pct / 100
		}
	}
	switch f[0] {
	case "super":
		return "super", scale
	case "sub":
		return "sub", scale
	case "0%", "0":
		return "", scale
	}
	if pct, err := strconv.ParseFloat(strings.TrimSuffix(f[0], "%"), 64); err == nil {
		switch {
		case pct > 0:
			return "super", scale
		case pct < 0:
			return "sub", scale
		}
	}
	return "", scale
}

// inherited returns the declarations of the text properties that a child
// element inherits.
func (c *odtConverter) inherited(p odtProps) []odtDecl {
	size := odtFontSize(p)
	if va, scale := odtTextPosition(p["text-position"]); va != "" {
		size *= scale
	}
	weight := "normal"
	switch w := p["font-weight"]; w {
	case "bold", "100", "200", "300", "400", "500", "600", "700", "800", "900":
		weight = w
	}
	style := "normal"
	if s := p["font-style"]; s == "italic" || s == "oblique" {
		style = "italic"
	}
	color := odtColor(p["color"])
	if color == "" {
		color = "#000000"
	}
	transform := "none"
	if t := p["text-transform"]; t == "uppercase" || t == "lowercase" || t == "capitalize" {
		transform = t
	}
	variant := "normal"
	if p["font-variant"] == "small-caps" {
		variant = "small-caps"
	}
	spacing := "normal"
	if v, ok := odtLength(p["letter-spacing"]); ok {
		spacing = odtPt(v)
	}
	return []odtDecl{
		{"font-family", c.family(p)},
		{"font-size", odtPt(size)},
		{"font-weight", weight},
		{"font-style", style},
		{"color", color},
		{"text-transform", transform},
		{"font-variant", variant},
		{"letter-spacing", spacing},
	}
}

// odtOwn returns the declarations of the text properties that a child element
// does not inherit.
func odtOwn(p odtProps) []odtDecl {
	var out []odtDecl
	var lines []string
	if u := p["text-underline-style"]; u != "" && u != "none" {
		lines = append(lines, "underline")
	}
	if l := p["text-line-through-style"]; l != "" && l != "none" {
		lines = append(lines, "line-through")
	}
	if len(lines) > 0 {
		out = append(out, odtDecl{"text-decoration-line", strings.Join(lines, " ")})
	}
	if v := odtColor(p["text-background-color"]); v != "" {
		out = append(out, odtDecl{"background-color", v})
	}
	if va, _ := odtTextPosition(p["text-position"]); va != "" {
		out = append(out, odtDecl{"vertical-align", va})
	}
	return out
}

// spanStyle returns the style attribute of a span with the properties p in
// a parent with the properties parent.
func (c *odtConverter) spanStyle(p, parent odtProps) string {
	var s odtCSS
	have := map[string]string{}
	for _, d := range c.inherited(parent) {
		have[d.name] = d.value
	}
	for _, d := range c.inherited(p) {
		if have[d.name] != d.value {
			s.set(d.name, d.value)
		}
	}
	for _, d := range odtOwn(p) {
		s.set(d.name, d.value)
	}
	return s.String()
}

// odtBox writes the borders, the paddings and the background of the
// properties p.
func odtBox(s *odtCSS, p odtProps) bool {
	boxed := false
	if v := odtSafeValue(p["border"]); v != "" && v != "none" {
		s.set("border", v)
		boxed = true
	}
	for _, side := range [...]string{"top", "right", "bottom", "left"} {
		if v := odtSafeValue(p["border-"+side]); v != "" {
			s.set("border-"+side, v)
			boxed = boxed || v != "none"
		}
	}
	if v, ok := odtLength(p["padding"]); ok && v > 0 {
		s.set("padding", odtPt(v))
	}
	for _, side := range [...]string{"top", "right", "bottom", "left"} {
		if v, ok := odtLength(p["padding-"+side]); ok && v >= 0 {
			s.set("padding-"+side, odtPt(v))
		}
	}
	if v := odtColor(p["background-color"]); v != "" {
		s.set("background-color", v)
		boxed = true
	}
	return boxed
}

// indents returns the margin at the left and the indent of the first line
// of a paragraph of the style name with the properties p, in the item of a
// list list, which can be nil. The indents of a level of a list apply, but
// in the mode label-alignment, an indent that the automatic style of the
// paragraph sets wins, as the direct formatting of LibreOffice does.
func (c *odtConverter) indents(name string, p odtProps, list *odtList) (left, indent float64) {
	left, _ = odtLength(p["margin-left"])
	indent, _ = odtLength(p["text-indent"])
	if list == nil || list.lvl == nil || !list.lvl.hasLabelPosition {
		return left, indent
	}
	var own odtProps
	if st, ok := c.st.styles["paragraph/"+name]; ok && st.auto && list.lvl.labelAlignment {
		own = st.props
	}
	if _, ok := own["margin-left"]; !ok {
		left = list.lvl.marginLeft
	}
	if _, ok := own["text-indent"]; !ok {
		indent = list.lvl.textIndent
	}
	return left, indent
}

// paragraphStyle returns the style attribute of a paragraph with the
// properties p, the margin at the left left and the indent of the first
// line indent.
func (c *odtConverter) paragraphStyle(p odtProps, left, indent float64) string {
	var s odtCSS
	for _, d := range c.inherited(p) {
		s.set(d.name, d.value)
	}
	odtBox(&s, p)
	right, _ := odtLength(p["margin-right"])
	if left != 0 {
		s.set("margin-left", odtPt(left))
	}
	if right != 0 {
		s.set("margin-right", odtPt(right))
	}
	if indent != 0 {
		s.set("text-indent", odtPt(indent))
	}
	// LibreOffice adds the spacing below a paragraph and the spacing above
	// the next one. Both are margins. The margins of CSS collapse to the
	// larger one, so the margin above holds the two spacings, which the
	// spacing below the paragraph before cannot pass. LibreOffice leaves
	// out the spacing above a paragraph at the top of a page, except on the
	// first page and after a page break, and the spacing below a paragraph
	// at the bottom of a page. The layout discards the margins at a break
	// (margin-break), and keeps them after a page break.
	top, _ := odtLength(p["margin-top"])
	bottom, _ := odtLength(p["margin-bottom"])
	forced := c.forced
	if v := p["break-before"]; v == "page" || v == "column" {
		forced = true
	}
	if top > 0 {
		before := c.lastBottom
		if forced {
			before = 0
		}
		s.set("margin-top", odtPt(top+max(before, 0)))
		if forced {
			s.set("margin-break", "keep")
		}
	}
	if bottom > 0 {
		s.set("margin-bottom", odtPt(bottom))
	}
	family, _ := c.st.font(p)
	size := odtFontSize(p)
	ratio := odtLineRatio(family, size, c.tight)
	switch lh := p["line-height"]; {
	case strings.HasSuffix(lh, "%"):
		if pct, err := strconv.ParseFloat(strings.TrimSuffix(lh, "%"), 64); err == nil && pct > 0 {
			// LibreOffice takes the percent of the height of a line in
			// twips, and drops the fraction of a twip
			h := math.Floor(ratio*size*20*min(pct/100, 10)+1e-6) / 20
			s.set("line-height", odtRatio(h/size))
			// LibreOffice puts the extra space of the lines below their
			// text
			if pct > 100 {
				s.set("-ivy-line-multiple", odtRatio(min(pct/100, 10)))
			}
		}
	default:
		if v, ok := odtLength(lh); ok && v > 0 {
			s.set("line-height", odtPt(v))
		} else if v, ok := odtLength(p["line-height-at-least"]); ok && v > ratio*size {
			s.set("line-height", odtPt(v))
		} else if v, ok := odtLength(p["line-spacing"]); ok {
			s.set("line-height", odtPt(max(ratio*size+v, 1)))
		} else {
			s.set("line-height", odtRatio(ratio))
		}
	}
	if p["number-lines"] == "false" {
		s.set("-ivy-line-numbers", "none")
	}
	switch p["text-align"] {
	case "center":
		s.set("text-align", "center")
	case "end", "right":
		s.set("text-align", "right")
	case "justify":
		s.set("text-align", "justify")
	default:
		s.set("text-align", "left")
	}
	for _, b := range [...]string{"break-before", "break-after"} {
		if v := p[b]; v == "page" || v == "column" {
			s.set(b, v)
		}
	}
	if p["keep-with-next"] == "always" {
		s.set("break-after", "avoid")
	}
	if p["keep-together"] == "always" {
		s.set("break-inside", "avoid")
	}
	// Without fo:orphans and fo:widows, LibreOffice splits a paragraph at
	// any line.
	for _, k := range [...]string{"orphans", "widows"} {
		n, err := strconv.Atoi(p[k])
		if err != nil || n < 1 {
			n = 1
		}
		s.set(k, strconv.Itoa(min(n, 100)))
	}
	if v, ok := odtLength(p["tab-stop-distance"]); ok && v > 0 {
		s.set("tab-size", odtPt(v))
	} else {
		s.set("tab-size", "36pt")
	}
	s.set("-ivy-tab-stops", odtTabStopsCSS(p["tab-stops"], left))
	return s.String()
}

// odtTabStopsCSS returns the value of -ivy-tab-stops of the tab stops of a
// paragraph, which odtTabStops encoded, or "" for a paragraph without
// tab stops. The position of a stop of OpenDocument is from the left
// margin of the paragraph, left, as LibreOffice places it, and the layout
// measures it from the edge of the margin of the paragraph.
func odtTabStopsCSS(stops string, left float64) string {
	var b strings.Builder
	for line := range strings.Lines(stops) {
		f := strings.Split(strings.TrimSuffix(line, "\n"), "\t")
		if len(f) != 3 {
			continue
		}
		pos, err := strconv.ParseFloat(f[0], 64)
		if err != nil {
			continue
		}
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		b.WriteString(odtPt(max(pos+left, 0)) + " " + f[1])
		if f[2] != "" {
			b.WriteString(" " + odtQuote(f[2]))
		}
	}
	return b.String()
}

// paragraph converts a text:p or a text:h element.
func (c *odtConverter) paragraph(n roxmltree.Node) {
	name := c.styleName(n)
	p := c.st.props("paragraph", name)
	list := c.list
	c.list = nil
	defer func() { c.list = list }()
	// A heading outside a list takes its number from the outline.
	item := list
	if item == nil && n.TagName().Name() == "h" {
		item = c.outlineItem(n)
	}
	left, indent := c.indents(name, p, item)
	if list != nil && list.marker == "" && list.lvl != nil && list.lvl.hasLabelPosition && !list.lvl.labelAlignment && indent < 0 {
		// In the mode label-width-and-position, the label of a paragraph
		// without a number, such as a list header, is empty but keeps its
		// width, text:min-label-width, so its first line starts at the
		// margin as its other lines.
		indent = 0
	}
	if p["contextual-spacing"] == "true" {
		// LibreOffice leaves out the spacing between two paragraphs of
		// the same paragraph style
		style := c.st.common("paragraph", name)
		var zero odtProps
		if prev, ok := odtParagraphNear(n, false); ok && c.st.common("paragraph", c.styleName(prev)) == style {
			zero = odtProps{"margin-top": "0pt"}
		}
		if next, ok := odtParagraphNear(n, true); ok && c.st.common("paragraph", c.styleName(next)) == style {
			zero = zero.over(odtProps{"margin-bottom": "0pt"})
		}
		if zero != nil {
			p = p.over(zero)
		}
	}
	c.w.WriteString(`<p style="` + odtAttrText(c.paragraphStyle(p, left, indent)) + `">`)
	mark := c.w.Len()
	if c.noteMark {
		c.noteMark = false
		c.w.WriteString(`<span class="fnref"></span>`)
	}
	if item != nil && item.marker != "" {
		c.marker(item, p, left, indent)
		item.marker = ""
	}
	c.space = true
	if bg := odtColor(p["text-background-color"]); bg != "" {
		// The background of the text of the paragraph is behind its
		// text only, and an empty paragraph shows none.
		var inner strings.Builder
		w := c.w
		c.w = &inner
		c.inlines(n, p)
		c.w = w
		if inner.Len() > 0 {
			c.w.WriteString(`<span style="background-color: ` + bg + `">` + inner.String() + "</span>")
		}
	} else {
		c.inlines(n, p)
	}
	if c.w.Len() == mark {
		c.w.WriteString("<br/>")
	}
	c.w.WriteString("</p>")
	c.lastBottom, _ = odtLength(p["margin-bottom"])
	c.forced = false
	if v := p["break-after"]; v == "page" || v == "column" {
		c.lastBottom, c.forced = 0, true
	}
}

// odtParagraphNear returns the paragraph after the paragraph n, or before
// it when next is false, in the same text: a text:p or a text:h, which can
// be in a list. It returns false when a table or the end of the text, such
// as the end of a cell, comes first.
func odtParagraphNear(n roxmltree.Node, next bool) (roxmltree.Node, bool) {
	sibling := func(e roxmltree.Node) (roxmltree.Node, bool) {
		if next {
			return e.NextSiblingElement()
		}
		return e.PrevSiblingElement()
	}
	cur := n
	for range 64 {
		for e, ok := sibling(cur); ok; e, ok = sibling(e) {
			if p, found := odtParagraphIn(e, next, 0); found {
				return p, true
			}
			if name := e.TagName().Name(); name != "soft-page-break" && !odtListElement(e) {
				return roxmltree.Node{}, false
			}
		}
		// the paragraphs after the list that holds n
		parent, ok := cur.Parent()
		if !ok || !parent.IsElement() || !odtListElement(parent) {
			break
		}
		cur = parent
	}
	return roxmltree.Node{}, false
}

// odtParagraphIn returns e if it is a paragraph, or else the first
// paragraph of the list e, or its last when next is false.
func odtParagraphIn(e roxmltree.Node, next bool, depth int) (roxmltree.Node, bool) {
	switch e.TagName().Name() {
	case "p", "h":
		return e, true
	}
	if !odtListElement(e) || depth > 64 {
		return roxmltree.Node{}, false
	}
	var last roxmltree.Node
	found := false
	for c := range e.Children() {
		if !c.IsElement() {
			continue
		}
		if p, ok := odtParagraphIn(c, next, depth+1); ok {
			if next {
				return p, true
			}
			last, found = p, true
		}
	}
	return last, found
}

// odtListElement reports whether e is a text:list, or an item or a header
// of one.
func odtListElement(e roxmltree.Node) bool {
	switch e.TagName().Name() {
	case "list", "list-item", "list-header":
		return true
	}
	return false
}

// odtOutlineKey is the key of the counters of the headings in
// odtConverter.counters. No list style has this name.
const odtOutlineKey = "\x00outline"

// outlineItem counts the heading n, a text:h outside a list, and returns
// the list item that gives its number from the text:outline-style. It
// returns nil if the heading has no number.
func (c *odtConverter) outlineItem(n roxmltree.Node) *odtList {
	levels := c.st.outline
	level := 1
	if v, err := strconv.Atoi(odtAttr(n, "outline-level")); err == nil {
		level = v
	}
	if levels == nil || level < 1 || level > 10 || odtAttr(n, "is-list-header") == "true" {
		return nil
	}
	lvl := levels[level-1]
	if lvl == nil {
		return nil
	}
	counters, ok := c.counters[odtOutlineKey]
	if !ok {
		counters = &[10]int{}
		c.counters[odtOutlineKey] = counters
	}
	i := level - 1
	switch v, err := strconv.Atoi(odtAttr(n, "start-value")); {
	case odtAttr(n, "restart-numbering") == "true" && err == nil:
		counters[i] = v - 1
	case odtAttr(n, "restart-numbering") == "true", counters[i] == 0:
		counters[i] = lvl.start - 1
	}
	counters[i]++
	for j := level; j < 10; j++ {
		counters[j] = 0
	}
	marker := odtListMarker(levels, lvl, counters, i)
	if marker == "" {
		return nil
	}
	return &odtList{level: i, marker: marker, lvl: lvl}
}

// marker writes the marker of the first paragraph of an item of a list,
// whose paragraph has the margin left and the indent of its first line
// indent. A label that follows a tab ends at the tab stop of the level when
// the stop is after the start of the line, else at the margin.
func (c *odtConverter) marker(list *odtList, p odtProps, left, indent float64) {
	var s odtCSS
	text := list.marker
	// space is the width from the start of the line to the text after
	// the label, or 0
	space := 0.0
	switch list.lvl.followedBy {
	case "space":
		text += " "
	case "nothing":
	default:
		switch {
		case list.lvl.hasTabStop && list.lvl.tabStop > left+indent:
			space = list.lvl.tabStop - left - indent
		case -indent > 0:
			space = -indent
		default:
			text += "\t"
		}
	}
	shift := ""
	switch list.lvl.labelAlign {
	case "end", "right":
		shift = "-100%"
	case "center":
		shift = "-50%"
	}
	if shift != "" && space > 0 {
		// The label ends, or has its center, at the start of the line,
		// and the text after it starts at the stop. The box of the label
		// has no width, and the label moves to the left as it is drawn.
		mp := c.markerProps(list, p)
		c.w.WriteString(`<span style="display: inline-block; width: 0; text-indent: 0"><span style="` +
			odtAttrText("display: inline-block; transform: translateX("+shift+")"+joinStyle(c.spanStyle(mp, p))) + `">`)
		odtEscape(c.w, text, false)
		c.w.WriteString(`</span></span><span style="display: inline-block; width: ` + odtPt(space) + `"></span>`)
		return
	}
	if space > 0 {
		s.set("display", "inline-block")
		s.set("min-width", odtPt(space))
		s.set("text-indent", "0")
	}
	mp := c.markerProps(list, p)
	style := s.String()
	if span := c.spanStyle(mp, p); span != "" {
		if style != "" {
			style += "; "
		}
		style += span
	}
	c.w.WriteString(`<span style="` + odtAttrText(style) + `">`)
	odtEscape(c.w, text, false)
	c.w.WriteString("</span>")
}

// markerProps returns the properties of the text of the marker of the
// list item list in a paragraph with the properties p.
func (c *odtConverter) markerProps(list *odtList, p odtProps) odtProps {
	mp := p.over(list.lvl.props)
	if v := list.lvl.props["font-name"]; v != "" && list.lvl.bullet != "" {
		// A bullet of a symbol font holds a character of Unicode, which the
		// font of the text draws.
		if fam, _ := c.st.font(odtProps{"font-name": v}); strings.Contains(strings.ToLower(fam), "symbol") || strings.Contains(strings.ToLower(fam), "dings") {
			mp["font-name"] = p["font-name"]
		}
	}
	return mp
}

// joinStyle returns the declarations s after a declaration: "; " and s,
// or "" when s is empty.
func joinStyle(s string) string {
	if s == "" {
		return ""
	}
	return "; " + s
}

// text writes text with the properties p, with its runs of white space
// collapsed to one space, as OpenDocument says. The East Asian language of
// p goes on the CJK text, so that its characters take the faces of that
// language.
func (c *odtConverter) text(s string, p odtProps) {
	var b strings.Builder
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !c.space {
				b.WriteByte(' ')
				c.space = true
			}
			continue
		}
		b.WriteRune(r)
		c.space = false
	}
	text := b.String()
	if !strings.ContainsFunc(text, fontdb.IsCJK) {
		odtEscape(c.w, text, false)
		return
	}
	lang := odtAsianLang(p)
	if lang != "" {
		c.w.WriteString(`<span lang="` + odtAttrText(lang) + `">`)
	}
	// The East Asian text takes its own size, style:font-size-asian.
	west, asian := odtFontSize(p), odtSizeOf(p, "-asian")
	if asian == west {
		odtEscape(c.w, text, false)
	} else {
		if va, scale := odtTextPosition(p["text-position"]); va != "" {
			asian *= scale
		}
		for len(text) > 0 {
			cjk := strings.IndexFunc(text, fontdb.IsCJK)
			if cjk < 0 {
				odtEscape(c.w, text, false)
				break
			}
			odtEscape(c.w, text[:cjk], false)
			text = text[cjk:]
			end := strings.IndexFunc(text, func(r rune) bool { return !fontdb.IsCJK(r) })
			if end < 0 {
				end = len(text)
			}
			c.w.WriteString(`<span style="font-size: ` + odtPt(asian) + `">`)
			odtEscape(c.w, text[:end], false)
			c.w.WriteString("</span>")
			text = text[end:]
		}
	}
	if lang != "" {
		c.w.WriteString("</span>")
	}
}

// odtSafeLink reports whether a link of a document can be a link of the
// layout: a URL of the web or of mail.
func odtSafeLink(href string) bool {
	lower := strings.ToLower(strings.TrimSpace(href))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:")
}

// inlines converts the inline content of n, whose text has the properties
// p.
func (c *odtConverter) inlines(n roxmltree.Node, p odtProps) {
	if !c.enter() {
		return
	}
	defer c.leave()
	for e := range n.Children() {
		if e.IsText() {
			s, _ := e.Text()
			c.text(s, p)
			continue
		}
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "span":
			sp := c.st.textOver(p, c.styleName(e))
			if sp["display"] == "none" {
				continue
			}
			if style := c.spanStyle(sp, p); style != "" {
				c.w.WriteString(`<span style="` + odtAttrText(style) + `">`)
				c.inlines(e, sp)
				c.w.WriteString("</span>")
			} else {
				c.inlines(e, sp)
			}
		case "s":
			count := 1
			if v, err := strconv.Atoi(odtAttr(e, "c")); err == nil && v > 1 {
				count = min(v, 1024)
			}
			c.w.WriteString(strings.Repeat(" ", count))
			c.space = false
		case "tab":
			c.w.WriteString("\t")
			c.space = true
		case "line-break":
			c.w.WriteString("<br/>")
			c.space = true
		case "a":
			if href := odtAttr(e, "href"); odtSafeLink(href) {
				c.w.WriteString(`<a href="` + odtAttrText(href) + `">`)
				c.inlines(e, p)
				c.w.WriteString("</a>")
			} else {
				c.inlines(e, p)
			}
		case "note":
			c.note(e)
		case "page-number":
			c.w.WriteString(`<span class="pg"></span>`)
			c.space = false
		case "page-count":
			c.w.WriteString(`<span class="pgs"></span>`)
			c.space = false
		case "frame":
			c.frame(e)
		case "ruby":
			if base, ok := odtChild(e, "ruby-base"); ok {
				c.inlines(base, p)
			}
		case "bookmark", "bookmark-start", "bookmark-end", "reference-mark", "reference-mark-start",
			"reference-mark-end", "soft-page-break", "change", "change-start", "change-end",
			"annotation", "annotation-end", "toc-mark", "alphabetical-index-mark", "user-index-mark":
		case "number":
			// text:number holds the number of a list item or a heading,
			// which the conversion counts itself.
		case "placeholder":
			// LibreOffice shows the text of a placeholder on the screen,
			// and does not print it.
		case "hidden-text", "hidden-paragraph":
			if odtAttr(e, "is-hidden") != "true" {
				c.inlines(e, p)
			}
		default:
			if odtIsShape(e) {
				c.shape(e)
				continue
			}
			// A field, such as text:date or text:sequence, shows the text
			// that the file holds. A link around a frame, draw:a, shows its
			// frame.
			c.inlines(e, p)
		}
	}
}

// listBlock converts a text:list.
func (c *odtConverter) listBlock(n roxmltree.Node) {
	style := odtAttr(n, "style-name")
	level := 0
	outer := c.list
	if outer != nil {
		level = outer.level + 1
		if style == "" {
			style = outer.style
		}
	}
	if level >= 10 || !c.enter() {
		c.deep = c.deep || level >= 10
		return
	}
	defer c.leave()
	levels := c.st.lists[style]
	counters, ok := c.counters[style]
	if !ok {
		counters = &[10]int{}
		c.counters[style] = counters
	}
	if level == 0 && odtAttr(n, "continue-numbering") != "true" && odtAttr(n, "continue-list") == "" {
		*counters = [10]int{}
	}
	for item := range n.Children() {
		if !item.IsElement() {
			continue
		}
		name := item.TagName().Name()
		if name != "list-item" && name != "list-header" {
			continue
		}
		var lvl *odtListLevel
		if levels != nil {
			lvl = levels[level]
		}
		if lvl == nil {
			lvl = &odtListLevel{bullet: "•", followedBy: "listtab", textIndent: -18, marginLeft: 36 * float64(level+1), hasLabelPosition: true, display: 1, start: 1, props: odtProps{}}
		}
		marker := ""
		if name == "list-item" {
			if v, err := strconv.Atoi(odtAttr(item, "start-value")); err == nil {
				counters[level] = v - 1
			} else if counters[level] == 0 {
				counters[level] = lvl.start - 1
			}
			counters[level]++
			for i := level + 1; i < 10; i++ {
				counters[i] = 0
			}
			marker = odtListMarker(levels, lvl, counters, level)
		}
		for e := range item.Children() {
			if !e.IsElement() {
				continue
			}
			c.list = &odtList{style: style, level: level, marker: marker, lvl: lvl}
			marker = ""
			if e.TagName().Name() == "list" {
				c.list.marker = ""
			}
			c.block(e)
		}
		c.list = outer
	}
	c.list = outer
}

// odtListMarker returns the text of the marker of an item of a list at the
// level.
func odtListMarker(levels *[10]*odtListLevel, lvl *odtListLevel, counters *[10]int, level int) string {
	if lvl.bullet != "" {
		return lvl.bullet
	}
	if lvl.format == "" {
		return lvl.prefix + lvl.suffix
	}
	var parts []string
	for i := max(level-lvl.display+1, 0); i <= level; i++ {
		format := lvl.format
		if levels != nil && levels[i] != nil && levels[i].format != "" {
			format = levels[i].format
		}
		parts = append(parts, odtNumber(max(counters[i], 1), format))
	}
	return lvl.prefix + strings.Join(parts, ".") + lvl.suffix
}

// odtNumber writes n in the number format of OpenDocument: "1", "a", "A",
// "i" or "I".
func odtNumber(n int, format string) string {
	switch format {
	case "a", "A":
		if n <= 0 {
			return strconv.Itoa(n)
		}
		s := strings.Repeat(string(rune('a'+(n-1)%26)), min((n-1)/26+1, 64))
		if format == "A" {
			s = strings.ToUpper(s)
		}
		return s
	case "i", "I":
		s := odtRoman(n)
		if format == "I" {
			s = strings.ToUpper(s)
		}
		return s
	}
	return strconv.Itoa(n)
}

// odtRoman writes n in lower case Roman numerals, or in decimal outside 1
// to 3999.
func odtRoman(n int) string {
	if n <= 0 || n >= 4000 {
		return strconv.Itoa(n)
	}
	var b strings.Builder
	for _, r := range [...]struct {
		v int
		s string
	}{{1000, "m"}, {900, "cm"}, {500, "d"}, {400, "cd"}, {100, "c"}, {90, "xc"}, {50, "l"}, {40, "xl"}, {10, "x"}, {9, "ix"}, {5, "v"}, {4, "iv"}, {1, "i"}} {
		for n >= r.v {
			b.WriteString(r.s)
			n -= r.v
		}
	}
	return b.String()
}

// note converts a text:note: a footnote at the foot of its page, or an
// endnote at the end of the text.
func (c *odtConverter) note(n roxmltree.Node) {
	body, ok := odtChild(n, "note-body")
	if !ok || !c.enter() {
		return
	}
	defer c.leave()
	space, list, last := c.space, c.list, c.lastBottom
	c.list = nil
	c.nested++
	c.lastBottom = 0
	defer func() {
		c.space, c.list, c.lastBottom = space, list, last
		c.nested--
	}()
	if odtAttr(n, "note-class") == "endnote" {
		c.endnoteNum++
		mark := odtRoman(c.endnoteNum)
		c.w.WriteString(`<span style="vertical-align: super; font-size: 0.58em">` + mark + "</span>")
		w := c.w
		c.w = &c.endnotes
		c.w.WriteString(`<div><span style="float: left; margin-right: 6pt">` + mark + "</span>")
		c.blocks(body)
		c.w.WriteString("</div>")
		c.w = w
		return
	}
	c.w.WriteString(`<span class="fn">`)
	c.noteMark = true
	c.blocks(body)
	c.noteMark = false
	c.w.WriteString("</span>")
}

// table converts a table:table.
func (c *odtConverter) table(n roxmltree.Node) {
	if !c.enter() {
		return
	}
	defer c.leave()
	list := c.list
	c.list = nil
	defer func() { c.list = list }()
	p := c.st.props("table", c.styleName(n))
	var widths []float64
	var rows []roxmltree.Node
	var headerRows int
	c.tableParts(n, &widths, &rows, &headerRows, 0)
	if len(rows) == 0 {
		return
	}
	var s odtCSS
	total := 0.0
	for _, w := range widths {
		total += w
	}
	if w, ok := odtLength(p["width"]); ok && w > 0 {
		s.set("width", odtPt(w))
	} else if total > 0 {
		s.set("width", odtPt(total))
	}
	if total > 0 {
		s.set("table-layout", "fixed")
	}
	switch p["align"] {
	case "center":
		s.set("margin-left", "auto")
		s.set("margin-right", "auto")
	case "right":
		s.set("margin-left", "auto")
	default:
		if v, ok := odtLength(p["margin-left"]); ok && v != 0 {
			s.set("margin-left", odtPt(v))
		}
	}
	// The spacing above the table adds to the spacing below the
	// paragraph before it, as for a paragraph.
	if v, ok := odtLength(p["margin-top"]); ok && v > 0 {
		forced := c.forced || p["break-before"] == "page"
		if forced {
			s.set("margin-top", odtPt(v))
			s.set("margin-break", "keep")
		} else {
			s.set("margin-top", odtPt(v+c.lastBottom))
		}
	}
	c.forced = false
	bottom, ok := odtLength(p["margin-bottom"])
	if ok && bottom > 0 {
		s.set("margin-bottom", odtPt(bottom))
	}
	if v := odtColor(p["background-color"]); v != "" {
		s.set("background-color", v)
	}
	c.w.WriteString(`<table style="` + odtAttrText(s.String()) + `">`)
	defer func() { c.lastBottom = max(bottom, 0) }()
	if total > 0 {
		c.w.WriteString("<colgroup>")
		for _, w := range widths {
			c.w.WriteString(`<col style="width: ` + odtPt(w) + `"/>`)
		}
		c.w.WriteString("</colgroup>")
	}
	if headerRows > 0 {
		c.w.WriteString("<thead>")
	} else {
		c.w.WriteString("<tbody>")
	}
	for i, row := range rows {
		if c.check() != nil {
			return
		}
		if i == headerRows && headerRows > 0 {
			c.w.WriteString("</thead><tbody>")
		}
		c.row(row)
	}
	c.w.WriteString("</tbody></table>")
}

// maxTableColumns bounds the columns of a table, since a column can repeat.
const maxTableColumns = 1024

// tableParts reads the columns and the rows of a table, with those in
// groups.
func (c *odtConverter) tableParts(n roxmltree.Node, widths *[]float64, rows *[]roxmltree.Node, headerRows *int, depth int) {
	if depth > 4 {
		return
	}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "table-column":
			repeat := 1
			if v, err := strconv.Atoi(odtAttr(e, "number-columns-repeated")); err == nil && v > 1 {
				repeat = v
			}
			w, _ := odtLength(c.st.props("table-column", c.styleName(e))["column-width"])
			for range min(repeat, maxTableColumns-len(*widths)) {
				*widths = append(*widths, max(w, 0))
			}
		case "table-columns", "table-column-group", "table-header-columns":
			c.tableParts(e, widths, rows, headerRows, depth+1)
		case "table-header-rows":
			before := len(*rows)
			c.tableParts(e, widths, rows, headerRows, depth+1)
			if before == 0 {
				*headerRows = len(*rows)
			}
		case "table-rows", "table-row-group":
			c.tableParts(e, widths, rows, headerRows, depth+1)
		case "table-row":
			repeat := 1
			if v, err := strconv.Atoi(odtAttr(e, "number-rows-repeated")); err == nil && v > 1 {
				repeat = min(v, 64)
			}
			for range repeat {
				if len(*rows) < c.lim.MaxObjects {
					*rows = append(*rows, e)
				}
			}
		}
	}
}

// row converts a table:table-row.
func (c *odtConverter) row(n roxmltree.Node) {
	p := c.st.props("table-row", c.styleName(n))
	var s odtCSS
	if v, ok := odtLength(p["row-height"]); ok && v > 0 {
		s.set("height", odtPt(v))
	} else if v, ok := odtLength(p["min-row-height"]); ok && v > 0 {
		s.set("height", odtPt(v))
	}
	c.w.WriteString(`<tr style="` + odtAttrText(s.String()) + `">`)
	cells := 0
	for e := range n.Children() {
		if !e.IsElement() || e.TagName().Name() != "table-cell" {
			continue
		}
		repeat := 1
		if v, err := strconv.Atoi(odtAttr(e, "number-columns-repeated")); err == nil && v > 1 {
			repeat = v
		}
		for range min(repeat, maxTableColumns-cells) {
			cells++
			c.cell(e)
		}
	}
	c.w.WriteString("</tr>")
}

// cell converts a table:table-cell.
func (c *odtConverter) cell(n roxmltree.Node) {
	p := c.st.props("table-cell", c.styleName(n))
	var s odtCSS
	odtBox(&s, p)
	switch p["vertical-align"] {
	case "middle":
		s.set("vertical-align", "middle")
	case "bottom":
		s.set("vertical-align", "bottom")
	}
	c.w.WriteString("<td")
	for _, a := range [...][2]string{{"number-columns-spanned", "colspan"}, {"number-rows-spanned", "rowspan"}} {
		if v, err := strconv.Atoi(odtAttr(n, a[0])); err == nil && v > 1 {
			c.w.WriteString(` ` + a[1] + `="` + strconv.Itoa(min(v, maxTableColumns)) + `"`)
		}
	}
	c.w.WriteString(` style="` + odtAttrText(s.String()) + `">`)
	mark := c.w.Len()
	c.lastBottom = 0
	c.blocks(n)
	if c.w.Len() == mark {
		c.w.WriteString("<p><br/></p>")
	}
	c.w.WriteString("</td>")
}

// frame converts a draw:frame: an image, a text box or an object, inline
// or anchored.
func (c *odtConverter) frame(n roxmltree.Node) {
	w, _ := odtLength(odtAttr(n, "width"))
	h, _ := odtLength(odtAttr(n, "height"))
	if v, ok := odtLength(odtAttr(n, "min-height")); ok && h == 0 {
		h = v
	}
	w, h = min(max(w, 0), 14400), min(max(h, 0), 14400)
	x, _ := odtLength(odtAttr(n, "x"))
	y, _ := odtLength(odtAttr(n, "y"))
	g := c.st.props("graphic", c.styleName(n))
	c.place(n, x, y, w, g, func() { c.frameContent(n, w, h, g) })
}

// odtFloatPosition returns the value of -ivy-float-position of a frame with
// wrap at y points from the top of its area, from the graphic properties g:
// the page for style:vertical-rel page, and the page area for page-content
// (D60). It returns "" for a frame of another vertical-rel, which floats
// at its paragraph.
func odtFloatPosition(g odtProps, y float64) string {
	area := ""
	switch g["vertical-rel"] {
	case "page":
		area = "page "
	case "page-content":
		area = "margin "
	default:
		return ""
	}
	switch g["vertical-pos"] {
	case "top":
		return area + "top"
	case "middle":
		return area + "center"
	case "bottom":
		return area + "bottom"
	}
	return area + odtPt(y)
}

// place writes the box of a frame or a shape n, of the width w, at the
// position (x, y) from its anchor, with the graphic properties g. content
// writes what the box holds.
func (c *odtConverter) place(n roxmltree.Node, x, y, w float64, g odtProps, content func()) {
	anchor := odtAttr(n, "anchor-type")
	var s odtCSS
	inline := anchor == "as-char"
	switch {
	case inline:
	case anchor == "page" || anchor == "frame" || g["wrap"] == "run-through":
		s.set("position", "absolute")
		if anchor == "page" && c.layout != nil {
			x -= c.layout.left
			y -= c.pageTop
			s.set("top", odtPt(y))
		} else if y != 0 {
			s.set("margin-top", odtPt(y))
		}
		s.set("left", odtPt(x))
	case g["wrap"] == "none" || g["wrap"] == "":
		s.set("display", "block")
		switch g["horizontal-pos"] {
		case "center":
			s.set("margin-left", "auto")
			s.set("margin-right", "auto")
		case "right":
			s.set("margin-left", "auto")
		default:
			if x > 0 {
				s.set("margin-left", odtPt(x))
			}
		}
	default:
		side := "left"
		if g["horizontal-pos"] == "right" || g["wrap"] == "left" {
			side = "right"
		}
		s.set("float", side)
		if p := odtFloatPosition(g, y); p != "" {
			s.set("-ivy-float-position", p)
		}
	}
	if !inline {
		s.set("width", odtPt(w))
	}
	if inline {
		c.w.WriteString(`<span>`)
	} else {
		c.w.WriteString(`<span style="` + odtAttrText(s.String()) + `">`)
	}
	content()
	c.w.WriteString("</span>")
	c.space = false
}

// odtShapes holds the local names of the shapes of the namespace draw that
// a text draws with odf/internal/drawpage.
var odtShapes = map[string]bool{
	"rect": true, "ellipse": true, "circle": true, "custom-shape": true, "polygon": true, "polyline": true,
	"path": true, "regular-polygon": true, "line": true, "connector": true, "g": true, "caption": true,
	"measure": true,
}

// odtNSDraw is the namespace of the drawings of OpenDocument.
const odtNSDraw = "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0"

// odtIsShape reports whether n is a shape that the conversion draws.
func odtIsShape(n roxmltree.Node) bool {
	uri, _ := n.TagName().Namespace()
	return uri == odtNSDraw && odtShapes[n.TagName().Name()]
}

// shape converts a shape of the namespace draw, such as draw:rect or a
// group draw:g, into an object of the layout that odf/internal/drawpage
// draws. The text of a shape that holds a frame or a table, such as a
// picture with its caption, goes through the layout over the shape, since
// drawpage draws only the paragraphs of a text.
func (c *odtConverter) shape(n roxmltree.Node) {
	if c.shapes == nil {
		cfg := c.d.cfg
		cfg.Limits = c.lim
		c.shapes = drawpage.NewShapes(drawpage.Options{Config: cfg, Read: c.d.readEntry}, c.content, c.styles)
	}
	c.objects++
	if c.objects > c.lim.MaxObjects {
		c.log.Warn("odf: too many objects, shape left out", "element", n.TagName().Name())
		return
	}
	over := odtRichText(n)
	sh, ok := c.shapes.Shape(c.ctx, n, !over)
	if !ok {
		return
	}
	id := "shape" + strconv.Itoa(len(c.charts)+1)
	c.charts[id] = sh
	g := c.st.props("graphic", c.styleName(n))
	src := odtAttrText(layout.ObjectURL(id))
	inline := odtAttr(n, "anchor-type") == "as-char"
	c.place(n, sh.X, sh.Y, sh.W, g, func() {
		var s odtCSS
		if !inline && !over {
			// a box apart from the line has no baseline
			s.set("display", "block")
		}
		if over {
			s.set("position", "absolute")
			s.set("left", "0")
			s.set("top", "0")
		}
		s.set("width", odtPt(sh.W))
		s.set("height", odtPt(sh.H))
		img := `<img src="` + src + `" alt="" style="` + s.String() + `"/>`
		if !over {
			c.w.WriteString(img)
			return
		}
		var box odtCSS
		box.set("display", "inline-block")
		box.set("position", "relative")
		box.set("vertical-align", "top")
		box.set("width", odtPt(sh.W))
		box.set("min-height", odtPt(sh.H))
		c.w.WriteString(`<span style="` + odtAttrText(box.String()) + `">` + img)
		// The text is a positioned box, which draws over the shape.
		var text odtCSS
		text.set("display", "block")
		text.set("position", "relative")
		text.set("box-sizing", "border-box")
		text.set("width", odtPt(sh.W))
		all, _ := odtLength(g["padding"])
		pad := (sh.W - odtShapeWidth(n)) / 2
		for _, side := range [...]string{"top", "right", "bottom", "left"} {
			v, ok := odtLength(g["padding-"+side])
			if !ok {
				v = all
			}
			text.set("padding-"+side, odtPt(max(v, 0)+pad))
		}
		c.w.WriteString(`<span style="` + odtAttrText(text.String()) + `">`)
		list, last := c.list, c.lastBottom
		c.list = nil
		c.nested++
		c.lastBottom = 0
		c.blocks(n)
		c.nested--
		c.list, c.lastBottom = list, last
		c.w.WriteString("</span></span>")
	})
}

// odtShapeWidth returns svg:width of the shape n, or 0.
func odtShapeWidth(n roxmltree.Node) float64 {
	w, _ := odtLength(odtAttr(n, "width"))
	return max(w, 0)
}

// odtRichText reports whether the text of the shape n holds a frame or a
// table, which the drawing of its text does not show. A group has no text
// of its own.
func odtRichText(n roxmltree.Node) bool {
	if n.TagName().Name() == "g" {
		return false
	}
	for d := range n.Descendants() {
		if !d.IsElement() {
			continue
		}
		switch d.TagName().Name() {
		case "frame", "table":
			return true
		}
	}
	return false
}

// frameContent writes the content of a frame of w by h points: its first
// image, text box or chart that the conversion reads.
func (c *odtConverter) frameContent(n roxmltree.Node, w, h float64, g odtProps) {
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "image":
			src, ok := "", false
			if href := odtAttr(e, "href"); href != "" {
				src, ok = c.file(href)
			} else if data, has := odtChild(e, "binary-data"); has {
				src, ok = c.binaryFile(data)
			}
			if !ok {
				continue
			}
			c.w.WriteString(`<img src="` + odtAttrText(src) + `" alt="" style="width: ` + odtPt(w) + `; height: ` + odtPt(h) + `"/>`)
			return
		case "text-box":
			var s odtCSS
			s.set("display", "inline-block")
			s.set("vertical-align", "top")
			s.set("box-sizing", "border-box")
			s.set("width", odtPt(w))
			s.set("min-height", odtPt(h))
			odtBox(&s, g)
			c.w.WriteString(`<span style="` + odtAttrText(s.String()) + `">`)
			list, last := c.list, c.lastBottom
			c.list = nil
			c.nested++
			c.lastBottom = 0
			c.blocks(e)
			c.nested--
			c.list, c.lastBottom = list, last
			c.w.WriteString("</span>")
			return
		case "object", "object-ole":
			if c.chart(odtAttr(e, "href"), w, h) {
				return
			}
		}
	}
}

// chart writes the embedded object href of w by h points as an object of
// the layout that internal/chart draws, if it is a chart. It returns false
// for another object, whose replacement image comes next in the frame.
func (c *odtConverter) chart(href string, w, h float64) bool {
	name := path.Clean(strings.TrimPrefix(href, "./"))
	if name == "." || strings.HasPrefix(name, "..") || strings.Contains(href, "://") {
		return false
	}
	buf, err := c.d.readEntry(c.ctx, name+"/content.xml")
	if err != nil {
		return false
	}
	c.objects++
	if c.objects > c.lim.MaxObjects {
		return false
	}
	ch, err := chart.ParseODF(buf, chart.Options{Config: c.d.cfg, Fonts: fontdb.Of(c.d.cfg), Width: w, Height: h})
	if err != nil {
		c.log.Debug("odf: object that is not a chart", "object", name, "err", err)
		return false
	}
	id := "chart" + strconv.Itoa(len(c.charts)+1)
	c.charts[id] = ch
	c.w.WriteString(`<img src="` + odtAttrText(layout.ObjectURL(id)) + `" alt="" style="width: ` + odtPt(w) + `; height: ` + odtPt(h) + `"/>`)
	return true
}
