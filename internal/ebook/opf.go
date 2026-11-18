package ebook

import (
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/okage/roxmltree"
)

// Package is an OPF package document: the metadata, the manifest, the spine
// and the guide of an EPUB file. An HTMLZ or a TXTZ file holds one as
// metadata.opf, for its metadata and its cover.
type Package struct {
	// Version is the version of EPUB, such as "2.0" or "3.0".
	Version  string
	Metadata Metadata
	// Manifest holds the items that resolve to a path in the archive, in the
	// order of the file.
	Manifest []Item
	Spine    []ItemRef
	// Toc is the id of the item of the NCX file, from the spine.
	Toc   string
	Guide []Reference

	// ids and paths map the id and the path of an item to its index in
	// Manifest. The first item wins.
	ids   map[string]int
	paths map[string]int
}

// Metadata is the metadata of a package.
type Metadata struct {
	// Title is the main title.
	Title string
	// Creators are the authors.
	Creators []string
	Subjects []string
	Language string
	// Publisher is the publisher.
	Publisher string
	// Date is the date of the publication, as the file writes it.
	Date string
	// Modified is the date of the last change, as the file writes it.
	Modified string
	// Identifier is the unique identifier of the package. The obfuscation of
	// fonts takes its key from it.
	Identifier string
	// Identifiers are all of the identifiers, the unique one first.
	Identifiers []string
	// Generator is the program that made the file.
	Generator string
	// Cover is the id of the item of the cover image, from the meta element
	// of EPUB 2 whose name is cover.
	Cover string
	// Layout is the layout of the documents of the book: "pre-paginated"
	// for a book of fixed layout, whose documents are pages of a fixed
	// size, or "reflowable" or empty for a book whose text flows onto the
	// pages. It comes from the property rendition:layout of EPUB 3, or from
	// the meta element fixed-layout of the Kindle.
	Layout string
	// Viewport is the default size of a page of fixed layout, such as
	// "width=1200, height=1600", from the property rendition:viewport of
	// EPUB 3, or from the meta element original-resolution of the Kindle.
	Viewport string
}

// Item is an item of the manifest.
type Item struct {
	ID string
	// Path is the path of the item in the archive.
	Path      string
	MediaType string
	// Properties are the properties of EPUB 3, such as cover-image and nav.
	Properties []string
	// Fallback is the id of the item to use if a reader cannot read this
	// one.
	Fallback string
}

// HasProperty reports whether the item has the property p.
func (it Item) HasProperty(p string) bool {
	return slices.Contains(it.Properties, p)
}

// IsDocument reports whether the item is a document of XHTML or HTML.
func (it Item) IsDocument() bool {
	switch it.MediaType {
	case "application/xhtml+xml", "text/html", "text/x-oeb1-document":
		return true
	case "":
		switch strings.ToLower(path.Ext(it.Path)) {
		case ".xhtml", ".xht", ".html", ".htm":
			return true
		}
	}
	return false
}

// IsImage reports whether the item is a raster image.
func (it Item) IsImage() bool {
	if strings.HasPrefix(it.MediaType, "image/") {
		return it.MediaType != "image/svg+xml"
	}
	return IsImagePath(it.Path)
}

