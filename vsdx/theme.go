package vsdx

import (
	"strconv"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/ooxml"
)

// theme is the theme of a Visio drawing: the theme of DrawingML, with the
// extensions of Visio 2013 ([MS-VSDX] 2.5): the styles of the connectors,
// the variations of the colors and of the styles, the arrows and the
// patterns of the lines, and the colors of the fonts.
type theme struct {
	// dml is the theme of DrawingML, for its colors and its fonts.
	dml *drawingml.Theme
	// fills, lines and effects are the lists of the format scheme, and
	// connFills and connLines those of the scheme of the connectors.
	fills, lines, effects []roxmltree.Node
	connFills, connLines  []roxmltree.Node
	// lineEx and connLineEx are the arrows, the patterns and the rounding
	// of the lines of the two schemes.
	lineEx, connLineEx []lineEx
	// fillPatterns are the patterns of the fills of the format scheme.
	fillPatterns []int
	// fonts and connFonts are the colors and the styles of the text.
	fonts, connFonts []fontProps
	// varColors are the variation colors, 7 for each variant.
	varColors [][]drawingml.RGBA
	// varStyles are the variation styles, 4 for each variant.
	varStyles [][]varStyle
}

// lineEx is a vt:lineEx of a theme: the arrows and the pattern of a line
// style. The rounding of the corners of the theme is left out.
type lineEx struct {
	begin, beginSize, end, endSize, pattern int
}

// fontProps is a vt:fontProps of a theme: the style and the color of the
// text of a font style.
type fontProps struct {
	style int
	// color is the vt:color element, which holds a color of DrawingML.
	color roxmltree.Node
}

// varStyle is a vt:varStyle of a theme: the indexes of the styles of a
// variation, from 1.
type varStyle struct {
	fill, line, effect, font int
}

// maxThemeStyles bounds the number of the entries of each list of a theme.
const maxThemeStyles = 64

// parseTheme reads a theme part of Visio.
func parseTheme(doc *roxmltree.Document) (*theme, error) {
	dml, err := drawingml.ParseTheme(doc)
	if err != nil {
		return nil, err
	}
	t := &theme{dml: dml}
	colors := &drawingml.Colors{Theme: dml}
	for n := range doc.RootElement().Descendants() {
		if !n.IsElement() {
			continue
		}
		switch n.TagName().Name() {
		case "fmtScheme":
			t.fills, t.lines, t.effects = styleLists(n)
		case "fmtConnectorScheme":
			t.connFills, t.connLines, _ = styleLists(n)
		case "fmtConnectorSchemeLineStyles":
			t.connLineEx = parseLineEx(n)
		case "fmtSchemeLineStyles":
			t.lineEx = parseLineEx(n)
		case "fillStyles":
			for e := range elements(n) {
				if e.TagName().Name() == "fillProps" && len(t.fillPatterns) < maxThemeStyles {
					t.fillPatterns = append(t.fillPatterns, attrInt(e, "pattern", 1))
				}
			}
		case "connectorFontStyles":
			t.connFonts = parseFontProps(n)
		case "fontStyles":
			t.fonts = parseFontProps(n)
		case "variationClrScheme":
			if len(t.varColors) >= maxThemeStyles {
				continue
			}
			var list []drawingml.RGBA
			for e := range elements(n) {
				if c, ok := colors.In(e); ok && len(list) < 7 {
					list = append(list, c)
				}
			}
			t.varColors = append(t.varColors, list)
		case "variationStyleScheme":
			if len(t.varStyles) >= maxThemeStyles {
				continue
			}
			var list []varStyle
			for e := range elements(n) {
				if e.TagName().Name() == "varStyle" && len(list) < 4 {
					list = append(list, varStyle{
						fill:   attrInt(e, "fillIdx", 1),
						line:   attrInt(e, "lineIdx", 1),
						effect: attrInt(e, "effectIdx", 1),
						font:   attrInt(e, "fontIdx", 1),
					})
				}
			}
			t.varStyles = append(t.varStyles, list)
		}
	}
	return t, nil
}

// elements yields the child elements of n.
func elements(n roxmltree.Node) func(func(roxmltree.Node) bool) {
	return func(yield func(roxmltree.Node) bool) {
		for e := range n.Children() {
			if e.IsElement() && !yield(e) {
				return
			}
		}
	}
}

// styleLists returns the elements of the lists of the fills, the lines and
// the effects of a format scheme.
func styleLists(n roxmltree.Node) (fills, lines, effects []roxmltree.Node) {
	for e := range elements(n) {
		var list *[]roxmltree.Node
		switch e.TagName().Name() {
		case "fillStyleLst":
			list = &fills
		case "lnStyleLst":
			list = &lines
		case "effectStyleLst":
			list = &effects
		default:
			continue
		}
		for c := range elements(e) {
			if len(*list) < maxThemeStyles {
				*list = append(*list, c)
			}
		}
	}
	return fills, lines, effects
}

