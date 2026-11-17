package odraw

import (
	"log/slog"
	"math"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/drawingml"
)

// EMUPerPoint is the number of EMUs in a point.
const EMUPerPoint = 12700

// The default insets of the text of a shape, in EMUs.
const (
	defaultInsetLR = 91440
	defaultInsetTB = 45720
)

// The flags of an OfficeArtCOLORREF (MS-ODRAW 2.2.2).
const (
	ColorSchemeIndex = 0x08000000
	ColorSysIndex    = 0x10000000
)

// The flags of the property fillShadeType of MS-ODRAW. The default is a
// shading with gamma and sigma.
const (
	shadeGamma   = 0x1
	shadeDefault = 0x3
)

// Style gives a drawing what its file holds outside Office Drawing: the
// colors of the scheme and the pictures of the store. It turns the shapes
// into the figures of internal/drawingml.
type Style struct {
	// Scheme returns the color of the index i of the scheme, which is the
	// color scheme of a slide in PowerPoint and the palette of the workbook
	// in Excel.
	Scheme func(i uint32) drawingml.RGBA
	// Picture returns the picture of the entry pib of the store, from 1,
	// with the color of the OfficeArtCOLORREF key transparent when keyed is
	// set, or nil. It can be nil.
	Picture func(pib int, key uint32, keyed bool) *drawingml.Picture
	// Background is the color of a fill of the type msofillBackground.
	Background drawingml.RGBA
	// Log gets the faults of the geometry. It can be nil.
	Log *slog.Logger
}

// RGB returns the opaque color of the bytes r, g and b.
func RGB(r, g, b uint8) drawingml.RGBA {
	return drawingml.RGBA{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: 1}
}

// Color resolves an OfficeArtCOLORREF (MS-ODRAW 2.2.2) of a shape with the
// properties c: an index of the scheme, a color of the shape for a system
// index, or an RGB value.
func (s *Style) Color(c Chain, v uint32) drawingml.RGBA {
	return s.color(c, v, 2)
}

func (s *Style) color(c Chain, v uint32, depth int) drawingml.RGBA {
	switch {
	case v&ColorSchemeIndex != 0:
		if s.Scheme == nil {
			return drawingml.Black
		}
		return s.Scheme(v & 0x00FFFFFF)
	case v&ColorSysIndex != 0 && depth > 0:
		var base drawingml.RGBA
		switch v & 0xFF {
		case 0xF0, 0xF7, 0xF4:
			base = s.color(c, c.Value(PropFillColor, 0xFFFFFF), depth-1)
		case 0xF1, 0xF2:
			base = s.color(c, c.Value(PropLineColor, 0), depth-1)
		case 0xF3:
			base = s.color(c, c.Value(PropShadowColor, 0x808080), depth-1)
		case 0xF5:
			base = s.color(c, c.Value(PropFillBackColor, 0xFFFFFF), depth-1)
		case 0xF6:
			base = s.color(c, c.Value(PropLineBackColor, 0xFFFFFF), depth-1)
		default:
			return drawingml.Black
		}
		f := float64((v>>16)&0xFF) / 255
		switch (v >> 8) & 0xFF {
		case 0x01:
			base = base.Mix(drawingml.Black, 1-f).WithAlpha(1)
		case 0x02:
			base = base.Mix(drawingml.White, 1-f).WithAlpha(1)
		}
		return base
	}
	return RGB(uint8(v), uint8(v>>8), uint8(v>>16))
}

// picture returns the picture of the entry pib of the store, or nil.
func (s *Style) picture(pib int, key uint32, keyed bool) *drawingml.Picture {
	if s.Picture == nil || pib <= 0 {
		return nil
	}
	return s.Picture(pib, key, keyed)
}

