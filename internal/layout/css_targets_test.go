// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_target.py.
//
// Upstream checks the text that target-counter(), target-counters() and
// target-text() give, which needs the boxes and the pagination of a later
// stage. The adapted tests here check the part that the cascade does: the
// computed content of the pseudo-elements, the anchors that the
// TargetCollector collects, and the targets that LookupTarget finds for
// each target function, also through attr().

package layout

import (
	"log/slog"
	"testing"
)

// targetsOf returns the anchor names that LookupTarget finds for the target
// functions of the content of the ::before of each element with an id, by
// id. It fails the test for a target that is not pending.
func targetsOf(t *testing.T, source string) (map[string][]string, *StyleFor, *HTML) {
	t.Helper()
	log, capture := captureLogs(slog.LevelInfo)
	document := fakeHTML(t, source, testBaseURL, log)
	collector := NewTargetCollector(log)
	sf := computedStyles(t, document, ComputedStylesOptions{TargetCollector: collector})
	result := map[string][]string{}
	for el := range document.EtreeElement.Iter() {
		id, ok := el.Get("id")
		if !ok {
			continue
		}
		for _, pseudo := range []string{"before", "after"} {
			style := sf.Get(el, pseudo)
			if style == nil {
				continue
			}
			content, _ := style.Get("content").(Tuple)
			for _, item := range content {
				item, ok := item.(Tuple)
				if !ok || !isStrIn(item[0], "target-counter()", "target-counters()", "target-text()") {
					continue
				}
				anchorToken := item[1].(Tuple)[0]
				target := collector.LookupTarget(anchorToken, &Box{Element: el, Style: style}, "content", func(map[string][]int) {})
				if target.State != "pending" {
					t.Errorf("%s: target %s is %s", id, ValueString(anchorToken), target.State)
					continue
				}
				for name, candidate := range collector.Targets {
					if candidate == target {
						result[id] = append(result[id], name)
					}
				}
			}
		}
	}
	if !collector.HadPendingTargets && len(result) > 0 {
		t.Error("expected pending targets")
	}
	assertNoLogs(t, capture)
	return result, sf, document
}

// assertTargets checks the anchor names of targetsOf.
func assertTargets(t *testing.T, got map[string][]string, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("got targets %v, want %v", got, want)
	}
	for id, name := range want {
		if len(got[id]) != 1 || got[id][0] != name {
			t.Errorf("%s: got targets %v, want %s", id, got[id], name)
		}
	}
}

// TestTargetCounter is test_target_counter. It checks the computed content
// and the targets instead of the text of the boxes.
func TestTargetCounter(t *testing.T) {
	got, sf, document := targetsOf(t, `
      <style>
        div:first-child { counter-reset: div }
        div { counter-increment: div }
        #id1::before { content: target-counter('#id4', div) }
        #id2::before { content: 'test ' target-counter('#id1', div) }
        #id3::before { content: target-counter(url(#id4), div, lower-roman) }
        #id4::before { content: target-counter('#id3', div) }
      </style>
      <body>
        <div id="id1"></div>
        <div id="id2"></div>
        <div id="id3"></div>
        <div id="id4"></div>`)
	assertTargets(t, got, map[string]string{"id1": "id4", "id2": "id1", "id3": "id4", "id4": "id3"})
	divs := elementsByTag(document.EtreeElement, "div")
	str := func(s string) Tuple { return Tuple{String("string"), String(s)} }
	for i, want := range []Tuple{
		{Tuple{String("target-counter()"), Tuple{str("#id4"), String("div"), String("decimal")}}},
		{str("test "), Tuple{String("target-counter()"), Tuple{str("#id1"), String("div"), String("decimal")}}},
		{Tuple{String("target-counter()"), Tuple{
			Tuple{String("url"), Tuple{String("internal"), String("id4")}}, String("div"), String("lower-roman"),
		}}},
		{Tuple{String("target-counter()"), Tuple{str("#id3"), String("div"), String("decimal")}}},
	} {
		assertValue(t, sf.Get(divs[i], "before"), "content", want)
	}
}

