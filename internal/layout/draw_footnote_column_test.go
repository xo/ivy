// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_footnote_column.py: the drawing of the footnotes in columns.

package layout

import "testing"

// TestDrawFootnoteColumnMarginTop is test_footnote_column_margin_top in tests/draw/test_footnote_column.py.
func TestDrawFootnoteColumnMarginTop(t *testing.T) {
	assertPixels(t, "footnote_column_margin_top", `
        RRRR_RRRR
        RRRR_RRRR
        _________
        _________
        _________
        RRRRRRRR_
        RRRRRRRR_
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        RRRR_____
        RRRR_____
        _________
    `, `
    <style>
      @page {
        size: 9px 7px;
        @footnote {
          margin-top: 2Px;
        }
      }
      div {
        color: red;
        columns: 2;
        column-gap: 1px;
        font-family: weasyprint;
        font-size: 2px;
        line-height: 1;
      }
      span {
        float: footnote;
      }
    </style>
    <div>a<span>de</span> ab ab ab ab ab ab</div>`)
}

// TestDrawFootnoteColumnFillAuto is test_footnote_column_fill_auto in tests/draw/test_footnote_column.py.
func TestDrawFootnoteColumnFillAuto(t *testing.T) {
	assertPixels(t, "footnote_column_fill_auto", `
        RRRR_____
        RRRR_____
        RRRR_____
        RRRR_____
        RRRR_____
        RRRR_____
        _________
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
    `, `
    <style>
      @page {
        size: 9px 13px;
      }
      div {
        color: red;
        columns: 2;
        column-fill: auto;
        column-gap: 1px;
        font-family: weasyprint;
        font-size: 2px;
        line-height: 1;
      }
      span {
        float: footnote;
      }
    </style>
    <div>a<span>de</span> a<span>de</span> a<span>de</span></div>`)
}

// TestDrawFootnoteColumnFillAutoBreakInsideAvoid is test_footnote_column_fill_auto_break_inside_avoid in tests/draw/test_footnote_column.py.
func TestDrawFootnoteColumnFillAutoBreakInsideAvoid(t *testing.T) {
	assertPixels(t, "footnote_column_fill_auto_break_inside_avoid", `
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        _________
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
    `, `
    <style>
      @page {
        size: 9px 13px;
      }
      div {
        color: red;
        columns: 2;
        column-fill: auto;
        column-gap: 1px;
        font-family: weasyprint;
        font-size: 2pX;
        line-height: 1;
      }
      article {
        break-inside: avoid;
      }
      span {
        float: footnote;
      }
    </style>
    <div>
      <article>a<span>de</span> a<span>de</span></article>
      <article>ab</article>
      <article>a<span>de</span> ab</article>
      <article>ab</article>
    </div>`)
}

// TestDrawFootnoteColumnPAfter is test_footnote_column_p_after in tests/draw/test_footnote_column.py.
func TestDrawFootnoteColumnPAfter(t *testing.T) {
	assertPixels(t, "footnote_column_p_after", `
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        KK__KK___
        KK__KK___
        _________
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        KK__KK___
        KK__KK___
        _________
        _________
        _________
        _________
        _________
        _________
        _________
        _________
        _________
    `, `
    <style>
      @page {
        size: 9px 11px;
      }
      body {
        font-family: weasyprint;
        font-size: 2px;
        line-height: 1;
      }
      div {
        color: red;
        columns: 2;
        column-gap: 1px;
      }
      span {
        float: footnote;
      }
    </style>
    <div>a<span>de</span> a<span>de</span> ab ab</div>
    <p>a a a a</p>`)
}

// TestDrawFootnoteColumnPBefore is test_footnote_column_p_before in tests/draw/test_footnote_column.py.
func TestDrawFootnoteColumnPBefore(t *testing.T) {
	assertPixels(t, "footnote_column_p_before", `
        KKKK_____
        KKKK_____
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RR__
        RRRR_RR__
        _________
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRR_RR__
        RRRR_RR__
        _________
        _________
        _________
        _________
        _________
        _________
        _________
        _________
        _________
        _________
        _________
    `, `
    <style>
      @page {
        size: 9px 13px;
      }
      body {
        font-family: weasyprint;
        font-size: 2px;
        line-height: 1;
      }
      div {
        color: red;
        columns: 2;
        column-gap: 1px;
      }
      span {
        float: footnote;
      }
    </style>
    <p>ab</p>
    <div>
    a<span>de</span> a<span>de</span>
    a<span>de</span> a ab a </div>`)
}

