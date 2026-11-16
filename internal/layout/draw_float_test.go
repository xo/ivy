// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_float.py.

package layout

import "testing"

// TestDrawFloat is test_float in tests/draw/test_float.py.
func TestDrawFloat(t *testing.T) {
	assertPixels(t, "float", `
        rBBB__aaaa
        BBBB__aaaa
        BBBB__aaaa
        BBBB__aaaa
        __________
    `, `
      <style>
        @page { size: 10px 5px }
      </style>
      <div>
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="blue.jpg">
      </div>
    `)
}

// TestDrawFloatRTL is test_float_rtl in tests/draw/test_float.py.
func TestDrawFloatRTL(t *testing.T) {
	assertPixels(t, "float_rtl", `
        rBBB__aaaa
        BBBB__aaaa
        BBBB__aaaa
        BBBB__aaaa
        __________
    `, `
      <style>
        @page { size: 10px 5Px }
      </style>
      <div style="direction: rtl">
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="blue.jpg">
      </div>
    `)
}

// TestDrawFloatInline is test_float_inline in tests/draw/test_float.py.
func TestDrawFloatInline(t *testing.T) {
	assertPixels(t, "float_inline", `
        rBBBGG_____aaaa
        BBBBGG_____aaaa
        BBBB_______aaaa
        BBBB_______aaaa
        _______________
    `, `
      <style>
        @page { size: 15px 5px }
        body { font-family: weasyprint; font-size: 2px; line-height: 1;
               color: lime }
      </style>
      <div>
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="blue.jpg">
        <span>a</span>
      </div>
    `)
}

// TestDrawFloatInlineRTL is test_float_inline_rtl in tests/draw/test_float.py.
func TestDrawFloatInlineRTL(t *testing.T) {
	assertPixels(t, "float_inline_rtl", `
        rBBB_____GGaaaa
        BBBB_____GGaaaa
        BBBB_______aaaa
        BBBB_______aaaa
        _______________
    `, `
      <style>
        @page { size: 15px 5px }
        body { font-family: weasyprint; font-size: 2px; line-height: 1;
               color: lime }
      </style>
      <div style="direction: rtl">
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="blue.jpg">
        <span>a</span>
      </div>
    `)
}

// TestDrawFloatInlineBlock is test_float_inline_block in tests/draw/test_float.py.
func TestDrawFloatInlineBlock(t *testing.T) {
	assertPixels(t, "float_inline_block", `
        rBBBGG_____aaaa
        BBBBGG_____aaaa
        BBBB_______aaaa
        BBBB_______aaaa
        _______________
    `, `
      <style>
        @page { size: 15px 5px }
        body { font-family: weasyprint; font-size: 2px; line-height: 1;
               color: lime }
      </style>
      <div>
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="blue.jpg">
        <span style="display: inline-block">a</span>
      </div>
    `)
}

// TestDrawFloatInlineBlockRTL is test_float_inline_block_rtl in tests/draw/test_float.py.
func TestDrawFloatInlineBlockRTL(t *testing.T) {
	assertPixels(t, "float_inline_block_rtl", `
        rBBB_____GGaaaa
        BBBB_____GGaaaa
        BBBB_______aaaa
        BBBB_______aaaa
        _______________
    `, `
      <style>
        @page { size: 15Px 5px }
        body { font-family: weasyprint; font-size: 2px; line-height: 1;
               color: lime }
      </style>
      <div style="direction: rtl">
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="blue.jpg">
        <span style="display: inline-block">a</span>
      </div>
    `)
}

// TestDrawFloatTable is test_float_table in tests/draw/test_float.py.
func TestDrawFloatTable(t *testing.T) {
	assertPixels(t, "float_table", `
        rBBBGG_____aaaa
        BBBBGG_____aaaa
        BBBB_______aaaa
        BBBB_______aaaa
        _______________
    `, `
      <style>
        @page { size: 15px 5px }
        body { font-family: weasyprint; font-size: 2px; line-height: 1;
               color: lime }
      </style>
      <div>
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="blue.jpg">
        <table><tbody><tr><td>a</td></tr></tbody></table>
      </div>
    `)
}

