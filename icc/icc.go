// Package icc converts the colors of an image to sRGB with an ICC profile or
// with the CICP code points of an image (D61).
//
// A program gives the image and the profile that its file embeds, such as
// the iCCP chunk of a PNG file or the colr box of an AVIF file. ToSRGB
// returns the image in sRGB, with the alpha of each pixel as it was. A
// program that converts several images with one profile parses the profile
// once with Parse, and calls the method ToSRGB of the Profile.
//
// The package is a wrapper over internal/moxcms, the port of a subset of the
// Rust crate moxcms by Radzivon Bartoshyk, under the BSD-3-Clause license or
// the Apache-2.0 license. LICENSE-MOXCMS.md and LICENSE-MOXCMS-APACHE.md in
// this folder are its license files. The default CMYK profile is a profile of
// Compact-ICC-Profiles, under CC0 1.0 Universal.
//
// The transforms of the port have samples of 8 bits. A transform of an
// image with samples of 16 bits keeps the alpha of 16 bits, and it converts
// the colors with 8 bits.
//
// A profile is input from a stranger. A profile that is not valid gives an
// error that wraps ivy.ErrFormat, and it never makes the package panic. A
// profile that the port cannot use gives an error that wraps
// ivy.ErrUnsupported. A profile or an image that is too large gives an error
// that wraps ivy.ErrLimit.
package icc

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/moxcms"
)

// MaxProfileSize is the size in bytes of the smallest profile that Parse
// refuses with ivy.ErrLimit. It is the limit of moxcms.
const MaxProfileSize = 10 << 20

// MaxPixels is the number of pixels of the largest image that ToSRGB
// converts. It is the default of ivy.Limits.MaxPixels.
const MaxPixels = ivy.DefaultMaxPixels

// Intent is a rendering intent of ICC.1. It chooses the table of a profile
// that maps the colors that sRGB cannot show. A matrix profile, such as the
// profile of a camera or of a display, has one mapping for each intent.
type Intent uint8

// The rendering intents of ICC.1, with their numbers in ICC.1.
const (
	// Perceptual scales all the colors into sRGB. It is the intent for a
	// photograph.
	Perceptual Intent = 0
	// RelativeColorimetric keeps the colors that sRGB can show, maps the
	// white of the profile to the white of sRGB, and clips the other colors.
	RelativeColorimetric Intent = 1
	// Saturation keeps the saturation of the colors, for a chart.
	Saturation Intent = 2
	// AbsoluteColorimetric keeps the colors that sRGB can show, with the
	// white of the profile, and clips the other colors.
	AbsoluteColorimetric Intent = 3
)

// String returns the name of the intent.
func (i Intent) String() string {
	switch i {
	case Perceptual:
		return "perceptual"
	case RelativeColorimetric:
		return "relative colorimetric"
	case Saturation:
		return "saturation"
	case AbsoluteColorimetric:
		return "absolute colorimetric"
	}
	return fmt.Sprintf("intent %d", uint8(i))
}

// space is the kind of the colors of a profile that ToSRGB converts.
type space uint8

// The kinds of colors.
const (
	// spaceNone is a profile that ToSRGB cannot convert.
	spaceNone space = iota
	// spaceGray is one channel of gray.
	spaceGray
	// spaceThree is three channels, such as RGB or Lab.
	spaceThree
	// spaceFour is four channels, such as CMYK.
	spaceFour
)

// Profile is an ICC profile that Parse read, or the profile of CICP code
// points that FromCICP made. It is safe for concurrent use.
type Profile struct {
	p     *moxcms.ColorProfile
	space space
}

// Parse reads the ICC profile in data. The Profile keeps no reference to
// data.
func Parse(data []byte) (*Profile, error) {
	if len(data) >= MaxProfileSize {
		return nil, fmt.Errorf("icc: reading a profile of %d bytes: %w", len(data), ivy.ErrLimit)
	}
	p, err := moxcms.NewFromSlice(data)
	if err != nil {
		return nil, fmt.Errorf("icc: reading the profile: %w", err)
	}
	return newProfile(p), nil
}

