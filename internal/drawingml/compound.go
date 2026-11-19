package drawingml

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/path"

	"github.com/xo/ivy/internal/display"
)

// This file draws the compound lines whose lines are not symmetric about
// the middle of the line, thickThin and thinThick. okage has no offset of a
// path, so a line on one side of the path is a stroke clipped to the region
// on that side.

// symmetricBands reports whether the lines of bands are symmetric about the
// middle of the line.
func symmetricBands(bands []float64) bool {
	for i, b := range bands {
		if math.Abs(b+bands[len(bands)-1-i]-1) > 1e-9 {
			return false
		}
	}
	return true
}

// sidedCompound strokes p as the lines of bands. A line is the set of the
// points between two distances from the path, on the left or on the right
// side. The part of a line on one side is the stroke of twice its far
// distance less the stroke of twice its near distance, clipped to that
// side. A line over the middle of the path draws the stroke of twice its
// smaller distance without a clip, so that the clips of its two parts meet
// inside it, without a seam.
func (l *Line) sidedCompound(dev display.Device, p *okage.Path, bands []float64, sh okage.Shader) {
	st := l.stroke()
	width := float64(st.Width)
	left, right, ok := pathSides(p, width)
	if !ok {
		dev.StrokePath(p, st, sh)
		return
	}
	// part draws the points from the distance near to the distance far
	// from the path, in the region s, or on both sides for no region
	part := func(near, far float64, s *side) {
		dev.Save()
		defer dev.Restore()
		if s != nil {
			dev.ClipPath(s.path, s.rule)
		}
		dev.BeginGroup(display.Group{Opacity: 1, Isolated: true})
		defer dev.EndGroup()
		st.Width = float32(2 * far)
		dev.StrokePath(p, st, sh)
		if near > 0 {
			dev.BeginGroup(display.Group{Opacity: 1, Blend: okage.BlendModeDestinationOut})
			st.Width = float32(2 * near)
			dev.StrokePath(p, st, okage.SolidColor(okage.ColorBlack))
			dev.EndGroup()
		}
	}
	for i := 0; i+1 < len(bands); i += 2 {
		// the distances of the line from the path, positive on the left
		hi, lo := width*(0.5-bands[i]), width*(0.5-bands[i+1])
		switch {
		case lo >= 0:
			part(lo, hi, &left)
		case hi <= 0:
			part(-hi, -lo, &right)
		default:
			core := min(hi, -lo)
			part(0, core, nil)
			if hi > core {
				part(0, hi, &left)
			}
			if -lo > core {
				part(0, -lo, &right)
			}
		}
	}
}

// side is a region of the user space: a path and its fill rule.
type side struct {
	path *okage.Path
	rule okage.FillRule
}

// pathSides returns the regions on the left and on the right of p, which
// hold the points within the distance d of p. A closed path splits the
// plane into its inside and its outside, and the way that it turns gives
// the side of each. The region on a side of an open path is the band of
// the width d along each of its segments, with its curves made flat. It
// returns false for no path, and for a path of more than one part with a
// closed one.
func pathSides(p *okage.Path, d float64) (left, right side, ok bool) {
	if len(p.Points()) < 2 || !(d > 0) || math.IsInf(d, 0) {
		return side{}, side{}, false
	}
	closed, moves := 0, 0
	for _, v := range p.Verbs() {
		switch v {
		case path.PathVerbClose:
			closed++
		case path.PathVerbMove:
			moves++
		}
	}
	if closed == 0 {
		l, r, ok := openSides(polylines(p), d)
		return side{l, okage.FillRuleWinding}, side{r, okage.FillRuleWinding}, ok
	}
	if moves != 1 {
		return side{}, side{}, false
	}
	b := p.Bounds()
	pad := float32(2 * d)
	big, ok := okage.RectFromLTRB(b.Left()-pad, b.Top()-pad, b.Right()+pad, b.Bottom()+pad)
	if !ok {
		return side{}, side{}, false
	}
	pb := okage.NewPathBuilder()
	pb.PushRect(big)
	pb.PushPath(p)
	outside, ok := pb.Finish()
	if !ok {
		return side{}, side{}, false
	}
	in, out := side{p, okage.FillRuleWinding}, side{outside, okage.FillRuleEvenOdd}
	// the outside of a path that turns clockwise, with y down, is on its
	// left
	if signedArea(polylines(p)) >= 0 {
		return out, in, true
	}
	return in, out, true
}

// curveSteps is the number of line segments of a curve that polylines makes.
const curveSteps = 16

