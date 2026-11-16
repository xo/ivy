package docx

import (
	"strings"

	"github.com/xo/ivy/internal/drawingml"
)

// decl is a declaration of CSS.
type decl struct {
	name, value string
}

// highlights are the colors of w:highlight.
var highlights = map[string]string{
	"yellow": "#ffff00", "green": "#00ff00", "cyan": "#00ffff", "magenta": "#ff00ff",
	"blue": "#0000ff", "red": "#ff0000", "darkBlue": "#000080", "darkCyan": "#008080",
	"darkGreen": "#008000", "darkMagenta": "#800080", "darkRed": "#800000",
	"darkYellow": "#808000", "darkGray": "#808080", "lightGray": "#c0c0c0",
	"black": "#000000", "white": "#ffffff",
}

// lineRatios are the heights of a single line of the fonts that Office
// documents use most, as a multiple of the font size: the ascent, the
// descent and the line gap of the font. Word and LibreOffice multiply this
// height for the spacing of the lines, and CSS multiplies the font size.
var lineRatios = map[string]float64{
	"calibri": 1.2207, "carlito": 1.2207, "calibri light": 1.2207,
	"cambria": 1.1719, "caladea": 1.1719,
	"arial": 1.1499, "liberation sans": 1.1499, "helvetica": 1.1499,
	"times new roman": 1.1499, "liberation serif": 1.1499, "times": 1.1499,
	"courier new": 1.1328, "liberation mono": 1.1328, "courier": 1.1328,
	"aptos": 1.2,
}

// defaultLineRatio is the height of a single line of a font that is not in
// lineRatios.
const defaultLineRatio = 1.17

// font returns the name of a font of the properties: a font of the theme,
// such as "theme:minorHAnsi", resolves through the theme.
func (c *converter) font(v string) string {
	theme, ok := strings.CutPrefix(v, "theme:")
	if !ok {
		return v
	}
	if c.theme == nil {
		return ""
	}
	switch theme {
	case "majorHAnsi", "majorAscii":
		return c.theme.Major.Latin
	case "minorHAnsi", "minorAscii":
		return c.theme.Minor.Latin
	case "majorEastAsia":
		return c.theme.Major.EastAsian
	case "minorEastAsia":
		return c.theme.Minor.EastAsian
	case "majorBidi":
		return c.theme.Major.ComplexScript
	case "minorBidi":
		return c.theme.Minor.ComplexScript
	}
	return ""
}

// genericFamily returns the generic family of CSS of a font, from the font
// table or from its name.
func (c *converter) genericFamily(name string) string {
	lower := strings.ToLower(name)
	if g := c.generic[lower]; g != "" {
		return g
	}
	return guessGeneric(lower)
}

// guessGeneric returns the generic family of a font from its name, in lower
// case.
func guessGeneric(lower string) string {
	for _, k := range [...]string{"mono", "courier", "consol", "code"} {
		if strings.Contains(lower, k) {
			return "monospace"
		}
	}
	for _, k := range [...]string{"sans", "arial", "calibri", "carlito", "helvetica", "verdana", "tahoma", "segoe", "aptos", "gothic", "trebuchet"} {
		if strings.Contains(lower, k) {
			return "sans-serif"
		}
	}
	return "serif"
}

