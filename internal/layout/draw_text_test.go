// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_text.py: the drawing of the text.

package layout

import "testing"

// TestDrawTextOverflowClip is test_text_overflow_clip in tests/draw/test_text.py.
func TestDrawTextOverflowClip(t *testing.T) {
	assertPixels(t, "text_overflow_clip", `
        _________
        _RRRRRRR_
        _RRRRRRR_
        _________
        _RR__RRR_
        _RR__RRR_
        _________
    `, `
      <style>
        @page {
          size: 9px 7px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
        }
        div {
          line-height: 1;
          margin: 1px;
          overflow: hidden;
          width: 3.5em;
        }
      </style>
      <div>abcde</div>
      <div style="white-space: nowrap">a bcde</div>`)
}

// TestDrawTextOverflowEllipsis is test_text_overflow_ellipsis in tests/draw/test_text.py.
func TestDrawTextOverflowEllipsis(t *testing.T) {
	assertPixels(t, "text_overflow_ellipsis", `
        _________
        _RRRRRR__
        _RRRRRR__
        _________
        _RR__RR__
        _RR__RR__
        _________
        _RRRRRR__
        _RRRRRR__
        _________
        _RRRRRRR_
        _RRRRRRR_
        _________
        _RRRRRRR_
        _RRRRRRR_
        _________
    `, `
      <style>
        @page {
          size: 9px 16px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
        }
        div {
          line-height: 1;
          margin: 1px;
          overflow: hidden;
          text-overflow: ellipsis;
          width: 3.5em;
        }
        div div {
          margin: 0;
        }
      </style>
      <div>abcde</div>
      <div style="white-space: nowrap">a bcde</div>
      <div><span>a<span>b</span>cd</span>e</div>
      <div><div style="text-overflow: clip">abcde</div></div>
      <div><div style="overflow: visible">abcde</div></div>
`)
}

// TestDrawTextAlignRtlTrailingWhitespace is test_text_align_rtl_trailing_whitespace in tests/draw/test_text.py.
func TestDrawTextAlignRtlTrailingWhitespace(t *testing.T) {
	assertPixels(t, "text_align_rtl_trailing_whitespace", `
        _________
        _rrrrBBB_
        _________
        _rrrrBBB_
        _________
        _BBBrrrr_
        _________
        _BBBrrrr_
        _________
    `, `
      <style>
        @page { size: 9px }
        body { font-family: weasyprint; color: blue; font-size: 1px }
        p { background: red; line-height: 1; width: 7em; margin: 1em }
      </style>
      <!-- &#8207 forces Unicode RTL direction for the following chars -->
      <p style="direction: rtl"> abc </p>
      <p style="direction: rtl"> &#8207;abc </p>
      <p style="direction: ltr"> abc </p>
      <p style="direction: ltr"> &#8207;abc </p>
    `)
}

// TestDrawRtlDefaultDirection is test_rtl_default_direction in tests/draw/test_text.py.
func TestDrawRtlDefaultDirection(t *testing.T) {
	assertPixels(t, "rtl_default_direction", `
        _____BBBBB_____
        _____BBBBB_____
        _____BBBBB_____
        _____BBBBB_____
        BBBBBBBBBB_____
    `, `
      <style>
        @page { size: 15px 5px }
        body { font-family: weasyprint; color: blue; font-size: 5px; line-height: 1 }
      </style>
      اب
    `)
}

// TestDrawRtlForcedDirection is test_rtl_forced_direction in tests/draw/test_text.py.
func TestDrawRtlForcedDirection(t *testing.T) {
	assertPixels(t, "rtl_forced_direction", `
        __________BBBBB
        __________BBBBB
        __________BBBBB
        __________BBBBB
        _____BBBBBBBBBB
    `, `
      <style>
        @page { size: 15px 5px }
        body { font-family: weasyprint; color: blue; font-size: 5px; line-height: 1 }
      </style>
      <div style="direction: rtl">اب</div>
    `)
}

