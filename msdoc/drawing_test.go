package msdoc

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/odraw"
)

// The tests of this file check the text boxes, the shapes of Office
// Drawing and the notes, with streams that they build in memory.

// The ids of the properties of the shapes of the tests (MS-ODRAW 2.3).
const (
	propLTxid = 0x0080
	spGroup   = 0x0001
)

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
	body := [][]byte{rec(2, typ, odraw.RecFSP, u32s(spid, flags))}
	if len(props) > 0 {
		body = append(body, rec(3, uint16(len(props)/2), odraw.RecFOPT, opt))
	}
	body = append(body, more...)
	return rec(0xF, 0, odraw.RecSpContainer, body...)
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
	rect := fsp(1025, odraw.SptRectangle, 0xA00, []uint32{odraw.PropFillColor, blue, odraw.PropLineColor, 0})
	group := rec(0xF, 0, odraw.RecSpgrContainer,
		fsp(1026, 0, spGroup|0x200, nil, rec(1, 0, odraw.RecFSPGR, u32s(0, 0, 200, 100))),
		fsp(1029, odraw.SptRectangle, 0xA02, []uint32{odraw.PropFillColor, blue}, rec(0, 0, odraw.RecChildAnchor, u32s(0, 0, 100, 100))),
		fsp(1030, odraw.SptEllipse, 0xA02, []uint32{odraw.PropFillColor, blue}, rec(0, 0, odraw.RecChildAnchor, u32s(100, 0, 200, 100))),
	)
	// no line, and a red fill
	box := fsp(1027, odraw.SptTextBox, 0xA00, []uint32{propLTxid, 0x10000, odraw.PropLineBools, 1 << 19, odraw.PropFillColor, 0x0000FF})
	line := fsp(1028, odraw.SptLine, 0xA80, []uint32{odraw.PropLineWidth, 12700})
	patriarch := fsp(1024, 0, spGroup|0x4, nil, rec(1, 0, odraw.RecFSPGR, make([]byte, 16)))
	dg := rec(0xF, 0, odraw.RecDgContainer, rec(0, 1, 0xF008, u32s(5, 1031)),
		rec(0xF, 0, odraw.RecSpgrContainer, patriarch, rect, group, box, line))
	dgg := append(rec(0xF, 0, odraw.RecDggContainer, rec(0, 0, 0xF006, make([]byte, 16))), 0)
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

// rendered is an object of the layout that a test drew, at s pixels for a
// point.
type rendered struct {
	pm *okage.Pixmap
	s  float32
}

// renderObject draws the object of the layout at the URL url at s pixels
// for a point.
func renderObject(t *testing.T, p *parser, url string, s float32) rendered {
	t.Helper()
	obj := p.objects[strings.TrimPrefix(url, layout.ObjectScheme+":")]
	if obj == nil {
		t.Fatalf("expected the object %q", url)
	}
	w, h := obj.Size()
	pm, ok := okage.NewPixmap(uint32(math.Ceil(w*float64(s))), uint32(math.Ceil(h*float64(s))))
	if !ok {
		t.Fatalf("no pixmap of %g by %g points", w, h)
	}
	rect, _ := okage.RectFromXYWH(0, 0, float32(w), float32(h))
	if err := obj.Draw(t.Context(), display.NewOkage(pm, okage.TransformFromScale(s, s)), rect); err != nil {
		t.Fatal(err)
	}
	return rendered{pm: pm, s: s}
}

// at returns the color of the pixel at x, y points, premultiplied.
func (r rendered) at(x, y float64) [4]uint8 {
	c, _ := r.pm.Pixel(uint32(x*float64(r.s)), uint32(y*float64(r.s)))
	return [4]uint8{c.Red(), c.Green(), c.Blue(), c.Alpha()}
}

// dark returns the number of pixels that are dark and opaque.
func (r rendered) dark() int {
	n := 0
	data := r.pm.Data()
	for i := 0; i+4 <= len(data); i += 4 {
		if data[i+3] > 0x80 && data[i] < 0x60 && data[i+1] < 0x60 && data[i+2] < 0x60 {
			n++
		}
	}
	return n
}

