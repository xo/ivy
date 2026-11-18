package odf

import (
	"bytes"
	"image"
	"image/color"
	"io/fs"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// odtNamespaces declares the prefixes of the namespaces of OpenDocument.
const odtNamespaces = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" ` +
	`xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" ` +
	`xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" ` +
	`xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" ` +
	`xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" ` +
	`xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" ` +
	`xmlns:xlink="http://www.w3.org/1999/xlink" ` +
	`xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0"`

// odtStylesXML is a styles.xml with a default style, a body style, a
// heading, a list style, and two master pages: a page of 5 by 4 inches
// with a header, and a landscape page.
const odtStylesXML = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles ` + odtNamespaces + `>
<office:font-face-decls><style:font-face style:name="Liberation Sans" svg:font-family="'Liberation Sans'" style:font-family-generic="swiss"/></office:font-face-decls>
<office:styles>
<style:default-style style:family="paragraph"><style:text-properties fo:font-size="12pt"/></style:default-style>
<style:style style:name="Standard" style:family="paragraph"/>
<style:style style:name="Body" style:family="paragraph" style:parent-style-name="Standard"><style:paragraph-properties fo:margin-bottom="6pt" fo:line-height="115%"/></style:style>
<style:style style:name="Heading" style:family="paragraph" style:parent-style-name="Body"><style:paragraph-properties fo:keep-with-next="always"/><style:text-properties style:font-name="Liberation Sans" fo:font-size="150%" fo:font-weight="bold"/></style:style>
<text:list-style style:name="Numbers">
<text:list-level-style-number text:level="1" style:num-suffix="." style:num-format="1"><style:list-level-properties text:list-level-position-and-space-mode="label-alignment"><style:list-level-label-alignment text:label-followed-by="listtab" fo:text-indent="-18pt" fo:margin-left="36pt"/></style:list-level-properties></text:list-level-style-number>
<text:list-level-style-number text:level="2" style:num-suffix=")" style:num-format="a" text:display-levels="2"><style:list-level-properties text:list-level-position-and-space-mode="label-alignment"><style:list-level-label-alignment text:label-followed-by="listtab" fo:text-indent="-18pt" fo:margin-left="54pt"/></style:list-level-properties></text:list-level-style-number>
</text:list-style>
</office:styles>
<office:automatic-styles>
<style:page-layout style:name="pm1"><style:page-layout-properties fo:page-width="5in" fo:page-height="4in" fo:margin-top="0.25in" fo:margin-bottom="0.5in" fo:margin-left="0.5in" fo:margin-right="0.5in"/><style:header-style><style:header-footer-properties fo:min-height="0in" fo:margin-bottom="0.1in"/></style:header-style></style:page-layout>
<style:page-layout style:name="pm2"><style:page-layout-properties fo:page-width="6in" fo:page-height="4in" fo:margin="0.5in"/></style:page-layout>
<style:style style:name="MP1" style:family="paragraph"><style:paragraph-properties fo:text-align="end"/><style:text-properties fo:font-size="10pt"/></style:style>
</office:automatic-styles>
<office:master-styles>
<style:master-page style:name="Standard" style:page-layout-name="pm1"><style:header><text:p text:style-name="MP1">Page <text:page-number>1</text:page-number> of <text:page-count>1</text:page-count></text:p></style:header></style:master-page>
<style:master-page style:name="Wide" style:page-layout-name="pm2"/>
</office:master-styles>
</office:document-styles>`

// odtContent returns a content.xml with the text body.
func odtContent(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content ` + odtNamespaces + `>
<office:automatic-styles>
<style:style style:name="T1" style:family="text"><style:text-properties fo:font-weight="bold" fo:color="#ff0000"/></style:style>
<style:style style:name="T2" style:family="text"><style:text-properties style:text-position="super 58%"/></style:style>
<style:style style:name="P1" style:family="paragraph" style:parent-style-name="Body"><style:paragraph-properties fo:text-align="center"/></style:style>
<style:style style:name="P2" style:family="paragraph" style:parent-style-name="Body" style:master-page-name="Wide"/>
<style:style style:name="Tbl" style:family="table"><style:table-properties style:width="3in"/></style:style>
<style:style style:name="Tbl.A" style:family="table-column"><style:table-column-properties style:column-width="1in"/></style:style>
<style:style style:name="Cell" style:family="table-cell"><style:table-cell-properties fo:padding="2pt" fo:border="0.5pt solid #000000" fo:background-color="#d9e2f3"/></style:style>
<style:style style:name="fr1" style:family="graphic"><style:graphic-properties style:wrap="none" style:horizontal-pos="center"/></style:style>
</office:automatic-styles>
<office:body><office:text>` + body + `</office:text></office:body></office:document-content>`
}

