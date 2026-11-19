package odf

import (
	"maps"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/fontdb"
)

// odtProps holds the properties of a style of OpenDocument, by the local
// name of the attribute, such as "font-size" for fo:font-size. The
// properties of the paragraph, of the text, of the table, of the cell and
// of the graphic share one map, because their names differ.
type odtProps map[string]string

// over returns the properties of p with the properties of top over them. A
// font size in percent is a percent of the size below it: fo:font-size
// of the size of the Western text, style:font-size-asian of the size of
// the East Asian text and style:font-size-complex of the size of the
// complex text.
func (p odtProps) over(top odtProps) odtProps {
	out := make(odtProps, len(p)+len(top))
	maps.Copy(out, p)
	for k, v := range top {
		if suffix, ok := strings.CutPrefix(k, "font-size"); ok && (suffix == "" || suffix == "-asian" || suffix == "-complex") && strings.HasSuffix(v, "%") {
			pct, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64)
			if err == nil && pct > 0 {
				v = strconv.FormatFloat(odtSizeOf(p, suffix)*pct/100, 'f', 2, 64) + "pt"
			}
		}
		out[k] = v
	}
	// style:use-window-font-color gives the automatic color of the text,
	// which replaces the color below it. A color of top replaces the
	// automatic color below it.
	if top["use-window-font-color"] == "true" {
		delete(out, "color")
	} else if _, ok := top["color"]; ok {
		delete(out, "use-window-font-color")
	}
	// A font family of top replaces a font face of p: style:font-name
	// wins only over a font family of its own style.
	for _, suffix := range [...]string{"", "-asian", "-complex"} {
		if _, ok := top["font-family"+suffix]; ok {
			if _, ok := top["font-name"+suffix]; !ok {
				delete(out, "font-name"+suffix)
			}
		}
	}
	return out
}

// odtFontSize returns the font size of p in points, or 12.
func odtFontSize(p odtProps) float64 {
	return odtSizeOf(p, "")
}

// odtSizeOf returns the font size of p in points of the script of the
// suffix: "" for the Western text, "-asian" or "-complex". A script
// without a size of its own takes the size of the Western text, or 12.
func odtSizeOf(p odtProps, suffix string) float64 {
	if v, ok := odtLength(p["font-size"+suffix]); ok && v > 0 && v < 1638 {
		return v
	}
	if suffix != "" {
		return odtSizeOf(p, "")
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
	// auto is set for an automatic style, which LibreOffice reads as the
	// direct formatting of a paragraph.
	auto bool
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
	// labelAlignment is set for a level of the mode label-alignment,
	// whose indents the paragraph can override. tabStop is the position of
	// the tab after the label from the left of the text area, where
	// hasTabStop is set, and labelAlign is the alignment of the label:
	// start, center or end.
	labelAlignment bool
	tabStop        float64
	hasTabStop     bool
	labelAlign     string
	props          odtProps
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
	// headerFixed and footerFixed are set when the header or the footer
	// has a fixed height, svg:height, and does not grow with its content.
	headerFixed, footerFixed bool
	// noSize is set when the layout gives no fo:page-width and no
	// fo:page-height, and orientation is its style:print-orientation.
	noSize      bool
	orientation string
}

// odtMasterPage is a style:master-page.
type odtMasterPage struct {
	name, layout string
	// next is the master page of the pages after the first, from
	// style:next-style-name, or "".
	next string
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
	// outline is the text:outline-style, which numbers the headings, or
	// nil.
	outline *[10]*odtListLevel
	layouts map[string]*odtPageLayout
	masters map[string]*odtMasterPage
	// masterOrder holds the names of the master pages, in their order.
	masterOrder []string
	fonts       map[string]string
	resolved    map[string]odtProps
	// relative holds whether a text style or one of its parents has a
	// font size in percent, by name.
	relative map[string]bool
	maxDepth int
	// partAuto holds the names of the automatic styles of styles.xml. They
	// are apart from the automatic styles of content.xml, which can have
	// the same names, and the styles map holds them under their name with
	// the prefix odtStylesPart.
	partAuto map[string]bool
	// lineNumbers is the value of -ivy-line-numbers of the text, from
	// text:linenumbering-configuration, or "" when its lines have no
	// numbers, and lineNumberStyle is the text style of the numbers.
	lineNumbers, lineNumberStyle string
}

// odtStylesPart is the prefix of the name of an automatic style of
// styles.xml in odtStyles.styles.
const odtStylesPart = "\x00"

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
		relative: map[string]bool{},
		maxDepth: maxDepth,
		partAuto: map[string]bool{},
	}
}

