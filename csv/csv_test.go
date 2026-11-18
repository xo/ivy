package csv

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// open opens buf.
func open(t *testing.T, buf []byte, cfg ivy.Config) *Document {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	})
	return d
}

// utf16Bytes encodes s in UTF-16, with a byte order mark when bom is set.
func utf16Bytes(s string, bigEndian, bom bool) []byte {
	var out []byte
	units := utf16.Encode([]rune(s))
	if bom {
		units = append([]uint16{0xfeff}, units...)
	}
	for _, u := range units {
		if bigEndian {
			out = append(out, byte(u>>8), byte(u))
		} else {
			out = append(out, byte(u), byte(u>>8))
		}
	}
	return out
}

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []byte
		sep  string
		want [][]string
	}{
		{"comma", []byte("a,b,c\n1,2,3\n"), ",", [][]string{{"a", "b", "c"}, {"1", "2", "3"}}},
		{"semicolon", []byte("a;b\n1,5;2,5\n"), ";", [][]string{{"a", "b"}, {"1,5", "2,5"}}},
		{"tab", []byte("a\tb, c\r\n1\t2\r\n"), "\t", [][]string{{"a", "b, c"}, {"1", "2"}}},
		{"pipe", []byte("a|b|c\nd|e|f"), "|", [][]string{{"a", "b", "c"}, {"d", "e", "f"}}},
		{"quotes", []byte("\"a,b\",\"say \"\"hi\"\"\",\"two\nlines\"\nx,y\"z,\n"), ",", [][]string{{"a,b", `say "hi"`, "two\nlines"}, {"x", `y"z`, ""}}},
		{"one column", []byte("a\nb\n"), ",", [][]string{{"a"}, {"b"}}},
		{"bom", []byte("\xef\xbb\xbfa,b\n"), ",", [][]string{{"a", "b"}}},
		{"utf-16le", utf16Bytes("é,ü\n1,2\n", false, true), ",", [][]string{{"é", "ü"}, {"1", "2"}}},
		{"utf-16be", utf16Bytes("é;ü\n1;2\n", true, true), ";", [][]string{{"é", "ü"}, {"1", "2"}}},
		{"utf-16 without a bom", utf16Bytes("ab,cd\n", false, false), ",", [][]string{{"ab", "cd"}}},
		{"windows-1252", []byte("caf\xe9,\x80 5\n"), ",", [][]string{{"café", "€ 5"}}},
		{"empty", nil, ",", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := open(t, tt.in, ivy.Config{})
			if d.sep != tt.sep {
				t.Errorf("expected the separator %q, got: %q", tt.sep, d.sep)
			}
			if !slices.EqualFunc(d.rows, tt.want, slices.Equal) {
				t.Errorf("expected %q, got: %q", tt.want, d.rows)
			}
		})
	}
}

func TestSeparator(t *testing.T) {
	// The fields of the file split on commas, but the program gives a
	// semicolon.
	buf := []byte("a;b,c\n1;2,3\n")
	d, err := Open(t.Context(), bytes.NewReader(buf), WithSeparator(';'))
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"a", "b,c"}, {"1", "2,3"}}; !slices.EqualFunc(d.rows, want, slices.Equal) {
		t.Errorf("expected %q, got: %q", want, d.rows)
	}
	if d := open(t, buf, ivy.Config{}); d.sep != "," {
		t.Errorf("expected the separator \",\" without the option, got: %q", d.sep)
	}
	d, err = Open(t.Context(), bytes.NewReader([]byte("a¦\"b¦c\"¦d\n1¦2¦3")), WithSeparator('¦'))
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"a", "b¦c", "d"}, {"1", "2", "3"}}; !slices.EqualFunc(d.rows, want, slices.Equal) {
		t.Errorf("expected %q, got: %q", want, d.rows)
	}
	for _, r := range []rune{'"', '\n', '\r', utf8.RuneError, -1, 0x110000} {
		if _, err := Open(t.Context(), bytes.NewReader(buf), WithSeparator(r)); !errors.Is(err, ivy.ErrUnsupported) {
			t.Errorf("expected ErrUnsupported for the separator %q, got: %v", r, err)
		}
	}
}

func TestDates(t *testing.T) {
	d := open(t, []byte("date,when\n2024-01-02,1/2/24\n03.04.2025,Jan 5\n"), ivy.Config{})
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	doc, err := fs.ReadFile(src.FS, src.Documents[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{">2024-01-02</td>", ">1/2/24</td>", ">03.04.2025</td>", ">Jan 5</td>"} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("expected %q in the table:\n%s", want, doc)
		}
	}
	// a date is text, not a number on the right
	if strings.Contains(string(doc), `<td class="n`) {
		t.Errorf("expected no cell of a number:\n%s", doc)
	}
}

func TestBinary(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader([]byte("PK\x03\x04\x00\x00\x01\x02"))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat for a binary file, got: %v", err)
	}
}

