// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/preferred.py: the preferred and the
// minimum preferred widths.
//
// Also known as max-content and min-content width, also known as the
// shrink-to-fit algorithm.
//
// Terms used (max-content width, min-content width) are defined in David
// Baron's unofficial draft (https://dbaron.org/css/intrinsic/).

package layout

import (
	"fmt"
	"iter"
	"math"
	"strings"

	"github.com/xo/okage/tinycss2"

	"github.com/xo/ivy"
)

// pyMaxSize is sys.maxsize of Python, "an infinitely large number" that is
// not infinite.
const pyMaxSize = float64(math.MaxInt64)

// tableWidths is the tuple that table_and_columns_preferred_widths
// returns: the content widths of the table and of its columns.
type tableWidths struct {
	minContentWidth              float64
	maxContentWidth              float64
	columnMinContentWidths       []float64
	columnMaxContentWidths       []float64
	columnIntrinsicPercentages   []float64
	constrainedness              []bool
	totalHorizontalBorderSpacing float64
	// grid is the zipped grid of upstream: for each column, the cell of
	// each row that starts in the column, or nil.
	grid [][]*Box
}

// tablePreferredWidths is an item of the cache context.tables: the widths
// of a table for outer false and for outer true.
type tablePreferredWidths struct {
	inner, outer tableWidths
}

// nonPercentLength returns the value of a computed length that is a Dimension
// and not a percentage, and false for auto, a percentage, a math function
// or any other value. It is the test `value == 'auto' or check_math(value)
// or value.unit == '%'` of upstream, negated.
func nonPercentLength(v Value) (float64, bool) {
	d, ok := v.(Dimension)
	if !ok || d.Unit == "%" {
		return 0, false
	}
	return d.Value, true
}

// percentValue returns the value of a computed length that is a
// percentage, and false for any other value.
func percentValue(v Value) (float64, bool) {
	d, ok := v.(Dimension)
	if !ok || d.Unit != "%" {
		return 0, false
	}
	return d.Value, true
}

// shrinkToFit is shrink_to_fit in weasyprint/layout/preferred.py: the
// shrink-to-fit width of box.
//
// Warning: both availableContentWidth and the return value are for width
// of the content area, not margin area.
//
// https://www.w3.org/TR/CSS21/visudet.html#float-width
func shrinkToFit(lc *layoutContext, box *Box, availableContentWidth float64) float64 {
	return min(
		max(minContentWidth(lc, box, false), availableContentWidth),
		maxContentWidth(lc, box, false))
}

// minContentWidth is min_content_width in weasyprint/layout/preferred.py:
// the min-content width of box. This is the width by breaking at every
// line-break opportunity.
func minContentWidth(lc *layoutContext, box *Box, outer bool) float64 {
	lc.enter()
	defer lc.leave()
	switch {
	case box.IsTableWrapper:
		return tableAndColumnsPreferredWidths(lc, box, outer).minContentWidth
	case box.Kind == KindTableCell:
		return tableCellMinContentWidth(lc, box, outer)
	case box.IsBlockContainer() || box.Kind == KindTableColumn:
		return blockMinContentWidth(lc, box, outer)
	case box.Kind == KindTableColumnGroup:
		return columnGroupContentWidth(box)
	case box.Kind == KindInline || box.Kind == KindLine:
		return inlineMinContentWidth(lc, box, outer, nil, false, true)
	case box.IsReplaced():
		return replacedMinContentWidth(box, outer)
	case box.IsFlexContainer():
		return flexMinContentWidth(lc, box, outer)
	case box.IsGridContainer():
		// TODO: Get real grid size.
		return blockMinContentWidth(lc, box, outer)
	}
	// Port: upstream raises TypeError, "min-content width for this box
	// not handled yet". The width of such a box is 0.
	return 0
}

// maxContentWidth is max_content_width in weasyprint/layout/preferred.py:
// the max-content width of box. This is the width by only breaking at
// forced line breaks.
func maxContentWidth(lc *layoutContext, box *Box, outer bool) float64 {
	lc.enter()
	defer lc.leave()
	switch {
	case box.IsTableWrapper:
		return tableAndColumnsPreferredWidths(lc, box, outer).maxContentWidth
	case box.Kind == KindTableCell:
		_, maxWidth := tableCellMinMaxContentWidth(lc, box, outer)
		return maxWidth
	case box.IsBlockContainer() || box.Kind == KindTableColumn:
		return blockMaxContentWidth(lc, box, outer)
	case box.Kind == KindTableColumnGroup:
		return columnGroupContentWidth(box)
	case box.Kind == KindInline || box.Kind == KindLine:
		return inlineMaxContentWidth(lc, box, outer, true)
	case box.IsReplaced():
		return replacedMaxContentWidth(box, outer)
	case box.IsFlexContainer():
		return flexMaxContentWidth(lc, box, outer)
	case box.IsGridContainer():
		// TODO: Get real grid size.
		return blockMaxContentWidth(lc, box, outer)
	}
	// Port: upstream raises TypeError, "max-content width for this box
	// not handled yet". The width of such a box is 0.
	return 0
}

