// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/draw/border.py: the borders, the border
// images, the mask borders, the column rules and the outlines.

package layout

import (
	"math"

	"github.com/xo/okage/tinycss2"
)

// sides is SIDES in weasyprint/draw/border.py.
var sides = [4]string{"top", "right", "bottom", "left"}

// The bounds of the drawing of the borders.
const (
	// maxBorderDashes is the number of dashes or dots that a side of a
	// border, a line or a wavy line draws.
	//
	// Port: upstream has no bound. A dotted border of a tiny width on a
	// large box makes millions of paths. Past the bound, the side or the
	// line is drawn solid.
	maxBorderDashes = 1 << 16
	// maxBorderImageRepeats is the number of copies of the image that a
	// region of a border image draws.
	//
	// Port: upstream has no bound. Each copy is clipped, and a tiny slice
	// on a large box makes millions of copies. The copies past the bound
	// are left out.
	maxBorderImageRepeats = 1024
)

// pixels returns a length of a computed style in pixels: a number, or a
// Dimension in pixels.
func pixels(v Value) float64 {
	if d, ok := v.(Dimension); ok {
		return d.Value
	}
	return num(v)
}

// sameColor reports whether two colors are the same, as the operator == of
// the colors of tinycss2 does.
func sameColor(a, b *tinycss2.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(b) && a.Alpha == b.Alpha
}

// setMaskBorder is set_mask_border in weasyprint/draw/border.py: it sets the
// mask border of the box as an alpha state on the stream.
func setMaskBorder(s *stream, box *Box) {
	source, _ := box.Style.Get("mask_border_source").(Tuple)
	if len(source) == 0 || isStr(source[0], "none") || box.MaskBorderImage == nil {
		return
	}
	r := box.RoundedBorderBox()
	mode := "luminosity"
	if isStr(box.Style.Get("mask_border_mode"), "alpha") {
		mode = "alpha"
	}
	maskStream := s.setAlphaState(r.X, r.Y, r.Width, r.Height, mode)
	drawBorderImage(box, maskStream, box.MaskBorderImage, box.Style.Get("mask_border_slice"),
		box.Style.Get("mask_border_repeat"), box.Style.Get("mask_border_outset"),
		box.Style.Get("mask_border_width"))
}

// drawColumnRules is draw_column_rules in weasyprint/draw/border.py: it
// draws the column rules.
func drawColumnRules(s *stream, box *Box) {
	skipNext := true
	ruleWidth := pixels(box.Style.Get("column_rule_width"))
	ruleStyle := str(box.Style.Get("column_rule_style"))
	borderWidths := [4]float64{0, 0, 0, ruleWidth}
	var gap float64
	if isStr(box.Style.Get("column_gap"), "normal") {
		gap = num(box.Style.Get("font_size")) // normal equals 1em
	} else {
		gap, _ = percentageFloat(box.Style.Get("column_gap"), box.Style, box.Width)
	}
	for _, child := range box.Children {
		if isStr(child.Style.Get("column_span"), "all") {
			skipNext = true
			continue
		} else if skipNext {
			skipNext = false
			continue
		}
		s.stacked(func() {
			var positionX float64
			if isStr(box.Style.Get("direction"), "rtl") {
				positionX = child.PositionX + child.Width + (gap-ruleWidth)/2
			} else {
				positionX = child.PositionX - (gap+ruleWidth)/2
			}
			borderBox := [4]float64{positionX, child.PositionY, ruleWidth, child.Height}
			clipBorderSegment(s, ruleStyle, ruleWidth, "left", borderBox, &borderWidths, nil)
			color := styledColor(ruleStyle, getColor(box.Style, "column_rule_color"), "left")
			drawRectBorder(s, borderBox, borderWidths, ruleStyle, color)
		})
	}
}

