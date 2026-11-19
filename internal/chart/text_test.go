package chart

import (
	"image/color"
	"slices"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fontdb"
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

// TestTextRuns checks that a label gives its text and the character of
// each glyph, for a writer of PDF (D58).
func TestTextRuns(t *testing.T) {
	tx := newTexter(fontdb.Default(), "Go")
	rec := &runRecorder{}
	st := textStyle{size: 10, color: color.NRGBA{A: 255}}
	tx.draw(rec, block{lines: []string{"Café 12", "Total"}, st: st}, 0, 0, alignStart, alignStart, 0)
	if len(rec.runs) != 2 {
		t.Fatalf("expected a run for each line, got %d", len(rec.runs))
	}
	run := rec.runs[0]
	if run.Text != "Café 12" || !run.TextRanges() {
		t.Fatalf("expected the text of the first line with valid ranges, got %q", run.Text)
	}
	got := make([][2]int, 0, len(run.Glyphs))
	for _, g := range run.Glyphs {
		got = append(got, [2]int{g.Start, g.End})
	}
	want := [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 5}, {5, 6}, {6, 7}, {7, 8}}
	if !slices.Equal(got, want) {
		t.Errorf("expected the ranges %v, got %v", want, got)
	}
	if _, ok := run.Face.Source(); !ok {
		t.Error("expected the face to have a source")
	}
	if rec.runs[1].Text != "Total" {
		t.Errorf("expected the text of the second line, got %q", rec.runs[1].Text)
	}
}
