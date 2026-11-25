package drawingml

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// TestRecolorTransfer checks the change of a channel by the contrast and the
// brightness. The washout of Word, a factor of contrast of 0.3 and a
// brightness of 0.7, gives 0.3 x + 0.805, as the export of
// floating-pictures.doc to PDF by Word shows. Three values fall half way
// between two levels: 64 with the contrast 2, 100 with the brightness 0.5
// and 255 with the brightness -0.5. The arithmetic of float32 in the
// transfer function of okage decides which level each one takes.
func TestRecolorTransfer(t *testing.T) {
	for _, c := range []struct {
		name string
		r    Recolor
		in   []int
		want []int
	}{
		{"washout", Recolor{Bright: 0.7, Contrast: ContrastOf(0.3)}, []int{0, 128, 255}, []int{205, 244, 255}},
		{"contrast 2", Recolor{Contrast: ContrastOf(2)}, []int{0, 64, 128, 192, 255}, []int{0, 1, 129, 255, 255}},
		{"contrast 0", Recolor{Contrast: ContrastOf(0)}, []int{0, 255}, []int{128, 128}},
		{"brightness 0.5", Recolor{Bright: 0.5}, []int{0, 100, 200}, []int{128, 228, 255}},
		{"darker", Recolor{Bright: -0.5}, []int{0, 200, 255}, []int{0, 73, 127}},
	} {
		t.Run(c.name, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, len(c.in), 1))
			for i, v := range c.in {
				img.SetNRGBA(i, 0, color.NRGBA{uint8(v), uint8(v), uint8(v), 255})
			}
			out := c.r.Apply(img)
			for i, v := range c.in {
				want := uint8(c.want[i])
				if got := out.NRGBAAt(i, 0); got != (color.NRGBA{want, want, want, 255}) {
					t.Errorf("expected %d for %d, got %v", want, v, got)
				}
			}
		})
	}
	if _, ok := (Recolor{}).transfer(); ok {
		t.Error("expected no transfer function for the zero change")
	}
}

// TestContrastOf checks the contrast of a:lum of a factor of Office
// Drawing, and the factor that it gives back.
func TestContrastOf(t *testing.T) {
	for _, k := range []float64{0, 0.3, 1, 2, 4} {
		c := ContrastOf(k)
		if c < -1 || c > 1 {
			t.Errorf("contrast of %g out of range: %g", k, c)
		}
		if got := (Recolor{Contrast: c}).factor(); math.Abs(got-k) > 1e-9 {
			t.Errorf("expected the factor %g back, got %g", k, got)
		}
	}
	if c := ContrastOf(math.NaN()); c != -1 {
		t.Errorf("expected -1 for NaN, got %g", c)
	}
}

// TestRecolorPixels checks the gray, the black and white and the duotone of
// the pixels of a picture, and that the alpha stays.
func TestRecolorPixels(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	img.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	img.SetNRGBA(1, 0, color.NRGBA{0, 255, 0, 128})
	img.SetNRGBA(2, 0, color.NRGBA{0, 0, 255, 255})
	for _, c := range []struct {
		name string
		r    Recolor
		want [3]color.NRGBA
	}{
		{"gray", Recolor{Gray: true}, [3]color.NRGBA{{76, 76, 76, 255}, {150, 150, 150, 128}, {29, 29, 29, 255}}},
		{"black and white", Recolor{BiLevel: true, Threshold: 0.5}, [3]color.NRGBA{{0, 0, 0, 255}, {255, 255, 255, 128}, {0, 0, 0, 255}}},
		{"duotone", Recolor{Duotone: [2]RGBA{Black, {1, 0, 0, 1}}, HasDuotone: true}, [3]color.NRGBA{{76, 0, 0, 255}, {150, 0, 0, 128}, {29, 0, 0, 255}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := c.r.Apply(img)
			for x := range 3 {
				if got := out.NRGBAAt(x, 0); got != c.want[x] {
					t.Errorf("pixel %d: expected %v, got %v", x, c.want[x], got)
				}
			}
			if img.NRGBAAt(0, 0) != (color.NRGBA{255, 0, 0, 255}) {
				t.Error("expected the picture to stay")
			}
		})
	}
}

