package odf

import (
	"bytes"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy"
)

// TestPageFloatFiles checks the position on the page of the frames of the
// files of D60, which LibreOffice wrote from the .docx files of the tests
// of docx, with style:vertical-rel page-content and wrap (D60).
func TestPageFloatFiles(t *testing.T) {
	for _, tt := range []struct {
		name, want string
	}{
		{"f-late", "-ivy-float-position: margin 72pt"},
		{"f-bottom", "-ivy-float-position: margin bottom"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", "float-"+tt.name+".odt"))
			if err != nil {
				t.Fatal(err)
			}
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			src, err := convertText(t.Context(), d)
			if err != nil {
				t.Fatal(err)
			}
			b, err := fs.ReadFile(src.FS, src.Documents[0])
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(string(b), "-ivy-float-position"); got != 1 || !strings.Contains(string(b), tt.want) {
				t.Errorf("got %d positions, want %q", got, tt.want)
			}
		})
	}
}

// TestFrameFloatFiles checks the place of the frames of the files
// float-*.odt, which LibreOffice wrote from flat files: a frame that floats
// in the middle of the column and at its right, at its svg:x (D63), and a
// frame anchored to the page with each style:wrap (D60). The places of the
// frames come from the pages of LibreOffice 26.8 at 72 dpi. The fonts of
// the test are not those of LibreOffice, so the test checks only the sides
// of the frame that hold text, and that the text keeps the distance of 0.1
// inch from the frame. A side is 1 with text, 0 without text, and -1 when
// it is too narrow to check: the room at the right of the frame of
// float-right.odt holds a few short words in LibreOffice.
func TestFrameFloatFiles(t *testing.T) {
	for _, tt := range []struct {
		name        string
		frame       image.Rectangle
		left, right int
	}{
		{"mid", image.Rect(234, 134, 378, 206), 1, 1},
		{"right", image.Rect(360, 134, 504, 206), 1, -1},
		{"page-left", image.Rect(234, 216, 378, 324), 1, 0},
		{"page-right", image.Rect(234, 216, 378, 324), 0, 1},
		{"page-parallel", image.Rect(234, 216, 378, 324), 1, 1},
		{"page-none", image.Rect(234, 216, 378, 324), 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", "float-"+tt.name+".odt"))
			if err != nil {
				t.Fatal(err)
			}
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			p, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
			if err != nil {
				t.Fatal(err)
			}
			// The word in the frame starts at its top left corner.
			if word := odtInk(img, tt.frame); word.Empty() || word.Min.X < tt.frame.Min.X || word.Min.X > tt.frame.Min.X+3 ||
				word.Min.Y < tt.frame.Min.Y || word.Min.Y > tt.frame.Min.Y+4 || word.Max.Y > tt.frame.Min.Y+20 {
				t.Errorf("expected the frame at %v, got the ink of its word at %v", tt.frame.Min, word)
			}
			// The text beside the frame is on the rows of the frame, 7.2
			// points from it.
			beside := func(want int, r image.Rectangle, far bool) bool {
				switch {
				case want < 0:
					return true
				case want == 0 || r.Empty():
					return (want == 0) == r.Empty()
				}
				return far && r.Min.Y <= tt.frame.Min.Y+4 && r.Max.Y >= tt.frame.Max.Y-12
			}
			left := odtInk(img, image.Rect(72, tt.frame.Min.Y, tt.frame.Min.X, tt.frame.Max.Y))
			if !beside(tt.left, left, left.Max.X <= tt.frame.Min.X-6) {
				t.Errorf("expected %d for the text at the left of the frame, got %v", tt.left, left)
			}
			right := odtInk(img, image.Rect(tt.frame.Max.X, tt.frame.Min.Y, 540, tt.frame.Max.Y))
			if !beside(tt.right, right, right.Min.X >= tt.frame.Max.X+6) {
				t.Errorf("expected %d for the text at the right of the frame, got %v", tt.right, right)
			}
		})
	}
}

