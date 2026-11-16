// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_image.py: how the images are drawn.

package layout

import (
	"log/slog"
	"testing"
)

// TestDrawImages is test_images in tests/draw/test_image.py.
func TestDrawImages(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"pattern.svg", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        img { overflow: hidden }
      </style>
      <div><img src="pattern.svg"></div>`, false},
		{"pattern.png", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        img { overflow: hidden }
      </style>
      <div><img src="pattern.png"></div>`, false},
		{"pattern.palette.png", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        img { overflow: hidden }
      </style>
      <div><img src="pattern.palette.png"></div>`, false},
		{"pattern.gif", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        img { overflow: hidden }
      </style>
      <div><img src="pattern.gif"></div>`, false},
		{"blue.jpg", `
    ________
    ________
    __aaaa__
    __aaaa__
    __aaaa__
    __aaaa__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        img { overflow: hidden }
      </style>
      <div><img src="blue.jpg"></div>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "images", tc.pixels, tc.html)
		})
	}
}

// TestDrawResizedImages is test_resized_images in tests/draw/test_image.py.
func TestDrawResizedImages(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"pattern.svg", `
    ____________
    ____________
    __rrBBBBBB__
    __rrBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        img { display: block; width: 8px; image-rendering: pixelated;
              overflow: hidden }
      </style>
      <div><img src="pattern.svg"></div>`, false},
		{"pattern.png", `
    ____________
    ____________
    __rrBBBBBB__
    __rrBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        img { display: block; width: 8px; image-rendering: pixelated;
              overflow: hidden }
      </style>
      <div><img src="pattern.png"></div>`, false},
		{"pattern.palette.png", `
    ____________
    ____________
    __rrBBBBBB__
    __rrBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        img { display: block; width: 8px; image-rendering: pixelated;
              overflow: hidden }
      </style>
      <div><img src="pattern.palette.png"></div>`, false},
		{"pattern.gif", `
    ____________
    ____________
    __rrBBBBBB__
    __rrBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        img { display: block; width: 8px; image-rendering: pixelated;
              overflow: hidden }
      </style>
      <div><img src="pattern.gif"></div>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "resized_images", tc.pixels, tc.html)
		})
	}
}

// TestDrawImageOverflow is test_image_overflow in tests/draw/test_image.py.
func TestDrawImageOverflow(t *testing.T) {
	assertPixels(t, "image_overflow", `
    ________
    ________
    __rBBBBB
    __BBBBBB
    __BBBBBB
    __BBBBBB
    __BBBBBB
    __BBBBBB
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
      </style>
      <div><img src="pattern.svg"></div>`)
}

// TestDrawSvgSizing is test_svg_sizing in tests/draw/test_image.py.
func TestDrawSvgSizing(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"None", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block }
      </style>
      <svg   >
        <rect width="4px" height="4px" fill="#00f" />
        <rect width="1px" height="1px" fill="#f00" />
      </svg>`, false},
		{"None", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block }
      </style>
      <svg width="4"  >
        <rect width="4px" height="4px" fill="#00f" />
        <rect width="1px" height="1px" fill="#f00" />
      </svg>`, false},
		{"None", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block }
      </style>
      <svg  height="4px" >
        <rect width="4px" height="4px" fill="#00f" />
        <rect width="1px" height="1px" fill="#f00" />
      </svg>`, false},
		{"None", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block }
      </style>
      <svg width="4" height="4px" >
        <rect width="4px" height="4px" fill="#00f" />
        <rect width="1px" height="1px" fill="#f00" />
      </svg>`, false},
		{"0 0 4 4", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block }
      </style>
      <svg width="4"  viewbox="0 0 4 4">
        <rect width="4px" height="4px" fill="#00f" />
        <rect width="1px" height="1px" fill="#f00" />
      </svg>`, false},
		{"0 0 4 4", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block }
      </style>
      <svg  height="4px" viewbox="0 0 4 4">
        <rect width="4px" height="4px" fill="#00f" />
        <rect width="1px" height="1px" fill="#f00" />
      </svg>`, false},
		{"0 0 4 4", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block }
      </style>
      <svg width="4" height="4px" viewbox="0 0 4 4">
        <rect width="4px" height="4px" fill="#00f" />
        <rect width="1px" height="1px" fill="#f00" />
      </svg>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "svg_sizing", tc.pixels, tc.html)
		})
	}
}

