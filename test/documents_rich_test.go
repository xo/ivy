package test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/docx"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/odf"
	"github.com/xo/ivy/rtf"
)

// richFODT is a flat OpenDocument text with a header with the number of
// the page, a heading, formatted text, a footnote, two lists, an image, a
// table with a merged cell, paragraphs that fill a page, and a section of
// landscape pages. The verbs are the base64 of the image and the
// paragraphs.
const richFODT = `<?xml version="1.0" encoding="UTF-8"?>
<office:document xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" office:version="1.3" office:mimetype="application/vnd.oasis.opendocument.text">
<office:font-face-decls>
 <style:font-face style:name="Liberation Serif" svg:font-family="'Liberation Serif'" style:font-family-generic="roman"/>
 <style:font-face style:name="Liberation Sans" svg:font-family="'Liberation Sans'" style:font-family-generic="swiss"/>
</office:font-face-decls>
<office:styles>
 <style:default-style style:family="paragraph"><style:paragraph-properties fo:orphans="2" fo:widows="2"/><style:text-properties style:font-name="Liberation Serif" fo:font-size="12pt"/></style:default-style>
 <style:style style:name="Standard" style:family="paragraph"/>
 <style:style style:name="Text_20_body" style:display-name="Text body" style:family="paragraph" style:parent-style-name="Standard"><style:paragraph-properties fo:margin-top="0in" fo:margin-bottom="0.0972in" fo:line-height="115%%" fo:text-align="justify"/></style:style>
 <style:style style:name="Heading_20_1" style:display-name="Heading 1" style:family="paragraph" style:parent-style-name="Standard" style:next-style-name="Text_20_body"><style:paragraph-properties fo:margin-top="0.1665in" fo:margin-bottom="0.0835in" fo:keep-with-next="always"/><style:text-properties style:font-name="Liberation Sans" fo:font-size="18pt" fo:font-weight="bold"/></style:style>
 <style:style style:name="Header" style:family="paragraph" style:parent-style-name="Standard"><style:text-properties fo:font-size="10pt"/></style:style>
 <style:style style:name="Footnote" style:family="paragraph" style:parent-style-name="Standard"><style:paragraph-properties fo:margin-left="0.2in" fo:text-indent="-0.2in"/><style:text-properties fo:font-size="10pt"/></style:style>
 <text:list-style style:name="L1">
  <text:list-level-style-bullet text:level="1" text:bullet-char="•"><style:list-level-properties text:list-level-position-and-space-mode="label-alignment"><style:list-level-label-alignment text:label-followed-by="listtab" text:list-tab-stop-position="0.5in" fo:text-indent="-0.25in" fo:margin-left="0.5in"/></style:list-level-properties></text:list-level-style-bullet>
 </text:list-style>
 <text:list-style style:name="L2">
  <text:list-level-style-number text:level="1" style:num-suffix="." style:num-format="1"><style:list-level-properties text:list-level-position-and-space-mode="label-alignment"><style:list-level-label-alignment text:label-followed-by="listtab" text:list-tab-stop-position="0.5in" fo:text-indent="-0.25in" fo:margin-left="0.5in"/></style:list-level-properties></text:list-level-style-number>
  <text:list-level-style-number text:level="2" style:num-suffix=")" style:num-format="a"><style:list-level-properties text:list-level-position-and-space-mode="label-alignment"><style:list-level-label-alignment text:label-followed-by="listtab" text:list-tab-stop-position="0.75in" fo:text-indent="-0.25in" fo:margin-left="0.75in"/></style:list-level-properties></text:list-level-style-number>
 </text:list-style>
</office:styles>
<office:automatic-styles>
 <style:style style:name="T1" style:family="text"><style:text-properties fo:font-weight="bold"/></style:style>
 <style:style style:name="T2" style:family="text"><style:text-properties fo:font-style="italic" fo:color="#ff0000" style:text-underline-style="solid" style:text-underline-width="auto" style:text-underline-color="font-color"/></style:style>
 <style:style style:name="P2" style:family="paragraph" style:parent-style-name="Text_20_body" style:master-page-name="Landscape"/>
 <style:style style:name="Tbl" style:family="table"><style:table-properties style:width="6in" table:align="left"/></style:style>
 <style:style style:name="Tbl.A" style:family="table-column"><style:table-column-properties style:column-width="1in"/></style:style>
 <style:style style:name="Tbl.B" style:family="table-column"><style:table-column-properties style:column-width="1.5in"/></style:style>
 <style:style style:name="Tbl.C" style:family="table-column"><style:table-column-properties style:column-width="3.5in"/></style:style>
 <style:style style:name="Cell" style:family="table-cell"><style:table-cell-properties fo:padding="0.0382in" fo:border="0.5pt solid #000000"/></style:style>
 <style:style style:name="fr1" style:family="graphic"><style:graphic-properties style:wrap="none" style:vertical-pos="top" style:horizontal-pos="center"/></style:style>
 <style:page-layout style:name="pm1"><style:page-layout-properties fo:page-width="8.5in" fo:page-height="11in" style:print-orientation="portrait" fo:margin-top="0.7874in" fo:margin-bottom="0.7874in" fo:margin-left="0.7874in" fo:margin-right="0.7874in"/><style:header-style><style:header-footer-properties fo:min-height="0in" fo:margin-bottom="0.1965in"/></style:header-style><style:footer-style><style:header-footer-properties fo:min-height="0in" fo:margin-top="0.1965in"/></style:footer-style></style:page-layout>
 <style:page-layout style:name="pm2"><style:page-layout-properties fo:page-width="11in" fo:page-height="8.5in" style:print-orientation="landscape" fo:margin-top="0.7874in" fo:margin-bottom="0.7874in" fo:margin-left="0.7874in" fo:margin-right="0.7874in"/></style:page-layout>
</office:automatic-styles>
<office:master-styles>
 <style:master-page style:name="Standard" style:page-layout-name="pm1">
  <style:header><text:p text:style-name="Header">Header text, page <text:page-number text:select-page="current">1</text:page-number> of <text:page-count>2</text:page-count></text:p></style:header>
  <style:footer><text:p text:style-name="Header">Footer text</text:p></style:footer>
 </style:master-page>
 <style:master-page style:name="Landscape" style:page-layout-name="pm2"/>
</office:master-styles>
<office:body><office:text>
<text:h text:style-name="Heading_20_1" text:outline-level="1">Section 1</text:h>
<text:p text:style-name="Text_20_body">A plain paragraph having some <text:span text:style-name="T1">bold</text:span> and some <text:span text:style-name="T2">italic red underlined</text:span> text, with a footnote<text:note text:id="ftn1" text:note-class="footnote"><text:note-citation>1</text:note-citation><text:note-body><text:p text:style-name="Footnote">The text of the footnote.</text:p></text:note-body></text:note> here.</text:p>
<text:list text:style-name="L1"><text:list-item><text:p text:style-name="Text_20_body">first bullet</text:p></text:list-item><text:list-item><text:p text:style-name="Text_20_body">second bullet</text:p></text:list-item></text:list>
<text:list text:style-name="L2"><text:list-item><text:p text:style-name="Text_20_body">first number</text:p></text:list-item><text:list-item><text:p text:style-name="Text_20_body">second number</text:p><text:list><text:list-item><text:p text:style-name="Text_20_body">nested letter</text:p></text:list-item></text:list></text:list-item><text:list-item><text:p text:style-name="Text_20_body">third number</text:p></text:list-item></text:list>
<text:p text:style-name="Text_20_body"><draw:frame draw:style-name="fr1" draw:name="Image1" text:anchor-type="as-char" svg:width="2in" svg:height="1in" draw:z-index="0"><draw:image><office:binary-data>%[1]s</office:binary-data></draw:image></draw:frame></text:p>
<table:table table:name="Table1" table:style-name="Tbl">
 <table:table-column table:style-name="Tbl.A"/><table:table-column table:style-name="Tbl.B"/><table:table-column table:style-name="Tbl.C"/>
 <table:table-row><table:table-cell table:style-name="Cell" office:value-type="string"><text:p>Qty</text:p></table:table-cell><table:table-cell table:style-name="Cell" office:value-type="string"><text:p>Id</text:p></table:table-cell><table:table-cell table:style-name="Cell" office:value-type="string"><text:p>Desc</text:p></table:table-cell></table:table-row>
 <table:table-row><table:table-cell table:style-name="Cell" table:number-rows-spanned="2" office:value-type="string"><text:p>3</text:p></table:table-cell><table:table-cell table:style-name="Cell"><text:p>101</text:p></table:table-cell><table:table-cell table:style-name="Cell"><text:p>Spam</text:p></table:table-cell></table:table-row>
 <table:table-row><table:covered-table-cell/><table:table-cell table:style-name="Cell"><text:p>422</text:p></table:table-cell><table:table-cell table:style-name="Cell"><text:p>Eggs</text:p></table:table-cell></table:table-row>
</table:table>
%[2]s
<text:p text:style-name="P2">Landscape section paragraph.</text:p>
</office:text></office:body></office:document>
`

