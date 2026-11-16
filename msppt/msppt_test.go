package msppt

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/magic/cfb"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// msoPassword is the password of the test files of msoffcrypto-tool.
const msoPassword = "Password1234_"

func readFile(t testing.TB, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

// slides returns the HTML documents that d makes for the layout.
func slides(t *testing.T, d *Document) []string {
	t.Helper()
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(src.Documents))
	for _, name := range src.Documents {
		buf, err := fs.ReadFile(src.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(buf))
	}
	return out
}

func TestOpen(t *testing.T) {
	d, err := Format.Open(t.Context(), bytes.NewReader(readFile(t, "slides.ppt")), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	doc := d.(*Document)
	info, err := doc.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Ivy test slides" || len(info.Authors) != 1 || info.Authors[0] != "Ivy Tester" {
		t.Errorf("expected the title and the author, got: %+v", info)
	}
	docs := slides(t, doc)
	if len(docs) != 2 {
		t.Fatalf("expected 2 slides, got: %d", len(docs))
	}
	for _, want := range []string{
		"size: 793.75pt 446.50pt;",
		`<div class="shape title" style="left: 39.62pt; top: 17.75pt; width: 714.25pt"><p>First slide title</p></div>`,
		`<div class="shape center-body"`,
		"<p>A subtitle</p>",
	} {
		if !strings.Contains(docs[0], want) {
			t.Errorf("expected %q in the first slide:\n%s", want, docs[0])
		}
	}
	for _, want := range []string{
		"<p>Second slide</p>",
		"<ul><li>First point</li><li>Second point</li><li>Third point</li></ul>",
		`<div class="shape other" style="left: 56.75pt; top: 425.25pt; width: 283.50pt"><p>A free text box</p></div>`,
	} {
		if !strings.Contains(docs[1], want) {
			t.Errorf("expected %q in the second slide:\n%s", want, docs[1])
		}
	}
	n, err := doc.PageCount(t.Context())
	if err != nil || n != 2 {
		t.Fatalf("expected 2 pages, got: %d, %v", n, err)
	}
	p, err := doc.Page(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 793.75 || h != 446.5 {
		t.Errorf("expected a page of 793.75x446.5, got: %gx%g", w, h)
	}
	if _, err := ivy.RenderImage(t.Context(), p, 36, color.White); err != nil {
		t.Fatal(err)
	}
}

func TestPowerPoint(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(readFile(t, "plain.ppt")))
	if err != nil {
		t.Fatal(err)
	}
	docs := slides(t, d)
	if len(docs) != 1 || !strings.Contains(docs[0], `<div class="shape center-title" style="left: 120.00pt; top: 88.38pt; width: 720.00pt"><p>Lorem ipsum</p></div>`) {
		t.Errorf("expected one slide with its title, got:\n%s", docs)
	}
	if !strings.Contains(docs[0], "size: 960.00pt 540.00pt;") {
		t.Errorf("expected a slide of 16:9, got:\n%s", docs[0])
	}
}

func TestPassword(t *testing.T) {
	buf := readFile(t, "rc4cryptoapi_password.ppt")
	if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword without a password, got: %v", err)
	}
	if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: "wrong"})); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword for a wrong password, got: %v", err)
	}
	cfg := ivy.Config{PasswordFunc: func(_ context.Context, attempt int) (string, error) {
		if attempt < 2 {
			return "wrong", nil
		}
		return msoPassword, nil
	}}
	d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if docs := slides(t, d); len(docs) != 1 || !strings.Contains(docs[0], "<p>Lorem ipsum</p>") {
		t.Errorf("expected the slide of the encrypted file, got:\n%s", docs)
	}
}