// drawBorder is draw_border in weasyprint/draw/border.py: it draws the
// borders and the column rules of the box.
func drawBorder(s *stream, box *Box) {
	// The box is hidden, easy.
	if !isStr(box.Style.Get("visibility"), "visible") {
		return
	}

	// Draw the column rules.
	columns := box.IsBlockContainer() &&
		(!isStr(box.Style.Get("column_width"), "auto") || !isStr(box.Style.Get("column_count"), "auto"))
	if columns && pixels(box.Style.Get("column_rule_width")) != 0 {
		drawColumnRules(s, box)
	}

	// If there is a border image, that takes precedence.
	if source, _ := box.Style.Get("border_image_source").(Tuple); len(source) > 0 && !isStr(source[0], "none") &&
		box.BorderImage != nil {
		drawBorderImage(box, s, box.BorderImage, box.Style.Get("border_image_slice"),
			box.Style.Get("border_image_repeat"), box.Style.Get("border_image_outset"),
			box.Style.Get("border_image_width"))
		return
	}

	widths := [4]float64{box.BorderTopWidth, box.BorderRightWidth, box.BorderBottomWidth, box.BorderLeftWidth}

	if widths == [4]float64{} {
		// No border, return early.
		return
	}

	var colors [4]*tinycss2.Color
	var styles [4]string
	for i, side := range sides {
		colors[i] = getColor(box.Style, "border_"+side+"_color")
		// Port: a transparent color gives the style 0 upstream, the empty
		// string here.
		if colors[i] != nil && colors[i].Alpha != 0 {
			styles[i] = str(box.Style.Get("border_" + side + "_style"))
		}
	}

	// One style, simple lines.
	simpleStyle := (styles[0] == "solid" || styles[0] == "double") &&
		styles[1] == styles[0] && styles[2] == styles[0] && styles[3] == styles[0]
	// One color.
	singleColor := sameColor(colors[0], colors[1]) && sameColor(colors[0], colors[2]) && sameColor(colors[0], colors[3])
	// No border of a width of 0, to avoid artifacts of PDF.
	fourSides := widths[0] != 0 && widths[1] != 0 && widths[2] != 0 && widths[3] != 0
	if simpleStyle && singleColor && fourSides {
		// Simple case, only draw rounded rectangles.
		s.stacked(func() {
			drawRoundedBorder(s, box, styles[0], []*tinycss2.Color{colors[0]})
		})
		return
	}

	// Fast path for dotted and dashed: a uniform border without rounded
	// corners. One line is stroked for each side, with the dash arrays of
	// PDF, which is orders of magnitude smaller in the PDF than the
	// generic clip path for each segment.
	rounded := box.RoundedBorderBox()
	noRadii := rounded.TopLeft == [2]float64{} && rounded.TopRight == [2]float64{} &&
		rounded.BottomRight == [2]float64{} && rounded.BottomLeft == [2]float64{}
	dottedDashed := (styles[0] == "dotted" || styles[0] == "dashed") &&
		styles[1] == styles[0] && styles[2] == styles[0] && styles[3] == styles[0]
	oneWidth := widths[1] == widths[0] && widths[2] == widths[0] && widths[3] == widths[0]
	if dottedDashed && singleColor && fourSides && oneWidth && noRadii {
		s.stacked(func() {
			drawDottedDashedBorder(s, box, styles[0], widths[0], colors[0])
		})
		return
	}

	// There is no smart way to find a good way to draw the borders, so
	// they are drawn side by side. The order is not specified, but this one
	// seems to be close to what other browsers do.
	borderBox := [4]float64{rounded.X, rounded.Y, rounded.Width, rounded.Height}
	radii := [4][2]float64{rounded.TopLeft, rounded.TopRight, rounded.BottomRight, rounded.BottomLeft}
	for _, index := range [4]int{2, 3, 1, 0} {
		side, width, color, style := sides[index], widths[index], colors[index], styles[index]
		if width == 0 || color == nil {
			continue
		}
		s.stacked(func() {
			clipBorderSegment(s, style, width, side, borderBox, &widths, &radii)
			drawRoundedBorder(s, box, style, styledColor(style, color, side))
		})
	}
}

