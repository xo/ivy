// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_note.py: the drawing of the notes.

package layout

import "testing"

// TestDrawOneNote is test_one_note in tests/draw/test_note.py.
func TestDrawOneNote(t *testing.T) {
	assertPixels(t, "one_note", `
        BBBBBBBBBB
        BBBBBBBBBB
        RRRRRRRR__
        RRRRRRRR__
    `, `
        <style>
            @page {
                size: 10px 4px;
                @note-area {
                    content: element(sidenotes, all-once);
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);

                &::note-call { color: red}
            }
        </style>
        <div>abc<span>de</span></div>`)
}

// TestDrawSeveralNote is test_several_note in tests/draw/test_note.py.
func TestDrawSeveralNote(t *testing.T) {
	assertPixels(t, "several_note", `
        BBBBBBBBBB______
        BBBBBBBBBB______
        BBBBBBBBBB______
        BBBBBBBBBB______
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
    `, `
        <style>
            @page {
                size: 16px 6px;
                @note-area {
                    content: element(sidenotes, all-once);
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);

                &::note-call { color: red}
            }
        </style>
        <div>abc<span>de</span>fgh<span>ij</span></div>`)
}

// TestDrawInlineNote is test_inline_note in tests/draw/test_note.py.
func TestDrawInlineNote(t *testing.T) {
	assertPixels(t, "inline_note", `
        BBBBBBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBBBBBB
        RRRRRRRRRRRRRRRR____
        RRRRRRRRRRRRRRRR____
    `, `
        <style>
            @page {
                size: 20px 4px;
                @note-area {
                    content: element(sidenotes, all-once);
                    font: 2px/1 weasyprint;
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: inline;
                position: note(sidenotes);

                &::note-call { color: red }
            }
        </style>
        <div>abc<span>de</span>fgh<span>ij</span></div>`)
}

// TestDrawAbsoluteNote is test_absolute_note in tests/draw/test_note.py.
func TestDrawAbsoluteNote(t *testing.T) {
	assertPixels(t, "absolute_note", `
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        ________________
        ________________
        ______RRRRRRRRRR
        ______RRRRRRRRRR
        ______RRRRRRRRRR
        ______RRRRRRRRRR
    `, `
        <style>
            @page {
                size: 16px 8px;
                @note-area {
                    content: element(sidenotes, all-once);
                    position: absolute;
                    bottom: 0;
                    right: 0;
                    width: calc(10 / 16 * 100%);
                }
            }
            div {
                color: red;
                font-family: weasyprint;
                font-size: 2px;
                line-height: 1;
            }
            span {
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>abc<span>de</span>fgh<span>ij</span></div>`)
}

// TestDrawFloatLeftNote is test_float_left_note in tests/draw/test_note.py.
func TestDrawFloatLeftNote(t *testing.T) {
	assertPixels(t, "float_left_note", `
        BBBBBBBBRRRRRRBB
        BBBBBBBBRRRRRRBB
        BBBBBBBBRRRRRRBB
        BBBBBBBBRRRRRRBB
    `, `
        <style>
            @page {
                size: 16px 4px;
                @note-area {
                    content: element(sidenotes, all-once);
                    float: left;
                    width: 50%;
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>abc<span>d</span>
          fgh<span>i</span></div>`)
}

// TestDrawFloatRightNote is test_float_right_note in tests/draw/test_note.py.
func TestDrawFloatRightNote(t *testing.T) {
	assertPixels(t, "float_right_note", `
        RRRRRRBBBBBBBBBB
        RRRRRRBBBBBBBBBB
        RRRRRRBBBBBBBBBB
        RRRRRRBBBBBBBBBB
    `, `
        <style>
            @page {
                size: 16px 4px;
                @note-area {
                    content: element(sidenotes, all-once);
                    float: right;
                    width: 50%;
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>abc<span>d</span>
          fgh<span>i</span></div>`)
}

// TestDrawFloatLeftMarginNote is test_float_left_margin_note in tests/draw/test_note.py.
func TestDrawFloatLeftMarginNote(t *testing.T) {
	assertPixels(t, "float_left_margin_note", `
        ________________________
        ________________________
        __BBBBBBBB______________
        __BBBBBBBB______________
        __BBBBBBBB__RRRRRRBB____
        __BBBBBBBB__RRRRRRBB____
        ____________RRRRRRBB____
        ____________RRRRRRBB____
        ____RRRRRRRRRR__________
        ____RRRRRRRRRR__________
        ________________________
        ________________________
        ________________________
        ________________________
    `, `
        <style>
            @page {
                size: 24px 14px;
                margin: 4px;
                @note-area {
                    content: element(sidenotes, all-once);
                    float: left;
                    margin: -2px 2px 2px -2px;
                    width: 50%;
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>abc<span>d</span>
          fgh<span>i</span>
          jklmn</div>`)
}

