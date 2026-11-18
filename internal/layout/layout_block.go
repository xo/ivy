// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/block.py: page breaking and layout for
// block-level and block-container boxes.

package layout

import (
	"iter"
	"maps"
	"math"
	"slices"
)

// blockResult is the result of the layouts of this file, with the list of
// the adjoining margins that it returns. The lists of the adjoining margins
// of upstream are shared: a function appends to the list of its caller,
// and returns a list that its caller changes again. So the parent of a box
// sees the margins of its first and its last children. The functions of
// this file share the lists with pointers to slices, and the functions of
// the other files get copies.
type blockResult struct {
	blockLayoutResult
	// margins is the list adjoining_margins of the result.
	margins *[]float64
}

// result returns the blockLayoutResult of the other files, with a copy of
// the adjoining margins.
func (r blockResult) result() blockLayoutResult {
	if r.margins != nil {
		r.adjoiningMargins = slices.Clone(*r.margins)
	}
	return r.blockLayoutResult
}

// sharedResult returns the blockResult of the result of a layout of
// another file, with a new list of adjoining margins.
func sharedResult(r blockLayoutResult) blockResult {
	margins := slices.Clone(r.adjoiningMargins)
	return blockResult{r, &margins}
}

// sharedMargins returns a new shared list that holds a copy of margins, or
// nil for nil, the None of upstream.
func sharedMargins(margins []float64) *[]float64 {
	if margins == nil {
		return nil
	}
	c := slices.Clone(margins)
	return &c
}

// blockLevelLayout is block_level_layout in weasyprint/layout/block.py: it
// lays out the block-level box. absoluteBoxes and fixedBoxes collect the
// placeholders of the absolute and the fixed boxes. A nil absoluteBoxes,
// fixedBoxes or adjoiningMargins is the None of upstream, a new list.
// maxLines is noMaxLines for None.
func blockLevelLayout(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, containingBlock *Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins []float64, firstLetterStyle, firstLineStyle *Style, discard bool, maxLines int) blockLayoutResult {
	return blockLevelLayoutShared(lc, box, bottomSpace, skip, containingBlock,
		pageIsEmpty, absoluteBoxes, fixedBoxes, sharedMargins(adjoiningMargins),
		firstLetterStyle, firstLineStyle, discard, maxLines).result()
}

// blockLevelLayoutShared is blockLevelLayout with a shared list of
// adjoining margins.
func blockLevelLayoutShared(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, containingBlock *Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins *[]float64, firstLetterStyle, firstLineStyle *Style, discard bool, maxLines int) blockResult {
	if absoluteBoxes == nil {
		absoluteBoxes = &[]*Box{}
	}
	if fixedBoxes == nil {
		fixedBoxes = &[]*Box{}
	}
	if adjoiningMargins == nil {
		adjoiningMargins = &[]float64{}
	}

	if !box.IsTable() {
		resolvePercentages(box, containingBlock)

		if box.MarginTop == Auto {
			box.MarginTop = 0
		}
		if box.MarginBottom == Auto {
			box.MarginBottom = 0
		}

		if lc.currentPage > 1 && pageIsEmpty {
			// When an unforced break occurs before or after a block-level
			// box, any margins adjoining the break are truncated to zero.
			// TODO: this condition is wrong, it only works for blocks whose
			// parent breaks collapsing margins. It should work for blocks
			// whose one of the ancestors breaks collapsing margins.
			// See test_margin_break_clearance.
			collapseWithPage := containingBlock.IsForRootElement || len(*adjoiningMargins) > 0
			if collapseWithPage {
				switch str(box.Style.Get("margin_break")) {
				case "discard":
					box.MarginTop = 0
				case "auto":
					if !lc.forcedBreak {
						box.MarginTop = 0
					}
				}
			}
		}

		collapsedMargin := collapseMargin(append(slices.Clip(*adjoiningMargins), box.MarginTop))
		direction := containingDirection(containingBlock)
		box.Clearance, box.HasClearance = getClearance(lc, box, direction, collapsedMargin)
		if box.HasClearance {
			topBorderEdge := box.PositionY + collapsedMargin + box.Clearance
			box.PositionY = topBorderEdge - box.MarginTop
			adjoiningMargins = &[]float64{}
		}
	}

	return blockLevelLayoutSwitchShared(lc, box, bottomSpace, skip, containingBlock,
		pageIsEmpty, absoluteBoxes, fixedBoxes, adjoiningMargins,
		firstLetterStyle, firstLineStyle, discard, maxLines)
}

// blockLevelLayoutSwitch is block_level_layout_switch in
// weasyprint/layout/block.py: it calls the layout function that matches
// the type of box.
func blockLevelLayoutSwitch(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, containingBlock *Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins []float64, firstLetterStyle, firstLineStyle *Style, discard bool, maxLines int) blockLayoutResult {
	return blockLevelLayoutSwitchShared(lc, box, bottomSpace, skip, containingBlock,
		pageIsEmpty, absoluteBoxes, fixedBoxes, sharedMargins(adjoiningMargins),
		firstLetterStyle, firstLineStyle, discard, maxLines).result()
}

// blockLevelLayoutSwitchShared is blockLevelLayoutSwitch with a shared
// list of adjoining margins.
func blockLevelLayoutSwitchShared(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, containingBlock *Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins *[]float64, firstLetterStyle, firstLineStyle *Style, discard bool, maxLines int) blockResult {
	lc.enter()
	defer lc.leave()
	if absoluteBoxes == nil {
		absoluteBoxes = &[]*Box{}
	}
	if fixedBoxes == nil {
		fixedBoxes = &[]*Box{}
	}

	var result blockLayoutResult
	switch {
	case box.IsTable():
		result = tableLayout(lc, box, bottomSpace, skip, containingBlock,
			pageIsEmpty, absoluteBoxes, fixedBoxes)
	case box.Is(KindBlock):
		return blockBoxLayoutShared(lc, box, bottomSpace, skip, containingBlock,
			pageIsEmpty, absoluteBoxes, fixedBoxes, adjoiningMargins,
			firstLetterStyle, firstLineStyle, discard, maxLines)
	case box.Kind == KindBlockReplaced:
		result = blockReplacedBoxLayout(lc, box, containingBlock)
	case box.Kind == KindFlex:
		result = flexLayout(lc, box, bottomSpace, skip, containingBlock,
			pageIsEmpty, absoluteBoxes, fixedBoxes, discard)
	case box.Kind == KindGrid:
		result = gridLayout(lc, box, bottomSpace, skip, containingBlock,
			pageIsEmpty, absoluteBoxes, fixedBoxes)
	default:
		// Port: upstream raises TypeError for the layout of another type
		// of box. The port keeps the box, without children and without
		// size.
		result = blockLayoutResult{box: emptyLayoutBox(box), nextPage: anyNextPage()}
	}
	result.maxLines = noMaxLines
	return sharedResult(result)
}

// blockBoxLayout is block_box_layout in weasyprint/layout/block.py: it lays
// out the block box.
func blockBoxLayout(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, containingBlock *Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins []float64, firstLetterStyle, firstLineStyle *Style, discard bool, maxLines int) blockLayoutResult {
	return blockBoxLayoutShared(lc, box, bottomSpace, skip, containingBlock,
		pageIsEmpty, absoluteBoxes, fixedBoxes, sharedMargins(adjoiningMargins),
		firstLetterStyle, firstLineStyle, discard, maxLines).result()
}

