package vsdx

import (
	"math"
	"strconv"
	"strings"
)

// eval computes a formula of a cell that gives a number: numbers with
// their units, the cells of the shape by their names, the cells of the
// sections User, Property, Geometry, Connection, Control and Scratch, the
// cells of other shapes (Sheet.5!Width), the operators of arithmetic and
// of comparison, parentheses, and the functions of evalFuncs, such as IF,
// MIN and GUARD. lookup returns the value of a cell by its reference, such
// as "Width", "User.Base" or "Sheet.5!Height". eval returns false for any
// other formula, such as one that gives a string or calls THEMEVAL, and the
// package then draws with the value of the cell.
func eval(f string, lookup func(name string) (float64, bool)) (float64, bool) {
	p := &evaluator{s: f, lookup: lookup}
	v, ok := p.compare(0)
	p.space()
	if !ok || p.i != len(p.s) || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// maxEvalDepth is the depth of the parentheses and of the calls that eval
// follows.
const maxEvalDepth = 16

// evaluator is the state of eval.
type evaluator struct {
	s      string
	i      int
	lookup func(name string) (float64, bool)
}

// space skips the spaces.
func (p *evaluator) space() {
	for p.i < len(p.s) && p.s[p.i] == ' ' {
		p.i++
	}
}

// compare reads a comparison of two sums, or a sum. A comparison gives 1
// when it is true, and 0 when it is false.
func (p *evaluator) compare(depth int) (float64, bool) {
	v, ok := p.sum(depth)
	if !ok {
		return 0, false
	}
	p.space()
	var op string
	for _, o := range [...]string{"<=", ">=", "<>", "=", "<", ">"} {
		if strings.HasPrefix(p.s[p.i:], o) {
			op = o
			break
		}
	}
	if op == "" {
		return v, true
	}
	p.i += len(op)
	w, ok := p.sum(depth)
	if !ok {
		return 0, false
	}
	var r bool
	switch op {
	case "<=":
		r = v <= w
	case ">=":
		r = v >= w
	case "<>":
		r = v != w
	case "=":
		r = v == w
	case "<":
		r = v < w
	case ">":
		r = v > w
	}
	return boolNum(r), true
}

// boolNum returns 1 for true and 0 for false.
func boolNum(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// sum reads a sum of products.
func (p *evaluator) sum(depth int) (float64, bool) {
	v, ok := p.product(depth)
	if !ok {
		return 0, false
	}
	for {
		p.space()
		if p.i >= len(p.s) || p.s[p.i] != '+' && p.s[p.i] != '-' {
			return v, true
		}
		op := p.s[p.i]
		p.i++
		w, ok := p.product(depth)
		if !ok {
			return 0, false
		}
		if op == '+' {
			v += w
		} else {
			v -= w
		}
	}
}

// product reads a product of powers.
func (p *evaluator) product(depth int) (float64, bool) {
	v, ok := p.power(depth)
	if !ok {
		return 0, false
	}
	for {
		p.space()
		if p.i >= len(p.s) || p.s[p.i] != '*' && p.s[p.i] != '/' {
			return v, true
		}
		op := p.s[p.i]
		p.i++
		w, ok := p.power(depth)
		if !ok {
			return 0, false
		}
		if op == '*' {
			v *= w
		} else {
			if w == 0 {
				return 0, false
			}
			v /= w
		}
	}
}

// power reads a factor, raised to the power of another factor with ^.
func (p *evaluator) power(depth int) (float64, bool) {
	v, ok := p.factor(depth)
	if !ok {
		return 0, false
	}
	p.space()
	if p.i < len(p.s) && p.s[p.i] == '^' {
		p.i++
		w, ok := p.factor(depth + 1)
		if !ok {
			return 0, false
		}
		v = math.Pow(v, w)
	}
	return v, true
}

// factor reads a number with its unit, a reference to a cell, a call of a
// function, a sign or an expression in parentheses.
func (p *evaluator) factor(depth int) (float64, bool) {
	p.space()
	if p.i >= len(p.s) || depth > maxEvalDepth {
		return 0, false
	}
	switch c := p.s[p.i]; {
	case c == '-' || c == '+':
		p.i++
		v, ok := p.factor(depth + 1)
		if c == '-' {
			v = -v
		}
		return v, ok
	case c == '(':
		p.i++
		v, ok := p.compare(depth + 1)
		p.space()
		if !ok || p.i >= len(p.s) || p.s[p.i] != ')' {
			return 0, false
		}
		p.i++
		return v, true
	case c >= '0' && c <= '9' || c == '.':
		return p.number()
	}
	start := p.i
	for p.i < len(p.s) && isNameByte(p.s[p.i]) {
		p.i++
	}
	name := p.s[start:p.i]
	p.space()
	if name == "" {
		return 0, false
	}
	if p.i < len(p.s) && p.s[p.i] == '(' {
		return p.call(strings.ToUpper(name), depth)
	}
	switch strings.ToUpper(name) {
	case "TRUE":
		return 1, true
	case "FALSE":
		return 0, true
	case "PI":
		return math.Pi, true
	}
	return p.lookup(name)
}

// number reads a number, and its unit when it has one, such as "2 in" or
// "1.5DL". A length takes inches, the unit of the values of the cells, and
// an angle takes radians.
func (p *evaluator) number() (float64, bool) {
	start := p.i
	for p.i < len(p.s) && (p.s[p.i] >= '0' && p.s[p.i] <= '9' || p.s[p.i] == '.' ||
		(p.s[p.i] == 'E' || p.s[p.i] == 'e') && p.i+1 < len(p.s) && (p.s[p.i+1] >= '0' && p.s[p.i+1] <= '9' || p.s[p.i+1] == '-' || p.s[p.i+1] == '+') ||
		(p.s[p.i] == '-' || p.s[p.i] == '+') && p.i > start && (p.s[p.i-1] == 'E' || p.s[p.i-1] == 'e')) {
		p.i++
	}
	v, err := strconv.ParseFloat(p.s[start:p.i], 64)
	if err != nil {
		return 0, false
	}
	j := p.i
	for j < len(p.s) && p.s[j] == ' ' {
		j++
	}
	k := j
	for k < len(p.s) && (p.s[k] >= 'a' && p.s[k] <= 'z' || p.s[k] >= 'A' && p.s[k] <= 'Z' || p.s[k] == '%' || p.s[k] == '"') {
		k++
	}
	if k == j {
		return v, true
	}
	scale, ok := units[strings.ToUpper(p.s[j:k])]
	if !ok {
		return 0, false
	}
	p.i = k
	return v * scale, true
}

// units maps the units of the numbers of a formula to the factor that
// gives inches for a length, or radians for an angle. The units of the
// drawing (DL, DP, DT) and of the angles (DA) are the units of the values.
var units = map[string]float64{
	"IN": 1, "INCH": 1, "INCHES": 1, `"`: 1, "FT": 12, "FEET": 12,
	"MM": 1 / 25.4, "CM": 1 / 2.54, "M": 1 / 0.0254,
	"PT": 1.0 / 72, "P": 1.0 / 6,
	"DL": 1, "DP": 1, "DT": 1, "DA": 1,
	"DEG": math.Pi / 180, "RAD": 1,
	"%": 0.01,
}

// evalFuncs are the functions of the ShapeSheet that eval computes, by the
// number of their arguments: -1 for any number.
var evalFuncs = map[string]int{
	"IF": 3, "AND": -1, "OR": -1, "NOT": 1, "MIN": -1, "MAX": -1,
	"ABS": 1, "SQRT": 1, "INT": 1, "TRUNC": 1, "ROUND": 2, "SIGN": 1,
	"GUARD": 1, "SIN": 1, "COS": 1, "TAN": 1, "ATAN2": 2, "MODULUS": 2,
}

// call reads the arguments of the function name, and computes it. IF
// computes only the argument that it gives.
func (p *evaluator) call(name string, depth int) (float64, bool) {
	n, ok := evalFuncs[name]
	if !ok || depth >= maxEvalDepth {
		return 0, false
	}
	p.i++ // (
	if name == "IF" {
		return p.callIf(depth)
	}
	var args []float64
	p.space()
	if p.i < len(p.s) && p.s[p.i] == ')' {
		p.i++
	} else {
		for {
			v, ok := p.compare(depth + 1)
			if !ok || len(args) >= 64 {
				return 0, false
			}
			args = append(args, v)
			p.space()
			if p.i >= len(p.s) {
				return 0, false
			}
			if p.s[p.i] == ')' {
				p.i++
				break
			}
			if p.s[p.i] != ',' {
				return 0, false
			}
			p.i++
		}
	}
	if n >= 0 && len(args) != n || n < 0 && len(args) == 0 {
		return 0, false
	}
	switch name {
	case "AND":
		r := true
		for _, a := range args {
			r = r && a != 0
		}
		return boolNum(r), true
	case "OR":
		r := false
		for _, a := range args {
			r = r || a != 0
		}
		return boolNum(r), true
	case "NOT":
		return boolNum(args[0] == 0), true
	case "MIN":
		v := args[0]
		for _, a := range args[1:] {
			v = math.Min(v, a)
		}
		return v, true
	case "MAX":
		v := args[0]
		for _, a := range args[1:] {
			v = math.Max(v, a)
		}
		return v, true
	case "ABS":
		return math.Abs(args[0]), true
	case "SQRT":
		return math.Sqrt(args[0]), args[0] >= 0
	case "INT":
		return math.Floor(args[0]), true
	case "TRUNC":
		return math.Trunc(args[0]), true
	case "ROUND":
		k := math.Pow(10, math.Round(args[1]))
		return math.Round(args[0]*k) / k, k != 0 && !math.IsInf(k, 0)
	case "SIGN":
		return boolNum(args[0] > 0) - boolNum(args[0] < 0), true
	case "GUARD":
		return args[0], true
	case "SIN":
		return math.Sin(args[0]), true
	case "COS":
		return math.Cos(args[0]), true
	case "TAN":
		return math.Tan(args[0]), true
	case "ATAN2":
		return math.Atan2(args[0], args[1]), true
	case "MODULUS":
		if args[1] == 0 {
			return 0, false
		}
		return args[0] - args[1]*math.Floor(args[0]/args[1]), true
	}
	return 0, false
}

// callIf reads the condition and the two values of IF, after its
// parenthesis, and computes the value that the condition gives. It reads
// the other value without its references.
func (p *evaluator) callIf(depth int) (float64, bool) {
	cond, ok := p.compare(depth + 1)
	if !ok || !p.comma() {
		return 0, false
	}
	var v float64
	for i := range 2 {
		if (i == 0) == (cond != 0) {
			if v, ok = p.compare(depth + 1); !ok {
				return 0, false
			}
		} else if !p.skip() {
			return 0, false
		}
		if i == 0 && !p.comma() {
			return 0, false
		}
	}
	p.space()
	if p.i >= len(p.s) || p.s[p.i] != ')' {
		return 0, false
	}
	p.i++
	return v, true
}

// comma reads a comma between two arguments.
func (p *evaluator) comma() bool {
	p.space()
	if p.i >= len(p.s) || p.s[p.i] != ',' {
		return false
	}
	p.i++
	return true
}

// skip moves past an argument that eval does not compute: to the next
// comma or closing parenthesis outside parentheses and strings.
func (p *evaluator) skip() bool {
	level := 0
	start := p.i
	for ; p.i < len(p.s); p.i++ {
		switch p.s[p.i] {
		case '"':
			j := strings.IndexByte(p.s[p.i+1:], '"')
			if j < 0 {
				return false
			}
			p.i += j + 1
		case '(':
			level++
		case ')':
			if level == 0 {
				return p.i > start
			}
			level--
		case ',':
			if level == 0 {
				return p.i > start
			}
		}
	}
	return false
}

// isNameByte reports whether c can be part of a name of a formula, such
// as "Sheet.5!User.Base".
func isNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '!'
}
