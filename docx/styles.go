package docx

import (
	"maps"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ooxml"
)

// props holds the properties of a level of formatting, such as a style or
// the direct formatting of a run, by name. A level above another one
// replaces each property that it sets. The names are those of
// WordprocessingML, with a dot before the name of an attribute, such as
// "ind.left" for the attribute left of w:ind.
type props map[string]string

// over returns the properties of p with the properties of top over them. It
// changes neither map.
func (p props) over(top props) props {
	out := make(props, len(p)+len(top))
	maps.Copy(out, p)
	for k, v := range top {
		switch k {
		// The indent of the first line and the hanging indent exclude each
		// other.
		case "ind.firstLine":
			delete(out, "ind.hanging")
		case "ind.hanging":
			delete(out, "ind.firstLine")
		}
		out[k] = v
	}
	return out
}

// on reports whether the property name is on: a toggle property, such as
// w:b, without the value false.
func (p props) on(name string) bool {
	v, ok := p[name]
	return ok && v != "0" && v != "false" && v != "off"
}

// num returns the number of the property name, or def.
func (p props) num(name string, def float64) float64 {
	v, ok := p[name]
	if !ok {
		return def
	}
	return number(v, def)
}

// number parses a number of WordprocessingML, or returns def. A number can
// carry a unit, such as "12pt", in the strict form of ECMA-376.
func number(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	scale := 1.0
	for _, u := range [...]struct {
		suffix string
		scale  float64
	}{{"pt", 20}, {"in", 1440}, {"cm", 1440 / 2.54}, {"mm", 144 / 2.54}, {"pc", 240}, {"pi", 240}, {"%", 1}} {
		if rest, ok := strings.CutSuffix(s, u.suffix); ok {
			s, scale = rest, u.scale
			break
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f != f {
		return def
	}
	return f * scale
}

// toggles are the properties of a run that are on or off.
var toggles = map[string]bool{
	"b": true, "i": true, "caps": true, "smallCaps": true, "strike": true,
	"dstrike": true, "vanish": true, "specVanish": true, "outline": true,
	"emboss": true, "imprint": true, "shadow": true,
}

// parseRPr reads the properties of a w:rPr element.
func parseRPr(n roxmltree.Node) props {
	p := props{}
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		name := c.TagName().Name()
		switch {
		case toggles[name]:
			v := ooxml.Attr(c, "val")
			if v == "" {
				v = "1"
			}
			p[name] = v
		case name == "rFonts":
			fonts(p, c)
		case name == "lang":
			if v := ooxml.Attr(c, "eastAsia"); v != "" {
				p["lang.ea"] = v
			}
		case name == "sz" || name == "u" || name == "highlight" || name == "vertAlign" ||
			name == "spacing" || name == "position" || name == "rStyle":
			p[name] = ooxml.Attr(c, "val")
		case name == "color":
			p["color"] = ooxml.Attr(c, "val")
			if t := ooxml.Attr(c, "themeColor"); t != "" {
				p["color.theme"] = t
			}
		case name == "shd":
			if fill := shading(c); fill != "" {
				p["shd"] = fill
			}
		}
	}
	return p
}

// fonts reads the fonts of w:rFonts into p: "font" for the Latin text and
// "font.ea" for the East Asian text. A font of the theme comes before a
// font that the element names, and the font of ASCII before the font of
// the other Latin text.
func fonts(p props, n roxmltree.Node) {
	for key, attrs := range map[string][]string{
		"font":    {"asciiTheme", "ascii", "hAnsiTheme", "hAnsi"},
		"font.ea": {"eastAsiaTheme", "eastAsia"},
	} {
		for _, a := range attrs {
			v := ooxml.Attr(n, a)
			if v == "" {
				continue
			}
			if strings.HasSuffix(a, "Theme") {
				v = "theme:" + v
			}
			p[key] = v
			break
		}
	}
}

// shading returns the color of a w:shd element: its fill, or its color for
// a solid pattern. It returns "" for no shading.
func shading(n roxmltree.Node) string {
	fill := ooxml.Attr(n, "fill")
	if ooxml.Attr(n, "val") == "solid" {
		fill = ooxml.Attr(n, "color")
	}
	if fill == "" || fill == "auto" {
		if ooxml.Attr(n, "val") == "nil" {
			return "none"
		}
		return ""
	}
	return fill
}

// parsePPr reads the properties of a w:pPr element.
func parsePPr(n roxmltree.Node) props {
	p := props{}
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		name := c.TagName().Name()
		switch name {
		case "jc", "pStyle", "outlineLvl":
			p[name] = ooxml.Attr(c, "val")
		case "keepNext", "keepLines", "pageBreakBefore", "contextualSpacing", "widowControl", "suppressAutoHyphens", "snapToGrid":
			v := ooxml.Attr(c, "val")
			if v == "" {
				v = "1"
			}
			p[name] = v
		case "ind":
			for _, a := range [...][2]string{
				{"left", "left"}, {"start", "left"}, {"right", "right"}, {"end", "right"},
				{"firstLine", "firstLine"}, {"hanging", "hanging"},
			} {
				if v := ooxml.Attr(c, a[0]); v != "" {
					switch a[1] {
					case "firstLine":
						delete(p, "ind.hanging")
					case "hanging":
						delete(p, "ind.firstLine")
					}
					p["ind."+a[1]] = v
				}
			}
		case "spacing":
			for _, a := range [...]string{"before", "after", "line", "lineRule", "beforeAutospacing", "afterAutospacing"} {
				if v := ooxml.Attr(c, a); v != "" {
					p["spacing."+a] = v
				}
			}
		case "pBdr":
			borders(p, "bdr.", c)
		case "shd":
			if fill := shading(c); fill != "" {
				p["shd"] = fill
			}
		case "numPr":
			for e := range c.Children() {
				if !e.IsElement() {
					continue
				}
				switch e.TagName().Name() {
				case "numId":
					p["numId"] = ooxml.Attr(e, "val")
				case "ilvl":
					p["ilvl"] = ooxml.Attr(e, "val")
				}
			}
		}
	}
	return p
}

