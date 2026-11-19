# Backlog

This document holds the work that is known and not done, with where it came
from. When an item is done, delete it, and record anything that was decided
in `docs/decisions/`.

## Units in progress

A unit is one package, or one file of a port with its tests. Claim a unit
here before you start it, with the date. When the change is committed,
delete the claim.

No unit is in progress on 2026-11-19.

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
2. Find why ivy draws the frame with the picture and the caption "Figure 1.2"
   on page 9 of `libreoffice/odt/ooo96040-2.odt` 19 pixels higher than
   LibreOffice. It moves two bullets to page 10 and gives 13 pages against 12.
   The lines now round the metrics of the font to twips, as LibreOffice does.
   Source: `odf`.
3. Check whether an empty last line of a table cell can pass the bottom of a
   page by its descent in Word or LibreOffice. `-ivy-page-overflow` (D52) does
   not apply to the lines of a table cell, a float, a flex or grid item, an
   inline block or a column, because their parents check the bottom of the
   page without the descent. If it can, the row check of `layout_table.go` and
   the parents of a table must subtract the descent too. Source:
   `internal/layout` and D52.
4. Find the default height of a row on the screen, S, for a font and a size
   that `measuredRows` of `internal/sheet` does not hold. ivy takes P from the
   face that draws the font, and S as P at 96 dots per inch, rounded up. This
   S is right for 6 of the 10 measured fonts that Windows has. It is 1 to 3
   pixels low for Courier New, MS PGothic and Century Gothic, and 1 pixel high
   for Verdana. Times New Roman of 20 and 26 points is 1 to 2 pixels low in
   `poi/xlsx/landscape-a4-tables.xlsx`. Excel prints 17 of the 156 rows of set
   heights of `xlsx/testdata/row-heights-*.xlsx` 1 dot away from trunc(h × P /
   S), in both directions. Source: `internal/sheet`.
5. Reserve the borders of a cell where a row of a table splits across pages,
   as LibreOffice does. LibreOffice draws the top and the bottom border of the
   split cell on both pages. ivy collapses the borders and reserves nothing at
   the split, so on page 15 of `libreoffice/odt/landscape-a4-table.odt` ivy
   fits one line more, and the pages drift by up to 3 lines around pages 20 to
   25. Source: `internal/layout` (layout_table.go).
6. Wrap the text on both sides of a picture with square wrapping that is in
   the middle of the column, as Word does on page 6 of
   `poi/doc/floating-pictures.doc`. A float of CSS keeps the text on one
   side. Source: `msdoc`.
7. Draw the picture effects of a Word 97 document: washout, brightness,
   contrast, grayscale and black and white (pages 4 and 5 of
   `poi/doc/floating-pictures.doc`). Source: `internal/odraw`, which draws
   the pictures of `msdoc`, `msppt` and `msxls`.
8. Place a table or a frame of `msdoc` that floats from the margin or from
   the page vertically, and a frame at a position other than the center or
   the right. A table that floats beside the text now floats in CSS, with the
   text around it. Source: `msdoc` and the layout.
9. Compare the path gradient "shape" of a focus out of the shape with
   PowerPoint. ivy puts the first stop where the ray from the focus enters the
   shape, and the last stop where the ray leaves it. On slide 4 of
   `pptx/testdata/path-gradients.pptx`, the star of ivy differs from
   PowerPoint. ivy draws a shape whose box has its center out of the shape, or
   on its outline, with the path "circle" about that center, as PowerPoint
   draws the moon. The rule of PowerPoint for this is not known. Source:
   `internal/drawingml`.
10. Measure in Excel how it fits a row when it opens a file. ivy counts the
    lines of text that wraps with the widths of Calibri (`TextWidth`), and it
    estimates the height of rotated text. An empty cell takes one line of its
    font. A row without cells keeps its height. A row keeps the height of its
    file when that height is at most 2 pixels or 10 percent away from the fit.
    These rules come from 4 files of the corpus and are not measured. Source:
    `internal/sheet`.
11. Find why Excel breaks some sheets of
    `poi/xls/conditional-formatting-samples.xls` into other pages than ivy: a
    narrow band of columns on a page of its own (pages 16, 18 and 20), and a
    page that only shows the header (page 10). Source: `internal/sheet`.
12. Find why the pages of the receipt pictures of
    `poi/xls/basic-expense-template-2011.xls`, from page 45, start higher in
    ivy than in Excel. Source: `msxls` and `internal/sheet`.
