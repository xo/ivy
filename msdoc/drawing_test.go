package msdoc

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"
)

// The tests of this file check the text boxes, the shapes of Office
// Drawing and the notes, with streams that they build in memory.

// rec returns a record of Office Drawing: its version and instance, its
// type and its body.
func rec(ver, inst, typ uint16, body ...[]byte) []byte {
	var data []byte
	for _, b := range body {
		data = append(data, b...)
	}
	out := u16s(inst<<4|ver, typ)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(data)))
	return append(out, data...)
}

// fsp returns an OfficeArtFSPContainer of the shape spid of the type typ,
// with the flags and the properties of its OfficeArtFOPT, which come in
// pairs of an id and a value, and more records.
func fsp(spid uint32, typ uint16, flags uint32, props []uint32, more ...[]byte) []byte {
	var opt []byte
	for i := 0; i+1 < len(props); i += 2 {
		opt = append(opt, u16s(uint16(props[i]))...)
		opt = binary.LittleEndian.AppendUint32(opt, props[i+1])
	}
	body := [][]byte{rec(2, typ, rtFSP, u32s(spid, flags))}
	if len(props) > 0 {
		body = append(body, rec(3, uint16(len(props)/2), rtFOPT, opt))
	}
	body = append(body, more...)
	return rec(0xF, 0, rtSpContainer, body...)
}

// fspa returns an FSPA of the shape spid at the rectangle r in twips, with
// the flags.
func fspa(spid uint32, r [4]int32, flags uint16) []byte {
	b := u32s(spid, uint32(r[0]), uint32(r[1]), uint32(r[2]), uint32(r[3]))
	b = append(b, u16s(flags)...)
	return append(b, 0, 0, 0, 0)
}

// drawingParser returns a parser whose drawing holds a rectangle 1025, a
// group 1026 of a rectangle and an ellipse, a text box 1027 and a line
// 1028, anchored at the characters 0, 1, 2 and 3 of the main document. The
// text of the text box is the characters 5 to 13 of the stream
// WordDocument, which holds one byte for each character.
func drawingParser(t *testing.T) *parser {
	t.Helper()
	const blue = 0xFFCC99
	rect := fsp(1025, shapeTypeRect, 0xA00, []uint32{propFillColor, blue, propLineColor, 0})
	group := rec(0xF, 0, rtSpgrContainer,
		fsp(1026, 0, fspGroup|0x200, nil, rec(1, 0, rtFSPGR, u32s(0, 0, 200, 100))),
		fsp(1029, shapeTypeRect, 0xA02, nil, rec(0, 0, rtChildAnchor, u32s(0, 0, 100, 100))),
		fsp(1030, shapeTypeEllipse, 0xA02, nil, rec(0, 0, rtChildAnchor, u32s(100, 0, 200, 100))),
	)
	// no line, and a red fill
	box := fsp(1027, shapeTypeTextBox, 0xA00, []uint32{propLTxid, 0x10000, propLineBools, 1 << 19, propFillColor, 0x0000FF})
	line := fsp(1028, shapeTypeLine, 0xA80, []uint32{propLineWidth, 12700})
	patriarch := fsp(1024, 0, fspGroup|0x4, nil, rec(1, 0, rtFSPGR, make([]byte, 16)))
	dg := rec(0xF, 0, rtDgContainer, rec(0, 1, 0xF008, u32s(5, 1031)),
		rec(0xF, 0, rtSpgrContainer, patriarch, rect, group, box, line))
	dgg := append(rec(0xF, 0, rtDggContainer, rec(0, 0, 0xF006, make([]byte, 16))), 0)
	dgg = append(dgg, dg...)
	// the PlcSpaMom: 5 characters and 4 FSPAs, square wrapping and no
	// wrapping at the margin
	square, front := uint16(2<<5), uint16(3<<5)
	spa := u32s(0, 1, 2, 3, 100)
	spa = append(spa, fspa(1025, [4]int32{1440, 0, 4320, 1440}, front)...)
	spa = append(spa, fspa(1026, [4]int32{0, 0, 2880, 1440}, front)...)
	spa = append(spa, fspa(1027, [4]int32{0, 0, 2880, 720}, square)...)
	spa = append(spa, fspa(1028, [4]int32{0, 0, 2880, 0}, front)...)
	// the PlcftxbxTxt: the text box 1027 and the last FTXBXS
	txbx := u32s(0, 9, 10)
	ftxbxs := func(lid uint32) []byte {
		b := u32s(1, 0)
		b = append(b, 0, 0)
		return append(b, u32s(0xFFFFFFFF, lid, 0)...)
	}
	txbx = append(txbx, ftxbxs(1027)...)
	txbx = append(txbx, ftxbxs(0)...)
	p := testParser(nil, nil)
	for i, b := range map[int][]byte{fcDggInfo: dgg, fcPlcSpaMom: spa, fcPlcftxbxTxt: txbx} {
		p.s.fib.fcLcb[i] = [2]uint32{uint32(len(p.s.table)), uint32(len(b))}
		p.s.table = append(p.s.table, b...)
	}
	p.s.word = []byte("ABCD\rBox text\r")
	p.s.fib.ccpText, p.s.fib.ccpTxbx = 5, 9
	p.pieces = []piece{{cpEnd: 14, compressed: true}}
	for i := range p.ansi {
		p.ansi[i] = rune(i)
	}
	return p
}

