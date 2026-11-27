package odf

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// odtFilePage renders the page i of the file name of testdata at 72 dpi.
func odtFilePage(t *testing.T, name string, i int) image.Image {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), i)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// odtRed is the color of the pictures and of the line of the test files.
var odtRed = color.NRGBA{R: 255, A: 255}

// TestLineFixedFile checks that the line of line-fixed.odt, with
// fo:line-height 18pt, keeps that height when its picture as a character
// is 1 inch high. In LibreOffice 26.8, the bottom of the picture is at the
// baseline, about 114 points from the top of the page, and the picture
// goes up over the paragraph before it, from 42 points. The line after it
// starts 18 points lower. ivy made the line grow, and put the picture from
// 100 points.
func TestLineFixedFile(t *testing.T) {
	img := odtFilePage(t, "line-fixed.odt", 0)
	pic, _ := odtColorBox(img, odtRed)
	if pic.Empty() || pic.Dy() < 70 || pic.Dy() > 74 || pic.Min.Y > 60 {
		t.Fatalf("expected the picture of 72 points from above the top margin, got: %v", pic)
	}
	// the text of the next line, at the left of the picture, is at most
	// 18 points below the bottom of the picture
	next := odtInk(img, image.Rect(72, pic.Max.Y+4, 230, pic.Max.Y+40))
	if next.Empty() || next.Min.Y > pic.Max.Y+20 {
		t.Errorf("expected the next line 18 points below the picture at %v, got: %v", pic, next)
	}
}

// TestLineNumberObjectFile checks that the number of the line of
// line-number-object.odt that holds only a picture is at the bottom of the
// picture, as in LibreOffice 26.8. ivy drew it at the top.
func TestLineNumberObjectFile(t *testing.T) {
	img := odtFilePage(t, "line-number-object.odt", 0)
	pic, _ := odtColorBox(img, odtRed)
	if pic.Empty() || pic.Dy() < 70 {
		t.Fatalf("expected the picture, got: %v", pic)
	}
	// the numbers end 0.2 inch before the text, at 72 points
	number := odtInk(img, image.Rect(40, pic.Min.Y, 70, pic.Max.Y+1))
	if number.Empty() || number.Max.Y < pic.Max.Y-2 || number.Max.Y > pic.Max.Y+1 {
		t.Errorf("expected the number at the bottom of the picture at %v, got: %v", pic, number)
	}
}

// TestPageMirroredFile checks that the left pages of page-mirrored.odt,
// with style:page-usage mirrored, swap the margins of 2 inches at the left
// and 0.5 inch at the right, as in LibreOffice 26.8. The text of each page
// starts at 144 points on the right pages, and at 36 points on the left
// pages.
func TestPageMirroredFile(t *testing.T) {
	for i, want := range []int{144, 36, 144} {
		img := odtFilePage(t, "page-mirrored.odt", i)
		ink := odtInk(img, image.Rect(0, 0, 612, 792))
		if ink.Empty() || ink.Min.X < want-1 || ink.Min.X > want+2 || ink.Max.X > want+432 {
			t.Errorf("page %d: expected the text from %d points, got: %v", i+1, want, ink)
		}
	}
}

// TestFootnoteSepFile checks the line above the footnote of
// footnote-sep.odt, from its style:footnote-sep: a red line of 0.1 cm,
// half as wide as the page area and at its center, 0.3 cm above the
// footnote. LibreOffice 26.8 draws it from 189 to 423 points, from 697.1
// to 699.9 points, and the footnote ends at the bottom margin, 720 points.
func TestFootnoteSepFile(t *testing.T) {
	img := odtFilePage(t, "footnote-sep.odt", 0)
	line, _ := odtColorBox(img, odtRed)
	near := func(got, want int) bool { return got >= want-1 && got <= want+1 }
	if line.Empty() || !near(line.Min.X, 189) || !near(line.Max.X, 423) || line.Dy() < 2 || line.Dy() > 4 {
		t.Fatalf("expected the red line from 189 to 423 points, got: %v", line)
	}
	note := odtInk(img, image.Rect(72, line.Max.Y+1, 540, 792))
	if note.Empty() || note.Min.Y < line.Max.Y+8 || note.Max.Y > 721 {
		t.Errorf("expected the footnote 0.3 cm below the line at %v, got: %v", line, note)
	}
}