// TestFloatLeft checks the place of a frame that floats in its paragraph
// or on the page, for each style:horizontal-pos and style:horizontal-rel
// (D63), on a page of 612 points with margins of 72 points, in a paragraph
// with margins of 36 and 54 points. The frame is 100 points wide, with a
// margin of 10 points at its left and at its right.
func TestFloatLeft(t *testing.T) {
	c := &odtConverter{layout: &odtPageLayout{width: 612, left: 72, right: 72}, paraLeft: 36, paraRight: 54}
	for _, tt := range []struct {
		anchor, pos, rel string
		want, width      float64
	}{
		{"paragraph", "from-left", "paragraph", 64, 378},
		{"paragraph", "from-left", "paragraph-content", 100, 378},
		{"char", "from-left", "", 64, 378},
		{"paragraph", "from-left", "page", -8, 378},
		{"paragraph", "left", "page", -98, 378},
		{"paragraph", "left", "page-start-margin", -98, 378},
		{"paragraph", "right", "page-end-margin", 394, 378},
		{"paragraph", "right", "paragraph-end-margin", 322, 378},
		{"paragraph", "left", "paragraph-start-margin", -26, 378},
		{"paragraph", "center", "paragraph-content", 139, 378},
		{"paragraph", "right", "page-content", 322, 378},
		{"paragraph", "outside", "paragraph", 322, 378},
		{"page", "from-left", "", 28, 468},
		{"page", "from-left", "page-content", 100, 468},
		{"page", "center", "page", 184, 468},
	} {
		g := odtProps{"horizontal-pos": tt.pos, "horizontal-rel": tt.rel, "margin-left": "10pt", "margin-right": "10pt"}
		left, width, ok := c.floatLeft(tt.anchor, g, 100, 100)
		if !ok || left != tt.want || width != tt.width {
			t.Errorf("floatLeft(%q, %q, %q) = %g, %g, %v, want %g, %g", tt.anchor, tt.pos, tt.rel, left, width, ok, tt.want, tt.width)
		}
	}
	// In a table cell, the converter does not know the width of the
	// column.
	c.cells = 1
	if _, _, ok := c.floatLeft("paragraph", odtProps{}, 0, 100); ok {
		t.Error("expected no place in a table cell")
	}
	// There, floatEdge gives the place from the edges of the content of
	// the paragraph, without the width of the column (D63).
	for _, tt := range []struct {
		pos, rel string
		side     string
		margin   float64
		ok       bool
	}{
		{"from-left", "paragraph", "left", 64, true},
		{"from-left", "paragraph-content", "left", 100, true},
		{"from-left", "", "left", 64, true},
		{"left", "paragraph", "left", -26, true},
		{"right", "paragraph", "right", -44, true},
		{"right", "paragraph-content", "right", 10, true},
		{"center", "paragraph", "", 0, false},
		{"from-left", "page", "", 0, false},
	} {
		g := odtProps{"horizontal-pos": tt.pos, "horizontal-rel": tt.rel, "margin-left": "10pt", "margin-right": "10pt"}
		side, margin, ok := c.floatEdge("paragraph", g, 100)
		if side != tt.side || margin != tt.margin || ok != tt.ok {
			t.Errorf("floatEdge(%q, %q) = %q, %g, %v, want %q, %g, %v", tt.pos, tt.rel, side, margin, ok, tt.side, tt.margin, tt.ok)
		}
	}
	if _, _, ok := c.floatEdge("page", odtProps{}, 100); ok {
		t.Error("expected no place for a frame anchored to the page")
	}
}

