package chm

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/ebook/ebooktest"
)

// cyrillic is "Привет" in windows-1251.
var cyrillic = []byte{0xcf, 0xf0, 0xe8, 0xe2, 0xe5, 0xf2}

// helpFiles returns the files of a help file in Russian: a default topic,
// a table of contents that lists b.htm before a.htm, a page that the table
// of contents leaves out, an image and a style sheet.
func helpFiles() []chmFile {
	page := func(title string, body []byte) []byte {
		return slices.Concat([]byte("<html><head><title>"+title+"</title></head><body><p>"), body, []byte("</p></body></html>"))
	}
	return []chmFile{
		{name: "/#SYSTEM", data: systemFile(map[uint16][]byte{
			systemContents: cstr("Toc.hhc"),
			systemDefault:  cstr("index.htm"),
			systemTitle:    cstr("Help of Tests"),
			systemLCID:     lcidEntry(0x0419),
		}), plain: true},
		{name: "/index.htm", data: page("Start", []byte(`<img src="images/logo.gif">`))},
		{name: "/Toc.hhc", data: []byte(`<html><body><ul>
<li><object type="text/sitemap"><param name="Name" value="B"><param name="Local" value="pages/b.htm#top"></object>
<li><object type="text/sitemap"><param name="Name" value="A"><param name="Local" value="mk:@MSITStore:help.chm::/pages/A.htm"></object>
<li><object type="text/sitemap"><param name="Name" value="Out"><param name="Local" value="http://example.com/"></object>
</ul></body></html>`)},
		{name: "/pages/", data: nil, plain: true},
		{name: "/pages/a.htm", data: page("A", cyrillic)},
		{name: "/pages/b.htm", data: page("B", []byte("plain text"))},
		{name: "/pages/c.htm", data: page("C", []byte("caf\xc3\xa9"))},
		{name: "/images/logo.gif", data: []byte("GIF89a"), plain: true},
		{name: "/style.css", data: []byte("p { color: red }")},
		{name: "/$WWKeywordLinks/BTree", data: []byte("x"), plain: true},
	}
}

