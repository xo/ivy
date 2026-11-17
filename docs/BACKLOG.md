# Backlog

This document holds the work that is known and not done, with where it came
from. When an item is done, delete it, and record anything that was decided
in `docs/decisions/`.

## Units in progress

A unit is one package, or one file of a port with its tests. Claim a unit
here before you start it, with the date. When the change is committed,
delete the claim.

1. `pdf`, from 2026-11-17: the option `WithPageBox` (D31), after the move
   of the PostScript interpreter, and the MediaBox rows of the review.

## Found by the visual review

The review command in `test/cmd/review` renders the corpus of
`test/review/corpus.tsv` beside reference programs. Its run of 2026-11-17
found these faults. Each item names a file of the corpus.

1. Add `--preserve-cover-aspect-ratio` to the calibre options of the
   review. calibre stretches a cover to the page, and ivy keeps its aspect
   ratio. Source: `test/review`.
2. Draw a stroke of 1 pixel or less as dark as MuPDF and Poppler do.
   `pdfjs/pdf/a1-plans.pdf` draws its thin lines lighter, because a line
   across a pixel border spreads over two light pixels. It needs stroke
   adjustment, which snaps a thin stroke to the pixels. Source: `pdf` and
   okage.
3. Map the codes of a symbolic TrueType font that a PDF does not embed,
   and that resolves by name, through its (3,0) cmap, not through the names
   of the glyphs. Source: `pdf`.
4. Do not advance for a code of a Unicode CMap that the CJK collection does
   not hold, as MuPDF does. ivy draws it with the width DW, and
   `pdfjs/pdf/text-cjk-nonembedded.pdf` moves every glyph after it. Source:
   `pdf`.
5. Return the text of PowerPoint 95, which `msppt` refuses with
   ErrUnsupported. Source: `msppt`.
6. Draw the text boxes of a Word 97 document, whose text is in the story of
   the text boxes after the headers, and the shapes of Office Drawing that
   are not pictures. `poi/doc/with-art-shapes.doc` has WordArt and shapes,
   and `libreoffice/doc/landscape-a4-certificate.doc` has text boxes. The
   header of `libreoffice/doc/landscape-letter-form.doc` has a text box, and
   Word puts the text of the body below it, so the form has 4 pages against
   3 in ivy. Source: `msdoc`.
7. Draw the shapes and the charts of an Excel 97 workbook, and the
   pictures in DIB and PICT. `msxls` shows the pictures in PNG, JPEG, EMF
   and WMF, and a frame for the other objects. `poi/xls/drawings.xls` gives
   9 pages, and Excel gives 10. Move the reader of Office Drawing of
   `msppt`, with its pictures, into a package that `msxls`, `msppt` and
   `msdoc` share. `msxls/drawing.go` holds a second, smaller reader of the
   pictures until then. Source: `msxls`.
8. Find the pages that `poi/xls/basic-expense-template-2011.xls` has more
   or fewer than in Excel: 69 against 66. The sheet ExpenseTypes gives 3
   blank pages, EmailXpenC gives 28 pages against 30, and the pictures of
   Receipt images give 25 pages against 22. Source: `msxls` and
   `internal/sheet`.
9. Read the HeadersFooters atom of a .ppt file, which shows or hides the
   date, the footer and the slide number of the master. ivy draws the
   shapes of the master as they are. No file of the corpus shows a fault:
   `poi/ppt/defense-final.ppt` has the slide numbers of PowerPoint.
   Source: `msppt`.
10. Read the region records of the EMF picture of page 21 of
    `poi/ppt/defense-final.ppt`. The reader stops with "byte 1844: region
    data size: malformed metafile", and the plot does not draw. Inkscape
    draws it. Source: `internal/gowemf`.
11. Lay out the tab stops with leaders of the tables of contents of ODF,
    which `libreoffice/odt/ooo96040-2.odt` uses. It has 13 pages against 12.
    Source: `odf`.
12. Find why these files have pages that LibreOffice does not:
    `poi/xlsx/conditional-formatting-samples.xlsx` (37 against 36 in Excel,
    and 37 in LibreOffice) and `datasets/csv/world-cities.csv` (1470 against
    1396). Source: `internal/sheet`.
13. Keep empty paragraphs at the bottom of a page when they do not fit, as
    Word seems to do. On page 3 of `poi/docx/illustrative-cases.docx`, three
    empty paragraphs after a table need about 2 points more than the page
    has left. Word keeps them on page 3. ivy moves one of them to page 4,
    and the empty paragraphs at the end of page 4 then go to a page 5: 5
    pages against 4. The rule of Word is not known. Source: `docx` and
    `internal/layout`.
14. Decide whether a text file reflows its paragraphs as calibre does.
    calibre joins the lines of a paragraph, and ivy keeps them. Source:
    `text`.
