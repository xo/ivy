package msxls

import (
	"context"
	"image/color"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/chart"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/numfmt"
	"github.com/xo/ivy/internal/sheet"
)

// This file reads the charts of a workbook: the chart substreams that
// follow a record OBJ of a chart in a worksheet, and the substream of a
// chart sheet (MS-XLS 2.1.7.20). It converts each chart into a part of
// DrawingML, c:chartSpace, which internal/chart reads and draws (D27), as
// it draws the charts of xlsx. The values of a series come from the cache
// of the chart, the records after SIIndex, or else from the cells that its
// formula names.
//
// Excel 2007 and later also write the fills, the lines and the texts of a
// chart as DrawingML, in the records ShapePropsStream and TextPropsStream.
// When a chart has them, the part takes them, and an object without them
// takes the defaults of the chart, as Excel draws it. A chart of Excel 97
// takes the formats of BIFF8, and the colors of the palette for its
// automatic formats.

// The records of a chart substream (MS-XLS 2.3).
const (
	recChartSeries    = 0x1003
	recDataFormat     = 0x1006
	recLineFormat     = 0x1007
	recMarkerFormat   = 0x1009
	recAreaFormat     = 0x100A
	recPieFormat      = 0x100B
	recAttachedLabel  = 0x100C
	recSeriesText     = 0x100D
	recChartFormat    = 0x1014
	recLegend         = 0x1015
	recBar            = 0x1017
	recLine           = 0x1018
	recPie            = 0x1019
	recArea           = 0x101A
	recScatter        = 0x101B
	recCrtLine        = 0x101C
	recAxis           = 0x101D
	recTick           = 0x101E
	recValueRange     = 0x101F
	recCatSerRange    = 0x1020
	recAxisLine       = 0x1021
	recDefaultText    = 0x1024
	recText           = 0x1025
	recFontX          = 0x1026
	recObjectLink     = 0x1027
	recFrame          = 0x1032
	recBegin          = 0x1033
	recEnd            = 0x1034
	recPlotArea       = 0x1035
	recChart3d        = 0x103A
	recDropBar        = 0x103D
	recRadar          = 0x103E
	recSurf           = 0x103F
	recRadarArea      = 0x1040
	recAxisParent     = 0x1041
	recShtProps       = 0x1044
	recSerToCrt       = 0x1045
	recIFmt           = 0x104E
	recBRAI           = 0x1051
	recSerFmt         = 0x105D
	recSerParent      = 0x104A
	recSerAuxTrend    = 0x104B
	recBopPop         = 0x1061
	recDat            = 0x1063
	recSIIndex        = 0x1065
	recShapeProps     = 0x08A4
	recTextProps      = 0x08A5
	recExternSheet    = 0x0017
	recSupBook        = 0x01AE
	bofChart          = 0x0020
	maxChartRecords   = 1 << 16
	maxChartPoints    = 1 << 14
	maxChartSeries    = 256
	maxChartGroups    = 16
	maxChartStreamLen = 1 << 16
)

// office2007 is the theme of Office 2007, which the charts of a workbook of
// Excel 2007 take.
var office2007 = &drawingml.Theme{
	Colors: map[string]color.NRGBA{
		"dk1": rgbOf(0x000000), "lt1": rgbOf(0xFFFFFF), "dk2": rgbOf(0x1F497D), "lt2": rgbOf(0xEEECE1),
		"accent1": rgbOf(0x4F81BD), "accent2": rgbOf(0xC0504D), "accent3": rgbOf(0x9BBB59),
		"accent4": rgbOf(0x8064A2), "accent5": rgbOf(0x4BACC6), "accent6": rgbOf(0xF79646),
		"hlink": rgbOf(0x0000FF), "folHlink": rgbOf(0x800080),
	},
	Major: drawingml.Fonts{Latin: "Cambria"},
	Minor: drawingml.Fonts{Latin: "Calibri"},
}

