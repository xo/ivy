package formula

import (
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/texmath"

	"github.com/xo/ivy/internal/display"
)

// recorder is a device that keeps the text and the origins of the glyphs,
// and the bounds of the paths that it fills, in the space of the page.
type recorder struct {
	display.Device
	stack  []okage.Transform
	t      okage.Transform
	text   strings.Builder
	points []okage.Point
	paths  []okage.Rect
}

func (r *recorder) Save()                    { r.stack = append(r.stack, r.t) }
func (r *recorder) Restore()                 { r.t, r.stack = r.stack[len(r.stack)-1], r.stack[:len(r.stack)-1] }
func (r *recorder) Concat(t okage.Transform) { r.t = r.t.PreConcat(t) }

func (r *recorder) FillPath(p *okage.Path, _ okage.FillRule, _ okage.Shader) {
	if q, ok := p.Transform(r.t); ok {
		r.paths = append(r.paths, q.Bounds())
	}
}

func (r *recorder) FillGlyphs(run display.GlyphRun, _ okage.Shader) {
	r.text.WriteString(run.Text)
	for _, g := range run.Glyphs {
		pt := okage.Point{X: g.X, Y: g.Y}
		r.t.MapPoint(&pt)
		r.points = append(r.points, pt)
	}
}

func TestObjectDraw(t *testing.T) {
	t.Parallel()
	ts, err := New()
	if err != nil {
		t.Fatal(err)
	}
	tree := texmath.List{texmath.Fraction{Num: italic("a"), Den: italic("b")}}
	obj, err := ts.Layout(tree, 20, texmath.Display, okage.ColorBlack)
	if err != nil {
		t.Fatal(err)
	}
	w, h := obj.Size()
	rect, _ := okage.RectFromXYWH(100, 200, float32(w), float32(h))
	r := &recorder{t: okage.IdentityTransform()}
	if err := obj.Draw(t.Context(), r, rect); err != nil {
		t.Fatal(err)
	}
	// The glyphs keep their text, for a writer of PDF.
	if got := r.text.String(); !strings.ContainsAny(got, "a𝑎") || !strings.ContainsAny(got, "b𝑏") {
		t.Errorf("expected the text of a and b, got %q", got)
	}
	// The numerator is above the denominator, and both are in the rect.
	if len(r.points) != 2 || r.points[0].Y >= r.points[1].Y {
		t.Fatalf("expected two glyphs, a above b, got %v", r.points)
	}
	for _, p := range r.points {
		if p.X < 100 || p.X > 100+float32(w) || p.Y < 200 || p.Y > 200+float32(h) {
			t.Errorf("expected the origin of each glyph in %v, got %v", rect, p)
		}
	}
	// The rule of the fraction is between them.
	if len(r.paths) != 1 || r.paths[0].Top() < r.points[0].Y || r.paths[0].Bottom() > r.points[1].Y {
		t.Errorf("expected the rule of the fraction between a and b, got %v", r.paths)
	}
}

func TestLayoutLimit(t *testing.T) {
	t.Parallel()
	ts, err := New()
	if err != nil {
		t.Fatal(err)
	}
	var n texmath.Node = italic("x")
	for range 300 {
		n = texmath.Scripts{Base: n, Sup: italic("y")}
	}
	if _, err := ts.Layout(n, 12, texmath.Text, okage.ColorBlack); err == nil {
		t.Errorf("expected an error for a tree of 300 levels")
	}
}
