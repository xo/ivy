package rtf

import "strings"

// skipped are the destinations that the conversion leaves out with their
// content.
var skipped = map[string]bool{
	"listtable": true, "listoverridetable": true, "revtbl": true,
	"rsidtbl": true, "generator": true, "nonshppict": true, "objdata": true,
	"objclass": true, "objname": true, "themedata": true, "colorschememapping": true,
	"datastore": true, "latentstyles": true, "xmlnstbl": true, "mmathPr": true,
	"annotation": true, "atnid": true, "atnauthor": true, "atndate": true, "txe": true,
	"xe": true, "tc": true, "bkmkstart": true, "bkmkend": true, "filetbl": true,
	"pgdsctbl": true, "docvar": true, "userprops": true, "private": true,
	"template": true, "fchars": true, "lchars": true, "pn": true, "pnseclvl": true,
	"pntxta": true, "pntxtb": true, "listpicture": true, "aftncn": true, "aftnsep": true,
	"aftnsepc": true, "ftncn": true, "ftnsep": true, "ftnsepc": true, "background": true,
	"wgrffmtfilter": true, "defchp": true, "defpap": true, "mhtmltag": true, "htmltag": true,
	"oldcprops": true, "oldpprops": true, "oldtprops": true, "oldsprops": true,
	"panose": true, "falt": true, "fontemb": true, "fontfile": true, "keywords": true,
	"doccomm": true, "comment": true, "hlinkbase": true, "buptim": true, "printim": true,
	"nonesttables": true,
}

// infoText are the fields of \info whose text the conversion keeps.
var infoText = map[string]bool{"title": true, "subject": true, "author": true, "operator": true, "company": true}

// word handles a control word.
func (c *converter) word(s *state, t token) {
	starred := c.starred
	c.starred = false
	if c.destination(s, t, starred) {
		return
	}
	if s.dest == destShape {
		c.shapeWord(t)
		return
	}
	if s.dest == destSkip || s.dest == destShapeName {
		return
	}
	if s.dest == destStyleSheet {
		c.styleWord(s, t)
		return
	}
	switch s.dest {
	case destFontTable:
		c.fontWord(s, t)
		return
	case destColorTable:
		c.colorWord(t)
		return
	case destPict:
		c.pictWord(s.pict, t)
		return
	case destInfo, destInfoText:
		if v := c.infoDates[s.info]; v != nil {
			i := map[string]int{"yr": 0, "mo": 1, "dy": 2, "hr": 3, "min": 4}
			if k, ok := i[t.word]; ok {
				v[k] = t.param
			}
		}
		return
	}
	if s.dest == destFieldInst {
		return
	}
	switch t.word {
	case "brdrt", "brdrb", "brdrl", "brdrr", "box":
		// a border of a paragraph, not of a cell
		c.row.side = 0
	case "brdrnone":
		if c.row.side != 0 {
			// the border of a cell that the border word before it starts
			c.tableWord(token{word: "brdrnil"})
			return
		}
	}
	switch t.word {
	case "s":
		c.applyStyle(s, 'p', t.param, 0)
		return
	case "cs":
		c.applyStyle(s, 'c', t.param, 0)
		return
	}
	if c.charWord(s, t) || c.paraWord(s, t) || c.tableWord(t) || c.sectionWord(t) {
		return
	}
	c.special(s, t)
}

