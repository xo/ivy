# ivy Plan

This document holds the plan of ivy: its purpose, its scope, its
architecture, its testing, its phases and its open questions. The decisions
are in [`decisions/`](decisions/README.md). The design of the API is in
[`API.md`](API.md). The survey of the formats, with the estimates, is in
[`FORMATS.md`](FORMATS.md).

## Status

On 2026-11-17 phases 1 to 6 are done. The root package holds the document
API of `API.md`, the display list and the fonts of D10 hold the drawing of a
page description, and XPS renders, as the section of phase 2 below says. PDF
renders, as the section of phase 3 says. The layout of HTML and CSS puts the
ebooks on pages, as the section of phase 4 says. The Office formats give
their pages, as the section of phase 5 says, and DjVu gives its pages, as
the section of phase 6 says. On 2026-11-19 phases 7a and 7b are done:
`pdf/pdfenc` writes the pages of a document as a PDF file, with the text
that a reader can search and copy (D58). These
formats open, as the section of phase 1 below says:
OpenDocument, Word, Excel and PowerPoint in Office Open XML, iWork,
OpenRaster, Krita, PostScript and EPS, the RAW files of cameras, comic book
archives in zip and in tar, and Photoshop. The maintainers decided D1 to D15,
and the decisions were consolidated before the first commit (D4). The work
that phase 1 left is in `BACKLOG.md`. The open questions are at the end of
this document.

## Purpose

ivy decodes, renders and encodes the file formats that a program shows as
images, in pure Go (D1). iv is the first program that uses it. For many of
these formats, iv runs an outside program or links a library through cgo
today. Each one is a dependency that a person must install, and the outside
programs are slow to start:

| Decoder of iv | What it uses | Formats | The package of ivy that replaces it |
| --- | --- | --- | --- |
| `calibre` | `ebook-convert`, about 1.5 seconds for each book | EPUB, MOBI, AZW, AZW3, FictionBook | `epub`, `mobi`, `fb2`, and the other ebooks |
| `gxps` | `xpstopng`, about 300 ms for each page | XPS, OpenXPS | `xps` |
| `libreoffice` | `soffice`, a second or more | the Office formats, RTF, CSV, Publisher, Visio, WordPerfect | `docx`, `xlsx`, `pptx`, `odf`, `rtf`, `msdoc`, `msxls`, `msppt`, and the previews |
| `vips-pdf` and `poppler` | libvips through cgo, then `pdftoppm` | PDF, with its password | `pdf` |
| `psd` | `oov/psd` in Go, then libvips | Photoshop | `psd` |
| `blitz` | the Rust engine Blitz through cgo | HTML, Markdown and text | `html`, `markdown`, `text`, once the layout exists |

ivy also adds formats that iv does not show today: PostScript, EPS, Adobe
Illustrator, DjVu, EMF, WMF, OpenRaster, Krita, iWork, camera RAW, and the
icons of disk images, applications and packages. `FORMATS.md` lists each
one.

A rendering is good enough when a person can review the document with it
(D5). ivy is not a replacement for the program that made a document.

## Scope

In scope:

1. Each format in `FORMATS.md` that is not in the long tail, in the order of
   the phases below.
2. The files that a password protects, for each format with a public method
   of encryption (D12).
3. The DRM that ivy can decode, as D12 says. The DRM that needs a key is in a
   module of its own.

Later, as D1 says: an encoder, when a program needs one, and each format of
the long tail, when a user asks for it.

Out of scope:

1. Editing a document, filling in a form, and selecting or searching text.
2. Exact fidelity to the program that made a document (D5).
3. The formats that iv decodes already and keeps: the images that the
   standard library of Go and libvips decode, fonts, the icons of a Windows
   executable, and the cover art of audio and video.

## Architecture

### The repositories

ivy decodes. okage draws, and magic identifies and decompresses (D8). A
program finds the type of a file with magic, opens it with the package of ivy
for that type, and gets pages that draw with okage:

```text
bytes ──> magic ──> mime type ──> ivy/<format> ──> document ──> page ──> okage.Pixmap
```

ivy reuses the parsers, the fonts and the geometry of okage, and the
decompression of magic, before it ports or copies code (D7). The work that
ivy needs from okage and from magic is in [`BACKLOG.md`](BACKLOG.md), under
the name of each repository.

### The packages and the API

