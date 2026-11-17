package vsdx

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
)

// themed is the value of a cell that the theme gives ([MS-VSDX] 2.2.5.4.3).
const themed = "Themed"

// quickStyle holds the cells of the section Theme Properties of a shape,
// which pick the colors and the styles of the theme.
type quickStyle struct {
	lineColor, fillColor, shadowColor, fontColor              int
	lineMatrix, fillMatrix, effectsMatrix, fontMatrix, styleT int
	// connector reports a shape that takes the styles of the connectors.
	connector bool
}

// quick returns the theme properties of the shape.
func (b *builder) quick(s *shape) quickStyle {
	n := func(name string) int {
		return int(b.num(s, name, 100))
	}
	q := quickStyle{
		lineColor:     n("QuickStyleLineColor"),
		fillColor:     n("QuickStyleFillColor"),
		shadowColor:   n("QuickStyleShadowColor"),
		fontColor:     n("QuickStyleFontColor"),
		lineMatrix:    n("QuickStyleLineMatrix"),
		fillMatrix:    n("QuickStyleFillMatrix"),
		effectsMatrix: n("QuickStyleEffectsMatrix"),
		fontMatrix:    n("QuickStyleFontMatrix"),
		styleT:        int(b.num(s, "QuickStyleType", 0)),
	}
	obj := int(b.num(s, "ObjType", 0))
	_, oneD := b.value(s, "BeginX")
	q.connector = obj&2 != 0 || oneD && obj&1 == 0
	return q
}

// themeColor returns the color of the theme with the index of a cell such
// as QuickStyleLineColor.
func (b *builder) themeColor(idx int) drawingml.RGBA {
	return b.theme.color(idx, b.variantColor)
}

// color reads the value of a cell of a color: "#RRGGBB", an index of the
// color table, or RGB(r, g, b). It returns false for "Themed" and for
// any other value.
func (b *builder) color(v string) (drawingml.RGBA, bool) {
	v = strings.TrimSpace(v)
	if v == "" || v == themed {
		return drawingml.RGBA{}, false
	}
	if strings.HasPrefix(v, "#") {
		c, ok := drawingml.ParseHex(v)
		return drawingml.RGBAFrom(c), ok
	}
	if args, ok := funcArgs(v, "RGB"); ok && len(args) == 3 {
		return drawingml.RGBA{R: args[0] / 255, G: args[1] / 255, B: args[2] / 255, A: 1}, true
	}
	if args, ok := funcArgs(v, "RGBA"); ok && len(args) == 4 {
		return drawingml.RGBA{R: args[0] / 255, G: args[1] / 255, B: args[2] / 255, A: 1 - args[3]/255}, true
	}
	if i, err := strconv.Atoi(v); err == nil {
		if c, ok := b.dr.colors[i]; ok {
			return c, true
		}
		return drawingml.Black, true
	}
	return drawingml.RGBA{}, false
}

// cellColor returns the color of the cell of the shape with the name, with
// the transparency of the cell trans. A themed color comes from theme,
// which returns false when the theme does not give it.
func (b *builder) cellColor(s *shape, name, trans string, theme func() (drawingml.RGBA, bool)) (drawingml.RGBA, bool) {
	v, _ := b.value(s, name)
	c, ok := b.color(v)
	if !ok {
		if v != themed || theme == nil {
			if v == "" {
				return drawingml.RGBA{}, false
			}
			return drawingml.Black, true
		}
		if c, ok = theme(); !ok {
			return drawingml.RGBA{}, false
		}
		if tv, _ := b.value(s, trans); tv == themed {
			return c, true
		}
	}
	if t := b.num(s, trans, 0); t > 0 {
		c = c.WithAlpha(1 - min(t, 1))
	}
	return c, true
}

// themeFill returns the fill of the theme for the shape, with the color of
// QuickStyleFillColor in place of phClr.
func (b *builder) themeFill(q quickStyle) (drawingml.Paint, bool) {
	if b.theme == nil {
		return drawingml.Paint{}, false
	}
	idx := b.theme.styleIndexes(q, b.variantStyle)
	list := b.theme.fills
	if q.connector && len(b.theme.connFills) > 0 {
		list = b.theme.connFills
	}
	n, ok := pick(list, idx.fill)
	if !ok {
		return drawingml.Paint{}, false
	}
	return b.colors.WithPlaceholder(b.themeColor(q.fillColor)).PaintOf(n, nil)
}

