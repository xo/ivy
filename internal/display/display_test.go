package display

import (
	"image"
	"image/color"
	"slices"
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
	d.BeginGroup(Group{Opacity: 0.5})
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
	d.DrawImage(img, RectTransform(img.Bounds().Size(), r), ImageOptions{Opacity: 1})
	d.DrawImage(img, RectTransform(img.Bounds().Size(), r), ImageOptions{Opacity: 1})
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
	l.DrawImage(image.NewGray(image.Rect(0, 0, 3, 3)), RectTransform(image.Pt(3, 3), r), ImageOptions{Opacity: 0.5})
	l.FillMask(image.NewAlpha(image.Rect(0, 0, 2, 2)), okage.IdentityTransform(), ImageOptions{Opacity: 1}, solid(0, 0, 0, 255))
	l.BeginGroup(Group{Opacity: 0.25, Blend: okage.BlendModeMultiply, Isolated: true, Mask: &Mask{Type: okage.MaskTypeLuminance}})
	l.EndGroup()
	l.Restore()
	got := l.String()
	for _, want := range []string{"save", "clip [0 0 4 4]", "fill [1 1 3 3]", "stroke [1 1 3 3] 1", "image (3,3)", "mask (2,2)", "group 0.25 Multiply isolated mask Luminance", "end group", "restore"} {
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

func TestDrawImageTransform(t *testing.T) {
	dst, d := newDevice(t, 20, 20, okage.IdentityTransform())
	img := fixture.Image(2, 1, color.NRGBA{0, 0, 255, 255})
	// the image of 2 by 1 pixels covers 10 by 20 pixels, turned by 90
	// degrees
	tr := okage.TransformFromRow(0, 10, 10, 0, 0, 0)
	d.DrawImage(img, tr, ImageOptions{Opacity: 1})
	if c := at(dst, 5, 15); c != (color.NRGBA{0, 0, 255, 255}) {
		t.Errorf("expected blue at (5, 15), got: %v", c)
	}
	if c := at(dst, 15, 5); c.A != 0 {
		t.Errorf("expected nothing at (15, 5), got: %v", c)
	}
}

func TestFillMask(t *testing.T) {
	mask := image.NewAlpha(image.Rect(0, 0, 2, 2))
	mask.SetAlpha(0, 0, color.Alpha{255})
	mask.SetAlpha(1, 1, color.Alpha{255})
	tr := okage.TransformFromScale(5, 5)
	stops := []okage.GradientStop{
		okage.NewGradientStop(0, okage.ColorFromRGBA8(255, 0, 0, 255)),
		okage.NewGradientStop(1, okage.ColorFromRGBA8(255, 0, 0, 255)),
	}
	gradient, ok := okage.NewLinearGradient(okage.PointFromXY(0, 0), okage.PointFromXY(10, 0), stops, okage.SpreadModePad, okage.IdentityTransform())
	if !ok {
		t.Fatal("expected a gradient")
	}
	for _, shader := range []okage.Shader{solid(255, 0, 0, 255), gradient} {
		dst, d := newDevice(t, 10, 10, okage.IdentityTransform())
		d.FillMask(mask, tr, ImageOptions{Opacity: 1}, shader)
		if c := at(dst, 2, 2); c != (color.NRGBA{255, 0, 0, 255}) {
			t.Errorf("%T: expected red at (2, 2), got: %v", shader, c)
		}
		if c := at(dst, 7, 2); c.A != 0 {
			t.Errorf("%T: expected nothing at (7, 2), got: %v", shader, c)
		}
	}
}

func TestGroupBlend(t *testing.T) {
	dst, d := newDevice(t, 10, 10, okage.IdentityTransform())
	d.FillPath(rect(0, 0, 10, 10), okage.FillRuleWinding, solid(255, 255, 0, 255))
	d.BeginGroup(Group{Opacity: 1, Blend: okage.BlendModeMultiply})
	d.FillPath(rect(0, 0, 10, 10), okage.FillRuleWinding, solid(0, 255, 255, 255))
	d.EndGroup()
	if c := at(dst, 5, 5); c != (color.NRGBA{0, 255, 0, 255}) {
		t.Errorf("expected green from the multiply of yellow and cyan, got: %v", c)
	}
}

func TestSoftMask(t *testing.T) {
	var drawn int
	mask := &Mask{
		Type: okage.MaskTypeLuminance,
		Content: func(d Device) {
			drawn++
			d.FillPath(rect(0, 0, 5, 10), okage.FillRuleWinding, solid(255, 255, 255, 255))
		},
		Key: 1,
	}
	dst, d := newDevice(t, 10, 10, okage.IdentityTransform())
	for range 2 {
		d.BeginGroup(Group{Opacity: 1, Mask: mask})
		d.FillPath(rect(0, 0, 10, 10), okage.FillRuleWinding, solid(255, 0, 0, 255))
		d.EndGroup()
	}
	if drawn != 1 {
		t.Errorf("expected the mask drawn once for its key, got %d", drawn)
	}
	if c := at(dst, 2, 5); c != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("expected red under the white of the mask, got: %v", c)
	}
	if c := at(dst, 7, 5); c.A != 0 {
		t.Errorf("expected nothing under the black of the mask, got: %v", c)
	}
	// a white backdrop and a transfer function that inverts the mask
	var invert [256]uint8
	for i := range invert {
		invert[i] = uint8(255 - i)
	}
	inverted := &Mask{
		Type:     okage.MaskTypeLuminance,
		Backdrop: okage.ColorWhite,
		Transfer: &invert,
		Content: func(d Device) {
			d.FillPath(rect(0, 0, 5, 10), okage.FillRuleWinding, solid(0, 0, 0, 255))
		},
	}
	dst, d = newDevice(t, 10, 10, okage.IdentityTransform())
	d.BeginGroup(Group{Opacity: 1, Mask: inverted})
	d.FillPath(rect(0, 0, 10, 10), okage.FillRuleWinding, solid(0, 0, 255, 255))
	d.EndGroup()
	if c := at(dst, 2, 5); c != (color.NRGBA{0, 0, 255, 255}) {
		t.Errorf("expected blue where the inverted mask is white, got: %v", c)
	}
	if c := at(dst, 7, 5); c.A != 0 {
		t.Errorf("expected nothing where the inverted mask is black, got: %v", c)
	}
}

func TestGroupBounds(t *testing.T) {
	white := &Mask{
		Type: okage.MaskTypeLuminance,
		Content: func(d Device) {
			d.FillPath(rect(0, 0, 40, 10), okage.FillRuleWinding, solid(255, 255, 255, 255))
		},
	}
	dst, d := newDevice(t, 40, 10, okage.IdentityTransform())
	// the first group draws red over the whole target inside a clip of the
	// left part. The second group reuses the layer of the first, and draws
	// blue inside a clip of the right part. A Restore too many inside a
	// group does not drop the clip of the group.
	for _, g := range []struct {
		x     float32
		color okage.Shader
	}{{0, solid(255, 0, 0, 255)}, {30, solid(0, 0, 255, 255)}} {
		d.Save()
		d.ClipPath(rect(g.x, 0, 10, 10), okage.FillRuleWinding)
		d.BeginGroup(Group{Opacity: 1, Mask: white})
		d.Restore()
		d.Restore()
		d.FillPath(rect(0, 0, 40, 10), okage.FillRuleWinding, g.color)
		d.EndGroup()
		d.Restore()
	}
	for _, tt := range []struct {
		x    int
		want color.NRGBA
	}{
		{5, color.NRGBA{255, 0, 0, 255}},
		{20, color.NRGBA{}},
		{35, color.NRGBA{0, 0, 255, 255}},
	} {
		if c := at(dst, tt.x, 5); c != tt.want {
			t.Errorf("expected %v at x %d, got: %v", tt.want, tt.x, c)
		}
	}
}

func TestFillTextCoverage(t *testing.T) {
	// the left edge of a rectangle at 16 positions inside the pixel 1: the
	// finer coverage of the pixel is near its area, in steps finer than a
	// quarter
	levels := make(map[uint8]bool)
	for f := range 16 {
		dst, d := newDevice(t, 12, 6, okage.IdentityTransform())
		x := 1 + float32(f)/16
		d.FillText(rect(x, 1, 9-x, 4), okage.FillRuleWinding, solid(0, 0, 0, 255))
		got := at(dst, 1, 2).A
		want := (1 - float64(f)/16) * 255
		if diff := float64(got) - want; diff < -20 || diff > 20 {
			t.Errorf("edge at %g: expected a coverage near %.0f, got %d", x, want, got)
		}
		levels[got] = true
		if c := at(dst, 5, 2); c != (color.NRGBA{0, 0, 0, 255}) {
			t.Errorf("edge at %g: expected black inside, got: %v", x, c)
		}
	}
	if len(levels) < 8 {
		t.Errorf("expected at least 8 levels of coverage, got %d", len(levels))
	}
}

func TestFillTextClip(t *testing.T) {
	dst, d := newDevice(t, 20, 20, okage.TransformFromScale(2, 2))
	d.ClipPath(rect(0, 0, 5, 10), okage.FillRuleWinding)
	FillText(d, rect(1, 1, 8, 8), okage.FillRuleWinding, solid(0, 0, 255, 255))
	if c := at(dst, 4, 4); c != (color.NRGBA{0, 0, 255, 255}) {
		t.Errorf("expected blue inside the clip, got: %v", c)
	}
	if c := at(dst, 14, 4); c.A != 0 {
		t.Errorf("expected nothing outside the clip, got: %v", c)
	}
	// a shader that is not a solid color fills as FillPath does
	shader, ok := okage.NewLinearGradient(okage.Point{X: 0, Y: 0}, okage.Point{X: 10, Y: 0},
		[]okage.GradientStop{okage.NewGradientStop(0, okage.ColorBlack), okage.NewGradientStop(1, okage.ColorWhite)},
		okage.SpreadModePad, okage.IdentityTransform())
	if !ok {
		t.Fatal("expected a gradient")
	}
	FillText(d, rect(1, 1, 8, 8), okage.FillRuleWinding, shader)
	if c := at(dst, 14, 4); c.A != 0 {
		t.Errorf("expected nothing outside the clip, got: %v", c)
	}
	var l List
	FillText(&l, rect(1, 1, 2, 2), okage.FillRuleWinding, solid(0, 0, 0, 255))
	if got := l.String(); got != "fill text [1 1 3 3] Winding" {
		t.Errorf("expected the fill of text in the list, got: %q", got)
	}
}

func TestFillGlyphsLarge(t *testing.T) {
	face, err := font.Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	gid, _ := face.Glyph('I')
	// a glyph larger than maxFineEm fills as FillPath does, with the same
	// pixels
	run := GlyphRun{Face: face, Size: 2 * maxFineEm, Glyphs: []Glyph{{ID: gid, X: 10, Y: 100}}, Transform: okage.IdentityTransform()}
	a, da := newDevice(t, 120, 120, okage.IdentityTransform())
	da.FillGlyphs(run, solid(0, 0, 0, 255))
	b, db := newDevice(t, 120, 120, okage.IdentityTransform())
	db.FillPath(run.Path(), okage.FillRuleWinding, solid(0, 0, 0, 255))
	if !slices.Equal(a.Data(), b.Data()) {
		t.Error("expected the pixels of FillPath for a large glyph")
	}
}

// near reports whether each channel of c is within 1 of want.
func near(c, want color.NRGBA) bool {
	d := func(a, b uint8) bool { return int(a) <= int(b)+1 && int(b) <= int(a)+1 }
	return d(c.R, want.R) && d(c.G, want.G) && d(c.B, want.B) && d(c.A, want.A)
}

// multiply draws a rectangle in a group with the blend mode multiply.
func multiply(d Device, r *okage.Path, shader okage.Shader) {
	d.BeginGroup(Group{Opacity: 1, Blend: okage.BlendModeMultiply})
	d.FillPath(r, okage.FillRuleWinding, shader)
	d.EndGroup()
}

func TestGroupNotIsolated(t *testing.T) {
	yellow, cyan := solid(255, 255, 0, 255), solid(0, 255, 255, 255)
	white := &Mask{
		Type: okage.MaskTypeLuminance,
		Content: func(d Device) {
			d.FillPath(rect(0, 0, 10, 10), okage.FillRuleWinding, solid(255, 255, 255, 255))
		},
	}
	for _, tt := range []struct {
		name string
		draw func(d *OkageDevice)
		want color.NRGBA
	}{
		{
			// the cyan multiplies with the yellow of the backdrop into
			// green, and the group draws the green at half opacity over the
			// yellow: (0.5*0 + 0.5*255, 255, 0)
			"not isolated",
			func(d *OkageDevice) {
				d.BeginGroup(Group{Opacity: 0.5})
				multiply(d, rect(0, 0, 10, 10), cyan)
				d.EndGroup()
			},
			color.NRGBA{128, 255, 0, 255},
		},
		{
			// the cyan multiplies with the transparent layer, and stays
			// cyan: (0.5*0 + 0.5*255, 255, 0.5*255 + 0.5*0)
			"isolated",
			func(d *OkageDevice) {
				d.BeginGroup(Group{Opacity: 0.5, Isolated: true})
				multiply(d, rect(0, 0, 10, 10), cyan)
				d.EndGroup()
			},
			color.NRGBA{128, 255, 128, 255},
		},
		{
			// a nested group that is not isolated takes the backdrop of
			// its parent, which takes the backdrop of the target
			"nested",
			func(d *OkageDevice) {
				d.BeginGroup(Group{Opacity: 0.5})
				d.BeginGroup(Group{Opacity: 1, Mask: white})
				multiply(d, rect(0, 0, 10, 10), cyan)
				d.EndGroup()
				d.EndGroup()
			},
			color.NRGBA{128, 255, 0, 255},
		},
		{
			// the red before the multiply is part of the group alone: the
			// cyan multiplies with the red over the yellow into black, and
			// the group draws the black at half opacity over the yellow
			"object before the blend",
			func(d *OkageDevice) {
				d.BeginGroup(Group{Opacity: 0.5})
				d.FillPath(rect(0, 0, 10, 10), okage.FillRuleWinding, solid(255, 0, 0, 255))
				multiply(d, rect(0, 0, 10, 10), cyan)
				d.EndGroup()
			},
			color.NRGBA{128, 128, 0, 255},
		},
	} {
		dst, d := newDevice(t, 10, 10, okage.IdentityTransform())
		d.FillPath(rect(0, 0, 10, 10), okage.FillRuleWinding, yellow)
		tt.draw(d)
		if c := at(dst, 5, 5); !near(c, tt.want) {
			t.Errorf("%s: expected %v, got: %v", tt.name, tt.want, c)
		}
		if len(d.groups) != 0 || d.dst != dst || d.alpha != nil {
			t.Errorf("%s: expected the device back on its target", tt.name)
		}
	}
}

func TestGroupNotIsolatedTransparent(t *testing.T) {
	// over a transparent backdrop, a group that is not isolated keeps the
	// colors and the alpha of its objects
	dst, d := newDevice(t, 10, 10, okage.IdentityTransform())
	d.FillPath(rect(0, 0, 5, 10), okage.FillRuleWinding, solid(255, 255, 0, 255))
	d.BeginGroup(Group{Opacity: 1, Mask: &Mask{
		Type: okage.MaskTypeAlpha,
		Content: func(d Device) {
			d.FillPath(rect(0, 0, 10, 10), okage.FillRuleWinding, solid(0, 0, 0, 255))
		},
	}})
	multiply(d, rect(0, 0, 10, 10), solid(0, 255, 255, 128))
	d.EndGroup()
	// over the yellow, half of the cyan multiplies into green: (127, 255,
	// 0). Over nothing, the cyan stays at half alpha.
	if c := at(dst, 2, 5); !near(c, color.NRGBA{127, 255, 0, 255}) {
		t.Errorf("expected yellow and green, got: %v", c)
	}
	if c := at(dst, 7, 5); !near(c, color.NRGBA{0, 255, 255, 128}) {
		t.Errorf("expected cyan at half alpha, got: %v", c)
	}
}

func TestGroupKnockout(t *testing.T) {
	// in an isolated knockout group on white, the blue at half alpha
	// replaces the red at half alpha where they overlap. The red over white
	// is (255, 127.5, 127.5), and the blue over white is (127.5, 127.5,
	// 255).
	dst, d := newDevice(t, 30, 10, okage.IdentityTransform())
	d.FillPath(rect(0, 0, 30, 10), okage.FillRuleWinding, solid(255, 255, 255, 255))
	d.BeginGroup(Group{Opacity: 1, Isolated: true, Knockout: true})
	d.FillPath(rect(0, 0, 20, 10), okage.FillRuleWinding, solid(255, 0, 0, 128))
	d.FillPath(rect(10, 0, 20, 10), okage.FillRuleWinding, solid(0, 0, 255, 128))
	d.EndGroup()
	for _, tt := range []struct {
		x    int
		want color.NRGBA
	}{
		{5, color.NRGBA{255, 127, 127, 255}},
		{15, color.NRGBA{127, 127, 255, 255}},
		{25, color.NRGBA{127, 127, 255, 255}},
	} {
		if c := at(dst, tt.x, 5); !near(c, tt.want) {
			t.Errorf("expected %v at x %d, got: %v", tt.want, tt.x, c)
		}
	}
	// without knockout, the blue covers the red. The layer is (63.75,
	// 0, 127.5) premultiplied at an alpha of 0.75, and over white it is
	// (127.5, 63.75, 191.25).
	dst, d = newDevice(t, 30, 10, okage.IdentityTransform())
	d.FillPath(rect(0, 0, 30, 10), okage.FillRuleWinding, solid(255, 255, 255, 255))
	d.BeginGroup(Group{Opacity: 1, Isolated: true})
	d.FillPath(rect(0, 0, 20, 10), okage.FillRuleWinding, solid(255, 0, 0, 128))
	d.FillPath(rect(10, 0, 20, 10), okage.FillRuleWinding, solid(0, 0, 255, 128))
	d.EndGroup()
	if c := at(dst, 15, 5); !near(c, color.NRGBA{127, 64, 191, 255}) {
		t.Errorf("expected the blue over the red, got: %v", c)
	}
}

func TestGroupKnockoutNotIsolated(t *testing.T) {
	// in a knockout group that is not isolated, the magenta multiplies with
	// the yellow of the backdrop into red, not with the green that the
	// cyan made
	dst, d := newDevice(t, 20, 10, okage.IdentityTransform())
	d.FillPath(rect(0, 0, 20, 10), okage.FillRuleWinding, solid(255, 255, 0, 255))
	d.BeginGroup(Group{Opacity: 1, Knockout: true})
	multiply(d, rect(0, 0, 20, 10), solid(0, 255, 255, 255))
	multiply(d, rect(0, 0, 10, 10), solid(255, 0, 255, 255))
	d.EndGroup()
	if c := at(dst, 5, 5); !near(c, color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("expected red, got: %v", c)
	}
	if c := at(dst, 15, 5); !near(c, color.NRGBA{0, 255, 0, 255}) {
		t.Errorf("expected green, got: %v", c)
	}
	// without knockout, the magenta multiplies with the green into black
	dst, d = newDevice(t, 20, 10, okage.IdentityTransform())
	d.FillPath(rect(0, 0, 20, 10), okage.FillRuleWinding, solid(255, 255, 0, 255))
	d.BeginGroup(Group{Opacity: 1})
	multiply(d, rect(0, 0, 20, 10), solid(0, 255, 255, 255))
	multiply(d, rect(0, 0, 10, 10), solid(255, 0, 255, 255))
	d.EndGroup()
	if c := at(dst, 5, 5); !near(c, color.NRGBA{0, 0, 0, 255}) {
		t.Errorf("expected black, got: %v", c)
	}
}

func TestSoftMaskBackdrop(t *testing.T) {
	// a mask of luminosity over a gray backdrop. Where the content is
	// transparent, the mask is the luminance of the gray, about 0.5.
	gray := okage.ColorFromRGBA8(128, 128, 128, 255)
	dst, d := newDevice(t, 10, 10, okage.IdentityTransform())
	d.BeginGroup(Group{Opacity: 1, Mask: &Mask{
		Type:     okage.MaskTypeLuminance,
		Backdrop: gray,
		Content: func(d Device) {
			d.FillPath(rect(0, 0, 5, 10), okage.FillRuleWinding, solid(255, 255, 255, 255))
		},
	}})
	d.FillPath(rect(0, 0, 10, 10), okage.FillRuleWinding, solid(0, 0, 255, 255))
	d.EndGroup()
	if c := at(dst, 2, 5); c != (color.NRGBA{0, 0, 255, 255}) {
		t.Errorf("expected blue under the white, got: %v", c)
	}
	if c := at(dst, 7, 5); !near(c, color.NRGBA{0, 0, 255, 128}) {
		t.Errorf("expected blue at the luminance of the gray, got: %v", c)
	}
}

func TestStrokeAdjusted(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(1, 3)
	pb.LineTo(9, 3)
	p, ok := pb.Finish()
	if !ok {
		t.Fatal("expected a path")
	}
	stroke := okage.DefaultStroke()
	// a line of 1 pixel on the border of the rows 2 and 3 covers half of
	// each. The stroke adjustment moves it into one row, which it covers
	// whole.
	dst, d := newDevice(t, 10, 6, okage.IdentityTransform())
	d.StrokePath(p, stroke, solid(0, 0, 0, 255))
	if a2, a3 := at(dst, 5, 2).A, at(dst, 5, 3).A; a2 < 120 || a2 > 136 || a3 < 120 || a3 > 136 {
		t.Errorf("expected two rows at half alpha, got: %d and %d", a2, a3)
	}
	dst, d = newDevice(t, 10, 6, okage.IdentityTransform())
	StrokeAdjusted(d, p, stroke, solid(0, 0, 0, 255))
	a2, a3 := at(dst, 5, 2).A, at(dst, 5, 3).A
	if max(a2, a3) != 255 || min(a2, a3) != 0 {
		t.Errorf("expected one opaque row, got: %d and %d", a2, a3)
	}
	var l List
	StrokeAdjusted(&l, p, stroke, solid(0, 0, 0, 255))
	if got := l.String(); got != "stroke adjusted [1 3 9 3] 1" {
		t.Errorf("expected the adjusted stroke in the list, got: %q", got)
	}
}
