# ivy Plan

This document holds the plan of ivy: its purpose, its scope, its
architecture, its testing, its phases and its open questions. The decisions
are in [`decisions/`](decisions/README.md). The design of the API is in
[`API.md`](API.md). The survey of the formats, with the estimates, is in
[`FORMATS.md`](FORMATS.md).

## Status

On 2026-11-16 the repository holds the plan and the rules, and no code. This
is phase 0 (D3). The maintainers decided D1 to D15, and the decisions were
consolidated before the first commit (D4). No question blocks phase 1, and
phase 1 starts when this plan is committed. The open questions are at the end
of this document.

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
text with the text API of okage, on go-text (D7). Where a parser of okage
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

### Phase 1. The harness and the previews (3 to 4 months)

The root package of `API.md`, `docs_test.go`, `skills_test.go`, the CI
workflow and the test module. Then the previews of D11 that are PNG, JPEG or
TIFF: OpenDocument, Office Open XML, iWork, OpenRaster, Krita, the TIFF and
EPSI previews of EPS, camera RAW, and the comic book archives in zip. PSD,
with the 16-bit files fixed. iv drops `oov/psd` and gets new formats. A
preview in EMF or WMF waits for phase 5, and CBR and CB7 wait for the
approval of their outside packages (D6, D7).

### The containers with icons (7 to 10 months, beside the phases)

DMG, IPA, APK, MSI, the macOS applications and installers, the Linux
packages, the disc images and the other containers of `FORMATS.md`. Most of
them depend only on the root package, so this work can run beside any
phase, after phase 1. DMG comes first, because the maintainers named it. It
waits for LZFSE and ADC in magic (D8). An icon of APK in vector XML waits for
the display list of phase 2.

### Phase 2. The display list, the fonts and XPS (3.5 to 5 months)

`internal/display`, the okage device, the first part of `internal/font`, and
`xps`. iv drops `xpstopng`.

### Phase 3. PDF (6.5 to 10 months)

The port of hayro onto the display list, with its encryption, its fonts, its
image decoders and moxcms (D14). Illustrator files that are compatible with
PDF. iv drops libvips and `pdftoppm` for PDF.

### Phase 4. The layout and the ebooks (8 to 10.5 months)

`internal/layout`, the port of WeasyPrint (D15), and the work in okage that
it needs. Then EPUB, FictionBook, Markdown, HTML and text, then MOBI, AZW3,
AZW4 and CHM. MOBI and CHM wait for their decompression in magic (D8). iv
drops `ebook-convert` and `blitz`.

### Phase 5. The Office formats (11 to 18 months)

EMF and WMF, then the slides, the documents, the spreadsheets, RTF, the
previews and the text of the binary formats, and the encryption of
MS-OFFCRYPTO and OpenDocument. iv drops `soffice`.

### Phase 6. DjVu (1 to 2 months)

A copy of the decoders of DjVu from dejapdf, after the backlog examines
them.

The phases can overlap. Phase 2 and the start of phase 4 do not depend on
each other, and the previews of phase 1 can continue while phase 2 starts.
The long tail is not a phase. Each format of it waits until a user asks for
it (D1).

## Risks

`FORMATS.md` lists the risks, under "The risks". The largest one is the
estimate itself. The models disagree by six times, so phase 1 measures one
format of each kind before the maintainers commit to phases 3 to 5.

## Open questions

A question has a number, and it stays here until the maintainers answer it.
The answer becomes a decision. The questions were numbered again when the
decisions were consolidated (D4). The next question is question 2.

### Question 1. Is a build with TinyGo a goal that CI tests?

okage builds with TinyGo (okage D25). ivy reads XML with the parser of okage,
which builds with TinyGo (D7), so the largest obstacle is gone. A goal in CI
also forbids `reflect` and `encoding/json` in the packages that a user
imports, and each outside package must build with TinyGo. No phase waits for
the answer.
