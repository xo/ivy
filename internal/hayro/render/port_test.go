// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the tests of the port that upstream does not have. Upstream
// compares its renders with reference images, which the port does not
// copy.

package render

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// counter is a display device that counts the commands, and checks that the
// saves and the groups balance.
type counter struct {
	display.List
	depth, groups, maxDepth int
	fills, strokes, images  int
	masks, softMasks        int
}

func (c *counter) Save() {
	c.depth++
	c.maxDepth = max(c.maxDepth, c.depth)
}

func (c *counter) Restore() { c.depth-- }

func (c *counter) FillPath(*okage.Path, okage.FillRule, okage.Shader) { c.fills++ }

func (c *counter) StrokePath(*okage.Path, okage.Stroke, okage.Shader) { c.strokes++ }

func (c *counter) DrawImage(image.Image, okage.Transform, display.ImageOptions) { c.images++ }

func (c *counter) FillMask(*image.Alpha, okage.Transform, display.ImageOptions, okage.Shader) {
	c.masks++
}

func (c *counter) BeginGroup(g display.Group) {
	c.groups++
	if g.Mask != nil {
		c.softMasks++
		var inner counter
		g.Mask.Content(&inner)
		if inner.depth != 0 || inner.groups != 0 {
			panic("the content of a soft mask does not balance")
		}
	}
}

func (c *counter) EndGroup() { c.groups-- }

// testFiles returns the test files of interpret and syntax.
func testFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"../interpret/testdata", "../syntax/testdata"} {
		names, err := filepath.Glob(filepath.Join(dir, "*.pdf"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, names...)
	}
	return files
}

func TestRenderIntoBalances(t *testing.T) {
	var total counter
	for _, name := range testFiles(t) {
		buf, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		pdf, err := syntax.NewPdf(buf)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		cache := NewRenderCache()
		for i, page := range pdf.Pages().Slice() {
			var c counter
			w, h := page.RenderDimensions()
			transform := interpret.TransformToKurbo(page.InitialTransform(true))
			err := RenderInto(page, cache, interpret.DefaultInterpreterSettings(), &RenderSettings{}, &c, int(w), int(h), transform)
			if err != nil {
				t.Errorf("%s page %d: %v", name, i+1, err)
			}
			if c.depth != 0 || c.groups != 0 {
				t.Errorf("%s page %d: expected the saves and the groups to balance, got %d and %d", name, i+1, c.depth, c.groups)
			}
			total.fills += c.fills
			total.strokes += c.strokes
			total.images += c.images
			total.masks += c.masks
			total.softMasks += c.softMasks
		}
	}
	if total.fills == 0 || total.strokes == 0 || total.images == 0 || total.masks == 0 || total.softMasks == 0 {
		t.Errorf("expected each kind of command, got %+v", total)
	}
}

func TestRender(t *testing.T) {
	buf, err := os.ReadFile("../interpret/testdata/pattern_tiling_simple.pdf")
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := syntax.NewPdf(buf)
	if err != nil {
		t.Fatal(err)
	}
	settings := PixmapSettings{XScale: 0.5, YScale: 0.5, BgColor: okage.ColorWhite}
	page := pdf.Pages().Slice()[0]
	pix, err := Render(page, NewRenderCache(), interpret.DefaultInterpreterSettings(), &RenderSettings{}, &settings)
	if err != nil || pix == nil {
		t.Fatalf("expected a pixmap, got %v", err)
	}
	w, h := page.RenderDimensions()
	if pix.Width() != uint32(w/2) || pix.Height() != uint32(h/2) {
		t.Errorf("expected a pixmap of %g by %g, got %d by %d", w/2, h/2, pix.Width(), pix.Height())
	}
	// the margin is the white of the background, and the middle holds the
	// green of the pattern
	if c, _ := pix.Pixel(5, 5); c.Red() != 255 || c.Green() != 255 || c.Blue() != 255 {
		t.Errorf("expected white near the corner, got: %v", c)
	}
	var green int
	for y := range pix.Height() {
		for x := range pix.Width() {
			if c, _ := pix.Pixel(x, y); c.Green() > 100 && c.Red() < 50 && c.Blue() < 50 {
				green++
			}
		}
	}
	if green < 100 {
		t.Errorf("expected the green cells of the pattern, got %d pixels", green)
	}
}

