package rtf

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// headerLine is the height of a line of a header, in twips, for the
// margins of a page whose header does not fit.
const headerLine = 300

// lineRatios are the heights of a single line of the fonts that documents
// use most, as a multiple of the font size.
var lineRatios = map[string]float64{
	"calibri": 1.2207, "carlito": 1.2207, "cambria": 1.1719, "caladea": 1.1719,
	"arial": 1.1499, "liberation sans": 1.1499, "helvetica": 1.1499,
	"times new roman": 1.1499, "liberation serif": 1.1499, "times": 1.1499,
	"courier new": 1.1328, "liberation mono": 1.1328, "courier": 1.1328,
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
	e, ok := c.fonts[f]
	if !ok || strings.TrimSpace(e.name) == "" {
		return "", ""
	}
	name := strings.TrimSpace(e.name)
	generic := e.generic
	if generic == "" || generic == "symbol" {
		generic = "serif"
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range name {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	b.WriteString(`", ` + generic)
	return b.String(), name
}

// runStyle returns the style attribute of a run with the properties p. A
// paragraph has the font and the size of \plain, so a run states the
// properties that differ from \plain.
func (c *converter) runStyle(p charProps) string {
	var s css
	if p.font != c.docFont {
		family, _ := c.family(p.font)
		s.set("font-family", family)
	}
	size := float64(p.size) / 2
	if p.vertical != 0 {
		size *= 0.65
	}
	if size != 12 {
		s.set("font-size", num(size)+"pt")
	}
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
	// The paragraph has the font and the size of \plain, and its runs
	// state theirs. The mark of the paragraph gives the height of its
	// lines.
	family, _ := c.family(c.docFont)
	s.set("font-family", family)
	s.set("font-size", "12pt")
	_, name := c.family(ch.font)
	size := float64(ch.size) / 2
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
	// The spacing before is a padding, which does not collapse with the
	// margin of the spacing after. The spacing after is a margin, which the
	// bottom of a page cuts, as in Word.
	if p.before > 0 {
		s.set(map[bool]string{true: "margin-top", false: "padding-top"}[boxed], pt(p.before))
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
	case p.line > 0 && float64(p.line)/20 > ratio*size:
		s.set("line-height", pt(p.line))
	case p.line < 0:
		s.set("line-height", pt(-p.line))
	default:
		s.set("line-height", num(ratio))
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
	return s.String()
}

// endParagraph ends the current paragraph of the target of s.
func (c *converter) endParagraph(s *state) {
	c.flushBytes(s)
	t := s.target
	c.flushRun(t)
	if !s.par.inTable && t.table {
		c.closeTable(t)
	}
	out := &t.out
	if s.par.inTable {
		out = &t.cell
	}
	if t == c.body && !s.par.inTable {
		c.openSection()
	}
	out.WriteString(`<p style="` + attrText(c.paraStyle(s.par, s.chr, t.breakNext)) + `">`)
	t.breakNext = ""
	if para := marker(t, s.par); para == "" {
		out.WriteString("<br/>")
	} else {
		out.WriteString(para)
	}
	out.WriteString("</p>")
	t.para.Reset()
}

// endCell ends the current cell of a row.
func (c *converter) endCell(s *state) {
	c.flushBytes(s)
	t := s.target
	c.flushRun(t)
	if t.para.Len() > 0 {
		s.par.inTable = true
		c.endParagraph(s)
	}
	if len(t.row) < 1024 {
		t.row = append(t.row, t.cell.String())
	}
	t.cell.Reset()
}

// endRow ends the current row of a table, with the definition of the row.
// The rows of a table wait for its end, as the edges of the cells of all
// its rows make its columns.
func (c *converter) endRow(s *state) {
	c.flushBytes(s)
	t := s.target
	c.flushRun(t)
	if t.para.Len() > 0 || t.cell.Len() > 0 {
		c.endCell(s)
	}
	if !t.table {
		if t == c.body {
			c.openSection()
		}
		t.table = true
	}
	def := c.row
	def.cells = append([]cellDef(nil), c.row.cells...)
	if len(t.rows) < c.lim.MaxObjects {
		t.rows = append(t.rows, tableRow{def: def, cells: append([]string(nil), t.row...)})
	}
	t.row = t.row[:0]
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

// closeTable closes the table of a target, and writes its rows. Each cell
// covers the columns between its edges, as in Word, where each row has
// cells of its own.
func (c *converter) closeTable(t *target) {
	if !t.table {
		return
	}
	t.table = false
	rows := t.rows
	t.rows = nil
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
	t.out.WriteString(`<table style="` + attrText(st.String()) + `">`)
	if len(grid) > 1 {
		t.out.WriteString("<colgroup>")
		for k := 1; k < len(grid); k++ {
			t.out.WriteString(`<col style="width: ` + pt(grid[k]-grid[k-1]) + `"/>`)
		}
		t.out.WriteString("</colgroup>")
	}
	// the rows of \trhdr at the start of the table repeat at the top of
	// each page
	head := 0
	for head < len(rows) && rows[head].def.header {
		head++
	}
	if head > 0 && head < len(rows) {
		t.out.WriteString("<thead>")
		for _, row := range rows[:head] {
			c.writeRow(t, row, column, len(grid) > 1)
		}
		t.out.WriteString("</thead>")
		rows = rows[head:]
	}
	t.out.WriteString("<tbody>")
	for _, row := range rows {
		c.writeRow(t, row, column, len(grid) > 1)
	}
	t.out.WriteString("</tbody></table>")
}

// writeRow writes a row of a table. column returns the column of the grid
// at an edge, when grid is set.
func (c *converter) writeRow(t *target, row tableRow, column func(int) int, grid bool) {
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
	t.out.WriteString(`<tr style="` + attrText(rs.String()) + `">`)
	at := 0
	if grid {
		if k := column(edges[0]); k > 0 {
			t.out.WriteString(`<td colspan="` + strconv.Itoa(k) + `" style="border: none"></td>`)
			at = k
		}
	}
	for i, html := range row.cells {
		var cs css
		span := 1
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
			for _, b := range [...]struct {
				on   bool
				side string
			}{{def.top, "top"}, {def.right2, "right"}, {def.bottom, "bottom"}, {def.left, "left"}} {
				if b.on {
					cs.set("border-"+b.side, "0.5pt solid black")
				}
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
				if v >= 0 {
					cs.set("padding-"+side, pt(v))
				}
			}
		}
		at += span
		t.out.WriteString("<td")
		if span > 1 {
			t.out.WriteString(` colspan="` + strconv.Itoa(span) + `"`)
		}
		t.out.WriteString(` style="` + attrText(cs.String()) + `">`)
		if html == "" {
			html = "<p><br/></p>"
		}
		t.out.WriteString(html + "</td>")
	}
	t.out.WriteString("</tr>")
}

// endTarget ends the paragraph, the row and the table of a target.
func (c *converter) endTarget(t *target) {
	c.flushRun(t)
	if t.para.Len() > 0 {
		s := &state{dest: destText, chr: t.runProps, par: defaultPara(), target: t}
		if t == c.body {
			c.openSection()
		}
		c.endParagraph(s)
	}
	if len(t.row) > 0 || t.cell.Len() > 0 {
		s := &state{dest: destText, chr: t.runProps, par: paraProps{inTable: true}, target: t}
		c.endRow(s)
	}
	c.closeTable(t)
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
	// The headers and the footers of the section before stay, until the
	// section gives its own.
	c.body.out.WriteString(`<div class="section" style="` + attrText(s.String()) + `">`)
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
	// A header that does not fit above the text pushes the text down, by
	// an estimate of one line.
	top, bottom := sec.top, sec.bottom
	if box("header", "headerr", "headerl") != "none" {
		top = max(top, sec.header+headerLine)
	}
	if box("footer", "footerr", "footerl") != "none" {
		bottom = max(bottom, sec.footer+headerLine)
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
		"; margin: " + pt(top) + " " + pt(sec.right) + " " + pt(bottom) + " " + pt(sec.left) + "; " +
		margins(box("header", "headerr"), box("footer", "footerr")) + " }\n")
	if _, ok := c.running["headerl"]; ok {
		c.sheet.WriteString("@page " + name + ":left { " + margins(box("headerl", "header"), box("footerl", "footer")) + " }\n")
	}
	if sec.titlePage {
		c.sheet.WriteString("@page " + name + ":nth(1 of " + name + ") { " + margins(box("headerf"), box("footerf")) + " }\n")
	}
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
	c.openSection()
	c.hfNum++
	name := "hf" + strconv.Itoa(c.hfNum)
	c.running[s.kind] = name
	c.body.out.WriteString(`<div style="position: running(` + name + `)">` + html + `</div>`)
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
	if parent.dest == destShape {
		if sw, sh := c.shapeBox[2]-c.shapeBox[0], c.shapeBox[3]-c.shapeBox[1]; sw > 0 && sh > 0 {
			w, h = sw, sh
		}
	}
	if w <= 0 || h <= 0 {
		// A picture without a goal size takes \picw and \pich: pixels for
		// a bitmap, and hundredths of a millimeter for a metafile.
		w, h = p.width*15, p.height*15
		if p.kind == "emf" || p.kind == "wmf" {
			w, h = p.width*1440/2540, p.height*1440/2540
		}
	}
	var s css
	if w > 0 && h > 0 {
		s.set("width", pt(min(w, 288000)))
		s.set("height", pt(min(h, 288000)))
	}
	c.inline(parent, `<img src="`+name+`" alt="" style="`+attrText(s.String())+`"/>`)
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
