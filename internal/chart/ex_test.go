package chart

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"testing"
)

// exXML returns a chart of Office 2016 with one series of the layout
// layout, its categories and its values, and the parts parts after the
// series.
func exXML(title, layout, parts string, cats []string, vals []float64) []byte {
	var b strings.Builder
	b.WriteString(`<cx:chartSpace xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:cx="http://schemas.microsoft.com/office/drawing/2014/chartex"><cx:chartData><cx:data id="0">`)
	fmt.Fprintf(&b, `<cx:strDim type="cat"><cx:f>A</cx:f><cx:lvl ptCount="%d">`, len(cats))
	for i, c := range cats {
		fmt.Fprintf(&b, `<cx:pt idx="%d">%s</cx:pt>`, i, c)
	}
	fmt.Fprintf(&b, `</cx:lvl></cx:strDim><cx:numDim type="val"><cx:f>B</cx:f><cx:lvl ptCount="%d" formatCode="General">`, len(vals))
	for i, v := range vals {
		fmt.Fprintf(&b, `<cx:pt idx="%d">%g</cx:pt>`, i, v)
	}
	b.WriteString(`</cx:lvl></cx:numDim></cx:data></cx:chartData><cx:chart>`)
	if title != "" {
		b.WriteString(`<cx:title pos="t" align="ctr" overlay="0"><cx:tx><cx:txData><cx:v>` + title +
			`</cx:v></cx:txData></cx:tx></cx:title>`)
	}
	b.WriteString(`<cx:plotArea><cx:plotAreaRegion><cx:series layoutId="` + layout + `" uniqueId="{1}">` +
		`<cx:tx><cx:txData><cx:v>S</cx:v></cx:txData></cx:tx><cx:dataId val="0"/>` + parts + `</cx:series>` +
		`</cx:plotAreaRegion><cx:axis id="0"><cx:catScaling gapWidth="0.5"/><cx:tickLabels/></cx:axis>` +
		`<cx:axis id="1"><cx:valScaling/><cx:majorGridlines/><cx:tickLabels/></cx:axis></cx:plotArea>` +
		`</cx:chart></cx:chartSpace>`)
	return []byte(b.String())
}

func TestChartEx(t *testing.T) {
	theme := testTheme(t)
	red, green, blue := color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0xc0, 0, 255}, color.NRGBA{0, 0, 255, 255}
	tests := []struct {
		name, layout, parts string
		cats                []string
		vals                []float64
		special             string
		colors              []color.NRGBA
	}{
		{name: "funnel", layout: "funnel", cats: []string{"a", "b", "c"}, vals: []float64{10, 6, 3},
			special: "funnel", colors: []color.NRGBA{red}},
		{name: "waterfall", layout: "waterfall", parts: `<cx:layoutPr><cx:subtotals><cx:idx val="3"/></cx:subtotals></cx:layoutPr>`,
			cats: []string{"a", "b", "c", "total"}, vals: []float64{10, -4, 3, 9},
			special: "waterfall", colors: []color.NRGBA{red, green, blue}},
		{name: "histogram", layout: "clusteredColumn", parts: `<cx:layoutPr><cx:binning intervalClosed="r"><cx:binCount val="3"/></cx:binning></cx:layoutPr>`,
			vals: []float64{1, 2, 2, 3, 3, 3, 7, 8, 9}, colors: []color.NRGBA{red}},
		{name: "box", layout: "boxWhisker", cats: []string{"x", "x", "x", "x", "y", "y", "y", "y"},
			vals: []float64{1, 2, 3, 4, 5, 6, 7, 40}, colors: []color.NRGBA{red}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := Parse(exXML("Title", test.layout, test.parts, test.cats, test.vals), Options{Theme: theme})
			if err != nil {
				t.Fatal(err)
			}
			if c.look != lookModern || len(c.plots) != 1 || c.plots[0].special != test.special {
				t.Fatalf("expected one plot of %q, got %d plots", test.special, len(c.plots))
			}
			pm := render(t, c, 400, 300)
			for _, col := range test.colors {
				if n := count(pm, col, 0, 0, 400, 300); n < 40 {
					t.Errorf("expected the color %v, got %d pixels", col, n)
				}
			}
			if n := ink(pm, 100, 0, 300, 30); n < 20 {
				t.Errorf("expected the ink of the title, got %d pixels", n)
			}
		})
	}
}

