package chart

import (
	"fmt"
	"image/color"
	"math"
	"testing"
)

func TestTrendlineFit(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	tests := []struct {
		name string
		t    trendline
		f    func(float64) float64
	}{
		{"linear", trendline{kind: "linear"}, func(x float64) float64 { return 2*x + 1 }},
		{"poly", trendline{kind: "poly", order: 2}, func(x float64) float64 { return x*x - 3*x + 2 }},
		{"exp", trendline{kind: "exp"}, func(x float64) float64 { return 3 * math.Exp(0.5*x) }},
		{"log", trendline{kind: "log"}, func(x float64) float64 { return 1 + 2*math.Log(x) }},
		{"power", trendline{kind: "power"}, func(x float64) float64 { return 2 * math.Pow(x, 1.5) }},
		// a fixed intercept of 1 through the points of 2x+1
		{"intercept", trendline{kind: "linear", intercept: 1, hasIntercept: true}, func(x float64) float64 { return 2*x + 1 }},
	}
	for _, test := range tests {
		ys := make([]float64, len(xs))
		for i, x := range xs {
			ys[i] = test.f(x)
		}
		f, ok := test.t.fit(xs, ys)
		if !ok {
			t.Errorf("%s: expected a fit", test.name)
			continue
		}
		for _, x := range []float64{1.5, 4, 7} {
			if got, exp := f(x), test.f(x); math.Abs(got-exp) > 1e-6*max(1, math.Abs(exp)) {
				t.Errorf("%s: expected %g at %g, got: %g", test.name, exp, x, got)
			}
		}
	}
	if _, ok := (trendline{kind: "linear"}).fit([]float64{1}, []float64{1}); ok {
		t.Error("expected no fit of one point")
	}
	if _, ok := (trendline{kind: "exp"}).fit(xs, []float64{-1, -2, -3, -4, -5}); ok {
		t.Error("expected no exponential fit of negative values")
	}
}

func TestErrorRange(t *testing.T) {
	vals := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	tests := []struct {
		b      errBars
		lo, hi float64
	}{
		{errBars{kind: "both", valType: "fixedVal", val: 1}, 3, 5},
		{errBars{kind: "plus", valType: "percentage", val: 50}, 4, 6},
		{errBars{kind: "minus", valType: "percentage", val: 50}, 2, 4},
		// the standard deviation of the sample is 2.138, around the mean
		// of 5
		{errBars{kind: "both", valType: "stdDev", val: 1}, 5 - 2.13809, 5 + 2.13809},
		{errBars{kind: "both", valType: "stdErr"}, 4 - 2.13809/math.Sqrt(8), 4 + 2.13809/math.Sqrt(8)},
		{errBars{kind: "both", valType: "cust", plus: []float64{9, 1}, minus: []float64{9, 2}}, 2, 5},
	}
	for _, test := range tests {
		lo, hi, ok := test.b.errorRange(vals, 1, 4)
		if !ok || math.Abs(lo-test.lo) > 1e-4 || math.Abs(hi-test.hi) > 1e-4 {
			t.Errorf("%s %s: expected %g to %g, got: %g to %g, %v", test.b.valType, test.b.kind, test.lo, test.hi, lo, hi, ok)
		}
	}
}