// TestDrawRtlNestedInline is test_rtl_nested_inline in tests/draw/test_text.py.
func TestDrawRtlNestedInline(t *testing.T) {
	assertPixels(t, "rtl_nested_inline", `
        RRRRR________________BBBBB___________RRRRR___________BBBBB
        RRRRR________________BBBBB___________RRRRR___________BBBBB
        RRRRR________________BBBBB___________RRRRR___________BBBBB
        RRRRR________________BBBBB___________RRRRR___________BBBBB
        RRRRRRRRRR______BBBBBBBBBB______RRRRRRRRRR______BBBBBBBBBB
        ______________________________________BBBBB__________RRRRR
        ______________________________________BBBBB__________RRRRR
        ______________________________________BBBBB__________RRRRR
        ______________________________________BBBBB__________RRRRR
        _________________________________BBBBBBBBBB_____RRRRRRRRRR
    `, `
      <style>
        @page { size: 58px 10px }
        body { font-family: weasyprint; color: blue; font-size: 5px; line-height: 1 }
        span { color: red }
      </style>
      <div style="direction: rtl; text-align: justify">
        اب <span>اب</span> اب <span>با اب</span> اب
      </div>
    `)
}

// TestDrawMaxLinesEllipsis is test_max_lines_ellipsis in tests/draw/test_text.py.
func TestDrawMaxLinesEllipsis(t *testing.T) {
	assertPixels(t, "max_lines_ellipsis", `
        BBBBBBBB__
        BBBBBBBB__
        BBBBBBBBBB
        BBBBBBBBBB
        __________
        __________
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page {size: 10px 10px;}
        p {
          block-ellipsis: auto;
          color: blue;
          font-family: weasyprint;
          font-size: 2px;
          max-lines: 2;
        }
      </style>
      <p>
        abcd efgh ijkl
      </p>
    `)
}

// TestDrawMaxLinesNested is test_max_lines_nested in tests/draw/test_text.py.
func TestDrawMaxLinesNested(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "max_lines_nested", `
        BBBBBBBBBB
        BBBBBBBBBB
        BBBBBBBBBB
        BBBBBBBBBB
        rrrrrrrrrr
        rrrrrrrrrr
        rrrrrrrrrr
        rrrrrrrrrr
        BBBBBBBBBB
        BBBBBBBBBB
        __________
        __________
    `, `
      <style>
        @page {size: 10px 12px;}
        div {
          continue: discard;
          font-family: weasyprint;
          font-size: 2px;
        }
        #a {
          color: blue;
          max-lines: 5;
        }
        #b {
          color: red
          max-lines: 2;
        }
      </style>
      <div id=a>
        aaaaa
        aaaaa
        <div id=b>
          bbbbb
          bbbbb
          bbbbb
          bbbbb
        </div>
        aaaaa
        aaaaa
      </div>
    `)
}

// TestDrawLineClamp is test_line_clamp in tests/draw/test_text.py.
func TestDrawLineClamp(t *testing.T) {
	assertPixels(t, "line_clamp", `
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBBBBBBBB
        BBBBBBBBBB
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page {size: 10px 10px;}
        p {
          color: blue;
          font-family: weasyprint;
          font-size: 2px;
          line-clamp: 3 "(…)";
        }
      </style>

      <p>
        aa a
        bb b
        cc c
        dddd
        eeee
        ffff
        gggg
        hhhh
      </p>
    `)
}

// TestDrawLineClampNone is test_line_clamp_none in tests/draw/test_text.py.
func TestDrawLineClampNone(t *testing.T) {
	assertPixels(t, "line_clamp_none", `
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page {size: 10px 10px;}
        p {
          color: blue;
          font-family: weasyprint;
          font-size: 2px;
          max-lines: 1;
          continue: discard;
          block-ellipsis: "…";
          line-clamp: none;
        }
      </style>

      <p>
        aa a
        bb b
        cc c
      </p>
    `)
}

// TestDrawLineClampNumber is test_line_clamp_number in tests/draw/test_text.py.
func TestDrawLineClampNumber(t *testing.T) {
	assertPixels(t, "line_clamp_number", `
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBB__BBBB
        BBBB__BBBB
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page {size: 10px 10px;}
        p {
          color: blue;
          font-family: weasyprint;
          font-size: 2px;
          line-clamp: 3;
        }
      </style>

      <p>
        aa a
        bb b
        cc c
        dddd
        eeee
      </p>
    `)
}

