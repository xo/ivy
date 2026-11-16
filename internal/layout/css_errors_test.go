// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_errors.py and tests/css/test_ua.py.

package layout

import (
	"log/slog"
	"strings"
	"testing"
)

// newTestCSS is CSS(string=source) with a logger.
func newTestCSS(log *logger, source string) *CSS {
	ctx := &cssContext{
		log:           log,
		fetcher:       &URLFetcher{},
		counterStyle:  CounterStyle{},
		colorProfiles: map[string]*ColorProfile{},
		budget:        newCascadeBudget(defaultLimits),
	}
	return newCSS(ctx, CSSSource{Text: source}, "print", nil)
}

// TestWarnings is test_warnings.
func TestWarnings(t *testing.T) {
	for _, c := range []struct {
		source   string
		messages []string
	}{
		{":lipsum { margin: 2cm", []string{"WARNING: Invalid or unsupported selector"}},
		{"::lipsum { margin: 2cm", []string{"WARNING: Invalid or unsupported selector"}},
		{"foo { margin-color: red", []string{"WARNING: Ignored", "unknown property"}},
		{"foo { margin-top: red", []string{"WARNING: Ignored", "invalid value"}},
		{`@import "relative-uri.css"`, []string{"ERROR: Relative URI reference"}},
		{`@import "invalid-protocol://absolute-URL"`, []string{"ERROR: Failed to load stylesheet"}},
		{"test", []string{"WARNING: Parse error"}},
		{"@test", []string{"WARNING: Unknown empty rule"}},
		{"@test {}", []string{"WARNING: Unknown rule"}},
		{`@import::before "test.css"`, []string{"WARNING: Invalid @import"}},
		{`@page { @top-left super { content: "a" } }`, []string{"WARNING: Invalid prelude"}},
		{`@page { @unknown { content: "a" } }`, []string{"WARNING: Unknown @page rule"}},
		{`@page { @footnote::before { content: "a" } }`, []string{"WARNING: Invalid prelude"}},
	} {
		log, capture := captureLogs(slog.LevelInfo)
		newTestCSS(log, c.source)
		logs := capture.logs()
		if len(logs) != 1 {
			t.Errorf("%s: expected 1 log, got: %q", c.source, logs)
			continue
		}
		for _, message := range c.messages {
			if !strings.Contains(logs[0], message) {
				t.Errorf("%s: expected %q in %q", c.source, message, logs[0])
			}
		}
	}
}

// TestWarningsStylesheet is test_warnings_stylesheet. Upstream renders the
// pages. The port computes the styles, which load the style sheets.
func TestWarningsStylesheet(t *testing.T) {
	_, _, capture := cascade(t, "<link rel=stylesheet href=invalid-protocol://absolute>")
	logs := capture.logs()
	if len(logs) != 1 || !strings.Contains(logs[0], "ERROR: Failed to load stylesheet at") {
		t.Errorf("expected 1 error, got: %q", logs)
	}
}

// TestErrorRecovery is test_error_recovery. Upstream reads the style of the
// box of a rendered page. The port reads the computed style of the element.
func TestErrorRecovery(t *testing.T) {
	for _, style := range []string{
		"<style> html { color red; color: blue; color",
		`<html style="color; color: blue; color red">`,
	} {
		document, styleFor, capture := cascade(t, style)
		assertRGBA(t, "color", styleOf(t, document, styleFor, "html").Get("color"), 0, 0, 1, 1) // blue
		if logs := capture.logs(); len(logs) != 2 {
			t.Errorf("%s: expected 2 logs, got: %q", style, logs)
		}
	}
}

// TestUAStylesheets is test_ua_stylesheets.
func TestUAStylesheets(t *testing.T) {
	for name, css := range map[string]string{"UA": uaCSS, "UA_FORM": uaFormCSS, "PH": phCSS} {
		log, capture := captureLogs(slog.LevelInfo)
		newTestCSS(log, css)
		for _, message := range capture.logs() {
			t.Errorf("%s: unexpected log %s", name, message)
		}
	}
}
