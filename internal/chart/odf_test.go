package chart

import (
	"image/color"
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
