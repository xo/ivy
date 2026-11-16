// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/draw/color.py: the colors of the borders.

package layout

import (
	"math"

	"github.com/xo/okage/tinycss2"
)

// getColor is get_color in weasyprint/draw/color.py. It returns the color of
// a property, with the color of the style for currentcolor, or nil.
func getColor(style *Style, key string) *tinycss2.Color {
	value := style.Get(key)
	if isStr(value, "currentcolor") {
		value = style.Get("color")
	}
	if c, ok := value.(Color); ok {
		return c.Color
	}
	return nil
}

// rgbToHSV is colorsys.rgb_to_hsv of Python.
func rgbToHSV(r, g, b float64) (h, s, v float64) {
	maxc := max(r, g, b)
	minc := min(r, g, b)
	rangec := maxc - minc
	v = maxc
	if minc == maxc {
		return 0, 0, v
	}
	s = rangec / maxc
	rc := (maxc - r) / rangec
	gc := (maxc - g) / rangec
	bc := (maxc - b) / rangec
	switch {
	case r == maxc:
		h = bc - gc
	case g == maxc:
		h = 2 + rc - bc
	default:
		h = 4 + gc - rc
	}
	h /= 6
	h -= math.Floor(h)
	return h, s, v
}

// hsvToRGB is colorsys.hsv_to_rgb of Python.
func hsvToRGB(h, s, v float64) (r, g, b float64) {
	if s == 0 {
		return v, v, v
	}
	i := math.Floor(h * 6)
	f := h*6 - i
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))
	switch int(i) % 6 {
	case 0:
		return v, t, p
	case 1:
		return q, v, p
	case 2:
		return p, v, t
	case 3:
		return p, q, v
	case 4:
		return t, p, v
	}
	return v, p, q
}

// darken is darken in weasyprint/draw/color.py: a darker color.
func darken(color *tinycss2.Color) *tinycss2.Color {
	// TODO: handle color spaces.
	r, g, b, alpha := srgbOf(color)
	hue, saturation, value := rgbToHSV(r, g, b)
	value /= 1.5
	saturation /= 1.25
	r, g, b = hsvToRGB(hue, saturation, value)
	return newSRGB(r, g, b, alpha)
}

// lighten is lighten in weasyprint/draw/color.py: a lighter color.
func lighten(color *tinycss2.Color) *tinycss2.Color {
	// TODO: handle color spaces.
	r, g, b, alpha := srgbOf(color)
	hue, saturation, value := rgbToHSV(r, g, b)
	value = 1 - (1-value)/1.5
	if saturation != 0 {
		saturation = 1 - (1-saturation)/1.25
	}
	r, g, b = hsvToRGB(hue, saturation, value)
	return newSRGB(r, g, b, alpha)
}

// styledColor is styled_color in weasyprint/draw/color.py: the colors of
// the inset, outset, ridge and groove borders. It returns one color, or two
// colors for ridge and groove, the tuple of upstream.
func styledColor(style string, color *tinycss2.Color, side string) []*tinycss2.Color {
	topLeft := side == "top" || side == "left"
	switch style {
	case "inset", "outset":
		if topLeft != (style == "inset") {
			return []*tinycss2.Color{lighten(color)}
		}
		return []*tinycss2.Color{darken(color)}
	case "ridge", "groove":
		if topLeft != (style == "ridge") {
			return []*tinycss2.Color{lighten(color), darken(color)}
		}
		return []*tinycss2.Color{darken(color), lighten(color)}
	}
	return []*tinycss2.Color{color}
}
