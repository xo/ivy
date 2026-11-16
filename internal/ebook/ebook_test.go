package ebook

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/zipdoc"
)

func TestText(t *testing.T) {
	for _, tt := range []struct {
		name string
		buf  []byte
		want string
	}{
		{"utf-8", []byte("caf\xc3\xa9"), "café"},
		{"utf-8 bom", []byte("\xef\xbb\xbfcaf\xc3\xa9"), "café"},
		{"utf-8 bom invalid", []byte("\xef\xbb\xbfa\xffb"), "a�b"},
		{"utf-16le bom", []byte{0xff, 0xfe, 'h', 0, 'i', 0, 0x3d, 0xd8, 0x00, 0xde}, "hi\U0001F600"},
		{"utf-16be bom", []byte{0xfe, 0xff, 0, 'h', 0, 'i'}, "hi"},
		{"utf-16le xml", []byte{'<', 0, 'a', 0, '/', 0, '>', 0}, "<a/>"},
		{"utf-16be xml", []byte{0, '<', 0, 'a', 0, '/', 0, '>'}, "<a/>"},
		{"utf-16 odd byte", []byte{0xff, 0xfe, 'h', 0, 'i'}, "h"},
		{"windows-1252", []byte("\x93caf\xe9\x94 \x80 \x81"), "“café” € \u0081"},
		{"empty", nil, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Text(tt.buf); got != tt.want {
				t.Errorf("expected %q, got: %q", tt.want, got)
			}
		})
	}
}

func TestParseXML(t *testing.T) {
	doc, err := ParseXML([]byte(`<?xml version="1.0"?>
<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">
<ncx><docTitle><text>A&nbsp;B &amp; C &bogus; &lt;</text></docTitle></ncx>`), ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := NodeText(doc.RootElement()), "A B & C &bogus; <"; got != want {
		t.Errorf("expected %q, got: %q", want, got)
	}
	_, err = ParseXML([]byte("<a><b/><b/><b/><b/></a>"), ivy.Limits{MaxObjects: 3})
	if !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
	for _, bad := range []string{"", "<a>", "not xml", "<a></b>"} {
		if _, err := ParseXML([]byte(bad), ivy.Limits{}); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%q: expected ivy.ErrFormat, got: %v", bad, err)
		}
	}
}

// opf3 is a package document of EPUB 3, with refinements.
const opf3 = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="isbn">urn:isbn:9780000000000</dc:identifier>
    <dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789abc</dc:identifier>
    <dc:title id="sub">A Subtitle</dc:title>
    <dc:title id="main">The   Main
      Title</dc:title>
    <meta refines="#main" property="title-type">main</meta>
    <meta refines="#sub" property="title-type">subtitle</meta>
    <dc:creator id="c1">Ann Author</dc:creator>
    <meta refines="#c1" property="role" scheme="marc:relators">aut</meta>
    <dc:creator id="c2">Ed Editor</dc:creator>
    <meta refines="#c2" property="role" scheme="marc:relators">edt</meta>
    <dc:creator>Bo Author</dc:creator>
    <dc:subject>Fiction</dc:subject>
    <dc:subject>Sea</dc:subject>
    <dc:language>en-US</dc:language>
    <dc:publisher>A Press</dc:publisher>
    <dc:date>2020-05-12</dc:date>
    <meta property="dcterms:modified">2021-01-02T03:04:05Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="c1" href="text/chapter%201.xhtml#top" media-type="application/xhtml+xml"/>
    <item id="img" href="images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
    <item id="out" href="../../etc/passwd" media-type="text/plain"/>
    <item id="web" href="https://example.com/a.css" media-type="text/css"/>
  </manifest>
  <spine>
    <itemref idref="c1"/>
    <itemref idref="nav" linear="no"/>
  </spine>