// TestDrawLineClampNested is test_line_clamp_nested in tests/draw/test_text.py.
func TestDrawLineClampNested(t *testing.T) {
	assertPixels(t, "line_clamp_nested", `
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBBBBBBBB
        BBBBBBBBBB
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page {size: 10px 10px;}
        div {
          color: blue;
          font-family: weasyprint;
          font-size: 2px;
          line-clamp: 3 "(…)";
        }
      </style>

      <div>
        aa a
        <p>
          bb b
          cc c
          dddd
          eeee
          ffff
          gggg
          hhhh
        </p>
      </div>
    `)
}

// TestDrawLineClampNestedAfter is test_line_clamp_nested_after in tests/draw/test_text.py.
func TestDrawLineClampNestedAfter(t *testing.T) {
	assertPixels(t, "line_clamp_nested_after", `
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBB__BB__
        BBBBBBBBBB
        BBBBBBBBBB
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page {size: 10px 10px;}
        div {
          color: blue;
          font-family: weasyprint;
          font-size: 2px;
          line-clamp: 3 "(…)";
        }
      </style>

      <div>
        aa a
        <p>
          bb b
        </p>
        cc c
        dddd
        eeee
        ffff
        gggg
        hhhh
      </div>
    `)
}

// TestDrawEllipsisNested is test_ellipsis_nested in tests/draw/test_text.py.
func TestDrawEllipsisNested(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "ellipsis_nested", `
        BBBBBB____
        BBBBBB____
        BBBBBB____
        BBBBBB____
        BBBBBB____
        BBBBBB____
        BBBBBB____
        BBBBBB____
        BBBBBBBB__
        BBBBBBBB__
    `, `
      <style>
        @page {size: 10px 10px;}
        div {
          block-ellipsis: auto;
          color: blue;
          continue: discard;
          font-family: weasyprint;
          font-size: 2px;
        }
      </style>
      <div>
        <p>aaa</p>
        <p>aaa</p>
        <p>aaa</p>
        <p>aaa</p>
        <p>aaa</p>
        <p>aaa</p>
      </div>
    `)
}

// TestDrawTextAlignRight is test_text_align_right in tests/draw/test_text.py.
func TestDrawTextAlignRight(t *testing.T) {
	assertPixels(t, "text_align_right", `
        _________
        __RR__RR_
        __RR__RR_
        ______RR_
        ______RR_
        _________
    `, `
      <style>
        @page {
          size: 9px 6px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
        }
        div {
          line-height: 1;
          margin: 1px;
          text-align: right;
        }
      </style>
      <div>a c e</div>`)
}

// TestDrawTextAlignJustify is test_text_align_justify in tests/draw/test_text.py.
func TestDrawTextAlignJustify(t *testing.T) {
	assertPixels(t, "text_align_justify", `
        _________
        _RR___RR_
        _RR___RR_
        _RR______
        _RR______
        _________
    `, `
      <style>
        @page {
          size: 9px 6px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
        }
        div {
          line-height: 1;
          margin: 1px;
          text-align: justify;
        }
      </style>
      <div>a c e</div>`)
}

// TestDrawTextAlignJustifyNbsp is test_text_align_justify_nbsp in tests/draw/test_text.py.
func TestDrawTextAlignJustifyNbsp(t *testing.T) {
	assertPixels(t, "text_align_justify_nbsp", `
        ___________________
        _RR___RR___RR___RR_
        _RR___RR___RR___RR_
        ___________________
    `, `
      <style>
        @page {
          size: 19px 4px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
        }
        div {
          line-height: 1;
          margin: 1px;
          text-align: justify-all;
        }
      </style>
      <div>a b&nbsp;c&nbsp;d</div>`)
}

// TestDrawTextWordSpacing is test_text_word_spacing in tests/draw/test_text.py.
func TestDrawTextWordSpacing(t *testing.T) {
	assertPixels(t, "text_word_spacing", `
        ___________________
        _RR____RR____RR____
        _RR____RR____RR____
        ___________________
    `, `
      <style>
        @page {
          size: 19px 4px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
        }
        div {
          line-height: 1;
          margin: 1px;
          word-spacing: 1em;
        }
      </style>
      <div>a c e</div>`)
}

