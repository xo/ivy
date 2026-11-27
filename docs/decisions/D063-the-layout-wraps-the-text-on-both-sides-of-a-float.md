# D63. The layout wraps the text on both sides of a float

Status: Decided.

Word and LibreOffice wrap the text on both sides of a picture that is in
the middle of the column. The layout of WeasyPrint (D15) puts the text on
one side of a float only. On 2026-11-21 item 6 of "Found by the visual
review" in `BACKLOG.md` named this for `msdoc`. Question 26 of `PLAN.md` asked the
maintainers to choose.

## The problem

On page 6 of `poi/doc/floating-pictures.doc`, two pictures of about 1 inch
have square and tight wrap. Each picture is in the middle of the column.
Word and LibreOffice put text to the left of the picture and to the right
of it. Each row of text is in two parts: the text fills the part on the
left, and then the part on the right at the same height. Then the next row
starts.

`msdoc` floats such a picture to the side of the column where its center
is, with a margin that gives its place in the column (`floatStyle` in
`msdoc/html.go`). On page 6 the picture floats to the left, with a left
margin that puts it in the middle. So the text is only on its right, and
the room on its left is empty. The review of 2026-11-21 gives a mean difference of 23.03
on page 6 against Word and 24.54 against LibreOffice. The other pages of
the file are from 1.1 to 10.7.

CSS has no way to give this rule. The CSS Exclusions draft of the W3C has
`wrap-flow: both`, which puts the text on both sides of an exclusion. The
upstream WeasyPrint does not have it. So the formats cannot give it in
their HTML, and the layout needs a property of its own, as D52, D56 and D60
add.

The formats give the side of the text in these fields:

| Format | Field | Values |
| --- | --- | --- |
| `msdoc` | wrk of the FSPA | both sides, left only, right only, the largest side |
| `docx` | `wrapText` of `wp:wrapSquare`, `wp:wrapTight` and `wp:wrapThrough` | `bothSides`, `left`, `right`, `largest` |
| `odf` | `style:wrap` | `parallel`, `left`, `right`, `dynamic`, `biggest` |
| `rtf` | `\shpwrk` | the values of wrk |

`odf` reads `left` and `right` of `style:wrap` to choose the side of the
float. The other formats do not read these fields today.

## The options

### 1. The largest side

The formats float the picture to the side that leaves the larger room for
the text, as today, and read only the side of the text. This needs no
change of the layout. But the text stays on one side, and the room on the
other side stays empty, as on page 6 today. Option 1 is not recommended.

### 2. A property of the layout for both sides

The layout gets a property in `css_ivy.go`:
`-ivy-wrap-flow: auto | [ both | start | end | maximum ] <length>?`. The
keywords follow `wrap-flow` of the CSS Exclusions draft. The length is the
distance between the float and the text. `auto` keeps the rule of CSS. With
`both`, a line next to the float can have a part on each side of it. This
is the recommendation.

### 3. An absolute box with text wrap

Option 4 of D60 places the picture as an absolute box, and the lines avoid
it with a second set of rules. D60 did not choose it. It costs more than
option 2 and gives the same pages.

## The proposal

1. The property `-ivy-wrap-flow` applies to a float. It is not inherited.
   Its initial value is `auto`.
2. With a keyword, the float keeps its place: the side of `float` and its
   margins. The lines avoid the border box of the float and the distance
   of the property around it, and not its margins. The margins only give
   the place of the float in the column, as `msdoc` gives them today.
3. A line next to such a float has two rooms: from the start of the line
   to the float, and from the float to the end of the line. The layout lays
   out the line in the first room that holds the minimum width of the
   line. When the line ends, the layout lays out the next line in the next
   room at the same height. When no room is left, the next line starts
   below.
4. Each part of a row is a line box of its own. The text alignment and the
   justification apply to each part, as in Word.
5. A room that is narrower than a minimum width holds no text. The value
   of this width in Word is not measured. The proposal takes 0.25 inch,
   and the build measures it.
6. With `start` or `end`, the text is only on that side of the float. With
   `maximum`, it is on the side with the larger room. These values give the
   left, right and largest of the formats.
7. The line numbers of D56 number the first part of a row only. Word
   numbers a row once.
8. The galley of D57 does not keep a line next to a float, so it gives no
   part of such a row.
9. `msdoc` reads wrk, `docx` reads `wrapText`, `odf` reads `style:wrap`,
   and `rtf` reads `\shpwrk` when it reads its shapes. Each gives
   `-ivy-wrap-flow` to its floats with square, tight or through wrap.
10. HTML, EPUB and the other formats of the web keep the rule of
    WeasyPrint. The public API does not change (D11).

## The cost

