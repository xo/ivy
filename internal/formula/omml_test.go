package formula

import (
	"reflect"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/texmath"
)

// omml parses an equation of Office Math: the elements xml in an m:oMath
// element.
func omml(t *testing.T, xml string) roxmltree.Node {
	t.Helper()
	doc, err := roxmltree.Parse(`<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` + xml + `</m:oMath>`)
	if err != nil {
		t.Fatal(err)
	}
	return doc.RootElement()
}

// run returns an m:r element with the text s.
func run(s string) string {
	return `<m:r><m:t>` + s + `</m:t></m:r>`
}

// italic returns the atom of the italic letter s.
func italic(s string) texmath.Atom {
	return texmath.Atom{Class: texmath.Ord, Text: s, Variant: texmath.Italic}
}

// upright returns the atom of the upright character s of the class c.
func upright(c texmath.Class, s string) texmath.Atom {
	return texmath.Atom{Class: c, Text: s, Variant: texmath.Upright}
}

func TestOMML(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		xml  string
		want texmath.List
	}{
		{
			name: "fraction",
			xml:  `<m:f><m:fPr><m:ctrlPr/></m:fPr><m:num>` + run("a") + `</m:num><m:den>` + run("b+1") + `</m:den></m:f>`,
			want: texmath.List{texmath.Fraction{
				Num: italic("a"),
				Den: texmath.List{italic("b"), upright(texmath.Bin, "+"), upright(texmath.Ord, "1")},
			}},
		},
		{
			name: "fraction without a bar",
			xml:  `<m:f><m:fPr><m:type m:val="noBar"/></m:fPr><m:num>` + run("n") + `</m:num><m:den>` + run("k") + `</m:den></m:f>`,
			want: texmath.List{texmath.Fraction{Num: italic("n"), Den: italic("k"), NoRule: true}},
		},
		{
			name: "square root",
			xml:  `<m:rad><m:radPr><m:degHide m:val="1"/></m:radPr><m:deg/><m:e>` + run("x") + `</m:e></m:rad>`,
			want: texmath.List{texmath.Radical{Body: italic("x")}},
		},
		{
			name: "cube root",
			xml:  `<m:rad><m:deg>` + run("3") + `</m:deg><m:e>` + run("y") + `</m:e></m:rad>`,
			want: texmath.List{texmath.Radical{Body: italic("y"), Degree: upright(texmath.Ord, "3")}},
		},
		{
			name: "sub and superscript",
			xml:  `<m:sSubSup><m:e>` + run("x") + `</m:e><m:sub>` + run("i") + `</m:sub><m:sup>` + run("2") + `</m:sup></m:sSubSup>`,
			want: texmath.List{texmath.Scripts{Base: italic("x"), Sub: italic("i"), Sup: upright(texmath.Ord, "2")}},
		},
		{
			name: "sum",
			xml: `<m:nary><m:naryPr><m:chr m:val="∑"/></m:naryPr><m:sub>` + run("k=0") + `</m:sub><m:sup>` + run("n") +
				`</m:sup><m:e>` + run("k") + `</m:e></m:nary>`,
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
			name: "integral without limits",
			xml:  `<m:nary><m:naryPr><m:subHide m:val="1"/><m:supHide m:val="1"/></m:naryPr><m:sub/><m:sup/><m:e>` + run("f") + `</m:e></m:nary>`,
			want: texmath.List{
				texmath.Atom{Class: texmath.Op, Text: "∫", Variant: texmath.Upright, Large: true, Limits: texmath.LimitsSide},
				italic("f"),
			},
		},
		{
			name: "matrix",
			xml: `<m:m><m:mPr><m:mcs><m:mc><m:mcPr><m:count m:val="2"/><m:mcJc m:val="left"/></m:mcPr></m:mc></m:mcs></m:mPr>` +
				`<m:mr><m:e>` + run("a") + `</m:e><m:e>` + run("b") + `</m:e></m:mr>` +
				`<m:mr><m:e>` + run("c") + `</m:e><m:e>` + run("d") + `</m:e></m:mr></m:m>`,
			want: texmath.List{texmath.Array{
				Kind:  texmath.ArrayMatrix,
				Rows:  [][]texmath.Node{{italic("a"), italic("b")}, {italic("c"), italic("d")}},
				Align: []texmath.Align{texmath.AlignLeft, texmath.AlignLeft},
			}},
		},
		{
			name: "delimiter",
			xml: `<m:d><m:dPr><m:begChr m:val="["/><m:endChr m:val=""/></m:dPr><m:e>` + run("a") + `</m:e><m:e>` +
				run("b") + `</m:e></m:d>`,
			want: texmath.List{texmath.Atom{Class: texmath.Ord, Body: texmath.Delimited{
				Open: '[',
				Body: texmath.List{italic("a"), texmath.Delimiter{Rune: '|', Class: texmath.Rel, Middle: true}, italic("b")},
			}}},
		},
		{
			name: "function",
			xml: `<m:func><m:fName><m:r><m:rPr><m:sty m:val="p"/></m:rPr><m:t>sin</m:t></m:r></m:fName><m:e>` +
				run("θ") + `</m:e></m:func>`,
			want: texmath.List{
				texmath.Atom{Class: texmath.Op, Body: texmath.List{
					upright(texmath.Ord, "s"), upright(texmath.Ord, "i"), upright(texmath.Ord, "n"),
				}, Limits: texmath.LimitsSide},
				italic("𝜃"),
			},
		},
		{
			name: "accent",
			xml:  `<m:acc><m:accPr><m:chr m:val="̃"/></m:accPr><m:e>` + run("x") + `</m:e></m:acc>`,
			want: texmath.List{texmath.Accent{Base: italic("x"), Accent: '˜'}},
		},
		{
			name: "normal text",
			xml:  `<m:r><m:rPr><m:nor/></m:rPr><w:rPr><w:b/></w:rPr><m:t>if x</m:t></m:r>`,
			want: texmath.List{texmath.TextRun{Text: "if x", Bold: true}},
		},
	}
	ts, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		got, ok := OMML(omml(t, tt.xml), 64)
		if !ok {
			t.Errorf("%s: expected the equation in the limits", tt.name)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: expected\n%#v\ngot\n%#v", tt.name, tt.want, got)
		}
		for _, style := range []texmath.Style{texmath.Display, texmath.Text} {
			obj, err := ts.Layout(got, 12, style, okage.ColorBlack)
			if err != nil {
				t.Errorf("%s: %v", tt.name, err)
				continue
			}
			if w, h := obj.Size(); !(w > 0 && h > obj.Depth() && obj.Depth() >= 0) {
				t.Errorf("%s: expected a box with a size, got %g by %g with a depth of %g", tt.name, w, h, obj.Depth())
			}
		}
	}
}

