// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/__init__.py: the helpers that render a document
// and compare its pixels with a map of characters, with document_write_png of
// tests/conftest.py.

package layout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
)

// rgb is a pixel of the tests: red, green and blue, from 0 to 255.
type rgb [3]int

// pixelsByChar is PIXELS_BY_CHAR in tests/draw/__init__.py. A nil pixel,
// "z", is not compared.
//
// NOTE: "r" is not half red on purpose. In the pixel strings it has better
// contrast with "B" than does "R". eg. "rBBBrrBrB" vs "RBBBRRBRB".
var pixelsByChar = map[rune]*rgb{
	'_': {255, 255, 255}, // white
	'R': {255, 0, 0},     // red
	'B': {0, 0, 255},     // blue
	'G': {0, 255, 0},     // lime green
	'V': {191, 0, 64},    // average of 1*B and 3*R
	'S': {255, 63, 63},   // R above R above _
	'C': {0, 255, 255},   // cyan
	'M': {255, 0, 255},   // magenta
	'Y': {255, 255, 0},   // yellow
	'K': {0, 0, 0},       // black
	'P': {50, 76, 103},   // blue CMYK with ICC
	'Q': {63, 88, 110},   // blue CMYK without ICC
	'r': {255, 0, 0},     // red
	'g': {0, 128, 0},     // half green
	'b': {0, 0, 128},     // half blue
	'v': {128, 0, 128},   // average of B and R
	's': {255, 127, 127}, // R above _
	't': {127, 255, 127}, // G above _
	'u': {128, 0, 127},   // r above B above _
	'h': {64, 0, 64},     // half average of B and R
	'a': {0, 0, 254},     // R in lossy JPG
	'p': {192, 0, 63},    // R above R above B above _
	'z': nil,
}