// TestDrawSvgSizingPercentage is test_svg_sizing_percentage in tests/draw/test_image.py.
func TestDrawSvgSizingPercentage(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"style=\"width: 100%; height: 100%", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 0; font-size: 0 }
        svg { display: block }
      </style>
      <div style="height: 4px">
        <svg viewBox="0 0 8 8" style="width: 100%; height: 100%">
          <rect width="8" height="8" fill="#00f" />
          <rect width="2" height="2" fill="#f00" />
        </svg>
      </div>`, false},
		{"width=\"100%\" height=\"100%", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 0; font-size: 0 }
        svg { display: block }
      </style>
      <div style="height: 4px">
        <svg viewBox="0 0 8 8" width="100%" height="100%">
          <rect width="8" height="8" fill="#00f" />
          <rect width="2" height="2" fill="#f00" />
        </svg>
      </div>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "svg_sizing_percentage", tc.pixels, tc.html)
		})
	}
}

// TestDrawSvgResizing is test_svg_resizing in tests/draw/test_image.py.
func TestDrawSvgResizing(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"None", `
    ____________
    ____________
    __rBBB______
    __BBBB______
    __BBBB______
    __BBBB______
    ____________
    ____________
    ____________
    ____________
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block; width: 8px }
      </style>
      <svg   >
        <rect width="4" height="4" fill="#00f" />
        <rect width="1" height="1" fill="#f00" />
      </svg>`, false},
		{"None", `
    ____________
    ____________
    __rBBB______
    __BBBB______
    __BBBB______
    __BBBB______
    ____________
    ____________
    ____________
    ____________
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block; width: 8px }
      </style>
      <svg width="8"  >
        <rect width="4" height="4" fill="#00f" />
        <rect width="1" height="1" fill="#f00" />
      </svg>`, false},
		{"None", `
    ____________
    ____________
    __rBBB______
    __BBBB______
    __BBBB______
    __BBBB______
    ____________
    ____________
    ____________
    ____________
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block; width: 8px }
      </style>
      <svg  height="8px" >
        <rect width="4" height="4" fill="#00f" />
        <rect width="1" height="1" fill="#f00" />
      </svg>`, false},
		{"None", `
    ____________
    ____________
    __rBBB______
    __BBBB______
    __BBBB______
    __BBBB______
    ____________
    ____________
    ____________
    ____________
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block; width: 8px }
      </style>
      <svg width="8" height="8px" >
        <rect width="4" height="4" fill="#00f" />
        <rect width="1" height="1" fill="#f00" />
      </svg>`, false},
		{"0 0 4 4", `
    ____________
    ____________
    __rrBBBBBB__
    __rrBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block; width: 8px }
      </style>
      <svg   viewbox="0 0 4 4">
        <rect width="4" height="4" fill="#00f" />
        <rect width="1" height="1" fill="#f00" />
      </svg>`, false},
		{"0 0 4 4", `
    ____________
    ____________
    __rrBBBBBB__
    __rrBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block; width: 8px }
      </style>
      <svg width="8"  viewbox="0 0 4 4">
        <rect width="4" height="4" fill="#00f" />
        <rect width="1" height="1" fill="#f00" />
      </svg>`, false},
		{"0 0 4 4", `
    ____________
    ____________
    __rrBBBBBB__
    __rrBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block; width: 8px }
      </style>
      <svg  height="8px" viewbox="0 0 4 4">
        <rect width="4" height="4" fill="#00f" />
        <rect width="1" height="1" fill="#f00" />
      </svg>`, false},
		{"0 0 4 4", `
    ____________
    ____________
    __rrBBBBBB__
    __rrBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    __BBBBBBBB__
    ____________
    ____________
`, `
      <style>
        @page { size: 12px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        svg { display: block; width: 8px }
      </style>
      <svg width="8" height="8px" viewbox="0 0 4 4">
        <rect width="4" height="4" fill="#00f" />
        <rect width="1" height="1" fill="#f00" />
      </svg>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "svg_resizing", tc.pixels, tc.html)
		})
	}
}

