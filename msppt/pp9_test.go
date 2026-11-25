package msppt

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/odraw"
)

// testRecord returns a record of the type typ and the instance inst with
// the body.
func testRecord(typ, inst uint16, body ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint16(nil, inst<<4)
	b = binary.LittleEndian.AppendUint16(b, typ)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(bytes.Join(body, nil))))
	return append(b, bytes.Join(body, nil)...)
}

// testPNG returns a PNG file of 2 by 2 pixels.
func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pp9Tags returns a ProgTags container with the binary tag "___PPT9" that
// holds the records.
func pp9Tags(records ...[]byte) []byte {
	var name []byte
	for _, u := range units16(pp9Tag) {
		name = binary.LittleEndian.AppendUint16(name, u)
	}
	return testRecord(rtProgTags, 0, testRecord(rtProgBinaryTag, 0,
		testRecord(rtCString, 0, name), testRecord(rtBinaryTagDataBlob, 0, records...)))
}

// TestBulletPictures checks the pictures of the picture bullets of a
// document: the BlipEntity9Atom records in the binary tag "___PPT9" of its
// DocInfoListContainer, as custom-geometry.ppt of the corpus holds them.
func TestBulletPictures(t *testing.T) {
	data := testPNG(t)
	// a PNG blip: its identifier, a tag of one byte and the file
	blip := testRecord(odraw.RecBlipPNG, 0x6E0, make([]byte, 17), data)
	doc := testRecord(rtList, 0, pp9Tags(testRecord(rtBlipCollection9, 0,
		testRecord(rtBlipEntity9Atom, 0, []byte{6, 6}, blip),
		testRecord(rtBlipEntity9Atom, 0, []byte{6, 6}))))
	pics := readBulletPictures(doc, ivy.Limits{}.WithDefaults())
	if len(pics) != 2 || !bytes.Equal(pics[0], data) || pics[1] != nil {
		t.Errorf("expected the PNG file and a missing picture, got %d pictures", len(pics))
	}
}

// TestStyleProps9 checks the records of a StyleTextProp9Atom of page 42 of
// custom-geometry.ppt: a TextPFException9 with the picture and the flag of
// the automatic numbers, an empty TextCFException9 and an empty
// TextSIException.
func TestStyleProps9(t *testing.T) {
	atom := bytes.Join([][]byte{
		// the picture 1, without automatic numbers
		{0, 0, 0x80, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// automatic numbers of the scheme 7 from 3
		{0, 0, 0, 3, 1, 0, 7, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// a record that the atom cuts
		{1, 2},
	}, nil)
	ps := readStyleProps9(atom, 100)
	if len(ps) != 2 {
		t.Fatalf("expected 2 records, got %d", len(ps))
	}
	if p := ps[0]; !p.blip.ok || p.blip.v != 1 || !p.autoNumber.ok || p.autoNumber.v {
		t.Errorf("expected the picture 1 without numbers, got %+v", p)
	}
	if p := ps[1]; p.blip.ok || !p.autoNumber.v || p.scheme.v != 7 || p.start.v != 3 {
		t.Errorf("expected the numbers of the scheme 7 from 3, got %+v", p)
	}
}

// TestBullets9 checks the bullets of the paragraphs of a text with a
// StyleTextProp9Atom: the field pp9rt of the first run of a paragraph
// selects its record, which gives a picture bullet or automatic numbers.
func TestBullets9(t *testing.T) {
	le := binary.LittleEndian
	b := testBuilder(nil, nil)
	data := testPNG(t)
	b.d.doc.bulletPics = [][]byte{data}
	// one run of paragraphs with a bullet for the three paragraphs
	style := le.AppendUint32(nil, 6)
	style = le.AppendUint16(style, 0)
	style = le.AppendUint32(style, pfHasBullet)
	style = le.AppendUint16(style, 1)
	// a run of characters for each paragraph, with its pp9rt
	for i := range 3 {
		style = le.AppendUint32(style, 2)
		style = le.AppendUint32(style, cfPP9rt)
		style = le.AppendUint16(style, uint16(i)<<10)
	}
	tx := &text{kind: txOther, units: units16("a\rb\rc"), style: style}
	props9 := []pf9{{}, {}, {}}
	props9[0].blip.set(0)
	props9[1].autoNumber.set(true)
	props9[1].scheme.set(7)
	props9[1].start.set(3)
	// a picture that the document does not hold keeps the character
	props9[2].blip.set(4)
	tb := b.textBody(tx, nil, props9)
	if len(tb.Paragraphs) != 3 {
		t.Fatalf("expected 3 paragraphs, got %d", len(tb.Paragraphs))
	}
	if bu := tb.Paragraphs[0].Style.Bullet.V; bu.Kind != drawingml.BulletPicture || !bytes.Equal(bu.Picture, data) {
		t.Errorf("expected a picture bullet, got %+v", bu)
	}
	p := tb.Paragraphs[1]
	if bu := p.Style.Bullet.V; bu.Kind != drawingml.BulletNumber || bu.Scheme != "romanUcPeriod" || p.Number != 3 {
		t.Errorf("expected the number III, got %+v and %d", bu, p.Number)
	}
	if bu := tb.Paragraphs[2].Style.Bullet.V; bu.Kind != drawingml.BulletChar {
		t.Errorf("expected a bullet of a character, got %+v", bu)
	}
}