// destination handles a control word that starts a destination. It
// returns false for another control word.
func (c *converter) destination(s *state, t token, starred bool) bool {
	w := t.word
	switch {
	case w == "stylesheet":
		s.dest = destStyleSheet
	case w == "fonttbl":
		s.dest = destFontTable
	case w == "colortbl":
		s.dest = destColorTable
		c.colors = []string{""}
	case w == "info":
		s.dest = destInfo
	case s.dest == destInfo && (infoText[w] || w == "creatim" || w == "revtim"):
		if w == "creatim" || w == "revtim" {
			c.infoDates[w] = &[5]int{}
		}
		s.dest = destInfoText
		s.info = w
		if w == "operator" {
			s.info = "operator"
		}
	case s.dest == destInfo:
		s.dest = destSkip
	case w == "shpinst":
		s.dest = destShape
		c.shape = shapeData{bx: originColumn, by: originColumn}
	case w == "shpgrp":
		s.shapeGroup = true
	case w == "sn" && s.dest == destShape:
		s.dest = destShapeName
		c.shape.prop = ""
	case w == "sv" && s.dest == destShape:
		// Only the picture of the property pib is the picture of the
		// shape. The other pictures fill it, and the other values are
		// text
		if strings.TrimSpace(c.shape.prop) != "pib" {
			s.dest = destShapeValue
		}
	case w == "shprslt":
		// The result of a shape repeats the picture of \shpinst.
		s.dest = destSkip
	case w == "shptxt" && s.dest == destShape:
		s.dest = destText
		if p := c.shapeParent(); p != nil && c.floatsShape(p) {
			// the text of a text box that floats waits for the end of
			// its shape, and a shape in the text box has its own state
			c.flushBytes(s)
			outer := c.shape
			s.kind, s.outer = "shptxt", &outer
			s.target = &target{}
			s.par = defaultPara()
		}
	case w == "listtext" || w == "pntext":
		if s.dest != destText {
			s.dest = destSkip
			return true
		}
		c.flushBytes(s)
		s.kind = "listtext"
		s.target = &target{}
	case w == "pict":
		if s.dest != destText && s.dest != destShape {
			s.dest = destSkip
			return true
		}
		s.dest = destPict
		s.pict = &picture{scaleX: 100, scaleY: 100}
	case w == "nesttableprops":
		// the definition of a row of a table that nests, before its
		// \nestrow. The definition of the row around it comes back at
		// the end of the group
		s.kind = "nesttableprops"
		saved := c.row
		saved.cells = append([]cellDef(nil), c.row.cells...)
		s.row = &saved
	case w == "shppict":
	case w == "field":
		s.field = &field{}
	case w == "fldinst":
		s.dest = destFieldInst
	case w == "fldrslt":
		c.fieldResult(s)
	case w == "header" || w == "headerl" || w == "headerr" || w == "headerf" ||
		w == "footer" || w == "footerl" || w == "footerr" || w == "footerf":
		if s.dest != destText {
			s.dest = destSkip
			return true
		}
		c.flushBytes(s)
		s.kind = w
		s.target = &target{}
		s.par = defaultPara()
	case w == "footnote":
		if s.dest != destText || s.target != c.body && s.kind == "" && len(c.stack) > 1 && c.inNote() {
			s.dest = destSkip
			return true
		}
		c.flushBytes(s)
		s.kind = "footnote"
		s.target = &target{}
		s.par = defaultPara()
	case skipped[w]:
		s.dest = destSkip
	case s.dest == destStyleSheet:
		// another control word of the style sheet, such as \cs after \*,
		// is a word of a style
		return false
	case starred:
		// An unknown destination after \* is left out.
		s.dest = destSkip
	default:
		return false
	}
	return true
}

// The limits of the style sheet: the number of the styles, the number of
// the control words of a style, and the depth of the styles that a style
// is based on.
const (
	maxStyles     = 4096
	maxStyleWords = 256
	maxStyleDepth = 8
)

// styleWord handles a control word of a style of the style sheet. The
// first control word of a group of the style sheet starts a style, which
// is a paragraph style until \cs says that it is a character style.
func (c *converter) styleWord(s *state, t token) {
	if s.style == nil {
		if len(c.styles) >= maxStyles {
			return
		}
		s.style = &styleEntry{kind: 'p', base: -1}
		c.styles = append(c.styles, s.style)
	}
	e := s.style
	switch t.word {
	case "s":
		e.kind, e.num = 'p', t.param
	case "cs":
		e.kind, e.num = 'c', t.param
	case "ds", "ts", "tsrowd":
		// a style of a section or of a table is not applied
		e.kind = 0
	case "sbasedon":
		e.base = t.param
	case "u", "uc", "plain", "pard":
	default:
		if len(e.words) < maxStyleWords {
			e.words = append(e.words, t)
		}
	}
}

