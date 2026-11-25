package formula

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xo/okage/texmath"
)

// Element is an element of MathML, as a format reads it: its local name,
// its attributes, its text and its child elements. The text of a token
// element, such as mi or mo, is its characters.
type Element struct {
	Name     string
	Attrs    map[string]string
	Text     string
	Children []*Element
}

// Attr returns the attribute name of e, or "".
func (e *Element) Attr(name string) string {
	return e.Attrs[name]
}

// MathML returns the tree of a formula of Presentation MathML, the math
// element e or an element in it, and whether its attribute display is
// block. The elements of Content MathML and the annotations draw nothing.
func MathML(e *Element) (texmath.List, bool) {
	r := &mathmlReader{}
	return r.row(e.Children), e.Attr("display") == "block"
}

// mathmlReader reads the elements of a formula of MathML. An element
// that nests deeper than maxNesting draws nothing.
type mathmlReader struct {
	depth int
}

// row returns the nodes of the elements of an mrow, or of an element that
// holds an mrow, such as msqrt. A first and a last mo that are fences, such
// as parentheses, grow to the height of the elements between them, as
// MathML stretches them.
func (r *mathmlReader) row(children []*Element) texmath.List {
	if n := len(children); n >= 2 && isFence(children[0], openOps) && isFence(children[n-1], closeOps) {
		open, _ := utf8.DecodeRuneInString(strings.TrimSpace(children[0].Text))
		closing, _ := utf8.DecodeRuneInString(strings.TrimSpace(children[n-1].Text))
		body := r.list(children[1 : n-1])
		return texmath.List{texmath.Atom{Class: texmath.Ord, Body: texmath.Delimited{Open: open, Close: closing, Body: body}}}
	}
	return r.list(children)
}

// list returns the nodes of the elements.
func (r *mathmlReader) list(children []*Element) texmath.List {
	var l texmath.List
	for _, c := range children {
		l = r.element(l, c)
	}
	return l
}

// isFence reports whether e is an mo of one character of chars that does
// not say that it does not stretch.
func isFence(e *Element, chars string) bool {
	if e.Name != "mo" || e.Attr("stretchy") == "false" {
		return false
	}
	s := strings.TrimSpace(e.Text)
	r, n := utf8.DecodeRuneInString(s)
	return n == len(s) && n > 0 && strings.ContainsRune(chars, r) && r != '!' && r != '?'
}

// node returns the node of the element e, the only node of its list or the
// list.
func (r *mathmlReader) node(e *Element) texmath.Node {
	if e == nil {
		return texmath.List{}
	}
	return single(r.element(nil, e))
}

// arg returns the node of the child i of e, or the empty list.
func (r *mathmlReader) arg(e *Element, i int) texmath.Node {
	if i >= len(e.Children) {
		return texmath.List{}
	}
	return r.node(e.Children[i])
}