D6 puts each decoder and each encoder in a package of its own, and D10 puts
the shared layers in `internal/`. [`API.md`](API.md) holds the expected
layout of the packages, the document API of D11, and an example of how iv
uses it.

### The display list

The display list is the form of a page description (D10). Its commands
follow the operators of PDF, because PDF is the largest format that uses it
and the others reduce to a subset of PDF:

1. Save and restore the state, and set the transform.
2. Fill and stroke a path, with a color, a gradient, a shading or a pattern.
3. Clip with a path.
4. Draw an image, with a mask.
5. Draw a run of glyphs from a font of the font layer.
6. Begin and end a group, with its opacity, its blend mode and its soft mask.

The device that draws with okage is the main device. The `Device` of
hayro-interpret is the model for it (D14). The layout draws its pages through
the same device (D15).

### The layout

A flow of text becomes HTML and CSS, and the layout puts it on pages (D10).
EPUB, the HTML of MOBI and CHM, FictionBook, Markdown, HTML, text, Word,
OpenDocument Text and RTF go through it. Each one converts its own structure
into HTML, with a style sheet that sets the default fonts and the page size.

The layout is a port of the layout of WeasyPrint (D15). It paginates with
`@page`. It parses XML, HTML and CSS with the parsers of okage, and it shapes
text with the text API of okage, on the copy of go-text in okage (D7, D28). Where a parser of okage
lacks something, the work goes to okage (D8).

### Go 1.27

D9 says that ivy uses generics and the features of Go 1.27 where they help.
The hot loops that are candidates for SIMD are the conversions of color
spaces, the planar channels of PSD, and the inverse wavelet transforms of
DjVu and JPEG 2000. Each SIMD path keeps a scalar path, and the two agree
within the tolerance of a review (D9).

## Testing plan

1. Each package has unit tests with small files in its `testdata/`. Git does
   not change them (`.gitattributes`).
2. Each decoder has a fuzz test of Go, and runs under the limits of its
   configuration (D5).
3. A test of each format package lists its imports with `go list -deps`, and
   fails if it links a package that it does not need (D6).
4. A test of each SIMD loop compares it with its scalar path (D9).
5. The test module compares ivy with the oracles within a tolerance. The
   corpus comes from the sources that ivy ports, such as the test files of
   hayro. A maintainer runs it by hand, and CI does not run it (D13).
6. `docs_test.go` and `skills_test.go` hold the setup of D2, as in the other
   `xo` repositories.

## Phases

The order comes from the survey in `FORMATS.md`. Each phase gives iv a
result that it can use. The estimates are the survey estimates, in months of
work for one engineer with coding agents. The phases and the track of the
containers add up to the 41 to 59 months of the core in `FORMATS.md`.

### Phase 0. The plan and the rules

This document, the design of the API, the decisions, the survey and the
setup of D2. Phase 0 ends when this plan is committed.

### Phase 1. The harness and the previews (3 to 4 months, done)

The root package of `API.md`, `docs_test.go`, `skills_test.go`, a test of
the imports of D6, the CI workflow and the test module. Then the previews of
D11 that are PNG, JPEG or TIFF: OpenDocument, Office Open XML, iWork,
OpenRaster, Krita, the TIFF and EPSI previews of EPS, camera RAW, and the
comic book archives in zip and in tar. PSD, with the 16-bit files fixed. A
preview in EMF or WMF waits for phase 5, and CBR and CB7 wait for the
approval of their outside packages (D6, D7).

Phase 1 delivered these packages, on 2026-11-16:

| Package | What a document gives |
| --- | --- |
| `odf` | the thumbnail of an OpenDocument file |
| `docx`, `xlsx`, `pptx` | the thumbnail of an Office Open XML file, in PNG, JPEG or GIF |
| `iwork` | the previews of Pages, Keynote and Numbers |
| `ora` | the merged image of OpenRaster and Krita as one page, the thumbnail, and `image.Decode` |
| `ps` | the TIFF preview of DOS EPS, the EPSI preview, and the DSC comments |
| `camera` | the JPEG images of CR2, CR3, NEF, ARW, DNG, ORF, RW2, PEF, SRW and RAF, the make, the model and the date, and `image.Decode` for the formats with a magic of their own |
| `comic` | each image of a CBZ or a CBT file as a page, and the first page as the cover |
| `psd` | the image of PSD and PSB as one page, the layers of a file without a merged image, the thumbnail, and `image.Decode` |

