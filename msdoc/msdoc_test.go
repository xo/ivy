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
	"github.com/xo/ivy/internal/offcrypto"
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
		`<h1 style="margin-top: 12pt; margin-bottom: 6pt; line-height: 1.1499; -ivy-line-multiple: 1; font-size: 18pt; font-family: &#34;Liberation Sans&#34;, &#34;Microsoft YaHei&#34;, sans-serif"><span style="font-weight: bold; font-size: 18pt;`,
		`<span style="font-weight: bold; font-size: 12pt; font-kerning: normal; font-family: &#34;Liberation Serif&#34;, &#34;NSimSun&#34;, serif">bold text</span>`,
		`<span style="font-style: italic; font-size: 12pt;`,
		"Unicode: éè € Ω.",
		`<p style="text-align: center; line-height: 1.1499; -ivy-line-multiple: 1; font-size: 12pt; font-family: &#34;Liberation Serif&#34;, &#34;NSimSun&#34;, serif"><span style="font-size: 20pt;`,
		"<table ",
		`<col style="width: 249.3pt"/>`,
		`border-left: 0.5pt solid #000000`,
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
		{"xor obfuscation", word(func(w []byte) { w[11] |= 0x81 }), ivy.ErrPassword},
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

// obfuscate applies the XOR obfuscation of Word to the streams of text.doc
// with the password, as Word does (MS-DOC 2.2.6.1). The obfuscation of
// method 2 is its own inverse, so the key decrypts and encrypts.
func obfuscate(t *testing.T, password string) []byte {
	t.Helper()
	s := streamsOf(t, "text.doc")
	verifier := offcrypto.XORVerifier2(password)
	key, err := offcrypto.XORMethod2(verifier).Key(t.Context(), password)
	if err != nil {
		t.Fatal(err)
	}
	w, table := bytes.Clone(s["WordDocument"]), bytes.Clone(s["1Table"])
	binary.LittleEndian.PutUint16(w[10:], binary.LittleEndian.Uint16(w[10:])|flagEncrypted|flagObfuscated)
	binary.LittleEndian.PutUint32(w[14:], verifier)
	if err := key.DecryptAt(w[clearBytes:], clearBytes, blockSizeBytes); err != nil {
		t.Fatal(err)
	}
	if err := key.DecryptAt(table, 0, blockSizeBytes); err != nil {
		t.Fatal(err)
	}
	return fixture.CFB(
		fixture.CFBEntry{Name: "WordDocument", Data: w},
		fixture.CFBEntry{Name: "1Table", Data: table},
	)
}

func TestXOR(t *testing.T) {
	const password = "Ivy test"
	buf := obfuscate(t, password)
	if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword without a password, got: %v", err)
	}
	if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: "wrong"})); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword for a wrong password, got: %v", err)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: password}))
	if err != nil {
		t.Fatal(err)
	}
	if got := htmlOf(t, d); !strings.Contains(got, "bold text") || !strings.Contains(got, "After the table.") {
		t.Errorf("expected the text of the document:\n%s", got)
	}
}

// word6Section returns a document of Word 6 with one section, whose Sepx
// holds sepx: the count of its bytes and its sprms.
func word6Section(sepx []byte) []byte {
	text := "Text.\r"
	w := make([]byte, 0x300+len(text), 0x400)
	le := binary.LittleEndian
	le.PutUint16(w, wIdentWord6)
	le.PutUint16(w[2:], 0x0065)
	le.PutUint16(w[6:], 0x0409)
	le.PutUint32(w[word6FcMinOffset:], 0x300)
	le.PutUint32(w[word6FcMacOffset:], uint32(0x300+len(text)))
	le.PutUint32(w[word6CcpOffset:], uint32(len(text)))
	copy(w[0x300:], text)
	sepxAt := len(w)
	w = append(w, sepx...)
	plc := u32s(0, uint32(len(text)))
	plc = append(plc, sed(uint32(sepxAt))...)
	le.PutUint32(w[word6PlcfSed:], uint32(len(w)))
	le.PutUint32(w[word6PlcfSed+4:], uint32(len(plc)))
	w = append(w, plc...)
	return fixture.CFB(fixture.CFBEntry{Name: "WordDocument", Data: w})
}

