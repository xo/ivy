// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_cmyk_color_profiles.py: the CMYK colors and the color profiles.

package layout

import "testing"

// TestDrawDeviceCmykWithIcc is test_device_cmyk_with_icc in tests/draw/test_cmyk_color_profiles.py.
func TestDrawDeviceCmykWithIcc(t *testing.T) {
	t.Skip("the expected pixels come from the CMYK profile of Ghostscript, and ivy reads no color profile when it draws")
	assertPixels(t, "device_cmyk_with_icc", `PP
PP`, `
      <style>
        @color-profile device-cmyk {
          src: url(cmyk.icc);
          components: C, M, Y, K;
        }
        @page { size: 2px }
        html, body { height: 100%; margin: 0 }
        html { background: device-cmyk(0.8 0.6 0.4 0.2) }
      </style>
      <body>`)
}

// TestDrawDeviceCmykWithoutIcc is test_device_cmyk_without_icc in tests/draw/test_cmyk_color_profiles.py.
func TestDrawDeviceCmykWithoutIcc(t *testing.T) {
	t.Skip("the expected pixels come from the CMYK profile of Ghostscript, and ivy converts DeviceCMYK to sRGB without a profile")
	assertPixels(t, "device_cmyk_without_icc", `QQ
QQ`, `
      <style>
        @page { size: 2px }
        html, body { height: 100%; margin: 0 }
        html { background: device-cmyk(0.8 0.6 0.4 0.2) }
      </style>
      <body>`)
}

// TestDrawCustomCmykWithIcc is test_custom_cmyk_with_icc in tests/draw/test_cmyk_color_profiles.py.
func TestDrawCustomCmykWithIcc(t *testing.T) {
	t.Skip("the expected pixels come from the CMYK profile of Ghostscript, and ivy reads no color profile when it draws")
	assertPixels(t, "custom_cmyk_with_icc", `PP
PP`, `
      <style>
        @color-profile --custom-cmyk {
          src: url(cmyk.icc);
          components: C, M, Y, K;
        }
        @page { size: 2px }
        html, body { height: 100%; margin: 0 }
        html { background: color(--custom-cmyk 0.8 0.6 0.4 0.2) }
      </style>
      <body>`)
}

// TestDrawImageCmykWithoutIcc is test_image_cmyk_without_icc in tests/draw/test_cmyk_color_profiles.py.
func TestDrawImageCmykWithoutIcc(t *testing.T) {
	t.Skip("the JPEG decoder of Go does not decode the subsampling of this CMYK image, and the expected pixels come from the CMYK profile of Ghostscript")
	assertPixels(t, "image_cmyk_without_icc", `QQ
QQ`, `
      <style>
        @page { size: 2px }
        img { display: block; }
      </style>
      <img src="cmyk_without_icc.jpg">`)
}

// TestDrawImageCmykWithExternalIcc is test_image_cmyk_with_external_icc in tests/draw/test_cmyk_color_profiles.py.
func TestDrawImageCmykWithExternalIcc(t *testing.T) {
	t.Skip("the JPEG decoder of Go does not decode the subsampling of this CMYK image, and ivy reads no color profile when it draws")
	assertPixels(t, "image_cmyk_with_external_icc", `PP
PP`, `
      <style>
        @color-profile device-cmyk {
          src: url(cmyk.icc);
          components: C, M, Y, K;
        }
        @page { size: 2px }
        img { display: block; }
      </style>
      <img src="cmyk_without_icc.jpg">`)
}

// TestDrawImageCmykWithIcc is test_image_cmyk_with_icc in tests/draw/test_cmyk_color_profiles.py.
func TestDrawImageCmykWithIcc(t *testing.T) {
	t.Skip("the JPEG decoder of Go does not decode the subsampling of this CMYK image, and ivy reads no color profile when it draws")
	assertPixels(t, "image_cmyk_with_icc", `QQ
QQ`, `
      <style>
        @page { size: 2px }
        img { display: block; }
      </style>
      <img src="cmyk_with_icc.jpg">`)
}
