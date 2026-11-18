package odf

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// The tests of this file come from the faults that the visual review found
// in the files of OpenDocument.

// TestTextHiddenSection checks that a hidden section shows nothing, and
// that the first page takes the master page of the first block that shows,
// even when the master page of the first page has a header.
func TestTextHiddenSection(t *testing.T) {
	body := `<text:section text:name="Help" text:display="none"><text:p text:style-name="Body">Hidden help</text:p></text:section>` +
		`<text:p text:style-name="P2">Shown on a wide page</text:p><text:p text:style-name="Body">More</text:p>`
	html, rules := odtHTML(t, odtFile(body))
	if strings.Contains(html, "Hidden help") {
		t.Error("expected the hidden section to be left out")
	}
	if n := strings.Count(html, `class="section"`); n != 1 {
		t.Errorf("expected one section of pages, got %d:\n%s", n, html)
	}
	// the master page Wide is 6 by 4 inches
	if !strings.Contains(rules, "@page m1 { size: 432pt 288pt") {
		t.Errorf("expected the first page on the master page Wide:\n%s", rules)
	}
}

// TestTextNextMaster checks that the pages after the first page of a master
// page take its next master page.
func TestTextNextMaster(t *testing.T) {
	styles := strings.Replace(odtStylesXML, `<style:master-page style:name="Wide" style:page-layout-name="pm2"/>`,
		`<style:master-page style:name="Wide" style:page-layout-name="pm2" style:next-style-name="Standard"/>`, 1)
	buf := fixture.Rezip(odtFile(`<text:p text:style-name="P2">First</text:p>`),
		fixture.Entry{Name: "styles.xml", Data: []byte(styles)})
	_, rules := odtHTML(t, buf)
	for _, want := range []string{
		// the pages after the first are on Standard, of 5 by 4 inches,
		// with its header
		"@page m1 { size: 360pt 288pt",
		"@top-center { content: element(hf1)",
		// the first page is on Wide, without a header
		"@page m1:nth(1 of m1) { size: 432pt 288pt",
	} {
		if !strings.Contains(rules, want) {
			t.Errorf("expected %q in the rules:\n%s", want, rules)
		}
	}
}

// TestDrawTileSize checks that a bitmap that repeats takes the size of its
// pixels at its resolution, or the size of draw:fill-image-width.
func TestDrawTileSize(t *testing.T) {
	img := fixture.Image(64, 64, drawRed)
	for x := 32; x < 64; x++ {
		for y := range 64 {
			img.Set(x, y, drawBlue)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	// 64 pixels at 144 dots per inch are 32 points
	data := fixture.PNGDPI(buf.Bytes(), 144)
	for _, c := range []struct {
		name, width string
		tile        int
	}{
		{"resolution", `draw:fill-image-width="0cm" draw:fill-image-height="0cm"`, 32},
		{"width", `draw:fill-image-width="16pt" draw:fill-image-height="16pt"`, 16},
	} {
		t.Run(c.name, func(t *testing.T) {
			o := &fixture.ODP{
				Styles: `<draw:fill-image draw:name="T" xlink:href="Pictures/t.png" xlink:type="simple"/>`,
				AutomaticStyles: drawStyles + `<style:style style:name="dpt" style:family="drawing-page"><style:drawing-page-properties draw:fill="bitmap" ` +
					`draw:fill-image-name="T" style:repeat="repeat" draw:fill-image-ref-point="top-left" ` + c.width + `/></style:style>`,
				Pages:   []string{` draw:style-name="dpt">`},
				Entries: []fixture.Entry{{Name: "Pictures/t.png", Data: data}},
			}
			out := drawRender(t, o.Bytes(), 0)
			half := c.tile / 2
			drawExpect(t, out, half/2, 4, drawRed)
			drawExpect(t, out, half+half/2, 4, drawBlue)
			drawExpect(t, out, c.tile+half/2, 4, drawRed)
		})
	}
}

// odtWith returns a text with the body, the automatic styles auto more in
// content.xml, the entries, and styles.xml changed by edit.
func odtWith(body, auto string, edit func(string) string, entries ...fixture.Entry) []byte {
	content := strings.Replace(odtContent(body), "</office:automatic-styles>", auto+"</office:automatic-styles>", 1)
	styles := odtStylesXML
	if edit != nil {
		styles = edit(styles)
	}
	all := make([]fixture.Entry, 0, 3+len(entries))
	all = append(all,
		fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "text")},
		fixture.Entry{Name: "content.xml", Data: []byte(content)},
		fixture.Entry{Name: "styles.xml", Data: []byte(styles)},
	)
	return fixture.Zip(append(all, entries...)...)
}