// drawBorderImage is draw_border_image in weasyprint/draw/border.py: it
// draws the image as a border image of the box. The border images and the
// mask borders share it.
func drawBorderImage(box *Box, s *stream, image Image, borderSlice, borderRepeat, borderOutset, borderWidth Value) {
	size := image.GetIntrinsicSize(num(box.Style.Get("image_resolution")), num(box.Style.Get("font_size")))
	var width, height, ratio *float64
	if size.HasWidth {
		width = &size.Width
	}
	if size.HasHeight {
		height = &size.Height
	}
	if size.HasRatio {
		ratio = &size.Ratio
	}
	intrinsicWidth, intrinsicHeight := defaultImageSizing(width, height, ratio, nil, nil,
		box.BorderWidth(), box.BorderHeight())

	slices, _ := borderSlice.(Tuple)
	if len(slices) < 5 {
		// Port: upstream fails on a slice of less than 5 values.
		return
	}
	imageSlice := slices[:4]
	shouldFill := slices[4] != nil

	computeSliceDimension := func(dimension Value, intrinsic float64) float64 {
		if isNumber(dimension) {
			return min(num(dimension), intrinsic)
		}
		// Port: upstream asserts that the unit is %.
		d, _ := dimension.(Dimension)
		return min(100, d.Value) / 100 * intrinsic
	}

	sliceTop := computeSliceDimension(imageSlice[0], intrinsicHeight)
	sliceRight := computeSliceDimension(imageSlice[1], intrinsicWidth)
	sliceBottom := computeSliceDimension(imageSlice[2], intrinsicHeight)
	sliceLeft := computeSliceDimension(imageSlice[3], intrinsicWidth)

	repeats, _ := borderRepeat.(Tuple)
	repeatX, repeatY := "stretch", "stretch"
	if len(repeats) == 2 {
		repeatX, repeatY = str(repeats[0]), str(repeats[1])
	}

	r := box.RoundedBorderBox()
	x, y, w, h := r.X, r.Y, r.Width, r.Height
	p := box.RoundedPaddingBox()
	px, py, pw, ph := p.X, p.Y, p.Width, p.Height
	borderLeft := px - x
	borderTop := py - y
	borderRight := w - pw - borderLeft
	borderBottom := h - ph - borderTop

	outsets, _ := borderOutset.(Tuple)
	if len(outsets) < 4 {
		outsets = Tuple{Int(0), Int(0), Int(0), Int(0)}
	}
	computeOutsetDimension := func(dimension Value, fromBorder float64) float64 {
		if d, ok := dimension.(Dimension); ok {
			if d.Unit == "" {
				return d.Value * fromBorder
			}
			// Port: upstream asserts that the unit is px.
			return d.Value
		}
		return num(dimension) * fromBorder
	}

	outsetTop := computeOutsetDimension(outsets[0], borderTop)
	outsetRight := computeOutsetDimension(outsets[1], borderRight)
	outsetBottom := computeOutsetDimension(outsets[2], borderBottom)
	outsetLeft := computeOutsetDimension(outsets[3], borderLeft)

	x -= outsetLeft
	y -= outsetTop
	w += outsetLeft + outsetRight
	h += outsetTop + outsetBottom

	computeWidthAdjustment := func(dimension Value, original, intrinsic, areaDimension float64) float64 {
		switch {
		case isStr(dimension, "auto"):
			return intrinsic
		case isNumber(dimension):
			return num(dimension) * original
		}
		d, _ := dimension.(Dimension)
		if d.Unit == "%" {
			return d.Value / 100 * areaDimension
		}
		// Port: upstream asserts that the unit is px.
		return d.Value
	}

	// The border_* variables are adjusted after the outsets are handled,
	// because the numerical outsets are relative to border-width, not to
	// border-image-width. Also, the area of the border image that the
	// percentages of border-image-width use includes the area that
	// border-image-outset expands.
	widths, _ := borderWidth.(Tuple)
	if len(widths) < 4 {
		widths = Tuple{Int(1), Int(1), Int(1), Int(1)}
	}
	borderTop = computeWidthAdjustment(widths[0], borderTop, sliceTop, h)
	borderRight = computeWidthAdjustment(widths[1], borderRight, sliceRight, w)
	borderBottom = computeWidthAdjustment(widths[2], borderBottom, sliceBottom, h)
	borderLeft = computeWidthAdjustment(widths[3], borderLeft, sliceLeft, w)

	drawRegion := func(x, y, width, height, sliceX, sliceY, sliceWidth, sliceHeight float64,
		repeatX, repeatY string, scaleX, scaleY float64,
	) (float64, float64) {
		var nRepeatsX, nRepeatsY, extraDX, extraDY float64
		if intrinsicWidth == 0 || width == 0 || sliceWidth == 0 {
			scaleX = 0
		} else {
			if scaleX == 0 {
				scaleX = 1
				if height != 0 && sliceHeight != 0 {
					scaleX = height / sliceHeight
				}
			}
			switch repeatX {
			case "repeat":
				nRepeatsX = math.Ceil(width / sliceWidth / scaleX)
			case "space":
				nRepeatsX = math.Floor(width / sliceWidth / scaleX)
				// The space is before the first repeat and after the
				// last, so there is one more space than repeats.
				extraDX = (width/scaleX - nRepeatsX*sliceWidth) / (nRepeatsX + 1)
			case "round":
				nRepeatsX = max(1, math.RoundToEven(width/sliceWidth/scaleX))
				scaleX = width / (nRepeatsX * sliceWidth)
			default:
				nRepeatsX = 1
				scaleX = width / sliceWidth
			}
		}

		if intrinsicHeight == 0 || height == 0 || sliceHeight == 0 {
			scaleY = 0
		} else {
			if scaleY == 0 {
				scaleY = 1
				if width != 0 && sliceWidth != 0 {
					scaleY = width / sliceWidth
				}
			}
			switch repeatY {
			case "repeat":
				nRepeatsY = math.Ceil(height / sliceHeight / scaleY)
			case "space":
				nRepeatsY = math.Floor(height / sliceHeight / scaleY)
				// The space is before the first repeat and after the
				// last, so there is one more space than repeats.
				extraDY = (height/scaleY - nRepeatsY*sliceHeight) / (nRepeatsY + 1)
			case "round":
				nRepeatsY = max(1, math.RoundToEven(height/sliceHeight/scaleY))
				scaleY = height / (nRepeatsY * sliceHeight)
			default:
				nRepeatsY = 1
				scaleY = height / sliceHeight
			}
		}

		if scaleX == 0 || scaleY == 0 {
			return scaleX, scaleY
		}

		// Port: the copies of a region are bounded.
		nx := int(min(max(nRepeatsX, 0), maxBorderImageRepeats))
		ny := 0
		if nx > 0 {
			ny = int(min(max(nRepeatsY, 0), float64(maxBorderImageRepeats/nx)))
		}

		renderedWidth := intrinsicWidth * scaleX
		renderedHeight := intrinsicHeight * scaleY
		offsetX := renderedWidth * sliceX / intrinsicWidth
		offsetY := renderedHeight * sliceY / intrinsicHeight

		s.stacked(func() {
			s.rectangle(x, y, width, height)
			s.clip(false)
			s.end()
			s.translate(x-offsetX+extraDX, y-offsetY+extraDY)
			s.transform(scaleX, 0, 0, scaleY, 0, 0)
			for i := range nx {
				for j := range ny {
					s.stacked(func() {
						translateX := float64(i) * (sliceWidth + extraDX)
						translateY := float64(j) * (sliceHeight + extraDY)
						s.translate(translateX, translateY)
						s.rectangle(offsetX/scaleX, offsetY/scaleY, sliceWidth, sliceHeight)
						s.clip(false)
						s.end()
						paintImage(s, image, intrinsicWidth, intrinsicHeight, box.Style)
					})
				}
			}
		})

		return scaleX, scaleY
	}

	// Top left.
	scaleLeft, scaleTop := drawRegion(x, y, borderLeft, borderTop, 0, 0, sliceLeft, sliceTop,
		"stretch", "stretch", 0, 0)
	// Top right.
	drawRegion(x+w-borderRight, y, borderRight, borderTop,
		intrinsicWidth-sliceRight, 0, sliceRight, sliceTop, "stretch", "stretch", 0, 0)
	// Bottom right.
	scaleRight, scaleBottom := drawRegion(x+w-borderRight, y+h-borderBottom, borderRight, borderBottom,
		intrinsicWidth-sliceRight, intrinsicHeight-sliceBottom, sliceRight, sliceBottom,
		"stretch", "stretch", 0, 0)
	// Bottom left.
	drawRegion(x, y+h-borderBottom, borderLeft, borderBottom,
		0, intrinsicHeight-sliceBottom, sliceLeft, sliceBottom, "stretch", "stretch", 0, 0)
	xMiddle := sliceLeft+sliceRight < intrinsicWidth
	if xMiddle {
		// Top middle.
		drawRegion(x+borderLeft, y, w-borderLeft-borderRight, borderTop,
			sliceLeft, 0, intrinsicWidth-sliceLeft-sliceRight, sliceTop, repeatX, "stretch", 0, 0)
		// Bottom middle.
		drawRegion(x+borderLeft, y+h-borderBottom, w-borderLeft-borderRight, borderBottom,
			sliceLeft, intrinsicHeight-sliceBottom, intrinsicWidth-sliceLeft-sliceRight, sliceBottom,
			repeatX, "stretch", 0, 0)
	}
	yMiddle := sliceTop+sliceBottom < intrinsicHeight
	if yMiddle {
		// Right middle.
		drawRegion(x+w-borderRight, y+borderTop, borderRight, h-borderTop-borderBottom,
			intrinsicWidth-sliceRight, sliceTop, sliceRight, intrinsicHeight-sliceTop-sliceBottom,
			"stretch", repeatY, 0, 0)
		// Left middle.
		drawRegion(x, y+borderTop, borderLeft, h-borderTop-borderBottom,
			0, sliceTop, sliceLeft, intrinsicHeight-sliceTop-sliceBottom, "stretch", repeatY, 0, 0)
	}
	if shouldFill && xMiddle && yMiddle {
		// Fill the middle.
		scaleX := scaleLeft
		if scaleX == 0 {
			scaleX = scaleRight
		}
		scaleY := scaleTop
		if scaleY == 0 {
			scaleY = scaleBottom
		}
		drawRegion(x+borderLeft, y+borderTop, w-borderLeft-borderRight, h-borderTop-borderBottom,
			sliceLeft, sliceTop, intrinsicWidth-sliceLeft-sliceRight, intrinsicHeight-sliceTop-sliceBottom,
			repeatX, repeatY, scaleX, scaleY)
	}
}

