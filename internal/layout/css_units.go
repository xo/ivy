// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/units.py.

package layout

import (
	"math"
	"strings"
)

// angleToRadians is ANGLE_TO_RADIANS in weasyprint/css/units.py: how many
// radians is one unit. See https://drafts.csswg.org/css-values-4/#angles.
var angleToRadians = map[string]float64{
	"rad":  1,
	"turn": 2 * math.Pi,
	"deg":  math.Pi / 180,
	"grad": math.Pi / 200,
}

// lengthsToPixels is LENGTHS_TO_PIXELS in weasyprint/css/units.py: how many
// CSS pixels is one unit. See
// https://www.w3.org/TR/CSS21/syndata.html#length-units.
var lengthsToPixels = map[string]float64{
	"px": 1,
	"pt": 1 / 0.75,
	"pc": 16,
	"in": 96,
	"cm": 96 / 2.54,
	"mm": 96 / 25.4,
	"q":  96 / 25.4 / 4,
}

// resolutionToDPPX is RESOLUTION_TO_DPPX in weasyprint/css/units.py: how
// many dppx is one unit. See https://drafts.csswg.org/css-values/#resolution.
var resolutionToDPPX = map[string]float64{
	"dppx": 1,
	"x":    1,
	"dpi":  1 / lengthsToPixels["in"],
	"dpcm": 1 / lengthsToPixels["cm"],
}

// The sets of units of weasyprint/css/units.py. See
// https://drafts.csswg.org/css-values-4/#lengths.
var (
	// absoluteUnits is ABSOLUTE_UNITS.
	absoluteUnits = setOf("px", "pt", "pc", "in", "cm", "mm", "q")
	// fontUnits is FONT_UNITS.
	fontUnits = setOf(
		"em", "ex", "cap", "ch", "ic", "lh",
		"rem", "rex", "rcap", "rch", "ric", "rlh")
	// viewportUnits is VIEWPORT_UNITS.
	viewportUnits = setOf(
		"vw", "vh", "vi", "vb", "vmin", "vmax",
		"lvw", "lvh", "lvi", "lvb", "lvmin", "lvmax",
		"svw", "svh", "svi", "svb", "svmin", "svmax",
		"dvw", "dvh", "dvi", "dvb", "dvmin", "dvmax",
		"pvw", "pvh", "pvi", "pvb", "pvmin", "pvmax")
	// relativeUnits is RELATIVE_UNITS.
	relativeUnits = union(fontUnits, viewportUnits)
	// lengthUnits is LENGTH_UNITS.
	lengthUnits = union(absoluteUnits, relativeUnits)
	// angleUnits is ANGLE_UNITS.
	angleUnits = setOf("rad", "turn", "deg", "grad")
)

// setOf makes a set of strings.
func setOf(strs ...string) map[string]bool {
	set := make(map[string]bool, len(strs))
	for _, s := range strs {
		set[s] = true
	}
	return set
}

// union returns the union of two sets.
func union(a, b map[string]bool) map[string]bool {
	set := make(map[string]bool, len(a)+len(b))
	for s := range a {
		set[s] = true
	}
	for s := range b {
		set[s] = true
	}
	return set
}

// toPixels is to_pixels in weasyprint/css/units.py. It returns the number of
// pixels of a length. style can be nil for an absolute length, and fontSize
// is the font size to use for em, or a negative number for the font size of
// the style, the None of upstream.
func toPixels(value Dimension, style *Style, propertyName string, fontSize float64) float64 {
	if value.Value == 0 {
		return 0
	}
	unit := strings.ToLower(value.Unit)
	switch {
	case unit == "px":
		return value.Value
	case lengthsToPixels[unit] != 0:
		// Convert absolute lengths to pixels.
		return value.Value * lengthsToPixels[unit]
	case fontUnits[unit]:
		if style == nil && fontSize < 0 {
			// Port: upstream asserts. A length that needs a font and has no
			// style is 0.
			return 0
		}
		if fontSize < 0 {
			fontSize = num(style.Get("font_size"))
		}
		switch {
		case unit == "lh":
			var lineHeight float64
			if propertyName == "font_size" || propertyName == "line_height" {
				parentStyle := style.ParentStyle
				if parentStyle == nil {
					parentStyle = style.RootStyle
				}
				lineHeight, _ = strut(parentStyle)
			} else {
				lineHeight, _ = strut(style)
			}
			return value.Value * lineHeight
		case unit == "rlh":
			lineHeight, _ := strut(style.RootStyle)
			return value.Value * lineHeight
		case unit == "em":
			return value.Value * fontSize
		case unit == "rem":
			return value.Value * num(style.RootStyle.Get("font_size"))
		case strings.HasPrefix(unit, "r"):
			ratio := characterRatio(style.RootStyle, unit[1:])
			return value.Value * num(style.RootStyle.Get("font_size")) * ratio
		default:
			ratio := characterRatio(style, unit)
			return value.Value * fontSize * ratio
		}
	case viewportUnits[unit]:
		var pageWidth, pageHeight float64
		var size *[2]float64
		if style != nil && style.ctx != nil {
			if unit[0] == 'p' {
				size = style.ctx.InitialPageSizes.Box
			} else {
				size = style.ctx.InitialPageSizes.Area
			}
		}
		if size == nil {
			if style != nil {
				style.ctx.log.warning("%s unit resolved before first page layout", unit)
			}
			pageWidth = toPixels(initialPageSize[0], nil, "", -1)
			pageHeight = toPixels(initialPageSize[1], nil, "", -1)
		} else {
			pageWidth, pageHeight = size[0], size[1]
		}
		// TODO: use writing-mode for vi and vb.
		switch {
		case strings.HasSuffix(unit, "vw"), strings.HasSuffix(unit, "vi"):
			return value.Value / 100 * pageWidth
		case strings.HasSuffix(unit, "vh"), strings.HasSuffix(unit, "vb"):
			return value.Value / 100 * pageHeight
		case strings.HasSuffix(unit, "vmin"):
			return value.Value / 100 * min(pageWidth, pageHeight)
		case strings.HasSuffix(unit, "vmax"):
			return value.Value / 100 * max(pageWidth, pageHeight)
		}
	}
	// Port: upstream returns None for an unknown unit.
	return 0
}

