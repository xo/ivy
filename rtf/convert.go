package rtf

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/charset"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
)

// documentName is the path of the XHTML document in the FS of the source.
const documentName = "index.xhtml"

// styleSheet is the style sheet of the format. The conversion writes the
// formatting of each element in its style attribute, and the rules of each
// section after this style sheet.
const styleSheet = `html { font-family: serif; font-size: 12pt; line-height: 1.15 }
body { margin: 0; counter-reset: note }
p { margin: 0; white-space: pre-wrap; orphans: 2; widows: 2; tab-size: 36pt }
table { border-collapse: collapse; border-spacing: 0; table-layout: fixed }
td { padding: 0 5.4pt; vertical-align: top; border: none }
a { color: inherit; text-decoration: none }
.pg::before { content: counter(page) }
.pgs::before { content: counter(pages) }
.fn { float: footnote; counter-increment: note }
::footnote-call { content: counter(note); vertical-align: super; font-size: 0.65em; line-height: inherit }
::footnote-marker { content: none }
.fnref::before { content: counter(note) }
@page {
  @footnote { margin-top: 6pt; padding-top: 3pt; border-top: 0.5pt solid black }
  @top-center { width: 100%; vertical-align: top; text-align: left }
  @bottom-center { width: 100%; vertical-align: bottom; text-align: left }
}
`

// destination is what the text of a group is.
type destination int

// The destinations that the conversion reads.
const (
	// destText is text that the document shows: the body, a header, a
	// footnote, a cell or the result of a field.
	destText destination = iota
	destSkip
	destFontTable
	destColorTable
	destInfo
	destInfoText
	destPict
	destFieldInst
	// destShape is the instruction of a shape, \shpinst. Its text is left
	// out, and its picture is shown.
	destShape
	// destShapeName is the name of a property of a shape, \sn.
	destShapeName
)

// charProps are the properties of the characters.
type charProps struct {
	font                    int
	size                    int
	bold, italic, underline bool
	strike, caps, smallCaps bool
	hidden                  bool
	color, background       int
	vertical                int
	spacing                 int
	highlight               int
}

// paraProps are the properties of a paragraph.
type paraProps struct {
	align                        byte
	left, right, first           int
	before, after                int
	line                         int
	lineMultiple                 bool
	keepNext, keepLines, pageBrk bool
	inTable                      bool
	shading                      int
	borderTop, borderBottom      bool
	borderLeft, borderRight      bool
	// side is the side of the border that the next border word sets.
	side byte
}

// sectionProps are the properties of a section, in twips.
type sectionProps struct {
	width, height            int
	left, right, top, bottom int
	header, footer           int
	cols, colSpace           int
	titlePage                bool
	breakKind                string
}

// cellDef is the definition of a cell of a row: its right edge and its
// properties.
type cellDef struct {
	right int
	// pad holds the paddings of the cell in twips, left, right, top and
	// bottom, or -1.
	pad                       [4]int
	shading                   int
	top, bottom, left, right2 bool
	valign                    byte
	merged                    bool
}

// setPad sets the padding of the side, 'l', 'r', 't' or 'b', in twips.
func (d *cellDef) setPad(side byte, twips int) {
	if i := strings.IndexByte("lrtb", side); i >= 0 {
		d.pad[i] = min(max(twips, 0), 14400)
	}
}

// rowDef is the definition of a row, from \trowd.
type rowDef struct {
	// pad holds the default paddings of the cells in twips, left, right,
	// top and bottom, or -1.
	pad       [4]int
	cells     []cellDef
	left, gap int
	height    int
	align     byte
	header    bool
}

// field is a field of RTF, \field.
type field struct {
	inst strings.Builder
}

// picture is a picture of RTF, \pict.
type picture struct {
	kind          string
	width, height int
	goalW, goalH  int
	scaleX        int
	scaleY        int
	hex           []byte
	half          int
	bin           []byte
}

