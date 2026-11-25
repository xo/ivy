package drawingml

import (
	"image/color"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
)

// parse parses the XML s, in the namespaces of DrawingML.
func parse(t *testing.T, s string) roxmltree.Node {
	t.Helper()
	doc, err := roxmltree.Parse(`<root xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` + s + `</root>`)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := doc.RootElement().FirstElementChild()
	if !ok {
		t.Fatal("no element")
	}
	return e
}

// testTheme is a theme with the colors and the fonts of Office.
var testTheme = &Theme{
	Colors: map[string]color.NRGBA{
		"dk1": {0, 0, 0, 255}, "lt1": {255, 255, 255, 255},
		"dk2": {0x44, 0x54, 0x6a, 255}, "lt2": {0xe7, 0xe6, 0xe6, 255},
		"accent1": {0x44, 0x72, 0xc4, 255}, "accent2": {0xed, 0x7d, 0x31, 255},
	},
	Major: Fonts{Latin: "Calibri Light"},
	Minor: Fonts{Latin: "Calibri"},
}

func TestColors(t *testing.T) {
	c := &Colors{Theme: testTheme}
	for _, tc := range []struct {
		xml  string
		want color.NRGBA
	}{
		{`<a:srgbClr val="FF0000"/>`, color.NRGBA{255, 0, 0, 255}},
		{`<a:schemeClr val="tx1"/>`, color.NRGBA{0, 0, 0, 255}},
		{`<a:schemeClr val="bg1"/>`, color.NRGBA{255, 255, 255, 255}},
		// the values that PowerPoint shows for accent1 darker 50 percent and
		// lighter 40 percent
		{`<a:schemeClr val="accent1"><a:lumMod val="50000"/></a:schemeClr>`, color.NRGBA{0x20, 0x38, 0x64, 255}},
		{`<a:schemeClr val="accent1"><a:lumMod val="60000"/><a:lumOff val="40000"/></a:schemeClr>`, color.NRGBA{0x8f, 0xaa, 0xdc, 255}},
		{`<a:srgbClr val="0000FF"><a:alpha val="50000"/></a:srgbClr>`, color.NRGBA{0, 0, 255, 128}},
		{`<a:prstClr val="dkBlue"/>`, color.NRGBA{0, 0, 139, 255}},
		{`<a:sysClr val="windowText" lastClr="111111"/>`, color.NRGBA{0x11, 0x11, 0x11, 255}},
		{`<a:hslClr hue="0" sat="100000" lum="50000"/>`, color.NRGBA{255, 0, 0, 255}},
		{`<a:scrgbClr r="100000" g="0" b="0"/>`, color.NRGBA{255, 0, 0, 255}},
		{`<a:srgbClr val="808080"><a:shade val="50000"/></a:srgbClr>`, color.NRGBA{0x5d, 0x5d, 0x5d, 255}},
		{`<a:srgbClr val="FF0000"><a:inv/></a:srgbClr>`, color.NRGBA{0, 255, 255, 255}},
		// the stops of a gradient of SmartArt in the PDF of PowerPoint: the
		// saturation goes past 1 before the channels are limited
		{`<a:srgbClr val="F79646"><a:tint val="50000"/><a:satMod val="300000"/></a:srgbClr>`, color.NRGBA{0xff, 0xbe, 0x86, 255}},
		{`<a:srgbClr val="F79646"><a:tint val="37000"/><a:satMod val="300000"/></a:srgbClr>`, color.NRGBA{0xff, 0xd0, 0xaa, 255}},
		{`<a:srgbClr val="F79646"><a:tint val="15000"/><a:satMod val="350000"/></a:srgbClr>`, color.NRGBA{0xff, 0xeb, 0xdb, 255}},
	} {
		got, ok := c.Of(parse(t, tc.xml))
		if !ok {
			t.Errorf("%s: no color", tc.xml)
			continue
		}
		n := got.NRGBA()
		d := func(a, b uint8) bool { return int(a) > int(b)+2 || int(b) > int(a)+2 }
		if d(n.R, tc.want.R) || d(n.G, tc.want.G) || d(n.B, tc.want.B) || d(n.A, tc.want.A) {
			t.Errorf("%s: expected %v, got %v", tc.xml, tc.want, n)
		}
	}
	// a color map swaps the dark and the light colors
	c = &Colors{Theme: testTheme, Map: map[string]string{"bg1": "dk1", "tx1": "lt1"}}
	if got, _ := c.Of(parse(t, `<a:schemeClr val="tx1"/>`)); got.NRGBA() != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("expected tx1 white with the map, got %v", got.NRGBA())
	}
	// phClr is the color of a style reference
	c = c.WithPlaceholder(RGBA{0, 1, 0, 1})
	if got, _ := c.Of(parse(t, `<a:schemeClr val="phClr"/>`)); got.NRGBA() != (color.NRGBA{0, 255, 0, 255}) {
		t.Errorf("expected phClr green, got %v", got.NRGBA())
	}
}

