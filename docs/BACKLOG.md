# Backlog

This document holds the work that is known and not done, with where it came
from. When an item is done, delete it, and record anything that was decided
in `docs/decisions/`.

## Units in progress

A unit is one package, or one file of a port with its tests. Claim a unit
here before you start it, with the date. When the change is committed,
delete the claim.


## Found by the visual review

The review command in `test/cmd/review` renders the corpus of
`test/review/corpus.tsv` beside reference programs. Its runs of 2026-11-17
found these faults. Each item names a file of the corpus.

1. Remove the white seam where two images of `pdfjs/pdf/a1-plans.pdf` meet at
   x = 396.6 pt. The images are in tiling patterns. The edge of the fill of
   each pattern, the clip in its cell and the edge of its image are
   antialiased at the same part of the same column of pixels, and their
   coverages multiply. MuPDF snaps a rectangular clip and the edges of an
   image to whole pixels. A test that moved the edges of each image out to
   whole pixels made the seam darker, but it stretched the images and failed
   the oracle test `image_inline_2`. Source: `internal/hayro/render`,
   2026-11-18.
2. Draw the pictures in PICT of an Excel 97 workbook. Source: `msxls`.
3. Find the last 3 pixels of drift of `libreoffice/odt/ooo96040-2.odt` by page
   8, which move one bullet to page 9 and give 13 pages against 12.
   LibreOffice rounds the ascent and the descent of a font to twips at each
   size, so the lines of Liberation Sans at 10 points are 11.15 points, and
   ivy takes 11.17 points from a ratio. Source: `odf`.
4. Check whether an empty last line of a table cell can pass the bottom of a
   page by its descent in Word or LibreOffice. `-ivy-page-overflow` (D52) does
   not apply to the lines of a table cell, a float, a flex or grid item, an
   inline block or a column, because their parents check the bottom of the
   page without the descent. If it can, the row check of `layout_table.go` and
   the parents of a table must subtract the descent too. Source:
   `internal/layout` and D52.
5. Find why Excel in the Windows VM of the review prints
   `poi/xlsx/picture.xlsx`, `poi/xlsx/landscape-a4-tables.xlsx`,
   `poi/xls/landscape-a4-drawing.xls` and
   `poi/xls/landscape-a4-japanese.xls` on US Letter, and export them again.
   Word keeps A4. The review marks the four as faults of the reference.
   Source: `test/cmd/review/vm`.
6. Find how Excel scales the printed height of a row for a default font
   other than Calibri and Arial. With Calibri 11 and Arial 10, Excel prints a
   row at 0.968 of its height. With Century Gothic 9, Excel prints the rows
   of 21 points of `poi/xlsx/landscape-letter-budget.xlsx` at 17.52 points,
   0.834. The text and the columns are not scaled, and the file gives 2
   pages against 1. With Courier New 10, Excel prints the rows of 15 points
   of `libreoffice/xlsx/smartart-import.xlsx` at 12.72 points, 0.848.
   Neither the digit widths nor the heights of the fonts explain the two
   ratios. A file in the VM with rows of known heights in a few default
   fonts can tell. Source: `internal/sheet`.
7. Draw a picture that spans rows of a sheet without the marks at its sides
   where the rows cut it, as the jellyfish of `poi/xls/drawings.xls` shows.
   The inside of the picture has no seam. Each row draws its slice of the
   picture 1 point into the next row, and the smoothed pixels of the left and
   the right edges of the picture draw twice there. A strip of 2 points across
   the edge of each row, without the sides, cuts the marks from about 46 to 17
   grey levels. It doubles the time of a page of pictures, because each slice
   draws the whole picture. Source: `internal/sheet` and the layout.
8. Reserve the borders of a cell where a row of a table splits across pages,
   as LibreOffice does. LibreOffice draws the top and the bottom border of the
   split cell on both pages. ivy collapses the borders and reserves nothing at
   the split, so on page 15 of `libreoffice/odt/landscape-a4-table.odt` ivy
   fits one line more, and the pages drift by up to 3 lines around pages 20 to
   25. Source: `internal/layout` (layout_table.go).