// blockContentWidth is _block_content_width in
// weasyprint/layout/preferred.py: the helper of blockMinContentWidth and
// blockMaxContentWidth. function is minContentWidth or maxContentWidth.
func blockContentWidth(lc *layoutContext, box *Box, function func(*layoutContext, *Box, bool) float64, outer bool) float64 {
	style := box.Style
	width, fixed := nonPercentLength(style.Get("width"))
	switch {
	case !fixed:
		// "percentages on the following properties are treated instead as
		// though they were the following: width: auto"
		// https://dbaron.org/css/intrinsic/#outer-intrinsic
		width = 0
		first := true
		for _, child := range box.Children {
			if !child.IsAbsolutelyPositioned() && !child.IsNote() {
				if w := function(lc, child, true); first || w > width {
					width, first = w, false
				}
			}
		}
	case isStr(style.Get("box_sizing"), "content-box"):
		// The width is the value.
	default:
		percentages := 0.0

		for _, value := range [...]string{"padding_left", "padding_right"} {
			if d, ok := style.Get(value).(Dimension); ok {
				if strings.ToLower(d.Unit) == "px" {
					width -= d.Value
				} else if d.Unit == "%" {
					percentages += d.Value
				}
				// Port: upstream asserts that the unit is %.
			}
		}

		// Same as margin_width().
		collapse := isStr(style.Get("border_collapse"), "collapse")
		if collapse && box.bordersSet {
			width -= box.BorderLeftWidth
		} else {
			width -= num(style.Get("border_left_width"))
		}
		if collapse && box.bordersSet {
			width -= box.BorderRightWidth
		} else {
			width -= num(style.Get("border_right_width"))
		}
		width = (100 - min(100, percentages)) * max(0, width) / 100
	}

	return adjust(box, outer, width, true, true)
}

// minMax is min_max in weasyprint/layout/preferred.py: the width of box
// from width and the min and the max widths of the box.
func minMax(box *Box, width float64) float64 {
	minWidth, ok := nonPercentLength(box.Style.Get("min_width"))
	if !ok {
		minWidth = 0
	}
	maxWidth, ok := nonPercentLength(box.Style.Get("max_width"))
	if !ok {
		maxWidth = math.Inf(1)
	}

	if box.IsReplaced() && box.extra().Replacement != nil {
		size := box.extra().Replacement.GetIntrinsicSize(1, num(box.Style.Get("font_size")))
		if size.HasRatio {
			ratio := size.Ratio
			minHeightValue := box.Style.Get("min_height")
			maxHeightValue := box.Style.Get("max_height")
			minPending := checkMath(minHeightValue)
			if minHeight, ok := nonPercentLength(minHeightValue); ok {
				minWidth = max(minWidth, minHeight*ratio)
			}
			// Port: upstream tests the math function of min-height, not of
			// max-height, and reads the unit of a math function of
			// max-height, which it does not have. The port keeps the test
			// of min-height, and skips the math function.
			if maxHeight, ok := nonPercentLength(maxHeightValue); ok && !minPending {
				maxWidth = min(maxWidth, maxHeight*ratio)
			}
		}
	}

	return max(minWidth, min(width, maxWidth))
}

// marginWidth is margin_width in weasyprint/layout/preferred.py: width
// with the paddings, the borders and the margins of the sides of box.
func marginWidth(box *Box, width float64, left, right bool) float64 {
	percentages := 0.0

	// See https://drafts.csswg.org/css-tables-3/#cell-intrinsic-offsets
	// It is a set of computed values for border-left-width, padding-left,
	// padding-right, and border-right-width (along with zero values for
	// margin-left and margin-right)
	// Port: the list of upstream is an array on the stack.
	var buf [4]string
	values := buf[:0]
	if left {
		values = append(values, "margin_left", "padding_left")
	}
	if right {
		values = append(values, "margin_right", "padding_right")
	}
	for _, value := range values {
		if d, ok := box.Style.Get(value).(Dimension); ok {
			if strings.ToLower(d.Unit) == "px" {
				width += d.Value
			} else if d.Unit == "%" {
				percentages += d.Value
			}
			// Port: upstream asserts that the unit is %.
		}
	}

	collapse := isStr(box.Style.Get("border_collapse"), "collapse")
	if left {
		if collapse && box.bordersSet {
			// In collapsed-borders mode: the computed horizontal padding of
			// the cell and, for border values, the used border-width values
			// of the cell (half the winning border-width)
			width += box.BorderLeftWidth
		} else {
			// In separated-borders mode: the computed horizontal padding and
			// border of the table-cell
			width += num(box.Style.Get("border_left_width"))
		}
	}
	if right {
		if collapse && box.bordersSet {
			// [...] the used border-width values of the cell
			width += box.BorderRightWidth
		} else {
			// [...] the computed border of the table-cell
			width += num(box.Style.Get("border_right_width"))
		}
	}

	if percentages < 100 {
		return width / (1 - percentages/100)
	}
	// Pathological case, ignore
	return 0
}

// adjust is adjust in weasyprint/layout/preferred.py: it respects the min
// and the max widths, and adjusts width depending on outer.
//
// If outer is set to true, it returns the margin width, else the content
// width.
func adjust(box *Box, outer bool, width float64, left, right bool) float64 {
	fixed := minMax(box, width)

	if outer {
		return marginWidth(box, fixed, left, right)
	}
	return fixed
}

// blockMinContentWidth is block_min_content_width in
// weasyprint/layout/preferred.py: the min-content width of a BlockBox.
func blockMinContentWidth(lc *layoutContext, box *Box, outer bool) float64 {
	return blockContentWidth(lc, box, minContentWidth, outer)
}

// blockMaxContentWidth is block_max_content_width in
// weasyprint/layout/preferred.py: the max-content width of a BlockBox.
func blockMaxContentWidth(lc *layoutContext, box *Box, outer bool) float64 {
	return blockContentWidth(lc, box, maxContentWidth, outer)
}

