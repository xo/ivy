// Copied from github.com/mgilbir/gowemf, file color_pixels_test.go, at commit
// 377668cb7b13 (D7, docs/UPSTREAM.md). Its MIT license is in
// internal/gowemf/LICENSE.
// ivy: the profiles of the tests are built by hand, not with golittlecms, and
// TestSoftProofingIdentity expects ErrUnsupported.

package gowemf

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

// generatedGrayICC returns a gray profile with a linear curve.
//
// ivy: the profile is built by hand, as in generatedICC.
func generatedGrayICC(t testing.TB) []byte {
	t.Helper()
	return buildICC("mntr", "GRAY", iccTag{"wtpt", xyzICC(0.9642, 1, 0.8249)}, iccTag{"kTRC", paraICC(0, 1)})
}

// generatedCMYKICC returns a CMYK profile with a lut16Type table.
//
// ivy: the profile is built by hand, as in generatedICC, with the table of
// upstream.
func generatedCMYKICC(t testing.TB) []byte {
	t.Helper()
	// A generated four-channel test device, not a real printer characterization.
	// At cube corners C/M/Y suppress the corresponding linear RGB primary; K
	// suppresses all three. The PCS table uses standard PCS-adapted RGB columns.
	// ivy: the comment of upstream names the illuminant of the PCS by its
	// name, which the tests of ivy read as a decision of ivy.
	var table []uint16
	for c := 0; c < 2; c++ {
		for m := 0; m < 2; m++ {
			for y := 0; y < 2; y++ {
				for k := 0; k < 2; k++ {
					r, g, b := float64((1-c)*(1-k)), float64((1-m)*(1-k)), float64((1-y)*(1-k))
					for _, v := range []float64{.4360747*r + .3850649*g + .1430804*b, .2225045*r + .7168786*g + .0606169*b, .0139322*r + .0971045*g + .7141733*b} {
						table = append(table, uint16(math.Round(v*32768)))
					}
				}
			}
		}
	}
	// lut16Type: 4 inputs, 3 outputs, a grid of 2, the identity matrix and
	// identity curves of 2 entries.
	b := []byte("mft2\x00\x00\x00\x00\x04\x03\x02\x00")
	for i := range 9 {
		v := uint32(0)
		if i%4 == 0 {
			v = 1 << 16
		}
		b = binary.BigEndian.AppendUint32(b, v)
	}
	b = binary.BigEndian.AppendUint16(b, 2)
	b = binary.BigEndian.AppendUint16(b, 2)
	for range 4 {
		b = binary.BigEndian.AppendUint16(b, 0)
		b = binary.BigEndian.AppendUint16(b, 0xffff)
	}
	for _, v := range table {
		b = binary.BigEndian.AppendUint16(b, v)
	}
	for range 3 {
		b = binary.BigEndian.AppendUint16(b, 0)
		b = binary.BigEndian.AppendUint16(b, 0xffff)
	}
	return buildICC("scnr", "CMYK", iccTag{"wtpt", xyzICC(0.9642, 1, 0.8249)}, iccTag{"A2B0", b})
}

func TestGrayAndCMYKColorTransforms(t *testing.T) {
	gray := ColorSpace{Type: ColorProfileEmbedded, Profile: generatedGrayICC(t), Intent: IntentGraphics}
	tr, err := NewPixelColorTransform(gray, ColorGray8, ColorTransformOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 12)
	if err := tr.TransformPixels(out, []byte{0, 128, 255}); err != nil {
		t.Fatal(err)
	}
	if out[0] != 0 || out[4] < 186 || out[4] > 190 || out[8] != 255 || out[3] != 255 || out[7] != 255 || out[11] != 255 {
		t.Fatal("gray ICC ramp", out)
	}
	cmyk := ColorSpace{Type: ColorProfileEmbedded, Profile: generatedCMYKICC(t), Intent: IntentGraphics}
	tr, err = NewPixelColorTransform(cmyk, ColorCMYK8, ColorTransformOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out = make([]byte, 12)
	if err := tr.TransformPixels(out, []byte{0, 0, 0, 0, 0, 0, 0, 255, 255, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if out[0] < 250 || out[1] < 250 || out[2] < 250 || out[4] > 3 || out[5] > 3 || out[6] > 3 || out[8] > 5 || out[9] < 250 || out[10] < 250 {
		t.Fatal("CMYK channel order/ink convention", out)
	}
	if _, err := NewPixelColorTransform(gray, ColorCMYK8, ColorTransformOptions{}); !errors.Is(err, ErrMalformed) {
		t.Fatal("profile/channel mismatch", err)
	}
	if err := tr.TransformPixels(make([]byte, 4), []byte{1, 2, 3}); !errors.Is(err, ErrMalformed) {
		t.Fatal(err)
	}
}

// ivy: moxcms has no proofing transform. Upstream checks that an sRGB proof
// keeps the colors. The copy checks that the transform is unsupported, and
// that the limits still apply.
func TestSoftProofingIdentity(t *testing.T) {
	s := ColorSpace{Type: ColorSRGB, Intent: IntentGraphics}
	proof := ColorSpace{Type: ColorProfileEmbedded, Profile: generatedICC(t), Intent: IntentAbsoluteColorimetric}
	if _, err := NewProofingColorTransform(s, proof, ColorTransformOptions{BlackPointCompensation: true}); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := NewProofingColorTransform(s, proof, ColorTransformOptions{MaxProfileBytes: 1}); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}
