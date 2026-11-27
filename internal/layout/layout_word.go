// This file is not a port. It holds the three rules of the layout of a word
// processor that D56 adds, which CSS cannot give:
//
//   - -ivy-line-numbers draws the numbers of the lines of a section in the
//     margin, as Word does, see numberLines.
//   - -ivy-line-multiple puts the line gap of a single line above the
//     text of a line, and the extra space of a multiple line spacing below
//     it, see leadingShift.
//   - -ivy-page-top-space leaves out the space before the first block of a
//     page, of a column or of the part of a split table cell after a break
//     that is not forced, see layoutContext.discardsSpaceBefore.

package layout

import (
	"strconv"
	"strings"
)

// maxLineNumber bounds the step and the first number of -ivy-line-numbers.
// Word takes at most 100 for the step and 32767 for the first number.
const maxLineNumber = 32767

// lineNumbers is the computed value of -ivy-line-numbers, or the zero value
// for 'none'.
type lineNumbers struct {
	// distance is the distance from the end of a number to the start of
	// the content of the box that declares the numbers, in pixels.
	distance float64
	// step shows the numbers that are a multiple of it, and first is the
	// number of the first line.
	step, first int
	// restart is 'page', 'section' or 'continuous'.
	restart string
	// side is 'left', 'right', 'inner' or 'outer': the side of the
	// content where the numbers go. Inner is the left on a right page.
	side string
	// noEmpty is set when the empty lines do not count, and reset when
	// the first line of the box takes the number first, in the section
	// around the box. objectBottom is set when the number of a line that
	// holds only atomic boxes goes at the bottom of those boxes.
	noEmpty, reset, objectBottom bool
}

// lineNumbersOf returns the value of -ivy-line-numbers of a style, and false
// for 'none'.
func lineNumbersOf(style *Style) (lineNumbers, bool) {
	t, ok := style.Get("line_numbers").(Tuple)
	if !ok || len(t) != 8 {
		return lineNumbers{}, false
	}
	distance, ok1 := t[0].(Float)
	step, ok2 := t[1].(Int)
	first, ok3 := t[2].(Int)
	restart, ok4 := t[3].(String)
	side, ok5 := t[4].(String)
	empty, ok6 := t[5].(String)
	reset, ok7 := t[6].(String)
	object, ok8 := t[7].(String)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 || !ok7 || !ok8 || step < 1 {
		return lineNumbers{}, false
	}
	return lineNumbers{
		distance: float64(distance), step: int(step), first: int(first), restart: string(restart),
		side: string(side), noEmpty: empty != "", reset: reset != "", objectBottom: object != "",
	}, true
}

// lineNumberState is the state of the numbers of the lines at the start of
// a page: the number of the last line, and the element of the box that
// declares the numbers of that line, the section. started is false before
// the first numbered line of the document.
type lineNumberState struct {
	last    int
	section *Element
	started bool
	// reset is the element of the last box with 'reset', whose part on
	// the next page does not reset the number again.
	reset *Element
}

