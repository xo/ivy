// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/test_boxes.py: the tests of the box tree before the
// layout. The tests that need the layout, render_pages of upstream, are
// skipped.

package layout

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"testing"
)

// boxTreeCases are the tests of upstream that only compare the box tree of
// a document, from parse or from parse_all.
var boxTreeCases = []struct {
	name     string
	all      bool
	source   string
	expected string
}{
	{"test_box_tree", false, `<p>`, `[('p', 'Block', [])]`},
	{"test_box_tree", false, `
      <style>
        span { display: inline-block }
      </style>
      <p>Hello <em>World <img src="pattern.png"><span>L</span></em>!</p>`, `[('p', 'Block', [('p', 'Text', 'Hello '), ('em', 'Inline', [('em', 'Text', 'World '), ('img', 'InlineReplaced', '<replaced>'), ('span', 'InlineBlock', [('span', 'Text', 'L')])]), ('p', 'Text', '!')])]`},
	{"test_whitespace", true, "\n      <p>Lorem \t\r\n  ipsum\t<strong>  dolor\n        <img src=pattern.png> sit\n        <span style=\"position: absolute\"></span> <em> amet </em>\n        consectetur</strong>.</p>\n      <pre>\t  foo\n</pre>\n      <pre style=\"white-space: pre-wrap\">\t  foo\n</pre>\n      <pre style=\"white-space: pre-line\">\t  foo\n</pre>\n    ", `[('p', 'Block', [('p', 'Line', [('p', 'Text', 'Lorem ipsum '), ('strong', 'Inline', [('strong', 'Text', 'dolor '), ('img', 'InlineReplaced', '<replaced>'), ('strong', 'Text', ' sit '), ('span', 'Block', []), ('em', 'Inline', [('em', 'Text', 'amet ')]), ('strong', 'Text', 'consectetur')]), ('p', 'Text', '.')])]), ('pre', 'Block', [('pre', 'Line', [('pre', 'Text', '\t  foo\n')])]), ('pre', 'Block', [('pre', 'Line', [('pre', 'Text', '\t  foo\n')])]), ('pre', 'Block', [('pre', 'Line', [('pre', 'Text', 'foo\n')])])]`},
	{"test_tables_1", true, `
      <style>
        x-table { display: table }
        x-tr { display: table-row }
        x-td, x-th { display: table-cell }
        x-thead { display: table-header-group }
        x-tfoot { display: table-footer-group }
        x-col { display: table-column }
        x-caption { display: table-caption }
      </style>
      <x-table>
        <x-tr>
          <x-th>foo</x-th>
          <x-th>bar</x-th>
        </x-tr>
        <x-tfoot></x-tfoot>
        <x-thead><x-th></x-th></x-thead>
        <x-caption style="caption-side: bottom"></x-caption>
        <x-thead></x-thead>
        <x-col></x-col>
        <x-caption>top caption</x-caption>
        <x-tr>
          <x-td>baz</x-td>
        </x-tr>
      </x-table>
    `, `[('x-table', 'Block', [('x-caption', 'TableCaption', [('x-caption', 'Line', [('x-caption', 'Text', 'top caption')])]), ('x-table', 'Table', [('x-table', 'TableColumnGroup', [('x-col', 'TableColumn', [])]), ('x-thead', 'TableRowGroup', [('x-thead', 'TableRow', [('x-th', 'TableCell', [])])]), ('x-table', 'TableRowGroup', [('x-tr', 'TableRow', [('x-th', 'TableCell', [('x-th', 'Line', [('x-th', 'Text', 'foo')])]), ('x-th', 'TableCell', [('x-th', 'Line', [('x-th', 'Text', 'bar')])])])]), ('x-thead', 'TableRowGroup', []), ('x-table', 'TableRowGroup', [('x-tr', 'TableRow', [('x-td', 'TableCell', [('x-td', 'Line', [('x-td', 'Text', 'baz')])])])]), ('x-tfoot', 'TableRowGroup', [])]), ('x-caption', 'TableCaption', [])])]`},
	{"test_tables_2", true, `
      <span style="display: table-cell">foo</span>
      <span style="display: table-cell">bar</span>
    `, `[('body', 'Block', [('body', 'Table', [('body', 'TableRowGroup', [('body', 'TableRow', [('span', 'TableCell', [('span', 'Line', [('span', 'Text', 'foo')])]), ('span', 'TableCell', [('span', 'Line', [('span', 'Text', 'bar')])])])])])])]`},
	{"test_tables_3", true, `
      <span style="display: table-column-group">
        1
        <em style="display: table-column">
          2
          <strong>3</strong>
        </em>
        <strong>4</strong>
      </span>
      <ins style="display: table-column-group"></ins>
    `, `[('body', 'Block', [('body', 'Table', [('span', 'TableColumnGroup', [('em', 'TableColumn', [])]), ('ins', 'TableColumnGroup', [('ins', 'TableColumn', [])])])])]`},
	{"test_tables_4", true, `<x-table style="display:table">foo <div></div></x-table>`, `[('x-table', 'Block', [('x-table', 'Table', [('x-table', 'TableRowGroup', [('x-table', 'TableRow', [('x-table', 'TableCell', [('x-table', 'Block', [('x-table', 'Line', [('x-table', 'Text', 'foo ')])]), ('div', 'Block', [])])])])])])]`},
	{"test_tables_5", true, `<x-thead style="display: table-header-group"><div></div><x-td style="display: table-cell"></x-td></x-thead>`, `[('body', 'Block', [('body', 'Table', [('x-thead', 'TableRowGroup', [('x-thead', 'TableRow', [('x-thead', 'TableCell', [('div', 'Block', [])]), ('x-td', 'TableCell', [])])])])])]`},
	{"test_tables_6", true, `<span><x-tr style="display: table-row"></x-tr></span>`, `[('body', 'Line', [('span', 'Inline', [('span', 'InlineBlock', [('span', 'InlineTable', [('span', 'TableRowGroup', [('x-tr', 'TableRow', [])])])])])])]`},
	{"test_tables_7", true, `
      <span>
        <em style="display: table-cell"></em>
        <em style="display: table-cell"></em>
      </span>
    `, `[('body', 'Line', [('span', 'Inline', [('span', 'Text', ' '), ('span', 'InlineBlock', [('span', 'InlineTable', [('span', 'TableRowGroup', [('span', 'TableRow', [('em', 'TableCell', []), ('em', 'TableCell', [])])])])]), ('span', 'Text', ' ')])])]`},
	{"test_tables_8", true, "<x-tr style=\"display: table-row\"></x-tr>\t<x-tr style=\"display: table-row\"></x-tr>", `[('body', 'Block', [('body', 'Table', [('body', 'TableRowGroup', [('x-tr', 'TableRow', []), ('x-tr', 'TableRow', [])])])])]`},
	{"test_tables_9", true, `<x-col style="display: table-column"></x-col>
<x-colgroup style="display: table-column-group"></x-colgroup>`, `[('body', 'Block', [('body', 'Table', [('body', 'TableColumnGroup', [('x-col', 'TableColumn', [])]), ('x-colgroup', 'TableColumnGroup', [('x-colgroup', 'TableColumn', [])])])])]`},
	{"test_before_after_1", true, `
      <style>
        p:before { content: normal }
        div:before { content: none }
        section::before { color: black }
      </style>
      <p></p>
      <div></div>
      <section></section>
    `, `[('p', 'Block', []), ('div', 'Block', []), ('section', 'Block', [])]`},
	{"test_before_after_2", true, `
      <style>
        p:before { content: 'a' 'b' }
        p::after { content: 'd' 'e' }
      </style>
      <p> c </p>
    `, `[('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', 'ab')]), ('p', 'Text', ' c '), ('p::after', 'Inline', [('p::after', 'Text', 'de')])])])]`},
	{"test_before_after_3", true, `
      <style>
        a[href]:before { content: '[' attr(href) '] ' }
      </style>
      <p><a href="some url">some text</a></p>
    `, `[('p', 'Block', [('p', 'Line', [('a', 'Inline', [('a::before', 'Inline', [('a::before', 'Text', '[some url] ')]), ('a', 'Text', 'some text')])])])]`},
	{"test_before_after_4", true, `
      <style>
        body { quotes: '«' '»' '“' '”' }
        q:before { content: open-quote ' '}
        q:after { content: ' ' close-quote }
      </style>
      <p><q>Lorem ipsum <q>dolor</q> sit amet</q></p>
    `, `[('p', 'Block', [('p', 'Line', [('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '«\xa0')]), ('q', 'Text', 'Lorem ipsum '), ('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '“\xa0')]), ('q', 'Text', 'dolor'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0”')])]), ('q', 'Text', ' sit amet'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0»')])])])])]`},
	{"test_quotes_auto", true, `
      <style>
        body { quotes: auto }
        q:before { content: open-quote ' '}
        q:after { content: ' ' close-quote }
      </style>
      <p><q>Lorem ipsum <q>dolor</q> sit amet</q></p>
    `, `[('p', 'Block', [('p', 'Line', [('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '“\xa0')]), ('q', 'Text', 'Lorem ipsum '), ('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '‘\xa0')]), ('q', 'Text', 'dolor'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0’')])]), ('q', 'Text', ' sit amet'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0”')])])])])]`},
	{"test_quotes_none", true, `
      <style>
        body { quotes: none }
        q:before { content: open-quote ' '}
        q:after { content: ' ' close-quote }
      </style>
      <p><q>Lorem ipsum <q>dolor</q> sit amet</q></p>
    `, `[('p', 'Block', [('p', 'Line', [('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '\xa0')]), ('q', 'Text', 'Lorem ipsum '), ('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '\xa0')]), ('q', 'Text', 'dolor'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0')])]), ('q', 'Text', ' sit amet'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0')])])])])]`},
	{"test_quotes_lang", true, `
      <style>
        q:before { content: open-quote ' '}
        q:after { content: ' ' close-quote }
      </style>
      <p lang="fr"><q>Lorem ipsum <q>dolor</q> sit amet</q></p>
    `, `[('p', 'Block', [('p', 'Line', [('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '«\xa0')]), ('q', 'Text', 'Lorem ipsum '), ('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '«\xa0')]), ('q', 'Text', 'dolor'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0»')])]), ('q', 'Text', ' sit amet'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0»')])])])])]`},
	{"test_quotes_lang_alternate", true, `
      <style>
        q:before { content: open-quote ' '}
        q:after { content: ' ' close-quote }
      </style>
      <p lang="fr_CH"><q>Lorem ipsum <q>dolor</q> sit amet</q></p>
    `, `[('p', 'Block', [('p', 'Line', [('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '«\xa0')]), ('q', 'Text', 'Lorem ipsum '), ('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '‹\xa0')]), ('q', 'Text', 'dolor'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0›')])]), ('q', 'Text', ' sit amet'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0»')])])])])]`},
	{"test_quotes_lang_parent", true, `
      <style>
        q:before { content: open-quote ' '}
        q:after { content: ' ' close-quote }
      </style>
      <p lang="fr_CH_alt"><q>Lorem ipsum <q>dolor</q> sit amet</q></p>
    `, `[('p', 'Block', [('p', 'Line', [('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '«\xa0')]), ('q', 'Text', 'Lorem ipsum '), ('q', 'Inline', [('q::before', 'Inline', [('q::before', 'Text', '‹\xa0')]), ('q', 'Text', 'dolor'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0›')])]), ('q', 'Text', ' sit amet'), ('q::after', 'Inline', [('q::after', 'Text', '\xa0»')])])])])]`},
}