// polylines returns the points of the parts of p, with each curve made of
// line segments.
func polylines(p *okage.Path) [][]okage.Point {
	var parts [][]okage.Point
	var cur []okage.Point
	pts := p.Points()
	i := 0
	for _, v := range p.Verbs() {
		switch v {
		case path.PathVerbMove:
			if len(cur) > 1 {
				parts = append(parts, cur)
			}
			if i >= len(pts) {
				return parts
			}
			cur = []okage.Point{pts[i]}
			i++
		case path.PathVerbLine:
			if i >= len(pts) || len(cur) == 0 {
				return parts
			}
			cur = append(cur, pts[i])
			i++
		case path.PathVerbQuad:
			if i+1 >= len(pts) || len(cur) == 0 {
				return parts
			}
			p0, p1, p2 := cur[len(cur)-1], pts[i], pts[i+1]
			for k := 1; k <= curveSteps; k++ {
				t := float32(k) / curveSteps
				u := 1 - t
				cur = append(cur, okage.Point{
					X: u*u*p0.X + 2*u*t*p1.X + t*t*p2.X,
					Y: u*u*p0.Y + 2*u*t*p1.Y + t*t*p2.Y,
				})
			}
			i += 2
		case path.PathVerbCubic:
			if i+2 >= len(pts) || len(cur) == 0 {
				return parts
			}
			p0, p1, p2, p3 := cur[len(cur)-1], pts[i], pts[i+1], pts[i+2]
			for k := 1; k <= curveSteps; k++ {
				t := float32(k) / curveSteps
				u := 1 - t
				cur = append(cur, okage.Point{
					X: u*u*u*p0.X + 3*u*u*t*p1.X + 3*u*t*t*p2.X + t*t*t*p3.X,
					Y: u*u*u*p0.Y + 3*u*u*t*p1.Y + 3*u*t*t*p2.Y + t*t*t*p3.Y,
				})
			}
			i += 3
		}
	}
	if len(cur) > 1 {
		parts = append(parts, cur)
	}
	return parts
}

// signedArea returns the area of the polygons of the parts, positive for a
// polygon that turns clockwise with y down.
func signedArea(parts [][]okage.Point) float64 {
	a := 0.0
	for _, pts := range parts {
		for i, p := range pts {
			q := pts[(i+1)%len(pts)]
			a += float64(p.X)*float64(q.Y) - float64(q.X)*float64(p.Y)
		}
	}
	return a / 2
}

// openSides returns the regions on the left and on the right of the open
// parts: the band of the width d along each segment, and a wedge at each
// turn, which fills the gap between the bands on the outer side of the
// turn.
func openSides(parts [][]okage.Point, d float64) (*okage.Path, *okage.Path, bool) {
	lb, rb := okage.NewPathBuilder(), okage.NewPathBuilder()
	n := 0
	for _, pts := range parts {
		var px, py float64
		turn := false
		for i := 0; i+1 < len(pts); i++ {
			a, b := pts[i], pts[i+1]
			dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
			length := math.Hypot(dx, dy)
			if !(length > 1e-6) || math.IsInf(length, 0) {
				continue
			}
			// the normal on the left of the direction, with y down
			nx, ny := dy/length*d, -dx/length*d
			polygon(lb, a, b, okage.Point{X: b.X + float32(nx), Y: b.Y + float32(ny)}, okage.Point{X: a.X + float32(nx), Y: a.Y + float32(ny)})
			polygon(rb, a, b, okage.Point{X: b.X - float32(nx), Y: b.Y - float32(ny)}, okage.Point{X: a.X - float32(nx), Y: a.Y - float32(ny)})
			if turn {
				wedge(lb, a, px, py, nx, ny)
				wedge(rb, a, -px, -py, -nx, -ny)
			}
			px, py, turn = nx, ny, true
			n++
		}
	}
	if n == 0 {
		return nil, nil, false
	}
	l, ok1 := lb.Finish()
	r, ok2 := rb.Finish()
	return l, r, ok1 && ok2
}

// polygon adds the polygon of the points to pb, turned clockwise with y
// down, so that the polygons of a region add up with the winding rule.
func polygon(pb *okage.PathBuilder, pts ...okage.Point) {
	if signedArea([][]okage.Point{pts}) < 0 {
		for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
			pts[i], pts[j] = pts[j], pts[i]
		}
	}
	pb.MoveTo(pts[0].X, pts[0].Y)
	for _, p := range pts[1:] {
		pb.LineTo(p.X, p.Y)
	}
	pb.Close()
}

// wedge adds to pb the wedge at the point v between the normals a and b of
// two segments, out to twice their length.
func wedge(pb *okage.PathBuilder, v okage.Point, ax, ay, bx, by float64) {
	mx, my := ax+bx, ay+by
	m := math.Hypot(mx, my)
	if !(m > 1e-9) {
		return
	}
	l := 2 * math.Hypot(ax, ay)
	mx, my = mx/m*l, my/m*l
	polygon(pb, v,
		okage.Point{X: v.X + float32(ax), Y: v.Y + float32(ay)},
		okage.Point{X: v.X + float32(mx), Y: v.Y + float32(my)},
		okage.Point{X: v.X + float32(bx), Y: v.Y + float32(by)})
}