// blockBoxLayoutShared is blockBoxLayout with a shared list of adjoining
// margins.
func blockBoxLayoutShared(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, containingBlock *Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins *[]float64, firstLetterStyle, firstLineStyle *Style, discard bool, maxLines int) blockResult {
	if !isStr(box.Style.Get("column_width"), "auto") || !isStr(box.Style.Get("column_count"), "auto") {
		var margins []float64
		if adjoiningMargins != nil {
			margins = *adjoiningMargins
		}
		result := columnsLayout(lc, box, bottomSpace, skip, containingBlock,
			pageIsEmpty, absoluteBoxes, fixedBoxes, margins,
			firstLetterStyle, firstLineStyle)
		// TODO: this condition and the whole relayout are probably wrong
		if result.resumeAt == nil && result.box != nil {
			newBox := result.box
			columnsBottomSpace := newBox.MarginBottom + newBox.PaddingBottom + newBox.BorderBottomWidth
			if columnsBottomSpace != 0 {
				removePlaceholders(lc, []*Box{newBox}, absoluteBoxes, fixedBoxes)
				bottomSpace += columnsBottomSpace
				result = columnsLayout(lc, box, bottomSpace, skip, containingBlock,
					pageIsEmpty, absoluteBoxes, fixedBoxes, margins,
					firstLetterStyle, firstLineStyle)
			}
		}
		result.maxLines = noMaxLines
		return sharedResult(result)
	} else if box.IsTableWrapper {
		tableWrapperWidth(lc, box, sizeBlock(containingBlock.Width, containingBlock.Height))
	}
	blockLevelWidth(box, containingBlock)

	result := blockContainerLayoutShared(lc, box, bottomSpace, skip, pageIsEmpty,
		absoluteBoxes, fixedBoxes, adjoiningMargins, firstLetterStyle,
		firstLineStyle, discard, maxLines)
	// TODO: columns and flex items shouldn't be block boxes, this condition
	// would then be useless when this is fixed.
	newBox := result.box
	if newBox == nil || newBox.IsColumn || newBox.IsFlexItem {
		return result
	}
	if newBox.IsTableWrapper || newBox.EstablishesFormattingContext() {
		// Don't collide with floats
		// https://www.w3.org/TR/CSS21/visuren.html#floats
		positionX, positionY, _ := avoidCollisions(lc, newBox, containingBlock, false)
		newBox.Translate(positionX-newBox.PositionX, positionY-newBox.PositionY, false)
	}
	return result
}

// blockLevelWidth is block_level_width in weasyprint/layout/block.py: it
// sets the width of box. containingBlock is a box or a sizeBlock.
func blockLevelWidth(box, containingBlock *Box) {
	blockLevelWidthMinMax(box, containingBlock, true)
}

// blockLevelWidthWithoutMinMax is block_level_width.without_min_max in
// weasyprint/layout/block.py.
func blockLevelWidthWithoutMinMax(box, containingBlock *Box) {
	blockLevelWidthMinMax(box, containingBlock, false)
}

// blockLevelWidthMinMax is block_level_width in weasyprint/layout/block.py,
// with its argument with_min_max.
func blockLevelWidthMinMax(box, containingBlock *Box, withMinMax bool) {
	// 'cb' stands for 'containing block'
	cbWidth := containingBlock.Width
	// TODO: what is the real text direction?
	direction := "ltr"
	if containingBlock.isBox() {
		direction = containingDirection(containingBlock)
	}

	paddingPlusBorder := box.PaddingLeft + box.PaddingRight + box.BorderLeftWidth + box.BorderRightWidth

	// See https://www.w3.org/TR/CSS21/visudet.html#blockwidth.
	// Set width. Only margin-left, margin-right and width can be 'auto'.
	// We want:  width of containing block ==
	//               margin-left + border-left-width + padding-left + width
	//               + padding-right + border-right-width + margin-right
	if box.Width == Auto {
		box.Width = cbWidth - paddingPlusBorder
		if box.MarginLeft != Auto {
			box.Width -= box.MarginLeft
		}
		if box.MarginRight != Auto {
			box.Width -= box.MarginRight
		}
	}
	if withMinMax {
		box.Width = max(box.MinWidth, min(box.MaxWidth, box.Width))
	}

	// Set auto margins to 0 for boxes larger than containing block.
	marginWidth := paddingPlusBorder + box.Width
	if box.MarginLeft != Auto {
		marginWidth += box.MarginLeft
	}
	if box.MarginRight != Auto {
		marginWidth += box.MarginRight
	}
	if marginWidth > cbWidth {
		if box.MarginLeft == Auto {
			box.MarginLeft = 0
		}
		if box.MarginRight == Auto {
			box.MarginRight = 0
		}
	}

	// Right-align right-to-left boxes.
	if direction == "rtl" && !box.IsColumn {
		box.PositionX += cbWidth - paddingPlusBorder - box.Width
		if box.MarginLeft != Auto {
			box.PositionX -= box.MarginLeft
		}
		if box.MarginRight != Auto {
			box.PositionX -= box.MarginRight
		}
	}

	// Set margins according to width.
	marginSum := cbWidth - paddingPlusBorder - box.Width
	switch {
	case box.MarginLeft == Auto && box.MarginRight == Auto:
		box.MarginLeft = marginSum / 2
		box.MarginRight = marginSum / 2
	case box.MarginLeft == Auto && box.MarginRight != Auto:
		box.MarginLeft = marginSum - box.MarginRight
	case box.MarginLeft != Auto && box.MarginRight == Auto:
		box.MarginRight = marginSum - box.MarginLeft
	}
}

// relativePositioning is relative_positioning in
// weasyprint/layout/block.py: it translates the box if it is relatively
// positioned. containingBlock is a box or a sizeBlock.
func relativePositioning(box, containingBlock *Box) {
	if isStr(box.Style.Get("position"), "relative") {
		resolvePositionPercentages(box, containingBlock)

		var translateX, translateY float64
		switch {
		case box.Left != Auto && box.Right != Auto:
			if isStr(box.Style.Get("direction"), "ltr") {
				translateX = box.Left
			} else {
				translateX = -box.Right
			}
		case box.Left != Auto:
			translateX = box.Left
		case box.Right != Auto:
			translateX = -box.Right
		}

		switch {
		case box.Top != Auto:
			translateY = box.Top
		case box.Bottom != Auto:
			translateY = -box.Bottom
		}

		box.Translate(translateX, translateY, false)
	}

	if box.Kind == KindInline || box.Kind == KindLine {
		for _, child := range box.Children {
			relativePositioning(child, containingBlock)
		}
	}
}

