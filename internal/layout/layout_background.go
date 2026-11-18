// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/background.py: it manages the position
// and the size of the backgrounds.

package layout

import (
	"math"
	"strings"

	"github.com/xo/okage/tinycss2"
)

// Background is the namedtuple Background of upstream: the layout of the
// background of a box, for the drawing.
type Background struct {
	// Color is the background color. It is never nil.
	Color *tinycss2.Color
	// Layers are the layers of the background images, from the first one
	// of background-image.
	Layers []BackgroundLayer
	// Style is the style that gives the background.
	Style *Style
}

// BackgroundLayer is the namedtuple BackgroundLayer of upstream: a layer
// of a background image.
type BackgroundLayer struct {
	// Image is the image of the layer, or nil for a layer without an image.
	// The other fields of a layer without an image are the 'unused' of
	// upstream, except PaintingArea and ClippedBoxes.
	Image Image
	// Size is the size of the image: (width, height).
	Size [2]float64
	// Position is the position of the image in PositioningArea: (x, y).
	Position [2]float64
	// Repeat is background-repeat: (repeat-x, repeat-y), such as
	// ("repeat", "no-repeat").
	Repeat [2]string
	// Unbounded is always false.
	Unbounded bool
	// PaintingArea is the area that the layer paints: (x, y, width,
	// height).
	PaintingArea [4]float64
	// PositioningArea is the area in which the image is positioned: (x,
	// y, width, height).
	PositioningArea [4]float64
	// ClippedBoxes are the rounded boxes that clip the layer.
	ClippedBoxes []RoundedBox
}

// boxRectangle is box_rectangle in weasyprint/layout/background.py: the
// rectangle (x, y, width, height) of the border box, the padding box or the
// content box of box.
func boxRectangle(box *Box, whichRectangle string) [4]float64 {
	switch whichRectangle {
	case "border-box":
		return [4]float64{box.BorderBoxX(), box.BorderBoxY(), box.BorderWidth(), box.BorderHeight()}
	case "padding-box":
		return [4]float64{box.PaddingBoxX(), box.PaddingBoxY(), box.PaddingWidth(), box.PaddingHeight()}
	}
	// Port: upstream asserts that the rectangle is the content box.
	return [4]float64{box.ContentBoxX(), box.ContentBoxY(), box.Width, box.Height}
}

// imageOf returns the image of a value of background-image,
// border-image-source or mask-border-source: (type, value), where the value
// is a URL or a gradient. It returns nil for none.
func imageOf(value Value, getImageFromURI func(url string, orientation Value) Image, orientation Value) Image {
	pair, ok := value.(Tuple)
	if !ok || len(pair) != 2 {
		return nil
	}
	if isStr(pair[0], "url") {
		return getImageFromURI(str(pair[1]), orientation)
	}
	if image, ok := pair[1].(Image); ok {
		return image
	}
	return nil
}

// layoutBoxBackgrounds is layout_box_backgrounds in
// weasyprint/layout/background.py. It fetches and positions the background
// images of box and, with layoutChildren, of its descendants. style is the
// style of the background, or nil for the style of box.
func layoutBoxBackgrounds(page, box *Box, getImageFromURI func(url string, orientation Value) Image, layoutChildren bool, style *Style) {
	// Resolve percentages in border-radius properties
	resolveRadiiPercentages(box)

	if layoutChildren {
		for _, child := range box.AllChildren() {
			layoutBoxBackgrounds(page, child, getImageFromURI, true, nil)
		}
	}

	if style == nil {
		style = box.Style
	}

	// This is for the border image, not the background, but this is a
	// convenient place to get the image.
	if source, ok := style.Get("border_image_source").(Tuple); ok && len(source) == 2 && !isStr(source[0], "none") {
		box.ownExtra().BorderImage = imageOf(source, getImageFromURI, nil)
	}

	if source, ok := style.Get("mask_border_source").(Tuple); ok && len(source) == 2 && !isStr(source[0], "none") {
		box.ownExtra().MaskBorderImage = imageOf(source, getImageFromURI, nil)
	}

	var images []Image
	var color *tinycss2.Color
	if isStr(style.Get("visibility"), "hidden") {
		color = transparent
	} else {
		orientation := style.Get("image_orientation")
		values, _ := style.Get("background_image").(Tuple)
		images = make([]Image, len(values))
		for i, value := range values {
			images[i] = imageOf(value, getImageFromURI, orientation)
		}
		color = getColor(style, "background_color")
		if color == nil {
			// Port: a background color that is not a color is
			// transparent. Upstream fails on it.
			color = transparent
		}
	}

	anyImage := false
	for _, image := range images {
		if image != nil {
			anyImage = true
			break
		}
	}
	if color.Alpha == 0 && !anyImage && !truthy(style.Get("box_shadow")) {
		if box != page { // Pages need a background for bleed box
			box.Background = nil
			return
		}
	}

	sizes, _ := style.Get("background_size").(Tuple)
	clips, _ := style.Get("background_clip").(Tuple)
	repeats, _ := style.Get("background_repeat").(Tuple)
	origins, _ := style.Get("background_origin").(Tuple)
	positions, _ := style.Get("background_position").(Tuple)
	attachments, _ := style.Get("background_attachment").(Tuple)
	resolution := num(style.Get("image_resolution"))
	var layers []BackgroundLayer
	// Port: upstream zips the images with the cycles of the other lists, so
	// an empty list gives no layer.
	if len(sizes) > 0 && len(clips) > 0 && len(repeats) > 0 && len(origins) > 0 && len(positions) > 0 && len(attachments) > 0 {
		layers = make([]BackgroundLayer, len(images))
		for i, image := range images {
			layers[i] = layoutBackgroundLayer(box, page, resolution, image,
				sizes[i%len(sizes)], str(clips[i%len(clips)]), repeats[i%len(repeats)],
				str(origins[i%len(origins)]), positions[i%len(positions)],
				str(attachments[i%len(attachments)]))
		}
	}
	box.Background = &Background{Color: color, Layers: layers, Style: style}
}

