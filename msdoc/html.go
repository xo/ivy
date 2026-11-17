package msdoc

import (
	"fmt"
	"html"
	"iter"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/xo/ivy"
)

// page is the size and the margins of the pages of the first section, in
// twips, 1/20 of a point.
type page struct {
	width, height            int
	top, right, bottom, left int
	// paper is the paper of the program, in points, when the section gives
	// no size, or zero. width and height hold it rounded to twips (D32).
	paper ivy.Paper
}

// letter is the page of a section that says nothing: US Letter, with the
// margins of Word 97 (MS-DOC 2.6.4).
var letter = page{width: 12240, height: 15840, top: 1440, right: 1800, bottom: 1440, left: 1800}

// defaultPage returns the page of a section that gives no size: the paper
// of the program, or the default xaPage and yaPage of the Sep of MS-DOC
// 2.6.4, US Letter in portrait. The margins of the Sep stay (D32).
func defaultPage(paper ivy.Paper) page {
	pg := letter
	if paper.Width > 0 && paper.Height > 0 {
		pg.width, pg.height = paperTwips(paper.Width), paperTwips(paper.Height)
		pg.paper = paper
	}
	return pg
}

// paperTwips returns a side of a paper in points as twips, within the
// bounds of a page.
func paperTwips(v float32) int {
	return int(math.Round(min(max(float64(v)*20, 1440), 31680)))
}

// size returns the size of the page for CSS: the paper of the program in
// exact points, which twips round, or the size in twips.
func (pg page) size() string {
	if pg.paper.Width > 0 && pg.paper.Height > 0 {
		side := func(v float32) string {
			return strconv.FormatFloat(math.Round(min(max(float64(v), 72), 1584)*100)/100, 'f', -1, 64) + "pt"
		}
		return side(pg.paper.Width) + " " + side(pg.paper.Height)
	}
	return pt(pg.width) + " " + pt(pg.height)
}

// section is a section of the document: the character where it starts,
// and its kind of break (MS-DOC 2.9.23).
type section struct {
	cp  uint32
	bkc int
}

// The kinds of the break of a section.
const (
	bkcContinuous = 0
	bkcNewPage    = 2
)

// sectionList is the sections of the document, in order.
type sectionList []section

// continuous reports whether a section that is continuous starts at the
// character cp.
func (sl sectionList) continuous(cp uint32) bool {
	i := sort.Search(len(sl), func(i int) bool { return sl[i].cp >= cp })
	return i < len(sl) && sl[i].cp == cp && sl[i].bkc == bkcContinuous
}

// sepx returns the modifiers of the section i of the PlcfSed plc of n
// sections, or nil (MS-DOC 2.8.26 and 2.9.253).
func (p *parser) sepx(plc []byte, n, i int) []byte {
	sed := plc[4*(n+1)+12*i:]
	fc := u32(sed, 2)
	word := p.s.word
	if fc == 0xFFFFFFFF || uint64(fc)+2 > uint64(len(word)) {
		return nil
	}
	cb := int(u16(word, int(fc)))
	start := int(fc) + 2
	return word[start:min(start+cb, len(word))]
}

// sedPlc returns the PlcfSed and its number of sections.
func (p *parser) sedPlc() ([]byte, int) {
	plc := p.s.part(fcPlcfSed)
	n := (len(plc) - 4) / 16
	if n < 1 {
		return nil, 0
	}
	return plc, min(n, p.lim.MaxObjects)
}

// sections reads the start and the kind of break of each section.
func (p *parser) sections() sectionList {
	plc, n := p.sedPlc()
	sl := make(sectionList, 0, n)
	for i := range n {
		sec := section{cp: u32(plc, 4*i), bkc: bkcNewPage}
		for s := range p.sectionSprms(p.sepx(plc, n, i)) {
			if s.op == sprmSBkc {
				sec.bkc = int(s.byteValue())
			}
		}
		sl = append(sl, sec)
	}
	sort.SliceStable(sl, func(i, j int) bool { return sl[i].cp < sl[j].cp })
	return sl
}

// firstPage reads the size of the pages of the first section. A section
// that gives no size takes paper, the paper of the program (D32).
func (p *parser) firstPage(paper ivy.Paper) page {
	return p.sectionPage(0, paper).page
}

