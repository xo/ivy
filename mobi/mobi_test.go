package mobi

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/ebook/ebooktest"
	"github.com/xo/ivy/internal/layout"
)

// The files of testdata are books that calibre made with ebook-convert from
// book.html, cover.png and pic.png, which the tests wrote:
//
//	ebook-convert book.html book-old.mobi --mobi-file-type old \
//	  --cover cover.png --authors "Ann Author" --title "Sample Book" \
//	  --language de --publisher "A Press" --pubdate 2020-05-12
//
// and the same with --mobi-file-type both for book-both.mobi, and with the
// output book.azw3 and no publisher for book.azw3.

// open opens a book of testdata.
func open(t *testing.T, name string) *Document {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// previews lists the previews of d, and decodes each one.
func previews(t *testing.T, d *Document) []ivy.Preview {
	t.Helper()
	var out []ivy.Preview
	for p, err := range d.Previews(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		img, err := p.Decode(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Size() != p.Size {
			t.Errorf("expected the size %v, got: %v", p.Size, img.Bounds().Size())
		}
		out = append(out, p)
	}
	return out
}

// readFile reads the file name of the source of d.
func readFile(t *testing.T, fsys fs.FS, name string) string {
	t.Helper()
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCalibre(t *testing.T) {
	for _, tt := range []struct {
		name string
		kind kind
	}{
		{"book-old.mobi", mobi6},
		{"book-both.mobi", kf8},
		{"book.azw3", kf8},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := open(t, tt.name)
			if d.kind != tt.kind {
				t.Errorf("expected the kind %v, got: %v", tt.kind, d.kind)
			}
			info, err := d.Describe(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if info.Title != "Sample Book" || !slices.Equal(info.Authors, []string{"Ann Author"}) || info.Language != "de" ||
				info.Created.Year() != 2020 || !strings.HasPrefix(info.Producer, "kindlegen (Linux) ") ||
				// calibre writes the UUID of the book in record 113
				len(info.Identifier) != 36 {
				t.Errorf("expected the metadata of the book, got: %+v", info)
			}
			got := previews(t, d)
			if len(got) != 2 || got[0].Kind != ivy.Cover || got[0].Size != image.Pt(60, 90) || got[1].Kind != ivy.Thumbnail {
				t.Errorf("expected the cover and the thumbnail, got: %+v", got)
			}
			src, err := d.source()
			if err != nil {
				t.Fatal(err)
			}
			wantTOC := []layout.TOCEntry{
				{Label: "Chapter One", Level: 1, Target: "part0000.xhtml#c1"},
				{Label: "Chapter Two", Level: 1, Target: "part0001.xhtml#c2"},
			}
			if d.kind == mobi6 {
				// the anchors of the targets are in the text, as the test of
				// the document below finds them
				wantTOC[0].Target, wantTOC[1].Target = "index.html#filepos109", "index.html#filepos501"
				doc := readFile(t, src.FS, "index.html")
				if !strings.Contains(doc, `<a id="filepos109"></a>`) || !strings.Contains(doc, `<a id="filepos501"></a>`) {
					t.Errorf("expected the anchors of the table of contents, got: %s", doc)
				}
			}
			if !slices.Equal(src.TOC, wantTOC) {
				t.Errorf("expected the table of contents %+v, got: %+v", wantTOC, src.TOC)
			}
			// a MOBI 6 book adds the style sheet of its HTML
			sheets := 2
			if d.kind == mobi6 {
				sheets = 3
			}
			if src.Title != "Sample Book" || len(src.StyleSheets) != sheets || src.StyleSheets[0] != ebook.BookStyle {
				t.Errorf("expected the title and the style sheets, got: %q %d", src.Title, len(src.StyleSheets))
			}
			name := "resource00001.jpg"
			if tt.name == "book.azw3" {
				// a KF8 book of calibre keeps the PNG image
				name = "resource00001.png"
			}
			if img := readFile(t, src.FS, name); !strings.HasPrefix(img, "\xff\xd8") && !strings.HasPrefix(img, "\x89PNG") {
				t.Errorf("expected the image of the book, got: %q", img[:min(len(img), 8)])
			}
			if tt.kind == mobi6 {
				if !slices.Equal(src.Documents, []string{"cover-page.xhtml", "index.html"}) {
					t.Fatalf("expected one document, got: %q", src.Documents)
				}
				doc := readFile(t, src.FS, "index.html")
				for _, want := range []string{
					"\uFEFF<html>",
					`<a id="filepos`,
					`<a href="#filepos`,
					`<img src="resource00001.jpg"`,
					`<div class="mbp-pagebreak"></div>`,
					"Café naïve",
				} {
					if !strings.Contains(doc, want) {
						t.Errorf("expected %q in the document, got: %s", want, doc)
					}
				}
				return
			}
			want := []string{"cover-page.xhtml", "part0000.xhtml", "part0001.xhtml", "part0002.xhtml"}
			if !slices.Equal(src.Documents, want) {
				t.Fatalf("expected the parts %q, got: %q", want, src.Documents)
			}
			part := readFile(t, src.FS, "part0000.xhtml")
			for _, want := range []string{
				`<h1 id="c1"`,
				`href="flow0001.css"`,
				`<a href="part0001.xhtml#c2"`,
				`<img src="resource00001.`,
			} {
				if !strings.Contains(part, want) {
					t.Errorf("expected %q in the first part, got: %s", want, part)
				}
			}
			if strings.Contains(part, "kindle:") {
				t.Errorf("expected no link of kindle, got: %s", part)
			}
			if css := readFile(t, src.FS, "flow0001.css"); !strings.Contains(css, ".x {") {
				t.Errorf("expected the style sheet of the book, got: %s", css)
			}
			if second := readFile(t, src.FS, "part0001.xhtml"); !strings.Contains(second, "Café naïve") {
				t.Errorf("expected the second chapter, got: %s", second)
			}
		})
	}
}

// pixel returns a PNG image of w by h pixels.
func pixel(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// mobiText is the text of a MOBI book in windows-1252, longer than one text
// record.
func mobiText() []byte {
	var b bytes.Buffer
	b.WriteString(`<html><head><guide><reference type="toc" title="Contents" filepos=0000000000 /></guide></head><body>`)
	b.WriteString(`<p>Caf` + "\xe9" + ` <a filepos="0000004500">to the end</a></p><img recindex="00002" hirecindex=00002><img recindex="00009"><mbp:pagebreak/>`)
	for b.Len() < 4500 {
		b.WriteString("<p>A paragraph of the book, which the writer repeats.</p>\n")
	}
	b.WriteString("<p>The end.</p></body></html>")
	return b.Bytes()
}

func TestMOBI(t *testing.T) {
	text := mobiText()
	buf := testBook{
		encoding:   encodingCP1252,
		extraFlags: 3,
		text:       text,
		locale:     0x0409,
		fullName:   "Full Name",
		images:     [][]byte{pixel(t, 3, 4), pixel(t, 5, 6)},
		exth: []exthRecord{
			exthText(exthAuthor, "Ann"),
			exthText(exthAuthor, "Bob"),
			exthText(exthSubject, "Tests"),
			exthText(exthSubject, "Books"),
			exthText(exthUpdated, "2021-01-02"),
			exthNumber(exthCover, 1),
			exthNumber(exthThumbnail, 1),
			exthNumber(exthCreator, 2),
			exthText(exthPublisher, "A Press"),
			exthText(exthASIN, "B000000000"),
			exthText(exthISBN, "9780000000000"),
		},
	}.build()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := ivy.Info{
		Title:      "Full Name",
		Authors:    []string{"Ann", "Bob"},
		Subject:    "Tests, Books",
		Language:   "en-US",
		Modified:   time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
		Producer:   "Mobipocket Creator",
		Publisher:  "A Press",
		Identifier: "9780000000000",
	}
	if info.Title != want.Title || !slices.Equal(info.Authors, want.Authors) || info.Subject != want.Subject ||
		info.Language != want.Language || !info.Modified.Equal(want.Modified) || info.Producer != want.Producer ||
		info.Publisher != want.Publisher || info.Identifier != want.Identifier {
		t.Errorf("expected %+v, got: %+v", want, info)
	}
	got := previews(t, d)
	if len(got) != 1 || got[0].Kind != ivy.Cover || got[0].Size != image.Pt(5, 6) {
		t.Errorf("expected one cover of 5 by 6, got: %+v", got)
	}
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	doc := readFile(t, src.FS, "index.html")
	for _, want := range []string{
		// the guide links to parts of the book and shows nothing
		"<html><head></head><body>",
		"\uFEFF<a id=\"filepos0\"></a><html>",
		"<p>Café",
		`<a href="#filepos4500">`,
		`<img src="resource00002.png" src="resource00002.png">`,
		`<img recindex="00009">`,
		`<div class="mbp-pagebreak"></div>`,
		`<a id="filepos4500"></a>`,
		"<p>The end.</p></body></html>",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected %q in the document", want)
		}
	}
	if strings.Count(doc, "<p>A paragraph") != strings.Count(string(text), "<p>A paragraph") {
		t.Error("expected each paragraph of the text")
	}
	if data := readFile(t, src.FS, "resource00002.png"); data != string(pixel(t, 5, 6)) {
		t.Error("expected the image of the book")
	}
	again, err := d.source()
	if err != nil || again.FS != src.FS {
		t.Errorf("expected the same source, got: %v", err)
	}
}

func TestIdentifier(t *testing.T) {
	for name, tt := range map[string]struct {
		exth []exthRecord
		want string
	}{
		"isbn": {[]exthRecord{exthText(exthISBN, " 9780000000000 "), exthText(exthASIN, "B000000000")}, "9780000000000"},
		"asin": {[]exthRecord{exthText(exthISBN, " "), exthText(exthASIN, "B000000000")}, "B000000000"},
		"none": {nil, ""},
	} {
		t.Run(name, func(t *testing.T) {
			buf := testBook{text: []byte("<p>A</p>"), exth: tt.exth, fullName: "Name"}.build()
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			info, err := d.Describe(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if info.Identifier != tt.want || info.Publisher != "" {
				t.Errorf("expected the identifier %q and no publisher, got: %q %q", tt.want, info.Identifier, info.Publisher)
			}
		})
	}
}

func TestHuffCDIC(t *testing.T) {
	huff, cdic := huffBook(map[byte][]byte{
		0x01: []byte("Hello, "),
		// a phrase of codes: code 0x01 and then "!"
		0x02: []byte("\x01!"),
	}, map[byte]bool{0x02: true})
	buf := testBook{
		compression: compressionHuffCDIC,
		extraFlags:  2,
		text:        make([]byte, 30),
		records:     [][]byte{[]byte("<p>\x01World</p>"), []byte("<p>\x02</p>")},
		huff:        [][]byte{huff, cdic},
	}.build()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	if doc := readFile(t, src.FS, "index.html"); doc != "\uFEFF<p>Hello, World</p><p>Hello, !</p>" {
		t.Errorf("expected the decoded text, got: %q", doc)
	}
	if info, err := d.Describe(t.Context()); err != nil || info.Title != "Test Database" {
		t.Errorf("expected the name of the database, got: %+v %v", info, err)
	}
}

func TestPalmDOC(t *testing.T) {
	buf := testBook{kind: kindPalmDOC, compression: compressionNone, encryption: 7, text: []byte("One <two> & caf\xe9")}.build()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if d.kind != palmDOC {
		t.Errorf("expected palmdoc, got: %v", d.kind)
	}
	src, err := d.source()
	if err != nil {
		t.Fatal(err)
	}
	if doc := readFile(t, src.FS, "index.html"); doc != "\uFEFF<html><body><pre class=\"palmdoc\">One &lt;two&gt; &amp; café</pre></body></html>" {
		t.Errorf("expected the text as html, got: %q", doc)
	}
	if got := previews(t, d); len(got) != 0 {
		t.Errorf("expected no preview, got: %+v", got)
	}
}

func TestDRM(t *testing.T) {
	buf := testBook{
		encryption: 2,
		text:       []byte("<p>secret</p>"),
		images:     [][]byte{pixel(t, 2, 2)},
		exth:       []exthRecord{exthText(exthTitle, "Locked"), exthNumber(exthCover, 0)},
	}.build()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if info, err := d.Describe(t.Context()); err != nil || info.Title != "Locked" {
		t.Errorf("expected the metadata, got: %+v %v", info, err)
	}
	if got := previews(t, d); len(got) != 1 {
		t.Errorf("expected the cover, got: %+v", got)
	}
	if _, err := d.source(); !errors.Is(err, ivy.ErrDRM) {
		t.Errorf("expected ivy.ErrDRM, got: %v", err)
	}
	if _, err := d.source(); !errors.Is(err, ivy.ErrDRM) {
		t.Errorf("expected ivy.ErrDRM again, got: %v", err)
	}
}

func TestReplica(t *testing.T) {
	text := append([]byte(replicaMagic), make([]byte, 20)...)
	for _, compression := range []int{compressionNone, compressionPalmDoc} {
		buf := testBook{compression: compression, text: text, exth: []exthRecord{exthText(exthTitle, "Replica")}}.build()
		d, err := Open(t.Context(), bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		if d.kind != replica {
			t.Errorf("expected print replica, got: %v", d.kind)
		}
		if info, err := d.Describe(t.Context()); err != nil || info.Title != "Replica" {
			t.Errorf("expected the metadata, got: %+v %v", info, err)
		}
		if _, err := d.source(); !errors.Is(err, ivy.ErrUnsupported) {
			t.Errorf("expected ivy.ErrUnsupported, got: %v", err)
		}
	}
}

func TestErrors(t *testing.T) {
	good := testBook{text: []byte("<p>text</p>")}.build()
	for _, tt := range []struct {
		name string
		buf  []byte
		lim  ivy.Limits
		want error
	}{
		{"empty", nil, ivy.Limits{}, ivy.ErrFormat},
		{"not a book", bytes.Repeat([]byte("x"), 100), ivy.Limits{}, ivy.ErrFormat},
		{"no record", slices.Concat(good[:pdbCountAt], []byte{0, 0}), ivy.Limits{}, ivy.ErrFormat},
		{"records", good, ivy.Limits{MaxObjects: 2}, ivy.ErrLimit},
		{"truncated", good[:pdbHeaderSize+4], ivy.Limits{}, ivy.ErrFormat},
		{"record 0", good, ivy.Limits{MaxMemory: 16}, ivy.ErrLimit},
		{"no mobi header", slices.Concat(good[:pdbHeaderSize+3*pdbEntrySize+2], make([]byte, 20)), ivy.Limits{}, ivy.ErrFormat},
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
	d, err := Open(t.Context(), bytes.NewReader(good))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Describe(ctx); err == nil {
		t.Error("expected the error of the context")
	}
	for _, err := range d.Previews(ctx) {
		if err == nil {
			t.Error("expected the error of the context")
		}
	}

	// the text is over the limit
	big := testBook{text: bytes.Repeat([]byte("<p>text</p>"), 1000)}.build()
	d, err = Open(t.Context(), bytes.NewReader(big), WithConfig(ivy.Config{Limits: ivy.Limits{MaxMemory: 5000}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.source(); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}

	// a compression that the package does not know
	d, err = Open(t.Context(), bytes.NewReader(testBook{compression: 3, text: []byte("x")}.build()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.source(); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("expected ivy.ErrUnsupported, got: %v", err)
	}

	// HUFF/CDIC without its records
	d, err = Open(t.Context(), bytes.NewReader(testBook{compression: compressionHuffCDIC, text: []byte("x")}.build()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.source(); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
}

func TestTrimTrailing(t *testing.T) {
	for _, tt := range []struct {
		rec   string
		flags uint16
		want  string
	}{
		{"text", 0, "text"},
		{"textX\x01", 1, "text"},
		{"text\x00", 1, "text"},
		{"text\x05\x06\x83", 2, "text"},
		{"textX\x01\x05\x06\x83", 3, "text"},
		{"text\x00\x82\x81", 6, "text"},
		{"text" + strings.Repeat("y", 0x100-2) + "\x82\x00", 2, "text"},
		{"\x90", 2, ""},
		{"\x03", 1, ""},
		{"", 3, ""},
	} {
		if got := string(trimTrailing([]byte(tt.rec), tt.flags)); got != tt.want {
			t.Errorf("trimTrailing(%q, %d): expected %q, got: %q", tt.rec, tt.flags, tt.want, got)
		}
	}
}

func TestDecodeFont(t *testing.T) {
	font := []byte("\x00\x01\x00\x00 a font of the tests")
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	if _, err := zw.Write(font); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	key := []byte{0x5a, 0xa5}
	data := bytes.Clone(z.Bytes())
	for i := range data {
		data[i] ^= key[i%len(key)]
	}
	be := func(v int) []byte { return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)} }
	rec := slices.Concat([]byte("FONT"), be(len(font)), be(fontZlib|fontXOR), be(fontHeader+len(key)), be(len(key)), be(fontHeader), key, data)
	got, err := decodeFont(rec, ivy.Limits{}.WithDefaults())
	if err != nil || !bytes.Equal(got, font) {
		t.Errorf("expected the font, got: %q %v", got, err)
	}
	if _, err := decodeFont(rec, ivy.Limits{MaxMemory: 10}.WithDefaults()); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
	if _, err := decodeFont(rec[:10], ivy.Limits{}.WithDefaults()); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
	if ext, ok := resourceType(rec); !ok || ext != "ttf" {
		t.Errorf("expected a font, got: %q %v", ext, ok)
	}
	lim := ivy.Limits{}.WithDefaults()
	if ext := fontType(rec, lim); ext != "ttf" {
		t.Errorf("expected a TrueType font, got: %q", ext)
	}
	for magic, want := range map[string]string{"OTTO": "otf", "wOFF": "woff", "wOF2": "woff2", "true": "ttf"} {
		rec := slices.Concat([]byte("FONT"), be(4), be(0), be(fontHeader), be(0), be(0), []byte(magic))
		if ext := fontType(rec, lim); ext != want {
			t.Errorf("%s: expected %q, got: %q", magic, want, ext)
		}
		if !isFont("resource00001." + want) {
			t.Errorf("expected resource00001.%s to be a font", want)
		}
	}
	if ext := fontType(rec[:10], lim); ext != "ttf" {
		t.Errorf("expected ttf for a broken record, got: %q", ext)
	}
}

func TestBase32(t *testing.T) {
	for s, want := range map[string]int{"0001": 1, "000V": 31, "0010": 32, "00a": 10, "W": -1} {
		if got := base32(s); got != want {
			t.Errorf("base32(%q): expected %d, got: %d", s, want, got)
		}
	}
}

func TestFormat(t *testing.T) {
	if Format.Name != "mobi" || !slices.Contains(Format.Mimes, "application/x-mobipocket-ebook") || !slices.Contains(Format.Exts, "azw3") {
		t.Errorf("expected the format of mobi, got: %+v", Format)
	}
	d, err := Format.Open(t.Context(), bytes.NewReader(testBook{text: []byte("x")}.build()), ivy.Config{})
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

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"book-old.mobi", "book-both.mobi", "book.azw3"} {
		buf, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(buf)
	}
	huff, cdic := huffBook(map[byte][]byte{0x01: []byte("ab")}, nil)
	f.Add(testBook{compression: compressionHuffCDIC, records: [][]byte{[]byte("\x01x")}, huff: [][]byte{huff, cdic}}.build())
	f.Add(testBook{encoding: encodingCP1252, extraFlags: 3, text: mobiText()}.build())
	f.Add(testBook{kind: kindPalmDOC, text: []byte("plain text")}.build())
	f.Add(testBook{text: replicaText(onePagePDF())}.build())
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
		ebooktest.RenderFirstPage(t, d)
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
	})
}

func TestReadEntry(t *testing.T) {
	rec := slices.Concat(make([]byte, 8), []byte("TAGX"), []byte{0, 0, 0, 32, 0, 0, 0, 2},
		[]byte{1, 1, 0x03, 0}, []byte{6, 2, 0x0c, 0}, []byte{0, 0, 0, 1}, []byte{3, 1, 0x01, 0}, []byte{0, 0, 0, 1})
	tags, controls, err := readTAGX(rec, 8)
	if err != nil || len(tags) != 5 || controls != 2 {
		t.Fatalf("expected the TAGX, got: %v %d %v", tags, controls, err)
	}
	// tag 1 has one value, tag 6 gives the size of its values, 2 bytes, and
	// tag 3 of the second control byte has one value
	entry := []byte{4, 'S', 'K', 'E', 'L', 0x0d, 0x01, 0x82, 0x82, 0x8a, 0x94, 0x87}
	e, err := readEntry(entry, tags, controls)
	if err != nil {
		t.Fatal(err)
	}
	if e.label != "SKEL" || !slices.Equal(e.tags[1], []uint32{2}) || !slices.Equal(e.tags[6], []uint32{10, 20}) || !slices.Equal(e.tags[3], []uint32{7}) {
		t.Errorf("expected the values of the tags, got: %+v", e)
	}
	if _, err := readEntry([]byte{9, 'x'}, tags, controls); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
	if _, _, err := readTAGX(rec, 9); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
	if v, n := vwi([]byte{0x01, 0x02}); n != 0 || v != 0 {
		t.Errorf("expected no number, got: %d %d", v, n)
	}
}

func TestAssemble(t *testing.T) {
	text := []byte("<html><body></body></html><p>one</p><p>two</p>")
	skeleton := indexEntry{label: "SKEL0000000000", tags: map[byte][]uint32{tagCount: {2}, tagGeometry: {0, 26}}}
	fragments := []indexEntry{
		{label: "0000000012", tags: map[byte][]uint32{tagGeometry: {0, 10}}},
		{label: "0000000022", tags: map[byte][]uint32{tagGeometry: {10, 10}}},
	}
	b := &kf8Book{flows: [][]byte{text}}
	if err := b.assemble([]indexEntry{skeleton}, fragments, ivy.Limits{}.WithDefaults()); err != nil {
		t.Fatal(err)
	}
	if len(b.parts) != 1 || string(b.parts[0]) != "<html><body><p>one</p><p>two</p></body></html>" {
		t.Errorf("expected the part, got: %q", b.parts)
	}
	if !slices.Equal(b.fragments, []fragmentPlace{{0, 12}, {0, 22}}) {
		t.Errorf("expected the places of the fragments, got: %v", b.fragments)
	}
	l := &links{book: b, parts: []string{"part0000.xhtml"}}
	if got := l.position(1, 0); got != "part0000.xhtml" {
		t.Errorf("expected the part, got: %q", got)
	}
	b.parts[0] = []byte(`<html><body><p>one</p><p id="x">two</p></body></html>`)
	if got := l.position(1, 0); got != "part0000.xhtml#x" {
		t.Errorf("expected the id, got: %q", got)
	}
	if got := l.position(5, 0); got != "#" {
		t.Errorf("expected no part, got: %q", got)
	}
	for _, bad := range []indexEntry{
		{label: "SKEL", tags: map[byte][]uint32{tagCount: {1}, tagGeometry: {0, 1000}}},
		{label: "SKEL", tags: map[byte][]uint32{tagCount: {1}}},
	} {
		b := &kf8Book{flows: [][]byte{text}}
		if err := b.assemble([]indexEntry{bad}, fragments, ivy.Limits{}.WithDefaults()); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("expected ivy.ErrFormat, got: %v", err)
		}
	}
	// a skeleton that reuses the whole text many times goes over the limit
	many := slices.Repeat([]indexEntry{{label: "SKEL", tags: map[byte][]uint32{tagGeometry: {0, uint32(len(text))}}}}, 10)
	if err := (&kf8Book{flows: [][]byte{text}}).assemble(many, nil, ivy.Limits{MaxMemory: 100}.WithDefaults()); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
}

func TestPages(t *testing.T) {
	for _, name := range []string{"book-old.mobi", "book-both.mobi", "book.azw3"} {
		t.Run(name, func(t *testing.T) {
			d := open(t, name)
			n, err := d.PageCount(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			// The page of the cover comes first, and the page break before
			// the second chapter makes two more pages.
			if n < 3 {
				t.Errorf("expected 3 pages or more, got: %d", n)
			}
			p, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			if w, h := p.Size(); w != 6*72 || h != 9*72 {
				t.Errorf("expected a page of 6 by 9 inches, got: %g by %g", w, h)
			}
			// The cover is blue, and it fills the page.
			if ink := ebooktest.Ink(ebooktest.RenderPage(t, d, 0), image.Rect(0, 0, 576, 864)); ink < 576*864*9/10 {
				t.Errorf("expected the cover on the first page, got %d pixels of ink", ink)
			}
			img := ebooktest.RenderPage(t, d, 1)
			// "Chapter One" is at the top of the page area. The KF8 books
			// have a page margin of their own, so the area starts at 20
			// pixels.
			if ink := ebooktest.Ink(img, image.Rect(30, 20, 528, 120)); ink < 50 {
				t.Errorf("expected the ink of the heading, got %d pixels", ink)
			}
			if ink := ebooktest.Ink(img, image.Rect(0, 0, 576, 20)); ink != 0 {
				t.Errorf("expected no ink in the top margin, got %d pixels", ink)
			}
			// The second chapter starts the third page.
			second := ebooktest.RenderPage(t, d, 2)
			if ink := ebooktest.Ink(second, image.Rect(30, 20, 528, 200)); ink < 50 {
				t.Errorf("expected the ink of the second chapter, got %d pixels", ink)
			}
			if _, err := d.Page(t.Context(), n); !errors.Is(err, ivy.ErrPageRange) {
				t.Errorf("expected ivy.ErrPageRange, got: %v", err)
			}
		})
	}
}

func TestPagesDRM(t *testing.T) {
	buf := testBook{encryption: 2, text: []byte("<p>secret</p>")}.build()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Page(t.Context(), 0); !errors.Is(err, ivy.ErrDRM) {
		t.Errorf("expected ivy.ErrDRM, got: %v", err)
	}
}

// replicaText returns the text of a Print Replica book: the header, and for
// each file of pdfs a table of two sections, the file and a resource.
func replicaText(pdfs ...[]byte) []byte {
	resource := []byte("resource")
	header := 4 + 4 + len(pdfs)*(4+2*8)
	values := make([]int, 0, 1+5*len(pdfs))
	values = append(values, len(pdfs))
	for range pdfs {
		values = append(values, 2)
	}
	offset := header
	for _, pdf := range pdfs {
		values = append(values, offset, len(pdf), offset+len(pdf), len(resource))
		offset += len(pdf) + len(resource)
	}
	var b bytes.Buffer
	b.WriteString(replicaMagic)
	for _, v := range values {
		b.Write([]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
	}
	for _, pdf := range pdfs {
		b.Write(pdf)
		b.Write(resource)
	}
	return b.Bytes()
}

// onePagePDF returns a PDF file of one page of 100 by 200 points, with a
// black square in its top left corner.
func onePagePDF() []byte {
	content := "0 0 0 rg 10 150 40 40 re f"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 200] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

func TestReplicaPages(t *testing.T) {
	text := replicaText(onePagePDF())
	for _, compression := range []int{compressionNone, compressionPalmDoc} {
		buf := testBook{compression: compression, text: text}.build()
		d, err := Open(t.Context(), bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		if d.kind != replica {
			t.Fatalf("expected print replica, got: %v", d.kind)
		}
		n, err := d.PageCount(t.Context())
		if err != nil || n != 1 {
			t.Fatalf("expected 1 page, got: %d %v", n, err)
		}
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			t.Fatal(err)
		}
		if w, h := p.Size(); w != 100 || h != 200 {
			t.Errorf("expected the page of the pdf, got: %g by %g", w, h)
		}
		img := ebooktest.RenderPage(t, d, 0)
		// The square of 40 points at (10, 10) from the top is at 96 dots
		// per inch from (13, 13) to (67, 67).
		if ink := ebooktest.Ink(img, image.Rect(16, 16, 64, 64)); ink != 48*48 {
			t.Errorf("expected the black square, got %d pixels", ink)
		}
		if _, err := d.Page(t.Context(), 1); !errors.Is(err, ivy.ErrPageRange) {
			t.Errorf("expected ivy.ErrPageRange, got: %v", err)
		}
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}
}

func TestReplicaPagesOfEachFile(t *testing.T) {
	buf := testBook{text: replicaText(onePagePDF(), twoPagePDF())}.build()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	n, err := d.PageCount(t.Context())
	if err != nil || n != 3 {
		t.Fatalf("expected the 3 pages of the two pdf files, got: %d %v", n, err)
	}
	for i, want := range [][2]float32{{100, 200}, {300, 400}, {300, 400}} {
		p, err := d.Page(t.Context(), i)
		if err != nil {
			t.Fatal(err)
		}
		if w, h := p.Size(); w != want[0] || h != want[1] {
			t.Errorf("page %d: expected %g by %g, got: %g by %g", i, want[0], want[1], w, h)
		}
	}
	for _, i := range []int{-1, 3} {
		if _, err := d.Page(t.Context(), i); !errors.Is(err, ivy.ErrPageRange) {
			t.Errorf("page %d: expected ivy.ErrPageRange, got: %v", i, err)
		}
	}
	// a pdf file that does not open fails the book
	buf = testBook{text: replicaText(onePagePDF(), []byte("%PDF-1.7 broken"))}.build()
	broken, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broken.PageCount(t.Context()); err == nil {
		t.Error("expected an error for the broken pdf file")
	}
}

// twoPagePDF returns a PDF file of two empty pages of 300 by 400 points.
func twoPagePDF() []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 /MediaBox [0 0 300 400] >>",
		"<< /Type /Page /Parent 2 0 R >>",
		"<< /Type /Page /Parent 2 0 R >>",
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

func TestReplicaFiles(t *testing.T) {
	pdf := onePagePDF()
	lim := ivy.Limits{}.WithDefaults()
	if got, err := replicaFiles(replicaText(pdf), lim); err != nil || len(got) != 1 || !bytes.Equal(got[0], pdf) {
		t.Errorf("expected the pdf file, got: %d files %v", len(got), err)
	}
	got, err := replicaFiles(replicaText(pdf, []byte("not a pdf"), pdf), lim)
	if err != nil || len(got) != 2 || !bytes.Equal(got[0], pdf) || !bytes.Equal(got[1], pdf) {
		t.Errorf("expected the two pdf files, got: %d files %v", len(got), err)
	}
	for name, text := range map[string][]byte{
		"short":        []byte(replicaMagic),
		"no tables":    append([]byte(replicaMagic), 0, 0, 0, 0),
		"many tables":  append([]byte(replicaMagic), 0xff, 0xff, 0xff, 0xff),
		"past the end": append([]byte(replicaMagic), 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 20, 0, 0, 1, 0),
		"no pdf":       replicaText([]byte("not a pdf")),
		"no section":   append([]byte(replicaMagic), 0, 0, 0, 1, 0, 0, 0, 2),
	} {
		if _, err := replicaFiles(text, lim); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%s: expected ivy.ErrFormat, got: %v", name, err)
		}
	}
	buf := testBook{text: append([]byte(replicaMagic), 0, 0, 0, 0)}.build()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Page(t.Context(), 0); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
	if _, err := d.PageCount(t.Context()); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat again, got: %v", err)
	}
}

func TestPaper(t *testing.T) {
	for _, name := range []string{"book-old.mobi", "book-both.mobi", "book.azw3"} {
		t.Run(name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Paper: ebooktest.Landscape}))
			if err != nil {
				t.Fatal(err)
			}
			ebooktest.CheckPaper(t, d)
		})
	}
}

