package xlsx

import (
	"testing"

	"github.com/xo/ivy/internal/sheet"
)

// TestSourceFormat checks the number format of the cells that a series of
// a chart names, which Excel formats the values with.
func TestSourceFormat(t *testing.T) {
	sh := &sheet.Sheet{Rows: []sheet.Row{{Index: 8, Cells: []sheet.Cell{
		{Col: 3, Text: "x"},
		{Col: 4, Kind: sheet.KindNumber, Value: 3750, Style: 1},
	}}}}
	b := &book{
		styles: &styles{codes: []string{"General", `"$"#,##0`}},
		sheets: map[string]*sheet.Sheet{"Simple Monthly Budget": sh, "It's": sh},
	}
	for _, test := range []struct {
		ref, exp string
	}{
		{"'Simple Monthly Budget'!$E$9:$F$9", `"$"#,##0`},
		{"'It''s'!E9", `"$"#,##0`},
		// a text, a sheet that the book did not read, and no sheet
		{"'It''s'!D9", ""},
		{"Other!E9", ""},
		{"E9", ""},
	} {
		if got := b.sourceFormat(test.ref); got != test.exp {
			t.Errorf("sourceFormat(%q): expected %q, got: %q", test.ref, test.exp, got)
		}
	}
}
