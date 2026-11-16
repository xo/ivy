package jpeg2000

// This file ports no file of hayro-jpeg2000. It holds the tests of the
// parts that the port adds: the sentinel errors, the limits, the ICC hook,
// the package image of Go, and the fuzz test.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
)

func TestErrorsWrapSentinels(t *testing.T) {
	_, err := New([]byte("not a jpeg 2000 file"), nil)
	if !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("New(garbage) = %v, want ivy.ErrFormat", err)
	}
	var de *DecodeError
	if !errors.As(err, &de) || de.Kind() != KindFormat {
		t.Errorf("New(garbage) = %v, want a *DecodeError of KindFormat", err)
	}

	s := DefaultDecodeSettings()
	s.Limits.MaxPixels = 100
	img, err := New(readStream(t, "codestream_gray"), &s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := img.Decode(nil); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("Decode with MaxPixels 100 = %v, want ivy.ErrLimit", err)
	}

	s = DefaultDecodeSettings()
	s.Cancel = NewCancelHandle(CancelFunc(func() bool { return true }))
	img, err = New(readStream(t, "codestream_gray"), &s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := img.Decode(nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Decode cancelled = %v, want context.Canceled", err)
	}
}

func TestFromCodestream(t *testing.T) {
	img, err := FromCodestream(readStream(t, "codestream_gray"), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cs := img.ColorSpace(); cs.Kind != ColorSpaceGray || cs.NumChannels() != 1 || img.HasAlpha() {
		t.Errorf("color space = %+v, alpha %v, want gray without alpha", cs, img.HasAlpha())
	}
	if _, err := FromCodestream(readStream(t, "jp2_rgba"), 3, nil); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("FromCodestream(JP2) = %v, want ivy.ErrFormat", err)
	}
}

// recordingTransform returns an ICCTransform that records its calls and
// copies the color channels that both layouts share.
func recordingTransform(calls *[][3]any) ICCTransform {
	return func(profile, src []byte, srcLayout Layout, dst []byte, dstLayout Layout) error {
		*calls = append(*calls, [3]any{len(profile), srcLayout, dstLayout})
		in := [...]int{1, 2, 3, 4}[srcLayout]
		out := [...]int{1, 2, 3, 4}[dstLayout]
		for i := 0; (i+1)*in <= len(src) && (i+1)*out <= len(dst); i++ {
			copy(dst[i*out:(i+1)*out], src[i*in:i*in+min(in, out)])
		}
		return nil
	}
}

func TestICCTransform(t *testing.T) {
	var calls [][3]any
	img, err := New(readStream(t, "integration_cmyk"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if img.ColorType() != ColorTypeRgb8 || img.ICCProfile() != nil {
		t.Fatalf("color type = %v, profile %v, want RGB without a profile", img.ColorType(), img.ICCProfile() != nil)
	}
	buf := make([]byte, img.TotalBytes())
	if err := img.ReadImage(buf, recordingTransform(&calls)); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != [3]any{len(cmykProfile), LayoutRGBA, LayoutRGB} {
		t.Errorf("calls = %v, want one from RGBA to RGB with the CMYK profile", calls)
	}

	// Without a transform, the port converts CMYK without a profile.
	if err := img.ReadImage(buf, nil); err != nil {
		t.Errorf("ReadImage without a transform: %v", err)
	}

	calls = nil
	m, err := DecodeImage(readStream(t, "integration_icc_rgb"), nil, recordingTransform(&calls))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.(*image.RGBA); !ok {
		t.Errorf("DecodeImage = %T, want *image.RGBA", m)
	}
	if len(calls) != 1 || calls[0][1] != LayoutRGB || calls[0][2] != LayoutRGB {
		t.Errorf("calls = %v, want one from RGB to RGB", calls)
	}
}

func TestDecodeImage(t *testing.T) {
	for _, tc := range []struct {
		stream string
		want   string
	}{
		{"codestream_gray", "*image.Gray"},
		{"jp2_gray_alpha", "*image.NRGBA"},
		{"jp2_colr_srgb", "*image.RGBA"},
		{"jp2_rgba", "*image.NRGBA"},
	} {
		m, err := DecodeImage(readStream(t, tc.stream), nil, nil)
		if err != nil {
			t.Errorf("%s: %v", tc.stream, err)
			continue
		}
		if got := fmt.Sprintf("%T", m); got != tc.want {
			t.Errorf("%s: DecodeImage = %s, want %s", tc.stream, got, tc.want)
		}
	}

	RegisterDecodingHook(nil)
	if RegisterDecodingHook(nil) {
		t.Error("RegisterDecodingHook registered twice")
	}
	m, format, err := image.Decode(bytes.NewReader(readStream(t, "jp2_rgba")))
	if err != nil || format != "jp2" {
		t.Fatalf("image.Decode = %v, %q, want jp2", err, format)
	}
	ref := reference(t, "main_rgba")
	if b := m.Bounds(); uint32(b.Dx()) != ref.width || uint32(b.Dy()) != ref.height {
		t.Errorf("image.Decode bounds = %v, want %dx%d", b, ref.width, ref.height)
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(readStream(t, "codestream_gray"))); err != nil || format != "j2c" {
		t.Errorf("image.DecodeConfig = %v, %q, want j2c", err, format)
	}
}

// FuzzDecode decodes malformed files. It fails when the decoder panics,
// which recoverPanic turns into an error.
func FuzzDecode(f *testing.F) {
	for _, name := range []string{
		"codestream_tiny.j2k",
		"codestream_single_row.j2k",
		"codestream_levels_more_than_size.j2k",
		"codestream_precincts_1x1.j2k",
		"codestream_subsampling_all_odd.j2k",
		"palette_small.jp2",
		"wavelet_reduced_precinct_limited.j2k",
		"quantization_irreversible_tiny_tiles.j2k",
		"tile_parts_truncated_layers.j2k",
		"jp2_colr_sycc_420.jp2",
		"packets_ppt_sop_eph_tiny_tiles.j2k",
	} {
		data, err := os.ReadFile(filepath.Join("testdata", "streams", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	settings := DefaultDecodeSettings()
	settings.Limits = ivy.Limits{MaxMemory: 1 << 24, MaxPixels: 1 << 16, MaxObjects: 1 << 12}
	failOnPanic := func(t *testing.T, err error) {
		t.Helper()
		var de *DecodeError
		if errors.As(err, &de) && de.stack != nil {
			t.Fatalf("%v\n%s", err, de.stack)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		img, err := New(data, &settings)
		failOnPanic(t, err)
		if err != nil {
			return
		}
		decoded, err := img.Decode(nil)
		failOnPanic(t, err)
		if err == nil {
			_, err = decoded.DataU8()
			failOnPanic(t, err)
		}
		if uint64(img.Width())*uint64(img.Height()) <= 1<<16 {
			buf := make([]byte, img.TotalBytes())
			var calls [][3]any
			failOnPanic(t, img.ReadImage(buf, recordingTransform(&calls)))
		}
	})
}
