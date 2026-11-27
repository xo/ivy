# D60. The layout places a float at a position on the page

Status: Decided.

Word places a frame, a table that floats and a shape at a vertical
position from the top of the page or of the margin. The text wraps around
it. The layout of WeasyPrint (D15) puts each float at the line of its
paragraph. So ivy puts such a float in another place than Word.
On 2026-11-19 item 8 of "Found by the visual review" in `BACKLOG.md`
named this for `msdoc`. On 2026-11-19 the maintainers asked for a design that places a
float at its position on the page. They asked for a proposal and not for
code (D3). Question 24 of `PLAN.md` asked the maintainers to choose.

## The problem

These properties give a float a vertical position from the page or from
the margin:

| Format | Property | Positions |
| --- | --- | --- |
| `msdoc` frame | sprmPPc (pcVert), sprmPDyaAbs, sprmPWr | a distance, top, center, bottom, inside, outside |
| `msdoc` table | sprmTPc (pcVert), sprmTDyaAbs | the same |
| `msdoc` shape | the FSPA and the properties of Office Drawing | a distance from the margin or the page |
| `docx` table | `w:tblpPr` with `w:vertAnchor` page or margin, `w:tblpY` or `w:tblpYSpec` | a distance, top, center, bottom, inside, outside |
| `docx` frame | `w:framePr` with `w:vAnchor` page or margin, `w:y` or `w:yAlign` | the same |
| `docx` shape | `wp:anchor` with `wp:positionV` and `wp:wrapSquare`, `wp:wrapTight` or `wp:wrapThrough`, and VML with `mso-position-vertical-relative` | a distance or an alignment |
| `odf` frame | `style:vertical-rel` page or page-content with `style:wrap` | a distance or an alignment |

The formats do this with them today:

1. `msdoc` uses the position of a frame or a table only when the frame or
   the table starts a page (`blockTop` in `msdoc/table.go`). Then its
   paragraph is at the top of the margin, and a margin at the top of the
   float gives the position. In every other place, the float stays at the
   line of its paragraph. A shape with wrap stays at the line of its
   paragraph.
2. `docx` does not read `w:tblpPr` or `w:framePr`. A table that floats is a
   block in the flow, and the paragraphs of a frame are ordinary paragraphs.
   A shape with wrap stays at the line of its paragraph, and its
   position from the page or the margin is not used.
3. `odf` places a frame that is anchored to the page with `position:
   absolute`, so the text does not wrap around it. A frame that is anchored
   to a paragraph floats at that paragraph.
4. A float without wrap, such as `wr` 3 of `msdoc` or `wp:wrapNone` of
   `docx`, already has its position. Its box has `position: absolute`, and
   the containing block of an absolute box in the flow of a page is the
   page area. Such a float is not part of this decision.

The layout places a float in `floatLayout` (`layout_float.go`). The top of
the float is the current position of the flow, and `findFloatPosition`
moves it down below the earlier floats and beside them. The lines that the
layout placed before the float stay where they are. So a float cannot go
above the line of its paragraph. Only the floats that the previous page cut
are placed before the flow of a page, in `makePageGrown`.

D57 adds a rule. The pages take a line from the galley only when the line
has the same inputs. `lineAtZero` in `galley_line.go` gives a line to the
galley only when no float of the formatting context is next to the line.
A float below the top of the line counts as next to it. A float that the
page places before the flow must keep this rule.

## The experiments

The experiments of 2026-11-19 ran on a copy of the repository and on files
outside it. The review VM was not used.

### 1. LibreOffice

A script made 11 `.docx` files of Letter paper with margins of 1 inch.
Paragraph k starts with the word `Pk` and has about 3 lines. A float is
4320 twips (3 inches) wide, at the left of the margin, and the text wraps
around it. LibreOffice 26.8 made a PDF of each file. `pdftotext -bbox` gave
the place of each word. The table gives the places in inches from the top
of the paper.

