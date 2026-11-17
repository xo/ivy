// Package chart reads the charts of the Office formats and draws them onto
// the display list of D10.
//
// Parse reads a chart of DrawingML, the part with the root c:chartSpace that
// a slide, a document or a sheet of Office Open XML links to (ECMA-376 Part
// 1, 21.2), and the part of a chart of Office 2016 with the root
// cx:chartSpace. ParseODF reads a chart of OpenDocument, the content.xml of
// an embedded object with a chart:chart element. Both read the values only
// from the caches and the tables in the chart. The package computes no
// formula.
//
// The package lays a chart out and draws it itself, as Office and
// LibreOffice draw it (D27): the chart area and the plot area, the title,
// the legend, the axes with their tick marks, their gridlines and their
// labels, the series and the labels on the data. The scale of an axis of
// values follows the rules of Excel. The text is glyphs of the fonts of
// Options.Fonts.
//
// A *Chart is an Object of internal/layout: it has Size and Draw, so a format
// that lays out HTML can give it in Source.Objects.
//
// The charts of the types bar, column, line, area, pie, doughnut, scatter,
// bubble, radar and stock draw in their own form, and a combination of them
// draws with a secondary axis of values. The trendlines, the error bars and
// the drop lines of a series draw too, and a date axis places its
// categories by their dates. A 3D chart draws as its 2D form, and a bar of
// pie or a pie of pie as a pie. Of the charts of Office 2016, the funnel,
// the waterfall, the histogram, the Pareto chart and the box and whisker
// chart draw in their own form, and a hidden series draws nothing. A
// surface chart, a treemap, a sunburst and a region map draw as a frame
// with their title and their type. The data tables are left out.
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
	// maxTicks is the number of the major or the minor ticks of an axis. An
	// axis whose unit gives more ticks takes an automatic unit.
	maxTicks = 1000
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

// look is the set of defaults of a chart: the colors, the sizes of the text
// and the tick marks that a chart takes when its part does not give them.
type look int

// The looks of the charts.
const (
	// lookOffice is the look of a chart of DrawingML without its own
	// formats, as Office 2007 and LibreOffice draw it: black text, a bold
	// title and gray lines.
	lookOffice look = iota
	// lookModern is the look of a chart of Office 2016: gray text, a title
	// of 14 points and light gridlines.
	lookModern
	// lookLibre is the look of a chart of LibreOffice.
	lookLibre
)

// Chart is a chart, ready to draw. It does not change after Parse, so it is
// safe for concurrent use.
type Chart struct {
	// title is the title, one line for each paragraph, or nil.
	title     []string
	titleText textStyle
	// titleAt is the top left corner of the title, as fractions of the
	// size of the chart, or NaN for the automatic place.
	titleAt [2]float64
	plots   []*plot
	// cat and val are the primary axes of the categories and of the
	// values, and cat2 and val2 the secondary ones. For a scatter chart,
	// cat is the axis of x.
	cat, val   axis
	cat2, val2 axis
	// legend is the position of the legend: right, left, top, bottom or
	// top-right. It is empty for no legend.
	legend     string
	legendText textStyle
	// legendAt is the box of the legend, as fractions of the size of the
	// chart, or NaN for the automatic box.
	legendAt [4]float64
	// legendOver and titleOver draw the legend and the title over the plot
	// area, which keeps its size, c:overlay.
	legendOver, titleOver bool
	// legendHidden holds the numbers of the entries that the legend leaves
	// out, c:legendEntry with c:delete.
	legendHidden map[int]bool
	// plotAt is the box of the plot area, as fractions of the size of the
	// chart, or NaN for the automatic box. plotInner says that the box is
	// the inner box, without the labels of the axes.
	plotAt    [4]float64
	plotInner bool
	// blanks says how a line draws a missing value: gap, zero or span.
	blanks string
	// font is the family of the text, and text the style of the text that
	// sets none of its own.
	font string
	text textStyle
	// fill and border are the background and the border of the chart, and
	// plotFill and plotBorder the ones of the plot area, or transparent.
	fill, border         color.NRGBA
	plotFill, plotBorder color.NRGBA
	// colors are the default colors of the series and of the points, in
	// order.
	colors []color.NRGBA
	look   look
	// rotX is the angle of the view of a 3D chart, in degrees.
	rotX float64
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
	kindRadar
	kindBox
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
	// radius, and firstAngle the angle of the first slice, in degrees
	// clockwise from the top. ccw draws the slices counterclockwise, as
	// LibreOffice does.
	hole, firstAngle float64
	ccw              bool
	// gap is the gap between the categories, in percent of the width of a
	// bar, and overlap the overlap of the bars of a category, in percent,
	// or NaN for the default.
	gap, overlap float64
	// markers draws the points of a line, lines the lines of a scatter
	// chart, and smooth draws them as curves. step draws a line as steps:
	// start, end or center.
	markers, lines, smooth bool
	step                   string
	// bubbles sizes the points of a scatter chart by their third value, as
	// areas, and bubbleScale is the size of the largest bubble in percent
	// of the default.
	bubbles     bool
	bubbleScale float64
	// filled fills the areas of a radar chart.
	filled bool
	// flat3D says that the plot is 3D, which a pie draws as an ellipse.
	flat3D bool
	// secondary draws the plot on the secondary axes.
	secondary bool
	// hiLow draws the lines from the highest to the lowest value of each
	// category of a stock chart, and upDown the bars from the first to the
	// last value.
	hiLow, upDown bool
	// special is the layout of a chart of Office 2016 that draws as bars:
	// funnel, waterfall or pareto.
	special string
	// drop is the line of the drop lines of a line or an area chart, or
	// nil.
	drop   *stroke
	labels labels
	series []*series
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
	// color is the color of the series, if hasColor: the fill of a bar,
	// or the line of a line. noFill draws no fill.
	color    color.NRGBA
	hasColor bool
	noFill   bool
	// line is the color of the outline of a bar, or of the line, if
	// hasLine, lineWidth its width in points, or zero for the default, and
	// dash its preset dash, such as dash or sysDot. noLine draws no line
	// for a series of a line or a scatter chart.
	line      color.NRGBA
	hasLine   bool
	noLine    bool
	lineWidth float64
	dash      string
	// marker is the symbol of the points, such as circle, or empty for the
	// default, and markerSize its size in points, or zero.
	marker     string
	markerSize float64
	// markerFill and markerLine are the colors of a marker, if set.
	markerFill, markerLine       color.NRGBA
	hasMarkerFill, hasMarkerLine bool
	// noMarker draws no points.
	noMarker bool
	// points holds the colors of single points, c:dPt.
	points map[int]color.NRGBA
	// explode moves the slices of a pie out of the center, as a fraction of
	// the radius, and explodes the single slices.
	explode  float64
	explodes map[int]float64
	// smooth draws the line of the series as a curve.
	smooth bool
	labels labels
	// format is the number format of the values, from their cache.
	format string
	// totals holds the points of a waterfall that are totals.
	totals map[int]bool
	// trends are the trendlines, and errs the error bars.
	trends []trendline
	errs   []errBars
	// dates are the numbers of the categories, which a date axis places
	// by their dates, and dateFormat their number format.
	dates      []float64
	dateFormat string
	// span draws a line across the missing values, such as the days
	// without a value of a date axis.
	span bool
}