// parseLineEx reads the vt:lineEx of the vt:lineStyle elements of n.
func parseLineEx(n roxmltree.Node) []lineEx {
	var list []lineEx
	for ls := range elements(n) {
		for e := range elements(ls) {
			if e.TagName().Name() != "lineEx" || len(list) >= maxThemeStyles {
				continue
			}
			list = append(list, lineEx{
				begin:     attrInt(e, "start", 0),
				beginSize: attrInt(e, "startSize", 2),
				end:       attrInt(e, "end", 0),
				endSize:   attrInt(e, "endSize", 2),
				pattern:   attrInt(e, "pattern", 1),
			})
		}
	}
	return list
}

// parseFontProps reads the vt:fontProps elements of n.
func parseFontProps(n roxmltree.Node) []fontProps {
	var list []fontProps
	for e := range elements(n) {
		if e.TagName().Name() != "fontProps" || len(list) >= maxThemeStyles {
			continue
		}
		p := fontProps{style: attrInt(e, "style", 0)}
		for c := range elements(e) {
			if c.TagName().Name() == "color" {
				p.color = c
			}
		}
		list = append(list, p)
	}
	return list
}

// attrInt returns the attribute of n with the name as an integer, or def.
func attrInt(n roxmltree.Node, name string, def int) int {
	v, err := strconv.Atoi(ooxml.Attr(n, name))
	if err != nil {
		return def
	}
	return v
}

// themeColors are the names of the colors of a theme, by the index of a
// cell such as QuickStyleFillColor.
var themeColors = [...]string{"dk1", "lt1", "accent1", "accent2", "accent3", "accent4", "accent5", "accent6"}

// color returns the color of the index of a cell such as
// QuickStyleFillColor: 0 to 7 for the colors of the scheme, and 100 to 106
// for the colors of the variant. A nil theme gives the colors of the Office
// theme of Visio 2013.
func (t *theme) color(idx, variant int) drawingml.RGBA {
	if idx >= 0 && idx < len(themeColors) {
		if t != nil {
			if c, ok := t.dml.Color(themeColors[idx]); ok {
				return drawingml.RGBAFrom(c)
			}
		}
		return officeColors[idx]
	}
	v := 0
	if idx >= 100 && idx <= 106 {
		v = idx - 100
	}
	if t != nil && len(t.varColors) > 0 {
		list := t.varColors[min(max(variant, 0), len(t.varColors)-1)]
		if v < len(list) {
			return list[v]
		}
		if len(list) > 0 {
			return list[0]
		}
	}
	return officeColors[2+v%6]
}

// officeColors are the colors of the Office theme: dk1, lt1, and the six
// accents.
var officeColors = [...]drawingml.RGBA{
	hex(0x000000), hex(0xffffff), hex(0x5b9bd5), hex(0xed7d31),
	hex(0xa5a5a5), hex(0xffc000), hex(0x4472c4), hex(0x70ad47),
}

// hex returns the opaque color 0xRRGGBB.
func hex(v uint32) drawingml.RGBA {
	return drawingml.RGBA{R: float64(v>>16&0xff) / 255, G: float64(v>>8&0xff) / 255, B: float64(v&0xff) / 255, A: 1}
}

// styleIndexes returns the indexes of the fill, the line, the effect and
// the font style of a shape, from 1, for the cells QuickStyleFillMatrix and
// the others. A matrix of 100 takes the style of the variant.
func (t *theme) styleIndexes(q quickStyle, variant int) varStyle {
	vs := varStyle{fill: 1, line: 1, effect: 1, font: 1}
	if t != nil && len(t.varStyles) > 0 {
		list := t.varStyles[min(max(variant, 0), len(t.varStyles)-1)]
		if len(list) > 0 {
			vs = list[0]
		}
	}
	matrix := func(m, def int) int {
		if m >= 1 && m <= maxThemeStyles {
			return m
		}
		return def
	}
	return varStyle{
		fill:   matrix(q.fillMatrix, vs.fill),
		line:   matrix(q.lineMatrix, vs.line),
		effect: matrix(q.effectsMatrix, vs.effect),
		font:   matrix(q.fontMatrix, vs.font),
	}
}

// pick returns the element of the list with the index, from 1, or false.
func pick[T any](list []T, idx int) (T, bool) {
	if idx < 1 || idx > len(list) {
		var zero T
		return zero, false
	}
	return list[idx-1], true
}
