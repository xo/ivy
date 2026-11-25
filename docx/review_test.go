package docx

import (
	"bytes"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
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

// TestParagraphSpacing checks that the spacing before a paragraph is a
// margin, which collapses with the spacing after the paragraph before, as
// Word puts the larger of the two, and a padding, which adds to it, with
// w:doNotUseHTMLParagraphAutoSpacing.
func TestParagraphSpacing(t *testing.T) {
	p := `<w:p><w:pPr><w:spacing w:before="480" w:after="480"/></w:pPr><w:r><w:t>Text</w:t></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(p+p))
	if !strings.Contains(html, `margin-top: 24pt`) || strings.Contains(html, `padding-top: 24pt`) {
		t.Errorf("expected the spacing before as a margin that collapses:\n%s", html)
	}
	settings := fixture.WordPart{ID: "rId9", Type: "settings", Name: "word/settings.xml",
		Data: fixture.WordXML("w:settings", `<w:compat><w:doNotUseHTMLParagraphAutoSpacing/></w:compat>`)}
	html, _ = convertHTML(t, fixture.DOCX(p+p, settings))
	if !strings.Contains(html, `padding-top: 24pt`) {
		t.Errorf("expected the spacing before as a padding that adds:\n%s", html)
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
	red := `<w:r><w:rPr><w:color w:val="FF0000"/><w:b/><w:sz w:val="48"/></w:rPr><w:t>BODY</w:t></w:r>`
	body := `<w:p>` + red + `</w:p><w:p><w:pPr><w:pageBreakBefore/></w:pPr>` + red + `</w:p>` +
		`<w:sectPr><w:headerReference w:type="default" r:id="rIdH"/>` +
		`<w:footerReference w:type="default" r:id="rIdF"/><w:pgSz w:w="12240" w:h="15840"/>` +
		`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720"/></w:sectPr>`
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body,
		fixture.WordPart{ID: "rIdH", Type: "header", Name: "word/header1.xml", Data: header},
		fixture.WordPart{ID: "rIdF", Type: "footer", Name: "word/footer1.xml", Data: footer})))
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		img := render(t, d, i)
		// The text of about 1200 characters of 10 points takes about 13
		// lines from 36 points down, and the body starts below it.
		bodyBox := redBox(img)
		if bodyBox.Empty() || bodyBox.Min.Y < 150 || bodyBox.Min.Y > 240 {
			t.Errorf("page %d: expected the body below the header, at about 190 points, got %v", i, bodyBox)
		}
		headerBox := inkBox(img, image.Rect(0, 0, 612, bodyBox.Min.Y-8))
		if headerBox.Empty() || headerBox.Min.Y > 40 || headerBox.Max.Y < 150 {
			t.Errorf("page %d: expected the header from 36 points down, got %v", i, headerBox)
		}
	}
}

// TestFooterSpaceBefore checks that the space before the first paragraph
// of a footer stays in the footer, so that the footer keeps its distance
// from the bottom of the paper, as in Word. The running element is a flow
// root, as in msdoc, rtf and odf.
func TestFooterSpaceBefore(t *testing.T) {
	footerBottom := "1440"
	bottom := func(before string) int {
		footer := fixture.WordXML("w:ftr", `<w:p><w:pPr><w:spacing w:before="`+before+`" w:after="0"/></w:pPr><w:r><w:t>Footer</w:t></w:r></w:p>`)
		body := `<w:p><w:r><w:t>Body</w:t></w:r></w:p>` +
			`<w:sectPr><w:footerReference w:type="default" r:id="rIdF"/><w:pgSz w:w="12240" w:h="15840"/>` +
			`<w:pgMar w:top="1440" w:right="1440" w:bottom="` + footerBottom + `" w:left="1440" w:header="720" w:footer="720"/></w:sectPr>`
		d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body,
			fixture.WordPart{ID: "rIdF", Type: "footer", Name: "word/footer1.xml", Data: footer})))
		if err != nil {
			t.Fatal(err)
		}
		box := inkBox(render(t, d, 0), image.Rect(0, 700, 612, 792))
		if box.Empty() {
			t.Fatalf("space before %s: expected the footer at the bottom of the page", before)
		}
		return box.Max.Y
	}
	for _, footerBottom = range []string{"1440", "720", "2880"} {
		if without, with := bottom("0"), bottom("480"); with != without {
			t.Errorf("bottom margin %s: expected the footer at the same place with 24 points of space before it, got %d and %d",
				footerBottom, with, without)
		}
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

// TestPageBordersShadow checks that the shadow of the borders of a page
// draws a black band outside the right and the bottom borders, as Word
// does on the A5 page of aspose/docx/rendering.docx.
func TestPageBordersShadow(t *testing.T) {
	side := `w:val="single" w:sz="24" w:space="24" w:color="92D050" w:shadow="1"`
	body := `<w:p><w:r><w:t>Text</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="8391" w:h="11907"/>` +
		`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/><w:pgBorders w:offsetFrom="page">` +
		`<w:top ` + side + `/><w:left ` + side + `/><w:bottom ` + side + `/><w:right ` + side + `/></w:pgBorders></w:sectPr>`
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body)))
	if err != nil {
		t.Fatal(err)
	}
	img := render(t, d, 0)
	// The right border of 3 points ends 24 points from the right edge of
	// the paper of 419.55 points, and the bottom border 24 points from the
	// bottom edge of 595.35 points. The shadow is the 3 points after them.
	dark := func(x, y int) bool {
		c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
		return c.R < 60 && c.G < 60 && c.B < 60
	}
	for _, p := range [][2]int{{397, 300}, {200, 573}, {395, 573}} {
		if !dark(p[0], p[1]) {
			t.Errorf("expected the shadow at %v, got %v", p, img.At(p[0], p[1]))
		}
	}
	for _, p := range [][2]int{{21, 300}, {200, 22}, {397, 26}} {
		if dark(p[0], p[1]) {
			t.Errorf("expected no shadow at %v", p)
		}
	}
}

