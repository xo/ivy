package odf

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/sheet"
)

// odsStyle is a style of an OpenDocument file, with the attributes of its
// properties elements by their local name, such as "background-color" or
// "column-width".
type odsStyle struct {
	family, parent string
	props          map[string]string
	// master is the master page of a style of a table.
	master string
}

// odsStyles holds the styles of a spreadsheet: the styles of styles.xml
// and the automatic styles of content.xml, the default styles of each
// family, the faces of the fonts, the page layouts and the master pages.
type odsStyles struct {
	styles   map[string]*odsStyle
	defaults map[string]*odsStyle
	fonts    map[string]odsFont
	layouts  map[string]*odsStyle
	masters  map[string]roxmltree.Node
	// resolved caches the properties of a style with its parents.
	resolved map[string]map[string]string
}

// odsFont is a font face of style:font-face.
type odsFont struct {
	family, generic string
}

// newODSStyles returns an empty set of styles.
func newODSStyles() *odsStyles {
	return &odsStyles{
		styles:   make(map[string]*odsStyle),
		defaults: make(map[string]*odsStyle),
		fonts:    make(map[string]odsFont),
		layouts:  make(map[string]*odsStyle),
		masters:  make(map[string]roxmltree.Node),
		resolved: make(map[string]map[string]string),
	}
}

// odsLocal returns the local name of the element n, or "".
func odsLocal(n roxmltree.Node) string {
	if !n.IsElement() {
		return ""
	}
	return ebook.Local(n)
}

// odsProps reads the attributes of the properties elements of n, such as
// style:table-cell-properties, into props. The header and footer
// properties of a page layout go under the prefix "header-" or "footer-".
func odsProps(n roxmltree.Node, props map[string]string) {
	for c := range n.Children() {
		name := odsLocal(c)
		if !strings.HasSuffix(name, "-properties") {
			if name == "header-style" || name == "footer-style" {
				prefix := strings.TrimSuffix(name, "style")
				sub := make(map[string]string)
				odsProps(c, sub)
				for k, v := range sub {
					props[prefix+k] = v
				}
				props[prefix+"on"] = "true"
			}
			continue
		}
		for a := range c.Attributes() {
			props[a.Name()] = a.Value()
		}
	}
}

// read reads the styles of the document of styles.xml or content.xml:
// the font faces, the styles, the default styles, the page layouts and the
// master pages.
func (s *odsStyles) read(doc *roxmltree.Document) {
	for n := range doc.RootElement().Children() {
		switch odsLocal(n) {
		case "font-face-decls":
			for f := range n.Children() {
				if odsLocal(f) != "font-face" {
					continue
				}
				family := strings.Trim(ebook.Attr(f, "font-family"), `'"`)
				if family == "" {
					family = ebook.Attr(f, "name")
				}
				s.fonts[ebook.Attr(f, "name")] = odsFont{family: family, generic: odsGeneric(ebook.Attr(f, "font-family-generic"))}
			}
		case "styles", "automatic-styles":
			for st := range n.Children() {
				s.readStyle(st)
			}
		case "master-styles":
			for m := range n.Children() {
				if odsLocal(m) == "master-page" {
					s.masters[ebook.Attr(m, "name")] = m
				}
			}
		}
	}
}

// readStyle reads one element of office:styles or office:automatic-styles.
func (s *odsStyles) readStyle(n roxmltree.Node) {
	st := &odsStyle{props: make(map[string]string)}
	switch odsLocal(n) {
	case "style":
		st.family = ebook.Attr(n, "family")
		st.parent = ebook.Attr(n, "parent-style-name")
		st.master = ebook.Attr(n, "master-page-name")
		odsProps(n, st.props)
		s.styles[st.family+"/"+ebook.Attr(n, "name")] = st
	case "default-style":
		st.family = ebook.Attr(n, "family")
		odsProps(n, st.props)
		s.defaults[st.family] = st
	case "page-layout":
		odsProps(n, st.props)
		s.layouts[ebook.Attr(n, "name")] = st
	}
}

// props returns the properties of the style name of the family, with the
// properties of its parents and of the default style of the family.
func (s *odsStyles) props(family, name string) map[string]string {
	key := family + "/" + name
	if p, ok := s.resolved[key]; ok {
		return p
	}
	var chain []*odsStyle
	seen := make(map[string]bool)
	for n := name; n != "" && !seen[n] && len(chain) < 32; {
		seen[n] = true
		st, ok := s.styles[family+"/"+n]
		if !ok {
			break
		}
		chain = append(chain, st)
		n = st.parent
	}
	p := make(map[string]string)
	if d, ok := s.defaults[family]; ok {
		for k, v := range d.props {
			p[k] = v
		}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		for k, v := range chain[i].props {
			p[k] = v
		}
	}
	s.resolved[key] = p
	return p
}

// style returns the style name of the family, or nil.
func (s *odsStyles) style(family, name string) *odsStyle {
	return s.styles[family+"/"+name]
}

// odsGeneric returns the generic family of CSS of a generic family of
// OpenDocument.
func odsGeneric(g string) string {
	switch g {
	case "roman":
		return "serif"
	case "modern":
		return "monospace"
	}
	return ""
}

