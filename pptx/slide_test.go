package pptx

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fixture"
)

// emu converts points to EMU.
func emu(pt float64) int {
	return int(pt * 12700)
}

// xfrm returns a:xfrm of the box at (x, y) of w by h points.
func xfrm(x, y, w, h float64, extra string) string {
	return fmt.Sprintf(`<a:xfrm%s><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, extra, emu(x), emu(y), emu(w), emu(h))
}

// shape returns p:sp with the properties and the text body.
func shape(id int, spPr, txBody string) string {
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="s%d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>%s</p:spPr>%s</p:sp>`, id, id, spPr, txBody)
}

// render opens the package and renders its slide n at 72 dots per inch,
// over white, so that a pixel is a point.
func render(t *testing.T, buf []byte, n int) image.Image {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// near reports whether the pixel of img at (x, y) is within tol of c in
// each channel.
func near(img image.Image, x, y int, c color.NRGBA, tol int) bool {
	got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	d := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	return d(got.R, c.R) <= tol && d(got.G, c.G) <= tol && d(got.B, c.B) <= tol
}

// expect fails the test if the pixel at (x, y) is not near c.
func expect(t *testing.T, img image.Image, x, y int, c color.NRGBA) {
	t.Helper()
	if !near(img, x, y, c, 12) {
		t.Errorf("pixel (%d, %d): expected %v, got %v", x, y, c, color.NRGBAModel.Convert(img.At(x, y)))
	}
}

// count counts the pixels of the rectangle r of img that are near c.
func count(img image.Image, r image.Rectangle, c color.NRGBA, tol int) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if near(img, x, y, c, tol) {
				n++
			}
		}
	}
	return n
}

var (
	white = color.NRGBA{255, 255, 255, 255}
	red   = color.NRGBA{255, 0, 0, 255}
	green = color.NRGBA{0, 255, 0, 255}
	blue  = color.NRGBA{0, 0, 255, 255}
)

func TestPageCount(t *testing.T) {
	buf := (&fixture.PPTX{Slides: []fixture.PPTXSlide{{}, {Hidden: true}, {}}}).Bytes()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 slides without the hidden one, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 720 || h != 540 {
		t.Errorf("expected 720x540, got: %gx%g", w, h)
	}
	if _, err := d.Page(t.Context(), 2); err == nil {
		t.Error("expected ErrPageRange")
	}
}

func TestShapeFillLine(t *testing.T) {
	sp := shape(2, xfrm(100, 100, 200, 100, "")+
		`<a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>`+
		`<a:ln w="76200"><a:solidFill><a:srgbClr val="0000FF"/></a:solidFill></a:ln>`, "")
	img := render(t, (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: sp}}}).Bytes(), 0)
	expect(t, img, 200, 150, red)
	expect(t, img, 100, 150, blue)
	expect(t, img, 300, 150, blue)
	// outside the ellipse, at the corner of its box
	expect(t, img, 105, 105, white)
}

func TestPicture(t *testing.T) {
	pic := `<p:pic><p:nvPicPr><p:cNvPr id="2" name="pic"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>` +
		`<p:blipFill><a:blip r:embed="rIdM1"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>` +
		`<p:spPr>` + xfrm(300, 300, 100, 80, "") + `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`
	buf := (&fixture.PPTX{
		Slides: []fixture.PPTXSlide{{Shapes: pic}},
		Media:  []fixture.Entry{{Name: "image1.png", Data: fixture.PNG(4, 4, green)}},
	}).Bytes()
	img := render(t, buf, 0)
	expect(t, img, 350, 340, green)
	expect(t, img, 295, 340, white)
}

