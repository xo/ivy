package msppt

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/magic/cfb"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/fixture"
)

// msoPassword is the password of the test files of msoffcrypto-tool.
const msoPassword = "Password1234_"

func readFile(t testing.TB, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

// slidePage returns the page n of d, with its figures.
func slidePage(t *testing.T, d *Document, n int) *page {
	t.Helper()
	p, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	return p.(*page)
}

// textOf returns the text of a figure, with a line feed between two
// paragraphs.
func textOf(f *drawingml.Figure) string {
	if f.Text == nil {
		return ""
	}
	var paras []string
	for _, p := range f.Text.Paragraphs {
		var sb strings.Builder
		for _, r := range p.Runs {
			sb.WriteString(r.Text)
		}
		paras = append(paras, sb.String())
	}
	return strings.Join(paras, "\n")
}

// figureWith returns the figure of p whose text is text, or nil.
func figureWith(p *page, text string) *drawingml.Figure {
	for _, f := range p.figures {
		if textOf(f) == text {
			return f
		}
	}
	return nil
}

// near reports whether a and b differ by less than 0.01.
func near(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}

// pixel returns the color of the pixel at x, y.
func pixel(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

// close reports whether the colors differ by at most 8 in each channel.
func closeTo(a, b color.NRGBA) bool {
	d := func(x, y uint8) bool { return x-y <= 8 || y-x <= 8 }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B)
}

func TestOpen(t *testing.T) {
	d, err := Format.Open(t.Context(), bytes.NewReader(readFile(t, "slides.ppt")), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	doc := d.(*Document)
	info, err := doc.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Ivy test slides" || len(info.Authors) != 1 || info.Authors[0] != "Ivy Tester" {
		t.Errorf("expected the title and the author, got: %+v", info)
	}
	n, err := doc.PageCount(t.Context())
	if err != nil || n != 2 {
		t.Fatalf("expected 2 pages, got: %d, %v", n, err)
	}
	p := slidePage(t, doc, 0)
	if w, h := p.Size(); w != 793.75 || h != 446.5 {
		t.Errorf("expected a page of 793.75x446.5, got: %gx%g", w, h)
	}
	title := figureWith(p, "First slide title")
	if title == nil {
		t.Fatalf("expected the title on the first slide, got %d figures", len(p.figures))
	}
	if x, y := title.Transform.TX, title.Transform.TY; !near(float64(x), 39.625) || !near(float64(y), 17.75) {
		t.Errorf("expected the title at 39.625, 17.75, got: %g, %g", x, y)
	}
	if run := title.Text.Paragraphs[0].Runs[0].Style; run.Size.V != 44 || run.Latin.V != "Arial" {
		t.Errorf("expected a title of 44 points in Arial, got: %+v", run)
	}
	if figureWith(p, "A subtitle") == nil {
		t.Error("expected the subtitle on the first slide")
	}
	p = slidePage(t, doc, 1)
	body := figureWith(p, "First point\nSecond point\nThird point")
	if body == nil {
		t.Fatal("expected the body of three points on the second slide")
	}
	// LibreOffice writes the points without bullets
	if b := body.Text.Paragraphs[0].Style.Bullet; b.V.Kind != drawingml.BulletNone {
		t.Errorf("expected a point without a bullet, got: %+v", b)
	}
	if figureWith(p, "A free text box") == nil {
		t.Error("expected the text box on the second slide")
	}
	if _, err := doc.Page(t.Context(), 2); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ivy.ErrPageRange, got: %v", err)
	}
	if _, err := ivy.RenderImage(t.Context(), p, 36, color.White); err != nil {
		t.Fatal(err)
	}
}

