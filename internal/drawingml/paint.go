package drawingml

import (
	"cmp"
	"context"
	"image"
	"math"
	"math/bits"
	"slices"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/raster"
)

// PaintKind is the kind of a Paint.
type PaintKind uint8

// The kinds of paints, from the fills of ECMA-376 Part 1, 20.1.8.
const (
	// PaintNone draws nothing, as a:noFill.
	PaintNone PaintKind = iota
	// PaintSolid draws one color, as a:solidFill.
	PaintSolid
	// PaintGradient draws a gradient, as a:gradFill.
	PaintGradient
	// PaintPicture draws a picture, as a:blipFill.
	PaintPicture
	// PaintPattern draws a pattern of two colors, as a:pattFill.
	PaintPattern
	// PaintGroup draws the fill of the group of the shape, as a:grpFill.
	PaintGroup
)

// Paint is the fill of a shape, of a line or of text.
type Paint struct {
	Kind PaintKind
	// Color is the color of a solid paint, and the foreground of a
	// pattern.
	Color RGBA
	// Background is the background of a pattern.
	Background RGBA
	// Pattern is the name of the preset of a pattern, such as "pct50" or
	// "dnDiag".
	Pattern string
	// Gradient is the gradient of a gradient paint.
	Gradient Gradient
	// Picture is the picture of a picture paint. It is nil when the
	// document does not hold the picture.
	Picture *Picture
	// Crop is the part of the picture that the paint shows (a:srcRect).
	Crop Insets
	// Stretch places the picture in the box of the shape (a:fillRect).
	Stretch Insets
	// Tile repeats the picture, or is nil to stretch it.
	Tile *Tile
	// Opacity multiplies the alpha of a picture, from 0 to 1.
	Opacity float64
}

// Gradient is a gradient of DrawingML.
type Gradient struct {
	// Stops are the stops, in order.
	Stops []Stop
	// Path is "" for a linear gradient, and "circle", "rect" or "shape" for
	// a gradient along a path.
	Path string
	// Angle is the direction of a linear gradient, in degrees, clockwise
	// from the x axis.
	Angle float64
	// Scaled scales the angle with the box of the shape.
	Scaled bool
	// Focus is the rectangle of a path gradient where the first stop is,
	// as insets of the box (a:fillToRect).
	Focus Insets
}

// Stop is a stop of a gradient.
type Stop struct {
	// Pos is the position, from 0 to 1.
	Pos   float64
	Color RGBA
}

// Tile places the copies of a picture that a picture paint repeats.
type Tile struct {
	// X and Y offset the tiles, in the units of the shape.
	X, Y float64
	// SX and SY scale the picture, from 1 for its size at its resolution.
	SX, SY float64
	// W and H are the size of a tile in the units of the shape. Zero is
	// the size of the picture, scaled by SX and SY.
	W, H float64
	// Flip is "none", "x", "y" or "xy".
	Flip string
	// Align is where the first tile is, such as "tl" or "ctr".
	Align string
}

// Solid returns the paint of one color.
func Solid(c RGBA) Paint {
	return Paint{Kind: PaintSolid, Color: c}
}

// Visible reports whether the paint draws anything.
func (pt Paint) Visible() bool {
	switch pt.Kind {
	case PaintNone, PaintGroup:
		return false
	case PaintSolid:
		return pt.Color.A > 0
	case PaintPicture:
		return pt.Picture != nil && pt.Opacity > 0
	}
	return true
}

// Mean returns one color that stands for the paint, such as the color of
// a text that a gradient fills.
func (pt Paint) Mean() RGBA {
	switch pt.Kind {
	case PaintSolid:
		return pt.Color
	case PaintPattern:
		return pt.Color.Mix(pt.Background, 0.5)
	case PaintGradient:
		if len(pt.Gradient.Stops) > 0 {
			return pt.Gradient.Stops[0].Color
		}
	}
	return RGBA{}
}

// Pictures gives the pictures of the relationships of a part.
type Pictures interface {
	// Picture returns the picture of the relationship with the ID id of
	// the part, or nil if the part has no such picture.
	Picture(id string) *Picture
}

// PaintIn returns the paint of the first child of n that is a fill element,
// such as the a:solidFill of p:spPr. It returns false if n has none.
func (c *Colors) PaintIn(n roxmltree.Node, pics Pictures) (Paint, bool) {
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		if pt, ok := c.PaintOf(e, pics); ok {
			return pt, true
		}
	}
	return Paint{}, false
}

