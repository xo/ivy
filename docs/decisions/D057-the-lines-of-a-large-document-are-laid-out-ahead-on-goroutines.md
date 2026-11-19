# D57. The lines of a large document are laid out ahead on goroutines

Status: Decided.

D48 lays out the documents of a book at the same time. A book of one
document, such as a long text file or one large HTML file, gets nothing
from it. Item 3 of "Left by the speed of 2026-11-18" in `BACKLOG.md` names
this. On 2026-11-19 the maintainers asked for a design that lays out the
pages of one large document on several goroutines. Then a book of one
document counts and shows its pages faster. They asked for a proposal and
not for code (D3).
Question 22 of `PLAN.md` asks the maintainers to choose.

## The problem

`test/cmd/perf` measured these times on 2026-11-19, on a computer of 28
CPUs that other work shared. "First page" is the time to open the file and
draw page 1. "Count" is the time of `PageCount` after it.

| Document | Pages | First page | Count |
| --- | --- | --- | --- |
| `gutenberg/txt/pg24264.txt` | 338 | 107 ms | 1.29 s |
| `gutenberg/html/pg1342-images.html` | 192 | 112 ms | 1.34 s |

An experiment timed the steps of the layout of each document on one
goroutine. The lower time is for a second layout in the same process. The
higher time is for the first layout, which also loads the fonts.

| Step | `pg24264.txt` | `pg1342-images.html` |
| --- | --- | --- |
| Read the HTML | 69 to 130 ms | 25 to 29 ms |
| Compute the styles | 6 ms | 20 to 22 ms |
| Build the boxes | 2 to 4 ms | 17 to 27 ms |
| Lay out the pages | 0.95 to 2.0 s | 0.92 to 1.24 s |
| Make the margin boxes | 36 to 51 ms | 16 to 18 ms |

The layout of the pages is almost all the time. Inside it, 90% of the time
of each document goes to `getNextLinebox`, which breaks the text into lines
and shapes the glyphs of each line. The rest places the lines and the blocks
on pages.

| | `pg24264.txt` | `pg1342-images.html` |
| --- | --- | --- |
| Blocks in the body | 485 | 2,167 |
| Lines laid out | 19,490 | 9,989 |
| Different lines | 18,416 | 9,321 |
| Lines laid out next to a float | 0 | 1,686 |
| Forced page breaks | 0 | 3 |

Each page starts from a state that the page before it leaves. The layout
keeps the state of each page in a `pageMaker`:

1. The resume point: a skip stack, which names the child box at each level
   and the byte of the text where the page starts.
2. The kind of the break before the page, the side of the page, the depth of
   the quotes, the counters of the page and the page groups of the named
   pages.

The layout context also carries state from one page to the next. It holds
the floats and the absolute boxes that a page break cut, and the footnotes
that did not fit. It holds the running elements and the named strings of
each page, which the margin boxes read. The index of a page selects `@page
:first` and `:nth()`. If two layouts start a page with the same state, they
lay out the same pages after it.

The pages must stay the same, pixel for pixel. These features can change at
a cut:

1. The counters and `counter(pages)`.
2. The footnotes and the floats.
3. The running headers and footers, and `string-set`.
4. The page breaks inside the tables and the lists.
5. The widows and the orphans.
6. The overflow of D52, and the collapse of the margins at the top of a
   page.

## The experiments

The experiments of 2026-11-19 ran on a copy of the repository, outside it.
A hash of the position, the size and the text of each box of a page compared
two layouts of a page. `perf -hash` compared the pixels of every page at 96
dpi.

### 1. Parts that join on the state of a page

Each part starts at a block of the body, on a new page. It lays out pages
until a page of the part before it starts with the same state as one of its
own pages. The parts join at that page. The pages of a part before the join
are dropped. This is the probe of D55 without the rows: the state of a page
replaces the probe row.

| Document | Pages that start at a block | Parts | Join after |
| --- | --- | --- | --- |
| `pg24264.txt` | 18 of 338 | 8 | no part joins in the whole document |
| `pg1342-images.html` | 104 of 192 | 8 | 2, 22 and 50 pages, then part 4 does not join in 96 pages |

