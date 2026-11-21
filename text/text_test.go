package text

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/ebook/ebooktest"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/layout"
)

func TestToHTML(t *testing.T) {
	for _, tt := range []struct {
		name, text, title, lang, want string
	}{
		{"empty", "", "", "", ""},
		{"blank", " \n\t\n", "", "", ""},
		{"one", "Hello", "", "", "<p>Hello</p>\n"},
		{
			"paragraphs", "Line one\n  line two\n\n\n\nNext <b> & \"q\"\n",
			"", "",
			"<p>Line one\n  line two</p>\n<p>Next &lt;b&gt; &amp; &#34;q&#34;</p>\n",
		},
		{"crlf", "a\r\nb\r\n\r\nc\rd", "", "", "<p>a\nb</p>\n<p>c\nd</p>\n"},
		{"tabs and nul", "\ta\x00b\n \t \nc", "", "", "<p>\ta�b</p>\n<p>c</p>\n"},
		{"form feed", "a\n\f\nb", "", "", "<p>a</p>\n<p>b</p>\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := string(toHTML(tt.text, tt.title, tt.lang, false))
			body := got[strings.Index(got, "<body>\n")+len("<body>\n") : strings.Index(got, "</body>")]
			if body != tt.want {
				t.Errorf("expected %q, got: %q", tt.want, body)
			}
		})
	}
	got := string(toHTML("x", "A <Title>", `e"n`, true))
	for _, want := range []string{`<html lang="e&#34;n">`, "<title>A &lt;Title&gt;</title>", `<meta charset="utf-8">`, "<!DOCTYPE html>"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
	if got := string(toHTML("x", "", "", true)); strings.Contains(got, "<title>") || strings.Contains(got, "lang=") {
		t.Errorf("expected no title and no lang, got: %q", got)
	}
}

func TestJoin(t *testing.T) {
	for _, tt := range []struct {
		name, text, want string
	}{
		{"paragraph", "A line that\nwraps,  and\r\nends.\n\nNext", "<p>A line that wraps,  and ends.</p>\n<p>Next</p>\n"},
		{"space at the end", "one  \t\ntwo", "<p>one two</p>\n"},
		{"indented block", "Code:\n    if x {\n\ty()\n    }\n", "<p>Code:\n    if x {\n\ty()\n    }</p>\n"},
		{"poem", "  Twinkle, twinkle,\n  little star,\n\n", "<p>  Twinkle, twinkle,\n  little star,</p>\n"},
		{"indented paragraphs", "    It was a dark\nand stormy night.\n    The rain\nfell.", "<p>    It was a dark and stormy night.\n    The rain fell.</p>\n"},
		{"table of contents", "Contents\n\n CHAPTER I.     Down\n CHAPTER II.    Tears\n", "<p>Contents</p>\n<p> CHAPTER I.     Down\n CHAPTER II.    Tears</p>\n"},
		{"chinese", "此開卷第一回也，作者\n自云。\n　　此回中凡用\n“夢”\n", "<p>此開卷第一回也，作者自云。\n　　此回中凡用“夢”</p>\n"},
		{"japanese and latin", "吾輩は\nCat である\nand dog", "<p>吾輩はCat であるand dog</p>\n"},
		{"korean", "가나\n다라", "<p>가나 다라</p>\n"},
		{"escape", "a <\n& b", "<p>a &lt; &amp; b</p>\n"},
		{"heading, rule and text", "第一回　甄士隱\n----------\n此開卷第一回也，\n作者自云。", "<p>第一回　甄士隱</p>\n<p>----------</p>\n<p>此開卷第一回也，作者自云。</p>\n"},
		{"rule between paragraphs", "One\nline.\n  ****  \nTwo\nlines.", "<p>One line.</p>\n<p>  ****</p>\n<p>Two lines.</p>\n"},
		{"box drawing rule", "a\n────\nb", "<p>a</p>\n<p>────</p>\n<p>b</p>\n"},
		{"mixed punctuation", "a\n-=-=\nb", "<p>a -=-= b</p>\n"},
		{"short rule", "a\n---\nb", "<p>a --- b</p>\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := string(toHTML(tt.text, "", "", true))
			body := got[strings.Index(got, "<body>\n")+len("<body>\n") : strings.Index(got, "</body>")]
			if body != tt.want {
				t.Errorf("expected %q, got: %q", tt.want, body)
			}
		})
	}
	// a text file joins its lines, and a TXTZ file keeps them
	d, err := Open(t.Context(), strings.NewReader("First paragraph\nstill first.\n\nSecond."))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := document(t, d), "<p>First paragraph still first.</p>\n<p>Second.</p>"; !strings.Contains(got, want) {
		t.Errorf("expected %q in %q", want, got)
	}
}