// richDocument returns the text of richFODT.
func richDocument() string {
	img := base64.StdEncoding.EncodeToString(fixture.PNG(200, 100, color.NRGBA{R: 30, G: 120, B: 200, A: 255}))
	var paras strings.Builder
	for i := range 14 {
		fmt.Fprintf(&paras, `<text:p text:style-name="Text_20_body">%s %d</text:p>`, strings.Join(documentText, " "), i)
	}
	return fmt.Sprintf(richFODT, img, paras.String())
}

// richTolerance is the distance in pixels at 96 dots per inch within which
// the ink of ivy must be from the ink of LibreOffice. The pages hold a
// header, a table and an image, whose places differ more than the lines of
// TestDocuments.
const richTolerance = 8

// openDocx, openODT and openRTF open a document of each format.
func openDocx(buf []byte, cfg ivy.Config) (documentPager, error) {
	return docx.Open(context.Background(), bytes.NewReader(buf), docx.WithConfig(cfg))
}

func openODT(buf []byte, cfg ivy.Config) (documentPager, error) {
	return odf.Open(context.Background(), bytes.NewReader(buf), odf.WithConfig(cfg))
}

func openRTF(buf []byte, cfg ivy.Config) (documentPager, error) {
	return rtf.Open(context.Background(), bytes.NewReader(buf), rtf.WithConfig(cfg))
}

