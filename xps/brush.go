package xps

import (
	"cmp"
	"fmt"
	"image"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// brush is a brush of XPS, ready to fill a path. A brush is a shader, or an
// image that draws once into a viewport.
type brush struct {
	shader okage.Shader

	// img draws into dst, inside the viewport, with the transform t and the
	// opacity, for an ImageBrush that does not tile.
	img      image.Image
	dst      okage.Rect
	viewport okage.Rect
	t        okage.Transform
	opacity  float32

	// pix keeps the pixmap of a tiled image alive for its pattern.
	pix *okage.Pixmap
}

// parseColor parses a color of XPS: #RRGGBB, #AARRGGBB, sc#R,G,B,
// sc#A,R,G,B, or ContextColor with the values of its profile.
func parseColor(s string) (okage.Color, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "#"):
		hex := s[1:]
		if len(hex) == 6 {
			hex = "ff" + hex
		}
		v, err := strconv.ParseUint(hex, 16, 32)
		if err != nil || len(hex) != 8 {
			return okage.Color{}, fmt.Errorf("color %q: %w", s, ivy.ErrFormat)
		}
		return okage.ColorFromRGBA8(uint8(v>>16), uint8(v>>8), uint8(v), uint8(v>>24)), nil
	case strings.HasPrefix(s, "sc#"):
		v, err := numbers(s[3:])
		if err != nil || (len(v) != 3 && len(v) != 4) {
			return okage.Color{}, fmt.Errorf("color %q: %w", s, ivy.ErrFormat)
		}
		a := float32(1)
		if len(v) == 4 {
			a, v = v[0], v[1:]
		}
		c, _ := okage.ColorFromRGBA(srgb(v[0]), srgb(v[1]), srgb(v[2]), clamp(a))
		return c, nil
	case strings.HasPrefix(s, "ContextColor"):
		// the values follow the URI of the profile: the alpha, then the
		// channels. ivy reads them as gray, RGB or CMYK, and ignores the
		// profile (D5)
		fields := strings.Fields(s)
		if len(fields) < 3 {
			return okage.Color{}, fmt.Errorf("color %q: %w", s, ivy.ErrFormat)
		}
		v, err := numbers(strings.Join(fields[2:], " "))
		if err != nil || len(v) < 2 {
			return okage.Color{}, fmt.Errorf("color %q: %w", s, ivy.ErrFormat)
		}
		a, ch := clamp(v[0]), v[1:]
		var c okage.Color
		switch len(ch) {
		case 1:
			c, _ = okage.ColorFromRGBA(clamp(ch[0]), clamp(ch[0]), clamp(ch[0]), a)
		case 3:
			c, _ = okage.ColorFromRGBA(clamp(ch[0]), clamp(ch[1]), clamp(ch[2]), a)
		default:
			k := clamp(ch[3])
			c, _ = okage.ColorFromRGBA((1-clamp(ch[0]))*(1-k), (1-clamp(ch[1]))*(1-k), (1-clamp(ch[2]))*(1-k), a)
		}
		return c, nil
	}
	return okage.Color{}, fmt.Errorf("color %q: %w", s, ivy.ErrFormat)
}

// srgb converts a channel of scRGB, which is linear, to sRGB.
func srgb(v float32) float32 {
	v = clamp(v)
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return float32(1.055*math.Pow(float64(v), 1/2.4) - 0.055)
}

// clamp clamps v to the range from 0 to 1.
func clamp(v float32) float32 {
	return min(max(v, 0), 1)
}

// withAlpha returns c with its alpha multiplied by opacity.
func withAlpha(c okage.Color, opacity float32) okage.Color {
	out, _ := okage.ColorFromRGBA(c.Red(), c.Green(), c.Blue(), clamp(c.Alpha()*opacity))
	return out
}

// solid returns a brush of one color.
func solid(c okage.Color) *brush {
	return &brush{shader: okage.SolidColor(c)}
}

// brushValue returns the brush of an attribute value: a color, or a static
// resource.
func (r *renderer) brushValue(s string, res *resources, opacity float32) (*brush, error) {
	if node, ok, err := res.static(s); ok || err != nil {
		if err != nil {
			return nil, err
		}
		return r.brushNode(node, res, opacity)
	}
	c, err := parseColor(s)
	if err != nil {
		return nil, err
	}
	return solid(withAlpha(c, opacity)), nil
}

// brushOf returns the brush of the property name of n, from its attribute or
// its property element, and nil if n has none.
func (r *renderer) brushOf(n roxmltree.Node, name string, res *resources, opacity float32) (*brush, error) {
	if hasAttr(n, name) {
		return r.brushValue(attr(n, name), res, opacity)
	}
	if node, ok := property(n, name); ok {
		return r.brushNode(node, res, opacity)
	}
	return nil, nil
}

// brushNode returns the brush of a brush element.
func (r *renderer) brushNode(n roxmltree.Node, res *resources, opacity float32) (*brush, error) {
	if hasAttr(n, "Opacity") {
		opacity *= number(attr(n, "Opacity"), 1)
	}
	switch local(n) {
	case "SolidColorBrush":
		c, err := parseColor(attr(n, "Color"))
		if err != nil {
			return nil, err
		}
		return solid(withAlpha(c, opacity)), nil
	case "LinearGradientBrush", "RadialGradientBrush":
		return r.gradient(n, res, opacity)
	case "ImageBrush":
		return r.imageBrush(n, res, opacity)
	}
	return nil, fmt.Errorf("brush %s: %w", local(n), ivy.ErrUnsupported)
}

// brushTransform returns the transform of a brush, from its attribute
// Transform or its property element.
func (r *renderer) brushTransform(n roxmltree.Node, res *resources) okage.Transform {
	t, err := r.transformOf(n, "Transform", res)
	if err != nil {
		r.fail(err)
	}
	return t
}

