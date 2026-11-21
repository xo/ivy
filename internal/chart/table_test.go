package chart

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
)

// segmentText returns the text of a line of rich text, with ^ before each
// superscript.
func segmentText(line []segment) string {
	var b strings.Builder
	for _, sg := range line {
		if sg.sup {
			b.WriteString("^")
		}
		b.WriteString(sg.text)
	}
	return b.String()
}

// TestEquation compares the equations and the R squared of trendlines with
// the labels that Excel 2024 drew for chart-trendline-labels.xlsx of
// xlsx/testdata.
func TestEquation(t *testing.T) {
	xs := catXs(10)
	linear := []float64{2.1, 3.9, 6.2, 7.8, 10.1, 12.2, 13.8, 16.3, 17.9, 20.4}
	curve := []float64{9.5, 6.2, 4.1, 3.2, 3.4, 4.6, 6.9, 10.2, 14.8, 19.9}
	tests := []struct {
		t      trendline
		ys     []float64
		eq, r2 string
	}{
		{trendline{kind: "linear"}, linear, "y = 2.02x - 0.04", "0.9989"},
		{trendline{kind: "poly", order: 2}, curve, "y = 0.5371x^2 - 4.7168x + 13.543", "0.9995"},
		{trendline{kind: "exp"}, xs, "y = 1.2753e^0.2304x", ""},
		{trendline{kind: "log"}, xs, "y = 3.9307ln(x) - 0.4371", "0.9057"},
		{trendline{kind: "power"}, xs, "y = x", "1"},
		{trendline{kind: "linear", intercept: 0, hasIntercept: true}, xs, "y = x", "1"},
	}
	for _, test := range tests {
		t.Run(test.t.kind, func(t *testing.T) {
			r, ok := test.t.regress(xs, test.ys)
			if !ok {
				t.Fatal("expected a fit")
			}
			if got := segmentText(equation(test.t, r)); got != test.eq {
				t.Errorf("expected %q, got %q", test.eq, got)
			}
			if test.r2 != "" {
				if got := trendNumber(r.r2(), ""); got != test.r2 {
					t.Errorf("expected R squared %s, got %s", test.r2, got)
				}
			}
		})
	}
}

func TestTrendNumber(t *testing.T) {
	tests := []struct {
		v      float64
		format string
		want   string
	}{
		{0.53712121, "", "0.5371"}, {13.5433333, "", "13.543"}, {-4.7168182, "", "-4.7168"},
		{2.02, "", "2.02"}, {123456.7, "", "123457"}, {0.000012345, "", "1.2345E-05"},
		{0, "", "0"}, {0.25, "0.0%", "25.0%"},
	}
	for _, test := range tests {
		if got := trendNumber(test.v, test.format); got != test.want {
			t.Errorf("%g: expected %q, got %q", test.v, test.want, got)
		}
	}
}

// tableChart returns a column chart of three series with a data table.
func tableChart(keys string) []byte {
	cats := strCache("A", "Q1", "Q2", "Q3", "Q4")
	var series strings.Builder
	for i, s := range []struct{ name, fill string }{{"North", "FF0000"}, {"South", "00C000"}, {"East", "0000FF"}} {
		series.WriteString(ser(i, s.name, solid(s.fill), cats, "120", "135", "150", "170"))
	}
	table := fmt.Sprintf(`<c:dTable><c:showHorzBorder val="1"/><c:showVertBorder val="1"/>`+
		`<c:showOutline val="1"/><c:showKeys val="%s"/></c:dTable>`, keys)
	return chartXML("", `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>`+series.String()+
		axisRefs+`</c:barChart>`+table, categoryAxes, "")
}

func TestDataTable(t *testing.T) {
	for _, keys := range []string{"0", "1"} {
		t.Run("keys "+keys, func(t *testing.T) {
			c, err := Parse(tableChart(keys), Options{})
			if err != nil {
				t.Fatal(err)
			}
			if c.table == nil || !c.table.horz || !c.table.vert || !c.table.outline || c.table.keys != (keys == "1") {
				t.Fatalf("expected a data table with lines, got %+v", c.table)
			}
			pm := render(t, c, 400, 300)
			cv := &canvas{c: c, t: newTexter(c.fontDatabase(), c.font), w: 400, h: 300}
			pl := &plane{cv: cv, c: c, cats: c.categories()}
			pl.n = len(pl.cats)
			if !pl.hasTable() {
				t.Fatal("expected the plot to draw the table")
			}
			// Four rows of the table below the bars: the categories and
			// the three series, with their names left of the plot.
			h := int(pl.tableHeight())
			if n := ink(pm, 0, 300-h-pad, 400, 300-pad); n < 400 {
				t.Errorf("expected the table at the bottom, got %d pixels of ink", n)
			}
			if n := ink(pm, pad, 300-h-pad, pad+int(pl.tableNames()), 300-pad); n < 60 {
				t.Errorf("expected the names of the series, got %d pixels of ink", n)
			}
			red := count(pm, color.NRGBA{255, 0, 0, 255}, 0, 300-h-pad, 60, 300-pad)
			if (keys == "1") != (red > 4) {
				t.Errorf("expected the keys %s, got %d red pixels in the names", keys, red)
			}
		})
	}
}

// trendChart returns a line chart with a linear trendline that shows its
// equation and its R squared.
func trendChart() []byte {
	cats := strCache("A", "a", "b", "c", "d")
	series := fmt.Sprintf(`<c:ser><c:idx val="0"/><c:order val="0"/><c:tx>%s</c:tx>`+
		`<c:trendline><c:name>Fit</c:name><c:trendlineType val="linear"/><c:dispRSqr val="1"/><c:dispEq val="1"/>`+
		`<c:trendlineLbl><c:numFmt formatCode="General" sourceLinked="0"/></c:trendlineLbl></c:trendline>`+
		`<c:cat>%s</c:cat><c:val>%s</c:val></c:ser>`,
		strCache("B", "S"), cats, numCache("C", "General", "2", "4", "6", "8"))
	return chartXML("", `<c:lineChart><c:grouping val="standard"/>`+series+axisRefs+`</c:lineChart>`, categoryAxes, "r")
}

func TestTrendLabel(t *testing.T) {
	c, err := Parse(trendChart(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	tr := c.plots[0].series[0].trends[0]
	if !tr.eq || !tr.r2 || tr.name != "Fit" || tr.format != "" {
		t.Fatalf("expected the equation, R squared and the name Fit, got %+v", tr)
	}
	cv := &canvas{c: c, t: newTexter(c.fontDatabase(), c.font), w: 400, h: 300}
	es := cv.entries()
	if len(es) != 2 || es[1].text != "Fit" || !es[1].line {
		t.Errorf("expected the trendline in the legend, got %d entries", len(es))
	}
	// The label, y = 2x and R² = 1, changes the drawing.
	plain := strings.Replace(string(trendChart()), `<c:dispRSqr val="1"/><c:dispEq val="1"/>`, "", 1)
	without, err := Parse([]byte(plain), Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, b := render(t, c, 400, 300), render(t, without, 400, 300)
	diff := 0
	for y := range 300 {
		for x := range 400 {
			if at(a, x, y) != at(b, x, y) {
				diff++
			}
		}
	}
	if diff < 50 {
		t.Errorf("expected the label of the trendline, got %d pixels of difference", diff)
	}
}
