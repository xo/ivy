package pdfenc

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/xo/okage"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/pdf"
)

// scenePage is a page that draws a scene of the display list.
type scenePage struct {
	w, h float32
	draw func(d display.Device)
	err  error
}

// The interfaces that a scenePage implements.
var (
	_ ivy.Page       = (*scenePage)(nil)
	_ display.Drawer = (*scenePage)(nil)
)

func (p *scenePage) Size() (float32, float32) {
	return p.w, p.h
}

func (p *scenePage) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error {
	return p.Draw(ctx, display.NewOkage(dst, t), 1)
}

func (p *scenePage) Draw(_ context.Context, d display.Device, _ float32) error {
	if p.draw != nil {
		p.draw(d)
	}
	return p.err
}

// renderPage is a page that only renders: it is not a display.Drawer.
type renderPage struct {
	page *scenePage
}

func (p *renderPage) Size() (float32, float32) {
	return p.page.Size()
}

func (p *renderPage) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error {
	return p.page.Render(ctx, dst, t)
}

// document is a document of pages, with its information.
type document struct {
	pages []ivy.Page
	info  ivy.Info
}

// The interfaces that a document implements.
var (
	_ ivy.Pager     = (*document)(nil)
	_ ivy.Describer = (*document)(nil)
)

func (d *document) Close() error {
	return nil
}

func (d *document) PageCount(context.Context) (int, error) {
	return len(d.pages), nil
}

func (d *document) Page(_ context.Context, n int) (ivy.Page, error) {
	if n < 0 || n >= len(d.pages) {
		return nil, ivy.ErrPageRange
	}
	return d.pages[n], nil
}

func (d *document) Describe(context.Context) (ivy.Info, error) {
	return d.info, nil
}

// rect returns the path of a rectangle.
func rect(x, y, w, h float32) *okage.Path {
	r, _ := okage.RectFromXYWH(x, y, w, h)
	return okage.PathFromRect(r)
}

// circle returns the path of a circle.
func circle(cx, cy, r float32) *okage.Path {
	pb := okage.NewPathBuilder()
	pb.PushCircle(cx, cy, r)
	p, _ := pb.Finish()
	return p
}

// solid returns a shader of one color.
func solid(r, g, b, a uint8) okage.Shader {
	return okage.SolidColor(okage.ColorFromRGBA8(r, g, b, a))
}

// stops returns the stops of a gradient from red to blue.
func stops() []okage.GradientStop {
	return []okage.GradientStop{
		okage.NewGradientStop(0, okage.ColorFromRGBA8(220, 30, 30, 255)),
		okage.NewGradientStop(1, okage.ColorFromRGBA8(30, 30, 220, 255)),
	}
}

// checker returns an image of 8 by 8 pixels in squares of colors, with an
// alpha.
func checker() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			c := color.NRGBA{R: uint8(x * 32), G: uint8(y * 32), B: 160, A: 255}
			if (x+y)%2 == 0 {
				c.A = 128
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// stencil returns a stencil of 16 by 16 pixels with a ramp of coverage.
func stencil() *image.Alpha {
	m := image.NewAlpha(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			if (x-8)*(x-8)+(y-8)*(y-8) < 49 {
				m.SetAlpha(x, y, color.Alpha{A: uint8(255 - x*8)})
			}
		}
	}
	return m
}

// overlap draws two overlapping squares, red and green.
func overlap(d display.Device) {
	d.FillPath(rect(30, 30, 90, 90), okage.FillRuleWinding, solid(220, 40, 40, 255))
	d.FillPath(rect(80, 80, 90, 90), okage.FillRuleWinding, solid(40, 180, 40, 255))
}

// scene is a scene of the tests, with the largest mean difference of the
// round trip.
type scene struct {
	name string
	draw func(t *testing.T, d display.Device)
	diff float64
	// want lists strings that the written file holds, and notWant the
	// strings that it does not hold. The streams of the file count
	// inflated.
	want, notWant []string
	// clear renders the scene over a transparent background, for the
	// raster fallback, which draws a part without its backdrop.
	clear bool
}

