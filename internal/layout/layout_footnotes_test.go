// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_footnotes.py and
// tests/layout/test_notes.py: the tests of the layout of the footnotes and
// of the notes.

package layout

import (
	"slices"
	"testing"
)

// footnoteStyle is the style of most of the tests of the footnotes, with
// the size of the page and more style.
func footnoteStyle(size, more string) string {
	return `
        <style>
            @page {
                size: ` + size + `;
            }
            div {
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            span {
                float: footnote;
            }
            ` + more + `
        </style>`
}

// footnoteTreePosition is tree_position in tests/testing_utils.py: the
// position in the tree of the first box, depth first, that matcher
// accepts, or nil.
func footnoteTreePosition(boxes []*Box, matcher func(*Box) bool) []int {
	for i, box := range boxes {
		if matcher(box) {
			return []int{i}
		}
		if position := footnoteTreePosition(box.Children, matcher); position != nil {
			return append([]int{i}, position...)
		}
	}
	return nil
}

// footnoteTextIs returns a matcher of the text boxes of a text.
func footnoteTextIs(text string) func(*Box) bool {
	return func(box *Box) bool { return box.Kind == KindText && box.Text == text }
}

// footnoteSorted fails the test when the positions are not in increasing
// order, as sorted(positions) == positions does upstream.
func footnoteSorted(t *testing.T, positions [][]int) {
	t.Helper()
	if !slices.IsSortedFunc(positions, slices.Compare[[]int]) {
		t.Errorf("positions %v are not sorted", positions)
	}
}

// footnoteMarkerText checks the marker and the text of the first line of
// the footnote of index i of the footnote area.
func footnoteMarkerText(t *testing.T, area *Box, i int, marker, text string) {
	t.Helper()
	children := unpack(t, columnChild(t, area, i, 0), 2)
	columnText(t, children[0], marker, 0)
	if children[1].Text != text {
		t.Errorf("footnote text %q, want %q", children[1].Text, text)
	}
}

// footnoteCall checks the text and the call of a line of a footnote call.
func footnoteCall(t *testing.T, line *Box, text, call string) {
	t.Helper()
	children := unpack(t, line, 2)
	if children[0].Text != text {
		t.Errorf("text %q, want %q", children[0].Text, text)
	}
	columnText(t, children[1], call, 0)
}

// TestInlineFootnote is test_inline_footnote.
func TestInlineFootnote(t *testing.T) {
	page := pageRender(t, footnoteStyle("9px 7px", "")+`
        <div>abc<span>de</span></div>`, 1)[0]
	children := unpack(t, page, 2)
	html, footnoteArea := children[0], children[1]
	div := only(t, only(t, html))
	line := columnChild(t, div, 0)
	footnoteCall(t, line, "abc", "1")
	pageEqual(t, "text y", line.Children[0].PositionY, 0)

	footnoteMarkerText(t, footnoteArea, 0, "1.", "de")
	pageEqual(t, "footnote area y", footnoteArea.PositionY, 5)
}

// TestFootnoteAreaRounding checks that a footnote stays on the page of its
// call when the sums of the margin, the border, the padding and the
// height of the footnote area differ from the bottom of the footnote in
// their last bit. It is not a test of upstream, which moves the footnote
// to the next page then. The lengths come from the line above the
// footnotes of a file of LibreOffice.
func TestFootnoteAreaRounding(t *testing.T) {
	pages := pageRender(t, footnoteStyle("612pt 792pt", `
            @page { margin: 72pt; @footnote { margin-top: 14.15pt; padding-top: 8.5pt; border-top: 2.87pt solid } }
            span { line-height: 10pt }`)+`
        <div>abc<span>de</span></div>`, 1)
	children := unpack(t, pages[0], 2)
	footnoteMarkerText(t, children[1], 0, "1.", "de")
}

