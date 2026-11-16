# D26. ivy reads DjVu with a copy of the decoders of dejapdf

Status: Decided.

DjVu is a format for scanned books. A page holds layers: a mask of the text
and the line art in JB2, the colors of that mask, and a background picture
in IW44 wavelets. A page can also hold its text and its links, and a book
can hold an outline. Phase 6 of `docs/PLAN.md` gives ivy the pages of DjVu
files, from a copy of the decoders of
[dejapdf](https://github.com/sergev/dejapdf) (MIT, pure Go), after the
backlog examines them.

On 2026-11-17 the backlog examined dejapdf at the commit
`2b6ca0c1123cc45f868cca3be99dd8123d5cdc6f`. dejapdf is a program that
converts DjVu to PDF and makes scanned PDF files smaller. All its code is in
`internal/` packages, so no program can import it. Its decoders of DjVu are
`iff`, `zp`, `bzz`, `jb2`, `iw44` and `djvu`, about 4,500 lines. Its packages
`jbig2`, `jpx`, `mq`, `stencil`, `shapes` and `segment` encode, and its
packages `pdf`, `pdfread`, `rewrite`, `reencode` and `scan` read and write
PDF. Those packages carry parts under the Apache License 2.0.

The maintainers asked for phase 6 on the same date, and they decided:

1. ivy copies the decoders of DjVu of dejapdf into `djvu/internal/`, with
   the license of dejapdf (D7). Only the format `djvu` uses them, so they
   stay inside it (D6).
2. ivy does not copy the encoders, the readers and writers of PDF, or the
   decoders of CCITT and JPEG of dejapdf. ivy has its own decoders of CCITT
   and of JPEG, and its own reader of PDF.
3. The package `djvu` gives the pages of a DjVu file, single or bundled,
   with its layers drawn onto the display list (D10), and the thumbnails
   that the file holds. An indirect document names other files, and a
   source of ivy has no folder, so it returns an error that wraps
   `ivy.ErrUnsupported`.
4. DjVuLibre is under the GPL, so it runs only as an oracle in the test
   module (D7, D13).
