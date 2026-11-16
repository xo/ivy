package pptx

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// contentTypes is a minimal [Content_Types].xml.
var contentTypes = fixture.Entry{Name: "[Content_Types].xml", Data: []byte("<Types/>")}

func TestOpenLibreOffice(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "sample.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	})
	// LibreOffice writes no thumbnail into an Office Open XML file
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
}

func TestThumbnail(t *testing.T) {
	buf := fixture.Zip(
		contentTypes,
		fixture.Entry{Name: "docProps/thumbnail.jpeg", Data: fixture.JPEG(40, 30, color.White)},
	)
	d, err := Format.Open(t.Context(), bytes.NewReader(buf), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(40, 30) {
		t.Errorf("expected 40x30, got: %v", img.Bounds().Size())
	}
}

func TestThumbnailEMF(t *testing.T) {
	buf := fixture.Zip(contentTypes, fixture.Entry{Name: "docProps/thumbnail.emf", Data: []byte("emf")})
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview until the EMF reader, got: %v", err)
	}
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader(fixture.Zip(fixture.Entry{Name: "a"}))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	cfb := append([]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, make([]byte, 504)...)
	if _, err := Open(t.Context(), bytes.NewReader(cfb)); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat for a damaged compound file, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	buf, err := os.ReadFile(filepath.Join("testdata", "sample.pptx"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf)
	f.Add(fixture.Zip(contentTypes, fixture.Entry{Name: "docProps/thumbnail.png", Data: fixture.PNG(4, 4, color.Black)}))
	text := `<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:pPr><a:buAutoNum type="arabicPeriod"/></a:pPr><a:r><a:t>x</a:t></a:r></a:p></p:txBody>`
	f.Add((&fixture.PPTX{
		Slides: []fixture.PPTXSlide{{Shapes: shape(2, xfrm(10, 10, 100, 50, ` rot="600000"`)+
			`<a:prstGeom prst="star5"><a:avLst/></a:prstGeom><a:gradFill><a:gsLst><a:gs pos="0"><a:schemeClr val="accent1"/></a:gs></a:gsLst></a:gradFill>`, text) +
			`<p:pic><p:nvPicPr><p:cNvPr id="3" name="p"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rIdM1"/><a:srcRect l="10000"/></p:blipFill><p:spPr>` +
			xfrm(0, 0, 10, 10, "") + `</p:spPr></p:pic>`}},
		Media: []fixture.Entry{{Name: "a.png", Data: fixture.PNG(2, 2, color.White)}},
	}).Bytes())
	f.Add((&fixture.PPTX{
		Slides: []fixture.PPTXSlide{{Shapes: `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="2" name="c"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
			`<p:xfrm><a:off x="0" y="0"/><a:ext cx="1270000" cy="1270000"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
			`<c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="rIdC1"/></a:graphicData></a:graphic></p:graphicFrame>`}},
		Charts: []fixture.Entry{{Name: "chart1.xml", Data: fixture.Chart("FF0000")}},
	}).Bytes())
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 1 << 12}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		for p, err := range d.Previews(t.Context()) {
			if err != nil {
				break
			}
			_, _ = p.Decode(t.Context())
		}
		if _, err := d.PageCount(t.Context()); err != nil {
			return
		}
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		_, _ = ivy.RenderImage(t.Context(), p, 8, nil)
	})
}
