// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/percent.py: it resolves percentages
// into fixed values.

package layout

import (
	"math"

	"github.com/xo/okage/tinycss2"
)

// percentage is percentage in weasyprint/layout/percent.py. It returns the
// percentage of the reference length referTo, the length for 100%, or the
// value unchanged: nil for None, String("auto") for auto, and a Float in
// pixels for a length, a percentage or a math function.
func percentage(value Value, computed *Style, referTo float64) Value {
	if value == nil || isStr(value, "auto") {
		return value
	}
	switch v := value.(type) {
	case Dimension:
		if v.Unit == "%" {
			return Float(referTo * v.Value / 100)
		}
		// px
		return Float(v.Value)
	case Token:
		return percentageToken(v.Node, computed, referTo, 0)
	case Float, Int:
		// Port: a number that a layout gives back to percentage, such as
		// a size that it already resolved, stays the same.
		return Float(num(v))
	}
	// Port: upstream reads the attribute unit of a value that has none,
	// and fails.
	return nil
}

// usedPercentage is usedValue(percentage(value, computed, referTo)).
//
// Port: percentage gives a Float in a Value, which the port makes only
// for the values that are not lengths.
func usedPercentage(value Value, computed *Style, referTo float64) float64 {
	if d, ok := value.(Dimension); ok {
		if d.Unit == "%" {
			return referTo * d.Value / 100
		}
		// px
		return d.Value
	}
	return usedValue(percentage(value, computed, referTo))
}

// percentageToken is percentage for a token of tinycss2: a math function
// that it resolves, or the token that resolve_math returns.
func percentageToken(token tinycss2.Node, computed *Style, referTo float64, depth int) Value {
	switch t := token.(type) {
	case *tinycss2.DimensionToken:
		return Float(t.Value)
	case *tinycss2.PercentageToken:
		return Float(referTo * t.Value / 100)
	}
	if depth > 1 {
		// Port: resolve_math never returns a math function, so the
		// recursion of upstream stops at the second call.
		return nil
	}
	resolved, err := ResolveMath(token, computed, "", &referTo)
	if err != nil || resolved == nil {
		// Port: upstream raises the errors of resolve_math. A math
		// function that cannot be resolved is None.
		return nil
	}
	return percentageToken(resolved, computed, referTo, depth+1)
}

// percentageFloat returns the number of pixels of percentage, and false
// when it is None or auto.
func percentageFloat(value Value, computed *Style, referTo float64) (float64, bool) {
	f, ok := percentage(value, computed, referTo).(Float)
	return float64(f), ok
}

// usedField returns the field of the box for the attribute of upstream
// that holds the used value of a property, such as box.margin_top for
// "margin_top", or nil for another name.
func (b *Box) usedField(name string) *float64 {
	switch name {
	case "margin_top":
		return &b.MarginTop
	case "margin_right":
		return &b.MarginRight
	case "margin_bottom":
		return &b.MarginBottom
	case "margin_left":
		return &b.MarginLeft
	case "padding_top":
		return &b.PaddingTop
	case "padding_right":
		return &b.PaddingRight
	case "padding_bottom":
		return &b.PaddingBottom
	case "padding_left":
		return &b.PaddingLeft
	case "border_top_width":
		return &b.BorderTopWidth
	case "border_right_width":
		return &b.BorderRightWidth
	case "border_bottom_width":
		return &b.BorderBottomWidth
	case "border_left_width":
		return &b.BorderLeftWidth
	case "width":
		return &b.Width
	case "height":
		return &b.Height
	case "min_width":
		return &b.MinWidth
	case "max_width":
		return &b.MaxWidth
	case "min_height":
		return &b.MinHeight
	case "max_height":
		return &b.MaxHeight
	case "top":
		return &b.ownExtra().Top
	case "right":
		return &b.ownExtra().Right
	case "bottom":
		return &b.ownExtra().Bottom
	case "left":
		return &b.ownExtra().Left
	case "text_indent":
		return &b.textIndent
	}
	return nil
}

// resolveOnePercentage is resolve_one_percentage in
// weasyprint/layout/percent.py. It sets a used length value from a computed
// length value. referTo is the length for 100%.
//
// Port: upstream also takes a referTo that is not a number, and then only
// replaces the percentages. No caller of upstream does that.
func resolveOnePercentage(box *Box, propertyName string, referTo float64) {
	// box attributes are used values
	percent := usedPercentage(box.Style.Get(propertyName), box.Style, referTo)
	field := box.usedField(propertyName)
	if field == nil {
		return
	}
	*field = percent
	if percent == Auto && (propertyName == "min_width" || propertyName == "min_height") {
		*field = 0
	}
}

