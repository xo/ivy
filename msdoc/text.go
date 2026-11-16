package msdoc

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/charset"
)

// piece is a piece of the piece table: a run of characters in the stream
// WordDocument (MS-DOC 2.9.177).
type piece struct {
	cpStart, cpEnd uint32
	fc             uint32
	compressed     bool
	prm            uint16
}

// fkpRun is a run of the formatted disk pages: the bytes [fcStart, fcEnd) of
// the stream WordDocument have the properties grpprl. For a paragraph, istd
// is its style.
type fkpRun struct {
	fcStart, fcEnd uint32
	istd           int
	grpprl         []byte
}

// run is a run of text of a paragraph with its character properties.
type run struct {
	text string
	chp  chp
}

// paragraph is a paragraph of the main document.
type paragraph struct {
	runs []run
	pap  pap
	// end is the character that ends the paragraph: 0x0D, or 0x07 for the
	// end of a cell or of a row of a table.
	end byte
	// pageBreak is set when a page break or a section break comes before
	// the paragraph.
	pageBreak bool
}

// parser reads the text and the properties of a document.
type parser struct {
	s      *streams
	lim    ivy.Limits
	pieces []piece
	grpprl [][]byte
	chpx   []fkpRun
	papx   []fkpRun
	styles *styleSheet
	fonts  []font
	// ansi maps a byte of a compressed piece to its character.
	ansi [256]rune
}

// newParser reads the piece table, the formatted disk pages, the styles and
// the fonts of the document.
func newParser(s *streams, lim ivy.Limits) (*parser, error) {
	p := &parser{s: s, lim: lim}
	if err := p.readClx(); err != nil {
		return nil, err
	}
	var err error
	if p.chpx, err = p.readFkps(fcPlcfBteChpx, false); err != nil {
		return nil, err
	}
	if p.papx, err = p.readFkps(fcPlcfBtePapx, true); err != nil {
		return nil, err
	}
	p.styles = parseStyles(s.part(fcStshf), lim.MaxObjects)
	p.fonts = parseFonts(s.part(fcSttbfFfn), lim.MaxObjects)
	// a compressed piece holds the characters of code page 1252 (MS-DOC
	// 2.4.1)
	var all [256]byte
	for i := range all {
		all[i] = byte(i)
	}
	text, _ := charset.Decode(all[:], "windows-1252")
	i := 0
	for _, r := range text {
		if i < len(p.ansi) {
			p.ansi[i] = r
		}
		i++
	}
	return p, nil
}

// readClx reads the CLX of the table stream: the property modifiers of the
// pieces, and the piece table (MS-DOC 2.9.38).
func (p *parser) readClx() error {
	clx := p.s.part(fcClx)
	if clx == nil {
		return fmt.Errorf("word document without a piece table: %w", ivy.ErrFormat)
	}
	for len(clx) > 0 && clx[0] == 0x01 {
		if len(clx) < 3 {
			return fmt.Errorf("damaged prc of the clx: %w", ivy.ErrFormat)
		}
		n := int(le.Uint16(clx[1:]))
		if 3+n > len(clx) {
			return fmt.Errorf("prc of %d bytes: %w", n, ivy.ErrFormat)
		}
		if len(p.grpprl) >= p.lim.MaxObjects {
			return fmt.Errorf("clx of more than %d prcs: %w", p.lim.MaxObjects, ivy.ErrLimit)
		}
		p.grpprl = append(p.grpprl, clx[3:3+n])
		clx = clx[3+n:]
	}
	if len(clx) < 5 || clx[0] != 0x02 {
		return fmt.Errorf("clx without a pcdt: %w", ivy.ErrFormat)
	}
	lcb := int(le.Uint32(clx[1:]))
	plc := clx[5:]
	if lcb < 4 || lcb > len(plc) {
		return fmt.Errorf("piece table of %d bytes: %w", lcb, ivy.ErrFormat)
	}
	plc = plc[:lcb]
	// n+1 character positions and n pieces of 8 bytes
	n := (lcb - 4) / 12
	if n > p.lim.MaxObjects {
		return fmt.Errorf("piece table of %d pieces: %w", n, ivy.ErrLimit)
	}
	p.pieces = make([]piece, 0, n)
	for i := range n {
		cpStart, cpEnd := le.Uint32(plc[4*i:]), le.Uint32(plc[4*i+4:])
		pcd := plc[4*(n+1)+8*i:]
		fc := le.Uint32(pcd[2:])
		pc := piece{cpStart: cpStart, cpEnd: cpEnd, prm: le.Uint16(pcd[6:])}
		if fc&0x40000000 != 0 {
			pc.compressed, pc.fc = true, (fc&0x3FFFFFFF)/2
		} else {
			pc.fc = fc & 0x3FFFFFFF
		}
		if cpEnd <= cpStart {
			continue
		}
		p.pieces = append(p.pieces, pc)
	}
	return nil
}

