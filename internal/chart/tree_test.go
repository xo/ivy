package chart

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
	"testing"
)

// groceries are the rows of the test files chart-treemap.xlsx and
// chart-sunburst.xlsx of xlsx/testdata: the category, the group, the item
// and the sales.
var groceries = [][4]string{
	{"Fruit", "Citrus", "Orange", "120"}, {"Fruit", "Citrus", "Lemon", "45"},
	{"Fruit", "Citrus", "Lime", "30"}, {"Fruit", "Berries", "Strawberry", "80"},
	{"Fruit", "Berries", "Blueberry", "60"}, {"Fruit", "Apple", "", "95"},
	{"Vegetables", "Roots", "Carrot", "70"}, {"Vegetables", "Roots", "Potato", "110"},
	{"Vegetables", "Greens", "Spinach", "40"}, {"Vegetables", "Greens", "Lettuce", "55"},
	{"Vegetables", "Greens", "Kale", "20"}, {"Dairy", "Milk", "", "130"},
	{"Dairy", "Cheese", "Cheddar", "65"}, {"Dairy", "Cheese", "Brie", "25"},
	{"Bakery", "Bread", "", "90"}, {"Bakery", "Cakes", "", "35"},
}

// groceryLevels returns the levels of the categories of groceries, from
// the innermost, and their values.
func groceryLevels() ([][]string, []float64) {
	levels := make([][]string, 3)
	vals := make([]float64, len(groceries))
	for i, row := range groceries {
		for l := range 3 {
			levels[l] = append(levels[l], row[2-l])
		}
		vals[i], _ = strconv.ParseFloat(row[3], 64)
	}
	return levels, vals
}

// treeXML returns a chart of Office 2016 of the layout layout, with the
// three levels of groceries and the parts parts of its series. With cells,
// the data has references to cells and no caches.
func treeXML(layout, parts string, cells bool) []byte {
	var b strings.Builder
	b.WriteString(`<cx:chartSpace xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:cx="http://schemas.microsoft.com/office/drawing/2014/chartex"><cx:chartData><cx:data id="0">`)
	if cells {
		b.WriteString(`<cx:strDim type="cat"><cx:f>_xlchart.v1.0</cx:f></cx:strDim>` +
			`<cx:numDim type="size"><cx:f>_xlchart.v1.2</cx:f></cx:numDim>`)
	} else {
		b.WriteString(`<cx:strDim type="cat"><cx:f>A2:C17</cx:f>`)
		for l := 2; l >= 0; l-- {
			fmt.Fprintf(&b, `<cx:lvl ptCount="%d">`, len(groceries))
			for i, row := range groceries {
				fmt.Fprintf(&b, `<cx:pt idx="%d">%s</cx:pt>`, i, row[l])
			}
			b.WriteString(`</cx:lvl>`)
		}
		fmt.Fprintf(&b, `</cx:strDim><cx:numDim type="size"><cx:f>D2:D17</cx:f><cx:lvl ptCount="%d" formatCode="General">`, len(groceries))
		for i, row := range groceries {
			fmt.Fprintf(&b, `<cx:pt idx="%d">%s</cx:pt>`, i, row[3])
		}
		b.WriteString(`</cx:lvl></cx:numDim>`)
	}
	b.WriteString(`</cx:data></cx:chartData><cx:chart><cx:plotArea><cx:plotAreaRegion>` +
		`<cx:series layoutId="` + layout + `" uniqueId="{1}"><cx:tx><cx:txData><cx:v>Sales</cx:v></cx:txData></cx:tx>` +
		`<cx:dataId val="0"/>` + parts + `</cx:series></cx:plotAreaRegion></cx:plotArea>` +
		`<cx:legend pos="t" align="ctr" overlay="0"/></cx:chart></cx:chartSpace>`)
	return []byte(b.String())
}

