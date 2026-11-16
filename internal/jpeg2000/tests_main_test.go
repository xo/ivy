// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 tests/main.rs. It decodes the JPEG 2000
// files in testdata/streams and compares them against testdata/references.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testReference is Reference in hayro-jpeg2000 tests/main.rs.
type testReference struct {
	width     uint32
	height    uint32
	channels  int
	samples   []byte
	tolerance uint8
}

// reference is reference in hayro-jpeg2000 tests/main.rs. It reads the
// reference image of 8 bits.
func reference(t *testing.T, name string) testReference {
	t.Helper()
	path := filepath.Join("testdata", "references", name+".png")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to open %s: %v", path, err)
	}
	// The color type and the bit depth of the IHDR chunk.
	if len(data) < 26 {
		t.Fatalf("failed to load %s: too short", path)
	}
	if data[24] != 8 {
		t.Fatalf("%s: references are 8-bit", path)
	}
	channels := map[byte]int{0: 1, 2: 3, 4: 2, 6: 4}[data[25]]
	if channels == 0 {
		t.Fatalf("%s: unsupported color type %d", path, data[25])
	}
	m, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to load %s: %v", path, err)
	}
	b := m.Bounds()
	r := testReference{
		width:    binary.BigEndian.Uint32(data[16:20]),
		height:   binary.BigEndian.Uint32(data[20:24]),
		channels: channels,
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			switch m := m.(type) {
			case *image.Gray:
				r.samples = append(r.samples, m.GrayAt(x, y).Y)
			case *image.NRGBA:
				c := m.NRGBAAt(x, y)
				if channels == 2 {
					r.samples = append(r.samples, c.R, c.A)
				} else {
					r.samples = append(r.samples, c.R, c.G, c.B, c.A)
				}
			case *image.RGBA:
				c := m.RGBAAt(x, y)
				r.samples = append(r.samples, c.R, c.G, c.B)
			default:
				t.Fatalf("%s: unexpected image type %T", path, m)
			}
		}
	}
	return r
}

// withTolerance is Reference::tolerance.
func (r testReference) withTolerance(tolerance uint8) testReference {
	r.tolerance = tolerance
	return r
}

// compare is Reference::compare.
func (r testReference) compare(t *testing.T, stream string, actual testDecoded) {
	t.Helper()
	if actual.width != r.width || actual.height != r.height {
		t.Fatalf("%s: wrong dimensions: got %dx%d, want %dx%d", stream, actual.width, actual.height, r.width, r.height)
	}
	if actual.channels != r.channels {
		t.Fatalf("%s: wrong number of components: got %d, want %d", stream, actual.channels, r.channels)
	}

	absDiff := func(a, b byte) byte {
		if a > b {
			return a - b
		}
		return b - a
	}
	first := -1
	var largest byte
	for i := range min(len(actual.samples), len(r.samples)) {
		d := absDiff(actual.samples[i], r.samples[i])
		if d > r.tolerance && first < 0 {
			first = i
		}
		largest = max(largest, d)
	}
	if first >= 0 {
		pixel := first / actual.channels
		t.Fatalf("%s: first mismatch in component %d at row %d, column %d: expected %d, got %d (largest difference %d)",
			stream, first%actual.channels, pixel/int(actual.width), pixel%int(actual.width),
			r.samples[first], actual.samples[first], largest)
	}
}