// TestBoxTree runs boxTreeCases.
func TestBoxTree(t *testing.T) {
	for _, c := range boxTreeCases {
		t.Run(c.name, func(t *testing.T) {
			log, capture := captureLogs(slog.LevelInfo)
			p := parseBase(t, c.source, testBaseURL, log)
			var box *Box
			if c.all {
				var err error
				box, err = p.builder.buildFormattingStructure(p.html.EtreeElement)
				if err != nil {
					t.Fatal(err)
				}
				sanityChecks(t, box)
			} else {
				boxes := p.builder.elementToBoxRoot(p.html.EtreeElement)
				if len(boxes) != 1 {
					t.Fatalf("got %d boxes", len(boxes))
				}
				box = boxes[0]
			}
			assertTree(t, box, c.expected)
			assertNoLogs(t, capture)
		})
	}
}

// TestHTMLEntities is test_html_entities.
func TestHTMLEntities(t *testing.T) {
	for _, quote := range []string{`"`, "&quot;", "&#x22;", "&#34;"} {
		assertTree(t, parse(t, "<p>"+quote+"abc"+quote), `[('p', 'Block', [('p', 'Text', '"abc"')])]`)
	}
}

// TestInlineInBlock is test_inline_in_block_1 to test_inline_in_block_4.
func TestInlineInBlock(t *testing.T) {
	// test_inline_in_block_1
	box := inlineInBlock(parse(t, "<div>Hello, <em>World</em>!\n<p>Lipsum.</p></div>"))
	assertTree(t, box, `[
        ('div', 'Block', [
            ('div', 'Block', [
                ('div', 'Line', [
                    ('div', 'Text', 'Hello, '),
                    ('em', 'Inline', [
                        ('em', 'Text', 'World')]),
                    ('div', 'Text', '!\n')])]),
            ('p', 'Block', [
                ('p', 'Line', [
                    ('p', 'Text', 'Lipsum.')])])])]`)

	// test_inline_in_block_2
	box = inlineInBlock(parse(t, "<div><p>Lipsum.</p>Hello, <em>World</em>!\n</div>"))
	assertTree(t, box, `[
        ('div', 'Block', [
            ('p', 'Block', [
                ('p', 'Line', [
                    ('p', 'Text', 'Lipsum.')])]),
            ('div', 'Block', [
                ('div', 'Line', [
                    ('div', 'Text', 'Hello, '),
                    ('em', 'Inline', [
                        ('em', 'Text', 'World')]),
                    ('div', 'Text', '!\n')])])])]`)

	// test_inline_in_block_3
	// Absolutes are left in the lines to get their static position later.
	expected := `[
        ('p', 'Block', [
            ('p', 'Line', [
                ('p', 'Text', 'Hello '),
                ('em', 'Block', [
                    ('em', 'Line', [
                        ('em', 'Text', 'World')])]),
                ('p', 'Text', '!')])])]`
	box = inlineInBlock(parse(t, `<p>Hello <em style="position:absolute;
                                    display: block">World</em>!</p>`))
	assertTree(t, box, expected)
	box = blockInInline(box)
	assertTree(t, box, expected)

	// test_inline_in_block_4
	// Floats are pull to the top of their containing blocks
	box = blockInInline(inlineInBlock(parse(t, `<p>Hello <em style="float: left">World</em>!</p>`)))
	assertTree(t, box, `[
        ('p', 'Block', [
            ('p', 'Line', [
                ('p', 'Text', 'Hello '),
                ('em', 'Block', [
                    ('em', 'Line', [
                        ('em', 'Text', 'World')])]),
                ('p', 'Text', '!')])])]`)
}