// scenes returns the scenes of the tests, one for each command of the
// device and each conversion.
func scenes() []scene {
	return []scene{
		{name: "fill", draw: func(_ *testing.T, d display.Device) {
			d.FillPath(circle(100, 100, 60), okage.FillRuleWinding, solid(30, 90, 200, 255))
			d.FillPath(rect(20, 20, 60, 40), okage.FillRuleEvenOdd, solid(200, 30, 30, 128))
		}, diff: 2},
		{name: "stroke", draw: func(_ *testing.T, d display.Device) {
			dash, _ := okage.NewStrokeDash([]float32{12, 6}, 3)
			st := okage.Stroke{Width: 6, MiterLimit: 4, LineCap: okage.LineCapRound, LineJoin: okage.LineJoinRound, Dash: dash}
			d.StrokePath(circle(100, 100, 60), st, solid(20, 20, 20, 255))
			st = okage.Stroke{Width: 10, MiterLimit: 10, LineJoin: okage.LineJoinBevel}
			d.StrokePath(rect(40, 40, 120, 120), st, solid(200, 120, 0, 255))
		}, diff: 2},
		{name: "stroke adjusted", draw: func(_ *testing.T, d display.Device) {
			for i := range 10 {
				y := 20 + float32(i)*16.3
				pb := okage.NewPathBuilder()
				pb.MoveTo(10, y)
				pb.LineTo(190, y)
				p, _ := pb.Finish()
				display.StrokeAdjusted(d, p, okage.Stroke{Width: 0.6, MiterLimit: 4}, solid(0, 0, 0, 255))
			}
		}, diff: 2, want: []string{"/SA true"}},
		{name: "clip", draw: func(_ *testing.T, d display.Device) {
			d.Save()
			d.ClipPath(circle(100, 100, 50), okage.FillRuleWinding)
			d.Concat(okage.TransformFromRow(1, 0.2, 0, 1, 0, -10))
			d.FillPath(rect(0, 0, 200, 200), okage.FillRuleWinding, solid(0, 140, 70, 255))
			d.Restore()
			d.FillPath(rect(150, 150, 40, 40), okage.FillRuleWinding, solid(0, 0, 0, 255))
		}, diff: 2},
		{name: "gradients", draw: func(_ *testing.T, d display.Device) {
			lin, _ := okage.NewLinearGradient(okage.PointFromXY(0, 0), okage.PointFromXY(100, 0), stops(), okage.SpreadModeReflect, okage.IdentityTransform())
			d.FillPath(rect(0, 0, 200, 60), okage.FillRuleWinding, lin)
			rad, _ := okage.NewRadialGradient(okage.PointFromXY(60, 130), 0, okage.PointFromXY(60, 130), 50, stops(), okage.SpreadModePad, okage.IdentityTransform())
			d.FillPath(circle(60, 130, 50), okage.FillRuleWinding, rad)
			sweep, _ := okage.NewSweepGradient(okage.PointFromXY(150, 130), 0, 360, stops(), okage.SpreadModePad, okage.IdentityTransform())
			d.FillPath(circle(150, 130, 40), okage.FillRuleWinding, sweep)
		}, diff: 2},
		{name: "mesh", draw: func(_ *testing.T, d display.Device) {
			v := func(x, y float32, r, g, b uint8) okage.MeshVertex {
				return okage.MeshVertex{Point: okage.PointFromXY(x, y), Color: okage.ColorFromRGBA8(r, g, b, 255)}
			}
			mesh, _ := okage.NewMeshGradient([]okage.MeshTriangle{
				{v(20, 20, 255, 0, 0), v(180, 20, 0, 255, 0), v(100, 180, 0, 0, 255)},
			}, okage.IdentityTransform())
			d.FillPath(rect(20, 20, 160, 160), okage.FillRuleWinding, mesh)
		}, diff: 2},
		{name: "pattern", draw: func(t *testing.T, d display.Device) {
			pm, err := raster.Pixmap(checker())
			if err != nil {
				t.Fatal(err)
			}
			pat := okage.NewPattern(pm.AsRef(), okage.SpreadModeRepeat, okage.FilterQualityNearest, 1, okage.TransformFromScale(5, 5))
			d.FillPath(rect(10, 10, 180, 180), okage.FillRuleWinding, pat)
		}, diff: 2},
		{name: "fallback shader", draw: func(_ *testing.T, d display.Device) {
			rad, _ := okage.NewRadialGradient(okage.PointFromXY(100, 100), 0, okage.PointFromXY(100, 100), 30, stops(), okage.SpreadModeRepeat, okage.IdentityTransform())
			d.FillPath(circle(100, 100, 80), okage.FillRuleWinding, rad)
		}, diff: 3},
		{name: "group", draw: func(_ *testing.T, d display.Device) {
			d.BeginGroup(display.Group{Opacity: 0.5, Isolated: true})
			overlap(d)
			d.EndGroup()
		}, diff: 2},
		{name: "blend", draw: func(_ *testing.T, d display.Device) {
			d.FillPath(rect(0, 0, 200, 100), okage.FillRuleWinding, solid(250, 200, 0, 255))
			d.BeginGroup(display.Group{Opacity: 1, Blend: okage.BlendModeMultiply})
			overlap(d)
			d.EndGroup()
		}, diff: 2, want: []string{"/BM/Multiply"}},
		{name: "knockout", draw: func(_ *testing.T, d display.Device) {
			d.FillPath(rect(0, 0, 200, 100), okage.FillRuleWinding, solid(0, 0, 200, 255))
			d.BeginGroup(display.Group{Opacity: 1, Knockout: true, Isolated: true})
			d.FillPath(rect(30, 30, 90, 90), okage.FillRuleWinding, solid(220, 40, 40, 128))
			d.FillPath(rect(80, 80, 90, 90), okage.FillRuleWinding, solid(40, 180, 40, 128))
			d.EndGroup()
		}, diff: 2, want: []string{"/K true"}},
		{name: "mask", draw: func(_ *testing.T, d display.Device) {
			var transfer [256]uint8
			for i := range transfer {
				transfer[i] = uint8(255 - i)
			}
			d.BeginGroup(display.Group{Opacity: 1, Mask: &display.Mask{
				Type:     okage.MaskTypeLuminance,
				Backdrop: okage.ColorFromRGBA8(40, 40, 40, 255),
				Transfer: &transfer,
				Content: func(d display.Device) {
					d.FillPath(circle(100, 100, 60), okage.FillRuleWinding, solid(255, 255, 255, 255))
				},
				Key: "mask",
			}})
			overlap(d)
			d.EndGroup()
		}, diff: 2, want: []string{"/SMask", "/BC", "/TR"}},
		{name: "fallback blend", draw: func(_ *testing.T, d display.Device) {
			d.BeginGroup(display.Group{Opacity: 1, Blend: okage.BlendModeXor})
			overlap(d)
			d.EndGroup()
		}, diff: 2, clear: true},
		{name: "image", draw: func(_ *testing.T, d display.Device) {
			img := checker()
			d.DrawImage(img, display.RectTransform(img.Rect.Size(), okage.Rect{}), display.ImageOptions{Opacity: 1})
			r, _ := okage.RectFromXYWH(20, 20, 80, 80)
			d.DrawImage(img, display.RectTransform(img.Rect.Size(), r), display.ImageOptions{Opacity: 1})
			r, _ = okage.RectFromXYWH(100, 100, 80, 80)
			d.DrawImage(img, display.RectTransform(img.Rect.Size(), r), display.ImageOptions{Opacity: 0.5})
		}, diff: 2},
		{name: "stencil", draw: func(_ *testing.T, d display.Device) {
			r, _ := okage.RectFromXYWH(10, 10, 80, 80)
			d.FillMask(stencil(), display.RectTransform(image.Pt(16, 16), r), display.ImageOptions{Opacity: 1}, solid(200, 0, 100, 255))
			lin, _ := okage.NewLinearGradient(okage.PointFromXY(100, 0), okage.PointFromXY(200, 0), stops(), okage.SpreadModePad, okage.IdentityTransform())
			r, _ = okage.RectFromXYWH(110, 110, 80, 80)
			d.FillMask(stencil(), display.RectTransform(image.Pt(16, 16), r), display.ImageOptions{Opacity: 1}, lin)
		}, diff: 2},
		{name: "glyphs", draw: func(t *testing.T, d display.Device) {
			face, err := font.Load(goregular.TTF, 0)
			if err != nil {
				t.Fatal(err)
			}
			run := display.GlyphRun{Face: face, Size: 24, Transform: okage.IdentityTransform()}
			x := float32(10)
			for _, r := range "Hello, PDF" {
				gid, _ := face.Glyph(r)
				run.Glyphs = append(run.Glyphs, display.Glyph{ID: gid, X: x, Y: 100})
				x += face.Advance(gid) * 24
			}
			d.FillGlyphs(run, solid(0, 0, 0, 255))
		}, diff: 2, notWant: []string{"/FontFile2"}},
		{name: "text", draw: func(t *testing.T, d display.Device) {
			d.FillGlyphs(textRun(t, goregular.TTF, "Hello, PDF", 10, 100, okage.IdentityTransform()), solid(0, 0, 0, 255))
		}, diff: 2, want: []string{"/Type0", "/FontFile2", "/ToUnicode", "+GoRegular", "<0048>"}},
		{name: "text transform", draw: func(t *testing.T, d display.Device) {
			// a slant, a width and a turn, as the faces of XPS and the
			// metafiles draw them
			slant := okage.TransformFromRow(1, 0, -0.36397, 1, 0, 0)
			d.FillGlyphs(textRun(t, goregular.TTF, "Slanted", 10, 40, slant), solid(0, 0, 0, 255))
			wide := okage.TransformFromScale(1.5, 1)
			d.FillGlyphs(textRun(t, goregular.TTF, "Wide", 10, 80, wide), solid(0, 0, 160, 255))
			turn := okage.TransformFromRow(0, -1, 1, 0, 0, 0)
			d.FillGlyphs(textRun(t, goregular.TTF, "Up", 150, 180, turn), solid(160, 0, 0, 255))
			d.Save()
			d.Concat(okage.TransformFromRotate(10))
			d.FillGlyphs(textRun(t, goregular.TTF, "Turned", 40, 120, okage.IdentityTransform()), solid(0, 120, 0, 255))
			d.Restore()
		}, diff: 2, want: []string{"/ToUnicode"}},
		{name: "text gradient", draw: func(t *testing.T, d display.Device) {
			lin, _ := okage.NewLinearGradient(okage.PointFromXY(10, 0), okage.PointFromXY(190, 0), stops(), okage.SpreadModePad, okage.IdentityTransform())
			run := textRun(t, goregular.TTF, "Gradient", 10, 100, okage.TransformFromScale(1.2, 1))
			run.Size = 36
			d.FillGlyphs(run, lin)
		}, diff: 2, want: []string{"/ToUnicode", "/ShadingType"}},
		{name: "text clusters", draw: func(t *testing.T, d display.Device) {
			// two glyphs of one cluster, and one glyph of two characters
			run := textRun(t, goregular.TTF, "ab", 10, 100, okage.IdentityTransform())
			run.Text = "a\u0301bc"
			run.Glyphs[0].Start, run.Glyphs[0].End = 0, 3
			run.Glyphs = append(run.Glyphs[:1], run.Glyphs...)
			run.Glyphs[2].Start, run.Glyphs[2].End = 3, 5
			d.FillGlyphs(run, solid(0, 0, 0, 255))
		}, diff: 2, want: []string{"/ActualText"}},
		{name: "text bad range", draw: func(t *testing.T, d display.Device) {
			run := textRun(t, goregular.TTF, "Ranges", 10, 100, okage.IdentityTransform())
			run.Text = "\u00e9"
			run.Glyphs[0].End = 1
			d.FillGlyphs(run, solid(0, 0, 0, 255))
		}, diff: 2, notWant: []string{"/FontFile2"}},
		{name: "text restricted", draw: func(t *testing.T, d display.Device) {
			data := bytes.Clone(goregular.TTF)
			setFSType(t, data, 0x0002)
			d.FillGlyphs(textRun(t, data, "Restricted", 10, 100, okage.IdentityTransform()), solid(0, 0, 0, 255))
		}, diff: 2, notWant: []string{"/FontFile2"}},
	}
}

