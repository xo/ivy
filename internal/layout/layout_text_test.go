// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports the tests of tests/test_text.py that need the layout:
// the tests that read the pages of render_pages. text_line_break_test.go
// holds the other tests of tests/test_text.py.
//
// SANS_FONTS and MONO_FONTS of upstream are the Go fonts. weasyprint.otf
// has glyphs that are 1em wide.

package layout

import (
	"fmt"
	"strings"
	"testing"
)

func TestLineBreakingNbsp(t *testing.T) {
	// Regression test for #1561.
	body := inlineBody(t, `
      <style>
        body { font-family: weasyprint; width: 7.5em }
      </style>
      <body>a <span>b</span> c d&nbsp;<span>ef
    `)
	lines := unpack(t, body, 2)
	inlineText(t, inlineAt(t, lines[0], 0), "a ")
	inlineText(t, inlineAt(t, lines[0], 1, 0), "b")
	inlineText(t, inlineAt(t, lines[0], 2), " c")
	inlineText(t, inlineAt(t, lines[1], 0), "d\u00a0")
	inlineText(t, inlineAt(t, lines[1], 1, 0), "ef")
}

func TestLineBreakingTrailingInlineWidth(t *testing.T) {
	body := inlineBody(t, `
      <style>
        @page { size: 520px 180px; margin: 20px }
        body {
          margin: 0;
          font-family: weasyprint;
          font-size: 16px;
          line-height: 1.25;
        }
        div {
          width: 27ch;
          text-align: right;
          border-right: 1px solid black;
        }
      </style>
      <div>
        <span>
          aaaaaaaaaa bbbbbbbbbb cccccccccc ddddd <span>e. 43</span>
          ff&nbsp;<span>ggggg</span>.
        </span>
      </div>
    `)
	div := only(t, body)
	lines := unpack(t, div, 3)
	span2 := only(t, lines[1])
	children := unpack(t, span2, 2)
	text2, nested2 := children[0], children[1]
	number := only(t, nested2)
	span3 := only(t, lines[2])
	children = unpack(t, span3, 3)
	text3, nested3, dot := children[0], children[1], children[2]
	word := only(t, nested3)

	inlineText(t, text2, "cccccccccc ddddd ")
	inlineText(t, number, "e. 43")
	inlineText(t, text3, "ff\u00a0")
	inlineText(t, word, "ggggg")
	inlineText(t, dot, ".")

	divRight := div.PositionX + div.Width
	numberRight := nested2.PositionX + nested2.Width
	spanRight := span2.PositionX + span2.Width
	assertApprox(t, "number_right", numberRight, divRight)
	assertApprox(t, "span_right", spanRight, numberRight)
}

func TestTextFontSizeZero(t *testing.T) {
	body := inlineBody(t, `
      <style>
        p { font-size: 0; }
      </style>
      <p>test font size zero</p>
    `)
	paragraph := only(t, body)
	line := only(t, paragraph)
	// zero-sized text boxes are removed
	if len(line.Children) != 0 {
		t.Errorf("line has %d children", len(line.Children))
	}
	inlineEqual(t, "line.height", line.Height, 0)
	inlineEqual(t, "paragraph.height", paragraph.Height, 0)
}

func TestTextFontSizeVerySmall(t *testing.T) {
	// Regression test for #1499.
	body := inlineBody(t, `
      <style>
        p { font-size: 0.00000001px }
      </style>
      <p>test font size zero</p>
    `)
	paragraph := only(t, body)
	line := only(t, paragraph)
	if line.Height >= 0.001 || paragraph.Height >= 0.001 {
		t.Errorf("heights %v %v", line.Height, paragraph.Height)
	}
}

func TestTextSpacedInlines(t *testing.T) {
	body := inlineBody(t, `
      <p>start <i><b>bi1</b> <b>bi2</b></i> <b>b1</b> end</p>
    `)
	line := only(t, only(t, body))
	children := unpack(t, line, 5)
	start, i, space, b, end := children[0], children[1], children[2], children[3], children[4]
	inlineText(t, start, "start ")
	inlineText(t, space, " ")
	if space.Width <= 0 {
		t.Errorf("space.width = %v", space.Width)
	}
	inlineText(t, end, " end")

	children = unpack(t, i, 3)
	inlineText(t, only(t, children[0]), "bi1")
	inlineText(t, children[1], " ")
	if children[1].Width <= 0 {
		t.Errorf("space.width = %v", children[1].Width)
	}
	inlineText(t, only(t, children[2]), "bi2")

	inlineText(t, only(t, b), "b1")
}

// inlineAlignedImages returns the positions of the images of the line of
// test_text_align_left, test_text_align_right and test_text_align_center.
func inlineAlignedImages(t *testing.T, css string) (x1, x2 float64) {
	t.Helper()
	body := inlineBody(t, `
      <style>
        @page { size: 200px }
        `+css+`
      </style>
      <body>
        <img src="pattern.png" style="width: 40px"
        ><img src="pattern.png" style="width: 60px">`)
	imgs := unpack(t, only(t, body), 2)
	return imgs[0].PositionX, imgs[1].PositionX
}

func TestTextAlignLeft(t *testing.T) {
	// <-------------------->  page, body
	//     +-----+
	// +---+     |
	// |   |     |
	// +---+-----+
	//
	// ^   ^     ^          ^
	// x=0 x=40  x=100      x=200
	x1, x2 := inlineAlignedImages(t, "")
	// initial value for text-align: left (in ltr text)
	inlineEqual(t, "img_1.position_x", x1, 0)
	inlineEqual(t, "img_2.position_x", x2, 40)
}

