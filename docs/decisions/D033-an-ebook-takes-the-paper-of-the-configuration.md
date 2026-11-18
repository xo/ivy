# D33. An ebook takes the paper of the configuration

Status: Decided. Amends D32.

D32 gives `Config.Paper` to a document whose file gives no paper. The
style sheet of an ebook can give a size in `@page`, but that size is rarely
meant for print, and calibre does not use it. Question 6 of `PLAN.md` asked
whether ivy keeps the size of the book.

On 2026-11-18 the maintainers decided:

1. An ebook takes `Config.Paper`, or 6 by 9 inches when it is zero, and the
   margins of the book style of `internal/ebook`, over the `@page` of its
   own style sheet. The ebooks are EPUB, MOBI and AZW3, FictionBook, HTMLZ
   and TXTZ.
2. An EPUB of fixed layout keeps the size of the viewport of each page, as
   its file asks.
3. A plain HTML, text, Markdown or CHM file keeps the `@page` of its own
   style sheet, and takes `Config.Paper` only without one, as D32 says.