// TestPictureRecolor checks that a picture changes its colors once for each
// change, and that a metafile draws in pixels with its colors changed.
func TestPictureRecolor(t *testing.T) {
	base := NewPicture(fixture.PNG(2, 2, color.NRGBA{255, 0, 0, 255}), ivy.Config{})
	r := Recolor{Gray: true}
	p := base.Recolor(r)
	if base.Recolor(r) != p || base.Recolor(Recolor{}) != base {
		t.Error("expected the same picture for the same change")
	}
	pix := renderPaint(t, Paint{Kind: PaintPicture, Picture: p, Opacity: 1}, 10, 10)
	if px, _ := pix.Pixel(5, 5); px.Red() != px.Green() || px.Red() < 60 || px.Red() > 90 {
		t.Errorf("expected gray, got %v", px)
	}
}

// TestParseEffects checks the effects of a:effectLst.
func TestParseEffects(t *testing.T) {
	n := parse(t, `<a:effectLst>`+
		`<a:glow rad="127000"><a:srgbClr val="FF0000"><a:alpha val="40000"/></a:srgbClr></a:glow>`+
		`<a:innerShdw blurRad="63500" dist="50800" dir="5400000"><a:srgbClr val="000000"/></a:innerShdw>`+
		`<a:outerShdw blurRad="38100" dist="25400" dir="0" sx="90000" sy="80000"><a:srgbClr val="00FF00"/></a:outerShdw>`+
		`<a:reflection blurRad="6350" stA="50000" endA="300" endPos="55000" dist="12700" dir="5400000" sy="-100000" algn="bl" rotWithShape="0"/>`+
		`<a:softEdge rad="63500"/>`+
		`<a:blur rad="12700"/>`+
		`</a:effectLst>`)
	fx, ok := (&Colors{}).ParseEffects(n)
	if !ok {
		t.Fatal("expected a list of effects")
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-6 }
	if g := fx.Glow; g == nil || !near(g.Rad, 10) || g.Color.R != 1 || !near(g.Color.A, 0.4) {
		t.Errorf("glow: %+v", fx.Glow)
	}
	if s := fx.InnerShadow; s == nil || !near(s.Blur, 5) || !near(s.DY, 4) || !near(s.DX, 0) {
		t.Errorf("inner shadow: %+v", fx.InnerShadow)
	}
	if s := fx.Shadow; s == nil || !near(s.DX, 2) || !near(s.SX, 0.9) || !near(s.SY, 0.8) || s.Color.G != 1 {
		t.Errorf("outer shadow: %+v", fx.Shadow)
	}
	if r := fx.Reflection; r == nil || !near(r.StartA, 0.5) || !near(r.EndA, 0.003) || !near(r.EndPos, 0.55) ||
		!near(r.DY, 1) || !near(r.SY, -1) || r.Align != "bl" || !near(r.FadeDir, 90) {
		t.Errorf("reflection: %+v", fx.Reflection)
	}
	if !near(fx.SoftEdge, 5) {
		t.Errorf("soft edge: %g", fx.SoftEdge)
	}
	if got := LeftOut(n); len(got) != 1 || got[0] != "blur" {
		t.Errorf("expected the blur left out, got %v", got)
	}
}

// effectScene returns a scene with a square of 40 points at (40, 40) of
// the fill col and the effects fx.
func effectScene(col RGBA, fx Effects) *Scene {
	s := NewScene(ivy.Config{})
	box, _ := okage.RectFromXYWH(0, 0, 40, 40)
	f := &Figure{
		Transform: okage.TransformFromTranslate(40, 40), W: 40, H: 40,
		Paths: []Path{{Path: okage.PathFromRect(box)}},
		Fill:  Solid(col),
		Line:  Line{Paint: Paint{Kind: PaintNone}},
	}
	f.SetEffects(fx)
	s.AddFigure(f)
	return s
}

// renderGray renders the scene on white, 120 points square, and returns the
// red channel of the pixel at x, y.
func renderGray(t *testing.T, s *Scene) func(x, y uint32) int {
	t.Helper()
	pix, _ := okage.NewPixmap(120, 120)
	pix.Fill(okage.ColorWhite)
	if err := s.Render(t.Context(), pix, okage.IdentityTransform()); err != nil {
		t.Fatal(err)
	}
	return func(x, y uint32) int { return red(pix, x, y) }
}

// TestGlow checks the glow of a square: full near the outline, half at
// half the radius, and gone past the radius.
func TestGlow(t *testing.T) {
	at := renderGray(t, effectScene(White, Effects{Glow: &Glow{Color: Black, Rad: 12}}))
	if g := at(38, 60); g > 40 {
		t.Errorf("near the outline: %d, expected dark", g)
	}
	if g := at(34, 60); g < 80 || g > 180 {
		t.Errorf("at half the radius: %d, expected gray", g)
	}
	if g := at(26, 60); g < 245 {
		t.Errorf("past the radius: %d, expected white", g)
	}
	if g := at(60, 60); g != 255 {
		t.Errorf("inside: %d, expected the white fill", g)
	}
}