// gradient returns the brush of a linear or a radial gradient.
func (r *renderer) gradient(n roxmltree.Node, res *resources, opacity float32) (*brush, error) {
	var stops []okage.GradientStop
	var last okage.Color
	if gs, ok := propertyElement(n, "GradientStops"); ok {
		type stop struct {
			offset float32
			color  okage.Color
		}
		var list []stop
		for _, s := range elements(gs) {
			c, err := parseColor(attr(s, "Color"))
			if err != nil {
				return nil, err
			}
			list = append(list, stop{clamp(number(attr(s, "Offset"), 0)), withAlpha(c, opacity)})
		}
		slices.SortStableFunc(list, func(a, b stop) int { return cmp.Compare(a.offset, b.offset) })
		for _, s := range list {
			stops = append(stops, okage.NewGradientStop(s.offset, s.color))
			last = s.color
		}
	}
	if len(stops) == 0 {
		return nil, fmt.Errorf("gradient without stops: %w", ivy.ErrFormat)
	}
	spread := okage.SpreadModePad
	switch attr(n, "SpreadMethod") {
	case "Reflect":
		spread = okage.SpreadModeReflect
	case "Repeat":
		spread = okage.SpreadModeRepeat
	}
	t := r.brushTransform(n, res)
	var shader okage.Shader
	var ok bool
	if local(n) == "LinearGradientBrush" {
		start, end := pointAttr(n, "StartPoint"), pointAttr(n, "EndPoint")
		shader, ok = okage.NewLinearGradient(start, end, stops, spread, t)
	} else {
		center, origin := pointAttr(n, "Center"), pointAttr(n, "GradientOrigin")
		rx, ry := number(attr(n, "RadiusX"), 0), number(attr(n, "RadiusY"), 0)
		if rx <= 0 || ry <= 0 {
			return nil, nil
		}
		// an ellipse is a circle of radius rx, scaled in y about the center
		scale := okage.TransformFromTranslate(center.X, center.Y).
			PreScale(1, ry/rx).
			PreConcat(okage.TransformFromTranslate(-center.X, -center.Y))
		origin.Y = center.Y + (origin.Y-center.Y)*rx/ry
		shader, ok = okage.NewRadialGradient(origin, 0, center, rx, stops, spread, t.PreConcat(scale))
	}
	if !ok {
		// a degenerate gradient fills with its last color
		return solid(last), nil
	}
	return &brush{shader: shader}, nil
}

// imageBrush returns the brush of an image. Its Viewbox in the image maps
// onto its Viewport in the user space. ivy takes the units of a Viewbox as
// pixels of the image at 96 dots per inch, unless the Viewbox starts at the
// origin, and then it covers the whole image (D5).
func (r *renderer) imageBrush(n roxmltree.Node, res *resources, opacity float32) (*brush, error) {
	img, err := r.doc.image(r.ctx, resolve(r.part, attr(n, "ImageSource")))
	if err != nil {
		return nil, err
	}
	viewbox, ok1 := parseRect(attr(n, "Viewbox"))
	viewport, ok2 := parseRect(attr(n, "Viewport"))
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("image brush without a viewbox or a viewport: %w", ivy.ErrFormat)
	}
	size := img.Bounds().Size()
	unitX, unitY := float32(1), float32(1)
	if viewbox.X() == 0 && viewbox.Y() == 0 {
		unitX, unitY = viewbox.Width()/float32(size.X), viewbox.Height()/float32(size.Y)
	}
	sx, sy := viewport.Width()/viewbox.Width(), viewport.Height()/viewbox.Height()
	dst, ok := okage.RectFromXYWH(
		viewport.X()-viewbox.X()*sx, viewport.Y()-viewbox.Y()*sy,
		float32(size.X)*unitX*sx, float32(size.Y)*unitY*sy,
	)
	if !ok {
		return nil, fmt.Errorf("image brush of an empty image: %w", ivy.ErrFormat)
	}
	t := r.brushTransform(n, res)
	switch tile := attr(n, "TileMode"); tile {
	case "", "None":
		return &brush{img: img, dst: dst, viewport: viewport, t: t, opacity: opacity}, nil
	default:
		pix, err := raster.Pixmap(img)
		if err != nil {
			return nil, err
		}
		spread := okage.SpreadModeRepeat
		if strings.HasPrefix(tile, "Flip") {
			spread = okage.SpreadModeReflect
		}
		// the pattern maps the pixels of the image onto dst, and repeats
		pattern := t.PreConcat(okage.TransformFromTranslate(dst.X(), dst.Y())).
			PreScale(dst.Width()/float32(size.X), dst.Height()/float32(size.Y))
		shader := okage.NewPattern(pix.AsRef(), spread, okage.FilterQualityBilinear, opacity, pattern)
		return &brush{shader: shader, pix: pix}, nil
	}
}

// propertyElement returns the property element <owner.name> of n itself.
func propertyElement(n roxmltree.Node, name string) (roxmltree.Node, bool) {
	want := local(n) + "." + name
	for _, c := range elements(n) {
		if local(c) == want {
			return c, true
		}
	}
	return roxmltree.Node{}, false
}

// pointAttr parses the point of the attribute name of n, "x,y".
func pointAttr(n roxmltree.Node, name string) okage.Point {
	v, err := numbers(attr(n, name))
	if err != nil || len(v) != 2 {
		return okage.Point{}
	}
	return okage.PointFromXY(v[0], v[1])
}

// number parses a number, and returns def if s is not a number.
func number(s string, def float32) float32 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return def
	}
	return float32(v)
}
