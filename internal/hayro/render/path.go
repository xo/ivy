// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro src/path.rs.

package render

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/ivy/internal/hayro/kurboext"
)

// minDashPeriod is MIN_DASH_PERIOD in hayro src/path.rs.
const minDashPeriod = 0.1

// strokeProperties is Renderer::set_stroke_properties in hayro
// src/path.rs. It returns the stroke of okage for the properties, drawn
// with the transform of the drawing.
//
// Port: upstream sets a stroke of kurbo on the context. A dash array of
// okage has an even length, so the port repeats an array of odd length, as
// PDF reads it. A dash array that okage refuses draws no dashes.
func (r *renderer) strokeProperties(props *interpret.StrokeProps, transform kurbo.Affine, isText bool) okage.Stroke {
	threshold := float32(1)
	if isText {
		threshold = 0.25
	}
	// Text gets a lower minimum width, since 1.0 makes it look bold at low
	// resolutions.
	minFactor := maxFactor(transform)
	lineWidth := rustMaxF32(props.LineWidth, 0.01/minFactor)
	transformedWidth := lineWidth * minFactor
	if transformedWidth < threshold && !r.insidePattern && !r.inType3Glyph {
		lineWidth /= transformedWidth
		lineWidth *= threshold
	}
	// At most one pixel wide, round caps and joins look like square and
	// bevel ones and are cheaper.
	isThin := transformedWidth <= 1
	stroke := okage.Stroke{
		Width:      lineWidth,
		MiterLimit: props.MiterLimit,
		LineCap:    okage.LineCapButt,
		LineJoin:   okage.LineJoinMiter,
	}
	switch props.LineCap {
	case kurboext.CapRound:
		stroke.LineCap = okage.LineCapRound
		if isThin {
			stroke.LineCap = okage.LineCapSquare
		}
	case kurboext.CapSquare:
		stroke.LineCap = okage.LineCapSquare
	}
	switch props.LineJoin {
	case kurboext.JoinRound:
		stroke.LineJoin = okage.LineJoinRound
		if isThin {
			stroke.LineJoin = okage.LineJoinBevel
		}
	case kurboext.JoinBevel:
		stroke.LineJoin = okage.LineJoinBevel
	}
	var dashPeriod float32
	for _, n := range props.DashArray {
		dashPeriod += float32(math.Abs(float64(n)))
	}
	if dashPeriod*minFactor >= minDashPeriod {
		dashes := props.DashArray
		if len(dashes)%2 == 1 {
			dashes = append(append([]float32(nil), dashes...), dashes...)
		}
		if dash, ok := okage.NewStrokeDash(dashes, props.DashOffset); ok {
			stroke.Dash = dash
		}
	}
	return stroke
}

// strokePath is Renderer::stroke_path in hayro src/path.rs.
func (r *renderer) strokePath(path *kurboext.BezPath, props interpret.DrawProps, strokeProps *interpret.StrokeProps, isText bool) {
	stroke := r.strokeProperties(strokeProps, props.Transform, isText)
	extent := float64(stroke.Width) / 2 * math.Max(float64(stroke.MiterLimit), math.Sqrt2)
	bbox := kurboext.RectInflate(controlBox(path), extent, extent)
	if r.missesTarget(props.Transform.TransformRectBbox(bbox)) {
		return
	}
	end := r.applyDrawProps(props.SoftMask, props.BlendMode)
	defer end()
	shader, clips := r.setPaint(props.Paint, props.Transform, path.BoundingBox, true, float64(stroke.Width))
	if shader == nil {
		return
	}
	defer r.popPaintClips(r.pushPaintClips(clips, interpret.FillRuleNonZero))
	r.dev.Save()
	defer r.dev.Restore()
	r.dev.Concat(toTransform(props.Transform))
	adjust := strokeAdjust(strokeProps, isText)
	if !adjust {
		if outline := singleSegmentOutline(path, &stroke); outline != nil {
			r.dev.FillPath(outline, okage.FillRuleWinding, shader)
			return
		}
	}
	p := toPath(path)
	if p == nil {
		return
	}
	if adjust {
		display.StrokeAdjusted(r.dev, p, stroke, shader)
		return
	}
	r.dev.StrokePath(p, stroke, shader)
}