func TestOpen(t *testing.T) {
	for _, o := range []chmOptions{
		{},
		{chunkSize: 0x100, interval: 1, lzxcVersion: 1, version: 2},
	} {
		buf := buildCHM(helpFiles(), o)
		d, err := Open(t.Context(), bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		info, err := d.Describe(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if info.Title != "Help of Tests" || info.Language != "ru" {
			t.Errorf("expected the title and the language, got: %+v", info)
		}
		for _, err := range d.Previews(t.Context()) {
			t.Errorf("expected no preview, got one with the error %v", err)
		}
		src, err := d.source()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"index.htm", "pages/b.htm", "pages/a.htm", "pages/c.htm"}
		if !slices.Equal(src.Documents, want) {
			t.Errorf("expected the documents %q, got: %q", want, src.Documents)
		}
		if src.Title != "Help of Tests" || len(src.StyleSheets) != 2 || src.StyleSheets[0] != ebook.PaperStyle || src.StyleSheets[1] != ebook.BodyStyle {
			t.Errorf("expected the title and the style sheets, got: %q %d", src.Title, len(src.StyleSheets))
		}
		data, err := fs.ReadFile(src.FS, "pages/a.htm")
		if err != nil {
			t.Fatal(err)
		}
		wantA := slices.Concat([]byte(`<html><head><meta charset="windows-1251"><title>A</title></head><body><p>`), cyrillic, []byte("</p></body></html>"))
		if !bytes.Equal(data, wantA) {
			t.Errorf("expected the charset of the LCID, got: %q", data)
		}
		if data, err := fs.ReadFile(src.FS, "pages/c.htm"); err != nil || !bytes.Contains(data, []byte(`<meta charset="utf-8">`)) {
			t.Errorf("expected the charset of UTF-8, got: %q %v", data, err)
		}
		if data, err := fs.ReadFile(src.FS, "pages/b.htm"); err != nil || bytes.Contains(data, []byte("<meta")) {
			t.Errorf("expected an ASCII document as it is, got: %q %v", data, err)
		}
		if data, err := fs.ReadFile(src.FS, "IMAGES/LOGO.GIF"); err != nil || string(data) != "GIF89a" {
			t.Errorf("expected the image of section 0 in any case, got: %q %v", data, err)
		}
		for _, name := range []string{"#SYSTEM", "$WWKeywordLinks/BTree", "::DataSpace/NameList"} {
			if _, err := src.FS.Open(name); err == nil {
				t.Errorf("expected no system file %s", name)
			}
		}
	}
}

// TestLargeContent reads files that span many frames and resets of LZX, in
// an order that goes back.
func TestLargeContent(t *testing.T) {
	big := make([]byte, 5*frameSize+123)
	for i := range big {
		big[i] = byte('a' + i*7%26)
	}
	files := []chmFile{
		{name: "/one.htm", data: big},
		{name: "/two.htm", data: []byte("<p>two</p>")},
	}
	d, err := Open(t.Context(), bytes.NewReader(buildCHM(files, chmOptions{interval: 2})))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(src.Documents, []string{"one.htm", "two.htm"}) {
		t.Errorf("expected the files of the directory, got: %q", src.Documents)
	}
	// fstest compares the sizes of Stat and of a read, which differ for a
	// document that gets a meta element, so these documents are in ASCII
	if err := fstest.TestFS(src.FS, "one.htm", "two.htm"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"two.htm", "one.htm", "two.htm"} {
		data, err := fs.ReadFile(src.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if name == "one.htm" && !bytes.Equal(data, big) {
			t.Error("expected the large file")
		}
	}
	if info, err := d.Describe(t.Context()); err != nil || info.Title != "" {
		t.Errorf("expected no title, got: %+v %v", info, err)
	}
}

// TestWindows reads the title, the table of contents and the default topic
// of #WINDOWS and #STRINGS, and the title of the default topic.
func TestWindows(t *testing.T) {
	strs := []byte("\x00Window Title\x00toc.hhc\x00b.htm\x00")
	window := make([]byte, 196)
	copy(window[0:], le32w(nil, 196))
	copy(window[windowTitle:], le32w(nil, 1))
	copy(window[windowContents:], le32w(nil, 14))
	copy(window[windowDefault:], le32w(nil, 22))
	files := []chmFile{
		{name: "/#WINDOWS", data: slices.Concat(le32w(nil, 1), le32w(nil, 196), window)},
		{name: "/#STRINGS", data: strs},
		{name: "/#SYSTEM", data: systemFile(map[uint16][]byte{systemLCID: lcidEntry(0x0411)})},
		{name: "/a.htm", data: []byte("<title>A</title>")},
		{name: "/b.htm", data: []byte("<title>The B</title>")},
		{name: "/toc.hhc", data: []byte(`<param name="local" value="a.htm">`)},
	}
	d, err := Open(t.Context(), bytes.NewReader(buildCHM(files, chmOptions{lcid: 0x0409})))
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.Describe(t.Context())
	if err != nil || info.Title != "Window Title" || info.Language != "ja" {
		t.Errorf("expected the title of #WINDOWS and the LCID of #SYSTEM, got: %+v %v", info, err)
	}
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(src.Documents, []string{"b.htm", "a.htm"}) {
		t.Errorf("expected the topic of #WINDOWS first, got: %q", src.Documents)
	}

	// Without #WINDOWS, the title is the one of the default topic, and the
	// table of contents is the .hhc file of the help file.
	files = []chmFile{
		{name: "/#SYSTEM", data: systemFile(map[uint16][]byte{systemDefault: cstr("/b.htm")})},
		{name: "/a.htm", data: []byte("<title>A</title>")},
		{name: "/b.htm", data: []byte("<title>The B</title>")},
		{name: "/c.htm", data: []byte("<title>C</title>")},
		{name: "/x.hhc", data: []byte(`<param name="Local" value="c.htm">`)},
	}
	d, err = Open(t.Context(), bytes.NewReader(buildCHM(files, chmOptions{lcid: 0x0407})))
	if err != nil {
		t.Fatal(err)
	}
	info, err = d.Describe(t.Context())
	if err != nil || info.Title != "The B" || info.Language != "de" {
		t.Errorf("expected the title of the topic and the LCID of the header, got: %+v %v", info, err)
	}
	if src, err := d.source(); err != nil || !slices.Equal(src.Documents, []string{"b.htm", "c.htm", "a.htm"}) {
		t.Errorf("expected the order of the .hhc file, got: %q %v", src.Documents, err)
	}
}

func TestWithCharset(t *testing.T) {
	for _, tt := range []struct {
		name, doc, charset, want string
	}{
		{"ascii", "<html><head></head></html>", "shift_jis", "<html><head></head></html>"},
		{"head", "<!DOCTYPE html><html><HEAD lang=x>\xe9", "windows-1250", "<!DOCTYPE html><html><HEAD lang=x><meta charset=\"windows-1250\">\xe9"},
		{"header is not head", "<header>\xe9", "windows-1250", "<meta charset=\"windows-1250\"><header>\xe9"},
		{"html", "<html>\xe9", "big5", "<html><meta charset=\"big5\">\xe9"},
		{"doctype", "<!doctype html>\xe9", "big5", "<!doctype html><meta charset=\"big5\">\xe9"},
		{"declared", "<meta http-equiv=\"Content-Type\" content=\"text/html; charset=gbk\">\xe9", "big5", "<meta http-equiv=\"Content-Type\" content=\"text/html; charset=gbk\">\xe9"},
		{"meta charset", "<meta charset=gbk>\xe9", "big5", "<meta charset=gbk>\xe9"},
		{"windows-1252", "<p>\xe9", "windows-1252", "<p>\xe9"},
		{"utf-8", "<p>caf\xc3\xa9", "windows-1252", "<meta charset=\"utf-8\"><p>caf\xc3\xa9"},
		{"bom", "\xef\xbb\xbf<p>\xe9", "big5", "\xef\xbb\xbf<p>\xe9"},
		{"bytes past ascii before head", "<p>\xe9\xe9\xe9<HEAD>x", "big5", "<p>\xe9\xe9\xe9<HEAD><meta charset=\"big5\">x"},
	} {
		if got := string(withCharset([]byte(tt.doc), tt.charset)); got != tt.want {
			t.Errorf("%s: expected %q, got: %q", tt.name, tt.want, got)
		}
	}
}

func TestFormat(t *testing.T) {
	if Format.Name != "chm" || !slices.Contains(Format.Mimes, "application/vnd.ms-htmlhelp") || !slices.Contains(Format.Exts, "chm") {
		t.Errorf("expected the format of chm, got: %+v", Format)
	}
	d, err := Format.Open(t.Context(), bytes.NewReader(buildCHM(helpFiles(), chmOptions{})), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.(ivy.Pager); !ok {
		t.Error("expected a pager")
	}
	if err := d.Close(); err != nil {
		t.Error(err)
	}
}

func TestErrors(t *testing.T) {
	good := buildCHM(helpFiles(), chmOptions{})
	for _, tt := range []struct {
		name string
		buf  []byte
		lim  ivy.Limits
		want error
	}{
		{"empty", nil, ivy.Limits{}, ivy.ErrFormat},
		{"not chm", bytes.Repeat([]byte("x"), 200), ivy.Limits{}, ivy.ErrFormat},
		{"version", slices.Concat([]byte("ITSF\x09\x00\x00\x00"), good[8:]), ivy.Limits{}, ivy.ErrUnsupported},
		{"truncated", good[:0x70], ivy.Limits{}, ivy.ErrFormat},
		{"objects", good, ivy.Limits{MaxObjects: 3}, ivy.ErrLimit},
		{"directory", good, ivy.Limits{MaxMemory: 64}, ivy.ErrLimit},
	} {
		_, err := Open(t.Context(), bytes.NewReader(tt.buf), WithConfig(ivy.Config{Limits: tt.lim}))
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: expected %v, got: %v", tt.name, tt.want, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(ctx, bytes.NewReader(good)); err == nil {
		t.Error("expected the error of the context")
	}
	if d, err := Open(t.Context(), bytes.NewReader(good)); err == nil {
		if _, err := d.Describe(ctx); err == nil {
			t.Error("expected the error of the context")
		}
	}
}

// TestSectionErrors reads files of section 1 whose control data or reset
// table is damaged.
func TestSectionErrors(t *testing.T) {
	files := []chmFile{{name: "/a.htm", data: []byte("<p>a</p>")}}
	good := buildCHM(files, chmOptions{})
	lzxc := bytes.Index(good, []byte("LZXC"))
	if lzxc < 0 {
		t.Fatal("expected the control data")
	}
	for _, tt := range []struct {
		name  string
		patch func([]byte)
		want  error
	}{
		{"interval", func(b []byte) { copy(b[lzxc+8:], le32w(nil, maxResetInterval+1)) }, ivy.ErrLimit},
		{"zero interval", func(b []byte) { copy(b[lzxc+8:], le32w(nil, 0)) }, ivy.ErrFormat},
		{"lzxc version", func(b []byte) { copy(b[lzxc+4:], le32w(nil, 7)) }, ivy.ErrUnsupported},
		{"window", func(b []byte) { copy(b[lzxc+12:], le32w(nil, 3)) }, ivy.ErrFormat},
		{"signature", func(b []byte) { copy(b[lzxc:], "LZXD") }, ivy.ErrFormat},
		{"reset entry size", func(b []byte) { copy(b[lzxc+24+8:], le32w(nil, 4)) }, ivy.ErrFormat},
	} {
		buf := bytes.Clone(good)
		tt.patch(buf)
		d, err := Open(t.Context(), bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		src, err := d.source()
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if _, err := fs.ReadFile(src.FS, "a.htm"); !errors.Is(err, tt.want) {
			t.Errorf("%s: expected %v, got: %v", tt.name, tt.want, err)
		}
	}
	// a file larger than the limit does not decode
	d, err := Open(t.Context(), bytes.NewReader(buildCHM([]chmFile{{name: "/a.htm", data: make([]byte, 5000)}}, chmOptions{chunkSize: 0x200})),
		WithConfig(ivy.Config{Limits: ivy.Limits{MaxMemory: 4096}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.fsys.ReadFile("a.htm"); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
	// a help file without HTML has no document
	d, err = Open(t.Context(), bytes.NewReader(buildCHM([]chmFile{{name: "/a.txt", data: []byte("a")}}, chmOptions{})))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.source(); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
}

// TestWriteSample writes a help file of the tests to the path in the
// environment variable CHM_SAMPLE, so that a person can read it with 7z or
// extract_chmLib.
func TestWriteSample(t *testing.T) {
	name := os.Getenv("CHM_SAMPLE")
	if name == "" {
		t.Skip("CHM_SAMPLE is not set")
	}
	if err := os.WriteFile(name, buildCHM(helpFiles(), chmOptions{chunkSize: 0x100}), 0o644); err != nil {
		t.Fatal(err)
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(buildCHM(helpFiles(), chmOptions{}))
	f.Add(buildCHM(helpFiles(), chmOptions{chunkSize: 0x100, interval: 1, lzxcVersion: 1, version: 2}))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxObjects: 1000}}
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
		src, err := d.source()
		check(err)
		if err != nil {
			return
		}
		_ = fs.WalkDir(src.FS, ".", func(name string, de fs.DirEntry, err error) error {
			if err == nil && !de.IsDir() {
				_, err := fs.ReadFile(src.FS, name)
				check(err)
			}
			return nil
		})
		ebooktest.RenderFirstPage(t, d)
	})
}

func TestPages(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(buildCHM(helpFiles(), chmOptions{})))
	if err != nil {
		t.Fatal(err)
	}
	// Each of the four documents starts a page.
	n, err := d.PageCount(t.Context())
	if err != nil || n != 4 {
		t.Fatalf("expected 4 pages, got: %d %v", n, err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 612 || h != 792 {
		t.Errorf("expected a page of US Letter, got: %g by %g", w, h)
	}
	// The default topic holds only an image that cannot be decoded. The
	// second page, b.htm, holds "plain text" in its top left corner, inside
	// the margins of 0.75 inch.
	img := ebooktest.RenderPage(t, d, 1)
	if ink := ebooktest.Ink(img, image.Rect(72, 72, 200, 100)); ink < 20 {
		t.Errorf("expected the ink of the text, got %d pixels", ink)
	}
	// The third page, a.htm, holds Russian text.
	img = ebooktest.RenderPage(t, d, 2)
	if ink := ebooktest.Ink(img, image.Rect(72, 72, 400, 100)); ink < 20 {
		t.Errorf("expected the ink of the Russian text, got %d pixels", ink)
	}
	if _, err := d.Page(t.Context(), n); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ivy.ErrPageRange, got: %v", err)
	}
}

func TestPaper(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(buildCHM(helpFiles(), chmOptions{})), WithConfig(ivy.Config{Paper: ebooktest.Landscape}))
	if err != nil {
		t.Fatal(err)
	}
	ebooktest.CheckPaper(t, d)
}
