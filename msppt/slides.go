package msppt

import (
	"unicode/utf16"
)

// The types of a text of MS-PPT 2.13.33.
const (
	txTitle       = 0
	txBody        = 1
	txNotes       = 2
	txOther       = 4
	txCenterBody  = 5
	txCenterTitle = 6
	txHalfBody    = 7
	txQuarterBody = 8
	txTypes       = 9
)

// More records of MS-PPT 2.13.24 that the package reads.
const (
	rtSlideAtom          = 0x03EF
	rtEnvironment        = 0x03F2
	rtMainMaster         = 0x03F8
	rtPPDrawingGroup     = 0x040B
	rtColorScheme        = 0x07F0
	rtFontCollection     = 0x07D5
	rtStyleTextProp      = 0x0FA1
	rtTextMasterStyle    = 0x0FA3
	rtTextCFException    = 0x0FA4
	rtTextPFException    = 0x0FA5
	rtTextRulerAtom      = 0x0FA6
	rtTxInteractiveInfo  = 0x0FDF
	rtFontEntity         = 0x0FB7
	rtSlideNumberMC      = 0x0FD8
	rtMasterListWithText = 1
)

// The flags of a SlideAtom (MS-PPT 2.5.11).
const (
	slideMasterObjects    = 0x0001
	slideMasterScheme     = 0x0002
	slideMasterBackground = 0x0004
)

// text is a text of a shape: its type, its characters with a carriage
// return between two paragraphs, and the records of its style.
type text struct {
	kind uint32
	// units are the characters in UTF-16, as the offsets of the style
	// count them.
	units []uint16
	// style is the body of the StyleTextPropAtom, or nil.
	style []byte
	// fields are the offsets of the slide numbers in text.
	fields []int
	// ruler holds the left margins and the indents of the levels of the
	// TextRulerAtom, which go over the styles of the master.
	ruler [masterLevels]rulerLevel
	// links are the ranges of the characters of the hyperlinks, from the
	// TxInteractiveInfoAtom records: the first and the end after the last.
	links [][2]int
}

// schemeHyperlink is the index of the color of the hyperlinks in a color
// scheme: "accent and hyperlink" of MS-PPT 2.12.8.
const schemeHyperlink = 6

// maxLinks is the number of the hyperlinks of a text that the package
// reads.
const maxLinks = 1024

// linkAt reports whether the character i of the text is in a hyperlink,
// and returns the offset where the hyperlink, or the text out of a
// hyperlink, ends.
func (t *text) linkAt(i int) (bool, int) {
	end := len(t.units) + 1
	for _, l := range t.links {
		switch {
		case i >= l[0] && i < l[1]:
			return true, l[1]
		case l[0] > i:
			end = min(end, l[0])
		}
	}
	return false, end
}

// rulerLevel is the left margin and the indent of a level of a
// TextRulerAtom, in master units.
type rulerLevel struct {
	margin, indent opt[int]
}

// The bits of the rulerMask of a TextRulerAtom (MS-PPT 2.9.57).
const (
	rulerDefaultTab = 0x0001
	rulerLevels     = 0x0002
	rulerTabs       = 0x0004
	rulerMargin1    = 0x0008
	rulerIndent1    = 0x0100
)

// readRuler reads the left margins and the indents of a TextRulerAtom:
// rulerMask, cLevels, defaultTabSize, the tab stops, then leftMargin1,
// indent1, leftMargin2 and so on to the fifth level.
func readRuler(b []byte) [masterLevels]rulerLevel {
	var levels [masterLevels]rulerLevel
	r := reader{b: b}
	m := r.u32()
	if m&rulerLevels != 0 {
		r.u16()
	}
	if m&rulerDefaultTab != 0 {
		r.u16()
	}
	if m&rulerTabs != 0 {
		n := int(r.u16())
		if n > 1024 {
			return levels
		}
		r.off += 4 * n
	}
	for i := range levels {
		if m&(rulerMargin1<<i) != 0 {
			levels[i].margin.set(int(r.u16()))
		}
		if m&(rulerIndent1<<i) != 0 {
			levels[i].indent.set(int(r.u16()))
		}
	}
	if r.bad {
		return [masterLevels]rulerLevel{}
	}
	return levels
}

// rect is a rectangle in master units, 576 to the inch.
type rect struct {
	left, top, right, bottom float64
}

// slideRef is a slide of the slide list: its persist object and its texts.
type slideRef struct {
	persist uint32
	texts   []text
}

// document is what the package reads of the DocumentContainer: the size of
// the slides, the slides in order, the masters, the fonts, the default
// styles of the text and the pictures.
type document struct {
	width, height float64 // master units
	slides        []slideRef
	// masters maps the id of a master to its persist object.
	masters map[uint32]uint32
	fonts   []string
	// defaults are the styles of the text that the masters inherit.
	defaults masterStyle
	// blips are the entries of the store of pictures, from index 1.
	blips []blipEntry
}

