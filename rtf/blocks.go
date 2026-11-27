package rtf

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/ivy/internal/ebook"
)

// lineRatios are the heights of a single line of the fonts that documents
// use most, as a multiple of the font size.
var lineRatios = map[string]float64{
	"calibri": 1.2207, "carlito": 1.2207, "cambria": 1.1719, "caladea": 1.1719,
	"arial": 1.1499, "liberation sans": 1.1499, "helvetica": 1.1499,
	"times new roman": 1.1499, "liberation serif": 1.1499, "times": 1.1499,
	"courier new": 1.1328, "liberation mono": 1.1328, "courier": 1.1328,
	"verdana": 1.2153, "tahoma": 1.207,
}

// escape writes s to b with the characters of XML escaped. It leaves out
// the characters that XML 1.0 does not allow.
func escape(b *strings.Builder, s string, attr bool) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch {
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '&':
			b.WriteString("&amp;")
		case r == '"' && attr:
			b.WriteString("&quot;")
		case r == '\t':
			if attr {
				b.WriteString("&#9;")
			} else {
				b.WriteRune(r)
			}
		case r < 0x20, r == 0xfffe, r == 0xffff, r == utf8.RuneError && size == 1:
		default:
			b.WriteRune(r)
		}
	}
}

// attrText returns s escaped for an attribute.
func attrText(s string) string {
	var b strings.Builder
	escape(&b, s, true)
	return b.String()
}

// pt formats a length in twips as points for CSS.
func pt(twips int) string {
	return num(float64(twips)/20) + "pt"
}

// num formats a number for CSS, with 2 decimals at most.
func num(v float64) string {
	return strconv.FormatFloat(float64(int64(v*100+map[bool]float64{true: -0.5, false: 0.5}[v < 0]))/100, 'f', -1, 64)
}

// css builds the declarations of a style attribute.
type css struct {
	b strings.Builder
}

// set adds the declaration name: value.
func (c *css) set(name, value string) {
	if value == "" {
		return
	}
	if c.b.Len() > 0 {
		c.b.WriteString("; ")
	}
	c.b.WriteString(name + ": " + value)
}

// String returns the declarations.
func (c *css) String() string {
	return c.b.String()
}

// color returns the color i of the color table, or "".
func (c *converter) color(i int) string {
	if i <= 0 || i >= len(c.colors) {
		return ""
	}
	return c.colors[i]
}

// family returns the value of font-family of the font f, and its name.
func (c *converter) family(f int) (string, string) {
	family := c.families(f)
	if family == "" {
		return "", ""
	}
	return family, strings.TrimSpace(c.fonts[f].name)
}

// families returns the value of font-family of the fonts, in order, with
// the generic family of the first font that the font table names, or "".
func (c *converter) families(fonts ...int) string {
	var b strings.Builder
	generic := ""
	for _, f := range fonts {
		e, ok := c.fonts[f]
		if !ok || strings.TrimSpace(e.name) == "" {
			continue
		}
		if generic == "" {
			generic = e.generic
			if generic == "" || generic == "symbol" {
				generic = "serif"
			}
		}
		b.WriteByte('"')
		for _, r := range strings.TrimSpace(e.name) {
			if r == '"' || r == '\\' {
				b.WriteByte('\\')
			}
			if r >= 0x20 && r != 0x7f {
				b.WriteRune(r)
			}
		}
		b.WriteString(`", `)
	}
	if generic == "" {
		return ""
	}
	return b.String() + generic
}

// The values of \fcharset of the East Asian fonts, and their languages.
var eastAsianCharsets = map[int]string{128: "ja", 129: "ko", 134: "zh-CN", 136: "zh-TW"}

// eastAsianFont reports whether the properties p have an East Asian font
// other than the font of the run: the font of \dbch\af, if it has the
// \fcharset of an East Asian font.
func (c *converter) eastAsianFont(p charProps) bool {
	if p.fontEA < 0 || p.fontEA == p.font {
		return false
	}
	e, ok := c.fonts[p.fontEA]
	if !ok || strings.TrimSpace(e.name) == "" {
		return false
	}
	_, cjk := eastAsianCharsets[e.charset]
	return cjk
}

