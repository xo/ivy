// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/interpret/path.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
)

// fillPath is fill_path in hayro-interpret src/interpret/path.rs.
func fillPath(ctx *Context, device Device, fillRule FillRule) {
	fillPathImpl(ctx, device, fillRule, nil)
	endPath(ctx, device)
}

// strokePath is stroke_path in hayro-interpret src/interpret/path.rs.
func strokePath(ctx *Context, device Device) {
	strokePathImpl(ctx, device, nil)
	endPath(ctx, device)
}

// fillStrokePath is fill_stroke_path in hayro-interpret
// src/interpret/path.rs.
func fillStrokePath(ctx *Context, device Device, fillRule FillRule) {
	fillPathImpl(ctx, device, fillRule, nil)
	strokePathImpl(ctx, device, nil)
	endPath(ctx, device)
}

// endPath is end_path in hayro-interpret src/interpret/path.rs. It applies
// a pending clip, and clears the path.
func endPath(ctx *Context, device Device) {
	if ctx.hasClip {
		clip := ctx.clip
		ctx.hasClip = false
		if ctx.path.Len() != 0 {
			clipPath := kurboext.MulBezPath(ctx.ctm(), ctx.path)
			ctx.pushClipPath(clipPath, clip, device)
		}
	}
	ctx.path.Truncate(0)
}

// closePath is close_path in hayro-interpret src/interpret/path.rs. Upstream
// notes that it avoids artifacts (issue 157), and that it causes odd lines
// in pdftc_900k_0907.
func closePath(ctx *Context) {
	els := ctx.path.Elements()
	if len(els) > 0 && els[len(els)-1].Kind != kurbo.PathElClosePath && !ctx.path.IsEmpty() {
		ctx.path.ClosePath()
		ctx.lastPoint = ctx.subPathStart
	}
}

// fillPathImpl is fill_path_impl in hayro-interpret src/interpret/path.rs.
// It fills path, or the current path when path is nil.
func fillPathImpl(ctx *Context, device Device, fillRule FillRule, path *kurboext.BezPath) {
	if !ctx.ocgState.isVisible() {
		return
	}
	props := ctx.drawProps(false)
	if path == nil {
		path = &ctx.path
	}
	// pdf.js issue 4260: a path of zero size becomes a small stroke.
	bbox := fastBoundingBox(path)
	xAdvance, yAdvance := XYAdvances(props.Transform)
	zeroWidth := IsNearlyZero(float32(bbox.Width() * xAdvance.Hypot()))
	zeroHeight := IsNearlyZero(float32(bbox.Height() * yAdvance.Hypot()))
	if !zeroWidth && !zeroHeight {
		mode := DrawModeFill{Rule: fillRule}
		if rect, ok := pathAsRect(path); ok {
			drawRect(device, rect, props, mode)
		} else {
			device.DrawPath(path, props, mode)
		}
		return
	}
	var line kurboext.BezPath
	line.MoveTo(kurbo.NewPoint(bbox.X0, bbox.Y0))
	line.LineTo(kurbo.NewPoint(bbox.X1, bbox.Y1))
	scale := float32(rustMax(xAdvance.Hypot(), yAdvance.Hypot()))
	strokeProps := DefaultStrokeProps()
	strokeProps.LineWidth = 0.001
	if scale > 0 {
		strokeProps.LineWidth = 1 / scale
	}
	strokeProps.LineJoin = kurboext.JoinBevel
	strokeProps.LineCap = kurboext.CapButt
	device.DrawPath(&line, props, DrawModeStroke{Props: strokeProps})
}

// strokePathImpl is stroke_path_impl in hayro-interpret
// src/interpret/path.rs. It strokes path, or the current path when path is
// nil.
func strokePathImpl(ctx *Context, device Device, path *kurboext.BezPath) {
	if !ctx.ocgState.isVisible() {
		return
	}
	strokeProps := cloneStrokeProps(*ctx.strokeProps())
	props := ctx.drawProps(true)
	if path == nil {
		path = &ctx.path
	}
	mode := DrawModeStroke{Props: strokeProps}
	if rect, ok := pathAsRect(path); ok {
		drawRect(device, rect, props, mode)
	} else {
		device.DrawPath(path, props, mode)
	}
}