// clipBorderSegment is clip_border_segment in weasyprint/draw/border.py: it
// clips one segment of the border of a box. The strategy is to remove the
// zones that the style or the side do not need before the painting.
// borderWidths and radii can be nil.
func clipBorderSegment(s *stream, style string, width float64, side string, borderBox [4]float64,
	borderWidths *[4]float64, radii *[4][2]float64,
) {
	bbx, bby, bbw, bbh := borderBox[0], borderBox[1], borderBox[2], borderBox[3]
	var corners [4][2]float64
	if radii != nil {
		corners = *radii
	}
	tlh, tlv := corners[0][0], corners[0][1]
	trh, trv := corners[1][0], corners[1][1]
	brh, brv := corners[2][0], corners[2][1]
	blh, blv := corners[3][0], corners[3][1]
	bt, br, bb, bl := width, width, width, width
	if borderWidths != nil {
		bt, br, bb, bl = borderWidths[0], borderWidths[1], borderWidths[2], borderWidths[3]
	}

	// transitionPoint returns the point used for the transition of the
	// border. The extra boolean is true if the point is in the padding box
	// (ie. the padding box is rounded).
	//
	// This point is not specified. It must be inside the rounded padding
	// box, and in the "transition zone" that the specification allows. The
	// corner of the transition zone is chosen. It is easy to get and gives
	// quite good results, but it seems to be different from what other
	// browsers do.
	transitionPoint := func(x1, y1, x2, y2 float64) (float64, float64, bool) {
		if math.Abs(x1) > math.Abs(x2) && math.Abs(y1) > math.Abs(y2) {
			return x1, y1, true
		}
		return x2, y2, false
	}

	// cornerHalfLength returns the length of the half of one corner of an
	// ellipse.
	//
	// Inspired by [Ramanujan, S., "Modular Equations and Approximations to
	// pi" Quart. J. Pure. Appl. Math., vol. 45 (1913-1914), pp. 350-372],
	// wonderfully explained by Dr Rob.
	//
	// https://mathforum.org/dr.math/faq/formulas/
	cornerHalfLength := func(a, b float64) float64 {
		x := (a - b) / (a + b)
		return math.Pi / 8 * (a + b) * (1 + 3*x*x/(10+math.Sqrt(4-3*x*x)))
	}

	// drawDash draws a single dash or dot centered on cx, cy.
	drawDash := func(cx, cy, width, height, r float64) {
		switch style {
		case "dotted":
			ratio := r / math.Sqrt(math.Pi)
			s.moveTo(cx+r, cy)
			s.curveTo(cx+r, cy+ratio, cx+ratio, cy+r, cx, cy+r)
			s.curveTo(cx-ratio, cy+r, cx-r, cy+ratio, cx-r, cy)
			s.curveTo(cx-r, cy-ratio, cx-ratio, cy-r, cx, cy-r)
			s.curveTo(cx+ratio, cy-r, cx+r, cy-ratio, cx+r, cy)
			s.close()
		case "dashed":
			s.rectangle(cx-width/2, cy-height/2, width, height)
		}
	}

	var px1, py1, px2, py2 float64
	var rounded1, rounded2 bool
	var way, angle float64
	var mainOffset float64
	switch side {
	case "top":
		px1, py1, rounded1 = transitionPoint(tlh, tlv, bl, bt)
		px2, py2, rounded2 = transitionPoint(-trh, trv, -br, bt)
		width = bt
		way = 1
		angle = 1
		mainOffset = bby
	case "right":
		px1, py1, rounded1 = transitionPoint(-trh, trv, -br, bt)
		px2, py2, rounded2 = transitionPoint(-brh, -brv, -br, -bb)
		width = br
		way = 1
		angle = 2
		mainOffset = bbx + bbw
	case "bottom":
		px1, py1, rounded1 = transitionPoint(blh, -blv, bl, -bb)
		px2, py2, rounded2 = transitionPoint(-brh, -brv, -br, -bb)
		width = bb
		way = -1
		angle = 3
		mainOffset = bby + bbh
	default: // left
		px1, py1, rounded1 = transitionPoint(tlh, tlv, bl, bt)
		px2, py2, rounded2 = transitionPoint(blh, -blv, bl, -bb)
		width = bl
		way = -1
		angle = 4
		mainOffset = bbx
	}

	horizontal := side == "top" || side == "bottom"
	var a1, b1, a2, b2, lineLength, length float64
	if horizontal {
		a1, b1 = px1-bl/2, way*py1-width/2
		a2, b2 = -px2-br/2, way*py2-width/2
		lineLength = bbw - px1 + px2
		length = bbw
		s.moveTo(bbx+bbw, mainOffset)
		s.lineTo(bbx, mainOffset)
		s.lineTo(bbx+px1, mainOffset+py1)
		s.lineTo(bbx+bbw+px2, mainOffset+py2)
	} else {
		a1, b1 = -way*px1-width/2, py1-bt/2
		a2, b2 = -way*px2-width/2, -py2-bb/2
		lineLength = bbh - py1 + py2
		length = bbh
		s.moveTo(mainOffset, bby+bbh)
		s.lineTo(mainOffset, bby)
		s.lineTo(mainOffset+px1, bby+py1)
		s.lineTo(mainOffset+px2, bby+bbh+py2)
	}

	// Port: a side with more than maxBorderDashes dashes or dots is drawn
	// solid.
	if (style == "dotted" || style == "dashed") && !(length/width <= maxBorderDashes) {
		style = "solid"
	}

	if style == "dotted" || style == "dashed" {
		dash := width
		if style == "dashed" {
			dash = 3 * width
		}
		s.clip(true)
		s.end()
		if rounded1 || rounded2 {
			// At least one of the two corners is rounded.
			chl1 := cornerHalfLength(a1, b1)
			chl2 := cornerHalfLength(a2, b2)
			length = lineLength + chl1 + chl2
			dashLength := math.RoundToEven(length / dash)
			if rounded1 && rounded2 {
				// 2x dashes.
				dash = length / (dashLength + pyModFloat(dashLength, 2))
			} else {
				// 2x - 1/2 dashes.
				dash = length / (dashLength + pyModFloat(dashLength, 2) - 0.5)
			}
			// Port: a negative count of dashes in a corner is 0. Upstream
			// reads a variable that is not set there.
			dashes1 := max(0, int(math.Ceil((chl1-dash/2)/dash)))
			dashes2 := max(0, int(math.Ceil((chl2-dash/2)/dash)))
			line := max(0, int(math.Floor(lineLength/dash)))

			drawDashes := func(dashes, line int, way, x, y, px, py, chl float64) (int, float64) {
				if style == "dotted" {
					switch dashes {
					case 0:
						return line + 1, -1
					case 1:
						return line + 1, -0.5
					}

					i := 1
					for k := 1; k < dashes; k += 2 {
						i = k
						a := ((2*angle - way) + float64(i)*way*dash/chl) / 4 * math.Pi
						cx := x
						if !horizontal {
							cx = mainOffset
						}
						cy := mainOffset
						if !horizontal {
							cy = y
						}
						drawDash(cx+px-(math.Abs(px)-dash/2)*math.Cos(a),
							cy+py-(math.Abs(py)-dash/2)*math.Sin(a), 0, 0, dash/2)
					}
					nextA := ((2*angle - way) + float64(i+2)*way*dash/chl) / 4 * math.Pi
					offset := nextA/math.Pi*2 - angle
					if dashes%2 != 0 {
						line++
					}
					return line, offset
				}

				if dashes == 0 {
					return line + 1, -1.0 / 3
				}

				var angle1, angle2, offset float64
				broken := false
				for k := 0; k < dashes; k += 2 {
					i := float64(k) + 0.5 // half dash
					angle1 = ((2*angle - way) + i*way*dash/chl) / 4 * math.Pi
					next := ((2*angle - way) + (i+1)*way*dash/chl) / 4 * math.Pi
					if way > 0 {
						angle2 = min(next, angle*math.Pi/2)
					} else {
						angle2 = max(next, angle*math.Pi/2)
					}
					if horizontal {
						s.moveTo(x+px, mainOffset+py)
						s.lineTo(x+px-way*px*1/math.Tan(angle2), mainOffset)
						s.lineTo(x+px-way*px*1/math.Tan(angle1), mainOffset)
					} else {
						s.moveTo(mainOffset+px, y+py)
						s.lineTo(mainOffset, y+py+way*py*math.Tan(angle2))
						s.lineTo(mainOffset, y+py+way*py*math.Tan(angle1))
					}
					if angle2 == angle*math.Pi/2 {
						offset = (angle1 - angle2) / (next - angle1)
						line++
						broken = true
						break
					}
				}
				if !broken {
					offset = 1 - ((angle*math.Pi/2 - angle2) / (angle2 - angle1))
				}
				return line, offset
			}

			line, offset := drawDashes(dashes1, line, way, bbx, bby, px1, py1, chl1)
			line, _ = drawDashes(dashes2, line, -way, bbx+bbw, bby+bbh, px2, py2, chl2)

			if lineLength > 1e-6 {
				for k := 0; k < line; k += 2 {
					i := float64(k) + offset
					var x1, x2, y1, y2 float64
					if horizontal {
						x1 = bbx + px1 + i*dash
						x2 = bbx + px1 + (i+1)*dash
						y1 = mainOffset
						if way < 0 {
							y1 -= width
						}
						y2 = y1 + width
					} else {
						y1 = bby + py1 + i*dash
						y2 = bby + py1 + (i+1)*dash
						x1 = mainOffset
						if way > 0 {
							x1 -= width
						}
						x2 = x1 + width
					}
					drawDash(x1+(x2-x1)/2, y1+(y2-y1)/2, x2-x1, y2-y1, width/2)
				}
			}
		} else {
			// No rounded corner, dashes on the corners and evenly spaced
			// between.
			numberOfSpaces := math.Floor(length / dash / 2)
			numberOfDashes := numberOfSpaces + 1
			var space float64
			if style == "dotted" {
				dash = width
				if numberOfSpaces != 0 {
					space = (length - numberOfDashes*dash) / numberOfSpaces
				} else {
					space = 0 // No space, unused.
				}
			} else {
				dash = length / (numberOfSpaces + numberOfDashes)
				if dash == 0 {
					dash = 1
				}
				space = dash
			}
			for i := range int(numberOfDashes) + 1 {
				advance := float64(i) * (space + dash)
				var cx, cy, dashWidth, dashHeight float64
				switch side {
				case "top":
					cx, cy = bbx+advance+dash/2, bby+width/2
					dashWidth, dashHeight = dash, width
				case "right":
					cx, cy = bbx+bbw-width/2, bby+advance+dash/2
					dashWidth, dashHeight = width, dash
				case "bottom":
					cx, cy = bbx+advance+dash/2, bby+bbh-width/2
					dashWidth, dashHeight = dash, width
				default:
					cx, cy = bbx+width/2, bby+advance+dash/2
					dashWidth, dashHeight = width, dash
				}
				drawDash(cx, cy, dashWidth, dashHeight, dash/2)
			}
		}
	}
	s.clip(true)
	s.end()
}