// odtParagraph returns the start tag and the text of the paragraph of
// html before the text.
func odtParagraph(t *testing.T, html, text string) string {
	t.Helper()
	i := strings.Index(html, text)
	if i < 0 {
		t.Fatalf("expected %q in the html:\n%s", text, html)
	}
	return html[strings.LastIndex(html[:i], "<p "):i]
}

// TestTextStylesPart checks that a header takes the automatic styles of
// styles.xml, apart from the automatic styles of content.xml of the same
// names (libreoffice/odt/ooo96040-2.odt).
func TestTextStylesPart(t *testing.T) {
	edit := func(s string) string {
		return strings.ReplaceAll(s, "MP1", "P1")
	}
	html, _ := odtHTML(t, odtWith(`<text:p text:style-name="P1">Body</text:p>`, "", edit))
	if p := odtParagraph(t, html, "Page "); !strings.Contains(p, "text-align: right") || !strings.Contains(p, "font-size: 10pt") {
		t.Errorf("expected the header with P1 of styles.xml, got: %s", p)
	}
	if p := odtParagraph(t, html, "Body"); !strings.Contains(p, "text-align: center") {
		t.Errorf("expected the text with P1 of content.xml, got: %s", p)
	}
}

// TestTextTightLeading checks that the lines leave out the external
// leading of their fonts when settings.xml sets UnxForceZeroExtLeading
// (libreoffice/odt/ooo96040-2.odt).
func TestTextTightLeading(t *testing.T) {
	settings := `<?xml version="1.0" encoding="UTF-8"?><office:document-settings ` + odtNamespaces +
		` xmlns:config="urn:oasis:names:tc:opendocument:xmlns:config:1.0"><office:settings><config:config-item-set config:name="ooo:configuration-settings">` +
		`<config:config-item config:name="UnxForceZeroExtLeading" config:type="boolean">true</config:config-item>` +
		`</config:config-item-set></office:settings></office:document-settings>`
	body := `<text:h text:style-name="Heading">Heading</text:h>`
	for _, c := range []struct {
		name    string
		entries []fixture.Entry
		want    float64
	}{
		{"default", nil, 1.15 * 1.1499},
		{"tight", []fixture.Entry{{Name: "settings.xml", Data: []byte(settings)}}, 1.15 * 1.1172},
	} {
		t.Run(c.name, func(t *testing.T) {
			html, _ := odtHTML(t, odtWith(body, "", nil, c.entries...))
			if p := odtParagraph(t, html, "Heading"); !strings.Contains(p, "line-height: "+odtNum(c.want)+";") {
				t.Errorf("expected the line height %s, got: %s", odtNum(c.want), p)
			}
		})
	}
}

// TestTextSpacing checks that the spacing above a paragraph adds to the
// spacing below the paragraph before it, and that the layout discards it
// at the top of a page, except after a page break, as LibreOffice does
// (libreoffice/odt/ooo96040-2.odt).
func TestTextSpacing(t *testing.T) {
	const auto = `<style:style style:name="PT" style:family="paragraph" style:parent-style-name="Body"><style:paragraph-properties fo:margin-top="12pt"/></style:style>` +
		`<style:style style:name="PB" style:family="paragraph" style:parent-style-name="PT"><style:paragraph-properties fo:break-before="page"/></style:style>`
	body := `<text:p text:style-name="Body">One</text:p><text:p text:style-name="PT">Two</text:p><text:p text:style-name="PB">Three</text:p>`
	html, _ := odtHTML(t, odtWith(body, auto, nil))
	if p := odtParagraph(t, html, "Two"); !strings.Contains(p, "margin-top: 18pt") || strings.Contains(p, "margin-break") {
		t.Errorf("expected the spacing of both paragraphs above the second, got: %s", p)
	}
	if p := odtParagraph(t, html, "Three"); !strings.Contains(p, "margin-top: 12pt") || !strings.Contains(p, "margin-break: keep") {
		t.Errorf("expected the spacing above a paragraph after a page break kept, got: %s", p)
	}
	if !strings.Contains(odtStyleSheet, "margin-break: discard") {
		t.Error("expected the style sheet to discard the margins at a break")
	}
}