| File | The float | Its anchor | LibreOffice places the float | The text beside it |
| --- | --- | --- | --- | --- |
| `t-late` | a table of 4 rows of 0.5 inch, 1 inch below the top of the margin | before P12, at 9.5 on page 1 | page 1, at 2.0 | P01 to P03, before the anchor |
| `t-page2` | the same | before P26, on page 2 | page 2, at 2.0 | P15 to P17 |
| `f-late` | a frame of 2 inches, 1 inch below the top of the margin | after P11, at 9.5 on page 1 | page 1, at 2.0 | P01 to P03 |
| `s-late` | a picture of 2 inches, 1 inch below the top of the margin, square wrap | in P12, at 9.5 on page 1 | page 1, at 2.0 | P01 to P03 |
| `f-bottom` | the frame, at the bottom of the margin | after P01, at 2.3 on page 1 | page 1, from 8.0 to 10.0 | P11 and P12, after the anchor |
| `t-bottom` | the table, at the bottom of the margin (`w:tblpYSpec`) | before P02, at 2.3 on page 1 | page 1, from 8.0 to 10.0 | P11 and P12 |
| `t-two` | two tables at the same place, 1 inch below the top of the margin | before P02 and before P05 | both on page 1, at 2.0, one on the other | P02 and P03 |
| `t-loop3` | a table of 12 rows of 0.5 inch, at the top of the margin | before P10 | page 1, at 1.0 | P00 to P05. P10 stays on page 1, at 9.7 |
| `t-loop2` | the same table | before P12, at 8.8 on page 1 without the table | page 2, at 1.0 | P12 to P17. P12 moves to page 2 |
| `t-nofit` | a table of 6 rows of 0.5 inch, 9.5 inches below the top of the paper | before P06 | 2 rows on page 1, at 9.5, and 4 rows at the top of page 2 | P06 and P07, on page 2 |

These facts come from the table:

1. LibreOffice puts the float on the page of its anchor, at its position.
   The text before the anchor on that page wraps around the float, when the
   float is above the anchor (`t-late`, `t-page2`, `f-late`, `s-late`).
2. Two floats at the same place cover each other (`t-two`). In `.docx`, a
   table that floats can cover another one unless `w:tblOverlap` is
   `never`.
3. In `t-loop2`, the text beside the table pushes the anchor to page 2. Then
   LibreOffice puts the table and its anchor on page 2. Page 1 ends after
   P11, with 1.2 inches free at its bottom, although P12 fits there without
   the table.
4. LibreOffice cuts a table that does not fit (`t-nofit`). It also moves
   the anchor and the text after it to page 2, so page 1 is empty between
   4.9 and 9.5 inches.

LibreOffice saved `t-late`, `t-page2`, `f-late`, `s-late` and `f-bottom`
as `.doc` files. It wrote the tables with sprmTPc and sprmTDyaAbs, with
pcVert 0 (the margin) and dyaAbs 1440. It wrote the frames and the picture
as shapes, with their position from the margin and square wrap.

### 2. ivy today

A copy of `msdoc`, with the code of its frames, wrote the HTML of the
`.doc` files of LibreOffice. A test of `internal/layout` laid out the HTML
and printed the place of each word.

| File | LibreOffice | ivy |
| --- | --- | --- |
| `t-late.doc` | the table on page 1 at 2.0, P01 to P03 beside it | the table at 8.7 on page 1, at the line of its anchor. Its page cuts it after 2 rows. P12 and P13 are beside it |
| `t-page2.doc` | the table on page 2 at 2.0, P15 to P17 beside it | the table at 8.7 on page 2, cut after 2 rows. P26 to P28 are beside it |

The table is 6.7 inches lower than in LibreOffice, and the text beside it
is other text.

### 3. The corpus

A scan of the corpus of the review found no float that needs this
decision:

1. Of the 17 `.docx` files, `aspose/docx/rendering.docx` has one table with
   `w:vertAnchor="text"`, from its paragraph. `poi/docx/drawing.docx` has
   one shape with `wp:positionV relativeFrom="line"`. No file has
   `w:framePr`.
