package msdoc

import (
	"context"

	"github.com/xo/ivy/internal/odraw"
)

// This file reads the inline pictures of a document: the character 0x01
// whose properties give the offset of a PICFAndOfficeArtData in the stream
// Data (MS-DOC 2.9.192). It holds a PICF, and the shape of the picture
// with the entries of its pictures, an OfficeArtInlineSpContainer of
// Office Drawing (MS-DOC 2.9.134). internal/odraw reads the shape and its
// pictures, which are in the entries or in the stream WordDocument.

// picfSize is the size of a PICF, the header of a picture.
const picfSize = 0x44

// mmShape is the mm of a PICF whose picture is a shape with a name.
const mmShape = 0x66

// picture is a picture or a shape in the source of the layout: the path of
// its file or the URL of its object, and its size in points. The object of
// a shape is larger than the shape by pad on each side.
type picture struct {
	name          string
	width, height float64
	pad           float64
}

// media is a file of a picture for the source of the layout.
type media struct {
	name string
	data []byte
}

// picture returns the picture of the character 0x01 with the properties c,
// or nil for a picture that the package does not read, such as a PICT.
func (p *parser) picture(ctx context.Context, c chp) *picture {
	if c.data || p.s.data == nil {
		return nil
	}
	if pic, ok := p.pictures[c.picLocation]; ok {
		return pic
	}
	p.pictures[c.picLocation] = nil
	if len(p.pictures) > p.lim.MaxObjects {
		return nil
	}
	data := p.s.data
	off := int(c.picLocation)
	if off < 0 || off+picfSize > len(data) {
		return nil
	}
	lcb := int(le.Uint32(data[off:]))
	cbHeader := int(le.Uint16(data[off+4:]))
	if cbHeader < picfSize || lcb < cbHeader || off+lcb > len(data) {
		return nil
	}
	picf := data[off : off+lcb]
	// the size that the picture shows, in twips, and its scale in tenths
	// of a percent
	dxa, dya := float64(le.Uint16(picf[28:])), float64(le.Uint16(picf[30:]))
	mx, my := float64(le.Uint16(picf[32:])), float64(le.Uint16(picf[34:]))
	if mx == 0 {
		mx = 1000
	}
	if my == 0 {
		my = 1000
	}
	rest := picf[cbHeader:]
	if le.Uint16(picf[6:]) == mmShape && len(rest) > 0 {
		rest = rest[min(1+int(rest[0]), len(rest)):]
	}
	w, h := dxa*mx/1000/20, dya*my/1000/20
	if w <= 0 || h <= 0 || w > 1584 || h > 1584 {
		w, h = 0, 0
	}
	sh, entries := p.inlineShape(rest)
	pics := odraw.NewPictures(entries, p.s.word, p.cfg)
	var pic *picture
	switch {
	case sh == nil:
		pic = p.plainPicture(ctx, &odraw.Shape{Type: odraw.SptPictureFrame, Props: pibProps(1)}, pics, w, h)
	case w > 0:
		if pic = p.plainPicture(ctx, sh, pics, w, h); pic == nil {
			if obj, ok := p.shapeObject(ctx, sh, w, h, pics); ok {
				pic = &picture{name: obj.name, width: w, height: h, pad: obj.pad}
			}
		}
	default:
		// a picture without a size shows at the size of its file
		pic = p.plainPicture(ctx, sh, pics, 0, 0)
	}
	p.pictures[c.picLocation] = pic
	return pic
}

// pibProps returns the properties of a picture frame that shows the entry
// pib of the store.
func pibProps(pib uint32) odraw.Props {
	var props odraw.Props
	props.Set(odraw.PropPib, pib)
	return props
}

// inlineShape returns the shape of the OfficeArtInlineSpContainer b, and
// the entries of its pictures. A container without a shape gives the
// first picture that it holds, in a container, in an entry or alone.
func (p *parser) inlineShape(b []byte) (*odraw.Shape, []odraw.Entry) {
	if h, ok := odraw.ReadHeader(b); ok && h.Type == odraw.RecSpContainer && uint64(h.Size) <= uint64(len(b)-8) {
		sh := odraw.ReadShape(b[8 : 8+h.Size])
		entries := odraw.ReadEntries(b[8+h.Size:], p.lim.MaxObjects)
		if len(entries) == 0 {
			if e, ok := firstEntry(b, p.s.word, p.lim.MaxMemory, 8); ok {
				entries = []odraw.Entry{e}
			}
		}
		if sh.Type == odraw.SptNotPrimitive && sh.Props.Complex(odraw.PropVertices) == nil {
			// a shape without a geometry of its own is the frame of the
			// picture
			sh.Type = odraw.SptPictureFrame
		}
		if pib, ok := sh.Props.Get(odraw.PropPib); (!ok || pib > uint32(len(entries))) && len(entries) > 0 && sh.Type == odraw.SptPictureFrame {
			// the pib of an inline picture is the index of its picture in
			// the store of the document, which does not hold it. Its first
			// entry holds it
			sh.Props.Set(odraw.PropPib, 1)
		}
		return sh, entries
	}
	if e, ok := firstEntry(b, p.s.word, p.lim.MaxMemory, 8); ok {
		return nil, []odraw.Entry{e}
	}
	return nil, nil
}

// firstEntry returns the first entry of the records in b with a picture
// that odraw reads: an OfficeArtFBSE, whose picture can be in the stream
// WordDocument, or a picture record alone. A metafile stays within
// maxMemory bytes. It looks into the containers, depth deep at most.
func firstEntry(b, word []byte, maxMemory int64, depth int) (odraw.Entry, bool) {
	if depth <= 0 {
		return odraw.Entry{}, false
	}
	for len(b) >= 8 {
		h, _ := odraw.ReadHeader(b)
		if uint64(h.Size) > uint64(len(b)-8) {
			return odraw.Entry{}, false
		}
		rec, body := b[:8+h.Size], b[8:8+h.Size]
		switch {
		case h.Ver == 0xF:
			if e, ok := firstEntry(body, word, maxMemory, depth-1); ok {
				return e, true
			}
		case h.Type == odraw.RecFBSE:
			for _, e := range odraw.ReadEntries(rec, 1) {
				if odraw.Data(e.Record(word), maxMemory) != nil {
					return e, true
				}
			}
		case h.Type >= rtBlipFirst && h.Type <= rtBlipLast:
			return odraw.Entry{Embedded: rec, OK: true}, true
		}
		b = b[8+h.Size:]
	}
	return odraw.Entry{}, false
}

// The types of the picture records of Office Drawing (MS-ODRAW 2.2.23).
const (
	rtBlipFirst = 0xF018
	rtBlipLast  = 0xF117
)