func TestBuildTree(t *testing.T) {
	levels, vals := groceryLevels()
	root := buildTree(levels, vals)
	if root.value != 1070 || len(root.children) != 4 {
		t.Fatalf("expected 4 top nodes of 1070, got %d of %g", len(root.children), root.value)
	}
	fruit := root.children[0]
	if fruit.name != "Fruit" || fruit.value != 430 || len(fruit.children) != 3 {
		t.Fatalf("expected Fruit of 430 with 3 children, got %q of %g with %d", fruit.name, fruit.value, len(fruit.children))
	}
	apple := fruit.children[2]
	if apple.name != "Apple" || len(apple.children) != 0 || apple.point != 5 || apple.top != 0 {
		t.Errorf("expected the leaf Apple of the point 5, got %q with %d children, point %d, top %d",
			apple.name, len(apple.children), apple.point, apple.top)
	}
	if got := root.children[3].children[1]; got.name != "Cakes" || got.top != 3 {
		t.Errorf("expected Cakes under the top node 3, got %q under %d", got.name, got.top)
	}
	if d := root.depth(); d != 3 {
		t.Errorf("expected a depth of 3, got %d", d)
	}
	// A point without a positive value is left out.
	root = buildTree([][]string{{"a", "b", "c"}}, []float64{1, 0, math.NaN()})
	if len(root.children) != 1 || root.value != 1 {
		t.Errorf("expected one leaf, got %d of %g", len(root.children), root.value)
	}
}

// TestTreemapExcel lays out the treemap of chart-treemap.xlsx in the plot
// area of Excel, and compares its rectangles with the rectangles of the
// PDF file that Excel 2024 printed, measured in points. Excel draws a white
// line of 1.44 points between the rectangles, which the measures leave out.
func TestTreemapExcel(t *testing.T) {
	levels, vals := groceryLevels()
	root := buildTree(levels, vals)
	tests := []struct {
		name   string
		banner float64
		want   map[string][4]float64
	}{
		{"plain", 0, map[string][4]float64{
			"Orange": {6.48, 56.17, 105.51, 106.95}, "Lemon": {6.48, 164.92, 62.66, 66.26},
			"Apple": {6.48, 232.62, 182.56, 48.61}, "Lime": {70.94, 164.92, 41.05, 66.26},
			"Strawberry": {113.79, 56.17, 75.26, 99.38}, "Blueberry": {113.79, 157.00, 75.26, 74.18},
			"Potato": {190.49, 56.17, 124.59, 82.82}, "Carrot": {190.49, 140.43, 124.59, 52.57},
			"Lettuce": {190.49, 194.45, 58.69, 86.78}, "Spinach": {250.62, 194.45, 64.46, 57.25},
			"Kale": {250.62, 253.14, 64.46, 28.09}, "Bread": {316.52, 200.57, 104.79, 80.66},
			"Milk": {316.88, 56.17, 85.34, 142.95}, "Cheddar": {404.02, 56.17, 58.69, 102.63},
			"Brie": {404.02, 160.60, 58.69, 38.53}, "Cakes": {422.74, 200.57, 39.97, 80.66},
		}},
		{"banner", 18.36, map[string][4]float64{
			"Orange": {6.48, 74.90, 105.51, 97.94}, "Apple": {6.48, 236.58, 182.56, 44.65},
			"Strawberry": {113.79, 74.90, 75.26, 90.74}, "Lettuce": {190.49, 201.65, 58.69, 79.58},
			"Milk": {316.88, 74.90, 85.34, 124.23}, "Bread": {316.52, 219.29, 104.79, 61.94},
			"Cheddar": {404.02, 74.90, 58.69, 89.30}, "Cakes": {422.74, 219.29, 39.97, 61.94},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tiles := layoutTreemap(root, box{5.76, 55.45, 463.43, 281.95}, test.banner)
			got := map[string]box{}
			for _, tl := range tiles {
				if !tl.banner {
					got[tl.n.name] = tl.b
				}
			}
			for name, w := range test.want {
				b, ok := got[name]
				want := box{w[0] - 0.72, w[1] - 0.72, w[0] + w[2] + 0.72, w[1] + w[3] + 0.72}
				if !ok || math.Abs(b.x0-want.x0) > 2 || math.Abs(b.y0-want.y0) > 2 ||
					math.Abs(b.x1-want.x1) > 2 || math.Abs(b.y1-want.y1) > 2 {
					t.Errorf("%s: expected %.1f, got %.1f", name, want, b)
				}
			}
		})
	}
}

