# D51. msdoc draws its shapes with internal/odraw

Status: Decided.

`msdoc` draws the shapes of a Word 97 document as SVG images in the HTML of
the layout. `msppt` and `msxls` draw the same shapes of Office Drawing
(MS-ODRAW) with `internal/odraw` and `internal/drawingml`, on the display
list. So `msdoc` lacks most preset shapes and the WordArt, and a fix in one
reader of shapes does not reach the other. The maintainers asked on
2026-11-18 how `msdoc` gets the full set of shapes.

The maintainers decided:

1. The layout can place a box whose content is a `display.List`. The box
   has a size, and the layout draws the list in it, at the place of the box.
   The layout records and replays the list as it does other content (D47).
2. `msdoc` gives its shapes, pictures and text boxes to `internal/odraw`,
   and puts the list that odraw draws in such a box. The readers of shapes
   and pictures of `msdoc`, `picture.go` and `shape.go`, move into
   `internal/odraw` where odraw does not read the same properties already.
3. Word, Excel and PowerPoint then share one reader and one drawing of the
   shapes of Office Drawing.

The other choices were an SVG output of `internal/drawingml`, which needs a
second output to keep in step, and more SVG code in `msdoc`, which repeats
`internal/odraw`.