// eastAsianLang returns the language of the East Asian text of the
// properties p: the language of \langfe, else the language of the
// \fcharset of the East Asian font, or "".
func (c *converter) eastAsianLang(p charProps) string {
	if p.langEA > 0 && p.langEA != 1024 {
		if v := ebook.LCIDLanguage(uint32(p.langEA)); v != "" {
			return v
		}
	}
	for _, f := range [...]int{p.fontEA, p.font} {
		if e, ok := c.fonts[f]; ok {
			if v, cjk := eastAsianCharsets[e.charset]; cjk {
				return v
			}
		}
	}
	return ""
}

// runStyle returns the style attribute of a run with the properties p. A
// paragraph has the font and the size of \plain, so a run states the
// properties that differ from \plain.
func (c *converter) runStyle(p charProps) string {
	var s css
	if ea := c.eastAsianFont(p); ea {
		// the East Asian font comes after the font of the run, and the
		// layout takes a character that the first font does not have from
		// it
		s.set("font-family", c.families(p.font, p.fontEA))
	} else if family, _ := c.family(p.font); family != "" {
		s.set("font-family", family)
	}
	// A run states its font and its size, since its paragraph has those of
	// its mark.
	size := float64(p.size) / 2
	if p.vertical != 0 {
		size *= 0.65
	}
	s.set("font-size", num(size)+"pt")
	if p.bold {
		s.set("font-weight", "bold")
	}
	if p.italic {
		s.set("font-style", "italic")
	}
	if v := c.color(p.color); v != "" {
		s.set("color", v)
	}
	var lines []string
	if p.underline {
		lines = append(lines, "underline")
	}
	if p.strike {
		lines = append(lines, "line-through")
	}
	if len(lines) > 0 {
		s.set("text-decoration-line", strings.Join(lines, " "))
	}
	if v := c.color(p.highlight); v != "" {
		s.set("background-color", v)
	} else if v := c.color(p.background); v != "" {
		s.set("background-color", v)
	}
	if p.caps {
		s.set("text-transform", "uppercase")
	} else if p.smallCaps {
		s.set("font-variant", "small-caps")
	}
	switch p.vertical {
	case 1:
		s.set("vertical-align", "super")
	case 2:
		s.set("vertical-align", "sub")
	}
	if p.spacing != 0 {
		s.set("letter-spacing", pt(p.spacing))
	}
	return s.String()
}

