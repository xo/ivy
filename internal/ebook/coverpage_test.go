package ebook

import (
	"context"
	"image"
	"image/color"
	"path"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/layout"
)

// imageBook returns the pages of a book whose only document is the image
// page of an image of w by h pixels, with the style sheet of an ebook.
func imageBook(t *testing.T, w, h int) (*Pages, Source) {
	t.Helper()
	page := "images/cover-page.xhtml"
	fsys := fstest.MapFS{
		"images/a cover.png": &fstest.MapFile{Data: fixture.PNG(w, h, color.Black)},
		page:                 &fstest.MapFile{Data: ImagePage(ImagePageLink(page, "images/a cover.png"), "A & B", ivy.Paper{})},
	}
	src := func(context.Context) (layout.Source, error) {
		return layout.Source{FS: fsys, Documents: []string{page}, StyleSheets: []string{BookStyle}}, nil
	}
	return new(Pages), src
}

func TestImagePage(t *testing.T) {
	for name, tt := range map[string]struct {
		w, h int
		// ink is the part of the page that the image covers, in pixels
		// of 96 dpi on a page of 576 by 864 pixels.
		ink image.Rectangle
	}{
		"same shape": {60, 90, image.Rect(0, 0, 576, 864)},
		"wide":       {200, 100, image.Rect(0, 288, 576, 576)},
		"tall":       {100, 300, image.Rect(144, 0, 432, 864)},
	} {
		t.Run(name, func(t *testing.T) {
			pages, src := imageBook(t, tt.w, tt.h)
			n, err := pages.PageCount(t.Context(), src, ivy.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Fatalf("expected one page, got: %d", n)
			}
			p, err := pages.Page(t.Context(), src, ivy.Config{}, 0)
			if err != nil {
				t.Fatal(err)
			}
			if w, h := p.Size(); w != 6*72 || h != 9*72 {
				t.Errorf("expected a page of 6 by 9 inches, got: %g by %g", w, h)
			}
			img, err := ivy.RenderImage(t.Context(), p, 96, color.White)
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds() != image.Rect(0, 0, 576, 864) {
				t.Fatalf("expected an image of 576 by 864 pixels, got: %v", img.Bounds())
			}
			// a pixel inside the image is black, and a pixel outside it
			// is white, with a margin of 3 pixels for the edges
			in := tt.ink.Inset(3)
			out := 0
			for y := range 864 {
				for x := range 576 {
					dark := gray(img.At(x, y)) < 0x4000
					inside := image.Pt(x, y).In(in)
					outside := !image.Pt(x, y).In(tt.ink.Inset(-3))
					switch {
					case inside && !dark, outside && dark:
						out++
					}
				}
			}
			if out > 0 {
				t.Errorf("expected the image scaled to fit in %v, got %d pixels out of place", tt.ink, out)
			}
		})
	}
}

// gray returns the luminance of c, from 0 to 0xffff.
func gray(c color.Color) uint32 {
	r, g, b, _ := c.RGBA()
	return (r + g + b) / 3
}

func TestImagePageLink(t *testing.T) {
	for _, tt := range []struct {
		page, name, want string
	}{
		{"cover-page.xhtml", "images/a b.jpg", "./images/a%20b.jpg"},
		{"OEBPS/cover-page.xhtml", "OEBPS/images/cover.jpg", "./images/cover.jpg"},
		{"OEBPS/Images/p1-page.xhtml", "OEBPS/Images/p1.svg", "./p1.svg"},
		{"a/b/page.xhtml", "c/d.png", "./../../c/d.png"},
		{"page.xhtml", "c:d.png", "./c:d.png"},
	} {
		if got := ImagePageLink(tt.page, tt.name); got != tt.want {
			t.Errorf("ImagePageLink(%q, %q): expected %q, got: %q", tt.page, tt.name, tt.want, got)
		}
		if got, ok := Resolve(path.Dir(tt.page), ImagePageLink(tt.page, tt.name)); !ok || got != tt.name {
			t.Errorf("expected the link of %q from %q to resolve to it, got: %q", tt.name, tt.page, got)
		}
	}
}

func TestImagePageName(t *testing.T) {
	taken := map[string]bool{"a/b-page.xhtml": true, "a/b-page-1.xhtml": true}
	exists := func(name string) bool { return taken[name] }
	if got := ImagePageName("a/b.svg", exists); got != "a/b-page-2.xhtml" {
		t.Errorf("expected a free name, got: %q", got)
	}
	if got := ImagePageName("c.png", exists); got != "c-page.xhtml" {
		t.Errorf("expected the first name, got: %q", got)
	}
	always := func(string) bool { return true }
	if got := ImagePageName("c.png", always); got == "" {
		t.Error("expected a name when each name is taken")
	}
}

func TestIsCoverPage(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 10 10"><image xlink:href="cover.jpg" width="10" height="10"/></svg>`
	fsys := fstest.MapFS{
		"text/page.xhtml":  {Data: []byte(`<html><head><title>A long title of the cover page</title><style>p { color: red }</style></head><body><h1 class="hidden">Cover</h1><div><img src="../img/cover.jpg" alt="The cover of the book"/></div></body></html>`)},
		"text/svg.xhtml":   {Data: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><image xlink:href="../img/cover.jpg"/></svg></body></html>`)},
		"text/title.xhtml": {Data: []byte(`<html><body><p>A</p><img src="../img/cover.jpg"/><h1>The Title of the Book</h1><p>By an author of many books, with a long name and a preface, and a list of the other books of the author.</p></body></html>`)},
		"text/other.xhtml": {Data: []byte(`<html><body><img src="../img/other.jpg"/></body></html>`)},
		"text/none.xhtml":  {Data: []byte(`<html><body><p>Text</p></body></html>`)},
		"text/late.xhtml":  {Data: []byte(`<html><body><p>The first chapter of the book starts here, with much more text than a page of the cover can hold.</p><img src="../img/cover.jpg"/></body></html>`)},
		"text/flow.xhtml":  {Data: []byte(`<html><body><img src="../img/cover.svg"/></body></html>`)},
		"img/cover.svg":    {Data: []byte(svg)},
	}
	for _, tt := range []struct {
		name        string
		page, shows bool
	}{
		{"text/page.xhtml", true, true},
		{"text/svg.xhtml", true, true},
		{"text/title.xhtml", false, true},
		{"text/other.xhtml", false, false},
		{"text/none.xhtml", false, false},
		{"text/late.xhtml", false, false},
		{"text/flow.xhtml", true, true},
		{"text/missing.xhtml", false, false},
	} {
		page, shows := IsCoverPage(fsys, tt.name, "img/cover.jpg", ivy.Limits{})
		if page != tt.page || shows != tt.shows {
			t.Errorf("%s: expected %t %t, got: %t %t", tt.name, tt.page, tt.shows, page, shows)
		}
	}
}

func TestImagePageTitle(t *testing.T) {
	page := ImagePage("./a.png", "A \x01 <b> & \ufffe", ivy.Paper{})
	if _, err := ParseXML(page, ivy.Limits{}); err != nil {
		t.Errorf("expected well formed XML, got: %v\n%s", err, page)
	}
}