The test module compares `psd` with ImageMagick. iv can now drop `oov/psd`.
That change is work for iv, in `BACKLOG.md`.

### The containers with icons (7 to 10 months, beside the phases)

DMG, IPA, APK, MSI, the macOS applications and installers, the Linux
packages, the disc images and the other containers of `FORMATS.md`. Most of
them depend only on the root package, so this work can run beside any
phase, after phase 1. DMG comes first, because the maintainers named it. It
waits for LZFSE and ADC in magic (D8). An icon of APK in vector XML waits for
the display list of phase 2.

### Phase 2. The display list, the fonts and XPS (3.5 to 5 months, done)

`internal/display`, the okage device, the first part of `internal/font`, and
`xps`. iv can drop `xpstopng`.

Phase 2 delivered these packages, on 2026-11-16:

| Package | What it holds |
| --- | --- |
| `internal/display` | the `Device` of the display list, a `List` that records it, and the device that draws with okage, with its clips and its groups |
| `internal/font` | the fonts of go-text: the map from characters to glyphs, the advances, and the outlines |
| `xps` | XPS and OpenXPS: each page, the thumbnail, and the core properties |

`xps` reads its parts with `roxmltree` of okage, which phase 2 made public in
okage (okage D47). Its pages match `xpstopng` within the tolerance of a
review in the test module. A VisualBrush, an OpacityMask, a sideways run of
glyphs and an image in JPEG XR are left out, and their page returns an
`*ivy.PartialError`.

### Phase 3. PDF (6.5 to 10 months, done)

The port of hayro onto the display list, with its encryption, its fonts, its
image decoders and moxcms (D14). Illustrator files that are compatible with
PDF. iv drops libvips and `pdftoppm` for PDF.

Phase 3 delivered these packages, on 2026-11-16:

| Package | What it holds |
| --- | --- |
| `pdf` | PDF and Illustrator compatible with PDF: each page, the thumbnails, the document information and the password (D12) |
| `internal/hayro/syntax`, `postscript`, `cmap`, `pdffont`, `interpret` | the ports of hayro-syntax, hayro-postscript, hayro-cmap, hayro-font and hayro-interpret (D16) |
| `internal/hayro/render` | the port of the crate hayro, which draws onto the display list (D20) |
| `internal/hayro/kurboext` | the parts of kurbo that the port needs and that okage does not port |
| `internal/ccitt`, `internal/jbig2`, `internal/jpeg2000` | the ports of hayro-ccitt, hayro-jbig2 and hayro-jpeg2000 (D17) |
| `internal/moxcms` | the subset of moxcms that hayro uses (D16) |

On 2026-11-17 the ports of hayro moved from `pdf/internal/` to
`internal/hayro/`, so that the interpreter of PostScript uses them too (D29).

`internal/display` grew groups with a blend mode and a soft mask, images that
a transform places, and image masks. `pdffont` reads TrueType and CFF with
go-text, and it does not hint glyphs (D19). The pages match the pages of hayro within a
mean difference of 1.5. pdftoppm draws 11 of the 63 test pages in another
way, and the other pages match pdftoppm within the tolerance of a review in
the test module. The work that is left is under "Left by phase 3" in
`BACKLOG.md`.

### Phase 4. The layout and the ebooks (8 to 10.5 months, done)

`internal/layout`, the port of WeasyPrint (D15), and the work in okage that
it needs. Then EPUB, FictionBook, Markdown, HTML and text, then MOBI, AZW3,
AZW4 and CHM. MOBI and CHM wait for their decompression in magic (D8). iv
drops `ebook-convert` and `blitz`.

Phase 4 delivered these packages, on 2026-11-16:

| Package | What it holds |
| --- | --- |
| `epub` | EPUB 2, EPUB 3 and kepub: the pages, the cover and the metadata. It undoes the obfuscation of fonts |
| `mobi` | MOBI, PRC, AZW and AZW3 through the layout, and AZW4 through `pdf`: the pages, the cover and the metadata |
| `fb2` | FictionBook 2, as XML or in a zip archive: the pages, the cover and the metadata |
| `chm` | Compiled HTML Help: the pages, the title and the language |
| `html` | HTML and HTMLZ: the pages, the cover of an HTMLZ file and the metadata |
| `text` | text and TXTZ: the pages, the cover of a TXTZ file and the metadata of a TXTZ file |
| `markdown` | Markdown, with the extensions of GitHub, footnotes and definition lists: the pages and the title |
| `internal/layout` | the port of the cascade, the boxes, the text, the layout, the drawing and the stacking of WeasyPrint 70.0, and the book that puts a whole ebook on pages (D15, D21) |
| `internal/ebook`, `internal/memfs`, `internal/charset` | the code that the ebooks share, a file system in memory, and the encodings of the Encoding Standard |
| `internal/goldmark` | the copy of goldmark (D7, D21) |