// applyStyle applies the formatting of the style num of the kind, 'p' or
// 'c', to s: the formatting of the style that it is based on, then its
// own. A paragraph style sets the properties of the paragraph and of its
// characters, and a character style the properties of the characters.
// The control words after \s or \cs override the style.
func (c *converter) applyStyle(s *state, kind byte, num, depth int) {
	if depth > maxStyleDepth {
		return
	}
	var e *styleEntry
	for _, st := range c.styles {
		if st.kind == kind && st.num == num {
			e = st
			break
		}
	}
	if e == nil {
		return
	}
	if e.base >= 0 && e.base != num {
		c.applyStyle(s, kind, e.base, depth+1)
	}
	for _, w := range e.words {
		if !c.charWord(s, w) && kind == 'p' {
			c.paraWord(s, w)
		}
	}
}

// inNote reports whether the current group is in a footnote.
func (c *converter) inNote() bool {
	for _, s := range c.stack {
		if s.kind == "footnote" {
			return true
		}
	}
	return false
}

// fontWord handles a control word in the font table.
func (c *converter) fontWord(s *state, t token) {
	switch t.word {
	case "f":
		e := &fontEntry{charset: -1}
		if len(c.fonts) < 4096 {
			c.fonts[t.param] = e
		}
		s.font = e
	case "froman":
		c.fontGeneric(s, "serif")
	case "fswiss":
		c.fontGeneric(s, "sans-serif")
	case "fmodern":
		c.fontGeneric(s, "monospace")
	case "fscript":
		c.fontGeneric(s, "cursive")
	case "fdecor":
		c.fontGeneric(s, "fantasy")
	case "ftech":
		c.fontGeneric(s, "symbol")
	case "fcharset":
		if s.font != nil {
			s.font.charset = t.param
		}
	case "cpg":
		if s.font != nil {
			s.font.cpg = t.param
		}
	}
}

// fontGeneric sets the generic family of the current font.
func (c *converter) fontGeneric(s *state, g string) {
	if s.font != nil {
		s.font.generic = g
	}
}

// colorWord handles a control word in the color table.
func (c *converter) colorWord(t token) {
	i := len(c.colors) - 1
	v := min(max(t.param, 0), 255)
	cur := c.colors[i]
	if cur == "" {
		cur = "#000000"
	}
	b := []byte(cur)
	hex := "0123456789abcdef"
	set := func(at int) {
		b[at], b[at+1] = hex[v>>4], hex[v&15]
	}
	switch t.word {
	case "red":
		set(1)
	case "green":
		set(3)
	case "blue":
		set(5)
	default:
		return
	}
	c.colors[i] = string(b)
}

// pictWord handles a control word of a picture.
func (c *converter) pictWord(p *picture, t token) {
	if p == nil {
		return
	}
	switch t.word {
	case "pngblip":
		p.kind = "png"
	case "jpegblip":
		p.kind = "jpg"
	case "emfblip":
		p.kind = "emf"
	case "wmetafile":
		p.kind = "wmf"
	case "dibitmap", "wbitmap", "macpict", "pmmetafile":
		p.kind = ""
	case "picw":
		p.width = t.param
	case "pich":
		p.height = t.param
	case "picwgoal":
		p.goalW = t.param
	case "pichgoal":
		p.goalH = t.param
	case "picscalex":
		p.scaleX = t.param
	case "picscaley":
		p.scaleY = t.param
	case "bin":
		if int64(t.param) <= c.lim.MaxMemory {
			p.bin = append(p.bin, c.lex.binary(t.param)...)
		} else {
			c.lex.binary(t.param)
		}
	}
}

