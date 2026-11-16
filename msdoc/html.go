package msdoc

import (
	"fmt"
	"html"
	"strconv"
	"strings"
)

// page is the size and the margins of the pages of the first section, in
// twips, 1/20 of a point.
type page struct {
	width, height            int
	top, right, bottom, left int
}

// letter is the page of a section that says nothing: US Letter, with the
// margins of Word 97 (MS-DOC 2.6.4).
var letter = page{width: 12240, height: 15840, top: 1440, right: 1800, bottom: 1440, left: 1800}

// firstPage reads the size of the pages of the first section, from the
// PlcfSed and its Sepx (MS-DOC 2.8.26 and 2.9.253).
func (p *parser) firstPage() page {
	pg := letter
	plc := p.s.part(fcPlcfSed)
	n := (len(plc) - 4) / 16
	if n < 1 {
		return pg
	}
	sed := plc[4*(n+1):]
	fc := u32(sed, 2)
	word := p.s.word
	if fc == 0xFFFFFFFF || uint64(fc)+2 > uint64(len(word)) {
		return pg
	}
	cb := int(u16(word, int(fc)))
	start := int(fc) + 2
	grpprl := word[start:min(start+cb, len(word))]
	for s := range sprms(grpprl) {
		v := int(uint16(s.value()))
		switch s.op {
		case sprmSXaPage:
			pg.width = v
		case sprmSYaPage:
			pg.height = v
		case sprmSDxaLeft:
			pg.left = v
		case sprmSDxaRight:
			pg.right = v
		case sprmSDyaTop:
			pg.top = int(int16(v))
		case sprmSDyaBottom:
			pg.bottom = int(int16(v))
		}
	}
	// a page must hold some text: a value out of range takes the default
	if pg.width < 1440 || pg.width > 31680 || pg.height < 1440 || pg.height > 31680 {
		pg.width, pg.height = letter.width, letter.height
	}
	clamp := func(v, limit int) int {
		return max(0, min(abs(v), limit))
	}
	pg.left, pg.right = clamp(pg.left, pg.width/3), clamp(pg.right, pg.width/3)
	pg.top, pg.bottom = clamp(pg.top, pg.height/3), clamp(pg.bottom, pg.height/3)
	return pg
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
  size: %s %s;
  margin: %s %s %s %s;
}
html {
  font-family: %s;
  font-size: 10pt;
}
p, h1, h2, h3, h4, h5, h6 {
  margin: 0;
  min-height: 1em;
  white-space: pre-wrap;
}
table {
  border-collapse: collapse;
  margin: 2pt 0;
}
td {
  border: 0.5pt solid black;
  padding: 1pt 4pt;
  vertical-align: top;
}
.page {
  break-before: page;
}
`, pt(pg.width), pt(pg.height), pt(pg.top), pt(pg.right), pt(pg.bottom), pt(pg.left), family)
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
// table becomes the paragraphs of its cell.
func (p *parser) body(paras []paragraph) string {
	var b strings.Builder
	var (
		inTable bool
		cells   []string
		cell    strings.Builder
	)
	writeRow := func(widths []int) {
		if len(cells) == 0 {
			return
		}
		b.WriteString("<tr>")
		for i, c := range cells {
			if i < len(widths) && widths[i] > 0 && widths[i] < 31680 {
				b.WriteString(`<td style="width: ` + pt(widths[i]) + `">`)
			} else {
				b.WriteString("<td>")
			}
			b.WriteString(c)
			b.WriteString("</td>")
		}
		b.WriteString("</tr>\n")
		cells = cells[:0]
	}
	closeTable := func() {
		if cell.Len() > 0 {
			cells = append(cells, cell.String())
			cell.Reset()
		}
		writeRow(nil)
		if inTable {
			b.WriteString("</table>\n")
			inTable = false
		}
	}
	for _, para := range paras {
		if !para.pap.inTable {
			closeTable()
			p.writeParagraph(&b, para)
			continue
		}
		if !inTable {
			b.WriteString("<table>\n")
			inTable = true
		}
		if para.pap.ttp && para.end == 0x07 {
			// the mark of the end of a row, with the widths of its cells
			cell.Reset()
			writeRow(para.pap.widths)
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
	b.WriteString("<" + tag)
	if para.pageBreak || para.pap.pageBreak {
		b.WriteString(` class="page"`)
	}
	if len(style) > 0 {
		b.WriteString(` style="` + strings.Join(style, "; ") + `"`)
	}
	b.WriteString(">")
	for _, r := range para.runs {
		p.writeRun(b, r)
	}
	b.WriteString("</" + tag + ">\n")
}

// writeRun writes a run of text in a span with its character properties.
func (p *parser) writeRun(b *strings.Builder, r run) {
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
	text = strings.ReplaceAll(text, "\n", "<br>")
	if len(style) == 0 {
		b.WriteString(text)
		return
	}
	b.WriteString(`<span style="` + html.EscapeString(strings.Join(style, "; ")) + `">`)
	b.WriteString(text)
	b.WriteString("</span>")
}
