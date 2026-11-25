// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_gradient.py: how the gradients are drawn.

package layout

import "testing"

// TestDrawLinearGradients1 is test_linear_gradients_1 in tests/draw/test_gradient.py.
func TestDrawLinearGradients1(t *testing.T) {
	assertPixels(t, "linear_gradients_1", `
        _____
        _____
        _____
        BBBBB
        BBBBB
        RRRRR
        RRRRR
        RRRRR
        RRRRR
    `, `<style>@page { size: 5px 9px; background: linear-gradient(
      white, white 3px, blue 0, blue 5px, red 0, red
    )`)
}

// TestDrawLinearGradients2 is test_linear_gradients_2 in tests/draw/test_gradient.py.
func TestDrawLinearGradients2(t *testing.T) {
	assertPixels(t, "linear_gradients_2", `
        _____
        _____
        _____
        BBBBB
        BBBBB
        RRRRR
        RRRRR
        RRRRR
        RRRRR
    `, `<style>@page { size: 5px 9px; background: linear-gradient(
      white 3px, blue 0, blue 5px, red 0
    )`)
}

// TestDrawLinearGradients3 is test_linear_gradients_3 in tests/draw/test_gradient.py.
func TestDrawLinearGradients3(t *testing.T) {
	assertPixels(t, "linear_gradients_3", `
        ___BBrrrr
        ___BBrrrr
        ___BBrrrr
        ___BBrrrr
        ___BBrrrr
    `, `<style>@page { size: 9px 5px; background: linear-gradient(
      to right, white 3px, blue 0, blue 5px, red 0
    )`)
}

// TestDrawLinearGradients4 is test_linear_gradients_4 in tests/draw/test_gradient.py.
func TestDrawLinearGradients4(t *testing.T) {
	assertPixels(t, "linear_gradients_4", `
        BBBBBBrrrr
        BBBBBBrrrr
        BBBBBBrrrr
        BBBBBBrrrr
        BBBBBBrrrr
    `, `<style>@page { size: 10px 5px; background: linear-gradient(
      to right, blue 5px, blue 6px, red 6px, red 9px
    )`)
}

// TestDrawLinearGradients5 is test_linear_gradients_5 in tests/draw/test_gradient.py.
func TestDrawLinearGradients5(t *testing.T) {
	assertPixels(t, "linear_gradients_5", `
        rBrrzBrrrB
        rBrrrBrrrB
        rBrrrBrrrB
        rBrrrBrrrB
        rBrrzBrrrB
    `, `
      <style>@page { size: 10px 5px; background: repeating-linear-gradient(
      to right, blue 50%, blue 60%, red 60%, red 90%
    )`)
}

// TestDrawLinearGradients6 is test_linear_gradients_6 in tests/draw/test_gradient.py.
func TestDrawLinearGradients6(t *testing.T) {
	assertPixels(t, "linear_gradients_6", `
        BBBrrrrrr
        BBBrrrrrr
        BBBrrrrrr
        BBBrrrrrr
        BBBrrrrrr
    `, `<style>@page { size: 9px 5px; background: linear-gradient(
      to right, blue 3px, blue 3px, red 3px, red 3px
    )`)
}

// TestDrawLinearGradients7 is test_linear_gradients_7 in tests/draw/test_gradient.py.
func TestDrawLinearGradients7(t *testing.T) {
	assertPixels(t, "linear_gradients_7", `
        hhhhhhhhh
        hhhhhhhhh
        hhhhhhhhh
        hhhhhhhhh
        hhhhhhhhh
    `, `<style>@page { size: 9px 5px; background:
    repeating-linear-gradient(
      to right, black 3px, black 3px, #800080 3px, #800080 3px
    )`)
}

// TestDrawLinearGradients8 is test_linear_gradients_8 in tests/draw/test_gradient.py.
func TestDrawLinearGradients8(t *testing.T) {
	assertPixels(t, "linear_gradients_8", `
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
    `, `<style>@page { size: 9px 5px; background:
      repeating-linear-gradient(to right, blue 3px)`)
}

// TestDrawLinearGradients9 is test_linear_gradients_9 in tests/draw/test_gradient.py.
func TestDrawLinearGradients9(t *testing.T) {
	assertPixels(t, "linear_gradients_9", `
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
    `, `<style>@page { size: 9px 5px; background:
      repeating-linear-gradient(45deg, blue 3px)`)
}