// rgbOf returns the color of the value 0xRRGGBB.
func rgbOf(v uint32) color.NRGBA {
	return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

// lineFmt is a record LineFormat (MS-XLS 2.4.156).
type lineFmt struct {
	c      color.NRGBA
	lns    int
	we     int
	auto   bool
	axisOn bool
}

// areaFmt is a record AreaFormat (MS-XLS 2.4.3).
type areaFmt struct {
	fore color.NRGBA
	fls  int
	auto bool
}

// markerFmt is a record MarkerFormat (MS-XLS 2.4.160).
type markerFmt struct {
	fore, back       color.NRGBA
	imk              int
	auto             bool
	noFill, noBorder bool
	size             float64
}

// chartFmt is the format of a frame, of a series or of a point.
type chartFmt struct {
	line   *lineFmt
	area   *areaFmt
	marker *markerFmt
	// spPr and markerSpPr are the shapes of DrawingML of Excel 2007, and
	// hasSpPr says that the object has a record ShapePropsStream.
	spPr, markerSpPr string
	hasSpPr          bool
	explode          int
	smooth           bool
	// labels are the flags of a record AttachedLabel, if hasLabels.
	labels    uint16
	hasLabels bool
}

// brai is a record BRAI: a link to the name, the values, the categories or
// the bubble sizes of a series (MS-XLS 2.4.30).
type brai struct {
	rt   int
	rgce []byte
}

// chartVal is a value of a series: a number or a text, with its XF.
type chartVal struct {
	num   float64
	text  string
	isNum bool
	xf    int
}

// chartSeries is a series of a chart.
type chartSeries struct {
	name string
	refs [4]brai
	// order is the order of the series in its chart group, iss of its
	// record DataFormat, which also picks its automatic color.
	order  int
	group  int
	sdtX   int
	format chartFmt
	points map[int]*chartFmt
	// cache holds the values of the records after SIIndex: 1 for the
	// values, 2 for the categories and 3 for the bubble sizes.
	cache [4]map[int]chartVal
	// parent is the number from 1 of the series that a trendline or error
	// bars belong to, SerParent, or 0 for a series of values. trend is the
	// trendline of such a series, or nil.
	parent int
	trend  *trendFmt
}

// trendFmt is a record SerAuxTrend: a trendline (MS-XLS 2.4.247).
type trendFmt struct {
	// regt is the type: 0 for a polynomial, which is linear of the order
	// 1, 1 exponential, 2 logarithmic, 3 power and 4 a moving average.
	// order is the order of the polynomial or the period of the moving
	// average.
	regt, order int
	// intercept is the value of y at x 0, if hasIntercept.
	intercept    float64
	hasIntercept bool
	eq, r2       bool
	// forward and backward extend the line.
	forward, backward float64
}

// datFmt is a record Dat: the data table of a chart (MS-XLS 2.4.72).
type datFmt struct {
	horz, vert, outline, keys bool
}

// chartGroup is a chart group: a record ChartFormat and its type.
type chartGroup struct {
	kind                              string
	parent                            int
	icrt                              int
	horizontal, stacked, percent      bool
	bubbles, varied, filled           bool
	overlap, gap, donut, start, scale int
	hiLow, upDown                     bool
	legend                            *chartLegend
}

// chartLegend is a record Legend with its text: its position, and its
// place in 1/4000 of the chart for a legend that is not docked.
type chartLegend struct {
	pos  int
	x, y int
	text *chartText
}

// chartAxis is an axis of an axis group.
type chartAxis struct {
	kind                   int
	between, reverse       bool
	maxCross, hasCatRange  bool
	labelSkip              int
	min, max, major, minor float64
	cross                  float64
	auto                   uint16
	hasRange               bool
	tickMajor, tickMinor   int
	labelPos               int
	hasTick                bool
	font                   *sheet.Font
	line, grid, minorGrid  *lineFmt
	lineID                 int
	spPr, gridSpPr         string
	hasSpPr, hasGridSpPr   bool
	txPr                   string
	ifmt                   int
	title                  *chartText
}

// chartText is a record Text with what follows it: its text, its font and
// what it labels.
type chartText struct {
	text       string
	ref        brai
	font       *sheet.Font
	link       int
	var1, var2 int
	flags      uint16
	txPr       string
	parent     int
	isDefault  int
}

// biffChart is a chart as its substream gives it.
type biffChart struct {
	modern  bool
	area    chartFmt
	plot    chartFmt
	hasPlot bool
	series  []*chartSeries
	groups  []*chartGroup
	axes    [2][3]*chartAxis
	title   *chartText
	labels  []*chartText
	defFont *sheet.Font
	blanks  int
	visOnly bool
	table   *datFmt
}

// chartLookup gives a chart the cells of the sheets of the workbook.
type chartLookup struct {
	ctx   context.Context
	lim   ivy.Limits
	w     *workbook
	cells map[int]map[[2]int]cell
}

// sheetCells returns the cells of the sheet itab, by their row and their
// column.
func (l *chartLookup) sheetCells(itab int) map[[2]int]cell {
	if m, ok := l.cells[itab]; ok {
		return m
	}
	m := map[[2]int]cell{}
	if l.cells == nil {
		l.cells = map[int]map[[2]int]cell{}
	}
	l.cells[itab] = m
	if itab < 0 || itab >= len(l.w.sheets) || l.w.sheets[itab].kind != 0 {
		return m
	}
	raw, err := l.w.readSheet(l.ctx, l.w.sheets[itab], l.lim)
	if err != nil || raw == nil {
		return m
	}
	for _, c := range raw.cells {
		m[[2]int{c.row, c.col}] = c
	}
	return m
}

// readChart reads the records of a chart substream.
func readChart(recs []record, w *workbook) *biffChart {
	c := &biffChart{blanks: -1}
	type level struct {
		typ uint16
		obj any
	}
	var stack []level
	var last uint16
	var lastObj any
	prev := uint16(0)
	parent := 0
	defaultText := -1
	si := 0
	top := func() level {
		if len(stack) == 0 {
			return level{}
		}
		return stack[len(stack)-1]
	}
	// near returns the nearest level of one of the types.
	near := func(types ...uint16) level {
		for i := len(stack) - 1; i >= 0; i-- {
			for _, t := range types {
				if stack[i].typ == t {
					return stack[i]
				}
			}
		}
		return level{}
	}
	for i, rec := range recs {
		if i >= maxChartRecords {
			break
		}
		b := rec.data
		r := newReader(rec)
		switch rec.typ {
		case recBegin:
			if len(stack) < 64 {
				stack = append(stack, level{typ: last, obj: lastObj})
			}
			continue
		case recEnd:
			if len(stack) > 0 {
				if t, ok := stack[len(stack)-1].obj.(*chartText); ok {
					c.addText(t)
					if t.isDefault >= 0 {
						defaultText = -1
					}
				}
				stack = stack[:len(stack)-1]
			}
			continue
		}
		last, lastObj = rec.typ, nil
		t := top()
		switch rec.typ {
		case recFrame:
			switch {
			case prev == recPlotArea:
				lastObj, c.hasPlot = &c.plot, true
			case len(stack) == 1:
				lastObj = &c.area
			default:
				lastObj = &chartFmt{}
			}
		case recLineFormat, recAreaFormat, recMarkerFormat, recPieFormat, recSerFmt, recAttachedLabel:
			switch obj := t.obj.(type) {
			case *chartFmt:
				obj.read(rec.typ, b)
			case *chartAxis:
				if rec.typ == recLineFormat {
					lf := readLineFmt(b)
					switch obj.lineID {
					case 0:
						obj.line = lf
					case 1:
						obj.grid = lf
					case 2:
						obj.minorGrid = lf
					}
				}
			}
		case recChartSeries:
			if len(c.series) < maxChartSeries && len(b) >= 8 {
				s := &chartSeries{sdtX: int(le.Uint16(b)), group: -1, order: len(c.series)}
				c.series = append(c.series, s)
				lastObj = s
			}
		case recBRAI:
			if len(b) < 8 {
				break
			}
			id, rt := int(b[0]), int(b[1])
			cce := int(le.Uint16(b[6:]))
			ref := brai{rt: rt}
			if 8+cce <= len(b) {
				ref.rgce = b[8 : 8+cce]
			}
			switch obj := t.obj.(type) {
			case *chartSeries:
				if id < 4 {
					obj.refs[id] = ref
				}
			case *chartText:
				obj.ref = ref
			}
		case recSeriesText:
			if len(b) < 3 {
				break
			}
			r.skip(2)
			text := r.shortString(w.version, w.codePage)
			switch obj := t.obj.(type) {
			case *chartSeries:
				obj.name = text
			case *chartText:
				obj.text = text
			}
		case recDataFormat:
			if len(b) < 4 {
				break
			}
			xi := int(le.Uint16(b))
			f := &chartFmt{}
			if s, ok := t.obj.(*chartSeries); ok {
				switch {
				case xi == 0xFFFF:
					f = &s.format
					if len(b) >= 6 {
						s.order = int(le.Uint16(b[4:]))
					}
				case xi < maxChartPoints:
					if s.points == nil {
						s.points = map[int]*chartFmt{}
					}
					s.points[xi] = f
				}
			}
			lastObj = f
		case recSerParent:
			if s, ok := t.obj.(*chartSeries); ok && len(b) >= 2 {
				s.parent = int(le.Uint16(b))
			}
		case recSerAuxTrend:
			if s, ok := t.obj.(*chartSeries); ok && len(b) >= 28 {
				s.trend = readTrend(b)
			}
		case recDat:
			if len(b) >= 2 && c.table == nil {
				f := le.Uint16(b)
				c.table = &datFmt{horz: f&0x01 != 0, vert: f&0x02 != 0, outline: f&0x04 != 0, keys: f&0x08 != 0}
				lastObj = c.table
			}
		case recSerToCrt:
			if s, ok := t.obj.(*chartSeries); ok && len(b) >= 2 {
				s.group = int(le.Uint16(b))
			}
		case recShtProps:
			if len(b) >= 3 {
				c.visOnly = le.Uint16(b)&0x0002 != 0
				c.blanks = int(b[2])
			}
		case recDefaultText:
			if len(b) >= 2 {
				defaultText = int(le.Uint16(b))
			}
		case recText:
			if len(b) >= 26 {
				ct := &chartText{flags: le.Uint16(b[24:]), parent: parent, isDefault: defaultText}
				if _, ok := t.obj.(*chartLegend); ok {
					ct.link = -2
				}
				lastObj = ct
			}
		case recFontX:
			if len(b) < 2 {
				break
			}
			f := w.font(int(le.Uint16(b)))
			switch obj := t.obj.(type) {
			case *chartText:
				obj.font = &f
			case *chartAxis:
				obj.font = &f
			}
		case recObjectLink:
			if ct, ok := t.obj.(*chartText); ok && len(b) >= 6 {
				ct.link, ct.var1, ct.var2 = int(le.Uint16(b)), int(le.Uint16(b[2:])), int(le.Uint16(b[4:]))
			}
		case recAxisParent:
			if len(b) >= 2 {
				parent = min(int(le.Uint16(b)), 1)
			}
		case recAxis:
			if len(b) >= 2 {
				a := &chartAxis{kind: min(int(le.Uint16(b)), 2), ifmt: -1}
				if c.axes[parent][a.kind] == nil {
					c.axes[parent][a.kind] = a
				}
				lastObj = a
			}
		case recCatSerRange:
			if a, ok := t.obj.(*chartAxis); ok && len(b) >= 8 {
				a.hasCatRange = true
				a.labelSkip = int(le.Uint16(b[2:]))
				flags := le.Uint16(b[6:])
				a.between, a.maxCross, a.reverse = flags&0x01 != 0, flags&0x02 != 0, flags&0x04 != 0
			}
		case recValueRange:
			if a, ok := t.obj.(*chartAxis); ok && len(b) >= 42 {
				a.hasRange = true
				a.min, a.max, a.major, a.minor, a.cross = f64(b), f64(b[8:]), f64(b[16:]), f64(b[24:]), f64(b[32:])
				a.auto = le.Uint16(b[40:])
			}
		case recTick:
			if a, ok := t.obj.(*chartAxis); ok && len(b) >= 3 {
				a.hasTick = true
				a.tickMajor, a.tickMinor, a.labelPos = int(b[0]), int(b[1]), int(b[2])
			}
		case recAxisLine:
			if a, ok := t.obj.(*chartAxis); ok && len(b) >= 2 {
				a.lineID = int(le.Uint16(b))
				lastObj = a
			}
		case recIFmt:
			if a, ok := t.obj.(*chartAxis); ok && len(b) >= 2 {
				a.ifmt = int(le.Uint16(b))
			}
		case recChartFormat:
			if len(c.groups) < maxChartGroups && len(b) >= 20 {
				g := &chartGroup{parent: parent, varied: le.Uint16(b[16:])&0x01 != 0, icrt: int(le.Uint16(b[18:]))}
				c.groups = append(c.groups, g)
				lastObj = g
			}
		case recBar, recLine, recPie, recArea, recScatter, recRadar, recRadarArea, recSurf, recBopPop:
			if g, ok := t.obj.(*chartGroup); ok {
				g.readType(rec.typ, b)
			}
		case recCrtLine:
			if g, ok := t.obj.(*chartGroup); ok && len(b) >= 2 && le.Uint16(b) == 1 {
				g.hiLow = true
			}
		case recDropBar:
			if g, ok := t.obj.(*chartGroup); ok {
				g.upDown = true
			}
		case recLegend:
			if g, ok := t.obj.(*chartGroup); ok && len(b) >= 17 {
				g.legend = &chartLegend{pos: int(b[16]), x: int(int32(le.Uint32(b))), y: int(int32(le.Uint32(b[4:])))}
				lastObj = g.legend
			}
		case recShapeProps, recTextProps:
			ctx, xml, ok := propsStream(b)
			if !ok {
				break
			}
			c.modern = true
			l := near(recFrame, recDataFormat, recAxis, recText, recChartFormat)
			switch obj := l.obj.(type) {
			case *chartFmt:
				if rec.typ == recShapeProps {
					switch ctx {
					case 0:
						obj.spPr, obj.hasSpPr = xml, true
					case 1:
						obj.markerSpPr = xml
					}
				}
			case *chartAxis:
				switch {
				case rec.typ == recTextProps:
					obj.txPr = xml
				case ctx == 0:
					obj.spPr, obj.hasSpPr = xml, true
				case ctx == 1:
					obj.gridSpPr, obj.hasGridSpPr = xml, true
				}
			case *chartText:
				if rec.typ == recTextProps {
					obj.txPr = xml
				}
			}
		case recSIIndex:
			if len(b) >= 2 {
				si = int(le.Uint16(b))
			}
		case recNumber, recLabel, recBlank, recBoolErr:
			if len(stack) > 0 || si < 1 || si > 3 || len(b) < 6 {
				break
			}
			pt, col, xf := int(le.Uint16(b)), int(le.Uint16(b[2:])), int(le.Uint16(b[4:]))
			if col >= len(c.series) || pt >= maxChartPoints {
				break
			}
			v := chartVal{xf: xf, num: math.NaN()}
			switch rec.typ {
			case recNumber:
				if len(b) >= 14 {
					v.num, v.isNum = f64(b[6:]), true
				}
			case recLabel:
				r.skip(6)
				v.text = r.longString(w.version, w.codePage)
			}
			s := c.series[col]
			if s.cache[si] == nil {
				s.cache[si] = map[int]chartVal{}
			}
			s.cache[si][pt] = v
		}
		prev = rec.typ
	}
	return c
}

// addText keeps a text that ends: the title, the title of an axis, the
// labels of a series, or the default text of the chart.
func (c *biffChart) addText(t *chartText) {
	switch {
	case t.isDefault >= 0:
		if t.font != nil && (t.isDefault == 2 || c.defFont == nil) {
			c.defFont = t.font
		}
	case t.link == 1:
		c.title = t
	case t.link == 2 || t.link == 3:
		kind := 1
		if t.link == 3 {
			kind = 0
		}
		if a := c.axes[t.parent][kind]; a != nil {
			a.title = t
		}
	case t.link == 4:
		if len(c.labels) < maxChartPoints {
			c.labels = append(c.labels, t)
		}
	case t.link == -2:
		for _, g := range c.groups {
			if g.legend != nil && g.legend.text == nil {
				g.legend.text = t
			}
		}
	}
}

// propsStream reads a record ShapePropsStream or TextPropsStream (MS-XLS
// 2.4.262 and 2.4.311): the context of the object and its XML of
// DrawingML, a:spPr or a:txPr.
func propsStream(b []byte) (int, string, bool) {
	if len(b) < 24 {
		return 0, "", false
	}
	ctx := int(le.Uint16(b[12:]))
	n := int(le.Uint32(b[20:]))
	if n < 0 || 24+n > len(b) || n > maxChartStreamLen {
		n = len(b) - 24
	}
	xml := string(b[24 : 24+n])
	if i := strings.Index(xml, "?>"); i >= 0 {
		xml = xml[i+2:]
	}
	return ctx, strings.TrimSpace(xml), true
}

// readTrend reads a record SerAuxTrend.
func readTrend(b []byte) *trendFmt {
	t := &trendFmt{regt: int(b[0]), order: int(b[1]), eq: b[10] != 0, r2: b[11] != 0}
	// a ChartNumNillable is nil when its two last bytes are 0xFFFF
	if le.Uint16(b[8:]) != 0xFFFF {
		t.intercept, t.hasIntercept = f64(b[2:]), true
	}
	if v := f64(b[12:]); v > 0 && v < 1e9 {
		t.forward = v
	}
	if v := f64(b[20:]); v > 0 && v < 1e9 {
		t.backward = v
	}
	return t
}

// readLineFmt reads a record LineFormat.
func readLineFmt(b []byte) *lineFmt {
	if len(b) < 10 {
		return nil
	}
	flags := le.Uint16(b[8:])
	return &lineFmt{
		c:      color.NRGBA{b[0], b[1], b[2], 255},
		lns:    int(le.Uint16(b[4:])),
		we:     int(int16(le.Uint16(b[6:]))),
		auto:   flags&0x0001 != 0,
		axisOn: flags&0x0004 != 0,
	}
}

// read reads a record of the format of a frame, a series or a point.
func (f *chartFmt) read(typ uint16, b []byte) {
	switch typ {
	case recLineFormat:
		f.line = readLineFmt(b)
	case recAreaFormat:
		if len(b) >= 12 {
			f.area = &areaFmt{
				fore: color.NRGBA{b[0], b[1], b[2], 255},
				fls:  int(le.Uint16(b[8:])),
				auto: le.Uint16(b[10:])&0x0001 != 0,
			}
		}
	case recMarkerFormat:
		if len(b) >= 20 {
			flags := le.Uint16(b[10:])
			f.marker = &markerFmt{
				fore:     color.NRGBA{b[0], b[1], b[2], 255},
				back:     color.NRGBA{b[4], b[5], b[6], 255},
				imk:      int(le.Uint16(b[8:])),
				auto:     flags&0x0001 != 0,
				noFill:   flags&0x0010 != 0,
				noBorder: flags&0x0020 != 0,
				size:     float64(le.Uint32(b[16:])) / 20,
			}
		}
	case recPieFormat:
		if len(b) >= 2 {
			f.explode = min(int(le.Uint16(b)), 400)
		}
	case recSerFmt:
		if len(b) >= 2 {
			f.smooth = le.Uint16(b)&0x0001 != 0
		}
	case recAttachedLabel:
		if len(b) >= 2 {
			f.labels, f.hasLabels = le.Uint16(b), true
		}
	}
}

// readType reads the record of the type of a chart group.
func (g *chartGroup) readType(typ uint16, b []byte) {
	flag := func(at int) uint16 {
		if at+2 > len(b) {
			return 0
		}
		return le.Uint16(b[at:])
	}
	switch typ {
	case recBar:
		g.kind = "bar"
		g.overlap, g.gap = int(int16(flag(0))), int(flag(2))
		f := flag(4)
		g.horizontal, g.stacked, g.percent = f&0x01 != 0, f&0x02 != 0, f&0x04 != 0
	case recLine, recArea:
		g.kind = "line"
		if typ == recArea {
			g.kind = "area"
		}
		f := flag(0)
		g.stacked, g.percent = f&0x01 != 0, f&0x02 != 0
	case recPie, recBopPop:
		g.kind = "pie"
		if typ == recPie {
			g.start, g.donut = int(flag(0))%360, min(int(flag(2)), 90)
		}
	case recScatter:
		g.kind = "scatter"
		g.scale = int(flag(0))
		g.bubbles = flag(4)&0x01 != 0
	case recRadar, recRadarArea:
		g.kind = "radar"
		g.filled = typ == recRadarArea
	case recSurf:
		g.kind = "surface"
	}
}

// chartObject returns the object of the layout that draws the chart of the
// records recs in a frame of w by h points, or nil.
func (w *workbook) chartObject(look *chartLookup, recs []record, width, height float64, cfg ivy.Config) layout.Object {
	if len(recs) == 0 || w.version < biff8 {
		return nil
	}
	c := readChart(recs, w)
	if len(c.series) == 0 || len(c.groups) == 0 {
		return nil
	}
	xml := c.xml(w, look)
	ch, err := chart.Parse([]byte(xml), chart.Options{
		Theme:  w.chartTheme(),
		Config: cfg,
		Fonts:  fontdb.Of(cfg),
		Width:  width,
		Height: height,
	})
	if err != nil {
		cfg.Log().Debug("msxls: chart left out", "err", err)
		return nil
	}
	return ch
}

// The namespaces of a chart part.
const (
	nsChart = "http://schemas.openxmlformats.org/drawingml/2006/chart"
	nsMain  = "http://schemas.openxmlformats.org/drawingml/2006/main"
)

// xmlEscaper escapes the text of an element or an attribute.
var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// chartXML writes the part of DrawingML of a chart.
type chartXML struct {
	c    *biffChart
	w    *workbook
	look *chartLookup
	b    strings.Builder
}

// xml returns the chart as a part of DrawingML, c:chartSpace.
func (c *biffChart) xml(w *workbook, look *chartLookup) string {
	x := &chartXML{c: c, w: w, look: look}
	b := &x.b
	b.WriteString(`<c:chartSpace xmlns:c="` + nsChart + `" xmlns:a="` + nsMain + `"><c:chart>`)
	x.title()
	b.WriteString("<c:plotArea>")
	used := [2]bool{}
	for gi, g := range c.groups {
		if g.kind == "" {
			continue
		}
		used[g.parent] = true
		x.group(gi, g)
	}
	for p := range 2 {
		if used[p] {
			x.axes(p)
		}
	}
	if t := c.table; t != nil {
		b.WriteString(`<c:dTable><c:showHorzBorder val="` + boolStr(t.horz) + `"/><c:showVertBorder val="` + boolStr(t.vert) +
			`"/><c:showOutline val="` + boolStr(t.outline) + `"/><c:showKeys val="` + boolStr(t.keys) + `"/></c:dTable>`)
	}
	if c.hasPlot {
		b.WriteString(x.spPr(&c.plot, "plot", 0))
	}
	b.WriteString("</c:plotArea>")
	x.legend()
	if c.visOnly {
		b.WriteString(`<c:plotVisOnly val="1"/>`)
	}
	switch c.blanks {
	case 0:
		b.WriteString(`<c:dispBlanksAs val="gap"/>`)
	case 1:
		b.WriteString(`<c:dispBlanksAs val="zero"/>`)
	case 2:
		b.WriteString(`<c:dispBlanksAs val="span"/>`)
	}
	b.WriteString("</c:chart>")
	if sp := x.spPr(&c.area, "area", 0); sp != "" {
		b.WriteString(sp)
	} else {
		// a chart of a sheet without its own format: a white chart area
		// with a border
		b.WriteString("<c:printSettings/>")
	}
	if f := c.defFont; f != nil {
		b.WriteString("<c:txPr><a:bodyPr/><a:p><a:pPr>")
		x.runProps("a:defRPr", f, false)
		b.WriteString("</a:pPr></a:p></c:txPr>")
	}
	b.WriteString("</c:chartSpace>")
	return b.String()
}

// title writes the title of the chart, or says that it has none.
func (x *chartXML) title() {
	t := x.c.title
	if t == nil || t.flags&0x0040 != 0 {
		x.b.WriteString(`<c:autoTitleDeleted val="1"/>`)
		return
	}
	x.b.WriteString("<c:title>")
	x.richText(t)
	x.b.WriteString(`<c:overlay val="0"/></c:title><c:autoTitleDeleted val="0"/>`)
}

// richText writes the text of a title as c:tx, or only its style when the
// title takes the automatic text.
func (x *chartXML) richText(t *chartText) {
	b := &x.b
	text := t.text
	if text == "" && t.ref.rt == 2 {
		if vals := x.refValues(t.ref.rgce, 1); len(vals) > 0 {
			text = x.valText(vals[0])
		}
	}
	if text == "" {
		if t.font != nil {
			b.WriteString("<c:txPr><a:bodyPr/><a:p><a:pPr>")
			x.runProps("a:defRPr", t.font, true)
			b.WriteString("</a:pPr></a:p></c:txPr>")
		}
		return
	}
	b.WriteString("<c:tx><c:rich><a:bodyPr/>")
	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		b.WriteString("<a:p><a:pPr>")
		x.runProps("a:defRPr", t.font, true)
		b.WriteString("</a:pPr><a:r>")
		x.runProps("a:rPr", t.font, true)
		b.WriteString("<a:t>" + xmlEscaper.Replace(line) + "</a:t></a:r></a:p>")
	}
	b.WriteString("</c:rich></c:tx>")
}