</package>`

func TestParseOPF3(t *testing.T) {
	p, err := ParseOPF([]byte(opf3), "OEBPS", ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	m := p.Metadata
	if m.Title != "The Main Title" {
		t.Errorf("expected the main title, got: %q", m.Title)
	}
	if !slices.Equal(m.Creators, []string{"Ann Author", "Bo Author"}) {
		t.Errorf("expected the authors without the editor, got: %q", m.Creators)
	}
	if m.Identifier != "urn:uuid:12345678-1234-1234-1234-123456789abc" || len(m.Identifiers) != 2 || m.Identifiers[0] != m.Identifier {
		t.Errorf("expected the unique identifier first, got: %q %q", m.Identifier, m.Identifiers)
	}
	if m.Publisher != "A Press" {
		t.Errorf("expected the publisher, got: %q", m.Publisher)
	}
	paths := make([]string, 0, len(p.Manifest))
	for _, it := range p.Manifest {
		paths = append(paths, it.Path)
	}
	if want := []string{"OEBPS/nav.xhtml", "OEBPS/text/chapter 1.xhtml", "OEBPS/images/cover.jpg"}; !slices.Equal(paths, want) {
		t.Errorf("expected %q, got: %q", want, paths)
	}
	if it, ok := p.Item("img"); !ok || !it.HasProperty("cover-image") || !it.IsImage() || it.IsDocument() {
		t.Errorf("expected the cover image item, got: %+v", it)
	}
	if it, ok := p.ItemByPath("OEBPS/text/chapter 1.xhtml"); !ok || it.ID != "c1" || !it.IsDocument() {
		t.Errorf("expected the chapter by its path, got: %+v", it)
	}
	if want := []ItemRef{{"c1", true}, {"nav", false}}; !slices.Equal(p.Spine, want) {
		t.Errorf("expected %v, got: %v", want, p.Spine)
	}
	if got := p.Covers(); !slices.Equal(got, []string{"OEBPS/images/cover.jpg"}) {
		t.Errorf("expected the cover, got: %q", got)
	}
	info := p.Info()
	want := ivy.Info{
		Title:    "The Main Title",
		Authors:  []string{"Ann Author", "Bo Author"},
		Subject:  "Fiction, Sea",
		Language: "en-US",
		Created:  time.Date(2020, 5, 12, 0, 0, 0, 0, time.UTC),
		Modified: time.Date(2021, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if info.Title != want.Title || !slices.Equal(info.Authors, want.Authors) || info.Subject != want.Subject ||
		info.Language != want.Language || !info.Created.Equal(want.Created) || !info.Modified.Equal(want.Modified) {
		t.Errorf("expected %+v, got: %+v", want, info)
	}
}

func TestParseOPF2(t *testing.T) {
	const opf2 = `<package xmlns="http://www.idpf.org/2007/opf" xmlns:opf="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="id">
  <metadata>
    <dc-metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
      <dc:title>Old Book</dc:title>
      <dc:creator opf:role="ill">Ilse Illustrator</dc:creator>
      <dc:date opf:event="modification">2011-02</dc:date>
      <dc:date opf:event="publication">1999</dc:date>
      <dc:identifier id="id">abc</dc:identifier>
    </dc-metadata>
    <meta name="cover" content="cover-img"/>
    <meta name="generator" content="a program"/>
  </metadata>
  <manifest>
    <item id="cover-img" href="cover.png" media-type="image/png"/>
    <item id="page" href="cover.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine toc="ncx"><itemref idref="page"/></spine>
  <guide><reference type="Cover" title="Cover" href="cover.xhtml#c"/></guide>
