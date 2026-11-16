// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_leader.py: the drawing of the leaders.

package layout

import "testing"

// TestDrawLeaderSimple is test_leader_simple in tests/draw/test_leader.py.
func TestDrawLeaderSimple(t *testing.T) {
	assertPixels(t, "leader_simple", `
        RR__BBBBBBBB__BB
        RR__BBBBBBBB__BB
        RRRR__BBBB__BBBB
        RRRR__BBBB__BBBB
        RR__BBBB__BBBBBB
        RR__BBBB__BBBBBB
    `, `
      <style>
        @page {
          size: 16px 6px;
        }
        body {
          color: red;
          counter-reset: count;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        div::after {
          color: blue;
          content: ' ' leader(dotted) ' ' counter(count, lower-roman);
          counter-increment: count;
        }
      </style>
      <div>a</div>
      <div>bb</div>
      <div>c</div>
    `)
}

// TestDrawLeaderTooLong is test_leader_too_long in tests/draw/test_leader.py.
func TestDrawLeaderTooLong(t *testing.T) {
	assertPixels(t, "leader_too_long", `
        RRRRRRRRRR______
        RRRRRRRRRR______
        BBBBBBBBBBBB__BB
        BBBBBBBBBBBB__BB
        RR__RR__RR__RR__
        RR__RR__RR__RR__
        RR__RR__RR______
        RR__RR__RR______
        BBBBBBBBBB__BBBB
        BBBBBBBBBB__BBBB
        RR__RR__RR__RR__
        RR__RR__RR__RR__
        RR__BBBB__BBBBBB
        RR__BBBB__BBBBBB
    `, `
      <style>
        @page {
          size: 16px 14px;
        }
        body {
          color: red;
          counter-reset: count;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        div::after {
          color: blue;
          content: ' ' leader(dotted) ' ' counter(count, lower-roman);
          counter-increment: count;
        }
      </style>
      <div>aaaaa</div>
      <div>a a a a a a a</div>
      <div>a a a a a</div>
    `)
}

// TestDrawLeaderAlone is test_leader_alone in tests/draw/test_leader.py.
func TestDrawLeaderAlone(t *testing.T) {
	assertPixels(t, "leader_alone", `
        RRBBBBBBBBBBBBBB
        RRBBBBBBBBBBBBBB
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
        div::after {
          color: blue;
          content: leader(dotted);
        }
      </style>
      <div>a</div>
    `)
}

// TestDrawLeaderContent is test_leader_content in tests/draw/test_leader.py.
func TestDrawLeaderContent(t *testing.T) {
	assertPixels(t, "leader_content", `
        RR____BB______BB
        RR____BB______BB
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
        div::after {
          color: blue;
          content: leader(' . ') 'a';
        }
      </style>
      <div>a</div>
    `)
}

// TestDrawLeaderFloat is test_leader_float in tests/draw/test_leader.py.
func TestDrawLeaderFloat(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "leader_float", `
        bbGRR___BB____BB
        bbGRR___BB____BB
        GGGRR___BB____BB
        ___RR___BB____BB
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
        article {
          background: lime;
          color: navy;
          float: left;
          height: 3px;
          width: 3px;
        }
        div::after {
          color: blue;
          content: leader('. ') 'a';
        }
      </style>
      <div>a<article>a</article></div>
      <div>a</div>
    `)
}

// TestDrawLeaderFloatSmall is test_leader_float_small in tests/draw/test_leader.py.
func TestDrawLeaderFloatSmall(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "leader_float_small", `
        bbRRBB__BB____BB
        bbRRBB__BB____BB
        RR__BB__BB____BB
        RR__BB__BB____BB
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
        article {
          background: lime;
          color: navy;
          float: left;
        }
        div::after {
          color: blue;
          content: leader('. ') 'a';
        }
      </style>
      <div>a<article>a</article></div>
      <div>a</div>
    `)
}

