// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// This file holds tests of the items of the package. kurbo tests them
// through other items, so the tests are not ports.

package kurboext

import (
	"math"
	"testing"

	"github.com/xo/okage/kurbo"
)

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestBoundingBox(t *testing.T) {
	var p BezPath
	p.MoveTo(kurbo.NewPoint(0, 0))
	p.CurveTo(kurbo.NewPoint(0, 10), kurbo.NewPoint(10, 10), kurbo.NewPoint(10, 0))
	p.ClosePath()
	got := p.BoundingBox()
	if !near(got.X0, 0) || !near(got.Y0, 0) || !near(got.X1, 10) || !near(got.Y1, 7.5) {
		t.Errorf("BoundingBox = %v, want (0, 0, 10, 7.5)", got)
	}
	var empty BezPath
	empty.MoveTo(kurbo.NewPoint(5, 5))
	if got := empty.BoundingBox(); got != RectZero {
		t.Errorf("BoundingBox of a lone MoveTo = %v, want zero", got)
	}
}

func TestRect(t *testing.T) {
	a := kurbo.NewRect(0, 0, 10, 10)
	b := kurbo.NewRect(5, 5, 20, 20)
	if got := RectIntersect(a, b); got != kurbo.NewRect(5, 5, 10, 10) {
		t.Errorf("RectIntersect = %v", got)
	}
	if got := RectIntersect(a, kurbo.NewRect(20, 20, 30, 30)); RectArea(got) != 0 {
		t.Errorf("RectIntersect of apart rectangles = %v", got)
	}
	if !RectOverlaps(a, b) || RectOverlaps(a, kurbo.NewRect(10, 0, 20, 10)) {
		t.Error("RectOverlaps")
	}
	if !RectContains(a, kurbo.NewPoint(0, 0)) || RectContains(a, kurbo.NewPoint(10, 5)) {
		t.Error("RectContains")
	}
	if got := RectInflate(a, 1, 2); got != kurbo.NewRect(-1, -2, 11, 12) {
		t.Errorf("RectInflate = %v", got)
	}
	if got := RectExpand(kurbo.NewRect(0.5, 0.5, 9.5, 9.5)); got != a {
		t.Errorf("RectExpand = %v", got)
	}
	if !RectIsZeroArea(kurbo.NewRect(1, 1, 1, 5)) {
		t.Error("RectIsZeroArea")
	}
	path := RectToPath(a, 0.1)
	if path.Len() != 5 || path.IsEmpty() {
		t.Errorf("RectToPath = %v", path.Elements())
	}
}

func TestMulBezPath(t *testing.T) {
	var p BezPath
	p.MoveTo(kurbo.NewPoint(1, 2))
	p.LineTo(kurbo.NewPoint(3, 4))
	got := MulBezPath(kurbo.AffineScale(2), p)
	if got.Elements()[1].P[0] != kurbo.NewPoint(6, 8) {
		t.Errorf("MulBezPath = %v", got.Elements())
	}
	if p.Elements()[1].P[0] != kurbo.NewPoint(3, 4) {
		t.Error("MulBezPath changed its input")
	}
	c := p.Clone()
	c.Truncate(1)
	if p.Len() != 2 || c.Len() != 1 {
		t.Error("Clone shares memory")
	}
}
