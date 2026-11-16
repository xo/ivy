# Backlog

This document holds the work that is known and not done, with where it came
from. When an item is done, delete it, and record anything that was decided
in `docs/decisions/`.

## Units in progress

A unit is one package, or one file of a port with its tests. Claim a unit
here before you start it, with the date. When the change is committed,
delete the claim.

None.

## Before a release of ivy

1. okage and magic tag the commits that the `go.mod` of ivy and of its test
   module require as pseudo-versions: okage at `73730383e33c` and magic at
   `5bda5bb2eaed`. Then the `go.mod` files require the tags. Source: the
   push of phases 1 to 6.

## Found by the visual review

The review command in `test/cmd/review` renders the corpus of
`test/review/corpus.tsv` beside reference programs. Its run of 2026-11-17
found these faults. Each item names a file of the corpus.

1. Give PostScript and EPS files their pages, which needs the interpreter
   of the long tail (D1). Five files of the corpus give no page. Source:
   `ps`.
2. Decide whether the page of the cover stretches the cover to the page,
   as calibre does by default. ivy keeps the aspect ratio of the cover, and
   the first page of each book with a cover differs by about 31. Source:
   `internal/ebook`.
3. Give `ripgrep-all/fb2/wasteland.fb2` its pages. ivy gives 30 pages and
   calibre gives 80. Source: `fb2`.
4. Leave out of the text of a FictionBook file the image of its coverpage
   when it is the first image of the annotation or the body, and give the
   table of contents that calibre gives. `fbreaderj/fb2/testfb2book.fb2`
   shows the cover twice. Source: `fb2`.
5. Lay out the cover part in SVG of `gutenberg/mobi/pg11-images-kf8.mobi`
   as calibre does, with grey bars above and below the image. Source:
   `mobi` and the layout.
6. Draw a stroke of 1 pixel or less as dark as MuPDF and Poppler do.
   `pdfjs/pdf/a1-plans.pdf` draws its thin lines lighter, because a line
   across a pixel border spreads over two light pixels. It needs stroke
   adjustment, which snaps a thin stroke to the pixels. Source: `pdf` and
   okage.
7. Map the codes of a symbolic TrueType font that a PDF does not embed,
   and that resolves by name, through its (3,0) cmap, not through the names
   of the glyphs. Source: `pdf`.
8. Do not advance for a code of a Unicode CMap that the CJK collection does
   not hold, as MuPDF does. ivy draws it with the width DW, and
   `pdfjs/pdf/text-cjk-nonembedded.pdf` moves every glyph after it. Source:
   `pdf`.
9. Read the properties of the characters and the paragraphs of Word 6 and
   Word 95, whose sprms have one byte, and return the text of PowerPoint
   95, which `msppt` refuses with ErrUnsupported. Source: `msdoc` and
   `msppt`.
10. Draw the pictures of a Word document: the inline pictures of the Data
   stream and the floating shapes of Office Drawing.
   `poi/doc/floating-pictures.doc` gives 6 pages, and LibreOffice gives 8.
   Source: `msdoc`.
11. Draw the shapes, the pictures and the charts of an Excel workbook.
   `poi/xls/drawings.xls` gives 1 page, and LibreOffice gives 10. Move the
   reader of Office Drawing of `msppt` into a package that `msxls` and
   `msdoc` share. Source: `msxls`.
12. Find why the columns of `msxls` are narrower than those of LibreOffice.
   `poi/xls/basic-expense-template-2011.xls` gives 44 pages, and
   LibreOffice gives 66. Source: `msxls` and `internal/sheet`.
13. Draw the date, the footer and the slide number of the master of a .ppt
   file, as its HeadersFooters atom says, and find why LibreOffice makes the
   body text of page 2 of `poi/ppt/defense-final.ppt` smaller. Source:
   `msppt`.
14. Draw the vertical text of the table cells of DrawingML (`a:tcPr vert="vert"`
   and `"vert270"`). `poi/pptx/table-test2.pptx` differs by 18. Source:
   `internal/drawingml`.