// TestDrawImagesBlock is test_images_block in tests/draw/test_image.py.
func TestDrawImagesBlock(t *testing.T) {
	assertPixels(t, "images_block", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 0; font-size: 0 }
        img { display: block; margin: 2px auto 0 }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDrawImagesNoSrc is test_images_no_src in tests/draw/test_image.py.
func TestDrawImagesNoSrc(t *testing.T) {
	assertPixels(t, "images_no_src", `
    ________
    ________
    ________
    ________
    ________
    ________
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 0; font-size: 0 }
        img { display: block; margin: 2px auto 0 }
      </style>
      <div><img alt=""></div>`)
}

// TestDrawImagesRepeatTransparent is test_images_repeat_transparent in tests/draw/test_image.py.
func TestDrawImagesRepeatTransparent(t *testing.T) {
	assertPixels(t, "images_repeat_transparent", `_
_
_`, `
      <style>
        @page { size: 1px }
        div { height: 100px; width: 100px; background: url(logo_small.png) }
      </style>
      <div></div><div></div><div></div>`)
}

// TestDrawImagesNoWidth is test_images_no_width in tests/draw/test_image.py.
func TestDrawImagesNoWidth(t *testing.T) {
	assertPixels(t, "images_no_width", `
    ________
    ________
    ________
    ________
    ________
    ________
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px; font-size: 0 }
      </style>
      <div><img src="pattern.png" alt="not shown"
                style="width: 0; height: 1px"></div>`)
}

// TestDrawImagesNoHeight is test_images_no_height in tests/draw/test_image.py.
func TestDrawImagesNoHeight(t *testing.T) {
	assertPixels(t, "images_no_height", `
    ________
    ________
    ________
    ________
    ________
    ________
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px; font-size: 0 }
      </style>
      <div><img src="pattern.png" alt="not shown"
                style="width: 1px; height: 0"></div>`)
}

// TestDrawImagesNoWidthHeight is test_images_no_width_height in tests/draw/test_image.py.
func TestDrawImagesNoWidthHeight(t *testing.T) {
	assertPixels(t, "images_no_width_height", `
    ________
    ________
    ________
    ________
    ________
    ________
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px; font-size: 0 }
      </style>
      <div><img src="pattern.png" alt="not shown"
                style="width: 0; height: 0"></div>`)
}

// TestDrawImagesPageBreak is test_images_page_break in tests/draw/test_image.py.
func TestDrawImagesPageBreak(t *testing.T) {
	assertPixels(t, "images_page_break", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________

    ________
    ________
    ________
    ________
    ________
    ________
    ________
    ________

    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px; margin: 2px }
        body { font-size: 0 }
      </style>
      <div><img src="pattern.png"></div>
      <div style="page-break-before: right"><img src="pattern.png"></div>`)
}

// TestDrawImageRepeatInline is test_image_repeat_inline in tests/draw/test_image.py.
func TestDrawImageRepeatInline(t *testing.T) {
	assertPixels(t, "image_repeat_inline", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________

    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
    ________
    ________
`, `
      <style>
        @page { size: 8px; margin: 0 }
        table { border-collapse: collapse; margin: 2px }
        th, td { border: none; padding: 0 }
        th { height: 4px; line-height: 4px }
        td { height: 2px }
        img { vertical-align: top }
      </style>
      <table>
        <thead>
          <tr><th><img src="pattern.png"></th></tr>
        </thead>
        <tbody>
          <tr><td></td></tr>
          <tr><td></td></tr>
        </tbody>
      </table>`)
}

// TestDrawImageRepeatBlock is test_image_repeat_block in tests/draw/test_image.py.
func TestDrawImageRepeatBlock(t *testing.T) {
	assertPixels(t, "image_repeat_block", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________

    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
    ________
    ________
`, `
      <style>
        @page { size: 8px; margin: 0 }
        table { border-collapse: collapse; margin: 2px }
        th, td { border: none; padding: 0 }
        th { height: 4px }
        td { height: 2px }
        img { display: block }
      </style>
      <table>
        <thead>
          <tr><th><img src="pattern.png"></th></tr>
        </thead>
        <tbody>
          <tr><td></td></tr>
          <tr><td></td></tr>
        </tbody>
      </table>`)
}

// TestDrawImagesPadding is test_images_padding in tests/draw/test_image.py.
func TestDrawImagesPadding(t *testing.T) {
	assertPixels(t, "images_padding", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { font-size: 0 }
      </style>
      <div style="line-height: 1px">
        <img src=pattern.png style="padding: 2px 0 0 2px">
      </div>`)
}