func TestPlaceholderInherits(t *testing.T) {
	// the body of the slide has no box and no fill: it takes them from the
	// placeholder of the master, through the layout
	body := `<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:cNvSpPr/><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr><p:spPr/>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Text</a:t></a:r></a:p></p:txBody></p:sp>`
	master := `<p:sp><p:nvSpPr><p:cNvPr id="9" name="Bar"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>` + xfrm(0, 520, 720, 20, "") +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="00FF00"/></a:solidFill></p:spPr></p:sp>`
	buf := (&fixture.PPTX{
		MasterShapes: master,
		Slides:       []fixture.PPTXSlide{{Shapes: body}},
	}).Bytes()
	img := render(t, buf, 0)
	// the shape of the master that is not a placeholder draws on the slide
	expect(t, img, 360, 530, green)
	// the text is at the place of the body of the master, at 36, 126, and
	// it is black and large, from the body style of the master
	if n := count(img, image.Rect(36, 126, 300, 200), color.NRGBA{0, 0, 0, 255}, 80); n < 50 {
		t.Errorf("expected the text of the body at the place of the master, got %d dark pixels", n)
	}
	if n := count(img, image.Rect(36, 20, 700, 120), color.NRGBA{0, 0, 0, 255}, 80); n > 0 {
		t.Errorf("expected no text in the title, which the slide does not have, got %d dark pixels", n)
	}
}

func TestTextRunsBullet(t *testing.T) {
	text := `<p:txBody><a:bodyPr wrap="square" lIns="0" tIns="0"/><a:lstStyle/><a:p>` +
		`<a:pPr marL="342900" indent="-342900"><a:buFont typeface="Arial"/><a:buChar char="&#8226;"/></a:pPr>` +
		`<a:r><a:rPr sz="4000"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:rPr><a:t>MMM</a:t></a:r>` +
		`<a:r><a:rPr sz="4000"><a:solidFill><a:srgbClr val="0000FF"/></a:solidFill></a:rPr><a:t>MMM</a:t></a:r></a:p></p:txBody>`
	sp := shape(2, xfrm(50, 50, 500, 100, "")+`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/>`, text)
	img := render(t, (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: sp}}}).Bytes(), 0)
	// the bullet takes the color of the first run, in the hanging indent
	// of 27 points
	if n := count(img, image.Rect(50, 50, 77, 110), red, 60); n < 4 {
		t.Errorf("expected a red bullet in the indent, got %d red pixels", n)
	}
	// the red run, then the blue run
	redBox := image.Rect(77, 50, 170, 110)
	blueBox := image.Rect(170, 50, 300, 110)
	if n := count(img, redBox, red, 60); n < 50 {
		t.Errorf("expected the red run, got %d red pixels", n)
	}
	if n := count(img, blueBox, blue, 60); n < 50 {
		t.Errorf("expected the blue run after the red one, got %d blue pixels", n)
	}
	if n := count(img, redBox, blue, 60); n > 0 {
		t.Errorf("expected no blue before the red run, got %d blue pixels", n)
	}
}

func TestTable(t *testing.T) {
	cell := func(rgb, text string) string {
		return `<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr sz="1000"/><a:t>` + text + `</a:t></a:r></a:p></a:txBody>` +
			`<a:tcPr><a:solidFill><a:srgbClr val="` + rgb + `"/></a:solidFill></a:tcPr></a:tc>`
	}
	frame := `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="2" name="t"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
		`<p:xfrm><a:off x="1270000" y="1270000"/><a:ext cx="2540000" cy="1270000"/></p:xfrm>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr/>` +
		`<a:tblGrid><a:gridCol w="1270000"/><a:gridCol w="1270000"/></a:tblGrid>` +
		`<a:tr h="635000">` + cell("FF0000", "a") + cell("00FF00", "b") + `</a:tr>` +
		`<a:tr h="635000">` + cell("0000FF", "c") + cell("FFFF00", "d") + `</a:tr>` +
		`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`
	img := render(t, (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: frame}}}).Bytes(), 0)
	expect(t, img, 180, 140, red)
	expect(t, img, 280, 140, green)
	expect(t, img, 180, 190, blue)
	expect(t, img, 280, 190, color.NRGBA{255, 255, 0, 255})
}

