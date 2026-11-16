// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_whitespace.py: the drawing of the white space.

package layout

import "testing"

// TestDrawWhitespaceInline is test_whitespace_inline in tests/draw/test_whitespace.py.
func TestDrawWhitespaceInline(t *testing.T) {
	assertPixels(t, "whitespace_inline", `
        RRRR__RRRR____
        RRRR__RRRR____
        ______________
        ______________
    `, `
        <style>
            @page {size: 14px 4px}
            body {
              color: red;
              font-family: weasyprint;
              font-size: 2px;
              line-height: 1;
            }
        </style>
        <span>aa </span><span> aa</span>
    `)
}

// TestDrawWhitespaceNestedInline is test_whitespace_nested_inline in tests/draw/test_whitespace.py.
func TestDrawWhitespaceNestedInline(t *testing.T) {
	assertPixels(t, "whitespace_nested_inline", `
        RRRR__RRRR____
        RRRR__RRRR____
        ______________
        ______________
    `, `
        <style>
            @page {size: 14px 4px}
            body {
              color: red;
              font-family: weasyprint;
              font-size: 2px;
              line-height: 1;
            }
        </style>
        <span><span>aa </span></span><span><span> aa</span></span>
    `)
}

// TestDrawWhitespaceInlineSpaceBetween is test_whitespace_inline_space_between in tests/draw/test_whitespace.py.
func TestDrawWhitespaceInlineSpaceBetween(t *testing.T) {
	assertPixels(t, "whitespace_inline_space_between", `
        RRRR__RRRR____
        RRRR__RRRR____
        ______________
        ______________
    `, `
        <style>
            @page {size: 14px 4px}
            body {
              color: red;
              font-family: weasyprint;
              font-size: 2px;
              line-height: 1;
            }
        </style>
        <span>aa </span> <span> aa</span>
    `)
}

// TestDrawWhitespaceFloatBetween is test_whitespace_float_between in tests/draw/test_whitespace.py.
func TestDrawWhitespaceFloatBetween(t *testing.T) {
	assertPixels(t, "whitespace_float_between", `
        RRRR__RRRR__BB
        RRRR__RRRR__BB
        ______________
        ______________
    `, `
        <style>
            @page {size: 14px 4px}
            body {
              color: red;
              font-family: weasyprint;
              font-size: 2px;
              line-height: 1;
            }
            div {float: right; color: blue}
        </style>
        <span>aa </span><div>a</div><span> aa</span>
    `)
}

// TestDrawWhitespaceInFloat is test_whitespace_in_float in tests/draw/test_whitespace.py.
func TestDrawWhitespaceInFloat(t *testing.T) {
	assertPixels(t, "whitespace_in_float", `
        RRRRRRRR____BB
        RRRRRRRR____BB
        ______________
        ______________
    `, `
        <style>
            @page {size: 14px 4px}
            body {
              color: red;
              font-family: weasyprint;
              font-size: 2px;
              line-height: 1;
            }
            div {
              color: blue;
              float: right;
            }
        </style>
        <span>aa</span><div> a </div><span>aa</span>
    `)
}

// TestDrawWhitespaceAbsoluteBetween is test_whitespace_absolute_between in tests/draw/test_whitespace.py.
func TestDrawWhitespaceAbsoluteBetween(t *testing.T) {
	assertPixels(t, "whitespace_absolute_between", `
        RRRR__RRRR__BB
        RRRR__RRRR__BB
        ______________
        ______________
    `, `
        <style>
            @page {size: 14px 4px}
            body {
              color: red;
              font-family: weasyprint;
              font-size: 2px;
              line-height: 1;
            }
            div {
              color: blue;
              position: absolute;
              right: 0;
              top: 0;
            }
        </style>
        <span>aa </span><div>a</div><span> aa</span>
    `)
}

// TestDrawWhitespaceInAbsolute is test_whitespace_in_absolute in tests/draw/test_whitespace.py.
func TestDrawWhitespaceInAbsolute(t *testing.T) {
	assertPixels(t, "whitespace_in_absolute", `
        RRRRRRRR____BB
        RRRRRRRR____BB
        ______________
        ______________
    `, `
        <style>
            @page {size: 14px 4px}
            body {
              color: red;
              font-family: weasyprint;
              font-size: 2px;
              line-height: 1;
            }
            div {
              color: blue;
              position: absolute;
              right: 0;
              top: 0;
            }
        </style>
        <span>aa</span><div> a </div><span>aa</span>
    `)
}

// TestDrawWhitespaceRunningBetween is test_whitespace_running_between in tests/draw/test_whitespace.py.
func TestDrawWhitespaceRunningBetween(t *testing.T) {
	assertPixels(t, "whitespace_running_between", `
        RRRR__RRRR____
        RRRR__RRRR____
        ______BB______
        ______BB______
    `, `
        <style>
            @page {
              size: 14px 4px;
              margin: 0 0 2px;
              @bottom-center {
                content: element(test);
              }
            }
            body {
              color: red;
              font-family: weasyprint;
              font-size: 2px;
              line-height: 1;
            }
            div {
              background: green;
              color: blue;
              position: running(test);
            }
        </style>
        <span>aa </span><div>a</div><span> aa</span>
    `)
}

// TestDrawWhitespaceInRunning is test_whitespace_in_running in tests/draw/test_whitespace.py.
func TestDrawWhitespaceInRunning(t *testing.T) {
	assertPixels(t, "whitespace_in_running", `
        RRRRRRRR______
        RRRRRRRR______
        ______BB______
        ______BB______
    `, `
        <style>
            @page {
              size: 14px 4px;
              margin: 0 0 2px;
              @bottom-center {
                content: element(test);
              }
            }
            body {
              color: red;
              font-family: weasyprint;
              font-size: 2px;
              line-height: 1;
            }
            div {
              background: green;
              color: blue;
              position: running(test);
            }
        </style>
        <span>aa</span><div> a </div><span>aa</span>
    `)
}
