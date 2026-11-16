# interpret

The package `interpret` is a port of the crate `hayro-interpret` of
[hayro](https://github.com/LaurenzV/hayro) (D14, D16). It runs the content
stream of a page and sends each mark to an abstract `Device`: a path, a run
of glyphs, an image, a clip, a transparency group. It draws nothing itself.
The port of the crate `hayro` implements a `Device` that draws with okage.

The port starts from the commit `61f50b42a7f7b15764780b9ba534cba91488bb28`
of hayro. The crate is under Apache-2.0 or MIT. `LICENSE-APACHE` and
`LICENSE-MIT` in this folder are the license files of hayro, byte for byte.
`assets/` holds the two ICC profiles of the crate, under CC0 1.0 Universal,
as `assets/README.md` and `assets/CGATS_LICENSE.txt` say.

## The files

Each Rust file `src/a/b.rs` becomes the Go file `a_b.go`. The tests of a
Rust file are in the Go test file of the same name.

| Upstream file | Lines | Go file | Lines |
| --- | ---: | --- | ---: |
| `src/lib.rs` | 64 | `lib.go` | 23 |
| `src/cache.rs` | 192 | `cache.go` | 142 |
| `src/canvas.rs` | 114 | `canvas.go` | 144 |
| `src/color/mod.rs` | 694 | `color_mod.go`, `color_mod_test.go` | 713, 73 |
| `src/color/cal_gray.rs` | 74 | `color_cal_gray.go` | 84 |
| `src/color/cal_rgb.rs` | 192 | `color_cal_rgb.go` | 192 |
| `src/color/device_cmyk.rs` | 20 | `color_device_cmyk.go` | 54 |
| `src/color/device_gray.rs` | 20 | `color_device_gray.go` | 35 |
| `src/color/device_n.rs` | 146 | `color_device_n.go` | 167 |
| `src/color/device_rgb.rs` | 16 | `color_device_rgb.go` | 31 |
| `src/color/icc.rs` | 175 | `color_icc.go` | 159 |
| `src/color/indexed.rs` | 117 | `color_indexed.go` | 142 |
| `src/color/lab.rs` | 43 | `color_lab.go` | 64 |
| `src/color/pattern.rs` | 38 | `color_pattern.go` | 55 |
| `src/color/separation.rs` | 74 | `color_separation.go` | 95 |
| `src/context.rs` | 432 | `context.go` | 442 |
| `src/convert.rs` | 30 | `convert.go` | 44 |
| `src/device.rs` | 72 | `device.go` | 135 |
| `src/encode.rs` | 428 | `encode.go` | 370 |
| `src/font/mod.rs` | 165 | `font_mod.go` | 161 |
| `src/font/type3.rs` | 142 | `font_type3.go` | 125 |
| `src/function/mod.rs` | 319 | `function_mod.go`, `function_mod_test.go` | 319, 70 |
| `src/function/type0.rs` | 175 | `function_type0.go`, `function_type0_test.go` | 171, 26 |
| `src/function/type2.rs` | 145 | `function_type2.go`, `function_type2_test.go` | 59, 102 |
| `src/function/type3.rs` | 165 | `function_type3.go`, `function_type3_test.go` | 108, 46 |
| `src/function/type4.rs` | 778 | `function_type4.go`, `function_type4_test.go` | 537, 354 |
| `src/function_shading.rs` | 179 | `function_shading.go` | 167 |
| `src/gradient.rs` | 321 | `gradient.go` | 324 |
| `src/interpret/mod.rs` | 674 | `interpret_mod.go` | 743 |
| `src/interpret/path.rs` | 123 | `interpret_path.go` | 117 |
| `src/interpret/stack.rs` | 182 | `interpret_stack.go` | 254 |
| `src/interpret/state.rs` | 403 | `interpret_state.go` | 443 |
| `src/interpret/text.rs` | 176 | `interpret_text.go` | 166 |
| `src/mesh.rs` | 387 | `mesh.go` | 414 |
| `src/ocg.rs` | 170 | `ocg.go` | 223 |
| `src/pattern.rs` | 337 | `pattern.go` | 301 |
| `src/shading.rs` | 784 | `shading.go`, `shading_test.go` | 760, 20 |
| `src/types.rs` | 349 | `types.go` | 375 |
| `src/util.rs` | 183 | `util.go` | 336 |
| `src/x_object/mod.rs` | 74 | `x_object_mod.go` | 66 |
| `src/x_object/form.rs` | 133 | `x_object_form.go` | 125 |
| `src/x_object/image.rs` | 243 | `x_object_image.go` | 209 |
| `src/x_object/soft_mask.rs` | 166 | `x_object_soft_mask.go` | 175 |
| `src/x_object/decode/mod.rs` | 386 | `x_object_decode_mod.go` | 365 |
| `src/x_object/decode/image.rs` | 310 | `x_object_decode_image.go` | 320 |
| `src/x_object/decode/mask.rs` | 141 | `x_object_decode_mask.go` | 144 |
| `examples/extract_html.rs`, `examples/extract_images.rs` | 262 | not ported. They are programs, not part of the crate | |
| `assets/*` | | `assets/`, byte for byte | |
| none | | `decoders.go`: the hooks of JBIG2 and JPX, see below | 145 |
| none | | `thumbnail.go`: `NewRasterImage`, the image of a stream that no page draws, such as a thumbnail | 25 |
| none | | `port_test.go`: the tests of the port and the fuzz test | 458 |

`util.go` also holds the helpers that give Go the arithmetic of Rust, such
as the casts `as u8` that saturate and `f32::total_cmp`.

`testdata/` holds 63 files of `tests/pdfs/custom` of hayro, byte for byte.
They cover shadings of each type, tiling patterns, soft masks, glyphs of
Type 3, annotations, images with JPX, JBIG2, CCITT and JPEG, and the color
spaces.

## The API for a renderer

A renderer implements `Device`, and it can also implement `ClipRectPusher`,
`RectDrawer` and `MarkedContentDevice`. It interprets a page like this:

```go
cache := interpret.NewInterpreterCache()
settings := interpret.DefaultInterpreterSettings()
ctx := interpret.NewContext(transform, deviceBox, cache, pdf.XRef(), settings)
err := interpret.InterpretPage(page, ctx, device)
```

The device draws the paint of `DrawProps`: a `Color`, a `*ShadingPattern`
or a `*TilingPattern`. It samples a shading with `ShadingPattern.Encode`,
then `EncodedShadingPattern.SampleTexture`, `RenderTexture`, or
`EncodedRadialAxialShading.AsSVGGradient`. It draws a tiling pattern with
`TilingPattern.Interpret`, a glyph of Type 3 with `Type3Glyph.Interpret`,
and a soft mask with `SoftMask.Interpret`. It decodes an image with
`RasterImage.WithRGBA` or `StencilImage.WithStencil`. A `GlyphRun` is valid
only during the call that receives it.

## The hooks of JBIG2 and JPX

`pdf/internal/syntax` cannot import the decoders of JBIG2 and JPX. The
function `ImageDecoders` returns a `syntax.ImageDecoders` with
`internal/jbig2` and `internal/jpeg2000`, with the settings that
`hayro-syntax` uses. `DefaultInterpreterSettings` puts it in
`InterpreterSettings.Decoders`, and the decoding of each image passes it in
`syntax.ImageDecodeParams.Decoders`. A caller that wants to stop a long
decoding makes the decoders with its own cancel function, and sets them.

The decoder of JPX does not apply the ICC profile of the image, because
`hayro-syntax` does not. So `jpeg2000.MoxcmsTransform` is not used.

## The shape of the Go code

1. A Rust enum with data becomes an interface that only its cases
   implement, where a device tells the cases apart: `Paint`, `Pattern`,
   `Image`, `Glyph`, `DrawMode`, `ShadingType`, `EncodedShadingType` and
   `SVGGradientKind`. An enum that the package reads itself becomes a
   struct with one field for each case, such as `ImageData`,
   `ShadingFunction` and `ActiveTransferFunction`.
2. A Rust `Option` becomes a nil pointer, a nil slice, or a second result
   of type `bool`. A size of an image of zero stands for `None`, as in
   `syntax.ImageDecodeParams`.
3. A value that upstream shares through an `Arc` or an `Rc` becomes a
   pointer. A lazy table of upstream becomes a `sync.Once`.
4. The trait `Device` has default methods in upstream. Go has none, so the
   default methods are the optional interfaces `ClipRectPusher`,
   `RectDrawer` and `MarkedContentDevice`.
5. The traits `Float32Ext` and `Float64Ext` become generic functions such
   as `IsNearlyZero`. `RectExt` and `TransformExt` become `RectToKurbo` and
   `TransformToKurbo`.
6. The cache keys of type `u128` are `Key128`, the 16 bytes of FNV-1a of
   pdffont. Upstream uses SipHash, so the keys differ, but they play the
   same role. `ClipPath.CacheKey` hashes the elements of the path, where
   upstream hashes its SVG text. Go cannot add a method to a type of another
   package, so the implementations of `CacheKey` for the types of syntax,
   pdffont and kurbo are functions, such as `GlyphIdentifierCacheKey`. Only
   those that the package uses are ported.
7. The geometry is `github.com/xo/okage/kurbo` and `pdf/internal/kurboext`.
8. The math of f32 runs through the package math in f64, so the last bit
   of a result can differ from upstream.

## The guards of D18

Each guard is marked with `Port:` in the code.

1. `InterpreterSettings.Limits` bounds the work of a page. A page runs at
   most 64 operators for each object of `MaxObjects`, which is 64 million
   by default. The nested forms, patterns, glyphs of Type 3 and soft masks
   share the count. Each glyph counts, and each form counts 1 for each 64
   bytes of its content, because each `Do` decodes it again. Past the
   limit, the page stops with an error that wraps `ivy.ErrLimit`.
2. `InterpreterSettings.Cancel` stops a page with `ErrCanceled`. The
   interpreter calls it every 1024 operators, and the drawing of a shading
   calls it every 1024 pieces or rows.
3. A glyph of Type 3 and a tiling pattern stop at the nesting depth of
   forms, 50. Upstream does not bound them, so one that draws itself
   recurses without end.
4. A function of type 3 reads at most 65,536 nested functions. An array of
   functions that names one object twice grows exponentially with the
   depth.
5. A function of type 0 holds at most 2^26 samples.
6. A mesh holds at most 2^20 triangles or patches, and a mesh with more
   than 32 colors fails.
7. The decoding of an image fails past `MaxPixels` pixels, or past
   `MaxMemory` bytes of samples. Upstream allocates the size that the
   dictionary states.
8. A texture of a shading holds at most 2^24 pixels. A larger one is
   empty.
9. A glyph of Type 3 keeps the graphics state with the default text state,
   because its content resets the text state. Upstream copies the paths of
   the clip of the text once for each glyph.
10. `InterpretPage`, `Type3Glyph.Interpret`, `TilingPattern.Interpret` and
    `SoftMask.Interpret` turn a runtime panic into a `PanicError`, which
    wraps `ivy.ErrFormat`. The fuzz test fails on it.
11. A code of a font of length 0 counts as length 1, so that the loop of
    the text ends.

## The other differences from upstream

1. `InterpretPage` returns an error, and `Context.Err` tells why an
   interpretation stopped. Upstream cannot fail.
2. `InterpreterSettings` has the fields `Cancel`, `Limits` and
   `Decoders`. `DefaultInterpreterSettings` sets the font resolver of the
   default features of upstream, `DefaultFontResolver`, and the predefined
   CMaps.
3. The image of a device holds its decoders and its limits with the
   warning sink.
4. The tag of `MarkedContentDevice.BeginMarkedContent` is a `syntax.Name`,
   where upstream gives the bytes.
5. A zero `ColorSpace` acts as DeviceGray. Where upstream panics on two
   buffers of different lengths, a conversion uses the shorter length.
6. The decoded data of an image is copied before the port changes it,
   because it can be the bytes of the file itself.
7. `TilingPattern.Interpret` returns false where upstream returns `None`.

## What the port leaves out

1. The examples of upstream, which are programs.
2. The features `embed-fonts` and `embed-cmaps`. The port always has the
   embedded fonts of pdffont and the CMaps of cmap.
3. The exports of the crates `hayro_cmap`, `hayro_syntax` and `kurbo`. A Go
   program imports those packages itself.

## Open problems

1. The ledger `docs/upstream/hayro.tsv` does not list this package yet.
2. No test compares the marks of a page with those of hayro. The tests
   interpret the test files and check that the clips and the groups
   balance, that each kind of mark appears, and that the images decode to
   their sizes.
3. JPEG goes through `image/jpeg`, as D17 says, so a truncated JPEG fails.
