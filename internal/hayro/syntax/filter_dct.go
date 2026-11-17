// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/dct.rs.
//
// Port: the JPEG decoder zune-jpeg becomes image/jpeg (D16). The decoder of
// Go converts the colors of CMYK and YCCK images, and it removes the Adobe
// inversion. This file undoes both, so that the bytes follow upstream: raw
// CMYK, and YCCK through the formula of pdf.js, which gives 255 minus each
// channel of the decoder of Go.

package syntax

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"

	"github.com/xo/ivy"
)

// dctColorSpace is the subset of the ColorSpace of zune-jpeg that dct.rs
// uses.
type dctColorSpace uint8

const (
	dctRGB dctColorSpace = iota
	dctYCbCr
	dctLuma
	dctCMYK
	dctYCCK
	dctMultiBand
)

// decodeDct is decode in hayro-syntax src/filter/dct.rs.
func decodeDct(data []byte, params Dict, imageParams *ImageDecodeParams, env *decodeEnv) (FilterResult, error) {
	if imageParams.Width > 0xFFFF || imageParams.Height > 0xFFFF {
		return FilterResult{}, fmt.Errorf("decoding jpeg: image too large: %w", ivy.ErrLimit)
	}
	// Some JPEGs have bogus oversized dimensions, so patch them from the
	// image dictionary.
	data, ok := maybePatchJpegDimensions(data, imageParams)
	if !ok {
		return FilterResult{}, fmt.Errorf("decoding jpeg: no frame header: %w", ivy.ErrFormat)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return FilterResult{}, fmt.Errorf("decoding jpeg: %w: %w", ivy.ErrFormat, err)
	}
	pixels := int64(cfg.Width) * int64(cfg.Height)
	if cfg.Width > 0xFFFF || cfg.Height > 0xFFFF || pixels > env.maxPixels || pixels*4 > env.maxMemory {
		return FilterResult{}, fmt.Errorf("decoding jpeg: image of %dx%d: %w", cfg.Width, cfg.Height, ivy.ErrLimit)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	var unsupported jpeg.UnsupportedError
	if errors.As(err, &unsupported) && len(data) >= 2 {
		// A 4-component JPEG without the APP14 marker of Adobe is CMYK for
		// zune-jpeg, but the decoder of Go refuses it. A marker with the
		// transform 0 makes the decoder of Go read it as CMYK.
		img, err = jpeg.Decode(bytes.NewReader(withAdobeMarker(data)))
	}
	if err != nil {
		return FilterResult{}, fmt.Errorf("decoding jpeg: %w: %w", ivy.ErrFormat, err)
	}
	var input dctColorSpace
	switch img.(type) {
	case *image.Gray:
		input = dctLuma
	case *image.YCbCr:
		input = dctYCbCr
	case *image.CMYK:
		input = dctCMYK
	default:
		input = dctRGB
	}
	colorTransform, hasColorTransform := params.Get[uint8](KeyColorTransform)
	var out dctColorSpace
	if n := imageParams.NumComponents; n != 0 && n != 1 && n != 3 && n != 4 {
		out = dctMultiBand
	} else {
		switch input {
		case dctYCbCr:
			if !hasColorTransform || colorTransform == 1 {
				out = dctRGB
			} else {
				out = dctYCbCr
			}
		case dctLuma:
			out = dctLuma
		case dctCMYK:
			out = dctCMYK
		default:
			out = dctRGB
		}
	}
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	var decoded []byte
	switch img := img.(type) {
	case *image.Gray:
		decoded = make([]byte, 0, width*height)
		for y := range height {
			decoded = append(decoded, img.Pix[y*img.Stride:y*img.Stride+width]...)
		}
	case *image.YCbCr:
		decoded = make([]byte, 0, width*height*3)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				yy := img.Y[img.YOffset(x, y)]
				ci := img.COffset(x, y)
				cb, cr := img.Cb[ci], img.Cr[ci]
				if out == dctRGB {
					r, g, bl := color.YCbCrToRGB(yy, cb, cr)
					decoded = append(decoded, r, g, bl)
				} else {
					decoded = append(decoded, yy, cb, cr)
				}
			}
		}
	case *image.CMYK:
		// The decoder of Go inverts each channel of CMYK, and gives 255 minus
		// the result of the formula of pdf.js for YCCK. Both become 255
		// minus the value of Go.
		decoded = make([]byte, 0, width*height*4)
		for y := range height {
			for _, v := range img.Pix[y*img.Stride : y*img.Stride+width*4] {
				decoded = append(decoded, 255-v)
			}
		}
	case *image.RGBA:
		decoded = make([]byte, 0, width*height*3)
		for y := range height {
			row := img.Pix[y*img.Stride : y*img.Stride+width*4]
			for x := 0; x < len(row); x += 4 {
				decoded = append(decoded, row[x], row[x+1], row[x+2])
			}
		}
	default:
		return FilterResult{}, fmt.Errorf("decoding jpeg: color model %T: %w", img, ivy.ErrUnsupported)
	}
	imageData := &ImageData{BitsPerComponent: 8, Width: uint32(width), Height: uint32(height)}
	switch out {
	case dctRGB, dctYCbCr:
		imageData.ColorSpace = &ImageColorSpace{Kind: ImageColorSpaceRGB}
	case dctLuma:
		imageData.ColorSpace = &ImageColorSpace{Kind: ImageColorSpaceGray}
	case dctYCCK, dctCMYK:
		imageData.ColorSpace = &ImageColorSpace{Kind: ImageColorSpaceCMYK}
	}
	return FilterResult{Data: decoded, ImageData: imageData}, nil
}

