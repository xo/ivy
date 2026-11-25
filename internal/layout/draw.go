// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/draw/__init__.py: it takes a box tree after the
// layout and draws it onto a stream, which records the commands of the
// display list of D10.
//
// Port: upstream marks the content for the tagged PDF with stream.marked and
// stream.artifact. The port draws no tags, so these calls are left out.

package layout

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/xo/okage/tinycss2"
)

// drawPage is draw_page in weasyprint/draw/__init__.py: it draws a page box.
func drawPage(s *stream, page *Box) {
	stackingContext := StackingContextFromPage(page)
	drawBackground(s, stackingContext.Box.Background, false)
	setMaskBorder(s, page)
	drawBackground(s, page.extra().CanvasBackground, false)
	drawBorder(s, page)
	drawStackingContext(s, stackingContext)
	drawMarks(s, page)
}

// drawStackingContext is draw_stacking_context in
// weasyprint/draw/__init__.py. See
// https://www.w3.org/TR/CSS2/zindex.html.
func drawStackingContext(s *stream, stackingContext *StackingContext) {
	if s.dc.stopped() {
		return
	}
	s.stacked(func() {
		box := stackingContext.Box

		// Apply the viewport_overflow to the html box, see #35.
		if box.IsForRootElement && !isStr(stackingContext.Page.Style.Get("overflow"), "visible") {
			roundedBox(s, stackingContext.Page.RoundedPaddingBox())
			s.clip(false)
			s.end()
		}

		if box.IsAbsolutelyPositioned() {
			if clip, ok := box.Style.Get("clip").(Tuple); ok && len(clip) == 4 {
				top, right, bottom, left := clipValue(clip[0], 0), clipValue(clip[1], 0),
					clipValue(clip[2], box.BorderHeight()), clipValue(clip[3], box.BorderWidth())
				s.rectangle(box.BorderBoxX()+right, box.BorderBoxY()+top, left-right, bottom-top)
				s.clip(false)
				s.end()
			}
		}

		opacity := num(box.Style.Get("opacity"))
		originalStream := s
		if opacity < 1 {
			r := s.dc.pageRectangle
			s = s.addGroup(r[0], r[1], r[2], r[3])
		}

		if m, ok := box.extra().TransformationMatrix.(Matrix); ok {
			if m.Determinant() == 0 {
				return
			}
			s.transform(m.A, m.B, m.C, m.D, m.E, m.F)
		}

		// Point 1 is done in drawPage.

		// Point 2.
		if box.Is(KindBlock) || box.Kind == KindMargin || box.Is(KindInlineBlock) || box.Is(KindTableCell) ||
			box.IsFlexContainer() || box.IsGridContainer() || box.IsReplaced() {
			setMaskBorder(s, box)
			// The canvas background was removed by layout_backgrounds.
			drawBackground(s, box.Background, true)
			drawBorder(s, box)
		}

		s.stacked(func() {
			// Do not clip the page box, see #35.
			if !isStr(box.Style.Get("overflow"), "visible") && box.Kind != KindPage {
				// Only clip the content and the children:
				// - the background is already clipped,
				// - the border must *not* be clipped.
				roundedBox(s, box.RoundedPaddingBox())
				s.clip(false)
				s.end()
			}

			// Point 3.
			for _, childContext := range stackingContext.NegativeZContexts {
				drawStackingContext(s, childContext)
			}

			// Point 4.
			for _, block := range stackingContext.BlockLevelBoxes {
				setMaskBorder(s, block)
				if block.IsTable() {
					drawTable(s, block)
				} else {
					drawBackground(s, block.Background, true)
					drawBorder(s, block)
				}
			}

			// Point 5.
			for _, childContext := range stackingContext.FloatContexts {
				drawStackingContext(s, childContext)
			}

			// Point 6.
			if box.Kind == KindInline {
				drawInlineLevel(s, stackingContext, box, 0, "clip", nil)
			}

			// Point 7.
			drawBlockLevel(s, stackingContext, []BlockAndCells{{Box: box, Children: stackingContext.BlocksAndCells}})

			// Point 8.
			for _, childContext := range stackingContext.ZeroZContexts {
				drawStackingContext(s, childContext)
			}

			// Point 9.
			for _, childContext := range stackingContext.PositiveZContexts {
				drawStackingContext(s, childContext)
			}
		})

		// Point 10.
		drawOutline(s, box, stackingContext.inline)

		if opacity < 1 {
			group := s
			s = originalStream
			s.stacked(func() {
				s.setAlpha(opacity, true, true)
				s.drawXObject(group)
			})
		}
	})
}