// readFkps reads the formatted disk pages of the characters, or of the
// paragraphs, from the PlcBte at the pair i of the FIB (MS-DOC 2.8.25 and
// 2.8.26). Each page has 512 bytes in the stream WordDocument.
func (p *parser) readFkps(i int, para bool) ([]fkpRun, error) {
	plc := p.s.part(i)
	if len(plc) < 4 {
		return nil, nil
	}
	n := (len(plc) - 4) / 8
	if n > p.lim.MaxObjects {
		return nil, fmt.Errorf("%d formatted disk pages: %w", n, ivy.ErrLimit)
	}
	var runs []fkpRun
	word := p.s.word
	for k := range n {
		pn := le.Uint32(plc[4*(n+1)+4*k:]) & 0x3FFFFF
		off := int64(pn) * 512
		if off+512 > int64(len(word)) {
			continue
		}
		page := word[off : off+512]
		count := int(page[511])
		if para {
			// rgfc, then rgbx of 13 bytes each
			if 4*(count+1)+13*count > 511 {
				continue
			}
			for j := range count {
				r := fkpRun{fcStart: le.Uint32(page[4*j:]), fcEnd: le.Uint32(page[4*j+4:]), istd: 0}
				bOff := 2 * int(page[4*(count+1)+13*j])
				if bOff > 0 && bOff < 511 {
					cb := 2 * int(page[bOff])
					start := bOff + 1
					if cb == 0 && bOff+1 < 511 {
						cb = 2 * int(page[bOff+1])
						start = bOff + 2
					} else {
						cb--
					}
					if cb >= 2 && start+cb <= 511 {
						r.istd = int(le.Uint16(page[start:]))
						r.grpprl = page[start+2 : start+cb]
					}
				}
				runs = append(runs, r)
			}
		} else {
			// rgfc, then rgb of one byte each
			if 4*(count+1)+count > 511 {
				continue
			}
			for j := range count {
				r := fkpRun{fcStart: le.Uint32(page[4*j:]), fcEnd: le.Uint32(page[4*j+4:])}
				bOff := 2 * int(page[4*(count+1)+j])
				if bOff > 0 && bOff < 511 {
					cb := int(page[bOff])
					if bOff+1+cb <= 511 {
						r.grpprl = page[bOff+1 : bOff+1+cb]
					}
				}
				runs = append(runs, r)
			}
		}
		if len(runs) > p.lim.MaxObjects {
			return nil, fmt.Errorf("more than %d runs of properties: %w", p.lim.MaxObjects, ivy.ErrLimit)
		}
	}
	slices.SortStableFunc(runs, func(a, b fkpRun) int {
		switch {
		case a.fcStart < b.fcStart:
			return -1
		case a.fcStart > b.fcStart:
			return 1
		}
		return 0
	})
	return runs, nil
}

