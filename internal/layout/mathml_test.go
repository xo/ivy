package layout

import (
	"image"
	"testing"
	"testing/fstest"
)

// inkIn returns the box of the pixels of img in r that are darker than
// mid grey.
func inkIn(img image.Image, r image.Rectangle) image.Rectangle {
	var box image.Rectangle
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if (cr+cg+cb)/3 < 0x8000 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// redIn returns the box of the red pixels of img in r.
func redIn(img image.Image, r image.Rectangle) image.Rectangle {
	var box image.Rectangle
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if cr > 0xe000 && cg < 0x6000 && cb < 0x6000 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// TestMathML checks that an inline formula of MathML sits on the baseline
// of its line, and that a formula of display="block" is centered on a line
// of its own, in XHTML and in HTML.
func TestMathML(t *testing.T) {
	const body = `<p>Let <math xmlns="http://www.w3.org/1998/Math/MathML"><msup><mi>x</mi><mn>2</mn></msup></math></p>` +
		`<math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><mfrac><mi>a</mi><mi>b</mi></mfrac></math>`
	for _, name := range []string{"a.xhtml", "a.html"} {
		doc := `<html xmlns="http://www.w3.org/1999/xhtml"><body>` + body + `</body></html>`
		src := Source{
			FS:          fstest.MapFS{name: &fstest.MapFile{Data: []byte(doc)}},
			Documents:   []string{name},
			StyleSheets: []string{`@page { size: 300px 200px; margin: 0 } body, p { margin: 0 } body { font-size: 20px } math { color: red }`},
		}
		img := renderSource(t, src)
		// "Let" is the only ink at the left of the page.
		let := inkIn(img, image.Rect(0, 0, 30, 200))
		if let.Empty() {
			t.Fatalf("%s: expected the ink of Let", name)
		}
		inline := redIn(img, image.Rect(let.Max.X, 0, 300, let.Max.Y+3))
		if inline.Empty() || abs(inline.Max.Y-let.Max.Y) > 1 || inline.Min.Y >= let.Min.Y {
			t.Errorf("%s: expected x squared on the baseline of Let at %v, got %v", name, let, inline)
		}
		block := redIn(img, image.Rect(0, let.Max.Y+4, 300, 200))
		if mid := (block.Min.X + block.Max.X) / 2; block.Empty() || mid < 147 || mid > 153 || block.Dy() < 30 {
			t.Errorf("%s: expected a fraction centered at 150 px, got %v", name, block)
		}
	}
}
