// This file is not a port. It tests the galley of D57: the pages that take
// their lines from the galley are the same as the pages without it.

package layout

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// galleyWords are the words of the paragraphs of the tests.
var galleyWords = strings.Fields(`lorem ipsum dolor sit amet consectetur adipiscing elit
	sed do eiusmod tempor incididunt ut labore et dolore magna aliqua enim ad minim
	veniam quis nostrud exercitation ullamco laboris nisi aliquip ex ea commodo
	consequat duis aute irure in reprehenderit voluptate velit esse cillum fugiat`)

// galleyText returns a text of n words, which starts at the word i.
func galleyText(i, n int) string {
	words := make([]string, n)
	for k := range words {
		words[k] = galleyWords[(i*7+k*3)%len(galleyWords)]
	}
	return strings.Join(words, " ")
}

// galleyParagraphs returns n paragraphs, with the HTML of extra(i) in the
// paragraph i.
func galleyParagraphs(n int, extra func(i int) string) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "<p>%s %s %s</p>\n", galleyText(i, 8+i%13), extra(i), galleyText(i+1, 6+i%17))
	}
	return b.String()
}

// none adds nothing to a paragraph.
func none(int) string { return "" }

// galleyPagesOf lays out the document of the HTML source in a book with the
// galley options o, and returns a dump of the boxes of its pages and the
// number of the lines that the pages took from the galley.
func galleyPagesOf(t *testing.T, source string, o GalleyOptions) (string, int) {
	t.Helper()
	defer SetGalleyOptions(o)()
	fsys := fstest.MapFS{"a.html": {Data: []byte(source)}}
	font, err := os.ReadFile("testdata/weasyprint.otf")
	if err != nil {
		t.Fatal(err)
	}
	fsys["weasyprint.otf"] = &fstest.MapFile{Data: font}
	b := NewBook(Source{FS: fsys, Documents: []string{"a.html"}}, BookOptions{})
	ctx := context.Background()
	n, err := b.PageCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	taken := 0
	if doc := b.docs[0].doc; doc != nil {
		taken = doc.lc.lines.taken
	}
	var dump strings.Builder
	for i := range n {
		p, err := b.Page(ctx, i)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&dump, "page %d\n", i)
		p.(*bookPage).box.descendants(true, func(box *Box) bool {
			fmt.Fprintf(&dump, "%v %s %v %v %v %v %q\n", box.Kind, box.ElementTag,
				box.PositionX, box.PositionY, box.Width, box.Height, box.Text)
			return true
		})
	}
	return dump.String(), taken
}

// checkGalley lays out the HTML source without the galley and with a
// galley of one block in each part, and fails the test when the pages are
// not the same. When take is set, the pages must take some lines from the
// galley.
func checkGalley(t *testing.T, source string, take bool) {
	t.Helper()
	want, _ := galleyPagesOf(t, source, GalleyOptions{Off: true})
	got, taken := galleyPagesOf(t, source, GalleyOptions{OneBlock: true, Wait: true})
	if got != want {
		wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
		for i := range min(len(wl), len(gl)) {
			if wl[i] != gl[i] {
				t.Fatalf("pages differ at line %d of %d:\nwant %s\n got %s", i, len(wl), wl[i], gl[i])
			}
		}
		t.Fatalf("pages differ: %d lines, want %d", len(gl), len(wl))
	}
	t.Logf("%d lines from the galley", taken)
	if take && taken == 0 {
		t.Error("the pages took no line from the galley")
	}
}

// galleyPage is the style of the pages of the tests.
const galleyPage = `@page { size: 220px 200px; margin: 10px } body { margin: 0; font-size: 10px; line-height: 1.2 }`

// galleyDocument returns an HTML document with the style sheet style and
// the body body.
func galleyDocument(style, body string) string {
	return "<html><head><style>" + galleyPage + style + "</style></head><body>" + body + "</body></html>"
}

