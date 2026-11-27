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

// TestWarps checks the shapes of the types of text effects with their
// default adjust values, as LibreOffice draws them: the top of an arch, the
// peak of a triangle, the sides of a fade, the slants and the cans, and the
// types that bend the text along straight lines, the stop and the
// cascades.
func TestWarps(t *testing.T) {
	for _, tt := range []struct {
		typ        int
		u, v, x, y float64
	}{
		{sptTextArchUpCurve, 0.5, 0, 0.5, 0},
		{sptTextArchUpCurve, 0, 0, 0, 0.5},
		{sptTextArchDownPour, 0.5, 1, 0.5, 1},
		{sptTextArchDownPour, 0.5, 0, 0.5, 0.75},
		{sptTextCircleCurve, 0.5, 0, 1, 0.5},
		{sptTextTriangle, 0, 0, 0, 0.5},
		{sptTextTriangle, 0.5, 0, 0.5, 0},
		{sptTextChevronInverted, 0.5, 1, 0.5, 1},
		{sptTextWave1, 0.25, 0, 0.25, 0},
		{sptTextWave1, 0.75, 0, 0.75, 2 * 1404.0 / 21600},
		{sptTextFadeUp, 0, 0, 1.0 / 3, 0},
		{sptTextFadeUp, 0, 1, 0, 1},
		{sptTextSlantUp, 1, 0, 1, 0},
		{sptTextSlantDown, 0, 0, 0, 0},
		{sptTextSlantDown, 1, 1, 1, 1},
		{sptTextSlantDown, 1, 0, 1, 1 - 12000.0/21600},
		{sptTextDeflate, 0.5, 0, 0.5, 8100.0 / 21600},
		{sptTextInflate, 0, 0, 0, 2945.0 / 21600},
		{sptTextCanUp, 0.5, 0, 0.5, 0},
		{sptTextCanUp, 0, 1, 0, 1},
		{sptTextCanDown, 0.5, 1, 0.5, 1},
		{sptTextCanDown, 0, 0, 0, 0},
		{sptTextStop, 0, 0, 0, 0.125},
		{sptTextStop, 0.5, 0, 0.5, 0},
		{sptTextStop, 0.5, 1, 0.5, 1},
		{sptTextCascadeUp, 1, 1, 1, 9600.0 / 21600},
		{sptTextCascadeUp, 0, 0, 0, (1 - 9600.0/21600) / 4},
		{sptTextCascadeDown, 0, 1, 0, 9600.0 / 21600},
	} {
		wp := warpOf(tt.typ, Chain{&Props{}}, 0.25, 200, 100)
		if wp == nil {
			t.Fatalf("type %d: expected a warp", tt.typ)
		}
		if x, y := wp(tt.u, tt.v); !near(x, tt.x) || !near(y, tt.y) {
			t.Errorf("type %d: expected (%g, %g) at (%g, %g), got (%g, %g)", tt.typ, tt.x, tt.y, tt.u, tt.v, x, y)
		}
	}
	for _, typ := range []int{SptTextEffectMin, 142, 147, 152, 166} {
		if warpOf(typ, Chain{&Props{}}, 0.25, 200, 100) != nil {
			t.Errorf("type %d: expected plain text without a warp", typ)
		}
	}
}

// minus90 is the angle -90 degrees in 16.16 bits, as a property holds it.
const minus90 = 1<<32 - 90<<16

