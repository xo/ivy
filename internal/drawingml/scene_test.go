package drawingml

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
)

// The grid of the text bodies of textScene: its columns, and the size of a
// text body in points.
const textCols, textW, textH = 5, 120, 40

// textScene returns a scene of n text bodies in a grid, each one a small
// document of HTML, and the size of the grid in points.
func textScene(n int) (*Scene, int, int) {
	s := NewScene(ivy.Config{})
	for i := range n {
		addTextBody(s, i)
	}
	rows := (n + textCols - 1) / textCols
	return s, textCols * textW, rows * textH
}

// addTextBody adds text body i of the grid of textScene to s.
func addTextBody(s *Scene, i int) {
	doc := fmt.Sprintf(`<p style="margin: 0; font: %dpt %s; color: #%02x3060">Text body %d with words</p>`,
		8+i%5, []string{"serif", "sans-serif", "monospace"}[i%3], 40*(i%6), i+1)
	s.AddHTML(doc, textW, textH, okage.TransformFromTranslate(float32(i%textCols*textW), float32(i/textCols*textH)))
}

// renderScene renders a scene of w by h points at 96 dpi. It reports an
// error with Error, so that a goroutine of the test can call it.
func renderScene(t testing.TB, s *Scene, w, h int) []uint8 {
	t.Helper()
	pix, _ := okage.NewPixmap(uint32(w*4/3), uint32(h*4/3))
	pix.Fill(okage.ColorWhite)
	if err := s.Render(t.Context(), pix, okage.TransformFromScale(4.0/3, 4.0/3)); err != nil {
		t.Error(err)
	}
	return pix.Data()
}

// TestSceneSharedFonts checks that the text bodies of a scene share one font
// configuration (D54), that they draw the pixels of text bodies with
// configurations of their own, and that renders of the scene at the same
// time give the same pixels.
func TestSceneSharedFonts(t *testing.T) {
	const n = 30
	s, w, h := textScene(n)
	want := renderScene(t, s, w, h)
	if s.fonts == nil {
		t.Fatal("expected a font configuration of the scene")
	}
	// the same text bodies, each one in a scene of its own, which has a
	// configuration of its own
	pix, _ := okage.NewPixmap(uint32(w*4/3), uint32(h*4/3))
	pix.Fill(okage.ColorWhite)
	for i := range n {
		one := NewScene(ivy.Config{})
		addTextBody(one, i)
		if err := one.Render(t.Context(), pix, okage.TransformFromScale(4.0/3, 4.0/3)); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(pix.Data(), want) {
		t.Error("the text bodies with a shared configuration draw other pixels")
	}
	// renders at the same time take turns on the lock of the configuration
	fresh, _, _ := textScene(n)
	var wg sync.WaitGroup
	got := make([][]uint8, 8)
	for i := range got {
		wg.Go(func() {
			got[i] = renderScene(t, fresh, w, h)
		})
	}
	wg.Wait()
	for i, g := range got {
		if !slices.Equal(g, want) {
			t.Errorf("render %d at the same time draws other pixels", i)
		}
	}
}

// BenchmarkSceneText lays out and draws a new scene of 30 text bodies, as a
// slide of a table or of many shapes with text.
func BenchmarkSceneText(b *testing.B) {
	for b.Loop() {
		s, w, h := textScene(30)
		renderScene(b, s, w, h)
	}
}
