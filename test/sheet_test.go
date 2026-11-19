package test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/odf"
	"github.com/xo/ivy/xlsx"
)

// The page of the sheets of TestXLSX and TestODS: US Letter at 96 dots per
// inch.
const (
	sheetPageWidth  = 816
	sheetPageHeight = 1056
)

// sheetTolerance is the distance in pixels at 96 dots per inch within which
// a box or a band of ivy matches the one of LibreOffice.
const sheetTolerance = 4

// widthTolerance is the share by which the width of the ink of a page of
// ivy can differ from the one of LibreOffice, for a sheet without a rule of
// its own (sheetRule).
const widthTolerance = 0.1

// sheetRule is the layout that ivy follows for a sheet whose sizes do not
// come from LibreOffice, such as an Excel file, which ivy prints as Excel
// 2024 does. LibreOffice still gives the structure of the pages: their
// number, their size, the top left corner of the ink, the bands of rows
// that are not rows of data, such as the print titles, and how far the
// text sits above the bottom of its row. The rule gives the sizes, in
// pixels at 96 dots per inch, for ivy and for LibreOffice.
type sheetRule struct {
	// top is the top of the area of the cells: the top margin.
	top float64
	// titles and row are the height of the rows of the print titles and of
	// a row of data, as ivy prints them, and refTitles and refRow as
	// LibreOffice prints them.
	titles, row       float64
	refTitles, refRow float64
	// pages holds for each page the number of rows of data that ivy and
	// LibreOffice print on it, and the width of the band of columns that
	// ivy prints.
	pages []rulePage
}

// rulePage is a page of a sheetRule.
type rulePage struct {
	rows, refRows int
	width         float64
}

// rowsPerPage returns the number of rows of data of the height row that a
// page of the height page holds under the print titles of the height
// titles.
func rowsPerPage(page, titles, row float64) int {
	return int((page - titles) / row)
}

