package epub

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/ebook/ebooktest"
	"github.com/xo/ivy/internal/fixture"
)

// uuid is the unique identifier of the books of the tests.
const uuid = "urn:uuid:12345678-1234-1234-1234-123456789abc"

// containerXML names the package document name.
func containerXML(name string) string {
	return `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="` + name + `" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`
}

// opf3 is the package document of the book of EPUB 3.
const opf3 = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">  ` + uuid + `  </dc:identifier>
    <dc:title>A Book of Tests</dc:title>
    <dc:creator>Ann Author</dc:creator>
    <dc:language>en</dc:language>
    <dc:publisher>A Press</dc:publisher>
    <dc:date>2020-05-12</dc:date>
    <dc:subject>Testing</dc:subject>
    <meta property="dcterms:modified">2021-01-02T03:04:05Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ch1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch2" href="text/ch%202.xhtml" media-type="application/xhtml+xml"/>
    <item id="notes" href="text/notes.xhtml" media-type="application/xhtml+xml"/>
    <item id="dtb" href="text/ch3.dtb" media-type="application/x-dtbook+xml" fallback="ch3"/>
    <item id="ch3" href="text/ch3.xhtml" media-type="application/xhtml+xml"/>
    <item id="loop1" href="text/l1.bin" media-type="application/octet-stream" fallback="loop2"/>
    <item id="loop2" href="text/l2.bin" media-type="application/octet-stream" fallback="loop1"/>
    <item id="gone" href="text/gone.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover" href="images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
    <item id="pic" href="images/pic.png" media-type="image/png"/>
    <item id="css" href="style.css" media-type="text/css"/>
    <item id="font1" href="fonts/idpf.otf" media-type="font/otf"/>
    <item id="font2" href="fonts/adobe.otf" media-type="font/otf"/>
  </manifest>
  <spine>
    <itemref idref="ch1"/>
    <itemref idref="notes" linear="no"/>
    <itemref idref="ch2"/>
    <itemref idref="dtb"/>
    <itemref idref="loop1"/>
    <itemref idref="gone"/>
    <itemref idref="pic"/>
    <itemref idref="unknown"/>
    <itemref idref="ch1"/>
  </spine>
</package>`

// encryptionXML lists the paths with their algorithms.
func encryptionXML(algorithms map[string]string) string {
	var b strings.Builder
	b.WriteString(`<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" xmlns:enc="http://www.w3.org/2001/04/xmlenc#">`)
	for name, alg := range algorithms {
		fmt.Fprintf(&b, `<enc:EncryptedData><enc:EncryptionMethod Algorithm="%s"/><enc:CipherData><enc:CipherReference URI="%s"/></enc:CipherData></enc:EncryptedData>`, alg, name)
	}
	b.WriteString(`</encryption>`)
	return b.String()
}

// fontData returns the bytes of a font of the tests.
func fontData() []byte {
	data := make([]byte, 2000)
	for i := range data {
		data[i] = byte(i * 7)
	}
	return data
}

// obfuscate obfuscates data with key, over its first n bytes.
func obfuscate(data, key []byte, n int) []byte {
	out := slices.Clone(data)
	for i := range min(n, len(out)) {
		out[i] ^= key[i%len(key)]
	}
	return out
}

// xhtml returns a document of XHTML with the body body.
func xhtml(body string) []byte {
	return []byte(`<?xml version="1.0" encoding="utf-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>t</title></head><body>` + body + `</body></html>`)
}

