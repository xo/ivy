package odraw

import (
	"math"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/fontdb"
)

// wordArtShape returns a shape of the type typ with the text of WordArt
// and the properties vals.
func wordArtShape(typ int, text string, vals map[uint16]uint32) *Shape {
	var b []byte
	for _, r := range text {
		b = le.AppendUint16(b, uint16(r))
	}
	var p Props
	opt := le.AppendUint16(nil, PropGtextUnicode|0x8000)
	opt = le.AppendUint32(opt, uint32(len(b)))
	p.Read(append(opt, b...), 1)
	for id, v := range vals {
		p.Set(id, v)
	}
	return &Shape{Type: typ, Props: p}
}

// wordArtBounds returns the bounds of each path of the WordArt shape sh in
// a box of w by h points at the origin.
func wordArtBounds(t *testing.T, sh *Shape, w, h float64) []okage.Rect {
	t.Helper()
	s := &Style{Fonts: fontdb.Default()}
	paths, ok := s.wordArt(sh, Chain{&sh.Props}, w, h)
	if !ok {
		t.Fatal("expected the outlines of the text")
	}
	out := make([]okage.Rect, 0, len(paths))
	for _, p := range paths {
		b, ok := p.Path.ComputeTightBounds()
		if !ok {
			t.Fatal("expected the bounds of a line")
		}
		out = append(out, b)
	}
	return out
}

// TestWordArtAlign checks that the longest line of WordArt fills the
// width, and that a shorter line is in the middle by default, at the left
// or at the right as gtextAlign says. The rows can go in the reverse
// order.
func TestWordArtAlign(t *testing.T) {
	for _, tt := range []struct {
		align   uint32
		reverse bool
		left    float64
		right   float64
	}{
		{alignCenter, false, -1, -1},
		{alignLeft, false, 0, -1},
		{alignRight, false, -1, 200},
		{alignCenter, true, -1, -1},
	} {
		vals := map[uint16]uint32{PropGtextAlign: tt.align}
		if tt.reverse {
			vals[PropGtextBools] = gtextReverseRows | gtextReverseRows<<16
		}
		b := wordArtBounds(t, wordArtShape(SptTextEffectMin, "Hi\nthere", vals), 200, 100)
		short, long := b[0], b[1]
		if tt.reverse {
			short, long = b[1], b[0]
			if short.Top() < 49.5 {
				t.Errorf("expected the short line at the bottom, got %v", short)
			}
		}
		if !near(float64(long.Left()), 0) || !near(float64(long.Right()), 200) {
			t.Errorf("align %d: expected the long line to fill the width, got %v", tt.align, long)
		}
		if short.Width() > 150 {
			t.Errorf("align %d: expected a short line, got %v", tt.align, short)
		}
		switch {
		case tt.left >= 0 && !near(float64(short.Left()), tt.left):
			t.Errorf("align %d: expected the short line at the left, got %v", tt.align, short)
		case tt.right >= 0 && !near(float64(short.Right()), tt.right):
			t.Errorf("align %d: expected the short line at the right, got %v", tt.align, short)
		case tt.left < 0 && tt.right < 0 && !near(float64(short.Left()+short.Right())/2, 100):
			t.Errorf("align %d: expected the short line in the middle, got %v", tt.align, short)
		}
	}
}

// TestGtextLine checks the spacing, the small capitals and the vertical
// text of a line of WordArt.
func TestGtextLine(t *testing.T) {
	f := (&Style{Fonts: fontdb.Default()}).face("", false, false)
	if f == nil {
		t.Fatal("expected a face")
	}
	bounds := func(g gtext, s string) okage.Rect {
		g.face = f
		p, ok := g.lineOutline(s)
		if !ok {
			t.Fatalf("expected the outline of %q", s)
		}
		b, _ := p.ComputeTightBounds()
		return b
	}
	gid, _ := f.Glyph('l')
	one := bounds(gtext{spacing: 1}, "ll").Width()
	two := bounds(gtext{spacing: 2}, "ll").Width()
	if !near(float64(two-one), float64(f.Advance(gid))) {
		t.Errorf("expected the double spacing to move the second glyph by one advance, got %g and %g", one, two)
	}
	capital := bounds(gtext{spacing: 1}, "A").Height()
	small := bounds(gtext{spacing: 1, smallcaps: true}, "a").Height()
	if !near(float64(small), float64(capital)*smallcapsScale) {
		t.Errorf("expected a small capital of %g of a capital, got %g and %g", smallcapsScale, small, capital)
	}
	if b := bounds(gtext{spacing: 1, vertical: true}, "AA"); b.Height() < 1.5*b.Width() {
		t.Errorf("expected the glyphs one below the other, got %v", b)
	}
}

