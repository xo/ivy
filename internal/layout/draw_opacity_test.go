// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_opacity.py: the opacity.

package layout

import "testing"

// TestDrawOpacityZero is test_opacity_zero in tests/draw/test_opacity.py.
func TestDrawOpacityZero(t *testing.T) {
	assertSameRenderings(t, "opacity_zero", 0,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div></div><div style="opacity: 0"></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div></div><div style="opacity: 0%"></div>`)
}

// TestDrawOpacityNormalRange is test_opacity_normal_range in tests/draw/test_opacity.py.
func TestDrawOpacityNormalRange(t *testing.T) {
	assertSameRenderings(t, "opacity_normal_range", 0,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div style="background: rgb(102, 102, 102)"></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div style="opacity: 0.6"></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div style="opacity: 60%"></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div style="opacity: 60.0%"></div>`)
}

// TestDrawOpacityNested is test_opacity_nested in tests/draw/test_opacity.py.
func TestDrawOpacityNested(t *testing.T) {
	assertSameRenderings(t, "opacity_nested", 0,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div style="background: rgb(102, 102, 102)"></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div style="opacity: 0.6"></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    
          <div style="background: none; opacity: 0.666666">
            <div style="opacity: 0.9"></div>
          </div>
        `)
}

// TestDrawOpacityPercentClampDown is test_opacity_percent_clamp_down in tests/draw/test_opacity.py.
func TestDrawOpacityPercentClampDown(t *testing.T) {
	assertSameRenderings(t, "opacity_percent_clamp_down", 0,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div style="opacity: 1.2"></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div style="opacity: 120%"></div>`)
}

// TestDrawOpacityPercentClampUp is test_opacity_percent_clamp_up in tests/draw/test_opacity.py.
func TestDrawOpacityPercentClampUp(t *testing.T) {
	assertSameRenderings(t, "opacity_percent_clamp_up", 0,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div></div><div style="opacity: -0.2"></div>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    <div></div><div style="opacity: -20%"></div>`)
}

// TestDrawOpacityBlack is test_opacity_black in tests/draw/test_opacity.py.
func TestDrawOpacityBlack(t *testing.T) {
	// Adapted: okage rounds a color with an alpha and a group with an
	// opacity to values that differ by 1.
	assertSameRenderings(t, "opacity_black", 1,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    a<span style="color: rgb(0, 0, 0, 0.5)">b</span>`,
		`
    <style>
        @page { size: 60px 60px }
        div { background: #000; width: 20px; height: 20px }
    </style>
    a<span style="opacity: 0.5">b</span>`)
}
