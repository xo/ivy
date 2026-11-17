package msdoc

import (
	"context"
	"math"
	"strings"
)

// This file reads the footnotes and the endnotes of a document (MS-DOC
// 2.3.4 and 2.3.5). The layout puts a footnote at the foot of its page.
// The endnotes go at the end of the document.

// The parts of the text of a document, in the order of their characters
// (MS-DOC 2.4.1).
const (
	partMain = iota
	partFootnotes
	partHeaders
	partMacros
	partComments
	partEndnotes
	partTextboxes
	partHeaderTextboxes
)

// partRange returns the first character of the part k of the text, and
// the character after its last one.
func (fb *fib) partRange(k int) (uint32, uint32) {
	counts := [...]uint32{fb.ccpText, fb.ccpFtn, fb.ccpHdd, fb.ccpMcr, fb.ccpAtn, fb.ccpEdn, fb.ccpTxbx, fb.ccpHdrTxbx}
	var start uint64
	for i := range k {
		start += uint64(counts[i])
	}
	end := start + uint64(counts[k])
	return uint32(min(start, math.MaxUint32)), uint32(min(end, math.MaxUint32))
}

// The kinds of a note.
const (
	noteFoot = iota
	noteEnd
)

// note is a footnote or an endnote at the character of its reference: its
// kind and its paragraphs.
type note struct {
	kind  int
	paras []paragraph
}

// noteRanges reads the notes of the PlcffndRef or the PlcfendRef at the
// pair ref of the FIB, whose texts the PlcffndTxt or the PlcfendTxt at the
// pair txt gives in the part part of the text. It returns the range of the
// text of each note, by the character of its reference.
func (p *parser) noteRanges(ref, txt, part int) map[uint32][2]uint32 {
	out := map[uint32][2]uint32{}
	if p.s.word6 {
		return out
	}
	refs, texts := p.s.part(ref), p.s.part(txt)
	// the PlcffndRef holds n+1 characters and an FRD of 2 bytes for each
	// note, and the PlcffndTxt at least n+1 characters
	n := min((len(refs)-4)/6, len(texts)/4-1, p.lim.MaxObjects)
	start, end := p.s.fib.partRange(part)
	for i := range max(n, 0) {
		a, b := uint64(start)+uint64(u32(texts, 4*i)), uint64(start)+uint64(u32(texts, 4*i+4))
		if a >= b || b > uint64(end) {
			continue
		}
		out[u32(refs, 4*i)] = [2]uint32{uint32(a), uint32(b)}
	}
	return out
}

// readNote returns the note whose reference is the character cp of the
// main document, or nil.
func (p *parser) readNote(ctx context.Context, cp uint32) (*note, error) {
	if p.notes == nil {
		p.notes = []map[uint32][2]uint32{
			p.noteRanges(fcPlcffndRef, fcPlcffndTxt, partFootnotes),
			p.noteRanges(fcPlcfendRef, fcPlcfendTxt, partEndnotes),
		}
	}
	for kind, m := range p.notes {
		r, ok := m[cp]
		if !ok {
			continue
		}
		paras, err := p.paragraphsIn(ctx, r[0], r[1])
		if err != nil {
			return nil, err
		}
		return &note{kind: kind, paras: paras}, nil
	}
	return nil, nil
}

// writeNote writes the reference of a note. A footnote goes in the flow, in
// an element that the layout puts at the foot of the page. An endnote
// writes its number, in lower case Roman as in Word, and its text goes to
// the end of the document.
func (p *parser) writeNote(b *strings.Builder, n *note) {
	saved, savedInline := p.mark, p.inline
	defer func() {
		p.mark, p.inline = saved, savedInline
	}()
	if n.kind == noteFoot {
		p.mark, p.inline = `<span class="fnref"></span>`, true
		b.WriteString(`<span class="fn">`)
		for _, para := range n.paras {
			if !para.pap.ttp {
				p.writeParagraph(b, para)
			}
		}
		b.WriteString("</span>")
		return
	}
	p.endnoteCount++
	mark := roman(p.endnoteCount, false)
	b.WriteString(`<span class="enref">` + mark + "</span>")
	// the number at the start of the text of the endnote has the
	// properties of its characters, which are superscript
	p.mark, p.inline = mark, false
	p.endnotes.WriteString(p.body(n.paras, nil))
}
