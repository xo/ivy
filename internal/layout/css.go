// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/__init__.py: it finds and applies CSS.
//
// The file takes care of steps 3 and 4 of the "CSS 2.1 processing model": it
// retrieves the style sheets of a document, and annotates each element with
// a value for each CSS property. See
// https://www.w3.org/TR/CSS21/intro.html#processing-model.
//
// GetAllComputedStyles does everything, with the other functions of this
// file.

package layout

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"math"
	"math/bits"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/xo/okage/cssselect2"
	"github.com/xo/okage/tinycss2"

	"github.com/xo/ivy"
)

// pseudoElements is PSEUDO_ELEMENTS in weasyprint/css/__init__.py. The
// empty string is the None of upstream. Any other pseudo-element is
// rejected.
var pseudoElements = setOf(
	"", "before", "after", "marker", "first-line", "first-letter",
	"footnote-call", "footnote-marker", "note-call", "note-marker", "note-callback")

// pageMarginBoxes is PAGE_MARGIN_BOXES in weasyprint/css/__init__.py.
var pageMarginBoxes = setOf(
	"bottom-center", "bottom-left", "bottom-left-corner", "bottom-right",
	"bottom-right-corner", "left-bottom", "left-middle", "left-top",
	"right-bottom", "right-middle", "right-top", "top-center", "top-left",
	"top-left-corner", "top-right", "top-right-corner", "footnote", "note-area")

// PageSelectorType is PageSelectorType in weasyprint/css/__init__.py: what
// a selector of @page selects.
type PageSelectorType struct {
	// Side is "left", "right", or "" for None.
	Side string
	// Blank and First are True when set, and None when false.
	Blank bool
	First bool
	// Index is the tuple (a, b, name) of :nth(), or nil for None.
	Index *PageIndex
	// Name is the name of the page, or "" for None.
	Name string
}

// PageIndex is the index of a PageSelectorType: the An+B of :nth(), and the
// name of a page group after "of", or "" for None.
type PageIndex struct {
	A, B  int
	Group string
}

// PageType is PageType in weasyprint/layout/page.py: the type of a page,
// which the selectors of @page match.
type PageType struct {
	// Side is "left" or "right".
	Side string
	// Blank is true for a blank page.
	Blank bool
	// Name is the name of the page.
	Name string
	// Index is the index of the page, from 0.
	Index int
	// Groups are the page groups of the page, with the index of the page in
	// each one.
	Groups []PageGroup
}

// PageGroup is a page group of a PageType: its name and the index of the
// page in it.
type PageGroup struct {
	Name  string
	Index int
}

// key returns a string that tells page types apart, so that a page type can
// be a key of a map, as the namedtuple of upstream is.
//
// Port: the layout calls it for each page and each style of a page, so it
// appends with strconv and not with fmt. The text is the same.
func (p PageType) key() string {
	b := make([]byte, 0, 32+len(p.Side)+len(p.Name))
	b = strconv.AppendQuote(b, p.Side)
	b = append(b, ' ')
	b = strconv.AppendBool(b, p.Blank)
	b = append(b, ' ')
	b = strconv.AppendQuote(b, p.Name)
	b = append(b, ' ')
	b = strconv.AppendInt(b, int64(p.Index), 10)
	for _, g := range p.Groups {
		b = append(b, ' ')
		b = strconv.AppendQuote(b, g.Name)
		b = append(b, ' ')
		b = strconv.AppendInt(b, int64(g.Index), 10)
	}
	return string(b)
}

// StyleOwner is what a style belongs to: an *Element, or a PageType for the
// styles of the pages. It is the element of the keys of the dictionaries of
// StyleFor in upstream, which is an element or a page type.
type StyleOwner interface {
	ownerKey() any
}

func (e *Element) ownerKey() any { return e }
func (p PageType) ownerKey() any { return p.key() }

// styleKey is the key of a style: an owner and a pseudo-element type, ""
// for None.
type styleKey struct {
	owner  any
	pseudo string
}

// styleMap maps the keys of the styles to values. It keeps the keys of the
// elements without a pseudo-element apart, in a map whose key is the
// pointer to the element.
//
// Port: upstream has a dict with the tuple (element, pseudo_type) as its
// key. A key of Go with an interface hashes slower, and the cascade looks
// up the style of each element more than once.
type styleMap[V any] struct {
	elements map[*Element]V
	others   map[styleKey]V
}

// newStyleMap returns an empty styleMap.
func newStyleMap[V any]() styleMap[V] {
	return styleMap[V]{elements: map[*Element]V{}, others: map[styleKey]V{}}
}

// get returns the value of k.
func (m styleMap[V]) get(k styleKey) (V, bool) {
	if k.pseudo == "" {
		if e, ok := k.owner.(*Element); ok {
			v, ok := m.elements[e]
			return v, ok
		}
	}
	v, ok := m.others[k]
	return v, ok
}

// set sets the value of k.
func (m styleMap[V]) set(k styleKey, v V) {
	if k.pseudo == "" {
		if e, ok := k.owner.(*Element); ok {
			m.elements[e] = v
			return
		}
	}
	m.others[k] = v
}

// newStyleKey makes the key of a style.
func newStyleKey(owner StyleOwner, pseudo string) styleKey {
	return styleKey{owner.ownerKey(), pseudo}
}

// weight is the weight of a cascaded value: the tuple (precedence,
// layer_order, specificity) of upstream. A value of a greater weight wins.
// See https://www.w3.org/TR/CSS21/cascade.html#cascading-order.
type weight struct {
	precedence  int
	layerOrder  float64
	specificity cssselect2.Specificity
}

// compare compares two weights as Python compares two tuples.
func (w weight) compare(o weight) int {
	if c := w.precedence - o.precedence; c != 0 {
		return c
	}
	if w.layerOrder != o.layerOrder {
		if w.layerOrder < o.layerOrder {
			return -1
		}
		return 1
	}
	return w.specificity.Compare(o.specificity)
}

// cascadedValue is a value of a cascaded style and its weight.
type cascadedValue struct {
	value  Value
	weight weight
}

// cascadedStyles holds the cascaded styles of a document, as the dict
// _cascaded_styles of upstream. It keeps the order of insertion, as a dict
// of Python does.
type cascadedStyles struct {
	styles styleMap[map[string]cascadedValue]
	// owners holds the owners that are not an element. The key of a style
	// of an element holds the element.
	owners map[styleKey]StyleOwner
	order  []styleKey
	// pseudos holds the keys of the pseudo-elements of each owner, by the
	// key of the owner, in the order of insertion.
	//
	// Port: upstream finds the pseudo-elements of a page in all the
	// cascaded styles, which holds the styles of each page before it. That
	// takes a time that grows as the square of the number of pages.
	pseudos map[any][]styleKey
}

// newCascadedStyles makes empty cascaded styles.
func newCascadedStyles() *cascadedStyles {
	return &cascadedStyles{
		styles:  newStyleMap[map[string]cascadedValue](),
		owners:  map[styleKey]StyleOwner{},
		pseudos: map[any][]styleKey{},
	}
}

// setDefault is dict.setdefault for a style: it returns the cascaded style
// of an owner and a pseudo-element type, and adds an empty one first if
// needed.
func (c *cascadedStyles) setDefault(owner StyleOwner, pseudo string) map[string]cascadedValue {
	style, ok := c.styles.get(newStyleKey(owner, pseudo))
	if !ok {
		style = map[string]cascadedValue{}
		c.add(owner, pseudo, style)
	}
	return style
}

// add adds the cascaded style of an owner and a pseudo-element type, which
// has none yet.
func (c *cascadedStyles) add(owner StyleOwner, pseudo string, style map[string]cascadedValue) {
	key := newStyleKey(owner, pseudo)
	c.styles.set(key, style)
	if _, isElement := owner.(*Element); !isElement {
		c.owners[key] = owner
	}
	c.order = append(c.order, key)
	if pseudo != "" {
		c.pseudos[key.owner] = append(c.pseudos[key.owner], key)
	}
}

// get returns the cascaded style of an owner and a pseudo-element type, or
// nil.
func (c *cascadedStyles) get(owner StyleOwner, pseudo string) map[string]cascadedValue {
	style, _ := c.styles.get(newStyleKey(owner, pseudo))
	return style
}

// owner returns the owner of a key of the cascaded styles.
func (c *cascadedStyles) owner(k styleKey) StyleOwner {
	if e, ok := k.owner.(*Element); ok {
		return e
	}
	return c.owners[k]
}

// clear removes the cascaded styles of the elements and the order of the
// keys.
//
// Port: upstream removes each cascaded style. clear keeps the cascaded
// styles of the pseudo-elements of the elements, for StyleFor.pseudoStyle.
func (c *cascadedStyles) clear() {
	clear(c.styles.elements)
	clear(c.owners)
	clear(c.pseudos)
	c.order = nil
}

// CascadedKey is a key of the cascaded styles: an owner and a
// pseudo-element type.
type CascadedKey struct {
	Owner  StyleOwner
	Pseudo string
}

// apply adds the declarations to a cascaded style, where their weight wins.
func applyDeclarations(style map[string]cascadedValue, declarations []Declaration, origin string, layerOrder float64, specificity cssselect2.Specificity) {
	for _, d := range declarations {
		w := weight{declarationPrecedence(origin, d.Important), layerOrder, specificity}
		if old, ok := style[d.Name]; !ok || old.weight.compare(w) <= 0 {
			style[d.Name] = cascadedValue{d.Value, w}
		}
	}
}

// sheetOrigin is an item of the list of style sheets of StyleFor: a style
// sheet, its origin, and a specificity that replaces the specificity of
// its selectors when set.
type sheetOrigin struct {
	sheet          *CSS
	origin         string
	specificity    cssselect2.Specificity
	hasSpecificity bool
	layerIndex     map[string]int
}

// InitialPageSizes holds the sizes of the first page, which the units of
// the viewport use: the size of the page box and the size of the page area,
// in pixels. A nil size is not set yet.
type InitialPageSizes struct {
	Box  *[2]float64
	Area *[2]float64
}

// FontConfig receives the @font-face rules of the style sheets. It is the
// hook of the class FontConfiguration of weasyprint/text/fonts.py, which the
// stage of the port that loads fonts fills.
type FontConfig interface {
	// AddFontFace is FontConfiguration.add_font_face. descriptors holds the
	// descriptors of the rule, by name in snake case, such as "src" and
	// "font_family".
	AddFontFace(descriptors map[string]Value, fetcher *URLFetcher)
}

// FontFaces is a FontConfig that keeps the @font-face rules, for a later
// stage of the port.
type FontFaces struct {
	Rules []map[string]Value
}

// AddFontFace satisfies FontConfig.
func (f *FontFaces) AddFontFace(descriptors map[string]Value, _ *URLFetcher) {
	f.Rules = append(f.Rules, descriptors)
}

// StyleContext holds what the styles of a document share: the attributes
// font_config, cache and initial_page_sizes that each style dictionary of
// upstream holds, and the logger and the limits of the port.
type StyleContext struct {
	// FontConfig is the font configuration of the document, or nil.
	FontConfig FontConfig
	// Metrics measures the fonts, or nil for the default metrics.
	Metrics TextMetrics
	// InitialPageSizes holds the sizes of the first page. The layout sets
	// them when it makes the first page.
	InitialPageSizes InitialPageSizes
	// Cache is the cache that the code of the text uses.
	Cache map[any]any

	log    *logger
	budget *cascadeBudget
}

// cascadeBudget bounds the work of the cascade on hostile style sheets.
//
// Port: upstream has no limits. The limits come from ivy.Limits: the rules,
// the declarations that the cascade applies and the computed styles count
// against 64 times MaxObjects, the imports nest at most MaxDepth deep and
// count at most maxImports, the style sheets read at most MaxMemory bytes,
// and the nested style rules nest at most MaxDepth deep.
type cascadeBudget struct {
	limits   ivy.Limits
	work     int
	rules    int
	imports  int
	bytes    int64
	nesting  int
	groups   int
	exceeded bool
}

// maxImports is the largest number of @import rules that a document follows.
const maxImports = 1024

// maxVarTokens is the largest number of tokens that var() substitutes in the
// value of one property.
//
// Port: upstream has no limit, and a chain of custom properties that each
// use the previous one twice makes a value of exponential size.
const maxVarTokens = 1 << 16

// newCascadeBudget makes a budget of the limits.
func newCascadeBudget(limits ivy.Limits) *cascadeBudget {
	return &cascadeBudget{limits: limits.WithDefaults()}
}

// spend counts n units of work. It returns false past the limit.
func (b *cascadeBudget) spend(n int) bool {
	if b == nil {
		return true
	}
	b.work += n
	if b.work > 64*b.limits.MaxObjects {
		b.exceeded = true
		return false
	}
	return true
}

// rule counts a rule. It returns false past the limit.
func (b *cascadeBudget) rule() bool {
	if b == nil {
		return true
	}
	b.rules++
	return b.rules <= b.limits.MaxObjects
}

// nest counts a level of nested style rules. It returns false past the
// limit.
func (b *cascadeBudget) nest() bool {
	if b == nil {
		return true
	}
	if b.nesting >= b.limits.MaxDepth {
		return false
	}
	b.nesting++
	return true
}

// unnest leaves a level of nested style rules.
func (b *cascadeBudget) unnest() {
	if b != nil {
		b.nesting--
	}
}

// nextGroup returns a new group of selectors.
func (b *cascadeBudget) nextGroup() int {
	if b == nil {
		return 0
	}
	b.groups++
	return b.groups
}

// StyleFor is StyleFor in weasyprint/css/__init__.py. It gives the computed
// styles of the elements and of the pages of a document.
//
// The styles are computed lazily, when a property is read, as upstream does.
// So a StyleFor and its styles are not safe for concurrent use.
type StyleFor struct {
	// Context holds what the styles share.
	Context *StyleContext

	cascaded *cascadedStyles
	computed styleMap[*Style]
	sheets   []sheetOrigin
	// styleOwners maps each computed style that the cascade makes for an
	// element to that element. An element that shares the computed style
	// of an earlier element shares the styles of its pseudo-elements too.
	styleOwners map[*Style]*Element

	// uses holds the features of the styles that need the whole layout
	// before a page (D43).
	uses wholeLayoutUses

	// root and baseURL are the root element and the base URL of the
	// document, for pseudoStyle.
	root    *Element
	baseURL string
}

