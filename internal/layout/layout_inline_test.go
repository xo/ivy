// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_inline.py: the tests of the layout of
// the inline boxes.

package layout

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// inlineSansFonts is SANS_FONTS in tests/testing_utils.py, with the fonts
// of the tests, as sansFonts.
const inlineSansFonts = "Go, sans-serif"

// inlineRender is render_pages with assert_no_logs, for a document of one
// page: `page, = render_pages(source)`.
func inlineRender(t *testing.T, source string) *Box {
	t.Helper()
	log, c := captureLogs(slog.LevelWarn)
	pages := renderPagesLog(t, source, testBaseURL, log)
	assertNoLogs(t, c)
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	return pages[0]
}

// inlineBody returns the box of the body of a document of one page.
func inlineBody(t *testing.T, source string) *Box {
	t.Helper()
	_, body := pageBody(t, inlineRender(t, source))
	return body
}

// inlineAt returns the descendant of box at a path of indexes of children,
// as box.children[i].children[j] does, and fails the test when it does not
// exist.
func inlineAt(t *testing.T, box *Box, path ...int) *Box {
	t.Helper()
	for _, i := range path {
		if i >= len(box.Children) {
			t.Fatalf("%s has %d children, no child %d", box, len(box.Children), i)
		}
		box = box.Children[i]
	}
	return box
}

// inlineText checks the text of a text box.
func inlineText(t *testing.T, box *Box, want string) {
	t.Helper()
	if box.Kind != KindText || box.Text != want {
		t.Errorf("%s has text %q, want %q", box, box.Text, want)
	}
}

// inlineEqual checks that a value is exactly the value of upstream.
func inlineEqual(t *testing.T, name string, got, want float64) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestEmptyLinebox(t *testing.T) {
	body := inlineBody(t, "<p> </p>")
	paragraph := only(t, body)
	if len(paragraph.Children) != 0 {
		t.Errorf("paragraph has %d children", len(paragraph.Children))
	}
	inlineEqual(t, "paragraph.height", paragraph.Height, 0)
}

func TestEmptyLineboxRemovedSpace(t *testing.T) {
	t.Skip("upstream marks test_empty_linebox_removed_space as an expected failure")
}

func TestBreakingLinebox(t *testing.T) {
	body := inlineBody(t, fmt.Sprintf(`
      <style>
      p { font-size: 13px;
          width: 300px;
          font-family: %s;
          background-color: #393939;
          color: #FFFFFF;
          line-height: 1;
          text-decoration: underline overline line-through;}
      </style>
      <p><em>Lorem<strong> Ipsum <span>is very</span>simply</strong><em>
      dummy</em>text of the printing and. naaaa </em> naaaa naaaa naaaa
      naaaa naaaa naaaa naaaa naaaa</p>
    `, inlineSansFonts))
	paragraph := only(t, body)
	if len(paragraph.Children) != 3 {
		t.Errorf("paragraph has %d lines, want 3", len(paragraph.Children))
	}
	for _, line := range paragraph.Children {
		if num(line.Style.Get("font_size")) != 13 || line.ElementTag != "p" {
			t.Errorf("line %s has a font size of %v", line, line.Style.Get("font_size"))
		}
		for _, child := range line.Children {
			if child.ElementTag != "em" && child.ElementTag != "p" {
				t.Errorf("unexpected child %s", child)
			}
			if num(child.Style.Get("font_size")) != 13 {
				t.Errorf("child %s has a font size of %v", child, child.Style.Get("font_size"))
			}
			for range child.Children {
				switch child.ElementTag {
				case "em", "strong", "span":
				default:
					t.Errorf("unexpected child %s", child)
				}
			}
		}
	}
}

func TestPositionXLTR(t *testing.T) {
	body := inlineBody(t, `
      <style>
        span {
          padding: 0 10px 0 15px;
          margin: 0 2px 0 3px;
          border: 1px solid;
         }
      </style>
      <body><span>a<br>b<br>c</span>`)
	lines := unpack(t, body, 3)
	span1 := only(t, lines[0])
	inlineEqual(t, "span1.position_x", span1.PositionX, 0)
	text1 := unpack(t, span1, 2)[0]
	inlineEqual(t, "text1.position_x", text1.PositionX, 15+3+1)
	span2 := only(t, lines[1])
	inlineEqual(t, "span2.position_x", span2.PositionX, 0)
	text2 := unpack(t, span2, 2)[0]
	inlineEqual(t, "text2.position_x", text2.PositionX, 0)
	span3 := only(t, lines[2])
	inlineEqual(t, "span3.position_x", span3.PositionX, 0)
	text3 := only(t, span3)
	inlineEqual(t, "text3.position_x", text3.PositionX, 0)
}