The pages after a join were the same: 122 of 122 pages. But the parts
rarely join. The lines of a paragraph all have the same height. Two layouts
that start a page at different lines of a paragraph keep the same offset in
lines on each page of that paragraph. Only the edges of the paragraphs can
change the offset, such as a margin that the layout drops at the top of a
page. They rarely make it zero.

### 2. A galley of lines

A galley is a column of the lines of the text, with no page breaks. The
experiment laid out the document on one page of 10<sup>8</sup> pixels. It
kept each line with its inputs: the box of the line, the resume point inside
it, the width of the containing block, the text indent and the bottom space.
Then it laid out the pages. When a line had the same inputs, with no float
next to it, the layout took a copy of the line from the galley.

The first try took lines that differed by about 10<sup>-13</sup> pixels.
The galley laid out a line at one height, and the layout moved it to
another height. Such a line does not round as a line laid out at that
height. So the experiment laid out each line at a height of 0 and then
moved it, in the galley and in the pages. A line next to a float, or a line
that holds a float or a positioned box, stays at its real height.

| | `pg24264.txt` | `pg1342-images.html` |
| --- | --- | --- |
| Lines taken from the galley | 19,490 | 7,883 |
| Lines laid out again | 0 | 2,106, of which 1,686 are next to a float |
| Lines from the galley that differ from the layout | 0 | 0 |
| Pages with the same boxes | 338 of 338 | 192 of 192 |
| Time of the galley | 0.93 s | 0.87 s |
| Time of the pages with the lines of the galley | 49 to 94 ms | 0.27 s |
| Time of the pages without the galley | 0.95 to 1.8 s | 0.92 to 0.99 s |

The lines at a height of 0 change the boxes of 64 of the 192 pages of
`pg1342-images.html` by about 10<sup>-13</sup> pixels. `perf -hash` gave
the same pixels for all the pages of both documents, with and without the
change.

### 3. The galley in parts on goroutines

The experiment cut the blocks of the body into 8 parts of about the same
length of text. Each part read, styled and built the document itself, with
a font configuration of its own, as `countAll` of D48 does. It kept the
blocks of its part and laid them out as a galley. All the parts ran at the
same time, in a new process.

| Document | Time of all the parts | Time of each part | Of which reading, styles and boxes |
| --- | --- | --- | --- |
| `pg24264.txt` | 0.36 s | 0.29 to 0.36 s | 58 to 72 ms |
| `pg1342-images.html` | 0.35 s | 0.31 to 0.35 s | 97 to 118 ms |

The experiment did not join the lines of a part to the pages of another
build. Experiment 2 used the lines of a copy of the same boxes.

## The options

### 1. Parts that start at the forced page breaks

At a forced page break, the next page starts at a known block. A part can
start there, with the counters and the side of the page that the part
before it gives. The pages stay the same. But `pg24264.txt` has no forced
break, and the 3 forced breaks of `pg1342-images.html` are in its first 3%.
The option gives no speedup on these documents. It suits a long HTML file
whose chapters start on a new page. Option 1 is not recommended now.

### 2. Parts that join on the state of a page

Experiment 1 tested this option. It keeps the pages the same, and it needs
no change in a format. But the parts of a flow of text rarely join, so the
pages wait for the parts before them, as in option 2 of D55. Option 2 is not
recommended.

### 3. The shaping on goroutines, and the pagination on one goroutine

The shaping is about three quarters of the time of a line, in a profile of
2026-11-19. But the text that the layout shapes depends on where the line
starts and on its width. To shape ahead, a goroutine must break the lines
first, and that is option 5. The layout can also shape a whole paragraph
once and reuse the glyphs for each line. That needs the glyphs that are
safe to break, item 2 of "Work for okage" in `BACKLOG.md`. Option 3 is not
recommended alone.

### 4. A cache of the lines on one goroutine

The layout lays out some lines more than once. Examples are the lines of a
paragraph that moves to the next page, and the lines that widows and
orphans take again. That is 5.5% of the lines of `pg24264.txt` and 6.7% of
`pg1342-images.html`. A cache saves at most these lines. Both documents need
one pass of the layout, so a cache across the passes of `counter(pages)`
saves nothing here. Option 5 includes this cache.