2. Of the 10 `.doc` files, `libreoffice/doc/legal-table.doc` and
   `libreoffice/doc/line-numbers.doc` place their tables and frames from
   the paragraph. `libreoffice/doc/landscape-a4-certificate.doc` has one
   shape from the page, without wrap, which is absolute today.
3. No `.odt` file has `style:vertical-rel` page or page-content.

So the corpus cannot measure this decision, and the review cache holds no
PDF of Word of such a float. The pages of the corpus stay the same with the
change.

## The options

### 1. The position is a lower bound

The float stays at the line of its paragraph. When its position is lower
on the page, the layout moves it down to its position. This needs one
layout of each page, and the float never covers text. It gives the result
of LibreOffice for `f-bottom` and `t-bottom`. But in `t-late`, `t-page2`,
`f-late` and `s-late` the float stays at its anchor, 6.7 inches too low.
These are the frequent cases in Word: a frame or a table whose anchor is
later than the place where it shows. Option 1 is not recommended.

### 2. The float goes to its position when the layout meets it

The layout places the float at its position when the flow meets its
anchor, also above the anchor. This needs one layout of each page. But the
lines before the anchor are already placed, so the float covers them. In
`t-late`, the table covers the text of P01 to P03, and the review cannot
read that text (D5). Option 2 is not recommended.

### 3. The page floats of CSS

The CSS Page Floats draft, of the GCPM work of the W3C, has `float: top`
and `float: bottom` with `float-reference: page`. They put a float at the
top or the bottom of the page or the column. They have no distance, so
they give only the top and the bottom of Word. The upstream WeasyPrint
does not have them. Option 3 is not recommended.

### 4. An absolute box with text wrap

The box has `position: absolute` at its place on the page, as a float
without wrap has today. A property, such as `wrap-flow` of the CSS
Exclusions draft, makes the text wrap around it. This needs the same second
layout of the page as option 5, because the lines above the anchor must
know the box. The layout places an absolute box after the flow of its
containing block, and `avoidCollisions` knows only floats. So option 4
needs a second set of rules for the shapes that the lines avoid. `wrap-flow:
both` can later give text on both sides of a picture (item 6 of the same
backlog list). Option 4 is not recommended now.

### 5. The recommendation: the page maker places the float before the flow

A property of the layout gives a float its position on the page. When the
float is above its anchor, the page maker makes the page again. It places
the float before the flow, as it already does for the floats that the
previous page cut. The text before the anchor then wraps around the float,
as in LibreOffice. Only a page with such a float is laid out again.

## The proposal

1. The layout gets a property of its own in `css_ivy.go`:
   `-ivy-float-position: auto | [ page | margin ] && [ <length> | top |
   center | bottom ]`. The initial value is `auto`, which keeps the rules
   of CSS. The property is not inherited.
2. The property applies to a box with `float: left` or `float: right`
   whose formatting context is the one of the root box of the page. Such a
   box is a page float in this decision. In another formatting context, such
   as a table cell, a column or a flex item, the box is an ordinary float.
3. `margin <length>` puts the top of the border box of the float at that
   distance below the top of the page area. `page <length>` puts it at that
   distance below the top of the page box. A negative length moves it up.
   `top`, `center` and `bottom` put the border box at the top, the center
   or the bottom of the area. The margins of the float stay its distances
   from the text around it.
4. The float keeps its horizontal place: the side of `float` and its
   margins in its containing block, as today.
5. When the flow meets a page float, the layout lays out the float without
   a page break, as it lays out the footnote area. It gives the float its
   place on the current page. A float can pass the bottom of its area. Then
   the layout moves it up until it fits, but not above the top of the area.
   The layout does not move a page float for the other floats, so two page
   floats can cover each other, as in `t-two`. The lines and the ordinary
   floats avoid a page float as they avoid any float.
6. The top of the margin box of the float can be at or below the current
   position of the flow. Then the float covers no line, and the page has one
   layout (`f-bottom`, `t-bottom`).