func TestPositionXRTL(t *testing.T) {
	body := inlineBody(t, `
      <style>
        body {
          direction: rtl;
          width: 100px;
        }
        span {
          padding: 0 10px 0 15px;
          margin: 0 2px 0 3px;
          border: 1px solid;
         }
      </style>
      <body><span>a<br>b<br>c</span>`)
	lines := unpack(t, body, 3)
	span1 := only(t, lines[0])
	text1 := unpack(t, span1, 2)[0]
	inlineEqual(t, "span1.position_x", span1.PositionX, 100-text1.Width-(10+2+1))
	inlineEqual(t, "text1.position_x", text1.PositionX, 100-text1.Width-(10+2+1))
	span2 := only(t, lines[1])
	text2 := unpack(t, span2, 2)[0]
	inlineEqual(t, "span2.position_x", span2.PositionX, 100-text2.Width)
	inlineEqual(t, "text2.position_x", text2.PositionX, 100-text2.Width)
	span3 := only(t, lines[2])
	text3 := only(t, span3)
	inlineEqual(t, "span3.position_x", span3.PositionX, 100-text3.Width-(15+3+1))
	inlineEqual(t, "text3.position_x", text3.PositionX, 100-text3.Width)
}

func TestBreakingLineboxRegression1(t *testing.T) {
	// See https://unicode.org/reports/tr14/
	body := inlineBody(t, "<pre>a\nb\rc\r\nd e</pre>")
	pre := only(t, body)
	texts := make([]string, 0, len(pre.Children))
	for _, line := range pre.Children {
		texts = append(texts, only(t, line).Text)
	}
	if got := strings.Join(texts, ","); got != "a,b,c,d,e" {
		t.Errorf("texts are %q", texts)
	}
}

func TestBreakingLineboxRegression2(t *testing.T) {
	const htmlSample = `
      <p style="width: %d.5em; font-family: weasyprint">ab
      <span style="padding-right: 1em; margin-right: 1em">c def</span>g
      hi</p>`
	for i := range 16 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			p := only(t, inlineBody(t, fmt.Sprintf(htmlSample, i)))
			switch {
			case i <= 3:
				lines := unpack(t, p, 4)
				inlineText(t, only(t, lines[0]), "ab")
				inlineText(t, only(t, only(t, lines[1])), "c")
				line3 := unpack(t, lines[2], 2)
				inlineText(t, only(t, line3[0]), "def")
				inlineText(t, line3[1], "g")
				inlineText(t, only(t, lines[3]), "hi")
			case i <= 8:
				lines := unpack(t, p, 3)
				line1 := unpack(t, lines[0], 2)
				inlineText(t, line1[0], "ab ")
				inlineText(t, only(t, line1[1]), "c")
				line2 := unpack(t, lines[1], 2)
				inlineText(t, only(t, line2[0]), "def")
				inlineText(t, line2[1], "g")
				inlineText(t, only(t, lines[2]), "hi")
			case i <= 10:
				lines := unpack(t, p, 2)
				line1 := unpack(t, lines[0], 2)
				inlineText(t, line1[0], "ab ")
				inlineText(t, only(t, line1[1]), "c")
				line2 := unpack(t, lines[1], 2)
				inlineText(t, only(t, line2[0]), "def")
				inlineText(t, line2[1], "g hi")
			case i <= 13:
				lines := unpack(t, p, 2)
				line1 := unpack(t, lines[0], 3)
				inlineText(t, line1[0], "ab ")
				inlineText(t, only(t, line1[1]), "c def")
				inlineText(t, line1[2], "g")
				inlineText(t, only(t, lines[1]), "hi")
			default:
				line1 := unpack(t, only(t, p), 3)
				inlineText(t, line1[0], "ab ")
				inlineText(t, only(t, line1[1]), "c def")
				inlineText(t, line1[2], "g hi")
			}
		})
	}
}

func TestBreakingLineboxRegression3(t *testing.T) {
	// Regression test for #560.
	div := only(t, inlineBody(t,
		`<div style="width: 5.5em; font-family: weasyprint">`+
			`aaaa aaaa a [<span>aaa</span>]`))
	lines := unpack(t, div, 4)
	inlineText(t, inlineAt(t, lines[0], 0), "aaaa")
	inlineText(t, inlineAt(t, lines[1], 0), "aaaa")
	inlineText(t, inlineAt(t, lines[2], 0), "a")
	line4 := unpack(t, lines[3], 3)
	inlineText(t, line4[0], "[")
	inlineText(t, line4[2], "]")
	inlineText(t, inlineAt(t, line4[1], 0), "aaa")
}

func TestBreakingLineboxRegression4(t *testing.T) {
	// Regression test for #560.
	div := only(t, inlineBody(t,
		`<div style="width: 5.5em; font-family: weasyprint">`+
			`aaaa a <span>b c</span>d`))
	lines := unpack(t, div, 3)
	inlineText(t, inlineAt(t, lines[0], 0), "aaaa")
	inlineText(t, inlineAt(t, lines[1], 0), "a ")
	inlineText(t, inlineAt(t, lines[1], 1, 0), "b")
	inlineText(t, inlineAt(t, lines[2], 0, 0), "c")
	inlineText(t, inlineAt(t, lines[2], 1), "d")
}

func TestBreakingLineboxRegression5(t *testing.T) {
	// Regression test for #580.
	div := only(t, inlineBody(t,
		`<div style="width: 5.5em; font-family: weasyprint">`+
			`<span>aaaa aaaa a a a</span><span>bc</span>`))
	lines := unpack(t, div, 4)
	inlineText(t, inlineAt(t, lines[0], 0, 0), "aaaa")
	inlineText(t, inlineAt(t, lines[1], 0, 0), "aaaa")
	inlineText(t, inlineAt(t, lines[2], 0, 0), "a a")
	inlineText(t, inlineAt(t, lines[3], 0, 0), "a")
	inlineText(t, inlineAt(t, lines[3], 1, 0), "bc")
}

