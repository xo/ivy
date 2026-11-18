package layout

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fontdb"
)

// bookStyle sets a small page, as the style sheet of a format does.
const bookStyle = `@page { size: 100px 80px; margin: 10px } body { margin: 0; font-size: 10px }`

// testBook returns a book of n documents of one short paragraph each, and
// the documents in bad that fail to parse.
func testBook(n int, bad map[int]bool) Source {
	fsys := fstest.MapFS{}
	docs := make([]string, 0, n)
	for i := range n {
		name := fmt.Sprintf("c%d.xhtml", i)
		body := fmt.Sprintf(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Chapter %d</p></body></html>`, i)
		if bad[i] {
			body = "<html><body><p>not closed"
		}
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
		docs = append(docs, name)
	}
	return Source{FS: fsys, Documents: docs, StyleSheets: []string{bookStyle}}
}

// inked reports whether a page has a pixel that is not white.
func inked(t *testing.T, p ivy.Page) bool {
	t.Helper()
	img, err := ivy.RenderImage(context.Background(), p, 96, color.White)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r < 0x8000 && g < 0x8000 && bl < 0x8000 {
				return true
			}
		}
	}
	return false
}

func TestBookLazy(t *testing.T) {
	ctx := context.Background()
	b := NewBook(testBook(50, nil), BookOptions{})
	p, err := b.Page(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if b.loads != 1 {
		t.Errorf("page 1 loaded %d documents, want 1", b.loads)
	}
	if w, h := p.Size(); w != 75 || h != 60 {
		t.Errorf("size = %g×%g, want 75×60", w, h)
	}
	if !inked(t, p) {
		t.Error("page 1 has no ink")
	}
	if _, err := b.Page(ctx, 4); err != nil {
		t.Fatal(err)
	}
	if b.loads != 5 {
		t.Errorf("page 5 loaded %d documents, want 5", b.loads)
	}
	n, err := b.PageCount(ctx)
	if err != nil || n != 50 {
		t.Fatalf("PageCount = %d, %v, want 50", n, err)
	}
	if len(b.loaded) > maxLoadedDocuments {
		t.Errorf("%d documents loaded, want at most %d", len(b.loaded), maxLoadedDocuments)
	}
	// A document that was dropped is laid out again.
	if _, err := b.Page(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Page(ctx, 50); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("page 51: %v, want ErrPageRange", err)
	}
	if _, err := b.Page(ctx, -1); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("page -1: %v, want ErrPageRange", err)
	}
}

func TestBookBadDocument(t *testing.T) {
	ctx := context.Background()
	src := testBook(3, map[int]bool{1: true})
	// A file that does not exist fails too.
	src.Documents = append(src.Documents, "missing.xhtml")
	b := NewBook(src, BookOptions{})
	n, err := b.PageCount(ctx)
	if err != nil || n != 2 {
		t.Fatalf("PageCount = %d, %v, want 2", n, err)
	}
	if _, err := b.Page(ctx, 1); err != nil {
		t.Fatal(err)
	}

	b = NewBook(Source{FS: fstest.MapFS{}, Documents: []string{"missing.xhtml"}}, BookOptions{})
	if _, err := b.Page(ctx, 0); err == nil || errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("book without a good document: %v, want the error of the document", err)
	}
	if _, err := b.PageCount(ctx); err == nil {
		t.Error("PageCount of a book without a good document gives no error")
	}
}

func TestBookCanceled(t *testing.T) {
	b := NewBook(testBook(2, nil), BookOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.Page(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Errorf("Page: %v, want context.Canceled", err)
	}
	if _, err := b.PageCount(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("PageCount: %v, want context.Canceled", err)
	}
	p, err := b.Page(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	dst, _ := okage.NewPixmap(10, 10)
	if err := p.Render(ctx, dst, okage.IdentityTransform()); !errors.Is(err, context.Canceled) {
		t.Errorf("Render: %v, want context.Canceled", err)
	}
}

func TestBookConcurrent(t *testing.T) {
	b := NewBook(testBook(4, nil), BookOptions{})
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range 4 {
		wg.Go(func() {
			p, err := b.Page(context.Background(), i)
			if err != nil {
				errs[i] = err
				return
			}
			_, errs[i] = ivy.RenderImage(context.Background(), p, 96, color.White)
		})
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("page %d: %v", i+1, err)
		}
	}
}

func TestBookPages(t *testing.T) {
	// A long document makes several pages, and the next document starts on
	// a page of its own.
	fsys := fstest.MapFS{
		"a.html": {Data: []byte("<p>" + strings.Repeat("word ", 300) + "</p>")},
		"b.html": {Data: []byte("<p>end</p>")},
	}
	b := NewBook(Source{FS: fsys, Documents: []string{"a.html", "b.html"}, StyleSheets: []string{bookStyle}}, BookOptions{})
	n, err := b.PageCount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Fatalf("PageCount = %d, want 3 or more", n)
	}
	last, err := b.Page(context.Background(), n-1)
	if err != nil {
		t.Fatal(err)
	}
	if !inked(t, last) {
		t.Error("the last page has no ink")
	}
}

// TestBookSystemFonts checks that a book with BookOptions.SystemFonts draws
// its text with a database of its own, which holds the default fonts.
func TestBookSystemFonts(t *testing.T) {
	b := NewBook(testBook(1, nil), BookOptions{SystemFonts: true})
	if b.fc == nil || fontdb.System() == fontdb.Default() {
		t.Fatal("expected the database of the fonts of the system")
	}
	p, err := b.Page(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !inked(t, p) {
		t.Error("expected ink on the page")
	}
}

// TestBookDraw checks that Book.Draw sends the glyphs of the text to the
// device, and that the commands draw the same pixels as Render, at a scale
// of the device.
func TestBookDraw(t *testing.T) {
	b := NewBook(testBook(2, nil), BookOptions{})
	var l display.List
	if err := b.Draw(context.Background(), 1, &l, okage.IdentityTransform(), 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(l.String(), "glyphs ") {
		t.Errorf("expected the glyphs of the text, got:\n%s", l.String())
	}
	if err := b.Draw(context.Background(), 2, &l, okage.IdentityTransform(), 1); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ivy.ErrPageRange past the last page, got %v", err)
	}
	// Render at 2 pixels a point, and Draw onto a device that scales its
	// user space of points by 2.
	p, err := b.Page(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	w, h := p.Size()
	want, _ := okage.NewPixmap(uint32(2*w), uint32(2*h))
	want.Fill(okage.ColorWhite)
	if err := p.Render(context.Background(), want, okage.TransformFromScale(2, 2)); err != nil {
		t.Fatal(err)
	}
	got, _ := okage.NewPixmap(uint32(2*w), uint32(2*h))
	got.Fill(okage.ColorWhite)
	dev := display.NewOkage(got, okage.TransformFromScale(2, 2))
	if err := b.Draw(context.Background(), 1, dev, okage.IdentityTransform(), 2); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Data(), want.Data()) {
		t.Error("Draw does not draw the pixels of Render")
	}
}

// bookMarginText returns the text of the margin box @top-center of page n
// of a book.
func bookMarginText(t *testing.T, b *Book, n int) string {
	t.Helper()
	p, err := b.Page(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for _, child := range p.(*bookPage).box.Children {
		if child.AtKeyword == "@top-center" {
			child.Descendants(func(d *Box) bool {
				if d.Kind == KindText {
					text += d.Text
				}
				return true
			})
		}
	}
	return text
}

// TestBookPagesCounter checks that counter(pages) counts the pages of the
// whole book (D43).
func TestBookPagesCounter(t *testing.T) {
	src := testBook(3, nil)
	src.FS.(fstest.MapFS)["c0.xhtml"] = &fstest.MapFile{Data: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body>` +
		`<p>One</p><p style="break-before: page">Two</p></body></html>`)}
	src.StyleSheets = append(src.StyleSheets, `@page { @top-center { content: counter(page) "/" counter(pages) } }`)
	b := NewBook(src, BookOptions{})
	if got := bookMarginText(t, b, 0); got != "1/4" {
		t.Errorf("page 1: got %q, want 1/4", got)
	}
	if b.loads != 3 {
		t.Errorf("page 1 loaded %d documents, want 3", b.loads)
	}
	for _, n := range []int{3, 1, 2} {
		if got, want := bookMarginText(t, b, n), fmt.Sprintf("%d/4", n+1); got != want {
			t.Errorf("page %d: got %q, want %q", n+1, got, want)
		}
	}
	if n, err := b.PageCount(context.Background()); err != nil || n != 4 {
		t.Fatalf("got %d pages and %v, want 4", n, err)
	}

	// Only the last document uses counter(pages): the first page stays
	// lazy.
	src = testBook(3, nil)
	src.FS.(fstest.MapFS)["c2.xhtml"] = &fstest.MapFile{Data: []byte(`<html xmlns="http://www.w3.org/1999/xhtml">` +
		`<head><style>@page { @top-center { content: counter(pages) } }</style></head><body><p>Three</p></body></html>`)}
	b = NewBook(src, BookOptions{})
	if got := bookMarginText(t, b, 0); got != "" {
		t.Errorf("page 1: got %q, want no text", got)
	}
	if b.loads != 1 {
		t.Errorf("page 1 loaded %d documents, want 1", b.loads)
	}
	if got := bookMarginText(t, b, 2); got != "3" {
		t.Errorf("page 3: got %q, want 3", got)
	}
}

