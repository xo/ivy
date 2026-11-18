// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the tests of the triangles of a mesh for okage, which
// upstream does not have.

package interpret

import (
	"math"
	"testing"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
)

// triangleArea returns the area of the triangle of the points.
func triangleArea(p [3]kurbo.Point) float64 {
	return math.Abs((p[1].X-p[0].X)*(p[2].Y-p[0].Y)-(p[2].X-p[0].X)*(p[1].Y-p[0].Y)) / 2
}

// newTestEmitter returns a meshEmitter over the area that collects its
// triangles.
func newTestEmitter(area kurbo.Rect, got *[]MeshTriangle) *meshEmitter {
	return &meshEmitter{
		area:  area,
		limit: kurboext.RectInflate(area, meshClipMargin, meshClipMargin),
		emit:  func(t MeshTriangle) { *got = append(*got, t) },
	}
}

// TestCutParameter cuts a triangle at the stops of a function with a jump
// at 0.5. The parts cover the triangle, no part crosses the jump, and the
// color of each part is the color of the stops at its center.
func TestCutParameter(t *testing.T) {
	red, green := [4]float32{1, 0, 0, 1}, [4]float32{0, 1, 0, 1}
	blue, white := [4]float32{0, 0, 1, 1}, [4]float32{1, 1, 1, 1}
	stops := []SVGGradientStop{{0, red}, {0.5, green}, {0.5, blue}, {1, white}}
	p := &parameter{lut: newColorLut(stops), stops: stops, scale: 1}
	var got []MeshTriangle
	e := newTestEmitter(kurbo.NewRect(0, 0, 100, 100), &got)
	points := [3]kurbo.Point{kurbo.NewPoint(0, 0), kurbo.NewPoint(100, 0), kurbo.NewPoint(0, 100)}
	e.fillPiece(points, [3][]float32{{0}, {1}, {0.5}}, p)
	if len(got) < 2 {
		t.Fatalf("got %d triangles, want at least 2", len(got))
	}
	// value is the value of the function at a point of the triangle.
	value := func(q kurbo.Point) float64 { return q.X/100 + q.Y/200 }
	// want is the color of the stops at v, on the side of the jump of side.
	want := func(v float64, low bool) [4]float32 {
		if low {
			return lerpColor(red, green, float32(v/0.5))
		}
		return lerpColor(blue, white, float32((v-0.5)/0.5))
	}
	var area float64
	for _, tri := range got {
		area += triangleArea(tri.Points)
		center := kurbo.NewPoint(
			(tri.Points[0].X+tri.Points[1].X+tri.Points[2].X)/3,
			(tri.Points[0].Y+tri.Points[1].Y+tri.Points[2].Y)/3)
		low := value(center) < 0.5
		for i, q := range tri.Points {
			v := value(q)
			if low && v > 0.5+1e-9 || !low && v < 0.5-1e-9 {
				t.Errorf("triangle %v crosses the jump at 0.5", tri.Points)
			}
			w := want(v, low)
			for k := range w {
				if math.Abs(float64(tri.Colors[i][k]-w[k])) > 1e-5 {
					t.Errorf("at %v (value %g): got %v, want %v", q, v, tri.Colors[i], w)
					break
				}
			}
		}
	}
	if math.Abs(area-5000) > 1e-6 {
		t.Errorf("the triangles cover %g, want 5000", area)
	}
}

