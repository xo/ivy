package chart

import (
	"image/color"
	"math"
	"slices"

	"github.com/xo/okage"
)

// This file draws a pie and a doughnut, and the radar chart. The angles
// are in radians, clockwise from the top.

// pieInset is the gap between the plot area and a pie, in points.
const pieInset = 3.5

// arcTo adds to pb an arc of the circle of the center (cx, cy) and the
// radius r, from the angle a0 to a1, as cubic curves of a quarter turn at
// most. The path is at the point of a0.
func arcTo(pb *okage.PathBuilder, cx, cy, r, a0, a1 float64) {
	n := max(1, int(math.Ceil(math.Abs(a1-a0)/(math.Pi/2)-1e-9)))
	d := (a1 - a0) / float64(n)
	k := 4.0 / 3 * math.Tan(d/4)
	for i := range n {
		t0 := a0 + float64(i)*d - math.Pi/2
		t1 := t0 + d
		x0, y0 := math.Cos(t0), math.Sin(t0)
		x1, y1 := math.Cos(t1), math.Sin(t1)
		pb.CubicTo(float32(cx+r*(x0-k*y0)), float32(cy+r*(y0+k*x0)),
			float32(cx+r*(x1+k*y1)), float32(cy+r*(y1-k*x1)),
			float32(cx+r*x1), float32(cy+r*y1))
	}
}

// polar returns the point at the angle a and the radius r of the center
// (cx, cy).
func polar(cx, cy, r, a float64) (float64, float64) {
	return cx + r*math.Sin(a), cy - r*math.Cos(a)
}

