package docx

import (
	"bytes"
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