// find returns the run of runs that holds the byte fc, or nil.
func find(runs []fkpRun, fc uint32) *fkpRun {
	i := sort.Search(len(runs), func(i int) bool { return runs[i].fcStart > fc })
	if i == 0 {
		return nil
	}
	r := &runs[i-1]
	if fc >= r.fcEnd {
		return nil
	}
	return r
}

// pieceGrpprl returns the property modifiers of the Prm of a piece (MS-DOC
// 2.2.6.4). A Prm of one modifier is not read, as only fast saves write it.
func (p *parser) pieceGrpprl(prm uint16) []byte {
	if prm&1 == 0 {
		return nil
	}
	i := int(prm >> 1)
	if i >= len(p.grpprl) {
		return nil
	}
	return p.grpprl[i]
}

// paraProps returns the properties of the paragraph whose mark is the byte
// fc, with the modifiers of its piece.
func (p *parser) paraProps(fc uint32, pieceMods []byte) pap {
	r := find(p.papx, fc)
	istd := 0
	var grpprl []byte
	if r != nil {
		istd, grpprl = r.istd, r.grpprl
	}
	var pp pap
	if st := p.styles.resolve(istd, p.lim.MaxDepth); st != nil && st.stk == stkParagraph {
		pp = st.pap
	}
	pp.istd = istd
	for s := range sprms(grpprl) {
		applyPap(&pp, s)
	}
	for s := range sprms(pieceMods) {
		applyPap(&pp, s)
	}
	return pp
}

// charProps returns the properties of the characters of the run r of the
// formatted disk pages, which can be nil, in a paragraph of the style istd,
// with the modifiers of their piece.
func (p *parser) charProps(r *fkpRun, istd int, pieceMods []byte) chp {
	base := defaultChp()
	base.ftc = p.styles.ftc
	if st := p.styles.resolve(istd, p.lim.MaxDepth); st != nil && st.stk == stkParagraph {
		base = st.chp
	}
	var grpprl []byte
	if r != nil {
		grpprl = r.grpprl
	}
	// a character style comes before the other modifiers
	for s := range sprms(grpprl) {
		if s.op != sprmCIstd {
			continue
		}
		if st := p.styles.resolve(int(uint16(s.value())), p.lim.MaxDepth); st != nil && st.stk == stkCharacter {
			cs := base
			for m := range sprms(st.chpx) {
				applyChp(&cs, m, &base)
			}
			base = cs
		}
	}
	c := base
	for s := range sprms(grpprl) {
		applyChp(&c, s, &base)
	}
	for s := range sprms(pieceMods) {
		applyChp(&c, s, &base)
	}
	return c
}

// char is a character of the main document and the byte of the stream
// WordDocument that holds it.
type char struct {
	r     rune
	fc    uint32
	piece int
}

// chars iterates over the characters of the main document, from 0 to
// ccpText, in the order of the pieces. The pieces of a valid document do not
// overlap, so the document has fewer characters than the stream
// WordDocument has bytes. The iteration stops there, so that pieces that
// repeat the same bytes cannot make it long.
func (p *parser) chars(yield func(char) bool) {
	limit := p.s.fib.ccpText
	word := p.s.word
	budget := len(word)
	for i, pc := range p.pieces {
		if pc.cpStart >= limit {
			return
		}
		end := min(pc.cpEnd, limit)
		for cp := pc.cpStart; cp < end; cp++ {
			if budget--; budget < 0 {
				return
			}
			k := cp - pc.cpStart
			var c char
			c.piece = i
			if pc.compressed {
				c.fc = pc.fc + k
				if uint64(c.fc) >= uint64(len(word)) {
					break
				}
				c.r = p.ansi[word[c.fc]]
			} else {
				c.fc = pc.fc + 2*k
				if uint64(c.fc)+2 > uint64(len(word)) {
					break
				}
				c.r = rune(le.Uint16(word[c.fc:]))
				if c.r >= 0xD800 && c.r < 0xDC00 && uint64(c.fc)+4 <= uint64(len(word)) {
					lo := rune(le.Uint16(word[c.fc+2:]))
					if lo >= 0xDC00 && lo < 0xE000 && cp+1 < end {
						c.r = 0x10000 + (c.r-0xD800)<<10 + (lo - 0xDC00)
						if !yield(c) {
							return
						}
						cp++
						continue
					}
				}
			}
			if !yield(c) {
				return
			}
		}
	}
}

