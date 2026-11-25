package engine

import (
	"math"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/okage"
)

// glyph is one glyph that a page draws: the text of its cluster, its
// origin and the size and the face of its run.
type glyph struct {
	text string
	x, y float32
	size float32
	face any
}

// glyphs records the glyphs of a page.
type glyphs struct {
	*display.List
	all []glyph
}

func (g *glyphs) FillGlyphs(run display.GlyphRun, shader okage.Shader) {
	for _, gl := range run.Glyphs {
		text := ""
		if gl.End > gl.Start && gl.End <= len(run.Text) {
			text = run.Text[gl.Start:gl.End]
		}
		g.all = append(g.all, glyph{text: text, x: gl.X, y: gl.Y, size: run.Size, face: run.Face})
	}
	g.List.FillGlyphs(run, shader)
}

// drawGlyphs draws page n of doc and returns its glyphs.
func drawGlyphs(t *testing.T, doc *Doc, n int) []glyph {
	t.Helper()
	g := &glyphs{List: new(display.List)}
	if err := doc.DrawPage(t.Context(), g, n, nil, 1); err != nil {
		t.Fatal(err)
	}
	return g.all
}

// find returns the first glyph of text after the glyph from, and false
// when there is none.
func find(gs []glyph, from int, text string) (int, bool) {
	for i := max(from, 0); i < len(gs); i++ {
		if gs[i].text == text {
			return i, true
		}
	}
	return -1, false
}

func near(a, b, tol float32) bool {
	return math.Abs(float64(a-b)) <= float64(tol)
}

func TestIvyTextEllipsis(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
a\ldots b\par
\end{document}`, latinModern(t))
	gs := drawGlyphs(t, doc, 0)
	a, ok := find(gs, 0, "a")
	if !ok || a+4 >= len(gs) {
		t.Fatalf("expected a, three periods and b, got: %v", gs)
	}
	// pdfLaTeX: a period of 2.78pt and a kern of \fontdimen3, 1.67pt, for
	// each period
	for i := a + 1; i <= a+3; i++ {
		if gs[i].text != "." {
			t.Fatalf("expected a period, got: %v", gs[a:a+5])
		}
		if d := gs[i+1].x - gs[i].x; !near(d, 4.44, 0.05) {
			t.Errorf("expected 4.44pt from a period to the next glyph, got: %g", d)
		}
	}
}

func TestIvyTextEmph(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
r \textit{x \emph{y} z} {\em u \emph{v}} \emph{w}
\end{document}`, latinModern(t))
	gs := drawGlyphs(t, doc, 0)
	face := map[string]any{}
	for _, g := range gs {
		face[g.text] = g.face
	}
	for _, test := range []struct {
		text    string
		upright bool
	}{
		{"x", false}, {"y", true}, {"z", false}, {"u", false}, {"v", true}, {"w", false},
	} {
		if got := face[test.text] == face["r"]; got != test.upright {
			t.Errorf("expected %s upright %t, got %t", test.text, test.upright, got)
		}
	}
}

