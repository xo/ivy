package markdown

import (
	"bytes"
	"image"
	"image/color"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/ebook/ebooktest"
	"github.com/xo/ivy/internal/fixture"
)

// The tests of this file draw the HTML that GitHub allows in a README. The
// pages are US Letter at 96 dots for each inch, so the text is between
// x = 72 and x = 744, with its center at x = 408.

// The colors of the images: a test finds a blue image, and must not see a
// red one.
var (
	blue = color.NRGBA{B: 0xff, A: 0xff}
	red  = color.NRGBA{R: 0xff, A: 0xff}
)

// readme returns the folder of a README: blue and red images.
func readme() fstest.MapFS {
	return fstest.MapFS{
		"b.png":              {Data: fixture.PNG(10, 10, blue)},
		"wide.png":           {Data: fixture.PNG(20, 10, blue)},
		"docs/gallery/x.png": {Data: fixture.PNG(20, 10, blue)},
		"dark.png":           {Data: fixture.PNG(10, 10, red)},
		"images/0.png":       {Data: fixture.PNG(10, 10, red)},
	}
}

// renderReadme renders the first page of the Markdown text with the folder
// of readme.
func renderReadme(t *testing.T, text string) image.Image {
	t.Helper()
	d, err := Open(t.Context(), strings.NewReader(text), WithDir(readme()))
	if err != nil {
		t.Fatal(err)
	}
	return ebooktest.RenderPage(t, d, 0)
}