func TestTrendlineErrBarsDropLines(t *testing.T) {
	cats := strCache("A", "a", "b", "c", "d")
	red := `<c:spPr><a:ln w="38100"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:ln></c:spPr>`
	green := `<c:spPr><a:ln w="38100"><a:solidFill><a:srgbClr val="00C000"/></a:solidFill></a:ln></c:spPr>`
	blue := `<c:spPr><a:ln w="38100"><a:solidFill><a:srgbClr val="0000FF"/></a:solidFill></a:ln></c:spPr>`
	series := fmt.Sprintf(`<c:ser><c:idx val="0"/><c:order val="0"/><c:tx>%s</c:tx>`+
		`<c:spPr><a:ln><a:noFill/></a:ln></c:spPr><c:marker><c:symbol val="none"/></c:marker>`+
		`<c:trendline>%s<c:trendlineType val="linear"/></c:trendline>`+
		`<c:errBars><c:errDir val="y"/><c:errBarType val="both"/><c:errValType val="fixedVal"/><c:val val="1"/>%s</c:errBars>`+
		`<c:cat>%s</c:cat><c:val>%s</c:val></c:ser>`,
		strCache("B", "S"), red, green, cats, numCache("C", "General", "2", "4", "6", "8"))
	xml := chartXML("", `<c:lineChart><c:grouping val="standard"/>`+series+`<c:dropLines>`+blue+`</c:dropLines>`+
		`<c:marker val="1"/>`+axisRefs+`</c:lineChart>`, categoryAxes, "")
	c, err := Parse(xml, Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := c.plots[0].series[0]
	if len(s.trends) != 1 || len(s.errs) != 1 || c.plots[0].drop == nil {
		t.Fatalf("expected a trendline, error bars and drop lines, got: %d, %d, %v", len(s.trends), len(s.errs), c.plots[0].drop)
	}
	pm := render(t, c, 400, 300)
	for _, col := range []color.NRGBA{{255, 0, 0, 255}, {0, 0xc0, 0, 255}, {0, 0, 255, 255}} {
		if n := count(pm, col, 0, 0, 400, 300); n < 40 {
			t.Errorf("expected the color %v, got %d pixels", col, n)
		}
	}
}

func TestDatePlaces(t *testing.T) {
	dateChart := func(unit string, dates ...string) *Chart {
		t.Helper()
		vals := make([]string, len(dates))
		for i := range vals {
			vals[i] = fmt.Sprint(i + 1)
		}
		base := ""
		if unit != "" {
			base = `<c:baseTimeUnit val="` + unit + `"/>`
		}
		xml := chartXML("", `<c:lineChart><c:grouping val="standard"/>`+
			`<c:ser><c:idx val="0"/><c:order val="0"/><c:cat>`+numCache("A", "General", dates...)+`</c:cat>`+
			`<c:val>`+numCache("B", "General", vals...)+`</c:val></c:ser>`+axisRefs+`</c:lineChart>`,
			`<c:dateAx><c:axId val="1"/><c:axPos val="b"/><c:numFmt formatCode="d mmm yyyy" sourceLinked="0"/>`+base+`</c:dateAx>`+
				`<c:valAx><c:axId val="2"/><c:axPos val="l"/></c:valAx>`, "")
		c, err := Parse(xml, Options{})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	// 15, 17 and 20 March 2023, out of order: six days with gaps
	s := dateChart("", "45000", "45005", "45002").plots[0].series[0]
	if len(s.cats) != 6 || s.cats[0] != "15 Mar 2023" || s.cats[5] != "20 Mar 2023" || !s.span {
		t.Errorf("expected six days from 15 March, got: %q", s.cats)
	}
	for i, exp := range []float64{1, math.NaN(), 3, math.NaN(), math.NaN(), 2} {
		if got := s.vals[i]; got != exp && !(math.IsNaN(got) && math.IsNaN(exp)) {
			t.Errorf("expected %v at day %d, got: %v", exp, i, got)
		}
	}
	// months: 15 March and 1 June 2023
	s = dateChart("months", "45000", "45078").plots[0].series[0]
	if len(s.cats) != 4 || s.cats[0] != "1 Mar 2023" || s.cats[3] != "1 Jun 2023" {
		t.Errorf("expected four months from March, got: %q", s.cats)
	}
	if got := serialOf(dateOf(45000)); got != 45000 {
		t.Errorf("expected the serial 45000 back, got: %v", got)
	}
	if got := serialOf(dateOf(59)); got != 59 {
		t.Errorf("expected the serial 59 back, got: %v", got)
	}
}