15. Drop an undefined entity of a FictionBook file, as calibre does.
    `fbreaderj/fb2/minihelp.ja.fb2` shows `&FBReaderVersion;` as text.
    Source: `internal/ebook`.
16. Find how PowerPoint applies `a:satMod` after `a:tint`. The gradients
    of the diagram of page 11 of `poi/pptx/custom-geometry.pptx` are
    lighter in ivy than in PowerPoint. A tint alone, such as the bands of
    the default table style, has the colors of PowerPoint. Source:
    `internal/drawingml`.
17. Draw a funnel chart (`cx:chart` with the layout `funnel`) with its
    first series only, as bars in the middle with gaps between them, as
    PowerPoint draws `libreoffice/pptx/funnel-chart.pptx`. ivy draws the
    three series over each other. Source: `internal/chart`.
18. Decide the pattern fill and the texture fill of
    `poi/ppt/backgrounds.ppt`. PowerPoint draws the pattern of page 2 red
    with green crosses, from a picture of the store that is a photo, and
    draws the texture of page 3 white. ivy draws the pattern as the mix of
    its two colors and tiles the texture, as MS-ODRAW describes them.
    Source: `msppt`.
19. Shade a gradient of PowerPoint 97 to 2003 with the sigma curve of
    `fillShadeType`. ivy mixes the colors in linear light, as its gamma
    flag says. Near their ends, the gradients of
    `poi/ppt/defense-final.ppt` stay closer to the colors of the ends in
    PowerPoint. Source: `msppt`.
20. Find when PowerPoint kerns. The lines of Calibri in
    `poi/pptx/custom-geometry.pptx` break as without kerning, even with
    `kern="1200"`, and the lines of Times New Roman in
    `poi/ppt/defense-final.ppt` are as wide as with kerning. Perhaps
    PowerPoint reads only the table `kern` of a font, which Calibri does not
    have. ivy draws the text of the slides without kerning. Source:
    `internal/drawingml`.
21. Give a row with a height in which no cell ends its height. The layout
    gives it the height 0, as WeasyPrint does, and `internal/sheet`
    carries the heights of such rows to the last row of a merged cell. The
    layout and `internal/sheet` must change together: the layout alone
    makes `TestMergedRows` of `internal/sheet` give 7 pages against 4. A
    merged cell with text that wraps into more lines than its last row
    holds still makes that row higher. `poi/xls/landscape-a4-drawing.xls`
    gives 4 pages against 2, because of the labels of 2 and 3 lines in the
    merged cells of its form. Source: `internal/layout` and
    `internal/sheet`.
22. Mark the pages of Excel of the files on A4 as a fault of the reference.
    Excel 2024 prints `poi/xlsx/picture.xlsx`,
    `poi/xlsx/landscape-a4-tables.xlsx`, `poi/xls/landscape-a4-drawing.xls`
    and `poi/xls/landscape-a4-japanese.xls` on US Letter, which the files
    do not ask for. The cause is probably the option "Scale content for A4
    or 8.5 x 11 paper sizes" of Excel on the machine of the review, which
    nobody checked. Legal stays Legal. ivy prints them on A4, as their
    paperSize 9 says (ECMA-376 Part 1, 18.3.1.63, and MS-XLS). Source:
    `test/review/reference-faults.tsv`.
23. Draw the WMF pictures of `poi/xls/landscape-a4-drawing.xls`, which
    draw blank. A WMF picture of the store of Office Drawing has no
    placeable header, and its size is in its OfficeArtMetafileHeader. The
    cause is not checked. Source: `msxls` and `internal/metafile`.
24. Find the width of a digit of Century Gothic, which no font of the
    review holds. Excel prints `poi/xlsx/landscape-letter-budget.xlsx` at
    83% on 1 page, and ivy at 100% on 2 pages, with digits of 0.55 of the
    size. Source: `internal/sheet`.
25. Read the record HeaderFooter of BIFF8, whose flag fHFScaleWithDoc can
    keep the header and the footer at their size. `msxls` scales them with
    the cells, as Excel does by default. Source: `msxls`.
26. Draw the shadow of the borders of a page, `w:shadow` of `w:pgBorders`,
    as Word does on the A5 page of `aspose/docx/rendering.docx`. Source:
    `docx`.
27. Place a VML shape that a paragraph anchors from the top of the part of
    the paragraph on its page. On page 2 of
    `libreoffice/docx/mixed-orientation.docx`, Word draws the rectangle 81
    points below the top of the page area, where the paragraph goes on. ivy
    draws it 81 points below the line of its anchor. Source: `docx`.
28. Draw the WordArt watermark of `libreoffice/rtf/watermark.rtf`, a shape
    of `\shp` with the text "ASAP", which Word draws across the page.
    Source: `rtf`.
29. Find why Word shows no line for the empty paragraph after the text of a
    cell, such as the cell "Jan 2009 – Dec 2009" on page 2 of
    `libreoffice/doc/landscape-letter-form.doc`. ivy gives the paragraph a
    line, and the row is higher than in Word. Source: `msdoc`.