// lengthValue returns the value of a computed length, the attribute value
// of a Dimension of upstream, or the number.
func lengthValue(v Value) float64 {
	if d, ok := v.(Dimension); ok {
		return d.Value
	}
	return num(v)
}

// clipValue returns a length of the property clip, or auto for 'auto'.
func clipValue(v Value, auto float64) float64 {
	if d, ok := v.(Dimension); ok {
		return d.Value
	}
	if isStr(v, "auto") {
		return auto
	}
	return num(v)
}

// spreadBox is _spread_box in weasyprint/draw/__init__.py.
func spreadBox(box RoundedBox, spread, tx, ty float64) RoundedBox {
	if spread == 0 && tx == 0 && ty == 0 {
		return box
	}

	// Update the coordinates.
	result := RoundedBox{
		X:      box.X + tx - spread,
		Y:      box.Y + ty - spread,
		Width:  max(0, box.Width+2*spread),
		Height: max(0, box.Height+2*spread),
	}

	// Update the radii.
	radius := func(r [2]float64) [2]float64 {
		if r[0] >= -spread && r[1] >= -spread && r[0] != 0 && r[1] != 0 {
			return [2]float64{r[0] + spread, r[1] + spread}
		}
		return [2]float64{0, 0}
	}
	result.TopLeft = radius(box.TopLeft)
	result.TopRight = radius(box.TopRight)
	result.BottomRight = radius(box.BottomRight)
	result.BottomLeft = radius(box.BottomLeft)
	return result
}