// slicePath returns the path of a slice of a pie, or of a ring with an
// inner radius, from the angle a0 to a1.
func slicePath(cx, cy, r, inner, a0, a1 float64) *okage.Path {
	pb := okage.NewPathBuilder()
	x, y := polar(cx, cy, r, a0)
	full := math.Abs(a1-a0) >= 2*math.Pi-1e-9
	if inner > 0 || full {
		pb.MoveTo(float32(x), float32(y))
	} else {
		pb.MoveTo(float32(cx), float32(cy))
		pb.LineTo(float32(x), float32(y))
	}
	arcTo(pb, cx, cy, r, a0, a1)
	if inner > 0 {
		ix, iy := polar(cx, cy, inner, a1)
		if full {
			pb.Close()
			pb.MoveTo(float32(ix), float32(iy))
		} else {
			pb.LineTo(float32(ix), float32(iy))
		}
		arcTo(pb, cx, cy, inner, a1, a0)
	}
	pb.Close()
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// pieLabel is a label on a slice of a pie.
type pieLabel struct {
	b      block
	w, h   float64
	mid    float64
	inside bool
	x, y   float64
	// edge is the point of the slice that a leader line starts from.
	ex, ey float64
	moved  bool
	// fixed says that the file gives the place of the label.
	fixed bool
}

// pie draws a pie or a doughnut into the box b. A doughnut draws each
// series as a ring, the first inside.
func (cv *canvas) pie(b box, inner bool) {
	c := cv.c
	p := c.plots[0]
	cats := c.categories()
	if len(p.series) == 0 {
		cv.frame(b)
		return
	}
	if !inner {
		b = box{b.x0 + pieInset, b.y0 + pieInset, b.x1 - pieInset, b.y1 - pieInset}
	}
	rings := p.series
	if p.hole == 0 {
		rings = rings[:1]
	}
	first := rings[0]
	cx, cy := (b.x0+b.x1)/2, (b.y0+b.y1)/2
	// A 3D pie draws flat, as an ellipse of the ratio k.
	k := 1.0
	if p.flat3D {
		k = max(0.2, min(1, math.Sin(c.rotX*math.Pi/180)))
		if inner {
			k = max(0.2, min(1, b.h()/b.w()))
		}
	}
	r0 := min(b.w(), b.h()/k) / 2
	if !(r0 > 1) {
		return
	}
	explode := first.explode
	for _, e := range first.explodes {
		explode = max(explode, e)
	}
	if p.hole > 0 {
		explode = 0
	}
	dir := 1.0
	if p.ccw {
		dir = -1
	}
	start := p.firstAngle * math.Pi / 180
	vary := p.varyColors
	// The labels of a pie can go outside, and make the pie smaller, unless
	// the file gives the box of the pie.
	r := r0 / (1 + explode)
	var tags []pieLabel
	if p.hole == 0 {
		for range 2 {
			tags = cv.pieLabels(p, first, cx, cy, r, start, dir, k)
			if inner {
				break
			}
			outside := false
			mw, mh := 0.0, 0.0
			for _, l := range tags {
				if !l.inside {
					outside = true
					mw, mh = max(mw, l.w), max(mh, l.h)
				}
			}
			if !outside {
				break
			}
			gap := c.text.size * 0.6
			r = max(min(b.w()/2-mw-gap, (b.h()/2-mh-gap)/k)/(1+explode), r0*0.4)
		}
	}
	width := r
	if p.hole > 0 {
		width = r * (1 - p.hole) / float64(len(rings))
	}
	cv.dev.Save()
	if k != 1 {
		cv.dev.Concat(okage.TransformFromRow(1, 0, 0, float32(k), 0, float32(cy*(1-k))))
	}
	for ring, s := range rings {
		total := 0.0
		for _, v := range s.vals {
			if !math.IsNaN(v) {
				total += math.Abs(v)
			}
		}
		if total == 0 {
			continue
		}
		outer := r
		in := 0.0
		if p.hole > 0 {
			in = r*p.hole + float64(ring)*width
			outer = in + width
		}
		a := start
		lab := labelsOf(p, s)
		for i, v := range s.vals {
			if math.IsNaN(v) || v == 0 {
				continue
			}
			d := math.Abs(v) / total * 2 * math.Pi
			a0, a1 := a, a+dir*d
			a = a1
			x, y := cx, cy
			if e := explodeOf(s, i); e > 0 && p.hole == 0 {
				x, y = polar(cx, cy, r*e, (a0+a1)/2)
			}
			path := slicePath(x, y, outer, in, min(a0, a1), max(a0, a1))
			col := c.pointColor(s, i, vary)
			if !s.noFill {
				cv.fill(path, col)
			}
			line := cv.outline(s)
			if line.A == 0 && c.look == lookModern {
				line = color.NRGBA{255, 255, 255, 255}
			}
			cv.stroke(path, line, outlineWidth, "", true)
			if p.hole == 0 {
				continue
			}
			t, ok := tagOf(lab, i, func(l labels) string {
				return labelText(l, seriesName(s), catOf(cats, s, i), v, math.Abs(v)/total, s.format)
			})
			if ok {
				mx, my := polar(x, y, (in+outer)/2, (a0+a1)/2)
				cv.t.draw(cv.dev, block{lines: t.lines, st: t.st}, mx+t.dx*cv.w, my+t.dy*cv.h, alignCenter, alignCenter, 0)
			}
		}
	}
	cv.dev.Restore()
	for _, l := range tags {
		if l.moved {
			cv.line(l.ex, l.ey, l.x, l.y, c.defaultTextColor(), 0.5)
		}
		ha := alignCenter
		if !l.inside {
			ha = alignStart
			if math.Sin(l.mid) < 0 {
				ha = alignEnd
			}
		}
		cv.t.draw(cv.dev, l.b, l.x, l.y, ha, alignCenter, 0)
	}
}

// explodeOf returns the explosion of the point i of a series.
func explodeOf(s *series, i int) float64 {
	if e, ok := s.explodes[i]; ok {
		return e
	}
	return s.explode
}

// catOf returns the name of the category i of a series, or of the
// categories of the chart.
func catOf(cats []string, s *series, i int) string {
	if i < len(s.cats) {
		return s.cats[i]
	}
	if i < len(cats) {
		return cats[i]
	}
	return ""
}

// pieLabels lays out the labels of the slices of a pie of the radius r: in
// a slice when they fit there, or when the file puts them there, else
// outside of it. The labels outside go down on each side, so that they do
// not overlap.
// The pie is an ellipse whose height is k times its width.
func (cv *canvas) pieLabels(p *plot, s *series, cx, cy, r, start, dir, k float64) []pieLabel {
	cats := cv.c.categories()
	lab := labelsOf(p, s)
	if !lab.any() {
		return nil
	}
	total := 0.0
	for _, v := range s.vals {
		if !math.IsNaN(v) {
			total += math.Abs(v)
		}
	}
	if total == 0 {
		return nil
	}
	maxW := max(cv.w*0.2, 40)
	var out []pieLabel
	a := start
	for i, v := range s.vals {
		if math.IsNaN(v) || v == 0 {
			continue
		}
		d := math.Abs(v) / total * 2 * math.Pi
		a0, a1 := a, a+dir*d
		a = a1
		mid := (a0 + a1) / 2
		t, ok := tagOf(lab, i, func(l labels) string {
			return labelText(l, seriesName(s), catOf(cats, s, i), v, math.Abs(v)/total, s.format)
		})
		if !ok {
			continue
		}
		pos := t.pos
		if pos == "" {
			pos = "bestFit"
		}
		var lines []string
		for _, l := range t.lines {
			lines = append(lines, cv.t.wrap(l, t.st, maxW)...)
		}
		bl := block{lines: lines, st: t.st}
		w, h := cv.t.size(bl)
		e := explodeOf(s, i)
		ox, oy := polar(cx, cy, r*e, mid)
		l := pieLabel{b: bl, w: w, h: h, mid: mid}
		switch pos {
		case "ctr":
			l.inside = true
			l.x, l.y = polar(ox, oy, r*0.5, mid)
		case "inEnd":
			l.inside = true
			l.x, l.y = polar(ox, oy, r*0.75, mid)
		case "outEnd":
		default:
			l.x, l.y = polar(ox, oy, r*0.62, mid)
			l.inside = fits(l.x-ox, l.y-oy, w, h, r, min(a0, a1), max(a0, a1))
		}
		if !l.inside {
			gap := t.st.size * 0.4
			l.ex, l.ey = polar(ox, oy, r, mid)
			l.x, l.y = polar(ox, oy, r+gap, mid)
			l.ey = cy + (l.ey-cy)*k
			l.y = cy + (l.y-cy)*k - h/2*math.Cos(mid)
		} else {
			l.y = cy + (l.y-cy)*k
		}
		if t.dx != 0 || t.dy != 0 {
			l.x += t.dx * cv.w
			l.y += t.dy * cv.h
			l.fixed = true
			l.ex, l.ey = polar(ox, oy, r, mid)
			l.ey = cy + (l.ey-cy)*k
			l.moved = math.Hypot(l.x-l.ex, l.y-l.ey) > r*0.6
		}
		out = append(out, l)
	}
	spread(out, cv.h-pad)
	return out
}

// fits says whether a box of w by h centered at (x, y) from the center of
// a pie of the radius r is inside the slice from the angle a0 to a1.
func fits(x, y, w, h, r, a0, a1 float64) bool {
	for _, d := range [][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
		px, py := x+d[0]*w/2, y+d[1]*h/2
		if math.Hypot(px, py) > r {
			return false
		}
		a := math.Atan2(px, -py)
		for a < a0 {
			a += 2 * math.Pi
		}
		for a > a0+2*math.Pi {
			a -= 2 * math.Pi
		}
		if a > a1 {
			return false
		}
	}
	return true
}

// spread moves the labels outside of a pie down on each side, so that
// they do not overlap, and up again when they pass the bottom.
func spread(labels []pieLabel, bottom float64) {
	for _, right := range []bool{true, false} {
		var idx []int
		for i, l := range labels {
			if !l.inside && !l.fixed && (math.Sin(l.mid) >= 0) == right {
				idx = append(idx, i)
			}
		}
		slices.SortFunc(idx, func(a, b int) int {
			switch {
			case labels[a].y < labels[b].y:
				return -1
			case labels[a].y > labels[b].y:
				return 1
			}
			return 0
		})
		for j := 1; j < len(idx); j++ {
			prev, cur := &labels[idx[j-1]], &labels[idx[j]]
			if minY := prev.y + (prev.h+cur.h)/2; cur.y < minY {
				cur.y = minY
				cur.moved = true
			}
		}
		for j := len(idx) - 1; j >= 0; j-- {
			cur := &labels[idx[j]]
			limit := bottom
			if j < len(idx)-1 {
				next := labels[idx[j+1]]
				limit = next.y - next.h/2
			}
			if over := cur.y + cur.h/2 - limit; over > 0 {
				cur.y -= over
				cur.moved = true
			}
		}
	}
}

// radar draws a radar chart into the box b: a spoke for each category
// from the top clockwise, a polygon for each major value, and the series as
// closed lines, or as filled areas.
func (cv *canvas) radar(b box) {
	c := cv.c
	t := cv.t
	cats := c.categories()
	n := len(cats)
	if n == 0 {
		cv.frame(b)
		return
	}
	st := c.cat.text
	lh := st.size * lineHeight
	mw := 0.0
	if c.cat.shown() {
		for _, s := range cats {
			mw = max(mw, t.width(s, st))
		}
	}
	cx, cy := (b.x0+b.x1)/2, (b.y0+b.y1)/2
	r := max(min(b.w()/2-mw-gapLabel*2, b.h()/2-lh-gapLabel*2), min(b.w(), b.h())/8)
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range c.plots {
		for _, s := range p.series {
			for _, v := range s.vals {
				if !math.IsNaN(v) {
					lo, hi = min(lo, v), max(hi, v)
				}
			}
		}
	}
	if math.IsInf(lo, 0) {
		lo, hi = 0, 1
	}
	vs := autoScale(lo, hi, c.val, intervals(r, c.val.text, false, 0, c.look == lookLibre))
	vs.reverse = false
	angle := func(i int) float64 { return 2 * math.Pi * float64(i) / float64(n) }
	grid := libreGray
	switch c.look {
	case lookModern:
		grid = gridGray
	case lookOffice:
		grid = officeGray
	}
	if c.val.hasGridColor {
		grid = c.val.gridColor
	}
	ticks := vs.ticks(false)
	if c.val.grid || !c.val.present {
		for _, v := range ticks {
			rr := r * vs.frac(v)
			pts := make([][2]float64, 0, n+1)
			for i := range n + 1 {
				x, y := polar(cx, cy, rr, angle(i%n))
				pts = append(pts, [2]float64{x, y})
			}
			cv.stroke(polyline(pts), grid, thin, "", false)
		}
	}
	spoke := grid
	if c.cat.hasLine {
		spoke = c.cat.line
	}
	if !c.cat.noLine {
		for i := range n {
			x, y := polar(cx, cy, r, angle(i))
			cv.line(cx, cy, x, y, spoke, thin)
		}
	}
	if c.val.shown() {
		_, texts := valueTexts(vs, c.val, "General")
		for i, v := range ticks {
			if i < len(texts) {
				t.draw(cv.dev, block{lines: []string{texts[i]}, st: c.val.text}, cx-gapLabel, cy-r*vs.frac(v),
					alignEnd, alignCenter, 0)
			}
		}
	}
	if c.cat.shown() {
		for i, name := range cats {
			a := angle(i)
			x, y := polar(cx, cy, r+gapLabel*2, a)
			ha, va := alignCenter, alignCenter
			switch s := math.Sin(a); {
			case s > 0.1:
				ha = alignStart
			case s < -0.1:
				ha = alignEnd
			}
			switch k := math.Cos(a); {
			case k > 0.1:
				va = alignEnd
			case k < -0.1:
				va = alignStart
			}
			t.draw(cv.dev, block{lines: []string{name}, st: st}, x, y, ha, va, 0)
		}
	}
	for _, p := range c.plots {
		if p.kind != kindRadar {
			continue
		}
		for _, s := range p.series {
			pb := okage.NewPathBuilder()
			started := false
			var pts [][2]float64
			for i := range n {
				v := value(s, i)
				if math.IsNaN(v) {
					v = vs.min
				}
				x, y := polar(cx, cy, r*max(0, vs.frac(v)), angle(i))
				pts = append(pts, [2]float64{x, y})
				if !started {
					pb.MoveTo(float32(x), float32(y))
					started = true
				} else {
					pb.LineTo(float32(x), float32(y))
				}
			}
			pb.Close()
			path, ok := pb.Finish()
			if !ok {
				continue
			}
			if p.filled {
				if !s.noFill {
					cv.fill(path, s.color)
				}
				if line := cv.outline(s); line.A > 0 {
					cv.stroke(path, line, outlineWidth, s.dash, true)
				}
				continue
			}
			if !s.noLine {
				width := s.lineWidth
				if !(width > 0) {
					width = lineWidth
				}
				cv.stroke(path, s.color, width, s.dash, true)
			}
			marker := cv.markerOf(p, s)
			for _, pt := range pts {
				cv.marker(marker, pt[0], pt[1], cv.markerSize(p, s), s, s.color)
			}
		}
	}
}
