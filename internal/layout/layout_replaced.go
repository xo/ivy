// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/replaced.py: the layout for images and
// other replaced elements.
//
// See https://drafts.csswg.org/css-images-3/#sizing

package layout

// optFloat returns a pointer to v when ok is true, and nil for the None of
// upstream.
func optFloat(v float64, ok bool) *float64 {
	if !ok {
		return nil
	}
	return &v
}

// defaultImageSizing is default_image_sizing in
// weasyprint/layout/replaced.py: the default sizing algorithm for the
// concrete object size. It returns (concrete_width, concrete_height). A nil
// value is None. A specified size of Auto is None too.
//
// See https://drafts.csswg.org/css-images-3/#default-sizing
func defaultImageSizing(intrinsicWidth, intrinsicHeight, intrinsicRatio, specifiedWidth, specifiedHeight *float64, defaultWidth, defaultHeight float64) (width, height float64) {
	if specifiedWidth != nil && *specifiedWidth == Auto {
		specifiedWidth = nil
	}
	if specifiedHeight != nil && *specifiedHeight == Auto {
		specifiedHeight = nil
	}

	switch {
	case specifiedWidth != nil && specifiedHeight != nil:
		return *specifiedWidth, *specifiedHeight
	case specifiedWidth != nil:
		switch {
		case intrinsicRatio != nil:
			return *specifiedWidth, *specifiedWidth / *intrinsicRatio
		case intrinsicHeight != nil:
			return *specifiedWidth, *intrinsicHeight
		}
		return *specifiedWidth, defaultHeight
	case specifiedHeight != nil:
		switch {
		case intrinsicRatio != nil:
			return *specifiedHeight * *intrinsicRatio, *specifiedHeight
		case intrinsicWidth != nil:
			return *intrinsicWidth, *specifiedHeight
		}
		return defaultWidth, *specifiedHeight
	}
	if intrinsicWidth != nil || intrinsicHeight != nil {
		return defaultImageSizing(
			intrinsicWidth, intrinsicHeight, intrinsicRatio,
			intrinsicWidth, intrinsicHeight, defaultWidth, defaultHeight)
	}
	return containConstraintImageSizing(defaultWidth, defaultHeight, intrinsicRatio)
}

// containConstraintImageSizing is contain_constraint_image_sizing in
// weasyprint/layout/replaced.py: the contain constraint sizing algorithm
// for the concrete object size. It returns (concrete_width,
// concrete_height).
//
// See https://drafts.csswg.org/css-images-3/#contain-constraint
func containConstraintImageSizing(constraintWidth, constraintHeight float64, intrinsicRatio *float64) (width, height float64) {
	return constraintImageSizing(constraintWidth, constraintHeight, intrinsicRatio, false)
}

// coverConstraintImageSizing is cover_constraint_image_sizing in
// weasyprint/layout/replaced.py: the cover constraint sizing algorithm for
// the concrete object size. It returns (concrete_width, concrete_height).
//
// See https://drafts.csswg.org/css-images-3/#cover-constraint
func coverConstraintImageSizing(constraintWidth, constraintHeight float64, intrinsicRatio *float64) (width, height float64) {
	return constraintImageSizing(constraintWidth, constraintHeight, intrinsicRatio, true)
}

// constraintImageSizing is _constraint_image_sizing in
// weasyprint/layout/replaced.py.
func constraintImageSizing(constraintWidth, constraintHeight float64, intrinsicRatio *float64, cover bool) (width, height float64) {
	switch {
	case intrinsicRatio == nil:
		return constraintWidth, constraintHeight
	case cover != (constraintWidth > constraintHeight**intrinsicRatio):
		return constraintHeight * *intrinsicRatio, constraintHeight
	}
	return constraintWidth, constraintWidth / *intrinsicRatio
}

// intrinsicSize returns the intrinsic size of the image of a replaced box,
// for its resolution and its font size.
func intrinsicSize(box *Box) IntrinsicSize {
	if box.extra().Replacement == nil {
		// Port: a replaced box always has an image upstream.
		return IntrinsicSize{}
	}
	return box.extra().Replacement.GetIntrinsicSize(
		num(box.Style.Get("image_resolution")), num(box.Style.Get("font_size")))
}

