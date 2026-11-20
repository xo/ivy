package icc

import (
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/color/palette"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/xo/ivy"
)

// testdata is the folder of the profiles of the port of moxcms.
var testdata = filepath.Join("..", "internal", "moxcms", "testdata")

// readProfile returns the profile name of testdata.
func readProfile(tb testing.TB, name string) []byte {
	tb.Helper()
	b, err := os.ReadFile(filepath.Join(testdata, name))
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

// tag is a tag of buildProfile.
type tag struct {
	sig  string
	data []byte
}

// buildProfile returns a display profile of version 2.1 in the color space
// space, with the PCS XYZ and the tags.
func buildProfile(space string, tags ...tag) []byte {
	b := make([]byte, 128+4+12*len(tags))
	binary.BigEndian.PutUint32(b[8:], 0x02100000)
	copy(b[12:], "mntr")
	copy(b[16:], space)
	copy(b[20:], "XYZ ")
	copy(b[36:], "acsp")
	// The illuminant D50 of the PCS.
	binary.BigEndian.PutUint32(b[68:], 0x0000f6d6)
	binary.BigEndian.PutUint32(b[72:], 0x00010000)
	binary.BigEndian.PutUint32(b[76:], 0x0000d32d)
	binary.BigEndian.PutUint32(b[128:], uint32(len(tags)))
	for i, t := range tags {
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
		e := 132 + 12*i
		copy(b[e:], t.sig)
		binary.BigEndian.PutUint32(b[e+4:], uint32(len(b)))
		binary.BigEndian.PutUint32(b[e+8:], uint32(len(t.data)))
		b = append(b, t.data...)
	}
	binary.BigEndian.PutUint32(b, uint32(len(b)))
	return b
}

// s15 returns v as an s15Fixed16Number.
func s15(v float64) uint32 {
	return uint32(int32(v * 65536))
}

// xyz returns an XYZType tag.
func xyz(x, y, z float64) []byte {
	b := make([]byte, 20)
	copy(b, "XYZ ")
	binary.BigEndian.PutUint32(b[8:], s15(x))
	binary.BigEndian.PutUint32(b[12:], s15(y))
	binary.BigEndian.PutUint32(b[16:], s15(z))
	return b
}

// para returns a parametricCurveType tag of the function type with the
// parameters.
func para(function uint16, params ...float64) []byte {
	b := make([]byte, 12+4*len(params))
	copy(b, "para")
	binary.BigEndian.PutUint16(b[8:], function)
	for i, p := range params {
		binary.BigEndian.PutUint32(b[12+4*i:], s15(p))
	}
	return b
}

// gamma returns a curveType tag of a pure gamma.
func gamma(g float64) []byte {
	b := make([]byte, 14)
	copy(b, "curv")
	binary.BigEndian.PutUint32(b[8:], 1)
	binary.BigEndian.PutUint16(b[12:], uint16(g*256))
	return b
}

// srgbProfile returns a matrix profile of sRGB, with the colorants of sRGB
// adapted to D50.
func srgbProfile() []byte {
	trc := para(3, 2.4, 1/1.055, 0.055/1.055, 1/12.92, 0.04045)
	return buildProfile("RGB ",
		tag{"wtpt", xyz(0.9642, 1, 0.8249)},
		tag{"rXYZ", xyz(0.4361, 0.2225, 0.0139)},
		tag{"gXYZ", xyz(0.3851, 0.7169, 0.0971)},
		tag{"bXYZ", xyz(0.1431, 0.0606, 0.7141)},
		tag{"rTRC", trc},
		tag{"gTRC", trc},
		tag{"bTRC", trc},
	)
}

// linearGrayProfile returns a gray profile with a linear curve.
func linearGrayProfile() []byte {
	return buildProfile("GRAY",
		tag{"wtpt", xyz(0.9642, 1, 0.8249)},
		tag{"kTRC", gamma(1)},
	)
}

// near reports whether a and b differ by d or less.
func near(a, b, d int) bool {
	return a-b <= d && b-a <= d
}

// nrgba64At returns the color of img at (x, y) without premultiplication.
func nrgba64At(img image.Image, x, y int) color.NRGBA64 {
	c, _ := color.NRGBA64Model.Convert(img.At(x, y)).(color.NRGBA64)
	return c
}

// sameColors fails the test when the colors of got and want differ by more
// than d levels of 8 bits, or when their alphas differ.
func sameColors(t *testing.T, got, want image.Image, d int) {
	t.Helper()
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds %v, want %v", got.Bounds(), want.Bounds())
	}
	b := want.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g, w := nrgba64At(got, x, y), nrgba64At(want, x, y)
			if g.A != w.A {
				t.Fatalf("alpha at (%d, %d) is %d, want %d", x, y, g.A, w.A)
			}
			if w.A == 0 {
				continue
			}
			if !near(int(g.R>>8), int(w.R>>8), d) || !near(int(g.G>>8), int(w.G>>8), d) || !near(int(g.B>>8), int(w.B>>8), d) {
				t.Fatalf("color at (%d, %d) is %v, want %v", x, y, g, w)
			}
		}
	}
}

