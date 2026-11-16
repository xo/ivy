package odf

import (
	"path"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/chart"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/layout"
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

// family returns the value of font-family of the properties, or "".
func (c *odtConverter) family(p odtProps) string {
	family, generic := c.st.font(p)
	if family == "" {
		return ""
	}
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
	return odtQuote(family) + ", " + g
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
	if v := odtColor(p["background-color"]); v != "" {
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

// paragraphStyle returns the style attribute of a paragraph with the
// properties p.
func (c *odtConverter) paragraphStyle(p odtProps, list *odtList) string {
	var s odtCSS
	for _, d := range c.inherited(p) {
		s.set(d.name, d.value)
	}
	boxed := odtBox(&s, p)
	left, _ := odtLength(p["margin-left"])
	right, _ := odtLength(p["margin-right"])
	indent, _ := odtLength(p["text-indent"])
	if list != nil && list.lvl != nil && list.lvl.hasLabelPosition {
		left, indent = list.lvl.marginLeft, list.lvl.textIndent
	}
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
	// the next one. The spacing above is a padding, which does not
	// collapse with the margin of the spacing below. The spacing below is
	// a margin, which the bottom of a page cuts, as in LibreOffice.
	top, _ := odtLength(p["margin-top"])
	bottom, _ := odtLength(p["margin-bottom"])
	if top > 0 {
		s.set(odtPick(boxed, "margin-top", "padding-top"), odtPt(top))
	}
	if bottom > 0 {
		s.set("margin-bottom", odtPt(bottom))
	}
	family, _ := c.st.font(p)
	ratio := odtLineRatio(family)
	size := odtFontSize(p)
	switch lh := p["line-height"]; {
	case strings.HasSuffix(lh, "%"):
		if pct, err := strconv.ParseFloat(strings.TrimSuffix(lh, "%"), 64); err == nil && pct > 0 {
			s.set("line-height", odtNum(min(pct/100, 10)*ratio))
		}
	default:
		if v, ok := odtLength(lh); ok && v > 0 {
			s.set("line-height", odtPt(v))
		} else if v, ok := odtLength(p["line-height-at-least"]); ok && v > ratio*size {
			s.set("line-height", odtPt(v))
		} else if v, ok := odtLength(p["line-spacing"]); ok {
			s.set("line-height", odtPt(max(ratio*size+v, 1)))
		} else {
			s.set("line-height", odtNum(ratio))
		}
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
	return s.String()
}

// paragraph converts a text:p or a text:h element.
func (c *odtConverter) paragraph(n roxmltree.Node) {
	p := c.st.props("paragraph", odtAttr(n, "style-name"))
	list := c.list
	c.list = nil
	defer func() { c.list = list }()
	c.w.WriteString(`<p style="` + odtAttrText(c.paragraphStyle(p, list)) + `">`)
	mark := c.w.Len()
	if c.noteMark {
		c.noteMark = false
		c.w.WriteString(`<span class="fnref"></span>`)
	}
	if list != nil && list.marker != "" {
		c.marker(list, p)
		list.marker = ""
	}
	c.space = true
	c.inlines(n, p)
	if c.w.Len() == mark {
		c.w.WriteString("<br/>")
	}
	c.w.WriteString("</p>")
}

// marker writes the marker of the first paragraph of an item of a list.
func (c *odtConverter) marker(list *odtList, p odtProps) {
	var s odtCSS
	text := list.marker
	switch list.lvl.followedBy {
	case "space":
		text += " "
	case "nothing":
	default:
		if w := -list.lvl.textIndent; w > 0 {
			s.set("display", "inline-block")
			s.set("min-width", odtPt(w))
			s.set("text-indent", "0")
		} else {
			text += "\t"
		}
	}
	mp := p.over(list.lvl.props)
	if v := list.lvl.props["font-name"]; v != "" && list.lvl.bullet != "" {
		// A bullet of a symbol font holds a character of Unicode, which the
		// font of the text draws.
		if fam, _ := c.st.font(odtProps{"font-name": v}); strings.Contains(strings.ToLower(fam), "symbol") || strings.Contains(strings.ToLower(fam), "dings") {
			mp["font-name"] = p["font-name"]
		}
	}
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

// text writes text with its runs of white space collapsed to one space, as
// OpenDocument says.
func (c *odtConverter) text(s string) {
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
	odtEscape(c.w, b.String(), false)
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
			c.text(s)
			continue
		}
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "span":
			sp := p.over(c.st.props("text", odtAttr(e, "style-name")))
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
		case "hidden-text", "hidden-paragraph":
			if odtAttr(e, "is-hidden") != "true" {
				c.inlines(e, p)
			}
		default:
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
	space, list := c.space, c.list
	c.list = nil
	c.nested++
	defer func() {
		c.space, c.list = space, list
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
	p := c.st.props("table", odtAttr(n, "style-name"))
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
	for _, side := range [...]string{"top", "bottom"} {
		if v, ok := odtLength(p["margin-"+side]); ok && v > 0 {
			s.set("margin-"+side, odtPt(v))
		}
	}
	if v := odtColor(p["background-color"]); v != "" {
		s.set("background-color", v)
	}
	c.w.WriteString(`<table style="` + odtAttrText(s.String()) + `">`)
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
			w, _ := odtLength(c.st.props("table-column", odtAttr(e, "style-name"))["column-width"])
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
	p := c.st.props("table-row", odtAttr(n, "style-name"))
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
	p := c.st.props("table-cell", odtAttr(n, "style-name"))
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
	g := c.st.props("graphic", odtAttr(n, "style-name"))
	anchor := odtAttr(n, "anchor-type")
	var s odtCSS
	inline := anchor == "as-char"
	switch {
	case inline:
	case anchor == "page" || anchor == "frame" || g["wrap"] == "run-through":
		x, _ := odtLength(odtAttr(n, "x"))
		y, _ := odtLength(odtAttr(n, "y"))
		s.set("position", "absolute")
		if anchor == "page" && c.layout != nil {
			x -= c.layout.left
			y -= c.layout.top
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
			if x, ok := odtLength(odtAttr(n, "x")); ok && x > 0 {
				s.set("margin-left", odtPt(x))
			}
		}
	default:
		side := "left"
		if g["horizontal-pos"] == "right" || g["wrap"] == "left" {
			side = "right"
		}
		s.set("float", side)
	}
	if !inline {
		s.set("width", odtPt(w))
	}
	if inline {
		c.w.WriteString(`<span>`)
	} else {
		c.w.WriteString(`<span style="` + odtAttrText(s.String()) + `">`)
	}
	c.frameContent(n, w, h, g)
	c.w.WriteString("</span>")
	c.space = false
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
			list := c.list
			c.list = nil
			c.nested++
			c.blocks(e)
			c.nested--
			c.list = list
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
