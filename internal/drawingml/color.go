package drawingml

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/tinycss2"
)

// RGBA is a color of DrawingML in sRGB, with each channel from 0 to 1. The
// transforms of a color, such as lumMod and tint, work on it before it
// becomes an okage.Color.
type RGBA struct {
	R, G, B, A float64
}

// Black and White are the opaque colors black and white.
var (
	Black = RGBA{0, 0, 0, 1}
	White = RGBA{1, 1, 1, 1}
)

// RGBAFrom returns the color c as an RGBA.
func RGBAFrom(c color.NRGBA) RGBA {
	return RGBA{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255, float64(c.A) / 255}
}

// Okage returns the color for okage, with each channel in its range.
func (c RGBA) Okage() okage.Color {
	return okage.ColorFromRGBAUnchecked(unit32(c.R), unit32(c.G), unit32(c.B), unit32(c.A))
}

// NRGBA returns the color with 8 bits for each channel.
func (c RGBA) NRGBA() color.NRGBA {
	return color.NRGBA{byte8(c.R), byte8(c.G), byte8(c.B), byte8(c.A)}
}

// CSS returns the color as a color of CSS, such as "rgba(255,0,0,0.5)".
func (c RGBA) CSS() string {
	n := c.NRGBA()
	return "rgba(" + strconv.Itoa(int(n.R)) + "," + strconv.Itoa(int(n.G)) + "," + strconv.Itoa(int(n.B)) + "," +
		strconv.FormatFloat(unit(c.A), 'f', 3, 64) + ")"
}

// WithAlpha returns the color with its alpha multiplied by a.
func (c RGBA) WithAlpha(a float64) RGBA {
	c.A = unit(c.A * a)
	return c
}

// Mix returns the color that is the fraction t of the way from c to d.
func (c RGBA) Mix(d RGBA, t float64) RGBA {
	return RGBA{c.R + (d.R-c.R)*t, c.G + (d.G-c.G)*t, c.B + (d.B-c.B)*t, c.A + (d.A-c.A)*t}
}

// unit returns v limited to the range from 0 to 1, and 0 for NaN.
func unit(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return min(max(v, 0), 1)
}

// unit32 returns v limited to the range from 0 to 1, as a float32.
func unit32(v float64) float32 {
	return float32(unit(v))
}

// byte8 returns v, from 0 to 1, as a byte.
func byte8(v float64) uint8 {
	return uint8(math.Round(unit(v) * 255))
}

// Colors resolves the colors of DrawingML: the colors of the theme through
// the color map of a slide, and the placeholder color of a style. The zero
// Colors resolves only the colors that hold their value.
type Colors struct {
	// Theme gives the colors of the scheme, or is nil.
	Theme *Theme
	// Map maps the names that a document uses, such as bg1 and tx1, to the
	// names of the scheme, such as lt1 and dk1. It is the color map of a
	// slide master, with the override of the slide. A nil Map maps bg1 to
	// lt1, tx1 to dk1, bg2 to lt2 and tx2 to dk2.
	Map map[string]string
	// Placeholder is the color phClr, which a style of the theme names. A
	// style reference of a shape sets it.
	Placeholder *RGBA
}

// WithPlaceholder returns a copy of c whose phClr is p.
func (c *Colors) WithPlaceholder(p RGBA) *Colors {
	d := *c
	d.Placeholder = &p
	return &d
}

// defaultMap is the color map of a document without one.
var defaultMap = map[string]string{"bg1": "lt1", "tx1": "dk1", "bg2": "lt2", "tx2": "dk2"}

// ParseColorMap reads the attributes of p:clrMap or a:overrideClrMapping,
// such as bg1="lt1".
func ParseColorMap(n roxmltree.Node) map[string]string {
	m := make(map[string]string)
	for a := range n.Attributes() {
		m[a.Name()] = a.Value()
	}
	return m
}

// scheme returns the color of the scheme with the name, through the color
// map.
func (c *Colors) scheme(name string) (RGBA, bool) {
	if name == "phClr" {
		if c.Placeholder != nil {
			return *c.Placeholder, true
		}
		return Black, true
	}
	m := c.Map
	if m == nil {
		m = defaultMap
	}
	if mapped, ok := m[name]; ok {
		name = mapped
	} else if mapped, ok := defaultMap[name]; ok {
		name = mapped
	}
	v, ok := c.Theme.Color(name)
	if !ok {
		// A theme without the color: black for the text and the dark
		// colors, white for the light colors.
		switch name {
		case "lt1", "lt2", "bg1", "bg2":
			return White, true
		}
		return Black, true
	}
	return RGBAFrom(v), true
}