// odsLength parses a length of OpenDocument, such as "2cm" or "0.8in", in
// points. It returns false for a length that it cannot read.
func odsLength(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	units := []struct {
		unit  string
		scale float64
	}{{"mm", 72 / 25.4}, {"cm", 72 / 2.54}, {"in", 72}, {"pt", 1}, {"pc", 12}, {"px", 0.75}}
	for _, u := range units {
		if v, ok := strings.CutSuffix(s, u.unit); ok {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil || math.IsNaN(f) || math.Abs(f) > 1e5 {
				return 0, false
			}
			return f * u.scale, true
		}
	}
	return 0, false
}

// odsColor parses a color such as "#ff0000". It returns false for
// "transparent" and for a color that it cannot read.
func odsColor(s string) (color.NRGBA, bool) {
	if !strings.HasPrefix(strings.TrimSpace(s), "#") {
		return color.NRGBA{}, false
	}
	return drawingml.ParseHex(s)
}

// odsBorder parses a border, such as "0.06pt solid #000000".
func odsBorder(s string) sheet.Border {
	var b sheet.Border
	for _, f := range strings.Fields(s) {
		switch f {
		case "none", "hidden":
			return sheet.Border{}
		case "solid", "dashed", "dotted", "double":
			b.Line = f
		case "dash-dot", "dash-dot-dot", "long-dash":
			b.Line = "dashed"
		case "fine-dashed":
			b.Line = "dashed"
		case "thin":
			b.Width = 0.75
		case "medium":
			b.Width = 1.5
		case "thick":
			b.Width = 2.25
		default:
			if v, ok := odsLength(f); ok {
				b.Width = v
			} else if c, ok := odsColor(f); ok {
				b.Color = c
			}
		}
	}
	if b.Line == "" {
		b.Line = "solid"
	}
	if b.Width > 0 && b.Width < 0.5 {
		// LibreOffice writes a hairline of 0.06 points, which it prints
		// as a thin line
		b.Width = 0.5
	}
	b.Width = math.Min(b.Width, 12)
	return b
}

// font returns the font of the properties p.
func (s *odsStyles) font(p map[string]string) sheet.Font {
	var f sheet.Font
	if name := p["font-name"]; name != "" {
		if face, ok := s.fonts[name]; ok {
			f.Family, f.Generic = face.family, face.generic
		} else {
			f.Family = name
		}
	}
	if fam := strings.Trim(p["font-family"], `'"`); fam != "" && f.Family == "" {
		f.Family = fam
		f.Generic = odsGeneric(p["font-family-generic"])
	}
	if v, ok := odsLength(p["font-size"]); ok && v > 0 && v < 410 {
		f.Size = v
	}
	switch p["font-weight"] {
	case "bold", "600", "700", "800", "900":
		f.Bold = true
	}
	f.Italic = p["font-style"] == "italic" || p["font-style"] == "oblique"
	if u := p["text-underline-style"]; u != "" && u != "none" {
		f.Underline = 1
		if p["text-underline-type"] == "double" {
			f.Underline = 2
		}
	}
	if l := p["text-line-through-style"]; l != "" && l != "none" {
		f.Strike = true
	}
	if c, ok := odsColor(p["color"]); ok {
		f.Color = c
	}
	if pos := p["text-position"]; pos != "" {
		switch {
		case strings.HasPrefix(pos, "super"), strings.HasPrefix(pos, "33%"):
			f.VertAlign = 1
		case strings.HasPrefix(pos, "sub"), strings.HasPrefix(pos, "-"):
			f.VertAlign = 2
		}
	}
	return f
}

// cellStyle returns the style of a cell of the properties p.
func (s *odsStyles) cellStyle(p map[string]string) sheet.Style {
	st := sheet.Style{Font: s.font(p)}
	if c, ok := odsColor(p["background-color"]); ok {
		st.Fill = c
	}
	if b := p["border"]; b != "" {
		bd := odsBorder(b)
		st.Borders = [4]sheet.Border{bd, bd, bd, bd}
	}
	for k, side := range []string{"border-top", "border-right", "border-bottom", "border-left"} {
		if b, ok := p[side]; ok {
			st.Borders[k] = odsBorder(b)
		}
	}
	if p["text-align-source"] != "value-type" {
		switch p["text-align"] {
		case "start", "left":
			st.HAlign = "left"
		case "end", "right":
			st.HAlign = "right"
		case "center":
			st.HAlign = "center"
		case "justify":
			st.HAlign = "justify"
		}
	}
	switch p["vertical-align"] {
	case "top":
		st.VAlign = "top"
	case "middle":
		st.VAlign = "center"
	}
	st.Wrap = p["wrap-option"] == "wrap"
	st.Shrink = p["shrink-to-fit"] == "true"
	if v, ok := odsLength(p["margin-left"]); ok && v > 0 {
		st.Indent = math.Min(v, 500)
	}
	if a, err := strconv.ParseFloat(strings.TrimSuffix(p["rotation-angle"], "deg"), 64); err == nil {
		a = math.Mod(a, 360)
		switch {
		case a > 180:
			a -= 360
		case a < -180:
			a += 360
		}
		if a >= -90 && a <= 90 {
			st.Rotation = int(math.Round(a))
		}
	}
	if p["direction"] == "ttb" {
		st.Rotation = 255
	}
	return st
}