// PaintOf returns the paint of the fill element n: a:noFill, a:solidFill,
// a:gradFill, a:blipFill, a:pattFill or a:grpFill. It returns false if n is
// not a fill element. pics can be nil.
func (c *Colors) PaintOf(n roxmltree.Node, pics Pictures) (Paint, bool) {
	switch n.TagName().Name() {
	case "noFill":
		return Paint{Kind: PaintNone}, true
	case "solidFill":
		v, ok := c.In(n)
		if !ok {
			// a:solidFill without a color is black
			v = Black
		}
		return Solid(v), true
	case "gradFill":
		return c.gradient(n), true
	case "blipFill":
		return c.picturePaint(n, pics), true
	case "pattFill":
		pt := Paint{Kind: PaintPattern, Pattern: attr(n, "prst", "pct5"), Color: Black, Background: White}
		if e, ok := child(n, "fgClr"); ok {
			if v, ok := c.In(e); ok {
				pt.Color = v
			}
		}
		if e, ok := child(n, "bgClr"); ok {
			if v, ok := c.In(e); ok {
				pt.Background = v
			}
		}
		return pt, true
	case "grpFill":
		return Paint{Kind: PaintGroup}, true
	}
	return Paint{}, false
}

// gradient reads a:gradFill.
func (c *Colors) gradient(n roxmltree.Node) Paint {
	pt := Paint{Kind: PaintGradient}
	g := &pt.Gradient
	g.Scaled = true
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "gsLst":
			for s := range e.Children() {
				if !s.IsElement() || s.TagName().Name() != "gs" {
					continue
				}
				if len(g.Stops) >= maxStops {
					break
				}
				v, ok := c.In(s)
				if !ok {
					continue
				}
				g.Stops = append(g.Stops, Stop{Pos: unit(pct(numberAttr(s, "pos", 0))), Color: v})
			}
		case "lin":
			g.Path = ""
			g.Angle = numberAttr(e, "ang", 0) / 60000
			g.Scaled = boolean(e, "scaled", false)
		case "path":
			g.Path = attr(e, "path", "circle")
			if r, ok := child(e, "fillToRect"); ok {
				g.Focus = ParseInsets(r)
			}
		}
	}
	slices.SortStableFunc(g.Stops, func(a, b Stop) int { return cmp.Compare(a.Pos, b.Pos) })
	if len(g.Stops) == 0 {
		return Paint{Kind: PaintNone}
	}
	return pt
}

// maxStops is the number of stops of a gradient that the package reads.
const maxStops = 256

// picturePaint reads a:blipFill.
func (c *Colors) picturePaint(n roxmltree.Node, pics Pictures) Paint {
	pt := Paint{Kind: PaintPicture, Opacity: 1}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "blip":
			id := relAttr(e, "embed")
			// a picture in SVG is in an extension, with a PNG in r:embed
			// for the programs that do not read SVG
			if svg, ok := svgBlip(e); ok {
				id = svg
			}
			if pics != nil && id != "" {
				pt.Picture = pics.Picture(id)
			}
			if t, ok := child(e, "alphaModFix"); ok {
				pt.Opacity = unit(pct(numberAttr(t, "amt", 100000)))
			}
		case "srcRect":
			pt.Crop = ParseInsets(e)
		case "stretch":
			if r, ok := child(e, "fillRect"); ok {
				pt.Stretch = ParseInsets(r)
			}
		case "tile":
			pt.Tile = &Tile{
				X:     emu(numberAttr(e, "tx", 0)),
				Y:     emu(numberAttr(e, "ty", 0)),
				SX:    pct(numberAttr(e, "sx", 100000)),
				SY:    pct(numberAttr(e, "sy", 100000)),
				Flip:  attr(e, "flip", "none"),
				Align: attr(e, "algn", "tl"),
			}
		}
	}
	return pt
}

// svgBlip returns the relationship of the SVG picture of a:blip, from its
// extension asvg:svgBlip.
func svgBlip(blip roxmltree.Node) (string, bool) {
	for n := range blip.Descendants() {
		if n.IsElement() && n.TagName().Name() == "svgBlip" {
			if id := relAttr(n, "embed"); id != "" {
				return id, true
			}
		}
	}
	return "", false
}