// runProps writes the properties of a run of the font f. bold writes the
// weight even when it is normal.
func (x *chartXML) runProps(name string, f *sheet.Font, bold bool) {
	b := &x.b
	if f == nil {
		b.WriteString("<" + name + "/>")
		return
	}
	b.WriteString("<" + name)
	if f.Size > 0 {
		b.WriteString(` sz="` + strconv.Itoa(int(math.Round(f.Size*100))) + `"`)
	}
	switch {
	case f.Bold:
		b.WriteString(` b="1"`)
	case bold:
		b.WriteString(` b="0"`)
	}
	b.WriteString(">")
	if f.Color.A != 0 {
		b.WriteString(solid(f.Color))
	}
	if f.Family != "" {
		b.WriteString(`<a:latin typeface="` + xmlEscaper.Replace(f.Family) + `"/>`)
	}
	b.WriteString("</" + name + ">")
}

// solid returns a solid fill of the color c.
func solid(c color.NRGBA) string {
	return `<a:solidFill><a:srgbClr val="` + hexColor(c) + `"/></a:solidFill>`
}

// hexColor formats a color as RRGGBB.
func hexColor(c color.NRGBA) string {
	const digits = "0123456789ABCDEF"
	out := []byte{
		digits[c.R>>4], digits[c.R&15], digits[c.G>>4], digits[c.G&15], digits[c.B>>4], digits[c.B&15],
	}
	return string(out)
}