// fontEntry is a font of the font table.
type fontEntry struct {
	name    string
	generic string
	charset int
	cpg     int
}

// state is the state of a group.
type state struct {
	dest destination
	chr  charProps
	par  paraProps
	uc   int
	// target receives the text of the group.
	target *target
	// field is the field of the group, and pict its picture.
	field *field
	pict  *picture
	// font is the entry of the font table that the group defines.
	font *fontEntry
	// info is the name of the field of \info that the group holds.
	info string
	// closeLink closes a link of a field at the end of the group.
	closeLink bool
	// kind names a header, a footer or a footnote that the group holds.
	kind string
}

// target is a flow of blocks: the body, a header, a footer or a footnote.
type target struct {
	out  strings.Builder
	para strings.Builder
	// run holds the text of the current run, with the properties runProps.
	run      strings.Builder
	runProps charProps
	// bytes holds the bytes of text that wait for their code page.
	bytes []byte
	// breakNext is the break before the next paragraph.
	breakNext string
	// marker is the marker of the current paragraph of a list, from
	// \listtext, and markerAt its place in para.
	marker   string
	markerAt int
	// table is set while a table is open, row holds the cells of the
	// current row, and cell the blocks of the current cell.
	table bool
	row   []string
	cell  strings.Builder
}

// converter converts RTF into XHTML.
type converter struct {
	d   *Document
	ctx context.Context
	lim ivy.Limits
	log *slog.Logger
	lex lexer

	fs     memfs.FS
	sheet  strings.Builder
	body   *target
	fonts  map[int]*fontEntry
	colors []string
	// docFont is the default font, and codePage the code page of the file.
	docFont  int
	codePage string

	stack []*state
	// skipped counts the groups past the limit of depth, which the
	// conversion skips.
	skipped int
	// skip is the number of characters to skip after \uN.
	skip int
	// high is a high surrogate of \uN that waits for its low surrogate.
	high rune
	// starred is set after \*, until the next control word.
	starred bool
	// shapeProp is the name of the last property of a shape, and shapeBox
	// the box of the shape in twips: left, top, right and bottom.
	shapeProp string
	shapeBox  [4]int

	doc     sectionProps
	section sectionProps
	row     rowDef
	// sectionNum counts the sections, and open is set while a section is
	// open.
	sectionNum int
	open       bool
	running    map[string]string
	hfNum      int
	files      int

	infoFields map[string]string
	infoDates  map[string]*[5]int
	err        error
	deep       bool
}

// newConverter returns a converter of the file of d.
func newConverter(ctx context.Context, d *Document) *converter {
	c := &converter{
		d: d, ctx: ctx, lim: d.lim, log: d.log,
		lex:        lexer{data: d.data},
		body:       &target{},
		fonts:      map[int]*fontEntry{},
		colors:     []string{""},
		codePage:   "windows-1252",
		running:    map[string]string{},
		infoFields: map[string]string{},
		infoDates:  map[string]*[5]int{},
	}
	c.doc = sectionProps{width: 12240, height: 15840, left: 1800, right: 1800, top: 1440, bottom: 1440, header: 720, footer: 720, cols: 1, colSpace: 720}
	c.section = c.doc
	return c
}

// defaultChar returns the properties of \plain.
func (c *converter) defaultChar() charProps {
	return charProps{font: c.docFont, size: 24}
}

// defaultPara returns the properties of \pard.
func defaultPara() paraProps {
	return paraProps{align: 'l'}
}

// top returns the state of the current group.
func (c *converter) top() *state {
	return c.stack[len(c.stack)-1]
}