// streamPath is stream_path in hayro-jpeg2000 tests/main.rs.
func streamPath(t *testing.T, stream string) string {
	t.Helper()
	for _, ext := range []string{"j2k", "jp2"} {
		p := filepath.Join("testdata", "streams", stream+"."+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Fatalf("no stream named %s", stream)
	return ""
}

// testDecoded is Decoded in hayro-jpeg2000 tests/main.rs.
type testDecoded struct {
	width    uint32
	height   uint32
	channels int
	samples  []byte
	// iccProfile reports that decodeOutput converted the samples of a gray
	// or RGB ICC profile to sRGB.
	iccProfile bool
}

// readStream is read_stream in hayro-jpeg2000 tests/main.rs.
func readStream(t *testing.T, stream string) []byte {
	t.Helper()
	path := streamPath(t, stream)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return data
}

// decodeStream is decode in hayro-jpeg2000 tests/main.rs.
func decodeStream(t *testing.T, stream string, settings DecodeSettings) (testDecoded, error) {
	t.Helper()
	data := readStream(t, stream)
	img, err := New(data, &settings)
	if err != nil {
		return testDecoded{}, err
	}
	width, height := img.Width(), img.Height()
	var ctx DecoderContext
	decoded, err := img.Decode(&ctx)
	if err != nil {
		return testDecoded{}, err
	}
	samples, err := decoded.DataU8()
	if err != nil {
		return testDecoded{}, err
	}
	return testDecoded{
		width:    width,
		height:   height,
		channels: len(decoded.Components()),
		samples:  samples,
	}, nil
}

// errNeedsColorManagement marks a test that needs the port of moxcms.
var errNeedsColorManagement = errors.New("the test needs the port of moxcms")

// decodeOutput is decode_output in hayro-jpeg2000 tests/main.rs.
//
// Port: upstream decodes through the crate image and converts the ICC
// profile with moxcms. The port decodes through ReadImage, with the port of
// moxcms. ReadImage keeps the samples of a gray or RGB ICC profile, and it
// returns that profile from ICCProfile. So decodeOutput then converts the
// samples to sRGB, as upstream does with to_srgb.
func decodeOutput(t *testing.T, stream string) (testDecoded, error) {
	t.Helper()
	img, err := New(readStream(t, stream), nil)
	if err != nil {
		return testDecoded{}, err
	}
	width, height := img.Dimensions()
	samples := make([]byte, img.TotalBytes())
	if err := img.ReadImage(samples, MoxcmsTransform); err != nil {
		return testDecoded{}, err
	}
	profile := img.ICCProfile()
	if profile != nil {
		if samples, err = testToSRGB(profile, img.ColorType().ChannelCount(), samples); err != nil {
			return testDecoded{}, err
		}
	}
	return testDecoded{
		width:      width,
		height:     height,
		channels:   img.ColorType().ChannelCount(),
		samples:    samples,
		iccProfile: profile != nil,
	}, nil
}

// testToSRGB is to_srgb in hayro-jpeg2000 tests/main.rs. It converts the
// samples of the ICC profile to sRGB, in the layout of their channel count.
func testToSRGB(profile []byte, channels int, samples []byte) ([]byte, error) {
	layout := LayoutRGBA
	switch channels {
	case 1:
		layout = LayoutGray
	case 2:
		layout = LayoutGrayAlpha
	case 3:
		layout = LayoutRGB
	}
	out := make([]byte, len(samples))
	if err := MoxcmsTransform(profile, samples, layout, out, layout); err != nil {
		return nil, err
	}
	return out, nil
}

// check is check in hayro-jpeg2000 tests/main.rs.
func check(t *testing.T, stream string, expected testReference) {
	t.Helper()
	checkWith(t, stream, DefaultDecodeSettings(), expected)
}

// checkWith is check_with in hayro-jpeg2000 tests/main.rs.
func checkWith(t *testing.T, stream string, settings DecodeSettings, expected testReference) {
	t.Helper()
	actual, err := decodeStream(t, stream, settings)
	if err != nil {
		t.Fatalf("%s: decoding failed: %v", stream, err)
	}
	expected.compare(t, stream, actual)
}

// checkComponents is check_components in hayro-jpeg2000 tests/main.rs. It
// compares the components side by side, as one gray image.
func checkComponents(t *testing.T, stream string, expected testReference) {
	t.Helper()
	actual, err := decodeStream(t, stream, DefaultDecodeSettings())
	if err != nil {
		t.Fatalf("%s: decoding failed: %v", stream, err)
	}
	width, channels := int(actual.width), actual.channels
	var samples []byte
	for row := 0; (row+1)*width*channels <= len(actual.samples); row++ {
		r := actual.samples[row*width*channels : (row+1)*width*channels]
		for c := range channels {
			for i := c; i < len(r); i += channels {
				samples = append(samples, r[i])
			}
		}
	}
	sideBySide := testDecoded{
		width:    actual.width * uint32(channels),
		height:   actual.height,
		channels: 1,
		samples:  samples,
	}
	expected.compare(t, stream, sideBySide)
}

// checkOutput is check_output in hayro-jpeg2000 tests/main.rs.
func checkOutput(t *testing.T, stream string, expected testReference) {
	t.Helper()
	actual, err := decodeOutput(t, stream)
	if errors.Is(err, errNeedsColorManagement) {
		t.Skipf("%s: %v", stream, err)
	}
	if err != nil {
		t.Fatalf("%s: decoding failed: %v", stream, err)
	}
	// Port: upstream made its references with moxcms 0.8.1. The port of
	// moxcms follows moxcms 0.9.2, which adapts the colorants of sRGB to the
	// PCS illuminant (0.9642, 1, 0.8249) of ICC.1 instead of the D50 of
	// Chromaticity. A sample of a gray or RGB ICC profile can then differ by
	// one, so the test allows that.
	if actual.iccProfile && expected.tolerance < 1 {
		expected.tolerance = 1
	}
	expected.compare(t, stream, actual)
}

// checkError is check_error in hayro-jpeg2000 tests/main.rs.
func checkError(t *testing.T, stream string, settings DecodeSettings, message string) {
	t.Helper()
	_, err := decodeStream(t, stream, settings)
	if err == nil {
		t.Fatalf("%s: decoding succeeded, but should have failed", stream)
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("%s: expected an error containing %q, got %q", stream, message, err.Error())
	}
}
