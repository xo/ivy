package test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/odf"
	"github.com/xo/ivy/pptx"
)

// The bounds of TestSlides, for each page. On 2026-11-17 the mean
// difference of the pixels was at most 4.42, and 2.98 after the blur, on
// the title slide of the PowerPoint file, where LibreOffice draws the title
// in Calibri Light. The other pages were at most 1.9 and 1.3. The
// differences come from the anti-aliasing, the glyphs of the text and the
// bullets. The bounds leave room for a change of LibreOffice. The bar of the
// master covers 4 percent of a page, and a page without it differs by about
// 5 after the blur.
const (
	slidesMeanDiff    = 8
	slidesBlurredDiff = 4
)

// slidesEMU converts points to EMU.
func slidesEMU(pt float64) int {
	return int(pt * 12700)
}

// pptxShape returns p:sp of the box at (x, y) of w by h points.
func pptxShape(id int, x, y, w, h float64, rot int, geom, fill, line, text string) string {
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="s%d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>`+
		`<a:xfrm rot="%d"><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="%s"><a:avLst/></a:prstGeom>%s%s</p:spPr>%s</p:sp>`,
		id, id, rot, slidesEMU(x), slidesEMU(y), slidesEMU(w), slidesEMU(h), geom, fill, line, text)
}

// slidesFill returns a:solidFill of the color.
func slidesFill(rgb string) string {
	return `<a:solidFill><a:srgbClr val="` + rgb + `"/></a:solidFill>`
}

