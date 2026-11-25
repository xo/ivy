package docx

import (
	"image/color"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// TestEmptyParagraphTabs checks that the tab stops of an empty paragraph
// do not count as content, so the paragraph takes the size of its mark,
// as the spacer paragraphs of 4 points of poi/docx/form-footnotes.docx do.
func TestEmptyParagraphTabs(t *testing.T) {
	body := `<w:p><w:pPr><w:tabs><w:tab w:val="clear" w:pos="4320"/></w:tabs><w:rPr><w:sz w:val="8"/></w:rPr></w:pPr></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	if !strings.Contains(html, "font-size: 4pt") {
		t.Errorf("expected the size of the mark, 4pt:\n%s", html)
	}
}

// TestMarkerTabStop checks that the text after a number starts at a tab
// stop before the left indent, as on page 1 of
// poi/docx/form-footnotes.docx.
func TestMarkerTabStop(t *testing.T) {
	numbering := fixture.WordPart{ID: "rIdN", Type: "numbering", Name: "word/numbering.xml", Data: fixture.WordXML("w:numbering",
		`<w:abstractNum w:abstractNumId="5"><w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/><w:lvlText w:val="-"/>`+
			`<w:pPr><w:tabs><w:tab w:val="num" w:pos="360"/></w:tabs><w:ind w:left="360" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>`+
			`<w:num w:numId="9"><w:abstractNumId w:val="5"/></w:num>`)}
	for _, tt := range []struct {
		name, tabs, want string
	}{
		{"indent", ``, "min-width: 18pt"},
		{"tab", `<w:tabs><w:tab w:val="clear" w:pos="360"/><w:tab w:val="num" w:pos="252"/></w:tabs>`, "min-width: 12.6pt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="9"/></w:numPr>` + tt.tabs + `</w:pPr><w:r><w:t>Text</w:t></w:r></w:p>`
			html, _ := convertHTML(t, fixture.DOCX(body, numbering))
			if !strings.Contains(html, tt.want) {
				t.Errorf("expected %q:\n%s", tt.want, html)
			}
		})
	}
}

// TestTablePropertyExceptions checks that the borders of w:tblPrEx replace
// those of the table in their row, as in the rows of
// poi/docx/form-footnotes.docx.
func TestTablePropertyExceptions(t *testing.T) {
	cell := `<w:tc><w:p><w:r><w:t>Cell</w:t></w:r></w:p></w:tc>`
	body := `<w:tbl><w:tblPr><w:tblBorders><w:insideV w:val="single" w:sz="8" w:space="0" w:color="auto"/></w:tblBorders></w:tblPr>` +
		`<w:tblGrid><w:gridCol w:w="2000"/><w:gridCol w:w="2000"/></w:tblGrid>` +
		`<w:tr>` + cell + cell + `</w:tr>` +
		`<w:tr><w:tblPrEx><w:tblBorders><w:insideV w:val="none" w:sz="0" w:space="0" w:color="auto"/></w:tblBorders></w:tblPrEx>` + cell + cell + `</w:tr></w:tbl>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	rows := strings.Split(html, "<tr")
	if len(rows) != 3 {
		t.Fatalf("expected two rows:\n%s", html)
	}
	if !strings.Contains(rows[1], "border-right: 1pt solid") {
		t.Errorf("expected the inside border in the first row:\n%s", rows[1])
	}
	if strings.Contains(rows[2], "border-right") || strings.Contains(rows[2], "border-left") {
		t.Errorf("expected no inside border in the row with the exceptions:\n%s", rows[2])
	}
}

// TestTableIndentCompat checks that Word 2013 and later put the edge of a
// table at its indent, and earlier versions put the text of its first
// cell there (libreoffice/docx/landscape-a4-table.docx).
func TestTableIndentCompat(t *testing.T) {
	body := `<w:tbl><w:tblPr/><w:tblGrid><w:gridCol w:w="2000"/></w:tblGrid><w:tr><w:tc><w:p><w:r><w:t>Cell</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`
	settings := func(mode string) fixture.WordPart {
		return fixture.WordPart{ID: "rIdG", Type: "settings", Name: "word/settings.xml", Data: fixture.WordXML("w:settings",
			`<w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="`+mode+`"/></w:compat>`)}
	}
	html, _ := convertHTML(t, fixture.DOCX(body, settings("14")))
	if !strings.Contains(html, "margin-left: -5.4pt") {
		t.Errorf("expected the table a cell margin to the left in mode 14:\n%s", html)
	}
	html, _ = convertHTML(t, fixture.DOCX(body, settings("15")))
	if strings.Contains(html, "margin-left: -5.4pt") {
		t.Errorf("expected the table at the margin in mode 15:\n%s", html)
	}
}

// TestHorizontalRule checks that a horizontal line of Word draws as a box
// of the width of the column, as on page 1 of poi/docx/drawing.docx.
func TestHorizontalRule(t *testing.T) {
	body := `<w:p><w:r><w:pict><v:rect style="width:0;height:1.5pt" o:hralign="center" o:hrstd="t" o:hr="t" fillcolor="#aca899" stroked="f"/></w:pict></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body))
	if !strings.Contains(html, "width: 100%; height: 1.5pt; background-color: rgba(172,168,153,1.000)") {
		t.Errorf("expected a line of the width of the column:\n%s", html)
	}
}

// TestVMLPicture checks that a VML picture at a position on the page is a
// box at that position, behind the text, as on page 4 of
// aspose/docx/rendering.docx.
func TestVMLPicture(t *testing.T) {
	img := fixture.WordPart{ID: "rIdI", Type: "image", Name: "word/media/a.png", Data: fixture.PNG(4, 4, color.White)}
	body := `<w:p><w:r><w:pict><v:shape style="position:absolute;margin-left:486pt;margin-top:27pt;width:101.25pt;height:101.25pt;z-index:-2;` +
		`mso-position-horizontal-relative:page;mso-position-vertical-relative:page" stroked="t"><v:imagedata r:id="rIdI"/></v:shape></w:pict></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body, img))
	for _, want := range []string{"position: absolute", "z-index: -1", "width: 101.25pt", "border: 0.75pt solid #000000", `<img src="`} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q:\n%s", want, html)
		}
	}
}