// fill sets each pixel of img to a color that depends on its position, with
// an alpha that is not opaque in one column.
func fill(img interface {
	image.Image
	Set(x, y int, c color.Color)
}) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			a := uint16(0xffff)
			if x == b.Min.X+1 {
				a = 0x8000
			}
			img.Set(x, y, color.NRGBA64{
				R: uint16(x * 9000), G: uint16(y * 11000), B: uint16((x + y) * 5000), A: a,
			})
		}
	}
}

// inputs returns an image of each type that ToSRGB reads.
func inputs() map[string]image.Image {
	r := image.Rect(-2, 3, 5, 8)
	gray := image.NewGray(r)
	gray16 := image.NewGray16(r)
	rgba := image.NewRGBA(r)
	nrgba := image.NewNRGBA(r)
	rgba64 := image.NewRGBA64(r)
	nrgba64 := image.NewNRGBA64(r)
	paletted := image.NewPaletted(r, palette.WebSafe)
	for _, m := range []interface {
		image.Image
		Set(x, y int, c color.Color)
	}{gray, gray16, rgba, nrgba, rgba64, nrgba64, paletted} {
		fill(m)
	}
	ycbcr := image.NewYCbCr(r, image.YCbCrSubsampleRatio420)
	for i := range ycbcr.Y {
		ycbcr.Y[i] = uint8(i * 7)
	}
	for i := range ycbcr.Cb {
		ycbcr.Cb[i], ycbcr.Cr[i] = uint8(100+i*5), uint8(200-i*3)
	}
	return map[string]image.Image{
		"Gray": gray, "Gray16": gray16, "RGBA": rgba, "NRGBA": nrgba,
		"RGBA64": rgba64, "NRGBA64": nrgba64, "YCbCr": ycbcr, "Paletted": paletted,
	}
}

// TestEachInput converts an image of each type with a profile of sRGB, and
// compares the result with the image.
func TestEachInput(t *testing.T) {
	t.Parallel()
	for name, img := range inputs() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := ToSRGB(t.Context(), img, srgbProfile(), Perceptual)
			if err != nil {
				t.Fatal(err)
			}
			_, wide := got.(*image.NRGBA64)
			if want := name == "Gray16" || name == "RGBA64" || name == "NRGBA64"; wide != want {
				t.Fatalf("result is %T", got)
			}
			if !wide {
				if _, ok := got.(*image.NRGBA); !ok {
					t.Fatalf("result is %T", got)
				}
			}
			sameColors(t, got, img, 1)
		})
	}
}

// TestNoProfile copies an image that is not CMYK, without a profile.
func TestNoProfile(t *testing.T) {
	t.Parallel()
	for name, img := range inputs() {
		got, err := ToSRGB(t.Context(), img, nil, Perceptual)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sameColors(t, got, img, 0)
	}
}