// textRun returns a run of the glyphs of s in the font data, at 24 points,
// from x, y, with its text.
func textRun(t *testing.T, data []byte, s string, x, y float32, tr okage.Transform) display.GlyphRun {
	t.Helper()
	face, err := font.Load(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	run := display.GlyphRun{Face: face, Size: 24, Transform: tr, Text: s}
	sx, _ := tr.GetScale()
	for i, r := range s {
		gid, _ := face.Glyph(r)
		run.Glyphs = append(run.Glyphs, display.Glyph{ID: gid, X: x, Y: y, Start: i, End: i + utf8.RuneLen(r)})
		x += face.Advance(gid) * 24 * max(sx, 1)
	}
	return run
}

// setFSType sets the field fsType of the table OS/2 of the font data to v.
func setFSType(t *testing.T, data []byte, v uint16) {
	t.Helper()
	off := tableOffset(t, data, "OS/2")
	binary.BigEndian.PutUint16(data[off+8:], v)
}

// tableOffset returns the offset of the table tag of the font data.
func tableOffset(t *testing.T, data []byte, tag string) int {
	t.Helper()
	n := int(binary.BigEndian.Uint16(data[4:]))
	for i := range n {
		rec := 12 + 16*i
		if string(data[rec:rec+4]) == tag {
			return int(binary.BigEndian.Uint32(data[rec+8:]))
		}
	}
	t.Fatalf("expected the table %s", tag)
	return 0
}

// inflated returns the bytes of the file, with the inflated data of each
// stream of the file after them.
func inflated(t *testing.T, data []byte) []byte {
	t.Helper()
	all := bytes.Clone(data)
	rest := data
	for {
		_, after, ok := bytes.Cut(rest, []byte("stream\n"))
		if !ok {
			return all
		}
		body, next, ok := bytes.Cut(after, []byte("endstream"))
		if !ok {
			return all
		}
		rest = next
		r, err := zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(r)
		all = append(all, b...)
	}
}

// encodeScene writes the scene as a PDF file of one page.
func encodeScene(t *testing.T, sc scene, opts ...Option) ([]byte, *scenePage) {
	t.Helper()
	page := &scenePage{w: 200, h: 200, draw: func(d display.Device) { sc.draw(t, d) }}
	var buf bytes.Buffer
	pages := func(yield func(ivy.Page, error) bool) { yield(page, nil) }
	if err := EncodePages(t.Context(), &buf, pages, opts...); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), page
}

