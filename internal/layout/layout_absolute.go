// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/absolute.py: the layout of the
// absolutely positioned boxes.

package layout

import "math"

// placeholderState is the state of the placeholder of an absolute box.
type placeholderState uint8

const (
	// notPlaceholder is a box that is not a placeholder.
	notPlaceholder placeholderState = iota
	// placeholderPending is a placeholder whose box is not laid out yet.
	placeholderPending
	// placeholderDone is a placeholder whose box is laid out.
	placeholderDone
)

// newAbsolutePlaceholder is AbsolutePlaceholder(box) in
// weasyprint/layout/absolute.py: the box left where an absolutely
// positioned box was taken out of the flow.
//
// Port: upstream wraps the box in an object that pretends to be the box,
// and that the layout replaces by the laid out box. The placeholder of the
// port is a copy of the box, with a state. setLaidOutBox copies the laid out
// box into it, so that each parent that holds the placeholder sees the laid
// out box. IsPlaceholder is isinstance(box, AbsolutePlaceholder).
func newAbsolutePlaceholder(box *Box) *Box {
	p := box.Copy()
	p.placeholder = placeholderPending
	return p
}

// IsPlaceholder reports whether the box is the placeholder of an absolutely
// positioned box, an AbsolutePlaceholder of upstream.
func (b *Box) IsPlaceholder() bool {
	return b.placeholder != notPlaceholder
}

// placeholderBox returns the box of a placeholder, the attribute _box of
// upstream: a copy of the placeholder that is not a placeholder.
func (b *Box) placeholderBox() *Box {
	c := b.Copy()
	c.placeholder = notPlaceholder
	return c
}

// setLaidOutBox is AbsolutePlaceholder.set_laid_out_box: the placeholder
// becomes the laid out box.
func (b *Box) setLaidOutBox(newBox *Box) {
	*b = *newBox
	b.placeholder = placeholderDone
}

// absoluteTranslation is the result of absolute_width and absolute_height:
// whether the box moves back by its laid out size, and the translation.
type absoluteTranslation struct {
	translateBoxSize bool
	translate        float64
}

// parentIsLTR reports whether the parent style of the style of box is
// missing or has the direction ltr, as absolute_width and
// absolute_replaced compute ltr.
func parentIsLTR(box *Box) bool {
	parent := box.Style.ParentStyle
	return parent == nil || isStr(parent.Get("direction"), "ltr")
}

// absoluteWidth is absolute_width in weasyprint/layout/absolute.py, with
// the decorator handle_min_max_width.
//
// Port: upstream also takes cb_y and cb_height, which it does not read.
func absoluteWidth(box *Box, lc *layoutContext, cbX, cbWidth float64) absoluteTranslation {
	return handleMinMaxWidth(box, func() absoluteTranslation {
		return absoluteWidthWithoutMinMax(box, lc, cbX, cbWidth)
	})
}

