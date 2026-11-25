// This file is not a port. It tests the text of the runs of glyphs, which a
// writer of PDF gives to a reader that searches or copies it (D58).

package layout

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
)

// runRecorder is a device that keeps the runs of glyphs.
type runRecorder struct {
	display.List
	runs []display.GlyphRun
}

// FillGlyphs satisfies the display.Device interface.
func (r *runRecorder) FillGlyphs(run display.GlyphRun, shader okage.Shader) {
	r.runs = append(r.runs, run)
	r.List.FillGlyphs(run, shader)
}

func TestDrawTextRuns(t *testing.T) {
	body := `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Café <b>bold</b> text, and more</p></body></html>`
	src := Source{
		FS:          fstest.MapFS{"c.xhtml": &fstest.MapFile{Data: []byte(body)}},
		Documents:   []string{"c.xhtml"},
		StyleSheets: []string{bookStyle},
	}
	b := NewBook(src, BookOptions{})
	var r runRecorder
	if err := b.Draw(context.Background(), 0, &r, okage.IdentityTransform(), 1); err != nil {
		t.Fatal(err)
	}
	var words []string
	for _, run := range r.runs {
		if !run.TextRanges() {
			t.Errorf("expected a text with valid ranges, got %q", run.Text)
			continue
		}
		if _, ok := run.Face.Source(); !ok {
			t.Errorf("expected the face of %q to have a source", run.Text)
		}
		for _, g := range run.Glyphs {
			if g.Start >= g.End {
				t.Errorf("expected each glyph of %q to cover a character, got %d to %d", run.Text, g.Start, g.End)
			}
		}
		words = append(words, strings.Fields(run.Text)...)
	}
	if got, want := strings.Join(words, " "), "Café bold text, and more"; got != want {
		t.Errorf("expected the words %q, got %q", want, got)
	}
}

// TestDrawTextOutline checks that -ivy-text-outline strokes the outlines of
// the glyphs in the color of the text, and does not fill them.
func TestDrawTextOutline(t *testing.T) {
	assertPixels(t, "text_outline", `
        zzzzzzzzzzzz
        zzzzzzzzzzzz
        zzzzzzzzzzzz
        zzzzzzzzzzzz
        zzzz____zzzz
        zzRz____zRzz
        zzzz____zzzz
        zzzz____zzzz
        zzzzzzzzzzzz
        zzzzzzzzzzzz
        zzzzzzzzzzzz
        zzzzzzzzzzzz
    `, `
      <style>
        @page { size: 12px }
        body {
          margin: 2px; color: red; font-family: weasyprint; font-size: 8px;
          line-height: 1; -ivy-text-outline: 2px
        }
      </style>
      <body>X`)
}

// TestHangingWhite checks that a line from right to left starts left of
// its box by the width of the white space at its end that the box leaves
// out, such as the space that hangs at the end of a line of white-space:
// pre-wrap, and by no more.
func TestHangingWhite(t *testing.T) {
	// "ab " drawn from right to left: the space first, then b and a.
	runs := []GlyphRun{{Glyphs: []Glyph{
		{Advance: 4, Cluster: 2, ClusterEnd: 3},
		{Advance: 10, Cluster: 1, ClusterEnd: 2},
		{Advance: 10, Cluster: 0, ClusterEnd: 1},
	}}}
	for _, c := range []struct {
		name   string
		text   string
		length int
		width  float64
		want   float64
	}{
		{"hanging", "ab ", 3, 20, 4},
		{"in the box", "ab ", 3, 24, 0},
		{"no space", "abc", 3, 20, 0},
		{"part in the box", "ab ", 3, 22, 2},
	} {
		if got := hangingWhite(c.text, c.length, runs, c.width); got != c.want {
			t.Errorf("%s: expected %g, got %g", c.name, c.want, got)
		}
	}
}
