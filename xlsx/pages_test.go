package xlsx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/layout"
)

// testStyles are the styles of the test workbook: the font Calibri of 11
// points, a bold font in the color accent1 of the theme, a solid fill, a
// pattern fill, a thin border, and the formats of a date, a percentage and
// a negative number in red.
const testStyles = `<numFmts count="2"><numFmt numFmtId="164" formatCode="yyyy-mm-dd"/>` +
	`<numFmt numFmtId="165" formatCode="#,##0.00;[Red]-#,##0.00"/></numFmts>` +
	`<fonts><font><sz val="11"/><name val="Calibri"/><family val="2"/></font>` +
	`<font><b/><sz val="14"/><color theme="4"/><name val="Calibri"/></font></fonts>` +
	`<fills><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill>` +
	`<fill><patternFill patternType="solid"><fgColor rgb="FFFFFF00"/></patternFill></fill>` +
	`<fill><patternFill patternType="solid"><fgColor theme="4" tint="0.79998168889431442"/></patternFill></fill></fills>` +
	`<borders><border/><border><left style="thin"/><right style="thin"/><top style="thin"/><bottom style="medium"><color indexed="10"/></bottom></border></borders>` +
	`<cellXfs><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>` +
	`<xf numFmtId="0" fontId="1" fillId="3" borderId="1"><alignment horizontal="center" vertical="center"/></xf>` +
	`<xf numFmtId="164" fontId="0" fillId="0" borderId="0"/>` +
	`<xf numFmtId="10" fontId="0" fillId="2" borderId="0"/>` +
	`<xf numFmtId="165" fontId="0" fillId="0" borderId="0"/>` +
	`<xf numFmtId="0" fontId="0" fillId="0" borderId="0"><alignment wrapText="1" vertical="top" textRotation="90"/></xf>` +
	`</cellXfs>`

// testBook returns the test workbook.
func testBook() fixture.XLSXBook {
	return fixture.XLSXBook{
		Styles:  testStyles,
		Strings: []string{"Title", "Name", "Amount", "a & b"},
		Sheets: []fixture.XLSXSheet{
			{Name: "Report", Body: `<sheetFormatPr defaultRowHeight="15"/>` +
				`<cols><col min="1" max="1" width="20.7109375" customWidth="1"/><col min="3" max="3" width="5" hidden="1"/></cols>` +
				`<sheetData>` +
				`<row r="1" ht="24" customHeight="1"><c r="A1" s="1" t="s"><v>0</v></c></row>` +
				`<row r="2"><c r="A2" t="s"><v>1</v></c><c r="B2" t="s"><v>2</v></c><c r="C2" t="inlineStr"><is><t>hidden</t></is></c></row>` +
				`<row r="3"><c r="A3" t="s"><v>3</v></c><c r="B3" s="4"><v>-1234.5</v></c></row>` +
				`<row r="4" hidden="1"><c r="A4" t="inlineStr"><is><t>secret</t></is></c></row>` +
				`<row r="5"><c r="A5" s="2"><v>45000</v></c><c r="B5" s="3"><v>0.256</v></c><c r="D5" t="b"><v>1</v></c><c r="E5" t="e"><v>#DIV/0!</v></c></row>` +
				`<row r="6"><c r="A6" t="str"><f>A1</f><v>formula text</v></c><c r="B6"><f>1+1</f><v>2</v></c></row>` +
				`</sheetData>` +
				`<mergeCells count="1"><mergeCell ref="A1:B1"/></mergeCells>` +
				`<pageMargins left="0.5" right="0.5" top="1" bottom="1" header="0.4" footer="0.4"/>` +
				`<pageSetup paperSize="9" orientation="landscape"/>` +
				`<headerFooter><oddHeader>&amp;L&amp;"Arial,Bold"&amp;12&amp;A&amp;RPage &amp;P of &amp;N</oddHeader></headerFooter>`},
			{Name: "Hidden", State: "hidden", Body: `<sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>no</t></is></c></row></sheetData>`},
			{Name: "Second", Body: `<sheetData><row r="1"><c r="A1"><v>3.14159</v></c></row></sheetData>`},
		},
	}
}

// open opens buf with the configuration cfg.
func open(t *testing.T, buf []byte, cfg ivy.Config) *Document {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	})
	return d
}

