// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_absolute.py.

package layout

import "testing"

// TestDrawAbsoluteSplit1 is test_absolute_split_1 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit1(t *testing.T) {
	assertPixels(t, "absolute_split_1", `
        BBBBRRRRRRRR____
        BBBBRRRRRRRR____
        BBBBRR__________
        BBBBRR__________
    `, `
        <style>
            @page {
                size: 16px 2Px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div.split {
                color: blue;
                left: 0;
                position: absolute;
                top: 0;
                width: 4px;
            }
        </style>
        <div class="split">aa aa</div>
        <div>bbbbbb bbb</div>
    `)
}

// TestDrawAbsoluteSplit2 is test_absolute_split_2 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit2(t *testing.T) {
	assertPixels(t, "absolute_split_2", `
        RRRRRRRRRRRRBBBB
        RRRRRRRRRRRRBBBB
        RRRR________BBBB
        RRRR________BBBB
    `, `
        <style>
            @page {
                size: 16px 2px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div.split {
                color: blue;
                position: absolute;
                top: 0;
                right: 0;
                width: 4px;
            }
        </style>
        <div class="split">aa aa</div>
        <div>bbbbbb bb</div>
    `)
}

// TestDrawAbsoluteSplit3 is test_absolute_split_3 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit3(t *testing.T) {
	assertPixels(t, "absolute_split_3", `
        BBBBRRRRRRRR____
        BBBBRRRRRRRR____
        RRRRRRRRRR______
        RRRRRRRRRR______
    `, `
        <style>
            @page {
                size: 16px 2px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2PX;
                line-height: 1;
            }
            div.split {
                color: blue;
                position: absolute;
                top: 0;
                left: 0;
                width: 4px;
            }
        </style>
        <div class="split">aa</div>
        <div>bbbbbb bbbbb</div>
    `)
}

// TestDrawAbsoluteSplit4 is test_absolute_split_4 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit4(t *testing.T) {
	assertPixels(t, "absolute_split_4", `
        RRRRRRRRRRRRBBBB
        RRRRRRRRRRRRBBBB
        RRRRRRRRRR______
        RRRRRRRRRR______
    `, `
        <style>
            @page {
                size: 16px 2px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div.split {
                color: blue;
                position: absolute;
                top: 0;
                right: 0;
                width: 4px;
            }
        </style>
        <div class="split">aa</div>
        <div>bbbbbb bbbbb</div>
    `)
}

// TestDrawAbsoluteSplit5 is test_absolute_split_5 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit5(t *testing.T) {
	assertPixels(t, "absolute_split_5", `
        BBBBRRRR____gggg
        BBBBRRRR____gggg
        BBBBRRRRRR__gggg
        BBBBRRRRRR__gggg
    `, `
        <style>
            @page {
                size: 16px 2px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div.split {
                color: blue;
                position: absolute;
                top: 0;
                left: 0;
                width: 4px;
            }
            div.split2 {
                color: green;
                position: absolute;
                top: 0;
                right: 0;
                width: 4px;
        </style>
        <div class="split">aa aa</div>
        <div class="split2">cc cc</div>
        <div>bbbb bbbbb</div>
    `)
}

// TestDrawAbsoluteSplit6 is test_absolute_split_6 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit6(t *testing.T) {
	assertPixels(t, "absolute_split_6", `
        BBBBRRRR____gggg
        BBBBRRRR____gggg
        BBBBRRRRRR______
        BBBBRRRRRR______
    `, `
        <style>
            @page {
                size: 16px 2px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div.split {
                color: blue;
                position: absolute;
                width: 4px;
            }
            div.split2 {
                color: green;
                position: absolute;
                top: 0;
                right: 0;
                width: 4px;
        </style>
        <div class="split">aa aa</div>
        <div class="split2">cc</div>
        <div>bbbb bbbbb</div>
    `)
}

// TestDrawAbsoluteSplit7 is test_absolute_split_7 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit7(t *testing.T) {
	assertPixels(t, "absolute_split_7", `
        BBBBRRRRRRRRgggg
        BBBBRRRRRRRRgggg
        ____RRRR____gggg
        ____RRRR____gggg
    `, `
        <style>
            @page {
                size: 16px 2px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div.split {
                color: blue;
                position: absolute;
                width: 4px;
            }
            div.split2 {
                color: green;
                position: absolute;
                top: 0;
                right: 0;
                width: 4px;
            }
            div.push {
                margin-left: 4px;
            }
        </style>
        <div class="split">aa</div>
        <div class="split2">cc cc</div>
        <div class="push">bbbb bb</div>
    `)
}