// inlineMinContentWidth is inline_min_content_width in
// weasyprint/layout/preferred.py: the min-content width of an InlineBox.
//
// The width is calculated from the lines from skip. If firstLine is true,
// only the first line minimum width is calculated.
func inlineMinContentWidth(lc *layoutContext, box *Box, outer bool, skip skipStack, firstLine, isLineStart bool) float64 {
	widths := inlineLineWidths(lc, box, outer, isLineStart, true, skip, firstLine)
	width := 0.0
	first := true
	for w := range widths {
		if firstLine {
			width = w
			break
		}
		if first || w > width {
			width = w
		}
		first = false
	}
	return adjust(box, outer, width, true, true)
}

// inlineMaxContentWidth is inline_max_content_width in
// weasyprint/layout/preferred.py: the max-content width of an InlineBox.
func inlineMaxContentWidth(lc *layoutContext, box *Box, outer, isLineStart bool) float64 {
	var widths []float64
	for w := range inlineLineWidths(lc, box, outer, isLineStart, false, nil, false) {
		widths = append(widths, w)
	}
	// Remove trailing space, as split_first_line keeps trailing spaces when
	// max_width is not set.
	widths[len(widths)-1] -= trailingWhitespaceSize(lc, box)
	width := widths[0]
	for _, w := range widths[1:] {
		width = max(width, w)
	}
	return adjust(box, outer, width, true, true)
}

// columnGroupContentWidth is column_group_content_width in
// weasyprint/layout/preferred.py: the min- and max-content width of a
// TableColumnGroupBox.
func columnGroupContentWidth(box *Box) float64 {
	width, ok := nonPercentLength(box.Style.Get("width"))
	if !ok {
		width = 0
	}
	// Port: upstream asserts that the unit is px.
	return adjust(box, false, width, true, true)
}

// tableCellMinContentWidth is table_cell_min_content_width in
// weasyprint/layout/preferred.py: the min-content width of a TableCellBox.
func tableCellMinContentWidth(lc *layoutContext, box *Box, outer bool) float64 {
	// See https://www.w3.org/TR/css-tables-3/#outer-min-content
	// The outer min-content width of a table-cell is
	// max(min-width, min-content width) adjusted by
	// the cell intrinsic offsets.
	childrenWidth := 0.0
	first := true
	for _, child := range box.Children {
		if !child.IsAbsolutelyPositioned() && !child.IsNote() {
			if w := minContentWidth(lc, child, true); first || w > childrenWidth {
				childrenWidth, first = w, false
			}
		}
	}
	return adjust(box, outer, childrenWidth, true, true)
}

// tableCellMinMaxContentWidth is table_cell_min_max_content_width in
// weasyprint/layout/preferred.py: the min- and max-content width of a
// TableCellBox.
func tableCellMinMaxContentWidth(lc *layoutContext, box *Box, outer bool) (minWidth, maxWidth float64) {
	// This is much faster than calling min and max separately.
	minWidth = tableCellMinContentWidth(lc, box, outer)
	maxWidth = max(minWidth, blockMaxContentWidth(lc, box, outer))
	return minWidth, maxWidth
}

// noLine is the None that inline_line_widths keeps in its lines to track
// the line breaks.
var noLine = math.NaN()

// orZero is `line or 0` of Python for a line of inline_line_widths.
func orZero(line float64) float64 {
	if math.IsNaN(line) {
		return 0
	}
	return line
}

// orNoLine is `line or None` of Python for a line of inline_line_widths.
func orNoLine(line float64) float64 {
	if line == 0 {
		return noLine
	}
	return line
}

