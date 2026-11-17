package msppt

import (
	"encoding/binary"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
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
		masterCache: map[uint32]*master{},
		pictures:    map[int]*drawingml.Picture{},
		rawPictures: map[int][]byte{},
		keyed:       map[pictureKey]*drawingml.Picture{},
	}
	return &builder{d: d, scene: drawingml.NewScene(cfg), scheme: defaultScheme, master: m, log: cfg.Log()}
}

// testShape returns a shape of the type typ at (8, 16) of 80 by 40 points,
// with the properties.
func testShape(typ int, flags uint32, vals map[uint16]uint32) *shape {
	return &shape{
		typ:       typ,
		flags:     flags,
		props:     props{vals: vals},
		anchor:    rect{left: 64, top: 128, right: 704, bottom: 448},
		hasAnchor: true,
		textRef:   -1,
	}
}

// TestFlipTurns checks that a shape that flips once turns the other way,
// as the orbits of page 9 of defense-final.ppt show it: the rotation of
// MS-ODRAW applies before the flip.
func TestFlipTurns(t *testing.T) {
	b := testBuilder(nil, nil)
	rot := uint32(30 << 16)
	b.addShape(testShape(3, 0, map[uint16]uint32{propRotation: rot}), nil)
	b.addShape(testShape(3, fspFlipH, map[uint16]uint32{propRotation: rot}), nil)
	b.addShape(testShape(3, fspFlipH|fspFlipV, map[uint16]uint32{propRotation: rot}), nil)
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
	b.addShape(testShape(1, 0, map[uint16]uint32{propFillBools: fillFilled | fillUseFilled}), nil)
	b.addShape(testShape(sptPictureFrame, 0, nil), nil)
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

// TestPathGradient checks that a gradient from the center starts at the
// focus with the back color, as page 4 of backgrounds.ppt shows it, and
// that the colors of a shading mix in linear light.
func TestPathGradient(t *testing.T) {
	b := testBuilder(nil, nil)
	pt := b.fill(chain{&props{vals: map[uint16]uint32{
		propFillType:      fillShadeCenter,
		propFillColor:     0x404040,
		propFillBackColor: 0xFFFFFF,
	}}})
	g := pt.Gradient
	if pt.Kind != drawingml.PaintGradient || g.Path != "rect" || len(g.Stops) != gammaSteps+1 {
		t.Fatalf("expected a path gradient, got %+v", pt)
	}
	if first, last := g.Stops[0].Color, g.Stops[len(g.Stops)-1].Color; first != drawingml.White || last.NRGBA().R != 0x40 {
		t.Errorf("expected white at the focus and gray at the edge, got %v and %v", first, last)
	}
	// the middle of white and of 40% gray in linear light is lighter than
	// the middle of their values
	if mid := g.Stops[gammaSteps/2].Color.NRGBA().R; mid < 0xb4 {
		t.Errorf("expected a light middle, got %#x", mid)
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
	texts = addText(texts, header{typ: rtTextHeaderAtom}, binary.LittleEndian.AppendUint32(nil, txOther))
	texts = addText(texts, header{typ: rtTextBytesAtom}, []byte("see www.x.org now"))
	link := binary.LittleEndian.AppendUint32(nil, 4)
	link = binary.LittleEndian.AppendUint32(link, 13)
	texts = addText(texts, header{typ: rtTxInteractiveInfo}, link)
	tb := b.textBody(&texts[0], nil)
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