// partName returns the name of the style name in odtStyles.styles for a
// reference from styles.xml, which part is set for, or from content.xml.
func (ts *odtStyles) partName(name string, part bool) string {
	if part && ts.partAuto[name] {
		return odtStylesPart + name
	}
	return name
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
		text := c.TagName().Name() == "text-properties"
		for a := range c.Attributes() {
			name := a.Name()
			// The background of the text is apart from the background of
			// the paragraph, the cell or the frame.
			if text && name == "background-color" {
				name = "text-background-color"
			}
			p[name] = a.Value()
		}
		// The columns of a section or of a page.
		if cols, ok := odtChild(c, "columns"); ok {
			p["column-count"] = odtAttr(cols, "column-count")
			p["column-gap"] = odtAttr(cols, "column-gap")
		}
		// The tab stops of a paragraph replace those of its parent.
		if tabs, ok := odtChild(c, "tab-stops"); ok {
			p["tab-stops"] = odtTabStops(tabs)
		}
	}
	return p
}

// odtTabLeaders gives the text of a leader of style:leader-style when the
// tab stop has no style:leader-text.
var odtTabLeaders = map[string]string{
	"dotted": ".", "dash": "-", "long-dash": "-", "dot-dash": "-", "dot-dot-dash": "-", "solid": "_", "wave": "~",
}

// odtTabStops encodes the tab stops of style:tab-stops as a property: a
// stop for each line, with its position in points, its alignment and its
// leader, separated by tabs.
func odtTabStops(n roxmltree.Node) string {
	var b strings.Builder
	count := 0
	for t := range n.Children() {
		if !t.IsElement() || t.TagName().Name() != "tab-stop" || count >= 64 {
			continue
		}
		pos, ok := odtLength(odtAttr(t, "position"))
		if !ok || pos < 0 {
			continue
		}
		align := "left"
		switch odtAttr(t, "type") {
		case "right":
			align = "right"
		case "center":
			align = "center"
		case "char":
			align = "decimal"
		}
		leader := ""
		if style := odtAttr(t, "leader-style"); style != "" && style != "none" {
			leader = odtTabLeaders[style]
		}
		if text := odtAttr(t, "leader-text"); text != "" && text != " " {
			leader = text
		}
		if leader == "" && odtAttr(t, "leader-type") != "" && odtAttr(t, "leader-type") != "none" {
			leader = "."
		}
		b.WriteString(strconv.FormatFloat(pos, 'f', 2, 64) + "\t" + align + "\t" + leader + "\n")
		count++
	}
	return b.String()
}

// read reads the styles of a document part: styles.xml, which part is set
// for, or content.xml.
func (ts *odtStyles) read(root roxmltree.Node, part bool) {
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
			ts.readStyles(n, n.TagName().Name() == "automatic-styles", part)
		case "master-styles":
			for m := range n.Children() {
				if !m.IsElement() || m.TagName().Name() != "master-page" {
					continue
				}
				mp := &odtMasterPage{name: odtAttr(m, "name"), layout: odtAttr(m, "page-layout-name"), next: odtAttr(m, "next-style-name"), parts: map[string]roxmltree.Node{}}
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

// readStyles reads the children of office:styles or office:automatic-styles,
// which auto is set for, of styles.xml, which part is set for, or of
// content.xml.
func (ts *odtStyles) readStyles(n roxmltree.Node, auto, part bool) {
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
				props: odtReadProps(s), auto: auto,
			}
			if auto && part {
				ts.partAuto[st.name] = true
				st.name = odtStylesPart + st.name
			}
			ts.styles[st.family+"/"+st.name] = st
		case "list-style":
			ts.lists[odtAttr(s, "name")] = readODTListStyle(s)
		case "outline-style":
			if ts.outline == nil {
				ts.outline = readODTListStyle(s)
			}
		case "page-layout":
			ts.layouts[odtAttr(s, "name")] = readODTPageLayout(s)
		case "linenumbering-configuration":
			ts.lineNumbers = odtLineNumbers(s)
			ts.lineNumberStyle = odtAttr(s, "style-name")
		}
	}
}