// inlineLineWidths is inline_line_widths in
// weasyprint/layout/preferred.py: it yields the width of each line.
func inlineLineWidths(lc *layoutContext, box *Box, outer, isLineStart, minimum bool, skip skipStack, firstLine bool) iter.Seq[float64] {
	return func(yield func(float64) bool) {
		lc.enter()
		defer lc.leave()

		// Set text indent.
		textIndent := 0.0
		if box.Kind == KindLine {
			indentToken := box.Style.Get("text_indent")
			if checkMath(indentToken) {
				// Ignore percentages by setting refer_to to 0.
				textIndent = mathTextIndent(indentToken, box.Style)
			} else if d, ok := indentToken.(Dimension); ok && d.Unit != "%" {
				textIndent = d.Value
			}
		}

		// Yield widths for each line.
		currentLine := 0.0
		start := 0
		if skip != nil {
			start, skip = skip.item()
		}
		// Port: upstream reads the children from a skip past the end as
		// no children.
		start = min(max(start, 0), len(box.Children))
		for _, child := range box.Children[start:] {
			lc.spend(1)
			// Skip absolutely positioned elements.
			if child.IsAbsolutelyPositioned() || child.IsNote() {
				continue
			}

			// None is used in "lines" to track line breaks, transformed to
			// 0 when yielded.
			var lines []float64
			switch child.Kind {
			case KindInline:
				// Inline box, call function recursively.
				childLines := inlineLineWidths(lc, child, outer, isLineStart, minimum, skip, firstLine)
				for line := range childLines {
					lines = append(lines, orNoLine(line))
					if firstLine {
						break
					}
				}
				if len(lines) == 0 {
					// Port: inline_line_widths always yields a line.
					lines = []float64{noLine}
				}
				if len(lines) == 1 {
					lines[0] = adjust(child, outer, orZero(lines[0]), true, true)
				} else {
					lines[0] = orNoLine(adjust(child, outer, orZero(lines[0]), true, false))
					lines[len(lines)-1] = orNoLine(adjust(child, outer, orZero(lines[len(lines)-1]), false, true))
				}
			case KindText:
				// Text box, split into lines.
				whiteSpace := child.Style.Get("white_space")
				spaceCollapse := isStrIn(whiteSpace, "normal", "nowrap", "pre-line")
				textWrap := isStrIn(whiteSpace, "normal", "pre-wrap", "pre-line")
				textSkip := 0
				if skip != nil {
					textSkip, _ = skip.item()
					// Port: upstream asserts that the skip stack of a text
					// box holds nothing more.
				}
				// Port: upstream fails on a skip past the end of the text.
				textSkip = min(max(textSkip, 0), len(child.Text))
				childText := child.Text[textSkip:]
				if isLineStart && spaceCollapse {
					childText = strings.TrimLeft(childText, " ")
				}
				var maxWidth *float64
				if minimum {
					zero := 0.0
					maxWidth = &zero
				}
				resumeIndex, newResumeIndex, hasNewResume := 0, 0, true
				for hasNewResume {
					lc.spend(1)
					resumeIndex += newResumeIndex
					// Port: upstream fails to decode a text cut in the middle
					// of a character. The port cuts at the byte.
					resumeIndex = min(resumeIndex, len(childText))
					metrics := SplitFirstLine(childText[resumeIndex:], child.Style, maxWidth,
						child.JustificationSpacing, isLineStart, true)
					newResumeIndex, hasNewResume = metrics.ResumeIndex, metrics.HasResume
					lines = append(lines, orNoLine(metrics.Width))
					if firstLine {
						break
					}
					if hasNewResume && newResumeIndex <= 0 {
						// Port: upstream asserts that each line takes some
						// text. The port stops, so that it never loops.
						break
					}
				}
				if firstLine && hasNewResume && newResumeIndex != 0 {
					// We only need the first line, break early.
					currentLine += orZero(lines[0])
					if !yield(currentLine + textIndent) {
						return
					}
					return
				}
				// TODO: use the real next character instead of 'a' to
				// detect line breaks.
				lastLetter := ""
				if childText != "" {
					lastLetter = lastCharacter(childText)
				}
				canBreak := CanBreakText(lastLetter+"a", str(child.Style.Get("lang")))
				if minimum && textWrap && canBreak {
					// Add all possible line breaks for minimal width.
					lines = append(lines, noLine)
				}
			default:
				// Replaced elements, inline blocks…
				// https://www.w3.org/TR/css-text-3/#overflow-wrap
				// "The line breaking behavior of a replaced element
				//  or other atomic inline is equivalent to that
				//  of the Object Replacement Character (U+FFFC)."
				// https://www.unicode.org/reports/tr14/#DescriptionOfProperties
				// "By default, there is a break opportunity
				//  both before and after any inline object."
				if minimum {
					// "For soft wrap opportunities defined by the boundary
					// between two characters or atomic inlines, the
					// white-space property on the nearest common ancestor of
					// the two characters controls breaking; which elements’
					// line-break, word-break, and overflow-wrap properties
					// control the determination of soft wrap opportunities
					// at such boundaries is undefined in this level." We
					// choose to always follow the parent’s value here, other
					// parts of the line-breaking algorithm do the same.
					if isStrIn(box.Style.Get("white_space"), "normal", "pre-wrap", "pre-line") {
						lines = []float64{noLine, minContentWidth(lc, child, true), noLine}
					} else {
						lines = []float64{minContentWidth(lc, child, true)}
					}
				} else {
					lines = []float64{maxContentWidth(lc, child, true)}
				}
			}
			// The first text line goes on the current line.
			currentLine += orZero(lines[0])
			if len(lines) > 1 {
				// Forced line break(s).
				if !yield(currentLine + textIndent) {
					return
				}
				textIndent = 0
				if len(lines) > 2 {
					for _, line := range lines[1 : len(lines)-1] {
						if !yield(orZero(line)) {
							return
						}
					}
				}
				currentLine = orZero(lines[len(lines)-1])
			}
			isLineStart = math.IsNaN(lines[len(lines)-1])
			skip = nil
		}
		yield(currentLine + textIndent)
	}
}

// lastCharacter returns the last character of s, s[-1:] of Python.
func lastCharacter(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] < 0x80 || s[i] >= 0xC0 {
			return s[i:]
		}
	}
	return s
}

// mathTextIndent returns the indent of a math function of text-indent,
// with the percentages as 0, as inline_line_widths computes it.
func mathTextIndent(indentToken Value, style *Style) float64 {
	token, ok := indentToken.(Token)
	if !ok {
		return 0
	}
	zero := 0.0
	result, err := ResolveMath(token.Node, style, "text_indent", &zero)
	if err != nil || result == nil {
		// Port: upstream raises the errors of resolve_math.
		return 0
	}
	pair, err := validateNonShorthand([]tinycss2.Node{result}, "text-indent", "", false)
	if err != nil {
		// Port: upstream raises InvalidValues.
		return 0
	}
	if d, ok := pair.value.(Dimension); ok && d.Unit != "%" {
		return d.Value
	}
	return 0
}

// percentageContribution is _percentage_contribution in
// weasyprint/layout/preferred.py: the percentage contribution of a cell, a
// column or a column group.
//
// https://dbaron.org/css/intrinsic/#pct-contrib
func percentageContribution(box *Box) float64 {
	minWidth, ok := percentValue(box.Style.Get("min_width"))
	if !ok {
		minWidth = 0
	}
	maxWidth, ok := percentValue(box.Style.Get("max_width"))
	if !ok {
		maxWidth = math.Inf(1)
	}
	width, ok := percentValue(box.Style.Get("width"))
	if !ok {
		width = 0
	}
	return max(minWidth, min(width, maxWidth))
}

// sumFloats returns the sum of a list of numbers.
func sumFloats(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
}

// percentageKey is the key of the cache of get_percentage_contribution.
type percentageKey struct {
	cell            *Box
	origin          int
	maxContentWidth float64
}