// TestSoftEdge checks the soft edge of a black square: empty at the
// outline, and full past the radius inside it.
func TestSoftEdge(t *testing.T) {
	at := renderGray(t, effectScene(Black, Effects{SoftEdge: 10}))
	if g := at(41, 60); g < 240 {
		t.Errorf("at the outline: %d, expected white", g)
	}
	if g := at(48, 60); g < 60 || g > 200 {
		t.Errorf("at 0.8 times the radius: %d, expected gray", g)
	}
	if g := at(60, 60); g > 5 {
		t.Errorf("inside: %d, expected black", g)
	}
}

// TestInnerShadow checks that the inner shadow lies along the edges that
// its offset points to, inside the shape.
func TestInnerShadow(t *testing.T) {
	at := renderGray(t, effectScene(White, Effects{InnerShadow: &Shadow{Color: Black, DX: 6, DY: 6}}))
	if g := at(77, 60); g > 10 {
		t.Errorf("at the right edge: %d, expected the shadow", g)
	}
	if g := at(42, 60); g != 255 {
		t.Errorf("at the left edge: %d, expected the fill", g)
	}
	if g := at(83, 60); g != 255 {
		t.Errorf("outside: %d, expected white", g)
	}
}

// TestReflection checks a reflection below a black square, which fades
// from half its alpha to nothing.
func TestReflection(t *testing.T) {
	at := renderGray(t, effectScene(Black, Effects{Reflection: &Reflection{
		StartA: 0.5, EndA: 0, EndPos: 1, FadeDir: 90, SX: 1, SY: -1, Align: "b",
	}}))
	if g := at(60, 82); g < 110 || g > 145 {
		t.Errorf("at the top of the reflection: %d, expected half", g)
	}
	if g := at(60, 100); g < 170 || g > 210 {
		t.Errorf("in the middle of the reflection: %d, expected a quarter", g)
	}
	if g := at(60, 119); g < 250 {
		t.Errorf("at the end of the reflection: %d, expected white", g)
	}
	if g := at(60, 60); g != 0 {
		t.Errorf("the shape: %d, expected black", g)
	}
}

// TestCompoundSide checks the side of the thick line of thickThin and
// thinThick, as PowerPoint 2024 draws them in effects.pptx: on the left of
// the direction of the path, which is the top of a line from left to right
// and the outside of a rectangle, for thickThin.
func TestCompoundSide(t *testing.T) {
	for _, c := range []struct {
		compound   string
		thickAbove bool
	}{
		{"thickThin", true},
		{"thinThick", false},
	} {
		t.Run(c.compound, func(t *testing.T) {
			s := NewScene(ivy.Config{})
			pb := okage.NewPathBuilder()
			pb.MoveTo(0, 0)
			pb.LineTo(80, 0)
			line, _ := pb.Finish()
			l := Line{Paint: Solid(Black), Width: 20, Compound: c.compound, Cap: okage.LineCapButt, Join: okage.LineJoinMiter}
			s.AddFigure(&Figure{
				Transform: okage.TransformFromTranslate(20, 30), W: 80, H: 0,
				Paths: []Path{{Path: line, Fill: FillNone, Stroke: true}},
				Fill:  Paint{Kind: PaintNone}, Line: l,
			})
			box, _ := okage.RectFromXYWH(0, 0, 60, 30)
			s.AddFigure(&Figure{
				Transform: okage.TransformFromTranslate(30, 70), W: 60, H: 30,
				Paths: []Path{{Path: okage.PathFromRect(box), Fill: FillNone, Stroke: true}},
				Fill:  Paint{Kind: PaintNone}, Line: l,
			})
			at := renderGray(t, s)
			// the line spans y = 20 to 40: the thick line takes 12 points
			// on one side, then a gap of 4 and a thin line of 4. The top
			// edge of the rectangle spans y = 60 to 80, with its outside
			// above
			gapLine, gapRect := 34, uint32(74)
			if !c.thickAbove {
				gapLine, gapRect = 26, 66
			}
			for _, p := range [][2]uint32{{60, 22}, {60, 38}, {60, 62}, {60, 78}} {
				if g := at(p[0], p[1]); g > 20 {
					t.Errorf("at %v: %d, expected a line", p, g)
				}
			}
			if g := at(60, uint32(gapLine)); g < 235 {
				t.Errorf("gap of the line: %d, expected white", g)
			}
			if g := at(60, gapRect); g < 235 {
				t.Errorf("gap of the rectangle: %d, expected white", g)
			}
		})
	}
}
