# Test files

LibreOffice 26.8 wrote these files with `soffice --headless --convert-to`,
from a short text, a CSV file of two rows, or a flat ODP file of one slide.
LibreOffice writes a thumbnail into an OpenDocument file, and none into an
Office Open XML file.

## The path gradients

A script wrote `path-gradients.pptx` as XML in a zip archive. Each of its
four slides holds a star (star5), an ellipse, a moon and a triangle of 3.2
by 3.2 inches. Each shape has a path gradient from red (FF0000) at the
focus to blue (0000FF) at the edge:

1. The path `shape`, with the focus at the center of the box.
2. The path `rect`, with the focus at the center of the box.
3. The path `circle`, with the focus at the center of the box.
4. The path `shape`, with the focus at the top left corner of the box.

On 2026-11-19, PowerPoint 2024 exported the file to PDF in the Windows VM
of the review, to show how PowerPoint draws each path.

## The effects

On 2026-11-19, PowerPoint 2024 wrote `effects.pptx` through COM in the
Windows VM of the review. A script of PowerShell made the shapes and set
their effects. Some shapes show a picture of 128 by 64 pixels with four
ramps from black: to white, red, green and blue. The slides are 720 by 405
points:

1. A glow on a rectangle, an ellipse, a rectangle without a fill and the
   picture.
2. A soft edge of 10 points on a rectangle, of 25 points on an ellipse, and
   of 5 points on the picture.
3. An inner shadow on two rectangles and an ellipse, with a blur, without a
   blur, and in two directions.
4. Reflections of the types 1, 2, 5 and 9 of PowerPoint, and one with a
   size of 80 percent, on rectangles, an ellipse and the picture.
5. Lines of 16 points of `thickThin` and `thinThick`: rectangles, ellipses,
   lines from left to right and from right to left, and lines down and up.

PowerPoint exported the file to PDF in the same VM. The export shows on
which side of a path each kind of line draws its thick line.