// tableAndColumnsPreferredWidths is table_and_columns_preferred_widths in
// weasyprint/layout/preferred.py: the content widths of the auto layout
// table and of its columns.
//
// https://dbaron.org/css/intrinsic/
func tableAndColumnsPreferredWidths(lc *layoutContext, box *Box, outer bool) tableWidths {
	table := box.GetWrappedTable()
	if table == nil {
		// Port: a table wrapper always holds a table upstream.
		return tableWidths{}
	}
	if result, ok := lc.tables[table]; ok {
		if outer {
			return result.outer
		}
		return result.inner
	}

	// Create the grid
	gridWidth, gridHeight := 0, 0
	rowNumber := 0
	for _, rowGroup := range table.Children {
		for _, row := range rowGroup.Children {
			for _, cell := range row.Children {
				gridWidth = max(cell.table().GridX+cell.table().Colspan, gridWidth)
				gridHeight = max(rowNumber+cell.table().Rowspan, gridHeight)
			}
			rowNumber++
		}
	}
	// Port: upstream has no bound. The grid holds at most as many cells as
	// the grid of the collapsed borders.
	if gridWidth*gridHeight > maxCollapsedBorders {
		lc.abort(fmt.Errorf("%w: a table has a grid of %d by %d cells", ivy.ErrLimit, gridWidth, gridHeight))
	}
	grid := make([][]*Box, gridHeight)
	for i := range grid {
		grid[i] = make([]*Box, gridWidth)
	}
	rowNumber = 0
	for _, rowGroup := range table.Children {
		for _, row := range rowGroup.Children {
			for _, cell := range row.Children {
				if rowNumber < gridHeight && cell.table().GridX >= 0 && cell.table().GridX < gridWidth {
					grid[rowNumber][cell.table().GridX] = cell
				}
			}
			rowNumber++
		}
	}

	zippedGrid := make([][]*Box, gridWidth)
	for x := range zippedGrid {
		zippedGrid[x] = make([]*Box, gridHeight)
		for y := range gridHeight {
			zippedGrid[x][y] = grid[y][x]
		}
	}

	// Define the total horizontal border spacing
	totalHorizontalBorderSpacing := 0.0
	if isStr(table.Style.Get("border_collapse"), "separate") && gridWidth > 0 {
		columns := 0
		for _, column := range zippedGrid {
			for _, cell := range column {
				if cell != nil {
					columns++
					break
				}
			}
		}
		spacingX, _ := tableBorderSpacing(table.Style)
		totalHorizontalBorderSpacing = spacingX * float64(1+columns)
	}

	if gridWidth == 0 || gridHeight == 0 {
		table.Children = nil
		minWidth := blockMinContentWidth(lc, table, false)
		maxWidth := blockMaxContentWidth(lc, table, false)
		outerMinWidth := adjust(box, true, blockMinContentWidth(lc, table, true), true, true)
		outerMaxWidth := adjust(box, true, blockMaxContentWidth(lc, table, true), true, true)
		result := &tablePreferredWidths{
			inner: tableWidths{
				minContentWidth: minWidth, maxContentWidth: maxWidth,
				totalHorizontalBorderSpacing: totalHorizontalBorderSpacing,
			},
			outer: tableWidths{
				minContentWidth: outerMinWidth, maxContentWidth: outerMaxWidth,
				totalHorizontalBorderSpacing: totalHorizontalBorderSpacing,
			},
		}
		lc.tables[table] = result
		if outer {
			return result.outer
		}
		return result.inner
	}

	columnGroups := make([]*Box, gridWidth)
	columns := make([]*Box, gridWidth)
	columnNumber := 0
groups:
	for _, columnGroup := range table.table().ColumnGroups {
		for _, column := range columnGroup.Children {
			columnGroups[columnNumber] = columnGroup
			columns[columnNumber] = column
			columnNumber++
			if columnNumber == gridWidth {
				break groups
			}
		}
	}

	var colspanCells []*Box
	colspans := map[int]bool{}

	// Define the intermediate content widths
	minContentWidths := make([]float64, gridWidth)
	maxContentWidths := make([]float64, gridWidth)
	intrinsicPercentages := make([]float64, gridWidth)

	// Intermediate content widths for span 1
	for i := range gridWidth {
		for _, groups := range [...][]*Box{columnGroups, columns} {
			if group := groups[i]; group != nil {
				minContentWidths[i] = max(minContentWidths[i], minContentWidth(lc, group, true))
				maxContentWidths[i] = max(maxContentWidths[i], maxContentWidth(lc, group, true))
				intrinsicPercentages[i] = max(intrinsicPercentages[i], percentageContribution(group))
			}
		}
		for _, cell := range zippedGrid[i] {
			if cell == nil {
				continue
			}
			lc.spend(1)
			if cell.table().Colspan == 1 {
				minWidth, maxWidth := tableCellMinMaxContentWidth(lc, cell, true)
				minContentWidths[i] = max(minContentWidths[i], minWidth)
				maxContentWidths[i] = max(maxContentWidths[i], maxWidth)
				intrinsicPercentages[i] = max(intrinsicPercentages[i], percentageContribution(cell))
			} else {
				colspanCells = append(colspanCells, cell)
				colspans[cell.table().Colspan-1] = true
			}
		}
	}

	// Intermediate content widths for span > 1 is wrong in the 4.1 section,
	// as explained in its third issue. Min- and max-content widths are
	// handled by the excess width distribution method, and percentages do
	// not distribute widths to columns that have originating cells.

	// Intermediate intrinsic percentage widths for span > 1
	rowsOrigins := make([][]int, len(grid))
	for y, row := range grid {
		origin := -1
		rowOrigins := make([]int, len(row))
		for x, cell := range row {
			if cell != nil {
				origin = x
			}
			rowOrigins[x] = origin
		}
		rowsOrigins[y] = rowOrigins
	}

	// Cached for big colspan values, see #1155.
	percentageCache := map[percentageKey]float64{}
	getPercentageContribution := func(originCell *Box, origin int, maxContentWidth float64) float64 {
		key := percentageKey{originCell, origin, maxContentWidth}
		if result, ok := percentageCache[key]; ok {
			return result
		}
		end := min(origin+originCell.table().Colspan, gridWidth)
		baselinePercentage := sumFloats(intrinsicPercentages[origin:end])
		cellPercentageContribution := percentageContribution(originCell)
		diff := max(0, cellPercentageContribution-baselinePercentage)
		otherColumnsContributionsSum := 0.0
		otherColumnsContributions := 0
		for j := origin; j < end; j++ {
			if intrinsicPercentages[j] == 0 {
				otherColumnsContributionsSum += maxContentWidths[j]
				otherColumnsContributions++
			}
		}
		var ratio float64
		if otherColumnsContributionsSum == 0 {
			ratio = 1 / float64(max(otherColumnsContributions, 1))
		} else {
			ratio = maxContentWidth / otherColumnsContributionsSum
		}
		percentageCache[key] = diff * ratio
		return diff * ratio
	}

	for span := range gridWidth {
		if !colspans[span] {
			continue
		}
		percentageContributions := make([]float64, 0, gridWidth)
		for i := range gridWidth {
			if contribution := intrinsicPercentages[i]; contribution != 0 {
				percentageContributions = append(percentageContributions, contribution)
				continue
			}
			lc.spend(len(grid))
			contribution := 0.0
			for y, row := range grid {
				origin := rowsOrigins[y][i]
				if origin < 0 {
					continue
				}
				originCell := row[origin]
				if originCell.table().Colspan-1 != span {
					continue
				}
				cellPercentageContribution := getPercentageContribution(originCell, origin, maxContentWidths[i])
				contribution = max(contribution, cellPercentageContribution)
			}

			percentageContributions = append(percentageContributions, contribution)
		}

		intrinsicPercentages = percentageContributions
	}

	// Define constrainedness
	constrainedness := make([]bool, gridWidth)
	for i := range gridWidth {
		if columnGroups[i] != nil {
			if _, ok := nonPercentLength(columnGroups[i].Style.Get("width")); ok {
				constrainedness[i] = true
				continue
			}
		}
		if columns[i] != nil {
			if _, ok := nonPercentLength(columns[i].Style.Get("width")); ok {
				constrainedness[i] = true
				continue
			}
		}
		for _, cell := range zippedGrid[i] {
			if cell != nil && cell.table().Colspan == 1 {
				if _, ok := nonPercentLength(cell.Style.Get("width")); ok {
					constrainedness[i] = true
					break
				}
			}
		}
	}

	clamped := make([]float64, gridWidth)
	prefix := 0.0
	for i, percentage := range intrinsicPercentages {
		clamped[i] = min(percentage, 100-prefix)
		prefix += percentage
	}
	intrinsicPercentages = clamped

	// Max- and min-content widths for span > 1
	for _, cell := range colspanCells {
		lc.spend(1)
		minContent := minContentWidth(lc, cell, true)
		maxContent := maxContentWidth(lc, cell, true)
		start, end := pySlice(cell.table().GridX, cell.table().GridX+cell.table().Colspan, true, gridWidth)
		columnsMinContent := sumFloats(minContentWidths[start:end])
		columnsMaxContent := sumFloats(maxContentWidths[start:end])
		spacing := 0.0
		if isStr(table.Style.Get("border_collapse"), "separate") {
			spacingX, _ := tableBorderSpacing(table.Style)
			spacing = float64(cell.table().Colspan-1) * spacingX
		}

		if minContent > columnsMinContent+spacing {
			excessWidth := minContent - (columnsMinContent + spacing)
			distributeExcessWidth(lc, zippedGrid, excessWidth, minContentWidths,
				constrainedness, intrinsicPercentages, maxContentWidths, start, end)
		}

		if maxContent > columnsMaxContent+spacing {
			excessWidth := maxContent - (columnsMaxContent + spacing)
			distributeExcessWidth(lc, zippedGrid, excessWidth, maxContentWidths,
				constrainedness, intrinsicPercentages, maxContentWidths, start, end)
		}
	}

	// Calculate the max- and min-content widths of table and columns
	var smallPercentageContributions []float64
	largePercentageContributionNumerator := 0.0
	for i := range gridWidth {
		if intrinsicPercentages[i] != 0 {
			smallPercentageContributions = append(smallPercentageContributions,
				maxContentWidths[i]/(intrinsicPercentages[i]/100))
		} else {
			largePercentageContributionNumerator += maxContentWidths[i]
		}
	}
	largePercentageContributionDenominator := (100 - sumFloats(intrinsicPercentages)) / 100
	var largePercentageContribution float64
	if largePercentageContributionDenominator == 0 {
		if largePercentageContributionNumerator == 0 {
			largePercentageContribution = 0
		} else {
			// "the large percentage contribution of the table [is] an
			// infinitely large number if the numerator is nonzero [and] the
			// denominator of that ratio is 0."
			//
			// https://dbaron.org/css/intrinsic/#autotableintrinsic
			//
			// Please note that "an infinitely large number" is not
			// "infinite", and that's probably not a coincindence: putting
			// 'inf' here breaks some cases (see #305).
			largePercentageContribution = pyMaxSize
		}
	} else {
		largePercentageContribution = largePercentageContributionNumerator / largePercentageContributionDenominator
	}

	tableMinContentWidth := totalHorizontalBorderSpacing + sumFloats(minContentWidths)
	tableMaxContentWidth := max(sumFloats(maxContentWidths), largePercentageContribution)
	for _, contribution := range smallPercentageContributions {
		tableMaxContentWidth = max(tableMaxContentWidth, contribution)
	}
	tableMaxContentWidth += totalHorizontalBorderSpacing

	var tableMinWidth, tableMaxWidth float64
	if d, ok := table.Style.Get("width").(Dimension); ok && strings.ToLower(d.Unit) == "px" {
		// "percentages on the following properties are treated instead as
		// though they were the following: width: auto"
		// https://dbaron.org/css/intrinsic/#outer-intrinsic
		tableMinWidth, tableMaxWidth = d.Value, d.Value
	} else {
		tableMinWidth = tableMinContentWidth
		tableMaxWidth = tableMaxContentWidth
	}

	tableMinContentWidth = max(tableMinContentWidth, adjust(table, false, tableMinWidth, true, true))
	tableMaxContentWidth = max(tableMaxContentWidth, adjust(table, false, tableMaxWidth, true, true))
	tableOuterMinContentWidth := marginWidth(table, marginWidth(box, tableMinContentWidth, true, true), true, true)
	tableOuterMaxContentWidth := marginWidth(table, marginWidth(box, tableMaxContentWidth, true, true), true, true)

	inner := tableWidths{
		minContentWidth:              tableMinContentWidth,
		maxContentWidth:              tableMaxContentWidth,
		columnMinContentWidths:       minContentWidths,
		columnMaxContentWidths:       maxContentWidths,
		columnIntrinsicPercentages:   intrinsicPercentages,
		constrainedness:              constrainedness,
		totalHorizontalBorderSpacing: totalHorizontalBorderSpacing,
		grid:                         zippedGrid,
	}
	outerWidths := inner
	outerWidths.minContentWidth = tableOuterMinContentWidth
	outerWidths.maxContentWidth = tableOuterMaxContentWidth
	result := &tablePreferredWidths{inner: inner, outer: outerWidths}
	lc.tables[table] = result
	if outer {
		return result.outer
	}
	return result.inner
}