</package>`
	p, err := ParseOPF([]byte(opf2), ".", ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	info := p.Info()
	if !slices.Equal(info.Authors, []string{"Ilse Illustrator"}) {
		t.Errorf("expected the only creator, got: %q", info.Authors)
	}
	if info.Created.Year() != 1999 || info.Modified != time.Date(2011, 2, 1, 0, 0, 0, 0, time.UTC) {
		t.Errorf("expected the dates of the events, got: %v %v", info.Created, info.Modified)
	}
	if info.Producer != "a program" || p.Toc != "ncx" || p.Version != "2.0" {
		t.Errorf("expected the generator, the toc and the version, got: %q %q %q", info.Producer, p.Toc, p.Version)
	}
	if want := []Reference{{"cover", "Cover", "cover.xhtml"}}; !slices.Equal(p.Guide, want) {
		t.Errorf("expected %v, got: %v", want, p.Guide)
	}
	if got := p.Covers(); !slices.Equal(got, []string{"cover.png", "cover.xhtml"}) {
		t.Errorf("expected the meta cover, then the guide, got: %q", got)
	}
	for _, bad := range []string{"<html/>", "<package", ""} {
		if _, err := ParseOPF([]byte(bad), ".", ivy.Limits{}); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%q: expected ivy.ErrFormat, got: %v", bad, err)
		}
	}
}

func TestResolve(t *testing.T) {
	for _, tt := range []struct {
		dir, href, want string
		ok              bool
	}{
		{"OEBPS", "text/a.xhtml", "OEBPS/text/a.xhtml", true},
		{"OEBPS/text", "../images/a%20b.png", "OEBPS/images/a b.png", true},
		{"OEBPS", "a.xhtml#frag", "OEBPS/a.xhtml", true},
		{"OEBPS", "a.xhtml?q=1", "OEBPS/a.xhtml", true},
		{"OEBPS", "/root.css", "root.css", true},
		{".", `sub\b.png`, "sub/b.png", true},
		{"", " a.png ", "a.png", true},
		{"OEBPS", "../../x", "", false},
		{"OEBPS", "http://example.com/a.png", "", false},
		{"OEBPS", "data:image/png;base64,AAAA", "", false},
		{"OEBPS", "//host/a.png", "", false},
		{"OEBPS", "#frag", "", false},
		{"OEBPS", "", "", false},
		{"OEBPS", "..", "", false},
		{"", ".", "", false},
	} {
		got, ok := Resolve(tt.dir, tt.href)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Resolve(%q, %q): expected %q %v, got: %q %v", tt.dir, tt.href, tt.want, tt.ok, got, ok)
		}
	}
}

func TestParseDate(t *testing.T) {
	for s, want := range map[string]time.Time{
		"2010":                      time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
		"2010-05":                   time.Date(2010, 5, 1, 0, 0, 0, 0, time.UTC),
		" 2010-05-12 ":              time.Date(2010, 5, 12, 0, 0, 0, 0, time.UTC),
		"2010-05-12T10:00:00Z":      time.Date(2010, 5, 12, 10, 0, 0, 0, time.UTC),
		"2010-05-12T10:00:00+02:00": time.Date(2010, 5, 12, 8, 0, 0, 0, time.UTC),
		"2010-05-12T10:00":          time.Date(2010, 5, 12, 10, 0, 0, 0, time.UTC),
		"May 2010":                  {},
		"":                          {},
	} {
		if got := ParseDate(s); !got.Equal(want) {
			t.Errorf("ParseDate(%q): expected %v, got: %v", s, want, got)
		}
	}
}

func TestScanHead(t *testing.T) {
	const doc = `<!DOCTYPE html>
<!-- <title>not this</title> -->
<HTML LANG=fr>
<head>
<script>var s = "<title>nor this</title>";</script>
<TITLE> Le  Titre &amp; la
  suite </TITLE>
<meta name="Author" content="Jean Dupont">
<meta name=author content='Marie Curie'>
<meta name="keywords" content="a, b">
<meta name="generator" content="An Editor 1.0">
<meta charset=utf-8>
</head>
<body><meta name="author" content="Not In Head"><title>Body title</title></body>
</HTML>`
	h := ScanHead(doc)
	want := Head{
		Title:     "Le Titre & la suite",
		Authors:   []string{"Jean Dupont", "Marie Curie"},
		Subject:   "a, b",
		Language:  "fr",
		Generator: "An Editor 1.0",
	}
	if h.Title != want.Title || !slices.Equal(h.Authors, want.Authors) || h.Subject != want.Subject ||
		h.Language != want.Language || h.Generator != want.Generator {
		t.Errorf("expected %+v, got: %+v", want, h)
	}
	if h := ScanHead(`<html xml:lang="de"><title>x`); h.Language != "de" || h.Title != "x" {
		t.Errorf("expected the xml:lang and an unclosed title, got: %+v", h)
	}
}

func TestFirstImage(t *testing.T) {
	for doc, want := range map[string]string{
		`<html><body><p>text</p><img alt="x" src="images/cover.jpg"/></body></html>`:                           "images/cover.jpg",
		`<svg xmlns:xlink="http://www.w3.org/1999/xlink"><image width="600" xlink:href="../cover.png"/></svg>`: "../cover.png",
		`<svg:svg><svg:image href="c.gif"/></svg:svg>`:                                                         "c.gif",
		`<img src=""><img src=unquoted.png>`:                                                                   "unquoted.png",
		`<!-- <img src="no.png"> --><script>"<img src='no.png'>"</script>`:                                     "",
		`<img src="a&amp;b.png">`:                                                                              "a&b.png",
		`<p>no image</p>`:                                                                                      "",
		`<img src="broken`:                                                                                     "broken",
		`<<<<>>>=<a =b c== d='e' "f"/ g>`:                                                                      "",
	} {
		got, ok := FirstImage(doc)
		if got != want || ok != (want != "") {
			t.Errorf("FirstImage(%q): expected %q, got: %q %v", doc, want, got, ok)
		}
	}
}

