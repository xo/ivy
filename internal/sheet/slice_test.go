package sheet

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/layout"
)

// layers is an object that draws as a chart draws: a white fill of its
// whole box, and a blue fill over it, inset by a tenth of the box.
type layers struct {
	w, h float64
}

// Size satisfies the interface Object of the layout.
func (o layers) Size() (float64, float64) {
	return o.w, o.h
}

// Draw satisfies the interface Object of the layout.
func (o layers) Draw(_ context.Context, dev display.Device, rect okage.Rect) error {
	x, y, w, h := rect.X(), rect.Y(), rect.Width(), rect.Height()
	if r, ok := okage.RectFromXYWH(x, y, w, h); ok {
		dev.FillPath(okage.PathFromRect(r), okage.FillRuleWinding, okage.SolidColor(okage.ColorWhite))
	}
	if r, ok := okage.RectFromXYWH(x+w/10, y+h/10, w*8/10, h*8/10); ok {
		dev.FillPath(okage.PathFromRect(r), okage.FillRuleWinding, okage.SolidColor(okage.ColorFromRGBA8(0, 0, 255, 255)))
	}
	return nil
}

// TestSliceSeams draws an object that rows of a height of a part of a
// pixel cut, and counts the light pixels inside its blue fill. Where two
// rows meet, the slice of the lower row covers a part of a pixel. The fills
// of the object under the blue one must not show through there.
func TestSliceSeams(t *testing.T) {
	sh := &Sheet{Name: "Chart", DefaultColWidth: 48, DefaultRowHeight: 14.3, Page: DefaultPage}
	for i := range 30 {
		sh.Rows = append(sh.Rows, Row{Index: i})
	}
	sh.Images = []Image{{Row: 1, Col: 1, DX: 3, DY: 2, Width: 300, Height: 260, Object: "chart1"}}
	wb := &Workbook{Sheets: []*Sheet{sh}, Objects: map[string]layout.Object{"chart1": layers{300, 260}}}
	src, err := wb.Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	book := layout.NewBook(src, layout.BookOptions{FontFiles: officeFonts(t)})
	page, err := book.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, dpi := range []float32{72, 96, 150} {
		img, err := ivy.RenderImage(t.Context(), page, dpi, color.White)
		if err != nil {
			t.Fatal(err)
		}
		if out := os.Getenv("IVY_TEST_OUT"); out != "" {
			writePNG(t, filepath.Join(out, fmt.Sprintf("slices-%v.png", dpi)), img)
		}
		box := blueBox(img)
		if box.Dx() < 100 || box.Dy() < 100 {
			t.Fatalf("%v dpi: expected the blue fill, got: %v", dpi, box)
		}
		box = box.Inset(2)
		light := 0
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				if c := nrgba(img.At(x, y)); c.R > 2 || c.G > 2 || c.B < 253 {
					light++
				}
			}
		}
		if light != 0 {
			t.Errorf("%v dpi: expected no light pixel in the blue fill %v, got: %d", dpi, box, light)
		}
	}
}

// blueBox returns the bounds of the pure blue pixels of img.
func blueBox(img image.Image) image.Rectangle {
	var box image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c := nrgba(img.At(x, y)); c.R == 0 && c.G == 0 && c.B == 255 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}