// absoluteWidthWithoutMinMax is absolute_width in
// weasyprint/layout/absolute.py, without its decorator.
func absoluteWidthWithoutMinMax(box *Box, lc *layoutContext, cbX, cbWidth float64) absoluteTranslation {
	// https://www.w3.org/TR/CSS2/visudet.html#abs-replaced-width
	ltr := parentIsLTR(box)
	paddingsBorders := box.PaddingLeft + box.PaddingRight + box.BorderLeftWidth + box.BorderRightWidth
	translateX := 0.0
	translateBoxWidth := false
	defaultTranslateX := cbX - box.PositionX

	switch {
	case box.Left == Auto && box.Right == Auto && box.Width == Auto:
		if box.MarginLeft == Auto {
			box.MarginLeft = 0
		}
		if box.MarginRight == Auto {
			box.MarginRight = 0
		}
		availableWidth := cbWidth - (paddingsBorders + box.MarginLeft + box.MarginRight)
		box.Width = shrinkToFit(lc, box, availableWidth)
		if box.IsOutsideMarker {
			translateBoxWidth = ltr
		} else if !ltr {
			translateBoxWidth = true
			translateX = defaultTranslateX + availableWidth
		}
	case box.Left != Auto && box.Right != Auto && box.Width != Auto:
		widthForMargins := cbWidth - (box.Right + box.Left + box.Width + paddingsBorders)
		switch {
		case box.MarginLeft == Auto && box.MarginRight == Auto:
			if box.Width+paddingsBorders+box.Right+box.Left <= cbWidth {
				box.MarginLeft = widthForMargins / 2
				box.MarginRight = widthForMargins / 2
			} else {
				if ltr {
					box.MarginLeft, box.MarginRight = 0, widthForMargins
				} else {
					box.MarginLeft, box.MarginRight = widthForMargins, 0
				}
			}
		case box.MarginLeft == Auto:
			box.MarginLeft = widthForMargins
		case box.MarginRight == Auto:
			box.MarginRight = widthForMargins
		case ltr:
			box.MarginRight = widthForMargins
		default:
			box.MarginLeft = widthForMargins
		}
		translateX = box.Left + defaultTranslateX
	default:
		if box.MarginLeft == Auto {
			box.MarginLeft = 0
		}
		if box.MarginRight == Auto {
			box.MarginRight = 0
		}
		spacing := paddingsBorders + box.MarginLeft + box.MarginRight
		switch {
		case box.Left == Auto && box.Width == Auto:
			box.Width = shrinkToFit(lc, box, cbWidth-spacing-box.Right)
			translateX = cbWidth - box.Right - spacing + defaultTranslateX
			translateBoxWidth = true
		case box.Left == Auto && box.Right == Auto:
			if !ltr {
				availableWidth := cbWidth - (paddingsBorders + box.MarginLeft + box.MarginRight)
				translateBoxWidth = true
				translateX = defaultTranslateX + availableWidth
			}
		case box.Width == Auto && box.Right == Auto:
			box.Width = shrinkToFit(lc, box, cbWidth-spacing-box.Left)
			translateX = box.Left + defaultTranslateX
		case box.Left == Auto:
			translateX = cbWidth + defaultTranslateX - (box.Right + spacing + box.Width)
		case box.Width == Auto:
			box.Width = cbWidth - box.Right - box.Left - spacing
			translateX = box.Left + defaultTranslateX
		case box.Right == Auto:
			translateX = box.Left + defaultTranslateX
		}
	}

	return absoluteTranslation{translateBoxWidth, translateX}
}

// absoluteHeight is absolute_height in weasyprint/layout/absolute.py.
//
// Port: upstream also takes the context, cb_x and cb_width, which it does
// not read.
func absoluteHeight(box *Box, cbY, cbHeight float64) absoluteTranslation {
	// https://www.w3.org/TR/CSS2/visudet.html#abs-non-replaced-height
	paddingsBorders := box.PaddingTop + box.PaddingBottom + box.BorderTopWidth + box.BorderBottomWidth
	translateY := 0.0
	translateBoxHeight := false
	defaultTranslateY := cbY - box.PositionY

	switch {
	case box.Top == Auto && box.Bottom == Auto && box.Height == Auto:
		// Keep the static position
		if box.MarginTop == Auto {
			box.MarginTop = 0
		}
		if box.MarginBottom == Auto {
			box.MarginBottom = 0
		}
	case box.Top != Auto && box.Bottom != Auto && box.Height != Auto:
		heightForMargins := cbHeight - (box.Top + box.Bottom + box.Height + paddingsBorders)
		switch {
		case box.MarginTop == Auto && box.MarginBottom == Auto:
			box.MarginTop = heightForMargins / 2
			box.MarginBottom = heightForMargins / 2
		case box.MarginTop == Auto:
			box.MarginTop = heightForMargins
		default:
			box.MarginBottom = heightForMargins
		}
		translateY = box.Top + defaultTranslateY
	default:
		if box.MarginTop == Auto {
			box.MarginTop = 0
		}
		if box.MarginBottom == Auto {
			box.MarginBottom = 0
		}
		spacing := paddingsBorders + box.MarginTop + box.MarginBottom
		switch {
		case box.Top == Auto && box.Height == Auto:
			translateY = cbHeight - box.Bottom - spacing + defaultTranslateY
			translateBoxHeight = true
		case box.Top == Auto && box.Bottom == Auto:
			// Keep the static position
		case box.Height == Auto && box.Bottom == Auto:
			translateY = box.Top + defaultTranslateY
		case box.Top == Auto:
			translateY = cbHeight + defaultTranslateY - (box.Bottom + spacing + box.Height)
		case box.Height == Auto:
			box.Height = cbHeight - box.Bottom - box.Top - spacing
			translateY = box.Top + defaultTranslateY
		case box.Bottom == Auto:
			translateY = box.Top + defaultTranslateY
		}
	}

	return absoluteTranslation{translateBoxHeight, translateY}
}

