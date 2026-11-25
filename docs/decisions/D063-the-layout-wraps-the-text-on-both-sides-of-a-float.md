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
