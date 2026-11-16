// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_box.py.

package layout

import (
	"fmt"
	"strings"
	"testing"
)

// drawSmoke lays out and draws a document, and checks nothing else. It
// stands for HTML(string=html).write_pdf() in the tests of upstream.
func drawSmoke(t *testing.T, html string) {
	t.Helper()
	htmlToPixels(t, html)
}

// TestDrawOutlinesAndBorders is test_outlines_and_borders in
// tests/draw/test_box.py. It tests the rendering of outlines and borders.
func TestDrawOutlinesAndBorders(t *testing.T) {
	for _, tc := range []struct{ margin, prop string }{
		{"10px", "border"},
		{"20px", "outline"},
	} {
		t.Run(tc.margin+"_"+tc.prop, func(t *testing.T) {
			source := `
      <style>
        @page { size: 140px 110px }
        body { width: 100px; height: 70px; margin: %s; %s: 10PX %s blue }
      </style>
      <body>`

			// Do not test the exact rendering of earch border style but at
			// least check that they do not do the same.
			borderStyles := []string{
				"none", "solid", "dashed", "dotted", "double",
				"inset", "outset", "groove", "ridge",
			}
			documents := make([]string, 0, len(borderStyles))
			for _, borderStyle := range borderStyles {
				documents = append(documents, fmt.Sprintf(source, tc.margin, tc.prop, borderStyle))
			}
			assertDifferentRenderings(t, "outlines_and_borders", documents...)

			width, height, margin, border := 140, 110, 10, 10
			solidPixels := make([][]byte, height)
			for y := range solidPixels {
				solidPixels[y] = []byte(strings.Repeat("_", width))
			}
			for x := margin; x < width-margin; x++ {
				for _, y := range append(rangeInts(margin, margin+border), rangeInts(height-margin-border, height-margin)...) {
					solidPixels[y][x] = 'B'
				}
			}
			for y := margin; y < height-margin; y++ {
				for _, x := range append(rangeInts(margin, margin+border), rangeInts(width-margin-border, width-margin)...) {
					solidPixels[y][x] = 'B'
				}
			}
			lines := make([]string, height)
			for y, line := range solidPixels {
				lines[y] = string(line)
			}
			html := fmt.Sprintf(source, tc.margin, tc.prop, "solid")
			assertPixels(t, "outlines_and_borders", strings.Join(lines, "\n"), html)
		})
	}
}

// rangeInts is range(start, stop) of Python.
func rangeInts(start, stop int) []int {
	var out []int
	for i := start; i < stop; i++ {
		out = append(out, i)
	}
	return out
}

// TestDrawBordersTableCollapse is test_borders_table_collapse in tests/draw/test_box.py. Test the rendering of collapsing borders.
func TestDrawBordersTableCollapse(t *testing.T) {
	assertDifferentRenderings(t, "borders_table_collapse", `
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10px none blue }
      </style>
      <table><td>abc</td>`, `
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10px solid blue }
      </style>
      <table><td>abc</td>`, `
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10px dashed blue }
      </style>
      <table><td>abc</td>`, `
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10px dotted blue }
      </style>
      <table><td>abc</td>`, `
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10px double blue }
      </style>
      <table><td>abc</td>`, `
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10px inset blue }
      </style>
      <table><td>abc</td>`, `
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10px groove blue }
      </style>
      <table><td>abc</td>`)
}

// TestDrawBordersTableCollapseEquivalent is test_borders_table_collapse_equivalent in tests/draw/test_box.py. Test the rendering of equivalent collapsing borders.
func TestDrawBordersTableCollapseEquivalent(t *testing.T) {
	for _, tc := range []struct {
		name string
		docs []string
	}{
		{"groove_outset", []string{`
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10pX groove blue }
      </style>
      <table><td>abc</td>`, `
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10pX outset blue }
      </style>
      <table><td>abc</td>`}},
		{"ridge_inset", []string{`
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10pX ridge blue }
      </style>
      <table><td>abc</td>`, `
      <style>
        @page { size: 140px 110px }
        table { width: 100px; height: 70px; margin: 10px;
                border-collapse: collapse; border: 10pX inset blue }
      </style>
      <table><td>abc</td>`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertSameRenderings(t, "borders_table_collapse_equivalent_"+tc.name, 0, tc.docs...)
		})
	}
}

