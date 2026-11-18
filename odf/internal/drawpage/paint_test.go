package drawpage

import (
	"testing"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/drawingml"
)

// parseGradient returns the paint of a draw:gradient with the attributes.
func parseGradient(t *testing.T, attrs string) drawingml.Paint {
	t.Helper()
	doc, err := roxmltree.Parse(`<draw:gradient xmlns:draw="` + nsDraw + `" ` + attrs + `/>`)
	if err != nil {
		t.Fatal(err)
	}
	return gradient(doc.RootElement(), 1)
}

// TestGradientPaths checks that the gradients of OpenDocument along a path
// keep their style, and that the angle turns an ellipsoid, a square and a
// rectangular one.
func TestGradientPaths(t *testing.T) {
	for _, tt := range []struct {
		style, path string
		angle       float64
	}{
		{"radial", "radial", 0},
		{"ellipsoid", "ellipsoid", -90},
		{"square", "square", -90},
		{"rectangular", "rectangular", -90},
		{"other", "radial", 0},
	} {
		pt := parseGradient(t, `draw:style="`+tt.style+`" draw:angle="900" draw:cx="20%" draw:cy="30%"`)
		g := pt.Gradient
		if g.Path != tt.path || g.Angle != tt.angle {
			t.Errorf("%s: expected the path %q at %g degrees, got %q at %g", tt.style, tt.path, tt.angle, g.Path, g.Angle)
		}
		if g.Focus.L != 0.2 || g.Focus.T != 0.3 {
			t.Errorf("%s: expected the center at 20%% and 30%%, got %+v", tt.style, g.Focus)
		}
	}
}

// TestAxialGradient checks that an axial gradient has the end color at
// both edges and the start color in the middle, with half of the border
// at each edge, as LibreOffice draws libreoffice/odp/shapes-test.odp.
func TestAxialGradient(t *testing.T) {
	pt := parseGradient(t, `draw:style="axial" draw:start-color="#ff0000" draw:end-color="#ffffff" draw:border="50%"`)
	stops := pt.Gradient.Stops
	white, red := drawingml.White, drawingml.RGBA{R: 1, A: 1}
	want := []drawingml.Stop{{Pos: 0, Color: white}, {Pos: 0.25, Color: white}, {Pos: 0.5, Color: red}, {Pos: 0.5, Color: red}, {Pos: 0.75, Color: white}}
	if len(stops) != len(want) {
		t.Fatalf("expected %d stops, got %+v", len(want), stops)
	}
	for i, s := range stops {
		if s != want[i] {
			t.Errorf("expected the stop %d to be %+v, got %+v", i, want[i], s)
		}
	}
}