// Fill returns the fill of the properties c.
func (s *Style) Fill(c Chain) drawingml.Paint {
	if !c.Flag(PropFillBools, FillFilled, FillUseFilled, true) {
		return drawingml.Paint{Kind: drawingml.PaintNone}
	}
	col := s.Color(c, c.Value(PropFillColor, 0xFFFFFF))
	back := s.Color(c, c.Value(PropFillBackColor, 0xFFFFFF))
	alpha := Unit(Fixed(c.Value(PropFillOpacity, 0x10000)))
	switch typ := c.Value(PropFillType, FillSolid); typ {
	case FillShade, FillShadeCenter, FillShadeShape, FillShadeScale, FillShadeTitle:
		return s.gradient(c, col, back.WithAlpha(Unit(Fixed(c.Value(PropFillBackOpacity, 0x10000)))), alpha)
	case FillPicture, FillTexture:
		if pic := s.picture(int(c.Value(PropFillBlip, 0)), 0, false); pic != nil {
			pt := drawingml.Paint{Kind: drawingml.PaintPicture, Picture: pic, Opacity: alpha}
			if typ == FillTexture {
				pt.Tile = &drawingml.Tile{SX: 1, SY: 1}
			}
			return pt
		}
		return drawingml.Solid(col.WithAlpha(alpha))
	case FillPattern:
		return drawingml.Paint{Kind: drawingml.PaintPattern, Pattern: "pct50", Color: col.WithAlpha(alpha), Background: back}
	case FillBackground:
		return drawingml.Solid(s.Background)
	}
	return drawingml.Solid(col.WithAlpha(alpha))
}

// gradient returns the gradient of a shaded fill. The angle of the fill
// turns counterclockwise from a vector that points up, from the back color
// to the fill color. The focus moves the fill color from the end of the
// vector: 0 leaves it at the end, 100 puts it at the start, and 50 in the
// middle with the back color at both ends. A negative focus swaps the two
// colors.
func (s *Style) gradient(c Chain, col, back drawingml.RGBA, alpha float64) drawingml.Paint {
	col = col.WithAlpha(col.A * alpha)
	g := drawingml.Gradient{Angle: math.Mod(90-Fixed(c.Value(PropFillAngle, 0))+720, 360)}
	if stops := s.shadeColors(c); len(stops) >= 2 {
		g.Stops = stops
	} else {
		focus := float64(int32(c.Value(PropFillFocus, 0))) / 100
		peak, edge := col, back
		if focus < 0 {
			peak, edge, focus = back, col, -focus
		}
		switch pos := 1 - focus; {
		case pos >= 1:
			g.Stops = []drawingml.Stop{{Pos: 0, Color: edge}, {Pos: 1, Color: peak}}
		case pos <= 0:
			g.Stops = []drawingml.Stop{{Pos: 0, Color: peak}, {Pos: 1, Color: edge}}
		default:
			g.Stops = []drawingml.Stop{{Pos: 0, Color: edge}, {Pos: pos, Color: peak}, {Pos: 1, Color: edge}}
		}
	}
	switch c.Value(PropFillType, 0) {
	case FillShadeCenter, FillShadeShape, FillShadeTitle:
		g.Path = "rect"
		g.Focus = drawingml.Insets{
			L: Unit(Fixed(c.Value(PropFillToLeft, 0))),
			T: Unit(Fixed(c.Value(PropFillToTop, 0))),
			R: 1 - Unit(Fixed(c.Value(PropFillToRight, 0))),
			B: 1 - Unit(Fixed(c.Value(PropFillToBottom, 0))),
		}
		// a path gradient goes from the focus to the edge, as the vector
		// of a linear one goes from its start to its end: the back color
		// is at the focus for a focus of 0, as PowerPoint draws it
	}
	if c.Value(PropFillShadeType, shadeDefault)&shadeGamma != 0 {
		g.Stops = gammaStops(g.Stops)
	}
	return drawingml.Paint{Kind: drawingml.PaintGradient, Gradient: g}
}

// gammaSteps is the number of the stops that gammaStops puts between two
// stops.
const gammaSteps = 8

// gammaStops returns the stops of a gradient whose colors mix in linear
// light, as the shading of msoshadeGamma does: a gradient from blue to
// purple stays lighter in its middle than a mix of the values of sRGB. The
// stops between two stops follow the curve of the mix.
func gammaStops(stops []drawingml.Stop) []drawingml.Stop {
	if len(stops) < 2 {
		return stops
	}
	out := make([]drawingml.Stop, 0, (len(stops)-1)*gammaSteps+1)
	for i := range len(stops) - 1 {
		a, b := stops[i], stops[i+1]
		for k := range gammaSteps {
			t := float64(k) / gammaSteps
			out = append(out, drawingml.Stop{Pos: a.Pos + (b.Pos-a.Pos)*t, Color: linearMix(a.Color, b.Color, t)})
		}
	}
	return append(out, stops[len(stops)-1])
}

