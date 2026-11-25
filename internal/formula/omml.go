package formula

import (
	"strings"
	"unicode/utf8"

	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/texmath"
)

// OMML returns the tree of an equation of Office Math, the markup of ECMA-376
// Part 1, section 22.1: an m:oMath element or an element in it. The elements
// are read by their local names. An element that nests deeper than maxDepth
// is left out, and OMML then returns false.
func OMML(n roxmltree.Node, maxDepth int) (texmath.List, bool) {
	r := &ommlReader{maxDepth: min(maxDepth, maxNesting)}
	l := r.list(n)
	return l, !r.deep
}

// maxNesting is the depth of the elements of an equation that OMML reads.
// Each element can give two levels of the tree, and the layout allows
// maxDepth levels for each of the two.
const maxNesting = maxDepth / 2

// ommlReader reads the elements of an equation of Office Math.
type ommlReader struct {
	depth, maxDepth int
	deep            bool
}

// list returns the nodes of the children of n.
func (r *ommlReader) list(n roxmltree.Node) texmath.List {
	if r.depth >= r.maxDepth {
		r.deep = true
		return nil
	}
	r.depth++
	defer func() { r.depth-- }()
	var l texmath.List
	for e := range n.Children() {
		if e.IsElement() {
			l = r.element(l, e)
		}
	}
	return l
}

// arg returns the node of the child element of n with the local name name,
// such as m:e. An argument of one node is that node, so that a nucleus of
// one character takes the italic correction and the kerning of its glyph.
// A missing argument is the empty list.
func (r *ommlReader) arg(n roxmltree.Node, name string) texmath.Node {
	e, ok := ommlChild(n, name)
	if !ok {
		return texmath.List{}
	}
	return single(r.list(e))
}

// optArg returns the node of the child element of n with the local name
// name, or nil when n has no such child or the child is empty.
func (r *ommlReader) optArg(n roxmltree.Node, name string) texmath.Node {
	e, ok := ommlChild(n, name)
	if !ok {
		return nil
	}
	l := r.list(e)
	if len(l) == 0 {
		return nil
	}
	return single(l)
}

// single returns the only node of l, or l.
func single(l texmath.List) texmath.Node {
	if len(l) == 1 {
		return l[0]
	}
	return l
}

// element appends the nodes of the element e to l.
func (r *ommlReader) element(l texmath.List, e roxmltree.Node) texmath.List {
	switch e.TagName().Name() {
	case "r":
		return r.run(l, e)
	case "oMath", "e", "box", "ins", "moveTo", "smartTag", "customXml", "sdtContent":
		return append(l, r.list(e)...)
	case "acc":
		return append(l, r.accent(e))
	case "bar":
		under := prop(e, "barPr", "pos") != "top"
		return append(l, texmath.Bar{Base: r.arg(e, "e"), Under: under})
	case "borderBox":
		return append(l, texmath.Boxed{Body: r.arg(e, "e")})
	case "d":
		return append(l, r.delimiter(e))
	case "eqArr":
		return append(l, r.eqArray(e))
	case "f":
		return r.fraction(l, e)
	case "func":
		name := texmath.Atom{Class: texmath.Op, Body: r.arg(e, "fName"), Limits: texmath.LimitsSide}
		return append(l, name, r.arg(e, "e"))
	case "groupChr":
		return append(l, r.groupChar(e))
	case "limLow", "limUpp":
		base := texmath.Atom{Class: texmath.Op, Body: r.arg(e, "e"), Limits: texmath.LimitsAbove}
		s := texmath.Scripts{Base: base}
		if e.TagName().Name() == "limLow" {
			s.Sub = r.arg(e, "lim")
		} else {
			s.Sup = r.arg(e, "lim")
		}
		return append(l, s)
	case "m":
		return append(l, r.matrix(e))
	case "nary":
		return r.nary(l, e)
	case "phant":
		if on(e, "phantPr", "show", true) {
			return append(l, r.arg(e, "e"))
		}
		return append(l, texmath.Phantom{Body: r.arg(e, "e"), Horizontal: !on(e, "phantPr", "zeroWid", false), Vertical: true})
	case "rad":
		rad := texmath.Radical{Body: r.arg(e, "e")}
		if !on(e, "radPr", "degHide", false) {
			rad.Degree = r.optArg(e, "deg")
		}
		return append(l, rad)
	case "sPre":
		return append(l, texmath.Scripts{Base: texmath.List{}, Sub: r.optArg(e, "sub"), Sup: r.optArg(e, "sup")}, r.arg(e, "e"))
	case "sSub":
		return append(l, texmath.Scripts{Base: r.arg(e, "e"), Sub: r.arg(e, "sub")})
	case "sSup":
		return append(l, texmath.Scripts{Base: r.arg(e, "e"), Sup: r.arg(e, "sup")})
	case "sSubSup":
		return append(l, texmath.Scripts{Base: r.arg(e, "e"), Sub: r.arg(e, "sub"), Sup: r.arg(e, "sup")})
	}
	// The properties, such as m:ctrlPr, and the deleted text draw nothing.
	return l
}