// TestDrawFlexNote is test_flex_note in tests/draw/test_note.py.
func TestDrawFlexNote(t *testing.T) {
	assertPixels(t, "flex_note", `
        BBBBBBBBBBBBBBBB
        BBBBBBBBBBBBBBBB
        RRRRRRBBRRRRRRBB
        RRRRRRBBRRRRRRBB
    `, `
        <style>
            @page {
                size: 16px 4px;
                @note-area {
                    content: element(sidenotes, all-once);
                    display: flex;
                    flex-wrap: wrap;
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>abc<span>d</span>fgh<span>i</span></div>`)
}

// TestDrawGridNote is test_grid_note in tests/draw/test_note.py.
func TestDrawGridNote(t *testing.T) {
	assertPixels(t, "grid_note", `
        BBBBBBBB__BBBBBBBB
        BBBBBBBB__BBBBBBBB
        RRRRRRBBRRRRRRBB__
        RRRRRRBBRRRRRRBB__
    `, `
        <style>
            @page {
                size: 18px 4px;
                @note-area {
                    content: element(sidenotes, all-once);
                    display: grid;
                    grid-template-columns: 1fr 1fr;
                    gap: 2px;
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>abc<span>d</span>fgh<span>i</span></div>`)
}

// TestDrawNextPageNote is test_next_page_note in tests/draw/test_note.py.
func TestDrawNextPageNote(t *testing.T) {
	assertPixels(t, "next_page_note", `
        BBBBBBBB________
        BBBBBBBB________
        BBBBBBBB________
        BBBBBBBB________
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        RRRRRRBBRRRRRRBB
        RRRRRRBBRRRRRRBB
        ________________
        ________________
        ________________
        ________________
    `, `
        <style>
            @page {
                size: 16px 6px;
                @note-area {
                    content: element(sidenotes, all-once);
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>aaaaaaaa
          abc<span>d</span>fgh<span>i</span></div>`)
}

// TestDrawNextPageSplitNote is test_next_page_split_note in tests/draw/test_note.py.
func TestDrawNextPageSplitNote(t *testing.T) {
	assertPixels(t, "next_page_split_note", `
        BBBBBBBB________
        BBBBBBBB________
        RRRRRRBB________
        RRRRRRBB________
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        BBBBBBBB________
        BBBBBBBB________
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        RRRRRRBB________
        RRRRRRBB________
    `, `
        <style>
            @page {
                size: 16px 6px;
                @note-area {
                    content: element(sidenotes, all-once);
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>abc<span>d</span>
          aaaaaaaa
          aaaaaaaa
          fgh<span>i</span></div>`)
}

// TestDrawNextPageSplitNotFittedNote is test_next_page_split_not_fitted_note in tests/draw/test_note.py.
func TestDrawNextPageSplitNotFittedNote(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "next_page_split_not_fitted_note", `
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        RRRRRRBB________
        RRRRRRBB________
        BBBBBBBB________
        BBBBBBBB________
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        RRRRRRBB________
        RRRRRRBB________
        BBBBBBBB________
        BBBBBBBB________
        ________________
        ________________
        ________________
        ________________
    `, `
        <style>
            @page {
                size: 16px 6px;
                @note-area {
                    content: element(sidenotes, all-once);
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>
          aaaaaaaa
          aaaaaaaa
          abc<span>d</span>
          aaaaaaaa
          fgh<span>i</span></div>`)
}

// TestDrawNextPageSplitNotFittedNote2 is test_next_page_split_not_fitted_note_2 in tests/draw/test_note.py.
func TestDrawNextPageSplitNotFittedNote2(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "next_page_split_not_fitted_note_2", `
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        RRRRRRBB________
        RRRRRRBB________
        BBBBBBBB________
        BBBBBBBB________
        BBBBBBBB________
        BBBBBBBB________
        RRRRRRBB________
        RRRRRRBB________
    `, `
        <style>
            @page {
                size: 16px 6px;
                @note-area {
                    content: element(sidenotes, all-once);
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>
          aaaaaaaa
          aaaaaaaa
          abc<span>d</span>
          fgh<span>i</span></div>`)
}

// TestDrawNextPageSplitNotFittedNote3 is test_next_page_split_not_fitted_note_3 in tests/draw/test_note.py.
func TestDrawNextPageSplitNotFittedNote3(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "next_page_split_not_fitted_note_3", `
        BBBBBBBB________
        BBBBBBBB________
        RRRRRRBB________
        RRRRRRBB________
        RRRRRRRRRRRRRRRR
        RRRRRRRRRRRRRRRR
        BBBBBBBB________
        BBBBBBBB________
        RRRRRRBB________
        RRRRRRBB________
        ________________
        ________________
    `, `
        <style>
            @page {
                size: 16px 6px;
                @note-area {
                    content: element(sidenotes, all-once);
                }
            }
            div {
                color: red;
                font: 2px/1 weasyprint;
            }
            span {
                color: blue;
                display: block;
                position: note(sidenotes);
            }
        </style>
        <div>abc<span>d</span>
          aaaaaaaa
          fgh<span>i</span></div>`)
}
