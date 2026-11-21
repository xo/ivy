package xps

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/okage"
)

// linkFile builds an XPS package of two pages, of 96 by 96 units, with
// links, link targets and an outline. docs is the number of its
// FixedDocuments, each of the two pages.
func linkFile(docs int) []byte {
	const ns = `xmlns="http://schemas.microsoft.com/xps/2005/06"`
	page1 := `<FixedPage ` + ns + ` Width="96" Height="96">` +
		`<Path Data="M 8,8 H 40 V 16 H 8 Z" Fill="#000000" NavigateUri="https://example.com/a"/>` +
		`<Canvas RenderTransform="1,0,0,1,0,20" NavigateUri="2.fpage#second">` +
		`<Path Data="M 8,8 H 20 V 12 H 8 Z" Fill="#000000"/><Path Data="M 30,8 H 40 V 30 H 30 Z" Fill="#000000"/></Canvas>` +
		`<Glyphs FontUri="/font.ttf" FontRenderingEmSize="10" OriginX="8" OriginY="80" UnicodeString="II" ` +
		`Fill="#000000" NavigateUri="2.fpage"/>` +
		`<Path Name="first" Data="M 50,50 H 60 V 60 H 50 Z" Fill="#000000"/>` +
		`</FixedPage>`
	page2 := `<FixedPage ` + ns + ` Width="96" Height="96">` +
		`<Path Name="second" Data="M 10,40 H 60 V 50 H 10 Z" Fill="#000000" NavigateUri="#first"/>` +
		`<Path Name="unlisted" Data="M 10,60 H 60 V 70 H 10 Z" Fill="#000000" NavigateUri="#nowhere"/>` +
		`</FixedPage>`
	structure := `<DocumentStructure xmlns="http://schemas.microsoft.com/xps/2005/06/documentstructure">` +
		`<DocumentStructure.Outline><DocumentOutline>` +
		`<OutlineEntry OutlineLevel="1" OutlineTarget="../doc.fdoc#first" Description="One"/>` +
		`<OutlineEntry OutlineLevel="2" OutlineTarget="../doc.fdoc#second" Description="One.Two"/>` +
		`<OutlineEntry OutlineLevel="1" OutlineTarget="../Pages/2.fpage" Description="Page two"/>` +
		`<OutlineEntry OutlineLevel="2" OutlineTarget="../doc.fdoc#missing" Description="Gone"/>` +
		`</DocumentOutline></DocumentStructure.Outline></DocumentStructure>`
	var seq strings.Builder
	seq.WriteString(`<FixedDocumentSequence ` + ns + `>`)
	entries := make([]fixture.Entry, 0, 3+5*docs)
	entries = append(entries,
		fixture.Entry{Name: "_rels/.rels", Data: []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Type="http://schemas.microsoft.com/xps/2005/06/fixedrepresentation" Target="/seq.fdseq" Id="R1"/></Relationships>`)},
		fixture.Entry{Name: "font.ttf", Data: goregular.TTF},
	)
	for i := range docs {
		dir := "Documents/" + string(rune('1'+i))
		seq.WriteString(`<DocumentReference Source="` + dir + `/doc.fdoc"/>`)
		entries = append(entries,
			fixture.Entry{Name: dir + "/doc.fdoc", Data: []byte(`<FixedDocument ` + ns + `>` +
				`<PageContent Source="Pages/1.fpage"><PageContent.LinkTargets><LinkTarget Name="first"/></PageContent.LinkTargets></PageContent>` +
				`<PageContent Source="Pages/2.fpage"><PageContent.LinkTargets><LinkTarget Name="second"/></PageContent.LinkTargets></PageContent>` +
				`</FixedDocument>`)},
			fixture.Entry{Name: dir + "/_rels/doc.fdoc.rels", Data: []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
				`<Relationship Type="http://schemas.microsoft.com/xps/2005/06/documentstructure" Target="Structure/doc.struct" Id="R1"/></Relationships>`)},
			fixture.Entry{Name: dir + "/Structure/doc.struct", Data: []byte(structure)},
			fixture.Entry{Name: dir + "/Pages/1.fpage", Data: []byte(page1)},
			fixture.Entry{Name: dir + "/Pages/2.fpage", Data: []byte(page2)},
		)
	}
	seq.WriteString(`</FixedDocumentSequence>`)
	entries = append(entries, fixture.Entry{Name: "seq.fdseq", Data: []byte(seq.String())})
	return fixture.Zip(entries...)
}

// linkLines draws page n of buf onto a list, and returns the lines of the
// links, the anchors and the bookmarks.
func linkLines(t *testing.T, buf []byte, n int) []string {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	l := &display.List{}
	if err := p.(display.Drawer).Draw(t.Context(), l, 1); err != nil {
		t.Fatal(err)
	}
	var out []string
	for line := range strings.SplitSeq(l.String(), "\n") {
		if strings.HasPrefix(line, "link ") || strings.HasPrefix(line, "anchor ") || strings.HasPrefix(line, "bookmark ") {
			out = append(out, line)
		}
	}
	return out
}

func TestLinks(t *testing.T) {
	buf := linkFile(1)
	got := strings.Join(linkLines(t, buf, 0), "\n")
	want := strings.Join([]string{
		`anchor "ivy-page-1-0-0" 0 0`,
		`link [8 8 40 16] "https://example.com/a"`,
		`link [8 8 40 30] #"second"`,
		`link [9 73 15 80] #"ivy-page-2-0-0"`,
		`anchor "first" 50 50`,
		`bookmark 1 "One" 50 50 true`,
	}, "\n")
	if got != want {
		t.Errorf("page 1: expected:\n%s\ngot:\n%s", want, got)
	}
	got = strings.Join(linkLines(t, buf, 1), "\n")
	want = strings.Join([]string{
		`anchor "ivy-page-2-0-0" 0 0`,
		`anchor "second" 10 40`,
		`link [10 40 60 50] #"first"`,
		`bookmark 2 "One.Two" 10 40 true`,
		`bookmark 1 "Page two" 0 0 true`,
	}, "\n")
	if got != want {
		t.Errorf("page 2: expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestLinksOfDocuments(t *testing.T) {
	buf := linkFile(2)
	got := strings.Join(linkLines(t, buf, 3), "\n")
	want := strings.Join([]string{
		`anchor "ivy-page-4-0-0" 0 0`,
		`anchor "2#second" 10 40`,
		`link [10 40 60 50] #"2#first"`,
		`bookmark 2 "One.Two" 10 40 true`,
		`bookmark 1 "Page two" 0 0 true`,
	}, "\n")
	if got != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestLinksNotRendered(t *testing.T) {
	// a device that takes no link gets no command of a link, and the walk
	// reads no index
	d, err := Open(t.Context(), bytes.NewReader(linkFile(1)))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	dst, _ := okage.NewPixmap(96, 96)
	if err := p.Render(t.Context(), dst, okage.IdentityTransform()); err != nil {
		t.Fatal(err)
	}
	if d.links != nil {
		t.Error("expected no index of the links after Render")
	}
}

func TestLinkTarget(t *testing.T) {
	ix := &linkIndex{
		names:    []map[string]int{{"a": 1}, {"a": 3, "b": 2}},
		pageDocs: []int{0, 0, 1, 1},
		partPage: map[string]int{"/d1/p1.fpage": 0, "/d1/p2.fpage": 1, "/d2/p1.fpage": 2, "/d2/p2.fpage": 3},
		docPart:  map[string]int{"/d1/doc.fdoc": 0, "/d2/doc.fdoc": 1},
		multi:    true,
	}
	tests := []struct {
		base, ref string
		doc       int
		page      int
		name      string
		ok        bool
	}{
		{"/d1/p1.fpage", "#a", 0, 1, "a", true},
		{"/d1/p1.fpage", "#b", 0, 0, "", false},
		{"/d1/p1.fpage", "p2.fpage", 0, 1, "", true},
		{"/d1/p1.fpage", "../d2/p2.fpage#a", 0, 3, "a", true},
		{"/d1/p1.fpage", "../d2/doc.fdoc", 0, 2, "", true},
		{"/d1/p1.fpage", "../d2/doc.fdoc#b", 0, 2, "b", true},
		{"/d1/p1.fpage", "/seq.fdseq#b", 0, 2, "b", true},
		{"/d1/p1.fpage", "/seq.fdseq", 0, 0, "", true},
		{"/d1/p1.fpage", "missing.fpage", 0, 0, "", false},
		{"/d1/p1.fpage", "", 0, 0, "", false},
	}
	for _, test := range tests {
		page, name, ok := ix.target(test.base, test.ref, test.doc)
		if page != test.page || name != test.name || ok != test.ok {
			t.Errorf("%s from %s: expected %d %q %v, got: %d %q %v", test.ref, test.base, test.page, test.name, test.ok, page, name, ok)
		}
	}
	if got := ix.anchorName(1, "a"); got != "2#a" {
		t.Errorf("expected the name of the second document, got: %q", got)
	}
}
