package xlsx

import (
	"image/color"
	"os"
	"testing"

	"github.com/xo/ivy"
)

// TestChartCells renders the treemap and the sunburst of
// testdata/chart-treemap.xlsx and testdata/chart-sunburst.xlsx. Excel
// writes a chart of Office 2016 of a workbook without a cache of its data,
// so the chart reads its cells through the defined names _xlchart.v1.N.
// Without the cells, the chart draws as a frame without colors.
func TestChartCells(t *testing.T) {
	// the first three accents of the theme of Office 2023, which give the
	// colors of Fruit, Vegetables and Dairy
	accents := []color.NRGBA{{0x15, 0x60, 0x82, 255}, {0xe9, 0x71, 0x32, 255}, {0x19, 0x6b, 0x24, 255}}
	for _, name := range []string{"chart-treemap.xlsx", "chart-sunburst.xlsx"} {
		t.Run(name, func(t *testing.T) {
			buf, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			d := open(t, buf, ivy.Config{})
			page, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			img, err := ivy.RenderImage(t.Context(), page, 72, color.White)
			if err != nil {
				t.Fatal(err)
			}
			b := img.Bounds()
			for _, accent := range accents {
				n := 0
				for y := b.Min.Y; y < b.Max.Y; y++ {
					for x := b.Min.X; x < b.Max.X; x++ {
						if near(nrgba(img.At(x, y)), accent) {
							n++
						}
					}
				}
				if n < 500 {
					t.Errorf("expected the color %v, got %d pixels", accent, n)
				}
			}
		})
	}
}