// odtRichBody is a text body with each feature that the tests check.
const odtRichBody = `<text:h text:style-name="Heading" text:outline-level="1">Heading</text:h>
<text:p text:style-name="P1">  Centered   <text:span text:style-name="T1">bold red</text:span><text:s text:c="2"/>end<text:span text:style-name="T2">sup</text:span><text:note text:note-class="footnote"><text:note-citation>1</text:note-citation><text:note-body><text:p text:style-name="Standard">The note.</text:p></text:note-body></text:note></text:p>
<text:list text:style-name="Numbers"><text:list-item><text:p text:style-name="Body">One</text:p><text:list><text:list-item><text:p text:style-name="Body">One A</text:p></text:list-item></text:list></text:list-item><text:list-item><text:p text:style-name="Body">Two</text:p></text:list-item></text:list>
<table:table table:style-name="Tbl"><table:table-column table:style-name="Tbl.A" table:number-columns-repeated="3"/>
<table:table-header-rows><table:table-row><table:table-cell table:style-name="Cell" table:number-columns-spanned="2"><text:p>Wide</text:p></table:table-cell><table:covered-table-cell/><table:table-cell table:style-name="Cell" table:number-rows-spanned="2"><text:p>Tall</text:p></table:table-cell></table:table-row></table:table-header-rows>
<table:table-row><table:table-cell table:style-name="Cell"><text:p>A</text:p></table:table-cell><table:table-cell table:style-name="Cell"><text:p>B</text:p></table:table-cell><table:covered-table-cell/></table:table-row></table:table>
<text:p text:style-name="Body"><draw:frame draw:style-name="fr1" text:anchor-type="paragraph" svg:width="72pt" svg:height="36pt"><draw:image xlink:href="Pictures/red.png"/></draw:frame></text:p>
<text:p text:style-name="Body"><text:a xlink:href="https://example.com/">link</text:a><text:line-break/>next<text:tab/>tab<text:soft-page-break/></text:p>
<text:p text:style-name="P2">Wide page</text:p>`

// odtFile returns an OpenDocument text with the body and the entries.
func odtFile(body string, entries ...fixture.Entry) []byte {
	all := make([]fixture.Entry, 0, 4+len(entries))
	all = append(all,
		fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "text")},
		fixture.Entry{Name: "content.xml", Data: []byte(odtContent(body))},
		fixture.Entry{Name: "styles.xml", Data: []byte(odtStylesXML)},
		fixture.Entry{Name: "Pictures/red.png", Data: fixture.PNG(40, 20, color.NRGBA{R: 255, A: 255})},
	)
	return fixture.Zip(append(all, entries...)...)
}

// odtHTML converts a text and returns its XHTML and the rules of its pages.
func odtHTML(t *testing.T, buf []byte, opts ...Option) (string, string) {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf), opts...)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.pagerOf()
	if err != nil {
		t.Fatal(err)
	}
	src, err := p.(*textPager).source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(src.FS, odtDocumentName)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), src.StyleSheets[1]
}

