package odf

import (
	"math"
	"strconv"
	"strings"

	"github.com/xo/ivy/internal/sheet"
)

// odsMap is a condition of a style of a cell, style:map: the cell takes
// the style apply when the condition is true. base is the cell that the
// relative references of the condition start from.
type odsMap struct {
	cond, apply string
	baseRow     int
	baseCol     int
}

// odsValue is the value of a cell: a number, or a text. empty is set for a
// cell without a value.
type odsValue struct {
	num   float64
	text  string
	isNum bool
	empty bool
}

// odsCond is a cell whose style has conditions: its row in Sheet.Rows, its
// index in the cells of the row, and its value.
type odsCond struct {
	row, cell int
	value     odsValue
}

// maxCondLen is the length of a condition that the reader evaluates.
const maxCondLen = 1024

// odsCellValue returns the value of a table:table-cell, from its office
// attributes, or its text.
func odsCellValue(c sheet.Cell, attr func(string) string) odsValue {
	switch attr("value-type") {
	case "float", "percentage", "currency":
		if v, err := strconv.ParseFloat(attr("value"), 64); err == nil {
			return odsValue{num: v, isNum: true}
		}
	case "boolean":
		if attr("boolean-value") == "true" {
			return odsValue{num: 1, isNum: true}
		}
		return odsValue{isNum: true}
	}
	if c.Text == "" {
		return odsValue{empty: true}
	}
	return odsValue{text: c.Text}
}

// applyConds gives each cell of conds the style of the first condition of
// its style that is true. values holds the values of the cells of the
// table, for the references of the conditions.
func (r *odsReader) applyConds(sh *sheet.Sheet, conds []odsCond, values map[[2]int]odsValue) {
	for _, cd := range conds {
		if cd.row >= len(sh.Rows) || cd.cell >= len(sh.Rows[cd.row].Cells) {
			continue
		}
		c := &sh.Rows[cd.row].Cells[cd.cell]
		row := sh.Rows[cd.row].Index
		for _, m := range r.maps[c.Style] {
			e := &odsEval{row: row, col: c.Col, value: cd.value, values: values, dRow: row - m.baseRow, dCol: c.Col - m.baseCol}
			if e.condition(m.cond) {
				c.Style = r.condStyle(c.Style, m.apply)
				break
			}
		}
	}
}

// condStyle returns the index of the style of a cell of the style index
// with the style apply of a condition over it. The style of a condition
// changes only what it sets itself, or its parents other than Default.
func (r *odsReader) condStyle(index int, apply string) int {
	key := strconv.Itoa(index) + "\x00" + apply
	if i, ok := r.condIndex[key]; ok {
		return i
	}
	i := index
	defer func() { r.condIndex[key] = i }()
	if r.styles.style("table-cell", apply) == nil || len(r.wb.Styles) >= r.d.lim.MaxObjects {
		return i
	}
	p := map[string]string{}
	for k, v := range r.cellProps[index] {
		p[k] = v
	}
	var chain []*odsStyle
	seen := map[string]bool{}
	for n := apply; n != "" && n != "Default" && !seen[n] && len(chain) < 32; {
		seen[n] = true
		st := r.styles.style("table-cell", n)
		if st == nil {
			break
		}
		chain = append(chain, st)
		n = st.parent
	}
	for j := len(chain) - 1; j >= 0; j-- {
		for k, v := range chain[j].props {
			p[k] = v
		}
	}
	i = len(r.wb.Styles)
	r.wb.Styles = append(r.wb.Styles, r.styles.cellStyle(p))
	r.cellProps = append(r.cellProps, p)
	return i
}

// odsEval evaluates a condition of a style:map for one cell: row and col
// are the cell, from 0, value its value, and dRow and dCol its offset from
// the base cell of the condition.
type odsEval struct {
	row, col   int
	value      odsValue
	values     map[[2]int]odsValue
	dRow, dCol int

	s   string
	pos int
	bad bool
}

// condition reports whether the condition s of ODF 1.3, 19.477
// (style:condition), is true for the cell: cell-content() and a
// comparison, cell-content-is-between(), cell-content-is-not-between() or
// is-true-formula(). A condition that the reader cannot evaluate is false.
func (e *odsEval) condition(s string) bool {
	if len(s) > maxCondLen {
		return false
	}
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "cell-content()"):
		e.s, e.pos = strings.TrimPrefix(s, "cell-content()"), 0
		e.space()
		op := e.operator()
		if op == "" {
			return false
		}
		v := e.expr()
		return e.done() && truth(compare(e.value, v, op))
	case strings.HasPrefix(s, "cell-content-is-between(") || strings.HasPrefix(s, "cell-content-is-not-between("):
		not := strings.HasPrefix(s, "cell-content-is-not-between(")
		e.s = s[strings.IndexByte(s, '(')+1:]
		e.pos = 0
		a := e.expr()
		if !e.eat(",") {
			return false
		}
		b := e.expr()
		if !e.eat(")") || !e.done() {
			return false
		}
		lo, hi := a, b
		if truth(compare(a, b, ">")) {
			lo, hi = b, a
		}
		in := truth(compare(e.value, lo, ">=")) && truth(compare(e.value, hi, "<="))
		return in != not
	case strings.HasPrefix(s, "is-true-formula("):
		e.s, e.pos = strings.TrimPrefix(s, "is-true-formula("), 0
		v := e.expr()
		return e.eat(")") && e.done() && truth(v)
	}
	return false
}

