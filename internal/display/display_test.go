package display

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/okage"
)

// rect returns the path of a rectangle.
func rect(x, y, w, h float32) *okage.Path {
	r, _ := okage.RectFromXYWH(x, y, w, h)
	return okage.PathFromRect(r)
}

// solid returns a solid color.
func solid(r, g, b, a uint8) okage.Shader {
	return okage.SolidColor(okage.ColorFromRGBA8(r, g, b, a))
}

// at returns the color of dst at (x, y), not premultiplied.
func at(dst *okage.Pixmap, x, y int) color.NRGBA {
	c, _ := color.NRGBAModel.Convert(dst.At(x, y)).(color.NRGBA)
	return c
}

func newDevice(t *testing.T, w, h uint32, tr okage.Transform) (*okage.Pixmap, *OkageDevice) {
	t.Helper()
	dst, ok := okage.NewPixmap(w, h)
	if !ok {
		t.Fatal("expected a pixmap")
	}
	return dst, NewOkage(dst, tr)
}

func TestFillAndTransform(t *testing.T) {
	dst, d := newDevice(t, 20, 20, okage.TransformFromScale(2, 2))
	d.Save()
	d.Concat(okage.TransformFromTranslate(5, 0))
	d.FillPath(rect(0, 0, 5, 5), okage.FillRuleWinding, solid(255, 0, 0, 255))
	d.Restore()
	d.FillPath(rect(0, 5, 5, 5), okage.FillRuleWinding, solid(0, 0, 255, 255))
	if c := at(dst, 15, 5); c != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("expected red at (15, 5), got: %v", c)
	}
	if c := at(dst, 5, 15); c != (color.NRGBA{0, 0, 255, 255}) {
		t.Errorf("expected blue at (5, 15), got: %v", c)
	}
	if c := at(dst, 5, 5); c.A != 0 {
		t.Errorf("expected nothing at (5, 5), got: %v", c)
	}
}

func TestClip(t *testing.T) {
	dst, d := newDevice(t, 20, 20, okage.IdentityTransform())
	d.Save()
	d.ClipPath(rect(0, 0, 10, 20), okage.FillRuleWinding)
	d.ClipPath(rect(0, 0, 20, 10), okage.FillRuleWinding)
	d.FillPath(rect(0, 0, 20, 20), okage.FillRuleWinding, solid(0, 255, 0, 255))
	d.Restore()
	d.FillPath(rect(15, 15, 5, 5), okage.FillRuleWinding, solid(0, 0, 0, 255))
	tests := []struct {
		x, y int
		a    uint8
	}{
		{5, 5, 255}, {15, 5, 0}, {5, 15, 0}, {17, 17, 255},
	}
	for _, test := range tests {
		if c := at(dst, test.x, test.y); c.A != test.a {
			t.Errorf("at (%d, %d): expected alpha %d, got: %v", test.x, test.y, test.a, c)
		}
	}
}

func TestGroup(t *testing.T) {
	dst, d := newDevice(t, 10, 10, okage.IdentityTransform())
	d.BeginGroup(0.5)
	d.FillPath(rect(0, 0, 10, 10), okage.FillRuleWinding, solid(255, 0, 0, 255))
	d.FillPath(rect(0, 0, 5, 10), okage.FillRuleWinding, solid(0, 0, 255, 255))
	d.EndGroup()
	// inside the group the blue covers the red, and the group draws at half
	// opacity, so no red shows through under the blue
	if c := at(dst, 2, 5); c.B < 250 || c.R > 5 || c.A < 120 || c.A > 135 {
		t.Errorf("expected blue at half opacity, got: %v", c)
	}
	if c := at(dst, 7, 5); c.R < 250 || c.A < 120 || c.A > 135 {
		t.Errorf("expected red at half opacity, got: %v", c)
	}
}

func TestDrawImage(t *testing.T) {
	dst, d := newDevice(t, 20, 20, okage.IdentityTransform())
	img := fixture.Image(2, 2, color.NRGBA{0, 128, 0, 255})
	r, _ := okage.RectFromXYWH(10, 10, 10, 10)
	d.DrawImage(img, r, 1)
	d.DrawImage(img, r, 1)
	if c := at(dst, 15, 15); c != (color.NRGBA{0, 128, 0, 255}) {
		t.Errorf("expected green, got: %v", c)
	}
	if c := at(dst, 5, 5); c.A != 0 {
		t.Errorf("expected nothing outside the rectangle, got: %v", c)
	}
}

func TestGlyphs(t *testing.T) {
	face, err := font.Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	gid, _ := face.Glyph('I')
	run := GlyphRun{Face: face, Size: 40, Glyphs: []Glyph{{ID: gid, X: 10, Y: 40}}, Transform: okage.IdentityTransform()}
	p := run.Path()
	if p == nil {
		t.Fatal("expected a path")
	}
	// a second path is the same: the outlines of the font do not change
	if again := run.Path(); again.Bounds() != p.Bounds() {
		t.Errorf("expected the same path twice, got: %v and %v", p.Bounds(), again.Bounds())
	}
	if b := p.Bounds(); b.Bottom() > 40.5 || b.Top() > 20 || b.Left() < 10 {
		t.Errorf("expected I above the baseline at y 40, got: %v", b)
	}
	dst, d := newDevice(t, 50, 50, okage.IdentityTransform())
	d.FillGlyphs(run, solid(0, 0, 0, 255))
	var ink int
	for y := range 50 {
		for x := range 50 {
			if at(dst, x, y).A > 128 {
				ink++
			}
		}
	}
	if ink < 50 {
		t.Errorf("expected the ink of the glyph, got %d pixels", ink)
	}
}

func TestList(t *testing.T) {
	var l List
	l.Save()
	l.Concat(okage.TransformFromTranslate(1, 2))
	l.ClipPath(rect(0, 0, 4, 4), okage.FillRuleEvenOdd)
	l.FillPath(rect(1, 1, 2, 2), okage.FillRuleWinding, solid(1, 2, 3, 255))
	l.StrokePath(rect(1, 1, 2, 2), okage.DefaultStroke(), solid(1, 2, 3, 255))
	r, _ := okage.RectFromXYWH(0, 0, 1, 1)
	l.DrawImage(image.NewGray(image.Rect(0, 0, 3, 3)), r, 0.5)
	l.BeginGroup(0.25)
	l.EndGroup()
	l.Restore()
	got := l.String()
	for _, want := range []string{"save", "clip [0 0 4 4]", "fill [1 1 3 3]", "stroke [1 1 3 3] 1", "image (3,3)", "group 0.25", "end group", "restore"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in:\n%s", want, got)
		}
	}
	dst, d := newDevice(t, 8, 8, okage.IdentityTransform())
	l.Replay(d)
	if c := at(dst, 3, 4); c.A != 255 {
		t.Errorf("expected the replayed fill, got: %v", c)
	}
}

func TestRestoreWithoutSave(t *testing.T) {
	_, d := newDevice(t, 2, 2, okage.IdentityTransform())
	d.Restore()
	d.EndGroup()
	d.ClipPath(nil, okage.FillRuleWinding)
	d.FillPath(rect(0, 0, 2, 2), okage.FillRuleWinding, solid(0, 0, 0, 255))
}