// TestFramePlaceFiles checks the frames of the files float-*.odt of the
// items 31, 34, 36, 37, 38 and 39 of the backlog of 2026-11-27, which
// LibreOffice wrote from flat files. The places come from the pages of
// LibreOffice 26.8 at 72 dpi. The fonts of the test are not those of
// LibreOffice, so the test checks the places on the page that do not
// depend on the fonts. A check finds the background of a frame, of the
// color of its draw:fill, in a region of a page, and compares its left
// edge and its top edge with LibreOffice, within 2 pixels. A check with
// the left edge 0 expects no frame in the region. A check of ink finds the
// text in the region, and compares the left edge of the ink.
func TestFramePlaceFiles(t *testing.T) {
	type check struct {
		page   int
		region image.Rectangle
		ink    bool
		x, y   int
	}
	const noY = -1
	whole := image.Rect(0, 0, 612, 792)
	for _, tt := range []struct {
		name   string
		checks []check
	}{
		// The text after a frame that covers the start of the lines
		// starts at the frame, plus its distance, plus the margin of the
		// paragraph: at 216 + 7.2 + 36 points.
		{"margin", []check{
			{0, image.Rect(222, 116, 540, 170), true, 259, noY},
		}},
		// The frame anchored to the page 2 is on the page 2.
		{"page2", []check{
			{0, whole, false, 0, 0},
			{1, whole, false, 234, 216},
		}},
		// A frame at the start of a paragraph with fo:text-indent of 18
		// points is at its place, at the left of the paragraph plus its
		// distance, and its text has no indent. The paragraph of the
		// second frame has a margin of 36 points.
		{"indent", []check{
			{0, image.Rect(72, 60, 230, 150), false, 79, 72},
			{0, image.Rect(72, 72, 225, 90), true, 80, noY},
			{0, image.Rect(72, 150, 230, 300), false, 79, noY},
		}},
		// A frame in a table cell or in a section with columns is at
		// its svg:x.
		{"cell", []check{
			{0, image.Rect(72, 72, 306, 400), false, 147, noY},
			{0, image.Rect(306, 72, 540, 400), false, 460, noY},
		}},
		{"section", []check{
			{0, image.Rect(72, 72, 306, 400), false, 144, noY},
			{0, image.Rect(306, 72, 540, 400), false, 360, noY},
		}},
		// A frame with the wrap run-through anchored to the page 2 is
		// on the page 2.
		{"page2-run", []check{
			{0, whole, false, 0, 0},
			{1, whole, false, 234, 216},
		}},
		// A frame with the wrap none 0.5 inch below the top of its
		// paragraph is there, below 2 lines of the paragraph, and no text
		// is beside it.
		{"below", []check{
			{0, whole, false, 144, 150},
			{0, image.Rect(296, 152, 540, 220), true, 0, 0},
		}},
		// The line of an empty paragraph with a frame with the wrap none
		// at its top is below the frame.
		{"empty", []check{
			{0, whole, false, 144, 114},
			{0, image.Rect(72, 188, 540, 197), true, 0, 0},
		}},
		// The frames anchored to the left page 2 of a mirrored page
		// layout take their places from the margins of that page: 0.5
		// inch at the left and 2 inches at the right.
		{"mirrored", []check{
			{0, image.Rect(0, 200, 612, 300), false, 216, 216},
			{0, image.Rect(0, 350, 612, 440), false, 151, 360},
			{0, image.Rect(0, 530, 612, 620), false, 216, 540},
			{1, image.Rect(0, 200, 612, 300), false, 108, 216},
			{1, image.Rect(0, 350, 612, 440), false, 43, 360},
			{1, image.Rect(0, 530, 612, 620), false, 216, 540},
		}},
		// A frame at the center of its paragraph is there in a header,
		// in a table cell and in a section with columns.
		{"center", []check{
			{0, image.Rect(0, 60, 612, 115), false, 270, 72},
			{0, image.Rect(72, 115, 306, 200), false, 153, noY},
			{0, image.Rect(306, 115, 540, 200), false, 387, noY},
			{0, image.Rect(72, 200, 306, 300), false, 144, noY},
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", "float-"+tt.name+".odt"))
			if err != nil {
				t.Fatal(err)
			}
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			fill := color.NRGBA{R: 0xc0, G: 0xe0, B: 0xff, A: 255}
			for _, c := range tt.checks {
				p, err := d.Page(t.Context(), c.page)
				if err != nil {
					t.Fatal(err)
				}
				img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
				if err != nil {
					t.Fatal(err)
				}
				var box image.Rectangle
				if c.ink {
					box = odtInk(img, c.region)
				} else {
					box, _ = odtColorBox(odtRegion{img, c.region}, fill)
				}
				near := func(got, want int) bool { return want == noY || got >= want-2 && got <= want+2 }
				switch {
				case c.x == 0:
					if !box.Empty() {
						t.Errorf("page %d: expected no frame in %v, got %v", c.page+1, c.region, box)
					}
				case box.Empty() || !near(box.Min.X, c.x) || !near(box.Min.Y, c.y):
					t.Errorf("page %d: expected the box in %v at (%d, %d), got %v", c.page+1, c.region, c.x, c.y, box)
				}
			}
		})
	}
}

