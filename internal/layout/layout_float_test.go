// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_float.py: the tests of the layout of
// the floating boxes.

package layout

import (
	"fmt"
	"strings"
	"testing"
)

// blockOuterArea is outer_area in tests/layout/test_float.py: the (x, y,
// w, h) rectangle of the outer area of a box.
func blockOuterArea(box *Box) [4]float64 {
	return [4]float64{box.PositionX, box.PositionY, box.MarginWidth(), box.MarginHeight()}
}

// blockAssertArea checks the outer area of a box.
func blockAssertArea(t *testing.T, name string, box *Box, want [4]float64) {
	t.Helper()
	if got := blockOuterArea(box); got != want {
		t.Errorf("outer_area(%s) = %v, want %v", name, got, want)
	}
}

// blockAssertText checks the text of a text box.
func blockAssertText(t *testing.T, name string, box *Box, want string) {
	t.Helper()
	if box.Text != want {
		t.Errorf("%s.text = %q, want %q", name, box.Text, want)
	}
}

// blockAssertStripped checks the text of a text box without the spaces
// around it, text.strip() of upstream.
func blockAssertStripped(t *testing.T, name string, box *Box, want string) {
	t.Helper()
	if got := strings.TrimSpace(box.Text); got != want {
		t.Errorf("%s.text.strip() = %q, want %q", name, got, want)
	}
}

func TestFloats1(t *testing.T) {
	// adjacent-floats-001
	body := blockBody(t, `
      <style>
        div { float: left }
        img { width: 100px; vertical-align: top }
      </style>
      <div><img src=pattern.png /></div>
      <div><img src=pattern.png /></div>`)
	divs := unpack(t, body, 2)
	blockAssertArea(t, "div_1", divs[0], [4]float64{0, 0, 100, 100})
	blockAssertArea(t, "div_2", divs[1], [4]float64{100, 0, 100, 100})
}

func TestFloats2(t *testing.T) {
	// c414-flt-fit-000
	body := blockBody(t, `
      <style>
        body { width: 290px }
        div { float: left; width: 100px;  }
        img { width: 60px; vertical-align: top }
      </style>
      <div><img src=pattern.png /><!-- 1 --></div>
      <div><img src=pattern.png /><!-- 2 --></div>
      <div><img src=pattern.png /><!-- 4 --></div>
      <img src=pattern.png /><!-- 3
      --><img src=pattern.png /><!-- 5 -->`)
	children := unpack(t, body, 4)
	lines := unpack(t, children[3], 2)
	img3, img5 := only(t, lines[0]), only(t, lines[1])
	blockAssertArea(t, "div_1", children[0], [4]float64{0, 0, 100, 60})
	blockAssertArea(t, "div_2", children[1], [4]float64{100, 0, 100, 60})
	blockAssertArea(t, "img_3", img3, [4]float64{200, 0, 60, 60})
	blockAssertArea(t, "div_4", children[2], [4]float64{0, 60, 100, 60})
	blockAssertArea(t, "img_5", img5, [4]float64{100, 60, 60, 60})
}

func TestFloats3(t *testing.T) {
	// c414-flt-fit-002
	body := blockBody(t, `
      <style type="text/css">
        body { width: 200px }
        p { width: 70px; height: 20px }
        .left { float: left }
        .right { float: right }
      </style>
      <p class="left"> ⇦ A 1 </p>
      <p class="left"> ⇦ B 2 </p>
      <p class="left"> ⇦ A 3 </p>
      <p class="right"> B 4 ⇨ </p>
      <p class="left"> ⇦ A 5 </p>
      <p class="right"> B 6 ⇨ </p>
      <p class="right"> B 8 ⇨ </p>
      <p class="left"> ⇦ A 7 </p>
      <p class="left"> ⇦ A 9 </p>
      <p class="left"> ⇦ B 10 </p>
    `)
	positions := make([][2]float64, 0, len(body.Children))
	for _, paragraph := range body.Children {
		positions = append(positions, [2]float64{paragraph.PositionX, paragraph.PositionY})
	}
	want := [][2]float64{
		{0, 0}, {70, 0}, {0, 20}, {130, 20}, {0, 40}, {130, 40},
		{130, 60}, {0, 60}, {0, 80}, {70, 80},
	}
	if fmt.Sprint(positions) != fmt.Sprint(want) {
		t.Errorf("positions = %v, want %v", positions, want)
	}
}

