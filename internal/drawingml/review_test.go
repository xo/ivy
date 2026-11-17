package drawingml

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// The tests of this file come from the faults that the visual review found
// in the drawings of Office.

func TestPictureResolution(t *testing.T) {
	blue := color.NRGBA{0, 0, 255, 255}
	for _, c := range []struct {
		name string
		data []byte
		dpi  float64
	}{
		{"jpeg of 300 dpi", fixture.JPEGDPI(fixture.JPEG(4, 4, blue), 300), 300},
		{"png of 144 dpi", fixture.PNGDPI(fixture.PNG(4, 4, blue), 144), 144},
		{"png without a resolution", fixture.PNG(4, 4, blue), 96},
		{"not a picture", []byte("text"), 96},
	} {
		t.Run(c.name, func(t *testing.T) {
			x, y := NewPicture(c.data, ivy.Config{}).Resolution()
			if x < c.dpi-0.5 || x > c.dpi+0.5 || y < c.dpi-0.5 || y > c.dpi+0.5 {
				t.Errorf("expected %g dots per inch, got %g by %g", c.dpi, x, y)
			}
		})
	}
}

// TestTileResolution checks that a picture that repeats takes the size of
// its pixels at its resolution: 64 pixels at 144 dots per inch are 32
// points.
func TestTileResolution(t *testing.T) {
	img := fixture.Image(64, 64, color.NRGBA{255, 0, 0, 255})
	for x := 32; x < 64; x++ {
		for y := range 64 {
			img.Set(x, y, color.NRGBA{0, 0, 255, 255})
		}
	}
	data := fixture.PNGDPI(encodePNG(t, img), 144)
	pt := Paint{Kind: PaintPicture, Picture: NewPicture(data, ivy.Config{}), Opacity: 1,
		Tile: &Tile{SX: 1, SY: 1, Flip: "none", Align: "tl"}}
	s := NewScene(ivy.Config{})
	box, _ := okage.RectFromXYWH(0, 0, 64, 64)
	s.AddFigure(&Figure{
		Transform: okage.IdentityTransform(), W: 64, H: 64,
		Paths: []Path{{Path: okage.PathFromRect(box)}},
		Fill:  pt, Line: Line{Paint: Paint{Kind: PaintNone}},
	})
	pix, _ := okage.NewPixmap(64, 64)
	pix.Fill(okage.ColorWhite)
	if err := s.Render(t.Context(), pix, okage.IdentityTransform()); err != nil {
		t.Fatal(err)
	}
	// the tile is 32 points across: red from 0 to 16, blue from 16 to 32,
	// and red again from 32
	for _, c := range []struct {
		x    uint32
		blue bool
	}{{8, false}, {24, true}, {40, false}, {56, true}} {
		p, _ := pix.Pixel(c.x, 8)
		if (p.Blue() > 128) != c.blue {
			t.Errorf("pixel %d: expected blue %v, got %v", c.x, c.blue, p)
		}
	}
}

func TestGroupSpace(t *testing.T) {
	// the children of a box of 10 by 20 map onto a group of 20 by 60
	s := space{m: okage.IdentityTransform(), sx: 2, sy: 3, ox: 100, oy: 200}
	x := s.place(Xfrm{X: 105, Y: 210, W: 5, H: 10, OK: true})
	if x.X != 10 || x.Y != 30 || x.W != 10 || x.H != 30 {
		t.Errorf("expected the box (10, 30) of 10 by 30, got (%g, %g) of %g by %g", x.X, x.Y, x.W, x.H)
	}
	// the identity space leaves the box as it is
	if got := rootSpace(okage.IdentityTransform()).place(Xfrm{X: 1, Y: 2, W: 3, H: 4}); got.X != 1 || got.W != 3 {
		t.Errorf("expected the same box, got %+v", got)
	}
}

