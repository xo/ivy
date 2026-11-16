package xlsx

import (
	"image/color"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// TestAlternateChart checks that a drawing takes the chart of Office 2016
// of an mc:Choice, and the picture of its mc:Fallback when the chart cannot
// be read.
func TestAlternateChart(t *testing.T) {
	drawing := `<?xml version="1.0" encoding="UTF-8"?><xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006">` +
		`<xdr:oneCellAnchor><xdr:from><xdr:col>1</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>` +
		`<xdr:ext cx="1270000" cy="635000"/><mc:AlternateContent><mc:Choice Requires="cx1"><xdr:graphicFrame><a:graphic>` +
		`<a:graphicData uri="http://schemas.microsoft.com/office/drawing/2014/chartex"><cx:chart xmlns:cx="http://schemas.microsoft.com/office/drawing/2014/chartex" r:id="rId2"/></a:graphicData>` +
		`</a:graphic></xdr:graphicFrame></mc:Choice><mc:Fallback><xdr:pic><xdr:blipFill><a:blip r:embed="rId1"/></xdr:blipFill></xdr:pic></mc:Fallback></mc:AlternateContent>` +
		`<xdr:clientData/></xdr:oneCellAnchor></xdr:wsDr>`
	book := fixture.XLSXBook{
		Sheets: []fixture.XLSXSheet{{
			Name: "Chart",
			Body: `<sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>x</t></is></c></row></sheetData><drawing r:id="rId1"/>`,
			Rels: `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>`,
		}},
		Extra: []fixture.Entry{
			{Name: "xl/drawings/drawing1.xml", Data: []byte(drawing)},
			{Name: "xl/drawings/_rels/drawing1.xml.rels", Data: []byte(`<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
				`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image1.png"/>` +
				`<Relationship Id="rId2" Type="http://schemas.microsoft.com/office/2014/relationships/chartEx" Target="../charts/chartEx1.xml"/></Relationships>`)},
			{Name: "xl/media/image1.png", Data: fixture.PNG(4, 2, color.NRGBA{0, 0, 255, 255})},
			// a part that is not a chart
			{Name: "xl/charts/chartEx1.xml", Data: []byte(`<?xml version="1.0"?><other/>`)},
		},
	}
	d := open(t, fixture.XLSX(book), ivy.Config{})
	_, docs := sourceOf(t, d)
	if !strings.Contains(docs[0], `src="xl/media/image1.png"`) {
		t.Errorf("expected the picture of the fallback:\n%s", docs[0])
	}
}