// paraStyle returns the style attribute of a paragraph with the properties
// p, whose mark has the properties ch.
func (c *converter) paraStyle(p paraProps, ch charProps, brk string) string {
	var s css
	if brk != "" {
		s.set("break-before", brk)
	}
	// The paragraph has the font and the size of its mark, which give the
	// least height of its lines, as in Word, and its runs state theirs.
	family, name := c.family(ch.font)
	if family == "" {
		family, _ = c.family(c.docFont)
	}
	s.set("font-family", family)
	size := float64(ch.size) / 2
	if size <= 0 {
		size = 12
	}
	s.set("font-size", num(size)+"pt")
	if p.left != 0 {
		s.set("margin-left", pt(p.left))
	}
	if p.right != 0 {
		s.set("margin-right", pt(p.right))
	}
	if p.first != 0 {
		s.set("text-indent", pt(p.first))
	}
	boxed := p.shading > 0 || p.borderTop || p.borderBottom || p.borderLeft || p.borderRight
	// With \htmautsp, the spacing before is a margin, which collapses with
	// the margin of the spacing after, as Word puts the larger of the two.
	// Else it is a padding, which does not collapse, as Word adds the two.
	// A paragraph with a border or a shading keeps a margin, which its
	// border and its shading do not hold. The spacing after is a margin,
	// which the bottom of a page cuts, as in Word.
	if p.before > 0 {
		s.set(map[bool]string{true: "margin-top", false: "padding-top"}[boxed || c.htmlSpacing], pt(p.before))
	}
	if p.after > 0 {
		s.set("margin-bottom", pt(p.after))
	}
	for _, b := range [...]struct {
		on   bool
		side string
	}{{p.borderTop, "top"}, {p.borderRight, "right"}, {p.borderBottom, "bottom"}, {p.borderLeft, "left"}} {
		if b.on {
			s.set("border-"+b.side, "0.5pt solid black")
			s.set("padding-"+b.side, "1pt")
		}
	}
	if v := c.color(p.shading); v != "" {
		s.set("background-color", v)
	}
	ratio, ok := lineRatios[strings.ToLower(name)]
	if !ok {
		ratio = 1.15
	}
	switch {
	case p.line > 0 && p.lineMultiple:
		s.set("line-height", num(min(float64(p.line)/240, 10)*ratio))
		// Word puts the line gap of a single line above its text, and
		// the extra space of the lines below it
		s.set("-ivy-line-multiple", num(min(float64(p.line)/240, 10)))
	case p.line > 0 && float64(p.line)/20 > ratio*size:
		s.set("line-height", pt(p.line))
	case p.line < 0:
		s.set("line-height", pt(-p.line))
	default:
		s.set("line-height", num(ratio))
		s.set("-ivy-line-multiple", "1")
	}
	switch p.align {
	case 'c':
		s.set("text-align", "center")
	case 'r':
		s.set("text-align", "right")
	case 'j':
		s.set("text-align", "justify")
	}
	if p.pageBrk && brk == "" {
		s.set("break-before", "page")
	}
	if p.keepNext {
		s.set("break-after", "avoid")
	}
	if p.keepLines {
		s.set("break-inside", "avoid")
	}
	if p.noLine {
		s.set("-ivy-line-numbers", "none")
	}
	return s.String()
}

// endParagraph ends the current paragraph of the target of s. A
// paragraph out of the table of its level closes the tables that nest
// deeper.
func (c *converter) endParagraph(s *state) {
	c.flushBytes(s)
	t := s.target
	c.flushRun(t)
	lvl := paraLevel(s.par)
	c.closeTables(t, lvl)
	out := &t.out
	if lvl > 0 {
		out = &t.level(lvl).cell
	}
	frame := c.paraFrame(t, s.par, lvl)
	if frame != t.frame {
		c.closeFrame(t)
	}
	if t == c.body && lvl == 0 {
		c.openSection()
	}
	if frame != t.frame {
		c.openFrame(t, frame)
	}
	out.WriteString(`<p style="` + attrText(c.paraStyle(s.par, s.chr, t.breakNext)) + `">`)
	t.breakNext = ""
	// a paragraph that holds only shapes that float has the line of its
	// mark, as in Word
	floats := t.floats > 0 && t.floats == t.para.Len() && t.marker == ""
	if para := marker(t, s.par); para == "" {
		out.WriteString("<br/>")
	} else {
		out.WriteString(para)
		if floats {
			out.WriteString("<br/>")
		}
	}
	out.WriteString("</p>")
	t.para.Reset()
	t.floats = 0
}

// tableLevel is the table of a target at one level of nesting: the rows
// that wait for the end of the table, the cells of the current row, and
// the blocks of the current cell.
type tableLevel struct {
	open bool
	row  []string
	cell strings.Builder
	rows []tableRow
}

// maxNesting is the deepest level of the tables that nest. A table that
// nests deeper is part of the table at that level.
const maxNesting = 16

// level returns the table of the level n of t, from 1.
func (t *target) level(n int) *tableLevel {
	n = min(max(n, 1), maxNesting)
	for len(t.levels) < n {
		t.levels = append(t.levels, &tableLevel{})
	}
	return t.levels[n-1]
}