// TestBoxOverflow checks that the text of a box anchored at its middle
// overflows the box by the same amount at its top and at its bottom, and
// the text anchored at the bottom overflows at the top.
func TestBoxOverflow(t *testing.T) {
	c := &Colors{Theme: testTheme}
	var list ListStyle
	for _, a := range []struct {
		anchor string
		up     bool
	}{{"t", false}, {"ctr", true}, {"b", true}} {
		body := parse(t, `<a:txBody><a:bodyPr anchor="`+a.anchor+`"/><a:p><a:r><a:t>text</a:t></a:r></a:p></a:txBody>`)
		tb := c.ParseTextBody(body, &list, nil, ParseBodyStyle(parse(t, `<a:bodyPr anchor="`+a.anchor+`"/>`)), Fields{})
		box, ok := tb.Box(testTheme, 200, 40)
		if !ok {
			t.Fatal("expected a box")
		}
		// the page of the text starts above the box when the text can
		// overflow upward
		if up := box.Transform.TY < 0; up != a.up {
			t.Errorf("anchor %s: expected the page above the box %v, got the offset %g", a.anchor, a.up, box.Transform.TY)
		}
		if a.anchor == "ctr" && !strings.Contains(box.HTML, `vertical-align:middle`) {
			t.Errorf("anchor ctr: expected the middle alignment:\n%s", box.HTML)
		}
	}
}

// encodePNG returns img as a PNG file.
func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestTextSpacing checks the spacing of the lines and the paragraphs of a
// text body, as PowerPoint draws backgrounds.pptx: a line of single
// spacing is 1.2 times the size of the text, and a space before of 20
// percent is 20 percent of that line. A break at the end of a paragraph
// starts an empty line, as in the titles of custom-geometry.ppt.
func TestTextSpacing(t *testing.T) {
	c := &Colors{Theme: testTheme}
	body := parse(t, `<a:txBody><a:bodyPr/>`+
		`<a:p><a:pPr><a:spcBef><a:spcPct val="20000"/></a:spcBef></a:pPr><a:r><a:rPr sz="3200"/><a:t>one</a:t></a:r></a:p>`+
		`<a:p><a:pPr><a:spcBef><a:spcPct val="20000"/></a:spcBef></a:pPr><a:r><a:rPr sz="3200"/><a:t>two</a:t></a:r><a:br/></a:p>`+
		`</a:txBody>`)
	var list ListStyle
	tb := c.ParseTextBody(body, &list, nil, BodyStyle{}, Fields{})
	box, ok := tb.Box(testTheme, 300, 200)
	if !ok {
		t.Fatal("expected a text box")
	}
	for _, want := range []string{
		`line-height:1.2;`,
		`margin-top:7.68pt;`,
		`two</span><br><br></p>`,
	} {
		if !strings.Contains(box.HTML, want) {
			t.Errorf("expected the document to hold %q:\n%s", want, box.HTML)
		}
	}
	// the first paragraph has no space before
	if n := strings.Count(box.HTML, "margin-top"); n != 1 {
		t.Errorf("expected one space before a paragraph, got %d", n)
	}
}

// TestTableVertical checks the text of a table cell that goes up, with
// vert="vert270", as in table-test2.pptx: the text turns in a box out of
// the flow, with lines as long as the rows of the cell, which grow to hold
// the text of their other cells.
func TestTableVertical(t *testing.T) {
	c := &Colors{Theme: testTheme}
	var list ListStyle
	text := func(s string) *TextBody {
		return c.ParseTextBody(parse(t, `<a:txBody><a:bodyPr/><a:p><a:r><a:rPr sz="1000"/><a:t>`+s+`</a:t></a:r></a:p></a:txBody>`),
			&list, nil, BodyStyle{}, Fields{})
	}
	insets := [4]float64{7.2, 3.6, 7.2, 3.6}
	tab := &Table{
		Cols: []float64{50, 100},
		Rows: []TableRow{
			{Height: 20, Cells: []TableCell{
				{ColSpan: 1, RowSpan: 2, Insets: insets, Anchor: "t", Vert: "vert270", Text: text("up")},
				{ColSpan: 1, RowSpan: 1, Insets: insets, Anchor: "t", Text: text("a")},
			}},
			{Height: 40, Cells: []TableCell{
				{ColSpan: 1, RowSpan: 1, Insets: insets, Anchor: "t", Text: c.ParseTextBody(parse(t,
					`<a:txBody><a:bodyPr/><a:p><a:r><a:rPr sz="1000"/><a:t>b</a:t></a:r><a:br/><a:r><a:rPr sz="1000"/><a:t>c</a:t></a:r></a:p></a:txBody>`),
					&list, nil, BodyStyle{}, Fields{})},
			}},
		},
	}
	doc, _, _ := tab.html(150, 40, testTheme)
	// the lines of the text that goes up are as long as the two rows
	// without the insets of the cell, 20 + 40 - 7.2 points, and the box is
	// as wide as the column without its insets
	for _, want := range []string{
		`transform:translate(0,52.8pt) rotate(-90deg)`,
		`width:52.8pt;`,
		`<tr style="height:35.59`,
		`up</span>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected the document to hold %q:\n%s", want, doc)
		}
	}
	// the row where the cell ends gives its height to its other cell, as
	// the layout gives the row the height of the whole cell
	if !strings.Contains(doc, `<tr><td style="vertical-align:top;padding:3.6pt 7.2pt 3.6pt 7.2pt;height:32.8pt`) {
		t.Errorf("expected the height of the row on its cell:\n%s", doc)
	}
}