func TestGroupRotation(t *testing.T) {
	// a group of 200 by 100 at (300, 100), turned by 90 degrees about its
	// center (400, 150). The child fills the left half of the group.
	child := shape(3, xfrm(0, 0, 100, 100, "")+`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="00FF00"/></a:solidFill>`, "")
	group := `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="2" name="g"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr>` +
		fmt.Sprintf(`<a:xfrm rot="5400000"><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/><a:chOff x="0" y="0"/><a:chExt cx="%d" cy="%d"/></a:xfrm>`,
			emu(300), emu(100), emu(200), emu(100), emu(200), emu(100)) +
		`</p:grpSpPr>` + child + `</p:grpSp>`
	img := render(t, (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: group}}}).Bytes(), 0)
	// the left half turns to the top half of the turned box
	expect(t, img, 400, 70, green)
	expect(t, img, 400, 230, white)
	expect(t, img, 320, 150, white)
}

func TestBackground(t *testing.T) {
	bg := `<p:bg><p:bgPr><a:solidFill><a:srgbClr val="00FFFF"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>`
	img := render(t, (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Background: bg}}}).Bytes(), 0)
	expect(t, img, 5, 5, color.NRGBA{0, 255, 255, 255})
	// the background of the master, from the style of the theme
	master := `<p:bg><p:bgRef idx="1001"><a:schemeClr val="accent6"/></p:bgRef></p:bg>`
	img = render(t, (&fixture.PPTX{MasterBackground: master, Slides: []fixture.PPTXSlide{{}}}).Bytes(), 0)
	expect(t, img, 700, 500, color.NRGBA{0x70, 0xad, 0x47, 255})
}

func TestThemeColors(t *testing.T) {
	// accent1 darker by 50 percent, which PowerPoint shows as 203864
	sp := shape(2, xfrm(100, 100, 100, 100, "")+`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`+
		`<a:solidFill><a:schemeClr val="accent1"><a:lumMod val="50000"/></a:schemeClr></a:solidFill>`, "")
	// a shape that takes its fill and its line from the style of the theme
	styled := `<p:sp><p:nvSpPr><p:cNvPr id="3" name="s"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>` + xfrm(300, 100, 100, 100, "") +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:style><a:lnRef idx="2"><a:schemeClr val="accent1"><a:shade val="50000"/></a:schemeClr></a:lnRef>` +
		`<a:fillRef idx="1"><a:schemeClr val="accent2"/></a:fillRef><a:effectRef idx="0"><a:schemeClr val="accent1"/></a:effectRef>` +
		`<a:fontRef idx="minor"><a:schemeClr val="lt1"/></a:fontRef></p:style></p:sp>`
	img := render(t, (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: sp + styled}}}).Bytes(), 0)
	expect(t, img, 150, 150, color.NRGBA{0x20, 0x38, 0x64, 255})
	expect(t, img, 350, 150, color.NRGBA{0xed, 0x7d, 0x31, 255})
}

func TestChartLeftOut(t *testing.T) {
	frame := `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="2" name="c"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
		`<p:xfrm><a:off x="0" y="0"/><a:ext cx="2540000" cy="1270000"/></p:xfrm>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="rId9"/></a:graphicData></a:graphic></p:graphicFrame>`
	img := render(t, (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: frame}}}).Bytes(), 0)
	expect(t, img, 100, 50, white)
}

func TestChart(t *testing.T) {
	frame := `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="2" name="c"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
		xfrm(100, 100, 300, 200, "") +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="rIdC1"/></a:graphicData></a:graphic></p:graphicFrame>`
	frame = strings.Replace(frame, "<a:xfrm>", "<p:xfrm>", 1)
	frame = strings.Replace(frame, "</a:xfrm>", "</p:xfrm>", 1)
	buf := (&fixture.PPTX{
		Slides: []fixture.PPTXSlide{{Shapes: frame}},
		Charts: []fixture.Entry{{Name: "chart1.xml", Data: fixture.Chart("FF0000")}},
	}).Bytes()
	img := render(t, buf, 0)
	// the red columns are in the frame, and nowhere else
	if n := count(img, image.Rect(100, 100, 400, 300), red, 40); n < 1000 {
		t.Errorf("expected the red columns of the chart in its frame, got %d red pixels", n)
	}
	b := img.Bounds()
	if n := count(img, b, red, 40) - count(img, image.Rect(100, 100, 400, 300), red, 40); n > 0 {
		t.Errorf("expected no red outside the frame, got %d red pixels", n)
	}
}

