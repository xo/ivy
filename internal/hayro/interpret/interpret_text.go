// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/interpret/text.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/pdffont"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// showTextString is show_text_string in hayro-interpret
// src/interpret/text.rs.
func showTextString(ctx *Context, device Device, resources *syntax.Resources, text syntax.String) {
	beginGlyphRun(ctx)
	appendTextString(ctx, resources, text)
	showGlyphRun(ctx, device)
}

// beginGlyphRun is begin_glyph_run in hayro-interpret
// src/interpret/text.rs.
func beginGlyphRun(ctx *Context) {
	clear(ctx.glyphScratch)
	ctx.glyphScratch = ctx.glyphScratch[:0]
}

// isASCII is <[u8]>::is_ascii of Rust.
func isASCII(b []byte) bool {
	for _, c := range b {
		if c >= 0x80 {
			return false
		}
	}
	return true
}

// appendTextString is append_text_string in hayro-interpret
// src/interpret/text.rs. It adds the glyphs of the text to the run, and
// moves the text matrix past them.
func appendTextString(ctx *Context, resources *syntax.Resources, text syntax.String) {
	font := ctx.textState().font
	if font == nil {
		return
	}
	data := text.AsBytes()
	showGlyphs := !font.fallback || isASCII(data)
	for cur := 0; cur < len(data); {
		code, adv := font.ReadCode(data, cur)
		if adv <= 0 {
			// Port: a code of no length makes the loop endless.
			adv = 1
		}
		cur += adv
		if showGlyphs && ctx.ocgState.isVisible() {
			// Port: each glyph counts as work, because a glyph of Type 3
			// runs a content stream (D18).
			if !ctx.run.step(1) {
				return
			}
			originDisplacement := font.OriginDisplacement(code)
			glyphID := font.MapCode(code)
			transform := ctx.textState().fullTransform().
				Mul(kurbo.AffineScale(1 / float64(pdffont.UnitsPerEm))).
				Mul(kurbo.AffineTranslate(originDisplacement)).
				Mul(kurbo.AffineScaleNonUniform(font.GlyphStretch(glyphID, code), 1))
			if glyph := getGlyph(font.Font, glyphID, code, ctx, resources); glyph != nil {
				ctx.glyphScratch = append(ctx.glyphScratch, PositionedGlyph{Glyph: glyph, transform: transform})
			}
		}
		ctx.textStateMut().applyCodeAdvance(code, adv)
	}
}

// applyGlyphRunAdjustment is apply_glyph_run_adjustment in hayro-interpret
// src/interpret/text.rs.
func applyGlyphRunAdjustment(ctx *Context, adjustment float32) {
	ctx.textStateMut().applyAdjustment(adjustment)
}

// nextLine is next_line in hayro-interpret src/interpret/text.rs.
func nextLine(ctx *Context, tx, ty float64) {
	t := ctx.textStateMut()
	m := t.textLineMatrix.Mul(kurbo.AffineTranslate(kurbo.NewVec2(tx, ty)))
	t.textLineMatrix = m
	t.textMatrix = m
}

// showGlyphRun is show_glyph_run in hayro-interpret src/interpret/text.rs. It
// sends the run to the device in the rendering mode of the text, and adds
// the outlines to the clip of the text for a mode that clips.
func showGlyphRun(ctx *Context, device Device) {
	if len(ctx.glyphScratch) == 0 {
		return
	}
	renderMode := ctx.textState().renderMode
	strokeProps := cloneStrokeProps(*ctx.strokeProps())
	fillProps := ctx.drawProps(false)
	strokeDrawProps := ctx.drawProps(true)
	run := &GlyphRun{glyphs: ctx.glyphScratch, writingMode: glyphRunWritingMode(ctx)}

	var clipPath *kurboext.BezPath
	switch renderMode {
	case textRenderingModeClip, textRenderingModeFillAndClip, textRenderingModeStrokeAndClip,
		textRenderingModeFillAndStrokeAndClip:
		var p kurboext.BezPath
		for i := range run.glyphs {
			clipGlyph(&p, &run.glyphs[i])
		}
		clipPath = &p
	}

	fill := DrawModeFill{Rule: FillRuleNonZero}
	stroke := DrawModeStroke{Props: strokeProps}
	switch renderMode {
	case textRenderingModeFill, textRenderingModeFillAndClip:
		device.DrawGlyphRun(run, fillProps, fill)
	case textRenderingModeStroke, textRenderingModeStrokeAndClip:
		device.DrawGlyphRun(run, strokeDrawProps, stroke)
	case textRenderingModeFillStroke, textRenderingModeFillAndStrokeAndClip:
		device.DrawGlyphRun(run, fillProps, fill)
		device.DrawGlyphRun(run, strokeDrawProps, stroke)
	case textRenderingModeInvisible, textRenderingModeClip:
		// Invisible text still reaches the device, for the extraction of
		// text. Upstream notes that the text of the mode Clip can have a
		// draw mode of its own, so that a device can tell it apart.
		device.DrawGlyphRun(run, fillProps, DrawModeInvisible{})
	}

	if clipPath != nil {
		ctx.textStateMut().clipPaths.Extend(clipPath.Elements())
	}
	beginGlyphRun(ctx)
}

// clipGlyph is clip_glyph in hayro-interpret src/interpret/text.rs. A glyph
// of Type 3 does not clip.
func clipGlyph(clipPath *kurboext.BezPath, glyph *PositionedGlyph) {
	if outline, ok := glyph.Glyph.(*pdffont.OutlineGlyph); ok {
		path := kurboext.MulBezPath(glyph.transform, outline.Outline())
		if path.HasSegments() {
			clipPath.Extend(path.Elements())
		}
	}
}

// textRenderingMode is TextRenderingMode in hayro-interpret
// src/interpret/text.rs. The values follow the operator Tr.
type textRenderingMode uint8

// The cases of TextRenderingMode.
const (
	textRenderingModeFill textRenderingMode = iota
	textRenderingModeStroke
	textRenderingModeFillStroke
	textRenderingModeInvisible
	textRenderingModeFillAndClip
	textRenderingModeStrokeAndClip
	textRenderingModeFillAndStrokeAndClip
	textRenderingModeClip
)
