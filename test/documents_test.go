package test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/docx"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/odf"
	"github.com/xo/ivy/rtf"
)

// documentText is the text of the paragraphs of the documents of
// TestDocuments.
var documentText = []string{
	"The quick brown fox jumps over the lazy dog. Pack my box with five dozen liquor jugs.",
	"How vexingly quick daft zebras jump. Sphinx of black quartz, judge my vow. The five boxing wizards jump quickly.",
	"Jackdaws love my big sphinx of quartz. Amazingly few discotheques provide jukeboxes for the dancers of the town.",
}

// documentParagraphs is the number of paragraphs of the documents, which
// fill more than one page.
const documentParagraphs = 40

// xmlText escapes s for XML.
func xmlText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// buildDocx returns a Word file of US Letter with margins of 1 inch: a
// heading in Arial and paragraphs in Calibri of 11 points, single spaced,
// with 8 points after each one.
func buildDocx() []byte {
	var body strings.Builder
	body.WriteString(`<w:p><w:pPr><w:spacing w:after="160"/></w:pPr><w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial"/><w:b/><w:sz w:val="32"/></w:rPr><w:t>Chapter One</w:t></w:r></w:p>`)
	for i := range documentParagraphs {
		fmt.Fprintf(&body, `<w:p><w:r><w:t>%s</w:t></w:r></w:p>`, xmlText(documentText[i%len(documentText)]))
	}
	body.WriteString(`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720"/></w:sectPr>`)
	styles := fixture.WordPart{ID: "rIdS", Type: "styles", Name: "word/styles.xml", Data: fixture.WordXML("w:styles",
		`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="22"/></w:rPr></w:rPrDefault>`+
			`<w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="240" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>`+
			`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>`)}
	return fixture.DOCX(body.String(), styles)
}

// buildODT returns an OpenDocument text of US Letter with margins of 1
// inch: a heading in Liberation Sans and paragraphs in Liberation Serif of
// 12 points, with 0.1 inch after each one.
func buildODT(t *testing.T) []byte {
	t.Helper()
	const ns = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" ` +
		`xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" ` +
		`xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" office:version="1.3"`
	styles := `<?xml version="1.0" encoding="UTF-8"?><office:document-styles ` + ns + `>
<office:font-face-decls><style:font-face style:name="Liberation Serif" svg:font-family="'Liberation Serif'" style:font-family-generic="roman"/><style:font-face style:name="Liberation Sans" svg:font-family="'Liberation Sans'" style:font-family-generic="swiss"/></office:font-face-decls>
<office:styles><style:default-style style:family="paragraph"><style:text-properties style:font-name="Liberation Serif" fo:font-size="12pt"/></style:default-style>
<style:style style:name="Standard" style:family="paragraph"><style:paragraph-properties fo:margin-top="0in" fo:margin-bottom="0.1in"/></style:style>
<style:style style:name="Heading" style:family="paragraph" style:parent-style-name="Standard"><style:text-properties style:font-name="Liberation Sans" fo:font-size="16pt" fo:font-weight="bold"/></style:style></office:styles>
<office:automatic-styles><style:page-layout style:name="pm1"><style:page-layout-properties fo:page-width="8.5in" fo:page-height="11in" fo:margin-top="1in" fo:margin-bottom="1in" fo:margin-left="1in" fo:margin-right="1in"/></style:page-layout></office:automatic-styles>
<office:master-styles><style:master-page style:name="Standard" style:page-layout-name="pm1"/></office:master-styles></office:document-styles>`
	var body strings.Builder
	body.WriteString(`<text:p text:style-name="Heading">Chapter One</text:p>`)
	for i := range documentParagraphs {
		fmt.Fprintf(&body, `<text:p text:style-name="Standard">%s</text:p>`, xmlText(documentText[i%len(documentText)]))
	}
	content := `<?xml version="1.0" encoding="UTF-8"?><office:document-content ` + ns + `><office:body><office:text>` + body.String() + `</office:text></office:body></office:document-content>`
	manifest := `<?xml version="1.0" encoding="UTF-8"?><manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.3">` +
		`<manifest:file-entry manifest:full-path="/" manifest:media-type="application/vnd.oasis.opendocument.text"/>` +
		`<manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/>` +
		`<manifest:file-entry manifest:full-path="styles.xml" manifest:media-type="text/xml"/></manifest:manifest>`
	return zipFiles(t, [][2]string{
		{"mimetype", "application/vnd.oasis.opendocument.text"},
		{"META-INF/manifest.xml", manifest},
		{"content.xml", content},
		{"styles.xml", styles},
	})
}

// buildRTF returns an RTF file of US Letter with margins of 1 inch: a
// heading in Arial and paragraphs in Times New Roman of 12 points, with 8
// points after each one.
func buildRTF() []byte {
	var b strings.Builder
	b.WriteString(`{\rtf1\ansi\ansicpg1252\deff0{\fonttbl{\f0\froman\fcharset0 Times New Roman;}{\f1\fswiss\fcharset0 Arial;}}` + "\n")
	b.WriteString(`\paperw12240\paperh15840\margl1440\margr1440\margt1440\margb1440` + "\n")
	b.WriteString(`\pard\sa160\f1\fs32\b Chapter One\par` + "\n")
	for i := range documentParagraphs {
		fmt.Fprintf(&b, "\\pard\\plain\\sa160\\f0\\fs24 %s\\par\n", documentText[i%len(documentText)])
	}
	b.WriteString("}")
	return []byte(b.String())
}

// documentPager is a document with pages that a test closes.
type documentPager interface {
	ivy.Pager
	Close() error
}

