package drawingml

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

func TestPreset(t *testing.T) {
	if n := len(presets); n != 187 {
		t.Errorf("%d presets, want 187", n)
	}
	for _, name := range []string{"rect", "upArrow", "upDownArrow", "wedgeRoundRectCallout", "accentBorderCallout1"} {
		g, ok := Preset(name)
		if !ok || g.name != name {
			t.Errorf("Preset(%q) = %v, %t", name, g, ok)
		}
	}
	for _, name := range []string{"", "Rect", "upArrows", "zzz"} {
		if _, ok := Preset(name); ok {
			t.Errorf("Preset(%q) is true", name)
		}
	}
}

// TestPresetNames makes sure that each argument of each preset is a number,
// a built-in variable, an adjust value or a guide before it.
func TestPresetNames(t *testing.T) {
	for i := range presets {
		g := &presets[i]
		known := map[string]bool{}
		check := func(where, s string) {
			if s == "" {
				return
			}
			if _, err := strconv.ParseFloat(s, 64); err == nil {
				return
			}
			if _, ok := builtin(s, 1, 1); ok || known[s] {
				return
			}
			t.Errorf("%s: %s uses the unknown name %q", g.name, where, s)
		}
		for _, a := range g.adjust {
			for _, s := range a.args {
				check(a.name, s)
			}
			known[a.name] = true
		}
		for _, gd := range g.guides {
			for _, s := range gd.args[:ops[gd.op].n] {
				check(gd.name, s)
			}
			known[gd.name] = true
		}
		for _, p := range g.paths {
			for _, c := range p.commands {
				for _, s := range c.args {
					check(verbNames[c.verb], s)
				}
			}
		}
		for _, s := range []string{g.text.l, g.text.t, g.text.r, g.text.b} {
			check("rect", s)
		}
	}
}

// TestPresetsEvaluate evaluates each preset for a few sizes, and makes sure
// that each path is finite, not empty and near the box.
func TestPresetsEvaluate(t *testing.T) {
	sizes := [][2]float64{{200, 100}, {100, 200}, {100, 100}, {914400, 457200}}
	for i := range presets {
		g := &presets[i]
		for _, size := range sizes {
			w, h := size[0], size[1]
			s, err := g.Evaluate(w, h, nil)
			if err != nil {
				t.Fatalf("%s %gx%g: %v", g.name, w, h, err)
			}
			if len(s.Paths) != len(g.paths) || len(s.Paths) == 0 {
				t.Errorf("%s %gx%g: %d paths, want %d", g.name, w, h, len(s.Paths), len(g.paths))
			}
			// A callout points outside its box. Allow one more box on each
			// side.
			for j, p := range s.Paths {
				for _, pt := range p.Path.Points() {
					x, y := float64(pt.X), float64(pt.Y)
					if math.IsNaN(x) || math.IsNaN(y) || x < -w || x > 2*w || y < -h || y > 2*h {
						t.Errorf("%s %gx%g: path %d has the point (%g, %g)", g.name, w, h, j, x, y)
						break
					}
				}
			}
			r := s.Text
			l, top, right, bottom := float64(r.Left()), float64(r.Top()), float64(r.Right()), float64(r.Bottom())
			if l < -0.01*w || top < -0.01*h || right > 1.01*w || bottom > 1.01*h {
				t.Errorf("%s %gx%g: text rectangle %v is outside the box", g.name, w, h, r)
			}
		}
	}
}

