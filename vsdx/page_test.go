package vsdx

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// Colors of the tests.
var (
	red    = color.NRGBA{255, 0, 0, 255}
	green  = color.NRGBA{0, 255, 0, 255}
	yellow = color.NRGBA{255, 255, 0, 255}
	white  = color.NRGBA{255, 255, 255, 255}
)

// cells returns XML of Cell elements from pairs of names and values.
var cells = fixture.VisioCells

// box returns the cells of the position and the size of a shape, in
// inches.
func box(x, y, w, h float64) string {
	return cells("PinX", num(x), "PinY", num(y), "Width", num(w), "Height", num(h),
		"LocPinX", num(w/2), "LocPinY", num(h/2))
}

// num formats a number for a cell.
func num(v float64) string {
	return fmt.Sprint(v)
}

// filled returns the cells of a shape filled with the color c, without a
// line.
func filled(c string) string {
	return cells("FillForegnd", c, "FillPattern", "1", "LinePattern", "0")
}

// render opens the drawing, and renders its page n at 72 dots per inch.
func render(t *testing.T, v *fixture.VSDX, n int) image.Image {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(v.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// at returns the color of the pixel of img at the point (x, y) of the
// page, in inches from its bottom left corner. The page is h inches high.
func at(img image.Image, x, y, h float64) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(int(x*72), int((h-y)*72))).(color.NRGBA)
}

// near reports whether the colors a and b differ by at most 40 in each
// channel.
func near(a, b color.NRGBA) bool {
	d := func(x, y uint8) bool { return math.Abs(float64(x)-float64(y)) <= 40 }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B)
}

// onePage returns a drawing of one page of 4 by 4 inches with the shapes.
func onePage(shapes string) *fixture.VSDX {
	return &fixture.VSDX{Pages: []fixture.VSDXPage{{ID: 0, Width: 4, Height: 4, Shapes: shapes}}}
}

// check checks the colors of points of a page of 4 by 4 inches.
func check(t *testing.T, img image.Image, points ...struct {
	x, y float64
	c    color.NRGBA
},
) {
	t.Helper()
	for _, p := range points {
		if got := at(img, p.x, p.y, 4); !near(got, p.c) {
			t.Errorf("at (%g, %g): expected %v, got: %v", p.x, p.y, p.c, got)
		}
	}
}

// point is a point of a page with its color.
type point = struct {
	x, y float64
	c    color.NRGBA
}

func TestRectangle(t *testing.T) {
	img := render(t, onePage(`<Shape ID="1" Type="Shape">`+box(1, 3, 1, 1)+filled("#FF0000")+fixture.VisioRect(0, 1, 1)+`</Shape>`), 0)
	if b := img.Bounds(); b.Dx() != 288 || b.Dy() != 288 {
		t.Fatalf("expected a page of 288 by 288, got: %v", b)
	}
	check(t, img, point{1, 3, red}, point{1.4, 2.6, red}, point{1.7, 3, white}, point{1, 2.3, white})
}

func TestRounding(t *testing.T) {
	img := render(t, onePage(`<Shape ID="1" Type="Shape">`+box(2, 2, 1, 1)+filled("#FF0000")+cells("Rounding", "0.3")+fixture.VisioRect(0, 1, 1)+`</Shape>`), 0)
	check(t, img, point{2, 2, red}, point{1.55, 2, red}, point{1.53, 2.47, white}, point{2.47, 1.53, white})
}

func TestEllipse(t *testing.T) {
	geom := `<Section N="Geometry" IX="0"><Row T="Ellipse" IX="1">` +
		cells("X", "0.5", "Y", "0.5", "A", "1", "B", "0.5", "C", "0.5", "D", "1") + `</Row></Section>`
	img := render(t, onePage(`<Shape ID="1" Type="Shape">`+box(3, 3, 1, 1)+filled("#FF0000")+geom+`</Shape>`), 0)
	check(t, img, point{3, 3, red}, point{3.4, 3, red}, point{2.6, 3, red}, point{2.6, 2.6, white}, point{3.42, 3.42, white})
}