// book3 returns the entries of the book of EPUB 3, with its fonts
// obfuscated.
func book3() []fixture.Entry {
	idpfKey := sha1.Sum([]byte(uuid))
	adobeKey, _ := hex.DecodeString(strings.ReplaceAll(strings.TrimPrefix(uuid, "urn:uuid:"), "-", ""))
	return []fixture.Entry{
		{Name: "mimetype", Data: []byte("application/epub+zip"), Store: true},
		{Name: "META-INF/container.xml", Data: []byte(containerXML("OEBPS/content.opf"))},
		{Name: "META-INF/encryption.xml", Data: []byte(encryptionXML(map[string]string{
			"OEBPS/fonts/idpf.otf":  algIDPF,
			"OEBPS/fonts/adobe.otf": algAdobe,
		}))},
		{Name: "OEBPS/content.opf", Data: []byte(opf3)},
		{Name: "OEBPS/nav.xhtml", Data: xhtml(`<nav epub:type="toc"><ol><li><a href="text/ch1.xhtml">One</a></li></ol></nav>`)},
		{Name: "OEBPS/text/ch1.xhtml", Data: xhtml(`<h1>One</h1><img src="../images/pic.png"/>`)},
		{Name: "OEBPS/text/ch 2.xhtml", Data: xhtml(`<h1>Two</h1>`)},
		{Name: "OEBPS/text/notes.xhtml", Data: xhtml(`<p>Notes</p>`)},
		{Name: "OEBPS/text/ch3.dtb", Data: []byte(`<dtbook/>`)},
		{Name: "OEBPS/text/ch3.xhtml", Data: xhtml(`<h1>Three</h1>`)},
		{Name: "OEBPS/images/cover.jpg", Data: fixture.JPEG(60, 90, color.White)},
		{Name: "OEBPS/images/pic.png", Data: fixture.PNG(10, 10, color.Black)},
		{Name: "OEBPS/style.css", Data: []byte(`@font-face { font-family: a; src: url(fonts/idpf.otf) }`)},
		{Name: "OEBPS/fonts/idpf.otf", Data: obfuscate(fontData(), idpfKey[:], idpfLength)},
		{Name: "OEBPS/fonts/adobe.otf", Data: obfuscate(fontData(), adobeKey, adobeLength)},
	}
}