## Left by the charts

1. Draw the treemap, the sunburst and the region map of Office 2016. They
   draw as a frame with their title. Source: `internal/chart` (D27).
2. Draw the trendlines, the error bars, the drop lines and the data tables
   of the charts of DrawingML. Source: `internal/chart`.
3. Place the categories of a date axis by their dates, with the gaps of the
   missing days. Source: `internal/chart`.
4. Draw the depth of a 3D chart. A 3D pie draws as a flat ellipse, and the
   other 3D charts as their 2D form. Source: `internal/chart`.

## Left by the CJK fonts of the system

On 2026-11-17 the layout and `internal/fontdb` came to take the East Asian
fonts of the system, by their local names too, and a system face of the
language of the text for a CJK character, when `Config.SystemFonts` is on.

1. Give the layout the East Asian font and the language of ODF:
   `style:font-name-asian` and `fo:language-asian` in the text, and
   `style:font-name-asian` in the drawing pages, as an East Asian font in
   font-family and as `lang` on the runs of CJK text, as `docx` does.
   Source: `odf`.
2. Pass the East Asian font (`sprmCRgFtc1`) and the language
   (`sprmCRgLid1`) of the characters of `msdoc`, and `\dbch` with the
   `\fcharset` of a font (128, 134, 136, 129) of `rtf`, in the same way.
   Source: `msdoc` and `rtf`.
3. Read the language of the runs of PowerPoint 97 (`lang` and `altLang` of
   TextSIException). Source: `msppt`.
4. Read the script fonts of a theme (`a:font script="Jpan"`, `"Hans"`,
   `"Hant"` and `"Hang"`). Word and PowerPoint use them for `minorEastAsia`
   when the `a:ea` of the theme is empty. `aspose/docx/asian-typography.docx`
   needs them. Source: `internal/drawingml`.
5. Find the language of a text or Markdown file, from its text or from the
   header of a Project Gutenberg file. Without it, Chinese text takes the
   Japanese faces first (`gutenberg/txt/pg24264.txt`). Source: `text`.
6. Make a bold face for a font without one, such as SimSun, so that the CJK
   headings draw bold (`fbreaderj/fb2/minihelp.zh.fb2`). Source: the layout.
7. Add the space between Latin and CJK text of `w:autoSpaceDE` and
   `w:autoSpaceDN`, as LibreOffice does. Source: `docx` and the layout.

## Left by the pages of Visio

The pages of Visio came on 2026-11-17 (D30).

1. `internal/gowemf`: draw an EMF+ DrawImage whose source rectangle is
   outside the image when the wrap mode is not a transparent clamp. Clamp or
   tile the source as GDI+ does. Three of the five pictures of
   `poi/vsdx/60973.vsdx` draw blank because of it.
2. `vsdx`: compute more ShapeSheet formulas, such as IF, THEMEVAL with
   arguments, and references to other shapes (`Sheet.N!Cell`). The package
   uses the V values, and computes only simple arithmetic.
3. `vsdx`: draw an OLE object that has no picture, and the theme effects
   other than the shadow: the bevel, the glow and the reflection.
4. `vsdx`: test a stencil (.vssx), which has no pages.

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
7. Read the footnotes and the text boxes of a `.doc` file, and the
   headers, the footers, the lists and the pictures of a file of Word 6 or
   Word 95. Source: `msdoc`.
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
    `odf` and `rtf` estimate one line for each paragraph of the header.
    `docx` and `msdoc` estimate the lines of the text of each paragraph and
    the rows of the tables, with a mean width of the characters. The layout
    could measure the margin boxes instead. Source: `odf`, `rtf` and
    `internal/layout`.
24. Lay out the text body of a shape as one `layout.Document` instead of a
    `layout.Book`, if that is cheaper. Source: `internal/drawingml`.
25. Keep the font database of a document for its charts. `fontdb.Of` reads
    an `fs.FS` that is not comparable again for each chart. Source:
    `internal/fontdb`.
26. Find why the fuzz test of `msppt` stalls after about 17 inputs. A copy
    of the code before the styles of 2026-11-17 stalls the same way, and the
    input that the fuzzer saved runs in 5 ms. A file must never make ivy
    hang (hard rule 10). Source: `msppt`.

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

1. Draw a sideways run of glyphs of XPS, `IsSideways`. Source: phase 2.
2. Decode the images of XPS in JPEG XR. No permissive decoder in Go is known.
   Source: phase 2.
3. Read the resolution of an image of an ImageBrush, from the pHYs of a PNG or
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
10. Let a program look up a character in a face with the cmap that okage
    caches. `DefaultFallbackSelector` and ivy parse the cmap again for each
    lookup. Source: the CJK fonts of the system, 2026-11-17.

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