// openPage opens the file and returns page n of it.
func openPage(t *testing.T, data []byte, n int) ivy.Page {
	t.Helper()
	doc, err := pdf.Open(t.Context(), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = doc.Close() })
	page, err := doc.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

// TestScenes writes each scene, opens the file with the pdf package of
// ivy, and compares its page with the scene, both at 72 dots for each inch.
func TestScenes(t *testing.T) {
	for _, sc := range scenes() {
		t.Run(sc.name, func(t *testing.T) {
			data, src := encodeScene(t, sc, WithRasterDPI(144))
			all := inflated(t, data)
			for _, s := range sc.want {
				if !bytes.Contains(all, []byte(s)) {
					t.Errorf("expected the file to hold %q", s)
				}
			}
			for _, s := range sc.notWant {
				if bytes.Contains(all, []byte(s)) {
					t.Errorf("expected the file not to hold %q", s)
				}
			}
			got := openPage(t, data, 0)
			if w, h := got.Size(); w != 200 || h != 200 {
				t.Fatalf("expected a page of 200x200 points, got: %gx%g", w, h)
			}
			var bg color.Color = color.White
			if sc.clear {
				bg = nil
			}
			want, err := ivy.RenderImage(t.Context(), src, 72, bg)
			if err != nil {
				t.Fatal(err)
			}
			img, err := ivy.RenderImage(t.Context(), got, 72, bg)
			if err != nil {
				t.Fatal(err)
			}
			diff := meanDiff(img, want)
			t.Logf("mean difference: %.2f", diff)
			if diff < 0 || diff > sc.diff {
				t.Errorf("expected a mean difference of %g at most, got: %.2f", sc.diff, diff)
			}
		})
	}
}

