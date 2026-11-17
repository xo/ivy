package odf

import (
	"context"
	"fmt"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/ooxml"
)

// metaName is the name of the entry of the metadata.
const metaName = "meta.xml"

// The capabilities of a document.
var _ ivy.Describer = (*Document)(nil)

// Describe satisfies the ivy.Describer interface. It reads the metadata of
// the file, in meta.xml: the title, the subject, the language, the dates,
// the program that made the file, and the author. The author is
// meta:initial-creator, who made the file, or else dc:creator, who changed
// it last. A file without meta.xml says nothing. The metadata is a help,
// so an entry that is damaged says nothing too, and the fault is logged.
func (d *Document) Describe(ctx context.Context) (info ivy.Info, err error) {
	defer ebook.Recover("describing odf", &err)
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing odf: %w", err)
	}
	if d.archive.Find(metaName) == nil {
		return ivy.Info{}, nil
	}
	buf, err := d.readEntry(ctx, metaName)
	if err != nil {
		return ivy.Info{}, err
	}
	doc, err := ooxml.ParseXML(buf, d.lim)
	if err != nil {
		d.cfg.Log().Debug("odf: metadata left out", "err", err)
		return ivy.Info{}, nil
	}
	meta, ok := odtChild(doc.RootElement(), "meta")
	if !ok {
		return ivy.Info{}, nil
	}
	var creator, initial string
	for n := range meta.Children() {
		if !n.IsElement() {
			continue
		}
		text := strings.TrimSpace(ebook.NodeText(n))
		switch n.TagName().Name() {
		case "title":
			info.Title = text
		case "subject":
			info.Subject = text
		case "language":
			info.Language = text
		case "initial-creator":
			initial = text
		case "creator":
			creator = text
		case "creation-date":
			info.Created = ebook.ParseDate(text)
		case "date":
			info.Modified = ebook.ParseDate(text)
		case "generator":
			info.Producer = text
		}
	}
	switch {
	case initial != "":
		info.Authors = []string{initial}
	case creator != "":
		info.Authors = []string{creator}
	}
	return info, nil
}
