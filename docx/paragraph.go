package docx

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/ooxml"
)

// cellStyle holds the properties that a table style gives to the
// paragraphs and the runs of a cell.
type cellStyle struct {
	ppr, rpr props
}

// tag is an inline element that is open: its start tag and its name.
type tag struct {
	start, name string
}

// block converts a block of the body, of a cell, of a note or of a header.
func (c *converter) block(n roxmltree.Node, tc *cellStyle) {
	if c.check() != nil {
		return
	}
	switch n.TagName().Name() {
	case "p":
		c.paragraph(n, tc)
	case "tbl":
		c.table(n)
	case "sdt":
		if content, ok := child(n, "sdtContent"); ok {
			c.blocks(content, tc)
		}
	case "customXml", "ins", "moveTo", "sdtContent", "txbxContent":
		c.blocks(n, tc)
	case "AlternateContent":
		if choice, ok := alternative(n); ok {
			c.blocks(choice, tc)
		}
	case "altChunk":
		c.log.Debug("docx: alternative content left out")
	}
}

// blocks converts the children of n as blocks.
func (c *converter) blocks(n roxmltree.Node, tc *cellStyle) {
	if !c.enter() {
		return
	}
	defer c.leave()
	last, skip := cellEnd(n)
	frame := ""
	for e := range n.Children() {
		if !e.IsElement() || skip && e.ID() == last {
			continue
		}
		c.openFrame(e, tc, &frame)
		c.block(e, tc)
	}
	c.closeFrame(&frame)
}

// openFrame opens the frame of the block e, and closes the frame of the
// blocks before when e is not in it. frame is the key of the open frame,
// or "". The paragraphs of the same frame share a block, see frameOf.
func (c *converter) openFrame(e roxmltree.Node, tc *cellStyle, frame *string) {
	key, style := "", ""
	if tc == nil && e.TagName().Name() == "p" {
		key, style = c.frameOf(e)
	}
	if key == *frame {
		return
	}
	c.closeFrame(frame)
	if key != "" {
		c.w.WriteString(`<div style="` + attrText(style) + `">`)
	}
	*frame = key
}

// closeFrame closes the frame of the key frame, when it is not "".
func (c *converter) closeFrame(frame *string) {
	if *frame != "" {
		c.w.WriteString("</div>")
		*frame = ""
	}
}

// frameOf returns the key and the style of the frame of the paragraph p,
// from the w:framePr of its properties, or "" for a paragraph that is not
// in such a frame. The paragraphs of the same key share the frame. Only a
// frame at a position from the margin or from the page, with text around
// it or above and below it, in the body of a section of one column, is a
// frame here. It floats at its position, see pageFloatCSS (D60). A frame
// of the width of its text takes the width of the column. A frame of a
// drop cap stays a paragraph.
func (c *converter) frameOf(p roxmltree.Node) (key, style string) {
	ppr, ok := child(p, "pPr")
	if !ok || c.margin != "" {
		return "", ""
	}
	fp, ok := child(ppr, "framePr")
	if !ok {
		return "", ""
	}
	attr := func(name string) string { return attrOf(fp, name) }
	if d := attr("dropCap"); d != "" && d != "none" || attr("wrap") == "none" {
		return "", ""
	}
	sec := c.sections[c.section]
	position := floatPosition(attr("vAnchor"), attr("yAlign"), number(attr("y"), 0)/20)
	if position == "" || sec.cols > 1 {
		return "", ""
	}
	colWidth := sec.width - sec.left - sec.right - sec.gutter
	w := min(max(number(attr("w"), 0)/20, 0), maxDrawingSize)
	if w == 0 {
		w = colWidth
	}
	x := anchorX(attr("hAnchor"), attr("xAlign"), number(attr("x"), 0)/20, w, colWidth, sec)
	h := min(max(number(attr("h"), 0)/20, 0), maxDrawingSize)
	hs := min(max(number(attr("hSpace"), 0)/20, 0), 720)
	vs := min(max(number(attr("vSpace"), 0)/20, 0), 720)
	var s css
	s.set("width", pt(w))
	switch {
	case h > 0 && attr("hRule") == "exact":
		s.set("height", pt(h))
	case h > 0:
		s.set("min-height", pt(h))
	}
	pageFloatCSS(&s, x, w, colWidth, [4]float64{vs, hs, vs, hs}, attr("wrap") != "notBeside", position)
	var k strings.Builder
	for _, name := range [...]string{"w", "h", "hRule", "wrap", "vAnchor", "hAnchor", "x", "xAlign", "y", "yAlign", "hSpace", "vSpace"} {
		k.WriteString(attr(name))
		k.WriteByte(0)
	}
	return k.String(), s.String()
}

