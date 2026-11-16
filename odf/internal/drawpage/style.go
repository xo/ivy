package drawpage

import (
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"
)

// The namespaces of OpenDocument that the drawings read.
const (
	nsDraw         = "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0"
	nsSVG          = "urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0"
	nsFO           = "urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0"
	nsStyle        = "urn:oasis:names:tc:opendocument:xmlns:style:1.0"
	nsText         = "urn:oasis:names:tc:opendocument:xmlns:text:1.0"
	nsPresentation = "urn:oasis:names:tc:opendocument:xmlns:presentation:1.0"
	nsXLink        = "http://www.w3.org/1999/xlink"
	nsTable        = "urn:oasis:names:tc:opendocument:xmlns:table:1.0"
	nsLOExt        = "urn:org:documentfoundation:names:experimental:office:xmlns:loext:1.0"
	nsOffice       = "urn:oasis:names:tc:opendocument:xmlns:office:1.0"
	nsDrawOOo      = "http://openoffice.org/2010/draw"
)

// prefixes gives the usual prefix of each namespace, for the keys of the
// properties of a style.
var prefixes = map[string]string{
	nsDraw:         "draw",
	nsSVG:          "svg",
	nsFO:           "fo",
	nsStyle:        "style",
	nsText:         "text",
	nsPresentation: "presentation",
	nsXLink:        "xlink",
	nsTable:        "table",
	nsLOExt:        "loext",
	nsOffice:       "office",
}

// maxStyleDepth is the number of parents of a style that the package
// follows.
const maxStyleDepth = 32

// at returns the attribute of n with the namespace and the local name.
func at(n roxmltree.Node, ns, name string) string {
	v, _ := n.Attribute(roxmltree.ExpandedNameFromTuple(ns, name))
	return v
}

// hasAt reports whether n has the attribute with the namespace and the
// local name.
func hasAt(n roxmltree.Node, ns, name string) bool {
	_, ok := n.Attribute(roxmltree.ExpandedNameFromTuple(ns, name))
	return ok
}

// is reports whether n is the element with the namespace and the local
// name.
func is(n roxmltree.Node, ns, name string) bool {
	if !n.IsElement() || n.TagName().Name() != name {
		return false
	}
	uri, _ := n.TagName().Namespace()
	return uri == ns
}

// firstChild returns the first child of n with the namespace and the local
// name.
func firstChild(n roxmltree.Node, ns, name string) (roxmltree.Node, bool) {
	for c := range n.Children() {
		if is(c, ns, name) {
			return c, true
		}
	}
	return roxmltree.Node{}, false
}

// props are the properties of a style, such as the attributes of its
// style:graphic-properties, by their prefixed name, such as "draw:fill".
type props map[string]string

// get returns the property with the prefixed name.
func (p props) get(name string) string {
	return p[name]
}

// styles holds the styles of a drawing or a presentation: the styles of
// styles.xml and the automatic styles of content.xml.
type styles struct {
	// named holds the styles by family and name. The automatic styles of
	// content.xml come before the others.
	named map[string]map[string]roxmltree.Node
	// defaults holds style:default-style by family.
	defaults map[string]roxmltree.Node
	// lists holds text:list-style by name.
	lists map[string]roxmltree.Node
	// gradients, hatches, images, dashes, markers and opacities hold the
	// named elements of office:styles, such as draw:gradient.
	gradients, hatches, images, dashes, markers, opacities map[string]roxmltree.Node
	// fonts maps the name of style:font-face to its family.
	fonts map[string]string
	// layouts holds style:page-layout by name.
	layouts map[string]roxmltree.Node
	// masters holds style:master-page by name, in order.
	masters     map[string]roxmltree.Node
	firstMaster roxmltree.Node
	hasMaster   bool
}

// newStyles returns empty styles.
func newStyles() *styles {
	return &styles{
		named:     map[string]map[string]roxmltree.Node{},
		defaults:  map[string]roxmltree.Node{},
		lists:     map[string]roxmltree.Node{},
		gradients: map[string]roxmltree.Node{},
		hatches:   map[string]roxmltree.Node{},
		images:    map[string]roxmltree.Node{},
		dashes:    map[string]roxmltree.Node{},
		markers:   map[string]roxmltree.Node{},
		opacities: map[string]roxmltree.Node{},
		fonts:     map[string]string{},
		layouts:   map[string]roxmltree.Node{},
		masters:   map[string]roxmltree.Node{},
	}
}

// add reads the styles of the root element of styles.xml or content.xml.
// A style that is there already keeps its first definition, so the
// automatic styles of content.xml, which come first, win.
func (s *styles) add(root roxmltree.Node) {
	for sec := range root.Children() {
		if !sec.IsElement() {
			continue
		}
		switch sec.TagName().Name() {
		case "font-face-decls":
			for f := range sec.Children() {
				if is(f, nsStyle, "font-face") {
					name := at(f, nsStyle, "name")
					if _, ok := s.fonts[name]; !ok {
						s.fonts[name] = unquote(at(f, nsSVG, "font-family"))
					}
				}
			}
		case "styles", "automatic-styles":
			s.addStyles(sec)
		case "master-styles":
			for m := range sec.Children() {
				if is(m, nsStyle, "master-page") {
					name := at(m, nsStyle, "name")
					if _, ok := s.masters[name]; !ok {
						s.masters[name] = m
					}
					if !s.hasMaster {
						s.firstMaster, s.hasMaster = m, true
					}
				}
			}
		}
	}
}

