package pdfenc

import (
	"testing"

	"github.com/xo/okage/pdf"
)

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