// TestBlockFootnote is test_block_footnote.
func TestBlockFootnote(t *testing.T) {
	page := pageRender(t, `
        <style>
          @page {
              size: 9px 7px;
          }
          div {
              font-family: weasyprint;
              font-size: 2px;
              line-height: 1;
          }
          div.footnote {
              float: footnote;
          }
        </style>
        <div>abc<div class="footnote">de</div></div>`, 1)[0]
	children := unpack(t, page, 2)
	html, footnoteArea := children[0], children[1]
	div := only(t, only(t, html))
	line := columnChild(t, div, 0)
	footnoteCall(t, line, "abc", "1")
	pageEqual(t, "text y", line.Children[0].PositionY, 0)
	footnoteMarkerText(t, footnoteArea, 0, "1.", "de")
	pageEqual(t, "footnote area y", footnoteArea.PositionY, 5)
}

// TestLongFootnote is test_long_footnote.
func TestLongFootnote(t *testing.T) {
	page := pageRender(t, footnoteStyle("9px 7px", "")+`
        <div>abc<span>de f</span></div>`, 1)[0]
	children := unpack(t, page, 2)
	html, footnoteArea := children[0], children[1]
	div := only(t, only(t, html))
	line := columnChild(t, div, 0)
	footnoteCall(t, line, "abc", "1")
	pageEqual(t, "text y", line.Children[0].PositionY, 0)
	lines := unpack(t, columnChild(t, footnoteArea, 0), 2)
	marker := unpack(t, lines[0], 2)
	columnText(t, marker[0], "1.", 0)
	if marker[1].Text != "de" {
		t.Errorf("text %q", marker[1].Text)
	}
	pageEqual(t, "footnote area y", footnoteArea.PositionY, 3)
	content2 := columnChild(t, lines[1], 0)
	if content2.Text != "f" {
		t.Errorf("text %q", content2.Text)
	}
	pageEqual(t, "content 2 y", content2.PositionY, 5)
}

// TestAfterMarkerFootnote is test_after_marker_footnote.
func TestAfterMarkerFootnote(t *testing.T) {
	t.Skip("upstream marks the test as an expected failure")
}

// TestSeveralFootnote is test_several_footnote.
func TestSeveralFootnote(t *testing.T) {
	pages := pageRender(t, footnoteStyle("9px 7px", "")+`
        <div>abcd e<span>fg</span> hijk l<span>mn</span></div>`, 2)
	for i, c := range []struct{ first, second, call, marker, text string }{
		{"abcd", "e", "1", "1.", "fg"},
		{"hijk", "l", "2", "2.", "mn"},
	} {
		children := unpack(t, pages[i], 2)
		html, footnoteArea := children[0], children[1]
		div := only(t, only(t, html))
		lines := unpack(t, div, 2)
		columnText(t, lines[0], c.first, 0)
		footnoteCall(t, lines[1], c.second, c.call)
		footnoteMarkerText(t, footnoteArea, 0, c.marker, c.text)
	}
}

// TestReportedFootnote1 is test_reported_footnote_1.
func TestReportedFootnote1(t *testing.T) {
	pages := pageRender(t, footnoteStyle("9px 7px", "")+`
        <div>abc<span>f1</span> hij<span>f2</span></div>`, 2)
	children := unpack(t, pages[0], 2)
	html1, footnoteArea1 := children[0], children[1]
	div1 := only(t, only(t, html1))
	lines := unpack(t, div1, 2)
	footnoteCall(t, lines[0], "abc", "1")
	footnoteCall(t, lines[1], "hij", "2")
	footnoteMarkerText(t, footnoteArea1, 0, "1.", "f1")

	children = unpack(t, pages[1], 2)
	unpack(t, children[0], 0)
	footnoteMarkerText(t, children[1], 0, "2.", "f2")
}

// TestReportedFootnote2 is test_reported_footnote_2.
func TestReportedFootnote2(t *testing.T) {
	pages := pageRender(t, footnoteStyle("9px 7px", "")+`
        <div>abc<span>f1</span> hij<span>f2</span> wow</div>`, 2)
	children := unpack(t, pages[0], 2)
	div1 := only(t, only(t, children[0]))
	lines := unpack(t, div1, 2)
	footnoteCall(t, lines[0], "abc", "1")
	footnoteCall(t, lines[1], "hij", "2")
	footnoteMarkerText(t, children[1], 0, "1.", "f1")

	children = unpack(t, pages[1], 2)
	div2 := only(t, only(t, children[0]))
	columnText(t, only(t, div2), "wow", 0)
	footnoteMarkerText(t, children[1], 0, "2.", "f2")
}

