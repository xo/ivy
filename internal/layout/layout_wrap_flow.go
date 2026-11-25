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
// text next to a float of -ivy-wrap-flow: 0.25 inch. The width of Word is
// not measured (D63).
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
// border box and its distance. A room narrower than minRoomWidth next to
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
		top := shape.BorderBoxY()
		bottom := top + shape.BorderHeight()
		if !collides(top, bottom, y, h) {
			continue
		}
		bottoms = append(bottoms, bottom)
		x := shape.BorderBoxX()
		cuts = append(cuts, cut{side, x - distance, x + shape.BorderWidth() + distance})
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
