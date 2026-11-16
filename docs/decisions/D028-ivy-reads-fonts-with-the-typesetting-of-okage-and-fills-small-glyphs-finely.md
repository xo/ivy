# D28. ivy reads fonts with the typesetting of okage, and fills small glyphs finely

Status: Decided.

On 2026-11-17 the visual review showed that the text of ivy looks lighter,
thinner and wider than the text of MuPDF, Poppler and LibreOffice. The
maintainers asked to copy go-text/typesetting into okage, to fix the text
there, and to measure the causes first. The measurements found:

1. The ebooks and the formats of text gave the layout no fonts of
   `ivy.Config`, so they drew with the Go fonts, which are wider than the
   fonts that calibre uses. The generic families stayed the Go fonts when
   the fonts module was loaded.
2. The references draw glyphs without hinting and without a correction of
   gamma. The advances of ivy match those of LibreOffice within a pixel.
   Hinting is not the cause, so D19 stays.
3. The rasterizer of okage samples a grid of 4 by 4 in each pixel, so the
   coverage of a small glyph moves in steps of a quarter. The references
   cover more smoothly.

The maintainers decided:

1. okage holds a copy of go-text/typesetting as a public package,
   `github.com/xo/okage/typesetting` (okage D52). ivy imports that copy, and
   `go.mod` of ivy does not require go-text. The approval of go-text in D7
   now covers the copy in okage.
2. Each format gives the layout the fonts of `ivy.Config`, through
   `ebook.Fonts`. When the font files hold Liberation Serif, Sans and Mono,
   they are the generic families serif, sans-serif and monospace, as in
   LibreOffice and calibre.
3. `internal/display` fills a glyph of at most 48 pixels to the em at twice
   its size into a mask, and averages each block of 2 by 2 pixels, so its
   coverage has the grid of 8 by 8 of the references. The layout and the
   glyphs of PDF fill their glyphs this way. A larger glyph fills as a path.
   The cost is about half again of the time of a page of text.
