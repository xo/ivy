# Test files

A script wrote the files `float-*.rtf` by hand, in the control words that
Word writes. Each page is Letter paper with margins of 1 inch, and has
paragraphs of about 3 lines in Liberation Serif. The tests compare the
places of the floats with the PDF files that LibreOffice 26.8 makes with
`soffice --headless --convert-to pdf`.

Each file `float-s-*.rtf` holds a picture of 2 by 1 inches in the second
paragraph, with square wrap, at the top of the paragraph (D63):

- `float-s-mid.rtf`: 2.25 inches from the left of the column, with the text
  on both sides.
- `float-s-right.rtf`: at the right of the column.
- `float-s-largest.rtf`: 1.5 inches from the left of the column, with the
  text on the largest side, `\shpwrk3`.

Each file `float-x-*.rtf` holds a text box of 2 by 1 inches, without a
line:

- `float-x-page.rtf`: 4 inches from the left of the page and 2 inches from
  its top, with square wrap, in the third paragraph (D60).
- `float-x-bottom.rtf`: at the right of the margin and at the bottom of the
  page, with square wrap and no distance from the text, in the third
  paragraph (D60).
- `float-x-topbottom.rtf`: 1 inch from the left of the column, at the top of
  the second paragraph, with the text above and below it.

Each file `float-f-*.rtf` holds a frame, a paragraph with the words of a
frame of RTF 1.9.1 (D60):

- `float-f-late.rtf`: a frame of 3 by 2 inches, 1 inch below the top of the
  margin, after the paragraph `P11` on page 1, with `\wrapdefault`.
- `float-f-bottom.rtf`: the same frame at the bottom of the margin, after
  the paragraph `P01`.
- `float-f-page.rtf`: a frame of 2 by 1 inches at the right of the page, 2
  inches from its top.
- `float-f-para.rtf`: a frame of 2 by 1 inches at the right of the margin,
  0.25 inch below the top of the next paragraph, `\pvpara`.
- `float-f-nowrap.rtf`: a frame of 2 by 1 inches at the center of the
  margin, 2 inches below its top, with the text above and below it,
  `\nowrap`.

The text boxes give the distances of the text with `dyWrapDistTop`,
`dyWrapDistBottom`, `dxWrapDistLeft` and `dxWrapDistRight`, because
LibreOffice takes other defaults than MS-ODRAW. The pictures do not use
`posrelv` 0, `posh`, `posv` or `\shpwr1`. LibreOffice 26.8 ignores them for
a picture, and places the picture from its paragraph, at `\shpleft` and
`\shptop`. The unit tests of `float_test.go` check these words and
properties.