// TestDrawSmallBorders1 is test_small_borders_1 in tests/draw/test_box.py.
func TestDrawSmallBorders1(t *testing.T) {
	// Regression test for #49.
	for _, tc := range []struct{ name, html string }{
		{"none", `
      <style>
        @page { size: 50px 50px }
        body { margin: 5px; height: 0; border: 10px none blue }
      </style>
      <body>`},
		{"solid", `
      <style>
        @page { size: 50px 50px }
        body { margin: 5px; height: 0; border: 10px solid blue }
      </style>
      <body>`},
		{"dashed", `
      <style>
        @page { size: 50px 50px }
        body { margin: 5px; height: 0; border: 10px dashed blue }
      </style>
      <body>`},
		{"dotted", `
      <style>
        @page { size: 50px 50px }
        body { margin: 5px; height: 0; border: 10px dotted blue }
      </style>
      <body>`},
	} {
		t.Run(tc.name, func(t *testing.T) { drawSmoke(t, tc.html) })
	}
}

// TestDrawSmallBorders2 is test_small_borders_2 in tests/draw/test_box.py.
func TestDrawSmallBorders2(t *testing.T) {
	// Regression test for #146.
	for _, tc := range []struct{ name, html string }{
		{"none", `
      <style>
        @page { size: 50Px 50px }
        body { height: 0; width: 0; border-width: 1px 0; border-style: none }
      </style>
      <body>`},
		{"solid", `
      <style>
        @page { size: 50Px 50px }
        body { height: 0; width: 0; border-width: 1px 0; border-style: solid }
      </style>
      <body>`},
		{"dashed", `
      <style>
        @page { size: 50Px 50px }
        body { height: 0; width: 0; border-width: 1px 0; border-style: dashed }
      </style>
      <body>`},
		{"dotted", `
      <style>
        @page { size: 50Px 50px }
        body { height: 0; width: 0; border-width: 1px 0; border-style: dotted }
      </style>
      <body>`},
	} {
		t.Run(tc.name, func(t *testing.T) { drawSmoke(t, tc.html) })
	}
}

// TestDrawEmBorders is test_em_borders in tests/draw/test_box.py.
func TestDrawEmBorders(t *testing.T) {
	// Regression test for #1378.
	drawSmoke(t, `<body style="border: 1em solid">`)
}