func TestTileAxis(t *testing.T) {
	// the area spans more than a step, so the cell repeats
	x := newTileAxis(2, 0, 10, 10, 0, 100)
	if !x.repeat || x.size != 20 || x.first != 0 || x.last != 0 {
		t.Errorf("expected a repeated cell of 20 pixels, got %+v", x)
	}
	// a cell wider than its step needs copies to the left
	x = newTileAxis(1, 0, 25, 10, 0, 100)
	if x.first != -2 || x.last != 0 {
		t.Errorf("expected the copies -2 to 0, got %+v", x)
	}
	// the area spans less than a step, so the cell covers the area
	x = newTileAxis(1, 0, 10, 100, 5, 50)
	if x.repeat || x.start != 5 || x.size != 45 {
		t.Errorf("expected a padded cell of 45 pixels from 5, got %+v", x)
	}
}

func TestSingleSegmentOutline(t *testing.T) {
	path := kurboext.BezPathFromElements([]kurbo.PathEl{kurbo.MoveTo(kurbo.NewPoint(0, 0)), kurbo.LineTo(kurbo.NewPoint(10, 0))})
	stroke := okage.Stroke{Width: 2, LineCap: okage.LineCapSquare}
	p := singleSegmentOutline(&path, &stroke)
	if p == nil {
		t.Fatal("expected an outline")
	}
	if b := p.Bounds(); b.Left() != -1 || b.Right() != 11 || b.Top() != -1 || b.Bottom() != 1 {
		t.Errorf("expected the bounds [-1 -1 11 1], got: %v", b)
	}
	stroke.LineCap = okage.LineCapRound
	if singleSegmentOutline(&path, &stroke) != nil {
		t.Error("expected no outline for round caps")
	}
}

func TestStrokeProperties(t *testing.T) {
	r := &renderer{}
	props := interpret.DefaultStrokeProps()
	props.LineCap = kurboext.CapRound
	props.DashArray = []float32{3}
	stroke := r.strokeProperties(&props, kurbo.AffineScale(4), false)
	if stroke.Dash == nil {
		t.Error("expected a dash array of odd length to repeat")
	}
	if stroke.LineCap != okage.LineCapRound || stroke.Width != 1 {
		t.Errorf("expected a round cap and a width of 1, got %+v", stroke)
	}
	// a hairline grows to one pixel, and its round cap becomes square
	props.LineWidth = 0
	stroke = r.strokeProperties(&props, kurbo.AffineScale(4), false)
	if stroke.LineCap != okage.LineCapSquare || stroke.Width < 0.24 || stroke.Width > 0.26 {
		t.Errorf("expected a square cap and a width of 0.25, got %+v", stroke)
	}
}

// strokePDF returns a PDF file of one page that strokes a line, after the
// graphics state gs with the entries of extGState.
func strokePDF(extGState string) []byte {
	content := "/G0 gs 10 10.5 m 90 10.5 l S"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 20] /Contents 4 0 R " +
			"/Resources << /ExtGState << /G0 << " + extGState + " >> >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

func TestStrokeAdjust(t *testing.T) {
	for _, tt := range []struct {
		extGState string
		want      string
	}{
		// a stroke of one line fills its outline without the stroke
		// adjustment
		{"/SA true", "stroke adjusted [10 10 90 10] 1"},
		{"/SA false", "fill [10 10 90 11] Winding"},
		{"/LW 2", "fill [10 10 90 12] Winding"},
	} {
		pdf, err := syntax.NewPdf(strokePDF(tt.extGState))
		if err != nil {
			t.Fatal(err)
		}
		page := pdf.Pages().Slice()[0]
		var l display.List
		w, h := page.RenderDimensions()
		transform := interpret.TransformToKurbo(page.InitialTransform(true))
		if err := RenderInto(page, NewRenderCache(), interpret.DefaultInterpreterSettings(), &RenderSettings{}, &l, int(w), int(h), transform); err != nil {
			t.Fatal(err)
		}
		if got := l.String(); !strings.Contains(got, "\n"+tt.want) {
			t.Errorf("%s: expected %q in the commands, got:\n%s", tt.extGState, tt.want, got)
		}
	}
}
