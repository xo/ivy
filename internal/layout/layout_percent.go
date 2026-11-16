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
		return &b.Top
	case "right":
		return &b.Right
	case "bottom":
		return &b.Bottom
	case "left":
		return &b.Left
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
	percent := usedValue(percentage(box.Style.Get(propertyName), box.Style, referTo))
	field := box.usedField(propertyName)
	if field == nil {
		return
	}
	*field = percent
	if percent == Auto && (propertyName == "min_width" || propertyName == "min_height") {
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
	resolveOnePercentage(box, "margin_left", cbWidth)
	resolveOnePercentage(box, "margin_right", cbWidth)
	resolveOnePercentage(box, "margin_top", maybeHeight)
	resolveOnePercentage(box, "margin_bottom", maybeHeight)
	resolveOnePercentage(box, "padding_left", cbWidth)
	resolveOnePercentage(box, "padding_right", cbWidth)
	resolveOnePercentage(box, "padding_top", maybeHeight)
	resolveOnePercentage(box, "padding_bottom", maybeHeight)
	resolveOnePercentage(box, "width", cbWidth)
	resolveOnePercentage(box, "min_width", cbWidth)
	resolveOnePercentage(box, "max_width", cbWidth)

	// XXX later: top, bottom, left and right on positioned elements

	if cbHeight == Auto {
		// Special handling when the height of the containing block
		// depends on its content.
		height := box.Style.Get("height")
		d, isDimension := height.(Dimension)
		if !isDimension || d.Unit == "%" || checkMath(height) {
			box.Height = Auto
		} else {
			// Port: upstream asserts that the unit is px.
			box.Height = d.Value
		}
		resolveOnePercentage(box, "min_height", 0)
		resolveOnePercentage(box, "max_height", math.Inf(1))
	} else {
		resolveOnePercentage(box, "height", cbHeight)
		resolveOnePercentage(box, "min_height", cbHeight)
		resolveOnePercentage(box, "max_height", cbHeight)
	}

	collapse := isStr(box.Style.Get("border_collapse"), "collapse")
	// Used value == computed value
	// border-{side}-width would have been resolved during border conflict
	// resolution for collapsed-borders
	if !collapse || !box.bordersSet {
		box.BorderTopWidth = num(box.Style.Get("border_top_width"))
		box.BorderRightWidth = num(box.Style.Get("border_right_width"))
		box.BorderBottomWidth = num(box.Style.Get("border_bottom_width"))
		box.BorderLeftWidth = num(box.Style.Get("border_left_width"))
		box.bordersSet = true
	}

	// Shrink *content* widths and heights according to box-sizing
	adjustBoxSizing(box, "width")
	adjustBoxSizing(box, "height")
}

// resolveRadiiPercentages is resolve_radii_percentages in
// weasyprint/layout/percent.py.
func resolveRadiiPercentages(box *Box) {
	corners := [...]struct {
		name   string
		sides  [2]string
		radius *[2]float64
	}{
		{"border_top_left_radius", [2]string{"top", "left"}, &box.BorderTopLeftRadius},
		{"border_top_right_radius", [2]string{"top", "right"}, &box.BorderTopRightRadius},
		{"border_bottom_right_radius", [2]string{"bottom", "right"}, &box.BorderBottomRightRadius},
		{"border_bottom_left_radius", [2]string{"bottom", "left"}, &box.BorderBottomLeftRadius},
	}
	for _, corner := range corners {
		computed, _ := box.Style.Get(corner.name).(Tuple)
		if len(computed) != 2 {
			// Port: upstream fails to unpack a value that is not a pair.
			*corner.radius = [2]float64{}
			continue
		}
		rx, ry := computed[0], computed[1]

		// Short track for common case
		zero := Dimension{Value: 0, Unit: "px"}
		if Equal(rx, zero) || Equal(ry, zero) {
			*corner.radius = [2]float64{}
			continue
		}

		if box.RemoveDecorationSides[corner.sides[0]] || box.RemoveDecorationSides[corner.sides[1]] {
			*corner.radius = [2]float64{}
			continue
		}
		x, _ := percentageFloat(rx, box.Style, box.BorderWidth())
		y, _ := percentageFloat(ry, box.Style, box.BorderHeight())
		*corner.radius = [2]float64{x, y}
	}
}

// adjustBoxSizing is adjust_box_sizing in weasyprint/layout/percent.py.
// axis is "width" or "height".
func adjustBoxSizing(box *Box, axis string) {
	var delta float64
	switch str(box.Style.Get("box_sizing")) {
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
