package xlsx

import (
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/sheet"
)

func TestEvalNumber(t *testing.T) {
	num := func(col int, v float64) sheet.Cell { return sheet.Cell{Col: col, Kind: sheet.KindNumber, Value: v} }
	budget := &sheet.Sheet{Name: "Budget", Rows: []sheet.Row{
		{Index: 3, Cells: []sheet.Cell{{Col: 1, Text: "Item"}, {Col: 2, Text: "Amount"}}},
		{Index: 4, Cells: []sheet.Cell{num(2, 2500)}},
		{Index: 5, Cells: []sheet.Cell{num(2, 1000), num(4, 7)}},
		{Index: 6, Cells: []sheet.Cell{num(2, 250), {Col: 4, Text: "text"}}},
		{Index: 7, Cells: []sheet.Cell{num(2, 9999)}},
	}}
	other := &sheet.Sheet{Name: "My Sheet", Rows: []sheet.Row{{Index: 0, Cells: []sheet.Cell{num(0, 5), num(1, 6)}}}}
	b := &book{
		lim:    ivy.Limits{}.WithDefaults(),
		sheets: map[string]*sheet.Sheet{"Budget": budget, "My Sheet": other, "Later": {Name: "Later"}},
		names: map[nameKey]string{
			{-1, "totalmonthlyincome"}: "SUM(tblIncome[Amount])",
			{-1, "loop"}:               "loop",
			{0, "local"}:               "Budget!$E$6",
			{-1, "local"}:              "1",
		},
		tables: map[string]table{
			// B4:C8 with a header and a row of totals
			"tblincome": {sheet: "Budget", ref: sheet.Range{Row0: 3, Col0: 1, Row1: 7, Col1: 2}, header: 1, totals: 1, cols: []string{"item", "amount"}},
		},
	}
	tests := []struct {
		f    string
		want float64
		ok   bool
	}{
		{"3.5", 3.5, true},
		{"=TotalMonthlyIncome", 3750, true},
		{"SUM(tblIncome[Amount])", 3750, true},
		{"sum(tblIncome[[#Data],[Amount]])", 3750, true},
		{"SUM(tblIncome[[#All],[Amount]])", 13749, true},
		{"SUM(tblIncome[[#Totals],[Amount]])", 9999, true},
		{"SUM(C5:C6, 'My Sheet'!A1:B1, 2)", 3513, true},
		{"SUM(E5:E7)", 7, true},
		{"$C$6", 1000, true},
		{"local", 7, true},
		{"loop", 0, false},
		{"SUM(tblIncome[Missing])", 0, false},
		{"SUM(Other!A1)", 0, false},
		// a sheet that the book did not read yet
		{"SUM(Later!A1:A3)", 0, false},
		{"SUM(A1)+SUM(B1)", 0, false},
		{"AVERAGE(C5:C7)", 0, false},
	}
	for _, tt := range tests {
		got, ok := b.evalNumber(budget, 0, tt.f, 0)
		if got != tt.want || ok != tt.ok {
			t.Errorf("evalNumber(%q): expected %v, %v, got: %v, %v", tt.f, tt.want, tt.ok, got, ok)
		}
	}
	// a threshold computes its formula once
	if v, ok := b.thresholdValue(budget, 0, "TotalMonthlyIncome"); !ok || v != 3750 || len(b.formulas) != 1 {
		t.Errorf("thresholdValue: %v, %v, %d formulas", v, ok, len(b.formulas))
	}
}
