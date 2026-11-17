package djvu

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xo/ivy"
)

// open opens the test file name with the configuration cfg.
func open(t testing.TB, name string, cfg ivy.Config) *Document {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(testFile(t, name)), WithConfig(cfg))
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}
	return d
}

// testFile reads the test file name.
func testFile(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// render renders page n of d at dpi over white.
func render(t *testing.T, d *Document, n int, dpi float32) image.Image {
	t.Helper()
	page, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), page, dpi, color.White)
	if err != nil {
		t.Fatalf("rendering page %d: %v", n, err)
	}
	return img
}

// gray returns the mean of the channels of the pixels of img in r.
func gray(img image.Image, r image.Rectangle) float64 {
	var sum, n float64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			sum += float64(cr+cg+cb) / 3 / 257
			n++
		}
	}
	return sum / n
}

// near reports whether the color c is within tol of want in each channel.
func near(c color.Color, want [3]int, tol int) bool {
	r, g, b, _ := c.RGBA()
	got := [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
	for i := range got {
		if got[i] < want[i]-tol || got[i] > want[i]+tol {
			return false
		}
	}
	return true
}

func TestFormat(t *testing.T) {
	t.Parallel()
	if Format.Name != "djvu" || len(Format.Exts) != 2 || len(Format.Mimes) != 2 {
		t.Errorf("expected the format djvu, got: %+v", Format)
	}
	d, err := Format.Open(t.Context(), bytes.NewReader(testFile(t, "bilevel.djvu")), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.(ivy.Pager); !ok {
		t.Error("expected a pager")
	}
	if err := d.Close(); err != nil {
		t.Error(err)
	}
}

func TestPages(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		pages int
		w, h  float32
	}{
		{"bilevel.djvu", 1, 612, 792},
		{"photo.djvu", 1, 612, 792},
		{"gray.djvu", 1, 612, 792},
		{"compound.djvu", 1, 612, 792},
		{"jpeg.djvu", 1, 612, 792},
		{"mmr.djvu", 1, 612, 792},
		{"bundled.djvu", 4, 612, 792},
		{"pdf2djvu.djvu", 4, 612, 792},
		{"shared-dict.djvu", 2, 612, 792},
		{"thumbnails.djvu", 2, 612, 792},
	} {
		d := open(t, c.name, ivy.Config{})
		n, err := d.PageCount(t.Context())
		if err != nil || n != c.pages {
			t.Errorf("%s: expected %d pages, got: %d, %v", c.name, c.pages, n, err)
		}
		for i := range n {
			page, err := d.Page(t.Context(), i)
			if err != nil {
				t.Fatal(err)
			}
			if w, h := page.Size(); w != c.w || h != c.h {
				t.Errorf("%s: page %d: expected %gx%g, got: %gx%g", c.name, i, c.w, c.h, w, h)
			}
		}
		for _, n := range []int{-1, n} {
			if _, err := d.Page(t.Context(), n); !errors.Is(err, ivy.ErrPageRange) {
				t.Errorf("%s: page %d: expected ErrPageRange, got: %v", c.name, n, err)
			}
		}
	}
}

// TestBilevel renders a page of text at the resolution of the file and
// small. The title is black, the margin white, and the small page keeps
// the ink of the large one, because the mask is reduced by its area.
func TestBilevel(t *testing.T) {
	t.Parallel()
	d := open(t, "bilevel.djvu", ivy.Config{})
	full := render(t, d, 0, 300)
	if got := full.Bounds().Size(); got != image.Pt(2550, 3300) {
		t.Fatalf("expected 2550x3300 pixels, got: %v", got)
	}
	if g := gray(full, image.Rect(0, 0, 200, 150)); g != 255 {
		t.Errorf("expected a white margin, got: %.1f", g)
	}
	if g := gray(full, image.Rect(240, 180, 960, 300)); g > 230 {
		t.Errorf("expected the black title, got a mean of %.1f", g)
	}
	ink := 255 - gray(full, full.Bounds())
	for _, dpi := range []float32{100, 37.5, 12} {
		small := render(t, d, 0, dpi)
		got := 255 - gray(small, small.Bounds())
		t.Logf("%g dpi: ink %.2f, at 300 dpi %.2f", dpi, got, ink)
		if got < ink*0.85 || got > ink*1.15 {
			t.Errorf("%g dpi: expected the ink of the full page, %.2f, got: %.2f", dpi, ink, got)
		}
	}
}

// TestLayers checks the colors of the background, of the foreground and of
// a palette, against the pages of ddjvu at 100 dots per inch.
func TestLayers(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		at   image.Point
		want [3]int
	}{
		{"photo.djvu", image.Pt(100, 100), [3]int{156, 179, 200}},
		{"photo.djvu", image.Pt(400, 500), [3]int{229, 102, 77}},
		{"compound.djvu", image.Pt(600, 900), [3]int{52, 51, 128}},
		{"jpeg.djvu", image.Pt(400, 500), [3]int{229, 102, 77}},
		{"gray.djvu", image.Pt(400, 500), [3]int{143, 143, 143}},
	} {
		img := render(t, open(t, c.name, ivy.Config{}), 0, 100)
		if got := img.At(c.at.X, c.at.Y); !near(got, c.want, 8) {
			t.Errorf("%s at %v: expected %v, got: %v", c.name, c.at, c.want, got)
		}
	}
	// the page of a palette draws its glyphs in the colors of the palette,
	// one of which is white
	img := render(t, open(t, "palette.djvu", ivy.Config{}), 0, 300)
	colors := map[color.RGBA]bool{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 3 {
		for x := b.Min.X; x < b.Max.X; x += 3 {
			r, g, bl, _ := img.At(x, y).RGBA()
			if c := (color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 0xff}); c.R < 200 || c.G < 200 || c.B < 200 {
				colors[c] = true
			}
		}
	}
	if len(colors) != 3 {
		t.Errorf("expected 3 dark colors of the palette, got: %v", colors)
	}
}

