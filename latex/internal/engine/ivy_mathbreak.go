// This file of ivy breaks a formula of the text between lines, as TeX does
// (D62).

package engine

import (
	"strings"
	"unicode/utf8"
)

// The classes of the atoms of a formula (The TeXbook, chapter 17).
const (
	atomOrd = iota
	atomOp
	atomBin
	atomRel
	atomOpen
	atomClose
	atomPunct
	atomInner
)

// mathAtom is one atom of the top level of a formula: its class, and the
// end of its nucleus and its scripts in the source.
type mathAtom struct {
	class int
	end   int
}

// mathBailCommands are the commands that stop the split of a formula. A
// style, a font or a color changes the rest of the formula, so a piece
// after it would lose it. A generalized fraction takes the whole formula,
// and an alignment or a skip reads what follows it.
var mathBailCommands = map[string]bool{
	"over": true, "atop": true, "above": true, "choose": true, "brace": true, "brack": true,
	"overwithdelims": true, "atopwithdelims": true, "abovewithdelims": true,
	"displaystyle": true, "textstyle": true, "scriptstyle": true, "scriptscriptstyle": true,
	"rm": true, "bf": true, "it": true, "sf": true, "tt": true, "cal": true, "mit": true,
	"sl": true, "em": true, "boldmath": true, "unboldmath": true, "mathversion": true,
	"color": true, "kern": true, "mkern": true, "hskip": true, "mskip": true,
	"raise": true, "lower": true, "moveleft": true, "moveright": true, "vcenter": true,
	"cr": true, "crcr": true, "eqno": true, "leqno": true, "tag": true, "label": true,
	"nonumber": true, "notag": true, "buildrel": true, "genfrac": true, "middle": true,
	"nobreak": true, "allowbreak": true, "penalty": true, "linebreak": true, "nolinebreak": true,
	"discretionary": true, "mathchoice": true, "char": true, "mathchar": true, "def": true,
	"let": true, "bgroup": true, "egroup": true, "begingroup": true, "endgroup": true,
}

// mathRelCommands are the commands of a relation.
var mathRelCommands = map[string]bool{
	"le": true, "leq": true, "ge": true, "geq": true, "ne": true, "neq": true, "lt": true, "gt": true,
	"in": true, "ni": true, "notin": true, "subset": true, "subseteq": true, "supset": true,
	"supseteq": true, "subsetneq": true, "supsetneq": true, "sqsubseteq": true, "sqsupseteq": true,
	"equiv": true, "sim": true, "simeq": true, "approx": true, "cong": true, "propto": true,
	"doteq": true, "asymp": true, "prec": true, "succ": true, "preceq": true, "succeq": true,
	"ll": true, "gg": true, "parallel": true, "perp": true, "mid": true, "models": true,
	"vdash": true, "dashv": true, "to": true, "gets": true, "mapsto": true, "longmapsto": true,
	"rightarrow": true, "leftarrow": true, "Rightarrow": true, "Leftarrow": true,
	"leftrightarrow": true, "Leftrightarrow": true, "longrightarrow": true, "longleftarrow": true,
	"Longrightarrow": true, "Longleftarrow": true, "longleftrightarrow": true,
	"Longleftrightarrow": true, "iff": true, "implies": true, "impliedby": true,
	"hookrightarrow": true, "hookleftarrow": true, "uparrow": true, "downarrow": true,
	"updownarrow": true, "nearrow": true, "searrow": true, "swarrow": true, "nwarrow": true,
	"rightharpoonup": true, "leftharpoonup": true, "rightleftharpoons": true,
	"leqslant": true, "geqslant": true, "lesssim": true, "gtrsim": true, "coloneqq": true,
	"triangleq": true, "bowtie": true, "smile": true, "frown": true,
}

