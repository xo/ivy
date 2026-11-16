// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_page.py: the pages.

package layout

import "testing"

// TestDrawNthPage is test_nth_page in tests/draw/test_page.py.
func TestDrawNthPage(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"2n", `_
R
_
R
_
R
_
R
_
R`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(2n) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"even", `_
R
_
R
_
R
_
R
_
R`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(even) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"2n+1", `R
_
R
_
R
_
R
_
R
_`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(2n+1) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"odd", `R
_
R
_
R
_
R
_
R
_`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(odd) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"2n+3", `_
_
R
_
R
_
R
_
R
_`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(2n+3) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"n", `R
R
R
R
R
R
R
R
R
R`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(n) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"n-1", `R
R
R
R
R
R
R
R
R
R`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(n-1) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"-n+3", `R
R
R
_
_
_
_
_
_
_`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(-n+3) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"-2n+3", `R
_
R
_
_
_
_
_
_
_`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(-2n+3) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"-n-3", `_
_
_
_
_
_
_
_
_
_`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(-n-3) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"3", `_
_
R
_
_
_
_
_
_
_`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(3) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
		{"0n+0", `_
_
_
_
_
_
_
_
_
_`, `
      <style>
        @page { size: 1px 1px }
        @page:nth(0n+0) { background: red }
        p { break-after: page }
      </style>
    <p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p><p></p>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "nth_page", tc.pixels, tc.html)
		})
	}
}
