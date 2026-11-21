# Backlog

This document holds the work that is known and not done, with where it came
from. When an item is done, delete it, and record anything that was decided
in `docs/decisions/`.

## Units in progress

A unit is one package, or one file of a port with its tests. Claim a unit
here before you start it, with the date. When the change is committed,
delete the claim.

No unit is in progress on 2026-11-21.

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
5. Find why a row of `libreoffice/odt/landscape-a4-table.odt` whose
   paragraphs have a line spacing of 1.5 is taller in ivy than in
   LibreOffice. The row "K bodu 96" on page 19 is about 7 pixels taller, and
   the pages from 20 to 29 drift by up to 3 lines. The layout draws the
   borders of a split cell on both pages since 2026-11-21, and pages 14 to 19
   now hold the same lines as in LibreOffice. LibreOffice also ends a split
   cell at its last line, and the layout makes it fill the page to the bottom,
   as CSS Fragmentation says. Source: `odf` and `-ivy-line-multiple` (D56).
6. Wrap the text on both sides of a picture with square wrapping that is in
   the middle of the column, as Word does on page 6 of
   `poi/doc/floating-pictures.doc`. A float of CSS keeps the text on one
   side. LibreOffice also wraps the text on both sides there. D63 decides
   the property `-ivy-wrap-flow`, and the formats give it. Source: `msdoc`,
   `docx`, `odf`, `rtf` and the layout, D63.
7. Measure the page floats of D60 in Word, with the files `float-*.docx` of
   `docx/testdata` and `float-*.doc` of `msdoc/testdata`. LibreOffice 26.8
   places the floats of these files within 0.05 inch of ivy, except for two
   files. In `float-t-loop2`, LibreOffice moves the anchor with the table to
   page 2, and ivy keeps the table on page 1, as D60 decides. In
   `float-t-nofit`, LibreOffice cuts the table at the bottom of the paper,
   and ivy moves it up until it fits. These parts of D60 are not built yet:
   the frames of `rtf`, the frames of `odf` that are anchored to the page
   with wrap, which stay absolute, and `w:tblOverlap` and `allowOverlap` of
   `docx`. The meaning of inside and outside on the vertical axis is not
   measured. Source: `msdoc`, `docx`, `odf` and the layout, D60.
8. Compare the path gradient "shape" of a focus out of the shape with
   PowerPoint. ivy puts the first stop where the ray from the focus enters the
   shape, and the last stop where the ray leaves it. On slide 4 of
   `pptx/testdata/path-gradients.pptx`, the star of ivy differs from
   PowerPoint. ivy draws a shape whose box has its center out of the shape, or
   on its outline, with the path "circle" about that center, as PowerPoint
   draws the moon. The rule of PowerPoint for this is not known. Source:
   `internal/drawingml`.
9. Measure in Excel how it fits a row when it opens a file. ivy counts the
   lines of text that wraps with the widths of Calibri (`TextWidth`), and it
   estimates the height of rotated text. An empty cell takes one line of its
   font. A row without cells keeps its height. A row keeps the height of its
   file when that height is at most 2 pixels or 10 percent away from the fit.
   These rules come from 4 files of the corpus and are not measured. Source:
   `internal/sheet`.
10. Find why Excel breaks some sheets of
    `poi/xls/conditional-formatting-samples.xls` into other pages than ivy: a
    narrow band of columns on a page of its own (pages 16, 18 and 20), and a
    page that only shows the header (page 10). Source: `internal/sheet`.
11. Find why the pages of the receipt pictures of
    `poi/xls/basic-expense-template-2011.xls`, from page 45, start higher in
    ivy than in Excel. Source: `msxls` and `internal/sheet`.
12. Compare the WordArt of `internal/odraw` with PowerPoint. No file of the
    corpus has WordArt. `internal/odraw/warp.go` bends the text of the
    triangles, the chevrons, the arches and the circles, the waves, inflate
    and deflate, the fades, the slants and the cans of MS-ODRAW 2.4.24, with
    fixed default adjust values. It does not read the adjust values of the
    shape. It draws the rings, the buttons, the curves, the cascades, the stop
    and the two mixes of inflate and deflate as plain text. It leaves out
    fKern and gtextFShadow. A file with WordArt of each kind can give the size
    of the text along an arch. Source: `internal/odraw`.