// TestDrawLeaderInInline is test_leader_in_inline in tests/draw/test_leader.py.
func TestDrawLeaderInInline(t *testing.T) {
	assertPixels(t, "leader_in_inline", `
        RR__GGBBBBBB__RR
        RR__GGBBBBBB__RR
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
        span {
          color: lime;
        }
        span::after {
          color: blue;
          content: leader('-');
        }
      </style>
      <div>a <span>a</span> a</div>
    `)
}

// TestDrawLeaderBadAlignment is test_leader_bad_alignment in tests/draw/test_leader.py.
func TestDrawLeaderBadAlignment(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "leader_bad_alignment", `
        RRRRRR__________
        RRRRRR__________
        ______BB______RR
        ______BB______RR
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
        div::after {
          color: blue;
          content: leader(' - ') 'a';
        }
      </style>
      <div>aaa</div>
    `)
}

// TestDrawLeaderSimpleRtl is test_leader_simple_rtl in tests/draw/test_leader.py.
func TestDrawLeaderSimpleRtl(t *testing.T) {
	assertPixels(t, "leader_simple_rtl", `
        BB__BBBBBBBB__RR
        BB__BBBBBBBB__RR
        BBBB__BBBB__RRRR
        BBBB__BBBB__RRRR
        BBBBBB__BBBB__RR
        BBBBBB__BBBB__RR
    `, `
      <style>
        @page {
          size: 16px 6px;
        }
        body {
          color: red;
          counter-reset: count;
          direction: rtl;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        div::after {
          color: blue;
          /* RTL Mark used in second space */
          content: ' ' leader(dotted) '\200f ' counter(count, lower-roman);
          counter-increment: count;
        }
      </style>
      <div>a</div>
      <div>bb</div>
      <div>c</div>
    `)
}

// TestDrawLeaderTooLongRtl is test_leader_too_long_rtl in tests/draw/test_leader.py.
func TestDrawLeaderTooLongRtl(t *testing.T) {
	assertPixels(t, "leader_too_long_rtl", `
        ______RRRRRRRRRR
        ______RRRRRRRRRR
        BB__BBBBBBBBBBBB
        BB__BBBBBBBBBBBB
        __RR__RR__RR__RR
        __RR__RR__RR__RR
        ______RR__RR__RR
        ______RR__RR__RR
        BBBB__BBBBBBBBBB
        BBBB__BBBBBBBBBB
        __RR__RR__RR__RR
        __RR__RR__RR__RR
        BBBBBB__BBBB__RR
        BBBBBB__BBBB__RR
    `, `
      <style>
        @page {
          size: 16px 14px;
        }
        body {
          color: red;
          counter-reset: count;
          direction: rtl;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        div::after {
          color: blue;
          /* RTL Mark used in second space */
          content: ' ' leader(dotted) '\200f ' counter(count, lower-roman);
          counter-increment: count;
        }
      </style>
      <div>aaaaa</div>
      <div>a a a a a a a</div>
      <div>a a a a a</div>
    `)
}

// TestDrawLeaderFloatLeader is test_leader_float_leader in tests/draw/test_leader.py.
func TestDrawLeaderFloatLeader(t *testing.T) {
	assertPixels(t, "leader_float_leader", `
        RR____________BB
        RR____________BB
        RRRR__________BB
        RRRR__________BB
        RR____________BB
        RR____________BB
    `, `
      <style>
        @page {
          size: 16px 6px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        div::after {
          color: blue;
          content: leader(' . ') 'a';
          float: right;
        }
      </style>
      <div>a</div>
      <div>bb</div>
      <div>c</div>
    `)
}

// TestDrawLeaderEmptyString is test_leader_empty_string in tests/draw/test_leader.py.
func TestDrawLeaderEmptyString(t *testing.T) {
	assertPixels(t, "leader_empty_string", `
        RRRR____
        ________
    `, `
      <style>
        @page {
          size: 8px 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 1px;
          line-height: 1;
        }
        div::after {
          color: blue;
          content: leader('');
        }
      </style>
      <div>aaaa</div>
    `)
}

