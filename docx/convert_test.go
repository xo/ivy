package docx

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// stylesPart is a styles.xml with defaults, a heading based on Normal, a
// character style, a table style and a loop of basedOn.
var stylesPart = fixture.WordPart{ID: "rIdS", Type: "styles", Name: "word/styles.xml", Data: fixture.WordXML("w:styles", `
<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:asciiTheme="minorHAnsi" w:hAnsiTheme="minorHAnsi"/><w:sz w:val="22"/></w:rPr></w:rPrDefault>
<w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="259" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>
<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>
<w:style w:type="paragraph" w:styleId="Heading1"><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="240" w:after="0"/></w:pPr><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial"/><w:b/><w:color w:val="2F5496"/><w:sz w:val="32"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading2"><w:basedOn w:val="Heading1"/><w:rPr><w:i/><w:sz w:val="26"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="LoopA"><w:basedOn w:val="LoopB"/><w:rPr><w:u w:val="single"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="LoopB"><w:basedOn w:val="LoopA"/><w:rPr><w:caps/></w:rPr></w:style>
<w:style w:type="character" w:styleId="Strong"><w:rPr><w:b/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="ListParagraph"><w:basedOn w:val="Normal"/><w:pPr><w:ind w:left="720"/><w:contextualSpacing/></w:pPr></w:style>
<w:style w:type="table" w:default="1" w:styleId="TableNormal"><w:tblPr><w:tblCellMar><w:left w:w="108" w:type="dxa"/><w:right w:w="108" w:type="dxa"/></w:tblCellMar></w:tblPr></w:style>
<w:style w:type="table" w:styleId="Grid"><w:basedOn w:val="TableNormal"/><w:pPr><w:spacing w:after="0"/></w:pPr><w:tblPr><w:tblBorders><w:top w:val="single" w:sz="4" w:color="000000"/><w:left w:val="single" w:sz="4" w:color="000000"/><w:bottom w:val="single" w:sz="4" w:color="000000"/><w:right w:val="single" w:sz="4" w:color="000000"/><w:insideH w:val="single" w:sz="4" w:color="000000"/><w:insideV w:val="single" w:sz="4" w:color="000000"/></w:tblBorders></w:tblPr>
<w:tblStylePr w:type="firstRow"><w:rPr><w:b/></w:rPr><w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="D9E2F3"/></w:tcPr></w:tblStylePr></w:style>
`)}

// themePart is a theme with Calibri for the body and Calibri Light for the
// headings.
var themePart = fixture.WordPart{ID: "rIdT", Type: "theme", Name: "word/theme/theme1.xml", Data: []byte(`<?xml version="1.0"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office"><a:themeElements>
<a:clrScheme name="Office"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:accent1><a:srgbClr val="4472C4"/></a:accent1></a:clrScheme>
<a:fontScheme name="Office"><a:majorFont><a:latin typeface="Calibri Light"/></a:majorFont><a:minorFont><a:latin typeface="Calibri"/></a:minorFont></a:fontScheme>
</a:themeElements></a:theme>`)}

// numberingPart is a numbering.xml with a bulleted list and a numbered list
// of two levels.
var numberingPart = fixture.WordPart{ID: "rIdN", Type: "numbering", Name: "word/numbering.xml", Data: fixture.WordXML("w:numbering", `
<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="`+"\uf0b7"+`"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr><w:rPr><w:rFonts w:ascii="Symbol" w:hAnsi="Symbol"/></w:rPr></w:lvl></w:abstractNum>
<w:abstractNum w:abstractNumId="1"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl>
<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%1.%2)"/><w:pPr><w:ind w:left="1440" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>
<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
<w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>
<w:num w:numId="3"><w:abstractNumId w:val="1"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="5"/></w:lvlOverride></w:num>
`)}