13. Convert the adjust values of the presets of Office Drawing that
    `internal/odraw/adjust.go` does not convert: the trapezoid, which Office
    Drawing draws with its narrow side down, the arc, the block arc, the
    circular arrow, the bent, curved and U-turn arrows, the ribbons and the
    arrow callouts. Map the callouts of the types 178 to 181. The defaults of
    the tips of the callouts 61, 62, 63 and 106 are not known, and they take
    the defaults of ECMA-376. Source: `internal/odraw`.
14. Make the running element of the headers and the footers of `docx` a flow
    root, as `msdoc`, `rtf` and `odf` do. A footer aligned at the bottom moves
    down by the space before its first paragraph. Source: `docx`.
15. Find why page 2 of `libreoffice/doc/landscape-letter-form.doc` draws the
    black bars of its table 1 pixel lower than Word. The header line of Arial
    16 is now 18.4 points high, as in Word. Source: `msdoc`.
16. Draw the lines, the shapes, the regions and the text of a QuickDraw
    picture (PICT). `internal/pict` draws only the bitmaps of a picture and
    its images of QuickTime, for the pictures of Office Drawing of `msxls`,
    `msppt` and `msdoc`. No file of the corpus has a PICT picture. Source:
    `internal/pict`, 2026-11-18.
17. Compare a chart of Excel 97 without the records ShapePropsStream with
    Excel. `msxls` reads the theme of the record THEME, and a workbook without
    one takes the theme of Office 2007. The corpus has no such chart. Source:
    `msxls`.
18. Find why a chart that rows cut shows a light seam at some edges of its
    rows, as on page 3 of `poi/xls/three-charts.xls`. A slice of a chart
    reaches 1 point into the next row. A strip across the edge of the row, as
    a picture has, gave more seams in the charts, and the cause is not known.
    Source: `internal/sheet` and the layout.
19. Put the line gap of a single line above its text, as Word and LibreOffice
    do. The layout puts half of the gap above the text and half below.
    `-ivy-line-multiple` (D56) puts the whole gap above only for a multiple
    line spacing. With Calibri, whose gap is 0.22 em, the text of a single
    line in Word is about 1 point lower than in ivy. A test with LibreOffice
    puts the baseline of Liberation Serif 0.935 em below the top of a single
    line: the ascent and the whole gap. Source: the layout, `docx`, `msdoc`,
    `odf` and `rtf`.
20. Find why the text in the cells of the table of
    `libreoffice/doc/legal-table.doc` is lower in Word than in ivy, and why
    the rows of Word are taller. `-ivy-line-multiple` (D56) moves the text up
    in its lines, and the review of the file went from 1.52 to 1.66. Source:
    `msdoc`.
21. Find why page 2 of `libreoffice/odt/tdf146272.odt` holds fewer lines in
    ivy than in LibreOffice. The text after the caption "Figure 2" is 2 pixels
    higher than in LibreOffice. With `-ivy-line-multiple` (D56) the review of
    the page went from 8.25 to 9.05, and pages 1 and 3 got better. Source:
    `odf`.
22. Read `text:count-empty-lines`, `text:count-in-text-boxes`, the right,
    inner and outer positions, and `text:line-number` of a paragraph in the
    line numbering of `odf`. The layout draws the numbers on the left only,
    and counts the empty lines. Source: `odf` and the layout.
23. Find why the lines of the bulleted list on page 3 of
    `poi/doc/floating-pictures.doc` are about 15.2 points apart in ivy and
    14.64 in Word. The text below the list is 3 to 5 pixels lower than in
    Word. Source: `msdoc`, 2026-11-19.
24. Measure the picture effects of Word with more values. The contrast and the
    brightness of `internal/drawingml/recolor.go` come from the washout, the
    brightness of 75 percent and the contrast of 75 percent of pages 4 and 5
    of `poi/doc/floating-pictures.doc`. With a contrast below 1, Word adds
    less of the brightness, and the rule of ivy for this fits only the
    washout. On 2026-11-19 a script made pictures with 9 other values through
    COM in the Windows VM of the review, but Word 2024 stopped when the script
    saved or exported the document, so these pictures have no reference.
    Source: `internal/odraw`, 2026-11-19.
25. Measure in Word how it draws the borders of a row that a page break
    splits. `odf` gives `box-decoration-break: clone` to its tables and its
    cells, so the layout draws the borders of a split cell on both pages, as
    LibreOffice does. `docx`, `msdoc` and `rtf` keep the rule of WeasyPrint,
    with no border at the split. No file of the corpus with a reference of
    Word has a split row. Source: `docx`, `msdoc` and `rtf`, 2026-11-21.