// layoutBackgroundLayer is layout_background_layer in
// weasyprint/layout/background.py.
func layoutBackgroundLayer(box, page *Box, resolution float64, image Image, size Value, clip string, repeat Value, origin string, position Value, attachment string) BackgroundLayer {
	// TODO: respect box-sizing for table cells?
	var clippedBoxes []RoundedBox
	var paintingArea [4]float64
	switch {
	case box == page:
		// [The page’s] background painting area is the bleed area […]
		// regardless of background-clip.
		// https://drafts.csswg.org/css-page-3/#painting
		paintingArea = page.BleedArea()
	case box.Kind == KindTableRowGroup:
		totalHeight := 0.0
		for _, row := range box.Children {
			for _, cell := range row.Children {
				clippedBoxes = append(clippedBoxes, cell.RoundedBorderBox())
				totalHeight = max(totalHeight, cell.BorderHeight())
			}
		}
		paintingArea = [4]float64{box.BorderBoxX(), box.BorderBoxY(), box.BorderWidth(), totalHeight}
	case box.Kind == KindTableRow:
		if len(box.Children) > 0 {
			height := math.Inf(-1)
			for _, cell := range box.Children {
				clippedBoxes = append(clippedBoxes, cell.RoundedBorderBox())
				height = max(height, cell.BorderHeight())
			}
			paintingArea = [4]float64{box.BorderBoxX(), box.BorderBoxY(), box.BorderWidth(), height}
		}
	case box.Kind == KindTableColumnGroup || box.Kind == KindTableColumn:
		var cells []*Box
		if box.Kind == KindTableColumn {
			cells = box.ColumnCells()
		} else {
			cells = box.GetCells(func(column *Box) []*Box { return column.ColumnCells() })
		}
		if len(cells) > 0 {
			minX, maxX := math.Inf(1), math.Inf(-1)
			for _, cell := range cells {
				clippedBoxes = append(clippedBoxes, cell.RoundedBorderBox())
				minX = min(minX, cell.BorderBoxX())
				maxX = max(maxX, cell.BorderBoxX()+cell.BorderWidth())
			}
			paintingArea = [4]float64{minX, box.BorderBoxY(), maxX - minX, box.BorderHeight()}
		}
	default:
		paintingArea = boxRectangle(box, clip)
		switch clip {
		case "border-box":
			clippedBoxes = []RoundedBox{box.RoundedBorderBox()}
		case "padding-box":
			clippedBoxes = []RoundedBox{box.RoundedPaddingBox()}
		default:
			// Port: upstream asserts that the clip is the content box.
			clippedBoxes = []RoundedBox{box.RoundedContentBox()}
		}
	}

	var intrinsic IntrinsicSize
	if image != nil {
		intrinsic = image.GetIntrinsicSize(resolution, num(box.Style.Get("font_size")))
	}
	if image == nil || (intrinsic.HasWidth && intrinsic.Width == 0) || (intrinsic.HasHeight && intrinsic.Height == 0) {
		return BackgroundLayer{PaintingArea: paintingArea, ClippedBoxes: clippedBoxes}
	}

	var positioningArea [4]float64
	switch {
	case attachment == "fixed" && box.Kind == KindPage:
		// Initial containing block
		// […] if background-attachment is fixed then the image is
		// positioned relative to the page box including its margins […].
		// https://drafts.csswg.org/css-page/#painting
		positioningArea = [4]float64{0, 0, box.MarginWidth(), box.MarginHeight()}
	case attachment == "fixed":
		positioningArea = boxRectangle(page, "content-box")
	default:
		positioningArea = boxRectangle(box, origin)
	}

	positioningWidth, positioningHeight := positioningArea[2], positioningArea[3]

	ratio := optFloat(intrinsic.Ratio, intrinsic.HasRatio)
	var imageWidth, imageHeight float64
	sizePair, _ := size.(Tuple)
	switch {
	case isStr(size, "cover"):
		imageWidth, imageHeight = coverConstraintImageSizing(positioningWidth, positioningHeight, ratio)
	case isStr(size, "contain"):
		imageWidth, imageHeight = containConstraintImageSizing(positioningWidth, positioningHeight, ratio)
	default:
		var sizeWidth, sizeHeight Value
		if len(sizePair) == 2 {
			sizeWidth, sizeHeight = sizePair[0], sizePair[1]
		}
		imageWidth, imageHeight = defaultImageSizing(
			optFloat(intrinsic.Width, intrinsic.HasWidth),
			optFloat(intrinsic.Height, intrinsic.HasHeight), ratio,
			specifiedSize(percentage(sizeWidth, box.Style, positioningWidth)),
			specifiedSize(percentage(sizeHeight, box.Style, positioningHeight)),
			positioningWidth, positioningHeight)
	}

	var originX, originY string
	var positionXValue, positionYValue Value
	if p, ok := position.(Tuple); ok && len(p) == 4 {
		originX, positionXValue, originY, positionYValue = str(p[0]), p[1], str(p[2]), p[3]
	}
	refX := positioningWidth - imageWidth
	refY := positioningHeight - imageHeight
	// Port: a position that cannot be resolved is 0. Upstream fails on it.
	positionX, _ := percentageFloat(positionXValue, box.Style, refX)
	positionY, _ := percentageFloat(positionYValue, box.Style, refY)
	if originX == "right" {
		positionX = refX - positionX
	}
	if originY == "bottom" {
		positionY = refY - positionY
	}

	var repeatX, repeatY string
	if r, ok := repeat.(Tuple); ok && len(r) == 2 {
		repeatX, repeatY = str(r[0]), str(r[1])
	}

	sizeAuto := func(i int) bool {
		return len(sizePair) == 2 && isStr(sizePair[i], "auto")
	}
	// Port: a size of 0 is not rounded. Upstream divides by zero.
	if repeatX == "round" && imageWidth != 0 {
		nRepeats := max(1, math.RoundToEven(positioningWidth/imageWidth))
		newWidth := positioningWidth / nRepeats
		positionX = 0 // Ignore background-position for this dimension
		if repeatY != "round" && sizeAuto(1) {
			imageHeight *= newWidth / imageWidth
		}
		imageWidth = newWidth
	}
	if repeatY == "round" && imageHeight != 0 {
		nRepeats := max(1, math.RoundToEven(positioningHeight/imageHeight))
		newHeight := positioningHeight / nRepeats
		positionY = 0 // Ignore background-position for this dimension
		if repeatX != "round" && sizeAuto(0) {
			imageWidth *= newHeight / imageHeight
		}
		imageHeight = newHeight
	}

	return BackgroundLayer{
		Image:           image,
		Size:            [2]float64{imageWidth, imageHeight},
		Position:        [2]float64{positionX, positionY},
		Repeat:          [2]string{repeatX, repeatY},
		Unbounded:       false,
		PaintingArea:    paintingArea,
		PositioningArea: positioningArea,
		ClippedBoxes:    clippedBoxes,
	}
}