func TestTextAlignRight(t *testing.T) {
	// <-------------------->  page, body
	//                +-----+
	//            +---+     |
	//            |   |     |
	//            +---+-----+
	//
	// ^          ^   ^     ^
	// x=0        x=100     x=200
	//                x=140
	x1, x2 := inlineAlignedImages(t, "body { text-align: right }")
	inlineEqual(t, "img_1.position_x", x1, 100) // 200 - 60 - 40
	inlineEqual(t, "img_2.position_x", x2, 140) // 200 - 60
}

func TestTextAlignCenter(t *testing.T) {
	// <-------------------->  page, body
	//           +-----+
	//       +---+     |
	//       |   |     |
	//       +---+-----+
	//
	// ^     ^   ^     ^
	// x=    x=50     x=150
	//           x=90
	x1, x2 := inlineAlignedImages(t, "body { text-align: center }")
	inlineEqual(t, "img_1.position_x", x1, 50)
	inlineEqual(t, "img_2.position_x", x2, 90)
}

// inlineJustifiedLine1 checks the first line of the tests of the
// justification: an image of 40px, a space, and a strong with three
// images.
func inlineJustifiedLine1(t *testing.T, line *Box, indent float64) {
	t.Helper()
	children := unpack(t, line, 3)
	image1, space1, strong := children[0], children[1], children[2]
	children = unpack(t, strong, 5)
	image2, space2, image3, space3, image4 := children[0], children[1], children[2], children[3], children[4]
	inlineText(t, space1, " ")
	inlineText(t, space2, " ")
	inlineText(t, space3, " ")

	if indent == 0 {
		inlineEqual(t, "image_1.position_x", image1.PositionX, 0)
		inlineEqual(t, "space_1.position_x", space1.PositionX, 40)
		inlineEqual(t, "strong.position_x", strong.PositionX, 70)
		inlineEqual(t, "image_2.position_x", image2.PositionX, 70)
		inlineEqual(t, "space_2.position_x", space2.PositionX, 130)
		inlineEqual(t, "image_3.position_x", image3.PositionX, 160)
		inlineEqual(t, "space_3.position_x", space3.PositionX, 170)
		inlineEqual(t, "image_4.position_x", image4.PositionX, 200)
		inlineEqual(t, "strong.width", strong.Width, 230)
	} else {
		inlineEqual(t, "image_1.position_x", image1.PositionX, 3)
		inlineEqual(t, "space_1.position_x", space1.PositionX, 43)
		inlineEqual(t, "strong.position_x", strong.PositionX, 72)
		inlineEqual(t, "image_2.position_x", image2.PositionX, 72)
		inlineEqual(t, "space_2.position_x", space2.PositionX, 132)
		inlineEqual(t, "image_3.position_x", image3.PositionX, 161)
		inlineEqual(t, "space_3.position_x", space3.PositionX, 171)
		inlineEqual(t, "image_4.position_x", image4.PositionX, 200)
		inlineEqual(t, "strong.width", strong.Width, 228)
	}
}

func TestTextAlignJustify(t *testing.T) {
	body := inlineBody(t, `
      <style>
        @page { size: 300px 1000px }
        body { text-align: justify }
      </style>
      <p><img src="pattern.png" style="width: 40px">
        <strong>
          <img src="pattern.png" style="width: 60px">
          <img src="pattern.png" style="width: 10px">
          <img src="pattern.png" style="width: 100px"
        ></strong><img src="pattern.png" style="width: 290px"
        ><!-- Last image will be on its own line. -->`)
	lines := unpack(t, only(t, body), 2)
	inlineJustifiedLine1(t, lines[0], 0)
	inlineEqual(t, "image_5.position_x", only(t, lines[1]).PositionX, 0)
}

func TestTextAlignJustifyAll(t *testing.T) {
	body := inlineBody(t, `
      <style>
        @page { size: 300px 1000px }
        body { text-align: justify-all }
      </style>
      <p><img src="pattern.png" style="width: 40px">
        <strong>
          <img src="pattern.png" style="width: 60px">
          <img src="pattern.png" style="width: 10px">
          <img src="pattern.png" style="width: 100px"
        ></strong><img src="pattern.png" style="width: 200px">
        <img src="pattern.png" style="width: 10px">`)
	lines := unpack(t, only(t, body), 2)
	inlineJustifiedLine1(t, lines[0], 0)
	children := unpack(t, lines[1], 3)
	inlineText(t, children[1], " ")
	inlineEqual(t, "image_5.position_x", children[0].PositionX, 0)
	inlineEqual(t, "space_4.position_x", children[1].PositionX, 200)
	inlineEqual(t, "image_6.position_x", children[2].PositionX, 290)
}

func TestTextAlignAllLast(t *testing.T) {
	body := inlineBody(t, `
      <style>
        @page { size: 300px 1000px }
        body { text-align-all: justify; text-align-last: right }
      </style>
      <p><img src="pattern.png" style="width: 40px">
        <strong>
          <img src="pattern.png" style="width: 60px">
          <img src="pattern.png" style="width: 10px">
          <img src="pattern.png" style="width: 100px"
        ></strong><img src="pattern.png" style="width: 200px"
        ><img src="pattern.png" style="width: 10px">`)
	lines := unpack(t, only(t, body), 2)
	inlineJustifiedLine1(t, lines[0], 0)
	children := unpack(t, lines[1], 2)
	inlineEqual(t, "image_5.position_x", children[0].PositionX, 90)
	inlineEqual(t, "image_6.position_x", children[1].PositionX, 290)
}