7. Else the float covers content that the page already placed. When the
   layout of the page ends, the page maker makes the page again from the
   same state. It places each such float before the flow, after the floats
   that the previous page cut. When the flow meets the anchor of a placed
   float, the flow skips the float (`t-late`, `t-page2`, `f-late`,
   `s-late`).
8. The anchor of a placed float can leave the page that the page maker
   made again (`t-loop2`). Then the page maker makes the page a third time,
   without that float. The float goes to the next page, before its flow, as
   a float that the page cut goes there.
9. The second or the third layout of a page can meet a new page float.
   That float gets its place by point 5, and does not make the page again.
   So a page has at most three layouts.
10. The layout of a page changes the state of the layout context. The page
    maker keeps this state, and sets it again before it makes the page
    again. The state holds the floats and the footnotes that the previous
    page cut, and the footnotes and the notes of the page. It also holds the
    running elements, the named strings, the anchors and the counters of the
    page. `pageIter` already makes a page again for `counter(pages)` (D43),
    from the state of the previous page.
11. A part of the galley (D57) does not place a page float. Only the pages
    place it.
12. The galley stops a part only when the pages complete a page after the
    part. Today the pages stop a part when a line of a later part is taken.
    A page that the page maker makes again can start in a part that its
    first layout passed.
13. The formats give the property:
    - `msdoc` gives it to its frames and its tables from pcVert and dyaAbs,
      with the special values top, center and bottom. It also gives it to
      its shapes with wrap. `blockTop` and its rule for a block that starts
      a page go away.
    - `docx` gives it to `wp:anchor` with square, tight or through wrap,
      and to VML with `mso-position-vertical-relative` page or margin. It
      reads `w:tblpPr` and `w:framePr`, which it does not read today.
    - `odf` gives it to a frame with wrap and `style:vertical-rel` page or
      page-content.
    - `rtf` does not read its frames today. They stay in the backlog.
14. The formats map inside to top and outside to bottom, see the open
    points. A float with wrap above and below gets a margin to the other
    side of the column, so that no line fits beside it.
15. The public API does not change (D11).

The rule of D57 holds for these reasons:

1. From the start of the page, a page float is in the excluded shapes of
   the root formatting context. `lineAtZero` sees it. So the pages lay out
   each line above it or next to it, and take no such line from the galley.
2. A second layout of a page takes the same lines from the galley as the
   first layout, with the same keys.
3. The parts of the galley do not place page floats, so no line of a part
   depends on one. A line that holds the anchor of a float holds a box out
   of the normal flow. The galley never keeps such a line.

## The cost

| Part | Lines of Go |
| --- | --- |
| The property and its validation | about 60 |
| The place of a page float | about 120 |
| The layouts of a page again, with the state that the page maker keeps | about 150 |
| The stop of the parts of the galley | about 20 |
| The tests of the layout | about 400 |
| `msdoc` | about 60 |
| The shapes of `docx` | about 40 |
| `w:tblpPr` and `w:framePr` of `docx` | about 250 |
| `odf` | about 40 |
| The tests of the formats | about 200 |

For one agent with a maintainer, the layout, `msdoc` and the shapes of
`docx` take about 1 week. The tables and the frames of `docx`, and `odf`,
take about 1 more week.

A page with a page float above its anchor is laid out two or three times.
One page of the documents of D57 takes 3 to 6 ms. A page without a page
float has one layout, as today. On a page with a page float, the pages
lay out the lines above the float and next to it themselves. So the galley
gives fewer lines on such a page.

## The tests

1. The 11 files of the experiments, and `.doc` files that LibreOffice
   makes from them, go in `testdata`. The test module compares the place of
   each float with the PDF of LibreOffice, within 0.05 inch. It also
   compares the place of the first word of each paragraph. For `t-loop2`
   and `t-nofit`, the test expects the differences of the open points.
2. Tests of `internal/layout` cover each value of the property: `margin`
   and `page` with a length, and `top`, `center` and `bottom`. They put a
   float above its anchor, below its anchor and on the next page. They
   also cover a float that does not fit, a float taller than the page
   area, and two floats at the same place. A test pushes the anchor to the
   next page, and counts the layouts of the page, at most three. A test
   puts the property on a float in a table cell, where it does nothing.