// newStyleFor is StyleFor.__init__ in weasyprint/css/__init__.py.
func newStyleFor(html *HTML, sheets []sheetOrigin, presentationalHints bool, ctx *StyleContext, targetCollector *TargetCollector) (*StyleFor, error) {
	sf := &StyleFor{
		Context:     ctx,
		cascaded:    newCascadedStyles(),
		computed:    newStyleMap[*Style](),
		sheets:      sheets,
		styleOwners: map[*Style]*Element{},
	}
	budget := ctx.budget
	log := ctx.log

	layerOrder := math.Inf(1)
	elementsDeclarations := map[*Element]bool{}
	for _, attr := range findStyleAttributes(html.EtreeElement, presentationalHints, html.BaseURL) {
		style := sf.cascaded.setDefault(attr.element, "")
		elementsDeclarations[attr.element] = true
		declarations := preprocessDeclarations(log, attr.baseURL, attr.declarations)
		if !budget.spend(len(declarations)) {
			return nil, fmt.Errorf("%w: the cascade of the document needs too much work", ivy.ErrLimit)
		}
		applyDeclarations(style, declarations, "author", layerOrder, attr.specificity)
	}

	// First, add declarations and set computed styles for "real" elements
	// in tree order. Tree order is important so that parents have computed
	// styles before their children, for inheritance.

	// Iterate on all elements, even if there is no cascaded style for them.
	//
	// Port: the key of computedCache is made of numbers and not with fmt,
	// with the same parts as upstream: the parent style, the element when
	// it has declarations of its own, and each match. styleIDs numbers the
	// parent styles. When the key is in the cache, the cascaded style of
	// the element is not made, because nothing reads it. Its declarations
	// are the declarations of the element that made the cached style.
	//
	// Port: the pseudo-elements of an element whose key is in the cache
	// share the computed styles of the pseudo-elements of the element that
	// made the cached style. The key holds each match, so the
	// pseudo-elements have the same declarations, and their parent is the
	// same style. Upstream makes them again for each element. The rules of
	// the user agent, such as ::marker and ::footnote-call, give each
	// element six pseudo-elements, so they were most of the work of the
	// cascade of a large table. sharedWork counts the styles that the
	// elements share, as upstream makes them again. anchors holds the
	// elements whose pseudo-elements can set an anchor, and shared holds
	// the elements that share their styles, in tree order.
	computedCache := map[string]*Style{}
	cacheOwners := map[string]*Element{}
	var shared []*Element
	sharedWork := 0
	anchors := map[*Element]bool{}
	styleIDs := map[*Style]int{}
	// cascadedCache holds the cascaded styles by the parts of key of their
	// matches. keyStart and keyEnd of a match are the bounds of its part.
	cascadedCache := map[string]map[string]cascadedValue{}
	type sheetMatch struct {
		sheet            int
		match            cssselect2.Match[*rulePayload]
		layerOrder       float64
		keyStart, keyEnd int
	}
	var matches []sheetMatch
	// found holds the matches of one sheet. Matcher.AppendMatches reuses
	// it, so the cascade does not allocate a slice for each element.
	var found []cssselect2.Match[*rulePayload]
	var key, cascadedKey []byte
	var pseudos []string
	elementIndex := 0
	for element := range html.WrapperElement.IterSubtree() {
		elementIndex++
		etreeElement := wrapperElement(element)
		var parent *Element
		var parentStyle *Style
		if element.Parent != nil {
			parent = wrapperElement(element.Parent)
			parentStyle, _ = sf.computed.get(newStyleKey(parent, ""))
		}
		key = key[:0]
		if parentStyle != nil {
			id, ok := styleIDs[parentStyle]
			if !ok {
				id = len(styleIDs) + 1
				styleIDs[parentStyle] = id
			}
			key = strconv.AppendInt(key, int64(id), 10)
		}
		if elementsDeclarations[etreeElement] {
			key = append(key, " e"...)
			key = strconv.AppendInt(key, int64(elementIndex), 10)
		}
		matches = matches[:0]
		for i, sheet := range sheets {
			// Add declarations for matching elements.
			found = sheet.sheet.Matcher.AppendMatches(found[:0], element)
			for _, match := range found {
				payload := match.Payload
				layerOrder := math.Inf(1)
				if payload.hasLayer {
					if index, ok := sheet.layerIndex[payload.layer]; ok {
						layerOrder = float64(index)
					}
				}
				keyStart := len(key)
				key = append(key, '|')
				key = strconv.AppendInt(key, int64(i), 10)
				for _, n := range match.Specificity {
					key = append(key, ' ')
					key = strconv.AppendInt(key, int64(n), 10)
				}
				key = append(key, ' ')
				key = strconv.AppendInt(key, int64(payload.id), 10)
				key = append(key, ' ')
				key = strconv.AppendFloat(key, layerOrder, 'g', -1, 64)
				matches = append(matches, sheetMatch{i, match, layerOrder, keyStart, len(key)})
			}
		}
		cached, hit := computedCache[string(key)]
		pseudo := false
		pseudos = pseudos[:0]
		for _, m := range matches {
			if !budget.spend(1 + len(m.match.Payload.declarations)) {
				return nil, fmt.Errorf("%w: the cascade of the document needs too much work", ivy.ErrLimit)
			}
			p := m.match.PseudoElement
			if p != "" {
				pseudo = true
			}
			if !hit && !slices.Contains(pseudos, p) {
				pseudos = append(pseudos, p)
			}
		}
		// Port: upstream adds the declarations of each match to the
		// cascaded style of its pseudo-element, in the order of the
		// matches. The port does the same for each pseudo-element in the
		// order of its first match. The cascaded styles of the same
		// matches are the same, so the elements share them through
		// cascadedCache. The cascaded style of an element with
		// declarations of its own is not shared.
		if n := len(pseudos); n > 1 {
			// Make room for the keys of the pseudo-elements at once.
			sf.cascaded.pseudos[etreeElement] = make([]styleKey, 0, n)
		}
		for _, p := range pseudos {
			var style map[string]cascadedValue
			if p == "" && elementsDeclarations[etreeElement] {
				style = sf.cascaded.setDefault(etreeElement, p)
			} else {
				cascadedKey = cascadedKey[:0]
				for _, m := range matches {
					if m.match.PseudoElement == p {
						cascadedKey = append(cascadedKey, key[m.keyStart:m.keyEnd]...)
					}
				}
				if same, ok := cascadedCache[string(cascadedKey)]; ok {
					sf.cascaded.add(etreeElement, p, same)
					continue
				}
				style = map[string]cascadedValue{}
				cascadedCache[string(cascadedKey)] = style
				sf.cascaded.add(etreeElement, p, style)
			}
			for _, m := range matches {
				if m.match.PseudoElement != p {
					continue
				}
				sheet, match := sheets[m.sheet], m.match
				specificity := match.Specificity
				if sheet.hasSpecificity {
					specificity = sheet.specificity
				}
				applyDeclarations(style, match.Payload.declarations, sheet.origin, m.layerOrder, specificity)
			}
		}

		// Store computed styles.
		if hit {
			// Equivalent computed style already exists, share it.
			sf.computed.set(newStyleKey(etreeElement, ""), cached)
			if pseudo {
				owner := cacheOwners[string(key)]
				sharedWork += len(sf.cascaded.pseudos[owner])
				if anchors[owner] {
					shared = append(shared, etreeElement)
				}
			}
		} else {
			// No equivalent computed style found, create it.
			var parentOwner StyleOwner
			if parent != nil {
				parentOwner = parent
			}
			if err := sf.SetComputedStyles(etreeElement, parentOwner, html.EtreeElement, "", html.BaseURL, targetCollector); err != nil {
				return nil, err
			}
			style, _ := sf.computed.get(newStyleKey(etreeElement, ""))
			computedCache[string(key)] = style
			cacheOwners[string(key)] = etreeElement
			sf.styleOwners[style] = etreeElement
			for _, m := range matches {
				if m.match.PseudoElement != "" && slices.ContainsFunc(m.match.Payload.declarations, isAnchor) {
					anchors[etreeElement] = true
				}
			}
		}
	}

	// Then computed styles for pseudo elements, in any order.
	// Pseudo-elements inherit from their associated element so they come
	// last. Do them in a second pass as there is no easy way to iterate on
	// the pseudo-elements for a given element with the current structure of
	// cascaded_styles. (Keys are (element, pseudo_type) tuples.)

	// Only iterate on pseudo-elements that have cascaded styles. (Others
	// might as well not exist.)
	//
	// Port: the style of a pseudo-element of an element is made when Get
	// asks for it, as the layout asks for few of them. The cascade reads
	// only the anchor of the style, and the anchor is none when the
	// cascaded style does not set it. So only the styles whose cascaded
	// style sets an anchor are made now. The work of each style is spent
	// now, as upstream makes each style now.
	for _, k := range slices.Clone(sf.cascaded.order) {
		if k.pseudo == "" {
			continue
		}
		owner := sf.cascaded.owner(k)
		if _, ok := owner.(*Element); ok {
			if _, anchor := sf.cascaded.get(owner, k.pseudo)["anchor"]; !anchor {
				if !budget.spend(1) {
					return nil, fmt.Errorf("%w: the document has too many styles", ivy.ErrLimit)
				}
				continue
			}
		}
		if err := sf.SetComputedStyles(owner, owner, html.EtreeElement, k.pseudo, html.BaseURL, targetCollector); err != nil {
			return nil, err
		}
	}
	if !budget.spend(sharedWork) {
		return nil, fmt.Errorf("%w: the document has too many styles", ivy.ErrLimit)
	}
	if targetCollector != nil {
		for _, element := range shared {
			sf.sharePseudoStyles(element, targetCollector)
		}
	}

	// Port: find the features that need the whole layout before the
	// cascaded styles go (D43).
	sf.uses = findWholeLayoutUses(sf.cascaded, sf.sheets)

	// Clear the cascaded styles, we do not need them anymore. Keep the
	// dictionary, it is used later for page margins.
	//
	// Port: the cascaded styles of the pseudo-elements stay for Get.
	sf.cascaded.clear()
	sf.root, sf.baseURL = html.EtreeElement, html.BaseURL
	return sf, nil
}

// sharePseudoStyles collects again the anchors of the styles of the
// pseudo-elements that an element shares, as SetComputedStyles collects
// them for the element that made them. The target collector logs that
// each one is defined twice. newStyleFor spends the work of these styles.
func (sf *StyleFor) sharePseudoStyles(element *Element, targetCollector *TargetCollector) {
	for _, k := range sf.cascaded.pseudos[sf.pseudoOwner(element)] {
		// A style that is not made has no anchor.
		if style, ok := sf.computed.get(k); ok {
			if anchor := style.Get("anchor"); truthy(anchor) {
				targetCollector.CollectAnchor(str(anchor))
			}
		}
	}
}

// isAnchor reports whether a declaration sets anchor.
func isAnchor(d Declaration) bool {
	return d.Name == "anchor"
}

// pseudoOwner returns the element whose pseudo-elements give the styles of
// the pseudo-elements of element: the element that made its computed
// style.
func (sf *StyleFor) pseudoOwner(element *Element) *Element {
	if style, _ := sf.computed.get(newStyleKey(element, "")); style != nil {
		if owner := sf.styleOwners[style]; owner != nil {
			return owner
		}
	}
	return element
}

// Get is StyleFor.__call__ in weasyprint/css/__init__.py. It returns the
// computed style of an owner and a pseudo-element type, "" for None, or nil
// when there is none.
func (sf *StyleFor) Get(owner StyleOwner, pseudoType string) *Style {
	if e, ok := owner.(*Element); ok && pseudoType != "" {
		// The element shares the styles of the pseudo-elements of the
		// element that made its computed style.
		owner = sf.pseudoOwner(e)
	}
	key := newStyleKey(owner, pseudoType)
	style, _ := sf.computed.get(key)
	if style == nil {
		style = sf.pseudoStyle(key)
	}
	if style == nil {
		return nil
	}
	display := style.Get("display")
	if tupleHas(display, "table") && isStr(style.Get("border_collapse"), "collapse") {
		// Padding does not apply.
		for _, side := range [...]string{"top", "bottom", "left", "right"} {
			style.Set("padding_"+side, ZeroPixels)
		}
	}
	if t, ok := display.(Tuple); ok && len(t) == 1 {
		if d := str(t[0]); strings.HasPrefix(d, "table-") && d != "table-caption" {
			// Margins do not apply.
			for _, side := range [...]string{"top", "bottom", "left", "right"} {
				style.Set("margin_"+side, ZeroPixels)
			}
		}
	}
	return style
}

// pseudoStyle makes the style of a pseudo-element of an element that
// newStyleFor did not make, as SetComputedStyles does, or returns nil when
// the pseudo-element has no cascaded style.
func (sf *StyleFor) pseudoStyle(key styleKey) *Style {
	owner, ok := key.owner.(*Element)
	if !ok || key.pseudo == "" {
		return nil
	}
	cascaded, ok := sf.cascaded.styles.get(key)
	if !ok {
		return nil
	}
	parentStyle, _ := sf.computed.get(newStyleKey(owner, ""))
	rootStyle, _ := sf.computed.get(newStyleKey(sf.root, ""))
	if parentStyle == nil || rootStyle == nil {
		return nil
	}
	style := newComputedStyle(parentStyle, cascaded, key.pseudo, rootStyle, sf.baseURL, sf.Context)
	sf.computed.set(key, style)
	return style
}

// SetComputedStyles is StyleFor.set_computed_styles in
// weasyprint/css/__init__.py. It sets the computed values of the styles of
// an element or a page.
//
// It takes the properties that the cascade left on an element or a
// pseudo-element, and assigns the computed values with respect to the
// cascade, the declaration priority (ie. !important) and the selector
// specificity. A nil parent is the None of upstream, for the root element.
func (sf *StyleFor) SetComputedStyles(element, parent StyleOwner, root *Element, pseudoType, baseURL string, targetCollector *TargetCollector) error {
	if !sf.Context.budget.spend(1) {
		return fmt.Errorf("%w: the document has too many styles", ivy.ErrLimit)
	}
	var parentStyle, rootStyle *Style
	if el, ok := element.(*Element); ok && el == root && pseudoType == "" {
		rootStyle = newInitialStyle(sf.Context)
	} else {
		if parent == nil {
			// Port: upstream asserts that a parent is given.
			return fmt.Errorf("%w: style without a parent", ivy.ErrFormat)
		}
		parentStyle, _ = sf.computed.get(newStyleKey(parent, ""))
		rootStyle, _ = sf.computed.get(newStyleKey(root, ""))
		if parentStyle == nil || rootStyle == nil {
			// Port: upstream raises KeyError.
			return fmt.Errorf("%w: style of a parent that has no style", ivy.ErrFormat)
		}
	}

	// Port: upstream gives an empty dict when the owner has no cascaded
	// style. The port gives a nil map, which nothing writes.
	cascaded := sf.cascaded.get(element, pseudoType)
	computed := newComputedStyle(parentStyle, cascaded, pseudoType, rootStyle, baseURL, sf.Context)
	sf.computed.set(newStyleKey(element, pseudoType), computed)
	if targetCollector != nil {
		if anchor := computed.Get("anchor"); truthy(anchor) {
			targetCollector.CollectAnchor(str(anchor))
		}
	}
	return nil
}

// AddPageDeclarations is StyleFor.add_page_declarations in
// weasyprint/css/__init__.py. It adds the declarations of the @page rules
// that match a page type to the cascaded styles.
func (sf *StyleFor) AddPageDeclarations(pageType PageType) {
	// TODO: use real layer order.
	layerOrder := 0.0
	for _, sheet := range sf.sheets {
		for _, rule := range *sheet.sheet.PageRules {
			for _, selector := range rule.Selectors {
				if !pageTypeMatch(selector.Type, pageType) {
					continue
				}
				specificity := selector.Specificity
				if sheet.hasSpecificity {
					specificity = sheet.specificity
				}
				style := sf.cascaded.setDefault(pageType, selector.PseudoType)
				applyDeclarations(style, rule.Declarations, sheet.origin, layerOrder, specificity)
			}
		}
	}
}

// CascadedKeys is the iteration on get_cascaded_styles() of upstream: the
// keys of the cascaded styles, in the order of insertion.
func (sf *StyleFor) CascadedKeys() []CascadedKey {
	keys := make([]CascadedKey, len(sf.cascaded.order))
	for i, k := range sf.cascaded.order {
		keys[i] = CascadedKey{sf.cascaded.owner(k), k.pseudo}
	}
	return keys
}

// pseudoKeys returns the keys of the cascaded styles of the pseudo-elements
// of owner, in the order of insertion. It gives the same keys as the
// CascadedKeys of the owner with a pseudo-element type, in the same order.
func (sf *StyleFor) pseudoKeys(owner StyleOwner) []CascadedKey {
	keys := sf.cascaded.pseudos[owner.ownerKey()]
	out := make([]CascadedKey, len(keys))
	for i, k := range keys {
		out[i] = CascadedKey{sf.cascaded.owner(k), k.pseudo}
	}
	return out
}

// pageTypeMatch is StyleFor._page_type_match in weasyprint/css/__init__.py.
func pageTypeMatch(selector PageSelectorType, pageType PageType) bool {
	if selector.Side != "" && selector.Side != pageType.Side {
		return false
	}
	if selector.Blank && !pageType.Blank {
		return false
	}
	if selector.First && pageType.Index != 0 {
		return false
	}
	if selector.Name != "" && selector.Name != pageType.Name {
		return false
	}
	if selector.Index != nil {
		a, b, name := selector.Index.A, selector.Index.B, selector.Index.Group
		matches := func(index int) bool {
			offset := index + 1 - b
			if a == 0 {
				return offset == 0
			}
			return float64(offset)/float64(a) >= 0 && pyMod(offset, a) == 0
		}
		if name == "" {
			return matches(pageType.Index)
		}
		if name != pageType.Name {
			return false
		}
		for _, group := range pageType.Groups {
			if name != group.Name {
				continue
			}
			if matches(group.Index) {
				return true
			}
		}
		return false
	}
	return true
}