// The properties that resolvePercentages reads.
var (
	knownBorderBottomWidth = newKnownProperty("border_bottom_width")
	knownBorderCollapse    = newKnownProperty("border_collapse")
	knownBorderLeftWidth   = newKnownProperty("border_left_width")
	knownBorderRightWidth  = newKnownProperty("border_right_width")
	knownBorderTopWidth    = newKnownProperty("border_top_width")
	knownBoxSizing         = newKnownProperty("box_sizing")
	knownHeight            = newKnownProperty("height")
	knownMarginBottom      = newKnownProperty("margin_bottom")
	knownMarginLeft        = newKnownProperty("margin_left")
	knownMarginRight       = newKnownProperty("margin_right")
	knownMarginTop         = newKnownProperty("margin_top")
	knownMaxHeight         = newKnownProperty("max_height")
	knownMaxWidth          = newKnownProperty("max_width")
	knownMinHeight         = newKnownProperty("min_height")
	knownMinWidth          = newKnownProperty("min_width")
	knownPaddingBottom     = newKnownProperty("padding_bottom")
	knownPaddingLeft       = newKnownProperty("padding_left")
	knownPaddingRight      = newKnownProperty("padding_right")
	knownPaddingTop        = newKnownProperty("padding_top")
	knownWidth             = newKnownProperty("width")
)

// resolveKnownPercentage is resolveOnePercentage for a knownProperty.
func resolveKnownPercentage(box *Box, p knownProperty, referTo float64) {
	// box attributes are used values
	percent := usedPercentage(box.Style.getKnown(p), box.Style, referTo)
	field := box.usedField(p.name)
	if field == nil {
		return
	}
	*field = percent
	if percent == Auto && (p.name == "min_width" || p.name == "min_height") {
		*field = 0
	}
}

// resolvePositionPercentages is resolve_position_percentages in
// weasyprint/layout/percent.py. containingBlock is a box or a sizeBlock.
func resolvePositionPercentages(box, containingBlock *Box) {
	cbWidth, cbHeight := containingBlock.Width, containingBlock.Height
	resolveOnePercentage(box, "left", cbWidth)
	resolveOnePercentage(box, "right", cbWidth)
	resolveOnePercentage(box, "top", cbHeight)
	resolveOnePercentage(box, "bottom", cbHeight)
}

// resolvePercentages is resolve_percentages in
// weasyprint/layout/percent.py. It sets the used values as fields of the
// box. containingBlock is a box, or a sizeBlock for a tuple (width,
// height). Its height can be Auto.
func resolvePercentages(box, containingBlock *Box) {
	// cb is short for containing block
	cbWidth, cbHeight := containingBlock.Width, containingBlock.Height
	maybeHeight := cbWidth
	if box.Kind == KindPage {
		maybeHeight = cbHeight
	}
	resolveKnownPercentage(box, knownMarginLeft, cbWidth)
	resolveKnownPercentage(box, knownMarginRight, cbWidth)
	resolveKnownPercentage(box, knownMarginTop, maybeHeight)
	resolveKnownPercentage(box, knownMarginBottom, maybeHeight)
	resolveKnownPercentage(box, knownPaddingLeft, cbWidth)
	resolveKnownPercentage(box, knownPaddingRight, cbWidth)
	resolveKnownPercentage(box, knownPaddingTop, maybeHeight)
	resolveKnownPercentage(box, knownPaddingBottom, maybeHeight)
	resolveKnownPercentage(box, knownWidth, cbWidth)
	resolveKnownPercentage(box, knownMinWidth, cbWidth)
	resolveKnownPercentage(box, knownMaxWidth, cbWidth)

	// XXX later: top, bottom, left and right on positioned elements

	if cbHeight == Auto {
		// Special handling when the height of the containing block
		// depends on its content.
		height := box.Style.getKnown(knownHeight)
		d, isDimension := height.(Dimension)
		if !isDimension || d.Unit == "%" || checkMath(height) {
			box.Height = Auto
		} else {
			// Port: upstream asserts that the unit is px.
			box.Height = d.Value
		}
		resolveKnownPercentage(box, knownMinHeight, 0)
		resolveKnownPercentage(box, knownMaxHeight, math.Inf(1))
	} else {
		resolveKnownPercentage(box, knownHeight, cbHeight)
		resolveKnownPercentage(box, knownMinHeight, cbHeight)
		resolveKnownPercentage(box, knownMaxHeight, cbHeight)
	}

	collapse := isStr(box.Style.getKnown(knownBorderCollapse), "collapse")
	// Used value == computed value
	// border-{side}-width would have been resolved during border conflict
	// resolution for collapsed-borders
	if !collapse || !box.bordersSet {
		box.BorderTopWidth = num(box.Style.getKnown(knownBorderTopWidth))
		box.BorderRightWidth = num(box.Style.getKnown(knownBorderRightWidth))
		box.BorderBottomWidth = num(box.Style.getKnown(knownBorderBottomWidth))
		box.BorderLeftWidth = num(box.Style.getKnown(knownBorderLeftWidth))
		box.bordersSet = true
	}

	// Shrink *content* widths and heights according to box-sizing
	adjustBoxSizing(box, "width")
	adjustBoxSizing(box, "height")
}

