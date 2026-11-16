package odf

import (
	"maps"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"
)

// odtProps holds the properties of a style of OpenDocument, by the local
// name of the attribute, such as "font-size" for fo:font-size. The
// properties of the paragraph, of the text, of the table, of the cell and
// of the graphic share one map, because their names differ.
type odtProps map[string]string

// over returns the properties of p with the properties of top over them. A
// font size in percent is a percent of the size below it.
func (p odtProps) over(top odtProps) odtProps {
	out := make(odtProps, len(p)+len(top))
	maps.Copy(out, p)
	for k, v := range top {
		if k == "font-size" && strings.HasSuffix(v, "%") {
			pct, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64)
			if err == nil && pct > 0 {
				v = strconv.FormatFloat(odtFontSize(p)*pct/100, 'f', 2, 64) + "pt"
			}
		}
		out[k] = v
	}
	return out
}

// odtFontSize returns the font size of p in points, or 12.
func odtFontSize(p odtProps) float64 {
	if v, ok := odtLength(p["font-size"]); ok && v > 0 && v < 1638 {
		return v
	}
	return 12
}

// odtLength parses a length of OpenDocument, such as "0.5in" or "12pt", in
// points.
func odtLength(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	for _, u := range [...]struct {
		suffix string
		scale  float64
	}{{"pt", 1}, {"in", 72}, {"cm", 72 / 2.54}, {"mm", 7.2 / 2.54}, {"pc", 12}, {"px", 0.75}} {
		if rest, ok := strings.CutSuffix(s, u.suffix); ok {
			v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
			if err != nil || v != v || v > 1e6 || v < -1e6 {
				return 0, false
			}
			return v * u.scale, true
		}
	}
	return 0, false
}

// odtStyle is a style of OpenDocument, style:style or style:default-style.
type odtStyle struct {
	name, family, parent string
	listStyle, master    string
	props                odtProps
}

// odtListLevel is a level of a list style.
type odtListLevel struct {
	bullet           string
	format           string
	prefix, suffix   string
	display          int
	start            int
	marginLeft       float64
	textIndent       float64
	followedBy       string
	hasLabelPosition bool
	props            odtProps
}

// odtPageLayout is a style:page-layout, with the sizes in points.
type odtPageLayout struct {
	width, height            float64
	top, right, bottom, left float64
	columns                  int
	columnGap                float64
	// header and footer are the min-height and the spacing of the header
	// and the footer.
	headerHeight, headerSpace float64
	footerHeight, footerSpace float64
}

// odtMasterPage is a style:master-page.
type odtMasterPage struct {
	name, layout string
	// parts are the style:header, style:footer, style:header-left and the
	// like, by their local name.
	parts map[string]roxmltree.Node
}

// odtStyles holds the styles of styles.xml and content.xml.
type odtStyles struct {
	// styles maps the family and the name of a style, such as
	// "paragraph/Standard", to the style.
	styles   map[string]*odtStyle
	defaults map[string]odtProps
	lists    map[string]*[10]*odtListLevel
	layouts  map[string]*odtPageLayout
	masters  map[string]*odtMasterPage
	// masterOrder holds the names of the master pages, in their order.
	masterOrder []string
	fonts       map[string]string
	resolved    map[string]odtProps
	maxDepth    int
}

// newODTStyles returns an empty set of styles.
func newODTStyles(maxDepth int) *odtStyles {
	return &odtStyles{
		styles:   map[string]*odtStyle{},
		defaults: map[string]odtProps{},
		lists:    map[string]*[10]*odtListLevel{},
		layouts:  map[string]*odtPageLayout{},
		masters:  map[string]*odtMasterPage{},
		fonts:    map[string]string{},
		resolved: map[string]odtProps{},
		maxDepth: maxDepth,
	}
}

// odtAttr returns the attribute of n with the local name name.
func odtAttr(n roxmltree.Node, name string) string {
	for a := range n.Attributes() {
		if a.Name() == name {
			return a.Value()
		}
	}
	return ""
}

// odtChild returns the first child element of n with the local name name.
func odtChild(n roxmltree.Node, name string) (roxmltree.Node, bool) {
	for c := range n.Children() {
		if c.IsElement() && c.TagName().Name() == name {
			return c, true
		}
	}
	return roxmltree.Node{}, false
}

