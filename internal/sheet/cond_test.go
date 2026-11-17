package sheet

import (
	"image/color"
	"strings"
	"testing"
)

// condSheet returns a sheet with the numbers 1 to 10 in the column A, and
// texts in the column B.
func condSheet(rules ...CondRule) *Sheet {
	sh := &Sheet{Name: "Cond", DefaultColWidth: 48, DefaultRowHeight: 15, Page: DefaultPage}
	texts := []string{"apple", "pear", "apple", "fig", "", "plum", "pear", "kiwi", "lime", "date"}
	for i := range 10 {
		row := Row{Index: i, Cells: []Cell{{Col: 0, Text: string(rune('0' + i + 1)), Kind: KindNumber, Value: float64(i + 1)}}}
		if texts[i] != "" {
			row.Cells = append(row.Cells, Cell{Col: 1, Text: texts[i]})
		}
		sh.Rows = append(sh.Rows, row)
	}
	sh.CondFormats = []CondFormat{{Ranges: []Range{{Row0: 0, Col0: 0, Row1: 9, Col1: 1}}, Rules: rules}}
	return sh
}

// matched returns the cells of the column col that a rule changes, as the
// numbers of their rows.
func matched(res map[[2]int]*condResult, col int) []int {
	var out []int
	for i := range 10 {
		if r := res[[2]int{i, col}]; r != nil && (len(r.diffs) > 0 || r.fill.A != 0) {
			out = append(out, i)
		}
	}
	return out
}