// TestWarps checks the shapes of some types of text effects: the top of an
// arch, the peak of a triangle and the sides of a fade.
func TestWarps(t *testing.T) {
	for _, tt := range []struct {
		typ        int
		u, v, x, y float64
	}{
		{sptTextArchUpCurve, 0.5, 0, 0.5, 0},
		{sptTextArchUpCurve, 0, 0, 0, 0.5},
		{sptTextArchDownPour, 0.5, 1, 0.5, 1},
		{sptTextArchDownPour, 0.5, 0, 0.5, 0.75},
		{sptTextCircleCurve, 0.25, 0, 0.5, 0},
		{sptTextTriangle, 0, 0, 0, 0.5},
		{sptTextTriangle, 0.5, 0, 0.5, 0},
		{sptTextChevronInverted, 0.5, 1, 0.5, 1},
		{sptTextWave1, 0.25, 0, 0.25, 0},
		{sptTextFadeUp, 0, 0, 1.0 / 6, 0},
		{sptTextFadeUp, 0, 1, 0, 1},
		{sptTextSlantUp, 1, 0, 1, 0},
		{sptTextDeflate, 0.5, 0, 0.5, 0.1875},
	} {
		wp := warpOf(tt.typ, 0.25, 200, 100)
		if wp == nil {
			t.Fatalf("type %d: expected a warp", tt.typ)
		}
		if x, y := wp(tt.u, tt.v); !near(x, tt.x) || !near(y, tt.y) {
			t.Errorf("type %d: expected (%g, %g) at (%g, %g), got (%g, %g)", tt.typ, tt.x, tt.y, tt.u, tt.v, x, y)
		}
	}
	if warpOf(SptTextEffectMin, 0.25, 200, 100) != nil {
		t.Error("expected plain text without a warp")
	}
}

// TestWarpPath checks that a straight line of the text follows the curve
// of a warp, and that a shape of an arch keeps its text in its box.
func TestWarpPath(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(0, 0)
	pb.LineTo(200, 0)
	p, _ := pb.Finish()
	q, ok := warpPath(p, warpOf(sptTextTriangle, 0, 200, 100), 200, 100)
	if !ok {
		t.Fatal("expected the bent line")
	}
	// the line goes up from the middle of the left side to the top
	// center, and down to the middle of the right side
	pts := q.Points()
	mid := pts[len(pts)/2]
	if len(pts) < 8 || !near(float64(pts[0].Y), 50) || !near(float64(mid.X), 100) || !near(float64(mid.Y), 0) {
		t.Errorf("expected a peak, got %v", pts)
	}
	for _, typ := range []int{sptTextArchUpCurve, sptTextCirclePour, sptTextWave2} {
		for _, b := range wordArtBounds(t, wordArtShape(typ, "WordArt", nil), 200, 100) {
			if b.Left() < -0.5 || b.Top() < -0.5 || b.Right() > 200.5 || b.Bottom() > 100.5 {
				t.Errorf("type %d: expected the text in the box, got %v", typ, b)
			}
		}
	}
}

// TestAdjustRules checks the adjust values of ECMA-376 that some types of
// shapes of Office Drawing give: the defaults of Office Drawing, which a
// shape without adjust values takes, and the values that a shape gives.
func TestAdjustRules(t *testing.T) {
	for _, tt := range []struct {
		typ  int
		vals map[uint16]uint32
		w, h float64
		want []float64
	}{
		// the head of a right arrow starts at 16200 of 21600 by default
		{13, nil, 200, 100, []float64{50000, 50000}},
		{13, map[uint16]uint32{PropAdjust1 + 1: 2700}, 100, 100, []float64{75000, 25000}},
		// the stars of Office Drawing have the defaults of ECMA-376
		{58, nil, 100, 100, []float64{38250}},
		{187, nil, 100, 100, []float64{12500}},
		// the tip of a callout is measured from the center, and its
		// default is not known
		{62, map[uint16]uint32{PropAdjust1: 0, PropAdjust1 + 1: 21600}, 100, 100, []float64{-50000, 50000, 16667}},
		{61, nil, 100, 100, []float64{math.NaN(), math.NaN()}},
		// a line callout gives its points from the end of the line
		{41, map[uint16]uint32{PropAdjust1: 0, PropAdjust1 + 1: 21600, PropAdjust1 + 2: 10800, PropAdjust1 + 3: 5400}, 100, 100, []float64{25000, 50000, 100000, 0}},
		// a connector bends at the share of its width
		{34, map[uint16]uint32{PropAdjust1: 5400}, 100, 100, []float64{25000}},
		// the width of the slant of a parallelogram
		{sptParallelogram, nil, 200, 100, []float64{50000}},
	} {
		var p Props
		for id, v := range tt.vals {
			p.Set(id, v)
		}
		got := adjustValues(tt.typ, Chain{&p}, tt.w, tt.h)
		if len(got) != len(tt.want) {
			t.Fatalf("type %d: expected %d values, got %v", tt.typ, len(tt.want), got)
		}
		for i, g := range got {
			if w := tt.want[i]; math.IsNaN(w) != math.IsNaN(g) || !math.IsNaN(w) && math.Abs(g-w) > 1 {
				t.Errorf("type %d: expected the value %d to be %g, got %g", tt.typ, i+1, w, g)
			}
		}
	}
	// a type without a rule takes the defaults of ECMA-376
	if got := adjustOf(95, Chain{&Props{}}, []string{"adj1"}, 100, 100); got != nil {
		t.Errorf("expected no values for the block arc, got %+v", got)
	}
	if got := adjustOf(61, Chain{&Props{}}, []string{"adj1", "adj2"}, 100, 100); got != nil {
		t.Errorf("expected the defaults of ECMA-376 for the unknown defaults, got %+v", got)
	}
}