// TestDrawLinearGradients10 is test_linear_gradients_10 in tests/draw/test_gradient.py.
func TestDrawLinearGradients10(t *testing.T) {
	assertPixels(t, "linear_gradients_10", `
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
    `, `<style>@page { size: 9px 5px; background: linear-gradient(
      45deg, blue 3px, red 3px, red 3px, blue 3px
    )`)
}

// TestDrawLinearGradients11 is test_linear_gradients_11 in tests/draw/test_gradient.py.
func TestDrawLinearGradients11(t *testing.T) {
	assertPixels(t, "linear_gradients_11", `
        BBBrBBBBB
        BBBrBBBBB
        BBBrBBBBB
        BBBrBBBBB
        BBBrBBBBB
    `, `<style>@page { size: 9px 5px; background: linear-gradient(
      to right, blue 3px, red 3px, red 4px, blue 4px
    )`)
}

// TestDrawLinearGradients12 is test_linear_gradients_12 in tests/draw/test_gradient.py.
func TestDrawLinearGradients12(t *testing.T) {
	assertPixels(t, "linear_gradients_12", `
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
        BBBBBBBBB
    `, `<style>@page { size: 9px 5px; background:
      repeating-linear-gradient(to right, red 3px, blue 3px, blue 4px, red 4px
    )`)
}

// TestDrawLinearGradients13 is test_linear_gradients_13 in tests/draw/test_gradient.py.
func TestDrawLinearGradients13(t *testing.T) {
	assertPixels(t, "linear_gradients_13", `
        _____
        _____
        _____
        SSSSS
        SSSSS
        RRRRR
        RRRRR
        RRRRR
        RRRRR
    `, `<style>@page { size: 5px 9px; background: linear-gradient(
      white, white 3px, rgba(255, 0, 0, 0.751) 0, rgba(255, 0, 0, 0.751) 5px,
      red 0, red
    )`)
}

// TestDrawLinearGradientsCurrentcolor is test_linear_gradients_currentcolor in tests/draw/test_gradient.py.
func TestDrawLinearGradientsCurrentcolor(t *testing.T) {
	assertPixels(t, "linear_gradients_currentcolor", `
        KKKKK
        KKKKK
        KKKKK
        KKKKK
        KKKKK
    `, `<style>@page { size: 5px 5px; background: linear-gradient(currentcolor)`)
}

// TestDrawLinearGradientsHints is test_linear_gradients_hints in tests/draw/test_gradient.py.
func TestDrawLinearGradientsHints(t *testing.T) {
	assertPixels(t, "linear_gradients_hints", `
        _____
        _____
        _____
        _____
        _____
        _____
        _____
        _____
        zzzzz
    `, `<style>@page { size: 5px 9px; background: linear-gradient(
      white, 8.9px, red
    )`)
}

// TestDrawLinearGradientsHintsPercentage is test_linear_gradients_hints_percentage in tests/draw/test_gradient.py.
func TestDrawLinearGradientsHintsPercentage(t *testing.T) {
	assertPixels(t, "linear_gradients_hints_percentage", `
        _____
        _____
        _____
        _____
        _____
        _____
        _____
        _____
        zzzzz
    `, `<style>@page { size: 5px 9px; background: linear-gradient(
      white, 99%, red
    )`)
}

// TestDrawRadialGradients1 is test_radial_gradients_1 in tests/draw/test_gradient.py.
func TestDrawRadialGradients1(t *testing.T) {
	assertPixels(t, "radial_gradients_1", `
        BBBBBB
        BBBBBB
        BBBBBB
        BBBBBB
        BBBBBB
        BBBBBB
    `, `<style>@page { size: 6px; background:
      radial-gradient(red -30%, blue -10%)`)
}

// TestDrawRadialGradients2 is test_radial_gradients_2 in tests/draw/test_gradient.py.
func TestDrawRadialGradients2(t *testing.T) {
	assertPixels(t, "radial_gradients_2", `
        RRRRRR
        RRRRRR
        RRRRRR
        RRRRRR
        RRRRRR
        RRRRRR
    `, `<style>@page { size: 6px; background:
      radial-gradient(red 110%, blue 130%)`)
}

// TestDrawRadialGradients3 is test_radial_gradients_3 in tests/draw/test_gradient.py.
func TestDrawRadialGradients3(t *testing.T) {
	assertPixels(t, "radial_gradients_3", `
        BzzzzzzzzB
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzRRzzzz
        zzzzRRzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        BzzzzzzzzB
    `, `<style>@page { size: 10px 16px; background:
      radial-gradient(red 20%, blue 80%)`)
}

