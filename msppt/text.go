package msppt

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/odraw"
)

// baseType returns the type of text whose master style a type of text
// inherits: the title for a center title, and the body for the other
// bodies.
func baseType(kind uint32) uint32 {
	switch kind {
	case txCenterTitle:
		return txTitle
	case txCenterBody, txHalfBody, txQuarterBody:
		return txBody
	}
	return kind
}

// levelStyle returns the style of the level of a type of text: the
// defaults of the document, the style of the master for the base type, and
// the style of the master for the type. A level of a master style holds
// only what differs from the level above it, so a level takes the styles
// of the levels above it first, as PowerPoint shows them: the second level
// of a body has the bullet of the first level.
func (b *builder) levelStyle(kind uint32, level int) levelStyle {
	level = max(0, min(level, masterLevels-1))
	if kind >= txTypes {
		kind = txOther
	}
	st := b.d.doc.defaults[level]
	if b.master == nil {
		return st
	}
	m := b.master.mainMaster()
	base := baseType(kind)
	if m.hasStyle[base] {
		for l := range level + 1 {
			st.over(&m.styles[base][l])
		}
	}
	if kind != base && m.hasStyle[kind] {
		for l := range level + 1 {
			st.over(&m.styles[kind][l])
		}
	}
	return st
}

// textColor resolves a ColorIndexStruct (MS-PPT 2.12.2): an RGB value, or an
// index of the color scheme.
func (b *builder) textColor(v uint32) (drawingml.RGBA, bool) {
	switch idx := v >> 24; {
	case idx == 0xFE:
		return rgb(uint8(v), uint8(v>>8), uint8(v>>16)), true
	case idx < 8:
		return b.scheme[idx], true
	}
	return drawingml.RGBA{}, false
}

// font returns the name of the font i of the font collection.
func (b *builder) font(i int) (string, bool) {
	if i < 0 || i >= len(b.d.doc.fonts) || b.d.doc.fonts[i] == "" {
		return "", false
	}
	return b.d.doc.fonts[i], true
}

// runStyle converts the style of a run.
func (b *builder) runStyle(c *cf) drawingml.RunStyle {
	var rs drawingml.RunStyle
	if c.size.ok && c.size.v >= 1 && c.size.v <= 4000 {
		rs.Size.Set(float64(c.size.v))
	}
	if c.bold.ok {
		rs.Bold.Set(c.bold.v)
	}
	if c.italic.ok {
		rs.Italic.Set(c.italic.v)
	}
	if c.underline.ok {
		rs.Underline.Set(c.underline.v)
	}
	if c.font.ok {
		if name, ok := b.font(c.font.v); ok {
			rs.Latin.Set(name)
		}
	}
	if c.eaFont.ok {
		if name, ok := b.font(c.eaFont.v); ok {
			rs.EastAsian.Set(name)
		}
	}
	if c.color.ok {
		if col, ok := b.textColor(c.color.v); ok {
			rs.Paint.Set(drawingml.Solid(col))
		}
	}
	if c.position.ok && c.position.v != 0 {
		rs.Baseline.Set(float64(max(-100, min(c.position.v, 100))) / 100)
	}
	return rs
}

// spacing converts a ParaSpacing: a percentage of a line, or a negative
// number of master units.
func spacing(v int) (drawingml.Spacing, bool) {
	switch {
	case v > 0 && v <= 13200:
		return drawingml.Spacing{Pct: float64(v) / 100}, true
	case v < 0:
		return drawingml.Spacing{Points: true, Pt: float64(-v) / unitsPerPoint}, true
	}
	return drawingml.Spacing{}, false
}

// The alignments of TextAlignmentEnum (MS-PPT 2.13.31).
var aligns = []string{"l", "ctr", "r", "just", "dist", "dist", "justLow"}

