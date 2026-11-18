package pdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/okage"
)

// build returns a PDF file of the objects, numbered from 1, with a valid
// table of cross references. Object 1 is the catalog.
func build(objects ...string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, len(objects), xref)
	return b.Bytes()
}

// stream returns a stream object with the content.
func stream(dict, content string) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dict, len(content), content)
}

// onePage returns a PDF file of one page of 100 by 100 points, with the
// content, the entries of the page dict and the info dict.
func onePage(content, page, info string) []byte {
	return build(
		"<< /Type /Catalog /Pages 2 0 R /Lang (de-AT) >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Contents 4 0 R "+page+" >>",
		stream("", content),
		"<< "+info+" >>",
	)
}

func open(t *testing.T, buf []byte, opts ...Option) *Document {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf), opts...)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func page(t *testing.T, d *Document, n int) ivy.Page {
	t.Helper()
	p, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// rgba returns the color of img at (x, y), not premultiplied.
func rgba(img image.Image, x, y int) color.NRGBA {
	c, _ := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	return c
}

// testFiles returns the test files of the ports of hayro.
func testFiles(t testing.TB) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"../internal/hayro/interpret/testdata", "../internal/hayro/syntax/testdata"} {
		names, err := filepath.Glob(filepath.Join(dir, "*.pdf"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, names...)
	}
	if len(files) == 0 {
		t.Fatal("expected test files")
	}
	return files
}