// TestTextBackground checks that the background of the text of a
// paragraph is behind its text only, and that an empty paragraph shows
// none (libreoffice/odt/ooo96040-2.odt).
func TestTextBackground(t *testing.T) {
	const auto = `<style:style style:name="PH" style:family="paragraph" style:parent-style-name="Body"><style:text-properties fo:background-color="#ffff99"/></style:style>` +
		`<style:style style:name="PG" style:family="paragraph" style:parent-style-name="Body"><style:paragraph-properties fo:background-color="#00ff00"/></style:style>`
	body := `<text:p text:style-name="PH"/><text:p text:style-name="PH">Marked</text:p><text:p text:style-name="PG">Green</text:p>`
	html, _ := odtHTML(t, odtWith(body, auto, nil))
	if n := strings.Count(html, "#ffff99"); n != 1 || !strings.Contains(html, `<span style="background-color: #ffff99">Marked</span>`) {
		t.Errorf("expected the background behind the text only, got %d:\n%s", n, html)
	}
	if p := odtParagraph(t, html, "Green"); !strings.Contains(p, "background-color: #00ff00") {
		t.Errorf("expected the background of the paragraph, got: %s", p)
	}
}

// TestTextWindowColor checks that style:use-window-font-color gives the
// automatic color, black, over the color of the parent style
// (libreoffice/odt/ooo96040-2.odt).
func TestTextWindowColor(t *testing.T) {
	const auto = `<style:style style:name="PW" style:family="paragraph" style:parent-style-name="Body"><style:text-properties fo:color="#ffffff"/></style:style>` +
		`<style:style style:name="PA" style:family="paragraph" style:parent-style-name="PW"><style:text-properties style:use-window-font-color="true"/></style:style>`
	html, _ := odtHTML(t, odtWith(`<text:p text:style-name="PW">White</text:p><text:p text:style-name="PA">Auto</text:p>`, auto, nil))
	if p := odtParagraph(t, html, "White"); !strings.Contains(p, "color: #ffffff") {
		t.Errorf("expected the color of the style, got: %s", p)
	}
	if p := odtParagraph(t, html, "Auto"); !strings.Contains(p, "color: #000000") {
		t.Errorf("expected the automatic color, got: %s", p)
	}
}

// TestTextListHeader checks that a list header in the mode
// label-width-and-position starts its first line at the margin of the
// text, as its empty label keeps its width (libreoffice/odt/ooo96040-2.odt).
func TestTextListHeader(t *testing.T) {
	const lists = `<text:list-style style:name="LW"><text:list-level-style-bullet text:level="1" text:bullet-char="•">` +
		`<style:list-level-properties text:space-before="18pt" text:min-label-width="18pt"/></text:list-level-style-bullet></text:list-style>`
	edit := func(s string) string {
		return strings.Replace(s, "</office:styles>", lists+"</office:styles>", 1)
	}
	body := `<text:list text:style-name="LW"><text:list-header><text:p text:style-name="Body">Header</text:p></text:list-header>` +
		`<text:list-item><text:p text:style-name="Body">Item</text:p></text:list-item></text:list>`
	html, _ := odtHTML(t, odtWith(body, "", edit))
	if p := odtParagraph(t, html, "Header"); !strings.Contains(p, "margin-left: 36pt") || strings.Contains(p, "text-indent") {
		t.Errorf("expected the header at the margin of the text, got: %s", p)
	}
	if p := odtParagraph(t, html, "•"); !strings.Contains(p, "text-indent: -18pt") {
		t.Errorf("expected the item with its label before the text, got: %s", p)
	}
}

// TestTextAsianSize checks that the East Asian text takes the size of
// style:font-size-asian, and a percent of the East Asian size of its
// parent.
func TestTextAsianSize(t *testing.T) {
	const auto = `<style:style style:name="PS" style:family="paragraph" style:parent-style-name="Body"><style:text-properties fo:font-size="10pt" style:font-size-asian="14pt"/></style:style>` +
		`<style:style style:name="PP" style:family="paragraph" style:parent-style-name="PS"><style:text-properties style:font-size-asian="50%"/></style:style>`
	body := `<text:p text:style-name="PS">abc 漢字 def</text:p><text:p text:style-name="PP">半分</text:p>`
	html, _ := odtHTML(t, odtWith(body, auto, nil))
	for _, want := range []string{
		`abc <span style="font-size: 14pt">漢字</span> def`,
		`<span style="font-size: 7pt">半分</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the html:\n%s", want, html)
		}
	}
}
