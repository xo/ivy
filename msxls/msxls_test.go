package msxls

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image/color"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// sheets returns the HTML documents that d makes for the layout.
func sheets(t *testing.T, d *Document) []string {
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
	d, err := Format.Open(t.Context(), bytes.NewReader(readFile(t, "sheet.xls")), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	doc := d.(*Document)
	info, err := doc.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Ivy test sheet" || len(info.Authors) != 1 || info.Authors[0] != "Ivy Tester" {
		t.Errorf("expected the title and the author, got: %+v", info)
	}
	if names := doc.Sheets(); len(names) != 2 || names[0] != "Numbers" || names[1] != "Second" {
		t.Errorf("expected the sheets Numbers and Second, got: %q", names)
	}
	docs := sheets(t, doc)
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got: %d", len(docs))
	}
	for _, want := range []string{
		`<td class="s21 c">Name</td>`,
		">3.14159</td>",
		">4.14159</td>",
		">1,234,567.89</td>",
		">25.00%</td>",
		">2023-03-15</td>",
		">one!</td>",
		">TRUE</td>",
		`colspan="2">Merged across</td>`,
		`@top-center{content:"Numbers"`,
		`content:"Page " counter(page)`,
		"size:612pt 792pt",
	} {
		if !strings.Contains(docs[0], want) {
			t.Errorf("expected %q in the first sheet:\n%s", want, docs[0])
		}
	}
	if !strings.Contains(docs[1], "Second sheet é") || !strings.Contains(docs[1], ">-42</td>") {
		t.Errorf("expected the cells of the second sheet:\n%s", docs[1])
	}
	n, err := doc.PageCount(t.Context())
	if err != nil || n != 2 {
		t.Fatalf("expected 2 pages, got: %d, %v", n, err)
	}
	p, err := doc.Page(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 612 || h != 792 {
		t.Errorf("expected a page of 612x792, got: %gx%g", w, h)
	}
	if _, err := ivy.RenderImage(t.Context(), p, 36, color.White); err != nil {
		t.Fatal(err)
	}
}

func TestPassword(t *testing.T) {
	tests := []struct {
		name, password, text string
	}{
		{"rc4_password.xls", loPassword, "Merged across"},
		{"rc4cryptoapi_password.xls", msoPassword, "lorem ipsum"},
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
			cfg := ivy.Config{PasswordFunc: func(_ context.Context, attempt int) (string, error) {
				if attempt < 2 {
					return "wrong", nil
				}
				return tt.password, nil
			}}
			d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
			if err != nil {
				t.Fatal(err)
			}
			if docs := sheets(t, d); !strings.Contains(docs[0], tt.text) {
				t.Errorf("expected %q in the sheet:\n%s", tt.text, docs[0])
			}
		})
	}
}

// biff builds a workbook stream of BIFF8 from its records.
type biff struct {
	buf []byte
}

func (b *biff) record(typ uint16, data ...[]byte) int {
	at := len(b.buf)
	body := bytes.Join(data, nil)
	b.buf = binary.LittleEndian.AppendUint16(b.buf, typ)
	b.buf = binary.LittleEndian.AppendUint16(b.buf, uint16(len(body)))
	b.buf = append(b.buf, body...)
	return at
}

func u16(v ...int) []byte {
	var out []byte
	for _, x := range v {
		out = binary.LittleEndian.AppendUint16(out, uint16(x))
	}
	return out
}

func u32(v ...int) []byte {
	var out []byte
	for _, x := range v {
		out = binary.LittleEndian.AppendUint32(out, uint32(x))
	}
	return out
}

func float(v float64) []byte {
	return binary.LittleEndian.AppendUint64(nil, math.Float64bits(v))
}

