package drawpage

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
)

// shapesContent is a content.xml of a text with a style of a shape and the
// shapes of the tests, in its order: a rectangle, a turned square, a line,
// a group, a square whose style gives the width of a marker and no marker,
// and a line with a marker.
const shapesContent = `<office:document-content xmlns:office="` + nsOffice + `" xmlns:style="` + nsStyle + `" xmlns:draw="` + nsDraw + `" xmlns:svg="` + nsSVG + `" xmlns:text="` + nsText + `" xmlns:fo="` + nsFO + `">
<office:automatic-styles>
<style:style style:name="gr1" style:family="graphic"><style:graphic-properties draw:fill="solid" draw:fill-color="#ff0000" draw:stroke="none"/></style:style>
<style:style style:name="gr2" style:family="graphic"><style:graphic-properties draw:stroke="solid" svg:stroke-width="4pt"/></style:style>
<style:style style:name="gr3" style:family="graphic"><style:graphic-properties draw:stroke="solid" svg:stroke-width="2pt" draw:marker-start="" draw:marker-start-width="10pt"/></style:style>
<style:style style:name="gr4" style:family="graphic"><style:graphic-properties draw:stroke="solid" svg:stroke-width="2pt" draw:marker-end="Arrow" draw:marker-end-width="10pt"/></style:style>
</office:automatic-styles>
<office:body><office:text><text:p>
<draw:rect draw:style-name="gr1" svg:x="10pt" svg:y="20pt" svg:width="30pt" svg:height="40pt"/>
<draw:rect draw:style-name="gr1" draw:transform="rotate (0.785398163397448) translate (0pt 0pt)" svg:width="10pt" svg:height="10pt"/>
<draw:line draw:style-name="gr2" svg:x1="0pt" svg:y1="5pt" svg:x2="100pt" svg:y2="5pt"/>
<draw:g><draw:rect draw:style-name="gr1" svg:x="0pt" svg:y="0pt" svg:width="10pt" svg:height="10pt"/><draw:ellipse draw:style-name="gr1" svg:x="50pt" svg:y="20pt" svg:width="10pt" svg:height="10pt"/></draw:g>
<draw:rect draw:style-name="gr3" svg:x="0pt" svg:y="0pt" svg:width="10pt" svg:height="10pt"/>
<draw:line draw:style-name="gr4" svg:x1="0pt" svg:y1="0pt" svg:x2="100pt" svg:y2="0pt"/>
</text:p></office:text></office:body></office:document-content>`

// recorder is a display.Device that counts the paths that it fills.
type recorder struct {
	display.Device
	fills int
}

func (r *recorder) Save()                                              {}
func (r *recorder) Restore()                                           {}
func (r *recorder) Concat(okage.Transform)                             {}
func (r *recorder) ClipPath(*okage.Path, okage.FillRule)               {}
func (r *recorder) StrokePath(*okage.Path, okage.Stroke, okage.Shader) {}
func (r *recorder) FillPath(*okage.Path, okage.FillRule, okage.Shader) {
	r.fills++
}

func TestShapes(t *testing.T) {
	doc, err := roxmltree.Parse(shapesContent)
	if err != nil {
		t.Fatal(err)
	}
	read := func(context.Context, string) ([]byte, error) { return nil, errors.New("no entry") }
	s := NewShapes(Options{Config: ivy.Config{}, Read: read}, doc, nil)
	var shapes []roxmltree.Node
	for n := range doc.Descendants() {
		if parent, ok := n.Parent(); ok && n.IsElement() && parent.IsElement() && parent.TagName().Name() == "p" {
			shapes = append(shapes, n)
		}
	}
	if len(shapes) != 6 {
		t.Fatalf("expected 6 shapes, got: %d", len(shapes))
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 0.01 }
	for i, exp := range [...][4]float64{
		{10, 20, 30, 40},
		// the corner (10, 0) turns 45 degrees counterclockwise, to y < 0
		{0, -7.07, 14.14, 14.14},
		// the half of the line of 4 points goes around it
		{-2, 3, 104, 4},
		{0, 0, 60, 30},
		// the width of a marker counts only with a marker
		{-1, -1, 12, 12},
		{-10, -10, 120, 20},
	} {
		sh, ok := s.Shape(t.Context(), shapes[i], true)
		if !ok {
			t.Fatalf("shape %d: expected a shape", i)
		}
		if !near(sh.X, exp[0]) || !near(sh.Y, exp[1]) || !near(sh.W, exp[2]) || !near(sh.H, exp[3]) {
			t.Errorf("shape %d: expected the box %v, got: %v %v %v %v", i, exp, sh.X, sh.Y, sh.W, sh.H)
		}
		if w, h := sh.Size(); w != sh.W || h != sh.H {
			t.Errorf("shape %d: expected the size of the box, got: %v by %v", i, w, h)
		}
	}
	group, _ := s.Shape(t.Context(), shapes[3], true)
	rect, _ := okage.RectFromXYWH(0, 0, 60, 30)
	r := &recorder{}
	if err := group.Draw(t.Context(), r, rect); err != nil {
		t.Fatal(err)
	}
	if r.fills != 2 {
		t.Errorf("expected the group to fill 2 paths, got: %d", r.fills)
	}
}