9. Compute the defined name of the maximum of the data bar of E4 of
   `poi/xlsx/landscape-letter-budget.xlsx`, SUM(tblIncome[Amount]). Excel
   draws the bar at 62 percent, and ivy draws it full. Source: `xlsx`.
10. Wrap the text on both sides of a picture with square wrapping that is in
    the middle of the column, as Word does on page 6 of
    `poi/doc/floating-pictures.doc`. A float of CSS keeps the text on one
    side. Source: `msdoc`.
11. Draw the picture effects of a Word 97 document: washout, brightness,
    contrast, grayscale and black and white (pages 4 and 5 of
    `poi/doc/floating-pictures.doc`). Source: `internal/odraw`, which draws
    the pictures of `msdoc`, `msppt` and `msxls`.
12. Find why the pages of `poi/doc/floating-pictures.doc` drift from page 3: a
    paragraph that Word puts at the top of page 4 is on page 3 in ivy. Source:
    `msdoc` and the layout.
13. Draw the line numbers of a section (sprmSNLnnMod) of
    `libreoffice/doc/line-numbers.doc`. Source: `msdoc`.
14. Wrap the text around a table that floats, and place a table or a frame
    that floats from the margin or from the page vertically. `msdoc` places a
    table that floats from its paragraph, and a frame only at the center or at
    the right. Source: `msdoc`.
15. Find why the number of the page in the frame of the footers of
    `libreoffice/doc/line-numbers.doc` is 13 pixels lower than in Word.
    Source: `msdoc`.
16. Resolve a percent of `fo:font-size` or `style:font-size-asian` of a text
    style against the size of its paragraph. `odtStyles.props` resolves a text
    style alone, against 12 points. Source: `odf`.
17. Leave the width of a border out of the content of a cell of `rtf`, as Word
    does. Word draws a border on the line between the cells and keeps the
    whole cell for the padding and the text. ivy takes half of each collapsed
    border from the cell, so the narrow cells of
    `libreoffice/rtf/landscape-a4-table.rtf` wrap "x 2" where Word does not.
    Source: `rtf`.
18. Draw the path gradient "shape" of DrawingML, and `msofillShadeShape` of
    Office Drawing, along the outline of the shape. `internal/drawingml` draws
    them along the box, as "rect". Source: `internal/drawingml`.
19. Read the parameter of sigma in the high 16 bits of `fillShadeType`. The
    default is 16384. ivy draws every sigma with the curve of the default.
    Source: `internal/odraw`.
20. Find why PowerPoint ends the path gradient of the background of page 4 of
    `poi/ppt/backgrounds.ppt` at about 0.97 of its colors at the far edge, as
    if the box were about 1 inch larger. Source: `msppt`.
21. Draw the gradients "ellipsoid" and "square" of OpenDocument as an ellipse
    and a square. `odf/internal/drawpage` gives them the paths "circle" and
    "rect", and "circle" no longer stretches with the box. Source: `odf`.
22. Read the thumbnail in WMF of `poi/pptx/grouping-issues.pptx`. It fails
    with "bitmap DIB pixel buffer: malformed metafile" at byte 23824. Source:
    `internal/gowemf`.
23. Excel prints the header row of `poi/xls/landscape-a4-japanese.xls` taller
    than the height in the file, for its wrapped headers. Find whether Excel
    fits the height of a row without fUnsynced again when it opens a file.
    Source: `msxls` and `internal/sheet`.
24. Align to the left a number whose format is text only, "@", as Excel does,
    such as the postal codes of the sheet Customers1 of
    `poi/xls/conditional-formatting-samples.xls`. Source: `internal/sheet`.
25. Find why Excel breaks some sheets of
    `poi/xls/conditional-formatting-samples.xls` into other pages than ivy: a
    narrow band of columns on a page of its own (pages 16, 18 and 20), and a
    page that only shows the header (page 10). Source: `internal/sheet`.
26. Find why the pages of the receipt pictures of
    `poi/xls/basic-expense-template-2011.xls`, from page 45, start higher in
    ivy than in Excel. Source: `msxls` and `internal/sheet`.
27. Read the theme of a workbook of Excel 2007 in the record THEME of a `.xls`
    file. The charts and the conditional formats of `msxls` take the theme of
    Office 2007. Compare a chart of Excel 97 without the records
    ShapePropsStream with Excel. The corpus has none. Source: `msxls`.