func TestNumberText(t *testing.T) {
	for _, tc := range []struct {
		scheme string
		n      int
		want   string
	}{
		{"arabicPeriod", 3, "3."},
		{"arabicParenR", 2, "2)"},
		{"alphaLcParenBoth", 3, "(c)"},
		{"alphaUcPeriod", 28, "BB."},
		{"romanUcPeriod", 14, "XIV."},
		{"romanLcParenR", 4, "iv)"},
		{"arabicPlain", 7, "7"},
		{"circleNumDbPlain", 1, "1."},
	} {
		if got := numberText(tc.scheme, tc.n); got != tc.want {
			t.Errorf("%s %d: expected %q, got %q", tc.scheme, tc.n, tc.want, got)
		}
	}
}

func TestBulletChar(t *testing.T) {
	for _, tc := range []struct {
		char, font, want string
		mapped           bool
	}{
		{"•", "Arial", "•", false},
		{"\uf0a7", "Wingdings", "▪", true},
		{"l", "Wingdings", "●", true},
		{"Ø", "Wingdings", "➢", true},
		{"\uf0b7", "Symbol", "•", true},
		{"\uf0ff", "", "•", true},
	} {
		got, mapped := bulletChar(tc.char, tc.font)
		if got != tc.want || mapped != tc.mapped {
			t.Errorf("%q in %s: expected %q %v, got %q %v", tc.char, tc.font, tc.want, tc.mapped, got, mapped)
		}
	}
}

// TestPictureBullet checks the bullet of a picture: an image of the layout,
// two thirds as high as the text, and a round bullet without a picture.
func TestPictureBullet(t *testing.T) {
	run := RunStyle{}
	run.Size.Set(24)
	p := Paragraph{Runs: []Run{{Text: "a", Style: run}}}
	p.Style.Indent.Set(-18)
	p.Style.Bullet.Set(Bullet{Kind: BulletPicture, Picture: []byte("pic")})
	var sb strings.Builder
	writeBullet(&sb, &p, &run, nil, 1)
	if got := sb.String(); !strings.Contains(got, `<img src="data:application/octet-stream;base64,cGlj" style="height:16pt">`) {
		t.Errorf("expected an image of 16 points, got %s", got)
	}
	p.Style.Bullet.Set(Bullet{Kind: BulletPicture})
	sb.Reset()
	writeBullet(&sb, &p, &run, nil, 1)
	if got := sb.String(); !strings.Contains(got, "• </span>") {
		t.Errorf("expected a round bullet, got %s", got)
	}
}

func TestTextBody(t *testing.T) {
	c := &Colors{Theme: testTheme}
	var list ListStyle
	list.Over(c.ParseListStyle(parse(t, `<a:lstStyle><a:lvl1pPr marL="342900" indent="-342900"><a:buChar char="•"/>`+
		`<a:defRPr sz="2400"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl1pPr></a:lstStyle>`)))
	body := parse(t, `<a:txBody><a:bodyPr anchor="ctr" lIns="0"><a:normAutofit fontScale="50000" lnSpcReduction="10000"/></a:bodyPr>`+
		`<a:lstStyle><a:lvl2pPr><a:buAutoNum type="arabicParenR"/></a:lvl2pPr></a:lstStyle>`+
		`<a:p><a:r><a:rPr b="1"/><a:t>Bold &amp; &lt;tag&gt;</a:t></a:r><a:br/><a:fld type="slidenum"><a:t>#</a:t></a:fld></a:p>`+
		`<a:p><a:pPr lvl="1"/><a:r><a:t>one</a:t></a:r></a:p><a:p><a:pPr lvl="1"/><a:r><a:t>two</a:t></a:r></a:p>`+
		`<a:p><a:r><a:rPr><a:latin typeface="+mj-lt"/></a:rPr><a:t>major</a:t></a:r></a:p></a:txBody>`)
	tb := c.ParseTextBody(body, &list, nil, ParseBodyStyle(parse(t, `<a:bodyPr anchor="ctr" lIns="0"><a:normAutofit fontScale="50000" lnSpcReduction="10000"/></a:bodyPr>`)), Fields{SlideNumber: 7})
	if len(tb.Paragraphs) != 4 {
		t.Fatalf("expected 4 paragraphs, got %d", len(tb.Paragraphs))
	}
	if n := tb.Paragraphs[2].Number; n != 2 {
		t.Errorf("expected the number 2, got %d", n)
	}
	box, ok := tb.Box(testTheme, 300, 200)
	if !ok {
		t.Fatal("expected a box")
	}
	for _, want := range []string{
		`vertical-align:middle`,
		`Bold &amp; &lt;tag&gt;`,
		`>7</span>`,
		`2) </span>`,
		`font-weight:bold`,
		// the size of 24 points, at the scale of the autofit
		`font-size:12pt`,
		`&#34;Calibri&#34;, sans-serif`,
		`&#34;Calibri Light&#34;, sans-serif`,
		`min-width:27pt`,
		`line-height:1.08`,
		// a body of DrawingML puts its baselines where PowerPoint does
		`-ivy-line-baseline: win`,
	} {
		if !strings.Contains(box.HTML, want) {
			t.Errorf("expected the document to hold %q:\n%s", want, box.HTML)
		}
	}
	// a body of another format keeps the baselines of CSS
	tb.WinBaseline = false
	if box, _ := tb.Box(testTheme, 300, 200); strings.Contains(box.HTML, "-ivy-line-baseline") {
		t.Errorf("expected no -ivy-line-baseline:\n%s", box.HTML)
	}
	if box.Width != 300-defaultInsetX {
		t.Errorf("expected a page of %g points across, got %g", 300-defaultInsetX, box.Width)
	}
}