// TestBlockInInline is test_block_in_inline.
func TestBlockInInline(t *testing.T) {
	box := parse(t, `
      <style>
        p { display: inline-block; }
        span, i { display: block; }
      </style>
      <p>Lorem <em>ipsum <strong>dolor <span>sit</span>
      <span>amet,</span></strong><span><em>conse<i>`)
	box = inlineInBlock(box)
	assertTree(t, box, `[
        ('body', 'Line', [
            ('p', 'InlineBlock', [
                ('p', 'Line', [
                    ('p', 'Text', 'Lorem '),
                    ('em', 'Inline', [
                        ('em', 'Text', 'ipsum '),
                        ('strong', 'Inline', [
                            ('strong', 'Text', 'dolor '),
                            ('span', 'Block', [  # This block is "pulled up"
                                ('span', 'Line', [
                                    ('span', 'Text', 'sit')])]),
                            ('strong', 'Text', '\n      '),
                            ('span', 'Block', [  # This block is "pulled up"
                                ('span', 'Line', [
                                    ('span', 'Text', 'amet,')])])]),
                        ('span', 'Block', [  # This block is "pulled up"
                            ('span', 'Line', [
                                ('em', 'Inline', [
                                    ('em', 'Text', 'conse'),
                                    ('i', 'Block', [])])])])])])])])]`)

	box = blockInInline(box)
	assertTree(t, box, `[
        ('body', 'Line', [
            ('p', 'InlineBlock', [
                ('p', 'Block', [
                    ('p', 'Line', [
                        ('p', 'Text', 'Lorem '),
                        ('em', 'Inline', [
                            ('em', 'Text', 'ipsum '),
                            ('strong', 'Inline', [
                                ('strong', 'Text', 'dolor ')])])])]),
                ('span', 'Block', [
                    ('span', 'Line', [
                        ('span', 'Text', 'sit')])]),
                ('p', 'Block', [
                    ('p', 'Line', [
                        ('em', 'Inline', [
                            ('strong', 'Inline', [
                                ('strong', 'Text', '\n      ')])])])]),
                ('span', 'Block', [
                    ('span', 'Line', [
                        ('span', 'Text', 'amet,')])]),

                ('p', 'Block', [
                    ('p', 'Line', [
                        ('em', 'Inline', [
                            ('strong', 'Inline', [])])])]),
                ('span', 'Block', [
                    ('span', 'Block', [
                        ('span', 'Line', [
                            ('em', 'Inline', [
                                ('em', 'Text', 'conse')])])]),
                    ('i', 'Block', []),
                    ('span', 'Block', [
                        ('span', 'Line', [
                            ('em', 'Inline', [])])])]),
                ('p', 'Block', [
                    ('p', 'Line', [
                        ('em', 'Inline', [])])])])])]`)
}