// cellEnd returns the empty paragraph that ends the cell n after a table,
// which takes no height in Word, and false when there is none. A cell must
// end with a paragraph, so Word writes one after a table that ends a cell.
func cellEnd(n roxmltree.Node) (roxmltree.NodeID, bool) {
	if n.TagName().Name() != "tc" {
		return 0, false
	}
	prev, last := "", ""
	var id roxmltree.NodeID
	var p roxmltree.Node
	for e := range n.Children() {
		if e.IsElement() {
			prev, last = last, e.TagName().Name()
			id, p = e.ID(), e
		}
	}
	if prev == "tbl" && last == "p" && empty(p) {
		return id, true
	}
	return 0, false
}

// alternative returns the content of mc:AlternateContent that the
// conversion uses: the first mc:Choice, else the mc:Fallback.
func alternative(n roxmltree.Node) (roxmltree.Node, bool) {
	if choice, ok := child(n, "Choice"); ok {
		return choice, true
	}
	return child(n, "Fallback")
}

// contentNames are the elements of a paragraph that show something.
var contentNames = map[string]bool{
	"t": true, "tab": true, "ptab": true, "drawing": true, "pict": true, "object": true,
	"sym": true, "br": true, "cr": true, "footnoteReference": true, "endnoteReference": true,
	"fldSimple": true, "noBreakHyphen": true, "oMath": true,
}

// empty reports whether the paragraph p shows nothing but its mark. The
// properties of p do not count, so a w:tab of its w:tabs is no content.
func empty(p roxmltree.Node) bool {
	for e := range p.Children() {
		if !e.IsElement() || e.TagName().Name() == "pPr" {
			continue
		}
		for d := range e.Descendants() {
			if d.IsElement() && contentNames[d.TagName().Name()] {
				return false
			}
		}
	}
	return true
}

// styleID returns the id of the style of the paragraph p, or "".
func styleID(p roxmltree.Node) string {
	if ppr, ok := child(p, "pPr"); ok {
		if s, ok := child(ppr, "pStyle"); ok {
			return ooxml.Attr(s, "val")
		}
	}
	return ""
}

// paragraph converts a w:p element.
func (c *converter) paragraph(p roxmltree.Node, tc *cellStyle) {
	if c.endsBeforePage(p) {
		return
	}
	if t, ok := c.hf.tops[p.ID()]; ok && c.margin != "" {
		c.hfTop = t
	}
	direct, mark := props{}, props{}
	if ppr, ok := child(p, "pPr"); ok {
		direct = parsePPr(ppr)
		if r, ok := child(ppr, "rPr"); ok {
			mark = parseRPr(r)
		}
	}
	sppr, srpr := props{}, props{}
	if st := c.styles.get(direct["pStyle"], "paragraph"); st != nil {
		sppr, srpr = st.ppr, st.rpr
	}
	tblP, tblR := props{}, props{}
	if tc != nil {
		tblP, tblR = tc.ppr, tc.rpr
	}
	base := c.styles.pDefault.over(tblP).over(sppr)
	merged := base.over(direct)
	var marker string
	var lvl *level
	if id := merged["numId"]; id != "" && id != "0" {
		ilvl, _ := strconv.Atoi(merged["ilvl"])
		if m, l, ok := c.numbering.next(id, ilvl, c.styles); ok {
			marker, lvl = m, l
		}
	}
	ppr := base
	if lvl != nil {
		ppr = ppr.over(lvl.ppr)
	}
	ppr = ppr.over(direct)
	rpr := c.styles.rDefault.over(tblR).over(srpr)
	isEmpty := empty(p)
	if isEmpty {
		rpr = rpr.over(mark)
	}
	if rpr.on("vanish") && isEmpty {
		return
	}
	// The paragraph takes the font of its largest run, so that the strut
	// of its lines has the height of a line of that run, as the lines of
	// Word have. A strut of another font makes the lines taller. The runs
	// take their properties from rpr, and write the font of their own when
	// it is not the font of the paragraph.
	line := c.lineFont(p, rpr)
	lineParent := c.lineParent
	c.lineParent = line
	defer func() { c.lineParent = lineParent }()

	style := c.paragraphStyle(p, ppr, line)
	start := `<p style="` + attrText(style) + `">`
	para, open := c.para, c.open
	c.para, c.open = start, nil
	c.paraSeq++
	defer func() { c.para, c.open = para, open }()
	c.w.WriteString(start)
	mark0 := c.w.Len()
	if lvl != nil && marker != "" {
		mr := rpr.over(mark).over(lvl.rpr)
		if font := c.font(mr["font"]); isSymbolFont(font) {
			codes, ok := symbolCodes(lvl.text)
			if lvl.format == "bullet" && ok && c.drawsAll(font, codes) {
				// The program has the symbol font, which draws the
				// bullet as Word does.
				marker = codes
			} else {
				// The marker holds Unicode characters, which the font of
				// the text draws.
				mr["font"] = rpr["font"]
			}
		}
		c.marker(marker, lvl, ppr, mr, rpr)
	}
	c.inlines(p, rpr)
	for i := len(c.open) - 1; i >= 0; i-- {
		c.w.WriteString("</" + c.open[i].name + ">")
	}
	switch {
	case c.w.Len() == mark0:
		// An empty paragraph takes the height of one line, as in Word.
		c.w.WriteString("<br/>")
	case endsWithBreak(c.w.String()[mark0:]):
		// In Word, the mark of a paragraph after a break of a line is on
		// a line of its own. In HTML, a break at the end of a block
		// starts no line.
		c.w.WriteString("<br/>")
	}
	c.w.WriteString("</p>")
}

