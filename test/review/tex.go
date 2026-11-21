package review

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing/fstest"
)

// The trees of TeX files that the review can give the package latex as
// ivy.Config.TeX (D65).
const (
	// TeXBase is the standard classes and the packages that they load, as
	// kpsewhich finds them, with the fonts of the folder fonts/lm of the
	// repository. test/latex_test.go gives the same files.
	TeXBase = "base"
	// TeXDist is the whole tree of the TeX distribution, TEXMFDIST of
	// kpsewhich.
	TeXDist = "dist"
	// TeXNone gives no tree.
	TeXNone = "none"
)

// texBaseFiles are the files of LaTeX that the engine of go-tex/engine
// embeds upstream: the standard classes and the packages that they load
// (D62). They are the files of texBase in test/latex_test.go.
var texBaseFiles = []string{
	"article.cls", "report.cls", "book.cls", "size10.clo", "size11.clo", "size12.clo",
	"bk10.clo", "bk11.clo", "bk12.clo", "keyval.sty", "etoolbox.sty", "iftex.sty",
	"ifpdf.sty", "ifluatex.sty", "ifxetex.sty",
}

// texTree is a tree of TeX files in memory. A pointer to it is a value that
// a map can hold, so the package latex walks the tree once for the run.
type texTree struct {
	fstest.MapFS
}

// TeXTree returns the tree of TeX files of mode: TeXBase, TeXDist or
// TeXNone. repo is the root of the repository, whose folder fonts/lm holds
// Latin Modern. It runs kpsewhich of the TeX distribution of the machine.
func TeXTree(ctx context.Context, mode, repo string) (fs.FS, error) {
	switch mode {
	case TeXNone:
		return nil, nil
	case TeXDist:
		out, err := exec.CommandContext(ctx, "kpsewhich", "-var-value", "TEXMFDIST").Output()
		if err != nil {
			return nil, fmt.Errorf("finding the TeX distribution with kpsewhich: %w", err)
		}
		dir := strings.TrimSpace(string(out))
		if dir == "" {
			return nil, errors.New("kpsewhich gives no TEXMFDIST")
		}
		return os.DirFS(dir), nil
	case TeXBase:
	default:
		return nil, fmt.Errorf("unknown tree of TeX %q: expected %s, %s or %s", mode, TeXBase, TeXDist, TeXNone)
	}
	args := append([]string{"-must-exist"}, texBaseFiles...)
	out, err := exec.CommandContext(ctx, "kpsewhich", args...).Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("finding the classes of LaTeX with kpsewhich: %w", err)
	}
	tree := &texTree{MapFS: fstest.MapFS{}}
	for line := range strings.SplitSeq(string(out), "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		data, err := os.ReadFile(line)
		if err != nil {
			return nil, fmt.Errorf("reading the TeX file: %w", err)
		}
		tree.MapFS["base/"+filepath.Base(line)] = &fstest.MapFile{Data: data}
	}
	if _, ok := tree.MapFS["base/article.cls"]; !ok {
		return nil, errors.New("kpsewhich finds no article.cls")
	}
	lm := filepath.Join(repo, "fonts", "lm")
	names, err := os.ReadDir(lm)
	if err != nil {
		return nil, fmt.Errorf("reading the fonts of TeX: %w", err)
	}
	for _, n := range names {
		if n.IsDir() || !strings.HasSuffix(n.Name(), ".otf") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(lm, n.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading the fonts of TeX: %w", err)
		}
		tree.MapFS["lm/"+n.Name()] = &fstest.MapFile{Data: data}
	}
	return tree, nil
}

// texKinds are the kinds of a TeX document.
var texKinds = []string{"tex", "latex", "ltx"}

// Limits of the copy of the folder of a TeX document.
const (
	maxTeXFiles = 2000
	maxTeXBytes = 256 << 20
)

// texRuns bounds the runs of pdfLaTeX on one document.
const texRuns = 4

// texEpoch is the date of the references of TeX, 2026-01-01, so that
// \today and the dates of the PDF file do not change from run to run.
const texEpoch = "1767225600"

// latexClass matches the line of a LaTeX document that names its class,
// outside a comment.
var latexClass = regexp.MustCompile(`(?m)^[^%\n]*\\document(class|style)\b`)

// texPaper gives the PDF page of a LaTeX document the paper of its class.
// Without it, pdfTeX takes the page of pdftexconfig.tex of the TeX
// distribution, such as A4, for a document of the class article on US
// Letter that loads neither geometry nor hyperref. ivy draws the paper of
// the class. A package that sets the page, such as geometry, sets it again
// after this hook.
const texPaper = `\AtBeginDocument{\pdfpagewidth=\paperwidth \pdfpageheight=\paperheight}`

// plainPaper gives the PDF page of a document of plain TeX the US Letter
// paper, whose text \hsize and \vsize of plain.tex fill with margins of
// one inch.
const plainPaper = `\pdfpagewidth=8.5in \pdfpageheight=11in `

// texProgram returns the program that sets the TeX document doc, and the
// input of the program: pdflatex, texPaper and the name of doc for a LaTeX
// document, and pdftex, the format of plain TeX, with plainPaper, for a
// document that names no class. A file of plain TeX can end without \bye,
// as story.tex of the TeXbook does, so the input of pdftex ends with \end.
func texProgram(doc string) (program, input string, err error) {
	data, err := os.ReadFile(doc)
	if err != nil {
		return "", "", fmt.Errorf("reading the document: %w", err)
	}
	name := filepath.Base(doc)
	if latexClass.Match(data) {
		return "pdflatex", texPaper + `\input{` + name + `}`, nil
	}
	return "pdftex", plainPaper + `\input ` + name + ` \end`, nil
}

