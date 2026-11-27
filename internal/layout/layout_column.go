// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/column.py: the layout for columns.

package layout

import (
	"math"
	"slices"
)

// maxColumns bounds the number of columns of a row of columns.
//
// Port: upstream has no bound. A column width of a fraction of a pixel
// makes millions of columns.
const maxColumns = 1000

// maxColumnLoops bounds the loops that search the height of the columns.
//
// Port: upstream adds at least one pixel to the height at each loop, until
// the content fits. With an infinite space at the bottom of the page, it
// can loop for a time without bound.
const maxColumnLoops = 1000

// columnsItem is an item of the list columns_and_blocks of upstream: the
// index of its first child, and the children that go in columns, or the
// block that spans all the columns.
type columnsItem struct {
	index    int
	children []*Box
	block    *Box
}

// columnsLayout is columns_layout in weasyprint/layout/column.py: it lays
// out a multi-column box.
//
// Port: upstream has no -ivy-page-top-space. pageTop is set when the box
// starts at the top of a page where the space before its first block is
// left out. The first block of each column loses the space before it as
// Word does, see layoutContext.startColumn.
func columnsLayout(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, containingBlock *Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins []float64, firstLetterStyle, firstLineStyle *Style, pageTop bool) blockLayoutResult {
	lc.enter()
	defer lc.leave()
	if absoluteBoxes == nil {
		absoluteBoxes = new([]*Box)
	}
	if fixedBoxes == nil {
		fixedBoxes = new([]*Box)
	}

	style := box.Style
	widthValue := style.Get("column_width")
	countValue := style.Get("column_count")
	height := style.Get("height")
	originalBottomSpace := bottomSpace
	lc.inColumn = true
	// Port: upstream has no -ivy-wrap-flow. The columns of the box see
	// only the floats of the columns of the box before them.
	defer func(state columnFloatState) { lc.columnFloats = state }(lc.columnFloats)
	lc.columnFloats = columnFloatState{}

	if isStr(style.Get("position"), "relative") {
		// New containing block, use a new absolute list
		absoluteBoxes = new([]*Box)
	}

	box = box.CopyWithChildren(box.Children)
	box.PositionY += collapseMargin(adjoiningMargins)

	// Set height if defined
	heightDefined := false
	if d, ok := height.(Dimension); ok && d.Unit != "%" {
		// Port: upstream asserts that the unit is px.
		heightDefined = true
		emptySpace := lc.pageBottom - box.ContentBoxY() - d.Value
		bottomSpace = max(bottomSpace, emptySpace)
	}

	// TODO: the columns container width can be unknown if the containing
	// block needs the size of this block to know its own size
	blockLevelWidth(box, containingBlock)

	var gap float64
	if isStr(style.Get("column_gap"), "normal") {
		// 1em because in column context
		gap = num(style.Get("font_size"))
	} else {
		gap, _ = percentageFloat(style.Get("column_gap"), box.Style, box.Width)
	}

	// Define the number of columns and their widths
	width, widthOK := toFloat(widthValue)
	countFloat, countOK := toFloat(countValue)
	count := 1
	if countOK {
		// Port: upstream reads a count that the validation keeps positive.
		count = int(min(max(countFloat, 1), maxColumns))
	}
	// columnsFor is floor((box.width + gap) / (width + gap)).
	//
	// Port: upstream divides by zero for a null width and gap. The port
	// takes one column, and at most maxColumns.
	columnsFor := func() int {
		if width+gap <= 0 {
			return 1
		}
		n := math.Floor((box.Width + gap) / (width + gap))
		if math.IsNaN(n) {
			return 1
		}
		return int(min(max(n, 1), maxColumns))
	}
	switch {
	case !widthOK && countOK:
		width = max(0, box.Width-float64(count-1)*gap) / float64(count)
	case widthOK && !countOK:
		count = columnsFor()
		width = (box.Width+gap)/float64(count) - gap
	case widthOK && countOK:
		// overconstrained, with width != 'auto' and count != 'auto'
		count = max(1, min(count, columnsFor()))
		width = (box.Width+gap)/float64(count) - gap
	default:
		// Port: upstream fails when both are auto. No caller lays out
		// such a box in columns. The port makes one column.
		count = 1
		width = box.Width
	}

	// Handle column-span property with the following structure:
	// columns_and_blocks = [
	//     [column_child_1, column_child_2],
	//     spanning_block,
	//     …
	// ]
	var columnsAndBlocks []columnsItem
	var columnChildren []*Box
	skipIndex := 0
	if len(skip) > 0 {
		skipIndex, _ = skip.item()
	}
	// Port: upstream slices the children. A skip past them gives no child.
	skipIndex = min(max(skipIndex, 0), len(box.Children))
	i := skipIndex
	for ; i < len(box.Children); i++ {
		child := box.Children[i]
		if isStr(child.Style.Get("column_span"), "all") {
			if len(columnChildren) > 0 {
				columnsAndBlocks = append(columnsAndBlocks,
					columnsItem{index: i - len(columnChildren), children: columnChildren})
			}
			columnsAndBlocks = append(columnsAndBlocks, columnsItem{index: i, block: child.Copy()})
			columnChildren = nil
			continue
		}
		columnChildren = append(columnChildren, child.Copy())
	}
	if len(columnChildren) > 0 {
		columnsAndBlocks = append(columnsAndBlocks,
			columnsItem{index: i - len(columnChildren), children: columnChildren})
	}

	if len(skip) > 0 {
		skip = skip[skipIndex]
	}

	// Port: upstream leaves next_page unbound for a box whose children all
	// come before the skip stack.
	next := anyNextPage()
	if len(box.Children) == 0 {
		next = anyNextPage()
		skip = nil
	}

	// Find height and balance.
	//
	// The current algorithm starts from the total available height, to
	// check whether the whole content can fit. If it doesn’t fit, we keep
	// the partial rendering. If it fits, we try to balance the columns
	// starting from the ideal height (the total height divided by the
	// number of columns). We then iterate until the last column is not the
	// highest one. At the end of each loop, we add the minimal height
	// needed to make one direct child at the top of one column go to the
	// end of the previous column.
	//
	// We rely on a real rendering for each loop, and with a stupid
	// algorithm like this it can last minutes…

	adjoiningMargins = nil
	currentPositionY := box.ContentBoxY()
	var newChildren []*Box
	var columnSkipStack skipStack
	// Port: upstream sets last_loop before it leaves the loop on the
	// items, so the test of last_loop in the loop on the heights is always
	// false. The port has no last_loop.
	breakPage := false
	footnoteAreaHeights := []float64{columnFootnoteAreaHeight(lc)}
	lastFootnotesHeight := 0.0
	index := 0
	for _, item := range columnsAndBlocks {
		index = item.index
		if item.block != nil {
			// We have a spanning block, we display it like other blocks
			block := item.block
			resolvePercentages(block, containingBlock)
			block.PositionX = box.ContentBoxX()
			block.PositionY = currentPositionY
			result := blockLevelLayout(lc, block, originalBottomSpace, skip, containingBlock,
				pageIsEmpty, absoluteBoxes, fixedBoxes, adjoiningMargins,
				firstLetterStyle, firstLineStyle, false, noMaxLines)
			newChild, resumeAt := result.box, result.resumeAt
			next = result.nextPage
			adjoiningMargins = result.adjoiningMargins
			skip = nil
			if newChild == nil {
				breakPage = true
				break
			}
			newChildren = append(newChildren, newChild)
			currentPositionY = newChild.BorderHeight() + newChild.BorderBoxY()
			adjoiningMargins = append(adjoiningMargins, newChild.MarginBottom)
			if len(resumeAt) > 0 {
				breakPage = true
				columnSkipStack = skipStack{0: resumeAt}
				break
			}
			pageIsEmpty = false
			continue
		}

		// We have a list of children that we have to balance between
		// columns
		columnChildren := item.children

		// Find the total height available for the first run
		currentPositionY += collapseMargin(adjoiningMargins)
		adjoiningMargins = nil
		columnBox := createColumnBox(box, containingBlock, columnChildren, width, currentPositionY)
		height := lc.pageBottom - currentPositionY - originalBottomSpace
		maxHeight := height

		// Try to render columns until the content fits, increase the
		// column height step by step
		lostSpace := math.Inf(1)
		originalExcludedShapes := len(lc.excludedShapes())
		originalPageIsEmpty := pageIsEmpty
		pageIsEmpty = false
		stopRendering := false
		balancing := false
		skip = skipStack{0: skip}
		for loop := 0; ; loop++ {
			lc.spend(1)
			// Remove extra excluded shapes introduced during the previous
			// loop
			lc.truncateExcludedShapes(originalExcludedShapes)
			lc.columnFloats.reset(width, gap, isStr(style.Get("direction"), "rtl"))

			// Render the columns
			columnSkipStack = skip
			var consumedHeights []float64
			var newBoxes []*Box
			// Port: upstream has no -ivy-page-top-space. forced is set
			// when a forced break ends the column before.
			forced := false
			for i := range count {
				// Render one column
				lc.startColumn(columnBox, i, originalPageIsEmpty && pageTop, forced)
				result := blockBoxLayout(lc, columnBox,
					lc.pageBottom-currentPositionY-height,
					columnSkipStack, containingBlock,
					pageIsEmpty || !balancing, new([]*Box), new([]*Box), []float64{},
					firstLetterStyle, firstLineStyle, false, noMaxLines)
				newBox, resumeAt := result.box, result.resumeAt
				next = result.nextPage
				forced = next.brk != "any"
				if newBox == nil {
					// We didn't render anything, retry
					columnSkipStack = skipStack{0: nil}
					break
				}
				newBoxes = append(newBoxes, newBox)
				columnSkipStack = resumeAt
				lc.columnFloats.collect(newBox, i)

				// Calculate consumed height, empty space and next box
				// height
				var consumedHeight, emptySpace, nextBoxHeight float64
				if inFlowChildren := newBox.InFlowChildren(); len(inFlowChildren) > 0 {
					last := inFlowChildren[len(inFlowChildren)-1]
					// Get the empty space at the bottom of the column box
					consumedHeight = last.MarginHeight() + last.PositionY - currentPositionY
					emptySpace = height - consumedHeight
					consumedHeight -= last.MarginBottom

					// Get the minimum size needed to render the next box
					if len(columnSkipStack) > 0 {
						lc.startColumn(columnBox, i+1, false, forced)
						nextBox := blockBoxLayout(lc, columnBox, math.Inf(1), columnSkipStack,
							containingBlock, true, new([]*Box), new([]*Box), []float64{},
							firstLetterStyle, firstLineStyle, false, noMaxLines).box
						// Port: upstream reads the children of None.
						if nextBox != nil {
							for _, child := range nextBox.InFlowChildren() {
								nextBoxHeight = child.MarginHeight()
								break
							}
							removePlaceholders(lc, []*Box{nextBox}, new([]*Box), new([]*Box))
						}
					}
				}

				consumedHeights = append(consumedHeights, consumedHeight)

				// Append the size needed to render the next box in this
				// column.
				//
				// The next box size may be smaller than the empty space, for
				// example when the next box can't be separated from its own
				// next box. In this case we don't try to find the real value
				// and let the workaround below fix this for us.
				//
				// We also want to avoid very small values that may have been
				// introduced by rounding errors. As the workaround below at
				// least adds 1 pixel for each loop, we can ignore lost spaces
				// lower than 1px.
				if nextBoxHeight-emptySpace > 1 {
					lostSpace = min(lostSpace, nextBoxHeight-emptySpace)
				}

				// Stop if we already rendered the whole content
				if resumeAt == nil {
					break
				}
			}

			// Remove placeholders but keep the current footnote area height
			lastFootnotesHeight = columnFootnoteAreaHeight(lc)
			removePlaceholders(lc, newBoxes, new([]*Box), new([]*Box))

			// Port: upstream loops without bound. The port stops the search
			// after maxColumnLoops loops, as when it reaches the maximum
			// height.
			if loop >= maxColumnLoops {
				stopRendering = true
				break
			}

			if balancing {
				if columnSkipStack == nil {
					// We rendered the whole content, stop
					break
				}

				// Increase the column heights and render them again
				addHeight := lostSpace
				if math.IsInf(lostSpace, 1) {
					addHeight = 1
				}
				height += addHeight

				if height > maxHeight {
					// We reached max height, stop rendering
					height = maxHeight
					stopRendering = true
					break
				}
			} else {
				if !slices.Contains(footnoteAreaHeights, lastFootnotesHeight) {
					// Footnotes have been rendered, try to re-render with the
					// new footnote area height
					height -= lastFootnotesHeight - footnoteAreaHeights[len(footnoteAreaHeights)-1]
					footnoteAreaHeights = append(footnoteAreaHeights, lastFootnotesHeight)
					continue
				}

				everythingFits := len(columnSkipStack) == 0 && maxFloat(consumedHeights) <= maxHeight
				if everythingFits {
					// Everything fits, start expanding columns at the average
					// of the column heights
					if isStr(style.Get("column_fill"), "balance") ||
						index < columnsAndBlocks[len(columnsAndBlocks)-1].index {
						balancing = true
						sum := 0.0
						for _, h := range consumedHeights {
							sum += h
						}
						height = sum / float64(count)
					} else {
						break
					}
				} else {
					// Content overflows even at maximum height, stop now and
					// let the columns continue on the next page
					stopRendering = true
					break
				}
			}
		}

		// TODO: check style['max']-height
		bottomSpace = max(bottomSpace, lc.pageBottom-currentPositionY-height)

		// Replace the current box children with real columns
		i := 0
		maxColumnHeight := 0.0
		var columns []*Box
		// Port: upstream has no -ivy-page-top-space, see above.
		forced := false
		lc.columnFloats.reset(width, gap, isStr(style.Get("direction"), "rtl"))
		for {
			lc.spend(1)
			columnBox := createColumnBox(box, containingBlock, columnChildren, width, currentPositionY)
			if isStr(style.Get("direction"), "rtl") {
				columnBox.PositionX += box.Width - float64(i+1)*width - float64(i)*gap
			} else {
				columnBox.PositionX += float64(i) * (width + gap)
			}
			previousSkip := skip
			lc.startColumn(columnBox, i, originalPageIsEmpty && pageTop, forced)
			result := blockBoxLayout(lc, columnBox, bottomSpace, skip, containingBlock,
				originalPageIsEmpty, absoluteBoxes, fixedBoxes, nil,
				firstLetterStyle, firstLineStyle, false, noMaxLines)
			newChild := result.box
			columnSkipStack = result.resumeAt
			forced = result.nextPage.brk != "any"
			if newChild == nil {
				columns = nil
				breakPage = true
				break
			}
			next = result.nextPage
			skip = columnSkipStack
			columns = append(columns, newChild)
			lc.columnFloats.collect(newChild, i)
			maxColumnHeight = max(maxColumnHeight, newChild.MarginHeight())
			if skip == nil {
				bottomSpace = originalBottomSpace
				break
			}
			i++
			if i == count && !heightDefined {
				// [If] a declaration that constrains the column height
				// (e.g., using height or max-height). In this case,
				// additional column boxes are created in the inline
				// direction.
				break
			}
			// Port: upstream makes columns until the content ends. The port
			// stops after maxColumns columns, or when a column takes
			// nothing.
			if i >= maxColumns || skipStackEqual(skip, previousSkip) {
				break
			}
		}

		// Update the current y position and set the columns’ height
		currentPositionY += min(maxHeight, maxColumnHeight)
		for _, column := range columns {
			column.Height = maxColumnHeight
			newChildren = append(newChildren, column)
		}

		skip = nil
		pageIsEmpty = false

		if stopRendering {
			break
		}
	}

	// Report footnotes above the defined footnotes height
	columnReportFootnotes(lc, footnoteAreaHeights[len(footnoteAreaHeights)-1])

	if len(box.Children) > 0 && len(newChildren) == 0 {
		// The box has children but none can be drawn, let's skip the whole
		// box
		lc.inColumn = false
		// Port: upstream returns the tuple (0, None) as resume_at. The port
		// returns {0: None}.
		return blockLayoutResult{resumeAt: skipStack{0: nil}, nextPage: anyNextPage(), maxLines: noMaxLines}
	}

	// Set the height of the containing box
	box.Children = newChildren
	currentPositionY += collapseMargin(adjoiningMargins)
	contentHeight := currentPositionY - box.ContentBoxY()
	var heightDifference float64
	if box.Height == Auto {
		box.Height = contentHeight
		heightDifference = 0
	} else {
		heightDifference = box.Height - contentHeight
	}

	// Update the latest columns’ height to respect min-height
	if box.MinHeight != Auto && box.MinHeight > box.Height {
		heightDifference += box.MinHeight - box.Height
		box.Height = box.MinHeight
	}
	for _, child := range slices.Backward(newChildren) {
		if !child.IsColumn {
			break
		}
		child.Height += heightDifference
	}

	if isStr(style.Get("position"), "relative") {
		// New containing block, resolve the layout of the absolute
		// descendants
		for _, absoluteBox := range *absoluteBoxes {
			absoluteLayout(lc, absoluteBox, box, fixedBoxes, bottomSpace, nil)
		}
	}

	// Calculate skip stack
	if len(columnSkipStack) > 0 {
		columnSkip, childSkip := columnSkipStack.item()
		skip = skipStack{index + columnSkip: childSkip}
	} else if breakPage {
		skip = skipStack{index: nil}
	}

	// Update page bottom according to the new footnotes
	if area := lc.currentFootnoteArea; area != nil && area.Height != Auto {
		lc.pageBottom += footnoteAreaHeights[0]
		lc.pageBottom -= area.MarginHeight()
	}

	lc.inColumn = false
	return blockLayoutResult{box: box, resumeAt: skip, nextPage: next, maxLines: noMaxLines}
}

