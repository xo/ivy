package drawpage

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
)

// asPartial returns the *ivy.PartialError in the chain of err.
func asPartial(err error) (*ivy.PartialError, bool) {
	return errors.AsType[*ivy.PartialError](err)
}

// color parses a color of OpenDocument, "#rrggbb".
func color(s string) (drawingml.RGBA, bool) {
	c, ok := drawingml.ParseHex(s)
	if !ok {
		return drawingml.RGBA{}, false
	}
	return drawingml.RGBAFrom(c), true
}

// opacity returns the opacity of a fill from its graphic properties: its
// draw:opacity, or the mean of its transparency gradient.
func (b *builder) opacity(gp props) float64 {
	if v, ok := percent(gp.get("draw:opacity")); ok {
		return min(max(v, 0), 1)
	}
	if name := gp.get("draw:opacity-name"); name != "" {
		if g, ok := b.st.opacities[name]; ok {
			s, ok1 := percent(at(g, nsDraw, "start"))
			e, ok2 := percent(at(g, nsDraw, "end"))
			if ok1 && ok2 {
				return min(max((s+e)/2, 0), 1)
			}
		}
	}
	return 1
}

// fill returns the fill of a shape or of a page from its graphic or its
// drawing page properties. A shape without draw:fill is filled when solid
// is set.
func (b *builder) fill(gp props, solid bool) drawingml.Paint {
	kind := gp.get("draw:fill")
	if kind == "" && solid {
		kind = "solid"
	}
	alpha := b.opacity(gp)
	switch kind {
	case "solid":
		c, ok := color(gp.get("draw:fill-color"))
		if !ok {
			c = drawingml.RGBA{R: 0.447, G: 0.624, B: 0.812, A: 1}
		}
		return drawingml.Solid(c.WithAlpha(alpha))
	case "gradient":
		g, ok := b.st.gradients[gp.get("draw:fill-gradient-name")]
		if !ok {
			return drawingml.Paint{Kind: drawingml.PaintNone}
		}
		return gradient(g, alpha)
	case "bitmap":
		img, ok := b.st.images[gp.get("draw:fill-image-name")]
		if !ok {
			return drawingml.Paint{Kind: drawingml.PaintNone}
		}
		pic := b.picture(img).pic
		if pic == nil {
			return drawingml.Paint{Kind: drawingml.PaintNone}
		}
		pt := drawingml.Paint{Kind: drawingml.PaintPicture, Picture: pic, Opacity: alpha}
		if gp.get("style:repeat") == "repeat" {
			pt.Tile = tile(gp)
		}
		return pt
	case "hatch":
		h, ok := b.st.hatches[gp.get("draw:fill-hatch-name")]
		if !ok {
			return drawingml.Paint{Kind: drawingml.PaintNone}
		}
		fg, ok := color(at(h, nsDraw, "color"))
		if !ok {
			fg = drawingml.Black
		}
		bg := drawingml.RGBA{}
		if gp.get("draw:fill-hatch-solid") == "true" {
			if c, ok := color(gp.get("draw:fill-color")); ok {
				bg = c
			}
		}
		return drawingml.Paint{Kind: drawingml.PaintPattern, Pattern: hatchPattern(h), Color: fg.WithAlpha(alpha), Background: bg.WithAlpha(alpha)}
	}
	return drawingml.Paint{Kind: drawingml.PaintNone}
}

// tile returns the tiles of a bitmap fill that repeats. A size of zero is
// the size of the picture at its resolution, and a percentage scales that
// size.
func tile(gp props) *drawingml.Tile {
	t := &drawingml.Tile{SX: 1, SY: 1, Flip: "none", Align: "ctr"}
	if v, ok := refPoints[gp.get("draw:fill-image-ref-point")]; ok {
		t.Align = v
	}
	w, wok := length(gp.get("draw:fill-image-width"))
	h, hok := length(gp.get("draw:fill-image-height"))
	if wok && hok && w > 0 && h > 0 {
		t.W, t.H = w, h
	}
	if v, ok := percent(gp.get("draw:fill-image-width")); ok && v > 0 {
		t.SX = v
	}
	if v, ok := percent(gp.get("draw:fill-image-height")); ok && v > 0 {
		t.SY = v
	}
	return t
}