// blockFloatsWrap checks the lines of test_floats_4 and test_floats_5.
func blockFloatsWrap(t *testing.T, body *Box) {
	t.Helper()
	children := unpack(t, body, 3)
	anonBlock := children[2]
	lines := unpack(t, anonBlock, 2)
	assertApprox(t, "anon_block.position_y", anonBlock.PositionY, 0)
	blockFields(t, "line_1", lines[0], map[string]float64{"position_x": 20, "position_y": 0})
	blockFields(t, "line_2", lines[1], map[string]float64{"position_x": 0, "position_y": 200})
}

func TestFloats4(t *testing.T) {
	// c414-flt-wrap-000 ... more or less
	blockFloatsWrap(t, blockBody(t, `
      <style>
        body { width: 100px }
        p { float: left; height: 100px }
        img { width: 60px; vertical-align: top }
      </style>
      <p style="width: 20px"></p>
      <p style="width: 100%"></p>
      <img src=pattern.png /><img src=pattern.png />
    `))
}

func TestFloats5(t *testing.T) {
	// c414-flt-wrap-000 with text ... more or less
	blockFloatsWrap(t, blockBody(t, `
      <style>
        body { width: 100px; font: 60px weasyprint; }
        p { float: left; height: 100px }
        img { width: 60px; vertical-align: top }
      </style>
      <p style="width: 20px"></p>
      <p style="width: 100%"></p>
      A B
    `))
}

func TestFloats6(t *testing.T) {
	// floats-placement-vertical-001b
	body := blockBody(t, `
      <style>
        body { width: 90px; font-size: 0 }
        img { vertical-align: top }
      </style>
      <body>
      <span>
        <img src=pattern.png style="width: 50px" />
        <img src=pattern.png style="width: 50px" />
        <img src=pattern.png style="float: left; width: 30px" />
      </span>
    `)
	lines := unpack(t, body, 2)
	img1 := only(t, only(t, lines[0]))
	imgs := unpack(t, only(t, lines[1]), 2)
	blockAssertArea(t, "img_1", img1, [4]float64{0, 0, 50, 50})
	blockAssertArea(t, "img_2", imgs[0], [4]float64{30, 50, 50, 50})
	blockAssertArea(t, "img_3", imgs[1], [4]float64{0, 50, 30, 30})
}

func TestFloats7(t *testing.T) {
	// Variant of the above: no <span>
	body := blockBody(t, `
      <style>
        body { width: 90px; font-size: 0 }
        img { vertical-align: top }
      </style>
      <body>
      <img src=pattern.png style="width: 50px" />
      <img src=pattern.png style="width: 50px" />
      <img src=pattern.png style="float: left; width: 30px" />
    `)
	lines := unpack(t, body, 2)
	img1 := only(t, lines[0])
	imgs := unpack(t, lines[1], 2)
	blockAssertArea(t, "img_1", img1, [4]float64{0, 0, 50, 50})
	blockAssertArea(t, "img_2", imgs[0], [4]float64{30, 50, 50, 50})
	blockAssertArea(t, "img_3", imgs[1], [4]float64{0, 50, 30, 30})
}

func TestFloats8(t *testing.T) {
	// Floats do no affect other pages
	pages := blockPagesN(t, `
      <style>
        body { width: 90px; font-size: 0 }
        img { vertical-align: top }
      </style>
      <body>
      <img src=pattern.png style="float: left; width: 30px" />
      <img src=pattern.png style="width: 50px" />
      <div style="page-break-before: always"></div>
      <img src=pattern.png style="width: 50px" />
    `, 2)
	_, body := pageBody(t, pages[0])
	children := unpack(t, body, 2)
	img1 := only(t, only(t, children[1]))
	blockAssertArea(t, "float_img", children[0], [4]float64{0, 0, 30, 30})
	blockAssertArea(t, "img_1", img1, [4]float64{30, 0, 50, 50})

	_, body = pageBody(t, pages[1])
	children = unpack(t, body, 2)
	only(t, only(t, children[1]))
}

func TestFloats9(t *testing.T) {
	// Regression test for #263.
	blockPage(t, `<div style="top:100%; float:left">`)
}