func TestTagsEnds(t *testing.T) {
	// each of these ends in the middle of a construct, and the scan must
	// end without a panic
	for _, doc := range []string{"<", "<a", "<a b", "<a b=", "<a b='", "<!--", "<!", "</", "<?", "<title", "<a/", "<a =", "x<1"} {
		for tag := range Tags(doc) {
			_ = tag
		}
	}
	var names []string
	for tag := range Tags(`<A><b-c><d:e/><1x><f g>`) {
		names = append(names, tag.Name)
	}
	if want := []string{"a", "b-c", "d:e", "f"}; !slices.Equal(names, want) {
		t.Errorf("expected %q, got: %q", want, names)
	}
}

func TestCoverPreviews(t *testing.T) {
	opf := `<package version="2.0"><metadata/><manifest/><guide><reference type="cover" href="images/front.png"/></guide></package>`
	for name, tt := range map[string]struct {
		entries []fixture.Entry
		want    image.Point
	}{
		"guide": {[]fixture.Entry{
			{Name: "metadata.opf", Data: []byte(opf)},
			{Name: "cover.jpg", Data: fixture.JPEG(5, 5, color.White)},
			{Name: "images/front.png", Data: fixture.PNG(7, 3, color.White)},
		}, image.Pt(7, 3)},
		"name": {[]fixture.Entry{
			{Name: "cover.png", Data: fixture.PNG(4, 2, color.White)},
		}, image.Pt(4, 2)},
		"none": {[]fixture.Entry{{Name: "index.html", Data: []byte("<p>")}}, image.Point{}},
	} {
		t.Run(name, func(t *testing.T) {
			archive, err := zipdoc.Open(bytes.NewReader(fixture.Zip(tt.entries...)), ivy.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			var pkg *Package
			if f := archive.Find("metadata.opf"); f != nil {
				buf, _ := archive.Read(f)
				if pkg, err = ParseOPF(buf, ".", ivy.Limits{}); err != nil {
					t.Fatal(err)
				}
			}
			var got []ivy.Preview
			for p, err := range CoverPreviews(t.Context(), archive, pkg) {
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, p)
			}
			switch {
			case tt.want == image.Point{} && len(got) != 0:
				t.Errorf("expected no preview, got: %d", len(got))
			case tt.want != image.Point{} && (len(got) != 1 || got[0].Size != tt.want || got[0].Kind != ivy.Cover):
				t.Errorf("expected one cover of %v, got: %+v", tt.want, got)
			}
		})
	}
}

func TestRecover(t *testing.T) {
	err := func() (err error) {
		defer Recover("doing a thing", &err)
		panic("boom")
	}()
	if !errors.Is(err, ivy.ErrFormat) || !strings.Contains(err.Error(), "panic: boom") {
		t.Errorf("expected a panic as ivy.ErrFormat, got: %v", err)
	}
}

func TestStyleSheets(t *testing.T) {
	for _, s := range []string{BookStyle, PaperStyle} {
		if !strings.Contains(s, "@page") || !strings.Contains(s, "font-family") {
			t.Errorf("expected a page and a font, got: %s", s)
		}
	}
}

func TestLCID(t *testing.T) {
	for _, tt := range []struct {
		lcid     uint32
		language string
		charset  string
	}{
		{0x0409, "en-US", "windows-1252"},
		{0x0c09, "en", "windows-1252"},
		{0x0407, "de", "windows-1252"},
		{0x0419, "ru", "windows-1251"},
		{0x0411, "ja", "shift_jis"},
		{0x0804, "zh-CN", "gbk"},
		{0x0404, "zh-TW", "big5"},
		{0x0412, "ko", "euc-kr"},
		{0x0405, "cs", "windows-1250"},
		{0x041a, "hr", "windows-1250"},
		{0x081a, "sr-Latn", "windows-1250"},
		{0x0c1a, "sr-Cyrl", "windows-1251"},
		{0x0408, "el", "windows-1253"},
		{0x041f, "tr", "windows-1254"},
		{0x040d, "he", "windows-1255"},
		{0x0401, "ar", "windows-1256"},
		{0x0425, "et", "windows-1257"},
		{0x042a, "vi", "windows-1258"},
		{0x041e, "th", "windows-874"},
		{0, "", "windows-1252"},
		{0x03ff, "", "windows-1252"},
	} {
		if got := LCIDLanguage(tt.lcid); got != tt.language {
			t.Errorf("LCIDLanguage(%#x): expected %q, got: %q", tt.lcid, tt.language, got)
		}
		if got := LCIDCharset(tt.lcid); got != tt.charset {
			t.Errorf("LCIDCharset(%#x): expected %q, got: %q", tt.lcid, tt.charset, got)
		}
	}
}