func TestMOBI6Attrs(t *testing.T) {
	for _, tt := range []struct {
		tag, want string
	}{
		{`<p height="1em" width="-14pt">`, `<p height="1em" width="-14pt" style="margin-top: 1em; text-indent: -14pt; margin-left: 14pt">`},
		{`<p width="2em"/>`, `<p width="2em" style="text-indent: 2em"/>`},
		{`<blockquote height="0pt">`, `<blockquote height="0pt" style="margin-top: 0pt">`},
		{`<font size="7">`, `<font size="7" style="font-size: 22pt">`},
		{`<font size="+1">`, `<font size="+1" style="font-size: 13.5pt">`},
		{`<font size="-9">`, `<font size="-9" style="font-size: 6pt">`},
		{`<font size="x">`, `<font size="x">`},
		{`<p width="1em" style="color: red">`, `<p width="1em" style="color: red">`},
		{`<p width="calc(1em)">`, `<p width="calc(1em)">`},
		{`<hr width="50%" size="2">`, `<hr size="2">`},
		{`<td width="10">`, `<td width="10">`},
		{`</p>`, `</p>`},
	} {
		if got := mobi6Attrs(tt.tag); got != tt.want {
			t.Errorf("mobi6Attrs(%q): expected %q, got: %q", tt.tag, tt.want, got)
		}
	}
}