// relAttr returns the attribute of n with the name in the namespace of
// the relationships, such as r:embed.
func relAttr(n roxmltree.Node, name string) string {
	for a := range n.Attributes() {
		if ns, _ := a.Namespace(); a.Name() == name && strings.Contains(ns, "relationships") {
			return a.Value()
		}
	}
	return ""
}

// child returns the first child element of n with the local name.
func child(n roxmltree.Node, name string) (roxmltree.Node, bool) {
	for c := range n.Children() {
		if c.IsElement() && c.TagName().Name() == name {
			return c, true
		}
	}
	return roxmltree.Node{}, false
}

// emu converts EMU, the unit of DrawingML, to points.
func emu(v float64) float64 {
	return v / 12700
}

// FillPath fills the path p, in the user space of dev, with the paint. box
// is the box of the shape, over which a gradient or a picture stretches.
// scale is the number of pixels of the device for a unit of the user space.
func (pt *Paint) FillPath(ctx context.Context, dev display.Device, p *okage.Path, rule okage.FillRule, box okage.Rect, scale float64) error {
	if p == nil {
		return nil
	}
	switch pt.Kind {
	case PaintSolid:
		if pt.Color.A > 0 {
			dev.FillPath(p, rule, okage.SolidColor(pt.Color.Okage()))
		}
	case PaintGradient:
		if sh, ok := pt.Gradient.shader(box); ok {
			dev.FillPath(p, rule, sh)
		}
	case PaintPattern:
		dev.FillPath(p, rule, pt.patternShader())
	case PaintPicture:
		return pt.fillPicture(ctx, dev, p, rule, box, scale)
	}
	return nil
}

// fillPicture fills p with a picture paint.
func (pt *Paint) fillPicture(ctx context.Context, dev display.Device, p *okage.Path, rule okage.FillRule, box okage.Rect, scale float64) error {
	if pt.Picture == nil || pt.Opacity <= 0 {
		return nil
	}
	if pt.Tile != nil {
		if pix, size, ok := pt.Picture.pattern(ctx); ok && size.X > 0 && size.Y > 0 {
			// a pixel of the picture takes the size of a dot at the
			// resolution of the picture
			dx, dy := pt.Picture.Resolution()
			sx, sy := pt.Tile.SX*72/dx, pt.Tile.SY*72/dy
			if pt.Tile.W > 0 && pt.Tile.H > 0 {
				sx, sy = pt.Tile.W/float64(size.X), pt.Tile.H/float64(size.Y)
			}
			if sx == 0 || sy == 0 {
				return nil
			}
			ox, oy := float64(box.X())+pt.Tile.X, float64(box.Y())+pt.Tile.Y
			tw, th := float64(size.X)*sx, float64(size.Y)*sy
			switch pt.Tile.Align {
			case "ctr", "t", "b":
				ox += (float64(box.Width()) - tw) / 2
			case "r", "tr", "br":
				ox += float64(box.Width()) - tw
			}
			switch pt.Tile.Align {
			case "ctr", "l", "r":
				oy += (float64(box.Height()) - th) / 2
			case "b", "bl", "br":
				oy += float64(box.Height()) - th
			}
			spread := okage.SpreadModeRepeat
			if pt.Tile.Flip != "none" && pt.Tile.Flip != "" {
				spread = okage.SpreadModeReflect
			}
			t := okage.TransformFromTranslate(float32(ox), float32(oy)).PreScale(float32(sx), float32(sy))
			dev.FillPath(p, rule, okage.NewPattern(pix.AsRef(), spread, okage.FilterQualityBilinear, float32(pt.Opacity), t))
			return nil
		}
	}
	dst, ok := pt.Stretch.apply(box)
	if !ok {
		return nil
	}
	dev.Save()
	defer dev.Restore()
	dev.ClipPath(p, rule)
	return pt.Picture.Draw(ctx, dev, dst, pt.Crop, pt.Opacity, scale)
}