// pyModFloat is the operator % of Python for floats: the result has the
// sign of the divisor.
func pyModFloat(a, b float64) float64 {
	m := math.Mod(a, b)
	if m != 0 && (m < 0) != (b < 0) {
		m += b
	}
	return m
}

// drawRoundedBorder is draw_rounded_border in weasyprint/draw/border.py.
// color holds one color, or two colors for ridge and groove.
func drawRoundedBorder(s *stream, box *Box, style string, color []*tinycss2.Color) {
	if style == "ridge" || style == "groove" {
		s.setColor(color[0], false)
		roundedBox(s, box.RoundedPaddingBox())
		roundedBox(s, box.RoundedBoxRatio(1.0/2))
		s.fill(true)
		s.setColor(color[len(color)-1], false)
		roundedBox(s, box.RoundedBoxRatio(1.0/2))
		roundedBox(s, box.RoundedBorderBox())
		s.fill(true)
		return
	}
	s.setColor(color[0], false)
	roundedBox(s, box.RoundedPaddingBox())
	if style == "double" {
		roundedBox(s, box.RoundedBoxRatio(1.0/3))
		roundedBox(s, box.RoundedBoxRatio(2.0/3))
	}
	roundedBox(s, box.RoundedBorderBox())
	s.fill(true)
}

