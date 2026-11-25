package chart

import (
	"image/color"
	"strings"
	"testing"
)

// odfChart is the content.xml of an embedded chart of OpenDocument: a bar
// chart of two series, with the colors in the styles of the series.
const odfChart = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
 xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"
 xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"
 xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0"
 xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0"
 xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0"
 xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0"
 xmlns:chart="urn:oasis:names:tc:opendocument:xmlns:chart:1.0" office:version="1.3">
<office:automatic-styles>
<style:style style:name="ch1" style:family="chart"><style:text-properties fo:font-size="10pt"/></style:style>
<style:style style:name="ch2" style:family="chart"><style:chart-properties chart:vertical="false"/></style:style>
<style:style style:name="s1" style:family="chart"><style:graphic-properties draw:fill-color="#ff0000"/></style:style>
<style:style style:name="s2" style:family="chart"><style:chart-properties chart:data-label-number="value"/>
<style:graphic-properties draw:fill-color="#0000ff"/></style:style>
</office:automatic-styles>
<office:body><office:chart>
<chart:chart svg:width="12cm" svg:height="8cm" chart:class="chart:bar" chart:style-name="ch1">
<chart:title><text:p>Fruit</text:p></chart:title>
<chart:legend chart:legend-position="bottom"/>
<chart:plot-area chart:style-name="ch2">
<chart:axis chart:dimension="x" chart:name="primary-x"><chart:categories table:cell-range-address="local-table.A2:.A4"/></chart:axis>
<chart:axis chart:dimension="y" chart:name="primary-y"><chart:grid chart:class="major"/></chart:axis>
<chart:series chart:style-name="s1" chart:values-cell-range-address="local-table.B2:.B4" chart:label-cell-address="local-table.B1"/>
<chart:series chart:style-name="s2" chart:values-cell-range-address="local-table.$C$2:.$C$4" chart:label-cell-address="local-table.$C$1"/>
</chart:plot-area>
<table:table table:name="local-table">
<table:table-header-columns><table:table-column/></table:table-header-columns>
<table:table-columns><table:table-column table:number-columns-repeated="2"/></table:table-columns>
<table:table-header-rows><table:table-row>
<table:table-cell><text:p/></table:table-cell>
<table:table-cell office:value-type="string"><text:p>2024</text:p></table:table-cell>
<table:table-cell office:value-type="string"><text:p>2025</text:p></table:table-cell>
</table:table-row></table:table-header-rows>
<table:table-rows>
<table:table-row><table:table-cell office:value-type="string"><text:p>Apples</text:p></table:table-cell>
<table:table-cell office:value-type="float" office:value="3"/><table:table-cell office:value-type="float" office:value="4"/></table:table-row>
<table:table-row><table:table-cell office:value-type="string"><text:p>Pears</text:p></table:table-cell>
<table:table-cell office:value-type="float" office:value="5"/><table:table-cell office:value-type="float" office:value="2"/></table:table-row>
<table:table-row><table:table-cell office:value-type="string"><text:p>Plums</text:p></table:table-cell>
<table:table-cell office:value-type="float" office:value="1"/><table:table-cell office:value-type="float" office:value="6"/></table:table-row>
</table:table-rows>
</table:table>
</chart:chart>
</office:chart></office:body></office:document-content>`

func TestParseODF(t *testing.T) {
	c, err := ParseODF([]byte(odfChart), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := c.Size(); w < 340 || w > 341 || h < 226 || h > 227 {
		t.Errorf("expected the size 12cm by 8cm in points, got %g by %g", w, h)
	}
	if len(c.title) != 1 || c.title[0] != "Fruit" {
		t.Errorf("expected the title Fruit, got: %q", c.title)
	}
	if len(c.plots) != 1 || c.plots[0].kind != kindBar || len(c.plots[0].series) != 2 {
		t.Fatalf("expected a bar chart of two series, got: %+v", c.plots)
	}
	s := c.plots[0].series[1]
	if s.name != "2025" || len(s.vals) != 3 || s.vals[2] != 6 || s.cats[1] != "Pears" || !s.labels.value {
		t.Errorf("expected the series 2025 with its values, its categories and its labels, got: %+v", s)
	}
	pm := render(t, c, 400, 300)
	for _, col := range []color.NRGBA{{255, 0, 0, 255}, {0, 0, 255, 255}} {
		if n := count(pm, col, 0, 0, 400, 300); n < 100 {
			t.Errorf("expected the color %v, got %d pixels", col, n)
		}
	}
	if n := ink(pm, 100, 0, 300, 30); n < 20 {
		t.Errorf("expected the ink of the title, got %d pixels", n)
	}
}

// odfExtras returns odfChart with a linear regression curve of the first
// series that shows its equation and its R squared, and a data table with
// the keys of the legend, as LibreOffice writes them.
func odfExtras() []byte {
	buf := strings.Replace(odfChart, `</office:automatic-styles>`,
		`<style:style style:name="r1" style:family="chart"><style:chart-properties chart:regression-type="linear" `+
			`loext:regression-max-degree="2" chart:regression-name=""/><style:graphic-properties draw:stroke="dash" `+
			`svg:stroke-color="#00c000" svg:stroke-width="0.05cm"/></style:style>`+
			`<style:style style:name="e1" style:family="chart"><style:text-properties fo:font-size="8pt"/></style:style>`+
			`<style:style style:name="t1" style:family="chart"><style:chart-properties loext:show-horizontal-border="true" `+
			`loext:show-vertical-border="true" loext:show-outline="true" loext:show-keys="true"/>`+
			`<style:graphic-properties svg:stroke-color="#d9d9d9"/></style:style></office:automatic-styles>`, 1)
	buf = strings.Replace(buf, `xmlns:chart=`, `xmlns:loext="urn:org:documentfoundation:names:experimental:office:xmlns:loext:1.0" xmlns:chart=`, 1)
	buf = strings.Replace(buf, `chart:label-cell-address="local-table.B1"/>`,
		`chart:label-cell-address="local-table.B1"><chart:regression-curve chart:style-name="r1">`+
			`<chart:equation chart:display-equation="true" chart:display-r-square="true" chart:style-name="e1"/>`+
			`</chart:regression-curve></chart:series>`, 1)
	buf = strings.Replace(buf, `<chart:plot-area`, `<loext:data-table chart:style-name="t1"/><chart:plot-area`, 1)
	return []byte(buf)
}

// TestODFExtras checks the trendline and the data table of a chart of
// OpenDocument.
func TestODFExtras(t *testing.T) {
	c, err := ParseODF(odfExtras(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := c.plots[0].series[0]
	if len(s.trends) != 1 {
		t.Fatalf("expected one trendline, got %d", len(s.trends))
	}
	tr := s.trends[0]
	green := color.NRGBA{0, 0xc0, 0, 255}
	if tr.kind != "linear" || !tr.eq || !tr.r2 || tr.color != green || tr.dash != "sysDash" ||
		tr.name != "Linear (2024)" || tr.text.size != 8 {
		t.Errorf("expected a green linear trendline with its label, got %+v", tr)
	}
	tb := c.table
	if tb == nil || !tb.horz || !tb.vert || !tb.outline || !tb.keys || tb.line != (color.NRGBA{0xd9, 0xd9, 0xd9, 255}) {
		t.Fatalf("expected a data table with lines and keys, got %+v", tb)
	}
	pm := render(t, c, 400, 300)
	if n := count(pm, green, 0, 0, 400, 300); n < 20 {
		t.Errorf("expected the trendline, got %d green pixels", n)
	}
	// The data table changes the bottom of the plot.
	without, err := ParseODF([]byte(odfChart), Options{})
	if err != nil {
		t.Fatal(err)
	}
	other, diff := render(t, without, 400, 300), 0
	for y := 150; y < 300; y++ {
		for x := range 400 {
			if at(pm, x, y) != at(other, x, y) {
				diff++
			}
		}
	}
	if diff < 500 {
		t.Errorf("expected the data table, got %d pixels of difference", diff)
	}
}
