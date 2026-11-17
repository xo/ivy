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

Each copied file has the same path as in upstream.

| Folder | What it holds |
| --- | --- |
| the root | the reader of the records, the decoders of the bitmaps, the color management, the playback, and their tests |
| `internal/bidi` | the bidirectional algorithm of Unicode, for the text, and its tests |
| `internal/raster` | a small rasterizer that only the tests of the playback use |

Two files are files of ivy, not of upstream:
`playback_plus_string.go` draws the EMF+ DrawString record (change 11),
and `ivy_test.go` holds the tests of the changes 6 to 11 and 13 to 16.

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
6. `emf.go` and `wmf.go` read the framing of a file as Windows does, not
   as strictly as MS-EMF and MS-WMF write it. Upstream refuses these files:
   - The count of records in the header of an EMF file is not checked.
   - The maximum size of a record in the header of a WMF file is not
     checked.
   - A file can have bytes after its EOF record. A size in the header that
     is smaller than the data is not an error. A larger size is still an
     error, because the file is cut short. Change 16 gives the one
     exception.
   - A file can end without an EOF record, if its last record ends at the
     end of the data.
7. `stream.go` and `stream_wmf.go` drop a record that Windows ignores, and
   give the other records to the visitor. Upstream returns `ErrMalformed`
   for these records:
   - the selection or the deletion of an object that does not exist, or of
     a stock object that cannot be deleted
   - the selection with SelectObject of a palette or a color space
   - a reference to an object of another type, such as a brush in place of
     a region, in WMF, EMF and EMF+ records
   - the change of a palette that is not selected, or that was deleted
   - the creation of a WMF object when the table is full, or of an EMF
     object with a handle outside the table. An EMF object with a handle
     in use replaces the object.
   - a WMF SelectClipRegion record names a slot without an object. It
     removes the clipping region, as SelectClipRgn does with a null region.
     The player does the same in `playback_records.go`.
8. `decode_wmf.go` reads a WMF ExtTextOut record that is too short for all
   its advances. The text has no advances, and takes the advances of its
   font.
9. `playback_state.go` and `playback_fills.go` read a WMF pattern brush of
   the style BS_PATTERN that holds a DIB in place of a Bitmap16, as Stata
   writes it. A monochrome DIB takes the colors of the device context, and
   another DIB is a DIB pattern.
10. `playback_records.go` and `playback_fills.go` pass two more kinds of
    drawing to a `RasterBackend`, a backend that reads its destination.
    Upstream reports them as not supported for every backend:
    - the inversion of a region, for the records WMF InvertRegion and
      EMR_INVERTRGN
    - a fill with a mix mode (ROP2) that combines the brush with the
      destination, such as R2_MASKPEN or R2_XORPEN. A stroke with such a
      mix mode is still reported.
11. `playback_plus.go` draws the EMF+ DrawString record with
    `plusDrawString` of `playback_plus_string.go`. MS-EMFPLUS does not
    define the layout of GDI+, so the layout is close, not exact: lines
    break at line feeds and at spaces, the margins are fractions of the em,
    and the tracking multiplies the advances. Upstream reports the record
    as not supported.
12. The tests of upstream that expect `ErrMalformed` for the cases of
    changes 6 and 7, or that expect DrawString to be reported, expect the
    behavior of ivy. Each such test has a comment that starts with `ivy:`.
13. `decode_emf.go` does not compare the field nRgnSize of the region data
    of an EMF record with the count of its rectangles. nRgnSize is the size
    of a buffer, which Windows lets be 0, and some writers make it larger,
    such as 168 bytes for one rectangle. The rectangles must still fit in
    the record.
14. `playback_nested.go` and `playback_plus.go` place the source rectangle
    of an EMF+ DrawImage of a metafile in the coordinates of the bounds of
    the metafile, as GDI+ does. The bounds are the frame of an EMF in
    pixels, or the bounds of a placeable WMF, and they need not start at
    0. A source rectangle can be outside the image by a rounding error of
    float32, up to 0.001 pixel.
15. `decode_wmf.go` reads a WMF CreateFontIndirect record whose face name
    has fewer than 32 bytes, as MathType writes it.
16. `wmf.go` reads a WMF without a placeable header whose header gives a
    size 22 bytes larger than the data. A WMF picture of the store of
    Office Drawing counts the placeable header that the store removed.
