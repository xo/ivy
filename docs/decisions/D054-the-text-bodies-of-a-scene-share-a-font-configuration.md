# D54. The text bodies of a scene share a font configuration

Status: Decided.

Each text body of a slide or a drawing of `internal/drawingml` lays out its
text as a book of its own, and each book parses the fonts again with
`NewFontConfiguration`. On a scene of 30 text bodies, that is about a
quarter of the time. The maintainers asked on 2026-11-18 to share the
fonts.

The maintainers decided:

1. `layout.BookOptions` gets a field that takes a font configuration that
   other books share. `internal/layout` is internal, so the public API of
   ivy does not change (D11).
2. The books that share a configuration share one mutex, because the maps
   and the faces of go-text are not safe for concurrent use (D48).
3. `internal/drawingml` makes one configuration for each scene, and gives it
   to each text body of the scene.
