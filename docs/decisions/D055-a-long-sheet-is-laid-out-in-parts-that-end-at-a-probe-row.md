# D55. A long sheet is laid out in parts that end at a probe row

Status: Decided. Amends D48.

`internal/sheet` makes one HTML document of each sheet. Before the first
page of a sheet, the layout reads the whole document, computes the styles
of each cell and builds the boxes of all the rows. On 2026-11-18 the
maintainers asked for a design that cuts a long sheet at its page breaks,
so that the first page does not wait for the whole sheet. They asked for a
proposal and not for code (D3). Question 21 of `PLAN.md` asks the
maintainers to choose.

## The problem

`test/cmd/perf` measured these times on 2026-11-18, on a computer of 28
CPUs that other work shared. "First page" is the time to open the file and
draw page 1. "Count" is the time of `PageCount` after it.

| Document | Pages | First page | Count |
| --- | --- | --- | --- |
| `datasets/csv/world-cities.csv` | 1456 | 1.37 s | 1.97 s |
| `poi/xlsx/many-merges.xlsx` | 1064 | 0.95 s | 0.36 s |

A profile of the first page of `world-cities.csv` gives 0.22 s to read the
HTML, 0.39 s to compute the styles, and 0.67 s to build the boxes. The
boxes include the anonymous boxes of the table and the collapsed borders,
0.32 s. `internal/sheet` writes the HTML in 0.03 s. Each of these costs
grows with the number of rows. The layout of page 1 itself is small.

`internal/sheet` cannot cut the sheet at its page breaks by itself. The
layout makes a row taller when its text needs more room than the row has,
and the size of the text depends on the fonts, which come from the program.
So only the layout knows where a page starts.

The pages must stay the same, pixel for pixel. These features of a sheet
can change at a cut:

1. The page numbers: `counter(page)` for `&P`, and `counter(pages)` for
   `&N`, which makes the layout lay out the whole book first (D43).
2. The print titles: a `thead` that each page of the table repeats.
3. The collapsed borders. The border between two rows comes from both rows.
   The table takes its left and right border widths from the first row of
   its grid, as CSS 2.1 says. The border under a repeated `thead` comes
   from the first row of the body of the table, on every page.
4. A merged cell, a `td` with `rowspan`, that holds rows on both sides of a
   page break. Its top and bottom borders come from the rows before and
   after it.
5. The pictures and the charts. A picture of up to 1024 rows shows as one
   slice in each row. A taller picture shows whole in its first cell, and
   can reach past the end of its page.
6. The manual breaks of rows and columns, and the bands of columns, down
   then over or over then down. Each band is a table that starts on a new
   page.
7. The scale of fit to page, the centering on the page, the header and the
   footer, the comments at the end, the note, and the limit of the cells
   (`Limits.MaxObjects`).

## An experiment

An experiment of 2026-11-18, outside the repository, cut the HTML of each
sheet at the real page breaks of its whole layout. It laid out each part on
its own, drew the pages at 96 dpi and compared the hash of the pixels of
each page with the page of the whole sheet. It gave each part the number of
pages before it and after it, so that `&P` and `&N` stayed right. The parts
follow these rules. A "cut row" is the first row of a part. It starts a
page in the layout of the whole sheet, and the page before it ends with the
row before it.

1. The part starts with a copy of the first row of the body of the table,
   without its `rowspan` attributes. The table then takes the same left and
   right border widths, and the same border under the `thead`.
2. Then come the lead rows. They are the row before the cut row, and they
   start one row before the first row of each merged cell that holds the
   row before the cut row. The cut row gets `break-before: page`. The
   pages before the page that starts at the cut row are dropped.
3. The part ends with the probe row, the cut row of the next part. It must
   start a page. Then come the tail rows, up to one row after the last row
   of each merged cell that holds the probe row. The page that starts at
   the probe row, and the pages after it, are dropped.

The experiment read the 29 spreadsheets of the corpus of the review that
open without a password: `xlsx`, `xls`, `ods` and `csv`. In 14 sheets of 9
of these files, a page starts inside a table. The corpus has no merged cell
across a page break, so the experiment also built 3 workbooks of 700 rows.
They have merged cells of 7 rows and of 3 rows by 2 columns, wide and double
borders, text that wraps, rows of 30 points, 19 frames of pictures in
slices, print titles, a footer with `&P` and `&N`, fit to width, and two
bands of columns with a title column.