// endsWithBreak reports whether the HTML s ends with a break of a line,
// before the end tags of its inline elements.
func endsWithBreak(s string) bool {
	for {
		i := strings.LastIndex(s, "</")
		if i < 0 || !strings.HasSuffix(s, ">") {
			break
		}
		switch s[i+2 : len(s)-1] {
		case "span", "a", "sup", "sub":
			s = s[:i]
			continue
		}
		break
	}
	return strings.HasSuffix(s, "<br/>")
}

// attrText returns s escaped for an attribute.
func attrText(s string) string {
	var b strings.Builder
	escape(&b, s, true)
	return b.String()
}

// hasSectPr reports whether the paragraph p ends a section, with a
// w:sectPr in its w:pPr.
func hasSectPr(p roxmltree.Node) bool {
	ppr, ok := child(p, "pPr")
	if !ok {
		return false
	}
	_, ok = child(ppr, "sectPr")
	return ok
}

// endsBeforePage reports whether the paragraph p only ends a section of
// the body, with nothing in it, and the next section starts a new page. Its
// line shows nothing, and the layout leaves it out. Word keeps such a
// paragraph on the page of its section, even when the bottom of the page
// has no room for its line, and the line would otherwise make a blank page
// before the next section.
func (c *converter) endsBeforePage(p roxmltree.Node) bool {
	if c.margin != "" || c.nested > 0 || c.section+1 >= len(c.sections) || !hasSectPr(p) || !empty(p) {
		return false
	}
	if parent, ok := p.ParentElement(); !ok || parent.TagName().Name() != "body" {
		return false
	}
	return c.sections[c.section+1].typ != "continuous"
}

// lineFont returns the properties rpr of the runs of the paragraph p with
// the font and the size of most of its text. A run of a larger font makes
// its own line taller, as in Word.
func (c *converter) lineFont(p roxmltree.Node, rpr props) props {
	type font struct{ font, ea, sz string }
	count := map[font]int{}
	runs := map[font]props{}
	var best props
	most := 0
	n := 0
	for d := range p.Descendants() {
		if !d.IsElement() || d.TagName().Name() != "r" || inDrawing(d, p) {
			continue
		}
		if n++; n > 256 {
			break
		}
		chars := 0
		for t := range d.Children() {
			if t.IsElement() && t.TagName().Name() == "t" {
				v, _ := t.Text()
				chars += len(v)
			}
		}
		if chars == 0 {
			continue
		}
		run := rpr
		if r, ok := child(d, "rPr"); ok {
			direct := parseRPr(r)
			if id := direct["rStyle"]; id != "" {
				if st := c.styles.get(id, "character"); st != nil {
					run = run.over(st.rpr)
				}
			}
			run = run.over(direct)
		}
		if run.on("vanish") || run["vertAlign"] == "superscript" || run["vertAlign"] == "subscript" {
			continue
		}
		k := font{run["font"], run["font.ea"], run["sz"]}
		if _, ok := runs[k]; !ok {
			runs[k] = run
		}
		count[k] += chars
		if count[k] > most {
			best, most = runs[k], count[k]
		}
	}
	if best == nil {
		return rpr
	}
	out := rpr.over(props{})
	for _, k := range [...]string{"font", "font.ea", "sz"} {
		if v, ok := best[k]; ok {
			out[k] = v
		} else {
			delete(out, k)
		}
	}
	return out
}

// inDrawing reports whether the run r of the paragraph p is in a drawing or
// a text box of p, which has paragraphs of its own.
func inDrawing(r, p roxmltree.Node) bool {
	for a := range r.Ancestors() {
		if a == p {
			return false
		}
		switch a.TagName().Name() {
		case "drawing", "pict", "object", "txbxContent":
			return true
		}
	}
	return false
}

// inCell reports whether the paragraph p is in a cell of a table.
func inCell(p roxmltree.Node) bool {
	parent, ok := p.ParentElement()
	return ok && parent.TagName().Name() == "tc"
}

