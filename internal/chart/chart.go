// Package chart reads the charts of the Office formats and draws them onto
// the display list of D10.
//
// Parse reads a chart of DrawingML, the part with the root c:chartSpace that
// a slide, a document or a sheet of Office Open XML links to (ECMA-376 Part
// 1, 21.2). ParseODF reads a chart of OpenDocument, the content.xml of an
// embedded object with a chart:chart element. Both read the values only from
// the caches and the tables in the chart. The package computes no formula.
//
// A chart becomes a spec of Vega-Lite. zuzu lays the spec out at the size of
// the chart, and ivy draws the scene of zuzu as vectors onto a
// display.Device (D24). After the layout, the drawing adds the labels on
// the data, the hole of a doughnut and the angle of the first slice of a pie
// to the scene, at the places that zuzu gives the marks.
//
// A *Chart is an Object of internal/layout: it has Size and Draw, so a format
// that lays out HTML can give it in Source.Objects.
//
// The charts of the types bar, column, line, area, pie, doughnut, scatter
// and bubble draw in their own form. A 3D chart draws as its 2D form. A
// radar chart and a stock chart draw as a line chart of the same values, a
// filled radar chart as an area chart, and a bar of pie or a pie of pie as a
// pie. A surface chart draws as a frame with its title and its type.
package chart

import (
	"image/color"
	"math"

	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
)

// The limits of one chart, below the limits of ivy.Config. A chart with more
// series or points draws the first ones, and Draw returns an
// *ivy.PartialError.
const (
	// maxSeries is the number of series of a chart.
	maxSeries = 256
	// maxPoints is the number of points of all the series of a chart.
	maxPoints = 1 << 14
	// maxCategories is the number of categories of a chart.
	maxCategories = 1 << 12
	// maxText is the number of bytes of a title or a label.
	maxText = 1 << 10
)

// Options are the options of Parse and ParseODF.
type Options struct {
	// Theme gives the colors and the fonts that a chart of DrawingML names,
	// or is nil for the theme of Office.
	Theme *drawingml.Theme
	// Config gives the limits, the logger and the fonts of the program.
	Config ivy.Config
	// Fonts are the fonts of the text. A format gives the database that it
	// lays out its own text with, so the chart looks like the text around
	// it. A nil Fonts is the database of the fonts of Config, which
	// fontdb.Of of internal/fontdb gives: the fonts of Config.Fonts, then
	// the Go fonts, then the fonts of the system when Config.SystemFonts is
	// set. The chart draws with its own font, then with the font of
	// font.Substitute that has the same metrics, such as Carlito for
	// Calibri, then with the sans serif family of the database.
	Fonts *svg.FontDatabase
	// Width and Height are the size of the frame of the chart in points,
	// or zero. A chart of DrawingML has no size of its own, and the frame
	// that holds it gives the size.
	Width, Height float64
}

// Chart is a chart, ready to draw. It does not change after Parse, so it is
// safe for concurrent use.
type Chart struct {
	// title is the title, one line for each paragraph, or nil.
	title     []string
	titleText textStyle
	plots     []*plot
	// cat and val are the axes of the categories and of the values. For a
	// scatter chart, cat is the axis of x.
	cat, val axis
	// legend is the position of the legend: right, left, top, bottom or
	// top-right. It is empty for no legend.
	legend     string
	legendText textStyle
	// blanks says how a line draws a missing value: gap, zero or span.
	blanks string
	// font is the family of the text, and text the style of the text that
	// sets none of its own.
	font string
	text textStyle
	// fill and border are the background and the border of the chart, or
	// transparent.
	fill, border color.NRGBA
	// colors are the default colors of the series and of the points, in
	// order.
	colors []color.NRGBA
	// unsupported names the type of a chart that draws as a frame.
	unsupported string
	// truncated says that the chart has more series or points than the
	// limits, and draws only the first ones.
	truncated bool

	width, height float64
	fonts         *svg.FontDatabase
	cfg           ivy.Config
}

// kind is the form that a plot draws in.
type kind int