### 5. The recommendation: lines ahead on goroutines

Goroutines lay out the lines of the later blocks as a galley. One goroutine
lays out the pages, as today, and takes a line from the galley when the line
has the same inputs. The pagination does not change, so each page keeps its
state and each feature above stays as it is.

## The proposal

1. A document starts the galley only when a program asks for all the
   pages, in `PageCount`. It also starts it for a page far after the pages
   that the layout knows. `Page(0)` does not start it, so the first page
   does not share the CPUs.
2. The layout cuts the blocks of the body into parts of about the same
   length of text, with at least 64 KB of text in each part. The first part
   has no galley, because the pages lay out its lines first. A document
   with less text has no galley.
3. Each part runs on a goroutine, on up to one goroutine for each CPU, and
   at most 8 (D47, D48). It reads, styles and builds the document with a
   font configuration of its own, as `countAll` does. It lays out the
   blocks of its part on one page of unbounded height. It drops a float
   whose bottom is above the next line, because such a float cannot touch
   a later line.
4. The galley keeps a line with its inputs: the index of the box of the line
   in tree order, the resume point inside the box, the width and the left
   edge of the containing block, the text indent and the bottom space. It
   keeps no line that holds a float, a box with `position: absolute`,
   `fixed` or `running()`, or a note, and no line of `::first-line` or
   `::first-letter`. The layout of such a line changes the layout context.
5. The galley and the pages lay out a line at a height of 0, and then move
   it to its height. A line next to a float, or a line that holds a float
   or a positioned box, stays at its real height. This is a difference
   from upstream, with a comment of the port.
6. The pages take a line from the galley only when no float is next to the
   line in the layout of the pages, and the inputs are the same. A line of
   the galley points to the boxes, the styles, the elements and the font
   configuration of its own build. The pages take a copy of the line, and
   map each pointer onto the document by the index in tree order. A line
   with a pointer that the document does not have, such as a style that the
   galley made during the layout, is laid out again.
7. The glyph runs name their faces by their `svg.FontID`. The faces of the
   program and the `@font-face` faces of the document come in the same
   order in each font configuration, so the IDs are the same. A line with a
   face that the font configuration of the document does not have at that
   ID is laid out again.
8. The pages do not wait for a part. If the lines of a part are not ready,
   the pages lay out the lines themselves. The layout stops the galley of a
   part that the pages passed. A part whose layout fails or panics
   gives no lines. The cancellation of the context stops the parts.
9. The pages keep the lines of a part until they pass the part. The galley
   keeps at most `Limits.MaxObjects` lines.
10. The public API does not change (D11).

The pagination stays on one goroutine, so these features stay the same:

1. The counters and `counter(pages)`. A document that uses `counter(pages)`
   lays out its pages again (D43), and the passes take the same lines.
2. The footnotes. The block layout lays out a footnote when it places the
   line of its call, as today.
3. The floats. A line next to a float, or a line that holds one, is laid
   out by the pages.
4. The running headers and footers, and `string-set`. A line that holds a
   running element is not in the galley. The margin boxes read the named
   strings from the boxes of the page, as today.
5. The page breaks inside the tables and the lists, the widows and the
   orphans, the overflow of D52, and the collapse of the margins. The block
   layout decides them, as today. It only takes its lines from the galley.
6. The named pages and `@page :left` or `:right` with other margins. A page
   of another width gives another width to the containing block, so the
   lines do not match and the pages lay them out.

## The cost

The code has about 150 lines of Go for the cache of the lines and the map
of the pointers, and 150 lines of Go for the parts and their goroutines in
`Document`. The placement at a height of 0 takes about 30 lines of Go. The
tests take about 400 lines of Go.

The galley lays out the lines of each part after the first one, and the
pages lay out again each line that they cannot take. Each part reads,
styles and builds the document again, in 60 to 120 ms. The memory holds a
font configuration for each goroutine, and the lines of the parts until the
pages pass them.

With 8 parts, a count takes about the sum of three times:

1. The reading, the styles and the boxes of the document.
2. The time of all the parts. The pages of the first part lay out their own
   lines while the other parts run.