// mathBinCommands are the commands of a binary operation.
var mathBinCommands = map[string]bool{
	"pm": true, "mp": true, "times": true, "div": true, "cdot": true, "ast": true, "star": true,
	"circ": true, "bullet": true, "cap": true, "cup": true, "uplus": true, "sqcap": true,
	"sqcup": true, "vee": true, "wedge": true, "lor": true, "land": true, "setminus": true,
	"wr": true, "diamond": true, "oplus": true, "ominus": true, "otimes": true, "oslash": true,
	"odot": true, "bigcirc": true, "dagger": true, "ddagger": true, "amalg": true,
	"triangleleft": true, "triangleright": true, "bigtriangleup": true, "bigtriangledown": true,
	"smallsetminus": true, "dotplus": true, "ltimes": true, "rtimes": true,
}

// mathOpCommands are the commands of a large operator.
var mathOpCommands = map[string]bool{
	"sum": true, "prod": true, "coprod": true, "int": true, "iint": true, "iiint": true,
	"oint": true, "bigcup": true, "bigcap": true, "bigsqcup": true, "bigvee": true,
	"bigwedge": true, "bigoplus": true, "bigotimes": true, "bigodot": true, "biguplus": true,
	"lim": true, "liminf": true, "limsup": true, "sup": true, "inf": true, "max": true,
	"min": true, "det": true, "gcd": true, "Pr": true, "log": true, "ln": true, "lg": true,
	"exp": true, "sin": true, "cos": true, "tan": true, "cot": true, "sec": true, "csc": true,
	"arcsin": true, "arccos": true, "arctan": true, "sinh": true, "cosh": true, "tanh": true,
	"coth": true, "arg": true, "deg": true, "dim": true, "hom": true, "ker": true,
}

// mathOpenCommands and mathCloseCommands are the delimiters of the classes
// Open and Close.
var (
	mathOpenCommands = map[string]bool{
		"{": true, "lbrace": true, "langle": true, "lfloor": true, "lceil": true, "lbrack": true,
		"lvert": true, "lVert": true, "bigl": true, "Bigl": true, "biggl": true, "Biggl": true,
	}
	mathCloseCommands = map[string]bool{
		"}": true, "rbrace": true, "rangle": true, "rfloor": true, "rceil": true, "rbrack": true,
		"rvert": true, "rVert": true, "bigr": true, "Bigr": true, "biggr": true, "Biggr": true,
	}
)

// mathSpaceCommands are the commands of a space, which are no atom.
var mathSpaceCommands = map[string]bool{
	",": true, ":": true, ";": true, "!": true, ">": true, " ": true, "quad": true, "qquad": true,
	"enspace": true, "thinspace": true, "medspace": true, "thickspace": true,
	"negthinspace": true, "negmedspace": true, "negthickspace": true, "limits": true,
	"nolimits": true, "displaylimits": true, "relax": true,
}

// mathArgCommands gives the number of the arguments of the commands whose
// arguments can be one token without braces, such as \frac12 or \hat x.
var mathArgCommands = map[string]int{
	"frac": 2, "dfrac": 2, "tfrac": 2, "cfrac": 2, "binom": 2, "dbinom": 2, "tbinom": 2,
	"textcolor": 2, "hat": 1, "check": 1, "tilde": 1, "acute": 1, "grave": 1, "dot": 1,
	"ddot": 1, "breve": 1, "bar": 1, "vec": 1, "widehat": 1, "widetilde": 1, "overline": 1,
	"underline": 1, "overbrace": 1, "underbrace": 1, "overrightarrow": 1, "overleftarrow": 1,
	"mathbf": 1, "mathrm": 1, "mathit": 1, "mathcal": 1, "mathbb": 1, "mathfrak": 1,
	"mathsf": 1, "mathtt": 1, "mathscr": 1, "mathnormal": 1, "boldsymbol": 1, "bm": 1,
	"pmb": 1, "text": 1, "textrm": 1, "textit": 1, "textbf": 1, "textsf": 1, "texttt": 1,
	"textnormal": 1, "textup": 1, "textsl": 1, "mbox": 1, "hbox": 1, "operatorname": 1,
	"phantom": 1, "hphantom": 1, "vphantom": 1, "smash": 1, "mathring": 1, "boxed": 1,
	"sqrt": 1, "big": 1, "Big": 1, "bigg": 1, "Bigg": 1, "bigm": 1, "Bigm": 1, "biggm": 1,
	"Biggm": 1, "mathord": 1, "mathop": 1, "mathbin": 1, "mathrel": 1, "mathopen": 1,
	"mathclose": 1, "mathpunct": 1, "mathinner": 1, "stackrel": 2, "overset": 2,
	"underset": 2, "bigl": 1, "Bigl": 1, "biggl": 1, "Biggl": 1, "bigr": 1, "Bigr": 1,
	"biggr": 1, "Biggr": 1,
}

