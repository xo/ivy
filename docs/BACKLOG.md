# Backlog

This document holds the work that is known and not done, with where it came
from. When an item is done, delete it, and record anything that was decided
in `docs/decisions/`.

## Units in progress

A unit is one package, or one file of a port with its tests. Claim a unit
here before you start it, with the date. When the change is committed,
delete the claim.

1. Phase 3, from 2026-11-16: `pdf`, `pdf/internal/render`, and the changes to
   `internal/display` and `xps` that PDF needs.

## Before phases 2 and 3 are pushed

1. okage tags a release with okage D47, the public `roxmltree`, and okage D48,
   the public `kurbo`. Then the `go.mod` of ivy requires that release, and the
   `go.work` that points at a checkout of okage is no longer needed. Until
   then CI fails, because okage v0.9.0 has neither package. Source: phases 2
   and 3.

## Left by phase 3

1. Draw the accented glyphs of a CFF font that the operator seac of a Type 2
   charstring builds. go-text draws nothing for them, so those characters are
   missing. Source: the port of hayro-font.
2. Read the Expert and ExpertSubset charsets of CFF, and the FontMatrix of each
   font dictionary of a CID-keyed CFF font and of a CFF table in OpenType.
   Source: the port of hayro-font.
3. Decode a JPEG that `image/jpeg` refuses: truncated, of 12 bits, or with
   arithmetic coding (D17). Source: the port of hayro-syntax.
4. If the maintainers decide again on hinting (D19), the measurement can run
   again: a small Rust program renders each page with hayro in
   `upstream/hayro`, with `RenderSettings.hinting` on and off, and a small Go
   program counts the pixels of ink that differ. Source: D19.
5. Draw a group that is not isolated onto its backdrop, and draw a knockout
   group. The device of okage draws each group as isolated, and not as
   knockout. hayro does the same. Source: the port of the crate hayro.
6. Read a PDF file through the `io.ReaderAt` of its source. `pdf.Open` reads
   the whole file into memory. Source: the package `pdf`.
7. Compare the pages of ivy with the pages of hayro in the test module. On
   2026-11-16, a scratch program rendered the 63 test files of
   `pdf/internal/interpret` with hayro, and ivy matched each page within a
   mean difference of 1.5. pdftoppm draws 11 of those pages in another way,
   so `TestPDF` does not compare them. Source: phase 3.

## Left by phase 2

1. Draw a VisualBrush and an OpacityMask of XPS. Source: phase 2.
2. Draw a sideways run of glyphs of XPS, `IsSideways`. Source: phase 2.
3. Decode the images of XPS in JPEG XR. No permissive decoder in Go is known.
   Source: phase 2.
4. Read the resolution of an image of an ImageBrush, from the pHYs of a PNG or
   the density of a JPEG. `xps` takes 96 dots per inch, unless the Viewbox
   starts at the origin. Source: phase 2.

## Left by phase 1

1. Describe the documents whose facts are in XML: the meta.xml of
   OpenDocument, the docProps/core.xml of Office Open XML, the stack.xml of
   OpenRaster, and the ComicInfo.xml of a comic. The XML parser of okage is
   public since phase 2, so this work can start. Source: phase 1.
2. Decode a TIFF preview of EPS with a palette of 2 or 4 bits, which
   ImageMagick writes and `golang.org/x/image/tiff` does not decode. `ps`
   returns `ErrUnsupported` for it. Source: phase 1.
3. Read the entries of a zip archive that ZipCrypto or the AES of WinZip
   encrypts, for a comic book archive (D12). `zipdoc` returns
   `ErrUnsupported` for such an entry. Source: phase 1.
4. Give `ivy.Cache` a way for a format package to store and load a value, for
   the fonts of phase 2. Source: phase 1.
5. Read the previews that ORF files keep in the maker notes of Olympus.
   `camera` reads only the previews of the IFDs. Source: phase 1.