func TestTextConvert(t *testing.T) {
	html, sheet := odtHTML(t, odtFile(odtRichBody))
	for _, want := range []string{
		// The heading is 150% of the 12pt of the default style.
		`font-family: &quot;Liberation Sans&quot;, sans-serif; font-size: 18pt; font-weight: bold`,
		`break-after: avoid`,
		`text-align: center`,
		`line-height: 1.35`,
		`margin-bottom: 6pt`,
		// The white space collapses, and text:s keeps its spaces.
		`>Centered <span style="font-weight: bold; color: #ff0000">bold red</span>  end<span style="font-size: 6.96pt; vertical-align: super">sup</span>`,
		`<span class="fn"><p `,
		`<span class="fnref"></span>The note.`,
		`margin-left: 36pt; text-indent: -18pt`,
		`min-width: 18pt; text-indent: 0">1.</span>One`,
		`>1.a)</span>One A`,
		`>2.</span>Two`,
		`<thead><tr style=""><td colspan="2" style="border: 0.5pt solid #000000; padding: 2pt; background-color: #d9e2f3">`,
		`rowspan="2"`,
		`<col style="width: 72pt"/><col style="width: 72pt"/><col style="width: 72pt"/>`,
		`display: block; margin-left: auto; margin-right: auto; width: 72pt`,
		`<img src="media/1.png" alt="" style="width: 72pt; height: 36pt"/>`,
		`<a href="https://example.com/">link</a><br/>next` + "\t" + `tab`,
		`Page <span class="pg"></span> of <span class="pgs"></span>`,
		`style="page: m2; break-before: page"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the xhtml to hold %q", want)
		}
	}
	if t.Failed() {
		t.Log(html)
	}
	for _, want := range []string{
		// The body starts below the header: 18pt, a line of 10pt, and 7.2pt.
		`@page m1 { size: 360pt 288pt; margin: 36.9pt 36pt 36pt 36pt; -ivy-margin-box-fit: grow; @top-center { content: element(hf1); padding-top: 18pt; padding-bottom: 7.2pt }`,
		`@page m2 { size: 432pt 288pt; margin: 36pt 36pt 36pt 36pt; -ivy-margin-box-fit: none;`,
	} {
		if !strings.Contains(sheet, want) {
			t.Errorf("expected the style sheet to hold %q, got:\n%s", want, sheet)
		}
	}
}

// odtInk returns the box of the pixels of img that are darker than mid
// grey, in the rectangle r.
func odtInk(img image.Image, r image.Rectangle) image.Rectangle {
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

func TestTextPages(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(odtFile(odtRichBody)))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("expected 2 pages at least, got: %d", n)
	}
	first, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := first.Size(); w != 360 || h != 288 {
		t.Errorf("expected a first page of 360 by 288 points, got: %v by %v", w, h)
	}
	last, err := d.Page(t.Context(), n-1)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := last.Size(); w != 432 || h != 288 {
		t.Errorf("expected a last page of 432 by 288 points, got: %v by %v", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), first, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	// The header is at the right of the top margin, from 18 points.
	if header := odtInk(img, image.Rect(0, 0, 360, 36)); header.Empty() || header.Min.X < 250 || header.Min.Y < 17 {
		t.Errorf("expected the header at the right of the top margin, got: %v", header)
	}
	if body := odtInk(img, image.Rect(0, 37, 360, 252)); body.Empty() || body.Min.X < 35 || body.Max.X > 325 {
		t.Errorf("expected the text inside the margins, got: %v", body)
	}
}

func TestTextChart(t *testing.T) {
	body := `<text:p text:style-name="Standard"><draw:frame text:anchor-type="as-char" svg:width="144pt" svg:height="108pt">` +
		`<draw:object xlink:href="./Object 1" xlink:type="simple"/></draw:frame></text:p>`
	buf := odtFile(body, fixture.Entry{Name: "Object 1/content.xml", Data: fixture.ODFChart("ff0000")})
	html, _ := odtHTML(t, buf)
	if want := `src="ivy-object:chart1"`; !strings.Contains(html, want) {
		t.Errorf("expected the xhtml to hold %q, got:\n%s", want, html)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	// the red bars of the chart are in its frame of 144 by 108 points, at
	// the left margin of 36 points
	var box image.Rectangle
	n := 0
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.R > 200 && c.G < 60 && c.B < 60 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
				n++
			}
		}
	}
	if n < 500 || box.Min.X < 36 || box.Max.X > 36+145 || box.Dy() > 108 {
		t.Errorf("expected the red bars of the chart in its frame, got %d red pixels in %v", n, box)
	}
}

func TestTextEncrypted(t *testing.T) {
	buf := fixture.EncryptODF(
		fixture.ODFEncryption{Password: "secret", Cipher: fixture.ODFAESCBC, Iterations: 100},
		mimePrefix+"text",
		fixture.Entry{Name: "content.xml", Data: []byte(odtContent(`<text:p>Secret text</text:p>`))},
		fixture.Entry{Name: "styles.xml", Data: []byte(odtStylesXML)},
	)
	html, _ := odtHTML(t, buf, WithConfig(ivy.Config{Password: "secret"}))
	if !strings.Contains(html, "Secret text") {
		t.Errorf("expected the decrypted text, got:\n%s", html)
	}
}

func TestTextDeep(t *testing.T) {
	const depth = 300
	body := strings.Repeat(`<text:section>`, depth) + `<text:p>deep</text:p>` + strings.Repeat(`</text:section>`, depth) + `<text:p>after</text:p>`
	html, _ := odtHTML(t, odtFile(body), WithConfig(ivy.Config{Limits: ivy.Limits{MaxDepth: 16}}))
	if strings.Contains(html, "deep") || !strings.Contains(html, "after") {
		t.Errorf("expected the content past the depth left out, and the rest kept")
	}
}

// odtPaperFile returns an OpenDocument text of one paragraph, whose master
// page Standard has a page layout with the properties props. An empty
// props gives a text without styles.xml.
func odtPaperFile(props string) []byte {
	entries := []fixture.Entry{
		{Name: "mimetype", Data: []byte(mimePrefix + "text")},
		{Name: "content.xml", Data: []byte(odtContent(`<text:p>Text</text:p>`))},
	}
	if props != "" {
		entries = append(entries, fixture.Entry{Name: "styles.xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles ` + odtNamespaces + `><office:automatic-styles>
<style:page-layout style:name="pm1"><style:page-layout-properties ` + props + `/></style:page-layout>
</office:automatic-styles><office:master-styles><style:master-page style:name="Standard" style:page-layout-name="pm1"/></office:master-styles>
</office:document-styles>`)})
	}
	return fixture.Zip(entries...)
}

