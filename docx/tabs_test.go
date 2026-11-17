package docx

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// tabsStyles is a styles.xml with a style of a table of contents: a right
// tab stop with dot leaders at 6 inches, and a left stop at 1 inch.
var tabsStyles = fixture.WordPart{ID: "rIdS", Type: "styles", Name: "word/styles.xml", Data: fixture.WordXML("w:styles", `
<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>
<w:style w:type="paragraph" w:styleId="TOC1"><w:basedOn w:val="Normal"/><w:pPr><w:tabs><w:tab w:val="left" w:pos="1440"/><w:tab w:val="right" w:leader="dot" w:pos="8640"/></w:tabs></w:pPr></w:style>
`)}

// tabsSection is a section of US Letter with margins of 1 inch.
const tabsSection = `<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720"/></w:sectPr>`

func TestTabStops(t *testing.T) {
	for _, tt := range []struct {
		ppr  string
		want string
	}{
		{``, "-ivy-tab-stops: auto"},
		{`<w:pStyle w:val="TOC1"/>`, "-ivy-tab-stops: 72pt left, 432pt right '.'"},
		// A stop of the paragraph clears a stop of the style, and adds
		// another one.
		{`<w:pStyle w:val="TOC1"/><w:tabs><w:tab w:val="clear" w:pos="1440"/><w:tab w:val="center" w:leader="hyphen" w:pos="2880"/></w:tabs>`, "-ivy-tab-stops: 144pt center '-', 432pt right '.'"},
		// The hanging indent is a stop too.
		{`<w:ind w:left="720" w:hanging="360"/><w:tabs><w:tab w:val="decimal" w:pos="4320"/><w:tab w:val="bar" w:pos="5000"/></w:tabs>`, "-ivy-tab-stops: 36pt left, 216pt decimal"},
	} {
		body := `<w:p><w:pPr>` + tt.ppr + `</w:pPr><w:r><w:t>A</w:t></w:r><w:r><w:tab/><w:t>1</w:t></w:r></w:p>` + tabsSection
		html, _ := convertHTML(t, fixture.DOCX(body, tabsStyles))
		if !strings.Contains(html, tt.want) {
			t.Errorf("%s: expected %q in the HTML, got: %s", tt.ppr, tt.want, html)
		}
	}
}

func TestTabStopsRender(t *testing.T) {
	// The page number of an entry of the table of contents ends at the
	// right tab stop, 6 inches from the left margin, and dots lead to it. The
	// text goes past the left stop at 1 inch.
	body := `<w:p><w:pPr><w:pStyle w:val="TOC1"/></w:pPr><w:r><w:t>The first chapter of the book</w:t></w:r><w:r><w:tab/></w:r><w:r><w:t>12</w:t></w:r></w:p>` + tabsSection
	d, err := Open(t.Context(), bytes.NewReader(fixture.DOCX(body, tabsStyles)))
	if err != nil {
		t.Fatal(err)
	}
	img := render(t, d, 0)
	line := inkBox(img, image.Rect(0, 72, 612, 100))
	if line.Empty() || line.Min.X < 71 || line.Min.X > 74 || line.Max.X < 500 || line.Max.X > 505 {
		t.Errorf("expected the line from 72 to 504 points, got: %v", line)
	}
	// The dots fill the space between the text and the number.
	if dots := inkBox(img, image.Rect(250, 72, 350, 100)); dots.Empty() {
		t.Error("expected the dots of the leader")
	}
}