// paragraphStyle returns the style attribute of a paragraph with the
// properties ppr, whose runs have the properties rpr.
func (c *converter) paragraphStyle(p roxmltree.Node, ppr, rpr props) string {
	var s css
	for _, d := range c.inherited(rpr) {
		s.set(d.name, d.value)
	}
	left := ppr.num("ind.left", 0) / 20
	right := ppr.num("ind.right", 0) / 20
	indent := ppr.num("ind.firstLine", 0)/20 - ppr.num("ind.hanging", 0)/20
	before := ppr.num("spacing.before", 0) / 20
	after := ppr.num("spacing.after", 0) / 20
	// The automatic spacing of HTML is 14 points, which collapses with the
	// spacing of the paragraph before, as the margins of HTML do. The first
	// paragraph of a cell has none before, and the last none after.
	autoBefore := ppr.on("spacing.beforeAutospacing")
	if autoBefore {
		before = 14
		if prev, ok := p.PrevSiblingElement(); !ok && inCell(p) || ok && prev.TagName().Name() == "tcPr" {
			before = 0
		}
	}
	if ppr.on("spacing.afterAutospacing") {
		after = 14
		if _, ok := p.NextSiblingElement(); !ok && inCell(p) {
			after = 0
		}
	}
	if ppr.on("contextualSpacing") {
		id := styleID(p)
		if prev, ok := p.PrevSiblingElement(); ok && prev.TagName().Name() == "p" && styleID(prev) == id {
			before = 0
		}
		if next, ok := p.NextSiblingElement(); ok && next.TagName().Name() == "p" && styleID(next) == id {
			after = 0
		}
	}
	boxed := ppr["shd"] != "" && ppr["shd"] != "none"
	for _, side := range [...]string{"top", "right", "bottom", "left"} {
		v, space := border(ppr["bdr."+side])
		if v == "" {
			continue
		}
		boxed = true
		s.set("border-"+side, v)
		s.set("padding-"+side, pt(space))
		if side == "left" {
			left -= space
		}
	}
	if left != 0 {
		s.set("margin-left", pt(left))
	}
	if right != 0 {
		s.set("margin-right", pt(right))
	}
	if indent != 0 {
		s.set("text-indent", pt(indent))
	}
	// Between two paragraphs, Word puts the larger of the spacing after
	// the first one and the spacing before the second one, as the margins
	// of CSS collapse. With addSpacing, Word adds the two, and the spacing
	// before is a padding, which does not collapse with the margin of the
	// spacing after. A paragraph with a border or a shading keeps a margin,
	// which its border and its shading do not hold. The spacing after is a
	// margin, which the bottom of a page cuts, as in Word.
	if before > 0 {
		s.set(pick(boxed || autoBefore || !c.addSpacing, "margin-top", "padding-top"), pt(before))
	}
	if after > 0 {
		s.set("margin-bottom", pt(after))
	}
	if v, ok := hexColor(ppr["shd"]); ok {
		s.set("background-color", v)
	}
	s.set("line-height", c.lineHeight(ppr, rpr))
	if m := c.lineMultiple(ppr, rpr); m != "" {
		// Word puts the line gap of a single line above its text, and the
		// extra space of the lines below it
		s.set("-ivy-line-multiple", m)
	}
	if ppr.on("suppressLineNumbers") {
		s.set("-ivy-line-numbers", "none")
	}
	switch ppr["jc"] {
	case "center":
		s.set("text-align", "center")
	case "right", "end":
		s.set("text-align", "right")
	case "both", "distribute", "lowKashida", "mediumKashida", "highKashida", "thaiDistribute":
		s.set("text-align", "justify")
	default:
		s.set("text-align", "left")
	}
	// An empty paragraph that only ends a section does not break the page
	// before it, as in LibreOffice: the section after it starts the page.
	if ppr.on("pageBreakBefore") && (!empty(p) || !hasSectPr(p)) {
		s.set("break-before", "page")
	}
	if ppr.on("keepNext") {
		s.set("break-after", "avoid")
	}
	if ppr.on("keepLines") {
		s.set("break-inside", "avoid")
	}
	if v, ok := ppr["widowControl"]; ok && !ppr.on("widowControl") && v != "" {
		s.set("orphans", "1")
		s.set("widows", "1")
	}
	s.set("tab-size", pt(c.defaultTab))
	s.set("-ivy-tab-stops", tabStops(ppr))
	// Word adds a space between East Asian text and Latin text, and
	// between East Asian text and digits, unless w:autoSpaceDE and
	// w:autoSpaceDN turn it off (ECMA-376 Part 1, 17.3.1.2 and 17.3.1.3).
	// The style sheet turns both on.
	if v, ok := autospace(ppr); ok {
		s.set("-ivy-text-autospace", v)
	}
	return s.String()
}

