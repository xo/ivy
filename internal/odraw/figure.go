package odraw

import (
	"log/slog"
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"

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

// The flags of the property fillShadeType of MS-ODRAW (MSOSHADETYPE). The
// default is a shading with gamma and sigma, and the parameter 16384 in
// the high 16 bits. The parameter is for the flag of a flat band, which
// MS-ODRAW does not define. Office draws the curve of sigma with a value of
// its own and does not read the parameter, so the package does not read it
// either.
const (
	shadeGamma   = 0x1
	shadeSigma   = 0x2
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
	// Fonts draws the text of WordArt. A nil Fonts draws a WordArt shape
	// as the geometry of its type.
	Fonts *svg.FontDatabase
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
	// the angle of msofillShadeScale is in the unit square, which the box
	// of the shape stretches
	g.Scaled = c.Value(PropFillType, 0) == FillShadeScale
	stops := s.shadeColors(c)
	colors := len(stops) >= 2
	if colors {
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
	if colors {
		// PowerPoint draws the stops of fillShadeColors linearly, whatever
		// the shading says
		return drawingml.Paint{Kind: drawingml.PaintGradient, Gradient: g}
	}
	typ := c.Value(PropFillShadeType, shadeDefault)
	g.Stops = drawingml.ShadeStops(g.Stops, typ&shadeSigma != 0, typ&shadeGamma != 0)
	return drawingml.Paint{Kind: drawingml.PaintGradient, Gradient: g}
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

// Bleed returns the width that a shape with the properties c, in the box
// r, draws outside the box: its corners when it turns, half its line,
// its arrowheads and its shadow, and one point more. A format that clips a
// shape to a box makes the box larger by the bleed on each side.
func (s *Style) Bleed(c Chain, r Rect) float64 {
	l := s.Line(c, true)
	pad := 0.0
	if rot := Fixed(c.Value(PropRotation, 0)); rot != 0 && !math.IsNaN(rot) {
		w, h := math.Abs(r.Right-r.Left), math.Abs(r.Bottom-r.Top)
		ww, hh := w, h
		if a := math.Mod(math.Abs(rot), 180); a >= 45 && a < 135 {
			// the shape turns with its sides swapped, as Figure draws it
			ww, hh = h, w
		}
		sin, cos := math.Abs(math.Sin(rot*math.Pi/180)), math.Abs(math.Cos(rot*math.Pi/180))
		pad = max((ww*cos+hh*sin-w)/2, (ww*sin+hh*cos-h)/2, 0)
	}
	if l.Visible() {
		pad += l.Width / 2
		if l.Head.Type != "" || l.Tail.Type != "" {
			pad += 3*l.Width + 3
		}
	}
	if sh := s.Shadow(c); sh != nil {
		pad += max(math.Abs(sh.DX), math.Abs(sh.DY))
	}
	return pad + 1
}

// wrapPolygonSize is the size of the square of the coordinates of a wrap
// polygon: the box of the shape is 21600 by 21600.
const wrapPolygonSize = 21600

// WrapPolygon returns the outline that the text wraps around, of the shape
// sh with the properties c in the box r, in points as x and y pairs: the
// wrap polygon of the shape (pWrapPolygonVertices of MS-ODRAW 2.3.4.33)
// when polygon is set and the shape has one, else the corners of the box.
// The outline turns and flips as Figure draws the shape.
func WrapPolygon(sh *Shape, c Chain, r Rect, polygon bool) []float64 {
	x, y := min(r.Left, r.Right), min(r.Top, r.Bottom)
	w, h := math.Abs(r.Right-r.Left), math.Abs(r.Bottom-r.Top)
	flipH, flipV := sh.Flags&FSPFlipH != 0, sh.Flags&FSPFlipV != 0
	rot := Fixed(c.Value(PropRotation, 0))
	if math.IsNaN(rot) || math.IsInf(rot, 0) {
		rot = 0
	}
	if flipH != flipV {
		rot = -rot
	}
	rot = math.Mod(rot, 360)
	if rot < 0 {
		rot += 360
	}
	if (rot >= 45 && rot < 135) || (rot >= 225 && rot < 315) {
		cx, cy := x+w/2, y+h/2
		w, h = h, w
		x, y = cx-w/2, cy-h/2
	}
	points := []float64{0, 0, w, 0, w, h, 0, h}
	if verts := Array(c.Complex(PropWrapPolygon), 4); polygon && len(verts) >= 3 {
		points = make([]float64, 0, 2*len(verts))
		for _, v := range verts {
			var vx, vy float64
			if len(v) >= 8 {
				vx, vy = float64(int32(le.Uint32(v))), float64(int32(le.Uint32(v[4:])))
			} else {
				vx, vy = float64(int16(le.Uint16(v))), float64(int16(le.Uint16(v[2:])))
			}
			points = append(points, vx*w/wrapPolygonSize, vy*h/wrapPolygonSize)
		}
	}
	t := drawingml.ShapeTransform(x, y, w, h, rot, flipH, flipV)
	for i := 0; i+1 < len(points); i += 2 {
		px, py := float32(points[i]), float32(points[i+1])
		points[i] = float64(t.SX*px + t.KX*py + t.TX)
		points[i+1] = float64(t.KY*px + t.SY*py + t.TY)
	}
	return points
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
	f.TextRect = s.TextRect(sh, c, w, h)
	if WordArt(sh, c) {
		if paths, ok := s.wordArt(sh, c, w, h); ok {
			f.Paths, filled = paths, true
		} else if s.Fonts != nil {
			// WordArt without text draws nothing
			f.Paths = nil
		}
	}
	if filled {
		f.Fill = s.Fill(c)
	} else {
		f.Fill = drawingml.Paint{Kind: drawingml.PaintNone}
	}
	if pib := int(c.Value(PropPib, 0)); pib > 0 {
		key, keyed := c.Get(PropPictureTransparent)
		if pic := s.picture(pib, key, keyed); pic != nil {
			pic = pic.Recolor(PictureRecolor(c))
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

// PictureRecolor returns the change of the colors of the picture of a
// shape with the properties c: its contrast, its brightness, its grayscale
// and its black and white (MS-ODRAW 2.3.23). The contrast is a factor of
// 16.16 bits, and 1 changes nothing. The brightness goes from -32768 to
// 32768. The washout of Word is a contrast of 0.3 and a brightness of 0.7.
// Black and white makes each pixel white when its luminance is at least one
// half, as Word does.
func PictureRecolor(c Chain) drawingml.Recolor {
	var r drawingml.Recolor
	if v, ok := c.Get(PropPictureContrast); ok && v != 0x10000 {
		r.Contrast = drawingml.ContrastOf(Fixed(v))
	}
	if v, ok := c.Get(PropPictureBrightness); ok {
		r.Bright = max(-1, min(float64(int32(v))/32768, 1))
	}
	switch {
	case c.Flag(PropPictureBools, PictureBiLevel, PictureUseBiLevel, false):
		r.BiLevel, r.Threshold = true, 0.5
	case c.Flag(PropPictureBools, PictureGray, PictureUseGray, false):
		r.Gray = true
	}
	return r
}

// BodyStyle returns the style of the text body of a shape: the insets, the
// anchor, the wrap and the direction of the properties c.
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
	// the text flow of MSOTXFL (MS-ODRAW 2.4.5): the tops of the glyphs
	// face the right side of the box and the lines go to the left, or the
	// tops face the left side and the lines go to the right
	switch c.Value(PropTextFlow, TextFlowHorz) {
	case TextFlowTtoBA, TextFlowTtoBN, TextFlowVertN:
		bs.Vert.Set("vert")
	case TextFlowBtoT:
		bs.Vert.Set("vert270")
	}
	return bs
}

// log returns the logger of s, or one that drops the records.
func (s *Style) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.New(slog.DiscardHandler)
}