// document returns the document of HTML of d.
func document(t *testing.T, d *Document) string {
	t.Helper()
	src := d.source()
	if len(src.Documents) != 1 {
		t.Fatalf("expected one document, got: %q", src.Documents)
	}
	data, err := fs.ReadFile(src.FS, src.Documents[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestText(t *testing.T) {
	for _, tt := range []struct {
		name string
		buf  []byte
		want string
	}{
		{"utf-8", []byte("caf\xc3\xa9"), "<p>café</p>"},
		{"utf-8 bom", []byte("\xef\xbb\xbfcaf\xc3\xa9"), "<p>café</p>"},
		{"utf-16le", []byte{0xff, 0xfe, 'h', 0, 'i', 0, '\n', 0, '\n', 0, '<', 0}, "<p>hi</p>\n<p>&lt;</p>"},
		{"utf-16be", []byte{0xfe, 0xff, 0, 'h', 0, 'i'}, "<p>hi</p>"},
		{"windows-1252", []byte("\x93caf\xe9\x94"), "<p>“café”</p>"},
		{"no language", []byte("text"), "<html>"},
		{"japanese", []byte("吾輩は猫である。"), `<html lang="ja">`},
		{"gutenberg", []byte("The Project Gutenberg eBook of 紅樓夢\n\nLanguage: Chinese\n\n*** START OF THE PROJECT GUTENBERG EBOOK ***\n\n此開卷第一回也"), `<html lang="zh-Hant">`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(tt.buf))
			if err != nil {
				t.Fatal(err)
			}
			if got := document(t, d); !strings.Contains(got, tt.want) {
				t.Errorf("expected %q in %q", tt.want, got)
			}
		})
	}
}

func TestSource(t *testing.T) {
	d, err := Format.Open(t.Context(), strings.NewReader("Title\n\nBody text."), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	doc := d.(*Document)
	src := doc.source()
	if !slices.Equal(src.Documents, []string{"text.html"}) || src.Title != "" {
		t.Errorf("expected the document without a title, got: %q %q", src.Documents, src.Title)
	}
	if len(src.StyleSheets) != 2 || src.StyleSheets[1] != ebook.BodyStyle || !strings.Contains(src.StyleSheets[0], "white-space: pre-wrap") ||
		!strings.Contains(src.StyleSheets[0], "monospace") || !strings.Contains(src.StyleSheets[0], "size: letter") {
		t.Errorf("expected the style sheet of text, got: %q", src.StyleSheets)
	}
	info, err := doc.Describe(t.Context())
	if err != nil || info.Title != "" || info.Authors != nil {
		t.Errorf("expected no metadata, got: %+v %v", info, err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected no preview, got: %v", err)
	}
	if err := doc.Close(); err != nil {
		t.Error(err)
	}
}

// metadata is the metadata.opf of a TXTZ file.
const metadata = `<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Plain &amp; Simple</dc:title>
    <dc:creator>Ann Author</dc:creator>
    <dc:language>en</dc:language>
  </metadata>
  <guide><reference type="cover" href="images/cover.png"/></guide>
</package>`

func TestTXTZ(t *testing.T) {
	buf := fixture.Zip(
		fixture.Entry{Name: "index.txt", Data: []byte("First paragraph\nstill first.\n\nSecond.")},
		fixture.Entry{Name: "metadata.opf", Data: []byte(metadata)},
		fixture.Entry{Name: "images/cover.png", Data: fixture.PNG(30, 45, color.White)},
		fixture.Entry{Name: "other.txt", Data: []byte("not this one")},
	)
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	src := d.source()
	if !slices.Equal(src.Documents, []string{"index.txt.html"}) || src.Title != "Plain & Simple" {
		t.Errorf("expected the document next to the text and the title, got: %q %q", src.Documents, src.Title)
	}
	if len(src.StyleSheets) != 2 || !strings.Contains(src.StyleSheets[0], "size: 6in 9in") || !strings.Contains(src.StyleSheets[1], "pre-wrap") {
		t.Errorf("expected the style sheets of an ebook and of text, got: %q", src.StyleSheets)
	}
	got := document(t, d)
	for _, want := range []string{"<p>First paragraph\nstill first.</p>\n<p>Second.</p>", "<title>Plain &amp; Simple</title>", `lang="en"`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
	if _, err := fs.ReadFile(src.FS, "images/cover.png"); err != nil {
		t.Errorf("expected the images in the file system, got: %v", err)
	}
	info, err := d.Describe(t.Context())
	if err != nil || info.Title != "Plain & Simple" || !slices.Equal(info.Authors, []string{"Ann Author"}) {
		t.Errorf("expected the metadata, got: %+v %v", info, err)
	}
	cover, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if cover.Bounds().Size() != image.Pt(30, 45) {
		t.Errorf("expected the cover, got: %v", cover.Bounds().Size())
	}
}

func TestTXTZPassword(t *testing.T) {
	buf := fixture.EncryptedZip(fixture.ZipCrypto, "secret",
		fixture.Entry{Name: "index.txt", Data: []byte("Hidden text.")},
		fixture.Entry{Name: "metadata.opf", Data: []byte(metadata)},
	)
	if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ErrPassword without a password, got: %v", err)
	}
	cfg := ivy.Config{PasswordFunc: func(context.Context, int) (string, error) { return "secret", nil }}
	d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if got := document(t, d); !strings.Contains(got, "<p>Hidden text.</p>") {
		t.Errorf("expected the text, got: %q", got)
	}
}

func TestTXTZWithoutIndex(t *testing.T) {
	buf := fixture.Zip(
		fixture.Entry{Name: "deep/a.txt", Data: []byte("deep")},
		fixture.Entry{Name: "b.TEXT", Data: []byte("shallow")},
		fixture.Entry{Name: "a.text/", Data: nil},
	)
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if src := d.source(); !slices.Equal(src.Documents, []string{"b.TEXT.html"}) {
		t.Errorf("expected the text nearest to the root, got: %q", src.Documents)
	}
	if got := document(t, d); !strings.Contains(got, "<p>shallow</p>") {
		t.Errorf("expected the shallow text, got: %q", got)
	}
}

func TestErrors(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader(fixture.Zip(fixture.Entry{Name: "a.png"}))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for a txtz without text, got: %v", err)
	}
	if _, err := Open(t.Context(), strings.NewReader("PK\x03\x04 broken")); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for a broken zip, got: %v", err)
	}
	buf := fixture.Zip(fixture.Entry{Name: "index.txt", Data: []byte("x")}, fixture.Entry{Name: "index.txt.html/a", Data: []byte("x")})
	if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for a folder in the way, got: %v", err)
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 10}}
	if _, err := Open(t.Context(), strings.NewReader(strings.Repeat("a", 11)), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit for the text, got: %v", err)
	}
	cfg = ivy.Config{Limits: ivy.Limits{MaxMemory: 200}}
	if _, err := Open(t.Context(), strings.NewReader(strings.Repeat("<", 100)), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit for the document of html, got: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(ctx, strings.NewReader("x")); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	// Small parts of the galley make the fuzzer reach its code (D57).
	defer layout.SetGalleyOptions(layout.GalleyOptions{OneBlock: true, MaxParts: 4, Wait: true, Early: true})()
	f.Add([]byte("Hello\n\nworld"))
	f.Add([]byte{0xff, 0xfe, 'h', 0})
	f.Add(fixture.Zip(
		fixture.Entry{Name: "index.txt", Data: []byte("text")},
		fixture.Entry{Name: "metadata.opf", Data: []byte(metadata)},
		fixture.Entry{Name: "images/cover.png", Data: fixture.PNG(3, 3, color.White)},
	))
	f.Add(fixture.EncryptedZip(fixture.ZipCrypto, "secret", fixture.Entry{Name: "index.txt", Data: []byte("text")}))
	cfg := ivy.Config{Password: "secret", Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 1000}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		check := func(err error) {
			if err != nil && strings.Contains(err.Error(), "panic:") {
				t.Fatal(err)
			}
		}
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		check(err)
		if err != nil {
			return
		}
		_, err = d.Describe(t.Context())
		check(err)
		_, err = ivy.BestPreview(t.Context(), d, image.Point{})
		check(err)
		src := d.source()
		_, err = fs.ReadFile(src.FS, src.Documents[0])
		check(err)
		ebooktest.RenderFirstPage(t, d)
	})
}