// IsImagePath reports whether the extension of name is a raster image.
func IsImagePath(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

// ItemRef is an item of the spine.
type ItemRef struct {
	IDRef string
	// Linear is false for an item outside of the main order of reading,
	// such as a note.
	Linear bool
	// Properties are the properties of EPUB 3, such as
	// rendition:layout-pre-paginated.
	Properties []string
}

// HasProperty reports whether the item of the spine has the property p.
func (ref ItemRef) HasProperty(p string) bool {
	return slices.Contains(ref.Properties, p)
}

// Reference is a reference of the guide of EPUB 2.
type Reference struct {
	// Type is the type in lower case, such as cover or text.
	Type  string
	Title string
	// Path is the path of the target in the archive.
	Path string
}

// ParseOPF parses an OPF package document. dir is the folder of the
// document in the archive, for the paths of its items. ParseOPF returns an
// error that wraps ivy.ErrFormat if buf is not a package document.
func ParseOPF(buf []byte, dir string, lim ivy.Limits) (*Package, error) {
	doc, err := ParseXML(buf, lim)
	if err != nil {
		return nil, fmt.Errorf("parsing package: %w", err)
	}
	root := doc.RootElement()
	if Local(root) != "package" {
		return nil, fmt.Errorf("package document with the root %q: %w", Local(root), ivy.ErrFormat)
	}
	p := &Package{
		Version: strings.TrimSpace(Attr(root, "version")),
		ids:     make(map[string]int),
		paths:   make(map[string]int),
	}
	for n := range root.Children() {
		if !n.IsElement() {
			continue
		}
		switch Local(n) {
		case "metadata":
			p.Metadata = readMetadata(n, Attr(root, "unique-identifier"))
		case "manifest":
			p.readManifest(n, dir)
		case "spine":
			p.Toc = Attr(n, "toc")
			for ref := range n.Children() {
				if ref.IsElement() && Local(ref) == "itemref" {
					p.Spine = append(p.Spine, ItemRef{
						IDRef:      Attr(ref, "idref"),
						Linear:     strings.TrimSpace(Attr(ref, "linear")) != "no",
						Properties: strings.Fields(Attr(ref, "properties")),
					})
				}
			}
		case "guide":
			for ref := range n.Children() {
				if !ref.IsElement() || Local(ref) != "reference" {
					continue
				}
				target, ok := Resolve(dir, Attr(ref, "href"))
				if !ok {
					continue
				}
				p.Guide = append(p.Guide, Reference{
					Type:  strings.ToLower(strings.TrimSpace(Attr(ref, "type"))),
					Title: Attr(ref, "title"),
					Path:  target,
				})
			}
		}
	}
	return p, nil
}

// readManifest reads the items of the manifest.
func (p *Package) readManifest(manifest roxmltree.Node, dir string) {
	for n := range manifest.Children() {
		if !n.IsElement() || Local(n) != "item" {
			continue
		}
		target, ok := Resolve(dir, Attr(n, "href"))
		if !ok {
			continue
		}
		mediaType, _, _ := strings.Cut(Attr(n, "media-type"), ";")
		it := Item{
			ID:         strings.TrimSpace(Attr(n, "id")),
			Path:       target,
			MediaType:  strings.ToLower(strings.TrimSpace(mediaType)),
			Properties: strings.Fields(Attr(n, "properties")),
			Fallback:   strings.TrimSpace(Attr(n, "fallback")),
		}
		if _, ok := p.ids[it.ID]; !ok && it.ID != "" {
			p.ids[it.ID] = len(p.Manifest)
		}
		if _, ok := p.paths[it.Path]; !ok {
			p.paths[it.Path] = len(p.Manifest)
		}
		p.Manifest = append(p.Manifest, it)
	}
}

// Item returns the item of the manifest with the id id.
func (p *Package) Item(id string) (Item, bool) {
	i, ok := p.ids[strings.TrimSpace(id)]
	if !ok {
		return Item{}, false
	}
	return p.Manifest[i], true
}

// ItemByPath returns the item of the manifest with the path name.
func (p *Package) ItemByPath(name string) (Item, bool) {
	i, ok := p.paths[name]
	if !ok {
		return Item{}, false
	}
	return p.Manifest[i], true
}

// Covers returns the paths of the cover images that the package names, in
// order: the item with the property cover-image of EPUB 3, the item of the
// meta element cover of EPUB 2, and the cover of the guide. A path can be
// an image or a page of XHTML.
func (p *Package) Covers() []string {
	var names []string
	for _, it := range p.Manifest {
		if it.HasProperty("cover-image") {
			names = append(names, it.Path)
			break
		}
	}
	if it, ok := p.Item(p.Metadata.Cover); ok {
		names = append(names, it.Path)
	}
	for _, ref := range p.Guide {
		if ref.Type == "cover" {
			names = append(names, ref.Path)
		}
	}
	return names
}

// Info returns the metadata as an ivy.Info. The identifier of the info is
// the unique identifier of the package, or else its first identifier.
func (p *Package) Info() ivy.Info {
	m := p.Metadata
	// identifier is the unique identifier, or else the first identifier
	var identifier string
	if len(m.Identifiers) != 0 {
		identifier = m.Identifiers[0]
	}
	return ivy.Info{
		Title:      m.Title,
		Authors:    slices.Clone(m.Creators),
		Subject:    strings.Join(m.Subjects, ", "),
		Language:   m.Language,
		Created:    ParseDate(m.Date),
		Modified:   ParseDate(m.Modified),
		Producer:   m.Generator,
		Publisher:  m.Publisher,
		Identifier: identifier,
	}
}

// metaElement is an element of the metadata, with the refinements of EPUB 3
// that name its id.
type metaElement struct {
	id    string
	text  string
	attrs map[string]string
}

// readMetadata reads the metadata. The elements of Dublin Core can sit in a
// dc-metadata element of OEB 1, so readMetadata reads each descendant.
func readMetadata(metadata roxmltree.Node, uniqueID string) Metadata {
	var m Metadata
	// refines maps an id to the properties that refine it, such as role
	refines := make(map[string]map[string]string)
	elements := make(map[string][]metaElement)
	for n := range metadata.Descendants() {
		if !n.IsElement() {
			continue
		}
		e := metaElement{id: strings.TrimSpace(Attr(n, "id")), text: NodeText(n), attrs: make(map[string]string)}
		for a := range n.Attributes() {
			e.attrs[a.Name()] = strings.TrimSpace(a.Value())
		}
		name := Local(n)
		if name == "meta" {
			if target, ok := strings.CutPrefix(e.attrs["refines"], "#"); ok {
				if refines[target] == nil {
					refines[target] = make(map[string]string)
				}
				refines[target][e.attrs["property"]] = e.text
				continue
			}
		}
		elements[name] = append(elements[name], e)
	}
	refined := func(e metaElement, property string) string {
		if v := e.attrs[property]; v != "" {
			return v
		}
		return refines[e.id][property]
	}

	for _, e := range elements["title"] {
		if e.text == "" {
			continue
		}
		if m.Title == "" || refined(e, "title-type") == "main" {
			m.Title = e.text
		}
		if refined(e, "title-type") == "main" {
			break
		}
	}
	var others []string
	for _, e := range elements["creator"] {
		if e.text == "" {
			continue
		}
		switch refined(e, "role") {
		case "", "aut":
			m.Creators = append(m.Creators, e.text)
		default:
			others = append(others, e.text)
		}
	}
	if len(m.Creators) == 0 {
		m.Creators = others
	}
	for _, e := range elements["subject"] {
		if e.text != "" {
			m.Subjects = append(m.Subjects, e.text)
		}
	}
	m.Language = first(elements["language"])
	m.Publisher = first(elements["publisher"])
	for _, e := range elements["date"] {
		switch strings.ToLower(e.attrs["event"]) {
		case "", "publication", "original-publication", "creation":
			if m.Date == "" {
				m.Date = e.text
			}
		case "modification":
			if m.Modified == "" {
				m.Modified = e.text
			}
		}
	}
	for _, e := range elements["identifier"] {
		if e.text == "" {
			continue
		}
		if e.id != "" && e.id == strings.TrimSpace(uniqueID) && m.Identifier == "" {
			m.Identifier = e.text
			m.Identifiers = slices.Insert(m.Identifiers, 0, e.text)
			continue
		}
		m.Identifiers = append(m.Identifiers, e.text)
	}
	for _, e := range elements["meta"] {
		switch {
		case e.attrs["property"] == "dcterms:modified" && e.text != "":
			m.Modified = e.text
		case strings.EqualFold(e.attrs["name"], "generator"):
			m.Generator = e.attrs["content"]
		case strings.EqualFold(e.attrs["name"], "cover"):
			m.Cover = e.attrs["content"]
		case e.attrs["property"] == "rendition:layout" && e.text != "":
			m.Layout = e.text
		case e.attrs["property"] == "rendition:viewport" && e.text != "" && m.Viewport == "":
			m.Viewport = e.text
		case strings.EqualFold(e.attrs["name"], "fixed-layout") && m.Layout == "":
			if strings.EqualFold(e.attrs["content"], "true") {
				m.Layout = "pre-paginated"
			}
		case strings.EqualFold(e.attrs["name"], "original-resolution") && m.Viewport == "":
			if w, h, ok := strings.Cut(strings.ToLower(e.attrs["content"]), "x"); ok {
				m.Viewport = "width=" + strings.TrimSpace(w) + ", height=" + strings.TrimSpace(h)
			}
		}
	}
	return m
}

// first returns the first text of the elements that is not empty.
func first(elements []metaElement) string {
	for _, e := range elements {
		if e.text != "" {
			return e.text
		}
	}
	return ""
}

// The layouts of a date of EPUB, which is a date of W3CDTF.
var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04Z07:00",
	"2006-01-02T15:04",
	"2006-01-02",
	"2006-01",
	"2006",
}

// ParseDate parses a date of W3CDTF, such as 2010, 2010-05-12 or
// 2010-05-12T10:00:00Z. It returns the zero time if s is not one.
func ParseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// Resolve returns the path in the archive of the link href, relative to the
// folder dir. It drops the fragment and the query of href, and decodes its
// escapes. It returns false for a link to another site, or for a link that
// goes out of the archive.
func Resolve(dir, href string) (string, bool) {
	href, _, _ = strings.Cut(strings.TrimSpace(href), "#")
	href, _, _ = strings.Cut(href, "?")
	if href == "" {
		return "", false
	}
	if u, err := url.Parse(href); err != nil || u.Scheme != "" || u.Host != "" {
		return "", false
	}
	if unescaped, err := url.PathUnescape(href); err == nil {
		href = unescaped
	}
	href = strings.ReplaceAll(href, `\`, "/")
	var name string
	if rest, ok := strings.CutPrefix(href, "/"); ok {
		name = path.Clean(rest)
	} else {
		name = path.Clean(path.Join(dir, href))
	}
	if name == "." || !fs.ValidPath(name) {
		return "", false
	}
	return name, true
}