// withAdobeMarker returns data with an APP14 marker of Adobe after the
// marker SOI. The marker states the transform 0.
func withAdobeMarker(data []byte) []byte {
	marker := []byte{0xFF, 0xEE, 0x00, 0x0E, 'A', 'd', 'o', 'b', 'e', 0x00, 0x64, 0x00, 0x00, 0x00, 0x00, 0x00}
	out := make([]byte, 0, len(data)+len(marker))
	out = append(out, data[:2]...)
	out = append(out, marker...)
	return append(out, data[2:]...)
}

// maybePatchJpegDimensions is maybe_patch_jpeg_dimensions in hayro-syntax
// src/filter/dct.rs. It writes the size of the image dictionary into the
// frame header when the header states a larger area.
func maybePatchJpegDimensions(data []byte, imageParams *ImageDecodeParams) ([]byte, bool) {
	sofOffset, ok := findSofMarker(data)
	if !ok {
		return nil, false
	}
	heightOffset := sofOffset + 5
	widthOffset := sofOffset + 7
	if widthOffset+2 > len(data) {
		return nil, false
	}
	jpegHeight := binary.BigEndian.Uint16(data[heightOffset:])
	jpegWidth := binary.BigEndian.Uint16(data[widthOffset:])
	jpegArea := uint64(jpegWidth) * uint64(jpegHeight)
	imageArea := uint64(imageParams.Width) * uint64(imageParams.Height)
	if jpegArea <= imageArea {
		return data, true
	}
	patched := bytes.Clone(data)
	binary.BigEndian.PutUint16(patched[heightOffset:], uint16(imageParams.Height))
	binary.BigEndian.PutUint16(patched[widthOffset:], uint16(imageParams.Width))
	return patched, true
}

// findSofMarker is find_sof_marker in hayro-syntax src/filter/dct.rs. It
// returns the offset of the first frame header.
func findSofMarker(data []byte) (int, bool) {
	i := 0
	for i+1 < len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		marker := data[i+1]
		switch {
		case marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC:
			// SOF0 to SOF15, without DHT, JPG and DAC.
			return i, true
		case marker == 0xFF:
			i++
		case marker == 0xD8 || marker == 0xD9 || marker == 0x01 || marker == 0x00:
			i += 2
		default:
			if i+3 >= len(data) {
				return 0, false
			}
			segLen := int(binary.BigEndian.Uint16(data[i+2:]))
			i += 2 + segLen
		}
	}
	return 0, false
}
