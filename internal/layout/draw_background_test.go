// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_background.py: how the backgrounds are drawn.

package layout

import "testing"

// TestDrawCanvasBackground is test_canvas_background in tests/draw/test_background.py.
func TestDrawCanvasBackground(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"0", `BBBBBBBBBB
BBBBBBBBBB
BBBBBBBBBB
BBBBBBBBBB
BBBBBBBBBB
BBBBBBBBBB
BBBBBBBBBB
BBBBBBBBBB
BBBBBBBBBB
BBBBBBBBBB
`, `
           <style>
             @page { size: 10px }
             /* body’s background propagates to the whole canvas */
             body { margin: 2px; background: #00f; height: 5Px }
           </style>
         <body>`, false},
		{"1", `
             rrrrrrrrrr
             rrrrrrrrrr
             rrBBBBBBrr
             rrBBBBBBrr
             rrBBBBBBrr
             rrBBBBBBrr
             rrBBBBBBrr
             rrrrrrrrrr
             rrrrrrrrrr
             rrrrrrrrrr
         `, `
           <style>
             @page { size: 10px }
             /* html’s background propagates to the whole canvas */
             html { padding: 1px; background: #f00 }
             /* html has a background, so body’s does not propagate */
             body { margin: 1px; background: #00f; height: 5px }
          </style>
          <body>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "canvas_background", tc.pixels, tc.html)
		})
	}
}

// TestDrawCanvasBackgroundSize is test_canvas_background_size in tests/draw/test_background.py.
func TestDrawCanvasBackgroundSize(t *testing.T) {
	assertPixels(t, "canvas_background_size", `
        __________
        __________
        __RRRRRR__
        __RGGGGR__
        __zzzzzz__
        __zzzzzz__
        __BBBBBB__
        __BBBBBB__
        __________
        __________
    `, `
      <style>
         @page { size: 10px; margin: 2px }
         /* html’s background propagates to the whole canvas */
         html { background: linear-gradient(
           red 0, red 50%, blue 50%, blue 100%); }
         /* html has a background, so body’s does not propagate */
         body { margin: 1px; background: lime; height: 1px }
      </style>
      <body>
    `)
}

// TestDrawBackgroundImage is test_background_image in tests/draw/test_background.py.
func TestDrawBackgroundImage(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"url(pattern.png)", `
        ______________
        ______________
        __rBBBrBBBrB__
        __BBBBBBBBBB__
        __BBBBBBBBBB__
        __BBBBBBBBBB__
        __rBBBrBBBrB__
        __BBBBBBBBBB__
        __BBBBBBBBBB__
        __BBBBBBBBBB__
        __rBBBrBBBrB__
        __BBBBBBBBBB__
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) repeat-x", `
        ______________
        ______________
        __rBBBrBBBrB__
        __BBBBBBBBBB__
        __BBBBBBBBBB__
        __BBBBBBBBBB__
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) repeat-x }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) repeat-y", `
        ______________
        ______________
        __rBBB________
        __BBBB________
        __BBBB________
        __BBBB________
        __rBBB________
        __BBBB________
        __BBBB________
        __BBBB________
        __rBBB________
        __BBBB________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) repeat-y }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat 0 0%", `
        ______________
        ______________
        __rBBB________
        __BBBB________
        __BBBB________
        __BBBB________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat 0 0% }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat 50% 0px", `
        ______________
        ______________
        _____rBBB_____
        _____BBBB_____
        _____BBBB_____
        _____BBBB_____
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat 50% 0px }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat 6px top", `
        ______________
        ______________
        ________rBBB__
        ________BBBB__
        ________BBBB__
        ________BBBB__
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat 6px top }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat bottom 6PX ri", `
        ______________
        ______________
        ________rBBB__
        ________BBBB__
        ________BBBB__
        ________BBBB__
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat bottom 6PX right 0 }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat left center", `
        ______________
        ______________
        ______________
        ______________
        ______________
        __rBBB________
        __BBBB________
        __BBBB________
        __BBBB________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat left center }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat center left", `
        ______________
        ______________
        ______________
        ______________
        ______________
        __rBBB________
        __BBBB________
        __BBBB________
        __BBBB________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat center left }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat 3px 3px", `
        ______________
        ______________
        ______________
        ______________
        ______________
        _____rBBB_____
        _____BBBB_____
        _____BBBB_____
        _____BBBB_____
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat 3px 3px }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat 100% 50%", `
        ______________
        ______________
        ______________
        ______________
        ______________
        ________rBBB__
        ________BBBB__
        ________BBBB__
        ________BBBB__
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat 100% 50% }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat 0% bottom", `
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        __rBBB________
        __BBBB________
        __BBBB________
        __BBBB________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat 0% bottom }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat center 6px", `
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        _____rBBB_____
        _____BBBB_____
        _____BBBB_____
        _____BBBB_____
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat center 6px }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat bottom center", `
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        _____rBBB_____
        _____BBBB_____
        _____BBBB_____
        _____BBBB_____
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat bottom center }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat 6px 100%", `
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ________rBBB__
        ________BBBB__
        ________BBBB__
        ________BBBB__
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat 6px 100% }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) repeat-x 1px 2px", `
        ______________
        ______________
        ______________
        ______________
        __BrBBBrBBBr__
        __BBBBBBBBBB__
        __BBBBBBBBBB__
        __BBBBBBBBBB__
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) repeat-x 1px 2px }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) repeat-y local 2pX 1px", `
        ______________
        ______________
        ____BBBB______
        ____rBBB______
        ____BBBB______
        ____BBBB______
        ____BBBB______
        ____rBBB______
        ____BBBB______
        ____BBBB______
        ____BBBB______
        ____rBBB______
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) repeat-y local 2pX 1px }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat fixed", `
        # The image is actually here:
        #######
        ______________
        ______________
        __BB__________
        __BB__________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat fixed }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat fixed right 3", `
        #                   x x x x
        ______________
        ______________
        ______________
        __________rB__   #
        __________BB__   #
        __________BB__   #
        __________BB__   #
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat fixed right 3px }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png)no-repeat fixed 50%cente", `
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        _____rBBB_____
        _____BBBB_____
        _____BBBB_____
        _____BBBB_____
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png)no-repeat fixed 50%center }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat, url(pattern.", `
        ______________
        ______________
        __rBBB________
        __BBBBBB______
        __BBBBBB______
        __BBBBBB______
        ____BBBB______
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat, url(pattern.png) no-repeat 2px 1px }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
		{"url(pattern.png) no-repeat 2px 1px, url(", `
        ______________
        ______________
        __rBBB________
        __BBrBBB______
        __BBBBBB______
        __BBBBBB______
        ____BBBB______
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
        ______________
    `, `
      <style>
        @page { size: 14px 16px }
        html { background: #fff }
        body { margin: 2px; height: 10px; background: url(pattern.png) no-repeat 2px 1px, url(pattern.png) no-repeat }
        p { background: none }
      </style>
      <body>
      <p>&nbsp;
    `, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "background_image", tc.pixels, tc.html)
		})
	}
}

