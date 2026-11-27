// This file is not a port. It lays out the lines next to a float of
// -ivy-wrap-flow, as a word processor wraps the text on both sides of a
// picture or on one side of it (D63). Upstream has no such property.
//
// A line next to such a float has rooms: the parts of the line between the
// floats. The layout puts a line in the first room that holds its minimum
// width. When the line ends, the next line goes in the next room at the
// same height. When no room is left, the next line starts below the row.
// Each part of a row is a line box of its own.

package layout

import (
	"cmp"
	"math"
	"slices"
)

// minRoomWidth is the width in pixels of the narrowest room that holds
// text next to a float of -ivy-wrap-flow: 0.25 inch. LibreOffice 26.8
// puts text in a room of any width that holds the first word of the line,
// such as a room of 4 points that holds an "i" of 12 points. The pages of
// Word of the corpus have no such room, so the width of Word is not
// measured (D63).
const minRoomWidth = 24

// lineRow is a row of lines next to a float of -ivy-wrap-flow: the height
// y of the row, the room from start to end of the last line of the row,
// and the bottom of the row. The next line of the row goes in a room after
// that one, at the height y. When no such room holds the line, the line
// goes to the bottom of the row.
type lineRow struct {
	y, start, end, bottom float64
}

// room is a part of a line between the floats, from start to end.
type room struct {
	start, end float64
}

// wrapFlow returns the keyword and the distance of -ivy-wrap-flow of a
// float, or "" for auto.
func wrapFlow(box *Box) (string, float64) {
	t, ok := box.Style.Get("wrap_flow").(Tuple)
	if !ok || len(t) != 2 {
		return "", 0
	}
	return str(t[0]), num(t[1])
}

// wrapShape returns the points of the polygon of -ivy-wrap-shape of a
// float, x and y in pixels from the top left corner of its border box, or
// nil for none.
func wrapShape(box *Box) []float64 {
	t, ok := box.Style.Get("wrap_shape").(Tuple)
	if !ok || len(t) < 6 || len(t)%2 != 0 {
		return nil
	}
	points := make([]float64, len(t))
	for i, v := range t {
		f, ok := v.(Float)
		if !ok {
			return nil
		}
		points[i] = float64(f)
	}
	return points
}

// polygonSpan returns the part of the polygon of points, at x0 and y0, that
// is next to a line box at the height y and h high: the left and the right
// of the part, and the bottom of the polygon. ok is false when the
// polygon is not next to the line box, by the rule of collides.
func polygonSpan(points []float64, x0, y0, y, h float64) (start, end, bottom float64, ok bool) {
	n := len(points) / 2
	top, bottom := math.Inf(1), math.Inf(-1)
	for i := range n {
		top, bottom = min(top, y0+points[2*i+1]), max(bottom, y0+points[2*i+1])
	}
	if !collides(top, bottom, y, h) {
		return 0, 0, bottom, false
	}
	// the part of each edge between the top and the bottom of the line
	// box gives the part of the polygon next to it
	lo, hi := max(y, top), min(y+h, bottom)
	start, end = math.Inf(1), math.Inf(-1)
	for i := range n {
		j := (i + 1) % n
		x1, y1 := x0+points[2*i], y0+points[2*i+1]
		x2, y2 := x0+points[2*j], y0+points[2*j+1]
		a, b := max(min(y1, y2), lo), min(max(y1, y2), hi)
		if a > b {
			continue
		}
		if y1 == y2 {
			start, end = min(start, x1, x2), max(end, x1, x2)
			continue
		}
		for _, v := range [2]float64{a, b} {
			x := x1 + (x2-x1)*(v-y1)/(y2-y1)
			start, end = min(start, x), max(end, x)
		}
	}
	if start > end {
		return 0, 0, bottom, false
	}
	return start, end, bottom, true
}

// wrapsFlow reports whether a float of the current formatting context has
// -ivy-wrap-flow.
func (lc *layoutContext) wrapsFlow() bool {
	for _, shape := range lc.excludedShapes() {
		if side, _ := wrapFlow(shape); side != "" {
			return true
		}
	}
	return false
}

// collides reports whether a shape from top to bottom is next to a box
// from y to y+h. It is the test of avoid_collisions in upstream.
func collides(top, bottom, y, h float64) bool {
	return (top < y && y < bottom) ||
		(top < y+h && y+h < bottom) ||
		(top >= y && bottom <= y+h)
}

