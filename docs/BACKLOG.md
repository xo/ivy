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
   adjustment, which snaps a thin stroke to the pixels. Item 9 of "Using
   okage v0.11.0" does it. Source: `pdf` and okage.
2. Draw the WordArt and the preset shapes of a Word 97 document that `msdoc`
   does not draw yet, the text of a shape in a group, the arrowheads and the
   comments. Move the two readers of Office Drawing of `msdoc` (`picture.go`
   and `shape.go`) into `internal/odraw`. Source: `msdoc`.
3. Draw the charts of an Excel 97 workbook, which draw as a frame, and the
   pictures in PICT. Source: `msxls`.
4. Number the pages of a workbook across its sheets, as Excel does.
   `poi/xls/basic-expense-template-2011.xls` has the page count of Excel,
   66, and Excel prints "Page 40" on the first page of ExpenseTypes, where
   ivy starts again at 1. Source: `internal/sheet`.
5. Find why the lines of `libreoffice/odt/ooo96040-2.odt` are a little
   taller than those of LibreOffice from page 8. The pages drift, and the
   file has 13 pages against 12. Source: `odf` and the layout.
6. Keep empty paragraphs at the bottom of a page when they do not fit, as
   Word seems to do. On page 3 of `poi/docx/illustrative-cases.docx`, three
   paragraphs after a table need about 2 points more than the page has left.
   Word keeps them on page 3. ivy moves one of them to page 4, and the file
   has 5 pages against 4. The first of them holds spaces and two tabs.
   Perhaps Word lets a line pass the bottom by its descent. A crafted file
   can tell. Source: `docx` and `internal/layout`.
7. Shade a gradient of PowerPoint 97 to 2003 with the sigma curve of
   `fillShadeType`, if MS-ODRAW gives its formula. ivy mixes the colors in
   linear light, as its gamma flag says. Source: `msppt`.
8. Kern the runs of a `.ppt` file as PowerPoint does. `internal/drawingml`
   kerns a run when its size is at least its kern size, as PowerPoint draws
   Calibri. `msppt` does not set `RunStyle.Kern` yet. Source: `msppt`.
9. Find why a merged cell whose text wraps into more lines than its rows
   hold makes its last row taller. Excel keeps the heights of the rows.
   Source: `internal/sheet` and `internal/layout`.
10. Find why Excel in the Windows VM of the review prints
    `poi/xlsx/picture.xlsx`, `poi/xlsx/landscape-a4-tables.xlsx`,
    `poi/xls/landscape-a4-drawing.xls` and
    `poi/xls/landscape-a4-japanese.xls` on US Letter, and export them again.
    Word keeps A4. The review marks the four as faults of the reference.
    Source: `test/cmd/review/vm`.
11. Find how Excel scales the printed height of a row for a font other than
    Calibri and Arial. Excel prints the rows of 21 points of
    `poi/xlsx/landscape-letter-budget.xlsx`, in Century Gothic, at 17.52
    points, where ivy prints them at 0.968 of their height. The file gives 2
    pages against 1. Source: `internal/sheet`.
12. Place a VML shape in the column where Word places it. On page 2 of
    `libreoffice/docx/mixed-orientation.docx`, Word draws the rectangle in
    column 3, and ivy in column 1, because the columns flow differently.
    Source: `docx`.
13. Find why "Division/Program" in the text box of
    `libreoffice/doc/landscape-letter-form.doc` wraps in ivy and not in
    Word. Source: `msdoc`.
14. Draw a picture or an object that spans rows of a sheet without the faint
    seams where the rows cut it, as the jellyfish of `poi/xls/drawings.xls`
    shows. Source: `internal/sheet`.

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
   the annotations of DjVu, when the API has a capability for them. D37
   waits for one capability for every format. Source: phase 6.

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
4. Draw the text of a metafile that names glyphs by index in a font that ivy
   replaces, and the vertical text of EMF+ DrawString. The layout of
   DrawString is close to GDI+, not exact. Source: `internal/metafile` and
   `internal/gowemf`.
5. Draw the end cap of an EMF+ line apart from its start cap, and its custom
   caps. `internal/gowemf` passes `Stroke.EndCap`, and `internal/metafile`
   needs a stroke without caps and two cap shapes. Source:
   `internal/metafile`.
6. Shape the text of complex scripts in a metafile. Bidi mirroring covers
   brackets only. Source: `internal/metafile`.
7. Read the headers, the footers, the lists, the pictures and the text boxes
   of a file of Word 6 or Word 95. Source: `msdoc`.
8. Draw what `msppt` does not draw yet: the WordArt text of the gtext
   properties, the pictures in PICT and the pictures of a file that a
   password protects, picture bullets, vertical text, autofit, notes,
   comments, charts and OLE objects other than their pictures. Source:
   `msppt`.
9. Read the NAME records of BIFF5, and the charts of a `.xls` file. `msxls`
   reads the print areas and the print titles of BIFF8. Source: `msxls`.
10. Compare the pages of `msdoc`, `msxls` and `msppt` with LibreOffice in
    the test module. Source: `msdoc`, `msxls` and `msppt`.