## Left by the package icc

From the request of the maintainers of 2026-11-20 for the package `icc`
(D61).

1. Port the transforms of 16 bits of moxcms, `create_transform_16bit`, so
   that `icc` converts the colors of an image of 16 bits with 16 bits. It
   converts them with 8 bits now, and keeps the alpha of 16 bits. Source:
   item 4 of D61.
2. Read the ICC profile that an image file embeds, and export it from
   `icc`: the APP2 segments `ICC_PROFILE` of JPEG, which can split a profile
   across several segments, the `iCCP` chunk of PNG, the TIFF tag 34675 and
   the `ICCP` chunk of WebP. ivy reads none of them now. The reader of TIFF
   in `internal/gowemf` reads the tag 34675 only for the bitmaps of a
   metafile, and it refuses most TIFF files before the tag. iv reads the
   profile itself until then. Source: item 10 of D61.
3. An image in PQ or HLG looks dark after `FromCICP`, because moxcms has no
   tone mapping. A tone mapping for iv needs a decision of the maintainers
   first. Source: item 9 of D61.

## LaTeX

From the request of the maintainers of 2026-11-20. D62 decides how ivy
reads and writes LaTeX, and phase 8 of `PLAN.md` gives its steps.

1. Read LaTeX: put a `.tex` document on pages, with its macros, classes,
   packages, math, tables, figures, cross-references and bibliography.
   Phases 8a and 8b give the package `latex`. "Left by phase 8a" holds
   what is left. Source: the maintainers, 2026-11-20.
2. Write LaTeX: `latex/latexenc` writes a document of ivy as LaTeX source.
   Source: the maintainers, 2026-11-20.
3. Draw the equations of Office Math (`docx`), MathML (`html` and `epub`)
   and the formulas of OpenDocument as math. `docx` now writes the text of
   an equation in italics, in `converter.math` of `docx/paragraph.go`. This
   needs the layout of math of item 2 of "Work for okage". Source: D62,
   2026-11-20.

## Left by phase 8a

The build of phase 8a of 2026-11-21 left these items. D62 and D65 say what
was built.

1. `parseTransform` of `mathpdf.go` of upstream panics on a transform of a
   special such as `a)b(c`, which the fuzz test of `latex` found. `ivy_svg.go`
   holds the fix. ivy posts nothing to go-tex/engine (`AGENTS.md`, rule 13),
   so an update of the copy checks whether upstream changed `parseTransform`,
   and keeps the fix of ivy until it does. Source: phase 8a, 2026-11-21.
2. The engine loses the text of a document that loads the `amsmath.sty` of TeX
   Live, as `latex/testdata/math.tex` does with the tree
   `/usr/share/texmf-dist/tex`. With the files that upstream embeds, it does
   not. Find the cause in the copy of the engine. ivy posts nothing to
   go-tex/engine (`AGENTS.md`, rule 13). Source: phase 8a, 2026-11-21.
3. Give the links and the destinations of a page to the device through
   `display.Linker`, and the outline of `Doc.Outline` through
   `display.Bookmarker`, which phase 7c added. Today `Doc.Links` of
   `latex/internal/engine/ivy_driver.go` holds them, and a PDF file of a LaTeX
   document has no links. Source: phase 8a, 2026-11-21.
4. Draw the pages of `\includepdf` with the package `pdf`, as the figures
   are drawn. Today they are empty, because upstream rasterizes them with a
   renderer that a program wires. Source: phase 8a, 2026-11-21.
5. Load the real pgf and TikZ, which upstream loads only with its variable
   GOTEX_PGF, and add `pgfsys-gotex.def` of upstream, under BSD-3-Clause,
   to the files that the resolver gives. Today a picture of TikZ is a
   framed box. Source: phase 8a, 2026-11-21.
6. Run more than one engine at a time. The engine changes maps of its
   package when it builds a run, such as `expandableSet` in
   `primitives.go`, `etexif.go` and `readstreams.go`, so `Compile` runs
   one engine at a time. A change of upstream that builds those maps once
   lifts the limit. Source: phase 8a, 2026-11-21.
7. Map the limits of `ivy.Config` onto the limits of the engine: the steps
   of expansion, the depth of the input and the size of a paragraph. Today
   the engine keeps its own limits, and the context stops a run. Source:
   phase 8a, 2026-11-21.