// TestStyles is test_styles.
func TestStyles(t *testing.T) {
	box := parse(t, `
      <style>
        span { display: block; }
        * { margin: 42px }
        html { color: blue }
      </style>
      <p>Lorem <em>ipsum <strong>dolor <span>sit</span>
        <span>amet,</span></strong><span>consectetur</span></em></p>`)
	box = blockInInline(inlineInBlock(box))

	descendants := box.descendantsList()
	if len(descendants) != 31 {
		t.Errorf("%d descendants", len(descendants))
	}
	if descendants[0] != box {
		t.Error("the first descendant is not the box")
	}
	blue := mustParseColor("blue")
	for _, child := range descendants {
		// All boxes inherit the color
		if !Equal(child.Style.Get("color"), blue) {
			t.Errorf("color of %v: %s", child, ValueString(child.Style.Get("color")))
		}
		// Only non-anonymous boxes have margins
		if margin := child.Style.Get("margin_top"); !Equal(margin, Dimension{0, "px"}) && !Equal(margin, Dimension{42, "px"}) {
			t.Errorf("margin of %v: %s", child, ValueString(margin))
		}
	}
}

// TestImages is test_images_1 and test_images_2.
func TestImages(t *testing.T) {
	// test_images_1
	log, capture := captureLogs(slog.LevelInfo)
	result := parseAllLog(t, `
          <p><img src=pattern.png
            /><img alt="No src"
            /><img src=inexistent.jpg alt="Inexistent src" /></p>
        `, testBaseURL, log)
	logs := capture.logs()
	if len(logs) != 1 || !strings.Contains(logs[0], "ERROR: Failed to load image") || !strings.Contains(logs[0], "inexistent.jpg") {
		t.Errorf("logs: %q", logs)
	}
	assertTree(t, result, `[
        ('p', 'Block', [
            ('p', 'Line', [
                ('img', 'InlineReplaced', '<replaced>'),
                ('img', 'Inline', [
                    ('img', 'Text', 'No src')]),
                ('img', 'Inline', [
                    ('img', 'Text', 'Inexistent src')])])])]`)

	// test_images_2
	log, capture = captureLogs(slog.LevelInfo)
	result = parseAllLog(t, `<p><img src=pattern.png alt="No base_url">`, "", log)
	logs = capture.logs()
	if len(logs) != 1 || !strings.Contains(logs[0], "ERROR: Relative URI reference without a base URI") {
		t.Errorf("logs: %q", logs)
	}
	assertTree(t, result, `[
        ('p', 'Block', [
            ('p', 'Line', [
                ('img', 'Inline', [
                    ('img', 'Text', 'No base_url')])])])]`)
}