// specifiedSize converts the result of percentage into the specified size
// of defaultImageSizing: nil for None and for auto.
func specifiedSize(v Value) *float64 {
	f, ok := v.(Float)
	return optFloat(float64(f), ok)
}

// layoutBackgrounds is layout_backgrounds in
// weasyprint/layout/background.py. It lays out the backgrounds of the page
// box and of its children.
//
// This function takes care of the canvas background, taken from the root
// elememt or a <body> child of the root element.
//
// See https://www.w3.org/TR/CSS21/colors.html#background
func layoutBackgrounds(page *Box, getImageFromURI func(url string, orientation Value) Image) {
	layoutBoxBackgrounds(page, page, getImageFromURI, true, nil)
	if len(page.Children) == 0 || page.Children[0].Kind == KindMargin {
		// Port: upstream asserts that the first child of the page is the
		// box of the root element.
		page.ownExtra().CanvasBackground = nil
		return
	}
	rootBox := page.Children[0]
	chosenBox := rootBox
	if strings.ToLower(rootBox.ElementTag) == "html" && rootBox.Background == nil {
		for _, child := range rootBox.Children {
			if strings.ToLower(child.ElementTag) == "body" {
				chosenBox = child
				break
			}
		}
	}

	if chosenBox.Background == nil {
		page.ownExtra().CanvasBackground = nil
		return
	}
	paintingArea := boxRectangle(page, "border-box")
	originalBackground := page.Background
	layoutBoxBackgrounds(page, page, getImageFromURI, false, chosenBox.Style)
	canvas := *page.Background
	// TODO: background-clip should be updated
	canvas.Layers = make([]BackgroundLayer, len(page.Background.Layers))
	for i, layer := range page.Background.Layers {
		layer.PaintingArea = paintingArea
		canvas.Layers[i] = layer
	}
	page.ownExtra().CanvasBackground = &canvas
	page.Background = originalBackground
	chosenBox.Background = nil
}