// The default formats of a chart of Excel 97: the colors of the palette
// for the series, and the gray of the plot area.
const (
	paletteFills = 24
	paletteLines = 32
)

// spPr returns the c:spPr of a format. role is area or plot for the chart
// area and the plot area, fill or line for a series that draws as an area
// or as a line, and point for the slice of a pie. index is the number of
// the series or of the point, for the automatic colors of Excel 97.
func (x *chartXML) spPr(f *chartFmt, role string, index int) string {
	if f.hasSpPr {
		return asSpPr(f.spPr, "c:spPr")
	}
	if x.c.modern {
		return ""
	}
	var b strings.Builder
	pal := x.w.palette
	switch a := f.area; {
	case role == "line":
	case a == nil || a.auto:
		switch role {
		case "plot":
			b.WriteString(solid(pal[22]))
		case "area":
			b.WriteString(solid(color.NRGBA{255, 255, 255, 255}))
		case "fill", "point":
			b.WriteString(solid(pal[paletteFills+index%8]))
		}
	case a.fls == 0:
		b.WriteString("<a:noFill/>")
	default:
		b.WriteString(solid(a.fore))
	}
	switch l := f.line; {
	case l == nil || l.auto:
		switch role {
		case "line":
			b.WriteString(`<a:ln w="28575">` + solid(pal[paletteLines+index%8]) + "</a:ln>")
		case "plot":
			b.WriteString(`<a:ln w="9525">` + solid(pal[23]) + "</a:ln>")
		default:
			b.WriteString(`<a:ln w="9525">` + solid(color.NRGBA{0, 0, 0, 255}) + "</a:ln>")
		}
	case l.lns == 5:
		b.WriteString("<a:ln><a:noFill/></a:ln>")
	default:
		b.WriteString(lineXML(l))
	}
	if b.Len() == 0 {
		return ""
	}
	return "<c:spPr>" + b.String() + "</c:spPr>"
}