// TestAlpha keeps the alpha of a premultiplied image, and removes the
// premultiplication from its colors.
func TestAlpha(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 3, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 64, G: 32, B: 16, A: 128})
	img.SetRGBA(1, 0, color.RGBA{})
	img.SetRGBA(2, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	got, err := ToSRGB(t.Context(), img, srgbProfile(), Perceptual)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := got.(*image.NRGBA)
	if c := m.NRGBAAt(0, 0); c.A != 128 || !near(int(c.R), 127, 1) || !near(int(c.G), 64, 1) || !near(int(c.B), 32, 1) {
		t.Errorf("half alpha gives %v, want {127 64 32 128}", c)
	}
	if c := m.NRGBAAt(1, 0); c.A != 0 {
		t.Errorf("no alpha gives %v", c)
	}
	if c := m.NRGBAAt(2, 0); c != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Errorf("white gives %v", c)
	}

	img16 := image.NewNRGBA64(image.Rect(0, 0, 1, 1))
	img16.SetNRGBA64(0, 0, color.NRGBA64{R: 0x8080, G: 0x4040, B: 0x2020, A: 0x1234})
	got, err = ToSRGB(t.Context(), img16, srgbProfile(), Perceptual)
	if err != nil {
		t.Fatal(err)
	}
	if c := got.(*image.NRGBA64).NRGBA64At(0, 0); c.A != 0x1234 || !near(int(c.R>>8), 0x80, 1) {
		t.Errorf("an alpha of 16 bits gives %v", c)
	}
}

// Test16Bits converts an image of 16 bits to an *image.NRGBA64.
func Test16Bits(t *testing.T) {
	t.Parallel()
	img := image.NewGray16(image.Rect(0, 0, 2, 1))
	img.SetGray16(0, 0, color.Gray16{Y: 0x8000})
	img.SetGray16(1, 0, color.Gray16{Y: 0xffff})
	got, err := ToSRGB(t.Context(), img, linearGrayProfile(), Perceptual)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := got.(*image.NRGBA64)
	if !ok {
		t.Fatalf("result is %T", got)
	}
	// The sRGB curve gives 0.735 for the linear value 0.5.
	if c := m.NRGBA64At(0, 0); !near(int(c.R>>8), 188, 2) || c.R != c.G || c.G != c.B || c.A != 0xffff {
		t.Errorf("linear half gives %v, want 188 in sRGB", c)
	}
	if c := m.NRGBA64At(1, 0); c != (color.NRGBA64{R: 0xffff, G: 0xffff, B: 0xffff, A: 0xffff}) {
		t.Errorf("white gives %v", c)
	}
}

// TestGrayProfile converts the luma of a color image with a gray profile.
func TestGrayProfile(t *testing.T) {
	t.Parallel()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 128, G: 128, B: 128, A: 200})
	got, err := ToSRGB(t.Context(), img, linearGrayProfile(), Perceptual)
	if err != nil {
		t.Fatal(err)
	}
	if c := got.(*image.NRGBA).NRGBAAt(0, 0); !near(int(c.R), 188, 2) || c.A != 200 {
		t.Errorf("linear half gives %v, want 188 in sRGB", c)
	}
}

// cmykImage returns a CMYK image with white, black, cyan and a gray of the
// black ink.
func cmykImage() *image.CMYK {
	img := image.NewCMYK(image.Rect(0, 0, 4, 1))
	img.SetCMYK(0, 0, color.CMYK{})
	img.SetCMYK(1, 0, color.CMYK{K: 255})
	img.SetCMYK(2, 0, color.CMYK{C: 255})
	img.SetCMYK(3, 0, color.CMYK{K: 128})
	return img
}

// TestCMYK converts a CMYK image with and without a profile.
func TestCMYK(t *testing.T) {
	t.Parallel()
	img := cmykImage()
	without, err := ToSRGB(t.Context(), img, nil, Perceptual)
	if err != nil {
		t.Fatal(err)
	}
	with, err := ToSRGB(t.Context(), img, DefaultCMYK(), Perceptual)
	if err != nil {
		t.Fatal(err)
	}
	sameColors(t, without, with, 0)
	m := without.(*image.NRGBA)
	if c := m.NRGBAAt(0, 0); c.R < 240 || c.G < 240 || c.B < 240 || c.A != 255 {
		t.Errorf("paper gives %v, want near white", c)
	}
	if c := m.NRGBAAt(1, 0); c.R > 80 || c.G > 80 || c.B > 80 {
		t.Errorf("black ink gives %v, want near black", c)
	}
	if c := m.NRGBAAt(2, 0); c.R > 80 || c.B < 160 {
		t.Errorf("cyan ink gives %v, want cyan", c)
	}

	// A profile that is not CMYK cannot convert CMYK, and a CMYK profile
	// cannot convert RGB.
	if _, err := ToSRGB(t.Context(), img, srgbProfile(), Perceptual); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("CMYK with an RGB profile gives %v", err)
	}
	if _, err := ToSRGB(t.Context(), image.NewNRGBA(img.Rect), DefaultCMYK(), Perceptual); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("RGB with a CMYK profile gives %v", err)
	}
}