// autospace returns the value of -ivy-text-autospace of a paragraph with
// the properties ppr, and false when it is the value of the style sheet,
// with both spaces.
func autospace(ppr props) (string, bool) {
	off := func(name string) bool {
		_, ok := ppr[name]
		return ok && !ppr.on(name)
	}
	var v []string
	if !off("autoSpaceDE") {
		v = append(v, "ideograph-alpha")
	}
	if !off("autoSpaceDN") {
		v = append(v, "ideograph-numeric")
	}
	switch len(v) {
	case 2:
		return "", false
	case 0:
		return "none", true
	}
	return v[0], true
}

// tabLeaders maps the leaders of the tab stops of Word to the text that
// fills a tab (ECMA-376 Part 1, 17.18.85).
var tabLeaders = map[string]string{
	"dot":        ".",
	"hyphen":     "-",
	"underscore": "_",
	"heavy":      "_",
	"middleDot":  "\b7",
}

// tabStops returns the value of -ivy-tab-stops of a paragraph with the
// properties ppr: its custom tab stops, from the left margin, and the
// hanging indent, which Word uses as a stop too. Without a stop, it is
// "auto", so that the default stops of tab-size also start at the left
// margin, as in Word.
func tabStops(ppr props) string {
	type stop struct {
		pos           int
		align, leader string
	}
	var stops []stop
	for k, v := range ppr {
		pos, ok := strings.CutPrefix(k, "tab.")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(pos)
		if err != nil {
			continue
		}
		val, leader, _ := strings.Cut(v, " ")
		align := ""
		switch val {
		case "left", "start", "num":
			align = "left"
		case "right", "end":
			align = "right"
		case "center", "decimal":
			align = val
		default:
			// "clear" removes a stop, and "bar" draws a line that is not
			// a stop.
			continue
		}
		stops = append(stops, stop{n, align, tabLeaders[leader]})
	}
	if hanging := ppr.num("ind.hanging", 0); hanging > 0 {
		left := int(ppr.num("ind.left", 0))
		if !slices.ContainsFunc(stops, func(s stop) bool { return s.pos == left }) {
			stops = append(stops, stop{left, "left", ""})
		}
	}
	if len(stops) == 0 {
		return "auto"
	}
	slices.SortFunc(stops, func(a, b stop) int { return a.pos - b.pos })
	// Word keeps at most 64 custom stops in a paragraph.
	stops = stops[:min(len(stops), 64)]
	var b strings.Builder
	for i, s := range stops {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(pt(float64(s.pos) / 20))
		b.WriteString(" " + s.align)
		if s.leader != "" {
			b.WriteString(" '" + s.leader + "'")
		}
	}
	return b.String()
}

// lineHeight returns the value of line-height of a paragraph.
func (c *converter) lineHeight(ppr, rpr props) string {
	line := ppr.num("spacing.line", 240)
	ratio := c.lineRatio(rpr)
	switch ppr["spacing.lineRule"] {
	case "exact":
		if line > 0 {
			return pt(line / 20)
		}
	case "atLeast":
		if line/20 > ratio*fontSize(rpr) {
			return pt(line / 20)
		}
		return num(ratio)
	}
	if line <= 0 {
		line = 240
	}
	// On a grid of lines, Word gives a line as many lines of the grid as
	// its font needs, and multiplies them for the spacing (ECMA-376 Part 1,
	// 17.6.5).
	if pitch := c.gridPitch(ppr); pitch > 0 {
		n := math.Ceil(ratio*fontSize(rpr)/pitch - 0.01)
		return pt(n * pitch * min(line/240, 10))
	}
	return num(min(line/240, 10) * ratio)
}

// lineMultiple returns the value of -ivy-line-multiple of a paragraph
// whose lines are a multiple of a single line, or "" for lines of a height
// in points. A line of the rule "atLeast" that is not higher than a single
// line is a single line, which does not snap to the grid of lines. Word
// centers the text of a single line in the lines of a grid, as CSS does,
// so a single line on a grid has no value.
func (c *converter) lineMultiple(ppr, rpr props) string {
	line := ppr.num("spacing.line", 240)
	switch ppr["spacing.lineRule"] {
	case "", "auto":
	case "atLeast":
		if line/20 > c.lineRatio(rpr)*fontSize(rpr) {
			return ""
		}
		return "1"
	default:
		return ""
	}
	if line <= 0 {
		line = 240
	}
	if line <= 240 && c.gridPitch(ppr) > 0 {
		return ""
	}
	return num(min(line/240, 10))
}