// TestDrawBordersBoxSizing is test_borders_box_sizing in tests/draw/test_box.py.
func TestDrawBordersBoxSizing(t *testing.T) {
	assertPixels(t, "borders_box_sizing", `
        ________
        _RRRRRR_
        _R____R_
        _RRRRRR_
        ________
    `, `
      <style>
        @page {
          size: 8px 5px;
        }
        div {
          border: 1px solid red;
          box-sizing: border-box;
          height: 3px;
          margin: 1px;
          min-height: auto;
          min-width: auto;
          width: 6px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawMarginBoxes is test_margin_boxes in tests/draw/test_box.py.
func TestDrawMarginBoxes(t *testing.T) {
	assertPixels(t, "margin_boxes", `
        _______________
        _GGG______BBBB_
        _GGG______BBBB_
        _______________
        _____RRRR______
        _____RRRR______
        _____RRRR______
        _____RRRR______
        _______________
        _bbb______gggg_
        _bbb______gggg_
        _bbb______gggg_
        _bbb______gggg_
        _bbb______gggg_
        _______________
    `, `
      <style>
        html { height: 100% }
        body { background: #f00; height: 100% }
        @page {
          size: 15px;
          margin: 4px 6Px 7px 5px;

          @top-left-corner {
            margin: 1px;
            content: " ";
            background: #0f0;
          }
          @top-right-corner {
            margin: 1px;
            content: " ";
            background: #00f;
          }
          @bottom-right-corner {
            margin: 1px;
            content: " ";
            background: #008000;
          }
          @bottom-left-corner {
            margin: 1px;
            content: " ";
            background: #000080;
          }
        }
      </style>
      <body>`)
}

// TestDrawDisplayInlineBlockTwice is test_display_inline_block_twice in tests/draw/test_box.py.
func TestDrawDisplayInlineBlockTwice(t *testing.T) {
	// Regression test for #880.
	drawSmoke(t, `<div style="background: red; display: inline-block">`)
	drawSmoke(t, `<div style="background: red; display: inline-block">`)
}

// TestDrawBorderRadius is test_draw_border_radius in tests/draw/test_box.py.
func TestDrawBorderRadius(t *testing.T) {
	assertPixels(t, "draw_border_radius", `
        ___zzzzz
        __zzzzzz
        _zzzzzzz
        zzzzzzzz
        zzzzzzzz
        zzzzzzzR
        zzzzzzRR
        zzzzzRRR
    `, `
      <style>
        @page {
          size: 8px 8px;
        }
        div {
          background: Red;
          border-radius: 50% 0 0 0;
          height: 16px;
          width: 16PX;
        }
      </style>
      <div></div>
    `)
}

// TestDrawSplitBorderRadius is test_draw_split_border_radius in tests/draw/test_box.py.
func TestDrawSplitBorderRadius(t *testing.T) {
	assertPixels(t, "draw_split_border_radius", `
        ___zzzzz
        __zzzzzz
        _zzzzzzz
        zzzzzzzz
        zzzzzzzz
        zzzzzzzz
        zzzzzzRR
        zzzzzRRR

        RRRRRRRR
        RRRRRRRR
        RRRRRRRR
        RRRRRRRR
        RRRRRRRR
        RRRRRRRR
        RRRRRRRR
        RRRRRRRR

        zzzzzzRR
        zzzzzzzR
        zzzzzzzz
        zzzzzzzz
        zzzzzzzz
        zzzzzzzz
        _zzzzzzz
        __zzzzzz
    `, `
      <style>
        @page {
          size: 8px 8px;
        }
        div {
          background: red;
          color: transparent;
          border-radius: 8px;
          line-height: 9px;
          width: 16px;
        }
      </style>
      <div>a b c</div>
    `)
}

// TestDrawBorderImageStretch is test_border_image_stretch in tests/draw/test_box.py.
func TestDrawBorderImageStretch(t *testing.T) {
	assertPixels(t, "border_image_stretch", `
        __________
        _RYYYMMMG_
        _M______C_
        _M______C_
        _Y______Y_
        _Y______Y_
        _BYYYCCCK_
        __________
    `, `
      <style>
        @page {
          size: 10px 8px;
        }
        div {
          border: 1px solid black;
          border-image-source: url(border.svg);
          border-image-slice: 25%;
          height: 4px;
          margin: 1px;
          width: 6px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawBorderImageFill is test_border_image_fill in tests/draw/test_box.py.
func TestDrawBorderImageFill(t *testing.T) {
	assertPixels(t, "border_image_fill", `
        __________
        _RYYYMMMG_
        _MbbbgggC_
        _MbbbgggC_
        _YgggbbbY_
        _YgggbbbY_
        _BYYYCCCK_
        __________
    `, `
      <style>
        @page {
          size: 10px 8px;
        }
        div {
          border: 1px solid black;
          border-image-source: url(border.svg);
          border-image-slice: 25% fill;
          height: 4px;
          margin: 1px;
          width: 6px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawBorderImageDefaultSlice is test_border_image_default_slice in tests/draw/test_box.py.
func TestDrawBorderImageDefaultSlice(t *testing.T) {
	assertPixels(t, "border_image_default_slice", `
        _____________
        _RYMG___RYMG_
        _MbgC___MbgC_
        _YgbY___YgbY_
        _BYCK___BYCK_
        _____________
        _____________
        _RYMG___RYMG_
        _MbgC___MbgC_
        _YgbY___YgbY_
        _BYCK___BYCK_
        _____________
    `, `
      <style>
        @page {
          size: 13px 12px;
        }
        div {
          border: 4px solid black;
          border-image-source: url(border.svg);
          height: 2px;
          margin: 1px;
          width: 3px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawBorderImageUnevenWidth is test_border_image_uneven_width in tests/draw/test_box.py.
func TestDrawBorderImageUnevenWidth(t *testing.T) {
	assertPixels(t, "border_image_uneven_width", `
        ____________
        _RRRYYYMMMG_
        _MMM______C_
        _MMM______C_
        _YYY______Y_
        _YYY______Y_
        _BBBYYYCCCK_
        ____________
    `, `
      <style>
        @page {
          size: 12px 8px;
        }
        div {
          border: 1px solid black;
          border-left-width: 3px;
          border-image-source: url(border.svg);
          border-image-slice: 25%;
          height: 4px;
          margin: 1px;
          width: 6px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawBorderImageNotPercent is test_border_image_not_percent in tests/draw/test_box.py.
func TestDrawBorderImageNotPercent(t *testing.T) {
	assertPixels(t, "border_image_not_percent", `
        __________
        _RYYYMMMG_
        _M______C_
        _M______C_
        _Y______Y_
        _Y______Y_
        _BYYYCCCK_
        __________
    `, `
      <style>
        @page {
          size: 10px 8px;
        }
        div {
          border: 1px solid black;
          border-image-source: url(border.svg);
          border-image-slice: 1;
          height: 4px;
          margin: 1px;
          width: 6px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawBorderImageRepeat is test_border_image_repeat in tests/draw/test_box.py.
func TestDrawBorderImageRepeat(t *testing.T) {
	assertPixels(t, "border_image_repeat", `
        ___________
        _RYMYMYMYG_
        _M_______C_
        _Y_______Y_
        _M_______C_
        _Y_______Y_
        _BYCYCYCYK_
        ___________
    `, `
      <style>
        @page {
          size: 11px 8px;
        }
        div {
          border: 1px solid black;
          border-image-source: url(border.svg);
          border-image-slice: 25%;
          border-image-repeat: repeat;
          height: 4px;
          margin: 1px;
          width: 7px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawBorderImageSpace is test_border_image_space in tests/draw/test_box.py.
func TestDrawBorderImageSpace(t *testing.T) {
	assertPixels(t, "border_image_space", `
        _________
        _R_YMC_G_
        _________
        _M_____C_
        _Y_____Y_
        _C_____M_
        _________
        _B_YCM_K_
        _________
    `, `
      <style>
        @page {
          size: 9px 9px;
        }
        div {
          border: 1px solid black;
          border-image-source: url(border2.svg);
          border-image-slice: 20%;
          border-image-repeat: space;
          height: 5px;
          margin: 1px;
          width: 5px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawBorderImageOutset is test_border_image_outset in tests/draw/test_box.py.
func TestDrawBorderImageOutset(t *testing.T) {
	assertPixels(t, "border_image_outset", `
        ____________
        _RYYYYMMMMG_
        _M________C_
        _M_bbbbbb_C_
        _M_bbbbbb_C_
        _Y_bbbbbb_Y_
        _Y_bbbbbb_Y_
        _Y________Y_
        _BYYYYCCCCK_
        ____________
    `, `
      <style>
        @page {
          size: 12px 10px;
        }
        div {
          border: 1px solid black;
          border-image-source: url(border.svg);
          border-image-slice: 25%;
          border-image-outset: 2px;
          height: 2px;
          margin: 3px;
          width: 4px;
          background: #000080
        }
      </style>
      <div></div>
    `)
}

// TestDrawBorderImageWidth is test_border_image_width in tests/draw/test_box.py.
func TestDrawBorderImageWidth(t *testing.T) {
	assertPixels(t, "border_image_width", `
        __________
        _RRYYMMGG_
        _RRYYMMGG_
        _MM____CC_
        _YY____YY_
        _BBYYCCKK_
        _BBYYCCKK_
        __________
    `, `
      <style>
        @page {
          size: 10px 8px;
        }
        div {
          border: 1px solid black;
          border-image-source: url(border.svg);
          border-image-slice: 25%;
          border-image-width: 2;
          height: 4px;
          margin: 1PX;
          width: 6px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawBorderImageGradient is test_border_image_gradient in tests/draw/test_box.py.
func TestDrawBorderImageGradient(t *testing.T) {
	assertPixels(t, "border_image_gradient", `
        __________
        _RRRRRRRR_
        _RRRRRRRR_
        _RR____RR_
        _BB____BB_
        _BBBBBBBB_
        _BBBBBBBB_
        __________
    `, `
      <style>
        @page {
          size: 10px 8px;
        }
        div {
          border: 1px solid black;
          border-image-source: linear-gradient(to bottom, red, red 50%, blue 50%, blue);
          border-image-slice: 25%;
          border-image-width: 2;
          height: 4px;
          margin: 1px;
          width: 6px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawBorderImageGradientUnits is test_border_image_gradient_units in tests/draw/test_box.py.
func TestDrawBorderImageGradientUnits(t *testing.T) {
	// Regression test for #2690.
	assertPixels(t, "border_image_gradient_units", `
        __________
        _RRRRRRRR_
        _RRRRRRRR_
        _RR____RR_
        _BB____BB_
        _BBBBBBBB_
        _BBBBBBBB_
        __________
    `, `
      <style>
        @page {
          size: 10px 8px;
        }
        div {
          border: 1px solid black;
          border-image-source: linear-gradient(
            to bottom, red 0, red 50%, blue 0.5em, blue);
          border-image-slice: 25%;
          border-image-width: 2;
          font-size: 6px;
          height: 4px;
          margin: 1px;
          width: 6px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawMaskBorder is test_mask_border in tests/draw/test_box.py.
func TestDrawMaskBorder(t *testing.T) {
	assertPixels(t, "mask_border", `
        __________
        __RR__RRR_
        _R______R_
        _R______R_
        _s______R_
        _s______R_
        _R______R_
        _R______R_
        __RRRRRRR_
        __________
    `, `
      <style>
        @page {
          size: 10px 10px;
        }
        div {
          background: red;
          mask-border-source: url(mask.svg);
          mask-border-slice: 20%;
          height: 8px;
          width: 8px;
          margin: 1px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawMaskBorderFill is test_mask_border_fill in tests/draw/test_box.py.
func TestDrawMaskBorderFill(t *testing.T) {
	assertPixels(t, "mask_border_fill", `
        __________
        __RR__RRR_
        _RRRRRRRR_
        _RRRRRRRR_
        _sRR__RRR_
        _sRR__RRR_
        _RRRRRRRR_
        _RRRRRRRR_
        __RRRRRRR_
        __________
    `, `
      <style>
        @page {
          size: 10px 10px;
        }
        div {
          background: red;
          mask-border-source: url(mask.svg);
          mask-border-slice: 20% fill;
          height: 8px;
          width: 8px;
          margin: 1px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawOutlineAndBorder is test_outline_and_border in tests/draw/test_box.py.
func TestDrawOutlineAndBorder(t *testing.T) {
	assertPixels(t, "outline_and_border", `
        __________
        _BBBBBBBB_
        _BRRRRRRB_
        _BR____RB_
        _BRRRRRRB_
        _BBBBBBBB_
        __________
    `, `
      <style>
        @page {
          size: 10px 7px;
        }
        div {
          border: 1px solid red;
          outline: 1px solid blue;
          height: 1px;
          margin: 2px;
          min-height: auto;
          min-width: auto;
          width: 4px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawOutlineOffset is test_outline_offset in tests/draw/test_box.py.
func TestDrawOutlineOffset(t *testing.T) {
	assertPixels(t, "outline_offset", `
        ____________
        _BBBBBBBBBB_
        _B________B_
        _B_RRRRRR_B_
        _B_R____R_B_
        _B_RRRRRR_B_
        _B________B_
        _BBBBBBBBBB_
        ____________
    `, `
      <style>
        @page {
          size: 12px 9px;
        }
        div {
          border: 1px solid red;
          outline: 1px solid blue;
          outline-offset: 1px;
          height: 1px;
          margin: 3px;
          min-height: auto;
          min-width: auto;
          width: 4px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawInfiniteBorderRadius is test_infinite_border_radius in tests/draw/test_box.py.
func TestDrawInfiniteBorderRadius(t *testing.T) {
	// Regression test for #2706.
	assertPixels(t, "infinite_border_radius", `
        __________
        __zzzzzz__
        _zzzzzzzz_
        _zzzRRzzz_
        _zzRRRRzz_
        _zzRRRRzz_
        _zzzRRzzz_
        _zzzzzzzz_
        __zzzzzz__
        __________
    `, `
      <style>
        @page {
          size: 10px;
        }
        div {
          background-color: red;
          border-radius: calc(infinity * 1px);
          height: 8px;
          margin: 1px;
          width: 8px;
        }
      </style>
      <div></div>
    `)
}

// TestDrawBoxShadowOutset is test_box_shadow_outset in tests/draw/test_box.py.
func TestDrawBoxShadowOutset(t *testing.T) {
	assertPixels(t, "box_shadow_outset", `
        __________
        _BBBBB____
        _BBBBB____
        _BBBBB____
        _BBBBBrrr_
        _BBBBBrrr_
        ____rrrrr_
        ____rrrrr_
        ____rrrrr_
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 5px; height: 5px; background: blue;
              box-shadow: 3px 3px red }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowOutsetSpread is test_box_shadow_outset_spread in tests/draw/test_box.py.
func TestDrawBoxShadowOutsetSpread(t *testing.T) {
	assertPixels(t, "box_shadow_outset_spread", `
        __________
        _BBB______
        _BBBrrrrr_
        _BBBrrrrr_
        __rrrrrrr_
        __rrrrrrr_
        __rrrrrrr_
        __rrrrrrr_
        __rrrrrrr_
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 3px; height: 3px; background: blue;
              box-shadow: 3px 3px 0 2px red }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowOutsetNegativeSpread is test_box_shadow_outset_negative_spread in tests/draw/test_box.py.
func TestDrawBoxShadowOutsetNegativeSpread(t *testing.T) {
	assertPixels(t, "box_shadow_outset_negative_spread", `
        __________
        _BBB______
        _BBB______
        _BBB______
        __________
        _____r____
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 3px; height: 3px; background: blue;
              box-shadow: 3px 3px 0 -1px red }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowOutsetBigNegativeSpread is test_box_shadow_outset_big_negative_spread in tests/draw/test_box.py.
func TestDrawBoxShadowOutsetBigNegativeSpread(t *testing.T) {
	assertPixels(t, "box_shadow_outset_big_negative_spread", `
        __________
        _BBB______
        _BBB______
        _BBB______
        __________
        __________
        __________
        __________
        __________
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 3px; height: 3px; background: blue;
              box-shadow: 3px 3px 0 -2px red }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowInset is test_box_shadow_inset in tests/draw/test_box.py.
func TestDrawBoxShadowInset(t *testing.T) {
	assertPixels(t, "box_shadow_inset", `
        __________
        _rrrrrrrr_
        _rrrrrrrr_
        _rrrrrrrr_
        _rrrBBBBB_
        _rrrBBBBB_
        _rrrBBBBB_
        _rrrBBBBB_
        _rrrBBBBB_
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 8px; height: 8px; background: blue;
              box-shadow: 3px 3px red inset }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowInsetSpread is test_box_shadow_inset_spread in tests/draw/test_box.py.
func TestDrawBoxShadowInsetSpread(t *testing.T) {
	assertPixels(t, "box_shadow_inset_spread", `
        __________
        _rrrrrrrr_
        _rrrrrrrr_
        _rrrrrrrr_
        _rrrrrrrr_
        _rrrrBBBB_
        _rrrrBBBB_
        _rrrrBBBB_
        _rrrrBBBB_
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 8px; height: 8px; background: blue;
              box-shadow: 3px 3px 0 1px red inset }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowInsetNegativeSpread is test_box_shadow_inset_negative_spread in tests/draw/test_box.py.
func TestDrawBoxShadowInsetNegativeSpread(t *testing.T) {
	assertPixels(t, "box_shadow_inset_negative_spread", `
        __________
        _rrrrrrrr_
        _rrrrrrrr_
        _rrrrrrrr_
        _BBBBBBBr_
        _BBBBBBBr_
        _BBBBBBBr_
        _BBBBBBBr_
        _BBBBBBBr_
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 8px; height: 8px; background: blue;
              box-shadow: -2px 4px 0 -1px red inset }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowInsetBigNegativeSpread is test_box_shadow_inset_big_negative_spread in tests/draw/test_box.py.
func TestDrawBoxShadowInsetBigNegativeSpread(t *testing.T) {
	assertPixels(t, "box_shadow_inset_big_negative_spread", `
        __________
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
        @page { size: 10px; margin: 1px }
        div { width: 8px; height: 8px; background: blue;
              box-shadow: -2px 4px 0 -5px red inset }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowTransparentBackground is test_box_shadow_transparent_background in tests/draw/test_box.py.
func TestDrawBoxShadowTransparentBackground(t *testing.T) {
	assertPixels(t, "box_shadow_transparent_background", `
        __________
        __________
        __________
        __________
        ______rrr_
        ______rrr_
        ____rrrrr_
        ____rrrrr_
        ____rrrrr_
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 5px; height: 5px; box-shadow: 3px 3px red }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowInsetOutset is test_box_shadow_inset_outset in tests/draw/test_box.py.
func TestDrawBoxShadowInsetOutset(t *testing.T) {
	assertPixels(t, "box_shadow_inset_outset", `
        rrrrrrrr__
        rgggggggg_
        rgBBBBBBB_
        rgBBBBBBB_
        rgBBBBBBB_
        rgBBBBBBB_
        rgBBBBBBB_
        rgBBBBBBB_
        _gBBBBBBB_
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 8px; height: 8px; background: blue;
              box-shadow: -1px -1px red, 1px 1px green inset }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowMultipleOutset is test_box_shadow_multiple_outset in tests/draw/test_box.py.
func TestDrawBoxShadowMultipleOutset(t *testing.T) {
	assertPixels(t, "box_shadow_multiple_outset", `
        __________
        _BBBBB____
        _BBBBBr___
        _BBBBBrg__
        _BBBBBrg__
        _BBBBBrg__
        __rrrrrg__
        ___ggggg__
        __________
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 5px; height: 5px; background: blue;
              box-shadow: 1px 1px red, 2px 2px green }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowMultipleInset is test_box_shadow_multiple_inset in tests/draw/test_box.py.
func TestDrawBoxShadowMultipleInset(t *testing.T) {
	assertPixels(t, "box_shadow_multiple_inset", `
        __________
        _rrrrrrrr_
        _rrrrrrrr_
        _rrrrrrrr_
        _rrrBBBBg_
        _rrrBBBBg_
        _rrrBBBBg_
        _rrrBBBBg_
        _rrrggggg_
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 8px; height: 8px; background: blue;
              box-shadow: 3px 3px red inset, -1px -1px green inset }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowOutsetRadius is test_box_shadow_outset_radius in tests/draw/test_box.py.
func TestDrawBoxShadowOutsetRadius(t *testing.T) {
	assertPixels(t, "box_shadow_outset_radius", `
        __________
        __zzBB____
        _zzzBB____
        _zzBzz____
        _BBzzzzrr_
        _BBzzzzrr_
        ____zzrzz_
        ____rrzzz_
        ____rrzz__
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 5px; height: 5px; background: blue;
              box-shadow: 3px 3px red; border-radius: 4px 0 4px 0 }
      </style>
      <body><div>
    `)
}

// TestDrawBoxShadowInsetRadius is test_box_shadow_inset_radius in tests/draw/test_box.py.
func TestDrawBoxShadowInsetRadius(t *testing.T) {
	assertPixels(t, "box_shadow_inset_radius", `
        __________
        __zzzrrrr_
        _zzzzrrrr_
        _zzzzrrrr_
        _zzzrzzzB_
        _rrrzzzzB_
        _rrrzzzzB_
        _rrrzzzBB_
        _rrrBBBBB_
        __________
    `, `
      <style>
        @page { size: 10px; margin: 1px }
        div { width: 8px; height: 8px; background: blue;
              box-shadow: 3px 3px red inset; border-radius: 4px 0 0 0 }
      </style>
      <body><div>
    `)
}