28. Draw the WordArt that bends its text, such as an arch, a wave or a circle
    (the types 136 to 175 of MS-ODRAW 2.4.24). `internal/odraw` draws the text
    of each WordArt straight, and each line fills its share of the box. It
    also leaves out gtextAlign, gtextSpacing, fKern, fVertical, fSmallcaps and
    the shadow of geometry text. No file of the corpus has WordArt. Source:
    `internal/odraw` and D51.
29. Set `odraw.Style.Fonts` in `msppt` and `msxls`, so that their WordArt
    draws its text. Without the fonts, a WordArt shape draws the geometry of
    its type, a rectangle with its fill. Source: `msppt` and `msxls`.
30. Convert the adjust values of the other preset shapes of Office Drawing to
    those of ECMA-376. `internal/odraw` converts those of the rounded
    rectangle, the triangle, the parallelogram and the hexagon. The other
    presets draw with the defaults of ECMA-376, whatever adjustValue says.
    Source: `internal/odraw`.

## Left by the charts

1. Draw the treemap, the sunburst and the region map of Office 2016. They
   draw as a frame with their title. The corpus has no file with a treemap
   or a sunburst, and the region map needs map data. Source:
   `internal/chart` (D27).
2. Draw the data tables of the charts of DrawingML, and the equation and
   the R² of a trendline. Source: `internal/chart`.
3. Draw the depth of a 3D chart. A 3D pie draws as a flat ellipse, and the
   other 3D charts as their 2D form. Source: `internal/chart`.

## Left by the CJK fonts of the system

On 2026-11-17 the layout and `internal/fontdb` came to take the East Asian
fonts of the system, by their local names too, and a system face of the
language of the text for a CJK character, when `Config.SystemFonts` is on.
The formats give the layout their East Asian fonts and languages.

1. Give a run of `docx` the script font of its theme: in
   `converter.font`, call `EastAsianFor(lang)` of the minor and major fonts
   for minorEastAsia and majorEastAsia, with the `w:eastAsia` of `w:lang` or
   of `w:themeFontLang`. `aspose/docx/asian-typography.docx` needs it.
   Source: `docx`.
2. Check the synthetic bold of the layout on
   `fbreaderj/fb2/minihelp.zh.fb2`, with SimSun. Source: the layout.

## Left by the pages of Visio

The pages of Visio came on 2026-11-17 (D30).

1. `vsdx`: compute THEMEVAL with arguments. The package uses the V value
   of such a cell.
2. `vsdx`: draw an OLE object that has no picture, and the theme effects
   other than the shadow: the bevel, the glow and the reflection. No sample
   is in the corpus.

## Left by phase 6

1. Read the text layer (TXTa and TXTz), the outline (NAVM) and the links of
   the annotations of DjVu, when the API has a capability for them. D37
   waits for one capability for every format. Source: phase 6.

## Left by phase 5

1. Test the XOR obfuscation of Word with a file that Word wrote. `msdoc`
   decodes it, and only a test that obfuscates `text.doc` with the key of
   ivy covers it. Source: the encryption of phase 5.
2. Read an OpenDocument file of old StarOffice whose password hash has the
   fault of SHA-1 for some lengths. `odf` tries the password in UTF-8, then
   in windows-1252. Source: the encryption of phase 5.
3. The layout of EMF+ DrawString, and its vertical format, are close to GDI+,
   not exact. Source: `internal/gowemf`.
4. Read the headers, the footers, the lists, the pictures and the text boxes
   of a file of Word 6 or Word 95. Source: `msdoc`.
5. Draw what `msppt` does not draw yet: the WordArt text of the gtext
   properties, the pictures in PICT and the pictures of a file that a
   password protects, picture bullets, vertical text, autofit, notes,
   comments, charts and OLE objects other than their pictures. Source:
   `msppt`.
6. Compare the pages of `msdoc`, `msxls` and `msppt` with LibreOffice in
   the test module. Source: `msdoc`, `msxls` and `msppt`.
7. Lay out a long sheet in parts, so that page 0 does not wait for the
   whole sheet. A sheet of 20,000 rows takes about 4 seconds before its
   first page. It needs a document of a `layout.Book` that goes on with the
   pages and the counters of the document before it, so that `&P` and `&N`
   stay right. Source: `internal/sheet` and `internal/layout`.
