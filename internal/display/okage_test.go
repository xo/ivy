package display

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/xo/okage"
)

// groupMasks returns the soft masks of TestGroupDrawn: none, a mask of
// luminosity with a key, a mask of alpha, and a mask of luminosity over a
// gray backdrop with a transfer function.
func groupMasks() []*Mask {
	var invert [256]uint8
	for i := range invert {
		invert[i] = uint8(255 - i)
	}
	return []*Mask{
		nil,
		{
			Type: okage.MaskTypeLuminance,
			Content: func(d Device) {
				d.FillPath(rect(0, 0, 24, 32), okage.FillRuleWinding, solid(255, 255, 255, 255))
				d.FillPath(rect(24, 0, 24, 32), okage.FillRuleWinding, solid(100, 100, 100, 255))
			},
			Key: "luminosity",
		},
		{
			Type: okage.MaskTypeAlpha,
			Content: func(d Device) {
				d.FillPath(rect(8, 6, 30, 20), okage.FillRuleWinding, solid(0, 0, 0, 150))
			},
		},
		{
			Type:     okage.MaskTypeLuminance,
			Backdrop: okage.ColorFromRGBA8(128, 128, 128, 255),
			Transfer: &invert,
			Content: func(d Device) {
				d.FillPath(rect(0, 0, 20, 32), okage.FillRuleWinding, solid(0, 0, 0, 255))
			},
		},
	}
}

// groupImage returns an image of 4 by 4 pixels with each alpha.
func groupImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			a := uint8(60 * (x + y) / 2)
			img.SetRGBA(x, y, color.RGBA{a, 0, a / 2, a})
		}
	}
	return img
}

// groupScene draws a scene with the group g onto d. The scene has a
// backdrop that is partly transparent, the group inside a clip when clip is
// true, and in the group objects that cover small parts of the clip, nested
// groups, a clip, an image and an object outside the clip. A second group
// then uses the layers of the first one again.
func groupScene(d Device, g Group, inner okage.BlendMode, clip bool) {
	d.FillPath(rect(0, 0, 30, 32), okage.FillRuleWinding, solid(255, 220, 0, 255))
	d.FillPath(rect(0, 10, 48, 8), okage.FillRuleWinding, solid(0, 60, 255, 100))
	d.Save()
	if clip {
		d.ClipPath(rect(2.5, 2, 41, 27.5), okage.FillRuleWinding)
	}
	d.BeginGroup(g)
	d.FillPath(rect(6, 4, 8, 6), okage.FillRuleWinding, solid(255, 0, 0, 200))
	d.BeginGroup(Group{Opacity: 1, Blend: inner})
	d.FillPath(rect(10, 8, 10, 10), okage.FillRuleWinding, solid(0, 255, 255, 180))
	d.EndGroup()
	d.BeginGroup(Group{Opacity: 0.7, Blend: okage.BlendModeLuminosity, Isolated: !g.Isolated, Knockout: !g.Knockout})
	pb := okage.NewPathBuilder()
	pb.MoveTo(22, 20)
	pb.LineTo(34, 26)
	pb.LineTo(40, 18)
	if p, ok := pb.Finish(); ok {
		stroke := okage.DefaultStroke()
		stroke.Width = 2.5
		d.StrokePath(p, stroke, solid(0, 160, 40, 220))
	}
	d.DrawImage(groupImage(), okage.TransformFromRow(2, 0, 0, 2, 31, 3), ImageOptions{Opacity: 0.9, Smooth: true})
	d.BeginGroup(Group{Opacity: 1, Blend: okage.BlendModeDifference})
	d.FillPath(rect(33, 6, 4, 4), okage.FillRuleWinding, solid(255, 255, 255, 255))
	d.EndGroup()
	d.EndGroup()
	d.Save()
	d.ClipPath(rect(1, 25, 4, 4), okage.FillRuleWinding)
	d.FillPath(rect(0, 0, 48, 32), okage.FillRuleWinding, solid(120, 0, 200, 160))
	d.Restore()
	d.FillPath(rect(60, 40, 4, 4), okage.FillRuleWinding, solid(0, 0, 0, 255))
	// a group that draws nothing
	d.BeginGroup(Group{Opacity: 1, Blend: inner, Isolated: true})
	d.EndGroup()
	d.EndGroup()
	d.Restore()
	d.BeginGroup(g)
	d.FillPath(rect(26, 12, 7, 7), okage.FillRuleWinding, solid(200, 100, 0, 230))
	d.EndGroup()
}

