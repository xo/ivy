package docx

import (
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ooxml"
)

// maxLevels is the number of levels of a list in WordprocessingML.
const maxLevels = 9

// level is a level of a list, w:lvl.
type level struct {
	start   int
	format  string
	text    string
	suffix  string
	legal   bool
	restart int
	ppr     props
	rpr     props
}

// abstractNum is a definition of a list, w:abstractNum.
type abstractNum struct {
	levels [maxLevels]*level
	// styleLink names the numbering style that gives the levels, for
	// w:numStyleLink.
	styleLink string
}

// numInstance is an instance of a list, w:num, with the overrides of its levels.
type numInstance struct {
	abstract  string
	overrides [maxLevels]*level
	starts    [maxLevels]int
	// restarts is true if the instance overrides a start, so that it counts
	// on its own.
	restarts bool
}

// numbering holds the lists of numbering.xml, and the counters of the
// paragraphs that the conversion met.
type numbering struct {
	abstracts map[string]*abstractNum
	nums      map[string]*numInstance
	// counters holds the counters of each list, by the key of counterKey.
	counters map[string]*[maxLevels]int
	// started tells which levels of a list counted a paragraph.
	started map[string]*[maxLevels]bool
}

// parseNumbering reads numbering.xml. A nil document gives no list.
func parseNumbering(doc *roxmltree.Document) *numbering {
	nb := &numbering{
		abstracts: map[string]*abstractNum{},
		nums:      map[string]*numInstance{},
		counters:  map[string]*[maxLevels]int{},
		started:   map[string]*[maxLevels]bool{},
	}
	if doc == nil {
		return nb
	}
	for n := range doc.RootElement().Children() {
		if !n.IsElement() {
			continue
		}
		switch n.TagName().Name() {
		case "abstractNum":
			a := &abstractNum{}
			for c := range n.Children() {
				if !c.IsElement() {
					continue
				}
				switch c.TagName().Name() {
				case "lvl":
					if i, lvl, ok := parseLevel(c); ok {
						a.levels[i] = lvl
					}
				case "numStyleLink":
					a.styleLink = ooxml.Attr(c, "val")
				}
			}
			nb.abstracts[ooxml.Attr(n, "abstractNumId")] = a
		case "num":
			m := &numInstance{}
			for c := range n.Children() {
				if !c.IsElement() {
					continue
				}
				switch c.TagName().Name() {
				case "abstractNumId":
					m.abstract = ooxml.Attr(c, "val")
				case "lvlOverride":
					i, err := strconv.Atoi(ooxml.Attr(c, "ilvl"))
					if err != nil || i < 0 || i >= maxLevels {
						continue
					}
					for e := range c.Children() {
						if !e.IsElement() {
							continue
						}
						switch e.TagName().Name() {
						case "startOverride":
							if v, err := strconv.Atoi(ooxml.Attr(e, "val")); err == nil {
								m.starts[i] = v
								m.restarts = true
							}
						case "lvl":
							if _, lvl, ok := parseLevel(e); ok {
								m.overrides[i] = lvl
								m.restarts = true
							}
						}
					}
				}
			}
			nb.nums[ooxml.Attr(n, "numId")] = m
		}
	}
	return nb
}

// parseLevel reads a w:lvl element.
func parseLevel(n roxmltree.Node) (int, *level, bool) {
	i, err := strconv.Atoi(ooxml.Attr(n, "ilvl"))
	if err != nil || i < 0 || i >= maxLevels {
		return 0, nil, false
	}
	lvl := &level{start: 1, format: "decimal", suffix: "tab", restart: -1, ppr: props{}, rpr: props{}}
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		v := ooxml.Attr(c, "val")
		switch c.TagName().Name() {
		case "start":
			if s, err := strconv.Atoi(v); err == nil {
				lvl.start = s
			}
		case "numFmt":
			lvl.format = v
		case "lvlText":
			lvl.text = v
		case "suff":
			lvl.suffix = v
		case "isLgl":
			lvl.legal = v == "" || v == "1" || v == "true" || v == "on"
		case "lvlRestart":
			if s, err := strconv.Atoi(v); err == nil {
				lvl.restart = s
			}
		case "pPr":
			lvl.ppr = parsePPr(c)
		case "rPr":
			lvl.rpr = parseRPr(c)
		}
	}
	return i, lvl, true
}