8. Draw the color scales, the data bars and the icon sets of the spreadsheets
   of `odf` (calcext), and their annotations. Source: `odf`.
9. Draw the glow, the soft edge, the inner shadow and the reflection of
   DrawingML. Find which side of a line the thick line of `thickThin` and
   `thinThick` takes. Source: `internal/drawingml`. On 2026-11-18 no page that
   the review renders uses these effects. Only slide 23 of
   `poi/pptx/custom-geometry.pptx` has an inner shadow.
10. Draw the objects of OpenDocument that only an SVM picture shows, and
    `fo:clip` of the pictures of a text. No permissive decoder of SVM is
    known, and D45 lets it wait. Source: `odf`.
11. Compare the custom caps and the arrows of EMF+ lines with GDI+.
    `internal/gowemf` draws them with a geometry that MS-EMFPLUS does not
    define. Draw the triangle cap and the anchor caps (round, diamond and
    arrow), which gowemf reports. Source: `internal/gowemf`.
12. Draw a stroke with a mix mode (ROP2) that reads the page with a styled pen
    in OPAQUE mode, and the source of an EMF+ DrawImage of a metafile outside
    the metafile with a tile mode. `internal/gowemf` reports them. The blend
    modes of the raster operations are exact for black and white masks only.
    Source: `internal/metafile` and `internal/gowemf`.
13. Take a fallback face for the characters that the face of a run of a
    metafile lacks, such as Arabic in Carlito. The run draws the empty glyph
    of its face. Source: `internal/metafile`.
14. Test change 19 of `internal/gowemf` (D53) with a metafile that a program
    wrote. The corpus of the review has no record that names its glyphs by
    index, so only synthetic records test the text of the indexes. Add to the
    corpus an EMF from a printer driver or from Uniscribe, and an EMF+ Dual
    file with DrawDriverString. Source: D53.

## Left by phase 4

1. Read the NCX and the guide of a MOBI file, and the table of contents of
   an EPUB, when the API has a capability for a table of contents (D37).
   Source: the packages `mobi` and `epub`.
2. Read the profiles of colors when the layout draws, with
   `internal/moxcms`. DeviceCMYK, Lab and `@color-profile` become sRGB
   without a profile. Source: stage D of the layout.

## Left by phase 3

1. Decode a JPEG that `image/jpeg` refuses: truncated, of 12 bits, or with
   arithmetic coding (D17). It needs a copy of `image/jpeg` or a port of
   zune-jpeg, and D45 lets it wait. Source: the port of hayro-syntax.
2. If the maintainers decide again on hinting (D19), the measurement can run
   again: a small Rust program renders each page with hayro in
   `upstream/hayro`, with `RenderSettings.hinting` on and off, and a small
   Go program counts the pixels of ink that differ. Source: D19.
3. Read a PDF file through the `io.ReaderAt` of its source. `pdf.Open` reads
   the whole file into memory, and D39 refuses a file larger than
   `Limits.MaxMemory`. The port works on one `[]byte`, so a reader that
   pages needs a rewrite of its reader and its xref layer. Source: the
   package `pdf`.
4. Compare the pages of ivy with the pages of hayro in the test module. On
   2026-11-16, a scratch program rendered the 63 test files of
   `internal/hayro/interpret` with hayro, and ivy matched each page within a
   mean difference of 1.5. pdftoppm draws 11 of those pages in another way,
   so `TestPDF` does not compare them. Source: phase 3.

## Left by phase 2

1. Decode the images of XPS in JPEG XR. No permissive decoder in Go is
   known. D45 keeps this item. Source: phase 2.

## Left by phase 1

1. Check the previews of the maker notes of Olympus with a real ORF file.
   The tests build the files in memory. Source: `camera`, 2026-11-17.

## Left by the use of okage v0.11.0

ivy uses the parts of okage v0.11.0 that the decisions okage D53 to okage
D59 added. These items are left.

