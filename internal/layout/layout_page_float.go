// This file is not a port. It places a float of the flow of the page at a
// vertical position from the top of the page or of the page area, by the
// property -ivy-float-position, as a word processor places a frame, a
// table that floats or a shape (D60). Upstream places each float at the
// line of its anchor.
//
// A page float is a box with float: left or float: right and
// -ivy-float-position, whose formatting context is the one of the root box
// of the page. The layout lays it out without a page break and gives it
// its place on the page. When the float is above the current position of
// the flow, it covers content that the page already placed. The page maker
// then makes the page again from the same state, and places the float
// before the flow. The flow skips the float when it meets its anchor
// again. A float that a page placed before its flow stays on that page,
// also when its anchor goes to the next page. The flow of a later page
// skips it too.

package layout

import (
	"math"
	"slices"
)

// pageFloat is a page float that the flow met: the box of the tree, a copy
// of its containing block, and the laid out box.
type pageFloat struct {
	box             *Box
	containingBlock *Box
	newBox          *Box
}

// pageFloatState is the state of the page floats of the page that the page
// maker makes.
type pageFloatState struct {
	// flow is set while the page lays out the flow of the root box. Only
	// the flow places a page float.
	flow bool
	// page is the page box of the page.
	page *Box
	// before holds the page floats that the page places before its flow,
	// and done the boxes of the page floats that the pages before placed.
	// The flow skips them.
	before []pageFloat
	done   []*Box
	// placed holds the page floats that the flow placed, and above those
	// of them that cover content that the page placed before them.
	placed []pageFloat
	above  []pageFloat
	// shapes holds the laid out page floats of the page, which the
	// ordinary floats do not stay below.
	shapes map[*Box]bool
	// layouts counts the layouts of pages, for the tests.
	layouts int
}

// startPageFloats starts a layout of a page that places the page floats
// before before its flow. makePageGrown sets the page and the floats of the
// pages before.
func (lc *layoutContext) startPageFloats(before []pageFloat) {
	s := &lc.pageFloats
	s.flow, s.page = false, nil
	s.before, s.done = before, nil
	s.placed, s.above = nil, nil
	s.shapes = nil
	s.layouts++
}

// isPageFloat reports whether the flow of the page places box as a page
// float. A float with -ivy-float-position in another formatting context,
// such as a table cell, a column or a float, is an ordinary float. So is a
// float of a part of the galley, as only the pages place a page float
// (D57).
func (lc *layoutContext) isPageFloat(box *Box) bool {
	if !lc.pageFloats.flow || lc.lines.recorder != nil || !box.IsFloated() {
		return false
	}
	if _, ok := box.Style.Get("float_position").(Tuple); !ok {
		return false
	}
	n := len(lc.excludedShapesRootBoxes)
	return n > 0 && lc.excludedShapesRootBoxes[n-1] == nil
}

// pageFloatSkipped reports whether the flow skips the page float box: the
// page placed it before its flow, or a page before placed it.
func (lc *layoutContext) pageFloatSkipped(box *Box) bool {
	s := &lc.pageFloats
	if slices.Contains(s.done, box) {
		return true
	}
	return slices.ContainsFunc(s.before, func(f pageFloat) bool { return f.box == box })
}

// flowPageFloat lays out the page float box that the flow meets, of the
// containing block containingBlock, when the flow is at the height flowY.
// It returns the laid out box, or nil when the flow skips the float.
func (lc *layoutContext) flowPageFloat(box, containingBlock *Box, flowY float64, absoluteBoxes, fixedBoxes *[]*Box) *Box {
	if lc.pageFloatSkipped(box) {
		return nil
	}
	newBox := pageFloatLayout(lc, box, containingBlock, absoluteBoxes, fixedBoxes)
	lc.addPageFloatShape(newBox)
	s := &lc.pageFloats
	f := pageFloat{box: box, containingBlock: containingBlock.Copy(), newBox: newBox}
	s.placed = putPageFloat(s.placed, f)
	// The top of the margin box of the float is above the flow, so the
	// float covers content that the page placed.
	if newBox.PositionY+1e-6 < flowY {
		s.above = putPageFloat(s.above, f)
	}
	return newBox
}

// putPageFloat puts f in floats, in the place of an earlier layout of the
// same box.
func putPageFloat(floats []pageFloat, f pageFloat) []pageFloat {
	for i, other := range floats {
		if other.box == f.box {
			floats[i] = f
			return floats
		}
	}
	return append(floats, f)
}

// addPageFloatShape adds a laid out page float to the excluded shapes of
// the current formatting context.
func (lc *layoutContext) addPageFloatShape(newBox *Box) {
	lc.appendExcludedShape(newBox)
	s := &lc.pageFloats
	if s.shapes == nil {
		s.shapes = map[*Box]bool{}
	}
	s.shapes[newBox] = true
}