// paragraphs reads the text of the main document into paragraphs. It drops
// the codes of the fields and keeps their results, and drops the hidden
// and the deleted text.
func (p *parser) paragraphs(ctx context.Context) ([]paragraph, error) {
	var (
		paras   []paragraph
		cur     paragraph
		text    strings.Builder
		runChp  chp
		haveRun bool
		fields  []bool // for each open field, whether its code is read
		pending []char
		total   int
	)
	flushRun := func() {
		if haveRun && text.Len() > 0 {
			cur.runs = append(cur.runs, run{text: text.String(), chp: runChp})
		}
		text.Reset()
		haveRun = false
	}
	inCode := func() bool {
		return slices.Contains(fields, true)
	}
	endPara := func(mark char) {
		mods := p.pieceGrpprl(p.pieces[mark.piece].prm)
		cur.pap = p.paraProps(mark.fc, mods)
		// the properties of the characters wait for those of the paragraph,
		// which come at its end
		// the characters of one run of one piece share their properties
		var (
			lastRun   *fkpRun
			lastPiece = -1
			cp        chp
		)
		for _, c := range pending {
			if r := find(p.chpx, c.fc); r != lastRun || c.piece != lastPiece || lastPiece < 0 {
				cp = p.charProps(r, cur.pap.istd, p.pieceGrpprl(p.pieces[c.piece].prm))
				lastRun, lastPiece = r, c.piece
			}
			if cp.hidden || cp.deleted {
				continue
			}
			s, ok := visible(c.r)
			if !ok {
				continue
			}
			if !haveRun || cp != runChp {
				flushRun()
				runChp, haveRun = cp, true
			}
			text.WriteString(s)
		}
		flushRun()
		pending = pending[:0]
		cur.end = byte(mark.r)
		paras = append(paras, cur)
		cur = paragraph{}
	}
	for c := range p.chars {
		total++
		if total%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("reading word document: %w", err)
			}
		}
		switch c.r {
		case 0x13:
			if len(fields) < p.lim.MaxDepth {
				fields = append(fields, true)
			}
			continue
		case 0x14:
			if len(fields) > 0 {
				fields[len(fields)-1] = false
			}
			continue
		case 0x15:
			if len(fields) > 0 {
				fields = fields[:len(fields)-1]
			}
			continue
		case 0x0D, 0x07:
			endPara(c)
			if len(paras) > p.lim.MaxObjects {
				return nil, fmt.Errorf("more than %d paragraphs: %w", p.lim.MaxObjects, ivy.ErrLimit)
			}
			continue
		case 0x0C:
			if inCode() {
				continue
			}
			// a page break ends the text before it, and the text after it
			// starts on a new page
			if len(pending) > 0 {
				mark := c
				mark.r = 0x0D
				endPara(mark)
			}
			cur.pageBreak = true
			continue
		}
		if inCode() {
			continue
		}
		pending = append(pending, c)
	}
	if len(pending) > 0 {
		last := pending[len(pending)-1]
		last.r = 0x0D
		endPara(last)
	}
	return paras, nil
}

// visible returns the text of the character r in a paragraph, or false for
// a character that has no text, such as the anchor of a picture.
func visible(r rune) (string, bool) {
	switch r {
	case 0x0B, 0x0E, '\n':
		return "\n", true
	case 0x09:
		return "\t", true
	case 0x1E:
		return "\u2011", true
	case 0x1F:
		return "\u00AD", true
	}
	if r < 0x20 || r == 0xFFFE || r == 0xFFFF || !utf8.ValidRune(r) {
		return "", false
	}
	return string(r), true
}