// lineXML returns the a:ln of a record LineFormat.
func lineXML(l *lineFmt) string {
	width := map[int]int{-1: 3175, 0: 12700, 1: 25400, 2: 38100}[min(max(l.we, -1), 2)]
	dash := [...]string{"", "dash", "sysDot", "dashDot", "lgDashDotDot", "", "", "", ""}[min(max(l.lns, 0), 8)]
	s := `<a:ln w="` + strconv.Itoa(width) + `">` + solid(l.c)
	if dash != "" {
		s += `<a:prstDash val="` + dash + `"/>`
	}
	return s + "</a:ln>"
}

// asSpPr renames the root of the XML of a ShapePropsStream or of a
// TextPropsStream, a:spPr or a:txPr, to name, and returns "" for an empty
// root.
func asSpPr(xml, name string) string {
	start := strings.Index(xml, "<")
	if start < 0 {
		return ""
	}
	end := strings.IndexAny(xml[start:], " />")
	if end < 0 {
		return ""
	}
	root := xml[start+1 : start+end]
	if strings.HasSuffix(strings.TrimSpace(xml), "/>") && strings.Count(xml, "<") == 1 {
		return ""
	}
	body := xml[start+end:]
	closing := "</" + root + ">"
	if !strings.HasSuffix(body, closing) {
		return ""
	}
	return "<" + name + body[:len(body)-len(closing)] + "</" + name + ">"
}

// group writes a chart group with its series.
func (x *chartXML) group(gi int, g *chartGroup) {
	b := &x.b
	var elem string
	switch g.kind {
	case "bar":
		elem = "c:barChart"
	case "line":
		elem = "c:lineChart"
	case "area":
		elem = "c:areaChart"
	case "pie":
		elem = "c:pieChart"
		if g.donut > 0 {
			elem = "c:doughnutChart"
		}
	case "scatter":
		elem = "c:scatterChart"
		if g.bubbles {
			elem = "c:bubbleChart"
		}
	case "radar":
		elem = "c:radarChart"
	default:
		elem = "c:surfaceChart"
	}
	b.WriteString("<" + elem + ">")
	grouping := "standard"
	switch {
	case g.percent:
		grouping = "percentStacked"
	case g.stacked:
		grouping = "stacked"
	case g.kind == "bar":
		grouping = "clustered"
	}
	switch g.kind {
	case "bar":
		dir := "col"
		if g.horizontal {
			dir = "bar"
		}
		b.WriteString(`<c:barDir val="` + dir + `"/><c:grouping val="` + grouping + `"/>`)
	case "line", "area":
		b.WriteString(`<c:grouping val="` + grouping + `"/>`)
	case "scatter":
		if !g.bubbles {
			b.WriteString(`<c:scatterStyle val="lineMarker"/>`)
		}
	case "radar":
		style := "marker"
		if g.filled {
			style = "filled"
		}
		b.WriteString(`<c:radarStyle val="` + style + `"/>`)
	}
	b.WriteString(`<c:varyColors val="` + boolStr(g.varied) + `"/>`)
	var list []*chartSeries
	for _, s := range x.c.series {
		// a series with a parent is a trendline or error bars
		if x.groupOf(s) == gi && s.parent == 0 {
			list = append(list, s)
		}
	}
	slices.SortStableFunc(list, func(a, b *chartSeries) int { return a.order - b.order })
	for _, s := range list {
		x.series(s.order, s, g)
	}
	switch g.kind {
	case "bar":
		b.WriteString(`<c:gapWidth val="` + strconv.Itoa(min(g.gap, 500)) + `"/>`)
		overlap := max(-100, min(100, g.overlap))
		if g.stacked || g.percent {
			overlap = 100
		}
		b.WriteString(`<c:overlap val="` + strconv.Itoa(overlap) + `"/>`)
	case "line":
		if g.hiLow {
			b.WriteString("<c:hiLowLines/>")
		}
		if g.upDown {
			b.WriteString("<c:upDownBars/>")
		}
		b.WriteString(`<c:marker val="1"/>`)
	case "pie":
		b.WriteString(`<c:firstSliceAng val="` + strconv.Itoa(g.start) + `"/>`)
		if g.donut > 0 {
			b.WriteString(`<c:holeSize val="` + strconv.Itoa(max(10, g.donut)) + `"/>`)
		}
	case "scatter":
		if g.bubbles && g.scale > 0 {
			b.WriteString(`<c:bubbleScale val="` + strconv.Itoa(min(g.scale, 300)) + `"/>`)
		}
	}
	if g.kind != "pie" {
		id := 100 * (g.parent + 1)
		b.WriteString(`<c:axId val="` + strconv.Itoa(id+1) + `"/><c:axId val="` + strconv.Itoa(id+2) + `"/>`)
	}
	b.WriteString("</" + elem + ">")
}

// groupOf returns the number of the chart group of a series.
func (x *chartXML) groupOf(s *chartSeries) int {
	for i, g := range x.c.groups {
		if g.icrt == s.group {
			return i
		}
	}
	if s.group >= 0 && s.group < len(x.c.groups) {
		return s.group
	}
	return 0
}

// boolStr returns "1" or "0".
func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// markerSymbols are the symbols of DrawingML by imk of MarkerFormat.
var markerSymbols = [...]string{"none", "square", "diamond", "triangle", "x", "star", "dash", "dash", "circle", "plus"}