// themeLine returns the line of the theme for the shape, and its extra
// properties, with the color of QuickStyleLineColor in place of phClr.
func (b *builder) themeLine(q quickStyle) (drawingml.Line, lineEx, bool) {
	if b.theme == nil {
		return drawingml.Line{}, lineEx{}, false
	}
	idx := b.theme.styleIndexes(q, b.variantStyle)
	list, exs := b.theme.lines, b.theme.lineEx
	if q.connector && len(b.theme.connLines) > 0 {
		list, exs = b.theme.connLines, b.theme.connLineEx
	}
	n, ok := pick(list, idx.line)
	if !ok {
		return drawingml.Line{}, lineEx{}, false
	}
	ex, ok := pick(exs, idx.line)
	if !ok {
		ex = lineEx{beginSize: 2, endSize: 2, pattern: 1}
	}
	l := b.colors.WithPlaceholder(b.themeColor(q.lineColor)).ParseLine(n, drawingml.DefaultLine(), nil)
	return l, ex, true
}

// themeFont returns the color and the style of the text of the theme for
// the shape.
func (b *builder) themeFont(q quickStyle) (drawingml.RGBA, int, bool) {
	if b.theme == nil {
		return drawingml.Black, 0, false
	}
	idx := b.theme.styleIndexes(q, b.variantStyle)
	list := b.theme.fonts
	if q.connector && len(b.theme.connFonts) > 0 {
		list = b.theme.connFonts
	}
	p, ok := pick(list, idx.font)
	if !ok || !p.color.IsElement() {
		return drawingml.Black, 0, false
	}
	// the colors of the variant give the fills and the lines, and the text
	// over them takes the dark color of the theme
	ph := b.themeColor(0)
	if q.fontColor >= 0 && q.fontColor < len(themeColors) {
		ph = b.themeColor(q.fontColor)
	}
	c, ok := b.colors.WithPlaceholder(ph).In(p.color)
	if !ok {
		c = drawingml.Black
	}
	return c, p.style, true
}

// fill returns the fill of the shape, from the cells FillPattern,
// FillForegnd, FillBkgnd and their transparencies, from the gradient of
// Visio 2013, or from the theme.
func (b *builder) fill(s *shape, q quickStyle) drawingml.Paint {
	none := drawingml.Paint{Kind: drawingml.PaintNone}
	tp, themeOK := b.themeFill(q)
	pattern := 1
	switch v, ok := b.value(s, "FillPattern"); {
	case !ok:
	case v == themed:
		if b.theme != nil {
			idx := b.theme.styleIndexes(q, b.variantStyle)
			if p, ok := pick(b.theme.fillPatterns, idx.fill); ok && !q.connector {
				pattern = p
			}
		}
	default:
		pattern = int(parseNum(v, 1))
	}
	if pattern == 0 {
		return none
	}
	fromTheme := func() (drawingml.RGBA, bool) {
		if themeOK {
			return tp.Mean(), true
		}
		return drawingml.White, true
	}
	fg, ok := b.cellColor(s, "FillForegnd", "FillForegndTrans", fromTheme)
	if !ok {
		fg = drawingml.White
	}
	bg, ok := b.cellColor(s, "FillBkgnd", "FillBkgndTrans", fromTheme)
	if !ok {
		bg = drawingml.White
	}
	if v, _ := b.value(s, "FillGradientEnabled"); isTrue(v) || v == themed && themeOK && tp.Kind == drawingml.PaintGradient {
		if g, ok := b.gradient(s, fg, tp, themeOK); ok {
			return g
		}
	}
	if v, _ := b.value(s, "FillForegnd"); v == themed && themeOK && pattern == 1 {
		if tp.Kind == drawingml.PaintGradient || tp.Kind == drawingml.PaintNone {
			return tp
		}
	}
	switch {
	case pattern == 1:
		return drawingml.Solid(fg)
	case pattern >= 2 && pattern <= 24:
		return drawingml.Paint{Kind: drawingml.PaintPattern, Color: fg, Background: bg, Pattern: fillPatterns[pattern-2]}
	case pattern >= 25 && pattern <= 40:
		return legacyGradient(pattern, fg, bg)
	}
	return drawingml.Solid(fg)
}