// drawBoxShadow is _draw_box_shadow in weasyprint/draw/__init__.py.
func drawBoxShadow(s *stream, box RoundedBox, style *Style, tx, ty, blur, spread float64, color *tinycss2.Color, inset bool) {
	s.setColor(color, false)

	// The values of the original rounded box.
	shadowBox := spreadBox(box, spread, tx, ty)
	innerBox := spreadBox(box, spread-blur, tx, ty)
	outerBox := spreadBox(box, spread+blur, tx, ty)

	// Mask the original box.
	if !inset {
		roundedBox(s, outerBox)
	}
	roundedBox(s, box)
	s.clip(true)
	s.end()

	// No blur: render the rounded box with the spread applied.
	if blur == 0 {
		if inset {
			roundedBox(s, box)
		}
		roundedBox(s, shadowBox)
		s.fill(true)
		return
	}

	// Fill the outer rectangle if inset, the plain rectangle if outset.
	if inset {
		roundedBox(s, box)
		roundedBox(s, outerBox)
		s.fill(true)
	} else {
		roundedBox(s, innerBox)
		s.fill(false)
	}

	// The variables of the blur: the sides and the corners of linear
	// gradients.
	r, g, b, _ := srgbOf(color)
	transparent := Color{newSRGB(r, g, b, 0)}
	repeat := false
	shape := "ellipse"
	x, y, w, h := innerBox.X, innerBox.Y, innerBox.Width, innerBox.Height
	tl, tr, br, bl := innerBox.TopLeft, innerBox.TopRight, innerBox.BottomRight, innerBox.BottomLeft
	sides := [4][5]float64{
		{0, x + tl[0], y - 2*blur, w - tl[0] - tr[0], 2 * blur},
		{math.Pi / 2, x + w, y + tr[1], 2 * blur, h - tr[1] - br[1]},
		{math.Pi, x + bl[0], y + h, w - bl[0] - br[0], 2 * blur},
		{3 * math.Pi / 2, x - 2*blur, y + tl[1], 2 * blur, h - tl[1] - bl[1]},
	}
	corners := [4][6]float64{
		{100, 100, x - 2*blur, y - 2*blur, tl[0], tl[1]},
		{0, 100, x + w - tr[0], y - 2*blur, tr[0], tr[1]},
		{0, 0, x + w - br[0], y + h - br[1], br[0], br[1]},
		{100, 0, x - 2*blur, y + h - bl[1], bl[0], bl[1]},
	}
	inColor, outColor := Value(Color{color}), Value(transparent)
	if inset {
		inColor, outColor = transparent, Color{color}
	}

	// Draw the sides.
	stops := [][2]Value{{inColor, nil}, {outColor, nil}}
	hints := []Value{Dimension{blur * 1.25, "px"}}
	for _, side := range sides {
		angle, e, f, width, height := side[0], side[1], side[2], side[3], side[4]
		gradient := newLinearGradient(stops, Tuple{String("angle"), Float(angle)}, repeat, hints)
		s.stacked(func() {
			s.translate(e, f)
			gradient.draw(s, width, height, style)
		})
	}

	// Draw the corners.
	s.stacked(func() {
		roundedBox(s, innerBox)
		roundedBox(s, outerBox)
		s.clip(true)
		s.end()
		for _, corner := range corners {
			left, top, e, f, cx, cy := corner[0], corner[1], corner[2], corner[3], corner[4], corner[5]
			width, height := 2*blur+cx, 2*blur+cy
			size := Tuple{String("explicit"), Tuple{Dimension{width, "px"}, Dimension{height, "px"}}}
			stops := [][2]Value{{inColor, Dimension{cx, "px"}}, {outColor, nil}}
			hints := []Value{Dimension{cx + blur*1.25, "px"}}
			center := Tuple{String("left"), Dimension{left, "%"}, String("top"), Dimension{top, "%"}}
			gradient := newRadialGradient(stops, shape, size, center, repeat, hints)
			s.stacked(func() {
				s.translate(e, f)
				gradient.draw(s, width, height, style)
			})
		}
	})
}

