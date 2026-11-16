// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file holds the tests of the port of weasyprint/text/fonts.py: the
// fonts of the @font-face rules, their unicode-range and their features,
// and font_features. Upstream tests the fonts in tests/test_fonts.py with
// render_pages, so the tests of the port measure the text of a style
// instead.

package layout

import (
	"log/slog"
	"math"
	"slices"
	"strings"
	"testing"
)

// fontFaceStyle computes the style of the first <p> of a document with a
// style sheet of @font-face rules, with the fonts of the tests, and
// returns it with the log.
func fontFaceStyle(t *testing.T, css string) (*Style, *logCapture) {
	t.Helper()
	log, capture := captureLogs(slog.LevelDebug)
	p := parseBase(t, "<style>"+css+"</style><p>a", testBaseURL, log)
	return p.styleFor.Get(findElement(p.html.EtreeElement, "p"), ""), capture
}

// textWidth returns the width of a text in a style, without a limit.
func textWidth(style *Style, text string) float64 {
	line, _, _ := CreateLayout(text, style, nil, 0).FirstLine()
	return line.Width
}

// TestFontFace checks that the @font-face rules load their fonts, in OTF
// and in WOFF, and from a local font, with their family.
func TestFontFace(t *testing.T) {
	for _, c := range []struct {
		name, css string
		want      float64
	}{
		{"otf", `@font-face { src: url(weasyprint.otf); font-family: wp } p { font-family: wp; font-size: 10px }`, 30},
		{"woff", `@font-face { src: url(weasyprint.woff); font-family: wp } p { font-family: wp; font-size: 10px }`, 30},
		{"fallback src", `@font-face { src: url(missing.otf), url(weasyprint.woff) format("woff"); font-family: wp } p { font-family: wp; font-size: 10px }`, 30},
		// The test style sheet of the user agent loads weasyprint.otf.
		{"user agent", `p { font-family: weasyprint; font-size: 10px }`, 30},
		{"local", `@font-face { src: local("Go Mono"); font-family: x } p { font-family: x; font-size: 10px }`, 18},
		{"local PostScript", `@font-face { src: local(GoMono-Bold); font-family: x } p { font-family: x; font-size: 10px }`, 18},
	} {
		t.Run(c.name, func(t *testing.T) {
			style, _ := fontFaceStyle(t, c.css)
			if got := textWidth(style, "abc"); !approx(got, c.want) {
				t.Errorf("width %v, want %v", got, c.want)
			}
		})
	}
}

// TestFontFaceMissing checks the warning of a font that cannot be loaded.
func TestFontFaceMissing(t *testing.T) {
	_, capture := fontFaceStyle(t, `@font-face { src: url(missing.otf); font-family: x }`)
	if !slices.ContainsFunc(capture.logs(), func(s string) bool {
		return strings.HasPrefix(s, "WARNING: Font-face") && strings.Contains(s, "cannot be loaded")
	}) {
		t.Errorf("logs: %q", capture.logs())
	}
}

// TestFontFaceUnicodeRange checks that a face of an @font-face rule draws
// only the characters of its unicode-range, and that the next family draws
// the others.
func TestFontFaceUnicodeRange(t *testing.T) {
	style, _ := fontFaceStyle(t, `
      @font-face { src: url(weasyprint.otf); font-family: wp; unicode-range: U+61 }
      p { font-family: wp, Go Mono; font-size: 10px }`)
	if got := textWidth(style, "aa"); got != 20 {
		t.Errorf("a: %v", got)
	}
	// Go Mono is 0.6em wide.
	if got := textWidth(style, "bb"); !approx(got, 12) {
		t.Errorf("b: %v", got)
	}
}

// TestFontFaceStyle checks that the descriptors of an @font-face rule
// select the face.
func TestFontFaceStyle(t *testing.T) {
	style, _ := fontFaceStyle(t, `
      @font-face { src: url(weasyprint.otf); font-family: x; font-weight: bold }
      @font-face { src: local(GoMono); font-family: x; font-weight: normal }
      p { font-family: x; font-size: 10px; font-weight: bold }`)
	if got := textWidth(style, "a"); got != 10 {
		t.Errorf("bold: %v", got)
	}
	style.Set("font_weight", Int(400))
	if got := textWidth(style, "a"); !approx(got, 6) {
		t.Errorf("normal: %v", got)
	}
}

// TestFontFeatures checks font_features.
func TestFontFeatures(t *testing.T) {
	features := fontFeatures(fontFeatureArgs{
		kerning:    String("none"),
		ligatures:  tupleOf("no-common-ligatures", "discretionary-ligatures"),
		position:   String("super"),
		caps:       String("all-small-caps"),
		numeric:    tupleOf("oldstyle-nums", "slashed-zero"),
		alternates: String("historical-forms"),
		eastAsian:  tupleOf("ruby"),
		settings:   Tuple{Tuple{String("liga"), Int(1)}, Tuple{String("ss01"), Int(2)}},
	})
	got := make([]string, 0, len(features))
	for _, f := range features {
		got = append(got, f.tag+"="+string(rune('0'+f.value)))
	}
	want := "kern=0 liga=1 clig=0 dlig=1 sups=1 c2sc=1 smcp=1 onum=1 zero=1 hist=1 ruby=1 ss01=2"
	if strings.Join(got, " ") != want {
		t.Errorf("got %s", strings.Join(got, " "))
	}
	if features := fontFeatures(fontFeatureArgs{
		kerning: String("auto"), ligatures: String("none"), position: String("normal"),
		caps: String("normal"), numeric: String("normal"), alternates: String("normal"),
		eastAsian: String("normal"), settings: String("normal"),
	}); len(features) != 5 {
		t.Errorf("none: %v", features)
	}
}

// TestFontFallback checks that a character that the fonts of the style do
// not draw takes a font of the database that draws it.
func TestFontFallback(t *testing.T) {
	style := textStyle(t, weasyprint(nil))
	// weasyprint.otf has no glyph for U+2192, which Go draws.
	fc := fontConfigOf(style)
	s := fc.shape("a→", style)
	if len(s.runs) != 2 || s.runs[0].id == s.runs[1].id {
		t.Errorf("runs: %+v", s.runs)
	}
}

// approx reports whether a and b are equal within 0.01, as pytest.approx
// does for the widths of the fonts. Go Mono is 1229/2048 em wide, not 0.6.
func approx(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}