// odtReadProps reads the attributes of the property elements of n, such as
// style:paragraph-properties and style:text-properties.
func odtReadProps(n roxmltree.Node) odtProps {
	p := odtProps{}
	for c := range n.Children() {
		if !c.IsElement() || !strings.HasSuffix(c.TagName().Name(), "-properties") {
			continue
		}
		for a := range c.Attributes() {
			p[a.Name()] = a.Value()
		}
		// The columns of a section or of a page.
		if cols, ok := odtChild(c, "columns"); ok {
			p["column-count"] = odtAttr(cols, "column-count")
			p["column-gap"] = odtAttr(cols, "column-gap")
		}
	}
	return p
}

// read reads the styles of a document part: styles.xml or content.xml.
func (ts *odtStyles) read(root roxmltree.Node) {
	for n := range root.Children() {
		if !n.IsElement() {
			continue
		}
		switch n.TagName().Name() {
		case "font-face-decls":
			for f := range n.Children() {
				if f.IsElement() && f.TagName().Name() == "font-face" {
					family := strings.Trim(odtAttr(f, "font-family"), `'" `)
					if family == "" {
						family = odtAttr(f, "name")
					}
					ts.fonts[odtAttr(f, "name")] = family + "\x00" + odtAttr(f, "font-family-generic")
				}
			}
		case "styles", "automatic-styles":
			ts.readStyles(n)
		case "master-styles":
			for m := range n.Children() {
				if !m.IsElement() || m.TagName().Name() != "master-page" {
					continue
				}
				mp := &odtMasterPage{name: odtAttr(m, "name"), layout: odtAttr(m, "page-layout-name"), parts: map[string]roxmltree.Node{}}
				for part := range m.Children() {
					if part.IsElement() {
						mp.parts[part.TagName().Name()] = part
					}
				}
				if _, ok := ts.masters[mp.name]; !ok {
					ts.masters[mp.name] = mp
					ts.masterOrder = append(ts.masterOrder, mp.name)
				}
			}
		}
	}
}

// readStyles reads the children of office:styles or office:automatic-styles.
func (ts *odtStyles) readStyles(n roxmltree.Node) {
	for s := range n.Children() {
		if !s.IsElement() {
			continue
		}
		switch s.TagName().Name() {
		case "default-style":
			ts.defaults[odtAttr(s, "family")] = odtReadProps(s)
		case "style":
			st := &odtStyle{
				name: odtAttr(s, "name"), family: odtAttr(s, "family"), parent: odtAttr(s, "parent-style-name"),
				listStyle: odtAttr(s, "list-style-name"), master: odtAttr(s, "master-page-name"),
				props: odtReadProps(s),
			}
			ts.styles[st.family+"/"+st.name] = st
		case "list-style":
			ts.lists[odtAttr(s, "name")] = readODTListStyle(s)
		case "page-layout":
			ts.layouts[odtAttr(s, "name")] = readODTPageLayout(s)
		}
	}
}

// readODTListStyle reads a text:list-style.
func readODTListStyle(n roxmltree.Node) *[10]*odtListLevel {
	var levels [10]*odtListLevel
	for l := range n.Children() {
		if !l.IsElement() {
			continue
		}
		i, err := strconv.Atoi(odtAttr(l, "level"))
		if err != nil || i < 1 || i > 10 {
			continue
		}
		lvl := &odtListLevel{display: 1, start: 1, followedBy: "listtab", props: odtProps{}}
		switch l.TagName().Name() {
		case "list-level-style-bullet":
			lvl.bullet = odtAttr(l, "bullet-char")
			if lvl.bullet == "" {
				lvl.bullet = "•"
			}
		case "list-level-style-number":
			lvl.format = odtAttr(l, "num-format")
			if d, err := strconv.Atoi(odtAttr(l, "display-levels")); err == nil && d > 0 {
				lvl.display = min(d, 10)
			}
			if s, err := strconv.Atoi(odtAttr(l, "start-value")); err == nil {
				lvl.start = s
			}
		case "list-level-style-image":
			lvl.bullet = "•"
		default:
			continue
		}
		lvl.prefix = odtAttr(l, "num-prefix")
		lvl.suffix = odtAttr(l, "num-suffix")
		if pr, ok := odtChild(l, "list-level-properties"); ok {
			if al, ok := odtChild(pr, "list-level-label-alignment"); ok {
				lvl.hasLabelPosition = true
				lvl.marginLeft, _ = odtLength(odtAttr(al, "margin-left"))
				lvl.textIndent, _ = odtLength(odtAttr(al, "text-indent"))
				if f := odtAttr(al, "label-followed-by"); f != "" {
					lvl.followedBy = f
				}
			} else {
				before, _ := odtLength(odtAttr(pr, "space-before"))
				width, _ := odtLength(odtAttr(pr, "min-label-width"))
				lvl.marginLeft = before + width
				lvl.textIndent = -width
				lvl.hasLabelPosition = before != 0 || width != 0
			}
		}
		lvl.props = odtReadProps(l)
		levels[i-1] = lvl
	}
	return &levels
}