// TestDefaultCMYK gives a copy of the profile.
func TestDefaultCMYK(t *testing.T) {
	t.Parallel()
	a := DefaultCMYK()
	a[0] ^= 0xff
	if b := DefaultCMYK(); a[0] == b[0] {
		t.Error("DefaultCMYK shares its bytes")
	}
	p, err := Parse(DefaultCMYK())
	if err != nil {
		t.Fatal(err)
	}
	if p.space != spaceFour {
		t.Errorf("the space is %d", p.space)
	}
}

// TestIntents converts with each intent, and refuses an intent past the
// last.
func TestIntents(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"CGATS001Compat-v2-micro.icc", "ProPhoto-v2-micro.icc"} {
		p, err := Parse(readProfile(t, name))
		if err != nil {
			t.Fatal(err)
		}
		var img image.Image = image.NewNRGBA(image.Rect(0, 0, 2, 2))
		if p.space == spaceFour {
			img = cmykImage()
		}
		for i := Perceptual; i <= AbsoluteColorimetric; i++ {
			if _, err := p.ToSRGB(t.Context(), img, i); err != nil {
				t.Errorf("%s, %s: %v", name, i, err)
			}
		}
		if _, err := p.ToSRGB(t.Context(), img, AbsoluteColorimetric+1); !errors.Is(err, ivy.ErrUnsupported) {
			t.Errorf("%s: intent 4 gives %v", name, err)
		}
	}
}

// TestBadProfile refuses a profile that is not valid.
func TestBadProfile(t *testing.T) {
	t.Parallel()
	good := srgbProfile()
	for name, b := range map[string][]byte{
		"text":      []byte("this is not a profile"),
		"truncated": good[:200],
		"header":    good[:128],
		"signature": append(append([]byte(nil), good[:36]...), append([]byte("xxxx"), good[40:]...)...),
	} {
		if _, err := Parse(b); !errors.Is(err, ivy.ErrFormat) && !errors.Is(err, ivy.ErrUnsupported) {
			t.Errorf("%s: %v wraps neither ivy.ErrFormat nor ivy.ErrUnsupported", name, err)
		}
		if _, err := ToSRGB(t.Context(), image.NewNRGBA(image.Rect(0, 0, 1, 1)), b, Perceptual); err == nil {
			t.Errorf("%s: ToSRGB gives no error", name)
		}
	}
	if _, err := Parse([]byte("x")); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("a byte gives %v, want ivy.ErrFormat", err)
	}
	if _, err := Parse(make([]byte, MaxProfileSize)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("a large profile gives %v, want ivy.ErrLimit", err)
	}
}

// TestLimit refuses an image of more than MaxPixels pixels.
func TestLimit(t *testing.T) {
	t.Parallel()
	// The image has no pixels in memory, only its bounds.
	img := &image.Uniform{C: color.White}
	if _, err := ToSRGB(t.Context(), img, srgbProfile(), Perceptual); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("an image without end gives %v", err)
	}
	if _, err := ToSRGB(t.Context(), img, nil, Perceptual); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("an image without end and without a profile gives %v", err)
	}
}

// TestCancel stops when the context is done.
func TestCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for _, profile := range [][]byte{srgbProfile(), nil} {
		if _, err := ToSRGB(ctx, img, profile, Perceptual); !errors.Is(err, context.Canceled) {
			t.Errorf("a done context gives %v", err)
		}
	}
	if _, err := ToSRGB(ctx, cmykImage(), nil, Perceptual); !errors.Is(err, context.Canceled) {
		t.Errorf("a done context gives %v for CMYK", err)
	}
}

