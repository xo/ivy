// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/x_object/form.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/pdf/internal/kurboext"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// formXObject is FormXObject in hayro-interpret src/x_object/form.rs.
type formXObject struct {
	decoded             []byte
	matrix              kurbo.Affine
	bbox                [4]float32
	isTransparencyGroup bool
	isolated            bool
	knockout            bool
	dict                syntax.Dict
	resources           *syntax.Resources
}

// newFormXObject is FormXObject::new in hayro-interpret
// src/x_object/form.rs.
func newFormXObject(stream syntax.Stream) (*formXObject, bool) {
	dict := stream.Dict()
	decoded, err := stream.DecodedData()
	if err != nil {
		return nil, false
	}
	var resources *syntax.Resources
	if d, ok := dict.Get[syntax.Dict](syntax.KeyResources); ok {
		resources = syntax.NewResources(d)
	}
	m, ok := dict.Get[[6]float64](syntax.KeyMatrix)
	if !ok {
		m = [6]float64{1, 0, 0, 1, 0, 0}
	}
	bbox, ok := dict.Get[[4]float32](syntax.KeyBBox)
	if !ok {
		return nil, false
	}
	group, hasGroup := dict.Get[syntax.Dict](syntax.KeyGroup)
	flag := func(key syntax.Name) bool {
		if !hasGroup {
			return false
		}
		v, _ := group.Get[bool](key)
		return v
	}
	return &formXObject{
		decoded:             decoded,
		matrix:              kurbo.NewAffine(m),
		isTransparencyGroup: hasGroup,
		isolated:            flag(syntax.KeyI),
		knockout:            flag(syntax.KeyK),
		bbox:                bbox,
		dict:                dict,
		resources:           resources,
	}, true
}

// draw is FormXObject::draw in hayro-interpret src/x_object/form.rs. It
// runs the content of the form in a saved state, clipped to its box, and in
// a transparency group when the form is a group.
func (f *formXObject) draw(resources *syntax.Resources, ctx *Context, device Device) {
	if !ctx.ocgState.isVisible() {
		return
	}
	// Port: the bytes of the content count as work, because each Do decodes
	// the form again (D18).
	if !ctx.run.step(int64(len(f.decoded)) / 64) {
		return
	}
	if !ctx.beginNestedInterpretation() {
		return
	}
	hasOC := xobjectOC(f.dict, ctx)
	if !ctx.ocgState.isVisible() {
		if hasOC {
			ctx.ocgState.endMarkedContent()
		}
		ctx.endNestedInterpretation()
		return
	}
	iter := syntax.NewTypedIter(f.decoded)
	ctx.path.Truncate(0)
	ctx.saveState()
	ctx.preConcatAffine(f.matrix)
	ctx.pushRootTransform()
	if f.isTransparencyGroup {
		comp := ctx.compositingMut()
		mask, blendMode := comp.softMask, comp.blendMode
		comp.softMask, comp.blendMode = nil, BlendModeNormal
		device.PushTransparencyGroup(ctx.fillPaint().alpha, mask, blendMode, f.isolated, f.knockout)
		ctx.fillPaintMut().alpha = 1
		ctx.strokePaintMut().alpha = 1
	}
	rect := kurbo.NewRect(float64(f.bbox[0]), float64(f.bbox[1]), float64(f.bbox[2]), float64(f.bbox[3]))
	device.PushClipPath(&ClipPath{
		Path: kurboext.MulBezPath(ctx.ctm(), kurboext.RectToPath(rect, 0.1)),
		Fill: FillRuleNonZero,
	})
	res := f.resources
	if res == nil {
		res = resources
	}
	Interpret(iter, res, ctx, device)
	device.PopClip()
	if f.isTransparencyGroup {
		device.PopTransparencyGroup()
	}
	ctx.popRootTransform()
	ctx.restoreState(device)
	if hasOC {
		ctx.ocgState.endMarkedContent()
	}
	ctx.endNestedInterpretation()
}