// blockOutOfFlowLayout is _out_of_flow_layout in
// weasyprint/layout/block.py: the layout of an out-of-flow child of box. It
// returns whether the layout of the parent stops after this layout, where
// to resume the in-flow layout, the laid out child, and where to resume
// the out-of-flow layout.
func blockOutOfFlowLayout(lc *layoutContext, box *Box, index int, child *Box, newChildren *[]*Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins []float64, bottomSpace float64) (stop bool, resumeAt skipStack, newChild *Box, outOfFlowResumeAt skipStack) {
	// Add the parent’s collapsing margins to shift the child’s position.
	// Don’t include the out-of-flow child’s top margin because it doesn’t
	// collapse with its parent.
	child.PositionY += collapseMargin(adjoiningMargins)

	switch {
	case child.IsAbsolutelyPositioned():
		// Absolute child layout: create placeholder.
		placeholder := newAbsolutePlaceholder(child)
		newChild = placeholder
		placeholder.index = index
		*newChildren = append(*newChildren, placeholder)
		if isStr(child.Style.Get("position"), "absolute") {
			*absoluteBoxes = append(*absoluteBoxes, placeholder)
		} else {
			*fixedBoxes = append(*fixedBoxes, placeholder)
		}

	case child.IsFloated():
		// Float child layout.
		// Check for forced break-before on the float.
		// https://drafts.csswg.org/css-break/#break-between
		newChild, outOfFlowResumeAt = floatLayout(lc, child, box, absoluteBoxes, fixedBoxes, bottomSpace, nil)

		// Check that child doesn’t overflow page.
		pageOverflow := lc.overflowsPage(bottomSpace, newChild.PositionY+newChild.Height)
		addChild := (pageIsEmpty && len(*newChildren) == 0) || !pageOverflow || box.IsMonolithic()
		if addChild {
			// Child fits or has to fit, add it.
			newChild.index = index
			*newChildren = append(*newChildren, newChild)
		} else {
			// Child doesn’t fit and we can break, find where to break and
			// stop parent rendering.
			lastChild := findLastInFlowChild(*newChildren)
			pageBreak := blockLevelPageBreak(lastChild, child)
			resumeAt = skipStack{index: nil}
			outOfFlowResumeAt = nil
			stop = true
			removePlaceholders(lc, []*Box{newChild}, absoluteBoxes, fixedBoxes)
			if len(*newChildren) > 0 && avoidPageBreak(pageBreak, lc) {
				// Can’t break inside float, find an earlier page break.
				if children, earlierResumeAt, ok := findEarlierPageBreak(lc, *newChildren, absoluteBoxes, fixedBoxes); ok {
					// Earlier page break found, drop whole child rendering.
					*newChildren = children
					resumeAt = earlierResumeAt
					newChild = nil
				}
			}
		}

	case child.IsRunning() || child.IsNote():
		// Running element layout.
		lc.addRunningElement(child)
	}

	return stop, resumeAt, newChild, outOfFlowResumeAt
}

// breakLine is _break_line in weasyprint/layout/block.py: it breaks the
// line where orphans and widows allow it. It returns (abort, stop,
// resume_at).
func breakLine(lc *layoutContext, box, line *Box, newChildren *[]*Box, needed int, pageIsEmpty bool, index int, skip, resumeAt skipStack, absoluteBoxes, fixedBoxes *[]*Box) (bool, bool, skipStack) {
	overOrphans := len(*newChildren) - int(num(box.Style.Get("orphans")))
	if overOrphans < 0 && !pageIsEmpty {
		// Reached the bottom of the page before we had
		// enough lines for orphans, cancel the whole box.
		removePlaceholders(lc, line.Children, absoluteBoxes, fixedBoxes)
		return true, false, resumeAt
	}
	// How many lines we need on the next page to satisfy widows
	// -1 for the current line.
	if needed > overOrphans && !pageIsEmpty {
		// Total number of lines < orphans + widows
		removePlaceholders(lc, line.Children, absoluteBoxes, fixedBoxes)
		return true, false, resumeAt
	}
	if needed > 0 && needed <= overOrphans {
		// Remove lines to keep them for the next page
		// Port: upstream reads the slice from the end, which keeps all the
		// lines for a needed larger than the lines.
		start := max(0, len(*newChildren)-needed)
		for _, child := range (*newChildren)[start:] {
			removePlaceholders(lc, child.Children, absoluteBoxes, fixedBoxes)
		}
		*newChildren = (*newChildren)[:start]
	}
	// Page break here, resume before this line
	removePlaceholders(lc, line.Children, absoluteBoxes, fixedBoxes)
	return false, true, skipStack{index: skip}
}

