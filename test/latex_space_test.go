package test

import (
	"bytes"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/latex"
)

// spaceDocs are the documents of TestLaTeXBaselines: the vertical space of
// LaTeX around lists, headings, a title, a title page and the contents, and
// the page breaks of the clubs. They hold no formula and no footnote, so
// that each line of text has one baseline.
var spaceDocs = []struct {
	name, src string
}{
	{"lists", `\documentclass{article}
\begin{document}
Alpha
\begin{itemize}\item Bravo
\begin{itemize}\item Charlie\end{itemize}\end{itemize}
Delta
\begin{itemize}\item Echo
\begin{enumerate}\item Foxtrot\item Golf\end{enumerate}
\end{itemize}

Hotel
\begin{itemize}\item India\end{itemize}
\begin{quote}Juliet\end{quote}
Kilo
\section{Lima}
\begin{itemize}\item Mike\end{itemize}
\section{November}
Oscar
\begin{enumerate}\item Papa\end{enumerate}
\subsection{Quebec}
Romeo
\end{document}
`},
	{"title", `\documentclass{article}
\title{Title}\author{Author}\date{}
\begin{document}
\maketitle

\section{Section}
Text.
\end{document}
`},
	{"titlepage", `\documentclass[11pt]{report}
\usepackage[margin=1in,headheight=14pt]{geometry}
\usepackage{fancyhdr}
\pagestyle{fancy}\fancyhf{}\fancyhead[R]{Head}\fancyfoot[C]{Page \thepage}
\title{A Report}\author{The Team}\date{1 January 2026}
\begin{document}
\maketitle
\begin{titlepage}
\vfil
Top of the page.
\end{titlepage}
\tableofcontents
\chapter{One}
\section{Alpha}
Text.
\section{Bravo}
Text.
\chapter{Two}
Text.
\end{document}
`},
	{"club", `\documentclass{article}
\begin{document}
` + strings.Repeat("Line.\n\n", 45) + strings.Repeat("Zulu is the first word of a paragraph of three lines. ", 6) + `
\end{document}
`},
}

// TestLaTeXBaselines compares the baselines of the lines of text of each page
// of spaceDocs with those that pdfLaTeX writes, as mutool reads them from its
// PDF file (D13). pdfLaTeX runs twice, for the contents.
func TestLaTeXBaselines(t *testing.T) {
	tree := texTree(t)
	cfg := ivy.Config{Fonts: os.DirFS(filepath.Join("..", "fonts")), TeX: tree}
	for _, tt := range spaceDocs {
		t.Run(tt.name, func(t *testing.T) {
			want := texBaselines(t, tt.name, tt.src)
			got := ivyBaselines(t, cfg, tt.src)
			if len(got) != len(want) {
				t.Fatalf("expected %d pages, got: %d", len(want), len(got))
			}
			for i := range want {
				if !sameBaselines(got[i], want[i]) {
					t.Errorf("page %d: expected the baselines %v, got: %v", i+1, want[i], got[i])
				}
			}
		})
	}
}

// sameBaselines reports whether a and b hold the same baselines, within
// 0.3pt.
func sameBaselines(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 0.3 {
			return false
		}
	}
	return true
}

// baselines returns ys, sorted, with the values within 0.5pt of each other
// taken once, at two decimals.
func baselines(ys []float64) []float64 {
	slices.Sort(ys)
	var out []float64
	for _, y := range ys {
		if len(out) > 0 && y-out[len(out)-1] < 0.5 {
			continue
		}
		out = append(out, math.Round(y*100)/100)
	}
	return out
}

// stextChar matches a character of the structured text of mutool, with the
// origin of its glyph.
var stextChar = regexp.MustCompile(`<char c="([^"]*)" .* y="([-\d.]+)"`)

// texBaselines runs pdfLaTeX on src, and returns the baselines of each page
// of its PDF file, in points from the top of the page.
func texBaselines(t *testing.T, name, src string) [][]float64 {
	t.Helper()
	dir := t.TempDir()
	file := name + ".tex"
	if err := os.WriteFile(filepath.Join(dir, file), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	tex, err := exec.LookPath("pdflatex")
	if err != nil {
		t.Fatalf("oracle pdflatex is not installed: %v", err)
	}
	for range 2 {
		cmd := exec.CommandContext(t.Context(), tex, "-interaction=nonstopmode", "-halt-on-error", file)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("running pdflatex: %v\n%s", err, out)
		}
	}
	mutool, err := exec.LookPath("mutool")
	if err != nil {
		t.Fatalf("oracle mutool is not installed: %v", err)
	}
	out, err := exec.CommandContext(t.Context(), mutool, "draw", "-F", "stext", "-o", "-", filepath.Join(dir, name+".pdf")).Output()
	if err != nil {
		t.Fatalf("running mutool: %v", err)
	}
	var pages [][]float64
	var ys []float64
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(line, "<page ") {
			if ys != nil {
				pages = append(pages, baselines(ys))
			}
			ys = []float64{}
			continue
		}
		m := stextChar.FindStringSubmatch(line)
		if m == nil || strings.TrimSpace(m[1]) == "" {
			continue
		}
		y, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			t.Fatal(err)
		}
		ys = append(ys, y)
	}
	if ys != nil {
		pages = append(pages, baselines(ys))
	}
	return pages
}

// ivyBaselines opens src with ivy, and returns the baselines of each page, in
// points from the top of the page.
func ivyBaselines(t *testing.T, cfg ivy.Config, src string) [][]float64 {
	t.Helper()
	d, err := latex.Open(t.Context(), bytes.NewReader([]byte(src)), latex.WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	pages := make([][]float64, 0, n)
	for i := range n {
		p, err := d.Page(t.Context(), i)
		if err != nil {
			t.Fatal(err)
		}
		rec := &baselineRecorder{cur: okage.IdentityTransform()}
		if err := p.(display.Drawer).Draw(t.Context(), rec, 1); err != nil {
			t.Fatal(err)
		}
		pages = append(pages, baselines(rec.ys))
	}
	return pages
}

// baselineRecorder records the baseline of the first glyph of each run of
// glyphs, through the transforms of the page.
type baselineRecorder struct {
	display.List
	cur   okage.Transform
	stack []okage.Transform
	ys    []float64
}

func (r *baselineRecorder) Save() {
	r.stack = append(r.stack, r.cur)
}

func (r *baselineRecorder) Restore() {
	r.cur = r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
}

func (r *baselineRecorder) Concat(t okage.Transform) {
	r.cur = r.cur.PreConcat(t)
}

func (r *baselineRecorder) FillGlyphs(run display.GlyphRun, _ okage.Shader) {
	if len(run.Glyphs) == 0 || strings.TrimSpace(run.Text) == "" {
		return
	}
	pt := okage.Point{X: run.Glyphs[0].X, Y: run.Glyphs[0].Y}
	r.cur.MapPoint(&pt)
	r.ys = append(r.ys, float64(pt.Y))
}