// TestTargetCounterAttr is test_target_counter_attr. It checks the targets
// that attr() gives instead of the text of the boxes.
func TestTargetCounterAttr(t *testing.T) {
	got, _, _ := targetsOf(t, `
      <style>
        div:first-child { counter-reset: div }
        div { counter-increment: div }
        div::before { content: target-counter(attr(data-count), div) }
        #id2::before { content: target-counter(attr(data-count url), div) }
        #id4::before {
          content: target-counter(attr(data-count), div, lower-alpha) }
      </style>
      <body>
        <div id="id1" data-count="#id4"></div>
        <div id="id2" data-count="#id1"></div>
        <div id="id3" data-count="#id2"></div>
        <div id="id4" data-count="#id3"></div>`)
	assertTargets(t, got, map[string]string{"id1": "id4", "id2": "id1", "id3": "id2", "id4": "id3"})
}

// TestTargetCounters is test_target_counters. It checks the targets instead
// of the text of the boxes.
func TestTargetCounters(t *testing.T) {
	got, _, _ := targetsOf(t, `
      <style>
        div:first-child { counter-reset: div }
        div { counter-increment: div }
        #id1-2::before { content: target-counters('#id4-2', div, '.') }
        #id2-1::before { content: target-counters(url(#id3), div, '++') }
        #id3::before {
          content: target-counters('#id2-1', div, '.', lower-alpha) }
        #id4-2::before {
          content: target-counters(attr(data-count, url), div, '') }
      </style>
      <body>
        <div id="id1"><div></div><div id="id1-2"></div></div>
        <div id="id2"><div id="id2-1"></div><div></div></div>
        <div id="id3"></div>
        <div id="id4">
          <div></div><div id="id4-2" data-count="#id1-2"></div>
        </div>`)
	assertTargets(t, got, map[string]string{
		"id1-2": "id4-2", "id2-1": "id3", "id3": "id2-1", "id4-2": "id1-2",
	})
}

// TestTargetText is test_target_text. It checks the targets instead of the
// text of the boxes.
func TestTargetText(t *testing.T) {
	got, _, _ := targetsOf(t, `
      <style>
        a { display: block; color: red }
        div:first-child { counter-reset: div }
        div { counter-increment: div }
        #id2::before { content: 'wow' }
        #link1::before { content: 'test ' target-text('#id4') }
        #link2::before { content: target-text(attr(data-count, url), before) }
        #link3::before { content: target-text('#id3', after) }
        #link4::before { content: target-text(url(#id1), first-letter) }
      </style>
      <body>
        <a id="link1"></a>
        <div id="id1">1 Chapter 1</div>
        <a id="link2" data-count="#id2"></a>
        <div id="id2">2 Chapter 2</div>
        <div id="id3">3 Chapter 3</div>
        <a id="link3"></a>
        <div id="id4">4 Chapter 4</div>
        <a id="link4"></a>`)
	assertTargets(t, got, map[string]string{"link1": "id4", "link2": "id2", "link3": "id3", "link4": "id1"})
}

// TestTargetTextWhitespaceAroundTarget is
// test_target_text_whitespace_around_target, a regression test for #1875.
func TestTargetTextWhitespaceAroundTarget(t *testing.T) {
	pages := renderPages(t, `
      <style>
        a::before { content: target-text(attr(href)) }
      </style>
      <p>before <a href="#ref"></a> after</p>
      <h2 id="ref">text</h2>`)
	_, body := pageBody(t, only(t, &Box{Children: pages}))
	p := unpack(t, body, 2)[0]
	line := only(t, p)
	children := unpack(t, line, 3)
	before, a, after := children[0], children[1], children[2]
	if before.Text != "before " || after.Text != " after" {
		t.Errorf("got %q and %q", before.Text, after.Text)
	}
	if got := only(t, only(t, a)).Text; got != "text" {
		t.Errorf("got target text %q", got)
	}
}