// columnFootnoteAreaHeight is the height of the footnote area that the
// layout of the columns reads: 0 when the height of the area is auto, and
// its margin height otherwise.
//
// Port: upstream reads the area that make_page sets. The port gives 0
// without an area.
func columnFootnoteAreaHeight(lc *layoutContext) float64 {
	area := lc.currentFootnoteArea
	if area == nil || area.Height == Auto {
		return 0
	}
	return area.MarginHeight()
}

// maxFloat returns the largest value, or 0 for no value.
//
// Port: upstream fails on max() of an empty list.
func maxFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return slices.Max(values)
}

// columnReportFootnotes is _report_footnotes in
// weasyprint/layout/column.py: it reports the footnotes above the defined
// footnotes height.
func columnReportFootnotes(lc *layoutContext, footnotesHeight float64) {
	if len(lc.currentPageFootnotes) == 0 {
		return
	}

	// Report and count footnotes
	reportedFootnotes := 0
	for lc.currentFootnoteArea.MarginHeight() > footnotesHeight {
		// Port: upstream fails when no footnote is left to report.
		if len(lc.currentPageFootnotes) == 0 {
			break
		}
		lc.reportFootnote(lc.currentPageFootnotes[len(lc.currentPageFootnotes)-1])
		reportedFootnotes++
	}

	// Revert reported footnotes, as they’ve been reported starting from the
	// last one
	if reportedFootnotes >= 2 && reportedFootnotes <= len(lc.reportedFootnotes) {
		slices.Reverse(lc.reportedFootnotes[len(lc.reportedFootnotes)-reportedFootnotes:])
	}
}

// createColumnBox is _create_column_box in weasyprint/layout/column.py: it
// creates a column box including the given children.
func createColumnBox(box, containingBlock *Box, children []*Box, width, positionY float64) *Box {
	columnBox := anonymousFrom(box.Kind, box, children)
	resolvePercentages(columnBox, containingBlock)
	columnBox.IsColumn = true
	columnBox.Width = width
	columnBox.PositionX = box.ContentBoxX()
	columnBox.PositionY = positionY
	return columnBox
}
