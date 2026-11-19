package sheet

import (
	"log/slog"
	"math"
	"os"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fontdb"
)

func TestExcelRows(t *testing.T) {
	// Arial of 10 points: S = 17 pixels and P = 103 dots
	e := NewExcelRows(nil, Font{Family: "Arial", Size: 10})
	if got := e.Default(); got != 12.36 {
		t.Errorf("expected the default height 12.36, got: %v", got)
	}
	// a row of 20 points is 26.67 pixels, and 161.57 dots, which the grid
	// rounds down to 161 dots
	sh := &Sheet{RowDots: true}
	if got := e.Printed(20); math.Abs(got-19.388) > 0.001 || sh.grid(got) != 19.32 {
		t.Errorf("expected 19.388 and 19.32 for a row of 20 points, got: %v, %v", got, sh.grid(got))
	}
	if got := e.Printed(0); got != 0 {
		t.Errorf("expected 0 for a row of 0 points, got: %v", got)
	}
	// Calibri of 11 points: 121 dots for 20 pixels
	if got := NewExcelRows(nil, Font{Family: "Calibri"}).Scale(); math.Abs(got-calibriRowScale) > 1e-9 {
		t.Errorf("expected the scale %v, got: %v", calibriRowScale, got)
	}
}

func TestPrintHeight(t *testing.T) {
	if _, err := os.Stat("../../fonts"); err != nil {
		t.Skip("no fonts module")
	}
	db, _ := fontdb.Load(os.DirFS("../../fonts"), false, ivy.Limits{}.WithDefaults(), slog.New(slog.DiscardHandler))
	// Carlito and Liberation Sans have the vertical metrics of Calibri and
	// Arial, and give P of Excel for them
	for _, test := range []struct {
		font Font
		want int
	}{
		{Font{Family: "Calibri", Size: 8}, 90},
		{Font{Family: "Calibri", Size: 11}, 121},
		{Font{Family: "Calibri", Size: 16}, 171},
		{Font{Family: "Arial", Size: 12}, 123},
	} {
		got, ok := printHeight(db, test.font, math.Round(test.font.Size*printDPI/72))
		if !ok || got != test.want {
			t.Errorf("%s %v: expected %d dots, got: %d, %t", test.font.Family, test.font.Size, test.want, got, ok)
		}
	}
	// a font that measuredRows does not hold takes P of its face, and S of
	// P
	e := NewExcelRows(db, Font{Family: "Calibri", Size: 12})
	if e.print < 125 || e.print > 140 || e.screen != int(math.Ceil(float64(e.print)*screenDPI/printDPI)) {
		t.Errorf("unexpected P and S of Calibri of 12 points: %d, %d", e.print, e.screen)
	}
}

func TestVDMX(t *testing.T) {
	// a table of version 1 with one ratio 1:1 and a group of two records:
	// 12 and 13 pixels per em
	b := []byte{
		0, 1, 0, 1, 0, 1,
		1, 1, 1, 1,
		0, 12,
		0, 2, 12, 13,
		0, 12, 0, 11, 0xFF, 0xFD,
		0, 13, 0, 12, 0xFF, 0xFC,
	}
	if h, ok := vdmxHeight(b, 13); !ok || h != 16 {
		t.Errorf("expected 16 at 13 pixels per em, got: %d, %t", h, ok)
	}
	if _, ok := vdmxHeight(b, 14); ok {
		t.Error("expected no height at 14 pixels per em")
	}
	for i := range b {
		vdmxHeight(b[:i], 13)
	}
}

func TestFit(t *testing.T) {
	e := NewExcelRows(nil, Font{Family: "Calibri", Size: 11})
	styles := []Style{
		DefaultStyle,
		{Font: Font{Family: "Calibri", Size: 16}},
		{Font: Font{Family: "Calibri", Size: 11}, Wrap: true},
		{Font: Font{Family: "Calibri", Size: 8}},
	}
	sh := &Sheet{
		DefaultColWidth: 48,
		Rows: []Row{
			// a row of 30 points with one line of the default font
			{Index: 0, Height: 29.04, Fit: true, Cells: []Cell{{Col: 0, Text: "a"}}},
			// a line of Calibri of 16 points: 28 pixels
			{Index: 1, Fit: true, Cells: []Cell{{Col: 0, Text: "b", Style: 1}}},
			// three lines of text that wraps
			{Index: 2, Fit: true, Cells: []Cell{{Col: 0, Text: "one\ntwo\nthree", Style: 2}}},
			// a merged cell of two rows does not count, and the row keeps
			// the default height
			{Index: 3, Fit: true, Cells: []Cell{{Col: 0, Text: "c", Style: 1}}},
			// a row that a person set keeps its height
			{Index: 4, Height: 29.04, Cells: []Cell{{Col: 0, Text: "d"}}},
			// an empty cell of Calibri of 16 points takes one line of 28
			// pixels, and a row without cells keeps its height
			{Index: 5, Fit: true, Cells: []Cell{{Col: 0, Style: 1}}},
			{Index: 8, Height: 30, Fit: true},
			// a line of Calibri of 8 points: 15 pixels
			{Index: 7, Fit: true, Cells: []Cell{{Col: 0, Text: "f", Style: 3}}},
			// a height of 21 pixels in the file is close to the fit of 20
			// pixels, and stays
			{Index: 6, Height: e.printed(21), Fit: true, Cells: []Cell{{Col: 0, Text: "e"}}},
		},
		Merges: []Range{{Row0: 3, Col0: 0, Row1: 4, Col1: 1}},
	}
	e.Fit(sh, styles)
	for i, want := range []float64{14.52, e.printed(28), e.printed(60), 0, 29.04, e.printed(28), 30, e.printed(15), e.printed(21)} {
		if got := sh.Rows[i].Height; got != want {
			t.Errorf("row %d: expected %v, got: %v", i, want, got)
		}
		if sh.Rows[i].Fit {
			t.Errorf("row %d: expected Fit to be clear", i)
		}
	}
}

func TestWrappedLines(t *testing.T) {
	for _, test := range []struct {
		text  string
		width float64
		want  int
	}{
		{"", 100, 1},
		{"a b", 0, 1},
		{"a\nb", 100, 2},
		// "aaa bbb" is 35.63 points wide in Calibri of 11 points
		{"aaa bbb", 40, 1},
		{"aaa bbb", 30, 2},
		// a word of 47.18 points in lines of 20 points
		{"wwwwww", 20, 3},
	} {
		if got := wrappedLines(test.text, 11, test.width); got != test.want {
			t.Errorf("%q in %v points: expected %d lines, got: %d", test.text, test.width, test.want, got)
		}
	}
}