// zipFiles returns a zip archive of the files, in order. The first file is
// stored, as the mimetype of an OpenDocument file must be.
func zipFiles(t *testing.T, files [][2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	modified := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	for i, f := range files {
		var w io.Writer
		var err error
		if i == 0 {
			// LibreOffice reads the mimetype only without a data
			// descriptor
			w, err = zw.CreateRaw(&zip.FileHeader{
				Name: f[0], Method: zip.Store, Modified: modified,
				CRC32: crc32.ChecksumIEEE([]byte(f[1])), CompressedSize64: uint64(len(f[1])), UncompressedSize64: uint64(len(f[1])),
			})
		} else {
			w, err = zw.CreateHeader(&zip.FileHeader{Name: f[0], Method: zip.Deflate, Modified: modified})
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(f[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// buildXLSX returns an Excel file of one sheet in Calibri of 11 points: a
// merged title with a fill, a row of headings with a border, then rows of
// text and numbers in eight columns, which are too wide for one page. The
// first two rows are the print titles, and the header and the footer show
// the name of the sheet and the page.
func buildXLSX(t *testing.T, rows int) []byte {
	t.Helper()
	const main = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	const rel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	var data strings.Builder
	data.WriteString(`<row r="1" ht="26" customHeight="1"><c r="A1" s="1" t="inlineStr"><is><t>Quarterly report</t></is></c>`)
	for _, c := range "BCDEFGH" {
		fmt.Fprintf(&data, `<c r="%c1" s="1"/>`, c)
	}
	data.WriteString(`</row><row r="2">`)
	for i, h := range []string{"Item", "Q1", "Q2", "Q3", "Q4", "Share", "Count", "Code"} {
		fmt.Fprintf(&data, `<c r="%c2" s="2" t="inlineStr"><is><t>%s</t></is></c>`, 'A'+i, h)
	}
	data.WriteString(`</row>`)
	for r := 3; r < rows+3; r++ {
		fmt.Fprintf(&data, `<row r="%d"><c r="A%d" t="inlineStr"><is><t>Item number %d</t></is></c>`, r, r, r-2)
		for i := range 4 {
			fmt.Fprintf(&data, `<c r="%c%d" s="3"><v>%d.%02d</v></c>`, 'B'+i, r, (r*37+i*1013)%9000, (r*13+i)%100)
		}
		fmt.Fprintf(&data, `<c r="F%d" s="4"><v>0.%02d</v></c><c r="G%d"><v>%d</v></c><c r="H%d" t="inlineStr"><is><t>X-%03d</t></is></c></row>`,
			r, r%100, r, r*3, r, r)
	}
	sheet := `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="` + main + `" xmlns:r="` + rel + `">` +
		`<sheetFormatPr defaultRowHeight="15"/>` +
		`<cols><col min="1" max="1" width="30.7109375" customWidth="1"/><col min="2" max="8" width="14.7109375" customWidth="1"/></cols>` +
		`<sheetData>` + data.String() + `</sheetData>` +
		`<mergeCells count="1"><mergeCell ref="A1:E1"/></mergeCells>` +
		`<pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/>` +
		`<pageSetup paperSize="1" orientation="portrait"/>` +
		`<headerFooter><oddHeader>&amp;C&amp;A</oddHeader><oddFooter>&amp;CPage &amp;P</oddFooter></headerFooter></worksheet>`
	styles := `<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="` + main + `">` +
		`<numFmts count="1"><numFmt numFmtId="164" formatCode="#,##0.00"/></numFmts>` +
		`<fonts count="2"><font><sz val="11"/><name val="Calibri"/><family val="2"/></font><font><b/><sz val="14"/><name val="Calibri"/><family val="2"/></font></fonts>` +
		`<fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill>` +
		`<fill><patternFill patternType="solid"><fgColor rgb="FFD9E1F2"/></patternFill></fill></fills>` +
		`<borders count="2"><border/><border><bottom style="thin"><color rgb="FF000000"/></bottom></border></borders>` +
		`<cellXfs count="5"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>` +
		`<xf numFmtId="0" fontId="1" fillId="2" borderId="0" applyAlignment="1"><alignment horizontal="center"/></xf>` +
		`<xf numFmtId="0" fontId="0" fillId="0" borderId="1"/>` +
		`<xf numFmtId="164" fontId="0" fillId="0" borderId="0"/>` +
		`<xf numFmtId="9" fontId="0" fillId="0" borderId="0"/></cellXfs></styleSheet>`
	return zipFiles(t, [][2]string{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="` + rel + `/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="` + main + `" xmlns:r="` + rel + `">` +
			`<sheets><sheet name="Data" sheetId="1" r:id="rId1"/></sheets>` +
			`<definedNames><definedName name="_xlnm.Print_Titles" localSheetId="0">Data!$1:$2</definedName></definedNames></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="` + rel + `/worksheet" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId2" Type="` + rel + `/styles" Target="styles.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", sheet},
		{"xl/styles.xml", styles},
	})
}

// odfNS declares the namespaces of OpenDocument.
const odfNS = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" ` +
	`xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" ` +
	`xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" ` +
	`xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" ` +
	`xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" ` +
	`xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" office:version="1.3"`

// buildODS returns an OpenDocument spreadsheet of one table in Liberation
// Sans of 10 points, the default of LibreOffice: a merged title with a
// fill, then rows of text and numbers in three columns, with a header and
// a footer, on US Letter.
func buildODS(t *testing.T, rows int) []byte {
	t.Helper()
	var data strings.Builder
	data.WriteString(`<table:table-row table:style-name="ro2"><table:table-cell table:style-name="ce1" table:number-columns-spanned="3" office:value-type="string"><text:p>Inventory</text:p></table:table-cell><table:covered-table-cell table:number-columns-repeated="2"/></table:table-row>`)
	for r := 1; r <= rows; r++ {
		fmt.Fprintf(&data, `<table:table-row table:style-name="ro1"><table:table-cell office:value-type="string"><text:p>Part %d</text:p></table:table-cell>`+
			`<table:table-cell office:value-type="float" office:value="%d"><text:p>%d</text:p></table:table-cell>`+
			`<table:table-cell office:value-type="string"><text:p>Shelf %c</text:p></table:table-cell></table:table-row>`, r, r*17, r*17, 'A'+r%26)
	}
	content := `<?xml version="1.0" encoding="UTF-8"?><office:document-content ` + odfNS + `>` +
		`<office:font-face-decls><style:font-face style:name="Liberation Sans" svg:font-family="'Liberation Sans'" style:font-family-generic="swiss"/></office:font-face-decls>` +
		`<office:automatic-styles>` +
		`<style:style style:name="co1" style:family="table-column"><style:table-column-properties style:column-width="2in"/></style:style>` +
		`<style:style style:name="co2" style:family="table-column"><style:table-column-properties style:column-width="1.25in"/></style:style>` +
		`<style:style style:name="ro1" style:family="table-row"><style:table-row-properties style:row-height="0.178in" style:use-optimal-row-height="false"/></style:style>` +
		`<style:style style:name="ro2" style:family="table-row"><style:table-row-properties style:row-height="0.3in" style:use-optimal-row-height="false"/></style:style>` +
		`<style:style style:name="ta1" style:family="table" style:master-page-name="Default"><style:table-properties table:display="true"/></style:style>` +
		`<style:style style:name="ce1" style:family="table-cell" style:parent-style-name="Default"><style:table-cell-properties fo:background-color="#c0c0c0"/>` +
		`<style:paragraph-properties fo:text-align="center"/><style:text-properties fo:font-weight="bold" fo:font-size="14pt"/></style:style>` +
		`</office:automatic-styles><office:body><office:spreadsheet><table:table table:name="Stock" table:style-name="ta1">` +
		`<table:table-column table:style-name="co1"/><table:table-column table:style-name="co2" table:number-columns-repeated="2"/>` +
		data.String() + `</table:table></office:spreadsheet></office:body></office:document-content>`
	styles := `<?xml version="1.0" encoding="UTF-8"?><office:document-styles ` + odfNS + `>` +
		`<office:font-face-decls><style:font-face style:name="Liberation Sans" svg:font-family="'Liberation Sans'" style:font-family-generic="swiss"/></office:font-face-decls>` +
		`<office:styles><style:default-style style:family="table-cell"><style:text-properties style:font-name="Liberation Sans" fo:font-size="10pt"/></style:default-style>` +
		`<style:style style:name="Default" style:family="table-cell"/></office:styles>` +
		`<office:automatic-styles><style:page-layout style:name="pm1"><style:page-layout-properties fo:page-width="8.5in" fo:page-height="11in" style:print-orientation="portrait" ` +
		`fo:margin-top="0.75in" fo:margin-bottom="0.75in" fo:margin-left="0.75in" fo:margin-right="0.75in" style:print="objects charts drawings zero-values"/>` +
		`<style:header-style><style:header-footer-properties fo:min-height="0.1in" fo:margin-bottom="0.1in"/></style:header-style>` +
		`<style:footer-style><style:header-footer-properties fo:min-height="0.1in" fo:margin-top="0.1in"/></style:footer-style></style:page-layout></office:automatic-styles>` +
		`<office:master-styles><style:master-page style:name="Default" style:page-layout-name="pm1">` +
		`<style:header><text:p><text:sheet-name>???</text:sheet-name></text:p></style:header>` +
		`<style:footer><text:p>Page <text:page-number>1</text:page-number></text:p></style:footer></style:master-page></office:master-styles></office:document-styles>`
	const mime = "application/vnd.oasis.opendocument.spreadsheet"
	return zipFiles(t, [][2]string{
		{"mimetype", mime},
		{"META-INF/manifest.xml", `<?xml version="1.0" encoding="UTF-8"?><manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.3">` +
			`<manifest:file-entry manifest:full-path="/" manifest:media-type="` + mime + `"/>` +
			`<manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/>` +
			`<manifest:file-entry manifest:full-path="styles.xml" manifest:media-type="text/xml"/></manifest:manifest>`},
		{"content.xml", content},
		{"styles.xml", styles},
	})
}

// sofficePDF converts the file path to PDF with LibreOffice, in a profile
// of its own in dir, and returns the path of the PDF file.
func sofficePDF(ctx context.Context, dir, path string) (string, error) {
	exe, err := exec.LookPath("soffice")
	if err != nil {
		return "", fmt.Errorf("oracle soffice is not installed: %w", err)
	}
	cmd := exec.CommandContext(ctx, exe, "--headless", "--norestore",
		"-env:UserInstallation=file://"+filepath.Join(dir, "profile"),
		"--convert-to", "pdf", "--outdir", dir, path)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running soffice: %w: %s", err, out.String())
	}
	pdf := strings.TrimSuffix(path, filepath.Ext(path)) + ".pdf"
	if _, err := os.Stat(pdf); err != nil {
		return "", fmt.Errorf("soffice wrote no pdf: %w: %s", err, out.String())
	}
	return pdf, nil
}

// officeFonts returns the fonts of the module github.com/xo/ivy/fonts,
// which have the metrics of the fonts of Office, as LibreOffice uses them.
func officeFonts(t *testing.T) fs.FS {
	t.Helper()
	dir := filepath.Join("..", "fonts")
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("the fonts of github.com/xo/ivy/fonts are missing:", err)
	}
	return os.DirFS(dir)
}

// cropRows returns the part of img between the rows y0 and y1, the area of
// the cells without the header and the footer.
func cropRows(img image.Image, y0, y1 int) image.Image {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := range b.Dy() {
		for x := range b.Dx() {
			if y >= y0 && y < y1 {
				out.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
			} else {
				out.Set(x, y, color.White)
			}
		}
	}
	return out
}

// inkCols returns, for each column of pixels of img, whether the column
// holds a pixel darker than mid grey.
func inkCols(img image.Image) []bool {
	b := img.Bounds()
	cols := make([]bool, b.Dx())
	for x := range b.Dx() {
		for y := range b.Dy() {
			c := overWhite(img.At(b.Min.X+x, b.Min.Y+y))
			if (c[0]+c[1]+c[2])/3 < 128 {
				cols[x] = true
				break
			}
		}
	}
	return cols
}

// pager is the part of a document that the comparison needs.
type pager interface {
	PageCount(ctx context.Context) (int, error)
	Page(ctx context.Context, n int) (ivy.Page, error)
}

// compareSheet compares the pages of ivy with the PDF file of LibreOffice:
// the number of pages, the size of each page, and for each page the box of
// the ink of the cells and the bands of the rows that hold text. The cells
// are between the rows of pixels y0 and y1, outside the header and the
// footer, whose fonts differ.
//
// With a rule, the width of the ink, its bottom, the number of the bands of
// rows and the places of the rows of data come from the rule, and not from
// LibreOffice.
func compareSheet(t *testing.T, name string, d pager, pdf string, y0, y1 int, rule *sheetRule) {
	t.Helper()
	wantPages, err := pdfPageCount(t.Context(), pdf)
	if err != nil {
		t.Fatal(err)
	}
	gotPages, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pages: ivy %d, LibreOffice %d", gotPages, wantPages)
	if gotPages != wantPages {
		t.Errorf("expected %d pages, as LibreOffice, got: %d", wantPages, gotPages)
	}
	if rule != nil && len(rule.pages) != wantPages {
		t.Errorf("expected the %d pages of the rule, as LibreOffice gives, got: %d", len(rule.pages), wantPages)
	}
	near := func(a, b int) bool { return a-b <= sheetTolerance && b-a <= sheetTolerance }
	for i := range min(gotPages, wantPages) {
		page, err := d.Page(t.Context(), i)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ivy.RenderImage(t.Context(), page, 96, color.White)
		if err != nil {
			t.Fatal(err)
		}
		n := fmt.Sprint(i + 1)
		want, err := run(t.Context(), "pdftoppm", "-png", "-r", "96", "-f", n, "-l", n, "-singlefile", pdf)
		if err != nil {
			t.Fatal(err)
		}
		if out := os.Getenv("IVY_ORACLE_OUT"); out != "" {
			writePNG(t, filepath.Join(out, fmt.Sprintf("%s-%d-ivy.png", name, i+1)), got)
			writePNG(t, filepath.Join(out, fmt.Sprintf("%s-%d-soffice.png", name, i+1)), want)
		}
		if got.Bounds().Size() != want.Bounds().Size() {
			t.Errorf("page %d: expected the size %v of LibreOffice, got: %v", i+1, want.Bounds().Size(), got.Bounds().Size())
			continue
		}
		got, want = cropRows(got, y0, y1), cropRows(want, y0, y1)
		gotBox, gotInk := inkBox(got)
		wantBox, wantInk := inkBox(want)
		t.Logf("page %d: ink box: ivy %v (%d pixels), LibreOffice %v (%d pixels)", i+1, gotBox, gotInk, wantBox, wantInk)
		gotRows, wantRows := lineBands(inkRows(got)), lineBands(inkRows(want))
		t.Logf("page %d: rows: ivy %d, LibreOffice %d", i+1, len(gotRows), len(wantRows))
		if rule != nil && i < len(rule.pages) {
			if !near(gotBox.Min.X, wantBox.Min.X) || !near(gotBox.Min.Y, wantBox.Min.Y) {
				t.Errorf("page %d: expected the top left corner of the ink of LibreOffice %v within %d pixels, got: %v", i+1, wantBox.Min, sheetTolerance, gotBox.Min)
			}
			rule.check(t, i, gotBox, wantBox, gotRows, wantRows)
			continue
		}
		if !near(gotBox.Min.X, wantBox.Min.X) || !near(gotBox.Min.Y, wantBox.Min.Y) || !near(gotBox.Max.Y, wantBox.Max.Y) {
			t.Errorf("page %d: expected the ink box of LibreOffice %v within %d pixels, got: %v", i+1, wantBox, sheetTolerance, gotBox)
		}
		if w, ww := float64(gotBox.Dx()), float64(wantBox.Dx()); w < ww*(1-widthTolerance) || w > ww*(1+widthTolerance) {
			t.Errorf("page %d: expected the width of the ink of LibreOffice %v within %v, got: %v", i+1, ww, widthTolerance, w)
		}
		if len(gotRows) != len(wantRows) {
			t.Errorf("page %d: expected %d bands of rows, as LibreOffice, got: %d\nivy: %v\nLibreOffice: %v", i+1, len(wantRows), len(gotRows), gotRows, wantRows)
		} else {
			worst := 0
			for k := range gotRows {
				for e := range 2 {
					worst = max(worst, gotRows[k][e]-wantRows[k][e], wantRows[k][e]-gotRows[k][e])
				}
			}
			t.Logf("page %d: largest shift of a band of rows: %d pixels", i+1, worst)
			if worst > sheetTolerance {
				t.Errorf("page %d: expected the bands of rows within %d pixels, got a shift of %d\nivy: %v\nLibreOffice: %v", i+1, sheetTolerance, worst, gotRows, wantRows)
			}
		}
		gotCols, wantCols := lineBands(inkCols(got)), lineBands(inkCols(want))
		t.Logf("page %d: bands of columns: ivy %d, LibreOffice %d", i+1, len(gotCols), len(wantCols))
	}
}

// check checks page i of ivy against the rule: the box and the bands of
// rows of the ink of ivy, got and gotRows, and of LibreOffice, want and
// wantRows.
func (r *sheetRule) check(t *testing.T, i int, got, want image.Rectangle, gotRows, wantRows [][2]int) {
	t.Helper()
	p := r.pages[i]
	near := func(a, b float64) bool { return math.Abs(a-b) <= sheetTolerance }
	// The border under the headings runs across the band of columns, so
	// the ink is as wide as the band.
	if w := float64(got.Dx()); !near(w, p.width) {
		t.Errorf("page %d: expected the ink as wide as the band of columns, %.1f pixels, within %d, got: %v", i+1, p.width, sheetTolerance, w)
	}
	// The bands that are not rows of data, such as the print titles, are
	// those of LibreOffice.
	others := len(wantRows) - p.refRows
	if others < 0 || len(gotRows) != others+p.rows {
		t.Errorf("page %d: expected %d bands of rows, %d rows of data and %d others as in LibreOffice, got: %d\nivy: %v\nLibreOffice: %v",
			i+1, others+p.rows, p.rows, others, len(gotRows), gotRows, wantRows)
		return
	}
	// The text of the last row sits as far above the bottom of its row as
	// in LibreOffice.
	refBottom := r.top + r.refTitles + float64(p.refRows)*r.refRow
	bottom := r.top + r.titles + float64(p.rows)*r.row
	if exp := bottom - (refBottom - float64(want.Max.Y)); !near(float64(got.Max.Y), exp) {
		t.Errorf("page %d: expected the bottom of the ink at %.1f within %d pixels, after %d rows of data, got: %d", i+1, exp, sheetTolerance, p.rows, got.Max.Y)
	}
	// Each row of data starts one row of the rule after the one before it,
	// counted from the first one, so that an error of the height adds up.
	data := gotRows[others:]
	worst := 0.0
	for k, band := range data {
		worst = max(worst, math.Abs(float64(band[0]-data[0][0])-float64(k)*r.row))
	}
	t.Logf("page %d: largest shift of a row of data from the rule: %.1f pixels", i+1, worst)
	if worst > sheetTolerance {
		t.Errorf("page %d: expected the rows of data %.2f pixels apart within %d pixels, got a shift of %.1f\nivy: %v", i+1, r.row, sheetTolerance, worst, data)
	}
}

// TestXLSX compares the pages of an Excel file that ivy draws with the
// pages of LibreOffice: soffice converts the file to PDF, and pdftoppm of
// poppler draws each page. The sheet is too wide for its page, so both cut
// it into two bands of columns, and too long, so each band takes two pages.
// The print titles repeat on each page. Both draw Calibri with Carlito.
func TestXLSX(t *testing.T) {
	dir := t.TempDir()
	data := buildXLSX(t, 70)
	path := filepath.Join(dir, "sheet.xlsx")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("IVY_ORACLE_OUT"); out != "" {
		if err := os.WriteFile(filepath.Join(out, "sheet.xlsx"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pdf, err := sofficePDF(t.Context(), dir, path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := xlsx.Open(t.Context(), bytes.NewReader(data), xlsx.WithConfig(ivy.Config{Fonts: officeFonts(t)}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	// the margins of 0.75 inch: 72 pixels
	compareSheet(t, "xlsx", d, pdf, 70, sheetPageHeight-70, xlsxRule(70))
}

// xlsxRule returns the layout of the sheet of buildXLSX with rows of data,
// as Excel 2024 prints it, which ivy follows (see internal/sheet), and as
// LibreOffice prints it.
//
// ivy no longer matches LibreOffice here, because Excel is the primary
// reference of the files of Excel, and Excel prints them otherwise:
//
//   - Excel prints a row of Calibri of 11 points at 0.968 of its height
//     (sheet.ExcelRows): a row of 15 points is 14.52 points, 19.36 pixels. LibreOffice prints it at 20
//     pixels. So a page of ivy holds 44 rows of data, and one of
//     LibreOffice 42.
//   - Excel prints a column in the digits of the printer: the pixels of
//     the screen, in digits of 7 pixels of Calibri of 11 points, times the
//     digit at 600 dots per inch, 47 dots or 5.64 points. Column A of
//     30.71 characters is 215 pixels on the screen and 173.2 points on
//     paper, and the columns B to H of 14.71 characters are 103 pixels
//     and 83.0 points each. LibreOffice makes them about 1.5% narrower.
//
// Both put the columns A to E in the first band and F to H in the second,
// on 2 pages each, down then over.
func xlsxRule(rows int) *sheetRule {
	const px = 96.0 / 72
	const rowScale = 0.968
	// the paper of 11 inches less the margins of 0.75 inch
	page := (792 - 2*54) * px
	r := &sheetRule{
		top:       72,
		titles:    (26 + 15) * rowScale * px,
		row:       15 * rowScale * px,
		refTitles: (26 + 15) * px,
		refRow:    15 * px,
	}
	digit := func(pixels float64) float64 { return pixels / 7 * 5.64 * px }
	bands := []float64{digit(215) + 4*digit(103), 3 * digit(103)}
	per := rowsPerPage(page, r.titles, r.row)
	refPer := rowsPerPage(page, r.refTitles, r.refRow)
	for _, width := range bands {
		// ivy and LibreOffice print the same number of pages
		for done, refDone := 0, 0; done < rows; {
			n, refN := min(per, rows-done), min(refPer, rows-refDone)
			r.pages = append(r.pages, rulePage{rows: n, refRows: refN, width: width})
			done, refDone = done+n, refDone+refN
		}
	}
	return r
}

// TestODS compares the pages of an OpenDocument spreadsheet that ivy draws
// with the pages of LibreOffice, as TestXLSX does. The table is long
// enough for three pages, under a header and over a footer.
func TestODS(t *testing.T) {
	dir := t.TempDir()
	data := buildODS(t, 120)
	path := filepath.Join(dir, "sheet.ods")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("IVY_ORACLE_OUT"); out != "" {
		if err := os.WriteFile(filepath.Join(out, "sheet.ods"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pdf, err := sofficePDF(t.Context(), dir, path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := odf.Open(t.Context(), bytes.NewReader(data), odf.WithConfig(ivy.Config{Fonts: officeFonts(t)}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	// the margins of 0.75 inch and the header of 0.2 inch: 91 pixels
	compareSheet(t, "ods", d, pdf, 90, sheetPageHeight-90, nil)
}
