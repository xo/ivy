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