func TestTextPaper(t *testing.T) {
	landscape := ivy.A4.Landscape()
	for _, test := range []struct {
		name  string
		props string
		paper ivy.Paper
		exp   ivy.Paper
	}{
		{"default", "", ivy.Paper{}, ivy.Letter},
		{"no styles", "", landscape, landscape},
		{"no size", `fo:margin="0.5in"`, landscape, landscape},
		{"orientation only", `style:print-orientation="landscape"`, ivy.A4, landscape},
		{"own size", `fo:page-width="8.5in" fo:page-height="14in"`, landscape, ivy.Legal},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(odtPaperFile(test.props)), WithConfig(ivy.Config{Paper: test.paper}))
			if err != nil {
				t.Fatal(err)
			}
			fixture.CheckPaper(t, d, test.exp)
		})
	}
	// the margins of the layout stay on the paper of the program
	_, rules := odtHTML(t, odtPaperFile(`fo:margin="0.5in"`), WithConfig(ivy.Config{Paper: landscape}))
	if exp := "size: 841.89pt 595.28pt; margin: 36pt 36pt 36pt 36pt"; !strings.Contains(rules, exp) {
		t.Errorf("expected %q in the rules of the pages, got:\n%s", exp, rules)
	}
}

func FuzzText(f *testing.F) {
	f.Add(odtFile(odtRichBody))
	f.Add(odtFile(`<text:list><text:list-item><text:list><text:list-item><text:p>x</text:p></text:list-item></text:list></text:list-item></text:list>`))
	f.Add(odtStyledFile(`<text:outline-style><text:outline-level-style text:level="1" style:num-format="1"/></text:outline-style>`+
		`<style:style style:name="T" style:family="paragraph"><style:paragraph-properties><style:tab-stops><style:tab-stop style:position="1in" style:type="right" style:leader-text="."/></style:tab-stops></style:paragraph-properties></style:style>`,
		`<text:h text:outline-level="1">H</text:h><text:p text:style-name="T">a<text:tab/>1<draw:rect text:anchor-type="as-char" svg:width="9pt" svg:height="9pt"><text:p>r</text:p></draw:rect></text:p>`+
			`<draw:custom-shape text:anchor-type="paragraph" svg:width="72pt" svg:height="72pt"><text:p><draw:frame svg:width="9pt" svg:height="9pt"><draw:image xlink:href="Pictures/red.png"/></draw:frame></text:p></draw:custom-shape>`,
		fixture.Entry{Name: "Pictures/red.png", Data: fixture.PNG(4, 4, color.Black)}))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 1 << 14, MaxDepth: 32}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		// Only a text renders here: the pages of the other kinds belong
		// to their own fuzz tests.
		if !strings.HasPrefix(d.mime, mimePrefix+"text") {
			return
		}
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		_, _ = ivy.RenderImage(t.Context(), p, 9, nil)
	})
}