// sourceOf returns the source of the layout of d and the text of each of
// its documents.
func sourceOf(t *testing.T, d *Document) (layout.Source, []string) {
	t.Helper()
	src, err := d.source(t.Context())
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
	return src, docs
}

// officeFonts returns the folder of the module github.com/xo/ivy/fonts, or
// nil when it is missing.
func officeFonts() fs.FS {
	dir := filepath.Join("..", "fonts")
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	return os.DirFS(dir)
}

func TestWorkbookHTML(t *testing.T) {
	d := open(t, fixture.XLSX(testBook()), ivy.Config{})
	_, docs := sourceOf(t, d)
	if len(docs) != 2 {
		t.Fatalf("expected 2 sheets without the hidden one, got: %d", len(docs))
	}
	doc := docs[0]
	for _, want := range []string{
		// A4 in landscape, with the margins of the sheet
		"@page{size:841.89pt 595.28pt;margin:72pt 36pt 72pt 36pt;",
		`@top-left{content:"Report";vertical-align:top;padding-top:28.8pt;white-space:pre;font-family:"Arial", sans-serif;font-size:12pt;font-weight:bold;}`,
		`@top-right{content:"Page " counter(page) " of " counter(pages);`,
		// 20.71 characters with their padding, of 7 pixels: 145 pixels,
		// and the default of 64 pixels. The hidden column C is left out.
		`<col style="width:108.75pt"><col style="width:48pt"><col style="width:48pt"><col style="width:48pt">`,
		`<tr style="height:24pt"><td class="s1 c" colspan="2">Title</td>`,
		`<td class="c">a &amp; b</td><td class="s4 n c" style="color:#ff0000">-1,234.50</td>`,
		`<td class="s2 n c">2023-03-15</td><td class="s3 n c">25.60%</td><td class="x c">TRUE</td><td class="x c">#DIV/0!</td>`,
		`<td class="c">formula text</td><td class="n c">2</td>`,
		// the fill of accent1 with its tint, and the border of index 10
		"background-color:#dae3f3;",
		"border-bottom:1.5pt solid #ff0000;",
		"td.s1{font-family:\"Calibri\", sans-serif;font-size:14pt;font-weight:bold;color:#4472c4;",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the sheet:\n%s", want, doc)
		}
	}
	for _, hidden := range []string{"secret", "hidden"} {
		if strings.Contains(doc, ">"+hidden+"<") {
			t.Errorf("expected the hidden cell %q left out", hidden)
		}
	}
	if !strings.Contains(docs[1], `<td class="n c">3.14159</td>`) {
		t.Errorf("expected the number of the second sheet:\n%s", docs[1])
	}
}

func TestPages(t *testing.T) {
	d := open(t, fixture.XLSX(testBook()), ivy.Config{Fonts: officeFonts()})
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 pages, one for each sheet, got: %d", n)
	}
	page, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := page.Size(); int(w) != 841 || int(h) != 595 {
		t.Errorf("expected a page of A4 in landscape, got: %vx%v", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), page, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("IVY_TEST_OUT"); out != "" {
		writePNG(t, filepath.Join(out, "xlsx-0.png"), img)
	}
	// The merged title spans A1 and B1, 156.75 points from the margin of
	// 36 points: its fill runs across both columns.
	fill := color.NRGBA{0xda, 0xe3, 0xf3, 255}
	for _, x := range []int{40, 188} {
		if got := nrgba(img.At(x, 80)); !near(got, fill) {
			t.Errorf("expected the fill of the merged cell at (%d, 80), got: %v", x, got)
		}
	}
	if got := nrgba(img.At(196, 80)); near(got, fill) {
		t.Errorf("expected no fill past the merged cell, got: %v", got)
	}
	if !hasInk(img, image.Rect(36, 0, 200, 72)) || !hasInk(img, image.Rect(600, 0, 806, 72)) {
		t.Error("expected the header in the top margin")
	}
	if _, err := d.Page(t.Context(), 2); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ErrPageRange, got: %v", err)
	}
}

