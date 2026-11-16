// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/device.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/pdf/internal/kurboext"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// Device is the trait Device in hayro-interpret src/device.rs. A device
// receives the marks of a page from the interpreter.
//
// Port: the trait of upstream has default methods for push_clip_rect,
// draw_rect, begin_marked_content and end_marked_content. Go has no default
// methods. A device that wants them implements ClipRectPusher, RectDrawer
// or MarkedContentDevice. The interpreter uses the default of upstream for
// a device that does not.
type Device interface {
	// DrawPath is Device::draw_path. It draws a path.
	DrawPath(path *kurboext.BezPath, props DrawProps, mode DrawMode)
	// PushClipPath is Device::push_clip_path. It pushes a clip path onto
	// the stack of clips.
	PushClipPath(clip *ClipPath)
	// PushTransparencyGroup is Device::push_transparency_group. It pushes a
	// transparency group onto the stack of blends. mask is nil when the
	// group has no soft mask.
	PushTransparencyGroup(opacity float32, mask *SoftMask, blendMode BlendMode, isolated, knockout bool)
	// DrawGlyphRun is Device::draw_glyph_run. It draws a run of glyphs.
	DrawGlyphRun(run *GlyphRun, props DrawProps, mode DrawMode)
	// DrawImage is Device::draw_image. It draws an image.
	DrawImage(image Image, props ImageDrawProps)
	// PopClip is Device::pop_clip. It pops the last clip from the stack of
	// clips.
	PopClip()
	// PopTransparencyGroup is Device::pop_transparency_group. It pops the
	// last transparency group from the stack of blends.
	PopTransparencyGroup()
}

// ClipRectPusher is a Device that implements Device::push_clip_rect of
// hayro-interpret src/device.rs.
type ClipRectPusher interface {
	// PushClipRect pushes a rectangle onto the stack of clips.
	PushClipRect(rect kurbo.Rect)
}

// RectDrawer is a Device that implements Device::draw_rect of
// hayro-interpret src/device.rs.
type RectDrawer interface {
	// DrawRect draws a rectangle without the general path pipeline.
	DrawRect(rect kurbo.Rect, props DrawProps, mode DrawMode)
}

// MarkedContentDevice is a Device that implements
// Device::begin_marked_content and Device::end_marked_content of
// hayro-interpret src/device.rs.
//
// Port: upstream gives the tag as bytes. Go gives the name.
type MarkedContentDevice interface {
	// BeginMarkedContent is called at the start of a sequence of marked
	// content, with its tag and its list of properties, or nil.
	BeginMarkedContent(tag syntax.Name, properties *syntax.Dict)
	// EndMarkedContent is called at the end of a sequence of marked
	// content.
	EndMarkedContent()
}

// pushClipRect calls Device::push_clip_rect of hayro-interpret
// src/device.rs, or its default.
func pushClipRect(d Device, rect kurbo.Rect) {
	if p, ok := d.(ClipRectPusher); ok {
		p.PushClipRect(rect)
		return
	}
	d.PushClipPath(&ClipPath{Path: kurboext.RectToPath(rect, 0.1), Fill: FillRuleNonZero})
}

// drawRect calls Device::draw_rect of hayro-interpret src/device.rs, or its
// default.
func drawRect(d Device, rect kurbo.Rect, props DrawProps, mode DrawMode) {
	if r, ok := d.(RectDrawer); ok {
		r.DrawRect(rect, props, mode)
		return
	}
	path := kurboext.RectToPath(rect, 0.1)
	d.DrawPath(&path, props, mode)
}

// beginMarkedContent calls Device::begin_marked_content of hayro-interpret
// src/device.rs, or its default, which does nothing.
func beginMarkedContent(d Device, tag syntax.Name, properties *syntax.Dict) {
	if m, ok := d.(MarkedContentDevice); ok {
		m.BeginMarkedContent(tag, properties)
	}
}

// endMarkedContent calls Device::end_marked_content of hayro-interpret
// src/device.rs, or its default, which does nothing.
func endMarkedContent(d Device) {
	if m, ok := d.(MarkedContentDevice); ok {
		m.EndMarkedContent()
	}
}

// DummyDevice is DummyDevice in hayro-interpret src/device.rs. It discards
// each mark.
type DummyDevice struct{}

// DrawPath does nothing.
func (DummyDevice) DrawPath(*kurboext.BezPath, DrawProps, DrawMode) {}

// PushClipPath does nothing.
func (DummyDevice) PushClipPath(*ClipPath) {}

// PushTransparencyGroup does nothing.
func (DummyDevice) PushTransparencyGroup(float32, *SoftMask, BlendMode, bool, bool) {}

// DrawGlyphRun does nothing.
func (DummyDevice) DrawGlyphRun(*GlyphRun, DrawProps, DrawMode) {}

// DrawImage does nothing.
func (DummyDevice) DrawImage(Image, ImageDrawProps) {}

// PopClip does nothing.
func (DummyDevice) PopClip() {}

// PopTransparencyGroup does nothing.
func (DummyDevice) PopTransparencyGroup() {}
