# oovpsd

This folder is a copy of [github.com/oov/psd](https://github.com/oov/psd) at
the commit `a5d50ec0acac`, under the MIT license in `LICENSE` (D7). The rules
for a copy are in [`docs/UPSTREAM.md`](../../../docs/UPSTREAM.md).

## The files

| File | Upstream file |
| --- | --- |
| `psd.go`, `layer.go`, `picker.go`, `imageres.go`, `util.go`, `colormode.go`, `blendmode.go`, `compress.go`, `compress_other.go` | the files of the same names in the root of upstream |
| `color/color.go` | `color/color.go` |

The copy leaves out the tests, the commands, the packages `blend`,
`composite` and `png`, and the files for JavaScript: `compress_js.go`,
`compress_wasm_js.go` and `compress.inc.js`.

## The changes

Each change is marked in the code with a comment that starts with `ivy:`.

1. The package is named `oovpsd`, after the folder.
2. `picker.go` imports the copy of the package `color`.
3. `psd.go` does not register PSD and PSB with the package `image`. The
   package `psd` of ivy registers them.
4. `compress_other.go` has no build constraint, because the copy holds no
   file for JavaScript.
5. `compress.go` counts the bytes of a line of an image of 16 or 32 bits. The
   decoder of RLE splits the lines between goroutines by that count, and
   upstream counts one byte for each pixel. A file of 16 bits that the decoder
   reads on more than one goroutine then fails as broken data.
6. gofmt indents the example in the comment of `Debug` in `psd.go`, as the
   form of a doc comment of Go 1.19 and later wants. The copy has no other
   change of format.
7. `util.go` has the function `readN`. The color mode data, the image
   resources, the compressed lines and the additional layer information read
   their bytes with it. Upstream allocates the length that the file gives
   before it reads, so a broken file of a few bytes can allocate gigabytes.
8. `DecodeOptions` has the field `MaxMemory`, and `psd.go` has the error
   `ErrLayerMemory`. `layer.go` counts the bytes of the images of the layers
   in `int64`, and stops at the limit. Upstream can overflow the size, or
   allocate a layer much larger than the image.
9. `layer.go` gives no picker to a layer of a color mode without a picker,
   or to a layer without one of the channels of its color mode. It skips a
   channel index past the channels of the color mode. Upstream panics in
   these cases.
10. `imageres.go` stops before a partial alpha identifier at the end of the
    resource, and `psd.go` reads the index of the transparent color only if
    the resource holds its 2 bytes. Upstream reads past the slices.
11. `picker.go` returns a transparent pixel for an index past the palette,
    or past the data. Upstream reads past the slices.