func TestFormat(t *testing.T) {
	if Format.Name != "pdf" || Format.Exts[0] != "pdf" || Format.Mimes[0] != "application/pdf" {
		t.Errorf("unexpected format: %+v", Format)
	}
	d, err := Format.Open(t.Context(), bytes.NewReader(onePage("", "", "")), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.(ivy.Pager); !ok {
		t.Error("expected a pager")
	}
}

func TestRenderTestFiles(t *testing.T) {
	for _, name := range testFiles(t) {
		t.Run(filepath.Base(name), func(t *testing.T) {
			t.Parallel()
			buf, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			d := open(t, buf)
			n, err := d.PageCount(t.Context())
			if err != nil || n == 0 {
				t.Fatalf("expected pages, got %d, %v", n, err)
			}
			for i := range min(n, 3) {
				img, err := ivy.RenderImage(t.Context(), page(t, d, i), 36, color.White)
				if err != nil {
					t.Errorf("page %d: %v", i+1, err)
				}
				if img == nil {
					t.Errorf("page %d: expected an image", i+1)
				}
			}
		})
	}
}

func TestInk(t *testing.T) {
	// a blue square from (10, 10) to (40, 40), from the bottom left corner
	d := open(t, onePage("0 0 1 rg 10 10 30 30 re f", "", ""))
	p := page(t, d, 0)
	if w, h := p.Size(); w != 100 || h != 100 {
		t.Errorf("expected a page of 100 by 100 points, got %g by %g", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if c := rgba(img, 25, 75); c != (color.NRGBA{0, 0, 255, 255}) {
		t.Errorf("expected blue at (25, 75), got: %v", c)
	}
	for _, pt := range []image.Point{{25, 25}, {75, 75}, {5, 95}} {
		if c := rgba(img, pt.X, pt.Y); c != (color.NRGBA{255, 255, 255, 255}) {
			t.Errorf("expected white at %v, got: %v", pt, c)
		}
	}
}

func TestRotateAndCropBox(t *testing.T) {
	// the crop box keeps the left half, and the page turns by 90 degrees
	d := open(t, onePage("1 0 0 rg 0 0 50 100 re f", "/CropBox [0 0 50 100] /Rotate 90", ""))
	p := page(t, d, 0)
	if w, h := p.Size(); w != 100 || h != 50 {
		t.Fatalf("expected a page of 100 by 50 points, got %g by %g", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if c := rgba(img, 50, 25); c != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("expected red, got: %v", c)
	}
}

// TestPageBox checks that WithPageBox renders the chosen page box, and that
// a page box that the page does not have takes its default (D31).
func TestPageBox(t *testing.T) {
	t.Parallel()
	// red on the left half, blue on the right half, and a crop box and a
	// trim box of the right half
	buf := onePage("1 0 0 rg 0 0 50 100 re f 0 0 1 rg 50 0 50 100 re f",
		"/CropBox [50 0 100 100] /TrimBox [50 50 100 100] /BleedBox [-10 0 100 100]", "")
	red, blue := color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 255}
	for _, c := range []struct {
		name string
		opts []Option
		w, h float32
		// the colors at the left and at the right of the middle row
		left, right color.NRGBA
	}{
		{"default", nil, 50, 100, blue, blue},
		{"CropBox", []Option{WithPageBox(CropBox)}, 50, 100, blue, blue},
		{"MediaBox", []Option{WithPageBox(MediaBox)}, 100, 100, red, blue},
		{"TrimBox", []Option{WithPageBox(TrimBox)}, 50, 50, blue, blue},
		// the bleed box is cut to the media box
		{"BleedBox", []Option{WithPageBox(BleedBox)}, 100, 100, red, blue},
		// the page has no art box, which defaults to the crop box
		{"ArtBox", []Option{WithPageBox(ArtBox)}, 50, 100, blue, blue},
	} {
		p := page(t, open(t, buf, c.opts...), 0)
		if w, h := p.Size(); w != c.w || h != c.h {
			t.Errorf("%s: expected a page of %g by %g points, got %g by %g", c.name, c.w, c.h, w, h)
			continue
		}
		img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
		if err != nil {
			t.Fatal(err)
		}
		b := img.Bounds()
		y := b.Dy() / 2
		if got := rgba(img, 2, y); got != c.left {
			t.Errorf("%s: expected %v at the left, got %v", c.name, c.left, got)
		}
		if got := rgba(img, b.Dx()-3, y); got != c.right {
			t.Errorf("%s: expected %v at the right, got %v", c.name, c.right, got)
		}
	}
	// the rotation applies to the trim box
	p := page(t, open(t, onePage("", "/TrimBox [0 0 50 100] /Rotate 90", ""), WithPageBox(TrimBox)), 0)
	if w, h := p.Size(); w != 100 || h != 50 {
		t.Errorf("rotated: expected a page of 100 by 50 points, got %g by %g", w, h)
	}
}

func TestCancel(t *testing.T) {
	var content strings.Builder
	for range 200000 {
		content.WriteString("0 0 1 rg 10 10 30 30 re f\n")
	}
	d := open(t, onePage(content.String(), "", ""))
	p := page(t, d, 0)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	dst, _ := okage.NewPixmap(100, 100)
	if err := p.Render(ctx, dst, okage.IdentityTransform()); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	ctx, cancel = context.WithTimeout(t.Context(), 5*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := p.Render(ctx, dst, okage.IdentityTransform())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Errorf("expected the render to stop soon, it took %v", el)
	}
}

func TestLimit(t *testing.T) {
	var content strings.Builder
	for range 1000 {
		content.WriteString("0 0 1 rg 10 10 30 30 re f\n")
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxObjects: 1}}
	d := open(t, onePage(content.String(), "", ""), WithConfig(cfg))
	dst, _ := okage.NewPixmap(100, 100)
	err := page(t, d, 0).Render(t.Context(), dst, okage.IdentityTransform())
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok || !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected a partial error of ivy.ErrLimit, got: %v", err)
	}
}

// largeSource is a source of size bytes that records each read.
type largeSource struct {
	size  int64
	reads int
}

func (s *largeSource) ReadAt([]byte, int64) (int, error) {
	s.reads++
	return 0, io.ErrUnexpectedEOF
}

func (s *largeSource) Size() int64 {
	return s.size
}

func TestMaxMemory(t *testing.T) {
	buf := onePage("0 0 1 rg 10 10 30 30 re f", "", "")
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: int64(len(buf))}}
	open(t, buf, WithConfig(cfg))
	cfg.Limits.MaxMemory--
	if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit for a file 1 byte over the limit, got: %v", err)
	}
	// The default limit refuses a file of 1 GiB before it reads a byte.
	src := &largeSource{size: 1 << 30}
	if _, err := Open(t.Context(), src); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit for a file of 1 GiB, got: %v", err)
	}
	if src.reads != 0 {
		t.Errorf("expected no read of a file over the limit, got %d", src.reads)
	}
}

func TestPassword(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "password_2.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	const password = "samplefiles"
	if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword without a password, got: %v", err)
	}
	if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: "wrong"})); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword for a wrong password, got: %v", err)
	}
	d := open(t, buf, WithConfig(ivy.Config{Password: password}))
	img, err := ivy.RenderImage(t.Context(), page(t, d, 0), 36, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if ink(img) == 0 {
		t.Error("expected ink on the page")
	}
	var attempts []int
	cfg := ivy.Config{PasswordFunc: func(_ context.Context, attempt int) (string, error) {
		attempts = append(attempts, attempt)
		if attempt == 1 {
			return "wrong", nil
		}
		return password, nil
	}}
	open(t, buf, WithConfig(cfg))
	if len(attempts) != 2 || attempts[1] != 2 {
		t.Errorf("expected attempts 1 and 2, got: %v", attempts)
	}
	errStop := errors.New("no more")
	cfg.PasswordFunc = func(context.Context, int) (string, error) { return "", errStop }
	if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg)); !errors.Is(err, ivy.ErrPassword) || !errors.Is(err, errStop) {
		t.Errorf("expected ivy.ErrPassword and the error of the function, got: %v", err)
	}
}