func TestPages(t *testing.T) {
	text := "First paragraph\n\n" + strings.Repeat("A line of the long text.\n\n", 200)
	d, err := Open(t.Context(), strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Paragraphs of a line of 15px, with a margin of 13px between them, on a
	// page of 9.5 inches give about 32 paragraphs a page.
	if n < 3 {
		t.Errorf("expected 3 pages or more, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 612 || h != 792 {
		t.Errorf("expected a page of US Letter, got: %g by %g", w, h)
	}
	img := ebooktest.RenderPage(t, d, 0)
	// As in calibre, the first line is 1em below the margin, and it starts
	// at the margin on the left: the body has no margin.
	if ink := ebooktest.Ink(img, image.Rect(72, 72, 816, 84)); ink != 0 {
		t.Errorf("expected no ink above the first line, got %d pixels", ink)
	}
	if ink := ebooktest.Ink(img, image.Rect(72, 84, 78, 102)); ink == 0 {
		t.Error("expected the ink of the first letter at the margin, got none")
	}
	if ink := ebooktest.Ink(img, image.Rect(72, 84, 250, 102)); ink < 20 {
		t.Errorf("expected the ink of the first line, got %d pixels", ink)
	}
	if ink := ebooktest.Ink(img, image.Rect(400, 72, 816, 1056)); ink != 0 {
		t.Errorf("expected no ink right of the short lines, got %d pixels", ink)
	}
	last := ebooktest.RenderPage(t, d, n-1)
	if ink := ebooktest.Ink(last, image.Rect(72, 72, 300, 90)); ink < 20 {
		t.Errorf("expected the ink of a line on the last page, got %d pixels", ink)
	}
	if _, err := d.Page(t.Context(), n); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ivy.ErrPageRange, got: %v", err)
	}
}

func TestPaper(t *testing.T) {
	txtz := fixture.Zip(
		fixture.Entry{Name: "index.txt", Data: []byte("First paragraph\nstill first.\n\nSecond.")},
		fixture.Entry{Name: "metadata.opf", Data: []byte(metadata)},
	)
	for name, src := range map[string][]byte{"text": []byte("First paragraph\n\nSecond."), "txtz": txtz} {
		t.Run(name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(src), WithConfig(ivy.Config{Paper: ebooktest.Landscape}))
			if err != nil {
				t.Fatal(err)
			}
			ebooktest.CheckPaper(t, d)
		})
	}
}
