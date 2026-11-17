package msppt

// opt is a value of a style that is set or not.
type opt[T any] struct {
	v  T
	ok bool
}

// set sets the value.
func (o *opt[T]) set(v T) {
	o.v, o.ok = v, true
}

// over sets o to p if p is set.
func (o *opt[T]) over(p opt[T]) {
	if p.ok {
		*o = p
	}
}

// pf is a TextPFException: the style of a paragraph (MS-PPT 2.9.20). A
// field that the record does not give is not set.
type pf struct {
	hasBullet, bulletHasFont, bulletHasColor, bulletHasSize opt[bool]
	bulletChar                                              opt[uint16]
	bulletFont                                              opt[int]
	bulletSize                                              opt[int]
	bulletColor                                             opt[uint32]
	align                                                   opt[int]
	lineSpacing, spaceBefore, spaceAfter                    opt[int]
	leftMargin, indent                                      opt[int]
}

// over sets each field of p that o sets.
func (p *pf) over(o *pf) {
	p.hasBullet.over(o.hasBullet)
	p.bulletHasFont.over(o.bulletHasFont)
	p.bulletHasColor.over(o.bulletHasColor)
	p.bulletHasSize.over(o.bulletHasSize)
	p.bulletChar.over(o.bulletChar)
	p.bulletFont.over(o.bulletFont)
	p.bulletSize.over(o.bulletSize)
	p.bulletColor.over(o.bulletColor)
	p.align.over(o.align)
	p.lineSpacing.over(o.lineSpacing)
	p.spaceBefore.over(o.spaceBefore)
	p.spaceAfter.over(o.spaceAfter)
	p.leftMargin.over(o.leftMargin)
	p.indent.over(o.indent)
}

// cf is a TextCFException: the style of a run of characters (MS-PPT
// 2.9.14).
type cf struct {
	bold, italic, underline opt[bool]
	font                    opt[int]
	size                    opt[int]
	color                   opt[uint32]
	position                opt[int]
	// eaFont is the font of the East Asian text, from oldEATypeface.
	eaFont opt[int]
	// symFont is the font of the characters of a symbol font, from
	// symbolTypeface.
	symFont opt[int]
}

// over sets each field of c that o sets.
func (c *cf) over(o *cf) {
	c.bold.over(o.bold)
	c.italic.over(o.italic)
	c.underline.over(o.underline)
	c.font.over(o.font)
	c.eaFont.over(o.eaFont)
	c.symFont.over(o.symFont)
	c.size.over(o.size)
	c.color.over(o.color)
	c.position.over(o.position)
}

// The bits of a PFMasks (MS-PPT 2.9.21).
const (
	pfHasBullet      = 1 << 0
	pfBulletHasFont  = 1 << 1
	pfBulletHasColor = 1 << 2
	pfBulletHasSize  = 1 << 3
	pfBulletFont     = 1 << 4
	pfBulletColor    = 1 << 5
	pfBulletSize     = 1 << 6
	pfBulletChar     = 1 << 7
	pfLeftMargin     = 1 << 8
	pfIndent         = 1 << 10
	pfAlign          = 1 << 11
	pfLineSpacing    = 1 << 12
	pfSpaceBefore    = 1 << 13
	pfSpaceAfter     = 1 << 14
	pfDefaultTabSize = 1 << 15
	pfFontAlign      = 1 << 16
	pfWrapFlags      = 7 << 17
	pfTabStops       = 1 << 20
	pfTextDirection  = 1 << 21
	pfBulletFlags    = pfHasBullet | pfBulletHasFont | pfBulletHasColor | pfBulletHasSize
)

// The bits of a CFMasks (MS-PPT 2.9.15).
const (
	cfBold          = 1 << 0
	cfItalic        = 1 << 1
	cfUnderline     = 1 << 2
	cfStyleFlags    = 0x3FF | 0xF<<10
	cfTypeface      = 1 << 16
	cfSize          = 1 << 17
	cfColor         = 1 << 18
	cfPosition      = 1 << 19
	cfOldEATypeface = 1 << 21
	cfANSITypeface  = 1 << 22
	cfSymbolFace    = 1 << 23
)

// reader reads the fields of a record, and fails at its end.
type reader struct {
	b   []byte
	off int
	bad bool
}

func (r *reader) u16() uint16 {
	if r.off+2 > len(r.b) {
		r.bad = true
		return 0
	}
	v := le.Uint16(r.b[r.off:])
	r.off += 2
	return v
}

func (r *reader) u32() uint32 {
	if r.off+4 > len(r.b) {
		r.bad = true
		return 0
	}
	v := le.Uint32(r.b[r.off:])
	r.off += 4
	return v
}

func (r *reader) i16() int {
	return int(int16(r.u16()))
}

