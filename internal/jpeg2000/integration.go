// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/integration.rs.
//
// Port: upstream integrates the decoder with the crate image, and converts
// colors with moxcms. The port integrates it with the package image of Go.
// ivy ports moxcms in internal/moxcms. This file takes only the CMYK profile
// from it, and a caller passes an ICCTransform for the conversions. Without
// one, the port decodes without color management. The method
// original_color_type and the function orig_bits_per_pixel are left out,
// because the package image of Go has no ExtendedColorType.

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/moxcms"
)

// cmykProfile is CMYK_PROFILE: the CMYK profile of upstream, under CC0 1.0
// Universal.
//
// Port: the port of moxcms holds the file, byte for byte, because
// hayro-interpret and the package icc use it too (D61).
var cmykProfile = moxcms.CmykProfile

// ColorType is ColorType of the crate image, for the four types that the
// integration gives. Each sample has 8 bits.
type ColorType uint8

// The color types.
const (
	// ColorTypeL8 is ColorType::L8: gray.
	ColorTypeL8 ColorType = iota
	// ColorTypeLa8 is ColorType::La8: gray and alpha.
	ColorTypeLa8
	// ColorTypeRgb8 is ColorType::Rgb8: red, green and blue.
	ColorTypeRgb8
	// ColorTypeRgba8 is ColorType::Rgba8: red, green, blue and alpha.
	ColorTypeRgba8
)

// ChannelCount is ColorType::channel_count. It returns the number of
// channels of a pixel.
func (t ColorType) ChannelCount() int {
	switch t {
	case ColorTypeL8:
		return 1
	case ColorTypeLa8:
		return 2
	case ColorTypeRgb8:
		return 3
	}
	return 4
}

// Layout is Layout of moxcms: the order of the channels of one pixel of 8
// bits.
type Layout uint8

// The layouts that the integration uses.
const (
	// LayoutGray is Layout::Gray.
	LayoutGray Layout = iota
	// LayoutGrayAlpha is Layout::GrayAlpha.
	LayoutGrayAlpha
	// LayoutRGB is Layout::Rgb.
	LayoutRGB
	// LayoutRGBA is Layout::Rgba. moxcms also reads four channels of CMYK
	// through this layout.
	LayoutRGBA
)

// ICCTransform converts samples of 8 bits from the color space of an ICC
// profile to sRGB. It reads src in srcLayout, and writes dst in dstLayout.
// An alpha channel passes through unchanged.
//
// It replaces these calls of moxcms in hayro-jpeg2000 src/integration.rs:
// ColorProfile::new_from_slice on the profile, ColorProfile::new_srgb,
// ColorProfile::create_transform_8bit with TransformOptions::default(), and
// the method transform of the result. A program that has the port of
// moxcms passes a function that makes those calls. An error makes the
// decoder fall back, as the error of moxcms does upstream.
type ICCTransform func(profile, src []byte, srcLayout Layout, dst []byte, dstLayout Layout) error

// errNoTransform is the error of toSRGB without an ICCTransform.
var errNoTransform = newError(KindColor, "no ICC transform", ivy.ErrUnsupported)

// errUnsupportedColor is the error unsupported_color_error in
// hayro-jpeg2000 src/integration.rs.
var errUnsupportedColor = newError(KindColor, "unsupported color type", ivy.ErrUnsupported)

// Dimensions is ImageDecoder::dimensions for Image. It returns the width
// and the height.
func (img *Image) Dimensions() (width, height uint32) {
	return img.Width(), img.Height()
}

// ColorType is ImageDecoder::color_type for Image. It returns the type of
// the pixels that ReadImage writes. ReadImage converts CMYK to RGB.
func (img *Image) ColorType() ColorType {
	switch n, a := img.colorSpace.NumChannels(), img.hasAlpha; {
	case n == 1 && !a:
		return ColorTypeL8
	case n == 1:
		return ColorTypeLa8
	case (n == 3 || n == 4) && a:
		return ColorTypeRgba8
	}
	return ColorTypeRgb8
}

// TotalBytes is ImageDecoder::total_bytes for Image. It returns the size of
// the buffer of ReadImage.
func (img *Image) TotalBytes() uint64 {
	return uint64(img.Width()) * uint64(img.Height()) * uint64(img.ColorType().ChannelCount())
}

// ICCProfile is ImageDecoder::icc_profile for Image, from the function
// icc_profile. It returns the ICC profile of the samples that ReadImage
// writes, or nil for sRGB. ReadImage converts the samples of another
// profile to sRGB.
func (img *Image) ICCProfile() []byte {
	if o := outputOf(img); o.kind == outputUnchanged && o.profile != nil {
		return append([]byte(nil), o.profile...)
	}
	return nil
}

