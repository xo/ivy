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
