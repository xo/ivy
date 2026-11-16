# ccitt

The package `ccitt` decodes CCITT fax data: Group 3 with one dimension (MH),
Group 3 with two dimensions (MR), and Group 4 (MMR), as ITU-T T.4 and T.6
define them. It is a port of the Rust crate `hayro-ccitt` of
[hayro](https://github.com/LaurenzV/hayro), at the commit
`61f50b42a7f7b15764780b9ba534cba91488bb28`. D17 decides the port.

The code is under the Apache-2.0 or MIT license of hayro. The files
`LICENSE-APACHE` and `LICENSE-MIT` are copies of the licenses of hayro.

## The API

`Decode` takes the data, a `Decoder` and the `DecodeSettings`. It gives each
row to `Decoder.PushRow` as the ascending positions where the color changes.
Each row starts white. `Decode` returns the number of bytes that it read:

```go
settings := ccitt.DecodeSettings{Columns: 1728, Encoding: ccitt.Group4}
n, err := ccitt.Decode(data, decoder, &settings)
```

A `Rows` of 0 decodes until an end-of-block marker or the end of the data.
`Strict` rejects malformed data. Without it, the decoder decodes as much as
it can. `Cancel` stops the decoder before a row. The type `CancelFunc` makes
a check from a function. Each error is a `DecodeError`. `ErrCancelled`
unwraps to `context.Canceled`, and each other error unwraps to
`ivy.ErrFormat`.

Two packages use it:

1. The filter `CCITTFaxDecode` of `pdf/internal/syntax`.
2. The MMR coding of the generic regions of `internal/jbig2`. The halftone
   regions of JBIG2 need the number of bytes that `Decode` read.

## The files

Each Rust file `src/a.rs` becomes the Go file `a.go`, and each test file
`tests/a.rs` becomes `tests_a_test.go`. Each Go file names its Rust file in
its first comment. A difference from upstream has a comment that starts with
`Port:`.

| Upstream file | Go file |
| --- | --- |
| `src/lib.rs` | `lib.go` |
| `src/bit_reader.rs` | `bit_reader.go` |
| `src/cancel.rs` | `cancel.go` |
| `src/codes.rs` | `codes.go` |
| `src/context.rs` | `context.go` |
| `src/error.rs` | `error.go` |
| `src/group3.rs` | `group3.go` |
| `src/group4.rs` | `group4.go` |
| `src/sync.rs` | not ported. A Go interface value takes the place of the `Arc` |
| `src/tables.rs` | `tables.go` |
| `tests/main.rs` | `tests_main_test.go` |
| `tests/cancel.rs` | `tests_cancel_test.go` |
| `tests/codes.rs` | `tests_codes_test.go` |
| `tests/dimensions.rs` | `tests_dimensions_test.go` |
| `tests/edges.rs` | `tests_edges_test.go` |
| `tests/g3_1d.rs` | `tests_g3_1d_test.go` |
| `tests/g3_2d.rs` | `tests_g3_2d_test.go` |
| `tests/g4.rs` | `tests_g4_test.go` |
| `tests/modes.rs` | `tests_modes_test.go` |
| `tests/recovery.rs` | `tests_recovery_test.go` |
| `tests/settings.rs` | `tests_settings_test.go` |

The file `ccitt_test.go` is not a port. It holds the fuzz test `FuzzDecode`
and the tests of the errors and of an image without columns.

## The shape of the Go code

1. The functions of `src/group3.rs` and `src/group4.rs` take the name of
   their module, such as `group4Decode` and `group3Recover`, as okage D10
   says. The names `decode` and `recover` collide with `Decode` and with the
   built-in function of Go.
2. The enum `Mode` is a struct with a kind and the offset of `Vertical`.
3. The type alias `Result` is not ported. A function returns its value and
   an error.
4. `CancelHandle` is not ported. `DecodeSettings.Cancel` holds the
   `CancelCheck` itself, and a nil value is the `None` of upstream.
5. The constants `Group3_1D` and `Group3_2D` keep the underscore of their
   Rust names.

## The differences from upstream

1. `tables.go` builds the lookup tables when the package starts. Upstream
   builds them at compile time in a `const fn`.
2. Each `DecodeError` unwraps to `ivy.ErrFormat` or to `context.Canceled`.
3. An `EncodingMode` that is not one of the three constants decodes as
   Group 4. Rust cannot make such a value.

The port adds no guard against hostile input (D18). Each loop of the decoder
reads at least one bit of the data, or it ends. So the work and the number
of rows grow with the length of the data. The rows that a `Decoder` keeps
are its own concern: `pdf/internal/syntax` bounds them with `MaxMemory`, and
`internal/jbig2` with the size of its bitmap.

## The test data

`testdata/streams` and `testdata/references` are copies of
`hayro-ccitt/tests/streams` and `hayro-ccitt/tests/references`, byte for
byte: 49 streams and 13 reference images.