| Sheets | Cuts | Pages compared | Same pixels |
| --- | --- | --- | --- |
| 14 sheets of the corpus and the 3 built workbooks, parts of up to 4 pages | 264 | 859 | all |
| `world-cities.csv` and `many-merges.xlsx`, parts of 16 pages | 80 | 1280 | all |

46 cuts of the built workbooks needed lead rows of a merged cell, and 30
needed tail rows. Each rule is necessary. The experiment left out one rule
at a time, on the 3 built workbooks and the 4 sheets of
`poi/xls/basic-expense-template-2011.xls`, 120 cuts:

| Left out | Cuts with the same pixels |
| --- | --- |
| nothing | 120 |
| all the rules: a new table that starts at the cut row | 52 |
| the copy of the first row | 91 |
| the lead rows of the merged cells, with only the row before the cut | 85 |
| the probe row and the tail rows: the part ends at the row before the probe | 82 |
| the tail rows | 116 |

The experiment did not cover a picture of more than 1024 rows, the order
over then down, a manual break at a cut, the comments at the end, or the
limit of the cells inside a part.

## The options

### 1. A probe row after a cut that the heights predict

`internal/sheet` predicts the page breaks from the heights of the rows, as
`rowPages` does for the order over then down. Each part ends with a probe
row, and the layout reports whether the probe row starts a page. The parts
do not depend on each other, so `PageCount` lays them out on goroutines
(D48), and page 1 needs only the first part.

A probe row alone does not keep the pixels. The part needs the copy of the
first row and the lead rows of the experiment. A prediction also fails when
the rows grow, such as a text that wraps, or a font of the program that is
higher than the row.

The experiment compared the page breaks of `rowPages` with the page breaks
of the layout. The prediction agrees on 8 of the 14 sheets of the corpus.
On the other 6, the layout puts 47 rows on a page and `rowPages` predicts
46. The 4 long sheets of the corpus are among these 6: `world-cities.csv`,
`country-codes.csv`, `many-merges.xlsx` and `fraction-formats.xls`.
`rowPages` keeps 2 points for the borders of the table, and the layout fits
47 rows of 14.52 points in 684 points. So no probe row of these sheets
starts a page. The prediction agrees on 2 of the 3 built workbooks. In the
third one, text that wraps in narrow columns makes the rows taller, and the
prediction fails. A prediction for the parts must keep only the half
widths of the outer borders, and it fails where the rows grow.

### 2. Parts that start where the last page of the part before starts

The layout of a part reports the first row of its last page. The next part
starts at that row, and the last page of each part is laid out twice. No
prediction is needed. With the rules of the experiment, the pages stay the
same. But each part waits for the part before it. `PageCount` cannot lay out
the parts on goroutines, and the last page waits for the whole sheet.

### 3. A first pass that measures the rows with the fonts

A first pass finds the height of each row, and then `internal/sheet` cuts
exactly. To find the height of a row, the pass must lay out the text of
each cell with the fonts of the program, the fallback fonts and the breaks
of the lines. That is the work of the layout itself, and a second copy of
its rules for the heights of the rows. The cost is close to the cost of the
whole layout, and the copy can disagree with the layout. Option 3 is not
recommended.

### 4. A lazy layout inside one document

The layout computes the styles and builds the boxes of a row only when a
page needs the row. The cascade of the port of WeasyPrint works on the
whole tree of elements. The anonymous boxes of a table, and its collapsed
borders, work on the whole table. This option changes the core of the port,
which then differs from upstream (D15). Option 4 is not recommended.

### 5. A document that goes on with the page of the document before it

The layout reports the height that a part uses on its last page, and the
next part goes on with the same page. A page of a book then holds two
documents. The rows of a merged cell and the borders between the rows
cross the two documents. This is the item "Lay out a long sheet in parts"
of `BACKLOG.md`. Option 5 is not recommended.

### 6. The recommendation: options 1 and 2 together

The parts follow the rules of the experiment. The cuts come from the
prediction of option 1. When a probe row does not start a page, the part
keeps its pages before the page of the probe row, and the next part starts
again at the first row of that page, as in option 2. The parts after it are
then cut again from that row.

## The decision

On 2026-11-18 the maintainers decided the proposal below, with parts of
about 16 pages, and answered question 21.