// TestMeshClip cuts a huge triangle to the limit of the emitter. The colors
// at the new corners follow the colors of the triangle.
func TestMeshClip(t *testing.T) {
	var got []MeshTriangle
	e := newTestEmitter(kurbo.NewRect(0, 0, 100, 100), &got)
	points := [3]kurbo.Point{kurbo.NewPoint(-1e12, -1e12), kurbo.NewPoint(1e12, -1e12), kurbo.NewPoint(0, 1e12)}
	// The red grows with x, and the other channels do not change.
	colors := [3][4]float32{{0, 0, 0, 1}, {1, 0, 0, 1}, {0.5, 0, 0, 1}}
	e.add(points, colors)
	if len(got) == 0 {
		t.Fatal("got no triangle")
	}
	l := e.limit
	for _, tri := range got {
		for i, q := range tri.Points {
			if q.X < l.X0-1e-6 || q.X > l.X1+1e-6 || q.Y < l.Y0-1e-6 || q.Y > l.Y1+1e-6 {
				t.Errorf("corner %v is out of the limit %v", q, l)
			}
			if r := (q.X + 1e12) / 2e12; math.Abs(float64(tri.Colors[i][0])-r) > 1e-6 {
				t.Errorf("at %v: got red %g, want %g", q, tri.Colors[i][0], r)
			}
		}
	}
}

// TestMeshTrianglesLimit stops the emitter at maxMeshTriangles triangles.
func TestMeshTrianglesLimit(t *testing.T) {
	var got []MeshTriangle
	e := newTestEmitter(kurbo.NewRect(0, 0, 100, 100), &got)
	points := [3]kurbo.Point{kurbo.NewPoint(0, 0), kurbo.NewPoint(10, 0), kurbo.NewPoint(0, 10)}
	for range maxMeshTriangles + 10 {
		e.add(points, [3][4]float32{})
	}
	if len(got) != maxMeshTriangles || !e.stopped() {
		t.Errorf("got %d triangles and stopped %t, want %d and true", len(got), e.stopped(), maxMeshTriangles)
	}
}

// TestMeshTrianglesBackground gives the triangles of a pattern of a
// triangle mesh with a background. The first two triangles cover the area
// with the background, and the triangle of the mesh comes after them.
func TestMeshTrianglesBackground(t *testing.T) {
	// One red triangle, with a flag, x, y and RGB in a byte each.
	data := "\x00\x28\x28\xff\x00\x00\x00\x3c\x28\xff\x00\x00\x00\x32\x3c\xff\x00\x00"
	shading := "<< /ShadingType 4 /ColorSpace /DeviceRGB /Background [0 1 0] /BitsPerCoordinate 8 " +
		"/BitsPerComponent 8 /BitsPerFlag 8 /Decode [0 255 0 255 0 1 0 1 0 1] /Length " + itoa(len(data)) +
		" >>\nstream\n" + data + "\nendstream"
	objects := []string{"<< /Pattern << /P 6 0 R >> >>", "<< /PatternType 2 /Shading 7 0 R >>", shading}
	rec := &recorder{t: t}
	errs, err := interpretPages(selfPdf("/Pattern cs /P scn 0 0 100 100 re f", objects), rec, DefaultInterpreterSettings(), 1)
	if err != nil || len(errs) != 1 || errs[0] != nil {
		t.Fatalf("got %v and %v", errs, err)
	}
	if len(rec.shadings) != 1 {
		t.Fatalf("got %d shading patterns, want 1", len(rec.shadings))
	}
	pattern := rec.shadings[0].Encode()
	area := kurbo.NewRect(0, 0, 100, 100)
	var got []MeshTriangle
	if !pattern.MeshTriangles(area, func(tri MeshTriangle) { got = append(got, tri) }) {
		t.Fatal("the shading is not a mesh")
	}
	if len(got) != 3 {
		t.Fatalf("got %d triangles, want the background and the mesh", len(got))
	}
	green, red := [4]float32{0, 1, 0, 1}, [4]float32{1, 0, 0, 1}
	for i, tri := range got {
		want := green
		if i == 2 {
			want = red
		}
		for _, c := range tri.Colors {
			if c != want {
				t.Errorf("triangle %d: got the color %v, want %v", i, c, want)
			}
		}
	}
	if a := triangleArea(got[0].Points) + triangleArea(got[1].Points); math.Abs(a-area.Width()*area.Height()) > 1e-6 {
		t.Errorf("the background covers %g, want %g", a, area.Width()*area.Height())
	}
	if a := triangleArea(got[2].Points); math.Abs(a-200) > 1e-6 {
		t.Errorf("the triangle covers %g, want 200", a)
	}
}
