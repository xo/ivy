// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro src/lib.rs.

// Package render is a port of the crate hayro of hayro (D14, D16). It
// implements the Device of internal/hayro/interpret, and draws each mark of a
// page onto a display.Device of internal/display (D10). Where upstream calls
// vello_cpu, the port sends a command to the display device.
package render

import (
	"math"
	"sync"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// maxCachedOutlines is the number of outlines of glyphs that a RenderCache
// keeps (D18). Past it, the cache starts again empty.
const maxCachedOutlines = 1 << 16

// globalState is GlobalState in hayro src/lib.rs. It holds what the
// renderers of one page share.
//
// Port: the cache of hinted outlines, the scaler of images, the aliasing
// threshold, the hinting and the cache of glyph bitmaps are left out. ivy
// does not hint (D19), the port resizes images with golang.org/x/image/draw,
// and okage always anti-aliases.
type globalState struct {
	cache                   *RenderCache
	forceImageInterpolation bool
}

// newGlobalState is GlobalState::new in hayro src/lib.rs.
func newGlobalState(cache *RenderCache, settings *RenderSettings) *globalState {
	return &globalState{cache: cache, forceImageInterpolation: settings.ForceImageInterpolation}
}

// renderer is Renderer in hayro src/lib.rs. It implements the Device of
// interpret, and draws onto a display.Device.
//
// Port: upstream keeps the state of the drawing in a RenderContext of
// vello_cpu. The display device holds no such state, so each drawing of the
// port sets its transform with Save, Concat and Restore, and begins a group
// for its soft mask and its blend mode. The user space of dev at each call
// is the space of the pixels of the target. The cache of soft masks of
// upstream is the cache of the display device, by the key of the mask.
type renderer struct {
	global *globalState
	dev    display.Device
	// width and height are the size of the target in pixels.
	width, height float64
	insidePattern bool
	inType3Glyph  bool
}

// The interfaces that a renderer implements.
var (
	_ interpret.Device         = (*renderer)(nil)
	_ interpret.ClipRectPusher = (*renderer)(nil)
	_ interpret.RectDrawer     = (*renderer)(nil)
)

// newRenderer is Renderer::new in hayro src/lib.rs.
func newRenderer(dev display.Device, width, height float64, global *globalState) *renderer {
	return &renderer{global: global, dev: dev, width: width, height: height}
}

// missesTarget is Renderer::misses_target in hayro src/lib.rs.
func (r *renderer) missesTarget(bbox kurbo.Rect) bool {
	return !kurboext.RectOverlaps(bbox, kurbo.NewRect(0, 0, r.width, r.height))
}

// child returns a renderer that draws onto dev, with the size and the state
// of r. It takes the place of Renderer::child_context of hayro src/lib.rs.
func (r *renderer) child(dev display.Device) *renderer {
	c := *r
	c.dev = dev
	return &c
}

// applyDrawProps is Renderer::apply_draw_props in hayro src/lib.rs. It
// begins the group that the soft mask and the blend mode of a drawing need,
// and returns the function that ends it.
//
// Port: upstream sets the transform, the mask and the blend mode of the
// context. The display device has no mask or blend mode for one drawing, so
// the port draws it in a group with them. The transform goes with each
// drawing.
func (r *renderer) applyDrawProps(mask *interpret.SoftMask, blend interpret.BlendMode) func() {
	if mask == nil && blend == interpret.BlendModeNormal {
		return func() {}
	}
	r.dev.BeginGroup(display.Group{Opacity: 1, Blend: convertBlendMode(blend), Mask: r.softMask(mask)})
	return r.dev.EndGroup
}

// DrawImage is the implementation of Device::draw_image for Renderer in
// hayro src/lib.rs.
func (r *renderer) DrawImage(image interpret.Image, props interpret.ImageDrawProps) {
	r.drawPDFImage(image, props)
}

// PushClipPath is the implementation of Device::push_clip_path for Renderer
// in hayro src/lib.rs.
func (r *renderer) PushClipPath(clip *interpret.ClipPath) {
	r.pushClipPath(clip)
}

// PushClipRect is the implementation of Device::push_clip_rect for Renderer
// in hayro src/lib.rs.
func (r *renderer) PushClipRect(rect kurbo.Rect) {
	r.pushClipRect(rect)
}

// PushTransparencyGroup is the implementation of
// Device::push_transparency_group for Renderer in hayro src/lib.rs.
//
// Port: upstream ignores isolated and knockout. The port passes them to the
// display device.
func (r *renderer) PushTransparencyGroup(opacity float32, mask *interpret.SoftMask, blendMode interpret.BlendMode, isolated, knockout bool) {
	r.pushTransparencyGroup(opacity, mask, blendMode, isolated, knockout)
}

// PopClip is the implementation of Device::pop_clip for Renderer in hayro
// src/lib.rs.
func (r *renderer) PopClip() {
	r.popClip()
}

// PopTransparencyGroup is the implementation of
// Device::pop_transparency_group for Renderer in hayro src/lib.rs.
func (r *renderer) PopTransparencyGroup() {
	r.popTransparencyGroup()
}

// DrawPath is the implementation of Device::draw_path for Renderer in hayro
// src/lib.rs.
func (r *renderer) DrawPath(path *kurboext.BezPath, props interpret.DrawProps, mode interpret.DrawMode) {
	r.drawPath(path, props, mode)
}

// DrawRect is the implementation of Device::draw_rect for Renderer in hayro
// src/lib.rs.
func (r *renderer) DrawRect(rect kurbo.Rect, props interpret.DrawProps, mode interpret.DrawMode) {
	r.drawRect(rect, props, mode)
}

// DrawGlyphRun is the implementation of Device::draw_glyph_run for Renderer
// in hayro src/lib.rs.
func (r *renderer) DrawGlyphRun(run *interpret.GlyphRun, props interpret.DrawProps, mode interpret.DrawMode) {
	r.drawGlyphRun(run, props, mode)
}

// RenderCache is RenderCache in hayro src/lib.rs. It is a cache that the
// renders of the pages of one PDF file share. It is safe for concurrent
// use.
//
// Port: upstream also caches hinted outlines and the bitmaps of glyphs. The
// port caches the outlines of glyphs as paths of okage.
//
//nolint:revive // The name is RenderCache of upstream.
type RenderCache struct {
	// InterpreterCache is the cache of the interpreter.
	InterpreterCache *interpret.InterpreterCache

	mu       sync.Mutex
	outlines map[interpret.Key128]*okage.Path
}

// NewRenderCache is RenderCache::new in hayro src/lib.rs.
func NewRenderCache() *RenderCache {
	return &RenderCache{
		InterpreterCache: interpret.NewInterpreterCache(),
		outlines:         make(map[interpret.Key128]*okage.Path),
	}
}

// RenderSettings is RenderSettings in hayro src/lib.rs. It holds the
// settings of the rendering of a page.
//
// Port: upstream also has aliasing_threshold, hinting and glyph_caching.
// okage always anti-aliases, ivy does not hint (D19), and the port has no
// cache of glyph bitmaps.
//
//nolint:revive // The name is RenderSettings of upstream.
type RenderSettings struct {
	// ForceImageInterpolation draws each image with bilinear filtering.
	ForceImageInterpolation bool
}

// PixmapSettings is PixmapSettings in hayro src/lib.rs. It holds the
// settings of the pixmap that Render returns.
type PixmapSettings struct {
	// XScale is the horizontal scale factor.
	XScale float32
	// YScale is the vertical scale factor.
	YScale float32
	// BgColor is the color of the background.
	BgColor okage.Color
}

// DefaultPixmapSettings is the Default implementation of PixmapSettings in
// hayro src/lib.rs.
func DefaultPixmapSettings() PixmapSettings {
	return PixmapSettings{XScale: 1, YScale: 1, BgColor: okage.ColorTransparent}
}

// Render is render in hayro src/lib.rs. It renders a page to a new pixmap.
// It returns the pixmap with the error of the interpretation, which is nil
// when the page drew in full.
//
// Port: upstream returns only the pixmap. It returns nil when the pixmap
// cannot be made, such as for a page of a size of 0.
func Render(page *syntax.Page, cache *RenderCache, interpreterSettings interpret.InterpreterSettings, renderSettings *RenderSettings, pixmapSettings *PixmapSettings) (*okage.Pixmap, error) {
	width, height := page.RenderDimensions()
	w, h := asU16(width*pixmapSettings.XScale), asU16(height*pixmapSettings.YScale)
	pix, ok := okage.NewPixmap(uint32(w), uint32(h))
	if !ok {
		return nil, nil
	}
	pix.Fill(pixmapSettings.BgColor)
	transform := kurbo.AffineScaleNonUniform(float64(pixmapSettings.XScale), float64(pixmapSettings.YScale)).
		Mul(interpret.TransformToKurbo(page.InitialTransform(true)))
	dev := display.NewOkage(pix, okage.IdentityTransform())
	err := RenderInto(page, cache, interpreterSettings, renderSettings, dev, int(w), int(h), transform)
	return pix, err
}

// RenderInto is render_into in hayro src/lib.rs. It renders a page onto
// dev with the transform from the space of the page to the pixels of a
// target of width by height pixels. The user space of dev at the call must
// be the space of the pixels of the target. It returns the error of the
// interpretation, which is nil when the page drew in full.
//
// Port: upstream draws into a RenderContext of vello_cpu, and returns
// nothing.
//
//nolint:revive // The name is render_into of upstream.
func RenderInto(page *syntax.Page, cache *RenderCache, interpreterSettings interpret.InterpreterSettings, renderSettings *RenderSettings, dev display.Device, width, height int, transform kurbo.Affine) error {
	target := kurbo.NewRect(0, 0, float64(width), float64(height))
	state := interpret.NewContext(transform, target, cache.InterpreterCache, page.XRef(), interpreterSettings)
	global := newGlobalState(cache, renderSettings)
	device := newRenderer(dev, float64(width), float64(height), global)
	cropBox := interpret.RectToKurbo(page.IntersectedCropBox())
	a, b, c, d := transform[0], transform[1], transform[2], transform[3]
	axisAligned := (b == 0 && c == 0) || (a == 0 && d == 0)
	mappedCropBox := kurboext.RectInflate(transform.TransformRectBbox(cropBox), 1.0/256, 1.0/256)
	clipCropBox := !axisAligned || !kurboext.RectContainsRect(mappedCropBox, target)
	if clipCropBox {
		device.pushClipPath(&interpret.ClipPath{
			Path: kurboext.MulBezPath(transform, kurboext.RectToPath(cropBox, 0.1)),
			Fill: interpret.FillRuleNonZero,
		})
	}
	err := interpret.InterpretPage(page, state, device)
	if clipCropBox {
		device.popClip()
	}
	return err
}

// convertFillRule is convert_fill_rule in hayro src/lib.rs.
func convertFillRule(rule interpret.FillRule) okage.FillRule {
	if rule == interpret.FillRuleEvenOdd {
		return okage.FillRuleEvenOdd
	}
	return okage.FillRuleWinding
}

// convertBlendMode is convert_blend_mode in hayro src/lib.rs.
func convertBlendMode(mode interpret.BlendMode) okage.BlendMode {
	switch mode {
	case interpret.BlendModeMultiply:
		return okage.BlendModeMultiply
	case interpret.BlendModeScreen:
		return okage.BlendModeScreen
	case interpret.BlendModeOverlay:
		return okage.BlendModeOverlay
	case interpret.BlendModeDarken:
		return okage.BlendModeDarken
	case interpret.BlendModeLighten:
		return okage.BlendModeLighten
	case interpret.BlendModeColorDodge:
		return okage.BlendModeColorDodge
	case interpret.BlendModeColorBurn:
		return okage.BlendModeColorBurn
	case interpret.BlendModeHardLight:
		return okage.BlendModeHardLight
	case interpret.BlendModeSoftLight:
		return okage.BlendModeSoftLight
	case interpret.BlendModeDifference:
		return okage.BlendModeDifference
	case interpret.BlendModeExclusion:
		return okage.BlendModeExclusion
	case interpret.BlendModeHue:
		return okage.BlendModeHue
	case interpret.BlendModeSaturation:
		return okage.BlendModeSaturation
	case interpret.BlendModeColor:
		return okage.BlendModeColor
	case interpret.BlendModeLuminosity:
		return okage.BlendModeLuminosity
	}
	return okage.BlendModeSourceOver
}

// asU16 is the cast `as u16` of Rust: it saturates, and NaN becomes 0.
func asU16(x float32) uint16 {
	switch {
	case math.IsNaN(float64(x)) || x <= 0:
		return 0
	case x >= math.MaxUint16:
		return math.MaxUint16
	}
	return uint16(x)
}