// TestDrawAbsoluteSplit8 is test_absolute_split_8 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit8(t *testing.T) {
	assertPixels(t, "absolute_split_8", `
        ______
        ______
        ______
        ______
        __RR__
        __RR__
        ______
        ______
    `, `
        <style>
            @page {
                margin: 2px 0;
                size: 6px 8px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div {
                position: absolute;
                left: 2px;
                top: 2px;
                width: 2px;
            }
        </style>
        <div>a a a a</div>
    `)
}

// TestDrawAbsoluteSplit9 is test_absolute_split_9 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit9(t *testing.T) {
	assertPixels(t, "absolute_split_9", `
        ______
        ______
        BBRRBB
        BBRRBB
        BBRR__
        BBRR__
        ______
        ______
    `, `
        <style>
            @page {
                margin: 2px 0;
                size: 6px 8px;
            }
            body {
                color: blue;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div {
                color: red;
                position: absolute;
                left: 2px;
                top: 0;
                width: 2px;
            }
        </style>
        aaa a<div>a a a a</div>
    `)
}

// TestDrawAbsoluteSplit10 is test_absolute_split_10 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit10(t *testing.T) {
	assertPixels(t, "absolute_split_10", `
        BB____
        BB____
        __RR__
        __RR__
        __RR__
        __RR__

        BBRR__
        BBRR__
        __RR__
        __RR__
        ______
        ______
    `, `
        <style>
            @page {
                size: 6px;
            }
            body {
                color: blue;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div {
                color: red;
                position: absolute;
                left: 2px;
                top: 2px;
                width: 2px;
            }
            div + article {
                break-before: page;
            }
        </style>
        <article>a</article>
        <div>a a a a</div>
        <article>a</article>
    `)
}

// TestDrawAbsoluteSplit11 is test_absolute_split_11 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit11(t *testing.T) {
	assertPixels(t, "absolute_split_11", `
        BBBBBB
        BBBBBB
        BBRRBB
        BBRRBB
        __RR__
        __RR__
    `, `
        <style>
            @page {
                size: 6px;
            }
            body {
                color: blue;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div {
                bottom: 0;
                color: red;
                position: absolute;
                left: 2px;
                width: 2px;
            }
        </style>
        aaa aaa<div>a a</div>
    `)
}

// TestDrawAbsoluteSplit12 is test_absolute_split_12 in tests/draw/test_absolute.py.
func TestDrawAbsoluteSplit12(t *testing.T) {
	assertPixels(t, "absolute_split_12", `
        BBBBBB__
        BBBBBB__
        ________
        ________
        ________
        ________
        ________
        ________
        BB______
        BB______
        BBRR____
        BBRR____
        BBRRRR__
        BBRRRR__
        BBRRRRRR
        BBRRRRRR
    `, `
        <style>
            @page {
                size: 8px;
            }
            body {
                color: blue;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div {
                break-inside: avoid;
            }
            section {
                left: 2px;
                position: absolute;
                color: red;
            }
        </style>
        aaa
        <div>
          a
          <section>x<br>xx<br>xxx</section>
          <br>
          a<br>
          a<br>
          a
        </div>
    `)
}

// TestDrawAbsoluteNextPage is test_absolute_next_page in tests/draw/test_absolute.py.
func TestDrawAbsoluteNextPage(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	// TODO: currently, the layout of absolute boxes forces to render a box,
	// even when it doesn’t fit in the page. This workaround avoids placeholders
	// with no box. Instead, we should remove these placeholders, or avoid
	// crashes when they’re rendered.
	assertPixels(t, "absolute_next_page", `
        RRRRRRRRRR______
        RRRRRRRRRR______
        RRRRRRRRRR______
        RRRRRRRRRR______
        BBBBBBRRRR______
        BBBBBBRRRR______
        BBBBBB__________
        ________________
    `, `
        <style>
            @page {
                size: 16px 4px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div.split {
                color: blue;
                position: absolute;
                font-size: 3px;
            }
        </style>
        aaaaa aaaaa
        <div class="split">bb</div>
        aaaaa
    `)
}

// TestDrawAbsoluteRTL1 is test_absolute_rtl_1 in tests/draw/test_absolute.py.
func TestDrawAbsoluteRTL1(t *testing.T) {
	assertPixels(t, "absolute_rtl_1", `
        __________RRRRRR
        __________RRRRRR
        ________________
    `, `
        <style>
            @page {
                size: 16px 3px;
            }
            body {
                direction: rtl;
            }
            div {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
                position: absolute;
            }
        </style>
        <div>bbb</div>
    `)
}

