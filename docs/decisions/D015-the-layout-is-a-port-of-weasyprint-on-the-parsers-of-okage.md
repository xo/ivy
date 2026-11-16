# D15. The layout is a port of WeasyPrint on the parsers of okage

Status: Decided.

The layout of D10, in `internal/layout`, is a port of
[WeasyPrint](https://github.com/Kozea/WeasyPrint) (BSD-3-Clause), made by
ivy. WeasyPrint lays out HTML and CSS onto pages, with `@page`.

ivy does not use two other engines:

1. webrender, the Go port of WeasyPrint, requires
   `benoitkugler/textprocessing`, a port of Pango, FriBidi and fontconfig under
   the LGPL-2.1, and D7 forbids it.
2. Blitz, which iv uses today through xo/blitz, is Rust and links through
   cgo, and D1 forbids it.

ivy ports only the parts of WeasyPrint that okage does not already have
(D7): the cascade and the computed values of the CSS properties, the boxes,
the layout of blocks, of lines, of tables and of flex boxes, and the pages.
Everything else comes from okage:

| Work | In WeasyPrint | In ivy |
| --- | --- | --- |
| Parse XHTML | tinyhtml5 or lxml | the XML parser of okage |
| Parse HTML | tinyhtml5 (MIT) | the package `html` of okage. okage D38 puts a parser of other HTML there. tinyhtml5 is its reference |
| Parse CSS and match selectors | tinycss2 and cssselect2 (BSD-3-Clause) | the CSS parser of okage, extended in okage. tinycss2 and cssselect2 are its reference |
| Shape and break text | Pango, through cffi (LGPL) | the text API of okage (okage D37), on go-text |
| Load fonts, also WOFF and WOFF2 | fonttools (MIT) | the fonts of okage (okage D45) and the font layer (D10) |
| Draw SVG images | its own SVG renderer | the package `svg` of okage |
| Draw the pages | pydyf, a writer of PDF | the device of the display list (D10) |
| Hyphenate | Pyphen (GPL, LGPL or MPL) | none. A review does not need it (D5) |

If a parser of okage lacks something that the layout needs, such as `@page`,
`@media` or a selector, the work goes to okage (D8). ivy does not hold a
second parser. The port follows WeasyPrint file by file, with a ledger, as
D14 does for hayro.
