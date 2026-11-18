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
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log/slog"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fontdb"
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

	// A length that okage does not read is read as upstream reads it, and a
	// viewBox that okage does not accept gives no ratio.
	for _, c := range []struct {
		svg  string
		size IntrinsicSize
	}{
		{`<svg xmlns="http://www.w3.org/2000/svg" width="1rem" height=" 2 "/>`, IntrinsicSize{16, 2, 8, true, true, true}},
		{`<svg xmlns="http://www.w3.org/2000/svg" width="3q" height="auto"/>`, IntrinsicSize{3 * 96 / 25.4 / 4, 0, 1, true, true, true}},
		{`<svg xmlns="http://www.w3.org/2000/svg" width="20" viewBox="0 0 -10 5"/>`, IntrinsicSize{20, 0, 0, true, false, false}},
		{`<svg xmlns="http://www.w3.org/2000/svg" width="6ex" height="3pc"/>`, IntrinsicSize{48, 48, 1, true, true, true}},
	} {
		img, err := newSVGImage([]byte(c.svg), "file:///a.svg", nil, defaultLimits)
		if err != nil {
			t.Fatal(err)
		}
		if got := img.GetIntrinsicSize(1, 16); got != c.size {
			t.Errorf("%s: %+v", c.svg, got)
		}
	}
}

// TestBuildSVG checks the document of an inline svg element: the
// namespaces of the names, the text, and the depth.
func TestBuildSVG(t *testing.T) {
	h := fakeHTML(t, `<p><svg width=10 height=20 xmlns:foo="urn:foo"><rect xlink:href="#a" width=5 height=5 />tail<foreignObject><div>t</div></foreignObject></svg>after`, testBaseURL, nil)
	element := findElement(h.EtreeElement, "{"+nsSVG+"}svg")
	img := newInlineSVGImage(element, testBaseURL, nil, defaultLimits)
	if img.err != nil {
		t.Fatal(img.err)
	}
	if size := img.GetIntrinsicSize(1, 16); size.Width != 10 || size.Height != 20 {
		t.Errorf("size: %+v", size)
	}
	root := img.doc.RootElement()
	if got := root.TagName(); got != roxmltree.ExpandedNameFromTuple(nsSVG, "svg") {
		t.Errorf("root: %v", got)
	}
	if _, ok := root.Attribute(roxmltree.ExpandedNameFromStr("xmlns:foo")); ok {
		t.Error("declaration of a namespace as an attribute")
	}
	var got []string
	for node := range root.Descendants() {
		switch {
		case node.IsElement():
			got = append(got, node.TagName().String())
			if href, ok := node.Attribute(roxmltree.ExpandedNameFromTuple(nsXLink, "href")); ok {
				got = append(got, "href="+href)
			}
		case node.IsText():
			text, _ := node.Text()
			got = append(got, text)
		}
	}
	want := []string{
		"{" + nsSVG + "}svg", "{" + nsSVG + "}rect", "href=#a", "tail",
		"{" + nsSVG + "}foreignObject", "{" + nsHTML + "}div", "t",
	}
	if !slices.Equal(got, want) {
		t.Errorf("nodes:\n got %q\nwant %q", got, want)
	}

	// An element deeper than DefaultMaxTreeDepth is left out, and its tail
	// is kept.
	deep := &Element{Tag: "{" + nsSVG + "}svg"}
	for parent, i := deep, 0; i < DefaultMaxTreeDepth+5; i++ {
		child := &Element{Tag: "{" + nsSVG + "}g", Tail: "x", Parent: parent}
		parent.Children = []*Element{child}
		parent = child
	}
	img = newInlineSVGImage(deep, testBaseURL, nil, defaultLimits)
	if img.err != nil {
		t.Fatal(img.err)
	}
	depth := 0
	for node := img.doc.RootElement(); ; depth++ {
		child, ok := node.FirstElementChild()
		if !ok {
			break
		}
		node = child
	}
	if depth != DefaultMaxTreeDepth {
		t.Errorf("depth: %d", depth)
	}

	// An element that the builder refuses is an image that cannot be drawn.
	img = newInlineSVGImage(&Element{Tag: "{" + nsSVG + "}"}, testBaseURL, nil, defaultLimits)
	if img.err == nil {
		t.Fatal("no error for an element without a name")
	}
	if _, err := img.Tree(); err == nil {
		t.Error("tree of an element without a name")
	}
	if size := img.GetIntrinsicSize(1, 16); size != (IntrinsicSize{}) {
		t.Errorf("size of an element without a name: %+v", size)
	}
	if _, err := img.sizedDocument(10, 10); err == nil {
		t.Error("sized document of an element without a name")
	}
}