// family returns the value of font-family of the properties, or "".
func (c *converter) family(p props) string {
	latin := c.font(p["font"])
	ea := c.font(p["font.ea"])
	var parts []string
	if latin != "" {
		parts = append(parts, quoteFamily(latin))
	}
	if ea != "" && ea != latin {
		parts = append(parts, quoteFamily(ea))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ") + ", " + c.genericFamily(latin)
}

// lineRatio returns the height of a single line of the font of the
// properties.
func (c *converter) lineRatio(p props) float64 {
	if r, ok := lineRatios[strings.ToLower(c.font(p["font"]))]; ok {
		return r
	}
	return defaultLineRatio
}

// fontSize returns the size of the font of the properties in points.
func fontSize(p props) float64 {
	sz := p.num("sz", 20) / 2
	if sz <= 0 || sz > 1638 {
		sz = 10
	}
	return sz
}

// color returns the color of the text of the properties as CSS.
func (c *converter) color(p props) string {
	if v, ok := hexColor(p["color"]); ok {
		return v
	}
	if t := p["color.theme"]; t != "" {
		if v, ok := c.theme.Color(t); ok {
			return drawingml.RGBAFrom(v).CSS()
		}
	}
	return "#000000"
}

// hexColor returns the color of a value of WordprocessingML, such as
// "1F497D", as CSS. It returns false for "auto" and for another value.
func hexColor(s string) (string, bool) {
	v, ok := drawingml.ParseHex(s)
	if !ok {
		return "", false
	}
	return drawingml.RGBAFrom(v).CSS(), true
}

// inherited returns the declarations of the properties of a run that a
// child element inherits: the font, the size, the weight, the style, the
// color and the case.
func (c *converter) inherited(p props) []decl {
	size := fontSize(p)
	if va := p["vertAlign"]; va == "superscript" || va == "subscript" {
		size *= 0.65
	}
	out := []decl{
		{"font-family", c.family(p)},
		{"font-size", pt(size)},
		{"font-weight", map[bool]string{true: "bold", false: "normal"}[p.on("b")]},
		{"font-style", map[bool]string{true: "italic", false: "normal"}[p.on("i")]},
		{"color", c.color(p)},
		{"text-transform", map[bool]string{true: "uppercase", false: "none"}[p.on("caps")]},
		{"font-variant", map[bool]string{true: "small-caps", false: "normal"}[p.on("smallCaps") && !p.on("caps")]},
	}
	spacing := p.num("spacing", 0) / 20
	if spacing != 0 {
		out = append(out, decl{"letter-spacing", pt(spacing)})
	} else {
		out = append(out, decl{"letter-spacing", "normal"})
	}
	return out
}

// own returns the declarations of the properties of a run that a child
// element does not inherit: the lines, the highlight and the position.
func own(p props) []decl {
	var out []decl
	var lines []string
	style := ""
	if u := p["u"]; u != "" && u != "none" {
		lines = append(lines, "underline")
		switch {
		case u == "double":
			style = "double"
		case strings.HasPrefix(u, "dot"):
			style = "dotted"
		case strings.HasPrefix(u, "dash"):
			style = "dashed"
		case strings.HasPrefix(u, "wav"):
			style = "wavy"
		}
	}
	if p.on("strike") || p.on("dstrike") {
		lines = append(lines, "line-through")
		if p.on("dstrike") && style == "" {
			style = "double"
		}
	}
	if len(lines) > 0 {
		out = append(out, decl{"text-decoration-line", strings.Join(lines, " ")})
		if style != "" {
			out = append(out, decl{"text-decoration-style", style})
		}
	}
	if h, ok := highlights[p["highlight"]]; ok {
		out = append(out, decl{"background-color", h})
	} else if v, ok := hexColor(p["shd"]); ok {
		out = append(out, decl{"background-color", v})
	}
	switch p["vertAlign"] {
	case "superscript":
		out = append(out, decl{"vertical-align", "super"})
	case "subscript":
		out = append(out, decl{"vertical-align", "sub"})
	default:
		if pos := p.num("position", 0) / 2; pos != 0 {
			out = append(out, decl{"vertical-align", pt(pos)})
		}
	}
	return out
}

// runStyle returns the style attribute of a run with the properties p in a
// parent with the properties parent.
func (c *converter) runStyle(p, parent props) string {
	if c.lineParent != nil {
		// the element of the paragraph has the font of its largest run
		parent = parent.over(props{})
		for _, k := range [...]string{"font", "font.ea", "sz"} {
			if v, ok := c.lineParent[k]; ok {
				parent[k] = v
			} else {
				delete(parent, k)
			}
		}
	}
	var s css
	have := map[string]string{}
	for _, d := range c.inherited(parent) {
		have[d.name] = d.value
	}
	for _, d := range c.inherited(p) {
		if have[d.name] != d.value {
			s.set(d.name, d.value)
		}
	}
	for _, d := range own(p) {
		s.set(d.name, d.value)
	}
	return s.String()
}

// border returns the value of a border of CSS for a border of
// WordprocessingML, "val sz color space", and the space in points.
func border(v string) (string, float64) {
	f := strings.Fields(v)
	if len(f) != 4 || f[0] == "nil" || f[0] == "none" {
		return "", 0
	}
	width := number(f[1], 4) / 8
	style := "solid"
	switch {
	case f[0] == "double", strings.HasPrefix(f[0], "thinThick"), strings.HasPrefix(f[0], "thickThin"), f[0] == "triple":
		style = "double"
		width = max(width*3, 1.5)
	case strings.HasPrefix(f[0], "dot") && !strings.HasPrefix(f[0], "dotDash"):
		style = "dotted"
	case strings.HasPrefix(f[0], "dash"):
		style = "dashed"
	}
	width = min(max(width, 0.25), 12)
	color, ok := hexColor(f[2])
	if !ok {
		color = "#000000"
	}
	return pt(width) + " " + style + " " + color, min(number(f[3], 0), 31)
}