// blockFloatsPageBreaks checks the images of the tests of the page breaks
// of the floats: their horizontal position, and their vertical positions
// on each page.
func blockFloatsPageBreaks(t *testing.T, height string, images int, want [][]float64) {
	t.Helper()
	pages := blockPagesN(t, fmt.Sprintf(`
      <style>
        @page { size: 100px; margin: 10px }
        img { height: %s; width:70px; float: left;}
      </style>
      <body>
        %s
    `, height, strings.Repeat("<img src=pattern.png>\n", images)), len(want))
	positions := make([][]float64, 0, len(pages))
	for _, page := range pages {
		var y []float64
		for _, img := range blockDescendants(page, "img") {
			assertApprox(t, "img.position_x", img.PositionX, 10)
			y = append(y, img.PositionY)
		}
		positions = append(positions, y)
	}
	if fmt.Sprint(positions) != fmt.Sprint(want) {
		t.Errorf("positions_y = %v, want %v", positions, want)
	}
}

func TestFloatsPageBreaks1(t *testing.T) {
	// Tests floated images shorter than the page
	blockFloatsPageBreaks(t, "45px", 2, [][]float64{{10}, {10}})
}

func TestFloatsPageBreaks2(t *testing.T) {
	// Tests floated images taller than the page
	blockFloatsPageBreaks(t, "81px", 2, [][]float64{{10}, {10}})
}

func TestFloatsPageBreaks3(t *testing.T) {
	// Tests floated images shorter than the page
	blockFloatsPageBreaks(t, "30px", 5, [][]float64{{10, 40}, {10, 40}, {10}})
}

func TestFloatOverflowSplitsFormattingContext(t *testing.T) {
	// A deferred (overflowing) float must not inflate its block formatting
	// context on the current page, or the whole block is pushed to the next
	// page instead of being split across the page break.
	pages := blockPagesN(t, `
      <style>@page { size: 100px }</style>
      <div style="height: 70px"></div>
      <div style="display: flow-root">
        <p style="height: 20px">a</p>
        <div style="float: right; width: 30px; height: 40px"></div>
        <p>b</p>
      </div>
    `, 2)

	// Page 1 keeps the spacer and the first paragraph: the block is split,
	// not deferred whole.
	_, body := pageBody(t, pages[0])
	children := unpack(t, body, 2)
	assertApprox(t, "spacer.height", children[0].Height, 70)
	paragraphA := only(t, children[1])
	if paragraphA.ElementTag != "p" {
		t.Errorf("paragraph_a is %v", paragraphA)
	}
	assertApprox(t, "paragraph_a.position_y", paragraphA.PositionY, 70)

	// Page 2 resumes the block with the deferred float and the last
	// paragraph.
	_, body = pageBody(t, pages[1])
	children = unpack(t, only(t, body), 2)
	floatBox, paragraphB := children[0], children[1]
	if !floatBox.IsFloated() {
		t.Errorf("float_box is %v", floatBox)
	}
	assertApprox(t, "float_box.position_y", floatBox.PositionY, 0)
	if paragraphB.ElementTag != "p" {
		t.Errorf("paragraph_b is %v", paragraphB)
	}
	assertApprox(t, "paragraph_b.position_y", paragraphB.PositionY, 0)
}

func TestFloatPreferredWidths1(t *testing.T) {
	getFloatWidth := func(bodyWidth int) float64 {
		body := blockBody(t, fmt.Sprintf(`
          <body style="width: %dpx; font-family: weasyprint">
          <p style="white-space: pre-line; float: left">
            Lorem ipsum dolor sit amet,
              consectetur elit
          </p>
                   <!--  ^  No-break space here  -->
        `, bodyWidth))
		return only(t, body).Width
	}
	// Preferred minimum width:
	assertApprox(t, "width", getFloatWidth(10), float64(len([]rune("consectetur\u00a0elit"))*16))
	// Preferred width:
	assertApprox(t, "width", getFloatWidth(1000000), float64(len("Lorem ipsum dolor sit amet,")*16))
}

func TestFloatPreferredWidths2(t *testing.T) {
	// Non-regression test:
	// Incorrect whitespace handling in preferred width used to cause
	// unnecessary line break.
	body := blockBody(t, `
      <p style="float: left">Lorem <em>ipsum</em> dolor.</p>
    `)
	if line := only(t, only(t, body)); line.Kind != KindLine {
		t.Errorf("child is %v", line)
	}
}