func TestArc(t *testing.T) {
	// a chord from (0, 0.5) to (1, 0.5), with the arc below it
	geom := `<Section N="Geometry" IX="0">` +
		`<Row T="MoveTo" IX="1">` + cells("X", "0", "Y", "0.5") + `</Row>` +
		`<Row T="ArcTo" IX="2">` + cells("X", "1", "Y", "0.5", "A", "0.5") + `</Row>` +
		`<Row T="LineTo" IX="3">` + cells("X", "0", "Y", "0.5") + `</Row></Section>`
	img := render(t, onePage(`<Shape ID="1" Type="Shape">`+box(1, 1, 1, 1)+filled("#FF0000")+geom+`</Shape>`), 0)
	check(t, img, point{1, 0.7, red}, point{1, 0.6, red}, point{1, 1.2, white}, point{0.55, 0.55, white})
}

func TestEllipticalArc(t *testing.T) {
	// half of an ellipse twice as wide as it is high, below its chord
	geom := `<Section N="Geometry" IX="0">` +
		`<Row T="MoveTo" IX="1">` + cells("X", "0", "Y", "1") + `</Row>` +
		`<Row T="EllipticalArcTo" IX="2">` + cells("X", "2", "Y", "1", "A", "1", "B", "0.5", "C", "0", "D", "2") + `</Row>` +
		`<Row T="LineTo" IX="3">` + cells("X", "0", "Y", "1") + `</Row></Section>`
	img := render(t, onePage(`<Shape ID="1" Type="Shape">`+box(2, 1, 2, 1)+filled("#FF0000")+geom+`</Shape>`), 0)
	check(t, img, point{2, 1.25, red}, point{1.3, 1.4, red}, point{2, 0.85, white}, point{1.1, 1.1, white})
}

func TestNURBS(t *testing.T) {
	// a cubic Bézier curve from (0, 0) to (1, 0) through the controls (0, 1)
	// and (1, 1), whose top is at 0.75
	geom := `<Section N="Geometry" IX="0">` +
		`<Row T="MoveTo" IX="1">` + cells("X", "0", "Y", "0") + `</Row>` +
		`<Row T="NURBSTo" IX="2">` + cells("X", "1", "Y", "0", "A", "0", "B", "1", "C", "0", "D", "1") +
		`<Cell N="E" V="0" F="NURBS(1, 3, 1, 1, 0, 1, 0, 1, 1, 1, 0, 1)"/></Row>` +
		`<Row T="LineTo" IX="3">` + cells("X", "0", "Y", "0") + `</Row></Section>`
	img := render(t, onePage(`<Shape ID="1" Type="Shape">`+box(1.5, 1.5, 1, 1)+filled("#FF0000")+geom+`</Shape>`), 0)
	check(t, img, point{1.5, 1.5, red}, point{1.5, 1.7, red}, point{1.5, 1.85, white}, point{1.05, 1.9, white})
}

func TestSpline(t *testing.T) {
	// the same curve as a spline of degree 3
	geom := `<Section N="Geometry" IX="0">` +
		`<Row T="MoveTo" IX="1">` + cells("X", "0", "Y", "0") + `</Row>` +
		`<Row T="SplineStart" IX="2">` + cells("X", "0", "Y", "1", "A", "0", "B", "0", "C", "1", "D", "3") + `</Row>` +
		`<Row T="SplineKnot" IX="3">` + cells("X", "1", "Y", "1", "A", "0") + `</Row>` +
		`<Row T="SplineKnot" IX="4">` + cells("X", "1", "Y", "0", "A", "0") + `</Row>` +
		`<Row T="LineTo" IX="5">` + cells("X", "0", "Y", "0") + `</Row></Section>`
	img := render(t, onePage(`<Shape ID="1" Type="Shape">`+box(1.5, 1.5, 1, 1)+filled("#FF0000")+geom+`</Shape>`), 0)
	check(t, img, point{1.5, 1.5, red}, point{1.5, 1.7, red}, point{1.5, 1.85, white})
}