// parsePixels is parse_pixels in tests/draw/__init__.py.
func parsePixels(t *testing.T, pixels string) (width, height int, raw []*rgb) {
	t.Helper()
	var lines []string
	for line := range strings.SplitSeq(pixels, "\n") {
		line, _, _ = strings.Cut(line, "#")
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	for _, line := range lines {
		if len([]rune(line)) != len([]rune(lines[0])) {
			t.Fatal("All lines of pixels must have the same width")
		}
	}
	if len(lines) == 0 {
		t.Fatal("no lines of pixels")
	}
	width, height = len([]rune(lines[0])), len(lines)
	for _, line := range lines {
		for _, char := range line {
			pixel, ok := pixelsByChar[char]
			if !ok {
				t.Fatalf("unknown pixel character %q", char)
			}
			raw = append(raw, pixel)
		}
	}
	return width, height, raw
}

// assertPixels is assert_pixels in tests/draw/__init__.py: it checks the
// size of the image and the values of the pixels.
func assertPixels(t *testing.T, name, expectedPixels, html string) {
	t.Helper()
	expectedWidth, expectedHeight, expected := parsePixels(t, expectedPixels)
	width, height, pixels := htmlToPixels(t, html)
	if expectedWidth != width || expectedHeight != height {
		t.Fatalf("Images do not have the same sizes:\n- expected: %d × %d\n- result: %d × %d\n%s",
			expectedWidth, expectedHeight, width, height, pixelsString(pixels, width))
	}
	// Adapted: okage rounds the compositing of a color with an alpha to the
	// nearest value, and Ghostscript truncates it, so a pixel can differ by
	// 1 from the pixel of upstream.
	assertPixelsEqual(t, name, width, height, pixels, expected, 1)
}

// assertPixelsLog is assertPixels for a document that logs to log.
func assertPixelsLog(t *testing.T, name, expectedPixels, html string, log *logger) {
	t.Helper()
	expectedWidth, expectedHeight, expected := parsePixels(t, expectedPixels)
	pages, fc := drawTestPagesLog(t, html, testBaseURL, log)
	width, height, pixels := documentToPixels(t, pages, fc)
	if expectedWidth != width || expectedHeight != height {
		t.Fatalf("Images do not have the same sizes:\n- expected: %d × %d\n- result: %d × %d",
			expectedWidth, expectedHeight, width, height)
	}
	assertPixelsEqual(t, name, width, height, pixels, expected, 1)
}

// assertSameRenderings is assert_same_renderings in tests/draw/__init__.py:
// it renders the documents and checks that they are the same.
func assertSameRenderings(t *testing.T, name string, tolerance int, documents ...string) {
	t.Helper()
	assertSameRenderingsLog(t, name, tolerance, nil, documents...)
}

// assertSameRenderingsLog is assertSameRenderings for documents that log
// to log.
func assertSameRenderingsLog(t *testing.T, name string, tolerance int, log *logger, documents ...string) {
	t.Helper()
	render := func(document string) (int, int, []rgb) {
		pages, fc := drawTestPagesLog(t, document, testBaseURL, log)
		return documentToPixels(t, pages, fc)
	}
	width, height, reference := render(documents[0])
	expected := make([]*rgb, len(reference))
	for i := range reference {
		expected[i] = &reference[i]
	}
	for i, document := range documents[1:] {
		_, _, pixels := render(document)
		assertPixelsEqual(t, fmt.Sprintf("%s_%d", name, i+1), width, height, pixels, expected, tolerance)
	}
}

// assertDifferentRenderings is assert_different_renderings in
// tests/draw/__init__.py: it renders the documents and checks that they are
// different.
func assertDifferentRenderings(t *testing.T, name string, documents ...string) {
	t.Helper()
	pixelsList := make([][]rgb, len(documents))
	for i, document := range documents {
		_, _, pixelsList[i] = htmlToPixels(t, document)
	}
	for i, pixels1 := range pixelsList {
		for j := i + 1; j < len(pixelsList); j++ {
			if equalPixels(pixels1, pixelsList[j]) {
				t.Errorf("%s_%d and %s_%d are the same", name, i+1, name, j+1)
			}
		}
	}
}

// equalPixels reports whether two images have the same pixels.
func equalPixels(a, b []rgb) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// assertPixelsEqual is assert_pixels_equal in tests/draw/__init__.py: it
// takes two matrices of pixels and checks that they are the same. A nil
// expected pixel is not compared. The message of a failure shows the pixels
// of the result, with the character of the nearest color.
func assertPixelsEqual(t *testing.T, name string, width, height int, raw []rgb, expected []*rgb, tolerance int) {
	t.Helper()
	for i := range max(len(raw), len(expected)) {
		var value rgb
		if i < len(raw) {
			value = raw[i]
		} else {
			value = rgb{-1, -1, -1}
		}
		if i >= len(expected) {
			t.Fatalf("%s: more pixels than expected", name)
		}
		want := expected[i]
		if want == nil {
			continue
		}
		for c := range 3 {
			if d := value[c] - want[c]; d > tolerance || -d > tolerance {
				writePNG(t, name, raw, width, len(raw)/max(width, 1))
				if width*height > maxDumpPixels {
					t.Fatalf("Pixel (%d, %d) in %s: expected rgba%v, got rgba%v", i%width, i/width, name, *want, value)
				}
				t.Fatalf("Pixel (%d, %d) in %s: expected rgba%v, got rgba%v\ngot:\n%s\nwant:\n%s",
					i%width, i/width, name, *want, value, pixelsString(raw, width), expectedString(expected, width))
			}
		}
	}
}

// maxDumpPixels is the number of pixels of the largest image that the
// message of a failure shows.
const maxDumpPixels = 64 * 64

// writePNG is write_png in tests/draw/__init__.py: it writes the pixels of a
// failure as a PNG file in the folder of the environment variable
// LAYOUT_DRAW_RESULTS, when it is set.
func writePNG(t *testing.T, name string, pixels []rgb, width, height int) {
	t.Helper()
	dir := os.Getenv("LAYOUT_DRAW_RESULTS")
	if dir == "" || width <= 0 || height <= 0 {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i, p := range pixels[:width*height] {
		img.Set(i%width, i/width, color.RGBA{uint8(p[0]), uint8(p[1]), uint8(p[2]), 255})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Log(err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name+".png"), b.Bytes(), 0o644); err != nil {
		t.Log(err)
	}
}

// pixelsString writes the pixels as lines of the characters of the nearest
// colors of pixelsByChar, for the messages of the tests.
func pixelsString(raw []rgb, width int) string {
	const chars = "_RBGVSCMYKgbvstuh"
	var b strings.Builder
	for i, p := range raw {
		best, bestDistance := '?', math.MaxInt
		for _, c := range chars {
			want := pixelsByChar[c]
			d := 0
			for k := range 3 {
				d += (p[k] - want[k]) * (p[k] - want[k])
			}
			if d < bestDistance {
				best, bestDistance = c, d
			}
		}
		if bestDistance > 3*20*20 {
			best = '?'
		}
		b.WriteRune(best)
		if width > 0 && i%width == width-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// expectedString writes the expected pixels as lines of characters.
func expectedString(expected []*rgb, width int) string {
	raw := make([]rgb, len(expected))
	for i, p := range expected {
		if p == nil {
			raw[i] = rgb{-1000, -1000, -1000}
			continue
		}
		raw[i] = *p
	}
	return strings.ReplaceAll(pixelsString(raw, width), "?", "z")
}

// htmlToPixels is html_to_pixels in tests/draw/__init__.py: it renders an
// HTML document and returns its pixels.
func htmlToPixels(t *testing.T, html string) (width, height int, pixels []rgb) {
	t.Helper()
	pages, fc := drawTestPages(t, html)
	return documentToPixels(t, pages, fc)
}

// drawTestPages lays out a document of the tests and returns its pages, and
// its font configuration.
func drawTestPages(t *testing.T, html string) ([]*Box, *FontConfiguration) {
	t.Helper()
	return drawTestPagesLog(t, html, testBaseURL, nil)
}

// drawTestPagesLog is drawTestPages with a base URL and a logger. The
// document reads each file of testdata, as FakeHTML reads the resources of
// upstream.
func drawTestPagesLog(t *testing.T, html, baseURL string, log *logger) ([]*Box, *FontConfiguration) {
	t.Helper()
	return drawTestPagesWith(t, html, drawTestOptions{baseURL: baseURL, log: log})
}

// drawTestOptions are the options of drawTestPagesWith.
type drawTestOptions struct {
	// baseURL is the URL of the document, testBaseURL by default.
	baseURL string
	log     *logger
	// files are files that the document reads, with the files of testdata.
	files map[string][]byte
	// userCSS are the user style sheets, the stylesheets of render.
	userCSS []string
	// mediaType is the media type of the document, "print" by default.
	mediaType string
}

// drawTestPagesWith lays out a document of the tests with options, and
// returns its pages and its font configuration.
func drawTestPagesWith(t *testing.T, html string, opts drawTestOptions) ([]*Box, *FontConfiguration) {
	t.Helper()
	if opts.baseURL == "" {
		opts.baseURL = testBaseURL
	}
	p := drawParseBase(t, html, opts)
	root, err := p.builder.buildFormattingStructure(p.html.EtreeElement)
	if err != nil {
		t.Fatal(err)
	}
	doc := NewDocument(p.html, root, p.styleFor, DocumentOptions{
		FontConfig:      p.fc,
		TargetCollector: p.builder.targetCollector,
		CounterStyle:    p.builder.counterStyle,
		Footnotes:       p.builder.footnotes,
	})
	pages, err := doc.All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return pages, p.fc
}

// drawParseBase is parseBase, with each file of testdata in the files of
// the document, so that the tests of the drawing find the resources of
// upstream that testdata holds.
func drawParseBase(t *testing.T, source string, opts drawTestOptions) parsed {
	t.Helper()
	h := fakeHTML(t, source, opts.baseURL, opts.log)
	if opts.mediaType != "" {
		h.MediaType = opts.mediaType
	}
	fsys := h.URLFetcher.FS.(fstest.MapFS)
	for _, dir := range []string{".", "sub_directory"} {
		entries, err := os.ReadDir(filepath.Join("testdata", dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				continue
			}
			name := path.Join(dir, entry.Name())
			data, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			fsys[name] = &fstest.MapFile{Data: data}
		}
	}
	for name, data := range opts.files {
		fsys[name] = &fstest.MapFile{Data: data}
	}
	fc := testFontConfig(opts.log)
	targetCollector := NewTargetCollector(opts.log)
	counterStyle := CounterStyle{}
	userStylesheets := make([]CSSSource, len(opts.userCSS))
	for i, css := range opts.userCSS {
		userStylesheets[i] = CSSSource{Text: css}
	}
	styleFor := computedStyles(t, h, ComputedStylesOptions{
		FontConfig:      fc,
		CounterStyle:    counterStyle,
		TargetCollector: targetCollector,
		UserStylesheets: userStylesheets,
	})
	b := newBuilder(h, styleFor, BuildOptions{TargetCollector: targetCollector, CounterStyle: counterStyle})
	return parsed{html: h, styleFor: styleFor, builder: b, fc: fc}
}

// documentToPixels is document_to_pixels in tests/draw/__init__.py, with
// document_write_png of tests/conftest.py: it draws each page at 96 dots
// per inch, one CSS pixel for each pixel, without antialiasing and without
// interpolation, as Ghostscript does with -dGraphicsAlphaBits=1, over a
// white page, with its bleed. The pages are stacked from top to bottom, and centered.
func documentToPixels(t *testing.T, pages []*Box, fc *FontConfiguration) (width, height int, pixels []rgb) {
	t.Helper()
	return documentToPixelsAt(t, pages, fc, 96)
}

// documentToPixelsAt is documentToPixels at a resolution in dots per inch,
// the resolution of write_png.
func documentToPixelsAt(t *testing.T, pages []*Box, fc *FontConfiguration, resolution float64) (width, height int, pixels []rgb) {
	t.Helper()
	scale := resolution / 96
	images := make([]*okage.Pixmap, len(pages))
	for i, page := range pages {
		// The page of PDF holds the bleed of the page.
		area := page.BleedArea()
		w := int(math.Round(area[2] * scale))
		h := int(math.Round(area[3] * scale))
		dst, ok := okage.NewPixmap(uint32(max(w, 1)), uint32(max(h, 1)))
		if !ok {
			t.Fatalf("page of %d by %d pixels", w, h)
		}
		dst.Fill(okage.ColorWhite)
		dev := display.NewOkage(dst, okage.IdentityTransform())
		dev.SetAntiAlias(false)
		toDevice := okage.TransformFromScale(float32(scale), float32(scale)).
			PreTranslate(float32(-area[0]), float32(-area[1]))
		err := DrawPage(context.Background(), dev, page, toDevice, DrawOptions{
			Aliased: true, Sharp: true, FontConfig: fc,
		})
		if err != nil {
			if _, ok := errors.AsType[*ivy.PartialError](err); !ok {
				t.Fatal(err)
			}
			t.Logf("page %d drawn in part: %v", i+1, err)
		}
		images[i] = dst
		width = max(width, int(dst.Width()))
		height += int(dst.Height())
	}
	pixels = make([]rgb, width*height)
	top := 0
	for _, img := range images {
		left := (width - int(img.Width())) / 2
		data := img.Data()
		for y := range int(img.Height()) {
			for x := range int(img.Width()) {
				i := 4 * (y*int(img.Width()) + x)
				pixels[(top+y)*width+left+x] = rgb{int(data[i]), int(data[i+1]), int(data[i+2])}
			}
		}
		top += int(img.Height())
	}
	return width, height, pixels
}

// TestDrawPageCanceled checks that a canceled context stops the drawing.
func TestDrawPageCanceled(t *testing.T) {
	pages, fc := drawTestPages(t, `<style>@page { size: 10px }</style><body style="background: red">`)
	dst, _ := okage.NewPixmap(10, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := DrawPage(ctx, display.NewOkage(dst, okage.IdentityTransform()), pages[0], okage.IdentityTransform(),
		DrawOptions{FontConfig: fc})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestDrawSmoke draws a background, as the first test of
// test_canvas_background.
func TestDrawSmoke(t *testing.T) {
	assertPixels(t, "smoke", strings.Repeat(strings.Repeat("B", 10)+"\n", 10), `
           <style>
             @page { size: 10px }
             /* body’s background propagates to the whole canvas */
             body { margin: 2px; background: #00f; height: 5Px }
           </style>
         <body>`)
}