// sectionPage reads the page of the section i, from the PlcfSed and its
// Sepx (MS-DOC 2.8.26 and 2.9.253): the size of its pages, its margins, the
// distances of its headers and its footers, and whether its first page has
// a header of its own. A section that gives no size takes paper, the paper
// of the program (D32).
func (p *parser) sectionPage(i int, paper ivy.Paper) sectionPage {
	def := defaultPage(paper)
	sp := sectionPage{page: def, header: 720, footer: 720}
	pg := &sp.page
	plc, n := p.sedPlc()
	if i >= n {
		return sp
	}
	for s := range p.sectionSprms(p.sepx(plc, n, i)) {
		v := int(uint16(s.value()))
		switch s.op {
		case sprmSXaPage:
			pg.width, pg.paper = v, ivy.Paper{}
		case sprmSYaPage:
			pg.height, pg.paper = v, ivy.Paper{}
		case sprmSDxaLeft:
			pg.left = v
		case sprmSDxaRight:
			pg.right = v
		case sprmSDyaTop:
			pg.top = int(int16(v))
		case sprmSDyaBottom:
			pg.bottom = int(int16(v))
		case sprmSDyaHdrTop:
			sp.header = v
		case sprmSDyaHdrBottom:
			sp.footer = v
		case sprmSFTitlePage:
			sp.titlePage = s.byteValue() != 0
		}
	}
	// a page must hold some text: a value out of range takes the default
	if pg.width < 1440 || pg.width > 31680 || pg.height < 1440 || pg.height > 31680 {
		pg.width, pg.height, pg.paper = def.width, def.height, def.paper
	}
	clamp := func(v, limit int) int {
		return max(0, min(abs(v), limit))
	}
	pg.left, pg.right = clamp(pg.left, pg.width/3), clamp(pg.right, pg.width/3)
	pg.top, pg.bottom = clamp(pg.top, pg.height/3), clamp(pg.bottom, pg.height/3)
	sp.header, sp.footer = clamp(sp.header, pg.height/3), clamp(sp.footer, pg.height/3)
	return sp
}

// sectionPage is the page of a section, with the distances of its header
// from the top of the paper and of its footer from the bottom, in twips.
type sectionPage struct {
	page
	header, footer int
	titlePage      bool
}

// sectionSprms iterates over the modifiers of a section. A document of
// Word 6 or Word 95 has modifiers of one byte, which the iteration gives
// with the codes of Word 97.
func (p *parser) sectionSprms(grpprl []byte) iter.Seq[sprm] {
	if !p.s.word6 {
		return sprms(grpprl)
	}
	return word6SectionSprms(grpprl)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// pt returns a length of twips in points, for CSS.
func pt(twips int) string {
	return strconv.FormatFloat(float64(twips)/20, 'f', -1, 64) + "pt"
}

// css returns the style sheet of the document: the page of the first
// section and the default font.
func (p *parser) css(pg page) string {
	family := "serif"
	if st := p.styles.resolve(0, p.lim.MaxDepth); st != nil && st.stk == stkParagraph {
		if f := p.fontFamily(st.chp.ftc); f != "" {
			family = f
		}
	}
	return fmt.Sprintf(`@page {
  size: %s;
  margin: %s %s %s %s;
}
html {
  font-family: %s;
  font-size: 10pt;
}
p, h1, h2, h3, h4, h5, h6 {
  margin: 0;
  font-size: 1em;
  font-weight: normal;
  white-space: pre-wrap;
}
table {
  border-collapse: collapse;
}
td {
  padding: 0;
  vertical-align: top;
}
.page {
  break-before: page;
}
.pg::before {
  content: counter(page);
}
.pgs::before {
  content: counter(pages);
}
`, pg.size(), pt(pg.top), pt(pg.right), pt(pg.bottom), pt(pg.left), family)
}

// fontFamily returns the font family of CSS for the font ftc, or "".
func (p *parser) fontFamily(ftc int) string {
	if ftc < 0 || ftc >= len(p.fonts) || p.fonts[ftc].name == "" {
		return ""
	}
	return cssFont(p.fonts[ftc].name) + ", " + p.fonts[ftc].generic
}

// cssFont quotes the name of a font for CSS.
func cssFont(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r < 0x20 {
			return -1
		}
		return r
	}, name)
	return `"` + name + `"`
}