// lineboxLayout is _linebox_layout in weasyprint/layout/block.py: the
// layout of the line box child of box. It returns (abort, stop,
// resume_at, position_y, new_footnotes, max_lines).
func lineboxLayout(lc *layoutContext, box *Box, index int, child *Box, newChildren *[]*Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins []float64, bottomSpace, positionY float64, skip skipStack, firstLetterStyle, firstLineStyle *Style, drawBottomDecoration bool, maxLines int) (abort, stop bool, resumeAt skipStack, endPositionY float64, newFootnotes []*Box, endMaxLines int) {
	// Port: upstream asserts that the line box has no siblings before the
	// layout.

	if len(adjoiningMargins) > 0 {
		positionY += collapseMargin(adjoiningMargins)
	}
	newContainingBlock := box
	linesIterator := iterLineBoxes(lc, child, positionY, bottomSpace, skip,
		newContainingBlock, absoluteBoxes, fixedBoxes, firstLetterStyle, firstLineStyle)
	nextLine, stopLines := iter.Pull2(linesIterator)
	defer stopLines()

	widows := int(num(box.Style.Get("widows")))
	orphans := int(num(box.Style.Get("orphans")))
	for {
		line, lineResumeAt, ok := nextLine()
		if !ok {
			break
		}
		resumeAt = lineResumeAt
		lc.spend(1)

		// Break box if we reached max-lines
		if maxLines != noMaxLines {
			if maxLines == 0 {
				// Port: upstream fails when no line is laid out yet.
				if n := len(*newChildren); n > 0 {
					(*newChildren)[n-1].ownExtra().BlockEllipsis = box.Style.Get("block_ellipsis")
				}
				break
			}
			maxLines--
		}

		// Update line resume_at and position_y
		line.resumeAt = resumeAt
		newPositionY := line.PositionY + line.Height

		// Add bottom padding and border to the bottom position of the
		// box if needed
		drawBottomDecoration = drawBottomDecoration || resumeAt == nil
		offsetY := 0.0
		if drawBottomDecoration {
			offsetY = box.BorderBottomWidth + box.PaddingBottom
		}

		// Allow overflow if the first line of the page is higher than the
		// page itself so that we put *something* on this page and can
		// advance in the context.
		overflow := (len(*newChildren) > 0 || !pageIsEmpty) &&
			lc.overflowsPage(bottomSpace, newPositionY+offsetY)
		if overflow {
			// If we couldn’t break the line before but can break now, first
			// try to report footnotes and see if we don’t overflow.
			couldBreakBefore, canBreakNow := true, true
			needed := widows - 1
			for {
				nextLineBox, _, ok := nextLine()
				if !ok {
					break
				}
				needed--
				removePlaceholders(lc, nextLineBox.Children, absoluteBoxes, fixedBoxes)
				// Don’t iterate over all lines as it can be long.
				if needed == -1 {
					break
				}
			}
			if len(*newChildren)+1 < orphans {
				canBreakNow = false
			} else if needed >= 0 {
				canBreakNow = false
			}
			if len(*newChildren) < orphans {
				couldBreakBefore = false
			} else if needed > 0 {
				couldBreakBefore = false
			}
			needed = max(0, needed)
			report := !lc.inColumn && canBreakNow && !couldBreakBefore
			reportedFootnotes := 0
			var footnotes []*Box
			for _, line := range *newChildren {
				line.Descendants(func(descendant *Box) bool {
					if descendant.extra().Footnote != nil && slices.Contains(lc.currentPageFootnotes, descendant.extra().Footnote) {
						footnotes = append(footnotes, descendant.extra().Footnote)
					}
					return true
				})
			}
			broke := false
			for report && len(footnotes) > 0 {
				footnote := footnotes[len(footnotes)-1]
				footnotes = footnotes[:len(footnotes)-1]
				lc.reportFootnote(footnote)
				reportedFootnotes++
				if !lc.overflowsPage(bottomSpace, newPositionY+offsetY) {
					stop = true
					overflow = false
					broke = true
					break
				}
			}
			if !broke {
				abort, stop, resumeAt = breakLine(lc, box, line, newChildren,
					needed, pageIsEmpty, index, skip, resumeAt, absoluteBoxes, fixedBoxes)
			}

			// Revert reported footnotes, as they’ve been reported starting
			// from the last one.
			if reportedFootnotes >= 2 {
				n := len(lc.reportedFootnotes)
				slices.Reverse(lc.reportedFootnotes[max(0, n-reportedFootnotes):])
			}
		}

		if overflow {
			break
		} else if pageIsEmpty && lc.overflowsPage(bottomSpace, newPositionY) {
			// TODO: this is incomplete.
			// See https://drafts.csswg.org/css-page-3/#allowed-pg-brk
			// "When an unforced page break occurs here, both the adjoining
			//  ‘margin-top’ and ‘margin-bottom’ are set to zero."
			// See issue #115.
			//
			// Remove the top border when a page is empty and the box is
			// too high to be drawn in one page
			newPositionY -= box.MarginTop
			line.Translate(0, -box.MarginTop, false)
			box.MarginTop = 0
		}

		if len(lc.footnotes) > 0 {
			breakLinebox := false
			var descendants []*Box
			line.Descendants(func(descendant *Box) bool {
				descendants = append(descendants, descendant)
				return true
			})
			for _, descendant := range descendants {
				footnote := descendant.extra().Footnote
				if footnote == nil || !slices.Contains(lc.footnotes, footnote) {
					continue
				}
				overflow := lc.layoutFootnote(footnote)
				newFootnotes = append(newFootnotes, footnote)
				overflow = overflow || len(lc.reportedFootnotes) > 0 ||
					lc.overflowsPage(bottomSpace, newPositionY+offsetY)
				if overflow {
					lc.reportFootnote(footnote)
					// If we've put other content on this page, then we may
					// want to push this line or block to the next page.
					// Otherwise, we can't (and would loop forever if we
					// tried), so don't even try.
					if len(*newChildren) > 0 || !pageIsEmpty {
						policy := str(footnote.Style.Get("footnote_policy"))
						if policy == "line" {
							needed := widows - 1
							if needed != 0 {
								for {
									if _, _, ok := nextLine(); !ok {
										break
									}
									needed--
									// Don’t iterate over all lines as it can be
									// long.
									if needed == 0 {
										break
									}
								}
							}
							abort, stop, resumeAt = breakLine(lc, box, line, newChildren,
								needed, pageIsEmpty, index, skip, resumeAt, absoluteBoxes, fixedBoxes)
							breakLinebox = true
							break
						} else if policy == "block" {
							abort, breakLinebox = true, true
							break
						}
					}
				}
			}
			if breakLinebox {
				break
			}
		}

		*newChildren = append(*newChildren, line)
		positionY = newPositionY
		skip = resumeAt
	}

	if n := len(*newChildren); n > 0 {
		resumeAt = skipStack{index: (*newChildren)[n-1].resumeAt}
	}

	return abort, stop, resumeAt, positionY, newFootnotes, maxLines
}

// inFlowResult is the tuple that _in_flow_layout returns: (abort, stop,
// resume_at, position_y, adjoining_margins, next_page, new_children,
// max_lines). The new children are in the list of the caller.
type inFlowResult struct {
	abort, stop      bool
	resumeAt         skipStack
	positionY        float64
	adjoiningMargins *[]float64
	nextPage         nextPage
	maxLines         int
}

