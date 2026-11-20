# D62. ivy reads and writes LaTeX with a copy of go-tex/engine

Status: Decided.

On 2026-11-20 the maintainers asked for a proper reader and writer of
LaTeX, and question 25 of `PLAN.md` asked how. `docs/FORMATS.md` said "No"
for LaTeX before this decision. On the same day the maintainers chose
choice 3 of the question: a TeX engine of ivy, copied from
`github.com/go-tex/engine`, that draws with okage, and a layout of math
that okage holds, so that zuzu can draw the math of its Mermaid labels with
it too.

## The facts

1. `go-tex/engine` is a faithful TeX engine under BSD-3-Clause. On
   2026-11-20, at the commit `8234996`, it has 135 files of source with
   about 50,000 lines, and 61,000 lines of tests. It runs the real classes
   `article`, `report` and `book`, and real TeX gates it.
2. Its outside modules are in about 10 files. `go-pdfkit` is in
   `pdfdriver.go`, `svgpdf.go`, `svgimage.go`, `mathpdf.go` and
   `hyperlink_internal.go`. `go-gfx` is in `image.go`. `go-opentype` is in
   `font.go` and `lmoptical.go`. `go-tex/math` is in `math.go`, and
   `go-typeset/linebreak` and `go-typeset/hyphenation` are in `libs.go`.
3. `Pages()` gives a tree of boxes. The PDF driver is a walk of 570 lines
   over that tree. The tree and its nodes are not exported, so a driver of
   ivy must be in the package. That needs a copy, not an import.
4. The fonts of the engine are behind the interface `fontFace`. Its math
   comes from `go-tex/math` as SVG with metrics.
5. The engine has limits on its steps of expansion, `StepLimit` and
   `NoProgressLimit`, but no context.
6. `go-tex/math` has 12 files of source, about 4,600 lines, under
   BSD-3-Clause. It reads the OpenType MATH table through `go-opentype`.
   okage has no reader of that table.
7. The engine merges about 10 changes each day, so a copy needs a regular
   update.

## The decision

1. `latex/` is the decoder, and `latex/latexenc` is the encoder (D6).
2. `latex/internal/engine` is a copy of `go-tex/engine`, under its license,
   recorded in `docs/upstream/upstreams.tsv` as `docs/UPSTREAM.md` says.
   The copy leaves out the files that use `go-pdfkit` and `go-gfx`. The
   changes of ivy go in files of their own, so that an update of the copy
   takes the files of upstream and keeps the files of ivy.
3. A driver of ivy walks the tree of boxes onto the display list of D10:
   the glyphs as runs with their text (D58), the rules, the colors, the
   transforms and the links. The specials of SVG, such as the output of
   TikZ, draw through the SVG of okage. The figures go through the decoders
   of ivy, so a figure in PDF stays a vector. PDF output goes through
   pdfenc.
4. The interface `fontFace` of the engine gets an implementation on the
   fonts of okage, in place of `go-opentype`. okage adds the small helpers
   for a glyph that it does not give yet: the metrics, the kerning of a
   pair and the ligature of a pair.
5. The math of the engine moves from `go-tex/math` to the layout of math
   of okage, when okage releases it. Until then the copy keeps
   `go-tex/math`, which needs the approval of the maintainers for its
   outside modules (D7), or the math waits.
6. okage holds the layout of math: a reader of the MATH table, a tree of
   math nodes, a parser of the syntax of TeX math ported from
   `go-tex/math`, and the layout of TeX Appendix G. Its result draws with
   okage, gives its outlines as paths, or gives its glyphs and rules. okage
   decides its API in okage D72. zuzu draws the math of its Mermaid labels
   with it. ivy draws the equations of Office Math, MathML and OpenDocument
   with it, from a tree that ivy builds.
7. A program gives the classes and the packages of LaTeX through
   `ivy.Config`. ivy fetches nothing from the network, and the module of
   ivy does not embed the files of LaTeX that are under the LaTeX Project
   Public License. The fonts, such as Latin Modern and STIX Two Math, go in
   the module `fonts` under their licenses (D24).
8. The copy gets a context and the limits of `ivy.Config` (D5, D11), and a
   fuzz test (D41). Real TeX is an oracle of the test module (D13).
9. `latex/latexenc` writes the HTML of a document of the layout as LaTeX.

## The cost

For one agent with a maintainer, the decoder is about 5 to 8 weeks: the
copy, the driver, the fonts, the package and its tests. The layout of math
in okage is about 3 to 5 weeks, and it can run at the same time. The
encoder is not estimated. The copy needs an update from upstream again and
again, and a test of the engine that compares a PDF file of `go-pdfkit`
changes or goes.