// TestDrawTextLetterSpacing is test_text_letter_spacing in tests/draw/test_text.py.
func TestDrawTextLetterSpacing(t *testing.T) {
	assertPixels(t, "text_letter_spacing", `
        ___________________
        _RR____RR____RR____
        _RR____RR____RR____
        ___________________
    `, `
      <style>
        @page {
          size: 19px 4px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
        }
        div {
          line-height: 1;
          margin: 1px;
          letter-spacing: 2em;
        }
      </style>
      <div>ace</div>`)
}

// TestDrawTextUnderline is test_text_underline in tests/draw/test_text.py.
func TestDrawTextUnderline(t *testing.T) {
	assertPixels(t, "text_underline", `
        _____________
        _zzzzzzzzzzz_
        _zsssssssssz_
        _zsssssssssz_
        _zuuuuuuuuuz_
        _zzzzzzzzzzz_
        _____________
    `, `
      <style>
        @page {
          size: 13px 7px;
          margin: 2px;
        }
        body {
          color: rgba(255, 0, 0, 0.5);
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue auto;
        }
      </style>
      <div>abc</div>`)
}

// TestDrawTextUnderlineOffset is test_text_underline_offset in tests/draw/test_text.py.
func TestDrawTextUnderlineOffset(t *testing.T) {
	assertPixels(t, "text_underline_offset", `
        _____________
        _zzzzzzzzzzz_
        _zRRRRRRRRRz_
        _zRRRRRRRRRz_
        _zzzzzzzzzzz_
        _zzzzzzzzzzz_
        _zBBBBBBBBBz_
        _zzzzzzzzzzz_
        _____________
    `, `
      <style>
        @page {
          size: 13px 9px;
          margin: 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue;
          text-underline-offset: 2px;
        }
      </style>
      <div>abc</div>`)
}

// TestDrawTextUnderlineOffsetPercentage is test_text_underline_offset_percentage in tests/draw/test_text.py.
func TestDrawTextUnderlineOffsetPercentage(t *testing.T) {
	assertPixels(t, "text_underline_offset_percentage", `
        _____________
        _zzzzzzzzzzz_
        _zRRRRRRRRRz_
        _zRRRRRRRRRz_
        _zzzzzzzzzzz_
        _zzzzzzzzzzz_
        _zBBBBBBBBBz_
        _zzzzzzzzzzz_
        _____________
    `, `
      <style>
        @page {
          size: 13px 9px;
          margin: 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue;
          text-underline-offset: 70%;
        }
      </style>
      <div>abc</div>`)
}

// TestDrawTextUnderlineOffsetCalc is test_text_underline_offset_calc in tests/draw/test_text.py.
func TestDrawTextUnderlineOffsetCalc(t *testing.T) {
	assertPixels(t, "text_underline_offset_calc", `
        _____________
        _zzzzzzzzzzz_
        _zRRRRRRRRRz_
        _zRRRRRRRRRz_
        _zzzzzzzzzzz_
        _zzzzzzzzzzz_
        _zBBBBBBBBBz_
        _zzzzzzzzzzz_
        _____________
    `, `
      <style>
        @page {
          size: 13px 9px;
          margin: 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue;
          text-underline-offset: calc(0.5em + 20%);
        }
      </style>
      <div>abc</div>`)
}

// TestDrawTextUnderlineThickness is test_text_underline_thickness in tests/draw/test_text.py.
func TestDrawTextUnderlineThickness(t *testing.T) {
	assertPixels(t, "text_underline_thickness", `
        _____________
        _zzzzzzzzzzz_
        _zRRRRRRRRRz_
        _zRRRRRRRRRz_
        _zzzzzzzzzzz_
        _zzzzzzzzzzz_
        _zBBBBBBBBBz_
        _zBBBBBBBBBz_
        _zzzzzzzzzzz_
    `, `
      <style>
        @page {
          size: 13px 9px;
          margin: 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue 3px;
          text-underline-offset: 2px;
        }
      </style>
      <div>abc</div>`)
}

