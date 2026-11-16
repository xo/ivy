package chart

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"
	"github.com/xo/zuzu"
	"github.com/xo/zuzu/scene"
	zsvg "github.com/xo/zuzu/svg"
	ztext "github.com/xo/zuzu/text"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/ivy/internal/fontdb"
)

// This file draws a chart. zuzu lays the spec out and returns its scene.
// The scene gets the labels on the data, and zuzu writes it as SVG. okage
// reads the SVG into its tree of paths, with the text as the outlines of
// its glyphs, and the drawing sends each path to the device. The chart
// stays vectors, and its text stays vectors too.

// The bounds of the size of the layout of a chart, in points. A chart of
// another size lays out at the nearest size in the bounds, and scales to its
// rectangle.
const (
	minSize = 24
	maxSize = 4096
	// maxTreeDepth is the depth of the groups of the tree of okage that
	// the drawing follows.
	maxTreeDepth = 64
)

// Draw draws the chart onto dev, into the rectangle rect of the user space of
// dev. The chart lays out at the size of Options, in points, and scales to
// rect. Without a size, it lays out at the size of rect, as points. Draw
// saves and restores the state of dev. It returns an *ivy.PartialError for a
// chart that has more series or points than the limits, and draws the first
// ones.
func (c *Chart) Draw(ctx context.Context, dev display.Device, rect okage.Rect) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ivy.PartialError{Err: fmt.Errorf("drawing chart: panic: %v: %w", r, ivy.ErrFormat)}
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	rw, rh := float64(rect.Width()), float64(rect.Height())
	if !(rw > 0) || !(rh > 0) || math.IsInf(rw, 0) || math.IsInf(rh, 0) {
		return nil
	}
	w, h := c.width, c.height
	if !(w > 0) || !(h > 0) {
		w, h = rw, rh
	}
	if k := maxSize / max(w, h); k < 1 {
		w, h = w*k, h*k
	}
	w, h = max(w, minSize), max(h, minSize)
	db := c.fontDatabase()
	family := c.family(db)
	src, err := c.Spec(w, h, family)
	if err != nil {
		return err
	}
	zc, report, err := zuzu.Parse(src)
	if err != nil {
		return fmt.Errorf("laying out chart: %w: %w", ivy.ErrFormat, err)
	}
	s, report2, err := zc.Render(ctx, zuzu.WithBackground(color.Transparent))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("laying out chart: %w: %w", ivy.ErrFormat, err)
	}
	if len(report)+len(report2) > 0 {
		c.cfg.Log().Debug("chart: zuzu report", "report", append(report, report2...))
	}
	c.decorate(s, family)
	var b bytes.Buffer
	if err := zsvg.Write(&b, s); err != nil {
		return fmt.Errorf("writing chart: %w: %w", ivy.ErrFormat, err)
	}
	tree, err := svg.FromData(b.Bytes(), svg.WithFontDatabase(db), svg.WithMaxDepth(maxTreeDepth))
	if err != nil {
		return fmt.Errorf("reading chart: %w: %w", ivy.ErrFormat, err)
	}
	sw, sh := s.Width, s.Height
	if !(sw > 0) || !(sh > 0) {
		sw, sh = w, h
	}
	dev.Save()
	defer dev.Restore()
	dev.Concat(okage.TransformFromRow(float32(rw/sw), 0, 0, float32(rh/sh), rect.X(), rect.Y()))
	c.background(dev, sw, sh)
	drawGroup(dev, tree.Root(), 0)
	if c.truncated {
		return &ivy.PartialError{Err: fmt.Errorf("chart with more than %d series or %d points: %w", maxSeries, maxPoints, ivy.ErrLimit)}
	}
	return nil
}

// background draws the fill and the border of the chart.
func (c *Chart) background(dev display.Device, w, h float64) {
	r, ok := okage.RectFromXYWH(0, 0, float32(w), float32(h))
	if !ok {
		return
	}
	p := okage.PathFromRect(r)
	if c.fill.A > 0 {
		dev.FillPath(p, okage.FillRuleWinding, okage.SolidColor(rgba(c.fill)))
	}
	if c.border.A > 0 {
		dev.StrokePath(p, okage.Stroke{Width: 0.75, MiterLimit: 4}, okage.SolidColor(rgba(c.border)))
	}
}

// rgba converts a color to a color of okage.
func rgba(c color.NRGBA) okage.Color {
	return okage.ColorFromRGBA8(c.R, c.G, c.B, c.A)
}

