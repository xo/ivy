// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the tests of the images that the renders of a document
// keep in the cache of the decoded images, which upstream does not have.

package render

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// imagePDF returns a PDF file of one page of 100 by 100 points. The page
// draws an image with a soft mask twice, at two scales, an image of JPX,
// and a stencil mask.
func imagePDF() []byte {
	var rgb, gray, jpx bytes.Buffer
	for y := range 16 {
		for x := range 16 {
			rgb.Write([]byte{byte(16 * x), byte(16 * y), byte(8 * (x + y))})
			gray.WriteByte(byte(255 - 8*(x+y)))
			jpx.Write([]byte{0, 0, 0})
		}
	}
	stencil := strings.Repeat("\xaa\x55\xf0\x0f", 32)
	content := "q 50 0 0 50 0 0 cm /Im0 Do Q q 8 0 0 8 60 60 cm /Im0 Do Q " +
		"q 30 0 0 30 60 0 cm /Im1 Do Q 1 0 0 rg q 20 0 0 20 0 60 cm /Im2 Do Q"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Contents 4 0 R " +
			"/Resources << /XObject << /Im0 5 0 R /Im1 7 0 R /Im2 8 0 R >> >> >>",
		pdfStream("", content),
		pdfStream("/Type /XObject /Subtype /Image /Width 16 /Height 16 /ColorSpace /DeviceRGB /BitsPerComponent 8 /SMask 6 0 R", rgb.String()),
		pdfStream("/Type /XObject /Subtype /Image /Width 16 /Height 16 /ColorSpace /DeviceGray /BitsPerComponent 8", gray.String()),
		pdfStream("/Type /XObject /Subtype /Image /Width 16 /Height 16 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /JPXDecode", jpx.String()),
		pdfStream("/Type /XObject /Subtype /Image /Width 32 /Height 32 /ImageMask true", stencil),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

// pdfStream returns a stream object with the entries of dict and data.
func pdfStream(dict, data string) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dict, len(data), data)
}

// jpxCounter is a decoder of JPX that counts its calls. It returns an image
// of 16 by 16 pixels of a color that the data does not give.
type jpxCounter struct {
	calls atomic.Int64
}

func (c *jpxCounter) decode([]byte, uint8, syntax.JPXSettings) (syntax.JPXImage, error) {
	c.calls.Add(1)
	data := make([]byte, 16*16*3)
	for i := range data {
		data[i] = byte(i)
	}
	return syntax.JPXImage{Width: 16, Height: 16, ColorSpace: syntax.JPXRGB, OriginalBitDepth: 8, Data: data}, nil
}

// settings returns the settings of the interpreter with the decoder of JPX
// of c and the limit of memory.
func (c *jpxCounter) settings(maxMemory int64) interpret.InterpreterSettings {
	settings := interpret.DefaultInterpreterSettings()
	settings.Limits.MaxMemory = maxMemory
	settings.Decoders = &syntax.ImageDecoders{JPX: c.decode}
	return settings
}

// renderImagePDF renders the page of imagePDF at 2 pixels for each point,
// moved up by dy pixels, onto a pixmap of 200 by 200 pixels.
func renderImagePDF(t *testing.T, page *syntax.Page, cache *RenderCache, settings interpret.InterpreterSettings, dy float64) []byte {
	t.Helper()
	data, err := renderImagePDFErr(page, cache, settings, dy)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// renderImagePDFErr is renderImagePDF for a goroutine of a test, which
// returns the error.
func renderImagePDFErr(page *syntax.Page, cache *RenderCache, settings interpret.InterpreterSettings, dy float64) ([]byte, error) {
	pix, ok := okage.NewPixmap(200, 200)
	if !ok {
		return nil, errors.New("expected a pixmap")
	}
	pix.Fill(okage.ColorWhite)
	transform := kurbo.AffineTranslate(kurbo.NewVec2(0, -dy)).
		Mul(kurbo.AffineScale(2)).
		Mul(interpret.TransformToKurbo(page.InitialTransform(true)))
	dev := display.NewOkage(pix, okage.IdentityTransform())
	if err := RenderInto(page, cache, settings, &RenderSettings{}, dev, 200, 200, transform); err != nil {
		return nil, err
	}
	return pix.Data(), nil
}

func imagePage(t *testing.T) *syntax.Page {
	t.Helper()
	pdf, err := syntax.NewPdf(imagePDF())
	if err != nil {
		t.Fatal(err)
	}
	return pdf.Pages().Slice()[0]
}

func TestImageCacheSamePixels(t *testing.T) {
	page := imagePage(t)
	var c jpxCounter
	cache := NewRenderCache()
	first := renderImagePDF(t, page, cache, c.settings(0), 0)
	second := renderImagePDF(t, page, cache, c.settings(0), 0)
	if n := c.calls.Load(); n != 1 {
		t.Errorf("expected 1 decoding of the image of JPX, got %d", n)
	}
	if !bytes.Equal(first, second) {
		t.Error("expected the same pixels from the cached images")
	}
	// the page drew each image
	var red, gray int
	for i := 0; i < len(first); i += 4 {
		switch r, g, b := first[i], first[i+1], first[i+2]; {
		case r > 200 && g < 50 && b < 50:
			red++
		case r != 255 || g != 255 || b != 255:
			gray++
		}
	}
	if red == 0 || gray == 0 {
		t.Errorf("expected the red of the stencil and the colors of the images, got %d and %d pixels", red, gray)
	}
}

func TestImageCacheOverBudget(t *testing.T) {
	page := imagePage(t)
	var cached jpxCounter
	want := renderImagePDF(t, page, NewRenderCache(), cached.settings(0), 0)
	// The budget is a quarter of 4000 bytes, less than the size of each image,
	// so the cache keeps no image. The images still decode and draw.
	var c jpxCounter
	cache := NewRenderCache()
	for range 2 {
		if got := renderImagePDF(t, page, cache, c.settings(4000), 0); !bytes.Equal(got, want) {
			t.Error("expected the same pixels without the cache")
		}
	}
	if n := c.calls.Load(); n != 2 {
		t.Errorf("expected 2 decodings of the image of JPX, got %d", n)
	}
}

func TestImageCacheConcurrent(t *testing.T) {
	page := imagePage(t)
	var c jpxCounter
	cache := NewRenderCache()
	want := make([][]byte, 8)
	for i := range want {
		want[i] = renderImagePDF(t, page, NewRenderCache(), c.settings(0), float64(25*i))
	}
	c.calls.Store(0)
	// The bands of a page render at the same time with one cache (D47).
	got := make([][]byte, 8)
	var wg sync.WaitGroup
	for i := range got {
		wg.Go(func() {
			var err error
			if got[i], err = renderImagePDFErr(page, cache, c.settings(0), float64(25*i)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := c.calls.Load(); n != 1 {
		t.Errorf("expected 1 decoding of the image of JPX, got %d", n)
	}
	for i := range got {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("band %d: expected the same pixels as without a shared cache", i)
		}
	}
}
