package pdfenc

import (
	"strings"
	"testing"

	"github.com/xo/okage/kurbo"
	"github.com/xo/okage/pdf"

	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// textDevice is a device of the interpreter of hayro that keeps the text of
// the runs of glyphs of a page, as a reader of the text of a PDF file reads
// it.
type textDevice struct {
	interpret.DummyDevice
	text strings.Builder
}

// DrawGlyphRun satisfies the interpret.Device interface.
func (d *textDevice) DrawGlyphRun(run *interpret.GlyphRun, _ interpret.DrawProps, _ interpret.DrawMode) {
	for _, g := range run.Glyphs() {
		u, ok := g.AsUnicode()
		switch {
		case !ok:
		case u.IsString:
			d.text.WriteString(u.String)
		default:
			d.text.WriteRune(u.Char)
		}
	}
	d.text.WriteByte(' ')
}

// pageText returns the text of the runs of glyphs of page n of the PDF
// file, with a space after each run.
func pageText(t *testing.T, data []byte, n int) string {
	t.Helper()
	doc, err := syntax.NewPdf(data)
	if err != nil {
		t.Fatal(err)
	}
	pages := doc.Pages().Slice()
	if n >= len(pages) {
		t.Fatalf("expected at least %d pages, got: %d", n+1, len(pages))
	}
	page := pages[n]
	ctx := interpret.NewContext(interpret.TransformToKurbo(page.InitialTransform(true)),
		kurbo.NewRect(0, 0, 1000, 1000), interpret.NewInterpreterCache(), page.XRef(), interpret.DefaultInterpreterSettings())
	var d textDevice
	if err := interpret.InterpretPage(page, ctx, &d); err != nil {
		t.Fatal(err)
	}
	return d.text.String()
}

// TestOutlineText checks that the text of a run whose outlines the page
// strokes, as -ivy-text-outline draws it, goes into the file as invisible
// text in the text rendering mode 3, which a reader can search and copy,
// without a graphics state, and that the file embeds one font for it and
// for the filled text. TestScenes checks that the page looks the same.
func TestOutlineText(t *testing.T) {
	data, _ := encodeScene(t, scene{draw: drawOutline})
	if got := strings.Fields(pageText(t, data, 0)); strings.Join(got, " ") != "Filled Outlined" {
		t.Errorf("expected the text %q, got: %q", "Filled Outlined", got)
	}
	if n := strings.Count(string(data), "/FontFile2"); n != 1 {
		t.Errorf("expected one embedded font, got: %d", n)
	}
	all := string(inflated(t, data))
	if n := strings.Count(all, "3 Tr"); n != 1 {
		t.Errorf("expected one run in the text rendering mode 3, got: %d", n)
	}
	if strings.Contains(all, "/ca 0") {
		t.Error("expected no graphics state with an opacity of 0")
	}
}

// TestBaseText checks that the text of a cluster from right to left goes
// to its base: the whole text when the cluster has more glyphs than
// characters, and one character to each glyph when it has as many, and
// that a run from left to right keeps its ranges.
func TestBaseText(t *testing.T) {
	g := func(start, end int, advance float32) pdf.Glyph {
		return pdf.Glyph{TextRange: pdf.TextRange{Start: start, End: end}, XAdvance: advance}
	}
	r := func(start, end int) pdf.TextRange {
		return pdf.TextRange{Start: start, End: end}
	}
	// "بيا" with a tanween on the alef: يا is 4 bytes, the alef 2 and the
	// tanween 2. From right to left, the glyphs are the tanween, the alef,
	// the dots of the yeh, its dotless base and the beh.
	text := "بياً"
	glyphs := []pdf.Glyph{g(4, 8, 0), g(4, 8, 0.3), g(2, 4, 0), g(2, 4, 0.4), g(0, 2, 0.5)}
	baseText(glyphs, text)
	want := []pdf.TextRange{r(6, 8), r(4, 6), r(2, 2), r(2, 4), r(0, 2)}
	for i, w := range want {
		if glyphs[i].TextRange != w {
			t.Errorf("glyph %d: expected %v, got %v", i, w, glyphs[i].TextRange)
		}
	}
	// From left to right, the first glyph of a cluster keeps its text.
	glyphs = []pdf.Glyph{g(0, 2, 0.5), g(0, 2, 0), g(2, 4, 0.4)}
	baseText(glyphs, "ab")
	for i, w := range []pdf.TextRange{r(0, 2), r(0, 2), r(2, 4)} {
		if glyphs[i].TextRange != w {
			t.Errorf("left to right, glyph %d: expected %v, got %v", i, w, glyphs[i].TextRange)
		}
	}
}