// TestReportedFootnote3 is test_reported_footnote_3.
func TestReportedFootnote3(t *testing.T) {
	pages := pageRender(t, footnoteStyle("9px 10px", "")+`
        <div>
          abc<span>1</span>
          def<span>v long 2</span>
          ghi<span>3</span>
        </div>`, 2)
	children := unpack(t, pages[0], 2)
	div1 := only(t, only(t, children[0]))
	lines := unpack(t, div1, 3)
	for i, c := range []struct{ text, call string }{{"abc", "1"}, {"def", "2"}, {"ghi", "3"}} {
		columnText(t, lines[i], c.text, 0)
		columnText(t, lines[i], c.call, 1, 0)
	}
	footnote1 := only(t, children[1])
	columnText(t, footnote1, "1.", 0, 0, 0)
	columnText(t, footnote1, "1", 0, 1)

	children = unpack(t, pages[1], 2)
	footnotes := unpack(t, children[1], 2)
	columnText(t, footnotes[0], "2.", 0, 0, 0)
	columnText(t, footnotes[0], "v", 0, 1)
	columnText(t, footnotes[0], "long", 1, 0)
	columnText(t, footnotes[0], "2", 2, 0)
	columnText(t, footnotes[1], "3.", 0, 0, 0)
	columnText(t, footnotes[1], "3", 0, 1)
}

// TestReportedSequentialFootnote is test_reported_sequential_footnote.
func TestReportedSequentialFootnote(t *testing.T) {
	pages := pageRender(t, footnoteStyle("9px 7px", "")+`
        <div>
            a<span>b</span><span>c</span><span>d</span><span>e</span>
        </div>`, -1)
	positions := make([][]int, 0, 5)
	for _, letter := range []string{"a", "b", "c", "d", "e"} {
		positions = append(positions, footnoteTreePosition(pages, footnoteTextIs(letter)))
	}
	footnoteSorted(t, positions)
}

// TestReportedSequentialFootnoteSecondLine is
// test_reported_sequential_footnote_second_line.
func TestReportedSequentialFootnoteSecondLine(t *testing.T) {
	pages := pageRender(t, footnoteStyle("9px 7px", "")+`
        <div>
            aaa a<span>b</span><span>c</span><span>d</span><span>e</span>
        </div>`, -1)
	positions := make([][]int, 0, 3)
	for _, letter := range []string{"a", "b", "c"} {
		positions = append(positions, footnoteTreePosition(pages, footnoteTextIs(letter)))
	}
	footnoteSorted(t, positions)
}

// footnoteOrphans renders the documents of the tests of the orphans with
// footnotes, and checks the number of lines of the div of each page and
// the number of footnotes of the footnote area.
func footnoteOrphans(t *testing.T, source string, lines [][]int, footnotes []int) {
	t.Helper()
	pages := pageRender(t, source, len(footnotes))
	for i, page := range pages {
		children := unpack(t, page, 2)
		body := only(t, children[0])
		divs := unpack(t, body, len(lines[i]))
		for j, div := range divs {
			if len(div.Children) != lines[i][j] {
				t.Errorf("page %d, div %d: %d lines, want %d", i+1, j+1, len(div.Children), lines[i][j])
			}
		}
		if len(children[1].Children) != footnotes[i] {
			t.Errorf("page %d: %d footnotes, want %d", i+1, len(children[1].Children), footnotes[i])
		}
	}
}

// TestFootnoteReportOrphans is test_footnote_report_orphans.
func TestFootnoteReportOrphans(t *testing.T) {
	footnoteOrphans(t, `
      <style>
        @page {
          size: 20px;
        }
        body {
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
          orphans: 2;
          widows: 2;
        }
        span {
          float: footnote;
        }
      </style>
      <div>
        a<br>
        b<br>
        c<br>
        d<br>
        e
      </div>
      <div>
        f<span>1</span><span>2</span><span>3</span><span>4</span><br>
        g<br>
        h<br>
        i
      </div>`, [][]int{{5, 2}, {2}}, []int{3, 1})
}