// slidesPPTX returns the PowerPoint file of TestSlides: a title and a body
// that inherit from the master, a picture, preset shapes, a group, a table,
// a hidden slide and a background.
func slidesPPTX() []byte {
	title := `<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Quarterly review</a:t></a:r></a:p></p:txBody></p:sp>`
	body := `<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:cNvSpPr/><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="457200" y="1600200"/><a:ext cx="4572000" cy="4525963"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr><a:normAutofit/></a:bodyPr><a:lstStyle/>` +
		`<a:p><a:r><a:rPr lang="en-US"/><a:t>Sales grew in each region</a:t></a:r></a:p>` +
		`<a:p><a:pPr lvl="1"/><a:r><a:rPr lang="en-US"/><a:t>North and </a:t></a:r><a:r><a:rPr lang="en-US" b="1"><a:solidFill><a:schemeClr val="accent2"/></a:solidFill></a:rPr><a:t>south</a:t></a:r></a:p>` +
		`<a:p><a:r><a:rPr lang="en-US"/><a:t>Costs stayed flat</a:t></a:r></a:p></p:txBody></p:sp>`
	pic := `<p:pic><p:nvPicPr><p:cNvPr id="4" name="pic"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>` +
		`<p:blipFill><a:blip r:embed="rIdM1"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>` +
		`<p:spPr><a:xfrm><a:off x="5486400" y="1828800"/><a:ext cx="2743200" cy="2057400"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`
	shapes := pptxShape(2, 40, 40, 200, 120, 0, "ellipse", slidesFill("C00000"), `<a:ln w="50800">`+slidesFill("1F4E79")+`</a:ln>`,
		`<p:txBody><a:bodyPr anchor="ctr"/><a:lstStyle/><a:p><a:pPr algn="ctr"/><a:r><a:rPr lang="en-US" sz="2000"><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></a:rPr><a:t>Ellipse</a:t></a:r></a:p></p:txBody>`) +
		pptxShape(3, 280, 40, 200, 120, 0, "roundRect",
			`<a:gradFill><a:gsLst><a:gs pos="0"><a:schemeClr val="accent1"><a:lumMod val="50000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="accent1"><a:lumMod val="40000"/><a:lumOff val="60000"/></a:schemeClr></a:gs></a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>`,
			`<a:ln><a:noFill/></a:ln>`, "") +
		pptxShape(4, 520, 40, 160, 120, 0, "star5", slidesFill("FFC000"), `<a:ln w="12700">`+slidesFill("000000")+`</a:ln>`, "") +
		pptxShape(5, 60, 220, 220, 100, 1800000, "rightArrow", `<a:solidFill><a:schemeClr val="accent6"/></a:solidFill>`, `<a:ln><a:noFill/></a:ln>`, "") +
		`<p:grpSp><p:nvGrpSpPr><p:cNvPr id="6" name="g"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr>` +
		fmt.Sprintf(`<a:xfrm rot="2700000"><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/><a:chOff x="0" y="0"/><a:chExt cx="%d" cy="%d"/></a:xfrm>`,
			slidesEMU(340), slidesEMU(220), slidesEMU(120), slidesEMU(120), slidesEMU(120), slidesEMU(120)) + `</p:grpSpPr>` +
		pptxShape(7, 0, 0, 60, 60, 0, "rect", slidesFill("7030A0"), "", "") +
		pptxShape(8, 60, 60, 60, 60, 0, "rect", slidesFill("00B0F0"), "", "") + `</p:grpSp>`
	cell := func(rgb, text string) string {
		return `<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US" sz="1400"/><a:t>` + text + `</a:t></a:r></a:p></a:txBody>` +
			`<a:tcPr><a:lnL w="6350">` + slidesFill("000000") + `</a:lnL><a:lnR w="6350">` + slidesFill("000000") + `</a:lnR><a:lnT w="6350">` + slidesFill("000000") + `</a:lnT><a:lnB w="6350">` + slidesFill("000000") + `</a:lnB>` + slidesFill(rgb) + `</a:tcPr></a:tc>`
	}
	table := `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="9" name="t"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
		fmt.Sprintf(`<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`, slidesEMU(60), slidesEMU(380), slidesEMU(600), slidesEMU(120)) +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr/>` +
		fmt.Sprintf(`<a:tblGrid><a:gridCol w="%d"/><a:gridCol w="%d"/><a:gridCol w="%d"/></a:tblGrid>`, slidesEMU(200), slidesEMU(200), slidesEMU(200)) +
		fmt.Sprintf(`<a:tr h="%d">`, slidesEMU(40)) + cell("4472C4", "Region") + cell("4472C4", "Sales") + cell("4472C4", "Costs") + `</a:tr>` +
		fmt.Sprintf(`<a:tr h="%d">`, slidesEMU(40)) + cell("D9E2F3", "North") + cell("D9E2F3", "120") + cell("D9E2F3", "80") + `</a:tr>` +
		fmt.Sprintf(`<a:tr h="%d">`, slidesEMU(40)) + cell("FFFFFF", "South") + cell("FFFFFF", "95") + cell("FFFFFF", "70") + `</a:tr>` +
		`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`
	numbered := pptxShape(2, 60, 60, 600, 300, 0, "rect", "", "",
		`<p:txBody><a:bodyPr/><a:lstStyle/>`+
			`<a:p><a:pPr marL="457200" indent="-457200"><a:buAutoNum type="arabicPeriod"/></a:pPr><a:r><a:rPr lang="en-US" sz="2800"/><a:t>Plan the next quarter</a:t></a:r></a:p>`+
			`<a:p><a:pPr marL="457200" indent="-457200"><a:buAutoNum type="arabicPeriod"/></a:pPr><a:r><a:rPr lang="en-US" sz="2800" i="1"/><a:t>Hire two people</a:t></a:r></a:p>`+
			`<a:p><a:pPr marL="457200" indent="-457200"><a:buAutoNum type="arabicPeriod"/></a:pPr><a:r><a:rPr lang="en-US" sz="2800" u="sng"/><a:t>Review the costs</a:t></a:r></a:p></p:txBody>`)
	master := pptxShape(20, 0, 520, 720, 20, 0, "rect", `<a:solidFill><a:schemeClr val="accent1"/></a:solidFill>`, `<a:ln><a:noFill/></a:ln>`, "")
	img := image.NewNRGBA(image.Rect(0, 0, 64, 48))
	for y := range 48 {
		for x := range 64 {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 4), uint8(y * 5), 160, 255})
		}
	}
	return (&fixture.PPTX{
		MasterShapes: master,
		Slides: []fixture.PPTXSlide{
			{Shapes: title + body + pic},
			{Shapes: shapes + table},
			{Shapes: numbered, Hidden: true},
			{Shapes: numbered, Background: `<p:bg><p:bgPr>` + slidesFill("FFF2CC") + `<a:effectLst/></p:bgPr></p:bg>`},
		},
		Media: []fixture.Entry{{Name: "image1.png", Data: slidesPNG(img)}},
	}).Bytes()
}

