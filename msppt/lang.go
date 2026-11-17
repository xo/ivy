package msppt

import (
	"github.com/xo/ivy/internal/fontdb"
)

// The records of the special information of a text (MS-PPT 2.9.68 and
// 2.9.69).
const (
	rtTextSpecialInfoAtom        = 0x0FAA
	rtTextSpecialInfoDefaultAtom = 0x0FAC
)

// The bits of the masks of a TextSIException (MS-PPT 2.9.70).
const (
	siSpell    = 0x0001
	siLang     = 0x0002
	siAltLang  = 0x0004
	siPp10ext  = 0x0020
	siBidi     = 0x0040
	siSmartTag = 0x0200
)

// siRun is a run of a TextSpecialInfoAtom: its number of characters and
// the East Asian language of its characters, from lang or altLang, or "".
type siRun struct {
	count int
	lang  string
}

// readSI reads a TextSIException at the start of b, and returns the East
// Asian language that it gives, its size, and false if b cuts it. The
// language is lang when it is East Asian, or else altLang, as Office takes
// altLang for the East Asian text of a run in a language of the West.
func readSI(b []byte) (string, int, bool) {
	r := reader{b: b}
	m := r.u32()
	if m&siSpell != 0 {
		r.u16()
	}
	var lang, alt uint16
	if m&siLang != 0 {
		lang = r.u16()
	}
	if m&siAltLang != 0 {
		alt = r.u16()
	}
	if m&siBidi != 0 {
		r.u16()
	}
	if m&siPp10ext != 0 {
		r.u32()
	}
	if m&siSmartTag != 0 {
		n := int(r.u32())
		if n > len(b)/4 {
			return "", 0, false
		}
		r.off += 4 * n
	}
	if r.bad || r.off > len(b) {
		return "", 0, false
	}
	l := fontdb.LangOfLCID(lang)
	if l == fontdb.NoLang {
		l = fontdb.LangOfLCID(alt)
	}
	return l.String(), r.off, true
}

// readSIRuns reads the runs of a TextSpecialInfoAtom of a text of n
// characters.
func readSIRuns(b []byte, n int) []siRun {
	var runs []siRun
	off := 0
	for total := 0; total <= n && off+4 <= len(b) && len(runs) < n+1; {
		count := int(le.Uint32(b[off:]))
		lang, size, ok := readSI(b[off+4:])
		if !ok || count <= 0 {
			break
		}
		off += 4 + size
		runs = append(runs, siRun{count: count, lang: lang})
		total += count
	}
	return runs
}

// langAt returns the East Asian language of the character i of the text,
// and the offset where its run ends. A character without a run takes the
// language def.
func (t *text) langAt(i int, def string) (string, int) {
	end := 0
	for _, r := range t.si {
		end += r.count
		if i < end {
			return r.lang, end
		}
	}
	return def, len(t.units) + 1
}
