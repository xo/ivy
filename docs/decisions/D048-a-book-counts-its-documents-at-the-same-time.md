# D48. A book counts its documents at the same time

Status: Decided.

D47 renders a page in bands and several pages at the same time, but the
layout of a book stayed serial. `PageCount` of
`idpf/epub/linear-algebra.epub`, a book of 623 pages, took 6.9 s on one
goroutine. The maintainers asked on 2026-11-18 to lay out the documents of
an ebook at the same time.

A book counts its pages document by document, and `place` already keeps the
count of a document when the number of pages before it changes. So the
count of a document does not depend on the documents before it. Two things
of the book are shared between its documents: the font configuration, whose
maps and faces of go-text are not safe for concurrent use, and the lock of
the book.

The maintainers decided:

1. Before `PageCount` walks the documents, `countAll` lays out each document
   whose count is not known, on up to one goroutine for each CPU, and at
   most 8. Each goroutine has a font configuration of its own, over the
   font database of the program, which is safe for concurrent use.
2. A goroutine keeps the count of the document and drops its layout,
   because the layout does not have the number of pages before it. The
   walk lays out a document again when the program asks for one of its
   pages, as before.
3. A document that uses `counter(pages)` needs the counts of the documents
   after it (D43), so it keeps no count from `countAll`. Neither does a
   document whose layout fails or panics. The walk lays these out in order,
   as before, with the errors and the logs of the book.

A measurement on 2026-11-18 on a computer of 28 CPUs gave these times for
`PageCount`, with the same pages and the same pixels as before:

| Book | Pages | Before | After |
| --- | --- | --- | --- |
| `idpf/epub/linear-algebra.epub` | 623 | 6.9 s | 1.4 s |
| `gutenberg/epub/pg24264-images-3.epub` | 842 | 2.3 s | 0.54 s |
| `idpf/epub/jlreq-in-japanese.epub` | 610 | 2.2 s | 0.56 s |

A book of one document, such as a text file, an HTML file or an FB2 file,
does not change, because it has one document to lay out.