// numberLines numbers the lines of the page, whose root box is root, from
// the state at the start of the page, and returns the state at its end. A
// line is numbered when its block has a value of -ivy-line-numbers other
// than 'none'. Only the line boxes of the in-flow blocks of the root box
// count, as in Word: the lines of a table, a float, a flex or grid item,
// an absolutely positioned box, a margin box or a footnote do not. With
// 'no-empty', a line without text and without an atomic box does not
// count, as LibreOffice does for an empty paragraph. A box with 'reset'
// gives the first number to its first line, and stays in the section
// around it. The number of a line whose number is a multiple of the step
// is drawn, with its baseline at the baseline of the line, or with
// 'object-bottom', at the bottom of the atomic boxes of a line that holds
// no text, see objectBottom. On the left, the
// number ends at the distance before the start of the content of the box
// that declares the numbers. On the right, it starts at the distance after
// the end of that content. rightPage is set on a right page, where 'inner'
// is the left and 'outer' the right. The numbers are text boxes in a block
// that is the last child of root.
func numberLines(lc *layoutContext, root *Box, state lineNumberState, rightPage bool) lineNumberState {
	var lines []*Box
	// pageStart is set before the first numbered line of the page, and
	// reset while a box with 'reset' waits for its first line.
	pageStart, reset := true, false
	var walk func(box, section *Box, parent lineNumbers, parentOn bool)
	walk = func(box, section *Box, parent lineNumbers, parentOn bool) {
		value, on := lineNumbersOf(box.Style)
		if on && (!parentOn || value != parent) {
			// the box declares the numbers: it starts a section, or
			// sets the number of its first line in the section around it
			if !value.reset || section == nil {
				section = box
			}
			if value.reset && box.Element != state.reset {
				reset, state.reset = true, box.Element
				defer func() { reset = false }()
			}
		}
		for _, child := range box.Children {
			switch {
			case child.Kind == KindLine:
				// a row next to a float of -ivy-wrap-flow has one number,
				// on its first part (D63)
				if !on || child.Height <= 0 || child.extra().rowPart || value.noEmpty && emptyLine(child) {
					continue
				}
				lc.spend(1)
				switch {
				case reset,
					!state.started,
					value.restart == "page" && pageStart,
					value.restart == "section" && section.Element != state.section:
					state.last = value.first
				default:
					state.last++
				}
				reset = false
				state.started, state.section, pageStart = true, section.Element, false
				if state.last%value.step == 0 {
					if line := numberLine(child, section, value, state.last, rightPage); line != nil {
						lines = append(lines, line)
					}
				}
			case child.Kind == KindBlock && child.IsInNormalFlow() && !child.IsTableWrapper && !child.IsRunning():
				walk(child, section, value, on)
			}
		}
	}
	walk(root, nil, lineNumbers{}, false)
	if len(lines) > 0 {
		block := anonymousFrom(KindBlock, root, lines)
		block.PositionX, block.PositionY = root.PositionX, root.PositionY
		root.Children = append(root.Children, block)
	}
	return state
}

// emptyLine reports whether the line box line holds no text other than
// spaces, and no atomic box, such as an image or an inline block. A float
// or an absolutely positioned box is not in the line.
func emptyLine(line *Box) bool {
	for _, child := range line.Children {
		switch {
		case child.IsFloated() || child.IsAbsolutelyPositioned():
		case child.Kind == KindText:
			if strings.TrimSpace(child.Text) != "" {
				return false
			}
		case child.Kind != KindInline || !emptyLine(child):
			return false
		}
	}
	return true
}

// objectBottom returns the bottom of the border boxes of the atomic boxes
// of the line box line, such as pictures, and false when the line holds
// text other than spaces or no atomic box. A float or an absolutely
// positioned box is not in the line. In LibreOffice, a line that holds
// only an object has no descent, and its baseline is at the bottom of the
// object, where LibreOffice draws its number. A format can put such an
// object at the top of its line, so that the line has no descent, which
// moves the baseline of the line to the top.
func objectBottom(line *Box) (float64, bool) {
	bottom, found := 0.0, false
	var walk func(box *Box) bool
	walk = func(box *Box) bool {
		for _, child := range box.Children {
			switch {
			case child.IsFloated() || child.IsAbsolutelyPositioned():
			case child.Kind == KindText:
				if strings.TrimSpace(child.Text) != "" {
					return false
				}
			case child.Kind == KindInline:
				if !walk(child) {
					return false
				}
			default:
				b := child.BorderBoxY() + child.BorderHeight()
				if !found || b > bottom {
					bottom = b
				}
				found = true
			}
		}
		return true
	}
	if !walk(line) || !found {
		return 0, false
	}
	return bottom, true
}

// numberLine returns a line box with the number n of the line box line,
// in the style of the box section that declares the numbers, or nil when
// the number has no text. rightPage is set on a right page.
func numberLine(line, section *Box, value lineNumbers, n int, rightPage bool) *Box {
	style := section.Style.AnonymousStyle()
	text := strconv.Itoa(n)
	m := SplitFirstLine(text, style, nil, 0, true, false)
	if m.Layout == nil {
		return nil
	}
	textBox := newTextBox(section.ElementTag, style, section.Element, m.Layout.Text)
	textBox.TextLayout = m.Layout
	textBox.Width, textBox.Height, textBox.Baseline = m.Width, m.Height, m.Baseline
	right := value.side == "right" || value.side == "inner" && !rightPage || value.side == "outer" && rightPage
	if right {
		textBox.PositionX = section.ContentBoxX() + section.Width + value.distance
	} else {
		textBox.PositionX = section.ContentBoxX() - value.distance - m.Width
	}
	baseline := line.PositionY + line.Baseline
	if bottom, ok := objectBottom(line); ok && value.objectBottom {
		baseline = bottom
	}
	textBox.PositionY = baseline - m.Baseline
	box := anonymousFrom(KindLine, section, []*Box{textBox})
	box.PositionX, box.PositionY = textBox.PositionX, textBox.PositionY
	box.Width, box.Height, box.Baseline = m.Width, m.Height, m.Baseline
	return box
}