// blockInFlowLayout is _in_flow_layout in weasyprint/layout/block.py: the
// layout of an in-flow child of box that is not a line box.
func blockInFlowLayout(lc *layoutContext, box *Box, index int, child *Box, newChildren *[]*Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins *[]float64, bottomSpace, positionY float64, skip skipStack, firstLetterStyle, firstLineStyle *Style, discard bool, next nextPage, maxLines int) inFlowResult {
	if adjoiningMargins == nil {
		adjoiningMargins = &[]float64{}
	}
	result := func(abort, stop bool, resumeAt skipStack) inFlowResult {
		return inFlowResult{abort, stop, resumeAt, positionY, adjoiningMargins, next, maxLines}
	}

	// Resolve percentages and collapsing top margins.
	if !box.IsTableWrapper {
		resolvePercentages(child, box)
		lastInFlowChild := findLastInFlowChild(*newChildren)
		if lastInFlowChild == nil && box.TopMarginCollapses() {
			// TODO: add the adjoining descendants' margin top to
			// [child.margin_top].
			oldCollapsedMargin := collapseMargin(*adjoiningMargins)
			// TODO: the margin-top value is set afterwards in
			// block_level_layout, we shouldn’t duplicate this code.
			childMarginTop := child.MarginTop
			if childMarginTop == Auto {
				childMarginTop = 0
			} else if lc.currentPage > 1 && pageIsEmpty {
				switch str(box.Style.Get("margin_break")) {
				case "discard":
					childMarginTop = 0
				case "auto":
					if !lc.forcedBreak {
						childMarginTop = 0
					}
				}
			}
			newCollapsedMargin := collapseMargin(append(slices.Clip(*adjoiningMargins), childMarginTop))
			collapsedMarginDifference := newCollapsedMargin - oldCollapsedMargin
			for _, previousNewChild := range *newChildren {
				previousNewChild.Translate(0, collapsedMarginDifference, false)
			}
			direction := str(box.Style.Get("direction"))
			if _, ok := getClearance(lc, child, direction, newCollapsedMargin); ok {
				for _, previousNewChild := range *newChildren {
					previousNewChild.Translate(0, -collapsedMarginDifference, false)
				}

				collapsedMargin := collapseMargin(*adjoiningMargins)
				box.PositionY += collapsedMargin - box.MarginTop
				// Count box.margin_top as we emptied adjoining_margins
				adjoiningMargins = &[]float64{}
				positionY = box.ContentBoxY()
			}
		}
	}

	// TODO: Merge this with block_container_layout, block_level_layout,
	// _in_flow_layout, and check code above.
	if len(*adjoiningMargins) > 0 {
		if box.IsTableWrapper { // should not be a special case
			collapsedMargin := collapseMargin(*adjoiningMargins)
			child.PositionY += collapsedMargin
			adjoiningMargins = &[]float64{}
		} else if !child.Is(KindBlock) { // blocks handle that themselves
			marginTop := 0.0
			if !isStr(child.Style.Get("margin_top"), "auto") {
				marginTop, _ = percentageFloat(child.Style.Get("margin_top"), child.Style, box.Width)
			}
			*adjoiningMargins = append(*adjoiningMargins, marginTop)
			offsetY := collapseMargin(*adjoiningMargins) - marginTop
			child.PositionY += offsetY
			adjoiningMargins = &[]float64{}
		}
	}

	pageIsEmptyWithNoChildren := pageIsEmpty
	for _, newChild := range *newChildren {
		if !newChild.IsPlaceholder() {
			pageIsEmptyWithNoChildren = false
			break
		}
	}

	childPositionX, childPositionY := child.PositionX, child.PositionY
	layout := blockLevelLayoutShared(lc, child, bottomSpace, skip, box,
		pageIsEmptyWithNoChildren, absoluteBoxes, fixedBoxes, adjoiningMargins,
		firstLetterStyle, firstLineStyle, discard, maxLines)
	newChild, resumeAt := layout.box, layout.resumeAt
	next, maxLines = layout.nextPage, layout.maxLines

	// Check that child doesn’t overflow and set next position_y.
	if newChild != nil {
		if !layout.collapsingThrough {
			// Find content position and check that it doesn’t overflow.
			newContentPositionY := newChild.ContentBoxY() + newChild.Height
			contentPageOverflow := lc.overflowsPage(bottomSpace, newContentPositionY)

			// Update bottom space to include new child bottom spacing and
			// check that it doesn’t overflow.
			bottomSpace += newChild.PaddingBottom + newChild.BorderBottomWidth
			if !box.BottomMarginCollapses() {
				bottomSpace += newChild.MarginBottom
			}
			newPositionY := newChild.BorderBoxY() + newChild.BorderHeight()
			borderPageOverflow := lc.overflowsPage(bottomSpace, newPositionY)

			canBreak := !pageIsEmptyWithNoChildren && !box.IsMonolithic()
			switch {
			case canBreak && contentPageOverflow:
				// Child content overflows the page area, display it on the
				// next page.
				removePlaceholders(lc, []*Box{newChild}, absoluteBoxes, fixedBoxes)
				newChild = nil
			case canBreak && borderPageOverflow:
				// Child border/padding/margin overflows the page area, do
				// the layout again with a bottom_space value that includes
				// them.
				removePlaceholders(lc, []*Box{newChild}, absoluteBoxes, fixedBoxes)
				child.PositionX, child.PositionY = childPositionX, childPositionY
				layout = blockLevelLayoutShared(lc, child, bottomSpace, skip, box,
					pageIsEmptyWithNoChildren, absoluteBoxes, fixedBoxes,
					adjoiningMargins, firstLetterStyle, firstLineStyle, discard, maxLines)
				newChild, resumeAt = layout.box, layout.resumeAt
				next, maxLines = layout.nextPage, layout.maxLines
				if newChild != nil {
					positionY = newChild.BorderBoxY() + newChild.BorderHeight()
				}
			default:
				positionY = newPositionY
			}
		}

		// Use the new child adjoining margins.
		adjoiningMargins = layout.margins
		if adjoiningMargins == nil {
			adjoiningMargins = &[]float64{}
		}
		if newChild != nil {
			*adjoiningMargins = append(*adjoiningMargins, newChild.MarginBottom)
		}

		// Handle clearance.
		if newChild != nil && newChild.HasClearance && newChild.Clearance != 0 {
			positionY = newChild.BorderBoxY() + newChild.BorderHeight()
		}
	}

	if newChild == nil {
		// Nothing fits in the remaining space of this page: break.
		pageBreak := "auto"
		if lastChild := findLastClassABreakPointChild(*newChildren); lastChild != nil {
			pageBreak = blockLevelPageBreak(lastChild, child)
		}
		if avoidPageBreak(pageBreak, lc) {
			// TODO: fill the blank space at the bottom of the page.
			if children, earlierResumeAt, ok := findEarlierPageBreak(lc, *newChildren, absoluteBoxes, fixedBoxes); ok {
				*newChildren = children
				return result(false, true, earlierResumeAt)
			}
			// We did not find any page break opportunity.
			if !pageIsEmpty {
				// The page has content *before* this block: cancel the
				// block and try to find a break in the parent.
				return result(true, false, resumeAt)
			}
			// else:
			// ignore this 'avoid' and break anyway.
		}

		allAbsolute := true
		for _, newChild := range *newChildren {
			if !newChild.IsAbsolutelyPositioned() {
				allAbsolute = false
				break
			}
		}
		if allAbsolute {
			// This box has only rendered absolute children, keep them for
			// the next page. This is for example useful for list markers.
			removePlaceholders(lc, *newChildren, absoluteBoxes, fixedBoxes)
			*newChildren = nil
		}

		if len(*newChildren) > 0 {
			// We already have children, keep them and stop the box
			// rendering.
			return result(false, true, skipStack{index: nil})
		}
		// This was the first child of this box, cancel the box completly.
		return result(true, false, resumeAt)
	}

	// Index in its non-laid-out parent, not in future new parent.
	// May be used in find_earlier_page_break().
	newChild.index = index

	*newChildren = append(*newChildren, newChild)
	if resumeAt != nil {
		return result(false, true, skipStack{index: resumeAt})
	}
	return result(false, false, resumeAt)
}

// styleDict returns the dict {key: style[key] for key in style.cascaded}
// of upstream: the computed values of the properties of style that the
// cascade sets. The port keeps the dict in a copy of style that holds only
// these values. The items of the dict are the values of the copy, which
// the layout of the lines reads.
func styleDict(style *Style) *Style {
	d := style.Copy()
	d.values = make(map[string]Value, len(style.cascaded))
	for key := range style.cascaded {
		d.values[key] = style.Get(key)
	}
	return d
}

// mergeStyleDict is the operator |= of the dicts of styleDict: it sets the
// items of other in d.
func mergeStyleDict(d, other *Style) {
	if d.values == nil {
		d.values = map[string]Value{}
	}
	maps.Copy(d.values, other.values)
}

// blockContainerLayout is block_container_layout in
// weasyprint/layout/block.py: it lays out the children of the block
// container box, and sets its height.
func blockContainerLayout(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins []float64, firstLetterStyle, firstLineStyle *Style, discard bool, maxLines int) blockLayoutResult {
	return blockContainerLayoutShared(lc, box, bottomSpace, skip, pageIsEmpty,
		absoluteBoxes, fixedBoxes, sharedMargins(adjoiningMargins),
		firstLetterStyle, firstLineStyle, discard, maxLines).result()
}