func TestTextAlignNotEnoughSpace(t *testing.T) {
	body := inlineBody(t, `
      <style>
        p { text-align: center; width: 0 }
        span { display: inline-block }
      </style>
      <p><span>aaaaaaaaaaaaaaaaaaaaaaaaaa</span></p>`)
	span := only(t, only(t, body))
	inlineEqual(t, "span.position_x", span.PositionX, 0)
}

func TestTextAlignJustifyNoSpace(t *testing.T) {
	// single-word line (zero spaces)
	body := inlineBody(t, `
      <style>
        body { text-align: justify; width: 50px }
      </style>
      <p>Supercalifragilisticexpialidocious bar</p>
    `)
	lines := unpack(t, only(t, body), 2)
	inlineEqual(t, "text.position_x", only(t, lines[0]).PositionX, 0)
}

func TestTextAlignJustifyTextIndent(t *testing.T) {
	// text-indent
	body := inlineBody(t, `
      <style>
        @page { size: 300px 1000px }
        body { text-align: justify }
        p { text-indent: 3px }
      </style>
      <p><img src="pattern.png" style="width: 40px">
        <strong>
          <img src="pattern.png" style="width: 60px">
          <img src="pattern.png" style="width: 10px">
          <img src="pattern.png" style="width: 100px"
        ></strong><img src="pattern.png" style="width: 290px"
        ><!-- Last image will be on its own line. -->`)
	lines := unpack(t, only(t, body), 2)
	inlineJustifiedLine1(t, lines[0], 3)
	inlineEqual(t, "image_5.position_x", only(t, lines[1]).PositionX, 0)
}

func TestTextAlignJustifyNoBreakBetweenChildren(t *testing.T) {
	// Test justification when line break happens between two inline
	// children that must stay together.
	// Regression test for #637.
	body := inlineBody(t, `
      <style>
        p { text-align: justify; font-family: weasyprint; width: 7em }
      </style>
      <p>
        <span>a</span>
        <span>b</span>
        <span>bla</span><span>,</span>
        <span>b</span>
      </p>
    `)
	lines := unpack(t, only(t, body), 2)
	children := unpack(t, lines[0], 4)
	inlineEqual(t, "span_1.position_x", children[0].PositionX, 0)
	inlineEqual(t, "span_2.position_x", children[2].PositionX, 6*16) // 1 character + 5 spaces
	inlineEqual(t, "line_1.width", lines[0].Width, 7*16)             // 7em

	children = unpack(t, lines[1], 5)
	inlineEqual(t, "span_1.position_x", children[0].PositionX, 0)
	inlineEqual(t, "span_2.position_x", children[1].PositionX, 3*16) // 3 characters
	inlineEqual(t, "span_3.position_x", children[3].PositionX, 5*16) // (3 + 1) characters + 1 space
}

// inlineFirstWidth returns the width of the only child of the only line of
// the body of a document.
func inlineFirstWidth(t *testing.T, html string) float64 {
	t.Helper()
	return only(t, only(t, inlineBody(t, html))).Width
}

func TestWordSpacing(t *testing.T) {
	for _, text := range []string{
		"Lorem ipsum dolor<em>sit amet</em>",
		"Lorem ipsum <em>dolorsit</em> amet",
		"Lorem ipsum <em></em>dolorsit amet",
		"Lorem ipsum<em> </em>dolorsit amet",
		"Lorem ipsum<em> dolorsit</em> amet",
		"Lorem ipsum <em>dolorsit </em>amet",
	} {
		t.Run(text, func(t *testing.T) {
			// Regression test.
			width1 := inlineFirstWidth(t, `
      <style></style> <!-- element.text is None -->
      <body><strong>Lorem ipsum dolor<em>sit amet</em></strong>`)
			width2 := inlineFirstWidth(t, `
      <style>strong { word-spacing: 11px }</style>
      <body><strong>`+text+`</strong>`)
			assertApprox(t, "strong_2.width - strong_1.width", width2-width1, 33)
		})
	}
}

func TestLetterSpacing1(t *testing.T) {
	width1 := inlineFirstWidth(t, `
        <body><strong>Supercalifragilisticexpialidocious</strong>`)
	width2 := inlineFirstWidth(t, `
        <style>strong { letter-spacing: 11px }</style>
        <body><strong>Supercalifragilisticexpialidocious</strong>`)
	assertApprox(t, "strong_2.width - strong_1.width", width2-width1, 34*11)

	// an embedded tag should not affect the single-line letter spacing
	width3 := inlineFirstWidth(t,
		`<style>strong { letter-spacing: 11px }</style>`+
			`<body><strong>Supercali<span>fragilistic</span>expialidocious`+
			`</strong>`)
	assertApprox(t, "strong_3.width", width3, width2)

	// duplicate wrapped lines should also have same overall width
	// Note work-around for word-wrap bug (issue #163) by marking word
	// as an inline-block
	body := inlineBody(t,
		`<style>`+
			`  strong {`+
			`    letter-spacing: 11px;`+
			fmt.Sprintf(`    max-width: %vpx`, width3*1.5)+
			`}`+
			`  span { display: inline-block }`+
			`</style>`+
			`<body><strong>`+
			`  <span>Supercali<i>fragilistic</i>expialidocious</span> `+
			`  <span>Supercali<i>fragilistic</i>expialidocious</span>`+
			`</strong>`)
	lines := unpack(t, body, 2)
	assertApprox(t, "line2.children[0].width", inlineAt(t, lines[1], 0).Width, inlineAt(t, lines[0], 0).Width)
	assertApprox(t, "line1.children[0].width", inlineAt(t, lines[0], 0).Width, width2)
}