// readPF reads a TextPFException at the start of b. It returns the number
// of bytes that it read.
func readPF(b []byte) (pf, int, bool) {
	r := reader{b: b}
	var p pf
	m := r.u32()
	if m&pfBulletFlags != 0 {
		f := r.u16()
		if m&pfHasBullet != 0 {
			p.hasBullet.set(f&1 != 0)
		}
		if m&pfBulletHasFont != 0 {
			p.bulletHasFont.set(f&2 != 0)
		}
		if m&pfBulletHasColor != 0 {
			p.bulletHasColor.set(f&4 != 0)
		}
		if m&pfBulletHasSize != 0 {
			p.bulletHasSize.set(f&8 != 0)
		}
	}
	if m&pfBulletChar != 0 {
		p.bulletChar.set(r.u16())
	}
	if m&pfBulletFont != 0 {
		p.bulletFont.set(int(r.u16()))
	}
	if m&pfBulletSize != 0 {
		p.bulletSize.set(r.i16())
	}
	if m&pfBulletColor != 0 {
		p.bulletColor.set(r.u32())
	}
	if m&pfAlign != 0 {
		p.align.set(int(r.u16()))
	}
	if m&pfLineSpacing != 0 {
		p.lineSpacing.set(r.i16())
	}
	if m&pfSpaceBefore != 0 {
		p.spaceBefore.set(r.i16())
	}
	if m&pfSpaceAfter != 0 {
		p.spaceAfter.set(r.i16())
	}
	if m&pfLeftMargin != 0 {
		p.leftMargin.set(r.i16())
	}
	if m&pfIndent != 0 {
		p.indent.set(r.i16())
	}
	if m&pfDefaultTabSize != 0 {
		r.u16()
	}
	if m&pfTabStops != 0 {
		n := int(r.u16())
		if n > 1024 {
			return p, 0, false
		}
		r.off += 4 * n
	}
	if m&pfFontAlign != 0 {
		r.u16()
	}
	if m&pfWrapFlags != 0 {
		r.u16()
	}
	if m&pfTextDirection != 0 {
		r.u16()
	}
	if r.bad || r.off > len(b) {
		return p, 0, false
	}
	return p, r.off, true
}

// readCF reads a TextCFException at the start of b. It returns the number
// of bytes that it read.
func readCF(b []byte) (cf, int, bool) {
	r := reader{b: b}
	var c cf
	m := r.u32()
	if m&cfStyleFlags != 0 {
		f := r.u16()
		if m&cfBold != 0 {
			c.bold.set(f&1 != 0)
		}
		if m&cfItalic != 0 {
			c.italic.set(f&2 != 0)
		}
		if m&cfUnderline != 0 {
			c.underline.set(f&4 != 0)
		}
	}
	if m&cfTypeface != 0 {
		c.font.set(int(r.u16()))
	}
	if m&cfOldEATypeface != 0 {
		c.eaFont.set(int(r.u16()))
	}
	if m&cfANSITypeface != 0 {
		r.u16()
	}
	if m&cfSymbolFace != 0 {
		c.symFont.set(int(r.u16()))
	}
	if m&cfSize != 0 {
		c.size.set(r.i16())
	}
	if m&cfColor != 0 {
		c.color.set(r.u32())
	}
	if m&cfPosition != 0 {
		c.position.set(r.i16())
	}
	if r.bad {
		return c, 0, false
	}
	return c, r.off, true
}

// levelStyle is the style of a level of the text of a master.
type levelStyle struct {
	pf pf
	cf cf
}

// over sets each field of l that o sets.
func (l *levelStyle) over(o *levelStyle) {
	l.pf.over(&o.pf)
	l.cf.over(&o.cf)
}

// masterLevels is the number of levels of the text of a master.
const masterLevels = 5

// masterStyle is the style of the five levels of a type of text.
type masterStyle [masterLevels]levelStyle

// readMasterStyle reads a TextMasterStyleAtom of the type of text inst
// (MS-PPT 2.9.35). A level that the record does not give is not set.
func readMasterStyle(b []byte, inst uint16) (masterStyle, bool) {
	var st masterStyle
	if len(b) < 2 {
		return st, false
	}
	n := min(int(le.Uint16(b)), masterLevels)
	off := 2
	for i := range n {
		lvl := i
		if inst >= txCenterBody {
			if off+2 > len(b) {
				return st, false
			}
			lvl = int(le.Uint16(b[off:]))
			off += 2
		}
		p, np, ok := readPF(b[off:])
		if !ok {
			return st, false
		}
		off += np
		c, nc, ok := readCF(b[off:])
		if !ok {
			return st, false
		}
		off += nc
		if lvl >= 0 && lvl < masterLevels {
			st[lvl] = levelStyle{pf: p, cf: c}
		}
	}
	return st, true
}

// styleRun is a run of the paragraphs or of the characters of a
// StyleTextPropAtom: a number of characters with a style.
type styleRun struct {
	count int
	level int
	pf    pf
	cf    cf
}

// readStyleRuns reads the runs of the paragraphs and of the characters of
// a StyleTextPropAtom, for a text of n characters (MS-PPT 2.9.44). The
// runs count the characters of the text and one more, the end of the last
// paragraph.
func readStyleRuns(b []byte, n int) (paras, chars []styleRun) {
	off := 0
	for total := 0; total <= n && off+6 <= len(b) && len(paras) < n+1; {
		count := int(le.Uint32(b[off:]))
		level := int(le.Uint16(b[off+4:]))
		p, np, ok := readPF(b[off+6:])
		if !ok || count <= 0 {
			return paras, nil
		}
		off += 6 + np
		paras = append(paras, styleRun{count: count, level: level, pf: p})
		total += count
	}
	for total := 0; total <= n && off+4 <= len(b) && len(chars) < n+1; {
		count := int(le.Uint32(b[off:]))
		c, nc, ok := readCF(b[off+4:])
		if !ok || count <= 0 {
			return paras, chars
		}
		off += 4 + nc
		chars = append(chars, styleRun{count: count, cf: c})
		total += count
	}
	return paras, chars
}