// levels returns the levels of the list numID, and the key of its
// counters. A numbering style links a definition to the levels of another
// one. It returns false for a list that the document does not define.
func (nb *numbering) levels(numID string, st *styles) ([maxLevels]*level, string, bool) {
	var out [maxLevels]*level
	m, ok := nb.nums[numID]
	if !ok || numID == "0" {
		return out, "", false
	}
	abstractID := m.abstract
	a := nb.abstracts[abstractID]
	// A w:numStyleLink names a numbering style, whose w:numPr names the
	// list with the levels.
	for range maxLevels {
		if a == nil || a.styleLink == "" {
			break
		}
		linked := st.get(a.styleLink, "numbering")
		if linked == nil {
			break
		}
		lm, ok := nb.nums[linked.ppr["numId"]]
		if !ok || lm.abstract == abstractID {
			break
		}
		abstractID = lm.abstract
		a = nb.abstracts[abstractID]
	}
	if a == nil {
		return out, "", false
	}
	for i := range maxLevels {
		out[i] = a.levels[i]
		if m.overrides[i] != nil {
			out[i] = m.overrides[i]
		}
		if out[i] != nil && m.starts[i] != 0 {
			copied := *out[i]
			copied.start = m.starts[i]
			out[i] = &copied
		}
	}
	key := "a" + abstractID
	if m.restarts {
		key = "n" + numID
	}
	return out, key, true
}

// next counts a paragraph of the list numID at the level ilvl, and returns
// the text of its marker and its level. It returns false for a paragraph
// that has no marker.
func (nb *numbering) next(numID string, ilvl int, st *styles) (string, *level, bool) {
	levels, key, ok := nb.levels(numID, st)
	if !ok || ilvl < 0 || ilvl >= maxLevels || levels[ilvl] == nil {
		return "", nil, false
	}
	counters, ok := nb.counters[key]
	if !ok {
		counters = &[maxLevels]int{}
		nb.counters[key] = counters
		nb.started[key] = &[maxLevels]bool{}
	}
	started := nb.started[key]
	if started[ilvl] {
		counters[ilvl]++
	} else {
		counters[ilvl] = levels[ilvl].start
		started[ilvl] = true
	}
	// A level restarts after a paragraph of a level above it, or after a
	// paragraph of the level that its w:lvlRestart names.
	for i := ilvl + 1; i < maxLevels; i++ {
		if levels[i] != nil && levels[i].restart == 0 {
			continue
		}
		if levels[i] != nil && levels[i].restart > 0 && levels[i].restart-1 < ilvl {
			continue
		}
		started[i] = false
	}
	lvl := levels[ilvl]
	if lvl.format == "none" {
		return "", lvl, true
	}
	text := lvl.text
	if lvl.format == "bullet" {
		return bulletText(text, lvl.rpr["font"]), lvl, true
	}
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '%' && i+1 < len(text) && text[i+1] >= '1' && text[i+1] <= '9' {
			k := int(text[i+1] - '1')
			i++
			if k > ilvl || levels[k] == nil {
				continue
			}
			v := counters[k]
			if !started[k] {
				v = levels[k].start
			}
			format := levels[k].format
			if lvl.legal {
				format = "decimal"
			}
			b.WriteString(formatNumber(v, format))
			continue
		}
		b.WriteByte(c)
	}
	return b.String(), lvl, true
}

// formatNumber writes n in the number format of WordprocessingML.
func formatNumber(n int, format string) string {
	switch format {
	case "upperRoman":
		return strings.ToUpper(roman(n))
	case "lowerRoman":
		return roman(n)
	case "upperLetter":
		return strings.ToUpper(letters(n))
	case "lowerLetter":
		return letters(n)
	case "decimalZero":
		if n >= 0 && n < 10 {
			return "0" + strconv.Itoa(n)
		}
	case "ordinal":
		return strconv.Itoa(n) + ordinalSuffix(n)
	case "none":
		return ""
	}
	return strconv.Itoa(n)
}

// roman writes n in lower case Roman numerals, or in decimal outside 1 to
// 3999.
func roman(n int) string {
	if n <= 0 || n >= 4000 {
		return strconv.Itoa(n)
	}
	var b strings.Builder
	for _, r := range [...]struct {
		v int
		s string
	}{{1000, "m"}, {900, "cm"}, {500, "d"}, {400, "cd"}, {100, "c"}, {90, "xc"}, {50, "l"}, {40, "xl"}, {10, "x"}, {9, "ix"}, {5, "v"}, {4, "iv"}, {1, "i"}} {
		for n >= r.v {
			b.WriteString(r.s)
			n -= r.v
		}
	}
	return b.String()
}

// letters writes n as Word does: a to z, then aa to zz, and so on.
func letters(n int) string {
	if n <= 0 {
		return strconv.Itoa(n)
	}
	n--
	return strings.Repeat(string(rune('a'+n%26)), min(n/26+1, 64))
}

// ordinalSuffix returns the English suffix of the ordinal n.
func ordinalSuffix(n int) string {
	if n%100 >= 11 && n%100 <= 13 {
		return "th"
	}
	switch n % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	}
	return "th"
}

// bulletText returns the text of a bullet, with a character of a symbol
// font replaced by its Unicode character.
func bulletText(text, font string) string {
	if text == "" {
		return "•"
	}
	var b strings.Builder
	for _, r := range text {
		b.WriteRune(symbolRune(font, r))
	}
	return b.String()
}