// gridPitch returns the pitch of the grid of lines that the lines of a
// paragraph snap to, or 0.
func (c *converter) gridPitch(ppr props) float64 {
	if c.margin != "" || ppr["snapToGrid"] == "0" || ppr["snapToGrid"] == "false" || ppr["snapToGrid"] == "off" {
		return 0
	}
	return c.linePitch()
}

// linePitch returns the pitch of the grid of lines of the current
// section, or 0.
func (c *converter) linePitch() float64 {
	if c.section < len(c.sections) {
		return c.sections[c.section].linePitch
	}
	return 0
}

// marker writes the marker of a paragraph of a list, with the properties
// mr, in a paragraph whose runs have the properties rpr.
func (c *converter) marker(text string, lvl *level, ppr, mr, rpr props) {
	var s css
	hanging := ppr.num("ind.hanging", 0) / 20
	suffix := ""
	switch lvl.suffix {
	case "space":
		suffix = " "
	case "nothing":
	default:
		if hanging > 0 {
			s.set("display", "inline-block")
			s.set("min-width", pt(markerTab(ppr)-(ppr.num("ind.left", 0)/20-hanging)))
			s.set("text-indent", "0")
		} else {
			suffix = "\t"
		}
	}
	style := s.String()
	if run := c.runStyle(mr, rpr); run != "" {
		if style != "" {
			style += "; "
		}
		style += run
	}
	c.w.WriteString(`<span style="` + attrText(style) + `">`)
	escape(c.w, text+suffix, false)
	c.w.WriteString("</span>")
}

// markerTab returns the place in points where the text after the number of
// a paragraph with the properties ppr and a hanging indent starts: the
// first tab stop between the start of the number and the left indent, else
// the left indent, as in Word.
func markerTab(ppr props) float64 {
	left := ppr.num("ind.left", 0) / 20
	start := left - ppr.num("ind.hanging", 0)/20
	stop := left
	for k, v := range ppr {
		pos, ok := strings.CutPrefix(k, "tab.")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(pos)
		if err != nil {
			continue
		}
		switch val, _, _ := strings.Cut(v, " "); val {
		case "left", "start", "num":
			if x := float64(n) / 20; x > start && x < stop {
				stop = x
			}
		}
	}
	return stop
}

// inlines converts the inline content of n: the runs, the links, the
// fields and the changes.
func (c *converter) inlines(n roxmltree.Node, base props) {
	if !c.enter() {
		return
	}
	defer c.leave()
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "r":
			c.run(e, base)
		case "hyperlink":
			c.hyperlink(e, base)
		case "fldSimple":
			c.simpleField(e, base)
		case "ins", "moveTo", "smartTag", "customXml", "dir", "bdo", "sdtContent":
			c.inlines(e, base)
		case "sdt":
			if content, ok := child(e, "sdtContent"); ok {
				c.inlines(content, base)
			}
		case "AlternateContent":
			if choice, ok := alternative(e); ok {
				c.inlines(choice, base)
			}
		case "oMath", "oMathPara":
			c.math(e, base)
		}
	}
}

// text writes a run of text with the properties p in a paragraph whose
// runs have the properties base.
func (c *converter) text(s string, p, base props) {
	if s == "" || c.fieldHidden() || p.on("vanish") {
		return
	}
	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
	style := c.runStyle(p, base)
	// The East Asian language selects the font of the East Asian text.
	lang := ""
	if v := p["lang.ea"]; v != "" && strings.ContainsFunc(s, fontdb.IsCJK) {
		lang = ` lang="` + attrText(v) + `"`
	}
	if style == "" && lang == "" {
		escape(c.w, s, false)
		return
	}
	if style != "" {
		style = ` style="` + attrText(style) + `"`
	}
	c.w.WriteString(`<span` + lang + style + `>`)
	escape(c.w, s, false)
	c.w.WriteString("</span>")
}

// run converts a w:r element.
func (c *converter) run(r roxmltree.Node, base props) {
	p := c.runProps(r, base)
	join := c.joinBorder
	c.joinBorder = [2]bool{}
	if bdr := p["bdr"]; bdr != "" {
		prev, prevOK := siblingRun(r, false)
		next, nextOK := siblingRun(r, true)
		c.joinBorder = [2]bool{
			prevOK && c.runProps(prev, base)["bdr"] == bdr,
			nextOK && c.runProps(next, base)["bdr"] == bdr,
		}
	}
	defer func() { c.joinBorder = join }()
	c.runContent(r, p, base)
}

// runProps returns the properties of the run r in a paragraph whose runs
// have the properties base: its character style, then its own properties.
func (c *converter) runProps(r roxmltree.Node, base props) props {
	direct := props{}
	if rpr, ok := child(r, "rPr"); ok {
		direct = parseRPr(rpr)
	}
	p := base
	if id := direct["rStyle"]; id != "" {
		if st := c.styles.get(id, "character"); st != nil {
			p = p.over(st.rpr)
		}
	}
	return p.over(direct)
}

