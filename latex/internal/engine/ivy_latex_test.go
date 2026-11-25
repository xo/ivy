package engine

import (
	"slices"
	"strings"
	"testing"
)

// indentedLine reports whether the line of page 0 of doc that starts with
// the character ch starts with the box of \parindent, and whether it found
// the line.
func indentedLine(doc *Doc, ch rune) (indented, found bool) {
	var walk func(b *boxNode) (bool, bool)
	walk = func(b *boxNode) (bool, bool) {
		for i, n := range b.list {
			switch c := n.(type) {
			case charNode:
				if c.ch == ch && i <= 1 && b.kind == hbox {
					_, box := b.list[0].(*boxNode)
					return i == 1 && box, true
				}
			case *boxNode:
				if ok, found := walk(c); found {
					return ok, true
				}
			}
		}
		return false, false
	}
	return walk(doc.pages[0])
}

func TestIvyEndpe(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
Before the list.
\begin{itemize}\item One.\end{itemize}
Xafter the list, in the same paragraph.
\begin{itemize}\item Two.\end{itemize}

Yafter a blank line.
\end{document}`, goFaces)
	// the text right after the list goes on without an indent, and a blank
	// line starts an indented paragraph
	tests := []struct {
		ch   rune
		want bool
	}{
		{'B', true},
		{'X', false},
		{'Y', true},
	}
	for _, test := range tests {
		indented, found := indentedLine(doc, test.ch)
		if !found {
			t.Fatalf("expected a line that starts with %q", test.ch)
		}
		if indented != test.want {
			t.Errorf("expected the line of %q indented %t, got %t", test.ch, test.want, indented)
		}
	}
}

func TestIvyLogos(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}\LaTeX\end{document}`, goFaces)
	r := drawPage(t, doc, 0)
	var sizeL, sizeA float32
	for i, s := range r.text {
		switch {
		case strings.HasPrefix(s, "L"):
			sizeL = r.size[i]
		case s == "A":
			sizeA = r.size[i]
		}
	}
	if sizeL == 0 || sizeA == 0 || sizeA >= sizeL {
		t.Errorf("expected a small A after the L, got: %q %v", r.text, r.size)
	}
}

func TestIvyPageNumbers(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\pagestyle{plain}\begin{document}
\pagenumbering{roman}A\newpage B\newpage
\pagenumbering{arabic}C\newpage D\newpage\setcounter{page}{7}E
\end{document}`, goFaces)
	want := []string{"i", "ii", "1", "2", "7"}
	if doc.PageCount() != len(want) {
		t.Fatalf("expected %d pages, got %d", len(want), doc.PageCount())
	}
	for i, w := range want {
		r := drawPage(t, doc, i)
		if len(r.text) == 0 || r.text[len(r.text)-1] != w {
			t.Errorf("expected the number %q on page %d, got: %q", w, i+1, r.text)
		}
	}
}

func TestIvyNoBlankPage(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
A\newpage\thispagestyle{empty}\markboth{x}{y}\clearpage B
\end{document}`, goFaces)
	if n := doc.PageCount(); n != 2 {
		t.Errorf("expected 2 pages, got %d", n)
	}
}

func TestIvyFancyLeftmark(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\usepackage{fancyhdr}
\pagestyle{fancy}\fancyhf{}\fancyhead[L]{\leftmark}\fancyfoot[C]{\thepage}
\renewcommand{\footrulewidth}{0.4pt}
\begin{document}\makeatletter\sectionmark{Alpha}Text.\newpage More text.\end{document}`, goFaces)
	r := drawPage(t, doc, 1)
	if !slices.ContainsFunc(r.text, func(s string) bool { return strings.Contains(s, "ALPHA") }) {
		t.Errorf("expected the mark of the section in the head, got: %q", r.text)
	}
}

func TestIvyBabelProvide(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\usepackage{babel}\babelprovide[import,main]{french}
\begin{document}\contentsname. \chaptername.\end{document}`, goFaces)
	r := drawPage(t, doc, 0)
	if !strings.Contains(strings.Join(r.text, " "), "mati") {
		t.Errorf("expected the French name of the contents, got: %q", r.text)
	}
}

func TestIvyOpenBox(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\documentclass{article}\begin{document}
\begin{proof}A proof.\end{proof}
\end{document}`, goFaces)
	list := drawPage(t, doc, 0).String()
	// the open box is four rules, where a filled square is one
	if n := strings.Count(list, "fill"); n < 4 {
		t.Errorf("expected the four rules of the open box, got:\n%s", list)
	}
}