// drawRectBorder is draw_rect_border in weasyprint/draw/border.py. color
// holds one color, or two colors for ridge and groove.
func drawRectBorder(s *stream, box, widths [4]float64, style string, color []*tinycss2.Color) {
	bbx, bby, bbw, bbh := box[0], box[1], box[2], box[3]
	bt, br, bb, bl := widths[0], widths[1], widths[2], widths[3]
	if style == "ridge" || style == "groove" {
		s.setColor(color[0], false)
		s.rectangle(bbx, bby, bbw, bbh)
		s.rectangle(bbx+bl/2, bby+bt/2, bbw-(bl+br)/2, bbh-(bt+bb)/2)
		s.fill(true)
		s.rectangle(bbx+bl/2, bby+bt/2, bbw-(bl+br)/2, bbh-(bt+bb)/2)
		s.rectangle(bbx+bl, bby+bt, bbw-bl-br, bbh-bt-bb)
		s.setColor(color[len(color)-1], false)
		s.fill(true)
		return
	}
	s.setColor(color[0], false)
	s.rectangle(bbx, bby, bbw, bbh)
	if style == "double" {
		s.rectangle(bbx+bl/3, bby+bt/3, bbw-(bl+br)/3, bbh-(bt+bb)/3)
		s.rectangle(bbx+bl*2/3, bby+bt*2/3, bbw-(bl+br)*2/3, bbh-(bt+bb)*2/3)
	}
	s.rectangle(bbx+bl, bby+bt, bbw-bl-br, bbh-bt-bb)
	s.fill(true)
}