// paraLevel returns the level of the table of a paragraph with the
// properties p, from 1, or 0 for a paragraph out of a table.
func paraLevel(p paraProps) int {
	if !p.inTable {
		return 0
	}
	return min(max(p.itap, 1), maxNesting)
}

// cellLevel returns the level of the table of the cell or the row that
// \cell or \row ends, or \nestcell or \nestrow when nested is set.
func cellLevel(p paraProps, nested bool) int {
	if !nested {
		return 1
	}
	return max(paraLevel(p), 2)
}

// endCell ends the current cell of a row of a table at the level of
// cellLevel.
func (c *converter) endCell(s *state, nested bool) {
	c.flushBytes(s)
	t := s.target
	c.flushRun(t)
	lvl := cellLevel(s.par, nested)
	if t.para.Len() > 0 {
		s.par.inTable = true
		s.par.itap = max(s.par.itap, lvl)
		c.endParagraph(s)
	}
	c.closeTables(t, lvl)
	l := t.level(lvl)
	if len(l.row) < 1024 {
		l.row = append(l.row, l.cell.String())
	}
	l.cell.Reset()
}

// endRow ends the current row of a table at the level of cellLevel, with
// the definition of the row. The rows of a table wait for its end, as the
// edges of the cells of all its rows make its columns.
func (c *converter) endRow(s *state, nested bool) {
	c.flushBytes(s)
	t := s.target
	c.flushRun(t)
	lvl := cellLevel(s.par, nested)
	l := t.level(lvl)
	if t.para.Len() > 0 || l.cell.Len() > 0 {
		c.endCell(s, nested)
	}
	c.closeTables(t, lvl)
	if !l.open {
		if t == c.body && lvl == 1 {
			c.openSection()
		}
		l.open = true
	}
	def := c.row
	def.cells = append([]cellDef(nil), c.row.cells...)
	if len(l.rows) < c.lim.MaxObjects {
		l.rows = append(l.rows, tableRow{def: def, cells: append([]string(nil), l.row...)})
	}
	l.row = l.row[:0]
}

// tableRow is a row of a table that waits for the end of the table: its
// definition and the XHTML of its cells.
type tableRow struct {
	def   rowDef
	cells []string
}

// edges returns the edges of the cells of the row r, in twips, one more
// than its cells.
func (r *rowDef) edges() []int {
	out := make([]int, 0, len(r.cells)+1)
	out = append(out, r.left)
	for _, cell := range r.cells {
		out = append(out, max(cell.right, out[len(out)-1]))
	}
	return out
}

// closeTable closes the tables of a target.
func (c *converter) closeTable(t *target) {
	c.closeTables(t, 0)
}

// closeTables closes the tables of t that nest deeper than the level lvl,
// the deepest first. A table that nests goes into the current cell of the
// table around it.
func (c *converter) closeTables(t *target, lvl int) {
	for n := len(t.levels); n > lvl; n-- {
		l := t.levels[n-1]
		if !l.open {
			continue
		}
		out := &t.out
		if n > 1 {
			out = &t.level(n - 1).cell
		} else {
			c.closeFrame(t)
		}
		c.writeTable(out, l)
	}
}