// inkBounds returns the smallest rectangle that holds the dark pixels of img
// in r.
func inkBounds(img image.Image, r image.Rectangle) image.Rectangle {
	var b image.Rectangle
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if c := color.GrayModel.Convert(img.At(x, y)).(color.Gray); c.Y < 128 {
				b = b.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return b
}

// count returns the number of pixels of img of the color c, and the
// smallest rectangle that holds them.
func count(img image.Image, c color.NRGBA) (int, image.Rectangle) {
	return countIn(img, c, img.Bounds())
}

// countIn is count inside the rectangle b.
func countIn(img image.Image, c color.NRGBA, b image.Rectangle) (int, image.Rectangle) {
	n := 0
	var r image.Rectangle
	b = b.Intersect(img.Bounds())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if color.NRGBAModel.Convert(img.At(x, y)) == c {
				n++
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return n, r
}

// near reports whether a and b differ by at most 2 pixels.
func near(a, b int) bool {
	return a-b <= 2 && b-a <= 2
}

func TestHTMLAlign(t *testing.T) {
	const img = `<img src="b.png" width="100" height="20" alt="b">`
	for name, tc := range map[string]struct {
		text string
		// left is the left edge of the image, and right its right edge.
		left, right int
	}{
		"div center": {text: `<div align="center">` + img + `</div>`, left: 358, right: 458},
		"p center":   {text: `<p align="center">` + img + `</p>`, left: 358, right: 458},
		"h1 center":  {text: `<h1 align="center">` + img + `</h1>`, left: 358, right: 458},
		"div right":  {text: `<div align="right">` + img + `</div>`, left: 644, right: 744},
		"img right":  {text: `<img src="b.png" width="100" height="20" align="right" alt="b">` + "\n\nText.", left: 644, right: 744},
		"img left":   {text: `<img src="b.png" width="100" height="20" align="left" alt="b">` + "\n\nText.", left: 72, right: 172},
		// the text of the cell starts after half of the border of 1 pixel,
		// as the borders collapse, and the padding of 13 pixels of the cell
		"td center": {text: `<table><tr><td align="center" width="400">` + img + `</td></tr></table>`, left: 86 + 150, right: 86 + 250},
	} {
		t.Run(name, func(t *testing.T) {
			_, b := count(renderReadme(t, tc.text), blue)
			if !near(b.Min.X, tc.left) || !near(b.Max.X, tc.right) || !near(b.Dy(), 20) {
				t.Errorf("expected the image from x = %d to %d, 20 pixels high, got: %v", tc.left, tc.right, b)
			}
		})
	}
}

func TestHTMLImage(t *testing.T) {
	text := `<img src="docs/gallery/x.png" width="260" alt="gallery">` + "\n\n" +
		`<img src="https://example.com/badge.svg" alt="badge">` + "\n\n" +
		`<img src="docs/gallery/none.png" alt="none">` + "\n\n" +
		`<img src="../outside.png" alt="outside">` + "\n"
	d, err := Open(t.Context(), strings.NewReader(text), WithDir(readme()))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if buf, err := fs.ReadFile(src.FS, "docs/gallery/x.png"); err != nil || !bytes.Equal(buf, readme()["docs/gallery/x.png"].Data) {
		t.Errorf("expected the image of the folder at its path, got: %v", err)
	}
	page := ebooktest.RenderPage(t, d, 0)
	// the image keeps the ratio of its file: 260 by 130 pixels
	_, b := count(page, blue)
	if !near(b.Min.X, 72) || !near(b.Dx(), 260) || !near(b.Dy(), 130) {
		t.Errorf("expected an image of 260 by 130 pixels, got: %v", b)
	}
	// the images that ivy does not read show their alternate text on the
	// line of the image
	if ink := ebooktest.Ink(page, image.Rect(b.Max.X, 72, 744, b.Max.Y)); ink < 50 {
		t.Errorf("expected the alternate texts, got %d pixels of ink", ink)
	}
	// without a folder, the image shows its alternate text
	d, err = Open(t.Context(), strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	page = ebooktest.RenderPage(t, d, 0)
	if n, _ := count(page, blue); n != 0 {
		t.Errorf("expected no image without a folder, got %d pixels", n)
	}
	if ink := ebooktest.Ink(page, page.Bounds()); ink < 50 {
		t.Errorf("expected the alternate texts, got %d pixels of ink", ink)
	}
}

// TestHTMLImageMaxWidth draws an image that is wider than the page. It is
// as wide as the text.
func TestHTMLImageMaxWidth(t *testing.T) {
	_, b := count(renderReadme(t, `<img src="wide.png" width="1000" alt="wide">`), blue)
	if !near(b.Min.X, 72) || !near(b.Max.X, 744) {
		t.Errorf("expected the image from x = 72 to 744, got: %v", b)
	}
}

// TestHTMLGrid draws a grid of images in a table, as the gallery of the
// README of ivy. Three images of 260 pixels are wider than the text, and
// the images shrink to fit it.
func TestHTMLGrid(t *testing.T) {
	cell := `<td align="center"><a href="wide.png"><img src="wide.png" width="260" alt="wide"></a><br>Caption</td>`
	row := "<tr>\n" + strings.Repeat(cell+"\n", 3) + "</tr>\n"
	page := renderReadme(t, "<table>\n"+row+row+"</table>\n")
	if b := inkBounds(page, page.Bounds()); b.Min.X < 72 || b.Max.X > 744 {
		t.Errorf("expected the table inside the text, from x = 72 to 744, got: %v", b)
	}
	// each of the three columns holds an image of about 200 by 100 pixels
	for i := range 3 {
		x := 72 + 224*i
		if n, _ := countIn(page, blue, image.Rect(x, 72, x+224, 1000)); n < 2*190*95 {
			t.Errorf("expected two images in the column %d, got %d pixels", i, n)
		}
	}
}

func TestHTMLDetails(t *testing.T) {
	const img = `<img src="b.png" width="100" height="100" alt="b">`
	summary := renderReadme(t, "<details><summary>Summary</summary></details>\n")
	for name, tc := range map[string]struct {
		text string
		open bool
	}{
		"closed":           {text: "<details>\n<summary>Summary</summary>\n\n" + img + "\n\nHidden text.\n</details>\n"},
		"closed bare text": {text: "<details><summary>Summary</summary>Hidden text. " + img + "</details>\n"},
		"open":             {text: "<details open>\n<summary>Summary</summary>\n\n" + img + "\n</details>\n", open: true},
	} {
		t.Run(name, func(t *testing.T) {
			page := renderReadme(t, tc.text)
			b := inkBounds(page, page.Bounds())
			switch {
			case tc.open && b.Dy() < 100:
				t.Errorf("expected the content of an open details element, got ink in %v", b)
			case !tc.open && b != inkBounds(summary, summary.Bounds()):
				t.Errorf("expected only the summary, in %v, got ink in %v", inkBounds(summary, summary.Bounds()), b)
			}
			// the summary has a triangle before it
			if ink := ebooktest.Ink(page, image.Rect(72, 72, 86, 110)); ink == 0 {
				t.Error("expected a triangle before the summary")
			}
		})
	}
}

func TestHTMLInline(t *testing.T) {
	// the first H is in [73, 85), and the second one starts after it
	plain := inkBounds(renderReadme(t, "HH"), image.Rect(0, 0, 816, 400))
	mid := plain.Min.X + plain.Dx()/2
	for name, tc := range map[string]struct {
		text string
		// check reports whether the second H, in second, is in its place,
		// with the first H in first
		check func(first, second image.Rectangle) bool
	}{
		"sup": {text: "H<sup>H</sup>", check: func(first, second image.Rectangle) bool { return second.Max.Y < first.Max.Y-2 }},
		"sub": {text: "H<sub>H</sub>", check: func(first, second image.Rectangle) bool { return second.Max.Y > first.Max.Y+2 }},
		"br":  {text: "H<br>H", check: func(first, second image.Rectangle) bool { return second.Min.Y > first.Max.Y }},
	} {
		t.Run(name, func(t *testing.T) {
			page := renderReadme(t, tc.text)
			first := inkBounds(page, image.Rect(0, 0, mid, 400))
			second := inkBounds(page, image.Rect(mid, 0, 816, 400))
			if tc.text == "H<br>H" {
				// the second H is on the next line
				first = inkBounds(page, image.Rect(0, 0, 816, plain.Max.Y))
				second = inkBounds(page, image.Rect(0, plain.Max.Y, 816, 400))
			}
			if first.Empty() || second.Empty() || !tc.check(first, second) {
				t.Errorf("unexpected ink: the first H in %v, the second one in %v", first, second)
			}
		})
	}
	// a key is in a gray box
	page := renderReadme(t, "Press <kbd>Ctrl</kbd>.")
	if n, _ := count(page, color.NRGBA{R: 0xf6, G: 0xf8, B: 0xfa, A: 0xff}); n < 100 {
		t.Errorf("expected the background of the key, got %d pixels", n)
	}
}

// TestHTMLPicture draws the img element of a picture element, and not its
// source for a dark page.
func TestHTMLPicture(t *testing.T) {
	text := "<picture>\n" +
		`<source media="(prefers-color-scheme: dark)" srcset="dark.png">` + "\n" +
		`<img src="b.png" width="100" height="100" alt="light">` + "\n" +
		"</picture>\n"
	page := renderReadme(t, text)
	if n, _ := count(page, blue); n < 90*90 {
		t.Errorf("expected the blue image, got %d pixels", n)
	}
	if n, _ := count(page, red); n != 0 {
		t.Errorf("expected no red image, got %d pixels", n)
	}
}

// TestHTMLImagePath gives an img element the path images/0.png, which the
// images of the Markdown text take. The image of the Markdown text takes
// another path.
func TestHTMLImagePath(t *testing.T) {
	text := `<img src="images/0.png" width="100" height="100" alt="red">` + "\n\n![black](b.png)\n"
	d, err := Open(t.Context(), strings.NewReader(text), WithDir(readme()))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := fs.ReadFile(src.FS, src.Documents[0])
	if !strings.Contains(string(doc), `<img src="images/1.png" alt="black">`) {
		t.Errorf("expected the image of the text at images/1.png, got:\n%s", doc)
	}
	for name, want := range map[string]string{"images/0.png": "images/0.png", "images/1.png": "b.png"} {
		buf, err := fs.ReadFile(src.FS, name)
		if err != nil || !bytes.Equal(buf, readme()[want].Data) {
			t.Errorf("expected %s at %s, got: %v", want, name, err)
		}
	}
}

// TestHTMLAnchor links to an a element with the attribute name.
func TestHTMLAnchor(t *testing.T) {
	d, err := Open(t.Context(), strings.NewReader("[Go](#here)\n\n### Heading <a name=\"here\"></a>\n"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	var l display.List
	if err := page.(display.Drawer).Draw(t.Context(), &l, 1); err != nil {
		t.Fatal(err)
	}
	list := l.String()
	if !strings.Contains(list, `anchor "index.html#here"`) || !strings.Contains(list, `#"index.html#here"`) {
		t.Errorf("expected a link to the anchor here, got:\n%s", list)
	}
}
