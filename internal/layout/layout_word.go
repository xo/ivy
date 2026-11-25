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

import "strconv"

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
}

// lineNumbersOf returns the value of -ivy-line-numbers of a style, and false
// for 'none'.
func lineNumbersOf(style *Style) (lineNumbers, bool) {
	t, ok := style.Get("line_numbers").(Tuple)
	if !ok || len(t) != 4 {
		return lineNumbers{}, false
	}
	distance, ok1 := t[0].(Float)
	step, ok2 := t[1].(Int)
	first, ok3 := t[2].(Int)
	restart, ok4 := t[3].(String)
	if !ok1 || !ok2 || !ok3 || !ok4 || step < 1 {
		return lineNumbers{}, false
	}
	return lineNumbers{float64(distance), int(step), int(first), string(restart)}, true
}

// lineNumberState is the state of the numbers of the lines at the start of
// a page: the number of the last line, and the element of the box that
// declares the numbers of that line, the section. started is false before
// the first numbered line of the document.
type lineNumberState struct {
	last    int
	section *Element
	started bool
}

// numberLines numbers the lines of the page, whose root box is root, from
// the state at the start of the page, and returns the state at its end. A
// line is numbered when its block has a value of -ivy-line-numbers other
// than 'none'. Only the line boxes of the in-flow blocks of the root box
// count, as in Word: the lines of a table, a float, a flex or grid item,
// an absolutely positioned box, a margin box or a footnote do not. The
// number of a line whose number is a multiple of the step is drawn, with
// its end at the distance before the start of the content of the box that
// declares the numbers, and its baseline at the baseline of the line. The
// numbers are text boxes in a block that is the last child of root.
func numberLines(lc *layoutContext, root *Box, state lineNumberState) lineNumberState {
	var lines []*Box
	pageStart := true
	var walk func(box, section *Box, parent lineNumbers, parentOn bool)
	walk = func(box, section *Box, parent lineNumbers, parentOn bool) {
		value, on := lineNumbersOf(box.Style)
		if on && (!parentOn || value != parent) {
			// the box declares the numbers: it starts a section
			section = box
		}
		for _, child := range box.Children {
			switch {
			case child.Kind == KindLine:
				// a row next to a float of -ivy-wrap-flow has one number,
				// on its first part (D63)
				if !on || child.Height <= 0 || child.extra().rowPart {
					continue
				}
				lc.spend(1)
				switch {
				case !state.started,
					value.restart == "page" && pageStart,
					value.restart == "section" && section.Element != state.section:
					state.last = value.first
				default:
					state.last++
				}
				state.started, state.section, pageStart = true, section.Element, false
				if state.last%value.step == 0 {
					if line := numberLine(child, section, value, state.last); line != nil {
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

// numberLine returns a line box with the number n of the line box line,
// in the style of the box section that declares the numbers, or nil when
// the number has no text.
func numberLine(line, section *Box, value lineNumbers, n int) *Box {
	style := section.Style.AnonymousStyle()
	text := strconv.Itoa(n)
	m := SplitFirstLine(text, style, nil, 0, true, false)
	if m.Layout == nil {
		return nil
	}
	textBox := newTextBox(section.ElementTag, style, section.Element, m.Layout.Text)
	textBox.TextLayout = m.Layout
	textBox.Width, textBox.Height, textBox.Baseline = m.Width, m.Height, m.Baseline
	textBox.PositionX = section.ContentBoxX() - value.distance - m.Width
	textBox.PositionY = line.PositionY + line.Baseline - m.Baseline
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
	m, ok := style.Get("line_multiple").(Float)
	if !ok || m < 1 || lineHeight <= textHeight {
		return 0
	}
	leading := lineHeight - textHeight
	return leading/2 - min(max(lineHeight/float64(m)-textHeight, 0), leading)
}

// strutTextHeight returns the height of the text of the strut of style,
// without the leading, for leadingShift.
func strutTextHeight(style *Style) float64 {
	normal := style.Copy()
	normal.Set("line_height", String("normal"))
	height, _ := strut(normal)
	return height
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
	return (lc.columnTopBox == box || inFlow && pageIsEmpty && lc.currentPage > 1 && lc.unforcedBreak) &&
		isStr(box.Style.Get("page_top_space"), "discard")
}

// splitCellTop reports whether the first in-flow child of the table cell
// cell, which resumes at skip, loses the space before it by
// -ivy-page-top-space: the property is 'discard', and the cell goes on
// from the page before at the top of a page that starts after a break that
// is not forced. Word leaves out the space before the first paragraph of
// such a part of a cell, as at the top of the page. A row that starts on
// the page keeps the space before the paragraphs of its cells.
func (lc *layoutContext) splitCellTop(cell *Box, skip skipStack, pageIsEmpty bool) bool {
	return len(skip) > 0 && pageIsEmpty && lc.currentPage > 1 && lc.unforcedBreak &&
		isStr(cell.Style.Get("page_top_space"), "discard")
}

// startColumn sets the column box column as the box whose first in-flow
// child loses the space before it, when -ivy-page-top-space is 'discard'
// and the column starts at the top of a column as Word sees it: the column
// i of a row of columns after a column that a forced break did not end, or
// the first column at the top of a page whose space before is discarded,
// pageTop. Word keeps the space after a column break, as after a page
// break.
func (lc *layoutContext) startColumn(column *Box, i int, pageTop, forced bool) {
	lc.columnTopBox = nil
	if (i > 0 && !forced || i == 0 && pageTop) && isStr(column.Style.Get("page_top_space"), "discard") {
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
