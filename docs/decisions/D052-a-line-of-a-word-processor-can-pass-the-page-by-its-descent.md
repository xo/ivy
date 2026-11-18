# D52. A line of a word processor can pass the page by its descent

Status: Decided.

Word and LibreOffice keep the last line of a page on the page when it
passes the bottom of the page by less than its descent. On page 3 of
`poi/docx/illustrative-cases.docx`, three short paragraphs after a table
pass the bottom by 1.9 points, and the descent of the last line is 2.4
points. The layout of WeasyPrint moves the last paragraph to the next page,
so the pages of ivy drift from the pages of Word. The maintainers asked on
2026-11-18 for a rule of the layout.

The maintainers decided:

1. The layout gets a property of its own, in `css_ivy.go`, that lets the
   last line of a page pass the bottom of the page by up to its descent.
   The property is off by default.
2. `docx`, `msdoc`, `odf` and `rtf` turn the property on for their
   documents. HTML, EPUB and the other formats of the web keep the rule of
   WeasyPrint.
