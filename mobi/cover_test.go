package mobi

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

func TestCoverPage(t *testing.T) {
	cover := fixture.PNG(60, 90, color.Black)
	const chapter = `<p>The first chapter of the book, which is long enough to be more than a page of the cover.</p>`
	for name, tt := range map[string]struct {
		text   string
		images [][]byte
		exth   []exthRecord
		// docs are the documents of the layout, and cover is the page
		// that the cover fills, or -1.
		docs  []string
		cover int
	}{
		"cover": {
			text:   `<html><body>` + chapter + `</body></html>`,
			images: [][]byte{cover},
			exth:   []exthRecord{exthNumber(exthCover, 0)},
			docs:   []string{coverPage, "index.html"},
			cover:  0,
		},
		"no cover": {
			text:   `<html><body>` + chapter + `</body></html>`,
			images: [][]byte{cover},
			docs:   []string{"index.html"},
			cover:  -1,
		},
		"cover out of the book": {
			text:   `<html><body>` + chapter + `</body></html>`,
			images: [][]byte{cover},
			exth:   []exthRecord{exthNumber(exthCover, 7)},
			docs:   []string{"index.html"},
			cover:  -1,
		},
		"text that starts with the cover": {
			text:   `<html><body><img recindex="00001">` + chapter + `</body></html>`,
			images: [][]byte{cover},
			exth:   []exthRecord{exthNumber(exthCover, 0)},
			docs:   []string{"index.html"},
			cover:  -1,
		},
		"text that is the cover": {
			text:   `<html><body><p><img recindex="00001"></p></body></html>`,
			images: [][]byte{cover},
			exth:   []exthRecord{exthNumber(exthCover, 0)},
			docs:   []string{coverPage, "index.html"},
			cover:  0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			buf := testBook{text: []byte(tt.text), images: tt.images, exth: tt.exth, fullName: "Cover"}.build()
			d, err := Open(t.Context(), bytes.NewReader(buf))
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
			pages := 0
			for _, err := range ivy.Pages(t.Context(), d) {
				if err != nil {
					t.Fatal(err)
				}
				pages++
			}
			if pages != n || n < len(tt.docs) {
				t.Errorf("expected %d pages from ivy.Pages, a page for each document at least, got: %d", n, pages)
			}
			for i := range n {
				ink := ebooktest.Ink(ebooktest.RenderPage(t, d, i), image.Rect(0, 0, 576, 864))
				if full := ink > 576*864*9/10; full != (i == tt.cover) {
					t.Errorf("page %d: expected the cover to fill the page %t, got %d pixels of ink", i, i == tt.cover, ink)
				}
			}
		})
	}
}