8. Update the copy from upstream, which merges about 10 changes each day,
   as `docs/UPSTREAM.md` says. Source: D62.
9. The tests of the test module and the review command give the folder
   `fonts` to the formats with `os.DirFS`, and the folder now holds `lm`,
   which `fonts.FS` leaves out (D65). `test/cmd/perf` hides it, so that its
   hashes stay those of `fonts.FS`. Hide it in the others too, such as
   `test/review/render.go` and `test/galley_test.go`. The review now gives
   Latin Modern to a TeX document through `Config.TeX` too. Source: phase
   8a, 2026-11-21.
10. Write LaTeX with `latex/latexenc`, phase 8e of `PLAN.md`: the HTML of a
    document of the layout as LaTeX source. Nothing of it is built. Source:
    D62, 2026-11-21.
11. Report what the engine skips. In the mode that ivy uses, the engine skips
    a command or a package that it does not model, and the page loses that
    part with no error. Give the program the list of what was skipped, for
    example through `Describe`, and count it in the review. Source: the README
    of go-tex/engine, 2026-11-21.
12. Load the real files of the packages that the engine emulates: `geometry`,
    `tikz`, `hyperref`, `graphicx` and the font encodings. The engine has its
    own code for the common uses of each, and other uses of them are lost.
    Source: the README of go-tex/engine, 2026-11-21.
13. Put a `\marginpar` note in the margin, and break the page at a
    `\pagebreak` inside a paragraph. Both need the primitive `\vadjust`, which
    the engine does not have. Today the note is not placed in the margin, and
    the break inside a paragraph has no effect. Source: the README of
    go-tex/engine, 2026-11-21.
14. Hyphenate languages other than American English. The copy holds only the
    patterns of American English. The patterns of other languages come from
    hyph-utf8 under several licenses, so a program can give them, or the
    maintainers choose which ones ivy holds. Source: phase 8a, 2026-11-21.
15. Find the documents that stop early. By the count of upstream, about one
    real paper in a thousand still stops before its end, and the step limits
    can stop a long figure of pgf, such as the output of tikzDevice. Measure
    this on the corpus of LaTeX documents of the review. Source: the README of
    go-tex/engine, 2026-11-21.
16. Draw a formula that holds a command that `texmath` of okage (okage D78)
    does not know. Today the engine drops that one formula. Count these
    formulas in the review, and ask okage for the commands that real documents
    use. Source: phase 8a, 2026-11-21.

## Found by the review of LaTeX

The review of 2026-11-21 renders the 14 TeX documents of
`test/review/corpus.tsv` beside pdfLaTeX of TeX Live 2026, with `-tex base`.
Each item names a file of the corpus.

1. Build the option `WithName` of `latex` that D66 decides, and give each TeX
   document of the review the name of its file. Today `latex` gives the engine
   no job name, so the engine reads `texput.bbl`, and `repo/tex/report.tex`
   loses its bibliography and the numbers of `\cite`. Source: the review of
   LaTeX, 2026-11-21.
2. Draw the headers and the footers of fancyhdr. In
   `repo/tex/report.tex`, pdfLaTeX draws `\leftmark`, "Test Report" and
   "Page N" with their rules on each page. ivy draws an empty page with a
   header after the title page, and draws the header on two pages only.
   Source: the review of LaTeX, 2026-11-21.
3. Turn an image of `\includegraphics` with the key `angle`. In
   `repo/tex/report.tex`, ivy draws both images of the section
   "Transforms" without the turn. Source: the review of LaTeX, 2026-11-21.
4. Take the names of babel for a language that `\babelprovide` loads. In
   `repo/tex/livre.tex`, ivy writes "Contents", "Chapter" and
   "Bibliography" where pdfLaTeX writes "Table des matières", "Chapitre"
   and "Bibliographie". ivy also leaves out the blank pages of `openright`
   of the class book, its running heads, and the roman numbers of the
   front matter, so it gives 6 pages where pdfLaTeX gives 11. Source: the
   review of LaTeX, 2026-11-21.
5. Indent the environments quote, quotation and verse, and break the lines
   of verse at `\\`. Draw `\LaTeX` as the logo, not as text. Draw a prime
   in math, `x'`, as the glyph prime, not as a quote in a superscript. All
   three are on page 2 of `latex2e/tex/sample2e.tex`, whose page breaks
   then differ. Source: the review of LaTeX, 2026-11-21.
