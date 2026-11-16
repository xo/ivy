// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_overflow.py.

package layout

import "testing"

// TestDrawOverflow1 is test_overflow_1 in tests/draw/test_overflow.py.
func TestDrawOverflow1(t *testing.T) {
	// See test_images
	assertPixels(t, "overflow_1", `
        ________
        ________
        __rBBB__
        __BBBB__
        ________
        ________
        ________
        ________
    `, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        div { height: 2px; overflow: hidden }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDrawOverflow2 is test_overflow_2 in tests/draw/test_overflow.py.
func TestDrawOverflow2(t *testing.T) {
	// <body> is only 1px high, but its overflow is propageted to the viewport
	// ie. the padding edge of the page box.
	assertPixels(t, "overflow_2", `
        ________
        ________
        __rBBB__
        __BBBB__
        __BBBB__
        ________
        ________
        ________
    `, `
      <style>
        @page { size: 8px; margin: 2px 2px 3px 2px }
        body { height: 1px; overflow: hidden; font-size: 0 }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDrawOverflow3 is test_overflow_3 in tests/draw/test_overflow.py.
func TestDrawOverflow3(t *testing.T) {
	// Assert that the border is not clipped by overflow: hidden
	assertPixels(t, "overflow_3", `
        ________
        ________
        __BBBB__
        __B__B__
        __B__B__
        __BBBB__
        ________
        ________
    `, `
      <style>
        @page { size: 8px; margin: 2px; }
        div { width: 2px; height: 2px; overflow: hidden;
              border: 1px solid blue; }
      </style>
      <div></div>`)
}

// TestDrawOverflow4 is test_overflow_4 in tests/draw/test_overflow.py.
func TestDrawOverflow4(t *testing.T) {
	// Assert that the page margins aren't clipped by body's overflow
	assertPixels(t, "overflow_4", `
        rr______
        rr______
        __BBBB__
        __BBBB__
        __BBBB__
        __BBBB__
        ________
        ________
    `, `
      <style>
        @page {
          size: 8px;
          margin: 2px;
          background:#fff;
          @top-left-corner { content: ''; background:#f00; } }
        body { overflow: auto; background:#00f; }
      </style>
      `)
}

// TestDrawOverflow5 is test_overflow_5 in tests/draw/test_overflow.py.
func TestDrawOverflow5(t *testing.T) {
	// Regression test for #2026.
	assertPixels(t, "overflow_5", `
        BBBBBB__
        BBBBBB__
        BBBB____
        BBBB____
        BBBB____
        ________
        ________
        ________
    `, `
      <style>
        @page { size: 8px }
        body { font-family: weasyprint; line-height: 1; font-size: 2px }
        p { color: blue }
      </style>
      <p>abc</p>
      <p style="height: 3px; overflow: hidden">ab<br>ab<br>ab<br>ab</p>
      `)
}

// TestDrawClip is test_clip in tests/draw/test_overflow.py.
func TestDrawClip(t *testing.T) {
	for _, tc := range []struct{ name, pixels, html string }{
		{"1_5px_5px_9px_auto_n_n_n_n_n_n_n_rBBBrBg_n_BBBBBBg_n_BBBBBBg_n_BBBBBBg_n_n_n_n_n_n_n", `
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______rBBBrBg_
        ______BBBBBBg_
        ______BBBBBBg_
        ______BBBBBBg_
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        div { margin: 1px; border: 1px green solid;
              background: url(pattern.png);
              position: absolute; /* clip only applies on abspos */
              top: 0; bottom: 2px; left: 0; right: 0;
              clip: rect(5px, 5px, 9px, auto); }
      </style>
      <div>`},
		{"2_5px_5px_auto_10px_n_n_n_n_n_n_n_rBBBr_n_BBBBB_n_BBBBB_n_BBBBB_n_rBBBr_n_BBBBB_n_ggggg_n_n_n_n", `
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______rBBBr___
        ______BBBBB___
        ______BBBBB___
        ______BBBBB___
        ______rBBBr___
        ______BBBBB___
        ______ggggg___
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        div { margin: 1px; border: 1px green solid;
              background: url(pattern.png);
              position: absolute; /* clip only applies on abspos */
              top: 0; bottom: 2px; left: 0; right: 0;
              clip: rect(5px, 5px, auto, 10px); }
      </style>
      <div>`},
		{"3_5px_auto_9px_10px_n_n_n_n_n_n_n_grBBBrBBBr_n_gBBBBBBBBB_n_gBBBBBBBBB_n_gBBBBBBBBB_n_n_n_n_n_n_n", `
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        _grBBBrBBBr___
        _gBBBBBBBBB___
        _gBBBBBBBBB___
        _gBBBBBBBBB___
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        div { margin: 1px; border: 1px green solid;
              background: url(pattern.png);
              position: absolute; /* clip only applies on abspos */
              top: 0; bottom: 2px; left: 0; right: 0;
              clip: rect(5px, auto, 9px, 10px); }
      </style>
      <div>`},
		{"4_auto_5px_9px_10px_n_n_ggggg_n_rBBBr_n_BBBBB_n_BBBBB_n_BBBBB_n_rBBBr_n_BBBBB_n_BBBBB_n_BBBBB_n_n_n_n_n_n_n", `
        ______________
        ______ggggg___
        ______rBBBr___
        ______BBBBB___
        ______BBBBB___
        ______BBBBB___
        ______rBBBr___
        ______BBBBB___
        ______BBBBB___
        ______BBBBB___
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        div { margin: 1px; border: 1px green solid;
              background: url(pattern.png);
              position: absolute; /* clip only applies on abspos */
              top: 0; bottom: 2px; left: 0; right: 0;
              clip: rect(auto, 5px, 9px, 10px); }
      </style>
      <div>`},
	} {
		t.Run(tc.name, func(t *testing.T) { assertPixels(t, "clip_"+tc.name, tc.pixels, tc.html) })
	}
}