// mathRelRunes and mathBinRunes are the characters of a relation and of a
// binary operation.
var (
	mathRelRunes = map[rune]bool{'=': true, '<': true, '>': true, ':': true,
		'≤': true, '≥': true, '≠': true, '∈': true, '→': true, '≈': true, '≡': true}
	mathBinRunes = map[rune]bool{'+': true, '-': true, '*': true, '±': true, '×': true, '·': true, '−': true}
)

// mathBraceEnd returns the end of the group that opens at i, or false when
// the group does not close.
func mathBraceEnd(src string, i int) (int, bool) {
	depth := 0
	for j := i; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return j + 1, true
			}
		}
	}
	return 0, false
}

// mathToken returns the end of the token at i: a control sequence, a group,
// or one character. It returns false at the end of src or on a group that
// does not close.
func mathToken(src string, i int) (int, bool) {
	i = skipMathSpaces(src, i)
	if i >= len(src) {
		return 0, false
	}
	switch src[i] {
	case '{':
		return mathBraceEnd(src, i)
	case '\\':
		_, end := mathCommand(src, i)
		return end, true
	}
	_, n := utf8.DecodeRuneInString(src[i:])
	return i + n, true
}

// mathCommand returns the name of the control sequence at i, and its end.
func mathCommand(src string, i int) (string, int) {
	j := i + 1
	for j < len(src) && (src[j] >= 'a' && src[j] <= 'z' || src[j] >= 'A' && src[j] <= 'Z') {
		j++
	}
	if j == i+1 && j < len(src) {
		_, n := utf8.DecodeRuneInString(src[j:])
		j += n
	}
	return src[i+1 : j], j
}

// skipMathSpaces returns the index of the first byte at or after i that is
// not a space.
func skipMathSpaces(src string, i int) int {
	for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i++
	}
	return i
}

