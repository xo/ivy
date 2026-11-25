package docx

import (
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
		{"top and bottom", anchor(`<wp:wrapTopAndBottom/>`), ""},
		{"none", anchor(`<wp:wrapNone/>`), ""},
		{"vml", vml(`<w10:wrap xmlns:w10="urn:schemas-microsoft-com:office:word" type="square" side="largest"/>`), "-ivy-wrap-flow: maximum 9pt"},
		{"vml both", vml(`<w10:wrap xmlns:w10="urn:schemas-microsoft-com:office:word" type="tight"/>`), "-ivy-wrap-flow: both 9pt"},
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
