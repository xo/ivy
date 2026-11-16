package layout

import (
	"context"
	"image"
	"image/color"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fixture"
)

// boxObject is an object that fills its rectangle with red.
type boxObject struct {
	width, height float64
	draws         atomic.Int32
	rect          okage.Rect
}

func (o *boxObject) Size() (float64, float64) {
	return o.width, o.height
}

func (o *boxObject) Draw(_ context.Context, dev display.Device, rect okage.Rect) error {
	o.draws.Add(1)
	o.rect = rect
	dev.FillPath(okage.PathFromRect(rect), okage.FillRuleWinding, okage.SolidColor(okage.ColorFromRGBA8(255, 0, 0, 255)))
	return nil
}

// renderSource renders page 1 of a book of src at 96 dots per inch.
func renderSource(t *testing.T, src Source) image.Image {
	t.Helper()
	b := NewBook(src, BookOptions{Logger: testLogger()})
	p, err := b.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 96, color.White)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// colorBox returns the box of the pixels of img that are near c.
func colorBox(img image.Image, c color.NRGBA) image.Rectangle {
	var box image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			near := func(v uint32, w uint8) bool {
				d := int(v>>8) - int(w)
				return d > -40 && d < 40
			}
			if near(r, c.R) && near(g, c.G) && near(bl, c.B) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

func TestObjectImage(t *testing.T) {
	obj := &boxObject{width: 30, height: 15}
	src := Source{
		FS: fstest.MapFS{"a.html": &fstest.MapFile{Data: []byte(
			`<p><img src="` + ObjectURL("chart1") + `"/></p>` +
				`<p><object data="` + ObjectURL("chart1") + `" style="width: 80px; height: 20px">fallback</object></p>` +
				`<p><img src="` + ObjectURL("missing") + `" alt="none"/></p>`,
		)}},
		Documents:   []string{"a.html"},
		StyleSheets: []string{`@page { size: 200px 200px; margin: 0 } body, p { margin: 0 } p { line-height: 0 }`},
		Objects:     map[string]Object{"chart1": obj},
	}
	img := renderSource(t, src)
	if n := obj.draws.Load(); n != 2 {
		t.Errorf("expected 2 draws of the object, got: %d", n)
	}
	box := colorBox(img, color.NRGBA{R: 255, A: 255})
	// The <img> is 30 by 15 points, which is 40 by 20 CSS pixels, and the
	// <object> is 80 by 20 CSS pixels below it.
	if want := image.Rect(0, 0, 80, 40); !near(box, want, 2) {
		t.Errorf("expected the red of the objects in %v, got: %v", want, box)
	}
}

func TestObjectImageNoSize(t *testing.T) {
	img := (&ObjectImage{Object: &boxObject{}}).GetIntrinsicSize(1, 16)
	if img.HasWidth || img.HasHeight || img.HasRatio {
		t.Errorf("expected no intrinsic size for an object of size 0, got: %+v", img)
	}
}

func TestMetafileImage(t *testing.T) {
	emf := fixture.NewEMF(100, 50)
	emf.Select(emf.Brush(color.NRGBA{B: 255, A: 255}, -1))
	emf.Select(fixture.Stock(8))
	emf.Rectangle(0, 0, 100, 50)
	wmf := fixture.NewWMF(100, 50, true)
	wmf.Select(wmf.Brush(color.NRGBA{G: 255, A: 255}))
	wmf.Select(wmf.NullPen())
	wmf.Rectangle(0, 0, 100, 50)
	src := Source{
		FS: fstest.MapFS{
			"a.html":    &fstest.MapFile{Data: []byte(`<p><img src="a.emf" style="width: 100px; height: 50px"/></p><p><img src="b.wmf" style="width: 100px; height: 50px"/></p>`)},
			"a.emf":     &fstest.MapFile{Data: emf.Bytes()},
			"b.wmf":     &fstest.MapFile{Data: wmf.Bytes()},
			"other.txt": &fstest.MapFile{Data: []byte("text")},
		},
		Documents:   []string{"a.html"},
		StyleSheets: []string{`@page { size: 200px 200px; margin: 0 } body, p { margin: 0 } p { line-height: 0 }`},
	}
	img := renderSource(t, src)
	if box, want := colorBox(img, color.NRGBA{B: 255, A: 255}), image.Rect(0, 0, 100, 50); !near(box, want, 2) {
		t.Errorf("expected the blue of the emf picture in %v, got: %v", want, box)
	}
	if box, want := colorBox(img, color.NRGBA{G: 255, A: 255}), image.Rect(0, 50, 100, 100); !near(box, want, 2) {
		t.Errorf("expected the green of the wmf picture in %v, got: %v", want, box)
	}
}

// near reports whether the corners of a and b are at most d pixels apart.
func near(a, b image.Rectangle, d int) bool {
	abs := func(v int) int {
		if v < 0 {
			return -v
		}
		return v
	}
	return abs(a.Min.X-b.Min.X) <= d && abs(a.Min.Y-b.Min.Y) <= d && abs(a.Max.X-b.Max.X) <= d && abs(a.Max.Y-b.Max.Y) <= d
}