| Part | Lines of Go |
| --- | --- |
| The property and its validation | about 40 |
| The rooms of a line and the second part of a row | about 200 |
| The tests of the layout | about 300 |
| `msdoc`, `docx` and `odf` | about 60 |
| The tests of the formats | about 100 |

For one agent with a maintainer, this takes 3 to 5 days.

## The tests

1. Tests of `internal/layout` cover each value of the property, a float in
   the middle of a line, a room narrower than the minimum, two floats on
   the same row, a float that starts inside a line, a page float of D60
   and the line numbers of D56.
2. The review compares page 6 of `poi/doc/floating-pictures.doc` with Word
   and LibreOffice, before and after.
3. `test/cmd/perf -hash` gives the same pixels for each HTML, EPUB and
   Markdown document of the corpus.
4. The fuzz tests of `msdoc`, `docx` and `odf` cover the property (D41).

## The open points

1. The minimum width of a room in Word and LibreOffice is not measured.
2. A tight or through wrap follows the outline of the picture in Word. The
   proposal excludes a rectangle, the border box and the distance, as
   `msdoc` and `docx` exclude a rectangle today.

## The decision

On 2026-11-21 the maintainers answered question 26 with choice 2: the layout
gets the property `-ivy-wrap-flow`, and `msdoc`, `docx`, `odf` and `rtf`
give it from the side of the text of their shapes.

## The build

The build of 2026-11-25 differs from the proposal in these places:

1. The distance applies to the left and to the right of the float. Above
   and below it, the lines avoid only the border box. The default distances
   of Word above and below a picture are 0.
2. A paragraph that ends beside the float ends at the bottom of its row of
   lines. The next paragraph starts below that row.
3. The layout puts no text in a part of a line that is narrower than 0.25
   inch.
4. `rtf` does not give the property. It draws the pictures and the text
   boxes of its shapes inline, so it has no float to give it to.
5. `odf` places a frame that floats at the edge of the column, because it
   does not read `svg:x` for a float. So a frame of `odf` gains only the
   distance.
6. On 2026-11-27 `odf` places a frame that floats at its `svg:x` and its
   `style:horizontal-pos`, in the area of its `style:horizontal-rel`. This
   replaces point 5. The frame floats to the side of the column where its
   center is, and a margin gives its place, as in `msdoc`. So the text goes
   on both sides of a frame in the middle of the column. For left, center
   and right, LibreOffice aligns the frame with its `fo:margin-left` and
   `fo:margin-right` in the area. For `svg:x`, it puts the border of the
   frame at that place. In a table cell, a frame, a header, a footer or a
   section with columns, `odf` does not know the width of the column, and
   the frame still floats at the edge of the column.
7. LibreOffice 26.8 puts the text of the wrap dynamic of `odf` on the larger
   side of the frame only. So `odf` gives `maximum` for dynamic, as for
   biggest.
8. On the test files `float-mid.odt` and `float-right.odt` of `odf`, ivy
   places the frames and the words of the text within 0.01 inch of
   LibreOffice 26.8. LibreOffice also puts the left margin of a paragraph
   again after a frame that covers the start of a line, and the layout does
   not.
9. On 2026-11-27 `rtf` floats the picture and the text box of a shape of
   the body, and gives them `-ivy-wrap-flow`. This replaces point 4. `rtf`
   reads `\shpwr`, `\shpwrk`, `\shpbx`, `\shpby`, `\shpleft`, `\shptop` and
   the properties `posrelh`, `posrelv`, `posh` and `posv`, as `msdoc` reads
   an FSPA. The wrap distances come from the properties `dxWrapDistLeft`,
   `dxWrapDistRight`, `dyWrapDistTop` and `dyWrapDistBottom`, with the
   defaults of MS-ODRAW: 9 points at the left and the right, and 0 above
   and below. A shape goes to the start of its paragraph, because Word
   places it from the top of the paragraph. A shape of a header, a footer,
   a note, a text box or a group of shapes stays in its line. On the test
   files `float-s-*.rtf` and `float-x-*.rtf` of `rtf`, ivy places the
   pictures, the text boxes and the words of the text within 0.01 inch of
   LibreOffice 26.8. LibreOffice 26.8 gives a text box other default
   distances than MS-ODRAW, so these test files give the distances.
10. On 2026-11-27 a float in the middle of the text of a line no longer
    moves the text before it into the room after it. The layout places the
    float at once, and lays out the line again in the rooms next to the
    float. So the text before the float stays in the first room.
11. On 2026-11-27 the lines of a column avoid a float of `-ivy-wrap-flow`
    of a column before it in the same row of columns, as Word and
    LibreOffice 26.8 wrap the text of each column around a shape that
    crosses the columns. The floats and the blocks of the column avoid
    such a float on the side of the column with the larger room. The
    float does not change the clearance or the height of the column. A
    column does not avoid a float of a column after it.