func TestBreakingLineboxRegression6(t *testing.T) {
	// Regression test for #586.
	div := only(t, inlineBody(t,
		`<div style="width: 5.5em; font-family: weasyprint">`+
			`a a <span style="white-space: nowrap">/ccc</span>`))
	lines := unpack(t, div, 2)
	inlineText(t, inlineAt(t, lines[0], 0), "a a")
	inlineText(t, inlineAt(t, lines[1], 0, 0), "/ccc")
}

func TestBreakingLineboxRegression7(t *testing.T) {
	// Regression test for #660.
	div := only(t, inlineBody(t,
		`<div style="width: 3.5em; font-family: weasyprint">`+
			`<span><span>abc d e</span></span><span>f`))
	lines := unpack(t, div, 3)
	inlineText(t, inlineAt(t, lines[0], 0, 0, 0), "abc")
	inlineText(t, inlineAt(t, lines[1], 0, 0, 0), "d")
	inlineText(t, inlineAt(t, lines[2], 0, 0, 0), "e")
	inlineText(t, inlineAt(t, lines[2], 1, 0), "f")
}

func TestBreakingLineboxRegression8(t *testing.T) {
	// Regression test for #783.
	p := only(t, inlineBody(t,
		"<p style=\"font-family: weasyprint\"><span>\n"+
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"+
			"bbbbbbbbbbb\n"+
			"<b>cccc</b></span>ddd</p>"))
	lines := unpack(t, p, 2)
	inlineText(t, inlineAt(t, lines[0], 0, 0), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa bbbbbbbbbbb")
	inlineText(t, inlineAt(t, lines[1], 0, 0, 0), "cccc")
	inlineText(t, inlineAt(t, lines[1], 1), "ddd")
}

func TestBreakingLineboxRegression9(t *testing.T) {
	t.Skip("upstream marks test_breaking_linebox_regression_9 as an expected failure")
}

func TestBreakingLineboxRegression10(t *testing.T) {
	// Regression test for #923.
	p := only(t, inlineBody(t,
		`<p style="width:195px; font-family: weasyprint">`+
			`  <span>`+
			`    <span>xxxxxx YYY yyyyyy yyy</span>`+
			`    ZZZZZZ zzzzz`+
			`  </span> )x `+
			`</p>`))
	lines := unpack(t, p, 4)
	inlineText(t, inlineAt(t, lines[0], 0, 0, 0), "xxxxxx YYY")
	inlineText(t, inlineAt(t, lines[1], 0, 0, 0), "yyyyyy yyy")
	inlineText(t, inlineAt(t, lines[2], 0, 0), "ZZZZZZ zzzzz")
	inlineText(t, inlineAt(t, lines[3], 0), ")x")
}

func TestBreakingLineboxRegression11(t *testing.T) {
	// Regression test for #953.
	p := only(t, inlineBody(t,
		`<p style="width:10em; font-family: weasyprint">`+
			`  line 1<br><span>123 567 90</span>x`+
			`</p>`))
	lines := unpack(t, p, 3)
	inlineText(t, inlineAt(t, lines[0], 0), "line 1")
	inlineText(t, inlineAt(t, lines[1], 0, 0), "123 567")
	inlineText(t, inlineAt(t, lines[2], 0, 0), "90")
	inlineText(t, inlineAt(t, lines[2], 1), "x")
}

func TestBreakingLineboxRegression12(t *testing.T) {
	// Regression test for #953.
	p := only(t, inlineBody(t,
		`<p style="width:10em; font-family: weasyprint">`+
			`  <br><span>123 567 90</span>x`+
			`</p>`))
	lines := unpack(t, p, 3)
	inlineText(t, inlineAt(t, lines[1], 0, 0), "123 567")
	inlineText(t, inlineAt(t, lines[2], 0, 0), "90")
	inlineText(t, inlineAt(t, lines[2], 1), "x")
}

func TestBreakingLineboxRegression13(t *testing.T) {
	// Regression test for #953.
	p := only(t, inlineBody(t,
		`<p style="width:10em; font-family: weasyprint">`+
			`  123 567 90 <span>123 567 90</span>x`+
			`</p>`))
	lines := unpack(t, p, 3)
	inlineText(t, inlineAt(t, lines[0], 0), "123 567 90")
	inlineText(t, inlineAt(t, lines[1], 0, 0), "123 567")
	inlineText(t, inlineAt(t, lines[2], 0, 0), "90")
	inlineText(t, inlineAt(t, lines[2], 1), "x")
}

func TestBreakingLineboxRegression14(t *testing.T) {
	// Regression test for #1638.
	body := inlineBody(t,
		`<style>`+
			`  body {font-family: weasyprint; width: 3em}`+
			`</style>`+
			`<span> <span>a</span> b</span><span>c</span>`)
	lines := unpack(t, body, 2)
	inlineText(t, inlineAt(t, lines[0], 0, 0, 0), "a")
	inlineText(t, inlineAt(t, lines[1], 0, 0), "b")
	inlineText(t, inlineAt(t, lines[1], 1, 0), "c")
}

func TestBreakingLineboxRegression15(t *testing.T) {
	// Regression test for #5507.
	body := inlineBody(t,
		"<style>"+
			"  body {font-family: weasyprint; font-size: 4px}"+
			"  pre {float: left}"+
			"</style>"+
			"<pre>ab©\n"+
			"déf\n"+
			"ghïj\n"+
			"klm</pre>")
	pre := only(t, body)
	lines := unpack(t, pre, 4)
	for i, want := range []string{"ab©", "déf", "ghïj", "klm"} {
		inlineText(t, inlineAt(t, lines[i], 0), want)
	}
	for i, want := range []float64{4 * 3, 4 * 3, 4 * 4, 4 * 3} {
		inlineEqual(t, fmt.Sprintf("line%d.children[0].width", i+1), inlineAt(t, lines[i], 0).Width, want)
	}
	inlineEqual(t, "pre.width", pre.Width, 4*4)
}

func TestBreakingLineboxRegression16(t *testing.T) {
	// Regression test for #1973.
	body := inlineBody(t,
		"<style>"+
			"  body {font-family: weasyprint; font-size: 4px}"+
			"  p {float: left}"+
			"</style>"+
			"<p>tést</p>"+
			"<pre>ab©\n"+
			"déf\n"+
			"ghïj\n"+
			"klm</pre>")
	children := unpack(t, body, 2)
	p, pre := children[0], children[1]
	inlineText(t, inlineAt(t, only(t, p), 0), "tést")
	inlineEqual(t, "p.width", p.Width, 4*4)
	lines := unpack(t, pre, 4)
	for i, want := range []string{"ab©", "déf", "ghïj", "klm"} {
		inlineText(t, inlineAt(t, lines[i], 0), want)
	}
	for i, want := range []float64{4 * 3, 4 * 3, 4 * 4, 4 * 3} {
		inlineEqual(t, fmt.Sprintf("line%d.children[0].width", i+1), inlineAt(t, lines[i], 0).Width, want)
	}
}

func TestLineboxText(t *testing.T) {
	body := inlineBody(t, fmt.Sprintf(`
      <style>
        p { width: 165px; font-family:%s;}
      </style>
      <p><em>Lorem Ipsum</em>is very <strong>coool</strong></p>
    `, inlineSansFonts))
	paragraph := only(t, body)
	lines := paragraph.Children
	if len(lines) != 2 {
		t.Errorf("paragraph has %d lines, want 2", len(lines))
	}
	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		var text strings.Builder
		line.Descendants(func(box *Box) bool {
			if box.Kind == KindText {
				text.WriteString(box.Text)
			}
			return true
		})
		texts = append(texts, text.String())
	}
	if got := strings.Join(texts, " "); got != "Lorem Ipsumis very coool" {
		t.Errorf("text is %q", got)
	}
}

