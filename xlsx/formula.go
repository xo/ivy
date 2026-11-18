package xlsx

import (
	"strconv"
	"strings"

	"github.com/xo/ivy/internal/sheet"
)

// This file computes the few formulas that a threshold of a conditional
// format can name: a number, a reference to a cell, a defined name, and
// SUM over ranges and the columns of tables, such as
// SUM(tblIncome[Amount]). It is not an engine of formulas. Another formula
// gives no value, and the threshold takes the lowest or the highest value.

// maxFormulaDepth bounds the defined names and the calls that a formula
// holds, one in the other.
const maxFormulaDepth = 8

// nameKey is a defined name: the index of its sheet, or -1 for the
// workbook, and its name in lower case.
type nameKey struct {
	sheet int
	name  string
}

// formulaValue is the value of a formula, and whether ivy computes it.
type formulaValue struct {
	value float64
	ok    bool
}

// table is a table of a worksheet (ECMA-376 Part 1, 18.5.1.2).
type table struct {
	// sheet is the name of the sheet of the table.
	sheet string
	ref   sheet.Range
	// header and totals are the number of the rows of the header and of
	// the totals.
	header, totals int
	// cols are the names of the columns, in lower case.
	cols []string
}

// readTable reads a table part of the worksheet sh.
func (b *book) readTable(sh *sheet.Sheet, part string) {
	if len(b.tables) >= b.lim.MaxObjects {
		return
	}
	doc, err := b.parsePart(part)
	if err != nil {
		return
	}
	root := doc.RootElement()
	ref, ok := parseRange(attr(root, "ref"))
	if !ok {
		return
	}
	t := table{
		sheet:  sh.Name,
		ref:    ref,
		header: max(0, integer(root, "headerRowCount", 1)),
		totals: max(0, integer(root, "totalsRowCount", 0)),
	}
	if c, ok := child(root, "tableColumns"); ok {
		for tc := range children(c, "tableColumn") {
			if len(t.cols) > ref.Col1-ref.Col0 {
				break
			}
			t.cols = append(t.cols, strings.ToLower(attr(tc, "name")))
		}
	}
	if b.tables == nil {
		b.tables = make(map[string]table)
	}
	for _, name := range []string{attr(root, "displayName"), attr(root, "name")} {
		if name != "" {
			b.tables[strings.ToLower(name)] = t
		}
	}
}

// evalNumber computes the formula f of the sheet sh of the index index. It
// returns false for a formula that it does not compute.
func (b *book) evalNumber(sh *sheet.Sheet, index int, f string, depth int) (float64, bool) {
	f = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(f), "="))
	if v, err := strconv.ParseFloat(f, 64); err == nil {
		return v, true
	}
	if depth > maxFormulaDepth || f == "" {
		return 0, false
	}
	if args, ok := call(f, "SUM"); ok {
		sum := 0.0
		for _, a := range splitArgs(args) {
			v, ok := b.sum(sh, index, a, depth+1)
			if !ok {
				return 0, false
			}
			sum += v
		}
		return sum, true
	}
	if def, ok := b.definedName(index, f); ok {
		return b.evalNumber(sh, index, def, depth+1)
	}
	if s, r, ok := b.cellRange(sh, f); ok && r.Row0 == r.Row1 && r.Col0 == r.Col1 {
		// a cell that holds no number is 0
		v, _ := sumRange(s, r)
		return v, true
	}
	return 0, false
}

// sum computes an argument of SUM: a range, a column of a table or a
// formula.
func (b *book) sum(sh *sheet.Sheet, index int, arg string, depth int) (float64, bool) {
	arg = strings.TrimSpace(arg)
	if depth > maxFormulaDepth {
		return 0, false
	}
	if v, err := strconv.ParseFloat(arg, 64); err == nil {
		return v, true
	}
	if s, r, ok := b.tableRange(sh, arg); ok {
		return sumRange(s, r)
	}
	if s, r, ok := b.cellRange(sh, arg); ok {
		return sumRange(s, r)
	}
	if def, ok := b.definedName(index, arg); ok {
		return b.sum(sh, index, def, depth+1)
	}
	return b.evalNumber(sh, index, arg, depth)
}

// sumRange adds the numbers of the cells of r in s. A cell of text counts
// as nothing, as in SUM.
func sumRange(s *sheet.Sheet, r sheet.Range) (float64, bool) {
	if s == nil {
		return 0, false
	}
	sum := 0.0
	for i := range s.Rows {
		row := &s.Rows[i]
		if row.Index < r.Row0 || row.Index > r.Row1 {
			continue
		}
		for _, c := range row.Cells {
			if c.Col >= r.Col0 && c.Col <= r.Col1 && c.Kind == sheet.KindNumber {
				sum += c.Value
			}
		}
	}
	return sum, true
}

// definedName returns the formula of the defined name s, of the sheet of
// the index index or of the workbook.
func (b *book) definedName(index int, s string) (string, bool) {
	name := strings.ToLower(s)
	if f, ok := b.names[nameKey{index, name}]; ok {
		return f, true
	}
	f, ok := b.names[nameKey{-1, name}]
	return f, ok
}