// TestDrawAbsoluteRTL2 is test_absolute_rtl_2 in tests/draw/test_absolute.py.
func TestDrawAbsoluteRTL2(t *testing.T) {
	assertPixels(t, "absolute_rtl_2", `
        ________________
        _________RRRRRR_
        _________RRRRRR_
    `, `
        <style>
            @page {
                size: 16px 3px;
            }
            body {
                direction: rtl;
            }
            div {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
                padding: 1px;
                position: absolute;
            }
        </style>
        <div>bbb</div>
    `)
}

// TestDrawAbsoluteRTL3 is test_absolute_rtl_3 in tests/draw/test_absolute.py.
func TestDrawAbsoluteRTL3(t *testing.T) {
	assertPixels(t, "absolute_rtl_3", `
        ________________
        RRRRRR__________
        RRRRRR__________
    `, `
        <style>
            @page {
                size: 16px 3px;
            }
            body {
                direction: rtl;
            }
            div {
                bottom: 0;
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                left: 0;
                line-height: 1;
                position: absolute;
            }
        </style>
        <div>bbb</div>
    `)
}

// TestDrawAbsoluteRTL4 is test_absolute_rtl_4 in tests/draw/test_absolute.py.
func TestDrawAbsoluteRTL4(t *testing.T) {
	assertPixels(t, "absolute_rtl_4", `
        ________________
        _________RRRRRR_
        _________RRRRRR_
    `, `
        <style>
            @page {
                size: 16px 3px;
            }
            body {
                direction: rtl;
            }
            div {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
                position: absolute;
                right: 1px;
                top: 1px;
            }
        </style>
        <div>bbb</div>
    `)
}

// TestDrawAbsoluteRTL5 is test_absolute_rtl_5 in tests/draw/test_absolute.py.
func TestDrawAbsoluteRTL5(t *testing.T) {
	assertPixels(t, "absolute_rtl_5", `
        RRRRRR__________
        RRRRRR__________
        ________________
    `, `
        <style>
            @page {
                size: 16px 3px;
            }
            div {
                color: red;
                direction: rtl;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
                position: absolute;
            }
        </style>
        <div>bbb</div>
    `)
}

// TestDrawAbsolutePagesCounter is test_absolute_pages_counter in tests/draw/test_absolute.py.
func TestDrawAbsolutePagesCounter(t *testing.T) {
	assertPixels(t, "absolute_pages_counter", `
        ______
        _RR___
        _RR___
        _RR___
        _RR___
        _____B
        ______
        _RR___
        _RR___
        _BB___
        _BB___
        _____B
    `, `
        <style>
            @page {
                font-family: weasyprint;
                margin: 1px;
                size: 6px 6px;
                @bottom-right-corner {
                    color: blue;
                    content: counter(pages);
                    font-size: 1px;
                }
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div {
                color: blue;
                position: absolute;
            }
        </style>
        a a a <div>a a</div>
    `)
}

// TestDrawAbsolutePagesCounterOrphans is test_absolute_pages_counter_orphans in tests/draw/test_absolute.py.
func TestDrawAbsolutePagesCounterOrphans(t *testing.T) {
	assertPixels(t, "absolute_pages_counter_orphans", `
        ______
        _RR___
        _RR___
        _RR___
        _RR___
        ______
        ______
        ______
        _____B
        ______
        _RR___
        _RR___
        _BB___
        _BB___
        _GG___
        _GG___
        ______
        _____B
    `, `
        <style>
            @page {
                font-family: weasyprint;
                margin: 1px;
                size: 6px 9px;
                @bottom-right-corner {
                    color: blue;
                    content: counter(pages);
                    font-size: 1PX;
                }
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
                orphans: 2;
                widows: 2;
            }
            div {
                color: blue;
                position: absolute;
            }
            div ~ div {
                color: lime;
            }
        </style>
        a a a <div>a a a</div> a <div>a a a</div>
    `)
}

// TestDrawAbsoluteInInline is test_absolute_in_inline in tests/draw/test_absolute.py.
func TestDrawAbsoluteInInline(t *testing.T) {
	assertPixels(t, "absolute_in_inline", `
        ______
        _GG___
        _GG___
        _GG___
        _GG___
        ______
        ______
        ______
        ______

        ______
        _RR___
        _RR___
        _RR___
        _RR___
        _BB___
        _BB___
        ______
        ______
    `, `
        <style>
            @page {
                margin: 1px;
                size: 6px 9px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
                orphans: 2;
                widows: 2;
            }
            p {
                color: lime;
            }
            div {
                color: blue;
                position: absolute;
            }
        </style>
        <p>a a</p> a a <div>a</div>
    `)
}

