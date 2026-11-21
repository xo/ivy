// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports no upstream file. It holds the document tree that the whole
// layout uses in place of the ElementTree of Python, built from an HTML
// document of okage or from an XHTML document of roxmltree.

package layout

import (
	"iter"
	"strings"

	"github.com/xo/okage/cssselect2"
	"github.com/xo/okage/html"
	"github.com/xo/okage/roxmltree"
)

// The namespaces that the tree names.
const (
	nsHTML  = "http://www.w3.org/1999/xhtml"
	nsSVG   = "http://www.w3.org/2000/svg"
	nsXML   = "http://www.w3.org/XML/1998/namespace"
	nsXLink = "http://www.w3.org/1999/xlink"
)

// Element is an element of the document tree. It is the Element of
// xml.etree.ElementTree that WeasyPrint reads, as tinyhtml5 builds it with
// namespace_html_elements=False:
//
//   - Tag is the name of an element of HTML, such as "p", and the name of
//     any other element in the Clark notation, such as
//     "{http://www.w3.org/2000/svg}svg". An element of XHTML has the tag of
//     the same element of HTML.
//   - Attrib holds the attributes in the order of the source. The name of an
//     attribute in a namespace is in the Clark notation, such as
//     "{http://www.w3.org/1999/xlink}href".
//   - Text is the text before the first child element, and Tail is the text
//     after the end of the element and before its next sibling, as the
//     attributes text and tail of ElementTree.
//   - Children are the child elements. Python iterates over an element to
//     get them.
//
// The tree holds no comment and no processing instruction. Their tails join
// the text around them. The tree does not change after it is built, except
// where a later stage of the port changes it as upstream does.
type Element struct {
	Tag      string
	Attrib   []Attr
	Text     string
	Tail     string
	Children []*Element
	Parent   *Element
}

// Attr is an attribute of an Element.
type Attr struct {
	Name  string
	Value string
}

// Get is Element.get of ElementTree. It returns the value of an attribute,
// and false when the element has no such attribute.
func (e *Element) Get(name string) (string, bool) {
	for _, attr := range e.Attrib {
		if attr.Name == name {
			return attr.Value, true
		}
	}
	return "", false
}

// GetDefault is Element.get of ElementTree with a default value.
func (e *Element) GetDefault(name, def string) string {
	if v, ok := e.Get(name); ok {
		return v
	}
	return def
}

// Iter is Element.iter of ElementTree. It returns the element and its
// descendants, in the order of the document.
func (e *Element) Iter() iter.Seq[*Element] {
	return func(yield func(*Element) bool) {
		e.walk(yield)
	}
}

// walk calls yield for the element and its descendants. It returns false
// when yield stops the walk. It keeps a stack, so a deep tree does not grow
// the stack of the goroutine.
func (e *Element) walk(yield func(*Element) bool) bool {
	type frame struct {
		el *Element
		i  int
	}
	if !yield(e) {
		return false
	}
	stack := []frame{{e, 0}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.i >= len(top.el.Children) {
			stack = stack[:len(stack)-1]
			continue
		}
		child := top.el.Children[top.i]
		top.i++
		if !yield(child) {
			return false
		}
		stack = append(stack, frame{child, 0})
	}
	return true
}

// IterTag is Element.iter(tag) of ElementTree. It returns the element and
// its descendants with the tag, in the order of the document.
func (e *Element) IterTag(tag string) iter.Seq[*Element] {
	return func(yield func(*Element) bool) {
		for el := range e.Iter() {
			if el.Tag == tag && !yield(el) {
				return
			}
		}
	}
}

// localName returns the local name and the namespace of the tag.
func (e *Element) localName() (local, namespace string) {
	if rest, ok := strings.CutPrefix(e.Tag, "{"); ok {
		if ns, name, ok := strings.Cut(rest, "}"); ok {
			return name, ns
		}
	}
	return e.Tag, ""
}

