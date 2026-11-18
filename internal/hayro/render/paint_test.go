// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the tests of the shadings that okage fills itself, which
// upstream does not have: a gradient with no extend, and a mesh.

package render

import (
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// renderPage renders a page of 100 by 100 points over white, with the
// content and the objects from 5 on. Object 5 is the resources.
func renderPage(t *testing.T, content string, objects ...string) *okage.Pixmap {
	t.Helper()
	var b strings.Builder
	b.WriteString("%PDF-1.7\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	b.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources 5 0 R /Contents 4 0 R >>\nendobj\n")
	b.WriteString("4 0 obj\n<< /Length " + strconv.Itoa(len(content)) + " >>\nstream\n" + content + "\nendstream\nendobj\n")
	for i, o := range objects {
		b.WriteString(strconv.Itoa(5+i) + " 0 obj\n" + o + "\nendobj\n")
	}
	b.WriteString("trailer\n<< /Root 1 0 R >>\n%%EOF\n")
	pdf, err := syntax.NewPdf([]byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	settings := PixmapSettings{XScale: 1, YScale: 1, BgColor: okage.ColorWhite}
	pix, err := Render(pdf.Pages().Slice()[0], NewRenderCache(), interpret.DefaultInterpreterSettings(), &RenderSettings{}, &settings)
	if err != nil || pix == nil {
		t.Fatalf("expected a pixmap, got %v", err)
	}
	return pix
}

// checkPixels checks the colors of the pixels of pix, within 2 of each
// channel.
func checkPixels(t *testing.T, pix *okage.Pixmap, want map[[2]uint32][3]uint8) {
	t.Helper()
	for p, w := range want {
		c, ok := pix.Pixel(p[0], p[1])
		if !ok {
			t.Fatalf("no pixel at %v", p)
		}
		got := [3]uint8{c.Red(), c.Green(), c.Blue()}
		for k := range got {
			if d := int(got[k]) - int(w[k]); d < -2 || d > 2 {
				t.Errorf("at %v: got %v, want %v", p, got, w)
				break
			}
		}
	}
}

// meshStream returns a stream of a shading of type 4 with the entries of
// dict, whose vertices hold a flag, x, y and the components in a byte each.
func meshStream(dict string, vertices ...[]byte) string {
	var data []byte
	for _, v := range vertices {
		data = append(data, v...)
	}
	return "<< /ShadingType 4 /BitsPerCoordinate 8 /BitsPerComponent 8 /BitsPerFlag 8 " + dict +
		" /Length " + strconv.Itoa(len(data)) + " >>\nstream\n" + string(data) + "\nendstream"
}

// TestAxialNoExtend fills the page with an axial shading from x = 25 to
// x = 75, with no extend. Nothing paints past its ends.
func TestAxialNoExtend(t *testing.T) {
	pix := renderPage(t, "/Pattern cs /P scn 0 0 100 100 re f",
		"<< /Pattern << /P 6 0 R >> >>",
		"<< /PatternType 2 /Shading << /ShadingType 2 /ColorSpace /DeviceRGB /Coords [25 0 75 0] "+
			"/Function << /FunctionType 2 /Domain [0 1] /C0 [1 0 0] /C1 [0 0 1] /N 1 >> >> >>")
	checkPixels(t, pix, map[[2]uint32][3]uint8{
		{10, 50}: {255, 255, 255},
		{24, 50}: {255, 255, 255},
		{26, 50}: {245, 0, 10},
		{50, 50}: {125, 0, 130},
		{74, 50}: {5, 0, 250},
		{76, 50}: {255, 255, 255},
		{90, 50}: {255, 255, 255},
	})
}

// TestMeshFunction fills a square of two triangles whose function jumps
// from red to blue at the middle. The jump stays sharp.
func TestMeshFunction(t *testing.T) {
	stitch := "/Function << /FunctionType 3 /Domain [0 1] /Bounds [0.5] /Encode [0 1 0 1] /Functions [" +
		"<< /FunctionType 2 /Domain [0 1] /C0 [1 0 0] /C1 [1 0 0] /N 1 >> " +
		"<< /FunctionType 2 /Domain [0 1] /C0 [0 0 1] /C1 [0 0 1] /N 1 >>] >>"
	shading := meshStream("/ColorSpace /DeviceRGB /Decode [0 255 0 255 0 1] "+stitch,
		[]byte{0, 0, 0, 0}, []byte{0, 100, 0, 255}, []byte{0, 0, 100, 0}, []byte{1, 100, 100, 255})
	pix := renderPage(t, "/Sh sh", "<< /Shading << /Sh 6 0 R >> >>", shading)
	checkPixels(t, pix, map[[2]uint32][3]uint8{
		{10, 50}: {255, 0, 0},
		{48, 20}: {255, 0, 0},
		{51, 80}: {0, 0, 255},
		{90, 50}: {0, 0, 255},
	})
}

// TestMeshBackground fills the page with a pattern of a red triangle with a
// green background.
func TestMeshBackground(t *testing.T) {
	shading := meshStream("/ColorSpace /DeviceRGB /Background [0 1 0] /Decode [0 255 0 255 0 1 0 1 0 1]",
		[]byte{0, 40, 40, 255, 0, 0}, []byte{0, 60, 40, 255, 0, 0}, []byte{0, 50, 60, 255, 0, 0})
	pix := renderPage(t, "/Pattern cs /P scn 0 0 100 100 re f",
		"<< /Pattern << /P 6 0 R >> >>", "<< /PatternType 2 /Shading 7 0 R >>", shading)
	checkPixels(t, pix, map[[2]uint32][3]uint8{
		{5, 5}:   {0, 255, 0},
		{95, 95}: {0, 255, 0},
		{50, 52}: {255, 0, 0},
	})
}