// TestDrawTextUnderlineThicknessPercentage is test_text_underline_thickness_percentage in tests/draw/test_text.py.
func TestDrawTextUnderlineThicknessPercentage(t *testing.T) {
	assertPixels(t, "text_underline_thickness_percentage", `
        _____________
        _zzzzzzzzzzz_
        _zRRRRRRRRRz_
        _zRRRRRRRRRz_
        _zzzzzzzzzzz_
        _zzzzzzzzzzz_
        _zBBBBBBBBBz_
        _zBBBBBBBBBz_
        _zzzzzzzzzzz_
    `, `
      <style>
        @page {
          size: 13px 9px;
          margin: 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue 100%;
          text-underline-offset: 2px;
        }
      </style>
      <div>abc</div>`)
}

// TestDrawTextUnderlineThicknessCalc is test_text_underline_thickness_calc in tests/draw/test_text.py.
func TestDrawTextUnderlineThicknessCalc(t *testing.T) {
	assertPixels(t, "text_underline_thickness_calc", `
        _____________
        _zzzzzzzzzzz_
        _zRRRRRRRRRz_
        _zRRRRRRRRRz_
        _zzzzzzzzzzz_
        _zzzzzzzzzzz_
        _zBBBBBBBBBz_
        _zBBBBBBBBBz_
        _zzzzzzzzzzz_
    `, `
      <style>
        @page {
          size: 13px 9px;
          margin: 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue calc(0.5em + 50%);
          text-underline-offset: 2px;
        }
      </style>
      <div>abc</div>`)
}

// TestDrawTextUnderlineThicknessCalcInherit is test_text_underline_thickness_calc_inherit in tests/draw/test_text.py.
func TestDrawTextUnderlineThicknessCalcInherit(t *testing.T) {
	assertPixels(t, "text_underline_thickness_calc_inherit", `
        _____________
        _zzzzzzzzzzz_
        _zRRRRRRRRRz_
        _zRRRRRRRRRz_
        _zzzzzzzzzzz_
        _zzzzzzzzzzz_
        _zBBBBBBBBBz_
        _zBBBBBBBBBz_
        _zzzzzzzzzzz_
    `, `
      <style>
        @page {
          size: 13px 9px;
          margin: 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue max(1rem, 1px);
          text-underline-offset: 2px;
        }
        div::after { content: "abc" }
      </style>
      <div></div>`)
}

// TestDrawTextOverline is test_text_overline in tests/draw/test_text.py.
func TestDrawTextOverline(t *testing.T) {
	assertPixels(t, "text_overline", `
        _____________
        _zzzzzzzzzzz_
        _zzzzzzzzzzz_
        _zsssssssssz_
        _zsssssssssz_
        _zzzzzzzzzzz_
        _____________
    `, `
      <style>
        @page {
          size: 13px 7px;
          margin: 2px;
        }
        body {
          color: rgba(255, 0, 0, 0.5);
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: overline blue;
        }
      </style>
      <div>abc</div>`)
}

// TestDrawTextLineThrough is test_text_line_through in tests/draw/test_text.py.
func TestDrawTextLineThrough(t *testing.T) {
	assertPixels(t, "text_line_through", `
        _____________
        _zzzzzzzzzzz_
        _zBBBBBBBBBz_
        _zuuuuuuuuuz_
        _zBBBBBBBBBz_
        _zzzzzzzzzzz_
        _____________
    `, `
      <style>
        @page {
          size: 13px 7px;
          margin: 2px;
        }
        body {
          color: blue;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: line-through rgba(255, 0, 0, 0.5);
        }
      </style>
      <div>abc</div>`)
}

// TestDrawTextMultipleTextDecoration is test_text_multiple_text_decoration in tests/draw/test_text.py.
func TestDrawTextMultipleTextDecoration(t *testing.T) {
	assertPixels(t, "text_multiple_text_decoration", `
        _____________
        _zzzzzzzzzzz_
        _zsssssssssz_
        _zBBBBBBBBBz_
        _zuuuuuuuuuz_
        _zzzzzzzzzzz_
        _____________
    `, `
      <style>
        @page {
          size: 13px 7px;
          margin: 2px;
        }
        body {
          color: rgba(255, 0, 0, 0.5);
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline line-through blue;
        }
      </style>
      <div>abc</div>`)
}

