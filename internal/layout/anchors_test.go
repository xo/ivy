package layout

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
)

// linkBook returns a book of two documents with links, anchors and
// headings, on pages of 100 by 80 pixels.
func linkBook(toc []TOCEntry) Source {
	const head = `<html xmlns="http://www.w3.org/1999/xhtml"><body>`
	return Source{
		FS: fstest.MapFS{
			"text/a.xhtml": {Data: []byte(head +
				`<h1 id="top">One</h1>` +
				`<p><a href="b.xhtml#sec">to b</a> <a href="#top">up</a></p>` +
				`<p><a href="https://example.com/x?y=1">web</a> <a href="missing.xhtml">gone</a> <a href="b.xhtml">b</a></p>` +
				`</body></html>`)},
			"text/b.xhtml": {Data: []byte(head +
				`<h2 style="bookmark-state: closed">Two</h2>` +
				`<p id="sec" style="transform: translate(5px, 7px)">Here</p>` +
				`<p id="sec">Again</p>` +
				`</body></html>`)},
		},
		Documents:   []string{"text/a.xhtml", "/text/b.xhtml"},
		StyleSheets: []string{bookStyle, "h1, h2, p { font-size: 10px; margin: 0 }"},
		TOC:         toc,
	}
}

// marks returns the lines of the links, the anchors and the bookmarks of
// page n of b.
func marks(t *testing.T, b *Book, n int) []string {
	t.Helper()
	var l display.List
	if err := b.Draw(context.Background(), n, &l, okage.IdentityTransform(), 1); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for line := range strings.SplitSeq(l.String(), "\n") {
		if strings.HasPrefix(line, "link ") || strings.HasPrefix(line, "anchor ") || strings.HasPrefix(line, "bookmark ") {
			lines = append(lines, line)
		}
	}
	return lines
}

// numbers matches the numbers of a line, which the test leaves out.
var numbers = regexp.MustCompile(`\[[-0-9 ]+\]| -?[0-9]+ -?[0-9]+( |$)`)

func TestLinks(t *testing.T) {
	b := NewBook(linkBook(nil), BookOptions{Links: true})
	for _, test := range []struct {
		n    int
		want []string
	}{
		{0, []string{
			`anchor "text/a.xhtml#top"`,
			`anchor "text/a.xhtml"`,
			`link #"text/b.xhtml#sec"`,
			`link #"text/a.xhtml#top"`,
			`link "https://example.com/x?y=1"`,
			`link #"text/b.xhtml"`,
			`bookmark 1 "One" true`,
		}},
		{1, []string{
			`anchor "text/b.xhtml#sec"`,
			`anchor "text/b.xhtml"`,
			`bookmark 2 "Two" false`,
		}},
	} {
		got := marks(t, b, test.n)
		for i := range got {
			got[i] = strings.Join(strings.Fields(numbers.ReplaceAllString(got[i], " ")), " ")
		}
		if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
			t.Errorf("page %d: expected\n%s\ngot\n%s", test.n, strings.Join(test.want, "\n"), strings.Join(got, "\n"))
		}
	}
}

func TestLinksTransform(t *testing.T) {
	b := NewBook(linkBook(nil), BookOptions{Links: true})
	var l display.List
	if err := b.Draw(context.Background(), 1, &l, okage.IdentityTransform(), 1); err != nil {
		t.Fatal(err)
	}
	// The first p of b.xhtml is at the left of the content box, 10 pixels
	// in, and the transform moves it by 5 by 7 pixels: 15 pixels are 11
	// points. The page has a scale of 0.75 points for each pixel, and the
	// anchor is in the pixels of the page.
	if !strings.Contains(l.String(), `anchor "text/b.xhtml#sec" 15 `) {
		t.Errorf("expected the anchor moved by its transform, got:\n%s", l.String())
	}
}

func TestLinksTOC(t *testing.T) {
	toc := []TOCEntry{
		{Label: "Book B", Level: 1, Target: "text/b.xhtml"},
		{Label: "Section", Level: 2, Target: "/text/./b.xhtml#sec"},
		{Label: "Start", Level: 1, Target: "text/a.xhtml"},
		{Label: "Nowhere", Level: 1, Target: "text/c.xhtml"},
	}
	b := NewBook(linkBook(toc), BookOptions{Links: true})
	var got []string
	for n := range 2 {
		for _, line := range marks(t, b, n) {
			if strings.HasPrefix(line, "bookmark ") {
				got = append(got, strings.Join(strings.Fields(numbers.ReplaceAllString(line, " ")), " "))
			}
		}
	}
	want := []string{`bookmark 1 "Start" true`, `bookmark 1 "Book B" true`, `bookmark 2 "Section" true`}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("expected\n%s\ngot\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

func TestLinksOff(t *testing.T) {
	// A book without Links, such as the text of a shape, gives no marks.
	b := NewBook(linkBook(nil), BookOptions{})
	if got := marks(t, b, 0); len(got) > 0 {
		t.Errorf("expected no marks, got %q", got)
	}
}

func TestLinksLimit(t *testing.T) {
	m := &pageMarks{max: 2}
	box := &Box{Kind: KindBlock, Style: newInitialStyle(nil)}
	box.Style.Set("anchor", String("a"))
	box.ownExtra().Link = GetLink(newLogger(nil), "#b", "")
	box.ownExtra().BookmarkLabel, box.ownExtra().HasBookmarkLabel = "c", true
	box.Style.Set("bookmark_level", Int(1))
	m.gather(box, nil, box.ext.Link)
	if len(m.links)+len(m.anchors)+len(m.bookmarks) != 2 {
		t.Errorf("expected 2 marks, got %d links, %d anchors and %d bookmarks", len(m.links), len(m.anchors), len(m.bookmarks))
	}
}

func TestDocumentPath(t *testing.T) {
	for in, want := range map[string]string{
		"a.xhtml":        "a.xhtml",
		"/a/./b.xhtml":   "a/b.xhtml",
		"a/../../b.html": "b.html",
		"":               "",
	} {
		if got := DocumentPath(in); got != want {
			t.Errorf("DocumentPath(%q) = %q, expected %q", in, got, want)
		}
	}
}

func TestRectangleAABB(t *testing.T) {
	if got := rectangleAABB(nil, 1, 2, 3, 4); got != [4]float64{1, 2, 4, 6} {
		t.Errorf("got %v", got)
	}
	// A rotation of a quarter turn.
	m := Matrix{A: 0, B: 1, C: -1, D: 0}
	if got := rectangleAABB(&m, 1, 2, 3, 4); got != [4]float64{-6, 1, -2, 4} {
		t.Errorf("got %v", got)
	}
}
