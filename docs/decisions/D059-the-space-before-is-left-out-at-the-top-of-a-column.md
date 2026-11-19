# D59. The space before is left out at the top of a column

Status: Decided. Amends D56.

Item 3 of D56 leaves out the space before the first paragraph of a page
when the page starts after a break that is not forced. Word does the same
at the top of each column of a section with columns. On 2026-11-19 `msdoc`
started to read the columns of a section, and the review found that the
layout kept the space before at the top of a column.

The maintainers decided on 2026-11-19 that `-ivy-page-top-space: discard`
also leaves out the space before the first block of a column in these
places:

1. Each column after a column that a forced break did not end.
2. The first column of a page that starts after a break that is not
   forced.
3. The first block inside the first block of such a column.

A forced column break keeps the space before, as a forced page break does.
The layout keeps the column at the top of which it is in
`layoutContext.columnTopBox`, with a "Port:" comment for each place where
it differs from WeasyPrint. `TestPageTopSpaceColumns` tests each case.
