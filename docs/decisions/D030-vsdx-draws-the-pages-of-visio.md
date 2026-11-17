# D30. vsdx draws the pages of Visio

Status: Decided.

The package `vsdx` gives only the thumbnail and the properties of a Visio
file. A file without a thumbnail gives nothing to see, and the visual
review of 2026-11-17 showed two such `.vsdx` files. A thumbnail also shows
only the first page, small. The maintainers asked to draw the pages.

The maintainers decided:

1. `vsdx` draws each page of a Visio file in the XML format (`.vsdx`,
   `.vsdm`, `.vstx`, `.vstm`) onto the display list (D10), at the size of
   the page: the shapes with their geometry (lines, arcs, elliptical arcs,
   ellipses, polylines, splines and NURBS), their fills, their lines and
   their arrows, their text, their groups and their transforms, with the
   inheritance from the masters, the styles and the theme, as [MS-VSDX]
   says.
2. The thumbnail stays a preview. A file in the binary format (`.vsd`)
   still gives only its thumbnail.
3. The code that draws DrawingML, in `internal/drawingml`, serves `vsdx`
   where the two formats agree, such as the theme and the colors.