13. Compare the WordArt of `internal/odraw` with PowerPoint. No file of the
    corpus has WordArt. `internal/odraw/warp.go` bends the text of the
    triangles, the chevrons, the arches and the circles, the waves, inflate
    and deflate, the fades, the slants and the cans of MS-ODRAW 2.4.24, with
    fixed default adjust values. It does not read the adjust values of the
    shape. It draws the rings, the buttons, the curves, the cascades, the stop
    and the two mixes of inflate and deflate as plain text. It leaves out
    fKern and gtextFShadow. A file with WordArt of each kind can give the size
    of the text along an arch. Source: `internal/odraw`.
14. Convert the adjust values of the presets of Office Drawing that
    `internal/odraw/adjust.go` does not convert: the trapezoid, which Office
    Drawing draws with its narrow side down, the arc, the block arc, the
    circular arrow, the bent, curved and U-turn arrows, the ribbons and the
    arrow callouts. Map the callouts of the types 178 to 181. The defaults of
    the tips of the callouts 61, 62, 63 and 106 are not known, and they take
    the defaults of ECMA-376. Source: `internal/odraw`.
15. Draw the border of characters (sprmCBrc) of a Word 97 document, such as
    the word "border" on page 3 of `poi/doc/floating-pictures.doc`. Word makes
    the line about 2 points taller for it. Source: `msdoc`.
16. Make the running element of the headers and the footers of `docx` a flow
    root, as `msdoc`, `rtf` and `odf` do. A footer aligned at the bottom moves
    down by the space before its first paragraph. Source: `docx`.
17. Find why page 2 of `libreoffice/doc/landscape-letter-form.doc` draws the
    black bars of its table 1 pixel lower than Word. The header line of Arial
    16 is now 18.4 points high, as in Word. Source: `msdoc`.
18. Draw the lines, the shapes, the regions and the text of a QuickDraw
    picture (PICT). `internal/pict` draws only the bitmaps of a picture and
    its images of QuickTime, for the pictures of Office Drawing of `msxls`,
    `msppt` and `msdoc`. No file of the corpus has a PICT picture. Source:
    `internal/pict`, 2026-11-18.
19. Compare a chart of Excel 97 without the records ShapePropsStream with
    Excel. `msxls` reads the theme of the record THEME, and a workbook without
    one takes the theme of Office 2007. The corpus has no such chart. Source:
    `msxls`.
20. Find why a chart that rows cut shows a light seam at some edges of its
    rows, as on page 3 of `poi/xls/three-charts.xls`. A slice of a chart
    reaches 1 point into the next row. A strip across the edge of the row, as
    a picture has, gave more seams in the charts, and the cause is not known.
    Source: `internal/sheet` and the layout.
21. Put the line gap of a single line above its text, as Word and LibreOffice
    do. The layout puts half of the gap above the text and half below.
    `-ivy-line-multiple` (D56) puts the whole gap above only for a multiple
    line spacing. With Calibri, whose gap is 0.22 em, the text of a single
    line in Word is about 1 point lower than in ivy. A test with LibreOffice
    puts the baseline of Liberation Serif 0.935 em below the top of a single
    line: the ascent and the whole gap. Source: the layout, `docx`, `msdoc`,
    `odf` and `rtf`.
22. Find why the text in the cells of the table of
    `libreoffice/doc/legal-table.doc` is lower in Word than in ivy, and why
    the rows of Word are taller. `-ivy-line-multiple` (D56) moves the text up
    in its lines, and the review of the file went from 1.52 to 1.66. Source:
    `msdoc`.
23. Find why page 2 of `libreoffice/odt/tdf146272.odt` holds fewer lines in
    ivy than in LibreOffice. The text after the caption "Figure 2" is 2 pixels
    higher than in LibreOffice. With `-ivy-line-multiple` (D56) the review of
    the page went from 8.25 to 9.05, and pages 1 and 3 got better. Source:
    `odf`.
24. Give the numbers of the lines of `msdoc` the font of the style Line Number
    (sti 40). They now take the font of the style Normal. Leave out the space
    before a paragraph at the top of a column and of a page in a section with
    columns, as Word does. `-ivy-page-top-space` (D56) only applies to the
    blocks of the flow of the root box. Source: `msdoc` and the layout.
25. Read `text:count-empty-lines`, `text:count-in-text-boxes`, the right,
    inner and outer positions, and `text:line-number` of a paragraph in the
    line numbering of `odf`. The layout draws the numbers on the left only,
    and counts the empty lines. Source: `odf` and the layout.

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