// TestDrawFloatTableRTL is test_float_table_rtl in tests/draw/test_float.py.
func TestDrawFloatTableRTL(t *testing.T) {
	assertPixels(t, "float_table_rtl", `
        rBBB_____GGaaaa
        BBBB_____GGaaaa
        BBBB_______aaaa
        BBBB_______aaaa
        _______________
    `, `
      <style>
        @page { size: 15px 5px }
        body { font-family: weasyprint; font-size: 2px; line-height: 1;
               color: lime }
      </style>
      <div style="direction: rtl">
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="blue.jpg">
        <table><tbody><tr><td>a</td></tr></tbody></table>
      </div>
    `)
}

// TestDrawFloatInlineTable is test_float_inline_table in tests/draw/test_float.py.
func TestDrawFloatInlineTable(t *testing.T) {
	assertPixels(t, "float_inline_table", `
        rBBBGG_____aaaa
        BBBBGG_____aaaa
        BBBB_______aaaa
        BBBB_______aaaa
        _______________
    `, `
      <style>
        @page { size: 15PX 5px }
        table { display: inline-table }
        body { font-family: weasyprint; font-size: 2px; line-height: 1;
               color: lime }
      </style>
      <div>
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="blue.jpg">
        <table><tbody><tr><td>a</td></tr></tbody></table>
      </div>
    `)
}

// TestDrawFloatInlineTableRTL is test_float_inline_table_rtl in tests/draw/test_float.py.
func TestDrawFloatInlineTableRTL(t *testing.T) {
	assertPixels(t, "float_inline_table_rtl", `
        rBBB_____GGaaaa
        BBBB_____GGaaaa
        BBBB_______aaaa
        BBBB_______aaaa
        _______________
    `, `
      <style>
        @page { size: 15px 5px }
        table { display: inline-table }
        body { font-family: weasyprint; font-size: 2px; line-height: 1;
               color: lime }
      </style>
      <div style="direction: rtl">
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="blue.jpg">
        <table><tbody><tr><td>a</td></tr></tbody></table>
      </div>
    `)
}

// TestDrawFloatReplacedBlock is test_float_replaced_block in tests/draw/test_float.py.
func TestDrawFloatReplacedBlock(t *testing.T) {
	assertPixels(t, "float_replaced_block", `
        rBBBaaaa___rBBB
        BBBBaaaa___BBBB
        BBBBaaaa___BBBB
        BBBBaaaa___BBBB
        _______________
    `, `
      <style>
        @page { size: 15px 5px }
      </style>
      <div>
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="pattern.png">
        <img style="display: block" src="blue.jpg">
      </div>
    `)
}

// TestDrawFloatReplacedBlockRTL is test_float_replaced_block_rtl in tests/draw/test_float.py.
func TestDrawFloatReplacedBlockRTL(t *testing.T) {
	assertPixels(t, "float_replaced_block_rtl", `
        rBBB___aaaarBBB
        BBBB___aaaaBBBB
        BBBB___aaaaBBBB
        BBBB___aaaaBBBB
        _______________
    `, `
      <style>
        @page { size: 15px 5px }
      </style>
      <div style="direction: rtl">
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="pattern.png">
        <img style="display: block" src="blue.jpg">
      </div>
    `)
}

// TestDrawFloatReplacedInline is test_float_replaced_inline in tests/draw/test_float.py.
func TestDrawFloatReplacedInline(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "float_replaced_inline", `
        rBBBaaaa___rBBB
        BBBBaaaa___BBBB
        BBBBaaaa___BBBB
        BBBBaaaa___BBBB
        _______________
    `, `
      <style>
        @page { size: 15px 5px }
        body { line-height: 1px }
      </style>
      <div>
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="pattern.png">
        <img src="blue.jpg">
      </div>
    `)
}

// TestDrawFloatReplacedInlineRTL is test_float_replaced_inline_rtl in tests/draw/test_float.py.
func TestDrawFloatReplacedInlineRTL(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "float_replaced_inline_rtl", `
        rBBB___aaaarBBB
        BBBB___aaaaBBBB
        BBBB___aaaaBBBB
        BBBB___aaaaBBBB
        _______________
    `, `
      <style>
        @page { size: 15px 5px }
        body { line-height: 1px }
      </style>
      <div style="direction: rtl">
        <img style="float: left" src="pattern.png">
        <img style="float: right" src="pattern.png">
        <img src="blue.jpg">
      </div>
    `)
}

// TestDrawFloatMargin is test_float_margin in tests/draw/test_float.py.
func TestDrawFloatMargin(t *testing.T) {
	assertPixels(t, "float_margin", `
        BBBBRRRRRRRRRR__
        BBBBRRRRRRRRRR__
        __RRRRRRRRRR____
        __RRRRRRRRRR____
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
                float: left;
                width: 4px;
            }
            div.pushed {
                margin-left: 2px;
            }
        </style>
        <div class="split">aa</div>
        <div class="pushed">bbbbb bbbbb</div>`)
}