// element appends the nodes of the element e to l.
func (r *mathmlReader) element(l texmath.List, e *Element) texmath.List {
	if r.depth >= maxNesting {
		return l
	}
	r.depth++
	defer func() { r.depth-- }()
	switch e.Name {
	case "mi":
		return r.identifier(l, e)
	case "mn":
		return Atoms(l, e.Text, mathVariant(e.Attr("mathvariant"), texmath.Upright))
	case "mo":
		return r.operator(l, e)
	case "mtext", "ms":
		if t := strings.TrimSpace(e.Text); t != "" {
			return append(l, texmath.TextRun{Text: collapseSpace(e.Text)})
		}
		return l
	case "mspace":
		if w, ok := length(e.Attr("width")); ok {
			return append(l, texmath.Space{Width: w})
		}
		return l
	case "mrow", "mstyle", "mpadded", "merror", "math", "menclose":
		node := single(r.row(e.Children))
		if e.Name == "mstyle" {
			switch e.Attr("displaystyle") {
			case "true":
				node = texmath.StyleChange{Style: texmath.Display, Body: node}
			case "false":
				node = texmath.StyleChange{Style: texmath.Text, Body: node}
			}
		}
		if e.Name == "menclose" && strings.Contains(e.Attr("notation"), "box") {
			node = texmath.Boxed{Body: node}
		}
		if list, ok := node.(texmath.List); ok {
			return append(l, list...)
		}
		return append(l, node)
	case "mphantom":
		return append(l, texmath.Phantom{Body: single(r.row(e.Children)), Horizontal: true, Vertical: true})
	case "semantics", "maction":
		if len(e.Children) > 0 {
			return r.element(l, e.Children[0])
		}
		return l
	case "mfrac":
		num, den := r.arg(e, 0), r.arg(e, 1)
		if e.Attr("bevelled") == "true" {
			return append(l, num, texmath.Atom{Class: texmath.Ord, Text: "/", Variant: texmath.Upright}, den)
		}
		thick := strings.TrimSpace(e.Attr("linethickness"))
		return append(l, texmath.Fraction{Num: num, Den: den, NoRule: thick == "0" || strings.HasPrefix(thick, "0px") || thick == "0pt"})
	case "msqrt":
		return append(l, texmath.Radical{Body: single(r.row(e.Children))})
	case "mroot":
		return append(l, texmath.Radical{Body: r.arg(e, 0), Degree: r.arg(e, 1)})
	case "msub":
		return append(l, texmath.Scripts{Base: r.arg(e, 0), Sub: r.arg(e, 1)})
	case "msup":
		return append(l, texmath.Scripts{Base: r.arg(e, 0), Sup: r.arg(e, 1)})
	case "msubsup":
		return append(l, texmath.Scripts{Base: r.arg(e, 0), Sub: r.arg(e, 1), Sup: r.arg(e, 2)})
	case "munder", "mover", "munderover":
		return append(l, r.underOver(e))
	case "mmultiscripts":
		return r.multiscripts(l, e)
	case "mtable":
		return append(l, r.table(e))
	case "mfenced":
		return append(l, r.fenced(e))
	}
	// annotation, annotation-xml, mglyph, mprescripts, none and an unknown
	// element draw nothing.
	return l
}

// identifier appends the atoms of an mi element. An mi of one character
// is italic, and an mi of more characters, such as sin, is upright. An mi
// of letters only is then an operator, as TeX sets \sin, so that a thin
// space follows it.
func (r *mathmlReader) identifier(l texmath.List, e *Element) texmath.List {
	s := strings.TrimSpace(e.Text)
	if utf8.RuneCountInString(s) <= 1 {
		return Atoms(l, s, mathVariant(e.Attr("mathvariant"), texmath.Italic))
	}
	body := Atoms(nil, s, mathVariant(e.Attr("mathvariant"), texmath.Upright))
	if !isWord(s) {
		return append(l, body...)
	}
	return append(l, texmath.Atom{Class: texmath.Op, Body: body, Limits: texmath.LimitsSide})
}

// isWord reports whether s holds letters only.
func isWord(s string) bool {
	for _, c := range s {
		if !unicode.IsLetter(c) {
			return false
		}
	}
	return s != ""
}

// operator appends the atom of an mo element. Its class comes from its
// character, or from a class MathClass-rel and its kin that TeX4ht writes.
// An invisible operator, such as the function application U+2061, draws
// nothing. A word, such as lim, is an operator. A large operator, such as
// the sum, grows in display style.
func (r *mathmlReader) operator(l texmath.List, e *Element) texmath.List {
	s := strings.TrimSpace(e.Text)
	s = strings.Map(func(c rune) rune {
		if c >= 0x2061 && c <= 0x2064 {
			return -1
		}
		return c
	}, s)
	if s == "" {
		return l
	}
	if isWord(s) {
		return append(l, texmath.Atom{Class: texmath.Op, Body: Atoms(nil, s, texmath.Upright), Limits: texmath.LimitsAuto})
	}
	start := len(l)
	l = Atoms(l, s, texmath.Upright)
	if len(l) != start+1 {
		return l
	}
	a, ok := l[start].(texmath.Atom)
	if !ok {
		return l
	}
	if c, ok := mathClasses[e.Attr("class")]; ok && !a.Large {
		a.Class = c
	}
	switch e.Attr("largeop") {
	case "true":
		a.Class, a.Large = texmath.Op, true
	case "false":
		a.Large = false
	}
	if a.Large && e.Attr("movablelimits") == "false" {
		a.Limits = texmath.LimitsAbove
	}
	l[start] = a
	return l
}