func TestLineboxPositions(t *testing.T) {
	for _, c := range []struct {
		width, expectedLines int
	}{{165, 2}, {1, 5}, {0, 5}} {
		t.Run(fmt.Sprint(c.width), func(t *testing.T) {
			body := inlineBody(t, fmt.Sprintf(`
          <style>
            p { width:%dpx; font-family:%s;
                line-height: 20px }
          </style>
          <p>this is test for <strong>Weasyprint</strong></p>`, c.width, inlineSansFonts))
			paragraph := only(t, body)
			lines := paragraph.Children
			if len(lines) != c.expectedLines {
				t.Fatalf("paragraph has %d lines, want %d", len(lines), c.expectedLines)
			}

			refPositionY := lines[0].PositionY
			refPositionX := lines[0].PositionX
			for _, line := range lines {
				inlineEqual(t, "line.position_y", line.PositionY, refPositionY)
				inlineEqual(t, "line.position_x", line.PositionX, refPositionX)
				for _, box := range line.Children {
					inlineEqual(t, "box.position_x", box.PositionX, refPositionX)
					refPositionX += box.Width
					inlineEqual(t, "box.position_y", box.PositionY, refPositionY)
				}
				if refPositionX-line.PositionX > line.Width {
					t.Errorf("the boxes of %s are wider than the line", line)
				}
				refPositionX = line.PositionX
				refPositionY += line.Height
			}
		})
	}
}

func TestForcedLineBreaksPre(t *testing.T) {
	// These lines should be small enough to fit on the default A4 page
	// with the default 12pt font-size.
	body := inlineBody(t, `
      <style> pre { line-height: 42px }</style>
      <pre>Lorem ipsum dolor sit amet,
          consectetur adipiscing elit.


          Sed sollicitudin nibh

          et turpis molestie tristique.</pre>
    `)
	pre := only(t, body)
	if pre.ElementTag != "pre" {
		t.Errorf("element tag %q", pre.ElementTag)
	}
	lines := unpack(t, pre, 7)
	for _, line := range lines {
		if line.Kind != KindLine {
			t.Errorf("%s is not a line box", line)
		}
		inlineEqual(t, "line.height", line.Height, 42)
	}
}