// strokeAdjust reports whether a stroke has the stroke adjustment of the
// display device: when the entry SA of the graphics state is true. The
// outlines of text do not. MuPDF and Ghostscript also adjust a stroke only
// when SA is true. Poppler adjusts every stroke.
//
// Port: upstream has no stroke adjustment.
func strokeAdjust(props *interpret.StrokeProps, isText bool) bool {
	return props.StrokeAdjust && !isText
}

// fillPath is Renderer::fill_path in hayro src/path.rs.
func (r *renderer) fillPath(path *okage.Path, bbox func() kurbo.Rect, props interpret.DrawProps, fillRule interpret.FillRule) {
	if path == nil {
		return
	}
	end := r.applyDrawProps(props.SoftMask, props.BlendMode)
	defer end()
	shader, clips := r.setPaint(props.Paint, props.Transform, bbox, false, 0)
	if shader == nil {
		return
	}
	defer r.popPaintClips(r.pushPaintClips(clips, fillRule))
	r.dev.Save()
	defer r.dev.Restore()
	r.dev.Concat(toTransform(props.Transform))
	r.dev.FillPath(path, convertFillRule(fillRule), shader)
}

// drawPath is Renderer::draw_path in hayro src/path.rs.
func (r *renderer) drawPath(path *kurboext.BezPath, props interpret.DrawProps, mode interpret.DrawMode) {
	switch m := mode.(type) {
	case interpret.DrawModeFill:
		r.fillPath(toPath(path), path.BoundingBox, props, m.Rule)
	case interpret.DrawModeStroke:
		r.strokePath(path, props, &m.Props, false)
	case interpret.DrawModeFillAndStroke:
		r.fillPath(toPath(path), path.BoundingBox, props, m.Rule)
		r.strokePath(path, props, &m.Props, false)
	}
}

// drawRect is Renderer::draw_rect in hayro src/path.rs.
func (r *renderer) drawRect(rect kurbo.Rect, props interpret.DrawProps, mode interpret.DrawMode) {
	bbox := func() kurbo.Rect { return rect }
	switch m := mode.(type) {
	case interpret.DrawModeFill:
		r.fillPath(rectPath(rect), bbox, props, m.Rule)
	case interpret.DrawModeStroke:
		path := kurboext.RectToPath(rect, 0.1)
		r.strokePath(&path, props, &m.Props, false)
	case interpret.DrawModeFillAndStroke:
		r.fillPath(rectPath(rect), bbox, props, m.Rule)
		path := kurboext.RectToPath(rect, 0.1)
		r.strokePath(&path, props, &m.Props, false)
	}
}

// maxFactor is max_factor in hayro src/path.rs. It returns the largest
// scale of the transform along its two axes.
func maxFactor(transform kurbo.Affine) float32 {
	x, y := interpret.XYAdvances(transform)
	return float32(math.Max(x.Hypot(), y.Hypot()))
}

// singleSegmentOutline is single_segment_outline in hayro src/path.rs. It
// returns the outline of the stroke of a path of one line, without dashes
// and round caps, or nil.
func singleSegmentOutline(path *kurboext.BezPath, stroke *okage.Stroke) *okage.Path {
	if stroke.Dash != nil {
		return nil
	}
	els := path.Elements()
	if len(els) != 2 || els[0].Kind != kurbo.PathElMoveTo || els[1].Kind != kurbo.PathElLineTo {
		return nil
	}
	p0, p1 := els[0].P[0], els[1].P[0]
	d := p1.Sub(p0)
	length := d.Hypot()
	if length <= 0 || math.IsInf(length, 0) || math.IsNaN(length) {
		return nil
	}
	half := 0.5 * float64(stroke.Width)
	dir := d.Mul(1 / length)
	n := kurbo.NewVec2(-dir.Y, dir.X).Mul(half)
	a, b := p0, p1
	switch stroke.LineCap {
	case okage.LineCapSquare:
		a, b = p0.SubVec(dir.Mul(half)), p1.Add(dir.Mul(half))
	case okage.LineCapRound:
		return nil
	}
	return toPathElements([]kurbo.PathEl{
		kurbo.MoveTo(a.Add(n)),
		kurbo.LineTo(b.Add(n)),
		kurbo.LineTo(b.SubVec(n)),
		kurbo.LineTo(a.SubVec(n)),
		kurbo.ClosePath(),
	})
}

// rustMaxF32 is f32::max of Rust: it returns the other value when one is
// NaN.
func rustMaxF32(a, b float32) float32 {
	return float32(rustMax(float64(a), float64(b)))
}