// mathClasses maps a class of TeX4ht on an mo element to the class of TeX.
var mathClasses = map[string]texmath.Class{
	"MathClass-ord":   texmath.Ord,
	"MathClass-op":    texmath.Op,
	"MathClass-bin":   texmath.Bin,
	"MathClass-rel":   texmath.Rel,
	"MathClass-open":  texmath.Open,
	"MathClass-close": texmath.Close,
	"MathClass-punc":  texmath.Punct,
	"MathClass-inner": texmath.Inner,
}

// underOver returns the node of an munder, an mover or an munderover
// element. A large operator, such as a sum, takes the scripts as its
// limits. An accent, such as a hat, a bar or an arrow, takes the form of
// TeX. Any other script is set over or under the base.
func (r *mathmlReader) underOver(e *Element) texmath.Node {
	base := r.arg(e, 0)
	var under, over *Element
	switch e.Name {
	case "munder":
		under = child(e, 1)
	case "mover":
		over = child(e, 1)
	case "munderover":
		under, over = child(e, 1), child(e, 2)
	}
	if a, ok := base.(texmath.Atom); ok && a.Class == texmath.Op {
		if a.Limits == texmath.LimitsSide {
			a.Limits = texmath.LimitsAbove
		}
		return texmath.Scripts{Base: a, Sub: r.optNode(under), Sup: r.optNode(over)}
	}
	if under == nil && over != nil {
		switch c := accentChar(over); c {
		case 0:
		case '¯', '‾', '_':
			return texmath.Bar{Base: base}
		case '→':
			return texmath.OverArrow{Base: base, Right: true}
		case '←':
			return texmath.OverArrow{Base: base, Left: true}
		case '⏞':
			return texmath.Brace{Base: base}
		default:
			_, one := base.(texmath.Atom)
			return texmath.Accent{Base: base, Accent: c, Stretch: !one}
		}
	}
	if over == nil && under != nil {
		switch accentChar(under) {
		case '¯', '‾', '_':
			return texmath.Bar{Base: base, Under: true}
		case '⏟':
			return texmath.Brace{Base: base, Under: true}
		}
	}
	return texmath.OverUnder{Base: base, Over: r.optNode(over), Under: r.optNode(under), Class: texmath.Ord}
}

// optNode returns the node of e, or nil for no element.
func (r *mathmlReader) optNode(e *Element) texmath.Node {
	if e == nil {
		return nil
	}
	return r.node(e)
}

// child returns the child i of e, or nil.
func child(e *Element, i int) *Element {
	if i < len(e.Children) {
		return e.Children[i]
	}
	return nil
}

// accentChar returns the spacing form of the accent that the element e
// holds, an mo of one character, or 0 when e is not an accent.
func accentChar(e *Element) rune {
	if e.Name != "mo" {
		return 0
	}
	s := strings.TrimSpace(e.Text)
	c, n := utf8.DecodeRuneInString(s)
	if n != len(s) || n == 0 {
		return 0
	}
	if sp, ok := spacingAccents[c]; ok {
		return sp
	}
	switch c {
	case '^':
		return 'ˆ'
	case '~':
		return '˜'
	case '.':
		return '˙'
	case '¯', '‾', '_', '→', '←', '⏞', '⏟', 'ˆ', '˜', '˙', '¨', 'ˇ', '˘', '´', '`', '˚', '⃗':
		return c
	}
	return 0
}

