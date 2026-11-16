// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_descriptors.py, the tests of the
// descriptors of the at-rules.

package layout

import (
	"log/slog"
	"slices"
	"testing"

	"github.com/xo/okage/tinycss2"
)

// fontFaceDescriptors parses an @font-face rule and returns its descriptors,
// as the tests of upstream do with preprocess_descriptors.
func fontFaceDescriptors(t *testing.T, log *logger, css string) []nameValue {
	t.Helper()
	stylesheet := tinycss2.ParseStylesheet(css)
	if len(stylesheet) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(stylesheet))
	}
	atRule, ok := stylesheet[0].(*tinycss2.AtRule)
	if !ok || atRule.AtKeyword != "font-face" {
		t.Fatalf("expected an @font-face rule, got: %v", stylesheet[0])
	}
	return preprocessDescriptors(log, "font-face", "https://weasyprint.org/foo/",
		tinycss2.ParseBlocksContents(atRule.Content))
}

// assertDescriptors checks the names and the values of descriptors.
func assertDescriptors(t *testing.T, got, want []nameValue) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d descriptors, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i].name != want[i].name || !Equal(got[i].value, want[i].value) {
			t.Errorf("descriptor %d: got (%s, %s), want (%s, %s)", i,
				got[i].name, ValueString(got[i].value), want[i].name, ValueString(want[i].value))
		}
	}
}

// external returns the tuple ('external', url).
func external(u string) Value {
	return Tuple{String("external"), String(u)}
}

func TestFontFace1(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	got := fontFaceDescriptors(t, log,
		"@font-face {"+
			"  font-family: Gentium Hard;"+
			"  src: url(https://example.com/fonts/Gentium.woff);"+
			"}")
	assertDescriptors(t, got, []nameValue{
		{"font_family", String("Gentium Hard")},
		{"src", Tuple{external("https://example.com/fonts/Gentium.woff")}},
	})
	assertNoLogs(t, capture)
}

func TestFontFace2(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	got := fontFaceDescriptors(t, log,
		"@font-face {"+
			`  font-family: "Fonty Smiley";`+
			"  src: url(Fonty-Smiley.woff);"+
			"  font-style: italic;"+
			"  font-weight: 200;"+
			"  font-stretch: condensed;"+
			"}")
	assertDescriptors(t, got, []nameValue{
		{"font_family", String("Fonty Smiley")},
		{"src", Tuple{external("https://weasyprint.org/foo/Fonty-Smiley.woff")}},
		{"font_style", String("italic")},
		{"font_weight", Int(200)},
		{"font_stretch", String("condensed")},
	})
	assertNoLogs(t, capture)
}

func TestFontFace3(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	got := fontFaceDescriptors(t, log,
		"@font-face {"+
			"  font-family: Gentium Hard;"+
			"  src: local();"+
			"}")
	assertDescriptors(t, got, []nameValue{
		{"font_family", String("Gentium Hard")},
		{"src", Tuple{Tuple{String("local"), nil}}},
	})
	assertNoLogs(t, capture)
}

func TestFontFace4(t *testing.T) {
	// Regression test for #487.
	log, capture := captureLogs(slog.LevelInfo)
	got := fontFaceDescriptors(t, log,
		"@font-face {"+
			"  font-family: Gentium Hard;"+
			"  src: local(Gentium Hard);"+
			"}")
	assertDescriptors(t, got, []nameValue{
		{"font_family", String("Gentium Hard")},
		{"src", Tuple{Tuple{String("local"), String("Gentium Hard")}}},
	})
	assertNoLogs(t, capture)
}

func TestFontFace5(t *testing.T) {
	// Regression test for #1653.
	log, capture := captureLogs(slog.LevelInfo)
	got := fontFaceDescriptors(t, log,
		"@font-face {"+
			"  font-family: Gentium Hard;"+
			"  src: local(Gentium Hard);"+
			"  src: local(Gentium Soft),"+
			"}")
	assertDescriptors(t, got, []nameValue{
		{"font_family", String("Gentium Hard")},
		{"src", Tuple{Tuple{String("local"), String("Gentium Hard")}}},
	})
	assertLog(t, capture, "invalid value")
}

