// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_column.py.

package layout

import "testing"

// TestDrawColumnRule1 is test_column_rule_1 in tests/draw/test_column.py.
func TestDrawColumnRule1(t *testing.T) {
	assertPixels(t, "column_rule_1", `
        a_r_a
        a_r_a
        _____
    `, `
      <style>
        img { display: inline-block; width: 1px; height: 1px }
        div { columns: 2; column-rule-style: solid;
              column-rule-width: 1px; column-gap: 3px;
              column-rule-color: red }
        body { margin: 0; font-size: 0 }
        @page { margin: 0; size: 5px 3Px }
      </style>
      <div>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
      </div>`)
}

// TestDrawColumnRule2 is test_column_rule_2 in tests/draw/test_column.py.
func TestDrawColumnRule2(t *testing.T) {
	assertPixels(t, "column_rule_2", `
        a_r_a
        a___a
        a_r_a
    `, `
      <style>
        img { display: inline-block; width: 1px; height: 1px }
        div { columns: 2; column-rule-style: dotted;
              column-rule-width: 1px; column-gap: 3px;
              column-rule-color: red }
        body { margin: 0; font-size: 0 }
        @page { margin: 0; size: 5px 3px }
      </style>
      <div>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
      </div>`)
}

// TestDrawColumnRuleSpan is test_column_rule_span in tests/draw/test_column.py.
func TestDrawColumnRuleSpan(t *testing.T) {
	assertPixels(t, "column_rule_span", `
        ___________
        ___________
        ___________
        ___a_______
        ___a_r_a___
        ___a_r_a___
        ___________
        ___________
        ___________
    `, `
      <style>
        img { display: inline-block; width: 1px; height: 1px }
        div { columns: 2; column-rule: 1px solid red; column-gap: 3px }
        article { column-span: all }
        body { margin: 0; font-size: 0 }
        @page { margin: 3px; size: 11px 9px }
      </style>
      <div>
        <article>
          <img src=blue.jpg>
        </article>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
      </div>`)
}

// TestDrawColumnRuleNormal is test_column_rule_normal in tests/draw/test_column.py.
func TestDrawColumnRuleNormal(t *testing.T) {
	// Regression test for #2217.
	assertPixels(t, "column_rule_normal", `
        a___a
        a___a
        _____
    `, `
      <style>
        img { display: inline-block; width: 1px; height: 1px }
        div { columns: 2; column-gap: normal }
        body { margin: 0; font: 3px/0 weasyprint }
        @page { margin: 0; size: 5PX 3px }
      </style>
      <div>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
      </div>`)
}

// TestDrawColumnRuleRTL is test_column_rule_rtl in tests/draw/test_column.py.
func TestDrawColumnRuleRTL(t *testing.T) {
	assertPixels(t, "column_rule_rtl", `
        a_r_a
        a___a
        a_r_a
    `, `
      <style>
        img { display: inline-block; width: 1px; height: 1px }
        div { columns: 2; column-rule-style: dotted;
              column-rule-width: 1px; column-gap: 3px;
              column-rule-color: red; direction: rtl }
        body { margin: 0; font-size: 0 }
        @page { margin: 0; size: 5px 3px }
      </style>
      <div>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
        <img src=blue.jpg>
      </div>`)
}
