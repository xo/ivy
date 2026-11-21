package latex

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/xo/magic"
	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fixture"
)

// fontsFS is the folder of the module fonts, which holds Latin Modern.
var fontsFS = os.DirFS(filepath.Join("..", "fonts"))

// open opens the TeX source src with the fonts of the module fonts and
// the options opts.
func open(t *testing.T, src string, opts ...Option) *Document {
	t.Helper()
	opts = append([]Option{WithConfig(ivy.Config{Fonts: fontsFS})}, opts...)
	d, err := Open(t.Context(), bytes.NewReader([]byte(src)), opts...)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// testdata returns the file name of testdata.
func testdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// recorder records the commands of a page, and the text of its runs of
// glyphs.
type recorder struct {
	*display.List
	runs []display.GlyphRun
}

func newRecorder() *recorder {
	return &recorder{List: new(display.List)}
}

func (r *recorder) FillGlyphs(run display.GlyphRun, shader okage.Shader) {
	r.runs = append(r.runs, run)
	r.List.FillGlyphs(run, shader)
}

// text returns the text of the runs, with a space between two runs.
func (r *recorder) text() string {
	parts := make([]string, 0, len(r.runs))
	for _, run := range r.runs {
		parts = append(parts, run.Text)
	}
	return strings.Join(parts, " ")
}

// draw draws page n of d onto a recorder.
func draw(t *testing.T, d *Document, n int) *recorder {
	t.Helper()
	p, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	r := newRecorder()
	if err := p.(display.Drawer).Draw(t.Context(), r, 1); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestArticle(t *testing.T) {
	t.Parallel()
	d := open(t, testdata(t, "article.tex"))
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 page, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 612 || h != 792 {
		t.Errorf("expected a page of 612x792, got: %gx%g", w, h)
	}
	r := draw(t, d, 0)
	text := r.text()
	for _, want := range []string{"Introduction", "italic", "bold", "monospace", "office", "find", "flow", "Alpha", "Beta", "22"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected the text %q on the page, got: %q", want, text)
		}
	}
	// the glyph of a ligature keeps the letters of its text
	for _, run := range r.runs {
		if !run.TextRanges() {
			t.Errorf("run %q: ranges of the text are not valid", run.Text)
		}
	}
	// the formulas draw their glyphs as runs too
	if !strings.Contains(text, "∑") {
		t.Errorf("expected the sum of the formula as a glyph, got: %q", text)
	}
	if _, err := d.PageCount(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Page(t.Context(), 1); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ErrPageRange, got: %v", err)
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "A Small Article" || !slices.Equal(info.Authors, []string{"The ivy test"}) {
		t.Errorf("expected the title and the author of the document, got: %+v", info)
	}
}

func TestPlain(t *testing.T) {
	t.Parallel()
	d := open(t, testdata(t, "plain.tex"))
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	// \hsize of 4in and \vsize of 3in, with a margin of 72 TeX points
	w, h := p.Size()
	if w < 400 || w > 450 || h < 300 || h > 400 {
		t.Errorf("expected a page of about 431x361, got: %gx%g", w, h)
	}
	r := draw(t, d, 0)
	if !strings.Contains(r.text(), "plain") || !strings.Contains(r.String(), "fill [") {
		t.Errorf("expected the text and the rules of the page, got:\n%s", r.List)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if dark(img) == 0 {
		t.Error("expected ink on the page")
	}
}

// dark counts the pixels of img that are darker than mid gray.
func dark(img image.Image) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r+g+bl < 3*0x8000 {
				n++
			}
		}
	}
	return n
}

func TestColor(t *testing.T) {
	t.Parallel()
	d := open(t, `\documentclass{article}\usepackage{xcolor}\begin{document}
\textcolor{red}{Red} text and \colorbox{yellow}{a box}.\end{document}`)
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	var red, yellow bool
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			red = red || (r > 0xc000 && g < 0x6000 && bl < 0x6000)
			yellow = yellow || (r > 0xf000 && g > 0xf000 && bl < 0x2000)
		}
	}
	if !red || !yellow {
		t.Errorf("expected red text and a yellow box, got red %v and yellow %v", red, yellow)
	}
}

// minimalPDF returns a PDF file of one page of 100 by 50 points, with a
// filled rectangle.
func minimalPDF() []byte {
	content := "0 0 1 rg 10 10 80 30 re f"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 50] /Contents 4 0 R >>",
		"<< /Length " + strconv.Itoa(len(content)) + " >>\nstream\n" + content + "\nendstream",
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		b.WriteString(strconv.Itoa(i+1) + " 0 obj\n" + o + "\nendobj\n")
	}
	xref := b.Len()
	b.WriteString("xref\n0 " + strconv.Itoa(len(objs)+1) + "\n0000000000 65535 f \n")
	for _, off := range offsets {
		s := strconv.Itoa(off)
		b.WriteString(strings.Repeat("0", 10-len(s)) + s + " 00000 n \n")
	}
	b.WriteString("trailer\n<< /Size " + strconv.Itoa(len(objs)+1) + " /Root 1 0 R >>\nstartxref\n" + strconv.Itoa(xref) + "\n%%EOF\n")
	return b.Bytes()
}

