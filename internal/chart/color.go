package chart

import (
	"image/color"
	"maps"
	"strconv"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/drawingml"
)

// officeColors are the colors of the theme of Office 2013 and later, for a
// chart without a theme, or with a theme that lacks a color.
var officeColors = map[string]color.NRGBA{
	"dk1": {0, 0, 0, 255}, "lt1": {255, 255, 255, 255},
	"dk2": {0x44, 0x54, 0x6a, 255}, "lt2": {0xe7, 0xe6, 0xe6, 255},
	"accent1": {0x44, 0x72, 0xc4, 255}, "accent2": {0xed, 0x7d, 0x31, 255},
	"accent3": {0xa5, 0xa5, 0xa5, 255}, "accent4": {0xff, 0xc0, 0x00, 255},
	"accent5": {0x5b, 0x9b, 0xd5, 255}, "accent6": {0x70, 0xad, 0x47, 255},
	"hlink": {0x05, 0x63, 0xc1, 255}, "folHlink": {0x95, 0x4f, 0x72, 255},
}

// The colors of the text and the lines of the charts.
var (
	// textGray is the color of the text of a chart of Office 2016, and
	// gridGray the color of its gridlines.
	textGray = color.NRGBA{0x59, 0x59, 0x59, 255}
	gridGray = color.NRGBA{0xd9, 0xd9, 0xd9, 255}
	// labelGray is the color of the labels on the data of a chart of
	// Office 2016.
	labelGray = color.NRGBA{0x40, 0x40, 0x40, 255}
	// officeGray is the color of the axes and the gridlines of a chart of
	// Office 2007, and libreGray the one of a chart of LibreOffice.
	officeGray = color.NRGBA{0x86, 0x86, 0x86, 255}
	libreGray  = color.NRGBA{0xb3, 0xb3, 0xb3, 255}
)

// withOffice returns a theme with the colors of theme, and the colors of
// the theme of Office that theme lacks.
func withOffice(theme *drawingml.Theme) *drawingml.Theme {
	t := &drawingml.Theme{Colors: maps.Clone(officeColors)}
	if theme == nil {
		t.Minor.Latin, t.Major.Latin = "Calibri", "Calibri Light"
		return t
	}
	cp := *theme
	cp.Colors = t.Colors
	maps.Copy(cp.Colors, theme.Colors)
	return &cp
}

// accent returns the default color of the series with the number index, as
// Office gives it: the six accents of the theme, then the accents darker and
// lighter in turns.
func accent(theme *drawingml.Theme, index int) color.NRGBA {
	index = max(0, index)
	c, ok := theme.Color("accent" + strconv.Itoa(index%6+1))
	if !ok {
		c = officeColors["accent"+strconv.Itoa(index%6+1)]
	}
	// Each round of six mixes the accents with black, a negative fraction,
	// or with white, a positive one.
	rounds := []float64{0, -0.4, 0.2, -0.2, 0.4, -0.5, 0.3, -0.3, 0.5}
	t := rounds[(index/6)%len(rounds)]
	rgba := drawingml.RGBAFrom(c)
	switch {
	case t < 0:
		rgba = rgba.Mix(drawingml.Black, -t)
	case t > 0:
		rgba = rgba.Mix(drawingml.White, t)
	}
	return rgba.NRGBA()
}

// fillColor reads the color of a fill or a line of DrawingML from n, the
// parent of a:solidFill, such as c:spPr or a:ln. It returns false for no
// color, and noFill reports a:noFill. A gradient gives the color of its
// first stop, and a pattern its foreground color.
func fillColor(n roxmltree.Node, colors *drawingml.Colors) (c color.NRGBA, ok, noFill bool) {
	for e := range elements(n) {
		switch e.TagName().Name() {
		case "noFill":
			return color.NRGBA{}, false, true
		case "solidFill":
			v, ok := colors.In(e)
			return v.NRGBA(), ok, false
		case "gradFill":
			for gs := range e.Descendants() {
				if gs.IsElement() && gs.TagName().Name() == "gs" {
					v, ok := colors.In(gs)
					return v.NRGBA(), ok, false
				}
			}
		case "pattFill":
			if fg, found := child(e, "fgClr"); found {
				v, ok := colors.In(fg)
				return v.NRGBA(), ok, false
			}
		}
	}
	return color.NRGBA{}, false, false
}