// writeTable closes the table l, and writes its rows to out. Each cell
// covers the columns between its edges, as in Word, where each row has
// cells of its own.
func (c *converter) writeTable(out *strings.Builder, l *tableLevel) {
	l.open = false
	rows := l.rows
	l.rows = nil
	if len(rows) == 0 {
		return
	}
	var all []int
	for i := range rows {
		all = append(all, rows[i].def.edges()...)
	}
	sort.Ints(all)
	var grid []int
	for _, x := range all {
		if len(grid) == 0 || x-grid[len(grid)-1] >= 40 {
			grid = append(grid, x)
		}
	}
	column := func(x int) int {
		k := sort.SearchInts(grid, x)
		if k > 0 && (k == len(grid) || x-grid[k-1] < grid[k]-x) {
			k--
		}
		return k
	}
	first := rows[0].def
	var st css
	if len(grid) > 1 {
		st.set("width", pt(grid[len(grid)-1]-grid[0]))
	}
	switch first.align {
	case 'c':
		st.set("margin-left", "auto")
		st.set("margin-right", "auto")
	case 'r':
		st.set("margin-left", "auto")
	default:
		if len(grid) > 0 && grid[0] != 0 {
			st.set("margin-left", pt(grid[0]))
		}
	}
	out.WriteString(`<table style="` + attrText(st.String()) + `">`)
	if len(grid) > 1 {
		out.WriteString("<colgroup>")
		for k := 1; k < len(grid); k++ {
			out.WriteString(`<col style="width: ` + pt(grid[k]-grid[k-1]) + `"/>`)
		}
		out.WriteString("</colgroup>")
	}
	// the rows of \trhdr at the start of the table repeat at the top of
	// each page, and the cells of the header rows merge among themselves
	head := 0
	for head < len(rows) && rows[head].def.header {
		head++
	}
	if head > 0 && head < len(rows) {
		out.WriteString("<thead>")
		for i := range rows[:head] {
			c.writeRow(out, rows[:head], i, column, len(grid) > 1)
		}
		out.WriteString("</thead>")
		rows = rows[head:]
	}
	out.WriteString("<tbody>")
	for i := range rows {
		c.writeRow(out, rows, i, column, len(grid) > 1)
	}
	out.WriteString("</tbody></table>")
}

// defaultCellPadding is the padding at the left and the right of a cell
// that gives none, in twips: the padding of td in styleSheet.
const defaultCellPadding = 108

// rowSpan returns the number of rows from the row k that the cell of the
// row k whose left edge is x covers: the cell, and the cells of \clvmrg
// at the same edge in the rows below it.
func rowSpan(rows []tableRow, k, x int) int {
	n := 1
	for _, row := range rows[k+1:] {
		edges := row.def.edges()
		merged := false
		for i, d := range row.def.cells {
			if abs(edges[i]-x) < 40 {
				merged = d.vmerge == 'r'
				break
			}
		}
		if !merged {
			break
		}
		n++
	}
	return n
}

// writeRow writes the row k of the rows of a table. column returns the
// column of the grid at an edge, when grid is set.
func (c *converter) writeRow(out *strings.Builder, rows []tableRow, k int, column func(int) int, grid bool) {
	row := rows[k]
	r := row.def
	edges := r.edges()
	var rs css
	if r.height != 0 {
		h := r.height
		if h < 0 {
			h = -h
		}
		rs.set("height", pt(h))
	}
	out.WriteString(`<tr style="` + attrText(rs.String()) + `">`)
	at := 0
	if grid {
		if k := column(edges[0]); k > 0 {
			out.WriteString(`<td colspan="` + strconv.Itoa(k) + `" style="border: none"></td>`)
			at = k
		}
	}
	for i, html := range row.cells {
		var cs css
		span, vspan := 1, 1
		if i < len(r.cells) {
			def := r.cells[i]
			// the cells that \clmrg merges into this one
			end := i + 1
			for end < len(r.cells) && end < len(row.cells) && r.cells[end].merged {
				end++
			}
			if grid {
				span = max(column(edges[end])-at, 1)
			} else {
				span = end - i
			}
			if def.merged {
				continue
			}
			if def.vmerge == 'r' {
				// a cell that the cell above covers
				at += span
				continue
			}
			if def.vmerge == 'f' {
				vspan = rowSpan(rows, k, edges[i])
			}
			// widths holds the width of the border of each side in twips,
			// in the order top, right, bottom and left, or 0
			var widths [4]int
			for k, b := range [...]struct {
				on   bool
				side string
			}{{def.top, "top"}, {def.right2, "right"}, {def.bottom, "bottom"}, {def.left, "left"}} {
				if !b.on {
					continue
				}
				// \brdrw gives the width of the line, and a line without
				// it is half a point wide.
				widths[k] = 10
				if w := def.borderWidth[k]; w > 0 {
					widths[k] = w
				}
				color := c.color(def.borderColor[k])
				if color == "" {
					color = "black"
				}
				cs.set("border-"+b.side, pt(widths[k])+" solid "+color)
			}
			if v := c.color(def.shading); v != "" {
				cs.set("background-color", v)
			}
			switch def.valign {
			case 'c':
				cs.set("vertical-align", "middle")
			case 'b':
				cs.set("vertical-align", "bottom")
			}
			for k, side := range [...]string{"left", "right", "top", "bottom"} {
				v := def.pad[k]
				if v < 0 {
					v = r.pad[k]
				}
				if v < 0 && k < 2 {
					v = r.gap
				}
				if k < 2 {
					// Word draws a border on the line between two cells,
					// and keeps the whole cell for the padding and the
					// text. The layout takes half of a collapsed border
					// from the cell, so the padding gives it back
					if v < 0 {
						v = defaultCellPadding
					}
					v = max(v-widths[3-2*k]/2, 0)
				}
				if v >= 0 {
					cs.set("padding-"+side, pt(v))
				}
			}
		}
		at += span
		out.WriteString("<td")
		if span > 1 {
			out.WriteString(` colspan="` + strconv.Itoa(span) + `"`)
		}
		if vspan > 1 {
			out.WriteString(` rowspan="` + strconv.Itoa(vspan) + `"`)
		}
		out.WriteString(` style="` + attrText(cs.String()) + `">`)
		if html == "" {
			html = "<p><br/></p>"
		}
		out.WriteString(html + "</td>")
	}
	out.WriteString("</tr>")
}