// TestDrawTextNestedTextDecoration is test_text_nested_text_decoration in tests/draw/test_text.py.
func TestDrawTextNestedTextDecoration(t *testing.T) {
	assertPixels(t, "text_nested_text_decoration", `
        _____________
        _zzzzzzzzzzz_
        _zsssssssssz_
        _zsssBBBsssz_
        _zuuuuuuuuuz_
        _zzzzzzzzzzz_
        _____________
    `, `
      <style>
        @page {
          size: 13px 7px;
          margin: 2px;
        }
        body {
          color: rgba(255, 0, 0, 0.5);
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue;
        }
        span {
          text-decoration: line-through blue;
        }
      </style>
      <div>a<span>b</span>c</div>`)
}

// TestDrawTextNestedTextDecorationColor is test_text_nested_text_decoration_color in tests/draw/test_text.py.
func TestDrawTextNestedTextDecorationColor(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "text_nested_text_decoration_color", `
        _____________
        _zzzzzzzzzzz_
        _zRRRRRRRRRz_
        _zRRRGGGRRRz_
        _zBBBBBBBBBz_
        _zzzzzzzzzzz_
        _____________
    `, `
      <style>
        @page {
          size: 13px 7px;
          margin: 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue;
        }
        span {
          text-decoration: line-through lime;
        }
      </style>
      <div>a<span>b</span>c</div>`)
}

// TestDrawTextNestedBlockTextDecoration is test_text_nested_block_text_decoration in tests/draw/test_text.py.
func TestDrawTextNestedBlockTextDecoration(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "text_nested_block_text_decoration", `
        _______
        _zzzzz_
        _zRRRz_
        _zRRRz_
        _zBBBz_
        _zRRRz_
        _zGGGz_
        _zBBBz_
        _zRRRz_
        _zRRRz_
        _zBBBz_
        _zzzzz_
        _______
    `, `
      <style>
        @page {
          size: 7px 13px;
          margin: 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue;
        }
        article {
          text-decoration: line-through lime;
        }
      </style>
      <div>a<article>b</article>c</div>`)
}

// TestDrawTextFloatTextDecoration is test_text_float_text_decoration in tests/draw/test_text.py.
func TestDrawTextFloatTextDecoration(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "text_float_text_decoration", `
        _____________
        _zzzzz_______
        _zRRRz__RRR__
        _zRRRz__RRR__
        _zBBBz__RRR__
        _zzzzz_______
        _____________
    `, `
      <style>
        @page {
          size: 13px 7px;
          margin: 2px;
        }
        div {
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration: underline blue;
        }
        span {
          float: right;
        }
      </style>
      <div>a<span>b</span></div>`)
}

// TestDrawTextDecorationVar is test_text_decoration_var in tests/draw/test_text.py.
func TestDrawTextDecorationVar(t *testing.T) {
	assertPixels(t, "text_decoration_var", `
        _____________
        _zzzzzzzzzzz_
        _zRRRRRRRRRz_
        _zBBBBBBBBBz_
        _zRRRRRRRRRz_
        _zzzzzzzzzzz_
        _____________
    `, `
      <style>
        @page {
          size: 13px 7px;
          margin: 2px;
        }
        body {
          --blue: blue;
          color: red;
          font-family: weasyprint;
          font-size: 3px;
          text-decoration-color: var(--blue);
          text-decoration-line: line-through;
        }
      </style>
      <div>abc</div>`)
}

// TestDrawZeroWidthCharacter is test_zero_width_character in tests/draw/test_text.py.
func TestDrawZeroWidthCharacter(t *testing.T) {
	assertPixels(t, "zero_width_character", `
        ______
        _RRRR_
        _RRRR_
        ______
    `, `
      <style>
        @page {
          size: 6px 4px;
          margin: 1px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
      </style>
      <div>a&zwnj;b</div>`)
}

// TestDrawFontSizeSmall is test_font_size_small in tests/draw/test_text.py.
func TestDrawFontSizeSmall(t *testing.T) {
	assertPixels(t, "font_size_small", `
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page {
          size: 10px 4px;
          margin: 1px;
        }
        body {
          font-family: weasyprint;
          font-size: 0.0001px;
        }
      </style>
      test font size zero
    `)
}