// TestDrawBackgroundImageZeroSizeBackground is test_background_image_zero_size_background in tests/draw/test_background.py.
func TestDrawBackgroundImageZeroSizeBackground(t *testing.T) {
	assertPixels(t, "background_image_zero_size_background", `
        __________
        __________
        __________
        __________
        __________
        __________
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page { size: 10px }
        html { background: #fff }
        body { background: url(pattern.png);
               background-size: cover;
               display: inline-block }
      </style>
      <body>
    `)
}

// TestDrawBackgroundOrigin is test_background_origin in tests/draw/test_background.py.
func TestDrawBackgroundOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"border-box", `
        ____________
        ____________
        ____________
        ____________
        ____________
        ____________
        ____________
        _______rBBB_
        _______BBBB_
        _______BBBB_
        _______BBBB_
        ____________
    `, `
      <style>
        @page { size: 12px }
        html { background: #fff }
        body { margin: 1px; padding: 1px; height: 6px;
               border: 1px solid transparent;
               background: url(pattern.png) bottom right no-repeat;
               background-origin: border-box }
      </style>
      <body>`, false},
		{"padding-box", `
        ____________
        ____________
        ____________
        ____________
        ____________
        ____________
        ______rBBB__
        ______BBBB__
        ______BBBB__
        ______BBBB__
        ____________
        ____________
    `, `
      <style>
        @page { size: 12px }
        html { background: #fff }
        body { margin: 1px; padding: 1px; height: 6px;
               border: 1px solid transparent;
               background: url(pattern.png) bottom right no-repeat;
               background-origin: padding-box }
      </style>
      <body>`, false},
		{"content-box", `
        ____________
        ____________
        ____________
        ____________
        ____________
        _____rBBB___
        _____BBBB___
        _____BBBB___
        _____BBBB___
        ____________
        ____________
        ____________
    `, `
      <style>
        @page { size: 12px }
        html { background: #fff }
        body { margin: 1px; padding: 1px; height: 6px;
               border: 1px solid transparent;
               background: url(pattern.png) bottom right no-repeat;
               background-origin: content-box }
      </style>
      <body>`, false},
		{"border-box; background-clip: content-box", `
        ____________
        ____________
        ____________
        ____________
        ____________
        ____________
        ____________
        _______rB___
        _______BB___
        ____________
        ____________
        ____________
    `, `
      <style>
        @page { size: 12px }
        html { background: #fff }
        body { margin: 1px; padding: 1px; height: 6px;
               border: 1px solid transparent;
               background: url(pattern.png) bottom right no-repeat;
               background-origin: border-box; background-clip: content-box }
      </style>
      <body>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "background_origin", tc.pixels, tc.html)
		})
	}
}

// TestDrawBackgroundTransform is test_background_transform in tests/draw/test_background.py.
func TestDrawBackgroundTransform(t *testing.T) {
	assertPixels(t, "background_transform", `
        _______
        _RRRRR_
        _RRRRR_
        _RRRRR_
        _RRRRR_
        _RRRRR_
        _______
    `, `
      <style>
        @page { size: 7px 7px }
        html { background: #fff }
        body { position: absolute;
               width: 5px; height: 5px;
               top: -5px; left: -5px;
               transform: translate(6px,6px);
               background: red }
      </style>
      <body>`)
}

// TestDrawBackgroundRepeatSpace1 is test_background_repeat_space_1 in tests/draw/test_background.py.
func TestDrawBackgroundRepeatSpace1(t *testing.T) {
	assertPixels(t, "background_repeat_space_1", `
        ____________
        _rBBB__rBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        ____________
        _rBBB__rBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        ____________
        _rBBB__rBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        ____________
    `, `
      <style>
        @page { size: 12px 16px }
        html { background: #fff }
        body { margin: 1px; height: 14px;
               background: url(pattern.png) space }
      </style>
      <body>`)
}

// TestDrawBackgroundRepeatSpace2 is test_background_repeat_space_2 in tests/draw/test_background.py.
func TestDrawBackgroundRepeatSpace2(t *testing.T) {
	assertPixels(t, "background_repeat_space_2", `
        ____________
        _rBBB__rBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        _rBBB__rBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        _rBBB__rBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        _BBBB__BBBB_
        ____________
    `, `
      <style>
        @page { size: 12px 14px }
        html { background: #fff }
        body { margin: 1px; height: 12px;
               background: url(pattern.png) space }
      </style>
      <body>`)
}

// TestDrawBackgroundRepeatSpace3 is test_background_repeat_space_3 in tests/draw/test_background.py.
func TestDrawBackgroundRepeatSpace3(t *testing.T) {
	assertPixels(t, "background_repeat_space_3", `
        ____________
        _rBBBrBBBrB_
        _BBBBBBBBBB_
        _BBBBBBBBBB_
        _BBBBBBBBBB_
        ____________
        ____________
        ____________
        _rBBBrBBBrB_
        _BBBBBBBBBB_
        _BBBBBBBBBB_
        _BBBBBBBBBB_
        ____________
    `, `
      <style>
        @page { size: 12px 13px }
        html { background: #fff }
        body { margin: 1px; height: 11px;
               background: url(pattern.png) repeat space }
      </style>
      <body>`)
}

// TestDrawBackgroundRepeatSpace4 is test_background_repeat_space_4 in tests/draw/test_background.py.
func TestDrawBackgroundRepeatSpace4(t *testing.T) {
	assertPixels(t, "background_repeat_space_4", `
        ________
        _rBBBGG_
        _BBBBGG_
        _BBBBGG_
        _BBBBGG_
        _GGGGGG_
        _GGGGGG_
        ________
    `, `
      <style>
        @page { size: 8px }
        html { background: #fff }
        body { margin: 1px; height: 6Px;
               background: url(pattern.png) space lime }
      </style>
      <body>`)
}

// TestDrawBackgroundRepeatRound1 is test_background_repeat_round_1 in tests/draw/test_background.py.
func TestDrawBackgroundRepeatRound1(t *testing.T) {
	assertPixels(t, "background_repeat_round_1", `
        __________
        _rrBBBBBB_
        _rrBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _rrBBBBBB_
        _rrBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        __________
    `, `
      <style>
        @page { size: 10px 14px }
        html { background: #fff }
        body { margin: 1px; height: 12px;
               image-rendering: pixelated;
               background: url(pattern.png) top/6px round repeat; }
      </style>
      <body>`)
}

// TestDrawBackgroundRepeatRound2 is test_background_repeat_round_2 in tests/draw/test_background.py.
func TestDrawBackgroundRepeatRound2(t *testing.T) {
	assertPixels(t, "background_repeat_round_2", `
        __________
        _rrBBBBBB_
        _rrBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _rrBBBBBB_
        _rrBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        __________
    `, `
      <style>
        @page { size: 10px 18px }
        html { background: #fff }
        body { margin: 1px; height: 16px;
               image-rendering: pixelated;
               background: url(pattern.png) center/auto 8px repeat round; }
      </style>
      <body>`)
}

// TestDrawBackgroundRepeatRound3 is test_background_repeat_round_3 in tests/draw/test_background.py.
func TestDrawBackgroundRepeatRound3(t *testing.T) {
	assertPixels(t, "background_repeat_round_3", `
        __________
        _rrBBBBBB_
        _rrBBBBBB_
        _rrBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        __________
    `, `
      <style>
        @page { size: 10px 14px }
        html { background: #fff }
        body { margin: 1px; height: 12px;
               image-rendering: pixelated;
               background: url(pattern.png) center/6px 9px round; }
      </style>
      <body>`)
}

// TestDrawBackgroundRepeatRound4 is test_background_repeat_round_4 in tests/draw/test_background.py.
func TestDrawBackgroundRepeatRound4(t *testing.T) {
	assertPixels(t, "background_repeat_round_4", `
        __________
        _rBBBrBBB_
        _rBBBrBBB_
        _rBBBrBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        __________
    `, `
      <style>
        @page { size: 10px 14px }
        html { background: #fff }
        body { margin: 1px; height: 12px;
               image-rendering: pixelated;
               background: url(pattern.png) center/5px 9px round; }
      </style>
      <body>`)
}

// TestDrawBackgroundClip is test_background_clip in tests/draw/test_background.py.
func TestDrawBackgroundClip(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"#00f border-box", `
        ________
        _BBBBBB_
        _BBBBBB_
        _BBBBBB_
        _BBBBBB_
        _BBBBBB_
        _BBBBBB_
        ________
    `, `
      <style>
        @page { size: 8px }
        html { background: #fff }
        body { margin: 1px; padding: 1px; height: 2px;
               border: 1px solid transparent;
               background: #00f border-box }
      </style>
      <body>`, false},
		{"#00f padding-box", `
        ________
        ________
        __BBBB__
        __BBBB__
        __BBBB__
        __BBBB__
        ________
        ________
    `, `
      <style>
        @page { size: 8px }
        html { background: #fff }
        body { margin: 1px; padding: 1px; height: 2px;
               border: 1px solid transparent;
               background: #00f padding-box }
      </style>
      <body>`, false},
		{"#00f content-box", `
        ________
        ________
        ________
        ___BB___
        ___BB___
        ________
        ________
        ________
    `, `
      <style>
        @page { size: 8px }
        html { background: #fff }
        body { margin: 1px; padding: 1px; height: 2px;
               border: 1px solid transparent;
               background: #00f content-box }
      </style>
      <body>`, false},
		{"url(pattern.png) padding-box, #0f0", `
        ________
        _GGGGGG_
        _GrBBBG_
        _GBBBBG_
        _GBBBBG_
        _GBBBBG_
        _GGGGGG_
        ________
    `, `
      <style>
        @page { size: 8px }
        html { background: #fff }
        body { margin: 1px; padding: 1px; height: 2px;
               border: 1px solid transparent;
               background: url(pattern.png) padding-box, #0f0 }
      </style>
      <body>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "background_clip", tc.pixels, tc.html)
		})
	}
}

// TestDrawBackgroundSize is test_background_size in tests/draw/test_background.py.
func TestDrawBackgroundSize(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"0", `
         ____________
         ____________
         ____________
         ___rrBBBBBB_
         ___rrBBBBBB_
         ___BBBBBBBB_
         ___BBBBBBBB_
         ___BBBBBBBB_
         ___BBBBBBBB_
         ___BBBBBBBB_
         ___BBBBBBBB_
         ____________
     `, `
       <style>
         @page { size: 12px }
         html { background: #fff }
         body { margin: 1px; height: 10px;
                /* Use nearest neighbor algorithm for image resizing: */
                image-rendering: pixelated;
                background: url(pattern.png) no-repeat
                            bottom right / 80% 8px; }
       </style>
       <body>`, false},
		{"1", `
         ____________
         ____________
         ____________
         ____________
         ____________
         ____________
         ____________
         _______rBBB_
         _______BBBB_
         _______BBBB_
         _______BBBB_
         ____________
     `, `
       <style>
         @page { size: 12px }
         html { background: #fff }
         body { margin: 1px; height: 10px;
                /* Use nearest neighbor algorithm for image resizing: */
                image-rendering: pixelated;
                background: url(pattern.png) bottom right/auto no-repeat }
       </style>
       <body>`, false},
		{"2", `
         ______________
         _rrBBBBBB_____
         _rrBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         ______________
     `, `
       <style>
         @page { size: 14px 10px }
         html { background: #fff }
         body { margin: 1px; height: 8px;
                /* Use nearest neighbor algorithm for image resizing: */
                image-rendering: pixelated;
                background: url(pattern.png) no-repeat;
                background-size: contain }
       </style>
       <body>`, false},
		{"3", `
         ______________
         _rrBBBBBB_____
         _rrBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         ______________
     `, `
       <style>
         @page { size: 14px 10px }
         html { background: #fff }
         body { margin: 1px; height: 8px;
                /* Use nearest neighbor algorithm for image resizing: */
                image-rendering: pixelated;
                background: url(pattern.png) no-repeat left / auto 8px;
                clip: auto; /* no-op to cover more validation */ }
       </style>
       <body>`, false},
		{"4", `
         ______________
         _rrBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         _BBBBBBBB_____
         ______________
         ______________
         ______________
         ______________
         ______________
     `, `
       <style>
         @page { size: 14px 10px }
         html { background: #fff }
         body { margin: 1px; height: 8px;
                /* Use nearest neighbor algorithm for image resizing: */
                image-rendering: pixelated;
                background: url(pattern.png) no-repeat 0 0 / 8px 4px;
                clip: auto; /* no-op to cover more validation */ }
       </style>
       <body>`, false},
		{"5", `
         ______________
         _rrrBBBBBBBBB_
         _rrrBBBBBBBBB_
         _rrrBBBBBBBBB_
         _BBBBBBBBBBBB_
         _BBBBBBBBBBBB_
         _BBBBBBBBBBBB_
         _BBBBBBBBBBBB_
         _BBBBBBBBBBBB_
         ______________
     `, `
       <style>
         @page { size: 14px 10px }
         html { background: #fff }
         body { margin: 1px; height: 8px;
                /* Use nearest neighbor algorithm for image resizing: */
                image-rendering: pixelated;
                background: url(pattern.png) no-repeat right 0/cover }
       </style>
       <body>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "background_size", tc.pixels, tc.html)
		})
	}
}

// TestDrawBleedBackgroundSize is test_bleed_background_size in tests/draw/test_background.py.
func TestDrawBleedBackgroundSize(t *testing.T) {
	assertPixels(t, "bleed_background_size", `
        RRRR
        RRRR
        RRRR
        RRRR
    `, `
      <style>
         @page { size: 2px; bleed: 1px; background: red }
      </style>
      <body>`)
}

// TestDrawBackgroundSizeClip is test_background_size_clip in tests/draw/test_background.py.
func TestDrawBackgroundSizeClip(t *testing.T) {
	assertPixels(t, "background_size_clip", `
        BBBB
        BRBB
        BBBB
        BBBB
    `, `
      <style>
         @page { size: 4px; margin: 1px;
                 background: url(pattern.png) red;
                 background-clip: content-box }
      </style>
      <body>`)
}

// TestDrawPageBackgroundFixed is test_page_background_fixed in tests/draw/test_background.py.
func TestDrawPageBackgroundFixed(t *testing.T) {
	assertPixels(t, "page_background_fixed", `
        RBBB
        BBBB
        BBBB
        BBBB
    `, `
      <style>
         @page { size: 4px; margin: 1px;
                 background: url(pattern.png) red;
                 background-attachment: fixed; }
      </style>
      <body>`)
}

// TestDrawPageBackgroundFixedBleed is test_page_background_fixed_bleed in tests/draw/test_background.py.
func TestDrawPageBackgroundFixedBleed(t *testing.T) {
	assertPixels(t, "page_background_fixed_bleed", `
        RRRRRR
        RRBBBR
        RBBBBR
        RBBBBR
        RBBBBR
        RRRRRR
    `, `
      <style>
         @page { size: 4px; margin: 1px; bleed: 1px;
                 background: url(pattern.png) no-repeat red;
                 background-attachment: fixed; }
      </style>
      <body>`)
}

// TestDrawBleedBackgroundSizeClip is test_bleed_background_size_clip in tests/draw/test_background.py.
func TestDrawBleedBackgroundSizeClip(t *testing.T) {
	assertPixels(t, "bleed_background_size_clip", `
        BBBBBB
        BBBBBB
        BBRBBB
        BBBBBB
        BBBBBB
        BBBBBB
    `, `
      <style>
         @page { size: 4px; bleed: 1px; margin: 1px;
                 background: url(pattern.png) red;
                 background-clip: content-box }
      </style>
      <body>`)
}

// TestDrawMarksCrop is test_marks_crop in tests/draw/test_background.py.
func TestDrawMarksCrop(t *testing.T) {
	assertPixels(t, "marks_crop", `
        KK__KK
        K____K
        ______
        ______
        K____K
        KK__KK
    `, `
      <style>
         @page { size: 4px; bleed: 1px; margin: 1px; marks: crop }
      </style>
      <body>`)
}

// TestDrawMarksCross is test_marks_cross in tests/draw/test_background.py.
func TestDrawMarksCross(t *testing.T) {
	assertPixels(t, "marks_cross", `
        __KK__
        ______
        K____K
        K____K
        ______
        __KK__
    `, `
      <style>
         @page { size: 4px; bleed: 1px; margin: 1px; marks: cross }
      </style>
      <body>`)
}

// TestDrawMarksCropCross is test_marks_crop_cross in tests/draw/test_background.py.
func TestDrawMarksCropCross(t *testing.T) {
	assertPixels(t, "marks_crop_cross", `
        KKKKKK
        K____K
        K____K
        K____K
        K____K
        KKKKKK
    `, `
      <style>
         @page { size: 4px; bleed: 1px; margin: 1px; marks: crop cross }
      </style>
      <body>`)
}

// TestDrawMarksCropOnTop is test_marks_crop_on_top in tests/draw/test_background.py.
func TestDrawMarksCropOnTop(t *testing.T) {
	assertPixels(t, "marks_crop_on_top", `
        KK__KK
        KB___K
        ______
        ______
        K____K
        KK__KK
    `, `
      <style>
         @page { size: 4px; bleed: 1px; margin: 1px; marks: crop }
      </style>
      <div style="width: 2px; height: 2px; background: blue; margin: -2px 0 0 -2px">`)
}