func TestSpacingEx(t *testing.T) {
	for _, spacing := range []string{"word-spacing", "letter-spacing"} {
		t.Run(spacing, func(t *testing.T) {
			// Regression test.
			inlineRender(t, fmt.Sprintf(`<div style="%s: 2ex">abc def`, spacing))
		})
	}
}

func TestTextIndent(t *testing.T) {
	for _, indent := range []string{"12px", "6%"} {
		t.Run(indent, func(t *testing.T) {
			body := inlineBody(t, `
        <style>
            @page { size: 220px }
            body { margin: 10px; text-indent: `+indent+` }
        </style>
        <p>Some text that is long enough that it take at least three line,
           but maybe more.
    `)
			lines := only(t, body).Children
			if len(lines) < 3 {
				t.Fatalf("%d lines", len(lines))
			}
			inlineEqual(t, "text_1.position_x", only(t, lines[0]).PositionX, 22) // 10px margin-left + 12px indent
			inlineEqual(t, "text_2.position_x", only(t, lines[1]).PositionX, 10) // No indent
			inlineEqual(t, "text_3.position_x", only(t, lines[2]).PositionX, 10) // No indent
		})
	}
}

func TestTextIndentInline(t *testing.T) {
	// Regression test for #1000.
	body := inlineBody(t, `
        <style>
            p { display: inline-block; text-indent: 1em; font-family: weasyprint }
        </style>
        <p><span>text
    `)
	line := only(t, only(t, body))
	inlineEqual(t, "line.width", line.Width, (4+1)*16)
}

func TestTextIndentMultipage(t *testing.T) {
	for _, indent := range []string{"12px", "6%"} {
		t.Run(indent, func(t *testing.T) {
			// Regression test for #706.
			pages := renderPages(t, `
        <style>
            @page { size: 220px 1.5em; margin: 0 }
            body { margin: 10px; text-indent: `+indent+` }
        </style>
        <p>Some text that is long enough that it take at least three line,
           but maybe more.
    `)
			if len(pages) < 2 {
				t.Fatalf("%d pages", len(pages))
			}
			_, body := pageBody(t, pages[0])
			text := only(t, only(t, only(t, body)))
			inlineEqual(t, "text.position_x", text.PositionX, 22) // 10px margin-left + 12px indent

			_, body = pageBody(t, pages[1])
			text = only(t, only(t, only(t, body)))
			inlineEqual(t, "text.position_x", text.PositionX, 10) // No indent
		})
	}
}

// inlineNoAutoHyphenation is the reason to skip the tests of hyphens:
// auto.
const inlineNoAutoHyphenation = "the port has no hyphenation for hyphens: auto (no Pyphen, D15)"

func TestHyphenateCharacter1(t *testing.T) { t.Skip(inlineNoAutoHyphenation) }
func TestHyphenateCharacter2(t *testing.T) { t.Skip(inlineNoAutoHyphenation) }
func TestHyphenateCharacter3(t *testing.T) { t.Skip(inlineNoAutoHyphenation) }
func TestHyphenateCharacter4(t *testing.T) { t.Skip(inlineNoAutoHyphenation) }
func TestHyphenateCharacter5(t *testing.T) { t.Skip(inlineNoAutoHyphenation) }

// inlineLineTexts returns the texts of the children of the lines.
func inlineLineTexts(lines []*Box) string {
	var b strings.Builder
	for _, line := range lines {
		for _, child := range line.Children {
			b.WriteString(child.Text)
		}
	}
	return b.String()
}

func TestHyphenateManual1(t *testing.T) {
	word := []rune("hyphénation")
	for i := 1; i < len(word); i++ {
		for _, hyphenateCharacter := range []string{"!", "ù ù"} {
			t.Run(fmt.Sprintf("%d %s", i, hyphenateCharacter), func(t *testing.T) {
				word := string(word[:i]) + "\u00ad" + string(word[i:])
				body := inlineBody(t,
					`<html style="width: 5em; font-family: weasyprint">`+
						`<body style="hyphens: manual;`+
						fmt.Sprintf(`  hyphenate-character: '%s'"`, hyphenateCharacter)+
						fmt.Sprintf(`  lang=fr>%s`, word))
				lines := unpack(t, body, 2)
				if !strings.HasSuffix(inlineAt(t, lines[0], 0).Text, hyphenateCharacter) {
					t.Errorf("line 1 is %q", inlineAt(t, lines[0], 0).Text)
				}
				if got := strings.ReplaceAll(inlineLineTexts(lines), hyphenateCharacter, ""); got != word {
					t.Errorf("text is %q, want %q", got, word)
				}
			})
		}
	}
}