// refPoints maps the values of draw:fill-image-ref-point onto the
// alignments of a tile of DrawingML.
var refPoints = map[string]string{
	"top-left": "tl", "top": "t", "top-right": "tr",
	"left": "l", "center": "ctr", "right": "r",
	"bottom-left": "bl", "bottom": "b", "bottom-right": "br",
}

// hatchPattern returns the pattern of DrawingML that looks like the hatch
// h: lines in its direction, crossed for a double or a triple hatch.
func hatchPattern(h roxmltree.Node) string {
	rot, _ := angleTenths(at(h, nsDraw, "rotation"))
	rot = math.Mod(math.Mod(rot, 180)+180, 180)
	double := at(h, nsDraw, "style") != "single"
	switch {
	case rot < 22.5 || rot >= 157.5:
		if double {
			return "cross"
		}
		return "ltHorz"
	case rot < 67.5:
		if double {
			return "diagCross"
		}
		return "ltUpDiag"
	case rot < 112.5:
		if double {
			return "cross"
		}
		return "ltVert"
	}
	if double {
		return "diagCross"
	}
	return "ltDnDiag"
}

// angleTenths parses an angle of a gradient or a hatch: a number in tenths
// of a degree, or a number with the unit deg, rad or grad. It returns the
// angle in degrees.
func angleTenths(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if v, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
		return v / 10, true
	}
	return angleValue(s)
}

// gradient returns the paint of draw:gradient, with the opacity alpha.
func gradient(g roxmltree.Node, alpha float64) drawingml.Paint {
	start, ok := color(at(g, nsDraw, "start-color"))
	if !ok {
		start = drawingml.Black
	}
	end, ok := color(at(g, nsDraw, "end-color"))
	if !ok {
		end = drawingml.White
	}
	if v, ok := percent(at(g, nsDraw, "start-intensity")); ok {
		start = start.Mix(drawingml.Black, 1-min(max(v, 0), 1))
	}
	if v, ok := percent(at(g, nsDraw, "end-intensity")); ok {
		end = end.Mix(drawingml.Black, 1-min(max(v, 0), 1))
	}
	border, _ := percent(at(g, nsDraw, "border"))
	border = min(max(border, 0), 0.99)
	// the stops from the start to the end, from a newer LibreOffice
	var stops []drawingml.Stop
	for c := range g.Children() {
		if !c.IsElement() || c.TagName().Name() != "gradient-stop" || len(stops) >= 256 {
			continue
		}
		off, _ := float(at(c, nsSVG, "offset"))
		col, ok := color(at(c, nsLOExt, "color-value"))
		if !ok {
			continue
		}
		stops = append(stops, drawingml.Stop{Pos: min(max(off, 0), 1), Color: col.WithAlpha(alpha)})
	}
	if len(stops) < 2 {
		stops = []drawingml.Stop{{Pos: 0, Color: start.WithAlpha(alpha)}, {Pos: 1, Color: end.WithAlpha(alpha)}}
	}
	angle, _ := angleTenths(at(g, nsDraw, "angle"))
	pt := drawingml.Paint{Kind: drawingml.PaintGradient}
	gr := &pt.Gradient
	switch style := at(g, nsDraw, "style"); style {
	case "linear", "axial", "":
		// the gradient goes down at the angle 0, and the angle turns it
		// counterclockwise
		gr.Angle = 90 - angle
		gr.Scaled = false
		list := make([]drawingml.Stop, 0, len(stops)+1)
		if style == "axial" {
			// the start at both edges and the end in the middle
			for _, s := range stops {
				list = append(list, drawingml.Stop{Pos: border/2 + s.Pos*(0.5-border/2), Color: s.Color})
			}
			for i := len(stops) - 1; i >= 0; i-- {
				list = append(list, drawingml.Stop{Pos: 1 - border/2 - stops[i].Pos*(0.5-border/2), Color: stops[i].Color})
			}
			list = append([]drawingml.Stop{{Pos: 0, Color: stops[0].Color}}, list...)
		} else {
			list = append(list, drawingml.Stop{Pos: 0, Color: stops[0].Color})
			for _, s := range stops {
				list = append(list, drawingml.Stop{Pos: border + s.Pos*(1-border), Color: s.Color})
			}
		}
		gr.Stops = list
	default:
		// radial, ellipsoid, square and rectangular: the start at the
		// edge and the end at the center, where the stops of DrawingML
		// start
		gr.Path = "circle"
		if style == "square" || style == "rectangular" {
			gr.Path = "rect"
		}
		cx, ok1 := percent(at(g, nsDraw, "cx"))
		cy, ok2 := percent(at(g, nsDraw, "cy"))
		if !ok1 {
			cx = 0.5
		}
		if !ok2 {
			cy = 0.5
		}
		gr.Focus = drawingml.Insets{L: cx, T: cy, R: 1 - cx, B: 1 - cy}
		list := make([]drawingml.Stop, 0, len(stops)+1)
		for i := len(stops) - 1; i >= 0; i-- {
			list = append(list, drawingml.Stop{Pos: (1 - stops[i].Pos) * (1 - border), Color: stops[i].Color})
		}
		list = append(list, drawingml.Stop{Pos: 1, Color: stops[0].Color})
		gr.Stops = list
	}
	return pt
}