// The kinds of plot.
const (
	kindBar kind = iota + 1
	kindLine
	kindArea
	kindPie
	kindScatter
)

// plot is one chart type of a plot area, such as c:barChart. A combination
// chart has several plots.
type plot struct {
	kind kind
	// horizontal draws the bars from the left, for c:barDir bar.
	horizontal bool
	// grouping is clustered, stacked, percentStacked or standard.
	grouping string
	// varyColors gives each point of a single series a color of its own.
	varyColors bool
	// hole is the size of the hole of a doughnut, as a fraction of the
	// radius, and firstAngle the angle of the first slice, in degrees.
	hole, firstAngle float64
	// gap is the gap between the categories, in percent of the width of a
	// bar.
	gap float64
	// markers draws the points of a line, lines the lines of a scatter
	// chart, and smooth draws them as curves.
	markers, lines, smooth bool
	// bubbles sizes the points of a scatter chart by their third value.
	bubbles bool
	labels  labels
	series  []*series
}

// series is one series of a plot.
type series struct {
	name string
	// index is the number of the series in the chart, c:idx, which picks
	// its default color.
	index int
	// cats are the labels of the categories, vals the values, and xs and
	// sizes the values of x and the sizes of the bubbles of a scatter
	// chart. A missing value is NaN.
	cats  []string
	vals  []float64
	xs    []float64
	sizes []float64
	// color is the color of the series, if hasColor.
	color    color.NRGBA
	hasColor bool
	// noLine draws no line for a series of a line or a scatter chart, and
	// noMarker no points.
	noLine, noMarker bool
	// points holds the colors of single points, c:dPt.
	points map[int]color.NRGBA
	labels labels
	// format is the number format of the values, from their cache.
	format string
}

// labels are the labels on the data of a plot or a series.
type labels struct {
	// set says that the chart gives the labels, and not the defaults.
	set                bool
	value, percent     bool
	category, seriesOf bool
	// format is the number format of the values, or empty.
	format string
	// position is the position of a label, such as outEnd or ctr.
	position string
	text     textStyle
}

// show says whether the labels draw anything.
func (l labels) show() bool {
	return l.value || l.percent || l.category || l.seriesOf
}

// axis is an axis of a chart.
type axis struct {
	present bool
	deleted bool
	// position is b, l, t or r.
	position string
	title    string
	// format is the number format of the labels, or empty for the format
	// of the values.
	format string
	// min, max and major are NaN when the chart does not give them.
	min, max, major float64
	grid            bool
	// reverse draws the axis from its maximum, c:orientation maxMin.
	reverse bool
	// date says that the axis is c:dateAx, whose categories are dates.
	date bool
	text textStyle
	// titleText is the style of the title of the axis.
	titleText textStyle
}

// newAxis returns an axis without limits.
func newAxis() axis {
	return axis{min: math.NaN(), max: math.NaN(), major: math.NaN()}
}

// textStyle is the style of a text: its size in points, bold, and its
// color, if hasColor.
type textStyle struct {
	size     float64
	bold     bool
	color    color.NRGBA
	hasColor bool
	// font is the family, or empty for the font of the chart.
	font string
}

// merge returns s with the fields that s does not set taken from base.
func (s textStyle) merge(base textStyle) textStyle {
	if s.size <= 0 {
		s.size = base.size
	}
	if !s.hasColor {
		s.color, s.hasColor = base.color, base.hasColor
	}
	if s.font == "" {
		s.font = base.font
	}
	return s
}

// Size returns the size of the chart in points, from Options or from the
// chart of OpenDocument. It returns zero when the chart has no size, and
// the frame that holds it gives the size.
func (c *Chart) Size() (width, height float64) {
	return c.width, c.height
}

// finish gives each series without a color its default color.
func (c *Chart) finish() {
	if len(c.colors) == 0 {
		c.colors = []color.NRGBA{{0, 0, 0, 255}}
	}
	for _, p := range c.plots {
		for _, s := range p.series {
			if !s.hasColor {
				s.color, s.hasColor = c.palette(s.index), true
			}
		}
	}
}