// readODTPageLayout reads a style:page-layout.
func readODTPageLayout(n roxmltree.Node) *odtPageLayout {
	pl := &odtPageLayout{width: 612, height: 792, top: 72, right: 72, bottom: 72, left: 72, columns: 1}
	if pr, ok := odtChild(n, "page-layout-properties"); ok {
		set := func(v *float64, name string) {
			if l, ok := odtLength(odtAttr(pr, name)); ok && l >= 0 {
				*v = l
			}
		}
		set(&pl.width, "page-width")
		set(&pl.height, "page-height")
		if m, ok := odtLength(odtAttr(pr, "margin")); ok && m >= 0 {
			pl.top, pl.right, pl.bottom, pl.left = m, m, m, m
		}
		set(&pl.top, "margin-top")
		set(&pl.right, "margin-right")
		set(&pl.bottom, "margin-bottom")
		set(&pl.left, "margin-left")
		if cols, ok := odtChild(pr, "columns"); ok {
			if v, err := strconv.Atoi(odtAttr(cols, "column-count")); err == nil && v > 1 {
				pl.columns = min(v, 16)
			}
			pl.columnGap, _ = odtLength(odtAttr(cols, "column-gap"))
		}
	}
	for _, part := range [...]string{"header-style", "footer-style"} {
		hs, ok := odtChild(n, part)
		if !ok {
			continue
		}
		pr, ok := odtChild(hs, "header-footer-properties")
		if !ok {
			continue
		}
		height, _ := odtLength(odtAttr(pr, "min-height"))
		if h, ok := odtLength(odtAttr(pr, "height")); ok {
			height = h
		}
		if part == "header-style" {
			pl.headerHeight = height
			pl.headerSpace, _ = odtLength(odtAttr(pr, "margin-bottom"))
		} else {
			pl.footerHeight = height
			pl.footerSpace, _ = odtLength(odtAttr(pr, "margin-top"))
		}
	}
	pl.width = min(max(pl.width, 72), 14400)
	pl.height = min(max(pl.height, 72), 14400)
	return pl
}

// props returns the properties of the style name of the family, with its
// parents and the default style of the family under it.
func (ts *odtStyles) props(family, name string) odtProps {
	key := family + "/" + name
	if p, ok := ts.resolved[key]; ok {
		return p
	}
	// A loop of parents resolves to the default style.
	ts.resolved[key] = ts.defaults[family]
	if ts.defaults[family] == nil {
		ts.resolved[key] = odtProps{}
	}
	var chain []*odtStyle
	seen := map[string]bool{}
	for n := name; n != "" && len(chain) < ts.maxDepth && !seen[n]; {
		seen[n] = true
		st, ok := ts.styles[family+"/"+n]
		if !ok {
			break
		}
		chain = append(chain, st)
		n = st.parent
	}
	p := ts.defaults[family]
	if p == nil {
		p = odtProps{}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		p = p.over(chain[i].props)
	}
	ts.resolved[key] = p
	return p
}

// inherited returns the attribute of the style name or of its parents,
// such as its list style or its master page.
func (ts *odtStyles) inherited(family, name string, get func(*odtStyle) string) string {
	seen := map[string]bool{}
	for n := name; n != "" && !seen[n] && len(seen) < ts.maxDepth; {
		seen[n] = true
		st, ok := ts.styles[family+"/"+n]
		if !ok {
			return ""
		}
		if v := get(st); v != "" {
			return v
		}
		n = st.parent
	}
	return ""
}

// font returns the family and the generic family of a font face of the
// font-face-decls, or of a family that the properties name.
func (ts *odtStyles) font(p odtProps) (string, string) {
	if name := p["font-name"]; name != "" {
		if v, ok := ts.fonts[name]; ok {
			family, generic, _ := strings.Cut(v, "\x00")
			return family, generic
		}
		return name, ""
	}
	return strings.Trim(p["font-family"], `'" `), p["font-family-generic"]
}