func TestSVGPageSize(t *testing.T) {
	doc := `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="100%" height="50%" viewBox="0 0 10 10"><rect width="100%"/></svg>`
	want := `<svg xmlns="http://www.w3.org/2000/svg" width="360.00pt" height="280.80pt" viewBox="0 0 10 10"><rect width="100%"/></svg>`
	if got := svgPageSize(doc, ivy.Paper{}); !strings.HasSuffix(got, want) {
		t.Errorf("expected the size of the page area of an ebook, got: %s", got)
	}
	want = `width="769.89pt"`
	if got := svgPageSize(doc, ivy.A4.Landscape()); !strings.Contains(got, want) {
		t.Errorf("expected the width of the page area of A4 in landscape, got: %s", got)
	}
}

func TestNCXOrder(t *testing.T) {
	entry := func(label string, level, parent int) ncxEntry {
		return ncxEntry{label: label, level: level, pos: -1, fid: -1, off: -1, parent: parent}
	}
	// KF8 lists the entries of each depth after those of the depth above
	flat := []ncxEntry{
		entry("A", 1, -1), entry("B", 1, -1),
		entry("A1", 2, 0), entry("A2", 2, 0), entry("B1", 2, 1),
		entry("A1a", 3, 2),
		// a cycle, which does not lead to a root
		entry("X", 2, 7), entry("Y", 2, 6),
	}
	labels := func(entries []ncxEntry) []string {
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.label)
		}
		return out
	}
	want := []string{"A", "A1", "A1a", "A2", "B", "B1", "X", "Y"}
	if got := labels(ncxOrder(flat, 64)); !slices.Equal(got, want) {
		t.Errorf("expected %q, got: %q", want, got)
	}
	// past the depth, an entry keeps its place after the tree
	want = []string{"A", "A1", "A2", "B", "B1", "A1a", "X", "Y"}
	if got := labels(ncxOrder(flat, 2)); !slices.Equal(got, want) {
		t.Errorf("expected %q, got: %q", want, got)
	}
	// without parents, the order is the order of the index
	plain := []ncxEntry{entry("B", 1, -1), entry("A", 2, -1)}
	if got := labels(ncxOrder(plain, 64)); !slices.Equal(got, []string{"B", "A"}) {
		t.Errorf("expected the order of the index, got: %q", got)
	}
}
