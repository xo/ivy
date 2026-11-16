package epub

import (
	"bytes"
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook/ebooktest"
	"github.com/xo/ivy/internal/fixture"
)

// The parts of the books of the tests of the cover.
const (
	coverItem   = `<item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/>` + "\n"
	chapterItem = `<item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>` + "\n"
	chapterRef  = `<itemref idref="ch1"/>` + "\n"
	square      = `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><rect width="100" height="100" fill="#000"/></svg>`
)

func TestCoverPage(t *testing.T) {
	cover := fixture.Entry{Name: "cover.png", Data: fixture.PNG(60, 90, color.Black)}
	chapter := fixture.Entry{Name: "ch1.xhtml", Data: fixture.XHTML(`<h1>One</h1>`)}
	for name, tt := range map[string]struct {
		pkg     fixture.EPUBPackage
		entries []fixture.Entry
		// docs are the documents of the layout, and cover is the page
		// that the cover fills, or -1.
		docs  []string
		cover int
	}{
		"text first": {
			pkg:     fixture.EPUBPackage{Manifest: coverItem + chapterItem, Spine: chapterRef},
			entries: []fixture.Entry{cover, chapter},
			docs:    []string{"OEBPS/cover-page.xhtml", "OEBPS/ch1.xhtml"},
			cover:   0,
		},
		"page of the cover": {
			pkg: fixture.EPUBPackage{
				Manifest: coverItem + chapterItem + `<item id="cp" href="text/cover.xhtml" media-type="application/xhtml+xml"/>`,
				Spine:    `<itemref idref="cp"/>` + chapterRef,
			},
			entries: []fixture.Entry{cover, chapter, {Name: "text/cover.xhtml", Data: fixture.XHTML(`<div><img src="../cover.png" alt="Cover"/></div>`)}},
			// the page of the book stays after the page of the cover, as
			// in calibre
			docs:  []string{"OEBPS/cover-page.xhtml", "OEBPS/text/cover.xhtml", "OEBPS/ch1.xhtml"},
			cover: 0,
		},
		"title page": {
			pkg: fixture.EPUBPackage{
				Manifest: coverItem + chapterItem + `<item id="tp" href="title.xhtml" media-type="application/xhtml+xml"/>`,
				Spine:    `<itemref idref="tp"/>` + chapterRef,
			},
			entries: []fixture.Entry{cover, chapter, {Name: "title.xhtml", Data: fixture.XHTML(
				`<img src="cover.png" width="60"/><h1>The Title of the Book</h1><p>By an author with a long name, and a list of the other books of the author.</p>`)}},
			docs:  []string{"OEBPS/title.xhtml", "OEBPS/ch1.xhtml"},
			cover: -1,
		},
		"cover of the guide": {
			pkg: fixture.EPUBPackage{
				Metadata: `<meta name="cover" content="cover"/>`,
				Manifest: coverItem + chapterItem + `<item id="front" href="front.xhtml" media-type="application/xhtml+xml"/>`,
				Spine:    `<itemref idref="front"/>` + chapterRef,
				Guide:    `<reference type="cover" href="front.xhtml"/>`,
			},
			entries: []fixture.Entry{cover, chapter, {Name: "front.xhtml", Data: fixture.XHTML(`<h1>Front</h1>`)}},
			docs:    []string{"OEBPS/front.xhtml", "OEBPS/ch1.xhtml"},
			cover:   -1,
		},
		"cover in the spine": {
			pkg:     fixture.EPUBPackage{Manifest: coverItem + chapterItem, Spine: `<itemref idref="cover"/>` + chapterRef},
			entries: []fixture.Entry{cover, chapter},
			docs:    []string{"OEBPS/cover-page.xhtml", "OEBPS/ch1.xhtml"},
			cover:   0,
		},
		"no cover": {
			pkg:     fixture.EPUBPackage{Manifest: chapterItem, Spine: chapterRef},
			entries: []fixture.Entry{chapter},
			docs:    []string{"OEBPS/ch1.xhtml"},
			cover:   -1,
		},
		"cover that is missing": {
			pkg:     fixture.EPUBPackage{Manifest: coverItem + chapterItem, Spine: chapterRef},
			entries: []fixture.Entry{chapter},
			docs:    []string{"OEBPS/ch1.xhtml"},
			cover:   -1,
		},
		"images of svg": {
			pkg: fixture.EPUBPackage{
				Manifest: `<item id="p1" href="img/p1.svg" media-type="image/svg+xml" properties="cover-image"/>` +
					`<item id="p2" href="img/p2.svg" media-type="image/svg+xml"/>`,
				Spine: `<itemref idref="p1"/><itemref idref="p2"/>`,
			},
			entries: []fixture.Entry{{Name: "img/p1.svg", Data: []byte(square)}, {Name: "img/p2.svg", Data: []byte(square)}},
			docs:    []string{"OEBPS/img/p1-page.xhtml", "OEBPS/img/p2-page.xhtml"},
			cover:   -1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(fixture.EPUB(tt.pkg, tt.entries...)))
			if err != nil {
				t.Fatal(err)
			}
			src, err := d.source()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(src.Documents, tt.docs) {
				t.Fatalf("expected the documents %q, got: %q", tt.docs, src.Documents)
			}
			n, err := d.PageCount(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if n != len(tt.docs) {
				t.Errorf("expected a page for each document, got %d pages", n)
			}
			pages := 0
			for _, err := range ivy.Pages(t.Context(), d) {
				if err != nil {
					t.Fatal(err)
				}
				pages++
			}
			if pages != n {
				t.Errorf("expected %d pages from ivy.Pages, got: %d", n, pages)
			}
			for i := range n {
				ink := ebooktest.Ink(ebooktest.RenderPage(t, d, i), image.Rect(0, 0, 576, 864))
				full := ink > 576*864*9/10
				if full != (i == tt.cover) {
					t.Errorf("page %d: expected the cover to fill the page %t, got %d pixels of ink", i, i == tt.cover, ink)
				}
				if ink == 0 {
					t.Errorf("page %d: expected ink", i)
				}
			}
		})
	}
}

// TestSVGPage checks that an image of SVG in the spine is scaled to fit its
// page.
func TestSVGPage(t *testing.T) {
	pkg := fixture.EPUBPackage{
		Manifest: `<item id="p1" href="p1.svg" media-type="image/svg+xml"/>`,
		Spine:    `<itemref idref="p1"/>`,
	}
	d, err := Open(t.Context(), bytes.NewReader(fixture.EPUB(pkg, fixture.Entry{Name: "p1.svg", Data: []byte(square)})))
	if err != nil {
		t.Fatal(err)
	}
	img := ebooktest.RenderPage(t, d, 0)
	// The square of SVG fills the width of the page, in the middle of its
	// height.
	if ink := ebooktest.Ink(img, image.Rect(0, 148, 576, 716)); ink < 576*560 {
		t.Errorf("expected the square in the middle of the page, got %d pixels of ink", ink)
	}
	if ink := ebooktest.Ink(img, image.Rect(0, 0, 576, 140)) + ebooktest.Ink(img, image.Rect(0, 724, 576, 864)); ink != 0 {
		t.Errorf("expected no ink above and below the square, got %d pixels", ink)
	}
}