func TestHyphenateManual2(t *testing.T) {
	word := []rune("hy phénation")
	for i := 1; i < len(word); i++ {
		for _, hyphenateCharacter := range []string{"!", "ù ù"} {
			t.Run(fmt.Sprintf("%d %s", i, hyphenateCharacter), func(t *testing.T) {
				word := string(word[:i]) + "\u00ad" + string(word[i:])
				body := inlineBody(t,
					`<html style="width: 5em; font-family: weasyprint">`+
						`<body style="hyphens: manual;`+
						fmt.Sprintf(`  hyphenate-character: '%s'"`, hyphenateCharacter)+
						fmt.Sprintf(`  lang=fr>%s`, word))
				lines := body.Children
				if len(lines) != 2 && len(lines) != 3 {
					t.Fatalf("%d lines", len(lines))
				}
				fullText := strings.ReplaceAll(inlineLineTexts(lines), hyphenateCharacter, "")
				first := inlineAt(t, lines[0], 0).Text
				if strings.HasSuffix(first, hyphenateCharacter) {
					if fullText != word {
						t.Errorf("text is %q, want %q", fullText, word)
					}
				} else {
					if !strings.HasSuffix(strings.TrimRight(first, "\u00ad"), "y") {
						t.Errorf("line 1 is %q", first)
					}
					if len(lines) == 3 {
						second := inlineAt(t, lines[1], 0).Text
						if !strings.HasSuffix(strings.TrimRight(second, "\u00ad"), hyphenateCharacter) {
							t.Errorf("line 2 is %q", second)
						}
					}
				}
			})
		}
	}
}

func TestHyphenateManual3(t *testing.T) {
	// Automatic hyphenation opportunities within a word must be ignored if
	// the word contains a conditional hyphen, in favor of the conditional
	// hyphen(s).
	body := inlineBody(t,
		`<html style="width: 0.1em" lang="en">`+
			`<body style="hyphens: auto">in&shy;lighten&shy;lighten&shy;in`)
	lines := unpack(t, body, 4)
	inlineText(t, inlineAt(t, lines[0], 0), "in\u00ad‐")
	inlineText(t, inlineAt(t, lines[1], 0), "lighten\u00ad‐")
	inlineText(t, inlineAt(t, lines[2], 0), "lighten\u00ad‐")
	inlineText(t, inlineAt(t, lines[3], 0), "in")
}

func TestHyphenateManual4(t *testing.T) {
	// Regression test for #1878.
	body := inlineBody(t,
		`<html style="width: 0.1em" lang="en">`+
			`<body style="hyphens: auto">test&shy;`)
	only(t, body)
	// TODO: should not end with an hyphen
	// assert line_1.children[0].text == 'test\xad'
}

func TestHyphenateManualKeepHyphen(t *testing.T) {
	// Regression test for #2614.
	body := inlineBody(t,
		`<html style="width: 6em; font-family: weasyprint">`+
			`<body style="hyphens: manual">super&shy;aa bb`)
	inlineText(t, inlineAt(t, body, 0, 0), "super\u00ad‐")
}

func TestHyphenateLimitZone1(t *testing.T) { t.Skip(inlineNoAutoHyphenation) }
func TestHyphenateLimitZone2(t *testing.T) { t.Skip(inlineNoAutoHyphenation) }
func TestHyphenateLimitZone3(t *testing.T) { t.Skip(inlineNoAutoHyphenation) }
func TestHyphenateLimitZone4(t *testing.T) { t.Skip(inlineNoAutoHyphenation) }

func TestHyphenNbsp(t *testing.T) { t.Skip(inlineNoAutoHyphenation) }

func TestHyphenateLimitChars(t *testing.T) {
	for _, css := range []string{
		"auto", "auto auto 0", "0 0 0", "4 4 auto", "6 2 4", "auto 1 auto",
		"7 auto auto", "6 auto auto", "5 2", "3", "2 4 6", "auto 4", "auto 2",
	} {
		t.Run(css, func(t *testing.T) { t.Skip(inlineNoAutoHyphenation) })
	}
}

func TestHyphenateLimitCharsPunctuation(t *testing.T) {
	for _, css := range []string{
		// light·en
		"3 3 3", // 'en' is shorter than 3
		"3 6 2", // 'light' is shorter than 6
		"8",     // 'lighten' is shorter than 8
	} {
		t.Run(css, func(t *testing.T) {
			// Regression test for #109.
			body := inlineBody(t,
				`<html style="width: 1em; font-family: weasyprint">`+
					`<body style="hyphens: auto;`+
					fmt.Sprintf(`hyphenate-limit-chars: %s" lang=en>`, css)+
					`..lighten..`)
			if len(body.Children) != 1 {
				t.Errorf("%d lines", len(body.Children))
			}
		})
	}
}

func TestLayoutOverflowWrap(t *testing.T) {
	for _, c := range []struct {
		wrap, text string
		test       func(int) bool
		fullText   string
		skip       string
	}{
		{"anywhere", "aaaaaaaa", func(a int) bool { return a > 1 }, "aaaaaaaa", ""},
		{"break-word", "aaaaaaaa", func(a int) bool { return a > 1 }, "aaaaaaaa", ""},
		{"normal", "aaaaaaaa", func(a int) bool { return a == 1 }, "aaaaaaaa", ""},
		{"break-word", "hyphenations", func(a int) bool { return a > 3 },
			"hy‐phen‐ations", inlineNoAutoHyphenation},
		{"break-word", "A splitted word.  An hyphenated word.", func(a int) bool { return a > 8 },
			"Asplittedword.Anhy‐phen‐atedword.", inlineNoAutoHyphenation},
	} {
		t.Run(c.wrap+" "+c.text, func(t *testing.T) {
			if c.skip != "" {
				t.Skip(c.skip)
			}
			body := inlineBody(t, fmt.Sprintf(`
      <style>
        body {width: 80px; overflow: hidden; font-family: weasyprint}
        span {overflow-wrap: %s}
      </style>
      <body style="hyphens: auto;" lang="en">
        <span>%s
    `, c.wrap, c.text))
			var lines []string
			for _, line := range body.Children {
				lines = append(lines, only(t, only(t, line)).Text)
			}
			if !c.test(len(lines)) {
				t.Errorf("%d lines: %q", len(lines), lines)
			}
			if got := strings.Join(lines, ""); got != c.fullText {
				t.Errorf("text is %q, want %q", got, c.fullText)
			}
		})
	}
}

