package xlsx

import (
	"image/color"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

func TestCondFormats(t *testing.T) {
	styles := `<fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>` +
		`<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>` +
		`<borders count="1"><border/></borders><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellXfs>` +
		`<dxfs count="1"><dxf><font><b/><color rgb="FF9C0006"/></font><fill><patternFill><bgColor rgb="FFFFC7CE"/></patternFill></fill></dxf></dxfs>`
	var rows strings.Builder
	for i := 1; i <= 5; i++ {
		rows.WriteString(`<row r="` + strconv.Itoa(i) + `"><c r="A` + strconv.Itoa(i) + `"><v>` + strconv.Itoa(i) + `</v></c></row>`)
	}
	body := `<sheetData>` + rows.String() + `</sheetData>` +
		`<conditionalFormatting sqref="A1:A5"><cfRule type="cellIs" dxfId="0" priority="2" operator="greaterThanOrEqual"><formula>$A$4</formula></cfRule>` +
		`<cfRule type="expression" dxfId="0" priority="1"><formula>MOD(ROW(),2)=0</formula></cfRule></conditionalFormatting>` +
		`<conditionalFormatting sqref="A1:A5"><cfRule type="colorScale" priority="3"><colorScale><cfvo type="min"/><cfvo type="max"/>` +
		`<color rgb="FFFFFFFF"/><color rgb="FF00FF00"/></colorScale></cfRule></conditionalFormatting>` +
		`<conditionalFormatting sqref="A1:A5"><cfRule type="dataBar" priority="4"><dataBar><cfvo type="min"/><cfvo type="max"/><color rgb="FF638EC6"/></dataBar></cfRule>` +
		`<cfRule type="iconSet" priority="5"><iconSet iconSet="3Arrows" showValue="0"><cfvo type="percent" val="0"/><cfvo type="percent" val="33"/><cfvo type="percent" val="67"/></iconSet></cfRule></conditionalFormatting>` +
		`<pageSetup pageOrder="overThenDown"/>`
	book := fixture.XLSXBook{Styles: styles, Sheets: []fixture.XLSXSheet{{Name: "Cond", Body: body}}}
	d := open(t, fixture.XLSX(book), ivy.Config{})
	sh, err := d.workbook(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	s := sh.Sheets[0]
	if !s.Page.OverThenDown {
		t.Error("expected the order over, then down")
	}
	if len(s.CondFormats) != 3 || len(s.CondFormats[0].Rules) != 1 {
		t.Fatalf("expected 3 formats, the first without its rule of a formula, got: %+v", s.CondFormats)
	}
	r := s.CondFormats[0].Rules[0]
	if r.Operands[0].Number != 4 || r.Diff == nil || r.Diff.Fill != rgb(0xffc7ce) || r.Diff.Color != rgb(0x9c0006) {
		t.Errorf("expected the operand 4 and the fill and the color of the dxf, got: %+v %+v", r.Operands, r.Diff)
	}
	if icons := s.CondFormats[2].Rules[1].Icons; icons == nil || icons.Name != "3Arrows" || !icons.HideValue || len(icons.Thresholds) != 3 {
		t.Errorf("expected the icons of 3Arrows, got: %+v", icons)
	}
	_, docs := sourceOf(t, d)
	for _, want := range []string{"background-color:#ffc7ce", `class="db"`, `src="cf/3arrows-3-2.svg"`} {
		if !strings.Contains(docs[0], want) {
			t.Errorf("expected %q in the sheet:\n%s", want, docs[0])
		}
	}
}

func TestCommentsAtEnd(t *testing.T) {
	comments := `<?xml version="1.0"?><comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<authors><author>Ann</author></authors><commentList>` +
		`<comment ref="B2" authorId="0"><text><r><rPr><b/></rPr><t>Ann:</t></r><r><t xml:space="preserve">` + "\n" + `Check this</t></r></text></comment>` +
		`</commentList></comments>`
	book := fixture.XLSXBook{
		Sheets: []fixture.XLSXSheet{{
			Name: "Notes",
			Body: `<sheetData><row r="2"><c r="B2"><v>1</v></c></row></sheetData><pageSetup cellComments="atEnd"/>`,
			Rels: `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="../comments1.xml"/>`,
		}},
		Extra: []fixture.Entry{{Name: "xl/comments1.xml", Data: []byte(comments)}},
	}
	d := open(t, fixture.XLSX(book), ivy.Config{})
	_, docs := sourceOf(t, d)
	if want := "<p>Cell: B2\nComment: Ann:\nCheck this</p>"; !strings.Contains(docs[0], want) {
		t.Errorf("expected %q in the sheet:\n%s", want, docs[0])
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 2 {
		t.Errorf("expected the comments on a page of their own, got: %d, %v", n, err)
	}
}

func TestShapes(t *testing.T) {
	sp := func(name, client string) string {
		return `<xdr:twoCellAnchor><xdr:from><xdr:col>1</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>` +
			`<xdr:to><xdr:col>3</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>4</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to>` +
			`<xdr:sp><xdr:nvSpPr><xdr:cNvPr id="2" name="` + name + `"/><xdr:cNvSpPr/></xdr:nvSpPr><xdr:spPr><a:xfrm><a:off x="609600" y="190500"/><a:ext cx="1219200" cy="571500"/></a:xfrm>` +
			`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></xdr:spPr></xdr:sp>` +
			`<xdr:clientData` + client + `/></xdr:twoCellAnchor>`
	}
	drawing := `<?xml version="1.0" encoding="UTF-8"?><xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` + sp("Box", "") + sp("Tip", ` fPrintsWithSheet="0"`) + `</xdr:wsDr>`
	book := fixture.XLSXBook{
		Sheets: []fixture.XLSXSheet{{
			Name: "Shapes",
			Body: `<sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>x</t></is></c></row></sheetData><drawing r:id="rId1"/>`,
			Rels: `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>`,
		}},
		Extra: []fixture.Entry{{Name: "xl/drawings/drawing1.xml", Data: []byte(drawing)}},
	}
	d := open(t, fixture.XLSX(book), ivy.Config{})
	wb, err := d.workbook(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	images := wb.Sheets[0].Images
	if len(images) != 1 || images[0].Object == "" || wb.Objects[images[0].Object] == nil {
		t.Fatalf("expected one shape that prints, got: %+v", images)
	}
	page, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), page, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	// the shape covers the columns B and C of the rows 2 to 4
	red := color.NRGBA{255, 0, 0, 255}
	x, y := 50+int(images[0].Width/2)+int(wb.Sheets[0].ColWidth(0)), 54+int(wb.Sheets[0].RowHeight(0))+20
	if got := nrgba(img.At(x, y)); !near(got, red) {
		t.Errorf("expected the red shape at (%d, %d), got: %v", x, y, got)
	}
}