3. Tests of `internal/layout` make a page again with each kind of state of
   point 10. They use a footnote, a note, a running element, `string-set`,
   `counter(page)` and a float that the previous page cut. They also use
   `-ivy-page-overflow` (D52), and the line numbers and the space before at
   the top of a page (D56).
4. The test of the test module of D57 lays out the files of test 1 with
   and without the galley, with parts of one block. It compares the
   pixels of each page. `go test -race` lays them out with the galley.
5. `test/cmd/perf -hash` gives the same pixels for every page of the
   corpus before and after the change. No file of the corpus has a page
   float.
6. The fuzz tests of `html`, `msdoc` and `docx` cover the property (D41).
7. Unit tests of `msdoc` and `docx` give the property for each position of
   the table at the start of this decision.

## The open points

1. A float that does not fit below its position. LibreOffice cuts it and
   moves its anchor to the next page (`t-nofit`). The proposal moves the
   float up until it fits, and does not cut it.
2. A float that pushes its anchor to the next page. LibreOffice moves the
   anchor with the float, and ends the page before the anchor (`t-loop2`).
   The proposal moves only the float, so the anchor can stay on the page.
   The tables and the frames of `msdoc` are blocks. For such an anchor, the
   third layout can break the page before the anchor.
3. Word was not measured, because the review VM was not used, and the
   cache holds no file with such a float. When the files of test 1 join the
   corpus, the review VM can measure Word.
4. The meaning of inside and outside on the vertical axis (MS-DOC YAS -16
   and -20, and `w:yAlign` of `docx`) is not measured. The proposal maps
   inside to top and outside to bottom.
5. The proposal does not keep two floats apart. `w:tblOverlap="never"` and
   `allowOverlap="0"` of `docx` are not read.
6. A page float in a section with columns stays an absolute box without
   wrap, as a shape of `docx` in columns is today.
7. The galley gives no line above a page float on its page. A finer test of
   `lineAtZero` can keep a line whose bottom is above the float. It is not
   measured.
8. Text on both sides of a float, item 6 of the same backlog list, is not
   part of this decision.

## The decision

On 2026-11-19 the maintainers answered question 24 with choice 5: the
property `-ivy-float-position`, with a second layout of the page when the
float is above its anchor. A float that does not fit moves up until it
fits on the page. A float that pushes its anchor to the next page stays on
its page, and the anchor goes on.

## The build

The build of 2026-11-19 differs from the proposal in these places:

1. A page with page floats has at most two layouts, not three. The
   decision keeps a float on its page when its anchor goes to the next
   page, so the third layout of point 8 of the proposal, which takes the
   float away, is not needed. A page has four layouts at most when
   `-ivy-margin-box-fit` also grows its margins.
2. A float at the top, the center or the bottom that is higher than its
   area starts at the top of the area. A negative length can still put a
   float above its area.
3. The rule of CSS that a float is never higher than an earlier float
   leaves out the page floats. Without this, a float at the bottom of the
   page pushes the later floats of the flow down to it.
4. The floats that the page places before the flow do not make the page
   count as not empty, so the flow breaks as in the first layout.
5. A page float inside a line is placed after that line.
6. In a section of `msdoc` with more than one column, the layout places
   such a float as a float of the flow. `docx` does not write the property
   in such a section.
7. Wrap above and below gets the margin of point 14 in the shapes of
   `msdoc` and `docx`, and not only in the tables and the frames.
8. The frames of `rtf` are not built. Point 10 builds the frames of `odf`
   that are anchored to the page.
9. On 2026-11-25 the property `-ivy-float-overlap: avoid` came. It moves a
   page float down below the page floats before it that it covers. `docx`
   gives it for `w:tblOverlap` never and for `allowOverlap` 0. A second
   layout of the page keeps the place that the first layout gave such a
   float. LibreOffice 26.8 moves such a table in the same way. It does not
   move such a picture, and ivy moves it, as ECMA-376 says.