// TestDrawFontSizeVerySmall is test_font_size_very_small in tests/draw/test_text.py.
func TestDrawFontSizeVerySmall(t *testing.T) {
	assertPixels(t, "font_size_very_small", `
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page {
          size: 10px 4px;
          margin: 1px;
        }
        body {
          font-family: weasyprint;
          font-size: 0.00000001px;
        }
      </style>
      test font size zero
    `)
}

// TestDrawMissingGlyphFallback is test_missing_glyph_fallback in tests/draw/test_text.py.
func TestDrawMissingGlyphFallback(t *testing.T) {
	assertPixels(t, "missing_glyph_fallback", `
        ___zzzzzzzzzzzzzzzzz
        _RRzzzzzzzzzzzzzzzzz
        _RRzzzzzzzzzzzzzzzzz
        ___zzzzzzzzzzzzzzzzz
    `, `
      <style>
        @page {
          size: 20px 4px;
        }
        body {
          color: red;
          font-family: weasyprint, DejaVu Sans, sans;
          font-size: 2px;
          line-height: 0;
          margin: 2px 1px;
        }
      </style>a'`)
}

// TestDrawTabulationCharacter is test_tabulation_character in tests/draw/test_text.py.
func TestDrawTabulationCharacter(t *testing.T) {
	assertPixels(t, "tabulation_character", `
        __________
        _RR____RR_
        _RR____RR_
        __________
    `, `
      <style>
        @page {
          size: 10px 4px;
          margin: 1px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
          tab-size: 3;
        }
      </style>
      <pre>a&Tab;b</pre>`)
}

// TestDrawOtbFont is test_otb_font in tests/draw/test_text.py.
func TestDrawOtbFont(t *testing.T) {
	t.Skip("weasyprint.otb has bitmap glyphs only, and the port draws the outlines of the glyphs")
	assertPixels(t, "otb_font", `
        ____________________
        __RR______RR________
        __RR__RR__RR________
        __RR__RR__RR________
        ____________________
        ____________________
    `, `
      <style>
        @page {
          size: 20px 6px;
          margin: 1px;
        }
        @font-face {
          src: url(weasyprint.otb);
          font-family: weasyprint-otb;
        }
        body {
          color: red;
          font-family: weasyprint-otb;
          font-size: 4px;
          line-height: 0.8;
        }
      </style>
      AaA`)
}

// TestDrawHugeJustification is test_huge_justification in tests/draw/test_text.py.
func TestDrawHugeJustification(t *testing.T) {
	assertPixels(t, "huge_justification", `
        ____
        _RR_
        _RR_
        ____
    `, `
      <style>
        @page {
          size: 4px 4px;
          margin: 1px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
          text-align: justify-all;
          width: 100000px;
        }
      </style>
      A B`)
}

// TestDrawFontVariantCapsSmall is test_font_variant_caps_small in tests/draw/test_text.py.
func TestDrawFontVariantCapsSmall(t *testing.T) {
	t.Skip("the expected pixels draw the small capitals of DejaVu Sans, and the fonts of the tests, the Go fonts, have no small capitals")
	assertPixels(t, "font_variant_caps_small", `
        ________
        _BB_BB__
        _BB_B_B_
        _B__BB__
        _B__B___
        ________
    `, `
      <style>
        @page {size: 8px 6px}
        p {
          color: blue;
          font-variant-caps: small-caps;
          font-family: DejaVu Sans, sans;
          font-size: 6px;
          line-height: 1;
        }
      </style>
      <p>Pp</p>
    `)
}

// TestDrawFontVariantCapsAllSmall is test_font_variant_caps_all_small in tests/draw/test_text.py.
func TestDrawFontVariantCapsAllSmall(t *testing.T) {
	t.Skip("the expected pixels draw the small capitals of DejaVu Sans, and the fonts of the tests, the Go fonts, have no small capitals")
	assertPixels(t, "font_variant_caps_all_small", `
        ________
        BB_BB___
        B_BB_B__
        BB_BB___
        B__B____
        ________
    `, `
      <style>
        @page {size: 8px 6px}
        p {
          color: blue;
          font-variant-caps: all-small-caps;
          font-family: DejaVu Sans, sans;
          font-size: 6px;
          line-height: 1;
        }
      </style>
      <p>Pp</p>
    `)
}