// TestDrawImagesInInlineBlock is test_images_in_inline_block in tests/draw/test_image.py.
func TestDrawImagesInInlineBlock(t *testing.T) {
	assertPixels(t, "images_in_inline_block", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
      </style>
      <div style="display: inline-block">
        <p><img src=pattern.png></p>
      </div>`)
}

// TestDrawImagesTransparentText is test_images_transparent_text in tests/draw/test_image.py.
func TestDrawImagesTransparentText(t *testing.T) {
	assertPixels(t, "images_transparent_text", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `<style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 2px; line-height: 0 }
      </style>
      <div style="color: #0001">123</div>
      <img src=pattern.png>
    `)
}

// TestDrawImagesSharedPattern is test_images_shared_pattern in tests/draw/test_image.py.
func TestDrawImagesSharedPattern(t *testing.T) {
	assertPixels(t, "images_shared_pattern", `
        ____________
        ____________
        __aaaaaaaa__
        __aaaaaaaa__
        ____________
        __aaaa______
        __aaaa______
        __aaaa______
        __aaaa______
        ____________
        ____________
        ____________
    `, `
      <style>
        @page { size: 12px }
        body { margin: 2px; font-size: 0 }
      </style>
      <div style="background: url(blue.jpg);
                  height: 2px; margin-bottom: 1px"></div>
      <img src=blue.jpg>
    `)
}

// TestDrawImageResolution is test_image_resolution in tests/draw/test_image.py.
func TestDrawImageResolution(t *testing.T) {
	assertSameRenderings(t, "image_resolution", 0,
		`
            <style>@page { size: 20px; margin: 2px }</style>
            <div style="font-size: 0">
                <img src="pattern.png" style="width: 8px"></div>
        `,
		`
            <style>@page { size: 20px; margin: 2px }</style>
            <div style="image-resolution: .5dppx; font-size: 0">
                <img src="pattern.png"></div>
        `,
		`
            <style>@page { size: 20px; margin: 2px }
                   div::before { content: url(pattern.png) }
            </style>
            <div style="image-resolution: .5x; font-size: 0"></div>
        `,
		`
            <style>@page { size: 20px; margin: 2px }
            </style>
            <div style="height: 16px; image-resolution: .5dppx;
                        background: url(pattern.png) no-repeat"></div>
        `)
}

// TestDrawImageCover is test_image_cover in tests/draw/test_image.py.
func TestDrawImageCover(t *testing.T) {
	assertPixels(t, "image_cover", `
    ________
    ________
    __BB____
    __BB____
    __BB____
    __BB____
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 2px 0 0 2px; font-size: 0 }
        img { object-fit: cover; height: 4px; width: 2px; overflow: hidden }
      </style>
      <img src="pattern.png">`)
}

// TestDrawImageContain is test_image_contain in tests/draw/test_image.py.
func TestDrawImageContain(t *testing.T) {
	assertPixels(t, "image_contain", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 1px 0 0 2px; font-size: 0 }
        img { object-fit: contain; height: 6px; width: 4px; overflow: hidden }
      </style>
      <img src="pattern.png">`)
}

// TestDrawImageNone is test_image_none in tests/draw/test_image.py.
func TestDrawImageNone(t *testing.T) {
	assertPixels(t, "image_none", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 1px 0 0 1px; font-size: 0 }
        img { object-fit: none; height: 6px; width: 6px }
      </style>
      <img src="pattern.png">`)
}

// TestDrawImageScaleDown is test_image_scale_down in tests/draw/test_image.py.
func TestDrawImageScaleDown(t *testing.T) {
	assertPixels(t, "image_scale_down", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 1px 0 0 1px; font-size: 0 }
        img { object-fit: scale-down; height: 6px; width: 6px }
      </style>
      <img src="pattern.png">`)
}