func TestGroup(t *testing.T) {
	// the child fills the left half of the group, which turns by 90 degrees
	// after a flip: the child then fills the top half
	child := `<Shape ID="2" Type="Shape">` + box(0.25, 0.5, 0.5, 1) + filled("#FF0000") + fixture.VisioRect(0, 0.5, 1) + `</Shape>`
	group := `<Shape ID="1" Type="Group">` + box(2, 2, 1, 1) + cells("Angle", num(math.Pi/2), "FlipX", "1") +
		`<Shapes>` + child + `</Shapes></Shape>`
	img := render(t, onePage(group), 0)
	check(t, img, point{2, 2.25, red}, point{1.7, 2.4, red}, point{2, 1.75, white})
}

func TestMaster(t *testing.T) {
	// the instance takes the size and the geometry of its master, and its
	// own fill
	v := onePage(`<Shape ID="1" Type="Shape" Master="7">` + cells("PinX", "1", "PinY", "1", "FillForegnd", "#00FF00") + `</Shape>` +
		`<Shape ID="2" Type="Shape" Master="7">` + cells("PinX", "3", "PinY", "1") + `</Shape>`)
	v.Masters = []fixture.VSDXMaster{{ID: 7, Shapes: `<Shape ID="5" Type="Shape">` + cells("Width", "1", "Height", "1") +
		`<Cell N="LocPinX" V="0.5" F="Width*0.5"/><Cell N="LocPinY" V="0.5" F="Height*0.5"/>` +
		filled("#FF0000") + fixture.VisioRect(0, 1, 1) + `</Shape>`}}
	img := render(t, v, 0)
	check(t, img, point{1, 1, green}, point{1.4, 0.6, green}, point{3, 1, red}, point{2, 1, white})
}

func TestMasterResized(t *testing.T) {
	// an instance twice as wide as its master computes the trivial formulas
	// of the geometry that it inherits
	geom := `<Section N="Geometry" IX="0">` +
		`<Row T="MoveTo" IX="1"><Cell N="X" V="0" F="Width*0"/><Cell N="Y" V="0" F="Height*0"/></Row>` +
		`<Row T="LineTo" IX="2"><Cell N="X" V="1" F="Width*1"/><Cell N="Y" V="0" F="Height*0"/></Row>` +
		`<Row T="LineTo" IX="3"><Cell N="X" V="1" F="Width*1"/><Cell N="Y" V="1" F="Height*1"/></Row>` +
		`<Row T="LineTo" IX="4"><Cell N="X" V="0" F="Width*0"/><Cell N="Y" V="1" F="Height*1"/></Row>` +
		`<Row T="LineTo" IX="5"><Cell N="X" V="0" F="Geometry1.X1"/><Cell N="Y" V="0" F="Geometry1.Y1"/></Row></Section>`
	v := onePage(`<Shape ID="1" Type="Shape" Master="7">` + cells("PinX", "2", "PinY", "2", "Width", "2") + `</Shape>`)
	v.Masters = []fixture.VSDXMaster{{ID: 7, Shapes: `<Shape ID="5" Type="Shape">` + cells("Width", "1", "Height", "1") +
		`<Cell N="LocPinX" V="0.5" F="Width*0.5"/><Cell N="LocPinY" V="0.5" F="Height*0.5"/>` + filled("#FF0000") + geom + `</Shape>`}}
	img := render(t, v, 0)
	check(t, img, point{1.1, 2, red}, point{2.9, 2, red}, point{2, 2.4, red}, point{0.9, 2, white}, point{3.1, 2, white})
}