// series writes a series of a chart group. si is its order, which picks
// its automatic color.
func (x *chartXML) series(si int, s *chartSeries, g *chartGroup) {
	b := &x.b
	b.WriteString(`<c:ser><c:idx val="` + strconv.Itoa(si) + `"/><c:order val="` + strconv.Itoa(si) + `"/>`)
	name := s.name
	if name == "" && s.refs[0].rt == 2 {
		if vals := x.refValues(s.refs[0].rgce, 1); len(vals) > 0 {
			name = x.valText(vals[0])
		}
	}
	if name != "" {
		b.WriteString("<c:tx><c:v>" + xmlEscaper.Replace(name) + "</c:v></c:tx>")
	}
	lined := g.kind == "line" || g.kind == "scatter" || (g.kind == "radar" && !g.filled)
	role := "fill"
	if lined {
		role = "line"
	}
	if g.kind == "pie" && g.varied {
		role = "pie"
	}
	if role != "pie" {
		b.WriteString(x.spPr(&s.format, role, si))
	}
	if s.format.explode > 0 && g.kind == "pie" {
		b.WriteString(`<c:explosion val="` + strconv.Itoa(s.format.explode) + `"/>`)
	}
	if lined {
		x.marker(&s.format, si)
	}
	if s.format.smooth && lined {
		b.WriteString(`<c:smooth val="1"/>`)
	}
	vals := x.data(s, 1)
	cats := x.data(s, 2)
	n := len(vals)
	for i := range n {
		f := s.points[i]
		vary := g.kind == "pie" && g.varied
		if f == nil && (!vary || x.c.modern) {
			continue
		}
		if f == nil {
			f = &chartFmt{}
		}
		sp := ""
		switch {
		case vary:
			sp = x.spPr(f, "point", i)
		case f.hasSpPr || f.area != nil || f.line != nil:
			sp = x.spPr(f, role, si)
		}
		if sp == "" && f.explode == 0 {
			continue
		}
		b.WriteString(`<c:dPt><c:idx val="` + strconv.Itoa(i) + `"/>`)
		if f.explode > 0 {
			b.WriteString(`<c:explosion val="` + strconv.Itoa(f.explode) + `"/>`)
		}
		b.WriteString(sp + "</c:dPt>")
	}
	x.dataLabels(slices.Index(x.c.series, s), s)
	x.trendlines(s)
	if g.kind == "scatter" {
		if s.sdtX == 1 && len(cats) > 0 {
			x.values("c:xVal", cats, true)
		}
		x.values("c:yVal", vals, true)
		if g.bubbles {
			x.values("c:bubbleSize", x.data(s, 3), true)
		}
	} else {
		if len(cats) > 0 {
			x.values("c:cat", cats, false)
		}
		x.values("c:val", vals, true)
	}
	b.WriteString("</c:ser>")
}

// trendKinds are the types of trendlines of DrawingML by regt of
// SerAuxTrend, after the polynomial.
var trendKinds = [...]string{"poly", "exp", "log", "power", "movingAvg"}

// trendlines writes the trendlines of the series s: the series with a
// record SerAuxTrend whose record SerParent names s.
func (x *chartXML) trendlines(s *chartSeries) {
	b := &x.b
	parent := slices.Index(x.c.series, s) + 1
	n := 0
	for ti, t := range x.c.series {
		tr := t.trend
		if tr == nil || t.parent != parent || tr.regt >= len(trendKinds) || n >= 4 {
			continue
		}
		n++
		b.WriteString("<c:trendline>")
		if t.name != "" {
			b.WriteString("<c:name>" + xmlEscaper.Replace(t.name) + "</c:name>")
		}
		b.WriteString(x.trendSpPr(&t.format))
		kind := trendKinds[tr.regt]
		switch {
		case kind == "poly" && tr.order < 2:
			b.WriteString(`<c:trendlineType val="linear"/>`)
		case kind == "poly":
			b.WriteString(`<c:trendlineType val="poly"/><c:order val="` + strconv.Itoa(min(tr.order, 6)) + `"/>`)
		case kind == "movingAvg":
			b.WriteString(`<c:trendlineType val="movingAvg"/><c:period val="` + strconv.Itoa(max(tr.order, 2)) + `"/>`)
		default:
			b.WriteString(`<c:trendlineType val="` + kind + `"/>`)
		}
		if tr.forward > 0 {
			b.WriteString(`<c:forward val="` + num(tr.forward) + `"/>`)
		}
		if tr.backward > 0 {
			b.WriteString(`<c:backward val="` + num(tr.backward) + `"/>`)
		}
		if tr.hasIntercept {
			b.WriteString(`<c:intercept val="` + num(tr.intercept) + `"/>`)
		}
		b.WriteString(`<c:dispRSqr val="` + boolStr(tr.r2) + `"/><c:dispEq val="` + boolStr(tr.eq) + `"/>`)
		// The label of the equation is a record Text of the trendline.
		for _, l := range x.c.labels {
			if l.var1 == ti && l.var2 == 0xFFFF && l.font != nil && (tr.eq || tr.r2) {
				b.WriteString("<c:trendlineLbl><c:txPr><a:bodyPr/><a:p><a:pPr>")
				x.runProps("a:defRPr", l.font, false)
				b.WriteString("</a:pPr></a:p></c:txPr></c:trendlineLbl>")
				break
			}
		}
		b.WriteString("</c:trendline>")
	}
}

// trendSpPr returns the c:spPr of the line of a trendline. Excel 97 draws
// an automatic trendline as a thin black line.
func (x *chartXML) trendSpPr(f *chartFmt) string {
	switch l := f.line; {
	case f.hasSpPr:
		return asSpPr(f.spPr, "c:spPr")
	case x.c.modern:
		return ""
	case l == nil || l.auto:
		return `<c:spPr><a:ln w="9525">` + solid(color.NRGBA{0, 0, 0, 255}) + "</a:ln></c:spPr>"
	case l.lns == 5:
		return "<c:spPr><a:ln><a:noFill/></a:ln></c:spPr>"
	}
	return "<c:spPr>" + lineXML(f.line) + "</c:spPr>"
}

// marker writes the marker of a series of a line, a scatter or a radar
// chart.
func (x *chartXML) marker(f *chartFmt, si int) {
	b := &x.b
	m := f.marker
	if m == nil && f.markerSpPr == "" {
		return
	}
	b.WriteString("<c:marker>")
	if m != nil && (!m.auto || m.imk == 0) {
		if m.imk >= 0 && m.imk < len(markerSymbols) {
			b.WriteString(`<c:symbol val="` + markerSymbols[m.imk] + `"/>`)
		}
		if m.size >= 2 && m.size <= 72 {
			b.WriteString(`<c:size val="` + strconv.Itoa(int(math.Round(m.size))) + `"/>`)
		}
	}
	switch {
	case f.markerSpPr != "":
		b.WriteString(asSpPr(f.markerSpPr, "c:spPr"))
	case m != nil && !m.auto && !x.c.modern:
		b.WriteString("<c:spPr>")
		if m.noFill {
			b.WriteString("<a:noFill/>")
		} else {
			b.WriteString(solid(m.back))
		}
		if m.noBorder {
			b.WriteString("<a:ln><a:noFill/></a:ln>")
		} else {
			b.WriteString(`<a:ln w="9525">` + solid(m.fore) + "</a:ln>")
		}
		b.WriteString("</c:spPr>")
	case m != nil && !x.c.modern:
		c := x.w.palette[paletteLines+si%8]
		b.WriteString("<c:spPr>" + solid(c) + `<a:ln w="9525">` + solid(c) + "</a:ln></c:spPr>")
	}
	b.WriteString("</c:marker>")
}

