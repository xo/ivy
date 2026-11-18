# D43. The layout lays out the whole document when a page needs the later pages

Status: Decided.

`Document.Page` lays out the pages up to the one that a program asks for,
so the first page comes fast. `counter(pages)`, the target counters and
the boxes with `position: fixed` of later pages need the whole layout, and
were wrong on an early page. `counter(pages)` also restarts in each
document of a book. Question 19 of `PLAN.md` asked which matters more.

On 2026-11-18 the maintainers decided:

1. The layout lays out the whole document before it gives a page, when the
   document uses `counter(pages)`, a target counter (`target-counter`,
   `target-counters` or `target-text`), or a box with `position: fixed`.
2. Another document keeps the lazy layout, so its first page comes fast.
3. A book lays out all its documents when one of them uses
   `counter(pages)`, so that the count covers the book.
