package pdfenc

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/pdf"
)

// linkPages returns three pages with links, anchors and bookmarks: a link
// to a URI, a link to an anchor of a later page and of an earlier page, a
// link to a page anchor, a link in a turned space, and the bookmarks of
// three levels.
func linkPages() []ivy.Page {
	r := func(x, y, w, h float32) okage.Rect {
		rect, _ := okage.RectFromXYWH(x, y, w, h)
		return rect
	}
	draw := func(f func(d display.Device)) *scenePage {
		return &scenePage{w: 200, h: 200, draw: func(d display.Device) {
			d.FillPath(rect(10, 10, 20, 20), okage.FillRuleWinding, solid(0, 0, 0, 255))
			f(d)
		}}
	}
	return []ivy.Page{
		draw(func(d display.Device) {
			display.Bookmark(d, 1, "Part one", 0, 0, true)
			display.Anchor(d, "start", 0, 0)
			display.LinkURI(d, r(10, 20, 50, 10), "https://example.com/a")
			display.LinkAnchor(d, r(10, 40, 50, 10), "later")
			display.LinkAnchor(d, r(10, 60, 50, 10), "nowhere")
			display.LinkAnchor(d, r(10, 80, 50, 10), display.PageAnchor(2, 0, 0))
			display.Anchor(d, display.PageAnchor(0, 0, 0), 0, 0)
		}),
		draw(func(d display.Device) {
			d.Save()
			d.Concat(okage.TransformFromTranslate(20, 30))
			display.Bookmark(d, 3, "Deep", 0, 0, false)
			display.Anchor(d, "later", 10, 10)
			display.Anchor(d, "start", 50, 50)
			// a link in a turned space gets quad points
			d.Concat(okage.TransformFromRotate(30))
			display.LinkAnchor(d, r(0, 0, 40, 10), "start")
			d.Restore()
			display.Anchor(d, display.PageAnchor(1, 0, 0), 0, 0)
		}),
		draw(func(d display.Device) {
			display.Bookmark(d, 2, "Part two", 0, 100, true)
			display.Anchor(d, display.PageAnchor(2, 0, 0), 0, 0)
		}),
	}
}

// encodeLinks writes the pages of linkPages, with the options.
func encodeLinks(t *testing.T, opts ...Option) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := Encode(t.Context(), &buf, &document{pages: linkPages()}, opts...); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// readMarks opens the file with the pdf package of ivy, and returns the
// links, the anchors and the bookmarks that each page gives.
func readMarks(t *testing.T, data []byte) [][]string {
	t.Helper()
	doc, err := pdf.Open(t.Context(), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	n, err := doc.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	marks := make([][]string, 0, n)
	for i := range n {
		page, err := doc.Page(t.Context(), i)
		if err != nil {
			t.Fatal(err)
		}
		var l display.List
		if err := page.(display.Drawer).Draw(context.Background(), &l, 1); err != nil {
			t.Fatal(err)
		}
		var lines []string
		for line := range strings.SplitSeq(l.String(), "\n") {
			if strings.HasPrefix(line, "link ") || strings.HasPrefix(line, "anchor ") || strings.HasPrefix(line, "bookmark ") {
				lines = append(lines, line)
			}
		}
		marks = append(marks, lines)
	}
	return marks
}

func TestLinks(t *testing.T) {
	data := encodeLinks(t)
	for _, s := range []string{"/Dests", "/Outlines", "/QuadPoints"} {
		if !bytes.Contains(data, []byte(s)) {
			t.Errorf("expected %s in the file", s)
		}
	}
	// "nowhere" names no anchor, and the page anchor of page 2 has no link
	for _, s := range []string{"nowhere", "(ivy-page-2-0-0)"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("expected no %s in the file", s)
		}
	}
	got := readMarks(t, data)
	want := [][]string{
		{
			`bookmark 1 "Part one" 0 0 true`,
			`anchor "start" 0 0`,
			`link [10 20 60 30] "https://example.com/a"`,
			`link [10 40 60 50] #"later"`,
			`link [10 80 60 90] #"ivy-page-3-0-0"`,
		},
		{
			`bookmark 2 "Deep" 20 30 false`,
			`anchor "later" 30 40`,
			// the bounds of the quad points
			`link [15 30 55 59] #"start"`,
		},
		{
			// a level 2 after a level 3 is a sibling of it, and an entry
			// without children reads as closed
			`bookmark 2 "Part two" 0 100 false`,
			`anchor "ivy-page-3-0-0" 0 0`,
		},
	}
	for i := range want {
		if i >= len(got) || !slices.Equal(got[i], want[i]) {
			t.Errorf("page %d: expected\n%s\ngot\n%s", i, strings.Join(want[i], "\n"), strings.Join(got[min(i, len(got)-1)], "\n"))
		}
	}
}

func TestLinksOff(t *testing.T) {
	data := encodeLinks(t, WithLinks(false), WithOutline(false))
	for _, s := range []string{"/Annots", "/Outlines"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("expected no %s in the file", s)
		}
	}
	// the anchors stay named destinations
	if !bytes.Contains(data, []byte("(later)")) {
		t.Error("expected the anchor later in the file")
	}
}