func TestIvyTextRobustLogos(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
\tableofcontents
\section{About \LaTeX\ and \TeX}
\begin{figure}\caption{A \LaTeX\ caption}\end{figure}
\end{document}`, latinModern(t))
	gs := drawGlyphs(t, doc, 0)
	var b strings.Builder
	for _, g := range gs {
		b.WriteString(g.text)
	}
	text := b.String()
	if strings.Contains(text, "@") || strings.Contains(text, "ivy") {
		t.Errorf("expected the logos, got the names of their macros: %q", text)
	}
	// the A of the logo in the contents and in the caption
	if n := strings.Count(text, "LATEX"); n < 3 {
		t.Errorf("expected the logo in the contents, the heading and the caption, got %d: %q", n, text)
	}
	// the caption is one line: its glyphs share a baseline
	f, ok := find(gs, 0, "F")
	if !ok {
		t.Fatalf("expected the caption, got: %q", text)
	}
	for _, g := range gs[f:] {
		if g.text != "A" && g.text != "E" && !near(g.y, gs[f].y, 0.01) {
			t.Errorf("expected the caption on one line, got %q at %g and %g", g.text, g.y, gs[f].y)
		}
	}
}

func TestIvyTextFootnoteMark(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
Text\footnote{Note.} more.
\end{document}`, latinModern(t))
	gs := drawGlyphs(t, doc, 0)
	tests := []struct {
		after       string
		size, shift float32
	}{
		// pdfLaTeX: the mark at 7pt, raised by \fontdimen14 of cmsy10,
		// 3.63pt, and in the note at 8pt, the mark at 6pt, raised 2.82pt
		{"t", 7, 3.63},
		{"", 6, 2.82},
	}
	from := 0
	for _, test := range tests {
		m, ok := find(gs, from, "1")
		if !ok || m+1 >= len(gs) {
			t.Fatalf("expected a mark, got: %v", gs)
		}
		base := gs[m+1]
		if test.after != "" {
			base = gs[m-1]
		}
		if gs[m].size != test.size {
			t.Errorf("expected a mark of %gpt, got: %g", test.size, gs[m].size)
		}
		if d := base.y - gs[m].y; !near(d, test.shift, 0.02) {
			t.Errorf("expected a mark raised %gpt, got: %g", test.shift, d)
		}
		from = m + 1
	}
}

func TestIvyTextShortTitles(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
\tableofcontents\listoffigures
\section[Short]{Long}
\subsection[Brief]{Lengthy}
\begin{figure}\caption[Tiny]{Large}\end{figure}
\end{document}`, latinModern(t))
	gs := drawGlyphs(t, doc, 0)
	var b strings.Builder
	for _, g := range gs {
		b.WriteString(g.text)
	}
	text := b.String()
	for _, want := range []string{"Short", "Brief", "Tiny", "Long", "Lengthy", "Large"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q, got: %q", want, text)
		}
	}
	if strings.Contains(text, "[") || strings.Count(text, "Long") != 1 {
		t.Errorf("expected the short titles in the lists, got: %q", text)
	}
}

func TestIvyTextSans(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
\noindent a \textsf{b \textit{c} \textbf{d} \textbf{\textit{e}} \emph{f}} \textit{g \textsf{h}} {\sffamily i \rmfamily j}
\end{document}`, latinModern(t))
	gs := drawGlyphs(t, doc, 0)
	face := map[string]any{}
	x := map[string]float32{}
	for _, g := range gs {
		face[g.text], x[g.text] = g.face, g.x
	}
	// pdfLaTeX: Latin Modern Sans and its oblique and bold extended shapes;
	// \rmfamily goes back to the roman family
	for _, test := range []struct {
		a, b string
		same bool
	}{
		{"a", "b", false}, {"b", "c", false}, {"b", "d", false}, {"c", "d", false},
		{"e", "c", false}, {"e", "d", false}, {"f", "c", true}, {"h", "c", true},
		{"g", "h", false}, {"i", "b", true}, {"j", "a", true},
	} {
		if got := face[test.a] == face[test.b]; got != test.same {
			t.Errorf("expected the faces of %s and %s the same %t, got %t", test.a, test.b, test.same, got)
		}
	}
	// pdfLaTeX: 8.51pt from b to c, b and the space of cmss10
	if d := x["c"] - x["b"]; !near(d, 8.51, 0.05) {
		t.Errorf("expected 8.51pt from b to c, got: %g", d)
	}
}

func TestIvyTextControlSpace(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
\noindent a\ b\par
\noindent a\     % comment
  b\par
\end{document}`, latinModern(t))
	gs := drawGlyphs(t, doc, 0)
	var xs []float32
	for _, g := range gs {
		if g.text == "b" {
			xs = append(xs, g.x)
		}
	}
	// TeX skips the spaces after a control space, as after a control word
	if len(xs) != 2 || !near(xs[0], xs[1], 0.01) {
		t.Errorf("expected b at the same place on both lines, got: %v", xs)
	}
}