func TestMasterGroupFormulas(t *testing.T) {
	// the children of a master group size themselves with formulas of
	// the group (Sheet.5!Width), of a sibling (Sheet.6!Width), of a row of
	// the section User and with IF. The instance is twice the size of the
	// master, and does not list the children.
	geom := `<Section N="Geometry" IX="0">` +
		`<Row T="MoveTo" IX="1"><Cell N="X" V="0" F="Width*0"/><Cell N="Y" V="0" F="Height*0"/></Row>` +
		`<Row T="LineTo" IX="2"><Cell N="X" V="1" F="Width*1"/><Cell N="Y" V="0" F="Height*0"/></Row>` +
		`<Row T="LineTo" IX="3"><Cell N="X" V="1" F="Width*1"/><Cell N="Y" V="1" F="Height*1"/></Row>` +
		`<Row T="LineTo" IX="4"><Cell N="X" V="0" F="Geometry1.X1"/><Cell N="Y" V="1" F="Height*1"/></Row>` +
		`<Row T="LineTo" IX="5"><Cell N="X" V="0" F="Geometry1.X1"/><Cell N="Y" V="0" F="Geometry1.Y1"/></Row></Section>`
	locPin := `<Cell N="LocPinX" V="0" F="Width*0.5"/><Cell N="LocPinY" V="0" F="Height*0.5"/>`
	v := onePage(`<Shape ID="1" Type="Group" Master="7">` + box(2, 2, 2, 2) + `</Shape>`)
	v.Masters = []fixture.VSDXMaster{{ID: 7, Shapes: `<Shape ID="5" Type="Group">` + box(0.5, 0.5, 1, 1) + `<Shapes>` +
		`<Shape ID="6" Type="Shape"><Cell N="PinX" V="0.75" F="Sheet.5!Width*0.75"/><Cell N="PinY" V="0.5" F="Sheet.5!Height*0.5"/>` +
		`<Cell N="Width" V="0.5" F="Sheet.5!Width*0.5"/><Cell N="Height" V="0.2" F="IF(Sheet.5!Width&gt;1,Sheet.5!Height,User.Small)"/>` +
		locPin + `<Section N="User"><Row N="Small"><Cell N="Value" V="0.2"/></Row></Section>` + filled("#FF0000") + geom + `</Shape>` +
		`<Shape ID="8" Type="Shape"><Cell N="PinX" V="0.25" F="Sheet.5!Width*0.25"/><Cell N="PinY" V="0.5" F="Sheet.5!Height*0.5"/>` +
		`<Cell N="Width" V="0.5" F="Sheet.6!Width"/><Cell N="Height" V="0.25" F="Sheet.5!Height*0.25"/>` +
		locPin + filled("#00FF00") + geom + `</Shape>` +
		`</Shapes></Shape>`}}
	img := render(t, v, 0)
	check(t, img, point{2.5, 2.5, red}, point{2.5, 1.2, red}, point{1.5, 2, green}, point{1.5, 2.4, white}, point{0.9, 2, white})
}

func TestCycles(t *testing.T) {
	// two masters and two styles that inherit from each other
	v := onePage(`<Shape ID="1" Type="Group" Master="1" FillStyle="1">` + box(2, 2, 1, 1) + `</Shape>`)
	v.Masters = []fixture.VSDXMaster{
		{ID: 1, Shapes: `<Shape ID="5" Type="Group" Master="2">` + box(0.5, 0.5, 1, 1) + `</Shape>`},
		{ID: 2, Shapes: `<Shape ID="5" Type="Group" Master="1">` + box(0.5, 0.5, 1, 1) + `<Cell N="LocPinX" V="1" F="LocPinX*2"/></Shape>`},
	}
	v.Styles = `<StyleSheet ID="1" FillStyle="2"/><StyleSheet ID="2" FillStyle="1"/>`
	render(t, v, 0)
}

func TestStyles(t *testing.T) {
	// the shape names the style 2, which inherits its fill from the style 1
	v := onePage(`<Shape ID="1" Type="Shape" LineStyle="0" FillStyle="2" TextStyle="0">` + box(2, 2, 1, 1) + fixture.VisioRect(0, 1, 1) + `</Shape>`)
	v.Styles = `<StyleSheet ID="1" LineStyle="0" FillStyle="0" TextStyle="0">` + cells("FillForegnd", "#FFFF00", "LinePattern", "0") + `</StyleSheet>` +
		`<StyleSheet ID="2" LineStyle="1" FillStyle="1" TextStyle="1">` + cells("LineWeight", "0") + `</StyleSheet>`
	img := render(t, v, 0)
	check(t, img, point{2, 2, yellow}, point{2.45, 2.45, yellow})
}