// TestFootnoteSep checks the rules of the footnote area from
// style:footnote-sep, as LibreOffice 26.8 lays it out: a thickness or a
// spacing that the element does not give is 0, the line style none keeps
// the thickness in the spacing without a line, and a page layout without
// the element takes the line of a new document of LibreOffice.
func TestFootnoteSep(t *testing.T) {
	for _, tt := range []struct {
		name, sep, want string
	}{
		{"lo", `<style:footnote-sep style:width="0.018cm" style:distance-before-sep="0.101cm" style:distance-after-sep="0.101cm" style:line-style="solid" style:adjustment="left" style:rel-width="25%" style:color="#000000"/>`,
			`@footnote { margin-top: 2.86pt; padding-top: 2.86pt; border-top: 0.51pt solid transparent; background: linear-gradient(#000000, #000000) border-box no-repeat; background-size: 25% 0.51pt; background-position: left top }`},
		{"part", `<style:footnote-sep style:adjustment="left" style:rel-width="25%" style:color="#000000"/>`,
			`@footnote { margin-top: 0pt; padding-top: 0pt; border-top: none }`},
		{"none", `<style:footnote-sep style:width="1pt" style:distance-before-sep="2pt" style:line-style="none" style:rel-width="25%"/>`,
			`@footnote { margin-top: 2pt; padding-top: 0pt; border-top: 1pt solid transparent }`},
		{"right", `<style:footnote-sep style:width="1pt" style:adjustment="right" style:rel-width="50%" style:color="#ff0000"/>`,
			`background-size: 50% 1pt; background-position: right top`},
		{"default", ``, `@footnote { ` + defaultODTFootnoteSep.css() + ` }`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			styles := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles ` + odtNamespaces + `><office:automatic-styles>
<style:page-layout style:name="pm1"><style:page-layout-properties fo:margin="0.5in">` + tt.sep + `</style:page-layout-properties></style:page-layout>
</office:automatic-styles><office:master-styles><style:master-page style:name="Standard" style:page-layout-name="pm1"/></office:master-styles>
</office:document-styles>`
			_, rules := odtHTML(t, fixture.Zip(
				fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "text")},
				fixture.Entry{Name: "content.xml", Data: []byte(odtContent(`<text:p>Text</text:p>`))},
				fixture.Entry{Name: "styles.xml", Data: []byte(styles)},
			))
			if !strings.Contains(rules, tt.want) {
				t.Errorf("expected %q in the rules of the pages, got:\n%s", tt.want, rules)
			}
		})
	}
	// the style sheet gives the same line to a page without a page layout
	if want := "@footnote { " + defaultODTFootnoteSep.css() + " }"; !strings.Contains(odtStyleSheet, want) {
		t.Errorf("expected %q in the style sheet", want)
	}
}

// TestContextualCellTop checks that a paragraph of a table cell keeps its
// spacing above it at the top of the part of the cell after a page break,
// also when contextual spacing leaves the spacing out between it and the
// paragraph before it, as LibreOffice does on page 23 of
// libreoffice/odt/landscape-a4-table.odt (D56).
func TestContextualCellTop(t *testing.T) {
	const auto = `<style:style style:name="PC" style:family="paragraph" style:parent-style-name="Body"><style:paragraph-properties fo:margin-top="12pt" fo:margin-bottom="10pt" style:contextual-spacing="true"/></style:style>`
	body := `<table:table><table:table-column/><table:table-row><table:table-cell>` +
		`<text:p text:style-name="PC">First</text:p><text:p text:style-name="PC">Second</text:p>` +
		`</table:table-cell></table:table-row></table:table>`
	html, _ := odtHTML(t, odtWith(body, auto, nil))
	p := odtParagraph(t, html, "Second")
	if strings.Contains(p, "margin-top") || !strings.Contains(p, "-ivy-page-top-space: discard 12pt") {
		t.Errorf("expected no margin above the paragraph, and the spacing at the top of the part of a cell, got: %s", p)
	}
}

// TestTableRelativeFile checks the tables of table-relative.odt, which
// LibreOffice 26.8 wrote with table:align margins and only
// style:rel-column-width, of 1 to 3. LibreOffice gives the first table the
// width between the margins of the page, from 72 to 540 points, and its
// first column a quarter of it, to 189 points. The second table has the
// margins of 36 and 72 points, from 108 to 468 points. ivy gave the
// columns the widths of their text.
func TestTableRelativeFile(t *testing.T) {
	cell := color.NRGBA{R: 0xc0, G: 0xe0, B: 0xff, A: 255}
	img := odtFilePage(t, "table-relative.odt", 0)
	for _, tt := range []struct {
		region image.Rectangle
		want   image.Rectangle
	}{
		{image.Rect(0, 84, 612, 116), image.Rect(73, 87, 540, 114)},
		{image.Rect(0, 126, 612, 172), image.Rect(109, 129, 468, 170)},
	} {
		box, _ := odtColorBox(odtRegion{img, tt.region}, cell)
		if d := box.Min.Sub(tt.want.Min); d.X < -2 || d.X > 2 || d.Y < -2 || d.Y > 2 || box.Max.X < tt.want.Max.X-2 || box.Max.X > tt.want.Max.X+2 {
			t.Errorf("expected the table at %v, got %v", tt.want, box)
		}
	}
	buf, err := os.ReadFile(filepath.Join("testdata", "table-relative.odt"))
	if err != nil {
		t.Fatal(err)
	}
	html, _ := odtHTML(t, buf)
	for _, want := range []string{
		`<table style="width: 100%; table-layout: fixed"><colgroup><col style="width: 25%"/><col style="width: 75%"/></colgroup>`,
		`<table style="width: calc(100% - 108pt); table-layout: fixed; margin-left: 36pt"><colgroup><col style="width: 25%"/>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the xhtml to hold %q, got:\n%s", want, html)
		}
	}
	for _, tt := range []struct {
		columns []odtColumn
		want    []float64
		rel     bool
	}{
		{[]odtColumn{{0, 1000}, {0, 3000}}, []float64{25, 75}, true},
		{[]odtColumn{{72, 1000}, {0, 3000}}, []float64{25, 75}, true},
		{[]odtColumn{{72, 1000}, {216, 3000}}, []float64{72, 216}, false},
		{[]odtColumn{{0, 1000}, {0, 0}}, []float64{0, 0}, false},
		{nil, []float64{}, false},
	} {
		got, rel := odtColumnWidths(tt.columns)
		if rel != tt.rel || !slices.Equal(got, tt.want) {
			t.Errorf("odtColumnWidths(%v) = %v, %v, want %v, %v", tt.columns, got, rel, tt.want, tt.rel)
		}
	}
}

// TestFrameGradientFile checks the text frame of frame-gradient.odt, from
// 144 to 216 points down the page, with a linear gradient from red at the
// top to blue at the bottom. ivy drew no fill.
func TestFrameGradientFile(t *testing.T) {
	img := odtFilePage(t, "frame-gradient.odt", 0)
	for _, tt := range []struct {
		y    int
		want color.NRGBA
	}{
		{146, color.NRGBA{R: 247, B: 8, A: 255}},
		{180, color.NRGBA{R: 126, B: 129, A: 255}},
		{214, color.NRGBA{R: 4, B: 251, A: 255}},
	} {
		c := color.NRGBAModel.Convert(img.At(216, tt.y)).(color.NRGBA)
		near := func(a, b uint8) bool { return int(a) >= int(b)-8 && int(a) <= int(b)+8 }
		if !near(c.R, tt.want.R) || !near(c.G, tt.want.G) || !near(c.B, tt.want.B) {
			t.Errorf("expected %v at %d points down the page, got %v", tt.want, tt.y, c)
		}
	}
}
