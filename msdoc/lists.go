package msdoc

import (
	"strconv"
	"strings"
	"unicode/utf16"
)

// This file reads the lists of a document of Word 97: the PlfLst, with the
// levels of each list, and the PlfLfo, which the paragraphs name (MS-DOC
// 2.9.150 and 2.9.131), and writes the number or the bullet of each
// paragraph of a list.

// The sprms of the lists of a paragraph.
const (
	sprmPIlvl = 0x260A
	sprmPIlfo = 0x460B
)

// The sizes of the structures of the lists.
const (
	lstfSize = 28
	lvlfSize = 28
	lfoSize  = 16
)

// nfcNone and nfcBullet are the formats of a number that show no number,
// and a bullet (MS-DOC 2.9.158).
const (
	nfcBullet = 23
	nfcNone   = 255
)

// listLevel is a level of a list: its first number, the format of its
// numbers, its text with the placeholders of the numbers of the levels,
// what follows the number, and the properties of the characters of the
// number.
type listLevel struct {
	start  int
	nfc    byte
	legal  bool
	text   []uint16
	follow byte
	chpx   []byte
	// papx holds the modifiers of the paragraphs of the level, such as
	// their indents and their tab stops.
	papx []byte
	// restart is set when the level starts again after a level above it.
	restart bool
}

// list is a list of the PlfLst: its id and its levels.
type list struct {
	lsid   uint32
	levels []listLevel
}

// lists holds the lists of a document, the lsid of each LFO, and the
// counters of the numbers of each list.
type lists struct {
	byID     map[uint32]*list
	lfos     []uint32
	counters map[uint32]*[9]int
	started  map[uint32]*[9]bool
}

// readLists reads the PlfLst, with the LVLs that follow it in the table
// stream, and the PlfLfo. A damaged part gives fewer lists.
func (p *parser) readLists() *lists {
	ls := &lists{byID: map[uint32]*list{}, counters: map[uint32]*[9]int{}, started: map[uint32]*[9]bool{}}
	if p.s.word6 || len(p.s.fib.fcLcb) <= fcPlfLfo {
		return ls
	}
	table := p.s.table
	fc, lcb := int64(p.s.fib.fcLcb[fcPlfLst][0]), int64(p.s.fib.fcLcb[fcPlfLst][1])
	if lcb >= 2 && fc+lcb <= int64(len(table)) {
		n := int(int16(le.Uint16(table[fc:])))
		off := fc + 2
		var lstfs [][]byte
		for i := 0; i < n && i < p.lim.MaxObjects && off+lstfSize <= fc+lcb; i++ {
			lstfs = append(lstfs, table[off:off+lstfSize])
			off += lstfSize
		}
		// the LVLs follow the PlfLst
		for _, lstf := range lstfs {
			l := &list{lsid: le.Uint32(lstf)}
			count := 9
			if lstf[26]&1 != 0 {
				count = 1
			}
			for range count {
				lvl, next, ok := readLevel(table, off)
				if !ok {
					break
				}
				l.levels = append(l.levels, lvl)
				off = next
			}
			ls.byID[l.lsid] = l
		}
	}
	fc, lcb = int64(p.s.fib.fcLcb[fcPlfLfo][0]), int64(p.s.fib.fcLcb[fcPlfLfo][1])
	if lcb >= 4 && fc+lcb <= int64(len(table)) {
		n := int64(le.Uint32(table[fc:]))
		for i := int64(0); i < n && i < int64(p.lim.MaxObjects) && fc+4+lfoSize*(i+1) <= fc+lcb; i++ {
			ls.lfos = append(ls.lfos, le.Uint32(table[fc+4+lfoSize*i:]))
		}
	}
	return ls
}

// readLevel reads the LVL at off in table, and returns the offset after it.
func readLevel(table []byte, off int64) (listLevel, int64, bool) {
	if off+lvlfSize > int64(len(table)) {
		return listLevel{}, 0, false
	}
	lvlf := table[off : off+lvlfSize]
	lvl := listLevel{
		start:   int(int32(le.Uint32(lvlf))),
		nfc:     lvlf[4],
		legal:   lvlf[5]&0x04 != 0,
		follow:  lvlf[15],
		restart: lvlf[5]&0x08 == 0,
	}
	cbChpx, cbPapx := int64(lvlf[24]), int64(lvlf[25])
	off += lvlfSize
	if off+cbPapx+cbChpx+2 > int64(len(table)) {
		return listLevel{}, 0, false
	}
	lvl.papx = table[off : off+cbPapx]
	off += cbPapx
	lvl.chpx = table[off : off+cbChpx]
	off += cbChpx
	cch := int64(le.Uint16(table[off:]))
	off += 2
	if cch > 256 || off+2*cch > int64(len(table)) {
		return listLevel{}, 0, false
	}
	for i := range cch {
		lvl.text = append(lvl.text, le.Uint16(table[off+2*i:]))
	}
	return lvl, off + 2*cch, true
}

