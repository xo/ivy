package msppt

import (
	"encoding/binary"
	"slices"
	"testing"
	"unicode/utf16"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/odraw"
)

// The tests of this file come from the faults that the visual review found
// in the slides of PowerPoint 97 to 2003, against the PDF files of
// PowerPoint.

// testBuilder returns a builder of a document with the fonts, and the
// master m, which can be nil.
func testBuilder(fonts []string, m *master) *builder {
	cfg := ivy.Config{}
	cfg.Limits = cfg.Limits.WithDefaults()
	d := &Document{
		cfg:         cfg,
		doc:         &document{fonts: fonts},
		stream:      &stream{},
		masterCache: map[uint32]*master{},
	}
	d.pics = odraw.NewPictures(nil, nil, cfg)
	b := &builder{d: d, scene: drawingml.NewScene(cfg), scheme: defaultScheme, master: m, log: cfg.Log()}
	b.style = b.newStyle()
	return b
}

// testShape returns a shape of the type typ at (8, 16) of 80 by 40 points,
// with the properties.
func testShape(typ int, flags uint32, vals map[uint16]uint32) *shape {
	sh := &shape{
		Shape:     &odraw.Shape{Type: typ, Flags: flags},
		anchor:    odraw.Rect{Left: 64, Top: 128, Right: 704, Bottom: 448},
		hasAnchor: true,
		textRef:   -1,
	}
	for id, v := range vals {
		sh.Props.Set(id, v)
	}
	return sh
}

// TestFlipTurns checks that a shape that flips once turns the other way,
// as the orbits of page 9 of defense-final.ppt show it: the rotation of
// MS-ODRAW applies before the flip.
func TestFlipTurns(t *testing.T) {
	b := testBuilder(nil, nil)
	rot := uint32(30 << 16)
	b.addShape(testShape(3, 0, map[uint16]uint32{odraw.PropRotation: rot}), nil)
	b.addShape(testShape(3, odraw.FSPFlipH, map[uint16]uint32{odraw.PropRotation: rot}), nil)
	b.addShape(testShape(3, odraw.FSPFlipH|odraw.FSPFlipV, map[uint16]uint32{odraw.PropRotation: rot}), nil)
	for i, want := range []okage.Transform{
		drawingml.ShapeTransform(8, 16, 80, 40, 30, false, false),
		drawingml.ShapeTransform(8, 16, 80, 40, 330, true, false),
		drawingml.ShapeTransform(8, 16, 80, 40, 30, true, true),
	} {
		if got := b.figures[i].Transform; got != want {
			t.Errorf("shape %d: expected the transform %v, got %v", i, want, got)
		}
	}
}

// TestShapeDefaults checks the properties of a shape that it does not
// give: the defaults of MS-ODRAW, white and a line of 0.75 points, as the
// plots of page 21 of defense-final.ppt show them. A picture has no
// outline.
func TestShapeDefaults(t *testing.T) {
	b := testBuilder(nil, nil)
	b.addShape(testShape(1, 0, map[uint16]uint32{odraw.PropFillBools: odraw.FillFilled | odraw.FillUseFilled}), nil)
	b.addShape(testShape(odraw.SptPictureFrame, 0, nil), nil)
	f := b.figures[0]
	if f.Fill.Kind != drawingml.PaintSolid || f.Fill.Color != drawingml.White {
		t.Errorf("expected a white fill, got %+v", f.Fill)
	}
	if !f.Line.Visible() || f.Line.Width != 0.75 {
		t.Errorf("expected a line of 0.75 points, got %+v", f.Line)
	}
	if b.figures[1].Line.Visible() {
		t.Errorf("expected a picture without an outline, got %+v", b.figures[1].Line)
	}
}

// TestRuler checks the margins of the levels of a TextRulerAtom, which
// give the text of page 5 of custom-geometry.ppt its left margin.
func TestRuler(t *testing.T) {
	le := binary.LittleEndian
	b := le.AppendUint32(nil, rulerLevels|rulerTabs|rulerMargin1|rulerIndent1|rulerIndent1<<1)
	b = le.AppendUint16(b, 2)
	// one tab stop
	b = le.AppendUint16(b, 1)
	b = le.AppendUint32(b, 0)
	b = le.AppendUint16(b, 2)
	b = le.AppendUint16(b, 0)
	b = le.AppendUint16(b, 288)
	r := readRuler(b)
	if !r[0].margin.ok || r[0].margin.v != 2 || !r[0].indent.ok || r[0].indent.v != 0 {
		t.Errorf("expected the margin 2 and the indent 0, got %+v", r[0])
	}
	if r[1].margin.ok || !r[1].indent.ok || r[1].indent.v != 288 {
		t.Errorf("expected the indent 288 of the second level, got %+v", r[1])
	}
	if r := readRuler(b[:10]); r[0].margin.ok {
		t.Errorf("expected nothing of a short ruler, got %+v", r)
	}
}