3. The time of the pages with the lines of the galley.

| Document | Count today | Expected count | Speedup |
| --- | --- | --- | --- |
| `pg24264.txt` | 1.29 s | about 0.5 s | about 2.5 times |
| `pg1342-images.html` | 1.34 s | about 0.7 s | about 2 times |

The floats of `pg1342-images.html` keep 21% of its lines on one goroutine.
The first page does not change, because `Page(0)` starts no galley.

## The tests

1. A test of the test module lays out each document of the corpus of the
   review whose kind goes through `internal/layout`, such as `txt`, `html`,
   `md`, `fb2`, `epub`, `rtf`, `docx`, `odt`, `doc`, `xlsx` and `csv`, with
   the galley and without it. It compares the number of pages and the hash
   of the pixels of each page. An option of the test makes parts of one
   block, so that each document has many parts.
2. Tests of `internal/layout` put each feature at the start of a page and
   next to a line of the galley: a float across a page break, a drop cap, a
   positioned box in a line, a running element, a note, a footnote call,
   `::first-line`, `::first-letter`, a text indent at the top of a page,
   tabs and leaders, a table cell across a page break, a list, widows and
   orphans, `-ivy-page-overflow`, a named page of another width, `@page
   :left` with other margins, `counter(page)` in the content, and an
   `@font-face` font.
3. `perf -hash` gives the same hashes before and after the lines at a
   height of 0, on the whole corpus of the review, in a commit of its own.
4. `go test -race` lays out the documents of the tests with the galley in
   parts.
5. The fuzz tests of `html`, `text` and `markdown` cover the code, with a
   small size of a part (D41).
6. `test/cmd/perf -hash` gives the same hashes before and after, and the
   time of the count of the two documents above.

## The open points

1. Each part reads, styles and builds the document again, as D48 does. The
   other way shares the boxes of the document, with a copy for each part,
   which takes 0.2 to 2.7 ms. But a style computes its values when the
   layout first reads them, and it keeps the fonts of its font
   configuration. For that way, the styles must be complete before the
   parts start, and a style must not hold a font configuration.
2. The size of a part, 64 KB of text, and the bound of 8 goroutines.
3. Whether `Page(n)` for a page far after the known pages starts the galley,
   as `partsTo` of D55 does for the parts of a sheet.
4. The lines at a height of 0 move the boxes by about 10<sup>-13</sup>
   pixels. The pixels of the two documents stay the same. The rest of the
   corpus is not measured yet.
5. The experiments did not cover footnotes, running elements, tables,
   `::first-line` or the join of lines across two builds.

## The decision

On 2026-11-19 the maintainers accepted the galley, and answered question 22
with these choices: each part builds the document again for now, a part
holds at least 64 KB of text, at most 8 goroutines lay out parts, `Page(n)`
for a page far ahead also starts the galley, and each line is laid out at a
height of 0 and then moved, after a check on the whole corpus.

## The build

The build of 2026-11-19 differs from the proposal in these places:

1. The galley and the pages do not drop a float. Each one ignores a float
   of the formatting context that ends above the line, and hides it while
   the line is laid out at a height of 0. This gives the result of point 3
   of the proposal.
2. A part gives lines only when its font configuration has the same faces
   and the same `@font-face` loads as the document. The pages also check
   that the document loaded no new face after the part. This is stricter
   than point 7, because the font configuration of a book can hold faces
   of other chapters.
3. The galley also keeps no line with a footnote call, an image, an
   inline-block or a box with the counters of a target or of a page. No
   pointer to the build of a part reaches the pages.
4. The galley does not start inside the goroutines of `countAll` (D48). A
   book of many documents gets no galley for its count.
5. If the document lays out its pages again for `counter(pages)`, the
   parts that the pages passed keep their lines until the layout is
   complete.
6. The test of the test module cuts each document into 16 parts of one
   block, not one part for each block.

`perf -hash` gave the same pixels for every page of the 116 documents of
the corpus that go through the layout, with and without the galley. The
count of `pg24264.txt` went from 1.3 s to 0.44 s, and the count of
`pg1342-images.html` from 1.2 s to 0.76 s.