15. Shrink the text of a body with `a:normAutofit` to fit its box, as
   LibreOffice does. ivy uses the `fontScale` that the file stores. Source:
   `internal/drawingml`.
16. Give the columns of an xlsx sheet the widths of LibreOffice, so that the
   bands of columns break at the same column. `poi/xlsx/picture.xlsx`
   differs by 36 on pages 2 and 3. Source: `internal/sheet`.
17. Decide the orientation of a chart sheet without a page setup. ivy prints
   it landscape, as Excel does, and LibreOffice prints it portrait
   (`poi/xlsx/chart-sheet.xlsx`). Source: `xlsx`.
18. Find the blank page before the A5 section with page borders in
   `aspose/docx/rendering.docx`, which has 6 pages against 5. Source:
   `docx`.
19. Draw the check boxes of the form fields of docx. Source: `docx`.
20. Lay out the tab stops with leaders of the tables of contents of ODF,
   which `libreoffice/odt/ooo96040-2.odt` uses. It has 13 pages against 12.
   Source: `odf`.
21. Measure the height of the paragraph of a footer for the drawings that it
   places. `docx` estimates one line of 11.5 points. Source: `docx`.
22. Find why these files have pages that LibreOffice does not:
   `poi/docx/illustrative-cases.docx` (5 against 4),
   `poi/xlsx/conditional-formatting-samples.xlsx` (38 against 37) and
   `datasets/csv/world-cities.csv` (1470 against 1396). Source: `docx` and
   `internal/sheet`.
23. Find why the port lays out long HTML books on fewer pages than
    WeasyPrint 70.0, whose layout it ports, with the same style sheet:
    `weasyprint-samples/book/book.html` gives 56 pages against 61,
    `gutenberg/html/pg1342-images.html` 192 against 266, and
    `gutenberg/html/pg11-images.html` 58 against 75. The first pages
    match, so the cause is in a later part of the layout. Source:
    `internal/layout`.

## Left by the charts

1. Draw the treemap, the sunburst and the region map of Office 2016. They
   draw as a frame with their title. Source: `internal/chart` (D27).
2. Draw the trendlines, the error bars, the drop lines and the data tables
   of the charts of DrawingML. Source: `internal/chart`.
3. Place the categories of a date axis by their dates, with the gaps of the
   missing days. Source: `internal/chart`.
4. Draw the depth of a 3D chart. A 3D pie draws as a flat ellipse, and the
   other 3D charts as their 2D form. Source: `internal/chart`.

## Left by phase 6

1. Read the text layer (TXTa and TXTz), the outline (NAVM) and the links of
   the annotations of DjVu, when the API has a capability for them. Source:
   phase 6.
2. Decode the BG2k and FG2k layers of DjVu, which are JPEG 2000, with
   `internal/jpeg2000`. Source: phase 6.
3. Open a standalone IW44 file (FORM:BM44, FORM:PM44). Source: phase 6.
4. Open an indirect DjVu document when a program can give a folder, as D22
   does for Markdown. This needs a decision. Source: D26.
5. Read a bundled DjVu document from the source on demand, not whole into
   memory. Source: phase 6.

## Left by phase 5

1. Test the XOR obfuscation of Word with a file that Word wrote. Only a
   test that obfuscates `text.doc` itself covers it. Source: the
   encryption of phase 5.
2. Read an OpenDocument file of old StarOffice whose password hash takes
   the bytes of windows-1252, or has the fault of SHA-1 for some lengths.
   `odf` hashes the password as UTF-8. Source: the encryption of phase 5.
3. Draw a stroke whose mix mode (ROP2) combines the pen with the page, and a
   raster operation that combines a pattern brush with a source bitmap.
   `internal/metafile` draws the other raster operations and mix modes as
   blend modes, which are exact for black and white masks only. Source:
   `internal/metafile`.
4. Draw the text of a metafile that names glyphs by index in a font that
   ivy replaces, and the vertical text of EMF+ DrawString. The layout of
   DrawString is close to GDI+, not exact. Source: `internal/metafile` and
   `internal/gowemf`.