// fontDatabase returns the fonts of the text of the chart: the fonts of
// Options, or the fonts of the program.
func (c *Chart) fontDatabase() *svg.FontDatabase {
	if c.fonts != nil {
		return c.fonts
	}
	return fontdb.Of(c.cfg)
}

// family returns the family that draws the text of the chart: its font if
// db has it, else the font with the same metrics if db has it, else the
// sans serif family of db, else the first family of db.
func (c *Chart) family(db *svg.FontDatabase) string {
	has := func(name string) bool {
		if name == "" {
			return false
		}
		for face := range db.Faces() {
			for _, f := range face.Families {
				if strings.EqualFold(f.Name, name) {
					return true
				}
			}
		}
		return false
	}
	if has(c.font) {
		return c.font
	}
	if sub, ok := font.Substitute(c.font); ok && has(sub) {
		return sub
	}
	if f := db.FamilyName(svg.FontFamily{Kind: svg.FontFamilySansSerif}); has(f) {
		return f
	}
	for face := range db.Faces() {
		if len(face.Families) > 0 {
			return face.Families[0].Name
		}
	}
	return ztext.Family
}

// decorate adds to the scene what zuzu does not draw from the spec: the
// labels on the data, the hole of a doughnut and the angle of the first
// slice of a pie.
func (c *Chart) decorate(s *scene.Scene, family string) {
	seen := map[[3]int]bool{}
	var walk func(marks []scene.Mark, depth int) []scene.Mark
	walk = func(marks []scene.Mark, depth int) []scene.Mark {
		if depth > maxTreeDepth {
			return marks
		}
		var added []scene.Mark
		for i := range marks {
			m := &marks[i]
			if m.Type == scene.Group {
				for j := range m.Items {
					m.Items[j].Items = walk(m.Items[j].Items, depth+1)
				}
				continue
			}
			if m.Type == scene.Arc {
				c.shapeArcs(m)
			}
			if l, ok := c.labelMark(m, family, seen); ok {
				added = append(added, l)
			}
		}
		return append(marks, added...)
	}
	root := walk([]scene.Mark{s.Root}, 0)
	s.Root = root[0]
	centerTitle(s)
}

// centerTitle puts the title in the middle of the chart, as Office does.
// zuzu puts the title of a spec of Vega-Lite at its start.
func centerTitle(s *scene.Scene) {
	if len(s.Root.Items) == 0 {
		return
	}
	frame := &s.Root.Items[0]
	for i := range frame.Items {
		m := &frame.Items[i]
		if m.Type != scene.Group || m.Role != "title" {
			continue
		}
		for j := range m.Items {
			g := &m.Items[j]
			for k := range g.Items {
				lines := &g.Items[k]
				if lines.Role != "title-text" {
					continue
				}
				for l := range lines.Items {
					lines.Items[l].X = s.Width/2 - frame.X - g.X
					lines.Items[l].Align = "center"
				}
			}
		}
	}
}

// datum returns the row of an item of the data of the chart, which has the
// field p, or false for an item of a guide.
func datum(it *scene.Item) (map[string]any, int, bool) {
	row, ok := it.Datum.(map[string]any)
	if !ok {
		return nil, 0, false
	}
	p, ok := row["p"].(float64)
	if !ok {
		return nil, 0, false
	}
	return row, int(p), true
}

// rowNumber returns a number field of a row, or NaN.
func rowNumber(row map[string]any, key string) float64 {
	if v, ok := row[key].(float64); ok {
		return v
	}
	return math.NaN()
}

// shapeArcs gives the arcs of a pie the hole of a doughnut and the angle of
// the first slice.
func (c *Chart) shapeArcs(m *scene.Mark) {
	for i := range m.Items {
		it := &m.Items[i]
		_, p, ok := datum(it)
		if !ok || p < 0 || p >= len(c.plots) {
			continue
		}
		pl := c.plots[p]
		if pl.hole > 0 {
			it.InnerRadius = pl.hole * it.OuterRadius
		}
		a := pl.firstAngle * math.Pi / 180
		it.StartAngle += a
		it.EndAngle += a
	}
}

