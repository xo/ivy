package chart

import (
	"image/color"
	"math"
	"testing"

	"github.com/xo/okage/svg"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fontdb"
)

func TestCombination(t *testing.T) {
	cats := strCache("A", "x", "y", "z")
	xml := chartXML("Combo", `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>`+
		ser(0, "Bars", solid("0000FF"), cats, "3", "5", "4")+axisRefs+`</c:barChart>`+
		`<c:lineChart><c:grouping val="standard"/>`+
		`<c:ser><c:idx val="1"/><c:order val="1"/><c:tx>`+strCache("B", "Line")+`</c:tx>`+
		`<c:spPr><a:ln w="28575"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:ln></c:spPr>`+
		`<c:marker><c:symbol val="none"/></c:marker>`+
		`<c:cat>`+cats+`</c:cat><c:val>`+numCache("C", "General", "1", "2", "6")+`</c:val></c:ser>`+
		`<c:marker val="1"/>`+axisRefs+`</c:lineChart>`, categoryAxes, "r")
	c, err := Parse(xml, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.plots) != 2 {
		t.Fatalf("expected two plots, got %d", len(c.plots))
	}
	pm := render(t, c, 400, 300)
	for _, col := range []color.NRGBA{{0, 0, 255, 255}, {255, 0, 0, 255}} {
		if n := count(pm, col, 0, 0, 400, 300); n < 40 {
			t.Errorf("expected the color %v, got %d pixels", col, n)
		}
	}
}

func TestDoughnut(t *testing.T) {
	xml := chartXML("", `<c:doughnutChart><c:varyColors val="1"/>`+
		ser(0, "S", "", strCache("A", "a", "b"), "1", "1")+
		`<c:firstSliceAng val="90"/><c:holeSize val="60"/></c:doughnutChart>`, "", "")
	c, err := Parse(xml, Options{Theme: testTheme(t)})
	if err != nil {
		t.Fatal(err)
	}
	pm := render(t, c, 300, 300)
	// The hole is empty, and the first slice starts at three o'clock, so
	// it covers the bottom half.
	if p := at(pm, 150, 150); p != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("expected the hole at the center, got: %v", p)
	}
	red, green := color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0xc0, 0, 255}
	if n := count(pm, red, 0, 160, 300, 300); n < 1000 {
		t.Errorf("expected the first slice at the bottom, got %d pixels", n)
	}
	if n := count(pm, green, 0, 0, 300, 140); n < 1000 {
		t.Errorf("expected the second slice at the top, got %d pixels", n)
	}
}

func TestBubble(t *testing.T) {
	xml := chartXML("", `<c:bubbleChart><c:varyColors val="0"/>`+
		`<c:ser><c:idx val="0"/><c:order val="0"/>`+
		`<c:xVal>`+numCache("A", "General", "1", "2", "3")+`</c:xVal>`+
		`<c:yVal>`+numCache("B", "General", "1", "3", "2")+`</c:yVal>`+
		`<c:bubbleSize>`+numCache("C", "General", "1", "4", "9")+`</c:bubbleSize></c:ser>`+
		`<c:axId val="1"/><c:axId val="2"/></c:bubbleChart>`,
		`<c:valAx><c:axId val="1"/><c:axPos val="b"/></c:valAx><c:valAx><c:axId val="2"/><c:axPos val="l"/></c:valAx>`, "")
	c, err := Parse(xml, Options{Theme: testTheme(t)})
	if err != nil {
		t.Fatal(err)
	}
	if !c.plots[0].bubbles || len(c.plots[0].series[0].sizes) != 3 {
		t.Fatalf("expected a bubble chart with sizes, got: %+v", c.plots[0])
	}
	pm := render(t, c, 300, 300)
	if n := count(pm, color.NRGBA{255, 0, 0, 255}, 0, 0, 300, 300); n < 200 {
		t.Errorf("expected the bubbles, got %d pixels", n)
	}
}

func TestDateAxis(t *testing.T) {
	xml := chartXML("", `<c:lineChart><c:grouping val="standard"/>`+
		`<c:ser><c:idx val="0"/><c:order val="0"/><c:cat>`+numCache("A", "General", "45000", "45001")+`</c:cat>`+
		`<c:val>`+numCache("B", "General", "1", "2")+`</c:val></c:ser>`+axisRefs+`</c:lineChart>`,
		`<c:dateAx><c:axId val="1"/><c:axPos val="b"/><c:numFmt formatCode="d mmm" sourceLinked="0"/></c:dateAx>`+
			`<c:valAx><c:axId val="2"/><c:axPos val="l"/></c:valAx>`, "")
	c, err := Parse(xml, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if cats := c.plots[0].series[0].cats; len(cats) != 2 || cats[0] != "15 Mar" || cats[1] != "16 Mar" {
		t.Errorf("expected the dates in the format of the axis, got: %q", cats)
	}
}

func TestFamily(t *testing.T) {
	db := svg.NewFontDatabase()
	db.LoadFontData(goregular.TTF)
	c, err := Parse(testCharts()[0].xml, Options{Fonts: db})
	if err != nil {
		t.Fatal(err)
	}
	if c.font != "Calibri" {
		t.Errorf("expected the font of the theme of Office, got %q", c.font)
	}
	// The database has neither Calibri nor Carlito, and no sans serif
	// family, so the chart draws with its first family.
	if f := c.family(db); f != "Go" {
		t.Errorf("expected the family Go, got %q", f)
	}
	// Without Options.Fonts, the chart draws with the fonts of the
	// program: the Go fonts by default.
	if f := c.family(fontdb.Of(ivy.Config{})); f != "Go" {
		t.Errorf("expected the sans serif family Go, got %q", f)
	}
	pm := render(t, c, 300, 200)
	if n := ink(pm, 75, 0, 225, 30); n < 30 {
		t.Errorf("expected the ink of the title in the font Go, got %d pixels", n)
	}
}

// TestKeepInside checks that a label of a pie that its place puts out of
// the chart moves into the chart, as Word does, with the gap of
// pieLabelMargin, and that a label inside the chart keeps its place.
func TestKeepInside(t *testing.T) {
	labels := []pieLabel{
		// at the left of the pie, aligned by its right side
		{w: 60, h: 20, mid: -math.Pi / 2, x: 40, y: 100},
		// at the right of the pie, aligned by its left side
		{w: 60, h: 20, mid: math.Pi / 2, x: 280, y: 195},
		// in a slice, centered
		{w: 30, h: 10, inside: true, x: 150, y: 100},
	}
	keepInside(labels, 300, 200)
	if x0 := labels[0].x - labels[0].w; x0 != pieLabelMargin || labels[0].y != 100 {
		t.Errorf("expected the left label at the margin, got the left side %g and y %g", x0, labels[0].y)
	}
	if x1, y1 := labels[1].x+labels[1].w, labels[1].y+labels[1].h/2; x1 != 300-pieLabelMargin || y1 != 200-pieLabelMargin {
		t.Errorf("expected the right label at the margins, got the right side %g and the bottom %g", x1, y1)
	}
	if labels[2].x != 150 || labels[2].y != 100 {
		t.Errorf("expected the label in its slice to keep its place, got (%g, %g)", labels[2].x, labels[2].y)
	}
}