// tableOf returns the wrapper and the table of the first child of the body
// of a document.
func tableOf(t *testing.T, html *Box) (wrapper, table *Box) {
	t.Helper()
	body := html.Children[0]
	if len(body.Children) != 1 {
		t.Fatalf("body has %d children", len(body.Children))
	}
	wrapper = body.Children[0]
	if len(wrapper.Children) != 1 {
		t.Fatalf("wrapper has %d children", len(wrapper.Children))
	}
	return wrapper, wrapper.Children[0]
}

// TestTableStyle is test_table_style.
func TestTableStyle(t *testing.T) {
	html := parseAll(t, `<table style="margin: 1px; padding: 2px"></table>`)
	wrapper, table := tableOf(t, html)
	if wrapper.Kind != KindBlock || table.Kind != KindTable {
		t.Fatalf("kinds: %v %v", wrapper, table)
	}
	for _, c := range []struct {
		style *Style
		name  string
		want  float64
	}{
		{wrapper.Style, "margin_top", 1},
		{wrapper.Style, "padding_top", 0},
		{table.Style, "margin_top", 0},
		{table.Style, "padding_top", 2},
	} {
		if got := c.style.Get(c.name); !Equal(got, Dimension{c.want, "px"}) {
			t.Errorf("%s: %s", c.name, ValueString(got))
		}
	}
}