// labelMark returns a mark of the labels on the points of the mark m, or
// false when its points have no labels. seen holds the points that have a
// label already, so that the points of a line get one label.
func (c *Chart) labelMark(m *scene.Mark, family string, seen map[[3]int]bool) (scene.Mark, bool) {
	out := scene.Mark{Type: scene.Text, Role: "ivy-labels"}
	for i := range m.Items {
		it := &m.Items[i]
		row, p, ok := datum(it)
		if !ok || p < 0 || p >= len(c.plots) {
			continue
		}
		l, _ := row["l"].(string)
		if l == "" {
			continue
		}
		key := [3]int{p, int(rowNumber(row, "k")), int(rowNumber(row, "i"))}
		if seen[key] {
			continue
		}
		seen[key] = true
		pl := c.plots[p]
		style := c.labelStyle(pl, int(rowNumber(row, "k")))
		t := scene.Item{
			Text: scene.Lines{l}, Font: family, FontSize: style.size,
			Fill: scene.Paint{Color: css(style.color)}, Align: "center", Baseline: "middle",
		}
		if style.bold {
			t.FontWeight = "bold"
		}
		if !place(&t, m.Type, it, pl, rowNumber(row, "v"), style.size) {
			continue
		}
		out.Items = append(out.Items, t)
	}
	return out, len(out.Items) > 0
}

// labelStyle returns the style of the labels of the series k of a plot.
func (c *Chart) labelStyle(p *plot, k int) textStyle {
	l := p.labels
	if k >= 0 && k < len(p.series) && p.series[k].labels.set {
		l = p.series[k].labels
	}
	base := textStyle{size: math.Round(c.text.size * 0.9), color: labelGray, hasColor: true}
	return l.text.merge(base)
}

// place puts the label t next to the item it of a mark of the type typ.
// It returns false for an item that has no place.
func place(t *scene.Item, typ string, it *scene.Item, p *plot, v, size float64) bool {
	gap := size / 3
	switch typ {
	case scene.Rect:
		x, y, w, h := box(it)
		inside := p.grouping == "stacked" || p.grouping == "percentStacked" || p.labels.position == "ctr"
		switch {
		case inside:
			t.X, t.Y = x+w/2, y+h/2
		case p.horizontal && v < 0:
			t.X, t.Y, t.Align = x-gap, y+h/2, "right"
		case p.horizontal:
			t.X, t.Y, t.Align = x+w+gap, y+h/2, "left"
		case v < 0:
			t.X, t.Y, t.Baseline = x+w/2, y+h+gap, "top"
		default:
			t.X, t.Y, t.Baseline = x+w/2, y-gap, "bottom"
		}
	case scene.Arc:
		a := (it.StartAngle + it.EndAngle) / 2
		r := it.InnerRadius + (it.OuterRadius-it.InnerRadius)*0.6
		if it.InnerRadius > 0 {
			r = (it.InnerRadius + it.OuterRadius) / 2
		}
		t.X, t.Y = it.X+r*math.Sin(a), it.Y-r*math.Cos(a)
	case scene.Symbol:
		r := 3.0
		if it.Size != nil && *it.Size > 0 {
			r = math.Sqrt(*it.Size) / 2
		}
		t.X, t.Y, t.Baseline = it.X, it.Y-r-gap, "bottom"
	case scene.Line, scene.Area:
		t.X, t.Y, t.Baseline = it.X, it.Y-gap, "bottom"
	default:
		return false
	}
	return !math.IsNaN(t.X) && !math.IsNaN(t.Y) && !math.IsInf(t.X, 0) && !math.IsInf(t.Y, 0)
}

// box returns the box of a rect item, with a width and a height that are
// not negative.
func box(it *scene.Item) (x, y, w, h float64) {
	x, y, w, h = it.X, it.Y, it.Width, it.Height
	if w == 0 && it.X2 != 0 {
		w = it.X2 - it.X
	}
	if h == 0 && it.Y2 != 0 {
		h = it.Y2 - it.Y
	}
	if w < 0 {
		x, w = x+w, -w
	}
	if h < 0 {
		y, h = y+h, -h
	}
	return x, y, w, h
}

// drawGroup draws a group of the tree of okage and its children.
func drawGroup(dev display.Device, g *svg.Group, depth int) {
	if g == nil || depth > maxTreeDepth {
		return
	}
	op := g.Opacity().Get()
	if op <= 0 {
		return
	}
	dev.Save()
	defer dev.Restore()
	dev.Concat(g.Transform())
	if cp, ok := g.ClipPath(); ok {
		p := clipPath(cp, depth)
		if p == nil {
			return
		}
		dev.ClipPath(p, okage.FillRuleWinding)
	}
	if op < 1 {
		dev.BeginGroup(display.Group{Opacity: op, Isolated: true})
		defer dev.EndGroup()
	}
	for _, n := range g.Children() {
		switch n := n.(type) {
		case *svg.Group:
			drawGroup(dev, n, depth+1)
		case *svg.Path:
			drawPath(dev, n)
		case *svg.Text:
			drawGroup(dev, n.Flattened(), depth+1)
		}
	}
}