// eps is an EPS figure of 100 by 50 points, with a filled rectangle.
const eps = `%!PS-Adobe-3.0 EPSF-3.0
%%BoundingBox: 0 0 100 50
0 0.5 0 setrgbcolor
10 10 80 30 rectfill
showpage
`

// svgFigure is an SVG figure of 100 by 50 points.
const svgFigure = `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50" viewBox="0 0 100 50">
<rect x="10" y="10" width="80" height="30" fill="#ff00ff"/></svg>`

func TestFigures(t *testing.T) {
	t.Parallel()
	dir := fstest.MapFS{
		"fig.png":      {Data: fixture.PNG(20, 10, color.NRGBA{255, 0, 0, 255})},
		"figs/doc.pdf": {Data: minimalPDF()},
		"plot.eps":     {Data: []byte(eps)},
		"shape.svg":    {Data: []byte(svgFigure)},
	}
	d := open(t, `\documentclass{article}\usepackage{graphicx}\begin{document}
\includegraphics[width=2in]{fig.png}
\includegraphics[width=2in]{figs/doc}
\includegraphics[width=2in]{plot.eps}
\includegraphics[width=2in]{shape.svg}
\end{document}`, WithDir(dir))
	r := draw(t, d, 0)
	list := r.String()
	if strings.Count(list, "image ") != 1 {
		t.Errorf("expected one image for the PNG figure, got:\n%s", list)
	}
	// the PDF, the EPS and the SVG figures are vectors: their rectangles
	// are fills
	if strings.Count(list, "fill [") < 3 {
		t.Errorf("expected the fills of the PDF, EPS and SVG figures, got:\n%s", list)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []color.NRGBA{{255, 0, 0, 255}, {0, 0, 255, 255}, {0, 128, 0, 255}, {255, 0, 255, 255}} {
		if !hasColor(img, want) {
			t.Errorf("expected the color %v of a figure on the page", want)
		}
	}
}

// hasColor reports whether img holds a pixel within 8 of c in each channel.
func hasColor(img image.Image, c color.NRGBA) bool {
	b := img.Bounds()
	near := func(a uint32, b uint8) bool {
		d := int(a>>8) - int(b)
		return d >= -8 && d <= 8
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if near(r, c.R) && near(g, c.G) && near(bl, c.B) {
				return true
			}
		}
	}
	return false
}

func TestSpecial(t *testing.T) {
	t.Parallel()
	d := open(t, `\hsize=3in \vsize=2in A special:\special{gotex:<rect x="{?x}" y="{?y}" width="20" height="10" fill="#00ff00"/>}\bye`)
	r := draw(t, d, 0)
	if !strings.Contains(r.String(), "fill [") {
		t.Errorf("expected the rectangle of the special, got:\n%s", r.List)
	}
}

func TestInputDir(t *testing.T) {
	t.Parallel()
	dir := fstest.MapFS{"chapter.tex": {Data: []byte("Text of the chapter.")}}
	d := open(t, `\input chapter \bye`, WithDir(dir))
	if text := draw(t, d, 0).text(); !strings.Contains(text, "chapter") {
		t.Errorf("expected the text of the file of \\input, got: %q", text)
	}
}