// sofficeConvert converts the file path to the format, such as "docx" or
// "rtf", with LibreOffice, in a profile of its own in dir, and returns the
// path of the new file in the folder out.
func sofficeConvert(ctx context.Context, dir, path, format, out string) (string, error) {
	exe, err := exec.LookPath("soffice")
	if err != nil {
		return "", fmt.Errorf("oracle soffice is not installed: %w", err)
	}
	if err := os.MkdirAll(out, 0o700); err != nil {
		return "", fmt.Errorf("making the folder of soffice: %w", err)
	}
	cmd := exec.CommandContext(ctx, exe, "--headless", "--norestore",
		"-env:UserInstallation=file://"+filepath.Join(dir, "profile"),
		"--convert-to", format, "--outdir", out, path)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running soffice: %w: %s", err, buf.String())
	}
	name := filepath.Join(out, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))+"."+strings.Split(format, ":")[0])
	if _, err := os.Stat(name); err != nil {
		return "", fmt.Errorf("soffice wrote no %s: %w: %s", format, err, buf.String())
	}
	return name, nil
}

// TestDocumentsRich compares the pages of a document with a header, a
// footnote, lists, an image, a table and two sections, which LibreOffice
// converts from a flat OpenDocument text into each format, with the pages
// that LibreOffice makes of each file. The fonts are Liberation Serif and
// Liberation Sans, from the module github.com/xo/ivy/fonts.
func TestDocumentsRich(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rich.fodt")
	if err := os.WriteFile(path, []byte(richDocument()), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, format string
		open         func(buf []byte, cfg ivy.Config) (documentPager, error)
		// pagesOnly compares the number of pages only. LibreOffice reads
		// the picture of its own RTF file as a shape at the offset of the
		// shape, 1 inch above its paragraph, and the header of the first
		// section on the landscape pages, so its pages of the RTF file
		// differ from its pages of the same text in the other formats.
		pagesOnly bool
	}{
		{"docx", "docx:MS Word 2007 XML", openDocx, false},
		{"odt", "odt", openODT, false},
		{"rtf", "rtf", openRTF, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sub := filepath.Join(dir, tc.name)
			file, err := sofficeConvert(t.Context(), dir, path, tc.format, sub)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			pdf, err := sofficePDF(t.Context(), sub, file)
			if err != nil {
				t.Fatal(err)
			}
			wantPages, err := pdfPageCount(t.Context(), pdf)
			if err != nil {
				t.Fatal(err)
			}
			d, err := tc.open(data, ivy.Config{Fonts: officeFonts(t)})
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
			t.Logf("pages: ivy %d, soffice %d", gotPages, wantPages)
			if gotPages != wantPages {
				t.Errorf("expected %d pages, as soffice, got: %d", wantPages, gotPages)
			}
			if tc.pagesOnly {
				return
			}
			for i := range min(gotPages, wantPages) {
				want, err := run(t.Context(), "pdftoppm", "-png", "-r", "96", "-f", fmt.Sprint(i+1), "-l", fmt.Sprint(i+1), "-singlefile", pdf)
				if err != nil {
					t.Fatal(err)
				}
				page, err := d.Page(t.Context(), i)
				if err != nil {
					t.Fatal(err)
				}
				got, err := ivy.RenderImage(t.Context(), page, 96, color.White)
				if err != nil {
					t.Fatal(err)
				}
				if out := os.Getenv("IVY_ORACLE_OUT"); out != "" {
					writePNG(t, filepath.Join(out, fmt.Sprintf("rich-%s-%d-ivy.png", tc.name, i+1)), got)
					writePNG(t, filepath.Join(out, fmt.Sprintf("rich-%s-%d-soffice.png", tc.name, i+1)), want)
				}
				compareRich(t, i, got, want)
			}
		})
	}
}