// clipPath returns the area of a clip path as one path in the user space
// of the group that it clips, or nil for an empty area.
func clipPath(cp *svg.ClipPath, depth int) *okage.Path {
	pb := okage.NewPathBuilder()
	var add func(g *svg.Group, t okage.Transform, depth int)
	add = func(g *svg.Group, t okage.Transform, depth int) {
		if g == nil || depth > maxTreeDepth {
			return
		}
		t = t.PreConcat(g.Transform())
		for _, n := range g.Children() {
			switch n := n.(type) {
			case *svg.Group:
				add(n, t, depth+1)
			case *svg.Text:
				add(n.Flattened(), t, depth+1)
			case *svg.Path:
				if p, ok := display.Clone(n.Data()).Transform(t); ok {
					pb.PushPath(p)
				}
			}
		}
	}
	add(cp.Root(), cp.Transform(), depth+1)
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// drawPath fills and strokes a path of the tree of okage, in its paint
// order.
func drawPath(dev display.Device, p *svg.Path) {
	if !p.IsVisible() || p.Data() == nil {
		return
	}
	fill := func() {
		f, ok := p.Fill()
		if !ok {
			return
		}
		if b := p.Data().Bounds(); b.Width() == 0 || b.Height() == 0 {
			return
		}
		sh := shader(f.Paint(), f.Opacity())
		if sh == nil {
			return
		}
		rule := okage.FillRuleWinding
		if f.Rule() == svg.FillRuleEvenOdd {
			rule = okage.FillRuleEvenOdd
		}
		dev.FillPath(p.Data(), rule, sh)
	}
	stroke := func() {
		s, ok := p.Stroke()
		if !ok {
			return
		}
		sh := shader(s.Paint(), s.Opacity())
		if sh == nil {
			return
		}
		st := s.ToTinySkia()
		if dash, ok := s.Dasharray(); ok {
			if d, ok := okage.NewStrokeDash(dash, s.Dashoffset()); ok {
				st.Dash = d
			}
		}
		dev.StrokePath(p.Data(), st, sh)
	}
	if p.PaintOrder() == svg.PaintOrderStrokeAndFill {
		stroke()
		fill()
		return
	}
	fill()
	stroke()
}

// shader returns the shader of a paint of the tree of okage, or nil for a
// paint that the drawing does not draw, a pattern.
func shader(paint svg.Paint, opacity svg.Opacity) okage.Shader {
	switch p := paint.(type) {
	case svg.Color:
		return okage.SolidColor(okage.ColorFromRGBA8(p.Red, p.Green, p.Blue, opacity.ToU8()))
	case *svg.LinearGradient:
		mode, stops := gradient(&p.BaseGradient, opacity)
		sh, ok := okage.NewLinearGradient(okage.PointFromXY(p.X1(), p.Y1()), okage.PointFromXY(p.X2(), p.Y2()),
			stops, mode, p.Transform())
		if ok {
			return sh
		}
	case *svg.RadialGradient:
		mode, stops := gradient(&p.BaseGradient, opacity)
		sh, ok := okage.NewRadialGradient(okage.PointFromXY(p.Fx(), p.Fy()), p.Fr().Get(),
			okage.PointFromXY(p.Cx(), p.Cy()), p.R().Get(), stops, mode, p.Transform())
		if ok {
			return sh
		}
	}
	return nil
}

// gradient returns the spread mode and the stops of a gradient of the tree
// of okage, as resvg draws them.
func gradient(g *svg.BaseGradient, opacity svg.Opacity) (okage.SpreadMode, []okage.GradientStop) {
	mode := okage.SpreadModePad
	switch g.SpreadMethod() {
	case svg.SpreadMethodReflect:
		mode = okage.SpreadModeReflect
	case svg.SpreadMethodRepeat:
		mode = okage.SpreadModeRepeat
	}
	stops := make([]okage.GradientStop, 0, len(g.Stops()))
	for _, s := range g.Stops() {
		c := s.Color()
		stops = append(stops, okage.NewGradientStop(s.Offset().Get(),
			okage.ColorFromRGBA8(c.Red, c.Green, c.Blue, s.Opacity().Mul(opacity).ToU8())))
	}
	return mode, stops
}
