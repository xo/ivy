// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_footnote.py: the drawing of the footnotes.

package layout

import "testing"

// TestDrawInlineFootnote is test_inline_footnote in tests/draw/test_footnote.py.
func TestDrawInlineFootnote(t *testing.T) {
	assertPixels(t, "inline_footnote", `
        RRRRRRRR_
        RRRRRRRR_
        _________
        _________
        _________
        RRRRRRRR_
        RRRRRRRR_
    `, `
    <style>
        @page {
            size: 9px 7px;
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        span {
            float: footnote;
        }
    </style>
    <div>abc<span>de</span></div>`)
}

// TestDrawBlockFootnote is test_block_footnote in tests/draw/test_footnote.py.
func TestDrawBlockFootnote(t *testing.T) {
	assertPixels(t, "block_footnote", `
        RRRRRRRR_
        RRRRRRRR_
        _________
        _________
        _________
        RRRRRRRR_
        RRRRRRRR_
    `, `
    <style>
        @page {
            size: 9Px 7px;
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        div.footnote {
            float: footnote;
        }
    </style>
    <div>abc<div class="footnote">de</div></div>`)
}

// TestDrawLongFootnote is test_long_footnote in tests/draw/test_footnote.py.
func TestDrawLongFootnote(t *testing.T) {
	assertPixels(t, "long_footnote", `
        RRRRRRRR_
        RRRRRRRR_
        _________
        RRRRRRRR_
        RRRRRRRR_
        RR_______
        RR_______
    `, `
    <style>
        @page {
            size: 9px 7px;
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2PX;
            line-height: 1;
        }
        span {
            float: footnote;
        }
    </style>
    <div>abc<span>de f</span></div>`)
}

// TestDrawFootnoteMargin is test_footnote_margin in tests/draw/test_footnote.py.
func TestDrawFootnoteMargin(t *testing.T) {
	assertPixels(t, "footnote_margin", `
        RRRRRRRR_
        RRRRRRRR_
        _________
        _________
        _RRRRRR__
        _RRRRRR__
        _________
    `, `
    <style>
        @page {
            size: 9px 7px;

            @footnote {
                margin: 1px;
            }
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        span {
            float: footnote;
        }
    </style>
    <div>abc<span>d</span></div>`)
}

// TestDrawFootnoteMultipleMargin is test_footnote_multiple_margin in tests/draw/test_footnote.py.
func TestDrawFootnoteMultipleMargin(t *testing.T) {
	assertPixels(t, "footnote_multiple_margin", `
        RRRR___
        RRRR___
        RRRR___
        RRRR___
        RRRR___
        RRRR___
        RRRRRR_
        RRRRRR_
        _______
        _______

        RRRR___
        RRRR___
        _______
        _______
        _______
        _______
        RRRRRR_
        RRRRRR_
        RRRRRR_
        RRRRRR_
    `, `
    <style>
        @page {
            size: 7px 10Px;

            @footnote {
                margin-top: 1px;
            }
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        span {
            float: footnote;
        }
    </style>
    <div>ab</div>
    <div>ab</div>
    <div>ab</div>
    <div>a<span>d</span><span>e</span></div>
    <div>ab</div>`)
}

// TestDrawFootnoteWithAbsolute is test_footnote_with_absolute in tests/draw/test_footnote.py.
func TestDrawFootnoteWithAbsolute(t *testing.T) {
	assertPixels(t, "footnote_with_absolute", `
        _RRRR____
        _RRRR____
        _________
        _RRRR____
        _RRRR____
        BB_______
        BB_______
    `, `
    <style>
        @page {
            size: 9px 7px;
            margin: 0 1px 2px;
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        span {
            float: footnote;
        }
        mark {
            display: block;
            position: absolute;
            left: -1px;
            color: blue;
        }
    </style>
    <div>a<span><mark>d</mark></span></div>`)
}