// multiscripts appends the nodes of an mmultiscripts element: its base,
// with the pairs of scripts after it, and the pairs after mprescripts
// before it. The first pair of each side is kept.
func (r *mathmlReader) multiscripts(l texmath.List, e *Element) texmath.List {
	if len(e.Children) == 0 {
		return l
	}
	base := r.node(e.Children[0])
	var post, pre []*Element
	cur := &post
	for _, c := range e.Children[1:] {
		if c.Name == "mprescripts" {
			cur = &pre
			continue
		}
		*cur = append(*cur, c)
	}
	script := func(els []*Element, i int) texmath.Node {
		if i >= len(els) || els[i].Name == "none" {
			return nil
		}
		return r.node(els[i])
	}
	if sub, sup := script(pre, 0), script(pre, 1); sub != nil || sup != nil {
		l = append(l, texmath.Scripts{Base: texmath.List{}, Sub: sub, Sup: sup})
	}
	sub, sup := script(post, 0), script(post, 1)
	if sub == nil && sup == nil {
		return append(l, base)
	}
	return append(l, texmath.Scripts{Base: base, Sub: sub, Sup: sup})
}

// table returns the node of an mtable element: a grid of its mtr rows of
// mtd cells, with the alignment of its attribute columnalign, or of the mtd
// cells of its first row. TeX4ht names the environment of LaTeX in the
// class of the mtable, such as align-star for align*. The rows of align
// and gather take the cells and the space of aligned and gathered of
// amsmath.
func (r *mathmlReader) table(e *Element) texmath.Node {
	a := texmath.Array{Kind: texmath.ArrayMatrix}
	switch class := e.Attr("class"); {
	case strings.HasPrefix(class, "align") || strings.HasPrefix(class, "flalign"):
		a.Kind = texmath.ArrayAligned
	case strings.HasPrefix(class, "gather"):
		a.Kind = texmath.ArrayGathered
	}
	for _, v := range strings.Fields(e.Attr("columnalign")) {
		a.Align = append(a.Align, alignOf(v))
	}
	if a.Kind == texmath.ArrayMatrix && len(a.Align) == 0 && len(e.Children) > 0 {
		set := false
		for _, td := range e.Children[0].Children {
			a.Align = append(a.Align, alignOf(td.Attr("columnalign")))
			set = set || td.Attr("columnalign") != ""
		}
		if !set {
			a.Align = nil
		}
	}
	for _, tr := range e.Children {
		if tr.Name != "mtr" && tr.Name != "mlabeledtr" {
			continue
		}
		cells := tr.Children
		if tr.Name == "mlabeledtr" && len(cells) > 0 {
			cells = cells[1:]
		}
		var row []texmath.Node
		for _, td := range cells {
			if td.Name == "mtd" {
				row = append(row, single(r.row(td.Children)))
			} else {
				row = append(row, r.node(td))
			}
		}
		a.Rows = append(a.Rows, row)
	}
	return a
}

// alignOf returns the alignment of a value of columnalign.
func alignOf(v string) texmath.Align {
	switch v {
	case "left":
		return texmath.AlignLeft
	case "right":
		return texmath.AlignRight
	}
	return texmath.AlignCenter
}

// fenced returns the node of an mfenced element: its children between the
// delimiters of its attributes open and close, parentheses by default,
// separated by the characters of separators, commas by default.
func (r *mathmlReader) fenced(e *Element) texmath.Node {
	open, closing := fenceChar(e, "open", '('), fenceChar(e, "close", ')')
	seps := []rune(strings.Join(strings.Fields(attrDefault(e, "separators", ",")), ""))
	var body texmath.List
	for i, c := range e.Children {
		if i > 0 && len(seps) > 0 {
			body = append(body, symbol(seps[min(i-1, len(seps)-1)]))
		}
		body = append(body, r.node(c))
	}
	return texmath.Atom{Class: texmath.Ord, Body: texmath.Delimited{Open: open, Close: closing, Body: body}}
}

