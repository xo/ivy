# jpeg2000

The package `jpeg2000` decodes JPEG 2000 images: JP2 files and raw
codestreams. It is a port of the crate `hayro-jpeg2000` of
[hayro](https://github.com/LaurenzV/hayro), at the commit
`61f50b42a7f7b15764780b9ba534cba91488bb28` (D14, D16). The PDF filter
`JPXDecode` calls it.

The code is under the Apache-2.0 or MIT license of hayro, in
`LICENSE-APACHE` and `LICENSE-MIT`. The ICC profiles in `assets/` are under
CC0 1.0 Universal, as `assets/README.md` and `assets/LICENSE.txt` say.

## The files

Each Rust file `src/a/b.rs` is the Go file `a_b.go`. Each test file
`tests/x.rs` is the Go file `tests_x_test.go`.

| Upstream file | Lines | Go file | Lines |
| --- | ---: | --- | ---: |
| `src/lib.rs` | 823 | `lib.go` | 804 |
| `src/error.rs` | 308 | `error.go` | 201 |
| `src/cancel.rs` | 73 | `cancel.go` | 92 |
| `src/sync.rs` | 5 | left out | |
| `src/math.rs` | 631 | `math.go` | 210 |
| `src/reader.rs` | 175 | `reader.go` | 212 |
| `src/integration.rs` | 308 | `integration.go` | 399 |
| `src/j2c/mod.rs` | 129 | `j2c_mod.go` | 133 |
| `src/j2c/arithmetic_decoder.rs` | 263 | `j2c_arithmetic_decoder.go` | 237 |
| `src/j2c/bitplane.rs` | 1496 | `j2c_bitplane.go` | 1247 |
| `src/j2c/build.rs` | 301 | `j2c_build.go` | 315 |
| `src/j2c/codestream.rs` | 978 | `j2c_codestream.go` | 1066 |
| `src/j2c/decode.rs` | 506 | `j2c_decode.go` | 460 |
| `src/j2c/idwt.rs` | 686 | `j2c_idwt.go` | 435 |
| `src/j2c/mct.rs` | 109 | `j2c_mct.go` | 65 |
| `src/j2c/progression.rs` | 364 | `j2c_progression.go` | 350 |
| `src/j2c/rect.rs` | 47 | `j2c_rect.go` | 47 |
| `src/j2c/segment.rs` | 292 | `j2c_segment.go` | 318 |
| `src/j2c/tag_tree.rs` | 189 | `j2c_tag_tree.go` | 161 |
| `src/j2c/tile.rs` | 879 | `j2c_tile.go`, `j2c_tile_test.go` | 611, 101 |
| `src/jp2/mod.rs` | 133 | `jp2_mod.go` | 131 |
| `src/jp2/box.rs` | 86 | `jp2_box.go` | 99 |
| `src/jp2/cdef.rs` | 86 | `jp2_cdef.go` | 106 |
| `src/jp2/cmap.rs` | 50 | `jp2_cmap.go` | 61 |
| `src/jp2/colr.rs` | 131 | `jp2_colr.go` | 207 |
| `src/jp2/icc.rs` | 81 | `jp2_icc.go` | 99 |
| `src/jp2/pclr.rs` | 76 | `jp2_pclr.go` | 81 |
| `tests/main.rs` | 230 | `tests_main_test.go` | 279 |
| the other 13 files of `tests/` | 1391 | the other 13 files `tests_*_test.go` | 869 |
| `examples/png.rs` | 18 | left out | |
| none | | `port_test.go` | 199 |

`port_test.go` holds the tests of the parts that the port adds, and the fuzz
test `FuzzDecode`.

## The test files

`testdata/streams` and `testdata/references` are copies of `tests/streams`
and `tests/references` of upstream. Together they hold 3.8 MB, so the port
holds all of them. Each test of upstream is a subtest of the same name. Nine
tests of `tests/integration.rs` need moxcms to convert an ICC profile to
sRGB. They skip, because this package does not import moxcms.

## What the port replaces

1. The integration with the crate `image` of `src/integration.rs` is part of
   the library, because the default features of the crate enable it. The
   port integrates with the package `image` of Go. `DecodeImage` returns an
   `image.Image`, and `RegisterDecodingHook` registers the formats `jp2` and
   `j2c` with `image.RegisterFormat`.
2. moxcms converts the colors in `src/integration.rs`. The port does not
   import `internal/moxcms`. The caller passes an `ICCTransform`, a function
   that converts samples of 8 bits from an ICC profile to sRGB. It replaces
   `ColorProfile::new_from_slice`, `ColorProfile::new_srgb`,
   `ColorProfile::create_transform_8bit` and the method `transform` of
   moxcms. Without a transform, the port decodes without color management.
   It converts CMYK with the naive formula.
3. fearless_simd and the type `f32x8` are left out. The port runs the scalar
   arithmetic of upstream. The inverse wavelet transforms, the multiple
   component transformation and the conversions of color are scalar loops
   in functions of their own, for SIMD later (D9). `mulAdd` never fuses the
   multiplication and the addition, so the result is the same on each
   architecture.
4. Each Rust enum with data becomes a struct, such as `ColorSpace`,
   `tilePart` and `colrColorSpace`. The nested error enums become one
   `*DecodeError`, with an `ErrorKind`, that wraps `ivy.ErrFormat`,
   `ivy.ErrUnsupported`, `ivy.ErrLimit` or `context.Canceled`.
5. `CancelHandle` holds a `CancelCheck` instead of an `Arc`. `src/sync.rs`
   only selects the `Arc`, so it is left out.

## What the port adds

1. `DecodeSettings.Limits` bounds the size of the image, the memory of the
   components and of the buffers, the number of tiles, and the storage of
   the precincts and code-blocks of a tile. Upstream has no limits.
2. Where upstream panics on an assertion or on an index, the port returns an
   error. A net in each public function also turns a runtime panic into an
   error that wraps `ivy.ErrFormat`. The fuzz test fails on such an error.
3. `TagNode::build` of upstream builds the subtrees without area before it
   discards them. A file can make that take billions of calls. The port does not
   build them, and the result is the same.

## What the port keeps

The port keeps the arithmetic of a build for release of upstream: the
integers wrap, a shift takes its amount modulo the width, and a conversion
of a float to an integer saturates. It also keeps the faults that the port
found. For example, `BitPlaneDecodeContext::reset` never sets the field
`strict`, so the bypass decoder and the check of the segmentation symbols
are always lenient.

## What the port leaves out

1. The method `original_color_type` and the function `orig_bits_per_pixel`
   of `src/integration.rs`, because the package `image` of Go has no
   `ExtendedColorType`.
2. `markers::to_string`, `tag_to_string`, `DecodingError::UnexpectedEof` and
   the methods `exchange_lps` and `exchange_mps`, because upstream never
   calls them.
3. `examples/png.rs`. `DecodeImage` and `image/png` do the same.