func TestTreemapSunburst(t *testing.T) {
	theme := testTheme(t)
	labels := `<cx:dataLabels pos="inEnd"><cx:visibility seriesName="0" categoryName="1" value="0"/></cx:dataLabels>`
	tests := []struct {
		name, layout, parts string
		special, parents    string
	}{
		{"treemap", "treemap", labels, "treemap", "overlapping"},
		{"banner", "treemap", labels + `<cx:layoutPr><cx:parentLabelLayout val="banner"/></cx:layoutPr>`, "treemap", "banner"},
		{"sunburst", "sunburst", labels, "sunburst", "overlapping"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := Parse(treeXML(test.layout, test.parts, false), Options{Theme: theme})
			if err != nil {
				t.Fatal(err)
			}
			if len(c.plots) != 1 || c.plots[0].special != test.special || c.plots[0].parents != test.parents {
				t.Fatalf("expected one %s with %s parents, got %d plots", test.special, test.parents, len(c.plots))
			}
			pm := render(t, c, 480, 300)
			// Each top level has a color of its own.
			for i := range 4 {
				if n := count(pm, c.palette(i), 0, 30, 480, 300); n < 300 {
					t.Errorf("expected the color %d, got %d pixels", i, n)
				}
			}
			if test.parents == "banner" {
				if n := count(pm, bannerGray, 0, 30, 480, 300); n < 200 {
					t.Errorf("expected the gray of the banners, got %d pixels", n)
				}
			}
			// The legend names the top level.
			es := (&canvas{c: c}).entries()
			if len(es) != 4 || es[0].text != "Fruit" || es[3].text != "Bakery" {
				t.Errorf("expected the legend of the top level, got %d entries", len(es))
			}
		})
	}
}

// TestTreeCells reads the data of a treemap from the cells of its
// references, as Excel writes a chart of a workbook.
func TestTreeCells(t *testing.T) {
	var refs []string
	opts := Options{SourceCells: func(ref string) [][]Cell {
		refs = append(refs, ref)
		var out [][]Cell
		for _, row := range groceries {
			switch ref {
			case "_xlchart.v1.0":
				out = append(out, []Cell{{Text: row[0]}, {Text: row[1]}, {Text: row[2]}})
			case "_xlchart.v1.2":
				v, _ := strconv.ParseFloat(row[3], 64)
				out = append(out, []Cell{{Text: row[3], Value: v, Number: true}})
			}
		}
		return out
	}}
	c, err := Parse(treeXML("treemap", "", true), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Errorf("expected two references, got %q", refs)
	}
	if len(c.plots) != 1 || c.plots[0].tree == nil || c.plots[0].tree.value != 1070 {
		t.Fatalf("expected a tree of 1070, got %d plots", len(c.plots))
	}
	if got := c.plots[0].tree.depth(); got != 3 {
		t.Errorf("expected the depth 3, got %d", got)
	}
	// A row of cells holds a point in each column.
	got := cellLevels([][]Cell{{{Text: "a"}, {Text: "b"}, {Text: "c"}}})
	if len(got) != 1 || strings.Join(got[0], ",") != "a,b,c" {
		t.Errorf("expected one level a,b,c, got %q", got)
	}
}

func TestFitLines(t *testing.T) {
	c, err := Parse(treeXML("sunburst", "", false), Options{})
	if err != nil {
		t.Fatal(err)
	}
	cv := &canvas{c: c, t: newTexter(c.fontDatabase(), c.font)}
	st := textStyle{size: 10, color: color.NRGBA{A: 255}}
	w := cv.t.width("Straw", st)
	lines, ok := cv.fitLines("Strawberry", st, w+0.5, 100)
	if !ok || len(lines) != 2 || lines[0] != "Straw" || lines[1] != "berry" {
		t.Errorf("expected Straw and berry, got %q %v", lines, ok)
	}
	if _, ok := cv.fitLines("Strawberry", st, w+0.5, 15); ok {
		t.Error("expected two lines not to fit 15 points")
	}
}