func TestWrapOverflowWordBreak(t *testing.T) {
	for _, c := range []struct {
		spanCSS       string
		expectedLines []string
	}{
		// overflow-wrap: anywhere and break-word are only allowed to break
		// a word "if there are no otherwise-acceptable break points in the
		// line", which means they should not split a word if it fits
		// cleanly into the next line. This can be done accidentally if it
		// is in its own inline element.
		{"overflow-wrap: anywhere", []string{"aaa", "bbb"}},
		{"overflow-wrap: break-word", []string{"aaa", "bbb"}},

		// On the other hand, word-break: break-all mandates a break
		// anywhere at the end of a line, even if the word could fit
		// cleanly onto the next line.
		{"word-break: break-all", []string{"aaa b", "bb"}},
	} {
		t.Run(c.spanCSS, func(t *testing.T) {
			body := inlineBody(t, fmt.Sprintf(`
      <style>
        body {width: 80px; overflow: hidden; font-family: weasyprint}
        span {%s}
      </style>
      <body>
        <span>aaa </span><span>bbb
    `, c.spanCSS))
			var lines []string
			for _, line := range body.Children {
				lineText := ""
				for _, spanBox := range line.Children {
					lineText += inlineAt(t, spanBox, 0).Text
				}
				lines = append(lines, lineText)
			}
			if strings.Join(lines, "|") != strings.Join(c.expectedLines, "|") {
				t.Errorf("lines are %q, want %q", lines, c.expectedLines)
			}
		})
	}
}

// inlineTableCellWidth returns the width of the cell of a table of one
// cell, for test_overflow_wrap_2 and test_overflow_wrap_trailing_space.
func inlineTableCellWidth(t *testing.T, html string) float64 {
	t.Helper()
	body := inlineBody(t, html)
	tableWrapper := only(t, body)
	table := only(t, tableWrapper)
	rowGroup := only(t, table)
	tr := only(t, rowGroup)
	return only(t, tr).Width
}

func TestOverflowWrap2(t *testing.T) {
	for _, c := range []struct {
		wrap, text               string
		bodyWidth, expectedWidth float64
	}{
		{"anywhere", "aaaaaa", 10, 20},
		{"anywhere", "aaaaaa", 40, 40},
		{"break-word", "aaaaaa", 40, 120},
		{"normal", "aaaaaa", 40, 120},
	} {
		t.Run(fmt.Sprint(c.wrap, c.bodyWidth), func(t *testing.T) {
			width := inlineTableCellWidth(t, fmt.Sprintf(`
      <style>
        body {width: %vpx; font-family: weasyprint; font-size: 20px}
        table {overflow-wrap: %s}
      </style>
      <table><tr><td>%s`, c.bodyWidth, c.wrap, c.text))
			inlineEqual(t, "td.width", width, c.expectedWidth)
		})
	}
}

func TestOverflowWrapTrailingSpace(t *testing.T) {
	for _, c := range []struct {
		wrap, text               string
		bodyWidth, expectedWidth float64
	}{
		{"anywhere", "aaaaaa", 10, 20},
		{"anywhere", "aaaaaa", 40, 40},
		{"break-word", "aaaaaa", 40, 120},
		{"normal", "abcdef", 40, 120},
	} {
		t.Run(fmt.Sprint(c.wrap, c.bodyWidth), func(t *testing.T) {
			width := inlineTableCellWidth(t, fmt.Sprintf(`
      <style>
        body {width: %vpx; font-family: weasyprint; font-size: 20px}
        table {overflow-wrap: %s}
      </style>
      <table><tr><td>%s `, c.bodyWidth, c.wrap, c.text))
			inlineEqual(t, "td.width", width, c.expectedWidth)
		})
	}
}

func TestOverflowWrapNoBreakOnSpace(t *testing.T) {
	// Regression test for #1817.
	body := inlineBody(t, `
      <style>
        body {width: 11px; font-family: weasyprint; font-size: 2px;
              overflow-wrap: anywhere}
      </style>.jpg, .png`)
	lines := unpack(t, body, 2)
	inlineText(t, only(t, lines[0]), ".jpg,")
	inlineText(t, only(t, lines[1]), ".png")
}

func TestLineBreakBeforeTrailingSpace(t *testing.T) {
	// Regression test for #1852.
	body := inlineBody(t, "\n"+
		"        <p style=\"display: inline-block\">test  </p>a\n"+
		"        <p style=\"display: inline-block\">test </p>a\n    ")
	children := unpack(t, only(t, body), 4)
	inlineEqual(t, "p1.width", children[0].Width, children[2].Width)
}

func TestLineBreakBeforeNestedTrailingSpace(t *testing.T) {
	// Regression test for #2448.
	body := inlineBody(t, `
    <div style="font: 2px weasyprint; width: 6px">
      AA <span><b><i>BBB</i></b> </span></div>
    `)
	div := only(t, body)
	if len(div.Children) != 2 {
		t.Errorf("div has %d lines", len(div.Children))
	}
}