func TestShapes(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(readFile(t, "shapes.ppt")))
	if err != nil {
		t.Fatal(err)
	}
	p := slidePage(t, d, 0)
	// the background, a rectangle with text, an ellipse with a gradient, a
	// triangle of a custom geometry and a picture
	if len(p.figures) != 5 {
		t.Fatalf("expected 5 figures, got: %d", len(p.figures))
	}
	if bg := p.figures[0].Fill; bg.Kind != drawingml.PaintSolid || bg.Color.NRGBA() != (color.NRGBA{0, 0x33, 0x66, 255}) {
		t.Errorf("expected a background of #003366, got: %+v", bg)
	}
	r := figureWith(p, "Red bold text")
	if r == nil {
		t.Fatal("expected the rectangle with its text")
	}
	if r.Fill.Color.NRGBA() != (color.NRGBA{0, 255, 0, 255}) || r.Line.Paint.Color.NRGBA() != (color.NRGBA{255, 0, 255, 255}) {
		t.Errorf("expected a green rectangle with a magenta outline, got: %+v, %+v", r.Fill, r.Line.Paint)
	}
	run := r.Text.Paragraphs[0].Runs[0].Style
	if run.Size.V != 28 || !run.Bold.V || run.Paint.V.Color.NRGBA() != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("expected red bold text of 28 points, got: %+v", run)
	}
	// the two colors mix in linear light, with 7 stops between them
	if g := p.figures[2].Fill; g.Kind != drawingml.PaintGradient || len(g.Gradient.Stops) != 9 || g.Gradient.Angle != 90 {
		t.Errorf("expected a vertical gradient, got: %+v", g)
	}
	if paths := p.figures[3].Paths; len(paths) != 1 || paths[0].Fill != drawingml.FillNorm {
		t.Errorf("expected the filled path of the triangle, got: %+v", paths)
	}
	if pic := p.figures[4].Fill; pic.Kind != drawingml.PaintPicture || pic.Picture == nil {
		t.Errorf("expected the picture, got: %+v", pic)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		x, y int
		want color.NRGBA
	}{
		{5, 5, color.NRGBA{0, 0x33, 0x66, 255}},
		{200, 120, color.NRGBA{0, 255, 0, 255}},
		{453, 35, color.NRGBA{255, 33, 0, 255}},
		{453, 190, color.NRGBA{255, 243, 0, 255}},
		{140, 400, color.NRGBA{255, 255, 255, 255}},
		{453, 340, color.NRGBA{0, 0, 255, 255}},
	} {
		if got := pixel(img, tt.x, tt.y); !closeTo(got, tt.want) {
			t.Errorf("expected %v at %d, %d, got: %v", tt.want, tt.x, tt.y, got)
		}
	}
	if os.Getenv("MSPPT_PNG") != "" {
		var buf bytes.Buffer
		_ = png.Encode(&buf, img)
		_ = os.WriteFile(os.Getenv("MSPPT_PNG"), buf.Bytes(), 0o644)
	}
}

func TestPowerPoint(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(readFile(t, "plain.ppt")))
	if err != nil {
		t.Fatal(err)
	}
	p := slidePage(t, d, 0)
	if w, h := p.Size(); w != 960 || h != 540 {
		t.Errorf("expected a slide of 16:9, got: %gx%g", w, h)
	}
	title := figureWith(p, "Lorem ipsum")
	if title == nil {
		t.Fatal("expected the title of the slide")
	}
	if run := title.Text.Paragraphs[0].Runs[0].Style; run.Size.V != 60 || run.Latin.V != "Calibri Light" {
		t.Errorf("expected the style of the title of the master, got: %+v", run)
	}
}

func TestPassword(t *testing.T) {
	buf := readFile(t, "rc4cryptoapi_password.ppt")
	if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword without a password, got: %v", err)
	}
	if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: "wrong"})); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword for a wrong password, got: %v", err)
	}
	cfg := ivy.Config{PasswordFunc: func(_ context.Context, attempt int) (string, error) {
		if attempt < 2 {
			return "wrong", nil
		}
		return msoPassword, nil
	}}
	d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if figureWith(slidePage(t, d, 0), "Lorem ipsum") == nil {
		t.Error("expected the title of the encrypted slide")
	}
}

// streamsOf returns the streams of the test file name.
func streamsOf(t *testing.T, name string) map[string][]byte {
	t.Helper()
	buf := readFile(t, name)
	f, err := cfb.Open(bytes.NewReader(buf), int64(len(buf)), cfb.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string][]byte)
	for _, e := range f.Root().Children {
		if out[e.Name], err = f.ReadFile(e.Name); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestThumbnail(t *testing.T) {
	s := streamsOf(t, "slides.ppt")
	// a bitmap of 2 by 1 pixels of 24 bits, in green
	dib := make([]byte, 40, 48)
	binary.LittleEndian.PutUint32(dib, 40)
	binary.LittleEndian.PutUint32(dib[4:], 2)
	binary.LittleEndian.PutUint32(dib[8:], 1)
	binary.LittleEndian.PutUint16(dib[12:], 1)
	binary.LittleEndian.PutUint16(dib[14:], 24)
	dib = append(dib, 0, 255, 0, 0, 255, 0, 0, 0)
	sum := fixture.Summary{ThumbnailTag: -1, ThumbnailFormat: fixture.ClipDIB, Thumbnail: dib}
	buf := fixture.CFB(
		fixture.CFBEntry{Name: "\x05SummaryInformation", Data: sum.Bytes()},
		fixture.CFBEntry{Name: "Current User", Data: s["Current User"]},
		fixture.CFBEntry{Name: "PowerPoint Document", Data: s["PowerPoint Document"]},
	)
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(2, 1) {
		t.Errorf("expected 2x1, got: %v", img.Bounds().Size())
	}
	if got := color.NRGBAModel.Convert(img.At(1, 0)); got != (color.NRGBA{0, 255, 0, 255}) {
		t.Errorf("expected green, got: %v", got)
	}
}

func TestOpenErrors(t *testing.T) {
	s := streamsOf(t, "slides.ppt")
	user := func(change func(b []byte)) []byte {
		cu := bytes.Clone(s["Current User"])
		change(cu)
		return fixture.CFB(
			fixture.CFBEntry{Name: "Current User", Data: cu},
			fixture.CFBEntry{Name: "PowerPoint Document", Data: s["PowerPoint Document"]},
		)
	}
	tests := []struct {
		name string
		buf  []byte
		want error
	}{
		{"not a compound file", []byte("plain text, not slides"), ivy.ErrFormat},
		{"no current user", fixture.CFB(fixture.CFBEntry{Name: "PowerPoint Document", Data: s["PowerPoint Document"]}), ivy.ErrFormat},
		{"no document", fixture.CFB(fixture.CFBEntry{Name: "Current User", Data: s["Current User"]}), ivy.ErrFormat},
		{"bad current user", user(func(b []byte) { b[2] = 0 }), ivy.ErrFormat},
		{"powerpoint 95", fixture.CFB(
			fixture.CFBEntry{Name: "Header", Data: []byte("PowerPoint 95 header")},
			fixture.CFBEntry{Name: "Current User", Data: []byte("short user")},
			fixture.CFBEntry{Name: "PowerPoint Document", Data: s["PowerPoint Document"]},
		), ivy.ErrUnsupported},
		{"powerpoint 4", fixture.CFB(fixture.CFBEntry{Name: "PP40", Data: []byte("x")}), ivy.ErrUnsupported},
		{"bad offset", user(func(b []byte) { binary.LittleEndian.PutUint32(b[16:], 3) }), ivy.ErrFormat},
		{"encrypted without a crypt session", user(func(b []byte) { binary.LittleEndian.PutUint32(b[12:], tokenEncrypted) }), ivy.ErrFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader(tt.buf)); !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got: %v", tt.want, err)
			}
		})
	}
}