5. Draw the end cap of an EMF+ line apart from its start cap, and its custom
   caps. Source: `internal/metafile`.
6. Shape the text of complex scripts in a metafile. Bidi mirroring covers
   brackets only. Source: `internal/metafile`.
7. Read the lists, the headers and footers, the footnotes, the text boxes
   and the pictures of a `.doc` file. Source: `msdoc`.
8. Read the fonts, sizes and colors of the runs of a `.ppt` file, from
   StyleTextPropAtom, and its pictures, masters and backgrounds. Source:
   `msppt`.
9. Read the print areas and the print titles of a `.xls` file, which are
   NAME records, and its charts and pictures. Source: `msxls`.
10. Compare the pages of `msdoc`, `msxls` and `msppt` with LibreOffice in
    the test module. Source: `msdoc`, `msxls` and `msppt`.
11. Decode the XOR obfuscation of Word. It gives ErrUnsupported now.
    Source: `msdoc`.
12. Lay out a long sheet in parts, so that page 0 does not wait for the
    whole sheet. A sheet of 20,000 rows takes about 4 seconds before its
    first page. Source: `internal/sheet`.
13. Let the text of a cell overflow to the left, or to both sides, as Excel
    does for text that is aligned right or centered. The layout overflows to
    the right only. Source: `internal/sheet`.
14. Read the conditional formats, the comments and the shapes of `xlsx` and
    of the spreadsheets of `odf`, and print in the order over then down.
    Source: `xlsx` and `odf`.
15. Read the dates of a CSV file. An option for the separator of `csv`
    changes the API, so it is a decision first (D11). Source: `csv`.
16. Draw the effects of DrawingML other than the outer shadow, the blur of
    a shadow, and a compound line as more than one line. Source:
    `internal/drawingml`.
17. Read `fo:clip` of the pictures of OpenDocument, the objects that only an
    SVM picture shows, and the fields of the header and the footer of a
    master page. Source: `odf/internal/drawpage`.
18. Test the SmartArt drawings and a `.pptx` file that a password protects.
    Source: `pptx`.
19. Lay out the custom tab stops of Word, such as a right tab with dot
    leaders. The layout has only `tab-size`. Source: `docx`.
20. Read the vertical merges of cells and the nested tables of RTF, and
    apply its style sheet. Source: `rtf`.
21. Number the headings of OpenDocument by their outline, and draw the
    shapes in a text. Source: `odf`.
22. Draw the SmartArt and the canvases of a `.docx` file. Source: `docx`.
23. Push the body of a page below a header that is taller than its space.
    The converters estimate one line for each paragraph of the header.
    Source: `docx`, `odf` and `rtf`.
24. Lay out the text body of a shape as one `layout.Document` instead of a
    `layout.Book`, if that is cheaper. Source: `internal/drawingml`.
25. Keep the font database of a document for its charts. `fontdb.Of` reads
    an `fs.FS` that is not comparable again for each chart. Source:
    `internal/fontdb`.

## Left by phase 4

1. Replace the scan of tags in `internal/ebook` with the parser of HTML of
   okage, which okage holds since okage D49. The scan finds the title and the
   first image of a page in linear time. It is not a parser, but a second way
   to read HTML must not stay (D7). Source: phase 4.
2. Lay out an EPUB of fixed layout, with `rendition:layout` set to
   `pre-paginated` and a viewport on each page. Source: the package `epub`.
3. Return the publisher and the identifier of a book. `internal/ebook` reads
   them, and `ivy.Info` has no field for them. A new field changes the API,
   so it is a decision first (D11). Source: the package `epub`.
4. Read the NCX and the guide of a MOBI file, and name a font resource by its
   real type. Source: the package `mobi`.
5. Bound the time of the selector matcher of okage. A `:has()` selector
   costs time in the square of the number of elements. Source: stage A of
   the layout.