// absoluteBlock is absolute_block in weasyprint/layout/absolute.py.
func absoluteBlock(lc *layoutContext, box, containingBlock *Box, fixedBoxes *[]*Box, bottomSpace float64, skip skipStack, cbX, cbY, cbWidth, cbHeight float64) (*Box, skipStack) {
	width := absoluteWidth(box, lc, cbX, cbWidth)
	var height absoluteTranslation
	if len(skip) == 0 {
		height = absoluteHeight(box, cbY, cbHeight)
	}

	if height.translateBoxSize {
		bottomSpace += -box.PositionY
	} else {
		bottomSpace += height.translate
	}

	// This box is the containing block for absolute descendants.
	var absoluteBoxes []*Box

	if box.IsTableWrapper {
		tableWrapperWidth(lc, box, sizeBlock(cbWidth, cbHeight))
	}

	var result blockLayoutResult
	switch {
	case box.Is(KindBlock):
		result = blockContainerLayout(lc, box, bottomSpace, skip, true,
			&absoluteBoxes, fixedBoxes, nil, nil, nil, false, noMaxLines)
	case box.IsFlexContainer():
		result = flexLayout(lc, box, bottomSpace, skip, containingBlock, true,
			&absoluteBoxes, fixedBoxes, false)
	case box.IsGridContainer():
		result = gridLayout(lc, box, bottomSpace, skip, containingBlock, true,
			&absoluteBoxes, fixedBoxes)
	case box.IsBlockContainer():
		// Port: upstream leaves new_box unbound for another kind of box
		// and fails. The port lays out the other block containers as
		// blocks.
		result = blockContainerLayout(lc, box, bottomSpace, skip, true,
			&absoluteBoxes, fixedBoxes, nil, nil, nil, false, noMaxLines)
	}
	newBox, resumeAt := result.box, result.resumeAt
	if newBox == nil {
		// Port: upstream fails on a box that it cannot lay out. The port
		// keeps the box, without children and without size.
		newBox = emptyLayoutBox(box)
		resumeAt = nil
	}

	for _, childPlaceholder := range absoluteBoxes {
		absoluteLayout(lc, childPlaceholder, newBox, fixedBoxes, bottomSpace, nil)
	}

	translateX, translateY := width.translate, height.translate
	if width.translateBoxSize {
		translateX -= newBox.Width
	}
	if height.translateBoxSize {
		translateY -= newBox.Height
	}
	newBox.Translate(translateX, translateY, false)

	return newBox, resumeAt
}

// emptyLayoutBox returns a copy of box without children, whose auto sizes
// and margins are 0. It is the box that the port keeps where upstream fails
// to lay out a box.
func emptyLayoutBox(box *Box) *Box {
	newBox := box.CopyWithChildren(nil)
	for _, field := range []*float64{
		&newBox.Width, &newBox.Height, &newBox.MarginTop, &newBox.MarginRight,
		&newBox.MarginBottom, &newBox.MarginLeft,
	} {
		if *field == Auto {
			*field = 0
		}
	}
	return newBox
}

// absoluteLayout is absolute_layout in weasyprint/layout/absolute.py: it
// sets the width of the absolute positioned box of placeholder, lays it
// out, and sets the laid out box in the placeholder.
//
// Port: upstream asserts that the placeholder is not laid out yet. The port
// lays out its box again.
func absoluteLayout(lc *layoutContext, placeholder, containingBlock *Box, fixedBoxes *[]*Box, bottomSpace float64, skip skipStack) {
	box := placeholder.placeholderBox()
	newBox, resumeAt := absoluteBoxLayout(lc, box, containingBlock, fixedBoxes, bottomSpace, skip)
	placeholder.setLaidOutBox(newBox)
	if len(resumeAt) > 0 {
		lc.addBrokenOutOfFlow(placeholder, box, containingBlock, resumeAt)
	}
}