12. On 2026-11-27 the layout gets the property `-ivy-wrap-shape: none |
    polygon(<length> <length>, ...)`. It gives a float of `-ivy-wrap-flow`
    the polygon that the text wraps around, in place of the border box.
    Each point is a distance from the left and from the top of the border
    box. A line avoids the part of the polygon next to it, and the distance
    at the left and the right of that part. This answers open point 2. The
    blocks of a later column still avoid the border box of the float, as
    in point 11.
13. On 2026-11-27 `msdoc` gives a shape with tight or through wrap its wrap
    polygon, pWrapPolygonVertices of MS-ODRAW. The points are in a square
    of 21600 by 21600 over the box of the shape, and they turn with the
    shape. A shape that turns with square wrap gives the box around the
    turned shape. The polygon and the box leave out the bleed of the
    object of the shape. On page 6 of `poi/doc/floating-pictures.doc`,
    each part of a row next to the two pictures starts within 1 point of
    Word.
14. On 2026-11-27 `docx` gives a drawing with tight or through wrap its
    `wp:wrapPolygon`, and a VML shape with tight or through wrap its
    `wrapcoords`. Word writes the points in a square of 21600 by 21600 over
    the extent of the drawing, and not in EMU. On the test file
    `float-s-tight.docx` of `docx`, ivy places the words next to a diamond
    within 0.2 point of LibreOffice 26.8.
15. On 2026-11-27 test files measured the narrowest room in LibreOffice
    26.8. LibreOffice puts text in a room of any width that holds the first
    word of the line, such as a room of 4 points that holds an "i" of 12
    points. The pages of Word of the corpus have no room narrower than 0.25
    inch next to a picture, so the width of Word is not measured. The layout
    keeps the rule of point 3.
16. On 2026-11-27 `-ivy-wrap-flow` gets the keyword `margin`, which `odf`
    gives. A line that starts after a float that covers the start of the
    line keeps the margin of its paragraph at the start side. LibreOffice
    26.8 starts such a line at the frame, plus the distance, plus the left
    margin of the paragraph. This answers the last sentence of point 8.
    LibreOffice does not keep the right margin of a paragraph before a
    frame at the end of a line. Word is not measured, so `msdoc`, `docx` and
    `rtf` do not give the keyword. On the test file `float-margin.odt` of
    `odf`, ivy starts the lines within 0.2 point of LibreOffice 26.8.
17. On 2026-11-27 `odf` places a frame that floats in a table cell, a
    frame, a header, a footer or a section with columns at its `svg:x`.
    This replaces the last sentence of point 6. Without the width of the
    column, the frame floats to the left, and a margin from the left edge
    of the content of its paragraph gives its place. A frame at the right
    floats to the right with a margin from the right edge. A frame at the
    center, or in an area of the page, still floats at the edge of the
    column. On the test files `float-cell.odt` and `float-section.odt` of
    `odf`, ivy places the frames within 0.5 point of LibreOffice 26.8. In a
    header, LibreOffice 26.8 places such a frame at its `svg:x` too, but
    does not wrap the text of the header around it.
18. On 2026-11-27 `rtf` gives a shape with tight wrap, `\shpwr4`, its wrap
    polygon, the property `pWrapPolygonVertices`. The points are in a
    square of 21600 by 21600 over the box of the shape, as in `msdoc`.
    LibreOffice 26.8 wraps the text of an RTF file with through wrap,
    `\shpwr5`, around the box of the shape, and not around the polygon. So
    `rtf` gives the polygon only for tight wrap. `rtf` does not turn or flip
    a shape that floats, so the polygon does not turn. On the test file
    `float-s-tight.rtf` of `rtf`, ivy places the words next to a diamond
    within 0.3 point of LibreOffice 26.8. `rtf` also gives `-ivy-wrap-flow:
    both` to a table that floats with half an inch of the column on each
    side of it (D60), as LibreOffice 26.8 puts the text on both sides of
    such a table. The lines then avoid only the border box of the table
    above and below it, as for a picture.
19. On 2026-11-27 a point of `-ivy-wrap-shape` can be in percentages of
    the width and of the height of the border box. `docx` gives them for
    the `wrapcoords` of a VML text box with `mso-wrap-style` none. Word
    draws such a text box as wide as its longest line of text, plus the
    insets of its text box and its line, and not at the width of its style.
    It keeps the left side and the height of the shape. For an ellipse,
    the width counts the insets of the width of its style. So `docx` gives
    such a shape no width, and the layout gives it the width of its text,
    which does not wrap. On page 1 of
    `libreoffice/docx/mixed-orientation.docx`, Word draws the ellipse 102.3
    points wide, and ivy 103.2 points. The mean difference from Word goes
    from 8.30 to 7.29. Word draws the text of the ellipse with the insets
    of the drawn width, so its text is 5.7 points more to the left than in
    ivy.