func TestGalley(t *testing.T) {
	tests := []struct {
		name  string
		style string
		body  string
		take  bool
	}{
		{"plain", "", galleyParagraphs(60, none), true},
		{"float", ".f { float: left; width: 40px; height: 70px }",
			galleyParagraphs(60, func(i int) string {
				if i%6 == 0 {
					return `<span class="f"></span>`
				}
				return ""
			}), true},
		{"float block", ".f { float: right; width: 60px; height: 150px }",
			galleyParagraphs(60, func(i int) string {
				if i%9 == 0 {
					return `</p><div class="f"></div><p>`
				}
				return ""
			}), true},
		{"drop cap", "p:nth-child(5n)::first-letter { float: left; font-size: 30px; line-height: 1 }",
			galleyParagraphs(60, none), true},
		{"positioned", ".a { position: absolute; top: 5px; left: 5px } .r { position: relative; top: 2px }",
			galleyParagraphs(60, func(i int) string {
				switch i % 7 {
				case 0:
					return `<span class="a">abs</span>`
				case 3:
					return `<span class="r">rel</span>`
				}
				return ""
			}), true},
		{"running", "h2 { position: running(head); font-size: 10px } @page { @top-center { content: element(head) } }",
			galleyParagraphs(60, func(i int) string {
				if i%8 == 0 {
					return fmt.Sprintf(`</p><h2>Head %d</h2><p>`, i)
				}
				return ""
			}), true},
		{"footnote", ".n { float: footnote }",
			galleyParagraphs(60, func(i int) string {
				if i%5 == 0 {
					return fmt.Sprintf(`<span class="n">Note %d %s</span>`, i, galleyText(i, 9))
				}
				return ""
			}), true},
		{"first line", "p::first-line { font-weight: bold; color: red }", galleyParagraphs(60, none), true},
		{"first letter", "p::first-letter { font-size: 18px }", galleyParagraphs(60, none), true},
		{"text indent", "p { text-indent: 25px; margin: 0 }", galleyParagraphs(60, none), true},
		{"tabs and leaders", "p { -ivy-tab-stops: 120px right '.' } p:nth-child(3n)::after { content: leader('.') ' 9' }",
			galleyParagraphs(60, func(i int) string {
				if i%4 == 0 {
					return "a\tb"
				}
				return ""
			}), true},
		{"table", "td { border: 1px solid; vertical-align: top }",
			"<table>" + strings.Repeat("<tr><td>"+galleyText(3, 30)+"</td><td>"+galleyText(5, 20)+"</td></tr>", 25) + "</table>" +
				galleyParagraphs(20, none), true},
		{"list", "li { margin: 2px 0 }",
			"<ol>" + strings.Repeat("<li>"+galleyText(2, 25)+"</li>", 40) + "</ol>" +
				"<ul>" + strings.Repeat("<li>"+galleyText(4, 15)+"</li>", 30) + "</ul>", true},
		{"widows and orphans", "p { widows: 4; orphans: 3 }", galleyParagraphs(60, none), true},
		{"page overflow", "body { -ivy-page-overflow: descent } p { margin: 0 }",
			galleyParagraphs(60, func(i int) string {
				if i%3 == 0 {
					return "</p><p>&#160;</p><p></p><p>"
				}
				return ""
			}), true},
		{"margins", "p { margin: 12px 0 } h2 { margin: 30px 0 10px; font-size: 14px } div { margin-top: 20px }",
			galleyParagraphs(60, func(i int) string {
				if i%7 == 0 {
					return fmt.Sprintf(`</p><div><h2>Title %d</h2></div><p>`, i)
				}
				return ""
			}), true},
		{"named page", "@page wide { size: 320px 200px } .w { page: wide }",
			galleyParagraphs(30, none) + `<div class="w">` + galleyParagraphs(20, none) + `</div>` + galleyParagraphs(20, none), true},
		{"left page", "@page :left { margin-left: 40px }", galleyParagraphs(60, none), true},
		{"counter page", "p:nth-child(4n)::before { content: counter(page) ' ' }", galleyParagraphs(60, none), true},
		{"counter pages", "p:nth-child(9n)::before { content: counter(pages) ' ' } @page { @bottom-center { content: counter(page) '/' counter(pages) } }",
			galleyParagraphs(60, none), true},
		{"string set", "h2 { string-set: title content(); font-size: 12px } @page { @top-left { content: string(title) } }",
			galleyParagraphs(60, func(i int) string {
				if i%6 == 0 {
					return fmt.Sprintf(`</p><h2>Part %d</h2><p>`, i)
				}
				return ""
			}), true},
		{"font face", "@font-face { font-family: wp; src: url(weasyprint.otf) } p:nth-child(2n) { font-family: wp }",
			galleyParagraphs(60, none), true},
		{"columns", ".c { columns: 2; column-gap: 10px }",
			galleyParagraphs(10, none) + `<div class="c">` + galleyParagraphs(20, none) + `</div>` + galleyParagraphs(10, none), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkGalley(t, galleyDocument(tt.style, tt.body), tt.take)
		})
	}
}