// headerPart is a header with the number of the page and the number of the
// pages.
var headerPart = fixture.WordPart{ID: "rIdH", Type: "header", Name: "word/header1.xml", Data: fixture.WordXML("w:hdr", `
<w:p><w:pPr><w:jc w:val="right"/></w:pPr><w:r><w:t xml:space="preserve">Page </w:t></w:r><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>9</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r><w:r><w:t xml:space="preserve"> of </w:t></w:r><w:fldSimple w:instr=" NUMPAGES "><w:r><w:t>9</w:t></w:r></w:fldSimple></w:p>`)}

// footnotesPart holds footnote 1.
var footnotesPart = fixture.WordPart{ID: "rIdF", Type: "footnotes", Name: "word/footnotes.xml", Data: fixture.WordXML("w:footnotes", `
<w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote>
<w:footnote w:id="1"><w:p><w:r><w:rPr><w:vertAlign w:val="superscript"/></w:rPr><w:footnoteRef/></w:r><w:r><w:t xml:space="preserve"> The note.</w:t></w:r></w:p></w:footnote>`)}

// imagePart is a red PNG image of 40 by 20 pixels.
var imagePart = fixture.WordPart{ID: "rIdI", Type: "image", Name: "word/media/image1.png", Data: fixture.PNG(40, 20, color.NRGBA{R: 255, A: 255})}

// linkPart is an external hyperlink.
var linkPart = fixture.WordPart{ID: "rIdL", Type: "hyperlink", Name: "https://example.com/a?b=1&c=2", External: true}

// inlineImage is a run with an inline drawing of the image of imagePart,
// of 1 by 0.5 inch.
const inlineImage = `<w:r><w:drawing><wp:inline><wp:extent cx="914400" cy="457200"/><wp:docPr id="1" name="Picture 1" descr="A red box"/>
<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:blipFill><a:blip r:embed="rIdI"/></pic:blipFill></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`

// sectionA5 is a section of a page of 5.83 by 8.27 inches with margins of
// 0.5 inch and the header of headerPart.
const sectionA5 = `<w:sectPr><w:headerReference w:type="default" r:id="rIdH"/><w:pgSz w:w="8392" w:h="11907"/><w:pgMar w:top="720" w:right="720" w:bottom="720" w:left="720" w:header="360" w:footer="360"/></w:sectPr>`

// richBody is a body with each feature that the tests check.
const richBody = `<w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr><w:r><w:t>Heading</w:t></w:r></w:p>
<w:p><w:r><w:t xml:space="preserve">Plain </w:t></w:r><w:r><w:rPr><w:rStyle w:val="Strong"/><w:highlight w:val="yellow"/></w:rPr><w:t>strong</w:t></w:r><w:r><w:rPr><w:strike/><w:vertAlign w:val="superscript"/></w:rPr><w:t>sup</w:t></w:r><w:ins><w:r><w:t xml:space="preserve"> inserted</w:t></w:r></w:ins><w:del><w:r><w:delText>deleted</w:delText></w:r></w:del><w:r><w:rPr><w:vanish/></w:rPr><w:t>hidden</w:t></w:r><w:r><w:footnoteReference w:id="1"/></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="ListParagraph"/><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Bullet</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="2"/></w:numPr></w:pPr><w:r><w:t>One</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="2"/></w:numPr></w:pPr><w:r><w:t>One A</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="2"/></w:numPr></w:pPr><w:r><w:t>Two</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="3"/></w:numPr></w:pPr><w:r><w:t>Five</w:t></w:r></w:p>
<w:p><w:hyperlink r:id="rIdL"><w:r><w:t>link</w:t></w:r></w:hyperlink><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText>REF x</w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t xml:space="preserve"> cached</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="LoopA"/></w:pPr><w:r><w:t>loop</w:t></w:r></w:p>
<w:tbl><w:tblPr><w:tblStyle w:val="Grid"/><w:tblW w:w="0" w:type="auto"/></w:tblPr><w:tblGrid><w:gridCol w:w="2000"/><w:gridCol w:w="3000"/></w:tblGrid>
<w:tr><w:trPr><w:tblHeader/></w:trPr><w:tc><w:p><w:r><w:t>H1</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>H2</w:t></w:r></w:p></w:tc></w:tr>
<w:tr><w:tc><w:tcPr><w:vMerge w:val="restart"/></w:tcPr><w:p><w:r><w:t>Merged</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc></w:tr>
<w:tr><w:tc><w:tcPr><w:vMerge/></w:tcPr><w:p/></w:tc><w:tc><w:p><w:r><w:t>C</w:t></w:r></w:p></w:tc></w:tr>
<w:tr><w:tc><w:tcPr><w:gridSpan w:val="2"/></w:tcPr><w:p><w:r><w:t>Wide</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
<w:p>` + inlineImage + `</w:p>
<w:p><w:r><w:br w:type="page"/></w:r><w:r><w:t>Second page</w:t></w:r></w:p>
` + sectionA5

