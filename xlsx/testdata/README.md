# Test files

LibreOffice 26.8 wrote these files with `soffice --headless --convert-to`,
from a short text, a CSV file of two rows, or a flat ODP file of one slide.
LibreOffice writes a thumbnail into an OpenDocument file, and none into an
Office Open XML file.

## The row heights

A script wrote each `row-heights-<font>-<size>.xlsx` file as XML in a zip
archive. The default font of the file is the font of its name, at its size.
Its sheet holds 19 rows and four columns with thin borders on each cell, on
US Letter, with margins of 0.5 inch and no header or footer:

- Rows 1 to 13 have custom heights of 6, 9, 12, 13.5, 15, 18, 21, 24, 30,
  36, 48, 72 and 100 points.
- Rows 14 to 16 have no height.
- Rows 17 and 18 have a height of 30 points that is not custom.
- Row 19 has a height of 12 points that is not custom, and a cell with two
  lines of wrapped text.

The columns A, B and C have custom widths of 10, 20 and 5 characters.
Column D has the default width. On 2026-11-19, Excel 2024 printed these
files in the Windows VM of the review, to measure how Excel scales the rows
and the columns on the printer.

## The charts

On 2026-11-21, Excel 2024 made the five `chart-*.xlsx` files in the
Windows VM of the review, through COM, and printed them to PDF. Each file
holds a table of data and one or two charts of that table:

- `chart-treemap.xlsx`: a treemap of three levels with the labels of the
  categories. The labels of the parents overlap the children.
- `chart-treemap-banner.xlsx`: the same treemap with banners, the labels
  of the categories and the values, and a legend.
- `chart-sunburst.xlsx`: a sunburst of the same data, with the labels of
  the categories. Some leaves are at the second level.
- `chart-data-table.xlsx`: a column chart with a data table with the keys
  of the legend, and a line chart with a data table without them.
- `chart-trendline-labels.xlsx`: a scatter chart with a linear and a
  polynomial trendline, and a column chart with an exponential, a
  logarithmic and a power trendline, with their equations and their R
  squared.

Excel writes a treemap and a sunburst of a workbook without a cache of
their data. The chart part names the cells through the defined names
`_xlchart.v1.0` to `_xlchart.v1.2`. The default font of the files is Aptos
Narrow, of the theme of Office 2023. The VM has no Aptos, so Excel printed
the cells with Arial, and its columns are wider than the columns of ivy.