// TestDrawing checks that the shapes of Office Drawing draw as objects of
// the layout, with their fills and lines, and the shapes of a group in the
// coordinates of the group (poi/doc/with-art-shapes.doc).
func TestDrawing(t *testing.T) {
	p := drawingParser(t)
	d, err := p.drawnAt(t.Context(), 0, false)
	if err != nil || d == nil || d.text {
		t.Fatalf("expected the rectangle, got %+v, %v", d, err)
	}
	// the line of 0.75 points takes a bleed of 2 points
	if d.pic.pad != 2 {
		t.Errorf("expected a bleed of 2 points, got %g", d.pic.pad)
	}
	var b strings.Builder
	p.writeDrawn(&b, d)
	if html := b.String(); !strings.Contains(html, `position: absolute; left: 70pt; top: -2pt`) ||
		!strings.Contains(html, `src="ivy-object:shape1"`) || !strings.Contains(html, "width: 148pt; height: 76pt") {
		t.Errorf("expected the object at the left of 72 points, got: %s", html)
	}
	r := renderObject(t, p, d.pic.name, 2)
	if got, want := r.at(74, 38), [4]uint8{0x99, 0xcc, 0xff, 0xff}; got != want {
		t.Errorf("expected the blue fill in the middle, got %v", got)
	}
	if got := r.at(2, 38); got[3] < 0x80 || got[0] > 0x40 {
		t.Errorf("expected the black line at the left, got %v", got)
	}
	if d, err = p.drawnAt(t.Context(), 1, false); err != nil || d == nil {
		t.Fatalf("expected the group, got %v", err)
	}
	// the group of 200 by 100 units is 144 by 72 points: the rectangle
	// fills its corner, and the ellipse does not
	r = renderObject(t, p, d.pic.name, 2)
	for _, c := range []struct {
		x, y   float64
		filled bool
	}{{38, 38, true}, {110, 38, true}, {5, 5, true}, {77, 5, false}} {
		if got := r.at(c.x, c.y); (got == [4]uint8{0x99, 0xcc, 0xff, 0xff}) != c.filled {
			t.Errorf("expected the fill at %g, %g to be %v, got %v", c.x, c.y, c.filled, got)
		}
	}
	// a line has no fill
	if d, err = p.drawnAt(t.Context(), 3, false); err != nil || d == nil {
		t.Fatalf("expected the line, got %v", err)
	}
	r = renderObject(t, p, d.pic.name, 2)
	if got := r.at(d.pic.pad+72, d.pic.pad); got[3] < 0x80 || got[0] > 0x40 {
		t.Errorf("expected the line, got %v", got)
	}
	if got := r.at(d.pic.pad+72, d.pic.pad+1); got[3] != 0 {
		t.Errorf("expected nothing beside the line, got %v", got)
	}
	if d, _ := p.drawnAt(t.Context(), 4, false); d != nil {
		t.Errorf("expected no shape at the character 4, got %+v", d)
	}
}

// TestTextbox checks that a text box writes its text from the part of
// the text boxes, as a block that floats before the paragraph of its
// anchor, over the object of its shape
// (libreoffice/doc/landscape-a4-certificate.doc). Its place is from the
// margin, so it floats at its position from the top of the margin (D60).
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
	// without a line, the bleed is 1 point, which the padding holds
	for _, want := range []string{
		`<div style="float: left; margin-left: 0pt; margin-right: 9pt; -ivy-float-position: margin -1pt; -ivy-wrap-flow: both 9pt; width: 146pt; height: 38pt;`,
		`padding: 4.6pt 8.2pt 4.6pt 8.2pt; background: url(ivy-object:shape1) no-repeat; background-size: 100% 100%">`,
		"Box text</span></p>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the html:\n%s", want, html)
		}
	}
	if !strings.HasPrefix(html, "<div") || !strings.Contains(html, "</div><p>After</p>") {
		t.Errorf("expected the text box before the paragraph:\n%s", html)
	}
	r := renderObject(t, p, d.obj, 2)
	if got, want := r.at(73, 19), [4]uint8{0xff, 0, 0, 0xff}; got != want {
		t.Errorf("expected the red fill of the text box, got %v", got)
	}
	if n := r.dark(); n != 0 {
		t.Errorf("expected a text box without a line, got %d dark pixels", n)
	}
}