// labels are the labels on the data of a plot or a series.
type labels struct {
	// set says that the chart gives the labels, and not the defaults.
	set                bool
	value, percent     bool
	category, seriesOf bool
	// format is the number format of the values, or empty.
	format string
	// separator is the text between the parts of a label, or empty for
	// the default.
	separator string
	hasSep    bool
	// position is the position of a label, such as outEnd or ctr.
	position string
	text     textStyle
	// points holds the labels of single points, c:dLbl.
	points map[int]*pointLabel
}

// pointLabel is the label of a single point, c:dLbl: its parts, or its own
// text, and its offset from its place.
type pointLabel struct {
	labels
	lines   []string
	hasText bool
	deleted bool
	// dx and dy move the label from its place, as fractions of the size of
	// the chart.
	dx, dy float64
}

// show says whether the labels draw anything.
func (l labels) show() bool {
	return l.value || l.percent || l.category || l.seriesOf
}

// any says whether the labels or the label of a point draw anything.
func (l labels) any() bool {
	return l.show() || len(l.points) > 0
}

// axis is an axis of a chart.
type axis struct {
	present bool
	deleted bool
	// id is c:axId, and cross the c:crossAx of the axis.
	id, cross string
	// position is b, l, t or r.
	position string
	title    string
	// format is the number format of the labels, or empty for the format
	// of the values.
	format string
	// min, max, major and minor are NaN when the chart does not give them.
	min, max, major, minor float64
	// logBase is the base of a logarithmic axis, or zero.
	logBase float64
	// unit divides the values of the labels, such as 1000 for thousands,
	// or is zero.
	unit            float64
	grid, minorGrid bool
	// gridColor and minorColor are the colors of the gridlines, if set.
	gridColor, minorColor       color.NRGBA
	hasGridColor, hasMinorColor bool
	// line is the color of the line of the axis, if hasLine. noLine draws
	// no line.
	line    color.NRGBA
	hasLine bool
	noLine  bool
	// majorTick and minorTick are the tick marks: out, in, cross or none,
	// or empty for the default.
	majorTick, minorTick string
	// labelPos is where the labels are: nextTo, low, high or none.
	labelPos string
	// crosses is where the axis crosses the other axis: autoZero, min or
	// max, and crossesAt the value, or NaN.
	crosses   string
	crossesAt float64
	// between puts the points of an axis of categories between the tick
	// marks, and not on them, if betweenSet. The axis of values gives it.
	between, betweenSet bool
	// reverse draws the axis from its maximum, c:orientation maxMin.
	reverse bool
	// date says that the axis is c:dateAx, whose categories are dates,
	// and baseUnit is its unit: days, months or years, or empty for the
	// automatic unit.
	date     bool
	baseUnit string
	// rotation is the angle of the labels in degrees, counterclockwise, if
	// hasRotation.
	rotation    float64
	hasRotation bool
	// skip draws one label of skip, or is zero for the automatic skip.
	skip int
	text textStyle
	// titleText is the style of the title of the axis.
	titleText textStyle
}

