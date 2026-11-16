# gowemf

This folder is a copy of [github.com/mgilbir/gowemf](https://github.com/mgilbir/gowemf)
at the commit `377668cb7b13`, under the MIT license in `LICENSE` (D7, D24).
`THIRD_PARTY_NOTICES.md` is the file of upstream. It keeps the notices of
the Windows code pages and of the Unicode data that the tables come from.
The rules for a copy are in [`docs/UPSTREAM.md`](../../docs/UPSTREAM.md).

gowemf reads WMF, EMF and EMF+ files, and plays their records through a
small interface. The package `internal/metafile` of ivy implements the
interface on the display list (D10).

## The files

Each file has the same path as in upstream.

| Folder | What it holds |
| --- | --- |
| the root | the reader of the records, the decoders of the bitmaps, the color management, the playback, and their tests |
| `internal/bidi` | the bidirectional algorithm of Unicode, for the text, and its tests |
| `internal/raster` | a small rasterizer that only the tests of the playback use |

The copy leaves out these files of upstream:

1. The folders `cmd`, `tools` and `rendercheck`. They are a command, the
   programs of the oracles in Java and C#, and a module of tests that
   imports an outside package.
2. The folders `internal/corpus`, `internal/corpusfetch`, `internal/oracle`,
   `internal/bidigen`, `internal/codepagegen` and `internal/codepage`. They
   download files from the network, run outside programs, or generate the
   tables.
3. The tests `corpus_test.go`, `oracle_test.go`, `render_oracle_test.go`
   and `windows_oracle_test.go`. They read downloaded files, or run Java,
   LibreOffice or Windows.
4. The files of the project: `go.mod`, `go.sum`, `Makefile`, `.gitignore`,
   `README.md`, `AGENTS.md`, `COVERAGE.md`, `ORACLES.md`, and the folders
   `.github` and `.claude`.

The copy imports only the standard library, its own packages,
`internal/moxcms` and the root package of ivy.

## The changes

Each change is marked in the code with a comment that starts with `ivy:`.

1. The imports of `github.com/mgilbir/gowemf/internal/...` name the copy,
   `github.com/xo/ivy/internal/gowemf/internal/...`.
2. `color_transform.go` and `color_pixels.go` use `internal/moxcms`, not
   golittlecms, so the copy adds no module to `go.mod` (D24). The names and
   the checks of the ICC profiles stay. These parts change:
   - A calibrated RGB space of a DIB or of an EMF record becomes a small ICC
     profile of version 4, with its colorants adapted to the white of the
     PCS by the Bradford transform. moxcms reads it as any other profile.
   - moxcms has no black point compensation. The option
     `BlackPointCompensation` changes nothing.
   - moxcms has no proofing transform. `NewProofingColorTransform` checks
     its arguments, and then returns `ErrUnsupported`.
   - An error of moxcms is wrapped with `ErrMalformed`, `ErrLimit` or
     `ErrUnsupported` of gowemf, after the error of ivy that it wraps.
3. The tests build their ICC profiles by hand, because moxcms writes no
   profile. `TestSoftProofingIdentity` expects `ErrUnsupported`.
4. `codepage_test.go` leaves out `TestCodePageTables` and `TestDBCSTables`,
   and `internal/bidi/bidi_test.go` leaves out `TestTables`, `TestBidiTest`
   and `TestBidiCharacterTest`. They read files that upstream downloads, or
   run the Go command.
5. A comment of `color_pixels_test.go` no longer names the illuminant of
   the PCS by its short name, which the tests of ivy read as the number of
   a decision.
