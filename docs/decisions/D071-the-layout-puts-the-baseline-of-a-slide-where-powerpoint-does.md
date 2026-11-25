# D71. The layout puts the baseline of a slide where PowerPoint does

Status: Decided.

The text of a slide of PowerPoint, of a shape of Excel and of a SmartArt
diagram goes through `internal/drawingml`, which writes HTML for the
layout. The review of 2026-11-25 compared the text of ivy with the PDF
files that Microsoft Office exports. The text of each slide was higher in
ivy than in PowerPoint, by the same share of the font size on each size of
text. This decision gives the layout a property for the rule of
PowerPoint, as D56 gives one for each rule of Word.

## The facts

These were measured on 2026-11-25 with `mutool draw -F stext` on the PDF
files of Office and of ivy, for the 24 files of PowerPoint of the review
that have a PDF file of Office.

1. A line of 100 percent spacing is 1.2 times the size of its text in
   PowerPoint and in ivy. The distance from one baseline to the next is
   the same in both.
2. The first baseline of a box is lower in PowerPoint. The difference is
   0.086 to 0.090 em for Calibri, 0.111 em for Candara, 0.028 em for Times
   New Roman and 0.016 to 0.026 em for Arial, at each size from 11 to 44
   points.
3. CSS puts half of the leading of a line above the text and half below
   it, with the ascent and the descent of the table `hhea` of the font.
   PowerPoint puts the baseline at the height of the line times
   `usWinAscent / (usWinAscent + usWinDescent)` of the table `OS/2`. This
   rule gives the differences of fact 2 within 0.004 em: 0.086 em for
   Calibri, 0.111 em for Candara, 0.028 em for Times New Roman and 0.026 em
   for Arial.
4. The table of `table-test2.pptx` was 1 point higher per row in ivy. The
   collapsed borders of CSS put half of each border in the cell, and
   PowerPoint draws a border on the line of the grid, over the inset of
   the cell.

## The choices

1. A property of `css_ivy.go`, `-ivy-line-baseline: win`, puts the
   baseline of each line box at the height of the line times the share of
   the Windows ascent of its first font. The layout reads the two values
   from the table `OS/2` of the face. `internal/drawingml` turns it on for
   the text bodies of DrawingML, of `msppt` and of `msxls`. The text bodies
   of `vsdx` and of `odf` keep the rule of CSS, because Visio and
   LibreOffice are not PowerPoint. This is the recommendation.
2. `internal/drawingml` moves each paragraph with `position: relative`,
   from a table of the ratios of the fonts of Office. This needs no
   property, but a font that the program does not have takes the ratio of
   the wrong font, and a paragraph of two sizes moves by one amount.
3. No change.

## The cost

Choice 1 is about 90 lines of Go in `internal/layout`, with a test, and
one declaration in each text body of `internal/drawingml`. The review of
2026-11-25 measured it together with three fixes that follow PowerPoint
more closely and need no decision: the table takes half of each border
from the padding of a cell, a text box of a slide takes the default text
style of the presentation, and the autofit rounds a size to a whole
point. With them, 12 files of PowerPoint come closer to Office and 1 file
moves 0.05 farther from LibreOffice. The mean of the worst page goes from
3.84 to 3.24 for PowerPoint, and from 5.49 to 4.64 for PowerPoint 97. The
files of OpenDocument and of Visio do not change.

## The decision

The maintainers have not decided. The build of 2026-11-25 follows choice
1, so that the lead of the round can measure it. Question 32 of `PLAN.md`
asks the maintainers to choose.

## The decision

On 2026-11-25 the maintainers answered question 30 and accepted the
proposal.
