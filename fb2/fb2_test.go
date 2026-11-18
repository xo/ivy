package fb2

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook/ebooktest"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/okage/roxmltree"
	"golang.org/x/text/encoding/charmap"
)

// book returns a FictionBook file with a cover, a poem, a table, a note and
// an image in the text. The text is in UTF-8, and the declaration names
// encoding, which the caller converts the text to.
func book(encoding string) string {
	cover := base64.StdEncoding.EncodeToString(fixture.PNG(30, 40, color.NRGBA{0, 0, 255, 255}))
	// a binary in base64 is often split into lines
	var lines []string
	for len(cover) > 60 {
		lines, cover = append(lines, cover[:60]), cover[60:]
	}
	lines = append(lines, cover)
	return `<?xml version="1.0" encoding="` + encoding + `"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
<description>
<title-info>
<genre>sf</genre>
<author><first-name>Лев</first-name><middle-name>Николаевич</middle-name><last-name>Толстой</last-name></author>
<author><nickname>anon</nickname></author>
<book-title>Война и мир</book-title>
<annotation><p>Роман о <emphasis>войне</emphasis>.</p></annotation>
<date value="1869-01-01">1869</date>
<coverpage><image l:href="#cover.png"/></coverpage>
<lang>ru</lang>
</title-info>
<document-info><program-used>FictionBook Editor 2.6</program-used><id>doc-1</id></document-info>
<publish-info><book-name>Война и мир</book-name><publisher>Русский вестник</publisher><isbn>978-0-00-000000-0</isbn></publish-info>
</description>
<body>
<title><p>Война и мир</p><p>Том 1</p></title>
<epigraph><p>Эпиграф</p><text-author>Автор</text-author></epigraph>
<section id="ch1">
<title><p>Часть первая</p></title>
<p>Текст с <strong>жирным</strong>, <strikethrough>зачёркнутым</strikethrough>, H<sub>2</sub>O и x<sup>2</sup> &amp; <code>код</code>.<a l:href="#n1" type="note">[1]</a></p>
<empty-line/>
<p>Ссылка на <a l:href="https://example.com/?a=1&amp;b=2">сайт</a> и <a l:href="javascript:alert(1)">скрипт</a>.</p>
<image l:href="#cover.png" alt="обложка"/>
<poem><title><p>Стих</p></title><stanza><v>Строка один</v><v>Строка два</v></stanza><text-author>Поэт</text-author><date>1812</date></poem>
<cite><p>Цитата</p></cite>
<subtitle>* * *</subtitle>
<table><tr align="center"><th colspan="2">Заголовок</th></tr><tr><td valign="top">a</td><td>b</td></tr></table>
<section><title><p>Глава</p></title><p id="ch1">Повтор id.</p><p><image l:href="#missing" alt="нет"/></p></section>
</section>
</body>
<body name="notes">
<section id="n1"><title><p>1</p></title><p>Примечание.</p></section>
</body>
<binary id="cover.png" content-type="image/png">
` + strings.Join(lines, "\n") + `
</binary>
</FictionBook>
`
}

// toWindows1251 converts UTF-8 text to windows-1251, for the letters of
// Russian and ASCII.
func toWindows1251(t testing.TB, s string) []byte {
	t.Helper()
	var buf []byte
	for _, r := range s {
		switch {
		case r < 0x80:
			buf = append(buf, byte(r))
		case r >= 0x410 && r <= 0x44f:
			buf = append(buf, byte(r-0x410+0xc0))
		case r == 'ё':
			buf = append(buf, 0xb8)
		default:
			t.Fatalf("no windows-1251 byte for %q", r)
		}
	}
	return buf
}