// meanDiff returns the mean absolute difference of the channels of a and b,
// each over white, from 0 to 255, as meanDiff of the test module does. It
// returns -1 if the sizes differ.
func meanDiff(a, b image.Image) float64 {
	if a.Bounds().Size() != b.Bounds().Size() {
		return -1
	}
	var sum, n float64
	ab, bb := a.Bounds(), b.Bounds()
	for y := range ab.Dy() {
		for x := range ab.Dx() {
			ca := overWhite(a.At(ab.Min.X+x, ab.Min.Y+y))
			cb := overWhite(b.At(bb.Min.X+x, bb.Min.Y+y))
			for i := range ca {
				sum += math.Abs(ca[i] - cb[i])
				n++
			}
		}
	}
	return sum / n
}

// overWhite returns c over white, in 8 bits for each channel.
func overWhite(c color.Color) [3]float64 {
	r, g, b, a := c.RGBA()
	white := float64(0xffff - a)
	return [3]float64{(float64(r) + white) / 257, (float64(g) + white) / 257, (float64(b) + white) / 257}
}

// TestImages makes sure that the file holds an image that pages share once,
// and that an image keeps the flag Interpolate.
func TestImages(t *testing.T) {
	img := checker()
	draw := func(d display.Device) {
		d.DrawImage(img, okage.TransformFromScale(10, 10), display.ImageOptions{Opacity: 1, Smooth: true})
	}
	doc := &document{pages: []ivy.Page{&scenePage{w: 100, h: 100, draw: draw}, &scenePage{w: 100, h: 100, draw: draw}}}
	var buf bytes.Buffer
	if err := Encode(t.Context(), &buf, doc); err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(buf.Bytes(), []byte("/Subtype/Image")); n != 2 {
		// the image and its soft mask
		t.Errorf("expected one image with its soft mask, got: %d images", n)
	}
	if !bytes.Contains(buf.Bytes(), []byte("/Interpolate true")) {
		t.Error("expected an image with /Interpolate true")
	}
}