// inlineWhiteSpaceLines is white_space_lines in tests/test_text.py.
func inlineWhiteSpaceLines(t *testing.T, width int, space string) []*Box {
	t.Helper()
	body := inlineBody(t, fmt.Sprintf(`
      <style>
        body { font-size: 100px; width: %dpx }
        span { white-space: %s }
      </style>
      <body><span>This +    `+"\n"+`    is text`, width, space))
	return body.Children
}

// inlineCheckWhiteSpace checks the texts of the lines of
// inlineWhiteSpaceLines.
func inlineCheckWhiteSpace(t *testing.T, width int, space string, texts ...string) {
	t.Helper()
	lines := inlineWhiteSpaceLines(t, width, space)
	if len(lines) != len(texts) {
		t.Fatalf("%d lines, want %d", len(lines), len(texts))
	}
	for i, line := range lines {
		inlineText(t, only(t, only(t, line)), texts[i])
	}
}

func TestWhiteSpace1(t *testing.T) {
	inlineCheckWhiteSpace(t, 1, "normal", "This", "+", "is", "text")
}

func TestWhiteSpace2(t *testing.T) {
	inlineCheckWhiteSpace(t, 1, "pre", "This +    ", "    is text")
}

func TestWhiteSpace3(t *testing.T) {
	inlineCheckWhiteSpace(t, 1, "nowrap", "This + is text")
}

func TestWhiteSpace4(t *testing.T) {
	inlineCheckWhiteSpace(t, 1, "pre-wrap", "This ", "+    ", "    ", "is ", "text")
}

func TestWhiteSpace5(t *testing.T) {
	inlineCheckWhiteSpace(t, 1, "pre-line", "This", "+", "is", "text")
}

func TestWhiteSpace6(t *testing.T) {
	inlineCheckWhiteSpace(t, 1000000, "normal", "This + is text")
}

func TestWhiteSpace7(t *testing.T) {
	inlineCheckWhiteSpace(t, 1000000, "pre", "This +    ", "    is text")
}

func TestWhiteSpace8(t *testing.T) {
	inlineCheckWhiteSpace(t, 1000000, "nowrap", "This + is text")
}

func TestWhiteSpace9(t *testing.T) {
	inlineCheckWhiteSpace(t, 1000000, "pre-wrap", "This +    ", "    is text")
}

func TestWhiteSpace10(t *testing.T) {
	inlineCheckWhiteSpace(t, 1000000, "pre-line", "This +", "is text")
}

func TestWhiteSpace11(t *testing.T) {
	// Regression test for #813.
	body := inlineBody(t, `
      <style>
        pre { width: 0 }
      </style>
      <body><pre>This<br/>is text`)
	lines := unpack(t, only(t, body), 2)
	children := unpack(t, lines[0], 2)
	inlineText(t, children[0], "This")
	if children[1].ElementTag != "br" {
		t.Errorf("element tag %q", children[1].ElementTag)
	}
	inlineText(t, only(t, lines[1]), "is text")
}

func TestWhiteSpace12(t *testing.T) {
	// Regression test for #813.
	body := inlineBody(t, `
      <style>
        pre { width: 0 }
      </style>
      <body><pre>This is <span>lol</span> text`)
	line := only(t, only(t, body))
	children := unpack(t, line, 3)
	inlineText(t, children[0], "This is ")
	if children[1].ElementTag != "span" {
		t.Errorf("element tag %q", children[1].ElementTag)
	}
	inlineText(t, children[2], " text")
}

func TestTabSize(t *testing.T) {
	for _, c := range []struct {
		value string
		width float64
	}{
		{"8", 144},   // (2 + (8 - 1)) * 16
		{"4", 80},    // (2 + (4 - 1)) * 16
		{"3em", 64},  // (2 + (3 - 1)) * 16
		{"25px", 41}, // 2 * 16 + 25 - 1 * 16
		// (0, 32),  # See Layout.set_tabs
	} {
		t.Run(c.value, func(t *testing.T) {
			body := inlineBody(t, `
      <style>
        pre { tab-size: `+c.value+`; font-family: weasyprint }
      </style>
      <pre>a&#9;a</pre>
    `)
			line := only(t, only(t, body))
			inlineEqual(t, "line.width", line.Width, c.width)
		})
	}
}

func TestTextFloatingPreLine(t *testing.T) {
	// Regression test for #610.
	inlineRender(t, `
      <div style="float: left; white-space: pre-line">This is
      oh this end </div>
    `)
}

func TestLeaderContent(t *testing.T) {
	for _, c := range []struct {
		leader, content string
	}{
		{"dotted", "."},
		{"solid", "_"},
		{"space", " "},
		{`" .-"`, " .-"},
	} {
		t.Run(c.leader, func(t *testing.T) {
			body := inlineBody(t, `
      <style>div::after { content: leader(`+c.leader+`) }</style>
      <div></div>
    `)
			line := only(t, only(t, body))
			after := only(t, line)
			inline := only(t, after)
			inlineText(t, inlineAt(t, inline, 0), c.content)
		})
	}
}

func TestMaxLines(t *testing.T) {
	t.Skip("upstream marks test_max_lines as an expected failure")
}

func TestContinue(t *testing.T) {
	body := inlineBody(t, `
      <style>
        @page {size: 10px 4px;}
        div {
          continue: discard;
          font-family: weasyprint;
          font-size: 2px;
        }
      </style>
      <div>
        abcd efgh ijkl
      </div>
    `)
	lines := unpack(t, only(t, body), 2)
	inlineText(t, only(t, lines[0]), "abcd")
	inlineText(t, only(t, lines[1]), "efgh")
}

