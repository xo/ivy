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