okage holds the parsers that the layout needs: a port of tinyhtml5 (okage
D49), ports of tinycss2 and cssselect2 (okage D50), and the readers of
WOFF, WOFF2 and EOT (okage D51). magic holds the decoders of PalmDoc,
HUFF/CDIC and LZX (magic D61). The layout shapes text with go-text
directly, because the text API of okage does not give the clusters that
the breaks of the lines need. The layout puts a page on the display list,
and okage draws it. A book lays out its documents only as far as the page
that a program asks for.

The port runs 1333 of the test functions of WeasyPrint, and 1271 of them
pass. The skipped tests need hyphenation, color glyphs, profiles of CMYK,
small capitals in the test fonts, or the PDF file of upstream. The pages
of an EPUB file match the pages of calibre within the tolerance of a
review in the test module. The work that is left is under "Left by phase 4"
in `BACKLOG.md` and under "Open problems" in `internal/layout/README.md`.

### Phase 5. The Office formats (11 to 18 months, done)

EMF and WMF, then the slides, the documents, the spreadsheets, RTF, the
previews and the text of the binary formats, and the encryption of
MS-OFFCRYPTO and OpenDocument. iv drops `soffice`. D24 holds the choices:
a copy of gowemf for EMF and WMF, a public reader of Compound Files in
magic, a module of fonts with the metrics of the Office fonts, the preset
shapes of ECMA-376, Blowfish from `golang.org/x/crypto`, charts with zuzu
(which D27 replaced with charts that ivy draws itself),
and every sheet of a spreadsheet.

Phase 5 delivered these packages, on 2026-11-17:

| Package | What it holds |
| --- | --- |
| `emf` | EMF, EMF+ and WMF, also compressed |
| `docx`, `odf`, `rtf` | the pages of Word, OpenDocument text and RTF, through the layout |
| `xlsx`, `odf`, `csv` | every sheet of Excel and OpenDocument, and CSV, as tables on pages |
| `pptx`, `odf` | one page for each slide or drawing, on the display list |
| `msdoc`, `msxls`, `msppt` | the text and the sheets of the binary formats through the layout, the slides on the display list with `internal/drawingml`, their thumbnails, and their RC4 and XOR encryption |
| `vsdx` | the pages of Visio on the display list (D30), and the thumbnails and the properties of every Visio file |
| `pub` | the thumbnails and the properties of Publisher |
| `fonts` | a module of fonts with the metrics of the fonts of Office (D24) |
| `internal/gowemf`, `internal/metafile` | the copy of gowemf, and the playback of a metafile onto the display list |
| `internal/drawingml`, `odf/internal/drawpage` | the shapes, the colors, the fills, the pictures and the text of slides |
| `internal/chart` | the charts, which ivy lays out and draws as Office does (D27) |
| `internal/sheet`, `internal/numfmt` | the sheets as HTML tables, and the number formats of Excel |
| `internal/offcrypto`, `odf/internal/odfcrypt` | the encryption of MS-OFFCRYPTO and of OpenDocument |
| `internal/msole`, `internal/fontdb` | the code of the binary formats, and the font database of a program |

magic holds the public reader of Compound Files, `magic/cfb` (magic D62).
The layout reads EMF and WMF, draws the
objects of a format such as a chart, and draws a page of a book onto a
device. The test module compares the pages of each kind with LibreOffice.
The work that is left is under "Left by phase 5" in `BACKLOG.md`. An
OpenDocument file that LibreOffice encrypts with Argon2id opens (D25).

### Phase 6. DjVu (1 to 2 months, done)

A copy of the decoders of DjVu from dejapdf, after the backlog examines
them (D26).

Phase 6 delivered these packages, on 2026-11-17:

