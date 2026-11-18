// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/flex.py: the layout of the flex
// containers and of the flex items.
//
// Port: upstream reads and sets the attributes of the main and the cross
// axes by name, with getattr(box, main). The port reads them with
// Box.usedField, which gives the field of a name such as "width" or
// "min_height".

package layout

import (
	"cmp"
	"math"
	"slices"
	"strings"
)

// flexItem holds the attributes that flex_layout sets on the flex items.
type flexItem struct {
	// flexBaseSize and hypotheticalMainSize are the flex base size and the
	// hypothetical main size of the item. mainOuterExtra is the size of
	// its margins, its borders and its paddings on the main axis.
	flexBaseSize         float64
	hypotheticalMainSize float64
	mainOuterExtra       float64
	// flexFactor is the used flex factor, the flex grow or the flex shrink
	// factor. scaledFlexShrinkFactor is the flex shrink factor times the
	// flex base size.
	flexFactor             float64
	scaledFlexShrinkFactor float64
	// targetMainSize is the target main size, frozen is set when the item
	// is frozen, and adjustment is the change of its size that a min or a
	// max violation gives.
	targetMainSize float64
	frozen         bool
	adjustment     float64
	// baseline is the attribute _baseline of upstream: the position of the
	// first in-flow baseline of the item, or 0.
	baseline float64
}

// flexLineItem is an item of a FlexLine of upstream: the index of a child
// in the children of the flex container, and the child.
type flexLineItem struct {
	index int
	child *Box
}

// flexLine is FlexLine in weasyprint/layout/flex.py: a line of a flex
// container.
type flexLine struct {
	items         []flexLineItem
	crossSize     float64
	lowerBaseline float64
}

// flexGap is the computation of the used value of row-gap and of
// column-gap in flex_layout: 0 for normal, and a percentage of size, which
// can be Auto.
func flexGap(gap Value, size float64) float64 {
	d, ok := gap.(Dimension)
	if !ok {
		// normal. Port: upstream reads the attribute unit of a math
		// function, and fails. The port gives 0.
		return 0
	}
	if d.Unit == "%" {
		if size == Auto {
			return 0
		}
		return d.Value / 100 * size
	}
	return d.Value
}

// autoZero returns 0 for Auto, and the value for another value.
func autoZero(v float64) float64 {
	if v == Auto {
		return 0
	}
	return v
}

// flexMagnitude is int(log10(space)) if space > 0 else -inf, the magnitude
// of the free space of flex_layout.
func flexMagnitude(space float64) float64 {
	if space > 0 {
		return math.Trunc(math.Log10(space))
	}
	return math.Inf(-1)
}

// flexPositionField returns the field position_x of box for the axis
// "width", and the field position_y for the axis "height".
func flexPositionField(box *Box, axis string) *float64 {
	if axis == "width" {
		return &box.PositionX
	}
	return &box.PositionY
}

