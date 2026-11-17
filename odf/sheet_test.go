package odf

import (
	"bytes"
	"context"
	"errors"
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

// sheetMime is the mime type of a spreadsheet.
const sheetMime = "application/vnd.oasis.opendocument.spreadsheet"

// testODS is a spreadsheet with a page layout of A4 in landscape, a header
// with the name of the sheet and a footer with the page number, styles of
// the columns, the rows and the cells, repeated rows and columns, a merged
// cell, a hidden row, and the long empty runs that LibreOffice writes to
// the end of a sheet.
var testODS = fixture.ODSBook{
	Styles: `<style:default-style style:family="table-cell"><style:text-properties style:font-name="Liberation Sans" fo:font-size="10pt"/></style:default-style>` +
		`<style:style style:name="Default" style:family="table-cell"/>` +
		`<style:style style:name="Heading" style:family="table-cell" style:parent-style-name="Default"><style:text-properties fo:font-weight="bold" fo:font-size="14pt"/></style:style>`,
	AutoStyles: `<style:page-layout style:name="pm1"><style:page-layout-properties fo:page-width="11.693in" fo:page-height="8.268in" style:print-orientation="landscape" ` +
		`fo:margin-top="0.5in" fo:margin-bottom="0.5in" fo:margin-left="0.75in" fo:margin-right="0.75in" style:print="grid charts"/>` +
		`<style:header-style><style:header-footer-properties fo:min-height="0.1in" fo:margin-bottom="0.1in"/></style:header-style>` +
		`<style:footer-style><style:header-footer-properties fo:min-height="0.1in" fo:margin-top="0.1in"/></style:footer-style></style:page-layout>`,
	MasterStyles: `<style:master-page style:name="Default" style:page-layout-name="pm1">` +
		`<style:header><style:region-left><text:p><text:sheet-name>???</text:sheet-name></text:p></style:region-left></style:header>` +
		`<style:footer><text:p>Page <text:page-number>1</text:page-number></text:p></style:footer></style:master-page>`,
	ContentStyles: `<style:style style:name="co1" style:family="table-column"><style:table-column-properties style:column-width="1.5in"/></style:style>` +
		`<style:style style:name="co2" style:family="table-column"><style:table-column-properties style:column-width="0.75in"/></style:style>` +
		`<style:style style:name="ro1" style:family="table-row"><style:table-row-properties style:row-height="0.25in"/></style:style>` +
		`<style:style style:name="ce1" style:family="table-cell" style:parent-style-name="Heading"><style:table-cell-properties fo:background-color="#ffcc00" fo:border="0.06pt solid #000000"/>` +
		`<style:paragraph-properties fo:text-align="center"/></style:style>` +
		`<style:style style:name="ce2" style:family="table-cell"><style:table-cell-properties fo:wrap-option="wrap"/></style:style>` +
		`<style:style style:name="T1" style:family="text"><style:text-properties fo:color="#ff0000" fo:font-style="italic"/></style:style>`,
	Body: `<table:table table:name="Budget">` +
		`<table:table-column table:style-name="co1"/><table:table-column table:style-name="co2" table:number-columns-repeated="2"/>` +
		`<table:table-column table:number-columns-repeated="16381"/>` +
		`<table:table-row table:style-name="ro1"><table:table-cell table:style-name="ce1" table:number-columns-spanned="3" office:value-type="string"><text:p>Budget 2026</text:p></table:table-cell>` +
		`<table:covered-table-cell table:number-columns-repeated="2"/><table:table-cell table:number-columns-repeated="16381"/></table:table-row>` +
		`<table:table-row table:number-rows-repeated="2"><table:table-cell office:value-type="string"><text:p>Item</text:p></table:table-cell>` +
		`<table:table-cell office:value-type="float" office:value="1250.5"><text:p>1,250.50</text:p></table:table-cell><table:table-cell table:number-columns-repeated="16382"/></table:table-row>` +
		`<table:table-row table:visibility="collapse"><table:table-cell office:value-type="string"><text:p>secret</text:p></table:table-cell></table:table-row>` +
		`<table:table-row><table:table-cell table:style-name="ce2" office:value-type="string"><text:p>a <text:span text:style-name="T1">red</text:span> word</text:p></table:table-cell>` +
		`<table:table-cell office:value-type="boolean" office:boolean-value="true"><text:p>TRUE</text:p></table:table-cell>` +
		`<table:table-cell office:value-type="percentage" office:value="0.5"/></table:table-row>` +
		`<table:table-row table:number-rows-repeated="1048570"><table:table-cell table:number-columns-repeated="16384"/></table:table-row>` +
		`</table:table>` +
		`<table:table table:name="Hidden" table:print="false"><table:table-row><table:table-cell office:value-type="string"><text:p>no</text:p></table:table-cell></table:table-row></table:table>` +
		`<table:table table:name="Notes"><table:table-row><table:table-cell office:value-type="string"><text:p>second</text:p></table:table-cell></table:table-row></table:table>`,
}

// openSheet opens a spreadsheet with the configuration cfg.
func openSheet(t *testing.T, buf []byte, cfg ivy.Config) *Document {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// sheetDocs returns the HTML documents of the spreadsheet d.
func sheetDocs(t *testing.T, d *Document) []string {
	t.Helper()
	p, err := d.pagerOf()
	if err != nil {
		t.Fatal(err)
	}
	src, err := p.(*sheetPager).source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	docs := make([]string, 0, len(src.Documents))
	for _, name := range src.Documents {
		buf, err := fs.ReadFile(src.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, string(buf))
	}
	return docs
}

// odsFonts returns the folder of the module github.com/xo/ivy/fonts, or
// nil when it is missing.
func odsFonts() fs.FS {
	dir := filepath.Join("..", "fonts")
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	return os.DirFS(dir)
}

func TestSheetHTML(t *testing.T) {
	d := openSheet(t, fixture.ODS(testODS), ivy.Config{})
	docs := sheetDocs(t, d)
	if len(docs) != 2 {
		t.Fatalf("expected 2 sheets without the one that does not print, got: %d", len(docs))
	}
	doc := docs[0]
	for _, want := range []string{
		// A4 in landscape. The header and the footer take a line of 10
		// points and their spacing of 0.1 inch from the area of the cells.
		"@page{size:841.9pt 595.3pt;margin:53.2pt 54pt 53.2pt 54pt;",
		`@top-left{content:"Budget";vertical-align:top;padding-top:36pt;`,
		`@bottom-center{content:"Page " counter(page);vertical-align:bottom;`,
		`<col style="width:108pt"><col style="width:54pt"><col style="width:54pt">`,
		`<tr style="height:18pt"><td class="s1 c" colspan="3">Budget 2026</td></tr>`,
		`<td class="c">Item</td><td class="n c">1,250.50</td><td></td></tr><tr><td class="c">Item</td><td class="n c">1,250.50</td>`,
		`<td class="s2 c">a <span style="font-family:&#34;Liberation Sans&#34;, sans-serif;font-size:10pt;font-style:italic;color:#ff0000;">red</span> word</td>`,
		`<td class="n c">TRUE</td><td class="n c">0.5</td>`,
		"td.s1{font-family:\"Liberation Sans\", sans-serif;font-size:14pt;font-weight:bold;background-color:#ffcc00;border-top:0.5pt solid #000000;",
		"border:0.5pt solid #c8c8c8",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the sheet:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "secret") {
		t.Error("expected the hidden row left out")
	}
	if n := strings.Count(doc, "<tr"); n != 4 {
		t.Errorf("expected 4 rows, got: %d", n)
	}
}

func TestSheetPages(t *testing.T) {
	d := openSheet(t, fixture.ODS(testODS), ivy.Config{Fonts: odsFonts()})
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 pages, got: %d", n)
	}
	page, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := page.Size(); int(w+0.5) != 842 || int(h+0.5) != 595 {
		t.Errorf("expected A4 in landscape, got: %vx%v", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), page, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("IVY_TEST_OUT"); out != "" {
		var buf bytes.Buffer
		if err := odsPNG(&buf, img); err == nil {
			_ = os.WriteFile(filepath.Join(out, "ods-0.png"), buf.Bytes(), 0o600)
		}
	}
	// the fill of the merged title runs across the three columns, from
	// 54 points to 270
	for _, x := range []int{60, 260} {
		c := color.NRGBAModel.Convert(img.At(x, 60)).(color.NRGBA)
		if c.R < 240 || c.G < 190 || c.G > 220 || c.B > 20 {
			t.Errorf("expected the fill of the title at (%d, 60), got: %v", x, c)
		}
	}
	if !odsInked(img, image.Rect(54, 30, 200, 50)) {
		t.Error("expected the header with the name of the sheet")
	}
	if !odsInked(img, image.Rect(300, 545, 540, 575)) {
		t.Error("expected the footer with the page number")
	}
}

func TestSheetLibreOffice(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "sheet.ods"))
	if err != nil {
		t.Fatal(err)
	}
	d := openSheet(t, buf, ivy.Config{})
	docs := sheetDocs(t, d)
	if len(docs) != 1 || !strings.Contains(docs[0], `<td class="n c">1</td><td class="n c">2</td>`) {
		t.Errorf("expected the cells of sheet.ods:\n%s", docs)
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 1 {
		t.Errorf("expected 1 page, got: %d, %v", n, err)
	}
}

func TestSheetEncrypted(t *testing.T) {
	book := fixture.ODS(testODS)
	entries := make([]fixture.Entry, 0, 2)
	for _, name := range []string{"content.xml", "styles.xml"} {
		entries = append(entries, fixture.Entry{Name: name, Data: odsEntry(t, book, name)})
	}
	enc := fixture.EncryptODF(fixture.ODFEncryption{Password: "secret", Cipher: fixture.ODFAESCBC, Iterations: 10}, sheetMime, entries...)
	d := openSheet(t, enc, ivy.Config{Password: "secret"})
	if n, err := d.PageCount(t.Context()); err != nil || n != 2 {
		t.Errorf("expected 2 pages of the encrypted file, got: %d, %v", n, err)
	}
}

// odsEntry returns the entry name of the zip archive buf.
func odsEntry(t *testing.T, buf []byte, name string) []byte {
	t.Helper()
	d := openSheet(t, buf, ivy.Config{})
	data, err := d.readEntry(t.Context(), name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSheetLimits(t *testing.T) {
	var body strings.Builder
	body.WriteString(`<table:table table:name="Long">`)
	for range 300 {
		body.WriteString(`<table:table-row><table:table-cell office:value-type="string"><text:p>x</text:p></table:table-cell></table:table-row>`)
	}
	// a row of text repeated a million times stops at the limit
	body.WriteString(`<table:table-row table:number-rows-repeated="1000000"><table:table-cell office:value-type="string"><text:p>y</text:p></table:table-cell></table:table-row>`)
	body.WriteString(`</table:table>`)
	d := openSheet(t, fixture.ODS(fixture.ODSBook{Body: body.String()}), ivy.Config{Limits: ivy.Limits{MaxObjects: 20000}})
	docs := sheetDocs(t, d)
	if !strings.Contains(docs[0], "past the limit of 5000 cells") {
		t.Error("expected the note of the limit of the sheet")
	}
	if !strings.Contains(docs[0], "more cells than the limit of ivy") {
		t.Error("expected the note of the limit of the workbook")
	}
	if _, err := d.PageCount(t.Context()); err != nil {
		t.Error(err)
	}
}

func TestSheetPaper(t *testing.T) {
	landscape := ivy.A4.Landscape()
	body := `<table:table table:name="Paper"><table:table-row><table:table-cell office:value-type="string"><text:p>Text</text:p></table:table-cell></table:table-row></table:table>`
	book := func(props string) []byte {
		b := fixture.ODSBook{Body: body}
		if props != "" {
			b.AutoStyles = `<style:page-layout style:name="pm1"><style:page-layout-properties ` + props + `/></style:page-layout>`
			b.MasterStyles = `<style:master-page style:name="Default" style:page-layout-name="pm1"/>`
		}
		return fixture.ODS(b)
	}
	for _, test := range []struct {
		name  string
		buf   []byte
		paper ivy.Paper
		exp   ivy.Paper
	}{
		{"default", book(""), ivy.Paper{}, ivy.Letter},
		{"no master page", book(""), landscape, landscape},
		{"no size", book(`fo:margin="0.5in"`), landscape, landscape},
		{"orientation only", book(`style:print-orientation="landscape"`), ivy.A4, landscape},
		{"own size", book(`fo:page-width="8.5in" fo:page-height="14in"`), landscape, ivy.Legal},
		{"own size in landscape", fixture.ODS(testODS), ivy.Letter, ivy.Paper{Width: 841.9, Height: 595.3}},
	} {
		t.Run(test.name, func(t *testing.T) {
			d := openSheet(t, test.buf, ivy.Config{Paper: test.paper})
			fixture.CheckPaper(t, d, test.exp)
		})
	}
}

func TestSheetCanceled(t *testing.T) {
	d := openSheet(t, fixture.ODS(testODS), ivy.Config{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := d.PageCount(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 2 {
		t.Errorf("expected 2 pages after the cancel, got: %d, %v", n, err)
	}
}

func TestODSRange(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want [4]int
		ok   bool
	}{
		{"Sheet1.A1:Sheet1.D20", [4]int{0, 0, 19, 3}, true},
		{"$'My.Sheet'.$B$2:.$C$3", [4]int{1, 1, 2, 2}, true},
		{"Sheet1.A1", [4]int{0, 0, 0, 0}, true},
		{"nothing", [4]int{}, false},
	} {
		r, ok := odsRange(tt.in)
		got := [4]int{r.Row0, r.Col0, r.Row1, r.Col1}
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("odsRange(%q): expected %v %v, got: %v %v", tt.in, tt.want, tt.ok, got, ok)
		}
	}
}

func FuzzSheet(f *testing.F) {
	f.Add(fixture.ODS(testODS))
	if buf, err := os.ReadFile(filepath.Join("testdata", "sheet.ods")); err == nil {
		f.Add(buf)
	}
	f.Fuzz(func(t *testing.T, buf []byte) {
		cfg := ivy.Config{Limits: ivy.Limits{MaxObjects: 1 << 14, MaxMemory: 16 << 20}}
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		page, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		_, _ = ivy.RenderImage(t.Context(), page, 8, color.White)
	})
}

func TestSheetChart(t *testing.T) {
	book := fixture.ODSBook{
		Body: `<table:table table:name="Chart"><table:table-row><table:table-cell office:value-type="string"><text:p>x</text:p>` +
			`<draw:frame svg:width="200pt" svg:height="150pt" svg:x="0pt" svg:y="0pt"><draw:object xlink:href="./Object 1" xlink:type="simple"/>` +
			`<draw:image xlink:href="./ObjectReplacements/Object 1" xlink:type="simple"/></draw:frame></table:table-cell></table:table-row>` +
			`<table:table-row><table:table-cell office:value-type="string"><text:p>y</text:p>` +
			`<draw:frame svg:width="100pt" svg:height="50pt" svg:x="0pt" svg:y="0pt"><draw:object xlink:href="./Object 2" xlink:type="simple"/></draw:frame>` +
			`</table:table-cell></table:table-row></table:table>`,
		Extra: []fixture.Entry{
			{Name: "Object 1/content.xml", Data: fixture.ODFChart("ff0000")},
			// an object that is not a chart shows as a frame
			{Name: "Object 2/content.xml", Data: []byte(`<?xml version="1.0"?><office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"/>`)},
		},
	}
	d := openSheet(t, fixture.ODS(book), ivy.Config{})
	doc := sheetDocs(t, d)[0]
	for _, want := range []string{
		`<img style="position:absolute;left:0pt;top:0pt;width:200pt;height:150pt" src="ivy-object:chart1" alt="">`,
		`<div class="ph" style="left:0pt;top:0pt;width:100pt;height:50pt">Chart</div>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the sheet:\n%s", want, doc)
		}
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	// the red bars of the chart are in its frame of 200 by 150 points, at
	// the first cell of the page
	red, out := 0, 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.R < 200 || c.G > 60 || c.B > 60 {
				continue
			}
			red++
			if x > 300 || y > 300 {
				out++
			}
		}
	}
	if red < 1000 || out > 0 {
		t.Errorf("expected the red bars of the chart at the top left of the page, got %d red pixels, %d far from it", red, out)
	}
}