// lineRooms returns the rooms of a line box at the height y and h high, in
// the order of the text of the containing block, and the bottoms of the
// floats next to it. The floats without -ivy-wrap-flow bound the line, as
// in avoidCollisions. A float with -ivy-wrap-flow cuts the line at its
// border box and its distance, or at the part of the polygon of its
// -ivy-wrap-shape next to the line and its distance. A room narrower than minRoomWidth next to
// such a float holds no text. bounds is the room of the line without the
// floats of -ivy-wrap-flow.
func (lc *layoutContext) lineRooms(box, containingBlock *Box, y, h float64) (rooms []room, bottoms []float64, bounds room) {
	ltr := containingDirection(containingBlock) == "ltr"
	left, right := containingBlock.ContentBoxX(), containingBlock.ContentBoxX()+containingBlock.Width
	left += box.MarginLeft
	right -= box.MarginRight
	type cut struct {
		side       string
		start, end float64
	}
	var cuts []cut
	for _, shape := range lc.excludedShapes() {
		side, distance := wrapFlow(shape)
		if side == "" {
			top, bottom := shape.PositionY, shape.PositionY+shape.MarginHeight()
			if !collides(top, bottom, y, h) {
				continue
			}
			bottoms = append(bottoms, bottom)
			switch floatValue := str(shape.Style.Get("float")); {
			case floatValue == "left", floatValue == "inline-start" && ltr, floatValue == "inline-end" && !ltr:
				left = max(left, shape.PositionX+shape.MarginWidth())
			case floatValue == "right", floatValue == "inline-end" && ltr, floatValue == "inline-start" && !ltr:
				right = min(right, shape.PositionX)
			}
			continue
		}
		x, top := shape.BorderBoxX(), shape.BorderBoxY()
		start, end, bottom := x, x+shape.BorderWidth(), top+shape.BorderHeight()
		if points := wrapShape(shape); points != nil {
			var ok bool
			if start, end, bottom, ok = polygonSpan(points, x, top, y, h); !ok {
				continue
			}
		} else if !collides(top, bottom, y, h) {
			continue
		}
		bottoms = append(bottoms, bottom)
		cuts = append(cuts, cut{side, start - distance, end + distance})
	}
	bounds = room{left, right}
	if len(cuts) == 0 {
		return []room{bounds}, bottoms, bounds
	}
	free := []room{bounds}
	for _, c := range cuts {
		// start and end keep the text on one side of the float: the side
		// of the start or of the end of the line
		switch {
		case c.side == "start" && ltr, c.side == "end" && !ltr:
			c.end = math.Inf(1)
		case c.side == "end" && ltr, c.side == "start" && !ltr:
			c.start = math.Inf(-1)
		case c.side == "maximum":
			if c.start-left >= right-c.end {
				c.end = math.Inf(1)
			} else {
				c.start = math.Inf(-1)
			}
		}
		var next []room
		for _, r := range free {
			if c.end <= r.start || c.start >= r.end {
				next = append(next, r)
				continue
			}
			if c.start > r.start {
				next = append(next, room{r.start, c.start})
			}
			if c.end < r.end {
				next = append(next, room{c.end, r.end})
			}
		}
		free = next
	}
	for _, r := range free {
		if r.end-r.start >= minRoomWidth {
			rooms = append(rooms, r)
		}
	}
	slices.SortFunc(rooms, func(a, b room) int {
		if ltr {
			return cmp.Compare(a.start, b.start)
		}
		return cmp.Compare(b.start, a.start)
	})
	return rooms, bottoms, bounds
}

// after reports whether the room r is after the room of the last line of
// the row, in the order of the text.
func (row *lineRow) after(r room, ltr bool) bool {
	if ltr {
		return r.start >= row.end
	}
	return r.end <= row.start
}