// absoluteBoxLayout is absolute_box_layout in
// weasyprint/layout/absolute.py: it lays out an absolutely positioned box,
// and returns the laid out box and where it resumes.
func absoluteBoxLayout(lc *layoutContext, box, containingBlock *Box, fixedBoxes *[]*Box, bottomSpace float64, skip skipStack) (*Box, skipStack) {
	lc.enter()
	defer lc.leave()

	// TODO: handle inline boxes (point 10.1.4.1)
	// https://www.w3.org/TR/CSS2/visudet.html#containing-block-details
	var cbX, cbY, cbWidth, cbHeight float64
	if containingBlock.Kind == KindPage || !containingBlock.isBox() {
		cbX = containingBlock.ContentBoxX()
		cbY = containingBlock.ContentBoxY()
		cbWidth = containingBlock.Width
		cbHeight = containingBlock.Height
	} else {
		cbX = containingBlock.PaddingBoxX()
		cbY = containingBlock.PaddingBoxY()
		cbWidth = containingBlock.PaddingWidth()
		cbHeight = containingBlock.PaddingHeight()
	}

	resolvePercentages(box, sizeBlock(cbWidth, cbHeight))
	resolvePositionPercentages(box, sizeBlock(cbWidth, cbHeight))

	if box.Kind == KindBlockReplaced {
		return absoluteReplaced(box, cbX, cbY, cbWidth, cbHeight), nil
	}
	// Absolute tables are wrapped into block boxes
	return absoluteBlock(lc, box, containingBlock, fixedBoxes, bottomSpace, skip, cbX, cbY, cbWidth, cbHeight)
}

// absoluteReplaced is absolute_replaced in weasyprint/layout/absolute.py.
//
// Port: upstream gives the tuple (cb_x, cb_y, cb_width, cb_height) as the
// containing block of inline_replaced_box_width_height, whose
// block_level_width reads cb_x as the width. The port gives the width and
// the height.
func absoluteReplaced(box *Box, cbX, cbY, cbWidth, cbHeight float64) *Box {
	inlineReplacedBoxWidthHeight(box, sizeBlock(cbWidth, cbHeight))
	ltr := parentIsLTR(box)

	// https://www.w3.org/TR/CSS21/visudet.html#abs-replaced-width
	if box.Left == Auto && box.Right == Auto {
		// static position:
		if ltr {
			box.Left = box.PositionX - cbX
		} else {
			box.Right = cbX + cbWidth - box.PositionX
		}
	}
	switch {
	case box.Left == Auto || box.Right == Auto:
		if box.MarginLeft == Auto {
			box.MarginLeft = 0
		}
		if box.MarginRight == Auto {
			box.MarginRight = 0
		}
		remaining := cbWidth - box.MarginWidth()
		if box.Left == Auto {
			box.Left = remaining - box.Right
		}
		if box.Right == Auto {
			box.Right = remaining - box.Left
		}
	case box.MarginLeft == Auto || box.MarginRight == Auto:
		remaining := cbWidth - (box.BorderWidth() + box.Left + box.Right)
		switch {
		case box.MarginLeft == Auto && box.MarginRight == Auto:
			switch {
			case remaining >= 0:
				box.MarginLeft = math.Floor(remaining / 2)
				box.MarginRight = box.MarginLeft
			case ltr:
				box.MarginLeft, box.MarginRight = 0, remaining
			default:
				box.MarginLeft, box.MarginRight = remaining, 0
			}
		case box.MarginLeft == Auto:
			box.MarginLeft = remaining
		default:
			box.MarginRight = remaining
		}
	default:
		// Over-constrained
		if ltr {
			box.Right = cbWidth - (box.MarginWidth() + box.Left)
		} else {
			box.Left = cbWidth - (box.MarginWidth() + box.Right)
		}
	}

	// https://www.w3.org/TR/CSS21/visudet.html#abs-replaced-height
	if box.Top == Auto && box.Bottom == Auto {
		box.Top = box.PositionY - cbY
	}
	switch {
	case box.Top == Auto || box.Bottom == Auto:
		if box.MarginTop == Auto {
			box.MarginTop = 0
		}
		if box.MarginBottom == Auto {
			box.MarginBottom = 0
		}
		remaining := cbHeight - box.MarginHeight()
		if box.Top == Auto {
			box.Top = remaining - box.Bottom
		}
		if box.Bottom == Auto {
			box.Bottom = remaining - box.Top
		}
	case box.MarginTop == Auto || box.MarginBottom == Auto:
		remaining := cbHeight - (box.BorderHeight() + box.Top + box.Bottom)
		switch {
		case box.MarginTop == Auto && box.MarginBottom == Auto:
			box.MarginTop = math.Floor(remaining / 2)
			box.MarginBottom = box.MarginTop
		case box.MarginTop == Auto:
			box.MarginTop = remaining
		default:
			box.MarginBottom = remaining
		}
	default:
		// Over-constrained
		box.Bottom = cbHeight - (box.MarginHeight() + box.Top)
	}

	// No children for replaced boxes, no need to .translate()
	box.PositionX = cbX + box.Left
	box.PositionY = cbY + box.Top
	return box
}
