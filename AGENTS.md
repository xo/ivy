# ivy

`ivy` decodes, renders and encodes the file formats that a program shows as
images, in pure Go (D1): ebooks, Office documents, PDF, XPS, PostScript, EPS,
Adobe Illustrator, Photoshop, and the containers that carry an icon or a
preview, such as a DMG file. A program that imports ivy needs the Go toolchain
and nothing else. ivy draws with [okage][okage], and it relies on
[magic][magic] to find the type of a file (D8). [iv][iv], an image viewer for
the terminal, is the first program that uses it.

A rendering is good enough when a person can review the document with it
(D5). ivy does not try to render a document exactly as the program that made
it.

On 2026-11-17 phases 1 to 6 are done: the document API, the previews and
pages of the formats of phase 1, the display list, the fonts, XPS, PDF, the
layout of HTML and CSS, the ebooks, the Office formats, and DjVu. The survey of the formats is in
`docs/FORMATS.md`, and the open questions are at the end of `docs/PLAN.md`.

[okage]: https://github.com/xo/okage
[magic]: https://github.com/xo/magic
[iv]: https://github.com/kenshaw/iv

## Standing rules

These rules hold in every `xo` repository, for every coding agent (dbmeta
D110, which D2 adopts).

1. Stage changes for review. Commit and push only when a maintainer says so.
2. Load the `simple-english` skill before you write any text that a person
   reads: project documentation, a code comment, an error message or a commit
   message. Follow it for that text.
3. In a Go project, load the `go-pedantry` skill before you write or review Go
   code. A rule in this file wins where the two conflict.

`CLAUDE.md` holds one line that imports this file, so that Claude Code and
every other agent read the same rules. Edit this file, not `CLAUDE.md`.

## Which document to read

| If you are | Read |
| --- | --- |
| asking why something is the way it is | the index in [docs/decisions/README.md](docs/decisions/README.md) |
| asking what the plan is, or what is not decided | [docs/PLAN.md](docs/PLAN.md). The open questions are at its end |
| asking which formats ivy supports, how, and at what cost | [docs/FORMATS.md](docs/FORMATS.md) |
| writing or changing the public API, or naming a type or a function | [docs/API.md](docs/API.md), then D11 |
| adding a format, or starting work on one | `docs/FORMATS.md`, then "The layout of the packages" in `docs/API.md`, then D6, D10 and D11 |
| naming a package, or giving a package a module of its own | D6 |
| working on PDF, EPS or Illustrator | D14, then the decisions that amend it: D16, D17 and D20. Then D18 and D19 |
| working on the PDF writer, `pdf/pdfenc` | D58, then D10 and okage D68 |
| working on the layout of HTML and CSS | D15, then D7 |
| reading XML, HTML or CSS, or about to port or copy any code | D7. Use what okage or magic has first |
| needing a decoder of compressed data | D8 |
| adding an encoder, or a format of the long tail | D1, then D6 |
| adding an outside package, or copying or porting code from another project | D7. Then [docs/UPSTREAM.md](docs/UPSTREAM.md) and `docs/upstream/upstreams.tsv` |
| adding a comparison with an oracle, or running one | D13, then `test/doc.go` |
| needing a feature of okage or of magic | D8, then "Work for okage" or "Work for magic" in [docs/BACKLOG.md](docs/BACKLOG.md), then "Other repositories" below |
| writing generic code, or a hot loop that can use SIMD | D9 |
| working on a file that a password protects, or on DRM | D12 |
| looking for work that is known and not done | [docs/BACKLOG.md](docs/BACKLOG.md) |
| writing a document, a code comment, an error message or a commit message | the `simple-english` skill. Load it first. Then "Writing documentation" below |
| writing or reviewing Go code | the `go-pedantry` skill. Load it first. Then "Hard rules" below |
| adding or updating an agent skill | "Agent skills" in [CONTRIBUTING.md](CONTRIBUTING.md) |

`README.md` is for a person who uses ivy. `CONTRIBUTING.md` is for a person
who changes it, and it is shorter than this file.

A document that is not in that table does not exist. If you cannot find where
something is written down, it is not written down. Ask a maintainer. Do not
decide it yourself, and do not write it as though it were settled. Add the
question to the end of `docs/PLAN.md`.

## Decision numbers

A bare number, such as D3, names a decision of this repository, in
`docs/decisions/`. A decision of another repository names that repository,
such as dbmeta D110 or okage D16. An open question has a number, and it is at
the end of `docs/PLAN.md` until the maintainers answer it. The numbers do not
repeat. The next decision has the number 60, and the next question is
question 24.

## Hard rules