// workbook returns a workbook of one wide sheet, with a shared string that
// goes on in a CONTINUE record in UTF-16.
func syntheticWorkbook(t *testing.T, cols int) []byte {
	t.Helper()
	var b biff
	b.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
	b.record(recFont, u16(200, 0, 0x7FFF, 400, 0), []byte{0, 0, 0, 0}, []byte{5, 0}, []byte("Arial"))
	b.record(recFont, u16(240, 0x02, 0x7FFF, 700, 0), []byte{0, 0, 0, 0}, []byte{5, 0}, []byte("Arial"))
	b.record(recFormat, u16(164), u16(6), []byte{0}, []byte("0.0\"x\""))
	b.record(recXF, u16(0, 0), make([]byte, 16))
	b.record(recXF, u16(1, 164), []byte{0, 0, 2}, make([]byte, 13))
	pos := b.record(recBoundSheet, u32(0), []byte{0, 0}, []byte{5, 0}, []byte("Wide!"))
	// the second string starts in the SST and goes on in a CONTINUE
	// record, in UTF-16
	b.record(recSST, u32(2, 2), u16(3), []byte{0}, []byte("abc"), u16(4), []byte{0}, []byte("de"))
	b.record(recContinue, []byte{1}, u16('f', 0x0416))
	b.record(recEOF)
	sheet := len(b.buf)
	binary.LittleEndian.PutUint32(b.buf[pos+4:], uint32(sheet))
	b.record(recBOF, u16(biff8, bofWorksheet, 0, 0), u32(0, 0))
	b.record(recColInfo, u16(0, cols-1, 20*256, 0, 0, 0))
	b.record(recLabelSST, u16(0, 0, 0), u32(0))
	b.record(recLabelSST, u16(0, cols-1, 0), u32(1))
	b.record(recNumber, u16(1, 0, 1), float(2.5))
	b.record(recRK, u16(1, 1, 0), u32(123<<2|2))
	b.record(recMulRK, u16(2, 0), u16(0), u32(1<<2|3), u16(0), u32(5<<2|2), u16(1))
	b.record(recBoolErr, u16(3, 0, 0), []byte{0x07, 1})
	b.record(recFormula, u16(4, 0, 0), []byte{0, 0, 0, 0, 0, 0, 0xFF, 0xFF}, u16(0), u32(0))
	b.record(recString, u16(5), []byte{0}, []byte("hello"))
	b.record(recMergeCells, u16(1), u16(5, 6, 0, 2))
	b.record(recRow, u16(6, 0, 1, 300, 0, 0, 0x0040, 15))
	b.record(recLabelSST, u16(5, 0, 0), u32(0))
	// a chart inside the sheet, whose records are not cells
	b.record(recBOF, u16(biff8, 0x20, 0, 0), u32(0, 0))
	b.record(recNumber, u16(9, 9, 0), float(99))
	b.record(recEOF)
	b.record(recEOF)
	return fixture.CFB(fixture.CFBEntry{Name: "Workbook", Data: b.buf})
}

func TestSynthetic(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(syntheticWorkbook(t, 12)))
	if err != nil {
		t.Fatal(err)
	}
	docs := sheets(t, d)
	if len(docs) != 1 {
		t.Fatalf("expected 1 sheet, got: %d", len(docs))
	}
	got := docs[0]
	for _, want := range []string{
		">abc</td>",
		">defЖ</td>",
		`<td class="s1 c">2.5x</td>`,
		">123</td>",
		">0.01</td>",
		">5</td>",
		">#DIV/0!</td>",
		">hello</td>",
		`colspan="3" rowspan="2"`,
		"font-weight:bold;font-style:italic",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the sheet:\n%s", want, got)
		}
	}
	if n := strings.Count(got, `<table class="sh"`); n != 3 {
		t.Errorf("expected 3 bands of columns, got: %d", n)
	}
	if strings.Contains(got, ">99<") {
		t.Error("expected no cell of the chart")
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 3 {
		t.Errorf("expected 3 pages, got: %d, %v", n, err)
	}
}

func TestOpenErrors(t *testing.T) {
	stream := func(data []byte) []byte {
		return fixture.CFB(fixture.CFBEntry{Name: "Workbook", Data: data})
	}
	var xor biff
	xor.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
	xor.record(recFilePass, u16(0, 0x1234, 0x5678))
	xor.record(recEOF)
	tests := []struct {
		name string
		buf  []byte
		want error
	}{
		{"not a compound file", []byte("plain text, not a workbook"), ivy.ErrFormat},
		{"no workbook", fixture.CFB(fixture.CFBEntry{Name: "Other", Data: []byte("x")}), ivy.ErrFormat},
		{"no bof", stream([]byte{1, 2, 3, 4, 5, 6, 7, 8}), ivy.ErrFormat},
		{"biff4", stream([]byte{0x09, 0x04, 6, 0, 0, 0, 0x10, 0}), ivy.ErrUnsupported},
		{"xor obfuscation", stream(xor.buf), ivy.ErrUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader(tt.buf)); !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got: %v", tt.want, err)
			}
		})
	}
	if _, err := Open(t.Context(), bytes.NewReader(syntheticWorkbook(t, 3)), WithConfig(ivy.Config{Limits: ivy.Limits{MaxObjects: 4}})); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
}

func TestRK(t *testing.T) {
	tests := []struct {
		v    uint32
		want float64
	}{
		{123<<2 | 2, 123},
		{uint32(0xFFFFFFFC) | 2, -1},
		{12345<<2 | 3, 123.45},
		{uint32(math.Float64bits(2.5)>>32) &^ 3, 2.5},
	}
	for _, tt := range tests {
		if got := rk(tt.v); got != tt.want {
			t.Errorf("expected %v for %#x, got: %v", tt.want, tt.v, got)
		}
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"sheet.xls", "rc4_password.xls", "plain.xls"} {
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