// TestDrawFloatSplit1 is test_float_split_1 in tests/draw/test_float.py.
func TestDrawFloatSplit1(t *testing.T) {
	assertPixels(t, "float_split_1", `
        BBBBRRRRRRRRRRRR
        BBBBRRRRRRRRRRRR
        BBBBRRRR________
        BBBBRRRR________
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
                float: left;
                width: 4px;
            }
        </style>
        <div class="split">aa aa</div>
        <div>bbbbbb bb</div>`)
}

// TestDrawFloatSplit2 is test_float_split_2 in tests/draw/test_float.py.
func TestDrawFloatSplit2(t *testing.T) {
	assertPixels(t, "float_split_2", `
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
            float: right;
            width: 4px;
          }
        </style>
        <div class="split">aa aa</div>
        <div>bbbbbb bb</div>`)
}

// TestDrawFloatSplit3 is test_float_split_3 in tests/draw/test_float.py.
func TestDrawFloatSplit3(t *testing.T) {
	assertPixels(t, "float_split_3", `
        BBBBRRRRRRRRRRRR
        BBBBRRRRRRRRRRRR
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
                float: left;
                width: 4px;
            }
        </style>
        <div class="split">aa</div>
        <div>bbbbbb bbbbb</div>`)
}

// TestDrawFloatSplit4 is test_float_split_4 in tests/draw/test_float.py.
func TestDrawFloatSplit4(t *testing.T) {
	assertPixels(t, "float_split_4", `
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
                float: right;
                width: 4px;
            }
        </style>
        <div class="split">aa</div>
        <div>bbbbbb bbbbb</div>`)
}

// TestDrawFloatSplit5 is test_float_split_5 in tests/draw/test_float.py.
func TestDrawFloatSplit5(t *testing.T) {
	assertPixels(t, "float_split_5", `
        BBBBRRRRRRRRgggg
        BBBBRRRRRRRRgggg
        BBBBRRRR____gggg
        BBBBRRRR____gggg
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
                float: left;
                width: 4px;
            }
            div.split2 {
                color: green;
                float: right;
                width: 4px;
        </style>
        <div class="split">aa aa</div>
        <div class="split2">cc cc</div>
        <div>bbbb bb</div>`)
}

// TestDrawFloatSplit6 is test_float_split_6 in tests/draw/test_float.py.
func TestDrawFloatSplit6(t *testing.T) {
	assertPixels(t, "float_split_6", `
        BBBBRRRRRRRRgggg
        BBBBRRRRRRRRgggg
        BBBBRRRR________
        BBBBRRRR________
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
                float: left;
                width: 4px;
            }
            div.split2 {
                color: green;
                float: right;
                width: 4px;
        </style>
        <div class="split">aa aa</div>
        <div class="split2">cc</div>
        <div>bbbb bb</div>`)
}

// TestDrawFloatSplit7 is test_float_split_7 in tests/draw/test_float.py.
func TestDrawFloatSplit7(t *testing.T) {
	assertPixels(t, "float_split_7", `
        BBBBRRRRRRRRgggg
        BBBBRRRRRRRRgggg
        RRRR________gggg
        RRRR________gggg
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
                float: left;
                width: 4px;
            }
            div.split2 {
                color: green;
                float: right;
                width: 4px;
        </style>
        <div class="split">aa</div>
        <div class="split2">cc cc</div>
        <div>bbbb bb</div>`)
}

// TestDrawFloatSplit8 is test_float_split_8 in tests/draw/test_float.py.
func TestDrawFloatSplit8(t *testing.T) {
	assertPixels(t, "float_split_8", `
        BBBB__RRRRRRRRRR
        BBBB__RRRRRRRRRR
        BBBB__RRRR______
        BBBB__RRRR______
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
                float: left;
                margin-right: 2px;
                width: 4px;
            }
        </style>
        <div class="split">aa aa</div>
        <div>bbbbb bb</div>`)
}

