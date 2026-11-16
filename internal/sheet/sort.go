package sheet

import (
	"cmp"
	"slices"
)

// sortRows sorts the rows, the cells and the columns of sh. Of two rows or
// two cells with the same index, the later one stays.
func sortRows(sh *Sheet) {
	slices.SortStableFunc(sh.Rows, func(a, b Row) int { return cmp.Compare(a.Index, b.Index) })
	rows := sh.Rows[:0]
	for _, r := range sh.Rows {
		if n := len(rows); n > 0 && rows[n-1].Index == r.Index {
			cells := make([]Cell, 0, len(rows[n-1].Cells)+len(r.Cells))
			cells = append(cells, rows[n-1].Cells...)
			cells = append(cells, r.Cells...)
			rows[n-1] = r
			rows[n-1].Cells = cells
			continue
		}
		rows = append(rows, r)
	}
	sh.Rows = rows
	for i := range sh.Rows {
		r := &sh.Rows[i]
		slices.SortStableFunc(r.Cells, func(a, b Cell) int { return cmp.Compare(a.Col, b.Col) })
		cells := r.Cells[:0]
		for _, c := range r.Cells {
			if n := len(cells); n > 0 && cells[n-1].Col == c.Col {
				cells[n-1] = c
				continue
			}
			cells = append(cells, c)
		}
		r.Cells = cells
	}
	slices.SortStableFunc(sh.Cols, func(a, b Col) int { return cmp.Compare(a.Min, b.Min) })
	cols := sh.Cols[:0]
	for _, c := range sh.Cols {
		if c.Max < c.Min {
			continue
		}
		if n := len(cols); n > 0 && c.Min <= cols[n-1].Max {
			// an overlap: the later run starts after the earlier one
			c.Min = cols[n-1].Max + 1
			if c.Max < c.Min {
				continue
			}
		}
		cols = append(cols, c)
	}
	sh.Cols = cols
}