// TestRasterPage makes sure that a raster page goes into the file as one
// image at its own pixels, and that a page that is not a display.Drawer
// goes into the file as an image of ivy.RenderImage.
func TestRasterPage(t *testing.T) {
	src := checker()
	page := raster.NewPage(src.Rect.Size(), func(context.Context) (image.Image, error) { return src, nil })
	only := &renderPage{&scenePage{w: 50, h: 40, draw: overlapSmall}}
	doc := &document{pages: []ivy.Page{page, only}}
	var buf bytes.Buffer
	if err := Encode(t.Context(), &buf, doc, WithRasterDPI(72)); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"/Width 8", "/Height 8", "/Width 50", "/Height 40"} {
		if !bytes.Contains(buf.Bytes(), []byte(s)) {
			t.Errorf("expected the file to hold %q", s)
		}
	}
	for i, src := range doc.pages {
		got := openPage(t, buf.Bytes(), i)
		gw, gh := got.Size()
		sw, sh := src.Size()
		if gw != sw || gh != sh {
			t.Errorf("page %d: expected a page of %gx%g points, got: %gx%g", i+1, sw, sh, gw, gh)
		}
		want, err := ivy.RenderImage(t.Context(), src, 72, color.White)
		if err != nil {
			t.Fatal(err)
		}
		img, err := ivy.RenderImage(t.Context(), got, 72, color.White)
		if err != nil {
			t.Fatal(err)
		}
		if diff := meanDiff(img, want); diff < 0 || diff > 2 {
			t.Errorf("page %d: expected a mean difference of 2 at most, got: %.2f", i+1, diff)
		}
	}
}

// overlapSmall draws two overlapping squares on a small page.
func overlapSmall(d display.Device) {
	d.FillPath(rect(5, 5, 25, 20), okage.FillRuleWinding, solid(220, 40, 40, 255))
	d.FillPath(rect(20, 15, 25, 20), okage.FillRuleWinding, solid(40, 180, 40, 255))
}

