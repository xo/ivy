package msdoc

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/charset"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/msole"
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
	// pic is the picture of the run, when the run is a picture and not
	// text.
	pic *picture
	// drawn is a picture, a text box or a shape of Office Drawing that
	// floats on the page, note the footnote or the endnote of a reference,
	// and noteMark is set for the number of a note at the start of its
	// text.
	drawn    *drawn
	note     *note
	noteMark bool
}

// The characters of the private use area that stand for the result of a
// field that the layout writes: the number of the page and the number of
// the pages.
const (
	runePage  = '\uE001'
	runePages = '\uE002'
)

// field is a field of the text, from its character 0x13 to its character
// 0x15.
type field struct {
	// code is set while the characters are those of the code of the field,
	// before its character 0x14.
	code  bool
	instr strings.Builder
	// skip drops the result of the field, which the layout writes itself.
	skip bool
}

// paragraph is a paragraph of the main document.
type paragraph struct {
	runs []run
	pap  pap
	// end is the character that ends the paragraph: 0x0D, or 0x07 for the
	// end of a cell or of a row of a table.
	end byte
	// pageBreak is set when a page break or a section break comes before
	// the paragraph, and columnBreak when a column break comes before it.
	pageBreak, columnBreak bool
	// cp is the character position of the mark of the paragraph.
	cp uint32
	// mark holds the character properties of the mark of the paragraph,
	// which give an empty paragraph the height of its line.
	mark chp
}

// parser reads the text and the properties of a document.
type parser struct {
	s      *streams
	cfg    ivy.Config
	lim    ivy.Limits
	pieces []piece
	grpprl [][]byte
	chpx   []fkpRun
	papx   []fkpRun
	styles *styleSheet
	fonts  []font
	// ansi maps a byte of a compressed piece to its character.
	ansi [256]rune
	// pictures holds the pictures by their offset in the stream Data,
	// media their files, and objects the objects of the shapes of Office
	// Drawing.
	pictures map[uint32]*picture
	media    []media
	objects  map[string]layout.Object
	// lists holds the lists of a document of Word 97, or nil.
	lists *lists
	// page is the page of the section that the conversion writes, and
	// columns the number of its columns.
	page    page
	columns int
	// notes holds the ranges of the text of the footnotes and of the
	// endnotes by the characters of their references, once read.
	notes []map[uint32][2]uint32
	// mark is the HTML of the number of the note that the conversion
	// writes, and inline is set while it writes a footnote, whose
	// paragraphs are in a paragraph. endnotes holds the HTML of the
	// endnotes, and endnoteCount their number.
	mark   string
	inline bool
	// pictureLine is set while it writes a paragraph whose only content is
	// pictures in the line.
	pictureLine bool
	// spacing is the spacing of the lines of the paragraph that it
	// writes.
	spacing lineSpacing
	// joinBorder is set, for the start and for the end, while it writes a
	// run whose border joins the border of the run before or after it.
	joinBorder   [2]bool
	endnotes     strings.Builder
	endnoteCount int
	// drawing holds the shapes and the pictures of Office Drawing, once
	// read, spas their FSPAs by the characters of their anchors, and txbxs
	// the ranges of the text of the text boxes by the ids of their shapes,
	// for the main document and for the headers.
	drawing *drawing
	spas    [2]map[uint32][]byte
	txbxs   [2]map[uint32][2]uint32
	// fontDB holds the fonts of the WordArt, once read.
	fontDB *svg.FontDatabase
}

// newParser reads the piece table, the formatted disk pages, the styles and
// the fonts of the document.
func newParser(s *streams, cfg ivy.Config) (*parser, error) {
	lim := cfg.Limits
	p := &parser{s: s, cfg: cfg, lim: lim, pictures: map[uint32]*picture{}}
	if s.word6 {
		return p, p.readWord6()
	}
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
	p.lists = p.readLists()
	// a compressed piece holds the characters of code page 1252 (MS-DOC
	// 2.4.1)
	var all [256]byte
	for i := range all {
		all[i] = byte(i)
	}
	p.setCodePage(all[:], "windows-1252")
	return p, nil
}

// setCodePage maps each byte of a compressed piece to its character in the
// code page label. all holds the 256 bytes.
func (p *parser) setCodePage(all []byte, label string) {
	text, ok := charset.Decode(all, label)
	if !ok {
		text, _ = charset.Decode(all, "windows-1252")
	}
	i := 0
	for _, r := range text {
		if i < len(p.ansi) {
			p.ansi[i] = r
		}
		i++
	}
}