// TestLevelInherits checks that a level of a master style takes the style
// of the levels above it, as the second level of the body of page 21 of
// defense-final.ppt shows its bullet.
func TestLevelInherits(t *testing.T) {
	m := &master{}
	m.hasStyle[txBody] = true
	m.styles[txBody][0].pf.hasBullet.set(true)
	m.styles[txBody][0].cf.font.set(0)
	m.styles[txBody][1].pf.bulletChar.set(0x2013)
	b := testBuilder([]string{"Times New Roman"}, m)
	st := b.levelStyle(txBody, 1)
	if !st.pf.hasBullet.v || st.pf.bulletChar.v != 0x2013 || !st.cf.font.ok {
		t.Errorf("expected the bullet and the font of the first level, got %+v", st)
	}
}

// TestPrivateChar checks a character of the private area from 0xF000 in a
// text of Times New Roman: it takes the symbol font of the run, as the
// arrow of page 9 of defense-final.ppt.
func TestPrivateChar(t *testing.T) {
	b := testBuilder([]string{"Times New Roman", "Symbol", "Wingdings"}, nil)
	var c cf
	if r := b.privateChar(0xF0AE, &c); r != '→' {
		t.Errorf("expected an arrow of the font Symbol, got %q", r)
	}
	c.symFont.set(2)
	if r := b.privateChar(0xF0A7, &c); r != '▪' {
		t.Errorf("expected a square of the font Wingdings, got %q", r)
	}
}

// TestLinks checks the text of a hyperlink, from a TxInteractiveInfoAtom:
// the color of the scheme for the hyperlinks and an underline, as page 38
// of custom-geometry.ppt shows it.
func TestLinks(t *testing.T) {
	b := testBuilder(nil, nil)
	var texts []text
	texts = addText(texts, header{Type: rtTextHeaderAtom}, binary.LittleEndian.AppendUint32(nil, txOther))
	texts = addText(texts, header{Type: rtTextBytesAtom}, []byte("see www.x.org now"))
	link := binary.LittleEndian.AppendUint32(nil, 4)
	link = binary.LittleEndian.AppendUint32(link, 13)
	texts = addText(texts, header{Type: rtTxInteractiveInfo}, link)
	tb := b.textBody(&texts[0], nil, nil)
	runs := tb.Paragraphs[0].Runs
	if len(runs) != 3 || runs[1].Text != "www.x.org" {
		t.Fatalf("expected the link in a run of its own, got %+v", runs)
	}
	if st := runs[1].Style; st.Paint.V.Color != defaultScheme[schemeHyperlink] || !st.Underline.V {
		t.Errorf("expected an underlined link of the color of the scheme, got %+v", st)
	}
	if runs[2].Style.Underline.V {
		t.Errorf("expected the text after the link without an underline, got %+v", runs[2].Style)
	}
}

// TestNoBulletIndent checks that the text of a paragraph without a bullet
// starts at its left margin: the indent is the place of the bullet, as
// page 38 of custom-geometry.ppt shows it.
func TestNoBulletIndent(t *testing.T) {
	b := testBuilder(nil, nil)
	var p pf
	p.leftMargin.set(216)
	p.indent.set(0)
	p.hasBullet.set(false)
	ps := b.paraStyle(&p, &cf{})
	if ps.MarL.V != 27 || ps.Indent.V != 0 {
		t.Errorf("expected a margin of 27 points without an indent, got %v and %v", ps.MarL.V, ps.Indent.V)
	}
	p.hasBullet.set(true)
	ps = b.paraStyle(&p, &cf{})
	if ps.MarL.V != 27 || ps.Indent.V != -27 {
		t.Errorf("expected a hanging bullet, got %v and %v", ps.MarL.V, ps.Indent.V)
	}
}