// stringToCSS is string_to_css in weasyprint/css/__init__.py. It turns a
// string into a CSS string with escapes and quotes.
func stringToCSS(s string) string {
	s = strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\A`,
		"\r", `\D`,
		"\f", `\C`,
	).Replace(s)
	return `"` + s + `"`
}

// urlToCSS is url_to_css in weasyprint/css/__init__.py. It turns a URL into
// a url() string.
func urlToCSS(s string) string {
	return "url(" + stringToCSS(s) + ")"
}

// getChildText is get_child_text in weasyprint/css/__init__.py. It returns
// the text directly in the element, not in its descendants.
func getChildText(element *Element) string {
	var b strings.Builder
	b.WriteString(element.Text)
	for _, child := range element.Children {
		b.WriteString(child.Tail)
	}
	return b.String()
}

// textDecoration is text_decoration in weasyprint/css/__init__.py. The
// text-decoration-* properties are not inherited but propagated using
// specific rules. See
// https://drafts.csswg.org/css-text-decor-3/#line-decoration.
func textDecoration(key string, value, parentValue Value, cascaded bool) Value {
	// TODO: these rules do not follow the specification.
	switch key {
	case "text_decoration_color", "text_decoration_style", "text_decoration_thickness":
		if !cascaded {
			value = parentValue
		}
	case "text_decoration_line":
		if !isStr(parentValue, "none") {
			if isStr(value, "none") {
				value = parentValue
			} else {
				vs, _ := value.(StringSet)
				ps, _ := parentValue.(StringSet)
				value = vs.union(ps)
			}
		}
	}
	return value
}

// findStylesheets is find_stylesheets in weasyprint/css/__init__.py. It
// returns the style sheets of the elements style and link of the document,
// in the order of the document.
func findStylesheets(ctx *cssContext, root *cssselect2.ElementWrapper, deviceMediaType, baseURL string) []*CSS {
	// Port: upstream matches the selector list "style, link" on the wrappers
	// of cssselect2. The port reads the local names of the elements in the
	// same order, without a wrapper for each element of the document.
	var sheets []*CSS
	rootElement := wrapperElement(root)
	if rootElement == nil {
		return nil
	}
	for element := range rootElement.Iter() {
		if local, _ := element.localName(); local != "style" && local != "link" {
			continue
		}
		mimeType, _, _ := strings.Cut(element.GetDefault("type", "text/css"), ";")
		// Only keep 'type/subtype' from 'type/subtype ; param1; param2'.
		if strings.TrimSpace(mimeType) != "text/css" {
			continue
		}
		mediaAttr := strings.TrimSpace(element.GetDefault("media", ""))
		if mediaAttr == "" {
			mediaAttr = "all"
		}
		var media []string
		for mediaType := range strings.SplitSeq(mediaAttr, ",") {
			media = append(media, strings.TrimSpace(mediaType))
		}
		if !evaluateMediaQuery(media, deviceMediaType) {
			continue
		}
		switch element.Tag {
		case "style":
			// Content is text that is directly in the <style> element, not
			// its descendants.
			content := getChildText(element)
			sheets = append(sheets, newCSS(ctx, CSSSource{Text: content, BaseURL: baseURL}, deviceMediaType, nil))
		case "link":
			href, ok := element.Get("href")
			if !ok || href == "" {
				continue
			}
			if !elementHasLinkType(element, "stylesheet") || elementHasLinkType(element, "alternate") {
				continue
			}
			u, ok := parseURL(ctx.log, href, baseURL, false)
			if !ok {
				continue
			}
			sheet, err := newCSSFromURL(ctx, u, deviceMediaType, nil, true)
			if err != nil {
				ctx.log.error("Failed to load stylesheet at %s: %s", u, err)
				continue
			}
			sheets = append(sheets, sheet)
		}
	}
	return sheets
}

// styleAttribute is an item that find_style_attributes yields: the
// declarations of an element, with their specificity and base URL.
type styleAttribute struct {
	specificity  cssselect2.Specificity
	element      *Element
	declarations []tinycss2.Node
	baseURL      string
}

// findStyleAttributes is find_style_attributes in
// weasyprint/css/__init__.py. It returns the declarations of the attributes
// style, with the specificity (1, 0, 0), and of the presentational hints when
// presentationalHints is true, with the specificity (0, 0, 0).
func findStyleAttributes(tree *Element, presentationalHints bool, baseURL string) []styleAttribute {
	var result []styleAttribute
	for element := range tree.Iter() {
		result = appendStyleAttributes(result, element, presentationalHints, baseURL)
	}
	return result
}

// appendStyleAttributes adds the declarations of an element to result, for
// findStyleAttributes.
func appendStyleAttributes(result []styleAttribute, element *Element, presentationalHints bool, baseURL string) []styleAttribute {
	// Apply style attribute.
	if style, _ := element.Get("style"); style != "" {
		declarations := tinycss2.ParseBlocksContents(style)
		result = append(result, styleAttribute{cssselect2.Specificity{1, 0, 0}, element, declarations, baseURL})
	}

	parseDeclaration := func(attribute string, el *Element) {
		declaration := tinycss2.ParseOneDeclaration(attribute)
		result = append(result, styleAttribute{cssselect2.Specificity{}, el, []tinycss2.Node{declaration}, baseURL})
	}
	declare := func(attribute string) { parseDeclaration(attribute, element) }

	if lang, _ := element.Get("lang"); lang != "" {
		declare(`-weasy-lang:"` + lang + `"`)
	}

	if id, _ := element.Get("id"); id != "" {
		declare(`-weasy-anchor:"` + id + `"`)
	}

	if element.Tag == "a" {
		if name, _ := element.Get("name"); name != "" {
			declare(`-weasy-anchor:"` + name + `"`)
		}
	}

	if element.Tag == "{"+nsSVG+"}svg" {
		// Handle presentation attributes. Only do that for height and width
		// at top level now, so that intrinsic size is defined by CSS, not
		// SVG. See:
		// https://www.w3.org/TR/SVG2/styling.html#PresentationAttributes.
		for _, attribute := range [...]string{"width", "height"} {
			if length, _ := element.Get(attribute); length != "" {
				token := tinycss2.ParseOneComponentValue(length)
				if getLength(token, true, true) != nil {
					declare(attribute + ":" + length)
				}
			}
		}
	}

	// Apply presentational hints.
	if !presentationalHints {
		return result
	}

	textAlign := func() {
		align := strings.ToLower(element.GetDefault("align", ""))
		// TODO: we should align descendants too.
		if align == "middle" {
			declare("text-align:center")
		} else if in(align, "center", "left", "right", "justify") {
			declare("text-align:" + align)
		}
	}
	backgroundImage := func() {
		if background, ok := parseURL(nil, element.GetDefault("background", ""), baseURL, false); ok {
			declare("background-image:" + urlToCSS(background))
		}
	}
	backgroundColor := func() {
		if bgcolor, _ := element.Get("bgcolor"); bgcolor != "" {
			declare("background-color:" + legacyColorCSS(bgcolor))
		}
	}
	dimension := func(attribute, property string, ignoringZero bool) {
		if v, _ := element.Get(attribute); v != "" {
			var value string
			var ok bool
			if ignoringZero {
				value, ok = mapToDimensionPropertyIgnoringZero(v)
			} else {
				value, ok = mapToDimensionProperty(v)
			}
			if ok {
				declare(property + ":" + value)
			}
		}
	}

	switch element.Tag {
	case "body":
		// TODO: we should check the container frame element.
		for _, attribute := range [...]string{"marginheight", "topmargin"} {
			if value, ok := mapToPixelLength(element.GetDefault(attribute, "")); ok {
				declare("margin-top:" + value)
				declare("margin-bottom:" + value)
				break
			}
		}
		for _, attribute := range [...]string{"marginwidth", "leftmargin"} {
			if value, ok := mapToPixelLength(element.GetDefault(attribute, "")); ok {
				declare("margin-left:" + value)
				declare("margin-right:" + value)
				break
			}
		}
		backgroundImage()
		backgroundColor()
		if text, _ := element.Get("text"); text != "" {
			declare("color:" + legacyColorCSS(text))
		}
		// TODO: we should support link, vlink, alink.
	case "center":
		declare("text-align:center")
	case "div":
		textAlign()
	case "font":
		if color, _ := element.Get("color"); color != "" {
			declare("color:" + legacyColorCSS(color))
		}
		if face, _ := element.Get("face"); face != "" {
			declare("font-family:" + stringToCSS(face))
		}
		if size, _ := element.Get("size"); size != "" {
			size = strings.Trim(size, whitespace)
			relativePlus := strings.HasPrefix(size, "+")
			relativeMinus := strings.HasPrefix(size, "-")
			if relativePlus || relativeMinus {
				size = size[1:]
			}
			if n, ok := parseInteger(size); ok {
				fontSizes := map[int]string{
					1: "x-small",
					2: "small",
					3: "medium",
					4: "large",
					5: "x-large",
					6: "xx-large",
					7: "48px", // 1.5 * xx-large
				}
				if relativePlus {
					n += 3
				} else if relativeMinus {
					n -= 3
				}
				n = max(1, min(7, n))
				declare("font-size:" + fontSizes[n])
			}
		}
	case "table":
		if cellspacing, _ := element.Get("cellspacing"); cellspacing != "" {
			if value, ok := mapToPixelLength(cellspacing); ok {
				declare("border-spacing:" + value)
			}
		}
		if cellpadding, _ := element.Get("cellpadding"); cellpadding != "" {
			if value, ok := mapToPixelLength(cellpadding); ok {
				// TODO: do not match subtables cells.
				for subelement := range element.Iter() {
					if subelement.Tag == "td" || subelement.Tag == "th" {
						parseDeclaration("padding:"+value, subelement)
					}
				}
			}
		}
		dimension("width", "width", true)
		dimension("height", "height", false)
		backgroundImage()
		backgroundColor()
		if bordercolor, _ := element.Get("bordercolor"); bordercolor != "" {
			declare("border-color:" + legacyColorCSS(bordercolor))
		}
		if border, _ := element.Get("border"); border != "" {
			if value, ok := mapToPixelLength(border); ok {
				declare("border-width:" + value)
			}
		}
	case "tr", "td", "th", "thead", "tbody", "tfoot":
		textAlign()
		backgroundImage()
		backgroundColor()
		switch element.Tag {
		case "td", "th":
			dimension("height", "height", true)
			dimension("width", "width", true)
		case "tr":
			dimension("height", "height", false)
		}
	case "caption":
		textAlign()
	case "col":
		dimension("width", "width", false)
	case "hr":
		size, ok := parseNonNegativeInteger(element.GetDefault("size", ""))
		if !ok {
			size = 0
		}
		_, hasColor := element.Get("color")
		_, hasNoshade := element.Get("noshade")
		switch {
		case hasColor || hasNoshade:
			if size >= 1 {
				declare("border-width:" + pyStrNumber(float64(size)/2) + "px")
			}
		case size == 1:
			declare("border-bottom-width:0")
		case size > 1:
			declare("height:" + strconv.Itoa(size-2) + "px")
		}
		dimension("width", "width", false)
		if color, _ := element.Get("color"); color != "" {
			declare("color:" + legacyColorCSS(color))
		}
	case "iframe", "applet", "embed", "img", "input", "object":
		if element.Tag != "input" || strings.ToLower(element.GetDefault("type", "")) == "image" {
			align := strings.ToLower(element.GetDefault("align", ""))
			if align == "middle" || align == "center" {
				// TODO: middle and center values are wrong.
				declare("vertical-align:middle")
			}
			if hspace, _ := element.Get("hspace"); hspace != "" {
				if value, ok := mapToDimensionProperty(hspace); ok {
					declare("margin-left:" + value)
					declare("margin-right:" + value)
				}
			}
			if vspace, _ := element.Get("vspace"); vspace != "" {
				if value, ok := mapToDimensionProperty(vspace); ok {
					declare("margin-top:" + value)
					declare("margin-bottom:" + value)
				}
			}
			// TODO: img seems to be excluded for width and height, but a lot
			// of W3C tests rely on this attribute being applied to img.
			dimension("width", "width", false)
			dimension("height", "height", false)
			if element.Tag == "img" || element.Tag == "object" || element.Tag == "input" {
				if border, _ := element.Get("border"); border != "" {
					if value, ok := mapToPixelLength(border); ok {
						declare("border-width:" + value)
						declare("border-style:solid")
					}
				}
			}
		}
	case "ol":
		// From https://www.w3.org/TR/css-lists-3/#ua-stylesheet.
		if start, _ := element.Get("start"); start != "" {
			if value, ok := parseInteger(start); ok {
				declare("counter-reset:list-item " + strconv.Itoa(value))
				declare("counter-increment:list-item -1")
			}
		}
	case "li":
		// From https://www.w3.org/TR/css-lists-3/#ua-stylesheet.
		if v, _ := element.Get("value"); v != "" {
			if value, ok := parseInteger(v); ok {
				declare("counter-reset:list-item " + strconv.Itoa(value))
				declare("counter-increment:none")
			}
		}
	}
	return result
}

// pyStrNumber formats a number as an f-string of Python formats a float.
func pyStrNumber(f float64) string {
	return pyStr(f)
}

// declarationPrecedence is declaration_precedence in
// weasyprint/css/__init__.py. It returns the precedence of a declaration.
// The values have no meaning unless compared to each other. The origin is
// "author", "user" or "user agent". See
// https://www.w3.org/TR/CSS21/cascade.html#cascading-order.
func declarationPrecedence(origin string, importance bool) int {
	switch {
	case origin == "user agent":
		return 1
	case origin == "user" && !importance:
		return 2
	case origin == "author" && !importance:
		return 3
	case origin == "author":
		return 4
	}
	return 5
}

// resolveVar is resolve_var in weasyprint/css/__init__.py. It returns the
// token with resolved CSS variables, as a list of tokens, or nil for None
// when the token holds no var(). The second result is false for None.
func resolveVar(computed *Style, token tinycss2.Node, parentStyle *Style) ([]tinycss2.Node, bool) {
	if !checkVar(token) {
		return nil, false
	}

	block, isBlock := token.(*tinycss2.ParenthesesBlock)
	fn, isFunction := token.(*tinycss2.FunctionBlock)
	if isBlock || (isFunction && fn.LowerName != "var") {
		var items []tinycss2.Node
		var tokenItems []tinycss2.Node
		if isFunction {
			tokenItems = fn.Arguments
		} else {
			tokenItems = block.Content
		}
		for _, argument := range tokenItems {
			switch argument.Type() {
			case tinycss2.TypeFunction, tinycss2.TypeParenthesesBlock:
				resolved, ok := resolveVar(computed, argument, parentStyle)
				if ok {
					items = append(items, resolved...)
				} else {
					items = append(items, argument)
				}
			default:
				items = append(items, argument)
			}
			if len(items) > maxVarTokens {
				return nil, true
			}
		}
		line, column := token.Source()
		if isBlock {
			token = tinycss2.NewParenthesesBlock(line, column, items)
		} else {
			token = tinycss2.NewFunctionBlock(line, column, fn.Name, items)
		}
		if resolved, ok := resolveVar(computed, token, parentStyle); ok {
			return resolved, true
		}
		return []tinycss2.Node{token}, true
	}

	f := newFunction(token)
	arguments := f.splitCommaGroups(true)
	if len(arguments) == 0 || len(arguments[0]) != 1 {
		return []tinycss2.Node{}, true // No arguments or wrong variable name.
	}

	name, _ := tokenValue(arguments[0][0])
	variableName := strings.ReplaceAll(name, "-", "_") // First arg is name.
	if value, ok := computed.Get(variableName).(Tokens); ok && len(value) > 0 {
		return value, true // Computed value of variable is correct.
	}

	if len(arguments) < 2 {
		return []tinycss2.Node{}, true // No computed value and no default value.
	}

	var computedValue []tinycss2.Node
	for _, value := range arguments[1] {
		resolved, ok := resolveVar(computed, value, parentStyle)
		if ok {
			computedValue = append(computedValue, resolved...)
		} else {
			computedValue = append(computedValue, value)
		}
		if len(computedValue) > maxVarTokens {
			return nil, true
		}
	}
	return computedValue, true // Default value with resolved variables.
}

// calcGroup is a group of tokens of _resolve_calc_sum or
// _resolve_calc_product, or an operator between two groups.
type calcGroup struct {
	tokens   []tinycss2.Node
	operator string
}

// resolveCalcSum is _resolve_calc_sum in weasyprint/css/__init__.py.
func resolveCalcSum(computed *Style, tokens []tinycss2.Node, propertyName string, referTo *float64) (tinycss2.Node, error) {
	groups := []calcGroup{{}}
	for _, token := range tokens {
		switch t := token.(type) {
		case *tinycss2.LiteralToken:
			if t.Value == "+" || t.Value == "-" {
				groups = append(groups, calcGroup{operator: t.Value}, calcGroup{})
				continue
			}
		case *tinycss2.ParenthesesBlock:
			content := removeWhitespace(t.Content)
			result, err := resolveCalcSum(computed, content, propertyName, referTo)
			if err != nil || result == nil {
				return nil, err
			}
			groups[len(groups)-1].tokens = append(groups[len(groups)-1].tokens, result)
			continue
		}
		groups[len(groups)-1].tokens = append(groups[len(groups)-1].tokens, token)
	}

	value, sign, unit := 0.0, "+", ""
	var exception error
	for _, group := range groups {
		if sign == "" {
			sign = group.operator
			continue
		}
		if len(group.tokens) == 0 {
			// Port: upstream asserts that a group is not empty.
			return nil, nil
		}
		product, err := resolveCalcProduct(computed, group.tokens, propertyName, referTo)
		if errors.Is(err, errRelativeLengthInMath) {
			// RelativeLengthInMath raised, assume that we got pixels and
			// continue to find if we have to raise PercentageInMath first.
			if unit == "%" {
				return nil, errPercentageInMath
			}
			exception = err
			unit = "px"
			sign = ""
			continue
		}
		if err != nil {
			return nil, err
		}
		if product == nil {
			return nil, nil
		}
		switch p := product.(type) {
		case *tinycss2.DimensionToken:
			switch unit {
			case "":
				unit = p.LowerUnit
			case "%":
				return nil, errPercentageInMath
			default:
				if unit != p.LowerUnit {
					return nil, nil
				}
			}
		case *tinycss2.PercentageToken:
			switch {
			case referTo != nil:
				p.Value = p.Value / 100 * *referTo
				unit = "px"
			case unit == "" || unit == "%":
				unit = "%"
			default:
				return nil, errPercentageInMath
			}
		}
		productValue, _ := tokenNumber(product)
		if sign == "+" {
			value += productValue
		} else {
			value -= productValue
		}
		sign = ""
	}

	// Raise RelativeLengthInMath, only if we did not raise PercentageInMath
	// before.
	if exception != nil {
		return nil, exception
	}

	return tokenize(value, nil, unit), nil
}

// resolveCalcProduct is _resolve_calc_product in
// weasyprint/css/__init__.py.
func resolveCalcProduct(computed *Style, tokens []tinycss2.Node, propertyName string, referTo *float64) (tinycss2.Node, error) {
	_ = referTo
	groups := []calcGroup{{}}
	for _, token := range tokens {
		last := &groups[len(groups)-1]
		switch t := token.(type) {
		case *tinycss2.LiteralToken:
			if t.Value == "*" || t.Value == "/" {
				groups = append(groups, calcGroup{operator: t.Value}, calcGroup{})
				continue
			}
			return nil, nil
		case *tinycss2.NumberToken:
			last.tokens = append(last.tokens, token)
		case *tinycss2.DimensionToken:
			switch {
			case lengthUnits[t.LowerUnit]:
				if computed == nil && relativeUnits[t.LowerUnit] {
					return nil, errRelativeLengthInMath
				}
				pixels := toPixels(Dimension{t.Value, t.LowerUnit}, computed, propertyName, -1)
				last.tokens = append(last.tokens, tokenize(pixels, nil, "px"))
			case angleUnits[t.LowerUnit]:
				last.tokens = append(last.tokens, tokenize(toRadians(Dimension{t.Value, t.LowerUnit}), nil, "rad"))
			default:
				return nil, nil
			}
		case *tinycss2.PercentageToken:
			last.tokens = append(last.tokens, tokenize(t.Value, nil, "%"))
		case *tinycss2.IdentToken:
			last.tokens = append(last.tokens, token)
		default:
			return nil, nil
		}
	}

	value, sign, unit := 1.0, "*", ""
	for _, group := range groups {
		if sign == "" {
			sign = group.operator
			continue
		}
		calc := resolveCalcValue(group.tokens)
		if calc == nil {
			return nil, nil
		}
		switch c := calc.(type) {
		case *tinycss2.DimensionToken:
			if unit == "" || unit == "%" {
				unit = c.LowerUnit
			} else {
				return nil, nil
			}
		case *tinycss2.PercentageToken:
			if unit == "" {
				unit = "%"
			}
		}
		calcValue, _ := tokenNumber(calc)
		switch {
		case sign == "*":
			value *= calcValue
		case calcValue == 0:
			// Division by zero resolves to ±infinity or NaN (IEEE-754), as
			// required by CSS Values 4, instead of raising. This matches the
			// value of an equivalent calc(infinity * ...) expression.
			if value == 0 {
				value = math.NaN()
			} else {
				value = math.Copysign(math.Inf(1), value)
			}
		default:
			value /= calcValue
		}
		sign = ""
	}

	return tokenize(value, nil, unit), nil
}

// resolveCalcValue is _resolve_calc_value in weasyprint/css/__init__.py.
func resolveCalcValue(tokens []tinycss2.Node) tinycss2.Node {
	if len(tokens) != 1 {
		return nil
	}
	switch t := tokens[0].(type) {
	case *tinycss2.NumberToken, *tinycss2.DimensionToken, *tinycss2.PercentageToken:
		return t
	case *tinycss2.IdentToken:
		switch t.LowerValue {
		case "e":
			return tokenE()
		case "pi":
			return tokenPi()
		case "infinity":
			return tokenPlusInfinity()
		case "-infinity":
			return tokenMinusInfinity()
		case "nan":
			return tokenNaN()
		}
	}
	return nil
}

// ResolveMath is resolve_math in weasyprint/css/__init__.py. It returns the
// token with resolved math functions, or nil for None.
//
// It returns, in order of priority, errPercentageInMath if percentages are
// mixed with other values with no referTo size, or errRelativeLengthInMath
// if no computed style is available to get the font or the viewport size.
//
// errPercentageInMath has to be returned before errRelativeLengthInMath so
// that it can be used to discard the validation of the properties that do
// not accept percentages. A nil referTo is the None of upstream.
func ResolveMath(token tinycss2.Node, computed *Style, propertyName string, referTo *float64) (tinycss2.Node, error) {
	return resolveMath(token, computed, propertyName, referTo)
}

// resolveMath is resolve_math in weasyprint/css/__init__.py.
func resolveMath(token tinycss2.Node, computed *Style, propertyName string, referTo *float64) (tinycss2.Node, error) {
	if !checkMathToken(token) {
		return nil, nil
	}

	var args [][]tinycss2.Node
	f := newFunction(token)
	for _, part := range f.splitCommaGroups(false) {
		args = append(args, []tinycss2.Node{})
		for _, arg := range part {
			if checkMathToken(arg) {
				resolved, err := resolveMath(arg, computed, propertyName, referTo)
				if err != nil || resolved == nil {
					return nil, err
				}
				arg = resolved
			}
			args[len(args)-1] = append(args[len(args)-1], arg)
		}
	}
	sum := func(tokens []tinycss2.Node) (tinycss2.Node, error) {
		return resolveCalcSum(computed, tokens, propertyName, referTo)
	}

	switch f.name {
	case "calc":
		result, err := sum(args[0])
		if err != nil || result == nil {
			return nil, err
		}
		return tokenize(result, nil, ""), nil

	case "min", "max":
		var targetValue, targetToken tinycss2.Node
		unit, hasUnit := "", false
		for _, tokens := range args {
			token, err := sum(tokens)
			if err != nil || token == nil {
				return nil, err
			}
			var value tinycss2.Node
			switch t := token.(type) {
			case *tinycss2.PercentageToken:
				if referTo == nil {
					if hasUnit && (unit == "px" || unit == "") {
						return nil, errPercentageInMath
					}
					unit, hasUnit = "%", true
					value = token
				} else {
					unit, hasUnit = "px", true
					token = tokenize(t.Value/100**referTo, nil, "px")
					value = token
				}
			case *tinycss2.NumberToken:
				if hasUnit && unit == "%" {
					return nil, errPercentageInMath
				} else if hasUnit && unit == "px" {
					return nil, nil
				}
				unit, hasUnit = "", true
				value = tokenize(t.Value, nil, "px")
			default:
				if hasUnit && unit == "%" {
					return nil, errPercentageInMath
				} else if hasUnit && unit == "" {
					return nil, nil
				}
				unit, hasUnit = "px", true
				value = tokenize(toPixels(tokenDimension(token), computed, propertyName, -1), nil, "px")
			}
			v, _ := tokenNumber(value)
			tv, _ := tokenNumber(targetValue)
			if targetValue == nil || (f.name == "min" && v < tv) || (f.name == "max" && v > tv) {
				targetValue, targetToken = value, token
			}
		}
		return tokenize(targetToken, nil, ""), nil

	case "round":
		strategy, multiple := "nearest", 1.0
		var numberToken tinycss2.Node
		var err error
		switch len(args) {
		case 1:
			numberToken, err = sum(args[0])
		case 2:
			if v, _ := tokenValue(args[0][0]); len(args[0]) == 1 && in(v, "nearest", "up", "down", "to-zero") {
				strategy = v
				numberToken, err = sum(args[1])
			} else {
				numberToken, err = sum(args[0])
				if err != nil {
					return nil, err
				}
				var multipleToken tinycss2.Node
				multipleToken, err = sum(args[1])
				if err != nil || numberToken == nil || multipleToken == nil {
					return nil, err
				}
				if numberToken.Type() != multipleToken.Type() {
					return nil, nil
				}
				multiple, _ = tokenNumber(multipleToken)
			}
		case 3:
			strategy, _ = tokenValue(args[0][0])
			numberToken, err = sum(args[1])
			if err != nil {
				return nil, err
			}
			var multipleToken tinycss2.Node
			multipleToken, err = sum(args[2])
			if err != nil || numberToken == nil || multipleToken == nil {
				return nil, err
			}
			if numberToken.Type() != multipleToken.Type() {
				return nil, nil
			}
			multiple, _ = tokenNumber(multipleToken)
		}
		if err != nil || numberToken == nil {
			return nil, err
		}
		var function func(float64) float64
		switch strategy {
		case "nearest":
			function = func(n float64) float64 { return math.Floor(n + 0.5) }
		case "up":
			function = math.Ceil
		case "down":
			function = math.Floor
		case "to-zero":
			if v, _ := tokenNumber(numberToken); v > 0 {
				function = math.Floor
			} else {
				function = math.Ceil
			}
		default:
			return nil, nil
		}
		return tokenize(numberToken, func(x float64) float64 { return function(x/multiple) * multiple }, ""), nil

	case "mod", "rem":
		numberToken, err := sum(args[0])
		if err != nil {
			return nil, err
		}
		parameterToken, err := sum(args[1])
		if err != nil || numberToken == nil || parameterToken == nil {
			return nil, err
		}
		if numberToken.Type() != parameterToken.Type() {
			return nil, nil
		}
		number, _ := tokenNumber(numberToken)
		parameter, _ := tokenNumber(parameterToken)
		value := pyFloatMod(number, parameter)
		if f.name == "rem" && number*parameter < 0 {
			value += math.Abs(parameter)
		}
		return tokenize(numberToken, func(float64) float64 { return value }, ""), nil

	case "sin", "cos", "tan":
		numberToken, err := sum(args[0])
		if err != nil || numberToken == nil {
			return nil, err
		}
		var angle float64
		if n, ok := numberToken.(*tinycss2.NumberToken); ok {
			angle = n.Value
		} else if a, ok := getAngle(numberToken); ok {
			angle = a
		} else {
			return nil, nil
		}
		var value float64
		switch f.name {
		case "sin":
			value = math.Sin(angle)
		case "cos":
			value = math.Cos(angle)
		default:
			value = math.Tan(angle)
		}
		return tokenize(value, nil, ""), nil

	case "asin", "acos", "atan":
		numberToken, err := sum(args[0])
		if err != nil || numberToken == nil {
			return nil, err
		}
		n, ok := numberToken.(*tinycss2.NumberToken)
		if !ok {
			return nil, nil
		}
		var value float64
		switch f.name {
		case "asin":
			value = math.Asin(n.Value)
		case "acos":
			value = math.Acos(n.Value)
		default:
			value = math.Atan(n.Value)
		}
		if math.IsNaN(value) && !math.IsNaN(n.Value) {
			// Python raises ValueError out of the domain.
			return nil, nil
		}
		return tokenize(value, nil, "rad"), nil

	case "atan2":
		yToken, err := sum(args[0])
		if err != nil {
			return nil, err
		}
		xToken, err := sum(args[1])
		if err != nil || yToken == nil || xToken == nil {
			return nil, err
		}
		y, yok := yToken.(*tinycss2.NumberToken)
		x, xok := xToken.(*tinycss2.NumberToken)
		if !yok || !xok {
			return nil, nil
		}
		return tokenize(math.Atan2(y.Value, x.Value), nil, "rad"), nil

	case "clamp":
		var pixelsList []tinycss2.Node
		unit := ""
		for _, tokens := range args {
			token, err := sum(tokens)
			if err != nil || token == nil {
				return nil, err
			}
			var value tinycss2.Node
			if p, ok := token.(*tinycss2.PercentageToken); ok {
				if referTo == nil {
					if unit == "px" {
						return nil, errPercentageInMath
					}
					unit = "%"
					value = token
				} else {
					// Upstream changes token here, and appends the value of
					// the previous iteration.
					unit = "px"
					_ = p
					if value == nil && len(pixelsList) > 0 {
						value = pixelsList[len(pixelsList)-1]
					}
				}
			} else {
				if unit == "%" {
					return nil, errPercentageInMath
				}
				unit = "px"
				pixels := toPixels(tokenDimension(token), computed, propertyName, -1)
				value = tokenize(pixels, nil, "px")
			}
			if value == nil {
				// Port: upstream raises NameError for an unbound value.
				return nil, nil
			}
			pixelsList = append(pixelsList, value)
		}
		minToken, token, maxToken := pixelsList[0], pixelsList[1], pixelsList[2]
		minValue, _ := tokenNumber(minToken)
		value, _ := tokenNumber(token)
		maxValue, _ := tokenNumber(maxToken)
		if value < minValue {
			token, value = minToken, minValue
		}
		if value > maxValue {
			token = maxToken
		}
		return tokenize(token, nil, ""), nil

	case "pow":
		numberToken, err := sum(args[0])
		if err != nil {
			return nil, err
		}
		powerToken, err := sum(args[1])
		if err != nil || numberToken == nil || powerToken == nil {
			return nil, err
		}
		_, nok := numberToken.(*tinycss2.NumberToken)
		power, pok := powerToken.(*tinycss2.NumberToken)
		if !nok || !pok {
			return nil, nil
		}
		return tokenize(numberToken, func(x float64) float64 { return math.Pow(x, power.Value) }, ""), nil

	case "sqrt":
		numberToken, err := sum(args[0])
		if err != nil || numberToken == nil {
			return nil, err
		}
		if _, ok := numberToken.(*tinycss2.NumberToken); !ok {
			return nil, nil
		}
		return tokenize(numberToken, func(x float64) float64 { return math.Pow(x, 0.5) }, ""), nil

	case "hypot":
		resolved := make([]tinycss2.Node, 0, len(args))
		for _, tokens := range args {
			token, err := sum(tokens)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, token)
		}
		values := make([]float64, len(resolved))
		for i, token := range resolved {
			if token == nil {
				return nil, nil
			}
			values[i], _ = tokenNumber(token)
		}
		value := 0.0
		for _, v := range values {
			value = math.Hypot(value, v)
		}
		return tokenize(resolved[0], func(float64) float64 { return value }, ""), nil

	case "log":
		numberToken, err := sum(args[0])
		if err != nil || numberToken == nil {
			return nil, err
		}
		if _, ok := numberToken.(*tinycss2.NumberToken); !ok {
			return nil, nil
		}
		base := math.E
		if len(args) == 2 {
			baseToken, err := sum(args[1])
			if err != nil || baseToken == nil {
				return nil, err
			}
			b, ok := baseToken.(*tinycss2.NumberToken)
			if !ok {
				return nil, nil
			}
			base = b.Value
		}
		return tokenize(numberToken, func(x float64) float64 { return math.Log(x) / math.Log(base) }, ""), nil

	case "exp":
		numberToken, err := sum(args[0])
		if err != nil || numberToken == nil {
			return nil, err
		}
		if _, ok := numberToken.(*tinycss2.NumberToken); !ok {
			return nil, nil
		}
		return tokenize(numberToken, math.Exp, ""), nil

	case "abs":
		numberToken, err := sum(args[0])
		if err != nil || numberToken == nil {
			return nil, err
		}
		return tokenize(numberToken, math.Abs, ""), nil

	case "sign":
		numberToken, err := sum(args[0])
		if err != nil || numberToken == nil {
			return nil, err
		}
		v, _ := tokenNumber(numberToken)
		sign := 0.0
		if v > 0 {
			sign = 1
		} else if v < 0 {
			sign = -1
		}
		return tokenize(sign, nil, ""), nil
	}

	fn, ok := token.(*tinycss2.FunctionBlock)
	if !ok {
		return nil, nil
	}
	arguments := make([]tinycss2.Node, 0, len(fn.Arguments))
	changed := false
	for _, argument := range fn.Arguments {
		if argument.Type() == tinycss2.TypeFunction {
			result, err := resolveMath(argument, computed, propertyName, referTo)
			if err != nil || result == nil {
				return nil, err
			}
			arguments = append(arguments, result)
			changed = changed || result != argument
		} else {
			arguments = append(arguments, argument)
		}
	}
	line, column := token.Source()
	token = tinycss2.NewFunctionBlock(line, column, fn.Name, arguments)
	if !changed {
		// Port: upstream calls itself again on the same function, which
		// recurses without end.
		return token, nil
	}
	resolved, err := resolveMath(token, computed, propertyName, referTo)
	if err != nil {
		return nil, err
	}
	if resolved != nil {
		return resolved, nil
	}
	return token, nil
}

// tokenDimension returns a dimension token as a Dimension. Upstream passes
// the token to to_pixels.
func tokenDimension(token tinycss2.Node) Dimension {
	v, _ := tokenNumber(token)
	return Dimension{v, tokenUnit(token)}
}

// pyFloatMod is the operator % of Python for floats: the sign of the result
// is the sign of the divisor.
func pyFloatMod(a, b float64) float64 {
	if b == 0 {
		return math.NaN()
	}
	m := math.Mod(a, b)
	if m != 0 && (m < 0) != (b < 0) {
		m += b
	}
	return m
}

// styleKind tells the classes of the styles of upstream apart.
type styleKind uint8

// The kinds of styles.
const (
	// kindInitial is InitialStyle.
	kindInitial styleKind = iota
	// kindAnonymous is AnonymousStyle.
	kindAnonymous
	// kindComputed is ComputedStyle.
	kindComputed
)

// Style is a style dictionary of weasyprint/css/__init__.py: Style and its
// subclasses InitialStyle, AnonymousStyle and ComputedStyle. It maps the
// name of each property, in snake case such as "margin_top", to its
// computed value. The name of a custom property starts with "__".
//
// The values are computed when they are read, as the method __missing__ of
// upstream does: Get is style[key], Set is style[key] = value, Has is key in
// style, and Delete is del style[key].
//
// A later stage reads a value with Get and a type switch or a helper, such
// as:
//
//	if d, ok := style.Get("margin_top").(Dimension); ok { ... }
//	fontSize := num(style.Get("font_size"))
//	if isStr(style.Get("float"), "none") { ... }
type Style struct {
	// ParentStyle is the style of the parent, or nil.
	ParentStyle *Style
	// RootStyle is the style of the root element, or the initial style for
	// the root element itself.
	RootStyle *Style
	// PseudoType is the pseudo-element type of a computed style, or "".
	PseudoType string
	// BaseURL is the base URL of a computed style.
	BaseURL string
	// IsRootElement is true for the style of the root element.
	IsRootElement bool

	// The fields of one byte follow IsRootElement, so that the style is
	// smaller.
	kind styleKind
	// initialSpecified holds a bit for position and float when their
	// specified value is their initial value, which missingComputed does
	// not store in specified.
	initialSpecified uint8
	// copied is true for a copy. The layout sets the values of a copy,
	// such as the values of the first line, after its copy.
	copied bool
	// fontKeysSet holds a bit for each key of fontKeys that the style
	// keeps.
	fontKeysSet uint8
	// flow caches the flags of flowFlags. Set and Delete clear it.
	flow uint8

	// cascadedSet holds the known properties of cascaded.
	cascadedSet propertySet
	// inheritFrom is the style that gives the value of each fromParent
	// property and each custom property of this computed style, or nil.
	// The style does not store these values. A computed style has one when
	// its cascaded style sets none of these properties and its parent is
	// not a copy. inheritFrom is a computed style and not a copy, as the
	// layout changes no inherited value of these styles.
	//
	// Port: upstream stores the value of the parent in each style that
	// reads it. A style whose parent stores the value gives the value of
	// its parent, as upstream does, and inheritFrom finds that parent in
	// one step.
	inheritFrom *Style

	// stored holds the ids of the known properties whose computed values
	// the style holds, and vals holds these values in the order of the
	// ids. other holds the values of the custom properties, and of the
	// names that are not properties, which Set can store.
	//
	// Port: a style of upstream is a dict. The port keeps the values of the
	// known properties in a slice, without a map and without the hash of
	// the name for each value that the style holds. See propertyInfo.
	stored propertySet
	vals   []Value
	other  map[string]Value

	solving   map[string]bool
	specified map[string]Value
	cascaded  map[string]cascadedValue
	ctx       *StyleContext
	anonymous *Style

	// fontKeys caches the keys of fontStyleCacheKey, without and with the
	// size, when the bit of fontKeysSet is set. Set and Delete clear the
	// keys that the property changes. See fontKeyBits.
	//
	// Port: upstream makes the key for each line. A computed value does
	// not change once computed, except through Set and Delete.
	fontKeys [2]string
	// fonts is the fontmap of the style for the shaping, or nil. Set and
	// Delete clear it.
	fonts *styleFonts
}

// Get is style[key] of upstream. It returns the computed value of a
// property, and computes it first when needed. It returns nil for an
// unknown property.
func (s *Style) Get(key string) Value {
	return s.get(key, propertyInfos[key])
}

// knownProperty is a known property that a hot path of the layout reads
// with getKnown, without the hash of its name. Make one with
// newKnownProperty in a variable of the package.
type knownProperty struct {
	name string
	info func() *propertyInfo
}

// newKnownProperty returns the knownProperty of a name. It finds the
// propertyInfo at its first use, after the init functions of the package.
func newKnownProperty(name string) knownProperty {
	return knownProperty{name, sync.OnceValue(func() *propertyInfo { return propertyInfos[name] })}
}

// getKnown is Get for a knownProperty.
func (s *Style) getKnown(p knownProperty) Value {
	return s.get(p.name, p.info())
}

// get is Get for a key and its propertyInfo, which is nil for a custom
// property and for a name that is not a property.
func (s *Style) get(key string, info *propertyInfo) Value {
	if v, ok := s.lookup(key, info); ok {
		return v
	}
	switch s.kind {
	case kindInitial:
		return s.missingInitial(key)
	case kindAnonymous:
		return s.missingAnonymous(key, info)
	}
	return s.missingComputed(key, info)
}

// lookup returns the value that the style holds for a key, and whether it
// holds one.
func (s *Style) lookup(key string, info *propertyInfo) (Value, bool) {
	if info == nil {
		v, ok := s.other[key]
		return v, ok
	}
	if !s.stored.has(info.id) {
		return nil, false
	}
	return s.vals[s.stored.rank(info.id)], true
}

// Set is style[key] = value of upstream.
func (s *Style) Set(key string, value Value) {
	s.store(key, value)
	s.fontKeysSet &^= fontKeyBits(key)
	s.flow = 0
	s.fonts = nil
}

// store keeps the computed value of a property.
func (s *Style) store(key string, value Value) {
	s.storeInfo(key, propertyInfos[key], value)
}

// storeInfo is store for a key and its propertyInfo.
func (s *Style) storeInfo(key string, info *propertyInfo, value Value) {
	if info == nil {
		if s.other == nil {
			s.other = make(map[string]Value)
		}
		s.other[key] = value
		return
	}
	i := s.stored.rank(info.id)
	if s.stored.has(info.id) {
		s.vals[i] = value
		return
	}
	s.stored.add(info.id)
	if s.vals == nil {
		// Most styles that hold a value hold a few.
		s.vals = make([]Value, 0, 4)
	}
	s.vals = slices.Insert(s.vals, i, value)
}

// remove removes the value that the style holds for a key, if any.
func (s *Style) remove(key string, info *propertyInfo) {
	if info == nil {
		delete(s.other, key)
		return
	}
	if !s.stored.has(info.id) {
		return
	}
	i := s.stored.rank(info.id)
	s.vals = slices.Delete(s.vals, i, i+1)
	s.stored.clear(info.id)
}

// storedKeys returns the names of the properties whose values the style
// holds, in no order.
func (s *Style) storedKeys() []string {
	keys := make([]string, 0, len(s.vals)+len(s.other))
	for id := range s.stored.all() {
		keys = append(keys, propertyFacts[id].name)
	}
	for key := range s.other {
		keys = append(keys, key)
	}
	return keys
}

// keepOnly makes the style hold the values of keys, as style computes
// them, and no other value. styleDict uses it.
func (s *Style) keepOnly(style *Style, keys map[string]cascadedValue) {
	s.stored, s.vals, s.other = propertySet{}, nil, nil
	for key := range keys {
		s.store(key, style.Get(key))
	}
}

// merge stores the values that other holds in the style.
func (s *Style) merge(other *Style) {
	i := 0
	for id := range other.stored.all() {
		s.store(propertyFacts[id].name, other.vals[i])
		i++
	}
	for key, value := range other.other {
		s.store(key, value)
	}
}

// Has is key in style of upstream: it reports whether the value of a
// property is already computed.
//
// Port: Has is false for a value that the style gives without storing it.
// See propertyInfo.
func (s *Style) Has(key string) bool {
	_, ok := s.lookup(key, propertyInfos[key])
	return ok
}

// Delete is del style[key] of upstream.
func (s *Style) Delete(key string) {
	s.remove(key, propertyInfos[key])
	s.fontKeysSet &^= fontKeyBits(key)
	s.flow = 0
	s.fonts = nil
}

// Context returns what the styles of the document share.
func (s *Style) Context() *StyleContext {
	return s.ctx
}

// Specified returns the specified value of position, float or display, as
// style.specified[key] of upstream.
func (s *Style) Specified(key string) Value {
	if s.kind != kindComputed {
		// The specified values of InitialStyle and AnonymousStyle are the
		// style itself.
		return s.Get(key)
	}
	if v, ok := s.specified[key]; ok {
		return v
	}
	if info := propertyInfos[key]; info != nil && s.initialSpecified&info.specified != 0 {
		// Port: missingComputed gave the initial value. See propertyInfo.
		return info.initial
	}
	return nil
}

// AnonymousStyle is the cached property anonymous_style of Style in
// weasyprint/css/__init__.py: the style of the anonymous boxes of this
// style.
func (s *Style) AnonymousStyle() *Style {
	if s.anonymous == nil {
		s.anonymous = newAnonymousStyle(s)
	}
	return s.anonymous
}

// Copy is the method copy of AnonymousStyle and ComputedStyle in
// weasyprint/css/__init__.py.
func (s *Style) Copy() *Style {
	c := *s
	c.vals = slices.Clone(s.vals)
	c.other = maps.Clone(s.other)
	c.copied = true
	c.anonymous = nil
	c.solving = nil
	c.fonts = nil
	c.flow = 0
	if s.kind == kindComputed {
		c.specified = maps.Clone(s.specified)
	}
	return &c
}

// newInitialStyle is InitialStyle.__init__ in weasyprint/css/__init__.py, a
// dummy computed style used to store initial values.
func newInitialStyle(ctx *StyleContext) *Style {
	return &Style{kind: kindInitial, ctx: ctx}
}

// missingInitial is InitialStyle.__missing__.
func (s *Style) missingInitial(key string) Value {
	value, ok := InitialValues[key]
	if !ok {
		// Port: upstream raises KeyError.
		return nil
	}
	s.store(key, value)
	return value
}

// newAnonymousStyle is AnonymousStyle.__init__ in
// weasyprint/css/__init__.py, the computed style used for anonymous boxes.
func newAnonymousStyle(parentStyle *Style) *Style {
	return &Style{
		kind: kindAnonymous,
		// border-*-style is none, so border-width computes to zero. Other
		// than that, properties that would need computing are
		// border-*-color, but they do not apply.
		//
		// Port: upstream stores the zero widths here. missingAnonymous
		// gives them.
		ParentStyle: parentStyle,
		RootStyle:   parentStyle.RootStyle,
		ctx:         parentStyle.ctx,
	}
}

// missingAnonymous is AnonymousStyle.__missing__. info is the
// propertyInfo of key, or nil.
func (s *Style) missingAnonymous(key string, info *propertyInfo) Value {
	// Port: see propertyInfo. The border widths and the width of the
	// outline are 0, as newAnonymousStyle says. An anonymous style stores
	// the values of its parent, as the layout reads the style of a text
	// many times.
	if info == nil {
		if !strings.HasPrefix(key, "__") {
			return nil
		}
		value := s.ParentStyle.get(key, nil)
		s.storeInfo(key, nil, value)
		return value
	}
	switch {
	case info.constant || info.specified != 0:
		return info.initial
	case info.zeroWidth:
		return Int(0)
	}
	fact := &propertyFacts[info.id]
	var value Value
	switch {
	case fact.inherited:
		value = s.ParentStyle.get(key, info)
	case key == "page":
		// page is not inherited but taken from the ancestor if 'auto'.
		value = s.ParentStyle.get(key, info)
	case fact.textDecoration:
		value = textDecoration(key, fact.initial, s.ParentStyle.get(key, info), false)
	default:
		value = fact.initial
		if fact.computer != nil && fact.initialNotComputed {
			// Value not computed yet: compute.
			value = fact.computer(s, key, value)
		}
		// The value is the same as when computed.
	}
	s.storeInfo(key, info, value)
	return value
}

// propertyFact is what does not change of a known property, for the
// styles that compute its value.
//
// Port: upstream reads these facts in its dicts, by the name of the
// property. The port reads them by the number of the property, without a
// hash of the name.
type propertyFact struct {
	// name is the name of the property, such as "margin_top".
	name string
	// inherited is Inherited[name].
	inherited bool
	// textDecoration is true for a property of the text decorations.
	textDecoration bool
	// initialNotComputed is initialNotComputed[name].
	initialNotComputed bool
	// initial is InitialValues[name].
	initial Value
	// computer is computerFunctions[name], or nil.
	computer computerFunc
	// physical is physicalFunctions[name], or nil.
	physical func(block, inline string) string
}

// propertyFacts holds the propertyFact of each known property, by its
// propertyInfo.id.
var propertyFacts []propertyFact

// newComputedStyle is ComputedStyle.__init__ in weasyprint/css/__init__.py,
// the computed style used for the boxes that are not anonymous.
func newComputedStyle(parentStyle *Style, cascaded map[string]cascadedValue, pseudoType string, rootStyle *Style, baseURL string, ctx *StyleContext) *Style {
	s := &Style{
		kind:          kindComputed,
		ParentStyle:   parentStyle,
		cascaded:      cascaded,
		IsRootElement: parentStyle == nil,
		PseudoType:    pseudoType,
		RootStyle:     rootStyle,
		BaseURL:       baseURL,
		ctx:           ctx,
	}
	inheritsAll := true
	for key := range cascaded {
		info := propertyInfos[key]
		if info != nil {
			s.cascadedSet.add(info.id)
		}
		if info != nil && info.fromParent || strings.HasPrefix(key, "__") {
			inheritsAll = false
		}
	}
	if parentStyle != nil && inheritsAll {
		s.inheritFrom = parentStyle.inheritedFrom()
	}
	return s
}

// propertyInfo is what missingComputed and missingAnonymous need to know
// of a known property before they compute its value.
//
// Port: upstream computes each value that a style reads and stores it in
// the style. Most values of the styles of a book are initial values, values
// of the parent, or zero widths. The port gives these values without
// storing them, so most styles hold few values, and most styles of the
// pseudo-elements hold none.
type propertyInfo struct {
	// id is the number of the property in a propertySet.
	id int
	// constant is true for a property whose computed value is its initial
	// value when the cascade does not set it. Such a property is not
	// inherited, its initial value is the same when computed, and
	// missingComputed has no special case for it.
	constant bool
	// specified is the bit of initialSpecified for position and float,
	// whose computed value is also their initial value when the cascade
	// does not set them, or 0.
	specified uint8
	// fromParent is true for a property whose computed value is the value
	// of the parent when the cascade does not set it: an inherited
	// property, page, or a property of the text decorations.
	fromParent bool
	// zeroWidth is true for a width of a border, of the outline or of the
	// rule of the columns, which computes to 0 when the cascade sets
	// neither the width nor its style.
	zeroWidth bool
	// initial is the initial value of a constant property, of position and
	// of float.
	initial Value
	// cascades holds the properties whose cascade changes the value of a
	// constant, specified or zeroWidth property: the property, its logical
	// properties, and the style of the line of a zeroWidth property and its
	// logical properties.
	cascades propertySet
}

// The bits of initialSpecified.
const (
	specifiedPosition uint8 = 1 << iota
	specifiedFloat
)

// propertySet is a set of known properties, by their propertyInfo.id.
type propertySet [4]uint64

// add adds the property id to the set.
func (p *propertySet) add(id int) {
	p[id/64] |= 1 << (id % 64)
}

// clear removes the property id from the set.
func (p *propertySet) clear(id int) {
	p[id/64] &^= 1 << (id % 64)
}

// has reports whether the property id is in the set.
func (p *propertySet) has(id int) bool {
	return p[id/64]&(1<<(id%64)) != 0
}

// all returns the ids of the set, in increasing order.
func (p *propertySet) all() iter.Seq[int] {
	return func(yield func(int) bool) {
		for w, word := range p {
			for word != 0 {
				if !yield(w*64 + bits.TrailingZeros64(word)) {
					return
				}
				word &= word - 1
			}
		}
	}
}

// rank returns the number of the properties of the set whose id is less
// than id.
func (p *propertySet) rank(id int) int {
	w := id / 64
	n := bits.OnesCount64(p[w] & (1<<(id%64) - 1))
	for i := range w {
		n += bits.OnesCount64(p[i])
	}
	return n
}

// intersects reports whether the two sets have a property in common.
func (p *propertySet) intersects(o *propertySet) bool {
	return p[0]&o[0] != 0 || p[1]&o[1] != 0 || p[2]&o[2] != 0 || p[3]&o[3] != 0
}

// propertyInfos holds the propertyInfo of each known property, by name.
var propertyInfos map[string]*propertyInfo

// newPropertyInfos returns the table of propertyInfos.
func newPropertyInfos() map[string]*propertyInfo {
	keys := slices.Sorted(maps.Keys(InitialValues))
	if len(keys) > len(propertySet{})*64 {
		// The styles need a bit for each property.
		panic("layout: too many properties for a propertySet")
	}
	m := make(map[string]*propertyInfo, len(keys))
	propertyFacts = make([]propertyFact, len(keys))
	for id, key := range keys {
		textDecoration := strings.HasPrefix(key, "text_decoration_")
		m[key] = &propertyInfo{
			id:         id,
			fromParent: Inherited[key] || key == "page" || textDecoration,
		}
		propertyFacts[id] = propertyFact{
			name:               key,
			inherited:          Inherited[key],
			textDecoration:     textDecoration,
			initialNotComputed: initialNotComputed[key],
			initial:            InitialValues[key],
			computer:           computerFunctions[key],
			physical:           physicalFunctions[key],
		}
	}
	// cascade adds a property and its logical properties to set.
	cascade := func(set *propertySet, key string) {
		set.add(m[key].id)
		if f, ok := physicalFunctions[key]; ok {
			// TODO: use writing-mode and text-orientation, as
			// missingComputed does.
			for _, direction := range [...]string{"ltr", "rtl"} {
				if l := m[f("ttb", direction)]; l != nil {
					set.add(l.id)
				}
			}
		}
	}
	for _, key := range keys {
		info, value := m[key], InitialValues[key]
		switch {
		case checkMath(value):
		case key == "position":
			info.specified, info.initial = specifiedPosition, value
		case key == "float":
			info.specified, info.initial = specifiedFloat, value
		case key == "border_top_width" || key == "border_right_width" ||
			key == "border_bottom_width" || key == "border_left_width" ||
			key == "outline_width" || key == "column_rule_width":
			// computeBorderWidth gives 0 for the style none, which is the
			// initial value of each style of a line.
			info.zeroWidth = true
			cascade(&info.cascades, strings.Replace(key, "width", "style", 1))
		case !info.fromParent && !initialNotComputed[key] && key != "display":
			info.constant, info.initial = true, value
		}
		cascade(&info.cascades, key)
	}
	return m
}

// inheritedFrom returns the style that gives the fromParent values of the
// computed children of s that inherit all of them, or nil when they must
// store them. A computed style that is not a copy keeps these values, as
// the layout does not change them.
func (s *Style) inheritedFrom() *Style {
	switch {
	case s == nil || s.copied || s.kind != kindComputed:
		return nil
	case s.inheritFrom != nil:
		return s.inheritFrom
	}
	return s
}

// missingComputed is ComputedStyle.__missing__. info is the propertyInfo
// of key, or nil.
func (s *Style) missingComputed(key string, info *propertyInfo) Value {
	// Port: see propertyInfo.
	known := info != nil
	switch {
	case !known:
		if s.inheritFrom != nil && strings.HasPrefix(key, "__") {
			return s.inheritFrom.get(key, nil)
		}
	case s.inheritFrom != nil && info.fromParent:
		return s.inheritFrom.get(key, info)
	case s.cascadedSet.intersects(&info.cascades):
	case info.constant:
		return info.initial
	case info.zeroWidth:
		return Int(0)
	case info.specified != 0:
		if key == "float" {
			// Set specified value for position, as below.
			s.Get("position")
		}
		// The layout reads them often, so they are stored.
		s.initialSpecified |= info.specified
		s.storeInfo(key, info, info.initial)
		return info.initial
	}
	switch key {
	case "float":
		// Set specified value for position, needed for computed value.
		s.Get("position")
	case "display":
		// Set specified value for float, needed for computed value.
		s.Get("float")
	}

	parentStyle := s.ParentStyle
	isCustom := strings.HasPrefix(key, "__")
	if !known && !isCustom {
		// Port: upstream raises KeyError for an unknown property.
		return nil
	}
	fact := &propertyFact{}
	if known {
		fact = &propertyFacts[info.id]
	}

	var value Value
	var w weight
	if c, ok := s.cascaded[key]; ok {
		// Property defined in cascaded properties.
		value, w = c.value, c.weight
	} else {
		// Property not defined in cascaded properties, define as inherited
		// or initial value.
		if fact.inherited || isCustom {
			value = String("inherit")
		} else {
			value = String("initial")
		}
	}
	_, pending := value.(*Pending)

	// Port: the name of the property in the CSS, with hyphens, is only
	// for the values of var(), so the port makes it there.
	wantedKey := key
	if logicalFunction := fact.physical; logicalFunction != nil {
		// TODO: use writing-mode and text-orientation.
		logicalKey := logicalFunction("ttb", str(s.Get("direction")))
		if c, ok := s.cascaded[logicalKey]; ok && c.weight.compare(w) >= 0 {
			wantedKey = logicalKey
			value = c.value
			_, pending = value.(*Pending)
		}
	}

	if isStr(value, "inherit") && parentStyle == nil {
		// On the root element, 'inherit' from initial values.
		value = String("initial")
	}

	if pending {
		p := value.(*Pending)
		var solvedTokens []tinycss2.Node
		tooLong := false
		for _, token := range p.Tokens {
			s.storeInfo(key, info, nil) // Mark property as invalid to avoid endless loops.
			tokens, ok := resolveVar(s, token, parentStyle)
			s.remove(key, info)
			switch {
			case !ok:
				solvedTokens = append(solvedTokens, token)
			case tokens == nil:
				tooLong = true
			default:
				solvedTokens = append(solvedTokens, tokens...)
			}
			if len(solvedTokens) > maxVarTokens {
				tooLong = true
			}
			if tooLong {
				break
			}
		}
		var err error
		if tooLong {
			// Port: a value that var() makes too long is not valid.
			err = invalidValues("too many tokens in the value of the variables")
			s.ctx.log.warning("Ignored `%s`, too many tokens in the value of the variables.", p.Name)
		} else {
			value, err = p.solve(s.ctx.log, solvedTokens, strings.ReplaceAll(wantedKey, "_", "-"), s.BaseURL)
		}
		if err != nil {
			switch {
			case fact.inherited && parentStyle != nil:
				// Values in parent_style are already computed.
				value = parentStyle.get(key, info)
				s.storeInfo(key, info, value)
			case isCustom:
				value = nil
			default:
				value = fact.initial
				if !fact.initialNotComputed {
					// The value is the same as when computed.
					s.storeInfo(key, info, value)
				}
			}
		}
	}

	if isStr(value, "initial") {
		if isCustom {
			value = Tokens{}
		} else {
			value = fact.initial
		}
		if !fact.initialNotComputed {
			// The value is the same as when computed.
			s.storeInfo(key, info, value)
		}
	} else if isStr(value, "inherit") {
		// Values in parent_style are already computed.
		value = parentStyle.get(key, info)
		s.storeInfo(key, info, value)
		// display depends on float, and float on position, call computer
		// again.
		if key == "display" || key == "float" {
			value = fact.computer(s, key, value)
			s.storeInfo(key, info, value)
		}
	}

	switch {
	case fact.textDecoration && parentStyle != nil:
		// Text decorations are not inherited but propagated. See
		// https://www.w3.org/TR/css-text-decor-3/#line-decoration.
		if fact.computer != nil {
			value = fact.computer(s, key, value)
		}
		_, cascaded := s.cascaded[key]
		s.storeInfo(key, info, textDecoration(key, value, parentStyle.get(key, info), cascaded))
	case key == "page" && isStr(value, "auto"):
		// The page property does not inherit. However, if the page value on
		// an element is auto, then its used value is the value specified on
		// its nearest ancestor with a non-auto value. When specified on the
		// root element, the used value for auto is the empty string. See
		// https://www.w3.org/TR/css-page-3/#using-named-pages.
		if parentStyle == nil {
			value = String("")
		} else {
			value = parentStyle.get(key, info)
		}
		s.remove(key, info)
	case key == "position" || key == "float" || key == "display":
		// Save specified values to define computed values for these specific
		// properties. See https://www.w3.org/TR/CSS21/visuren.html#dis-pos-flo.
		if s.specified == nil {
			s.specified = make(map[string]Value)
		}
		s.specified[key] = value
	}

	if checkMath(value) {
		value = s.solveMath(key, value)
	}

	if v, ok := s.lookup(key, info); ok {
		// Value already computed and saved: return.
		return v
	}

	if fact.computer != nil {
		// Value not computed yet: compute.
		value = fact.computer(s, key, value)
	}

	s.storeInfo(key, info, value)
	return value
}

// solveMath resolves the math functions of a value, for missingComputed.
func (s *Style) solveMath(key string, value Value) Value {
	var solvedTokens []tinycss2.Node
	values, ok := value.(Tuple)
	if !ok {
		values = Tuple{value}
	}
	var function tinycss2.Node
	failed := false
	if s.solving[key] {
		// Port: a math function of the property needs the property itself,
		// such as font-size: calc(1em + 1%). Upstream recurses until
		// RecursionError, catches it in the deepest call, and gives the
		// inherited or the initial value with one warning. The port stops
		// at the second call with the same result.
		for _, v := range values {
			if t, ok := v.(Token); ok && checkMath(v) {
				function = t.Node
				break
			}
		}
		values, failed = nil, true
	} else {
		if s.solving == nil {
			s.solving = map[string]bool{}
		}
		s.solving[key] = true
		defer delete(s.solving, key)
	}
	for _, v := range values {
		if _, isToken := v.(Token); checkMath(v) && !isToken {
			// Port: upstream gives a tuple that holds a math function, such
			// as a position of background-position, to resolve_math, which
			// raises TypeError, and then AttributeError when it logs it.
			// The port keeps the value, and the computer functions and the
			// layout resolve its math functions as they do for a single one.
			return value
		}
	}
	for _, v := range values {
		if checkMath(v) {
			function = v.(Token).Node
			token, err := resolveMath(function, s, key, nil)
			switch {
			case errors.Is(err, errPercentageInMath):
				solvedTokens = append(solvedTokens, function)
			case err != nil || token == nil:
				failed = true
			default:
				solvedTokens = append(solvedTokens, token)
			}
		} else if t, ok := v.(Token); ok {
			solvedTokens = append(solvedTokens, t.Node)
		} else {
			// Upstream gives a value that is not a token to the validator,
			// which raises an exception.
			failed = true
		}
		if failed {
			break
		}
	}
	if !failed {
		originalKey := strings.ReplaceAll(key, "_", "-")
		pair, err := validateNonShorthand(solvedTokens, originalKey, "", false)
		if err == nil {
			if _, pending := pair.value.(*Pending); !pending {
				return pair.value
			}
		}
	}
	if function != nil {
		line, column := function.Source()
		s.ctx.log.warning("Invalid math function at %d:%d: %s", line, column, function.Serialize())
	}
	if Inherited[key] && s.ParentStyle != nil {
		// Values in parent_style are already computed.
		value = s.ParentStyle.Get(key)
		s.store(key, value)
		return value
	}
	value = InitialValues[key]
	if !initialNotComputed[key] {
		// The value is the same as when computed.
		s.store(key, value)
	}
	return value
}

// ColorProfile is ColorProfile in weasyprint/css/__init__.py: the color
// profile of an @color-profile rule.
//
// Upstream reads the profile with PIL, for its name and its content. The
// port keeps the bytes. The name of the profile waits for the stage of the
// port that writes colors.
type ColorProfile struct {
	Src             string
	RenderingIntent Value
	Components      Value
	Content         []byte
}

// addLayer is _add_layer in weasyprint/css/__init__.py. It adds a layer to
// the list of layers, handling the order.
func addLayer(layer string, layers *[]string) {
	index := -1
	fullLayer := ""
	for part := range strings.SplitSeq(layer, ".") {
		if fullLayer != "" {
			fullLayer += "."
		}
		fullLayer += part
		if i := slices.Index(*layers, fullLayer); i >= 0 {
			index = i
			continue
		}
		if index < 0 {
			*layers = append(*layers, fullLayer)
			index = len(*layers) - 1
		} else {
			*layers = slices.Insert(*layers, index, fullLayer)
			index--
		}
	}
}

// parseLayer is _parse_layer in weasyprint/css/__init__.py. It parses the
// tokens of a layer name, and returns "" for None.
func parseLayer(tokens []tinycss2.Node) string {
	if len(tokens) == 0 {
		return ""
	}
	var newLayer strings.Builder
	lastDot := true
	for _, token := range tokens {
		switch t := token.(type) {
		case *tinycss2.IdentToken:
			if !lastDot {
				return ""
			}
			newLayer.WriteString(t.Value)
			lastDot = false
		case *tinycss2.LiteralToken:
			if t.Value != "." || lastDot {
				return ""
			}
			newLayer.WriteString(".")
			lastDot = true
		default:
			return ""
		}
	}
	if lastDot {
		return ""
	}
	return newLayer.String()
}

// parseColorProfileName is parse_color_profile_name in
// weasyprint/css/__init__.py. It returns "" for None.
func parseColorProfileName(prelude []tinycss2.Node) string {
	tokens := removeWhitespace(prelude)
	if len(tokens) != 1 {
		return ""
	}
	ident, ok := tokens[0].(*tinycss2.IdentToken)
	if !ok {
		return ""
	}
	if strings.HasPrefix(ident.Value, "--") || ident.Value == "device-cmyk" {
		return ident.Value
	}
	return ""
}

// pageData is a page data of parse_page_selectors: a PageSelectorType and
// its specificity.
type pageData struct {
	types       PageSelectorType
	specificity cssselect2.Specificity
}

// parsePageSelectors is parse_page_selectors in weasyprint/css/__init__.py.
// It parses the selectors of an @page rule. It returns false when something
// went wrong while parsing the rule. See
// https://drafts.csswg.org/css-page-3/#syntax-page-selector.
func parsePageSelectors(rule *tinycss2.AtRule) ([]pageData, bool) {
	tokens := removeWhitespace(rule.Prelude)
	var data []pageData

	// TODO: Specificity is probably wrong, should clean and test that.
	if len(tokens) == 0 {
		return []pageData{{}}, true
	}

	for len(tokens) > 0 {
		var types pageData

		if ident, ok := tokens[0].(*tinycss2.IdentToken); ok {
			tokens = tokens[1:]
			types.types.Name = ident.Value
			types.specificity[0] = 1
		}

		if len(tokens) == 1 {
			return nil, false
		} else if len(tokens) == 0 {
			return append(data, types), true
		}

	selectors:
		for len(tokens) > 0 {
			literal, ok := tokens[0].(*tinycss2.LiteralToken)
			tokens = tokens[1:]
			if !ok {
				return nil, false
			}

			switch literal.Value {
			case ":":
				if len(tokens) == 0 {
					return nil, false
				}
				switch t := tokens[0].(type) {
				case *tinycss2.IdentToken:
					tokens = tokens[1:]
					pseudoClass := t.LowerValue
					switch pseudoClass {
					case "left", "right":
						if types.types.Side != "" && types.types.Side != pseudoClass {
							return nil, false
						}
						types.types.Side = pseudoClass
						types.specificity[2]++
						continue
					case "blank":
						types.types.Blank = true
						types.specificity[1]++
						continue
					case "first":
						types.types.First = true
						types.specificity[1]++
						continue
					}
				case *tinycss2.FunctionBlock:
					tokens = tokens[1:]
					if t.Name != "nth" {
						return nil, false
					}
					nth := t.Arguments
					var group []tinycss2.Node
					hasGroup := false
					for i, argument := range t.Arguments {
						if ident, ok := argument.(*tinycss2.IdentToken); ok && ident.Value == "of" {
							// Upstream drops the token before "of", and
							// slices from the end for i == 0.
							end := i - 1
							if end < 0 {
								end += len(t.Arguments)
							}
							nth = t.Arguments[:end]
							group = t.Arguments[i+1:]
							hasGroup = true
							break
						}
					}

					a, b, ok := tinycss2.ParseNth(nth)
					if !ok {
						return nil, false
					}

					groupName := ""
					if hasGroup {
						group = removeWhitespace(group)
						if len(group) != 1 {
							return nil, false
						}
						ident, ok := group[0].(*tinycss2.IdentToken)
						if !ok {
							return nil, false
						}
						groupName = ident.Value
					}

					types.types.Index = &PageIndex{A: clampInt(a), B: clampInt(b), Group: groupName}
					// TODO: specificity is not specified yet
					// https://github.com/w3c/csswg-drafts/issues/3524
					types.specificity[1]++
					if groupName != "" {
						types.specificity[0]++
					}
					continue
				}
				return nil, false
			case ",":
				if len(tokens) > 0 && types.specificity != (cssselect2.Specificity{}) {
					break selectors
				}
				return nil, false
			}
		}

		data = append(data, types)
	}

	return data, true
}

// clampInt converts an int64 to an int, and saturates out of range.
func clampInt(v int64) int {
	return int(max(min(v, math.MaxInt32), math.MinInt32))
}

// rulePayload is the payload of a selector in the matcher of a CSS: the
// tuple (declarations, layer) of upstream. id tells the payloads apart, as
// id(declarations) does in upstream.
type rulePayload struct {
	declarations []Declaration
	layer        string
	hasLayer     bool
	id           int
}

// PageRule is an item of the page rules of a CSS: the tuple (rule,
// selector_list, declarations) of upstream.
type PageRule struct {
	// Rule is the @page rule or the margin rule.
	Rule tinycss2.Node
	// Selectors are the selectors of the rule.
	Selectors []PageSelector
	// Declarations are the declarations of the rule.
	Declarations []Declaration
}

// PageSelector is a selector of a PageRule: the tuple (specificity,
// pseudo_type, page_selector_type) of upstream. PseudoType is "" for None,
// or the at-keyword of a margin box, such as "@top-left".
type PageSelector struct {
	Specificity cssselect2.Specificity
	PseudoType  string
	Type        PageSelectorType
}

// cssShared holds the arguments of CSS() of upstream that the style sheets
// share with the style sheets that they import: matcher, page_rules, layers
// and layer.
type cssShared struct {
	matcher   *cssselect2.Matcher[*rulePayload]
	pageRules *[]PageRule
	layers    *[]string
	layer     string
	hasLayer  bool
}

// cssContext holds what the style sheets of a document share while they are
// parsed: the arguments of CSS() of upstream that pass from sheet to sheet,
// such as url_fetcher, font_config, counter_style and color_profiles, and
// the logger and the budget of the port.
type cssContext struct {
	log           *logger
	fetcher       *URLFetcher
	fontConfig    FontConfig
	counterStyle  CounterStyle
	colorProfiles map[string]*ColorProfile
	budget        *cascadeBudget
	importDepth   int
	payloads      int
}

// CSS is the class CSS of weasyprint/__init__.py: a style sheet that
// tinycss2 parsed.
type CSS struct {
	// BaseURL is the base URL of the style sheet.
	BaseURL string
	// Matcher holds the selectors of the style rules.
	Matcher *cssselect2.Matcher[*rulePayload]
	// PageRules are the @page rules and their margin rules.
	PageRules *[]PageRule
	// Layers are the names of the cascade layers, in order.
	Layers *[]string
}

// CSSSource is the source of a CSS that is a string: the argument string of
// CSS() of upstream, and its base URL.
type CSSSource struct {
	// Text is the text of the style sheet.
	Text string
	// BaseURL is the URL that the links of the style sheet resolve against,
	// or "" for none.
	BaseURL string
}

// newShared returns shared, or new shared values for nil.
func newShared(shared *cssShared) *cssShared {
	if shared == nil {
		shared = &cssShared{}
	}
	s := *shared
	if s.matcher == nil {
		s.matcher = cssselect2.NewMatcher[*rulePayload]()
	}
	if s.pageRules == nil {
		s.pageRules = &[]PageRule{}
	}
	if s.layers == nil {
		s.layers = &[]string{}
	}
	return &s
}

// newCSS is CSS.__init__ in weasyprint/__init__.py for a string.
func newCSS(ctx *cssContext, src CSSSource, mediaType string, shared *cssShared) *CSS {
	stylesheet := tinycss2.ParseStylesheet(src.Text)
	return newCSSFromRules(ctx, stylesheet, src.BaseURL, mediaType, shared)
}

// newCSSFromRules makes a CSS of parsed rules.
func newCSSFromRules(ctx *cssContext, stylesheet []tinycss2.Node, baseURL, mediaType string, shared *cssShared) *CSS {
	shared = newShared(shared)
	css := &CSS{
		BaseURL:   baseURL,
		Matcher:   shared.matcher,
		PageRules: shared.pageRules,
		Layers:    shared.layers,
	}
	preprocessStylesheet(ctx, mediaType, baseURL, stylesheet, shared, false)
	return css
}

// newCSSFromURL is CSS.__init__ in weasyprint/__init__.py for a URL. It
// returns an error that wraps ErrURLFetching when the fetcher cannot read
// the URL.
func newCSSFromURL(ctx *cssContext, u, mediaType string, shared *cssShared, checkMimeType bool) (*CSS, error) {
	budget := ctx.budget
	if budget != nil {
		// Port: the imports and the bytes of the style sheets are bounded.
		if budget.imports >= maxImports {
			return nil, fmt.Errorf("%w: %w: more than %d style sheets", ErrURLFetching, ivy.ErrLimit, maxImports)
		}
		budget.imports++
	}
	response, err := ctx.fetcher.Fetch(u)
	if err != nil {
		return nil, err
	}
	if budget != nil {
		budget.bytes += int64(len(response.Body))
		if budget.bytes > budget.limits.MaxMemory {
			return nil, fmt.Errorf("%w: %w: style sheets larger than %d bytes", ErrURLFetching, ivy.ErrLimit, budget.limits.MaxMemory)
		}
	}
	if checkMimeType && response.ContentType != "text/css" {
		ctx.log.error("Unsupported stylesheet type %s for %s", response.ContentType, response.URL)
		return newCSSFromRules(ctx, nil, u, mediaType, shared), nil
	}
	stylesheet, _ := tinycss2.ParseStylesheetBytes(response.Body, response.Charset, nil)
	return newCSSFromRules(ctx, stylesheet, response.URL, mediaType, shared), nil
}

// ruleContent returns the content of a rule, and false for the None of
// upstream: a rule that is not a block.
func ruleContent(rule tinycss2.Node) ([]tinycss2.Node, bool) {
	switch r := rule.(type) {
	case *tinycss2.QualifiedRule:
		return r.Content, true
	case *tinycss2.AtRule:
		return r.Content, r.Content != nil
	}
	return nil, false
}

// preprocessStylesheet is preprocess_stylesheet in
// weasyprint/css/__init__.py. It does what can be done early on a style
// sheet, before it is in a document.
func preprocessStylesheet(ctx *cssContext, deviceMediaType, baseURL string, stylesheetRules []tinycss2.Node, shared *cssShared, ignoreImports bool) {
	log := ctx.log
	for _, rule := range stylesheetRules {
		if _, ok := ruleContent(rule); !ok {
			if e, ok := rule.(*tinycss2.ParseError); ok {
				log.warning("Parse error at %d:%d: %s", e.SourceLine, e.SourceColumn, e.Message)
			}
			atRule, ok := rule.(*tinycss2.AtRule)
			if !ok {
				continue
			}
			if atRule.LowerAtKeyword != "import" && atRule.LowerAtKeyword != "layer" {
				log.warning("Unknown empty rule %s at %d:%d", rule, atRule.SourceLine, atRule.SourceColumn)
				continue
			}
		}

		switch r := rule.(type) {
		case *tinycss2.QualifiedRule:
			if preprocessStyleRule(ctx, baseURL, r, shared) {
				ignoreImports = true
			}

		case *tinycss2.AtRule:
			switch r.LowerAtKeyword {
			case "import":
				if ignoreImports {
					log.warning(
						"@import rule %q not at the beginning of the the whole rule was ignored at %d:%d.",
						tinycss2.Serialize(r.Prelude), r.SourceLine, r.SourceColumn)
					continue
				}
				preprocessImport(ctx, deviceMediaType, baseURL, r, shared)

			case "media":
				media, ok := parseMediaQuery(log, r.Prelude)
				if !ok {
					log.warning(
						"Invalid media type %q the whole @media rule was ignored at %d:%d.",
						tinycss2.Serialize(r.Prelude), r.SourceLine, r.SourceColumn)
					continue
				}
				if !evaluateMediaQuery(media, deviceMediaType) {
					continue
				}
				contentRules := tinycss2.ParseRuleList(r.Content)
				inner := *shared
				inner.layer, inner.hasLayer = "", false
				preprocessStylesheet(ctx, deviceMediaType, baseURL, contentRules, &inner, true)

			case "page":
				preprocessPageRule(ctx, baseURL, r, shared)
				ignoreImports = true

			case "font-face":
				ignoreImports = true
				content := tinycss2.ParseBlocksContents(r.Content)
				ruleDescriptors := map[string]Value{}
				for _, pair := range preprocessDescriptors(log, "font-face", baseURL, content) {
					ruleDescriptors[pair.name] = pair.value
				}
				missing := false
				for _, key := range [...]string{"src", "font_family"} {
					if _, ok := ruleDescriptors[key]; !ok {
						log.warning(
							"Missing %s descriptor in '@font-face' rule at %d:%d",
							strings.ReplaceAll(key, "_", "-"), r.SourceLine, r.SourceColumn)
						missing = true
						break
					}
				}
				if !missing && ctx.fontConfig != nil {
					ctx.fontConfig.AddFontFace(ruleDescriptors, ctx.fetcher)
				}

			case "color-profile":
				ignoreImports = true
				preprocessColorProfile(ctx, baseURL, r)

			case "counter-style":
				name := parseCounterStyleName(r.Prelude, ctx.counterStyle)
				if name == "" {
					log.warning(
						"Invalid counter style name %q, the whole @counter-style rule was ignored at %d:%d.",
						tinycss2.Serialize(r.Prelude), r.SourceLine, r.SourceColumn)
					continue
				}
				ignoreImports = true
				preprocessCounterStyle(ctx, baseURL, name, r)

			case "layer":
				preprocessLayer(ctx, deviceMediaType, baseURL, r, shared)

			default:
				log.warning("Unknown rule %s at %d:%d", rule, r.SourceLine, r.SourceColumn)
			}

		default:
			line, column := rule.Source()
			log.warning("Unknown rule %s at %d:%d", rule, line, column)
		}
	}
}

// preprocessStyleRule is the part of preprocess_stylesheet for a qualified
// rule. It returns true when the rule makes the next @import rules ignored.
func preprocessStyleRule(ctx *cssContext, baseURL string, rule *tinycss2.QualifiedRule, shared *cssShared) bool {
	log := ctx.log
	contents := tinycss2.ParseBlocksContents(rule.Content)
	selectorsDeclarations, err := preprocessRuleDeclarations(log, baseURL, contents, rule.Prelude, ctx.budget)
	debug := false
	if err == nil && len(selectorsDeclarations) > 0 {
		// Group the declarations of each list of selectors, as groupby of
		// upstream does.
		for start := 0; start < len(selectorsDeclarations) && err == nil; {
			end := start + 1
			for end < len(selectorsDeclarations) && selectorsDeclarations[end].group == selectorsDeclarations[start].group {
				end++
			}
			declarations := make([]Declaration, 0, end-start)
			for _, sd := range selectorsDeclarations[start:end] {
				declarations = append(declarations, sd.declaration)
			}
			ctx.payloads++
			payload := &rulePayload{declarations, shared.layer, shared.hasLayer, ctx.payloads}
			for _, selector := range selectorsDeclarations[start].selectors {
				if !ctx.budget.rule() {
					// Port: the number of rules is bounded.
					log.warning("Ignored rule at %d:%d, too many rules.", rule.SourceLine, rule.SourceColumn)
					return true
				}
				shared.matcher.AddSelector(selector, payload)
				if !pseudoElements[selector.PseudoElement] {
					prelude := tinycss2.Serialize(rule.Prelude)
					if strings.HasPrefix(selector.PseudoElement, "-") {
						debug = true
						err = &cssselect2.SelectorError{Message: fmt.Sprintf(
							"'%s', ignored prefixed pseudo-element: %s", prelude, selector.PseudoElement)}
					} else {
						err = &cssselect2.SelectorError{Message: fmt.Sprintf(
							"'%s', unknown pseudo-element: %s", prelude, selector.PseudoElement)}
					}
					break
				}
			}
			start = end
		}
	}
	if err != nil {
		var selectorError *cssselect2.SelectorError
		message := err.Error()
		if errors.As(err, &selectorError) {
			message = selectorError.Message
		}
		if debug {
			log.debug("Invalid or unsupported selector, %s", message)
		} else {
			log.warning("Invalid or unsupported selector, %s", message)
		}
		return false
	}
	return true
}

// preprocessImport is the part of preprocess_stylesheet for an @import rule.
func preprocessImport(ctx *cssContext, deviceMediaType, baseURL string, rule *tinycss2.AtRule, shared *cssShared) {
	log := ctx.log
	tokens := removeWhitespace(rule.Prelude)
	u := ""
	hasURL := false
	invalidSyntax := true
	if len(tokens) > 0 {
		if s, ok := tokens[0].(*tinycss2.StringToken); ok {
			invalidSyntax = false
			u, hasURL = urlJoin(log, baseURL, s.Value, false,
				fmt.Sprintf("@import at %d:%d", rule.SourceLine, rule.SourceColumn))
		} else {
			urlTuple, err := getURL(tokens[0], baseURL)
			if t, ok := urlTuple.(Tuple); err == nil && ok {
				if ext, ok := t[1].(Tuple); ok && isStr(ext[0], "external") {
					invalidSyntax = false
					u, hasURL = str(ext[1]), true
				}
			}
		}
	}
	if !hasURL {
		if invalidSyntax {
			log.warning(
				"Invalid @import rule %q, the whole rule was ignored at %d:%d.",
				tinycss2.Serialize(rule.Prelude), rule.SourceLine, rule.SourceColumn)
		}
		return
	}

	newLayer := ""
	var nextTokens []tinycss2.Node
	if len(tokens) > 0 {
		nextTokens = tokens[1:]
	}
	if len(nextTokens) > 0 {
		switch t := nextTokens[0].(type) {
		case *tinycss2.FunctionBlock:
			if t.Name == "layer" {
				nextTokens = nextTokens[1:]
				if newLayer = parseLayer(t.Arguments); newLayer == "" {
					log.warning(
						"Invalid layer name %q the whole @import rule was ignored at %d:%d.",
						t.Serialize(), rule.SourceLine, rule.SourceColumn)
					return
				}
			}
		case *tinycss2.IdentToken:
			if t.Value == "layer" {
				nextTokens = nextTokens[1:]
				newLayer = fmt.Sprintf("@anonymous%d", len(*shared.layers))
			}
		}
		if newLayer != "" {
			if shared.hasLayer {
				newLayer = shared.layer + "." + newLayer
			}
			addLayer(newLayer, shared.layers)
		}
	}

	media, ok := parseMediaQuery(log, nextTokens)
	if !ok {
		log.warning(
			"Invalid media type %q the whole @import rule was ignored at %d:%d.",
			tinycss2.Serialize(rule.Prelude), rule.SourceLine, rule.SourceColumn)
		return
	}
	if !evaluateMediaQuery(media, deviceMediaType) {
		return
	}
	if ctx.importDepth >= ctx.budget.limits.MaxDepth {
		// Port: the imports nest at most MaxDepth deep.
		log.error("Failed to load stylesheet at %s : %s", u, "too many nested imports")
		return
	}
	inner := *shared
	inner.layer, inner.hasLayer = newLayer, newLayer != ""
	ctx.importDepth++
	_, err := newCSSFromURL(ctx, u, deviceMediaType, &inner, false)
	ctx.importDepth--
	if err != nil {
		log.error("Failed to load stylesheet at %s : %s", u, err)
	}
}

// preprocessPageRule is the part of preprocess_stylesheet for an @page rule.
func preprocessPageRule(ctx *cssContext, baseURL string, rule *tinycss2.AtRule, shared *cssShared) {
	log := ctx.log
	data, ok := parsePageSelectors(rule)
	if !ok {
		log.warning(
			"Unsupported @page selector %q, the whole @page rule was ignored at %d:%d.",
			tinycss2.Serialize(rule.Prelude), rule.SourceLine, rule.SourceColumn)
		return
	}

	for _, page := range data {
		specificity := page.specificity
		pageSelectorType := page.types
		content := tinycss2.ParseBlocksContents(rule.Content)
		declarations := preprocessDeclarations(log, baseURL, content)

		if len(declarations) > 0 {
			if !ctx.budget.rule() {
				log.warning("Ignored rule at %d:%d, too many rules.", rule.SourceLine, rule.SourceColumn)
				return
			}
			*shared.pageRules = append(*shared.pageRules, PageRule{
				Rule:         rule,
				Selectors:    []PageSelector{{specificity, "", pageSelectorType}},
				Declarations: declarations,
			})
		}

		for _, item := range content {
			marginRule, ok := item.(*tinycss2.AtRule)
			if !ok {
				continue
			}
			line, column := marginRule.SourceLine, marginRule.SourceColumn
			if !pageMarginBoxes[marginRule.LowerAtKeyword] {
				log.warning("Unknown @page rule %s at %d:%d", marginRule, line, column)
				continue
			}
			if marginRule.Content == nil {
				log.warning("Empty @page rule %s at %d:%d", marginRule, line, column)
				continue
			}
			if len(removeWhitespace(marginRule.Prelude)) > 0 {
				log.warning(
					"Invalid prelude %q for @page rule %s at %d:%d",
					tinycss2.Serialize(marginRule.Prelude), marginRule, line, column)
				continue
			}
			declarations := preprocessDeclarations(log, baseURL, tinycss2.ParseBlocksContents(marginRule.Content))
			if len(declarations) > 0 {
				if !ctx.budget.rule() {
					log.warning("Ignored rule at %d:%d, too many rules.", line, column)
					return
				}
				*shared.pageRules = append(*shared.pageRules, PageRule{
					Rule:         marginRule,
					Selectors:    []PageSelector{{specificity, "@" + marginRule.LowerAtKeyword, pageSelectorType}},
					Declarations: declarations,
				})
			}
		}
	}
}

// preprocessColorProfile is the part of preprocess_stylesheet for an
// @color-profile rule.
func preprocessColorProfile(ctx *cssContext, baseURL string, rule *tinycss2.AtRule) {
	log := ctx.log
	name := parseColorProfileName(rule.Prelude)
	if name == "" {
		log.warning(
			"Invalid color profile name %q, the whole @color-profile rule was ignored at %d:%d.",
			tinycss2.Serialize(rule.Prelude), rule.SourceLine, rule.SourceColumn)
		return
	}

	content := tinycss2.ParseBlocksContents(rule.Content)
	ruleDescriptors := preprocessDescriptors(log, "color-profile", baseURL, content)

	descriptors := map[string]Value{
		"src":              nil,
		"rendering-intent": String("relative-colorimetric"),
		"components":       nil,
	}
	for _, pair := range ruleDescriptors {
		if _, ok := descriptors[pair.name]; ok {
			descriptors[pair.name] = pair.value
		} else {
			log.warning(
				"Unknown descriptor %q for profile named %q at %d:%d.",
				pair.name, tinycss2.Serialize(rule.Prelude), rule.SourceLine, rule.SourceColumn)
		}
	}

	src, ok := descriptors["src"].(Tuple)
	if !ok || len(src) < 2 {
		log.warning(
			"No source for profile named %q, the whole @color-profile rule was ignored at %d:%d.",
			tinycss2.Serialize(rule.Prelude), rule.SourceLine, rule.SourceColumn)
		return
	}

	u := str(src[1])
	response, err := ctx.fetcher.Fetch(u)
	if err != nil {
		// Upstream lets the error of the fetcher go up.
		log.error("Failed to load color profile at %s: %s", u, err)
		return
	}
	ctx.colorProfiles[name] = &ColorProfile{
		Src:             u,
		RenderingIntent: descriptors["rendering-intent"],
		Components:      descriptors["components"],
		Content:         response.Body,
	}
}

// preprocessCounterStyle is the part of preprocess_stylesheet for an
// @counter-style rule.
func preprocessCounterStyle(ctx *cssContext, baseURL, name string, rule *tinycss2.AtRule) {
	log := ctx.log
	content := tinycss2.ParseBlocksContents(rule.Content)
	counter := CounterStyleRule{
		"system":           nil,
		"negative":         nil,
		"prefix":           nil,
		"suffix":           nil,
		"range":            nil,
		"pad":              nil,
		"fallback":         nil,
		"symbols":          nil,
		"additive_symbols": nil,
	}
	for _, pair := range preprocessDescriptors(log, "counter-style", baseURL, content) {
		counter[pair.name] = pair.value
	}

	system, ok := counter["system"].(Tuple)
	if !ok || len(system) != 3 {
		system = Tuple{nil, String("symbolic"), nil}
	}

	if system[0] == nil {
		symbols, _ := counter["symbols"].(Tuple)
		additiveSymbols, _ := counter["additive_symbols"].(Tuple)
		switch systemName := str(system[1]); systemName {
		case "cyclic", "fixed", "symbolic":
			if len(symbols) < 1 {
				log.warning(
					"In counter style %q at %d:%d, counter style %q needs at least one symbol",
					name, rule.SourceLine, rule.SourceColumn, systemName)
				return
			}
		case "alphabetic", "numeric":
			if len(symbols) < 2 {
				log.warning(
					"In counter style %q at %d:%d, counter style %q needs at least two symbols",
					name, rule.SourceLine, rule.SourceColumn, systemName)
				return
			}
		case "additive":
			if len(additiveSymbols) < 2 {
				log.warning(
					`In counter style %q at %d:%d, counter style "additive" needs at least two additive symbols`,
					name, rule.SourceLine, rule.SourceColumn)
				return
			}
		}
	}

	ctx.counterStyle[name] = counter
}

// preprocessLayer is the part of preprocess_stylesheet for an @layer rule.
func preprocessLayer(ctx *cssContext, deviceMediaType, baseURL string, rule *tinycss2.AtRule, shared *cssShared) {
	log := ctx.log
	var newLayers []string
	invalid := false
	prelude := removeWhitespace(rule.Prelude)
	if len(prelude) > 0 {
		for _, tokens := range splitOnComma(prelude) {
			newLayer := parseLayer(tokens)
			if newLayer == "" {
				invalid = true
				break
			}
			if shared.hasLayer {
				newLayer = shared.layer + "." + newLayer
			}
			newLayers = append(newLayers, newLayer)
		}
	}
	if invalid {
		log.warning(
			"Unsupported @layer selector %q, the whole @layer rule was ignored at %d:%d.",
			tinycss2.Serialize(rule.Prelude), rule.SourceLine, rule.SourceColumn)
		return
	}
	if len(newLayers) > 1 {
		if len(rule.Content) > 0 {
			log.warning(
				"@layer rule with multiple layer names, the whole @layer rule was ignored at %d:%d.",
				rule.SourceLine, rule.SourceColumn)
			return
		}
		for _, newLayer := range newLayers {
			addLayer(newLayer, shared.layers)
		}
		return
	}

	var newLayer string
	if len(newLayers) == 1 {
		newLayer = newLayers[0]
	} else {
		newLayer = fmt.Sprintf("@anonymous%d", len(*shared.layers))
		if shared.hasLayer {
			newLayer = shared.layer + "." + newLayer
		}
	}
	addLayer(newLayer, shared.layers)

	if rule.Content == nil {
		return
	}
	contentRules := tinycss2.ParseRuleList(rule.Content)
	inner := *shared
	inner.layer, inner.hasLayer = newLayer, true
	preprocessStylesheet(ctx, deviceMediaType, baseURL, contentRules, &inner, true)
}

// ComputedStylesOptions holds the arguments of get_all_computed_styles of
// upstream, and the hooks and the limits of the port.
type ComputedStylesOptions struct {
	// UserStylesheets are the style sheets of the user. A
	// format gives its style sheets here: the cascade gives them the origin
	// "user", as upstream does for the style sheets of the API.
	UserStylesheets []CSSSource
	// PresentationalHints follows the presentational hints of HTML.
	PresentationalHints bool
	// Forms adds the style sheet of the forms.
	Forms bool
	// FontConfig receives the @font-face rules, or nil.
	FontConfig FontConfig
	// Metrics measures the fonts, or nil for the default metrics.
	Metrics TextMetrics
	// CounterStyle receives the @counter-style rules, or nil for a new one.
	CounterStyle CounterStyle
	// ColorProfiles receive the @color-profile rules, or nil for a new map.
	ColorProfiles map[string]*ColorProfile
	// TargetCollector collects the anchors, or nil.
	TargetCollector *TargetCollector
	// UserAgentStylesheets replaces the style sheets of the user agent when
	// it is not nil, as the tests of upstream do. The counter styles of the
	// user agent stay.
	UserAgentStylesheets []CSSSource
}

// GetAllComputedStyles is get_all_computed_styles in
// weasyprint/css/__init__.py. It computes the computed styles of all the
// elements of a document: it finds the author style sheets, parses them and
// applies them. It returns a StyleFor that gives the style of an element and
// of a pseudo-element.
//
// It returns an error that wraps ivy.ErrLimit when the cascade needs more
// work than the limits allow, and an error that wraps ivy.ErrFormat for a
// panic.
//
// Port: a panic becomes an error that wraps ivy.ErrFormat.
func GetAllComputedStyles(html *HTML, opts ComputedStylesOptions) (sf *StyleFor, err error) {
	defer func() {
		if r := recover(); r != nil {
			sf, err = nil, fmt.Errorf("%w: panic in the cascade: %v", ivy.ErrFormat, r)
		}
	}()
	ctx := &cssContext{
		log:           html.log,
		fetcher:       html.URLFetcher,
		fontConfig:    opts.FontConfig,
		counterStyle:  opts.CounterStyle,
		colorProfiles: opts.ColorProfiles,
		budget:        newCascadeBudget(html.Limits),
	}
	// List stylesheets. Order here is not important ('origin' is).
	var sheets []sheetOrigin
	if ctx.counterStyle == nil {
		ctx.counterStyle = CounterStyle{}
	}
	if ctx.colorProfiles == nil {
		ctx.colorProfiles = map[string]*ColorProfile{}
	}
	ua := userAgentStylesheets()
	for key, value := range ua.counterStyle {
		ctx.counterStyle[key] = value
	}
	if opts.UserAgentStylesheets != nil {
		for _, src := range opts.UserAgentStylesheets {
			sheet := newCSS(ctx, src, html.MediaType, nil)
			sheets = append(sheets, sheetOrigin{sheet: sheet, origin: "user agent"})
		}
	} else {
		sheets = append(sheets, sheetOrigin{sheet: ua.stylesheet, origin: "user agent"})
		if opts.Forms {
			sheets = append(sheets, sheetOrigin{sheet: ua.formStylesheet, origin: "user agent"})
		}
		if opts.PresentationalHints {
			sheets = append(sheets, sheetOrigin{sheet: ua.phStylesheet, origin: "author", hasSpecificity: true})
		}
	}
	for _, sheet := range findStylesheets(ctx, html.WrapperElement, html.MediaType, html.BaseURL) {
		sheets = append(sheets, sheetOrigin{sheet: sheet, origin: "author"})
	}
	for _, src := range opts.UserStylesheets {
		sheet := newCSS(ctx, src, html.MediaType, nil)
		sheets = append(sheets, sheetOrigin{sheet: sheet, origin: "user"})
	}
	for i := range sheets {
		sheets[i].layerIndex = map[string]int{}
		for index, layer := range *sheets[i].sheet.Layers {
			if _, ok := sheets[i].layerIndex[layer]; !ok {
				sheets[i].layerIndex[layer] = index
			}
		}
	}

	styleContext := &StyleContext{
		FontConfig: opts.FontConfig,
		Metrics:    opts.Metrics,
		Cache:      map[any]any{},
		log:        html.log,
		budget:     ctx.budget,
	}
	return newStyleFor(html, sheets, opts.PresentationalHints, styleContext, opts.TargetCollector)
}