// TestDrawRadialGradients4 is test_radial_gradients_4 in tests/draw/test_gradient.py.
func TestDrawRadialGradients4(t *testing.T) {
	assertPixels(t, "radial_gradients_4", `
        BzzzzzzzzB
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzRRzzzz
        zzzzRRzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        BzzzzzzzzB
    `, `<style>@page { size: 10px 16px; background:
      radial-gradient(red 50%, blue 50%)`)
}

// TestDrawRadialGradients5 is test_radial_gradients_5 in tests/draw/test_gradient.py.
func TestDrawRadialGradients5(t *testing.T) {
	assertPixels(t, "radial_gradients_5", `
        SzzzzzzzzS
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzRRzzzz
        zzzzRRzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        SzzzzzzzzS
    `, `<style>@page { size: 10px 16px; background:
      radial-gradient(red 50%, rgba(255, 0, 0, 0.751) 50%)`)
}

// TestDrawRadialGradientsNegative is test_radial_gradients_negative in tests/draw/test_gradient.py.
func TestDrawRadialGradientsNegative(t *testing.T) {
	assertPixels(t, "radial_gradients_negative", `
        BzzzzzzzzB
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzBBzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        BzzzRRzzzB
        BzzzRRzzzB
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzBBzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        BzzzzzzzzB
    `, `<style>@page { size: 10px 16px; background:
      radial-gradient(circle, red -1px, red 3px, blue 3px, blue 6px)`)
}

// TestDrawRadialGradientsRepeating is test_radial_gradients_repeating in tests/draw/test_gradient.py.
func TestDrawRadialGradientsRepeating(t *testing.T) {
	assertPixels(t, "radial_gradients_repeating", `
        RzzzzzzzzR
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzBBzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zBzzRRzzBz
        zBzzRRzzBz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzBBzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        RzzzzzzzzR
    `, `<style>@page { size: 10px 16px; background:
      repeating-radial-gradient(circle, red 0, red 3px, blue 3px, blue 6px)`)
}

// TestDrawRadialGradientsRepeatingOuter is test_radial_gradients_repeating_outer in tests/draw/test_gradient.py.
func TestDrawRadialGradientsRepeatingOuter(t *testing.T) {
	assertPixels(t, "radial_gradients_repeating_outer", `
        RzzzzzzzzR
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzBBzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zBzzRRzzBz
        zBzzRRzzBz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzBBzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        RzzzzzzzzR
    `, `<style>@page { size: 10px 16px; background:
      repeating-radial-gradient(circle, red 6px, red 9px, blue 9px, blue 12px)`)
}

// TestDrawRadialGradientsRepeatingOuterPartial is test_radial_gradients_repeating_outer_partial in tests/draw/test_gradient.py.
func TestDrawRadialGradientsRepeatingOuterPartial(t *testing.T) {
	assertPixels(t, "radial_gradients_repeating_outer_partial", `
        RzzzzzzzzR
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzBBzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zBzzRRzzBz
        zBzzRRzzBz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzBBzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        RzzzzzzzzR
    `, `<style>@page { size: 10px 16px; background:
      repeating-radial-gradient(circle, blue 3px, blue 6px, red 6px, red 9px)`)
}

// TestDrawRadialGradientsRepeatingNegative is test_radial_gradients_repeating_negative in tests/draw/test_gradient.py.
func TestDrawRadialGradientsRepeatingNegative(t *testing.T) {
	assertPixels(t, "radial_gradients_repeating_negative", `
        RzzzzzzzzR
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzBBzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zBzzRRzzBz
        zBzzRRzzBz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzBBzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        zzzzzzzzzz
        RzzzzzzzzR
    `, `<style>@page { size: 10px 16px; background:
      repeating-radial-gradient(circle, blue -3px, blue 0, red 0, red 3px)`)
}

// TestDrawRadialGradientsHints is test_radial_gradients_hints in tests/draw/test_gradient.py.
func TestDrawRadialGradientsHints(t *testing.T) {
	assertPixels(t, "radial_gradients_hints", `
        zzzzzz
        zzBBzz
        zBBBBz
        zBBBBz
        zzBBzz
        zzzzzz
    `, `<style>@page { size: 6px; background:
      radial-gradient(blue, 4px, white)`)
}

// TestDrawRadialGradientsHintsPercentage is test_radial_gradients_hints_percentage in tests/draw/test_gradient.py.
func TestDrawRadialGradientsHintsPercentage(t *testing.T) {
	assertPixels(t, "radial_gradients_hints_percentage", `
        zzzzzz
        zzBBzz
        zBBBBz
        zBBBBz
        zzBBzz
        zzzzzz
    `, `<style>@page { size: 6px; background:
      radial-gradient(blue, 99%, white)`)
}