// TestGalleyParts tests the cut of a document into parts of about the same
// length of text.
func TestGalleyParts(t *testing.T) {
	var body strings.Builder
	for i := range 400 {
		fmt.Fprintf(&body, "<p>%s</p>", galleyText(i, 100))
	}
	source := galleyDocument("", "<div>"+body.String()+"</div>")
	fsys := fstest.MapFS{"a.html": {Data: []byte(source)}}
	b := NewBook(Source{FS: fsys, Documents: []string{"a.html"}}, BookOptions{})
	doc, err := b.build(b.docs[0])
	if err != nil {
		t.Fatal(err)
	}
	plan := doc.galleyPlan
	if plan == nil {
		t.Fatal("no plan of the galley")
	}
	parts := plan.parts(GalleyOptions{})
	text := treeText(doc.root)
	if want := min(text/galleyPartText, galleyWorkers); len(parts) != want {
		t.Fatalf("%d parts, want %d", len(parts), want)
	}
	for i, p := range parts {
		n := 0
		for j := p.lo; j < p.hi; j++ {
			n += len(plan.tree.boxes[j].Text)
		}
		if n < text/len(parts)-2*len(galleyText(0, 100)) {
			t.Errorf("part %d holds %d bytes of text of %d", i, n, text)
		}
		if i > 0 && p.lo != parts[i-1].hi {
			t.Errorf("part %d starts at %d, want %d", i, p.lo, parts[i-1].hi)
		}
	}
}

// TestGalleySmall tests that a small document has no galley.
func TestGalleySmall(t *testing.T) {
	fsys := fstest.MapFS{"a.html": {Data: []byte(galleyDocument("", galleyParagraphs(20, none)))}}
	b := NewBook(Source{FS: fsys, Documents: []string{"a.html"}}, BookOptions{})
	doc, err := b.build(b.docs[0])
	if err != nil {
		t.Fatal(err)
	}
	if doc.galleyPlan != nil {
		t.Error("a small document has a plan of the galley")
	}
}

// TestGalleyCanceled tests that the cancellation of the context of the
// count stops the galley, and that the pages are the same after it.
func TestGalleyCanceled(t *testing.T) {
	defer SetGalleyOptions(GalleyOptions{OneBlock: true})()
	source := galleyDocument("", galleyParagraphs(80, none))
	fsys := fstest.MapFS{"a.html": {Data: []byte(source)}}
	b := NewBook(Source{FS: fsys, Documents: []string{"a.html"}}, BookOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.PageCount(ctx); err == nil {
		t.Fatal("PageCount with a canceled context gives no error")
	}
	n, err := b.PageCount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want, _ := galleyPagesOf(t, source, GalleyOptions{Off: true})
	if got := strings.Count(want, "page "); got != n {
		t.Errorf("%d pages, want %d", n, got)
	}
}