// endTarget ends the paragraph, the rows and the tables of a target.
func (c *converter) endTarget(t *target) {
	c.flushRun(t)
	if t.para.Len() > 0 {
		s := &state{dest: destText, chr: t.runProps, par: defaultPara(), target: t}
		if t == c.body {
			c.openSection()
		}
		c.endParagraph(s)
	}
	if len(t.levels) > 0 && (len(t.levels[0].row) > 0 || t.levels[0].cell.Len() > 0) {
		s := &state{dest: destText, chr: t.runProps, par: paraProps{inTable: true}, target: t}
		c.endRow(s, false)
	}
	c.closeTable(t)
	c.closeFrame(t)
}

// openSection opens the element of the current section, if it is not open.
func (c *converter) openSection() {
	if c.open {
		return
	}
	c.open = true
	c.sectionNum++
	name := "s" + strconv.Itoa(c.sectionNum)
	var s css
	s.set("page", name)
	if c.sectionNum > 1 {
		switch c.section.breakKind {
		case "left", "right":
			s.set("break-before", c.section.breakKind)
		default:
			s.set("break-before", "page")
		}
	}
	if c.section.cols > 1 {
		s.set("columns", strconv.Itoa(c.section.cols))
		s.set("column-gap", pt(c.section.colSpace))
	}
	if v := c.section.lineNumbers(); v != "" {
		s.set("-ivy-line-numbers", v)
	}
	// The headers and the footers of the section before stay, until the
	// section gives its own.
	c.body.out.WriteString(`<div class="section" style="` + attrText(s.String()) + `">`)
	c.writeFixed()
}

// writeFixed writes the images of the WordArt of the headers and the
// footers into the open section, where they show on each page.
func (c *converter) writeFixed() {
	if !c.open {
		return
	}
	for _, html := range c.fixed {
		c.body.out.WriteString(html)
	}
	c.fixed = nil
}