// ink counts the pixels of img that are not white.
func ink(img image.Image) int {
	var n int
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c := rgba(img, x, y); c.R < 200 || c.G < 200 || c.B < 200 {
				n++
			}
		}
	}
	return n
}

func TestNotPDF(t *testing.T) {
	for _, buf := range [][]byte{nil, []byte("not a pdf file")} {
		if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("expected ivy.ErrFormat for %q, got: %v", buf, err)
		}
	}
}

func TestPageRange(t *testing.T) {
	d := open(t, onePage("", "", ""))
	for _, n := range []int{-1, 1} {
		if _, err := d.Page(t.Context(), n); !errors.Is(err, ivy.ErrPageRange) {
			t.Errorf("page %d: expected ivy.ErrPageRange, got: %v", n, err)
		}
	}
}

func TestDescribe(t *testing.T) {
	info := `/Title (Caf\351 \200) /Author <FEFF00410042> /Subject (Tests) /Producer (ivy)
		/CreationDate (D:20240102030405+01'30') /ModDate (D:2025)`
	d := open(t, onePage("", "", info))
	got, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := ivy.Info{
		Title:    "Café •",
		Authors:  []string{"AB"},
		Subject:  "Tests",
		Language: "de-AT",
		Producer: "ivy",
		Created:  time.Date(2024, 1, 2, 3, 4, 5, 0, time.FixedZone("", 5400)),
		Modified: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if got.Title != want.Title || len(got.Authors) != 1 || got.Authors[0] != want.Authors[0] ||
		got.Subject != want.Subject || got.Language != want.Language || got.Producer != want.Producer ||
		!got.Created.Equal(want.Created) || !got.Modified.Equal(want.Modified) {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

func TestPreviews(t *testing.T) {
	thumb := stream("/Width 2 /Height 1 /ColorSpace /DeviceRGB /BitsPerComponent 8", "\xff\x00\x00\x00\x00\xff")
	buf := build(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Thumb 4 0 R >>",
		thumb,
		"<< >>",
	)
	d := open(t, buf)
	var previews []ivy.Preview
	for p, err := range d.Previews(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		previews = append(previews, p)
	}
	if len(previews) != 1 || previews[0].Kind != ivy.Thumbnail || previews[0].Size != image.Pt(2, 1) {
		t.Fatalf("expected one thumbnail of 2 by 1 pixels, got: %+v", previews)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if c := rgba(img, 0, 0); c != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("expected red, got: %v", c)
	}
	if c := rgba(img, 1, 0); c != (color.NRGBA{0, 0, 255, 255}) {
		t.Errorf("expected blue, got: %v", c)
	}
	if _, err := ivy.BestPreview(t.Context(), open(t, onePage("", "", "")), image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ivy.ErrNoPreview, got: %v", err)
	}
}

func TestAnnotations(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("..", "internal", "hayro", "interpret", "testdata", "annotation_checkbox.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	with, err := ivy.RenderImage(t.Context(), page(t, open(t, buf), 0), 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	without, err := ivy.RenderImage(t.Context(), page(t, open(t, buf, WithAnnotations(false)), 0), 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := ink(with), ink(without); a <= b {
		t.Errorf("expected more ink with the annotations, got %d and %d", a, b)
	}
}

func TestConcurrentRender(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("..", "internal", "hayro", "interpret", "testdata", "pattern_shading_on_text.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d := open(t, buf)
	p := page(t, d, 0)
	var wg sync.WaitGroup
	inks := make([]int, 4)
	for i := range inks {
		wg.Go(func() {
			img, err := ivy.RenderImage(t.Context(), p, 36, color.White)
			if err != nil {
				t.Error(err)
				return
			}
			inks[i] = ink(img)
		})
	}
	wg.Wait()
	for _, n := range inks[1:] {
		if n != inks[0] {
			t.Errorf("expected the same ink in each render, got: %v", inks)
		}
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range testFiles(f) {
		buf, err := os.ReadFile(name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(buf)
	}
	f.Add(onePage("0 0 1 rg 10 10 30 30 re f", "", "/Title (x)"))
	f.Add(cjkPage)
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 4 << 20, MaxPixels: 1 << 20, MaxObjects: 1000, MaxDepth: 16}}
	// The fonts of the module github.com/xo/ivy/fonts resolve the fonts
	// that a file does not embed, when the folder is there.
	if _, err := os.Stat(fontsDir); err == nil {
		cfg.Fonts = os.DirFS(fontsDir)
	}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			if strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
			return
		}
		_, _ = d.Describe(t.Context())
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		w, h := p.Size()
		dst, _ := okage.NewPixmap(32, 32)
		scale := 32 / max(w, h, 1)
		err = p.Render(t.Context(), dst, okage.TransformFromScale(scale, scale))
		if _, ok := errors.AsType[*interpret.PanicError](err); ok || (err != nil && strings.Contains(err.Error(), "panic")) {
			t.Fatal(err)
		}
	})
}