func TestFloatPreferredWidths3(t *testing.T) {
	body := blockBody(t, `
      <style>img { width: 20px }</style>
      <p style="float: left">
        <img src=pattern.png><img src=pattern.png><br>
        <img src=pattern.png></p>
    `)
	assertApprox(t, "paragraph.width", only(t, body).Width, 40)
}

func TestFloatPreferredWidths4(t *testing.T) {
	body := blockBody(t, `<style>  p { font: 20px weasyprint }</style>`+
		`<p style="float: left">XX<br>XX<br>X</p>`)
	assertApprox(t, "paragraph.width", only(t, body).Width, 40)
}

func TestFloatPreferredWidths5(t *testing.T) {
	// The space is the start of the line is collapsed.
	body := blockBody(t, `<style>  p { font: 20px weasyprint }</style>`+
		`<p style="float: left">XX<br> XX<br>X</p>`)
	assertApprox(t, "paragraph.width", only(t, body).Width, 40)
}

func TestFloatInInline1(t *testing.T) {
	body := blockBody(t, `
      <style>
        body {
          font-family: weasyprint;
          font-size: 20px;
        }
        p {
          width: 14em;
          text-align: justify;
        }
        span {
          float: right;
        }
      </style>
      <p>
        aa bb <a><span>cc</span> ddd</a> ee ff
      </p>
    `)
	lines := unpack(t, only(t, body), 2)
	children := unpack(t, lines[0], 3)
	p1, a, p2 := children[0], children[1], children[2]
	assertApprox(t, "p1.width", p1.Width, 6*20)
	blockAssertText(t, "p1", p1, "aa bb ")
	assertApprox(t, "p1.position_x", p1.PositionX, 0*20)
	assertApprox(t, "p2.width", p2.Width, 3*20)
	blockAssertText(t, "p2", p2, " ee")
	assertApprox(t, "p2.position_x", p2.PositionX, 9*20)
	aChildren := unpack(t, a, 2)
	span, aText := aChildren[0], aChildren[1]
	assertApprox(t, "a_text.width", aText.Width, 3*20) // leading space collapse
	blockAssertText(t, "a_text", aText, "ddd")
	assertApprox(t, "a_text.position_x", aText.PositionX, 6*20)
	assertApprox(t, "span.width", span.Width, 2*20)
	blockAssertText(t, "span text", span.Children[0].Children[0], "cc")
	assertApprox(t, "span.position_x", span.PositionX, 12*20)

	assertApprox(t, "p3.width", only(t, lines[1]).Width, 2*20)
}

// blockFloatInInlineSource is the document of the tests test_float_in_inline
// 2 to 4, with the content of the span.
const blockFloatInInlineSource = `
      <style>
        @page {
          size: 10em;
        }
        article {
          font-family: weasyprint;
          line-height: 1;
        }
        div {
          float: left;
          width: 50%%;
        }
      </style>
      <article>
        <span>
          %s
        </span>
      </article>`

func TestFloatInInline2(t *testing.T) {
	body := blockBody(t, fmt.Sprintf(blockFloatInInlineSource, `<div>a b c</div>
          1 2 3 4 5 6`))
	lines := unpack(t, only(t, body), 2)
	children := unpack(t, only(t, lines[0]), 2)
	blockAssertStripped(t, "div", children[0].Children[0].Children[0], "a b c")
	blockAssertStripped(t, "text", children[1], "1 2 3")
	blockAssertStripped(t, "text", only(t, only(t, lines[1])), "4 5 6")
}

func TestFloatInInline3(t *testing.T) {
	body := blockBody(t, fmt.Sprintf(blockFloatInInlineSource, `1 2 3 <div>a b c</div> 4 5 6`))
	lines := unpack(t, only(t, body), 2)
	children := unpack(t, only(t, lines[0]), 2)
	blockAssertStripped(t, "text", children[0], "1 2 3")
	blockAssertStripped(t, "div", children[1].Children[0].Children[0], "a b c")
	blockAssertStripped(t, "text", only(t, only(t, lines[1])), "4 5 6")
}