// linearMix returns the color the fraction t of the way from a to b, mixed
// in linear light. The alpha mixes as it is.
func linearMix(a, b drawingml.RGBA, t float64) drawingml.RGBA {
	mix := func(x, y float64) float64 {
		lx, ly := math.Pow(Unit(x), 2.2), math.Pow(Unit(y), 2.2)
		return math.Pow(lx+(ly-lx)*t, 1/2.2)
	}
	return drawingml.RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: a.A + (b.A-a.A)*t}
}

// shadeColors reads the stops of fillShadeColors, an array of a color and a
// position of 16.16 bits each.
func (s *Style) shadeColors(c Chain) []drawingml.Stop {
	elems := Array(c.Complex(PropFillShadeColors), 8)
	stops := make([]drawingml.Stop, 0, len(elems))
	for _, e := range elems {
		stops = append(stops, drawingml.Stop{
			Color: s.Color(c, le.Uint32(e)),
			Pos:   Unit(Fixed(le.Uint32(e[4:]))),
		})
	}
	return stops
}

// Unit returns v within [0, 1].
func Unit(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return max(0, min(v, 1))
}

// The values of lineDashing (MS-ODRAW 2.4.11), in multiples of the width.
var dashes = map[uint32][]float64{
	1:  {3, 1},
	2:  {1, 1},
	3:  {3, 1, 1, 1},
	4:  {3, 1, 1, 1, 1, 1},
	5:  {1, 3},
	6:  {4, 3},
	7:  {8, 3},
	8:  {4, 3, 1, 3},
	9:  {8, 3, 1, 3},
	10: {8, 3, 1, 3, 1, 3},
}

// The ends of a line of lineStartArrowhead and lineEndArrowhead (MS-ODRAW
// 2.4.10).
var arrows = map[uint32]string{1: "triangle", 2: "stealth", 3: "diamond", 4: "oval", 5: "arrow"}

// arrow returns the end of a line of the type typ, with the width w and the
// length l of the properties c: from 0 for narrow or short to 2 for wide or
// long, and 1 by default (MS-ODRAW 2.4.8 and 2.4.9).
func arrow(c Chain, typ, w, l uint16) drawingml.Arrow {
	a, ok := arrows[c.Value(typ, 0)]
	if !ok {
		return drawingml.Arrow{}
	}
	return drawingml.Arrow{Type: a, W: int(min(c.Value(w, 1), 2)), L: int(min(c.Value(l, 1), 2))}
}

// Line returns the outline of the properties c. def is whether the shape
// has an outline when c does not say.
func (s *Style) Line(c Chain, def bool) drawingml.Line {
	l := drawingml.DefaultLine()
	if !c.Flag(PropLineBools, LineLine, LineUseLine, def) {
		return l
	}
	col := s.Color(c, c.Value(PropLineColor, 0))
	col = col.WithAlpha(Unit(Fixed(c.Value(PropLineOpacity, 0x10000))))
	l.Paint = drawingml.Solid(col)
	l.Width = min(float64(c.Value(PropLineWidth, 9525))/EMUPerPoint, 1000)
	l.Dash = dashes[c.Value(PropLineDashing, 0)]
	l.Head = arrow(c, PropLineStartArrow, PropLineStartWidth, PropLineStartLength)
	l.Tail = arrow(c, PropLineEndArrow, PropLineEndWidth, PropLineEndLength)
	return l
}

// Shadow returns the shadow of the properties c, or nil.
func (s *Style) Shadow(c Chain) *drawingml.Shadow {
	if !c.Flag(PropShadowBools, ShadowShadow, ShadowUseShdw, false) {
		return nil
	}
	col := s.Color(c, c.Value(PropShadowColor, 0x808080))
	return &drawingml.Shadow{
		Color: col.WithAlpha(Unit(Fixed(c.Value(PropShadowOpacity, 0x10000)))),
		DX:    float64(int32(c.Value(PropShadowOffsetX, 25400))) / EMUPerPoint,
		DY:    float64(int32(c.Value(PropShadowOffsetY, 25400))) / EMUPerPoint,
	}
}

// Hidden reports whether the properties c hide the shape.
func Hidden(c Chain) bool {
	return c.Flag(PropGroupBools, GroupHidden, GroupUseHidden, false)
}

