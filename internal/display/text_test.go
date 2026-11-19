package display

import "testing"

func TestTextRanges(t *testing.T) {
	for _, tt := range []struct {
		name   string
		text   string
		ranges [][2]int
		want   bool
	}{
		{"valid", "aé", [][2]int{{0, 1}, {1, 3}}, true},
		{"one cluster", "aé", [][2]int{{0, 3}, {0, 3}}, true},
		{"empty range", "a", [][2]int{{1, 1}}, true},
		{"no text", "", [][2]int{{0, 0}}, false},
		{"inside a character", "é", [][2]int{{0, 1}}, false},
		{"past the end", "a", [][2]int{{0, 2}}, false},
		{"reversed", "ab", [][2]int{{1, 0}}, false},
		{"negative", "ab", [][2]int{{-1, 1}}, false},
		{"not UTF-8", "a\xff", [][2]int{{0, 1}}, false},
	} {
		run := GlyphRun{Text: tt.text}
		for _, r := range tt.ranges {
			run.Glyphs = append(run.Glyphs, Glyph{Start: r[0], End: r[1]})
		}
		if got := run.TextRanges(); got != tt.want {
			t.Errorf("%s: expected %t, got %t", tt.name, tt.want, got)
		}
	}
}
