package metafile

import (
	"image/color"
	"slices"
	"testing"
	"unicode/utf16"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/gowemf"
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

// TestDrawTextRuns checks the text of the runs and the clusters of their
// glyphs, for a writer of PDF (D58): a surrogate pair, a letter of Hebrew
// with its point, and a run of glyph indexes, which has no text.
func TestDrawTextRuns(t *testing.T) {
	rec := &runRecorder{}
	dst, _ := okage.RectFromXYWH(0, 0, 100, 100)
	pic := &Picture{lim: ivy.Limits{}.WithDefaults(), log: ivy.Config{}.Log()}
	b := newBackend(t.Context(), rec, dst, pic)
	b.pic.fonts = newFontSet(nil, officeFonts(t))
	font := gowemf.FontRequest{FaceName: "Liberation Sans", Height: -10}
	paint := gowemf.Paint{Kind: gowemf.PaintSolid, Color: color.NRGBA{A: 255}}
	draw := func(units []uint16, levels []uint8, glyphs bool) {
		t.Helper()
		run := gowemf.TextRun{Font: font, Text: units, Levels: levels, Glyphs: glyphs, Transform: gowemf.Identity(), Paint: paint}
		for i := range units {
			run.Origins = append(run.Origins, gowemf.Point{X: float64(10 * i)})
			run.Advances = append(run.Advances, 10)
		}
		if err := b.DrawText(run, nil); err != nil {
			t.Fatal(err)
		}
	}
	draw(utf16.Encode([]rune("a\U0001D400b")), nil, false)
	draw(utf16.Encode([]rune("שָל")), []uint8{1, 1, 1}, false)
	draw([]uint16{36, 37}, nil, true)
	if len(rec.runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(rec.runs))
	}
	for i, want := range []struct {
		text   string
		ranges [][2]int
	}{
		{"a\U0001D400b", [][2]int{{0, 1}, {1, 5}, {5, 6}}},
		{"שָל", [][2]int{{0, 4}, {0, 4}, {4, 6}}},
		{"", [][2]int{{0, 0}, {0, 0}}},
	} {
		run := rec.runs[i]
		got := make([][2]int, 0, len(run.Glyphs))
		for _, g := range run.Glyphs {
			got = append(got, [2]int{g.Start, g.End})
		}
		slices.SortFunc(got, func(a, b [2]int) int { return a[0] - b[0] })
		if run.Text != want.text || !slices.Equal(got, want.ranges) {
			t.Errorf("run %d: expected %q %v, got %q %v", i, want.text, want.ranges, run.Text, got)
		}
		if want.text != "" && !run.TextRanges() {
			t.Errorf("run %d: expected valid ranges", i)
		}
	}
}

func TestRunText(t *testing.T) {
	text, offsets := runText([]uint16{'a', 0xD835, 0xDC00, 0xD800, 'b'})
	if text != "a\U0001D400�b" || !slices.Equal(offsets, []int{0, 1, 5, 5, 8, 9}) {
		t.Errorf("expected the text and the offsets of the units, got %q %v", text, offsets)
	}
}