func TestFloatInInline4(t *testing.T) {
	body := blockBody(t, fmt.Sprintf(blockFloatInInlineSource, `1 2 3 4 <div>a b c</div> 5 6`))
	lines := unpack(t, only(t, body), 2)
	children := unpack(t, lines[0], 2)
	span1, div := children[0], children[1]
	texts := unpack(t, span1, 2)
	blockAssertStripped(t, "text1", texts[0], "1 2 3 4")
	blockAssertStripped(t, "text2", texts[1], "5")
	assertApprox(t, "div.position_y", div.PositionY, 16)
	blockAssertStripped(t, "div", div.Children[0].Children[0], "a b c")
	blockAssertStripped(t, "text", only(t, only(t, lines[1])), "6")
}

func TestFloatNextLine(t *testing.T) {
	body := blockBody(t, `
      <style>
        body {
          font-family: weasyprint;
          font-size: 20px;
        }
        p {
          text-align: justify;
          width: 13em;
        }
        span {
          float: left;
        }
      </style>
      <p>pp pp pp pp <a><span>ppppp</span> aa</a> pp pp pp pp pp</p>`)
	lines := unpack(t, only(t, body), 3)
	unpack(t, lines[0], 1)
	unpack(t, lines[2], 1)
	children := unpack(t, lines[1], 2)
	a, p := children[0], children[1]
	aChildren := unpack(t, a, 2)
	span, aText := aChildren[0], aChildren[1]
	assertApprox(t, "span.position_x", span.PositionX, 0)
	assertApprox(t, "span.width", span.Width, 5*20)
	assertApprox(t, "a_text.position_x", aText.PositionX, 5*20)
	assertApprox(t, "a.position_x", a.PositionX, 5*20)
	assertApprox(t, "a_text.width", aText.Width, 2*20)
	assertApprox(t, "a.width", a.Width, 2*20)
	assertApprox(t, "p.position_x", p.PositionX, 7*20)
}

// blockFloatTextIndentSource is the document of the tests
// test_float_text_indent, with the float of the span and the content of
// the paragraph.
const blockFloatTextIndentSource = `
      <style>
        body {
          font-family: weasyprint;
          font-size: 20px;
        }
        p {
          text-align: justify;
          text-indent: 1em;
          width: 14em;
        }
        span {
          float: %s;
        }
      </style>
      <p>%s</p>`

// blockFloatTextIndentSpan checks the line of the anchor of the tests
// test_float_text_indent, and returns its three children.
func blockFloatTextIndentSpan(t *testing.T, line *Box, spanX float64) (a1, span, a2 *Box) {
	t.Helper()
	children := unpack(t, only(t, line), 3)
	a1, span, a2 = children[0], children[1], children[2]
	spanText := only(t, span)
	assertApprox(t, "span.position_x", span.PositionX, spanX)
	assertApprox(t, "span_text.position_x", spanText.PositionX, spanX)
	assertApprox(t, "span.width", span.Width, (1+5)*20) // text-indent + span text
	assertApprox(t, "span_text.width", spanText.Width, (1+5)*20)
	return a1, span, a2
}

func TestFloatTextIndent1(t *testing.T) {
	body := blockBody(t, fmt.Sprintf(blockFloatTextIndentSource, "left", `<a>aa <span>float</span> aa</a>`))
	line := only(t, only(t, body))
	a1, _, a2 := blockFloatTextIndentSpan(t, line, 0)
	assertApprox(t, "a1.width", a1.Width, 3*20)
	assertApprox(t, "a1.position_x", a1.PositionX, (1+5+1)*20)   // span + a1 text-indent
	assertApprox(t, "a2.width", a2.Width, 2*20)                  // leading space collapse
	assertApprox(t, "a2.position_x", a2.PositionX, (1+5+1+3)*20) // span + a1 t-i + a1
}

func TestFloatTextIndent2(t *testing.T) {
	body := blockBody(t, fmt.Sprintf(blockFloatTextIndentSource, "left", `
        oooooooooooo
        <a>aa <span>float</span> aa</a>`))
	lines := unpack(t, only(t, body), 2)
	p1 := only(t, lines[0])
	assertApprox(t, "p1.position_x", p1.PositionX, 1*20) // text-indent
	assertApprox(t, "p1.width", p1.Width, 12*20)         // p text

	a1, _, a2 := blockFloatTextIndentSpan(t, lines[1], 0)
	assertApprox(t, "a1.width", a1.Width, 3*20)
	assertApprox(t, "a1.position_x", a1.PositionX, (1+5)*20)   // span
	assertApprox(t, "a2.width", a2.Width, 2*20)                // leading space collapse
	assertApprox(t, "a2.position_x", a2.PositionX, (1+5+3)*20) // span + a1
}

