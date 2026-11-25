package pdfenc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/xo/okage"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/pdf"
)

// program reads the input of FuzzEncode as a sequence of commands of the
// display list.
type program struct {
	data []byte
	face *font.Face
	// depth is the depth of the soft masks that the program draws.
	depth int
}

// byte returns the next byte of the input, or 0 at its end.
func (p *program) byte() byte {
	if len(p.data) == 0 {
		return 0
	}
	b := p.data[0]
	p.data = p.data[1:]
	return b
}

// num returns a number: most often a coordinate of the page, and
// sometimes a number that a page must not hold, such as NaN, an infinity
// or a huge value.
func (p *program) num() float32 {
	b := p.byte()
	switch b {
	case 250:
		return float32(math.NaN())
	case 251:
		return float32(math.Inf(1))
	case 252:
		return -1e30
	case 253:
		return 1e30
	case 254:
		return 0
	case 255:
		return math.MaxFloat32
	}
	return float32(b) - 20
}

// text returns the text of a run of glyphs: letters, a character of two
// bytes, and sometimes a byte that is not valid UTF-8.
func (p *program) text() string {
	var b []byte
	for range int(p.byte() % 6) {
		switch c := p.byte(); c % 8 {
		case 6:
			b = append(b, "é"...)
		case 7:
			b = append(b, c)
		default:
			b = append(b, 'a'+c%26)
		}
	}
	return string(b)
}

// unit returns a number from 0 to 1.
func (p *program) unit() float32 {
	return float32(p.byte()) / 255
}

// color returns a color.
func (p *program) color() okage.Color {
	return okage.ColorFromRGBA8(p.byte(), p.byte(), p.byte(), p.byte())
}

// transform returns a transform.
func (p *program) transform() okage.Transform {
	if p.byte()%4 == 0 {
		return okage.TransformFromRow(p.num()/20, p.num()/40, p.num()/40, p.num()/20, p.num(), p.num())
	}
	return okage.TransformFromTranslate(p.num()/4, p.num()/4)
}

// path returns a path, or nil.
func (p *program) path() *okage.Path {
	pb := okage.NewPathBuilder()
	n := int(p.byte() % 8)
	pb.MoveTo(p.num(), p.num())
	for range n {
		switch p.byte() % 4 {
		case 0:
			pb.LineTo(p.num(), p.num())
		case 1:
			pb.QuadTo(p.num(), p.num(), p.num(), p.num())
		case 2:
			pb.CubicTo(p.num(), p.num(), p.num(), p.num(), p.num(), p.num())
		case 3:
			pb.Close()
		}
	}
	path, ok := pb.Finish()
	if !ok {
		return nil
	}
	return path
}

// stops returns the stops of a gradient.
func (p *program) stops() []okage.GradientStop {
	n := 1 + int(p.byte()%4)
	if p.byte() == 255 {
		n = 300
	}
	stops := make([]okage.GradientStop, n)
	for i := range stops {
		stops[i] = okage.NewGradientStop(p.unit(), p.color())
	}
	return stops
}

// shader returns a shader of each kind.
func (p *program) shader() okage.Shader {
	mode := okage.SpreadMode(p.byte() % 4)
	var sh okage.Shader
	ok := true
	switch p.byte() % 7 {
	case 0, 1:
		return okage.SolidColor(p.color())
	case 2:
		sh, ok = okage.NewLinearGradient(okage.PointFromXY(p.num(), p.num()), okage.PointFromXY(p.num(), p.num()), p.stops(), mode, p.transform())
	case 3:
		sh, ok = okage.NewRadialGradient(okage.PointFromXY(p.num(), p.num()), p.unit()*10, okage.PointFromXY(p.num(), p.num()), p.unit()*100, p.stops(), mode, p.transform())
	case 4:
		start := p.num()
		sh, ok = okage.NewSweepGradient(okage.PointFromXY(p.num(), p.num()), start, start+p.unit()*360, p.stops(), mode, p.transform())
	case 5:
		v := func() okage.MeshVertex {
			return okage.MeshVertex{Point: okage.PointFromXY(p.num(), p.num()), Color: p.color()}
		}
		sh, ok = okage.NewMeshGradient([]okage.MeshTriangle{{v(), v(), v()}}, p.transform())
	case 6:
		pm, err := raster.Pixmap(p.image())
		if err != nil {
			return nil
		}
		sh = okage.NewPattern(pm.AsRef(), mode, okage.FilterQualityBilinear, p.unit(), p.transform())
	}
	if !ok {
		return nil
	}
	return sh
}

