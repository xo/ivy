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
6. okage holds the layout of math: a reader of the MATH table, a tree of math
   nodes, a parser of the syntax of TeX math ported from `go-tex/math`, and
   the layout of TeX Appendix G. Its result draws with okage, gives its
   outlines as paths, or gives its glyphs and rules. okage decides its API in
   okage D78, the package `texmath`. zuzu draws the math of its Mermaid labels
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

## The build of phase 8a

On 2026-11-21 phases 8a and 8b, and the math of the engine of phase 8d,
were built. The build differs from the decision in these points:

1. The copy is of the commit `8234996d66f827438f248e8b0059031f52cb9964`.
   It leaves out `image.go`, `lmoptical.go`, `lmfonts.go` and `texmf.go`
   too, because they use go-gfx or embed fonts and files of LaTeX. The
   files of ivy, `ivy_*.go`, hold their parts that ivy keeps.
2. The fonts of okage have no helper for the kerning or the ligature of a
   pair, so `latex/internal/opentype` shapes the pair with the shaper of
   okage, with only that feature on. Item 4 of "Work for okage" in
   `BACKLOG.md` still asks for the helpers.
3. okage released `texmath` (okage D78) before this build, so the copy
   does not keep `go-tex/math`. `latex/internal/mathsvg` gives the API of
   go-tex/math on texmath, and the driver draws the glyphs of a formula as
   runs of glyphs.
4. The copy keeps `golang.org/x/text/unicode/norm` in `accents.go`. The
   module of ivy already requires golang.org/x/text (D7).
5. go-typeset/linebreak and go-typeset/hyphenation are under BSD-3-Clause.
   They are copies in `latex/internal`. The patterns of hyphenation of
   American English that upstream holds are under a license that allows a
   copy, so the copy keeps them.
6. The engine reads no file of the disk and no variable of the environment.
   The files of the copy that import the package os import
   `latex/internal/nofs`. The package `latex` gives the files of a
   document through the resolver of the engine: the folder of the document
   and the tree of TeX files of D65.
7. The engine changes maps of its package when it builds a run, so two
   runs race. ivy runs one engine at a time.
8. Latin Modern is in the folder `lm` of the module `fonts`, and the
   function `TeX` of the module gives it, apart from `FS` (D65).
9. The links of a page are kept on the page, because the display list has
   no link yet (phase 7c). Real TeX is an oracle of the test module with
   pdfLaTeX and Ghostscript.

