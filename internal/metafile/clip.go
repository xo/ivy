package metafile

import (
	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/gowemf"
)

// clipped calls draw inside the clip. A region that is the intersection of
// paths, which is the clip of most files, becomes clip paths of the device.
// Another region, such as a union or a difference, becomes the soft mask of a
// group, which the device keeps for each region.
func (b *backend) clipped(clip gowemf.Clip, draw func()) {
	if len(clip) == 0 {
		draw()
		return
	}
	b.d.Save()
	defer b.d.Restore()
	var masks []*gowemf.ClipRegion
	for _, r := range clip {
		steps, ok := clipSteps(r, nil)
		if !ok {
			masks = append(masks, r)
			continue
		}
		for _, s := range steps {
			p := b.path(s.Area)
			if p == nil {
				// an empty area clips everything
				return
			}
			b.d.ClipPath(p, rule(s.Rule))
		}
	}
	for _, r := range masks {
		b.d.BeginGroup(display.Group{Opacity: 1, Mask: &display.Mask{
			Type:    okage.MaskTypeAlpha,
			Content: func(d display.Device) { b.region(d, r) },
			Key:     r,
		}})
	}
	draw()
	for range masks {
		b.d.EndGroup()
	}
}

// clipSteps appends to steps the regions whose intersection is r, and
// returns false if r is not such an intersection. A nil region is the whole
// surface, which needs no step.
func clipSteps(r *gowemf.ClipRegion, steps []*gowemf.ClipRegion) ([]*gowemf.ClipRegion, bool) {
	if r == nil {
		return steps, true
	}
	switch r.Op {
	case gowemf.ClipReplace:
		if r.Operand != nil {
			return clipSteps(r.Operand, steps)
		}
		return append(steps, r), true
	case gowemf.ClipIntersect:
		steps, ok := clipSteps(r.Base, steps)
		if !ok {
			return nil, false
		}
		if r.Operand != nil {
			return clipSteps(r.Operand, steps)
		}
		return append(steps, r), true
	}
	return nil, false
}

// region draws the region r onto d in opaque black, for a soft mask. Each
// operator of a region becomes the blend mode of a group, which combines
// the operand with the base that d holds.
func (b *backend) region(d display.Device, r *gowemf.ClipRegion) {
	if r == nil {
		d.FillPath(okage.PathFromRect(b.surface), okage.FillRuleWinding, okage.SolidColor(okage.ColorBlack))
		return
	}
	var blend okage.BlendMode
	switch r.Op {
	case gowemf.ClipReplace:
		b.operand(d, r)
		return
	case gowemf.ClipOffset:
		if !b.near(b.cx+r.Offset.X, b.cy+r.Offset.Y) {
			return
		}
		d.Save()
		d.Concat(okage.TransformFromTranslate(float32(r.Offset.X), float32(r.Offset.Y)))
		b.region(d, r.Base)
		d.Restore()
		return
	case gowemf.ClipIntersect:
		blend = okage.BlendModeDestinationIn
	case gowemf.ClipUnion:
		blend = okage.BlendModeSourceOver
	case gowemf.ClipXor:
		blend = okage.BlendModeXor
	case gowemf.ClipDifference:
		blend = okage.BlendModeDestinationOut
	case gowemf.ClipComplement:
		blend = okage.BlendModeSourceOut
	default:
		return
	}
	b.region(d, r.Base)
	d.BeginGroup(display.Group{Opacity: 1, Blend: blend, Isolated: true})
	b.operand(d, r)
	d.EndGroup()
}

// operand draws the operand of r: a region, or an area.
func (b *backend) operand(d display.Device, r *gowemf.ClipRegion) {
	if r.Operand != nil {
		b.region(d, r.Operand)
		return
	}
	if p := b.path(r.Area); p != nil {
		d.FillPath(p, rule(r.Rule), okage.SolidColor(okage.ColorBlack))
	}
}