// body converts the paragraphs into the body of an HTML document. The
// paragraphs of a table become the cells of an HTML table. A table inside a
// table becomes the paragraphs of its cell. section, when it is not nil, is
// called before each paragraph, and can end the table and write the start
// of a section.
func (p *parser) body(paras []paragraph, section func(b *strings.Builder, para paragraph, closeTable func())) string {
	var b strings.Builder
	var (
		rows  []tableRow
		cells []string
		cell  strings.Builder
	)
	closeTable := func() {
		if cell.Len() > 0 {
			cells = append(cells, cell.String())
			cell.Reset()
		}
		if len(cells) > 0 {
			rows = append(rows, tableRow{cells: cells})
			cells = nil
		}
		writeTable(&b, rows)
		rows = nil
	}
	for _, para := range paras {
		// a section starts between the rows of a table at the earliest
		if section != nil && len(cells) == 0 && cell.Len() == 0 {
			section(&b, para, closeTable)
		}
		if !para.pap.inTable {
			closeTable()
			p.writeParagraph(&b, para)
			continue
		}
		if para.pap.ttp && para.end == 0x07 {
			// the mark of the end of a row, with the properties of the row
			cell.Reset()
			rows = append(rows, tableRow{cells: cells, tap: para.pap.tap})
			cells = nil
			continue
		}
		if para.pap.innerTtp {
			continue
		}
		p.writeParagraph(&cell, para)
		if para.end == 0x07 && para.pap.itap <= 1 {
			cells = append(cells, cell.String())
			cell.Reset()
		}
	}
	closeTable()
	return b.String()
}

// writeParagraph writes one paragraph: its tag, its style and its runs.
func (p *parser) writeParagraph(b *strings.Builder, para paragraph) {
	tag := "p"
	if level := p.styles.heading(para.pap.istd); level > 0 {
		tag = "h" + strconv.Itoa(min(level, 6))
	}
	var style []string
	switch para.pap.jc {
	case 1:
		style = append(style, "text-align: center")
	case 2:
		style = append(style, "text-align: right")
	case 3, 4, 5:
		style = append(style, "text-align: justify")
	}
	if para.pap.left > 0 && para.pap.left < 14400 {
		style = append(style, "margin-left: "+pt(para.pap.left))
	}
	if para.pap.right > 0 && para.pap.right < 14400 {
		style = append(style, "margin-right: "+pt(para.pap.right))
	}
	if para.pap.first != 0 && abs(para.pap.first) < 14400 {
		style = append(style, "text-indent: "+pt(para.pap.first))
	}
	if para.pap.before > 0 && para.pap.before < 14400 {
		style = append(style, "margin-top: "+pt(para.pap.before))
	}
	if para.pap.after > 0 && para.pap.after < 14400 {
		style = append(style, "margin-bottom: "+pt(para.pap.after))
	}
	switch {
	case para.pap.lineMult > 0:
		// a single line of Word is about 1.15 times the size of the font
		style = append(style, "line-height: "+strconv.FormatFloat(1.15*para.pap.lineMult, 'f', 3, 64))
	case para.pap.lineExact > 0:
		style = append(style, "line-height: "+pt(para.pap.lineExact))
	}
	// the paragraph has the font of its mark, so that the strut of its
	// lines is not taller than the lines of Word
	if hps := para.mark.hps; hps > 0 {
		style = append(style, "font-size: "+strconv.FormatFloat(float64(hps)/2, 'f', -1, 64)+"pt")
	}
	if f := p.fontFamily(para.mark.ftc); f != "" {
		style = append(style, "font-family: "+f)
	}
	b.WriteString("<" + tag)
	if para.pageBreak || para.pap.pageBreak {
		b.WriteString(` class="page"`)
	}
	if len(style) > 0 {
		b.WriteString(` style="` + html.EscapeString(strings.Join(style, "; ")) + `"`)
	}
	b.WriteString(">")
	if text, c, ok := p.marker(para.pap.ilfo, para.pap.ilvl, para.mark); ok {
		p.writeMarker(b, text, c, para.pap.first)
	}
	for _, r := range para.runs {
		p.writeRun(b, r)
	}
	if len(para.runs) == 0 {
		// an empty paragraph has the line of the font of its mark
		b.WriteString("<br>")
	}
	b.WriteString("</" + tag + ">\n")
}

// writeMarker writes the number or the bullet of a paragraph of a list,
// with the properties c. A hanging indent of first, negative, gives it the
// width of the indent, so that the text after it starts at the indent, as
// at the tab stop of Word.
func (p *parser) writeMarker(b *strings.Builder, text string, c chp, first int) {
	if first < 0 && strings.HasSuffix(text, "\t") {
		b.WriteString(`<span style="display: inline-block; text-indent: 0; min-width: ` + pt(-first) + `">`)
		p.writeRun(b, run{text: strings.TrimSuffix(text, "\t") + " ", chp: c})
		b.WriteString("</span>")
		return
	}
	p.writeRun(b, run{text: strings.ReplaceAll(text, "\t", " "), chp: c})
}

