package ooxml

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/zipdoc"
)

// The types of the relationships that lead to the main part of a package.
// ECMA-376 has a transitional and a strict form of each.
const (
	RelOfficeDocument       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	RelOfficeDocumentStrict = "http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument"
)

// Relationship is a relationship of a part, from a .rels part of the
// Open Packaging Conventions (ECMA-376 Part 2).
type Relationship struct {
	ID   string
	Type string
	// Target is the name of the target part, resolved from the source
	// part, without a leading slash. For an external target, it is the URL
	// as the package gives it.
	Target   string
	External bool
}

// TypeIs reports whether the type of r ends with the name, such as "image"
// or "officeDocument". It matches the transitional and the strict form of
// a type of ECMA-376.
func (r Relationship) TypeIs(name string) bool {
	return strings.HasSuffix(r.Type, "/"+name)
}

// Archive returns the zip archive of the package.
func (d *Document) Archive() *zipdoc.Archive {
	return d.archive
}

// ReadPart reads the part name, such as "word/document.xml". A leading
// slash is ignored. It returns an error that wraps ivy.ErrFormat if the
// package has no such part, and one that wraps ivy.ErrLimit if the part is
// larger than the limits.
func (d *Document) ReadPart(name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "/")
	f := d.archive.Find(name)
	if f == nil {
		return nil, fmt.Errorf("part %s: %w", name, ivy.ErrFormat)
	}
	return d.archive.Read(f)
}

// HasPart reports whether the package holds the part name.
func (d *Document) HasPart(name string) bool {
	return d.archive.Find(strings.TrimPrefix(name, "/")) != nil
}

// Relationships returns the relationships of the part source, from its
// .rels part. The source "" is the package itself, whose relationships are
// in _rels/.rels. A part without a .rels part has no relationships.
func (d *Document) Relationships(source string, lim ivy.Limits) ([]Relationship, error) {
	source = strings.TrimPrefix(source, "/")
	dir, file := path.Split(source)
	name := dir + "_rels/" + file + ".rels"
	if !d.HasPart(name) {
		return nil, nil
	}
	buf, err := d.ReadPart(name)
	if err != nil {
		return nil, err
	}
	doc, err := ParseXML(buf, lim)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	var rels []Relationship
	for n := range doc.RootElement().Children() {
		if !n.IsElement() || n.TagName().Name() != "Relationship" {
			continue
		}
		r := Relationship{ID: Attr(n, "Id"), Type: Attr(n, "Type"), Target: Attr(n, "Target")}
		if strings.EqualFold(Attr(n, "TargetMode"), "External") {
			r.External = true
		} else {
			r.Target = ResolveTarget(source, r.Target)
		}
		rels = append(rels, r)
	}
	return rels, nil
}

// Relationship returns the relationship of the part source with the ID id.
func (d *Document) Relationship(source, id string, lim ivy.Limits) (Relationship, bool, error) {
	rels, err := d.Relationships(source, lim)
	if err != nil {
		return Relationship{}, false, err
	}
	for _, r := range rels {
		if r.ID == id {
			return r, true, nil
		}
	}
	return Relationship{}, false, nil
}

// MainPart returns the name of the main part of the package, such as
// "word/document.xml", from the relationship of the type officeDocument.
func (d *Document) MainPart(lim ivy.Limits) (string, error) {
	rels, err := d.Relationships("", lim)
	if err != nil {
		return "", err
	}
	for _, r := range rels {
		if !r.External && (r.Type == RelOfficeDocument || r.Type == RelOfficeDocumentStrict) {
			return r.Target, nil
		}
	}
	return "", fmt.Errorf("package without a main part: %w", ivy.ErrFormat)
}

// ResolveTarget resolves the target of a relationship from the part
// source, as the Open Packaging Conventions say. A target that starts with
// a slash is from the root of the package. The result has no leading slash.
func ResolveTarget(source, target string) string {
	target = strings.ReplaceAll(target, `\`, "/")
	if i := strings.IndexAny(target, "#?"); i >= 0 {
		target = target[:i]
	}
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	return strings.TrimPrefix(path.Clean(path.Join("/", path.Dir("/"+source), target)), "/")
}

// ParseXML parses a part of XML with the XML parser of okage (D7). It
// reads UTF-8 and UTF-16, as ECMA-376 allows. The number of nodes is
// bounded by lim.MaxObjects. It returns an error that wraps ivy.ErrFormat
// for XML that is not well formed, and one that wraps ivy.ErrLimit for too
// many nodes.
func ParseXML(buf []byte, lim ivy.Limits) (*roxmltree.Document, error) {
	lim = lim.WithDefaults()
	opt := roxmltree.DefaultParsingOptions()
	opt.NodesLimit = uint32(min(lim.MaxObjects, 1<<31))
	doc, err := roxmltree.ParseWithOptions(xmlText(buf), opt)
	switch {
	case errors.Is(err, okage.ErrXMLNodesLimitReached):
		return nil, fmt.Errorf("xml of more than %d nodes: %w", lim.MaxObjects, ivy.ErrLimit)
	case err != nil:
		return nil, fmt.Errorf("%w: %w", ivy.ErrFormat, err)
	}
	return doc, nil
}

// xmlText returns the text of a part of XML as UTF-8, without its byte
// order mark and without its XML declaration, which can name another
// encoding.
func xmlText(buf []byte) string {
	text := ebook.Text(buf)
	if strings.HasPrefix(text, "<?xml") {
		if end := strings.Index(text, "?>"); end >= 0 {
			return text[end+2:]
		}
	}
	return text
}

// Attr returns the value of the attribute of n with the local name name,
// in any namespace, or the empty string.
func Attr(n roxmltree.Node, name string) string {
	for a := range n.Attributes() {
		if a.Name() == name {
			return a.Value()
		}
	}
	return ""
}