// image returns a small image.
func (p *program) image() image.Image {
	w, h := 1+int(p.byte()%6), 1+int(p.byte()%6)
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = p.byte()
	}
	return img
}

// stroke returns a stroke.
func (p *program) stroke() okage.Stroke {
	st := okage.Stroke{
		Width:      p.num() / 8,
		MiterLimit: p.num() / 8,
		LineCap:    okage.LineCap(p.byte() % 3),
		LineJoin:   okage.LineJoin(p.byte() % 4),
	}
	if p.byte()%3 == 0 {
		st.Dash, _ = okage.NewStrokeDash([]float32{p.num(), p.num()}, p.num())
	}
	return st
}

// mask returns a soft mask, whose content is the next commands of the
// program.
func (p *program) mask() *display.Mask {
	if p.byte()%2 == 0 || p.depth > 2 {
		return nil
	}
	m := &display.Mask{Type: okage.MaskType(p.byte() % 2), Backdrop: p.color()}
	if p.byte()%2 == 0 {
		var tr [256]uint8
		for i := range tr {
			tr[i] = uint8(255 - i)
		}
		m.Transfer = &tr
	}
	sub := display.NewRecording()
	p.depth++
	p.run(sub, 1+int(p.byte()%4))
	p.depth--
	m.Content = sub.Replay
	if p.byte()%2 == 0 {
		m.Key = int(p.byte())
	}
	return m
}

// run sends up to n commands to d.
func (p *program) run(d display.Device, n int) {
	for range n {
		if len(p.data) == 0 {
			return
		}
		switch p.byte() % 16 {
		case 0:
			d.Save()
		case 1:
			d.Restore()
		case 2:
			d.Concat(p.transform())
		case 3:
			d.FillPath(p.path(), okage.FillRule(p.byte()%2), p.shader())
		case 4:
			d.StrokePath(p.path(), p.stroke(), p.shader())
		case 5:
			display.StrokeAdjusted(d, p.path(), p.stroke(), p.shader())
		case 6:
			d.ClipPath(p.path(), okage.FillRule(p.byte()%2))
		case 7:
			d.DrawImage(p.image(), p.transform(), display.ImageOptions{Opacity: p.unit() * 1.2, Smooth: p.byte()%2 == 0})
		case 8:
			m := image.NewAlpha(image.Rect(0, 0, 1+int(p.byte()%5), 1+int(p.byte()%5)))
			for i := range m.Pix {
				m.Pix[i] = p.byte()
			}
			d.FillMask(m, p.transform(), display.ImageOptions{Opacity: p.unit() * 1.2}, p.shader())
		case 9:
			display.FillText(d, p.path(), okage.FillRuleWinding, p.shader())
		case 10:
			run := display.GlyphRun{Face: p.face, Size: p.num(), Transform: p.transform(), Text: p.text()}
			for range int(p.byte() % 4) {
				g := display.Glyph{ID: font.GID(p.byte()), X: p.num(), Y: p.num()}
				g.Start, g.End = int(p.byte())%(len(run.Text)+2), int(p.byte())%(len(run.Text)+2)
				if p.byte() == 255 {
					g.ID = 1 << 17
				}
				run.Glyphs = append(run.Glyphs, g)
			}
			d.FillGlyphs(run, p.shader())
		case 11, 12:
			d.BeginGroup(display.Group{
				Opacity:  p.unit() * 1.2,
				Blend:    okage.BlendMode(p.byte() % 30),
				Isolated: p.byte()%2 == 0,
				Knockout: p.byte()%2 == 0,
				Mask:     p.mask(),
			})
		case 13, 14:
			d.EndGroup()
		case 15:
			p.mark(d)
		}
	}
}

