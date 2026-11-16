package layout

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/xo/okage"

	"github.com/xo/ivy"
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