// TestMetadata makes sure that the information of the document goes into
// the file, from Describe or from WithInfo.
func TestMetadata(t *testing.T) {
	created := time.Date(2024, 5, 6, 7, 8, 9, 0, time.FixedZone("", 2*3600))
	info := ivy.Info{Title: "The title", Authors: []string{"An author"}, Subject: "A subject", Language: "en-US", Created: created, Producer: "A program"}
	for _, tt := range []struct {
		name string
		info ivy.Info
		opts []Option
	}{
		{name: "describe", info: info},
		{name: "option", opts: []Option{WithInfo(info)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := &document{pages: []ivy.Page{&scenePage{w: 100, h: 100, draw: overlapSmall}}, info: tt.info}
			var buf bytes.Buffer
			if err := Encode(t.Context(), &buf, doc, tt.opts...); err != nil {
				t.Fatal(err)
			}
			got, err := pdf.Open(t.Context(), bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = got.Close() }()
			in, err := got.Describe(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if in.Title != info.Title || in.Subject != info.Subject || in.Language != info.Language ||
				len(in.Authors) != 1 || in.Authors[0] != info.Authors[0] || !in.Created.Equal(created) {
				t.Errorf("expected the information %+v, got: %+v", info, in)
			}
			if !bytes.Contains(buf.Bytes(), []byte("(A program)")) {
				t.Error("expected the producer of the document as the creator of the file")
			}
		})
	}
}

// TestPages makes sure that WithPages writes the pages in its order, and
// that a page past the last one fails before Encode writes a byte.
func TestPages(t *testing.T) {
	small := &scenePage{w: 50, h: 60, draw: overlapSmall}
	large := &scenePage{w: 100, h: 120, draw: overlapSmall}
	doc := &document{pages: []ivy.Page{small, large}}
	var buf bytes.Buffer
	if err := Encode(t.Context(), &buf, doc, WithPages(1, 0, 1)); err != nil {
		t.Fatal(err)
	}
	for i, want := range []float32{100, 50, 100} {
		if w, _ := openPage(t, buf.Bytes(), i).Size(); w != want {
			t.Errorf("page %d: expected a width of %g, got: %g", i+1, want, w)
		}
	}
	buf.Reset()
	if err := Encode(t.Context(), &buf, doc, WithPages(0, 2)); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ErrPageRange, got: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no bytes, got: %d", buf.Len())
	}
	pages := func(yield func(ivy.Page, error) bool) {
		_ = yield(small, nil) && yield(large, nil)
	}
	if err := EncodePages(t.Context(), &buf, pages, WithPages(2)); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ErrPageRange, got: %v", err)
	}
}

// failWriter is a writer that fails.
type failWriter struct{}

var errWrite = errors.New("write failed")

func (failWriter) Write([]byte) (int, error) {
	return 0, errWrite
}

// TestErrors makes sure of the errors of D58: a page drawn in part, a page
// that fails, the writer, the memory, the context and a document without
// pages.
func TestErrors(t *testing.T) {
	partial := &scenePage{w: 50, h: 50, draw: overlapSmall, err: &ivy.PartialError{Err: ivy.ErrFormat}}
	failed := &scenePage{w: 50, h: 50, draw: overlapSmall, err: ivy.ErrFormat}
	good := &scenePage{w: 50, h: 50, draw: overlapSmall}

	var buf bytes.Buffer
	err := Encode(t.Context(), &buf, &document{pages: []ivy.Page{good, partial}})
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok || !strings.Contains(err.Error(), "page 2") {
		t.Errorf("expected a *ivy.PartialError that names page 2, got: %v", err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("expected the file of a page drawn in part")
	}

	buf.Reset()
	err = Encode(t.Context(), &buf, &document{pages: []ivy.Page{good, failed}})
	if !errors.Is(err, ivy.ErrFormat) || !strings.Contains(err.Error(), "page 2") {
		t.Errorf("expected ErrFormat that names page 2, got: %v", err)
	}
	if _, ok := errors.AsType[*ivy.PartialError](err); ok || buf.Len() != 0 {
		t.Errorf("expected no file and no *ivy.PartialError, got: %d bytes and %v", buf.Len(), err)
	}

	if err := Encode(t.Context(), failWriter{}, &document{pages: []ivy.Page{good}}); !errors.Is(err, errWrite) {
		t.Errorf("expected the error of the writer, got: %v", err)
	}

	img := &scenePage{w: 100, h: 100, draw: func(d display.Device) {
		d.DrawImage(image.NewRGBA(image.Rect(0, 0, 100, 100)), okage.IdentityTransform(), display.ImageOptions{Opacity: 1})
	}}
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 10000}}
	buf.Reset()
	if err := Encode(t.Context(), &buf, &document{pages: []ivy.Page{img}}, WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) || buf.Len() != 0 {
		t.Errorf("expected ErrLimit and no file, got: %v and %d bytes", err, buf.Len())
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Encode(ctx, &buf, &document{pages: []ivy.Page{good}}); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}

	if err := Encode(t.Context(), io.Discard, struct{ ivy.Document }{}); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("expected ErrUnsupported for a document without pages, got: %v", err)
	}

	bad := &scenePage{w: float32(math.NaN()), h: 10}
	if err := Encode(t.Context(), io.Discard, &document{pages: []ivy.Page{bad}}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat for a page without a size, got: %v", err)
	}
}