// slidesPNG encodes img as PNG.
func slidesPNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// slidesODP returns the OpenDocument presentation of TestSlides: text with
// a list, shapes, a custom shape, a picture, a table, a hidden page and a
// background.
func slidesODP() []byte {
	styles := `<style:style style:name="red" style:family="graphic" style:parent-style-name="standard">
<style:graphic-properties draw:fill="solid" draw:fill-color="#c00000" draw:stroke="solid" svg:stroke-color="#1f4e79" svg:stroke-width="4pt" draw:textarea-vertical-align="middle"/></style:style>
<style:style style:name="grad" style:family="graphic" style:parent-style-name="standard">
<style:graphic-properties draw:fill="gradient" draw:fill-gradient-name="g1" draw:stroke="none"/></style:style>
<style:style style:name="gold" style:family="graphic" style:parent-style-name="standard">
<style:graphic-properties draw:fill="solid" draw:fill-color="#ffc000" draw:stroke="solid" svg:stroke-color="#000000" svg:stroke-width="1pt"/></style:style>
<style:style style:name="none" style:family="graphic" style:parent-style-name="standard">
<style:graphic-properties draw:fill="none" draw:stroke="none"/></style:style>
<style:style style:name="dp2" style:family="drawing-page"><style:drawing-page-properties draw:fill="solid" draw:fill-color="#fff2cc" presentation:background-visible="true"/></style:style>
<style:style style:name="hide" style:family="drawing-page"><style:drawing-page-properties presentation:visibility="hidden"/></style:style>
<style:style style:name="P1" style:family="paragraph"><style:paragraph-properties fo:text-align="center"/><style:text-properties fo:font-size="40pt"/></style:style>
<style:style style:name="T1" style:family="text"><style:text-properties fo:color="#ffffff" fo:font-size="20pt"/></style:style>
<style:style style:name="T2" style:family="text"><style:text-properties fo:color="#ed7d31" fo:font-weight="bold"/></style:style>
<style:style style:name="c1" style:family="table-cell"><style:graphic-properties draw:fill="solid" draw:fill-color="#4472c4"/></style:style>
<style:style style:name="c2" style:family="table-cell"><style:graphic-properties draw:fill="solid" draw:fill-color="#d9e2f3"/></style:style>
<style:style style:name="col" style:family="table-column"><style:table-column-properties style:column-width="200pt"/></style:style>
<style:style style:name="row" style:family="table-row"><style:table-row-properties style:row-height="40pt"/></style:style>
<text:list-style style:name="L1"><text:list-level-style-bullet text:level="1" text:bullet-char="•"><style:list-level-properties text:min-label-width="0.8cm"/></text:list-level-style-bullet>
<text:list-level-style-bullet text:level="2" text:bullet-char="–"><style:list-level-properties text:space-before="0.8cm" text:min-label-width="0.8cm"/></text:list-level-style-bullet></text:list-style>`
	gradient := `<draw:gradient draw:name="g1" draw:style="linear" draw:start-color="#203864" draw:end-color="#b4c7e7" draw:angle="0" draw:border="0%"/>
<style:style style:name="bar" style:family="graphic"><style:graphic-properties draw:fill="solid" draw:fill-color="#4472c4" draw:stroke="none"/></style:style>`
	page1 := `><draw:frame draw:style-name="none" svg:x="36pt" svg:y="21.6pt" svg:width="648pt" svg:height="90pt"><draw:text-box><text:p text:style-name="P1">Quarterly review</text:p></draw:text-box></draw:frame>
<draw:frame draw:style-name="none" svg:x="36pt" svg:y="126pt" svg:width="360pt" svg:height="356pt"><draw:text-box>
<text:list text:style-name="L1"><text:list-item><text:p>Sales grew in each region</text:p>
<text:list><text:list-item><text:p>North and <text:span text:style-name="T2">south</text:span></text:p></text:list-item></text:list></text:list-item>
<text:list-item><text:p>Costs stayed flat</text:p></text:list-item></text:list></draw:text-box></draw:frame>
<draw:frame draw:style-name="none" svg:x="432pt" svg:y="144pt" svg:width="216pt" svg:height="162pt"><draw:image xlink:href="Pictures/image1.png" xlink:type="simple"/></draw:frame>`
	page2 := `><draw:ellipse draw:style-name="red" svg:x="40pt" svg:y="40pt" svg:width="200pt" svg:height="120pt"><text:p><text:span text:style-name="T1">Ellipse</text:span></text:p></draw:ellipse>
<draw:custom-shape draw:style-name="grad" svg:x="280pt" svg:y="40pt" svg:width="200pt" svg:height="120pt"><draw:enhanced-geometry draw:type="round-rectangle"/></draw:custom-shape>
<draw:polygon draw:style-name="gold" svg:x="520pt" svg:y="40pt" svg:width="160pt" svg:height="120pt" svg:viewBox="0 0 160 120" draw:points="80,0 99,45 160,45 111,73 130,120 80,91 30,120 49,73 0,45 61,45"/>
<draw:frame svg:x="60pt" svg:y="380pt" svg:width="600pt" svg:height="120pt"><table:table>
<table:table-column table:style-name="col" table:number-columns-repeated="3"/>
<table:table-row table:style-name="row"><table:table-cell table:style-name="c1"><text:p>Region</text:p></table:table-cell><table:table-cell table:style-name="c1"><text:p>Sales</text:p></table:table-cell><table:table-cell table:style-name="c1"><text:p>Costs</text:p></table:table-cell></table:table-row>
<table:table-row table:style-name="row"><table:table-cell table:style-name="c2"><text:p>North</text:p></table:table-cell><table:table-cell table:style-name="c2"><text:p>120</text:p></table:table-cell><table:table-cell table:style-name="c2"><text:p>80</text:p></table:table-cell></table:table-row>
</table:table></draw:frame>`
	img := image.NewNRGBA(image.Rect(0, 0, 64, 48))
	for y := range 48 {
		for x := range 64 {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 4), uint8(y * 5), 160, 255})
		}
	}
	return (&fixture.ODP{
		Width:           "720pt",
		Height:          "540pt",
		Styles:          gradient,
		MasterShapes:    `<draw:rect draw:style-name="bar" svg:x="0pt" svg:y="520pt" svg:width="720pt" svg:height="20pt"/>`,
		AutomaticStyles: styles,
		Pages:           []string{page1, page2, `draw:style-name="hide"` + page2, `draw:style-name="dp2"` + page1},
		Entries:         []fixture.Entry{{Name: "Pictures/image1.png", Data: slidesPNG(img)}},
	}).Bytes()
}