// charWord handles a control word of the formatting of the characters. It
// returns false for another control word.
func (c *converter) charWord(s *state, t token) bool {
	on := !t.hasParam || t.param != 0
	ch := &s.chr
	switch t.word {
	case "plain":
		*ch = c.defaultChar()
	case "f":
		ch.font = t.param
	case "fs":
		if t.param > 0 && t.param < 3276 {
			ch.size = t.param
		}
	case "b":
		ch.bold = on
	case "i":
		ch.italic = on
	case "ul", "uld", "uldash", "uldb", "ulth", "ulw", "ulwave", "uldashd", "uldashdd", "ulhwave", "ulldash", "ulthd", "ulthdash":
		ch.underline = on
	case "ulnone":
		ch.underline = false
	case "strike", "striked":
		ch.strike = on
	case "caps":
		ch.caps = on
	case "scaps":
		ch.smallCaps = on
	case "v":
		ch.hidden = on
	case "cf":
		ch.color = t.param
	case "cb", "chcbpat":
		ch.background = t.param
	case "highlight":
		ch.highlight = t.param
	case "super":
		ch.vertical = 1
	case "sub":
		ch.vertical = 2
	case "nosupersub":
		ch.vertical = 0
	case "up":
		ch.vertical = map[bool]int{true: 1, false: 0}[t.param > 0]
	case "dn":
		ch.vertical = map[bool]int{true: 2, false: 0}[t.param > 0]
	case "expnd":
		ch.spacing = t.param * 5
	case "expndtw":
		ch.spacing = t.param
	case "dbch":
		s.dbch = true
	case "loch", "hich":
		s.dbch = false
	case "af":
		if s.dbch {
			ch.fontEA = t.param
		}
	case "langfe":
		ch.langEA = t.param
	case "uc":
		s.uc = min(max(t.param, 0), 16)
	case "u":
		c.unicode(s, t.param)
	default:
		return false
	}
	return true
}

// paraWord handles a control word of the formatting of a paragraph. It
// returns false for another control word.
func (c *converter) paraWord(s *state, t token) bool {
	p := &s.par
	switch t.word {
	case "pard":
		*p = defaultPara()
	case "ql":
		p.align = 'l'
	case "qc":
		p.align = 'c'
	case "qr":
		p.align = 'r'
	case "qj", "qd":
		p.align = 'j'
	case "li", "lin":
		p.left = t.param
	case "ri", "rin":
		p.right = t.param
	case "fi":
		p.first = t.param
	case "sb":
		p.before = t.param
	case "sa":
		p.after = t.param
	case "sl":
		p.line = t.param
	case "slmult":
		p.lineMultiple = t.param != 0
	case "noline":
		p.noLine = true
	case "keepn":
		p.keepNext = true
	case "keep":
		p.keepLines = true
	case "pagebb":
		p.pageBrk = true
	case "intbl":
		p.inTable = true
	case "itap":
		p.itap = min(max(t.param, 0), maxNesting)
		p.inTable = p.itap > 0
	case "cbpat":
		p.shading = t.param
	case "brdrt":
		p.borderTop, p.side = true, 't'
	case "brdrb":
		p.borderBottom, p.side = true, 'b'
	case "brdrl":
		p.borderLeft, p.side = true, 'l'
	case "brdrr":
		p.borderRight, p.side = true, 'r'
	case "box":
		p.borderTop, p.borderBottom, p.borderLeft, p.borderRight = true, true, true, true
	case "brdrnone":
		switch p.side {
		case 't':
			p.borderTop = false
		case 'b':
			p.borderBottom = false
		case 'l':
			p.borderLeft = false
		case 'r':
			p.borderRight = false
		}
	default:
		return p.frame.word(t)
	}
	return true
}