// borders reads the borders of an element such as w:pBdr or w:tblBorders
// into p, as "val sz color space" under the prefix and the side. w:start
// and w:end are the left and the right side.
func borders(p props, prefix string, n roxmltree.Node) {
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		side := c.TagName().Name()
		switch side {
		case "start":
			side = "left"
		case "end":
			side = "right"
		}
		val := ooxml.Attr(c, "val")
		if val == "" {
			continue
		}
		color := ooxml.Attr(c, "color")
		if color == "" {
			color = "auto"
		}
		sz := ooxml.Attr(c, "sz")
		if sz == "" {
			sz = "4"
		}
		space := ooxml.Attr(c, "space")
		if space == "" {
			space = "0"
		}
		p[prefix+side] = val + " " + sz + " " + color + " " + space
	}
}

// parseTblPr reads the properties of a w:tblPr element.
func parseTblPr(n roxmltree.Node) props {
	p := props{}
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		switch name := c.TagName().Name(); name {
		case "tblStyle", "jc", "tblStyleRowBandSize", "tblStyleColBandSize":
			p[name] = ooxml.Attr(c, "val")
		case "tblW", "tblInd":
			p[name] = ooxml.Attr(c, "w")
			p[name+".type"] = ooxml.Attr(c, "type")
		case "tblBorders":
			borders(p, "tbl.", c)
		case "tblCellMar":
			margins(p, "mar.", c)
		case "shd":
			if fill := shading(c); fill != "" {
				p["shd"] = fill
			}
		case "tblLook":
			look(p, c)
		}
	}
	return p
}

