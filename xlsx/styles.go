package xlsx

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/numfmt"
	"github.com/xo/ivy/internal/sheet"
)

// styles holds the styles of a workbook, from styles.xml (ECMA-376 Part 1,
// 18.8).
type styles struct {
	theme *drawingml.Theme
	// palette is the indexed palette of the colors.
	palette []color.NRGBA
	// numFmts are the format codes of the file, by their number.
	numFmts map[int]string
	// codes are the format codes of the styles of the cells, by their
	// index.
	codes   []string
	fonts   []sheet.Font
	fills   []color.NRGBA
	borders [][4]sheet.Border
	// cells are the styles of the cells, the xf elements of cellXfs, and
	// formats their format codes.
	cells   []sheet.Style
	formats []*numfmt.Format
	// dxfs are the differential formats of the conditional formats.
	dxfs []sheet.Diff
}

// defaultPalette is the palette of the indexed colors of Excel. The index
// 64 is the color of the text of the system, and 65 the color of the
// background of the system.
var defaultPalette = []color.NRGBA{
	rgb(0x000000), rgb(0xffffff), rgb(0xff0000), rgb(0x00ff00), rgb(0x0000ff), rgb(0xffff00), rgb(0xff00ff), rgb(0x00ffff),
	rgb(0x000000), rgb(0xffffff), rgb(0xff0000), rgb(0x00ff00), rgb(0x0000ff), rgb(0xffff00), rgb(0xff00ff), rgb(0x00ffff),
	rgb(0x800000), rgb(0x008000), rgb(0x000080), rgb(0x808000), rgb(0x800080), rgb(0x008080), rgb(0xc0c0c0), rgb(0x808080),
	rgb(0x9999ff), rgb(0x993366), rgb(0xffffcc), rgb(0xccffff), rgb(0x660066), rgb(0xff8080), rgb(0x0066cc), rgb(0xccccff),
	rgb(0x000080), rgb(0xff00ff), rgb(0xffff00), rgb(0x00ffff), rgb(0x800080), rgb(0x800000), rgb(0x008080), rgb(0x0000ff),
	rgb(0x00ccff), rgb(0xccffff), rgb(0xccffcc), rgb(0xffff99), rgb(0x99ccff), rgb(0xff99cc), rgb(0xcc99ff), rgb(0xffcc99),
	rgb(0x3366ff), rgb(0x33cccc), rgb(0x99cc00), rgb(0xffcc00), rgb(0xff9900), rgb(0xff6600), rgb(0x666699), rgb(0x969696),
	rgb(0x003366), rgb(0x339966), rgb(0x003300), rgb(0x333300), rgb(0x993300), rgb(0x993366), rgb(0x333399), rgb(0x333333),
	rgb(0x000000), rgb(0xffffff),
}