6. Number the pages of a book across its documents. In a `layout.Book`,
   `counter(page)` and `counter(pages)` restart in each document, because
   each document of the spine is one document of WeasyPrint. Source: stage
   D of the layout.
7. Give a page that a program asks for early the fixed boxes and the
   target counters of the later pages. `Document.Page` lays out only the
   pages up to the one that a program asks for. Source: stage C of the
   layout.
8. Find why `test_page_counters` breaks the text of a margin box into three
   lines where WeasyPrint gives one. Source: stage C of the layout.
9. Draw the color glyphs of SVG, PNG and COLR, and the bitmap glyphs of a
   font. `internal/font` gives the outlines of the glyphs only. Source:
   stage D of the layout.
10. Read the profiles of colors when the layout draws. DeviceCMYK, Lab and
    `@color-profile` become sRGB without a profile. Source: stage D of the
    layout.
11. Give the pages of every PDF of an AZW4 file. `mobi` gives the pages of
    the first one only. Source: the package `mobi`.

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
   ports, with its base commit, before its first commit: mscfb, and the
   codecs of dejapdf. Source: D7.

## Sources to examine

Each source below was named in the survey and was not examined past its
license and its size. Examine each one before a decision names it. Source:
`docs/FORMATS.md`.

1. [blacktop/go-apfs](https://github.com/blacktop/go-apfs) (Apache-2.0) and
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
4. Give the text API of okage what the layout needs: the advance and the
   byte offset of each cluster, the features, the language and the spacing of
   a run, the parsed face and its metrics, and the map from a weight to the
   axes of a variable font. The layout shapes with go-text itself, and it
   parses each face a second time. Source: stage B of the layout.
5. Let `svg` read the width, the height and the viewBox of an SVG image
   before it parses the image, take an element that is already parsed, and
   accept an image of size zero. The layout reads the size again with
   `roxmltree`, and it writes an inline `<svg>` back to XML. Source: stage B
   of the layout.
6. A spread mode of a gradient that paints nothing before the first stop
   and after the last stop, for the gradients of the layout. Source: stage
   D of the layout.
7. Make the antialiased `FillPath` and `StrokePath` safe with huge
   coordinates. With points near 1e12 device pixels, they panic with "slice
   bounds out of range" in `subPixmapMut.slice16AtXY`. A quadrilateral with
   points near 1.5e13 under `TransformFromScale(0.09, 0.09)` on a pixmap of
   32 by 32 shows it. `internal/metafile` leaves out such paths. Source:
   the fuzz test of `emf`.
8. A fast path in `DrawPixmap` for an image that is aligned to the pixels
   and drawn with the nearest filter. It takes about 40 percent of the time
   of a page of DjVu, in the highp pipeline. Source: phase 6.
9. A finer coverage of small glyphs in the rasterizer, with 8 by 8
   samples in each pixel, as the references of the visual review draw.
   `internal/display` now fills small glyphs at twice their size and
   averages them (D28). A change of the rasterizer of okage would differ
   from tiny-skia, so it needs a decision of okage. Source: D28.

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
5. Replace the decoders of iv that run `ebook-convert` or use `blitz` with
   the packages `epub`, `mobi`, `fb2`, `chm`, `html`, `text` and `markdown`
   of ivy. Source: phase 4.
6. Replace the decoders of iv that run `soffice` with the packages of
   phase 5: `docx`, `xlsx`, `pptx`, `odf`, `rtf`, `csv`, `msdoc`, `msxls`,
   `msppt`, `vsdx`, `pub` and `emf`. Source: phase 5.

## Work for magic

Each item is a request to magic (D8).

1. Make sure that magic tells these formats apart from the containers that
   hold them: XPS and OpenXPS from zip, an encrypted Office Open XML file from
   other Compound Files, AZW3 from MOBI, CBZ from zip, APK and IPA from zip,
   and Krita and OpenRaster from zip. Source: iv, which tells several of them
   apart by their extension today.
2. The decoders of LZFSE and ADC in `magic/compress`. magic holds the
   decoders of LZX, PalmDoc and HUFF/CDIC since magic D61. Source: D8.