func TestGroupDrawn(t *testing.T) {
	// a group composites only on the pixels that its objects drew, and the
	// pixels must stay the same as when it composites on the bounds of its
	// clip
	var failed int
	masks := groupMasks()
	for blend := okage.BlendModeClear; blend <= okage.BlendModeLuminosity; blend++ {
		for _, opacity := range []float32{1, 0.5} {
			for mi, mask := range masks {
				for _, flags := range []struct{ isolated, knockout bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
					g := Group{Opacity: opacity, Blend: blend, Isolated: flags.isolated, Knockout: flags.knockout, Mask: mask}
					inner := okage.BlendModeMultiply + okage.BlendMode(mi)
					clip := mi%2 == 0
					name := fmt.Sprintf("%v opacity %v mask %d isolated %t knockout %t", blend, opacity, mi, flags.isolated, flags.knockout)
					want, d := newDevice(t, 48, 32, okage.IdentityTransform())
					d.whole = true
					groupScene(d, g, inner, clip)
					got, d := newDevice(t, 48, 32, okage.IdentityTransform())
					groupScene(d, g, inner, clip)
					if !bytes.Equal(got.Data(), want.Data()) {
						t.Errorf("%s: expected the same pixels as the composite on the bounds of the clip", name)
						failed++
					}
					for i, layer := range d.free {
						if slices.ContainsFunc(layer.Data(), func(b byte) bool { return b != 0 }) {
							t.Errorf("%s: expected the free layer %d transparent", name, i)
							failed++
						}
					}
					if failed > 10 {
						t.Fatal("too many failures")
					}
				}
			}
		}
	}
}