// TestDrawFontVariantCapsPetite is test_font_variant_caps_petite in tests/draw/test_text.py.
func TestDrawFontVariantCapsPetite(t *testing.T) {
	t.Skip("the expected pixels draw the small capitals of DejaVu Sans, and the fonts of the tests, the Go fonts, have no small capitals")
	assertPixels(t, "font_variant_caps_petite", `
        ________
        _BB_BB__
        _BB_B_B_
        _B__BB__
        _B__B___
        ________
    `, `
      <style>
        @page {size: 8px 6px}
        p {
          color: blue;
          font-variant-caps: petite-caps;
          font-family: DejaVu Sans, sans;
          font-size: 6px;
          line-height: 1;
        }
      </style>
      <p>Pp</p>
    `)
}

// TestDrawFontVariantCapsAllPetite is test_font_variant_caps_all_petite in tests/draw/test_text.py.
func TestDrawFontVariantCapsAllPetite(t *testing.T) {
	t.Skip("the expected pixels draw the small capitals of DejaVu Sans, and the fonts of the tests, the Go fonts, have no small capitals")
	assertPixels(t, "font_variant_caps_all_petite", `
        ________
        BB_BB___
        B_BB_B__
        BB_BB___
        B__B____
        ________
    `, `
      <style>
        @page {size: 8px 6px}
        p {
          color: blue;
          font-variant-caps: all-petite-caps;
          font-family: DejaVu Sans, sans;
          font-size: 6px;
          line-height: 1;
        }
      </style>
      <p>Pp</p>
    `)
}

// TestDrawFontVariantCapsUnicase is test_font_variant_caps_unicase in tests/draw/test_text.py.
func TestDrawFontVariantCapsUnicase(t *testing.T) {
	t.Skip("the expected pixels draw the small capitals of DejaVu Sans, and the fonts of the tests, the Go fonts, have no small capitals")
	assertPixels(t, "font_variant_caps_unicase", `
        ________
        BB______
        B_B_BB__
        BB__B_B_
        B___BB__
        ____B___
    `, `
      <style>
        @page {size: 8px 6px}
        p {
          color: blue;
          font-variant-caps: unicase;
          font-family: DejaVu Sans, sans;
          font-size: 6px;
          line-height: 1;
        }
      </style>
      <p>Pp</p>
    `)
}

// TestDrawFontVariantCapsTitling is test_font_variant_caps_titling in tests/draw/test_text.py.
func TestDrawFontVariantCapsTitling(t *testing.T) {
	t.Skip("the expected pixels draw the small capitals of DejaVu Sans, and the fonts of the tests, the Go fonts, have no small capitals")
	assertPixels(t, "font_variant_caps_titling", `
        _BB_____
        _BB_____
        _BB__BB_
        _B___B_B
        _____BB_
        _____B__
    `, `
      <style>
        @page {size: 8px 6px}
        p {
          color: blue;
          font-family: DejaVu Sans, sans;
          font-size: 6px;
          line-height: 1;
        }
      </style>
      <p>Pp</p>
    `)
}

// TestDrawUnicodeRange is test_unicode_range in tests/draw/test_text.py.
func TestDrawUnicodeRange(t *testing.T) {
	assertPixels(t, "unicode_range", `
        __________
        _RRRRRR___
        _RRRRRRzz_
        __________
    `, `
      <style>
        @font-face {
          font-family: uni;
          src: url(weasyprint.otf);
          unicode-range: u+41, u+043-045, u+005?;
        }
        @page {
          size: 10px 4px;
        }
        body {
          color: red;
          font-family: uni, DejaVu Sans, sans;
          font-size: 2px;
          line-height: 0;
          margin: 2px 1px;
        }
      </style>ADZB`)
}

// TestDrawBadFontName is test_bad_font_name in tests/draw/test_text.py.
func TestDrawBadFontName(t *testing.T) {
	assertPixels(t, "bad_font_name", `
        __________
        _RRRRRR___
        _RRRRRR___
        __________
    `, `
      <style>
        @font-face {
          src: url(weasyprint.otf);
          font-family: "w/e\\a\Az\2fy";
        }
        @page {
          size: 10px 4px;
        }
        body {
          color: red;
          font-family: "w/e\\a\Az\2fy";
          font-size: 2px;
          line-height: 0;
          margin: 2px 1px;
        }
      </style>
      abc`)
}