// readDocument reads the DocumentContainer of s.
func readDocument(s *stream) (*document, bool) {
	h, body, ok := s.object(s.docRef)
	if !ok || h.typ != rtDocument {
		return nil, false
	}
	d := &document{width: 5760, height: 4320, masters: make(map[uint32]uint32)}
	if atom := child(body, rtDocumentAtom); len(atom) >= 8 {
		w, ht := float64(int32(le.Uint32(atom))), float64(int32(le.Uint32(atom[4:])))
		if w >= 576 && ht >= 576 && w <= 576*200 && ht <= 576*200 {
			d.width, d.height = w, ht
		}
	}
	for h, b := range children(body) {
		switch {
		case h.typ == rtSlideListWithText && h.inst == 0:
			d.readSlideList(b, s.lim.MaxObjects)
		case h.typ == rtSlideListWithText && h.inst == rtMasterListWithText:
			for ch, a := range children(b) {
				if ch.typ == rtSlidePersistAtom && len(a) >= 16 && len(d.masters) < s.lim.MaxObjects {
					d.masters[le.Uint32(a[12:])] = le.Uint32(a)
				}
			}
		case h.typ == rtEnvironment:
			d.readEnvironment(b, s.lim.MaxObjects)
		case h.typ == rtPPDrawingGroup:
			d.readDrawingGroup(child(b, rtOfficeArtDggContainer), s.lim.MaxObjects)
		}
	}
	return d, true
}

// readSlideList reads the slides of a SlideListWithTextContainer and their
// texts.
func (d *document) readSlideList(b []byte, maxObjects int) {
	var cur *slideRef
	for ch, a := range children(b) {
		if ch.typ == rtSlidePersistAtom {
			if len(a) >= 4 && len(d.slides) < maxObjects {
				d.slides = append(d.slides, slideRef{persist: le.Uint32(a)})
				cur = &d.slides[len(d.slides)-1]
			}
			continue
		}
		if cur != nil && len(cur.texts) < maxObjects {
			cur.texts = addText(cur.texts, ch, a)
		}
	}
}

// addText adds the record of the header h and the body b to the texts of a
// list: a TextHeaderAtom starts a new text, and the records after it give
// its characters and its style.
func addText(texts []text, h header, b []byte) []text {
	if h.typ == rtTextHeaderAtom {
		var kind uint32 = txOther
		if len(b) >= 4 {
			kind = le.Uint32(b)
		}
		return append(texts, text{kind: kind})
	}
	if len(texts) == 0 {
		return texts
	}
	t := &texts[len(texts)-1]
	switch h.typ {
	case rtTextCharsAtom, rtTextBytesAtom:
		t.units = decodeText(h.typ, b)
	case rtStyleTextProp:
		t.style = b
	case rtTextRulerAtom:
		t.ruler = readRuler(b)
	case rtTxInteractiveInfo:
		if len(b) >= 8 && len(t.links) < maxLinks {
			first, last := int(int32(le.Uint32(b))), int(int32(le.Uint32(b[4:])))
			if first >= 0 && last > first {
				t.links = append(t.links, [2]int{first, last})
			}
		}
	case rtSlideNumberMC:
		if len(b) >= 4 && len(t.fields) < 64 {
			t.fields = append(t.fields, int(int32(le.Uint32(b))))
		}
	}
	return texts
}

// readEnvironment reads the fonts and the default styles of the text from
// the DocumentTextInfoContainer.
func (d *document) readEnvironment(b []byte, maxObjects int) {
	for h, a := range children(b) {
		switch h.typ {
		case rtFontCollection:
			for ch, e := range children(a) {
				if ch.typ == rtFontEntity && len(d.fonts) < maxObjects {
					d.fonts = append(d.fonts, fontName(e))
				}
			}
		case rtTextCFException:
			if cf, _, ok := readCF(a); ok {
				for i := range d.defaults {
					d.defaults[i].cf = cf
				}
			}
		case rtTextPFException:
			if len(a) > 2 {
				if pf, _, ok := readPF(a[2:]); ok {
					for i := range d.defaults {
						d.defaults[i].pf = pf
					}
				}
			}
		case rtTextMasterStyle:
			if st, ok := readMasterStyle(a, h.inst); ok {
				for i := range d.defaults {
					d.defaults[i].over(&st[i])
				}
			}
		}
	}
}

// fontName returns the name of the font of a FontEntityAtom: a string of
// UTF-16 of 32 characters at most, which a NUL ends.
func fontName(b []byte) string {
	units := make([]uint16, 0, 32)
	for i := 0; i+2 <= min(len(b), 64); i += 2 {
		u := le.Uint16(b[i:])
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}

// readDrawingGroup reads the store of pictures from the
// OfficeArtDggContainer.
func (d *document) readDrawingGroup(b []byte, maxObjects int) {
	for h, a := range children(b) {
		if h.typ != rtOfficeArtBStore {
			continue
		}
		for ch, e := range children(a) {
			if len(d.blips) >= maxObjects {
				break
			}
			d.blips = append(d.blips, readFBSE(ch, e))
		}
	}
}

// decodeText decodes a TextCharsAtom, in UTF-16, or a TextBytesAtom, whose
// bytes are the low bytes of UTF-16.
func decodeText(typ uint16, b []byte) []uint16 {
	var units []uint16
	if typ == rtTextCharsAtom {
		units = make([]uint16, len(b)/2)
		for i := range units {
			units[i] = le.Uint16(b[2*i:])
		}
	} else {
		units = make([]uint16, len(b))
		for i, c := range b {
			units[i] = uint16(c)
		}
	}
	return units
}