// TestDrawImagePosition is test_image_position in tests/draw/test_image.py.
func TestDrawImagePosition(t *testing.T) {
	assertPixels(t, "image_position", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
        @page { size: 8px }
        body { margin: 1px 0 0 1px; font-size: 0 }
        img { object-fit: none; height: 6px; width: 6px;
              object-position: bottom 50% right 50% }
      </style>
      <img src="pattern.png">`)
}

// TestDrawImagesBorder is test_images_border in tests/draw/test_image.py.
func TestDrawImagesBorder(t *testing.T) {
	assertPixels(t, "images_border", `
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
        body { margin: 0; font-size: 0 }
        img { margin: 1px; border: 1px solid lime }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDrawImagesBorderAbsolute is test_images_border_absolute in tests/draw/test_image.py.
func TestDrawImagesBorderAbsolute(t *testing.T) {
	assertPixels(t, "images_border_absolute", `
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
        body { margin: 0; font-size: 0 }
        img { margin: 1px; border: 1px solid lime; position: absolute }
      </style>
      <div><img src="pattern.png"></div>`)
}

// TestDrawImageExif is test_image_exif in tests/draw/test_image.py.
func TestDrawImageExif(t *testing.T) {
	assertSameRenderings(t, "image_exif", 25,
		`
            <style>@page { size: 10px }</style>
            <img style="display: block" src="not-optimized.jpg">
        `,
		`
            <style>@page { size: 10px }</style>
            <img style="display: block" src="not-optimized-exif.jpg">
        `)
}

// TestDrawImageExifImageOrientation is test_image_exif_image_orientation in tests/draw/test_image.py.
func TestDrawImageExifImageOrientation(t *testing.T) {
	assertSameRenderings(t, "image_exif_image_orientation", 25,
		`
            <style>@page { size: 10px }</style>
            <img style="display: block; image-orientation: 180deg"
                 src="not-optimized-exif.jpg">
        `,
		`
            <style>@page { size: 10px }</style>
            <img style="display: block" src="not-optimized-exif.jpg">
        `)
}

// TestDrawImageAfterAlpha is test_image_after_alpha in tests/draw/test_image.py.
func TestDrawImageAfterAlpha(t *testing.T) {
	for _, tc := range []struct {
		name, pixels, html string
		xfail              bool
	}{
		{"pattern.svg", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
         @page { size: 8px; margin: 2px }
         div { width: 4px; height: 4px; box-sizing: border-box }
         div + div { margin-top: -4px }
      </style>
      <div style="border: 1px solid #1111"></div>
      <div style="background: url(pattern.svg)"></div>
    `, false},
		{"pattern.svg", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
         @page { size: 8px; margin: 2px }
         div { width: 4px; height: 4px; box-sizing: border-box }
         div + div { margin-top: -4px }
      </style>
      <div style="border: 1px dotted #1111"></div>
      <div style="background: url(pattern.svg)"></div>
    `, false},
		{"pattern.png", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
         @page { size: 8px; margin: 2px }
         div { width: 4px; height: 4px; box-sizing: border-box }
         div + div { margin-top: -4px }
      </style>
      <div style="border: 1px solid #1111"></div>
      <div style="background: url(pattern.png)"></div>
    `, false},
		{"pattern.png", `
    ________
    ________
    __rBBB__
    __BBBB__
    __BBBB__
    __BBBB__
    ________
    ________
`, `
      <style>
         @page { size: 8px; margin: 2px }
         div { width: 4px; height: 4px; box-sizing: border-box }
         div + div { margin-top: -4px }
      </style>
      <div style="border: 1px dotted #1111"></div>
      <div style="background: url(pattern.png)"></div>
    `, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.xfail {
				t.Skip("upstream marks it as an expected failure")
			}
			assertPixels(t, "image_after_alpha", tc.pixels, tc.html)
		})
	}
}

// noImage is no_image in tests/draw/test_image.py.
const noImage = `
    ________
    ________
    ________
    ________
    ________
    ________
    ________
    ________
`

// TestDrawImagesNotFound is test_images_not_found in
// tests/draw/test_image.py.
func TestDrawImagesNotFound(t *testing.T) {
	log, logs := captureLogs(slog.LevelInfo)
	assertPixelsLog(t, "images_not_found", noImage, `
          <style>
            @page { size: 8px }
            body { margin: 0; font-size: 0 }
            img { display: block; margin: 2px auto 0 }
          </style>
          <div><img src="inexistent1.png" alt=""></div>`, log)
	assertLog(t, logs, "ERROR: Failed to load image")
	assertLog(t, logs, "inexistent1.png")
}

// TestDrawImagesRefused is test_images_refused in
// tests/draw/test_image.py.
func TestDrawImagesRefused(t *testing.T) {
	log, logs := captureLogs(slog.LevelInfo)
	assertPixelsLog(t, "images_refused", noImage, `
          <style>
            @page { size: 8px }
            body { margin: 0; font-size: 0 }
            img { display: block; margin: 2px auto 0 }
          </style>
          <div><img src="dangerous.eps" alt=""></div>`, log)
	assertLog(t, logs, "ERROR: Failed to load image")
	assertLog(t, logs, "dangerous.eps")
}

// TestDrawImagesAlt is test_images_alt in tests/draw/test_image.py.
func TestDrawImagesAlt(t *testing.T) {
	log, logs := captureLogs(slog.LevelInfo)
	documents := make([]string, 0, 4)
	for _, html := range []string{
		"Hello",
		`<img src="inexistent2.png" alt="Hello">`,
		`<img alt="Hello">`,
		`<img src="data:image/svg+xml,<svg></svg>" alt="Hello">`,
	} {
		documents = append(documents, `
              <style>
                @page { size: 200px 30px }
                body { margin: 0; font-size: 0 }
              </style>
              <div>`+html+`</div>`)
	}
	assertSameRenderingsLog(t, "images_alt", 0, log, documents...)
	assertLog(t, logs, "ERROR: Failed to load image")
	assertLog(t, logs, "inexistent2.png")
}

// TestDrawImageExifImageOrientationKeepFormat is
// test_image_exif_image_orientation_keep_format in tests/draw/test_image.py.
func TestDrawImageExifImageOrientationKeepFormat(t *testing.T) {
	t.Skip("it checks that the PDF file keeps the data of the JPEG image, and ivy writes no PDF file")
}