// compareRich compares page i of ivy with the page of LibreOffice: the
// size of the page, the box of the ink and the bands of the lines.
func compareRich(t *testing.T, i int, got, want image.Image) {
	t.Helper()
	if got.Bounds().Size() != want.Bounds().Size() {
		t.Errorf("page %d: expected a page of %v pixels, as soffice, got: %v", i+1, want.Bounds().Size(), got.Bounds().Size())
		return
	}
	gotBox, _ := inkBox(got)
	wantBox, _ := inkBox(want)
	t.Logf("page %d: ink box: ivy %v, soffice %v", i+1, gotBox, wantBox)
	near := func(a, b int) bool { return a-b <= richTolerance && b-a <= richTolerance }
	if !near(gotBox.Min.X, wantBox.Min.X) || !near(gotBox.Min.Y, wantBox.Min.Y) ||
		!near(gotBox.Max.X, wantBox.Max.X) || !near(gotBox.Max.Y, wantBox.Max.Y) {
		t.Errorf("page %d: expected the ink box of soffice %v within %d pixels, got: %v", i+1, wantBox, richTolerance, gotBox)
	}
	// The thin borders of a table are lighter in ivy than in poppler,
	// which draws a line at least one pixel wide. The bands count the
	// light grey too, so the borders of a table join its rows in both.
	gotBands, wantBands := lineBands(inkRowsBelow(got, 224)), lineBands(inkRowsBelow(want, 224))
	t.Logf("page %d: lines: ivy %d, soffice %d", i+1, len(gotBands), len(wantBands))
	if d := len(gotBands) - len(wantBands); d > 2 || d < -2 {
		t.Errorf("page %d: expected %d bands of ink, as soffice, within 2, got: %d\nivy: %v\nsoffice: %v", i+1, len(wantBands), len(gotBands), gotBands, wantBands)
	}
}

// inkRowsBelow returns, for each row of pixels of img, whether the row holds
// a pixel darker than the grey level, over white.
func inkRowsBelow(img image.Image, level float64) []bool {
	b := img.Bounds()
	rows := make([]bool, b.Dy())
	for y := range b.Dy() {
		for x := range b.Dx() {
			c := overWhite(img.At(b.Min.X+x, b.Min.Y+y))
			if (c[0]+c[1]+c[2])/3 < level {
				rows[y] = true
				break
			}
		}
	}
	return rows
}