// odtRegion is the part of an image in a rectangle.
type odtRegion struct {
	image.Image
	r image.Rectangle
}

// Bounds returns the rectangle of the region in the image.
func (o odtRegion) Bounds() image.Rectangle {
	return o.r.Intersect(o.Image.Bounds())
}

// TestTopFrameFile checks that the frame with the wrap none after the text
// of its paragraph in float-top.odt goes at the top of the paragraph,
// before its text, as LibreOffice 26.8 puts it.
func TestTopFrameFile(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "float-top.odt"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	src, err := convertText(t.Context(), d)
	if err != nil {
		t.Fatal(err)
	}
	b, err := fs.ReadFile(src.FS, src.Documents[0])
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)
	frame, text := strings.Index(html, ">FRAME<"), strings.Index(html, ">P02 ")
	if frame < 0 || text < 0 || frame > text || strings.Count(html, ">FRAME<") != 1 {
		t.Errorf("expected the frame once, before the text of its paragraph, got:\n%s", html)
	}
}

// TestFramePlaceHTML checks the HTML of the frames that float: a frame
// without style:wrap, which has the wrap parallel as in LibreOffice 26.8,
// a frame with the wrap run-through anchored to a later page, a frame with
// the wrap none below the top of its paragraph, frames in a table cell,
// where the converter does not know the width of the column, frames in
// paragraphs without text, and a frame in a paragraph with an indent.
func TestFramePlaceHTML(t *testing.T) {
	const styles = `<style:style style:name="plain" style:family="graphic"><style:graphic-properties style:vertical-pos="from-top" style:vertical-rel="paragraph"/></style:style>` +
		`<style:style style:name="run" style:family="graphic"><style:graphic-properties style:wrap="run-through"/></style:style>` +
		`<style:style style:name="none" style:family="graphic"><style:graphic-properties style:wrap="none" style:vertical-pos="from-top" style:vertical-rel="paragraph"/></style:style>` +
		`<style:style style:name="center" style:family="graphic"><style:graphic-properties style:wrap="parallel" style:horizontal-pos="center" style:horizontal-rel="paragraph"/></style:style>` +
		`<style:style style:name="indent" style:family="paragraph"><style:paragraph-properties fo:text-indent="18pt"/></style:style>`
	frame := func(style, anchor, x, y string) string {
		return `<draw:frame draw:style-name="` + style + `" text:anchor-type="` + anchor + `" svg:x="` + x + `" svg:y="` + y +
			`" svg:width="72pt" svg:height="36pt"><draw:text-box><text:p>F</text:p></draw:text-box></draw:frame>`
	}
	for _, tt := range []struct {
		name, body, want string
	}{
		{"no wrap", `<text:p>A` + frame("plain", "paragraph", "0pt", "0pt") + `</text:p>`, `<span style="float: left; -ivy-wrap-flow: both 0pt margin; text-indent: 0; width: 72pt">`},
		{"run-through", strings.Replace(frame("run", "page", "100pt", "200pt"), "<draw:frame ", `<draw:frame text:anchor-page-number="3" `, 1) + `<text:p>A</text:p>`,
			`<span style="float: left; width: 0; height: 0; -ivy-float-position: margin 0pt; -ivy-float-page: 3"><span style="position: absolute; top: `},
		{"below", `<text:p>A` + frame("none", "paragraph", "10pt", "24pt") + `</text:p>`, `-ivy-float-position: block 24pt`},
		{"below in a cell", `<table:table><table:table-column/><table:table-row><table:table-cell><text:p>A` + frame("none", "paragraph", "10pt", "24pt") +
			`</text:p></table:table-cell></table:table-row></table:table>`, `<span style="float: left; margin-left: 10pt; margin-right: calc(100% - 82pt); -ivy-float-position: block 24pt; text-indent: 0; width: 72pt">`},
		{"center in a cell", `<table:table><table:table-column/><table:table-row><table:table-cell><text:p>A` + frame("center", "paragraph", "0pt", "0pt") +
			`</text:p></table:table-cell></table:table-row></table:table>`, `<span style="float: left; margin-left: calc(50% - 36pt); -ivy-wrap-flow: both 0pt margin; text-indent: 0; width: 72pt">`},
		// A paragraph without text has the line of its paragraph mark.
		{"empty", `<text:p>` + frame("plain", "paragraph", "0pt", "0pt") + `</text:p>`, `</span><br/></p>`},
		{"empty run-through", `<text:p>` + frame("run", "paragraph", "0pt", "0pt") + `</text:p>`, `</span><br/></p>`},
		// The box of a frame resets the indent of the first line, which
		// it inherits from its paragraph.
		{"indent", `<text:p text:style-name="indent">` + frame("plain", "paragraph", "0pt", "0pt") + `A</text:p>`, `-ivy-wrap-flow: both 0pt margin; text-indent: 0; width: 72pt`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html, _ := odtHTML(t, odtStyledFile(styles, tt.body))
			if !strings.Contains(html, tt.want) {
				t.Errorf("expected the xhtml to hold %q, got:\n%s", tt.want, html)
			}
		})
	}
}

