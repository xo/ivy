# D27. ivy draws the charts of Office itself

Status: Decided. Amends D24.

D24 item 6 drew the charts of DrawingML and OpenDocument with zuzu: ivy
turned a chart into a spec of Vega-Lite, and zuzu laid it out. On
2026-11-17 the visual review showed that those charts look too different
from the charts of Office and LibreOffice, in their axes, their spacing,
their labels and their legends. zuzu follows the look of Vega, and an
Office chart has a look of its own.

The maintainers decided:

1. `internal/chart` lays out and draws the charts of Office itself, onto
   the display list (D10), as Office and LibreOffice draw them: the plot
   area, the axes with their ticks, their gridlines and their labels, the
   series, the data labels, the title and the legend.
2. ivy does not use zuzu for the charts of documents, and `go.mod` does not
   require zuzu.
3. The parsers of the chart parts and the API of `internal/chart` stay, so
   the formats that draw charts do not change.
