// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/float.py: the layout of the floating
// boxes.

package layout

import (
	"math"
	"slices"
)

// floatWidth is float_width in weasyprint/layout/float.py, with the
// decorator handle_min_max_width.
func floatWidth(box *Box, lc *layoutContext, containingBlock *Box) {
	handleMinMaxWidth(box, func() struct{} {
		// Check that box.width is auto even if the caller does it too,
		// because the handle_min_max_width decorator can change the value
		if box.Width == Auto {
			box.Width = shrinkToFit(lc, box, containingBlock.Width)
		}
		return struct{}{}
	})
}

// floatLayout is float_layout in weasyprint/layout/float.py: it sets the
// width and the position of the floating box, and returns the laid out box
// and where it resumes.
func floatLayout(lc *layoutContext, box, containingBlock *Box, absoluteBoxes, fixedBoxes *[]*Box, bottomSpace float64, skip skipStack) (*Box, skipStack) {
	lc.enter()
	defer lc.leave()

	cbWidth, cbHeight := containingBlock.Width, containingBlock.Height
	resolvePercentages(box, sizeBlock(cbWidth, cbHeight))

	// TODO: This is only handled later in blocks.block_container_layout
	// https://www.w3.org/TR/CSS21/visudet.html#normal-block
	if cbHeight == Auto {
		cbHeight = containingBlock.PositionY - containingBlock.ContentBoxY()
	}

	resolvePositionPercentages(box, sizeBlock(cbWidth, cbHeight))

	if box.MarginLeft == Auto {
		box.MarginLeft = 0
	}
	if box.MarginRight == Auto {
		box.MarginRight = 0
	}
	if box.MarginTop == Auto {
		box.MarginTop = 0
	}
	if box.MarginBottom == Auto {
		box.MarginBottom = 0
	}

	if clearance, ok := getClearance(lc, box, containingDirection(containingBlock), 0); ok {
		box.PositionY += clearance
	}

	if box.Kind == KindBlockReplaced {
		inlineReplacedBoxWidthHeight(box, containingBlock)
	} else if box.Width == Auto {
		floatWidth(box, lc, containingBlock)
	}

	if box.IsTableWrapper {
		tableWrapperWidth(lc, box, sizeBlock(cbWidth, cbHeight))
	}

	var result blockLayoutResult
	switch {
	case box.IsBlockContainer():
		result = blockContainerLayout(lc, box, bottomSpace, skip, true,
			absoluteBoxes, fixedBoxes, nil, nil, nil, false, noMaxLines)
	case box.IsFlexContainer():
		result = flexLayout(lc, box, bottomSpace, skip, containingBlock, true,
			absoluteBoxes, fixedBoxes, false)
	case box.IsGridContainer():
		result = gridLayout(lc, box, bottomSpace, skip, containingBlock, true,
			absoluteBoxes, fixedBoxes)
	default:
		// Port: upstream asserts that the box is a BlockReplacedBox.
		result = blockLayoutResult{box: box.Copy()}
	}
	newBox, resumeAt := result.box, result.resumeAt
	if newBox == nil {
		// Port: upstream fails on a float that its layout cancels. The
		// port keeps the box, without children and without size.
		newBox, resumeAt = emptyLayoutBox(box), nil
	}

	newBox = findFloatPosition(lc, newBox, containingBlock)

	lc.appendExcludedShape(newBox)
	return newBox, resumeAt
}

// containingDirection returns the direction of a containing block, and
// ltr for a sizeBlock.
func containingDirection(containingBlock *Box) string {
	if !containingBlock.isBox() || containingBlock.Style == nil {
		return "ltr"
	}
	return str(containingBlock.Style.Get("direction"))
}

// findFloatPosition is find_float_position in weasyprint/layout/float.py:
// it gets the right position of the float box.
func findFloatPosition(lc *layoutContext, box, containingBlock *Box) *Box {
	// See https://www.w3.org/TR/CSS2/visuren.html#float-position

	// Point 4 is already handled as box.position_y is set according to the
	// containing box top position, with collapsing margins handled

	// Points 5 and 6, box.position_y is set to the highest position_y
	// possible
	if shapes := lc.excludedShapes(); len(shapes) > 0 {
		highestY := shapes[len(shapes)-1].PositionY
		if box.PositionY < highestY {
			box.Translate(0, highestY-box.PositionY, false)
		}
	}

	// Points 1 and 2
	positionX, positionY, availableWidth := avoidCollisions(lc, box, containingBlock, true)

	// Point 9
	// position_y is set now, let's define position_x
	// for float: left elements, it's already done!
	floatValue := box.Style.Get("float")
	direction := box.Style.Get("direction")
	floatRight := isStr(floatValue, "right") ||
		(isStr(direction, "ltr") && isStr(floatValue, "inline-end")) ||
		(isStr(direction, "rtl") && isStr(floatValue, "inline-start"))
	if floatRight {
		positionX += availableWidth - box.MarginWidth()
	}

	box.Translate(positionX-box.PositionX, positionY-box.PositionY, false)

	return box
}