// TestDrawFootnoteColumn3 is test_footnote_column_3 in tests/draw/test_footnote_column.py.
func TestDrawFootnoteColumn3(t *testing.T) {
	assertPixels(t, "footnote_column_3", `
        RRRR_RRRR_RRRR
        RRRR_RRRR_RRRR
        ______________
        RRRRRRRR______
        RRRRRRRR______
        RRRR_RRRR_____
        RRRR_RRRR_____
        ______________
        ______________
        ______________
    `, `
    <style>
      @page {
        size: 14px 5px;
      }
      body {
        font-family: weasyprint;
        font-size: 2px;
        line-height: 1;
      }
      div {
        color: red;
        columns: 3;
        column-gap: 1Px;
      }
      span {
        float: footnote;
      }
    </style>
    <div>ab ab a<span>de</span> ab ab </div>`)
}

// TestDrawFootnoteColumn3PBefore is test_footnote_column_3_p_before in tests/draw/test_footnote_column.py.
func TestDrawFootnoteColumn3PBefore(t *testing.T) {
	assertPixels(t, "footnote_column_3_p_before", `
        KKKK__________
        KKKK__________
        RRRR_RRRR_RRRR
        RRRR_RRRR_RRRR
        RRRR_RRRR_RRRR
        RRRR_RRRR_RRRR
        ______________
        RRRRRRRR______
        RRRRRRRR______
        RRRR_RRRR_____
        RRRR_RRRR_____
        ______________
        ______________
        ______________
        ______________
        ______________
        RRRRRRRR______
        RRRRRRRR______
    `, `
    <style>
      @page {
        size: 14px 9px;
      }
      body {
        font-family: weasyprint;
        font-size: 2px;
        line-height: 1;
      }
      div {
        color: red;
        columns: 3;
        column-gap: 1px;
      }
      span {
        float: footnote;
      }
    </style>
    <p>ab</p>
    <div>ab ab a<span>de</span> ab ab ab a<span>de</span> ab </div>`)
}

// TestDrawFootnoteColumnCloneDecoration is test_footnote_column_clone_decoration in tests/draw/test_footnote_column.py.
func TestDrawFootnoteColumnCloneDecoration(t *testing.T) {
	assertPixels(t, "footnote_column_clone_decoration", `
        _________
        RRRR_RRRR
        RRRR_RRRR
        _________
        _________
        RRRRRRRR_
        RRRRRRRR_
        _________
        RRRR_RRRR
        RRRR_RRRR
        _________
        _________
        _________
        _________
    `, `
    <style>
      @page {
        size: 9px 7px;
      }
      div {
        box-decoration-break: clone;
        color: red;
        columns: 2;
        column-gap: 1px;
        font-family: weasyprint;
        font-size: 2px;
        line-height: 1;
        padding: 1px 0;
      }
      span {
        float: footnote;
      }
    </style>
    <div>a<span>de</span> ab ab ab</div>`)
}

// TestDrawFootnoteColumnMaxHeight is test_footnote_column_max_height in tests/draw/test_footnote_column.py.
func TestDrawFootnoteColumnMaxHeight(t *testing.T) {
	assertPixels(t, "footnote_column_max_height", `
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        _________
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRR_RRRR
        RRRR_RRRR
        _________
        _________
        _________
        _________
        _________
        RRRRRRRR_
        RRRRRRRR_
    `, `
    <style>
      @page {
        size: 9px 9px;
        @footnote {
          max-height: 2em;
        }
      }
      div {
        color: red;
        columns: 2;
        column-gap: 1px;
        font-family: weasyprint;
        font-size: 2px;
        line-height: 1;
      }
      span {
        float: footnote;
      }
    </style>
    <div>
      a<span>de</span> a<span>de</span>
      a<span>de</span> ab
      ab ab
    </div>`)
}

// TestDrawFootnoteColumnReportedSplit is test_footnote_column_reported_split in tests/draw/test_footnote_column.py.
func TestDrawFootnoteColumnReportedSplit(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "footnote_column_reported_split", `
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        RRRR_RRRR
        _________
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRRRRRR_
        RRRR_____
        RRRR_____
        _________
        _________
        _________
        _________
        _________
        RRRRRRRR_
        RRRRRRRR_
    `, `
    <style>
      @page {
        size: 9px 9px;
      }
      div {
        color: red;
        columns: 2;
        column-gap: 1px;
        font-family: weasyprint;
        font-size: 2px;
        line-height: 1;
      }
      span {
        float: footnote;
      }
    </style>
    <div>
      <article>a<span>de</span> a<span>de</span></article>
      <article>a<span>de</span> ab ab</article>
    </div>`)
}