// footnoteOrphansStyle is the style of the tests of the orphans of the
// current and of the previous lines.
const footnoteOrphansStyle = `
      <style>
        @page { size: 20px }
        body { font: 2px/1 weasyprint; orphans: 2; widows: 2 }
        span { float: footnote }
      </style>`

// TestFootnoteCurrentLineOrphans is test_footnote_current_line_orphans.
func TestFootnoteCurrentLineOrphans(t *testing.T) {
	footnoteOrphans(t, footnoteOrphansStyle+`
      <div>
        a<br>
        b<span>1</span><br>
        c<br>
        d<br>
        e<br>
        f<br>
        g<br>
        h
      </div>
      <div>
        i<br>
        j<span>2</span><br>
        k<br>
        l<br>
        m
      </div>`, [][]int{{8}, {5}}, []int{1, 1})
}

// TestFootnotePreviousLineOrphans is test_footnote_previous_line_orphans.
func TestFootnotePreviousLineOrphans(t *testing.T) {
	footnoteOrphans(t, footnoteOrphansStyle+`
      <div>
        a<br>
        b<span>1</span><br>
        c<br>
        d<br>
        e<br>
        f<br>
        g
      </div>
      <div>
        i<span>2</span><br>
        j<br>
        k<br>
        l<br>
        m
      </div>`, [][]int{{7}, {5}}, []int{1, 1})
}

// TestFootnoteReportPreviousAndCurrentLineOrphans is
// test_footnote_report_previous_and_current_line_orphans.
func TestFootnoteReportPreviousAndCurrentLineOrphans(t *testing.T) {
	footnoteOrphans(t, footnoteOrphansStyle+`
      <div>
        a<br>
        b<span>1</span><br>
        c<br>
        d<br>
        e<br>
        f<br>
        g
      </div>
      <div>
        i<span>2</span><br>
        j<span>3</span><br>
        k<br>
        l<span>4</span><br>
        m
      </div>`, [][]int{{7}, {5}}, []int{1, 3})
}

// TestFootnoteAreaAfterCall is test_footnote_area_after_call.
func TestFootnoteAreaAfterCall(t *testing.T) {
	for _, c := range []struct{ css, tail string }{
		{"p { break-inside: avoid }", "<br>e<br>f"},
		{"p { widows: 4 }", "<br>e<br>f"},
		{"p + p { break-before: avoid }", "</p><p>e<br>f"},
		{"p + p { break-before: avoid }", "<span>y</span><span>z</span></p><p>e"},
	} {
		t.Run(c.css+" "+c.tail, func(t *testing.T) {
			pages := pageRender(t, `
        <style>
            @page {
                size: 9px 10px;
                margin: 0;
            }
            body {
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
                orphans: 2;
                widows: 2;
                margin: 0;
            }
            span {
                float: footnote;
            }
            `+c.css+`
        </style>
        <div>a<br>b</div>
        <p>c<br>d<span>x</span>`+c.tail+`</p>`, -1)
			call := footnoteTreePosition(pages, func(box *Box) bool { return box.ElementTag == "span::footnote-call" })
			area := footnoteTreePosition(pages, func(box *Box) bool { return box.Kind == KindFootnoteArea })
			if call == nil || area == nil || slices.Compare(call, area) >= 0 {
				t.Errorf("footnote call at %v, footnote area at %v", call, area)
			}
		})
	}
}

// TestFootnoteDisplayInline is test_footnote_display_inline.
func TestFootnoteDisplayInline(t *testing.T) {
	page := pageRender(t, footnoteStyle("9px 50px", "span { footnote-display: inline; }")+`
        <div>abc<span>d</span> fgh<span>i</span></div>`, 1)[0]
	children := unpack(t, page, 2)
	div := only(t, only(t, children[0]))
	lines := unpack(t, div, 2)
	footnoteCall(t, lines[0], "abc", "1")
	footnoteCall(t, lines[1], "fgh", "2")
	line := columnChild(t, children[1], 0)
	for i, c := range []struct{ marker, text string }{{"1.", "d"}, {"2.", "i"}} {
		parts := unpack(t, columnChild(t, line, i), 2)
		columnText(t, parts[0], c.marker, 0)
		if parts[1].Text != c.text {
			t.Errorf("text %q, want %q", parts[1].Text, c.text)
		}
	}
}