// margins reads the margins of an element such as w:tblCellMar into p,
// under the prefix and the side, in twips.
func margins(p props, prefix string, n roxmltree.Node) {
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		side := c.TagName().Name()
		switch side {
		case "start":
			side = "left"
		case "end":
			side = "right"
		}
		if t := ooxml.Attr(c, "type"); t == "dxa" || t == "" {
			p[prefix+side] = ooxml.Attr(c, "w")
		}
	}
}

// look reads w:tblLook, which tells the parts of a table style that apply,
// as attributes or as a number of flags.
func look(p props, n roxmltree.Node) {
	flags := map[string]uint64{
		"firstRow": 0x20, "lastRow": 0x40, "firstColumn": 0x80, "lastColumn": 0x100,
		"noHBand": 0x200, "noVBand": 0x400,
	}
	val, _ := strconv.ParseUint(ooxml.Attr(n, "val"), 16, 32)
	for name, flag := range flags {
		v := ooxml.Attr(n, name)
		switch {
		case v != "":
			p["look."+name] = v
		case val&flag != 0:
			p["look."+name] = "1"
		default:
			p["look."+name] = "0"
		}
	}
}

// parseTcPr reads the properties of a w:tcPr element, or of the w:tcPr of
// a conditional part of a table style.
func parseTcPr(n roxmltree.Node) props {
	p := props{}
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		switch name := c.TagName().Name(); name {
		case "gridSpan", "vAlign":
			p[name] = ooxml.Attr(c, "val")
		case "vMerge":
			v := ooxml.Attr(c, "val")
			if v == "" {
				v = "continue"
			}
			p["vMerge"] = v
		case "hMerge":
			v := ooxml.Attr(c, "val")
			if v == "" {
				v = "continue"
			}
			p["hMerge"] = v
		case "tcW":
			p["tcW"] = ooxml.Attr(c, "w")
			p["tcW.type"] = ooxml.Attr(c, "type")
		case "tcBorders":
			borders(p, "tc.", c)
		case "tcMar":
			margins(p, "mar.", c)
		case "shd":
			if fill := shading(c); fill != "" {
				p["shd"] = fill
			}
		case "noWrap":
			p["noWrap"] = "1"
		}
	}
	return p
}

// style is a style of styles.xml.
type style struct {
	id, typ, basedOn string
	ppr, rpr         props
	// tbl and tc are the properties of a table style for the table and for
	// its cells.
	tbl, tc props
	// conds holds the conditional parts of a table style, by type, such as
	// firstRow.
	conds map[string]*condition
}

// condition is a conditional part of a table style, w:tblStylePr.
type condition struct {
	ppr, rpr, tc props
}

// styles holds the styles of a document, with their inheritance resolved.
type styles struct {
	byID map[string]*style
	// pDefault, rDefault are the properties of w:docDefaults.
	pDefault, rDefault props
	// defaults are the ids of the default style of each type.
	defaults map[string]string
	// resolved holds the styles whose inheritance is resolved.
	resolved map[string]*style
	maxDepth int
}

// parseStyles reads styles.xml. A nil document gives no style.
func parseStyles(doc *roxmltree.Document, maxDepth int) *styles {
	s := &styles{
		byID:     map[string]*style{},
		pDefault: props{},
		rDefault: props{},
		defaults: map[string]string{},
		resolved: map[string]*style{},
		maxDepth: maxDepth,
	}
	if doc == nil {
		return s
	}
	for n := range doc.RootElement().Children() {
		if !n.IsElement() {
			continue
		}
		switch n.TagName().Name() {
		case "docDefaults":
			for c := range n.Descendants() {
				if !c.IsElement() {
					continue
				}
				switch c.TagName().Name() {
				case "rPr":
					s.rDefault = parseRPr(c)
				case "pPr":
					s.pDefault = parsePPr(c)
				}
			}
		case "style":
			st := parseStyle(n)
			if st.id == "" {
				continue
			}
			if _, ok := s.byID[st.id]; !ok {
				s.byID[st.id] = st
			}
			if v := ooxml.Attr(n, "default"); v == "1" || v == "true" {
				if _, ok := s.defaults[st.typ]; !ok {
					s.defaults[st.typ] = st.id
				}
			}
		}
	}
	return s
}

