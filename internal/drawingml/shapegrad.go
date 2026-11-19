package drawingml

import (
	"math"
	"slices"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// The bounds of the mesh of a gradient of the path "shape": the most edges
// of the flat outline, the most segments of a curve, and the most rays of a
// sector of a focus out of the shape.
const (
	maxShapeEdges   = 1024
	maxCurveSteps   = 64
	maxSectorSteps  = 8
	shapeOuterScale = 1.5
)

// vec is a point of the flat outline of a shape.
type vec struct{ x, y float64 }

func (a vec) sub(b vec) vec       { return vec{a.x - b.x, a.y - b.y} }
func (a vec) add(b vec) vec       { return vec{a.x + b.x, a.y + b.y} }
func (a vec) scale(s float64) vec { return vec{a.x * s, a.y * s} }
func (a vec) cross(b vec) float64 { return a.x*b.y - a.y*b.x }
func (a vec) lerp(b vec, s float64) vec {
	return vec{a.x + (b.x-a.x)*s, a.y + (b.y-a.y)*s}
}

// shapeShader returns the shader of the gradient of the path "shape" in the
// outline p, over the box, as PowerPoint draws it. scale is the number of
// pixels of the device for a unit of the user space.
//
// The color of a point is its place on the ray from the focus to the
// outline: the first stop at the focus, and the last one on the outline. So
// the lines of one color are copies of the outline, scaled about the focus.
// A focus out of the shape, such as a corner of the box of a triangle, puts
// the first stop where the ray enters the shape, and the last one where it
// leaves it. This is closer to PowerPoint than the gradient of the path
// "circle" about the focus. A shape whose box has its center out of the
// shape or on its outline, such as a moon, takes the gradient of the path
// "circle" about the center of the box, as PowerPoint draws it.
//
// The mesh has a sector of triangles for each part of the outline between
// two corners, as the focus sees them. In a sector, the place on the ray is
// linear in the point when the focus is in the shape, so the mesh is exact.
// A focus out of the shape divides each sector into more rays. It returns
// false when the outline gives no mesh.
func (g *Gradient) shapeShader(p *okage.Path, box okage.Rect, scale float64) (okage.Shader, bool) {
	polys := flatten(p, scale)
	edges := 0
	for _, poly := range polys {
		edges += len(poly)
	}
	if edges < 3 || edges > maxShapeEdges {
		return nil, false
	}
	x, y := float64(box.X()), float64(box.Y())
	w, h := max(float64(box.Width()), 1e-3), max(float64(box.Height()), 1e-3)
	center := vec{x + w/2, y + h/2}
	if !inside(polys, center) || edgeDistance(polys, center) < 0.01*min(w, h) {
		c := *g
		c.Path = "circle"
		c.Focus = Insets{L: 0.5, T: 0.5, R: 0.5, B: 0.5}
		return c.shader(box)
	}
	f := g.Focus
	focus := vec{x + (unit(f.L)+1-unit(f.R))/2*w, y + (unit(f.T)+1-unit(f.B))/2*h}
	stops := g.Stops
	if stops[0].Pos > 0 {
		stops = append([]Stop{{Pos: 0, Color: stops[0].Color}}, stops...)
	}
	if last := stops[len(stops)-1]; last.Pos < 1 {
		stops = append(stops[:len(stops):len(stops)], Stop{Pos: 1, Color: last.Color})
	}
	in := inside(polys, focus)
	// the angles of the corners as the focus sees them, in order
	angles := make([]float64, 0, edges+1)
	for _, poly := range polys {
		for _, v := range poly {
			d := v.sub(focus)
			if d.x != 0 || d.y != 0 {
				angles = append(angles, math.Atan2(d.y, d.x))
			}
		}
	}
	slices.Sort(angles)
	angles = slices.CompactFunc(angles, func(a, b float64) bool { return math.Abs(b-a) < 1e-9 })
	if len(angles) < 2 {
		return nil, false
	}
	angles = append(angles, angles[0]+2*math.Pi)
	first, last := stops[0].Color, stops[len(stops)-1].Color
	vertex := func(p vec, c RGBA) okage.MeshVertex {
		return okage.MeshVertex{Point: okage.PointFromXY(float32(p.x), float32(p.y)), Color: c.Okage()}
	}
	quad := func(tris []okage.MeshTriangle, a0, b0, a1, b1 vec, c0, c1 RGBA) []okage.MeshTriangle {
		return append(tris,
			okage.MeshTriangle{vertex(a0, c0), vertex(b0, c0), vertex(b1, c1)},
			okage.MeshTriangle{vertex(a0, c0), vertex(b1, c1), vertex(a1, c1)},
		)
	}
	// the bands out of the outline and before the entry take the colors of
	// the ends, under the bands of the gradient, so that the curves of the
	// outline between the points of the flat outline have a color
	var outer, bands []okage.MeshTriangle
	for i := range len(angles) - 1 {
		a0, a1 := angles[i], angles[i+1]
		if a1-a0 < 1e-9 {
			continue
		}
		near, far, ok := crossings(polys, focus, (a0+a1)/2)
		if !ok {
			continue
		}
		if in {
			near = [2]vec{}
		}
		steps := 1
		if !in {
			steps = int(min(maxSectorSteps, math.Ceil((a1-a0)/(2*math.Pi/180))))
		}
		var qn, qf []vec
		for j := range steps + 1 {
			a := a0 + (a1-a0)*float64(j)/float64(steps)
			dir := vec{math.Cos(a), math.Sin(a)}
			tf, okf := rayLine(focus, dir, far)
			tn := 0.0
			okn := true
			if !in {
				tn, okn = rayLine(focus, dir, near)
			}
			if !okf || !okn || tf < tn {
				qn, qf = nil, nil
				break
			}
			qn = append(qn, focus.add(dir.scale(tn)))
			qf = append(qf, focus.add(dir.scale(tf)))
		}
		for j := 0; j+1 < len(qf); j++ {
			o0 := focus.add(qf[j].sub(focus).scale(shapeOuterScale))
			o1 := focus.add(qf[j+1].sub(focus).scale(shapeOuterScale))
			outer = quad(outer, qf[j], qf[j+1], o0, o1, last, last)
			if !in {
				outer = append(outer, okage.MeshTriangle{vertex(focus, first), vertex(qn[j], first), vertex(qn[j+1], first)})
			}
			for k := range len(stops) - 1 {
				s0, s1 := stops[k], stops[k+1]
				bands = quad(bands,
					qn[j].lerp(qf[j], s0.Pos), qn[j+1].lerp(qf[j+1], s0.Pos),
					qn[j].lerp(qf[j], s1.Pos), qn[j+1].lerp(qf[j+1], s1.Pos),
					s0.Color, s1.Color)
			}
		}
	}
	return okage.NewMeshGradient(append(outer, bands...), okage.IdentityTransform())
}

// flatten returns the subpaths of p as polygons, with each curve cut into
// segments of about 4 pixels at scale pixels for a unit.
func flatten(p *okage.Path, scale float64) [][]vec {
	if p == nil {
		return nil
	}
	if !(scale > 0) || math.IsInf(scale, 0) {
		scale = 1
	}
	var polys [][]vec
	var poly []vec
	end := func() {
		if len(poly) >= 3 {
			polys = append(polys, poly)
		}
		poly = nil
	}
	pts := p.Points()
	at := func(i int) vec { return vec{float64(pts[i].X), float64(pts[i].Y)} }
	cur := vec{}
	i := 0
	curve := func(ctrl []vec) {
		length := 0.0
		prev := cur
		for _, c := range ctrl {
			length += math.Hypot(c.x-prev.x, c.y-prev.y)
			prev = c
		}
		n := int(min(maxCurveSteps, max(1, math.Ceil(length*scale/4))))
		p0 := cur
		for k := 1; k <= n; k++ {
			t := float64(k) / float64(n)
			var q vec
			if len(ctrl) == 2 {
				a, b := p0.lerp(ctrl[0], t), ctrl[0].lerp(ctrl[1], t)
				q = a.lerp(b, t)
			} else {
				a, b, c := p0.lerp(ctrl[0], t), ctrl[0].lerp(ctrl[1], t), ctrl[1].lerp(ctrl[2], t)
				q = a.lerp(b, t).lerp(b.lerp(c, t), t)
			}
			poly = append(poly, q)
		}
		cur = ctrl[len(ctrl)-1]
	}
	for _, v := range p.Verbs() {
		switch v {
		case path.PathVerbMove:
			if i >= len(pts) {
				return polys
			}
			end()
			cur = at(i)
			poly = append(poly, cur)
			i++
		case path.PathVerbLine:
			if i >= len(pts) {
				return polys
			}
			cur = at(i)
			poly = append(poly, cur)
			i++
		case path.PathVerbQuad:
			if i+2 > len(pts) {
				return polys
			}
			curve([]vec{at(i), at(i + 1)})
			i += 2
		case path.PathVerbCubic:
			if i+3 > len(pts) {
				return polys
			}
			curve([]vec{at(i), at(i + 1), at(i + 2)})
			i += 3
		case path.PathVerbClose:
			end()
		}
		if len(poly) > maxShapeEdges {
			return nil
		}
	}
	end()
	return polys
}

// inside reports whether the point p is in the polygons, by the rule even
// odd.
func inside(polys [][]vec, p vec) bool {
	in := false
	for _, poly := range polys {
		for i := range poly {
			a, b := poly[i], poly[(i+1)%len(poly)]
			if (a.y > p.y) != (b.y > p.y) && p.x < a.x+(p.y-a.y)/(b.y-a.y)*(b.x-a.x) {
				in = !in
			}
		}
	}
	return in
}

// crossings returns the nearest and the farthest edge of the polygons that
// the ray from p at the angle a crosses. It returns false when the ray
// crosses no edge.
func crossings(polys [][]vec, p vec, a float64) (near, far [2]vec, ok bool) {
	dir := vec{math.Cos(a), math.Sin(a)}
	tn, tf := math.Inf(1), math.Inf(-1)
	for _, poly := range polys {
		for i := range poly {
			e := [2]vec{poly[i], poly[(i+1)%len(poly)]}
			d := e[1].sub(e[0])
			den := dir.cross(d)
			if math.Abs(den) < 1e-12 {
				continue
			}
			w := e[0].sub(p)
			t := w.cross(d) / den
			u := w.cross(dir) / den
			if t <= 0 || u < 0 || u > 1 {
				continue
			}
			if t < tn {
				tn, near = t, e
			}
			if t > tf {
				tf, far = t, e
			}
		}
	}
	return near, far, tf > 0
}

// rayLine returns the distance from p along the unit vector dir to the line
// through the edge e. It returns false when the ray does not meet the line.
func rayLine(p, dir vec, e [2]vec) (float64, bool) {
	d := e[1].sub(e[0])
	den := dir.cross(d)
	if math.Abs(den) < 1e-12 {
		return 0, false
	}
	t := e[0].sub(p).cross(d) / den
	return t, t >= 0 && !math.IsInf(t, 0) && !math.IsNaN(t)
}

// edgeDistance returns the distance from the point p to the nearest edge of
// the polygons.
func edgeDistance(polys [][]vec, p vec) float64 {
	best := math.Inf(1)
	for _, poly := range polys {
		for i := range poly {
			a, b := poly[i], poly[(i+1)%len(poly)]
			d := b.sub(a)
			t := 0.0
			if l := d.x*d.x + d.y*d.y; l > 0 {
				t = min(1, max(0, (p.sub(a).x*d.x+p.sub(a).y*d.y)/l))
			}
			q := a.lerp(b, t)
			best = min(best, math.Hypot(p.x-q.x, p.y-q.y))
		}
	}
	return best
}