// footnoteLines checks the lines of the first footnote of a footnote area:
// the marker and the texts of the lines.
func footnoteLines(t *testing.T, area *Box, marker string, texts ...string) {
	t.Helper()
	lines := unpack(t, columnChild(t, area, 0), len(texts))
	first := unpack(t, lines[0], 2)
	columnText(t, first[0], marker, 0)
	if first[1].Text != texts[0] {
		t.Errorf("text %q, want %q", first[1].Text, texts[0])
	}
	for i, line := range lines[1:] {
		columnText(t, line, texts[i+1], 0)
	}
}

// TestFootnoteLongerThanSpaceLeft is test_footnote_longer_than_space_left.
func TestFootnoteLongerThanSpaceLeft(t *testing.T) {
	pages := pageRender(t, footnoteStyle("9px 7px", "")+`
        <div>abc<span>def ghi jkl</span></div>`, 2)
	div := only(t, only(t, only(t, pages[0])))
	footnoteCall(t, columnChild(t, div, 0), "abc", "1")

	children := unpack(t, pages[1], 2)
	unpack(t, children[0], 0)
	footnoteLines(t, children[1], "1.", "def", "ghi", "jkl")
}

// TestFootnoteLongerThanPage is test_footnote_longer_than_page.
func TestFootnoteLongerThanPage(t *testing.T) {
	// Nothing is defined for this use case in the specification. In
	// WeasyPrint, the content simply overflows.
	pages := pageRender(t, footnoteStyle("9px 7px", "")+`
        <div>abc<span>def ghi jkl mno</span></div>`, 2)
	div := only(t, only(t, only(t, pages[0])))
	footnoteCall(t, columnChild(t, div, 0), "abc", "1")

	children := unpack(t, pages[1], 2)
	unpack(t, children[0], 0)
	footnoteLines(t, children[1], "1.", "def", "ghi", "jkl", "mno")
}

// TestFootnotePolicyLine is test_footnote_policy_line.
func TestFootnotePolicyLine(t *testing.T) {
	pages := pageRender(t, `
        <style>
            @page {
                size: 9px 9px;
            }
            div {
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
                orphans: 2;
                widows: 2;
            }
            span {
                float: footnote;
                footnote-policy: line;
            }
        </style>
        <div>abc def ghi jkl<span>1</span></div>`, 2)
	div := only(t, only(t, only(t, pages[0])))
	lines := unpack(t, div, 2)
	columnText(t, lines[0], "abc", 0)
	columnText(t, lines[1], "def", 0)

	children := unpack(t, pages[1], 2)
	div = only(t, only(t, children[0]))
	lines = unpack(t, div, 2)
	columnText(t, lines[0], "ghi", 0)
	footnoteCall(t, lines[1], "jkl", "1")
	footnoteMarkerText(t, children[1], 0, "1.", "1")
}

// TestFootnotePolicyBlock is test_footnote_policy_block.
func TestFootnotePolicyBlock(t *testing.T) {
	pages := pageRender(t, `
        <style>
            @page {
                size: 9px 9px;
            }
            div {
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            span {
                float: footnote;
                footnote-policy: block;
            }
        </style>
        <div>abc</div><div>def ghi jkl<span>1</span></div>`, 2)
	div := only(t, only(t, only(t, pages[0])))
	columnText(t, only(t, div), "abc", 0)

	children := unpack(t, pages[1], 2)
	div = only(t, only(t, children[0]))
	lines := unpack(t, div, 3)
	columnText(t, lines[0], "def", 0)
	columnText(t, lines[1], "ghi", 0)
	footnoteCall(t, lines[2], "jkl", "1")
	footnoteMarkerText(t, children[1], 0, "1.", "1")
}