// TestDrawGradientPositions is test_gradient_positions in tests/draw/test_gradient.py.
func TestDrawGradientPositions(t *testing.T) {
	assertSameRenderings(t, "gradient_positions", 0,
		`<style>@page { size: 10px 1px; background: linear-gradient(to right, blue, red, pink, green) }`,
		`<style>@page { size: 10px 1px; background: linear-gradient(to right, blue, red 33.3333%, pink, green) }`,
		`<style>@page { size: 10px 1px; background: linear-gradient(to right, blue, red 33.3333%, pink 66.6667%, green) }`,
		`<style>@page { size: 10px 1px; background: linear-gradient(to right, blue 0.0001%, red, pink, green 99.9999%) }`,
		`<style>@page { size: 10px 1px; background: linear-gradient(to right, blue 0.0001%, red 33.3333%, pink, green 99.9999%) }`,
		`<style>@page { size: 10px 1px; background: linear-gradient(to right, blue 0.0001%, red 33.3333%, pink 66.6666%, green 99.9999%) }`)
}

// TestDrawGradientHints is test_gradient_hints in tests/draw/test_gradient.py.
func TestDrawGradientHints(t *testing.T) {
	assertSameRenderings(t, "gradient_hints", 3,
		`<style>@page { size: 9px 1px; background: linear-gradient(to right, blue, red, green) }`,
		`<style>@page { size: 9px 1px; background: linear-gradient(to right, blue, 25%, red, 75%, green) }`,
		`<style>@page { size: 9px 1px; background: linear-gradient(to right, blue 0.0001%, red, green) }`,
		`<style>@page { size: 9px 1px; background: linear-gradient(to right, blue, red, green 99.9999%) }`,
		`<style>@page { size: 9px 1px; background: linear-gradient(to right, blue, red 50.00001%, green) }`,
		`<style>@page { size: 9px 1px; background: linear-gradient(to right, blue 0.0001%, 24.9999%, red 50.00001%, 75.0001%, green 99.9999%) }`)
}

// TestDrawGradientHintsAndPositions is test_gradient_hints_and_positions in tests/draw/test_gradient.py.
func TestDrawGradientHintsAndPositions(t *testing.T) {
	assertSameRenderings(t, "gradient_hints_and_positions", 0,
		`<style>@page { size: 9px 1px; background: linear-gradient(to right, blue, 10%, red, 60%, green) }`,
		`<style>@page { size: 9px 1px; background: linear-gradient(to right, blue, 10%, red 35%, 60%, green) }`,
		`<style>@page { size: 9px 1px; background: linear-gradient(to right, blue 0%, 10%, red, 60%, green 100%) }`)
}

// TestDrawGradientDegenerate is test_gradient_degenerate in tests/draw/test_gradient.py.
func TestDrawGradientDegenerate(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"blue, 100%, red", `zBBBBBBBz`, `
      <style>
        @page {
          size: 9px 1px;
          background:linear-gradient(to right ,blue, 100%, red)
        }
      </style>
    `, false},
		{"blue, 110%, red", `zBBBBBBBz`, `
      <style>
        @page {
          size: 9px 1px;
          background:linear-gradient(to right ,blue, 110%, red)
        }
      </style>
    `, false},
		{"red, 0, blue", `zBBBBBBBz`, `
      <style>
        @page {
          size: 9px 1px;
          background:linear-gradient(to right ,red, 0, blue)
        }
      </style>
    `, false},
		{"blue, 100%, red, green", `zBBBBBBBz`, `
      <style>
        @page {
          size: 9px 1px;
          background:linear-gradient(to right ,blue, 100%, red, green)
        }
      </style>
    `, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "gradient_degenerate", tc.pixels, tc.html)
		})
	}
}

// TestDrawGradientSize checks that a gradient with a background-size and
// no-repeat covers only its size. Upstream has no such test.
func TestDrawGradientSize(t *testing.T) {
	for _, tc := range []struct{ gradient, row string }{
		{"blue, blue", "BBBB_____"},
		{"to right, blue 50%, lime 50%", "BBGG_____"},
	} {
		t.Run(tc.gradient, func(t *testing.T) {
			assertPixels(t, "gradient_size", tc.row+"\n"+tc.row+"\n_________\n_________", `
      <style>
        @page { size: 9px 4px }
        div {
          height: 4px;
          background: linear-gradient(`+tc.gradient+`) no-repeat;
          background-size: 4px 2px;
        }
      </style>
      <div></div>
    `)
		})
	}
}
