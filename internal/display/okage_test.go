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