6. Draw the end of a proof of amsthm as an open box, as amsthm does, and
   keep the page breaks of pdfLaTeX. `latex2e/tex/testmath.tex` gives 40
   pages where pdfLaTeX gives 41, and its pages differ by 3 to 9 after the
   blur, most by the place of the break. Source: the review of LaTeX,
   2026-11-21.
7. Set a document of a class that the tree of `-tex base` does not hold.
   `latex2e/tex/cfgguide.tex` uses the class ltxguide: ivy shows the
   characters `|` of its short verbatim, numbers the sections, and gives 18
   pages where pdfLaTeX gives 12. With `-tex dist`, the engine loads
   `ltxguide.cls` and loses the text: it gives 2 pages. The same happens to
   `latex2e/tex/grfguide.tex`, 51 pages with `base` and 2 with `dist`. It is
   item 2 of "Left by phase 8a" with other files. Source: the review of
   LaTeX, 2026-11-21.
8. Make the engine faster on `latex2e/tex/rotex.tex` with `-tex dist`. One
   run takes about 260 seconds, and the other documents wait for it,
   because `Compile` runs one engine at a time (item 6 of "Left by phase
   8a"). With `-tex base` it takes 2 seconds. Source: the review of LaTeX,
   2026-11-21.
9. Give a slide of beamer its size, 128 by 96 millimeters, and its theme.
   ivy draws the 23 frames of `beamer/tex/beamerexample-conference-talk.tex`
   as pages of US Letter with the text of the frames only. Give a picture
   of the class standalone the size of the picture.
   `janosh/tex/multilayer-perceptron.tex` is a small framed box at the top
   of a page of US Letter (item 5 of "Left by phase 8a"). Source: the
   review of LaTeX, 2026-11-21.
10. Give a document of plain TeX the page of US Letter. ivy gives a page of
    the text with margins of one inch, 8.5 by 10.9 inches, so the pages of
    `repo/tex/plain.tex` and `knuth/tex/story.tex` are 10 pixels shorter
    than the pages of pdfTeX at 96 dots per inch. Source: the review of
    LaTeX, 2026-11-21.
11. Install pgf, the packages of `collection-latexextra` and `repstopdf` in
    the TeX Live of the machine of the review. Without pgf, the documents
    of beamer and of TikZ have no reference. Without `standalone.cls`,
    `csquotes.sty` and `imakeidx.sty`, a document that loads them has no
    reference. Without `repstopdf`, a document with an EPS figure has no
    reference, such as `latex2e/tex/rotex.tex` and
    `latex2e/tex/grfguide.tex`. Then run `render` with `-retry`. Source: the
    review of LaTeX, 2026-11-21.

## Left by the charts

1. Draw the region map of Office 2016. It draws as a frame with its
   title. Excel takes the shapes of the regions from the Bing Maps
   service of Microsoft, and no data under a license that ivy can bundle
   gives the same shapes. Excel keeps the answers of the service in the
   chart part, in `cx:geoCache`: XML that raw deflate compresses, with the
   polygons in the point compression of Bing Maps, and the copyrights of
   Microsoft and OpenStreetMap. ivy can draw the map from that cache, and
   bundle nothing. The maintainers decide whether ivy draws data of the
   service from a file. `libreoffice/xlsx/region-map.xlsx` holds such a
   cache. Source: `internal/chart` (D27).
2. Draw the depth of a 3D chart. A 3D pie draws as a flat ellipse, and the
   other 3D charts as their 2D form. Source: `internal/chart`.
3. Draw the data table of a chart whose bars go across, and the
   trendlines and the data tables of a chart of OpenDocument and of a chart
   of Excel 97 to 2003. `internal/chart` draws the data table of a chart
   of DrawingML whose categories go along the bottom, and the trendlines of
   DrawingML only. Source: `internal/chart`, 2026-11-21.
4. Find how Excel places the label of a trendline. ivy puts the right
   side of the label 9 points left of the end of the line, and its middle
   at the height of the end. In `repo/xlsx/chart-trendline-labels.xlsx`,
   Excel puts the right side 9 to 19 points left of the end. Source:
   `internal/chart`, 2026-11-21.
5. Find when Excel puts the children of a node of a treemap side by side
   in a box that is a little higher than wide. In the banners of
   `repo/xlsx/chart-treemap-banner.xlsx`, Excel puts Potato and Carrot side
   by side in a box of 126.0 by 127.1 points. The squarified layout puts
   them one over the other. Source: `internal/chart`, 2026-11-21.
6. Give a chart of Office 2016 the fill and the line of its chart area
   from its style part, `cs:chartStyle`, when the chart part has no
   `cx:spPr`. Word and Excel draw the white chart area and the gray border
   of the style, as on page 3 of `poi/docx/chartex.docx`. ivy draws no
   border. Source: `internal/chart` and its callers, 2026-11-21.

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

## Left by phase 7c

1. Draw the pages once when a page links to an anchor of a later page.
   pdfenc now draws every page a second time, because the writer of okage
   needs the destination of a link when its page is drawn. Item 3 of "Work
   for okage" asks for the change. Source: `pdf/pdfenc`, 2026-11-21.
2. Give the links of the text of a shape, such as a link of a slide of
   `pptx` or of a drawing of `odf`. The book of a text body does not set
   `BookOptions.Links`, so such a text gives no link. Source:
   `internal/drawingml`, D58.
3. Give the quad points of a link annotation of PDF. `pdf` gives the
   rectangle `/Rect` of each link, so a link that covers two lines of text
   covers the space between them too. Source: `pdf/links.go`.
4. Give the links of PDF that go to another file (`/GoToR`) or start a
   program (`/Launch`), and the actions of a `Next` chain. `pdf` gives only
   `URI` and `GoTo`. Source: `pdf/links.go`.
5. Read the links of a PDF file only for the pages that a writer asks for.
   Today the first `Draw` onto a device that takes links reads the
   annotations of every page, to find the explicit destinations that the
   links use. On a file of many pages and many links, that read comes
   before the first page. Source: `pdf/links.go`.
6. Give a link of XPS to a name that `PageContent.LinkTargets` does not
   list. Today `xps` makes an anchor only of a name that LinkTargets lists,
   because some files put a Name on every element. A link to any other
   name is dropped. Source: `xps/links.go`.
7. Give the links of the elements of a `VisualBrush` in XPS. Today the walk
   of a visual gives no link. Source: `xps/links.go`.
8. Give a link of a DjVu file the point of its target, not only the top of
   the page. NAVM and maparea name a page and no point, so every link and
   bookmark of `djvu` goes to the top of its target page. Source:
   `djvu/links.go`.
9. Give the highlights and the comments of a maparea of DjVu, such as
   `(hilite)`, `(border)` and the text of the comment. `djvu` gives only the
   URL and the box of the shape. Source: `djvu/internal/doc/links.go`.
10. In a KF8 book, give an `id` to the element at a `kindle:pos:fid`
    position that has only an `aid`, as calibre does. Today a link or an
    entry of the table of contents that targets such an element goes to
    the start of its part. Source: `mobi/kf8.go`, `links.position`.
11. Read the table of contents of a MOBI 6 book without an NCX index from
    the page that the "toc" reference of its guide names. Today such a book
    gives no bookmarks. Source: `mobi/mobi6.go`.

## Left by phase 7b

1. Give the text of a `Glyphs` element of XPS whose fill is a
   `VisualBrush`. `xps` fills the outlines of such a run as a path, so the
   run has no text in a PDF file. Source: `xps/page.go`.
2. Give the text of a color glyph and of a bitmap glyph of the layout, such
   as an emoji. The layout draws such a glyph as an image, so it has no
   text in a PDF file. The writer of okage can draw the glyphs of the tables
   `COLR`, `sbix`, `CBDT` and `SVG` as a Type 3 font with their text.
   Source: `internal/layout/draw_text.go`.
3. Keep the text of a group that the PDF device draws as pixels, such as a
   group of a blend mode that PDF does not have. The device can draw the
   text again over the pixels as invisible text. Source: `pdf/pdfenc`.
4. Remove the second drawing of the pages in pdfenc when the subset of a font
   fails, and use the check of the ranges of `DrawGlyphs` that okage exports.
   Since okage D80 the writer of okage draws such a font as paths with its
   text as invisible text, so the fallback of pdfenc in `encoder.failedFonts`
   no longer runs. The writer gives one subset to fonts of one name, so when a
   font fails, another font of the same name is drawn as paths too. Source:
   `pdf/pdfenc` and okage D80, 2026-11-21.

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
3. Write the glyphs of a Type 3 font of bitmaps as a Type 3 font of the
   written file. `Page.Draw` of `pdf` gives the image mask of each glyph with
   the pixels of the file, and pdfenc writes it as an image mask of the page.
   At 72 dots for each inch, ivy, MuPDF and Poppler each scale the image of a
   Type 3 glyph down in another way than an image of the page. So
   `repo/pdf/font-type3-widths-with-matrix.pdf` differs by 13.9 through ivy,
   27.5 through MuPDF and 39.4 through Poppler, and `pdfencDiffers` of the
   test module lists it. At 300 dots for each inch, the written page differs
   from the source page by 0 through ivy, 3.8 through MuPDF and 8.3 through
   Poppler. The writer of okage makes a Type 3 font only from the glyphs of
   a font file. Source: `pdf` and `pdf/pdfenc`, 2026-11-19.

## Left by phase 6

1. Read the outline (NAVM) and the links of the annotations of DjVu, when
   the API has a capability for them. D37 waits for one capability for
   every format. Source: phase 6.
2. Give the text layer of DjVu to a program and to pdfenc. `doc.Page.Text`
   in `djvu/internal/doc` reads it into zones, and no code calls it yet.
   D64 decides the capability `ivy.Texter`, in the phases 64a, 64b and 64c.
   Source: phase 6.

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
8. Draw the objects of OpenDocument that only an SVM picture shows, and
   `fo:clip` of the pictures of a text. No permissive decoder of SVM is
   known, and D45 lets it wait. Source: `odf`.
9. Compare the custom caps and the arrows of EMF+ lines with GDI+.
   `internal/gowemf` draws them with a geometry that MS-EMFPLUS does not
   define. Draw the triangle cap and the anchor caps (round, diamond and
   arrow), which gowemf reports. Source: `internal/gowemf`.
10. Draw a stroke with a mix mode (ROP2) that reads the page with a styled pen
    in OPAQUE mode, and the source of an EMF+ DrawImage of a metafile outside
    the metafile with a tile mode. `internal/gowemf` reports them. The blend
    modes of the raster operations are exact for black and white masks only.
    Source: `internal/metafile` and `internal/gowemf`.
11. Take a fallback face for the characters that the face of a run of a
    metafile lacks, such as Arabic in Carlito. The run draws the empty glyph
    of its face. Source: `internal/metafile`.
12. Test change 19 of `internal/gowemf` (D53) with a metafile that a program
    wrote. The corpus of the review has no record that names its glyphs by
    index, so only synthetic records test the text of the indexes. Add to the
    corpus an EMF from a printer driver or from Uniscribe, and an EMF+ Dual
    file with DrawDriverString. Source: D53.
13. Blur the reflection of DrawingML by its blurRad, and keep the reflection
    of a turned shape upright when rotWithShape is 0. `internal/drawingml`
    draws the reflection of a shape and its text through a soft mask, without
    a blur, and turns it with the shape. Source: `internal/drawingml`,
    2026-11-19.
14. Measure the glow and the soft edge of PowerPoint for more radii. ivy grows
    the shape by half the radius of a glow and blurs it with a standard
    deviation of a sixth of the radius. It makes the shape smaller by 0.8
    times the radius of a soft edge and blurs it with a standard deviation of
    a third of the radius. These rules come from a glow of 18 points and a
    soft edge of 10 points of `pptx/testdata/effects.pptx`. Source:
    `internal/drawingml`, 2026-11-19.

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
3. Allocate less in the boxes of a book. On 2026-11-21 `PageCount` of
   `idpf/epub/linear-algebra.epub` allocates 1,716 MB in 12.3 million
   objects, down from 2,088 MB in 23.3 million with okage at 5e860c67. Most
   of the rest comes from three places:
   - The lines. `splitInlineBox` of `layout_inline.go` makes about 1.9
     million objects for each count: the lists `waitingChildren`,
     `childWaitingFloats` and `newChildren`, the copies of the boxes, and
     the set of the removed sides that `Box.Copy` makes for each copy.
   - The text boxes of white space. About 400,000 of the 524,000 text
     boxes hold only white space, and `inlineInBlock` drops most of them.
     The layout cannot leave them out, because the collapsible spaces of
     `ProcessWhitespace` go from one text box to the next. The builder now
     makes them in blocks, without a new string, and a box is 352 bytes.
   - okage: `roxmltree` and `cssselect2`. See item 5 of "Work for okage".

   `zipdoc.Archive.Read` also reads each file of a book with
   `raster.ReadAll`, whose buffer grows as it reads. The size of the entry
   can give the size of the buffer. Source: `internal/layout`,
   `internal/zipdoc` and okage, 2026-11-21.
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
9. Shape each text of a paragraph once. A text that is the start or the end
   of the last long text that the layout shaped for a font takes the glyphs
   of that text (okage D70, `internal/layout/text_shape_slice.go`). Two kinds
   of texts still shape again. The short text of the next line starts in the
   last text and ends after it. A text in the middle of the last text, such
   as a word for the minimum width of a box, also shapes again. So the layout
   still shapes about 1.3 characters for each character of the lines of
   `pg1342-images.html`, and about 1.6 for `pg24264.txt`. A last text that
   holds the whole paragraph, or a cut at each end of a part, can give these
   texts their glyphs too. Source: `internal/layout`, 2026-11-19.
10. Find why the line breaks of page 3 of `idpf/epub/accessible-epub-3.epub`
    change between okage v0.18.0 and the commit `5e860c67` of okage. Four
    lines break at another word, such as "are" and "registered". The book sets
    its text in its own FreeSerif. The shaping of each of these lines, the
    advances of the glyphs and the flags unsafe-to-break are the same in both
    versions, so the cause is in another part of the layout of the line: the
    break opportunities of the segmenter, the loading of a font of
    `@font-face`, or the shaping of okage D77. No other page of the corpus
    changes. Source: the update to okage `5e860c67`, 2026-11-21.

## Work for okage

Each item is a request to okage (D8). Ask a maintainer, as `AGENTS.md` says.

1. Give the blur and the color matrix of the filters of `svg` an API on a
   `Mask` and on a `Pixmap`. `internal/drawingml` blurs the masks of the
   shadows, the glows and the soft edges with box blurs of its own, in
   `blur.go`, and changes the colors of the pictures itself, in `recolor.go`.
   Source: `internal/drawingml`, 2026-11-19.
2. Read the OpenType MATH table in `typesetting`, and give a layout of math: a
   tree of math nodes, a parser of TeX math ported from `go-tex/math`, and the
   layout of TeX Appendix G, whose result draws with okage, gives its outlines
   as paths, or gives its glyphs and rules. ivy draws the math of LaTeX,
   Office Math, MathML and OpenDocument with it, and zuzu the math of its
   Mermaid labels. Also give the metrics, the kerning of a pair and the
   ligature of a pair of glyphs, for the engine of D62. okage decides the API
   in okage D78 (`texmath`). texmath came with okage at 5e860c67. The
   helpers for the kerning and the ligature of a pair are not in okage yet,
   and `latex/internal/opentype` shapes the pair to find them. Source: D62,
   2026-11-20.
3. In `pdf`, take a link annotation to a named destination by its name
   alone, and resolve the name in `Document.Finish`, after the last page.
   Today the destination of a link must be known when its page is drawn,
   so pdfenc draws every page a second time when a page links to a later
   page. Source: `pdf/pdfenc`, 2026-11-21.
4. Allocate less in `roxmltree` and in `cssselect2` for a large document.
   `idpf/epub/linear-algebra.epub` holds 25 MB of XHTML with about 270,000
   elements. On 2026-11-21 one run of `test/cmd/perf` on it, which opens
   the book and counts its pages, allocates these:
   - `roxmltree.parse` allocates about 410 MB. The guess of the number of
     nodes, `strings.Count(text, "<")`, does not count the text nodes, so
     `appendNode` grows the list of the nodes, about 175 MB. Then
     `shrinkToFit` copies the list again, about 90 MB.
   - `processText` makes a new `textBuffer` for each text that holds an
     entity, the buffer grows, and `finish` copies it. These are about 1.1
     million objects. One buffer for the parse can serve each text.
   - `IterSubtree` of `cssselect2` makes an `ElementWrapper` for each
     element, about 270,000 objects and 63 MB, and a `frame` on the heap
     for each element, about 270,000 objects. A list of frames by value
     needs no object for each element.
   - `EtreeChildren` grows the list of the children with `append`, and the
     iterator of `Element.Children` makes two closures for each element.
     These are about 900,000 objects. An optional method of `Element` that
     gives the children as a slice, or their number, needs none of them.

   Source: item 3 of "Left by the speed of 2026-11-18", 2026-11-21.

## Work for magic

Each item is a request to magic (D8). No item is open on 2026-11-19.
