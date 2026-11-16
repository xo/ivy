# Backlog

This document holds the work that is known and not done, with where it came
from. When an item is done, delete it, and record anything that was decided
in `docs/decisions/`.

## Units in progress

A unit is one package, or one file of a port with its tests. Claim a unit
here before you start it, with the date. When the change is committed,
delete the claim.

None.

## Before the first port

1. Write `docs/UPSTREAM.md`, the rules for a port and for a copy, before the
   first port or copy (D7). okage `docs/UPSTREAM.md` is the model. It lists
   hayro, moxcms and WeasyPrint, with a base commit for each, and the copies,
   such as oov/psd, mscfb and the codecs of dejapdf (D14, D15). Source: D7.

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
4. Export as public packages the parts of okage that ivy reuses: the XML
   parser in `internal/roxmltree`, the CSS parser in `internal/simplecss`,
   kurbo in `internal/kurbo`, WOFF, WOFF2 and EOT in `internal/webfont`, and
   brotli in `internal/brotli`. Source: D7.
5. Extend `html` with a parser of the HTML of web pages and ebooks, as okage
   D38 allows, with tinyhtml5 (MIT) as the reference. Source: D15.
6. Extend the CSS parser with what the layout of D15 needs, such as `@page`,
   `@media`, `@font-face` and the selectors of CSS 3, with tinycss2 and
   cssselect2 (BSD-3-Clause) as the reference. Source: D15.

## Work for magic

Each item is a request to magic (D8).

1. Make sure that magic tells these formats apart from the containers that
   hold them: XPS and OpenXPS from zip, an encrypted Office Open XML file from
   other Compound Files, AZW3 from MOBI, CBZ from zip, APK and IPA from zip,
   and Krita and OpenRaster from zip. Source: iv, which tells several of them
   apart by their extension today.
2. The decoders of LZX, LZFSE, ADC, PalmDoc and HUFF/CDIC in
   `magic/compress`. Source: D8.
