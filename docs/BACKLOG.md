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

1. Draw a stroke of 1 pixel or less as dark as MuPDF and Poppler do.
   `pdfjs/pdf/a1-plans.pdf` draws its thin lines lighter, because a line
   across a pixel border spreads over two light pixels. It needs stroke
   adjustment, which snaps a thin stroke to the pixels. Source: `pdf` and
   okage.
2. Return the text of PowerPoint 95, which `msppt` refuses with
   ErrUnsupported. MS-PPT does not describe PowerPoint 95, and PowerPoint
   2024 does not open `poi/ppt/ppt95.ppt` either. Question 16 of
   `PLAN.md` asks whether ivy reads it. Source: `msppt`.
3. Draw the WordArt and the preset shapes of a Word 97 document that
   `msdoc` does not draw yet, the text of a shape in a group, the arrowheads
   and the comments. Move the two readers of Office Drawing of `msdoc`
   (`picture.go` and `shape.go`) into `internal/odraw`. Source: `msdoc`.
4. Draw the charts of an Excel 97 workbook, which draw as a frame, and the
   pictures in PICT. Source: `msxls`.
5. Number the pages of a workbook across its sheets, as Excel does.
   `poi/xls/basic-expense-template-2011.xls` has the page count of Excel,
   66, and Excel prints "Page 40" on the first page of ExpenseTypes, where
   ivy starts again at 1. Source: `internal/sheet`.
6. Find why the lines of `libreoffice/odt/ooo96040-2.odt` are a little
   taller than those of LibreOffice from page 8. The pages drift, and the
   file has 13 pages against 12. Source: `odf` and the layout.
7. Keep empty paragraphs at the bottom of a page when they do not fit, as
   Word seems to do. On page 3 of `poi/docx/illustrative-cases.docx`, three
   paragraphs after a table need about 2 points more than the page has
   left. Word keeps them on page 3. ivy moves one of them to page 4, and the
   file has 5 pages against 4. The first of them holds spaces and two tabs.
   Perhaps Word lets a line pass the bottom by its descent. A crafted file
   can tell. Source: `docx` and `internal/layout`.
8. Decide whether a text file reflows its paragraphs as calibre does.
   calibre joins the lines of a paragraph, and ivy keeps them. Question 7 of
   `PLAN.md` asks it. Source: `text`.
9. Shade a gradient of PowerPoint 97 to 2003 with the sigma curve of
   `fillShadeType`, if MS-ODRAW gives its formula. ivy mixes the colors in
   linear light, as its gamma flag says. Source: `msppt`.
10. Kern the runs of a `.ppt` file as PowerPoint does. `internal/drawingml`
    kerns a run when its size is at least its kern size, as PowerPoint draws
    Calibri. `msppt` does not set `RunStyle.Kern` yet. Source: `msppt`.
11. Find why a merged cell whose text wraps into more lines than its rows
    hold makes its last row taller. Excel keeps the heights of the rows.
    Source: `internal/sheet` and `internal/layout`.
12. Find why Excel in the Windows VM of the review prints
    `poi/xlsx/picture.xlsx`, `poi/xlsx/landscape-a4-tables.xlsx`,
    `poi/xls/landscape-a4-drawing.xls` and
    `poi/xls/landscape-a4-japanese.xls` on US Letter, and export them again.
    Word keeps A4. The review marks the four as faults of the reference.
    Source: `test/cmd/review/vm`.
13. Find how Excel scales the printed height of a row for a font other than
    Calibri and Arial. Excel prints the rows of 21 points of
    `poi/xlsx/landscape-letter-budget.xlsx`, in Century Gothic, at 17.52
    points, where ivy prints them at 0.968 of their height. The file gives 2
    pages against 1. Source: `internal/sheet`.
14. Place a VML shape in the column where Word places it. On page 2 of
    `libreoffice/docx/mixed-orientation.docx`, Word draws the rectangle in
    column 3, and ivy in column 1, because the columns flow differently.
    Source: `docx`.
15. Find why "Division/Program" in the text box of
    `libreoffice/doc/landscape-letter-form.doc` wraps in ivy and not in
    Word. Source: `msdoc`.
16. Draw a picture or an object that spans rows of a sheet without the
    faint seams where the rows cut it, as the jellyfish of
    `poi/xls/drawings.xls` shows. Source: `internal/sheet`.

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
2. Read `fo:font-size-asian` and the other sizes of the East Asian text of
   OpenDocument. Source: `odf`.
3. Check the synthetic bold of the layout on
   `fbreaderj/fb2/minihelp.zh.fb2`, with SimSun. Source: the layout.

## Left by the pages of Visio

The pages of Visio came on 2026-11-17 (D30).

1. `internal/gowemf`: clamp or tile the source rectangle of an EMF+
   DrawImage that is truly outside the image, as GDI+ does.
