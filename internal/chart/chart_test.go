package chart

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
)

// update writes the golden specs in testdata.
var update = flag.Bool("update", false, "write the golden specs in testdata")

// The namespaces of the charts of the tests.
const chartNS = `xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" ` +
	`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`

// themeXML is a theme whose accents are pure colors, so that a test finds
// them in the pixels.
const themeXML = `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Test">
<a:themeElements>
<a:clrScheme name="Test">
<a:dk1><a:srgbClr val="000000"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
<a:dk2><a:srgbClr val="1F497D"/></a:dk2><a:lt2><a:srgbClr val="EEECE1"/></a:lt2>
<a:accent1><a:srgbClr val="FF0000"/></a:accent1><a:accent2><a:srgbClr val="00C000"/></a:accent2>
<a:accent3><a:srgbClr val="0000FF"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4>
<a:accent5><a:srgbClr val="C000C0"/></a:accent5><a:accent6><a:srgbClr val="00C0C0"/></a:accent6>
</a:clrScheme>
<a:fontScheme name="Test">
<a:majorFont><a:latin typeface="Calibri Light"/></a:majorFont>
<a:minorFont><a:latin typeface="Calibri"/></a:minorFont>
</a:fontScheme>
</a:themeElements>
</a:theme>`

// testTheme returns the theme of themeXML.
func testTheme(t *testing.T) *drawingml.Theme {
	t.Helper()
	doc, err := roxmltree.Parse(themeXML)
	if err != nil {
		t.Fatal(err)
	}
	theme, err := drawingml.ParseTheme(doc)
	if err != nil {
		t.Fatal(err)
	}
	return theme
}

// chartXML returns a chart part with a title, a plot area and a legend.
func chartXML(title, plot, axes, legend string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<c:chartSpace ` + chartNS + `><c:chart>`)
	if title != "" {
		b.WriteString(`<c:title><c:tx><c:rich><a:bodyPr/><a:p><a:pPr><a:defRPr sz="1400" b="1"/></a:pPr>` +
			`<a:r><a:t>` + title + `</a:t></a:r></a:p></c:rich></c:tx><c:overlay val="0"/></c:title>` +
			`<c:autoTitleDeleted val="0"/>`)
	} else {
		b.WriteString(`<c:autoTitleDeleted val="1"/>`)
	}
	b.WriteString(`<c:plotArea><c:layout/>` + plot + axes + `</c:plotArea>`)
	if legend != "" {
		b.WriteString(`<c:legend><c:legendPos val="` + legend + `"/><c:overlay val="0"/></c:legend>`)
	}
	b.WriteString(`<c:plotVisOnly val="1"/><c:dispBlanksAs val="gap"/></c:chart></c:chartSpace>`)
	return []byte(b.String())
}

// strCache returns a c:strRef with its cache.
func strCache(ref string, values ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<c:strRef><c:f>%s</c:f><c:strCache><c:ptCount val="%d"/>`, ref, len(values))
	for i, v := range values {
		fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, v)
	}
	b.WriteString(`</c:strCache></c:strRef>`)
	return b.String()
}

