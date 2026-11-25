package engine

import "testing"

func TestMathAtoms(t *testing.T) {
	t.Parallel()
	// The breaks are the ends of the atoms of the class Bin or Rel that a
	// Rel does not follow, as TeX inserts its penalties.
	tests := []struct {
		src    string
		breaks []string
		ok     bool
	}{
		{`A_{q,n}=A_{p,n}`, []string{`A_{q,n}=`}, true},
		{`a+b-c`, []string{`a+`, `a+b-`}, true},
		{`-a=-b`, []string{`-a=`}, true},
		{`a\le b<c`, []string{`a\le`, `a\le b<`}, true},
		{`x:=y`, []string{`x:=`}, true},
		{`(a+)`, nil, true},
		{`a+`, nil, true},
		{`\frac12=x^{a=b}_2`, []string{`\frac12=`}, true},
		{`\left(a=b\right)+c`, []string{`\left(a=b\right)+`}, true},
		{`\begin{matrix}a=b\end{matrix}`, nil, true},
		{`\overset{\text{def}}{=}x`, []string{`\overset{\text{def}}{=}`}, true},
		{`\not= x`, []string{`\not=`}, true},
		{`x' = \hat y_i`, []string{`x' =`}, true},
		{`\sum_{i=1}^n -a`, nil, true},
		{`a\cdot b`, []string{`a\cdot`}, true},
		{`\sqrt[3]{x}+1`, []string{`\sqrt[3]{x}+`}, true},
		{`\displaystyle a=b`, nil, false},
		{`a \over b`, nil, false},
		{`{a=b`, nil, false},
		{`a=b\label{x}`, nil, false},
	}
	for _, test := range tests {
		atoms, ok := mathAtoms(test.src)
		if ok != test.ok {
			t.Errorf("%s: expected ok %t, got: %t", test.src, test.ok, ok)
			continue
		}
		var got []string
		for k, a := range atoms {
			if k+1 < len(atoms) && atoms[k+1].class != atomRel && (a.class == atomRel || a.class == atomBin) {
				got = append(got, test.src[:a.end])
			}
		}
		if len(got) != len(test.breaks) {
			t.Errorf("%s: expected the breaks %q, got: %q", test.src, test.breaks, got)
			continue
		}
		for i := range got {
			if got[i] != test.breaks[i] {
				t.Errorf("%s: expected the breaks %q, got: %q", test.src, test.breaks, got)
				break
			}
		}
	}
}