// TestConcurrent converts with one profile from several goroutines.
func TestConcurrent(t *testing.T) {
	t.Parallel()
	p, err := Parse(srgbProfile())
	if err != nil {
		t.Fatal(err)
	}
	img := inputs()["NRGBA"]
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := p.ToSRGB(t.Context(), img, Perceptual); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

// TestCICP converts with the profiles of CICP code points.
func TestCICP(t *testing.T) {
	t.Parallel()
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 0, G: 255, B: 0, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
	img.SetNRGBA(2, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 99})
	convert := func(c CICP) *image.NRGBA {
		t.Helper()
		p, err := FromCICP(c)
		if err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		got, err := p.ToSRGB(t.Context(), img, Perceptual)
		if err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		return got.(*image.NRGBA)
	}

	// sRGB converts to itself.
	sameColors(t, convert(CICP{1, 13, 0, true}), img, 1)

	// The green of BT.2020 is past the gamut of sRGB, and clips.
	bt2020 := convert(CICP{9, 1, 9, false})
	if c := bt2020.NRGBAAt(0, 0); c.R != 0 || c.G != 255 || c.B != 0 {
		t.Errorf("the green of BT.2020 gives %v", c)
	}
	if c := bt2020.NRGBAAt(2, 0); c != (color.NRGBA{R: 255, G: 255, B: 255, A: 99}) {
		t.Errorf("the white of BT.2020 gives %v", c)
	}

	// PQ has no tone mapping: the half of the code values is dark.
	pq := convert(CICP{9, 16, 9, false})
	if c := pq.NRGBAAt(1, 0); c.R > 100 || c.R != c.G || c.G != c.B {
		t.Errorf("the gray of PQ gives %v", c)
	}
	// HLG converts too.
	convert(CICP{9, 18, 9, false})
	// Display P3.
	if c := convert(CICP{12, 13, 0, true}).NRGBAAt(0, 0); c.R != 0 || c.G != 255 {
		t.Errorf("the green of Display P3 gives %v", c)
	}

	for _, c := range []CICP{
		{2, 13, 0, true}, {0, 13, 0, true}, {3, 13, 0, true}, {200, 13, 0, true},
		{1, 2, 0, true}, {1, 0, 0, true}, {1, 19, 0, true},
	} {
		if _, err := FromCICP(c); !errors.Is(err, ivy.ErrUnsupported) {
			t.Errorf("%v gives %v, want ivy.ErrUnsupported", c, err)
		}
	}
	if _, err := FromCICP(CICP{1, 13, 18, true}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("the matrix coefficients 18 give %v, want ivy.ErrFormat", err)
	}
}

// FuzzParse reads a profile, and when it is valid, converts a few images
// with it. It must never panic, and each error must wrap a sentinel error of
// ivy.
func FuzzParse(f *testing.F) {
	for _, name := range []string{"LAB.icc", "CGATS001Compat-v2-micro.icc", "ProPhoto-v2-micro.icc"} {
		f.Add(readProfile(f, name))
	}
	f.Add(srgbProfile())
	f.Add(linearGrayProfile())
	imgs := []image.Image{image.NewNRGBA(image.Rect(0, 0, 3, 2)), image.NewGray16(image.Rect(0, 0, 2, 2)), cmykImage()}
	for _, img := range imgs[:2] {
		fill(img.(interface {
			image.Image
			Set(x, y int, c color.Color)
		}))
	}
	check := func(t *testing.T, err error) {
		t.Helper()
		if err != nil && !errors.Is(err, ivy.ErrFormat) && !errors.Is(err, ivy.ErrUnsupported) && !errors.Is(err, ivy.ErrLimit) {
			t.Fatalf("error %v wraps no sentinel of ivy", err)
		}
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		p, err := Parse(b)
		check(t, err)
		if err != nil {
			return
		}
		for _, img := range imgs {
			for i := Perceptual; i <= AbsoluteColorimetric; i++ {
				_, err := p.ToSRGB(t.Context(), img, i)
				check(t, err)
			}
		}
	})
}