// appendText adds text after the last child of the element, or to its text
// when it has no child.
func (e *Element) appendText(text string) {
	if n := len(e.Children); n > 0 {
		e.Children[n-1].Tail += text
	} else {
		e.Text += text
	}
}

// appendChild adds a child element.
func (e *Element) appendChild(child *Element) {
	child.Parent = e
	e.Children = append(e.Children, child)
}

// clarkName returns a name in the Clark notation, or the local name alone
// when the namespace is empty.
func clarkName(namespace, local string) string {
	if namespace == "" {
		return local
	}
	return "{" + namespace + "}" + local
}

// clarkNames keeps the names in the Clark notation of a tree, so that the
// elements and the attributes of a name share one string.
//
// Port: ElementTree makes a string for each element. The names of a tree
// are few, and the elements of a book are many.
type clarkNames map[[2]string]string

// name returns clarkName(namespace, local).
func (c clarkNames) name(namespace, local string) string {
	if namespace == "" {
		return local
	}
	key := [2]string{namespace, local}
	if name, ok := c[key]; ok {
		return name
	}
	name := clarkName(namespace, local)
	c[key] = name
	return name
}

// slab gives new values of a type from blocks, for the many values of a
// tree that last as long as the tree, such as its elements and its boxes.
// Each block is twice as large as the last one, up to maxSlab values.
//
// Port: upstream makes each value on its own. A block makes many values
// with one allocation.
type slab[T any] struct {
	free []T
	size int
}

// maxSlab is the largest number of values of a block of a slab.
const maxSlab = 128

// next returns a new zero value.
func (s *slab[T]) next() *T {
	if len(s.free) == 0 {
		s.size = min(max(2*s.size, 8), maxSlab)
		s.free = make([]T, s.size)
	}
	v := &s.free[0]
	s.free = s.free[1:]
	return v
}

// treeBudget counts the elements that a tree builds, against a limit.
type treeBudget struct {
	elements int
	limit    int
}

// take counts one element. It returns false past the limit.
func (b *treeBudget) take() bool {
	b.elements++
	return b.limit <= 0 || b.elements <= b.limit
}

// TreeFromHTML builds the tree of an HTML document of okage. It returns the
// html element, the root of the tree, or nil for a document without it.
//
// Port: maxElements bounds the number of elements, from ivy.Limits, and
// maxDepth bounds their depth. The elements past a limit are left out, with
// their text.
func TreeFromHTML(doc *html.Document, maxElements, maxDepth int) *Element {
	root := doc.DocumentElement()
	if root == nil {
		return nil
	}
	budget := &treeBudget{limit: maxElements}
	if !budget.take() {
		return nil
	}
	var elements slab[Element]
	el := htmlElement(root, &elements)
	type frame struct {
		src   *html.Node
		dst   *Element
		depth int
	}
	stack := []frame{{root, el, 1}}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		// Port: the list of the children has the size of their number.
		n := 0
		for child := range f.src.Children() {
			if child.Type() == html.ElementNode {
				n++
			}
		}
		if n > 0 {
			f.dst.Children = make([]*Element, 0, n)
		}
		for child := range f.src.Children() {
			switch child.Type() {
			case html.TextNode:
				f.dst.appendText(child.Data())
			case html.ElementNode:
				if f.depth >= maxDepth || !budget.take() {
					continue
				}
				c := htmlElement(child, &elements)
				f.dst.appendChild(c)
				stack = append(stack, frame{child, c, f.depth + 1})
			}
		}
	}
	return el
}

// htmlElement makes an Element of an element of okage, with no children.
// elements gives the element.
func htmlElement(n *html.Node, elements *slab[Element]) *Element {
	tag := n.Name()
	if ns := n.Namespace(); ns != "" && ns != html.NamespaceHTML {
		tag = clarkName(ns, tag)
	}
	attrs := n.Attributes()
	el := elements.next()
	el.Tag, el.Attrib = tag, make([]Attr, len(attrs))
	for i, attr := range attrs {
		el.Attrib[i] = Attr{Name: clarkName(attr.Namespace, attr.Name), Value: attr.Value}
	}
	return el
}