func TestStyleRuns(t *testing.T) {
	le := binary.LittleEndian
	var b []byte
	// a paragraph of 6 characters at level 1, centered, with a bullet
	b = le.AppendUint32(b, 6)
	b = le.AppendUint16(b, 1)
	b = le.AppendUint32(b, pfHasBullet|pfAlign)
	b = le.AppendUint16(b, 1)
	b = le.AppendUint16(b, 1)
	// two runs of characters: bold, then red of 20 points
	b = le.AppendUint32(b, 2)
	b = le.AppendUint32(b, cfBold)
	b = le.AppendUint16(b, 1)
	b = le.AppendUint32(b, 4)
	b = le.AppendUint32(b, cfSize|cfColor)
	b = le.AppendUint16(b, 20)
	b = le.AppendUint32(b, 0xFE0000FF)
	paras, chars := readStyleRuns(b, 5)
	if len(paras) != 1 || paras[0].level != 1 || !paras[0].pf.hasBullet.v || paras[0].pf.align.v != 1 {
		t.Errorf("expected one centered paragraph with a bullet, got: %+v", paras)
	}
	if len(chars) != 2 || !chars[0].cf.bold.v || chars[1].cf.size.v != 20 || chars[1].cf.color.v != 0xFE0000FF {
		t.Errorf("expected a bold run and a red run, got: %+v", chars)
	}
	// a record that ends early gives the runs before its end
	if paras, _ := readStyleRuns(b[:8], 5); len(paras) != 0 {
		t.Errorf("expected no paragraph, got: %+v", paras)
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"slides.ppt", "plain.ppt", "rc4cryptoapi_password.ppt", "shapes.ppt"} {
		f.Add(readFile(f, name))
	}
	cfg := ivy.Config{
		Password: msoPassword,
		Limits:   ivy.Limits{MaxMemory: 4 << 20, MaxPixels: 1 << 20, MaxObjects: 10000, MaxDepth: 16},
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Open(t.Context(), bytes.NewReader(data), WithConfig(cfg))
		if err != nil {
			if strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
			return
		}
		_, _ = d.Describe(t.Context())
		for p, err := range d.Previews(t.Context()) {
			if err == nil {
				_, _ = p.Decode(t.Context())
			}
		}
		n, err := d.PageCount(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		for i := range min(n, 3) {
			p, err := d.Page(t.Context(), i)
			if err != nil {
				if strings.Contains(err.Error(), "panic") {
					t.Fatal(err)
				}
				continue
			}
			if _, err := ivy.RenderImage(t.Context(), p, 8, color.White); err != nil && strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
		}
	})
}

func TestSymbolChar(t *testing.T) {
	for _, tt := range []struct {
		u    uint16
		want rune
	}{{0x61, 'α'}, {0xF0A3, '≤'}, {0x44, 'Δ'}, {0xB7, '•'}} {
		if got, ok := symbolChar(tt.u); !ok || got != tt.want {
			t.Errorf("expected %q for %#x, got: %q", tt.want, tt.u, got)
		}
	}
	if _, ok := symbolChar(0x10); ok {
		t.Error("expected no character for 0x10")
	}
}