// TestPictureDuotone checks the colors of a:duotone: the luminance of a
// pixel takes it from the first color to the second.
func TestPictureDuotone(t *testing.T) {
	black, red := RGBA{0, 0, 0, 1}, RGBA{1, 0, 0, 1}
	for _, c := range []struct {
		in   color.NRGBA
		want color.NRGBA
	}{
		{color.NRGBA{255, 255, 255, 255}, color.NRGBA{255, 0, 0, 255}},
		{color.NRGBA{0, 0, 0, 255}, color.NRGBA{0, 0, 0, 255}},
		{color.NRGBA{255, 255, 255, 128}, color.NRGBA{255, 0, 0, 128}},
	} {
		base := NewPicture(fixture.PNG(2, 2, c.in), ivy.Config{})
		p := base.Duotone(black, red)
		if base.Duotone(black, red) != p {
			t.Error("expected the picture of the same colors once")
		}
		if err := p.decode(t.Context()); err != nil {
			t.Fatal(err)
		}
		if got := color.NRGBAModel.Convert(p.img.At(0, 0)).(color.NRGBA); got != c.want {
			t.Errorf("expected %v for %v, got %v", c.want, c.in, got)
		}
	}
}

// TestTextLink checks the text of a hyperlink, as PowerPoint draws the
// links of custom-geometry.pptx: the color hlink of the theme and an
// underline, when the run gives no color. The bullet keeps the color of
// the paragraph, the space after the last paragraph does not move the
// text, and the text does not kern.
func TestTextLink(t *testing.T) {
	theme := &Theme{Colors: map[string]color.NRGBA{"dk1": {0, 0, 0, 255}, "hlink": {0x05, 0x63, 0xc1, 255}}, Minor: Fonts{Latin: "Calibri"}}
	c := &Colors{Theme: theme}
	body := parse(t, `<a:txBody><a:bodyPr anchor="ctr"/><a:p><a:pPr><a:spcAft><a:spcPts val="1200"/></a:spcAft><a:buChar char="•"/></a:pPr>`+
		`<a:r><a:rPr><a:hlinkClick r:id="rId1"/></a:rPr><a:t>link</a:t></a:r>`+
		`<a:r><a:rPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill><a:hlinkClick r:id="rId1"/></a:rPr><a:t>red</a:t></a:r></a:p></a:txBody>`)
	var list ListStyle
	list[1].Run.Paint.Set(Solid(Black))
	tb := c.ParseTextBody(body, &list, nil, BodyStyle{}, Fields{})
	runs := tb.Paragraphs[0].Runs
	if got := runs[0].Style.Paint.V.Color; got != RGBAFrom(color.NRGBA{0x05, 0x63, 0xc1, 255}) || !runs[0].Style.Underline.V {
		t.Errorf("expected an underlined link of the color hlink, got %+v", runs[0].Style)
	}
	if got := runs[1].Style.Paint.V.Color; got != RGBAFrom(color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("expected a red link, got %v", got)
	}
	box, ok := tb.Box(theme, 300, 200)
	if !ok {
		t.Fatal("expected a text box")
	}
	for _, want := range []string{
		`font-kerning: none`,
		`color:rgba(0,0,0,1.000)">• </span>`,
	} {
		if !strings.Contains(box.HTML, want) {
			t.Errorf("expected the document to hold %q:\n%s", want, box.HTML)
		}
	}
	if strings.Contains(box.HTML, "margin-bottom") {
		t.Errorf("expected no space after the last paragraph:\n%s", box.HTML)
	}
}

