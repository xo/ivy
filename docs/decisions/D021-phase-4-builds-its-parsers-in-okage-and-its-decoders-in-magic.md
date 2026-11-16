# D21. Phase 4 builds its parsers in okage and its decoders in magic

Status: Decided.

On 2026-11-16, okage and magic lacked parts that phase 4 needs. The package
`html` of okage reads only the HTML labels of Graphviz. The CSS parser of
okage, `internal/simplecss`, reads no `@page`, no `@media` and few selectors
of CSS 3. `magic/compress` has no decoder for PalmDoc, HUFF/CDIC or LZX.
The maintainers chose these answers:

1. This session of ivy builds the parsers in okage, as it built okage D47 and
   okage D48. The HTML parser is a port of
   [tinyhtml5](https://github.com/CourtBouillon/tinyhtml5) (MIT). The CSS
   parser and the selectors are ports of
   [tinycss2](https://github.com/Kozea/tinycss2) and
   [cssselect2](https://github.com/Kozea/cssselect2) (BSD-3-Clause). The
   readers of WOFF, WOFF2 and EOT become a public package. Each change is a
   decision of okage, and it waits in okage for the review of a maintainer
   (D8).
2. This session of ivy adds the decoders of PalmDoc, HUFF/CDIC and LZX to
   `magic/compress`. Each change is a decision of magic, and it waits in
   magic for the review of a maintainer (D8). So MOBI, AZW3, AZW4 and CHM
   are part of phase 4.
3. `internal/layout` ports all of the layout of WeasyPrint 70.0: blocks,
   inlines, floats, tables, flex, grid, columns, absolute positioning,
   footnotes and pages (D15). A cut would make the port differ from its
   source, and grid, flex and columns are about 15 percent of the layout.
4. The maintainers approve a copy of
   [goldmark](https://github.com/yuin/goldmark) (MIT) in `internal/`, with
   its license, for Markdown (D7). goldmark is pure Go and follows
   CommonMark.
