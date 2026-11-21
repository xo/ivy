package epub

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/layout"
)

// tocOPF returns a package document with the documents of tocBook, and the
// item of the table of contents item, which the spine names with toc when
// ncx is set.
func tocOPF(item string, ncx bool) string {
	spine := `<spine>`
	if ncx {
		spine = `<spine toc="toc">`
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">` + uuid + `</dc:identifier>
    <dc:title>Contents</dc:title>
  </metadata>
  <manifest>
    ` + item + `
    <item id="ch1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch2" href="text/ch%202.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  ` + spine + `
    <itemref idref="ch1"/>
    <itemref idref="ch2"/>
  </spine>
</package>`
}

// tocBook returns the entries of a book whose table of contents is the file
// toc.xhtml or toc.ncx, with data.
func tocBook(name, item string, ncx bool, data string) []fixture.Entry {
	return []fixture.Entry{
		{Name: "mimetype", Data: []byte("application/epub+zip"), Store: true},
		{Name: "META-INF/container.xml", Data: []byte(containerXML("OEBPS/content.opf"))},
		{Name: "OEBPS/content.opf", Data: []byte(tocOPF(item, ncx))},
		{Name: "OEBPS/" + name, Data: []byte(data)},
		{Name: "OEBPS/text/ch1.xhtml", Data: xhtml(`<h1 id="a">One</h1><h2 id="b c">Two</h2>`)},
		{Name: "OEBPS/text/ch 2.xhtml", Data: xhtml(`<h1>Three</h1>`)},
	}
}

// navItem is the item of a navigation document of EPUB 3.
const navItem = `<item id="nav" href="nav/toc.xhtml" media-type="application/xhtml+xml" properties="nav"/>`

// ncxItem is the item of an NCX file of EPUB 2.
const ncxItem = `<item id="toc" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`

func TestTOC(t *testing.T) {
	nav := xhtml(`<nav epub:type="landmarks"><ol><li><a href="../text/ch1.xhtml">Landmark</a></li></ol></nav>
<nav epub:type="toc"><h1>Contents</h1><ol>
<li><a href="../text/ch1.xhtml#a">One
  </a><ol><li><a href="../text/ch1.xhtml#b%20c">Two</a></li><li><span>Heading</span><ol><li><a href="../text/ch%202.xhtml">Deep</a></li></ol></li></ol></li>
<li><a href="../text/ch%202.xhtml">Three</a></li>
<li><a href="https://example.com/">Away</a></li>
<li><a href="../text/ch1.xhtml"> </a></li>
</ol></nav>`)
	ncx := `<?xml version="1.0"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><navMap>
<navPoint id="p1"><navLabel><text>One</text></navLabel><content src="text/ch1.xhtml#a"/>
  <navPoint id="p2"><navLabel><text>Two</text></navLabel><content src="text/ch1.xhtml#b%20c"/></navPoint>
</navPoint>
<navPoint id="p3"><navLabel><text>Three</text></navLabel><content src="text/ch%202.xhtml"/></navPoint>
</navMap></ncx>`
	for name, tt := range map[string]struct {
		entries []fixture.Entry
		want    []layout.TOCEntry
	}{
		"nav": {tocBook("nav/toc.xhtml", navItem, false, string(nav)), []layout.TOCEntry{
			{Label: "One", Level: 1, Target: "OEBPS/text/ch1.xhtml#a"},
			{Label: "Two", Level: 2, Target: "OEBPS/text/ch1.xhtml#b c"},
			{Label: "Deep", Level: 3, Target: "OEBPS/text/ch 2.xhtml"},
			{Label: "Three", Level: 1, Target: "OEBPS/text/ch 2.xhtml"},
		}},
		"ncx": {tocBook("toc.ncx", ncxItem, true, ncx), []layout.TOCEntry{
			{Label: "One", Level: 1, Target: "OEBPS/text/ch1.xhtml#a"},
			{Label: "Two", Level: 2, Target: "OEBPS/text/ch1.xhtml#b c"},
			{Label: "Three", Level: 1, Target: "OEBPS/text/ch 2.xhtml"},
		}},
		"ncx without spine": {tocBook("toc.ncx", ncxItem, false, ncx), []layout.TOCEntry{
			{Label: "One", Level: 1, Target: "OEBPS/text/ch1.xhtml#a"},
			{Label: "Two", Level: 2, Target: "OEBPS/text/ch1.xhtml#b c"},
			{Label: "Three", Level: 1, Target: "OEBPS/text/ch 2.xhtml"},
		}},
		"broken nav":  {tocBook("nav/toc.xhtml", navItem, false, "<html"), nil},
		"missing nav": {tocBook("other.xhtml", navItem, false, ""), nil},
		"none":        {tocBook("other.xhtml", "", false, ""), nil},
	} {
		t.Run(name, func(t *testing.T) {
			d := mustOpen(t, tt.entries)
			src, err := d.source()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(src.TOC, tt.want) {
				t.Errorf("expected:\n%+v\ngot:\n%+v", tt.want, src.TOC)
			}
		})
	}
}

func TestTOCLimits(t *testing.T) {
	// a list nested deeper than MaxDepth
	var b strings.Builder
	for i := range 20 {
		fmt.Fprintf(&b, `<ol><li><a href="../text/ch1.xhtml#a">L%d</a>`, i+1)
	}
	b.WriteString(strings.Repeat(`</li></ol>`, 20))
	nav := xhtml(`<nav epub:type="toc">` + b.String() + `</nav>`)
	lim := ivy.Limits{MaxDepth: 5, MaxObjects: 1000}
	d := mustOpen(t, tocBook("nav/toc.xhtml", navItem, false, string(nav)), WithConfig(ivy.Config{Limits: lim}))
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	if len(src.TOC) != 5 || src.TOC[4].Level != 5 || src.TOC[4].Label != "L5" {
		t.Errorf("expected 5 levels of entries, got: %+v", src.TOC)
	}
}

func TestTOCImagePage(t *testing.T) {
	// an image of the spine is on a page of its own, which an entry of the
	// table of contents that targets the image targets
	entries := book3()
	entries[4].Data = xhtml(`<nav epub:type="toc"><ol><li><a href="images/pic.png">Picture</a></li></ol></nav>`)
	d := mustOpen(t, entries)
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	if len(src.TOC) != 1 || !slices.Contains(src.Documents, src.TOC[0].Target) || src.TOC[0].Target == "OEBPS/images/pic.png" {
		t.Errorf("expected the entry to target the page of the image, got: %+v in %q", src.TOC, src.Documents)
	}
}
