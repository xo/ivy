package msdoc

import (
	"iter"
)

// This file reads the properties of the characters and the paragraphs of a
// document of Word 6 or Word 95. Their modifiers have codes of one byte,
// which the iteration gives with the codes of Word 97, and their formatted
// disk pages and their style sheet differ a little from those of Word 97.

// The pairs of offsets and sizes of the FIB of Word 6 that the package
// reads, at their offsets in the FIB.
const (
	word6Stshf        = 0x60
	word6PlcfBteChpx  = 0xB8
	word6PlcfBtePapx  = 0xC0
	word6SttbfFfn     = 0xD0
	word6BxSize       = 7
	word6StdBaseSize  = 8
	word6SprmVariable = -1
	word6SprmTable    = -2
	// word6SprmCSymbol is sprmCSymbol of Word 6. Its operand is its size,
	// 3, then the index of the font in 2 bytes and the character in 1
	// byte, where the CSymbolOperand of Word 97 has the character in 2
	// bytes. Sprm 73 before it is sprmCChse, of 3 bytes.
	word6SprmCSymbol = 74
)

// word6Sprms97 maps the codes of the modifiers of Word 6 that the package
// reads to the codes of Word 97.
var word6Sprms97 = map[byte]uint16{
	2: sprmPIstd, 5: sprmPJc, 9: sprmPFPageBreak, 14: sprmPFNoLineNumb, 15: sprmPChgTabsPapx, 16: sprmPDxaRight, 17: sprmPDxaLeft,
	19: sprmPDxaLeft1, 20: sprmPDyaLine, 21: sprmPDyaBefore, 22: sprmPDyaAfter, 23: sprmPChgTabs,
	24: sprmPFInTable, 25: sprmPFTtp,
	71: sprmCFData, word6SprmCSymbol: sprmCSymbol, 80: sprmCIstd, 85: sprmCFBold, 86: sprmCFItalic, 87: sprmCFStrike,
	90: sprmCFSmallCaps, 91: sprmCFCaps, 92: sprmCFVanish, 93: sprmCRgFtc0, 94: sprmCKul,
	96: sprmCDxaSpace, 98: sprmCIco, 99: sprmCHps, 104: sprmCIss, 117: sprmCFSpec,
}

// word6SprmSizes holds the sizes of the operands of the modifiers of the
// paragraphs, the characters, the pictures and the tables of Word 6:
// a size, 0 for no operand, word6SprmVariable for a size in the first
// byte, and word6SprmTable for a size in the first 2 bytes. A code that is
// not in the table stops the iteration, as its size is not known.
var word6SprmSizes = map[byte]int{
	2: 2, 3: word6SprmVariable, 4: 1, 5: 1, 6: 1, 7: 1, 8: 1, 9: 1, 10: 1, 11: 1,
	12: word6SprmVariable, 13: 1, 14: 1, 15: word6SprmVariable, 16: 2, 17: 2, 18: 2,
	19: 2, 20: 4, 21: 2, 22: 2, 23: word6SprmVariable, 24: 1, 25: 1, 26: 2, 27: 2,
	28: 2, 29: 1, 30: 2, 31: 2, 32: 2, 33: 2, 34: 2, 35: 2, 36: 2, 37: 1, 38: 2,
	39: 2, 40: 2, 41: 2, 42: 2, 43: 2, 44: 1, 45: 2, 46: 2, 47: 2, 48: 2, 49: 2,
	50: 1, 51: 1, 52: word6SprmVariable,
	65: 1, 66: 1, 67: 1, 68: word6SprmVariable, 69: 2, 70: 4, 71: 1, 72: 2, 73: 3,
	74: word6SprmVariable, 75: 1, 80: 2, 81: word6SprmVariable, 82: 0, 83: 0, 85: 1,
	86: 1, 87: 1, 88: 1, 89: 1, 90: 1, 91: 1, 92: 1, 93: 2, 94: 1, 95: 3, 96: 2,
	97: 2, 98: 1, 99: 2, 100: 1, 101: 2, 102: 1, 103: word6SprmVariable, 104: 1,
	105: word6SprmVariable, 106: word6SprmVariable, 107: 2,
	108: word6SprmVariable, 109: 2, 110: 2, 117: 1, 118: 1,
	119: 1, 120: word6SprmVariable, 121: 2, 122: 2, 123: 2, 124: 2,
	182: 2, 183: 2, 184: 2, 185: 1, 186: 1, 187: 12, 188: word6SprmVariable, 189: 2,
	190: word6SprmTable, 191: word6SprmVariable, 192: 4, 193: 5, 194: 4, 195: 2,
	196: 4, 197: 2, 198: 2, 199: 5, 200: 4,
}