11. Lay out a long sheet in parts, so that page 0 does not wait for the
    whole sheet. A sheet of 20,000 rows takes about 4 seconds before its
    first page. It needs a document of a `layout.Book` that goes on with the
    pages and the counters of the document before it, so that `&P` and `&N`
    stay right. Source: `internal/sheet` and `internal/layout`.
12. Fill the conditional formats of `msxls` from its CF and CF12 records,
    its print order from its SETUP record, and `Cell.Value` of its numbers,
    as `xlsx` does. Draw the color scales, the data bars and the icon sets
    of the spreadsheets of `odf` (calcext), and their annotations. Source:
    `msxls` and `odf`.
13. Draw the glow, the soft edge, the inner shadow and the reflection of
    DrawingML. Find which side of a line the thick line of `thickThin` and
    `thinThick` takes. Source: `internal/drawingml`.
14. Draw the objects of OpenDocument that only an SVM picture shows, and
    `fo:clip` of the pictures of a text. No permissive decoder of SVM is
    known, and D45 lets it wait. Source: `odf`.
15. Push the body of a page of `rtf` and `msdoc` below a header that is
    taller than its space, with `-ivy-margin-box-fit: grow` of the layout,
    as `docx` and `odf` do. They estimate the height of the header. Source:
    `rtf` and `msdoc`.
16. Let the text bodies of one scene of `internal/drawingml` share a
    `FontConfiguration`. Each text body parses the fonts again, which is
    about a quarter of the time of a scene of 30 text bodies. Source:
    `internal/drawingml` and `internal/layout`.

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
2. Give `internal/font` the vertical metrics of a glyph, its advance and its
   origin, so that `xps` does not parse a font again for a sideways run.
   Source: the sideways runs of `xps`, 2026-11-17.

## Left by phase 1

1. Describe a document of Office Open XML by its docProps/core.xml. The
   series and the number of ComicInfo.xml get no field of `ivy.Info` (D35).
   Source: phase 1.
2. Let `internal/drawingml` read the resolution of a picture with
   `raster.Resolution`, in place of its own reader. Source: the ImageBrush
   of `xps`, 2026-11-17.
3. Open the encrypted entries of the other formats of `zipdoc` with
   `Archive.Unlock`, as `comic` does. Source: the encrypted comics,
   2026-11-17.
4. Check the previews of the maker notes of Olympus with a real ORF file.
   The tests build the files in memory. Source: `camera`, 2026-11-17.

## Using okage v0.11.0

okage v0.11.0 gives what the work for okage asked for, in the decisions
okage D53 to okage D59. ivy requires it. The CFF fonts of `pdf` take the
charsets, `seac` and the FontMatrix from it. These changes let ivy use the
other new parts and remove its own copies.

1. Fill the shadings of types 4 to 7 with `okage.NewMeshGradient` in
   `internal/hayro/render/paint.go` and `ps/internal/graphics/shading.go`,
   in place of the sampled texture. A color that a function gives is not
   linear in the triangle, so the patches split until it is close to linear
   (okage D55).
2. Use `okage.SpreadModeDecal` for an axial or a radial shading with no
   extend at either end, in `internal/hayro/render/paint.go`,
   `ps/internal/graphics/shading.go` and `gradientShader` of
   `internal/layout/pdf_stream.go` (okage D55).
3. Draw a group that is not isolated onto its backdrop, and a knockout
   group, with `Pixmap.RemoveBackdrop` and `Pixmap.Knockout` in
   `internal/display/okage.go`. Make the soft masks with
   `okage.MaskFromLuminance` and `Mask.ApplyTransfer` (okage D56).
4. Take the parsed fonts, the faces, the cmap lookups and the metrics from
   the `svg.FontDatabase` of `internal/fontdb`, in `internal/layout`,
   `internal/fontdb/find.go`, `internal/metafile/text.go` and
   `internal/chart/text.go`. Map the weight, the stretch and the style with
   `svg.AutoFontVariations` (okage D57).
5. Draw the color glyphs of SVG and COLR in `drawFirstLine` of
   `internal/layout/draw_text.go`, with `FontDatabase.ColorGlyph` (okage
   D57).
6. Build an inline `<svg>` and a sized SVG image with `roxmltree.Builder`
   and `svg.FromXMLTree` in `internal/layout/images.go`, in place of XML
   text, and read the size with `svg.NodeDimensions` (okage D58).
7. Remove the check of far points (`near`) in `internal/metafile`. A fill
   with huge coordinates no longer panics (okage D53).
8. Fill small glyphs with `Paint.Coverage` set to `okage.Coverage8x8` in
   `internal/display/okage.go`, in place of `fillFine`, and amend D28
   (okage D54).
9. Set `Paint.StrokeAdjust` from the key SA of PDF and from `strokeadjust`
   of PostScript, and make sure that `pdfjs/pdf/a1-plans.pdf` draws its thin
   lines as dark as MuPDF does (okage D54).
10. Remove the fill of a straight solid line in `drawShape` of
    `msdoc/shape.go`. A thin stroke of a straight line draws now (okage
    D54).

## Work for okage

Each item is a request to okage (D8). Ask a maintainer, as `AGENTS.md` says.
No item is open. okage v0.11.0 did the items of 2026-11-17.

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