// TestWarpAdjust checks that the adjust values of a shape change the shape
// of its text: the depth of a curve, the height of a wave, the span of an
// arch and the band of a pour.
func TestWarpAdjust(t *testing.T) {
	for _, tt := range []struct {
		typ        int
		vals       []uint32
		u, v, x, y float64
	}{
		// the sides of a triangle
		{sptTextTriangle, []uint32{5400}, 0, 0, 0, 0.25},
		// the middle of the bottom of a chevron
		{sptTextChevron, []uint32{2700}, 0.5, 1, 0.5, 0.875},
		// the height of a wave
		{sptTextWave3, []uint32{2700}, 0.125, 0, 0.125, 0},
		{sptTextWave3, []uint32{2700}, 0.375, 0, 0.375, 0.25},
		// the depth of a can
		{sptTextCanUp, []uint32{16200}, 0, 0, 0, 0.25},
		// an arch that starts at 135 degrees spans 270 degrees, and its
		// text starts under the middle on the left
		{sptTextArchUpCurve, []uint32{135 << 16}, 0, 0, 0.5 - 0.5*math.Sqrt2/2, 0.5 + 0.5*math.Sqrt2/2},
		// an arch down that starts at 45 degrees spans 90 degrees
		{sptTextArchDownPour, []uint32{45 << 16}, 0, 1, 0.5 - 0.5*math.Sqrt2/2, 0.5 + 0.5*math.Sqrt2/2},
		// the inner radius of the band of a pour
		{sptTextArchUpPour, []uint32{180 << 16, 2700}, 0.5, 1, 0.5, 0.375},
		// a circle that starts at -90 degrees spans half of the circle
		{sptTextCirclePour, []uint32{minus90}, 0, 0, 0.5, 0},
		// a stop and a cascade
		{sptTextStop, []uint32{5400}, 0, 1, 0, 0.75},
		{sptTextCascadeDown, []uint32{5400}, 1, 0, 1, 0.1875},
	} {
		var p Props
		for i, v := range tt.vals {
			p.Set(PropAdjust1+uint16(i), v)
		}
		wp := warpOf(tt.typ, Chain{&p}, 0.25, 200, 100)
		if wp == nil {
			t.Fatalf("type %d: expected a warp", tt.typ)
		}
		if x, y := wp(tt.u, tt.v); !near(x, tt.x) || !near(y, tt.y) {
			t.Errorf("type %d %v: expected (%g, %g) at (%g, %g), got (%g, %g)", tt.typ, tt.vals, tt.x, tt.y, tt.u, tt.v, x, y)
		}
	}
}

// TestWarpPath checks that a straight line of the text follows the curve
// of a warp, and that a shape of an arch keeps its text in its box.
func TestWarpPath(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(0, 0)
	pb.LineTo(200, 0)
	p, _ := pb.Finish()
	q, ok := warpPath(p, warpOf(sptTextTriangle, Chain{&Props{}}, 0, 200, 100), 200, 100)
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
		// the narrow side of a trapezoid
		{8, nil, 200, 100, []float64{50000}},
		// the angles of a block arc, the end mirrors the start, and its
		// inner radius
		{95, nil, 100, 100, []float64{10800000, 0, 25000}},
		{95, map[uint16]uint32{PropAdjust1: 150 << 16, PropAdjust1 + 1: 2700}, 100, 100, []float64{9000000, 1800000, 37500}},
		// the U-turn arrow has no adjust values
		{101, nil, 100, 100, []float64{28264, 27546, 27593, 39583, 66019}},
		// the bent arrows
		{91, nil, 100, 100, []float64{29444, 28148, 30093, 42870}},
		{90, nil, 100, 100, []float64{28056, 28380, 33333}},
		// a curved arrow: the width of the shaft and of the head, and the
		// length of the head
		{102, nil, 100, 100, []float64{20000, 40000, 33333}},
		{104, map[uint16]uint32{PropAdjust1: 10000, PropAdjust1 + 1: 16000, PropAdjust1 + 2: 12000}, 100, 100, []float64{1852, 53704, 55556}},
		// the ribbons: the fold, and the width of the middle
		{53, nil, 100, 100, []float64{12500, 50000}},
		{54, map[uint16]uint32{PropAdjust1: 4000, PropAdjust1 + 1: 16000}, 100, 100, []float64{25926, 62963}},
		{107, nil, 100, 100, []float64{25000, 50000, 12500}},
		// the arrow callouts: the shaft, the head, its length and the box
		{77, nil, 100, 100, []float64{25000, 25000, 16667, 66667}},
		{78, map[uint16]uint32{PropAdjust1: 12000, PropAdjust1 + 1: 4000, PropAdjust1 + 2: 19000, PropAdjust1 + 3: 7000}, 100, 100, []float64{35185, 31481, 12037, 55556}},
		{82, nil, 100, 100, []float64{25000, 24537, 12500, 50000}},
		{83, nil, 100, 100, []float64{12963, 12500, 12500, 50000}},
		// a callout with a vertical line
		{178, nil, 100, 100, []float64{18750, -8333, 112500, -8333}},
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
	if got := adjustOf(76, Chain{&Props{}}, []string{"adj1"}, 100, 100); got != nil {
		t.Errorf("expected no values for the quad arrow, got %+v", got)
	}
	if got := adjustOf(61, Chain{&Props{}}, []string{"adj1", "adj2"}, 100, 100); got != nil {
		t.Errorf("expected the defaults of ECMA-376 for the unknown defaults, got %+v", got)
	}
}