// blockContainerLayoutShared is blockContainerLayout with a shared list of
// adjoining margins.
func blockContainerLayoutShared(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box, adjoiningMargins *[]float64, firstLetterStyle, firstLineStyle *Style, discard bool, maxLines int) blockResult {
	lc.enter()
	defer lc.leave()
	// Port: upstream asserts that box is a BlockContainerBox.
	if absoluteBoxes == nil {
		absoluteBoxes = &[]*Box{}
	}
	if fixedBoxes == nil {
		fixedBoxes = &[]*Box{}
	}

	if box.EstablishesFormattingContext() {
		lc.createBlockFormattingContext(box)
	}

	// TODO: merge this with _in_flow_layout, flex_layout…
	isStart := skip == nil
	box.RemoveDecoration(!isStart, false)

	discard = discard || isStr(box.Style.Get("continue"), "discard")
	drawBottomDecoration := discard || isStr(box.Style.Get("box_decoration_break"), "clone")

	if drawBottomDecoration {
		bottomSpace += box.PaddingBottom + box.BorderBottomWidth + box.MarginBottom
	}

	if adjoiningMargins == nil {
		adjoiningMargins = &[]float64{}
	}
	*adjoiningMargins = append(*adjoiningMargins, box.MarginTop)
	thisBoxAdjoiningMargins := adjoiningMargins

	var positionY float64
	if box.TopMarginCollapses() {
		// Not counting margins in adjoining_margins, if any (there are not
		// padding or borders, see above).
		positionY = box.PositionY
	} else {
		box.PositionY += collapseMargin(*adjoiningMargins) - box.MarginTop
		adjoiningMargins = &[]float64{}
		positionY = box.ContentBoxY()
	}

	positionX := box.ContentBoxX()

	if isStr(box.Style.Get("position"), "relative") {
		// New containing block, use a new absolute list.
		absoluteBoxes = &[]*Box{}
	}

	var newChildren []*Box
	next := anyNextPage()

	if styleMaxLines := box.Style.Get("max_lines"); !isStr(styleMaxLines, "none") && isNumber(styleMaxLines) {
		if maxLines <= 0 {
			// max_lines or inf is inf for None and for 0.
			maxLines = int(num(styleMaxLines))
		} else {
			maxLines = min(int(num(styleMaxLines)), maxLines)
		}
	}

	if isStart && box.extra().FirstLineStyle != nil {
		boxFirstLineStyle := styleDict(box.extra().FirstLineStyle)
		if firstLineStyle == nil {
			firstLineStyle = boxFirstLineStyle
		} else {
			mergeStyleDict(firstLineStyle, boxFirstLineStyle)
		}
	}

	if box.extra().FirstLetterStyle != nil {
		boxFirstLetterStyle := styleDict(box.extra().FirstLetterStyle)
		if firstLetterStyle == nil {
			firstLetterStyle = boxFirstLetterStyle
		} else {
			mergeStyleDict(firstLetterStyle, boxFirstLetterStyle)
		}
	}

	// Layout box children.
	skipIndex := 0
	if !isStart {
		skipIndex, skip = skip.item()
	}
	// Port: upstream gives no child for a skip stack past the children.
	skipIndex = min(max(skipIndex, 0), len(box.Children))
	var resumeAt skipStack
	stopped := false
	for index := skipIndex; index < len(box.Children); index++ {
		child := box.Children[index]
		lc.spend(1)
		child.PositionX = positionX
		child.PositionY = positionY // doesn’t count adjoining_margins
		abort, stop := false, false
		var newFootnotes []*Box

		if child.IsInNormalFlow() || child.IsFloated() {
			// Find possible page break between siblings.
			if lastChild := findLastClassABreakPointChild(newChildren); lastChild != nil {
				pageBreak := blockLevelPageBreak(lastChild, child)
				pageName := blockLevelPageName(lastChild, child)
				if truthy(pageName) || forcePageBreak(pageBreak, lc) {
					next = nextPage{brk: pageBreak, page: pageName}
					resumeAt = skipStack{index: nil}
					stop = true
				}
			}
		}

		switch {
		case !stop && !child.IsInNormalFlow():
			// Layout out-of-flow child.
			var newChild *Box
			var outResumeAt skipStack
			stop, resumeAt, newChild, outResumeAt = blockOutOfFlowLayout(lc, box, index, child,
				&newChildren, pageIsEmpty, absoluteBoxes, fixedBoxes, *adjoiningMargins, bottomSpace)
			if len(outResumeAt) > 0 && newChild != nil {
				lc.addBrokenOutOfFlow(newChild, child, box, outResumeAt)
			}
			if child.IsOutsideMarker && newChild != nil {
				newChild.PositionX = box.BorderBoxX()
				if isStr(child.Style.Get("direction"), "rtl") {
					newChild.PositionX += box.Width + box.PaddingRight
				}
			}

		case !stop && child.Kind == KindLine:
			// Layout line child.
			abort, stop, resumeAt, positionY, newFootnotes, maxLines = lineboxLayout(
				lc, box, index, child, &newChildren, pageIsEmpty, absoluteBoxes,
				fixedBoxes, *adjoiningMargins, bottomSpace, positionY, skip,
				firstLetterStyle, firstLineStyle, drawBottomDecoration, maxLines)
			drawBottomDecoration = drawBottomDecoration || resumeAt == nil
			adjoiningMargins = &[]float64{}
			firstLetterStyle, firstLineStyle = nil, nil

		case !stop:
			// Layout in-flow child.
			result := blockInFlowLayout(lc, box, index, child, &newChildren,
				pageIsEmpty, absoluteBoxes, fixedBoxes, adjoiningMargins,
				bottomSpace, positionY, skip, firstLetterStyle, firstLineStyle,
				discard, next, maxLines)
			abort, stop, resumeAt = result.abort, result.stop, result.resumeAt
			positionY, adjoiningMargins, next = result.positionY, result.adjoiningMargins, result.nextPage
			newMaxLines := result.maxLines
			skip = nil
			firstLetterStyle, firstLineStyle = nil, nil

			// Handle max-lines.
			if newMaxLines != noMaxLines && maxLines != noMaxLines {
				maxLines = newMaxLines
				if maxLines <= 0 {
					// Maximum number of lines reached, stop box rendering.
					stop = true
					lastChild := child == box.Children[len(box.Children)-1]
					if !lastChild {
						// This is not the last line, recursively find the
						// last line to set ellipsis on it.
						children := newChildren
						for len(children) > 0 {
							lc.spend(1)
							last := children[len(children)-1]
							if last.Kind == KindLine {
								last.ownExtra().BlockEllipsis = box.Style.Get("block_ellipsis")
							} else if last.IsParent() {
								children = last.Children
								continue
							}
							break
						}
					}
				}
			}
		}

		if abort {
			// Abort the rendering of box.
			page, _ := child.PageValues()
			removePlaceholders(lc, box.Children[skipIndex:], absoluteBoxes, fixedBoxes)
			for _, footnote := range newFootnotes {
				lc.unlayoutFootnote(footnote)
			}
			if box.EstablishesFormattingContext() {
				lc.finishBlockFormattingContext(nil)
			}
			return sharedResult(blockLayoutResult{nextPage: nextPage{brk: "any", page: page}, maxLines: maxLines})
		} else if stop {
			// Stop after the rendering of child.
			if box.Height != Auto {
				boxBottom := box.PositionY + box.BorderHeight()
				bottomMargin := collapseMargin(*adjoiningMargins)
				if overflows(boxBottom, positionY-bottomMargin) {
					// Box height is fixed and it overflows the page, forget
					// overflowing children.
					resumeAt = nil
				}
			}
			adjoiningMargins = &[]float64{}
			stopped = true
			break
		}
	}
	if !stopped {
		resumeAt = nil
	}

	boxIsFragmented := resumeAt != nil || box.ForceFragmentation
	if isStr(box.Style.Get("continue"), "discard") {
		resumeAt = nil
	}

	if boxIsFragmented && avoidPageBreak(str(box.Style.Get("break_inside")), lc) && !pageIsEmpty {
		removePlaceholders(lc, append(slices.Clip(newChildren), box.Children[skipIndex:]...), absoluteBoxes, fixedBoxes)
		if box.EstablishesFormattingContext() {
			lc.finishBlockFormattingContext(nil)
		}
		return sharedResult(blockLayoutResult{nextPage: anyNextPage(), maxLines: maxLines})
	}

	if box.TopMarginCollapses() {
		box.PositionY += collapseMargin(*thisBoxAdjoiningMargins) - box.MarginTop
	}

	// Detect collapsing-through situation and collapse margins accordingly.
	lastInFlowChild := findLastInFlowChild(newChildren)
	collapsingThrough := false
	if lastInFlowChild == nil {
		// No in-flow child in box, collapse its top and bottom margins.
		collapsedMargin := collapseMargin(*adjoiningMargins)
		direction := str(box.Style.Get("direction")) // TODO: should be parent’s one instead
		_, hasClearance := getClearance(lc, box, direction, collapsedMargin)
		if (box.Height == Auto || box.Height == 0) && !hasClearance &&
			box.MinHeight == 0 && box.BorderTopWidth == 0 && box.PaddingTop == 0 &&
			box.BorderBottomWidth == 0 && box.PaddingBottom == 0 {
			// Collapse through the box.
			collapsingThrough = true
		} else {
			// Don’t collapse through the box, update position and reset
			// margins.
			positionY += collapsedMargin
			adjoiningMargins = &[]float64{}
		}
	} else if box.Height != Auto {
		// In-flow children, collapse last child bottom margin and box
		// bottom margin.
		// Not adjoining, reset margins.
		adjoiningMargins = &[]float64{}
	}

	if !box.BottomMarginCollapses() {
		positionY += collapseMargin(*adjoiningMargins)
		adjoiningMargins = &[]float64{}
	}

	// Add block ellipsis
	if boxIsFragmented && len(newChildren) > 0 {
		if lastChild := newChildren[len(newChildren)-1]; lastChild.Kind == KindLine {
			lastChild.ownExtra().BlockEllipsis = box.Style.Get("block_ellipsis")
		}
	}

	newBox := box.CopyWithChildren(newChildren)
	newBox.RemoveDecoration(!isStart, boxIsFragmented && !discard)

	if newBox.Height == Auto {
		newBox.Height = max(0, positionY-newBox.ContentBoxY())
	}

	if isStr(newBox.Style.Get("position"), "relative") {
		// New containing block, resolve the layout of the absolute
		// descendants
		for _, absoluteBox := range *absoluteBoxes {
			absoluteLayout(lc, absoluteBox, newBox, fixedBoxes, bottomSpace, nil)
		}
	}

	for _, child := range newBox.Children {
		relativePositioning(child, sizeBlock(newBox.Width, newBox.Height))
	}

	if box.EstablishesFormattingContext() {
		lc.finishBlockFormattingContext(newBox)
	}

	if discard || !boxIsFragmented {
		// After finish_block_formatting_context which may increment
		// new_box.height
		newBox.Height = max(min(newBox.Height, newBox.MaxHeight), newBox.MinHeight)
	} else if bottomSpace > math.Inf(-1) && !newBox.IsColumn {
		// Make the box fill the blank space at the bottom of the page
		// https://www.w3.org/TR/css-break-3/#box-splitting
		newBoxHeight := lc.pageBottom - bottomSpace - newBox.PositionY -
			(newBox.MarginHeight() - newBox.Height)
		if newBoxHeight > newBox.Height {
			newBox.Height = newBoxHeight
			if drawBottomDecoration {
				newBox.Height += box.PaddingBottom + box.BorderBottomWidth + box.MarginBottom
			}
		}
	}

	if next.page == nil {
		_, next.page = newBox.PageValues()
	}

	return blockResult{blockLayoutResult{
		box:               newBox,
		resumeAt:          resumeAt,
		nextPage:          next,
		collapsingThrough: collapsingThrough,
		maxLines:          maxLines,
	}, adjoiningMargins}
}