// fenceChar returns the delimiter of the attribute name of an mfenced: def
// when it is missing, and 0 when it is empty.
func fenceChar(e *Element, name string, def rune) rune {
	v, ok := e.Attrs[name]
	if !ok {
		return def
	}
	c, _ := utf8.DecodeRuneInString(strings.TrimSpace(v))
	if c == utf8.RuneError {
		return 0
	}
	return c
}

// attrDefault returns the attribute name of e, or def when e has none.
func attrDefault(e *Element, name, def string) string {
	if v, ok := e.Attrs[name]; ok {
		return v
	}
	return def
}

// mathVariant returns the math alphabet of a value of mathvariant, or def
// for none.
func mathVariant(v string, def texmath.Variant) texmath.Variant {
	switch v {
	case "normal":
		return texmath.Upright
	case "bold":
		return texmath.Bold
	case "italic":
		return texmath.Italic
	case "bold-italic":
		return texmath.BoldItalic
	case "double-struck":
		return texmath.DoubleStruck
	case "script", "bold-script":
		return texmath.Calligraphic
	case "fraktur", "bold-fraktur":
		return texmath.Fraktur
	case "sans-serif":
		return texmath.SansSerif
	case "bold-sans-serif":
		return texmath.SansSerifBold
	case "sans-serif-italic":
		return texmath.SansSerifItalic
	case "sans-serif-bold-italic":
		return texmath.SansSerifBoldItalic
	case "monospace":
		return texmath.Monospace
	}
	return def
}

// length returns a length of MathML, such as 0.5em or 1ex, in a unit of
// texmath. A named space, such as thinmathspace, is a number of eighteenths
// of an em. A unit that texmath does not have is converted to points.
func length(s string) (texmath.Length, bool) {
	s = strings.TrimSpace(s)
	if mu, ok := namedSpaces[s]; ok {
		return texmath.Length{Value: mu, Unit: texmath.Mu}, true
	}
	i := strings.IndexFunc(s, func(c rune) bool { return (c < '0' || c > '9') && c != '.' && c != '-' && c != '+' })
	if i < 0 {
		i = len(s)
	}
	v, err := strconv.ParseFloat(s[:i], 64)
	if err != nil || v != v || v > 1000 || v < -1000 {
		return texmath.Length{}, false
	}
	switch strings.TrimSpace(s[i:]) {
	case "em":
		return texmath.Length{Value: v, Unit: texmath.Em}, true
	case "ex":
		return texmath.Length{Value: v, Unit: texmath.Ex}, true
	case "mu":
		return texmath.Length{Value: v, Unit: texmath.Mu}, true
	case "pt":
		return texmath.Length{Value: v, Unit: texmath.Pt}, true
	case "px":
		return texmath.Length{Value: v * 0.75, Unit: texmath.Pt}, true
	case "in":
		return texmath.Length{Value: v * 72, Unit: texmath.Pt}, true
	case "cm":
		return texmath.Length{Value: v * 72 / 2.54, Unit: texmath.Pt}, true
	case "mm":
		return texmath.Length{Value: v * 72 / 25.4, Unit: texmath.Pt}, true
	case "pc":
		return texmath.Length{Value: v * 12, Unit: texmath.Pt}, true
	}
	return texmath.Length{}, false
}

// namedSpaces maps a named space of MathML to its width in eighteenths of
// an em.
var namedSpaces = map[string]float64{
	"veryverythinmathspace": 1, "verythinmathspace": 2, "thinmathspace": 3,
	"mediummathspace": 4, "thickmathspace": 5, "verythickmathspace": 6,
	"veryverythickmathspace": 7,
}

// collapseSpace returns s with each run of white space as one space, and
// without the white space at its ends.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