// TestPresetFrames checks the shapes of Office Drawing that their presets
// of ECMA-376 draw in a frame other than the box: the trapezoid, which
// has its narrow side down, the arc, whose circle has its center in the
// box, and the circular arrow, whose head goes out of the box.
func TestPresetFrames(t *testing.T) {
	s := &Style{}
	bounds := func(typ int, vals map[uint16]uint32, w, h float64) (okage.Rect, []okage.Point, bool) {
		t.Helper()
		sh := &Shape{Type: typ}
		for id, v := range vals {
			sh.Props.Set(id, v)
		}
		paths, filled := s.Geometry(sh, Chain{&sh.Props}, w, h)
		pb := okage.NewPathBuilder()
		for _, p := range paths {
			pb.PushPath(p.Path)
		}
		all, ok := pb.Finish()
		if !ok {
			t.Fatalf("type %d: expected a path", typ)
		}
		b, _ := all.ComputeTightBounds()
		return b, all.Points(), filled
	}
	has := func(pts []okage.Point, x, y float64) bool {
		for _, p := range pts {
			if math.Abs(float64(p.X)-x) < 0.5 && math.Abs(float64(p.Y)-y) < 0.5 {
				return true
			}
		}
		return false
	}
	// the trapezoid is wide at the top and narrow at the bottom
	_, pts, _ := bounds(8, nil, 200, 100)
	for _, p := range [][2]float64{{0, 0}, {200, 0}, {50, 100}, {150, 100}} {
		if !has(pts, p[0], p[1]) {
			t.Errorf("trapezoid: expected the corner %v, got %v", p, pts)
		}
	}
	if r := s.TextRect(&Shape{Type: 8}, Chain{&Props{}}, 200, 100); r.Top() > 1 || r.Bottom() < 60 {
		t.Errorf("trapezoid: expected the text from the wide side, got %v", r)
	}
	// the default arc goes from the top left corner to the bottom right
	// corner, and an arc of half of the circle fills the box too
	for _, vals := range []map[uint16]uint32{nil, {PropAdjust1: minus90, PropAdjust1 + 1: 90 << 16}} {
		b, pts, filled := bounds(19, vals, 200, 100)
		if filled || !near(float64(b.Left()), 0) || !near(float64(b.Top()), 0) || !near(float64(b.Right()), 200) || !near(float64(b.Bottom()), 100) {
			t.Errorf("arc %v: expected an open arc that fills the box, got %v", vals, b)
		}
		if !has(pts, 0, 0) {
			t.Errorf("arc %v: expected the start at the top left corner, got %v", vals, pts)
		}
	}
	// the ring of a circular arrow touches the box, and its head goes out
	// by at most an eighth of the shorter side
	b, _, _ := bounds(99, nil, 200, 100)
	if !near(float64(b.Left()), 0) || !near(float64(b.Top()), 0) || b.Right() < 205 || b.Right() > 212.6 {
		t.Errorf("circular arrow: expected the ring in the box and the head out of it, got %v", b)
	}
	if bleed := s.Bleed(&Shape{Type: 99}, Chain{&Props{}}, Rect{Right: 200, Bottom: 100}); bleed < 6 {
		t.Errorf("circular arrow: expected a bleed for the head, got %g", bleed)
	}
	// the arrows that bend are filled, and the connectors are not
	for _, tt := range []struct {
		typ    int
		filled bool
	}{{91, true}, {102, true}, {90, true}, {120, true}, {177, true}, {34, false}, {38, false}, {19, false}} {
		if _, _, filled := bounds(tt.typ, nil, 200, 100); filled != tt.filled {
			t.Errorf("type %d: expected filled %t", tt.typ, tt.filled)
		}
	}
	// the callouts 178 to 181 draw their line on the left of the box
	b, _, _ = bounds(178, nil, 216, 100)
	if !near(float64(b.Left()), -18) {
		t.Errorf("callout 178: expected the line at -18, got %v", b)
	}
}