// fillPatterns maps the fill patterns 2 to 24 of Visio to the patterns of
// DrawingML that look the most like them.
var fillPatterns = [...]string{
	"horz", "vert", "upDiag", "dnDiag", "cross", "diagCross", "dkUpDiag", "dkDnDiag",
	"wdUpDiag", "wdDnDiag", "pct25", "pct50", "pct75", "smCheck", "lgCheck", "plaid",
	"sphere", "trellis", "shingle", "weave", "divot", "dotGrid", "smConfetti",
}

// legacyGradient returns the fill of the gradients 25 to 40 of the fill
// patterns of Visio, from the foreground fg to the background bg.
func legacyGradient(pattern int, fg, bg drawingml.RGBA) drawingml.Paint {
	g := drawingml.Gradient{Stops: []drawingml.Stop{{Pos: 0, Color: fg}, {Pos: 1, Color: bg}}}
	switch pattern {
	case 25:
		g.Angle = 0
	case 26:
		g.Stops = []drawingml.Stop{{Pos: 0, Color: bg}, {Pos: 0.5, Color: fg}, {Pos: 1, Color: bg}}
	case 27:
		g.Angle = 180
	case 28:
		g.Angle = 90
	case 29:
		g.Angle = 90
		g.Stops = []drawingml.Stop{{Pos: 0, Color: bg}, {Pos: 0.5, Color: fg}, {Pos: 1, Color: bg}}
	case 30:
		g.Angle = 270
	case 31, 32, 33, 34:
		g.Angle = float64(45 + 90*(pattern-31))
	default:
		g.Path = "circle"
		g.Focus = drawingml.Insets{L: 0.5, T: 0.5, R: 0.5, B: 0.5}
	}
	return drawingml.Paint{Kind: drawingml.PaintGradient, Gradient: g}
}

// maxRows bounds the rows of a section that the package reads.
const maxRows = 1 << 16

// gradient returns the gradient fill of Visio 2013: the stops of the
// section FillGradient, in the direction of FillGradientDir and
// FillGradientAngle. A themed stop takes the gradient of the theme tp.
func (b *builder) gradient(s *shape, fg drawingml.RGBA, tp drawingml.Paint, themeOK bool) (drawingml.Paint, bool) {
	c := b.chain(s, catFill)
	var stops []drawingml.Stop
	for _, r := range c.rows("FillGradient", maxRows) {
		cv, _, _ := r.cell("GradientStopColor")
		col, ok := b.color(cv.v)
		if !ok {
			if themeOK && tp.Kind == drawingml.PaintGradient {
				return tp, true
			}
			col = fg
		}
		tv, _, _ := r.cell("GradientStopColorTrans")
		if t := parseNum(tv.v, 0); t > 0 {
			col = col.WithAlpha(1 - min(t, 1))
		}
		pv, _, _ := r.cell("GradientStopPosition")
		stops = append(stops, drawingml.Stop{Pos: min(max(parseNum(pv.v, 0), 0), 1), Color: col})
	}
	if len(stops) == 0 {
		return drawingml.Paint{}, false
	}
	g := drawingml.Gradient{Stops: stops}
	if b.num(s, "FillGradientDir", 0) == 0 {
		g.Angle = b.num(s, "FillGradientAngle", math.Pi/2) * 180 / math.Pi
	} else {
		g.Path = "circle"
		g.Focus = drawingml.Insets{L: 0.5, T: 0.5, R: 0.5, B: 0.5}
	}
	return drawingml.Paint{Kind: drawingml.PaintGradient, Gradient: g}, true
}