// shader returns the shader of the gradient over the box.
func (g *Gradient) shader(box okage.Rect) (okage.Shader, bool) {
	if len(g.Stops) == 0 {
		return nil, false
	}
	stops := make([]okage.GradientStop, 0, len(g.Stops))
	for _, s := range g.Stops {
		stops = append(stops, okage.NewGradientStop(float32(s.Pos), s.Color.Okage()))
	}
	x, y := float64(box.X()), float64(box.Y())
	w, h := max(float64(box.Width()), 1e-3), max(float64(box.Height()), 1e-3)
	var sh okage.Shader
	var ok bool
	if g.Path == "" {
		var p0, p1 okage.Point
		var t okage.Transform
		if g.Scaled {
			// the angle is in the unit square, which the box stretches
			p0, p1 = linearEnds(g.Angle, 1, 1)
			t = okage.TransformFromTranslate(float32(x), float32(y)).PreScale(float32(w), float32(h))
		} else {
			p0, p1 = linearEnds(g.Angle, w, h)
			t = okage.TransformFromTranslate(float32(x), float32(y))
		}
		sh, ok = okage.NewLinearGradient(p0, p1, stops, okage.SpreadModePad, t)
	} else {
		// the first stop is at the focus, and the last one at the edge.
		// The gradients of the rectangle and of the shape are drawn as an
		// ellipse.
		f := g.Focus
		cx := (f.L + (1 - f.R)) / 2
		cy := (f.T + (1 - f.B)) / 2
		// the radius reaches the farthest corner of the unit square
		r := math.Hypot(max(cx, 1-cx), max(cy, 1-cy))
		if g.Path != "circle" {
			r = max(max(cx, 1-cx), max(cy, 1-cy))
		}
		t := okage.TransformFromTranslate(float32(x), float32(y)).PreScale(float32(w), float32(h))
		c := okage.PointFromXY(float32(cx), float32(cy))
		sh, ok = okage.NewRadialGradient(c, 0, c, float32(r), stops, okage.SpreadModePad, t)
	}
	if !ok {
		return okage.SolidColor(g.Stops[len(g.Stops)-1].Color.Okage()), true
	}
	return sh, true
}

// linearEnds returns the ends of a linear gradient at the angle, in degrees,
// across a box of the size w by h from the origin. The line goes through the
// center, and its ends are on the lines through the corners that are normal
// to it.
func linearEnds(angle, w, h float64) (okage.Point, okage.Point) {
	sin, cos := math.Sincos(angle * math.Pi / 180)
	half := (math.Abs(w*cos) + math.Abs(h*sin)) / 2
	cx, cy := w/2, h/2
	return okage.PointFromXY(float32(cx-cos*half), float32(cy-sin*half)),
		okage.PointFromXY(float32(cx+cos*half), float32(cy+sin*half))
}

