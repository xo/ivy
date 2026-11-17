package pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// The tests of this file come from the faults that the visual review found
// in the slides of PowerPoint.

// entry returns the data of the entry name of the zip archive buf.
func entry(t *testing.T, buf []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := zr.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestGroupLineWidth checks that the width of the outline of a shape in a
// group does not scale with the group. The children of the group are in
// a space of 10 by 10 EMU, which the group scales to 100 by 100 points.
func TestGroupLineWidth(t *testing.T) {
	child := `<p:sp><p:nvSpPr><p:cNvPr id="3" name="c"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>` +
		`<a:xfrm><a:off x="0" y="0"/><a:ext cx="10" cy="10"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>` +
		`<a:solidFill><a:srgbClr val="00FF00"/></a:solidFill><a:ln w="12700"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:ln></p:spPr></p:sp>`
	group := `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="2" name="g"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr>` +
		fmt.Sprintf(`<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/><a:chOff x="0" y="0"/><a:chExt cx="10" cy="10"/></a:xfrm>`,
			emu(200), emu(200), emu(100), emu(100)) +
		`</p:grpSpPr>` + child + `</p:grpSp>`
	img := render(t, (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: group}}}).Bytes(), 0)
	expect(t, img, 250, 250, green)
	// a line of 1 point stays at the edge of the box, where a line scaled
	// with the group would cover the slide
	if r, _, _, _ := img.At(200, 250).RGBA(); r>>8 < 100 {
		t.Errorf("expected the red line at (200, 250), got %v", img.At(200, 250))
	}
	expect(t, img, 195, 250, white)
	expect(t, img, 250, 195, white)
}

// TestOwnStyleFill checks that the p:style of a placeholder fills it, over
// the p:spPr of the placeholder of the master that has no fill.
func TestOwnStyleFill(t *testing.T) {
	p := &fixture.PPTX{}
	buf := p.Bytes()
	master := entry(t, buf, "ppt/slideMasters/slideMaster1.xml")
	// the title of the master has no fill
	master = strings.Replace(master, `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>`,
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:ln><a:noFill/></a:ln></p:spPr>`, 1)
	title := `<p:sp><p:nvSpPr><p:cNvPr id="2" name="t"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr>` +
		xfrm(100, 100, 200, 100, "") + `</p:spPr><p:style><a:lnRef idx="0"><a:schemeClr val="accent1"/></a:lnRef>` +
		`<a:fillRef idx="1"><a:schemeClr val="accent2"/></a:fillRef><a:effectRef idx="0"><a:schemeClr val="accent1"/></a:effectRef>` +
		`<a:fontRef idx="minor"><a:schemeClr val="lt1"/></a:fontRef></p:style></p:sp>`
	p.Slides = []fixture.PPTXSlide{{Shapes: title}}
	buf = fixture.Rezip(p.Bytes(), fixture.Entry{Name: "ppt/slideMasters/slideMaster1.xml", Data: []byte(master)})
	img := render(t, buf, 0)
	expect(t, img, 120, 120, color.NRGBA{0xed, 0x7d, 0x31, 255})
}

// TestThemePicture checks that a picture fill of the styles of the theme
// takes its picture from the relationships of the theme part, not from
// those of the slide or of the master.
func TestThemePicture(t *testing.T) {
	p := &fixture.PPTX{
		MasterBackground: `<p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg>`,
		Slides:           []fixture.PPTXSlide{{}},
	}
	buf := p.Bytes()
	theme := entry(t, buf, "ppt/theme/theme1.xml")
	theme = strings.Replace(theme, `<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill>`,
		`<a:bgFillStyleLst><a:blipFill><a:blip r:embed="rId1"/><a:stretch><a:fillRect/></a:stretch></a:blipFill>`, 1)
	buf = fixture.Rezip(buf,
		fixture.Entry{Name: "ppt/theme/theme1.xml", Data: []byte(theme)},
		fixture.Entry{Name: "ppt/theme/_rels/theme1.xml.rels", Data: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/bg.png"/></Relationships>`)},
		fixture.Entry{Name: "ppt/media/bg.png", Data: fixture.PNG(4, 4, blue)},
	)
	img := render(t, buf, 0)
	if n := count(img, image.Rect(300, 200, 400, 300), blue, 12); n < 9000 {
		t.Errorf("expected the blue picture of the theme, got %d blue pixels", n)
	}
}

// TestThemeDuotone checks that a picture of the theme with a:duotone takes
// the colors of the duotone: the white of the picture becomes the second
// color, as the backgrounds of the theme Trek of themes.pptx show it.
func TestThemeDuotone(t *testing.T) {
	p := &fixture.PPTX{
		MasterBackground: `<p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg>`,
		Slides:           []fixture.PPTXSlide{{}},
	}
	buf := p.Bytes()
	theme := entry(t, buf, "ppt/theme/theme1.xml")
	theme = strings.Replace(theme, `<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill>`,
		`<a:bgFillStyleLst><a:blipFill><a:blip r:embed="rId1"><a:duotone><a:srgbClr val="000000"/><a:srgbClr val="FF0000"/></a:duotone></a:blip>`+
			`<a:stretch><a:fillRect/></a:stretch></a:blipFill>`, 1)
	buf = fixture.Rezip(buf,
		fixture.Entry{Name: "ppt/theme/theme1.xml", Data: []byte(theme)},
		fixture.Entry{Name: "ppt/theme/_rels/theme1.xml.rels", Data: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/bg.png"/></Relationships>`)},
		fixture.Entry{Name: "ppt/media/bg.png", Data: fixture.PNG(4, 4, white)},
	)
	img := render(t, buf, 0)
	if n := count(img, image.Rect(300, 200, 400, 300), red, 12); n < 9000 {
		t.Errorf("expected the white picture of the theme in red, got %d red pixels", n)
	}
}

// TestDefaultTableStyle checks that a table with the default table style of
// PowerPoint takes its colors when ppt/tableStyles.xml does not define it,
// as PowerPoint draws custom-geometry.pptx: the first row in accent1.
func TestDefaultTableStyle(t *testing.T) {
	cell := `<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="en-US"/></a:p></a:txBody><a:tcPr/></a:tc>`
	frame := `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="2" name="t"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
		`<p:xfrm><a:off x="1270000" y="1270000"/><a:ext cx="2540000" cy="1270000"/></p:xfrm>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl>` +
		`<a:tblPr firstRow="1" bandRow="1"><a:tableStyleId>{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}</a:tableStyleId></a:tblPr>` +
		`<a:tblGrid><a:gridCol w="1270000"/><a:gridCol w="1270000"/></a:tblGrid>` +
		`<a:tr h="635000">` + cell + cell + `</a:tr><a:tr h="635000">` + cell + cell + `</a:tr>` +
		`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`
	img := render(t, (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: frame}}}).Bytes(), 0)
	// accent1 is 4472C4, and the band of the second row is accent1 with a
	// tint of 40 percent, CFD5EA
	expect(t, img, 150, 120, color.NRGBA{0x44, 0x72, 0xc4, 255})
	expect(t, img, 150, 170, color.NRGBA{0xcf, 0xd5, 0xea, 255})
}