// TestBookPageCounter checks that the counter page goes on across the
// documents of a book, in any order of the calls.
func TestBookPageCounter(t *testing.T) {
	src := testBook(3, nil)
	src.FS.(fstest.MapFS)["c0.xhtml"] = &fstest.MapFile{Data: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body>` +
		`<p>One</p><p style="break-before: page">Two</p></body></html>`)}
	src.StyleSheets = append(src.StyleSheets, `@page { @top-center { content: counter(page) } }`)
	number := func(b *Book, n int) string {
		t.Helper()
		p, err := b.Page(context.Background(), n)
		if err != nil {
			t.Fatal(err)
		}
		var text string
		for _, child := range p.(*bookPage).box.Children {
			if child.AtKeyword == "@top-center" {
				child.Descendants(func(d *Box) bool {
					if d.Kind == KindText {
						text += d.Text
					}
					return true
				})
			}
		}
		return text
	}
	b := NewBook(src, BookOptions{})
	for _, n := range []int{3, 0, 1, 2} {
		if got, want := number(b, n), fmt.Sprint(n+1); got != want {
			t.Errorf("page %d: got the number %q, want %q", n, got, want)
		}
	}
	b = NewBook(src, BookOptions{})
	if n, err := b.PageCount(context.Background()); err != nil || n != 4 {
		t.Fatalf("got %d pages and %v, want 4", n, err)
	}
	if got := number(b, 2); got != "3" {
		t.Errorf("page 2 after the count: got the number %q, want 3", got)
	}
}

// TestRenderReplay checks that Render, which records a page once and
// replays the recording, gives the pixels of a drawing of the page at the
// same transform, also when goroutines render the bands of the page at the
// same time (D47).
func TestRenderReplay(t *testing.T) {
	ctx := context.Background()
	style := `@page { size: 300px 400px; margin: 20px } body { margin: 0; font-size: 14px }
	.box { border: 2px solid #c33; padding: 6px; background: linear-gradient(#fff, #9cf) }
	.faded { opacity: .5; background: #3a6 } .masked { mask-image: linear-gradient(black, transparent) }`
	body := `<html xmlns="http://www.w3.org/1999/xhtml"><body>
	<p class="box">A paragraph in a box with a gradient.</p>
	<p class="faded">A paragraph at half opacity.</p>
	<p class="masked" style="background: #c60">A paragraph under a mask.</p>
	<svg xmlns="http://www.w3.org/2000/svg" width="120" height="60"><circle cx="30" cy="30" r="25" fill="#36c"/><rect x="60" y="10" width="50" height="40" fill="#e90"/></svg>
	<p>` + strings.Repeat("Text that fills the page. ", 30) + `</p>
	</body></html>`
	src := Source{
		FS:          fstest.MapFS{"c.xhtml": &fstest.MapFile{Data: []byte(body)}},
		Documents:   []string{"c.xhtml"},
		StyleSheets: []string{style},
	}
	b := NewBook(src, BookOptions{})
	page, err := b.Page(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	p := page.(*bookPage)
	const scale = 96.0 / 72
	// the bands of 128 rows of a page of 400 by 533 pixels, as RenderImage
	// cuts them, and a transform with a fraction of a pixel
	transforms := []okage.Transform{okage.TransformFromRow(scale, 0, 0, scale, 0.25, -0.5)}
	for y := 0; y < 533; y += 128 {
		transforms = append(transforms, okage.TransformFromRow(scale, 0, 0, scale, 0, -float32(y)))
	}
	pixmap := func() *okage.Pixmap {
		pm, ok := okage.NewPixmap(400, 160)
		if !ok {
			t.Fatal("making a pixmap")
		}
		return pm
	}
	want := make([][]byte, len(transforms))
	for i, tr := range transforms {
		pm := pixmap()
		if err := p.draw(ctx, display.NewOkage(pm, okage.IdentityTransform()), tr, 1); err != nil {
			t.Fatal(err)
		}
		want[i] = pm.Data()
	}
	got := make([][]byte, len(transforms))
	errs := make([]error, len(transforms))
	var wg sync.WaitGroup
	for i, tr := range transforms {
		wg.Go(func() {
			pm := pixmap()
			errs[i] = p.Render(ctx, pm, tr)
			got[i] = pm.Data()
		})
	}
	wg.Wait()
	for i := range transforms {
		if errs[i] != nil {
			t.Fatalf("transform %d: %v", i, errs[i])
		}
		if !slices.Equal(got[i], want[i]) {
			t.Errorf("transform %d: expected the pixels of the drawing at the same transform", i)
		}
	}
	if n := len(b.recordings); n != 2 {
		t.Errorf("expected 2 recordings, one for each fraction of a pixel, got %d", n)
	}
}