// TestColumnStyle is test_column_style.
func TestColumnStyle(t *testing.T) {
	html := parseAll(t, `
      <table>
        <col span=3 style="width: 10px"></col>
        <col span=2></col>
      </table>
    `)
	_, table := tableOf(t, html)
	if len(table.table().ColumnGroups) != 1 {
		t.Fatalf("%d column groups", len(table.table().ColumnGroups))
	}
	colgroup := table.table().ColumnGroups[0]
	widths := make(Tuple, 0, len(colgroup.Children))
	gridX := make([]string, 0, len(colgroup.Children))
	for _, col := range colgroup.Children {
		widths = append(widths, col.Style.Get("width"))
		gridX = append(gridX, strconv.Itoa(col.table().GridX))
	}
	ten := Dimension{10, "px"}
	if want := (Tuple{ten, ten, ten, String("auto"), String("auto")}); !Equal(widths, want) {
		t.Errorf("widths: %s", ValueString(widths))
	}
	if got := strings.Join(gridX, " "); got != "0 1 2 3 4" {
		t.Errorf("grid_x: %s", got)
	}
	// copies, not the same box object
	if colgroup.Children[0] == colgroup.Children[1] {
		t.Error("the columns are the same box")
	}
}

// TestNestedGridX is test_nested_grid_x.
func TestNestedGridX(t *testing.T) {
	html := parseAll(t, `
      <table>
        <col span=2></col>
        <colgroup span=2></colgroup>
        <colgroup>
          <col></col>
          <col span=2></col>
        </colgroup>
        <col></col>
      </table>
    `)
	_, table := tableOf(t, html)
	grid := make([]string, 0, len(table.table().ColumnGroups))
	for _, colgroup := range table.table().ColumnGroups {
		cols := make([]string, 0, len(colgroup.Children))
		for _, col := range colgroup.Children {
			cols = append(cols, strconv.Itoa(col.table().GridX))
		}
		grid = append(grid, fmt.Sprintf("(%d, [%s])", colgroup.table().GridX, strings.Join(cols, ", ")))
	}
	if got := strings.Join(grid, ", "); got != "(0, [0, 1]), (2, [2, 3]), (4, [4, 5, 6]), (7, [7])" {
		t.Errorf("grid: %s", got)
	}
}

// cellGrid returns the values of a field of the cells of each row of a row
// group, as a list of lists of Python.
func cellGrid(group *Box, field func(*Box) int) string {
	rows := make([]string, 0, len(group.Children))
	for _, row := range group.Children {
		cells := make([]string, 0, len(row.Children))
		for _, cell := range row.Children {
			cells = append(cells, strconv.Itoa(field(cell)))
		}
		rows = append(rows, "["+strings.Join(cells, ", ")+"]")
	}
	return "[" + strings.Join(rows, ", ") + "]"
}