2. `vsdx`: compute THEMEVAL with arguments. The package uses the V value
   of such a cell.
3. `vsdx`: draw an OLE object that has no picture, and the theme effects
   other than the shadow: the bevel, the glow and the reflection. No sample
   is in the corpus.

## Left by phase 6

1. Read the text layer (TXTa and TXTz), the outline (NAVM) and the links of
   the annotations of DjVu, when the API has a capability for them.
   Question 11 of `PLAN.md` asks it. Source: phase 6.
2. Open an indirect DjVu document when a program can give a folder, as D22
   does for Markdown. This needs a decision. Question 12 of `PLAN.md` asks
   it. Source: D26.

## Left by phase 5

1. Test the XOR obfuscation of Word with a file that Word wrote. `msdoc`
   decodes it, and only a test that obfuscates `text.doc` with the key of
   ivy covers it. Source: the encryption of phase 5.
2. Read an OpenDocument file of old StarOffice whose password hash has the
   fault of SHA-1 for some lengths. `odf` tries the password in UTF-8, then
   in windows-1252. Source: the encryption of phase 5.
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
   caps. `internal/gowemf` passes `Stroke.EndCap`, and `internal/metafile`
   needs a stroke without caps and two cap shapes. Source:
   `internal/metafile`.
6. Shape the text of complex scripts in a metafile. Bidi mirroring covers
   brackets only. Source: `internal/metafile`.
7. Read the headers, the footers, the lists, the pictures and the text
   boxes of a file of Word 6 or Word 95. Source: `msdoc`.
8. Draw what `msppt` does not draw yet: the WordArt text of the gtext
   properties, the pictures in PICT and the pictures of a file that a
   password protects, picture bullets, vertical text, autofit, notes,
   comments, charts and OLE objects other than their pictures. Source:
   `msppt`.
9. Read the NAME records of BIFF5, and the charts of a `.xls` file.
   `msxls` reads the print areas and the print titles of BIFF8. Source:
   `msxls`.
10. Compare the pages of `msdoc`, `msxls` and `msppt` with LibreOffice in
    the test module. Source: `msdoc`, `msxls` and `msppt`.
11. Lay out a long sheet in parts, so that page 0 does not wait for the
    whole sheet. A sheet of 20,000 rows takes about 4 seconds before its
    first page. It needs a document of a `layout.Book` that goes on with the
    pages and the counters of the document before it, so that `&P` and `&N`
    stay right. Source: `internal/sheet` and `internal/layout`.
12. Fill the conditional formats of `msxls` from its CF and CF12 records,
    its print order from its SETUP record, and `Cell.Value` of its numbers,
    as `xlsx` does. Draw the color scales, the data bars and the icon sets of
    the spreadsheets of `odf` (calcext), and their annotations. Source:
    `msxls` and `odf`.
13. Read the dates of a CSV file. An option for the separator of `csv`
    changes the API, so it is a decision first (D11). Question 9 of
    `PLAN.md` asks it. Source: `csv`.
14. Draw the glow, the soft edge, the inner shadow and the reflection of
    DrawingML. Find which side of a line the thick line of `thickThin` and
    `thinThick` takes. Source: `internal/drawingml`.
15. Draw the objects of OpenDocument that only an SVM picture shows, and
    `fo:clip` of the pictures of a text. No permissive decoder of SVM is
    known. Question 20 of `PLAN.md` asks about it. Source: `odf`.
16. Push the body of a page of `rtf` and `msdoc` below a header that is
    taller than its space, with `-ivy-margin-box-fit: grow` of the layout,
    as `docx` and `odf` do. They estimate the height of the header. Source:
    `rtf` and `msdoc`.
17. Let the text bodies of one scene of `internal/drawingml` share a
    `FontConfiguration`. Each text body parses the fonts again, which is
    about a quarter of the time of a scene of 30 text bodies. Source:
    `internal/drawingml` and `internal/layout`.

## Left by phase 4

1. Read the HTML of `mobi` (`rewriteTags` of `mobi6.go`) and of `chm`
   (`metaPlace` of `charset.go`) without a scan of its bytes, if D7 covers
   them. They change text at byte positions, which a tree cannot do.
   Question 18 of `PLAN.md` asks it. Source: `mobi` and `chm`.
2. Return the publisher and the identifier of a book. `internal/ebook` reads
   them, and `ivy.Info` has no field for them. A new field changes the API,
   so it is a decision first (D11). Question 8 of `PLAN.md` asks it.
   Source: the package `epub`.
3. Read the NCX and the guide of a MOBI file, and the table of contents
   of an EPUB, when the API has a capability for a table of contents.
   Question 11 of `PLAN.md` asks it. Source: the packages `mobi` and
   `epub`.
4. Bound the time of the selector matcher of okage. A `:has()` selector
   costs time in the square of the number of elements. Source: stage A of
   the layout.