// drawDottedDashedBorder is draw_dotted_dashed_border in
// weasyprint/draw/border.py: the fast path for a uniform dotted or dashed
// border without rounded corners.
//
// Dotted uses a round line cap with a dash of zero length, so each "on"
// position becomes a filled circle. Dashed uses a butt cap with an equal
// dash and gap, so each "on" segment becomes a flush rectangle. Either way,
// one stroked line for each side replaces hundreds of paths. Each side picks
// its own dash period, so that the dots or the dashes land on both corners,
// as the generic path does for each side.
func drawDottedDashedBorder(s *stream, box *Box, style string, width float64, color *tinycss2.Color) {
	r := box.RoundedBorderBox()
	bbx, bby, bbw, bbh := r.X, r.Y, r.Width, r.Height
	half := width / 2
	// Round caps extend the line by half on each end, so the ends of a
	// dotted line are inset to keep the dots centered on the corners. Butt
	// caps do not extend, so a dashed line runs flush from corner to
	// corner.
	inset := 0.0
	if style == "dotted" {
		inset = half
	}

	dash := func(outerLength float64) []float64 {
		if style == "dotted" {
			// A dot on each corner, with equal spacing between.
			count := max(2, math.Floor(outerLength/(2*width))+1)
			return []float64{0, (outerLength - width) / (count - 1)}
		}
		// The size of a dash is 3 * width. The dash and the gap are
		// equal, so that a whole count fits flush between the two corners.
		count := math.Floor(outerLength/(6*width)) + 1
		dashLength := outerLength / (2*count - 1)
		return []float64{dashLength, dashLength}
	}

	s.setColor(color, true)
	s.setLineWidth(width)
	if style == "dotted" {
		s.setLineCap(1)
	} else {
		s.setLineCap(0)
	}
	lines := [4][5]float64{
		{bbx + inset, bby + half, bbx + bbw - inset, bby + half, bbw},
		{bbx + bbw - half, bby + inset, bbx + bbw - half, bby + bbh - inset, bbh},
		{bbx + inset, bby + bbh - half, bbx + bbw - inset, bby + bbh - half, bbw},
		{bbx + half, bby + inset, bbx + half, bby + bbh - inset, bbh},
	}
	for _, l := range lines {
		x1, y1, x2, y2, outerLength := l[0], l[1], l[2], l[3], l[4]
		// Port: a side with more than maxBorderDashes dots or dashes is
		// drawn solid.
		if outerLength/width <= maxBorderDashes {
			s.setDash(dash(outerLength), 0)
		} else {
			s.setDash(nil, 0)
		}
		if style == "dotted" {
			// Port: okage can drop the dash of zero length at the very end
			// of a line, so the line ends a little after its last dot.
			x2 += (x2 - x1) * 1e-6
			y2 += (y2 - y1) * 1e-6
		}
		s.moveTo(x1, y1)
		s.lineTo(x2, y2)
		s.stroke()
	}
}