func TestLinksPages(t *testing.T) {
	// Without the page of "later", the link to it is dropped, and the
	// outline holds the bookmarks of the pages of the file, in their order.
	var buf bytes.Buffer
	if err := Encode(t.Context(), &buf, &document{pages: linkPages()}, WithPages(2, 0)); err != nil {
		t.Fatal(err)
	}
	got := readMarks(t, buf.Bytes())
	want := [][]string{
		{`bookmark 1 "Part two" 0 100 false`, `anchor "ivy-page-3-0-0" 0 0`},
		{`bookmark 1 "Part one" 0 0 false`, `anchor "start" 0 0`, `link [10 20 60 30] "https://example.com/a"`, `link [10 80 60 90] #"ivy-page-3-0-0"`},
	}
	for i := range want {
		if i >= len(got) || !slices.Equal(got[i], want[i]) {
			t.Errorf("page %d: expected\n%s\ngot\n%q", i, strings.Join(want[i], "\n"), got)
		}
	}
}

func TestLinksEncodePages(t *testing.T) {
	// EncodePages writes no outline, and keeps the links.
	var buf bytes.Buffer
	pages := func(yield func(ivy.Page, error) bool) {
		for _, p := range linkPages() {
			if !yield(p, nil) {
				return
			}
		}
	}
	if err := EncodePages(t.Context(), &buf, pages); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if bytes.Contains(data, []byte("/Outlines")) {
		t.Error("expected no outline")
	}
	if !bytes.Contains(data, []byte("(later)")) {
		t.Error("expected the anchor later")
	}
}

func TestOutline(t *testing.T) {
	// make_page_bookmark_tree of WeasyPrint: a skipped level does not make
	// a deeper entry, and an entry past MaxDepth is left out.
	e := newEncoder(newOptions([]Option{WithConfig(ivy.Config{Limits: ivy.Limits{MaxDepth: 2}})}), nil)
	for i, level := range []int{1, 3, 4, 2, 1, 6, 1} {
		e.bookmarks = append(e.bookmarks, bookmark{level: level, label: string(rune('a' + i))})
	}
	if _, ok := e.outline(); !ok {
		t.Fatal("expected an outline")
	}
	// the tree, written as labels and parentheses
	var tree func(nodes []*outlineNode) string
	tree = func(nodes []*outlineNode) string {
		var b strings.Builder
		for _, n := range nodes {
			b.WriteString(n.label)
			if len(n.children) > 0 {
				b.WriteString("(" + tree(n.children) + ")")
			}
		}
		return b.String()
	}
	if got := tree(e.outlineTree()); got != "a(bd)e(f)g" {
		t.Errorf("expected a(bd)e(f)g, got %s", got)
	}
}

func TestLinksLimit(t *testing.T) {
	var buf bytes.Buffer
	cfg := ivy.Config{Limits: ivy.Limits{MaxObjects: 3}}
	if err := Encode(t.Context(), &buf, &document{pages: linkPages()}, WithConfig(cfg)); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, lines := range readMarks(t, buf.Bytes()) {
		n += len(lines)
	}
	if n > 3 {
		t.Errorf("expected at most 3 marks, got %d", n)
	}
}

func TestLinksMask(t *testing.T) {
	// The links of the content of a soft mask are not links of the page,
	// and the links of a group of the raster fallback stay links.
	page := &scenePage{w: 100, h: 100, draw: func(d display.Device) {
		r, _ := okage.RectFromXYWH(10, 10, 20, 20)
		d.BeginGroup(display.Group{Opacity: 1, Mask: &display.Mask{Type: okage.MaskTypeAlpha, Content: func(d display.Device) {
			d.FillPath(rect(0, 0, 100, 100), okage.FillRuleWinding, solid(0, 0, 0, 255))
			display.LinkURI(d, r, "https://example.com/mask")
		}}})
		d.EndGroup()
		d.BeginGroup(display.Group{Opacity: 1, Blend: okage.BlendModeDestinationOut})
		d.Save()
		d.Concat(okage.TransformFromTranslate(5, 5))
		d.FillPath(rect(0, 0, 50, 50), okage.FillRuleWinding, solid(200, 0, 0, 255))
		display.LinkURI(d, r, "https://example.com/group")
		d.Restore()
		d.EndGroup()
	}}
	var buf bytes.Buffer
	if err := Encode(t.Context(), &buf, &document{pages: []ivy.Page{page}}); err != nil {
		t.Fatal(err)
	}
	got := readMarks(t, buf.Bytes())
	want := []string{`link [15 15 35 35] "https://example.com/group"`}
	if len(got) != 1 || !slices.Equal(got[0], want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// countPage counts the drawings of a page.
type countPage struct {
	*scenePage
	draws int
}

func (p *countPage) Draw(ctx context.Context, d display.Device, res float32) error {
	p.draws++
	return p.scenePage.Draw(ctx, d, res)
}

func TestLinksDraws(t *testing.T) {
	// A link to an anchor of a later page, or of an earlier page, draws
	// the pages once (okage D104).
	for _, test := range []struct {
		name  string
		pages []int
		draws int
	}{
		{"forward", []int{0, 1, 2}, 1},
		{"backward", []int{1, 2}, 1},
	} {
		pages := linkPages()
		counted := make([]*countPage, 0, len(test.pages))
		doc := &document{}
		for _, n := range test.pages {
			c := &countPage{scenePage: pages[n].(*scenePage)}
			counted = append(counted, c)
			doc.pages = append(doc.pages, c)
		}
		var buf bytes.Buffer
		if err := Encode(t.Context(), &buf, doc); err != nil {
			t.Fatal(err)
		}
		if counted[0].draws != test.draws {
			t.Errorf("%s: expected %d drawings, got %d", test.name, test.draws, counted[0].draws)
		}
	}
}