// leadingShift returns how far -ivy-line-multiple moves the text of a box
// of style up in its line, whose height is lineHeight. textHeight is the
// height of the text of the box, from its ascent to its descent. A line of
// a word processor with a multiple line spacing m is m times as high as a
// single line. Word and LibreOffice put the whole line gap of a single
// line, of the height lineHeight / m, above the text, and add the rest of
// the line below the text. CSS puts half of the leading above the text and
// half below. So the top of the text moves from (lineHeight - textHeight) /
// 2 to lineHeight/m - textHeight. It stays between the top of the line and
// the place where the text ends at the bottom of the line. A single line,
// of the multiple 1, has its whole leading above the text. A multiple
// below 1, and a line that is not higher than its text, keep the rule of
// CSS. The shift is negative when the text moves down.
func leadingShift(style *Style, lineHeight, textHeight float64) float64 {
	m, _, ok := lineMultipleOf(style)
	if !ok || m < 1 || lineHeight <= textHeight {
		return 0
	}
	leading := lineHeight - textHeight
	return leading/2 - min(max(lineHeight/m-textHeight, 0), leading)
}

// lineMultipleOf returns the multiple of -ivy-line-multiple of a style,
// whether the value has 'last', and false for 'none'.
func lineMultipleOf(style *Style) (m float64, last, ok bool) {
	switch v := style.Get("line_multiple").(type) {
	case Float:
		return float64(v), false, true
	case Tuple:
		if len(v) == 2 {
			if m, ok := v[0].(Float); ok {
				return float64(m), isStr(v[1], "last"), true
			}
		}
	}
	return 0, false, false
}

// lastLineSpace returns the extra space below the text of the line box
// line of box that 'last' of -ivy-line-multiple leaves out, or 0. The
// extra space of a line of a multiple m above 1 is the part of its height
// above a single line, its height times (1 - 1/m), see leadingShift.
// LibreOffice leaves it out below the last line of a page in the flow of
// the page, so that line stays on the page when its text fits. When the
// setting AddParaLineSpacingToTableCells of the document is false, it
// also leaves it out below the last line of each part of a table cell,
// see lastLineCut.
func lastLineSpace(box, line *Box) float64 {
	m, last, ok := lineMultipleOf(box.Style)
	if !ok || !last || m <= 1 || line.Height <= 0 || line.extra().lastCut {
		return 0
	}
	return line.Height * (1 - 1/m)
}

// lastLineCut takes the extra space of 'last' of -ivy-line-multiple away
// from the last line box of the block container box, the last line of a
// part of a table cell, and returns the height that box loses. The line
// is in the last in-flow child of box, or in the last in-flow child of
// that child, and so on, through blocks whose height is 'auto'. Each block
// on the way loses the same height. A table or a replaced box ends the
// search.
func lastLineCut(box *Box) float64 {
	child := findLastInFlowChild(box.Children)
	switch {
	case child == nil:
		return 0
	case child.Kind == KindLine:
		cut := lastLineSpace(box, child)
		if cut > 0 {
			child.Height -= cut
			child.ownExtra().lastCut = true
		}
		return cut
	case child.Kind != KindBlock || child.IsTableWrapper || !isStr(child.Style.Get("height"), "auto"):
		return 0
	}
	cut := lastLineCut(child)
	child.Height -= cut
	return cut
}

// pageTopSpaceOf returns whether -ivy-page-top-space of a style is
// 'discard', and the length that a block keeps at the top of the part of
// a split table cell.
func pageTopSpaceOf(style *Style) (discard bool, keep float64) {
	switch v := style.Get("page_top_space").(type) {
	case String:
		return v == "discard", 0
	case Tuple:
		if len(v) == 2 {
			keep, _ := v[1].(Float)
			return isStr(v[0], "discard"), float64(keep)
		}
	}
	return false, 0
}

// strutTextHeight returns the height of the text of the strut of style,
// without the leading, for leadingShift.
func strutTextHeight(style *Style) float64 {
	normal := style.Copy()
	normal.Set("line_height", String("normal"))
	height, _ := strut(normal)
	return height
}

