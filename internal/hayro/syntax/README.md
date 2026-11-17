# syntax

The package `syntax` is a port of the crate `hayro-syntax` of
[hayro](https://github.com/LaurenzV/hayro) (D14, D16). It reads a PDF file at
a low level: the xref tables, the objects, the streams and their filters, the
encryption, the pages, and the operators of content streams.

The port starts from the commit `61f50b42a7f7b15764780b9ba534cba91488bb28`
of hayro. The crate is under Apache-2.0 or MIT. `LICENSE-APACHE` and
`LICENSE-MIT` in this folder are the license files of hayro.

## The files

Each Rust file `src/a/b.rs` becomes the Go file `a_b.go`.

| Upstream file | Go file |
| --- | --- |
| `src/lib.rs` | `lib.go` |
| `src/bit_reader.rs` | `bit_reader.go` |
| `src/byte_reader.rs` | `byte_reader.go` |
| `src/content/mod.rs` | `content_mod.go` |
| `src/content/ops.rs` | `content_ops.go` |
| `src/content/ops_generated.rs` | `content_ops_generated.go` |
| `src/content/gen_ops.py` | not ported. A script made `content_ops_generated.go` from the Rust file |
| `src/crypto/mod.rs` | `crypto_mod.go` |
| `src/crypto/aes.rs` | replaced by `crypto/aes` and `crypto/cipher` |
| `src/crypto/md5.rs` | replaced by `crypto/md5` |
| `src/crypto/rc4.rs` | replaced by `crypto/rc4` |
| `src/crypto/sha256.rs` | replaced by `crypto/sha256` |
| `src/crypto/sha384.rs`, `src/crypto/sha512.rs` | replaced by `crypto/sha512` |
| `src/data.rs` | `data.go` |
| `src/filter/mod.rs` | `filter_mod.go` |
| `src/filter/ascii_85.rs` | `filter_ascii_85.go` |
| `src/filter/ascii_hex.rs` | `filter_ascii_hex.go` |
| `src/filter/ccitt.rs` | `filter_ccitt.go` |
| `src/filter/dct.rs` | `filter_dct.go` |
| `src/filter/jbig2.rs` | `filter_jbig2.go` |
| `src/filter/jpx.rs` | `filter_jpx.go` |
| `src/filter/lzw_flate.rs` | `filter_lzw_flate.go` |
| `src/filter/png/mod.rs` | `filter_png_mod.go` |
| `src/filter/png/paeth.rs` | `filter_png_paeth.go` |
| `src/filter/predictor.rs` | `filter_predictor.go` |
| `src/filter/run_length.rs` | `filter_run_length.go` |
| `src/metadata.rs` | `metadata.go` |
| `src/object/mod.rs` | `object_mod.go` |
| `src/object/array.rs` | `object_array.go` |
| `src/object/bool.rs` | `object_bool.go` |
| `src/object/date.rs` | `object_date.go` |
| `src/object/dict.rs` | `object_dict.go` |
| `src/object/indirect.rs` | `object_indirect.go` |
| `src/object/name.rs` | `object_name.go` |
| `src/object/null.rs` | `object_null.go` |
| `src/object/number.rs` | `object_number.go` |
| `src/object/rect.rs` | `object_rect.go` |
| `src/object/ref.rs` | `object_ref.go` |
| `src/object/stream.rs` | `object_stream.go` |
| `src/object/string.rs` | `object_string.go` |
| `src/object/tuple.rs` | `object_tuple.go` |
| `src/page.rs` | `page.go` |
| `src/pdf.rs` | `pdf.go` |
| `src/reader.rs` | `reader.go` |
| `src/sync.rs` | `sync.go` |
| `src/transform.rs` | `transform.go` |
| `src/trivia.rs` | `trivia.go` |
| `src/util.rs` | `util.go` |
| `src/xref.rs` | `xref.go` |
| `examples/iterate.rs` | `TestIterate` in `pdf_test.go` |

The tests of each Rust file are in the Go test file of the same name. The
tests of `object/bool.rs` and `object/null.rs` share `object_bool_test.go`.
The tests of the filters share `filter_test.go`. `port_test.go` holds the
tests of the port that upstream does not have, and the fuzz test
`FuzzNewPdf`. `testdata/` holds four small files of `tests/pdfs` of hayro,
byte for byte.

## The shape of the Go code

1. The trait `Readable` of upstream reads a type from bytes. Go uses the
   generic methods of Go 1.27: `Reader.Read[T]`, `Dict.Get[T]`,
   `Array.Iter[T]`, `FlexArrayIter.Next[T]` and `XRef.Get[T]`. A type switch
   picks the code for each type of upstream. A type that upstream cannot
   read makes the call panic, because it is a fault of the program.
2. The Rust enum `Object` becomes the sealed interface `Object`. Its cases
   are `Null`, `Bool`, `Number`, `String`, `Name`, `Dict`, `Array` and
   `Stream`.
3. A Rust tuple becomes `Tuple2` to `Tuple5`, and a tuple struct of the
   content operators becomes a struct with the fields `V0`, `V1` and so on.
4. The type `usize` of upstream becomes `int`. A read of `int` does not take
   a negative number.
5. A Rust `Option` becomes a second result of type `bool`, or a nil pointer.
6. Each key of the module `keys` of `object/dict.rs` is a constant `Key` and
   its value, such as `KeyExtGState` for `EXT_G_STATE`. A value that is not a
   Go name takes the Rust name in MixedCaps, such as `KeyCaNs` for `CA_NS`.
7. The operator `Transform` of the content stream becomes `TransformOps`,
   because the type `Transform` of `transform.rs` has the name (okage D10).
8. `GoString` returns the derived `Debug` text of upstream, and `String`
   returns its `Display` text.
9. Each error wraps a sentinel error of ivy: `ivy.ErrFormat` for malformed
   data, `ivy.ErrPassword` for a missing or wrong password,
   `ivy.ErrUnsupported`, and `ivy.ErrLimit`.

## What the port replaces

D16 replaces the code of hayro that Go or `golang.org/x/image` holds:

1. MD5, SHA-256, SHA-384, SHA-512, AES and RC4 become the packages of
   `crypto`. The derivation of the keys and the standard security handler of
   `crypto/mod.rs` stay.
2. The inflate of `miniz_oxide` and `zlib-rs` becomes `compress/flate`. The
   output of a truncated stream counts as a success, and a corrupt stream
   gives its partial output after a second attempt with a window of zeros,
   as upstream does.
3. The LZW decoder becomes `golang.org/x/image/tiff/lzw` for the early
   change of PDF, and `compress/lzw` without it.
4. The JPEG decoder `zune-jpeg` becomes `image/jpeg`. The decoder of Go
   removes the Adobe inversion of CMYK, and converts YCCK. The port gives 255
   minus each of these values, which is the raw CMYK of upstream and the
   result of the YCCK formula of pdf.js. A 4-component JPEG without the
   marker of Adobe gets a marker with the transform 0, so that the decoder of
   Go reads it as CMYK.

## What the port leaves out

1. The feature `unsafe` of upstream, with SIMD, `zlib-rs` and `memchr`.
2. The trait `ByteBuf` and `SmallVec`. Go uses slices.
3. The crates `hayro-jbig2` and `hayro-jpeg2000`. The hooks below take their
   place.
4. `PdfData`. A slice of bytes holds the file.
5. The crate `hayro-ccitt`. The filter `CCITTFaxDecode` calls its port in
   `internal/ccitt` (D17), with the settings of upstream.

## The hooks of JBIG2 and JPX

The other ports of ivy hold JBIG2 and JPEG 2000. This package does not import
them. A caller fills `ImageDecoders`, in `Options.Decoders` of
`NewPdfWithOptions`, or in `ImageDecodeParams.Decoders` for one call:

1. `JBIG2Func` takes the data and the decoded `JBIG2Globals`, and returns
   the page as rows of packed bits, with 1 for black.
2. `JPXFunc` takes the data, the number of channels of a raw codestream or 0,
   and a target size. It returns the size, the color space, the bit depth,
   the alpha flag and the samples of 8 bits.

The code around each call of upstream stays in `filter_jbig2.go` and
`filter_jpx.go`: the inversion and the packing of JBIG2, the split of the
alpha channel and the scaling of JPX. A nil hook makes the filter fail with
`ivy.ErrUnsupported`.

## The differences from upstream

Each difference is marked with `Port:` in the code.

1. `Options.Limits` bounds the work. `MaxMemory` bounds each decoded stream,
   each JPEG and CCITT image, and the joined content streams of a page.
   `MaxPixels` bounds each image. A stream past a limit fails with
   `ivy.ErrLimit`.
2. The parent chain of a `ReaderContext` holds at most 4096 objects. Upstream
   copies the chain on each read, which costs time and memory that grow with
   the square of its length.
3. The scan after an inline image nests at most 4 levels. Upstream does not
   bound the nesting, and a crafted stream can take exponential time.
4. The search for `endstream` remembers its last result. The result does not
   change, but many streams without `endstream` no longer take quadratic
   time.
5. An object stream that fails to decode is not decoded again.
6. `XRef.objects` breaks a tie of two offsets by the identifier, where
   upstream keeps the order of a hash map.
7. The filter `CCITTFaxDecode` stops the decoder when its rows pass
   `MaxMemory`, also when the number of rows is not known. It also leaves
   out a span of black pixels that is empty or past the row, so that a
   fault of the decoder cannot make a panic. Upstream bounds neither.
8. `image/jpeg` fails on a truncated JPEG, an arithmetic code or a precision
   of 12 bits. The conversion of YCbCr to RGB of Go can differ by 1 from
   that of zune-jpeg. For a color space with other than 1, 3 or 4
   components, the port gives the raw components of the decoder of Go.
9. A zero `Bpc`, `NumComponents` or `TargetDimension` of `ImageDecodeParams`
   stands for `None`.
10. `Reader.ReadU8` is `read_byte` of upstream, because `go vet` requires
    the signature of `io.ByteReader` for a method named `ReadByte`.
