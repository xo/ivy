package docx

import (
	"math"
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
	for e := range n.Children() {
		if e.IsElement() {
			c.block(e, tc)
		}
	}
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

// empty reports whether the paragraph p shows nothing but its mark.
func empty(p roxmltree.Node) bool {
	for d := range p.Descendants() {
		if d.IsElement() && contentNames[d.TagName().Name()] {
			return false
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
		if isSymbolFont(c.font(mr["font"])) {
			// The marker holds Unicode characters, which the font of the
			// text draws.
			mr["font"] = rpr["font"]
		}
		c.marker(marker, lvl, ppr, mr, rpr)
	}
	c.inlines(p, rpr)
	for i := len(c.open) - 1; i >= 0; i-- {
		c.w.WriteString("</" + c.open[i].name + ">")
	}
	if c.w.Len() == mark0 {
		// An empty paragraph takes the height of one line, as in Word.
		c.w.WriteString("<br/>")
	}
	c.w.WriteString("</p>")
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
	// Word adds the spacing after a paragraph and the spacing before the
	// next one. The spacing before is a padding, which does not collapse
	// with the margin of the spacing after. The spacing after is a margin,
	// which the bottom of a page cuts, as in Word.
	if before > 0 {
		s.set(pick(boxed || autoBefore, "margin-top", "padding-top"), pt(before))
	}
	if after > 0 {
		s.set("margin-bottom", pt(after))
	}
	if v, ok := hexColor(ppr["shd"]); ok {
		s.set("background-color", v)
	}
	s.set("line-height", c.lineHeight(ppr, rpr))
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
	return s.String()
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
	if pitch := c.linePitch(); pitch > 0 && c.margin == "" && ppr["snapToGrid"] != "0" && ppr["snapToGrid"] != "false" && ppr["snapToGrid"] != "off" {
		n := math.Ceil(ratio*fontSize(rpr)/pitch - 0.01)
		return pt(n * pitch * min(line/240, 10))
	}
	return num(min(line/240, 10) * ratio)
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
			s.set("min-width", pt(hanging))
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

// math writes the text of an equation of Office Math, in italics.
func (c *converter) math(n roxmltree.Node, base props) {
	var text strings.Builder
	for d := range n.Descendants() {
		if d.IsElement() && d.TagName().Name() == "t" {
			s, _ := d.Text()
			text.WriteString(s)
		}
	}
	c.text(text.String(), base.over(props{"i": "1"}), base)
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
	p = p.over(direct)
	c.runContent(r, p, base)
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
