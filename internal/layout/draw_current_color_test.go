// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_current_color.py: currentcolor.

package layout

import "testing"

// TestDrawCurrentColor1 is test_current_color_1 in tests/draw/test_current_color.py.
func TestDrawCurrentColor1(t *testing.T) {
	assertPixels(t, "current_color_1", `GG
GG`, `
      <style>
        @page { size: 2px }
        html, body { height: 100%; margin: 0 }
        html { color: red; background: currentColor }
        body { color: lime; background: inherit }
      </style>
      <body>`)
}

// TestDrawCurrentColor2 is test_current_color_2 in tests/draw/test_current_color.py.
func TestDrawCurrentColor2(t *testing.T) {
	assertPixels(t, "current_color_2", `GG
GG`, `
      <style>
        @page { size: 2px }
        html { color: red; border-color: currentColor }
        body { color: lime; border: 1Px solid; border-color: inherit;
               margin: 0 }
      </style>
      <body>`)
}

// TestDrawCurrentColor3 is test_current_color_3 in tests/draw/test_current_color.py.
func TestDrawCurrentColor3(t *testing.T) {
	assertPixels(t, "current_color_3", `GG
GG`, `
      <style>
        @page { size: 2px }
        html { color: red; outline-color: currentColor }
        body { color: lime; outline: 1px solid; outline-color: inherit;
               margin: 1px }
      </style>
      <body>`)
}

// TestDrawCurrentColor4 is test_current_color_4 in tests/draw/test_current_color.py.
func TestDrawCurrentColor4(t *testing.T) {
	assertPixels(t, "current_color_4", `GG
GG`, `
      <style>
        @page { size: 2px }
        html { color: red; border-color: currentColor; }
        body { margin: 0 }
        table { border-collapse: collapse;
                color: lime; border: 1PX solid; border-color: inherit }
      </style>
      <table><td>`)
}

// TestDrawCurrentColorSvg1 is test_current_color_svg_1 in tests/draw/test_current_color.py.
func TestDrawCurrentColorSvg1(t *testing.T) {
	assertPixels(t, "current_color_svg_1", `KK
KK`, `
      <style>
        @page { size: 2px }
        svg { display: block }
      </style>
      <svg xmlns="http://www.w3.org/2000/svg"
           width="2" height="2" fill="currentColor">
        <rect width="2" height="2"></rect>
      </svg>`)
}

// TestDrawCurrentColorSvg2 is test_current_color_svg_2 in tests/draw/test_current_color.py.
func TestDrawCurrentColorSvg2(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "current_color_svg_2", `GG
GG`, `
      <style>
        @page { size: 2px }
        svg { display: block }
        body { color: lime }
      </style>
      <svg xmlns="http://www.w3.org/2000/svg"
           width="2" height="2">
        <rect width="2" height="2" fill="currentColor"></rect>
      </svg>`)
}

// TestDrawCurrentColorVariable is test_current_color_variable in tests/draw/test_current_color.py.
func TestDrawCurrentColorVariable(t *testing.T) {
	assertPixels(t, "current_color_variable", `GG
GG`, `
      <style>
        @page { size: 2px }
        html { color: lime; font-family: weasyprint; --var: currentColor }
        div { color: var(--var); font-size: 2px; line-height: 1 }
      </style>
      <div>aa`)
}

// TestDrawCurrentColorVariableBorder is test_current_color_variable_border in tests/draw/test_current_color.py.
func TestDrawCurrentColorVariableBorder(t *testing.T) {
	assertPixels(t, "current_color_variable_border", `GG
GG`, `
      <style>
        @page { size: 2px }
        html { color: lime; --var: currentColor }
        div { color: var(--var); width: 0; height: 0; border: 1px solid }
      </style>
      <div>`)
}
