// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the tests of the port that upstream does not have: the
// replaced decoders, the hooks of JBIG2 and JPX, and the fuzz test.

package syntax

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
)

func TestInlineImage(t *testing.T) {
	it := NewTypedIter([]byte("q BI /W 2 /H 1 /BPC 8 /CS /G ID \x00\xff EI Q"))
	if _, ok := next(t, it).(SaveState); !ok {
		t.Error("q")
	}
	// The data keeps the white space before EI, as upstream does.
	op, ok := next(t, it).(InlineImage)
	if !ok || !bytes.Equal(op.V0.data, []byte{0x00, 0xff, ' '}) {
		t.Errorf("BI: %v", op)
	}
	if _, ok := next(t, it).(RestoreState); !ok {
		t.Error("Q")
	}
	expectEnd(t, it)
}

func TestPngPredictor(t *testing.T) {
	params, _ := FromBytes[Dict]([]byte("<< /Predictor 12 /Columns 3 >>"))
	// Two rows of the filter Up after a row of the filter Sub.
	data := []byte{1, 1, 1, 1, 2, 1, 1, 1}
	got, ok := applyPredictor(data, params)
	if !ok || !bytes.Equal(got, []byte{1, 2, 3, 2, 3, 4}) {
		t.Fatalf("got %v, %v", got, ok)
	}
}

func TestTiffPredictor(t *testing.T) {
	params, _ := FromBytes[Dict]([]byte("<< /Predictor 2 /Columns 3 /Colors 1 >>"))
	got, ok := applyPredictor([]byte{1, 1, 1, 5, 1, 1}, params)
	if !ok || !bytes.Equal(got, []byte{1, 2, 3, 5, 6, 7}) {
		t.Fatalf("got %v, %v", got, ok)
	}
}

func TestFlateLimit(t *testing.T) {
	var b bytes.Buffer
	b.Write([]byte{0x78, 0x9c})
	zeros := make([]byte, 1<<16)
	compressed := deflate(t, zeros)
	b.Write(compressed)
	_, err := decodeFlate(b.Bytes(), EmptyDict(), 1<<10)
	if !errors.Is(err, ivy.ErrLimit) {
		t.Fatalf("got %v", err)
	}
}

// deflate returns data as raw DEFLATE data in stored blocks, which need no
// compression.
func deflate(t *testing.T, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	for len(data) > 0 {
		n := min(len(data), 0xFFFF)
		final := byte(0)
		if n == len(data) {
			final = 1
		}
		out.WriteByte(final)
		out.Write([]byte{byte(n), byte(n >> 8), ^byte(n), ^byte(n >> 8)})
		out.Write(data[:n])
		data = data[n:]
	}
	return out.Bytes()
}

func TestDctGrayAndRGB(t *testing.T) {
	gray := image.NewGray(image.Rect(0, 0, 4, 2))
	for i := range gray.Pix {
		gray.Pix[i] = 0x80
	}
	rgb := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for i := 0; i < len(rgb.Pix); i += 4 {
		rgb.Pix[i], rgb.Pix[i+1], rgb.Pix[i+2], rgb.Pix[i+3] = 0xFF, 0, 0, 0xFF
	}
	tests := []struct {
		img   image.Image
		cs    ImageColorSpaceKind
		bytes int
	}{
		{gray, ImageColorSpaceGray, 8},
		{rgb, ImageColorSpaceRGB, 24},
	}
	for _, test := range tests {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, test.img, &jpeg.Options{Quality: 100}); err != nil {
			t.Fatal(err)
		}
		res, err := decodeDct(buf.Bytes(), EmptyDict(), &ImageDecodeParams{Width: 4, Height: 2}, defaultDecodeEnv())
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Data) != test.bytes || res.ImageData.ColorSpace.Kind != test.cs || res.ImageData.Width != 4 {
			t.Errorf("got %d bytes, %v", len(res.Data), res.ImageData.ColorSpace)
		}
		if test.cs == ImageColorSpaceRGB {
			r, g, b := res.Data[0], res.Data[1], res.Data[2]
			if r < 0xF0 || g > 0x10 || b > 0x10 {
				t.Errorf("got the color %d %d %d", r, g, b)
			}
		}
	}
}

