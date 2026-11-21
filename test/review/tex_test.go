package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestScanTeXErrors reads the errors of a TeX log, in the default form and
// in the form of -file-line-error, and leaves out the warnings.
func TestScanTeXErrors(t *testing.T) {
	log := strings.Join([]string{
		"This is pdfTeX, Version 3.141592653-2.6-1.40.29 (TeX Live 2026)",
		"(./doc.tex",
		"LaTeX Warning: Reference `x' on page 1 undefined on input line 12.",
		"File: size10.clo 2025/01/22 v1.4n Standard LaTeX file (size option)",
		"Package: hyperref 2026-01-01 v7.01",
		"./doc.tex:12: Undefined control sequence.",
		"l.12 \\foo",
		"",
		"! LaTeX Error: File `pgfmath.sty' not found.",
		"",
		"/usr/share/texmf-dist/tex/latex/beamer/beamer.cls:370: Emergency stop.",
		"<read *>",
		"./doc.tex:20: Missing $ inserted.",
	}, "\n")
	got, err := scanTeXErrors(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"./doc.tex:12: Undefined control sequence. l.12 \\foo",
		"! LaTeX Error: File `pgfmath.sty' not found.",
		"/usr/share/texmf-dist/tex/latex/beamer/beamer.cls:370: Emergency stop. <read *>",
	}
	if !slices.Equal(got, want) {
		t.Errorf("expected the errors:\n%q\ngot:\n%q", want, got)
	}
}

// TestTeXProgram picks pdflatex for a document with a class, and pdftex for
// plain TeX, whose input ends with \end.
func TestTeXProgram(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name, src, program, input string
	}{
		{"article.tex", "% a comment\n\\documentclass{article}\n\\begin{document}x\\end{document}\n", "pdflatex", texPaper + `\input{article.tex}`},
		{"old.tex", "\\documentstyle{article}\n", "pdflatex", texPaper + `\input{old.tex}`},
		{"story.tex", "% \\documentclass{article} in a comment\n\\centerline{\\bf A SHORT STORY}\n", "pdftex", plainPaper + `\input story.tex \end`},
	}
	for _, tt := range tests {
		path := filepath.Join(dir, tt.name)
		if err := os.WriteFile(path, []byte(tt.src), 0o644); err != nil {
			t.Fatal(err)
		}
		program, input, err := texProgram(path)
		if err != nil {
			t.Fatal(err)
		}
		if program != tt.program || input != tt.input {
			t.Errorf("%s: expected %s with %q, got %s with %q", tt.name, tt.program, tt.input, program, input)
		}
	}
}

// TestRenderTeX renders TeX documents of the repository with ivy and with
// the reference of pdfLaTeX: a LaTeX document whose figures, bibliography
// and .bbl are in its folder, and a document of plain TeX.
func TestRenderTeX(t *testing.T) {
	for _, name := range []string{"kpsewhich", "pdflatex", "pdftex", "mutool", "pdftoppm", "pdftocairo", "pdfinfo", "fc-list"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("oracle %s is not installed: %v", name, err)
		}
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := TeXTree(t.Context(), TeXBase, repo)
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	entries := []Entry{
		{Name: "tex/report.tex", Kind: "tex", URL: "repo:latex/testdata/report/report.tex", Size: -1},
		{Name: "tex/plain.tex", Kind: "tex", URL: "repo:latex/testdata/plain.tex", Size: -1},
	}
	opts := RenderOptions{
		Repo: repo, Cache: cache, Out: filepath.Join(t.TempDir(), "out"), Pages: 1, DPI: 72, Jobs: 2,
		Timeout: 2 * time.Minute, TeX: tree, TeXMode: TeXBase,
	}
	rep, err := Render(t.Context(), entries, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(rep.Programs, func(p Program) bool { return p.Name == "pdflatex" && strings.Contains(p.Version, "pdfTeX") }) {
		t.Errorf("expected the version of pdflatex in the report, got: %v", rep.Programs)
	}
	for _, d := range rep.Docs {
		if d.Status != StatusOK || d.Pages < 1 {
			t.Errorf("%s: expected ivy to open the document, got %s with %d pages: %s", d.Name, d.Status, d.Pages, d.Error)
		}
		if d.RefError != "" || d.RefPages < 1 {
			t.Errorf("%s: expected a reference, got %d pages: %s", d.Name, d.RefPages, d.RefError)
			continue
		}
		for _, c := range d.Rows[0].Refs {
			if c.Error != "" || c.Metric == nil {
				t.Errorf("%s: expected a page of %s with a metric, got: %s", d.Name, c.Name, c.Error)
			}
		}
	}
}