// TestVMLShapeNextPage checks that a VML shape that the part of a paragraph
// on the next page anchors is placed from the top of that part, as in Word
// (libreoffice/docx/mixed-orientation.docx).
func TestVMLShapeNextPage(t *testing.T) {
	text := strings.Repeat("The paragraph goes on to the next page. ", 160)
	shape := `<w:r><w:pict><v:rect style="position:absolute;margin-left:92pt;margin-top:81pt;width:50pt;height:30pt" fillcolor="#ff0000"/></w:pict></w:r>`
	body := `<w:p><w:r><w:t>` + text + `</w:t></w:r>` + shape + `<w:r><w:t>End.</w:t></w:r></w:p>` + tabsSection
	html, _ := convertHTML(t, fixture.DOCX(body))
	if !strings.Contains(html, "-ivy-static-top: block") {
		t.Errorf("expected the shape to start at the top of its paragraph:\n%s", html)
	}
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body)))
	if err != nil {
		t.Fatal(err)
	}
	if box := redBox(render(t, d, 0)); !box.Empty() {
		t.Errorf("expected no shape on the first page, got %v", box)
	}
	// The part of the paragraph on the second page starts at the top
	// margin of 72 points.
	if box := redBox(render(t, d, 1)); box.Min.Y < 150 || box.Min.Y > 156 || box.Min.X < 162 || box.Min.X > 166 {
		t.Errorf("expected the shape at (164, 153), got %v", box)
	}
}

// TestVMLShapeColumn checks that a VML shape is placed from the left of the
// column of its anchor, and that the columns of the last section are not
// balanced, as in Word: the rectangle of page 2 of
// libreoffice/docx/mixed-orientation.docx is in the column where its
// paragraph ends.
func TestVMLShapeColumn(t *testing.T) {
	text := strings.Repeat("The text fills the first column. ", 130)
	shape := `<w:r><w:pict><v:rect style="position:absolute;margin-left:20pt;margin-top:30pt;width:50pt;height:30pt" fillcolor="#ff0000"/></w:pict></w:r>`
	section := `<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/><w:cols w:num="2" w:space="720"/></w:sectPr>`
	body := `<w:p><w:r><w:t>` + text + `</w:t></w:r>` + shape + `</w:p>` + section
	html, _ := convertHTML(t, fixture.DOCX(body))
	if !strings.Contains(html, "column-fill: auto") {
		t.Errorf("expected the columns of the last section to fill in turn:\n%s", html)
	}
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body)))
	if err != nil {
		t.Fatal(err)
	}
	// The second column starts at 72 + 216 + 36 = 324 points.
	if box := redBox(render(t, d, 0)); box.Min.X < 342 || box.Min.X > 346 || box.Min.Y < 100 || box.Min.Y > 104 {
		t.Errorf("expected the shape at (344, 102) in the second column, got %v", box)
	}
}

// TestBreakBeforeMark checks that a break of a line at the end of a
// paragraph gives the mark of the paragraph a line of its own, as in Word,
// and that a break that clears a float just before it does not
// (aspose/docx/rendering.docx). A break that clears no float is a break of
// a line (page 2 of poi/docx/drawing.docx).
func TestBreakBeforeMark(t *testing.T) {
	clearing := `<w:p><w:r><w:br w:type="textWrapping" w:clear="all"/></w:r></w:p>`
	body := `<w:p><w:r><w:t>Text</w:t><w:br/><w:br/></w:r></w:p>` + clearing
	html, _ := convertHTML(t, fixture.DOCX(body))
	if !strings.Contains(html, "Text</span><br/><br/><br/></p>") && !strings.Contains(html, "Text<br/><br/><br/></p>") {
		t.Errorf("expected a third break for the mark of the paragraph:\n%s", html)
	}
	if strings.Contains(html, `<br class="clear"/>`) || !strings.Contains(html, `"><br/><br/></p>`) {
		t.Errorf("expected a line for the mark after a break that clears no float:\n%s", html)
	}
	float := `<w:tbl><w:tblPr><w:tblpPr w:vertAnchor="text" w:tblpX="2880" w:tblpY="1"/></w:tblPr>` +
		`<w:tblGrid><w:gridCol w:w="2000"/></w:tblGrid><w:tr><w:tc><w:p><w:r><w:t>Cell</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`
	html, _ = convertHTML(t, fixture.DOCX(float+`<w:p/>`+clearing))
	if !strings.Contains(html, `<br class="clear"/></p>`) {
		t.Errorf("expected no break after a break that clears a float:\n%s", html)
	}
}
