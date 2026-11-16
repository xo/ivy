// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/test_acid2.py: the famous Acid2 test.

package layout

import (
	"log/slog"
	"os"
	"testing"
)

// TestAcid2 is test_acid2 in tests/test_acid2.py.
func TestAcid2(t *testing.T) {
	// Reduce the size of the image and avoid the rounding problems of
	// Ghostscript.
	stylesheets := []string{"@page { size: 500px 800px }"}

	render := func(filename string, log *logger) ([]*Box, *FontConfiguration) {
		data, err := os.ReadFile("testdata/" + filename)
		if err != nil {
			t.Fatal(err)
		}
		return drawTestPagesWith(t, string(data), drawTestOptions{
			baseURL: pathToURL(filename), log: log, userCSS: stylesheets,
		})
	}

	// This is a copy of https://www.webstandards.org/files/acid2/test.html.
	log, _ := captureLogs(slog.LevelInfo)
	pages, fc := render("acid2-test.html", log)
	if len(pages) != 2 {
		t.Fatalf("acid2-test.html has %d pages, want 2", len(pages))
	}
	// Ignore the intro page: it is not in the reference.
	_, _, testPixels := documentToPixels(t, pages[1:], fc)

	// This is a copy of https://www.webstandards.org/files/acid2/reference.html.
	refPages, refFC := render("acid2-reference.html", nil)
	width, height, refPixels := documentToPixels(t, refPages, refFC)
	if len(testPixels) != width*height {
		t.Fatalf("the test page has %d pixels, want %d", len(testPixels), width*height)
	}
	// Adapted: the diagonal edges of the nose run through the corners of
	// the pixels. Without antialiasing, Ghostscript paints each pixel that a
	// path touches, and okage each pixel whose center a path covers. So the
	// pixels along these edges differ, and the test accepts 32 of them.
	var different []int
	for i := range testPixels {
		for c := range 3 {
			if d := testPixels[i][c] - refPixels[i][c]; d > 2 || -d > 2 {
				different = append(different, i)
				break
			}
		}
	}
	if len(different) > 32 {
		writePNG(t, "acid2", testPixels, width, height)
		writePNG(t, "acid2.expected", refPixels, width, height)
		i := different[0]
		t.Fatalf("%d pixels differ, the first one at (%d, %d): expected rgba%v, got rgba%v",
			len(different), i%width, i/width, refPixels[i], testPixels[i])
	}
}