// done reports whether the evaluation read the whole text without a fault.
func (e *odsEval) done() bool {
	e.space()
	return !e.bad && e.pos == len(e.s)
}

// space skips the white space.
func (e *odsEval) space() {
	for e.pos < len(e.s) && e.s[e.pos] == ' ' {
		e.pos++
	}
}

// eat reads the text tok, and reports whether it was there.
func (e *odsEval) eat(tok string) bool {
	e.space()
	if strings.HasPrefix(e.s[e.pos:], tok) {
		e.pos += len(tok)
		return true
	}
	return false
}

// operator reads a comparison operator, or returns "".
func (e *odsEval) operator() string {
	e.space()
	for _, op := range [...]string{"<=", ">=", "!=", "<>", "=", "<", ">"} {
		if strings.HasPrefix(e.s[e.pos:], op) {
			e.pos += len(op)
			if op == "!=" {
				return "<>"
			}
			return op
		}
	}
	return ""
}

// expr reads a comparison.
func (e *odsEval) expr() odsValue {
	v := e.concat()
	for !e.bad {
		op := e.operator()
		if op == "" {
			return v
		}
		v = compare(v, e.concat(), op)
	}
	return v
}

// concat reads a concatenation of texts with &.
func (e *odsEval) concat() odsValue {
	v := e.sum()
	for !e.bad && e.eat("&") {
		w := e.sum()
		v = odsValue{text: textOf(v) + textOf(w)}
	}
	return v
}

// sum reads a sum or a difference.
func (e *odsEval) sum() odsValue {
	v := e.product()
	for !e.bad {
		switch {
		case e.eat("+"):
			v = odsValue{num: number(v) + number(e.product()), isNum: true}
		case e.eat("-"):
			v = odsValue{num: number(v) - number(e.product()), isNum: true}
		default:
			return v
		}
	}
	return v
}

// product reads a product or a quotient.
func (e *odsEval) product() odsValue {
	v := e.unary()
	for !e.bad {
		switch {
		case e.eat("*"):
			v = odsValue{num: number(v) * number(e.unary()), isNum: true}
		case e.eat("/"):
			d := number(e.unary())
			if d == 0 {
				e.bad = true
				return v
			}
			v = odsValue{num: number(v) / d, isNum: true}
		default:
			return v
		}
	}
	return v
}

// unary reads a value with a sign.
func (e *odsEval) unary() odsValue {
	switch {
	case e.eat("-"):
		return odsValue{num: -number(e.unary()), isNum: true}
	case e.eat("+"):
		return e.unary()
	}
	return e.primary()
}

// primary reads a number, a text, a reference to a cell, a function or an
// expression in parentheses.
func (e *odsEval) primary() odsValue {
	e.space()
	if e.pos >= len(e.s) {
		e.bad = true
		return odsValue{}
	}
	switch ch := e.s[e.pos]; {
	case ch == '(':
		e.pos++
		v := e.expr()
		if !e.eat(")") {
			e.bad = true
		}
		return v
	case ch == '"':
		end := strings.IndexByte(e.s[e.pos+1:], '"')
		if end < 0 {
			e.bad = true
			return odsValue{}
		}
		v := odsValue{text: e.s[e.pos+1 : e.pos+1+end]}
		e.pos += end + 2
		return v
	case ch == '[':
		end := strings.IndexByte(e.s[e.pos:], ']')
		if end < 0 {
			e.bad = true
			return odsValue{}
		}
		ref := e.s[e.pos+1 : e.pos+end]
		e.pos += end + 1
		return e.ref(ref)
	case ch >= '0' && ch <= '9' || ch == '.':
		start := e.pos
		for e.pos < len(e.s) && (e.s[e.pos] >= '0' && e.s[e.pos] <= '9' || e.s[e.pos] == '.' || e.s[e.pos] == 'E' || e.s[e.pos] == 'e') {
			e.pos++
		}
		v, err := strconv.ParseFloat(e.s[start:e.pos], 64)
		if err != nil {
			e.bad = true
		}
		return odsValue{num: v, isNum: true}
	}
	start := e.pos
	for e.pos < len(e.s) && (e.s[e.pos] >= 'A' && e.s[e.pos] <= 'Z' || e.s[e.pos] >= 'a' && e.s[e.pos] <= 'z' || e.s[e.pos] == ':' || e.s[e.pos] == '.') {
		e.pos++
	}
	name := strings.ToUpper(strings.TrimPrefix(e.s[start:e.pos], "of:"))
	if name == "" || !e.eat("(") {
		e.bad = true
		return odsValue{}
	}
	var args []odsValue
	if !e.eat(")") {
		for len(args) < 32 {
			args = append(args, e.expr())
			if e.bad || e.eat(")") {
				break
			}
			if !e.eat(";") && !e.eat(",") {
				e.bad = true
				break
			}
		}
	}
	return e.call(name, args)
}

