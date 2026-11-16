package layout

import (
	"context"
	"image"
	"image/color"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy"
)

// The tests of this file come from the faults that the visual review found
// in the documents of Office, which the layout lays out as HTML.

// reviewBook returns a book of the document doc.
func reviewBook(doc string) *Book {
	fsys := fstest.MapFS{"a.html": &fstest.MapFile{Data: []byte(doc)}}
	return NewBook(Source{FS: fsys, Documents: []string{"a.html"}}, BookOptions{})
}

// reviewPage renders page n of the book at 72 dots per inch, so that a
// pixel is a point.
func reviewPage(t *testing.T, b *Book, n int) image.Image {
	t.Helper()
	p, err := b.Page(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(context.Background(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// pixelAt returns the color of the pixel of img at (x, y).
func pixelAt(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

// darkIn reports whether the rectangle r of img holds a dark pixel.
func darkIn(img image.Image, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if c := pixelAt(img, x, y); c.R < 100 && c.G < 100 && c.B < 100 {
				return true
			}
		}
	}
	return false
}

// TestSplitRowHeight checks that a row with a height splits at the end of a
// page, as Word and LibreOffice split it, and does not move whole to the
// next page.
func TestSplitRowHeight(t *testing.T) {
	b := reviewBook(`<!DOCTYPE html><html><head><style>@page { size: 200pt 200pt; margin: 10pt }
p { margin: 0; font-size: 12pt; line-height: 14pt } td { padding: 0 }</style></head><body>
<div style="height: 150pt"></div>
<table style="border-collapse: collapse; width: 100%"><tr style="height: 45pt"><td><p>one</p><p>two</p><p>three</p></td></tr></table>
</body></html>`)
	n, err := b.PageCount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 pages, got %d", n)
	}
	// the first line of the row is on the first page, below the block
	if !darkIn(reviewPage(t, b, 0), image.Rect(10, 160, 100, 175)) {
		t.Error("expected the first line of the row on the first page")
	}
}

// TestMarginBoxBehind checks that a box of a margin box with a negative
// z-index draws behind the content of the page, as the watermark of a
// header of Word does.
func TestMarginBoxBehind(t *testing.T) {
	b := reviewBook(`<!DOCTYPE html><html><head><style>@page { size: 200pt 200pt; margin: 40pt; @top-center { content: element(h); width: 100% } }
</style></head><body>
<div style="position: running(h)"><div style="position: absolute; left: 0; top: 60pt; width: 100pt; height: 60pt; background: #00ff00; z-index: -1"></div></div>
<div style="height: 50pt; background: #000000"></div>
</body></html>`)
	img := reviewPage(t, b, 0)
	// the black block of the page is over the green box
	if c := pixelAt(img, 60, 70); c.G > 60 {
		t.Errorf("expected the black block over the green box at (60, 70), got %v", c)
	}
	// the green box shows below the black block
	if c := pixelAt(img, 60, 110); c.G < 200 || c.R > 60 {
		t.Errorf("expected the green box at (60, 110), got %v", c)
	}
}

// TestJustifyPreWrap checks that a text of white-space: pre-wrap is
// justified, as the documents of Word are.
func TestJustifyPreWrap(t *testing.T) {
	b := reviewBook(`<!DOCTYPE html><html><head><style>@page { size: 300pt 200pt; margin: 20pt }
p { margin: 0; font-size: 12pt; text-align: justify; white-space: pre-wrap }</style></head><body>
<p>Some text that is long enough to wrap onto more than one line in the paragraph so that it shows justification.</p>
</body></html>`)
	img := reviewPage(t, b, 0)
	// the first line reaches the right edge of the page area, at 280
	if !darkIn(img, image.Rect(272, 20, 280, 36)) {
		t.Error("expected the first line to reach the right edge")
	}
}

// TestCellAbsolute checks that a box of a cell that is absolutely
// positioned at the top of the cell stays there when the cell aligns its
// content to the bottom.
func TestCellAbsolute(t *testing.T) {
	b := reviewBook(`<!DOCTYPE html><html><head><style>@page { size: 200pt 200pt; margin: 10pt }</style></head><body>
<table style="border-collapse: collapse; width: 100pt"><tr style="height: 150pt"><td style="position: relative; vertical-align: bottom; padding: 0">
<div style="position: absolute; left: 0; top: 0; width: 50pt; height: 20pt; background: #000000"></div></td></tr></table>
</body></html>`)
	img := reviewPage(t, b, 0)
	if !darkIn(img, image.Rect(15, 12, 50, 28)) {
		t.Error("expected the box at the top of the cell")
	}
	if darkIn(img, image.Rect(15, 140, 50, 160)) {
		t.Error("expected no box at the bottom of the cell")
	}
}
