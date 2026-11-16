// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_visibility.py.

package layout

import "testing"

// TestDrawVisibility1 is test_visibility_1 in tests/draw/test_visibility.py.
func TestDrawVisibility1(t *testing.T) {
	assertPixels(t, "visibility_1", `
        ____________
        _rBBB_rBBB__
        _BBBB_BBBB__
        _BBBB_BBBB__
        _BBBB_BBBB__
        ____________
        ____________
    `, `
  <style>
    @page { size: 12px 7px }
    body { font-size: 0; line-height: 0 }
    img { margin: 1px 0 0 1px }
    
  </style>
  <div>
    <img src="pattern.png">
    <span><img src="pattern.png"></span>
  </div>`)
}

// TestDrawVisibility2 is test_visibility_2 in tests/draw/test_visibility.py.
func TestDrawVisibility2(t *testing.T) {
	assertPixels(t, "visibility_2", `
        ____________
        ____________
        ____________
        ____________
        ____________
        ____________
        ____________
    `, `
  <style>
    @page { size: 12px 7px }
    body { font-size: 0; line-height: 0 }
    img { margin: 1px 0 0 1px }
    div { visibility: hidden }
  </style>
  <div>
    <img src="pattern.png">
    <span><img src="pattern.png"></span>
  </div>`)
}

// TestDrawVisibility3 is test_visibility_3 in tests/draw/test_visibility.py.
func TestDrawVisibility3(t *testing.T) {
	assertPixels(t, "visibility_3", `
        ____________
        ______rBBB__
        ______BBBB__
        ______BBBB__
        ______BBBB__
        ____________
        ____________
    `, `
  <style>
    @page { size: 12px 7px }
    body { font-size: 0; line-height: 0 }
    img { margin: 1px 0 0 1px }
    div { visibility: hidden } span { visibility: visible }
  </style>
  <div>
    <img src="pattern.png">
    <span><img src="pattern.png"></span>
  </div>`)
}

// TestDrawVisibility4 is test_visibility_4 in tests/draw/test_visibility.py.
func TestDrawVisibility4(t *testing.T) {
	assertPixels(t, "visibility_4", `
        ____________
        _rBBB_rBBB__
        _BBBB_BBBB__
        _BBBB_BBBB__
        _BBBB_BBBB__
        ____________
        ____________
    `, `
  <style>
    @page { size: 12px 7px }
    body { font-size: 0; line-height: 0 }
    img { margin: 1px 0 0 1px }
    @page { visibility: hidden; background: red }
  </style>
  <div>
    <img src="pattern.png">
    <span><img src="pattern.png"></span>
  </div>`)
}