func TestFloatTextIndent3(t *testing.T) {
	body := blockBody(t, fmt.Sprintf(blockFloatTextIndentSource, "right", `
        oooooooooooo
        <a>aa <span>float</span> aa</a>
        oooooooooooo
      `))
	lines := unpack(t, only(t, body), 3)
	p1 := only(t, lines[0])
	assertApprox(t, "p1.position_x", p1.PositionX, 1*20) // text-indent
	assertApprox(t, "p1.width", p1.Width, 12*20)         // p text

	a1, _, a2 := blockFloatTextIndentSpan(t, lines[1], (14-5-1)*20)
	assertApprox(t, "a1.position_x", a1.PositionX, 0) // span
	assertApprox(t, "a2.width", a2.Width, 2*20)       // leading space collapse
	assertApprox(t, "a2.position_x", a2.PositionX, (14-5-1-2)*20)

	p2 := only(t, lines[2])
	assertApprox(t, "p2.position_x", p2.PositionX, 0)
	assertApprox(t, "p2.width", p2.Width, 12*20) // p text
}

func TestFloatPreviousBreak(t *testing.T) {
	pages := blockPagesN(t, `
      <style>
        @page {
          size: 13px;
        }
        body {
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        p {
          break-after: avoid;
        }
        section {
          break-inside: avoid;
          float: left;
        }
      </style>
      <article>
        oooooo
        oooooo
        oooooo
        oooooo
      </article>
      <p>xxxxxx</p>
      <p>yyyyyy</p>
      <section>
        <div>aaaaaa</div>
        <div>cccccc</div>
      </section>
      <article>
        dddddd
      </article>`, 2)
	_, body := pageBody(t, pages[0])
	only(t, body)
	_, body = pageBody(t, pages[1])
	unpack(t, body, 4)
}

func TestFloatFail(t *testing.T) {
	t.Skip("upstream marks test_float_fail as an expected failure")
}

func TestFloatTableAbortedRow(t *testing.T) {
	pages := renderPages(t, `
      <style>
        @page {size: 10px 7px}
        body {font-family: weasyprint; font-size: 2px; line-height: 1}
        div {float: right; orphans: 1}
        td {break-inside: avoid}
      </style>
      <table><tbody>
        <tr><td>abc</td></tr>
        <tr><td>abc</td></tr>
        <tr><td>def <div>f<br>g</div> ghi</td></tr>
      </tbody></table>
    `)
	if len(pages) != 2 {
		t.Fatalf("%d pages", len(pages))
	}

	_, body := pageBody(t, pages[0])
	tbody := only(t, only(t, only(t, body)))
	for _, tr := range tbody.Children {
		textbox := only(t, only(t, only(t, tr)))
		blockAssertText(t, "textbox", textbox, "abc")
	}

	_, body = pageBody(t, pages[1])
	td := only(t, only(t, only(t, only(t, only(t, body)))))
	lines := unpack(t, td, 2)
	children := unpack(t, lines[0], 2)
	blockAssertText(t, "textbox", children[0], "def ")
	blockAssertText(t, "textbox", only(t, lines[1]), "ghi")
	divLines := unpack(t, children[1], 2)
	blockAssertText(t, "textbox", unpack(t, divLines[0], 2)[0], "f")
	blockAssertText(t, "textbox", only(t, divLines[1]), "g")
}

func TestFloatFormattingContextAvoidRTL(t *testing.T) {
	renderPages(t, `
      <div style="direction: rtl">
        <div style="overflow: hidden"></div>
      </div>
    `)
}

func TestFloatNestedRightFloat(t *testing.T) {
	// Regression test for #1510.
	body := blockBody(t, `
      <style>
        body { width: 100px; font: 20px weasyprint }
        div { float: right; width: 50px }
      </style>
      <b><i><div></div></i></b>ab c`)
	lines := unpack(t, body, 2)
	assertApprox(t, "line1.width", lines[0].Width, 40)
	assertApprox(t, "line2.width", lines[1].Width, 20)
}