// maskScene draws onto d groups with soft masks, the blend mode and the
// opacity, each on a small part of a clip. The masks hold gradients, an
// image and a group with a mask of its own. Three groups then use a mask
// with a key on other pixels. maskScene returns the number of times that
// the content of the mask with the key drew.
func maskScene(t *testing.T, d Device, blend okage.BlendMode, opacity float32) int {
	t.Helper()
	stops := []okage.GradientStop{
		okage.NewGradientStop(0, okage.ColorFromRGBA8(255, 255, 255, 255)),
		okage.NewGradientStop(0.6, okage.ColorFromRGBA8(90, 30, 200, 160)),
		okage.NewGradientStop(1, okage.ColorFromRGBA8(0, 0, 0, 40)),
	}
	linear, ok := okage.NewLinearGradient(okage.PointFromXY(3.3, 1.7), okage.PointFromXY(43.1, 29.9), stops, okage.SpreadModeReflect, okage.IdentityTransform())
	if !ok {
		t.Fatal("expected a gradient")
	}
	radial, ok := okage.NewRadialGradient(okage.PointFromXY(19.2, 11.4), 2.1, okage.PointFromXY(21.7, 13.1), 17.3, stops, okage.SpreadModePad, okage.IdentityTransform())
	if !ok {
		t.Fatal("expected a gradient")
	}
	inner := &Mask{
		Type: okage.MaskTypeLuminance,
		Content: func(d Device) {
			d.FillPath(rect(0, 0, 48, 32), okage.FillRuleWinding, radial)
		},
	}
	masks := []*Mask{
		{
			Type: okage.MaskTypeLuminance,
			Content: func(d Device) {
				d.FillPath(rect(0, 0, 48, 32), okage.FillRuleWinding, linear)
			},
		},
		{
			Type: okage.MaskTypeAlpha,
			Content: func(d Device) {
				d.FillPath(rect(1.5, 0.5, 44, 30), okage.FillRuleEvenOdd, radial)
				d.DrawImage(groupImage(), okage.TransformFromRow(9.3, 0.4, -0.7, 7.1, 2.2, 1.9), ImageOptions{Opacity: 0.8, Smooth: true})
			},
		},
		{
			Type:     okage.MaskTypeLuminance,
			Backdrop: okage.ColorFromRGBA8(60, 60, 60, 255),
			Content: func(d Device) {
				d.FillPath(rect(0, 0, 48, 32), okage.FillRuleWinding, linear)
				d.BeginGroup(Group{Opacity: 0.8, Blend: okage.BlendModeMultiply, Mask: inner})
				d.FillPath(rect(4.4, 3.6, 36.2, 22.5), okage.FillRuleWinding, solid(250, 240, 10, 255))
				d.EndGroup()
			},
		},
	}
	var keyed int
	shared := &Mask{
		Type: okage.MaskTypeLuminance,
		Content: func(d Device) {
			keyed++
			d.FillPath(rect(0, 0, 48, 32), okage.FillRuleWinding, radial)
		},
		Key: "shared",
	}
	d.FillPath(rect(0, 0, 48, 32), okage.FillRuleWinding, solid(255, 220, 0, 255))
	d.FillPath(rect(0, 9.5, 48, 8), okage.FillRuleWinding, solid(0, 60, 255, 100))
	d.Save()
	d.ClipPath(rect(1.3, 0.7, 44.2, 29.1), okage.FillRuleWinding)
	for i, m := range masks {
		x := 11 * float32(i)
		d.BeginGroup(Group{Opacity: opacity, Blend: blend, Mask: m})
		d.FillPath(rect(3.7+x, 4.2, 7.9, 6.6), okage.FillRuleWinding, solid(255, 0, 0, 200))
		d.FillPath(rect(5.1+x, 15.3, 4.4, 9.2), okage.FillRuleWinding, linear)
		d.EndGroup()
	}
	// the first group draws the shared mask on its pixels, the second on
	// the bounds of the clip, and the third takes it from the cache
	for _, x := range []float32{2.6, 30.4, 17.2} {
		d.BeginGroup(Group{Opacity: opacity, Blend: blend, Mask: shared})
		d.FillPath(rect(x, 20.1, 9.3, 6.2), okage.FillRuleWinding, solid(0, 160, 40, 220))
		d.EndGroup()
	}
	d.Restore()
	return keyed
}

func TestGroupMaskArea(t *testing.T) {
	// a group draws its soft mask only on the pixels that it composites,
	// and the pixels must stay the same as when it draws the mask on the
	// bounds of its clip, under a transform that does not move the pixels
	// by whole numbers
	tr := okage.TransformFromRow(1.37, 0.05, -0.08, 1.21, 0.29, 0.61)
	for _, blend := range []okage.BlendMode{okage.BlendModeSourceOver, okage.BlendModeMultiply, okage.BlendModeDestinationIn} {
		for _, opacity := range []float32{1, 0.6} {
			name := fmt.Sprintf("%v opacity %v", blend, opacity)
			want, d := newDevice(t, 64, 44, tr)
			d.whole = true
			if n := maskScene(t, d, blend, opacity); n != 1 {
				t.Errorf("%s: expected the shared mask drawn once on the bounds of the clip, got %d", name, n)
			}
			got, d := newDevice(t, 64, 44, tr)
			n := maskScene(t, d, blend, opacity)
			if !bytes.Equal(got.Data(), want.Data()) {
				t.Errorf("%s: expected the same pixels as the mask on the bounds of the clip", name)
			}
			// a blend mode that clears the parent composites on the
			// bounds of the clip, and draws the mask there once
			draws := 2
			if !keepsParent(blend) {
				draws = 1
			}
			if n != draws {
				t.Errorf("%s: expected the shared mask drawn %d times, got %d", name, draws, n)
			}
		}
	}
}
