// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/table.py: the layout of the tables and
// of the internal table boxes. collapse_table_borders is also called by the
// building of the boxes, for a table with collapsed borders.

package layout

import (
	"math"
	"slices"

	"github.com/xo/okage/tinycss2"
)

// transparent is TRANSPARENT in weasyprint/layout/table.py.
var transparent = &tinycss2.Color{Space: "srgb", Coordinates: []float64{0, 0, 0}, Alpha: 0}

// CollapsedBorder is a border of the grid of the collapsed borders of a
// table: its style, its width and its color.
type CollapsedBorder struct {
	Style string
	Width float64
	Color *tinycss2.Color

	// score is the score of the resolution of the conflicts: hidden, the
	// width and the rank of the style.
	score [3]float64
}

// CollapsedBorderGrid is the result of collapse_table_borders in
// weasyprint/layout/table.py: the borders of the grid of a table, always
// from left to right and from top to bottom.
type CollapsedBorderGrid struct {
	// Vertical holds grid_height rows of grid_width + 1 borders.
	Vertical [][]CollapsedBorder
	// Horizontal holds grid_height + 1 rows of grid_width borders.
	Horizontal [][]CollapsedBorder
}

// maxCollapsedBorders bounds the borders of the grid of a table.
//
// Port: upstream has no bound. A table of a hostile document with a wide
// and high grid needs more memory than it has.
const maxCollapsedBorders = 1 << 22

// borderStyleScores are the scores of the styles of the borders, the
// reversed list of upstream.
var borderStyleScores = map[string]float64{
	"none": 0, "inset": 1, "groove": 2, "outset": 3, "ridge": 4,
	"dotted": 5, "dashed": 6, "solid": 7, "double": 8, "hidden": 9,
}