func TestForcedLineBreaksParagraph(t *testing.T) {
	body := inlineBody(t, `
      <style> p { line-height: 42px }</style>
      <p>Lorem ipsum dolor sit amet,<br>
        consectetur adipiscing elit.<br><br><br>
        Sed sollicitudin nibh<br>
        <br>

        et turpis molestie tristique.</p>
    `)
	paragraph := only(t, body)
	if paragraph.ElementTag != "p" {
		t.Errorf("element tag %q", paragraph.ElementTag)
	}
	lines := unpack(t, paragraph, 7)
	for _, line := range lines {
		if line.Kind != KindLine {
			t.Errorf("%s is not a line box", line)
		}
		inlineEqual(t, "line.height", line.Height, 42)
	}
}

func TestInlineboxSplitting(t *testing.T) {
	// Regression test for #389.
	// The text is strange to test some corner cases.
	for _, width := range []int{10000, 100, 10, 0} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			body := inlineBody(t, fmt.Sprintf(`
          <style>p { font-family:%s; width: %dpx; }</style>
          <p><strong>WeasyPrint is a frée softwäre ./ visual rendèring enginè
                     for HTML !!! and CSS.</strong></p>
        `, inlineSansFonts, width))
			paragraph := only(t, body)
			lines := paragraph.Children
			if width == 10000 && len(lines) != 1 || width != 10000 && len(lines) <= 1 {
				t.Errorf("paragraph has %d lines", len(lines))
			}
			var textParts []string
			for _, line := range lines {
				strong := only(t, line)
				textParts = append(textParts, only(t, strong).Text)
			}
			want := "WeasyPrint is a frée softwäre ./ visual " +
				"rendèring enginè for HTML !!! and CSS."
			if got := strings.Join(textParts, " "); got != want {
				t.Errorf("text is %q", got)
			}
		})
	}
}

func TestWhitespaceProcessing(t *testing.T) {
	for _, source := range []string{"a", "  a  ", " \n  \ta", " a\t "} {
		t.Run(fmt.Sprintf("%q", source), func(t *testing.T) {
			body := inlineBody(t, fmt.Sprintf("<p><em>%s</em></p>", source))
			p := only(t, body)
			line := only(t, p)
			em := only(t, line)
			inlineText(t, only(t, em), "a")

			body = inlineBody(t, fmt.Sprintf(
				"<p style=\"white-space: pre-line\">\n\n<em>%s</em></pre>",
				strings.ReplaceAll(source, "\n", " ")))
			p = only(t, body)
			lines := unpack(t, p, 3)
			em = only(t, lines[2])
			inlineText(t, only(t, em), "a")
		})
	}
}

func TestInlineReplacedAutoMargins(t *testing.T) {
	body := inlineBody(t, `
      <style>
        @page { size: 200px }
        img { display: inline; margin: auto; width: 50px }
      </style>
      <body><img src="pattern.png" />`)
	line := only(t, body)
	img := only(t, line)
	inlineEqual(t, "img.margin_top", img.MarginTop, 0)
	inlineEqual(t, "img.margin_right", img.MarginRight, 0)
	inlineEqual(t, "img.margin_bottom", img.MarginBottom, 0)
	inlineEqual(t, "img.margin_left", img.MarginLeft, 0)
}

func TestEmptyInlineAutoMargins(t *testing.T) {
	body := inlineBody(t, `
      <style>
        @page { size: 200px }
        span { margin: auto }
      </style>
      <body><span></span>`)
	block := only(t, body)
	span := only(t, block)
	if span.MarginTop == 0 {
		t.Error("span.margin_top is 0")
	}
	inlineEqual(t, "span.margin_right", span.MarginRight, 0)
	if span.MarginBottom == 0 {
		t.Error("span.margin_bottom is 0")
	}
	inlineEqual(t, "span.margin_left", span.MarginLeft, 0)
}

func TestFontStretch(t *testing.T) {
	t.Skip("the fonts of the tests have no condensed face: Go has one width")
}

func TestLineCount(t *testing.T) {
	for _, c := range []struct {
		source     string
		linesCount int
		skip       string
	}{
		{"<body>hyphénation", 1, ""},                       // Default: no hyphenation
		{"<body lang=fr>hyphénation", 1, ""},               // lang only: no hyphenation
		{`<body style="hyphens: auto">hyphénation`, 1, ""}, // hyphens only: no hyph.
		{`<body style="hyphens: auto" lang=fr>hyphénation`, 4, "the port has no hyphenation for hyphens: auto"}, // both: hyph.
		{"<body>hyp&shy;hénation", 2, ""},                       // Hyphenation with soft hyphens
		{`<body style="hyphens: none">hyp&shy;hénation`, 1, ""}, // … unless disabled
	} {
		t.Run(c.source, func(t *testing.T) {
			if c.skip != "" {
				t.Skip(c.skip)
			}
			body := inlineBody(t, `<html style="width: 5em; font-family: weasyprint">`+c.source)
			if len(body.Children) != c.linesCount {
				t.Errorf("body has %d lines, want %d", len(body.Children), c.linesCount)
			}
		})
	}
}

func TestVerticalAlign1(t *testing.T) {
	//            +-------+      <- position_y = 0
	//      +-----+       |
	// 40px |     |       | 60px
	//      |     |       |
	//      +-----+-------+      <- baseline
	body := inlineBody(t, `
      <span>
        <img src="pattern.png" style="width: 40px"
        ><img src="pattern.png" style="width: 60px"
      ></span>`)
	line := only(t, body)
	span := only(t, line)
	imgs := unpack(t, span, 2)
	inlineEqual(t, "img_1.height", imgs[0].Height, 40)
	inlineEqual(t, "img_2.height", imgs[1].Height, 60)
	inlineEqual(t, "img_1.position_y", imgs[0].PositionY, 20)
	inlineEqual(t, "img_2.position_y", imgs[1].PositionY, 0)
	// 60px + the descent of the font below the baseline
	if !(60 < line.Height && line.Height < 70) {
		t.Errorf("line.height = %v", line.Height)
	}
	inlineEqual(t, "body.height", body.Height, line.Height)
}