func TestBandsAndTitles(t *testing.T) {
	var rows strings.Builder
	for r := 1; r <= 120; r++ {
		fmt.Fprintf(&rows, `<row r="%d">`, r)
		for c := range 12 {
			fmt.Fprintf(&rows, `<c r="%c%d"><v>%d</v></c>`, 'A'+c, r, r*100+c)
		}
		rows.WriteString("</row>")
	}
	book := fixture.XLSXBook{
		Sheets: []fixture.XLSXSheet{{Name: "Wide", Body: `<cols><col min="1" max="12" width="20" customWidth="1"/></cols>` +
			"<sheetData>" + rows.String() + "</sheetData>" +
			`<rowBreaks count="1"><brk id="60" max="16383" man="1"/></rowBreaks>`}},
		Names: `<definedName name="_xlnm.Print_Titles" localSheetId="0">Wide!$A:$A,Wide!$1:$1</definedName>`,
	}
	d := open(t, fixture.XLSX(book), ivy.Config{Fonts: officeFonts()})
	_, docs := sourceOf(t, d)
	doc := docs[0]
	// The columns of 145 pixels, 108.75 points: 4 fit in the 511.2 points
	// of a letter page. The title column repeats in the next bands.
	if n := strings.Count(doc, `<table class="sh"`); n != 4 {
		t.Errorf("expected 4 bands, got: %d", n)
	}
	if n := strings.Count(doc, "<thead>"); n != 4 {
		t.Errorf("expected the title row in each band, got: %d", n)
	}
	if !strings.Contains(doc, `<tr class="pb">`) {
		t.Error("expected the manual break of row 61")
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// A page of 684 points holds the title row and 44 rows of 15 points.
	// The 59 rows before the break take 2 pages, and the 60 rows after it
	// take 2 more, in each band.
	if n != 16 {
		t.Errorf("expected 16 pages, got: %d", n)
	}
}

func TestFitToPage(t *testing.T) {
	var rows strings.Builder
	for r := 1; r <= 100; r++ {
		fmt.Fprintf(&rows, `<row r="%d"><c r="A%d"><v>%d</v></c><c r="P%d"><v>1</v></c></row>`, r, r, r, r)
	}
	book := fixture.XLSXBook{Sheets: []fixture.XLSXSheet{{Name: "Fit", Body: `<sheetPr><pageSetUpPr fitToPage="1"/></sheetPr>` +
		"<sheetData>" + rows.String() + "</sheetData>" + `<pageSetup fitToWidth="1" fitToHeight="1"/>`}}}
	d := open(t, fixture.XLSX(book), ivy.Config{Fonts: officeFonts()})
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected the sheet on 1 page, got: %d", n)
	}
}

