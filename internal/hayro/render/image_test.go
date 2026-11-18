// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the tests of the changes of the port to the drawing of
// images and of the cells of tiling patterns, which upstream does not have.

package render

import (
	"math"
	"testing"

	"github.com/xo/okage/kurbo"
)

// TestGridFit moves an edge of an image that keeps the axes onto a whole
// pixel when it is very near it, and leaves an image that turns by another
// angle as it is.
func TestGridFit(t *testing.T) {
	tests := []struct {
		name string
		in   kurbo.Affine
		want kurbo.Affine
	}{
		{"far", kurbo.Affine{0.5, 0, 0, 0.5, 10.25, 20.75}, kurbo.Affine{0.5, 0, 0, 0.5, 10.25, 20.75}},
		{"whole", kurbo.Affine{0.5, 0, 0, 0.5, 10, 20}, kurbo.Affine{0.5, 0, 0, 0.5, 10, 20}},
		{"near", kurbo.Affine{0.5, 0, 0, 0.5, 10.001, 19.999}, kurbo.Affine{0.5, 0, 0, 0.5, 10, 20}},
		{"scale", kurbo.Affine{0.2500001, 0, 0, -0.2500001, 0, 2}, kurbo.Affine{0.25, 0, 0, -0.25, 0, 2}},
		{"one edge", kurbo.Affine{0.6, 0, 0, 0.5, 10.001, 20.25}, kurbo.Affine{0.60025, 0, 0, 0.5, 10, 20.25}},
		{"turned", kurbo.Affine{0, 0.5, -0.5, 0, 10.001, 19.999}, kurbo.Affine{0, 0.5, -0.5, 0, 10, 20}},
		{"skewed", kurbo.Affine{0.5, 0.1, 0, 0.5, 10.001, 20}, kurbo.Affine{0.5, 0.1, 0, 0.5, 10.001, 20}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gridFit(tt.in, 4, 4)
			for i := range got {
				if math.Abs(got[i]-tt.want[i]) > 1e-9 {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestImageSoftMaskResize draws an image of a black pixel that its soft mask
// shows and a white pixel that it hides, at half its width. The colors and
// the soft mask are resized each on its own, as MuPDF and Poppler do, so the
// pixel is a gray at half its alpha: 255 - 0.5 * (255 - 127.5).
func TestImageSoftMaskResize(t *testing.T) {
	image := "<< /Type /XObject /Subtype /Image /Width 2 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8 /SMask 7 0 R /Length 2 >>\nstream\n\x00\xff\nendstream"
	mask := "<< /Type /XObject /Subtype /Image /Width 2 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length 2 >>\nstream\n\xff\x00\nendstream"
	pix := renderPage(t, "q 1 0 0 1 50 50 cm /Im Do Q", "<< /XObject << /Im 6 0 R >> >>", image, mask)
	checkPixels(t, pix, map[[2]uint32][3]uint8{
		{50, 49}: {191, 191, 191},
	})
}

// TestAlignedTileAxis starts the pixmap of a direction that pads on a pixel
// of the device, at the scale of the device, and leaves a direction that
// repeats as upstream lays it out.
func TestAlignedTileAxis(t *testing.T) {
	for _, dev := range []deviceAxis{{0.75, 0.3}, {-0.75, 100}, {0.2844, -0.308}} {
		a := newAlignedTileAxis(math.Abs(dev.k), 0, 200, 300, 10, 50, dev)
		edge := dev.k*a.start + dev.offset
		if math.Abs(edge-math.Round(edge)) > 1e-9 || a.start > 10 || a.scale != math.Abs(dev.k) {
			t.Errorf("%v: start %v at %v on the device, scale %v", dev, a.start, edge, a.scale)
		}
		if end := a.start + float64(a.size)/a.scale; end < 50 {
			t.Errorf("%v: the pixmap ends at %v, before 50", dev, end)
		}
		repeat := newAlignedTileAxis(math.Abs(dev.k), 0, 20, 30, 10, 500, dev)
		if want := newTileAxis(math.Abs(dev.k), 0, 20, 30, 10, 500); repeat != want {
			t.Errorf("%v: got %+v, want %+v", dev, repeat, want)
		}
	}
}