func TestVerticalAlign2(t *testing.T) {
	//            +-------+      <- position_y = 0
	//       35px |       |
	//      +-----+       | 60px
	// 40px |     |       |
	//      |     +-------+      <- baseline
	//      +-----+  15px
	body := inlineBody(t, `
      <span>
        <img src="pattern.png" style="width: 40px; vertical-align: -15px"
        ><img src="pattern.png" style="width: 60px"></span>`)
	line := only(t, body)
	imgs := unpack(t, only(t, line), 2)
	inlineEqual(t, "img_1.height", imgs[0].Height, 40)
	inlineEqual(t, "img_2.height", imgs[1].Height, 60)
	inlineEqual(t, "img_1.position_y", imgs[0].PositionY, 35)
	inlineEqual(t, "img_2.position_y", imgs[1].PositionY, 0)
	inlineEqual(t, "line.height", line.Height, 75)
	inlineEqual(t, "body.height", body.Height, line.Height)
}

func TestVerticalAlign3(t *testing.T) {
	// Same as previously, but with percentages
	body := inlineBody(t, `
      <span style="line-height: 10px">
        <img src="pattern.png" style="width: 40px; vertical-align: -150%"
        ><img src="pattern.png" style="width: 60px"></span>`)
	line := only(t, body)
	imgs := unpack(t, only(t, line), 2)
	inlineEqual(t, "img_1.height", imgs[0].Height, 40)
	inlineEqual(t, "img_2.height", imgs[1].Height, 60)
	inlineEqual(t, "img_1.position_y", imgs[0].PositionY, 35)
	inlineEqual(t, "img_2.position_y", imgs[1].PositionY, 0)
	inlineEqual(t, "line.height", line.Height, 75)
	inlineEqual(t, "body.height", body.Height, line.Height)
}

func TestVerticalAlign4(t *testing.T) {
	// Same again, but have the vertical-align on an inline box.
	body := inlineBody(t, `
      <span style="line-height: 10px">
        <span style="line-height: 10px; vertical-align: -15px">
          <img src="pattern.png" style="width: 40px"></span>
        <img src="pattern.png" style="width: 60px"></span>`)
	line := only(t, body)
	span1 := only(t, line)
	children := unpack(t, span1, 3)
	span2, img2 := children[0], children[2]
	img1 := only(t, span2)
	inlineEqual(t, "img_1.height", img1.Height, 40)
	inlineEqual(t, "img_2.height", img2.Height, 60)
	inlineEqual(t, "img_1.position_y", img1.PositionY, 35)
	inlineEqual(t, "img_2.position_y", img2.PositionY, 0)
	inlineEqual(t, "line.height", line.Height, 75)
	inlineEqual(t, "body.height", body.Height, line.Height)
}

func TestVerticalAlign5(t *testing.T) {
	// Same as previously, but with percentages
	body := inlineBody(t,
		`<span style="line-height: 12px; font-size: 12px; font-family: weasyprint">`+
			`<img src="pattern.png" style="width: 40px; vertical-align: middle">`+
			`<img src="pattern.png" style="width: 60px"></span>`)
	line := only(t, body)
	imgs := unpack(t, only(t, line), 2)
	inlineEqual(t, "img_1.height", imgs[0].Height, 40)
	inlineEqual(t, "img_2.height", imgs[1].Height, 60)
	// middle of the image (position_y + 20) is at half the ex-height above
	// the baseline of the parent. The ex-height of weasyprint.otf is 0.8em
	// TODO: ex unit doesn't work with @font-face fonts, see computed_values.py
	// assert img_1.position_y == 35.2  # 60 - 0.5 * 0.8 * font-size - 40/2
	inlineEqual(t, "img_2.position_y", imgs[1].PositionY, 0)
	// assert line.height == 75.2
	inlineEqual(t, "body.height", body.Height, line.Height)
}

func TestVerticalAlign6(t *testing.T) {
	// sup and sub currently mean +/- 0.5 em
	// With the initial 16px font-size, that’s 8px.
	body := inlineBody(t, `
      <span style="line-height: 10px">
        <img src="pattern.png" style="width: 60px"
        ><img src="pattern.png" style="width: 40px; vertical-align: super"
        ><img src="pattern.png" style="width: 40px; vertical-align: sub"
      ></span>`)
	line := only(t, body)
	imgs := unpack(t, only(t, line), 3)
	inlineEqual(t, "img_1.height", imgs[0].Height, 60)
	inlineEqual(t, "img_2.height", imgs[1].Height, 40)
	inlineEqual(t, "img_3.height", imgs[2].Height, 40)
	inlineEqual(t, "img_1.position_y", imgs[0].PositionY, 0)
	inlineEqual(t, "img_2.position_y", imgs[1].PositionY, 12) // 20 - 16 * 0.5
	inlineEqual(t, "img_3.position_y", imgs[2].PositionY, 28) // 20 + 16 * 0.5
	inlineEqual(t, "line.height", line.Height, 68)
	inlineEqual(t, "body.height", body.Height, line.Height)
}