func TestPaper(t *testing.T) {
	landscape := ivy.A4.Landscape()
	// a section without a size, and a section of US Legal: the sprms
	// sprmSBkc, sprmSXaPage and sprmSYaPage of Word 6
	none := word6Section([]byte{2, 0, 142, 2})
	legal := word6Section([]byte{6, 0, 164, 0xd0, 0x2f, 165, 0xc0, 0x4e})
	for _, test := range []struct {
		name  string
		buf   []byte
		paper ivy.Paper
		exp   ivy.Paper
	}{
		{"default", none, ivy.Paper{}, ivy.Letter},
		{"no size", none, landscape, landscape},
		{"own size", legal, landscape, ivy.Legal},
		{"own size without paper", legal, ivy.Paper{}, ivy.Legal},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(test.buf), WithConfig(ivy.Config{Paper: test.paper}))
			if err != nil {
				t.Fatal(err)
			}
			fixture.CheckPaper(t, d, test.exp)
		})
	}
	// the margins of the Sep stay on the paper of the program
	d, err := Open(t.Context(), bytes.NewReader(none), WithConfig(ivy.Config{Paper: landscape}))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if exp := "size: 841.89pt 595.28pt;\n  margin: 72pt 90pt 72pt 90pt;"; !strings.Contains(src.StyleSheets[0], exp) {
		t.Errorf("expected %q in the style sheet, got:\n%s", exp, src.StyleSheets[0])
	}
}

func TestWord6(t *testing.T) {
	// a FIB of Word 6 and its text, which Word did not save fast
	text := "First paragraph.\rSecond \xe9t\xe9.\r\x0cNew section.\r"
	w := make([]byte, 0x300+len(text), 0x400)
	le := binary.LittleEndian
	le.PutUint16(w, wIdentWord6)
	le.PutUint16(w[2:], 0x0065)
	le.PutUint16(w[6:], 0x0409)
	le.PutUint32(w[word6FcMinOffset:], 0x300)
	le.PutUint32(w[word6FcMacOffset:], uint32(0x300+len(text)))
	le.PutUint32(w[word6CcpOffset:], uint32(len(text)))
	copy(w[0x300:], text)
	// two sections of the same page, the second continuous, with the sprms
	// of Word 6
	sepx := []byte{5, 0, 164, 0x82, 0x2e, 142, 2}
	sepx2 := []byte{5, 0, 164, 0x82, 0x2e, 142, 0}
	sepxAt := len(w)
	w = append(w, sepx...)
	w = append(w, sepx2...)
	plc := u32s(0, 30, uint32(len(text)))
	plc = append(plc, sed(uint32(sepxAt))...)
	plc = append(plc, sed(uint32(sepxAt+len(sepx)))...)
	le.PutUint32(w[word6PlcfSed:], uint32(len(w)))
	le.PutUint32(w[word6PlcfSed+4:], uint32(len(plc)))
	w = append(w, plc...)
	buf := fixture.CFB(fixture.CFBEntry{Name: "WordDocument", Data: w})
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	got := htmlOf(t, d)
	for _, want := range []string{">First paragraph.<", ">Second été.<", ">New section.<"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the html:\n%s", want, got)
		}
	}
	if strings.Contains(got, `class="page"`) {
		t.Errorf("expected no page break before a continuous section:\n%s", got)
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 1 {
		t.Errorf("expected 1 page, got: %d, %v", n, err)
	}
	if pg := d.parser.firstPage(ivy.Paper{}); pg.width != 0x2e82 {
		t.Errorf("expected the width of the page from the section, got: %d", pg.width)
	}
	// a Word 6 document that a password protects
	le.PutUint16(w[10:], flagEncrypted)
	buf = fixture.CFB(fixture.CFBEntry{Name: "WordDocument", Data: w})
	if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("expected ivy.ErrUnsupported, got: %v", err)
	}
	// a FIB that the stream cuts
	buf = fixture.CFB(fixture.CFBEntry{Name: "WordDocument", Data: w[:0x100]})
	if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
}

// u32s returns the values in 4 bytes each, little endian.
func u32s(vs ...uint32) []byte {
	var out []byte
	for _, v := range vs {
		out = binary.LittleEndian.AppendUint32(out, v)
	}
	return out
}

