// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_transform.py: the transformations.

package layout

import "testing"

// TestDraw2dTransform1 is test_2d_transform_1 in tests/draw/test_transform.py.
func TestDraw2dTransform1(t *testing.T) {
	assertPixels(t, "2d_transform_1", `
        __________
        __________
        __BBBr____
        __BBBB____
        __BBBB____
        __BBBB____
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page { size: 10px; margin: 2px }
        body { font-size: 0 }
        img { transform: rotate(90deg) }
      </style>
      <body><img src="pattern.png">`)
}

// TestDraw2dTransform2 is test_2d_transform_2 in tests/draw/test_transform.py.
func TestDraw2dTransform2(t *testing.T) {
	assertPixels(t, "2d_transform_2", `
        __________
        __________
        _____BBBr_
        _____BBBB_
        _____BBBB_
        _____BBBB_
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page { size: 10px; margin: 2px }
        body { font-size: 0 }
        img { transform: translateX(3px) rotate(90deg) }
      </style>
      <body><img src="pattern.png">`)
}

// TestDraw2dTransform3 is test_2d_transform_3 in tests/draw/test_transform.py.
func TestDraw2dTransform3(t *testing.T) {
	assertPixels(t, "2d_transform_3", `
        __________
        __________
        __________
        __________
        __________
        __BBBr____
        __BBBB____
        __BBBB____
        __BBBB____
        __________
    `, `
      <style>
        @page { size: 10px; margin: 2px }
        body { font-size: 0 }
        img { transform: rotate(90deg) translateX(3px) }
      </style>
      <body><img src="pattern.png">`)
}

// TestDraw2dTransform4 is test_2d_transform_4 in tests/draw/test_transform.py.
func TestDraw2dTransform4(t *testing.T) {
	assertPixels(t, "2d_transform_4", `
        __________
        __________
        __________
        __________
        __________
        __BBBr____
        __BBBB____
        __BBBB____
        __BBBB____
        __________
    `, `
      <style>
        @page { size: 10px; margin: 2px }
        div { transform: rotate(90deg); font-size: 0; width: 4px }
        img { transform: translateX(3px) }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDraw2dTransform5 is test_2d_transform_5 in tests/draw/test_transform.py.
func TestDraw2dTransform5(t *testing.T) {
	assertPixels(t, "2d_transform_5", `
        ________
        ________
        __BBBr__
        __BBBB__
        __BBBB__
        __BBBB__
        ________
        ________
    `, `
      <style>
        @page { size: 8px; margin: 2px; }
        div { transform: matrix(-1, 0, 0, 1, 0, 0); font-size: 0 }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDraw2dTransform6 is test_2d_transform_6 in tests/draw/test_transform.py.
func TestDraw2dTransform6(t *testing.T) {
	assertPixels(t, "2d_transform_6", `
        ________
        ________
        ________
        ________
        ___rBBB_
        ___BBBB_
        ___BBBB_
        ___BBBB_
    `, `
      <style>
        @page { size: 8px; margin: 2px; }
        div { transform: translate(1px, 2px); font-size: 0 }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDraw2dTransform7 is test_2d_transform_7 in tests/draw/test_transform.py.
func TestDraw2dTransform7(t *testing.T) {
	assertPixels(t, "2d_transform_7", `
        ________
        ________
        ___rBBB_
        ___BBBB_
        ___BBBB_
        ___BBBB_
        ________
        ________
    `, `
      <style>
        @page { size: 8px; margin: 2px; }
        div { transform: translate(25%, 0); font-size: 0 }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDraw2dTransform8 is test_2d_transform_8 in tests/draw/test_transform.py.
func TestDraw2dTransform8(t *testing.T) {
	assertPixels(t, "2d_transform_8", `
        ________
        ________
        _____rBB
        _____BBB
        _____BBB
        _____BBB
        ________
        ________
    `, `
      <style>
        @page { size: 8px; margin: 2px; }
        div { transform: translateX(0.25em); font-size: 12px }
        div div { font-size: 0 }
      </style>
      <div><div><img src="pattern.png"></div></div>`)
}

// TestDraw2dTransform9 is test_2d_transform_9 in tests/draw/test_transform.py.
func TestDraw2dTransform9(t *testing.T) {
	assertPixels(t, "2d_transform_9", `
        ________
        __rBBB__
        __BBBB__
        __BBBB__
        __BBBB__
        ________
        ________
        ________
    `, `
      <style>
        @page { size: 8px; margin: 2px; }
        div { transform: translateY(-1px); font-size: 0 }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDraw2dTransform10 is test_2d_transform_10 in tests/draw/test_transform.py.
func TestDraw2dTransform10(t *testing.T) {
	assertPixels(t, "2d_transform_10", `
        __________
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
        @page { size: 10px; margin: 2px; }
        div { transform: scale(2, 2);
              transform-origin: 1px 1px 1px;
              image-rendering: pixelated;
              font-size: 0 }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDraw2dTransform11 is test_2d_transform_11 in tests/draw/test_transform.py.
func TestDraw2dTransform11(t *testing.T) {
	assertPixels(t, "2d_transform_11", `
        __________
        __rBBB____
        __rBBB____
        __BBBB____
        __BBBB____
        __BBBB____
        __BBBB____
        __BBBB____
        __BBBB____
        __________
    `, `
      <style>
        @page { size: 10px; margin: 2px; }
        div { transform: scale(1, 2);
              transform-origin: 1px 1px;
              image-rendering: pixelated;
              font-size: 0 }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDraw2dTransform12 is test_2d_transform_12 in tests/draw/test_transform.py.
func TestDraw2dTransform12(t *testing.T) {
	assertPixels(t, "2d_transform_12", `
        __________
        __rBBB____
        __rBBB____
        __BBBB____
        __BBBB____
        __BBBB____
        __BBBB____
        __BBBB____
        __BBBB____
        __________
    `, `
      <style>
        @page { size: 10px; margin: 2px; }
        div { transform: scaleY(2);
              transform-origin: 1px 1px 0;
              image-rendering: pixelated;
              font-size: 0 }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDraw2dTransform13 is test_2d_transform_13 in tests/draw/test_transform.py.
func TestDraw2dTransform13(t *testing.T) {
	assertPixels(t, "2d_transform_13", `
        __________
        __________
        _rrBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        _BBBBBBBB_
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page { size: 10px; margin: 2px; }
        div { transform: scaleX(2);
              transform-origin: 1px 1px;
              image-rendering: pixelated;
              font-size: 0 }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDraw2dTransformOpacity is test_2d_transform_opacity in tests/draw/test_transform.py.
func TestDraw2dTransformOpacity(t *testing.T) {
	assertPixels(t, "2d_transform_opacity", `
        __________
        __________
        __________
        _ss_______
        _ss_______
        __________
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page { size: 10px; margin: 2px; }
        div { transform: translate(-1px, 1px); background: red;
              height: 2px; width: 2px; opacity: 0.5 }
      </style>
      <div></div>`)
}