func TestFontFaceBad1(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	got := fontFaceDescriptors(t, log,
		"@font-face {"+
			`  font-family: "Bad Font";`+
			"  src: url(BadFont.woff);"+
			"  font-stretch: expanded;"+
			"  font-style: wrong;"+
			"  font-weight: bolder;"+
			"  font-stretch: wrong;"+
			"}")
	assertDescriptors(t, got, []nameValue{
		{"font_family", String("Bad Font")},
		{"src", Tuple{external("https://weasyprint.org/foo/BadFont.woff")}},
		{"font_stretch", String("expanded")},
	})
	want := []string{
		"WARNING: Ignored `font-style: wrong` at 1:91, invalid value.",
		"WARNING: Ignored `font-weight: bolder` at 1:111, invalid value.",
		"WARNING: Ignored `font-stretch: wrong` at 1:133, invalid value.",
	}
	if logs := capture.logs(); !slices.Equal(logs, want) {
		t.Errorf("got logs %q, want %q", logs, want)
	}
}

// preprocessStylesheetLogs is preprocess_stylesheet('print',
// 'https://wp.org/foo/', stylesheet, ...) of the tests of upstream. It
// returns the log.
func preprocessStylesheetLogs(css string) []string {
	log, capture := captureLogs(slog.LevelInfo)
	ctx := &cssContext{
		log:           log,
		fetcher:       &URLFetcher{},
		counterStyle:  CounterStyle{},
		colorProfiles: map[string]*ColorProfile{},
	}
	preprocessStylesheet(ctx, "print", "https://wp.org/foo/", tinycss2.ParseStylesheet(css), newShared(nil), false)
	return capture.logs()
}

func TestFontFaceBad(t *testing.T) {
	for _, c := range []struct {
		css  string
		want []string
	}{
		{"@font-face{}", []string{
			"WARNING: Missing src descriptor in '@font-face' rule at 1:1",
		}},
		{"@font-face{src: url(test.woff)}", []string{
			"WARNING: Missing font-family descriptor in '@font-face' rule at 1:1",
		}},
		{"@font-face{font-family: test}", []string{
			"WARNING: Missing src descriptor in '@font-face' rule at 1:1",
		}},
		{"@font-face { font-family: test; src: wrong }", []string{
			"WARNING: Ignored `src: wrong ` at 1:33, invalid value.",
			"WARNING: Missing src descriptor in '@font-face' rule at 1:1",
		}},
		{"@font-face { font-family: good, bad; src: url(test.woff) }", []string{
			"WARNING: Ignored `font-family: good, bad` at 1:14, invalid value.",
			"WARNING: Missing font-family descriptor in '@font-face' rule at 1:1",
		}},
		{"@font-face { font-family: good, bad; src: really bad }", []string{
			"WARNING: Ignored `font-family: good, bad` at 1:14, invalid value.",
			"WARNING: Ignored `src: really bad ` at 1:38, invalid value.",
			"WARNING: Missing src descriptor in '@font-face' rule at 1:1",
		}},
	} {
		if logs := preprocessStylesheetLogs(c.css); !slices.Equal(logs, c.want) {
			t.Errorf("%s: got logs %q, want %q", c.css, logs, c.want)
		}
	}
}

func TestCounterStyleInvalid(t *testing.T) {
	for _, rule := range []string{
		"@counter-style test {system: alphabetic; symbols: a}",
		"@counter-style test {system: cyclic}",
		"@counter-style test {system: additive; additive-symbols: a 1}",
		"@counter-style test {system: additive; additive-symbols: 10 x, 1 i, 5 v}",
	} {
		if logs := preprocessStylesheetLogs(rule); len(logs) < 1 {
			t.Errorf("%s: expected a log", rule)
		}
	}
}
