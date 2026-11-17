// This file is not a port. It tests the synthetic bold of a face that has
// no bold, which upstream gets from Pango and Cairo.

package layout

import "testing"

// boldInk returns the number of red pixels of a letter of weasyprint.otf,
// which has no bold face, drawn with the font weight weight.
func boldInk(t *testing.T, family, weight string) int {
	t.Helper()
	_, _, pixels := htmlToPixels(t, `
      <style>
        @font-face { src: url(weasyprint.otf); font-family: weasyprint }
        @page { size: 60px 60px }
        body { margin: 0; color: red; font: `+weight+` 48px/60px `+family+` }
      </style>a`)
	n := 0
	for _, p := range pixels {
		if p[0] > 200 && p[1] < 100 {
			n++
		}
	}
	return n
}

func TestSyntheticBold(t *testing.T) {
	// The glyph of weasyprint.otf is a square of 48px, which the bold
	// draws 1px larger on each side.
	normal, bold := boldInk(t, "weasyprint", "normal"), boldInk(t, "weasyprint", "bold")
	if normal == 0 || bold < normal+100 {
		t.Errorf("expected the synthetic bold to grow the glyph, got %d pixels, then %d", normal, bold)
	}
	// A family with a bold face draws that face, without a synthetic bold.
	if syntheticBoldOf(t, "Go") {
		t.Error("expected no synthetic bold for Go, which has a bold face")
	}
	if !syntheticBoldOf(t, "weasyprint") {
		t.Error("expected a synthetic bold for weasyprint.otf")
	}
}

// syntheticBoldOf reports whether the bold text of family draws with a
// synthetic bold.
func syntheticBoldOf(t *testing.T, family string) bool {
	t.Helper()
	pages, fc := drawTestPagesLog(t, `
      <style>
        @font-face { src: url(weasyprint.otf); font-family: weasyprint }
        body { font: bold 10px `+family+` }
      </style>a`, testBaseURL, nil)
	var bold, found bool
	pages[0].Descendants(func(b *Box) bool {
		if b.Kind == KindText && b.TextLayout != nil && !found {
			found = true
			b.TextLayout.Reactivate(b.Style)
			line, _, _ := b.TextLayout.FirstLine()
			for _, run := range line.GlyphRuns() {
				bold = fc.syntheticBold(run.Font, b.Style)
			}
			b.TextLayout.Deactivate()
		}
		return true
	})
	if !found {
		t.Fatal("no text")
	}
	return bold
}
