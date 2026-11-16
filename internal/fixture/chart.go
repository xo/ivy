package fixture

import "strings"

// Chart returns a chart part of DrawingML, c:chartSpace: a column chart
// without a title or a legend, of one series of three columns in the color
// rgb, such as "FF0000".
func Chart(rgb string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><c:chart><c:autoTitleDeleted val="1"/><c:plotArea><c:layout/>
<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/><c:ser><c:idx val="0"/><c:order val="0"/><c:spPr><a:solidFill><a:srgbClr val="` + rgb + `"/></a:solidFill></c:spPr>
<c:cat><c:strRef><c:f>A</c:f><c:strCache><c:ptCount val="3"/><c:pt idx="0"><c:v>a</c:v></c:pt><c:pt idx="1"><c:v>b</c:v></c:pt><c:pt idx="2"><c:v>c</c:v></c:pt></c:strCache></c:strRef></c:cat>
<c:val><c:numRef><c:f>B</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="3"/><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt><c:pt idx="2"><c:v>3</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser>
<c:axId val="1"/><c:axId val="2"/></c:barChart><c:catAx><c:axId val="1"/><c:crossAx val="2"/></c:catAx><c:valAx><c:axId val="2"/><c:crossAx val="1"/></c:valAx></c:plotArea><c:plotVisOnly val="1"/></c:chart></c:chartSpace>`)
}

// ODFChart returns the content.xml of an embedded chart of OpenDocument: a
// bar chart without a title or a legend, of one series of three bars in the
// color rgb, such as "ff0000".
func ODFChart(rgb string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
 xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"
 xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"
 xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0"
 xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0"
 xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0"
 xmlns:chart="urn:oasis:names:tc:opendocument:xmlns:chart:1.0" office:version="1.3">
<office:automatic-styles>
<style:style style:name="s1" style:family="chart"><style:graphic-properties draw:fill-color="#` + strings.ToLower(rgb) + `"/></style:style>
</office:automatic-styles>
<office:body><office:chart>
<chart:chart svg:width="8cm" svg:height="6cm" chart:class="chart:bar">
<chart:plot-area>
<chart:axis chart:dimension="x" chart:name="primary-x"><chart:categories table:cell-range-address="local-table.A2:.A4"/></chart:axis>
<chart:axis chart:dimension="y" chart:name="primary-y"/>
<chart:series chart:style-name="s1" chart:values-cell-range-address="local-table.B2:.B4" chart:label-cell-address="local-table.B1"/>
</chart:plot-area>
<table:table table:name="local-table">
<table:table-header-columns><table:table-column/></table:table-header-columns>
<table:table-columns><table:table-column/></table:table-columns>
<table:table-header-rows><table:table-row>
<table:table-cell><text:p/></table:table-cell>
<table:table-cell office:value-type="string"><text:p>Values</text:p></table:table-cell>
</table:table-row></table:table-header-rows>
<table:table-rows>
<table:table-row><table:table-cell office:value-type="string"><text:p>a</text:p></table:table-cell><table:table-cell office:value-type="float" office:value="1"/></table:table-row>
<table:table-row><table:table-cell office:value-type="string"><text:p>b</text:p></table:table-cell><table:table-cell office:value-type="float" office:value="2"/></table:table-row>
<table:table-row><table:table-cell office:value-type="string"><text:p>c</text:p></table:table-cell><table:table-cell office:value-type="float" office:value="3"/></table:table-row>
</table:table-rows>
</table:table>
</chart:chart>
</office:chart></office:body></office:document-content>`)
}
