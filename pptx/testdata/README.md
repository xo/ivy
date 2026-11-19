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
