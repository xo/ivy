package msdoc

import "iter"

// This file reads the property modifiers of MS-DOC 2.6: a sprm and its
// operand change one property of the characters, the paragraphs, the
// sections or the tables.

// The sprms that the package reads.
const (
	sprmCFRMarkDel  = 0x0800
	sprmCFBold      = 0x0835
	sprmCFItalic    = 0x0836
	sprmCFStrike    = 0x0837
	sprmCFSmallCaps = 0x083A
	sprmCFCaps      = 0x083B
	sprmCFVanish    = 0x083C
	sprmCHighlight  = 0x2A0C
	sprmCKul        = 0x2A3E
	sprmCIco        = 0x2A42
	sprmCIss        = 0x2A48
	sprmCFDStrike   = 0x2A53
	sprmCIstd       = 0x4A30
	sprmCHps        = 0x4A43
	sprmCRgFtc0     = 0x4A4F
	sprmCCv         = 0x6870
	sprmPIstd       = 0x4600
	sprmPJc80       = 0x2403
	sprmPFPageBreak = 0x2407
	sprmPFInTable   = 0x2416
	sprmPFTtp       = 0x2417
	sprmPFInnerCell = 0x244B
	sprmPFInnerTtp  = 0x244C
	sprmPJc         = 0x2461
	sprmPOutLvl     = 0x2640
	sprmPItap       = 0x6649
	sprmPDxaRight80 = 0x840E
	sprmPDxaLeft80  = 0x840F
	sprmPDxaLeft180 = 0x8411
	sprmPDxaRight   = 0x845D
	sprmPDxaLeft    = 0x845E
	sprmPDxaLeft1   = 0x8460
	sprmPDyaBefore  = 0xA413
	sprmPDyaAfter   = 0xA414
	sprmPChgTabs    = 0xC615
	sprmTDefTable   = 0xD608
	sprmSDyaTop     = 0x9023
	sprmSDyaBottom  = 0x9024
	sprmSXaPage     = 0xB01F
	sprmSYaPage     = 0xB020
	sprmSDxaLeft    = 0xB021
	sprmSDxaRight   = 0xB022
)

// sprm is one property modifier and its operand.
type sprm struct {
	op      uint16
	operand []byte
}

// sprms iterates over the modifiers in grpprl, and stops at the first one
// that does not fit.
func sprms(grpprl []byte) iter.Seq[sprm] {
	return func(yield func(sprm) bool) {
		for len(grpprl) >= 2 {
			op := le.Uint16(grpprl)
			rest := grpprl[2:]
			n := operandSize(op, rest)
			if n < 0 || n > len(rest) {
				return
			}
			if !yield(sprm{op: op, operand: rest[:n]}) {
				return
			}
			grpprl = rest[n:]
		}
	}
}

// operandSize returns the size of the operand of op at the start of b, or
// -1 if b is too short to tell (MS-DOC 2.2.5.1).
func operandSize(op uint16, b []byte) int {
	switch op >> 13 {
	case 0, 1:
		return 1
	case 2, 4, 5:
		return 2
	case 3:
		return 4
	case 7:
		return 3
	}
	switch op {
	case sprmTDefTable:
		if len(b) < 2 {
			return -1
		}
		return int(le.Uint16(b)) + 1
	case sprmPChgTabs:
		if len(b) < 1 {
			return -1
		}
		if b[0] != 255 {
			return int(b[0]) + 1
		}
		// PChgTabsDelClose and PChgTabsAdd (MS-DOC 2.9.180)
		if len(b) < 2 {
			return -1
		}
		del := int(b[1])
		at := 2 + 4*del
		if len(b) < at+1 {
			return -1
		}
		return at + 1 + 3*int(b[at])
	}
	if len(b) < 1 {
		return -1
	}
	return int(b[0]) + 1
}

// value returns the operand of s as a number of up to 4 bytes, little
// endian.
func (s sprm) value() int64 {
	var v uint32
	for i := len(s.operand) - 1; i >= 0 && i < 4; i-- {
		v = v<<8 | uint32(s.operand[i])
	}
	switch len(s.operand) {
	case 1:
		return int64(int8(v))
	case 2:
		return int64(int16(v))
	}
	return int64(int32(v))
}

// byteValue returns the first byte of the operand of s, or 0.
func (s sprm) byteValue() byte {
	if len(s.operand) == 0 {
		return 0
	}
	return s.operand[0]
}

// chp holds the character properties that the package draws.
type chp struct {
	bold, italic, strike, underline bool
	smallCaps, caps                 bool
	hidden, deleted                 bool
	// iss is 1 for superscript and 2 for subscript.
	iss byte
	// hps is the size in half points, or 0.
	hps int
	// color and highlight are colors as 0xRRGGBB, valid when hasColor or
	// hasHighlight is set.
	color, highlight       uint32
	hasColor, hasHighlight bool
	// ftc is the index of the font in the table of fonts, or -1.
	ftc int
	// istd is the character style, or -1.
	istd int
}