// In returns the color of the first child of n that is a color element,
// such as the a:srgbClr of an a:solidFill.
func (c *Colors) In(n roxmltree.Node) (RGBA, bool) {
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		if v, ok := c.Of(e); ok {
			return v, true
		}
	}
	return RGBA{}, false
}

// Of returns the color of the color element n, such as a:srgbClr or
// a:schemeClr, with its transforms. It returns false if n is not a color
// element.
func (c *Colors) Of(n roxmltree.Node) (RGBA, bool) {
	var v RGBA
	switch n.TagName().Name() {
	case "srgbClr":
		h, ok := ParseHex(attr(n, "val", ""))
		if !ok {
			return RGBA{}, false
		}
		v = RGBAFrom(h)
	case "schemeClr":
		s, ok := c.scheme(attr(n, "val", ""))
		if !ok {
			return RGBA{}, false
		}
		v = s
	case "sysClr":
		if h, ok := ParseHex(attr(n, "lastClr", "")); ok {
			v = RGBAFrom(h)
			break
		}
		v = systemColor(attr(n, "val", ""))
	case "prstClr":
		p, ok := PresetColor(attr(n, "val", ""))
		if !ok {
			return RGBA{}, false
		}
		v = p
	case "hslClr":
		hue := numberAttr(n, "hue", 0) / 60000
		sat := pct(numberAttr(n, "sat", 0))
		lum := pct(numberAttr(n, "lum", 0))
		r, g, b := hslToRGB(hue/360, unit(sat), unit(lum))
		v = RGBA{r, g, b, 1}
	case "scrgbClr":
		v = RGBA{
			fromLinear(pct(numberAttr(n, "r", 0))),
			fromLinear(pct(numberAttr(n, "g", 0))),
			fromLinear(pct(numberAttr(n, "b", 0))),
			1,
		}
	default:
		return RGBA{}, false
	}
	return transform(v, n), true
}

// systemColor returns the usual value of a system color of Windows.
func systemColor(name string) RGBA {
	switch name {
	case "window", "highlightText", "captionText", "menu", "infoBk":
		return White
	case "btnFace", "menuBar", "3dLight", "scrollBar":
		return RGBA{0.94, 0.94, 0.94, 1}
	case "highlight", "hotLight":
		return RGBA{0, 0.47, 0.84, 1}
	case "grayText", "btnShadow":
		return RGBA{0.43, 0.43, 0.43, 1}
	}
	return Black
}

// PresetColor returns the color of a preset color of DrawingML, such as
// "dkBlue" or "coral". The names are the colors of CSS, with the prefixes
// dk, lt and med for dark, light and medium.
func PresetColor(name string) (RGBA, bool) {
	s := name
	for _, p := range [...][2]string{{"dk", "dark"}, {"lt", "light"}, {"med", "medium"}} {
		if rest, ok := strings.CutPrefix(s, p[0]); ok {
			s = p[1] + rest
			break
		}
	}
	s = strings.ToLower(s)
	if s == "" || strings.ContainsAny(s, " ()#,;") {
		return RGBA{}, false
	}
	c, _ := tinycss2.ParseColor3(tinycss2.ParseOneComponentValue(s))
	if c == nil {
		return RGBA{}, false
	}
	return RGBA{c.Red, c.Green, c.Blue, c.Alpha}, true
}

// transform applies the transforms of the color element n, in order.
func transform(c RGBA, n roxmltree.Node) RGBA {
	for t := range n.Children() {
		if !t.IsElement() {
			continue
		}
		val := pct(numberAttr(t, "val", 0))
		switch t.TagName().Name() {
		case "alpha":
			c.A = unit(val)
		case "alphaMod":
			c.A = unit(c.A * val)
		case "alphaOff":
			c.A = unit(c.A + val)
		case "lum":
			c = withHSL(c, func(h, s, _ float64) (float64, float64, float64) { return h, s, val })
		case "lumMod":
			c = withHSL(c, func(h, s, l float64) (float64, float64, float64) { return h, s, l * val })
		case "lumOff":
			c = withHSL(c, func(h, s, l float64) (float64, float64, float64) { return h, s, l + val })
		case "sat":
			c = withHSL(c, func(h, _, l float64) (float64, float64, float64) { return h, val, l })
		case "satMod":
			c = withSaturation(c, func(s float64) float64 { return s * val })
		case "satOff":
			c = withSaturation(c, func(s float64) float64 { return s + val })
		case "hue":
			deg := numberAttr(t, "val", 0) / 60000
			c = withHSL(c, func(_, s, l float64) (float64, float64, float64) { return deg / 360, s, l })
		case "hueMod":
			c = withHSL(c, func(h, s, l float64) (float64, float64, float64) { return h * val, s, l })
		case "hueOff":
			deg := numberAttr(t, "val", 0) / 60000
			c = withHSL(c, func(h, s, l float64) (float64, float64, float64) { return h + deg/360, s, l })
		case "tint":
			// a tint moves the color to white, in linear RGB
			c = withLinear(c, func(v float64) float64 { return v*val + (1 - val) })
		case "shade":
			// a shade moves the color to black, in linear RGB
			c = withLinear(c, func(v float64) float64 { return v * val })
		case "comp":
			c = withHSL(c, func(h, s, l float64) (float64, float64, float64) { return h + 0.5, s, l })
		case "inv":
			c.R, c.G, c.B = 1-c.R, 1-c.G, 1-c.B
		case "gray":
			y := 0.3*c.R + 0.59*c.G + 0.11*c.B
			c.R, c.G, c.B = y, y, y
		case "gamma":
			c = withLinear(c, fromLinear)
		case "invGamma":
			c = withLinear(c, toLinear)
		case "red":
			c.R = unit(val)
		case "redMod":
			c.R = unit(c.R * val)
		case "redOff":
			c.R = unit(c.R + val)
		case "green":
			c.G = unit(val)
		case "greenMod":
			c.G = unit(c.G * val)
		case "greenOff":
			c.G = unit(c.G + val)
		case "blue":
			c.B = unit(val)
		case "blueMod":
			c.B = unit(c.B * val)
		case "blueOff":
			c.B = unit(c.B + val)
		}
	}
	return c
}