// slidesPDF converts the file at path to PDF with LibreOffice, in dir, and
// returns the path of the PDF file. LibreOffice keeps its profile in dir.
func slidesPDF(ctx context.Context, dir, path string) (string, error) {
	bin, err := exec.LookPath("soffice")
	if err != nil {
		return "", fmt.Errorf("oracle soffice is not installed: %w", err)
	}
	cmd := exec.CommandContext(ctx, bin, "-env:UserInstallation=file://"+filepath.Join(dir, "profile"),
		"--headless", "--convert-to", "pdf", "--outdir", dir, path)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running soffice: %w: %s", err, out.String())
	}
	pdf := filepath.Join(dir, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))+".pdf")
	if _, err := os.Stat(pdf); err != nil {
		return "", fmt.Errorf("soffice wrote no pdf: %w: %s", err, out.String())
	}
	return pdf, nil
}

// slidesFonts returns the fonts of the module github.com/xo/ivy/fonts, which
// LibreOffice also uses for the fonts of Office.
func slidesFonts(t *testing.T) fs.FS {
	t.Helper()
	dir := filepath.Join("..", "fonts")
	if _, err := os.Stat(filepath.Join(dir, "carlito")); err != nil {
		t.Fatal("the fonts of github.com/xo/ivy/fonts are missing:", err)
	}
	return os.DirFS(dir)
}

