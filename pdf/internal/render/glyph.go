// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro src/glyph.rs.

package render

import (
	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/pdf/internal/interpret"
	"github.com/xo/ivy/pdf/internal/kurboext"
)

// fillGlyph is Renderer::fill_glyph in hayro src/glyph.rs.
//
// Port: upstream hints the outline. ivy does not hint (D19).
func (r *renderer) fillGlyph(glyph interpret.Glyph, props interpret.DrawProps, transform kurbo.Affine) {
	switch g := glyph.(type) {
	case *interpret.OutlineGlyph:
		outline := r.cachedOutline(g)
		props.Transform = props.Transform.Mul(transform)
		r.fillPath(outline, func() kurbo.Rect { return pathBounds(outline) }, props, interpret.FillRuleNonZero)
	case *interpret.Type3Glyph:
		r.inType3Glyph = true
		g.Interpret(r, props.Transform, transform, props.Paint)
		r.inType3Glyph = false
	}
}

// strokeGlyph is Renderer::stroke_glyph in hayro src/glyph.rs.
func (r *renderer) strokeGlyph(glyph interpret.Glyph, props interpret.DrawProps, transform kurbo.Affine, strokeProps *interpret.StrokeProps) {
	switch g := glyph.(type) {
	case *interpret.OutlineGlyph:
		path := kurboext.MulBezPath(transform, g.Outline())
		r.strokePath(&path, props, strokeProps, true)
	case *interpret.Type3Glyph:
		g.Interpret(r, props.Transform, transform, props.Paint)
	}
}

// cachedOutline is Renderer::cached_outline in hayro src/glyph.rs, with
// Renderer::cached_path of the same file. It returns the outline of the
// glyph as a path of okage, or nil for an empty outline. The path is
// shared, and a caller must not transform it in place.
//
// Port: upstream caches the outline of kurbo. The port caches the path of
// okage, which it draws.
func (r *renderer) cachedOutline(glyph *interpret.OutlineGlyph) *okage.Path {
	cache := r.global.cache
	key := glyph.Identifier().GlyphKey()
	cache.mu.Lock()
	path, ok := cache.outlines[key]
	cache.mu.Unlock()
	if ok {
		return path
	}
	outline := glyph.Outline()
	path = toPath(&outline)
	cache.mu.Lock()
	// Port: see maxCachedOutlines.
	if len(cache.outlines) >= maxCachedOutlines {
		cache.outlines = make(map[interpret.Key128]*okage.Path)
	}
	cache.outlines[key] = path
	cache.mu.Unlock()
	return path
}

// pathBounds returns the bounds of a path of okage as a rectangle of kurbo.
func pathBounds(p *okage.Path) kurbo.Rect {
	if p == nil {
		return kurbo.Rect{}
	}
	b := p.Bounds()
	return kurbo.NewRect(float64(b.Left()), float64(b.Top()), float64(b.Right()), float64(b.Bottom()))
}

// transformedFillOutlines is Renderer::transformed_fill_outlines in hayro
// src/glyph.rs. It returns the outlines of the run in the space of the run,
// and their bounds.
//
// Port: upstream hints the outlines, and maps them back from the space of
// the pixels. Without hinting the outlines are those of
// transformedOutlines.
func (r *renderer) transformedFillOutlines(run *interpret.GlyphRun) ([]*okage.Path, kurbo.Rect) {
	return r.transformedOutlines(run)
}

// transformedOutlines is Renderer::transformed_outlines in hayro
// src/glyph.rs, with Renderer::outline_bbox of the same file. It returns
// the outlines of the run in the space of the run, and their bounds.
func (r *renderer) transformedOutlines(run *interpret.GlyphRun) ([]*okage.Path, kurbo.Rect) {
	var outlines []*okage.Path
	var bbox kurbo.Rect
	for i := range run.Glyphs() {
		glyph := &run.Glyphs()[i]
		outline, ok := glyph.Glyph.(*interpret.OutlineGlyph)
		if !ok {
			continue
		}
		path := r.cachedOutline(outline)
		if path == nil {
			continue
		}
		p, ok := display.Clone(path).Transform(toTransform(glyph.Transform()))
		if !ok {
			continue
		}
		if len(outlines) == 0 {
			bbox = pathBounds(p)
		} else {
			bbox = bbox.Union(pathBounds(p))
		}
		outlines = append(outlines, p)
	}
	return outlines, bbox
}

