package vsdx

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ooxml"
)

// cell is a cell of a ShapeSheet: its value, from the attribute V, and its
// formula, from the attribute F. The package draws with the values, and
// computes only the formulas that are trivial (see eval).
type cell struct {
	v, f string
}

// row is a row of a section of a ShapeSheet, such as a row of a Geometry
// section.
type row struct {
	// t is the type of the row, such as "LineTo", or "" when the row takes
	// the type of the row that it inherits.
	t string
	// del reports a row that the sheet deletes from the row that it
	// inherits.
	del   bool
	cells map[string]cell
}

// section is a section of a ShapeSheet, such as Geometry or Character.
type section struct {
	// del reports a section that the sheet deletes.
	del bool
	// cells are the cells of the section itself, such as NoFill of a
	// Geometry section.
	cells map[string]cell
	// rows maps the key of each row to the row. The key is the index IX
	// of the row, or "N:" and its name.
	rows map[string]*row
	// keys are the keys of the rows, in the order of the file.
	keys []string
}

// The categories of the cells, for the styles that a cell inherits from.
const (
	catLine = iota
	catFill
	catText
	catOther
)

// sheet is a ShapeSheet: the cells and the sections of a shape, of a style
// or of a page.
type sheet struct {
	cells map[string]cell
	// sections maps the key of each section to the section. The key of a
	// section is its name, and for a section that a shape can have more
	// than once, such as Geometry, its name and its index IX.
	sections map[string]*section
	// styles are the IDs of the line style, the fill style and the text
	// style, from the attributes LineStyle, FillStyle and TextStyle, or ""
	// when the sheet does not name them.
	styles [3]string
}

// counter bounds the cells, the rows and the shapes that a page reads.
type counter struct {
	n, max int
}

// add counts n more objects. It returns false when the count passes the
// maximum.
func (c *counter) add(n int) bool {
	c.n += n
	return c.n <= c.max
}

// over reports whether the count passed the maximum.
func (c *counter) over() bool {
	return c.n > c.max
}

// parseSheet reads the cells and the sections of the element n, such as a
// Shape, a StyleSheet or a PageSheet.
func parseSheet(n roxmltree.Node, cnt *counter) sheet {
	s := sheet{cells: make(map[string]cell), sections: make(map[string]*section)}
	s.styles = [3]string{ooxml.Attr(n, "LineStyle"), ooxml.Attr(n, "FillStyle"), ooxml.Attr(n, "TextStyle")}
	for e := range n.Children() {
		if !e.IsElement() || !cnt.add(1) {
			continue
		}
		switch e.TagName().Name() {
		case "Cell":
			s.cells[ooxml.Attr(e, "N")] = parseCell(e)
		case "Section":
			name := ooxml.Attr(e, "N")
			key := name
			if ix := ooxml.Attr(e, "IX"); ix != "" && multiSection(name) {
				key = name + ix
			}
			s.sections[key] = parseSection(e, cnt)
		}
	}
	return s
}

// multiSection reports whether a shape can have more than one section with
// the name, each with its index.
func multiSection(name string) bool {
	return name == "Geometry"
}

// parseCell reads a Cell element.
func parseCell(n roxmltree.Node) cell {
	return cell{v: ooxml.Attr(n, "V"), f: ooxml.Attr(n, "F")}
}

// parseSection reads a Section element.
func parseSection(n roxmltree.Node, cnt *counter) *section {
	sec := &section{del: ooxml.Attr(n, "Del") == "1", cells: make(map[string]cell), rows: make(map[string]*row)}
	for e := range n.Children() {
		if !e.IsElement() || !cnt.add(1) {
			continue
		}
		switch e.TagName().Name() {
		case "Cell":
			sec.cells[ooxml.Attr(e, "N")] = parseCell(e)
		case "Row":
			key := ooxml.Attr(e, "IX")
			if key == "" {
				key = "N:" + ooxml.Attr(e, "N")
			}
			r := &row{t: ooxml.Attr(e, "T"), del: ooxml.Attr(e, "Del") == "1", cells: make(map[string]cell)}
			for c := range e.Children() {
				if c.IsElement() && c.TagName().Name() == "Cell" && cnt.add(1) {
					r.cells[ooxml.Attr(c, "N")] = parseCell(c)
				}
			}
			if _, ok := sec.rows[key]; !ok {
				sec.keys = append(sec.keys, key)
			}
			sec.rows[key] = r
		}
	}
	return sec
}