// drawMarks is draw_marks in weasyprint/draw/__init__.py: it draws the
// marks of the page.
func drawMarks(s *stream, page *Box) {
	bleed := page.Bleed()
	marks, _ := page.Style.Get("marks").(Tuple)
	if len(marks) == 0 || (bleed["top"] == 0 && bleed["right"] == 0 && bleed["bottom"] == 0 && bleed["left"] == 0) {
		return
	}
	area := page.BleedArea()
	x, y, w, h := area[0], area[1], area[2], area[3]
	t, r, b, l := bleed["top"], bleed["right"], bleed["bottom"], bleed["left"]
	sc := "scale(0.5)"
	marksSVG := ""
	if tupleHas(marks, "crop") {
		marksSVG = fmt.Sprintf(`
          <path d="M0,%[1]g h%[3]g"/>
          <path d="M0,%[1]g h%[4]g" transform="translate(%[5]g,0) scale(-1,1)"/>
          <path d="M0,%[2]g h%[4]g" transform="translate(%[5]g,%[6]g) scale(-1,-1)"/>
          <path d="M0,%[2]g h%[3]g" transform="translate(0,%[6]g) scale(1,-1)"/>
          <path d="M%[7]g,0 v%[9]g"/>
          <path d="M%[8]g,0 v%[10]g" transform="translate(%[5]g,%[6]g) scale(-1,-1)"/>
          <path d="M%[7]g,0 v%[10]g" transform="translate(0,%[6]g) scale(1,-1)"/>
          <path d="M%[8]g,0 v%[9]g" transform="translate(%[5]g,0) scale(-1,1)"/>`,
			t, b, l/2, r/2, w, h, l, r, t/2, b/2)
	}
	crossSVG := ""
	if tupleHas(marks, "cross") {
		crossSVG = fmt.Sprintf(`
          <circle r="%[1]g" transform="%[9]s translate(%[5]g,%[1]g) %[9]s"/>
          <path d="M-%[1]g,%[1]g h%[2]g M0,0 v%[2]g" transform="%[9]s translate(%[5]g,0)"/>
          <circle r="%[3]g" transform="translate(0,%[6]g) %[9]s translate(%[5]g,-%[3]g) %[9]s"/>
          <path d="M-%[3]g,-%[3]g h%[4]g M0,0 v-%[4]g"
                transform="translate(0,%[6]g) %[9]s translate(%[5]g,0)"/>
          <circle r="%[7]g" transform="%[9]s translate(%[7]g,%[6]g) %[9]s"/>
          <path d="M%[7]g,-%[7]g v%[8]g M0,0 h%[8]g" transform="%[9]s translate(0,%[6]g)"/>
          <circle r="%[10]g" transform="translate(%[5]g,0) %[9]s translate(-%[10]g,%[6]g) %[9]s"/>
          <path d="M-%[10]g,-%[10]g v%[11]g M0,0 h-%[11]g"
                transform="translate(%[5]g,0) %[9]s translate(0,%[6]g)"/>`,
			t/2, t, b/2, b, w, h, l/2, l, sc, r/2, r)
	}
	svgData := fmt.Sprintf(`
          <svg height="%g" width="%g" fill="transparent" stroke="black"
               stroke-width="1" xmlns="http://www.w3.org/2000/svg">
            %s %s
          </svg>`, h, w, marksSVG, crossSVG)
	image, err := newSVGImage([]byte(svgData), "", nil, s.dc.limits)
	if err != nil {
		return
	}
	s.stacked(func() {
		s.translate(x, y)
		image.draw(s, w, h, nil)
	})
}

// drawBoxShadows is draw_box_shadows in weasyprint/draw/__init__.py.
func drawBoxShadows(s *stream, boxes []RoundedBox, style *Style, inset bool) {
	shadows, _ := style.Get("box_shadow").(Tuple)
	for i := len(shadows) - 1; i >= 0; i-- {
		shadow, ok := shadows[i].(Tuple)
		if !ok || len(shadow) != 6 {
			continue
		}
		// Only draw the inset or the outset shadows.
		shadowInset, _ := shadow[5].(Bool)
		if inset != bool(shadowInset) {
			continue
		}
		tx, ty, blur := lengthValue(shadow[0]), lengthValue(shadow[1]), lengthValue(shadow[2])
		spread := lengthValue(shadow[3])
		if inset {
			spread = -spread
		}
		color := getColor(style, "color")
		if c, ok := shadow[4].(Color); ok && c.Color != nil {
			color = c.Color
		}
		for _, box := range boxes {
			s.stacked(func() {
				drawBoxShadow(s, box, style, tx, ty, blur, spread, color, inset)
			})
		}
	}
}

// drawBackground is draw_background in weasyprint/draw/__init__.py: it
// draws the background color and the images. If clipBox is false, the
// background is not clipped to the border box of the background, but only
// to the painting area.
func drawBackground(s *stream, background *Background, clipBox bool) {
	if background == nil || len(background.Layers) == 0 {
		return
	}

	last := background.Layers[len(background.Layers)-1]
	clippedBoxes := last.ClippedBoxes
	paintingArea := last.PaintingArea

	drawBoxShadows(s, clippedBoxes, background.Style, false)

	s.stacked(func() {
		if clipBox {
			for _, box := range clippedBoxes {
				roundedBox(s, box)
			}
			s.clip(false)
			s.end()
		}

		// Draw the background color.
		if background.Color != nil && background.Color.Alpha > 0 {
			s.stacked(func() {
				s.setColor(background.Color, false)
				s.rectangle(paintingArea[0], paintingArea[1], paintingArea[2], paintingArea[3])
				s.clip(false)
				s.end()
				s.rectangle(paintingArea[0], paintingArea[1], paintingArea[2], paintingArea[3])
				s.fill(false)
			})
		}

		// Paint in the reversed order: the first layer is "closest" to the
		// viewer.
		for i := len(background.Layers) - 1; i >= 0; i-- {
			drawBackgroundImage(s, background.Layers[i], background.Style)
		}
	})

	drawBoxShadows(s, clippedBoxes, background.Style, true)
}