func TestVerticalAlign7(t *testing.T) {
	body := inlineBody(t, `
      <body style="line-height: 10px">
        <span>
          <img src="pattern.png" style="vertical-align: text-top"
          ><img src="pattern.png" style="vertical-align: text-bottom"
        ></span>`)
	line := only(t, body)
	imgs := unpack(t, only(t, line), 2)
	inlineEqual(t, "img_1.height", imgs[0].Height, 4)
	inlineEqual(t, "img_2.height", imgs[1].Height, 4)
	inlineEqual(t, "img_1.position_y", imgs[0].PositionY, 0)
	inlineEqual(t, "img_2.position_y", imgs[1].PositionY, 12) // 16 - 4
	inlineEqual(t, "line.height", line.Height, 16)
	inlineEqual(t, "body.height", body.Height, line.Height)
}

func TestVerticalAlign8(t *testing.T) {
	// This case used to cause an exception:
	// The second span has no children but should count for line heights
	// since it has padding.
	body := inlineBody(t, `<span style="line-height: 1.5">
      <span style="padding: 1px"></span></span>`)
	line := only(t, body)
	span1 := only(t, line)
	span2 := only(t, span1)
	inlineEqual(t, "span_1.height", span1.Height, 16)
	inlineEqual(t, "span_2.height", span2.Height, 16)
	// The line’s strut does not has 'line-height: normal' but the result
	// should be smaller than 1.5.
	inlineEqual(t, "span_1.margin_height()", span1.MarginHeight(), 24)
	inlineEqual(t, "span_2.margin_height()", span2.MarginHeight(), 24)
	inlineEqual(t, "line.height", line.Height, 24)
}

func TestVerticalAlign9(t *testing.T) {
	body := inlineBody(t, `
      <span>
        <img src="pattern.png" style="width: 40px; vertical-align: -15px"
        ><img src="pattern.png" style="width: 60px"
      ></span><div style="display: inline-block; vertical-align: 3px">
        <div>
          <div style="height: 100px">foo</div>
          <div>
            <img src="pattern.png" style="
                 width: 40px; vertical-align: -15px"
            ><img src="pattern.png" style="width: 60px"
          ></div>
        </div>
      </div>`)
	line := only(t, body)
	children := unpack(t, line, 2)
	span, div1 := children[0], children[1]
	inlineEqual(t, "line.height", line.Height, 178)
	inlineEqual(t, "body.height", body.Height, line.Height)

	// Same as earlier
	imgs := unpack(t, span, 2)
	inlineEqual(t, "img_1.height", imgs[0].Height, 40)
	inlineEqual(t, "img_2.height", imgs[1].Height, 60)
	inlineEqual(t, "img_1.position_y", imgs[0].PositionY, 138)
	inlineEqual(t, "img_2.position_y", imgs[1].PositionY, 103)

	div2 := only(t, div1)
	divs := unpack(t, div2, 2)
	div3, div4 := divs[0], divs[1]
	divLine := only(t, div4)
	divImgs := unpack(t, divLine, 2)
	inlineEqual(t, "div_1.position_y", div1.PositionY, 0)
	inlineEqual(t, "div_1.height", div1.Height, 175)
	inlineEqual(t, "div_3.height", div3.Height, 100)
	inlineEqual(t, "div_line.height", divLine.Height, 75)
	inlineEqual(t, "div_img_1.height", divImgs[0].Height, 40)
	inlineEqual(t, "div_img_2.height", divImgs[1].Height, 60)
	inlineEqual(t, "div_img_1.position_y", divImgs[0].PositionY, 135)
	inlineEqual(t, "div_img_2.position_y", divImgs[1].PositionY, 100)
}

func TestVerticalAlign10(t *testing.T) {
	// The first two images bring the top of the line box 30px above
	// the baseline and 10px below.
	// Each of the inner span
	body := inlineBody(t, `
      <span style="font-size: 0">
        <img src="pattern.png" style="vertical-align: 26px">
        <img src="pattern.png" style="vertical-align: -10px">
        <span style="vertical-align: top">
          <img src="pattern.png" style="vertical-align: -10px">
          <span style="vertical-align: -10px">
            <img src="pattern.png" style="vertical-align: bottom">
          </span>
        </span>
        <span style="vertical-align: bottom">
          <img src="pattern.png" style="vertical-align: 6px">
        </span>
      </span>`)
	line := only(t, body)
	span1 := only(t, line)
	children := unpack(t, span1, 4)
	img1, img2, span2, span4 := children[0], children[1], children[2], children[3]
	children = unpack(t, span2, 2)
	img3, span3 := children[0], children[1]
	img4 := only(t, span3)
	img5 := only(t, span4)
	inlineEqual(t, "body.height", body.Height, line.Height)
	inlineEqual(t, "line.height", line.Height, 40)
	inlineEqual(t, "img_1.position_y", img1.PositionY, 0)
	inlineEqual(t, "img_2.position_y", img2.PositionY, 36)
	inlineEqual(t, "img_3.position_y", img3.PositionY, 6)
	inlineEqual(t, "img_4.position_y", img4.PositionY, 36)
	inlineEqual(t, "img_5.position_y", img5.PositionY, 30)
}