// parseStyle reads a w:style element.
func parseStyle(n roxmltree.Node) *style {
	st := &style{
		id:  ooxml.Attr(n, "styleId"),
		typ: ooxml.Attr(n, "type"),
		ppr: props{}, rpr: props{}, tbl: props{}, tc: props{},
	}
	if st.typ == "" {
		st.typ = "paragraph"
	}
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		switch c.TagName().Name() {
		case "name":
			// Word gives a style without an id the id of its name without
			// the spaces, which a paragraph can name.
			if st.id == "" {
				st.id = strings.ReplaceAll(ooxml.Attr(c, "val"), " ", "")
			}
		case "basedOn":
			st.basedOn = ooxml.Attr(c, "val")
		case "pPr":
			st.ppr = parsePPr(c)
		case "rPr":
			st.rpr = parseRPr(c)
		case "tblPr":
			st.tbl = parseTblPr(c)
		case "tcPr":
			st.tc = parseTcPr(c)
		case "tblStylePr":
			if st.conds == nil {
				st.conds = map[string]*condition{}
			}
			cond := &condition{ppr: props{}, rpr: props{}, tc: props{}}
			for e := range c.Children() {
				if !e.IsElement() {
					continue
				}
				switch e.TagName().Name() {
				case "pPr":
					cond.ppr = parsePPr(e)
				case "rPr":
					cond.rpr = parseRPr(e)
				case "tcPr":
					cond.tc = parseTcPr(e)
				case "tblPr":
					// The borders of a conditional part apply to its cells.
					for k, v := range parseTblPr(e) {
						if side, ok := strings.CutPrefix(k, "tbl."); ok {
							cond.tc["tc."+side] = v
						}
					}
				}
			}
			st.conds[ooxml.Attr(c, "type")] = cond
		}
	}
	return st
}

// get returns the style id with its inheritance resolved, or nil. A style
// of another type than typ is not used. An empty id is the default style of
// the type.
func (s *styles) get(id, typ string) *style {
	if id == "" {
		id = s.defaults[typ]
	}
	st := s.resolve(id, 0)
	if st == nil || st.typ != typ {
		if id != s.defaults[typ] && s.defaults[typ] != "" {
			return s.resolve(s.defaults[typ], 0)
		}
		return nil
	}
	return st
}

// resolve returns the style id with the properties of the styles it is
// based on under its own. It stops at a loop or past the limit of depth.
func (s *styles) resolve(id string, depth int) *style {
	if st, ok := s.resolved[id]; ok {
		return st
	}
	st, ok := s.byID[id]
	if !ok {
		return nil
	}
	// A loop of basedOn resolves to the style without its base.
	s.resolved[id] = st
	if st.basedOn == "" || st.basedOn == id || depth >= s.maxDepth {
		return st
	}
	base := s.resolve(st.basedOn, depth+1)
	if base == nil || base == st {
		return st
	}
	out := &style{
		id: st.id, typ: st.typ, basedOn: st.basedOn,
		ppr: base.ppr.over(st.ppr),
		rpr: base.rpr.over(st.rpr),
		tbl: base.tbl.over(st.tbl),
		tc:  base.tc.over(st.tc),
	}
	if len(base.conds) > 0 || len(st.conds) > 0 {
		out.conds = map[string]*condition{}
		maps.Copy(out.conds, base.conds)
		for k, c := range st.conds {
			if b, ok := base.conds[k]; ok {
				c = &condition{ppr: b.ppr.over(c.ppr), rpr: b.rpr.over(c.rpr), tc: b.tc.over(c.tc)}
			}
			out.conds[k] = c
		}
	}
	s.resolved[id] = out
	return out
}
