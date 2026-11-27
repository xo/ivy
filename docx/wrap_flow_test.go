package docx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// TestWrapFlow checks that a drawing with square, tight or through wrap
// gives -ivy-wrap-flow from the side of the text of its wrapText, with the
// larger of distL and distR, and that a VML shape gives it from the side
// of its w10:wrap (D63).
func TestWrapFlow(t *testing.T) {
	anchor := func(wrap string) string {
		return `<w:p><w:r><w:drawing><wp:anchor behindDoc="0" distL="114300" distR="228600" distT="0" distB="0">` +
			`<wp:positionH relativeFrom="column"><wp:posOffset>1270000</wp:posOffset></wp:positionH>` +
			`<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>` +
			`<wp:extent cx="635000" cy="635000"/>` + wrap + `<wp:docPr id="1" name="Picture 1"/>` +
			`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"/></a:graphic>` +
			`</wp:anchor></w:drawing></w:r><w:r><w:t>Text</w:t></w:r></w:p>`
	}
	vml := func(wrap string) string {
		return `<w:p><w:r><w:pict><v:rect style="position:absolute;margin-left:100pt;margin-top:0;width:50pt;height:50pt" fillcolor="red">` +
			wrap + `</v:rect></w:pict></w:r><w:r><w:t>Text</w:t></w:r></w:p>`
	}
	for _, tt := range []struct {
		name, body, want string
	}{
		{"both sides", anchor(`<wp:wrapSquare wrapText="bothSides"/>`), "-ivy-wrap-flow: both 18pt"},
		{"left", anchor(`<wp:wrapTight wrapText="left"/>`), "-ivy-wrap-flow: start 18pt"},
		{"right", anchor(`<wp:wrapThrough wrapText="right"/>`), "-ivy-wrap-flow: end 18pt"},
		{"largest", anchor(`<wp:wrapSquare wrapText="largest"/>`), "-ivy-wrap-flow: maximum 18pt"},
		{"no side", anchor(`<wp:wrapSquare/>`), "-ivy-wrap-flow: both 18pt"},
		// The points of the polygon are in a square of 21600 over the
		// extent of 50 points.
		{"polygon", anchor(`<wp:wrapTight wrapText="bothSides"><wp:wrapPolygon edited="1"><wp:start x="10800" y="0"/>` +
			`<wp:lineTo x="21600" y="10800"/><wp:lineTo x="10800" y="21600"/><wp:lineTo x="-432" y="10800"/>` +
			`<wp:lineTo x="10800" y="0"/></wp:wrapPolygon></wp:wrapTight>`),
			"-ivy-wrap-flow: both 18pt; -ivy-wrap-shape: polygon(25pt 0pt, 50pt 25pt, 25pt 50pt, -1pt 25pt, 25pt 0pt)"},
		{"square polygon", anchor(`<wp:wrapSquare><wp:wrapPolygon><wp:start x="0" y="0"/><wp:lineTo x="1" y="1"/>` +
			`<wp:lineTo x="2" y="0"/></wp:wrapPolygon></wp:wrapSquare>`), "-ivy-wrap-flow: both 18pt; width"},
		{"top and bottom", anchor(`<wp:wrapTopAndBottom/>`), ""},
		{"none", anchor(`<wp:wrapNone/>`), ""},
		{"vml", vml(`<w10:wrap xmlns:w10="urn:schemas-microsoft-com:office:word" type="square" side="largest"/>`), "-ivy-wrap-flow: maximum 9pt"},
		{"vml both", vml(`<w10:wrap xmlns:w10="urn:schemas-microsoft-com:office:word" type="tight"/>`), "-ivy-wrap-flow: both 9pt"},
		// the wrapcoords of a VML shape are in a square of 21600 over the
		// shape of 50 points
		{"vml wrapcoords", strings.Replace(vml(`<w10:wrap xmlns:w10="urn:schemas-microsoft-com:office:word" type="tight"/>`),
			`fillcolor`, `wrapcoords="10800 0 21600 21600 0 21600 10800 0" fillcolor`, 1),
			"-ivy-wrap-flow: both 9pt; -ivy-wrap-shape: polygon(25pt 0pt, 50pt 50pt, 0pt 50pt, 25pt 0pt)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html, _ := convertHTML(t, fixture.DOCX(tt.body))
			if tt.want == "" {
				if strings.Contains(html, "-ivy-wrap-flow") {
					t.Errorf("expected no -ivy-wrap-flow:\n%s", html)
				}
				return
			}
			if !strings.Contains(html, tt.want) {
				t.Errorf("expected %q in the xhtml:\n%s", tt.want, html)
			}
		})
	}
}

// TestWrapPolygonFile checks the wrap polygon of the picture of
// float-s-tight.docx of testdata: a diamond in a picture of 3 by 2 inches,
// from x = 180 to x = 396 points on the page (D63). LibreOffice 26.8 wraps
// the text around the diamond, and ivy places each word of the rows next
// to it within 0.2 point of LibreOffice. In the row from y = 141 to y =
// 155, the diamond and its distance of 9 points cut the line from x = 262
// to x = 314.
func TestWrapPolygonFile(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "float-s-tight.docx"))
	if err != nil {
		t.Fatal(err)
	}
	html, _ := convertHTML(t, buf)
	want := "-ivy-wrap-flow: both 9pt; -ivy-wrap-shape: polygon(108pt 0pt, 216pt 72pt, 108pt 144pt, 0pt 72pt, 108pt 0pt)"
	if !strings.Contains(html, want) {
		t.Errorf("expected %q in the xhtml:\n%s", want, html)
	}
}