// targetAfter returns the text box and the ::after box of the link of the
// tests of the targets: the only inline of the only line of block.
func targetAfter(t *testing.T, block *Box) (textBox, after *Box) {
	t.Helper()
	children := unpack(t, only(t, only(t, block)), 2)
	return children[0], children[1]
}

// renderTargetPage renders a document of one page, with no log.
func renderTargetPage(t *testing.T, source string) (body *Box) {
	t.Helper()
	log, capture := captureLogs(slog.LevelInfo)
	pages := renderPagesLog(t, source, testBaseURL, log)
	assertNoLogs(t, capture)
	if len(pages) != 1 {
		t.Fatalf("got %d pages", len(pages))
	}
	_, body = pageBody(t, pages[0])
	return body
}

// TestTargetFloat is test_target_float.
func TestTargetFloat(t *testing.T) {
	body := renderTargetPage(t, `
      <style>
        a::after {
          content: target-counter('#h', page);
          float: right;
        }
      </style>
      <div><a id="span">link</a></div>
      <h1 id="h">abc</h1>`)
	div := unpack(t, body, 2)[0]
	textBox, after := targetAfter(t, div)
	if textBox.Text != "link" {
		t.Errorf("got text %q", textBox.Text)
	}
	if got := only(t, only(t, after)).Text; got != "1" {
		t.Errorf("got counter %q", got)
	}
}

// TestTargetAbsolute is test_target_absolute.
func TestTargetAbsolute(t *testing.T) {
	body := renderTargetPage(t, `
      <style>
        a::after {
          content: target-counter('#h', page);
        }
        div {
          position: absolute;
        }
      </style>
      <div><a id="span">link</a></div>
      <h1 id="h">abc</h1>`)
	div := unpack(t, body, 2)[0]
	textBox, after := targetAfter(t, div)
	if textBox.Text != "link" {
		t.Errorf("got text %q", textBox.Text)
	}
	if got := only(t, after).Text; got != "1" {
		t.Errorf("got counter %q", got)
	}
}

// TestTargetAbsoluteNonRoot is test_target_absolute_non_root.
func TestTargetAbsoluteNonRoot(t *testing.T) {
	body := renderTargetPage(t, `
      <style>
        a::after {
          content: target-counter('#h', page);
        }
        section {
          position: relative;
        }
        div {
          position: absolute;
        }
      </style>
      <section><div><a id="span">link</a></div></section>
      <h1 id="h">abc</h1>`)
	section := unpack(t, body, 2)[0]
	div := only(t, section)
	textBox, after := targetAfter(t, div)
	if textBox.Text != "link" {
		t.Errorf("got text %q", textBox.Text)
	}
	if got := only(t, after).Text; got != "1" {
		t.Errorf("got counter %q", got)
	}
}

// TestTargetUndefined checks that LookupTarget logs an error for a target
// that no anchor defines, and that CollectAnchor warns for an anchor defined
// twice.
func TestTargetUndefined(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	collector := NewTargetCollector(log)
	collector.CollectAnchor("a")
	target := collector.LookupTarget(Tuple{String("string"), String("#b")}, &Box{Element: &Element{}}, "content", nil)
	if target.State != "undefined" {
		t.Errorf("got state %q, want undefined", target.State)
	}
	assertLog(t, capture, "target points to undefined anchor")

	log, capture = captureLogs(slog.LevelInfo)
	collector = NewTargetCollector(log)
	collector.CollectAnchor("a")
	collector.CollectAnchor("a")
	assertLog(t, capture, "Anchor defined twice")

	called := 0
	collector.LookupTarget(Tuple{String("string"), String("#a")}, &Box{Element: &Element{}}, "content", func(map[string][]int) { called++ })
	collector.CheckPendingTargets()
	if called != 1 || collector.HadPendingTargets {
		t.Errorf("got %d calls and pending %t, want 1 call and no pending target", called, collector.HadPendingTargets)
	}
}