// TestDrawLeaderZeroWidthString is test_leader_zero_width_string in tests/draw/test_leader.py.
func TestDrawLeaderZeroWidthString(t *testing.T) {
	assertPixels(t, "leader_zero_width_string", `
        RRRR____
        ________
    `, `
      <style>
        @page {
          size: 8px 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 1px;
          line-height: 1;
        }
        div::after {
          color: blue;
          content: leader('\200b');  /* zero-width space */
        }
      </style>
      <div>aaaa</div>
    `)
}

// TestDrawLeaderAbsolute is test_leader_absolute in tests/draw/test_leader.py.
func TestDrawLeaderAbsolute(t *testing.T) {
	assertPixels(t, "leader_absolute", `
        BBBBRRRR
        ______GG
    `, `
      <style>
        @page {
          size: 8px 2px;
        }
        body {
          color: red;
          font-family: weasyprint;
          font-size: 1px;
          line-height: 1;
        }
        div::before {
          color: blue;
          content: leader('z');
        }
        article {
          bottom: 0;
          color: lime;
          position: absolute;
          right: 0;
        }
      </style>
      <div>aa<article>bb</article>aa</div>
    `)
}

// TestDrawLeaderPadding is test_leader_padding in tests/draw/test_leader.py.
func TestDrawLeaderPadding(t *testing.T) {
	assertPixels(t, "leader_padding", `
        RR__BBBBBBBB__BB
        RR__BBBBBBBB__BB
        __RR__BBBB__BBBB
        __RR__BBBB__BBBB
    `, `
      <style>
        @page {
          size: 16px 4px;
        }
        body {
          color: red;
          counter-reset: count;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        div::after {
          color: blue;
          content: ' ' leader(dotted) ' ' counter(count, lower-roman);
          counter-increment: count;
        }
        div + div {
          padding-left: 2px;
        }
      </style>
      <div>a</div>
      <div>b</div>
    `)
}

// TestDrawLeaderInlinePadding is test_leader_inline_padding in tests/draw/test_leader.py.
func TestDrawLeaderInlinePadding(t *testing.T) {
	assertPixels(t, "leader_inline_padding", `
        RR__BBBBBBBB__BB
        RR__BBBBBBBB__BB
        __RR__BBBB__BBBB
        __RR__BBBB__BBBB
    `, `
      <style>
        @page {
          size: 16px 4px;
        }
        body {
          color: red;
          counter-reset: count;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        span::after {
          color: blue;
          content: ' ' leader(dotted) ' ' counter(count, lower-roman);
          counter-increment: count;
        }
        div + div span {
          padding-left: 2px;
        }
      </style>
      <div><span>a</span></div>
      <div><span>b</span></div>
    `)
}

// TestDrawLeaderMargin is test_leader_margin in tests/draw/test_leader.py.
func TestDrawLeaderMargin(t *testing.T) {
	assertPixels(t, "leader_margin", `
        RR__BBBBBBBB__BB
        RR__BBBBBBBB__BB
        __RR__BBBB__BBBB
        __RR__BBBB__BBBB
    `, `
      <style>
        @page {
          size: 16px 4px;
        }
        body {
          color: red;
          counter-reset: count;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        div::after {
          color: blue;
          content: ' ' leader(dotted) ' ' counter(count, lower-roman);
          counter-increment: count;
        }
        div + div {
          margin-left: 2px;
        }
      </style>
      <div>a</div>
      <div>b</div>
    `)
}

// TestDrawLeaderInlineMargin is test_leader_inline_margin in tests/draw/test_leader.py.
func TestDrawLeaderInlineMargin(t *testing.T) {
	assertPixels(t, "leader_inline_margin", `
        RR__BBBBBBBB__BB
        RR__BBBBBBBB__BB
        __RR__BBBB__BBBB
        __RR__BBBB__BBBB
    `, `
      <style>
        @page {
          size: 16px 4px;
        }
        body {
          color: red;
          counter-reset: count;
          font-family: weasyprint;
          font-size: 2px;
          line-height: 1;
        }
        span::after {
          color: blue;
          content: ' ' leader(dotted) ' ' counter(count, lower-roman);
          counter-increment: count;
        }
        div + div span {
          margin-left: 2px;
        }
      </style>
      <div><span>a</span></div>
      <div><span>b</span></div>
    `)
}