// siblingRun returns the run of text next to the run r, after it if next
// is set, else before it, and true. The marks of a bookmark or of a
// proofing error between them do not count. It returns false when no run
// of text is next to r. A run that holds a drawing, a picture or a note is
// not a run of text.
func siblingRun(r roxmltree.Node, next bool) (roxmltree.Node, bool) {
	n := r
	for {
		var ok bool
		if next {
			n, ok = n.NextSiblingElement()
		} else {
			n, ok = n.PrevSiblingElement()
		}
		if !ok {
			return roxmltree.Node{}, false
		}
		switch n.TagName().Name() {
		case "bookmarkStart", "bookmarkEnd", "proofErr", "permStart", "permEnd":
			continue
		case "r":
			for e := range n.Children() {
				if !e.IsElement() {
					continue
				}
				switch e.TagName().Name() {
				case "drawing", "pict", "object", "footnoteReference", "endnoteReference":
					return roxmltree.Node{}, false
				}
			}
			return n, true
		}
		return roxmltree.Node{}, false
	}
}

// runContent converts the content of a run with the properties p.
func (c *converter) runContent(r roxmltree.Node, p, base props) {
	var text strings.Builder
	flush := func() {
		c.text(text.String(), p, base)
		text.Reset()
	}
	for e := range r.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "t":
			s, _ := e.Text()
			text.WriteString(s)
		case "tab", "ptab":
			text.WriteByte('\t')
		case "noBreakHyphen":
			text.WriteRune('\u2011')
		case "softHyphen":
			text.WriteRune('\u00ad')
		case "sym":
			v, err := strconv.ParseUint(ooxml.Attr(e, "char"), 16, 32)
			if err == nil && v < 0x110000 {
				text.WriteRune(symbolRune(ooxml.Attr(e, "font"), rune(v)))
			}
		case "br":
			flush()
			if c.fieldHidden() || p.on("vanish") {
				continue
			}
			switch ooxml.Attr(e, "type") {
			case "page":
				c.pageBreak("page")
			case "column":
				c.pageBreak("column")
			default:
				if v := ooxml.Attr(e, "clear"); v != "" && v != "none" && c.floatSeq > 0 && c.paraSeq-c.floatSeq <= 3 {
					// A break that clears a float just before it puts
					// the mark of its paragraph below the float, and so
					// adds no line for the mark, as in Word. The class
					// tells it from a break that does. Without such a
					// float, the break is a break of a line.
					c.w.WriteString(`<br class="clear"/>`)
					continue
				}
				c.w.WriteString("<br/>")
			}
		case "cr":
			flush()
			if !c.fieldHidden() && !p.on("vanish") {
				c.w.WriteString("<br/>")
			}
		case "fldChar":
			flush()
			c.fieldChar(e, p, base)
		case "instrText":
			if len(c.fields) > 0 {
				s, _ := e.Text()
				f := c.fields[len(c.fields)-1]
				if !f.result && f.instr.Len() < 4096 {
					f.instr.WriteString(s)
				}
			}
		case "drawing":
			flush()
			if !c.fieldHidden() && !p.on("vanish") {
				c.drawing(e)
			}
		case "pict", "object":
			flush()
			if !c.fieldHidden() && !p.on("vanish") {
				c.pict(e)
			}
		case "footnoteReference":
			flush()
			if !c.fieldHidden() {
				c.footnote(ooxml.Attr(e, "id"))
			}
		case "endnoteReference":
			flush()
			if !c.fieldHidden() {
				c.endnote(ooxml.Attr(e, "id"), p, base)
			}
		case "AlternateContent":
			flush()
			if choice, ok := alternative(e); ok && c.enter() {
				c.runContent(choice, p, base)
				c.leave()
			}
		case "footnoteRef":
			flush()
			if !c.fieldHidden() {
				c.w.WriteString(`<span class="fnref"`)
				if style := c.runStyle(p, base); style != "" {
					c.w.WriteString(` style="` + attrText(style) + `"`)
				}
				c.w.WriteString("></span>")
			}
		case "ruby":
			flush()
			if rb, ok := child(e, "rubyBase"); ok {
				c.inlines(rb, p)
			}
		}
	}
	flush()
}

// pageBreak ends the paragraph and starts it again after a break of the
// kind: page or column. The elements that are open close and open again.
func (c *converter) pageBreak(kind string) {
	if c.para == "" || c.nested > 0 {
		c.w.WriteString("<br/>")
		return
	}
	for i := len(c.open) - 1; i >= 0; i-- {
		c.w.WriteString("</" + c.open[i].name + ">")
	}
	c.w.WriteString("</p>")
	c.para = strings.Replace(c.para, `style="`, `style="break-before: `+kind+`; `, 1)
	c.w.WriteString(c.para)
	for _, t := range c.open {
		c.w.WriteString(t.start)
	}
}