## Left by phase 7a

1. Keep a thin line thin at every resolution in a PDF file. The port of
   hayro widens a stroke of less than one pixel of its target to one pixel,
   and so does the interpreter of PostScript (`minLinePixels`). `Draw`
   draws at the resolution of `WithRasterDPI`, so such a line is 0.24 points
   wide in the file at 300 dots for each inch, and a viewer at 72 dots for
   each inch draws it lighter than `Render` does. A stroke of width 0 is the
   thinnest line of the viewer at each resolution. Source: `pdf/pdfenc`,
   `internal/hayro/render`, `ps/internal/graphics`.
2. Find why Poppler 26.11 draws the tiles of the background of page 3 of
   `poi/ppt/backgrounds.ppt` at another phase than ivy and MuPDF in the file
   of pdfenc. A tiling pattern in a simple group draws at the same phase in
   both viewers. Source: `TestPDFEnc` of the test module.

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
   of a file of Word 6 or Word 95. Source: `msdoc`. The corpus has no file of
   Word 6 or Word 95 with a header, and LibreOffice cannot write one. The FIB
   of Word 6 has ccpHdd at 0x3C and fcPlcfhdd at 0xB0, and the PlcfHdd lists
   only the stories that grpfIhdt of each section sets.
5. Draw what `msppt` does not draw yet: the pictures of a file that a password
   protects, picture bullets, vertical text, autofit, notes, comments, charts
   and OLE objects other than their pictures. Source: `msppt`.
6. Compare the pages of `msdoc`, `msxls` and `msppt` with LibreOffice in
   the test module. Source: `msdoc`, `msxls` and `msppt`.
7. Draw the annotations of the spreadsheets of `odf`. Source: `odf`.
8. Draw the glow, the soft edge, the inner shadow and the reflection of
   DrawingML. Find which side of a line the thick line of `thickThin` and
   `thinThick` takes. Source: `internal/drawingml`. On 2026-11-18 no page that
   the review renders uses these effects. Only slide 23 of
   `poi/pptx/custom-geometry.pptx` has an inner shadow.
9. Draw the objects of OpenDocument that only an SVM picture shows, and
   `fo:clip` of the pictures of a text. No permissive decoder of SVM is
   known, and D45 lets it wait. Source: `odf`.
10. Compare the custom caps and the arrows of EMF+ lines with GDI+.
    `internal/gowemf` draws them with a geometry that MS-EMFPLUS does not
    define. Draw the triangle cap and the anchor caps (round, diamond and
    arrow), which gowemf reports. Source: `internal/gowemf`.
11. Draw a stroke with a mix mode (ROP2) that reads the page with a styled pen
    in OPAQUE mode, and the source of an EMF+ DrawImage of a metafile outside
    the metafile with a tile mode. `internal/gowemf` reports them. The blend
    modes of the raster operations are exact for black and white masks only.
    Source: `internal/metafile` and `internal/gowemf`.
12. Take a fallback face for the characters that the face of a run of a
    metafile lacks, such as Arabic in Carlito. The run draws the empty glyph
    of its face. Source: `internal/metafile`.
13. Test change 19 of `internal/gowemf` (D53) with a metafile that a program
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

## Left by the speed of 2026-11-18

The measurement of the speed of 2026-11-18 and D47 left these items. Each
time is for a page at 96 dpi.

1. Predict the height of a row whose line of text is higher than the row.
   `internal/sheet` predicts the page breaks of a long sheet from the heights
   of its rows (D55). At a small scale, such as fit to page at 30%, the line
   of the font and the halves of the borders make a row of 4.5 points about
   4.53 points high. Then the probe row of each part does not start a page,
   and the book lays out the parts one after the other. Text that wraps does
   the same. Source: `internal/sheet`.
2. Draw the soft masks of `hayro/pdf/tile-clamp-bug.pdf` faster. A group
   now draws its soft mask only on the pixels that it composites, but in
   this document each group covers almost all of the clip of its band. So
   `softMask` is still about a quarter of the time. The time goes to the
   content of the masks: the luminosity masks that `applyDrawProps` gives
   to each drawing, and the alpha mask of an image with an SMask in
   `drawImageWithAlphaMask`, which has no key for the cache. Find whether
   the mask of the image can have a key, or draw it without a group.
   Source: `internal/hayro/render` and `internal/display`.