// ReadImage is ImageDecoder::read_image for Image, with the function
// convert_inner. It decodes the image into buf, which holds TotalBytes
// bytes, in the ColorType of the image. toSRGB converts the colors of the
// profiles that sRGB cannot hold unchanged, such as CMYK. A nil toSRGB
// decodes without color management.
func (img *Image) ReadImage(buf []byte, toSRGB ICCTransform) (err error) {
	defer recoverPanic(&err)
	var ctx DecoderContext

	o := outputOf(img)
	switch o.kind {
	case outputUnchanged:
		d, err := img.Decode(&ctx)
		if err != nil {
			return err
		}
		return d.StoreU8Into(buf)
	case outputConverted:
		d, err := img.Decode(&ctx)
		if err != nil {
			return err
		}
		decoded, err := d.DataU8()
		if err != nil {
			return err
		}
		if toSRGBInto(img, o.from, decoded, buf, toSRGB) == nil {
			return nil
		}
		switch img.colorSpace.NumChannels() {
		case 3:
			copy(buf, decoded)
			return nil
		case 4:
			if toSRGB == nil {
				// Port: without color management, the port converts CMYK
				// to RGB with the naive formula.
				naiveCMYKToRGB(img.hasAlpha, decoded, buf)
				return nil
			}
			if err := toSRGBInto(img, cmykProfile, decoded, buf, toSRGB); err != nil {
				return fmt.Errorf("jpeg2000: converting to sRGB: %w: %w", ivy.ErrUnsupported, err)
			}
			return nil
		}
	}
	return errUnsupportedColor
}

// outputKind is the case of Output in hayro-jpeg2000 src/integration.rs.
type outputKind uint8

const (
	// outputUnchanged is Output::Unchanged: the samples need no
	// conversion. profile is the ICC profile of the samples, or nil for
	// sRGB.
	outputUnchanged outputKind = iota
	// outputConverted is Output::Converted: the samples need a conversion
	// from the profile from.
	outputConverted
	// outputUnsupported is Output::Unsupported.
	outputUnsupported
)

// output is Output in hayro-jpeg2000 src/integration.rs.
type output struct {
	kind    outputKind
	profile []byte
	from    []byte
}

// outputOf is output in hayro-jpeg2000 src/integration.rs.
func outputOf(img *Image) output {
	cs := img.colorSpace
	switch cs.Kind {
	case ColorSpaceGray, ColorSpaceRGB:
		return output{kind: outputUnchanged}
	case ColorSpaceCMYK:
		return output{kind: outputConverted, from: cmykProfile}
	case ColorSpaceICC:
		if m, ok := iccMetadataFromData(cs.Profile); ok && (m.colorSpace == iccGray || m.colorSpace == iccRGB) {
			return output{kind: outputUnchanged, profile: cs.Profile}
		}
		return output{kind: outputConverted, from: cs.Profile}
	}
	return output{kind: outputUnsupported}
}

// toSRGBInto is to_srgb in hayro-jpeg2000 src/integration.rs. It converts
// the decoded samples to sRGB in buf with the profile.
func toSRGBInto(img *Image, profile, decoded, buf []byte, toSRGB ICCTransform) error {
	if toSRGB == nil {
		return errNoTransform
	}
	switch n, a := img.colorSpace.NumChannels(), img.hasAlpha; {
	case n == 3 && !a:
		return toSRGB(profile, decoded, LayoutRGB, buf, LayoutRGB)
	case n == 3:
		return toSRGB(profile, decoded, LayoutRGBA, buf, LayoutRGBA)
	case n == 4 && !a:
		// moxcms reads four-channel CMYK through the RGBA layout.
		return toSRGB(profile, decoded, LayoutRGBA, buf, LayoutRGB)
	case n == 4:
		// moxcms does not support CMYK with interleaved alpha.
		pixels := len(decoded) / 5
		cmyk := make([]byte, 0, pixels*4)
		alpha := make([]byte, 0, pixels)
		for i := range pixels {
			s := decoded[i*5 : i*5+5]
			cmyk = append(cmyk, s[0], s[1], s[2], s[3])
			alpha = append(alpha, s[4])
		}
		rgb := make([]byte, pixels*3)
		if err := toSRGB(profile, cmyk, LayoutRGBA, rgb, LayoutRGB); err != nil {
			return err
		}
		for i := 0; i < pixels && i*4+4 <= len(buf); i++ {
			copy(buf[i*4:], rgb[i*3:i*3+3])
			buf[i*4+3] = alpha[i]
		}
		return nil
	}
	return errUnsupportedColor
}

// naiveCMYKToRGB converts CMYK to RGB without a profile: each channel is
// (255 - C) * (255 - K) / 255.
//
// Port: upstream always converts CMYK with moxcms.
func naiveCMYKToRGB(hasAlpha bool, decoded, buf []byte) {
	in, out := 4, 3
	if hasAlpha {
		in, out = 5, 4
	}
	for i := 0; (i+1)*in <= len(decoded) && (i+1)*out <= len(buf); i++ {
		s := decoded[i*in:]
		d := buf[i*out:]
		k := 255 - uint32(s[3])
		for c := range 3 {
			d[c] = uint8((255 - uint32(s[c])) * k / 255)
		}
		if hasAlpha {
			d[3] = s[4]
		}
	}
}