// TestSlides compares the slides of a PowerPoint file and of an OpenDocument
// presentation that the test builds with the pages that LibreOffice exports
// to PDF, which pdftoppm renders. The files hold text, lists, preset and
// custom shapes, gradients, a group, a picture, a table, a hidden slide and
// backgrounds. The test compares the number of pages, which leaves out the
// hidden slide, and the mean difference of each page, before and after a
// blur. LibreOffice cannot make its pipe in a sandbox, so the test runs
// outside of one.
//
// IVY_ORACLE_OUT keeps the pages, for a person who looks at them.
func TestSlides(t *testing.T) {
	cfg := ivy.Config{Fonts: slidesFonts(t)}
	for _, c := range []struct {
		name string
		data []byte
		open func(context.Context, ivy.Source, ivy.Config) (ivy.Document, error)
	}{
		{"slides.pptx", slidesPPTX(), pptx.Format.Open},
		{"slides.odp", slidesODP(), odf.Format.Open},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, c.name)
			if err := os.WriteFile(path, c.data, 0o600); err != nil {
				t.Fatal(err)
			}
			pdf, err := slidesPDF(t.Context(), dir, path)
			if err != nil {
				t.Fatal(err)
			}
			wantPages, err := pdfPageCount(t.Context(), pdf)
			if err != nil {
				t.Fatal(err)
			}
			d, err := c.open(t.Context(), bytes.NewReader(c.data), cfg)
			if err != nil {
				t.Fatal(err)
			}
			pager := d.(ivy.Pager)
			gotPages, err := pager.PageCount(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("pages: ivy %d, LibreOffice %d", gotPages, wantPages)
			if gotPages != wantPages {
				t.Errorf("expected %d pages, as LibreOffice, got: %d", wantPages, gotPages)
			}
			for n := range min(gotPages, wantPages) {
				page, err := pager.Page(t.Context(), n)
				if err != nil {
					t.Fatal(err)
				}
				got, err := ivy.RenderImage(t.Context(), page, 72, color.White)
				if err != nil {
					t.Fatal(err)
				}
				want, err := run(t.Context(), "pdftoppm", "-png", "-r", "72", "-f", fmt.Sprint(n+1), "-l", fmt.Sprint(n+1), "-singlefile", pdf)
				if err != nil {
					t.Fatal(err)
				}
				if out := os.Getenv("IVY_ORACLE_OUT"); out != "" {
					writePNG(t, filepath.Join(out, fmt.Sprintf("%s-%d-ivy.png", c.name, n+1)), got)
					writePNG(t, filepath.Join(out, fmt.Sprintf("%s-%d-libreoffice.png", c.name, n+1)), want)
				}
				if got.Bounds().Size() != want.Bounds().Size() {
					t.Errorf("page %d: expected %v pixels, got: %v", n+1, want.Bounds().Size(), got.Bounds().Size())
					continue
				}
				diff := meanDiff(got, want)
				blurred := meanDiff(blur(got, 2), blur(want, 2))
				t.Logf("page %d: mean difference %.2f, after a blur of 5 by 5 pixels %.2f", n+1, diff, blurred)
				if diff > slidesMeanDiff {
					t.Errorf("page %d: expected a mean difference of at most %v, got: %.2f", n+1, slidesMeanDiff, diff)
				}
				if blurred > slidesBlurredDiff {
					t.Errorf("page %d: expected a mean difference after the blur of at most %v, got: %.2f", n+1, slidesBlurredDiff, blurred)
				}
			}
		})
	}
}
