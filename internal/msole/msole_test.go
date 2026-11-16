package msole

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xo/magic/cfb"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/offcrypto"
)

var red = color.NRGBA{255, 0, 0, 255}

// redEMF returns an EMF of 200 by 100 pixels at 100 to the inch, filled
// with red.
func redEMF() []byte {
	e := fixture.NewEMF(200, 100)
	e.Select(e.Brush(red, -1))
	e.Select(fixture.Stock(8))
	e.Rectangle(0, 0, 200, 100)
	return e.Bytes()
}

// redWMF returns a METAFILEPICT of 2 by 1 inches and a WMF without a
// placeable header, filled with red.
func redWMF() []byte {
	w := fixture.NewWMF(200, 100, false)
	w.Select(w.Brush(red))
	w.Select(w.NullPen())
	w.Rectangle(0, 0, 200, 100)
	head := binary.LittleEndian.AppendUint16(nil, 8)
	head = binary.LittleEndian.AppendUint16(head, 5080)
	head = binary.LittleEndian.AppendUint16(head, 2540)
	head = binary.LittleEndian.AppendUint16(head, 0)
	return append(head, w.Bytes()...)
}

// dib returns a bitmap of 2 by 2 pixels of 24 bits: red, then blue below.
func dib() []byte {
	b := make([]byte, 40, 56)
	binary.LittleEndian.PutUint32(b, 40)
	binary.LittleEndian.PutUint32(b[4:], 2)
	binary.LittleEndian.PutUint32(b[8:], 2)
	binary.LittleEndian.PutUint16(b[12:], 1)
	binary.LittleEndian.PutUint16(b[14:], 24)
	// the rows go from the bottom, in blue, green and red, padded to 4
	// bytes
	b = append(b, 255, 0, 0, 255, 0, 0, 0, 0)
	b = append(b, 0, 0, 255, 0, 0, 255, 0, 0)
	return b
}