// linePatterns maps the line patterns 2 to 23 of Visio to dashes, in
// multiples of the width of the line.
var linePatterns = [...][]float64{
	{4, 3}, {1, 3}, {4, 3, 1, 3}, {4, 3, 1, 3, 1, 3}, {4, 3, 4, 3, 1, 3}, {8, 3},
	{8, 3, 1, 3}, {8, 3, 1, 3, 1, 3}, {2, 2}, {1, 1}, {2, 1, 1, 1}, {2, 1, 1, 1, 1, 1},
	{2, 1, 2, 1, 1, 1}, {6, 2}, {6, 2, 1, 2}, {6, 2, 1, 2, 1, 2}, {4, 1}, {1, 2},
	{4, 1, 1, 1}, {4, 1, 1, 1, 1, 1}, {4, 1, 4, 1, 1, 1}, {3, 2},
}

// arrows are the arrows at the ends of the line of a shape, with their
// sizes from 0 to 6.
type arrows struct {
	begin, beginSize, end, endSize int
}

// line returns the line of the shape and its arrows, from the cells
// LineWeight, LineColor, LinePattern, LineCap and the arrows, or from the
// theme.
func (b *builder) line(s *shape, q quickStyle) (drawingml.Line, arrows) {
	tl, ex, themeOK := b.themeLine(q)
	l := drawingml.DefaultLine()
	l.Join = okage.LineJoinRound
	l.Cap = okage.LineCapRound
	pattern := 1
	switch v, ok := b.value(s, "LinePattern"); {
	case !ok:
	case v == themed:
		if themeOK {
			pattern = ex.pattern
		}
	default:
		pattern = int(parseNum(v, 1))
	}
	if pattern == 0 {
		return l, arrows{}
	}
	switch v, ok := b.value(s, "LineWeight"); {
	case v == themed && themeOK:
		l.Width = tl.Width
	case ok && v != themed:
		l.Width = min(max(parseNum(v, 0)*72, 0), 1000)
	}
	fromTheme := func() (drawingml.RGBA, bool) {
		if themeOK {
			return tl.Paint.Mean(), true
		}
		return drawingml.Black, true
	}
	c, ok := b.cellColor(s, "LineColor", "LineColorTrans", fromTheme)
	if !ok {
		c = drawingml.Black
	}
	l.Paint = drawingml.Solid(c)
	if v, _ := b.value(s, "LineGradientEnabled"); isTrue(v) {
		for _, r := range b.chain(s, catLine).rows("LineGradient", 1) {
			cv, _, _ := r.cell("GradientStopColor")
			if col, ok := b.color(cv.v); ok {
				l.Paint = drawingml.Solid(col)
			}
		}
	}
	if pattern >= 2 && pattern-2 < len(linePatterns) {
		l.Dash = linePatterns[pattern-2]
	}
	switch v, _ := b.value(s, "LineCap"); {
	case v == themed && themeOK:
		l.Cap = tl.Cap
	case v == "1", v == "2":
		l.Cap = okage.LineCapSquare
	}
	a := arrows{
		begin:     b.themedInt(s, "BeginArrow", 0, themeOK, ex.begin),
		beginSize: b.themedInt(s, "BeginArrowSize", 2, themeOK, ex.beginSize),
		end:       b.themedInt(s, "EndArrow", 0, themeOK, ex.end),
		endSize:   b.themedInt(s, "EndArrowSize", 2, themeOK, ex.endSize),
	}
	return l, a
}

// themedInt returns the integer of the cell with the name, the value of
// the theme th for a themed cell, or def.
func (b *builder) themedInt(s *shape, name string, def int, themeOK bool, th int) int {
	v, ok := b.value(s, name)
	switch {
	case !ok:
		return def
	case v == themed:
		if themeOK {
			return th
		}
		return def
	}
	return int(parseNum(v, float64(def)))
}

// shadow returns the shadow of the shape, from the cells ShdwPattern,
// ShdwForegnd and the offsets, or nil. The shadows of the effects of the
// theme are left out.
func (b *builder) shadow(s *shape) *drawingml.Shadow {
	v, _ := b.value(s, "ShdwPattern")
	if v == themed || parseNum(v, 0) == 0 {
		return nil
	}
	if show, _ := b.value(s, "ShapeShdwShow"); show == "2" {
		return nil
	}
	c, ok := b.cellColor(s, "ShdwForegnd", "ShdwForegndTrans", nil)
	if !ok {
		c = drawingml.Black
	}
	dx, dy := b.num(s, "ShapeShdwOffsetX", 0), b.num(s, "ShapeShdwOffsetY", 0)
	if dx == 0 && dy == 0 {
		dx, dy = parseNum(b.pageCell("ShdwOffsetX"), 0.125), parseNum(b.pageCell("ShdwOffsetY"), -0.125)
	}
	if math.Abs(dx) > 100 || math.Abs(dy) > 100 {
		return nil
	}
	return &drawingml.Shadow{Color: c, DX: dx * 72, DY: -dy * 72, SX: 1, SY: 1}
}