// convert converts the file into a layout.Source.
func (c *converter) convert() (layout.Source, error) {
	c.stack = []*state{{dest: destText, chr: c.defaultChar(), par: defaultPara(), uc: 1, target: c.body}}
	work := 0
	for {
		t := c.lex.next()
		if t.kind == tokenEOF {
			break
		}
		work++
		if work%4096 == 0 {
			if err := c.check(); err != nil {
				return layout.Source{}, err
			}
		}
		c.token(t)
		if c.err != nil {
			return layout.Source{}, c.err
		}
	}
	for len(c.stack) > 1 {
		c.closeGroup()
	}
	c.endTarget(c.body)
	if c.sectionNum == 0 {
		// A file without text has one empty page.
		c.openSection()
		c.body.out.WriteString("<p><br/></p>")
	}
	c.closeSection()
	if err := c.check(); err != nil {
		return layout.Source{}, err
	}
	if c.deep {
		c.log.Warn("rtf: groups nest deeper than the limit and are left out", "limit", c.lim.MaxDepth)
	}
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	out.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml"><head><meta charset="UTF-8"/><title></title></head>` + "\n<body>")
	out.WriteString(c.body.out.String())
	out.WriteString("</body></html>\n")
	c.fs.Add(documentName, []byte(out.String()))
	return layout.Source{
		FS:          &c.fs,
		Documents:   []string{documentName},
		StyleSheets: []string{styleSheet, c.sheet.String()},
	}, nil
}

// check records an error if the context ends or the document grows past
// the limit of memory.
func (c *converter) check() error {
	if c.err != nil {
		return c.err
	}
	if err := c.ctx.Err(); err != nil {
		c.err = fmt.Errorf("converting rtf: %w", err)
	} else if int64(c.body.out.Len()+c.body.para.Len()) > c.lim.MaxMemory {
		c.err = fmt.Errorf("xhtml of more than %d bytes: %w", c.lim.MaxMemory, ivy.ErrLimit)
	}
	return c.err
}

// token handles a token.
func (c *converter) token(t token) {
	switch t.kind {
	case tokenOpen:
		c.skip = 0
		if c.skipped == 0 {
			c.flushBytes(c.top())
		}
		if c.skipped > 0 || len(c.stack) >= c.lim.MaxDepth {
			if c.skipped == 0 {
				c.deep = true
			}
			c.skipped++
			return
		}
		s := *c.top()
		s.closeLink, s.kind, s.info = false, "", ""
		c.stack = append(c.stack, &s)
		return
	case tokenClose:
		c.skip = 0
		if c.skipped > 0 {
			c.skipped--
			return
		}
		if len(c.stack) > 1 {
			c.closeGroup()
		}
		return
	}
	if c.skipped > 0 {
		return
	}
	// The bytes of text decode with the font of the text, so a control
	// word, which can change the font, decodes them first.
	if t.kind == tokenWord {
		c.flushBytes(c.top())
	}
	if c.skip > 0 {
		switch t.kind {
		case tokenText:
			n := min(c.skip, len(t.text))
			c.skip -= n
			t.text = t.text[n:]
			if len(t.text) == 0 {
				return
			}
		default:
			c.skip--
			return
		}
	}
	s := c.top()
	switch t.kind {
	case tokenText:
		c.text(s, t.text)
	case tokenSymbol:
		c.symbol(s, t)
	case tokenWord:
		c.word(s, t)
	}
}

// text handles a run of text in the destination of s.
func (c *converter) text(s *state, b []byte) {
	switch s.dest {
	case destText:
		if !s.chr.hidden {
			s.target.bytes = append(s.target.bytes, b...)
		}
	case destFontTable:
		if s.font == nil {
			return
		}
		str := string(b)
		if i := strings.IndexByte(str, ';'); i >= 0 {
			str = str[:i]
		}
		if len(s.font.name) < 256 {
			s.font.name += str
		}
	case destColorTable:
		for _, ch := range b {
			if ch == ';' && len(c.colors) < 1<<16 {
				c.colors = append(c.colors, "")
			}
		}
	case destInfoText:
		if v := c.infoFields[s.info]; len(v) < 4096 {
			c.infoFields[s.info] = v + c.decode(b, s)
		}
	case destPict:
		c.pictHex(s.pict, b)
	case destFieldInst:
		if s.field != nil && s.field.inst.Len() < 4096 {
			s.field.inst.Write(b)
		}
	case destShapeName:
		if len(c.shapeProp) < 64 {
			c.shapeProp += string(b)
		}
	}
}

