package docx

import (
	"bytes"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// The tests of this file come from the faults that the visual review found
// in the files of Word.

// letterSection is a section of a page of US Letter with margins of 1 inch,
// a header at half an inch from the top, and the default header rIdW.
const letterSection = `<w:sectPr><w:headerReference w:type="default" r:id="rIdW"/><w:pgSz w:w="12240" w:h="15840"/>` +
	`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`

// TestHeaderWatermark checks that a picture of VML in a header, which Word
// places in the middle of the page and washes out, draws there on each
// page, behind the text.
func TestHeaderWatermark(t *testing.T) {
	header := fixture.WordXML("w:hdr", `<w:p><w:r><w:pict><v:shape style="position:absolute;margin-left:0;margin-top:0;width:200pt;height:100pt;`+
		`z-index:-251657216;mso-position-horizontal:center;mso-position-horizontal-relative:margin;mso-position-vertical:center;mso-position-vertical-relative:margin">`+
		`<v:imagedata r:id="rId1" gain="19661f" blacklevel="22938f"/></v:shape></w:pict></w:r></w:p>`)
	body := `<w:p><w:r><w:t>Body text</w:t></w:r></w:p>` + letterSection
	buf := fixture.DOCX(body, fixture.WordPart{ID: "rIdW", Type: "header", Name: "word/header1.xml", Data: header})
	buf = fixture.Rezip(buf,
		fixture.Entry{Name: "word/_rels/header1.xml.rels", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/w.png"/></Relationships>`)},
		fixture.Entry{Name: "word/media/w.png", Data: fixture.PNG(4, 2, color.NRGBA{R: 255, A: 255})},
	)
	html, _ := convertHTML(t, buf)
	for _, want := range []string{`z-index: -1`, `opacity: 0.300`, `position: absolute`} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the xhtml:\n%s", want, html)
		}
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	img := render(t, d, 0)
	// the middle of the page area of 468 by 648 points at (72, 72) is at
	// (306, 396): a light red
	got := color.NRGBAModel.Convert(img.At(306, 396)).(color.NRGBA)
	if got.R < 240 || got.G < 150 || got.G > 215 {
		t.Errorf("expected the washed out red of the watermark at (306, 396), got %v", got)
	}
	if got := color.NRGBAModel.Convert(img.At(306, 200)).(color.NRGBA); got.G < 250 {
		t.Errorf("expected white above the watermark, got %v", got)
	}
}

// TestAutospacing checks that the automatic spacing of HTML collapses, as
// the margins of HTML do.
func TestAutospacing(t *testing.T) {
	p := `<w:p><w:pPr><w:spacing w:before="100" w:beforeAutospacing="1" w:after="100" w:afterAutospacing="1"/></w:pPr><w:r><w:t>Web</w:t></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(p+p))
	if !strings.Contains(html, `margin-top: 14pt`) || strings.Contains(html, `padding-top: 14pt`) {
		t.Errorf("expected the spacing before as a margin that collapses:\n%s", html)
	}
}

// TestSectionEndBreak checks that an empty paragraph that ends a section
// does not break the page before it, so that the next section starts the
// next page, and no page is blank.
func TestSectionEndBreak(t *testing.T) {
	sect := `<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>`
	body := `<w:p><w:r><w:t>First</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:pageBreakBefore/>` + sect + `</w:pPr></w:p>` +
		`<w:p><w:r><w:t>Second</w:t></w:r></w:p>` + sect
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body)))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 pages, got %d", n)
	}
}

// TestLinePitch checks that the lines of a paragraph whose runs have
// another font than the paragraph have the height of a single line of the
// font of the runs: 1.1328 times 10 points for Courier New, which
// Liberation Mono replaces. A strut of the font of the paragraph makes
// them taller.
func TestLinePitch(t *testing.T) {
	text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 12)
	body := `<w:p><w:pPr><w:spacing w:before="0" w:after="0"/></w:pPr><w:r><w:rPr><w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/>` +
		`<w:sz w:val="20"/></w:rPr><w:t>` + text + `</w:t></w:r></w:p>` +
		`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>`
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body)), WithConfig(ivy.Config{Fonts: officeFonts(t)}))
	if err != nil {
		t.Fatal(err)
	}
	img := render(t, d, 0)
	// the tops of the lines of ink, at 72 dots per inch
	var tops []int
	inked := false
	for y := 60; y < 300; y++ {
		row := false
		for x := 72; x < 540; x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r>>8 < 128 {
				row = true
				break
			}
		}
		if row && !inked {
			tops = append(tops, y)
		}
		inked = row
	}
	if len(tops) < 4 {
		t.Fatalf("expected 4 lines or more, got %d", len(tops))
	}
	pitch := float64(tops[len(tops)-1]-tops[0]) / float64(len(tops)-1)
	if pitch < 10.8 || pitch > 11.9 {
		t.Errorf("expected lines of 11.3 points, got %.2f", pitch)
	}
}

// officeFonts returns the folder of the module github.com/xo/ivy/fonts, or
// skips the test when it is missing.
func officeFonts(t *testing.T) fs.FS {
	t.Helper()
	dir := filepath.Join("..", "fonts")
	if _, err := os.Stat(filepath.Join(dir, "liberation")); err != nil {
		t.Skip("the fonts of github.com/xo/ivy/fonts are missing:", err)
	}
	return os.DirFS(dir)
}

// TestSectionEndAtPageBottom checks that an empty paragraph that ends a
// section, at the bottom of a full page, makes no blank page before the
// next section, which starts a new page (aspose/docx/rendering.docx).
func TestSectionEndAtPageBottom(t *testing.T) {
	sect := `<w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/>`
	line := `<w:p><w:pPr><w:spacing w:before="0" w:after="0" w:line="280" w:lineRule="exact"/></w:pPr><w:r><w:t>Line</w:t></w:r></w:p>`
	// 46 lines of 14 points take 644 of the 648 points of the page area
	body := strings.Repeat(line, 46) +
		`<w:p><w:pPr><w:keepNext/><w:spacing w:before="0" w:after="0" w:line="280" w:lineRule="exact"/>` +
		`<w:sectPr><w:type w:val="continuous"/>` + sect + `</w:sectPr></w:pPr></w:p>` +
		`<w:p><w:r><w:t>Next section</w:t></w:r></w:p><w:sectPr>` + sect + `</w:sectPr>`
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body)))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 pages, got %d", n)
	}
}

// TestContinuousSectionPage checks that a continuous section names the
// page of the section before, so that a page that it starts keeps the size
// and the headers of that page.
func TestContinuousSectionPage(t *testing.T) {
	body := `<w:p><w:pPr><w:sectPr>` + `<w:pgSz w:w="8391" w:h="11907"/>` + `</w:sectPr></w:pPr></w:p>` +
		`<w:p><w:r><w:t>Text</w:t></w:r></w:p><w:sectPr><w:type w:val="continuous"/><w:pgSz w:w="8391" w:h="11907"/></w:sectPr>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	if got := strings.Count(html, "page: s1"); got != 2 {
		t.Errorf("expected both sections on the page s1, got %d:\n%s", got, html)
	}
}

// TestFormCheckBox checks that the legacy form fields of Word show a check
// box, checked or not, and the entry of a drop-down list
// (poi/docx/form-footnotes.docx).
func TestFormCheckBox(t *testing.T) {
	field := func(ffData, code string) string {
		return `<w:r><w:fldChar w:fldCharType="begin"><w:ffData>` + ffData + `</w:ffData></w:fldChar></w:r>` +
			`<w:r><w:instrText xml:space="preserve"> ` + code + ` </w:instrText></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>`
	}
	body := `<w:p>` +
		field(`<w:checkBox><w:sizeAuto/><w:default w:val="0"/></w:checkBox>`, "FORMCHECKBOX") +
		`<w:r><w:t>No</w:t></w:r></w:p><w:p>` +
		field(`<w:checkBox><w:sizeAuto/><w:default w:val="0"/><w:checked/></w:checkBox>`, "FORMCHECKBOX") +
		`<w:r><w:t>Yes</w:t></w:r></w:p><w:p>` +
		field(`<w:ddList><w:result w:val="1"/><w:listEntry w:val="Red"/><w:listEntry w:val="Green"/></w:ddList>`, "FORMDROPDOWN") +
		`</w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	if got := strings.Count(html, "<rect "); got != 2 {
		t.Errorf("expected 2 check boxes, got %d:\n%s", got, html)
	}
	if got := strings.Count(html, "<path "); got != 1 {
		t.Errorf("expected 1 checked box, got %d:\n%s", got, html)
	}
	if !strings.Contains(html, "Green") || strings.Contains(html, "Red") {
		t.Errorf("expected the entry Green of the list:\n%s", html)
	}
}

// TestHeaderHeight checks that a header taller than the top margin pushes
// the text of the page down, as in Word, and that a short footer leaves the
// bottom margin as it is (libreoffice/docx/mixed-orientation.docx).
func TestHeaderHeight(t *testing.T) {
	text := strings.Repeat("The header of the page has a long text. ", 30)
	header := fixture.WordXML("w:hdr", `<w:p><w:r><w:t>`+text+`</w:t></w:r></w:p>`)
	footer := fixture.WordXML("w:ftr", `<w:p><w:r><w:t>Footer</w:t></w:r></w:p>`)
	body := `<w:p><w:r><w:t>Body</w:t></w:r></w:p><w:sectPr><w:headerReference w:type="default" r:id="rIdH"/>` +
		`<w:footerReference w:type="default" r:id="rIdF"/><w:pgSz w:w="12240" w:h="15840"/>` +
		`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720"/></w:sectPr>`
	_, sheet := convertHTML(t, fixture.DOCX(body,
		fixture.WordPart{ID: "rIdH", Type: "header", Name: "word/header1.xml", Data: header},
		fixture.WordPart{ID: "rIdF", Type: "footer", Name: "word/footer1.xml", Data: footer}))
	// The text of about 1200 characters of 10 points takes 13 lines of
	// 11.7 points on the 468 points of the column.
	m := regexp.MustCompile(`margin-top: ([0-9.]+)pt; margin-bottom: ([0-9.]+)pt`).FindStringSubmatch(sheet)
	if m == nil {
		t.Fatalf("expected the margins in the style sheet:\n%s", sheet)
	}
	top, _ := strconv.ParseFloat(m[1], 64)
	bottom, _ := strconv.ParseFloat(m[2], 64)
	if top < 150 || top > 220 {
		t.Errorf("expected a top margin of about 190 points, got %g", top)
	}
	if bottom != 72 {
		t.Errorf("expected a bottom margin of 72 points, got %g", bottom)
	}
}

// TestDocGridLines checks that the lines of a section with a grid of lines
// take a whole number of lines of the grid.
func TestDocGridLines(t *testing.T) {
	body := `<w:p><w:r><w:rPr><w:sz w:val="22"/></w:rPr><w:t>Text</w:t></w:r></w:p>` +
		`<w:sectPr><w:docGrid w:type="lines" w:linePitch="326"/></w:sectPr>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	if !strings.Contains(html, "line-height: 16.3pt") {
		t.Errorf("expected lines of 16.3 points:\n%s", html)
	}
}

// TestVMLShapeBox checks that a VML ellipse of the body whose position is
// absolute draws with its fill, its line and its text.
func TestVMLShapeBox(t *testing.T) {
	body := `<w:p><w:r><w:pict><v:oval style="position:absolute;margin-left:20pt;margin-top:10pt;width:140pt;height:92pt;v-text-anchor:middle" fillcolor="aqua">` +
		`<v:textbox inset="0,0,0,0"><w:txbxContent><w:p><w:r><w:t>Shape: Ellipse</w:t></w:r></w:p></w:txbxContent></v:textbox>` +
		`<w10:wrap xmlns:w10="urn:schemas-microsoft-com:office:word" type="tight"/></v:oval></w:pict></w:r><w:r><w:t>Body</w:t></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	for _, want := range []string{"float: left", "border-radius: 50%", "background-color: aqua", "width: 140pt", "justify-content: center", "Shape: Ellipse"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the xhtml:\n%s", want, html)
		}
	}
}