// avoidRooms is avoidCollisions for a line box when a float of the
// formatting context has -ivy-wrap-flow. It returns the position and the
// available width of the first room that holds the width of the line
// box. When row is not nil and the line box is at the height of the row,
// the room is after the room of the last line of the row. When no such
// room holds the line box, the line box goes to the bottom of the row.
// When no room holds it, it goes below the floats next to it, as in
// avoidCollisions.
func (lc *layoutContext) avoidRooms(box, containingBlock *Box, row *lineRow) (positionX, positionY, availableWidth float64) {
	ltr := containingDirection(containingBlock) == "ltr"
	positionY = box.BorderBoxY()
	boxWidth, boxHeight := box.BorderWidth(), box.BorderHeight()
	var chosen room
	for {
		lc.spend(1)
		inRow := row != nil && positionY == row.y
		rooms, bottoms, bounds := lc.lineRooms(box, containingBlock, positionY, boxHeight)
		found := false
		for _, r := range rooms {
			if (!inRow || row.after(r, ltr)) && r.end-r.start >= boxWidth {
				chosen, found = r, true
				break
			}
		}
		if found {
			break
		}
		if inRow && row.bottom > positionY {
			positionY = row.bottom
			continue
		}
		next := math.Inf(1)
		for _, bottom := range bottoms {
			if bottom > positionY {
				next = min(next, bottom)
			}
		}
		if !math.IsInf(next, 1) {
			positionY = next
			continue
		}
		// No solution, the box goes here
		chosen = bounds
		for _, r := range rooms {
			if !inRow || row.after(r, ltr) {
				chosen = r
				break
			}
		}
		break
	}
	positionX = chosen.start
	if !ltr {
		// The position of the line is the position of the cursor, at the
		// right bound.
		positionX = chosen.end
	}
	return positionX - box.MarginLeft, positionY - box.MarginTop, chosen.end - chosen.start
}

// lineCollisions is avoidCollisions for a line box, or avoidRooms when a
// float of the formatting context has -ivy-wrap-flow. rooms reports the
// second case.
func (lc *layoutContext) lineCollisions(box, containingBlock *Box, row *lineRow) (positionX, positionY, availableWidth float64, rooms bool) {
	if lc.wrapsFlow() {
		positionX, positionY, availableWidth = lc.avoidRooms(box, containingBlock, row)
		return positionX, positionY, availableWidth, true
	}
	positionX, positionY, availableWidth = avoidCollisions(lc, box, containingBlock, false)
	return positionX, positionY, availableWidth, false
}

// rowAfter returns the row of the line when a room after the room from
// start to end of the line is next to a float of -ivy-wrap-flow, at the
// height of the line, or nil.
func (lc *layoutContext) rowAfter(line, containingBlock *Box, start, end float64) *lineRow {
	if line.Height <= 0 {
		return nil
	}
	ltr := containingDirection(containingBlock) == "ltr"
	row := &lineRow{y: line.PositionY, start: start, end: end}
	rooms, _, _ := lc.lineRooms(line, containingBlock, line.PositionY, line.Height)
	for _, r := range rooms {
		if row.after(r, ltr) {
			return row
		}
	}
	return nil
}

// lineFloatState holds the floats of -ivy-wrap-flow that the layout of a
// line placed. A float of the line keeps its place when the layout lays
// out the line again.
type lineFloatState struct {
	// active is set while getNextLinebox lays out a line.
	active bool
	// placed maps the box of the tree of a float of the line to its laid
	// out box.
	placed map[*Box]*Box
	// added holds the floats that the current try of the line placed for
	// the first time.
	added []*Box
}

// wrapsLine reports whether the line places the float box of
// -ivy-wrap-flow at once, and lays out its text in the rooms next to it.
// waiting holds the floats that wait for the end of the line, which keep
// their order.
func (lc *layoutContext) wrapsLine(box *Box, waiting []*Box) bool {
	side, _ := wrapFlow(box)
	return side != "" && lc.lineFloats.active && len(waiting) == 0
}

// lineWrapFloat lays out the float box of -ivy-wrap-flow of a line, of the
// containing block containingBlock, at its place, or returns the box that
// an earlier try of the line laid out. That box is again an excluded shape
// of the formatting context.
func (lc *layoutContext) lineWrapFloat(box, containingBlock *Box, absoluteBoxes, fixedBoxes *[]*Box, bottomSpace float64) *Box {
	s := &lc.lineFloats
	if placed, ok := s.placed[box]; ok {
		if !slices.Contains(lc.excludedShapes(), placed) {
			lc.appendExcludedShape(placed)
		}
		return placed
	}
	newBox, resumeAt := floatLayout(lc, box, containingBlock, absoluteBoxes, fixedBoxes, bottomSpace, nil)
	if len(resumeAt) > 0 {
		lc.addBrokenOutOfFlow(box, box, containingBlock, resumeAt)
	}
	if s.placed == nil {
		s.placed = map[*Box]*Box{}
	}
	s.placed[box] = newBox
	s.added = append(s.added, newBox)
	return newBox
}