1. `internal/sheet` writes the HTML of the whole sheet once, as today. So
   the styles, the limit of the cells and the note stay the same. It keeps
   the offset of each row of each table. A part is a slice of these rows,
   with the head of the document, the `colgroup`, the `thead`, the copy of
   the first row, the lead rows, the probe row and the tail rows. Each part
   has the same style sheet.
2. `internal/sheet` predicts the page breaks of each table from the heights
   of its rows, the scale and the print titles. It keeps the half widths of
   the outer borders of the table, not 2 points. It makes a part of about
   16 pages, and makes no part for a sheet of fewer than 32 pages. It cuts
   only between two rows. It does not cut where the lead rows or the tail
   rows are more than 256 rows, or inside the rows of a picture of more
   than 1024 rows. The part grows past such a cut.
3. A table that starts a new page, such as a band of columns or a table of
   the order over then down, is a cut that needs no lead rows and no probe
   row. The experiment did not test such a cut. The last part of a sheet
   holds the end of the document: the comments and the note.
4. In `internal/layout`, a document of a book can name a cut row and a
   probe row. The document drops the pages before the page that starts at
   the cut row, and the pages from the page that starts at the probe row.
   The dropped pages do not count. `counter(page)` of the first page that
   the document keeps is the number of pages before it, plus 1.
   `counter(pages)` counts only the pages that the documents keep.
5. When the probe row does not start a page, the document keeps the pages
   before the page that holds the probe row. It reports the first row of
   that page. The book then asks `internal/sheet` for a new next part that
   starts at that row. The source of the layout gets a function for this.
   It is internal, and the public API does not change (D11).
6. `PageCount` lays out the parts on goroutines (D48). The count of a part
   is good when the probe of the part before it started a page. The walk
   of `PageCount` lays out again, in order, each part after a probe that
   failed. `Page(n)` for a page past the parts that the book knows counts
   the parts before it in the same way.
7. A document whose `counter(pages)` is only in its margin boxes, such as
   a sheet whose header or footer has `&N`, keeps the count that `countAll`
   gives it, because the number of pages does not change its layout. This
   amends D48. A document that uses `counter(pages)` in its content still
   waits for the whole book (D43).

## The cost

The code is about 300 lines in `internal/sheet`, 150 lines in
`internal/layout` and 150 lines in the book, with their tests. Each part
lays out its lead page, its probe page and its tail pages a second time,
and reads its style sheet again. With parts of 16 pages, the layout of the
whole sheet does about 2 pages more for each 16 pages, about 12 percent
more work, before the goroutines of D48. The book keeps the layout of 4
documents, so a long sheet no longer holds the boxes of all its rows.

In the experiment, a part of 16 pages gave its first page, with its lead
page, in 18 ms for `world-cities.csv` and in 16 ms for `many-merges.xlsx`.
The whole sheet gave its first page in 1.37 s and in 1.01 s in the same
run. These times cover the reading of the HTML, the styles, the boxes and
the layout, and not the drawing.

The first page of a sheet is fast whatever the prediction. The pages of the
first part before its probe row are right, and a probe row that fails only
moves the start of the next part. The prediction matters for `PageCount`
and for a page far into the sheet. Where the rows grow often, such as a
CSV file of long fields, the parts are laid out one after the other, as in
option 2.

## The tests

1. A test of the test module lays out each sheet of the corpus whole and in
   parts, for `xlsx`, `xls`, `ods`, `csv` and a TSV file, and compares the
   number of pages and the hash of the pixels of each page. It uses parts
   of 1 page and of 4 pages, so that each sheet has many cuts.
2. Tests of `internal/sheet` build workbooks with each feature of the list
   above at a cut: a merged cell across the cut, a wide border on the row
   before the cut, a first row with wider side borders than the other
   rows, print titles of rows and of columns, a picture in slices and a
   picture of more than 1024 rows, a manual break at the cut and next to
   it, both orders of the bands, fit to page, `&P` and `&N`, the comments
   at the end, and the limit of the cells inside a part.
3. A test makes a text that wraps, so that the rows are taller than the
   prediction. The probe row fails, the book cuts again, and the pages stay
   the same.
4. The fuzz tests of `csv` and `xlsx` cover the code of the parts (D41).
5. `test/cmd/perf -hash` gives the same hashes before and after, and the
   time of the first page of the two documents above.