5. Count `counter(pages)` across the documents of a book. `counter(page)`
   goes on across them, and the total needs the whole book laid out.
   Question 19 of `PLAN.md` asks about it. Source: stage D of the layout.
6. Give a page that a program asks for early the fixed boxes and the
   target counters of the later pages. `Document.Page` lays out only the
   pages up to the one that a program asks for. Question 19 of `PLAN.md`
   asks about it. Source: stage C of the layout.
7. Draw the color glyphs of SVG and COLR. The layout draws the bitmap
   glyphs of sbix, CBDT, EBDT and bdat. okage draws SVG and COLR glyphs
   inside `svg`, and does not export it. Source: stage D of the layout.
8. Read the profiles of colors when the layout draws, with
   `internal/moxcms`. DeviceCMYK, Lab and `@color-profile` become sRGB
   without a profile. Source: stage D of the layout.

## Left by phase 3

1. Decode a JPEG that `image/jpeg` refuses: truncated, of 12 bits, or with
   arithmetic coding (D17). It needs a copy of `image/jpeg` or a port of
   zune-jpeg. Question 15 of `PLAN.md` asks which. Source: the port of
   hayro-syntax.
2. If the maintainers decide again on hinting (D19), the measurement can run
   again: a small Rust program renders each page with hayro in
   `upstream/hayro`, with `RenderSettings.hinting` on and off, and a small Go
   program counts the pixels of ink that differ. Source: D19.
3. Draw a group that is not isolated onto its backdrop, and draw a knockout
   group. The device of okage draws each group as isolated, and not as
   knockout. hayro does the same. Source: the port of the crate hayro.
4. Read a PDF file through the `io.ReaderAt` of its source. `pdf.Open` reads
   the whole file into memory, up to its size, without a check against
   `Limits.MaxMemory`. The port works on one `[]byte`, so a reader that
   pages needs a rewrite of its reader and its xref layer. Question 14 of
   `PLAN.md` asks it. Source: the package `pdf`.
5. Compare the pages of ivy with the pages of hayro in the test module. On
   2026-11-16, a scratch program rendered the 63 test files of
   `internal/hayro/interpret` with hayro, and ivy matched each page within a
   mean difference of 1.5. pdftoppm draws 11 of those pages in another way,
   so `TestPDF` does not compare them. Source: phase 3.

## Left by phase 2

1. Decode the images of XPS in JPEG XR. No permissive decoder in Go is known.
   Question 13 of `PLAN.md` asks whether to keep this item. Source: phase 2.
2. Give `internal/font` the vertical metrics of a glyph, its advance and its
   origin, so that `xps` does not parse a font again for a sideways run.
   Source: the sideways runs of `xps`, 2026-11-17.

## Left by phase 1

1. Describe a document of Office Open XML by its docProps/core.xml. The
   publisher, the series and the number of ComicInfo.xml need fields of
   `ivy.Info`, which question 8 of `PLAN.md` asks about. Source: phase 1.
2. Give `ivy.Cache` a way for a format package to store and load a value, for
   the fonts of phase 2. Question 10 of `PLAN.md` asks whether it is still
   needed. Source: phase 1.
3. Let `internal/drawingml` read the resolution of a picture with
   `raster.Resolution`, in place of its own reader. Source: the ImageBrush of
   `xps`, 2026-11-17.
4. Open the encrypted entries of the other formats of `zipdoc` with
   `Archive.Unlock`, as `comic` does. Source: the encrypted comics,
   2026-11-17.
5. Check the previews of the maker notes of Olympus with a real ORF file.
   The tests build the files in memory. Source: `camera`, 2026-11-17.

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
11. Export the predefined charsets of CFF (ISOAdobe, Expert and
    ExpertSubset), and draw `seac` and the FontMatrix of each font
    dictionary in `cff.LoadGlyph`. ivy keeps copies of the charset tables
    of the specification in the port of hayro-font. Source: the CFF of
    `pdf`, 2026-11-17.
12. A public function that turns one glyph of a face into something to
    draw, an `svg.Tree` in em units or paths with paints, with the
    variations and the palette of CPAL, for the SVG and COLR glyphs of the
    layout. Source: the color glyphs of the layout, 2026-11-17.
13. Apply the PairPos subtables of a GPOS extension lookup (type 9) with
    flag 8, as Calibri has. The pairs t-a, a-t, t-e and n-t of Calibri do
    not kern, and "AV" does. Source: the kerning of `internal/drawingml`,
    2026-11-17.
14. Draw an SVG path with a stroke only, under about 0.75 points, when the
    path has no width or no height. It draws nothing, where a closed
    rectangle with the same stroke draws. `msdoc` fills the area of a
    straight solid line instead. Source: the shapes of `msdoc`,
    2026-11-17.

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