// replacedBoxLayout is replacedbox_layout in weasyprint/layout/replaced.py:
// the size and the position of the image of a replaced box, for the
// drawing.
func replacedBoxLayout(box *Box) (drawWidth, drawHeight, positionX, positionY float64) {
	// TODO: respect box-sizing ?
	objectFit := str(box.Style.Get("object_fit"))
	position := box.Style.Get("object_position")

	size := intrinsicSize(box)
	intrinsicWidth, intrinsicHeight := size.Width, size.Height
	intrinsicRatio := optFloat(size.Ratio, size.HasRatio)
	if !size.HasWidth || !size.HasHeight {
		intrinsicWidth, intrinsicHeight = containConstraintImageSizing(
			box.Width, box.Height, intrinsicRatio)
	}

	if objectFit == "fill" {
		drawWidth, drawHeight = box.Width, box.Height
	} else {
		switch objectFit {
		case "contain", "scale-down":
			drawWidth, drawHeight = containConstraintImageSizing(
				box.Width, box.Height, intrinsicRatio)
		case "cover":
			drawWidth, drawHeight = coverConstraintImageSizing(
				box.Width, box.Height, intrinsicRatio)
		default:
			// Port: upstream asserts that object-fit is none.
			drawWidth, drawHeight = intrinsicWidth, intrinsicHeight
		}

		if objectFit == "scale-down" {
			drawWidth = min(drawWidth, intrinsicWidth)
			drawHeight = min(drawHeight, intrinsicHeight)
		}
	}

	// Port: upstream fails to unpack a position that is not a tuple of
	// four items. The port places such an image at the top left corner.
	var originX, originY string
	var valueX, valueY Value
	if positions, ok := position.(Tuple); ok && len(positions) > 0 {
		if first, ok := positions[0].(Tuple); ok && len(first) == 4 {
			originX, valueX, originY, valueY = str(first[0]), first[1], str(first[2]), first[3]
		}
	}
	refX := box.Width - drawWidth
	refY := box.Height - drawHeight

	positionX, _ = percentageFloat(valueX, box.Style, refX)
	positionY, _ = percentageFloat(valueY, box.Style, refY)
	if originX == "right" {
		positionX = refX - positionX
	}
	if originY == "bottom" {
		positionY = refY - positionY
	}

	positionX += box.ContentBoxX()
	positionY += box.ContentBoxY()

	return drawWidth, drawHeight, positionX, positionY
}

// replacedBoxWidth is replaced_box_width in weasyprint/layout/replaced.py,
// with the decorator handle_min_max_width: it sets the used width for
// replaced boxes.
func replacedBoxWidth(box, containingBlock *Box) {
	handleMinMaxWidth(box, func() struct{} {
		replacedBoxWidthWithoutMinMax(box, containingBlock)
		return struct{}{}
	})
}

// replacedBoxWidthWithoutMinMax is replaced_box_width.without_min_max in
// weasyprint/layout/replaced.py.
func replacedBoxWidthWithoutMinMax(box, containingBlock *Box) {
	size := intrinsicSize(box)

	// This algorithm simply follows the different points of the
	// specification:
	// https://www.w3.org/TR/CSS21/visudet.html#inline-replaced-width
	if box.Height == Auto && box.Width == Auto {
		if size.HasWidth {
			// Point #1
			box.Width = size.Width
		} else if size.HasRatio {
			if size.HasHeight {
				// Point #2 first part
				box.Width = size.Height * size.Ratio
			} else {
				// Point #3
				blockLevelWidth(box, containingBlock)
			}
		}
	}

	if box.Width == Auto {
		switch {
		case size.HasRatio && box.Height != Auto:
			// Point #2 second part
			box.Width = box.Height * size.Ratio
		case size.HasWidth:
			// Point #4
			box.Width = size.Width
		default:
			// Point #5
			// It's pretty useless to rely on device size to set width.
			//
			// Port: upstream multiplies the height auto by the ratio and
			// fails. The port gives the default width.
			box.Width = 300
		}
	}
}

// replacedBoxHeight is replaced_box_height in
// weasyprint/layout/replaced.py, with the decorator handle_min_max_height:
// it computes and sets the used height for replaced boxes.
func replacedBoxHeight(box *Box) {
	handleMinMaxHeight(box, func() struct{} {
		replacedBoxHeightWithoutMinMax(box)
		return struct{}{}
	})
}

// replacedBoxHeightWithoutMinMax is replaced_box_height.without_min_max in
// weasyprint/layout/replaced.py.
func replacedBoxHeightWithoutMinMax(box *Box) {
	// https://www.w3.org/TR/CSS21/visudet.html#inline-replaced-height
	size := intrinsicSize(box)

	// Test 'auto' on the computed width, not the used width
	//
	// Port: upstream sets the height None for an image without intrinsic
	// height. The port keeps Auto, which the rules below resolve.
	if box.Height == Auto && box.Width == Auto {
		if size.HasHeight {
			box.Height = size.Height
		}
	} else if box.Height == Auto && size.HasRatio && size.Ratio != 0 {
		box.Height = box.Width / size.Ratio
	}

	switch {
	case box.Height == Auto && box.Width == Auto && size.HasHeight:
		box.Height = size.Height
	case size.HasRatio && box.Height == Auto && size.Ratio != 0 && box.Width != Auto:
		// Port: upstream divides by a ratio of 0 and fails.
		box.Height = box.Width / size.Ratio
	case box.Height == Auto && size.HasHeight:
		box.Height = size.Height
	case box.Height == Auto:
		// It's pretty useless to rely on device size to set width.
		box.Height = 150
	}
}

// inlineReplacedBoxLayout is inline_replaced_box_layout in
// weasyprint/layout/replaced.py: it lays out an inline ReplacedBox box.
func inlineReplacedBoxLayout(box, containingBlock *Box) {
	for _, margin := range []*float64{&box.MarginTop, &box.MarginRight, &box.MarginBottom, &box.MarginLeft} {
		if *margin == Auto {
			*margin = 0
		}
	}
	inlineReplacedBoxWidthHeight(box, containingBlock)
}