// TestSVGSizedDocument checks the attributes of the root element of the
// sized document, and the copy of its children.
func TestSVGSizedDocument(t *testing.T) {
	const head = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xlink:title="t" `
	for _, c := range []struct {
		svg  string
		want map[string]string
	}{
		{
			head + `width="2em" height="1in" preserveAspectRatio="xMinYMin"><rect width="1" height="1"/>text<!-- comment --></svg>`,
			map[string]string{"width": "30", "height": "40.5", "viewBox": "0 0 32 96", "preserveAspectRatio": "none"},
		},
		{
			head + `width="2em" viewBox="0 0 4 2" preserveAspectRatio="xMinYMin"><rect width="1" height="1"/>text<!-- comment --></svg>`,
			map[string]string{"width": "30", "height": "40.5", "viewBox": "0 0 4 2", "preserveAspectRatio": "xMinYMin"},
		},
		{
			head + `><rect width="1" height="1"/>text<!-- comment --></svg>`,
			map[string]string{"width": "30", "height": "40.5"},
		},
	} {
		img, err := newSVGImage([]byte(c.svg), "file:///a.svg", nil, defaultLimits)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := img.sizedDocument(30, 40.5)
		if err != nil {
			t.Fatal(err)
		}
		root := doc.RootElement()
		got := map[string]string{}
		for attr := range root.Attributes() {
			if _, ok := attr.Namespace(); !ok {
				got[attr.Name()] = attr.Value()
			}
		}
		if !maps.Equal(got, c.want) {
			t.Errorf("%s: %v", c.svg, got)
		}
		if title, _ := root.Attribute(roxmltree.ExpandedNameFromTuple(nsXLink, "title")); title != "t" {
			t.Errorf("%s: xlink:title %q", c.svg, title)
		}
		if uri, ok := root.LookupNamespaceURI("xlink"); !ok || uri != nsXLink {
			t.Errorf("%s: prefix xlink: %q", c.svg, uri)
		}
		var children []string
		for child := range root.Children() {
			if child.IsText() {
				text, _ := child.Text()
				children = append(children, text)
			} else {
				children = append(children, child.TagName().String())
			}
		}
		if want := []string{"{" + nsSVG + "}rect", "text"}; !slices.Equal(children, want) {
			t.Errorf("%s: children %q", c.svg, children)
		}
		tree, err := svg.FromXMLTree(doc)
		if err != nil {
			t.Fatal(err)
		}
		if size := tree.Size(); size.Width() != 30 || size.Height() != 40.5 {
			t.Errorf("%s: tree of %v", c.svg, size)
		}
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
	if _, ok := line.Children[0].extra().Replacement.(*SVGImage); !ok {
		t.Errorf("object: %T", line.Children[0].extra().Replacement)
	}
	inline, ok := line.Children[3].extra().Replacement.(*SVGImage)
	if !ok {
		t.Fatalf("svg: %T", line.Children[3].extra().Replacement)
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

// svgBenchFiles returns a document of n inline svg elements and n img
// elements of one SVG file, with that file.
func svgBenchFiles(n int) fstest.MapFS {
	var image strings.Builder
	image.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="60" height="30" viewBox="0 0 60 30">`)
	image.WriteString(`<defs><linearGradient id="g"><stop offset="0" stop-color="#00f"/><stop offset="1" stop-color="#f00"/></linearGradient></defs>`)
	for i := range 40 {
		fmt.Fprintf(&image, `<rect x="%d" y="%d" width="4" height="4" fill="url(#g)"/>`, i%15*4, i/15*10)
	}
	image.WriteString(`<text x="2" y="28" font-size="8">SVG</text></svg>`)
	var page strings.Builder
	page.WriteString(`<style>@page { size: 600px 400px; margin: 10px } body { margin: 0 }</style><p>`)
	for i := range n {
		fmt.Fprintf(&page, `<svg width="40" height="20" viewBox="0 0 40 20"><rect width="%d" height="10" fill="#00f"/>`, i%40)
		fmt.Fprintf(&page, `<circle cx="30" cy="10" r="8" fill="red"/><a xlink:href="#x"><text x="1" y="19" font-size="6">%d</text></a></svg> `, i)
		page.WriteString(`<img src="image.svg"> `)
	}
	return fstest.MapFS{
		"a.html":    &fstest.MapFile{Data: []byte(page.String())},
		"image.svg": &fstest.MapFile{Data: []byte(image.String())},
	}
}

// BenchmarkSVGImages lays out a document of many inline svg elements and SVG
// images, and draws its pages.
func BenchmarkSVGImages(b *testing.B) {
	ctx := context.Background()
	fsys := svgBenchFiles(200)
	fonts, _ := fontdb.Load(nil, false, ivy.Limits{}.WithDefaults(), testLogger())
	open := func() *Book {
		return NewBook(Source{FS: fsys, Documents: []string{"a.html"}}, BookOptions{Fonts: fonts})
	}
	b.Run("layout", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := open().PageCount(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("draw", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			book := open()
			n, err := book.PageCount(ctx)
			if err != nil {
				b.Fatal(err)
			}
			for i := range n {
				p, err := book.Page(ctx, i)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := ivy.RenderImage(ctx, p, 96, color.White); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}