// newProfile returns the Profile of p.
func newProfile(p *moxcms.ColorProfile) *Profile {
	s := spaceNone
	switch p.ColorSpace {
	case moxcms.DataColorSpaceGray:
		s = spaceGray
	case moxcms.DataColorSpaceRgb, moxcms.DataColorSpaceLab, moxcms.DataColorSpaceXyz,
		moxcms.DataColorSpaceLuv, moxcms.DataColorSpaceYCbr, moxcms.DataColorSpaceYxy,
		moxcms.DataColorSpaceHsv, moxcms.DataColorSpaceHls, moxcms.DataColorSpaceCmy,
		moxcms.DataColorSpaceColor3:
		s = spaceThree
	case moxcms.DataColorSpaceCmyk, moxcms.DataColorSpaceColor4:
		s = spaceFour
	}
	return &Profile{p: p, space: s}
}

// defaultCMYK is the Profile of DefaultCMYK, parsed once.
var defaultCMYK = sync.OnceValues(func() (*Profile, error) {
	return Parse(moxcms.CmykProfile)
})

// DefaultCMYK returns a copy of the CMYK profile that ivy uses for CMYK
// colors without a profile, such as the color space DeviceCMYK of PDF or a
// JPEG 2000 image in CMYK. It is CGATS001Compat-v2-micro.icc of
// Compact-ICC-Profiles, a compact profile of CGATS TR 001 (SWOP), under CC0
// 1.0 Universal.
func DefaultCMYK() []byte {
	return bytes.Clone(moxcms.CmykProfile)
}

// ToSRGB parses profile and returns img in sRGB, as the method ToSRGB of
// Profile does. If profile is empty, ToSRGB converts an image in CMYK with
// DefaultCMYK, and it takes any other image as sRGB, so it only copies the
// pixels.
func ToSRGB(ctx context.Context, img image.Image, profile []byte, intent Intent) (image.Image, error) {
	if len(profile) != 0 {
		p, err := Parse(profile)
		if err != nil {
			return nil, err
		}
		return p.ToSRGB(ctx, img, intent)
	}
	if isCMYK(img) {
		p, err := defaultCMYK()
		if err != nil {
			return nil, err
		}
		return p.ToSRGB(ctx, img, intent)
	}
	return copySRGB(ctx, img)
}

// ToSRGB returns img in sRGB, with the colors of img in the space of p and
// the rendering intent intent. The result has the bounds of img. It is an
// *image.NRGBA64 when img has samples of more than 8 bits, such as an
// *image.Gray16, an *image.RGBA64 or an *image.NRGBA64, and an *image.NRGBA
// for any other image. The alpha of each pixel stays as it was.
//
// A gray profile takes the luma of the colors of img. A profile of three
// channels, such as RGB or Lab, takes the red, the green and the blue of
// img, as the package image gives them. A CMYK profile needs an
// *image.CMYK, or another image with the model color.CMYKModel.
//
// ToSRGB checks ctx between the rows of the image. It refuses an image of
// more than MaxPixels pixels with ivy.ErrLimit.
func (p *Profile) ToSRGB(ctx context.Context, img image.Image, intent Intent) (image.Image, error) {
	if intent > AbsoluteColorimetric {
		return nil, fmt.Errorf("icc: converting with the %s: %w", intent, ivy.ErrUnsupported)
	}
	if err := checkSize(img); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var src moxcms.Layout
	switch p.space {
	case spaceGray:
		src = moxcms.LayoutGray
	case spaceThree:
		src = moxcms.LayoutRgb
	case spaceFour:
		if !isCMYK(img) {
			return nil, fmt.Errorf("icc: converting an image of %T with a CMYK profile: %w", img, ivy.ErrUnsupported)
		}
		src = moxcms.LayoutRgba
	default:
		return nil, fmt.Errorf("icc: converting from the color space %d: %w", p.p.ColorSpace, ivy.ErrUnsupported)
	}
	if p.space != spaceFour && isCMYK(img) {
		return nil, fmt.Errorf("icc: converting an image in CMYK with a profile that is not CMYK: %w", ivy.ErrUnsupported)
	}
	options := moxcms.DefaultTransformOptions()
	options.RenderingIntent = moxcms.RenderingIntent(intent)
	t, err := p.p.CreateTransform8Bit(src, moxcms.NewSrgb(), moxcms.LayoutRgb, options)
	if err != nil {
		return nil, fmt.Errorf("icc: making the transform to sRGB: %w", err)
	}
	return convert(ctx, img, p.space, t)
}
