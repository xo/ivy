package mathsvg

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/texmath"

	"github.com/xo/ivy"
)

func TestRender(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "fonts", "lm", "latinmodern-math.otf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, font := range [][]byte{data, nil} {
		r, err := New(font)
		if err != nil {
			t.Fatal(err)
		}
		svg, m, err := r.RenderSVGMetrics(`\frac{a}{b}`, 10)
		if err != nil {
			t.Fatal(err)
		}
		if !(m.Width > 0 && m.Height > 0 && m.Depth > 0) {
			t.Errorf("expected a box with a width, a height and a depth, got: %+v", m)
		}
		if !strings.Contains(svg, `data-ivy-math="0"`) || !strings.Contains(svg, `fill="currentColor"`) || !strings.Contains(svg, "<path d=\"M") {
			t.Errorf("expected the SVG of go-tex/math with the box of ivy, got: %s", svg)
		}
		box, ok := r.Box(0)
		if !ok || len(box.Items) == 0 {
			t.Fatal("expected the box of the formula")
		}
		var glyphs, rules int
		for _, it := range box.Items {
			switch it.Kind {
			case texmath.ItemGlyph:
				glyphs++
				if it.Face != r.MathFace() {
					t.Error("expected the glyphs of the math face")
				}
			case texmath.ItemRule:
				rules++
			}
		}
		if glyphs != 2 || rules != 1 {
			t.Errorf("expected 2 glyphs and the rule of the fraction, got: %d and %d", glyphs, rules)
		}
		if r.DrawFace() == nil {
			t.Error("expected a face that draws")
		}
		_, dm, err := r.RenderDisplaySVGMetrics(`\sum_{n=1}^\infty x_n`, 10)
		if err != nil {
			t.Fatal(err)
		}
		if dm.Height <= m.Height {
			t.Errorf("expected a sum in display taller than a fraction in text, got: %+v", dm)
		}
		if _, ok := r.Box(2); ok {
			t.Error("expected no box 2")
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()
	r, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = r.RenderSVGMetrics(`a + \nosuchcommand`, 10)
	// the engine reads the name in the words of go-tex/math
	if err == nil || !strings.Contains(err.Error(), `unknown command \nosuchcommand`) {
		t.Errorf("expected an unknown command, got: %v", err)
	}
	if !errors.Is(err, okage.ErrTeXMathUnknownCommand) {
		t.Errorf("expected ErrTeXMathUnknownCommand, got: %v", err)
	}
}

func TestColor(t *testing.T) {
	t.Parallel()
	r, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	svg, _, err := r.RenderSVGMetrics(`\color{red} x`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg, `fill="#ff0000"`) {
		t.Errorf("expected the color of the item, got: %s", svg)
	}
}

func TestNotMath(t *testing.T) {
	t.Parallel()
	if _, err := New([]byte("not a font")); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	if got := ftoa(-0.0001); got != "0" {
		t.Errorf("expected 0, got: %q", got)
	}
	if got := ftoa(1.5); got != "1.5" {
		t.Errorf("expected 1.5, got: %q", got)
	}
}

func TestBasePrimes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"x", "x"},
		{"x'", `x\prime `},
		{"f''(x)", `f\prime \prime (x)`},
		{`x'^2`, `x\prime ^2`},
		{`\text{it's} y'`, `\text{it's} y\prime `},
		{`\mbox {a'{b'}} c'`, `\mbox {a'{b'}} c\prime `},
		{`\'e`, `\'e`},
		{`\text{a'`, `\text{a'`},
		{`x^{\prime} + y^\prime`, `x\prime  + y\prime`},
		{`x^{\prime\prime}_1`, `x\prime\prime _1`},
	}
	for _, test := range tests {
		if got := basePrimes(test.in); got != test.want {
			t.Errorf("basePrimes(%q): expected %q, got %q", test.in, test.want, got)
		}
	}
}

func TestSubscriptsOfTeX(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "fonts", "lm", "latinmodern-math.otf"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(data)
	if err != nil {
		t.Fatal(err)
	}
	// pdfLaTeX shifts the n of K_n down by sub1, 1.5pt, and the i of
	// \hat x_i too. It shifts the subscript of x_i^2 down by 2.6pt.
	tests := []struct {
		tex  string
		text string
		want float64
	}{
		{`K_n`, "𝑛", 1.5},
		{`\hat x_i`, "𝑖", 1.5},
		{`x_i^2`, "𝑖", 2.6},
	}
	for i, test := range tests {
		if _, _, err := r.RenderSVGMetrics(test.tex, 10); err != nil {
			t.Fatal(err)
		}
		box, _ := r.Box(i)
		var got float64
		for _, it := range box.Items {
			if it.Text == test.text {
				got = it.Y
			}
		}
		if got < test.want-0.05 || got > test.want+0.05 {
			t.Errorf("%s: expected the subscript %.2fpt down, got: %.3f", test.tex, test.want, got)
		}
	}
}

func TestMathDots(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{`1,\dots,n`, `1,` + lowDots + `,n`},
		{`a+\dots+b`, `a+` + centerDots + `+b`},
		{`a\dots \le b`, `a` + centerDots + ` \le b`},
		{`x_1\cdots x_n`, `x_1` + centerDots + ` x_n`},
		{`\ldots\dotsc\dotsb`, lowDots + lowDots + centerDots},
		{`\text{a\dots} \dotso`, `\text{a\dots} ` + lowDots},
		{`\dotsx`, `\dotsx`},
		{`x`, `x`},
	}
	for _, test := range tests {
		if got := mathDots(test.in); got != test.want {
			t.Errorf("mathDots(%q): expected %q, got %q", test.in, test.want, got)
		}
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "fonts", "lm", "latinmodern-math.otf"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(data)
	if err != nil {
		t.Fatal(err)
	}
	// pdfLaTeX sets \ldots 11.67pt wide at 10pt: three dots and two thin
	// spaces.
	_, m, err := r.RenderSVGMetrics(`\ldots`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if m.Width < 11 || m.Width > 12.5 {
		t.Errorf("expected the dots 11.67pt wide, got: %.3f", m.Width)
	}
}

func TestDisplayCells(t *testing.T) {
	t.Parallel()
	const ds = `\displaystyle `
	tests := []struct {
		in, want string
	}{
		{`a=b`, `a=b`},
		{`\begin{split}a&=b\\&=c\end{split}`, `\begin{split}` + ds + `a&` + ds + `=b\\` + ds + `&` + ds + `=c\end{split}`},
		{`\begin{aligned}[t]a\\[2pt]b\\\end{aligned}`, `\begin{aligned}[t]` + ds + `a\\[2pt]` + ds + `b\\\end{aligned}`},
		{`\begin{alignedat}{2}a&b\end{alignedat}`, `\begin{alignedat}{2}` + ds + `a&` + ds + `b\end{alignedat}`},
		{`\begin{gathered}\begin{matrix}a&b\end{matrix}\\c\end{gathered}`, `\begin{gathered}` + ds + `\begin{matrix}a&b\end{matrix}\\` + ds + `c\end{gathered}`},
		{`\begin{pmatrix}a&b\end{pmatrix}`, `\begin{pmatrix}a&b\end{pmatrix}`},
		{`\begin{split}\text{a&b}&{x&y}\end{split}`, `\begin{split}` + ds + `\text{a&b}&` + ds + `{x&y}\end{split}`},
	}
	for _, test := range tests {
		if got := displayCells(test.in); got != test.want {
			t.Errorf("displayCells(%q):\nexpected %q\ngot      %q", test.in, test.want, got)
		}
	}
}