// run appends the atoms of an m:r element. Its m:rPr gives the math alphabet
// of its letters: m:scr the script, such as double-struck, and m:sty the
// style, italic by default. Its m:nor sets it as normal text, in the
// upright font of the text, with its spaces.
func (r *ommlReader) run(l texmath.List, e roxmltree.Node) texmath.List {
	var text strings.Builder
	for c := range e.Children() {
		if c.IsElement() && c.TagName().Name() == "t" {
			s, _ := c.Text()
			text.WriteString(s)
		}
	}
	scr, sty, normal := "", "", false
	if pr, ok := ommlChild(e, "rPr", "m"); ok {
		scr = prop(pr, "", "scr")
		sty = prop(pr, "", "sty")
		normal = on(pr, "", "nor", false)
	}
	if normal {
		var bold, italic bool
		if w, ok := ommlChild(e, "rPr", "w"); ok {
			bold = on(w, "", "b", false)
			italic = on(w, "", "i", false)
		}
		if text.Len() == 0 {
			return l
		}
		return append(l, texmath.TextRun{Text: text.String(), Bold: bold, Italic: italic})
	}
	// Word sets the ellipsis on the math axis.
	return Atoms(l, strings.ReplaceAll(text.String(), "…", "⋯"), variant(scr, sty))
}

// variant returns the math alphabet of the script scr and the style sty of
// a run of Office Math.
func variant(scr, sty string) texmath.Variant {
	switch scr {
	case "script":
		return texmath.Calligraphic
	case "fraktur":
		return texmath.Fraktur
	case "double-struck":
		return texmath.DoubleStruck
	case "monospace":
		return texmath.Monospace
	case "sans-serif":
		switch sty {
		case "p":
			return texmath.SansSerif
		case "b":
			return texmath.SansSerifBold
		case "bi":
			return texmath.SansSerifBoldItalic
		}
		return texmath.SansSerifItalic
	}
	switch sty {
	case "p":
		return texmath.Upright
	case "b":
		return texmath.Bold
	case "bi":
		return texmath.BoldItalic
	}
	return texmath.Italic
}

// accent returns the node of an m:acc element. Its m:chr is a combining
// mark, U+0302 by default, and texmath takes the spacing form of a mark.
func (r *ommlReader) accent(e roxmltree.Node) texmath.Node {
	acc := char(e, "accPr", "chr", 0x302)
	if s, ok := spacingAccents[acc]; ok {
		acc = s
	}
	return texmath.Accent{Base: r.arg(e, "e"), Accent: acc}
}

// spacingAccents maps a combining mark to its spacing form.
var spacingAccents = map[rune]rune{
	0x300: '`', 0x301: '´', 0x302: 'ˆ', 0x303: '˜', 0x304: '¯', 0x305: '¯',
	0x306: '˘', 0x307: '˙', 0x308: '¨', 0x30A: '˚', 0x30C: 'ˇ',
}

// delimiter returns the node of an m:d element: its m:e elements between
// two delimiters that grow, parentheses by default, and separated by its
// m:sepChr, a vertical bar by default.
func (r *ommlReader) delimiter(e roxmltree.Node) texmath.Node {
	open := char(e, "dPr", "begChr", '(')
	closing := char(e, "dPr", "endChr", ')')
	sep := char(e, "dPr", "sepChr", '|')
	grow := on(e, "dPr", "grow", true)
	var body texmath.List
	first := true
	for c := range e.Children() {
		if !c.IsElement() || c.TagName().Name() != "e" {
			continue
		}
		if !first && sep != 0 {
			body = append(body, separator(sep, grow))
		}
		first = false
		body = append(body, single(r.list(c)))
	}
	if !grow {
		var l texmath.List
		if open != 0 {
			l = append(l, symbol(open))
		}
		l = append(l, body...)
		if closing != 0 {
			l = append(l, symbol(closing))
		}
		return l
	}
	// TeX sets \left and \right as an inner atom, with a thin space
	// next to a letter. Word sets the delimiters with no space, as an
	// ordinary atom.
	return texmath.Atom{Class: texmath.Ord, Body: texmath.Delimited{Open: open, Close: closing, Body: body}}
}

