package html

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
	"testing/fstest"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook/ebooktest"
	"github.com/xo/ivy/internal/fixture"
)

// page is an HTML file with a head.
const page = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>A Page &amp; Its Title</title>
<meta name="author" content="Ann Author">
<meta name="generator" content="An Editor">
<link rel="stylesheet" href="style.css">
</head>
<body><p>Hello</p><img src="a.png"></body>
</html>`

func TestHTML(t *testing.T) {
	d, err := Open(t.Context(), strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(src.Documents, []string{"index.html"}) || src.Title != "A Page & Its Title" {
		t.Errorf("expected the document and its title, got: %q %q", src.Documents, src.Title)
	}
	if len(src.StyleSheets) != 1 || !strings.Contains(src.StyleSheets[0], "size: letter") {
		t.Errorf("expected the style sheet of paper, got: %q", src.StyleSheets)
	}
	data, err := fs.ReadFile(src.FS, "index.html")
	if err != nil || string(data) != page {
		t.Errorf("expected the file as it is, got: %q %v", data, err)
	}
	if _, err := fs.ReadFile(src.FS, "style.css"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected no other file, got: %v", err)
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := ivy.Info{Title: "A Page & Its Title", Authors: []string{"Ann Author"}, Language: "en", Producer: "An Editor"}
	if info.Title != want.Title || !slices.Equal(info.Authors, want.Authors) || info.Language != want.Language || info.Producer != want.Producer {
		t.Errorf("expected %+v, got: %+v", want, info)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected no preview, got: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Error(err)
	}
}

func TestXHTML(t *testing.T) {
	for doc, want := range map[string]string{
		"\xef\xbb\xbf  <?xml version=\"1.0\"?><html xmlns=\"http://www.w3.org/1999/xhtml\"/>": "index.xhtml",
		"<html><title>x</title></html>": "index.html",
		"":                              "index.html",
	} {
		d, err := Open(t.Context(), strings.NewReader(doc))
		if err != nil {
			t.Fatal(err)
		}
		src, err := d.source()
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(src.Documents, []string{want}) {
			t.Errorf("%q: expected %s, got: %q", doc, want, src.Documents)
		}
	}
}

func TestWindows1252Title(t *testing.T) {
	d, err := Open(t.Context(), strings.NewReader("<title>\x93Caf\xe9\x94</title>"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "“Café”" {
		t.Errorf("expected the title in windows-1252, got: %q", info.Title)
	}
}

// metadata is the metadata.opf of an HTMLZ file, as calibre writes it.
const metadata = `<?xml version='1.0' encoding='utf-8'?>
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="uuid_id" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:identifier opf:scheme="uuid" id="uuid_id">0b5f6a49-0000-0000-0000-000000000000</dc:identifier>
    <dc:title>The Zipped Book</dc:title>
    <dc:creator opf:file-as="Author, Ann" opf:role="aut">Ann Author</dc:creator>
    <dc:contributor opf:role="bkp">calibre (7.0.0) [https://calibre-ebook.com]</dc:contributor>
    <dc:date>2019-03-04T00:00:00+00:00</dc:date>
    <dc:language>fr</dc:language>
  </metadata>
  <guide>
    <reference type="cover" title="Cover" href="cover.jpg"/>
  </guide>
</package>`

// htmlz returns the entries of an HTMLZ file.
func htmlz() []fixture.Entry {
	return []fixture.Entry{
		{Name: "index.html", Data: []byte(`<html><head><title>Index Title</title><link href="style.css" rel="stylesheet"></head><body><img src="images/a.png"></body></html>`)},
		{Name: "style.css", Data: []byte(`p { margin: 0 }`)},
		{Name: "images/a.png", Data: fixture.PNG(5, 5, color.Black)},
		{Name: "metadata.opf", Data: []byte(metadata)},
		{Name: "cover.jpg", Data: fixture.JPEG(40, 60, color.White)},
	}
}

func TestHTMLZ(t *testing.T) {
	d, err := Format.Open(t.Context(), bytes.NewReader(fixture.Zip(htmlz()...)), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	doc := d.(*Document)
	src, err := doc.source()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(src.Documents, []string{"index.html"}) || src.Title != "The Zipped Book" {
		t.Errorf("expected the index and the title of the metadata, got: %q %q", src.Documents, src.Title)
	}
	if len(src.StyleSheets) != 1 || !strings.Contains(src.StyleSheets[0], "size: 6in 9in") {
		t.Errorf("expected the style sheet of an ebook, got: %q", src.StyleSheets)
	}
	for _, name := range []string{"index.html", "style.css", "images/a.png"} {
		if _, err := fs.ReadFile(src.FS, name); err != nil {
			t.Errorf("expected %s in the file system, got: %v", name, err)
		}
	}
	info, err := doc.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "The Zipped Book" || !slices.Equal(info.Authors, []string{"Ann Author"}) || info.Language != "fr" || info.Created.Year() != 2019 {
		t.Errorf("expected the metadata of metadata.opf, got: %+v", info)
	}
	cover, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if cover.Bounds().Size() != image.Pt(40, 60) {
		t.Errorf("expected the cover, got: %v", cover.Bounds().Size())
	}
}

func TestHTMLZPassword(t *testing.T) {
	buf := fixture.EncryptedZip(fixture.WinZipAES2, "secret", htmlz()...)
	if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ErrPassword without a password, got: %v", err)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: "secret"}))
	if err != nil {
		t.Fatal(err)
	}
	if info, err := d.Describe(t.Context()); err != nil || info.Title != "The Zipped Book" {
		t.Errorf("expected the title of the metadata, got: %+v, %v", info, err)
	}
	if n, err := d.PageCount(t.Context()); err != nil || n < 1 {
		t.Errorf("expected pages, got: %d, %v", n, err)
	}
}

func TestHTMLZWithoutMetadata(t *testing.T) {
	entries := []fixture.Entry{
		{Name: "book/b.htm", Data: []byte(`<title>B</title>`)},
		{Name: "z.xhtml", Data: []byte(`<?xml version="1.0"?><html><head><title>Z Title</title></head></html>`)},
		{Name: "a/a.html", Data: []byte(`<title>A</title>`)},
		{Name: "metadata.opf", Data: []byte(`not xml`)},
		{Name: "cover.png", Data: fixture.PNG(3, 4, color.White)},
	}
	d, err := Open(t.Context(), bytes.NewReader(fixture.Zip(entries...)))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(src.Documents, []string{"z.xhtml"}) || src.Title != "Z Title" {
		t.Errorf("expected the document nearest to the root and its title, got: %q %q", src.Documents, src.Title)
	}
	cover, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if cover.Bounds().Size() != image.Pt(3, 4) {
		t.Errorf("expected cover.png, got: %v", cover.Bounds().Size())
	}
}

func TestErrors(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader(fixture.Zip(fixture.Entry{Name: "a.txt"}))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for an htmlz without html, got: %v", err)
	}
	if _, err := Open(t.Context(), strings.NewReader("PK\x03\x04 broken")); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for a broken zip, got: %v", err)
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 10}}
	if _, err := Open(t.Context(), strings.NewReader(page), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
	d, err := Open(t.Context(), bytes.NewReader(fixture.Zip(htmlz()...)), WithConfig(ivy.Config{Limits: ivy.Limits{MaxMemory: 64}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.source(); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit from a large index, got: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(ctx, strings.NewReader(page)); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	f.Add([]byte(page))
	f.Add(fixture.Zip(htmlz()...))
	f.Add(fixture.EncryptedZip(fixture.WinZipAES2, "secret", htmlz()...))
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
		src, err := d.source()
		check(err)
		if err == nil {
			_, err = fs.ReadFile(src.FS, src.Documents[0])
			check(err)
		}
		ebooktest.RenderFirstPage(t, d)
	})
}

func TestPages(t *testing.T) {
	d, err := Open(t.Context(), strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil || n != 1 {
		t.Fatalf("expected 1 page, got: %d %v", n, err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 612 || h != 792 {
		t.Errorf("expected a page of US Letter, got: %g by %g", w, h)
	}
	img := ebooktest.RenderPage(t, d, 0)
	// "Hello" is in the top left corner, inside the margins of 0.75 inch.
	if ink := ebooktest.Ink(img, image.Rect(72, 72, 160, 100)); ink < 20 {
		t.Errorf("expected the ink of the text, got %d pixels", ink)
	}
	if ink := ebooktest.Ink(img, image.Rect(0, 0, 816, 70)); ink != 0 {
		t.Errorf("expected no ink in the top margin, got %d pixels", ink)
	}
	if _, err := d.Page(t.Context(), 1); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ivy.ErrPageRange, got: %v", err)
	}
}

func TestPagesHTMLZ(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(fixture.Zip(htmlz()...)))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 6*72 || h != 9*72 {
		t.Errorf("expected a page of 6 by 9 inches, got: %g by %g", w, h)
	}
	img := ebooktest.RenderPage(t, d, 0)
	// The black image of 5 pixels is on the first line, in the top left
	// corner of the page area, inside the margins of 0.5 and 0.6 inches.
	if ink := ebooktest.Ink(img, image.Rect(48, 57, 54, 80)); ink < 20 {
		t.Errorf("expected the image, got %d pixels", ink)
	}
}

// TestWithDir checks that the layout reads a style sheet that an HTML file
// links to from the folder of WithDir, and that it reads none without it.
func TestWithDir(t *testing.T) {
	doc := `<html><head><link rel="stylesheet" href="css/red.css"></head><body><p>Hello</p></body></html>`
	dir := fstest.MapFS{"css/red.css": {Data: []byte("p { background: #f00 }")}}
	for _, test := range []struct {
		name string
		opts []Option
		red  bool
	}{
		{"with a folder", []Option{WithDir(dir)}, true},
		{"without a folder", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, err := Open(t.Context(), strings.NewReader(doc), test.opts...)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := d.Close(); err != nil {
					t.Error(err)
				}
			})
			img := ebooktest.RenderPage(t, d, 0)
			bounds, n := img.Bounds(), 0
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					if r, g, b, _ := img.At(x, y).RGBA(); r > 0xf000 && g < 0x1000 && b < 0x1000 {
						n++
					}
				}
			}
			if red := n > 0; red != test.red {
				t.Errorf("expected a red paragraph: %t, got %d red pixels", test.red, n)
			}
		})
	}
}

func TestPaper(t *testing.T) {
	for name, src := range map[string][]byte{"html": []byte(page), "htmlz": fixture.Zip(htmlz()...)} {
		t.Run(name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(src), WithConfig(ivy.Config{Paper: ebooktest.Landscape}))
			if err != nil {
				t.Fatal(err)
			}
			ebooktest.CheckPaper(t, d)
		})
	}
}