// TestFootnoteRepagination is test_footnote_repagination.
func TestFootnoteRepagination(t *testing.T) {
	page := pageRender(t, footnoteStyle("9px 7px", "div::after { content: counter(pages); }")+`
        <div>ab<span>de</span></div>`, 1)[0]
	children := unpack(t, page, 2)
	div := only(t, only(t, children[0]))
	parts := unpack(t, columnChild(t, div, 0), 3)
	if parts[0].Text != "ab" {
		t.Errorf("text %q", parts[0].Text)
	}
	columnText(t, parts[1], "1", 0)
	pageEqual(t, "text y", parts[0].PositionY, 0)
	columnText(t, parts[2], "1", 0)

	footnoteMarkerText(t, children[1], 0, "1.", "de")
	pageEqual(t, "footnote area y", children[1].PositionY, 5)
}

// TestReportedFootnoteRepagination is test_reported_footnote_repagination.
func TestReportedFootnoteRepagination(t *testing.T) {
	// Regression test for #1700.
	pages := pageRender(t, footnoteStyle("5px", "a::after { content: target-counter(attr(href), page); }")+`
        <div><a href="#i">a</a> bb<span>de</span> <i id="i">fg</i></div>`, 2)
	div := only(t, only(t, only(t, pages[0])))
	lines := unpack(t, div, 2)
	a := only(t, lines[0])
	columnText(t, a, "a", 0)
	columnText(t, a, "2", 1, 0)
	parts := unpack(t, lines[1], 3)
	if parts[0].Text != "bb" {
		t.Errorf("text %q", parts[0].Text)
	}
	columnText(t, parts[1], "1", 0)

	children := unpack(t, pages[1], 2)
	div = only(t, only(t, children[0]))
	i := only(t, only(t, div))
	columnText(t, i, "fg", 0)
	footnoteMarkerText(t, children[1], 0, "1.", "de")
	pageEqual(t, "footnote area y", children[1].PositionY, 3)
}

// TestFootnoteMaxHeight is test_footnote_max_height.
func TestFootnoteMaxHeight(t *testing.T) {
	pages := pageRender(t, `
      <style>
        @page {
            size: 12px 6px;

            @footnote {
                margin-left: 1px;
                max-height: 4px;
            }
        }
        div {
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        div.footnote {
            float: footnote;
        }
      </style>
      <div>ab<div class="footnote">c</div><div class="footnote">d</div>
      <div class="footnote">e</div></div>
      <div>fg</div>`, 2)
	children := unpack(t, pages[0], 2)
	div := only(t, only(t, children[0]))
	parts := unpack(t, columnChild(t, div, 0), 5)
	if parts[0].Text != "ab" || parts[3].Text != " " {
		t.Errorf("texts %q %q", parts[0].Text, parts[3].Text)
	}
	columnText(t, parts[1], "1", 0)
	columnText(t, parts[2], "2", 0)
	columnText(t, parts[4], "3", 0)
	footnotes := unpack(t, children[1], 2)
	for i, c := range []struct{ marker, text string }{{"1.", "c"}, {"2.", "d"}} {
		line := only(t, footnotes[i])
		marker := unpack(t, line, 2)
		columnText(t, marker[0], c.marker, 0)
		if marker[1].Text != c.text {
			t.Errorf("text %q, want %q", marker[1].Text, c.text)
		}
	}

	children = unpack(t, pages[1], 2)
	div2 := only(t, only(t, children[0]))
	if got := only(t, columnChild(t, div2, 0)).Text; got != "fg" {
		t.Errorf("text %q", got)
	}
	line := only(t, columnChild(t, children[1], 0))
	marker := unpack(t, line, 2)
	columnText(t, marker[0], "3.", 0)
	if marker[1].Text != "e" {
		t.Errorf("text %q", marker[1].Text)
	}
}

// footnoteTableBody returns the only table body of the page.
func footnoteTableBody(t *testing.T, html *Box) *Box {
	t.Helper()
	body := only(t, html)
	wrapper := only(t, body)
	table := only(t, wrapper)
	return only(t, table)
}

