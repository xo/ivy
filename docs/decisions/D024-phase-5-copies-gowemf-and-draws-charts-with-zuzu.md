# D24. Phase 5 copies gowemf and draws charts with zuzu

Status: Decided. Amended by D27.

Phase 5 is the Office formats: EMF and WMF, the slides, the documents, the
spreadsheets, RTF, the previews and the text of the binary formats, and the
encryption of MS-OFFCRYPTO and OpenDocument (`docs/PLAN.md`). On 2026-11-17
the plan left seven choices open, and the maintainers chose these answers:

1. ivy copies [gowemf](https://github.com/mgilbir/gowemf) (MIT) into
   `internal/gowemf`, with its license (D7). gowemf decodes WMF, EMF and
   EMF+, and plays them back through a small interface. ivy drops its one
   dependency, golittlecms, and gives it a color transform on
   `internal/moxcms`. A small package draws its commands onto the display
   list (D10).
2. magic makes its reader of Compound Files a public package, for the
   binary Office formats, their property sets and their encryption. This
   session of ivy builds it in magic, as a decision of magic that waits for
   the review of a maintainer (D8), as D21 did for the decoders of phase 4.
3. The fonts with the metrics of the Office fonts go in a module of their
   own, `github.com/xo/ivy/fonts` (D6): Liberation Sans, Serif and Mono for
   Arial, Times New Roman and Courier New, Carlito for Calibri, and Caladea
   for Cambria. They are under the SIL Open Font License, and the module
   keeps the license of each font. A program that imports the module gives
   its fonts to a format with the field `Fonts` of `ivy.Config`. The main
   module stays small, and a program that does not import the module draws
   with the Go fonts, or with the fonts of the system (D23).
4. The preset shapes of DrawingML come from `presetShapeDefinitions.xml` of
   ECMA-376. ivy keeps the file with the copyright notice of Ecma, and
   generates Go tables from it.
5. The maintainers approve `golang.org/x/crypto` (BSD-3-Clause, pure Go) for
   Blowfish, which encrypted OpenDocument files before ODF 1.2 use (D7).
   The other encryption uses the standard library.
6. ivy draws the charts of DrawingML with [zuzu](https://github.com/xo/zuzu).
   ivy converts a chart into a spec of Vega-Lite, and zuzu lays the chart
   out. ivy draws the scene of zuzu onto the display list.
7. A spreadsheet puts every sheet on pages, in order, as printing does. The
   plan said the first sheet only.

The field `Fonts` of `ivy.Config` is a change to the public API (D11). It is
an `fs.FS` of font files. A format that lays out text uses them after the
fonts that the document embeds, and before the fonts of the system.