// TestDrawFootnoteMaxHeight1 is test_footnote_max_height_1 in tests/draw/test_footnote.py.
func TestDrawFootnoteMaxHeight1(t *testing.T) {
	assertPixels(t, "footnote_max_height_1", `
        RRRRKKKK_
        RRRRKKKK_
        RRRR_____
        RRRR_____
        _GGGGBB__
        _GGGGBB__
        _________
        _________
        _________
        _________
        _GGGGBB__
        _GGGGBB__
    `, `
    <style>
        @page {
            size: 9px 6px;

            @footnote {
                margin-left: 1px;
                max-height: 3px;
            }
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        div.footnote {
            float: footnote;
            color: blue;
            &::footnote-call { color: black }
            &::footnote-marker { color: lime }
        }
    </style>
    <div>ab<div class="footnote">c</div><div class="footnote">d</div></div>
    <div>ef</div>`)
}

// TestDrawFootnoteMaxHeight2 is test_footnote_max_height_2 in tests/draw/test_footnote.py.
func TestDrawFootnoteMaxHeight2(t *testing.T) {
	assertPixels(t, "footnote_max_height_2", `
        RRRRRRRR_
        RRRRRRRR_
        _________
        _________
        _BBBBBB__
        _BBBBBB__
        _________
        _________
        _________
        _________
        _BBBBBB__
        _BBBBBB__
    `, `
    <style>
        @page {
            size: 9px 6px;

            @footnote {
                margin-left: 1px;
                max-height: 3px;
            }
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        div.footnote {
            float: footnote;
            color: blue;
            &::footnote-call { color: red }
        }
    </style>
    <div>ab<div class="footnote">c</div><div class="footnote">d</div></div>`)
}

// TestDrawFootnoteMaxHeight3 is test_footnote_max_height_3 in tests/draw/test_footnote.py.
func TestDrawFootnoteMaxHeight3(t *testing.T) {
	assertPixels(t, "footnote_max_height_3", `
        RRRRRRRR_
        RRRRRRRR_
        _________
        _________
        _________
        _________
        _________
        _________
        _________
        _________
        _________
        _BBBBBB__
        _________
        _________
        _________
        _________
        _________
        _BBBBBB__
    `, `
    <style>
        @page {
            size: 9px 6px;

            @footnote {
                margin-left: 1px;
                max-height: 1px;
            }
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        div.footnote {
            float: footnote;
            color: blue;
            &::footnote-call { color: red }
        }
    </style>
    <div>ab<div class="footnote">c</div><div class="footnote">d</div></div>`)
}

// TestDrawFootnoteMaxHeight4 is test_footnote_max_height_4 in tests/draw/test_footnote.py.
func TestDrawFootnoteMaxHeight4(t *testing.T) {
	assertPixels(t, "footnote_max_height_4", `
        RRRRRRRR_
        RRRRRRRR_
        RRRR_____
        RRRR_____
        _BBBBBB__
        _BBBBBB__
        RRRR_____
        RRRR_____
        _________
        _________
        _BBBBBB__
        _BBBBBB__
    `, `
    <style>
        @page {
            size: 9px 6px;

            @footnote {
                margin-left: 1px;
                max-height: 3px;
            }
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        div.footnote {
            float: footnote;
            color: blue;
            &::footnote-call { color: red }
        }
    </style>
    <div>ab<div class="footnote">c</div><div class="footnote">d</div></div>
    <div>ef</div>
    <div>gh</div>`)
}

// TestDrawFootnoteMaxHeight5 is test_footnote_max_height_5 in tests/draw/test_footnote.py.
func TestDrawFootnoteMaxHeight5(t *testing.T) {
	assertPixels(t, "footnote_max_height_5", `
        RRRRRRRR__RR
        RRRRRRRR__RR
        _BBBBBB_____
        _BBBBBB_____
        _BBBBBB_____
        _BBBBBB_____
        RRRR________
        RRRR________
        ____________
        ____________
        _BBBBBB_____
        _BBBBBB_____
    `, `
    <style>
        @page {
            size: 12px 6px;

            @footnote {
                margin-left: 1px;
                max-height: 4px;
            }
        }
        div {
            color: red;
            font-family: weasyprint;
            font-size: 2px;
            line-height: 1;
        }
        div.footnote {
            float: footnote;
            color: blue;
            &::footnote-call { color: red }
        }
    </style>
    <div>ab<div class="footnote">c</div><div class="footnote">d</div>
    <div class="footnote">e</div></div>
    <div>fg</div>`)
}