func open(t *testing.T, entries ...fixture.CFBEntry) *cfb.File {
	t.Helper()
	buf := fixture.CFB(entries...)
	f, err := Open(bytes.NewReader(buf), ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(bytes.NewReader([]byte("not a compound file at all")), ivy.Limits{}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
	f := open(t, fixture.CFBEntry{Name: "a", Data: make([]byte, 5000)})
	if _, err := ReadStream(f, "missing", ivy.Limits{}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for a missing stream, got: %v", err)
	}
	if _, err := ReadStream(f, "a", ivy.Limits{MaxMemory: 100}); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
	buf, err := ReadStream(f, "a", ivy.Limits{})
	if err != nil || len(buf) != 5000 {
		t.Errorf("expected 5000 bytes, got: %d, %v", len(buf), err)
	}
}

func TestSummary(t *testing.T) {
	created := time.Date(2020, 5, 6, 7, 8, 9, 0, time.UTC)
	changed := time.Date(2021, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name      string
		codePage  uint16
		title     []byte
		wantTitle string
	}{
		{"windows-1252", 1252, []byte("Caf\xe9 \x80"), "Café €"},
		{"windows-1251", 1251, []byte("\xcf\xf0\xe8\xe2\xe5\xf2"), "Привет"},
		{"utf-8", 65001, []byte("Grüße"), "Grüße"},
		{"mac", 10000, []byte("Caf\x8e"), "Café"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum := fixture.Summary{CodePage: tt.codePage, Title: tt.title, Author: []byte("Ann"), Created: created, Changed: changed}
			f := open(t, fixture.CFBEntry{Name: SummaryName, Data: sum.Bytes()})
			s, err := ReadSummary(f, ivy.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if s.Info.Title != tt.wantTitle {
				t.Errorf("expected the title %q, got: %q", tt.wantTitle, s.Info.Title)
			}
			if len(s.Info.Authors) != 1 || s.Info.Authors[0] != "Ann" {
				t.Errorf("expected the author Ann, got: %q", s.Info.Authors)
			}
			if !s.Info.Created.Equal(created) || !s.Info.Modified.Equal(changed) {
				t.Errorf("expected the dates %v and %v, got: %v and %v", created, changed, s.Info.Created, s.Info.Modified)
			}
		})
	}
}

func TestSummaryMissing(t *testing.T) {
	f := open(t, fixture.CFBEntry{Name: "a", Data: []byte("a")})
	s, err := ReadSummary(f, ivy.Limits{})
	if err != nil || s.Thumbnail != nil || s.Info.Title != "" {
		t.Errorf("expected an empty summary, got: %+v, %v", s, err)
	}
	if s := ParseSummary([]byte("damaged")); s.Thumbnail != nil {
		t.Errorf("expected an empty summary, got: %+v", s)
	}
}

func TestThumbnails(t *testing.T) {
	tests := []struct {
		name   string
		tag    int32
		format uint32
		data   []byte
		size   image.Point
		at     image.Point
		want   color.NRGBA
	}{
		{"emf", -1, fixture.ClipEnhMetafile, redEMF(), image.Pt(256, 128), image.Pt(128, 64), red},
		{"wmf", -1, fixture.ClipMetafilePict, redWMF(), image.Pt(256, 128), image.Pt(128, 64), red},
		{"dib", -1, fixture.ClipDIB, dib(), image.Pt(2, 2), image.Pt(0, 0), red},
		{"png", -1, 2, fixture.PNG(30, 20, red), image.Pt(30, 20), image.Pt(5, 5), red},
		{"named png", 4, 0, append([]byte("PNG\x00"), fixture.PNG(30, 20, red)...), image.Pt(30, 20), image.Pt(5, 5), red},
		{"emf without a format", 0, 0, redEMF(), image.Pt(256, 128), image.Pt(128, 64), red},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum := fixture.Summary{ThumbnailTag: tt.tag, ThumbnailFormat: tt.format, Thumbnail: tt.data}
			f := open(t, fixture.CFBEntry{Name: SummaryName, Data: sum.Bytes()})
			s, err := ReadSummary(f, ivy.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			var previews []ivy.Preview
			for p, err := range s.Previews(t.Context(), ivy.Config{}) {
				if err != nil {
					t.Fatal(err)
				}
				previews = append(previews, p)
			}
			if len(previews) != 1 {
				t.Fatalf("expected 1 preview, got: %d", len(previews))
			}
			if previews[0].Size != tt.size {
				t.Errorf("expected the size %v, got: %v", tt.size, previews[0].Size)
			}
			img, err := previews[0].Decode(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Size() != tt.size {
				t.Errorf("expected an image of %v, got: %v", tt.size, img.Bounds().Size())
			}
			got := color.NRGBAModel.Convert(img.At(tt.at.X, tt.at.Y)).(color.NRGBA)
			if got != tt.want {
				t.Errorf("expected %v at %v, got: %v", tt.want, tt.at, got)
			}
		})
	}
}

func TestThumbnailErrors(t *testing.T) {
	if _, err := ThumbnailPreview(cfb.ClipboardData{Tag: -1, Format: cfb.CFBitmap, Data: []byte{1, 2, 3}}, ivy.Config{}); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("expected ivy.ErrUnsupported for a bitmap, got: %v", err)
	}
	if _, err := ThumbnailPreview(cfb.ClipboardData{Tag: -1, Format: cfb.CFMetafilePict, Data: []byte{1}}, ivy.Config{}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat for a short metafilepict, got: %v", err)
	}
	if _, err := ThumbnailPreview(cfb.ClipboardData{Tag: -1, Format: cfb.CFDIB, Data: []byte{1, 2}}, ivy.Config{}); err == nil {
		t.Error("expected an error for a damaged bitmap")
	}
	lim := ivy.Config{Limits: ivy.Limits{MaxPixels: 100}}
	if _, err := ThumbnailPreview(cfb.ClipboardData{Tag: -1, Format: cfb.CFEnhMetafile, Data: redEMF()}, lim); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
	// the summary leaves out a thumbnail that it cannot read
	sum := fixture.Summary{ThumbnailTag: -1, ThumbnailFormat: 2, Thumbnail: []byte{1, 2, 3}}
	s := ParseSummary(sum.Bytes())
	for range s.Previews(t.Context(), ivy.Config{}) {
		t.Error("expected no preview")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, err := range s.Previews(ctx, ivy.Config{}) {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	}
}

func TestImageSize(t *testing.T) {
	tests := []struct {
		w, h float64
		want image.Point
	}{
		{612, 792, image.Pt(792, 1024)},
		{72, 36, image.Pt(256, 128)},
		{540, 360, image.Pt(720, 480)},
	}
	for _, tt := range tests {
		got, ok := imageSize(tt.w, tt.h)
		if !ok || got != tt.want {
			t.Errorf("expected %v for %gx%g, got: %v", tt.want, tt.w, tt.h, got)
		}
	}
	if _, ok := imageSize(0, 10); ok {
		t.Error("expected no size for an empty picture")
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		cp   uint16
		in   string
		want string
	}{
		{1252, "\x93hi\x94", "“hi”"},
		{1251, "\xc4\xe0", "Да"},
		{932, "\x82\xa0", "あ"},
		{0, "abc", "abc"},
		{28602, "\xe9", "é"},
	}
	for _, tt := range tests {
		if got := Decode([]byte(tt.in), tt.cp); got != tt.want {
			t.Errorf("expected %q for the code page %d, got: %q", tt.want, tt.cp, got)
		}
	}
}

func TestKey(t *testing.T) {
	info := rc4Info(t)
	if _, err := Key(t.Context(), info, ivy.Config{}); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword, got: %v", err)
	}
	if _, err := Key(t.Context(), info, ivy.Config{Password: "wrong"}, "secret"); err != nil {
		t.Errorf("expected the default password to open it, got: %v", err)
	}
	var attempts []int
	cfg := ivy.Config{Password: "first", PasswordFunc: func(_ context.Context, attempt int) (string, error) {
		attempts = append(attempts, attempt)
		if attempt == 2 {
			return "secret", nil
		}
		return "wrong", nil
	}}
	if _, err := Key(t.Context(), info, cfg); err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 {
		t.Errorf("expected 2 attempts, got: %v", attempts)
	}
	stop := errors.New("stop")
	cfg.PasswordFunc = func(context.Context, int) (string, error) { return "", stop }
	if _, err := Key(t.Context(), info, cfg); !errors.Is(err, ivy.ErrPassword) || !errors.Is(err, stop) {
		t.Errorf("expected ivy.ErrPassword and the error of the function, got: %v", err)
	}
}

// rc4Info returns the encryption header of RC4 of a Word document that
// LibreOffice encrypted with the password "secret", from the test files of
// offcrypto.
func rc4Info(t *testing.T) *offcrypto.Info {
	t.Helper()
	buf, err := os.ReadFile("../offcrypto/testdata/libreoffice_rc4_doc.table")
	if err != nil {
		t.Fatal(err)
	}
	info, err := offcrypto.ParseEncryptionInfo(buf)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestHTML(t *testing.T) {
	got := HTML("A <b>", "en", "<style></style>\n", "<p>x</p>\n")
	for _, want := range []string{`<html lang="en">`, "<title>A &lt;b&gt;</title>", "<style></style>", "<body>\n<p>x</p>"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
}

func FuzzThumbnail(f *testing.F) {
	f.Add(int32(-1), uint32(fixture.ClipEnhMetafile), redEMF())
	f.Add(int32(-1), uint32(fixture.ClipMetafilePict), redWMF())
	f.Add(int32(-1), uint32(fixture.ClipDIB), dib())
	f.Add(int32(-1), uint32(2), fixture.PNG(4, 4, red))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 4 << 20, MaxPixels: 1 << 20, MaxObjects: 10000, MaxDepth: 16}}
	f.Fuzz(func(t *testing.T, tag int32, format uint32, data []byte) {
		p, err := ThumbnailPreview(cfb.ClipboardData{Tag: tag, Format: format, Data: data}, cfg)
		if err != nil {
			if strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
			return
		}
		if _, err := p.Decode(t.Context()); err != nil && strings.Contains(err.Error(), "panic") {
			t.Fatal(err)
		}
	})
}