// closeSection closes the element of the current section, and writes the
// rules of its pages.
func (c *converter) closeSection() {
	if !c.open {
		return
	}
	c.open = false
	sec := c.section
	name := "s" + strconv.Itoa(c.sectionNum)
	box := func(kinds ...string) string {
		for _, k := range kinds {
			if r, ok := c.running[k]; ok {
				return "element(" + r + ")"
			}
		}
		return "none"
	}
	width := min(max(sec.width, 1440), 288000)
	height := min(max(sec.height, 1440), 288000)
	// Word puts the text of a page below its header: the top of the text
	// is the larger of the top margin and the distance of the header from
	// the top of the paper plus the height of the header, and the same
	// holds for the footer at the bottom. The layout makes the margins of
	// a page larger when its header or its footer needs it.
	top, bottom := sec.top, sec.bottom
	fit := "none"
	if len(c.running) > 0 {
		fit = "grow"
	}
	margins := func(header, footer string) string {
		return "@top-center { content: " + header + "; padding-top: " + pt(min(sec.header, sec.top)) + " } " +
			"@bottom-center { content: " + footer + "; padding-bottom: " + pt(min(sec.footer, sec.bottom)) + " }"
	}
	size := pt(width) + " " + pt(height)
	if sec.paper {
		// The paper of the program, in the orientation of the section, as
		// exact points that twips round.
		p := c.d.cfg.Paper.Portrait()
		if width > height {
			p = p.Landscape()
		}
		size = num(min(max(float64(p.Width), 72), 14400)) + "pt " + num(min(max(float64(p.Height), 72), 14400)) + "pt"
	}
	c.sheet.WriteString("@page " + name + " { size: " + size +
		"; margin: " + pt(top) + " " + pt(sec.right) + " " + pt(bottom) + " " + pt(sec.left) +
		"; -ivy-margin-box-fit: " + fit + "; " +
		margins(box("header", "headerr"), box("footer", "footerr")) + " }\n")
	if _, ok := c.running["headerl"]; ok {
		c.sheet.WriteString("@page " + name + ":left { " + margins(box("headerl", "header"), box("footerl", "footer")) + " }\n")
	}
	if sec.titlePage {
		c.sheet.WriteString("@page " + name + ":nth(1 of " + name + ") { " + margins(box("headerf"), box("footerf")) + " }\n")
	}
	c.closeFrame(c.body)
	c.body.out.WriteString("</div>")
}

// endNested writes a header, a footer or a footnote that a group ended.
func (c *converter) endNested(parent, s *state) {
	html := s.target.out.String()
	if s.kind == "footnote" {
		c.inline(parent, `<span class="fn">`+html+`</span>`)
		return
	}
	// A header or a footer is a running element at the start of its
	// section.
	c.closeFrame(c.body)
	c.openSection()
	c.writeFixed()
	c.hfNum++
	name := "hf" + strconv.Itoa(c.hfNum)
	c.running[s.kind] = name
	// the element is a flow root, so that the space before its first
	// paragraph stays in it, and a footer that the layout aligns at the
	// bottom of its box does not move down by that space
	c.body.out.WriteString(`<div style="position: running(` + name + `); display: flow-root">` + html + `</div>`)
}

// listText writes the text of the marker of a paragraph of a list, from
// \listtext or \pntext. A marker of white space only is left out. A tab
// after the marker goes to the hanging indent of the paragraph.
func (c *converter) listText(parent, s *state) {
	html := s.target.para.String()
	plain := html
	for {
		start := strings.IndexByte(plain, '<')
		end := strings.IndexByte(plain, '>')
		if start < 0 || end < start {
			break
		}
		plain = plain[:start] + plain[end+1:]
	}
	if strings.TrimSpace(plain) == "" {
		return
	}
	c.flushBytes(parent)
	c.flushRun(parent.target)
	parent.target.marker = html
	parent.target.markerAt = parent.target.para.Len()
}

// marker returns the paragraph of t with its marker. The paragraph gives
// its indents after its marker, so the tab after the marker goes to the
// hanging indent here.
func marker(t *target, p paraProps) string {
	para := t.para.String()
	html := t.marker
	at := min(t.markerAt, len(para))
	t.marker = ""
	if html == "" {
		return para
	}
	if i := strings.LastIndexByte(html, '\t'); i >= 0 && p.first < 0 {
		html = html[:i] + html[i+1:]
		html = `<span style="display: inline-block; min-width: ` + pt(-p.first) + `; text-indent: 0">` + html + "</span>"
	}
	return para[:at] + html + para[at:]
}

