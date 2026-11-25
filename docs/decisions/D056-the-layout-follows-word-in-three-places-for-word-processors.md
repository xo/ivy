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