10. On 2026-11-27 `odf` gives the property to a frame or a shape that is
    anchored to the page, with a `style:wrap` other than run-through, and
    with `style:vertical-rel` page or page-content, or without it. The
    frame floats at its `svg:x`, and its wrap gives `-ivy-wrap-flow`
    (D63). A frame with the wrap none gets the margin of point 14 of the
    proposal. `fo:margin-top` and `fo:margin-bottom` give the distances of
    the text above and below the frame. A frame without `style:wrap` or with
    the wrap run-through stays an absolute box. So does a frame on a page
    with columns. On the test files `float-page-*.odt` of `odf`, with the
    wraps left, right, parallel and none, ivy places the frame and the
    words of the text within 0.01 inch of LibreOffice 26.8. The frame goes
    on the page where its element is in the text, which is the first page.
    `odf` does not read `text:anchor-page-number`.
11. On 2026-11-27 `rtf` gives the property to its frames and to its
    shapes. This replaces point 8. A frame of `rtf` is a paragraph with the
    words of a frame of RTF 1.9.1, such as `\absw`, `\phmrg`, `\pvpg` and
    `\posy`. The paragraphs with the same words share one frame. A frame at
    a position from the margin or from the page floats there, as a frame of
    `docx` does. A frame without `\pvmrg`, `\pvpg` or `\pvpara` has its
    position from the margin, as in LibreOffice 26.8. A frame with
    `\pvpara` and a width floats at the top of the next paragraph, and its
    position is a margin above it. `\nowrap` gives the margin of point 14
    of the proposal, and `\overlay` gives an absolute box. `\dxfrtext`,
    `\dfrmtxtx` and `\dfrmtxty` give the distances of the text. A frame of a
    drop cap, of a table, of a header, a footer or a note, or of a section
    with columns stays a paragraph. A shape with wrap floats at its
    position from the margin or from the page, from `\shpby` or from the
    property `posrelv`. Its property `posv` gives top, center and bottom.
    On the test files `float-f-*.rtf` and `float-x-*.rtf` of `rtf`, ivy
    places the frames, the text boxes and the words of the text within 0.01
    inch of LibreOffice 26.8. LibreOffice 26.8 places a picture of a shape
    from its paragraph, and ignores `\shpbymargin`, `posrelv` 0, `posh` and
    `posv` for it. So only the unit tests of `rtf` test such a picture.
12. On 2026-11-27 `docx` gives `-ivy-float-overlap: avoid` to a VML
    shape with `o:allowoverlap` off, as for `allowOverlap` 0. In a section
    with columns, a drawing or a VML shape with square, tight or through
    wrap floats at its paragraph, in its column, as in `msdoc` (point 6).
    This replaces open point 6 for these shapes. `docx` takes the width
    of a column of the section for the side of the float. A shape that
    crosses the right side of its column floats at the left, with a margin
    that gives its place. The layout makes the lines of the later columns
    of the same row avoid such a float, as Word and LibreOffice 26.8 do
    (D63). A VML shape with the wrap top and bottom in a section with
    columns stays an absolute box without wrap. Word and LibreOffice place
    it from the top of the part of its paragraph on the page, and the
    layout cannot float it there. A float with a position from the margin
    or from the page in a section with columns floats at its paragraph.
    A continuous section of `docx` also gets its own left and right
    margins, on the page of the section before it too, as in Word. The
    pages that start in the section keep the page and the headers of the
    section before it. On page 1 of `libreoffice/docx/mixed-orientation.docx`,
    the mean difference from Word goes from 14.59 to 8.97. Word draws the
    ellipse of that page 102 points wide, and not 140 points as the file
    says. LibreOffice draws it 140 points wide, and ivy places the text
    around it as LibreOffice does.

The restore of the saved state of a page also no longer keeps the name of
a running element that the page added. The page lost the rest of a float
that the page before cut, and `-ivy-margin-box-fit` had the same fault.
On its test files, ivy places the floats within 0.03 inches of LibreOffice
26.8, except in the two cases where the decision differs from LibreOffice.
The pages of the corpus do not change.