// tableBorderSpacing returns the horizontal and the vertical border spacings of
// the style of a table, style['border_spacing'].
func tableBorderSpacing(style *Style) (x, y float64) {
	spacing, _ := style.Get("border_spacing").(Tuple)
	if len(spacing) == 2 {
		return num(spacing[0]), num(spacing[1])
	}
	// Port: upstream fails to unpack a value that is not a pair.
	return 0, 0
}

// intrinsicSizePointers returns the intrinsic size of an image as the pointers
// of defaultImageSizing, with nil for None.
func intrinsicSizePointers(size IntrinsicSize) (width, height, ratio *float64) {
	if size.HasWidth {
		width = &size.Width
	}
	if size.HasHeight {
		height = &size.Height
	}
	if size.HasRatio {
		ratio = &size.Ratio
	}
	return width, height, ratio
}

// replacedSpecifiedHeight returns the height of the style of a replaced box for
// default_image_sizing, with nil for auto.
func replacedSpecifiedHeight(box *Box) *float64 {
	if height, ok := nonPercentLength(box.Style.Get("height")); ok {
		// Port: upstream asserts that the unit is px.
		return &height
	}
	return nil
}

// replacedMinContentWidth is replaced_min_content_width in
// weasyprint/layout/preferred.py: the min-content width of an
// InlineReplacedBox.
func replacedMinContentWidth(box *Box, outer bool) float64 {
	widthValue := box.Style.Get("width")
	var width float64
	if isStr(widthValue, "auto") {
		height := replacedSpecifiedHeight(box)
		_, unknownMaxWidth := percentValue(box.Style.Get("max_width"))
		if unknownMaxWidth || box.extra().Replacement == nil {
			// See https://drafts.csswg.org/css-sizing/#intrinsic-contribution
			//
			// Port: a replaced box without an image has a width of 0.
			width = 0
		} else {
			image := box.extra().Replacement
			intrinsicWidth, intrinsicHeight, intrinsicRatio := intrinsicSizePointers(image.GetIntrinsicSize(
				num(box.Style.Get("image_resolution")), num(box.Style.Get("font_size"))))
			width, _ = defaultImageSizing(intrinsicWidth, intrinsicHeight, intrinsicRatio, nil, height, 0, 0)
		}
	} else if w, ok := nonPercentLength(widthValue); ok {
		// Port: upstream asserts that the unit is px.
		width = w
		// Port: a replaced box with a percentage max-width can shrink to
		// nothing, as css-sizing-3 says for a compressible replaced
		// element and as browsers do. Upstream does it only when the width
		// is auto. An image of a README with the attribute width and a
		// max-width of 100% then fits in the cell of a table.
		if _, ok := percentValue(box.Style.Get("max_width")); ok {
			width = 0
		}
	} else {
		// See https://drafts.csswg.org/css-sizing/#intrinsic-contribution
		width = 0
	}
	return adjust(box, outer, width, true, true)
}

