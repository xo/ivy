# D7. ivy reuses okage and magic, then copies permissive code

Status: Decided.

ivy is mostly self-contained, and it holds no second copy of code that
another `xo` repository holds.

## Reuse first

Before ivy ports or copies code, it looks in okage and in magic. If one of
them has the code, ivy uses it. If the code is close but lacks something, the
work goes to that repository (D8). On 2026-11-16 they hold these parts that
ivy needs:

| Part | Where it is | ivy needs it for |
| --- | --- | --- |
| XML parser, a port of roxmltree | `okage/roxmltree`, exported in phase 2 | XPS, Office Open XML, OpenDocument, FictionBook, EPUB, the XML plists of Apple |
| CSS parser, a port of simplecss | `okage/internal/simplecss` | the layout (D15) |
| HTML parser | `okage/html` | the layout (D15), after okage extends it past the labels of Graphviz |
| Geometry, a port of kurbo | `okage/internal/kurbo` | the port of hayro (D14) |
| WOFF, WOFF2 and EOT | `okage/internal/webfont` | the fonts of EPUB and of web pages |
| brotli | `okage/internal/brotli` | WOFF2 |
| SVG images | `okage/svg` | EPUB and Office files that hold SVG |
| Text, fonts and the fonts of the system | `okage/svg` (okage D36, okage D37 and okage D46) | the layout and the font layer (D10) |
| xz and zstd | `magic/compress` | the Linux packages and other containers |

ivy reads XML with the XML parser of okage, and it does not import
`encoding/xml`. The parser builds with TinyGo. okage exports it as the
package `roxmltree`, a change that phase 2 made in okage at the request of the
maintainers.

Go does not let another module import an internal package. For each other
part under `internal/`, the backlog asks okage to export it as a public
package. Until okage exports a part, the format that needs it waits.

## Then copy or port

1. A Go package from outside the `xo` repositories is allowed only if it is
   pure Go and under a permissive license, such as MIT, BSD or Apache-2.0.
   The maintainers approve each one.
2. The first choice is to copy the code into a package under `internal/`, or
   to port it from another language into one. The copy keeps the license of
   its source, in a `LICENSE` file in its folder, and each file keeps its
   notice, as okage D13 does.
3. Code under the GPL, the LGPL or the AGPL is never copied, ported or
   imported. calibre, LibreOffice, Ghostscript, poppler, MuPDF, libgxps and
   DjVuLibre are such code. Only the test module can run them, as oracles
   (D13).
4. Code under the MPL-2.0 can be ported only if the maintainers approve it,
   because each file keeps that license.

## Approved outside packages

The maintainers approved these packages from outside the `xo` repositories.
okage requires each one already, and okage D6 approved it for okage:

| Package | ivy uses it for | Approved |
| --- | --- | --- |
| `golang.org/x/image` | the TIFF decoder of `ps`, the WebP decoder of `comic`, the BMP, TIFF and WebP decoders of `internal/layout`, and the Go fonts, which the layout uses when a program gives no fonts | 2026-11-16 |
| `github.com/go-text/typesetting` | the fonts of `internal/font`: TrueType, OpenType and CFF, the map from characters to glyphs, the outlines and the advances. The shaping and the breaks of the lines of `internal/layout` | 2026-11-16 |

The maintainers also approved a copy of goldmark in `internal/`, for Markdown (D21).

A port follows its source file by file, as okage does, and keeps a ledger of
the commits of its source after a base commit. A copy also records its
source and its commit. `docs/UPSTREAM.md` holds the rules before the first
port or copy starts.