// sed returns a SED of 12 bytes whose Sepx is at fc.
func sed(fc uint32) []byte {
	b := make([]byte, 12)
	binary.LittleEndian.PutUint32(b[2:], fc)
	return b
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

func TestLineSpacing(t *testing.T) {
	tests := []struct {
		dya, mult uint16
		want      string
	}{
		{480, 1, "line-height: 2.300"},
		{0xFFFF - 359, 0, "line-height: 18pt"},
		{240, 1, ""},
	}
	p := &parser{styles: parseStyles(nil, 0)}
	for _, tt := range tests {
		var pp pap
		operand := binary.LittleEndian.AppendUint16(nil, tt.dya)
		operand = binary.LittleEndian.AppendUint16(operand, tt.mult)
		applyPap(&pp, sprm{op: sprmPDyaLine, operand: operand})
		var b strings.Builder
		p.writeParagraph(&b, paragraph{pap: pp, runs: []run{{text: "x", chp: defaultChp()}}})
		got := b.String()
		if tt.want != "" && !strings.Contains(got, tt.want) || tt.want == "" && strings.Contains(got, "line-height") {
			t.Errorf("expected %q for %d, %d, got: %s", tt.want, tt.dya, tt.mult, got)
		}
	}
}

// TestLineHeights checks that the lines of a paragraph have the height of
// a single line of Word for the font of its mark, with the line gap of the
// font, and that a run of another font has the height of its own font
// (poi/doc/floating-pictures.doc and libreoffice/doc/landscape-letter-form.doc).
func TestLineHeights(t *testing.T) {
	p := &parser{styles: parseStyles(nil, 0), fonts: []font{{name: "Times New Roman"}, {name: "Noto Sans"}, {name: "Calibri"}}}
	mark := defaultChp()
	mark.ftc, mark.hps = 0, 20
	tnr, noto, calibri := mark, mark, mark
	noto.ftc, calibri.ftc = 1, 2
	super := mark
	super.iss = 1
	spaced := mark
	applyChp(&spaced, sprm{op: sprmCDxaSpace, operand: u16s(40)}, &mark)
	lspd := func(dya, mult uint16) pap {
		var pp pap
		applyPap(&pp, sprm{op: sprmPDyaLine, operand: u16s(dya, mult)})
		return pp
	}
	tests := []struct {
		name string
		pap  pap
		runs []run
		want []string
		not  []string
	}{
		{"single", pap{}, []run{{text: "x", chp: tnr}}, []string{`<p style="line-height: 1.1499; -ivy-line-multiple: 1;`}, []string{`<span style="font-size: 10pt; line-height`}},
		{"at least", lspd(360, 0), []run{{text: "x", chp: tnr}}, []string{"line-height: 18pt"}, nil},
		{"at least small", lspd(100, 0), []run{{text: "x", chp: tnr}}, []string{`<p style="line-height: 1.1499;`}, nil},
		{"other font", pap{}, []run{{text: "x", chp: noto}}, []string{`<span style="font-size: 10pt; line-height: normal;`}, nil},
		{"double", lspd(480, 1), []run{{text: "x", chp: calibri}}, []string{`<p style="line-height: 2.300;`, `<span style="font-size: 10pt; line-height: 2.4414;`}, nil},
		{"superscript", pap{}, []run{{text: "x", chp: super}}, []string{"vertical-align: super; line-height: 0"}, nil},
		{"spacing", pap{}, []run{{text: "x", chp: spaced}}, []string{"letter-spacing: 2pt"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			p.writeParagraph(&b, paragraph{pap: tt.pap, mark: mark, runs: tt.runs})
			got := b.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("expected %q, got: %s", want, got)
				}
			}
			for _, not := range tt.not {
				if strings.Contains(got, not) {
					t.Errorf("expected no %q, got: %s", not, got)
				}
			}
		})
	}
}

// TestPictureLine checks that a line of pictures ends at the bottom of the
// pictures, as in Word, and that a picture in a line of text stays on the
// baseline (page 4 of poi/doc/floating-pictures.doc).
func TestPictureLine(t *testing.T) {
	p := &parser{styles: parseStyles(nil, 0)}
	pic := &picture{name: "media/1.png", width: 72, height: 54}
	for _, tt := range []struct {
		runs []run
		want bool
	}{
		{[]run{{pic: pic}, {text: " "}, {pic: pic}}, true},
		{[]run{{text: "Text "}, {pic: pic}}, false},
	} {
		var b strings.Builder
		p.writeParagraph(&b, paragraph{runs: tt.runs})
		if got := strings.Contains(b.String(), "vertical-align: text-bottom"); got != tt.want {
			t.Errorf("expected the alignment at the bottom %t, got: %s", tt.want, b.String())
		}
	}
	if p.pictureLine {
		t.Error("expected the flag to be reset after the paragraph")
	}
}