// TestDrawFixedInInline is test_fixed_in_inline in tests/draw/test_absolute.py.
func TestDrawFixedInInline(t *testing.T) {
	assertPixels(t, "fixed_in_inline", `
        ______
        _GG___
        _GG___
        _GG___
        _GG___
        _BB___
        _BB___
        ______
        ______

        ______
        _RR___
        _RR___
        _RR___
        _RR___
        _BB___
        _BB___
        ______
        ______
    `, `
        <style>
            @page {
                margin: 1px;
                size: 6px 9px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2PX;
                line-height: 1;
                orphans: 2;
                widows: 2;
            }
            p {
                color: lime;
            }
            div {
                color: blue;
                position: fixed;
            }
        </style>
        <p>a a</p> a a <div>a</div>
    `)
}

// TestDrawAbsoluteImageBackground is test_absolute_image_background in tests/draw/test_absolute.py.
func TestDrawAbsoluteImageBackground(t *testing.T) {
	assertPixels(t, "absolute_image_background", `
        ____
        _RBB
        _BBB
        _BBB
    `, `
        <style>
          @page {
            size: 4px;
          }
          img {
            background: blue;
            position: absolute;
            top: 1px;
            left: 1px;
          }
        </style>
        <img src="pattern-transparent.svg" />
    `)
}

// TestDrawAbsoluteInAbsoluteBreak is test_absolute_in_absolute_break in tests/draw/test_absolute.py.
func TestDrawAbsoluteInAbsoluteBreak(t *testing.T) {
	// Regression test for #2134.
	assertPixels(t, "absolute_in_absolute_break", `
        BBBB
        BBBB
        BBBB
        BBBB
        BBBB

        BBBB
        BBBB
        BBBB
        BBBB
        BBBB

        BBBB
        BBBB
        RRRR
        RRRR
        RRRR

        RRRR
        RRRR
        ____
        ____
        ____
    `, `
        <style>
          @page {
            size: 4px 5px;
          }
          body {
            font-size: 2px;
            line-height: 1;
          }
          div {
            position: absolute;
            width: 100%;
          }
        </style>
        <div style="background: blue">
          <br><br><br><br><br>
          <div style="background: red">
            <br><br>
          </div>
        </div>
        <br><br><br><br><br><br><br>
    `)
}

// TestDrawAbsoluteAlternativeLayout is test_absolute_alternative_layout in tests/draw/test_absolute.py.
func TestDrawAbsoluteAlternativeLayout(t *testing.T) {
	// Regression test for #2450.
	for _, tc := range []struct{ name, pixels, html string }{
		{"absolute_grid", `
        ______
        _RRRR_
        _RBBR_
        _RBBR_
        _RRRR_
        ______
    `, `
        <style>
          @page {
            size: 6px;
          }
          div {
            background: blue;
            position: absolute;
            display: grid;
            top: 1px;
            left: 1px;
            border: 1px solid red;
            font: 2px weasyprint;
            color: transparent;
            width: 2px;
          }
        </style>
        <div>a</div>
    `},
		{"absolute_flex", `
        ______
        _RRRR_
        _RBBR_
        _RBBR_
        _RRRR_
        ______
    `, `
        <style>
          @page {
            size: 6px;
          }
          div {
            background: blue;
            position: absolute;
            display: flex;
            top: 1px;
            left: 1px;
            border: 1px solid red;
            font: 2px weasyprint;
            color: transparent;
            width: 2px;
          }
        </style>
        <div>a</div>
    `},
		{"relative_grid", `
        ______
        _RRRR_
        _RBBR_
        _RBBR_
        _RRRR_
        ______
    `, `
        <style>
          @page {
            size: 6px;
          }
          div {
            background: blue;
            position: relative;
            display: grid;
            top: 1px;
            left: 1px;
            border: 1px solid red;
            font: 2px weasyprint;
            color: transparent;
            width: 2px;
          }
        </style>
        <div>a</div>
    `},
		{"relative_flex", `
        ______
        _RRRR_
        _RBBR_
        _RBBR_
        _RRRR_
        ______
    `, `
        <style>
          @page {
            size: 6px;
          }
          div {
            background: blue;
            position: relative;
            display: flex;
            top: 1px;
            left: 1px;
            border: 1px solid red;
            font: 2px weasyprint;
            color: transparent;
            width: 2px;
          }
        </style>
        <div>a</div>
    `},
	} {
		t.Run(tc.name, func(t *testing.T) { assertPixels(t, "absolute_alternative_layout_"+tc.name, tc.pixels, tc.html) })
	}
}