// collapseMargin is collapse_margin in weasyprint/layout/block.py: the
// amount of collapsed margin for a list of adjoining margins.
func collapseMargin(adjoiningMargins []float64) float64 {
	// add 0 to make sure that max/min don’t get an empty list
	positive, negative := 0.0, 0.0
	for _, m := range adjoiningMargins {
		if m >= 0 {
			positive = max(positive, m)
		}
		if m <= 0 {
			negative = min(negative, m)
		}
	}
	return positive + negative
}

// addBreakValues is _add_break_values in weasyprint/layout/block.py: the
// values of break-before or break-after of box and of its first or last
// descendants. side is "before" or "after".
func addBreakValues(box *Box, side string) []string {
	blockParallel := func(box *Box) bool {
		return box != nil && (box.IsBlockLevel() || box.Kind == KindTableRowGroup || box.Kind == KindTableRow)
	}
	var values []string
	for blockParallel(box) && (box.IsInNormalFlow() || box.IsFloated()) {
		values = append(values, str(box.Style.Get("break_"+side)))
		if len(box.Children) == 0 {
			break
		}
		if side == "after" {
			box = box.Children[len(box.Children)-1]
		} else {
			box = box.Children[0]
		}
	}
	if side == "after" {
		slices.Reverse(values)
	}
	return values
}

// blockLevelPageBreak is block_level_page_break in
// weasyprint/layout/block.py: the correct page break value between
// siblings. siblingBefore and siblingAfter can be nil for None.
//
// Return the value of “page-break-before“ or “page-break-after“ that
// "wins" for boxes that meet at the margin between two sibling boxes.
//
// For boxes before the margin, the 'page-break-after' value is considered;
// for boxes after the margin the 'page-break-before' value is considered.
//
//   - 'avoid' takes priority over 'auto'
//   - 'page' takes priority over 'avoid' or 'auto'
//   - 'left' or 'right' take priority over 'always', 'avoid' or 'auto'
//   - Among 'left' and 'right', later values in the tree take priority.
func blockLevelPageBreak(siblingBefore, siblingAfter *Box) string {
	// https://drafts.csswg.org/css-break-3/#possible-breaks
	result := "auto"
	values := append(addBreakValues(siblingBefore, "after"), addBreakValues(siblingAfter, "before")...)
	for _, value := range values {
		switch value {
		case "left", "right", "recto", "verso":
			result = value
		case "page", "column":
			switch result {
			case "auto", "avoid", "avoid-page", "avoid-column":
				result = value
			}
		case "avoid", "avoid-page", "avoid-column":
			if result == "auto" {
				result = value
			}
		}
	}

	return result
}