6. Add oracles to the test module for EPS, with Ghostscript, and for the
   previews of OpenDocument, with LibreOffice (D13). Source: phase 1.
7. Add each package of `docs/upstream/upstreams.tsv` that a phase copies or
   ports, with its base commit, before its first commit: hayro, moxcms and
   WeasyPrint (D14, D15), mscfb, and the codecs of dejapdf. Source: D7.

## Sources to examine

Each source below was named in the survey and was not examined past its
license and its size. Examine each one before a decision names it. Source:
`docs/FORMATS.md`.

1. [sergev/dejapdf](https://github.com/sergev/dejapdf) (MIT). It holds DjVu
   decoders, and packages named `jbig2`, `jpx`, `ccitt` and `pdfread`. Find
   out which of them decode, and which only encode.
2. [mgilbir/gowemf](https://github.com/mgilbir/gowemf) (MIT). A pure Go
   reader of EMF, EMF+ and WMF from 2026-10. Find out how complete it is.
3. [blacktop/go-apfs](https://github.com/blacktop/go-apfs) (Apache-2.0) and
   [diskfs/go-diskfs](https://github.com/diskfs/go-diskfs) (MIT). Find out
   if they read from an `io.ReaderAt` without a file on disk.

## Work for okage

Each item is a request to okage (D8). Do not edit okage from this
repository. Ask a maintainer or the `okage` peer, as `AGENTS.md` says.

1. A fill of a mesh of triangles with colors that the fill interpolates, for
   the shadings of types 4 to 7 of PDF and PostScript. Source:
   `docs/FORMATS.md`, "What okage lacks".
2. Make sure that okage draws a soft mask of luminance and an isolated group,
   as PDF and XPS need. Source: `docs/FORMATS.md`.
3. Add a note to okage `docs/FORMATS.md` that the PDF reader, EPS and
   Illustrator go to ivy, and that okage draws their pages. Source: D14.
4. Export as public packages the parts of okage that ivy reuses: the CSS
   parser in `internal/simplecss`,
   kurbo in `internal/kurbo`, WOFF, WOFF2 and EOT in `internal/webfont`, and
   brotli in `internal/brotli`. Source: D7.
5. Extend `html` with a parser of the HTML of web pages and ebooks, as okage
   D38 allows, with tinyhtml5 (MIT) as the reference. Source: D15.
6. Extend the CSS parser with what the layout of D15 needs, such as `@page`,
   `@media`, `@font-face` and the selectors of CSS 3, with tinycss2 and
   cssselect2 (BSD-3-Clause) as the reference. Source: D15.

## Work for iv

Each item is a change to iv that ivy makes possible. Do not edit iv from this
repository. Ask a maintainer or the `iv` peer.

1. Replace the decoder `psd` of iv, which uses `oov/psd` and falls back to
   libvips, with the package `psd` of ivy, which decodes the files of 16 bits.
   Source: phase 1.
2. Add decoders for the previews of phase 1: `odf`, `docx`, `xlsx`, `pptx`,
   `iwork`, `ora`, `ps`, `camera` and `comic`, before the decoders that run
   `soffice`, `ebook-convert` or libvips. Source: phase 1.
3. Replace the decoder `gxps` of iv, which runs `xpstopng`, with the package
   `xps` of ivy. Source: phase 2.
4. Replace the decoders of iv that read PDF with libvips and `pdftoppm` with
   the package `pdf` of ivy. Source: phase 3.

## Work for magic

Each item is a request to magic (D8).

1. Make sure that magic tells these formats apart from the containers that
   hold them: XPS and OpenXPS from zip, an encrypted Office Open XML file from
   other Compound Files, AZW3 from MOBI, CBZ from zip, APK and IPA from zip,
   and Krita and OpenRaster from zip. Source: iv, which tells several of them
   apart by their extension today.
2. The decoders of LZX, LZFSE, ADC, PalmDoc and HUFF/CDIC in
   `magic/compress`. Source: D8.