// numCache returns a c:numRef with its cache. An empty value is missing.
func numCache(ref, format string, values ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<c:numRef><c:f>%s</c:f><c:numCache><c:formatCode>%s</c:formatCode><c:ptCount val="%d"/>`,
		ref, format, len(values))
	for i, v := range values {
		if v != "" {
			fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, v)
		}
	}
	b.WriteString(`</c:numCache></c:numRef>`)
	return b.String()
}

// ser returns a series of categories with a name, a fill and values.
func ser(idx int, name, spPr, cats string, values ...string) string {
	return fmt.Sprintf(`<c:ser><c:idx val="%d"/><c:order val="%d"/><c:tx>%s</c:tx>%s<c:cat>%s</c:cat><c:val>%s</c:val></c:ser>`,
		idx, idx, strCache("Sheet1!$A$1", name), spPr, cats, numCache("Sheet1!$B$2:$B$9", "General", values...))
}

// solid returns a c:spPr with a solid fill.
func solid(hex string) string {
	return `<c:spPr><a:solidFill><a:srgbClr val="` + hex + `"/></a:solidFill></c:spPr>`
}

// categoryAxes are the axes of a chart of categories, with grid lines on
// the values.
const categoryAxes = `<c:catAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling>` +
	`<c:delete val="0"/><c:axPos val="b"/><c:crossAx val="2"/></c:catAx>` +
	`<c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/>` +
	`<c:axPos val="l"/><c:majorGridlines/><c:numFmt formatCode="General" sourceLinked="1"/><c:crossAx val="1"/></c:valAx>`

// axisIDs are the axes of a chart type.
const axisIDs = `<c:axId val="1"/><c:axId val="2"/>`

// testChart is a chart of the tests and what its drawing must show.
type testChart struct {
	name string
	xml  []byte
	// colors are the colors that the plot must show.
	colors []color.NRGBA
}

// testCharts returns the charts of the tests.
func testCharts() []testChart {
	quarters := strCache("Sheet1!$A$2:$A$4", "Q1", "Q2", "Q3")
	red, green, blue := color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0xc0, 0, 255}, color.NRGBA{0, 0, 255, 255}
	orange, purple := color.NRGBA{0xff, 0xc0, 0, 255}, color.NRGBA{0xc0, 0, 0xc0, 255}
	return []testChart{{
		name: "clustered",
		xml: chartXML("Sales by quarter", `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>`+
			`<c:varyColors val="0"/>`+
			ser(0, "East", solid("1F77B4"), quarters, "10", "14", "-3")+
			ser(1, "West", solid("FF7F0E"), quarters, "12", "8", "9")+
			`<c:dLbls><c:showLegendKey val="0"/><c:showVal val="1"/></c:dLbls><c:gapWidth val="150"/>`+
			axisIDs+`</c:barChart>`, categoryAxes, "b"),
		colors: []color.NRGBA{{0x1f, 0x77, 0xb4, 255}, {0xff, 0x7f, 0x0e, 255}},
	}, {
		name: "stacked",
		xml: chartXML("Stacked", `<c:barChart><c:barDir val="bar"/><c:grouping val="stacked"/>`+
			ser(0, "A", "", quarters, "1", "2", "3")+
			ser(1, "B", "", quarters, "2", "2", "2")+
			ser(2, "C", "", quarters, "3", "1", "1")+
			`<c:overlap val="100"/>`+axisIDs+`</c:barChart>`,
			strings.ReplaceAll(strings.ReplaceAll(categoryAxes, `"b"`, `"x"`), `"l"`, `"b"`), "r"),
		colors: []color.NRGBA{red, green, blue},
	}, {
		name: "line",
		xml: chartXML("Trend", `<c:lineChart><c:grouping val="standard"/>`+
			ser(0, "Visits", "", strCache("Sheet1!$A$2:$A$6", "Mon", "Tue", "Wed", "Thu", "Fri"), "5", "7", "", "6", "9")+
			ser(1, "Sales", "", strCache("Sheet1!$A$2:$A$6", "Mon", "Tue", "Wed", "Thu", "Fri"), "1", "3", "4", "2", "5")+
			`<c:marker val="1"/>`+axisIDs+`</c:lineChart>`, categoryAxes, "t"),
		colors: []color.NRGBA{red, green},
	}, {
		name: "pie",
		xml: chartXML("Share", `<c:pieChart><c:varyColors val="1"/>`+
			ser(0, "Share", "", strCache("Sheet1!$A$2:$A$6", "a", "b", "c", "d", "e"), "10", "20", "30", "40", "50")+
			`<c:dLbls><c:showPercent val="1"/></c:dLbls><c:firstSliceAng val="0"/></c:pieChart>`, "", "r"),
		colors: []color.NRGBA{red, green, blue, orange, purple},
	}, {
		name: "scatter",
		xml: chartXML("", `<c:scatterChart><c:scatterStyle val="lineMarker"/><c:varyColors val="0"/>`+
			`<c:ser><c:idx val="0"/><c:order val="0"/><c:tx>`+strCache("Sheet1!$B$1", "Height")+`</c:tx>`+
			`<c:spPr><a:ln w="19050"><a:noFill/></a:ln></c:spPr>`+
			`<c:xVal>`+numCache("Sheet1!$A$2:$A$6", "General", "1.5", "2", "3.25", "4", "5")+`</c:xVal>`+
			`<c:yVal>`+numCache("Sheet1!$B$2:$B$6", "0.0", "10", "12.5", "9", "15", "20")+`</c:yVal></c:ser>`+
			`<c:axId val="1"/><c:axId val="2"/></c:scatterChart>`,
			`<c:valAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/>`+
				`<c:axPos val="b"/><c:title><c:tx><c:rich><a:p><a:r><a:t>Age</a:t></a:r></a:p></c:rich></c:tx></c:title>`+
				`<c:crossAx val="2"/></c:valAx>`+
				`<c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/><c:max val="25"/><c:min val="0"/></c:scaling>`+
				`<c:delete val="0"/><c:axPos val="l"/><c:majorGridlines/><c:majorUnit val="5"/><c:crossAx val="1"/></c:valAx>`, ""),
		colors: []color.NRGBA{red},
	}}
}

func TestSpec(t *testing.T) {
	theme := testTheme(t)
	for _, test := range testCharts() {
		t.Run(test.name, func(t *testing.T) {
			c, err := Parse(test.xml, Options{Theme: theme})
			if err != nil {
				t.Fatal(err)
			}
			b, err := c.Spec(400, 300, "Carlito")
			if err != nil {
				t.Fatal(err)
			}
			var got bytes.Buffer
			if err := json.Indent(&got, b, "", "  "); err != nil {
				t.Fatal(err)
			}
			got.WriteByte('\n')
			name := filepath.Join("testdata", test.name+".json")
			if *update {
				if err := os.WriteFile(name, got.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			exp, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got.Bytes(), exp) {
				t.Errorf("expected the spec of %s, got:\n%s", name, got.String())
			}
		})
	}
}

// render draws a chart onto a pixmap of w by h, on white.
func render(t *testing.T, c *Chart, w, h uint32) *okage.Pixmap {
	t.Helper()
	pm, ok := okage.NewPixmap(w, h)
	if !ok {
		t.Fatal("expected a pixmap")
	}
	pm.Fill(okage.ColorWhite)
	dev := display.NewOkage(pm, okage.IdentityTransform())
	r, _ := okage.RectFromXYWH(0, 0, float32(w), float32(h))
	if err := c.Draw(context.Background(), dev, r); err != nil {
		t.Fatal(err)
	}
	return pm
}

// at returns the color of pm at (x, y), not premultiplied.
func at(pm *okage.Pixmap, x, y int) color.NRGBA {
	c, _ := color.NRGBAModel.Convert(pm.At(x, y)).(color.NRGBA)
	return c
}

// count counts the pixels of pm in the rectangle r that are near c.
func count(pm *okage.Pixmap, c color.NRGBA, x0, y0, x1, y1 int) int {
	n := 0
	near := func(a, b uint8) bool { return a >= b-min(b, 24) && a <= b+min(255-b, 24) }
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			p := at(pm, x, y)
			if near(p.R, c.R) && near(p.G, c.G) && near(p.B, c.B) && p.A == 255 {
				n++
			}
		}
	}
	return n
}

// ink counts the pixels of pm in a rectangle that are darker than light
// gray.
func ink(pm *okage.Pixmap, x0, y0, x1, y1 int) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if p := at(pm, x, y); int(p.R)+int(p.G)+int(p.B) < 3*160 {
				n++
			}
		}
	}
	return n
}

func TestDraw(t *testing.T) {
	theme := testTheme(t)
	const w, h = 400, 300
	for _, test := range testCharts() {
		t.Run(test.name, func(t *testing.T) {
			c, err := Parse(test.xml, Options{Theme: theme})
			if err != nil {
				t.Fatal(err)
			}
			pm := render(t, c, w, h)
			if dir := os.Getenv("IVY_CHART_PNG"); dir != "" {
				if b, err := pm.EncodePNG(); err == nil {
					_ = os.WriteFile(filepath.Join(dir, test.name+".png"), b, 0o644)
				}
			}
			for _, col := range test.colors {
				if n := count(pm, col, 0, 0, w, h); n < 40 {
					t.Errorf("expected the color %v, got %d pixels", col, n)
				}
			}
			if len(c.title) > 0 {
				if n := ink(pm, w/4, 0, 3*w/4, h/10); n < 30 {
					t.Errorf("expected the ink of the title, got %d pixels", n)
				}
			}
		})
	}
}

func TestPieSlices(t *testing.T) {
	theme := testTheme(t)
	test := testCharts()[3]
	c, err := Parse(test.xml, Options{Theme: theme})
	if err != nil {
		t.Fatal(err)
	}
	const w, h = 400, 300
	pm := render(t, c, w, h)
	// The slices are 10, 20, 30, 40 and 50 of 150, so each color covers more
	// than the one before.
	prev := 0
	for i, col := range test.colors {
		n := count(pm, col, 0, 0, w, h)
		if n <= prev {
			t.Errorf("slice %d: expected more than %d pixels of %v, got %d", i, prev, col, n)
		}
		prev = n
	}
	// The first slice starts at the top and goes clockwise, so the top
	// right of the pie is red, and the top left is the color of the last.
	cx, cy := centerOf(pm, test.colors, w*3/4, h)
	if p := at(pm, cx+8, cy-40); p != test.colors[0] {
		t.Errorf("expected the first slice at the top right, got: %v", p)
	}
	if p := at(pm, cx-8, cy-40); p != test.colors[4] {
		t.Errorf("expected the last slice at the top left, got: %v", p)
	}
}

// centerOf returns the center of the pixels of the colors.
func centerOf(pm *okage.Pixmap, colors []color.NRGBA, w, h int) (int, int) {
	x0, y0, x1, y1 := w, h, 0, 0
	for y := range h {
		for x := range w {
			p := at(pm, x, y)
			for _, c := range colors {
				if p == c {
					x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
				}
			}
		}
	}
	return (x0 + x1) / 2, (y0 + y1) / 2
}

func TestDrawList(t *testing.T) {
	c, err := Parse(testCharts()[0].xml, Options{Width: 360, Height: 216})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := c.Size(); w != 360 || h != 216 {
		t.Errorf("expected the size 360 by 216, got %g by %g", w, h)
	}
	var l display.List
	r, _ := okage.RectFromXYWH(10, 20, 480, 288)
	if err := c.Draw(context.Background(), &l, r); err != nil {
		t.Fatal(err)
	}
	ops := strings.Split(l.String(), "\n")
	if ops[0] != "save" || ops[len(ops)-1] != "restore" {
		t.Errorf("expected the drawing between save and restore, got: %s ... %s", ops[0], ops[len(ops)-1])
	}
	// The layout of 360 by 216 points scales by 4/3 into the rectangle.
	if !strings.HasPrefix(ops[1], "concat") || !strings.Contains(ops[1], "1.3333") {
		t.Errorf("expected a scale of 4/3, got: %s", ops[1])
	}
	fills := 0
	for _, op := range ops {
		if strings.HasPrefix(op, "fill") {
			fills++
		}
		if strings.HasPrefix(op, "image") || strings.HasPrefix(op, "mask") {
			t.Errorf("expected vectors only, got: %s", op)
		}
	}
	if fills < 10 {
		t.Errorf("expected the fills of the bars and the text, got %d", fills)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []string{
		``,
		`<x/>`,
		`<c:chartSpace ` + chartNS + `/>`,
	}
	for _, test := range tests {
		if _, err := Parse([]byte(test), Options{}); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%q: expected ivy.ErrFormat, got: %v", test, err)
		}
	}
}

func TestLimits(t *testing.T) {
	vals := make([]string, 0, maxPoints+10)
	for i := range maxPoints + 10 {
		vals = append(vals, fmt.Sprint(i%7))
	}
	xml := chartXML("", `<c:lineChart><c:grouping val="standard"/>`+ser(0, "Big", "", "", vals...)+axisIDs+
		`</c:lineChart>`, categoryAxes, "")
	c, err := Parse(xml, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !c.truncated {
		t.Fatal("expected a truncated chart")
	}
	var l display.List
	r, _ := okage.RectFromXYWH(0, 0, 200, 100)
	err = c.Draw(context.Background(), &l, r)
	if pe, ok := errors.AsType[*ivy.PartialError](err); !ok || !errors.Is(pe, ivy.ErrLimit) {
		t.Errorf("expected an *ivy.PartialError of ivy.ErrLimit, got: %v", err)
	}
}

func TestUnsupported(t *testing.T) {
	xml := chartXML("Surface", `<c:surfaceChart><c:wireframe val="0"/>`+
		ser(0, "S", "", strCache("A", "x", "y"), "1", "2")+axisIDs+`</c:surfaceChart>`, categoryAxes, "")
	c, err := Parse(xml, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if c.unsupported != "surface chart" {
		t.Errorf("expected a surface chart that draws as a frame, got: %q", c.unsupported)
	}
	pm := render(t, c, 300, 200)
	if n := ink(pm, 0, 0, 300, 200); n < 30 {
		t.Errorf("expected the ink of the title and the type, got %d pixels", n)
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		v    float64
		code string
		exp  string
	}{
		{1234.5, "General", "1234.5"},
		{1234.5, "#,##0.00", "1,234.50"},
		{0.256, "0%", "26%"},
		{0.256, "0.0%", "25.6%"},
		{-5, "0;(0)", "(5)"},
		{3, `"$"#,##0`, "$3"},
		{1234567, `[$€-407]#,##0`, "€1,234,567"},
		{45000, "m/d/yyyy", "3/15/2023"},
		{45000, "mmm yy", "Mar 23"},
		{45000.75, "h:mm AM/PM", "6:00 PM"},
		{12, "0.0E+00", "1.2E+01"},
		{7, "@", "7"},
	}
	for _, test := range tests {
		if got := formatValue(test.v, test.code); got != test.exp {
			t.Errorf("formatValue(%g, %q): expected %q, got %q", test.v, test.code, test.exp, got)
		}
	}
}

func TestD3Format(t *testing.T) {
	tests := map[string]string{
		"General": "", "0": ".0f", "#,##0.00": ",.2f", "0%": ".0%", `"$"#,##0`: "$,.0f", "m/d/yyyy": "",
	}
	for code, exp := range tests {
		if got := d3Format(code); got != exp {
			t.Errorf("d3Format(%q): expected %q, got %q", code, exp, got)
		}
	}
}
