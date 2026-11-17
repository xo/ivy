// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/inline.py: the layout for inline-level
// boxes.

package layout

import (
	"iter"
	"math"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// maxLineTries bounds the tries of get_next_linebox to fit a line between
// the floats, and maxBreakTries bounds the tries of
// _break_waiting_children to break a waiting child.
//
// Port: upstream has no bound.
const (
	maxLineTries  = 1 << 10
	maxBreakTries = 1 << 16
)

// lineChild is an item of the list line_children of upstream: a child of a
// line box and its index in the children of the line box.
type lineChild struct {
	index int
	box   *Box
}

// waitingChild is an item of the lists children and waiting_children of
// split_inline_box: the index of a child, the laid out child, and the
// original child.
type waitingChild struct {
	index    int
	box      *Box
	original *Box
}

// floatWidths is the dict float_widths of upstream, {'left': 0, 'right':
// 0}: the widths of the floats on each side of a line.
type floatWidths struct {
	left, right float64
}

// letterKind is the kind of a first_letter or a last_letter of upstream.
type letterKind uint8

const (
	// letterNone is None.
	letterNone letterKind = iota
	// letterTrue is True: a collapsible space.
	letterTrue
	// letterFalse is False: the end of a leader.
	letterFalse
	// letterText is a string of one character.
	letterText
)

// edgeLetter is a first_letter or a last_letter of upstream: None, True,
// False, or the first or the last character of a box.
type edgeLetter struct {
	kind letterKind
	text string
}

// textLetter returns the edgeLetter of a character.
func textLetter(text string) edgeLetter {
	return edgeLetter{kind: letterText, text: text}
}

// atomicLetter is the letter of an atomic inline: "⺀". See
// https://www.w3.org/TR/css-text-3/#line-breaking. Atomic inlines behave
// like ideographic characters.
var atomicLetter = textLetter("⺀")

// inlineSplit is the tuple that split_inline_level and split_inline_box
// return: (new_box, resume_at, preserved_line_break, first_letter,
// last_letter, float_widths).
type inlineSplit struct {
	box                     *Box
	resumeAt                skipStack
	preservedLineBreak      bool
	firstLetter, lastLetter edgeLetter
	floatWidths             floatWidths
}

// appendBoxes appends boxes to a list that upstream mutates. A nil list
// keeps nothing.
func appendBoxes(list *[]*Box, boxes ...*Box) {
	if list != nil {
		*list = append(*list, boxes...)
	}
}

// styleKeys returns the names of the properties that a style holds, the
// keys of style.items() in upstream, in sorted order.
//
// Port: the keys of a style of upstream are in the order of their
// computation. The order does not change the result.
func styleKeys(style *Style) []string {
	keys := make([]string, 0, len(style.values))
	for key := range style.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// swapExcludedShapes sets the list of the excluded shapes of the current
// formatting context, and returns the previous list. It is the setter of
// context.excluded_shapes, which keeps a FakeList.
func (lc *layoutContext) swapExcludedShapes(list *shapeList) *shapeList {
	n := len(lc.excludedShapesRootBoxes)
	if n == 0 {
		return nil
	}
	root := lc.excludedShapesRootBoxes[n-1]
	previous := lc.excludedShapesLists[root]
	lc.excludedShapesLists[root] = list
	return previous
}

// iterLineBoxes is iter_line_boxes in weasyprint/layout/inline.py: an
// iterator of (line, resume_at). line is a laid-out LineBox with as much
// content as possible that fits in the available width.
func iterLineBoxes(lc *layoutContext, box *Box, positionY, bottomSpace float64, skip skipStack, containingBlock *Box, absoluteBoxes, fixedBoxes *[]*Box, firstLetterStyle, firstLineStyle *Style) iter.Seq2[*Box, skipStack] {
	return func(yield func(*Box, skipStack) bool) {
		resolvePercentages(box, containingBlock)
		if skip == nil {
			resolveOnePercentage(box, "text_indent", containingBlock.Width)
		} else {
			box.textIndent = 0
		}
		for {
			lc.spend(1)
			line, resumeAt := getNextLinebox(
				lc, box, positionY, bottomSpace, skip, containingBlock,
				absoluteBoxes, fixedBoxes, firstLetterStyle, firstLineStyle)
			firstLineStyle = nil
			if line != nil {
				handleLeader(lc, line, containingBlock)
				positionY = line.PositionY + line.Height
			}
			if line == nil {
				return
			}
			// Port: upstream lays out the same line again and again when a
			// line resumes where it starts. The port ends the lines there.
			if resumeAt != nil && skipStackEqual(resumeAt, skip) {
				resumeAt = nil
			}
			if !yield(line, resumeAt) {
				return
			}
			if resumeAt == nil {
				return
			}
			skip = resumeAt
			box.textIndent = 0
			firstLetterStyle = nil
		}
	}
}

// getNextLinebox is get_next_linebox in weasyprint/layout/inline.py: it
// returns the next line from the given linebox, and where the next line
// resumes. line is a new linebox copied from the original one, with
// replaced children, or nil for None.
//
// This function takes care of excluded floating shapes to avoid
// collisions.
func getNextLinebox(lc *layoutContext, linebox *Box, positionY, bottomSpace float64, skip skipStack, containingBlock *Box, absoluteBoxes, fixedBoxes *[]*Box, firstLetterStyle, firstLineStyle *Style) (*Box, skipStack) {
	skip, cont := skipFirstWhitespace(linebox, skip)
	if cont {
		return nil, nil
	}

	skip = firstLetterToBox(linebox, skip, firstLetterStyle)

	linebox.PositionY = positionY

	if len(lc.excludedShapes()) > 0 {
		// Width and height must be calculated to avoid floats
		linebox.Width = inlineMinContentWidth(lc, linebox, true, skip, true, false)
		linebox.Height, _ = strut(linebox.Style)
	} else {
		// No float, width and height will be set by the lines
		linebox.Width, linebox.Height = 0, 0
	}
	positionX, positionY, availableWidth := avoidCollisions(lc, linebox, containingBlock, false)

	candidateHeight := linebox.Height

	excludedShapes := &shapeList{boxes: slices.Clone(lc.excludedShapes())}

	// Port: upstream has no tab stops. They are measured from the start
	// edge of the margin box of the block container of the line.
	defer func(origin float64) { lc.tabOrigin = origin }(lc.tabOrigin)
	lc.tabOrigin = containingBlock.PositionX

	var line *Box
	var resumeAt skipStack
	var linePlaceholders, waitingFloats []*Box
	var lineAbsolutes, lineFixed []*Box
	for tries := 0; ; tries++ {
		originalPositionX := positionX
		originalPositionY := positionY
		linebox.PositionX, linebox.PositionY = positionX, positionY
		originalWidth := linebox.Width
		maxX := positionX + availableWidth
		positionX += linebox.textIndent

		linePlaceholders, waitingFloats = nil, nil
		lineAbsolutes, lineFixed = nil, nil
		var lineChildren []lineChild

		split := splitInlineBox(
			lc, linebox, positionX, maxX, bottomSpace, skip,
			containingBlock, &lineAbsolutes, &lineFixed, &linePlaceholders,
			&waitingFloats, &lineChildren, firstLetterStyle, firstLineStyle)
		line, resumeAt = split.box, split.resumeAt
		preservedLineBreak := split.preservedLineBreak
		alignTabs(line, lc.tabOrigin)
		linebox.Width, linebox.Height = line.Width, line.Height

		if isPhantomLinebox(line) && !preservedLineBreak {
			line.Height = 0
			break
		}

		removeLastWhitespace(lc, line)

		_, _, newAvailableWidth := avoidCollisions(lc, linebox, containingBlock, false)
		offsetX := textAlign(lc, line, newAvailableWidth, resumeAt == nil || preservedLineBreak)
		if isStr(containingBlock.Style.Get("direction"), "rtl") {
			offsetX *= -1
			offsetX -= line.Width
		}

		bottom, top := lineBoxVerticality(line)
		line.Baseline = -top
		line.PositionY = top
		line.Height = bottom - top
		offsetY := positionY - top
		line.MarginTop = 0
		line.MarginBottom = 0

		line.Translate(offsetX, offsetY, false)
		// Avoid floating point errors, as position_y - top + top != position_y
		// Removing this line breaks the position == linebox.position test below
		// See issue #583.
		line.PositionY = positionY

		if line.Height <= candidateHeight {
			break
		}
		candidateHeight = line.Height

		// Port: upstream tries without end. The port stops after
		// maxLineTries tries.
		if tries >= maxLineTries {
			break
		}
		lc.spend(1)

		newExcludedShapes := lc.swapExcludedShapes(excludedShapes)
		positionX, positionY, availableWidth = avoidCollisions(lc, line, containingBlock, false)

		if firstLineStyle != nil {
			firstLineBox := line.CopyWithChildren(line.Children)
			firstLineBox.ElementTag += "::first-line"
			firstLineBox.Style = firstLineBox.Style.Copy()
			for _, key := range styleKeys(firstLineStyle) {
				firstLineBox.Style.Set(key, firstLineStyle.Get(key))
			}
			line.Children = []*Box{firstLineBox}
			adjustLineHeight(firstLineBox)
		}

		var condition bool
		if isStr(containingBlock.Style.Get("direction"), "ltr") {
			condition = positionX == originalPositionX && positionY == originalPositionY
		} else {
			condition = positionX+line.Width == originalPositionX+originalWidth && positionY == originalPositionY
		}
		if condition {
			lc.swapExcludedShapes(newExcludedShapes)
			break
		}
	}

	appendBoxes(absoluteBoxes, lineAbsolutes...)
	appendBoxes(fixedBoxes, lineFixed...)

	for _, placeholder := range linePlaceholders {
		if isStr(placeholder.Style.Get("static_top"), "block") {
			// Port: upstream has no -ivy-static-top. The static position is
			// at the top of the part of the block container on this page,
			// inside its border.
			top := containingBlock.PositionY + containingBlock.MarginTop + containingBlock.BorderTopWidth
			placeholder.Translate(line.PositionX-placeholder.PositionX, top-placeholder.PositionY, false)
			continue
		}
		if tupleHas(placeholder.Style.Specified("display"), "inline") {
			// Inline-level static position:
			placeholder.Translate(0, positionY-placeholder.PositionY, false)
		} else {
			// Block-level static position: at the start of the next line
			placeholder.Translate(
				line.PositionX-placeholder.PositionX,
				positionY+line.Height-placeholder.PositionY, false)
		}
	}

	var floatChildren []*Box
	waitingFloatsY := line.PositionY + line.Height
	for _, waitingFloat := range waitingFloats {
		waitingFloat.PositionY = waitingFloatsY
		newWaitingFloat, waitingFloatResumeAt := floatLayout(
			lc, waitingFloat, containingBlock, absoluteBoxes, fixedBoxes,
			bottomSpace, nil)
		floatChildren = append(floatChildren, newWaitingFloat)
		if len(waitingFloatResumeAt) > 0 {
			lc.addBrokenOutOfFlow(
				newWaitingFloat, waitingFloat, containingBlock,
				waitingFloatResumeAt)
		}
	}
	if len(floatChildren) > 0 {
		line.Children = append(slices.Clip(line.Children), floatChildren...)
	}

	return line, resumeAt
}

// skipFirstWhitespace is skip_first_whitespace in
// weasyprint/layout/inline.py: it returns the skip stack that starts just
// after the removable leading spaces. It returns true for the 'continue'
// of upstream, when there is no text to see from the skip stack.
//
// See https://www.w3.org/TR/CSS21/text.html#white-space-model
func skipFirstWhitespace(box *Box, skip skipStack) (skipStack, bool) {
	var index int
	var nextSkip skipStack
	if skip != nil {
		index, nextSkip = skip.item()
	}

	if box.Kind == KindText {
		// Port: upstream asserts that the skip stack of a text box ends
		// there.
		whiteSpace := box.Style.Get("white_space")
		text := box.Text
		if index >= len(text) {
			// Starting a the end of the TextBox, no text to see: Continue
			//
			// Port: upstream also reads an index past the end of the
			// text. It finds no text either.
			return nil, true
		}
		if isStrIn(whiteSpace, "normal", "nowrap", "pre-line") {
			for index < len(text) && text[index] == ' ' {
				index++
			}
		}
		if index != 0 {
			return skipStack{index: nil}, false
		}
		return nil, false
	}

	if box.Kind == KindLine || box.Kind == KindInline {
		if index == 0 && len(box.Children) == 0 {
			return nil, false
		}
		if index < 0 || index >= len(box.Children) {
			// Port: upstream fails on an index past the children.
			return nil, true
		}
		result, cont := skipFirstWhitespace(box.Children[index], nextSkip)
		if cont {
			index++
			if index >= len(box.Children) {
				return nil, true
			}
			result, _ = skipFirstWhitespace(box.Children[index], nil)
		}
		if index != 0 || result != nil {
			return skipStack{index: result}, false
		}
		return nil, false
	}

	// Port: upstream asserts that the skip stack is None.
	return nil, false
}

// removeLastWhitespace is remove_last_whitespace in
// weasyprint/layout/inline.py: it removes in place the space characters at
// the end of a line.
//
// This also reduces the width and position of the inline parents of the
// modified text.
func removeLastWhitespace(lc *layoutContext, line *Box) {
	var ancestors []*Box
	box := line
	for box.Kind == KindLine || box.Kind == KindInline {
		ancestors = append(ancestors, box)
		if len(box.Children) == 0 {
			return
		}
		box = box.Children[len(box.Children)-1]
	}
	if box.Kind != KindText || !isStrIn(box.Style.Get("white_space"), "normal", "nowrap", "pre-line") {
		return
	}
	newText := strings.TrimRight(box.Text, " ")
	var spaceWidth float64
	if newText != "" {
		if len(newText) == len(box.Text) {
			return
		}
		box.Text = newText
		newBox, _, _, _ := splitTextBoxWidth(lc, box, nil, 0, true)
		if newBox == nil {
			// Port: upstream asserts that the text gives a box.
			return
		}
		spaceWidth = box.Width - newBox.Width
		box.Width = newBox.Width
	} else {
		spaceWidth = box.Width
		box.Width = 0
		box.Text = ""
	}

	// RTL line, the trailing space is at the left of the box. We have to
	// translate the box to align the stripped text with the right edge of
	// the box.
	if box.TextLayout != nil && box.TextLayout.FirstLineDirection%2 != 0 {
		for _, child := range line.Children {
			child.Translate(-spaceWidth, 0, true)
		}
	}

	for _, ancestor := range ancestors {
		ancestor.Width -= spaceWidth
	}

	// TODO: All tabs (U+0009) are rendered as a horizontal shift that
	// lines up the start edge of the next glyph with the next tab stop.
	// Tab stops occur at points that are multiples of 8 times the width
	// of a space (U+0020) rendered in the block's font from the block's
	// starting content edge.

	// TODO: If spaces (U+0020) or tabs (U+0009) at the end of a line have
	// 'white-space' set to 'pre-wrap', UAs may visually collapse them.
}

// firstLetterToBox is first_letter_to_box in weasyprint/layout/inline.py:
// it creates a box for the ::first-letter selector, and returns the new
// skip stack.
func firstLetterToBox(box *Box, skip skipStack, firstLetterStyle *Style) skipStack {
	if firstLetterStyle == nil || len(box.Children) == 0 {
		return skip
	}
	// Some properties must be ignored in first-letter boxes.
	// https://drafts.csswg.org/selectors-3/#application-in-css
	// At least, position is ignored to avoid layout troubles.
	firstLetterStyle.Set("position", String("static"))

	child := box.Children[0]
	if child.Kind == KindText {
		letterStyle := box.Style.Copy()
		for _, key := range styleKeys(firstLetterStyle) {
			letterStyle.Set(key, firstLetterStyle.Get(key))
		}
		if strings.HasSuffix(child.ElementTag, "::first-letter") {
			letterBox := newBox(KindInline, box.ElementTag+"::first-letter", letterStyle, box.Element, []*Box{child})
			box.Children = append([]*Box{letterBox}, box.Children[1:]...)
		} else if child.Text != "" {
			var childSkip skipStack
			if len(skip) > 0 {
				_, childSkip = skip.item()
				if len(childSkip) > 0 {
					index, _ := childSkip.item()
					// Port: upstream slices the text in characters with
					// an index in UTF-8 bytes. The port slices it in
					// bytes, at the start of a character.
					index = min(max(index, 0), len(child.Text))
					for index < len(child.Text) && !utf8.RuneStart(child.Text[index]) {
						index++
					}
					child.Text = child.Text[index:]
					skip = nil
				}
			}
			firstLetter := getFirstLetter(child.Text)
			if firstLetter != "" {
				child.Text = child.Text[len(firstLetter):]
			}
			if strings.TrimLeft(firstLetter, "\n") != "" {
				// "This type of initial letter is similar to an
				// inline-level element if its 'float' property is 'none',
				// otherwise it is similar to a floated element."
				childrenStyle := letterStyle.AnonymousStyle()
				tag := box.ElementTag + "::first-letter"
				var textBox *Box
				if isStr(letterStyle.Get("float"), "none") {
					letterBox := newBox(KindInline, tag, letterStyle, box.Element, nil)
					textBox = newTextBox(tag, childrenStyle, box.Element, firstLetter)
					letterBox.Children = []*Box{textBox}
					box.Children = append([]*Box{letterBox}, box.Children...)
				} else {
					letterBox := newBox(KindBlock, tag, letterStyle, box.Element, nil)
					lineBox := newBox(KindLine, tag, childrenStyle, box.Element, nil)
					letterBox.Children = []*Box{lineBox}
					textBox = newTextBox(tag, childrenStyle, box.Element, firstLetter)
					lineBox.Children = []*Box{textBox}
					box.Children = append([]*Box{letterBox}, box.Children...)
				}
				textBox.ProcessTextTransform()
				if len(skip) > 0 && len(childSkip) > 0 {
					index, _ := skip.item()
					childIndex, grandchildSkip := childSkip.item()
					skip = skipStack{index: {childIndex + 1: grandchildSkip}}
				}
			}
		}
	} else if child.IsParent() {
		var childSkip skipStack
		if len(skip) > 0 {
			_, childSkip = skip.item()
		}
		childSkip = firstLetterToBox(child, childSkip, firstLetterStyle)
		if len(skip) > 0 {
			index, _ := skip.item()
			skip = skipStack{index: childSkip}
		}
	}
	return skip
}

// atomicBox is atomic_box in weasyprint/layout/inline.py: it computes the
// width and the height of the atomic box.
func atomicBox(lc *layoutContext, box *Box, positionX float64, skip skipStack, containingBlock *Box, absoluteBoxes, fixedBoxes *[]*Box) *Box {
	lc.enter()
	defer lc.leave()
	switch {
	case box.IsReplaced():
		box = box.Copy()
		inlineReplacedBoxLayout(box, containingBlock)
		box.Baseline = box.MarginHeight()
	case box.Kind == KindInlineBlock:
		var width, minWidth, maxWidth float64
		if box.IsTableWrapper {
			containingSize := sizeBlock(containingBlock.Width, containingBlock.Height)
			tableWrapperWidth(lc, box, containingSize)
			width, minWidth, maxWidth = box.Width, box.MinWidth, box.MaxWidth
		}
		box = inlineBlockBoxLayout(
			lc, box, positionX, skip, containingBlock,
			absoluteBoxes, fixedBoxes)
		if box.IsTableWrapper {
			box.Width, box.MinWidth, box.MaxWidth = width, minWidth, maxWidth
		}
	default:
		// Port: upstream raises TypeError for another kind of box. The
		// port keeps the box.
	}
	return box
}

// inlineBlockBoxLayout is inline_block_box_layout in
// weasyprint/layout/inline.py.
func inlineBlockBoxLayout(lc *layoutContext, box *Box, positionX float64, skip skipStack, containingBlock *Box, absoluteBoxes, fixedBoxes *[]*Box) *Box {
	lc.enter()
	defer lc.leave()
	resolvePercentages(box, containingBlock)

	// https://www.w3.org/TR/CSS21/visudet.html#inlineblock-width
	if box.MarginLeft == Auto {
		box.MarginLeft = 0
	}
	if box.MarginRight == Auto {
		box.MarginRight = 0
	}
	// https://www.w3.org/TR/CSS21/visudet.html#block-root-margin
	if box.MarginTop == Auto {
		box.MarginTop = 0
	}
	if box.MarginBottom == Auto {
		box.MarginBottom = 0
	}

	inlineBlockWidth(box, lc, containingBlock)

	box.PositionX = positionX
	box.PositionY = 0
	result := blockContainerLayout(
		lc, box, math.Inf(-1), skip, true, absoluteBoxes, fixedBoxes,
		nil, nil, nil, false, noMaxLines)
	if result.box != nil {
		box = result.box
	}
	// Port: upstream always gets a box from block_container_layout here.
	// The port keeps the box before its layout when it gets none.
	box.Baseline = inlineBlockBaseline(box)
	return box
}

// inlineBlockBaseline is inline_block_baseline in
// weasyprint/layout/inline.py: the y position of the baseline for an inline
// block, from the top of its margin box.
//
// https://www.w3.org/TR/CSS21/visudet.html#propdef-vertical-align
func inlineBlockBaseline(box *Box) float64 {
	if box.IsTableWrapper {
		// Inline table's baseline is its first row's baseline
		for _, child := range box.Children {
			if child.IsTable() {
				if len(child.Children) > 0 && len(child.Children[0].Children) > 0 {
					firstRow := child.Children[0].Children[0]
					return firstRow.Baseline
				}
			}
		}
	} else if isStr(box.Style.Get("overflow"), "visible") {
		if result, ok := findInFlowBaseline(box, true); ok && result != 0 {
			return result
		}
	}
	return box.PositionY + box.MarginHeight()
}

// inlineBlockWidth is inline_block_width in weasyprint/layout/inline.py,
// with the decorator handle_min_max_width.
func inlineBlockWidth(box *Box, lc *layoutContext, containingBlock *Box) {
	handleMinMaxWidth(box, func() struct{} {
		availableContentWidth := containingBlock.Width - (box.MarginLeft + box.MarginRight +
			box.BorderLeftWidth + box.BorderRightWidth +
			box.PaddingLeft + box.PaddingRight)
		if box.Width == Auto {
			box.Width = shrinkToFit(lc, box, availableContentWidth)
		}
		return struct{}{}
	})
}

// splitInlineLevel is split_inline_level in weasyprint/layout/inline.py: it
// fits as much content as possible from an inline-level box in a width.
//
// resume_at of the result is nil if all of the content fits. Otherwise it
// can be passed as a skip stack to resume where we left off.
//
// The new box is non-empty (unless the box is empty) and as big as possible
// while respecting maxX, if possible (may overflow is no split is
// possible.)
func splitInlineLevel(lc *layoutContext, box *Box, positionX, maxX, bottomSpace float64, skip skipStack, containingBlock *Box, absoluteBoxes, fixedBoxes, linePlaceholders, waitingFloats *[]*Box, lineChildren *[]lineChild, firstLetterStyle, firstLineStyle *Style) inlineSplit {
	if firstLineStyle != nil {
		box = box.Copy()
		box.Style = box.Style.Copy()
		for _, key := range styleKeys(firstLineStyle) {
			if Inherited[key] {
				box.Style.Set(key, firstLineStyle.Get(key))
			}
		}
		box.ProcessTextTransform()
	}
	resolvePercentages(box, containingBlock)
	var result inlineSplit
	switch {
	case box.Kind == KindText:
		box.PositionX = positionX
		skipBytes := 0
		if skip != nil {
			// Port: upstream asserts that the skip stack of a text box
			// ends there.
			skipBytes, _ = skip.item()
		}

		isLineStart := lineChildren == nil || len(*lineChildren) == 0
		newBox, resumeIndex, hasResume, preservedLineBreak := splitTextBox(
			lc, box, maxX-positionX, skipBytes, isLineStart)
		if newBox != nil && newBox.isTab {
			// Port: upstream has no tab stops.
			lc.placeTab(newBox, positionX)
		}
		result.box = newBox
		result.preservedLineBreak = preservedLineBreak

		if hasResume {
			result.resumeAt = skipStack{resumeIndex: nil}
		}
		if box.Text != "" {
			first, _ := utf8.DecodeRuneInString(box.Text)
			result.firstLetter = textLetter(string(first))
			end := len(box.Text)
			if hasResume {
				end = min(max(resumeIndex, 0), len(box.Text))
			}
			if last, size := utf8.DecodeLastRuneInString(box.Text[:end]); size > 0 {
				result.lastLetter = textLetter(string(last))
			} else {
				// Port: upstream fails to read the last character of an
				// empty text.
				result.lastLetter = edgeLetter{}
			}
		}
	case box.Kind == KindInline:
		if box.MarginLeft == Auto {
			box.MarginLeft = 0
		}
		if box.MarginRight == Auto {
			box.MarginRight = 0
		}
		result = splitInlineBox(
			lc, box, positionX, maxX, bottomSpace, skip,
			containingBlock, absoluteBoxes, fixedBoxes, linePlaceholders,
			waitingFloats, lineChildren, firstLetterStyle, firstLineStyle)
	case box.IsAtomicInlineLevel():
		newBox := atomicBox(
			lc, box, positionX, skip, containingBlock,
			absoluteBoxes, fixedBoxes)
		newBox.PositionX = positionX
		result.box = newBox
		result.firstLetter = atomicLetter
		result.lastLetter = atomicLetter
	case box.Kind == KindInlineFlex || box.Kind == KindInlineGrid:
		box.PositionX = positionX
		box.PositionY = 0
		for _, margin := range []*float64{&box.MarginTop, &box.MarginRight, &box.MarginBottom, &box.MarginLeft} {
			if *margin == Auto {
				*margin = 0
			}
		}
		var layout blockLayoutResult
		if box.Kind == KindInlineFlex {
			layout = flexLayout(
				lc, box, math.Inf(-1), skip, containingBlock, false,
				absoluteBoxes, fixedBoxes, false)
		} else {
			layout = gridLayout(
				lc, box, math.Inf(-1), skip, containingBlock, false,
				absoluteBoxes, fixedBoxes)
		}
		result.box, result.resumeAt = layout.box, layout.resumeAt
		result.firstLetter = atomicLetter
		result.lastLetter = atomicLetter
	default:
		// Port: upstream raises TypeError for another kind of box. The
		// port makes no box, as for an empty text box.
	}
	return result
}

// inlineOutOfFlowLayout is _out_of_flow_layout in
// weasyprint/layout/inline.py: the layout of a child of an inline box that
// is not in the normal flow.
//
// Port: upstream also changes position_x and max_x, which are local
// variables of the function. The caller does not see the changes, so the
// port does not make them.
func inlineOutOfFlowLayout(lc *layoutContext, box, containingBlock *Box, index int, child *Box, children []waitingChild, lineChildren *[]lineChild, waitingChildren *[]waitingChild, waitingFloats, absoluteBoxes, fixedBoxes, linePlaceholders *[]*Box, widths *floatWidths, maxX, positionX, bottomSpace float64) {
	switch {
	case child.IsAbsolutelyPositioned():
		child.PositionX = positionX
		placeholder := newAbsolutePlaceholder(child)
		appendBoxes(linePlaceholders, placeholder)
		*waitingChildren = append(*waitingChildren, waitingChild{index, placeholder, child})
		if isStr(child.Style.Get("position"), "absolute") {
			appendBoxes(absoluteBoxes, placeholder)
		} else {
			appendBoxes(fixedBoxes, placeholder)
		}

	case child.IsFloated():
		child.PositionX = positionX
		floatWidth := shrinkToFit(lc, child, containingBlock.Width)

		// To retrieve the real available space for floats, we must remove
		// the trailing whitespaces from the line
		var lastNonFloating *Box
		for _, item := range slices.Concat(children, *waitingChildren) {
			if !item.box.IsFloated() {
				lastNonFloating = item.box
			}
		}
		if lastNonFloating != nil {
			floatWidth -= trailingWhitespaceSize(lc, lastNonFloating)
		}

		if floatWidth > maxX-positionX || len(*waitingFloats) > 0 {
			// TODO: the absolute and fixed boxes in the floats must be
			// added here, and not in iter_line_boxes
			appendBoxes(waitingFloats, child)
			return
		}
		newChild, floatResumeAt := floatLayout(
			lc, child, containingBlock, absoluteBoxes, fixedBoxes,
			bottomSpace, nil)
		if len(floatResumeAt) > 0 {
			lc.addBrokenOutOfFlow(child, child, containingBlock, floatResumeAt)
		}
		*waitingChildren = append(*waitingChildren, waitingChild{index, newChild, child})
		child = newChild

		// Translate previous line children
		dx := max(child.MarginWidth(), 0)
		float := str(child.Style.Get("float"))
		ltr := isStr(box.Style.Get("direction"), "ltr")
		floatLeft := float == "left"
		floatRight := float == "right"
		if ltr {
			if float == "inline-start" {
				floatLeft = true
			}
			if float == "inline-end" {
				floatRight = true
			}
		} else {
			if float == "inline-start" {
				floatRight = true
			}
			if float == "inline-end" {
				floatLeft = true
			}
		}
		// Port: upstream adds dx to float_widths[float], and fails for
		// inline-start and inline-end. The port adds it to the side of
		// the float.
		switch {
		case floatLeft:
			widths.left += dx
		case floatRight:
			widths.right += dx
		}

		for _, item := range *lineChildren {
			oldChild := item.box
			if !oldChild.IsInNormalFlow() {
				continue
			}
			floatAlign := (floatLeft && ltr) || (floatRight && !ltr)
			if floatAlign {
				oldChild.Translate(dx, 0, false)
			}
		}

	case child.IsRunning() || child.IsNote():
		lc.addRunningElement(child)
	}
}

// breakWaitingChildren is _break_waiting_children in
// weasyprint/layout/inline.py: it breaks a waiting child at its last
// possible breaking point, and returns where the line resumes, or nil for
// None.
//
// Port: upstream takes max_x, and sets it before it reads it. The port
// does not take it.
func breakWaitingChildren(lc *layoutContext, box *Box, bottomSpace float64, initialSkip skipStack, absoluteBoxes, fixedBoxes, linePlaceholders, waitingFloats *[]*Box, lineChildren *[]lineChild, children *[]waitingChild, waitingChildren []waitingChild, firstLetterStyle, firstLineStyle *Style) skipStack {
	if len(waitingChildren) > 0 {
		// Too wide, try to cut inside waiting children, starting from the
		// end.
		// TODO: we should take care of children added into absolute_boxes,
		// fixed_boxes and other lists.
		waitingChildrenCopy := slices.Clone(waitingChildren)
		for len(waitingChildrenCopy) > 0 {
			item := waitingChildrenCopy[len(waitingChildrenCopy)-1]
			waitingChildrenCopy = waitingChildrenCopy[:len(waitingChildrenCopy)-1]
			childIndex, child, originalChild := item.index, item.box, item.original
			if !child.IsInNormalFlow() || !canBreakInside(child) {
				continue
			}

			var childSkip skipStack
			if s, ok := initialSkip[childIndex]; ok {
				childSkip = s
			}

			// Break the waiting child at its last possible breaking point.
			// TODO: The dirty solution chosen here is to decrease the
			// actual size by 1 and render the waiting child again with this
			// constraint. We may find a better way.
			maxX := child.PositionX + child.MarginWidth() - 1
			var split inlineSplit
			found := false
			for tries := 0; maxX > child.PositionX; tries++ {
				// Port: upstream tries each pixel without end. The port
				// stops after maxBreakTries tries.
				if tries >= maxBreakTries {
					break
				}
				lc.spend(1)
				split = splitInlineLevel(
					lc, originalChild, child.PositionX, maxX,
					bottomSpace, childSkip, box, absoluteBoxes,
					fixedBoxes, linePlaceholders, waitingFloats,
					lineChildren, firstLetterStyle, firstLineStyle)
				if len(split.resumeAt) > 0 {
					found = true
					break
				}
				maxX--
			}
			if !found {
				// No line break found
				continue
			}

			*children = append(*children, waitingChildrenCopy...)
			// May be None where we have an empty TextBox.
			//
			// Port: upstream asserts that a child without a box is a text
			// box.
			if split.box != nil {
				*children = append(*children, waitingChild{childIndex, split.box, child})
			}

			return skipStack{childIndex: split.resumeAt}
		}
	}

	if len(*children) > 0 {
		// Too wide, can't break waiting children and the inline is
		// non-empty: put child entirely on the next line.
		return skipStack{(*children)[len(*children)-1].index + 1: nil}
	}
	return nil
}

// adjustLineHeight is _adjust_line_height in weasyprint/layout/inline.py:
// it sets the margins to the half leading to respect the line height.
//
// Also compensate for borders and padding, we want margin_height() ==
// line_height.
func adjustLineHeight(box *Box) {
	var lineHeight float64
	lineHeight, box.Baseline = strut(box.Style)
	box.Height = num(box.Style.Get("font_size"))
	halfLeading := (lineHeight - box.Height) / 2
	box.MarginTop = halfLeading - box.BorderTopWidth - box.PaddingTop
	box.MarginBottom = halfLeading - box.BorderBottomWidth - box.PaddingBottom
}

// splitInlineBox is split_inline_box in weasyprint/layout/inline.py: it
// fits as much content as possible from an inline box in a width.
//
// resume_at of the result is nil if all of the content fits. Otherwise it
// can be passed as a skip stack to resume where we left off.
//
// The new box is non-empty (unless the box is empty) and as big as possible
// while respecting maxX, if possible (may overflow is no split is
// possible.)
//
// This is the recursive step that loops over the children of the box; base
// steps of recursion are handled by splitInlineLevel.
//
// In this phase, excluded floating shapes are ignored.
func splitInlineBox(lc *layoutContext, box *Box, positionX, maxX, bottomSpace float64, skip skipStack, containingBlock *Box, absoluteBoxes, fixedBoxes, linePlaceholders, waitingFloats *[]*Box, lineChildren *[]lineChild, firstLetterStyle, firstLineStyle *Style) inlineSplit {
	lc.enter()
	defer lc.leave()

	// In some cases (shrink-to-fit result being the preferred width)
	// max_x is coming from Pango itself,
	// but floating point errors have accumulated:
	//   width2 = (width + X) - X   # in some cases, width2 < width
	// Increase the value a bit to compensate and not introduce
	// an unexpected line break. The 1e-9 value comes from PEP 485.
	maxX *= 1 + 1e-9

	isStart := skip == nil
	initialPositionX := positionX
	initialSkip := skip
	// Port: upstream asserts that the box is a line box or an inline box.
	leftSpacing := box.PaddingLeft + box.MarginLeft + box.BorderLeftWidth
	rightSpacing := box.PaddingRight + box.MarginRight + box.BorderRightWidth

	var children, waitingChildren []waitingChild
	preservedLineBreak := false
	var firstLetter, lastLetter edgeLetter
	var widths floatWidths
	floatResumeIndex := 0

	if lineChildren == nil {
		lineChildren = &[]lineChild{}
	}
	if isStr(box.Style.Get("position"), "relative") {
		var relativeAbsoluteBoxes []*Box
		absoluteBoxes = &relativeAbsoluteBoxes
	}

	skipIndex := 0
	if !isStart {
		skipIndex, skip = skip.item()
	}

	var resumeAt skipStack
	broken := false
	if skipIndex >= 0 {
		for index := skipIndex; index < len(box.Children); index++ {
			child := box.Children[index]
			child.PositionY = box.PositionY

			if !child.IsInNormalFlow() {
				inlineOutOfFlowLayout(
					lc, box, containingBlock, index, child, children,
					lineChildren, &waitingChildren, waitingFloats,
					absoluteBoxes, fixedBoxes, linePlaceholders, &widths,
					maxX, positionX, bottomSpace)
				if child.IsFloated() {
					floatResumeIndex = index + 1
					if waitingFloats == nil || !slices.Contains(*waitingFloats, child) {
						maxX -= child.MarginWidth()
					}
				}
				continue
			}

			isLastChild := index == len(box.Children)-1
			availableWidth := maxX
			var childWaitingFloats []*Box
			split := splitInlineLevel(
				lc, child, positionX, availableWidth, bottomSpace,
				skip, containingBlock, absoluteBoxes, fixedBoxes,
				linePlaceholders, &childWaitingFloats, lineChildren,
				firstLetterStyle, firstLineStyle)
			var endSpacing float64
			if isStr(box.Style.Get("direction"), "rtl") {
				endSpacing = leftSpacing
				maxX -= split.floatWidths.left
			} else {
				endSpacing = rightSpacing
				maxX -= split.floatWidths.right
			}
			if isLastChild && endSpacing != 0 && split.resumeAt == nil {
				// TODO: we should take care of children added into
				// absolute_boxes, fixed_boxes and other lists.
				availableWidth -= endSpacing
				split = splitInlineLevel(
					lc, child, positionX, availableWidth, bottomSpace,
					skip, containingBlock, absoluteBoxes, fixedBoxes,
					linePlaceholders, &childWaitingFloats, lineChildren,
					firstLetterStyle, firstLineStyle)
			}
			newChild, resume := split.box, split.resumeAt

			widths.left = max(widths.left, split.floatWidths.left)
			widths.right = max(widths.right, split.floatWidths.right)

			skip = nil
			if split.preservedLineBreak {
				preservedLineBreak = true
			}

			canBreak, canBreakSet := false, false
			switch {
			case lastLetter.kind == letterTrue:
				lastLetter = textLetter(" ")
			case lastLetter.kind == letterFalse:
				lastLetter = textLetter(" ") // no-break space
			case isStrIn(box.Style.Get("white_space"), "pre", "nowrap"):
				canBreak, canBreakSet = false, true
			}
			first := split.firstLetter
			if !canBreakSet {
				switch {
				case lastLetter.kind == letterNone || first.kind == letterNone:
					canBreak = false
				case first.kind == letterTrue || first.kind == letterFalse:
					canBreak = first.kind == letterTrue
				default:
					canBreak = CanBreakText(lastLetter.text+first.text, str(child.Style.Get("lang")))
				}
			}

			if canBreak {
				children = append(children, waitingChildren...)
				waitingChildren = nil
			}

			if firstLetter.kind == letterNone {
				firstLetter = first
			}
			if child.TrailingCollapsibleSpace {
				lastLetter = edgeLetter{kind: letterTrue}
			} else {
				lastLetter = split.lastLetter
			}

			// May be None where we have an empty TextBox.
			//
			// Port: upstream asserts that a child without a box is a text
			// box.
			if newChild != nil {
				// Store lines to get previous break points.
				if box.Kind == KindLine {
					*lineChildren = append(*lineChildren, lineChild{index, newChild})
				}

				// Check that text doesn’t overflow.
				newPositionX := newChild.PositionX + newChild.MarginWidth()
				if newPositionX > maxX {
					// Only calculate trailing space size if needed.
					trailingSpace := trailingWhitespaceSize(lc, newChild)
					if newPositionX-trailingSpace > maxX {
						// Text overflows, find previous break point.
						previousResumeAt := breakWaitingChildren(
							lc, containingBlock, bottomSpace,
							initialSkip, absoluteBoxes, fixedBoxes,
							linePlaceholders, waitingFloats, lineChildren, &children,
							waitingChildren, firstLetterStyle, firstLineStyle)
						if len(previousResumeAt) > 0 {
							resumeAt = previousResumeAt
							broken = true
							break
						}
					}
				}

				positionX = newPositionX
				waitingChildren = append(waitingChildren, waitingChild{index, newChild, child})
			}

			appendBoxes(waitingFloats, childWaitingFloats...)
			if resume != nil {
				children = append(children, waitingChildren...)
				resumeAt = skipStack{index: resume}
				broken = true
				break
			}
		}
	}
	if !broken {
		children = append(children, waitingChildren...)
		resumeAt = nil
	}

	// Reorder inline blocks when direction is right-to-left.
	if isStr(box.Style.Get("direction"), "rtl") && len(children) > 1 {
		var inFlowChildren []*Box
		for _, item := range children {
			if item.box.IsInNormalFlow() {
				inFlowChildren = append(inFlowChildren, item.box)
			}
		}
		// Port: upstream fails when no child is in the normal flow.
		if len(inFlowChildren) > 0 {
			positionX := inFlowChildren[0].PositionX
			for _, child := range slices.Backward(inFlowChildren) {
				child.Translate(positionX-child.PositionX, 0, true)
				positionX += child.MarginWidth()
			}
		}
	}

	// Remove decoration.
	isEnd := resumeAt == nil
	newChildren := make([]*Box, len(children))
	for i, item := range children {
		newChildren[i] = item.box
	}
	newBox := box.CopyWithChildren(newChildren)
	newBox.RemoveDecoration(!isStart, !isEnd)

	// Translate boxes to respect box decoration and floats.
	if box.Kind != KindLine {
		newBox.PositionX = initialPositionX
		var decorationSide bool
		if isStr(box.Style.Get("direction"), "ltr") {
			decorationSide = isStart
		} else {
			decorationSide = isEnd
		}
		leftDecoration := leftSpacing != 0 && (isStr(box.Style.Get("box_decoration_break"), "clone") || decorationSide)
		if leftDecoration {
			for _, child := range newBox.Children {
				child.Translate(leftSpacing, 0, false)
			}
		}
		newBox.Translate(widths.left, 0, true)
	}

	// Reset line box width according to its new children.
	newBox.Width = 0
	reverse := isStr(newBox.Style.Get("direction"), "ltr")
	for i := range newBox.Children {
		child := newBox.Children[i]
		if reverse {
			child = newBox.Children[len(newBox.Children)-1-i]
		}
		if child.IsInNormalFlow() {
			newBox.Width = child.PositionX + child.MarginWidth() - newBox.ContentBoxX()
			break
		}
	}

	adjustLineHeight(newBox)

	if isStr(newBox.Style.Get("position"), "relative") {
		for _, absoluteBox := range *absoluteBoxes {
			absoluteLayout(lc, absoluteBox, newBox, fixedBoxes, bottomSpace, nil)
		}
	}

	if resumeAt != nil {
		index, _ := resumeAt.item()
		if index < floatResumeIndex {
			resumeAt = skipStack{floatResumeIndex: nil}
		}
	}

	if box.IsLeader {
		firstLetter = edgeLetter{kind: letterTrue}
		lastLetter = edgeLetter{kind: letterFalse}
	}

	return inlineSplit{
		box:                newBox,
		resumeAt:           resumeAt,
		preservedLineBreak: preservedLineBreak,
		firstLetter:        firstLetter,
		lastLetter:         lastLetter,
		floatWidths:        widths,
	}
}

// splitTextBox is split_text_box in weasyprint/layout/inline.py: it keeps
// as much text as possible from a TextBox in a limited width.
//
// Try not to overflow but always have some text in the new box.
//
// It returns the new box, nil for None, the number of UTF-8 bytes to skip
// from the start of the TextBox for the next line, with false for None if
// all of the text fits, and whether a preserved line break ends the line.
//
// Also break on preserved line breaks.
//
// Port: an infinite available width is the None of upstream, an
// unconstrained width.
func splitTextBox(lc *layoutContext, box *Box, availableWidth float64, skip int, isLineStart bool) (newBox *Box, resumeIndex int, hasResume, preservedLineBreak bool) {
	if math.IsInf(availableWidth, 1) {
		return splitTextBoxWidth(lc, box, nil, skip, isLineStart)
	}
	return splitTextBoxWidth(lc, box, &availableWidth, skip, isLineStart)
}

// splitTextBoxWidth is splitTextBox with an available width that can be
// nil for None.
func splitTextBoxWidth(lc *layoutContext, box *Box, availableWidth *float64, skip int, isLineStart bool) (newBox *Box, resumeIndex int, hasResume, preservedLineBreak bool) {
	lc.spend(1)
	// Port: upstream asserts that the box is a text box.
	fontSize := num(box.Style.Get("font_size"))
	// Port: upstream reads an empty text for a skip past the end. The port
	// also skips to the start of a character.
	skip = min(max(skip, 0), len(box.Text))
	for skip < len(box.Text) && !utf8.RuneStart(box.Text[skip]) {
		skip++
	}
	text := box.Text[skip:]
	if fontSize == 0 || text == "" {
		return nil, 0, false, false
	}
	m := SplitFirstLine(text, box.Style, availableWidth, box.JustificationSpacing, isLineStart, false)
	resumeIndex, hasResume = m.ResumeIndex, m.HasResume
	if hasResume && resumeIndex <= 0 {
		// Port: upstream asserts that the resume index is not 0. The port
		// skips one character, so that the next line goes on.
		_, size := utf8.DecodeRuneInString(text)
		resumeIndex = size
	}
	length := m.Length

	if length > 0 {
		box = box.CopyWithText(m.Layout.Text)
		box.Width = m.Width
		box.TextLayout = m.Layout
		// "The height of the content area should be based on the font,
		//  but this specification does not specify how."
		// https://www.w3.org/TR/CSS21/visudet.html#inline-non-replaced
		// We trust Pango and use the height of the LayoutLine.
		box.Height = m.Height
		// "only the 'line-height' is used when calculating the height
		//  of the line box."
		// Set margins so that margin_height() == line_height
		lineHeight, _ := strut(box.Style)
		halfLeading := (lineHeight - m.Height) / 2
		box.MarginTop = halfLeading
		box.MarginBottom = halfLeading
		// form the top of the content box
		box.Baseline = m.Baseline
		// form the top of the margin box
		box.Baseline += box.MarginTop
	} else {
		box = nil
	}

	if !hasResume {
		preservedLineBreak = false
	} else {
		start, end := min(max(length, 0), len(text)), min(resumeIndex, len(text))
		between := ""
		if start < end {
			between = text[start:end]
		}
		preservedLineBreak = length != resumeIndex && strings.Trim(between, " ") != ""
		// See https://unicode.org/reports/tr14/
		// \r is already handled by process_whitespace
		//
		// Port: upstream asserts that the text between the lines is a
		// line break: '\n', '\t', '\f', '\u0085', ' ' or ' '.
		resumeIndex += skip
	}

	return box, resumeIndex, hasResume, preservedLineBreak
}

// lineBoxVerticality is line_box_verticality in
// weasyprint/layout/inline.py: it handles vertical-align within a LineBox.
//
// It places all boxes vertically assuming that the baseline of box is at
// y = 0, and returns (max_y, min_y), the maximum and minimum vertical
// position of margin boxes.
func lineBoxVerticality(box *Box) (maxY, minY float64) {
	var topBottomSubtrees []*Box
	maxY, minY = alignedSubtreeVerticality(box, &topBottomSubtrees, 0)
	type subtreeMinMax struct {
		subtree      *Box
		maxY, minY   float64
		hasPositions bool
	}
	// Port: the subtrees that the loop finds while it goes are appended to
	// the list, as the comprehension of upstream reads them.
	var subtreesWithMinMax []subtreeMinMax
	for i := 0; i < len(topBottomSubtrees); i++ {
		subtree := topBottomSubtrees[i]
		if subtree.IsFloated() {
			subtreesWithMinMax = append(subtreesWithMinMax, subtreeMinMax{subtree: subtree})
		} else {
			subMaxY, subMinY := alignedSubtreeVerticality(subtree, &topBottomSubtrees, 0)
			subtreesWithMinMax = append(subtreesWithMinMax, subtreeMinMax{subtree, subMaxY, subMinY, true})
		}
	}

	if len(subtreesWithMinMax) > 0 {
		var subPositions []float64
		for _, item := range subtreesWithMinMax {
			if !item.subtree.IsFloated() {
				subPositions = append(subPositions, item.maxY-item.minY)
			}
		}
		if len(subPositions) > 0 {
			highestSub := slices.Max(subPositions)
			maxY = max(maxY, minY+highestSub)
		}
	}

	for _, item := range subtreesWithMinMax {
		var dy float64
		switch {
		case item.subtree.IsFloated():
			dy = minY - item.subtree.PositionY
		case isStr(item.subtree.Style.Get("vertical_align"), "top"):
			dy = minY - item.minY
		default:
			// Port: upstream asserts that vertical-align is bottom.
			dy = maxY - item.maxY
		}
		translateSubtree(item.subtree, dy)
	}
	return maxY, minY
}

// translateSubtree is translate_subtree in weasyprint/layout/inline.py.
func translateSubtree(box *Box, dy float64) {
	if box.Kind == KindInline {
		box.PositionY += dy
		if isStrIn(box.Style.Get("vertical_align"), "top", "bottom") {
			for _, child := range box.Children {
				translateSubtree(child, dy)
			}
		}
	} else {
		// Text or atomic boxes
		box.Translate(0, dy, false)
	}
}

// alignedSubtreeVerticality is aligned_subtree_verticality in
// weasyprint/layout/inline.py.
func alignedSubtreeVerticality(box *Box, topBottomSubtrees *[]*Box, baselineY float64) (maxY, minY float64) {
	maxY, minY, ok := inlineBoxVerticality(box, topBottomSubtrees, baselineY)
	// Account for the line box itself:
	top := baselineY - box.Baseline
	bottom := top + box.MarginHeight()
	if !ok || top < minY {
		minY = top
	}
	if !ok || bottom > maxY {
		maxY = bottom
	}

	return maxY, minY
}

// inlineBoxVerticality is inline_box_verticality in
// weasyprint/layout/inline.py: it handles vertical-align within an
// InlineBox.
//
// It places all boxes vertically assuming that the baseline of box is at
// y = baselineY, and returns (max_y, min_y), the maximum and minimum
// vertical position of margin boxes, with false for (None, None).
func inlineBoxVerticality(box *Box, topBottomSubtrees *[]*Box, baselineY float64) (maxY, minY float64, ok bool) {
	if box.Kind != KindLine && box.Kind != KindInline {
		return 0, 0, false
	}

	for _, child := range box.Children {
		if !child.IsInNormalFlow() {
			if child.IsFloated() {
				*topBottomSubtrees = append(*topBottomSubtrees, child)
			}
			continue
		}
		verticalAlign := child.Style.Get("vertical_align")
		var childBaselineY float64
		switch {
		case isStr(verticalAlign, "baseline"):
			childBaselineY = baselineY
		case isStr(verticalAlign, "middle"):
			oneEx := num(box.Style.Get("font_size")) * characterRatio(box.Style, "ex")
			top := baselineY - (oneEx+child.MarginHeight())/2
			childBaselineY = top + child.Baseline
		case isStr(verticalAlign, "text-top"):
			// align top with the top of the parent’s content area
			top := baselineY - box.Baseline + box.MarginTop +
				box.BorderTopWidth + box.PaddingTop
			childBaselineY = top + child.Baseline
		case isStr(verticalAlign, "text-bottom"):
			// align bottom with the bottom of the parent’s content area
			bottom := baselineY - box.Baseline + box.MarginTop +
				box.BorderTopWidth + box.PaddingTop + box.Height
			childBaselineY = bottom - child.MarginHeight() + child.Baseline
		case isStrIn(verticalAlign, "top", "bottom"):
			// TODO: actually implement vertical-align: top and bottom
			// Later, we will assume for this subtree that its baseline
			// is at y=0.
			childBaselineY = 0
		case checkMath(verticalAlign):
			height, _ := strut(box.Style)
			value, _ := percentageFloat(verticalAlign, box.Style, height)
			childBaselineY = baselineY - value
		default:
			if _, pending := verticalAlign.(*Pending); pending {
				// Port: percentage of upstream resolves a pending value.
				// The port has none after the computation.
				childBaselineY = baselineY
			} else {
				// Numeric value: The child’s baseline is `vertical_align`
				// above (lower y) the parent’s baseline.
				childBaselineY = baselineY - num(verticalAlign)
			}
		}

		// the child’s `top` is `child.baseline` above (lower y) its baseline.
		top := childBaselineY - child.Baseline
		switch child.Kind {
		case KindInlineBlock, KindInlineFlex, KindInlineGrid:
			// This also includes table wrappers for inline tables.
			child.Translate(0, top-child.PositionY, false)
		default:
			child.PositionY = top
			// grand-children for inline boxes are handled below
		}

		if isStrIn(verticalAlign, "top", "bottom") {
			// top or bottom are special, they need to be handled in
			// a later pass.
			*topBottomSubtrees = append(*topBottomSubtrees, child)
			continue
		}

		bottom := top + child.MarginHeight()
		if !ok || top < minY {
			minY = top
		}
		if !ok || bottom > maxY {
			maxY = bottom
		}
		ok = true
		if child.Kind == KindInline {
			childrenMaxY, childrenMinY, childrenOK := inlineBoxVerticality(
				child, topBottomSubtrees, childBaselineY)
			if childrenOK && childrenMinY < minY {
				minY = childrenMinY
			}
			if childrenOK && childrenMaxY > maxY {
				maxY = childrenMaxY
			}
		}
	}
	return maxY, minY, ok
}

// textAlign is text_align in weasyprint/layout/inline.py: the horizontal
// offset of the line for text-align.
func textAlign(lc *layoutContext, line *Box, availableWidth float64, last bool) float64 {
	// "When the total width of the inline-level boxes on a line is less
	// than the width of the line box containing them, their horizontal
	// distribution within the line box is determined by the 'text-align'
	// property."
	if line.Width >= availableWidth {
		return 0
	}

	align := str(line.Style.Get("text_align_all"))
	if last {
		alignLast := str(line.Style.Get("text_align_last"))
		if alignLast != "auto" {
			align = alignLast
		}
	}
	spaceCollapse := isStrIn(line.Style.Get("white_space"), "normal", "nowrap", "pre-line")
	if align == "left" || align == "right" {
		if (align == "left") != isStr(line.Style.Get("direction"), "rtl") {
			align = "start"
		} else {
			align = "end"
		}
	}
	if align == "start" {
		return 0
	}
	offset := availableWidth - line.Width
	switch align {
	case "justify":
		if spaceCollapse {
			// Justification of texts where white space is not collapsing
			// is
			// - forbidden by CSS 2, and
			// - not required by CSS 3 Text.
			justifyLine(lc, line, offset)
		} else if isStr(line.Style.Get("white_space"), "pre-wrap") {
			// Port: a text of pre-wrap is justified too, without the
			// spaces at the end of the line, which hang. The documents of
			// Word keep their spaces with pre-wrap, and justify.
			if n := countExpandableSpaces(line) - trailingSpaces(line); n > 0 {
				addWordSpacing(lc, line, offset/float64(n), 0)
			}
		}
		return 0
	case "center":
		return offset / 2
	}
	// Port: upstream asserts that the alignment is end.
	return offset
}

// justifyLine is justify_line in weasyprint/layout/inline.py.
func justifyLine(lc *layoutContext, line *Box, extraWidth float64) {
	// TODO: We should use a better algorithm here, see
	// https://www.w3.org/TR/css-text-3/#justify-algos
	if nbSpaces := countExpandableSpaces(line); nbSpaces > 0 {
		addWordSpacing(lc, line, extraWidth/float64(nbSpaces), 0)
	}
}

// trailingSpaces returns the number of spaces at the end of the last text
// of the line box or the inline box box.
func trailingSpaces(box *Box) int {
	for i := len(box.Children) - 1; i >= 0; i-- {
		child := box.Children[i]
		if !child.IsInNormalFlow() {
			continue
		}
		switch child.Kind {
		case KindText:
			return len(child.Text) - len(strings.TrimRight(child.Text, " "))
		case KindInline:
			return trailingSpaces(child)
		}
		return 0
	}
	return 0
}

// countExpandableSpaces is count_expandable_spaces in
// weasyprint/layout/inline.py: it counts the expandable spaces (space and
// nbsp) for the justification.
func countExpandableSpaces(box *Box) int {
	switch box.Kind {
	case KindText:
		// TODO: remove trailing spaces correctly
		return strings.Count(box.Text, " ") + strings.Count(box.Text, " ")
	case KindLine, KindInline:
		n := 0
		for _, child := range box.Children {
			n += countExpandableSpaces(child)
		}
		return n
	}
	return 0
}

// addWordSpacing is add_word_spacing in weasyprint/layout/inline.py.
func addWordSpacing(lc *layoutContext, box *Box, justificationSpacing, xAdvance float64) float64 {
	switch box.Kind {
	case KindText:
		box.JustificationSpacing = justificationSpacing
		box.PositionX += xAdvance
		nbSpaces := countExpandableSpaces(box)
		if nbSpaces > 0 {
			layout := CreateLayout(box.Text, box.Style, nil, box.JustificationSpacing)
			layout.Deactivate()
			extraSpace := justificationSpacing * float64(nbSpaces)
			xAdvance += extraSpace
			box.Width += extraSpace
			box.TextLayout = layout
		}
	case KindLine, KindInline:
		box.PositionX += xAdvance
		previousXAdvance := xAdvance
		children := box.Children
		rtl := isStr(box.Style.Get("direction"), "rtl")
		for i := range children {
			child := children[i]
			if rtl {
				child = children[len(children)-1-i]
			}
			if child.IsInNormalFlow() {
				xAdvance = addWordSpacing(lc, child, justificationSpacing, xAdvance)
			}
		}
		box.Width += xAdvance - previousXAdvance
	default:
		// Atomic inline-level box
		box.Translate(xAdvance, 0, false)
	}
	return xAdvance
}

// isPhantomLinebox is is_phantom_linebox in weasyprint/layout/inline.py.
func isPhantomLinebox(linebox *Box) bool {
	// See https://www.w3.org/TR/CSS21/visuren.html#phantom-line-box
	for _, child := range linebox.Children {
		if child.Kind == KindInline {
			if !isPhantomLinebox(child) {
				return false
			}
			for _, side := range [...]string{"top", "right", "bottom", "left"} {
				if margin, ok := child.Style.Get("margin_" + side).(Dimension); ok && margin.Value != 0 {
					return false
				}
				if num(child.Style.Get("border_"+side+"_width")) != 0 {
					return false
				}
				// Port: upstream reads the attribute value of a padding
				// that is a math function, and fails. The port counts it.
				padding, ok := child.Style.Get("padding_" + side).(Dimension)
				if !ok || padding.Value != 0 {
					return false
				}
			}
		} else if child.IsInNormalFlow() {
			return false
		}
	}
	return true
}

// canBreakInside is can_break_inside in weasyprint/layout/inline.py.
func canBreakInside(box *Box) bool {
	// See https://www.w3.org/TR/css-text-3/#white-space-property
	textWrap := isStrIn(box.Style.Get("white_space"), "normal", "pre-wrap", "pre-line")
	switch {
	case box.IsAtomicInlineLevel():
		return false
	case textWrap && box.Kind == KindText:
		return CanBreakText(box.Text, str(box.Style.Get("lang")))
	case textWrap && box.IsParent():
		for _, child := range box.Children {
			if canBreakInside(child) {
				return true
			}
		}
	}
	return false
}