// cellRange returns the sheet and the range of a reference, such as $B$2,
// B2:C5 or 'My Sheet'!B2:C5. A reference without a sheet is a reference
// to sh.
func (b *book) cellRange(sh *sheet.Sheet, s string) (*sheet.Sheet, sheet.Range, bool) {
	name, ref, found := cutSheet(s)
	if found {
		if sh = b.otherSheet(sh, name); sh == nil {
			return nil, sheet.Range{}, false
		}
	}
	if strings.ContainsAny(ref, "[]()") {
		return nil, sheet.Range{}, false
	}
	if !strings.Contains(ref, ":") {
		// a single cell, and not a name such as A, which parseRange
		// reads as a column
		row, col, ok := parseRef(ref)
		return sh, sheet.Range{Row0: row, Col0: col, Row1: row, Col1: col}, ok
	}
	r, ok := parseRange(ref)
	return sh, r, ok
}

// cutSheet cuts the name of the sheet from a reference, such as
// 'My Sheet'!B2, and reports whether the reference names a sheet.
func cutSheet(s string) (string, string, bool) {
	s = strings.TrimSpace(s)
	i := strings.LastIndexByte(s, '!')
	if i < 0 {
		return "", s, false
	}
	name := s[:i]
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = strings.ReplaceAll(name[1:len(name)-1], "''", "'")
	}
	return name, s[i+1:], true
}

// tableRange returns the sheet and the range of a structured reference to
// a column of a table, such as tblIncome[Amount] or
// tblIncome[[#Data],[Amount]]. A reference without a column takes the
// whole table.
func (b *book) tableRange(sh *sheet.Sheet, s string) (*sheet.Sheet, sheet.Range, bool) {
	open := strings.IndexByte(s, '[')
	if open <= 0 || !strings.HasSuffix(s, "]") {
		return nil, sheet.Range{}, false
	}
	t, ok := b.tables[strings.ToLower(strings.TrimSpace(s[:open]))]
	if !ok {
		return nil, sheet.Range{}, false
	}
	inner := s[open+1 : len(s)-1]
	var items []string
	if strings.HasPrefix(inner, "[") {
		for item := range strings.SplitSeq(inner, ",") {
			item = strings.TrimSpace(item)
			if len(item) < 2 || item[0] != '[' || item[len(item)-1] != ']' {
				return nil, sheet.Range{}, false
			}
			items = append(items, item[1:len(item)-1])
		}
	} else if inner != "" {
		items = []string{inner}
	}
	r := t.ref
	r.Row0 += t.header
	r.Row1 -= t.totals
	col := -1
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item)) {
		case "#data":
		case "#all":
			r.Row0, r.Row1 = t.ref.Row0, t.ref.Row1
		case "#headers":
			r.Row0, r.Row1 = t.ref.Row0, t.ref.Row0+t.header-1
		case "#totals":
			r.Row0, r.Row1 = t.ref.Row1-t.totals+1, t.ref.Row1
		default:
			if col >= 0 {
				return nil, sheet.Range{}, false
			}
			name := strings.ToLower(item)
			for i, c := range t.cols {
				if c == name {
					col = i
				}
			}
			if col < 0 {
				return nil, sheet.Range{}, false
			}
		}
	}
	if col >= 0 {
		r.Col0 += col
		r.Col1 = r.Col0
	}
	if r.Row0 > r.Row1 {
		return nil, sheet.Range{}, false
	}
	sh = b.otherSheet(sh, t.sheet)
	return sh, r, sh != nil
}

// otherSheet returns the sheet of the name name: sh, or another sheet that
// the book read before sh, or nil. The book reads the sheets in order, and
// a sheet after sh has no rows yet.
func (b *book) otherSheet(sh *sheet.Sheet, name string) *sheet.Sheet {
	if strings.EqualFold(name, sh.Name) {
		return sh
	}
	if s := b.sheets[name]; s != nil && len(s.Rows) > 0 {
		return s
	}
	return nil
}

// call returns the arguments of a call of the function name, such as
// SUM(A1:A3), when the call is the whole formula f.
func call(f, name string) (string, bool) {
	if len(f) < len(name)+2 || !strings.EqualFold(f[:len(name)], name) || f[len(name)] != '(' || f[len(f)-1] != ')' {
		return "", false
	}
	args := f[len(name)+1 : len(f)-1]
	// the parenthesis of the call must close at the end of f
	depth := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case '(':
			depth++
		case ')':
			if depth--; depth < 0 {
				return "", false
			}
		}
	}
	return args, depth == 0
}

// splitArgs splits the arguments of a call at each comma outside quotes,
// brackets and parentheses.
func splitArgs(s string) []string {
	var args []string
	depth, quoted, start := 0, false, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\'' || c == '"':
			quoted = !quoted
		case quoted:
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			depth--
		case c == ',' && depth == 0:
			args = append(args, s[start:i])
			start = i + 1
		}
	}
	return append(args, s[start:])
}
