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
		`@page m1 { size: 360pt 288pt; margin: 36.9pt 36pt 36pt 36pt; @top-center { content: element(hf1); padding-top: 18pt }`,
		`@page m2 { size: 432pt 288pt; margin: 36pt 36pt 36pt 36pt;`,
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

func FuzzText(f *testing.F) {
	f.Add(odtFile(odtRichBody))
	f.Add(odtFile(`<text:list><text:list-item><text:list><text:list-item><text:p>x</text:p></text:list-item></text:list></text:list-item></text:list>`))
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