// rgb returns the opaque color of 0xRRGGBB.
func rgb(v uint32) color.NRGBA {
	return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

// themeNames are the colors of the theme by their index in SpreadsheetML,
// which swaps the dark and the light colors of the scheme.
var themeNames = []string{"lt1", "dk1", "lt2", "dk2", "accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "hlink", "folHlink"}

// defaultFont is the font of a workbook without fonts.
var defaultFont = sheet.Font{Family: "Calibri", Size: 11}

// newStyles returns the styles of a workbook without a styles part.
func newStyles(theme *drawingml.Theme) *styles {
	return &styles{
		theme:   theme,
		palette: defaultPalette,
		numFmts: make(map[int]string),
	}
}

// read reads the styles part.
func (s *styles) read(doc *roxmltree.Document) {
	root := doc.RootElement()
	// The palette comes first, because the other elements name its
	// colors.
	if colors, ok := child(root, "colors"); ok {
		if idx, ok := child(colors, "indexedColors"); ok {
			palette := append([]color.NRGBA(nil), defaultPalette...)
			i := 0
			for c := range children(idx, "rgbColor") {
				if i < len(palette) {
					if v, ok := argb(attr(c, "rgb")); ok {
						palette[i] = v
					}
				}
				i++
			}
			s.palette = palette
		}
	}
	for n := range root.Children() {
		switch local(n) {
		case "numFmts":
			for f := range children(n, "numFmt") {
				s.numFmts[integer(f, "numFmtId", -1)] = attr(f, "formatCode")
			}
		case "fonts":
			for f := range children(n, "font") {
				s.fonts = append(s.fonts, s.font(f))
			}
		case "fills":
			for f := range children(n, "fill") {
				s.fills = append(s.fills, s.fill(f))
			}
		case "borders":
			for b := range children(n, "border") {
				s.borders = append(s.borders, s.border(b))
			}
		case "dxfs":
			for d := range children(n, "dxf") {
				if len(s.dxfs) < maxCondRules {
					s.dxfs = append(s.dxfs, s.dxf(d))
				}
			}
		}
	}
	if xfs, ok := child(root, "cellXfs"); ok {
		for xf := range children(xfs, "xf") {
			s.addXf(xf)
		}
	}
}

// defaultFont returns the font of the workbook: the first font.
func (s *styles) defaultFont() sheet.Font {
	if len(s.fonts) > 0 {
		f := s.fonts[0]
		if f.Size <= 0 {
			f.Size = 11
		}
		if f.Family == "" {
			f.Family = "Calibri"
		}
		return f
	}
	return defaultFont
}

// addXf adds the style of a cell of cellXfs.
func (s *styles) addXf(xf roxmltree.Node) {
	var st sheet.Style
	if i := integer(xf, "fontId", 0); i >= 0 && i < len(s.fonts) {
		st.Font = s.fonts[i]
	} else {
		st.Font = s.defaultFont()
	}
	if i := integer(xf, "fillId", 0); i >= 0 && i < len(s.fills) {
		st.Fill = s.fills[i]
	}
	if i := integer(xf, "borderId", 0); i >= 0 && i < len(s.borders) {
		st.Borders = s.borders[i]
	}
	if a, ok := child(xf, "alignment"); ok {
		st.HAlign = attr(a, "horizontal")
		if st.HAlign == "general" {
			st.HAlign = ""
		}
		st.VAlign = attr(a, "vertical")
		st.Wrap = flag(a, "wrapText", false)
		st.Shrink = flag(a, "shrinkToFit", false)
		// an indent is about three spaces, 9 pixels at 100%
		st.Indent = math.Min(250, math.Max(0, number(a, "indent", 0))) * 6.75
		rot := integer(a, "textRotation", 0)
		switch {
		case rot == 255:
			st.Rotation = 255
		case rot > 90 && rot <= 180:
			st.Rotation = 90 - rot
		case rot >= 0 && rot <= 90:
			st.Rotation = rot
		}
	}
	s.cells = append(s.cells, st)
	code := s.code(integer(xf, "numFmtId", 0))
	s.formats = append(s.formats, numfmt.Parse(code))
	s.codes = append(s.codes, code)
}

// code returns the format code of the number id.
func (s *styles) code(id int) string {
	if c, ok := s.numFmts[id]; ok {
		return c
	}
	if c, ok := numfmt.Builtin(id); ok {
		return c
	}
	switch {
	case id >= 27 && id <= 36, id >= 50 && id <= 58:
		// the dates of the locales of East Asia
		return "yyyy-mm-dd"
	}
	return "General"
}

// format returns the format of the style of a cell.
func (s *styles) format(i int) *numfmt.Format {
	if i >= 0 && i < len(s.formats) {
		return s.formats[i]
	}
	return numfmt.Parse("General")
}

// font reads a font element, or the properties rPr of a run of rich text.
func (s *styles) font(n roxmltree.Node) sheet.Font {
	var f sheet.Font
	for c := range n.Children() {
		switch local(c) {
		case "name", "rFont":
			f.Family = attr(c, "val")
		case "sz":
			f.Size = math.Min(409, math.Max(0, number(c, "val", 0)))
		case "b":
			f.Bold = flag(c, "val", true)
		case "i":
			f.Italic = flag(c, "val", true)
		case "strike":
			f.Strike = flag(c, "val", true)
		case "u":
			switch attr(c, "val") {
			case "", "single", "singleAccounting":
				f.Underline = 1
			case "double", "doubleAccounting":
				f.Underline = 2
			}
		case "vertAlign":
			switch attr(c, "val") {
			case "superscript":
				f.VertAlign = 1
			case "subscript":
				f.VertAlign = 2
			}
		case "color":
			if v, ok := s.color(c); ok {
				f.Color = v
			}
		case "family":
			switch attr(c, "val") {
			case "1":
				f.Generic = "serif"
			case "3":
				f.Generic = "monospace"
			}
		}
	}
	return f
}

// patternCover is the share of the foreground color of each pattern of a
// fill. A pattern is drawn as the blend of its two colors, at the quality
// of a review.
var patternCover = map[string]float64{
	"darkGray": 0.75, "mediumGray": 0.5, "lightGray": 0.25, "gray125": 0.125, "gray0625": 0.0625,
	"darkHorizontal": 0.5, "darkVertical": 0.5, "darkDown": 0.5, "darkUp": 0.5, "darkGrid": 0.5, "darkTrellis": 0.75,
	"lightHorizontal": 0.25, "lightVertical": 0.25, "lightDown": 0.25, "lightUp": 0.25, "lightGrid": 0.25, "lightTrellis": 0.25,
}

// fill reads a fill element: its color, or transparent.
func (s *styles) fill(n roxmltree.Node) color.NRGBA {
	if p, ok := child(n, "patternFill"); ok {
		kind := attr(p, "patternType")
		fg, hasFg := color.NRGBA{}, false
		bg, hasBg := color.NRGBA{}, false
		if c, ok := child(p, "fgColor"); ok {
			fg, hasFg = s.color(c)
		}
		if c, ok := child(p, "bgColor"); ok {
			bg, hasBg = s.color(c)
		}
		switch kind {
		case "", "none":
			return color.NRGBA{}
		case "solid":
			if !hasFg {
				return rgb(0x000000)
			}
			return fg
		}
		cover, ok := patternCover[kind]
		if !ok {
			return color.NRGBA{}
		}
		if !hasFg {
			fg = rgb(0x000000)
		}
		if !hasBg {
			bg = rgb(0xffffff)
		}
		return blend(fg, bg, cover)
	}
	if g, ok := child(n, "gradientFill"); ok {
		var r, gr, b, k float64
		for stop := range children(g, "stop") {
			if c, ok := child(stop, "color"); ok {
				if v, ok := s.color(c); ok {
					r, gr, b, k = r+float64(v.R), gr+float64(v.G), b+float64(v.B), k+1
				}
			}
		}
		if k > 0 {
			return color.NRGBA{uint8(r / k), uint8(gr / k), uint8(b / k), 255}
		}
	}
	return color.NRGBA{}
}

// blend returns the color of a share t of fg over bg.
func blend(fg, bg color.NRGBA, t float64) color.NRGBA {
	mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a)*t + float64(b)*(1-t))) }
	return color.NRGBA{mix(fg.R, bg.R), mix(fg.G, bg.G), mix(fg.B, bg.B), 255}
}