// odtStyledFile returns an OpenDocument text with the body, whose
// styles.xml holds the styles of odtStylesXML and the elements styles in
// office:styles.
func odtStyledFile(styles, body string, entries ...fixture.Entry) []byte {
	all := make([]fixture.Entry, 0, 3+len(entries))
	all = append(all,
		fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "text")},
		fixture.Entry{Name: "content.xml", Data: []byte(odtContent(body))},
		fixture.Entry{Name: "styles.xml", Data: []byte(strings.Replace(odtStylesXML, "</office:styles>", styles+"</office:styles>", 1))},
	)
	return fixture.Zip(append(all, entries...)...)
}

func TestTextOutline(t *testing.T) {
	const outline = `<text:outline-style style:name="Outline">` +
		`<text:outline-level-style text:level="1" style:num-format="1"><style:list-level-properties text:min-label-width="0.5in"/></text:outline-level-style>` +
		`<text:outline-level-style text:level="2" style:num-format="1" text:display-levels="2"><style:list-level-properties text:min-label-width="0.5in"/></text:outline-level-style>` +
		`<text:outline-level-style text:level="3" style:num-format=""/>` +
		`</text:outline-style>`
	body := `<text:h text:outline-level="1">One</text:h>` +
		`<text:h text:outline-level="2"><text:number>9.9</text:number>One A</text:h>` +
		`<text:h text:outline-level="2">One B</text:h>` +
		`<text:h text:outline-level="3">Plain</text:h>` +
		`<text:h text:outline-level="1">Two</text:h>` +
		`<text:h text:outline-level="2">Two A</text:h>` +
		`<text:h text:outline-level="1" text:is-list-header="true">Header<text:placeholder text:placeholder-type="text">Enter</text:placeholder></text:h>` +
		`<text:h text:outline-level="1" text:restart-numbering="true" text:start-value="7">Seven</text:h>` +
		`<text:list text:style-name="Numbers"><text:list-item><text:h text:outline-level="1">Listed</text:h></text:list-item></text:list>`
	html, _ := odtHTML(t, odtStyledFile(outline, body))
	for _, want := range []string{
		`margin-left: 36pt; text-indent: -36pt`,
		`min-width: 36pt; text-indent: 0">1</span>One<`,
		`>1.1</span>One A<`,
		`>1.2</span>One B<`,
		`>2</span>Two<`,
		`>2.1</span>Two A<`,
		`>7</span>Seven<`,
		// a heading in a list takes the number of the list
		`>1.</span>Listed<`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the xhtml to hold %q", want)
		}
	}
	// text:number gives no second number, a heading of a list header and
	// a level without a format have no number, and a placeholder does not
	// print
	for _, unwanted := range []string{"9.9", `</span>Plain`, `</span>Header`, "Enter"} {
		if strings.Contains(html, unwanted) {
			t.Errorf("expected the xhtml not to hold %q", unwanted)
		}
	}
	if t.Failed() {
		t.Log(html)
	}
}

// odtColorBox returns the box of the pixels of img near the color c, and
// their number.
func odtColorBox(img image.Image, c color.NRGBA) (image.Rectangle, int) {
	var box image.Rectangle
	n := 0
	near := func(a, b uint8) bool { return a > b-40 && a < b+40 || a == b }
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			p := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if near(p.R, c.R) && near(p.G, c.G) && near(p.B, c.B) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
				n++
			}
		}
	}
	return box, n
}