// paraStyle converts the style of a paragraph, with the style c of its
// runs.
func (b *builder) paraStyle(p *pf, c *cf) drawingml.ParaStyle {
	var ps drawingml.ParaStyle
	if p.align.ok && p.align.v >= 0 && p.align.v < len(aligns) {
		ps.Align.Set(aligns[p.align.v])
	}
	if p.lineSpacing.ok {
		if s, ok := spacing(p.lineSpacing.v); ok {
			ps.LineSpacing.Set(s)
		}
	}
	if p.spaceBefore.ok {
		if s, ok := spacing(p.spaceBefore.v); ok {
			ps.SpaceBefore.Set(s)
		}
	}
	if p.spaceAfter.ok {
		if s, ok := spacing(p.spaceAfter.v); ok {
			ps.SpaceAfter.Set(s)
		}
	}
	margin := float64(p.leftMargin.v) / unitsPerPoint
	if p.leftMargin.ok {
		ps.MarL.Set(margin)
	}
	if p.indent.ok {
		ps.Indent.Set(float64(p.indent.v)/unitsPerPoint - margin)
	}
	switch {
	case p.hasBullet.ok && p.hasBullet.v:
		ch, font := b.bullet(p)
		ps.Bullet.Set(drawingml.Bullet{Kind: drawingml.BulletChar, Char: ch})
		ps.BulletFont.Set(font)
		if p.bulletHasColor.v && p.bulletColor.ok {
			if col, ok := b.textColor(p.bulletColor.v); ok {
				ps.BulletColor.Set(&col)
			}
		}
		if p.bulletHasSize.v && p.bulletSize.ok {
			switch v := p.bulletSize.v; {
			case v >= 25 && v <= 400:
				ps.BulletSize.Set(&drawingml.BulletSize{Pct: float64(v) / 100})
			case v < 0 && v >= -4000:
				ps.BulletSize.Set(&drawingml.BulletSize{Points: true, Pt: float64(-v)})
			}
		}
	default:
		ps.Bullet.Set(drawingml.Bullet{Kind: drawingml.BulletNone})
		// the indent is the place of the bullet, and the text of a
		// paragraph without a bullet starts at its left margin, as
		// PowerPoint draws it
		if p.leftMargin.ok {
			ps.Indent.Set(0)
		}
	}
	ps.Run = b.runStyle(c)
	return ps
}

// The characters of the bullets of the font Wingdings that the package
// draws with a font of Unicode.
var (
	wingdingsBullets = map[uint16]string{0x9F: "•", 0xA7: "▪", 0x6C: "●", 0x6E: "■", 0x71: "❑", 0x75: "◆", 0xD8: "➢", 0xFC: "✓", 0xA8: "◻", 0x76: "❖", 0xE0: "➔", 0xF0: "⇨", 0x77: "⬥", 0xA4: "◉", 0x6F: "□", 0x70: "◼"}
)

// bullet returns the character of the bullet of p and its font. A
// character of a symbol font takes its character of Unicode and the font
// of the text.
func (b *builder) bullet(p *pf) (string, string) {
	ch := uint16(0x2022)
	if p.bulletChar.ok && p.bulletChar.v != 0 {
		ch = p.bulletChar.v
	}
	var font string
	if p.bulletHasFont.v && p.bulletFont.ok {
		font, _ = b.font(p.bulletFont.v)
	}
	lower := strings.ToLower(font)
	code := ch
	if code >= 0xF000 && code <= 0xF0FF {
		code -= 0xF000
	}
	switch {
	case strings.HasPrefix(lower, "wingdings"):
		if s, ok := wingdingsBullets[code]; ok {
			return s, ""
		}
		return "•", ""
	case lower == "symbol":
		if r, ok := symbolChar(code); ok {
			return string(r), ""
		}
		return "•", ""
	case ch >= 0xF000 && ch <= 0xF0FF:
		return "•", ""
	}
	if ch < 0x20 || (ch >= 0xD800 && ch < 0xE000) {
		return "•", font
	}
	return string(rune(ch)), font
}

