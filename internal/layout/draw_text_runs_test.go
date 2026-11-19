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