func TestTextShapes(t *testing.T) {
	const styles = `<style:style style:name="red" style:family="graphic"><style:graphic-properties draw:fill="solid" draw:fill-color="#ff0000" draw:stroke="none"/></style:style>` +
		`<style:style style:name="blue" style:family="graphic"><style:graphic-properties draw:fill="solid" draw:fill-color="#0000ff" draw:stroke="none" style:wrap="none"/></style:style>` +
		`<style:style style:name="green" style:family="graphic"><style:graphic-properties draw:stroke="solid" svg:stroke-color="#00ff00" svg:stroke-width="4pt"/></style:style>`
	body := `<text:p>Box <draw:rect text:anchor-type="as-char" draw:style-name="red" svg:width="36pt" svg:height="18pt"><text:p/></draw:rect> after</text:p>` +
		`<text:p><draw:g text:anchor-type="paragraph" draw:style-name="blue">` +
		`<draw:ellipse draw:style-name="blue" svg:x="36pt" svg:y="0pt" svg:width="36pt" svg:height="36pt"/>` +
		`<draw:rect draw:style-name="blue" svg:x="108pt" svg:y="0pt" svg:width="36pt" svg:height="36pt"/></draw:g></text:p>` +
		`<draw:line text:anchor-type="page" draw:style-name="green" svg:x1="36pt" svg:y1="250pt" svg:x2="324pt" svg:y2="250pt"/>`
	buf := odtStyledFile(styles, body)
	html, _ := odtHTML(t, buf)
	for _, want := range []string{
		`Box <span><img src="ivy-object:shape1" alt="" style="width: 36pt; height: 18pt"/></span> after`,
		`<span style="display: block; margin-left: 36pt; width: 108pt"><img src="ivy-object:shape2" alt="" style="display: block; width: 108pt; height: 36pt"/></span>`,
		// the box of the line takes the half of its width of 4 points
		// around it, and the body starts 36.9 points down the page
		`<span style="position: absolute; top: 211.1pt; left: -2pt; width: 292pt"><img src="ivy-object:shape3" alt="" style="display: block; width: 292pt; height: 4pt"/></span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the xhtml to hold %q", want)
		}
	}
	if t.Failed() {
		t.Log(html)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if box, n := odtColorBox(img, color.NRGBA{R: 255, A: 255}); n < 500 || box.Dx() > 38 || box.Dy() > 20 {
		t.Errorf("expected a red box of 36 by 18 points, got %d pixels in %v", n, box)
	}
	// the ellipse and the square of the group, side by side from 72 points
	if box, n := odtColorBox(img, color.NRGBA{B: 255, A: 255}); n < 1500 || box.Min.X < 70 || box.Max.X > 182 || box.Dy() > 38 {
		t.Errorf("expected the blue group from 72 to 180 points, got %d pixels in %v", n, box)
	}
	if box, n := odtColorBox(img, color.NRGBA{G: 255, A: 255}); n < 500 || box.Min.X > 38 || box.Max.X < 322 || box.Min.Y < 246 || box.Max.Y > 254 {
		t.Errorf("expected the green line at 250 points, got %d pixels in %v", n, box)
	}
}

func TestTextEastAsian(t *testing.T) {
	const styles = `<style:style style:name="Japanese" style:family="paragraph"><style:text-properties style:font-name="Liberation Sans" style:font-name-asian="Mincho" style:language-asian="ja" style:country-asian="JP"/></style:style>` +
		`<style:style style:name="Chinese" style:family="text"><style:text-properties style:font-family-asian="SimSun" style:language-asian="zh" style:country-asian="CN"/></style:style>`
	body := `<text:p text:style-name="Japanese">Latin 日本語<text:span text:style-name="Chinese">中文</text:span></text:p>`
	// the font face Mincho names the family MS Mincho
	stylesXML := strings.NewReplacer(
		`</office:font-face-decls>`, `<style:font-face style:name="Mincho" svg:font-family="'MS Mincho'"/></office:font-face-decls>`,
		`</office:styles>`, styles+`</office:styles>`,
	).Replace(odtStylesXML)
	html, _ := odtHTML(t, fixture.Zip(
		fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "text")},
		fixture.Entry{Name: "content.xml", Data: []byte(odtContent(body))},
		fixture.Entry{Name: "styles.xml", Data: []byte(stylesXML)},
	))
	for _, want := range []string{
		`font-family: &quot;Liberation Sans&quot;, &quot;MS Mincho&quot;, sans-serif`,
		`><span lang="ja-JP">Latin 日本語</span><span style="font-family: &quot;Liberation Sans&quot;, &quot;SimSun&quot;, sans-serif"><span lang="zh-CN">中文</span></span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the xhtml to hold %q", want)
		}
	}
	if t.Failed() {
		t.Log(html)
	}
}