// TestDrawing checks that the shapes of Office Drawing draw as SVG images,
// with their fills and lines, and the shapes of a group in the coordinates
// of the group (poi/doc/with-art-shapes.doc).
func TestDrawing(t *testing.T) {
	p := drawingParser(t)
	d, err := p.drawnAt(t.Context(), 0, false)
	if err != nil || d == nil || d.text {
		t.Fatalf("expected the rectangle, got %+v, %v", d, err)
	}
	svg := string(p.media[len(p.media)-1].data)
	for _, want := range []string{`d="M0 0 L144 0 L144 72 L0 72 Z"`, `fill="#99ccff"`, `stroke="#000000" stroke-width="0.75"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("expected %q in the svg:\n%s", want, svg)
		}
	}
	var b strings.Builder
	p.writeDrawn(&b, d)
	if html := b.String(); !strings.Contains(html, `position: absolute; left: 71.15pt; top: -0.85pt`) || !strings.Contains(html, ".svg") {
		t.Errorf("expected the image at the left of 72 points, got: %s", html)
	}
	if d, err = p.drawnAt(t.Context(), 1, false); err != nil || d == nil {
		t.Fatalf("expected the group, got %v", err)
	}
	svg = string(p.media[len(p.media)-1].data)
	// the group of 200 by 100 units is 144 by 72 points
	for _, want := range []string{`d="M0 0 L72 0 L72 72 L0 72 Z"`, `translate(72 0)`, `A36 36 0 1 0 72 36`} {
		if !strings.Contains(svg, want) {
			t.Errorf("expected %q in the svg of the group:\n%s", want, svg)
		}
	}
	// a solid line is a stroke of 1 point
	if d, err = p.drawnAt(t.Context(), 3, false); err != nil || d == nil {
		t.Fatalf("expected the line, got %v", err)
	}
	data := p.media[len(p.media)-1].data
	svg = string(data)
	if !strings.Contains(svg, `d="M0 0 L144 0" fill="none" stroke="#000000" stroke-width="1"`) {
		t.Errorf("expected the stroke of the line in the svg:\n%s", svg)
	}
	// the line covers a row of pixels at 96 dpi. At 18 dpi it is a quarter
	// of a pixel wide on an image of one row, and draws light
	for _, c := range []struct {
		s    float32
		want uint8
	}{{96.0 / 72, 0xff}, {18.0 / 72, 0x20}} {
		if a := darkest(t, data, c.s); a < c.want {
			t.Errorf("expected an alpha of at least %d at the scale %g, got %d", c.want, c.s, a)
		}
	}
	if d, _ := p.drawnAt(t.Context(), 4, false); d != nil {
		t.Errorf("expected no shape at the character 4, got %+v", d)
	}
}

// darkest renders the SVG image data at s pixels for each point, and
// returns the largest alpha of its pixels.
func darkest(t *testing.T, data []byte, s float32) uint8 {
	t.Helper()
	tree, err := svg.FromData(data)
	if err != nil {
		t.Fatal(err)
	}
	// the size of the tree is in the pixels of CSS, 96 for each inch
	k := s * 72 / 96
	size := tree.Size()
	pm, ok := okage.NewPixmap(uint32(math.Ceil(float64(size.Width()*k))), uint32(math.Ceil(float64(size.Height()*k))))
	if !ok {
		t.Fatal("no pixmap")
	}
	mut := pm.AsMut()
	svg.Render(tree, okage.TransformFromScale(k, k), &mut)
	var a uint8
	for i := 3; i < len(pm.Data()); i += 4 {
		a = max(a, pm.Data()[i])
	}
	return a
}

// TestTextbox checks that a text box writes its text from the part of
// the text boxes, as a block that floats before the paragraph of its
// anchor (libreoffice/doc/landscape-a4-certificate.doc).
func TestTextbox(t *testing.T) {
	p := drawingParser(t)
	d, err := p.drawnAt(t.Context(), 2, false)
	if err != nil || d == nil || !d.text {
		t.Fatalf("expected the text box, got %+v, %v", d, err)
	}
	para := paragraph{runs: []run{{drawn: d}, {text: "After"}}}
	var b strings.Builder
	p.writeParagraph(&b, para)
	html := b.String()
	for _, want := range []string{
		`<div style="float: left; margin-left: 0pt; margin-right: 9pt; width: 144pt; height: 36pt;`,
		`padding: 3.6pt 7.2pt 3.6pt 7.2pt`, `background-color: #ff0000`, "Box text</span></p>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the html:\n%s", want, html)
		}
	}
	if strings.Contains(html, "border:") {
		t.Errorf("expected a text box without a line:\n%s", html)
	}
	if !strings.HasPrefix(html, "<div") || !strings.Contains(html, "</div><p>After</p>") {
		t.Errorf("expected the text box before the paragraph:\n%s", html)
	}
}