func TestSceneText(t *testing.T) {
	c := &Colors{Theme: testTheme}
	body := parse(t, `<a:txBody><a:bodyPr/><a:p><a:r><a:rPr sz="4000"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:rPr><a:t>MMMM</a:t></a:r></a:p></a:txBody>`)
	var list ListStyle
	tb := c.ParseTextBody(body, &list, nil, BodyStyle{}, Fields{})
	s := NewScene(ivy.Config{})
	box, _ := okage.RectFromXYWH(0, 0, 200, 100)
	s.AddFigure(&Figure{
		Transform:     okage.TransformFromTranslate(10, 10),
		TextTransform: okage.TransformFromTranslate(10, 10),
		W:             200,
		H:             100,
		Paths:         []Path{{Path: okage.PathFromRect(box), Stroke: true}},
		Fill:          Solid(RGBA{0, 0, 1, 0.2}),
		Line:          DefaultLine(),
		Text:          tb,
	})
	pix, _ := okage.NewPixmap(220, 120)
	pix.Fill(okage.ColorWhite)
	if err := s.Render(t.Context(), pix, okage.IdentityTransform()); err != nil {
		t.Fatal(err)
	}
	red := 0
	for y := range 120 {
		for x := range 220 {
			p, _ := pix.Pixel(uint32(x), uint32(y))
			if p.Red() > 180 && p.Green() < 80 && p.Blue() < 120 {
				red++
			}
		}
	}
	if red < 50 {
		t.Errorf("expected red text, got %d red pixels", red)
	}
	// a device that records the commands holds the box and the glyphs of
	// the text
	var l display.List
	if err := s.Draw(t.Context(), &l, okage.IdentityTransform()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(l.String(), "fill [0 0 200 100]") {
		t.Errorf("expected the fill of the box, got:\n%s", l.String())
	}
	if !strings.Contains(l.String(), "glyphs 4 ") {
		t.Errorf("expected the glyphs of the text, got:\n%s", l.String())
	}
	// the glyphs replay onto a pixmap as the render draws them
	replay, _ := okage.NewPixmap(220, 120)
	replay.Fill(okage.ColorWhite)
	l.Replay(display.NewOkage(replay, okage.IdentityTransform()))
	if !slices.Equal(replay.Data(), pix.Data()) {
		t.Error("the replay of the commands differs from the render")
	}
}

// TestScaledSize checks that a size at the font scale of an autofit is a
// whole point, as PowerPoint draws it.
func TestScaledSize(t *testing.T) {
	for _, tt := range []struct {
		size, scale, want float64
	}{
		{44, 0.9, 40},
		{46, 0.775, 36},
		{47, 0.775, 36},
		{26, 0.775, 20},
		{10.5, 1, 10.5},
		{1, 0.1, 1},
	} {
		if got := scaledSize(tt.size, tt.scale); got != tt.want {
			t.Errorf("scaledSize(%g, %g) = %g, want %g", tt.size, tt.scale, got, tt.want)
		}
	}
}