// writeRun writes a run of text in a span with its character properties,
// or the image of a picture.
func (p *parser) writeRun(b *strings.Builder, r run) {
	if r.float != nil {
		p.writeFloating(b, r.float)
		return
	}
	if r.pic != nil {
		b.WriteString(`<img src="` + html.EscapeString(r.pic.name) + `" alt=""`)
		if r.pic.width > 0 {
			fmt.Fprintf(b, ` style="width: %.2fpt; height: %.2fpt"`, r.pic.width, r.pic.height)
		}
		b.WriteString("/>")
		return
	}
	var style []string
	c := r.chp
	if c.bold {
		style = append(style, "font-weight: bold")
	}
	if c.italic {
		style = append(style, "font-style: italic")
	}
	var deco []string
	if c.underline {
		deco = append(deco, "underline")
	}
	if c.strike {
		deco = append(deco, "line-through")
	}
	if len(deco) > 0 {
		style = append(style, "text-decoration: "+strings.Join(deco, " "))
	}
	if c.caps {
		style = append(style, "text-transform: uppercase")
	} else if c.smallCaps {
		style = append(style, "font-variant: small-caps")
	}
	if c.hps > 0 {
		size := strconv.FormatFloat(float64(c.hps)/2, 'f', -1, 64) + "pt"
		if c.iss == 1 || c.iss == 2 {
			size = strconv.FormatFloat(float64(c.hps)/3, 'f', 1, 64) + "pt"
		}
		style = append(style, "font-size: "+size)
	}
	switch c.iss {
	case 1:
		style = append(style, "vertical-align: super")
	case 2:
		style = append(style, "vertical-align: sub")
	}
	if c.hasColor {
		style = append(style, fmt.Sprintf("color: #%06x", c.color))
	}
	if c.hasHighlight {
		style = append(style, fmt.Sprintf("background-color: #%06x", c.highlight))
	}
	if f := p.fontFamily(c.ftc); f != "" {
		style = append(style, "font-family: "+f)
	}
	text := html.EscapeString(r.text)
	text = strings.NewReplacer("\n", "<br>", string(runePage), `<span class="pg"></span>`,
		string(runePages), `<span class="pgs"></span>`).Replace(text)
	if len(style) == 0 {
		b.WriteString(text)
		return
	}
	b.WriteString(`<span style="` + html.EscapeString(strings.Join(style, "; ")) + `">`)
	b.WriteString(text)
	b.WriteString("</span>")
}

// writeFloating writes a picture that floats on the page. A picture that
// text wraps around floats to the side of the column where it is, a
// picture above and below text is a block, and a picture that text does not
// wrap is placed on the page, as the package docx does.
func (p *parser) writeFloating(b *strings.Builder, f *floating) {
	pg := p.page
	if pg.width == 0 {
		pg = letter
	}
	col := float64(pg.width-pg.left-pg.right) / 20
	w, h := f.pic.width, f.pic.height
	x := float64(f.left) / 20
	if f.bx == 1 {
		x -= float64(pg.left) / 20
	}
	y := float64(f.top) / 20
	if f.by == 1 {
		y -= float64(pg.top) / 20
	}
	var style []string
	switch f.wr {
	case 1:
		style = append(style, "display: block", "margin-left: "+num(max(x, 0))+"pt")
		if f.by == 2 && y > 0 {
			style = append(style, "margin-top: "+num(y)+"pt")
		}
	case 3:
		style = append(style, "position: absolute", "left: "+num(x)+"pt")
		if f.by == 2 {
			style = append(style, "margin-top: "+num(y)+"pt")
		} else {
			style = append(style, "top: "+num(y)+"pt")
		}
		if f.behind {
			style = append(style, "z-index: -1")
		}
	default:
		if x+w/2 > col/2 {
			style = append(style, "float: right", "margin-right: "+num(max(col-x-w, 0))+"pt", "margin-left: 9pt")
		} else {
			style = append(style, "float: left", "margin-left: "+num(max(x, 0))+"pt", "margin-right: 9pt")
		}
		if f.by == 2 && y > 0 {
			style = append(style, "margin-top: "+num(y)+"pt")
		}
	}
	style = append(style, "width: "+num(w)+"pt", "height: "+num(h)+"pt")
	b.WriteString(`<img src="` + html.EscapeString(f.pic.name) + `" alt="" style="` + strings.Join(style, "; ") + `"/>`)
}
