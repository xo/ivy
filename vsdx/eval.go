package vsdx

import (
	"math"
	"strconv"
	"strings"
)

// eval computes a trivial formula of a cell: numbers, the cells of the
// shape by their names, the operators +, -, * and /, and parentheses, such
// as "Width*0.5" or "TxtHeight*0.5". lookup returns the value of a cell of
// the shape. eval returns false for any other formula, and the package then
// draws with the value of the cell. It does not evaluate the formulas of
// the ShapeSheet in general (D30).
func eval(f string, lookup func(name string) (float64, bool)) (float64, bool) {
	p := &evaluator{s: f, lookup: lookup}
	v, ok := p.sum(0)
	p.space()
	if !ok || p.i != len(p.s) || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// maxEvalDepth is the depth of the parentheses that eval follows.
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

// product reads a product of factors.
func (p *evaluator) product(depth int) (float64, bool) {
	v, ok := p.factor(depth)
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
		w, ok := p.factor(depth)
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

// factor reads a number, a name, a sign or an expression in parentheses.
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
		v, ok := p.sum(depth + 1)
		p.space()
		if !ok || p.i >= len(p.s) || p.s[p.i] != ')' {
			return 0, false
		}
		p.i++
		return v, true
	case c >= '0' && c <= '9' || c == '.':
		start := p.i
		for p.i < len(p.s) && (p.s[p.i] >= '0' && p.s[p.i] <= '9' || p.s[p.i] == '.' || p.s[p.i] == 'E' || p.s[p.i] == 'e' ||
			(p.s[p.i] == '-' || p.s[p.i] == '+') && p.i > start && (p.s[p.i-1] == 'E' || p.s[p.i-1] == 'e')) {
			p.i++
		}
		v, err := strconv.ParseFloat(p.s[start:p.i], 64)
		return v, err == nil
	}
	start := p.i
	for p.i < len(p.s) && isNameByte(p.s[p.i]) {
		p.i++
	}
	name := p.s[start:p.i]
	p.space()
	if name == "" || strings.ContainsAny(name, ".!") || p.i < len(p.s) && p.s[p.i] == '(' {
		// a function, or a cell of another shape or of a section
		return 0, false
	}
	return p.lookup(name)
}

// isNameByte reports whether c can be part of a name of a formula.
func isNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '!'
}