func TestTextTallHeader(t *testing.T) {
	// The header of the master page holds a picture of 72 points, which
	// the estimate of one line for each paragraph does not count.
	styles := strings.Replace(odtStylesXML, `<text:p text:style-name="MP1">Page <text:page-number>1</text:page-number> of <text:page-count>1</text:page-count></text:p>`,
		`<text:p><draw:frame text:anchor-type="as-char" svg:width="72pt" svg:height="72pt"><draw:image xlink:href="Pictures/red.png"/></draw:frame></text:p>`, 1)
	buf := fixture.Zip(
		fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "text")},
		fixture.Entry{Name: "content.xml", Data: []byte(odtContent(`<text:p>Body</text:p>`))},
		fixture.Entry{Name: "styles.xml", Data: []byte(styles)},
		fixture.Entry{Name: "Pictures/red.png", Data: fixture.PNG(40, 20, color.NRGBA{R: 255, A: 255})},
	)
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	red, _ := odtColorBox(img, color.NRGBA{R: 255, A: 255})
	if red.Min.Y < 17 || red.Max.Y > 91 {
		t.Errorf("expected the picture of the header from 18 to 90 points, got: %v", red)
	}
	// the body starts below the header and its spacing of 7.2 points
	if body := odtInk(img, image.Rect(0, red.Max.Y, 360, 288)); body.Empty() || body.Min.Y < 97 {
		t.Errorf("expected the body below 97 points, got: %v", body)
	}
}

func TestTextTabStops(t *testing.T) {
	const styles = `<style:style style:name="TOC" style:family="paragraph"><style:paragraph-properties fo:margin-left="0.5in">` +
		`<style:tab-stops><style:tab-stop style:position="1in"/><style:tab-stop style:position="3.5in" style:type="right" style:leader-style="dotted" style:leader-text="."/></style:tab-stops>` +
		`</style:paragraph-properties></style:style>` +
		`<style:style style:name="TOC2" style:family="paragraph" style:parent-style-name="TOC"><style:paragraph-properties>` +
		`<style:tab-stops><style:tab-stop style:position="2cm" style:type="char" style:char="."/><style:tab-stop style:position="3in" style:type="center" style:leader-style="solid"/></style:tab-stops>` +
		`</style:paragraph-properties></style:style>`
	body := `<text:p text:style-name="TOC">1<text:tab/>Chapter<text:tab/>5</text:p>` +
		`<text:p text:style-name="TOC2">x<text:tab/>y</text:p><text:p>plain</text:p>`
	buf := odtStyledFile(styles, body)
	html, _ := odtHTML(t, buf)
	for _, want := range []string{
		// the stops are from the left margin of the paragraph
		`-ivy-tab-stops: 108pt left, 288pt right &quot;.&quot;">1` + "\t" + `Chapter` + "\t" + `5</p>`,
		`-ivy-tab-stops: 92.69pt decimal, 252pt center &quot;_&quot;">x`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the xhtml to hold %q", want)
		}
	}
	if strings.Count(html, "-ivy-tab-stops") != 2 {
		t.Errorf("expected the tab stops on 2 paragraphs, got %d", strings.Count(html, "-ivy-tab-stops"))
	}
	if t.Failed() {
		t.Log(html)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	// The page has a margin of 36 points. The number 5 ends at the stop
	// at 288 points from the margin, and the dots of the leader fill the
	// space before it.
	line := odtInk(img, image.Rect(0, 37, 360, 52))
	if line.Max.X < 320 || line.Max.X > 326 {
		t.Errorf("expected the line to end at the right stop at 324 points, got: %v", line)
	}
	if dots := odtInk(img, image.Rect(230, 37, 310, 52)); dots.Empty() {
		t.Error("expected the dots of the leader before the right stop")
	}
}

