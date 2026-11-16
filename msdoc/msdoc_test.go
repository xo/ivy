package msdoc

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

// The passwords of the test files.
const (
	loPassword  = "secret"
	msoPassword = "Password1234_"
)

func readFile(t testing.TB, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

// htmlOf returns the HTML document that d makes for the layout.
func htmlOf(t *testing.T, d *Document) string {
	t.Helper()
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	buf, err := fs.ReadFile(src.FS, src.Documents[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(buf)
}

// streamsOf returns the streams of the test file name, to build other
// files from them.
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

func TestOpen(t *testing.T) {
	d, err := Format.Open(t.Context(), bytes.NewReader(readFile(t, "text.doc")), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	doc := d.(*Document)
	info, err := doc.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Ivy test document" || len(info.Authors) != 1 || info.Authors[0] != "Ivy Tester" || info.Language != "en-US" {
		t.Errorf("expected the title, the author and the language, got: %+v", info)
	}
	if doc.Template() {
		t.Error("expected a document, not a template")
	}
	got := htmlOf(t, doc)
	for _, want := range []string{
		`<h1 style="margin-top: 12pt; margin-bottom: 6pt"><span style="font-weight: bold; font-size: 18pt;`,
		`<span style="font-weight: bold; font-size: 12pt; font-family: &#34;Liberation Serif&#34;, serif">bold text</span>`,
		`<span style="font-style: italic; font-size: 12pt;`,
		"Unicode: éè € Ω.",
		`<p style="text-align: center"><span style="font-size: 20pt;`,
		"<table>",
		`<td style="width: `,
		"Cell B2",
		"After the table.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the html:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "<tr>"); n != 2 {
		t.Errorf("expected 2 rows, got: %d", n)
	}
	n, err := doc.PageCount(t.Context())
	if err != nil || n != 1 {
		t.Fatalf("expected 1 page, got: %d, %v", n, err)
	}
	p, err := doc.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 612 || h != 792 {
		t.Errorf("expected a page of 612x792, got: %gx%g", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), p, 36, color.White)
	if err != nil {
		t.Fatal(err)
	}
	dark := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r < 0x8000 {
				dark++
			}
		}
	}
	if dark < 50 {
		t.Errorf("expected text on the page, got %d dark pixels", dark)
	}
	if _, err := doc.Page(t.Context(), 1); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ivy.ErrPageRange, got: %v", err)
	}
}

func TestPassword(t *testing.T) {
	tests := []struct {
		name, password, text string
	}{
		{"rc4_password.doc", loPassword, "After the table."},
		{"rc4cryptoapi_password.doc", msoPassword, "lorem ipsum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := readFile(t, tt.name)
			if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ivy.ErrPassword without a password, got: %v", err)
			}
			if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: "wrong"})); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ivy.ErrPassword for a wrong password, got: %v", err)
			}
			attempts := 0
			cfg := ivy.Config{PasswordFunc: func(_ context.Context, attempt int) (string, error) {
				attempts = attempt
				if attempt < 2 {
					return "wrong", nil
				}
				return tt.password, nil
			}}
			d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
			if err != nil {
				t.Fatal(err)
			}
			if attempts != 2 {
				t.Errorf("expected 2 attempts, got: %d", attempts)
			}
			if got := htmlOf(t, d); !strings.Contains(got, tt.text) {
				t.Errorf("expected %q in the html:\n%s", tt.text, got)
			}
		})
	}
}