// TreeFromXML builds the tree of an XML document of roxmltree, such as an
// XHTML document. It returns the root element of the document. An element in
// the namespace of XHTML gets the tag of the same element of HTML, so that
// the rest of the layout reads both kinds of documents the same way.
//
// Port: maxElements and maxDepth bound the elements, as in TreeFromHTML.
func TreeFromXML(doc *roxmltree.Document, maxElements, maxDepth int) *Element {
	root, ok := doc.Root().FirstElementChild()
	if !ok {
		return nil
	}
	budget := &treeBudget{limit: maxElements}
	if !budget.take() {
		return nil
	}
	names := clarkNames{}
	var elements slab[Element]
	el := xmlElement(root, names, &elements)
	type frame struct {
		src   roxmltree.Node
		dst   *Element
		depth int
	}
	stack := []frame{{root, el, 1}}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		// Port: the list of the children has the size of their number.
		n := 0
		for child := range f.src.Children() {
			if child.IsElement() {
				n++
			}
		}
		if n > 0 {
			f.dst.Children = make([]*Element, 0, n)
		}
		for child := range f.src.Children() {
			switch {
			case child.IsText():
				text, _ := child.Text()
				f.dst.appendText(text)
			case child.IsElement():
				if f.depth >= maxDepth || !budget.take() {
					continue
				}
				c := xmlElement(child, names, &elements)
				f.dst.appendChild(c)
				stack = append(stack, frame{child, c, f.depth + 1})
			}
		}
	}
	return el
}

// xmlElement makes an Element of an element of roxmltree, with no children.
// names keeps the names of the tree, and elements gives the element.
func xmlElement(n roxmltree.Node, names clarkNames, elements *slab[Element]) *Element {
	name := n.TagName()
	ns, _ := name.Namespace()
	tag := name.Name()
	if ns != nsHTML {
		tag = names.name(ns, tag)
	}
	el := elements.next()
	el.Tag = tag
	count := 0
	for range n.Attributes() {
		count++
	}
	if count > 0 {
		el.Attrib = make([]Attr, 0, count)
	}
	for attr := range n.Attributes() {
		ans, _ := attr.Namespace()
		el.Attrib = append(el.Attrib, Attr{Name: names.name(ans, attr.Name()), Value: attr.Value()})
	}
	return el
}

// selectorElement adapts an Element to cssselect2.Element, as the
// ElementTree of Python is for cssselect2. It is comparable.
type selectorElement struct {
	el *Element
}

// Children satisfies cssselect2.Element.
func (s selectorElement) Children() iter.Seq[cssselect2.Element] {
	return func(yield func(cssselect2.Element) bool) {
		for _, child := range s.el.Children {
			if !yield(selectorElement{child}) {
				return
			}
		}
	}
}

// LocalName satisfies cssselect2.Element.
func (s selectorElement) LocalName() string {
	local, _ := s.el.localName()
	return local
}

// NamespaceURL satisfies cssselect2.Element.
func (s selectorElement) NamespaceURL() string {
	_, ns := s.el.localName()
	return ns
}

// Attr satisfies cssselect2.Element.
func (s selectorElement) Attr(namespaceURL, name string) (string, bool) {
	return s.el.Get(clarkName(namespaceURL, name))
}

// Text satisfies cssselect2.Element.
func (s selectorElement) Text() string {
	return s.el.Text
}

// wrapperElement returns the Element of a wrapper of cssselect2, the
// etree_element of upstream.
func wrapperElement(w *cssselect2.ElementWrapper) *Element {
	if w == nil {
		return nil
	}
	if s, ok := w.Element.(selectorElement); ok {
		return s.el
	}
	return nil
}
