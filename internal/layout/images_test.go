// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file holds the tests of the port of weasyprint/images.py: the
// loading of the raster and the SVG images, their intrinsic sizes, their
// orientation, the replaced boxes of html.py, and the layout of the
// gradients. Upstream tests the images in tests/draw, with the drawing.

package layout

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log/slog"
	"math"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
)

// testImageLoader returns a loader of the files of testdata and of extra
// files, with limits.
func testImageLoader(t *testing.T, extra map[string][]byte, limits ivy.Limits) (*imageLoader, *logCapture) {
	t.Helper()
	fsys := testFS(t, nil)
	p := parseBase(t, "", testBaseURL, nil)
	for name, file := range p.html.URLFetcher.FS.(fstest.MapFS) {
		fsys[name] = file
	}
	for name, data := range extra {
		fsys[name] = &fstest.MapFile{Data: data}
	}
	log, capture := captureLogs(slog.LevelInfo)
	return newImageLoader(&URLFetcher{FS: fsys}, limits, log), capture
}

// encodePNG encodes an image of a size in PNG.
func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// encodeJPEGWithOrientation encodes an image of a size in JPEG, with an
// EXIF orientation.
func encodeJPEGWithOrientation(t *testing.T, w, h int, orientation uint16) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	// TIFF data of one directory with one entry: the orientation.
	var tiff bytes.Buffer
	tiff.WriteString("MM\x00\x2a")
	for _, v := range []any{uint32(8), uint16(1), []uint16{0x0112, 3}, uint32(1), []uint16{orientation, 0}, uint32(0)} {
		if err := binary.Write(&tiff, binary.BigEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	segment := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	app1 := make([]byte, 0, 4+len(segment))
	app1 = append(app1, 0xFF, 0xE1, byte((len(segment)+2)>>8), byte(len(segment)+2))
	app1 = append(app1, segment...)
	return append(append(append([]byte{}, data[:2]...), app1...), data[2:]...)
}

// TestRasterImage checks the intrinsic size of the raster images, with
// their orientation.
func TestRasterImage(t *testing.T) {
	loader, capture := testImageLoader(t, map[string][]byte{
		"wide.png":     encodePNG(t, 30, 10),
		"rotated.jpg":  encodeJPEGWithOrientation(t, 30, 10, 6),
		"flipped.jpg":  encodeJPEGWithOrientation(t, 30, 10, 2),
		"too-big.png":  encodePNG(t, 200, 200),
		"not-an-image": []byte("hello"),
	}, ivy.Limits{MaxPixels: 100 * 100})
	for _, c := range []struct {
		url         string
		orientation Value
		w, h        float64
		o           Orientation
	}{
		{"file:///pattern.png", String("from-image"), 4, 4, Orientation{}},
		{"file:///wide.png", String("from-image"), 30, 10, Orientation{}},
		{"file:///rotated.jpg", String("from-image"), 10, 30, Orientation{270, false}},
		{"file:///flipped.jpg", String("from-image"), 30, 10, Orientation{0, true}},
		{"file:///wide.png", Tuple{Int(90), Bool(true)}, 10, 30, Orientation{90, true}},
	} {
		loader.cache = map[string]Image{}
		img, ok := loader.get(c.url, "", c.orientation).(*RasterImage)
		if !ok {
			t.Fatalf("%s: no raster image", c.url)
		}
		size := img.GetIntrinsicSize(1, 16)
		if size.Width != c.w || size.Height != c.h || size.Ratio != c.w/c.h || img.Orientation != c.o {
			t.Errorf("%s: %+v %+v", c.url, size, img.Orientation)
		}
		if size := img.GetIntrinsicSize(2, 16); size.Width != c.w/2 {
			t.Errorf("%s: resolution 2: %+v", c.url, size)
		}
		decoded, err := img.Decode(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if b := decoded.Bounds(); float64(b.Dx()) != c.w || float64(b.Dy()) != c.h {
			t.Errorf("%s: decoded %v", c.url, b)
		}
	}
	assertNoLogs(t, capture)

	for _, url := range []string{"file:///too-big.png", "file:///not-an-image", "file:///missing.png"} {
		if img := loader.get(url, "", String("none")); img != nil {
			t.Errorf("%s: %T", url, img)
		}
	}
	logs := capture.logs()
	if len(logs) != 3 || !strings.Contains(logs[0], "ERROR: Failed to load image") {
		t.Errorf("logs: %q", logs)
	}
	if _, err := newRasterImage(encodePNG(t, 200, 200), "", nil, ivy.Limits{MaxPixels: 100}); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("limit: %v", err)
	}
}

// TestOrient checks that orient turns and flips the pixels as Pillow does.
func TestOrient(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	red := func(i image.Image, x, y int) bool {
		r, _, _, _ := i.At(x, y).RGBA()
		return r > 0
	}
	for _, c := range []struct {
		o    Orientation
		x, y int
	}{
		{Orientation{90, false}, 0, 1},  // ROTATE_90 is counterclockwise
		{Orientation{270, false}, 0, 0}, // ROTATE_270 is clockwise
		{Orientation{180, false}, 1, 0},
		{Orientation{0, true}, 1, 0},
	} {
		if out := orient(img, c.o); !red(out, c.x, c.y) {
			t.Errorf("%+v: no red at %d, %d", c.o, c.x, c.y)
		}
	}
}

// TestSVGImage checks the intrinsic size of the SVG images, from their
// attributes and their viewBox, as SVG.get_intrinsic_size of upstream
// gives it.
func TestSVGImage(t *testing.T) {
	for _, c := range []struct {
		svg  string
		size IntrinsicSize
	}{
		{`<svg xmlns="http://www.w3.org/2000/svg" width="4px" height="4px" viewBox="0 0 4 4"/>`, IntrinsicSize{4, 4, 1, true, true, true}},
		{`<svg xmlns="http://www.w3.org/2000/svg" width="2em" height="1in"/>`, IntrinsicSize{32, 96, 32.0 / 96, true, true, true}},
		{`<svg xmlns="http://www.w3.org/2000/svg" width="20" viewBox="0 0 10 5"/>`, IntrinsicSize{20, 10, 2, true, true, true}},
		{`<svg xmlns="http://www.w3.org/2000/svg" height="50%" viewBox="0,0,10,5"/>`, IntrinsicSize{0, 0, 2, false, false, true}},
		{`<svg xmlns="http://www.w3.org/2000/svg"/>`, IntrinsicSize{}},
		{`<svg xmlns="http://www.w3.org/2000/svg" width="0" height="0"/>`, IntrinsicSize{0, 0, 1, true, true, true}},
	} {
		img, err := newSVGImage([]byte(c.svg), "file:///a.svg", nil, defaultLimits)
		if err != nil {
			t.Fatal(err)
		}
		if got := img.GetIntrinsicSize(1, 16); got != c.size {
			t.Errorf("%s: %+v", c.svg, got)
		}
		// okage draws only an image with a size.
		if _, err := img.Tree(); (err != nil) != (c.size.HasWidth && c.size.Width == 0) {
			t.Errorf("%s: %v", c.svg, err)
		}
	}
	if _, err := newSVGImage([]byte("<svg"), "", nil, defaultLimits); err == nil {
		t.Error("no error for XML that is not valid")
	}
}

// TestSVGSize checks size and normalize of weasyprint/svg/utils.py.
func TestSVGSize(t *testing.T) {
	for _, c := range []struct {
		s    string
		want float64
	}{
		{"", 0}, {"12", 12}, {" 1e1 ", 10}, {"2em", 32}, {"1rem", 16}, {"4ex", 32},
		{"10%", 5}, {"1in", 96}, {"3pt", 4}, {"1q", 96 / 25.4 / 4}, {"1foo", 0},
	} {
		if got := svgSize(c.s, 16, 50); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("size(%q): %v", c.s, got)
		}
	}
	if got := svgNormalize("1E2,-3.5.5 \n4"); got != "1e2 -3.5 .5 4" {
		t.Errorf("normalize: %q", got)
	}
}

