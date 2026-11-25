package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
)

// goFaces are the Go fonts, for the tests that do not need Latin Modern.
var goFaces = Faces{Regular: goregular.TTF, Bold: gobold.TTF, Italic: goitalic.TTF, Mono: gomono.TTF}

// latinModern registers the files of Latin Modern of the module fonts.
func latinModern(t *testing.T) Faces {
	t.Helper()
	files := map[string][]byte{}
	for _, name := range LatinModernFiles() {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "fonts", "lm", name+".otf"))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = data
	}
	lm := RegisterLatinModern(files)
	return Faces{Regular: lm.Regular, Bold: lm.Bold, Italic: lm.Italic, Mono: lm.Mono, Math: lm.Math}
}

// ivyCompile runs src with the faces.
func ivyCompile(t *testing.T, src string, faces Faces) *Doc {
	t.Helper()
	doc, err := Compile(t.Context(), []byte(src), RunOptions{Faces: faces})
	if err != nil {
		t.Fatal(err)
	}
	if doc.PageCount() == 0 {
		t.Fatal("expected a page")
	}
	return doc
}

// runs records the runs of glyphs of a page.
type runs struct {
	*display.List
	text []string
	size []float32
}

func (r *runs) FillGlyphs(run display.GlyphRun, shader okage.Shader) {
	r.text = append(r.text, run.Text)
	r.size = append(r.size, run.Size)
	r.List.FillGlyphs(run, shader)
}

// drawPage draws page n of doc onto a recorder.
func drawPage(t *testing.T, doc *Doc, n int) *runs {
	t.Helper()
	r := &runs{List: new(display.List)}
	if err := doc.DrawPage(t.Context(), r, n, nil, 1); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDriverText(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\hsize=3in Hello world, office.\par\bye`, latinModern(t))
	r := drawPage(t, doc, 0)
	joined := strings.Join(r.text, " ")
	for _, want := range []string{"Hello", "world,", "office."} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected the run %q, got: %q", want, r.text)
		}
	}
	// the page scales TeX points to big points first
	if !strings.HasPrefix(r.String(), "save\nconcat {0.996264 0 0 0.996264 0 0}") {
		t.Errorf("expected the scale of TeX points, got:\n%s", r.List)
	}
	w, h := doc.PageSize(0)
	if w <= 0 || h <= 0 {
		t.Errorf("expected a size, got: %gx%g", w, h)
	}
}

func TestDriverOptical(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}Body {\footnotesize small} {\Large large}\end{document}`, latinModern(t))
	r := drawPage(t, doc, 0)
	sizes := map[string]float32{}
	for i, s := range r.text {
		sizes[s] = r.size[i]
	}
	if !(sizes["small"] < sizes["Body"] && sizes["Body"] < sizes["large"]) {
		t.Errorf("expected three sizes, got: %v", sizes)
	}
	fam := opticalOf(latinModern(t).Regular)
	if _, name := fam.pick(8); name != "lmroman8-regular" {
		t.Errorf("expected lmroman8 at 8pt, got: %s", name)
	}
	if _, name := fam.pick(10.95); name != "lmroman10-regular" {
		t.Errorf("expected lmroman10 at 10.95pt, got: %s", name)
	}
	if opticalOf(goregular.TTF) != nil {
		t.Error("expected no table for the Go font")
	}
}

func TestDriverBoxes(t *testing.T) {
	t.Parallel()
	src := `\hsize=4in
\noindent\vrule width 10pt height 5pt depth 1pt\ text
\par\noindent\hbox to 2in{a\hrulefill b}
\par\noindent\hbox to 2in{a\dotfill b}
\special{gotex:<rect x="{?x}" y="{?y}" width="10" height="10" fill="red"/>}
\bye`
	doc := ivyCompile(t, src, goFaces)
	r := drawPage(t, doc, 0)
	list := r.String()
	if strings.Count(list, "fill [") < 3 {
		t.Errorf("expected the rule, the leader and the special, got:\n%s", list)
	}
	dots := 0
	for _, s := range r.text {
		if s == "" {
			dots++
		}
	}
	if dots == 0 {
		t.Errorf("expected the dots of the leader, got: %q", r.text)
	}
}