// streamsOf returns the streams of the test file name.
func streamsOf(t *testing.T, name string) map[string][]byte {
	t.Helper()
	buf := readFile(t, name)
	f, err := cfb.Open(bytes.NewReader(buf), int64(len(buf)), cfb.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string][]byte)
	for _, e := range f.Root().Children {
		if out[e.Name], err = f.ReadFile(e.Name); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestThumbnail(t *testing.T) {
	s := streamsOf(t, "slides.ppt")
	// a bitmap of 2 by 1 pixels of 24 bits, in green
	dib := make([]byte, 40, 48)
	binary.LittleEndian.PutUint32(dib, 40)
	binary.LittleEndian.PutUint32(dib[4:], 2)
	binary.LittleEndian.PutUint32(dib[8:], 1)
	binary.LittleEndian.PutUint16(dib[12:], 1)
	binary.LittleEndian.PutUint16(dib[14:], 24)
	dib = append(dib, 0, 255, 0, 0, 255, 0, 0, 0)
	sum := fixture.Summary{ThumbnailTag: -1, ThumbnailFormat: fixture.ClipDIB, Thumbnail: dib}
	buf := fixture.CFB(
		fixture.CFBEntry{Name: "\x05SummaryInformation", Data: sum.Bytes()},
		fixture.CFBEntry{Name: "Current User", Data: s["Current User"]},
		fixture.CFBEntry{Name: "PowerPoint Document", Data: s["PowerPoint Document"]},
	)
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(2, 1) {
		t.Errorf("expected 2x1, got: %v", img.Bounds().Size())
	}
	if got := color.NRGBAModel.Convert(img.At(1, 0)); got != (color.NRGBA{0, 255, 0, 255}) {
		t.Errorf("expected green, got: %v", got)
	}
}

func TestOpenErrors(t *testing.T) {
	s := streamsOf(t, "slides.ppt")
	user := func(change func(b []byte)) []byte {
		cu := bytes.Clone(s["Current User"])
		change(cu)
		return fixture.CFB(
			fixture.CFBEntry{Name: "Current User", Data: cu},
			fixture.CFBEntry{Name: "PowerPoint Document", Data: s["PowerPoint Document"]},
		)
	}
	tests := []struct {
		name string
		buf  []byte
		want error
	}{
		{"not a compound file", []byte("plain text, not slides"), ivy.ErrFormat},
		{"no current user", fixture.CFB(fixture.CFBEntry{Name: "PowerPoint Document", Data: s["PowerPoint Document"]}), ivy.ErrFormat},
		{"no document", fixture.CFB(fixture.CFBEntry{Name: "Current User", Data: s["Current User"]}), ivy.ErrFormat},
		{"bad current user", user(func(b []byte) { b[2] = 0 }), ivy.ErrFormat},
		{"bad offset", user(func(b []byte) { binary.LittleEndian.PutUint32(b[16:], 3) }), ivy.ErrFormat},
		{"encrypted without a crypt session", user(func(b []byte) { binary.LittleEndian.PutUint32(b[12:], tokenEncrypted) }), ivy.ErrFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader(tt.buf)); !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got: %v", tt.want, err)
			}
		})
	}
}

func TestTransform(t *testing.T) {
	tr := &transform{from: rect{0, 0, 100, 100}, to: rect{10, 20, 210, 120}}
	got := tr.apply(rect{50, 50, 100, 100})
	if got != (rect{110, 70, 210, 120}) {
		t.Errorf("expected the rectangle in the slide, got: %+v", got)
	}
	var none *transform
	if got := none.apply(rect{1, 2, 3, 4}); got != (rect{1, 2, 3, 4}) {
		t.Errorf("expected the same rectangle, got: %+v", got)
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"slides.ppt", "plain.ppt", "rc4cryptoapi_password.ppt"} {
		f.Add(readFile(f, name))
	}
	cfg := ivy.Config{
		Password: msoPassword,
		Limits:   ivy.Limits{MaxMemory: 4 << 20, MaxPixels: 1 << 20, MaxObjects: 10000, MaxDepth: 16},
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Open(t.Context(), bytes.NewReader(data), WithConfig(cfg))
		if err != nil {
			if strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
			return
		}
		_, _ = d.Describe(t.Context())
		for p, err := range d.Previews(t.Context()) {
			if err == nil {
				_, _ = p.Decode(t.Context())
			}
		}
		if _, err := d.source(t.Context()); err != nil && strings.Contains(err.Error(), "panic") {
			t.Fatal(err)
		}
	})
}