// radiusCorners are the properties of the radii of the corners of the
// border, in the order of Box.radii, with the sides of each corner.
var radiusCorners = [...]struct {
	name  string
	sides [2]string
}{
	{"border_top_left_radius", [2]string{"top", "left"}},
	{"border_top_right_radius", [2]string{"top", "right"}},
	{"border_bottom_right_radius", [2]string{"bottom", "right"}},
	{"border_bottom_left_radius", [2]string{"bottom", "left"}},
}

// resolveRadiiPercentages is resolve_radii_percentages in
// weasyprint/layout/percent.py.
//
// Port: the box keeps nil for four radii of 0, and a new array for other
// radii. See Box.radii.
func resolveRadiiPercentages(box *Box) {
	var radii [4][2]float64
	for i, corner := range radiusCorners {
		radius := &radii[i]
		computed, _ := box.Style.Get(corner.name).(Tuple)
		if len(computed) != 2 {
			// Port: upstream fails to unpack a value that is not a pair.
			*radius = [2]float64{}
			continue
		}
		rx, ry := computed[0], computed[1]

		// Short track for common case
		zero := Dimension{Value: 0, Unit: "px"}
		if Equal(rx, zero) || Equal(ry, zero) {
			*radius = [2]float64{}
			continue
		}

		if box.decorationRemoved(corner.sides[0]) || box.decorationRemoved(corner.sides[1]) {
			*radius = [2]float64{}
			continue
		}
		x, _ := percentageFloat(rx, box.Style, box.BorderWidth())
		y, _ := percentageFloat(ry, box.Style, box.BorderHeight())
		*radius = [2]float64{x, y}
	}
	if radii == ([4][2]float64{}) {
		box.radii = nil
	} else {
		r := radii
		box.radii = &r
	}
}

// adjustBoxSizing is adjust_box_sizing in weasyprint/layout/percent.py.
// axis is "width" or "height".
func adjustBoxSizing(box *Box, axis string) {
	var delta float64
	switch str(box.Style.getKnown(knownBoxSizing)) {
	case "border-box":
		if axis == "width" {
			delta = box.PaddingLeft + box.PaddingRight + box.BorderLeftWidth + box.BorderRightWidth
		} else {
			delta = box.PaddingTop + box.PaddingBottom + box.BorderTopWidth + box.BorderBottomWidth
		}
	case "padding-box":
		if axis == "width" {
			delta = box.PaddingLeft + box.PaddingRight
		} else {
			delta = box.PaddingTop + box.PaddingBottom
		}
	default:
		// Port: upstream asserts that box-sizing is content-box.
		delta = 0
	}

	// Keep at least min_* >= 0 to prevent funny output in case box.width or
	// box.height become negative.
	// Restricting max_* seems reasonable, too.
	if delta > 0 {
		size, minSize, maxSize := &box.Width, &box.MinWidth, &box.MaxWidth
		if axis == "height" {
			size, minSize, maxSize = &box.Height, &box.MinHeight, &box.MaxHeight
		}
		if *size != Auto {
			*size = max(0, *size-delta)
		}
		*maxSize = max(0, *maxSize-delta)
		if *minSize != Auto {
			*minSize = max(0, *minSize-delta)
		}
	}
}