// mustOpen opens the entries as an EPUB file.
func mustOpen(t *testing.T, entries []fixture.Entry, opts ...Option) *Document {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(fixture.Zip(entries...)), opts...)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestEPUB3(t *testing.T) {
	d := mustOpen(t, book3())
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	// The page of the cover comes first, and the image of the spine is on
	// a page of its own.
	want := []string{
		"OEBPS/images/cover-page.xhtml",
		"OEBPS/text/ch1.xhtml",
		"OEBPS/text/ch 2.xhtml",
		"OEBPS/text/ch3.xhtml",
		"OEBPS/images/pic-page.xhtml",
		"OEBPS/text/notes.xhtml",
	}
	if !slices.Equal(src.Documents, want) {
		t.Errorf("expected the documents %q, got: %q", want, src.Documents)
	}
	if src.Title != "A Book of Tests" {
		t.Errorf("expected the title, got: %q", src.Title)
	}
	if len(src.StyleSheets) != 1 || !strings.Contains(src.StyleSheets[0], "size: 6in 9in") {
		t.Errorf("expected the style sheet of an ebook, got: %q", src.StyleSheets)
	}
	for _, name := range src.Documents {
		if _, err := fs.ReadFile(src.FS, name); err != nil {
			t.Errorf("expected each document in the file system, got: %v", err)
		}
	}
	for _, name := range []string{"OEBPS/style.css", "OEBPS/images/pic.png", "mimetype"} {
		if _, err := fs.ReadFile(src.FS, name); err != nil {
			t.Errorf("expected %s in the file system, got: %v", name, err)
		}
	}
	for _, name := range []string{"OEBPS/fonts/idpf.otf", "OEBPS/fonts/adobe.otf"} {
		data, err := fs.ReadFile(src.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, fontData()) {
			t.Errorf("expected %s deobfuscated", name)
		}
	}

	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "A Book of Tests" || !slices.Equal(info.Authors, []string{"Ann Author"}) ||
		info.Language != "en" || info.Subject != "Testing" ||
		!info.Created.Equal(time.Date(2020, 5, 12, 0, 0, 0, 0, time.UTC)) ||
		!info.Modified.Equal(time.Date(2021, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("expected the metadata, got: %+v", info)
	}

	cover, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if cover.Bounds().Size() != image.Pt(60, 90) {
		t.Errorf("expected the cover-image, got: %v", cover.Bounds().Size())
	}
	if err := d.Close(); err != nil {
		t.Error(err)
	}
}

func TestFormat(t *testing.T) {
	doc, err := Format.Open(t.Context(), bytes.NewReader(fixture.Zip(book3()...)), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.(ivy.Previewer); !ok {
		t.Error("expected a previewer")
	}
	if _, ok := doc.(ivy.Pager); !ok {
		t.Error("expected a pager")
	}
	if _, ok := doc.(ivy.Describer); !ok {
		t.Error("expected a describer")
	}
	if Format.Name != "epub" || !slices.Contains(Format.Mimes, "application/epub+zip") || !slices.Contains(Format.Exts, "epub") {
		t.Errorf("unexpected format: %+v", Format)
	}
}

// book2 returns the entries of a book of EPUB 2, with the package document
// in the root, an NCX file and the cover in the guide.
func book2(opfExtra, guide string) []fixture.Entry {
	opf := `<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:identifier id="id">book-2</dc:identifier>
    <dc:creator opf:role="aut">Bo Author</dc:creator>
    ` + opfExtra + `
  </metadata>
  <manifest>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="titlepage" href="titlepage.xhtml" media-type="application/xhtml+xml"/>
    <item id="c1" href="c1.html" media-type="text/html"/>
    <item id="img1" href="images/front.gif" media-type="image/gif"/>
    <item id="img2" href="images/other.png" media-type="image/png"/>
  </manifest>
  <spine toc="ncx"><itemref idref="titlepage"/><itemref idref="c1"/></spine>
  <guide>` + guide + `</guide>
</package>`
	return []fixture.Entry{
		{Name: "mimetype", Data: []byte("application/epub+zip"), Store: true},
		{Name: "META-INF/container.xml", Data: []byte(containerXML("content.opf"))},
		{Name: "content.opf", Data: []byte(opf)},
		{Name: "toc.ncx", Data: []byte(`<?xml version="1.0"?>
<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"><docTitle><text>Title &amp; From NCX</text></docTitle></ncx>`)},
		{Name: "titlepage.xhtml", Data: xhtml(`<div><svg xmlns:xlink="http://www.w3.org/1999/xlink"><image xlink:href="images/other.png"/></svg></div>`)},
		{Name: "c1.html", Data: []byte(`<html><body><p>Hello&nbsp;world</body></html>`)},
		{Name: "images/front.gif", Data: fixture.PNG(30, 40, color.White)},
		{Name: "images/other.png", Data: fixture.PNG(20, 25, color.White)},
	}
}

func TestEPUB2(t *testing.T) {
	for name, tt := range map[string]struct {
		extra, guide string
		cover        image.Point
		// docs are the documents of the layout. The title page shows
		// images/other.png without text, so it is the page of that cover,
		// and it stays after the page of the cover.
		docs []string
	}{
		"meta cover": {`<meta name="cover" content="img1"/>`, ``, image.Pt(30, 40), []string{"images/front-page.xhtml", "titlepage.xhtml", "c1.html"}},
		"meta path":  {`<meta name="cover" content="images/other.png"/>`, ``, image.Pt(20, 25), []string{"images/other-page.xhtml", "titlepage.xhtml", "c1.html"}},
		"guide page": {``, `<reference type="cover" href="titlepage.xhtml"/>`, image.Pt(20, 25), []string{"images/other-page.xhtml", "titlepage.xhtml", "c1.html"}},
		"guide img":  {``, `<reference type="cover" href="images/front.gif"/>`, image.Pt(30, 40), []string{"images/front-page.xhtml", "titlepage.xhtml", "c1.html"}},
		"no cover":   {``, ``, image.Point{}, []string{"titlepage.xhtml", "c1.html"}},
	} {
		t.Run(name, func(t *testing.T) {
			d := mustOpen(t, book2(tt.extra, tt.guide))
			img, err := ivy.BestPreview(t.Context(), d, image.Point{})
			switch {
			case tt.cover == image.Point{}:
				if !errors.Is(err, ivy.ErrNoPreview) {
					t.Errorf("expected no preview, got: %v", err)
				}
			case err != nil:
				t.Fatal(err)
			case img.Bounds().Size() != tt.cover:
				t.Errorf("expected a cover of %v, got: %v", tt.cover, img.Bounds().Size())
			}
			info, err := d.Describe(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if info.Title != "Title & From NCX" || !slices.Equal(info.Authors, []string{"Bo Author"}) {
				t.Errorf("expected the title of the NCX file, got: %+v", info)
			}
			src, err := d.source()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(src.Documents, tt.docs) || src.Title != info.Title {
				t.Errorf("expected the spine and the title, got: %q %q", src.Documents, src.Title)
			}
		})
	}
}

func TestCoverPageByID(t *testing.T) {
	entries := book2(``, ``)
	for i, e := range entries {
		if e.Name == "content.opf" {
			entries[i].Data = bytes.Replace(e.Data, []byte(`id="titlepage"`), []byte(`id="my-cover-page"`), 1)
			entries[i].Data = bytes.Replace(entries[i].Data, []byte(`idref="titlepage"`), []byte(`idref="my-cover-page"`), 1)
		}
	}
	d := mustOpen(t, entries)
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(20, 25) {
		t.Errorf("expected the image of the cover page, got: %v", img.Bounds().Size())
	}
}

func TestNoContainer(t *testing.T) {
	entries := slices.DeleteFunc(book2(``, ``), func(e fixture.Entry) bool {
		return e.Name == "META-INF/container.xml"
	})
	d := mustOpen(t, entries)
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Documents) != 2 {
		t.Errorf("expected the package found by its extension, got: %q", src.Documents)
	}
}

func TestManifestFallback(t *testing.T) {
	entries := book2(``, ``)
	for i, e := range entries {
		if e.Name == "content.opf" {
			entries[i].Data = bytes.Replace(e.Data,
				[]byte(`<spine toc="ncx"><itemref idref="titlepage"/><itemref idref="c1"/></spine>`),
				[]byte(`<spine/>`), 1)
		}
	}
	d := mustOpen(t, entries)
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(src.Documents, []string{"titlepage.xhtml", "c1.html"}) {
		t.Errorf("expected the documents of the manifest, got: %q", src.Documents)
	}
}

func TestDRM(t *testing.T) {
	entries := book3()
	entries[2] = fixture.Entry{Name: "META-INF/encryption.xml", Data: []byte(encryptionXML(map[string]string{
		"OEBPS/text/ch1.xhtml": "http://www.w3.org/2001/04/xmlenc#aes128-cbc",
	}))}
	entries = append(entries, fixture.Entry{Name: "META-INF/rights.xml", Data: []byte(`<adept:rights xmlns:adept="http://ns.adobe.com/adept"/>`)})
	d := mustOpen(t, entries)
	if _, err := d.source(); !errors.Is(err, ivy.ErrDRM) {
		t.Errorf("expected ivy.ErrDRM, got: %v", err)
	}
	if _, err := d.fsys.Open("OEBPS/text/ch1.xhtml"); !errors.Is(err, ivy.ErrDRM) {
		t.Errorf("expected ivy.ErrDRM from the file, got: %v", err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); err != nil {
		t.Errorf("expected the cover in the clear, got: %v", err)
	}
	if info, err := d.Describe(t.Context()); err != nil || info.Title == "" {
		t.Errorf("expected the metadata in the clear, got: %+v %v", info, err)
	}

	// a cover that DRM encrypts is no preview
	entries[2] = fixture.Entry{Name: "META-INF/encryption.xml", Data: []byte(encryptionXML(map[string]string{
		"OEBPS/images/cover.jpg": "http://www.w3.org/2001/04/xmlenc#aes256-cbc",
	}))}
	d = mustOpen(t, entries)
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected no preview, got: %v", err)
	}
	if _, err := d.source(); err != nil {
		t.Errorf("expected the documents in the clear, got: %v", err)
	}
}

func TestObfuscationWithoutKey(t *testing.T) {
	entries := book3()
	for i, e := range entries {
		if e.Name == "OEBPS/content.opf" {
			entries[i].Data = bytes.ReplaceAll(e.Data, []byte(uuid), []byte("not-a-uuid"))
		}
	}
	d := mustOpen(t, entries)
	if _, err := fs.ReadFile(d.fsys, "OEBPS/fonts/adobe.otf"); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for a font without a key, got: %v", err)
	}
	// the key of IDPF is any identifier
	key := sha1.Sum([]byte("not-a-uuid"))
	data, err := fs.ReadFile(d.fsys, "OEBPS/fonts/idpf.otf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, obfuscate(obfuscate(fontData(), sha1Sum(uuid), idpfLength), key[:], idpfLength)) {
		t.Error("expected the font deobfuscated with the identifier of the package")
	}
}

// sha1Sum returns the SHA-1 of s.
func sha1Sum(s string) []byte {
	sum := sha1.Sum([]byte(s))
	return sum[:]
}

func TestUUIDKey(t *testing.T) {
	for id, ok := range map[string]bool{
		uuid: true,
		"URN:UUID:12345678-1234-1234-1234-123456789ABC": true,
		"{12345678-1234-1234-1234-123456789abc}":        true,
		"12345678123412341234123456789abc":              true,
		"urn:isbn:9780000000000":                        false,
		"12345678-1234-1234-1234-123456789abz":          false,
	} {
		if _, got := uuidKey(id); got != ok {
			t.Errorf("uuidKey(%q): expected %v, got: %v", id, ok, got)
		}
	}
}

func TestErrors(t *testing.T) {
	for name, tt := range map[string]struct {
		buf  []byte
		want error
	}{
		"not zip":     {[]byte("not a zip archive"), ivy.ErrFormat},
		"empty zip":   {fixture.Zip(), ivy.ErrFormat},
		"no package":  {fixture.Zip(fixture.Entry{Name: "META-INF/container.xml", Data: []byte(containerXML("a.opf"))}), ivy.ErrFormat},
		"bad package": {fixture.Zip(fixture.Entry{Name: "content.opf", Data: []byte("<html/>")}), ivy.ErrFormat},
		"bad xml":     {fixture.Zip(fixture.Entry{Name: "content.opf", Data: []byte("<package>")}), ivy.ErrFormat},
		"bad encryption": {fixture.Zip(
			fixture.Entry{Name: "content.opf", Data: []byte(`<package version="3.0"/>`)},
			fixture.Entry{Name: "META-INF/encryption.xml", Data: []byte("<encryption")},
		), ivy.ErrFormat},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader(tt.buf)); !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got: %v", tt.want, err)
			}
		})
	}

	d := mustOpen(t, []fixture.Entry{{Name: "content.opf", Data: []byte(`<package version="3.0"><metadata/></package>`)}})
	if _, err := d.source(); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for a book without a document, got: %v", err)
	}
	if info, err := d.Describe(t.Context()); err != nil || info.Title != "" {
		t.Errorf("expected empty metadata, got: %+v %v", info, err)
	}

	cfg := ivy.Config{Limits: ivy.Limits{MaxObjects: 10}}
	if _, err := Open(t.Context(), bytes.NewReader(fixture.Zip(book3()...)), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
	cfg = ivy.Config{Limits: ivy.Limits{MaxMemory: 100}}
	if _, err := Open(t.Context(), bytes.NewReader(fixture.Zip(book3()...)), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(ctx, bytes.NewReader(fixture.Zip(book3()...))); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	d = mustOpen(t, book3())
	if _, err := d.Describe(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	for _, err := range d.Previews(ctx) {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(fixture.Zip(book3()...))
	f.Add(fixture.Zip(book2(`<meta name="cover" content="img1"/>`, `<reference type="cover" href="titlepage.xhtml"/>`)...))
	f.Add(fixture.EPUB(fixture.EPUBPackage{
		Manifest: coverItem + `<item id="p1" href="p1.svg" media-type="image/svg+xml"/>`,
		Spine:    `<itemref idref="p1"/>`,
	}, fixture.Entry{Name: "p1.svg", Data: []byte(square)}, fixture.Entry{Name: "cover.png", Data: fixture.PNG(2, 3, color.Black)}))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 1000}}
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
		if err != nil {
			return
		}
		for _, name := range src.Documents {
			if data, err := fs.ReadFile(src.FS, name); err == nil {
				_ = ebook.ScanHead(ebook.Text(data))
			}
		}
		ebooktest.RenderFirstPage(t, d)
	})
}

