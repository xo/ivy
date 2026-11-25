package docx

import "testing"

// TestSymbolCodes checks that the text of a bullet of a symbol font takes
// the characters of the private area, where the font maps them.
func TestSymbolCodes(t *testing.T) {
	tests := []struct {
		s, exp string
		ok     bool
	}{
		{"", "", true},
		{"§", "", true},
		{"o", "", true},
		{"", "", false},
		{"•", "", false},
	}
	for _, test := range tests {
		got, ok := symbolCodes(test.s)
		if got != test.exp || ok != test.ok {
			t.Errorf("%q: expected %q %t, got %q %t", test.s, test.exp, test.ok, got, ok)
		}
	}
}