// replacedMaxContentWidth is replaced_max_content_width in
// weasyprint/layout/preferred.py: the max-content width of an
// InlineReplacedBox.
func replacedMaxContentWidth(box *Box, outer bool) float64 {
	widthValue := box.Style.Get("width")
	var width float64
	if isStr(widthValue, "auto") {
		height := replacedSpecifiedHeight(box)
		var intrinsicWidth, intrinsicHeight, intrinsicRatio *float64
		if box.extra().Replacement != nil {
			// Port: a replaced box without an image has no intrinsic size.
			intrinsicWidth, intrinsicHeight, intrinsicRatio = intrinsicSizePointers(box.extra().Replacement.GetIntrinsicSize(
				num(box.Style.Get("image_resolution")), num(box.Style.Get("font_size"))))
		}
		width, _ = defaultImageSizing(intrinsicWidth, intrinsicHeight, intrinsicRatio, nil, height, 300, 150)
	} else if w, ok := nonPercentLength(widthValue); ok {
		// Port: upstream asserts that the unit is px.
		width = w
	} else {
		// See https://drafts.csswg.org/css-sizing/#intrinsic-contribution
		width = 0
	}
	return adjust(box, outer, width, true, true)
}

// preferredColumnGap returns the used column-gap of a flex container for its
// preferred widths: 0 for normal, a percentage or a math function.
func preferredColumnGap(box *Box) float64 {
	gap, ok := nonPercentLength(box.Style.Get("column_gap"))
	if !ok {
		return 0
	}
	return gap
}

