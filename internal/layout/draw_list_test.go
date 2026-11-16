// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_list.py: the drawing of the lists.

package layout

import "testing"

// TestDrawListStyleImage is test_list_style_image in tests/draw/test_list.py.
func TestDrawListStyleImage(t *testing.T) {
	t.Run("0", func(t *testing.T) {
		assertPixels(t, "list_style_image_0", `
        ____________
        ____________
        ___rBBB_____
        ___BBBB_____
        ___BBBB_____
        ___BBBB_____
        ____________
        ____________
        ____________
        ____________
     `, `
      <style>
        @page { size: 12px 10px }
        body { margin: 0; font-family: DejaVu Sans, sans }
        ul { margin: 2px 2px 0 7px; list-style: url(pattern.png) outside;
             font-size: 2px }
      </style>
      <ul><li></li></ul>`)
	})
	t.Run("1", func(t *testing.T) {
		assertPixels(t, "list_style_image_1", `
        ____________
        ____________
        _______rBBB_
        _______BBBB_
        _______BBBB_
        _______BBBB_
        ____________
        ____________
        ____________
        ____________
     `, `
      <style>
        @page { size: 12px 10px }
        body { margin: 0; font-family: DejaVu Sans, sans }
        ul { margin: 2px 2px 0 7px; list-style: url(pattern.png) inside;
             font-size: 2px }
      </style>
      <ul><li></li></ul>`)
	})
}

// TestDrawListStyleImageNone is test_list_style_image_none in tests/draw/test_list.py.
func TestDrawListStyleImageNone(t *testing.T) {
	assertPixels(t, "list_style_image_none", `
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
        body { margin: 0; font-family: DejaVu Sans, sans }
        ul { margin: 0 0 0 5px; list-style: none; font-size: 2px; }
      </style>
      <ul><li>`)
}