// TestReplacedBoxes checks the handlers of <img>, <object>, <embed> and
// <svg> of html.py.
func TestReplacedBoxes(t *testing.T) {
	html := parseAll(t, `
      <p><img src=pattern.png style="display: block"><object data=pattern.svg type=image/svg+xml>fallback</object><object data=missing.png>fallback</object><embed src=pattern.png><embed src=missing.png><svg width=10 height=20><rect width=5 height=5 /></svg>`)
	assertTree(t, html, `[
        ('p', 'Block', [
            ('img', 'BlockReplaced', '<replaced>'),
            ('p', 'Block', [
                ('p', 'Line', [
                    ('object', 'InlineReplaced', '<replaced>'),
                    ('object', 'Inline', [
                        ('object', 'Text', 'fallback')]),
                    ('embed', 'InlineReplaced', '<replaced>'),
                    ('{http://www.w3.org/2000/svg}svg', 'InlineReplaced', '<replaced>')])])])]`)
	line := html.Children[0].Children[0].Children[1].Children[0]
	if _, ok := line.Children[0].Replacement.(*SVGImage); !ok {
		t.Errorf("object: %T", line.Children[0].Replacement)
	}
	inline, ok := line.Children[3].Replacement.(*SVGImage)
	if !ok {
		t.Fatalf("svg: %T", line.Children[3].Replacement)
	}
	if size := inline.GetIntrinsicSize(1, 16); size.Width != 10 || size.Height != 20 {
		t.Errorf("svg: %+v", size)
	}
	tree, err := inline.Tree()
	if err != nil {
		t.Fatal(err)
	}
	if size := tree.Size(); size.Width() != 10 || size.Height() != 20 {
		t.Errorf("svg tree: %v", size)
	}
	_ = svg.FromData
}