func TestDriverLinks(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\usepackage{hyperref}\begin{document}
\href{https://example.com}{a link} and \hypertarget{here}{a target} and \hyperlink{here}{a jump}.
\end{document}`, goFaces)
	links, dests := doc.Links(t.Context(), 0)
	var uri, dest bool
	for _, l := range links {
		uri = uri || l.URI == "https://example.com"
		dest = dest || l.Dest == "here"
		if l.Rect.Width() <= 0 || l.Rect.Height() <= 0 {
			t.Errorf("expected a rectangle, got: %v", l.Rect)
		}
	}
	if !uri || !dest {
		t.Errorf("expected the link and the jump, got: %+v", links)
	}
	if len(dests) != 1 || dests[0].Name != "here" {
		t.Errorf("expected the destination, got: %+v", dests)
	}
	// DrawPage gives them to a device that is a display.Linker
	list := drawPage(t, doc, 0).String()
	for _, want := range []string{`"https://example.com"`, `#"here"`, `anchor "here"`} {
		if !strings.Contains(list, want) {
			t.Errorf("expected %s in the display list, got:\n%s", want, list)
		}
	}
}

func TestDriverOutline(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
\tableofcontents
\section{Alpha}Text.\subsection{Beta}Text.\newpage\section{Gamma}Text.
\end{document}`, goFaces)
	tests := []struct {
		page int
		want []string
	}{
		{0, []string{`bookmark 1 "Alpha"`, `bookmark 2 "Beta"`}},
		{1, []string{`bookmark 1 "Gamma"`}},
	}
	for _, test := range tests {
		list := drawPage(t, doc, test.page).String()
		for _, want := range test.want {
			if !strings.Contains(list, want) {
				t.Errorf("expected %s on page %d, got:\n%s", want, test.page, list)
			}
		}
	}
}

func TestDriverMath(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `Inline $a^2+b^2$ and $$\int_0^1 x\,dx$$\bye`, latinModern(t))
	r := drawPage(t, doc, 0)
	joined := strings.Join(r.text, "")
	for _, want := range []string{"a", "2", "+", "∫"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected the glyph %q of a formula, got: %q", want, r.text)
		}
	}
	box, color, ok := doc.mathBox(`<svg data-ivy-math="0"><g fill="#ff0000" transform="translate(0,1)"></g></svg>`)
	if !ok || box == nil || color != 0xff0000 {
		t.Errorf("expected the box 0 in red, got: %v %x %v", box, color, ok)
	}
	if _, _, ok := doc.mathBox(`<svg/>`); ok {
		t.Error("expected no box")
	}
}

func TestDriverTransform(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\usepackage{graphicx}\begin{document}
\scalebox{2}{Big} \rotatebox{90}{Up} \fbox{Framed} \underline{Under}
\end{document}`, goFaces)
	r := drawPage(t, doc, 0)
	list := r.String()
	if !strings.Contains(list, "concat {2 -0 -0 2") {
		t.Errorf("expected the scale of \\scalebox, got:\n%s", list)
	}
	// the matrix of \rotatebox{90} in the space of the page, with y down
	if !strings.Contains(list, "concat {6.123234e-17 1 -1 6.123234e-17") {
		t.Errorf("expected the rotation of \\rotatebox, got:\n%s", list)
	}
}

func TestStopped(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	s := ivyState{done: ctx.Done()}
	for range 2048 {
		if s.stopped() {
			t.Fatal("expected a run that goes on")
		}
	}
	cancel()
	stopped := false
	for range 2048 {
		stopped = stopped || s.stopped()
	}
	if !stopped {
		t.Error("expected a run that stops")
	}
	_, err := Compile(ctx, []byte(`x\bye`), RunOptions{Faces: goFaces})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	if _, err := Compile(t.Context(), []byte(`x`), RunOptions{}); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("expected ErrUnsupported without a font, got: %v", err)
	}
}

func TestSpecialsParse(t *testing.T) {
	t.Parallel()
	tree, err := parseSpecials(`<g transform="translate(1,2)"><path d="M0 0L10 0L10 10Z" stroke="blue" fill="none"/>`, 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	var l display.List
	tree.draw(&l)
	if !strings.Contains(l.String(), "stroke [") {
		t.Errorf("expected the stroke of the path, got:\n%s", l.String())
	}
	if _, err := parseSpecials(`<g><path d="`, 10, 10); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	if err := drawSVGImage(&l, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><circle cx="5" cy="5" r="5" fill="url(#g)"/><defs><linearGradient id="g"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs></svg>`), 20, 20); err != nil {
		t.Fatal(err)
	}
}