// endPicture writes a picture that a group \pict ended.
func (c *converter) endPicture(parent *state, p *picture) {
	data := p.bin
	if len(data) == 0 {
		data = p.hex
	}
	if p.kind == "" || len(data) == 0 || parent.dest != destText && parent.dest != destShape || parent.chr.hidden {
		return
	}
	c.files++
	if c.files > c.lim.MaxObjects {
		return
	}
	name := "media/" + strconv.Itoa(c.files) + "." + p.kind
	c.fs.Add(name, append([]byte(nil), data...))
	// The size of the picture is the size of its shape, or its goal size
	// in twips, scaled.
	w := p.goalW * max(p.scaleX, 1) / 100
	h := p.goalH * max(p.scaleY, 1) / 100
	if w <= 0 || h <= 0 {
		// A picture without a goal size takes \picw and \pich: pixels for
		// a bitmap, and hundredths of a millimeter for a metafile.
		w, h = p.width*15, p.height*15
		if p.kind == "emf" || p.kind == "wmf" {
			w, h = p.width*1440/2540, p.height*1440/2540
		}
	}
	if parent.dest == destShape {
		// the picture of a shape that floats waits for the end of the
		// shape, which places it. Another one stays in its line, with the
		// size of its shape
		if p := c.shapeParent(); p != nil && c.floatsShape(p) {
			c.shape.pic, c.shape.picW, c.shape.picH = name, w, h
			return
		}
		if sw, sh := c.shape.box[2]-c.shape.box[0], c.shape.box[3]-c.shape.box[1]; sw > 0 && sh > 0 {
			w, h = sw, sh
		}
	}
	var s css
	sizeCSS(&s, w, h)
	c.inline(parent, imgHTML(name, s.String()))
}

// sizeCSS sets the width and the height of a box of w by h twips. It sets
// neither when w or h is not more than 0.
func sizeCSS(s *css, w, h int) {
	if w > 0 && h > 0 {
		s.set("width", pt(min(w, 288000)))
		s.set("height", pt(min(h, 288000)))
	}
}

// imgHTML returns the element of the picture name with the declarations
// style.
func imgHTML(name, style string) string {
	return `<img src="` + name + `" alt="" style="` + attrText(style) + `"/>`
}

// symbolChars maps the characters of the font Symbol to Unicode.
var symbolChars = map[rune]rune{
	'a': 'α', 'b': 'β', 'c': 'χ', 'd': 'δ', 'e': 'ε', 'f': 'φ', 'g': 'γ', 'h': 'η',
	'i': 'ι', 'k': 'κ', 'l': 'λ', 'm': 'μ', 'n': 'ν', 'p': 'π', 'q': 'θ', 'r': 'ρ',
	's': 'σ', 't': 'τ', 'u': 'υ', 'w': 'ω', 'x': 'ξ', 'y': 'ψ', 'z': 'ζ',
	'D': 'Δ', 'F': 'Φ', 'G': 'Γ', 'L': 'Λ', 'P': 'Π', 'Q': 'Θ', 'S': 'Σ', 'W': 'Ω',
	0xa3: '≤', 0xa5: '∞', 0xae: '→', 0xb0: '°', 0xb1: '±', 0xb3: '≥', 0xb4: '×',
	0xb7: '•', 0xb8: '÷', 0xb9: '≠', 0xbb: '≈', 0xd6: '√', 0xe5: '∑',
}

// symbolRune returns the Unicode character of a character of a symbol
// font. A character that it does not know is a bullet above 0x7f.
func symbolRune(r rune) rune {
	if u, ok := symbolChars[r]; ok {
		return u
	}
	if r >= 0x80 {
		return '•'
	}
	return r
}

// abs returns the absolute value of v.
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