// TestGradientLayout checks the layout of the gradients.
func TestGradientLayout(t *testing.T) {
	red, blue := mustParseColor("red"), mustParseColor("blue")
	style := newInitialStyle(&StyleContext{})

	// to bottom
	g := newLinearGradient([][2]Value{{red, nil}, {blue, nil}}, Tuple{String("angle"), Float(math.Pi)}, false, []Value{nil})
	l := g.Layout(100, 50, style)
	if l.Type != "linear" || !floatsEqual(l.Points, []float64{50, 0, 50, 50}) || !floatsEqual(l.Positions, []float64{0, 1}) {
		t.Errorf("linear: %+v", l)
	}

	// One color is a solid color.
	g = newLinearGradient([][2]Value{{red, nil}}, Tuple{String("angle"), Float(0)}, false, nil)
	if l := g.Layout(100, 50, style); l.Type != "solid" || len(l.Colors) != 1 {
		t.Errorf("solid: %+v", l)
	}

	// The same stops repeated are the average color.
	g = newLinearGradient([][2]Value{{red, Dimension{10, "px"}}, {blue, Dimension{10, "px"}}}, Tuple{String("angle"), Float(0)}, true, []Value{nil})
	if l := g.Layout(100, 50, style); l.Type != "solid" {
		t.Errorf("repeating: %+v", l)
	} else if r, _, b, a := srgba(l.Colors[0]); r != 0.5 || b != 0.5 || a != 1 {
		t.Errorf("repeating: %v %v %v", r, b, a)
	}

	// A repeating gradient of 10px repeats on 100px.
	g = newLinearGradient([][2]Value{{red, Dimension{0, "px"}}, {blue, Dimension{10, "px"}}}, Tuple{String("angle"), Float(math.Pi / 2)}, true, []Value{nil})
	if l := g.Layout(100, 50, style); l.Type != "linear" || len(l.Positions) < 10 {
		t.Errorf("repeating: %+v", l)
	}

	// A circle of the farthest corner.
	r := newRadialGradient([][2]Value{{red, nil}, {blue, nil}}, "circle", Tuple{String("keyword"), String("farthest-corner")},
		Tuple{String("left"), Dimension{50, "%"}, String("top"), Dimension{50, "%"}}, false, []Value{nil})
	l = r.Layout(60, 80, style)
	if l.Type != "radial" || !floatsEqual(l.Points, []float64{30, 40, 0, 30, 40, 50}) || l.ScaleY != 1 {
		t.Errorf("radial: %+v", l)
	}

	// A repeating ellipse of explicit size.
	r = newRadialGradient([][2]Value{{red, nil}, {blue, Dimension{10, "px"}}}, "ellipse", Tuple{String("explicit"), Tuple{Dimension{20, "px"}, Dimension{10, "px"}}},
		Tuple{String("left"), Dimension{50, "%"}, String("top"), Dimension{50, "%"}}, true, []Value{nil})
	if l := r.Layout(60, 80, style); l.Type != "radial" || l.ScaleY != 0.5 || len(l.Positions) < 4 {
		t.Errorf("repeating radial: %+v", l)
	}
}

// TestProcessColorStops checks process_color_stops.
func TestProcessColorStops(t *testing.T) {
	positions, hints := processColorStops(200,
		[]Value{nil, Dimension{25, "%"}, nil, nil, Dimension{10, "px"}},
		[]Value{nil, Dimension{100, "px"}, nil, nil}, newInitialStyle(&StyleContext{}))
	// The last stop is at least the hint before it, and the stops between
	// get the positions around them.
	if want := []float64{0, 50, 100, 100, 100}; !floatsEqual(positions, want) {
		t.Errorf("positions: %v", positions)
	}
	if len(hints) != 4 || hints[1] != 1<<16 {
		t.Errorf("hints: %v", hints)
	}
}