// fillOutlineRun is Renderer::fill_outline_run in hayro src/glyph.rs.
//
// Port: upstream draws the glyphs of a color from a cache of bitmaps. The
// port fills each outline.
func (r *renderer) fillOutlineRun(run *interpret.GlyphRun, props *interpret.DrawProps) {
	if c, ok := props.Paint.(interpret.Color); ok {
		end := r.applyDrawProps(props.SoftMask, props.BlendMode)
		defer end()
		shader := colorShader(c)
		for i := range run.Glyphs() {
			glyph := &run.Glyphs()[i]
			outline, ok := glyph.Glyph.(*interpret.OutlineGlyph)
			if !ok {
				continue
			}
			path := r.cachedOutline(outline)
			if path == nil {
				continue
			}
			r.dev.Save()
			r.dev.Concat(toTransform(props.Transform.Mul(glyph.Transform())))
			// Port: ivy fills a glyph with display.FillText, which gives a
			// small glyph a finer coverage than FillPath.
			display.FillText(r.dev, path, okage.FillRuleWinding, shader)
			r.dev.Restore()
		}
		return
	}
	outlines, bbox := r.transformedFillOutlines(run)
	r.drawOutlines(outlines, bbox, props, nil)
}

// strokeOutlineRun is Renderer::stroke_outline_run in hayro src/glyph.rs.
func (r *renderer) strokeOutlineRun(run *interpret.GlyphRun, props *interpret.DrawProps, strokeProps *interpret.StrokeProps) {
	outlines, bbox := r.transformedOutlines(run)
	stroke := r.strokeProperties(strokeProps, props.Transform, true)
	if _, ok := props.Paint.(interpret.Color); ok {
		bbox = kurbo.Rect{}
	}
	r.drawOutlines(outlines, bbox, props, &stroke)
}

// drawOutlines fills, or strokes with stroke, the outlines of a run in the
// space of the run. It takes the place of Renderer::begin_fill_run,
// Renderer::begin_stroke_run and Renderer::finish_run of hayro
// src/glyph.rs.
func (r *renderer) drawOutlines(outlines []*okage.Path, bbox kurbo.Rect, props *interpret.DrawProps, stroke *okage.Stroke) {
	if len(outlines) == 0 {
		return
	}
	end := r.applyDrawProps(props.SoftMask, props.BlendMode)
	defer end()
	var width float64
	if stroke != nil {
		width = float64(stroke.Width)
	}
	shader, clipPath := r.setPaint(props.Paint, props.Transform, func() kurbo.Rect { return bbox }, stroke != nil, width)
	if shader == nil {
		return
	}
	if clipPath != nil {
		r.pushClipPathInner(clipPath, interpret.FillRuleNonZero)
		defer r.popClip()
	}
	r.dev.Save()
	defer r.dev.Restore()
	r.dev.Concat(toTransform(props.Transform))
	for _, outline := range outlines {
		if stroke != nil {
			r.dev.StrokePath(outline, *stroke, shader)
		} else {
			r.dev.FillPath(outline, okage.FillRuleWinding, shader)
		}
	}
}

// drawGlyphsIndividually is Renderer::draw_glyphs_individually in hayro
// src/glyph.rs.
func (r *renderer) drawGlyphsIndividually(run *interpret.GlyphRun, props interpret.DrawProps, mode interpret.DrawMode) {
	for i := range run.Glyphs() {
		glyph := &run.Glyphs()[i]
		transform := glyph.Transform()
		switch m := mode.(type) {
		case interpret.DrawModeFill:
			r.fillGlyph(glyph.Glyph, props, transform)
		case interpret.DrawModeStroke:
			r.strokeGlyph(glyph.Glyph, props, transform, &m.Props)
		case interpret.DrawModeFillAndStroke:
			r.fillGlyph(glyph.Glyph, props, transform)
			r.strokeGlyph(glyph.Glyph, props, transform, &m.Props)
		}
	}
}

// drawGlyphRun is Renderer::draw_glyph_run in hayro src/glyph.rs.
func (r *renderer) drawGlyphRun(run *interpret.GlyphRun, props interpret.DrawProps, mode interpret.DrawMode) {
	for i := range run.Glyphs() {
		if _, ok := run.Glyphs()[i].Glyph.(*interpret.OutlineGlyph); !ok {
			r.drawGlyphsIndividually(run, props, mode)
			return
		}
	}
	switch m := mode.(type) {
	case interpret.DrawModeFill:
		r.fillOutlineRun(run, &props)
	case interpret.DrawModeStroke:
		r.strokeOutlineRun(run, &props, &m.Props)
	case interpret.DrawModeFillAndStroke:
		r.fillOutlineRun(run, &props)
		r.strokeOutlineRun(run, &props, &m.Props)
	}
}