// dataLabels writes the labels of a series: the records Text that link to
// it, and the records AttachedLabel of its formats.
func (x *chartXML) dataLabels(si int, s *chartSeries) {
	var all *chartText
	points := map[int]*chartText{}
	for _, t := range x.c.labels {
		if t.var1 != si {
			continue
		}
		if t.var2 == 0xFFFF {
			all = t
		} else if len(points) < maxChartPoints {
			points[t.var2] = t
		}
	}
	flags, has := uint16(0), false
	if all != nil {
		flags, has = labelFlags(all.flags), all.flags&0x0040 == 0
	}
	if s.format.hasLabels {
		flags, has = s.format.labels, true
	}
	if !has && len(points) == 0 {
		return
	}
	b := &x.b
	b.WriteString("<c:dLbls>")
	for i, t := range points {
		b.WriteString(`<c:dLbl><c:idx val="` + strconv.Itoa(i) + `"/>`)
		if t.flags&0x0040 != 0 {
			b.WriteString(`<c:delete val="1"/></c:dLbl>`)
			continue
		}
		labelParts(b, labelFlags(t.flags))
		b.WriteString("</c:dLbl>")
	}
	if all != nil && all.font != nil {
		b.WriteString("<c:txPr><a:bodyPr/><a:p><a:pPr>")
		x.runProps("a:defRPr", all.font, false)
		b.WriteString("</a:pPr></a:p></c:txPr>")
	}
	labelParts(b, flags)
	b.WriteString("</c:dLbls>")
}

// labelFlags converts the flags of a record Text of a label to the flags of
// a record AttachedLabel.
func labelFlags(f uint16) uint16 {
	var out uint16
	if f&0x0004 != 0 {
		out |= 0x0001
	}
	if f&0x1000 != 0 {
		out |= 0x0002
	}
	if f&0x0800 != 0 {
		out |= 0x0004
	}
	if f&0x4000 != 0 {
		out |= 0x0010
	}
	if f&0x2000 != 0 {
		out |= 0x0020
	}
	return out
}

// labelParts writes what a label shows, from the flags of a record
// AttachedLabel.
func labelParts(b *strings.Builder, f uint16) {
	b.WriteString(`<c:showLegendKey val="0"/>`)
	b.WriteString(`<c:showVal val="` + boolStr(f&0x0001 != 0) + `"/>`)
	b.WriteString(`<c:showCatName val="` + boolStr(f&0x0014 != 0) + `"/>`)
	b.WriteString(`<c:showSerName val="` + boolStr(f&0x0040 != 0) + `"/>`)
	b.WriteString(`<c:showPercent val="` + boolStr(f&0x0006 != 0) + `"/>`)
}

// values writes the data of a series as a literal, c:numLit or c:strLit.
func (x *chartXML) values(name string, vals []chartVal, numeric bool) {
	b := &x.b
	isNum := numeric
	if !numeric {
		isNum = true
		for _, v := range vals {
			if !v.isNum && v.text != "" {
				isNum = false
				break
			}
		}
	}
	b.WriteString("<" + name + ">")
	if isNum {
		b.WriteString("<c:numLit>")
		code := "General"
		for _, v := range vals {
			if v.isNum {
				code = x.w.numberFormatOf(v.xf)
				break
			}
		}
		b.WriteString("<c:formatCode>" + xmlEscaper.Replace(code) + "</c:formatCode>")
	} else {
		b.WriteString("<c:strLit>")
	}
	b.WriteString(`<c:ptCount val="` + strconv.Itoa(len(vals)) + `"/>`)
	for i, v := range vals {
		var s string
		switch {
		case isNum && v.isNum && !math.IsNaN(v.num) && !math.IsInf(v.num, 0):
			s = strconv.FormatFloat(v.num, 'g', -1, 64)
		case isNum:
			continue
		default:
			s = x.valText(v)
		}
		b.WriteString(`<c:pt idx="` + strconv.Itoa(i) + `"><c:v>` + xmlEscaper.Replace(s) + "</c:v></c:pt>")
	}
	if isNum {
		b.WriteString("</c:numLit>")
	} else {
		b.WriteString("</c:strLit>")
	}
	b.WriteString("</" + name + ">")
}

// valText returns the text of a value, a number in its number format.
func (x *chartXML) valText(v chartVal) string {
	if !v.isNum {
		return v.text
	}
	if math.IsNaN(v.num) {
		return ""
	}
	return numfmt.Number(x.w.numberFormatOf(v.xf), v.num, x.w.date1904).Text
}

// data returns the values of a series: 1 for its values, 2 for its
// categories and 3 for its bubble sizes. The cache of the chart comes
// first, and then the cells of the formula of the series.
func (x *chartXML) data(s *chartSeries, id int) []chartVal {
	if m := s.cache[id]; len(m) > 0 {
		n := 0
		for i := range m {
			n = max(n, i+1)
		}
		out := make([]chartVal, n)
		for i := range out {
			out[i] = chartVal{num: math.NaN()}
		}
		for i, v := range m {
			out[i] = v
		}
		return out
	}
	if s.refs[id].rt != 2 {
		return nil
	}
	return x.refValues(s.refs[id].rgce, maxChartPoints)
}

// refValues returns the values of the cells of a formula of areas, at most
// limit, by rows and then by columns.
func (x *chartXML) refValues(rgce []byte, limit int) []chartVal {
	if x.look == nil {
		return nil
	}
	var out []chartVal
	for _, a := range x.w.chartAreas(rgce) {
		cells := x.look.sheetCells(a.itab)
		for r := a.r.Row0; r <= a.r.Row1; r++ {
			for col := a.r.Col0; col <= a.r.Col1; col++ {
				if len(out) >= limit {
					return out
				}
				c, ok := cells[[2]int{r, col}]
				v := chartVal{num: math.NaN(), xf: c.xf}
				switch {
				case !ok:
				case c.kind == cellNumber:
					v.num, v.isNum = c.num, true
				case c.kind == cellText:
					v.text = c.text
				case c.kind == cellBool:
					v.text = "FALSE"
					if c.num != 0 {
						v.text = "TRUE"
					}
				}
				out = append(out, v)
			}
		}
	}
	return out
}

// sheetArea is an area of cells of a sheet, by its index in the workbook.
type sheetArea struct {
	itab int
	r    sheet.Range
}

// chartAreas returns the areas of the cells of a formula of a chart: areas
// and references of 3D, joined by unions.
func (w *workbook) chartAreas(rgce []byte) []sheetArea {
	var out []sheetArea
	itab := func(ixti int) int {
		if ixti < 0 || ixti >= len(w.xti) {
			return -1
		}
		return w.xti[ixti]
	}
	for i := 0; i < len(rgce) && len(out) < maxPrintAreas; {
		ptg := rgce[i]
		base := ptg
		if ptg >= 0x20 && ptg < 0x80 {
			base = ptg&0x1F | 0x20
		}
		switch {
		case ptg == ptgUnion, ptg == ptgRange, ptg == ptgParen:
			i++
		case base == ptgMemFunc:
			i += 3
		case base == ptgArea3d && i+11 <= len(rgce):
			v := rgce[i+1:]
			r0, r1 := int(le.Uint16(v[2:])), int(le.Uint16(v[4:]))
			c0, c1 := int(le.Uint16(v[6:])&0x3FFF), int(le.Uint16(v[8:])&0x3FFF)
			if r0 <= r1 && c0 <= c1 && c1 <= maxCol {
				out = append(out, sheetArea{itab(int(le.Uint16(v))), sheet.Range{Row0: r0, Col0: c0, Row1: r1, Col1: c1}})
			}
			i += 11
		case base == ptgRef3d && i+7 <= len(rgce):
			v := rgce[i+1:]
			r, c := int(le.Uint16(v[2:])), int(le.Uint16(v[4:])&0x3FFF)
			if c <= maxCol {
				out = append(out, sheetArea{itab(int(le.Uint16(v))), sheet.Range{Row0: r, Col0: c, Row1: r, Col1: c}})
			}
			i += 7
		default:
			return out
		}
	}
	return out
}