// call returns the value of the function name of the arguments args. A
// function that the reader does not know makes the condition false.
func (e *odsEval) call(name string, args []odsValue) odsValue {
	b := func(v bool) odsValue {
		if v {
			return odsValue{num: 1, isNum: true}
		}
		return odsValue{isNum: true}
	}
	n := func(i int) float64 {
		if i >= len(args) {
			e.bad = true
			return 0
		}
		return number(args[i])
	}
	switch name {
	case "ROW":
		return odsValue{num: float64(e.row + 1), isNum: true}
	case "COLUMN":
		return odsValue{num: float64(e.col + 1), isNum: true}
	case "TRUE":
		return b(true)
	case "FALSE":
		return b(false)
	case "ISEVEN":
		return b(int64(math.Trunc(n(0)))%2 == 0)
	case "ISODD":
		return b(int64(math.Trunc(n(0)))%2 != 0)
	case "MOD":
		d := n(1)
		if d == 0 {
			e.bad = true
			return odsValue{}
		}
		x := n(0)
		return odsValue{num: x - d*math.Floor(x/d), isNum: true}
	case "ABS":
		return odsValue{num: math.Abs(n(0)), isNum: true}
	case "INT":
		return odsValue{num: math.Floor(n(0)), isNum: true}
	case "NOT":
		if len(args) != 1 {
			e.bad = true
			return odsValue{}
		}
		return b(!truth(args[0]))
	case "AND", "OR":
		if len(args) == 0 {
			e.bad = true
			return odsValue{}
		}
		all, some := true, false
		for _, a := range args {
			all = all && truth(a)
			some = some || truth(a)
		}
		return b(name == "AND" && all || name == "OR" && some)
	case "ISBLANK":
		return b(len(args) == 1 && args[0].empty)
	}
	e.bad = true
	return odsValue{}
}

// ref returns the value of the reference to a cell ref, such as ".$U$9" or
// ".A2". A part without $ is relative to the base cell of the condition. A
// reference to another sheet or to a range makes the condition false.
func (e *odsEval) ref(ref string) odsValue {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, ".") || strings.Contains(ref, ":") {
		e.bad = true
		return odsValue{}
	}
	s := ref[1:]
	absCol := strings.HasPrefix(s, "$")
	s = strings.TrimPrefix(s, "$")
	i := 0
	for i < len(s) && (s[i]|0x20) >= 'a' && (s[i]|0x20) <= 'z' {
		i++
	}
	absRow := strings.HasPrefix(s[i:], "$")
	row, col, ok := odsCell(s[:i] + strings.TrimPrefix(s[i:], "$"))
	if !ok {
		e.bad = true
		return odsValue{}
	}
	if !absRow {
		row += e.dRow
	}
	if !absCol {
		col += e.dCol
	}
	if v, ok := e.values[[2]int{row, col}]; ok {
		return v
	}
	return odsValue{empty: true}
}

// number returns v as a number. A text is 0, as Calc reads it in a sum.
func number(v odsValue) float64 {
	if v.isNum {
		return v.num
	}
	return 0
}

// textOf returns v as a text.
func textOf(v odsValue) string {
	if v.isNum {
		return strconv.FormatFloat(v.num, 'g', -1, 64)
	}
	return v.text
}

// truth reports whether v is true: a number other than 0.
func truth(v odsValue) bool {
	return v.isNum && v.num != 0
}

// compare compares a and b with op, as Calc does: an empty cell is 0 or
// the empty text, a number is less than a text, and texts compare without
// case.
func compare(a, b odsValue, op string) odsValue {
	if a.empty {
		a = odsValue{isNum: b.isNum || b.empty}
	}
	if b.empty {
		b = odsValue{isNum: a.isNum}
	}
	c := 0
	switch {
	case a.isNum && b.isNum:
		switch {
		case a.num < b.num:
			c = -1
		case a.num > b.num:
			c = 1
		}
	case a.isNum:
		c = -1
	case b.isNum:
		c = 1
	default:
		c = strings.Compare(strings.ToLower(a.text), strings.ToLower(b.text))
	}
	var v bool
	switch op {
	case "=":
		v = c == 0
	case "<>":
		v = c != 0
	case "<":
		v = c < 0
	case ">":
		v = c > 0
	case "<=":
		v = c <= 0
	case ">=":
		v = c >= 0
	}
	if v {
		return odsValue{num: 1, isNum: true}
	}
	return odsValue{isNum: true}
}