// borderStyles maps the styles of the borders of SpreadsheetML to a width
// in points and a line of CSS.
var borderStyles = map[string]sheet.Border{
	"hair":             {Width: 0.5, Line: "dotted"},
	"thin":             {Width: 0.75, Line: "solid"},
	"medium":           {Width: 1.5, Line: "solid"},
	"thick":            {Width: 2.25, Line: "solid"},
	"dashed":           {Width: 0.75, Line: "dashed"},
	"dotted":           {Width: 0.75, Line: "dotted"},
	"double":           {Width: 2.25, Line: "double"},
	"mediumDashed":     {Width: 1.5, Line: "dashed"},
	"dashDot":          {Width: 0.75, Line: "dashed"},
	"mediumDashDot":    {Width: 1.5, Line: "dashed"},
	"dashDotDot":       {Width: 0.75, Line: "dashed"},
	"mediumDashDotDot": {Width: 1.5, Line: "dashed"},
	"slantDashDot":     {Width: 1.5, Line: "dashed"},
}

// border reads a border element.
func (s *styles) border(n roxmltree.Node) [4]sheet.Border {
	var out [4]sheet.Border
	for c := range n.Children() {
		k := -1
		switch local(c) {
		case "top":
			k = 0
		case "right", "end":
			k = 1
		case "bottom":
			k = 2
		case "left", "start":
			k = 3
		}
		if k < 0 {
			continue
		}
		b, ok := borderStyles[attr(c, "style")]
		if !ok {
			continue
		}
		if cc, ok := child(c, "color"); ok {
			if v, ok := s.color(cc); ok {
				b.Color = v
			}
		}
		out[k] = b
	}
	return out
}

// color reads a color element of SpreadsheetML: rgb, theme with tint,
// indexed or auto. It returns false for auto and for a color that it
// cannot read.
func (s *styles) color(n roxmltree.Node) (color.NRGBA, bool) {
	var c color.NRGBA
	switch {
	case attr(n, "rgb") != "":
		v, ok := argb(attr(n, "rgb"))
		if !ok {
			return color.NRGBA{}, false
		}
		c = v
	case attr(n, "theme") != "":
		i := integer(n, "theme", -1)
		if i < 0 || i >= len(themeNames) {
			return color.NRGBA{}, false
		}
		v, ok := s.theme.Color(themeNames[i])
		if !ok {
			v, ok = defaultThemeColor(themeNames[i])
		}
		if !ok {
			return color.NRGBA{}, false
		}
		c = v
	case attr(n, "indexed") != "":
		i := integer(n, "indexed", -1)
		if i < 0 || i >= len(s.palette) {
			return color.NRGBA{}, false
		}
		c = s.palette[i]
	default:
		return color.NRGBA{}, false
	}
	if t := number(n, "tint", 0); t != 0 {
		c = sheet.Tint(c, t)
	}
	return c, true
}

// paletteColor returns the color of the number n of [Color n] of a format
// code: the color of index n+7 of the palette.
func (s *styles) paletteColor(n int) color.NRGBA {
	if i := n + 7; n > 0 && i < len(s.palette) {
		return s.palette[i]
	}
	return color.NRGBA{}
}

// defaultThemeColor returns a color of the theme of Office, for a workbook
// without a theme.
func defaultThemeColor(name string) (color.NRGBA, bool) {
	v, ok := map[string]uint32{
		"dk1": 0x000000, "lt1": 0xffffff, "dk2": 0x44546a, "lt2": 0xe7e6e6,
		"accent1": 0x4472c4, "accent2": 0xed7d31, "accent3": 0xa5a5a5, "accent4": 0xffc000,
		"accent5": 0x5b9bd5, "accent6": 0x70ad47, "hlink": 0x0563c1, "folHlink": 0x954f72,
	}[name]
	return rgb(v), ok
}

// argb parses a color of SpreadsheetML, AARRGGBB or RRGGBB. The alpha of
// SpreadsheetML is not used, as in Excel.
func argb(s string) (color.NRGBA, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 8 {
		s = s[2:]
	}
	if len(s) != 6 {
		return color.NRGBA{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.NRGBA{}, false
	}
	return rgb(uint32(v)), true
}