// tableWord handles a control word of a table. It returns false for
// another control word.
func (c *converter) tableWord(t token) bool {
	r := &c.row
	cell := func() *cellDef {
		if len(r.cells) == 0 || r.cells[len(r.cells)-1].right != -1 {
			if len(r.cells) >= 1024 {
				return &cellDef{}
			}
			r.cells = append(r.cells, cellDef{right: -1, pad: [4]int{-1, -1, -1, -1}})
		}
		return &r.cells[len(r.cells)-1]
	}
	switch t.word {
	case "trowd":
		*r = rowDef{gap: 108, pad: [4]int{-1, -1, -1, -1}}
	case "trleft":
		r.left = t.param
	case "trgaph":
		r.gap = t.param
	case "trrh":
		r.height = t.param
	case "trqc":
		r.align = 'c'
	case "trqr":
		r.align = 'r'
	case "trhdr":
		r.header = true
	case "cellx":
		cell().right = t.param
		r.side = 0
	case "clcbpat":
		cell().shading = t.param
	case "clbrdrt":
		cell().top, r.side = true, 't'
	case "clbrdrb":
		cell().bottom, r.side = true, 'b'
	case "clbrdrl":
		cell().left, r.side = true, 'l'
	case "clbrdrr":
		cell().right2, r.side = true, 'r'
	case "brdrtbl", "brdrnil":
		// a cell border with no line (RTF 1.9.1, "Table definitions")
		if r.side != 0 {
			d := cell()
			switch r.side {
			case 't':
				d.top = false
			case 'b':
				d.bottom = false
			case 'l':
				d.left = false
			case 'r':
				d.right2 = false
			}
			r.side = 0
		}
	case "clvertalc":
		cell().valign = 'c'
	case "clvertalb":
		cell().valign = 'b'
	case "clmrg":
		cell().merged = true
	case "clvmgf":
		cell().vmerge = 'f'
	case "clvmrg":
		cell().vmerge = 'r'
	case "clpadr", "clpadb":
		cell().setPad(t.word[5], t.param)
	case "clpadl":
		// Word reads \clpadl as the top padding and \clpadt as the left
		// padding, the reverse of their names. The cells of
		// libreoffice/rtf/landscape-a4-table.rtf have \clpadt108, and Word
		// prints them with 5.4 points of padding at the left.
		cell().setPad('t', t.param)
	case "clpadt":
		cell().setPad('l', t.param)
	case "brdrw", "brdrcf":
		// the width or the color of the border of a cell that the border
		// word before it starts
		i := strings.IndexByte("trbl", r.side)
		if i < 0 {
			return false
		}
		if t.word == "brdrw" {
			cell().borderWidth[i] = min(max(t.param, 0), 255)
		} else {
			cell().borderColor[i] = t.param
		}
	case "trpaddl", "trpaddr", "trpaddt", "trpaddb":
		r.pad[strings.IndexByte("lrtb", t.word[6])] = min(max(t.param, 0), 14400)
	default:
		return false
	}
	return true
}