// TestMasks renders the same mask from JB2 and from G4, and a mask of G4
// in stripes.
func TestMasks(t *testing.T) {
	t.Parallel()
	want := render(t, open(t, "bilevel.djvu", ivy.Config{}), 0, 50)
	got := render(t, open(t, "mmr.djvu", ivy.Config{}), 0, 50)
	if g, w := gray(got, got.Bounds()), gray(want, want.Bounds()); g != w {
		t.Errorf("expected the page of bilevel.djvu, %.3f, got: %.3f", w, g)
	}
	striped := render(t, open(t, "mmr-striped.djvu", ivy.Config{}), 0, 50)
	if g := gray(striped, image.Rect(40, 30, 160, 50)); g > 240 {
		t.Errorf("expected the title, got a mean of %.1f", g)
	}
}

// TestSharedDictionary renders the pages of a bundled document whose pages
// share a dictionary of shapes, and that dictionary decodes once.
func TestSharedDictionary(t *testing.T) {
	t.Parallel()
	d := open(t, "shared-dict.djvu", ivy.Config{})
	for i := range 2 {
		img := render(t, d, i, 50)
		if g := gray(img, img.Bounds()); g > 250 || g < 200 {
			t.Errorf("page %d: expected text, got a mean of %.1f", i, g)
		}
	}
}

// TestRotation turns a page by the flags of its INFO chunk.
func TestRotation(t *testing.T) {
	t.Parallel()
	data := testFile(t, "compound.djvu")
	plain := render(t, open(t, "compound.djvu", ivy.Config{}), 0, 50)
	pb := plain.Bounds()
	for _, c := range []struct {
		flag byte
		at   func(x, y int) image.Point
	}{
		{5, func(x, y int) image.Point { return image.Pt(pb.Dy()-1-y, x) }},
		{2, func(x, y int) image.Point { return image.Pt(pb.Dx()-1-x, pb.Dy()-1-y) }},
		{6, func(x, y int) image.Point { return image.Pt(y, pb.Dx()-1-x) }},
	} {
		patched := patchInfo(t, data, 9, c.flag)
		d, err := Open(t.Context(), bytes.NewReader(patched))
		if err != nil {
			t.Fatal(err)
		}
		page, _ := d.Page(t.Context(), 0)
		w, h := page.Size()
		if c.flag != 2 && (w != 792 || h != 612) {
			t.Errorf("flag %d: expected 792x612, got: %gx%g", c.flag, w, h)
		}
		img := render(t, d, 0, 50)
		for _, p := range []image.Point{{100, 150}, {200, 300}, {300, 450}} {
			q := c.at(p.X, p.Y)
			r, g, b, _ := plain.At(p.X, p.Y).RGBA()
			if !near(img.At(q.X, q.Y), [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}, 24) {
				t.Errorf("flag %d: expected the pixel %v at %v, got: %v", c.flag, plain.At(p.X, p.Y), q, img.At(q.X, q.Y))
			}
		}
	}
}