// pap holds the paragraph properties that the package reads.
type pap struct {
	istd int
	// jc is the alignment: 0 left, 1 center, 2 right, 3 justify.
	jc                       byte
	inTable, ttp             bool
	innerCell, innerTtp      bool
	itap                     int
	pageBreak                bool
	left, right, first       int // twips
	before, after            int // twips
	outline                  int
	hasOutline, hasLeftFirst bool
	// widths are the widths of the cells of a row, in twips, from the
	// mark of the end of the row.
	widths []int
}

// The 16 colors of the ico of MS-DOC 2.9.119, as 0xRRGGBB.
var icoColors = [...]uint32{
	0x000000, 0x000000, 0x0000FF, 0x00FFFF, 0x00FF00, 0xFF00FF, 0xFF0000, 0xFFFF00,
	0xFFFFFF, 0x000080, 0x008080, 0x008000, 0x800080, 0x800000, 0x808000, 0x808080,
	0xC0C0C0,
}

// toggle returns the value of a toggle property of a character (MS-DOC
// 2.9.326): 0 clears it, 1 sets it, 0x80 takes the value of the style, and
// 0x81 takes the opposite of the value of the style.
func toggle(op byte, style bool) bool {
	switch op {
	case 0:
		return false
	case 1:
		return true
	case 0x80:
		return style
	case 0x81:
		return !style
	}
	return op&1 != 0
}

// applyChp applies the character modifier s to c. base holds the
// properties of the style, for the toggles.
func applyChp(c *chp, s sprm, base *chp) {
	v := s.byteValue()
	switch s.op {
	case sprmCFBold:
		c.bold = toggle(v, base.bold)
	case sprmCFItalic:
		c.italic = toggle(v, base.italic)
	case sprmCFStrike, sprmCFDStrike:
		c.strike = toggle(v, base.strike)
	case sprmCFSmallCaps:
		c.smallCaps = toggle(v, base.smallCaps)
	case sprmCFCaps:
		c.caps = toggle(v, base.caps)
	case sprmCFVanish:
		c.hidden = toggle(v, base.hidden)
	case sprmCFRMarkDel:
		c.deleted = v != 0
	case sprmCKul:
		c.underline = v != 0
	case sprmCIss:
		c.iss = v
	case sprmCHps:
		if hps := int(s.value()); hps >= 2 && hps <= 3276 {
			c.hps = hps
		}
	case sprmCIco:
		if int(v) < len(icoColors) {
			c.color, c.hasColor = icoColors[v], v != 0
		}
	case sprmCCv:
		if len(s.operand) == 4 {
			// a COLORREF: red, green, blue, and fAuto
			c.color = uint32(s.operand[0])<<16 | uint32(s.operand[1])<<8 | uint32(s.operand[2])
			c.hasColor = s.operand[3] == 0
		}
	case sprmCHighlight:
		if v > 0 && int(v) < len(icoColors) {
			c.highlight, c.hasHighlight = icoColors[v], true
		} else {
			c.hasHighlight = false
		}
	case sprmCRgFtc0:
		c.ftc = int(uint16(s.value()))
	}
}

// applyPap applies the paragraph modifier s to p.
func applyPap(p *pap, s sprm) {
	v := s.byteValue()
	switch s.op {
	case sprmPIstd:
		p.istd = int(uint16(s.value()))
	case sprmPJc, sprmPJc80:
		p.jc = v
	case sprmPFInTable:
		p.inTable = v != 0
	case sprmPFTtp:
		p.ttp = v != 0
	case sprmPFInnerCell:
		p.innerCell = v != 0
	case sprmPFInnerTtp:
		p.innerTtp = v != 0
	case sprmPItap:
		p.itap = int(s.value())
		if p.itap > 0 {
			p.inTable = true
		}
	case sprmPFPageBreak:
		p.pageBreak = v != 0
	case sprmPDxaLeft, sprmPDxaLeft80:
		p.left, p.hasLeftFirst = int(s.value()), true
	case sprmPDxaRight, sprmPDxaRight80:
		p.right = int(s.value())
	case sprmPDxaLeft1, sprmPDxaLeft180:
		p.first, p.hasLeftFirst = int(s.value()), true
	case sprmPDyaBefore:
		p.before = int(uint16(s.value()))
	case sprmPDyaAfter:
		p.after = int(uint16(s.value()))
	case sprmPOutLvl:
		p.outline, p.hasOutline = int(v), true
	case sprmTDefTable:
		// cb, the number of cells, and the n+1 edges of the cells
		// (MS-DOC 2.9.302)
		b := s.operand
		if len(b) < 3 {
			return
		}
		n := int(b[2])
		if 3+2*(n+1) > len(b) {
			return
		}
		p.widths = make([]int, n)
		for i := range n {
			p.widths[i] = int(int16(le.Uint16(b[3+2*i+2:]))) - int(int16(le.Uint16(b[3+2*i:])))
		}
	}
}