// richDocx returns a Word file of richBody with every part.
func richDocx() []byte {
	return fixture.DOCX(richBody, stylesPart, themePart, numberingPart, headerPart, footnotesPart, imagePart, linkPart)
}

// convertHTML converts a Word file and returns its XHTML and the rules of
// its sections.
func convertHTML(t *testing.T, buf []byte) (string, string) {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(src.FS, documentName)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), src.StyleSheets[1]
}

func TestConvert(t *testing.T) {
	html, sheet := convertHTML(t, richDocx())
	for _, want := range []string{
		// The heading inherits the font, the bold and the color of
		// Heading1, and its own italic and size.
		`font-family: &quot;Arial&quot;, sans-serif; font-size: 13pt; font-weight: bold; font-style: italic; color: rgba(47,84,150,1.000)`,
		`break-after: avoid`,
		// A font of the theme resolves through the theme.
		`font-family: &quot;Calibri&quot;, sans-serif; font-size: 11pt`,
		// The line spacing of 259 is 1.08 times a line of Calibri.
		`line-height: 1.32`,
		`<span style="font-weight: bold; background-color: #ffff00">strong</span>`,
		`text-decoration-line: line-through; vertical-align: super">sup</span>`,
		` inserted`,
		// The bullet of Symbol is a bullet of Unicode.
		`min-width: 18pt; text-indent: 0`,
		`>•</span>Bullet`,
		`>1.</span>One`,
		`>1.a)</span>One A`,
		`>2.</span>Two`,
		`>5.</span>Five`,
		`<a href="https://example.com/a?b=1&amp;c=2">link</a>`,
		` cached`,
		`<span class="fn"><p `,
		`<span class="fnref" style="font-size: 7.15pt; vertical-align: super"></span> The note.`,
		`<col style="width: 100pt"/><col style="width: 150pt"/>`,
		`<thead><tr>`,
		`rowspan="2"`,
		`colspan="2"`,
		`background-color: rgba(217,226,243,1.000)`,
		`border-top: 0.5pt solid rgba(0,0,0,1.000)`,
		`<img src="media/1.png" alt="A red box" style="width: 72pt; height: 36pt"/>`,
		`<p style="break-before: page; `,
		`<span class="pg"></span> of <span class="pgs"></span>`,
		`position: running(hf1)`,
		`text-transform: uppercase`,
		`<span style="text-decoration-line: underline">loop</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the xhtml to hold %q", want)
		}
	}
	if t.Failed() {
		t.Log(html)
	}
	for _, unwanted := range []string{"deleted", "hidden", ">9<", "REF x"} {
		if strings.Contains(html, unwanted) {
			t.Errorf("expected the xhtml without %q, got:\n%s", unwanted, html)
		}
	}
	for _, want := range []string{
		// The header at 18 points from the top of the paper can take more
		// than the margin of 36 points, which then grows to hold it.
		`@page s1 { size: 419.6pt 595.35pt; margin: 36pt 36pt 36pt 36pt; -ivy-margin-box-fit: grow; @top-center { content: element(hf1); padding-top: 18pt }`,
	} {
		if !strings.Contains(sheet, want) {
			t.Errorf("expected the style sheet to hold %q, got:\n%s", want, sheet)
		}
	}
}

// render renders page n of a Word file at 72 dots per inch.
func render(t *testing.T, d *Document, n int) image.Image {
	t.Helper()
	p, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// inkBox returns the box of the pixels of img that are darker than mid
// grey, in the rectangle r.
func inkBox(img image.Image, r image.Rectangle) image.Rectangle {
	var box image.Rectangle
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if (cr+cg+cb)/3 < 0x8000 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// redBox returns the box of the red pixels of img.
func redBox(img image.Image) image.Rectangle {
	var box image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r > 0xe000 && g < 0x2000 && bl < 0x2000 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

func TestPages(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(richDocx()))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 pages, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w < 419 || w > 420 || h < 595 || h > 596 {
		t.Errorf("expected a page of 419.6 by 595.35 points, got: %v by %v", w, h)
	}
	img := render(t, d, 0)
	// The text stays in the margins of 36 points, and the header is in the
	// top margin, at the right.
	body := inkBox(img, image.Rect(0, 36, 420, 559))
	// The borders of the table start a cell margin of 5.4 points before
	// the left margin, as in Word.
	if body.Empty() || body.Min.X < 29 || body.Max.X > 386 {
		t.Errorf("expected the text inside the margins, got: %v", body)
	}
	header := inkBox(img, image.Rect(0, 0, 420, 36))
	if header.Empty() || header.Min.X < 300 || header.Min.Y < 17 {
		t.Errorf("expected the header at the right of the top margin, below 18 points, got: %v", header)
	}
	// The image is 72 by 36 points of red.
	if box := redBox(img); box.Dx() < 70 || box.Dx() > 74 || box.Dy() < 34 || box.Dy() > 38 {
		t.Errorf("expected a red image of 72 by 36 points, got: %v", box)
	}
	// The table draws its borders: a column of ink at the left edge of the
	// table, which starts a cell margin before the text.
	if second := inkBox(render(t, d, 1), image.Rect(0, 36, 420, 559)); second.Empty() {
		t.Error("expected text on the second page")
	}
}

func TestHeaderNumbers(t *testing.T) {
	body := `<w:p><w:r><w:t>One</w:t></w:r></w:p><w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>Two</w:t></w:r></w:p>` + sectionA5
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body, headerPart)))
	if err != nil {
		t.Fatal(err)
	}
	// "Page 1 of 2" and "Page 2 of 2" differ only in a digit, so the ink of
	// the headers differs.
	h1 := inkBox(render(t, d, 0), image.Rect(0, 0, 420, 36))
	h2 := inkBox(render(t, d, 1), image.Rect(0, 0, 420, 36))
	if h1.Empty() || h2.Empty() {
		t.Fatalf("expected a header on each page, got: %v and %v", h1, h2)
	}
}

func TestSections(t *testing.T) {
	body := `<w:p><w:pPr><w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr></w:pPr><w:r><w:t>Portrait</w:t></w:r></w:p>
<w:p><w:r><w:t>Landscape</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="15840" w:h="12240" w:orient="landscape"/><w:pgMar w:top="720" w:right="720" w:bottom="720" w:left="720"/><w:cols w:num="2" w:space="360"/></w:sectPr>`
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body)))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 2 {
		t.Fatalf("expected 2 pages, got: %d, %v", n, err)
	}
	for i, want := range [][2]float32{{612, 792}, {792, 612}} {
		p, err := d.Page(t.Context(), i)
		if err != nil {
			t.Fatal(err)
		}
		if w, h := p.Size(); w != want[0] || h != want[1] {
			t.Errorf("expected page %d of %v points, got: %v by %v", i+1, want, w, h)
		}
	}
	html, _ := convertHTML(t, fixture.DOCX(body))
	if !strings.Contains(html, `style="page: s2; break-before: page; columns: 2; column-gap: 18pt; column-fill: auto"`) {
		t.Errorf("expected the second section with two columns, got:\n%s", html)
	}
}

func TestPaper(t *testing.T) {
	landscape := ivy.A4.Landscape()
	text := `<w:p><w:r><w:t>Text</w:t></w:r></w:p>`
	for _, test := range []struct {
		name  string
		body  string
		paper ivy.Paper
		exp   ivy.Paper
	}{
		{"default", text, ivy.Paper{}, ivy.Letter},
		{"no sectPr", text, landscape, landscape},
		{"no pgSz", text + `<w:sectPr><w:pgMar w:top="720" w:right="720" w:bottom="720" w:left="720"/></w:sectPr>`, landscape, landscape},
		{"orient only", text + `<w:sectPr><w:pgSz w:orient="landscape"/></w:sectPr>`, ivy.A4, landscape},
		{"own paper", text + `<w:sectPr><w:pgSz w:w="12240" w:h="20160"/></w:sectPr>`, landscape, ivy.Legal},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(test.body)), WithConfig(ivy.Config{Paper: test.paper}))
			if err != nil {
				t.Fatal(err)
			}
			fixture.CheckPaper(t, d, test.exp)
		})
	}
	// the margins of Word stay on the paper of the program
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(text)), WithConfig(ivy.Config{Paper: landscape}))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if exp := "size: 841.89pt 595.28pt; margin: 72pt 72pt 72pt 72pt"; !strings.Contains(src.StyleSheets[len(src.StyleSheets)-1], exp) {
		t.Errorf("expected %q in the rules of the pages, got:\n%s", exp, src.StyleSheets[len(src.StyleSheets)-1])
	}
}

func TestDescribe(t *testing.T) {
	buf := fixture.Zip(contentTypes,
		fixture.Entry{Name: "docProps/core.xml", Data: []byte(`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>A title</dc:title><dc:creator>An author</dc:creator></cp:coreProperties>`)})
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "A title" || len(info.Authors) != 1 || info.Authors[0] != "An author" {
		t.Errorf("expected the title and the author, got: %+v", info)
	}
}

func TestNoMainPart(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(fixture.Zip(contentTypes)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.PageCount(t.Context()); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for a package without a main part, got: %v", err)
	}
}

func TestDeepTables(t *testing.T) {
	var b strings.Builder
	const depth = 200
	for range depth {
		b.WriteString(`<w:tbl><w:tr><w:tc>`)
	}
	b.WriteString(`<w:p><w:r><w:t>deep</w:t></w:r></w:p>`)
	for range depth {
		b.WriteString(`</w:tc></w:tr></w:tbl>`)
	}
	b.WriteString(`<w:p><w:r><w:t>after</w:t></w:r></w:p>`)
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(b.String())), WithConfig(ivy.Config{Limits: ivy.Limits{MaxDepth: 16}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.PageCount(t.Context()); err != nil {
		t.Fatal(err)
	}
	html, _ := convertHTML(t, fixture.DOCX(b.String()))
	if strings.Contains(html, "deep") || !strings.Contains(html, "after") {
		t.Errorf("expected the content past the depth left out, and the rest kept")
	}
}

func TestMetafileImage(t *testing.T) {
	emf := fixture.NewEMF(100, 50)
	emf.Select(emf.Brush(color.NRGBA{R: 255, A: 255}, -1))
	emf.Select(fixture.Stock(8))
	emf.Rectangle(0, 0, 100, 50)
	body := `<w:p>` + inlineImage + `</w:p>`
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body,
		fixture.WordPart{ID: "rIdI", Type: "image", Name: "word/media/image1.emf", Data: emf.Bytes()})))
	if err != nil {
		t.Fatal(err)
	}
	if box := redBox(render(t, d, 0)); box.Dx() < 70 || box.Dx() > 74 || box.Dy() < 34 || box.Dy() > 38 {
		t.Errorf("expected a red picture of 72 by 36 points, got: %v", box)
	}
}

func TestAnchoredTextBox(t *testing.T) {
	body := `<w:p><w:r><mc:AlternateContent><mc:Choice Requires="wps"><w:drawing><wp:anchor behindDoc="0" distL="0" distR="0" distT="0" distB="0">
<wp:positionH relativeFrom="column"><wp:align>right</wp:align></wp:positionH><wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
<wp:extent cx="1270000" cy="635000"/><wp:wrapSquare wrapText="bothSides"/><wp:docPr id="2" name="Text Box 2"/>
<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"><wps:wsp><wps:spPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></wps:spPr>
<wps:txbx><w:txbxContent><w:p><w:r><w:t>In the box</w:t></w:r></w:p></w:txbxContent></wps:txbx><wps:bodyPr/></wps:wsp></a:graphicData></a:graphic></wp:anchor></w:drawing></mc:Choice>
<mc:Fallback><w:pict/></mc:Fallback></mc:AlternateContent></w:r><w:r><w:t>Text that wraps.</w:t></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	for _, want := range []string{`float: right`, `width: 100pt`, `background-color: rgba(255,0,0,1.000)`, `In the box`} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the xhtml to hold %q, got:\n%s", want, html)
		}
	}
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body)))
	if err != nil {
		t.Fatal(err)
	}
	// The box floats to the right of the column, which ends at 540 points.
	if box := redBox(render(t, d, 0)); box.Max.X < 536 || box.Max.X > 542 || box.Dx() < 98 {
		t.Errorf("expected a red box of 100 points at the right of the column, got: %v", box)
	}
}