// toKOI8R converts UTF-8 text to KOI8-R.
func toKOI8R(t testing.TB, s string) []byte {
	t.Helper()
	buf, err := charmap.KOI8R.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

// toUTF16 converts UTF-8 text to UTF-16LE with a byte order mark.
func toUTF16(s string) []byte {
	units := utf16.Encode([]rune(s))
	buf := make([]byte, 0, 2+2*len(units))
	buf = append(buf, 0xff, 0xfe)
	for _, u := range units {
		buf = append(buf, byte(u), byte(u>>8))
	}
	return buf
}

func TestOpen(t *testing.T) {
	for name, buf := range map[string][]byte{
		"utf-8":        []byte(book("UTF-8")),
		"windows-1251": toWindows1251(t, book("windows-1251")),
		"utf-16":       toUTF16(book("UTF-16")),
		"koi8-r":       toKOI8R(t, book("KOI8-R")),
		"zip":          fixture.Zip(fixture.Entry{Name: "book.fb2", Data: []byte(book("utf-8"))}),
	} {
		t.Run(name, func(t *testing.T) {
			d, err := Format.Open(t.Context(), bytes.NewReader(buf), ivy.Config{})
			if err != nil {
				t.Fatal(err)
			}
			info, err := d.(ivy.Describer).Describe(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			want := ivy.Info{
				Title:      "Война и мир",
				Authors:    []string{"Лев Николаевич Толстой", "anon"},
				Language:   "ru",
				Created:    time.Date(1869, 1, 1, 0, 0, 0, 0, time.UTC),
				Producer:   "FictionBook Editor 2.6",
				Publisher:  "Русский вестник",
				Identifier: "978-0-00-000000-0",
			}
			if !equalInfo(info, want) {
				t.Errorf("expected %+v, got: %+v", want, info)
			}
			cover, err := ivy.BestPreview(t.Context(), d, image.Point{})
			if err != nil {
				t.Fatal(err)
			}
			if cover.Bounds().Size() != image.Pt(30, 40) {
				t.Errorf("expected a cover of 30x40, got: %v", cover.Bounds().Size())
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// equalInfo reports whether two descriptions are equal.
func equalInfo(a, b ivy.Info) bool {
	return a.Title == b.Title && strings.Join(a.Authors, "|") == strings.Join(b.Authors, "|") &&
		a.Subject == b.Subject && a.Language == b.Language && a.Created.Equal(b.Created) &&
		a.Modified.Equal(b.Modified) && a.Producer == b.Producer && a.Publisher == b.Publisher &&
		a.Identifier == b.Identifier
}

func TestDocumentID(t *testing.T) {
	const fb2 = `<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description>
<title-info><book-title>Book</book-title></title-info>
<document-info><id> 0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0 </id></document-info>
<publish-info><isbn> </isbn></publish-info>
</description>
<body><section><p>Text</p></section></body>
</FictionBook>`
	d, err := Open(t.Context(), strings.NewReader(fb2))
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Identifier != "0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0" || info.Publisher != "" {
		t.Errorf("expected the id of the document-info and no publisher, got: %q %q", info.Identifier, info.Publisher)
	}
}

func TestSource(t *testing.T) {
	d, err := Open(t.Context(), strings.NewReader(book("utf-8")))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(src.Documents, []string{coverName, documentName}) || src.Title != "Война и мир" || len(src.StyleSheets) != 2 {
		t.Fatalf("unexpected source: %q %q", src.Documents, src.Title)
	}
	buf, err := fs.ReadFile(src.FS, documentName)
	if err != nil {
		t.Fatal(err)
	}
	xhtml := string(buf)
	if _, err := roxmltree.Parse(xhtml); err != nil {
		t.Fatalf("the xhtml is not well formed: %v\n%s", err, xhtml)
	}
	for _, want := range []string{
		`<html xmlns="http://www.w3.org/1999/xhtml" lang="ru" xml:lang="ru">`,
		`<title>Война и мир</title>`,
		`<div class="annotation info">` + "\n" + `<h1>Annotation</h1>` + "\n" + `<p>Роман о <em>войне</em>.</p>` + "\n</div>\n<hr/>",
		`<ul class="toc">` + "\n" + `<li><a href="#toc-1">Война и мир</a>`,
		`<li><a href="#toc-2">Часть первая</a>`,
		`<li><a href="#toc-4">notes</a>`,
		`<h1 class="title" id="toc-1">Война и мир<br/>Том 1</h1>`,
		`<blockquote class="epigraph">` + "\n" + `<p>Эпиграф</p>` + "\n" + `<p class="text-author">Автор</p>`,
		`<div class="section" id="ch1">` + "\n" + `<h1 class="title" id="toc-2">Часть первая</h1>`,
		`<strong>жирным</strong>, <s>зачёркнутым</s>, H<sub>2</sub>O и x<sup>2</sup> &amp; <code>код</code>.`,
		`<a class="note" href="#n1">[1]</a>`,
		"<p class=\"empty-line\"> </p>",
		`<a href="https://example.com/?a=1&amp;b=2">сайт</a> и <span>скрипт</span>`,
		`<div class="image"><img src="images/0.png" alt="обложка"/></div>`,
		`<div class="poem-title">` + "\n" + `<p>Стих</p>`,
		`<div class="stanza">` + "\n" + `<p class="v">Строка один</p>`,
		`<p class="v">Строка два</p>` + "\n" + `<p class="empty-line">&#160;</p>` + "\n</div>",
		`<p class="date">1812</p>`,
		`<blockquote class="cite">`,
		`<p class="subtitle">* * *</p>`,
		`<tr style="text-align: center"><th colspan="2">Заголовок</th></tr>`,
		`<td style="vertical-align: top">a</td>`,
		`<h2 class="title" id="toc-3">Глава</h2>` + "\n" + `<p>Повтор id.</p>` + "\n" + `<p><span>нет</span></p>`,
		`<hr/>` + "\n" + `<p class="bodyname" id="toc-4">notes</p>`,
		`<div class="body notes">`,
		`<div class="section" id="n1">`,
	} {
		if !strings.Contains(xhtml, want) {
			t.Errorf("expected the xhtml to hold %q, got:\n%s", want, xhtml)
		}
	}
	img, err := fs.ReadFile(src.FS, "images/0.png")
	if err != nil {
		t.Fatal(err)
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(img)); err != nil || cfg.Width != 30 {
		t.Errorf("expected the cover in the fs, got: %v, %v", cfg, err)
	}
	again, err := d.source(t.Context())
	if err != nil || again.FS != src.FS {
		t.Errorf("expected the same source on a second call, got: %v", err)
	}
}

func TestErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		buf  string
		cfg  ivy.Config
		want error
	}{
		"empty":         {buf: "", want: ivy.ErrFormat},
		"not xml":       {buf: "plain text", want: ivy.ErrFormat},
		"other root":    {buf: "<html/>", want: ivy.ErrFormat},
		"zip of two":    {buf: string(fixture.Zip(fixture.Entry{Name: "a.txt"}, fixture.Entry{Name: "b.txt"})), want: ivy.ErrFormat},
		"broken zip":    {buf: "PK\x03\x04broken", want: ivy.ErrFormat},
		"too large":     {buf: book("utf-8"), cfg: ivy.Config{Limits: ivy.Limits{MaxMemory: 100}}, want: ivy.ErrLimit},
		"too many":      {buf: book("utf-8"), cfg: ivy.Config{Limits: ivy.Limits{MaxObjects: 10}}, want: ivy.ErrLimit},
		"entity attack": {buf: billionLaughs, want: ivy.ErrFormat},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Open(t.Context(), strings.NewReader(tc.buf), WithConfig(tc.cfg))
			if !errors.Is(err, tc.want) {
				t.Errorf("expected %v, got: %v", tc.want, err)
			}
		})
	}
}

// billionLaughs is a file whose entities expand to a billion characters.
const billionLaughs = `<?xml version="1.0"?>
<!DOCTYPE FictionBook [
<!ENTITY a "aaaaaaaaaa">
<!ENTITY b "&a;&a;&a;&a;&a;&a;&a;&a;&a;&a;">
<!ENTITY c "&b;&b;&b;&b;&b;&b;&b;&b;&b;&b;">
<!ENTITY d "&c;&c;&c;&c;&c;&c;&c;&c;&c;&c;">
<!ENTITY e "&d;&d;&d;&d;&d;&d;&d;&d;&d;&d;">
<!ENTITY f "&e;&e;&e;&e;&e;&e;&e;&e;&e;&e;">
<!ENTITY g "&f;&f;&f;&f;&f;&f;&f;&f;&f;&f;">
<!ENTITY h "&g;&g;&g;&g;&g;&g;&g;&g;&g;&g;">
<!ENTITY i "&h;&h;&h;&h;&h;&h;&h;&h;&h;&h;">
]>
<FictionBook><body><p>&i;</p></body></FictionBook>`

func TestDeepNesting(t *testing.T) {
	const depth = 10000
	text := "<FictionBook><body>" + strings.Repeat("<section>", depth) + "<p>deep</p>" +
		strings.Repeat("</section>", depth) + "<p>" + strings.Repeat("<emphasis>", depth) + "x" +
		strings.Repeat("</emphasis>", depth) + "</p></body></FictionBook>"
	d, err := Open(t.Context(), strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	buf, err := fs.ReadFile(src.FS, src.Documents[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(buf), "deep") {
		t.Error("expected the content past the limit of depth to be left out")
	}
	if _, err := roxmltree.Parse(string(buf)); err != nil {
		t.Errorf("the xhtml is not well formed: %v", err)
	}
}

func TestBadBinaries(t *testing.T) {
	text := `<FictionBook><description><title-info><coverpage><image href="#c"/></coverpage></title-info></description>
<body><image href="#c"/><image href="#x"/></body><binary id="c" content-type="image/png">!!!not base64</binary>
<binary id="x" content-type="image/png">` + base64.StdEncoding.EncodeToString([]byte("not a png")) + `</binary></FictionBook>`
	d, err := Open(t.Context(), strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range d.Previews(t.Context()) {
		if !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("expected ivy.ErrFormat, got: %v", err)
		}
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	buf, _ := fs.ReadFile(src.FS, src.Documents[0])
	if strings.Count(string(buf), "<img") != 1 {
		t.Errorf("expected only the image that decodes from base64, got:\n%s", buf)
	}
}

func TestCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(ctx, strings.NewReader(book("utf-8"))); err == nil {
		t.Error("expected an error for a canceled context")
	}
	d, err := Open(t.Context(), strings.NewReader(book("utf-8")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.source(ctx); err == nil {
		t.Error("expected an error for a canceled context")
	}
	if _, err := d.source(t.Context()); err != nil {
		t.Errorf("expected a later call to convert the book, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	f.Add([]byte(book("utf-8")))
	f.Add(toWindows1251(f, book("windows-1251")))
	f.Add(fixture.Zip(fixture.Entry{Name: "book.fb2", Data: []byte(book("utf-8"))}))
	f.Add([]byte(billionLaughs))
	f.Add(fixture.FictionBook(``, `<coverpage><image l:href="#c.png"/></coverpage>`, `<p foo:a="1">a<a l:href="#n">1</a></p>`,
		fixture.Binary{ID: "c.png", ContentType: "image/png", Data: fixture.PNG(2, 3, color.Black)}))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 4 << 20, MaxPixels: 1 << 20, MaxObjects: 1000, MaxDepth: 16}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		_, _ = d.Describe(t.Context())
		for p, err := range d.Previews(t.Context()) {
			if err == nil {
				_, _ = p.Decode(t.Context())
			}
		}
		src, err := d.source(t.Context())
		if err != nil {
			return
		}
		for _, name := range src.Documents {
			xhtml, err := fs.ReadFile(src.FS, name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := roxmltree.Parse(string(xhtml)); err != nil {
				t.Fatalf("the xhtml is not well formed: %v\n%s", err, xhtml)
			}
		}
		ebooktest.RenderFirstPage(t, d)
	})
}

func TestHTMLEntities(t *testing.T) {
	text := `<?xml version="1.0" encoding="windows-1251"?><FictionBook><body><p>a&nbsp;b&mdash;c&Undefined;d</p></body></FictionBook>`
	d, err := Open(t.Context(), strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	buf, _ := fs.ReadFile(src.FS, src.Documents[0])
	if !strings.Contains(string(buf), "<p>a b—cd</p>") {
		t.Errorf("expected the entities of html as characters, and no undefined entity, got:\n%s", buf)
	}
}

func TestPages(t *testing.T) {
	d, err := Open(t.Context(), strings.NewReader(book("utf-8")))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// The page of the cover comes first.
	if n < 2 {
		t.Fatalf("expected 2 pages or more, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 6*72 || h != 9*72 {
		t.Errorf("expected a page of 6 by 9 inches, got: %g by %g", w, h)
	}
	// The cover of 30 by 40 pixels fills the width of the page, and it is
	// in the middle of its height.
	cover := ebooktest.RenderPage(t, d, 0)
	if ink := ebooktest.Ink(cover, image.Rect(0, 50, 576, 814)); ink < 576*760 {
		t.Errorf("expected the cover on the first page, got %d pixels of ink", ink)
	}
	if ink := ebooktest.Ink(cover, image.Rect(0, 0, 576, 46)) + ebooktest.Ink(cover, image.Rect(0, 818, 576, 864)); ink != 0 {
		t.Errorf("expected no ink above and below the cover, got %d pixels", ink)
	}
	img := ebooktest.RenderPage(t, d, 1)
	// The title of the book is at the top of the page area, inside the
	// margins of 0.5 and 0.6 inches.
	if ink := ebooktest.Ink(img, image.Rect(48, 58, 528, 120)); ink < 50 {
		t.Errorf("expected the ink of the title, got %d pixels", ink)
	}
	if ink := ebooktest.Ink(img, image.Rect(0, 0, 576, 50)); ink != 0 {
		t.Errorf("expected no ink in the top margin, got %d pixels", ink)
	}
	if _, err := d.Page(t.Context(), n); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ivy.ErrPageRange, got: %v", err)
	}
}

func TestPaper(t *testing.T) {
	d, err := Open(t.Context(), strings.NewReader(book("utf-8")), WithConfig(ivy.Config{Paper: ebooktest.Landscape}))
	if err != nil {
		t.Fatal(err)
	}
	ebooktest.CheckPaper(t, d)
}