// TestSaveRestore makes sure that the device keeps the pushes of the
// surface in balance whatever the order of the commands, so that the file
// is written.
func TestSaveRestore(t *testing.T) {
	draw := func(d display.Device) {
		d.Restore()
		d.EndGroup()
		d.Save()
		d.Concat(okage.TransformFromScale(2, 2))
		d.BeginGroup(display.Group{Opacity: 0.5})
		d.Save()
		d.ClipPath(rect(0, 0, 40, 40), okage.FillRuleWinding)
		d.Restore()
		d.Restore()
		d.Restore()
		d.FillPath(rect(0, 0, 20, 20), okage.FillRuleWinding, solid(0, 0, 0, 255))
		d.BeginGroup(display.Group{Opacity: 1, Blend: okage.BlendModeDestinationOut})
		d.Save()
		d.BeginGroup(display.Group{Opacity: 1})
		d.Concat(okage.TransformFromRow(0, 0, 0, 0, 0, 0))
		d.FillPath(rect(0, 0, 20, 20), okage.FillRuleWinding, solid(0, 0, 0, 255))
	}
	var buf bytes.Buffer
	if err := Encode(t.Context(), &buf, &document{pages: []ivy.Page{&scenePage{w: 100, h: 100, draw: draw}}}); err != nil {
		t.Fatal(err)
	}
	openPage(t, buf.Bytes(), 0)
}

// TestTextSubsetFails draws text with a font whose subset the writer
// cannot make: the location of a glyph points past the table glyf. Encode
// draws the pages again with the glyphs of that font as paths (D58).
func TestTextSubsetFails(t *testing.T) {
	data := bytes.Clone(goregular.TTF)
	head := tableOffset(t, data, "head")
	loca := tableOffset(t, data, "loca")
	face, err := font.Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	gid, _ := face.Glyph('A')
	if binary.BigEndian.Uint16(data[head+50:]) == 0 {
		binary.BigEndian.PutUint16(data[loca+2*int(gid)+2:], 0xFFFF)
	} else {
		binary.BigEndian.PutUint32(data[loca+4*int(gid)+4:], 0x7FFFFFF0)
	}
	draws := 0
	page := &scenePage{w: 200, h: 200, draw: func(d display.Device) {
		draws++
		d.FillGlyphs(textRun(t, data, "BAD", 10, 100, okage.IdentityTransform()), solid(0, 0, 0, 255))
		d.FillGlyphs(textRun(t, goregular.TTF, "Good", 10, 150, okage.IdentityTransform()), solid(0, 0, 0, 255))
	}}
	var buf bytes.Buffer
	if err := Encode(t.Context(), &buf, &document{pages: []ivy.Page{page}}); err != nil {
		t.Fatal(err)
	}
	if draws != 2 {
		t.Errorf("expected the page drawn twice, got: %d", draws)
	}
	if n := bytes.Count(buf.Bytes(), []byte("/FontFile2")); n != 1 {
		t.Errorf("expected one embedded font, got: %d", n)
	}
	openPage(t, buf.Bytes(), 0)
}