// toRadians is to_radians in weasyprint/css/units.py. It returns the number
// of radians of an angle.
func toRadians(value Dimension) float64 {
	unit := strings.ToLower(value.Unit)
	if unit == "rad" {
		return value.Value
	}
	return value.Value * angleToRadians[unit]
}

// TextMetrics measures the fonts of a style for the units of CSS that depend
// on a font, such as ex and lh. It is the hook of the functions strut and
// character_ratio of weasyprint/text/line_break.py, which the stage of the
// port that shapes text fills.
type TextMetrics interface {
	// Strut is strut in weasyprint/text/line_break.py. It returns the used
	// value of line-height and the baseline, from the top edge of the line
	// height.
	Strut(style *Style) (lineHeight, baseline float64)
	// CharacterRatio is character_ratio in weasyprint/text/line_break.py. It
	// returns the ratio of the size of a character to the font size, for the
	// unit "ex", "cap", "ic" or "ch".
	CharacterRatio(style *Style, unit string) float64
}

// metricsOf returns the TextMetrics of a style: the Metrics of its context,
// or its FontConfig when it measures the fonts too, such as a
// FontConfiguration, or the default metrics.
func metricsOf(style *Style) TextMetrics {
	if style.ctx != nil {
		if style.ctx.Metrics != nil {
			return style.ctx.Metrics
		}
		if metrics, ok := style.ctx.FontConfig.(TextMetrics); ok {
			return metrics
		}
	}
	return defaultMetrics{}
}

// strut is strut in weasyprint/text/line_break.py, through the TextMetrics
// of the style.
func strut(style *Style) (lineHeight, baseline float64) {
	if style == nil {
		return 0, 0
	}
	return metricsOf(style).Strut(style)
}

// characterRatio is character_ratio in weasyprint/text/line_break.py,
// through the TextMetrics of the style.
func characterRatio(style *Style, unit string) float64 {
	if style == nil {
		return 0.5
	}
	return metricsOf(style).CharacterRatio(style, unit)
}

// defaultMetrics is the TextMetrics of a style without one. It measures no
// font. A normal line is 1.2 times the font size, with the baseline at 0.8
// of the font size from its top, and each character is half the font size,
// the fallback of character_ratio of upstream.
type defaultMetrics struct{}

// Strut satisfies TextMetrics.
func (defaultMetrics) Strut(style *Style) (lineHeight, baseline float64) {
	fontSize := num(style.Get("font_size"))
	if fontSize == 0 {
		return 0, 0
	}
	textHeight, textBaseline := 1.2*fontSize, 0.9*fontSize
	lh := style.Get("line_height")
	if isStr(lh, "normal") {
		return textHeight, textBaseline
	}
	switch v := lh.(type) {
	case Float:
		lineHeight = float64(v) * fontSize
	case Int:
		lineHeight = float64(v) * fontSize
	case Dimension:
		lineHeight = v.Value
	default:
		return textHeight, textBaseline
	}
	return lineHeight, textBaseline + (lineHeight-textHeight)/2
}

// CharacterRatio satisfies TextMetrics.
func (defaultMetrics) CharacterRatio(*Style, string) float64 {
	return 0.5
}