// The pages of the documents of TestDocuments: US Letter with margins of 1
// inch, at 96 dots per inch.
const (
	letterWidth  = 816
	letterHeight = 1056
	letterMargin = 96
)

// documentTolerance is the distance in pixels at 96 dots per inch within
// which a line of ivy must be from the line of LibreOffice. The fonts are
// the same files, but LibreOffice rounds the height of each line to its
// own units, so the lines drift by a fraction of a pixel per line.
const documentTolerance = 6

// TestDocuments compares the first page of a Word file, of an OpenDocument
// text and of an RTF file that ivy draws with the page that LibreOffice
// makes: soffice converts each file to PDF, and pdftoppm of poppler draws
// page 1 of the PDF file. ivy draws with the fonts of the module
// github.com/xo/ivy/fonts, which LibreOffice also has: Carlito for Calibri,
// and Liberation for Arial and Times New Roman. As TestEPUB, the test
// compares what the layout decides: the number of pages, the box of the ink
// and the bands of the lines of text.
func TestDocuments(t *testing.T) {
	for _, tc := range []struct {
		name string
		ext  string
		data []byte
		open func(buf []byte, cfg ivy.Config) (documentPager, error)
	}{
		{"docx", "docx", buildDocx(), func(buf []byte, cfg ivy.Config) (documentPager, error) {
			return docx.Open(t.Context(), bytes.NewReader(buf), docx.WithConfig(cfg))
		}},
		{"odt", "odt", buildODT(t), func(buf []byte, cfg ivy.Config) (documentPager, error) {
			return odf.Open(t.Context(), bytes.NewReader(buf), odf.WithConfig(cfg))
		}},
		{"rtf", "rtf", buildRTF(), func(buf []byte, cfg ivy.Config) (documentPager, error) {
			return rtf.Open(t.Context(), bytes.NewReader(buf), rtf.WithConfig(cfg))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "document."+tc.ext)
			if err := os.WriteFile(path, tc.data, 0o600); err != nil {
				t.Fatal(err)
			}
			pdf, err := sofficePDF(t.Context(), dir, path)
			if err != nil {
				t.Fatal(err)
			}
			want, err := run(t.Context(), "pdftoppm", "-png", "-r", "96", "-f", "1", "-l", "1", "-singlefile", pdf)
			if err != nil {
				t.Fatal(err)
			}
			wantPages, err := pdfPageCount(t.Context(), pdf)
			if err != nil {
				t.Fatal(err)
			}
			d, err := tc.open(tc.data, ivy.Config{Fonts: officeFonts(t)})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := d.Close(); err != nil {
					t.Error(err)
				}
			}()
			gotPages, err := d.PageCount(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			page, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ivy.RenderImage(t.Context(), page, 96, color.White)
			if err != nil {
				t.Fatal(err)
			}
			if out := os.Getenv("IVY_ORACLE_OUT"); out != "" {
				writePNG(t, filepath.Join(out, tc.name+"-ivy.png"), got)
				writePNG(t, filepath.Join(out, tc.name+"-soffice.png"), want)
			}
			compareDocument(t, got, want, gotPages, wantPages)
		})
	}
}

// compareDocument compares page 1 of ivy with page 1 of LibreOffice.
func compareDocument(t *testing.T, got, want image.Image, gotPages, wantPages int) {
	t.Helper()
	for name, img := range map[string]image.Image{"ivy": got, "soffice": want} {
		if size := img.Bounds().Size(); size != image.Pt(letterWidth, letterHeight) {
			t.Errorf("expected a page of %dx%d pixels from %s, got: %v", letterWidth, letterHeight, name, size)
		}
	}
	t.Logf("pages: ivy %d, soffice %d", gotPages, wantPages)
	if gotPages != wantPages {
		t.Errorf("expected %d pages, as soffice, got: %d", wantPages, gotPages)
	}
	gotBox, gotInk := inkBox(got)
	wantBox, wantInk := inkBox(want)
	t.Logf("ink box: ivy %v (%d pixels), soffice %v (%d pixels)", gotBox, gotInk, wantBox, wantInk)
	near := func(a, b, d int) bool { return a-b <= d && b-a <= d }
	if !near(gotBox.Min.X, wantBox.Min.X, 3) || !near(gotBox.Min.Y, wantBox.Min.Y, 3) ||
		!near(gotBox.Max.X, wantBox.Max.X, documentTolerance) || !near(gotBox.Max.Y, wantBox.Max.Y, documentTolerance*2) {
		t.Errorf("expected the ink box of soffice %v, got: %v", wantBox, gotBox)
	}
	if gotBox.Min.X < letterMargin-2 || gotBox.Min.Y < letterMargin-2 ||
		gotBox.Max.X > letterWidth-letterMargin+2 || gotBox.Max.Y > letterHeight-letterMargin+2 {
		t.Errorf("expected the ink inside the margins, got: %v", gotBox)
	}
	gotBands, wantBands := lineBands(inkRows(got)), lineBands(inkRows(want))
	t.Logf("lines: ivy %d, soffice %d", len(gotBands), len(wantBands))
	if len(gotBands) != len(wantBands) {
		t.Errorf("expected %d lines, as soffice, got: %d\nivy: %v\nsoffice: %v", len(wantBands), len(gotBands), gotBands, wantBands)
		return
	}
	shift := 0
	for i := range gotBands {
		d := gotBands[i][0] - wantBands[i][0]
		shift = max(shift, d, -d)
		if !near(gotBands[i][0], wantBands[i][0], documentTolerance) {
			t.Errorf("expected line %d at the rows %v of soffice within %d pixels, got: %v", i+1, wantBands[i], documentTolerance, gotBands[i])
		}
	}
	t.Logf("largest shift of a line: %d pixels", shift)
}