// TestDrawFloatSplit9 is test_float_split_9 in tests/draw/test_float.py.
func TestDrawFloatSplit9(t *testing.T) {
	assertPixels(t, "float_split_9", `
        RRRRRRRRRRBBBB__
        RRRRRRRRRRBBBB__
        RRRR______BBBB__
        RRRR______BBBB__
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
                float: right;
                margin-right: 2px;
                width: 4px;
            }
        </style>
        <div class="split">aa aa</div>
        <div>bbbbb bb</div>`)
}

// TestDrawFloatSplit10 is test_float_split_10 in tests/draw/test_float.py.
func TestDrawFloatSplit10(t *testing.T) {
	assertPixels(t, "float_split_10", `
        RRRRRRRRRR______
        RRRRRRRRRR______
        RRRRRRRRRR______
        RRRRRRRRRR______
        ________________
        RRRRRR____BBBB__
        RRRRRR____BBBB__
        RRRRRR____BBBB__
        RRRRRR____BBBB__
        ________________
    `, `
        <style>
            @page {
                size: 16px 5px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div.split {
                color: blue;
                float: right;
                margin-right: 2px;
                width: 4px;
            }
        </style>
        <div>bbbbb bbbbb</div>
        <div class="split">aa aa</div>
        <div>bbb bbb</div>`)
}

// TestDrawFloatSplit11 is test_float_split_11 in tests/draw/test_float.py.
func TestDrawFloatSplit11(t *testing.T) {
	assertPixels(t, "float_split_11", `
        ________________
        _BBBBBBBBBB_____
        _BBBBBBBBBB_____
        _BBBBBBBBBB_____
        _BBBBBBBBBB_____
        ________________
        ________________
        ________________
        _BBBB___________
        _BBBB___________
        _rrrrrrrrrrrrrr_
        _rrrrrrrrrrrrrr_
        ________________
        ________________
    `, `
        <style>
            @page {
                margin: 1px;
                size: 16px 7px;
            }
            body {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            div.split {
                color: blue;
                float: right;
            }
        </style>
        <div class="split">aaaaa aaaaa aa</div>
        bbbbbbb`)
}

// TestDrawFloatSplit12 is test_float_split_12 in tests/draw/test_float.py.
func TestDrawFloatSplit12(t *testing.T) {
	assertPixels(t, "float_split_12", `
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBGG______BBBB
        BBBBGG______BBBB
        BBBB________BBBB
        BBBB________BBBB
        ________________
    `, `
        <style>
            @page {
                size: 16px 5px;
            }
            body {
                color: lime;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            article {
                background: blue;
                height: 5px;
            }
            div {
                background: red;
                color: blue;
            }
        </style>
        <article></article>
        <section>
          a
          <div style="float: left"><p>aa<p>aa</div>
          <div style="float: right"><p>bb<p>bb</div>`)
}

// TestDrawFloatSplit13 is test_float_split_13 in tests/draw/test_float.py.
func TestDrawFloatSplit13(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "float_split_13", `
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBGG______BBBB
        BBBBGG______BBBB
        BBBB________BBBB
        BBBB________BBBB
        ________________
    `, `
        <style>
            @page {
                size: 16px 5px;
            }
            body {
                color: lime;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            article {
                background: blue;
                height: 5px;
            }
            div {
                background: red;
                color: blue;
            }
        </style>
        <article></article>
        <section>
          <div style="float: left"><p>a<p>aa</div>
          a
          <div style="float: right"><p>bb<p>bb</div>`)
}

// TestDrawFloatSplit14 is test_float_split_14 in tests/draw/test_float.py.
func TestDrawFloatSplit14(t *testing.T) {
	assertPixels(t, "float_split_14", `
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        BBBBGG______BBBB
        BBBBGG______BBBB
        BBBB________BBBB
        BBBB________BBBB
        ________________
    `, `
        <style>
            @page {
                size: 16px 5px;
            }
            body {
                color: lime;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            article {
                background: blue;
                height: 5px;
            }
            div {
                background: red;
                color: blue;
            }
        </style>
        <article></article>
        a
        <div style="float: left"><p>aa<p>aa</div>
        <div style="float: right"><p>bb<p>bb</div>`)
}

// TestDrawFloatSplit15 is test_float_split_15 in tests/draw/test_float.py.
func TestDrawFloatSplit15(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "float_split_15", `
        BB__RRRRRRRRRR__
        BB__RRRRRRRRRR__
        BB__RRRRRRRRRR__
        BB__RRRRRRRRRR__
        GGBBRRRRRRRRRR__
        GGBBRRRRRRRRRR__
        GGBBRRRRRRRRRR__
        GGBBRRRRRRRRRR__
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
        </style>
        <div style="float: left; position: relative; color: blue; width: 4px">
          a a a
          <div style="float: left; color: lime; width: 2px">
            a a
          </div>
          a a
        </div>
        <div>bbbbb bbbbb bbbbb bbbbb bbbbb</div>`)
}