// TestColspanRowspan is test_colspan_rowspan_1 and test_colspan_rowspan_2.
func TestColspanRowspan(t *testing.T) {
	// +---+---+---+
	// | A | B | C | X
	// +---+---+---+
	// | D |     E | X
	// +---+---+   +---+
	// |  F ...|   |   |   <-- overlap
	// +---+---+---+   +
	// | H | X   X | G |
	// +---+---+   +   +
	// | I | J | X |   |
	// +---+---+   +---+

	// X: empty cells
	html := parseAll(t, `
      <table>
        <tr>
          <td>A <td>B <td>C
        </tr>
        <tr>
          <td>D <td colspan=2 rowspan=2>E
        </tr>
        <tr>
          <td colspan=2>F <td rowspan=0>G
        </tr>
        <tr>
          <td>H
        </tr>
        <tr>
          <td>I <td>J
        </tr>
      </table>
    `)
	_, table := tableOf(t, html)
	group := table.Children[0]
	gridX := func(c *Box) int { return c.table().GridX }
	colspan := func(c *Box) int { return c.table().Colspan }
	rowspan := func(c *Box) int { return c.table().Rowspan }
	for _, c := range []struct {
		field func(*Box) int
		want  string
	}{
		{gridX, "[[0, 1, 2], [0, 1], [0, 3], [0], [0, 1]]"},
		{colspan, "[[1, 1, 1], [1, 2], [2, 1], [1], [1, 1]]"},
		{rowspan, "[[1, 1, 1], [1, 2], [1, 3], [1], [1, 1]]"},
	} {
		if got := cellGrid(group, c.field); got != c.want {
			t.Errorf("got %s, want %s", got, c.want)
		}
	}

	// A cell box cannot extend beyond the last row box of a table.
	html = parseAll(t, `
        <table>
            <tr>
                <td rowspan=5></td>
                <td></td>
            </tr>
            <tr>
                <td></td>
            </tr>
        </table>
    `)
	_, table = tableOf(t, html)
	group = table.Children[0]
	for _, c := range []struct {
		field func(*Box) int
		want  string
	}{
		{gridX, "[[0, 1], [1]]"},
		{colspan, "[[1, 1], [1]]"},
		{rowspan, "[[2, 1], [1]]"}, // Not 5
	} {
		if got := cellGrid(group, c.field); got != c.want {
			t.Errorf("got %s, want %s", got, c.want)
		}
	}
}

// TestBeforeAfter5 is test_before_after_5.
func TestBeforeAfter5(t *testing.T) {
	// Regression test.
	log, capture := captureLogs(slog.LevelInfo)
	result := parseAllLog(t, `
          <style>
            p:before {
              content: 'a' url(pattern.png) 'b';
              /* Invalid, ignored in favor of the one above. */
              content: some-function(nested-function(something));
            }
          </style>
          <p>c</p>
        `, testBaseURL, log)
	assertTree(t, result, `[
            ('p', 'Block', [
                ('p', 'Line', [
                    ('p::before', 'Inline', [
                        ('p::before', 'Text', 'a'),
                        ('p::before', 'InlineReplaced', '<replaced>'),
                        ('p::before', 'Text', 'b')]),
                    ('p', 'Text', 'c')])])]`)
	logs := capture.logs()
	if len(logs) != 1 || !strings.Contains(logs[0], "nested-function(") || !strings.Contains(logs[0], "invalid value") {
		t.Errorf("logs: %q", logs)
	}
}

// TestDisplayNoneRoot is test_display_none_root.
func TestDisplayNoneRoot(t *testing.T) {
	for _, source := range []string{
		`<html style="display: none">`,
		`<html style="display: none">abc`,
		`<html style="display: none"><p>abc`,
		`<body style="display: none"><p>abc`,
	} {
		box := parseAll(t, source)
		if !Equal(box.Style.Get("display"), tupleOf("block", "flow")) {
			t.Errorf("%s: display %s", source, ValueString(box.Style.Get("display")))
		}
		if len(box.Children) != 0 {
			t.Errorf("%s: %d children", source, len(box.Children))
		}
	}
}