// readWord6 reads the pieces of a document of Word 6 or Word 95, each
// with one byte for each character. A document that Word did not save fast
// has no piece table, and its text is one piece from fcMin.
func (p *parser) readWord6() error {
	var all [256]byte
	for i := range all {
		all[i] = byte(i)
	}
	p.setCodePage(all[:], msole.CodePageLabel(p.s.codePage))
	p.readWord6Properties()
	if p.s.fib.flags&flagComplex == 0 {
		if p.s.fcMac < p.s.fcMin {
			return fmt.Errorf("word 6 document with the text from %d to %d: %w", p.s.fcMin, p.s.fcMac, ivy.ErrFormat)
		}
		end := min(p.s.fib.ccpText, p.s.fcMac-p.s.fcMin)
		p.pieces = []piece{{cpEnd: end, fc: p.s.fcMin, compressed: true}}
		return nil
	}
	if err := p.readClx(); err != nil {
		return err
	}
	for i := range p.pieces {
		pc := &p.pieces[i]
		// a piece of Word 6 has the offset of its bytes, without flags,
		// and its modifiers have the codes of Word 6
		pc.compressed, pc.prm = true, 0
	}
	return nil
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
		switch {
		case p.s.word6:
			pc.fc = fc
		case fc&0x40000000 != 0:
			pc.compressed, pc.fc = true, (fc&0x3FFFFFFF)/2
		default:
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
	if n := (len(plc) - 4) / 8; n > p.lim.MaxObjects {
		return nil, fmt.Errorf("%d formatted disk pages: %w", n, ivy.ErrLimit)
	}
	return p.fkpRuns(plc, para, 4), nil
}

// fkpRuns reads the formatted disk pages of the PlcBte plc, whose page
// numbers have bteSize bytes: 4 in Word 97, and 2 in Word 6, whose pages
// of paragraphs have a BX of 7 bytes and a PAPX without the escape of a
// size of 0. It reads at most MaxObjects pages.
func (p *parser) fkpRuns(plc []byte, para bool, bteSize int) []fkpRun {
	if len(plc) < 4 {
		return nil
	}
	word6 := bteSize == 2
	bx := 13
	if word6 {
		bx = word6BxSize
	}
	n := min((len(plc)-4)/(4+bteSize), p.lim.MaxObjects)
	var runs []fkpRun
	word := p.s.word
	for k := range n {
		var pn uint32
		if word6 {
			pn = uint32(le.Uint16(plc[4*(n+1)+2*k:]))
		} else {
			pn = le.Uint32(plc[4*(n+1)+4*k:]) & 0x3FFFFF
		}
		off := int64(pn) * 512
		if off+512 > int64(len(word)) {
			continue
		}
		page := word[off : off+512]
		count := int(page[511])
		if para {
			// rgfc, then rgbx of 13 bytes each, or 7 in Word 6
			if 4*(count+1)+bx*count > 511 {
				continue
			}
			for j := range count {
				r := fkpRun{fcStart: le.Uint32(page[4*j:]), fcEnd: le.Uint32(page[4*j+4:]), istd: 0}
				bOff := 2 * int(page[4*(count+1)+bx*j])
				if bOff > 0 && bOff < 511 {
					cb := 2 * int(page[bOff])
					start := bOff + 1
					switch {
					case word6:
					case cb == 0 && bOff+1 < 511:
						cb = 2 * int(page[bOff+1])
						start = bOff + 2
					default:
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
			break
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
	return runs
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
	var style pap
	if st := p.styles.resolve(istd, p.lim.MaxDepth); st != nil && st.stk == stkParagraph {
		style = st.pap
	}
	// the properties of a row come from its mark, not from a style
	style.istd, style.tap = istd, nil
	pp := style
	direct := func(pp *pap) {
		for s := range sprmsOf(p.s.word6, grpprl) {
			if s.op == sprmPHugePapx {
				// the modifiers that do not fit in the page are in the
				// stream Data, with their size in 2 bytes (MS-DOC 2.9.103)
				fc := int64(uint32(s.value()))
				data := p.s.data
				if fc >= 0 && fc+2 <= int64(len(data)) {
					n := int64(le.Uint16(data[fc:]))
					for h := range sprms(data[fc+2 : min(fc+2+n, int64(len(data)))]) {
						applyPap(pp, h)
					}
				}
				continue
			}
			applyPap(pp, s)
		}
		for s := range sprmsOf(p.s.word6, pieceMods) {
			applyPap(pp, s)
		}
	}
	direct(&pp)
	// the level of a list gives its indents and its tab stops to its
	// paragraphs, after the style and before the paragraph
	if lvl := p.level(pp.ilfo, pp.ilvl); lvl != nil && len(lvl.papx) > 0 {
		pp = style
		for s := range sprms(lvl.papx) {
			if s.op != sprmPIlfo && s.op != sprmPIlvl && !isTableSprm(s.op) {
				applyPap(&pp, s)
			}
		}
		direct(&pp)
	}
	return pp
}

// charProps returns the properties of the characters of the run r of the
// formatted disk pages, which can be nil, in a paragraph of the style istd,
// with the modifiers of their piece.
func (p *parser) charProps(r *fkpRun, istd int, pieceMods []byte) chp {
	base := defaultChp()
	base.ftc, base.ftcEA = p.styles.ftc, p.styles.ftcEA
	if st := p.styles.resolve(istd, p.lim.MaxDepth); st != nil && st.stk == stkParagraph {
		base = st.chp
	}
	var grpprl []byte
	if r != nil {
		grpprl = r.grpprl
	}
	// a character style comes before the other modifiers
	for s := range sprmsOf(p.s.word6, grpprl) {
		if s.op != sprmCIstd {
			continue
		}
		if st := p.styles.resolve(int(uint16(s.value())), p.lim.MaxDepth); st != nil && st.stk == stkCharacter {
			cs := base
			for m := range sprmsOf(p.s.word6, st.chpx) {
				applyChp(&cs, m, &base)
			}
			base = cs
		}
	}
	c := base
	for s := range sprmsOf(p.s.word6, grpprl) {
		applyChp(&c, s, &base)
	}
	for s := range sprmsOf(p.s.word6, pieceMods) {
		applyChp(&c, s, &base)
	}
	return c
}

// char is a character of the main document and the byte of the stream
// WordDocument that holds it.
type char struct {
	r     rune
	cp    uint32
	fc    uint32
	piece int
}

// chars iterates over the characters of the document from the character
// position from to the position to, in the order of the pieces. The pieces
// of a valid document do not overlap, so the document has fewer characters
// than the stream WordDocument has bytes. The iteration stops there, so
// that pieces that repeat the same bytes cannot make it long.
func (p *parser) chars(from, to uint32) iter.Seq[char] {
	return func(yield func(char) bool) {
		p.charsIn(from, to, yield)
	}
}

// charsIn is the iteration of chars.
func (p *parser) charsIn(from, limit uint32, yield func(char) bool) {
	word := p.s.word
	budget := len(word)
	for i, pc := range p.pieces {
		if pc.cpStart >= limit {
			return
		}
		if pc.cpEnd <= from {
			continue
		}
		end := min(pc.cpEnd, limit)
		for cp := max(pc.cpStart, from); cp < end; cp++ {
			if budget--; budget < 0 {
				return
			}
			k := cp - pc.cpStart
			var c char
			c.piece, c.cp = i, cp
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

// paragraphs reads the text of the main document into paragraphs.
func (p *parser) paragraphs(ctx context.Context) ([]paragraph, error) {
	return p.paragraphsIn(ctx, 0, p.s.fib.ccpText)
}

// paragraphsIn reads the text from the character position from to the
// position to into paragraphs. It drops the codes of the fields and keeps
// their results, and drops the hidden and the deleted text. The result of
// the field PAGE or NUMPAGES becomes a character that the layout replaces
// with the number.
func (p *parser) paragraphsIn(ctx context.Context, from, to uint32) ([]paragraph, error) {
	var (
		paras   []paragraph
		cur     paragraph
		text    strings.Builder
		runChp  chp
		haveRun bool
		fields  []*field
		pending []char
		total   int
		secs    = p.sections()
		errAt   error
	)
	mainEnd := p.s.fib.ccpText
	hdrStart, hdrEnd := p.s.fib.partRange(partHeaders)
	flushRun := func() {
		if haveRun && text.Len() > 0 {
			cur.runs = append(cur.runs, run{text: text.String(), chp: runChp})
		}
		text.Reset()
		haveRun = false
	}
	hidden := func() bool {
		for _, f := range fields {
			if f.code || f.skip {
				return true
			}
		}
		return false
	}
	// pageField adds the character of the number of the page for the field
	// f, if it is PAGE or NUMPAGES, at the character c.
	pageField := func(f *field, c char) bool {
		code := strings.Fields(strings.ToUpper(f.instr.String()))
		if len(code) == 0 {
			return false
		}
		switch code[0] {
		case "PAGE":
			c.r = runePage
		case "NUMPAGES", "SECTIONPAGES":
			c.r = runePages
		default:
			return false
		}
		if !hidden() {
			pending = append(pending, c)
		}
		return true
	}
	endPara := func(mark char) {
		mods := p.pieceGrpprl(p.pieces[mark.piece].prm)
		cur.pap = p.paraProps(mark.fc, mods)
		cur.cp = mark.cp
		cur.mark = p.charProps(find(p.chpx, mark.fc), cur.pap.istd, mods)
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
			if c.r == 0x08 && cp.special {
				// the anchor of a shape of Office Drawing
				if c.cp < mainEnd || c.cp >= hdrStart && c.cp < hdrEnd {
					d, err := p.drawnAt(ctx, c.cp, c.cp >= mainEnd)
					if err != nil {
						errAt = err
					}
					if d != nil {
						flushRun()
						cur.runs = append(cur.runs, run{chp: cp, drawn: d})
					}
				}
				continue
			}
			if c.r == 0x02 && cp.special {
				// the reference of a footnote or of an endnote, or the
				// number of a note at the start of its text
				flushRun()
				if c.cp >= mainEnd {
					cur.runs = append(cur.runs, run{chp: cp, noteMark: true})
					continue
				}
				n, err := p.readNote(ctx, c.cp)
				if err != nil {
					errAt = err
				}
				if n != nil {
					cur.runs = append(cur.runs, run{chp: cp, note: n})
				}
				continue
			}
			if c.r == 0x01 && cp.special {
				// a picture of the stream Data
				if pic := p.picture(ctx, cp); pic != nil {
					flushRun()
					cur.runs = append(cur.runs, run{chp: cp, pic: pic})
				}
				continue
			}
			rc, r := cp, c.r
			if cp.special && cp.symbol {
				// a symbol that the character stands for, which the font
				// of sprmCSymbol draws
				rc.ftc, r = cp.symbolFtc, p.symbolChar(cp.symbolFtc, cp.symbolCode)
			}
			s, ok := visible(r)
			if !ok {
				continue
			}
			if !haveRun || rc != runChp {
				flushRun()
				runChp, haveRun = rc, true
			}
			text.WriteString(s)
		}
		flushRun()
		pending = pending[:0]
		cur.end = byte(mark.r)
		paras = append(paras, cur)
		cur = paragraph{}
	}
	for c := range p.chars(from, to) {
		total++
		if total%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("reading word document: %w", err)
			}
		}
		switch c.r {
		case 0x13:
			if len(fields) < p.lim.MaxDepth {
				fields = append(fields, &field{code: true})
			}
			continue
		case 0x14:
			if len(fields) > 0 {
				f := fields[len(fields)-1]
				if f.code {
					f.code = false
					f.skip = pageField(f, c)
				}
			}
			continue
		case 0x15:
			if len(fields) > 0 {
				f := fields[len(fields)-1]
				fields = fields[:len(fields)-1]
				if f.code {
					// a field without a result
					pageField(f, c)
				}
			}
			continue
		case 0x0D, 0x07:
			endPara(c)
			if errAt != nil {
				return nil, errAt
			}
			if len(paras) > p.lim.MaxObjects {
				return nil, fmt.Errorf("more than %d paragraphs: %w", p.lim.MaxObjects, ivy.ErrLimit)
			}
			continue
		case 0x0C:
			if hidden() {
				continue
			}
			// a page break ends the text before it, and the text after it
			// starts on a new page. A section break does the same, unless
			// the next section is continuous
			if len(pending) > 0 {
				mark := c
				mark.r = 0x0D
				endPara(mark)
			}
			if !secs.continuous(c.cp + 1) {
				cur.pageBreak = true
			}
			continue
		case 0x0E:
			if hidden() {
				continue
			}
			// a column break ends the text before it too, and the text
			// after it starts in the next column
			if len(pending) > 0 {
				mark := c
				mark.r = 0x0D
				endPara(mark)
			}
			cur.columnBreak = true
			continue
		}
		if len(fields) > 0 {
			if f := fields[len(fields)-1]; f.code {
				if f.instr.Len() < 256 {
					f.instr.WriteRune(c.r)
				}
				continue
			}
		}
		if hidden() {
			continue
		}
		pending = append(pending, c)
	}
	if len(pending) > 0 {
		last := pending[len(pending)-1]
		last.r = 0x0D
		endPara(last)
	}
	if errAt != nil {
		return nil, errAt
	}
	return paras, nil
}

// visible returns the text of the character r in a paragraph, or false for
// a character that has no text, such as the anchor of a picture.
func visible(r rune) (string, bool) {
	switch r {
	case 0x0B, '\n':
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