// blockLevelPageName is block_level_page_name in
// weasyprint/layout/block.py: the next page name when siblings don't have
// the same names, or nil.
func blockLevelPageName(siblingBefore, siblingAfter *Box) Value {
	_, beforePage := siblingBefore.PageValues()
	afterPage, _ := siblingAfter.PageValues()
	if !Equal(beforePage, afterPage) {
		return afterPage
	}
	return nil
}

// findEarlierPageBreak is find_earlier_page_break in
// weasyprint/layout/block.py: the last possible page break in children.
//
// Because of a `page-break-before: avoid` or a `page-break-after: avoid` we
// need to find an earlier page break opportunity inside `children`.
//
// Absolute or fixed placeholders removed from children should also be
// removed from `absolute_boxes` or `fixed_boxes`.
//
// It returns the new children and where to resume, and false for None.
func findEarlierPageBreak(lc *layoutContext, children []*Box, absoluteBoxes, fixedBoxes *[]*Box) ([]*Box, skipStack, bool) {
	lc.spend(1)
	if len(children) > 0 && children[0].Kind == KindLine {
		// Normally `orphans` and `widows` apply to the block container, but
		// line boxes inherit them.
		orphans := int(num(children[0].Style.Get("orphans")))
		widows := int(num(children[0].Style.Get("widows")))
		index := len(children) - widows // how many lines we keep
		// Port: upstream fails on an empty list of lines to keep.
		if index < orphans || index <= 0 {
			return nil, nil, false
		}
		newChildren := children[:index:index]
		resumeAt := skipStack{0: newChildren[len(newChildren)-1].resumeAt}
		removePlaceholders(lc, children[index:], absoluteBoxes, fixedBoxes)
		return newChildren, resumeAt, true
	}

	var previousInFlow *Box
	var newChildren []*Box
	var resumeAt skipStack
	found := false
	index := len(children) - 1
	for ; index >= 0; index-- {
		child := children[index]
		if child.Kind == KindTableRowGroup && (child.table().IsHeader || child.table().IsFooter) {
			// We don’t want to break pages before table headers or footers.
			continue
		} else if child.IsColumn {
			// We don’t want to break pages between columns.
			continue
		}

		if child.IsInNormalFlow() {
			pageBreak := blockLevelPageBreak(child, previousInFlow)
			if previousInFlow != nil && !avoidPageBreak(pageBreak, lc) {
				index++ // break after child
				newChildren = slices.Clone(children[:index])
				// Get the index in the original parent
				resumeAt = skipStack{children[index].index: nil}
				found = true
				break
			}
			previousInFlow = child
		}

		avoidBreakInChild := avoidPageBreak(str(child.Style.Get("break_inside")), lc)
		if child.IsInNormalFlow() && !avoidBreakInChild {
			if child.Is(KindBlock) || child.IsTable() || child.Kind == KindTableRowGroup {
				if newGrandChildren, childResumeAt, ok := findEarlierPageBreak(lc, child.Children, absoluteBoxes, fixedBoxes); ok {
					newChild := child.CopyWithChildren(newGrandChildren)
					newChildren = append(slices.Clone(children[:index]), newChild)

					// Re-add footer at the end of split table
					// TODO: fix table height and footer position
					if child.Kind == KindTableRowGroup {
						for _, nextChild := range children[index:] {
							if nextChild.table().IsFooter {
								newChildren = append(newChildren, nextChild)
							}
						}
					}

					// Index in the original parent
					resumeAt = skipStack{newChild.index: childResumeAt}
					index++ // Remove placeholders after child
					found = true
					break
				}
			}
		}
	}
	if !found {
		return nil, nil, false
	}

	// TODO: don’t remove absolute and fixed placeholders found in table
	// footers
	removePlaceholders(lc, children[index:], absoluteBoxes, fixedBoxes)
	return newChildren, resumeAt, true
}

// findLastInFlowChild is find_last_in_flow_child in
// weasyprint/layout/block.py: the last in-flow child of children, or nil.
func findLastInFlowChild(children []*Box) *Box {
	for i := len(children) - 1; i >= 0; i-- {
		if children[i].IsInNormalFlow() {
			return children[i]
		}
	}
	return nil
}

// findLastClassABreakPointChild is find_last_class_a_break_point_child in
// weasyprint/layout/block.py: the last child of children for Class-A break
// points, or nil.
func findLastClassABreakPointChild(children []*Box) *Box {
	// See https://www.w3.org/TR/css-break-4/#btw-blocks.
	for i := len(children) - 1; i >= 0; i-- {
		if children[i].IsInNormalFlow() || children[i].IsFloated() {
			return children[i]
		}
	}
	return nil
}

// removePlaceholders is remove_placeholders in weasyprint/layout/block.py:
// it removes placeholders from absolute and fixed lists.
//
// For boxes that have been removed in find_earlier_page_break(), remove
// the matching placeholders in absolute_boxes and fixed_boxes.
//
// Also takes care of removed footnotes and floats.
func removePlaceholders(lc *layoutContext, boxList []*Box, absoluteBoxes, fixedBoxes *[]*Box) {
	for _, box := range boxList {
		lc.spend(1)
		if len(box.Children) > 0 {
			removePlaceholders(lc, box.Children, absoluteBoxes, fixedBoxes)
		}
		position := box.Style.Get("position")
		if isStr(position, "absolute") && absoluteBoxes != nil {
			if i := slices.Index(*absoluteBoxes, box); i >= 0 {
				*absoluteBoxes = slices.Delete(*absoluteBoxes, i, i+1)
			}
		} else if isStr(position, "fixed") && fixedBoxes != nil {
			if i := slices.Index(*fixedBoxes, box); i >= 0 {
				*fixedBoxes = slices.Delete(*fixedBoxes, i, i+1)
			}
		}
		if box.extra().Footnote != nil {
			lc.unlayoutFootnote(box.extra().Footnote)
		}
		lc.brokenOutOfFlow = slices.DeleteFunc(lc.brokenOutOfFlow, func(item brokenOutOfFlow) bool {
			return item.newBox == box
		})
		if shapes := lc.currentShapes(); shapes != nil {
			if i := slices.Index(shapes.boxes, box); i >= 0 {
				shapes.boxes = slices.Delete(shapes.boxes, i, i+1)
			}
		}
		if box.extra().Note != nil {
			lc.removeRunningElement(box.extra().Note)
		}
	}
}

// avoidPageBreak is avoid_page_break in weasyprint/layout/block.py: it
// reports whether we should avoid breaks.
func avoidPageBreak(pageBreak string, lc *layoutContext) bool {
	if lc.inColumn {
		return pageBreak == "avoid" || pageBreak == "avoid-page" || pageBreak == "avoid-column"
	}
	return pageBreak == "avoid" || pageBreak == "avoid-page"
}

// forcePageBreak is force_page_break in weasyprint/layout/block.py: it
// reports whether we should force breaks.
func forcePageBreak(pageBreak string, lc *layoutContext) bool {
	switch pageBreak {
	case "page", "left", "right", "recto", "verso":
		return true
	case "column":
		return lc.inColumn
	}
	return false
}