// pct returns a percentage of DrawingML, in 1000ths of a percent, as a
// fraction. It also reads a percentage that ends in "%", as ECMA-376 2nd
// edition and later write it.
func pct(v float64) float64 {
	return v / 100000
}

// numberAttr returns the attribute of n with the name as a number, or def. A
// value that ends in "%" is a percentage, and returns 1000 times its number,
// as the units of DrawingML.
func numberAttr(n roxmltree.Node, name string, def float64) float64 {
	s := attr(n, name, "")
	if s == "" {
		return def
	}
	scale := 1.0
	if rest, ok := strings.CutSuffix(s, "%"); ok {
		s, scale = rest, 1000
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return def
	}
	return v * scale
}

// withHSL changes c in the space of hue, saturation and lightness. The hue
// is a fraction of the circle.
func withHSL(c RGBA, f func(h, s, l float64) (float64, float64, float64)) RGBA {
	h, s, l := rgbToHSL(c.R, c.G, c.B)
	h, s, l = f(h, s, l)
	h -= math.Floor(h)
	c.R, c.G, c.B = hslToRGB(h, unit(s), unit(l))
	return c
}

// withSaturation changes the saturation of c in the space of hue,
// saturation and lightness. As PowerPoint does, it does not limit the new
// saturation to 1, but limits each channel of the color that it gives.
// The stops of the gradients of SmartArt in poi/pptx/custom-geometry.pptx,
// such as a tint of 50% and a satMod of 300% of F79646, have the colors of
// the PDF of PowerPoint only so: FFBE86, and not FFD2BD.
func withSaturation(c RGBA, f func(s float64) float64) RGBA {
	h, s, l := rgbToHSL(c.R, c.G, c.B)
	s = max(f(s), 0)
	c.R, c.G, c.B = hslToRGB(h, s, unit(l))
	c.R, c.G, c.B = unit(c.R), unit(c.G), unit(c.B)
	return c
}

// withLinear changes each channel of c in linear RGB.
func withLinear(c RGBA, f func(v float64) float64) RGBA {
	c.R = fromLinear(unit(f(toLinear(c.R))))
	c.G = fromLinear(unit(f(toLinear(c.G))))
	c.B = fromLinear(unit(f(toLinear(c.B))))
	return c
}

// toLinear converts a channel of sRGB to linear RGB.
func toLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// fromLinear converts a channel of linear RGB to sRGB.
func fromLinear(v float64) float64 {
	v = unit(v)
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

// rgbToHSL converts a color to hue, saturation and lightness, each from 0
// to 1.
func rgbToHSL(r, g, b float64) (h, s, l float64) {
	hi, lo := max(r, g, b), min(r, g, b)
	l = (hi + lo) / 2
	d := hi - lo
	if d == 0 {
		return 0, 0, l
	}
	if l < 0.5 {
		s = d / (hi + lo)
	} else {
		s = d / (2 - hi - lo)
	}
	switch hi {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

// hslToRGB converts hue, saturation and lightness, each from 0 to 1, to a
// color.
func hslToRGB(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		return l, l, l
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	return hue(p, q, h+1.0/3), hue(p, q, h), hue(p, q, h-1.0/3)
}

// hue is a step of hslToRGB.
func hue(p, q, t float64) float64 {
	t -= math.Floor(t)
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	}
	return p
}
