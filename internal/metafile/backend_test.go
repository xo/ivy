package metafile

import (
	"image"
	"image/color"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/gowemf"
)

// testBackend returns a backend that draws onto a white pixmap of 100 by
// 100 pixels.
func testBackend(t *testing.T) (*backend, *okage.Pixmap) {
	t.Helper()
	pm, ok := okage.NewPixmap(100, 100)
	if !ok {
		t.Fatal("no pixmap")
	}
	pm.Fill(okage.ColorWhite)
	dst, _ := okage.RectFromXYWH(0, 0, 100, 100)
	pic := &Picture{lim: ivy.Limits{}.WithDefaults(), log: ivy.Config{}.Log()}
	return newBackend(t.Context(), display.NewOkage(pm, okage.IdentityTransform()), dst, pic), pm
}

// line returns the path of a line from (x0, y0) to (x1, y1).
func line(x0, y0, x1, y1 float64) gowemf.Path {
	return gowemf.Path{
		Verbs:  []gowemf.PathVerb{gowemf.PathMoveTo, gowemf.PathLineTo},
		Points: []gowemf.Point{{X: x0, Y: y0}, {X: x1, Y: y1}},
	}
}

// TestEndCap checks a pen of EMF+ whose start cap and end cap differ.
func TestEndCap(t *testing.T) {
	white := color.NRGBA{255, 255, 255, 255}
	for _, test := range []struct {
		name       string
		start, end gowemf.LineCap
		want       map[[2]int]color.NRGBA
	}{
		{"flat and round", gowemf.CapFlat, gowemf.CapRound, map[[2]int]color.NRGBA{
			{50, 50}: black, {15, 50}: white, {86, 50}: black, {88, 58}: white,
		}},
		{"square and round", gowemf.CapSquare, gowemf.CapRound, map[[2]int]color.NRGBA{
			{15, 50}: black, {12, 42}: black, {86, 50}: black, {88, 58}: white,
		}},
		{"round and flat", gowemf.CapRound, gowemf.CapFlat, map[[2]int]color.NRGBA{
			{14, 50}: black, {12, 58}: white, {85, 50}: white,
		}},
		{"a reversed line", gowemf.CapFlat, gowemf.CapSquare, map[[2]int]color.NRGBA{
			{85, 50}: white, {12, 42}: black,
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, pm := testBackend(t)
			p := line(20, 50, 80, 50)
			if test.name == "a reversed line" {
				p = line(80, 50, 20, 50)
			}
			s := gowemf.Stroke{
				Paint:     gowemf.Paint{Kind: gowemf.PaintSolid, Color: black},
				Width:     20,
				Transform: gowemf.Identity(),
				Cap:       test.start,
				EndCap:    test.end,
				Join:      gowemf.JoinMiter,
				Dash:      gowemf.DashSolid,
			}
			if err := b.StrokePath(p, s, nil); err != nil {
				t.Fatal(err)
			}
			b.close()
			for pt, want := range test.want {
				expect(t, pm, pt[0], pt[1], want)
			}
		})
	}
}

// TestEndCapTranslucent checks that a translucent pen covers the end of a
// line once, where the cap and the line overlap.
func TestEndCapTranslucent(t *testing.T) {
	b, pm := testBackend(t)
	s := gowemf.Stroke{
		Paint:     gowemf.Paint{Kind: gowemf.PaintSolid, Color: color.NRGBA{0, 0, 0, 128}},
		Width:     20,
		Transform: gowemf.Identity(),
		Cap:       gowemf.CapFlat,
		EndCap:    gowemf.CapRound,
		Dash:      gowemf.DashSolid,
	}
	if err := b.StrokePath(line(20, 50, 80, 50), s, nil); err != nil {
		t.Fatal(err)
	}
	b.close()
	gray := color.NRGBA{127, 127, 127, 255}
	expect(t, pm, 50, 50, gray)
	expect(t, pm, 76, 50, gray)
	expect(t, pm, 85, 50, gray)
}

// TestPatternFlip checks the textures of EMF+ that mirror in one direction.
// The pattern filters its pixels, so the colors are near the pixels.
func TestPatternFlip(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, red)
	img.SetNRGBA(1, 0, green)
	img.SetNRGBA(0, 1, blue)
	img.SetNRGBA(1, 1, black)
	for _, test := range []struct {
		wrap gowemf.WrapMode
		want map[[2]int]color.NRGBA
	}{
		{gowemf.WrapTileFlipX, map[[2]int]color.NRGBA{{5, 5}: red, {15, 5}: green, {25, 5}: green, {35, 5}: red, {5, 25}: red, {5, 35}: blue}},
		{gowemf.WrapTileFlipY, map[[2]int]color.NRGBA{{5, 5}: red, {25, 5}: red, {35, 5}: green, {5, 15}: blue, {5, 25}: blue, {5, 35}: red}},
		{gowemf.WrapTileFlipXY, map[[2]int]color.NRGBA{{25, 5}: green, {5, 25}: blue, {25, 25}: black}},
		{gowemf.WrapTile, map[[2]int]color.NRGBA{{25, 5}: red, {5, 25}: red, {35, 35}: black}},
	} {
		b, pm := testBackend(t)
		paint := gowemf.Paint{Kind: gowemf.PaintPattern, Pattern: img, PatternTransform: gowemf.Matrix{M11: 10, M22: 10}, Wrap: test.wrap}
		area := gowemf.Path{
			Verbs:  []gowemf.PathVerb{gowemf.PathMoveTo, gowemf.PathLineTo, gowemf.PathLineTo, gowemf.PathLineTo, gowemf.PathClose},
			Points: []gowemf.Point{{X: 0, Y: 0}, {X: 40, Y: 0}, {X: 40, Y: 40}, {X: 0, Y: 40}},
		}
		if err := b.FillPath(area, gowemf.NonZero, paint, nil); err != nil {
			t.Fatal(err)
		}
		b.close()
		for pt, want := range test.want {
			if got := pixel(pm, pt[0], pt[1]); !near(got, want, 32) {
				t.Errorf("wrap %d: pixel %v = %v, want %v", test.wrap, pt, got, want)
			}
		}
	}
}

func TestOpenEnds(t *testing.T) {
	p := gowemf.Path{
		Verbs: []gowemf.PathVerb{
			gowemf.PathMoveTo, gowemf.PathLineTo, gowemf.PathCubicTo,
			gowemf.PathMoveTo, gowemf.PathLineTo, gowemf.PathLineTo, gowemf.PathClose,
			gowemf.PathMoveTo, gowemf.PathLineTo,
		},
		Points: []gowemf.Point{
			{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 0}, {X: 20, Y: 10}, {X: 20, Y: 20},
			{X: 50, Y: 50}, {X: 60, Y: 50}, {X: 60, Y: 60},
			{X: 5, Y: 5}, {X: 5, Y: 5},
		},
	}
	starts, ends := openEnds(p)
	if len(starts) != 1 || len(ends) != 1 {
		t.Fatalf("got %d starts and %d ends, want 1 and 1", len(starts), len(ends))
	}
	if want := (capEnd{at: gowemf.Point{}, out: gowemf.Point{X: -10}}); starts[0] != want {
		t.Errorf("start = %v, want %v", starts[0], want)
	}
	if want := (capEnd{at: gowemf.Point{X: 20, Y: 20}, out: gowemf.Point{Y: 10}}); ends[0] != want {
		t.Errorf("end = %v, want %v", ends[0], want)
	}
}
