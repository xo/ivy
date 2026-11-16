# cmap

The package `cmap` is a port of the Rust crate `hayro-cmap`, at the commit
`61f50b42a7f7b15764780b9ba534cba91488bb28` of hayro (D14, D16). The crate
parses the CMap files of PDF. A CMap maps the character codes of text to CIDs
(the numbers of the glyphs of a CID font), and it can map codes to Unicode
text. The parser reads CMaps with the package `postscript`.

The code is under the Apache-2.0 or the MIT license, as upstream is. The
files `LICENSE-APACHE` and `LICENSE-MIT` are copies of the files of hayro.

## Files

Each Go file ports one Rust file of `hayro-cmap`. The tests of
`src/lib.rs`, in its modules `tests` and `bcmap_tests`, are in
`lib_test.go`.

| Upstream file | Go file |
| --- | --- |
| `src/lib.rs` | `lib.go`, `lib_test.go` |
| `src/parse.rs` | `parse.go` |
| `src/bcmap/mod.rs` | `bcmap_mod.go` |
| `src/bcmap/embedded.rs` | `bcmap_embedded.go` |
| `src/bcmap/huffman.rs` | `bcmap_huffman.go` |
| `src/bcmap/reader.rs` | `bcmap_reader.go` |
| `assets/cmaps.brotli` | `data/cmaps.bin`, decompressed |
| `assets/LICENSE.txt` | `data/LICENSE.txt` |
| none | `port_test.go`, `fuzz_test.go` |

`port_test.go` tests the parts of the port that upstream does not have.
`fuzz_test.go` holds `FuzzParse`, a fuzz test that upstream does not have.

## The predefined CMaps

Upstream embeds 65 predefined CMaps, such as `UniGB-UCS2-H`, in the file
`assets/cmaps.brotli`. The file holds the CMaps in a binary form, compressed
with brotli. The format is at
<https://github.com/LaurenzV/cmap-resources/tree/hayro>. Upstream decompresses
the file with the crate `brotli` at the first use.

ivy has no decoder of brotli. The port embeds the decompressed data with
`go:embed`. The brotli command made `data/cmaps.bin` from the file of
upstream:

```sh
brotli -d -c upstream/hayro/hayro-cmap/assets/cmaps.brotli > pdf/internal/cmap/data/cmaps.bin
```

| File | Bytes | SHA-256 |
| --- | --- | --- |
| `assets/cmaps.brotli` | 250,740 | `ddf37af521d904d5646c2fc37b2b7e912dc1b56f0d210190b61f598154758e46` |
| `data/cmaps.bin` | 467,235 | `230f46e3068770bf0e2c515e95473e46df6f0854eff3552e48e355e8ab148868` |

The data of the CMaps comes from Adobe, under the license in
`data/LICENSE.txt`, a copy of `assets/LICENSE.txt` of upstream. It is the
BSD 3-clause license of Adobe, "Copyright 1990-2023 Adobe".

## Differences

A comment that starts with `Port:` marks each difference in the code. These
are the differences:

1. The brotli command decompressed the data of the predefined CMaps at the
   time of the port. Upstream decompresses it at run time. The package does
   not depend on brotli.
2. Upstream has the features `std` and `embed-cmaps`. The port always has the
   predefined CMaps.
3. Upstream returns `None` for each error. Go returns an error that wraps
   `ivy.ErrFormat`, or `ivy.ErrLimit` for a limit. A lookup returns a value
   and a `bool`.
4. The enum `CMapName` is the string type `Name`, because `cmap.CMapName`
   stutters in Go. Its constants are the predefined names. Any other name is
   a custom name.
5. `CMap::identity_h` and `CMap::identity_v` are `NewIdentityH` and
   `NewIdentityV`, because the constants `IdentityH` and `IdentityV` take the
   names.
6. The enums `CidFamily` and `BfString` hold data. `CIDFamily` and `BfString`
   are structs with a field that tells the variant. `==` compares two values
   as the derived `PartialEq` of upstream does.
7. A field of `Metadata` that upstream holds in an `Option` is a pointer or a
   slice. `nil` means `None`.
8. The resolver of `usecmap` is the type `Resolver`. It can be nil. Then a
   `usecmap` operator is an error.
9. `findInRanges` follows `slice::binary_search_by` of Rust step by step. If
   two ranges overlap, it finds the same range as upstream.
10. `LoadEmbedded` uses the string of a name. A custom name that is equal to
    a predefined name finds the predefined CMap.

The port keeps the faults of upstream. For example, the search for a CID
does not test the codespace ranges, and a CMap keeps only the last
`usecmap`.

## Limits

The input is untrusted. The port adds two limits to those of upstream:

1. A `usecmap` segment of a binary CMap adds one to the depth, as the
   `usecmap` operator does. Upstream keeps the depth. If the resolver
   returns a binary CMap that names itself, upstream recurses without end.
   The depth of `usecmap` is at most 16, as in upstream.
2. One call of `Parse` reads at most 64 CMaps through `usecmap`, at all
   depths together. Upstream reads the named CMap again for each `usecmap`
   operator, so a small file can make it read a large predefined CMap
   thousands of times.

Each loop reads a token or a symbol of the input at each step, so the size
of the input bounds it. A range of codes is never expanded into one entry for
each code. A `bfrange` array has one entry for each string of the input. The
parser of the binary form reserves memory for at most eight symbols for each
byte of the encoded data.