// list returns the list of the LFO ilfo, with a level ilvl, or nil.
func (p *parser) list(ilfo, ilvl int) *list {
	ls := p.lists
	if ls == nil || ilfo < 1 || ilfo > len(ls.lfos) || ilvl < 0 || ilvl > 8 {
		return nil
	}
	l, ok := ls.byID[ls.lfos[ilfo-1]]
	if !ok || len(l.levels) == 0 {
		return nil
	}
	return l
}

// level returns the level ilvl of the list of the LFO ilfo, or nil. A
// level after the last level of the list is its last level.
func (p *parser) level(ilfo, ilvl int) *listLevel {
	l := p.list(ilfo, ilvl)
	if l == nil {
		return nil
	}
	return &l.levels[min(ilvl, len(l.levels)-1)]
}

// marker returns the number or the bullet of a paragraph of the list of the
// LFO ilfo, at the level ilvl, and the properties of its characters over
// base. It counts the paragraph in the list. It returns false for a
// paragraph that shows no number.
func (p *parser) marker(ilfo, ilvl int, base chp) (string, chp, bool) {
	l := p.list(ilfo, ilvl)
	if l == nil {
		return "", chp{}, false
	}
	ls, lsid := p.lists, l.lsid
	ilvl = min(ilvl, len(l.levels)-1)
	counts, started := ls.counters[lsid], ls.started[lsid]
	if counts == nil {
		counts, started = &[9]int{}, &[9]bool{}
		ls.counters[lsid], ls.started[lsid] = counts, started
	}
	lvl := l.levels[ilvl]
	if started[ilvl] {
		counts[ilvl]++
	} else {
		counts[ilvl], started[ilvl] = lvl.start, true
	}
	// a level after this one starts again
	for k := ilvl + 1; k < 9; k++ {
		if k < len(l.levels) && l.levels[k].restart {
			started[k] = false
		}
	}
	c := base
	for s := range sprms(lvl.chpx) {
		applyChp(&c, s, &base)
	}
	if lvl.nfc == nfcNone {
		return "", chp{}, false
	}
	var b strings.Builder
	for _, u := range utf16.Decode(lvl.text) {
		switch {
		case u < 9:
			k := int(u)
			if k >= len(l.levels) {
				continue
			}
			nfc := l.levels[k].nfc
			if lvl.legal {
				nfc = 0
			}
			n := counts[k]
			if !started[k] {
				n = l.levels[k].start
			}
			b.WriteString(formatNumber(n, nfc))
		case u >= 0xF000 && u <= 0xF0FF:
			b.WriteRune(symbolRune(u - 0xF000))
		default:
			b.WriteRune(u)
		}
	}
	switch lvl.follow {
	case 0:
		b.WriteByte('\t')
	case 1:
		b.WriteByte(' ')
	}
	return b.String(), c, true
}

// symbolRune returns the character of the code c of a font of symbols, such
// as Symbol or Wingdings, that Word uses for bullets. Another code stays.
func symbolRune(c rune) rune {
	switch c {
	case 0xB7:
		return '•'
	case 0xA7:
		return '▪'
	case 0xFC:
		return '✓'
	case 0xD8:
		return '➢'
	case 0x76:
		return '❖'
	case 0x6E:
		return '■'
	case 0x71:
		return '❏'
	}
	return c
}

// formatNumber returns the number n in the format nfc (MS-DOC 2.9.158):
// decimal, roman or letters. Another format is decimal.
func formatNumber(n int, nfc byte) string {
	switch nfc {
	case 1:
		return roman(n, true)
	case 2:
		return roman(n, false)
	case 3:
		return letters(n, 'A')
	case 4:
		return letters(n, 'a')
	case 22:
		if n < 10 && n >= 0 {
			return "0" + strconv.Itoa(n)
		}
	case nfcBullet:
		return ""
	}
	return strconv.Itoa(n)
}

// roman returns n in roman numerals, or in decimal out of 1 to 3999.
func roman(n int, upper bool) string {
	if n < 1 || n > 3999 {
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
	if upper {
		return strings.ToUpper(b.String())
	}
	return b.String()
}

// letters returns n as letters from first: a to z, then aa to zz, as in
// Word.
func letters(n int, first rune) string {
	if n < 1 {
		return strconv.Itoa(n)
	}
	n--
	return strings.Repeat(string(first+rune(n%26)), min(n/26+1, 32))
}