// textExtent returns the top and the bottom of the text of the strut of a
// box with -ivy-line-multiple, from the ascent to the descent of its font,
// when its baseline is at baselineY. ok is false for another box. The
// content area of a box of upstream is as high as its font size, so its
// bottom is above the descent of the text. A word processor ends a line of
// single spacing that holds only pictures at the bottom of the pictures,
// and they align at the bottom of the text with text-bottom, as on page 5
// of poi/doc/floating-pictures.doc.
func textExtent(box *Box, baselineY float64) (top, bottom float64, ok bool) {
	if _, _, ok := lineMultipleOf(box.Style); !ok {
		return 0, 0, false
	}
	normal := box.Style.Copy()
	normal.Set("line_height", String("normal"))
	height, baseline := strut(normal)
	if height <= 0 {
		return 0, 0, false
	}
	return baselineY - baseline, baselineY - baseline + height, true
}

// discardsSpaceBefore reports whether box, a block of the flow of the root
// box, loses the space before it by -ivy-page-top-space: the property is
// 'discard', the box starts a page other than the first, and the page
// starts after a break that is not forced. The space is the top margin,
// and the top padding when the box has no top border and no background,
// which docx gives as the space before. Word leaves out the space before
// a paragraph at the top of such a page, and keeps it after a page break
// or a section break. The margin-break of CSS does the same only for the
// children of the root box and of the boxes whose margins collapse with
// it, and upstream counts each break of a named page as forced.
//
// In columns, the box loses the space before it when it starts a column at
// its top, see layoutContext.columnTopBox and startColumn. Word leaves out
// the space before a paragraph at the top of each column too.
func (lc *layoutContext) discardsSpaceBefore(box *Box, pageIsEmpty, inFlow bool) bool {
	discard, _ := pageTopSpaceOf(box.Style)
	return (lc.columnTopBox == box || inFlow && pageIsEmpty && lc.currentPage > 1 && lc.unforcedBreak) && discard
}

// spaceBeforeKept returns the top margin of box, whose space before
// discardsSpaceBefore leaves out: 0, or at the top of the part of a split
// table cell, the length of -ivy-page-top-space. LibreOffice keeps the
// space above the first paragraph of such a part, without the space below
// the paragraph before it, which the margin also holds. It keeps that
// space too when the paragraph goes on from the page before, see
// blockContainerLayoutShared, and when contextual spacing leaves the space
// out between two paragraphs of the same style, so the length can be
// larger than the margin.
func (lc *layoutContext) spaceBeforeKept(box *Box) float64 {
	if !lc.cellTop || lc.columnTopBox != box {
		return 0
	}
	_, keep := pageTopSpaceOf(box.Style)
	return max(keep, 0)
}

// splitCellTop reports whether the first in-flow child of the table cell
// cell, which resumes at skip, loses the space before it by
// -ivy-page-top-space: the property is 'discard', and the cell goes on
// from the page before at the top of a page that starts after a break that
// is not forced. Word leaves out the space before the first paragraph of
// such a part of a cell, as at the top of the page. A row that starts on
// the page keeps the space before the paragraphs of its cells.
func (lc *layoutContext) splitCellTop(cell *Box, skip skipStack, pageIsEmpty bool) bool {
	discard, _ := pageTopSpaceOf(cell.Style)
	return len(skip) > 0 && pageIsEmpty && lc.currentPage > 1 && lc.unforcedBreak && discard
}

// startColumn sets the column box column as the box whose first in-flow
// child loses the space before it, when -ivy-page-top-space is 'discard'
// and the column starts at the top of a column as Word sees it: the column
// i of a row of columns after a column that a forced break did not end, or
// the first column at the top of a page whose space before is discarded,
// pageTop. Word keeps the space after a column break, as after a page
// break. It also sets i as the index of the column for the floats of the
// columns before it, see layout_column_float.go.
func (lc *layoutContext) startColumn(column *Box, i int, pageTop, forced bool) {
	lc.columnTopBox, lc.cellTop = nil, false
	lc.columnFloats.index = i
	if discard, _ := pageTopSpaceOf(column.Style); (i > 0 && !forced || i == 0 && pageTop) && discard {
		lc.columnTopBox = column
	}
}

// hasBackground reports whether a style gives a box a background: a color
// that is not transparent, or an image.
func hasBackground(style *Style) bool {
	if c := getColor(style, "background_color"); c != nil && c.Alpha > 0 {
		return true
	}
	images, _ := style.Get("background_image").(Tuple)
	for _, image := range images {
		if t, ok := image.(Tuple); ok && len(t) > 0 && !isStr(t[0], "none") {
			return true
		}
	}
	return false
}
