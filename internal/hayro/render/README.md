# render

The package `render` is a port of the crate `hayro` of
[hayro](https://github.com/LaurenzV/hayro) (D14, D16). It implements the
`Device` of `internal/hayro/interpret`, and draws each mark of a page onto a
`display.Device` of `internal/display` (D10). Upstream draws with vello_cpu.
Where upstream calls vello_cpu, the port sends a command to the display
device.

The port starts from the commit `61f50b42a7f7b15764780b9ba534cba91488bb28`
of hayro. The crate is under Apache-2.0 or MIT. `LICENSE-APACHE` and
`LICENSE-MIT` in this folder are the license files of hayro, byte for byte.

## The files

| Upstream file | Lines | Go file | Lines |
| --- | ---: | --- | ---: |
| `src/lib.rs` | 420 | `lib.go` | 326 |
| `src/clip.rs` | 29 | `clip.go` | 43 |
| `src/glyph.rs` | 330 | `glyph.go` | 233 |
| `src/glyph_cache.rs` | 182 | not ported, see below | |
| `src/image.rs` | 812 | `image.go` | 364 |
| `src/mask.rs` | 117 | `mask.go` | 65 |
| `src/paint.rs` | 144 | `paint.go` | 161 |
| `src/path.rs` | 207 | `path.go` | 216 |
| `src/tiling.rs` | 121 | `tiling.go` | 182 |
| none | | `convert.go`: the geometry of kurbo as the geometry of okage | 98 |
| none | | `port_test.go`: the tests of the port | 205 |

## How the port draws

Upstream keeps the state of the drawing in a `RenderContext` of vello_cpu:
a transform, a paint, a fill rule, a stroke, a blend mode and a mask. The
display device holds only a transform and a clip. So the port draws each
mark like this:

1. The user space of the display device is the space of the pixels of the
   target. Each drawing saves the state, concatenates its transform, draws,
   and restores the state.
2. A clip of upstream is a `Save` and a `ClipPath` in the space of the
   pixels. Its pop is a `Restore`.
3. A paint of upstream becomes a shader of okage in the user space of the
   drawing: a solid color, a gradient of okage, or a pattern of okage for
   the texture of a shading and for the cell of a tiling pattern.
4. The soft mask and the blend mode of one drawing become a group of the
   display device around the drawing. A transparency group of the
   interpreter becomes a group with its opacity, its blend mode, its
   isolation, its knockout and its soft mask.
5. A soft mask becomes a `display.Mask`. Its content is a function that
   runs `SoftMask.Interpret` onto the device that the display device gives
   it, and its key is the cache key of the mask. The display device keeps
   the mask that it drew for a key, where upstream keeps a cache in each
   renderer.
6. A stencil image becomes `FillMask` with the shader of its paint. An
   image becomes `DrawImage` of an image of RGBA, premultiplied.

## The other differences from upstream

1. `Render` and `RenderInto` return the error of the interpretation.
   `RenderInto` draws onto a display device of a size in pixels. `Render`
   returns nil for a page whose pixmap cannot be made.
2. The port does not hint glyphs (D19). It has no cache of glyph bitmaps,
   so `src/glyph_cache.rs` is not ported. okage fills each outline. The
   `RenderCache` keeps the outline of each glyph as a path of okage.
3. okage always anti-aliases, so `RenderSettings` has no aliasing
   threshold.
4. Upstream resizes images with the crate pic_scale. The port resizes with
   `golang.org/x/image/draw`, with the same kernel of Hermite. Upstream
   resizes the channels of luma, RGB and alpha apart. The port resizes the
   image of RGBA, premultiplied, which gives the same pixels.
5. An image whose alpha channel has another size or another interpolation
   draws in a group with a soft mask of type alpha. Upstream draws the alpha
   in a layer that composes with DestIn.
6. A pattern of okage has one mode of repetition for both directions. The
   cell of a tiling pattern covers the whole area in a direction that pads,
   so the port repeats when one direction repeats.
7. A dash array of odd length repeats once, because okage takes an array
   of even length, and PDF reads an odd array twice. A dash array that okage
   refuses draws no dashes.
8. A drawing culls by its bounds before it begins the group of its soft
   mask and its blend mode. Upstream sets them first.
9. A paint of a pattern in a space without an inverse draws nothing.
10. The display device draws every group as isolated, and a knockout group
    as a group that is not knockout. Upstream ignores both flags.

## The guards of D18

Each guard is marked with `Port:` in the code.

1. The pixmap of a cell of a tiling pattern is at most 16,384 pixels wide
   and high, and at most 2^22 pixels in all. A larger cell draws at a lower
   resolution. Upstream stores the size in a u16, which saturates.
2. The pixmap of a cell draws at most 256 copies of the cell in each
   direction, and 4,096 in all. The copies nearest to the end of each range
   stay. A cell that is much larger than its step needs many copies, and
   each copy runs the content of the cell.
3. The cache of outlines holds at most 65,536 glyphs. Past it, the cache
   starts again empty.
4. The data of an image that is shorter than its size draws nothing. A
   texture of a shading that interpret does not sample, because it is too
   large, draws nothing.
5. A panic in a call of the interpreter to the renderer stops the page with
   a `PanicError` of interpret, because `InterpretPage` recovers it.

## Open problems

1. The ledger `docs/upstream/hayro.tsv` does not list this package yet.
2. No test compares the renders with those of hayro. The tests render the
   test files of interpret and syntax, check that the saves and the groups
   balance and that each kind of command appears, and check the pixels of
   a few simple pages.
3. A group that is not isolated draws as an isolated one. A blend mode
   inside such a group blends with the transparent layer, not with the
   backdrop of the page.
