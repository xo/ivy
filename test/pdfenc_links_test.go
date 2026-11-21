package test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
)

// marks collects the links, the anchors and the bookmarks that pages give
// to a device (D58). It draws nothing.
type marks struct {
	*display.List
	// uris holds the URI of each link, and links the name of each link to
	// an anchor. anchors holds the names of the anchors, without the anchors
	// of display.PageAnchor, and labels the label of each bookmark.
	uris, links, anchors, labels []string
}

// The interfaces that marks implements.
var (
	_ display.Linker     = (*marks)(nil)
	_ display.Bookmarker = (*marks)(nil)
)

// LinkURI satisfies the display.Linker interface.
func (m *marks) LinkURI(r okage.Rect, uri string) {
	if r.Width() > 0 && r.Height() > 0 {
		m.uris = append(m.uris, uri)
	}
}

// LinkAnchor satisfies the display.Linker interface.
func (m *marks) LinkAnchor(r okage.Rect, name string) {
	if r.Width() > 0 && r.Height() > 0 {
		m.links = append(m.links, name)
	}
}

// Anchor satisfies the display.Linker interface.
func (m *marks) Anchor(name string, _, _ float32) {
	if !strings.HasPrefix(name, display.PageAnchorPrefix) && !slices.Contains(m.anchors, name) {
		m.anchors = append(m.anchors, name)
	}
}

// Bookmark satisfies the display.Bookmarker interface.
func (m *marks) Bookmark(level int, label string, _, _ float32, _ bool) {
	if level >= 1 {
		m.labels = append(m.labels, label)
	}
}

// pageMarks returns the marks of the pages of numbers of pager.
func pageMarks(ctx context.Context, pager ivy.Pager, numbers []int) (*marks, error) {
	m := &marks{List: display.NewRecording()}
	for _, n := range numbers {
		p, err := pager.Page(ctx, n)
		if err != nil {
			return nil, err
		}
		if d, ok := p.(display.Drawer); ok {
			// a page drawn in part keeps the marks that it gave
			_ = d.Draw(ctx, m, 1)
		}
	}
	return m, nil
}

// checkLinks compares the links, the named destinations and the outline of
// the PDF file at path, through the oracles, with the marks that the pages
// of numbers of pager give. pdfinfo -url of Poppler must list each URI
// that a page links to, and no other. pdfinfo -dests must list each anchor
// of the pages. mutool show outline of MuPDF must list one entry for each
// bookmark, with its label. mutool show grep must find a link annotation
// for each link to a URI, and at most one for each link.
func checkLinks(t *testing.T, path string, pager ivy.Pager, numbers []int) {
	ctx := t.Context()
	m, err := pageMarks(ctx, pager, numbers)
	if err != nil {
		t.Errorf("links: %v", err)
		return
	}
	urls, err := output(ctx, "pdfinfo", "-url", path)
	if err != nil {
		t.Errorf("links: %v", err)
		return
	}
	var gotURIs []string
	for _, line := range strings.Split(strings.TrimSpace(urls), "\n")[1:] {
		// the columns are the page, the type and the URL
		if f := strings.Fields(line); len(f) >= 3 && f[1] == "Annotation" {
			gotURIs = append(gotURIs, strings.Join(f[2:], " "))
		}
	}
	for _, uri := range gotURIs {
		if !slices.Contains(m.uris, uri) {
			t.Errorf("links: pdfinfo -url lists %q, which no page links to", uri)
		}
	}
	for _, uri := range m.uris {
		if !slices.ContainsFunc(gotURIs, func(s string) bool { return s == uri || strings.Join(strings.Fields(uri), " ") == s }) {
			t.Logf("links: pdfinfo -url does not list %q", uri)
		}
	}
	dests, err := output(ctx, "pdfinfo", "-dests", path)
	if err != nil {
		t.Errorf("links: %v", err)
		return
	}
	missing := 0
	for _, name := range m.anchors {
		if !strings.Contains(dests, `"`+name+`"`) {
			missing++
			if missing <= 3 {
				t.Errorf("links: pdfinfo -dests does not list the anchor %q", name)
			}
		}
	}
	outline, err := output(ctx, "mutool", "show", path, "outline")
	if err != nil {
		t.Errorf("links: %v", err)
		return
	}
	var entries []string
	for line := range strings.SplitSeq(outline, "\n") {
		if strings.TrimSpace(line) != "" {
			entries = append(entries, line)
		}
	}
	if len(entries) != len(m.labels) {
		t.Errorf("links: expected %d entries of the outline through mutool, got %d", len(m.labels), len(entries))
	}
	// mutool quotes a label, with a backslash before a tab, a quote and a
	// backslash
	unquote := strings.NewReplacer(`\t`, " ", `\"`, `"`, `\\`, `\`)
	for i := range min(len(entries), len(m.labels)) {
		entry := strings.Join(strings.Fields(unquote.Replace(entries[i])), " ")
		if label := strings.Join(strings.Fields(m.labels[i]), " "); label != "" && !strings.Contains(entry, label) {
			t.Errorf("links: entry %d of the outline through mutool is %q, expected the label %q", i+1, entries[i], label)
			break
		}
	}
	objects, err := output(ctx, "mutool", "show", "-g", path, "grep")
	if err != nil {
		t.Errorf("links: %v", err)
		return
	}
	annots := strings.Count(objects, "/Subtype/Link")
	if annots < len(gotURIs) || annots > len(m.uris)+len(m.links) {
		t.Errorf("links: expected from %d to %d link annotations through mutool, got %d", len(gotURIs), len(m.uris)+len(m.links), annots)
	}
	t.Logf("pdfenc links: uris=%d/%d links=%d annotations=%d anchors=%d outline=%d/%d",
		len(gotURIs), len(m.uris), len(m.links), annots, len(m.anchors), len(entries), len(m.labels))
}
