# jbig2

The package `jbig2` decodes JBIG2 images (ITU-T T.88). It is a port of the
Rust crate `hayro-jbig2` of [hayro](https://github.com/LaurenzV/hayro), at the
commit `61f50b42a7f7b15764780b9ba534cba91488bb28`. The PDF filter
`JBIG2Decode` uses it (D14, D16).

The code is under the Apache-2.0 or MIT license of hayro. The files
`LICENSE-APACHE` and `LICENSE-MIT` are copies of the licenses of hayro.

## The API for a PDF filter

A PDF reader decodes the stream `JBIG2Globals` with `NewGlobals`, and the
data of the filter with `NewEmbeddedPage`. Then it calls `Decode`:

```go
globals, err := jbig2.NewGlobals(globalsData, jbig2.DecodeSettings{})
page, err := jbig2.NewEmbeddedPage(data, globals, jbig2.DecodeSettings{})
bitmap, err := page.Decode()
```

The globals can be nil. `PageBitmap` gives the rows as words of 32 pixels,
with black as 1, or `DataU8` gives 8-bit samples, with black as 0.
`DecodeSettings.MemoryLimit` bounds the memory of a page. Its zero value is
1 GiB. `DecodeSettings.Cancel` stops the decoder. Each error of the package
unwraps to `ivy.ErrFormat`, `ivy.ErrLimit`, `ivy.ErrUnsupported` or
`context.Canceled`.

## The files

Each Go file ports one Rust file, and it names that file in its first
comment. A difference from upstream has a comment that starts with `Port:`.

| Upstream file | Go file |
| --- | --- |
| `hayro-jbig2/src/lib.rs` | `lib.go` |
| `hayro-jbig2/src/arithmetic_decoder.rs` | `arithmetic_decoder.go` |
| `hayro-jbig2/src/bitmap.rs` | `bitmap.go`, `bitmap_test.go` |
| `hayro-jbig2/src/cancel.rs` | `cancel.go` |
| `hayro-jbig2/src/error.rs` | `error.go` |
| `hayro-jbig2/src/file.rs` | `file.go` |
| `hayro-jbig2/src/gray_scale.rs` | `gray_scale.go` |
| `hayro-jbig2/src/huffman_table.rs` | `huffman_table.go` |
| `hayro-jbig2/src/huffman_tables_generated.rs` | `huffman_tables_generated.go` |
| `hayro-jbig2/src/integer_decoder.rs` | `integer_decoder.go` |
| `hayro-jbig2/src/integration.rs` | `integration.go` |
| `hayro-jbig2/src/memory.rs` | `memory.go`, `memory_test.go` |
| `hayro-jbig2/src/page_info.rs` | `page_info.go` |
| `hayro-jbig2/src/reader.rs` | `reader.go` |
| `hayro-jbig2/src/segment.rs` | `segment.go`, `segment_test.go` |
| `hayro-jbig2/src/symbol_id_decoder.rs` | `symbol_id_decoder.go` |
| `hayro-jbig2/src/decode/mod.rs` | `decode_mod.go` |
| `hayro-jbig2/src/decode/generic.rs` | `decode_generic.go` |
| `hayro-jbig2/src/decode/generic_refinement.rs` | `decode_generic_refinement.go` |
| `hayro-jbig2/src/decode/halftone.rs` | `decode_halftone.go` |
| `hayro-jbig2/src/decode/pattern.rs` | `decode_pattern.go` |
| `hayro-jbig2/src/decode/symbol.rs` | `decode_symbol.go` |
| `hayro-jbig2/src/decode/text.rs` | `decode_text.go` |
| `hayro-jbig2/tests/main.rs` | `tests_main_test.go` |
| `hayro-jbig2/tests/<name>.rs` | `tests_<name>_test.go` |

The file `jbig2_test.go` is not a port. It tests the embedded pages and the
globals, the 8-bit samples, the errors and the image integration. It also
holds the fuzz tests `FuzzFile` and `FuzzEmbeddedPage`.

## What the port replaces or leaves out

1. `hayro-ccitt`: the MMR coding of `src/decode/generic.rs` calls the port
   of `hayro-ccitt` in `internal/ccitt` (D17). The package holds no copy of
   its own.
2. `fearless_simd` and `src/simd.rs`: the port uses scalar loops in
   `gray_scale.go`.
3. `src/lazy.rs` and `src/sync.rs`: the standard Huffman tables are package
   variables, a `*huffmanTable` replaces the `Arc`, and a mutex replaces
   the `OnceLock` of the lazy globals.
4. The crate `image`: the library uses it only in `src/integration.rs`,
   behind the feature `image`, and the tests use it to read PNG files.
   `integration.go` uses the package `image` of Go instead, and the tests
   use `image/png`.
5. `examples/jbig2png.rs` and `generate_huffman_tables.py`: not ported.
   `huffman_tables_generated.go` is a conversion of the generated Rust file.
6. `CancelHandle`: a `CancelCheck` interface value takes its place. The
   type `CancelFunc` makes a check from a function.
7. `PageBitmap.DataU8` also returns `ErrMemoryLimitExceeded` when the
   samples do not fit in the memory limit. Upstream does not bound it.

## The test data

`testdata/streams` and `testdata/references` hold all 187 streams and the
4 reference images of `hayro-jbig2/tests`, about 0.8 MB. The streams of
`hayro-ccitt/tests` are in `internal/ccitt/testdata`.
