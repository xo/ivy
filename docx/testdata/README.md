# Test files

LibreOffice 26.8 wrote these files with `soffice --headless --convert-to`,
from a short text, a CSV file of two rows, or a flat ODP file of one slide.
LibreOffice writes a thumbnail into an OpenDocument file, and none into an
Office Open XML file.

## The page floats of D60

A script wrote the files `float-*.docx` with the elements of Office Open
XML that D60 names. Each file has Letter paper with margins of 1 inch, and
paragraphs of about 3 lines of Liberation Serif of 12 points. Paragraph k
starts with the word `Pk`, with k in two digits. A float is 3 inches wide,
at the left of the margin, and the text wraps around it. A cell of a table
holds the word `Tn` for its row n, and a frame holds the word `Frame`.

| File | The float | Its anchor |
| --- | --- | --- |
| `float-t-late.docx` | a table of 4 rows of 0.5 inch, 1 inch below the top of the margin (`w:tblpPr`) | before P12 |
| `float-t-page2.docx` | the same table | before P26, on page 2 |
| `float-f-late.docx` | a frame of 2 inches, 1 inch below the top of the margin (`w:framePr`) | after P11 |
| `float-s-late.docx` | a picture of 2 inches, 1 inch below the top of the margin, square wrap (`wp:anchor`) | in P12 |
| `float-f-bottom.docx` | the frame, at the bottom of the margin (`w:yAlign`) | after P01 |
| `float-t-bottom.docx` | the table, at the bottom of the margin (`w:tblpYSpec`) | before P02 |
| `float-t-two.docx` | two tables at the same place | before P02 and before P05 |
| `float-t-loop3.docx` | a table of 12 rows, at the top of the margin | before P10 |
| `float-t-loop2.docx` | the same table | before P12 |
| `float-t-nofit.docx` | a table of 6 rows, 9.5 inches below the top of the page | before P06 |