// pictHex adds the hexadecimal digits of b to the data of a picture.
func (c *converter) pictHex(p *picture, b []byte) {
	if p == nil {
		return
	}
	for _, ch := range b {
		v := hexValue(ch)
		if v < 0 {
			continue
		}
		if int64(len(p.hex)) >= c.lim.MaxMemory {
			return
		}
		if p.half%2 == 0 {
			p.hex = append(p.hex, byte(v<<4))
		} else {
			p.hex[len(p.hex)-1] |= byte(v)
		}
		p.half++
	}
}

// symbol handles a control symbol.
func (c *converter) symbol(s *state, t token) {
	switch t.word {
	case "'":
		if len(t.text) == 1 {
			c.text(s, t.text)
		}
	case "*":
		// The next control word is a destination that the conversion
		// skips if it does not know it.
		c.starred = true
	case "~":
		c.runes(s, "\u00a0")
	case "-":
		c.runes(s, "\u00ad")
	case "_":
		c.runes(s, "\u2011")
	case "{", "}", "\\":
		c.text(s, []byte(t.word))
	case "|", ":":
	}
}

// runes writes text that is already Unicode.
func (c *converter) runes(s *state, text string) {
	if s.dest != destText || s.chr.hidden {
		if s.dest == destInfoText {
			c.infoFields[s.info] += text
		}
		return
	}
	c.flushBytes(s)
	c.addText(s, text)
}

// flushBytes decodes the bytes of text that wait in the target of s.
func (c *converter) flushBytes(s *state) {
	t := s.target
	if t == nil || len(t.bytes) == 0 {
		return
	}
	text := c.decode(t.bytes, s)
	t.bytes = t.bytes[:0]
	c.addText(s, text)
}

// addText adds text with the properties of s to the current run.
func (c *converter) addText(s *state, text string) {
	t := s.target
	if text == "" {
		return
	}
	if t.run.Len() > 0 && t.runProps != s.chr {
		c.flushRun(t)
	}
	if t.run.Len() == 0 {
		t.runProps = s.chr
	}
	t.run.WriteString(text)
}

// fontCodePage returns the label of the encoding of the font f.
func (c *converter) fontCodePage(f int) string {
	e, ok := c.fonts[f]
	if !ok {
		return c.codePage
	}
	if e.cpg > 0 {
		if l := codePageLabel(e.cpg); l != "" {
			return l
		}
	}
	if l, ok := charsetLabels[e.charset]; ok {
		if l == "" {
			return c.codePage
		}
		return l
	}
	return c.codePage
}

// charsetLabels maps the values of \fcharset to the labels of the Encoding
// Standard. An empty label is the code page of the file.
var charsetLabels = map[int]string{
	0: "", 1: "", 2: "symbol", 77: "macintosh", 128: "shift_jis", 129: "euc-kr",
	130: "euc-kr", 134: "gbk", 136: "big5", 161: "windows-1253", 162: "windows-1254",
	163: "windows-1258", 177: "windows-1255", 178: "windows-1256", 186: "windows-1257",
	204: "windows-1251", 222: "windows-874", 238: "windows-1250", 255: "ibm866",
}

// codePageLabel returns the label of the Encoding Standard for a code page
// of Windows, or "".
func codePageLabel(cp int) string {
	if cp == 874 || cp >= 1250 && cp <= 1258 {
		return "windows-" + strconv.Itoa(cp)
	}
	switch cp {
	case 932:
		return "shift_jis"
	case 936:
		return "gbk"
	case 949:
		return "euc-kr"
	case 950:
		return "big5"
	case 10000:
		return "macintosh"
	case 866:
		return "ibm866"
	case 65001:
		return "utf-8"
	case 437, 850:
		return "windows-1252"
	}
	return ""
}