// TestCcittLimit makes sure that a CCITT image past the memory limit fails
// with ivy.ErrLimit, with known rows and with unknown rows.
func TestCcittLimit(t *testing.T) {
	env := &decodeEnv{maxMemory: 4, maxPixels: 1 << 20}
	for _, params := range []string{"<< /K 0 /Columns 8 /Rows 1 >>", "<< /K 0 /Columns 8 >>"} {
		dict, _ := FromBytes[Dict]([]byte(params))
		for _, indexed := range []bool{false, true} {
			if indexed {
				// One packed row of 8 columns takes 1 byte.
				env.maxMemory = 0
			}
			_, err := decodeCcitt([]byte{0x35, 0x14}, dict, &ImageDecodeParams{IsIndexed: indexed}, env)
			if !errors.Is(err, ivy.ErrLimit) {
				t.Errorf("%s, indexed %v: got %v, want ivy.ErrLimit", params, indexed, err)
			}
			env.maxMemory = 4
		}
	}
}

func TestImageHooks(t *testing.T) {
	dict, _ := FromBytes[Dict]([]byte("<< /Filter /JBIG2Decode >>"))
	stream := newStream([]byte("data"), dict)
	if _, err := stream.DecodedImage(&ImageDecodeParams{}); !errors.Is(err, ivy.ErrUnsupported) {
		t.Fatalf("without a hook: got %v", err)
	}
	decoders := &ImageDecoders{
		JBIG2: func(_, _ []byte) (JBIG2Bitmap, error) {
			// A page of 3 by 1 pixels: black, white, black.
			return JBIG2Bitmap{Width: 3, Height: 1, Data: []byte{0b10100000}}, nil
		},
		JPX: func(_ []byte, channels uint8, _ JPXSettings) (JPXImage, error) {
			if channels != 0 {
				return JPXImage{}, fmt.Errorf("no codestream: %w", ivy.ErrFormat)
			}
			return JPXImage{Width: 1, Height: 1, ColorSpace: JPXRGB, OriginalBitDepth: 8, HasAlpha: true, Data: []byte{1, 2, 3, 4}}, nil
		},
	}
	res, err := stream.DecodedImage(&ImageDecodeParams{Decoders: decoders})
	if err != nil || !bytes.Equal(res.Data, []byte{0, 0xFF, 0}) {
		t.Fatalf("jbig2: got %v, %v", res.Data, err)
	}
	res, err = stream.DecodedImage(&ImageDecodeParams{IsIndexed: true, Decoders: decoders})
	if err != nil || !bytes.Equal(res.Data, []byte{0b01000000}) {
		t.Fatalf("jbig2 indexed: got %08b, %v", res.Data, err)
	}
	dict, _ = FromBytes[Dict]([]byte("<< /Filter /JPXDecode >>"))
	stream = newStream([]byte("data"), dict)
	res, err = stream.DecodedImage(&ImageDecodeParams{NumComponents: 3, Decoders: decoders})
	if err != nil || !bytes.Equal(res.Data, []byte{1, 2, 3}) || !bytes.Equal(res.ImageData.Alpha, []byte{4}) {
		t.Fatalf("jpx: got %v, %v", res.Data, err)
	}
}

func TestPasswordError(t *testing.T) {
	err := error(&LoadPdfError{Decryption: DecryptionPasswordProtected})
	if !errors.Is(err, ivy.ErrPassword) {
		t.Fatal("not ErrPassword")
	}
}

// FuzzNewPdf reads a whole file, its pages, their operators and the streams
// of its objects.
func FuzzNewPdf(f *testing.F) {
	files, _ := filepath.Glob("testdata/*.pdf")
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte("%PDF-1.4\n1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n2 0 obj << /Type /Pages /Kids [3 0 R] >> endobj\n3 0 obj << /Contents 4 0 R >> endobj\n4 0 obj << /Length 5 >> stream\n0 0 m\nendstream endobj\n"))
	f.Fuzz(func(_ *testing.T, data []byte) {
		fuzzOne(data)
	})
}

// fuzzOne reads data as FuzzNewPdf does.
func fuzzOne(data []byte) {
	{
		opts := Options{Limits: ivy.Limits{MaxMemory: 1 << 22, MaxPixels: 1 << 20}}
		pdf, err := NewPdfWithOptions(data, opts)
		if err != nil {
			return
		}
		_ = pdf.Version()
		_ = pdf.Metadata()
		for _, page := range pdf.Pages().Slice() {
			_ = page.InitialTransform(true)
			ops := page.TypedOperations()
			for range 10000 {
				op, ok := ops.Next()
				if !ok {
					break
				}
				_ = op.String()
			}
		}
		n := 0
		for o := range pdf.Objects() {
			if n++; n > 1000 {
				break
			}
			_ = o.String()
			if s, ok := o.(Stream); ok {
				_, _ = s.DecodedImage(&ImageDecodeParams{Width: 16, Height: 16})
			}
		}
	}
}