func TestOMMLEquationArray(t *testing.T) {
	t.Parallel()
	got, _ := OMML(omml(t, `<m:eqArr><m:e>`+run("x&amp;=1")+`</m:e><m:e>`+run("y&amp;=2")+`</m:e></m:eqArr>`), 64)
	want := texmath.List{texmath.Array{Kind: texmath.ArrayAligned, Rows: [][]texmath.Node{
		{texmath.List{italic("x")}, texmath.List{upright(texmath.Rel, "="), upright(texmath.Ord, "1")}},
		{texmath.List{italic("y")}, texmath.List{upright(texmath.Rel, "="), upright(texmath.Ord, "2")}},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected\n%#v\ngot\n%#v", want, got)
	}
}

func TestOMMLDepth(t *testing.T) {
	t.Parallel()
	xml := strings.Repeat(`<m:sSup><m:e>`, 200) + run("x") + strings.Repeat(`</m:e><m:sup>`+run("2")+`</m:sup></m:sSup>`, 200)
	if _, ok := OMML(omml(t, xml), 1000); ok {
		t.Errorf("expected an equation of 200 levels past the limit")
	}
}

func TestAtoms(t *testing.T) {
	t.Parallel()
	got := Atoms(nil, "f(x) = 2.5x - y, α", texmath.Italic)
	want := texmath.List{
		italic("f"), upright(texmath.Open, "("), italic("x"), upright(texmath.Close, ")"),
		upright(texmath.Rel, "="), upright(texmath.Ord, "2.5"), italic("x"), upright(texmath.Bin, "−"),
		italic("y"), upright(texmath.Punct, ","), italic("𝛼"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected\n%#v\ngot\n%#v", want, got)
	}
}