// DecodeImage is the decoding of Jp2Decoder in hayro-jpeg2000
// src/integration.rs, followed by the conversion of the test harness of
// upstream to sRGB. It decodes data into an image of the package image of
// Go: *image.Gray for gray, *image.RGBA for RGB, and *image.NRGBA for an
// image with alpha. toSRGB converts the colors of an ICC profile. A nil
// settings is DefaultDecodeSettings.
//
// Port: the crate image hands the ICC profile to the caller. The package
// image of Go cannot hold one, so DecodeImage converts the samples with toSRGB, as the
// tests of upstream do. Without toSRGB, the samples keep their profile.
func DecodeImage(data []byte, settings *DecodeSettings, toSRGB ICCTransform) (m image.Image, err error) {
	defer recoverPanic(&err)
	img, err := New(data, settings)
	if err != nil {
		return nil, err
	}
	lim := newLimits(img.settings.Limits)
	if err := lim.pixels(uint64(img.Width()) * uint64(img.Height())); err != nil {
		return nil, err
	}
	total := img.TotalBytes()
	if err := lim.memory(total); err != nil {
		return nil, err
	}
	buf := make([]byte, total)
	if err := img.ReadImage(buf, toSRGB); err != nil {
		return nil, err
	}

	ct := img.ColorType()
	if profile := img.ICCProfile(); profile != nil && toSRGB != nil {
		layout := [...]Layout{LayoutGray, LayoutGrayAlpha, LayoutRGB, LayoutRGBA}[ct]
		converted := make([]byte, len(buf))
		if err := toSRGB(profile, buf, layout, converted, layout); err != nil {
			return nil, fmt.Errorf("jpeg2000: converting to sRGB: %w: %w", ivy.ErrUnsupported, err)
		}
		buf = converted
	}
	return toGoImage(buf, ct, int(img.Width()), int(img.Height())), nil
}

// toGoImage wraps the interleaved samples in an image of the package image
// of Go.
func toGoImage(buf []byte, ct ColorType, w, h int) image.Image {
	r := image.Rect(0, 0, w, h)
	switch ct {
	case ColorTypeL8:
		return &image.Gray{Pix: buf, Stride: w, Rect: r}
	case ColorTypeRgba8:
		return &image.NRGBA{Pix: buf, Stride: 4 * w, Rect: r}
	}
	n := ct.ChannelCount()
	pix := make([]byte, 4*w*h)
	for i := range w * h {
		s := buf[i*n:]
		d := pix[i*4 : i*4+4]
		switch ct {
		case ColorTypeLa8:
			d[0], d[1], d[2], d[3] = s[0], s[0], s[0], s[1]
		default:
			d[0], d[1], d[2], d[3] = s[0], s[1], s[2], 0xff
		}
	}
	if ct == ColorTypeLa8 {
		return &image.NRGBA{Pix: pix, Stride: 4 * w, Rect: r}
	}
	return &image.RGBA{Pix: pix, Stride: 4 * w, Rect: r}
}

// decodeConfig returns the configuration of the image in data.
func decodeConfig(data []byte) (image.Config, error) {
	img, err := New(data, nil)
	if err != nil {
		return image.Config{}, err
	}
	model := color.NRGBAModel
	switch img.ColorType() {
	case ColorTypeL8:
		model = color.GrayModel
	case ColorTypeRgb8:
		model = color.RGBAModel
	}
	return image.Config{ColorModel: model, Width: int(img.Width()), Height: int(img.Height())}, nil
}

var registered sync.Once

// RegisterDecodingHook is register_decoding_hook in hayro-jpeg2000
// src/integration.rs. It registers the decoder with the package image of
// Go, as the formats "jp2" for JP2 files and "j2c" for raw codestreams. It
// returns false when it registered them already. The decoder converts
// colors with toSRGB, and it reads all of its input.
func RegisterDecodingHook(toSRGB ICCTransform) bool {
	first := false
	registered.Do(func() {
		first = true
		decode := func(r io.Reader) (image.Image, error) {
			data, err := io.ReadAll(r)
			if err != nil {
				return nil, fmt.Errorf("jpeg2000: reading: %w", err)
			}
			return DecodeImage(data, nil, toSRGB)
		}
		config := func(r io.Reader) (image.Config, error) {
			data, err := io.ReadAll(r)
			if err != nil {
				return image.Config{}, fmt.Errorf("jpeg2000: reading: %w", err)
			}
			return decodeConfig(data)
		}
		image.RegisterFormat("jp2", string(jp2Magic), decode, config)
		image.RegisterFormat("j2c", string(codestreamMagic), decode, config)
	})
	return first
}