// flexMinContentWidth is flex_min_content_width in
// weasyprint/layout/preferred.py: the min-content width of a
// FlexContainerBox.
func flexMinContentWidth(lc *layoutContext, box *Box, outer bool) float64 {
	// TODO: use real values, see
	// https://www.w3.org/TR/css-flexbox-1/#intrinsic-sizes
	var minContents []float64
	for _, child := range box.Children {
		if child.IsFlexItem {
			minContents = append(minContents, minContentWidth(lc, child, true))
		}
	}
	if len(minContents) == 0 {
		return adjust(box, outer, 0, true, true)
	}
	singleRow := strings.HasPrefix(str(box.Style.Get("flex_direction")), "row") &&
		isStr(box.Style.Get("flex_wrap"), "nowrap")
	if singleRow {
		columnGaps := float64(len(minContents)-1) * preferredColumnGap(box)
		return adjust(box, outer, sumFloats(minContents), true, true) + columnGaps
	}
	width := minContents[0]
	for _, w := range minContents[1:] {
		width = max(width, w)
	}
	return adjust(box, outer, width, true, true)
}

// flexMaxContentWidth is flex_max_content_width in
// weasyprint/layout/preferred.py: the max-content width of a
// FlexContainerBox.
func flexMaxContentWidth(lc *layoutContext, box *Box, outer bool) float64 {
	// TODO: use real values, see
	// https://www.w3.org/TR/css-flexbox-1/#intrinsic-sizes
	var maxContents []float64
	for _, child := range box.Children {
		if child.IsFlexItem {
			maxContents = append(maxContents, maxContentWidth(lc, child, true))
		}
	}
	if len(maxContents) == 0 {
		return adjust(box, outer, 0, true, true)
	}
	if strings.HasPrefix(str(box.Style.Get("flex_direction")), "row") {
		columnGaps := float64(len(maxContents)-1) * preferredColumnGap(box)
		return adjust(box, outer, sumFloats(maxContents), true, true) + columnGaps
	}
	width := maxContents[0]
	for _, w := range maxContents[1:] {
		width = max(width, w)
	}
	return adjust(box, outer, width, true, true)
}

// trailingWhitespaceSize is trailing_whitespace_size in
// weasyprint/layout/preferred.py: the size of the trailing white space of
// box.
//
// Port: upstream gives split_text_box an available width of None. The
// port gives an infinite width, which split_first_line treats as None.
func trailingWhitespaceSize(lc *layoutContext, box *Box) float64 {
	// Find last box child, keep last parent to remove nested trailing
	// spaces.
	var lastParent *Box
	for box.Kind == KindInline || box.Kind == KindLine {
		if len(box.Children) == 0 {
			return 0
		}
		lastParent, box = box, box.Children[len(box.Children)-1]
	}

	// Return early if possible.
	switch {
	case box.Kind != KindText || box.Text == "":
		// There’s no text in last child.
		return 0
	case !isStrIn(box.Style.Get("white_space"), "normal", "nowrap", "pre-line"):
		// Spaces don’t collapse.
		return 0
	case num(box.Style.Get("font_size")) == 0:
		// Trailing spaces take no space.
		return 0
	case !strings.HasSuffix(box.Text, " "):
		// No trailing space.
		return 0
	}

	// Strip text.
	inf := math.Inf(1)
	if strippedText := strings.TrimRight(box.Text, " "); strippedText != "" {
		// Stripped text is not empty, calculate width difference.
		resume, hasResume := 0, true
		oldResume := 0
		var oldBox *Box
		for hasResume {
			lc.spend(1)
			oldResume = resume
			oldBox, resume, hasResume, _ = splitTextBox(lc, box, inf, resume, true)
			if hasResume && resume <= oldResume {
				// Port: the text must go on, so that the loop ends.
				break
			}
		}
		if oldBox == nil {
			// Port: upstream asserts that the box is not None.
			return 0
		}
		strippedBox := box.CopyWithText(strippedText)
		strippedBox, _, _, _ = splitTextBox(lc, strippedBox, inf, oldResume, true)
		if strippedBox == nil {
			// Old box is split just before the trailing spaces.
			return oldBox.Width
		}
		// Return difference between old width and stripped width.
		return oldBox.Width - strippedBox.Width
	}
	// Stripped text is empty, render spaces to get width.
	width := SplitFirstLine(box.Text, box.Style, nil, box.JustificationSpacing, true, false).Width
	// Remove possible trailing spaces from previous child.
	if lastParent != nil && len(lastParent.Children) >= 2 {
		width += trailingWhitespaceSize(lc, lastParent.Children[len(lastParent.Children)-2])
	}
	return width
}