// separator returns the separator sep of a delimiter. A bar grows as the
// delimiters do, and a comma or another character does not.
func separator(sep rune, grow bool) texmath.Node {
	if grow && strings.ContainsRune("|‖∣∥/\\", sep) {
		return texmath.Delimiter{Rune: sep, Class: texmath.Rel, Middle: true}
	}
	return symbol(sep)
}

// eqArray returns the node of an m:eqArr element, an array of equations,
// each m:e a row. An ampersand in a run marks a point where the rows align,
// so the rows are cut into cells there, as the cells of aligned in LaTeX.
func (r *ommlReader) eqArray(e roxmltree.Node) texmath.Node {
	var rows [][]texmath.Node
	cols := 0
	for c := range e.Children() {
		if !c.IsElement() || c.TagName().Name() != "e" {
			continue
		}
		row := r.cells(c)
		cols = max(cols, len(row))
		rows = append(rows, row)
	}
	if cols <= 1 {
		return texmath.Array{Kind: texmath.ArrayGathered, Rows: rows}
	}
	return texmath.Array{Kind: texmath.ArrayAligned, Rows: rows}
}

// cells returns the cells of a row of an array of equations, cut at each
// ampersand of a run that is a child of the row.
func (r *ommlReader) cells(row roxmltree.Node) []texmath.Node {
	var cells []texmath.Node
	var cur texmath.List
	for c := range row.Children() {
		if !c.IsElement() {
			continue
		}
		if c.TagName().Name() != "r" || !hasAmpersand(c) {
			cur = r.element(cur, c)
			continue
		}
		// The run is cut at each ampersand, with the properties of the run.
		parts := strings.Split(runText(c), "&")
		v := texmath.Italic
		if pr, ok := ommlChild(c, "rPr", "m"); ok {
			v = variant(prop(pr, "", "scr"), prop(pr, "", "sty"))
		}
		for i, s := range parts {
			if i > 0 {
				cells = append(cells, cur)
				cur = nil
			}
			cur = Atoms(cur, s, v)
		}
	}
	return append(cells, cur)
}

// hasAmpersand reports whether the run r holds an ampersand.
func hasAmpersand(r roxmltree.Node) bool {
	return strings.Contains(runText(r), "&")
}

// runText returns the text of the m:t elements of the run r.
func runText(r roxmltree.Node) string {
	var b strings.Builder
	for c := range r.Children() {
		if c.IsElement() && c.TagName().Name() == "t" {
			s, _ := c.Text()
			b.WriteString(s)
		}
	}
	return b.String()
}

// fraction appends the node of an m:f element. Its m:type is bar by
// default, noBar for a stack without a rule, and lin or skw for a fraction
// on the line, with a slash.
func (r *ommlReader) fraction(l texmath.List, e roxmltree.Node) texmath.List {
	num, den := r.arg(e, "num"), r.arg(e, "den")
	switch prop(e, "fPr", "type") {
	case "lin", "skw":
		return append(l, num, texmath.Atom{Class: texmath.Ord, Text: "/", Variant: texmath.Upright}, den)
	case "noBar":
		return append(l, texmath.Fraction{Num: num, Den: den, NoRule: true})
	}
	return append(l, texmath.Fraction{Num: num, Den: den})
}

// groupChar returns the node of an m:groupChr element: a character over or
// under its m:e, a curly brace under it by default.
func (r *ommlReader) groupChar(e roxmltree.Node) texmath.Node {
	c := char(e, "groupChrPr", "chr", '⏟')
	top := prop(e, "groupChrPr", "pos") == "top"
	base := r.arg(e, "e")
	switch c {
	case 0:
		return base
	case '⏟', '⏞':
		return texmath.Brace{Base: base, Under: c == '⏟'}
	case '→', '←', '↔':
		return texmath.OverArrow{Base: base, Left: c != '→', Right: c != '←'}
	}
	mark := texmath.Atom{Class: texmath.Ord, Text: string(c), Variant: texmath.Upright}
	if top {
		return texmath.OverUnder{Base: base, Over: mark, Class: texmath.Ord}
	}
	return texmath.OverUnder{Base: base, Under: mark, Class: texmath.Ord}
}

