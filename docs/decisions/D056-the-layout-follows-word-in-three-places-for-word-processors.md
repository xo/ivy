# D56. The layout follows Word in three places for word processors

Status: Decided. Amended by D59.

The review found three places where the layout of WeasyPrint puts the lines
of a document of a word processor in other places than Word does. CSS has
no way to give these rules, so the formats cannot give them in their HTML.
The maintainers asked on 2026-11-19 for a property of the layout for each.

The maintainers decided:

1. The layout draws the line numbers of a section, as Word does. A property
   of `css_ivy.go` gives the distance from the text, the step, the first
   number and when the numbers restart: on each page, on each section, or
   never. The layout draws a number at the baseline of each line box of a
   paragraph in the flow. It does not number the lines of a table, a float
   or a margin box. `msdoc` and `docx` read the line numbering of each
   section.
2. The layout puts the extra space of a multiple line spacing below the
   text of a line, as Word does. CSS puts half of it above the text and half
   below. The layout also puts the whole line gap of the font above the
   text, where CSS puts half of the gap above and half below. A test with
   LibreOffice puts the baseline of Liberation Serif 0.935 em below the top
   of the line, which is the ascent and the whole gap. With this rule, the
   lines of Calibri at a spacing of 1.15 are at the same height as in Word.
   The maintainers accepted this addition on 2026-11-19. A property of
   `css_ivy.go` turns this on.
3. The layout leaves out the space before the first paragraph of a page when
   the page starts after a break that is not forced, as Word does. A
   property of `css_ivy.go` turns this on.

Each property is off by default. `docx`, `msdoc`, `odf` and `rtf` turn on
the ones that their application uses. HTML, EPUB and the other formats of
the web keep the rules of WeasyPrint.

## The build of 2026-11-25

Until 2026-11-25 the layout put the line gap above the text only for a
multiple line spacing above 1. A single line kept half of the gap above the
text and half below. Since then, item 2 also applies to a single line: the
formats give `-ivy-line-multiple: 1`, and the whole gap goes above the
text. A multiple below 1, and a line that is not higher than its text,
keep the rule of CSS. `docx` gives no value to a single line on a document
grid, because Word centers that text. The single lines of Verdana and of
Tahoma take the ratios of Word, 1.2153 and 1.207.

## The build of 2026-11-27

LibreOffice leaves out the extra space of a multiple line spacing in three
places, and keeps the spacing above a paragraph in one more place. The
measures come from files that LibreOffice 26 converted to PDF, and from
`libreoffice/odt/landscape-a4-table.odt`.

1. LibreOffice keeps the last line of a page when its text fits and only
   its extra space passes the bottom of the page. The keyword `last` of
   `-ivy-line-multiple` makes the layout do this: the line stays on the
   page, and it loses its extra space. `odf` writes `last` for the
   paragraphs of the flow of the page that have a proportional line
   spacing above 100%.
2. LibreOffice leaves out the extra space below the last line of each part
   of a table cell when the setting AddParaLineSpacingToTableCells is
   false. A file whose settings.xml has the configuration settings without
   this item takes false, as the files of older versions do. A file without
   these settings takes true, the default of a new document. With `last`,
   the layout takes the extra space away from the last line of each part
   of a cell, so the part ends below its text. `odf` writes `last` for the
   paragraphs of a cell only when the setting is false.
3. At the top of the part of a cell after a page break, LibreOffice keeps
   the spacing above the first paragraph, and leaves out the spacing below
   the paragraph before it. It keeps it too when the paragraph goes on
   from the page before. A length after `discard` of `-ivy-page-top-space`
   gives that spacing, and the layout keeps up to that length of the top
   margin there. Word leaves out the whole space, so `docx` and `msdoc`
   give no length. `odf` gives fo:margin-top of each paragraph of a cell.

With these rules, the pages of `landscape-a4-table.odt` hold the same lines
as in LibreOffice. The row "K bodu 96" on page 19 is as high as in
LibreOffice.

LibreOffice also keeps the spacing above a paragraph at the top of a part
of a cell when contextual spacing leaves it out. So the length of
`-ivy-page-top-space` can be larger than the top margin of the paragraph,
and the layout takes the length. `odf` gives fo:margin-top of the
paragraph, before contextual spacing. With this rule, the text of page 23
of `landscape-a4-table.odt` starts 12 points lower, as in LibreOffice. The
pages after it keep their lines, because `odf` reads style:footnote-sep
for the line above the footnotes since the same day. That line and its
spacing take 6.2 points, where the line of ivy took 9.5 points.

On the same day, `-ivy-line-numbers` took three keywords after its four
values, for the line numbering of `odf`. The side is `left`, `right`,
`inner` or `outer`, and `inner` and `outer` take the side of the page.
With `no-empty`, an empty line does not count. With `reset`, the first line
of the box takes the first number. `odf` gives them from
`text:number-position`, `text:count-empty-lines` and `text:line-number`.
LibreOffice writes `inside` and `outside`, and `odf` reads both pairs.
Later that day, `-ivy-line-numbers` took a fourth keyword,
`object-bottom`. With it, the number of a line that holds only an object
has its baseline at the bottom of the object. LibreOffice draws it there,
because such a line has no descent. `odf` puts the object at the top of
its line for that reason, which moves the baseline of the line to the
top. `odf` always gives the keyword.