// category returns the category of the cell with the name, which says
// which style the cell inherits from.
func category(name string) int {
	switch {
	case strings.HasPrefix(name, "Line"), strings.HasPrefix(name, "BeginArrow"), strings.HasPrefix(name, "EndArrow"),
		name == "Rounding", name == "CompoundType":
		return catLine
	case strings.HasPrefix(name, "Fill"), strings.HasPrefix(name, "Shdw"), strings.HasPrefix(name, "ShapeShdw"):
		return catFill
	case strings.HasSuffix(name, "Margin"), name == "VerticalAlign", strings.HasPrefix(name, "TextBkgnd"),
		name == "TextDirection", name == "DefaultTabStop":
		return catText
	}
	return catOther
}

// chain is the list of the sheets that a shape inherits from, nearest
// first: the shape, its styles and its masters, in the order of [MS-VSDX].
type chain []*sheet

// cell returns the cell with the name from the first sheet of the chain
// that has it, and the index of that sheet.
func (c chain) cell(name string) (cell, int, bool) {
	for i, s := range c {
		if v, ok := s.cells[name]; ok {
			return v, i, true
		}
	}
	return cell{}, 0, false
}

// sectionKeys returns the keys of the sections whose names start with
// prefix, such as the Geometry sections, that the chain holds and does not
// delete, in the order of their indexes.
func (c chain) sectionKeys(prefix string) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, s := range c {
		for key, sec := range s.sections {
			if !strings.HasPrefix(key, prefix) || seen[key] {
				continue
			}
			seen[key] = true
			if !sec.del {
				keys = append(keys, key)
			}
		}
	}
	slices.SortFunc(keys, func(a, b string) int {
		return compareIndex(strings.TrimPrefix(a, prefix), strings.TrimPrefix(b, prefix))
	})
	return keys
}

// compareIndex compares two keys of rows or of sections: numbers in their
// order, before names.
func compareIndex(a, b string) int {
	x, errA := strconv.Atoi(a)
	y, errB := strconv.Atoi(b)
	switch {
	case errA == nil && errB == nil:
		return cmp.Compare(x, y)
	case errA == nil:
		return -1
	case errB == nil:
		return 1
	}
	return strings.Compare(a, b)
}

// sectionCell returns a cell of the section with the key itself, such as
// NoFill of a Geometry section.
func (c chain) sectionCell(key, name string) (cell, bool) {
	for _, s := range c {
		sec, ok := s.sections[key]
		if !ok {
			continue
		}
		if sec.del {
			return cell{}, false
		}
		if v, ok := sec.cells[name]; ok {
			return v, true
		}
	}
	return cell{}, false
}

// rowView is a row of a section with the rows that it inherits, nearest
// first.
type rowView struct {
	key string
	t   string
	// rows holds the row of each sheet of the chain that has it, and
	// sheets holds the index of that sheet in the chain.
	rows   []*row
	sheets []int
}

// cell returns the cell of the row with the name, and the index in the
// chain of the sheet that gives it.
func (r *rowView) cell(name string) (cell, int, bool) {
	for i, x := range r.rows {
		if v, ok := x.cells[name]; ok {
			return v, r.sheets[i], true
		}
	}
	return cell{}, 0, false
}

// rows returns the rows of the section with the key, with the rows that
// they inherit, in the order of their indexes. It leaves out a deleted
// section and the deleted rows. It returns at most limit rows.
func (c chain) rows(key string, limit int) []*rowView {
	var out []*rowView
	byKey := make(map[string]*rowView)
	deleted := make(map[string]bool)
	for i, s := range c {
		sec, ok := s.sections[key]
		if !ok {
			continue
		}
		if sec.del && len(out) == 0 && len(deleted) == 0 {
			return nil
		}
		for _, k := range sec.keys {
			r := sec.rows[k]
			if deleted[k] {
				continue
			}
			v, ok := byKey[k]
			if !ok {
				if r.del {
					deleted[k] = true
					continue
				}
				if len(out) >= limit {
					continue
				}
				v = &rowView{key: k}
				byKey[k] = v
				out = append(out, v)
			}
			if v.t == "" {
				v.t = r.t
			}
			v.rows = append(v.rows, r)
			v.sheets = append(v.sheets, i)
		}
	}
	slices.SortStableFunc(out, func(a, b *rowView) int {
		return compareIndex(a.key, b.key)
	})
	return out
}
