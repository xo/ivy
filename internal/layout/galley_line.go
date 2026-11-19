// This file is not a port. It lays out a line at a height of 0 and then
// moves it to its height, so that the lines of the galley and the lines of
// the pages are the same (D57).

package layout

// lineState is the state of the layout of the lines that the port adds to
// the layout context (D57).
type lineState struct {
	// outOfFlow caches, for each line box of the boxes, whether one of its
	// descendants is out of the normal flow.
	outOfFlow map[*Box]bool
	// index holds the index of each box of the document in tree order,
	// for the keys of the lines of the galley, or nil.
	index map[*Box]int
	// galley is the galley of the pages, or nil. See galleyReader.
	galley *galleyReader
	// recorder keeps the lines of a part of the galley, or nil. See
	// galleyRecorder.
	recorder *galleyRecorder
	// taken counts the lines that the pages took from the galley, for the
	// tests.
	taken int
}

// layoutLine is the call of get_next_linebox in iter_line_boxes. It takes
// the line from the galley when it can, and else lays it out with
// getNextLinebox.
//
// Port: upstream lays out each line at its height. The port lays out a line
// at a height of 0 and then moves it to its height, when no float is next
// to the line and when the line holds no box out of the normal flow. Two
// layouts of the same line at two heights then give the same line, moved.
// The move changes the positions by about 1e-13 pixels (D57).
func layoutLine(lc *layoutContext, box *Box, positionY, bottomSpace float64, skip skipStack, containingBlock *Box, absoluteBoxes, fixedBoxes *[]*Box, firstLetterStyle, firstLineStyle *Style) (*Box, skipStack) {
	if !lc.lineAtZero(box, positionY, firstLineStyle) {
		return getNextLinebox(lc, box, positionY, bottomSpace, skip, containingBlock,
			absoluteBoxes, fixedBoxes, firstLetterStyle, firstLineStyle)
	}
	plain := firstLetterStyle == nil && firstLineStyle == nil
	var key galleyKey
	if plain && (lc.lines.galley != nil || lc.lines.recorder != nil) {
		key = newGalleyKey(lc, box, skip, containingBlock, bottomSpace)
	}
	if plain && lc.lines.galley != nil {
		if line, resumeAt, ok := lc.lines.galley.take(lc, key); ok {
			box.PositionY = positionY
			line.Translate(0, positionY, false)
			return line, resumeAt
		}
	}
	// The floats above the line cannot touch it. They leave the formatting
	// context while the line is laid out, so that they do not touch the
	// line at the height of 0.
	var hidden *shapeList
	if len(lc.excludedShapes()) > 0 {
		hidden = lc.swapExcludedShapes(&shapeList{})
	}
	line, resumeAt := getNextLinebox(lc, box, 0, bottomSpace, skip, containingBlock,
		absoluteBoxes, fixedBoxes, firstLetterStyle, firstLineStyle)
	if hidden != nil {
		lc.swapExcludedShapes(hidden)
	}
	if line == nil {
		return nil, resumeAt
	}
	if plain && lc.lines.recorder != nil {
		lc.lines.recorder.record(key, line, resumeAt)
	}
	box.PositionY += positionY
	line.Translate(0, positionY, false)
	return line, resumeAt
}

// lineAtZero reports whether the next line of the line box box, at the
// height positionY, is laid out at a height of 0: no box of the line box is
// out of the normal flow, and each float of the formatting context is
// above positionY. A line of ::first-line next to a float stays at its
// height, because getNextLinebox gives it another box when a float is in
// the formatting context.
func (lc *layoutContext) lineAtZero(box *Box, positionY float64, firstLineStyle *Style) bool {
	shapes := lc.excludedShapes()
	if len(shapes) > 0 && firstLineStyle != nil {
		return false
	}
	for _, shape := range shapes {
		if shape.PositionY >= positionY || shape.PositionY+shape.MarginHeight() > positionY {
			return false
		}
	}
	return !lc.lineOutOfFlow(box)
}

// lineOutOfFlow reports whether a descendant of the line box box is out of
// the normal flow, such as a float, a positioned box, a running element or
// a note. The layout of such a line depends on its height.
func (lc *layoutContext) lineOutOfFlow(box *Box) bool {
	if out, ok := lc.lines.outOfFlow[box]; ok {
		return out
	}
	out := false
	box.Descendants(func(d *Box) bool {
		if d != box && !d.IsInNormalFlow() {
			out = true
		}
		return !out
	})
	if lc.lines.outOfFlow == nil {
		lc.lines.outOfFlow = map[*Box]bool{}
	}
	lc.lines.outOfFlow[box] = out
	return out
}