// sectionWord handles a control word of the document or of a section. It
// returns false for another control word.
func (c *converter) sectionWord(t token) bool {
	d, s := &c.doc, &c.section
	twips := min(max(t.param, 0), 288000)
	switch t.word {
	case "paperw":
		d.width, s.width = twips, twips
		d.paper, s.paper = false, false
	case "paperh":
		d.height, s.height = twips, twips
		d.paper, s.paper = false, false
	case "margl":
		d.left, s.left = twips, twips
	case "margr":
		d.right, s.right = twips, twips
	case "margt":
		d.top, s.top = twips, twips
	case "margb":
		d.bottom, s.bottom = twips, twips
	case "pgwsxn":
		s.width, s.paper = twips, false
	case "pghsxn":
		s.height, s.paper = twips, false
	case "marglsxn":
		s.left = twips
	case "margrsxn":
		s.right = twips
	case "margtsxn":
		s.top = twips
	case "margbsxn":
		s.bottom = twips
	case "headery":
		s.header = twips
	case "footery":
		s.footer = twips
	case "cols":
		s.cols = min(max(t.param, 1), 16)
	case "colsx":
		s.colSpace = twips
	case "titlepg":
		s.titlePage = true
	case "linemod":
		s.lineMod = min(max(t.param, 0), 100)
	case "linex":
		s.lineX = twips
	case "linestarts":
		s.lineStart = min(max(t.param, 1), 32767)
	case "lineppage":
		s.lineRestart = "page"
	case "linerestart":
		s.lineRestart = "section"
	case "linecont":
		s.lineRestart = "continuous"
	case "landscape", "lndscpsxn":
		if s.width < s.height {
			s.width, s.height = s.height, s.width
		}
	case "sbknone":
		s.breakKind = "none"
	case "sbkpage", "sbkcol":
		s.breakKind = "page"
	case "sbkeven":
		s.breakKind = "left"
	case "sbkodd":
		s.breakKind = "right"
	case "sectd":
		header, footer := s.header, s.footer
		*s = *d
		s.header, s.footer = header, footer
		s.cols, s.titlePage, s.breakKind = 1, false, ""
	case "ansicpg":
		if l := codePageLabel(t.param); l != "" {
			c.codePage = l
		}
	case "mac":
		c.codePage = "macintosh"
	case "deff":
		c.docFont = t.param
	case "htmautsp":
		c.htmlSpacing = true
	default:
		return false
	}
	return true
}

// special handles the control words that write: the breaks, the tabs, the
// marks and the ends of paragraphs, of cells, of rows and of sections.
func (c *converter) special(s *state, t token) {
	text := map[string]string{
		"tab": "\t", "emdash": "—", "endash": "–", "lquote": "‘",
		"rquote": "’", "ldblquote": "“", "rdblquote": "”", "bullet": "•",
		"emspace": "\u2003", "enspace": "\u2002", "qmspace": "\u2005", "zwj": "\u200d",
		"zwnj": "\u200c", "ltrmark": "\u200e", "rtlmark": "\u200f", "zwbo": "\u200b",
	}
	if v, ok := text[t.word]; ok {
		c.runes(s, v)
		return
	}
	if s.dest != destText {
		return
	}
	switch t.word {
	case "par":
		c.endParagraph(s)
	case "line":
		c.inline(s, "<br/>")
	case "page":
		c.endParagraph(s)
		s.target.breakNext = "page"
	case "column":
		c.endParagraph(s)
		s.target.breakNext = "column"
	case "cell", "nestcell":
		c.endCell(s, t.word == "nestcell")
	case "row", "nestrow":
		c.endRow(s, t.word == "nestrow")
	case "sect":
		if s.target == c.body {
			c.endParagraph(s)
			c.endTarget(c.body)
			c.closeSection()
		}
	case "chpgn":
		c.inline(s, `<span class="pg"></span>`)
	case "chftn":
		if c.inNote() {
			c.inline(s, `<span class="fnref"></span>`)
		}
	}
}

// inline writes an element into the current paragraph of s.
func (c *converter) inline(s *state, html string) {
	c.flushBytes(s)
	c.flushRun(s.target)
	s.target.para.WriteString(html)
}

// fieldResult starts the result of a field: the number of the page, a
// link, or the text of the result.
func (c *converter) fieldResult(s *state) {
	if s.field == nil {
		return
	}
	f := strings.Fields(s.field.inst.String())
	if len(f) == 0 {
		return
	}
	switch strings.ToUpper(f[0]) {
	case "PAGE":
		c.inline(s, `<span class="pg"></span>`)
		s.dest = destSkip
	case "NUMPAGES", "SECTIONPAGES":
		c.inline(s, `<span class="pgs"></span>`)
		s.dest = destSkip
	case "HYPERLINK":
		for _, a := range f[1:] {
			if strings.HasPrefix(a, `\`) {
				continue
			}
			href := strings.Trim(a, `"`)
			lower := strings.ToLower(href)
			if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") {
				c.inline(s, `<a href="`+attrText(href)+`">`)
				s.closeLink = true
			}
			break
		}
	}
}