func TestCondRules(t *testing.T) {
	red := &Diff{Fill: color.NRGBA{255, 0, 0, 255}}
	tests := []struct {
		name string
		rule CondRule
		col  int
		exp  []int
	}{
		{"between", CondRule{Kind: "cellIs", Operator: "between", Operands: []Operand{{Number: 7}, {Number: 3}}, Diff: red}, 0, []int{2, 3, 4, 5, 6}},
		{"greater", CondRule{Kind: "cellIs", Operator: "greaterThan", Operands: []Operand{{Number: 8}}, Diff: red}, 0, []int{8, 9}},
		{"equal text", CondRule{Kind: "cellIs", Operator: "equal", Operands: []Operand{{Text: "PEAR", IsText: true}}, Diff: red}, 1, []int{1, 6}},
		{"top 3", CondRule{Kind: "top10", Rank: 3, Diff: red}, 0, []int{7, 8, 9}},
		{"bottom 20%", CondRule{Kind: "top10", Rank: 20, Percent: true, Bottom: true, Diff: red}, 0, []int{0, 1}},
		{"above average", CondRule{Kind: "aboveAverage", Diff: red}, 0, []int{5, 6, 7, 8, 9}},
		{"below average", CondRule{Kind: "aboveAverage", Below: true, Diff: red}, 0, []int{0, 1, 2, 3, 4}},
		{"duplicates", CondRule{Kind: "duplicateValues", Diff: red}, 1, []int{0, 1, 2, 6}},
		{"contains", CondRule{Kind: "containsText", Text: "I", Diff: red}, 1, []int{3, 7, 8}},
		{"blanks", CondRule{Kind: "containsBlanks", Diff: red}, 1, []int{4}},
	}
	for _, test := range tests {
		res := condSheet(test.rule).evalCond()
		if got := matched(res, test.col); !equalInts(got, test.exp) {
			t.Errorf("%s: expected the rows %v, got: %v", test.name, test.exp, got)
		}
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCondPriority(t *testing.T) {
	// the first rule wins over the second where they both set the fill,
	// and the second stops the third
	first := &Diff{Fill: color.NRGBA{255, 0, 0, 255}}
	second := &Diff{Fill: color.NRGBA{0, 255, 0, 255}, Bold: On}
	third := &Diff{Italic: On}
	res := condSheet(
		CondRule{Kind: "cellIs", Operator: "greaterThan", Operands: []Operand{{Number: 5}}, Diff: second, Priority: 2, StopIfTrue: true},
		CondRule{Kind: "cellIs", Operator: "greaterThan", Operands: []Operand{{Number: 8}}, Diff: first, Priority: 1},
		CondRule{Kind: "cellIs", Operator: "greaterThan", Operands: []Operand{{Number: 0}}, Diff: third, Priority: 3},
	).evalCond()
	st := applyDiffs(DefaultStyle, res[[2]int{9, 0}].diffs, color.NRGBA{})
	if st.Fill != first.Fill || !st.Font.Bold || st.Font.Italic {
		t.Errorf("expected the fill of the first rule, bold and not italic, got: %+v", st)
	}
	st = applyDiffs(DefaultStyle, res[[2]int{6, 0}].diffs, color.NRGBA{})
	if st.Fill != second.Fill || !st.Font.Bold || st.Font.Italic {
		t.Errorf("expected the fill of the second rule, bold and not italic, got: %+v", st)
	}
	st = applyDiffs(DefaultStyle, res[[2]int{2, 0}].diffs, color.NRGBA{})
	if st.Fill.A != 0 || st.Font.Bold || !st.Font.Italic {
		t.Errorf("expected italic only, got: %+v", st)
	}
}

func TestCondScales(t *testing.T) {
	white, green := color.NRGBA{255, 255, 255, 255}, color.NRGBA{0, 200, 0, 255}
	res := condSheet(CondRule{Kind: "colorScale", Scale: []ScalePoint{
		{Threshold: Threshold{Kind: "min"}, Color: white}, {Threshold: Threshold{Kind: "max"}, Color: green}}}).evalCond()
	if got := res[[2]int{0, 0}].fill; got != white {
		t.Errorf("expected white at the lowest value, got: %v", got)
	}
	if got := res[[2]int{9, 0}].fill; got != green {
		t.Errorf("expected green at the highest value, got: %v", got)
	}
	if got := res[[2]int{4, 0}].fill; got.G < 200 || got.R < 100 || got.R > 150 {
		t.Errorf("expected a color between at the middle, got: %v", got)
	}
	res = condSheet(CondRule{Kind: "dataBar", Bar: &DataBar{Min: Threshold{Kind: "min"}, Max: Threshold{Kind: "max"}}}).evalCond()
	if b := res[[2]int{9, 0}].bar; b == nil || b.frac != 1 {
		t.Errorf("expected a whole bar at the highest value, got: %+v", b)
	}
	if b := res[[2]int{0, 0}].bar; b == nil || b.frac != 0 {
		t.Errorf("expected an empty bar at the lowest value, got: %+v", b)
	}
	icons := &IconSet{Name: "3Arrows", Thresholds: []Threshold{{Kind: "percent", GTE: true}, {Kind: "percent", Value: 33, GTE: true}, {Kind: "percent", Value: 67, GTE: true}}}
	res = condSheet(CondRule{Kind: "iconSet", Icons: icons}).evalCond()
	// the thresholds of 33 and 67 percent are the values 3.97 and 7.03
	for row, exp := range map[int]int{0: 0, 2: 0, 3: 1, 6: 1, 7: 2, 9: 2} {
		if ic := res[[2]int{row, 0}].icon; ic == nil || ic.index != exp {
			t.Errorf("expected the icon %d in the row %d, got: %+v", exp, row, ic)
		}
	}
}

func TestCondHTML(t *testing.T) {
	sh := condSheet(
		CondRule{Kind: "cellIs", Operator: "equal", Operands: []Operand{{Number: 2}}, Diff: &Diff{Fill: color.NRGBA{255, 0, 0, 255}, Bold: On}, Priority: 1},
		CondRule{Kind: "dataBar", Priority: 2, Bar: &DataBar{Min: Threshold{Kind: "min"}, Max: Threshold{Kind: "max"}, Color: color.NRGBA{0, 0, 255, 255}}},
		CondRule{Kind: "iconSet", Priority: 3, Icons: &IconSet{Name: "3Arrows", Thresholds: []Threshold{{Kind: "percent"}, {Kind: "percent", Value: 50, GTE: true}}}},
	)
	src, err := (&Workbook{Sheets: []*Sheet{sh}}).Source(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc := document(t, src, 0)
	for _, want := range []string{
		// the style that the rule makes comes after the styles of the
		// workbook
		`td.s1{font-family:"Calibri", sans-serif;font-size:11pt;font-weight:bold;background-color:#ff0000;`,
		`<td class="s1 n a c"><img class="ic" src="cf/3arrows-2-0.svg"`,
		// the bar of the lowest value is a tenth of the cell
		`<div class="db" style="left:1.5pt;top:1.5pt;width:4.5pt;height:12pt;background-color:#0000ff"></div><span class="t" style="padding-left:13.5pt">1</span>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the document:\n%s", want, doc)
		}
	}
	if _, err := src.FS.Open("cf/3arrows-2-1.svg"); err != nil {
		t.Errorf("expected the icon in the files: %v", err)
	}
	if n := pageCount(t, src); n != 1 {
		t.Errorf("expected 1 page, got: %d", n)
	}
}
