package ooxml

import (
	"context"
	"fmt"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
)

// The types of the relationships of the parts of the core and the
// extended properties, in the transitional and the strict forms.
const (
	relCore       = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties"
	relCoreStrict = "http://schemas.openxmlformats.org/officedocument/2006/relationships/metadata/core-properties"
	relApp        = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties"
	relAppStrict  = "http://purl.oclc.org/ooxml/officeDocument/relationships/extendedProperties"
)

// Describe returns the properties of the package. The core properties, in
// docProps/core.xml, give the title, the author, the subject, the language,
// the identifier and the dates. The author is dc:creator, or else
// cp:lastModifiedBy. The extended properties, in docProps/app.xml, give the
// program that made the package. A package without them says nothing. The
// properties are a help, so a part that is damaged says nothing too, and
// the fault is logged.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing office open xml: %w", err)
	}
	core, app := "docProps/core.xml", "docProps/app.xml"
	if rels, err := d.Relationships("", d.cfg.Limits); err == nil {
		for _, r := range rels {
			switch {
			case r.External:
			case r.Type == relCore || r.Type == relCoreStrict:
				core = r.Target
			case r.Type == relApp || r.Type == relAppStrict:
				app = r.Target
			}
		}
	}
	elems, err := d.properties(core)
	if err != nil {
		return ivy.Info{}, err
	}
	var info ivy.Info
	var creator, modifier string
	for _, n := range elems {
		text := strings.TrimSpace(ebook.NodeText(n))
		switch n.TagName().Name() {
		case "title":
			info.Title = text
		case "creator":
			creator = text
		case "lastModifiedBy":
			modifier = text
		case "subject":
			info.Subject = text
		case "language":
			info.Language = text
		case "identifier":
			info.Identifier = text
		case "created":
			info.Created = ebook.ParseDate(text)
		case "modified":
			info.Modified = ebook.ParseDate(text)
		}
	}
	switch {
	case creator != "":
		info.Authors = []string{creator}
	case modifier != "":
		info.Authors = []string{modifier}
	}
	if elems, err = d.properties(app); err != nil {
		return ivy.Info{}, err
	}
	var program, version string
	for _, n := range elems {
		switch n.TagName().Name() {
		case "Application":
			program = strings.TrimSpace(ebook.NodeText(n))
		case "AppVersion":
			version = strings.TrimSpace(ebook.NodeText(n))
		}
	}
	if program != "" {
		info.Producer = strings.TrimSpace(program + " " + version)
	}
	return info, nil
}

// properties returns the child elements of the root of the part name, which
// holds properties. A package without the part gives none. A part that is
// damaged gives none too, and the fault is logged.
func (d *Document) properties(name string) ([]roxmltree.Node, error) {
	if !d.HasPart(name) {
		return nil, nil
	}
	buf, err := d.ReadPart(name)
	if err != nil {
		return nil, err
	}
	doc, err := ParseXML(buf, d.cfg.Limits)
	if err != nil {
		d.cfg.Log().Debug("ooxml: properties left out", "name", name, "err", err)
		return nil, nil
	}
	var elems []roxmltree.Node
	for n := range doc.RootElement().Children() {
		if n.IsElement() {
			elems = append(elems, n)
		}
	}
	return elems, nil
}