func TestConcurrent(t *testing.T) {
	d := mustOpen(t, book3())
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			src, err := d.source()
			if err != nil {
				t.Error(err)
				return
			}
			for _, name := range append(src.Documents, "OEBPS/fonts/idpf.otf") {
				if _, err := fs.ReadFile(src.FS, name); err != nil {
					t.Error(err)
				}
			}
			if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); err != nil {
				t.Error(err)
			}
			if _, err := d.Describe(t.Context()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

func TestPages(t *testing.T) {
	d := mustOpen(t, book3())
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// The page of the cover, the four documents of the spine and the image
	// of the spine each start a page.
	if n != 6 {
		t.Errorf("expected 6 pages, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 6*72 || h != 9*72 {
		t.Errorf("expected a page of 6 by 9 inches, got: %g by %g", w, h)
	}
	// The cover is white, and it fills the page.
	if ink := ebooktest.Ink(ebooktest.RenderPage(t, d, 0), image.Rect(0, 0, 576, 864)); ink != 0 {
		t.Errorf("expected the white cover on the first page, got %d pixels of ink", ink)
	}
	img := ebooktest.RenderPage(t, d, 1)
	// The heading "One" is in the top left corner, inside the margins of
	// 0.5 and 0.6 inches.
	if ink := ebooktest.Ink(img, image.Rect(48, 58, 200, 110)); ink < 20 {
		t.Errorf("expected the ink of the heading, got %d pixels", ink)
	}
	if ink := ebooktest.Ink(img, image.Rect(0, 0, 576, 50)); ink != 0 {
		t.Errorf("expected no ink in the top margin, got %d pixels", ink)
	}
	if _, err := d.Page(t.Context(), n); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ivy.ErrPageRange, got: %v", err)
	}
	if _, err := d.Page(t.Context(), -1); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ivy.ErrPageRange, got: %v", err)
	}
	pages := 0
	for _, err := range ivy.Pages(t.Context(), d) {
		if err != nil {
			t.Fatal(err)
		}
		pages++
	}
	if pages != n {
		t.Errorf("expected %d pages from ivy.Pages, got: %d", n, pages)
	}
}

func TestPagesDRM(t *testing.T) {
	entries := book3()
	entries[2].Data = []byte(encryptionXML(map[string]string{"OEBPS/text/ch1.xhtml": "http://www.w3.org/2001/04/xmlenc#aes128-cbc"}))
	d := mustOpen(t, entries)
	if _, err := d.Page(t.Context(), 0); !errors.Is(err, ivy.ErrDRM) {
		t.Errorf("expected ivy.ErrDRM, got: %v", err)
	}
	if _, err := d.PageCount(t.Context()); !errors.Is(err, ivy.ErrDRM) {
		t.Errorf("expected ivy.ErrDRM, got: %v", err)
	}
}