// word6Sprms iterates over the modifiers of the paragraphs and of the
// characters of Word 6 or Word 95 in grpprl, with the codes of Word 97. It
// gives a modifier that the package does not read with the code 0.
func word6Sprms(grpprl []byte) iter.Seq[sprm] {
	return func(yield func(sprm) bool) {
		for len(grpprl) >= 1 {
			code := grpprl[0]
			rest := grpprl[1:]
			n, ok := word6SprmSizes[code]
			if !ok {
				return
			}
			switch n {
			case word6SprmVariable:
				if len(rest) < 1 {
					return
				}
				n = 1 + int(rest[0])
			case word6SprmTable:
				if len(rest) < 2 {
					return
				}
				n = int(le.Uint16(rest)) + 1
			}
			if n > len(rest) {
				return
			}
			s := sprm{op: word6Sprms97[code], operand: rest[:n]}
			if code == word6SprmCSymbol {
				s.operand = nil
				if n == 4 {
					s.operand = []byte{rest[1], rest[2], rest[3], 0}
				}
			}
			if !yield(s) {
				return
			}
			grpprl = rest[n:]
		}
	}
}

// sprmsOf iterates over the modifiers of grpprl of a document of Word 97,
// or of Word 6 or Word 95 when word6 is set.
func sprmsOf(word6 bool, grpprl []byte) iter.Seq[sprm] {
	if word6 {
		return word6Sprms(grpprl)
	}
	return sprms(grpprl)
}

// readWord6Properties reads the formatted disk pages, the style sheet and
// the fonts of a document of Word 6 or Word 95, from the pairs of offsets
// and sizes of its FIB. A part that is missing gives no properties.
func (p *parser) readWord6Properties() {
	word := p.s.word
	pair := func(off int) []byte {
		fc, lcb := uint64(u32(word, off)), uint64(u32(word, off+4))
		if lcb == 0 || fc+lcb > uint64(len(word)) {
			return nil
		}
		return word[fc : fc+lcb]
	}
	p.chpx = p.fkpRuns(pair(word6PlcfBteChpx), false, 2)
	p.papx = p.fkpRuns(pair(word6PlcfBtePapx), true, 2)
	p.styles = parseStylesOf(pair(word6Stshf), p.lim.MaxObjects, true)
	p.fonts = parseWord6Fonts(pair(word6SttbfFfn), p.lim.MaxObjects)
}

// parseWord6Fonts reads the fonts of the table of fonts of Word 6: its size
// in 2 bytes, then FFNs whose names are in single bytes from their byte 6.
func parseWord6Fonts(buf []byte, maxFonts int) []font {
	if len(buf) < 2 {
		return nil
	}
	end := min(int(le.Uint16(buf)), len(buf))
	var out []font
	for off := 2; off < end && len(out) < maxFonts; {
		cb := int(buf[off]) + 1
		ffn := buf[off:min(off+cb, end)]
		off += cb
		f := font{generic: "serif"}
		if len(ffn) > 1 {
			f.generic = generics[ffn[1]>>4&7]
		}
		if len(ffn) > 6 {
			name := ffn[6:]
			for i, c := range name {
				if c == 0 {
					name = name[:i]
					break
				}
			}
			// the name is in code page 1252, whose letters of a name are
			// those of Latin-1
			r := make([]rune, len(name))
			for i, c := range name {
				r[i] = rune(c)
			}
			f.name = string(r)
		}
		out = append(out, f)
	}
	return out
}