1. Code follows the plan and the decisions. A change to the plan is a
   decision first. If the plan does not say how to do something, ask a
   maintainer. No code is written before the plan is ready (D3).
2. The module that a user imports is pure Go. It does not use cgo, it does
   not link a library of C, C++ or Rust, it does not run WebAssembly, and it
   does not run an outside program (D1). Only the test module in `test/`,
   which has its own `go.mod`, can use cgo or run an outside program (D13).
3. Each decoder and each encoder is a package of its own. The root package
   imports no format. No package imports every format. The decoder is
   `<format>/`, and the encoder is `<format>/<format>enc/` (D6).
4. Use what okage or magic holds before you port or copy code. Do not write
   a second copy of it, such as a parser of XML, HTML or CSS. Ask them for
   what is missing (D7, D8).
5. Do not add a Go package from outside the `xo` repositories without the
   approval of the maintainers, and that includes a package that only a test
   imports. An outside package must be pure Go and under a permissive
   license. The first choice is to copy it into `internal/`, with its license
   (D7).
6. Never copy, port or import code under the GPL, the LGPL or the AGPL, such
   as calibre, LibreOffice, Ghostscript, poppler, MuPDF, libgxps or
   DjVuLibre. A test of the test module can run them as oracles (D7, D13).
7. ivy draws with okage and identifies with magic. Do not write a second
   renderer or a second set of rules for the types of files. If ivy needs a
   change in okage or in magic, record it in `docs/BACKLOG.md` and ask (D8).
8. Read XML with the XML parser of okage, never with `encoding/xml` (D7).
9. `upstream/` in the root holds the checkouts of the projects that ivy ports
   or copies, one folder for each. Do not edit a file in them and do not
   commit them. Git ignores the folder.
10. A file must never crash ivy or make it hang. Each decoder takes its
    limits from `ivy.Config`, and has a fuzz test (D5).
11. Code for a DRM scheme that needs a key, such as Adobe ADEPT, Kindle or
    Readium LCP, goes only in the module `drm`. A program gives the key, and
    ivy never searches for one. No decoder of a format imports that module
    (D12).
12. A public name follows `docs/API.md`. A change to the public API is a
    decision first (D11).
13. Posts to GitHub follow three rules:
    - A post to `github.com/xo/ivy` is allowed.
    - An issue or a discussion in another `github.com/xo` repository, or in
      `github.com/kenshaw/iv`, is allowed only after a maintainer approves
      that post. Do not open a pull request or write a comment there.
    - Do not post anything to a third party repository, such as hayro,
      webrender or a project that ivy copies: no issue, comment, pull
      request, review, discussion or request.

    Reading any repository, for example with `gh api`, is allowed.

When the maintainers answer a new question, the answer becomes a decision,
and a rule that comes from it goes here.

## Other repositories

No Claude Code session exists for okage, magic or iv (D50). iv uses ivy
for its documents.

1. If ivy needs a change in okage or in magic, record it in "Work for okage"
   or "Work for magic" of `docs/BACKLOG.md`, and tell a maintainer.
2. If a maintainer asks for the change, make it in a checkout or a worktree
   of that repository. Follow the `AGENTS.md` and the decisions of that
   repository.
3. Commit, tag and push okage or magic only when a maintainer says so.

## Layout

On 2026-11-16 the repository holds these files:

| Path | What it holds |
| --- | --- |
| `README.md`, `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md` | the four documents in the root |
| `LICENSE` | the MIT license of ivy. A copied or ported package keeps the license of its source in its own folder (D7) |
| `go.mod` | the module `github.com/xo/ivy` |
| `skills-lock.json`, `.agents/skills/`, `.claude/skills/` | the agent skills |
| `.gitignore`, `.gitattributes` | the files that git reads |
| `docs/PLAN.md` | the plan and the open questions |
| `docs/API.md` | the design of the public API and the expected layout of the packages |
| `docs/FORMATS.md` | the survey of the formats, with the estimates of the outside models |
| `docs/BACKLOG.md` | the work that is known and not done, and the work for okage and magic |
| `docs/UPSTREAM.md` | the rules for code that ivy copies or ports |
| `docs/upstream/upstreams.tsv` | the projects that ivy copies or ports, with their commits |
| `docs/decisions/` | the decisions, one file each, with an index |
| `.golangci.yml`, `.github/workflows/test.yml` | the linters and CI |
| `*.go` in the root | the package `ivy`: the document API of D11, and the tests of the documents, the skills and the imports |
| `odf/`, `docx/`, `xlsx/`, `pptx/`, `iwork/`, `ora/`, `ps/`, `camera/`, `comic/`, `psd/` | the formats of phase 1, each with its tests and its `testdata/`. Phase 5 gives the pages of `odf`, `docx`, `xlsx` and `pptx`: the texts and the sheets through the layout, and the slides and the drawings on the display list |
| `rtf/` | RTF, as RTF 1.9.1 describes it, converted into HTML and CSS for the layout |
| `xps/` | XPS and OpenXPS, the format of phase 2 |
| `pdf/` | PDF and Illustrator compatible with PDF, the format of phase 3 |
| `pdf/pdfenc/` | the PDF writer of phase 7: the pages of a document as a PDF file, through the PDF writer of okage, with the PDF device that maps the display list onto it (D58) |
| `internal/hayro/` | the ports of the crates of hayro, which `pdf` and `ps` share (D14, D16, D20, D29) |
| `ps/internal/` | the interpreter of PostScript, which gives the pages of PostScript and EPS files (D29) |
| `epub/`, `html/`, `text/`, `fb2/`, `markdown/`, `mobi/`, `chm/` | the ebooks and the documents of text of phase 4, each with its tests and its `testdata/` |
| `internal/layout/` | the port of the layout and the drawing of WeasyPrint (D15, D21), and the book that puts a whole ebook on pages |
| `internal/ebook/`, `internal/memfs/`, `internal/charset/` | the code that the ebooks share: the style sheets, the metadata, the text encodings and the pages, a file system in memory, and the encodings of the WHATWG Encoding Standard |
| `internal/goldmark/` | the copy of goldmark, for Markdown (D7, D21) |
| `emf/` | EMF, EMF+ and WMF, with EMZ and WMZ, the first format of phase 5 |
| `internal/gowemf/`, `internal/metafile/` | the copy of github.com/mgilbir/gowemf (D7, D24), and the playback of a metafile onto the display list, which the Office formats call for an embedded EMF or WMF image |
| `djvu/` | DjVu, the format of phase 6. `djvu/internal/` holds the copy of the decoders of dejapdf: `iff`, `zp`, `bzz`, `jb2`, `iw44` and the document model `doc` (D26) |
| `internal/drawingml/` | the shapes of DrawingML and the slides: the tables of the preset shapes, made from `presetShapeDefinitions.xml` of ECMA-376, which `testdata/ecma` holds with the copyright notice of Ecma (D24), the colors, fills, lines, pictures and text bodies, and the scene that draws a page of fixed positions, with its text through `internal/layout` |
| `odf/internal/drawpage/` | the pages of the presentations and the drawings of OpenDocument, on the display list |
| `msdoc/`, `msxls/`, `msppt/` | Word, Excel and PowerPoint 97 to 2003: the text and the sheets on pages, the summary information and its thumbnail, and RC4 encryption (D12, D24) |
| `vsdx/` | Visio: the pages of the files of Office Open XML on the display list, with the masters, the styles and the theme (D30), and the thumbnail and the properties of every Visio file |
| `pub/` | Publisher: the thumbnail and the properties |
| `internal/odraw/` | the shapes, the properties and the pictures of Office Drawing (MS-ODRAW), which `msppt` and `msxls` share |
| `internal/msole/` | the code that the binary Office formats share: the Compound File within the limits, the summary information, the thumbnails in WMF, EMF, DIB, PNG or JPEG, and the password |
| `internal/pict/` | the pictures of QuickDraw (PICT) in the store of Office Drawing: their bitmaps and their images of QuickTime, without their lines, shapes and text |
| `csv/` | CSV, TSV and TAB files, as one table on pages |
| `internal/sheet/`, `internal/numfmt/` | the sheets of a spreadsheet as HTML tables on pages, for `xlsx`, `odf`, `csv` and `msxls`, the sizes of the papers, the widths of the columns and the headers and footers that they share, and the number formats of Excel |
| `internal/fontdb/` | the font database of a program, from `ivy.Config.Fonts`, the Go fonts and the fonts of the system, which the layout, the metafiles and the charts share (D23, D24) |
| `internal/chart/` | the charts of DrawingML, of Office 2016 (cx:chartSpace) and of OpenDocument: ivy lays them out and draws them onto the display list as Office and LibreOffice do, with the scales of the axes of Excel and the fonts of fontdb (D27) |
| `internal/offcrypto/`, `odf/internal/odfcrypt/` | the encryption of MS-OFFCRYPTO (Agile, Standard, RC4 CryptoAPI and RC4), and the encryption of OpenDocument (D12, D24) |
| `fonts/` | the module `github.com/xo/ivy/fonts`: Liberation, Carlito and Caladea, with the metrics of the fonts of Office, under the SIL Open Font License (D24) |
| `internal/ccitt/`, `internal/jbig2/`, `internal/jpeg2000/`, `internal/moxcms/` | the codecs and the color management of the port of hayro (D16, D17) |
| `internal/display/`, `internal/font/` | the display list and the fonts of D10. The layout also reads EMF and WMF, and draws the objects of a format, such as the charts |
| `psd/internal/oovpsd/` | the copy of github.com/oov/psd (D7) |
| `internal/raster/`, `internal/zipdoc/`, `internal/ooxml/` | the code that the formats share: raster images and pages, zip archives, Office Open XML |
| `internal/fixture/` | the files that the tests build in memory |
| `test/` | the test module, with the oracles (D13) |

