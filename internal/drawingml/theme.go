package drawingml

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
)

// Theme is the theme of an Office Open XML package (ECMA-376 Part 1,
// 20.1.6): the colors of its color scheme and the fonts of its font scheme.
// Word, Excel and PowerPoint name a color or a font of the theme instead of
// a value.
type Theme struct {
	// Colors maps the name of each color of the scheme to its color: dk1,
	// lt1, dk2, lt2, accent1 to accent6, hlink and folHlink.
	Colors map[string]color.NRGBA
	// Major and Minor are the fonts of the headings and of the body text.
	Major, Minor Fonts
	// Pictures gives the pictures of the theme part, for the picture fills
	// of its styles. It is nil when the format does not read them, and
	// those fills then have no picture.
	Pictures Pictures

	// fills, lines, effects and backgrounds are the elements of the lists
	// of the format scheme, which a style reference of a shape names by its
	// index: a:fillStyleLst, a:lnStyleLst, a:effectStyleLst and
	// a:bgFillStyleLst.
	fills, lines, effects, backgrounds []roxmltree.Node
}

// maxStyles is the number of the styles of each list of a format scheme
// that a theme keeps.
const maxStyles = 64

// Fonts are the fonts of one part of a font scheme, for each kind of
// script.
type Fonts struct {
	Latin, EastAsian, ComplexScript string
}

// Color returns the color of the scheme with the name name. It also takes
// the names that a document uses for the same colors: tx1 and tx2 for dk1
// and dk2, bg1 and bg2 for lt1 and lt2, and the names of WordprocessingML,
// such as "text1" and "background1", "dark1" and "light1", and
// "hyperlink".
func (t *Theme) Color(name string) (color.NRGBA, bool) {
	if t == nil {
		return color.NRGBA{}, false
	}
	if c, ok := t.Colors[name]; ok {
		return c, true
	}
	alias := map[string]string{
		"tx1": "dk1", "tx2": "dk2", "bg1": "lt1", "bg2": "lt2",
		"text1": "dk1", "text2": "dk2", "background1": "lt1", "background2": "lt2",
		"dark1": "dk1", "dark2": "dk2", "light1": "lt1", "light2": "lt2",
		"hyperlink": "hlink", "followedHyperlink": "folHlink",
	}[name]
	c, ok := t.Colors[alias]
	return c, ok
}

// ParseTheme parses a theme part, such as ppt/theme/theme1.xml. A color or
// a font that the part does not give stays missing. It returns an error
// that wraps ivy.ErrFormat for a part that is not a theme.
func ParseTheme(doc *roxmltree.Document) (*Theme, error) {
	root := doc.RootElement()
	if name := root.TagName().Name(); name != "theme" && name != "themeOverride" {
		return nil, fmt.Errorf("theme with the root %q: %w", name, ivy.ErrFormat)
	}
	t := &Theme{Colors: make(map[string]color.NRGBA)}
	for n := range root.Descendants() {
		if !n.IsElement() {
			continue
		}
		switch n.TagName().Name() {
		case "clrScheme":
			for c := range n.Children() {
				if !c.IsElement() {
					continue
				}
				if v, ok := schemeColor(c); ok {
					t.Colors[c.TagName().Name()] = v
				}
			}
		case "fillStyleLst":
			t.fills = styleList(n)
		case "lnStyleLst":
			t.lines = styleList(n)
		case "effectStyleLst":
			t.effects = styleList(n)
		case "bgFillStyleLst":
			t.backgrounds = styleList(n)
		case "majorFont":
			t.Major = parseFonts(n)
		case "minorFont":
			t.Minor = parseFonts(n)
		}
	}
	return t, nil
}

// styleList returns the child elements of a list of a format scheme.
func styleList(n roxmltree.Node) []roxmltree.Node {
	var list []roxmltree.Node
	for c := range n.Children() {
		if c.IsElement() && len(list) < maxStyles {
			list = append(list, c)
		}
	}
	return list
}

// style returns the element of the list with the index of a style
// reference, from 1.
func style(list []roxmltree.Node, idx int) (roxmltree.Node, bool) {
	if idx < 1 || idx > len(list) {
		return roxmltree.Node{}, false
	}
	return list[idx-1], true
}

// FillStyle returns the fill of the index of a:fillRef or a:bgRef: from 1
// for a:fillStyleLst, and from 1001 for a:bgFillStyleLst. The element is a
// fill element, such as a:solidFill, whose colors can be phClr.
func (t *Theme) FillStyle(idx int) (roxmltree.Node, bool) {
	if t == nil {
		return roxmltree.Node{}, false
	}
	if idx > 1000 {
		return style(t.backgrounds, idx-1000)
	}
	return style(t.fills, idx)
}

// StylePictures returns the pictures of the styles of the theme, or nil.
func (t *Theme) StylePictures() Pictures {
	if t == nil {
		return nil
	}
	return t.Pictures
}

// LineStyle returns the a:ln of the index of a:lnRef, from 1.
func (t *Theme) LineStyle(idx int) (roxmltree.Node, bool) {
	if t == nil {
		return roxmltree.Node{}, false
	}
	return style(t.lines, idx)
}

// EffectStyle returns the a:effectStyle of the index of a:effectRef, from
// 1.
func (t *Theme) EffectStyle(idx int) (roxmltree.Node, bool) {
	if t == nil {
		return roxmltree.Node{}, false
	}
	return style(t.effects, idx)
}

// schemeColor reads the color of an element of a color scheme, which holds
// an a:srgbClr or an a:sysClr. A system color takes its last color.
func schemeColor(n roxmltree.Node) (color.NRGBA, bool) {
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		switch c.TagName().Name() {
		case "srgbClr":
			return ParseHex(attr(c, "val", ""))
		case "sysClr":
			if v, ok := ParseHex(attr(c, "lastClr", "")); ok {
				return v, true
			}
			switch attr(c, "val", "") {
			case "window":
				return color.NRGBA{255, 255, 255, 255}, true
			case "windowText":
				return color.NRGBA{0, 0, 0, 255}, true
			}
		}
	}
	return color.NRGBA{}, false
}

// parseFonts reads the fonts of a:majorFont or a:minorFont.
func parseFonts(n roxmltree.Node) Fonts {
	var f Fonts
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		switch c.TagName().Name() {
		case "latin":
			f.Latin = attr(c, "typeface", "")
		case "ea":
			f.EastAsian = attr(c, "typeface", "")
		case "cs":
			f.ComplexScript = attr(c, "typeface", "")
		}
	}
	return f
}

// ParseHex parses a color of six hexadecimal digits, such as "1F497D", as
// Office writes it. It returns false for any other text.
func ParseHex(s string) (color.NRGBA, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return color.NRGBA{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.NRGBA{}, false
	}
	return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, true
}
