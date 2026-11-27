// This file is not a port. It lets the lines of a column avoid a float of
// -ivy-wrap-flow of a column before it in the same row of columns, as a
// word processor wraps the text of each column around a shape that
// crosses the columns (D60, D63). In upstream, each column is a formatting
// context of its own, and its lines see only the floats of that column.
//
// The layout of a row of columns keeps the floats of -ivy-wrap-flow of
// each column that it lays out. The formatting context of a later column
// gets a copy of each of them, at the place on the page where the float
// is from that column. The lines of the column avoid the border box of
// the copy and its distance, as for any float of -ivy-wrap-flow. The
// floats and the blocks of the column avoid the copy on the side of the
// column where the copy leaves the larger room. The copy does not change
// the clearance, the height of the column or the place of a later float
// of the column.

package layout

// columnFloat is a float of -ivy-wrap-flow of a column: its laid out box,
// the index of its column, and the distance from the left of its column
// to the left of its margin box.
type columnFloat struct {
	box    *Box
	column int
	offset float64
}

// columnFloatState holds the floats of -ivy-wrap-flow of the columns of a
// row of columns that the layout laid out before the current column.
type columnFloatState struct {
	floats []columnFloat
	// copies holds the copies of the floats in the formatting contexts of
	// the columns.
	copies map[*Box]bool
	// step is the distance from the left of a column to the left of the
	// next column. It is negative for the columns from right to left.
	step float64
	// index is the index of the column that the layout lays out.
	index int
	// container is the box of the tree of the row of columns, and group
	// the index of the first child of the row in it. With index, they
	// name a column for the page floats of D60, see floatRoot.
	container *Box
	group     int
}

// reset starts a layout of a row of columns of the width width, with the
// gap gap between them, from right to left when rtl is set.
func (s *columnFloatState) reset(width, gap float64, rtl bool) {
	s.floats, s.copies = nil, nil
	s.step = width + gap
	if rtl {
		s.step = -s.step
	}
}

// collect keeps the floats of -ivy-wrap-flow of the laid out column box
// column of the index i. It does not keep the floats of a formatting
// context inside the column, such as a table cell.
func (s *columnFloatState) collect(column *Box, i int) {
	var walk func(box *Box)
	walk = func(box *Box) {
		for _, child := range box.Children {
			switch {
			case child.IsFloated():
				if side, _ := wrapFlow(child); side != "" {
					s.floats = append(s.floats, columnFloat{child, i, child.PositionX - column.PositionX})
				}
			case !child.EstablishesFormattingContext():
				walk(child)
			}
		}
	}
	walk(column)
}

// addColumnFloats adds to the formatting context of the column box column
// a copy of each float of the columns before it.
func (lc *layoutContext) addColumnFloats(column *Box) {
	s := &lc.columnFloats
	for _, f := range s.floats {
		if f.column >= s.index {
			continue
		}
		c := f.box.Copy()
		c.PositionX = column.PositionX + f.offset + float64(f.column-s.index)*s.step
		if s.copies == nil {
			s.copies = map[*Box]bool{}
		}
		s.copies[c] = true
		lc.appendExcludedShape(c)
	}
}

// isColumnFloat reports whether the excluded shape shape is the copy of a
// float of a column before the current column.
func (lc *layoutContext) isColumnFloat(shape *Box) bool {
	return lc.columnFloats.copies[shape]
}

// columnFloatBounds returns the bounds of the room of a box next to the
// copy shape of a float of a column before it, between left and right: the
// part of the room before the border box of the shape and its distance, or
// the part after it, whichever is wider.
func columnFloatBounds(shape *Box, left, right float64) (float64, float64) {
	_, distance := wrapFlow(shape)
	start := shape.BorderBoxX() - distance
	end := shape.BorderBoxX() + shape.BorderWidth() + distance
	if end <= left || start >= right {
		return left, right
	}
	if start-left >= right-end {
		return left, min(right, start)
	}
	return max(left, end), right
}
