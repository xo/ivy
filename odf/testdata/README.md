# Test files

LibreOffice 26.8 wrote these files with `soffice --headless --convert-to`,
from a short text, a CSV file of two rows, or a flat ODP file of one slide.
LibreOffice writes a thumbnail into an OpenDocument file, and none into an
Office Open XML file.

LibreOffice 26.8 wrote the encrypted files with the password `secret`, from
a short text, through its API:

- `encrypted_blowfish.odt`: the version ODF 1.1, with Blowfish and SHA-1.
- `encrypted_aes.odt`: the version ODF 1.2, with AES-256 and SHA-256.
- `encrypted_argon2id.odt`: the default version, ODF 1.4, which encrypts the
  whole package with AES-GCM and Argon2id.

LibreOffice writes no thumbnail into an encrypted file. The tests encrypt a
thumbnail with the fixture of `internal/fixture`.

LibreOffice 26.8 wrote the files `float-f-late.odt` and `float-f-bottom.odt`
from the files of the same names of `docx/testdata`, with `soffice --headless
--convert-to odt`. They hold a frame with `style:vertical-rel` page-content
and wrap, 1 inch below the top of the margin or at its bottom (D60).

LibreOffice 26.8 wrote the files `float-mid.odt`, `float-right.odt` and
`float-page-*.odt` from flat files that a script wrote, with `soffice
--headless --convert-to odt`. Then `zip -d` removed the thumbnail and the
empty folder `Configurations2` from each file. Each page is Letter paper
with margins of 1 inch, and has paragraphs of about 3 lines. Each file
holds one text frame of 2 inches width, with a distance of 0.1 inch at its
left and at its right:

- `float-mid.odt`: a frame in the second paragraph, with `style:wrap`
  parallel, at `svg:x` 2.25 inches from the paragraph area (D63).
- `float-right.odt`: the same frame at `svg:x` 4 inches, 0.5 inch from the
  right of the column.
- `float-page-left.odt`, `float-page-right.odt`, `float-page-parallel.odt`
  and `float-page-none.odt`: a frame of 1.5 inches height anchored to the
  page, at 3.25 inches from the left of the page and 3 inches from its top,
  with each `style:wrap` (D60).

LibreOffice 26.8 wrote the files `float-margin.odt`, `float-page2.odt`,
`float-cell.odt`, `float-section.odt` and `float-top.odt` in the same way.
Each holds text frames of the color `#c0e0ff`, from `draw:fill` solid:

- `float-margin.odt`: a frame of 2 inches with `style:wrap` parallel at the
  left edge of a paragraph with margins of 0.5 inch.
- `float-page2.odt`: a frame with `style:wrap` parallel anchored to the
  page 2 by `text:anchor-page-number`, before the first paragraph.
- `float-cell.odt`: a frame of 1 inch with `style:wrap` parallel in each
  cell of a table of two columns of 3.25 inches, at `svg:x` 1 inch and 2.1
  inches.
- `float-section.odt`: the same frames in a section of two columns, at
  `svg:x` 1 inch and 0.5 inch.
- `float-top.odt`: a frame with `style:wrap` none at the top of its
  paragraph. LibreOffice writes the frame before the text of the
  paragraph. A script then moved it after the twentieth word, as other
  programs write it. LibreOffice 26.8 puts the frame at the same place in
  both files.

LibreOffice 26.8 wrote the files `float-page2-run.odt`, `float-below.odt`,
`float-empty.odt`, `float-mirrored.odt` and `float-center.odt` in the same
way. Each holds text frames of the color `#c0e0ff`, with a distance of 0.1
inch at their left and at their right:

- `float-page2-run.odt`: a frame with `style:wrap` run-through anchored
  to the page 2 by `text:anchor-page-number`.
- `float-below.odt`: a frame with `style:wrap` none 0.5 inch below the top
  of its paragraph. LibreOffice puts 2 lines of the paragraph above it.
- `float-empty.odt`: a frame with `style:wrap` none at the top of an empty
  paragraph. LibreOffice puts the line of the paragraph below it.
- `float-mirrored.odt`: frames anchored to the pages 1 and 2 of a page
  layout with `style:page-usage="mirrored"`, a margin of 2 inches at the
  left and of 0.5 inch at the right. The frames are at 1 inch from the left
  of the page area, at the left of the page area, and at 3 inches from the
  left of the page.
- `float-center.odt`: frames at the center of their paragraphs in the
  header, in two table cells of 3.25 inches, and in a section of two
  columns.
- `float-indent.odt`: a frame of 2 inches with `style:wrap` right at the
  start of a paragraph with `fo:text-indent` 0.25 inch, and the same frame
  in a paragraph that also has a margin of 0.5 inch at its left.
  LibreOffice puts each frame at 0.1 inch from the left of the page area,
  and does not indent its text.

LibreOffice 26.8 wrote the files `table-relative.odt` and
`frame-gradient.odt` from flat files in the same way:

- `table-relative.odt`: six tables of two columns with
  `style:rel-column-width` of 1 to 3, as in the flat file. LibreOffice
  writes a table without `style:width` with `table:align` margins and only
  the relative widths, and it gives the table the width between its
  margins. The second table has margins of 0.5 inch and 1 inch. The
  fourth table has a width of 4 inches and `table:align` left, and
  LibreOffice writes its columns with `style:column-width`.
- `frame-gradient.odt`: a text frame of 2 by 1 inches at 2 inches from the
  top and the left of the page, with a `draw:gradient` linear from red at
  the top to blue at the bottom.

LibreOffice 26.8 wrote the files `line-fixed.odt`, `line-number-object.odt`,
`page-mirrored.odt` and `footnote-sep.odt` in the same way. Each page is
Letter paper, and the text is Liberation Serif of 12 points. The pictures
are red PNG images of 1 inch:

- `line-fixed.odt`: a picture as a character in the first line of a
  paragraph with `fo:line-height` 18pt. LibreOffice keeps the line at
  18 points, and the picture goes up over the paragraph before it.
- `line-number-object.odt`: the numbers of the lines, and a paragraph
  that holds only a picture as a character. LibreOffice draws the number
  of that line at the bottom of the picture.
- `page-mirrored.odt`: 4 pages of a page layout with
  `style:page-usage="mirrored"`, a margin of 2 inches at the left and of
  0.5 inch at the right. The left pages swap the margins.
- `footnote-sep.odt`: a footnote below a `style:footnote-sep` with a red
  line of 0.1 cm, half as wide as the page area and at its center, 0.5 cm
  below the text and 0.3 cm above the footnote.