// odtLineNumbers returns the value of -ivy-line-numbers of a
// text:linenumbering-configuration, or "" when the lines have no numbers
// (ODF 1.3 Part 3, 16.29.1). text:number-lines is true by default. The
// layout draws the numbers on the left only, and counts the empty lines,
// as text:count-empty-lines does by default. text:increment is the step,
// and text:offset the distance from the text. The numbers restart on each
// page with text:restart-on-page, and else go on through the text.
func odtLineNumbers(n roxmltree.Node) string {
	if odtAttr(n, "number-lines") == "false" {
		return ""
	}
	if pos := odtAttr(n, "number-position"); pos != "" && pos != "left" {
		return ""
	}
	step, err := strconv.Atoi(odtAttr(n, "increment"))
	if err != nil || step < 1 || step > 100 {
		step = 1
	}
	distance, ok := odtLength(odtAttr(n, "offset"))
	if !ok || distance < 0 || distance > 1584 {
		distance = 14.15
	}
	restart := "continuous"
	if odtAttr(n, "restart-on-page") == "true" {
		restart = "page"
	}
	return odtPt(distance) + " " + strconv.Itoa(step) + " 1 " + restart
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
		case "list-level-style-number", "outline-level-style":
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
				lvl.hasLabelPosition, lvl.labelAlignment = true, true
				lvl.marginLeft, _ = odtLength(odtAttr(al, "margin-left"))
				lvl.textIndent, _ = odtLength(odtAttr(al, "text-indent"))
				if f := odtAttr(al, "label-followed-by"); f != "" {
					lvl.followedBy = f
				}
				lvl.tabStop, lvl.hasTabStop = odtLength(odtAttr(al, "list-tab-stop-position"))
				lvl.labelAlign = odtAttr(pr, "text-align")
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

// defaultODTPageLayout returns the page layout of a page that gives no
// size and no margins. ODF 1.3 gives no default to fo:page-width and
// fo:page-height. ivy takes US Letter in portrait, with margins of 1 inch,
// and odtConverter.pageLayout puts the page on the paper of the program
// (D32).
func defaultODTPageLayout() *odtPageLayout {
	return &odtPageLayout{width: 612, height: 792, top: 72, right: 72, bottom: 72, left: 72, columns: 1, noSize: true}
}

// readODTPageLayout reads a style:page-layout.
func readODTPageLayout(n roxmltree.Node) *odtPageLayout {
	pl := defaultODTPageLayout()
	if pr, ok := odtChild(n, "page-layout-properties"); ok {
		pl.noSize = odtAttr(pr, "page-width") == "" && odtAttr(pr, "page-height") == ""
		pl.orientation = odtAttr(pr, "print-orientation")
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
		h, fixed := odtLength(odtAttr(pr, "height"))
		if fixed {
			height = h
		}
		if part == "header-style" {
			pl.headerHeight, pl.headerFixed = height, fixed
			pl.headerSpace, _ = odtLength(odtAttr(pr, "margin-bottom"))
		} else {
			pl.footerHeight, pl.footerFixed = height, fixed
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
	chain := ts.chain(family, name)
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

// chain returns the style name of the family and its parents, from the
// style to its last parent.
func (ts *odtStyles) chain(family, name string) []*odtStyle {
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
	return chain
}

// textOver returns the properties of the text style name over base, the
// properties of the text around it. A font size in percent of the text
// style or of its parents is a percent of the size of base, as in
// LibreOffice.
func (ts *odtStyles) textOver(base odtProps, name string) odtProps {
	relative, ok := ts.relative[name]
	if !ok {
		for _, st := range ts.chain("text", name) {
			for _, suffix := range [...]string{"", "-asian", "-complex"} {
				if strings.HasSuffix(st.props["font-size"+suffix], "%") {
					relative = true
				}
			}
		}
		ts.relative[name] = relative
	}
	if !relative {
		return base.over(ts.props("text", name))
	}
	chain := ts.chain("text", name)
	p := base
	if d := ts.defaults["text"]; d != nil {
		p = p.over(d)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		p = p.over(chain[i].props)
	}
	return p
}

// common returns the name of the common style of the style name of the
// family: the parent of an automatic style, or name itself.
func (ts *odtStyles) common(family, name string) string {
	if st, ok := ts.styles[family+"/"+name]; ok && st.auto {
		return st.parent
	}
	return name
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
	return ts.fontOf(p, "")
}

// asianFont returns the family of the East Asian font of the properties,
// from style:font-name-asian or style:font-family-asian, or "".
func (ts *odtStyles) asianFont(p odtProps) string {
	family, _ := ts.fontOf(p, "-asian")
	return family
}

// fontOf returns the font of the properties whose names end with suffix,
// such as "-asian" for the East Asian font.
func (ts *odtStyles) fontOf(p odtProps, suffix string) (string, string) {
	if name := p["font-name"+suffix]; name != "" {
		if v, ok := ts.fonts[name]; ok {
			family, generic, _ := strings.Cut(v, "\x00")
			return family, generic
		}
		return name, ""
	}
	return strings.Trim(p["font-family"+suffix], `'" `), p["font-family-generic"+suffix]
}

// odtAsianLang returns the language of the East Asian text of the
// properties, from style:language-asian and style:country-asian, such as
// "ja-JP", or "" if it is not an East Asian language.
func odtAsianLang(p odtProps) string {
	lang := p["language-asian"]
	if country := p["country-asian"]; country != "" && country != "none" {
		lang += "-" + country
	}
	if fontdb.ParseLang(lang) == fontdb.NoLang {
		return ""
	}
	return lang
}