func TestFloatFirstLetterFloat(t *testing.T) {
	// Regression test for #1859.
	body := blockBody(t, `
      <style>
        body { width: 100px; font: 20px weasyprint }
        p:first-letter { float: left }
      </style>
      <p>Lor`)
	children := unpack(t, only(t, only(t, body)), 2)
	assertApprox(t, "first_letter.position_x", children[0].PositionX, 0)
	// TODO: fix problem described in #1859.
	// assert text.position_x == 20
}

// blockAssertPageDivs checks that the body of each page holds one div, at
// the top of the page, whose text at the depth of the path is the text of
// the page.
func blockAssertPageDivs(t *testing.T, pages []*Box, depths []int, texts []string) {
	t.Helper()
	if len(pages) != len(texts) {
		t.Fatalf("%d pages, want %d", len(pages), len(texts))
	}
	for i, page := range pages {
		_, body := pageBody(t, page)
		div := only(t, body)
		assertApprox(t, "div.position_y", div.PositionY, 0)
		box := div
		for range depths[i] {
			if len(box.Children) == 0 {
				t.Fatalf("page %d: %v has no child", i+1, box)
			}
			box = box.Children[0]
		}
		blockAssertText(t, "text", box, texts[i])
	}
}

func TestFloatBreakAfterPage(t *testing.T) {
	pages := blockPages(t, `
      <div style="break-after: page; float: left">float</div>
      <div>end</div>
    `)
	blockAssertPageDivs(t, pages, []int{2, 2}, []string{"float", "end"})
}

func TestFloatInBlockBreakAfterPage(t *testing.T) {
	pages := blockPages(t, `
      <div>
        <div style="break-after: page; float: left">float</div>
      </div>
      <div>end</div>
    `)
	blockAssertPageDivs(t, pages, []int{3, 2}, []string{"float", "end"})
}

func TestFloatBreakBeforePage(t *testing.T) {
	pages := blockPages(t, `
      <div>start</div>
      <div style="break-before: page; float: left">float</div>
    `)
	blockAssertPageDivs(t, pages, []int{2, 2}, []string{"start", "float"})
}

func TestFloatInDivBreakBeforePage(t *testing.T) {
	pages := blockPages(t, `
      <div>start</div>
      <div>
        <div style="break-before: page; float: left">float</div>
      </div>
    `)
	blockAssertPageDivs(t, pages, []int{2, 3}, []string{"start", "float"})
}

func TestFloatBreakAfterPageNoFit(t *testing.T) {
	pages := blockPages(t, `
      <style>
        @page { size: 100px }
        html { line-height: 20px }
      </style>
      <div style="height: 90px">start</div>
      <div style="break-after: right; float: left">float</div>
      <div>end</div>
    `)
	blockAssertPageDivs(t, pages, []int{2, 2, 2}, []string{"start", "float", "end"})
}

// TestPageFloatInsideLine tests a line next to a page float whose top is
// below the strut of the line and inside the line. It is not a test of
// upstream (D60).
func TestPageFloatInsideLine(t *testing.T) {
	// The strut of P1 is 20px high, and its text makes the line 30px high,
	// from y = 50 to y = 80. The float starts at y = 75.
	for _, tt := range []struct {
		name, text, want string
	}{
		// The line goes beside the float, as in a word processor. At the
		// full width of the page area, the line holds "P1 aa", which is
		// wider than the room beside the float.
		{"beside", "P1 aa", "F1@50,75 P1@150,50 P2@150,110 P3@150,130 P4@50,150"},
		// A word wider than the room beside the float puts the line below
		// the float.
		{"below", "P1xxxx", "F1@50,75 P1xxxx@50,135 P2@50,165 P3@50,185 P4@50,205"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pages, _ := renderPageFloats(t, pageFloatStyle+
				`<p><span style="font-size: 30px; line-height: 30px">`+tt.text+`</span></p>`+
				pageFloatDiv("1", "margin 25px", "")+paragraphs(2, 3))
			if got := describePages(pages); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFloatDisplayInherit(t *testing.T) {
	// Regression test for #2998.
	body := blockBody(t, `
      <style>
        @page { size: 100px }
        em { display: inherit; float: right; width: 15px; height: 15px }
      </style>
      <body><span><em></em></span></body>
    `)
	line := only(t, body)
	span := unpack(t, line, 2)[0]
	assertApprox(t, "em.position_x", only(t, span).PositionX, 85)
}