// TestTextXfrm checks that the text of a shape of a drawing of SmartArt
// goes into the box of its dsp:txXfrm, as PowerPoint draws the diagrams of
// custom-geometry.pptx.
func TestTextXfrm(t *testing.T) {
	tree := parse(t, `<dsp:spTree xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram"><dsp:sp><dsp:spPr>`+
		`<a:xfrm><a:off x="0" y="0"/><a:ext cx="2540000" cy="1270000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></dsp:spPr>`+
		`<dsp:txBody><a:bodyPr lIns="0" tIns="0" rIns="0" bIns="0" anchor="ctr"/><a:p><a:pPr algn="ctr"/><a:r><a:rPr sz="2000"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:rPr><a:t>MM</a:t></a:r></a:p></dsp:txBody>`+
		`<dsp:txXfrm><a:off x="1270000" y="0"/><a:ext cx="1270000" cy="1270000"/></dsp:txXfrm></dsp:sp></dsp:spTree>`)
	s := NewScene(ivy.Config{})
	tr := &Tree{Ctx: t.Context(), Scene: s, Colors: &Colors{Theme: testTheme}}
	tr.Draw(tree, okage.IdentityTransform())
	pix, _ := okage.NewPixmap(200, 100)
	pix.Fill(okage.ColorWhite)
	if err := s.Render(t.Context(), pix, okage.IdentityTransform()); err != nil {
		t.Fatal(err)
	}
	left, right := 0, 0
	for y := range 100 {
		for x := range 200 {
			p, _ := pix.Pixel(uint32(x), uint32(y))
			if p.Red() > 180 && p.Green() < 80 {
				if x < 100 {
					left++
				} else {
					right++
				}
			}
		}
	}
	if left > 0 || right < 20 {
		t.Errorf("expected the text in the right half, got %d red pixels on the left and %d on the right", left, right)
	}
}

// TestEffects checks the blur of a shadow and a compound line.
func TestEffects(t *testing.T) {
	s := NewScene(ivy.Config{})
	box, _ := okage.RectFromXYWH(0, 0, 40, 40)
	s.AddFigure(&Figure{
		Transform: okage.TransformFromTranslate(20, 20), W: 40, H: 40,
		Paths:  []Path{{Path: okage.PathFromRect(box)}},
		Fill:   Solid(White),
		Line:   Line{Paint: Paint{Kind: PaintNone}},
		Shadow: &Shadow{Color: Black, DX: 40, Blur: 8, SX: 1, SY: 1},
	})
	line, _ := okage.RectFromXYWH(0, 0, 60, 20)
	s.AddFigure(&Figure{
		Transform: okage.TransformFromTranslate(20, 120), W: 60, H: 20,
		Paths: []Path{{Path: okage.PathFromRect(line), Fill: FillNone, Stroke: true}},
		Fill:  Paint{Kind: PaintNone},
		Line:  Line{Paint: Solid(Black), Width: 9, Compound: "dbl", Cap: okage.LineCapButt, Join: okage.LineJoinMiter},
	})
	pix, _ := okage.NewPixmap(160, 160)
	pix.Fill(okage.ColorWhite)
	if err := s.Render(t.Context(), pix, okage.IdentityTransform()); err != nil {
		t.Fatal(err)
	}
	gray := func(x, y uint32) uint8 {
		p, _ := pix.Pixel(x, y)
		return p.Red()
	}
	// the middle of the shadow is dark, and its edge is between dark and
	// white over the radius of the blur
	if g := gray(80, 40); g > 20 {
		t.Errorf("middle of the shadow: %d, expected dark", g)
	}
	if g := gray(100, 40); g < 60 || g > 200 {
		t.Errorf("edge of the shadow: %d, expected gray", g)
	}
	if g := gray(112, 40); g < 240 {
		t.Errorf("past the blur: %d, expected white", g)
	}
	// the double line of 9 points: a line of 3, a gap of 3 and a line of 3
	// across the top edge at y = 120
	if a, b, c := gray(50, 117), gray(50, 120), gray(50, 123); a > 60 || b < 200 || c > 60 {
		t.Errorf("double line: %d, %d, %d, expected dark, white, dark", a, b, c)
	}
}