func TestFirstLetterLineHeight(t *testing.T) {
	body := inlineBody(t, `
      <style>
        p { font-family: weasyprint }
        p:first-letter { line-height: 1; font-size: 20px }
      </style>
      <p>abc
    `)
	line := only(t, only(t, body))
	firstLetter := unpack(t, line, 2)[0]
	firstLetterText := only(t, firstLetter)
	inlineText(t, firstLetterText, "a")
	inlineEqual(t, "first_letter_text.height", firstLetterText.Height, 20)
}

func TestFirstLetterTextTransform(t *testing.T) {
	// Regression test for #1906.
	body := inlineBody(t, `
      <style>
        p::first-letter { text-transform: uppercase }
      </style>
      <p>abc
    `)
	line := only(t, only(t, body))
	children := unpack(t, line, 2)
	inlineText(t, only(t, children[0]), "A")
	inlineText(t, children[1], "bc")
}

// inlineCheckFont checks the font style and the font weight of a box.
func inlineCheckFont(t *testing.T, name string, box *Box, fontStyle string, fontWeight float64) {
	t.Helper()
	if got := box.Style.Get("font_style"); !isStr(got, fontStyle) {
		t.Errorf("%s font_style = %v, want %s", name, got, fontStyle)
	}
	if got := num(box.Style.Get("font_weight")); got != fontWeight {
		t.Errorf("%s font_weight = %v, want %v", name, got, fontWeight)
	}
}

func TestFirstLetterNested(t *testing.T) {
	body := inlineBody(t, `
      <style>
        body::first-letter { font-style: italic }
        p::first-letter { font-weight: 700 }
      </style>
      <p>abc</p>
      <p>abc</p>
    `)
	ps := unpack(t, body, 2)
	children := unpack(t, only(t, ps[0]), 2)
	inlineCheckFont(t, "first_letter_text", only(t, children[0]), "italic", 700)
	inlineCheckFont(t, "next_text", children[1], "normal", 400)
	children = unpack(t, only(t, ps[1]), 2)
	inlineCheckFont(t, "first_letter_text", only(t, children[0]), "normal", 700)
	inlineCheckFont(t, "next_text", children[1], "normal", 400)
}

func TestFirstLineLineHeight(t *testing.T) {
	body := inlineBody(t, `
      <style>
        p { font-family: weasyprint }
        p:first-line { line-height: 1; font-size: 20px }
      </style>
      <p>abc<br>def
    `)
	lines := unpack(t, only(t, body), 2)
	firstLineBox := only(t, lines[0])
	firstLineText := unpack(t, firstLineBox, 2)[0]
	inlineText(t, firstLineText, "abc")
	inlineEqual(t, "first_line_text.height", firstLineText.Height, 20)
}

func TestFirstLineTextTransform(t *testing.T) {
	body := inlineBody(t, `
      <style>
        p::first-line { text-transform: uppercase }
      </style>
      <p>abc<br>def
    `)
	lines := unpack(t, only(t, body), 2)
	firstLineBox := only(t, lines[0])
	inlineText(t, unpack(t, firstLineBox, 2)[0], "ABC")
	inlineText(t, only(t, lines[1]), "def")
}

func TestFirstLineAcrossPages(t *testing.T) {
	pages := renderPages(t, `
      <style>
        @page { size: 10px 10px }
        p { font-family: weasyprint; font-size: 2px; line-height: 1 }
        p::first-line { font-weight: bold }
      </style>
      <p>aa bb cc dd ee ff gg hh ii jj kk ll mm nn oo pp</p>
    `)
	if len(pages) != 2 {
		t.Fatalf("%d pages", len(pages))
	}
	_, body1 := pageBody(t, pages[0])
	paragraph1 := inlineAt(t, body1, 0)
	firstLineBox := only(t, inlineAt(t, paragraph1, 0))
	if firstLineBox.ElementTag != "p::first-line" {
		t.Errorf("element tag %q", firstLineBox.ElementTag)
	}
	_, body2 := pageBody(t, pages[1])
	paragraph2 := inlineAt(t, body2, 0)
	lineBox := only(t, inlineAt(t, paragraph2, 0))
	if lineBox.ElementTag != "p" {
		t.Errorf("element tag %q", lineBox.ElementTag)
	}
}

func TestFirstLineNested(t *testing.T) {
	body := inlineBody(t, `
      <style>
        body::first-line { font-style: italic }
        p::first-line { font-weight: 700 }
      </style>
      <p>abc<br>def</p>
      <p>abc<br>def</p>
    `)
	ps := unpack(t, body, 2)
	for i, fontStyle := range []string{"italic", "normal"} {
		lines := unpack(t, ps[i], 2)
		firstLineBox := only(t, lines[0])
		inlineCheckFont(t, "first_line_text", unpack(t, firstLineBox, 2)[0], fontStyle, 700)
		inlineCheckFont(t, "next_line_text", only(t, lines[1]), "normal", 400)
	}
}

func TestFirstLineFirstLetter(t *testing.T) {
	body := inlineBody(t, `
      <style>
        p::first-letter { font-style: italic }
        p::first-line { font-weight: 700 }
      </style>
      <p>abc<br>def</p>
    `)
	lines := unpack(t, only(t, body), 2)
	firstLineBox := only(t, lines[0])
	children := unpack(t, firstLineBox, 3)
	inlineCheckFont(t, "first_letter_text", only(t, children[0]), "italic", 700)
	inlineCheckFont(t, "first_line_text", children[1], "normal", 700)
	inlineCheckFont(t, "next_line_text", only(t, lines[1]), "normal", 400)
}