// mathAtoms returns the atoms of the top level of the formula src, with
// the classes that TeX gives them after the rules 5 and 6 of Appendix G: a
// Bin after nothing, a Bin, an Op, a Rel, an Open or a Punct is an Ord, and
// so is a Bin before a Rel, a Close, a Punct or the end. It returns false
// when the formula holds a command of mathBailCommands at its top level, or
// a group that does not close.
func mathAtoms(src string) ([]mathAtom, bool) {
	var atoms []mathAtom
	for i := 0; ; {
		i = skipMathSpaces(src, i)
		if i >= len(src) {
			break
		}
		class := atomOrd
		var end int
		switch c := src[i]; c {
		case '{':
			e, ok := mathBraceEnd(src, i)
			if !ok {
				return nil, false
			}
			end = e
		case '^', '_', '\'':
			// a script with no nucleus of its own
			if len(atoms) == 0 {
				atoms = append(atoms, mathAtom{class: atomOrd, end: i})
			}
			i = mathScripts(src, i)
			if i < 0 {
				return nil, false
			}
			atoms[len(atoms)-1].end = i
			continue
		case '}', '&', '#', '%', '$', '~':
			return nil, false
		case '(', '[':
			class, end = atomOpen, i+1
		case ')', ']', '!', '?':
			class, end = atomClose, i+1
		case ',', ';':
			class, end = atomPunct, i+1
		case '\\':
			name, e := mathCommand(src, i)
			end = e
			switch {
			case mathBailCommands[name] || name == `\` || name == "right" || name == "end":
				return nil, false
			case mathSpaceCommands[name]:
				i = end
				continue
			case name == "left":
				e, ok := mathLeftRight(src, i)
				if !ok {
					return nil, false
				}
				class, end = atomInner, e
			case name == "begin":
				e, ok := mathEnvironment(src, i)
				if !ok {
					return nil, false
				}
				end = e
			case name == "not":
				e, ok := mathToken(src, end)
				if !ok {
					return nil, false
				}
				class, end = atomRel, e
			case mathRelCommands[name]:
				class = atomRel
			case mathBinCommands[name]:
				class = atomBin
			case mathOpCommands[name]:
				class = atomOp
			case mathOpenCommands[name]:
				class = atomOpen
			case mathCloseCommands[name]:
				class = atomClose
			case name == "colon":
				class = atomPunct
			}
			if n := mathArgCommands[name]; n > 0 {
				var last int
				end, last = mathArgs(src, end, n, name == "sqrt" || name == "operatorname")
				if end < 0 {
					return nil, false
				}
				switch name {
				case "mathrel", "stackrel", "bigm", "Bigm", "biggm", "Biggm":
					class = atomRel
				case "mathbin":
					class = atomBin
				case "mathop", "operatorname":
					class = atomOp
				case "mathopen":
					class = atomOpen
				case "mathclose":
					class = atomClose
				case "mathpunct":
					class = atomPunct
				case "mathinner":
					class = atomInner
				case "overset", "underset":
					// the class of the base, as \binrel@ of amsmath gives it
					if sub, ok := mathAtoms(strings.Trim(src[last:end], "{} ")); ok && len(sub) == 1 {
						class = sub[0].class
					}
				}
			}
		default:
			r, n := utf8.DecodeRuneInString(src[i:])
			end = i + n
			switch {
			case mathRelRunes[r]:
				class = atomRel
			case mathBinRunes[r]:
				class = atomBin
			}
		}
		end = mathScripts(src, end)
		if end < 0 {
			return nil, false
		}
		// rule 5: a Bin after nothing or after an operator is an Ord
		if class == atomBin {
			if len(atoms) == 0 {
				class = atomOrd
			} else {
				switch atoms[len(atoms)-1].class {
				case atomBin, atomOp, atomRel, atomOpen, atomPunct:
					class = atomOrd
				}
			}
		}
		// rule 6: a Bin before a Rel, a Close or a Punct is an Ord
		if (class == atomRel || class == atomClose || class == atomPunct) &&
			len(atoms) > 0 && atoms[len(atoms)-1].class == atomBin {
			atoms[len(atoms)-1].class = atomOrd
		}
		atoms = append(atoms, mathAtom{class: class, end: end})
		i = end
	}
	if n := len(atoms); n > 0 && atoms[n-1].class == atomBin {
		atoms[n-1].class = atomOrd
	}
	return atoms, true
}

// mathScripts returns the end of the scripts and the primes that follow i,
// or -1 when a script has no argument.
func mathScripts(src string, i int) int {
	for {
		j := skipMathSpaces(src, i)
		if j >= len(src) {
			return i
		}
		switch src[j] {
		case '^', '_':
			e, ok := mathToken(src, j+1)
			if !ok {
				return -1
			}
			i = e
		case '\'':
			i = j + 1
		case '\\':
			name, e := mathCommand(src, j)
			if name != "limits" && name != "nolimits" && name != "displaylimits" {
				return i
			}
			i = e
		default:
			return i
		}
	}
}

// mathArgs returns the end of the n arguments that follow i, with an
// optional argument in brackets first when opt is set, and the start of the
// last argument. It returns -1 when an argument is missing.
func mathArgs(src string, i, n int, opt bool) (int, int) {
	if opt {
		j := skipMathSpaces(src, i)
		if j < len(src) && src[j] == '*' {
			j++
		}
		if j < len(src) && src[j] == '[' {
			k := strings.IndexByte(src[j:], ']')
			if k < 0 {
				return -1, 0
			}
			j += k + 1
		}
		i = j
	}
	last := i
	for range n {
		last = skipMathSpaces(src, i)
		e, ok := mathToken(src, i)
		if !ok {
			return -1, 0
		}
		i = e
	}
	return i, last
}

// mathLeftRight returns the end of the \left at i, after the delimiter of
// its \right.
func mathLeftRight(src string, i int) (int, bool) {
	depth := 0
	for j := i; j < len(src); {
		switch src[j] {
		case '{':
			e, ok := mathBraceEnd(src, j)
			if !ok {
				return 0, false
			}
			j = e
			continue
		case '\\':
			name, e := mathCommand(src, j)
			j = e
			switch name {
			case "left":
				depth++
			case "right":
				depth--
				if depth == 0 {
					return mathToken(src, j)
				}
			}
			continue
		}
		j++
	}
	return 0, false
}

// mathEnvironment returns the end of the environment that \begin opens at i,
// after its \end.
func mathEnvironment(src string, i int) (int, bool) {
	depth := 0
	for j := i; j < len(src); {
		if src[j] != '\\' {
			j++
			continue
		}
		name, e := mathCommand(src, j)
		j = e
		switch name {
		case "begin":
			depth++
		case "end":
			depth--
			if depth == 0 {
				return mathToken(src, j)
			}
		}
	}
	return 0, false
}

// inlineMathPieces returns the formula m of the text as nodes, broken where
// TeX can break a line: after an atom of the class Bin or Rel of its top
// level that a Rel does not follow (tex.web §761). Each break is a penalty,
// \binoppenalty or \relpenalty, and the glue of the space after the atom,
// \medmuskip or \thickmuskip. The engine set a formula as one box, so a
// paragraph with a long formula was set with the spaces too wide or too
// narrow where TeX breaks the formula.
//
// A piece that ends with a Bin is set with an empty atom after it and the
// space of that atom taken back, so that the Bin keeps its class and its
// space before it. It returns m alone when the formula has no break, or
// when a piece does not lay out.
func (e *Engine) inlineMathPieces(m mathNode) []node {
	if m.svg == "" || !strings.HasPrefix(m.svg, "<svg") {
		return []node{m}
	}
	atoms, ok := mathAtoms(m.src)
	if !ok || len(atoms) < 2 {
		return []node{m}
	}
	relPen, binPen := e.namedInt("relpenalty"), e.namedInt("binoppenalty")
	r := e.mathRenderer()
	if r == nil {
		return []node{m}
	}
	size := e.mathSize()
	mu := float64(size) / 18
	thick := glueSpec{width: ptToSP(5 * mu), stretch: ptToSP(5 * mu)}
	med := glueSpec{width: ptToSP(4 * mu), stretch: ptToSP(2 * mu), shrink: ptToSP(4 * mu)}
	color := "black"
	if e.curColor != 0 {
		color = hexColor(e.curColor)
	}
	var out []node
	start := 0
	piece := func(end int, suffix string) bool {
		src := strings.TrimSpace(m.src[start:end])
		svg, pm, err := r.RenderSVGMetrics(src+suffix, size)
		if err != nil {
			return false
		}
		n := m
		n.svg = strings.ReplaceAll(svg, "currentColor", color)
		n.src = src
		n.width, n.height, n.depth = ptToSP(pm.Width), ptToSP(pm.Height), ptToSP(pm.Depth)
		out = append(out, n)
		start = end
		return true
	}
	for k, a := range atoms[:len(atoms)-1] {
		if atoms[k+1].class == atomRel {
			continue
		}
		var suffix string
		var pen int
		var glue glueSpec
		switch a.class {
		case atomRel:
			pen, glue = relPen, thick
		case atomBin:
			pen, glue, suffix = binPen, med, `{}\mkern-4mu`
		default:
			continue
		}
		if pen >= 10000 {
			continue
		}
		if !piece(a.end, suffix) {
			return []node{m}
		}
		out = append(out, penaltyNode{penalty: pen}, glueNode{spec: glue})
	}
	if len(out) == 0 {
		return []node{m}
	}
	if !piece(len(m.src), "") {
		return []node{m}
	}
	return out
}