func TestLimit(t *testing.T) {
	buf := []byte(strings.Repeat("a,b,c,d\n", 100))
	d := open(t, buf, ivy.Config{Limits: ivy.Limits{MaxObjects: 50}})
	if !d.truncated || len(d.rows) != 12 {
		t.Errorf("expected 12 rows and the limit, got: %d %v", len(d.rows), d.truncated)
	}
	if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Limits: ivy.Limits{MaxMemory: 10}})); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
}

func TestHTML(t *testing.T) {
	buf := []byte("Name,Amount in US dollars,Note\nalpha,\"1,234.50\",short\nbeta,-2," + strings.Repeat("long ", 80) + "\n")
	d := open(t, buf, ivy.Config{})
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	doc, err := fs.ReadFile(src.FS, src.Documents[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<col style="width:48pt"><col style="width:100.11pt"><col style="width:300pt">`,
		`<td class="c">Name</td><td class="c">Amount in US dollars</td><td class="s1 c">Note</td>`,
		`<td class="c">alpha</td><td class="n c">1,234.50</td>`,
		`<td class="n c">-2</td><td class="s1 c">long long`,
		"border:0.5pt solid #c8c8c8",
	} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("expected %q in the table:\n%s", want, doc)
		}
	}
	if strings.Contains(string(doc), "@top-left") {
		t.Error("expected no header without a name")
	}
}

func TestPages(t *testing.T) {
	var b strings.Builder
	for i := range 200 {
		b.WriteString("row\t")
		b.WriteString(strings.Repeat("x", i%7+1))
		b.WriteString("\t3.5\n")
	}
	d := open(t, []byte(b.String()), ivy.Config{Fonts: officeFonts()})
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// 45 rows of 15 points fit the 684 points of a letter page
	if n != 5 {
		t.Errorf("expected 5 pages, got: %d", n)
	}
	page, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := page.Size(); w != 612 || h != 792 {
		t.Errorf("expected a letter page, got: %vx%v", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), page, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if !inked(img, image.Rect(50, 54, 100, 70)) {
		t.Error("expected the text of the first row")
	}
	if _, err := d.Page(t.Context(), 5); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ErrPageRange, got: %v", err)
	}
}

func TestEmpty(t *testing.T) {
	d := open(t, nil, ivy.Config{})
	if n, err := d.PageCount(t.Context()); err != nil || n != 1 {
		t.Errorf("expected 1 empty page, got: %d, %v", n, err)
	}
}

func TestCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(ctx, bytes.NewReader([]byte("a,b"))); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	d := open(t, []byte("a,b"), ivy.Config{})
	if _, err := d.PageCount(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 1 {
		t.Errorf("expected 1 page after the cancel, got: %d, %v", n, err)
	}
}

func TestIsNumber(t *testing.T) {
	for s, want := range map[string]bool{
		"1": true, "-1.5": true, "+3": true, "1,234.50": true, "12%": true, "1e3": true, ".5": true,
		"1,23": false, "abc": false, "": false, "1.2.3": false, "Inf": false, "NaN": false, "0x10": false,
	} {
		if got := isNumber(s); got != want {
			t.Errorf("isNumber(%q): expected %v", s, want)
		}
	}
}

// officeFonts returns the folder of the module github.com/xo/ivy/fonts, or
// nil when it is missing.
func officeFonts() fs.FS {
	dir := filepath.Join("..", "fonts")
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	return os.DirFS(dir)
}

// inked reports whether img has a dark pixel in r.
func inked(img image.Image, r image.Rectangle) bool {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if int(c.R)+int(c.G)+int(c.B) < 3*128 {
				return true
			}
		}
	}
	return false
}

func TestPaper(t *testing.T) {
	buf := []byte("a,b,c\n1,2,3\n")
	fixture.CheckPaper(t, open(t, buf, ivy.Config{}), ivy.Letter)
	fixture.CheckPaper(t, open(t, buf, ivy.Config{Paper: ivy.A4.Landscape()}), ivy.A4.Landscape())
	fixture.CheckPaper(t, open(t, []byte("x\ty\n"), ivy.Config{Paper: ivy.A5}), ivy.A5)
}

func FuzzOpen(f *testing.F) {
	f.Add([]byte("a,b,c\n1,2,3\n"), rune(0))
	f.Add([]byte("\"a\"\"b\";c\r\n\"open"), rune(0))
	f.Add(utf16Bytes("x\ty\n", false, true), rune(0))
	f.Add([]byte("caf\xe9|\x80\n"), rune(0))
	f.Add([]byte("a;b,c\n1;2,3\n"), ';')
	f.Add([]byte("a¦\"b¦c\"¦d\n1¦2¦3"), '¦')
	f.Fuzz(func(t *testing.T, buf []byte, sep rune) {
		cfg := ivy.Config{Limits: ivy.Limits{MaxObjects: 1 << 12, MaxMemory: 1 << 20}}
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg), WithSeparator(sep))
		if err != nil {
			return
		}
		page, err := d.Page(t.Context(), 0)
		if err != nil {
			t.Fatalf("expected page 0 of any text: %v", err)
		}
		_, _ = ivy.RenderImage(t.Context(), page, 8, color.White)
	})
}