func TestColorTable(t *testing.T) {
	v := onePage(`<Shape ID="1" Type="Shape">` + box(2, 2, 1, 1) + filled("24") + fixture.VisioRect(0, 1, 1) + `</Shape>`)
	v.Colors = `<ColorEntry IX="24" RGB="#00FF00"/>`
	check(t, render(t, v, 0), point{2, 2, green})
}

func TestTheme(t *testing.T) {
	// the fill comes from the first fill style of the theme, with the
	// accent 1 in place of phClr
	v := onePage(`<Shape ID="1" Type="Shape">` + box(2, 2, 1, 1) + cells("FillForegnd", "Themed", "LinePattern", "0",
		"QuickStyleFillColor", "2", "QuickStyleFillMatrix", "1") + fixture.VisioRect(0, 1, 1) + `</Shape>`)
	v.Theme = `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:themeElements>` +
		`<a:clrScheme name="t"><a:dk1><a:srgbClr val="000000"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>` +
		`<a:accent1><a:srgbClr val="00FF00"/></a:accent1></a:clrScheme>` +
		`<a:fmtScheme name="t"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>` +
		`<a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst></a:fmtScheme>` +
		`</a:themeElements></a:theme>`
	check(t, render(t, v, 0), point{2, 2, green})
}

func TestText(t *testing.T) {
	// two runs of text: the first red, the second blue
	text := `<Section N="Character"><Row IX="0">` + cells("Color", "#FF0000", "Size", "0.5") + `</Row>` +
		`<Row IX="1">` + cells("Color", "#0000FF", "Size", "0.5") + `</Row></Section>` +
		`<Text><cp IX="0"/>MMM<cp IX="1"/>WWW</Text>`
	img := render(t, onePage(`<Shape ID="1" Type="Shape">`+box(2, 2, 3.5, 1)+cells("LinePattern", "0", "FillPattern", "0")+text+`</Shape>`), 0)
	var reds, blues int
	left, right := 0, 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			switch {
			case c.R > 200 && c.G < 80 && c.B < 80:
				reds++
				left += x
			case c.B > 200 && c.R < 80 && c.G < 80:
				blues++
				right += x
			}
		}
	}
	if reds < 50 || blues < 50 {
		t.Fatalf("expected red and blue text, got %d red and %d blue pixels", reds, blues)
	}
	if left/reds >= right/blues {
		t.Errorf("expected the red run before the blue run, got the centers %d and %d", left/reds, right/blues)
	}
}