// textBody converts a text with its style, in a shape with the properties
// c. The text of a slide number takes the number of the slide.
func (b *builder) textBody(t *text, c odraw.Chain) *drawingml.TextBody {
	units := t.units
	if len(units) == 0 {
		return nil
	}
	tb := &drawingml.TextBody{Body: odraw.BodyStyle(c), NoKerning: true}
	n := len(units)
	if t.styleLen > 0 {
		n = t.styleLen
	}
	paras, chars := readStyleRuns(t.style, n)
	fields := make(map[int]bool, len(t.fields))
	for _, f := range t.fields {
		fields[f] = true
	}
	paraEnds, charEnds := runEnds(paras), runEnds(chars)
	// runAt returns the run of runs that holds the character i, and the
	// offset of its end
	runAt := func(runs []styleRun, ends []int, i int) (*styleRun, int) {
		k := sort.SearchInts(ends, i+1)
		if k >= len(runs) && t.styleLen > 0 && len(runs) > 0 {
			// the text replaces the text of the style, whose last run goes
			// on to its end
			return &runs[len(runs)-1], len(units) + 1
		}
		if k >= len(runs) {
			return nil, len(units) + 1
		}
		return &runs[k], ends[k]
	}
	maxParas := b.d.cfg.Limits.MaxObjects
	for start := 0; start <= len(units) && len(tb.Paragraphs) < maxParas; {
		end := start
		for end < len(units) && units[end] != '\r' {
			end++
		}
		level := 0
		var own pf
		if r, _ := runAt(paras, paraEnds, start); r != nil {
			level, own = r.level, r.pf
		}
		ls := b.levelStyle(t.kind, level)
		p := ls.pf
		// the ruler of the text gives the margins of its levels
		if r := t.ruler[max(0, min(level, masterLevels-1))]; r.margin.ok || r.indent.ok {
			p.leftMargin.over(r.margin)
			p.indent.over(r.indent)
		}
		p.over(&own)
		para := drawingml.Paragraph{Level: max(0, min(level, masterLevels-1)), Style: b.paraStyle(&p, &ls.cf)}
		for i := start; i < end; {
			var cc cf
			runEnd := end
			if r, e := runAt(chars, charEnds, i); r != nil {
				cc, runEnd = r.cf, min(e, end)
			}
			full := ls.cf
			full.over(&cc)
			rs := b.runStyle(&full)
			link, linkEnd := t.linkAt(i)
			runEnd = min(runEnd, linkEnd)
			lang, langEnd := t.langAt(i, b.d.doc.lang)
			runEnd = min(runEnd, langEnd)
			if lang != "" {
				rs.Lang.Set(lang)
			}
			if link {
				// a hyperlink takes the color of the scheme for the
				// hyperlinks, and an underline, as PowerPoint draws it
				rs.Paint.Set(drawingml.Solid(b.scheme[schemeHyperlink]))
				rs.Underline.Set(true)
				rs.Link.Set(true)
			}
			symbol := strings.EqualFold(rs.Latin.V, "symbol")
			if symbol {
				// the characters of the font Symbol take their characters of
				// Unicode, in the font of the text
				rs.Latin = drawingml.Opt[string]{}
			}
			var sb strings.Builder
			j := i
			for ; j < runEnd; j++ {
				u := units[j]
				switch {
				case u == 0x0B:
					if sb.Len() > 0 {
						para.Runs = append(para.Runs, drawingml.Run{Text: sb.String(), Style: rs})
						sb.Reset()
					}
					para.Runs = append(para.Runs, drawingml.Run{Break: true, Style: rs})
					continue
				case fields[j] && u == '*':
					sb.WriteString(strconv.Itoa(b.number))
					continue
				case u == '\t':
					sb.WriteByte('\t')
					continue
				case u < 0x20:
					continue
				}
				if symbol {
					if r, ok := symbolChar(u); ok {
						sb.WriteRune(r)
						continue
					}
				}
				if u >= 0xF000 && u <= 0xF0FF {
					// a character of a symbol font in the private area
					// takes the symbol font of the run, as PowerPoint draws
					// it
					sb.WriteRune(b.privateChar(u, &full))
					continue
				}
				if utf16.IsSurrogate(rune(u)) && j+1 < runEnd {
					if r := utf16.DecodeRune(rune(u), rune(units[j+1])); r != 0xFFFD {
						sb.WriteRune(r)
						j++
						continue
					}
				}
				sb.WriteRune(rune(u))
			}
			if sb.Len() > 0 {
				para.Runs = append(para.Runs, drawingml.Run{Text: sb.String(), Style: rs})
			}
			para.End = rs
			i = max(j, i+1)
		}
		if start == end {
			full := ls.cf
			if r, _ := runAt(chars, charEnds, start); r != nil {
				full.over(&r.cf)
			}
			para.End = b.runStyle(&full)
		}
		tb.Paragraphs = append(tb.Paragraphs, para)
		start = end + 1
	}
	return tb
}