// TestLine evaluates the presets of lines, which have a height of zero.
func TestLine(t *testing.T) {
	for _, name := range []string{"line", "straightConnector1", "bentConnector3", "curvedConnector3"} {
		g, _ := Preset(name)
		s, err := g.Evaluate(200, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(s.Paths) != 1 {
			t.Fatalf("%s: %d paths", name, len(s.Paths))
		}
		checkFinite(t, name, s)
	}
}

// point is a point that a test expects in a path.
type point struct{ x, y float64 }

func TestKnownPoints(t *testing.T) {
	r := 100 * 16667 / 100000.0
	k := 4.0 / 3 * math.Tan(math.Pi/8)
	// star5 at 200x100
	swd2, shd2 := 100*105146/100000.0, 50*110557/100000.0
	starX1 := 100 - swd2*math.Cos(18*math.Pi/180)
	starY1 := shd2 - shd2*math.Sin(18*math.Pi/180)
	tests := []struct {
		name   string
		w, h   float64
		path   int
		points []point
		text   [4]float64
	}{
		{"rect", 200, 100, 0, []point{{0, 0}, {200, 0}, {200, 100}, {0, 100}}, [4]float64{0, 0, 200, 100}},
		{"roundRect", 200, 100, 0, []point{
			{0, r}, {r, 0}, {200 - r, 0}, {200, r}, {200, 100 - r}, {200 - r, 100}, {r, 100}, {0, 100 - r},
			// the first control point of the first corner
			{0, r - k*r},
		}, [4]float64{r * 0.29289, r * 0.29289, 200 - r*0.29289, 100 - r*0.29289}},
		{"ellipse", 200, 100, 0, []point{{0, 50}, {100, 0}, {200, 50}, {100, 100}, {0, 50 - 50*k}, {100 - 100*k, 0}}, [4]float64{
			100 - 100*math.Cos(math.Pi/4), 50 - 50*math.Sin(math.Pi/4), 100 + 100*math.Cos(math.Pi/4), 50 + 50*math.Sin(math.Pi/4),
		}},
		{"rightArrow", 200, 100, 0, []point{{0, 25}, {150, 25}, {150, 0}, {200, 50}, {150, 100}, {150, 75}, {0, 75}}, [4]float64{0, 25, 175, 75}},
		{"upArrow", 200, 100, 0, []point{{0, 50}, {50, 50}, {50, 100}, {150, 100}, {150, 50}, {200, 50}, {100, 0}}, [4]float64{50, 25, 150, 100}},
		{"star5", 200, 100, 0, []point{{starX1, starY1}, {100, 0}, {200 - starX1, starY1}}, [4]float64{}},
		{"wedgeRectCallout", 200, 100, 0, []point{{0, 0}, {200, 0}, {200, 100}, {200.0 * 5 / 12, 100}, {175.0 / 3, 112.5}, {200.0 * 2 / 12, 100}, {0, 100}}, [4]float64{0, 0, 200, 100}},
		{"chevron", 200, 100, 0, []point{{0, 0}, {150, 0}, {200, 50}, {150, 100}, {0, 100}, {50, 50}}, [4]float64{}},
		{"cloud", 200, 100, 0, []point{{3900 * 200 / 43200.0, 14370 * 100 / 43200.0}}, [4]float64{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, ok := Preset(tt.name)
			if !ok {
				t.Fatalf("no preset %s", tt.name)
			}
			s, err := g.Evaluate(tt.w, tt.h, nil)
			if err != nil {
				t.Fatal(err)
			}
			p := s.Paths[tt.path].Path
			for _, want := range tt.points {
				if !hasPoint(p, want) {
					t.Errorf("no point (%g, %g) in %v", want.x, want.y, p.Points())
				}
			}
			if tt.text != ([4]float64{}) {
				checkRect(t, s.Text, tt.text)
			}
			// The arcs of cloud end a little outside the box.
			b, _ := p.ComputeTightBounds()
			w, h := float32(tt.w), float32(tt.h)
			if b.Left() < -0.005*w || b.Top() < -0.005*h || b.Right() > 1.005*w || b.Bottom() > 1.005*h {
				if tt.name != "wedgeRectCallout" {
					t.Errorf("bounds %v outside the box", b)
				}
			}
		})
	}
}

// TestArc checks the arc of pie, whose guides compute the end of the arc. The
// arc must end there, and its center must be the center of the box, for an
// ellipse that is not a circle.
func TestArc(t *testing.T) {
	g, _ := Preset("pie")
	adjust := parseAdjust(t, `<a:avLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:gd name="adj1" fmla="val 2700000"/><a:gd name="adj2" fmla="val 12000000"/></a:avLst>`)
	s, err := g.Evaluate(200, 100, adjust)
	if err != nil {
		t.Fatal(err)
	}
	at := func(deg float64) point {
		a := deg * math.Pi / 180
		p := math.Atan2(100*math.Sin(a), 50*math.Cos(a))
		return point{100 + 100*math.Cos(p), 50 + 50*math.Sin(p)}
	}
	pts := s.Paths[0].Path.Points()
	start, end := at(45), at(200)
	if !near(pts[0], start) {
		t.Errorf("start %v, want %v", pts[0], start)
	}
	// The arc ends before the line to the center.
	if p := pts[len(pts)-2]; !near(p, end) {
		t.Errorf("end %v, want %v", p, end)
	}
	if p := pts[len(pts)-1]; !near(p, point{100, 50}) {
		t.Errorf("last point %v, want the center", p)
	}
	// Each point of the arc is on the ellipse, with the tolerance of the
	// cubic curves.
	for i := 0; i < len(pts)-1; i += 3 {
		x, y := (float64(pts[i].X)-100)/100, (float64(pts[i].Y)-50)/50
		if d := x*x + y*y; math.Abs(d-1) > 1e-4 {
			t.Errorf("point %d %v is not on the ellipse: %g", i, pts[i], d)
		}
	}
	checkRect(t, s.Text, [4]float64{100 - 100*math.Sqrt2/2, 50 - 50*math.Sqrt2/2, 100 + 100*math.Sqrt2/2, 50 + 50*math.Sqrt2/2})
}

const customXML = `<a:custGeom xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:avLst><a:gd name="adj" fmla="val 25000"/></a:avLst>
  <a:gdLst><a:gd name="x" fmla="*/ w adj 100000"/></a:gdLst>
  <a:ahLst/>
  <a:cxnLst/>
  <a:rect l="x" t="t" r="r" b="b"/>
  <a:pathLst>
    <a:path w="100" h="100" extrusionOk="0">
      <a:moveTo><a:pt x="0" y="0"/></a:moveTo>
      <a:lnTo><a:pt x="100" y="50"/></a:lnTo>
      <a:quadBezTo><a:pt x="100" y="100"/><a:pt x="50" y="100"/></a:quadBezTo>
      <a:close/>
    </a:path>
    <a:path fill="darkenLess" stroke="false">
      <a:moveTo><a:pt x="x" y="t"/></a:moveTo>
      <a:cubicBezTo><a:pt x="x" y="b"/><a:pt x="r" y="b"/><a:pt x="r" y="t"/></a:cubicBezTo>
      <a:extLst/>
    </a:path>
  </a:pathLst>
</a:custGeom>`

func TestCustom(t *testing.T) {
	g := parseCustom(t, customXML)
	s, err := g.Evaluate(400, 200, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Paths) != 2 {
		t.Fatalf("%d paths", len(s.Paths))
	}
	p0, p1 := s.Paths[0], s.Paths[1]
	if p0.Fill != FillNorm || !p0.Stroke || p0.ExtrusionOK {
		t.Errorf("path 0: %v %t %t", p0.Fill, p0.Stroke, p0.ExtrusionOK)
	}
	if p1.Fill != FillDarkenLess || p1.Stroke || !p1.ExtrusionOK {
		t.Errorf("path 1: %v %t %t", p1.Fill, p1.Stroke, p1.ExtrusionOK)
	}
	for _, want := range []point{{0, 0}, {400, 100}, {400, 200}, {200, 200}} {
		if !hasPoint(p0.Path, want) {
			t.Errorf("path 0: no point %v in %v", want, p0.Path.Points())
		}
	}
	for _, want := range []point{{100, 0}, {100, 200}, {400, 200}, {400, 0}} {
		if !hasPoint(p1.Path, want) {
			t.Errorf("path 1: no point %v in %v", want, p1.Path.Points())
		}
	}
	checkRect(t, s.Text, [4]float64{100, 0, 400, 200})

	// The adjust value of the shape replaces that of the geometry.
	adjust := parseAdjust(t, `<a:avLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:gd name="adj" fmla="val 50000"/></a:avLst>`)
	s, err = g.Evaluate(400, 200, adjust)
	if err != nil {
		t.Fatal(err)
	}
	checkRect(t, s.Text, [4]float64{200, 0, 400, 200})
}

func TestFormulas(t *testing.T) {
	tests := []struct {
		fmla string
		want float64
	}{
		{"val 7", 7},
		{"val w", 400},
		{"*/ w 3 4", 300},
		{"*/ w 3 0", 0},
		{"+- w 10 20", 390},
		{"+/ w 100 5", 100},
		{"+/ w 100 0", 0},
		{"?: 1 2 3", 2},
		{"?: 0 2 3", 3},
		{"?: -1 2 3", 3},
		{"abs -5", 5},
		{"at2 1 1", 2700000},
		{"at2 0 -1", -5400000},
		{"cat2 10 3 4", 6},
		{"sat2 10 3 4", 8},
		{"cos 10 cd2", -10},
		{"sin 10 cd4", 10},
		{"tan 10 cd8", 10},
		{"max 3 -4", 3},
		{"min 3 -4", -4},
		{"mod 2 3 6", 7},
		{"pin 10 5 20", 10},
		{"pin 10 25 20", 20},
		{"pin 10 15 20", 15},
		{"pin 30 15 20", 30},
		{"sqrt 16", 4},
		{"sqrt -16", 0},
		{"val ss", 200},
		{"val ls", 400},
		{"val hd3", 200.0 / 3},
		{"val wd32", 12.5},
		{"val ssd8", 25},
		{"val 7cd8", 18900000},
		{"val nothing", 0},
		{"val NaN", 0},
		{"val -Inf", 0},
		{"val 1e300", maxValue},
		{"*/ 1e300 1e300 1", maxValue},
		{"tan 1 5400000", maxValue},
	}
	e := &evaluator{w: 400, h: 200, vars: map[string]float64{}}
	for _, tt := range tests {
		g, err := parseFormula("g", tt.fmla)
		if err != nil {
			t.Errorf("%q: %v", tt.fmla, err)
			continue
		}
		if got := e.eval(g); math.Abs(got-tt.want) > 1e-6*max(1, math.Abs(tt.want)) {
			t.Errorf("%q = %g, want %g", tt.fmla, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	const ns = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
	tests := []struct {
		xml  string
		want error
	}{
		{`<a:custGeom ` + ns + `><a:gdLst><a:gd name="x" fmla="pow 1 2"/></a:gdLst></a:custGeom>`, ivy.ErrFormat},
		{`<a:custGeom ` + ns + `><a:gdLst><a:gd name="x" fmla="*/ 1 2"/></a:gdLst></a:custGeom>`, ivy.ErrFormat},
		{`<a:custGeom ` + ns + `><a:gdLst><a:gd name="x" fmla=""/></a:gdLst></a:custGeom>`, ivy.ErrFormat},
		{`<a:custGeom ` + ns + `><a:gdLst><a:gd fmla="val 1"/></a:gdLst></a:custGeom>`, ivy.ErrFormat},
		{`<a:custGeom ` + ns + `><a:pathLst><a:path fill="bright"/></a:pathLst></a:custGeom>`, ivy.ErrFormat},
		{`<a:custGeom ` + ns + `><a:pathLst><a:path><a:cubicBezTo><a:pt x="1" y="1"/></a:cubicBezTo></a:path></a:pathLst></a:custGeom>`, ivy.ErrFormat},
		{`<a:custGeom ` + ns + `><a:gdLst>` + strings.Repeat(`<a:gd name="x" fmla="val 1"/>`, maxGuides+1) + `</a:gdLst></a:custGeom>`, ivy.ErrLimit},
		{`<a:custGeom ` + ns + `><a:pathLst>` + strings.Repeat(`<a:path/>`, maxPaths+1) + `</a:pathLst></a:custGeom>`, ivy.ErrLimit},
		{`<a:custGeom ` + ns + `><a:pathLst><a:path>` + strings.Repeat(`<a:close/>`, maxCommands+1) + `</a:path></a:pathLst></a:custGeom>`, ivy.ErrLimit},
	}
	for i, tt := range tests {
		doc, err := roxmltree.Parse(tt.xml)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseCustom(doc.RootElement()); !errors.Is(err, tt.want) {
			t.Errorf("%d: error %v, want %v", i, err, tt.want)
		}
	}
	g, _ := Preset("rect")
	for _, size := range [][2]float64{{-1, 1}, {1, math.NaN()}, {math.Inf(1), 1}, {1, 2 * maxSize}} {
		if _, err := g.Evaluate(size[0], size[1], nil); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("Evaluate(%g, %g): %v", size[0], size[1], err)
		}
	}
}

// TestHostile evaluates geometries with values that would overflow, divide
// by zero or give NaN, and makes sure that each point is finite and near the
// box.
func TestHostile(t *testing.T) {
	const ns = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
	tests := []string{
		`<a:gdLst><a:gd name="big" fmla="*/ 99999999999999999999 99999999999999999999 1"/><a:gd name="z" fmla="+/ 1 1 0"/></a:gdLst>
		<a:pathLst><a:path w="1" h="1"><a:moveTo><a:pt x="big" y="z"/></a:moveTo><a:lnTo><a:pt x="-big" y="big"/></a:lnTo>
		<a:arcTo wR="big" hR="1" stAng="big" swAng="-big"/><a:arcTo wR="0" hR="5" stAng="0" swAng="99999999999"/></a:path></a:pathLst>`,
		`<a:gdLst><a:gd name="t" fmla="tan 1 cd4"/><a:gd name="s" fmla="sqrt -1"/><a:gd name="a" fmla="at2 0 0"/></a:gdLst>
		<a:rect l="t" t="s" r="-t" b="a"/>
		<a:pathLst><a:path w="0" h="99999999999999999999999"><a:lnTo><a:pt x="t" y="t"/></a:lnTo><a:arcTo wR="t" hR="t" stAng="t" swAng="t"/>
		<a:quadBezTo><a:pt x="NaN" y="Inf"/><a:pt x="1e400" y="-1e400"/></a:quadBezTo></a:path></a:pathLst>`,
		`<a:pathLst><a:path w="0.5" h="1e-300"><a:arcTo wR="10" hR="10" stAng="0" swAng="21600000"/><a:close/><a:lnTo><a:pt x="1" y="1"/></a:lnTo></a:path></a:pathLst>`,
	}
	for i, body := range tests {
		g := parseCustom(t, `<a:custGeom `+ns+`>`+body+`</a:custGeom>`)
		for _, size := range [][2]float64{{100, 50}, {0, 0}, {maxSize, 1}} {
			s, err := g.Evaluate(size[0], size[1], nil)
			if err != nil {
				t.Fatalf("%d: %v", i, err)
			}
			checkFinite(t, fmt.Sprint(i), s)
		}
	}
}

func TestFillString(t *testing.T) {
	if s := FillLightenLess.String(); s != "lightenLess" {
		t.Errorf("FillLightenLess = %q", s)
	}
	if s := Fill(99).String(); s != "Fill(99)" {
		t.Errorf("Fill(99) = %q", s)
	}
}

// checkFinite makes sure that each point and the text rectangle of s are
// finite.
func checkFinite(t *testing.T, name string, s *Shape) {
	t.Helper()
	for _, p := range s.Paths {
		for _, pt := range p.Path.Points() {
			if !finite(pt.X) || !finite(pt.Y) {
				t.Errorf("%s: point %v", name, pt)
			}
		}
	}
	r := s.Text
	if !finite(r.Left()) || !finite(r.Top()) || !finite(r.Right()) || !finite(r.Bottom()) {
		t.Errorf("%s: text rectangle %v", name, r)
	}
}

func finite(v float32) bool {
	return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0)
}

func near(p okage.Point, want point) bool {
	return math.Abs(float64(p.X)-want.x) < 1e-3 && math.Abs(float64(p.Y)-want.y) < 1e-3
}

func hasPoint(p *okage.Path, want point) bool {
	for _, pt := range p.Points() {
		if near(pt, want) {
			return true
		}
	}
	return false
}

func checkRect(t *testing.T, r okage.Rect, want [4]float64) {
	t.Helper()
	got := [4]float32{r.Left(), r.Top(), r.Right(), r.Bottom()}
	for i := range got {
		if math.Abs(float64(got[i])-want[i]) > 1e-3 {
			t.Errorf("text rectangle %v, want %v", got, want)
			return
		}
	}
}

func parseCustom(t testing.TB, s string) *Geometry {
	t.Helper()
	doc, err := roxmltree.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseCustom(doc.RootElement())
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func parseAdjust(t testing.TB, s string) []Guide {
	t.Helper()
	doc, err := roxmltree.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	list, err := ParseAdjust(doc.RootElement())
	if err != nil {
		t.Fatal(err)
	}
	return list
}
