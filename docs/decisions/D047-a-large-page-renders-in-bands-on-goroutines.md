# D47. A large page renders in bands on goroutines

Status: Decided.

The maintainers asked on 2026-11-18 whether ivy can draw on more than one
CPU, for example several pages at the same time. Each page drew on one
goroutine, and a slow page, such as a PDF file of many transparency groups,
used one CPU for 28 s. D11 makes a document safe for concurrent use, and
`Page.Render` safe when each call has its own pixmap. So a program could
already draw several pages at the same time, but ivy itself did not.

A prototype drew the bands of a page on goroutines, each band with the
transform moved up by its top. It found three problems:

1. A band of a layout page was slower, not faster. The drawing of a layout
   page held the lock of the book, so the bands ran one after the other, and
   each band recorded the whole page again and rendered each SVG image in
   full.
2. Each band paid for the objects outside it, and most for its clips: a
   clip made a mask of the size of the band, also when it was outside the
   band.
3. A pixel near the edge of a band differed from the same pixel in the
   middle of a band, and the pixels depended on the number of CPUs.

The maintainers decided:

1. `RenderImage` renders a page of 262,144 pixels or more in bands of 256
   rows. Each band renders into a pixmap of its own, with 32 rows more above
   and below, which it does not keep, and copies its rows into the image.
   The bands run on up to one goroutine for each CPU, and at most 8. The
   bands do not depend on the number of CPUs, so a page gives the same
   pixels on each computer.
2. The okage device of `internal/display` skips an object whose bounds are
   outside the bounds of its clip: a path, a stroke, a run of glyphs, an
   image and a mask. A clip outside the bounds of the clip makes no mask,
   and a group with no area makes no layer. These objects draw nothing, so
   the pixels do not change. A clip inside a clip clones the mask of the
   state, and only the first clip makes a new mask.
3. A layout page records its drawing once into a `display.List`, under the
   lock of the book, at its transform without the whole pixels of its
   translation. Each band replays the recording onto its own device without
   the lock, moved by a whole number of pixels, so it sends the device the
   commands of a drawing at its own transform. A list records the content of
   a soft mask at once, as a list of its own, so goroutines can replay one
   list at the same time. A book keeps the last 4 recordings.
4. `RenderPages` renders several pages of a pager at the same time, on up to
   one goroutine for each CPU, and yields the images in the order of the
   pages. It renders the bands of each page one after the other, so the
   goroutines do not multiply, and it gives the pixels of `RenderImage`.

A measurement on 2026-11-18 rendered the first 3 pages and the last page of
each of the 256 documents of the corpus of the review at 96 dpi, on a
computer of 28 CPUs that other work shared:

| Kind | Documents | Before | After |
| --- | --- | --- | --- |
| all | 256 | 142.1 s | 83.8 s |
| pdf | 50 | 73.4 s | 29.4 s |
| ai | 5 | 6.9 s | 2.2 s |
| psd | 6 | 5.5 s | 2.3 s |
| xls | 11 | 5.7 s | 3.1 s |
| epub | 10 | 15.1 s | 14.9 s |

No kind became slower. The page of `pdfjs/pdf/transparency-blend-modes.pdf`
went from 27.9 s to 8.5 s. The pixels of 163 documents stayed the same. The
pixels of the others differ by the rounding of the transform of a band, in a
few pixels at the edges of shapes. The okage device without bands gives the
same pixels as before for each of the 256 documents.

`Page.Render` of a format still draws a page in one call. A program that
calls it directly gets no bands. The layout of a flow of text, such as an
EPUB file, is still serial: the time of `PageCount` of a large ebook does
not change.