// TestFootnoteTableAbortedRow is test_footnote_table_aborted_row.
func TestFootnoteTableAbortedRow(t *testing.T) {
	pages := renderPages(t, `
      <style>
        @page {size: 10px 35px}
        body {font-family: weasyprint; font-size: 2px}
        tr {height: 10px}
        .footnote {float: footnote}
      </style>
      <table><tbody>
        <tr><td>abc</td></tr>
        <tr><td>abc</td></tr>
        <tr><td>abc</td></tr>
        <tr><td>def<div class="footnote">f</div></td></tr>
      </tbody></table>
    `)
	if len(pages) != 2 {
		t.Fatalf("%d pages", len(pages))
	}
	tbody := footnoteTableBody(t, only(t, pages[0]))
	for _, tr := range tbody.Children {
		line := only(t, only(t, tr))
		columnText(t, only(t, line), "abc")
	}

	children := unpack(t, pages[1], 2)
	tbody = footnoteTableBody(t, children[0])
	line := only(t, only(t, only(t, tbody)))
	columnText(t, unpack(t, line, 2)[0], "def")
	footnoteLine := only(t, only(t, children[1]))
	columnText(t, unpack(t, footnoteLine, 2)[1], "f")
}

// TestFootnoteTableAbortedGroup is test_footnote_table_aborted_group.
func TestFootnoteTableAbortedGroup(t *testing.T) {
	pages := renderPages(t, `
      <style>
        @page {size: 10px 35px}
        body {font-family: weasyprint; font-size: 2px}
        tr {height: 10px}
        tbody {break-inside: avoid}
        .footnote {float: footnote}
      </style>
      <table>
        <tbody>
          <tr><td>abc</td></tr>
          <tr><td>abc</td></tr>
        </tbody>
        <tbody>
          <tr><td>def<div class="footnote">f</div></td></tr>
          <tr><td>ghi</td></tr>
        </tbody>
      </table>
    `)
	if len(pages) != 2 {
		t.Fatalf("%d pages", len(pages))
	}
	tbody := footnoteTableBody(t, only(t, pages[0]))
	for _, tr := range tbody.Children {
		line := only(t, only(t, tr))
		columnText(t, only(t, line), "abc")
	}

	children := unpack(t, pages[1], 2)
	tbody = footnoteTableBody(t, children[0])
	trs := unpack(t, tbody, 2)
	line := only(t, only(t, trs[0]))
	columnText(t, unpack(t, line, 2)[0], "def")
	line = only(t, only(t, trs[1]))
	columnText(t, only(t, line), "ghi")
	footnoteLine := only(t, only(t, children[1]))
	columnText(t, unpack(t, footnoteLine, 2)[1], "f")
}

// TestFootnoteBottomMargin is test_footnote_bottom_margin.
func TestFootnoteBottomMargin(t *testing.T) {
	page := pageRender(t, footnoteStyle("9px 7px", "span { margin-bottom: 1px; }")+`
        <div>abc<span>de</span></div>`, 1)[0]
	children := unpack(t, page, 2)
	div := only(t, only(t, children[0]))
	line := columnChild(t, div, 0)
	footnoteCall(t, line, "abc", "1")
	pageEqual(t, "text y", line.Children[0].PositionY, 0)
	footnoteMarkerText(t, children[1], 0, "1.", "de")
	pageEqual(t, "footnote area y", children[1].PositionY, 5)
}

// footnoteColumnsStyle is the style of the tests of the footnotes in
// columns.
const footnoteColumnsStyle = `
        <style>
            @page {
                size: 9px;
            }
            html {
                font: 2px/1 weasyprint;
            }
            div {
                columns: 2;
            }
            span {
                float: footnote;
            }
        </style>`

// TestFootnotesColumnConvergeEnd is test_footnotes_column_converge_end.
func TestFootnotesColumnConvergeEnd(t *testing.T) {
	pages := pageRender(t, footnoteColumnsStyle+`
        <div>
            a<span>1</span>
            b<span>2</span>
            c
            d<span>3</span>
        </div>
        <p>e</p>
    `, 2)

	// Page 1 contains a b c d 1 2.
	children := unpack(t, pages[0], 2)
	div := only(t, only(t, children[0]))
	columns := unpack(t, div, 2)
	pageEqual(t, "column 1 height", columns[0].Height, 4)
	pageEqual(t, "column 2 height", columns[1].Height, 4)
	pageEqual(t, "footnote area height", children[1].Height, 4)

	// Page 2 contains e 3.
	children = unpack(t, pages[1], 2)
	p := only(t, only(t, children[0]))
	pageEqual(t, "p height", p.Height, 2)
	pageEqual(t, "footnote area height", children[1].Height, 2)
}