func TestHistogram(t *testing.T) {
	c, err := Parse(exXML("", "clusteredColumn", `<cx:layoutPr><cx:binning intervalClosed="r"><cx:binSize val="3"/></cx:binning></cx:layoutPr>`,
		nil, []float64{1, 2, 2, 3, 3, 3, 7, 8, 9}), Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := c.plots[0].series[0]
	exp := []string{"[1, 4]", "(4, 7]", "(7, 10]"}
	if strings.Join(s.cats, "|") != strings.Join(exp, "|") || len(s.vals) != 3 || s.vals[0] != 6 || s.vals[1] != 1 || s.vals[2] != 2 {
		t.Errorf("expected the bins %q with 6, 1 and 2 values, got %q and %v", exp, s.cats, s.vals)
	}
}

func TestQuartile(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	if q := quartile(xs, 0.25, false); q != 2.25 {
		t.Errorf("expected the first quartile 2.25 of QUARTILE.EXC, got %g", q)
	}
	if q := quartile(xs, 0.25, true); q != 2.75 {
		t.Errorf("expected the first quartile 2.75 of QUARTILE.INC, got %g", q)
	}
	if q := quartile(xs, 0.5, false); q != 4.5 {
		t.Errorf("expected the median 4.5, got %g", q)
	}
}

func TestChartExFrame(t *testing.T) {
	for _, layout := range []string{"regionMap"} {
		c, err := Parse(exXML("Map", layout, "", []string{"a", "b"}, []float64{1, 2}), Options{})
		if err != nil {
			t.Fatal(err)
		}
		if c.unsupported == "" {
			t.Errorf("%s: expected a frame", layout)
		}
		pm := render(t, c, 300, 200)
		if n := ink(pm, 50, 0, 250, 30); n < 20 {
			t.Errorf("%s: expected the ink of the title, got %d pixels", layout, n)
		}
	}
}

func TestRadar(t *testing.T) {
	cats := strCache("A", "a", "b", "c", "d", "e")
	xml := chartXML("", `<c:radarChart><c:radarStyle val="marker"/>`+
		ser(0, "One", "", cats, "4", "3", "5", "2", "4")+ser(1, "Two", "", cats, "1", "2", "1", "3", "2")+
		axisRefs+`</c:radarChart>`, categoryAxes, "r")
	c, err := Parse(xml, Options{Theme: testTheme(t)})
	if err != nil {
		t.Fatal(err)
	}
	if c.plots[0].kind != kindRadar {
		t.Fatalf("expected a radar chart, got the kind %d", c.plots[0].kind)
	}
	pm := render(t, c, 300, 300)
	for _, col := range []color.NRGBA{{255, 0, 0, 255}, {0, 0xc0, 0, 255}} {
		if n := count(pm, col, 0, 0, 240, 300); n < 40 {
			t.Errorf("expected the color %v, got %d pixels", col, n)
		}
	}
}

func TestStock(t *testing.T) {
	cats := strCache("A", "d1", "d2", "d3")
	noLine := `<c:spPr><a:ln><a:noFill/></a:ln></c:spPr>`
	xml := chartXML("", `<c:stockChart>`+
		ser(0, "High", noLine, cats, "9", "8", "7")+ser(1, "Low", noLine, cats, "2", "3", "1")+
		ser(2, "Close", noLine, cats, "5", "6", "4")+`<c:hiLowLines/>`+axisRefs+`</c:stockChart>`, categoryAxes, "")
	c, err := Parse(xml, Options{Theme: testTheme(t), Width: 150, Height: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !c.plots[0].hiLow {
		t.Fatal("expected the high-low lines")
	}
	pm := render(t, c, 600, 400)
	// The high-low lines are black and vertical, in the middle of each
	// category.
	if n := count(pm, color.NRGBA{0, 0, 0, 255}, 0, 0, 600, 400); n < 300 {
		t.Errorf("expected the high-low lines, got %d pixels", n)
	}
}

func TestSecondaryAxis(t *testing.T) {
	cats := strCache("A", "x", "y", "z")
	xml := chartXML("", `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>`+
		ser(0, "Bars", solid("0000FF"), cats, "3", "5", "4")+axisRefs+`</c:barChart>`+
		`<c:lineChart><c:grouping val="standard"/>`+ser(1, "Line", "", cats, "1000", "3000", "2000")+
		`<c:axId val="3"/><c:axId val="4"/></c:lineChart>`,
		categoryAxes+`<c:catAx><c:axId val="3"/><c:delete val="1"/><c:axPos val="b"/><c:crossAx val="4"/></c:catAx>`+
			`<c:valAx><c:axId val="4"/><c:delete val="0"/><c:axPos val="r"/><c:crosses val="max"/><c:crossAx val="3"/></c:valAx>`, "")
	c, err := Parse(xml, Options{Theme: testTheme(t)})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.plots) != 2 || !c.plots[1].secondary || !c.val2.present {
		t.Fatalf("expected a line on the secondary axis, got %d plots", len(c.plots))
	}
	pm := render(t, c, 400, 300)
	// The labels of the secondary axis are at the right.
	if n := ink(pm, 360, 20, 400, 280); n < 30 {
		t.Errorf("expected the labels of the secondary axis, got %d pixels", n)
	}
	if n := count(pm, color.NRGBA{0, 0xc0, 0, 255}, 0, 0, 400, 300); n < 40 {
		t.Errorf("expected the line, got %d pixels", n)
	}
}

func TestPointLabels(t *testing.T) {
	cats := strCache("A", "a", "b")
	dLbls := `<c:dLbls><c:dLbl><c:idx val="1"/><c:layout><c:manualLayout><c:x val="0.1"/><c:y val="0.05"/></c:manualLayout></c:layout>` +
		`<c:tx><c:rich><a:bodyPr/><a:p><a:r><a:t>Own text</a:t></a:r></a:p></c:rich></c:tx><c:showVal val="1"/></c:dLbl>` +
		`<c:dLbl><c:idx val="0"/><c:delete val="1"/></c:dLbl><c:showVal val="1"/></c:dLbls>`
	xml := chartXML("", `<c:pieChart><c:varyColors val="1"/>`+
		strings.Replace(ser(0, "S", "", cats, "1", "3"), "</c:ser>", dLbls+"</c:ser>", 1)+`</c:pieChart>`, "", "")
	c, err := Parse(xml, Options{})
	if err != nil {
		t.Fatal(err)
	}
	l := c.plots[0].series[0].labels
	pt := l.points[1]
	if pt == nil || !pt.hasText || pt.lines[0] != "Own text" || pt.dx != 0.1 || !l.points[0].deleted {
		t.Fatalf("expected the labels of the points, got %+v", l.points)
	}
	tg, ok := tagOf(l, 1, func(labels) string { return "auto" })
	if !ok || tg.lines[0] != "Own text" || tg.dy != 0.05 {
		t.Errorf("expected the own text of the point, got %+v", tg)
	}
	if _, ok := tagOf(l, 0, func(labels) string { return "auto" }); ok {
		t.Error("expected no label on a deleted point")
	}
	render(t, c, 300, 200)
}

func TestPie3D(t *testing.T) {
	xml := chartXML("", `<c:pie3DChart><c:varyColors val="1"/>`+
		ser(0, "S", "", strCache("A", "a", "b"), "1", "1")+`</c:pie3DChart>`, "", "")
	c, err := Parse(xml, Options{Theme: testTheme(t)})
	if err != nil {
		t.Fatal(err)
	}
	pm := render(t, c, 300, 300)
	// The pie is an ellipse of half the height of its width, at the
	// angle of view of 30 degrees.
	red := color.NRGBA{255, 0, 0, 255}
	if n := count(pm, red, 0, 0, 300, 75); n > 0 {
		t.Errorf("expected nothing at the top, got %d pixels of red", n)
	}
	if n := count(pm, red, 0, 0, 300, 300); n < 1000 {
		t.Errorf("expected the first slice, got %d pixels", n)
	}
}

func TestODFSheetRange(t *testing.T) {
	doc := strings.Replace(odfChart, `chart:values-cell-range-address="local-table.B2:.B4"`,
		`chart:values-cell-range-address="'My Sheet'.$D$5:'My Sheet'.$D$7"`, 1)
	doc = strings.Replace(doc, `<table:table-cell office:value-type="float" office:value="3"/>`,
		`<table:table-cell office:value-type="float" office:value="3"><draw:g><svg:desc>'My Sheet'.$D$5:'My Sheet'.$D$7</svg:desc></draw:g></table:table-cell>`, 1)
	c, err := ParseODF([]byte(doc), Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := c.plots[0].series[0]
	if len(s.vals) != 3 || s.vals[0] != 3 || s.vals[1] != 5 || s.vals[2] != 1 {
		t.Errorf("expected the values of the range of the sheet, got %v", s.vals)
	}
	if got := splitRanges("'A B'.C1:'A B'.C2 'D'.E1"); len(got) != 2 || got[0] != "'A B'.C1:'A B'.C2" {
		t.Errorf("expected two ranges, got %q", got)
	}
}

func TestLabelText(t *testing.T) {
	l := labels{value: true, category: true, percent: true, format: "0.0"}
	if got := labelText(l, "S", "Cat", 1.25, 0.5, ""); got != "Cat, 1.3, 50%" {
		t.Errorf("expected the parts of the label, got %q", got)
	}
	l.separator, l.hasSep = "\n", true
	if got := labelText(l, "S", "Cat", 2, math.NaN(), ""); got != "Cat\n2.0" {
		t.Errorf("expected the parts on their own lines, got %q", got)
	}
}

// TestFunnelSeries checks that a funnel draws its first series only, with
// the gap of its axis and without labels that the series does not ask
// for, and that a hidden series draws nothing.
func TestFunnelSeries(t *testing.T) {
	buf := string(exXML("", "funnel", "", []string{"a", "b"}, []float64{10, 6}))
	buf = strings.Replace(buf, `</cx:series></cx:plotAreaRegion>`, `</cx:series>`+
		`<cx:series layoutId="funnel" hidden="1"><cx:tx><cx:txData><cx:v>Hidden</cx:v></cx:txData></cx:tx><cx:dataId val="0"/></cx:series>`+
		`<cx:series layoutId="funnel"><cx:tx><cx:txData><cx:v>Second</cx:v></cx:txData></cx:tx><cx:dataId val="0"/></cx:series></cx:plotAreaRegion>`, 1)
	c, err := Parse([]byte(buf), Options{Theme: testTheme(t)})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.plots) != 1 || len(c.plots[0].series) != 1 || c.plots[0].series[0].name != "S" {
		t.Fatalf("expected one plot of the first series, got: %d plots", len(c.plots))
	}
	pl := c.plots[0]
	if pl.gap != 50 {
		t.Errorf("expected the gap of 50%% of the axis, got: %v", pl.gap)
	}
	if pl.series[0].labels.set {
		t.Error("expected no labels")
	}
	// a hidden series of another layout draws nothing either
	buf = strings.Replace(string(exXML("", "waterfall", "", []string{"a"}, []float64{1})), `<cx:series layoutId="waterfall"`, `<cx:series layoutId="waterfall" hidden="1"`, 1)
	if c, err = Parse([]byte(buf), Options{Theme: testTheme(t)}); err != nil {
		t.Fatal(err)
	}
	if len(c.plots) != 0 {
		t.Errorf("expected no plot, got: %d", len(c.plots))
	}
}

// TestChartExStyle checks that a chart of Office 2016 without cx:spPr takes
// the fill and the line of its chart area from its style part.
func TestChartExStyle(t *testing.T) {
	style := `<cs:chartStyle xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:cs="http://schemas.microsoft.com/office/drawing/2012/chartStyle" id="381">` +
		`<cs:chartArea mods="allowNoFillOverride allowNoLineOverride"><cs:lnRef idx="0"/><cs:fillRef idx="0"/>` +
		`<cs:spPr><a:solidFill><a:schemeClr val="bg1"/></a:solidFill><a:ln w="9525"><a:solidFill>` +
		`<a:srgbClr val="D9D9D9"/></a:solidFill></a:ln></cs:spPr></cs:chartArea></cs:chartStyle>`
	buf := exXML("Title", "funnel", "", []string{"a", "b"}, []float64{2, 1})
	white, gray := color.NRGBA{255, 255, 255, 255}, color.NRGBA{0xd9, 0xd9, 0xd9, 255}
	tests := []struct {
		name         string
		buf          []byte
		style        string
		fill, border color.NRGBA
	}{
		{"style", buf, style, white, gray},
		{"no style", buf, "", color.NRGBA{}, color.NRGBA{}},
		{"broken style", buf, "<cs:chartStyle", color.NRGBA{}, color.NRGBA{}},
		{"own spPr", []byte(strings.Replace(string(buf), `</cx:chart>`,
			`</cx:chart><cx:spPr><a:noFill/><a:ln><a:noFill/></a:ln></cx:spPr>`, 1)),
			style, color.NRGBA{}, color.NRGBA{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts := Options{Theme: testTheme(t)}
			if test.style != "" {
				opts.Style = func() []byte { return []byte(test.style) }
			}
			c, err := Parse(test.buf, opts)
			if err != nil {
				t.Fatal(err)
			}
			if c.fill != test.fill || c.border != test.border {
				t.Errorf("expected the fill %v and the border %v, got %v and %v", test.fill, test.border, c.fill, c.border)
			}
		})
	}
}