// stroke returns the outline of a shape from its graphic properties. A
// shape without draw:stroke is outlined when solid is set.
func (b *builder) stroke(gp props, solid bool) drawingml.Line {
	l := drawingml.DefaultLine()
	l.Width = 0
	kind := gp.get("draw:stroke")
	if kind == "" && solid {
		kind = "solid"
	}
	if kind != "solid" && kind != "dash" {
		return l
	}
	c, ok := color(gp.get("svg:stroke-color"))
	if !ok {
		c = drawingml.RGBA{R: 0.204, G: 0.396, B: 0.643, A: 1}
	}
	if v, ok := percent(gp.get("svg:stroke-opacity")); ok {
		c = c.WithAlpha(min(max(v, 0), 1))
	}
	l.Paint = drawingml.Solid(c)
	if v, ok := length(gp.get("svg:stroke-width")); ok {
		l.Width = min(max(v, 0), 1000)
	}
	switch gp.get("svg:stroke-linecap") {
	case "round":
		l.Cap = okage.LineCapRound
	case "square":
		l.Cap = okage.LineCapSquare
	}
	switch gp.get("draw:stroke-linejoin") {
	case "miter":
		l.Join = okage.LineJoinMiter
	case "bevel":
		l.Join = okage.LineJoinBevel
	}
	if kind == "dash" {
		if d, ok := b.st.dashes[gp.get("draw:stroke-dash")]; ok {
			l.Dash = dashes(d, l.Width)
			if at(d, nsDraw, "style") == "round" {
				l.Cap = okage.LineCapRound
			}
		}
	}
	l.Head = b.marker(gp, "start", l.Width)
	l.Tail = b.marker(gp, "end", l.Width)
	return l
}

// dashes returns the dashes of draw:stroke-dash, in multiples of the width
// of a line of the width w.
func dashes(d roxmltree.Node, w float64) []float64 {
	unit := max(w, 0.75)
	size := func(name string) float64 {
		s := at(d, nsDraw, name)
		if v, ok := percent(s); ok {
			return max(v, 0.01)
		}
		if v, ok := length(s); ok {
			return max(v/unit, 0.01)
		}
		return 1
	}
	count := func(name string) int {
		v, _ := float(at(d, nsDraw, name))
		return int(min(max(v, 0), 16))
	}
	gap := size("distance")
	list := make([]float64, 0, 2*(count("dots1")+count("dots2")))
	for range count("dots1") {
		list = append(list, size("dots1-length"), gap)
	}
	for range count("dots2") {
		list = append(list, size("dots2-length"), gap)
	}
	if len(list) == 0 {
		return []float64{4, 3}
	}
	return list
}

