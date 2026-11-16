package font

import "testing"

func TestSubstitute(t *testing.T) {
	t.Parallel()
	for family, want := range map[string]string{
		"Arial":           "Liberation Sans",
		"helvetica":       "Liberation Sans",
		"Times New Roman": "Liberation Serif",
		"TIMES":           "Liberation Serif",
		"Courier New":     "Liberation Mono",
		" Courier ":       "Liberation Mono",
		"Calibri":         "Carlito",
		"Cambria":         "Caladea",
		"Comic Sans MS":   "",
		"":                "",
	} {
		got, ok := Substitute(family)
		if got != want || ok != (want != "") {
			t.Errorf("Substitute(%q) = %q, %t, expected %q", family, got, ok, want)
		}
	}
}