// TestFootnotesColumnConverge is test_footnotes_column_converge.
func TestFootnotesColumnConverge(t *testing.T) {
	pages := pageRender(t, footnoteColumnsStyle+`
        <div>
            a<span>1</span>
            b<span>2</span>
            c
            d<span>3</span>
            e
            f
        </div>
        <p>g</p>
    `, 2)

	// Page 1 contains a b c d 1 2.
	children := unpack(t, pages[0], 2)
	div := only(t, only(t, children[0]))
	columns := unpack(t, div, 2)
	pageEqual(t, "column 1 height", columns[0].Height, 4)
	pageEqual(t, "column 2 height", columns[1].Height, 4)
	pageEqual(t, "footnote area height", children[1].Height, 4)

	// Page 2 contains e f g 3.
	children = unpack(t, pages[1], 2)
	bodyChildren := unpack(t, only(t, children[0]), 2)
	columns = unpack(t, bodyChildren[0], 2)
	pageEqual(t, "column 1 height", columns[0].Height, 2)
	pageEqual(t, "column 2 height", columns[1].Height, 2)
	pageEqual(t, "p height", bodyChildren[1].Height, 2)
	pageEqual(t, "footnote area height", children[1].Height, 2)
}

// noteStyle is the style of the tests of the notes, with the size of the
// page.
func noteStyle(size string) string {
	return `
        <style>
            @page {
                size: ` + size + `;
                @note-area {
                    content: element(sidenotes, all-once);
                }
            }
            div {
                font: 2px/1 weasyprint;
            }
            span {
                position: note(sidenotes);
                display: block;
            }
        </style>`
}

// noteCheck checks the marker, the text and the callback of the note of
// index i of the note area.
func noteCheck(t *testing.T, area *Box, i int, marker, text string) {
	t.Helper()
	parts := unpack(t, columnChild(t, area, i, 0), 3)
	columnText(t, parts[0], marker, 0)
	if parts[1].Text != text {
		t.Errorf("note text %q, want %q", parts[1].Text, text)
	}
	columnText(t, parts[2], "b", 0)
}

// TestOneNote is test_one_note of tests/layout/test_notes.py.
func TestOneNote(t *testing.T) {
	page := pageRender(t, noteStyle("10px 4px")+`
        <div>abc<span>de</span></div>`, 1)[0]
	children := unpack(t, page, 2)
	html, noteArea := children[0], children[1]
	div := only(t, columnChild(t, html, 0))
	line := columnChild(t, div, 0)
	footnoteCall(t, line, "abc", "1")
	pageEqual(t, "text y", line.Children[0].PositionY, 2)
	noteCheck(t, noteArea, 0, "1.", "de")
}

// TestSeveralBlockNotes is test_several_block_notes of
// tests/layout/test_notes.py.
func TestSeveralBlockNotes(t *testing.T) {
	page := pageRender(t, noteStyle("30px 8px")+`
        <div>abc<span>de</span>
          fgh<span>ij</span>
          klm<span>no</span></div>`, 1)[0]
	children := unpack(t, page, 2)
	html, noteArea := children[0], children[1]
	div := only(t, columnChild(t, html, 0))
	divChildren := columnChild(t, div, 0).Children
	if len(divChildren) < 6 {
		t.Fatalf("line children %v", divChildren)
	}
	for i, c := range []struct{ text, call string }{{"abc", "1"}, {" fgh", "2"}, {" klm", "3"}} {
		if divChildren[2*i].Text != c.text {
			t.Errorf("text %q, want %q", divChildren[2*i].Text, c.text)
		}
		columnText(t, divChildren[2*i+1], c.call, 0)
		pageEqual(t, "text y", divChildren[2*i].PositionY, 6)
	}
	noteCheck(t, noteArea, 0, "1.", "de")
	noteCheck(t, noteArea, 1, "2.", "ij")
	noteCheck(t, noteArea, 2, "3.", "no")
}
