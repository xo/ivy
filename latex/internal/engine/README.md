# engine

This folder is a copy of
[github.com/go-tex/engine](https://github.com/go-tex/engine) at the commit
`8234996d66f827438f248e8b0059031f52cb9964`, under the BSD-3-Clause license
in `LICENSE` (D7, D62). The rules for a copy are in
[`docs/UPSTREAM.md`](../../../docs/UPSTREAM.md).

## The files

The copy holds each file of source of the root of upstream, with its name,
except the files below. It also holds `hyph_en_us_patterns.txt`, the
patterns of hyphenation of American English of the hyph-utf8 package. Its
notice allows a copy and a change of the file, with the notice.

The copy leaves out these files of upstream:

| File | Why |
| --- | --- |
| `pdfdriver.go`, `svgpdf.go`, `svgimage.go`, `mathpdf.go`, `hyperlink_internal.go` | They write PDF with go-pdfkit. `ivy_driver.go` draws the pages on the display list, and pdf/pdfenc writes PDF (D58) |
| `image.go` | It decodes the figures with go-gfx. `ivy_image.go` holds it without the decoders |
| `lmoptical.go`, `lmfonts.go` | They embed Latin Modern. The fonts are in the module fonts (D24), and `ivy_lmoptical.go` and `ivy_lmfonts.go` take them from a program |
| `texmf.go` and the folder `texmf` | They embed classes and packages of LaTeX under the LaTeX Project Public License. A program gives them (D62) |
| the folder `fonts` | Latin Modern, which is in `fonts/lm` of the module fonts |
| the tests, the folders `cmd`, `examples`, `internal`, `scripts` and `testdata` | The tests of upstream compare files of PDF of go-pdfkit, and the commands are programs |

The files of ivy start with `ivy_`. An update of the copy takes the files of
upstream and keeps them:

| File | What it holds |
| --- | --- |
| `ivy_engine.go` | `Compile`, the function compile of `api.go` with the context, the fonts, the resolver and the math renderer of a run of ivy, and the state of ivy that the step counter reads |
| `ivy_driver.go` | the driver: the walk of the tree of boxes onto the display list of ivy (D10) |
| `ivy_svgdraw.go` | the specials of SVG and the figures in SVG, through the SVG of okage |
| `ivy_image.go` | `image.go` without go-gfx and without the reads of the disk |
| `ivy_svg.go` | the parts of `mathpdf.go` and `svgimage.go` that do not use go-pdfkit. It reads the root of an SVG image with the XML parser of okage (D7), and `parseTransform` looks for the parenthesis that closes after the one that opens, where upstream slices out of range on a transform such as `a)b(c` |
| `ivy_hyperlink.go` | `hyperlink_internal.go` without the method of the PDF driver |
| `ivy_lmoptical.go`, `ivy_lmfonts.go` | the tables of the optical sizes of Latin Modern, and the files of Latin Modern and of TeX that a program gives |
| `ivy_pdf.go` | `RenderPDF`, which returns an error |

## The changes

Each change of a file of upstream is marked with a comment that starts with
`ivy:`. Each file of upstream opens with a line that names the file and the
commit.

1. `font.go` imports `latex/internal/opentype`, the API of go-opentype on
   the fonts of okage, in place of go-opentype.
2. `math.go` imports `latex/internal/mathsvg`, the API of go-tex/math on
   texmath of okage (okage D78), in place of go-tex/math.
3. `libs.go` imports the copies of go-typeset/linebreak and
   go-typeset/hyphenation in `latex/internal`.
4. `engine.go`, `io.go`, `packages.go`, `readpolicy.go`, `readaudit.go`,
   `floatplace.go` and `fltrace.go` import `latex/internal/nofs` in place
   of the package os. A document reads no file of the disk and no variable
   of the environment (D5). It reads the files that a program gives
   through the resolver of the options.
5. `engine.go` has the field `ivy` in `Engine`, and `stepOverrun` stops
   the run when the context of the run is done.

`accents.go` imports golang.org/x/text/unicode/norm, which the module of
ivy already requires (D7).

## What a run of ivy does not do

1. The engine changes maps of the package when it builds a run, so runs do
   not run at the same time. `Compile` waits for the run before it.
2. TikZ and pgf load the real sources only with the variable GOTEX_PGF of
   upstream, and nofs gives no variable. A picture of TikZ is a framed box,
   as upstream does by default.
3. `\includepdf` gives empty pages, because upstream rasterizes the pages
   with a renderer that a program wires.
