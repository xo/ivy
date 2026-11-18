package odf

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// calcextODS has a color scale in column A, a data bar in column B and an
// icon set in column C, over the numbers 0 to 4, and a date in D1.
var calcextODS = func() fixture.ODSBook {
	rows := ""
	for i := range 5 {
		v := strconv.Itoa(i)
		cell := `<table:table-cell office:value-type="float" office:value="` + v + `"><text:p>` + v + `</text:p></table:table-cell>`
		rows += `<table:table-row>` + cell + cell + cell
		if i == 0 {
			rows += `<table:table-cell office:value-type="date" office:date-value="2026-11-18"><text:p>2026-11-18</text:p></table:table-cell>`
		}
		rows += `</table:table-row>`
	}
	return fixture.ODSBook{
		Body: `<table:table table:name="Sheet1">` + rows +
			`<calcext:conditional-formats xmlns:calcext="urn:org:documentfoundation:names:experimental:calc:xmlns:calcext:1.0">` +
			`<calcext:conditional-format calcext:target-range-address="Sheet1.A1:Sheet1.A5">` +
			`<calcext:color-scale><calcext:color-scale-entry calcext:value="0" calcext:type="minimum" calcext:color="#ff0000"/>` +
			`<calcext:color-scale-entry calcext:value="0" calcext:type="maximum" calcext:color="#0000ff"/></calcext:color-scale></calcext:conditional-format>` +
			`<calcext:conditional-format calcext:target-range-address="Sheet1.B1:Sheet1.B5">` +
			`<calcext:data-bar calcext:positive-color="#00ff00" calcext:gradient="false" calcext:show-value="false">` +
			`<calcext:formatting-entry calcext:value="0" calcext:type="auto-minimum"/>` +
			`<calcext:formatting-entry calcext:value="8" calcext:type="number"/></calcext:data-bar></calcext:conditional-format>` +
			`<calcext:conditional-format calcext:target-range-address="'Sheet1'.C1:'Sheet1'.C5 Sheet1.C7">` +
			`<calcext:icon-set calcext:icon-set-type="3Arrows">` +
			`<calcext:formatting-entry calcext:value="0" calcext:type="percent"/>` +
			`<calcext:formatting-entry calcext:value="33" calcext:type="percent"/>` +
			`<calcext:formatting-entry calcext:value="67" calcext:type="percent"/></calcext:icon-set></calcext:conditional-format>` +
			`</calcext:conditional-formats></table:table>`,
	}
}()

func TestSheetCalcext(t *testing.T) {
	d := openSheet(t, fixture.ODS(calcextODS), ivy.Config{})
	wb, err := d.odsWorkbook(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	sh := wb.Sheets[0]
	if len(sh.CondFormats) != 3 {
		t.Fatalf("expected 3 conditional formats, got: %d", len(sh.CondFormats))
	}
	scale := sh.CondFormats[0].Rules[0]
	if scale.Kind != "colorScale" || len(scale.Scale) != 2 || scale.Scale[0].Kind != "min" || scale.Scale[1].Color != (color.NRGBA{0, 0, 255, 255}) {
		t.Errorf("color scale: %+v", scale)
	}
	bar := sh.CondFormats[1].Rules[0].Bar
	if bar == nil || bar.Min.Kind != "min" || bar.Max.Kind != "num" || bar.Max.Value != 8 || bar.Gradient || !bar.HideValue {
		t.Errorf("data bar: %+v", bar)
	}
	icons := sh.CondFormats[2]
	if len(icons.Ranges) != 2 || icons.Rules[0].Icons == nil || len(icons.Rules[0].Icons.Thresholds) != 3 {
		t.Errorf("icon set: %+v", icons)
	}
	// the conditional formats compare the value of a number, and not of a
	// date
	for _, r := range sh.Rows {
		for _, c := range r.Cells {
			switch {
			case c.Col == 3 && !math.IsNaN(c.Value):
				t.Errorf("date: value %v", c.Value)
			case c.Col < 3 && c.Value != float64(r.Index):
				t.Errorf("cell %d, %d: value %v", r.Index, c.Col, c.Value)
			}
		}
	}
	// the color scale fills A1 in red, and the bar of B5 is green
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if !hasColor(img, func(c color.NRGBA) bool { return c.R > 240 && c.G < 20 && c.B < 20 }) {
		t.Error("expected the red fill of the color scale")
	}
	if !hasColor(img, func(c color.NRGBA) bool { return c.G > 240 && c.R < 20 && c.B < 20 }) {
		t.Error("expected the green bar of the data bar")
	}
}

// hasColor reports whether a pixel of img matches.
func hasColor(img image.Image, match func(color.NRGBA) bool) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if match(color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)) {
				return true
			}
		}
	}
	return false
}