// TestGradient checks the fill of the frames of a text: a frame with a
// gradient fills its box, and a frame with a solid fill or without a size
// gives no shape.
func TestGradient(t *testing.T) {
	const content = `<office:document-content xmlns:office="` + nsOffice + `" xmlns:style="` + nsStyle + `" xmlns:draw="` + nsDraw + `" xmlns:svg="` + nsSVG + `" xmlns:text="` + nsText + `">
<office:styles><draw:gradient draw:name="g1" draw:style="axial" draw:start-color="#ff0000" draw:end-color="#0000ff" draw:angle="1800"/></office:styles>
<office:automatic-styles>
<style:style style:name="fr1" style:family="graphic"><style:graphic-properties draw:fill="gradient" draw:fill-gradient-name="g1"/></style:style>
<style:style style:name="fr2" style:family="graphic"><style:graphic-properties draw:fill="solid" draw:fill-color="#ff0000"/></style:style>
</office:automatic-styles>
<office:body><office:text><text:p>
<draw:frame draw:style-name="fr1" svg:width="100pt" svg:height="50pt"><draw:text-box/></draw:frame>
<draw:frame draw:style-name="fr2" svg:width="100pt" svg:height="50pt"><draw:text-box/></draw:frame>
</text:p></office:text></office:body></office:document-content>`
	doc, err := roxmltree.Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	read := func(context.Context, string) ([]byte, error) { return nil, errors.New("no entry") }
	s := NewShapes(Options{Config: ivy.Config{}, Read: read}, doc, nil)
	var frames []roxmltree.Node
	for n := range doc.Descendants() {
		if n.IsElement() && n.TagName().Name() == "frame" {
			frames = append(frames, n)
		}
	}
	sh, ok := s.Gradient(t.Context(), frames[0], 100, 50)
	if !ok {
		t.Fatal("expected the gradient of the frame")
	}
	if w, h := sh.Size(); w != 100 || h != 50 {
		t.Errorf("expected the size 100 by 50, got: %v by %v", w, h)
	}
	r := &recorder{}
	rect, _ := okage.RectFromXYWH(0, 0, 100, 80)
	if err := sh.Draw(t.Context(), r, rect); err != nil {
		t.Fatal(err)
	}
	if r.fills != 1 {
		t.Errorf("expected the gradient to fill 1 path, got: %d", r.fills)
	}
	if _, ok := s.Gradient(t.Context(), frames[1], 100, 50); ok {
		t.Error("expected no shape for a solid fill")
	}
	if _, ok := s.Gradient(t.Context(), frames[0], 0, 50); ok {
		t.Error("expected no shape for a frame without a width")
	}
}

func TestRunStyleEastAsian(t *testing.T) {
	b := &builder{st: newStyles()}
	b.st.fonts["Mincho"] = "MS Mincho"
	for _, test := range []struct {
		tp   props
		font string
		lang string
	}{
		{props{"style:font-name-asian": "Mincho", "style:language-asian": "ja", "style:country-asian": "JP"}, "MS Mincho", "ja-JP"},
		{props{"style:font-family-asian": "'SimSun', serif", "style:language-asian": "zh", "style:country-asian": "none"}, "SimSun", "zh"},
		{props{"style:language-asian": "en", "style:country-asian": "US"}, "-", "-"},
	} {
		s := b.runStyle(test.tp)
		if v := s.EastAsian.Or("-"); v != test.font {
			t.Errorf("%v: expected the East Asian font %q, got: %q", test.tp, test.font, v)
		}
		if v := s.Lang.Or("-"); v != test.lang {
			t.Errorf("%v: expected the language %q, got: %q", test.tp, test.lang, v)
		}
	}
}

func TestClip(t *testing.T) {
	for _, test := range []struct {
		s   string
		exp drawingml.Insets
	}{
		{"rect(1in, 2in, 3in, 4in)", drawingml.Insets{T: 0.1, R: 0.2, B: 0.3, L: 0.4}},
		{"rect(72pt 0pt auto 144pt)", drawingml.Insets{T: 0.1, L: 0.2}},
		{"rect(0cm, 0cm, 0cm, 0cm)", drawingml.Insets{}},
		// nothing of the picture is left
		{"rect(5in, 0in, 5in, 0in)", drawingml.Insets{}},
		{"rect(-1in, 0in, 0in, 0in)", drawingml.Insets{}},
		{"auto", drawingml.Insets{}},
		{"rect(1in, 2in)", drawingml.Insets{}},
	} {
		got := clip(test.s, 720, 720)
		near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
		if !near(got.T, test.exp.T) || !near(got.R, test.exp.R) || !near(got.B, test.exp.B) || !near(got.L, test.exp.L) {
			t.Errorf("%s: expected %+v, got: %+v", test.s, test.exp, got)
		}
	}
	if got := clip("rect(1in, 1in, 1in, 1in)", 0, 720); !got.Zero() {
		t.Errorf("expected no clip of a picture without a size, got: %+v", got)
	}
}
