package ebook

import (
	"net/url"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
)

// TOC collects the entries of the table of contents of a book, within the
// limits of a configuration. The layout gives a bookmark at the target of
// each entry (D58).
type TOC struct {
	// Entries are the entries, in their order.
	Entries []layout.TOCEntry

	lim ivy.Limits
}

// NewTOC returns an empty table of contents with the limits of lim.
func NewTOC(lim ivy.Limits) *TOC {
	return &TOC{lim: lim.WithDefaults()}
}

// Add adds an entry with label at level, from 1, to target, a path of the
// book with or without a fragment. The label is collapsed to single spaces.
// Add drops an entry without a label or a target, and an entry deeper than
// MaxDepth. It returns false when the table holds MaxObjects entries, so
// that the caller can stop.
func (t *TOC) Add(label string, level int, target string) bool {
	if len(t.Entries) >= t.lim.MaxObjects {
		return false
	}
	label = strings.Join(strings.Fields(label), " ")
	if label == "" || target == "" || level < 1 || level > t.lim.MaxDepth {
		return true
	}
	t.Entries = append(t.Entries, layout.TOCEntry{Label: label, Level: level, Target: target})
	return true
}

// Full reports whether the table holds MaxObjects entries.
func (t *TOC) Full() bool {
	return len(t.Entries) >= t.lim.MaxObjects
}

// MaxDepth returns the deepest level that the table takes.
func (t *TOC) MaxDepth() int {
	return t.lim.MaxDepth
}

// ResolveTarget returns the target of an entry of a table of contents for
// the link href, relative to the folder dir: the path in the archive, as
// Resolve gives it, with "#" and the decoded fragment of href when it has
// one. It returns false where Resolve does.
func ResolveTarget(dir, href string) (string, bool) {
	href = strings.TrimSpace(href)
	name, ok := Resolve(dir, href)
	if !ok {
		return "", false
	}
	_, fragment, found := strings.Cut(href, "#")
	if !found || fragment == "" {
		return name, true
	}
	if unescaped, err := url.PathUnescape(fragment); err == nil {
		fragment = unescaped
	}
	return name + "#" + fragment, true
}