// arrowLength is the length of an arrow of each size from 0 to 6, in
// points, for a thin line. An arrow grows with the width of its line.
var arrowLength = [...]float64{3, 5, 7, 9, 12, 16, 24}

// drawArrow draws the arrow kind of the size at the point tip, pointing
// away from the point from, with the shader sh, onto dev. w is the width of
// the line in points.
func drawArrow(dev display.Device, kind, size int, tip, from pt, w float64, sh okage.Shader) {
	d := tip.sub(from)
	l := math.Hypot(d.x, d.y)
	if l == 0 || kind <= 0 {
		return
	}
	ux, uy := d.x/l, d.y/l
	al := arrowLength[min(max(size, 0), len(arrowLength)-1)] * max(1, w/1.5)
	aw := al * 0.8
	at := func(along, across float64) (float32, float32) {
		return float32(tip.x - ux*along - uy*across), float32(tip.y - uy*along + ux*across)
	}
	pb := okage.NewPathBuilder()
	fill := true
	switch kind {
	case 1, 8, 12, 15:
		// an open arrow of two lines
		pb.MoveTo(at(al, aw/2))
		pb.LineTo(at(0, 0))
		pb.LineTo(at(al, -aw/2))
		fill = false
	case 10, 20, 21, 41, 42:
		r, ok := okage.RectFromXYWH(float32(tip.x-al/3), float32(tip.y-al/3), float32(al*2/3), float32(al*2/3))
		if !ok {
			return
		}
		if p, ok := okage.PathFromOval(r); ok {
			dev.FillPath(p, okage.FillRuleWinding, sh)
		}
		return
	case 22, 23, 24, 25:
		pb.MoveTo(at(0, 0))
		pb.LineTo(at(al/2, aw/2))
		pb.LineTo(at(al, 0))
		pb.LineTo(at(al/2, -aw/2))
		pb.Close()
	case 6, 7, 16, 17:
		// a stealth arrow
		pb.MoveTo(at(0, 0))
		pb.LineTo(at(al, aw/2))
		pb.LineTo(at(al*0.6, 0))
		pb.LineTo(at(al, -aw/2))
		pb.Close()
	default:
		pb.MoveTo(at(0, 0))
		pb.LineTo(at(al, aw/2))
		pb.LineTo(at(al, -aw/2))
		pb.Close()
	}
	p, ok := pb.Finish()
	if !ok {
		return
	}
	if fill {
		dev.FillPath(p, okage.FillRuleWinding, sh)
		return
	}
	st := okage.DefaultStroke()
	st.Width = float32(max(w, 0.75))
	st.LineJoin = okage.LineJoinMiter
	dev.StrokePath(p, st, sh)
}

// addArrows adds the arrows of a line to the scene. m maps the coordinates
// of the shape onto the frame of the figure, and frame maps the frame onto
// the page.
func (b *builder) addArrows(subs []subpath, l drawingml.Line, a arrows, m affine, frame okage.Transform) {
	if a.begin <= 0 && a.end <= 0 || !l.Visible() {
		return
	}
	begin, beginFrom, end, endFrom, ok := ends(subs)
	if !ok {
		return
	}
	col := l.Paint.Mean()
	w := l.Width
	b.scene.Add(frame, func(_ context.Context, dev display.Device, _ float64) error {
		sh := okage.SolidColor(col.Okage())
		if a.begin > 0 {
			drawArrow(dev, a.begin, a.beginSize, m.apply(begin), m.apply(beginFrom), w, sh)
		}
		if a.end > 0 {
			drawArrow(dev, a.end, a.endSize, m.apply(end), m.apply(endFrom), w, sh)
		}
		return nil
	})
}

// parseNum parses the value of a cell as a number, or returns def.
func parseNum(v string, def float64) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return def
	}
	return f
}