// addStyles reads the children of office:styles or
// office:automatic-styles.
func (s *styles) addStyles(sec roxmltree.Node) {
	keep := func(m map[string]roxmltree.Node, name string, n roxmltree.Node) {
		if _, ok := m[name]; !ok {
			m[name] = n
		}
	}
	for e := range sec.Children() {
		if !e.IsElement() {
			continue
		}
		name := at(e, nsStyle, "name")
		if name == "" {
			name = at(e, nsDraw, "name")
		}
		switch e.TagName().Name() {
		case "style":
			family := at(e, nsStyle, "family")
			if s.named[family] == nil {
				s.named[family] = map[string]roxmltree.Node{}
			}
			keep(s.named[family], name, e)
		case "default-style":
			keep(s.defaults, at(e, nsStyle, "family"), e)
		case "list-style":
			keep(s.lists, name, e)
		case "gradient":
			keep(s.gradients, name, e)
		case "hatch":
			keep(s.hatches, name, e)
		case "fill-image":
			keep(s.images, name, e)
		case "stroke-dash":
			keep(s.dashes, name, e)
		case "marker":
			keep(s.markers, name, e)
		case "opacity":
			keep(s.opacities, name, e)
		case "page-layout":
			keep(s.layouts, name, e)
		}
	}
}

// unquote removes the quotes around a family name of a font.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// style returns the style of the family with the name.
func (s *styles) style(family, name string) (roxmltree.Node, bool) {
	if name == "" {
		return roxmltree.Node{}, false
	}
	n, ok := s.named[family][name]
	return n, ok
}

// chain returns the style of the family with the name and its parents, the
// root first, after the default style of the family.
func (s *styles) chain(family, name string) []roxmltree.Node {
	var list []roxmltree.Node
	for range maxStyleDepth {
		n, ok := s.style(family, name)
		if !ok {
			break
		}
		list = append(list, n)
		name = at(n, nsStyle, "parent-style-name")
	}
	if d, ok := s.defaults[family]; ok {
		list = append(list, d)
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list
}

// props returns the properties of the element kind, such as
// "graphic-properties", of the styles of the chain, the last one winning.
// A font size in percent is relative to the size before it.
func (s *styles) props(chain []roxmltree.Node, kind string, into props) props {
	if into == nil {
		into = props{}
	}
	for _, st := range chain {
		for e := range st.Children() {
			if !e.IsElement() || e.TagName().Name() != kind {
				continue
			}
			mergeProps(into, e)
		}
	}
	return into
}

// mergeProps sets the attributes of the element e in p.
func mergeProps(p props, e roxmltree.Node) {
	for a := range e.Attributes() {
		ns, _ := a.Namespace()
		prefix, ok := prefixes[ns]
		if !ok {
			continue
		}
		key := prefix + ":" + a.Name()
		v := a.Value()
		if key == "fo:font-size" && strings.HasSuffix(v, "%") {
			if base, ok := length(p[key]); ok {
				if f, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64); err == nil {
					v = strconv.FormatFloat(base*f/100, 'f', -1, 64) + "pt"
				}
			}
		}
		p[key] = v
	}
}

// listStyleOf returns the nearest text:list-style that the graphic
// properties of the chain hold, as a presentation style does.
func listStyleOf(chain []roxmltree.Node) (roxmltree.Node, bool) {
	for i := len(chain) - 1; i >= 0; i-- {
		for e := range chain[i].Children() {
			if !e.IsElement() || e.TagName().Name() != "graphic-properties" {
				continue
			}
			if l, ok := firstChild(e, nsText, "list-style"); ok {
				return l, true
			}
		}
	}
	return roxmltree.Node{}, false
}

// length parses a length of OpenDocument, such as "2cm" or "12pt", in
// points. It returns false for a length without a unit or a percentage.
func length(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	units := []struct {
		suffix string
		pt     float64
	}{
		{"cm", 72 / 2.54}, {"mm", 72 / 25.4}, {"in", 72}, {"inch", 72},
		{"pt", 1}, {"pc", 12}, {"px", 0.75},
	}
	for _, u := range units {
		if rest, ok := strings.CutSuffix(s, u.suffix); ok {
			v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
				return 0, false
			}
			return min(max(v*u.pt, -1e7), 1e7), true
		}
	}
	return 0, false
}

// percent parses a percentage, such as "45%", as a fraction.
func percent(s string) (float64, bool) {
	rest, ok := strings.CutSuffix(strings.TrimSpace(s), "%")
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(rest, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v / 100, true
}

// float parses a number.
func float(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}
