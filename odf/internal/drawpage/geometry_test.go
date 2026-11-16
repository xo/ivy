package drawpage

import (
	"math"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// enhanced parses a draw:enhanced-geometry with the attributes and the
// equations, for a shape of w by h points.
func enhanced(t *testing.T, attrs, equations string, w, h float64) *geometry {
	t.Helper()
	doc, err := roxmltree.Parse(`<draw:enhanced-geometry xmlns:draw="` + nsDraw + `" xmlns:svg="` + nsSVG + `" ` + attrs + `>` + equations + `</draw:enhanced-geometry>`)
	if err != nil {
		t.Fatal(err)
	}
	return parseGeometry(doc.RootElement(), w, h)
}

func TestFormulas(t *testing.T) {
	g := enhanced(t, `svg:viewBox="0 0 21600 21600" draw:modifiers="5400 3"`,
		`<draw:equation draw:name="f0" draw:formula="$0 *2"/>`+
			`<draw:equation draw:name="f1" draw:formula="?f0 +width/-2"/>`+
			`<draw:equation draw:name="f2" draw:formula="if($1 -4,1,max(abs(-7),sqrt(16)))"/>`+
			`<draw:equation draw:name="f3" draw:formula="cos(pi)*atan2(1,1)*4/pi"/>`+
			`<draw:equation draw:name="f4" draw:formula="?f5 +1"/>`+
			`<draw:equation draw:name="f5" draw:formula="?f4 +1"/>`+
			`<draw:equation draw:name="f6" draw:formula="logwidth"/>`+
			`<draw:equation draw:name="f7" draw:formula="(1"/>`, 72, 36)
	for _, c := range []struct {
		param string
		want  float64
	}{
		{"?f0", 10800},
		{"?f1", 0},
		{"?f2", 7},
		{"?f3", -1},
		// a cycle of equations gives a value and stops
		{"?f6", 2540},
		{"?f7", 0},
		{"$1", 3},
		{"$9", 0},
		{"right", 21600},
		{"12.5", 12.5},
	} {
		if got := g.param(c.param); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("%s: expected %g, got %g", c.param, c.want, got)
		}
	}
	_ = g.param("?f4")
}

func TestEnhancedPath(t *testing.T) {
	// a triangle and an arc of a quarter of a circle, in a viewBox of 100
	g := enhanced(t, `svg:viewBox="0 0 100 100" draw:enhanced-path="M 50 0 L 100 100 0 100 Z N F M 0 0 T 50 50 50 50 90 180 N" draw:text-areas="10 20 90 80"`, "", 200, 100)
	paths, ok := g.paths()
	if !ok || len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	b := paths[0].Path.Bounds()
	if b.Left() != 0 || b.Top() != 0 || b.Right() != 200 || b.Bottom() != 100 {
		t.Errorf("expected the triangle in the box of 200 by 100, got %v", b)
	}
	if paths[1].Fill == paths[0].Fill {
		t.Error("expected the second path without a fill")
	}
	r, ok := g.textRect()
	if !ok || r.X() != 20 || r.Y() != 20 || r.Width() != 160 || r.Height() != 60 {
		t.Errorf("expected the text area at (20, 20) of 160 by 60, got %v", r)
	}
}

func TestEmptyViewBox(t *testing.T) {
	// LibreOffice writes the shapes of DrawingML in the size of the shape,
	// in 1/100 mm
	g := enhanced(t, `svg:viewBox="0 0 0 0" draw:enhanced-path="M 0 0 L logwidth logheight N"`, "", 72, 36)
	paths, ok := g.paths()
	if !ok {
		t.Fatal("expected a path")
	}
	if b := paths[0].Path.Bounds(); math.Abs(float64(b.Right())-72) > 0.01 || math.Abs(float64(b.Bottom())-36) > 0.01 {
		t.Errorf("expected the line to the corner (72, 36), got %v", b)
	}
}

func TestParseTransform(t *testing.T) {
	m := parseTransform("rotate (1.5707963267949) translate (2cm 72pt)")
	p := []okage.Point{okage.PointFromXY(10, 0)}
	m.MapPoints(p)
	// a point on the x axis turns counterclockwise to above the origin,
	// then moves
	if math.Abs(float64(p[0].X)-72/2.54*2) > 0.01 || math.Abs(float64(p[0].Y)-(72-10)) > 0.01 {
		t.Errorf("expected (%g, 62), got %v", 72/2.54*2, p[0])
	}
	if !parseTransform("matrix (1 0 0 1 3pt 4pt) skewX (bad)").IsFinite() {
		t.Error("expected a finite transform")
	}
}

func TestLength(t *testing.T) {
	for _, c := range []struct {
		s    string
		want float64
		ok   bool
	}{
		{"2.54cm", 72, true},
		{"25.4mm", 72, true},
		{"1in", 72, true},
		{"12pt", 12, true},
		{"16px", 12, true},
		{"12", 0, false},
		{"50%", 0, false},
	} {
		got, ok := length(c.s)
		if ok != c.ok || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: expected %g %v, got %g %v", c.s, c.want, c.ok, got, ok)
		}
	}
}

func FuzzGeometry(f *testing.F) {
	f.Add("M 0 0 L 21600 21600 Z N", "?f0 *2+sin($0 )", "10800")
	f.Add("U 10800 10800 10800 10800 0 360 Z N", "if(1,2,3)", "")
	f.Add("M0 0 A 0 0 100 100 0 50 50 0 W 0 0 1 1 1 0 0 1 X 5 5 Y 6 6 G 1 1 0 90 Q 1 1 2 2 C 1 2 3 4 5 6", "atan2(?f0 ,1)", "1 2")
	f.Fuzz(func(t *testing.T, path, formula, modifiers string) {
		doc, err := roxmltree.Parse(`<g xmlns:draw="` + nsDraw + `" xmlns:svg="` + nsSVG + `"><draw:equation draw:name="f0"/></g>`)
		if err != nil {
			t.Fatal(err)
		}
		g := parseGeometry(doc.RootElement(), 100, 50)
		g.path, g.textAreas = path, path
		g.modifiers = numbers(modifiers)
		if x, err := parseExpr(formula); err == nil {
			g.formulas[0] = x
		}
		_, _ = g.paths()
		_, _ = g.textRect()
	})
}
