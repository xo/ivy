package test

import (
	"bytes"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy"
	"github.com/xo/ivy/latex"
	"github.com/xo/ivy/pdf/pdfenc"
)

// texBase are the files of LaTeX that the engine of go-tex/engine embeds
// upstream: the standard classes and the packages that they load (D62).
// The test reads them from the TeX distribution of the machine.
var texBase = []string{
	"article.cls", "report.cls", "book.cls", "size10.clo", "size11.clo", "size12.clo",
	"bk10.clo", "bk11.clo", "bk12.clo", "keyval.sty", "etoolbox.sty", "iftex.sty",
	"ifpdf.sty", "ifluatex.sty", "ifxetex.sty",
}

// texTree returns a tree with the base files of the TeX distribution, as
// kpsewhich finds them.
func texTree(t *testing.T) fstest.MapFS {
	t.Helper()
	path, err := exec.LookPath("kpsewhich")
	if err != nil {
		t.Fatalf("oracle kpsewhich is not installed: %v", err)
	}
	tree := fstest.MapFS{}
	for _, name := range texBase {
		out, err := exec.CommandContext(t.Context(), path, name).Output()
		if err != nil {
			continue
		}
		data, err := os.ReadFile(strings.TrimSpace(string(out)))
		if err != nil {
			t.Fatal(err)
		}
		tree["base/"+name] = &fstest.MapFile{Data: data}
	}
	if _, ok := tree["base/article.cls"]; !ok {
		t.Fatal("kpsewhich finds no article.cls")
	}
	return tree
}

// TestLaTeX compares the pages of the test files of latex with the pages
// that pdfLaTeX writes, which Ghostscript renders (D13, D62). pdfLaTeX and
// Ghostscript run as outside programs, and no code of them is read. Both
// set the text in Latin Modern, at the same sizes, so the pages differ by
// the positions of the glyphs where the engine breaks a line or places a
// formula in another way, and by the hinting of the fonts. With the
// variable IVY_LATEX_OUT, the test writes both pages to that folder.
//
// On 2026-11-25 the mean differences are 1.88 for article.tex, 0.87 for
// math.tex and 0.82 for boxes.tex. In article.tex, the engine puts the
// table at the margin, where pdfLaTeX indents it.
func TestLaTeX(t *testing.T) {
	tree := texTree(t)
	cfg := ivy.Config{Fonts: os.DirFS(filepath.Join("..", "fonts")), TeX: tree}
	tests := []struct {
		name string
		max  float64
	}{
		{"article.tex", 4},
		{"math.tex", 3},
		{"boxes.tex", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("..", "latex", "testdata", tt.name))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tt.name), src, 0o600); err != nil {
				t.Fatal(err)
			}
			tex := "pdflatex"
			path, err := exec.LookPath(tex)
			if err != nil {
				t.Fatalf("oracle %s is not installed: %v", tex, err)
			}
			cmd := exec.CommandContext(t.Context(), path, "-interaction=nonstopmode", "-halt-on-error", tt.name)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("running %s: %v\n%s", tex, err, out)
			}
			pdf := filepath.Join(dir, strings.TrimSuffix(tt.name, ".tex")+".pdf")
			want, err := run(t.Context(), "gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=png16m", "-r72",
				"-dFirstPage=1", "-dLastPage=1", "-dTextAlphaBits=4", "-dGraphicsAlphaBits=4", "-sOutputFile=-", pdf)
			if err != nil {
				t.Fatal(err)
			}
			d, err := latex.Open(t.Context(), bytes.NewReader(src), latex.WithConfig(cfg))
			if err != nil {
				t.Fatal(err)
			}
			page, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ivy.RenderImage(t.Context(), page, 72, color.White)
			if err != nil {
				t.Fatal(err)
			}
			diff := meanDiff(got, want)
			t.Logf("mean difference: %.2f", diff)
			if out := os.Getenv("IVY_LATEX_OUT"); out != "" {
				writePNG(t, filepath.Join(out, tt.name+"-ivy.png"), got)
				writePNG(t, filepath.Join(out, tt.name+"-tex.png"), want)
			}
			if diff < 0 || diff > tt.max {
				stem := filepath.Join(t.TempDir(), tt.name)
				writePNG(t, stem+"-ivy.png", got)
				writePNG(t, stem+"-tex.png", want)
				t.Errorf("expected the page of pdfLaTeX within a mean difference of %g, got: %.2f (%v and %v, in %s)",
					tt.max, diff, got.Bounds().Size(), want.Bounds().Size(), stem)
			}
		})
	}
}

// TestLaTeXText writes the page of article.tex as PDF with pdfenc, and reads
// its text with pdftotext of Poppler (D58). The words of the page, the
// letters of a ligature among them, must be in the text, in order.
func TestLaTeXText(t *testing.T) {
	cfg := ivy.Config{Fonts: os.DirFS(filepath.Join("..", "fonts")), TeX: texTree(t)}
	src, err := os.ReadFile(filepath.Join("..", "latex", "testdata", "article.tex"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := latex.Open(t.Context(), bytes.NewReader(src), latex.WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "article.pdf")
	var buf bytes.Buffer
	if err := pdfenc.Encode(t.Context(), &buf, d); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := output(t.Context(), "pdftotext", "-raw", "-enc", "UTF-8", path, "-")
	if err != nil {
		t.Fatal(err)
	}
	rest := text
	for _, word := range []string{"Small", "Article", "Introduction", "italic", "office", "find", "flow", "Alpha", "Beta", "example"} {
		i := strings.Index(rest, word)
		if i < 0 {
			t.Fatalf("expected the word %q in order in the text, got:\n%s", word, text)
		}
		rest = rest[i+len(word):]
	}
}
