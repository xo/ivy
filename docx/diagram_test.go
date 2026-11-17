package docx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// diagramParts are the data part and the drawing part of a diagram of
// SmartArt: the drawing holds a red rectangle of 100 by 50 points, 50
// points from the left of the diagram.
var diagramParts = []fixture.WordPart{
	{ID: "rIdD", Type: "diagramData", Name: "word/diagrams/data1.xml", Data: []byte(`<?xml version="1.0"?>
<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
<dgm:ptLst/><dgm:extLst><a:ext uri="http://schemas.microsoft.com/office/drawing/2008/diagram">
<dsp:dataModelExt xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" relId="rIdW" minVer="http://schemas.openxmlformats.org/drawingml/2006/diagram"/></a:ext></dgm:extLst></dgm:dataModel>`)},
	{ID: "rIdW", Type: "diagramDrawing", Name: "word/diagrams/drawing1.xml", Data: []byte(`<?xml version="1.0"?>
<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
<dsp:spTree><dsp:nvGrpSpPr><dsp:cNvPr id="0" name=""/><dsp:cNvGrpSpPr/></dsp:nvGrpSpPr><dsp:grpSpPr/>
<dsp:sp modelId="{1}"><dsp:nvSpPr><dsp:cNvPr id="0" name=""/><dsp:cNvSpPr/></dsp:nvSpPr>
<dsp:spPr><a:xfrm><a:off x="635000" y="0"/><a:ext cx="1270000" cy="635000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></dsp:spPr>
</dsp:sp></dsp:spTree></dsp:drawing>`)},
}

// diagramRun is a run with an inline diagram of 200 by 100 points.
const diagramRun = `<w:r><w:drawing><wp:inline><wp:extent cx="2540000" cy="1270000"/><wp:docPr id="1" name="Diagram 1"/>
<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:relIds xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" r:dm="rIdD" r:lo="rIdL" r:qs="rIdQ" r:cs="rIdC"/></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`

func TestDiagram(t *testing.T) {
	buf := fixture.DOCX(`<w:p>`+diagramRun+`</w:p>`+tabsSection, diagramParts...)
	html, _ := convertHTML(t, buf)
	if !strings.Contains(html, `src="ivy-object:diagram1"`) {
		t.Fatalf("expected the object of the diagram:\n%s", html)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	// The diagram starts at the left margin of 72 points, on the first
	// line, and the rectangle is 50 points into it.
	box := redBox(render(t, d, 0))
	if box.Min.X < 120 || box.Min.X > 124 || box.Dx() < 98 || box.Dx() > 102 || box.Dy() < 48 || box.Dy() > 52 {
		t.Errorf("expected a red rectangle of 100 by 50 points at x 122, got %v", box)
	}
}

func TestCanvas(t *testing.T) {
	// A drawing canvas holds a shape with a red fill at 20 points from its
	// corner.
	canvas := `<w:r><w:drawing><wp:inline><wp:extent cx="2540000" cy="1270000"/><wp:docPr id="1" name="Canvas 1"/>
<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas"><wpc:wpc xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas"><wpc:bg/><wpc:whole/>
<wps:wsp><wps:cNvPr id="2" name="Rectangle"/><wps:spPr><a:xfrm><a:off x="254000" y="254000"/><a:ext cx="635000" cy="635000"/></a:xfrm><a:prstGeom prst="rect"/><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></wps:spPr><wps:bodyPr/></wps:wsp>
</wpc:wpc></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(`<w:p>`+canvas+`</w:p>`+tabsSection)))
	if err != nil {
		t.Fatal(err)
	}
	box := redBox(render(t, d, 0))
	if box.Min.X < 90 || box.Min.X > 94 || box.Dx() < 48 || box.Dx() > 52 {
		t.Errorf("expected a red square of 50 points at x 92, got %v", box)
	}
}