// inlineReplacedBoxWidthHeight is inline_replaced_box_width_height in
// weasyprint/layout/replaced.py.
func inlineReplacedBoxWidthHeight(box, containingBlock *Box) {
	if isStr(box.Style.Get("width"), "auto") && isStr(box.Style.Get("height"), "auto") {
		replacedBoxWidthWithoutMinMax(box, containingBlock)
		replacedBoxHeightWithoutMinMax(box)
		minMaxAutoReplaced(box)
	} else {
		replacedBoxWidth(box, containingBlock)
		replacedBoxHeight(box)
	}
}

// minMaxAutoReplaced is min_max_auto_replaced in
// weasyprint/layout/replaced.py: it resolves the min and max constraints
// on replaced elements with auto sizes.
func minMaxAutoReplaced(box *Box) {
	width := box.Width
	height := box.Height
	minWidth := box.MinWidth
	minHeight := box.MinHeight
	maxWidth := max(minWidth, box.MaxWidth)
	maxHeight := max(minHeight, box.MaxHeight)

	// (violation_width, violation_height)
	violation := func(size, minSize, maxSize float64) string {
		switch {
		case size < minSize:
			return "min"
		case size > maxSize:
			return "max"
		}
		return ""
	}
	violations := [2]string{violation(width, minWidth, maxWidth), violation(height, minHeight, maxHeight)}

	// Work around divisions by zero. These are pathological cases anyway.
	// TODO: is there a cleaner way?
	if width == 0 {
		width = 1e-6
	}
	if height == 0 {
		height = 1e-6
	}

	// ('', ''): nothing to do
	switch violations {
	case [2]string{"max", ""}:
		box.Width = maxWidth
		box.Height = max(maxWidth*height/width, minHeight)
	case [2]string{"min", ""}:
		box.Width = minWidth
		box.Height = min(minWidth*height/width, maxHeight)
	case [2]string{"", "max"}:
		box.Width = max(maxHeight*width/height, minWidth)
		box.Height = maxHeight
	case [2]string{"", "min"}:
		box.Width = min(minHeight*width/height, maxWidth)
		box.Height = minHeight
	case [2]string{"max", "max"}:
		if maxWidth/width <= maxHeight/height {
			box.Width = maxWidth
			box.Height = max(minHeight, maxWidth*height/width)
		} else {
			box.Width = max(minWidth, maxHeight*width/height)
			box.Height = maxHeight
		}
	case [2]string{"min", "min"}:
		if minWidth/width <= minHeight/height {
			box.Width = min(maxWidth, minHeight*width/height)
			box.Height = minHeight
		} else {
			box.Width = minWidth
			box.Height = min(maxHeight, minWidth*height/width)
		}
	case [2]string{"min", "max"}:
		box.Width = minWidth
		box.Height = maxHeight
	case [2]string{"max", "min"}:
		box.Width = maxWidth
		box.Height = minHeight
	}
}

// blockReplacedBoxLayout is block_replaced_box_layout in
// weasyprint/layout/replaced.py: it lays out the block ReplacedBox box.
func blockReplacedBoxLayout(lc *layoutContext, box, containingBlock *Box) blockLayoutResult {
	box = box.Copy()
	if isStr(box.Style.Get("width"), "auto") && isStr(box.Style.Get("height"), "auto") {
		computedMarginLeft, computedMarginRight := box.MarginLeft, box.MarginRight
		blockReplacedWidthWithoutMinMax(box, containingBlock)
		replacedBoxHeightWithoutMinMax(box)
		minMaxAutoReplaced(box)
		box.MarginLeft, box.MarginRight = computedMarginLeft, computedMarginRight
		blockLevelWidthWithoutMinMax(box, containingBlock)
	} else {
		blockReplacedWidth(box, containingBlock)
		replacedBoxHeight(box)
	}

	// TODO: flex items shouldn't be block boxes, this condition
	// would then be useless when this is fixed.
	if !box.IsFlexItem {
		// Don't collide with floats
		// https://www.w3.org/TR/CSS21/visuren.html#floats
		box.PositionX, box.PositionY, _ = avoidCollisions(lc, box, containingBlock, false)
	}
	return blockLayoutResult{
		box:      box,
		nextPage: anyNextPage(),
		maxLines: noMaxLines,
	}
}

// blockReplacedWidth is block_replaced_width in
// weasyprint/layout/replaced.py, with the decorator handle_min_max_width.
func blockReplacedWidth(box, containingBlock *Box) {
	handleMinMaxWidth(box, func() struct{} {
		blockReplacedWidthWithoutMinMax(box, containingBlock)
		return struct{}{}
	})
}

// blockReplacedWidthWithoutMinMax is block_replaced_width.without_min_max
// in weasyprint/layout/replaced.py.
func blockReplacedWidthWithoutMinMax(box, containingBlock *Box) {
	// https://www.w3.org/TR/CSS21/visudet.html#block-replaced-width
	replacedBoxWidthWithoutMinMax(box, containingBlock)
	blockLevelWidthWithoutMinMax(box, containingBlock)
}
