// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports the parts of tests/test_api.py that draw pixels. The other
// tests of the module check the API of Python, the command line, the PDF
// files and the fetchers of URLs of upstream, which ivy does not have.

package layout

import (
	"log/slog"
	"strings"
	"testing"
)

// checkPNGPattern is check_png_pattern in tests/test_api.py, for the pixels
// of a document.
func checkPNGPattern(t *testing.T, width, height int, pixels []rgb, x2, blank, rotated bool) {
	t.Helper()
	var expectedPixels string
	switch {
	case blank:
		expectedPixels = `
            ________
            ________
            ________
            ________
            ________
            ________
            ________
            ________
        `
	case x2:
		expectedPixels = `
            ________________
            ________________
            ________________
            ________________
            ____rrBBBBBB____
            ____rrBBBBBB____
            ____BBBBBBBB____
            ____BBBBBBBB____
            ____BBBBBBBB____
            ____BBBBBBBB____
            ____BBBBBBBB____
            ____BBBBBBBB____
            ________________
            ________________
            ________________
            ________________
        `
	case rotated:
		expectedPixels = `
            ________
            ________
            __BBBB__
            __BBBB__
            __BBBB__
            __rBBB__
            ________
            ________
        `
	default:
		expectedPixels = `
            ________
            ________
            __rBBB__
            __BBBB__
            __BBBB__
            __BBBB__
            ________
            ________
        `
	}
	expectedWidth, expectedHeight, expected := parsePixels(t, expectedPixels)
	if width != expectedWidth || height != expectedHeight {
		t.Fatalf("size %d × %d, want %d × %d", width, height, expectedWidth, expectedHeight)
	}
	assertPixelsEqual(t, "png_pattern", width, height, pixels, expected, 0)
}

// TestAPIPythonRender is test_python_render in tests/test_api.py, for its
// pixels.
func TestAPIPythonRender(t *testing.T) {
	htmlString := `<body><img src=pattern.png>`
	cssString := `
        @page { margin: 2px; size: 8px }
        body { margin: 0; font-size: 0 }
        img { image-rendering: pixelated }

        @media screen { img { transform: rotate(-90deg) } }
    `
	pages, fc := drawTestPagesWith(t, htmlString, drawTestOptions{userCSS: []string{cssString}})
	width, height, pixels := documentToPixels(t, pages, fc)
	checkPNGPattern(t, width, height, pixels, false, false, false)

	width, height, pixels = documentToPixelsAt(t, pages, fc, 192)
	checkPNGPattern(t, width, height, pixels, true, false, false)

	pages, fc = drawTestPagesWith(t, htmlString, drawTestOptions{userCSS: []string{cssString}, mediaType: "screen"})
	width, height, rotated := documentToPixels(t, pages, fc)
	checkPNGPattern(t, width, height, rotated, false, false, true)

	pages, fc = drawTestPagesWith(t, "<style>"+cssString+"</style>"+htmlString, drawTestOptions{mediaType: "screen"})
	_, _, pixels = documentToPixels(t, pages, fc)
	if !equalPixels(pixels, rotated) {
		t.Error("the style sheet of the document draws another image")
	}
}

// TestAPIUnicodeFilenames is test_unicode_filenames in tests/test_api.py,
// for its pixels. ivy reads no file of the system, so the names of the
// files are not tested.
func TestAPIUnicodeFilenames(t *testing.T) {
	width, height, pixels := htmlToPixels(t, `
        <style>
            @page { margin: 2px; size: 8px }
            html { background: #00f; }
            body { background: #f00; width: 1px; height: 1px }
        </style>
        <body>
    `)
	checkPNGPattern(t, width, height, pixels, false, false, false)
}

// TestAPILowLevelAPI is test_low_level_api in tests/test_api.py, for its
// pixels and the sizes of its pages.
func TestAPILowLevelAPI(t *testing.T) {
	css := `
        @page { margin: 2px; size: 8px }
        html { background: #00f; }
        body { background: #f00; width: 1px; height: 1px }
    `
	pages, fc := drawTestPagesWith(t, "<body>", drawTestOptions{userCSS: []string{css}})
	if len(pages) != 1 || pages[0].MarginWidth() != 8 || pages[0].MarginHeight() != 8 {
		t.Fatalf("pages: %v", pages)
	}
	width, height, pixels := documentToPixelsAt(t, pages, fc, 192)
	checkPNGPattern(t, width, height, pixels, true, false, false)

	// A resolution that is not a multiple of 96.
	if width, height, _ := documentToPixelsAt(t, pages, fc, 145.2); width != 12 || height != 12 {
		t.Errorf("size %d × %d at 145.2 dpi, want 12 × 12", width, height)
	}

	pages, fc = drawTestPages(t, `
        <style>
            @page:first { size: 5px 10px } @page { size: 6px 4px }
            p { page-break-before: always }
        </style>
        <p></p>
        <p></p>
    `)
	if len(pages) != 2 {
		t.Fatalf("%d pages, want 2", len(pages))
	}
	for i, want := range [][2]float64{{5, 10}, {6, 4}} {
		if w, h := pages[i].MarginWidth(), pages[i].MarginHeight(); w != want[0] || h != want[1] {
			t.Errorf("page %d is %g × %g, want %g × %g", i+1, w, h, want[0], want[1])
		}
	}
	// The largest width of the pages, and the sum of their heights.
	if width, height, _ := documentToPixels(t, pages, fc); width != 6 || height != 14 {
		t.Errorf("size %d × %d, want 6 × 14", width, height)
	}
	if width, height, _ := documentToPixels(t, pages[:1], fc); width != 5 || height != 10 {
		t.Errorf("size %d × %d, want 5 × 10", width, height)
	}
	if width, height, _ := documentToPixels(t, pages[1:], fc); width != 6 || height != 4 {
		t.Errorf("size %d × %d, want 6 × 4", width, height)
	}
}

// TestAPIURLFetcherDefault is test_url_fetcher_default in
// tests/test_api.py.
func TestAPIURLFetcherDefault(t *testing.T) {
	pages, fc := drawTestPagesWith(t, `<body><img src="pattern.png">`, drawTestOptions{
		userCSS: []string{"@page{size:8px;margin:2px} body{font-size:0}"},
	})
	width, height, pixels := documentToPixels(t, pages, fc)
	checkPNGPattern(t, width, height, pixels, false, false, false)
}

// TestAPIURLFetcherBadImage is test_url_fetcher_bad_image in
// tests/test_api.py. The fetcher of ivy reads no URL of the scheme custom.
func TestAPIURLFetcherBadImage(t *testing.T) {
	log, logs := captureLogs(slog.LevelInfo)
	pages, fc := drawTestPagesWith(t, `<body><img src="custom:foo/bar">`, drawTestOptions{
		log: log, userCSS: []string{"@page { size: 8px; margin: 2px } body { font-size: 0 }"},
	})
	messages := logs.logs()
	if len(messages) != 1 || !strings.HasPrefix(messages[0], "ERROR: Failed to load image at") {
		t.Errorf("logs: %q", messages)
	}
	width, height, pixels := documentToPixels(t, pages, fc)
	checkPNGPattern(t, width, height, pixels, false, true, false)
}
