package chart

import (
	"context"
	"image/color"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
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

// TestRichTitle checks that each run of the rich text of the title of a
// chart and of the title of an axis keeps its own style, as a label of
// rich text does. The lines of the title of an axis join with a space.
func TestRichTitle(t *testing.T) {
	axes := strings.Replace(categoryAxes, `<c:axPos val="b"/>`, `<c:axPos val="b"/><c:title><c:tx><c:rich><a:bodyPr/>`+
		`<a:p><a:r><a:rPr b="0"/><a:t>Age</a:t></a:r></a:p><a:p><a:r><a:rPr b="1"/><a:t>in years</a:t></a:r></a:p>`+
		`</c:rich></c:tx></c:title>`, 1)
	xml := chartXML(`Audit </a:t></a:r><a:r><a:t>Areas`, `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>`+
		ser(0, "S", "", strCache("A", "x", "y"), "3", "5")+axisRefs+`</c:barChart>`, axes, "")
	xml = []byte(strings.Replace(string(xml), `<a:r><a:t>Audit `, `<a:r><a:rPr b="0"/><a:t>Audit `, 1))
	c, err := Parse(xml, Options{Fonts: fontdb.Default()})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.titleSpans) != 1 || len(c.titleSpans[0]) != 2 || !slices.Equal(c.title, []string{"Audit Areas"}) {
		t.Fatalf("expected a title of two runs, got %q %+v", c.title, c.titleSpans)
	}
	if !c.titleText.bold {
		t.Errorf("expected the bold of the body as the style of the title, got %+v", c.titleText)
	}
	if c.cat.title != "Age in years" || len(c.cat.titleSpans) != 2 {
		t.Fatalf("expected an axis title of two runs on one line, got %q %+v", c.cat.title, c.cat.titleSpans)
	}
	rec := &runRecorder{}
	r, _ := okage.RectFromXYWH(0, 0, 400, 300)
	if err := c.Draw(context.Background(), rec, r); err != nil {
		t.Fatal(err)
	}
	faces := map[string]*font.Face{}
	for _, run := range rec.runs {
		faces[run.Text] = run.Face
	}
	for _, pair := range [][2]string{{"Audit ", "Areas"}, {"Age ", "in years"}} {
		a, b := faces[pair[0]], faces[pair[1]]
		if a == nil || b == nil || a == b {
			t.Errorf("expected %q and %q in two faces, got the faces %p and %p", pair[0], pair[1], a, b)
		}
	}
}