// marker returns the arrow of the end of a line, start or end, from its
// marker. The kind of the arrow comes from the name of the marker, such as
// "Arrow" or "Circle".
func (b *builder) marker(gp props, end string, lw float64) drawingml.Arrow {
	name := gp.get("draw:marker-" + end)
	if name == "" {
		return drawingml.Arrow{Type: "none"}
	}
	if _, ok := b.st.markers[name]; !ok {
		return drawingml.Arrow{Type: "none"}
	}
	a := drawingml.Arrow{Type: "triangle", W: 1, L: 1}
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "circle"):
		a.Type = "oval"
	case strings.Contains(lower, "square"), strings.Contains(lower, "diamond"):
		a.Type = "diamond"
	case strings.Contains(lower, "line"):
		a.Type = "arrow"
	}
	if w, ok := length(gp.get("draw:marker-" + end + "-width")); ok {
		u := max(lw, 0.75)
		switch {
		case w < 2.5*u:
			a.W, a.L = 0, 0
		case w > 4.5*u:
			a.W, a.L = 2, 2
		}
	}
	return a
}

// shadow returns the shadow of a shape from its graphic properties.
func (b *builder) shadow(gp props) *drawingml.Shadow {
	if gp.get("draw:shadow") != "visible" {
		return nil
	}
	dx, _ := length(gp.get("draw:shadow-offset-x"))
	dy, _ := length(gp.get("draw:shadow-offset-y"))
	c, ok := color(gp.get("draw:shadow-color"))
	if !ok {
		c = drawingml.RGBA{R: 0.5, G: 0.5, B: 0.5, A: 1}
	}
	if v, ok := percent(gp.get("draw:shadow-opacity")); ok {
		c = c.WithAlpha(min(max(v, 0), 1))
	}
	blur, _ := length(gp.get("loext:shadow-blur"))
	return &drawingml.Shadow{Color: c, DX: dx, DY: dy, Blur: blur, SX: 1, SY: 1}
}

// table adds table:table in the box of w by h points of the space m.
func (b *builder) table(t roxmltree.Node, w, h float64, m okage.Transform) {
	tab := &drawingml.Table{}
	cells := 0
	for c := range t.Children() {
		if !c.IsElement() {
			continue
		}
		switch c.TagName().Name() {
		case "table-column":
			repeat, _ := float(at(c, nsTable, "number-columns-repeated"))
			width := 0.0
			cp := b.st.props(b.st.chain("table-column", at(c, nsTable, "style-name")), "table-column-properties", nil)
			if v, ok := length(cp.get("style:column-width")); ok {
				width = v
			}
			for range int(min(max(repeat, 1), 1024)) {
				if len(tab.Cols) < 1024 {
					tab.Cols = append(tab.Cols, width)
				}
			}
		case "table-row", "table-header-rows", "table-rows":
			rows := []roxmltree.Node{c}
			if c.TagName().Name() != "table-row" {
				rows = rows[:0]
				for r := range c.Children() {
					if is(r, nsTable, "table-row") {
						rows = append(rows, r)
					}
				}
			}
			for _, r := range rows {
				rp := b.st.props(b.st.chain("table-row", at(r, nsTable, "style-name")), "table-row-properties", nil)
				row := drawingml.TableRow{}
				if v, ok := length(rp.get("style:row-height")); ok {
					row.Height = v
				} else if v, ok := length(rp.get("style:min-row-height")); ok {
					row.Height = v
				}
				for cell := range r.Children() {
					if !is(cell, nsTable, "table-cell") {
						continue
					}
					cells++
					if cells > 1<<14 {
						break
					}
					row.Cells = append(row.Cells, b.cell(cell))
				}
				tab.Rows = append(tab.Rows, row)
			}
		}
	}
	// the rows grow to the height of the frame, as LibreOffice draws them
	rows := 0.0
	for _, r := range tab.Rows {
		rows += r.Height
	}
	if rows > 0 && rows < h {
		for i := range tab.Rows {
			tab.Rows[i].Height *= h / rows
		}
	}
	// columns without a width share the width of the frame
	known, unknown := 0.0, 0
	for _, c := range tab.Cols {
		if c > 0 {
			known += c
		} else {
			unknown++
		}
	}
	for i, c := range tab.Cols {
		if c <= 0 {
			tab.Cols[i] = max(w-known, 0) / float64(unknown)
		}
	}
	b.scene.AddTable(tab, w, h, m, nil)
}