// matrix returns the node of an m:m element: its m:mr rows of m:e cells.
// The m:mcJc of each m:mc of its properties aligns a column.
func (r *ommlReader) matrix(e roxmltree.Node) texmath.Node {
	a := texmath.Array{Kind: texmath.ArrayMatrix}
	if pr, ok := ommlChild(e, "mPr"); ok {
		if mcs, ok := ommlChild(pr, "mcs"); ok {
			for mc := range mcs.Children() {
				if !mc.IsElement() || mc.TagName().Name() != "mc" {
					continue
				}
				align := texmath.AlignCenter
				switch prop(mc, "mcPr", "mcJc") {
				case "left":
					align = texmath.AlignLeft
				case "right":
					align = texmath.AlignRight
				}
				count := 1
				if v := prop(mc, "mcPr", "count"); v != "" {
					count = min(max(atoi(v), 1), 64)
				}
				for range count {
					a.Align = append(a.Align, align)
				}
			}
		}
	}
	for mr := range e.Children() {
		if !mr.IsElement() || mr.TagName().Name() != "mr" {
			continue
		}
		var row []texmath.Node
		for c := range mr.Children() {
			if c.IsElement() && c.TagName().Name() == "e" {
				row = append(row, single(r.list(c)))
			}
		}
		a.Rows = append(a.Rows, row)
	}
	return a
}

// nary appends the nodes of an m:nary element: a large operator, its
// m:chr, an integral by default, with the limits m:sub and m:sup, and its
// m:e after it. m:limLoc undOvr sets the limits over and under the
// operator in display style, and subSup at its side. Without it, an
// integral has them at its side, and another operator over and under it,
// as Word does.
func (r *ommlReader) nary(l texmath.List, e roxmltree.Node) texmath.List {
	c := char(e, "naryPr", "chr", '∫')
	if c == 0 {
		c = '∫'
	}
	op := texmath.Atom{Class: texmath.Op, Text: string(c), Variant: texmath.Upright, Large: true, Limits: limitsOf(c)}
	switch prop(e, "naryPr", "limLoc") {
	case "undOvr":
		op.Limits = texmath.LimitsAuto
	case "subSup":
		op.Limits = texmath.LimitsSide
	}
	var sub, sup texmath.Node
	if !on(e, "naryPr", "subHide", false) {
		sub = r.optArg(e, "sub")
	}
	if !on(e, "naryPr", "supHide", false) {
		sup = r.optArg(e, "sup")
	}
	if sub == nil && sup == nil {
		l = append(l, op)
	} else {
		l = append(l, texmath.Scripts{Base: op, Sub: sub, Sup: sup})
	}
	return append(l, r.arg(e, "e"))
}

// ommlChild returns the first child element of n with the local name name.
// An optional prefix names the namespace by its usual prefix, m or w, for
// the two m:rPr and w:rPr of a run.
func ommlChild(n roxmltree.Node, name string, prefix ...string) (roxmltree.Node, bool) {
	for c := range n.Children() {
		if !c.IsElement() || c.TagName().Name() != name {
			continue
		}
		if len(prefix) > 0 && !inNamespace(c, prefix[0]) {
			continue
		}
		return c, true
	}
	return roxmltree.Node{}, false
}

// inNamespace reports whether the element e is in the namespace of
// WordprocessingML for the prefix w, or of Office Math for the prefix m.
func inNamespace(e roxmltree.Node, prefix string) bool {
	ns, _ := e.TagName().Namespace()
	if prefix == "w" {
		return strings.Contains(ns, "wordprocessingml")
	}
	return strings.Contains(ns, "math")
}

// prop returns the attribute val of the child element name of the child
// element pr of n, such as the m:chr of the m:naryPr of an m:nary. An empty
// pr reads the child element name of n. It returns "" when one is missing.
func prop(n roxmltree.Node, pr, name string) string {
	v, _ := propOK(n, pr, name)
	return v
}

// propOK returns the value of prop, and whether the element name exists.
func propOK(n roxmltree.Node, pr, name string) (string, bool) {
	if pr != "" {
		p, ok := ommlChild(n, pr)
		if !ok {
			return "", false
		}
		n = p
	}
	e, ok := ommlChild(n, name)
	if !ok {
		return "", false
	}
	return val(e), true
}

// on returns the value of a property of Office Math that is on or off,
// such as m:degHide: def when it is missing, and on when it has no value.
func on(n roxmltree.Node, pr, name string, def bool) bool {
	v, ok := propOK(n, pr, name)
	if !ok {
		return def
	}
	switch v {
	case "0", "off", "false":
		return false
	}
	return true
}

// char returns the character of a property, such as the m:begChr of the
// m:dPr of an m:d: def when the property is missing, and 0, no character,
// when its value is empty.
func char(n roxmltree.Node, pr, name string, def rune) rune {
	v, ok := propOK(n, pr, name)
	if !ok {
		return def
	}
	if v == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(v)
	return r
}

// atoi returns the number in s, or 0.
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' || n > 1<<20 {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// val returns the attribute val of the element e, such as m:val, by its
// local name, or "".
func val(e roxmltree.Node) string {
	for a := range e.Attributes() {
		if a.Name() == "val" {
			return a.Value()
		}
	}
	return ""
}
