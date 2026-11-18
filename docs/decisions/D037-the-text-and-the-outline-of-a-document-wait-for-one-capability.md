# D37. The text and the outline of a document wait for one capability

Status: Decided.

DjVu has a text layer, an outline and links. PDF has text and an outline.
EPUB and MOBI have a table of contents. Question 11 of `PLAN.md` asked
whether ivy has an API for them.

On 2026-11-18 the maintainers decided:

1. ivy has no API for the text, the outline or the table of contents of
   one format.
2. When a program needs them, one decision designs one capability for
   every format (D11).
3. The items of the backlog for them stay, and wait for that decision.
