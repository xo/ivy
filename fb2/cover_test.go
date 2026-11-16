package fb2

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook/ebooktest"
	"github.com/xo/ivy/internal/fixture"
)

func TestCoverPage(t *testing.T) {
	const xlink = `xmlns:l="http://www.w3.org/1999/xlink"`
	const body = `<section><p>The text of the book.</p></section>`
	cover := fixture.Binary{ID: "cover.png", ContentType: "image/png", Data: fixture.PNG(60, 90, color.Black)}
	for name, tt := range map[string]struct {
		titleInfo string
		binaries  []fixture.Binary
		docs      []string
	}{
		"cover":          {`<coverpage><image l:href="#cover.png"/></coverpage>`, []fixture.Binary{cover}, []string{coverName, documentName}},
		"no cover":       {``, []fixture.Binary{cover}, []string{documentName}},
		"missing binary": {`<coverpage><image l:href="#other.png"/></coverpage>`, []fixture.Binary{cover}, []string{documentName}},
		"bad binary": {`<coverpage><image l:href="#cover.png"/></coverpage>`,
			[]fixture.Binary{{ID: "cover.png", ContentType: "image/png", Data: []byte("not an image")}}, []string{documentName}},
	} {
		t.Run(name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(fixture.FictionBook(xlink, tt.titleInfo, body, tt.binaries...)))
			if err != nil {
				t.Fatal(err)
			}
			src, err := d.source(t.Context())
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
			if _, err := d.Page(t.Context(), n); !errors.Is(err, ivy.ErrPageRange) {
				t.Errorf("expected ivy.ErrPageRange, got: %v", err)
			}
			if len(tt.docs) == 1 {
				return
			}
			if ink := ebooktest.Ink(ebooktest.RenderPage(t, d, 0), image.Rect(0, 0, 576, 864)); ink < 576*864*9/10 {
				t.Errorf("expected the cover to fill the first page, got %d pixels of ink", ink)
			}
		})
	}
}

// TestUndeclaredPrefix opens books that use the prefixes l: and xlink:, and
// a prefix of no known namespace, without a declaration, as some
// FictionBook files do.
func TestUndeclaredPrefix(t *testing.T) {
	cover := fixture.Binary{ID: "c.png", ContentType: "image/png", Data: fixture.PNG(6, 9, color.Black)}
	for name, attrs := range map[string]string{
		"none":     ``,
		"xlink":    `xmlns:xlink="http://www.w3.org/1999/xlink"`,
		"prefix l": `xmlns:l="http://www.w3.org/1999/xlink"`,
	} {
		t.Run(name, func(t *testing.T) {
			body := `<section><p>Text<a type="note" l:href="#n1">[1]</a> and <a xlink:href="#n1">[2]</a>.</p><image xlink:href="#c.png"/><p foo:style="x">Odd</p></section>`
			buf := fixture.FictionBook(attrs, `<coverpage><image l:href="#c.png"/></coverpage>`, body, cover)
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			src, err := d.source(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(src.Documents, []string{coverName, documentName}) {
				t.Errorf("expected the page of the cover, got: %q", src.Documents)
			}
			doc, err := fs.ReadFile(src.FS, documentName)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{`<a class="note" href="#n1">[1]</a>`, `<a href="#n1">[2]</a>`, `<img src="images/0.png"`, `Odd`} {
				if !strings.Contains(string(doc), want) {
					t.Errorf("expected %q in the document, got:\n%s", want, doc)
				}
			}
		})
	}
}
