# D61. The package icc converts the colors of an image to sRGB

Status: Decided. Amends D16.

On 2026-11-20 the maintainers asked for a public package of color
management for iv. iv moves from libvips to decoders in pure Go, for HEIC,
AVIF, JPEG XL, WebP, JPEG 2000 and OpenEXR. libvips also converts the colors
of an image with the ICC profile of the image. No public package in pure Go
does that. ivy has the port of moxcms in `internal/moxcms` (D16), and a
program cannot import an internal package. This decision holds the API of
the package and the choices that the change made.

## The API

```go
package icc // import "github.com/xo/ivy/icc"

// MaxProfileSize is the size in bytes of the smallest profile that Parse
// refuses with ivy.ErrLimit.
const MaxProfileSize = 10 << 20

// MaxPixels is the number of pixels of the largest image that ToSRGB
// converts.
const MaxPixels = ivy.DefaultMaxPixels

// Intent is a rendering intent of ICC.1.
type Intent uint8

const (
	Perceptual           Intent = 0
	RelativeColorimetric Intent = 1
	Saturation           Intent = 2
	AbsoluteColorimetric Intent = 3
)

func (i Intent) String() string

// Profile is an ICC profile, or the profile of CICP code points. It is safe
// for concurrent use.
type Profile struct{ /* unexported */ }

func Parse(data []byte) (*Profile, error)
func (p *Profile) ToSRGB(ctx context.Context, img image.Image, intent Intent) (image.Image, error)

// ToSRGB parses profile and converts img. An empty profile converts a CMYK
// image with DefaultCMYK, and copies any other image as sRGB.
func ToSRGB(ctx context.Context, img image.Image, profile []byte, intent Intent) (image.Image, error)

// DefaultCMYK returns a copy of the CMYK profile that ivy uses for CMYK
// without a profile.
func DefaultCMYK() []byte

// CICP holds the code points of Rec. ITU-T H.273.
type CICP struct {
	ColorPrimaries          uint8
	TransferCharacteristics uint8
	MatrixCoefficients      uint8
	FullRange               bool
}

func FromCICP(c CICP) (*Profile, error)
```

## The choices

1. The name is `icc`, in the folder `icc/`. The name of a decoder is its
   format (D6), and an ICC profile is the format that the package reads. The
   package is not a format of a document. So it has no `Format`, no `Open`
   and no document (D11). It imports only the root package and
   `internal/moxcms`. `internal/moxcms` stays the implementation.
2. The package declares no sentinel error (D11). A profile that is not valid
   gives an error that wraps `ivy.ErrFormat`. A profile that the port cannot
   use, an intent past `AbsoluteColorimetric`, a CICP code point that is
   reserved or unspecified, and a profile that does not fit the image give
   an error that wraps `ivy.ErrUnsupported`. A profile of 10 MiB or more,
   the limit of moxcms, and an image of more than `ivy.DefaultMaxPixels`
   pixels give an error that wraps `ivy.ErrLimit`. When the context is done,
   the error of the context comes back as it is.
3. The functions take no `ivy.Limits`, because they take no `ivy.Config`.
   The limit of the pixels is the default of `ivy.Limits.MaxPixels`. The
   function `ToSRGB` checks the context every 64 rows.
4. The port has the transforms of 8 bits only (D16). An image with samples
   of 16 bits converts its colors with 8 bits, and it keeps its alpha of 16
   bits. The result is an `*image.NRGBA64` all the same, so that the API
   does not change when the port gets the transforms of 16 bits. Any other
   image gives an `*image.NRGBA`.
5. The alpha stays as it is. The package removes the premultiplication of
   an `*image.RGBA` or an `*image.RGBA64` before the transform, and the
   result is not premultiplied.
6. The color space of the profile chooses the samples of the image. A gray
   profile takes the luma of `color.Gray16Model`. A profile of three
   channels, such as RGB or Lab, takes the red, the green and the blue. A
   CMYK profile takes an `*image.CMYK`, or another image with the model
   `color.CMYKModel`, and refuses any other image. A CMYK image refuses a
   profile that is not CMYK.
7. Without a profile, `ToSRGB` converts a CMYK image with the default CMYK
   profile, and copies any other image as sRGB.
8. The default CMYK profile is `CGATS001Compat-v2-micro.icc` of
   Compact-ICC-Profiles, under CC0 1.0 Universal. It is the profile of the
   color space DeviceCMYK of `pdf`, and of a JPEG 2000 image in CMYK. The
   ports of hayro-interpret and of hayro-jpeg2000 each held a copy of it.
   The change moves the one copy to `internal/moxcms/assets`, with its
   license, and the two ports read it from there. `psd` uses no profile. It
   converts CMYK with the formula of `color.CMYK`.
9. moxcms makes a profile from CICP code points with
   `ColorProfile::new_from_cicp`. The port adds it, and the items of
   `cicp.rs`, `profile.rs`, `chromaticity.rs` and `trc.rs` that it needs.
   This amends item 3 of D16: the subset of moxcms also holds the items that
   the package `icc` calls. `SUBSET.md` lists them. The profile uses the
   primaries and the transfer characteristics. The matrix coefficients and
   the range belong to the conversion from YCbCr to RGB, which the codec
   makes first. moxcms has no tone mapping. So an image in PQ looks dark,
   because a linear value of 1 is 10000 cd/m2 in PQ, and the white of sRGB
   in the transform.
10. ivy reads no ICC profile that a JPEG file (APP2 `ICC_PROFILE`), a PNG
    file (`iCCP`) or a WebP file (`ICCP`) embeds. ivy reads the TIFF tag
    34675 only in the reader of TIFF of `internal/gowemf`, for the bitmaps
    of a metafile. That reader refuses a TIFF file in CMYK, in tiles or in
    JPEG before it reads the tag. So the package exports no function that
    finds an embedded profile. `docs/BACKLOG.md` holds that work.
11. The package is a wrapper over the port of moxcms. `icc/` holds the two
    license files of moxcms, byte for byte, as `LICENSE-MOXCMS.md` and
    `LICENSE-MOXCMS-APACHE.md`. `internal/moxcms/assets/LICENSE.txt` holds
    the license of the default CMYK profile.
12. A `Profile` is safe for concurrent use. Each call of `ToSRGB` makes its
    own transform, so a `Profile` holds no state of a transform.
