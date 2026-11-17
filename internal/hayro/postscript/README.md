# postscript

The package `postscript` is a port of the Rust crate `hayro-postscript`, at
the commit `61f50b42a7f7b15764780b9ba534cba91488bb28` of hayro (D14, D16). The
crate is a small scanner of PostScript. It splits a program into numbers,
names, strings and procedures. The package `cmap` uses it to read CMap files.

The code is under the Apache-2.0 or the MIT license, as upstream is. The
files `LICENSE-APACHE` and `LICENSE-MIT` are copies of the files of hayro.

## Files

Each Go file ports one Rust file of `hayro-postscript`. A test file ports the
tests of the Rust file of the same name.

| Upstream file | Go file | Tests |
| --- | --- | --- |
| `src/lib.rs` | `lib.go` | `lib_test.go` |
| `src/error.rs` | `error.go` | |
| `src/name.rs` | `name.go` | `name_test.go` |
| `src/number.rs` | `number.go` | `number_test.go` |
| `src/procedure.rs` | `procedure.go` | `procedure_test.go` |
| `src/reader.rs` | `reader.go` | |
| `src/string.rs` | `string.go` | `string_test.go` |
| `src/string/ascii_85.rs` | `string_ascii_85.go` | |
| `src/string/ascii_hex.rs` | `string_ascii_hex.go` | |
| `src/string/literal.rs` | `string_literal.go` | |
| `src/token.rs` | `token.go` | `token_test.go` |
| `examples/tokenize.rs` | left out | |
| none | | `fuzz_test.go` |

`fuzz_test.go` holds `FuzzScanner`, a fuzz test that upstream does not have.

## Differences

A comment that starts with `Port:` marks each difference in the code. These
are the differences:

1. The package leaves out the example `examples/tokenize.rs`. It is a program,
   not a part of the scanner.
2. Upstream returns a `Result`. Go returns an error next to the value. The
   type `Error` has the two values of upstream, `ErrSyntax` and
   `ErrLimitCheck`. `ErrSyntax` wraps `ivy.ErrFormat`, and `ErrLimitCheck`
   wraps `ivy.ErrLimit`.
3. The enums `Token` and `Number` hold data. `Token` is an interface that
   `Number`, `Name`, `String` and `Procedure` implement. `Number` is a struct
   with the field `IsReal`.
4. Rust derives the comparison `PartialEq`. Go cannot compare a struct that
   holds a slice with `==`, so `Name`, `String` and `Procedure` have the
   method `Equal`, and `TokenEqual` compares two tokens.
5. `String.DecodeInto` returns the slice that it fills, as `append` does.
6. `Scanner.SetOffset` takes an `int`, which can be negative. After a
   negative offset, each read fails.
7. `strconv.ParseFloat` reports a real number that overflows as an error.
   Rust returns an infinity. The scanner reports both as `ErrLimitCheck`, as
   upstream does.

The port keeps the faults of upstream. For example, a literal string skips
all the ends of lines that follow an end of line, and an octal escape over
255 gives no byte.

## Limits

The input is untrusted. The scanner never recurses. It counts the depth of
braces and parentheses. Each token that the scanner returns moves it forward
by at least one byte, so a loop over the tokens ends.
