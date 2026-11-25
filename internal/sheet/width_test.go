package sheet

import (
	"math"
	"testing"
)

func TestTextWidth(t *testing.T) {
	tests := []struct {
		s    string
		size float64
		exp  float64
	}{
		// the digits of Calibri are 1038 units wide
		{"0123456789", 11, 10 * 1038 * 11 / 2048.0},
		{"Hello", 11, (1276 + 1019 + 470 + 470 + 1080) * 11 / 2048.0},
		// Latin-1 and Latin Extended-A from the table, a combining mark
		// without a width
		{"é", 10, 1019 * 10 / 2048.0},
		{"é", 10, 1019 * 10 / 2048.0},
		// a character of East Asia is as wide as the font is high, a
		// half-width katakana half as wide, and another character half
		{"漢字", 11, 22},
		{"ｱ", 11, 5.5},
		{"Ж", 11, 5.5},
		{"", 11, 0},
	}
	for _, test := range tests {
		if got := TextWidth(test.s, test.size); math.Abs(got-test.exp) > 1e-9 {
			t.Errorf("TextWidth(%q, %g): expected %g, got: %g", test.s, test.size, test.exp, got)
		}
	}
}

func TestArialTextWidth(t *testing.T) {
	tests := []struct {
		s    string
		size float64
		exp  float64
	}{
		// the digits of Arial are 1139 units wide
		{"0123456789", 10, 10 * 1139 * 10 / 2048.0},
		{"Hello", 10, (1479 + 1139 + 455 + 455 + 1139) * 10 / 2048.0},
		{"é", 10, 1139 * 10 / 2048.0},
		{"漢", 10, 10},
		{"", 10, 0},
	}
	for _, test := range tests {
		if got := ArialTextWidth(test.s, test.size); math.Abs(got-test.exp) > 1e-9 {
			t.Errorf("ArialTextWidth(%q, %g): expected %g, got: %g", test.s, test.size, test.exp, got)
		}
	}
}