// chartPart is a chart part of a column chart of three red columns.
var chartPart = fixture.WordPart{ID: "rIdC", Type: "chart", Name: "word/charts/chart1.xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><c:chart><c:autoTitleDeleted val="1"/><c:plotArea><c:layout/>
<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/><c:ser><c:idx val="0"/><c:order val="0"/><c:spPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></c:spPr>
<c:cat><c:strRef><c:f>A</c:f><c:strCache><c:ptCount val="3"/><c:pt idx="0"><c:v>a</c:v></c:pt><c:pt idx="1"><c:v>b</c:v></c:pt><c:pt idx="2"><c:v>c</c:v></c:pt></c:strCache></c:strRef></c:cat>
<c:val><c:numRef><c:f>B</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="3"/><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt><c:pt idx="2"><c:v>3</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser>
<c:axId val="1"/><c:axId val="2"/></c:barChart><c:catAx><c:axId val="1"/><c:crossAx val="2"/></c:catAx><c:valAx><c:axId val="2"/><c:crossAx val="1"/></c:valAx></c:plotArea><c:plotVisOnly val="1"/></c:chart></c:chartSpace>`)}

func TestChart(t *testing.T) {
	body := `<w:p><w:r><w:drawing><wp:inline><wp:extent cx="2540000" cy="1905000"/><wp:docPr id="1" name="Chart 1"/>
<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rIdC"/></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`
	buf := fixture.DOCX(body, chartPart)
	html, _ := convertHTML(t, buf)
	if want := `<img src="ivy-object:chart1" alt="" style="width: 200pt; height: 150pt"/>`; !strings.Contains(html, want) {
		t.Errorf("expected the xhtml to hold %q, got:\n%s", want, html)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	// The columns of the chart are red, inside the box of the chart, from
	// the margin of 72 points.
	if box := redBox(render(t, d, 0)); box.Empty() || box.Min.X < 72 || box.Max.X > 272 || box.Max.Y > 222 {
		t.Errorf("expected red columns inside the chart of 200 by 150 points, got: %v", box)
	}
}