// flexLayout is flex_layout in weasyprint/layout/flex.py: the layout of a
// flex container and of its flex items. See
// https://www.w3.org/TR/css-flexbox-1/#layout-algorithm.
func flexLayout(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, containingBlock *Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, discard bool) blockLayoutResult {
	lc.enter()
	defer lc.leave()
	// Port: a nil list is a new list.
	if absoluteBoxes == nil {
		absoluteBoxes = &[]*Box{}
	}
	if fixedBoxes == nil {
		fixedBoxes = &[]*Box{}
	}

	// TODO: merge this with block_container_layout.
	lc.createFlexFormattingContext(box)
	var resumeAt skipStack
	nextPage := anyNextPage()

	isStart := skip == nil
	box.RemoveDecoration(!isStart, false)

	discard = discard || isStr(box.Style.Get("continue"), "discard")
	drawBottomDecoration := discard || isStr(box.Style.Get("box_decoration_break"), "clone")

	rowGapValue, columnGapValue := box.Style.Get("row_gap"), box.Style.Get("column_gap")

	if drawBottomDecoration {
		bottomSpace += box.PaddingBottom + box.BorderBottomWidth + autoZero(box.MarginBottom)
	}

	if isStr(box.Style.Get("position"), "relative") {
		// New containing block, use a new absolute list
		absoluteBoxes = &[]*Box{}
	}

	// References are to: https://www.w3.org/TR/css-flexbox-1/#layout-algorithm.

	// 1 Initial setup, done in formatting_structure.build.

	// 2 Determine the available main and cross space for the flex items.
	flexDirection := str(box.Style.Get("flex_direction"))
	main, cross := "height", "width"
	if strings.HasPrefix(flexDirection, "row") {
		main, cross = "width", "height"
	}
	reverse := strings.HasSuffix(flexDirection, "-reverse")

	marginLeft := autoZero(box.MarginLeft)
	marginRight := autoZero(box.MarginRight)

	// Define available main space.
	// TODO: min- and max-content not implemented.
	var availableMainSpace float64
	if size := *box.usedField(main); size != Auto {
		// If that dimension of the flex container’s content box is a definite size…
		availableMainSpace = size
	} else {
		// Otherwise, subtract the flex container’s margin, border, and padding…
		if main == "width" {
			availableMainSpace = containingBlock.Width -
				marginLeft - marginRight -
				box.PaddingLeft - box.PaddingRight -
				box.BorderLeftWidth - box.BorderRightWidth
		} else {
			availableMainSpace = math.Inf(1)
		}
	}

	// Same as above for available cross space.
	// TODO: min- and max-content not implemented.
	var availableCrossSpace float64
	if size := *box.usedField(cross); size != Auto {
		availableCrossSpace = size
	} else {
		if cross == "width" {
			availableCrossSpace = containingBlock.Width -
				marginLeft - marginRight -
				box.PaddingLeft - box.PaddingRight -
				box.BorderLeftWidth - box.BorderRightWidth
		} else {
			availableCrossSpace = math.Inf(1)
		}
	}

	// 3 Determine the flex base size and hypothetical main size of each item.
	parentBox := box.Copy()
	resolvePercentages(parentBox, containingBlock)
	parentBox.RemoveDecoration(!isStart, false)
	blockLevelWidth(parentBox, containingBlock)
	children := slices.Clone(box.Children)
	slices.SortStableFunc(children, func(a, b *Box) int {
		return cmp.Compare(num(a.Style.Get("order")), num(b.Style.Get("order")))
	})
	skipIndex := 0
	if skip != nil {
		skipIndex, skip = skip.item()
		// Port: upstream slices the list with any index. The port keeps
		// the index in the list.
		skipIndex = max(0, skipIndex)
		if reverse {
			children = children[:min(skipIndex+1, len(children))]
		} else {
			children = children[min(skipIndex, len(children)):]
		}
	}
	childSkipStack := skip

	rowGap := flexGap(rowGapValue, box.Height)
	columnGap := flexGap(columnGapValue, box.Width)
	mainGap, crossGap := rowGap, columnGap
	if main == "width" {
		mainGap, crossGap = columnGap, rowGap
	}

	positionX := parentBox.PositionX + parentBox.BorderLeftWidth + parentBox.PaddingLeft
	if parentBox.MarginLeft != Auto {
		positionX += parentBox.MarginLeft
	}
	positionY := parentBox.PositionY + parentBox.BorderTopWidth + parentBox.PaddingTop
	if parentBox.MarginTop != Auto {
		positionY += parentBox.MarginTop
	}
	for index, child := range children {
		lc.spend(1)
		if !child.IsFlexItem {
			// Absolute child layout: create placeholder.
			if child.IsAbsolutelyPositioned() {
				child.PositionX = positionX
				child.PositionY = positionY
				placeholder := newAbsolutePlaceholder(child)
				placeholder.index = index
				children[index] = placeholder
				if isStr(child.Style.Get("position"), "absolute") {
					*absoluteBoxes = append(*absoluteBoxes, placeholder)
				} else {
					*fixedBoxes = append(*fixedBoxes, placeholder)
				}
			} else if child.IsRunning() || child.IsNote() {
				lc.addRunningElement(child)
			}
			continue
		}
		// See https://www.w3.org/TR/css-flexbox-1/#min-size-auto.
		var childContainingBlock *Box
		if main == "width" {
			childContainingBlock = sizeBlock(availableMainSpace, parentBox.Height)
		} else {
			childContainingBlock = sizeBlock(parentBox.Width, availableMainSpace)
		}
		resolvePercentages(child, childContainingBlock)
		if child.IsTableWrapper {
			tableWrapperWidth(lc, child, childContainingBlock)
		}
		child.PositionX = positionX
		child.PositionY = positionY
		if isStr(child.Style.Get("min_width"), "auto") {
			specifiedSize := child.Width
			newChild := child.Copy()
			newChild.Style = child.Style.Copy()
			newChild.Style.Set("width", String("auto"))
			newChild.Style.Set("min_width", Dimension{Value: 0, Unit: "px"})
			newChild.Style.Set("max_width", Dimension{Value: math.Inf(1), Unit: "px"})
			contentSize := minContentWidth(lc, newChild, false)
			transferredSize, hasTransferredSize := 0.0, false
			if child.IsReplaced() && child.extra().Replacement != nil {
				size := child.extra().Replacement.GetIntrinsicSize(
					num(child.Style.Get("image_resolution")), num(child.Style.Get("font_size")))
				if size.HasRatio && size.Ratio != 0 && size.HasHeight && size.Height != 0 {
					transferredSize, hasTransferredSize = size.Height*size.Ratio, true
					contentSize = max(child.MinWidth, min(child.MaxWidth, contentSize))
				}
			}
			switch {
			case specifiedSize != Auto:
				child.MinWidth = min(specifiedSize, contentSize)
			case hasTransferredSize:
				child.MinWidth = min(transferredSize, contentSize)
			default:
				child.MinWidth = contentSize
			}
		}
		if isStr(child.Style.Get("min_height"), "auto") {
			// TODO: avoid calling block_level_layout, write min_content_height instead.
			specifiedSize := child.Height
			newChild := child.Copy()
			newChild.Style = child.Style.Copy()
			newChild.Style.Set("height", String("auto"))
			newChild.Style.Set("min_height", Dimension{Value: 0, Unit: "px"})
			newChild.Style.Set("max_height", Dimension{Value: math.Inf(1), Unit: "px"})
			if isStr(newChild.Style.Get("width"), "auto") {
				newChildWidth := maxContentWidth(lc, newChild, true)
				newChild.Style.Set("width", Dimension{Value: newChildWidth, Unit: "px"})
			}
			newChild = blockLevelLayout(lc, newChild, bottomSpace, childSkipStack, parentBox,
				pageIsEmpty, nil, nil, nil, nil, nil, false, noMaxLines).box
			contentSize := 0.0
			if newChild != nil {
				contentSize = newChild.Height
			}
			transferredSize, hasTransferredSize := 0.0, false
			if child.IsReplaced() && child.extra().Replacement != nil {
				size := child.extra().Replacement.GetIntrinsicSize(
					num(child.Style.Get("image_resolution")), num(child.Style.Get("font_size")))
				if size.HasRatio && size.Ratio != 0 && size.HasWidth && size.Width != 0 {
					transferredSize, hasTransferredSize = size.Width/size.Ratio, true
					contentSize = max(child.MinHeight, min(child.MaxHeight, contentSize))
				} else if !size.HasWidth || size.Width == 0 {
					// TODO: wrongly set by block_level_layout, would be OK with
					// min_content_height.
					contentSize = 0
				}
			}
			switch {
			case specifiedSize != Auto:
				child.MinHeight = min(specifiedSize, contentSize)
			case hasTransferredSize:
				child.MinHeight = min(transferredSize, contentSize)
			default:
				child.MinHeight = contentSize
			}
			if newChild != nil {
				removePlaceholders(lc, newChild.Children, absoluteBoxes, fixedBoxes)
			}
		}

		flexBasis, contentBasis := 0.0, false
		if isStr(child.Style.Get("flex_basis"), "content") {
			contentBasis = true
		} else {
			basis := percentage(child.Style.Get("flex_basis"), child.Style, availableMainSpace)
			if isStr(basis, "auto") {
				if flexBasis = *child.usedField(main); flexBasis == Auto {
					contentBasis = true
				}
			} else {
				// Port: a basis of None is 0.
				flexBasis = usedValue(basis)
			}
		}

		// 3.A If the item has a definite used flex basis…
		if !contentBasis {
			child.flex().flexBaseSize = flexBasis
			if main == "width" {
				child.flex().mainOuterExtra = child.BorderLeftWidth + child.BorderRightWidth +
					child.PaddingLeft + child.PaddingRight
				if child.MarginLeft != Auto {
					child.flex().mainOuterExtra += child.MarginLeft
				}
				if child.MarginRight != Auto {
					child.flex().mainOuterExtra += child.MarginRight
				}
			} else {
				child.flex().mainOuterExtra = child.BorderTopWidth + child.BorderBottomWidth +
					child.PaddingTop + child.PaddingBottom
				if child.MarginTop != Auto {
					child.flex().mainOuterExtra += child.MarginTop
				}
				if child.MarginBottom != Auto {
					child.flex().mainOuterExtra += child.MarginBottom
				}
			}
		} else {
			// TODO: 3.B If the flex item has an intrinsic aspect ratio…
			// TODO: 3.C If the used flex basis is 'content'…
			// TODO: 3.D Otherwise, if the used flex basis is 'content'…

			// 3.E Otherwise…
			newChild := child.Copy()
			newChild.Style = child.Style.Copy()
			if main == "width" {
				// … the item’s min and max main sizes are ignored.
				newChild.Style.Set("min_width", Dimension{Value: 0, Unit: "px"})
				newChild.Style.Set("max_width", Dimension{Value: math.Inf(1), Unit: "px"})

				child.flex().flexBaseSize = maxContentWidth(lc, newChild, false)
				child.flex().mainOuterExtra = maxContentWidth(lc, child, true) - child.flex().flexBaseSize
			} else {
				// … the item’s min and max main sizes are ignored.
				newChild.Style.Set("min_height", Dimension{Value: 0, Unit: "px"})
				newChild.Style.Set("max_height", Dimension{Value: math.Inf(1), Unit: "px"})

				newChild.Width = math.Inf(1)
				result := blockLevelLayout(lc, newChild, bottomSpace, childSkipStack, parentBox,
					pageIsEmpty, absoluteBoxes, fixedBoxes, nil, nil, nil, false, noMaxLines)
				if newChild = result.box; newChild != nil {
					// As flex items margins never collapse (with other flex items or
					// with the flex container), we can add the adjoining margins to the
					// child height.
					newChild.Height += collapseMargin(result.adjoiningMargins)
					child.flex().flexBaseSize = newChild.Height
					child.flex().mainOuterExtra = newChild.MarginHeight() - newChild.Height
				} else {
					child.flex().flexBaseSize, child.flex().mainOuterExtra = 0, 0
				}
			}
		}

		if main == "width" {
			positionX += child.flex().flexBaseSize + child.flex().mainOuterExtra
		} else {
			positionY += child.flex().flexBaseSize + child.flex().mainOuterExtra
		}

		minSize := *child.usedField("min_" + main)
		maxSize := *child.usedField("max_" + main)
		child.flex().hypotheticalMainSize = max(minSize, min(child.flex().flexBaseSize, maxSize))

		// Skip stack is only for the first child.
		childSkipStack = nil
	}

	// 4 Determine the main size of the flex container using the rules of the formatting
	// context in which it participates.
	originalBoxHeight := box.Height
	if main == "width" {
		blockLevelWidth(box, containingBlock)
	} else {
		if box.Height == Auto {
			box.Height = 0
			i := 0
			for _, child := range children {
				if !child.IsFlexItem {
					continue
				}
				box.Height += child.flex().hypotheticalMainSize + child.flex().mainOuterExtra
				if i > 0 {
					box.Height += mainGap
				}
				i++
			}
		}
		box.Height = max(box.MinHeight, min(box.Height, box.MaxHeight))
	}

	// 5 If the flex container is single-line, collect all the flex items into a single
	// flex line.
	var flexLines []*flexLine
	var line []flexLineItem
	lineSize := 0.0
	mainSize := *box.usedField(main)
	wrap := !isStr(box.Style.Get("flex_wrap"), "nowrap")
	for j, child := range children {
		i := j + skipIndex
		if !child.IsFlexItem {
			continue
		}
		lineSize += child.flex().hypotheticalMainSize + child.flex().mainOuterExtra
		if i > skipIndex {
			lineSize += mainGap
		}
		if wrap && lineSize > mainSize {
			if len(line) > 0 {
				flexLines = append(flexLines, &flexLine{items: line})
				line = []flexLineItem{{i, child}}
				lineSize = child.flex().hypotheticalMainSize + child.flex().mainOuterExtra
			} else {
				line = append(line, flexLineItem{i, child})
				flexLines = append(flexLines, &flexLine{items: line})
				line = nil
				lineSize = 0
			}
		} else {
			line = append(line, flexLineItem{i, child})
		}
	}
	if len(line) > 0 {
		flexLines = append(flexLines, &flexLine{items: line})
	}

	// TODO: Handle *-reverse using the terminology from the specification.
	if isStr(box.Style.Get("flex_wrap"), "wrap-reverse") {
		slices.Reverse(flexLines)
	}
	if reverse {
		for _, line := range flexLines {
			slices.Reverse(line.items)
		}
	}

	// 6 Resolve the flexible lengths of all the flex items to find their used main size.
	availableMainSpace = *box.usedField(main)
	for _, line := range flexLines {
		// 9.7.1 Determine the used flex factor.
		hypotheticalMainSize := 0.0
		for _, item := range line.items {
			hypotheticalMainSize += item.child.flex().hypotheticalMainSize + item.child.flex().mainOuterExtra
		}
		grow := hypotheticalMainSize < availableMainSpace

		// 9.7.3 Size inflexible items.
		for _, item := range line.items {
			child := item.child
			var flexCondition bool
			if grow {
				child.flex().flexFactor = num(child.Style.Get("flex_grow"))
				flexCondition = child.flex().flexBaseSize > child.flex().hypotheticalMainSize
			} else {
				child.flex().flexFactor = num(child.Style.Get("flex_shrink"))
				flexCondition = child.flex().flexBaseSize < child.flex().hypotheticalMainSize
			}
			if child.flex().flexFactor == 0 || flexCondition {
				child.flex().targetMainSize = child.flex().hypotheticalMainSize
				child.flex().frozen = true
			} else {
				child.flex().frozen = false
			}
		}

		// 9.7.4 Calculate initial free space.
		initialFreeSpace := availableMainSpace
		for i, item := range line.items {
			child := item.child
			if child.flex().frozen {
				initialFreeSpace -= child.flex().targetMainSize + child.flex().mainOuterExtra
			} else {
				initialFreeSpace -= child.flex().flexBaseSize + child.flex().mainOuterExtra
			}
			if i > 0 {
				initialFreeSpace -= mainGap
			}
		}

		// 9.7.5.a Check for flexible items.
		allFrozen := func() bool {
			for _, item := range line.items {
				if !item.child.flex().frozen {
					return false
				}
			}
			return true
		}
		// Port: each pass of the loop freezes at least one item. The port
		// stops after one pass more than the number of items, and freezes
		// them all.
		for pass := 0; !allFrozen(); pass++ {
			lc.spend(1)
			if pass > len(line.items) {
				for _, item := range line.items {
					item.child.flex().frozen = true
				}
				break
			}
			unfrozenFactorSum := 0.0
			remainingFreeSpace := availableMainSpace

			// 9.7.5.b Calculate the remaining free space.
			for i, item := range line.items {
				child := item.child
				if child.flex().frozen {
					remainingFreeSpace -= child.flex().targetMainSize + child.flex().mainOuterExtra
				} else {
					remainingFreeSpace -= child.flex().flexBaseSize + child.flex().mainOuterExtra
					unfrozenFactorSum += child.flex().flexFactor
				}
				if i > 0 {
					remainingFreeSpace -= mainGap
				}
			}

			if unfrozenFactorSum < 1 {
				initialFreeSpace *= unfrozenFactorSum
			}

			if math.IsInf(initialFreeSpace, 1) {
				initialFreeSpace = math.MaxInt64
			}
			if math.IsInf(remainingFreeSpace, 1) {
				remainingFreeSpace = math.MaxInt64
			}

			initialMagnitude := flexMagnitude(initialFreeSpace)
			remainingMagnitude := flexMagnitude(remainingFreeSpace)
			if initialMagnitude < remainingMagnitude {
				remainingFreeSpace = initialFreeSpace
			}

			// 9.7.5.c Distribute free space proportional to the flex factors.
			if remainingFreeSpace == 0 {
				// If the remaining free space is zero: "Do nothing", but we at least set
				// the flex_base_size as target_main_size for next step.
				for _, item := range line.items {
					if !item.child.flex().frozen {
						item.child.flex().targetMainSize = item.child.flex().flexBaseSize
					}
				}
			} else {
				scaledFlexShrinkFactorsSum := 0.0
				flexGrowFactorsSum := 0.0
				for _, item := range line.items {
					child := item.child
					if !child.flex().frozen {
						child.flex().scaledFlexShrinkFactor = child.flex().flexBaseSize * num(child.Style.Get("flex_shrink"))
						scaledFlexShrinkFactorsSum += child.flex().scaledFlexShrinkFactor
						flexGrowFactorsSum += num(child.Style.Get("flex_grow"))
					}
				}
				for _, item := range line.items {
					child := item.child
					if child.flex().frozen {
						continue
					}
					if grow {
						// If using the flex grow factor…
						if flexGrowFactorsSum == 0 {
							// Port: upstream divides by zero.
							child.flex().targetMainSize = child.flex().flexBaseSize
						} else {
							ratio := num(child.Style.Get("flex_grow")) / flexGrowFactorsSum
							child.flex().targetMainSize = child.flex().flexBaseSize + remainingFreeSpace*ratio
						}
					} else {
						// If using the flex shrink factor…
						if scaledFlexShrinkFactorsSum == 0 {
							child.flex().targetMainSize = child.flex().flexBaseSize
						} else {
							ratio := child.flex().scaledFlexShrinkFactor / scaledFlexShrinkFactorsSum
							child.flex().targetMainSize = child.flex().flexBaseSize + remainingFreeSpace*ratio
						}
					}
					child.flex().targetMainSize = minMax(child, child.flex().targetMainSize)
				}
			}

			// 9.7.5.d Fix min/max violations.
			for _, item := range line.items {
				child := item.child
				child.flex().adjustment = 0
				if !child.flex().frozen {
					minSize := *child.usedField("min_" + main)
					maxSize := *child.usedField("max_" + main)
					minSize = max(minSize, min(child.flex().targetMainSize, maxSize))
					if child.flex().targetMainSize < minSize {
						child.flex().adjustment = minSize - child.flex().targetMainSize
						child.flex().targetMainSize = minSize
					}
				}
			}

			// 9.7.5.e Freeze over-flexed items.
			adjustments := 0.0
			for _, item := range line.items {
				adjustments += item.child.flex().adjustment
			}
			for _, item := range line.items {
				child := item.child
				switch {
				case adjustments == 0:
					// Zero: Freeze all items.
					child.flex().frozen = true
				case adjustments > 0 && child.flex().adjustment > 0:
					// Positive: Freeze all the items with min violations.
					child.flex().frozen = true
				case adjustments < 0 && child.flex().adjustment < 0:
					// Negative: Freeze all the items with max violations.
					child.flex().frozen = true
				}
			}
		}

		// 9.7.6 Set each item’s used main size to its target main size.
		for _, item := range line.items {
			*item.child.usedField(main) = item.child.flex().targetMainSize
		}
	}

	// 7 Determine the hypothetical cross size of each item.
	// TODO: Handle breaks.
	var newFlexLines []*flexLine
	childSkipStack = skip
	for _, line := range flexLines {
		newFlexLine := &flexLine{}
		for _, item := range line.items {
			child := item.child
			// TODO: Fix this value, see test_flex_item_auto_margin_cross.
			if child.MarginTop == Auto {
				child.MarginTop = 0
			}
			if child.MarginBottom == Auto {
				child.MarginBottom = 0
			}
			// TODO: Find another way than calling block_level_layout_switch.
			newChild := child.Copy()
			result := blockLevelLayoutSwitch(lc, newChild, math.Inf(-1), childSkipStack, parentBox,
				pageIsEmpty, absoluteBoxes, fixedBoxes, []float64{}, nil, nil, discard, noMaxLines)
			newChild = result.box
			laidOut := newChild != nil
			if !laidOut {
				// Port: upstream reads the attributes of the None that the
				// layout can return. The port reads an empty copy of the
				// item.
				newChild = child.Copy()
				newChild.Children = nil
				newChild.Width, newChild.Height = autoZero(newChild.Width), 0
			}
			child.flex().baseline = 0
			if baseline, ok := findInFlowBaseline(newChild, false); ok {
				child.flex().baseline = baseline
			}
			if cross == "height" {
				child.Height = newChild.Height
				// As flex items margins never collapse (with other flex items or
				// with the flex container), we can add the adjoining margins to the
				// child height.
				child.Height += collapseMargin(result.adjoiningMargins)
			} else {
				if child.Width == Auto {
					minWidth := minContentWidth(lc, child, false)
					maxWidth := maxContentWidth(lc, child, false)
					child.Width = min(max(minWidth, newChild.Width), maxWidth)
				} else {
					child.Width = newChild.Width
				}
			}
			if laidOut {
				removePlaceholders(lc, newChild.Children, absoluteBoxes, fixedBoxes)
			}

			newFlexLine.items = append(newFlexLine.items, item)

			// Skip stack is only for the first child.
			childSkipStack = nil
		}

		if len(newFlexLine.items) > 0 {
			newFlexLines = append(newFlexLines, newFlexLine)
		}
	}
	flexLines = newFlexLines

	// 8 Calculate the cross size of each flex line.
	if crossSize := *box.usedField(cross); len(flexLines) == 1 && crossSize != Auto {
		// If the flex container is single-line…
		flexLines[0].crossSize = crossSize
	} else {
		// Otherwise, for each flex line…
		// 8.1 Collect all the flex items whose inline-axis is parallel to the main-axis…
		for _, line := range flexLines {
			var collectedItems, notCollectedItems []*Box
			for _, item := range line.items {
				child := item.child
				alignSelf := child.Style.Get("align_self")
				collect := strings.HasPrefix(flexDirection, "row") &&
					tupleHas(alignSelf, "baseline") &&
					child.MarginTop != Auto && child.MarginBottom != Auto
				if collect {
					collectedItems = append(collectedItems, child)
				} else {
					notCollectedItems = append(notCollectedItems, child)
				}
			}
			crossStartDistance, crossEndDistance := 0.0, 0.0
			for _, child := range collectedItems {
				baseline := child.flex().baseline - child.PositionY
				crossStartDistance = max(crossStartDistance, baseline)
				crossEndDistance = max(crossEndDistance, child.MarginHeight()-baseline)
			}
			collectedCrossSize := crossStartDistance + crossEndDistance
			nonCollectedCrossSize := 0.0
			// 8.2 Find the largest outer hypothetical cross size.
			if len(notCollectedItems) > 0 {
				nonCollectedCrossSize = math.Inf(-1)
				for _, child := range notCollectedItems {
					var childCrossSize float64
					if cross == "height" {
						childCrossSize = child.BorderHeight()
						if child.MarginTop != Auto {
							childCrossSize += child.MarginTop
						}
						if child.MarginBottom != Auto {
							childCrossSize += child.MarginBottom
						}
					} else {
						childCrossSize = child.BorderWidth()
						if child.MarginLeft != Auto {
							childCrossSize += child.MarginLeft
						}
						if child.MarginRight != Auto {
							childCrossSize += child.MarginRight
						}
					}
					nonCollectedCrossSize = max(childCrossSize, nonCollectedCrossSize)
				}
			}
			// 8.3 Set the used cross-size of the flex line.
			line.crossSize = max(collectedCrossSize, nonCollectedCrossSize)
		}
	}

	// 8.3 If the flex container is single-line…
	if len(flexLines) == 1 {
		line := flexLines[0]
		minCrossSize := *box.usedField("min_" + cross)
		if minCrossSize == Auto {
			minCrossSize = math.Inf(-1)
		}
		maxCrossSize := *box.usedField("max_" + cross)
		if maxCrossSize == Auto {
			maxCrossSize = math.Inf(1)
		}
		line.crossSize = max(minCrossSize, min(line.crossSize, maxCrossSize))
	}

	// 9 Handle 'align-content: stretch'.
	alignContent := box.Style.Get("align_content")
	if tupleHas(alignContent, "normal") {
		alignContent = Tuple{String("stretch")}
	}
	if tupleHas(alignContent, "stretch") {
		definiteCrossSize, hasDefiniteCrossSize := 0.0, false
		if cross == "height" && box.Height != Auto {
			definiteCrossSize, hasDefiniteCrossSize = box.Height, true
		} else if cross == "width" {
			if box.Is(KindFlex) {
				if box.Width == Auto {
					definiteCrossSize, hasDefiniteCrossSize = availableCrossSpace, true
				} else {
					definiteCrossSize, hasDefiniteCrossSize = box.Width, true
				}
			}
		}
		if hasDefiniteCrossSize {
			extraCrossSize := definiteCrossSize
			for _, line := range flexLines {
				extraCrossSize -= line.crossSize
			}
			extraCrossSize -= float64(len(flexLines)-1) * crossGap
			if extraCrossSize != 0 {
				for _, line := range flexLines {
					line.crossSize += extraCrossSize / float64(len(flexLines))
				}
			}
		}
	}

	// TODO: 10 Collapse 'visibility: collapse' items.

	// 11 Determine the used cross size of each flex item.
	alignItems := box.Style.Get("align_items")
	if tupleHas(alignItems, "normal") {
		alignItems = Tuple{String("stretch")}
	}
	for _, line := range flexLines {
		for _, item := range line.items {
			child := item.child
			alignSelf := child.Style.Get("align_self")
			if tupleHas(alignSelf, "normal") {
				alignSelf = Tuple{String("stretch")}
			} else if tupleHas(alignSelf, "auto") {
				alignSelf = alignItems
			}
			if tupleHas(alignSelf, "stretch") && isStr(child.Style.Get(cross), "auto") {
				var crossMargins [2]Value
				if cross == "height" {
					crossMargins = [2]Value{child.Style.Get("margin_top"), child.Style.Get("margin_bottom")}
				} else {
					crossMargins = [2]Value{child.Style.Get("margin_left"), child.Style.Get("margin_right")}
				}
				if !isStr(crossMargins[0], "auto") && !isStr(crossMargins[1], "auto") {
					crossSize := line.crossSize
					if cross == "height" {
						crossSize -= child.MarginTop + child.MarginBottom +
							child.PaddingTop + child.PaddingBottom +
							child.BorderTopWidth +
							child.BorderBottomWidth
					} else {
						crossSize -= child.MarginLeft + child.MarginRight +
							child.PaddingLeft + child.PaddingRight +
							child.BorderLeftWidth +
							child.BorderRightWidth
					}
					*child.usedField(cross) = crossSize
				}
			}
			// else: Cross size has been set by step 7.
		}
	}

	// 12 Distribute any remaining free space.
	originalPositionMain := box.ContentBoxY()
	if main == "width" {
		originalPositionMain = box.ContentBoxX()
	}
	justifyContent := box.Style.Get("justify_content")
	if tupleHas(justifyContent, "normal") {
		justifyContent = Tuple{String("flex-start")}
	}
	if reverse {
		switch {
		case tupleHas(justifyContent, "flex-start"):
			justifyContent = Tuple{String("flex-end")}
		case tupleHas(justifyContent, "flex-end"):
			justifyContent = Tuple{String("flex-start")}
		case tupleHas(justifyContent, "start"):
			justifyContent = Tuple{String("end")}
		case tupleHas(justifyContent, "end"):
			justifyContent = Tuple{String("start")}
		}
	}
	widthRTL := isStr(box.Style.Get("direction"), "rtl") && main == "width"
	for _, line := range flexLines {
		positionMain := originalPositionMain
		var freeSpace float64
		if main == "width" {
			freeSpace = box.Width
			for _, item := range line.items {
				child := item.child
				freeSpace -= child.BorderWidth()
				if child.MarginLeft != Auto {
					freeSpace -= child.MarginLeft
				}
				if child.MarginRight != Auto {
					freeSpace -= child.MarginRight
				}
			}
		} else {
			freeSpace = box.Height
			for _, item := range line.items {
				child := item.child
				freeSpace -= child.BorderHeight()
				if child.MarginTop != Auto {
					freeSpace -= child.MarginTop
				}
				if child.MarginBottom != Auto {
					freeSpace -= child.MarginBottom
				}
			}
		}
		freeSpace -= float64(len(line.items)-1) * mainGap

		// 12.1 If the remaining free space is positive…
		margins := 0
		for _, item := range line.items {
			child := item.child
			if main == "width" {
				if child.MarginLeft == Auto {
					margins++
				}
				if child.MarginRight == Auto {
					margins++
				}
			} else {
				if child.MarginTop == Auto {
					margins++
				}
				if child.MarginBottom == Auto {
					margins++
				}
			}
		}
		if margins > 0 {
			freeSpace /= float64(margins)
			for _, item := range line.items {
				child := item.child
				if main == "width" {
					if child.MarginLeft == Auto {
						child.MarginLeft = freeSpace
					}
					if child.MarginRight == Auto {
						child.MarginRight = freeSpace
					}
				} else {
					if child.MarginTop == Auto {
						child.MarginTop = freeSpace
					}
					if child.MarginBottom == Auto {
						child.MarginBottom = freeSpace
					}
				}
			}
			freeSpace = 0
		}

		if widthRTL {
			freeSpace *= -1
		}

		// 12.2 Align the items along the main-axis per justify-content.
		lineLength := float64(len(line.items))
		switch {
		case tupleHas(justifyContent, "end") || tupleHas(justifyContent, "flex-end") || tupleHas(justifyContent, "right"):
			positionMain += freeSpace
		case tupleHas(justifyContent, "center"):
			positionMain += freeSpace / 2
		case tupleHas(justifyContent, "space-around"):
			positionMain += freeSpace / lineLength / 2
		case tupleHas(justifyContent, "space-evenly"):
			positionMain += freeSpace / (lineLength + 1)
		}

		growths := 0.0
		for _, child := range children {
			growths += num(child.Style.Get("flex_grow"))
		}
		for i, item := range line.items {
			child := item.child
			if i > 0 {
				if widthRTL {
					positionMain -= mainGap
				} else {
					positionMain += mainGap
				}
			}
			if main == "width" {
				child.PositionX = positionMain
				if tupleHas(justifyContent, "stretch") && growths != 0 {
					child.Width += freeSpace * num(child.Style.Get("flex_grow")) / growths
				}
			} else {
				child.PositionY = positionMain
			}
			marginMain := child.MarginHeight()
			if main == "width" {
				marginMain = child.MarginWidth()
			}
			if widthRTL {
				positionMain -= marginMain
			} else {
				positionMain += marginMain
			}
			switch {
			case tupleHas(justifyContent, "space-around"):
				positionMain += freeSpace / lineLength
			case tupleHas(justifyContent, "space-between"):
				if len(line.items) > 1 {
					positionMain += freeSpace / (lineLength - 1)
				}
			case tupleHas(justifyContent, "space-evenly"):
				positionMain += freeSpace / (lineLength + 1)
			}
		}
	}

	// 13 Resolve cross-axis auto margins.
	if cross == "width" {
		// Make sure width/margins are no longer "auto", as we did not do it above in
		// step 4.
		blockLevelWidth(box, containingBlock)
	}
	positionCross := box.ContentBoxX()
	if cross == "height" {
		positionCross = box.ContentBoxY()
	}
	for _, line := range flexLines {
		line.lowerBaseline = math.Inf(-1)
		// TODO: Don't duplicate this loop.
		for _, item := range line.items {
			child := item.child
			alignSelf := child.Style.Get("align_self")
			if tupleHas(alignSelf, "auto") {
				alignSelf = alignItems
			}
			if tupleHas(alignSelf, "baseline") && main == "width" {
				// TODO: handle vertical text.
				child.Baseline = child.flex().baseline - positionCross
				line.lowerBaseline = max(line.lowerBaseline, child.Baseline)
			}
		}
		if math.IsInf(line.lowerBaseline, -1) {
			line.lowerBaseline = 0
			if len(line.items) > 0 {
				line.lowerBaseline = line.items[0].child.flex().baseline
			}
		}
		for _, item := range line.items {
			child := item.child
			var crossMargins [2]Value
			if cross == "height" {
				crossMargins = [2]Value{child.Style.Get("margin_top"), child.Style.Get("margin_bottom")}
			} else {
				crossMargins = [2]Value{child.Style.Get("margin_left"), child.Style.Get("margin_right")}
			}
			autoMargins := 0
			for _, margin := range crossMargins {
				if isStr(margin, "auto") {
					autoMargins++
				}
			}
			// If a flex item has auto cross-axis margins…
			if autoMargins > 0 {
				extraCross := line.crossSize
				if cross == "height" {
					extraCross -= child.BorderHeight()
					if child.MarginTop != Auto {
						extraCross -= child.MarginTop
					}
					if child.MarginBottom != Auto {
						extraCross -= child.MarginBottom
					}
				} else {
					extraCross -= child.BorderWidth()
					if child.MarginLeft != Auto {
						extraCross -= child.MarginLeft
					}
					if child.MarginRight != Auto {
						extraCross -= child.MarginRight
					}
				}
				if extraCross > 0 {
					// If its outer cross size is less than the cross size…
					extraCross /= float64(autoMargins)
					if cross == "height" {
						if isStr(child.Style.Get("margin_top"), "auto") {
							child.MarginTop = extraCross
						}
						if isStr(child.Style.Get("margin_bottom"), "auto") {
							child.MarginBottom = extraCross
						}
					} else {
						if isStr(child.Style.Get("margin_left"), "auto") {
							child.MarginLeft = extraCross
						}
						if isStr(child.Style.Get("margin_right"), "auto") {
							child.MarginRight = extraCross
						}
					}
				} else {
					// Otherwise…
					if cross == "height" {
						if child.MarginTop == Auto {
							child.MarginTop = 0
						}
						child.MarginBottom = extraCross
					} else {
						if child.MarginLeft == Auto {
							child.MarginLeft = 0
						}
						child.MarginRight = extraCross
					}
				}
			} else {
				// 14 Align all flex items along the cross-axis.
				alignSelf := child.Style.Get("align_self")
				if tupleHas(alignSelf, "normal") {
					alignSelf = Tuple{String("stretch")}
				} else if tupleHas(alignSelf, "auto") {
					alignSelf = alignItems
				}
				*flexPositionField(child, cross) = positionCross
				switch {
				case tupleHas(alignSelf, "end") || tupleHas(alignSelf, "self-end") || tupleHas(alignSelf, "flex-end"):
					if cross == "height" {
						child.PositionY += line.crossSize - child.MarginHeight()
					} else {
						child.PositionX += line.crossSize - child.MarginWidth()
					}
				case tupleHas(alignSelf, "center"):
					if cross == "height" {
						child.PositionY += (line.crossSize - child.MarginHeight()) / 2
					} else {
						child.PositionX += (line.crossSize - child.MarginWidth()) / 2
					}
				case tupleHas(alignSelf, "baseline"):
					if cross == "height" {
						child.PositionY += line.lowerBaseline - child.Baseline
					}
					// Else: TODO: Handle vertical text.
				case tupleHas(alignSelf, "stretch"):
					// Port: upstream computes the margins of the cross
					// axis of a stretched item here, and does not use them.
				}
			}
		}
		positionCross += line.crossSize
	}

	// 15 Determine the flex container’s used cross size.
	// TODO: Use the updated algorithm.
	if *box.usedField(cross) == Auto {
		// Otherwise, use the sum of the flex lines' cross sizes…
		// TODO: Handle min-max.
		// TODO: What about align-content here?
		crossSize := 0.0
		for _, line := range flexLines {
			crossSize += line.crossSize
		}
		crossSize += float64(len(flexLines)-1) * crossGap
		*box.usedField(cross) = crossSize
	}

	if len(flexLines) > 1 {
		// 15 If the cross size property is a definite size, use that…
		extraCrossSize := *box.usedField(cross)
		for _, line := range flexLines {
			extraCrossSize -= line.crossSize
		}
		extraCrossSize -= float64(len(flexLines)-1) * crossGap
		// 16 Align all flex lines per align-content.
		crossTranslate := 0.0
		linesLength := float64(len(flexLines))
		// currentValue keeps its value from one line to the next one, as
		// the variable current_value of upstream does.
		currentValue := 0.0
		for i, line := range flexLines {
			var flexItems []*Box
			for _, item := range line.items {
				if item.child.IsFlexItem {
					flexItems = append(flexItems, item.child)
				}
			}
			if i > 0 {
				crossTranslate += crossGap
			}
			for _, child := range flexItems {
				currentValue = *flexPositionField(child, cross) + crossTranslate
				*flexPositionField(child, cross) = currentValue
			}
			if extraCrossSize == 0 {
				continue
			}
			for _, child := range flexItems {
				switch {
				case tupleHas(alignContent, "flex-end") || tupleHas(alignContent, "end"):
					*flexPositionField(child, cross) = currentValue + extraCrossSize
				case tupleHas(alignContent, "center"):
					*flexPositionField(child, cross) = currentValue + extraCrossSize/2
				case tupleHas(alignContent, "space-around"):
					*flexPositionField(child, cross) = currentValue + extraCrossSize/linesLength/2
				case tupleHas(alignContent, "space-evenly"):
					*flexPositionField(child, cross) = currentValue + extraCrossSize/(linesLength+1)
				}
			}
			switch {
			case tupleHas(alignContent, "space-between"):
				crossTranslate += extraCrossSize / (linesLength - 1)
			case tupleHas(alignContent, "space-around"):
				crossTranslate += extraCrossSize / linesLength
			case tupleHas(alignContent, "space-evenly"):
				crossTranslate += extraCrossSize / (linesLength + 1)
			}
		}
	}

	// Now we are no longer in the flex algorithm.
	var outOfFlow []*Box
	for _, child := range children {
		if child.IsAbsolutelyPositioned() {
			outOfFlow = append(outOfFlow, child)
		}
	}
	box = box.CopyWithChildren(outOfFlow)
	childSkipStack = skip
lines:
	for _, line := range flexLines {
		for _, item := range line.items {
			index, child := item.index, item.child
			if child.IsFlexItem {
				// TODO: Don't use block_level_layout_switch.
				result := blockLevelLayoutSwitch(lc, child, bottomSpace, childSkipStack, box,
					pageIsEmpty, absoluteBoxes, fixedBoxes, []float64{}, nil, nil, discard, noMaxLines)
				newChild, childResumeAt := result.box, result.resumeAt
				nextPage = result.nextPage
				if newChild == nil {
					resumeIndex := 0
					if len(resumeAt) > 0 {
						resumeIndex, _ = resumeAt.item()
						resumeIndex--
					}
					resumeAt = skipStack{resumeIndex + index: nil}
				} else {
					pageIsEmpty = false
					box.Children = append(box.Children, newChild)
					positionY = newChild.PositionY + newChild.BorderHeight()
					if childResumeAt != nil {
						firstLevelSkip := 0
						if len(resumeAt) > 0 {
							firstLevelSkip, _ = resumeAt.item()
						}
						resumeAt = skipStack{firstLevelSkip + index: childResumeAt}
					}
				}
				if len(resumeAt) > 0 {
					break lines
				}
			}

			// Skip stack is only for the first child.
			childSkipStack = nil
		}
	}

	if originalBoxHeight != Auto {
		// Don’t resume if flex items overflow flex container bottom.
		boxBottom := box.PositionY + box.BorderHeight()
		if overflows(boxBottom, positionY) {
			resumeAt = nil
		}
	}

	if isStr(box.Style.Get("position"), "relative") {
		// New containing block, resolve the layout of the absolute descendants.
		for _, absoluteBox := range *absoluteBoxes {
			absoluteLayout(lc, absoluteBox, box, fixedBoxes, bottomSpace, nil)
		}
	}

	for _, child := range box.Children {
		relativePositioning(child, sizeBlock(box.Width, box.Height))
	}

	// TODO: Use real algorithm, see https://www.w3.org/TR/css-flexbox-1/#flex-baselines.
	if box.Kind == KindInlineFlex {
		if main == "width" { // and main text direction is horizontal
			box.Baseline = 0
			if len(flexLines) > 0 {
				box.Baseline = flexLines[0].lowerBaseline
			}
		} else {
			// The baseline of the first in-flow child, or 0.
			box.Baseline = 0
			if inFlow := box.InFlowChildren(); len(inFlow) > 0 {
				if baseline, ok := findInFlowBaseline(inFlow[0], false); ok {
					box.Baseline = baseline
				}
			}
		}
	}

	box.RemoveDecoration(false, len(resumeAt) > 0 && !discard)

	lc.finishFlexFormattingContext()

	return blockLayoutResult{
		box:      box,
		resumeAt: resumeAt,
		nextPage: nextPage,
		maxLines: noMaxLines,
	}
}