// TestGroupText checks that the text of a shape of a group shows on the
// object of the group, at the place of the shape, and that a line draws the
// arrowhead at its end.
func TestGroupText(t *testing.T) {
	p := drawingParser(t)
	plain, err := p.drawnAt(t.Context(), 3, false)
	if err != nil || plain == nil {
		t.Fatalf("expected the line, got %v", err)
	}
	// the ellipse of the group holds the text of the text box, and the
	// line has a triangle at its end
	p.txbxs[0][1030] = p.txbxs[0][1027]
	p.drawing.shapes[1028].Props.Set(odraw.PropLineEndArrow, 1)
	d, err := p.drawnAt(t.Context(), 1, false)
	if err != nil || d == nil || len(d.texts) != 1 {
		t.Fatalf("expected the group with one text, got %+v, %v", d, err)
	}
	var b strings.Builder
	p.writeParagraph(&b, paragraph{runs: []run{{drawn: d}, {text: "After"}}})
	html := b.String()
	for _, want := range []string{
		`<div style="position: absolute; left: -2pt; top: -2pt; width: 148pt; height: 76pt">`,
		`display: block; width: 148pt; height: 76pt`,
		`position: absolute; left: 74pt; top: 2pt; width: 72pt; height: 72pt`, "Box text</span></p>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the html:\n%s", want, html)
		}
	}
	if !strings.HasPrefix(html, "<div") || !strings.Contains(html, "</div><p>After</p>") {
		t.Errorf("expected the group before the paragraph:\n%s", html)
	}
	arrow, err := p.drawnAt(t.Context(), 3, false)
	if err != nil || arrow == nil {
		t.Fatalf("expected the line, got %v", err)
	}
	if arrow.pic.pad <= plain.pic.pad {
		t.Errorf("expected a larger bleed for the arrowhead, got %g and %g", arrow.pic.pad, plain.pic.pad)
	}
	if n, m := renderObject(t, p, arrow.pic.name, 4).dark(), renderObject(t, p, plain.pic.name, 4).dark(); n <= m {
		t.Errorf("expected the arrowhead to draw more than the line, got %d and %d dark pixels", n, m)
	}
}