// axes writes the axes of the axis group p.
func (x *chartXML) axes(p int) {
	horizontal, scatter := false, false
	for _, g := range x.c.groups {
		if g.parent != p {
			continue
		}
		horizontal = horizontal || (g.kind == "bar" && g.horizontal)
		scatter = scatter || g.kind == "scatter"
	}
	id := 100 * (p + 1)
	catPos, valPos := "b", "l"
	if horizontal {
		catPos, valPos = "l", "b"
	}
	if p == 1 {
		catPos, valPos = map[string]string{"b": "t", "l": "r"}[catPos], map[string]string{"b": "t", "l": "r"}[valPos]
	}
	cat, val := x.c.axes[p][0], x.c.axes[p][1]
	catElem := "c:catAx"
	if scatter {
		catElem = "c:valAx"
	}
	x.axis(catElem, cat, id+1, id+2, catPos, val, scatter)
	x.axis("c:valAx", val, id+2, id+1, valPos, cat, scatter)
}

// axis writes one axis. other is the axis that it crosses.
func (x *chartXML) axis(elem string, a *chartAxis, id, cross int, pos string, other *chartAxis, scatter bool) {
	b := &x.b
	b.WriteString("<" + elem + `><c:axId val="` + strconv.Itoa(id) + `"/><c:scaling>`)
	orientation := "minMax"
	if a != nil && (a.reverse || (a.hasRange && a.auto&0x0040 != 0)) {
		orientation = "maxMin"
	}
	b.WriteString(`<c:orientation val="` + orientation + `"/>`)
	if a != nil && a.hasRange {
		if a.auto&0x0020 != 0 {
			b.WriteString(`<c:logBase val="10"/>`)
		}
		if a.auto&0x0002 == 0 && !math.IsNaN(a.max) {
			b.WriteString(`<c:max val="` + num(a.max) + `"/>`)
		}
		if a.auto&0x0001 == 0 && !math.IsNaN(a.min) {
			b.WriteString(`<c:min val="` + num(a.min) + `"/>`)
		}
	}
	b.WriteString("</c:scaling>")
	if a == nil {
		b.WriteString(`<c:delete val="1"/><c:axPos val="` + pos + `"/><c:crossAx val="` + strconv.Itoa(cross) + `"/></` + elem + ">")
		return
	}
	b.WriteString(`<c:delete val="0"/><c:axPos val="` + pos + `"/>`)
	if a.grid != nil || a.hasGridSpPr {
		b.WriteString("<c:majorGridlines>")
		switch {
		case a.hasGridSpPr:
			b.WriteString(asSpPr(a.gridSpPr, "c:spPr"))
		case !x.c.modern && !a.grid.auto && a.grid.lns != 5:
			b.WriteString("<c:spPr>" + lineXML(a.grid) + "</c:spPr>")
		}
		b.WriteString("</c:majorGridlines>")
	}
	if a.minorGrid != nil {
		b.WriteString("<c:minorGridlines/>")
	}
	if t := a.title; t != nil && t.flags&0x0040 == 0 {
		b.WriteString("<c:title>")
		x.richText(t)
		b.WriteString(`<c:overlay val="0"/></c:title>`)
	}
	if a.ifmt > 0 {
		b.WriteString(`<c:numFmt formatCode="` + xmlEscaper.Replace(x.w.numberFormat(a.ifmt)) + `" sourceLinked="0"/>`)
	}
	ticks := [...]string{"none", "in", "out", "cross"}
	labels := [...]string{"none", "low", "high", "nextTo"}
	if a.hasTick {
		b.WriteString(`<c:majorTickMark val="` + ticks[min(a.tickMajor, 3)] + `"/>`)
		b.WriteString(`<c:minorTickMark val="` + ticks[min(a.tickMinor, 3)] + `"/>`)
		b.WriteString(`<c:tickLblPos val="` + labels[min(a.labelPos, 3)] + `"/>`)
	}
	switch {
	case a.hasSpPr:
		b.WriteString(asSpPr(a.spPr, "c:spPr"))
	case x.c.modern:
	case a.line != nil && (a.line.lns == 5 || (!a.line.auto && !a.line.axisOn)):
		b.WriteString("<c:spPr><a:ln><a:noFill/></a:ln></c:spPr>")
	case a.line != nil && !a.line.auto:
		b.WriteString("<c:spPr>" + lineXML(a.line) + "</c:spPr>")
	}
	switch {
	case a.txPr != "":
		b.WriteString(asSpPr(a.txPr, "c:txPr"))
	case a.font != nil:
		b.WriteString("<c:txPr><a:bodyPr/><a:p><a:pPr>")
		x.runProps("a:defRPr", a.font, false)
		b.WriteString("</a:pPr></a:p></c:txPr>")
	}
	b.WriteString(`<c:crossAx val="` + strconv.Itoa(cross) + `"/>`)
	// where this axis crosses the other: the other axis gives it
	switch {
	case other == nil:
		b.WriteString(`<c:crosses val="autoZero"/>`)
	case other.hasRange && other.auto&0x0080 != 0, other.hasCatRange && other.maxCross:
		b.WriteString(`<c:crosses val="max"/>`)
	case other.hasRange && other.auto&0x0010 == 0 && !math.IsNaN(other.cross):
		b.WriteString(`<c:crossesAt val="` + num(other.cross) + `"/>`)
	default:
		b.WriteString(`<c:crosses val="autoZero"/>`)
	}
	if elem == "c:valAx" {
		between := "between"
		if scatter || (other != nil && other.hasCatRange && !other.between) {
			between = "midCat"
		}
		b.WriteString(`<c:crossBetween val="` + between + `"/>`)
		if a.hasRange && a.auto&0x0004 == 0 && a.major > 0 {
			b.WriteString(`<c:majorUnit val="` + num(a.major) + `"/>`)
		}
		if a.hasRange && a.auto&0x0008 == 0 && a.minor > 0 {
			b.WriteString(`<c:minorUnit val="` + num(a.minor) + `"/>`)
		}
	} else if a.labelSkip > 1 {
		b.WriteString(`<c:tickLblSkip val="` + strconv.Itoa(a.labelSkip) + `"/>`)
	}
	b.WriteString("</" + elem + ">")
}

// num formats a number of the XML.
func num(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// legend writes the legend of the chart, which the first chart group with
// a legend gives.
func (x *chartXML) legend() {
	for _, g := range x.c.groups {
		l := g.legend
		if l == nil {
			continue
		}
		pos := map[int]string{0: "b", 1: "tr", 2: "t", 3: "r", 4: "l"}[l.pos]
		if pos == "" {
			// a legend that is not docked takes the side that it is
			// nearest to
			switch {
			case l.y > 3000:
				pos = "b"
			case l.y < 1000 && l.x > 1000 && l.x < 3000:
				pos = "t"
			case l.x < 1000:
				pos = "l"
			default:
				pos = "r"
			}
		}
		x.b.WriteString(`<c:legend><c:legendPos val="` + pos + `"/><c:overlay val="0"/>`)
		if t := l.text; t != nil {
			switch {
			case t.txPr != "":
				x.b.WriteString(asSpPr(t.txPr, "c:txPr"))
			case t.font != nil:
				x.b.WriteString("<c:txPr><a:bodyPr/><a:p><a:pPr>")
				x.runProps("a:defRPr", t.font, false)
				x.b.WriteString("</a:pPr></a:p></c:txPr>")
			}
		}
		x.b.WriteString("</c:legend>")
		return
	}
}