// drawLine is draw_line in weasyprint/draw/border.py: it draws a vertical or
// a horizontal line. color holds one color, or two colors for ridge and
// groove.
func drawLine(s *stream, x1, y1, x2, y2, thickness float64, style string, color []*tinycss2.Color, offset float64) {
	// Port: upstream asserts that the line is vertical or horizontal.
	if (x1 != x2 && y1 != y2) || len(color) == 0 {
		return
	}

	// Port: a dashed, a dotted or a wavy line of more than maxBorderDashes
	// periods is drawn solid.
	lineLength := math.Abs(x2-x1) + math.Abs(y2-y1)
	if (style == "dashed" || style == "dotted" || style == "wavy") && !(lineLength/thickness <= maxBorderDashes) {
		style = "solid"
	}

	s.stacked(func() {
		if style != "ridge" && style != "groove" {
			s.setColor(color[0], true)
		}

		switch style {
		case "dashed":
			s.setDash([]float64{5 * thickness}, offset)
		case "dotted":
			s.setLineCap(1)
			s.setDash([]float64{0, 2 * thickness}, offset)
		}

		switch style {
		case "double":
			s.setLineWidth(thickness / 3)
			if x1 == x2 {
				s.moveTo(x1-thickness/3, y1)
				s.lineTo(x2-thickness/3, y2)
				s.moveTo(x1+thickness/3, y1)
				s.lineTo(x2+thickness/3, y2)
			} else {
				s.moveTo(x1, y1-thickness/3)
				s.lineTo(x2, y2-thickness/3)
				s.moveTo(x1, y1+thickness/3)
				s.lineTo(x2, y2+thickness/3)
			}
		case "ridge", "groove":
			s.setLineWidth(thickness / 2)
			s.setColor(color[0], true)
			if x1 == x2 {
				s.moveTo(x1+thickness/4, y1)
				s.lineTo(x2+thickness/4, y2)
			} else {
				s.moveTo(x1, y1+thickness/4)
				s.lineTo(x2, y2+thickness/4)
			}
			s.stroke()
			s.setColor(color[len(color)-1], true)
			if x1 == x2 {
				s.moveTo(x1-thickness/4, y1)
				s.lineTo(x2-thickness/4, y2)
			} else {
				s.moveTo(x1, y1-thickness/4)
				s.lineTo(x2, y2-thickness/4)
			}
		case "wavy":
			// Port: upstream asserts that the line is horizontal, which
			// only the text decoration draws.
			if y1 != y2 {
				return
			}
			up := 1.0
			radius := 0.75 * thickness

			s.rectangle(x1, y1-2*radius, x2-x1, 4*radius)
			s.clip(false)
			s.end()

			x := x1 - offset
			s.moveTo(x, y1)
			for x < x2 {
				s.setLineWidth(thickness)
				s.curveTo(x+radius/2, y1+up*radius, x+3*radius/2, y1+up*radius, x+2*radius, y1)
				x += 2 * radius
				up *= -1
			}
		default:
			s.setLineWidth(thickness)
			s.moveTo(x1, y1)
			s.lineTo(x2, y2)
		}
		s.stroke()
	})
}

// drawOutline is draw_outline in weasyprint/draw/border.py: it draws the
// outlines of the box and of its descendants. inline holds the stacking
// contexts that stand in the children, which upstream skips because they
// are not boxes.
func drawOutline(s *stream, box *Box, inline map[*Box]*StackingContext) {
	width := pixels(box.Style.Get("outline_width"))
	offset := pixels(box.Style.Get("outline_offset"))
	color := getColor(box.Style, "outline_color")
	style := str(box.Style.Get("outline_style"))
	if isStr(box.Style.Get("visibility"), "visible") && width != 0 && color != nil && color.Alpha != 0 {
		outlineBox := [4]float64{
			box.BorderBoxX() - width - offset,
			box.BorderBoxY() - width - offset,
			box.BorderWidth() + 2*width + 2*offset,
			box.BorderHeight() + 2*width + 2*offset,
		}
		for _, side := range sides {
			s.stacked(func() {
				clipBorderSegment(s, style, width, side, outlineBox, nil, nil)
				drawRectBorder(s, outlineBox, [4]float64{width, width, width, width}, style,
					styledColor(style, color, side))
			})
		}
	}

	for _, child := range box.Children {
		if s.dc.stopped() {
			return
		}
		// A stacking context in the children draws its own outlines.
		if _, ok := inline[child]; ok {
			continue
		}
		drawOutline(s, child, inline)
	}
}

// roundedBox is rounded_box in weasyprint/draw/border.py: it adds the path
// of a rounded box to the stream.
func roundedBox(s *stream, radii RoundedBox) {
	x, y, w, h := radii.X, radii.Y, radii.Width, radii.Height
	tl, tr, br, bl := radii.TopLeft, radii.TopRight, radii.BottomRight, radii.BottomLeft

	if (tl[0] == 0 || tl[1] == 0) && (tr[0] == 0 || tr[1] == 0) &&
		(br[0] == 0 || br[1] == 0) && (bl[0] == 0 || bl[1] == 0) {
		// No radius, draw a rectangle.
		s.rectangle(x, y, w, h)
		return
	}

	r := 0.45

	s.moveTo(x+tl[0], y)
	s.lineTo(x+w-tr[0], y)
	s.curveTo(x+w-tr[0]*r, y, x+w, y+tr[1]*r, x+w, y+tr[1])
	s.lineTo(x+w, y+h-br[1])
	s.curveTo(x+w, y+h-br[1]*r, x+w-br[0]*r, y+h, x+w-br[0], y+h)
	s.lineTo(x+bl[0], y+h)
	s.curveTo(x+bl[0]*r, y+h, x, y+h-bl[1]*r, x, y+h-bl[1])
	s.lineTo(x, y+tl[1])
	s.curveTo(x, y+tl[1]*r, x+tl[0]*r, y, x+tl[0], y)
}