// patterns holds the bits of the patterns of DrawingML that are lines, a
// row of 8 pixels in each byte, from the top. The other patterns are drawn
// as the mix of their colors.
var patterns = map[string][8]byte{
	"horz":       {0xff, 0, 0, 0, 0, 0, 0, 0},
	"ltHorz":     {0xff, 0, 0, 0, 0xff, 0, 0, 0},
	"narHorz":    {0xff, 0, 0xff, 0, 0xff, 0, 0xff, 0},
	"dkHorz":     {0xff, 0xff, 0, 0, 0xff, 0xff, 0, 0},
	"wdHorz":     {0xff, 0xff, 0xff, 0, 0, 0, 0, 0},
	"vert":       {0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80},
	"ltVert":     {0x88, 0x88, 0x88, 0x88, 0x88, 0x88, 0x88, 0x88},
	"narVert":    {0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa},
	"dkVert":     {0xcc, 0xcc, 0xcc, 0xcc, 0xcc, 0xcc, 0xcc, 0xcc},
	"wdVert":     {0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0, 0xe0},
	"dnDiag":     {0x80, 0x40, 0x20, 0x10, 0x08, 0x04, 0x02, 0x01},
	"ltDnDiag":   {0x88, 0x44, 0x22, 0x11, 0x88, 0x44, 0x22, 0x11},
	"dkDnDiag":   {0xcc, 0x66, 0x33, 0x99, 0xcc, 0x66, 0x33, 0x99},
	"wdDnDiag":   {0xc1, 0xe0, 0x70, 0x38, 0x1c, 0x0e, 0x07, 0x83},
	"upDiag":     {0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80},
	"ltUpDiag":   {0x11, 0x22, 0x44, 0x88, 0x11, 0x22, 0x44, 0x88},
	"dkUpDiag":   {0x33, 0x66, 0xcc, 0x99, 0x33, 0x66, 0xcc, 0x99},
	"wdUpDiag":   {0x83, 0x07, 0x0e, 0x1c, 0x38, 0x70, 0xe0, 0xc1},
	"cross":      {0xff, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80},
	"smGrid":     {0xff, 0x88, 0x88, 0x88, 0xff, 0x88, 0x88, 0x88},
	"lgGrid":     {0xff, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80},
	"diagCross":  {0x81, 0x42, 0x24, 0x18, 0x18, 0x24, 0x42, 0x81},
	"smCheck":    {0x99, 0x66, 0x66, 0x99, 0x99, 0x66, 0x66, 0x99},
	"lgCheck":    {0xf0, 0xf0, 0xf0, 0xf0, 0x0f, 0x0f, 0x0f, 0x0f},
	"dashHorz":   {0xf0, 0, 0, 0, 0x0f, 0, 0, 0},
	"dashVert":   {0x80, 0x80, 0x80, 0x80, 0x08, 0x08, 0x08, 0x08},
	"horzBrick":  {0xff, 0x80, 0x80, 0x80, 0xff, 0x08, 0x08, 0x08},
	"dotGrid":    {0xaa, 0, 0x80, 0, 0x80, 0, 0x80, 0},
	"plaid":      {0xaa, 0x55, 0xaa, 0x55, 0xf0, 0xf0, 0xf0, 0xf0},
	"trellis":    {0xff, 0x66, 0xff, 0x99, 0xff, 0x66, 0xff, 0x99},
	"zigZag":     {0x81, 0x42, 0x24, 0x18, 0x81, 0x42, 0x24, 0x18},
	"wave":       {0x00, 0x18, 0xa4, 0x03, 0x00, 0x18, 0xa4, 0x03},
	"sphere":     {0x77, 0x98, 0xf8, 0xf8, 0x77, 0x89, 0x8f, 0x8f},
	"shingle":    {0x03, 0x84, 0x48, 0x30, 0x0c, 0x02, 0x01, 0x01},
	"divot":      {0x00, 0x08, 0x04, 0x08, 0x00, 0x80, 0x40, 0x80},
	"weave":      {0x88, 0x54, 0x22, 0x45, 0x88, 0x14, 0x22, 0x51},
	"diagBrick":  {0x80, 0x40, 0x20, 0x10, 0x18, 0x24, 0x42, 0x81},
	"solidDmnd":  {0x10, 0x38, 0x7c, 0xfe, 0x7c, 0x38, 0x10, 0x00},
	"openDmnd":   {0x80, 0x41, 0x22, 0x14, 0x08, 0x14, 0x22, 0x41},
	"dotDmnd":    {0x80, 0x00, 0x22, 0x00, 0x08, 0x00, 0x22, 0x00},
	"smConfetti": {0x80, 0x04, 0x40, 0x02, 0x20, 0x01, 0x10, 0x08},
	"lgConfetti": {0xc6, 0x0c, 0x60, 0x6c, 0x06, 0x30, 0x03, 0x18},
}

// patternShader returns the shader of a pattern paint. A pattern of pctN is
// drawn as N percent of the foreground over the background.
func (pt *Paint) patternShader() okage.Shader {
	name := pt.Pattern
	if rest, ok := strings.CutPrefix(name, "pct"); ok {
		var n int
		for _, r := range rest {
			if r < '0' || r > '9' || n > 100 {
				break
			}
			n = n*10 + int(r-'0')
		}
		return okage.SolidColor(pt.Background.Mix(pt.Color, float64(min(n, 100))/100).Okage())
	}
	rows, ok := patterns[name]
	if !ok {
		return okage.SolidColor(pt.Background.Mix(pt.Color, 0.5).Okage())
	}
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	fg, bg := pt.Color.NRGBA(), pt.Background.NRGBA()
	on := 0
	for y, row := range rows {
		on += bits.OnesCount8(row)
		for x := range 8 {
			c := bg
			if row&(0x80>>x) != 0 {
				c = fg
			}
			img.SetNRGBA(x, y, c)
		}
	}
	pix, err := raster.Pixmap(img)
	if err != nil {
		return okage.SolidColor(pt.Background.Mix(pt.Color, float64(on)/64).Okage())
	}
	// a pixel of the pattern is a pixel of a screen at 96 dots per inch
	return okage.NewPattern(pix.AsRef(), okage.SpreadModeRepeat, okage.FilterQualityNearest, 1, okage.TransformFromScale(0.75, 0.75))
}