// TestFullWidthFloat checks that an object that leaves less than half an
// inch beside it puts the text below it, as Word does
// (libreoffice/doc/landscape-letter-form.doc, whose text box spans the
// page).
func TestFullWidthFloat(t *testing.T) {
	p := testParser(nil, nil)
	p.page = letter
	col := float64(letter.width-letter.left-letter.right) / 20
	got := strings.Join(p.floatStyle(&floating{wr: 2}, col-10, 20), "; ")
	if !strings.HasPrefix(got, "display: block") {
		t.Errorf("expected a block, got %q", got)
	}
	got = strings.Join(p.floatStyle(&floating{wr: 2}, col/2, 20), "; ")
	if !strings.HasPrefix(got, "float: left") {
		t.Errorf("expected a float, got %q", got)
	}
}

// TestNotes checks that a footnote goes into the paragraph of its
// reference, with its number, and that an endnote goes to the end of the
// document with a Roman number.
func TestNotes(t *testing.T) {
	// the main document, a footnote and an endnote, each after the
	// character 0x02 of its reference or its number
	text := "Text\x02 and\x02\r" + "\x02 Foot\r" + "\x02 End\r"
	pairs := map[int][]byte{
		fcPlcffndRef: append(u32s(4, 11), u16s(1)...),
		fcPlcffndTxt: u32s(0, 7, 8),
		fcPlcfendRef: append(u32s(9, 11), u16s(1)...),
		fcPlcfendTxt: u32s(0, 6, 7),
	}
	p := testParser(nil, nil)
	for i, b := range pairs {
		p.s.fib.fcLcb[i] = [2]uint32{uint32(len(p.s.table)), uint32(len(b))}
		p.s.table = append(p.s.table, b...)
	}
	p.s.word = []byte(text)
	p.s.fib.ccpText, p.s.fib.ccpFtn, p.s.fib.ccpEdn = 11, 7, 6
	p.pieces = []piece{{cpEnd: uint32(len(text)), compressed: true}}
	for i := range p.ansi {
		p.ansi[i] = rune(i)
	}
	// the characters 0x02 are special
	spec := append(u16s(sprmCFSpec), 1)
	for _, fc := range []uint32{4, 9, 11, 18} {
		p.chpx = append(p.chpx, fkpRun{fcStart: fc, fcEnd: fc + 1, grpprl: spec})
	}
	paras, err := p.paragraphs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	html := p.body(paras, nil)
	for _, want := range []string{
		`Text</span><span class="fn"><span style="display: block; white-space: pre-wrap; font-size: 10pt">` +
			`<span style="font-size: 10pt"><span class="fnref"></span></span><span style="font-size: 10pt"> Foot</span></span>`,
		` and</span><span class="enref">i</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the html:\n%s", want, html)
		}
	}
	if got := p.endnotes.String(); !strings.Contains(got, `<span style="font-size: 10pt">i</span><span style="font-size: 10pt"> End</span></p>`) {
		t.Errorf("expected the endnote with its number, got: %s", got)
	}
}
