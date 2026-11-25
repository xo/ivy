package msppt

import (
	"unicode/utf16"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/odraw"
)

// This file reads the extension of PowerPoint 2000 (MS-PPT 2.9.71 and
// 2.9.79): the binary tag "___PPT9" of the document holds the pictures of
// the picture bullets, and the binary tag "___PPT9" of a shape holds a
// StyleTextProp9Atom, with the picture bullets and the automatic numbers
// of the paragraphs of its text. The style of a run of characters selects
// its record of the atom with its field pp9rt.

// The records of the binary tags (MS-PPT 2.13.24).
const (
	rtList               = 0x07D0
	rtProgTags           = 0x1388
	rtProgBinaryTag      = 0x138A
	rtBinaryTagDataBlob  = 0x138B
	rtBlipCollection9    = 0x07F8
	rtBlipEntity9Atom    = 0x07F9
	rtStyleTextProp9Atom = 0x0FAC
)

// pp9Tag is the name of the binary tag of PowerPoint 2000.
const pp9Tag = "___PPT9"

// The bits of the PFMasks of a TextPFException9 (MS-PPT 2.9.21), and the
// bit of the CFMasks of a TextCFException9 (MS-PPT 2.9.15).
const (
	pfBulletBlip      = 1 << 23
	pfBulletScheme    = 1 << 24
	pfBulletHasScheme = 1 << 25
	cfPP10ext         = 1 << 20
)

// defaultScheme9 is the scheme of the numbers of a paragraph that has
// automatic numbers without a scheme: ANM_ArabicPeriod.
const defaultScheme9 = 3

// autoNumberSchemes are the names of DrawingML of the schemes of
// TextAutoNumberSchemeEnum (MS-PPT 2.13.28), in the order of their values.
var autoNumberSchemes = [...]string{
	"alphaLcPeriod", "alphaUcPeriod", "arabicParenR", "arabicPeriod",
	"romanLcParenBoth", "romanLcParenR", "romanLcPeriod", "romanUcPeriod",
	"alphaLcParenBoth", "alphaLcParenR", "alphaUcParenBoth", "alphaUcParenR",
	"arabicParenBoth", "arabicPlain", "romanUcParenBoth", "romanUcParenR",
	"ea1ChsPlain", "ea1ChsPeriod", "circleNumDbPlain", "circleNumWdWhitePlain",
	"circleNumWdBlackPlain", "ea1ChtPlain", "ea1ChtPeriod", "arabic1Minus",
	"arabic2Minus", "hebrew2Minus", "ea1JpnKorPlain", "ea1JpnKorPeriod",
	"arabicDbPlain", "arabicDbPeriod", "thaiAlphaPeriod", "thaiAlphaParenR",
	"thaiAlphaParenBoth", "thaiNumPeriod", "thaiNumParenR", "thaiNumParenBoth",
	"hindiAlphaPeriod", "hindiNumPeriod", "ea1JpnChsDbPeriod", "hindiNumParenR",
	"hindiAlpha1Period",
}

// pf9 is a TextPFException9: the picture bullet and the automatic number
// of a paragraph (MS-PPT 2.9.23).
type pf9 struct {
	// blip is the index of the picture of the bullet in the pictures of
	// the document.
	blip opt[int]
	// autoNumber says whether the paragraph has automatic numbers, with
	// the scheme and the first number of scheme and start.
	autoNumber    opt[bool]
	scheme, start opt[int]
}

// pp9Data returns the body of the BinaryTagDataBlob of the binary tag
// "___PPT9" in the body b of a ProgTags container, or nil.
func pp9Data(b []byte) []byte {
	for h, tag := range children(b) {
		if h.Type != rtProgBinaryTag {
			continue
		}
		var name string
		var blob []byte
		for th, tb := range children(tag) {
			switch th.Type {
			case rtCString:
				name = string(utf16.Decode(decodeText(rtTextCharsAtom, tb)))
			case rtBinaryTagDataBlob:
				blob = tb
			}
		}
		if name == pp9Tag {
			return blob
		}
	}
	return nil
}

// readBulletPictures reads the pictures of the picture bullets of the body
// b of the DocumentContainer, from the tags of its DocInfoListContainer:
// the BlipEntity9Atom records of the BlipCollection9Container, each with
// the type of its picture, a byte that is not used, and an OfficeArtBlip
// record (MS-PPT 2.9.73). A picture that the package does not read is nil.
func readBulletPictures(b []byte, lim ivy.Limits) [][]byte {
	coll := child(pp9Data(child(child(b, rtList), rtProgTags)), rtBlipCollection9)
	var pics [][]byte
	for h, e := range children(coll) {
		if h.Type != rtBlipEntity9Atom {
			continue
		}
		if len(pics) >= lim.MaxObjects {
			break
		}
		var data []byte
		if len(e) > 2 {
			data = odraw.Data(e[2:], lim.MaxMemory)
		}
		pics = append(pics, data)
	}
	return pics
}

// shapeProps9 returns the records of the StyleTextProp9Atom of the client
// data b of a shape, or nil.
func shapeProps9(b []byte, maxObjects int) []pf9 {
	atom := child(pp9Data(child(b, rtProgTags)), rtStyleTextProp9Atom)
	if atom == nil {
		return nil
	}
	return readStyleProps9(atom, maxObjects)
}

// readStyleProps9 reads the StyleTextProp9 records of a
// StyleTextProp9Atom: each a TextPFException9, a TextCFException9 and a
// TextSIException (MS-PPT 2.9.68).
func readStyleProps9(b []byte, maxObjects int) []pf9 {
	var out []pf9
	for len(b) >= 4 && len(out) < maxObjects {
		r := reader{b: b}
		var p pf9
		m := r.u32()
		if m&pfBulletBlip != 0 {
			p.blip.set(r.i16())
		}
		if m&pfBulletHasScheme != 0 {
			p.autoNumber.set(r.i16() == 1)
		}
		if m&pfBulletScheme != 0 {
			p.scheme.set(int(r.u16()))
			p.start.set(r.i16())
		}
		if r.u32()&cfPP10ext != 0 {
			r.u32()
		}
		if r.bad {
			break
		}
		_, n, ok := readSI(b[r.off:])
		if !ok {
			break
		}
		out = append(out, p)
		b = b[r.off+n:]
	}
	return out
}

// bullet9 returns the bullet of a paragraph with a bullet, from the record
// p of its StyleTextProp9Atom: a picture bullet, or automatic numbers. It
// returns false when the record gives neither, or when the picture is
// missing.
func (b *builder) bullet9(p *pf9) (drawingml.Bullet, bool) {
	pics := b.d.doc.bulletPics
	switch {
	case p.blip.ok && p.blip.v >= 0 && p.blip.v < len(pics) && pics[p.blip.v] != nil:
		return drawingml.Bullet{Kind: drawingml.BulletPicture, Picture: pics[p.blip.v]}, true
	case p.autoNumber.ok && p.autoNumber.v:
		scheme := defaultScheme9
		if p.scheme.ok && p.scheme.v >= 0 && p.scheme.v < len(autoNumberSchemes) {
			scheme = p.scheme.v
		}
		start := 1
		if p.start.ok && p.start.v >= 1 {
			start = p.start.v
		}
		return drawingml.Bullet{Kind: drawingml.BulletNumber, Scheme: autoNumberSchemes[scheme], Start: start}, true
	}
	return drawingml.Bullet{}, false
}
