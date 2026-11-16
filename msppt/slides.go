package msppt

import (
	"strings"
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
)

// text is a text of a slide with its type.
type text struct {
	kind uint32
	text string
}

// rect is a rectangle in master units, 576 to the inch.
type rect struct {
	left, top, right, bottom float64
}

// shape is a shape of a slide that holds text.
type shape struct {
	text
	anchor    rect
	hasAnchor bool
}

// slide is a slide with its texts.
type slide struct {
	shapes []shape
	// texts are the texts of the slide list for the slide, which the
	// placeholders name by their index.
	texts []text
}

// slideRef is a slide of the slide list: its persist object and its texts.
type slideRef struct {
	persist uint32
	texts   []text
}

// document is what the package reads of the DocumentContainer: the size of
// the slides and the list of the slides, in order.
type document struct {
	width, height float64 // master units
	slides        []slideRef
}

// readDocument reads the DocumentContainer of s.
func readDocument(s *stream) (*document, bool) {
	h, body, ok := s.object(s.docRef)
	if !ok || h.typ != rtDocument {
		return nil, false
	}
	d := &document{width: 5760, height: 4320}
	if atom := child(body, rtDocumentAtom); len(atom) >= 8 {
		w, ht := float64(int32(le.Uint32(atom))), float64(int32(le.Uint32(atom[4:])))
		if w >= 576 && ht >= 576 && w <= 576*200 && ht <= 576*200 {
			d.width, d.height = w, ht
		}
	}
	for h, list := range children(body) {
		if h.typ != rtSlideListWithText || h.inst != 0 {
			continue
		}
		var cur *slideRef
		var kind uint32 = txOther
		for ch, b := range children(list) {
			switch ch.typ {
			case rtSlidePersistAtom:
				if len(b) >= 4 {
					d.slides = append(d.slides, slideRef{persist: le.Uint32(b)})
					cur = &d.slides[len(d.slides)-1]
				}
			case rtTextHeaderAtom:
				if len(b) >= 4 {
					kind = le.Uint32(b)
				}
			case rtTextCharsAtom, rtTextBytesAtom:
				if cur != nil {
					cur.texts = append(cur.texts, text{kind: kind, text: decodeText(ch.typ, b)})
				}
			}
			if len(d.slides) > s.lim.MaxObjects {
				return d, true
			}
		}
	}
	return d, true
}

// decodeText decodes a TextCharsAtom, in UTF-16, or a TextBytesAtom, whose
// bytes are the low bytes of UTF-16.
func decodeText(typ uint16, b []byte) string {
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
	return string(utf16.Decode(units))
}

// readSlide reads the shapes of the slide ref, with their anchors and their
// texts. It returns the size of the record of the slide.
func readSlide(s *stream, ref slideRef, maxDepth int) (slide, int) {
	sl := slide{texts: ref.texts}
	h, body, ok := s.object(ref.persist)
	if !ok || h.typ != rtSlide {
		return sl, len(body)
	}
	dg := child(child(body, rtPPDrawing), rtOfficeArtDg)
	if spgr := child(dg, rtOfficeArtSpgr); spgr != nil {
		sl.readGroup(spgr, nil, maxDepth)
	}
	return sl, len(body)
}

// transform maps the coordinates of a group onto the coordinates of the
// slide.
type transform struct {
	from, to rect
}

// apply maps r through t.
func (t *transform) apply(r rect) rect {
	if t == nil {
		return r
	}
	sx := (t.to.right - t.to.left) / nonzero(t.from.right-t.from.left)
	sy := (t.to.bottom - t.to.top) / nonzero(t.from.bottom-t.from.top)
	return rect{
		left:   t.to.left + (r.left-t.from.left)*sx,
		top:    t.to.top + (r.top-t.from.top)*sy,
		right:  t.to.left + (r.right-t.from.left)*sx,
		bottom: t.to.top + (r.bottom-t.from.top)*sy,
	}
}

func nonzero(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}

// readGroup reads the shapes of the OfficeArtSpgrContainer b. The first
// shape of a group is the group itself, with the coordinates of its
// children (MS-ODRAW 2.2.16).
func (sl *slide) readGroup(b []byte, t *transform, depth int) {
	if depth <= 0 {
		return
	}
	first := true
	for h, body := range children(b) {
		switch h.typ {
		case rtOfficeArtSpgr:
			sl.readGroup(body, t, depth-1)
		case rtOfficeArtSp:
			if first {
				first = false
				if fspgr := child(body, rtOfficeArtFSPGR); len(fspgr) >= 16 {
					anchor, ok := anchorOf(body)
					if ok {
						from := rect{
							left:   float64(int32(le.Uint32(fspgr))),
							top:    float64(int32(le.Uint32(fspgr[4:]))),
							right:  float64(int32(le.Uint32(fspgr[8:]))),
							bottom: float64(int32(le.Uint32(fspgr[12:]))),
						}
						t = &transform{from: from, to: t.apply(anchor)}
					}
				}
				continue
			}
			sl.readShape(body, t)
		}
		if len(sl.shapes) > 1<<16 {
			return
		}
	}
}

// anchorOf returns the anchor of a shape: its client anchor on a slide, or
// its child anchor in a group.
func anchorOf(sp []byte) (rect, bool) {
	if a := child(sp, rtOfficeArtClientAnch); a != nil {
		switch {
		case len(a) >= 16:
			// a RectStruct: top, left, right, bottom
			return rect{
				top:    float64(int32(le.Uint32(a))),
				left:   float64(int32(le.Uint32(a[4:]))),
				right:  float64(int32(le.Uint32(a[8:]))),
				bottom: float64(int32(le.Uint32(a[12:]))),
			}, true
		case len(a) >= 8:
			// a SmallRectStruct: top, left, right, bottom
			return rect{
				top:    float64(int16(le.Uint16(a))),
				left:   float64(int16(le.Uint16(a[2:]))),
				right:  float64(int16(le.Uint16(a[4:]))),
				bottom: float64(int16(le.Uint16(a[6:]))),
			}, true
		}
	}
	if a := child(sp, rtOfficeArtChild); len(a) >= 16 {
		// a RECT: left, top, right, bottom
		return rect{
			left:   float64(int32(le.Uint32(a))),
			top:    float64(int32(le.Uint32(a[4:]))),
			right:  float64(int32(le.Uint32(a[8:]))),
			bottom: float64(int32(le.Uint32(a[12:]))),
		}, true
	}
	return rect{}, false
}

// readShape reads the text of a shape, from its client text box: the text
// itself, or a reference to a text of the slide list.
func (sl *slide) readShape(sp []byte, t *transform) {
	box := child(sp, rtOfficeArtClientText)
	if box == nil {
		return
	}
	sh := shape{text: text{kind: txOther}}
	found := false
	for h, b := range children(box) {
		switch h.typ {
		case rtTextHeaderAtom:
			if len(b) >= 4 {
				sh.kind = le.Uint32(b)
			}
		case rtTextCharsAtom, rtTextBytesAtom:
			sh.text.text = decodeText(h.typ, b)
			found = true
		case rtOutlineTextRefAtom:
			if len(b) >= 4 {
				i := int(int32(le.Uint32(b)))
				if i >= 0 && i < len(sl.texts) {
					sh.text = sl.texts[i]
					found = true
				}
			}
		}
	}
	if !found || strings.TrimSpace(sh.text.text) == "" {
		return
	}
	if anchor, ok := anchorOf(sp); ok {
		sh.anchor, sh.hasAnchor = t.apply(anchor), true
	}
	sl.shapes = append(sl.shapes, sh)
}