// mark sends a link, an anchor or a bookmark to d, or fills a path.
func (p *program) mark(d display.Device) {
	names := [...]string{"a", "b", display.PageAnchor(0, 0, 0), display.PageAnchor(1, 0, 0)}
	switch p.byte() % 5 {
	case 0:
		d.FillPath(p.path(), okage.FillRuleWinding, okage.SolidColor(p.color()))
	case 1:
		if r, ok := okage.RectFromXYWH(p.num(), p.num(), p.num(), p.num()); ok {
			display.LinkURI(d, r, "https://example.com/"+p.text())
		}
	case 2:
		if r, ok := okage.RectFromXYWH(p.num(), p.num(), p.num(), p.num()); ok {
			display.LinkAnchor(d, r, names[p.byte()%4])
		}
	case 3:
		display.Anchor(d, names[p.byte()%4], p.num(), p.num())
	case 4:
		display.Bookmark(d, int(p.byte()%8)-1, p.text(), p.num(), p.num(), p.byte()%2 == 0)
	}
}

// FuzzEncode turns its input into the commands of the display list for two
// pages, and writes them as a PDF file (D58). Encode must not panic, and its
// only error is a limit: the device must not give the writer a command that
// the writer refuses. The file must open with the pdf package of ivy, and
// its pages must render.
func FuzzEncode(f *testing.F) {
	f.Add([]byte{3, 0, 10, 10, 1, 100, 100, 0, 0, 255, 0, 0, 255})
	f.Add([]byte{11, 128, 3, 1, 1, 0, 0, 30, 30, 1, 3, 0, 1, 10, 10, 40, 1, 0, 2, 200, 200, 0, 0, 0, 255, 13})
	f.Add([]byte{6, 4, 10, 10, 0, 100, 10, 0, 100, 100, 3, 0, 7, 3, 3, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 1, 5, 5, 255, 0})
	f.Add([]byte{12, 200, 23, 0, 0, 1, 2, 3, 0, 4, 0, 1, 5, 5, 60, 60, 0, 2, 3, 4, 255, 14, 5, 2, 1, 1, 2, 2, 60, 1, 2, 0, 2, 3, 1, 9, 9})
	f.Add([]byte{15, 2, 10, 10, 50, 20, 1, 15, 4, 2, 3, 97, 98, 99, 0, 0, 1, 15, 3, 0, 30, 30, 15, 1, 5, 5, 20, 20, 2, 97, 98})
	f.Add([]byte{10, 44, 1, 20, 20, 3, 0, 1, 2, 3, 40, 60, 120, 0, 1, 0, 41, 80, 120, 1, 2, 0, 42, 100, 120, 2, 3, 0, 0, 0, 0, 0, 0, 255})
	face, err := font.Load(goregular.TTF, 0)
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			return
		}
		half := len(data) / 2
		pages := []ivy.Page{
			&scenePage{w: 120, h: 90, draw: func(d display.Device) {
				(&program{data: data[:half], face: face}).run(d, 64)
			}},
			&scenePage{w: 60, h: 80, draw: func(d display.Device) {
				(&program{data: data[half:], face: face}).run(d, 64)
			}},
		}
		var buf bytes.Buffer
		cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 64 << 20, MaxPixels: 1 << 20}}
		err := Encode(t.Context(), &buf, &document{pages: pages}, WithConfig(cfg), WithRasterDPI(36))
		if err != nil {
			if !errors.Is(err, ivy.ErrLimit) {
				t.Fatalf("expected ErrLimit, got: %v", err)
			}
			return
		}
		doc, err := pdf.Open(t.Context(), bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("opening the written file: %v", err)
		}
		defer doc.Close()
		n, err := doc.PageCount(t.Context())
		if err != nil || n != 2 {
			t.Fatalf("expected 2 pages, got: %d, %v", n, err)
		}
		for i := range n {
			page, err := doc.Page(t.Context(), i)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ivy.RenderImage(t.Context(), page, 18, color.White); err != nil {
				t.Fatalf("rendering page %d: %v", i+1, err)
			}
			// the links, the anchors and the outline read back
			if err := page.(display.Drawer).Draw(t.Context(), display.NewRecording(), 1); err != nil {
				t.Fatalf("drawing page %d: %v", i+1, err)
			}
		}
	})
}