// TestDrawFloatSplitClear is test_float_split_clear in tests/draw/test_float.py.
func TestDrawFloatSplitClear(t *testing.T) {
	assertPixels(t, "float_split_clear", `
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
        rrrr
        rrrr
    `, `
        <style>
            @page {
                size: 4px;
            }
            body {
                color: red;
                font: 2px / 1 weasyprint;
            }
            div {
                color: blue;
                float: left;
            }
            article {
                clear: left;
            }
        </style>
        <div>bb bb bb bb bb</div>
        <article>aa</article>`)
}

// TestDrawFloatSplitClearFloat is test_float_split_clear_float in tests/draw/test_float.py.
func TestDrawFloatSplitClearFloat(t *testing.T) {
	assertPixels(t, "float_split_clear_float", `
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
        rrrr
        rrrr
    `, `
        <style>
            @page {
                size: 4px;
            }
            body {
                color: red;
                font: 2px / 1 weasyprint;
            }
            div {
                color: blue;
                float: left;
            }
            article {
                clear: left;
                float: left;
                text-decoration: underline;
            }
        </style>
        <div>bb bb bb bb bb</div>
        <article>aa</article>`)
}

// TestDrawFloatSplitClearEmpty is test_float_split_clear_empty in tests/draw/test_float.py.
func TestDrawFloatSplitClearEmpty(t *testing.T) {
	assertPixels(t, "float_split_clear_empty", `
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
        ____
        ____
    `, `
        <style>
            @page {
                size: 4px;
            }
            body {
                color: red;
                font: 2px / 1 weasyprint;
            }
            div {
                color: blue;
                float: left;
            }
            article {
                clear: left;
            }
        </style>
        <div>bb bb bb bb bb</div>
        <article></article>`)
}

// TestDrawFloatSplitLargeParent is test_float_split_large_parent in tests/draw/test_float.py.
func TestDrawFloatSplitLargeParent(t *testing.T) {
	assertPixels(t, "float_split_large_parent", `
        BBBB
        BBBB
        BBBB
        BBBB
        GGGG

        BBBB
        BBBB
        BBBB
        BBBB
        GGGG

        BBBB
        BBBB
        GGGG
        GGGG
        GGGG

        rrrr
        rrrr
        GGGG
        GGGG
        GGGG
    `, `
        <style>
            @page {
                size: 4px 5px;
            }
            body {
                color: red;
                font: 2px / 1 weasyprint;
            }
            section {
                background: lime;
                height: 100px;
            }
            div {
                color: blue;
                float: left;
            }
            article {
                clear: left;
            }
        </style>
        <section>
        <div>bb bb bb bb bb</div>
        <article>aa</article>`)
}

// TestDrawFloatSplitInStackingContext is test_float_split_in_stacking_context in tests/draw/test_float.py.
func TestDrawFloatSplitInStackingContext(t *testing.T) {
	assertPixels(t, "float_split_in_stacking_context", `
        RRRR
        RRRR
        BBBB
        BBBB
        ____

        BBBB
        BBBB
        RRRR
        RRRR
        ____

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
                color: red;
                font: 2px / 1 weasyprint;
            }
            section {
                display: flow-root;
            }
            div {
                color: blue;
                float: left;
            }
            article {
                clear: left;
            }
        </style>
        <section>
        aa
        <div>bb bb</div>
        cc cc`)
}

// TestDrawFloatNextPageNotRenderedAgain is test_float_next_page_not_rendered_again in tests/draw/test_float.py.
func TestDrawFloatNextPageNotRenderedAgain(t *testing.T) {
	// Regression test for #3003.
	assertPixels(t, "float_next_page_not_rendered_again", `
        RRRR
        RRRR
        RRRR
        RRRR
        ____

        RRaa
        RRaa
        ____
        ____
        ____
    `, `
        <style>
          @page { size: 4px 5px }
          body { color: red; font: 2px / 1 weasyprint }
          p:first-of-type::before { content: counter(page) }
        </style>
        <p>a</p>
        <p>bb</p>
        <p><img style="float: right; width: 2px" src="blue.jpg"></p>
        <p>c</p>`)
}