// Figure returns the figure of the shape sh with the properties c, in the
// box r in points on the page, or nil when the box cannot be drawn. The
// figure has no text: the format gives it.
func (s *Style) Figure(sh *Shape, c Chain, r Rect) *drawingml.Figure {
	x, y := r.Left, r.Top
	w, h := r.Right-r.Left, r.Bottom-r.Top
	if w < 0 {
		x, w = x+w, -w
	}
	if h < 0 {
		y, h = y+h, -h
	}
	if !(w < 1e6 && h < 1e6) || !(math.Abs(x) < 1e6 && math.Abs(y) < 1e6) {
		return nil
	}
	flipH, flipV := sh.Flags&FSPFlipH != 0, sh.Flags&FSPFlipV != 0
	rot := Fixed(c.Value(PropRotation, 0))
	if flipH != flipV {
		// the shape flips after it turns, so a shape that flips once
		// turns the other way, as PowerPoint draws it
		rot = -rot
	}
	rot = math.Mod(rot, 360)
	if rot < 0 {
		rot += 360
	}
	if (rot >= 45 && rot < 135) || (rot >= 225 && rot < 315) {
		// the anchor of a shape that turns by about 90 degrees is the box of
		// the shape turned
		cx, cy := x+w/2, y+h/2
		w, h = h, w
		x, y = cx-w/2, cy-h/2
	}
	f := &drawingml.Figure{
		Transform:     drawingml.ShapeTransform(x, y, w, h, rot, flipH, flipV),
		W:             w,
		H:             h,
		TextTransform: drawingml.TextTransform(x, y, w, h, rot, flipV),
		// a picture has no outline unless it gives one
		Line:   s.Line(c, sh.Type != SptPictureFrame),
		Shadow: s.Shadow(c),
	}
	var filled bool
	f.Paths, filled = s.Geometry(sh, c, w, h)
	if filled {
		f.Fill = s.Fill(c)
	} else {
		f.Fill = drawingml.Paint{Kind: drawingml.PaintNone}
	}
	if pib := int(c.Value(PropPib, 0)); pib > 0 {
		key, keyed := c.Get(PropPictureTransparent)
		if pic := s.picture(pib, key, keyed); pic != nil {
			f.Fill = drawingml.Paint{Kind: drawingml.PaintPicture, Picture: pic, Opacity: 1, Crop: drawingml.Insets{
				L: Fixed(c.Value(PropCropLeft, 0)),
				T: Fixed(c.Value(PropCropTop, 0)),
				R: Fixed(c.Value(PropCropRight, 0)),
				B: Fixed(c.Value(PropCropBottom, 0)),
			}}
			if sh.Type == SptPictureFrame {
				// a picture frame shows the picture over its paths
				box, _ := okage.RectFromXYWH(0, 0, float32(max(w, 1e-3)), float32(max(h, 1e-3)))
				f.Paths = []drawingml.Path{{Path: okage.PathFromRect(box), Stroke: true}}
			}
		}
	}
	return f
}

// BodyStyle returns the style of the text body of a shape: the insets, the
// anchor and the wrap of the properties c.
func BodyStyle(c Chain) drawingml.BodyStyle {
	var bs drawingml.BodyStyle
	ins := []struct {
		id  uint16
		def uint32
	}{{PropTextLeft, defaultInsetLR}, {PropTextTop, defaultInsetTB}, {PropTextRight, defaultInsetLR}, {PropTextBottom, defaultInsetTB}}
	for i, in := range ins {
		v := float64(int32(c.Value(in.id, in.def))) / EMUPerPoint
		bs.Insets[i].Set(max(0, min(v, 1000)))
	}
	switch c.Value(PropAnchorText, 0) {
	case 1, 4:
		bs.Anchor.Set("ctr")
	case 2, 5, 7, 9:
		bs.Anchor.Set("b")
	default:
		bs.Anchor.Set("t")
	}
	switch c.Value(PropAnchorText, 0) {
	case 3, 4, 5, 8, 9:
		bs.AnchorCenter.Set(true)
	}
	bs.Wrap.Set(c.Value(PropWrapText, 0) != 2)
	return bs
}

// log returns the logger of s, or one that drops the records.
func (s *Style) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.New(slog.DiscardHandler)
}
