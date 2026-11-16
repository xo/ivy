// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro src/clip.rs.

package render

import (
	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/pdf/internal/interpret"
	"github.com/xo/ivy/pdf/internal/kurboext"
)

// pushClipPathInner is Renderer::push_clip_path_inner in hayro
// src/clip.rs. The path is in the space of the pixels.
//
// Port: upstream pushes a clip of vello_cpu. The port saves the state of
// the display device and clips it, and popClip restores it. An empty path
// clips everything.
func (r *renderer) pushClipPathInner(clipPath *kurboext.BezPath, fill interpret.FillRule) {
	r.dev.Save()
	r.dev.ClipPath(toPath(clipPath), convertFillRule(fill))
}

// pushClipPath is Renderer::push_clip_path in hayro src/clip.rs.
func (r *renderer) pushClipPath(clip *interpret.ClipPath) {
	r.pushClipPathInner(&clip.Path, clip.Fill)
}

// pushClipRect is Renderer::push_clip_rect in hayro src/clip.rs.
func (r *renderer) pushClipRect(rect kurbo.Rect) {
	r.dev.Save()
	r.dev.ClipPath(rectPath(rect), okage.FillRuleWinding)
}

// popClip is Renderer::pop_clip in hayro src/clip.rs.
func (r *renderer) popClip() {
	r.dev.Restore()
}