// getClearance is get_clearance in weasyprint/layout/float.py. It returns
// the clearance of box, and false when there is no clearance. direction is
// the value of direction of the containing block.
func getClearance(lc *layoutContext, box *Box, direction string, collapsedMargin float64) (float64, bool) {
	// clears is the closure that reports whether clear and float values
	// match.
	clears := func(clearValue, floatValue string) bool {
		leftIf := func(left bool) string {
			if left {
				return "left"
			}
			return "right"
		}
		if clearValue == "inline-start" {
			clearValue = leftIf(direction == "ltr")
		}
		if clearValue == "inline-end" {
			clearValue = leftIf(direction == "rtl")
		}
		if floatValue == "inline-start" {
			floatValue = leftIf(direction == "ltr")
		}
		if floatValue == "inline-end" {
			floatValue = leftIf(direction == "rtl")
		}
		return clearValue == floatValue || clearValue == "both"
	}

	boxClear := str(box.Style.Get("clear"))
	// Box should be after shape that’s broken on this page.
	for _, broken := range lc.brokenOutOfFlow {
		if broken.newBox.IsFloated() {
			if clears(boxClear, str(broken.newBox.Style.Get("float"))) {
				// Port: upstream returns sys.maxsize.
				return math.MaxInt64, true
			}
		}
	}
	// Hypothetical position is the position of the top border edge
	clearance, hasClearance := 0.0, false
	hypotheticalPosition := box.PositionY + collapsedMargin
	for _, excludedShape := range lc.excludedShapes() {
		if clears(boxClear, str(excludedShape.Style.Get("float"))) {
			y, h := excludedShape.PositionY, excludedShape.MarginHeight()
			if hypotheticalPosition < y+h {
				clearance = max(clearance, y+h-hypotheticalPosition)
				hasClearance = true
			}
		}
	}
	return clearance, hasClearance
}

// avoidCollisions is avoid_collisions in weasyprint/layout/float.py: it
// returns the position and the available width of box among the excluded
// shapes of the formatting context.
func avoidCollisions(lc *layoutContext, box, containingBlock *Box, outer bool) (positionX, positionY, availableWidth float64) {
	excludedShapes := lc.excludedShapes()
	positionY = box.BorderBoxY()
	boxWidth, boxHeight := box.BorderWidth(), box.BorderHeight()
	if outer {
		positionY = box.PositionY
		boxWidth, boxHeight = box.MarginWidth(), box.MarginHeight()
	}

	if box.BorderHeight() == 0 && box.IsFloated() {
		return 0, positionY, containingBlock.Width
	}

	leftKeywords := []string{"left"}
	rightKeywords := []string{"right"}
	if containingDirection(containingBlock) == "ltr" {
		leftKeywords = append(leftKeywords, "inline-start")
		rightKeywords = append(rightKeywords, "inline-end")
	} else {
		leftKeywords = append(leftKeywords, "inline-end")
		rightKeywords = append(rightKeywords, "inline-start")
	}

	var maxLeftBound, maxRightBound float64
	for {
		lc.spend(1)
		var collidingShapes []*Box
		for _, shape := range excludedShapes {
			// Assign locals to avoid slow attribute lookups.
			shapePositionY := shape.PositionY
			shapeMarginHeight := shape.MarginHeight()
			if (shapePositionY < positionY && positionY < shapePositionY+shapeMarginHeight) ||
				(shapePositionY < positionY+boxHeight && positionY+boxHeight < shapePositionY+shapeMarginHeight) ||
				(shapePositionY >= positionY && shapePositionY+shapeMarginHeight <= positionY+boxHeight) {
				collidingShapes = append(collidingShapes, shape)
			}
		}
		var leftBounds, rightBounds []float64
		for _, shape := range collidingShapes {
			floatValue := str(shape.Style.Get("float"))
			if slices.Contains(leftKeywords, floatValue) {
				leftBounds = append(leftBounds, shape.PositionX+shape.MarginWidth())
			}
			if slices.Contains(rightKeywords, floatValue) {
				rightBounds = append(rightBounds, shape.PositionX)
			}
		}

		// Set the default maximum bounds
		maxLeftBound = containingBlock.ContentBoxX()
		maxRightBound = maxLeftBound + containingBlock.Width

		if !outer {
			maxLeftBound += box.MarginLeft
			maxRightBound -= box.MarginRight
		}

		// Set the real maximum bounds according to sibling float elements
		if len(leftBounds) > 0 || len(rightBounds) > 0 {
			if len(leftBounds) > 0 {
				maxLeftBound = max(slices.Max(leftBounds), maxLeftBound)
			}
			if len(rightBounds) > 0 {
				maxRightBound = min(slices.Min(rightBounds), maxRightBound)
			}

			// Points 3, 7 and 8
			if boxWidth > maxRightBound-maxLeftBound {
				// The box does not fit here
				newPositionY := math.Inf(1)
				for _, shape := range collidingShapes {
					newPositionY = min(newPositionY, shape.PositionY+shape.MarginHeight())
				}
				if newPositionY > positionY {
					// We can find a solution with a higher position_y
					positionY = newPositionY
					continue
				}
				// No solution, we must put the box here
			}
		}
		break
	}

	// See https://www.w3.org/TR/CSS21/visuren.html#floats
	// Boxes that can’t collide with floats are:
	// - floats
	// - line boxes
	// - table wrappers
	// - block-level replaced box
	// - element establishing new formatting contexts
	//
	// Port: upstream asserts that the box is one of them.

	// The x-position of the box depends on its type.
	positionX = maxLeftBound
	if isStr(box.Style.Get("float"), "none") {
		if containingDirection(containingBlock) == "rtl" {
			if box.Kind == KindLine {
				// The position of the line is the position of the cursor,
				// at the right bound.
				positionX = maxRightBound
			} else {
				// For a table wrapper: the position of the right border of
				// the table is at the right bound. Else: the position of the
				// right border of the replaced box or formatting context is
				// at the right bound.
				positionX = maxRightBound - boxWidth
			}
		}
	}

	availableWidth = maxRightBound - maxLeftBound

	if !outer {
		positionX -= box.MarginLeft
		positionY -= box.MarginTop
	}

	return positionX, positionY, availableWidth
}