3. Allocate less in the boxes of a book. `PageCount` of
   `idpf/epub/linear-algebra.epub` now allocates 2.3 GB, down from 2.5 GB, in
   26.5 million objects instead of 31.8 million. 397,000 of its 524,000 text
   boxes hold only white space, and `inlineInBlock` drops most of them later.
   A box is 384 bytes, so these boxes take about 150 MB. The layout cannot
   leave them out, because the collapsible spaces of `ProcessWhitespace` go
   from one text box to the next. Most of the rest comes from okage:
   `roxmltree`, `newElementWrapper` and `addRelevantSelectors` of
   `cssselect2`, and the glyphs of `shaping.Output`. Source: `internal/layout`
   and okage.
4. Render each page of a long sheet without a second layout of its parts.
   `PageCount` lays out the parts on goroutines and drops their layouts (D48),
   so the pages after it lay out each part again, with its lead page and its
   probe page. On 2026-11-18 the mean time of a page of
   `datasets/csv/world-cities.csv` went from 32.5 ms to 36.9 ms, and of
   `poi/xlsx/many-merges.xlsx` from 25.6 ms to 28.8 ms. Source:
   `internal/layout`.
5. Make the clips of a sheet cheaper. Each clip of `OkageDevice.ClipPath`
   makes a mask of the size of its band, and a clip in a clip clones the mask.
   On the pages of `poi/xls/legal-portrait.xls` and
   `poi/xls/landscape-a4-drawing.xls`, which take about 140 ms and 117 ms,
   `ClipPath` is about a quarter of the time. In
   `poi/xls/basic-expense-template-2011.xls`, `NewMask` and `Mask.Clone` make
   about 80% of the bytes that the pages allocate. A rectangle under a
   transform without a turn can clip without a mask, or the mask can cover
   only the bounds of the clip. Source: `internal/display`. A picture that
   rows cut has a clip for each row and for the strip of each row, so a page
   of `poi/xlsx/picture.xlsx` takes about 12 percent more time with the
   strips.
6. Give the galley of D57 the CPUs that `countAll` of D48 leaves idle. The
   galley does not start for the documents that `countAll` counts at the same
   time, so a book of many documents gets no galley for its count. Source:
   D57, 2026-11-19.
7. Take more lines from the galley of D57. The galley keeps no line that holds
   an image, an inline-block or a footnote call, and each part reads, styles
   and builds the document again (open point 1 of D57). The floats of
   `pg1342-images.html` keep the pages on one goroutine for about a fifth of
   its lines. Its count went from 1.2 s to 0.76 s, and the count of
   `pg24264.txt` went from 1.3 s to 0.44 s. Source: D57, 2026-11-19.
8. Make the test `TestGalley` of the test module faster. It takes about 8
   minutes on the whole corpus, so it runs only when `IVY_REVIEW_CACHE` is
   set, and CI does not run it. Source: D57, 2026-11-19.

## Work for okage

Each item is a request to okage (D8). Ask a maintainer, as `AGENTS.md` says.

1. Give `shaping.Output` the unsafe-to-break flags of HarfBuzz, so that the
   layout can reuse the shaping of a text for a line that breaks in it, and
   allocate the glyphs of `shaping.Output` less. Half of the layout of
   `gutenberg/html/pg1342-images.html` is the shaper. Source:
   `internal/layout`, 2026-11-18.
2. Give the dash array and the offset of a `StrokeDash`, so that pdfenc
   writes the dashes of a stroke, not a path cut into dashes. Source:
   `pdf/pdfenc`, 2026-11-19.
3. In `pdf`, write a gradient with more than 257 stops so that MuPDF 1.28
   reads it, such as with a sampled function. MuPDF reads at most 256
   functions of a stitching function and draws nothing for more. pdfenc
   draws such a gradient with the raster fallback. Source: `pdf/pdfenc` and
   `hayro/pdf/pattern-shading-type2-many.pdf`, 2026-11-19.
4. In `pdf`, let `Surface.FillStencil` write a stencil as an image mask
   (`/ImageMask`) when its fill is one color. A viewer scales a soft mask
   down in another way than a stencil mask, so the glyph bitmaps of
   `repo/pdf/font-type3-widths-with-matrix.pdf` differ by 16 at 72 dots for
   each inch, and by 0.01 at 300. Source: `pdf/pdfenc`, 2026-11-19.

## Work for magic

Each item is a request to magic (D8). No item is open on 2026-11-19.