// TestGamma lightens a page whose INFO chunk gives a gamma below 2.2.
func TestGamma(t *testing.T) {
	t.Parallel()
	d, err := Open(t.Context(), bytes.NewReader(patchInfo(t, testFile(t, "photo.djvu"), 8, 14)))
	if err != nil {
		t.Fatal(err)
	}
	// ddjvu gives (187, 204, 218), where gamma 2.2 gives (156, 179, 200)
	if got := render(t, d, 0, 100).At(100, 100); !near(got, [3]int{187, 204, 218}, 8) {
		t.Errorf("expected a lighter color, got: %v", got)
	}
}

// patchInfo returns a copy of the single page data, with the byte at off
// in its INFO chunk set to v.
func patchInfo(t *testing.T, data []byte, off int, v byte) []byte {
	t.Helper()
	i := bytes.Index(data, []byte("INFO"))
	if i < 0 {
		t.Fatal("expected an INFO chunk")
	}
	out := bytes.Clone(data)
	out[i+8+off] = v
	return out
}

func TestPreviews(t *testing.T) {
	t.Parallel()
	d := open(t, "thumbnails.djvu", ivy.Config{})
	var previews []ivy.Preview
	for p, err := range d.Previews(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		previews = append(previews, p)
	}
	if len(previews) != 2 {
		t.Fatalf("expected 2 thumbnails, got: %d", len(previews))
	}
	for _, p := range previews {
		img, err := p.Decode(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if p.Kind != ivy.Thumbnail || img.Bounds().Size() != p.Size || p.Size.X != 64 {
			t.Errorf("expected a thumbnail 64 pixels wide, got: %v, %v", p.Size, img.Bounds())
		}
		if g := gray(img, img.Bounds()); g > 252 || g < 150 {
			t.Errorf("expected a page of text, got a mean of %.1f", g)
		}
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Pt(48, 64)); err != nil {
		t.Error(err)
	}
	if _, err := ivy.BestPreview(t.Context(), open(t, "bilevel.djvu", ivy.Config{}), image.Pt(48, 64)); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
}

func TestDescribe(t *testing.T) {
	t.Parallel()
	info, err := open(t, "bundled.djvu", ivy.Config{}).Describe(t.Context())
	if err != nil || info.Title != "dejapdf test corpus" {
		t.Errorf("expected the title of bundled.djvu, got: %+v, %v", info, err)
	}
	info, err = open(t, "pdf2djvu.djvu", ivy.Config{}).Describe(t.Context())
	created := time.Date(2026, 11, 13, 19, 38, 50, 0, time.FixedZone("", -8*3600))
	if err != nil || info.Title != "dejapdf test corpus source" || info.Producer != "GPL Ghostscript 10.08.0" ||
		!info.Created.Equal(created) || !info.Modified.Equal(created) {
		t.Errorf("expected the metadata of pdf2djvu.djvu, got: %+v, %v", info, err)
	}
	info, err = open(t, "bilevel.djvu", ivy.Config{}).Describe(t.Context())
	if err != nil || info.Title != "" {
		t.Errorf("expected no metadata, got: %+v, %v", info, err)
	}
	// a METa chunk of a single page
	data := form("DJVU", chunk("INFO", info2550()), chunk("METa", []byte(`(metadata (Author "A. Writer") (title "Meta"))`)))
	d, err := Open(t.Context(), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	info, err = d.Describe(t.Context())
	if err != nil || info.Title != "Meta" || len(info.Authors) != 1 || info.Authors[0] != "A. Writer" {
		t.Errorf("expected the metadata of METa, got: %+v, %v", info, err)
	}
}

// chunk returns an IFF chunk.
func chunk(id string, data []byte) []byte {
	b := []byte(id)
	b = binary.BigEndian.AppendUint32(b, uint32(len(data)))
	b = append(b, data...)
	if len(data)%2 == 1 {
		b = append(b, 0)
	}
	return b
}

// form returns a DjVu file of one FORM chunk of the type typ.
func form(typ string, kids ...[]byte) []byte {
	d := []byte(typ)
	for _, k := range kids {
		d = append(d, k...)
	}
	return append([]byte("AT&T"), chunk("FORM", d)...)
}

// info2550 returns an INFO chunk for a page of 2550x3300 pixels at 300 dots
// per inch.
func info2550() []byte {
	return []byte{0x09, 0xf6, 0x0c, 0xe4, 24, 0, 0x2c, 0x01, 22, 1}
}

func TestErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		data []byte
		want error
	}{
		{"empty", nil, ivy.ErrFormat},
		{"text", []byte("hello, this is not a djvu file"), ivy.ErrFormat},
		{"other form", form("ABCD", chunk("INFO", info2550())), ivy.ErrFormat},
		{"iw44 photo without a size", form("PM44", chunk("PM44", []byte{0, 1})), ivy.ErrFormat},
		{"iw44 photo without data", form("PM44", chunk("INFO", info2550())), ivy.ErrFormat},
		{"shared data", form("DJVI", chunk("Djbz", []byte{1})), ivy.ErrUnsupported},
		{"indirect", testFile(t, "indirect/index.djvu"), ivy.ErrUnsupported},
		{"no info", form("DJVU", chunk("Sjbz", []byte{1, 2, 3})), ivy.ErrFormat},
	} {
		if _, err := Open(t.Context(), bytes.NewReader(c.data)); !errors.Is(err, c.want) {
			t.Errorf("%s: expected %v, got: %v", c.name, c.want, err)
		}
	}
	// a page with a damaged mask draws its background, in part
	data := testFile(t, "compound.djvu")
	i := bytes.Index(data, []byte("Sjbz"))
	damaged := bytes.Clone(data)
	for k := i + 8; k < i+400; k++ {
		damaged[k] ^= 0x5a
	}
	d, err := Open(t.Context(), bytes.NewReader(damaged))
	if err != nil {
		t.Fatal(err)
	}
	page, _ := d.Page(t.Context(), 0)
	img, err := ivy.RenderImage(t.Context(), page, 50, color.White)
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok || !errors.Is(err, ivy.ErrFormat) {
		t.Fatalf("expected a partial error, got: %v", err)
	}
	if !near(img.At(50, 50), [3]int{156, 179, 200}, 24) {
		t.Errorf("expected the background, got: %v", img.At(50, 50))
	}
	// each prefix of a file opens or fails, and renders or fails, without a
	// panic
	for _, name := range []string{"bundled.djvu", "jpeg.djvu", "mmr-striped.djvu"} {
		data := testFile(t, name)
		for n := 0; n < len(data); n += len(data)/13 + 1 {
			d, err := Open(t.Context(), bytes.NewReader(data[:n]))
			if err != nil {
				continue
			}
			count, _ := d.PageCount(t.Context())
			for i := range count {
				page, _ := d.Page(t.Context(), i)
				_, _ = ivy.RenderImage(t.Context(), page, 20, color.White)
			}
		}
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		lim  ivy.Limits
	}{
		{"bundled.djvu", ivy.Limits{MaxObjects: 3}},
	} {
		_, err := Open(t.Context(), bytes.NewReader(testFile(t, c.name)), WithConfig(ivy.Config{Limits: c.lim}))
		if !errors.Is(err, ivy.ErrLimit) {
			t.Errorf("%s with %+v: expected ErrLimit, got: %v", c.name, c.lim, err)
		}
	}
	for _, c := range []struct {
		name string
		lim  ivy.Limits
	}{
		{"bilevel.djvu", ivy.Limits{MaxPixels: 1 << 20}},
		{"bilevel.djvu", ivy.Limits{MaxObjects: 100}},
		{"photo.djvu", ivy.Limits{MaxMemory: 1 << 20}},
		{"mmr.djvu", ivy.Limits{MaxPixels: 1 << 20}},
		{"jpeg.djvu", ivy.Limits{MaxPixels: 1 << 20}},
		// a bundled document reads the chunks of a page when it renders
		{"bundled.djvu", ivy.Limits{MaxDepth: 1}},
	} {
		d := open(t, c.name, ivy.Config{Limits: c.lim})
		page, _ := d.Page(t.Context(), 0)
		if _, err := ivy.RenderImage(t.Context(), page, 20, color.White); !errors.Is(err, ivy.ErrLimit) {
			t.Errorf("%s with %+v: expected ErrLimit, got: %v", c.name, c.lim, err)
		}
	}
	// a small limit of the memory draws the page at a coarser resolution
	d := open(t, "bilevel.djvu", ivy.Config{Limits: ivy.Limits{MaxMemory: 32 << 20}})
	page, _ := d.Page(t.Context(), 0)
	if _, err := ivy.RenderImage(t.Context(), page, 300, color.White); err != nil {
		t.Errorf("expected a coarse page, got: %v", err)
	}
	if l := d.pages[0].layers.Load(); l == nil || 8*int64(len(l.comp.Pix)/4) > 32<<20 {
		t.Errorf("expected a composite within the limit, got: %+v", l)
	}
}

func TestCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(ctx, bytes.NewReader(testFile(t, "bilevel.djvu"))); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	d := open(t, "compound.djvu", ivy.Config{})
	page, _ := d.Page(t.Context(), 0)
	if _, err := ivy.RenderImage(ctx, page, 50, color.White); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	// a cancelled render keeps nothing, and the next one draws the page
	img := render(t, d, 0, 50)
	if !near(img.At(50, 50), [3]int{156, 179, 200}, 24) {
		t.Errorf("expected the background, got: %v", img.At(50, 50))
	}
	if _, err := d.Describe(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

// TestConcurrent renders the pages of a document from many goroutines,
// at several resolutions, while the cache forgets pages.
func TestConcurrent(t *testing.T) {
	t.Parallel()
	d := open(t, "bundled.djvu", ivy.Config{})
	want := make([]float64, 4)
	for i := range want {
		img := render(t, d, i, 30)
		want[i] = gray(img, img.Bounds())
	}
	var wg sync.WaitGroup
	for k := range 16 {
		wg.Go(func() {
			n := k % 4
			page, err := d.Page(t.Context(), n)
			if err != nil {
				t.Error(err)
				return
			}
			dpi := []float32{30, 45}[k/4%2]
			img, err := ivy.RenderImage(t.Context(), page, dpi, color.White)
			if err != nil {
				t.Error(err)
				return
			}
			if g := gray(img, img.Bounds()); dpi == 30 && g != want[n] {
				t.Errorf("page %d: expected a mean of %.2f, got: %.2f", n, want[n], g)
			}
		})
	}
	wg.Wait()
	if len(d.recent) > maxCached {
		t.Errorf("expected at most %d pages in the cache, got: %d", maxCached, len(d.recent))
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"bilevel.djvu", "palette.djvu", "shared-dict.djvu", "thumbnails.djvu", "gray.djvu"} {
		f.Add(testFile(f, name))
	}
	f.Add(form("DJVU", chunk("INFO", info2550()), chunk("METa", []byte(`(metadata (title "x"))`))))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 64 << 20, MaxPixels: 1 << 24, MaxObjects: 1 << 16, MaxDepth: 16}}
	// a recovered panic is still a failure, so that the cause is fixed
	check := func(t *testing.T, err error) {
		if err != nil && strings.Contains(err.Error(), "panic") {
			t.Fatal(err)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Open(t.Context(), bytes.NewReader(data), WithConfig(cfg))
		check(t, err)
		if err != nil {
			return
		}
		_, err = d.Describe(t.Context())
		check(t, err)
		for p := range d.Previews(t.Context()) {
			_, err = p.Decode(t.Context())
			check(t, err)
		}
		page, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		_, err = ivy.RenderImage(t.Context(), page, 10, color.White)
		check(t, err)
	})
}