Code goes where "The layout of the packages" in `docs/API.md` says (D6).

## Before you stage

Standing rule 1 applies. Stage the change with `git add`, and give a
maintainer a proposed commit message. Do not commit and do not tag.

Work goes in units: one package, or one file of a port with its tests. Claim
a unit under "Units in progress" in `docs/BACKLOG.md` before you start. When
the change is committed, delete the claim.

Before you stage a change to a document, make sure of these facts:

1. The text follows the `simple-english` skill, including its self-check.
2. A new document is in the table of this file and in the list of `README.md`.
3. A new decision has a row in `docs/decisions/README.md`.
4. A link to another document of this repository goes to a file that exists.

On 2026-11-19 the `go.mod` of ivy requires okage v0.17.0 and magic v0.9.1.
If a change needs a commit of okage or magic that is pushed and not tagged,
require it as a pseudo-version. When okage and magic tag that commit,
require the tag. A change that needs a commit of okage or magic that is not
pushed builds through a `go.work` that points at the checkouts next to ivy.
Git ignores `go.work`. Make it with
`go work init . ./test ./fonts ../okage ../magic`. Run the checks without it,
with `GOWORK=off`, before you push, as CI does.

These commands must pass before you stage a change to Go code. They cover
the scalar build, the SIMD build (D9), the race detector (D11), the test
module, which needs the oracles (D13), and the targets that CI builds:

```sh
test -z "$(gofmt -l $(git ls-files -co --exclude-standard '*.go'))"
go vet ./... && GOEXPERIMENT=simd go vet ./...
go test -race -count=1 ./...
GOEXPERIMENT=simd go test -count=1 ./...
golangci-lint run ./...
(cd test && go vet ./... && go test -count=1 ./... && golangci-lint run ./...)
(cd fonts && go vet ./... && go test -count=1 ./... && golangci-lint run ./...)
for t in freebsd/amd64 linux/386 linux/arm linux/riscv64 linux/s390x \
  windows/386 js/wasm wasip1/wasm; do
  CGO_ENABLED=0 GOOS=${t%/*} GOARCH=${t#*/} go build ./... || echo "$t failed"
done
```

Run them with the version of golangci-lint that `.github/workflows/test.yml`
installs. On 2026-11-18 it is v2.15.0. Another version reports other
issues, and CI then fails on a change that passed here. Install that version
with `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0`.
The targets of 32 bits, such as linux/386, fail on a constant that does
not fit in an `int` of 32 bits, such as `1 << 31` or `math.MaxUint32`.

Before you stage a change to a decoder, run its fuzz test for a minute, such
as `go test -run '^$' -fuzz FuzzOpen -fuzztime 1m -fuzzminimizetime 2s ./psd`
(D41).

## Writing documentation

Only `README.md`, `AGENTS.md`, `CLAUDE.md` and `CONTRIBUTING.md` go in the
repository root. A new document goes in `docs/`. Add it to the table at the
top of this file and to the list in `README.md`.

A decision goes in a file of its own in `docs/decisions/`, with the next
number (D4). Name it `D<nnn>-<title>.md`, with the number in three digits. It
opens with `# D<n>. <title>`, a blank line, and `Status: <status>.` The status
is `Decided` or `Proposed`. If the decision changes an earlier one, the status
says `Amends D<n>`, and the status of the earlier one says `Amended by D<m>`.
Add its row to `docs/decisions/README.md`.

Write `Decided` only when the maintainers chose it. If you do not know, write
`Proposed`, and add the question to the end of `docs/PLAN.md`.

Do not name a person in a document of this repository. Write "the
maintainers" or "a maintainer".

Work that is known and not done goes in `docs/BACKLOG.md`, with where it came
from. When an item is done, delete it, and record anything that was decided in
`docs/decisions/`.

Standing rule 2 applies to every such text. In brief: short sentences, the
active voice, no contractions, no semicolons and no em dashes. Use `can`,
`will` and `must`, never `should`, `may` or `might`. Put the condition before
the command.