// placePageFloatsBefore lays out the page floats that the page places
// before its flow, in the formatting context of the root box, and returns
// them.
func (lc *layoutContext) placePageFloatsBefore(absoluteBoxes, fixedBoxes *[]*Box) []*Box {
	s := &lc.pageFloats
	boxes := make([]*Box, 0, len(s.before))
	for _, f := range s.before {
		lc.spend(1)
		f.box.PositionY = s.page.ContentBoxY()
		newBox := pageFloatLayout(lc, f.box, f.containingBlock, absoluteBoxes, fixedBoxes)
		lc.addPageFloatShape(newBox)
		boxes = append(boxes, newBox)
	}
	return boxes
}

// finishPageFloats returns the boxes of the page floats of the pages up to
// the page whose root box is root: done, the floats that the page placed
// before its flow, and the floats that the flow placed and that the page
// holds. A float whose anchor the page did not keep, such as the anchor in
// a line that went to the next page, is not on the page. It also keeps in
// above only the floats that the page holds.
func (lc *layoutContext) finishPageFloats(root *Box) []*Box {
	s := &lc.pageFloats
	s.flow = false
	if len(s.before) == 0 && len(s.placed) == 0 {
		return s.done
	}
	held := map[*Box]bool{}
	if len(s.placed) > 0 {
		root.Descendants(func(d *Box) bool {
			held[d] = true
			return true
		})
	}
	done := slices.Clip(s.done)
	for _, f := range s.before {
		done = append(done, f.box)
	}
	for _, f := range s.placed {
		if held[f.newBox] && !slices.Contains(done, f.box) {
			done = append(done, f.box)
		}
	}
	s.above = slices.DeleteFunc(s.above, func(f pageFloat) bool { return !held[f.newBox] })
	return done
}

// pageFloatLayout lays out the page float box of the containing block
// containingBlock without a page break, as the footnote area, and gives it
// its place on the page.
//
// Port: upstream places a float at the line of its anchor, below the
// floats before it and beside them. A page float is at the position of
// -ivy-float-position. A float that passes the bottom of its area moves up
// until it fits, but not above the top of the area. A negative length can
// put the float above the top of the area. The float keeps the
// side of its float value and its margins in its containing block. The
// layout does not move it for the other floats, so two page floats can
// cover each other.
func pageFloatLayout(lc *layoutContext, box, containingBlock *Box, absoluteBoxes, fixedBoxes *[]*Box) *Box {
	lc.enter()
	defer lc.leave()
	newBox, _ := floatBoxLayout(lc, box, containingBlock, absoluteBoxes, fixedBoxes, math.Inf(-1), nil, false)
	y := lc.pageFloatTop(newBox)
	x := containingBlock.ContentBoxX()
	if floatsRight(newBox) {
		x += containingBlock.Width - newBox.MarginWidth()
	}
	newBox.Translate(x-newBox.PositionX, y-newBox.MarginTop-newBox.PositionY, false)
	return newBox
}

// pageFloatTop returns the top of the border box of the laid out page
// float box, from its -ivy-float-position on the page of the state.
func (lc *layoutContext) pageFloatTop(box *Box) float64 {
	page := lc.pageFloats.page
	t, _ := box.Style.Get("float_position").(Tuple)
	if page == nil || len(t) != 2 {
		return box.BorderBoxY()
	}
	top, bottom := page.ContentBoxY(), page.ContentBoxY()+page.Height
	if isStr(t[0], "page") {
		top, bottom = page.PositionY, page.PositionY+page.MarginHeight()
	}
	height := box.BorderHeight()
	var y float64
	switch v := t[1].(type) {
	case Float:
		y = top + float64(v)
	case String:
		switch v {
		case "center":
			y = top + (bottom-top-height)/2
		case "bottom":
			y = bottom - height
		default:
			y = top
		}
	}
	if y+height > bottom {
		y = min(y, max(bottom-height, top))
	}
	if _, ok := t[1].(String); ok {
		// A float at the center or at the bottom of an area that is
		// lower than the float starts at the top of the area.
		y = max(y, top)
	}
	return y
}

// floatsRight reports whether the float box floats to the right of its
// containing block.
func floatsRight(box *Box) bool {
	floatValue := box.Style.Get("float")
	direction := box.Style.Get("direction")
	return isStr(floatValue, "right") ||
		(isStr(direction, "ltr") && isStr(floatValue, "inline-end")) ||
		(isStr(direction, "rtl") && isStr(floatValue, "inline-start"))
}

// lastOrdinaryShape returns the last excluded shape of shapes that is not
// a page float, or nil.
func (lc *layoutContext) lastOrdinaryShape(shapes []*Box) *Box {
	for i := len(shapes) - 1; i >= 0; i-- {
		if !lc.pageFloats.shapes[shapes[i]] {
			return shapes[i]
		}
	}
	return nil
}