func TestImageAndChart(t *testing.T) {
	drawing := `<?xml version="1.0" encoding="UTF-8"?><xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<xdr:oneCellAnchor><xdr:from><xdr:col>1</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>` +
		`<xdr:ext cx="1270000" cy="635000"/><xdr:pic><xdr:blipFill><a:blip r:embed="rId1"/></xdr:blipFill></xdr:pic><xdr:clientData/></xdr:oneCellAnchor>` +
		`<xdr:twoCellAnchor><xdr:from><xdr:col>1</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>8</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>` +
		`<xdr:to><xdr:col>4</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>18</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to>` +
		`<xdr:graphicFrame><a:graphic><a:graphicData><c:chart r:id="rId2"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:twoCellAnchor>` +
		`<xdr:oneCellAnchor><xdr:from><xdr:col>6</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>` +
		`<xdr:ext cx="1270000" cy="635000"/><xdr:graphicFrame><a:graphic><a:graphicData><c:chart r:id="rId3"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:oneCellAnchor></xdr:wsDr>`
	// a part that is not a chart, but has a title, shows as a frame
	broken := `<?xml version="1.0" encoding="UTF-8"?><c:other xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><c:title><c:tx><c:rich><a:p><a:r><a:t>Sales</a:t></a:r></a:p></c:rich></c:tx></c:title></c:other>`
	book := fixture.XLSXBook{
		Sheets: []fixture.XLSXSheet{{
			Name: "Pictures",
			Body: `<sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>x</t></is></c></row></sheetData><drawing r:id="rId1"/>`,
			Rels: `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>`,
		}},
		Extra: []fixture.Entry{
			{Name: "xl/drawings/drawing1.xml", Data: []byte(drawing)},
			{Name: "xl/drawings/_rels/drawing1.xml.rels", Data: []byte(`<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
				`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image1.png"/>` +
				`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>` +
				`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart2.xml"/></Relationships>`)},
			{Name: "xl/media/image1.png", Data: fixture.PNG(4, 2, color.NRGBA{0, 0, 255, 255})},
			{Name: "xl/charts/chart1.xml", Data: fixture.Chart("FF0000")},
			{Name: "xl/charts/chart2.xml", Data: []byte(broken)},
		},
	}
	d := open(t, fixture.XLSX(book), ivy.Config{Fonts: officeFonts()})
	_, docs := sourceOf(t, d)
	doc := docs[0]
	for _, want := range []string{
		// the image covers 4 rows, and each row shows its slice
		`<td class="a"><div class="cl" style="width:144pt;height:16pt"><img style="position:absolute;left:0pt;top:0pt;width:100pt;height:50pt" src="xl/media/image1.png" alt=""></div></td>`,
		`<td class="a"><div class="cl" style="width:144pt;height:16pt"><img style="position:absolute;left:0pt;top:-45pt;width:100pt;height:50pt" src="xl/media/image1.png" alt=""></div></td>`,
		`<img style="position:absolute;left:0pt;top:0pt;width:144pt;height:150pt" src="ivy-object:chart2" alt="">`,
		`<div class="ph" style="left:0pt;top:0pt;width:100pt;height:50pt">Chart: Sales</div>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the sheet:\n%s", want, doc)
		}
	}
	page, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), page, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("IVY_TEST_OUT"); out != "" {
		writePNG(t, filepath.Join(out, "xlsx-image.png"), img)
	}
	// the image fills B2 and on: 50.4 + 48 points from the left, and
	// 54 + 15 from the top
	if got := nrgba(img.At(130, 90)); !near(got, color.NRGBA{0, 0, 255, 255}) {
		t.Errorf("expected the blue image at (130, 90), got: %v", got)
	}
	// the red columns of the chart are in its frame of 144 by 150 points
	// at B9: 50.4 + 48 points from the left, and 54 + 8 * 15 from the top
	red := 0
	for y := 174; y < 324; y++ {
		for x := 99; x < 242; x++ {
			if near(nrgba(img.At(x, y)), color.NRGBA{255, 0, 0, 255}) {
				red++
			}
		}
	}
	if red < 500 {
		t.Errorf("expected the red columns of the chart in its frame, got %d red pixels", red)
	}
}

// chartSheetBook returns a workbook of one chart sheet, whose chart has
// columns of the color rgb.
func chartSheetBook(rgb string) fixture.XLSXBook {
	drawing := `<?xml version="1.0" encoding="UTF-8"?><xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<xdr:absoluteAnchor><xdr:pos x="0" y="0"/><xdr:ext cx="1270000" cy="1270000"/>` +
		`<xdr:graphicFrame><a:graphic><a:graphicData><c:chart r:id="rId1"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:absoluteAnchor></xdr:wsDr>`
	return fixture.XLSXBook{
		Sheets: []fixture.XLSXSheet{{
			Name:  "Chart1",
			Chart: true,
			Body:  `<pageMargins left="0.5" right="0.5" top="0.5" bottom="0.5" header="0.3" footer="0.3"/><drawing r:id="rId1"/>`,
			Rels:  `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>`,
		}},
		Extra: []fixture.Entry{
			{Name: "xl/drawings/drawing1.xml", Data: []byte(drawing)},
			{Name: "xl/drawings/_rels/drawing1.xml.rels", Data: []byte(`<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
				`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/></Relationships>`)},
			{Name: "xl/charts/chart1.xml", Data: fixture.Chart(rgb)},
		},
	}
}