// TestHeadersFooters checks that the placeholders of the date, the slide
// number and the footer of the master show as the HeadersFootersAtom of
// the slide says, with the footer and the date that it gives.
func TestHeadersFooters(t *testing.T) {
	ph := func(typ int, s string) *shape {
		sh := testShape(1, 0, nil)
		sh.placeholder = typ
		sh.text = &text{kind: txOther, units: units16(s)}
		if s == "*" {
			sh.text.fields = []int{0}
		}
		return sh
	}
	shapes := []*shape{ph(phMasterDate, "date"), ph(phMasterSlideNumber, "*"), ph(phMasterFooter, "master footer"), ph(1, "title")}
	root := &odraw.Shape{}
	dr := &drawing{od: &odraw.Drawing{Root: root}, byShape: map[*odraw.Shape]*shape{}}
	for _, sh := range shapes {
		root.Children = append(root.Children, sh.Shape)
		dr.byShape[sh.Shape] = sh
	}
	m := &master{drawing: dr, spids: map[uint32]*shape{}}
	for _, tt := range []struct {
		name string
		hf   headersFooters
		want []string
	}{
		{"none", headersFooters{}, []string{}},
		{"number and footer", headersFooters{flags: hfHasSlideNumber | hfHasFooter, footer: units16("Footer")}, []string{"7", "Footer"}},
		{"user date and master footer", headersFooters{flags: hfHasDate | hfHasUserDate | hfHasFooter, userDate: units16("May 1")}, []string{"May 1", "master footer"}},
		{"date of today", headersFooters{flags: hfHasDate}, []string{}},
	} {
		b := testBuilder(nil, m)
		b.number = 7
		b.hf = tt.hf
		b.masterObjects(m, 2)
		got := make([]string, 0, len(b.figures))
		for _, f := range b.figures {
			got = append(got, textOf(f))
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: expected %q, got: %q", tt.name, tt.want, got)
		}
	}
}

// TestReadHeadersFooters checks the flags and the strings of a
// HeadersFootersContainer of the slides, and that the one of the notes is
// left out.
func TestReadHeadersFooters(t *testing.T) {
	le := binary.LittleEndian
	rec := func(typ, inst uint16, body []byte) []byte {
		b := le.AppendUint16(nil, inst<<4)
		b = le.AppendUint16(b, typ)
		b = le.AppendUint32(b, uint32(len(body)))
		return append(b, body...)
	}
	str := func(s string) []byte {
		var b []byte
		for _, u := range units16(s) {
			b = le.AppendUint16(b, u)
		}
		return b
	}
	atom := func(flags uint16) []byte { return rec(rtHeadersFootersAtom, 0, le.AppendUint16([]byte{0, 0}, flags)) }
	notes := rec(rtHeadersFooters, 4, atom(hfHasSlideNumber))
	slides := rec(rtHeadersFooters, hfSlides, append(append(atom(hfHasFooter|hfHasDate|hfHasUserDate), rec(rtCString, 0, str("1 May"))...), rec(rtCString, 2, str("Foot"))...))
	hf := readHeadersFooters(append(notes, slides...))
	if !hf.ok || hf.flags != hfHasFooter|hfHasDate|hfHasUserDate || string(utf16.Decode(hf.footer)) != "Foot" || string(utf16.Decode(hf.userDate)) != "1 May" {
		t.Errorf("expected the footer Foot and the date 1 May, got: %+v", hf)
	}
}

// units16 returns the characters of s in UTF-16.
func units16(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

// TestWordArt checks that a shape of WordArt draws the outlines of its
// text with its fill, and not the rectangle of its type.
func TestWordArt(t *testing.T) {
	var text []byte
	for _, r := range "WordArt" {
		text = binary.LittleEndian.AppendUint16(text, uint16(r))
	}
	sh := testShape(odraw.SptTextEffectMin, 0, map[uint16]uint32{odraw.PropFillColor: 0xFF})
	opt := binary.LittleEndian.AppendUint16(nil, odraw.PropGtextUnicode|0x8000)
	opt = binary.LittleEndian.AppendUint32(opt, uint32(len(text)))
	sh.Props.Read(append(opt, text...), 1)
	b := testBuilder(nil, nil)
	b.addShape(sh, nil)
	if len(b.figures) != 1 {
		t.Fatalf("expected one figure, got %d", len(b.figures))
	}
	f := b.figures[0]
	if len(f.Paths) != 1 || f.Fill.Kind != drawingml.PaintSolid || f.Fill.Color.R != 1 {
		t.Fatalf("expected the glyphs of one line with the red fill, got %d paths and %+v", len(f.Paths), f.Fill)
	}
	// the outlines of the glyphs have far more points than a rectangle
	if n := len(f.Paths[0].Path.Points()); n < 20 {
		t.Errorf("expected the outlines of the glyphs, got %d points", n)
	}
}
