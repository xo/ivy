// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/test_unicode.py: the texts and the names of files
// in Unicode.

package layout

import (
	"os"
	"testing"
)

// TestUnicode is test_unicode in tests/test_unicode.py.
//
// Port: upstream writes the style sheet, the image and the document in a
// folder. The port gives them to the document as files, at the root of its
// files.
func TestUnicode(t *testing.T) {
	text := "I løvë Unicode"
	style := `
      @page { size: 200px 50px }
      p { color: blue }
    `
	expectedWidth, expectedHeight, expectedLines := htmlToPixels(t, `
      <style>`+style+`</style>
      <p><img src="pattern.png"> `+text+`</p>
    `)

	image, err := os.ReadFile("testdata/pattern.png")
	if err != nil {
		t.Fatal(err)
	}
	htmlContent := `
      <link rel=stylesheet href="style.css">
      <p><img src="pattern.png"> ` + text + `</p>
    `
	pages, fc := drawTestPagesWith(t, htmlContent, drawTestOptions{
		baseURL: pathToURL("doc.html"),
		files:   map[string][]byte{"style.css": []byte(style), "pattern.png": image},
	})
	width, height, lines := documentToPixels(t, pages, fc)
	if expectedWidth != width || expectedHeight != height {
		t.Fatalf("size %d × %d, want %d × %d", width, height, expectedWidth, expectedHeight)
	}
	expected := make([]*rgb, len(expectedLines))
	for i := range expectedLines {
		expected[i] = &expectedLines[i]
	}
	assertPixelsEqual(t, "unicode", width, height, lines, expected, 0)
}
