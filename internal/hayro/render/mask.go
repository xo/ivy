// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro src/mask.rs.

package render

import (
	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/hayro/interpret"
)

// softMask is Renderer::apply_soft_mask in hayro src/mask.rs, with
// draw_soft_mask of the same file. It returns the soft mask of the display
// device for mask, or nil for a nil mask.
//
// Port: upstream draws the mask into a Mask of vello_cpu, and keeps it in a
// cache of the renderer. The port gives the display device a function that
// draws the content, and the cache key of the mask, so that the device
// draws the mask once and keeps it. The transfer function goes to the device
// as a table of the 256 values of a byte.
func (r *renderer) softMask(mask *interpret.SoftMask) *display.Mask {
	if mask == nil {
		return nil
	}
	m := &display.Mask{
		Type: okage.MaskTypeLuminance,
		Content: func(d display.Device) {
			mask.Interpret(newRenderer(d, r.width, r.height, r.global))
		},
		Key: mask.CacheKey(),
	}
	if mask.MaskType() == interpret.MaskTypeAlpha {
		m.Type = okage.MaskTypeAlpha
	}
	bg := mask.BackgroundColor().ToRGBA().ToRGBA8()
	if bg != [4]uint8{0, 0, 0, 255} {
		m.Backdrop = okage.ColorFromRGBA8(bg[0], bg[1], bg[2], bg[3])
	}
	if tf := mask.TransferFunction(); tf != nil {
		var table [256]uint8
		for i := range table {
			table[i] = uint8(i)
		}
		tf.ApplyTo(table[:])
		m.Transfer = &table
	}
	return m
}

// pushTransparencyGroup is Renderer::push_transparency_group in hayro
// src/mask.rs.
func (r *renderer) pushTransparencyGroup(opacity float32, mask *interpret.SoftMask, blendMode interpret.BlendMode, isolated, knockout bool) {
	r.dev.BeginGroup(display.Group{
		Opacity:  opacity,
		Blend:    convertBlendMode(blendMode),
		Isolated: isolated,
		Knockout: knockout,
		Mask:     r.softMask(mask),
	})
}

// popTransparencyGroup is Renderer::pop_transparency_group in hayro
// src/mask.rs.
func (r *renderer) popTransparencyGroup() {
	r.dev.EndGroup()
}