// openTag writes the start tag of an inline element and keeps it open.
func (c *converter) openTag(start, name string) {
	c.w.WriteString(start)
	c.open = append(c.open, tag{start: start, name: name})
}

// closeTag closes the inline element that openTag opened last.
func (c *converter) closeTag() {
	if len(c.open) == 0 {
		return
	}
	c.w.WriteString("</" + c.open[len(c.open)-1].name + ">")
	c.open = c.open[:len(c.open)-1]
}

// safeLink reports whether a link of a document can be a link of the
// layout: a URL of the web or of mail.
func safeLink(href string) bool {
	lower := strings.ToLower(strings.TrimSpace(href))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:")
}

// hyperlink converts a w:hyperlink element.
func (c *converter) hyperlink(n roxmltree.Node, base props) {
	href := ""
	if id := ooxml.Attr(n, "id"); id != "" {
		if r, ok := c.relationship(id); ok && r.External && safeLink(r.Target) {
			href = r.Target
		}
	}
	if href == "" {
		c.inlines(n, base)
		return
	}
	c.openTag(`<a href="`+attrText(href)+`">`, "a")
	depth := len(c.open)
	c.inlines(n, base)
	for len(c.open) >= depth {
		c.closeTag()
	}
}

// fieldCode returns the name of the field of an instruction, in upper
// case, and its first argument.
func fieldCode(instr string) (string, string) {
	f := strings.Fields(instr)
	if len(f) == 0 {
		return "", ""
	}
	arg := ""
	for _, a := range f[1:] {
		if !strings.HasPrefix(a, `\`) {
			arg = strings.Trim(a, `"`)
			break
		}
	}
	return strings.ToUpper(f[0]), arg
}

// pageField writes the number of the page or the number of the pages, for
// the field code, and returns false for another field.
func (c *converter) pageField(code string, p, base props) bool {
	class := ""
	switch code {
	case "PAGE":
		class = "pg"
	case "NUMPAGES", "SECTIONPAGES":
		class = "pgs"
	default:
		return false
	}
	c.w.WriteString(`<span class="` + class + `"`)
	if style := c.runStyle(p, base); style != "" {
		c.w.WriteString(` style="` + attrText(style) + `"`)
	}
	c.w.WriteString("></span>")
	return true
}

// simpleField converts a w:fldSimple element: a field of the page, or the
// result of another field.
func (c *converter) simpleField(n roxmltree.Node, base props) {
	code, _ := fieldCode(ooxml.Attr(n, "instr"))
	if !c.fieldHidden() && c.pageField(code, base, base) {
		return
	}
	c.inlines(n, base)
}

// fieldHidden reports whether a field hides the content: the instruction
// of a field, or the result of a field that the conversion writes itself.
func (c *converter) fieldHidden() bool {
	for _, f := range c.fields {
		if !f.result || f.hide {
			return true
		}
	}
	return false
}

// fieldChar converts a w:fldChar element of a run with the properties p, in
// a paragraph whose runs have the properties base.
func (c *converter) fieldChar(e roxmltree.Node, p, base props) {
	switch ooxml.Attr(e, "fldCharType") {
	case "begin":
		if len(c.fields) < 64 {
			f := &field{}
			if ff, ok := child(e, "ffData"); ok {
				f.form = parseFormField(ff)
			}
			c.fields = append(c.fields, f)
		}
	case "separate":
		if len(c.fields) == 0 {
			return
		}
		f := c.fields[len(c.fields)-1]
		if f.result {
			return
		}
		f.result = true
		hidden := c.fieldHidden()
		code, arg := fieldCode(f.instr.String())
		switch {
		case hidden:
		case c.pageField(code, p, p):
			f.hide = true
		case c.formField(code, f.form, p, base):
			f.hide = true
		case code == "HYPERLINK" && safeLink(arg) && c.para != "":
			c.openTag(`<a href="`+attrText(arg)+`">`, "a")
			f.link = c.paraSeq
		}
	case "end":
		if len(c.fields) == 0 {
			return
		}
		f := c.fields[len(c.fields)-1]
		c.fields = c.fields[:len(c.fields)-1]
		if !f.result && !c.fieldHidden() {
			code, _ := fieldCode(f.instr.String())
			if !c.pageField(code, p, p) {
				c.formField(code, f.form, p, base)
			}
		}
		// A link closes in the paragraph that opened it. The end of a
		// paragraph closes it too.
		if f.link != 0 && f.link == c.paraSeq && len(c.open) > 0 && c.open[len(c.open)-1].name == "a" {
			c.closeTag()
		}
	}
}