| Package | What it holds |
| --- | --- |
| `djvu` | the pages of single and bundled DjVu documents, with shared dictionaries, G4 masks, JPEG layers, palettes, rotation and gamma, the thumbnails and the metadata |
| `djvu/internal` | the copy of the decoders of dejapdf: `iff`, `zp`, `bzz`, `jb2`, `iw44` and the document model `doc` |

The test module compares each page with ddjvu of DjVuLibre, within a mean
difference of 1.6. An indirect document returns `ErrUnsupported` (D26).
The work that is left is under "Left by phase 6" in `BACKLOG.md`.

### Phase 7. PDF output (9 to 14 weeks)

The package `pdf/pdfenc` writes the pages of a document as a PDF file,
through the PDF writer of okage (D58). Each estimate is for one agent with
a maintainer, and assumes okage v0.17.0, which holds okage D68.

| Phase | What it gives | Estimate |
| --- | --- | --- |
| 7a. The API and the device | `Encode`, `EncodePages` and their options, the PDF device, the raster fallback, `display.Drawer` and the method `Draw` of each page type, the raster pages as images, the metadata, the limits, the errors, `FuzzEncode`, the round trip in CI, and the oracles of MuPDF, Poppler and qpdf. The glyphs are paths | 3 to 5 weeks |
| 7b. Fonts and text | `GlyphRun.Text` and the clusters, the source of `font.Face`, the font database with the bytes of its faces, the text of the layout, XPS, the metafiles and the charts, `fsType`, and the tests of the text | 3 to 4 weeks |
| 7c. Links and the outline | `display.Linker` and `display.Bookmarker`, the port of the rest of `weasyprint/anchors.py`, the tables of contents of the books, the links and the outline of `pdf`, `xps` and `djvu`, and the named destinations | 3 to 5 weeks |

A later decision can add the Type 3 fonts from outlines for `pdf` and
`ps`, invisible text for the text layer of DjVu, a picture that goes into
the file without a new encode, and a writer that streams its pages.

Phase 7a was done on 2026-11-19. The work that is left is under "Left by
phase 7a" in `BACKLOG.md`. Phase 7b was done on 2026-11-19, as "The build
of phase 7b" in D58 says. The work that is left is under "Left by phase
7b" in `BACKLOG.md`.

### Phase 8. LaTeX (8 to 13 weeks)

ivy reads and writes LaTeX with a copy of `go-tex/engine` that draws with
okage (D62). okage holds the layout of math (okage D72), which zuzu also
uses. Each estimate is for one agent with a maintainer.

| Phase | What it gives | Estimate |
| --- | --- | --- |
| 8a. The copy and the driver | `latex/internal/engine` without `go-pdfkit` and `go-gfx`, the driver from the tree of boxes to the display list, the specials of SVG, the figures through the decoders of ivy, a context and the limits | 2 to 3 weeks |
| 8b. The fonts | `fontFace` on the fonts of okage, Latin Modern in the module `fonts`, and the helpers of okage for the metrics, the kerning and the ligatures | 1 to 2 weeks |
| 8c. The package | `latex/` with the document API of D11, the classes and packages from `ivy.Config`, the fuzz test, the corpus, and real TeX as an oracle | 1.5 to 2 weeks |
| 8d. The math | the math of the engine, and the equations of Office Math, MathML and OpenDocument, through the layout of math of okage, after its release | 1 to 2 weeks in ivy, and 3 to 5 weeks in okage at the same time |
| 8e. The writer | `latex/latexenc`, which writes the HTML of a document of the layout as LaTeX | not estimated |

The phases can overlap. Phase 2 and the start of phase 4 do not depend on
each other, and the previews of phase 1 can continue while phase 2 starts.
The long tail is not a phase. Each format of it waits until a user asks for
it (D1). The first one is the interpreter of PostScript (D29). From
2026-11-17 it gives the pages of PostScript and EPS files.

## Risks

`FORMATS.md` lists the risks, under "The risks". The largest one is the
estimate itself. The models disagree by six times, so phase 1 measures one
format of each kind before the maintainers commit to phases 3 to 5.

## Open questions

A question has a number, and it stays here until the maintainers answer it.
The answer becomes a decision. The questions were numbered again when the
decisions were consolidated (D4). D22 answers question 3, D23 answers question
4, and D25 answers question 5. D33 to D45 answer questions 1 and 6 to 20. D55
answers question 21, D57 answers question 22, D58 answers question 23, and
D60 answers question 24, and D62 answers question 25. No question is open.
The next question is question 26.