// TestNoDisk makes sure that a document reads no file of the disk (D5).
func TestNoDisk(t *testing.T) {
	t.Parallel()
	secret := filepath.Join(t.TempDir(), "secret.tex")
	if err := os.WriteFile(secret, []byte("SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := open(t, `Before \input{`+filepath.ToSlash(secret)+`} after.\bye`)
	if text := draw(t, d, 0).text(); strings.Contains(text, "SECRET") {
		t.Errorf("expected no text of a file of the disk, got: %q", text)
	}
}

func TestTeXTree(t *testing.T) {
	t.Parallel()
	tree := fstest.MapFS{
		"tex/latex/demo/demo.sty": {Data: []byte(`\def\demotext{Text of the package}`)},
	}
	d, err := Open(t.Context(), bytes.NewReader([]byte(`\documentclass{article}\usepackage{demo}\begin{document}\demotext\end{document}`)),
		WithConfig(ivy.Config{Fonts: fontsFS, TeX: tree}))
	if err != nil {
		t.Fatal(err)
	}
	if text := draw(t, d, 0).text(); !strings.Contains(text, "package") {
		t.Errorf("expected the text of the package of the tree, got: %q", text)
	}
}

// TestTeXFonts takes Latin Modern from the tree of TeX, as fonts.TeX of the
// module fonts gives it.
func TestTeXFonts(t *testing.T) {
	t.Parallel()
	lm, err := os.ReadFile(filepath.Join("..", "fonts", "lm", "lmroman10-regular.otf"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := ivy.Config{TeX: os.DirFS(filepath.Join("..", "fonts"))}
	d, err := Open(t.Context(), bytes.NewReader([]byte(`Text in Latin Modern.\bye`)), WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	r := draw(t, d, 0)
	if len(r.runs) == 0 {
		t.Fatal("expected runs of glyphs")
	}
	src, ok := r.runs[0].Face.Source()
	if !ok || !bytes.Equal(src.Data, lm) {
		t.Error("expected the text in lmroman10-regular")
	}
}

func TestGoFonts(t *testing.T) {
	t.Parallel()
	d, err := Open(t.Context(), bytes.NewReader([]byte(`Text in the Go fonts and $x^2$.\bye`)))
	if err != nil {
		t.Fatal(err)
	}
	if text := draw(t, d, 0).text(); !strings.Contains(text, "fonts") {
		t.Errorf("expected the text, got: %q", text)
	}
}

func TestNotText(t *testing.T) {
	t.Parallel()
	_, err := Open(t.Context(), bytes.NewReader([]byte("\x89PNG\r\n\x1a\n\x00\x00")))
	if !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	_, err = Open(t.Context(), bytes.NewReader(nil))
	if !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat for an empty file, got: %v", err)
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 4}}
	_, err = Open(t.Context(), bytes.NewReader([]byte("a long text")), WithConfig(cfg))
	if !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
}

func TestEmpty(t *testing.T) {
	t.Parallel()
	d := open(t, `\bye`)
	if _, err := d.PageCount(t.Context()); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat for a document without pages, got: %v", err)
	}
}

// TestContext makes sure that the context stops a document that loops.
func TestContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	d := open(t, `Text.\bye`)
	if _, err := d.PageCount(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	// the run can run again with another context
	if _, err := d.PageCount(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	d = open(t, `\def\a{\b x\a}\def\b{\a}\a`)
	start := time.Now()
	_, err := d.PageCount(ctx)
	if err == nil {
		t.Error("expected an error for a loop")
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("expected the loop to stop, it took %v", time.Since(start))
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()
	src := testdata(t, "article.tex")
	res, err := magic.Detect(t.Context(), []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(Format.Mimes, res.MIME) {
		t.Errorf("expected the mime type %q of magic in Format.Mimes %v", res.MIME, Format.Mimes)
	}
	d, err := Format.Open(t.Context(), bytes.NewReader([]byte(src)), ivy.Config{Fonts: fontsFS})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.(ivy.Pager); !ok {
		t.Error("expected a Pager")
	}
}

// TestConcurrent renders a page in bands on many goroutines, for the race
// detector.
func TestConcurrent(t *testing.T) {
	t.Parallel()
	d := open(t, testdata(t, "math.tex"))
	pages := []int{0, 0, 0, 0}
	for img, err := range ivy.RenderPages(t.Context(), d, pages, 150, color.White) {
		if err != nil {
			t.Fatal(err)
		}
		if dark(img) == 0 {
			t.Error("expected ink on the page")
		}
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"article.tex", "plain.tex", "math.tex"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte(`\special{gotex:<g transform="scale(2)"><rect width="5" height="5"/>}x\bye`))
	f.Add([]byte(`\def\a#1{#1#1}\a{\a{\a{x}}}$\frac{a}{b}$\bye`))
	cfg := ivy.Config{
		Fonts:  fontsFS,
		Limits: ivy.Limits{MaxMemory: 4 << 20, MaxPixels: 1 << 20, MaxObjects: 1000, MaxDepth: 16},
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		d, err := Open(ctx, bytes.NewReader(data), WithConfig(cfg))
		if err != nil {
			return
		}
		n, err := d.PageCount(ctx)
		if err != nil {
			if strings.Contains(err.Error(), "panic") {
				t.Fatal(err)
			}
			return
		}
		for i := range min(n, 2) {
			p, err := d.Page(ctx, i)
			if err != nil {
				t.Fatal(err)
			}
			w, h := p.Size()
			if !(w > 0 && h > 0) || w > 1e5 || h > 1e5 {
				continue
			}
			_, _ = ivy.RenderImage(ctx, p, 9, color.White)
		}
	})
}