func TestThumbnail(t *testing.T) {
	s := streamsOf(t, "text.doc")
	e := fixture.NewEMF(100, 100)
	e.Select(e.Brush(color.NRGBA{0, 0, 255, 255}, -1))
	e.Rectangle(0, 0, 100, 100)
	sum := fixture.Summary{Title: []byte("Thumb"), ThumbnailTag: -1, ThumbnailFormat: fixture.ClipEnhMetafile, Thumbnail: e.Bytes()}
	buf := fixture.CFB(
		fixture.CFBEntry{Name: "\x05SummaryInformation", Data: sum.Bytes()},
		fixture.CFBEntry{Name: "WordDocument", Data: s["WordDocument"]},
		fixture.CFBEntry{Name: "1Table", Data: s["1Table"]},
	)
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(256, 256) {
		t.Errorf("expected 256x256, got: %v", img.Bounds().Size())
	}
	if info, _ := d.Describe(t.Context()); info.Title != "Thumb" {
		t.Errorf("expected the title Thumb, got: %q", info.Title)
	}
	// a document without a thumbnail
	d, err = Open(t.Context(), bytes.NewReader(readFile(t, "text.doc")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ivy.ErrNoPreview, got: %v", err)
	}
}

func TestOpenErrors(t *testing.T) {
	s := streamsOf(t, "text.doc")
	word := func(change func(w []byte)) []byte {
		w := bytes.Clone(s["WordDocument"])
		change(w)
		return fixture.CFB(
			fixture.CFBEntry{Name: "WordDocument", Data: w},
			fixture.CFBEntry{Name: "1Table", Data: s["1Table"]},
		)
	}
	tests := []struct {
		name string
		buf  []byte
		want error
	}{
		{"not a compound file", []byte("plain text, not a document"), ivy.ErrFormat},
		{"no stream WordDocument", fixture.CFB(fixture.CFBEntry{Name: "Other", Data: []byte("x")}), ivy.ErrFormat},
		{"no fib", word(func(w []byte) { w[0] = 0 }), ivy.ErrFormat},
		{"word 95", word(func(w []byte) { binary.LittleEndian.PutUint16(w[2:], 0x0065) }), ivy.ErrUnsupported},
		{"no table stream", fixture.CFB(fixture.CFBEntry{Name: "WordDocument", Data: s["WordDocument"]}), ivy.ErrFormat},
		{"xor obfuscation", word(func(w []byte) { w[11] |= 0x81 }), ivy.ErrUnsupported},
		{"no piece table", word(func(w []byte) {
			off := 34 + 2*int(binary.LittleEndian.Uint16(w[32:]))
			off += 2 + 4*int(binary.LittleEndian.Uint16(w[off:])) + 2
			binary.LittleEndian.PutUint32(w[off+8*fcClx+4:], 0)
		}), ivy.ErrFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader(tt.buf)); !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got: %v", tt.want, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(ctx, bytes.NewReader(readFile(t, "text.doc"))); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestOperandSize(t *testing.T) {
	tests := []struct {
		op   uint16
		b    []byte
		want int
	}{
		{sprmCFBold, []byte{1}, 1},
		{sprmCHps, []byte{24, 0}, 2},
		{sprmCCv, []byte{1, 2, 3, 4}, 4},
		{0xC601, []byte{3, 1, 2, 3}, 4},
		{sprmTDefTable, []byte{5, 0}, 6},
		{sprmPChgTabs, []byte{255, 1, 0, 0, 0, 0, 2}, 1 + 1 + 4 + 1 + 6},
		{sprmPChgTabs, []byte{}, -1},
	}
	for _, tt := range tests {
		if got := operandSize(tt.op, tt.b); got != tt.want {
			t.Errorf("expected %d for %#x, got: %d", tt.want, tt.op, got)
		}
	}
}

func TestToggle(t *testing.T) {
	for _, tt := range []struct {
		op          byte
		style, want bool
	}{
		{0, true, false}, {1, false, true}, {0x80, true, true}, {0x81, true, false}, {0x81, false, true},
	} {
		if got := toggle(tt.op, tt.style); got != tt.want {
			t.Errorf("expected %v for %#x over %v, got: %v", tt.want, tt.op, tt.style, got)
		}
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"text.doc", "rc4_password.doc", "rc4cryptoapi_password.doc"} {
		f.Add(readFile(f, name))
	}
	cfg := ivy.Config{
		Password: loPassword,
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