func TestTextDisplayList(t *testing.T) {
	text := `<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr sz="2400"/><a:t>Text</a:t></a:r></a:p></p:txBody>`
	sp := shape(2, xfrm(50, 50, 300, 100, "")+`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`, text)
	d, err := Open(t.Context(), bytes.NewReader((&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: sp}}}).Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	// a device that records the commands holds the glyphs of the four
	// letters of the text
	var l display.List
	if err := p.(*page).scene.Draw(t.Context(), &l, okage.IdentityTransform()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(l.String(), "glyphs 4 ") {
		t.Errorf("expected the glyphs of the text, got:\n%s", l.String())
	}
}

// TestSlideTextStyle checks that a text box of a slide takes the default
// text style of the presentation, without the other style of the master,
// as PowerPoint draws it, and that a text box of the master takes the
// other style of the master.
func TestSlideTextStyle(t *testing.T) {
	text := `<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Text</a:t></a:r></a:p></p:txBody>`
	geom := `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`
	box := strings.Replace(shape(2, xfrm(50, 50, 300, 100, "")+geom, text), "<p:cNvSpPr/>", `<p:cNvSpPr txBox="1"/>`, 1)
	master := strings.Replace(shape(9, xfrm(50, 300, 300, 100, "")+geom, strings.Replace(text, "Text", "Master", 1)), "<p:cNvSpPr/>", `<p:cNvSpPr txBox="1"/>`, 1)
	f := &fixture.PPTX{
		DefaultTextStyle: `<a:lvl1pPr><a:defRPr sz="1200"><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl1pPr>`,
		MasterShapes:     master,
		Slides:           []fixture.PPTXSlide{{Shapes: box}},
	}
	d, err := Open(t.Context(), bytes.NewReader(f.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	var l display.List
	if err := p.(*page).scene.Draw(t.Context(), &l, okage.IdentityTransform()); err != nil {
		t.Fatal(err)
	}
	// the four letters of Text at 12 points, 16 pixels, and the six
	// letters of Master at the 18 points of the other style, 24 pixels
	for _, want := range []string{"glyphs 4 16", "glyphs 6 24"} {
		if !strings.Contains(l.String(), want) {
			t.Errorf("expected %q, got:\n%s", want, l.String())
		}
	}
}

func TestDescribe(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader((&fixture.PPTX{}).Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if info, err := d.Describe(t.Context()); err != nil || info.Title != "" {
		t.Errorf("expected no information and no error, got: %+v, %v", info, err)
	}
}

// inkWidth returns the width of the columns of img that hold a pixel darker
// than mid gray.
func inkWidth(img image.Image) int {
	b := img.Bounds()
	lo, hi := b.Max.X, b.Min.X
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			if c.Y < 128 {
				lo, hi = min(lo, x), max(hi, x)
			}
		}
	}
	return hi - lo
}

func TestConfigFonts(t *testing.T) {
	dir := filepath.Join("..", "fonts")
	if _, err := os.Stat(filepath.Join(dir, "carlito")); err != nil {
		t.Skip("the fonts of github.com/xo/ivy/fonts are missing:", err)
	}
	// the text names Calibri through the minor font of the theme
	text := `<p:txBody><a:bodyPr wrap="none"/><a:lstStyle/><a:p><a:r><a:rPr sz="3600"/><a:t>Calibri text</a:t></a:r></a:p></p:txBody>`
	buf := (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: shape(2, xfrm(20, 20, 600, 100, "")+`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`, text)}}}).Bytes()
	widths := make([]int, 2)
	for i, cfg := range []ivy.Config{{}, {Fonts: os.DirFS(dir)}} {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			t.Fatal(err)
		}
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			t.Fatal(err)
		}
		img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
		if err != nil {
			t.Fatal(err)
		}
		widths[i] = inkWidth(img)
	}
	// Carlito, which has the metrics of Calibri, is narrower than the Go
	// font
	if widths[1] <= 0 || widths[1] >= widths[0] {
		t.Errorf("expected the text in Carlito narrower than in Go, got %d and %d points", widths[1], widths[0])
	}
}