func TestChartSheet(t *testing.T) {
	book := chartSheetBook("0000FF")
	d := open(t, fixture.XLSX(book), ivy.Config{})
	page, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := page.Size(); w < h {
		t.Errorf("expected a landscape page, got %g by %g", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), page, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	// the chart fills the page within its margins of 36 points, and its
	// blue columns are on both halves of the page
	b := img.Bounds()
	left, right := 0, 0
	for y := 36; y < b.Dy()-36; y++ {
		for x := 36; x < b.Dx()-36; x++ {
			if near(nrgba(img.At(x, y)), color.NRGBA{0, 0, 255, 255}) {
				if x < b.Dx()/2 {
					left++
				} else {
					right++
				}
			}
		}
	}
	if left < 1000 || right < 1000 {
		t.Errorf("expected the blue columns of the chart across the page, got %d and %d blue pixels", left, right)
	}
}

func TestNoStyles(t *testing.T) {
	book := fixture.XLSXBook{Sheets: []fixture.XLSXSheet{{Name: "Plain", Body: `<sheetData><row r="2"><c r="B2"><v>1.5</v></c><c r="C2" t="s"><v>9</v></c></row></sheetData>`}}}
	d := open(t, fixture.XLSX(book), ivy.Config{})
	_, docs := sourceOf(t, d)
	if !strings.Contains(docs[0], `<td class="n c">1.5</td>`) {
		t.Errorf("expected the number with General:\n%s", docs[0])
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 1 {
		t.Errorf("expected 1 page, got: %d, %v", n, err)
	}
}

func TestEmptyWorkbook(t *testing.T) {
	book := fixture.XLSXBook{Sheets: []fixture.XLSXSheet{{Name: "Empty", Body: `<sheetData/>`}}}
	d := open(t, fixture.XLSX(book), ivy.Config{})
	if n, err := d.PageCount(t.Context()); err != nil || n != 1 {
		t.Errorf("expected 1 empty page, got: %d, %v", n, err)
	}
}

func TestLimits(t *testing.T) {
	var rows strings.Builder
	for r := 1; r <= 400; r++ {
		fmt.Fprintf(&rows, `<row r="%d"><c r="A%d"><v>%d</v></c></row>`, r, r, r)
	}
	book := fixture.XLSXBook{Sheets: []fixture.XLSXSheet{{Name: "Long", Body: "<sheetData>" + rows.String() + "</sheetData>"}}}
	d := open(t, fixture.XLSX(book), ivy.Config{Limits: ivy.Limits{MaxObjects: 4000}})
	_, docs := sourceOf(t, d)
	if !strings.Contains(docs[0], "past the limit of 1000 cells") {
		t.Errorf("expected the note of the limit:\n%s", docs[0])
	}
	if _, err := d.PageCount(t.Context()); err != nil {
		t.Error(err)
	}
	// a sheet with more nodes than the limit is left out with a line
	d = open(t, fixture.XLSX(book), ivy.Config{Limits: ivy.Limits{MaxObjects: 500}})
	_, docs = sourceOf(t, d)
	if !strings.Contains(docs[0], "ivy cannot show this sheet") {
		t.Errorf("expected the line of the sheet that ivy cannot read:\n%s", docs[0])
	}
}

func TestCanceled(t *testing.T) {
	d := open(t, fixture.XLSX(testBook()), ivy.Config{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := d.PageCount(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	// the error of the context is not kept
	if n, err := d.PageCount(t.Context()); err != nil || n != 2 {
		t.Errorf("expected 2 pages after the cancel, got: %d, %v", n, err)
	}
}

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		in       string
		row, col int
		ok       bool
	}{
		{"A1", 0, 0, true}, {"$B$12", 11, 1, true}, {"XFD1048576", 1048575, 16383, true},
		{"XFE1", 0, 0, false}, {"A0", 0, 0, false}, {"1A", 0, 0, false}, {"", 0, 0, false},
	} {
		row, col, ok := parseRef(tt.in)
		if ok != tt.ok || (ok && (row != tt.row || col != tt.col)) {
			t.Errorf("parseRef(%q): expected %d %d %v, got: %d %d %v", tt.in, tt.row, tt.col, tt.ok, row, col, ok)
		}
	}
	if got := splitNames(`'My, Sheet'!$A$1:$B$2,Sheet2!$C:$D`); len(got) != 2 || got[0] != "$A$1:$B$2" || got[1] != "$C:$D" {
		t.Errorf("unexpected split: %q", got)
	}
	if got := decodeEscapes("a_x000D_b_x0009_c"); got != "ab\tc" {
		t.Errorf("unexpected escapes: %q", got)
	}
}