// newAxis returns an axis without limits.
func newAxis() axis {
	return axis{min: math.NaN(), max: math.NaN(), major: math.NaN(), minor: math.NaN(), crossesAt: math.NaN()}
}

// shown says whether the axis draws its labels.
func (a axis) shown() bool {
	return a.present && !a.deleted && a.labelPos != "none"
}

// textStyle is the style of a text: its size in points, bold, and its
// color, if hasColor.
type textStyle struct {
	size     float64
	bold     bool
	hasBold  bool
	color    color.NRGBA
	hasColor bool
	// font is the family, or empty for the font of the chart.
	font string
	// rotation is the angle of the text in degrees, counterclockwise, if
	// hasRotation.
	rotation    float64
	hasRotation bool
}

// merge returns s with the fields that s does not set taken from base.
func (s textStyle) merge(base textStyle) textStyle {
	if s.size <= 0 {
		s.size = base.size
	}
	if !s.hasColor {
		s.color, s.hasColor = base.color, base.hasColor
	}
	if !s.hasBold {
		s.bold, s.hasBold = base.bold, base.hasBold
	}
	if s.font == "" {
		s.font = base.font
	}
	if !s.hasRotation {
		s.rotation, s.hasRotation = base.rotation, base.hasRotation
	}
	return s
}

// Size returns the size of the chart in points, from Options or from the
// chart of OpenDocument. It returns zero when the chart has no size, and
// the frame that holds it gives the size.
func (c *Chart) Size() (width, height float64) {
	return c.width, c.height
}

// nan4 returns a box without a value.
func nan4() [4]float64 {
	return [4]float64{math.NaN(), math.NaN(), math.NaN(), math.NaN()}
}

// finish gives each series without a color its default color, and the text
// of the chart its defaults.
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

// palette returns the default color of the series or the point with the
// number i.
func (c *Chart) palette(i int) color.NRGBA {
	return c.colors[max(0, i)%len(c.colors)]
}

// pointColor returns the color of the point i of a series. With vary, a
// point without a color of its own takes the color of its number.
func (c *Chart) pointColor(s *series, i int, vary bool) color.NRGBA {
	if pc, ok := s.points[i]; ok {
		return pc
	}
	if vary {
		return c.palette(i)
	}
	return s.color
}

// newChart returns a chart with the defaults of Office.
func newChart(opts Options) *Chart {
	return &Chart{
		cat: newAxis(), val: newAxis(), cat2: newAxis(), val2: newAxis(),
		blanks:   "gap",
		rotX:     30,
		text:     textStyle{size: 10},
		titleAt:  [2]float64{math.NaN(), math.NaN()},
		legendAt: nan4(),
		plotAt:   nan4(),
		width:    max(0, opts.Width), height: max(0, opts.Height),
		fonts: opts.Fonts,
		cfg:   opts.Config,
	}
}

// styleText gives the text of the chart the defaults of its look: the size
// and the color of each text that the chart does not set.
func (c *Chart) styleText() {
	base := c.text
	if base.size <= 0 {
		base.size = 10
	}
	if !base.hasColor {
		base.color, base.hasColor = c.defaultTextColor(), true
	}
	c.text = base
	small := base
	switch c.look {
	case lookModern:
		small.size = math.Round(base.size*9) / 10
		c.titleText = c.titleText.merge(textStyle{size: math.Round(base.size*14) / 10}).merge(base)
	case lookLibre:
		c.titleText = c.titleText.merge(textStyle{size: math.Round(base.size*13) / 10}).merge(base)
	default:
		c.titleText = c.titleText.merge(textStyle{size: max(18, base.size*1.2), bold: true, hasBold: true}).merge(base)
	}
	c.legendText = c.legendText.merge(small)
	axisTitle := base
	if c.look == lookOffice {
		axisTitle.bold, axisTitle.hasBold = true, true
	}
	for _, a := range []*axis{&c.cat, &c.val, &c.cat2, &c.val2} {
		a.text = a.text.merge(small)
		a.titleText = a.titleText.merge(axisTitle)
	}
	label := small
	if c.look == lookModern {
		label.color = labelGray
	}
	for _, p := range c.plots {
		p.labels.text = p.labels.text.merge(label)
		for _, s := range p.series {
			s.labels.text = s.labels.text.merge(p.labels.text)
		}
	}
}

// defaultTextColor returns the color of a text that sets no color.
func (c *Chart) defaultTextColor() color.NRGBA {
	if c.look == lookModern {
		return textGray
	}
	return color.NRGBA{0, 0, 0, 255}
}