// TestFloatCenter checks the margin of a frame at the center of its
// paragraph without the width of the column, in a paragraph with margins
// of 36 and 54 points. The frame is 100 points wide, with margins of 10
// and 20 points.
func TestFloatCenter(t *testing.T) {
	c := &odtConverter{paraLeft: 36, paraRight: 54}
	for _, tt := range []struct {
		pos, rel string
		want     float64
		ok       bool
	}{
		{"center", "paragraph", -46, true},
		{"center", "", -46, true},
		{"center", "paragraph-content", -55, true},
		{"center", "page", 0, false},
		{"left", "paragraph", 0, false},
	} {
		g := odtProps{"horizontal-pos": tt.pos, "horizontal-rel": tt.rel, "margin-left": "10pt", "margin-right": "20pt"}
		got, ok := c.floatCenter("paragraph", g, 100)
		if got != tt.want || ok != tt.ok {
			t.Errorf("floatCenter(%q, %q) = %g, %v, want %g, %v", tt.pos, tt.rel, got, ok, tt.want, tt.ok)
		}
	}
	if _, ok := c.floatCenter("page", odtProps{"horizontal-pos": "center"}, 100); ok {
		t.Error("expected no margin for a frame anchored to the page")
	}
	for _, tt := range []struct {
		pct, pt float64
		want    string
	}{
		{0, 12, "12pt"},
		{50, 12, "calc(50% + 12pt)"},
		{50, -12, "calc(50% - 12pt)"},
		{100, -82, "calc(100% - 82pt)"},
		{100, 0, "100%"},
	} {
		if got := odtCalc(tt.pct, tt.pt); got != tt.want {
			t.Errorf("odtCalc(%g, %g) = %q, want %q", tt.pct, tt.pt, got, tt.want)
		}
	}
}