// drawBackgroundImage is draw_background_image in
// weasyprint/draw/__init__.py.
func drawBackgroundImage(s *stream, layer BackgroundLayer, style *Style) {
	if layer.Image == nil || layer.Size[0] == 0 || layer.Size[1] == 0 {
		return
	}

	paintingX, paintingY, paintingWidth, paintingHeight := layer.PaintingArea[0], layer.PaintingArea[1],
		layer.PaintingArea[2], layer.PaintingArea[3]
	positioningX, positioningY, positioningWidth, positioningHeight := layer.PositioningArea[0],
		layer.PositioningArea[1], layer.PositioningArea[2], layer.PositioningArea[3]
	positionX, positionY := layer.Position[0], layer.Position[1]
	repeatX, repeatY := layer.Repeat[0], layer.Repeat[1]
	imageWidth, imageHeight := layer.Size[0], layer.Size[1]

	if repeatX == "no-repeat" && repeatY == "no-repeat" {
		// A pattern is not used when it is not needed, because some
		// viewers (e.g., Preview on Mac) introduce unnecessary pixelation
		// when vector images are used in patterns.
		if !layer.Unbounded {
			s.rectangle(paintingX, paintingY, paintingWidth, paintingHeight)
			s.clip(false)
			s.end()
		}
		// Put the image in a group so that masking outside the image and
		// masking within the image do not conflict.
		r := s.dc.pageRectangle
		group := s.addGroup(r[0], r[1], r[2], r[3])
		group.translate(positionX+positioningX, positionY+positioningY)
		paintImage(group, layer.Image, imageWidth, imageHeight, style)
		s.drawXObject(group)
		return
	}

	var repeatWidth, repeatHeight float64
	switch repeatX {
	case "no-repeat":
		// At least the whole image_width is drawn on the sub-surface, but
		// it must not be repeated on the painting_width. The painting width
		// is doubled to ensure viewers do not incorrectly bleed the edge of
		// the pattern into the painting area. (See #1539.)
		repeatWidth = max(imageWidth, 2*paintingWidth)
	case "repeat", "round":
		// The image repeats each image_width.
		repeatWidth = imageWidth
	default:
		// Port: upstream asserts that repeat_x is 'space'.
		nRepeats := math.Floor(positioningWidth / imageWidth)
		if nRepeats >= 2 {
			// The repeat width is the whole positioning width with one
			// image removed, divided by (the number of repeated images -
			// 1). This way, the width of one image + one space is got.
			// background-position is ignored for this dimension.
			repeatWidth = (positioningWidth - imageWidth) / (nRepeats - 1)
			positionX = 0
		} else {
			// The image is not repeated.
			repeatWidth = positioningWidth
		}
	}

	// The comments above apply here too.
	switch repeatY {
	case "no-repeat":
		repeatHeight = max(imageHeight, 2*paintingHeight)
	case "repeat", "round":
		repeatHeight = imageHeight
	default:
		nRepeats := math.Floor(positioningHeight / imageHeight)
		if nRepeats >= 2 {
			repeatHeight = (positioningHeight - imageHeight) / (nRepeats - 1)
			positionY = 0
		} else {
			repeatHeight = positioningHeight
		}
	}

	matrix := translation(positionX+positioningX, positionY+positioningY).Mul(s.ctm())
	pattern := s.addPattern(0, 0, imageWidth, imageHeight, repeatWidth, repeatHeight, matrix)
	group := pattern.addGroup(0, 0, repeatWidth, repeatHeight)

	s.stacked(func() {
		paintImage(group, layer.Image, imageWidth, imageHeight, style)
		pattern.drawXObject(group)
		s.setColorPattern(pattern)
		if layer.Unbounded {
			r := s.dc.pageRectangle
			s.rectangle(r[0], r[1], r[2], r[3])
		} else {
			s.rectangle(paintingX, paintingY, paintingWidth, paintingHeight)
		}
		s.fill(false)
	})
}