// TestStyleWithoutID checks that a paragraph can name a style that has no
// id by its name without the spaces, as in Word
// (poi/docx/landscape-letter.docx).
func TestStyleWithoutID(t *testing.T) {
	styles := fixture.WordXML("w:styles", `<w:style><w:name w:val="Empty Cell Style"/><w:rPr><w:sz w:val="2"/></w:rPr></w:style>`)
	body := `<w:p><w:pPr><w:pStyle w:val="EmptyCellStyle"/></w:pPr><w:r><w:t>Small</w:t></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body, fixture.WordPart{ID: "rIdS", Type: "styles", Name: "word/styles.xml", Data: styles}))
	if !strings.Contains(html, "font-size: 1pt") {
		t.Errorf("expected the size of the style:\n%s", html)
	}
}

// TestMissingFontLine checks that a font that the program does not have
// takes the height of a line of the font that draws it instead, Liberation
// Serif, as Word does with Times New Roman (poi/docx/illustrative-cases.docx).
func TestMissingFontLine(t *testing.T) {
	body := `<w:p><w:r><w:rPr><w:rFonts w:ascii="Perpetua No Such Font" w:hAnsi="Perpetua No Such Font"/></w:rPr><w:t>Text</w:t></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	if !strings.Contains(html, "line-height: 1.15") {
		t.Errorf("expected the line of Liberation Serif:\n%s", html)
	}
}

