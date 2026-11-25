package docx

import (
	"bytes"
	"image"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// mathNS declares the prefix of the namespace of Office Math.
const mathNS = `xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"`

// red is the properties of a red run of 12 points.
const red = `<w:rPr><w:color w:val="FF0000"/><w:sz w:val="24"/></w:rPr>`

func TestEquations(t *testing.T) {
	t.Parallel()
	// An inline x squared after the word "Let", and a display fraction of
	// a over b, both red.
	body := `<w:p><w:r><w:rPr><w:sz w:val="24"/></w:rPr><w:t xml:space="preserve">Let </w:t></w:r>` +
		`<m:oMath ` + mathNS + `><m:sSup><m:e><m:r>` + red + `<m:t>x</m:t></m:r></m:e><m:sup><m:r>` + red +
		`<m:t>2</m:t></m:r></m:sup></m:sSup></m:oMath></w:p>` +
		`<w:p><m:oMathPara ` + mathNS + `><m:oMath><m:f><m:num><m:r>` + red + `<m:t>a</m:t></m:r></m:num>` +
		`<m:den><m:r>` + red + `<m:t>b</m:t></m:r></m:den></m:f></m:oMath></m:oMathPara></w:p>`
	buf := fixture.DOCX(body)
	html, _ := convertHTML(t, buf)
	for _, want := range []*regexp.Regexp{
		// The inline equation sits on the baseline by its depth.
		regexp.MustCompile(`<img src="ivy-object:math1" alt="" style="width: [0-9.]+pt; height: [0-9.]+pt; vertical-align: -?[0-9.]+pt"/>`),
		// The display equation is a line of its own, centered.
		regexp.MustCompile(`<span style="display: block; text-align: center; text-indent: 0"><img src="ivy-object:math2" alt="" style="width: [0-9.]+pt; height: [0-9.]+pt; vertical-align: -[0-9.]+pt"/></span>`),
	} {
		if !want.MatchString(html) {
			t.Errorf("expected the xhtml to match %s, got:\n%s", want, html)
		}
	}
	if strings.Contains(html, "font-style: italic") {
		t.Errorf("expected no equation as italic text, got:\n%s", html)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	img := render(t, d, 0)
	b := img.Bounds()
	// "Let" is the only ink at the left of the page, from the margin of 72
	// points, and the inline equation follows it on its line.
	let := inkBox(img, image.Rect(b.Min.X, b.Min.Y, 92, b.Max.Y))
	if let.Empty() {
		t.Fatalf("expected the ink of Let")
	}
	inline := redIn(img, image.Rect(let.Max.X, let.Min.Y-8, b.Max.X, let.Max.Y+3))
	// The x sits on the baseline of "Let": the bottoms of their ink match.
	if inline.Empty() || abs(let.Max.Y-inline.Max.Y) > 1 || inline.Min.Y >= let.Min.Y {
		t.Errorf("expected x squared on the baseline after Let at %v, got %v", let, inline)
	}
	// The display fraction is on the next line, in the middle of the text
	// area of 72 to 540 points.
	display := redIn(img, image.Rect(b.Min.X, let.Max.Y+4, b.Max.X, b.Max.Y))
	if mid := (display.Min.X + display.Max.X) / 2; display.Empty() || mid < 304 || mid > 308 {
		t.Errorf("expected the fraction centered at 306 points, got %v", display)
	}
}

// redIn returns the box of the red pixels of img in the rectangle r.
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

// abs returns the absolute value of v.
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