// runEnds returns the offset of the end of each run.
func runEnds(runs []styleRun) []int {
	ends := make([]int, len(runs))
	end := 0
	for i, r := range runs {
		end += r.count
		ends[i] = end
	}
	return ends
}

// privateChar returns the character of Unicode of the character u of the
// private area from 0xF000, which is the character u-0xF000 of the symbol
// font of the style c: Symbol by default. A character of a font that the
// package does not map is a bullet.
func (b *builder) privateChar(u uint16, c *cf) rune {
	font := "Symbol"
	if c.symFont.ok {
		if name, ok := b.font(c.symFont.v); ok {
			font = name
		}
	}
	lower := strings.ToLower(font)
	switch {
	case lower == "symbol":
		if r, ok := symbolChar(u); ok {
			return r
		}
	case strings.HasPrefix(lower, "wingdings"):
		if s, ok := wingdingsBullets[u-0xF000]; ok {
			r, _ := utf8.DecodeRuneInString(s)
			return r
		}
	}
	return '•'
}

// symbolChars maps the characters of the font Symbol from 0x20 onto
// Unicode, as the encoding of the font Symbol of Adobe does.
var symbolChars = [...]rune{
	' ', '!', '∀', '#', '∃', '%', '&', '∋', '(', ')', '∗', '+', ',', '−', '.', '/',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ':', ';', '<', '=', '>', '?',
	'≅', 'Α', 'Β', 'Χ', 'Δ', 'Ε', 'Φ', 'Γ', 'Η', 'Ι', 'ϑ', 'Κ', 'Λ', 'Μ', 'Ν', 'Ο',
	'Π', 'Θ', 'Ρ', 'Σ', 'Τ', 'Υ', 'ς', 'Ω', 'Ξ', 'Ψ', 'Ζ', '[', '∴', ']', '⊥', '_',
	'‾', 'α', 'β', 'χ', 'δ', 'ε', 'φ', 'γ', 'η', 'ι', 'ϕ', 'κ', 'λ', 'μ', 'ν', 'ο',
	'π', 'θ', 'ρ', 'σ', 'τ', 'υ', 'ϖ', 'ω', 'ξ', 'ψ', 'ζ', '{', '|', '}', '∼', ' ',
}

// symbolHigh maps the characters of the font Symbol from 0xA0.
var symbolHigh = map[uint16]rune{
	0xA1: 'ϒ', 0xA2: '′', 0xA3: '≤', 0xA4: '⁄', 0xA5: '∞', 0xA6: 'ƒ', 0xA7: '♣',
	0xA8: '♦', 0xA9: '♥', 0xAA: '♠', 0xAB: '↔', 0xAC: '←', 0xAD: '↑', 0xAE: '→',
	0xAF: '↓', 0xB0: '°', 0xB1: '±', 0xB2: '″', 0xB3: '≥', 0xB4: '×', 0xB5: '∝',
	0xB6: '∂', 0xB7: '•', 0xB8: '÷', 0xB9: '≠', 0xBA: '≡', 0xBB: '≈', 0xBC: '…',
	0xC5: '⊕', 0xC6: '∅', 0xC7: '∩', 0xC8: '∪', 0xC9: '⊃', 0xCA: '⊇', 0xCB: '⊄',
	0xCC: '⊂', 0xCD: '⊆', 0xCE: '∈', 0xCF: '∉', 0xD0: '∠', 0xD1: '∇', 0xD6: '√',
	0xD7: '⋅', 0xD8: '¬', 0xD9: '∧', 0xDA: '∨', 0xDB: '⇔', 0xDC: '⇐', 0xDD: '⇑',
	0xDE: '⇒', 0xDF: '⇓', 0xE5: '∑', 0xF2: '∫',
}

// symbolChar returns the character of Unicode of the character u of the
// font Symbol, which can be in the private area from 0xF000.
func symbolChar(u uint16) (rune, bool) {
	if u >= 0xF000 && u <= 0xF0FF {
		u -= 0xF000
	}
	switch {
	case u >= 0x20 && u < 0x80:
		return symbolChars[u-0x20], true
	case u >= 0xA0 && u <= 0xFF:
		r, ok := symbolHigh[u]
		return r, ok
	}
	return 0, false
}