// cell reads table:table-cell.
func (b *builder) cell(c roxmltree.Node) drawingml.TableCell {
	chain := b.st.chain("table-cell", at(c, nsTable, "style-name"))
	gp := b.st.props(chain, "graphic-properties", nil)
	cp := b.st.props(chain, "table-cell-properties", nil)
	// the insets of a cell of Impress without padding: 0.25 cm across and
	// 0.13 cm down
	cell := drawingml.TableCell{ColSpan: 1, RowSpan: 1, Anchor: "t", Insets: [4]float64{7.09, 3.69, 7.09, 3.69}}
	if v, ok := float(at(c, nsTable, "number-columns-spanned")); ok && v > 1 {
		cell.ColSpan = int(min(v, 1024))
	}
	if v, ok := float(at(c, nsTable, "number-rows-spanned")); ok && v > 1 {
		cell.RowSpan = int(min(v, 1024))
	}
	cell.Fill = b.fill(gp, false)
	if !cell.Fill.Visible() {
		if col, ok := color(cp.get("fo:background-color")); ok {
			cell.Fill = drawingml.Solid(col)
		}
	}
	switch first(gp.get("draw:textarea-vertical-align"), cp.get("style:vertical-align")) {
	case "middle":
		cell.Anchor = "ctr"
	case "bottom":
		cell.Anchor = "b"
	}
	pad, hasPad := length(first(cp.get("fo:padding"), gp.get("fo:padding")))
	border := first(cp.get("fo:border"), gp.get("fo:border"))
	for i, side := range [...]string{"left", "top", "right", "bottom"} {
		if v, ok := length(first(cp.get("fo:padding-"+side), gp.get("fo:padding-"+side))); ok {
			cell.Insets[i] = v
		} else if hasPad {
			cell.Insets[i] = pad
		}
		if l, ok := borderLine(first(cp.get("fo:border-"+side), border)); ok {
			cell.Borders[i] = &l
		}
	}
	// the text takes the default graphic style, as the text of a shape
	base := &textBase{chain: append(b.st.chain("graphic", ""), chain...)}
	cell.Text = b.textBody(c, base)
	return cell
}

// first returns the first string that is not empty.
func first(list ...string) string {
	for _, s := range list {
		if s != "" {
			return s
		}
	}
	return ""
}

// borderLine parses a border of CSS, such as "0.5pt solid #000000".
func borderLine(s string) (drawingml.Line, bool) {
	if s == "" || s == "none" {
		return drawingml.Line{}, false
	}
	l := drawingml.DefaultLine()
	l.Paint = drawingml.Solid(drawingml.Black)
	for _, f := range strings.Fields(s) {
		if v, ok := length(f); ok {
			l.Width = v
		} else if c, ok := color(f); ok {
			l.Paint = drawingml.Solid(c)
		} else if f == "none" || f == "hidden" {
			return drawingml.Line{}, false
		} else if f == "dashed" || f == "dotted" {
			l.Dash = []float64{3, 2}
		}
	}
	return l, true
}