// lessScore reports whether the score a is less than the score b, as the
// comparison of the tuples of Python does.
func lessScore(a, b [3]float64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// pyIndex returns the index of Python i in a list of length n, and false
// when it is out of the list.
//
// Port: upstream raises IndexError out of the list.
func pyIndex(i, n int) (int, bool) {
	if i < 0 {
		i += n
	}
	return i, i >= 0 && i < n
}

// collapseTableBorders is collapse_table_borders in
// weasyprint/layout/table.py. It resolves the conflicts of the borders of a
// table in the collapsing border model: it sets the widths of the borders
// of the table, the column groups, the columns, the row groups, the rows
// and the cells, and returns the grid of the borders.
func collapseTableBorders(table *Box, gridWidth, gridHeight int) *CollapsedBorderGrid {
	if gridWidth == 0 || gridHeight == 0 {
		// Don’t bother with empty tables.
		return &CollapsedBorderGrid{}
	}
	if (gridWidth+1)*(gridHeight+1) > maxCollapsedBorders {
		return &CollapsedBorderGrid{}
	}

	styleMap := map[string]string{"inset": "ridge", "outset": "groove"}
	weakNullBorder := CollapsedBorder{score: [3]float64{0, 0, borderStyleScores["none"]}, Style: "none", Width: 0, Color: transparent}

	// Borders are always stored left to right, top to bottom.
	verticalBorders := make([][]CollapsedBorder, gridHeight)
	for i := range verticalBorders {
		verticalBorders[i] = make([]CollapsedBorder, gridWidth+1)
		for j := range verticalBorders[i] {
			verticalBorders[i][j] = weakNullBorder
		}
	}
	horizontalBorders := make([][]CollapsedBorder, gridHeight+1)
	for i := range horizontalBorders {
		horizontalBorders[i] = make([]CollapsedBorder, gridWidth)
		for j := range horizontalBorders[i] {
			horizontalBorders[i][j] = weakNullBorder
		}
	}

	setOneBorder := func(borderGrid [][]CollapsedBorder, boxStyle *Style, side string, gridX, gridY int) {
		style := str(boxStyle.Get("border_" + side + "_style"))
		width := num(boxStyle.Get("border_" + side + "_width"))
		color := getColor(boxStyle, "border_"+side+"_color")

		// See https://www.w3.org/TR/CSS21/tables.html#border-conflict-resolution.
		hidden := 0.0
		if style == "hidden" {
			hidden = 1
		}
		score := [3]float64{hidden, width, borderStyleScores[style]}

		if mapped, ok := styleMap[style]; ok {
			style = mapped
		}
		y, ok := pyIndex(gridY, len(borderGrid))
		if !ok {
			return
		}
		x, ok := pyIndex(gridX, len(borderGrid[y]))
		if !ok {
			return
		}
		// Strict < so that the earlier call wins in case of a tie.
		if lessScore(borderGrid[y][x].score, score) {
			borderGrid[y][x] = CollapsedBorder{score: score, Style: style, Width: width, Color: color}
		}
	}

	ltr := isStr(table.Style.Get("direction"), "ltr")
	setBorders := func(box *Box, x, y, w, h int) {
		style := box.Style

		// x and y are logical (possibly rtl), but borders are graphical
		// (always ltr).
		if ltr {
			for yy := y; yy < y+h; yy++ {
				setOneBorder(verticalBorders, style, "left", x, yy)
				setOneBorder(verticalBorders, style, "right", x+w, yy)
			}
			for xx := x; xx < x+w; xx++ {
				setOneBorder(horizontalBorders, style, "top", xx, y)
				setOneBorder(horizontalBorders, style, "bottom", xx, y+h)
			}
		} else {
			for yy := y; yy < y+h; yy++ {
				setOneBorder(verticalBorders, style, "left", -1-w-x, yy)
				setOneBorder(verticalBorders, style, "right", -1-x, yy)
			}
			for xx := -1 - x; xx > -1-x-w; xx-- {
				setOneBorder(horizontalBorders, style, "top", xx, y)
				setOneBorder(horizontalBorders, style, "bottom", xx, y+h)
			}
		}
	}

	// Set cell borders. The order is important here:
	// "A style set on a cell wins over one on a row, which wins over a
	//  row group, column, column group and, lastly, table"
	// See https://www.w3.org/TR/CSS21/tables.html#border-conflict-resolution.
	strongNullBorder := CollapsedBorder{score: [3]float64{1, 0, borderStyleScores["hidden"]}, Style: "hidden", Width: 0, Color: transparent}
	setStrong := func(grid [][]CollapsedBorder, xx, yy int) {
		y, ok := pyIndex(yy, len(grid))
		if !ok {
			return
		}
		if x, ok := pyIndex(xx, len(grid[y])); ok {
			grid[y][x] = strongNullBorder
		}
	}
	gridY := 0
	for _, rowGroup := range table.Children {
		for _, row := range rowGroup.Children {
			for _, cell := range row.Children {
				// Force null border inside of a cell with rowspan or
				// colspan.
				gridX, colspan, rowspan := cell.table().GridX, cell.table().Colspan, cell.table().Rowspan
				var verticalX, horizontalX []int
				if ltr {
					for xx := gridX + 1; xx < gridX+colspan; xx++ {
						verticalX = append(verticalX, xx)
					}
					for xx := gridX; xx < gridX+colspan; xx++ {
						horizontalX = append(horizontalX, xx)
					}
				} else {
					for xx := -2 - gridX; xx > -1-gridX-colspan; xx-- {
						verticalX = append(verticalX, xx)
					}
					for xx := -1 - gridX; xx > -1-gridX-colspan; xx-- {
						horizontalX = append(horizontalX, xx)
					}
				}
				for _, xx := range verticalX {
					for yy := gridY; yy < gridY+rowspan; yy++ {
						setStrong(verticalBorders, xx, yy)
					}
				}
				for _, xx := range horizontalX {
					for yy := gridY + 1; yy < gridY+rowspan; yy++ {
						setStrong(horizontalBorders, xx, yy)
					}
				}
				// Set cell border.
				setBorders(cell, gridX, gridY, colspan, rowspan)
			}
			gridY++
		}
	}

	// Set row borders.
	gridY = 0
	for _, rowGroup := range table.Children {
		for _, row := range rowGroup.Children {
			setBorders(row, 0, gridY, gridWidth, 1)
			gridY++
		}
	}

	// Set row group borders.
	gridY = 0
	for _, rowGroup := range table.Children {
		rowspan := len(rowGroup.Children)
		setBorders(rowGroup, 0, gridY, gridWidth, rowspan)
		gridY += rowspan
	}

	// Set column borders.
	for _, columnGroup := range table.table().ColumnGroups {
		for _, column := range columnGroup.Children {
			setBorders(column, column.table().GridX, 0, 1, gridHeight)
		}
	}

	// Set column group group borders.
	for _, columnGroup := range table.table().ColumnGroups {
		setBorders(columnGroup, columnGroup.table().GridX, 0, columnGroup.Span(), gridHeight)
	}

	// Set table borders.
	setBorders(table, 0, 0, gridWidth, gridHeight)

	// Now that all conflicts are resolved, set transparent borders of the
	// correct widths on each box. The actual border grid will be painted
	// separately.
	setBorderUsedWidth := func(box *Box, side string, twiceWidth float64) {
		switch side {
		case "top":
			box.BorderTopWidth = twiceWidth / 2
		case "right":
			box.BorderRightWidth = twiceWidth / 2
		case "bottom":
			box.BorderBottomWidth = twiceWidth / 2
		case "left":
			box.BorderLeftWidth = twiceWidth / 2
		}
		box.bordersSet = true
	}

	removeBorders := func(box *Box) {
		setBorderUsedWidth(box, "top", 0)
		setBorderUsedWidth(box, "right", 0)
		setBorderUsedWidth(box, "bottom", 0)
		setBorderUsedWidth(box, "left", 0)
	}

	maxVerticalWidth := func(x, y1, y2 int) float64 {
		width := 0.0
		start, end := pySlice(y1, y2, true, len(verticalBorders))
		for _, gridRow := range verticalBorders[start:end] {
			if i, ok := pyIndex(x, len(gridRow)); ok {
				width = max(width, gridRow[i].Width)
			}
		}
		return width
	}

	maxHorizontalWidth := func(x1, y, x2 int, hasX2 bool) float64 {
		width := 0.0
		yy, ok := pyIndex(y, len(horizontalBorders))
		if !ok {
			return 0
		}
		row := horizontalBorders[yy]
		start, end := pySlice(x1, x2, hasX2, len(row))
		for _, border := range row[start:end] {
			width = max(width, border.Width)
		}
		return width
	}

	gridY = 0
	for _, rowGroup := range table.Children {
		removeBorders(rowGroup)
		for _, row := range rowGroup.Children {
			removeBorders(row)
			for _, cell := range row.Children {
				x, y := cell.table().GridX, gridY
				colspan, rowspan := cell.table().Colspan, cell.table().Rowspan
				var top, bottom, left, right float64
				if ltr {
					top = maxHorizontalWidth(x, y, x+colspan, true)
					bottom = maxHorizontalWidth(x, y+rowspan, x+colspan, true)
					left = maxVerticalWidth(x, y, y+rowspan)
					right = maxVerticalWidth(x+colspan, y, y+rowspan)
				} else {
					// -x or None of upstream: no end for x == 0.
					top = maxHorizontalWidth(-colspan-x, y, -x, x != 0)
					bottom = maxHorizontalWidth(-colspan-x, y+rowspan, -x, x != 0)
					left = maxVerticalWidth(-1-colspan-x, y, y+rowspan)
					right = maxVerticalWidth(-1-x, y, y+rowspan)
				}
				setBorderUsedWidth(cell, "top", top)
				setBorderUsedWidth(cell, "bottom", bottom)
				setBorderUsedWidth(cell, "left", left)
				setBorderUsedWidth(cell, "right", right)
			}
			gridY++
		}
	}

	for _, columnGroup := range table.table().ColumnGroups {
		removeBorders(columnGroup)
		for _, column := range columnGroup.Children {
			removeBorders(column)
		}
	}

	setBorderUsedWidth(table, "top", maxHorizontalWidth(0, 0, gridWidth, true))
	setBorderUsedWidth(table, "bottom", maxHorizontalWidth(0, gridHeight, gridWidth, true))
	// "UAs must compute an initial left and right border width for the table
	//  by examining the first and last cells in the first row of the table."
	// https://www.w3.org/TR/CSS21/tables.html#collapsing-borders
	// ... so h=1, not grid_height:
	setBorderUsedWidth(table, "left", maxVerticalWidth(0, 0, 1))
	setBorderUsedWidth(table, "right", maxVerticalWidth(gridWidth, 0, 1))

	return &CollapsedBorderGrid{Vertical: verticalBorders, Horizontal: horizontalBorders}
}

// pySlice returns the bounds of the slice [start:end] of Python in a list of
// length n. hasEnd is false for no end.
func pySlice(start, end int, hasEnd bool, n int) (int, int) {
	norm := func(i int) int {
		if i < 0 {
			i += n
		}
		return min(max(i, 0), n)
	}
	start = norm(start)
	if !hasEnd {
		end = n
	} else {
		end = norm(end)
	}
	return start, max(start, end)
}

// tableAttrs holds the attributes that table_layout sets on the boxes of a
// table and reads, besides the fields of Box that the drawing reads.
type tableAttrs struct {
	// computedHeight is the used height of a cell before the layout of
	// its content, which can be Auto. contentHeight is the height of its
	// content.
	computedHeight float64
	contentHeight  float64
	// verticalAlign is the vertical alignment of a cell: "top", "middle",
	// "bottom" or "baseline".
	verticalAlign string
	// hasBaseline is set when the baseline of a row is set, for the None of
	// upstream.
	hasBaseline bool
	// getCells returns the cells that start in a column, the method
	// get_cells that table_layout sets on the columns. It is nil before the
	// layout.
	getCells func() []*Box
}

// ColumnCells is get_cells of TableColumnBox in
// weasyprint/formatting_structure/boxes.py: the cells that start in the
// column. The layout of the table sets them. A column that is not laid out
// has no cells.
//
// Port: upstream raises NotImplementedError for a column that is not laid
// out.
func (b *Box) ColumnCells() []*Box {
	if b.table().getCells == nil {
		return nil
	}
	return b.table().getCells()
}

// tableGroupsResult is what the functions of table_layout return for the
// row groups: the laid out boxes, where the next page resumes, the next
// page, the position after them, and whether the layout fails, for the
// None of upstream.
type tableGroupsResult struct {
	header, footer *Box
	children       []*Box
	resumeAt       skipStack
	nextPage       nextPage
	positionY      float64
	failed         bool
}

// tableLayout is table_layout in weasyprint/layout/table.py: the layout of
// a table box. Upstream does not read the containing block.
func tableLayout(lc *layoutContext, table *Box, bottomSpace float64, skip skipStack, _ *Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box) blockLayoutResult {
	lc.enter()
	defer lc.leave()

	// Remove top and bottom decorations for split tables.
	hasHeader := len(table.Children) > 0 && table.Children[0].table().IsHeader
	hasFooter := len(table.Children) > 0 && table.Children[len(table.Children)-1].table().IsFooter
	collapse := isStr(table.Style.Get("border_collapse"), "collapse")
	// Port: upstream does not draw the collapsed borders at the edges of a
	// row that a page break splits, also with box-decoration-break: clone
	// on the table. With clone on the table, the port draws the border
	// above the split row at the top of the next part of the table, and
	// the border below it at the bottom of this part, as LibreOffice draws
	// the borders of a split cell on both pages. A cell with clone keeps
	// its half of each border and its padding on both pages, as upstream
	// keeps them.
	cloneCollapsed := collapse && isStr(table.Style.Get("box_decoration_break"), "clone")
	removeStartDecoration := skip != nil && !hasHeader
	table.RemoveDecoration(removeStartDecoration, false)

	// Set border spacings.
	var borderSpacingX, borderSpacingY float64
	if !collapse {
		borderSpacingX, borderSpacingY = tableBorderSpacing(table.Style)
	}

	// Define column positions.
	columnWidths := table.table().ColumnWidths
	columnPositions := make([]float64, 0, len(columnWidths))
	rowsLeftX := table.ContentBoxX() + borderSpacingX
	var rowsWidth float64
	if isStr(table.Style.Get("direction"), "ltr") {
		positionX := table.ContentBoxX()
		rowsX := positionX + borderSpacingX
		for _, width := range columnWidths {
			positionX += borderSpacingX
			columnPositions = append(columnPositions, positionX)
			positionX += width
		}
		rowsWidth = positionX - rowsX
	} else {
		positionX := table.ContentBoxX() + table.Width
		rowsX := positionX - borderSpacingX
		for _, width := range columnWidths {
			positionX -= borderSpacingX
			positionX -= width
			columnPositions = append(columnPositions, positionX)
		}
		rowsWidth = rowsX - positionX
	}
	table.ownTable().ColumnPositions = columnPositions

	// Set border top width on tables with collapsed borders and split
	// cells.
	skippedRows := 0
	if collapse {
		table.ownTable().SkipCellBorderTop = false
		table.ownTable().SkipCellBorderBottom = false
		splitCells := false
		if len(skip) > 0 {
			skippedGroups, groupSkip := skip.item()
			if len(groupSkip) > 0 {
				var cellsSkip skipStack
				skippedRows, cellsSkip = groupSkip.item()
				if len(cellsSkip) > 0 {
					splitCells = true
				}
			}
			for _, group := range table.Children[:min(max(skippedGroups, 0), len(table.Children))] {
				skippedRows += len(group.Children)
			}
		}
		if (!splitCells || cloneCollapsed) && !hasHeader {
			if grid := table.table().CollapsedBorderGrid; grid != nil && len(grid.Horizontal) > 0 {
				// Port: upstream fails on a row out of the grid.
				if row, ok := pyIndex(skippedRows, len(grid.Horizontal)); ok && len(grid.Horizontal[row]) > 0 {
					width := grid.Horizontal[row][0].Width
					for _, border := range grid.Horizontal[row][1:] {
						width = max(width, border.Width)
					}
					table.BorderTopWidth = width / 2
				}
			}
		}
	}

	// Make this a sub-function so that many local variables like rows_x
	// don't need to be passed as parameters.
	groupLayout := func(group *Box, positionY, bottomSpace float64, pageIsEmpty bool, skip skipStack) (*Box, skipStack, nextPage) {
		lc.spend(1)
		var resumeAt skipStack
		nextPage := anyNextPage()
		originalPageIsEmpty := pageIsEmpty
		resolvePercentages(group, table)
		group.PositionX = rowsLeftX
		group.PositionY = positionY
		group.Width = rowsWidth
		var newGroupChildren []*Box
		// For each row, cells for which this is the last row (with
		// rowspan).
		endingCellsByRow := make([][]*Box, len(group.Children))

		isGroupStart := skip == nil
		start := 0
		if !isGroupStart {
			start, skip = skip.item()
		}
		// Port: upstream skips the rows of a skip stack past the end.
		start = min(max(start, 0), len(group.Children))
		for indexRow := start; indexRow < len(group.Children); indexRow++ {
			row := group.Children[indexRow]
			lc.spend(1)
			row.index = indexRow

			if len(newGroupChildren) > 0 {
				pageBreak := blockLevelPageBreak(newGroupChildren[len(newGroupChildren)-1], row)
				if forcePageBreak(pageBreak, lc) {
					nextPage.brk = pageBreak
					resumeAt = skipStack{indexRow: nil}
					break
				}
			}

			resolvePercentages(row, table)
			row.PositionX = rowsLeftX
			row.PositionY = positionY
			row.Width = rowsWidth
			// Place cells at the top of the row and layout their content.
			var newRowChildren []*Box
			for indexCell, cell := range row.Children {
				lc.spend(1)
				first, last := pySlice(cell.table().GridX, 0, false, len(columnWidths))
				last = min(last, first+max(cell.table().Colspan, 0))
				spannedWidths := columnWidths[first:last]
				// In the fixed layout the grid width is set by cells in
				// the first row and column elements.
				// This may be less than the previous value of cell.colspan
				// if that would bring the cell beyond the grid width.
				cell.ownTable().Colspan = len(spannedWidths)
				if cell.table().Colspan == 0 {
					// The cell is entirely beyond the grid width, remove it
					// entirely. Subsequent cells in the same row have
					// greater grid_x, so they are beyond too.
					ignoredCells := row.Children[indexCell:]
					lc.log.warning("This row has more columns than the table, ignored %d cells", len(ignoredCells))
					break
				}
				resolvePercentages(cell, table)
				if isStr(table.Style.Get("direction"), "ltr") {
					cell.PositionX = columnPositions[cell.table().GridX]
				} else {
					cell.PositionX = columnPositions[cell.table().GridX+cell.table().Colspan-1]
				}
				cell.PositionY = row.PositionY
				cell.MarginTop = 0
				cell.MarginLeft = 0
				cell.Width = 0
				bordersPlusPadding := cell.BorderWidth() // with width==0
				// TODO: we should remove the number of columns with no
				// originating cells to cell.colspan, see
				// test_layout_table_auto_49.
				cell.Width = sumFloats(spannedWidths) +
					borderSpacingX*float64(cell.table().Colspan-1) -
					bordersPlusPadding
				var cellSkip skipStack
				if len(skip) > 0 {
					if s, ok := skip[indexCell]; ok {
						cellSkip = s
					} else {
						cellSkip = skipStack{len(cell.Children): nil}
					}
				}

				// Adapt cell and table collapsing borders when a row is
				// split.
				if len(cellSkip) > 0 && collapse {
					if hasHeader {
						// We have a header, we have to adapt the position
						// of the split cell to match the header’s bottom
						// border.
						headerRows := table.Children[0].Children
						if len(headerRows) > 0 && len(headerRows[len(headerRows)-1].Children) > 0 {
							headers := headerRows[len(headerRows)-1].Children
							width := headers[0].BorderBottomWidth
							for _, header := range headers[1:] {
								width = max(width, header.BorderBottomWidth)
							}
							cell.PositionY += width
						}
					} else if !cloneCollapsed {
						// We don’t have a header, we have to skip the
						// decoration at the top of the table when it’s
						// drawn.
						table.ownTable().SkipCellBorderTop = true
					}
				}

				// First try to render content as if there was already
				// something on the page to avoid hitting
				// block_level_layout’s TODO. Then force to render
				// something if the page is actually empty, or just draw an
				// empty cell otherwise. See
				// test_table_break_children_margin.
				// Pretend that height is not set, keeping computed height
				// as a minimum.
				cell.ownTable().computedHeight = cell.Height
				cell.Height = Auto
				originalStyle := cell.Style
				if !isStr(cell.Style.Get("height"), "auto") {
					styleCopy := cell.Style.Copy()
					styleCopy.Set("height", String("auto"))
					cell.Style = styleCopy
				}
				result := blockContainerLayout(lc, cell, bottomSpace, cellSkip, pageIsEmpty,
					absoluteBoxes, fixedBoxes, nil, nil, nil, false, noMaxLines)
				cell.Style = originalStyle
				newCell, cellResumeAt := result.box, result.resumeAt
				if newCell == nil {
					cell = cell.CopyWithChildren(nil)
					var cellAbsoluteBoxes, cellFixedBoxes []*Box
					cell = blockContainerLayout(lc, cell, bottomSpace, cellSkip, true,
						&cellAbsoluteBoxes, &cellFixedBoxes, nil, nil, nil, false, noMaxLines).box
					cellResumeAt = skipStack{0: nil}
					if cell == nil {
						// Port: the layout of an empty cell on an empty
						// page always gives a box upstream.
						continue
					}
				} else {
					cell = newCell
				}

				cell.RemoveDecoration(cellSkip != nil, false)
				if len(cellResumeAt) > 0 {
					if resumeAt == nil {
						resumeAt = skipStack{indexRow: skipStack{}}
					}
					if resumeAt[indexRow] == nil {
						resumeAt[indexRow] = skipStack{}
					}
					resumeAt[indexRow][indexCell] = cellResumeAt
				}
				cell.ownTable().Empty = len(cell.InFlowOrFloatedChildren()) == 0
				cell.ownTable().contentHeight = cell.Height
				if cell.ownTable().computedHeight != Auto {
					cell.Height = max(cell.Height, cell.ownTable().computedHeight)
				}
				newRowChildren = append(newRowChildren, cell)
			}

			if len(resumeAt) > 0 && !pageIsEmpty {
				// Avoid break when "break-inside: avoid" is set on row or
				// any on its cells.
				avoidBreak := avoidPageBreak(str(row.Style.Get("break_inside")), lc)
				for _, cell := range row.Children {
					if avoidBreak {
						break
					}
					avoidBreak = avoidPageBreak(str(cell.Style.Get("break_inside")), lc)
				}
				if avoidBreak {
					removePlaceholders(lc, newRowChildren, absoluteBoxes, fixedBoxes)
					newGroupChildren, resumeAt = tableFindEarlierPageBreak(lc, row, indexRow,
						newGroupChildren, originalPageIsEmpty, absoluteBoxes, fixedBoxes)
					if resumeAt == nil {
						return nil, nil, nextPage
					}
					break
				}
			}

			if len(resumeAt) > 0 {
				// Remove bottom decoration if row is split.
				for _, cell := range newRowChildren {
					cell.RemoveDecoration(false, true)
				}
			}

			row = row.CopyWithChildren(newRowChildren)

			// Table height algorithm
			// https://www.w3.org/TR/CSS21/tables.html#height-layout

			// Set row baseline with cells with vertical-align: baseline.
			var baselineCells []*Box
			for _, cell := range row.Children {
				verticalAlign := cell.Style.Get("vertical_align")
				if isStrIn(verticalAlign, "top", "middle", "bottom") {
					cell.ownTable().verticalAlign = str(verticalAlign)
				} else {
					// Assume 'baseline' for any other value
					cell.ownTable().verticalAlign = "baseline"
					cell.Baseline = cellBaseline(cell)
					baselineCells = append(baselineCells, cell)
				}
			}
			if len(baselineCells) > 0 {
				row.Baseline = baselineCells[0].Baseline
				for _, cell := range baselineCells[1:] {
					row.Baseline = max(row.Baseline, cell.Baseline)
				}
				row.ownTable().hasBaseline = true
				for _, cell := range baselineCells {
					extra := row.Baseline - cell.Baseline
					if cell.Baseline != row.Baseline && extra != 0 {
						addTopPadding(cell, extra)
					}
				}
			}

			// Set row height.
			for _, cell := range row.Children {
				// Port: upstream fails on a row span past the rows of the
				// group. The port ends the cell on the last row.
				i, ok := pyIndex(cell.table().Rowspan-1, len(endingCellsByRow))
				if !ok {
					i = len(endingCellsByRow) - 1
				}
				if i >= 0 {
					endingCellsByRow[i] = append(endingCellsByRow[i], cell)
				}
			}
			var endingCells []*Box
			if len(endingCellsByRow) > 0 {
				endingCells = endingCellsByRow[0]
				endingCellsByRow = endingCellsByRow[1:]
			}
			var rowBottomY float64
			switch {
			case len(endingCells) > 0: // in this row
				// Port: each part of a row that a page break splits takes
				// the height of its content. Upstream keeps the height of
				// the row, which then overflows the page, so that a row
				// with a height never splits.
				if row.Height == Auto || len(resumeAt) > 0 || len(skip) > 0 {
					rowBottomY = math.Inf(-1)
					for _, cell := range endingCells {
						rowBottomY = max(rowBottomY, cell.PositionY+cell.BorderHeight())
					}
					row.Height = max(rowBottomY-row.PositionY, 0)
				} else {
					// Port: a cell that spans rows ends in this row with
					// the part of its height that is past the rows before
					// it, as in the branch above. Upstream compares the
					// whole height of the cell with the height of the row.
					height := row.Height
					for _, rowCell := range endingCells {
						height = max(height, rowCell.PositionY+rowCell.BorderHeight()-row.PositionY)
					}
					row.Height = height
					rowBottomY = row.PositionY + row.Height
				}
			case row.Height != Auto && len(resumeAt) == 0 && len(skip) == 0:
				// Port: a row with a height in which no cell ends keeps its
				// height, as browsers and spreadsheets keep it. Upstream
				// gives the row the height 0. A cell that spans rows and
				// ends in a later row then starts at the same place as in
				// a browser.
				rowBottomY = row.PositionY + row.Height
			default:
				rowBottomY = row.PositionY
				row.Height = 0
			}

			if len(baselineCells) == 0 {
				row.Baseline = rowBottomY
				row.ownTable().hasBaseline = true
			}

			// Add extra padding to make the cells the same height as the
			// row and honor vertical-align.
			for _, cell := range endingCells {
				cellBottomY := cell.PositionY + cell.BorderHeight()
				extra := rowBottomY - cellBottomY
				if extra != 0 {
					switch cell.ownTable().verticalAlign {
					case "bottom":
						addTopPadding(cell, extra)
					case "middle":
						extra /= 2
						addTopPadding(cell, extra)
						cell.PaddingBottom += extra
					default:
						cell.PaddingBottom += extra
					}
				}
				if cell.ownTable().computedHeight != Auto {
					verticalAlignShift := 0.0
					switch cell.ownTable().verticalAlign {
					case "middle":
						verticalAlignShift = (cell.ownTable().computedHeight - cell.ownTable().contentHeight) / 2
					case "bottom":
						verticalAlignShift = cell.ownTable().computedHeight - cell.ownTable().contentHeight
					}
					if verticalAlignShift > 0 {
						for _, child := range cell.Children {
							child.Translate(0, verticalAlignShift, false)
						}
						keepAbsolute(cell, verticalAlignShift)
					}
				}
			}

			nextPositionY := row.PositionY + row.Height
			if resumeAt == nil {
				nextPositionY += borderSpacingY
			}

			// Break if one cell was broken.
			breakCell := false
			if len(resumeAt) > 0 {
				allEmpty := true
				for _, child := range row.Children {
					if !child.table().Empty {
						allEmpty = false
						break
					}
				}
				if allEmpty {
					// No cell was displayed, give up row.
					nextPositionY = math.Inf(1)
					pageIsEmpty = false
					resumeAt = nil
				} else {
					breakCell = true
				}
			}

			// Break if this row overflows the page, unless there is no
			// other content on the page.
			overflow := lc.overflowsPage(bottomSpace, nextPositionY)
			if !pageIsEmpty && overflow {
				removePlaceholders(lc, row.Children, absoluteBoxes, fixedBoxes)
				newGroupChildren, resumeAt = tableFindEarlierPageBreak(lc, row, indexRow,
					newGroupChildren, originalPageIsEmpty, absoluteBoxes, fixedBoxes)
				if resumeAt == nil {
					return nil, nil, nextPage
				}
				break
			}

			newGroupChildren = append(newGroupChildren, row)
			positionY = nextPositionY
			pageIsEmpty = false
			skip = nil

			if breakCell && collapse && !hasFooter && !cloneCollapsed {
				table.ownTable().SkipCellBorderBottom = true
			}

			if breakCell || len(resumeAt) > 0 {
				break
			}
		}

		// Do not keep the row group if we made a page break
		// before any of its rows or with 'avoid'.
		abort := len(resumeAt) > 0 && !originalPageIsEmpty &&
			(avoidPageBreak(str(group.Style.Get("break_inside")), lc) || len(newGroupChildren) == 0)
		if abort {
			removePlaceholders(lc, newGroupChildren, absoluteBoxes, fixedBoxes)
			return nil, nil, nextPage
		}

		group = group.CopyWithChildren(newGroupChildren)
		group.RemoveDecoration(!isGroupStart, resumeAt != nil)

		// Set missing baselines in a second loop because of rowspan.
		for _, row := range group.Children {
			if !row.ownTable().hasBaseline {
				if len(row.Children) > 0 {
					// Set baseline to lowest bottom content edge.
					bottom := math.Inf(-1)
					for _, cell := range row.Children {
						bottom = max(bottom, cell.ContentBoxY()+cell.Height)
					}
					row.Baseline = bottom - row.PositionY
				} else {
					row.Baseline = 0
				}
				row.ownTable().hasBaseline = true
			}
		}
		group.Height = positionY - group.PositionY
		if len(group.Children) > 0 {
			// The last border spacing is outside of the group.
			group.Height -= borderSpacingY
		}

		return group, resumeAt, nextPage
	}

	bodyGroupsLayout := func(skip skipStack, positionY, bottomSpace float64, pageIsEmpty bool) tableGroupsResult {
		start := 0
		if skip != nil {
			start, skip = skip.item()
		}
		start = min(max(start, 0), len(table.Children))
		newTableChildren := []*Box{}
		var resumeAt skipStack
		nextPage := anyNextPage()

		for i, group := range table.Children[start:] {
			if group.table().IsHeader || group.table().IsFooter {
				continue
			}

			// Index is useless for headers and footers, as we never want
			// to break pages after the header or before the footer.
			indexGroup := i + start
			group.index = indexGroup

			if len(newTableChildren) > 0 {
				pageBreak := blockLevelPageBreak(newTableChildren[len(newTableChildren)-1], group)
				if forcePageBreak(pageBreak, lc) {
					nextPage.brk = pageBreak
					resumeAt = skipStack{indexGroup: nil}
					break
				}
			}

			var newGroup *Box
			newGroup, resumeAt, nextPage = groupLayout(group, positionY, bottomSpace, pageIsEmpty, skip)
			skip = nil

			if newGroup == nil {
				newTableChildren, resumeAt = tableFindEarlierPageBreak(lc, group, indexGroup,
					newTableChildren, pageIsEmpty, absoluteBoxes, fixedBoxes)
				if resumeAt == nil {
					return tableGroupsResult{nextPage: nextPage, positionY: positionY, failed: true}
				}
				break
			}

			newTableChildren = append(newTableChildren, newGroup)
			positionY += newGroup.Height + borderSpacingY
			pageIsEmpty = false

			if len(resumeAt) > 0 {
				resumeAt = skipStack{indexGroup: resumeAt}
				break
			}
		}

		return tableGroupsResult{children: newTableChildren, resumeAt: resumeAt, nextPage: nextPage, positionY: positionY}
	}

	// Layout row groups, rows and cells.
	positionY := table.ContentBoxY()
	if skip == nil {
		positionY += borderSpacingY
	}
	initialPositionY := positionY
	var tableRows []*Box
	for _, child := range table.Children {
		if !child.table().IsHeader && !child.table().IsFooter {
			tableRows = append(tableRows, child)
		}
	}

	allGroupsLayout := func() tableGroupsResult {
		// If the page is not empty, we try to render the header and the
		// footer on it. If the table does not fit on the page, we try to
		// render it on the next page.

		// If the page is empty and the header and footer are too big, there
		// are not rendered. If no row can be rendered because of the header
		// and the footer, the header and/or the footer are not rendered.

		headerFooterBottomSpace := math.Inf(-1)
		if pageIsEmpty {
			headerFooterBottomSpace = bottomSpace
		}

		var header, footer *Box
		var headerHeight, footerHeight float64
		if hasHeader {
			var resumeAt skipStack
			header, resumeAt, _ = groupLayout(table.Children[0], positionY, headerFooterBottomSpace, false, nil)
			if header != nil && len(resumeAt) == 0 {
				headerHeight = header.Height + borderSpacingY
			} else {
				// Header too big for the page.
				header = nil
			}
		}

		if hasFooter {
			var resumeAt skipStack
			footer, resumeAt, _ = groupLayout(table.Children[len(table.Children)-1], positionY, headerFooterBottomSpace, false, nil)
			if footer != nil && len(resumeAt) == 0 {
				footerHeight = footer.Height + borderSpacingY
			} else {
				// Footer too big for the page.
				footer = nil
			}
		}

		// Don't remove headers and footers if breaks are avoided in line
		// groups
		start := 0
		if len(skip) > 0 {
			start, _ = skip.item()
		}
		avoidBreaks := false
		for _, group := range table.Children[min(max(start, 0), len(table.Children)):] {
			if !group.table().IsHeader && !group.table().IsFooter {
				avoidBreaks = avoidPageBreak(str(group.Style.Get("break_inside")), lc)
				break
			}
		}

		if header != nil && footer != nil {
			// Try with both the header and footer.
			result := bodyGroupsLayout(skip, positionY+headerHeight, bottomSpace+footerHeight, avoidBreaks)
			if len(result.children) > 0 || len(tableRows) == 0 || !pageIsEmpty {
				footer.Translate(0, result.positionY-footer.PositionY, false)
				result.positionY += footerHeight
				result.header, result.footer = header, footer
				return result
			}
			// We could not fit any content, drop the footer.
			footer = nil
		}

		if header != nil && footer == nil {
			// Try with just the header.
			result := bodyGroupsLayout(skip, positionY+headerHeight, bottomSpace, avoidBreaks)
			if len(result.children) > 0 || len(tableRows) == 0 || !pageIsEmpty {
				result.header, result.footer = header, footer
				return result
			}
			// We could not fit any content, drop the header.
			header = nil
		}

		if footer != nil && header == nil {
			// Try with just the footer.
			result := bodyGroupsLayout(skip, positionY, bottomSpace+footerHeight, avoidBreaks)
			if len(result.children) > 0 || len(tableRows) == 0 || !pageIsEmpty {
				footer.Translate(0, result.positionY-footer.PositionY, false)
				result.positionY += footerHeight
				result.header, result.footer = header, footer
				return result
			}
			// We could not fit any content, drop the footer.
		}

		return bodyGroupsLayout(skip, positionY, bottomSpace, pageIsEmpty)
	}

	result := allGroupsLayout()
	header, newTableChildren, footer := result.header, result.children, result.footer
	positionY = result.positionY
	resumeAt, nextPage := result.resumeAt, result.nextPage

	if result.failed {
		return blockLayoutResult{nextPage: nextPage, maxLines: noMaxLines}
	}

	children := make([]*Box, 0, len(newTableChildren)+2)
	if header != nil {
		children = append(children, header)
	}
	children = append(children, newTableChildren...)
	if footer != nil {
		children = append(children, footer)
	}
	table = table.CopyWithChildren(children)
	columnGroups := make([]*Box, len(table.table().ColumnGroups))
	for i, columnGroup := range table.table().ColumnGroups {
		columnGroups[i] = columnGroup.DeepCopy()
	}
	table.ownTable().ColumnGroups = columnGroups
	removeEndDecoration := resumeAt != nil && !hasFooter
	if cloneCollapsed && removeEndDecoration && len(newTableChildren) > 0 {
		// Port: with clone, the bottom border of this part of the table is
		// the other half of the collapsed border below its last row.
		group := newTableChildren[len(newTableChildren)-1]
		if len(group.Children) > 0 {
			table.BorderBottomWidth = 0
			for _, cell := range group.Children[len(group.Children)-1].Children {
				table.BorderBottomWidth = max(table.BorderBottomWidth, cell.BorderBottomWidth)
			}
		}
	}
	table.RemoveDecoration(removeStartDecoration, removeEndDecoration)
	if collapse {
		table.ownTable().SkippedRows = skippedRows
	}

	// If the height property has a bigger value, just add blank space
	// below the last row group.
	height := table.Height
	if height == Auto {
		height = 0
	}
	table.Height = max(height, positionY-table.ContentBoxY())

	// Layout column groups and columns.
	columnsHeight := positionY - initialPositionY
	if len(table.Children) > 0 {
		// The last border spacing is below the columns.
		columnsHeight -= borderSpacingY
	}
	laidOutTable := table
	for _, group := range table.table().ColumnGroups {
		for _, column := range group.Children {
			resolvePercentages(column, table)
			if column.table().GridX >= 0 && column.table().GridX < len(columnPositions) {
				column.PositionX = columnPositions[column.table().GridX]
				column.PositionY = initialPositionY
				column.Width = columnWidths[column.table().GridX]
				column.Height = columnsHeight
			} else {
				// Ignore extra empty columns.
				column.PositionX = 0
				column.PositionY = 0
				column.Width = 0
				column.Height = 0
			}
			resolvePercentages(group, table)
			column.ownTable().getCells = tableColumnCells(laidOutTable, column)
		}
		if len(group.Children) == 0 {
			// Port: upstream fails on a column group without columns.
			continue
		}
		first := group.Children[0]
		last := group.Children[len(group.Children)-1]
		group.PositionX = first.PositionX
		group.PositionY = initialPositionY
		group.Width = last.PositionX + last.Width - first.PositionX
		group.Height = columnsHeight
	}

	// Invert columns for drawing.
	if isStr(table.Style.Get("direction"), "rtl") {
		slices.Reverse(columnWidths)
		slices.Reverse(columnPositions)
	}

	avoidBreak := avoidPageBreak(str(table.Style.Get("break_inside")), lc)
	if len(resumeAt) > 0 && !pageIsEmpty && avoidBreak {
		removePlaceholders(lc, []*Box{table}, absoluteBoxes, fixedBoxes)
		table = nil
		resumeAt = nil
	}

	return blockLayoutResult{box: table, resumeAt: resumeAt, nextPage: nextPage, maxLines: noMaxLines}
}

// tableColumnCells is get_column_cells, the nested function of
// table_layout in weasyprint/layout/table.py: it returns the function that
// gives the cells of table that start in column.
func tableColumnCells(table, column *Box) func() []*Box {
	return func() []*Box {
		var cells []*Box
		for _, rowGroup := range table.Children {
			for _, row := range rowGroup.Children {
				for _, cell := range row.Children {
					if cell.table().GridX == column.table().GridX {
						cells = append(cells, cell)
					}
				}
			}
		}
		return cells
	}
}

// addTopPadding is add_top_padding in weasyprint/layout/table.py: it
// increases the top padding of a box. This also translates the children.
//
// Port: an absolutely positioned box of the cell with a top or a bottom of
// its own stays in place, because the padding box of the cell, its
// containing block, does not move. Upstream moves it with the content.
func addTopPadding(box *Box, extraPadding float64) {
	box.PaddingTop += extraPadding
	for _, child := range box.Children {
		child.Translate(0, extraPadding, false)
	}
	keepAbsolute(box, extraPadding)
}

// keepAbsolute moves back by dy the absolutely positioned descendants of box
// that have a top or a bottom, and whose containing block is box or
// outside it.
func keepAbsolute(box *Box, dy float64) {
	for _, child := range box.AllChildren() {
		switch {
		case child.IsAbsolutelyPositioned():
			if !isStr(child.Style.Get("top"), "auto") || !isStr(child.Style.Get("bottom"), "auto") {
				child.Translate(0, -dy, false)
			}
		case !isStr(child.Style.Get("position"), "static"):
			// a positioned box is the containing block of its absolutely
			// positioned descendants, which move with it
		default:
			keepAbsolute(child, dy)
		}
	}
}

// fixedTableLayout is fixed_table_layout in weasyprint/layout/table.py: it
// runs the fixed table layout and sets the widths of the columns.
//
// https://www.w3.org/TR/CSS21/tables.html#fixed-table-layout
func fixedTableLayout(box *Box) {
	table := box.GetWrappedTable()
	if table == nil {
		// Port: a table wrapper always holds a table upstream.
		return
	}
	// Port: upstream asserts that the width of the table is not auto.

	var allColumns []*Box
	for _, columnGroup := range table.table().ColumnGroups {
		allColumns = append(allColumns, columnGroup.Children...)
	}
	var firstRowCells []*Box
	if len(table.Children) > 0 && len(table.Children[0].Children) > 0 {
		firstRowgroup := table.Children[0]
		firstRowCells = firstRowgroup.Children[0].Children
	}
	colspans := 0
	for _, cell := range firstRowCells {
		colspans += max(cell.table().Colspan, 0)
	}
	numColumns := max(len(allColumns), colspans)
	// known is false for the None of upstream: not known yet.
	columnWidths := make([]float64, numColumns)
	known := make([]bool, numColumns)

	// Set width on column boxes.
	for i, column := range allColumns {
		resolveOnePercentage(column, "width", table.Width)
		if column.Width != Auto {
			columnWidths[i] = column.Width
			known[i] = true
		}
	}

	borderSpacingX := 0.0
	if isStr(table.Style.Get("border_collapse"), "separate") {
		borderSpacingX, _ = tableBorderSpacing(table.Style)
	}

	// Set width on cells of the first row.
	i := 0
	for _, cell := range firstRowCells {
		resolvePercentages(cell, table)
		if cell.Width != Auto {
			width := cell.BorderWidth()
			width -= borderSpacingX * float64(cell.table().Colspan-1)
			// In the general case, this width affects several columns
			// (through colspan) some of which already have a width.
			// Subtract these known widths and divide among remaining
			// columns.
			var columnsWithoutWidth []int // and occupied by this cell
			for j := i; j < min(i+cell.table().Colspan, numColumns); j++ {
				if !known[j] {
					columnsWithoutWidth = append(columnsWithoutWidth, j)
				} else {
					width -= columnWidths[j]
				}
			}
			if len(columnsWithoutWidth) > 0 {
				widthPerColumn := width / float64(len(columnsWithoutWidth))
				for _, j := range columnsWithoutWidth {
					columnWidths[j] = widthPerColumn
					known[j] = true
				}
			}
		}
		i += max(cell.table().Colspan, 0)
	}

	// Distribute the remaining space equally on columns that do not have
	// a width yet.
	allBorderSpacing := borderSpacingX * float64(numColumns+1)
	minTableWidth := allBorderSpacing
	var columnsWithoutWidth []int
	for i, w := range columnWidths {
		if known[i] {
			minTableWidth += w
		} else {
			columnsWithoutWidth = append(columnsWithoutWidth, i)
		}
	}
	if len(columnsWithoutWidth) > 0 && table.Width >= minTableWidth {
		remainingWidth := table.Width - minTableWidth
		widthPerColumn := remainingWidth / float64(len(columnsWithoutWidth))
		for _, i := range columnsWithoutWidth {
			columnWidths[i] = widthPerColumn
		}
	} else {
		// This is bad, but we were given a broken table.
		for _, i := range columnsWithoutWidth {
			columnWidths[i] = 0
		}
	}

	// If the sum is less than the table width, distribute the remaining
	// space equally.
	extraWidth := table.Width - sumFloats(columnWidths) - allBorderSpacing
	if extraWidth <= 0 {
		// Substract a negative: widen the table.
		table.Width -= extraWidth
	} else if numColumns > 0 {
		extraPerColumn := extraWidth / float64(numColumns)
		for i := range columnWidths {
			columnWidths[i] += extraPerColumn
		}
	}

	// Now we have table.width == sum(column_widths) + all_border_spacing
	// with possible floating point rounding errors (unless there is zero
	// column).
	table.ownTable().ColumnWidths = columnWidths
}

// autoTableLayout is auto_table_layout in weasyprint/layout/table.py: it
// runs the auto table layout and sets the widths of the columns.
// containingBlock is a box or a sizeBlock.
//
// https://www.w3.org/TR/CSS21/tables.html#auto-table-layout
func autoTableLayout(lc *layoutContext, box, containingBlock *Box) {
	table := box.GetWrappedTable()
	if table == nil {
		// Port: a table wrapper always holds a table upstream.
		return
	}
	widths := tableAndColumnsPreferredWidths(lc, box, false)
	tableMinContentWidth, tableMaxContentWidth := widths.minContentWidth, widths.maxContentWidth
	columnMinContentWidths := widths.columnMinContentWidths
	columnMaxContentWidths := widths.columnMaxContentWidths
	columnIntrinsicPercentages := widths.columnIntrinsicPercentages
	constrainedness := widths.constrainedness
	grid := widths.grid

	margins := 0.0
	if box.MarginLeft != Auto {
		margins += box.MarginLeft
	}
	if box.MarginRight != Auto {
		margins += box.MarginRight
	}
	paddings := table.PaddingLeft + table.PaddingRight
	borders := table.BorderLeftWidth + table.BorderRightWidth

	cbWidth := containingBlock.Width
	availableWidth := cbWidth - margins - paddings - borders

	if table.Width == Auto {
		switch {
		case availableWidth <= tableMinContentWidth:
			table.Width = tableMinContentWidth
		case availableWidth < tableMaxContentWidth:
			table.Width = availableWidth
		default:
			table.Width = tableMaxContentWidth
		}
	} else if table.Width < tableMinContentWidth {
		table.Width = tableMinContentWidth
	}

	if len(grid) == 0 {
		table.ownTable().ColumnWidths = []float64{}
		return
	}

	assignableWidth := table.Width - widths.totalHorizontalBorderSpacing
	minContentGuess := slices.Clone(columnMinContentWidths)
	minContentPercentageGuess := slices.Clone(columnMinContentWidths)
	minContentSpecifiedGuess := slices.Clone(columnMinContentWidths)
	maxContentGuess := slices.Clone(columnMaxContentWidths)
	guesses := [...][]float64{
		minContentGuess, minContentPercentageGuess,
		minContentSpecifiedGuess, maxContentGuess,
	}
	// See https://www.w3.org/TR/css-tables-3/#width-distribution-algorithm.
	for i := range grid {
		if columnIntrinsicPercentages[i] != 0 {
			minContentPercentageGuess[i] = max(
				columnIntrinsicPercentages[i]/100*assignableWidth,
				columnMinContentWidths[i])
			minContentSpecifiedGuess[i] = minContentPercentageGuess[i]
			maxContentGuess[i] = minContentPercentageGuess[i]
		} else if constrainedness[i] {
			// Any other column that is constrained is assigned its
			// max-content width.
			minContentSpecifiedGuess[i] = columnMaxContentWidths[i]
		}
	}

	// We have to work around floating point rounding errors here.
	// The 1e-9 value comes from PEP 485.
	if assignableWidth*(1+1e-9) < sumFloats(maxContentGuess) {
		// Port: upstream fails when no guess is lower, or no guess is
		// upper, than the assignable width. The port takes the first and
		// the last guesses.
		lowerGuess, upperGuess := guesses[0], guesses[len(guesses)-1]
		for _, guess := range guesses {
			if sumFloats(guess) <= assignableWidth*(1+1e-9) {
				lowerGuess = guess
			} else {
				break
			}
		}
		for i := len(guesses) - 1; i >= 0; i-- {
			if sumFloats(guesses[i]) >= assignableWidth*(1-1e-9) {
				upperGuess = guesses[i]
			} else {
				break
			}
		}
		if slices.Equal(upperGuess, lowerGuess) {
			table.ownTable().ColumnWidths = upperGuess
		} else {
			addedWidths := make([]float64, len(grid))
			for i := range grid {
				addedWidths[i] = upperGuess[i] - lowerGuess[i]
			}
			addedSum := sumFloats(addedWidths)
			if addedSum == 0 {
				// Port: upstream divides by zero.
				table.ownTable().ColumnWidths = lowerGuess
				return
			}
			availableRatio := (assignableWidth - sumFloats(lowerGuess)) / addedSum
			columnWidths := make([]float64, len(grid))
			for i := range grid {
				columnWidths[i] = lowerGuess[i] + addedWidths[i]*availableRatio
			}
			table.ownTable().ColumnWidths = columnWidths
		}
	} else {
		table.ownTable().ColumnWidths = maxContentGuess
		excessWidth := assignableWidth - sumFloats(maxContentGuess)
		distributeExcessWidth(lc, grid, excessWidth, table.table().ColumnWidths, constrainedness,
			columnIntrinsicPercentages, columnMaxContentWidths, 0, len(grid))
	}
}

// tableWrapperWidth is table_wrapper_width in weasyprint/layout/table.py:
// it finds the width of each column and derives the width of the wrapper.
// containingBlock is a box or a sizeBlock.
func tableWrapperWidth(lc *layoutContext, wrapper, containingBlock *Box) {
	table := wrapper.GetWrappedTable()
	if table == nil {
		// Port: a table wrapper always holds a table upstream.
		wrapper.Width = 0
		return
	}
	resolvePercentages(table, containingBlock)

	if isStr(table.Style.Get("table_layout"), "fixed") && table.Width != Auto {
		fixedTableLayout(wrapper)
	} else {
		autoTableLayout(lc, wrapper, containingBlock)
	}

	wrapper.Width = table.BorderWidth()
}

// cellBaseline is cell_baseline in weasyprint/layout/table.py: the
// position of the baseline of a cell from the top of its border box.
//
// See https://www.w3.org/TR/CSS21/tables.html#height-layout
func cellBaseline(cell *Box) float64 {
	if result, ok := findInFlowBaseline(cell, false, KindLine, KindTableRow); ok {
		return result - cell.PositionY
	}
	// Default to the bottom of the content area.
	return cell.BorderTopWidth + cell.PaddingTop + cell.Height
}

// findInFlowBaseline is find_in_flow_baseline in
// weasyprint/layout/table.py: the absolute position of the first, or the
// last, in-flow baseline of box, and false when there is no in-flow
// baseline. baselineTypes are the kinds of boxes whose baseline counts.
// None gives the line boxes, the default of upstream.
func findInFlowBaseline(box *Box, last bool, baselineTypes ...Kind) (float64, bool) {
	if len(baselineTypes) == 0 {
		baselineTypes = []Kind{KindLine}
	}
	// TODO: synthetize baseline when needed.
	// See https://www.w3.org/TR/css-align-3/#synthesize-baseline.
	for _, kind := range baselineTypes {
		if box.Is(kind) {
			return box.PositionY + box.Baseline, true
		}
	}
	if box.Kind == KindTableCaption {
		return 0, false
	}
	children := box.InFlowChildren()
	if last {
		slices.Reverse(children)
	}
	for _, child := range children {
		if result, ok := findInFlowBaseline(child, last, baselineTypes...); ok {
			return result, true
		}
	}
	return 0, false
}

// distributeExcessWidth is distribute_excess_width in
// weasyprint/layout/table.py: it distributes the available width to the
// columns of the slice [start:end] of grid, the zipped grid.
//
// See https://www.w3.org/TR/css-tables-3/#distributing-width-to-columns
func distributeExcessWidth(lc *layoutContext, grid [][]*Box, excessWidth float64, columnWidths []float64, constrainedness []bool, columnIntrinsicPercentages, columnMaxContentWidths []float64, start, end int) {
	start, end = pySlice(start, end, true, len(grid))
	lc.spend(1)
	group := func(test func(i int) bool) []int {
		var columns []int
		for i := start; i < end; i++ {
			if test(i) {
				columns = append(columns, i)
			}
		}
		return columns
	}

	// First group.
	columns := group(func(i int) bool {
		return !constrainedness[i] && columnIntrinsicPercentages[i] == 0 && columnMaxContentWidths[i] > 0
	})
	if len(columns) > 0 {
		sumMaxContentWidths := 0.0
		for _, i := range columns {
			sumMaxContentWidths += columnMaxContentWidths[i]
		}
		ratio := excessWidth / sumMaxContentWidths
		for _, i := range columns {
			columnWidths[i] += columnMaxContentWidths[i] * ratio
		}
		return
	}

	// Second group.
	columns = group(func(i int) bool {
		return !constrainedness[i] && columnIntrinsicPercentages[i] == 0
	})
	if len(columns) > 0 {
		for _, i := range columns {
			columnWidths[i] += excessWidth / float64(len(columns))
		}
		return
	}

	// Third group.
	columns = group(func(i int) bool {
		return constrainedness[i] && columnIntrinsicPercentages[i] == 0 && columnMaxContentWidths[i] > 0
	})
	if len(columns) > 0 {
		sumMaxContentWidths := 0.0
		for _, i := range columns {
			sumMaxContentWidths += columnMaxContentWidths[i]
		}
		ratio := excessWidth / sumMaxContentWidths
		for _, i := range columns {
			columnWidths[i] += columnMaxContentWidths[i] * ratio
		}
		return
	}

	// Fourth group.
	columns = group(func(i int) bool {
		return columnIntrinsicPercentages[i] > 0 && columnMaxContentWidths[i] > 0
	})
	if len(columns) > 0 {
		sumIntrinsicPercentages := 0.0
		for _, i := range columns {
			sumIntrinsicPercentages += columnIntrinsicPercentages[i]
		}
		ratio := excessWidth / sumIntrinsicPercentages
		for _, i := range columns {
			columnWidths[i] += columnIntrinsicPercentages[i] * ratio
		}
		return
	}

	// Fifth group.
	columns = group(func(i int) bool {
		return len(grid[i]) > 0
	})
	if len(columns) > 0 {
		for _, i := range columns {
			columnWidths[i] += excessWidth / float64(len(columns))
		}
		return
	}

	// Sixth group.
	columns = group(func(int) bool { return true })
	for _, i := range columns {
		columnWidths[i] += excessWidth / float64(len(columns))
	}
}

// tableFindEarlierPageBreak is _find_earlier_page_break in
// weasyprint/layout/table.py. It returns the children to keep and where to
// resume, and a nil skip stack for (None, None).
func tableFindEarlierPageBreak(lc *layoutContext, element *Box, index int, previousList []*Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box) ([]*Box, skipStack) {
	if len(previousList) > 0 {
		pageBreak := blockLevelPageBreak(previousList[len(previousList)-1], element)
		if avoidPageBreak(pageBreak, lc) {
			if children, resumeAt, ok := findEarlierPageBreak(lc, previousList, absoluteBoxes, fixedBoxes); ok {
				return children, resumeAt
			}
		} else {
			return previousList, skipStack{index: nil}
		}
	}

	if pageIsEmpty {
		return previousList, skipStack{index: nil}
	}
	return nil, nil
}