// drawTable is draw_table in weasyprint/draw/__init__.py.
func drawTable(s *stream, table *Box) {
	// Draw the backgrounds.
	drawBackground(s, table.Background, true)
	for _, columnGroup := range table.table().ColumnGroups {
		drawBackground(s, columnGroup.Background, true)
		for _, column := range columnGroup.Children {
			drawBackground(s, column.Background, true)
		}
	}
	collapse := isStr(table.Style.Get("border_collapse"), "collapse")
	for _, rowGroup := range table.Children {
		drawBackground(s, rowGroup.Background, true)
		for _, row := range rowGroup.Children {
			drawBackground(s, row.Background, true)
			for _, cell := range row.Children {
				if collapse || isStr(cell.Style.Get("empty_cells"), "show") || !cell.table().Empty {
					drawBackground(s, cell.Background, true)
				}
			}
		}
	}

	// Draw the borders.
	if collapse {
		drawCollapsedBorders(s, table)
		return
	}
	drawBorder(s, table)
	for _, rowGroup := range table.Children {
		for _, row := range rowGroup.Children {
			for _, cell := range row.Children {
				if isStr(cell.Style.Get("empty_cells"), "show") || !cell.table().Empty {
					drawBorder(s, cell)
				}
			}
		}
	}
}

// collapsedSegment is a segment of a collapsed border.
type collapsedSegment struct {
	score  [3]float64
	style  string
	width  float64
	color  *tinycss2.Color
	side   string
	border [4]float64
	adjust bool
}

