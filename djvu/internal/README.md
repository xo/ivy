# dejapdf

This folder is a copy of the decoders of DjVu of
[github.com/sergev/dejapdf](https://github.com/sergev/dejapdf) at the commit
`2b6ca0c1123c`, under the MIT license in `LICENSE` (D7, D26). `NOTICE` keeps
the part of the notice of dejapdf that applies to the copy. The rules for a
copy are in [`docs/UPSTREAM.md`](../../docs/UPSTREAM.md).

dejapdf converts DjVu to PDF. Its decoders follow the specification of DjVu
3, not the code of DjVuLibre. The package `djvu` of ivy uses them to read
the pages of a DjVu file, and draws the pages through the display list of
ivy (D10).

## The files

Each folder holds the files of the folder of the same name in
`internal/` of dejapdf, with their tests. The folder `doc` holds the files
of `internal/djvu`, because the package of the format is `djvu`.

| Folder | What it holds |
| --- | --- |
| `iff` | the reader of the chunks of IFF85 |
| `zp` | the ZP arithmetic decoder, which BZZ, JB2 and IW44 use |
| `bzz` | the decoder of BZZ, for DIRM, ANTz, FGbz and the other compressed chunks |
| `jb2` | the decoder of JB2 masks and of their shared dictionaries, Sjbz and Djbz |
| `iw44` | the decoder of IW44 wavelet images, BG44, FG44 and TH44 |
| `doc` | the document model: the directory of a bundled document, the INFO chunk, the includes, the masks with their shared dictionaries, the G4 masks, the palettes and the annotations |

The tests read the files of `djvu/testdata`, which holds a part of the
corpus of dejapdf. `doc/testdata` keeps the seed of `FuzzParse` of
upstream.

The copy leaves out these files of upstream:

1. The encoders: `jbig2`, `jpx`, `mq`, `stencil`, `shapes` and `segment`.
   Some carry parts under the Apache License 2.0.
2. The readers and the writers of PDF: `pdf`, `pdfread`, `rewrite`,
   `reencode`, `scan` and `convert`.
3. `ccitt` and `jpeg`. ivy has its own decoder of CCITT in `internal/ccitt`,
   and decodes JPEG with the package `image/jpeg` of Go (D26).
4. `raster` and `progress`, which only the converter uses.
5. In `internal/djvu`: `text.go` and `outline.go`, the text layer and the
   outline, which ivy does not draw, and `dump.go`, a dump of the chunks for
   the command of dejapdf. Their tests, `bench_test.go`, and the tests of
   `text_test.go` other than `TestParseSexps`, go with them.
6. The command in `cmd`, the documents in `docs`, the scripts and the files
   of `testdata`, and the files of the project: `go.mod`, `Makefile`,
   `README.md`, `CONTRIBUTING.md` and `CLAUDE.md`.

## The changes

Each change is marked in the code with a comment that starts with `ivy:`.

1. The imports of `github.com/sergev/dejapdf/internal/...` name the copy,
   `github.com/xo/ivy/djvu/internal/...`. The package `djvu` of dejapdf is
   the package `doc`.
2. The decoders take the limits of `ivy.Limits`, and an error past a limit
   wraps `ivy.ErrLimit` (D5):
   - `bzz.DecodeMax` bounds the output of BZZ.
   - `jb2.DecodeWithin` and `jb2.DecodeDictWithin` bound the size of the
     page by `MaxPixels`, the pixels of the bitmaps by `MaxMemory`, and the
     shapes and the blits by `MaxObjects`. `Decode` and `DecodeDict` keep
     the limits of upstream, with the defaults of ivy.
   - `doc.ParseWithin` bounds the components of the directory by
     `MaxObjects` and the depth of the includes by `MaxDepth`, and gives the
     limits to the masks, the palettes and the annotations. `Parse` takes the
     defaults.
   - `doc.Page.Layer`, the decoders of G4 masks and of palettes check the
     size of their images and lists before they decode them.
3. `doc.Document` indexes the components of a directory by id and by offset.
   Upstream scans the list for each component and each include, and a
   directory of many components takes a time that grows with the square of
   their number.
4. `doc.Document` forgets its decoded shared dictionaries when they take
   more than `MaxMemory`, and decodes a dictionary again when a page needs
   it.
5. `iw44.(*Decoder).ImageScaled` reconstructs an image at 1/2, 1/4, 1/8 or
   1/16 of its size. It inverts the wavelet transform only down to that
   scale, for a page that ivy draws small.
6. `doc.Page.Layer` returns the decoder of an IW44 layer, for
   `ImageScaled`. If a later chunk fails, it returns the decoder with the
   chunks before it, and the error.
7. `doc.AnnotationsOf` reads the annotations of any forms, for the shared
   annotations of a bundled document.
8. `mmr.go` decodes the G4 streams of an Smmr mask with `internal/ccitt` of
   ivy. A stream that fails keeps the rows that it decoded, and the mask
   comes with the error, so that a page can show the part.
9. The copy leaves out `Open` of `internal/djvu`, which reads a document
   and the files that it names from the file system. ivy reads only the
   source that a program gives. The tests define an `Open` of their own,
   which reads a single or a bundled document without its includes.
10. `doc/limits.go` is not in upstream. It holds `unbzz`, which decodes BZZ
    within the limits, and `u24` of `text.go`.
11. The tests read the files of `djvu/testdata`, and skip a line of
    `iw44.sha256` or `masks.sha256` whose file is not there. `TestOracle`
    of `bzz` is left out, because it runs the program `bzz` of DjVuLibre,
    and only the test module of ivy runs an outside program (D13).
    `TestDumpCorpus`, `TestIncludeFile`, `TestIndirect` and
    `TestBundledMatchesIndirect` of `internal/djvu` are left out, with the
    dump and the indirect documents. `iw44` adds `TestImageScaled`.