1. A radial shading with no extend whose circles do not nest, and a radial
   shading of `ps` that extends only at its start and whose circles do not
   nest, still use a texture in `pdf` and an image in `ps`. okage paints a
   point with the largest circle that holds it, and then paints nothing when
   that circle is past the end. PDF and PostScript paint it with the largest
   circle that holds it in the domain. Item 1 of "Work for okage" asks for it.
   Source: the decal gradients, 2026-11-18.

## Left by the speed of 2026-11-18

The measurement of the speed of 2026-11-18 and D47 left these items. Each
time is for a page at 96 dpi.

1. Lay out the first page of a large sheet without the whole sheet. The
   cascade now shares the styles of the pseudo-elements of the elements whose
   styles it shares, and a row leaves out its empty cells at its end.
   `datasets/csv/world-cities.csv` still needs about 2.3 s and
   `poi/xlsx/many-merges.xlsx` about 1.5 s before the first page, on a
   computer that other work shared. The cascade and the boxes still cover each
   cell of the one HTML document of the sheet: `Matcher.Match` of okage takes
   about 0.3 to 0.6 µs for each element since okage v0.12.0, and
   `BuildFormattingStructure` builds the boxes of all the rows.
   `internal/sheet` cannot cut a large sheet into documents at its page
   breaks, because the layout makes a row taller when the line of its font is
   higher than the row, and the fonts come from the program. An exact cut
   needs the layout to tell where a page starts, for example with a row that a
   document adds after its last row, to check that this row starts a new page.
   On 2026-11-18 the maintainers asked for a proposed decision that cuts a
   sheet at its page breaks, such as with a row that the layout checks.
   Source: `internal/sheet` and `internal/layout`.
2. Draw the soft masks of `hayro/pdf/tile-clamp-bug.pdf` faster. A group
   now draws its soft mask only on the pixels that it composites, but in
   this document each group covers almost all of the clip of its band. So
   `softMask` is still about a quarter of the time. The time goes to the
   content of the masks: the luminosity masks that `applyDrawProps` gives
   to each drawing, and the alpha mask of an image with an SMask in
   `drawImageWithAlphaMask`, which has no key for the cache. Find whether
   the mask of the image can have a key, or draw it without a group.
   Source: `internal/hayro/render` and `internal/display`.
3. Lay out a book of one large document faster. D48 counts the documents
   of a book at the same time, but a book of one document, such as
   `gutenberg/txt/pg24264.txt` (1.7 s) or
   `gutenberg/html/pg1342-images.html` (1.1 s), lays out its pages in
   order on one goroutine. Source: `internal/layout`.
4. Allocate less in the boxes and the selectors of a book. The cascade of
   `idpf/epub/linear-algebra.epub` now allocates about 150 MB of the 2.7 GB of
   its `PageCount`, down from about 600 MB of 3.1 GB. A style gives its
   initial values, the values of its parent and the zero widths of its borders
   without storing them. The elements with the same matches share their
   cascaded styles. The layout makes the style of a pseudo-element when it
   asks for it. Most of the rest comes from `newBox`, `Box.Copy`,
   `splitInlineBox` and `shapeText` of the layout, and from
   `addRelevantSelectors` and `newElementWrapper` of `cssselect2` in okage.
   Source: `internal/layout`.

## Work for okage

Each item is a request to okage (D8). Ask a maintainer, as `AGENTS.md` says.

1. Give a radial gradient a mode that takes the largest t in 0..=1 that has a
   circle with a radius of at least 0, for the radial shadings of PDF and
   PostScript whose circles do not nest. Source: `pdf` and `ps`, 2026-11-18.

## Work for magic

Each item is a request to magic (D8).

1. Write the package of magic that tells these formats apart from the
   containers that hold them: XPS and OpenXPS from zip, an encrypted Office
   Open XML file from other Compound Files, AZW3 from MOBI, CBZ from zip, APK
   and IPA from zip, and Krita and OpenRaster from zip. magic D64 decides that
   a package of its own names them, as `cfb` does. `magic.Detect` keeps the
   output of libmagic, which names an encrypted Office Open XML file "CDFV2
   Encrypted", AZW3 "version 8" in the description, Krita and OpenRaster by a
   MIME type in the description, APK and IPA by their MIME types when they
   appear, and XPS, OpenXPS and CBZ as zip. Source: iv, and magic D64.