// TestWordArt checks that a shape of WordArt draws the outlines of its
// text with its fill, and not the box of the shape.
func TestWordArt(t *testing.T) {
	p := drawingParser(t)
	if d, _ := p.drawnAt(t.Context(), 4, false); d != nil {
		t.Fatalf("expected no shape at the character 4, got %+v", d)
	}
	var text []byte
	for _, r := range "WordArt" {
		text = binary.LittleEndian.AppendUint16(text, uint16(r))
	}
	opt := binary.LittleEndian.AppendUint16(nil, odraw.PropGtextUnicode|0x8000)
	opt = binary.LittleEndian.AppendUint32(opt, uint32(len(text)))
	sh := p.drawing.shapes[1025]
	sh.Type = odraw.SptTextEffectMin
	sh.Props.Read(append(opt, text...), 1)
	d, err := p.drawnAt(t.Context(), 0, false)
	if err != nil || d == nil {
		t.Fatalf("expected the WordArt, got %+v, %v", d, err)
	}
	r := renderObject(t, p, d.pic.name, 1)
	blue, all := 0, 0
	data := r.pm.Data()
	for i := 0; i+4 <= len(data); i += 4 {
		all++
		if data[i] == 0x99 && data[i+1] == 0xcc && data[i+2] == 0xff && data[i+3] == 0xff {
			blue++
		}
	}
	// the glyphs cover a part of the box of 144 by 72 points
	if blue < 500 || blue > all*3/4 {
		t.Errorf("expected the blue glyphs to cover a part of the box, got %d of %d pixels", blue, all)
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
	got := strings.Join(p.floatStyle(&floating{wr: 2, by: 2}, col-10, 20), "; ")
	if !strings.HasPrefix(got, "display: block") {
		t.Errorf("expected a block, got %q", got)
	}
	got = strings.Join(p.floatStyle(&floating{wr: 2, by: 2}, col/2, 20), "; ")
	if !strings.HasPrefix(got, "float: left") {
		t.Errorf("expected a float, got %q", got)
	}
}

// TestPageFloatShape checks that an object at a position from the margin
// or from the page floats at that position, and that an object above and
// below the text has a margin to the right side of the column (D60).
func TestPageFloatShape(t *testing.T) {
	p := testParser(nil, nil)
	p.page = letter
	col := float64(letter.width-letter.left-letter.right) / 20
	for _, tt := range []struct {
		name string
		f    floating
		w    float64
		want string
	}{
		{"margin", floating{wr: 2, by: 0, left: 1440, top: 2880}, 144,
			"float: left; margin-left: 72pt; margin-right: 9pt; -ivy-float-position: margin 144pt; -ivy-wrap-flow: both 9pt; width: 144pt; height: 20pt"},
		{"page", floating{wr: 2, by: 1, left: 1440, top: 2880}, 144,
			"float: left; margin-left: 72pt; margin-right: 9pt; -ivy-float-position: page 144pt; -ivy-wrap-flow: both 9pt; width: 144pt; height: 20pt"},
		{"right", floating{wr: 4, by: 0, left: 5760, top: 0}, 144,
			"float: right; margin-right: 0pt; margin-left: 9pt; -ivy-float-position: margin 0pt; -ivy-wrap-flow: both 9pt; width: 144pt; height: 20pt"},
		{"top and bottom", floating{wr: 1, by: 0, left: 1440, top: 720}, 144,
			"float: left; margin-left: 72pt; margin-right: 216pt; -ivy-float-position: margin 36pt; width: 144pt; height: 20pt"},
		// an object that leaves less than half an inch beside it puts the
		// text below it
		{"full width", floating{wr: 2, by: 1, top: 1440}, col - 10,
			"float: left; margin-left: 0pt; margin-right: 10pt; -ivy-float-position: page 72pt; width: 422pt; height: 20pt"},
		// the paragraph and no wrap stay as they are
		{"paragraph", floating{wr: 2, by: 2, top: 200}, 144,
			"float: left; margin-left: 0pt; margin-right: 9pt; margin-top: 10pt; -ivy-wrap-flow: both 9pt; width: 144pt; height: 20pt"},
		{"none", floating{wr: 3, by: 0, top: 200}, 144,
			"position: absolute; left: 0pt; top: 10pt; width: 144pt; height: 20pt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(p.floatStyle(&tt.f, tt.w, 20), "; "); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWrapSide checks that the text wraps around an object on the sides of
// the wrk of its FSPA, 9 points from it, and that the text above and below
// an object and no wrap give no side (D63).
func TestWrapSide(t *testing.T) {
	p := testParser(nil, nil)
	p.page = letter
	for _, tt := range []struct {
		f    floating
		want string
	}{
		{floating{wr: 2, wrk: 0, left: 2880}, "-ivy-wrap-flow: both 9pt"},
		{floating{wr: 2, wrk: 1, left: 2880}, "-ivy-wrap-flow: start 9pt"},
		{floating{wr: 4, wrk: 2, left: 2880}, "-ivy-wrap-flow: end 9pt"},
		{floating{wr: 5, wrk: 3, left: 2880}, "-ivy-wrap-flow: maximum 9pt"},
		{floating{wr: 1, by: 0, left: 2880}, ""},
		{floating{wr: 3, left: 2880}, ""},
	} {
		style := p.floatStyle(&tt.f, 72, 20)
		got := ""
		for _, s := range style {
			if strings.HasPrefix(s, "-ivy-wrap-flow") {
				got = s
			}
		}
		if got != tt.want {
			t.Errorf("wr %d, wrk %d: got %q, want %q in %q", tt.f.wr, tt.f.wrk, got, tt.want, style)
		}
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