// drawCollapsedBorders is draw_collapsed_borders in
// weasyprint/draw/__init__.py: it draws the borders of the cells of a table
// whose borders collapse.
func drawCollapsedBorders(s *stream, table *Box) {
	var rowHeights, rowPositions []float64
	for _, rowGroup := range table.Children {
		for _, row := range rowGroup.Children {
			rowHeights = append(rowHeights, row.Height)
			rowPositions = append(rowPositions, row.PositionY)
		}
	}
	columnWidths := table.table().ColumnWidths
	if len(rowHeights) == 0 || len(columnWidths) == 0 || table.table().CollapsedBorderGrid == nil {
		// One of the lists is empty: do not bother with empty tables.
		return
	}
	columnPositions := slices.Clone(table.table().ColumnPositions)
	gridHeight := len(rowHeights)
	gridWidth := len(columnWidths)
	// Port: upstream asserts that the grid has a position for each column.
	if gridWidth != len(columnPositions) {
		return
	}
	verticalBorders, horizontalBorders := table.table().CollapsedBorderGrid.Vertical, table.table().CollapsedBorderGrid.Horizontal
	// Add the end of the last column.
	columnPositions = append(columnPositions, columnPositions[len(columnPositions)-1]+columnWidths[len(columnWidths)-1])
	// Add the end of the last row.
	rowPositions = append(rowPositions, rowPositions[len(rowPositions)-1]+rowHeights[len(rowHeights)-1])
	headerRows, footerRows := 0, 0
	if table.Children[0].table().IsHeader {
		headerRows = len(table.Children[0].Children)
	}
	if last := table.Children[len(table.Children)-1]; last.table().IsFooter {
		footerRows = len(last.Children)
	}
	bodyRowsOffset := 0
	if table.table().SkippedRows != 0 {
		bodyRowsOffset = table.table().SkippedRows - headerRows
	}
	originalGridHeight := len(verticalBorders)
	footerRowsOffset := originalGridHeight - gridHeight

	rowNumber := func(y int, horizontal bool) int {
		// The examples in the comments are for 2 header rows, 5 body rows
		// and 3 footer rows.
		h := 0
		if horizontal {
			h = 1
		}
		switch {
		case headerRows != 0 && y < headerRows+h:
			// A row in the header: y < 2 for vertical, y < 3 for
			// horizontal.
			return y
		case footerRows != 0 && y >= gridHeight-footerRows:
			// A row in the footer: y >= 7 for vertical and horizontal.
			return y + footerRowsOffset
		}
		// A row in the body: 2 >= y > 7 for vertical, 3 >= y > 7 for
		// horizontal.
		return y + bodyRowsOffset
	}

	// border returns the border at (yy, x) of a grid, or false where
	// upstream raises IndexError.
	border := func(list [][]CollapsedBorder, yy, x int) (CollapsedBorder, bool) {
		if yy < 0 || yy >= len(list) || x < 0 || x >= len(list[yy]) {
			return CollapsedBorder{}, false
		}
		return list[yy][x], true
	}

	var segments []collapsedSegment

	halfMaxWidth := func(borderList [][]CollapsedBorder, yxPairs [][2]int, vertical bool) float64 {
		result := 0.0
		for _, yx := range yxPairs {
			y, x := yx[0], yx[1]
			var inside bool
			if vertical {
				inside = 0 <= y && y < gridHeight && 0 <= x && x <= gridWidth
			} else {
				inside = 0 <= y && y <= gridHeight && 0 <= x && x < gridWidth
			}
			if inside {
				yy := rowNumber(y, !vertical)
				if b, ok := border(borderList, yy, x); ok {
					result = max(result, b.Width)
				}
			}
		}
		return result / 2
	}

	addVertical := func(x, y int) {
		yy := rowNumber(y, false)
		b, ok := border(verticalBorders, yy, x)
		if !ok || b.Width == 0 || b.Color == nil || b.Color.Alpha == 0 {
			return
		}
		posX := columnPositions[x]
		posY1 := rowPositions[y]
		if y != 0 || !table.table().SkipCellBorderTop {
			posY1 -= halfMaxWidth(horizontalBorders, [][2]int{{y, x - 1}, {y, x}}, false)
		}
		posY2 := rowPositions[y+1]
		if y != gridHeight-1 || !table.table().SkipCellBorderBottom {
			posY2 += halfMaxWidth(horizontalBorders, [][2]int{{y + 1, x - 1}, {y + 1, x}}, false)
		}
		segments = append(segments, collapsedSegment{
			score: b.score, style: b.Style, width: b.Width, color: b.Color, side: "left",
			border: [4]float64{posX, posY1, 0, posY2 - posY1}, adjust: b.adjust,
		})
	}

	addHorizontal := func(x, y int) {
		if y == 0 && table.table().SkipCellBorderTop {
			return
		}
		if y == gridHeight && table.table().SkipCellBorderBottom {
			return
		}
		yy := rowNumber(y, true)
		b, ok := border(horizontalBorders, yy, x)
		if !ok || b.Width == 0 || b.Color == nil || b.Color.Alpha == 0 {
			return
		}
		posY := rowPositions[y]
		shiftBefore := halfMaxWidth(verticalBorders, [][2]int{{y - 1, x}, {y, x}}, true)
		shiftAfter := halfMaxWidth(verticalBorders, [][2]int{{y - 1, x + 1}, {y, x + 1}}, true)
		posX1 := columnPositions[x] - shiftBefore
		posX2 := columnPositions[x+1] + shiftAfter
		segments = append(segments, collapsedSegment{
			score: b.score, style: b.Style, width: b.Width, color: b.Color, side: "top",
			border: [4]float64{posX1, posY, posX2 - posX1, 0}, adjust: b.adjust,
		})
	}

	for x := range gridWidth {
		addHorizontal(x, 0)
	}
	for y := range gridHeight {
		addVertical(0, y)
		for x := range gridWidth {
			addVertical(x+1, y)
			addHorizontal(x, y+1)
		}
		if s.dc.stopped() {
			return
		}
	}

	// Sort the bigger scores last (painted later, on top).
	slices.SortStableFunc(segments, func(a, b collapsedSegment) int {
		for i := range a.score {
			if c := cmp.Compare(a.score[i], b.score[i]); c != 0 {
				return c
			}
		}
		return 0
	})

	for _, segment := range segments {
		bx, by, bw, bh := segment.border[0], segment.border[1], segment.border[2], segment.border[3]
		color := styledColor(segment.style, segment.color, segment.side)
		s.stacked(func() {
			// Port: upstream has no -ivy-stroke-adjust.
			s.state.strokeAdjust = segment.adjust
			drawLine(s, bx, by, bx+bw, by+bh, segment.width, segment.style, color, 0)
		})
	}
}