func TestPageNumber(t *testing.T) {
	text := `<Section N="Character"><Row IX="0">` + cells("Color", "#FF0000", "Size", "0.5") + `</Row></Section>` +
		`<Section N="Field"><Row IX="0"><Cell N="Value" V="9" F="PAGENUMBER()"/></Row></Section>` +
		`<Text><cp IX="0"/><fld IX="0">9</fld></Text>`
	shape := `<Shape ID="1" Type="Shape">` + box(2, 2, 2, 1) + cells("LinePattern", "0", "FillPattern", "0") + text + `</Shape>`
	v := onePage("")
	v.Pages = append(v.Pages, fixture.VSDXPage{ID: 1, Width: 4, Height: 4, Shapes: shape})
	d, err := Open(t.Context(), bytes.NewReader(v.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 2 {
		t.Fatalf("expected 2 pages, got: %d, %v", n, err)
	}
	dr, err := d.drawing(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	shapes, err := dr.pageContents(dr.pages[1].part, &counter{max: 1000})
	if err != nil || len(shapes) != 1 {
		t.Fatalf("expected one shape, got: %d, %v", len(shapes), err)
	}
	b := dr.newBuilder(t.Context(), 1)
	tb := b.textBody(shapes[0], b.quick(shapes[0]))
	if tb == nil || len(tb.Paragraphs) != 1 || len(tb.Paragraphs[0].Runs) != 1 || tb.Paragraphs[0].Runs[0].Text != "2" {
		t.Errorf("expected the text 2, got: %+v", tb)
	}
}

func TestInstanceFieldValue(t *testing.T) {
	// the instance gives its field a new value, and inherits the element
	// Text of its master, which holds the value of the master
	v := onePage(`<Shape ID="1" Type="Shape" Master="7">` + box(2, 2, 2, 1) +
		`<Section N="Field"><Row IX="0"><Cell N="Value" V="15 U" U="STR" F="Inh"/></Row></Section></Shape>` +
		`<Shape ID="2" Type="Shape" Master="7">` + box(2, 1, 2, 1) + `</Shape>`)
	v.Masters = []fixture.VSDXMaster{{ID: 7, Shapes: `<Shape ID="5" Type="Shape">` + box(1, 0.5, 2, 1) +
		`<Section N="Field"><Row IX="0"><Cell N="Value" V="42 U" U="STR" F="ROUND(Prop.UCount,0)&amp;&quot; U&quot;"/></Row></Section>` +
		`<Text><fld IX="0">42 U</fld></Text></Shape>`}}
	d, err := Open(t.Context(), bytes.NewReader(v.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	dr, err := d.drawing(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	shapes, err := dr.pageContents(dr.pages[0].part, &counter{max: 1000})
	if err != nil || len(shapes) != 2 {
		t.Fatalf("expected two shapes, got: %d, %v", len(shapes), err)
	}
	b := dr.newBuilder(t.Context(), 0)
	for i, want := range []string{"15 U", "42 U"} {
		tb := b.textBody(shapes[i], b.quick(shapes[i]))
		if tb == nil || len(tb.Paragraphs) != 1 || len(tb.Paragraphs[0].Runs) != 1 || tb.Paragraphs[0].Runs[0].Text != want {
			t.Errorf("shape %d: expected the text %q, got: %+v", i+1, want, tb)
		}
	}
}

func TestArrow(t *testing.T) {
	// a line of 2 inches with an arrow at its end, at its right
	geom := `<Section N="Geometry" IX="0">` +
		`<Row T="MoveTo" IX="1">` + cells("X", "0", "Y", "0") + `</Row>` +
		`<Row T="LineTo" IX="2">` + cells("X", "2", "Y", "0") + `</Row></Section>`
	shape := `<Shape ID="1" Type="Shape">` + box(2, 2, 2, 0) +
		cells("LineColor", "#FF0000", "LineWeight", "0.03", "EndArrow", "4", "EndArrowSize", "4", "FillPattern", "0") + geom + `</Shape>`
	img := render(t, onePage(shape), 0)
	// the arrow is wider than the line near its end, and the line is thin
	// near its start
	check(t, img, point{2.85, 2.03, red}, point{1.15, 2.03, white}, point{2, 2.005, red})
}

func TestBackgroundPage(t *testing.T) {
	v := &fixture.VSDX{Pages: []fixture.VSDXPage{
		{ID: 0, Width: 4, Height: 4, BackPage: 3, Shapes: `<Shape ID="1" Type="Shape">` + box(2, 2, 1, 1) + filled("#FF0000") + fixture.VisioRect(0, 1, 1) + `</Shape>`},
		{ID: 3, Width: 4, Height: 4, Background: true, Shapes: `<Shape ID="1" Type="Shape">` + box(2, 2, 4, 4) + filled("#FFFF00") + fixture.VisioRect(0, 4, 4) + `</Shape>`},
	}}
	d, err := Open(t.Context(), bytes.NewReader(v.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 1 {
		t.Fatalf("expected one foreground page, got: %d, %v", n, err)
	}
	check(t, render(t, v, 0), point{2, 2, red}, point{0.5, 0.5, yellow}, point{3.5, 3.5, yellow})
}

func TestDrawingScale(t *testing.T) {
	// a page of 400 inches of the drawing at 1 to 100 is 4 inches of paper
	v := &fixture.VSDX{Pages: []fixture.VSDXPage{{ID: 0, Width: 400, Height: 400,
		Sheet:  cells("PageScale", "1", "DrawingScale", "100"),
		Shapes: `<Shape ID="1" Type="Shape">` + box(100, 300, 100, 100) + filled("#FF0000") + fixture.VisioRect(0, 100, 100) + `</Shape>`,
	}}}
	img := render(t, v, 0)
	if b := img.Bounds(); b.Dx() != 288 || b.Dy() != 288 {
		t.Fatalf("expected a page of 288 by 288, got: %v", b)
	}
	check(t, img, point{1, 3, red}, point{2, 2, white})
}

func TestPicture(t *testing.T) {
	v := onePage(`<Shape ID="1" Type="Foreign">` + box(2, 2, 1, 1) + cells("LinePattern", "0", "FillPattern", "0") +
		`<ForeignData ForeignType="Bitmap" CompressionType="PNG"><Rel r:id="media:a.png"/></ForeignData></Shape>`)
	v.Media = map[string][]byte{"a.png": fixture.PNG(4, 4, green)}
	check(t, render(t, v, 0), point{2, 2, green}, point{2.4, 2.4, green}, point{2.7, 2, white})
}

func TestLimits(t *testing.T) {
	var shapes string
	for i := range 20 {
		shapes += fmt.Sprintf(`<Shape ID="%d" Type="Shape">`, i+1) + box(2, 2, 1, 1) + filled("#FF0000") + fixture.VisioRect(0, 1, 1) + `</Shape>`
	}
	d, err := Open(t.Context(), bytes.NewReader(onePage(shapes).Bytes()), WithConfig(ivy.Config{Limits: ivy.Limits{MaxObjects: 300}}))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ivy.RenderImage(t.Context(), p, 72, color.White)
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok || !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected a partial error of a limit, got: %v", err)
	}
}

func TestPageErrors(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(onePage("").Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Page(t.Context(), 1); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected %v, got: %v", ivy.ErrPageRange, err)
	}
	doc, err := Format.Open(t.Context(), bytes.NewReader(onePage("").Bytes()), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.(ivy.Pager); !ok {
		t.Error("expected a document with pages")
	}
}

func TestEval(t *testing.T) {
	cellsOf := map[string]float64{"Width": 2, "Height": 3, "User.Offset": 0.5, "Sheet.5!Width": 4}
	lookup := func(name string) (float64, bool) {
		v, ok := cellsOf[name]
		return v, ok
	}
	tests := []struct {
		f    string
		want float64
		ok   bool
	}{
		{"Width*0.5", 1, true},
		{"Height - 1", 2, true},
		{"(Width+Height)/5", 1, true},
		{"-Width*2", -4, true},
		{"1.5e1", 15, true},
		{"Width/0", 0, false},
		{"THEMEVAL()", 0, false},
		{`THEMEVAL("LineWeight",0.75pt)`, 0, false},
		{"Sheet.5!Width", 4, true},
		{"Sheet.6!Width", 0, false},
		{"User.Offset*2", 1, true},
		{"0.25 in", 0.25, true},
		{"2.54CM+72pt", 2, true},
		{"90 deg", math.Pi / 2, true},
		{"50%", 0.5, true},
		{"2 furlongs", 0, false},
		{"Inh", 0, false},
		{"((((((((((((((((((1))))))))))))))))))", 0, false},
		{"IF(Width>1,Height,Width)", 3, true},
		{"IF(Width<=1,Unknown,Width)", 2, true},
		{"IF(AND(Width=2,NOT(FALSE)),1,0)", 1, true},
		{"IF(OR(Width<>2,FALSE),1,0)", 0, true},
		{"MIN(Width,Height,1)+MAX(Width,Height)", 4, true},
		{"GUARD(ROUND(2.345,2))", 2.35, true},
		{"INT(-1.5)+ABS(-2)+SQRT(9)", 3, true},
		{"2^3", 8, true},
		{"IF(Width>1,2,3", 0, false},
		{"UNKNOWN(1)", 0, false},
		{`ROUND(Width,0)&" U"`, 0, false},
	}
	for _, tt := range tests {
		got, ok := eval(tt.f, lookup)
		if ok != tt.ok || ok && math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("eval(%q): expected %g, %v, got: %g, %v", tt.f, tt.want, tt.ok, got, ok)
		}
	}
}