// pdflatex converts the TeX document doc to PDF with pdfLaTeX, or with
// pdfTeX for plain TeX, see texProgram. It copies the folder of doc, which
// holds the files that doc reads, such as its figures and its bibliography,
// into a temporary folder, and runs the program there. The program runs at
// least twice, for the cross-references and the table of contents, and
// again while its log asks for it. An error in the log fails the
// conversion, because a page with an error is not a reference.
func pdflatex(ctx context.Context, j *refJob, doc, dir string) (string, error) {
	program, input, err := texProgram(doc)
	if err != nil {
		return "", err
	}
	work, err := os.MkdirTemp(j.tmp, "tex-")
	if err != nil {
		return "", fmt.Errorf("making a folder: %w", err)
	}
	if err := copyTeXFolder(filepath.Dir(doc), work); err != nil {
		return "", err
	}
	name := filepath.Base(doc)
	job := strings.TrimSuffix(name, filepath.Ext(name))
	env := slices.Concat(j.env, []string{"SOURCE_DATE_EPOCH=" + texEpoch, "FORCE_SOURCE_DATE=1"})
	log := filepath.Join(work, job+".log")
	for run := 1; run <= texRuns; run++ {
		// an error of the document shows in the log, which is read below
		if _, err := runTool(ctx, env, work, program, "-interaction=nonstopmode", "-file-line-error",
			"-jobname="+job, input); err != nil && ctx.Err() != nil {
			return "", err
		}
		rerun, err := texLogRerun(log)
		if err != nil {
			return "", err
		}
		if run >= 2 && !rerun {
			break
		}
	}
	if msgs, err := texLogErrors(log); err != nil {
		return "", err
	} else if len(msgs) > 0 {
		return "", &toolError{fmt.Errorf("%s: %s", program, strings.Join(msgs, " | "))}
	}
	pdf := filepath.Join(work, job+".pdf")
	if !fileExists(pdf) {
		return "", &toolError{fmt.Errorf("%s wrote no PDF file", program)}
	}
	if err := moveFile(pdf, filepath.Join(dir, "out.pdf")); err != nil {
		return "", fmt.Errorf("moving the PDF file: %w", err)
	}
	if err := moveFile(log, filepath.Join(dir, "out.log")); err != nil {
		return "", fmt.Errorf("moving the log: %w", err)
	}
	return "out.pdf", nil
}

// copyTeXFolder copies the regular files of the folder src and of its
// folders into dst, within maxTeXFiles and maxTeXBytes.
func copyTeXFolder(src, dst string) error {
	files, size := 0, int64(0)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return fmt.Errorf("finding the path of %s: %w", p, err)
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("making %s: %w", target, err)
			}
			return nil
		case !d.Type().IsRegular():
			return nil
		}
		st, err := d.Info()
		if err != nil {
			return fmt.Errorf("reading %s: %w", p, err)
		}
		files++
		size += st.Size()
		if files > maxTeXFiles || size > maxTeXBytes {
			return &toolError{fmt.Errorf("the folder %s holds more than %d files or %d bytes", src, maxTeXFiles, maxTeXBytes)}
		}
		// a copy, not a link: pdfLaTeX can write a file of the folder
		return copyFile(p, target)
	})
	if err != nil {
		return fmt.Errorf("copying the folder of the document: %w", err)
	}
	return nil
}

// texLogRerun reports whether the TeX log at path asks for one more run.
func texLogRerun(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, &toolError{errors.New("TeX wrote no log")}
		}
		return false, fmt.Errorf("reading the log of TeX: %w", err)
	}
	return strings.Contains(string(data), "Rerun to get") ||
		strings.Contains(string(data), "Label(s) may have changed") ||
		strings.Contains(string(data), "Rerun LaTeX"), nil
}

// maxTeXErrors bounds the errors of a TeX log that an error keeps.
const maxTeXErrors = 3

// texLogErrors returns the first errors of the TeX log at path: each line
// that starts with "!", or with a file, a line and ":" of -file-line-error,
// with the line that follows it.
func texLogErrors(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading the log of TeX: %w", err)
	}
	defer func() { _ = f.Close() }()
	return scanTeXErrors(f)
}

// scanTeXErrors returns the first errors of the TeX log r.
func scanTeXErrors(r io.Reader) ([]string, error) {
	var msgs []string
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64<<10), 1<<20)
	for s.Scan() && len(msgs) < maxTeXErrors {
		line := s.Text()
		if !isTeXError(line) {
			continue
		}
		msg := strings.TrimSpace(line)
		if s.Scan() {
			if next := strings.TrimSpace(s.Text()); next != "" {
				msg += " " + next
			}
		}
		msgs = append(msgs, msg)
	}
	if err := s.Err(); err != nil {
		return msgs, fmt.Errorf("reading the log of TeX: %w", err)
	}
	return msgs, nil
}

// isTeXError reports whether the line of a TeX log starts an error: "! "
// in the default form, or "file.tex:12: " with -file-line-error.
func isTeXError(line string) bool {
	if strings.HasPrefix(line, "! ") {
		return true
	}
	file, rest, ok := strings.Cut(line, ":")
	if !ok || !strings.HasPrefix(file, "./") && path.Ext(file) == "" {
		return false
	}
	n, rest, ok := strings.Cut(rest, ": ")
	if !ok || n == "" || rest == "" {
		return false
	}
	for _, c := range n {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