// TestPageBorders checks that the borders of the pages of a section draw
// at their space from the edge of the paper (aspose/docx/rendering.docx).
func TestPageBorders(t *testing.T) {
	body := `<w:p><w:r><w:t>Text</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="8391" w:h="11907"/>` +
		`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/><w:pgBorders w:offsetFrom="page">` +
		`<w:top w:val="single" w:sz="24" w:space="24" w:color="92D050"/><w:left w:val="single" w:sz="24" w:space="24" w:color="92D050"/>` +
		`<w:bottom w:val="single" w:sz="24" w:space="24" w:color="92D050"/><w:right w:val="single" w:sz="24" w:space="24" w:color="92D050"/>` +
		`</w:pgBorders></w:sectPr>`
	buf := fixture.DOCX(body)
	html, sheet := convertHTML(t, buf)
	if !strings.Contains(html, "position: running(pb1)") || !strings.Contains(sheet, "@top-left-corner { content: element(pb1)") {
		t.Fatalf("expected the borders of the page:\n%s\n%s", sheet, html)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	img := render(t, d, 0)
	// the left border of 3 points at 24 points from the edge, at 72 dots
	// per inch, half way down the page
	got := color.NRGBAModel.Convert(img.At(25, 300)).(color.NRGBA)
	if got.G < 150 || got.R > 200 || got.B > 120 {
		t.Errorf("expected the green border at (25, 300), got %v", got)
	}
	if got := color.NRGBAModel.Convert(img.At(40, 300)).(color.NRGBA); got.R < 250 {
		t.Errorf("expected white inside the border, got %v", got)
	}
}

// TestFooterDrawingTop checks that a drawing that a paragraph of a footer
// places starts at the top of the paragraph, with the height of the
// paragraphs of the footer below it.
func TestFooterDrawingTop(t *testing.T) {
	c := &converter{sections: []*section{defaultSection(ivy.Paper{})}, margin: "footer"}
	c.hf = hfHeight{height: 40}
	c.hfTop = 10
	// the footer of 40 points ends 36 points above the bottom of the
	// paper of 792 points, and the paragraph starts 10 points into it: 726
	// points from the top of the paper, 654 from the top of the page area
	if got := c.paragraphTop(); got != 654 {
		t.Errorf("expected the paragraph at 654 points, got %g", got)
	}
}
