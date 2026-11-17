package msdoc

import (
	"unicode/utf16"
)

// The kinds of a style, the stk of MS-DOC 2.9.304.
const (
	stkParagraph = 1
	stkCharacter = 2
	stkTable     = 3
	stkList      = 4
)

// style is a style of the style sheet, with its properties resolved through
// the styles that it is based on.
type style struct {
	// sti is the identifier of a built-in style, such as 1 for "heading 1".
	sti    int
	stk    int
	base   int
	papx   []byte
	chpx   []byte
	chp    chp
	pap    pap
	done   bool
	inWork bool
}

// styleSheet is the style sheet of a document (MS-DOC 2.9.271).
type styleSheet struct {
	styles []style
	// ftc is the default font, from rgftcStandardChpStsh, or -1.
	ftc int
	// word6 is set for the style sheet of Word 6 or Word 95.
	word6 bool
}

// defaultChp returns the properties of a character without a style.
func defaultChp() chp {
	return chp{hps: 20, ftc: -1, istd: -1}
}

// parseStyles reads the style sheet in buf. It reads what fits, and a
// damaged sheet gives fewer styles.
func parseStyles(buf []byte, maxStyles int) *styleSheet {
	return parseStylesOf(buf, maxStyles, false)
}

// parseStylesOf reads the style sheet in buf, of Word 6 or Word 95 when
// word6 is set, whose names have one byte for each character.
func parseStylesOf(buf []byte, maxStyles int, word6 bool) *styleSheet {
	ss := &styleSheet{ftc: -1, word6: word6}
	cbStshi := int(u16(buf, 0))
	if cbStshi < 4 || 2+cbStshi > len(buf) {
		return ss
	}
	stshi := buf[2 : 2+cbStshi]
	cstd := min(int(u16(stshi, 0)), maxStyles)
	cbBase := int(u16(stshi, 2))
	if len(stshi) >= 14 {
		ss.ftc = int(u16(stshi, 12))
	}
	off := 2 + cbStshi
	ss.styles = make([]style, 0, min(cstd, len(buf)/2))
	for range cstd {
		if off+2 > len(buf) {
			break
		}
		cb := int(u16(buf, off))
		off += 2
		if off+cb > len(buf) {
			break
		}
		ss.styles = append(ss.styles, parseStd(buf[off:off+cb], cbBase, word6))
		off += cb
	}
	return ss
}

// parseStd reads one STD: its StdfBase, its name and its UPXs. The base of
// an STD of Word 6 has 8 bytes, and its name one byte for each character.
func parseStd(b []byte, cbBase int, word6 bool) style {
	st := style{base: -1}
	minBase := 10
	if word6 {
		minBase = word6StdBaseSize
	}
	if len(b) < minBase || cbBase < minBase || cbBase > len(b) {
		st.stk = 0
		return st
	}
	st.sti = int(u16(b, 0) & 0x0FFF)
	st.stk = int(u16(b, 2) & 0x000F)
	st.base = int(u16(b, 2) >> 4)
	if st.base == 0x0FFF {
		st.base = -1
	}
	cupx := int(u16(b, 4) & 0x000F)
	off := cbBase
	// the name: a count of characters, the characters and a nul
	if word6 {
		if off < len(b) {
			off += 1 + int(b[off]) + 1
		}
		// the UPXs start at an even offset
		off += off % 2
	} else {
		cch := int(u16(b, off))
		off += 2 + 2*cch + 2
	}
	upx := make([][]byte, 0, cupx)
	for range cupx {
		if off+2 > len(b) {
			break
		}
		n := int(u16(b, off))
		off += 2
		if off+n > len(b) {
			break
		}
		upx = append(upx, b[off:off+n])
		off += n + n%2
	}
	switch st.stk {
	case stkParagraph:
		if len(upx) > 0 && len(upx[0]) >= 2 {
			// a UpxPapx starts with the istd
			st.papx = upx[0][2:]
		}
		if len(upx) > 1 {
			st.chpx = upx[1]
		}
	case stkCharacter:
		if len(upx) > 0 {
			st.chpx = upx[0]
		}
	case stkTable:
		if len(upx) > 1 && len(upx[1]) >= 2 {
			st.papx = upx[1][2:]
		}
		if len(upx) > 2 {
			st.chpx = upx[2]
		}
	}
	return st
}

// resolve returns the style istd with its properties, after those of the
// styles that it is based on. A loop of bases ends at the style that closes
// it. depth bounds the chain.
func (ss *styleSheet) resolve(istd, depth int) *style {
	if istd < 0 || istd >= len(ss.styles) {
		return nil
	}
	st := &ss.styles[istd]
	if st.done || st.inWork || depth <= 0 {
		return st
	}
	st.inWork = true
	c := defaultChp()
	c.ftc = ss.ftc
	p := pap{istd: istd}
	if b := ss.resolve(st.base, depth-1); b != nil && b.done && b.stk == st.stk {
		c, p = b.chp, b.pap
		p.istd = istd
	}
	base := c
	for s := range sprmsOf(ss.word6, st.chpx) {
		applyChp(&c, s, &base)
	}
	for s := range sprmsOf(ss.word6, st.papx) {
		applyPap(&p, s)
	}
	p.istd = istd
	st.chp, st.pap = c, p
	st.done, st.inWork = true, false
	return st
}

// heading returns the level of a heading, 1 to 9, of the paragraph style
// istd, or 0. The built-in styles 1 to 9 are "heading 1" to "heading 9".
func (ss *styleSheet) heading(istd int) int {
	if istd < 0 || istd >= len(ss.styles) {
		return 0
	}
	if sti := ss.styles[istd].sti; sti >= 1 && sti <= 9 {
		return sti
	}
	return 0
}

// font is a font of the table of fonts: its name and the generic family of
// CSS that is closest to it.
type font struct {
	name, generic string
}

// The generic families of the ff of an FFN (MS-DOC 2.9.85).
var generics = [8]string{"serif", "serif", "sans-serif", "monospace", "cursive", "fantasy", "serif", "serif"}

// parseFonts reads the fonts of the table SttbfFfn (MS-DOC 2.9.284). The
// name of a font starts at byte 39 of its FFN.
func parseFonts(buf []byte, maxFonts int) []font {
	if len(buf) < 4 {
		return nil
	}
	n := min(int(u16(buf, 0)), maxFonts)
	off := 4
	if u16(buf, 0) == 0xFFFF {
		// an extended table: the count follows
		n = min(int(u16(buf, 2)), maxFonts)
		off = 6
	}
	names := make([]font, 0, min(n, len(buf)/40))
	for range n {
		if off >= len(buf) {
			break
		}
		cb := int(buf[off]) + 1
		ffn := buf[off+1 : min(off+cb, len(buf))]
		off += cb
		var name []uint16
		for i := 39; i+1 < len(ffn); i += 2 {
			c := le.Uint16(ffn[i:])
			if c == 0 {
				break
			}
			name = append(name, c)
		}
		f := font{name: string(utf16.Decode(name)), generic: "serif"}
		if len(ffn) > 0 {
			f.generic = generics[ffn[0]>>4&7]
		}
		names = append(names, f)
	}
	return names
}