func TestVerticalAlign11(t *testing.T) {
	body := inlineBody(t, `
      <span style="font-size: 0">
        <img src="pattern.png" style="vertical-align: bottom">
        <img src="pattern.png" style="vertical-align: top; height: 100px">
      </span>
    `)
	line := only(t, body)
	imgs := unpack(t, only(t, line), 2)
	inlineEqual(t, "img_1.position_y", imgs[0].PositionY, 96)
	inlineEqual(t, "img_2.position_y", imgs[1].PositionY, 0)
}

func TestVerticalAlign12(t *testing.T) {
	// Reference for the next test
	body := inlineBody(t, `
      <span style="font-size: 0; vertical-align: top">
        <img src="pattern.png">
      </span>
    `)
	line := only(t, body)
	img1 := only(t, only(t, line))
	inlineEqual(t, "img_1.position_y", img1.PositionY, 0)
}

func TestVerticalAlign13(t *testing.T) {
	// Should be the same as above
	body := inlineBody(t, `
      <span style="font-size: 0; vertical-align: top; display: inline-block">
        <img src="pattern.png">
      </span>`)
	line1 := only(t, body)
	span := only(t, line1)
	line2 := only(t, span)
	img1 := only(t, line2)
	if img1.ElementTag != "img" {
		t.Errorf("element tag %q", img1.ElementTag)
	}
	inlineEqual(t, "img_1.position_y", img1.PositionY, 0)
}

func TestBoxDecorationBreakInlineSlice(t *testing.T) {
	// https://www.w3.org/TR/css-backgrounds-3/#the-box-decoration-break
	body := inlineBody(t, `
      <style>
        @page { size: 100px }
        span { font-family: weasyprint; box-decoration-break: slice;
               padding: 5px; border: 1px solid black }
      </style>
      <span>a<br/>b<br/>c</span>`)
	lines := unpack(t, body, 3)
	span := only(t, lines[0])
	inlineEqual(t, "span.width", span.Width, 16)
	inlineEqual(t, "span.margin_width()", span.MarginWidth(), 16+5+1)
	inlineEqual(t, "text.position_x", unpack(t, span, 2)[0].PositionX, 5+1)
	span = only(t, lines[1])
	inlineEqual(t, "span.width", span.Width, 16)
	inlineEqual(t, "span.margin_width()", span.MarginWidth(), 16)
	inlineEqual(t, "text.position_x", unpack(t, span, 2)[0].PositionX, 0)
	span = only(t, lines[2])
	inlineEqual(t, "span.width", span.Width, 16)
	inlineEqual(t, "span.margin_width()", span.MarginWidth(), 16+5+1)
	inlineEqual(t, "text.position_x", only(t, span).PositionX, 0)
}

func TestBoxDecorationBreakInlineClone(t *testing.T) {
	// https://www.w3.org/TR/css-backgrounds-3/#the-box-decoration-break
	body := inlineBody(t, `
      <style>
        @page { size: 100px }
        span { font-size: 12pt; font-family: weasyprint;
               box-decoration-break: clone;
               padding: 5px; border: 1px solid black }
      </style>
      <span>a<br/>b<br/>c</span>`)
	lines := unpack(t, body, 3)
	for i, line := range lines {
		span := only(t, line)
		inlineEqual(t, "span.width", span.Width, 16)
		inlineEqual(t, "span.margin_width()", span.MarginWidth(), 16+2*(5+1))
		text := inlineAt(t, span, 0)
		if i < 2 {
			unpack(t, span, 2)
		} else {
			only(t, span)
		}
		inlineEqual(t, "text.position_x", text.PositionX, 5+1)
	}
}

func TestBidiPositionXInvariant(t *testing.T) {
	body := inlineBody(t, `
      <style>
        .float-border {
          float: left;
          border-right: 100px solid black;
        }
      </style>
      <div class="float-border" style="direction: ltr">abc</div>
      <div>&nbsp;</div>
      <div class="float-border" style="direction: rtl">abc</div>
    `)
	blocks := unpack(t, body, 3)
	blockLTR, blockRTL := blocks[0], blocks[2]
	lineLTR := only(t, blockLTR)
	textLTR := only(t, lineLTR)
	lineRTL := only(t, blockRTL)
	textRTL := only(t, lineRTL)
	inlineEqual(t, "block_rtl.position_x", blockRTL.PositionX, blockLTR.PositionX)
	inlineEqual(t, "line_rtl.position_x", lineRTL.PositionX, lineLTR.PositionX)
	inlineEqual(t, "text_rtl.position_x", textRTL.PositionX, textLTR.PositionX)
}

func TestNestedWaitingChildrenWidth(t *testing.T) {
	// Regression test for #2275.
	body := inlineBody(t,
		`<body style="width: 3em; font-family: weasyprint">`+
			`<b><i style="width: 100%">a b</i>c`)
	lines := unpack(t, body, 2)
	inlineText(t, inlineAt(t, lines[0], 0, 0, 0), "a")
	inlineText(t, inlineAt(t, lines[1], 0, 0, 0), "b")
	inlineText(t, inlineAt(t, lines[1], 0, 1), "c")
}
