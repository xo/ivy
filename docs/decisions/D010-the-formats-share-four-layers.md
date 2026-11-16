# D10. The formats share four layers

Status: Decided.

Most formats of ivy reduce to one of two forms. A page description, such as
PDF, XPS, PostScript, EMF or a slide of PowerPoint, places each mark at a
fixed point. A flow of text, such as EPUB, FictionBook, Word or OpenDocument
Text, places its text by a layout. Four shared layers serve these two forms.
The last three are internal packages, so that their API can change while the
formats grow. If a program needs one, the maintainers decide to export it.

1. The document API, in the root package `ivy` (D11).
2. The display list, in `internal/display`. A page description emits
   commands to a device: fill a path, stroke a path, clip, draw an image,
   draw glyphs, and begin or end a group. The device that draws with okage is
   the main one. A device that records the commands serves the tests.
3. The layout, in `internal/layout`. A flow of text becomes HTML and CSS,
   and the layout puts it on pages. The layout is a port of WeasyPrint
   (D15), and it draws its pages through the device of the display list.
4. The fonts, in `internal/font`. It loads the fonts that a document
   embeds, finds a substitute for a font that it names but does not embed,
   and gives the outline of a glyph. It builds on the fonts of okage and on
   go-text, which read TrueType, OpenType and CFF (D7). It adds Type 1 and
   Type 3, which PDF and PostScript need, from the port of hayro-font (D14).

A format imports only the layers that it uses (D6).