// TestTextListTabStop checks the labels of the lists of
// libreoffice/odt/landscape-a4-table.odt: the text after a label starts at
// the tab stop of the level, a label aligned at the end ends at the start
// of the line, an indent of the automatic style of the paragraph replaces
// the indent of the level, and a paragraph of the same style as the next
// one has no spacing below.
func TestTextListTabStop(t *testing.T) {
	const lists = `<text:list-style style:name="L1">` +
		`<text:list-level-style-number text:level="1" style:num-format="1"><style:list-level-properties text:list-level-position-and-space-mode="label-alignment"><style:list-level-label-alignment text:label-followed-by="listtab" text:list-tab-stop-position="36pt" fo:text-indent="-18pt" fo:margin-left="18pt"/></style:list-level-properties></text:list-level-style-number>` +
		`</text:list-style><text:list-style style:name="L2">` +
		`<text:list-level-style-number text:level="1" style:num-format="i"><style:list-level-properties text:list-level-position-and-space-mode="label-alignment" fo:text-align="end"><style:list-level-label-alignment text:label-followed-by="listtab" text:list-tab-stop-position="36pt" fo:text-indent="-18pt" fo:margin-left="18pt"/></style:list-level-properties></text:list-level-style-number>` +
		`</text:list-style>`
	const auto = `<style:style style:name="PI" style:family="paragraph" style:parent-style-name="Body"><style:paragraph-properties fo:margin-left="18pt" fo:text-indent="-11pt"/></style:style>` +
		`<style:style style:name="PC" style:family="paragraph" style:parent-style-name="Body"><style:paragraph-properties fo:margin-bottom="6pt" style:contextual-spacing="true"/></style:style>`
	body := `<text:list text:style-name="L1"><text:list-item><text:p>Head</text:p></text:list-item></text:list>` +
		`<text:list text:style-name="L2"><text:list-item><text:p text:style-name="PI">Item</text:p></text:list-item></text:list>` +
		`<text:p text:style-name="PC">One</text:p><text:p text:style-name="PC">Two</text:p><text:p>Three</text:p>`
	content := strings.Replace(odtContent(body), "</office:automatic-styles>", auto+"</office:automatic-styles>", 1)
	buf := fixture.Zip(
		fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "text")},
		fixture.Entry{Name: "content.xml", Data: []byte(content)},
		fixture.Entry{Name: "styles.xml", Data: []byte(strings.Replace(odtStylesXML, "</office:styles>", lists+"</office:styles>", 1))},
	)
	html, _ := odtHTML(t, buf)
	for _, want := range []string{
		// the label starts at 0, and the text at the stop of 36 points
		`<span style="display: inline-block; min-width: 36pt; text-indent: 0`,
		// the paragraph keeps its own indent, and the label ends at 7
		// points, where the line starts
		`margin-left: 18pt; text-indent: -11pt`,
		`<span style="display: inline-block; width: 0; text-indent: 0"><span style="display: inline-block; transform: translateX(-100%)`,
		`<span style="display: inline-block; width: 29pt"></span>Item`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the html", want)
		}
	}
	one := html[strings.Index(html, "One")-400 : strings.Index(html, "One")]
	if strings.Contains(one[strings.LastIndex(one, "<p "):], "margin-bottom") {
		t.Errorf("expected no spacing below the paragraph before one of the same style:\n%s", one)
	}
	two := html[strings.Index(html, "Two")-400 : strings.Index(html, "Two")]
	if !strings.Contains(two[strings.LastIndex(two, "<p "):], "margin-bottom: 6pt") {
		t.Errorf("expected the spacing below the last paragraph of the style:\n%s", two)
	}
	if t.Failed() {
		t.Log(html)
	}
}