// decode decodes bytes of text in the code page of the font of s.
func (c *converter) decode(b []byte, s *state) string {
	label := c.fontCodePage(s.chr.font)
	if label == "symbol" {
		var sb strings.Builder
		for _, ch := range b {
			sb.WriteRune(symbolRune(rune(ch)))
		}
		return sb.String()
	}
	text, ok := charset.Decode(b, label)
	if !ok {
		text, _ = charset.Decode(b, "windows-1252")
	}
	return text
}

// flushRun writes the current run of a target into its paragraph.
func (c *converter) flushRun(t *target) {
	if t.run.Len() == 0 {
		return
	}
	style := c.runStyle(t.runProps)
	if style != "" {
		t.para.WriteString(`<span style="` + attrText(style) + `">`)
	}
	escape(&t.para, t.run.String(), false)
	if style != "" {
		t.para.WriteString("</span>")
	}
	t.run.Reset()
}

// closeGroup ends the current group.
func (c *converter) closeGroup() {
	s := c.top()
	c.flushBytes(s)
	c.stack = c.stack[:len(c.stack)-1]
	parent := c.top()
	switch {
	case s.dest == destPict && s.pict != nil && s.pict != parent.pict:
		c.endPicture(parent, s.pict)
	case s.kind == "listtext" && s.target != parent.target:
		c.flushRun(s.target)
		c.listText(parent, s)
	case s.kind != "" && s.target != parent.target:
		c.endTarget(s.target)
		c.endNested(parent, s)
	case s.closeLink:
		c.flushBytes(parent)
		c.flushRun(parent.target)
		parent.target.para.WriteString("</a>")
	case s.dest == destFontTable && s.font != nil && s.font != parent.font:
		// A font entry in a group of its own ends here.
	}
	if s.dest == destColorTable && parent.dest != destColorTable && len(c.colors) > 1 && c.colors[len(c.colors)-1] == "" {
		c.colors = c.colors[:len(c.colors)-1]
	}
}

// info returns what the group \info says.
func (c *converter) info() ivy.Info {
	info := ivy.Info{
		Title:   strings.TrimSpace(c.infoFields["title"]),
		Subject: strings.TrimSpace(c.infoFields["subject"]),
	}
	if a := strings.TrimSpace(c.infoFields["author"]); a != "" {
		info.Authors = []string{a}
	}
	date := func(v *[5]int) time.Time {
		if v == nil || v[0] <= 0 || v[1] < 1 || v[1] > 12 || v[2] < 1 || v[2] > 31 {
			return time.Time{}
		}
		return time.Date(v[0], time.Month(v[1]), v[2], min(max(v[3], 0), 23), min(max(v[4], 0), 59), 0, 0, time.UTC)
	}
	info.Created = date(c.infoDates["creatim"])
	info.Modified = date(c.infoDates["revtim"])
	return info
}

// unicode handles \uN.
func (c *converter) unicode(s *state, n int) {
	if n < 0 {
		n += 65536
	}
	r := rune(n)
	c.skip = s.uc
	switch {
	case utf16.IsSurrogate(r) && r < 0xdc00:
		c.high = r
		return
	case utf16.IsSurrogate(r):
		if c.high != 0 {
			r = utf16.DecodeRune(c.high, r)
		}
		c.high = 0
	default:
		c.high = 0
	}
	if s.dest == destText && s.target != nil {
		label := c.fontCodePage(s.chr.font)
		if label == "symbol" && r >= 0xf020 && r <= 0xf0ff {
			r = symbolRune(r - 0xf000)
		}
	}
	if s.dest == destFieldInst && s.field != nil {
		s.field.inst.WriteRune(r)
		return
	}
	c.runes(s, string(r))
}
