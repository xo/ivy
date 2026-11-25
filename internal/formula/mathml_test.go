package formula

import (
	"reflect"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/texmath"
)

// mathml parses a formula of MathML, the elements xml in a math element,
// into the elements of the package.
func mathml(t *testing.T, xml string) *Element {
	t.Helper()
	doc, err := roxmltree.Parse(`<math xmlns="http://www.w3.org/1998/Math/MathML">` + xml + `</math>`)
	if err != nil {
		t.Fatal(err)
	}
	var conv func(n roxmltree.Node) *Element
	conv = func(n roxmltree.Node) *Element {
		e := &Element{Name: n.TagName().Name(), Attrs: map[string]string{}}
		for a := range n.Attributes() {
			e.Attrs[a.Name()] = a.Value()
		}
		for c := range n.Children() {
			if c.IsElement() {
				e.Children = append(e.Children, conv(c))
			} else if s, ok := c.Text(); ok {
				e.Text += s
			}
		}
		return e
	}
	return conv(doc.RootElement())
}

func TestMathML(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		xml  string
		want texmath.List
	}{
		{
			name: "fraction",
			xml:  `<mfrac><mi>a</mi><mrow><mi>b</mi><mo>+</mo><mn>1</mn></mrow></mfrac>`,
			want: texmath.List{texmath.Fraction{
				Num: italic("a"),
				Den: texmath.List{italic("b"), upright(texmath.Bin, "+"), upright(texmath.Ord, "1")},
			}},
		},
		{
			name: "roots",
			xml:  `<msqrt><mi>x</mi></msqrt><mroot><mi>y</mi><mn>3</mn></mroot>`,
			want: texmath.List{
				texmath.Radical{Body: italic("x")},
				texmath.Radical{Body: italic("y"), Degree: upright(texmath.Ord, "3")},
			},
		},
		{
			name: "sub and superscript",
			xml:  `<msubsup><mi>x</mi><mi>i</mi><mn>2</mn></msubsup>`,
			want: texmath.List{texmath.Scripts{Base: italic("x"), Sub: italic("i"), Sup: upright(texmath.Ord, "2")}},
		},
		{
			name: "sum",
			xml:  `<munderover><mo>∑</mo><mrow><mi>k</mi><mo>=</mo><mn>0</mn></mrow><mi>n</mi></munderover><mi>k</mi>`,
			want: texmath.List{
				texmath.Scripts{
					Base: texmath.Atom{Class: texmath.Op, Text: "∑", Variant: texmath.Upright, Large: true},
					Sub:  texmath.List{italic("k"), upright(texmath.Rel, "="), upright(texmath.Ord, "0")},
					Sup:  italic("n"),
				},
				italic("k"),
			},
		},
		{
			name: "matrix in fences",
			xml:  `<mrow><mo>(</mo><mtable><mtr><mtd><mi>a</mi></mtd><mtd><mi>b</mi></mtd></mtr><mtr><mtd><mi>c</mi></mtd><mtd><mi>d</mi></mtd></mtr></mtable><mo>)</mo></mrow>`,
			want: texmath.List{texmath.Atom{Class: texmath.Ord, Body: texmath.Delimited{
				Open: '(', Close: ')',
				Body: texmath.List{texmath.Array{
					Kind: texmath.ArrayMatrix,
					Rows: [][]texmath.Node{{italic("a"), italic("b")}, {italic("c"), italic("d")}},
				}},
			}}},
		},
		{
			name: "mfenced",
			xml:  `<mfenced open="[" close="]"><mi>a</mi><mi>b</mi></mfenced>`,
			want: texmath.List{texmath.Atom{Class: texmath.Ord, Body: texmath.Delimited{
				Open: '[', Close: ']',
				Body: texmath.List{italic("a"), upright(texmath.Punct, ","), italic("b")},
			}}},
		},
		{
			name: "function and class of TeX4ht",
			xml:  `<mi>sin</mi><mo>&#x2061;</mo><mi>x</mi><mo class="MathClass-rel">|</mo><mi>y</mi>`,
			want: texmath.List{
				texmath.Atom{Class: texmath.Op, Body: texmath.List{
					upright(texmath.Ord, "s"), upright(texmath.Ord, "i"), upright(texmath.Ord, "n"),
				}, Limits: texmath.LimitsSide},
				italic("x"), upright(texmath.Rel, "|"), italic("y"),
			},
		},
		{
			name: "accent",
			xml:  `<mover><mi>x</mi><mo>^</mo></mover>`,
			want: texmath.List{texmath.Accent{Base: italic("x"), Accent: 'ˆ'}},
		},
	}
	ts, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		got, block := MathML(mathml(t, tt.xml))
		if block {
			t.Errorf("%s: expected an inline formula", tt.name)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: expected\n%#v\ngot\n%#v", tt.name, tt.want, got)
		}
		if _, err := ts.Layout(got, 12, texmath.Text, okage.ColorBlack); err != nil {
			t.Errorf("%s: %v", tt.name, err)
		}
	}
}

func TestMathMLBlock(t *testing.T) {
	t.Parallel()
	e := mathml(t, `<mi>x</mi>`)
	e.Attrs["display"] = "block"
	if _, block := MathML(e); !block {
		t.Errorf("expected a formula of display block")
	}
}