// drawReplacedbox is draw_replacedbox in weasyprint/draw/__init__.py: it
// draws a replaced box.
func drawReplacedbox(s *stream, box *Box) {
	if !isStr(box.Style.Get("visibility"), "visible") || box.Width == 0 || box.Height == 0 {
		return
	}

	drawWidth, drawHeight, drawX, drawY := replacedBoxLayout(box)
	if drawWidth <= 0 || drawHeight <= 0 {
		return
	}

	s.stacked(func() {
		s.setAlpha(1, false, true)
		s.translate(drawX, drawY)
		s.stacked(func() {
			// TODO: Use the real intrinsic size here, not affected by
			// 'image-resolution'?
			paintImage(s, box.extra().Replacement, drawWidth, drawHeight, box.Style)
		})
	})
}

// drawInlineLevel is draw_inline_level in weasyprint/draw/__init__.py.
// sc is the stacking context whose page and inline contexts it reads.
func drawInlineLevel(s *stream, sc *StackingContext, box *Box, offsetX float64, textOverflow string, blockEllipsis Value) {
	if s.dc.stopped() {
		return
	}
	if stackingContext, ok := sc.inline[box]; ok {
		// Port: upstream asserts that the box of the context is an inline
		// block, an inline flex or an inline grid box.
		drawStackingContext(s, stackingContext)
		return
	}
	setMaskBorder(s, box)
	drawBackground(s, box.Background, true)
	drawBorder(s, box)
	switch box.Kind {
	case KindInline, KindLine:
		if box.Kind == KindLine {
			textOverflow = box.extra().TextOverflow
			blockEllipsis = box.extra().BlockEllipsis
		}
		var ellipsis Value
		for i, child := range box.Children {
			if i == len(box.Children)-1 {
				// The last child.
				ellipsis = blockEllipsis
			}
			childOffsetX := offsetX
			if _, ok := sc.inline[child]; !ok {
				childOffsetX = offsetX + child.PositionX - box.PositionX
			}
			if child.Kind == KindText {
				drawText(s, child, childOffsetX, textOverflow, ellipsis)
			} else {
				drawInlineLevel(s, sc, child, childOffsetX, textOverflow, ellipsis)
			}
		}
	case KindInlineReplaced:
		drawReplacedbox(s, box)
	case KindText:
		// It should only happen for the list markers.
		drawText(s, box, offsetX, textOverflow, nil)
	}
}

// drawBlockLevel is draw_block_level in weasyprint/draw/__init__.py.
func drawBlockLevel(s *stream, sc *StackingContext, blocksAndCells []BlockAndCells) {
	for _, item := range blocksAndCells {
		if s.dc.stopped() {
			return
		}
		block := item.Box
		if block.IsReplaced() {
			drawReplacedbox(s, block)
		} else if len(block.Children) > 0 {
			if block.Children[len(block.Children)-1].Kind == KindLine {
				for _, child := range block.Children {
					drawInlineLevel(s, sc, child, 0, "clip", nil)
				}
			}
		}
		drawBlockLevel(s, sc, item.Children)
	}
}
