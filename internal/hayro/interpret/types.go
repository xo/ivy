// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/types.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// ClipPath is ClipPath in hayro-interpret src/types.rs. It is a path that
// clips the drawing.
type ClipPath struct {
	// Path is the clipping path.
	Path kurboext.BezPath
	// Fill is the fill rule.
	Fill FillRule
}

// CacheKey is the CacheKey implementation of ClipPath in hayro-interpret
// src/types.rs.
//
// Port: upstream hashes the SVG text of the path. Go hashes the elements of
// the path, which identify it in the same way.
func (c *ClipPath) CacheKey() Key128 {
	h := newKeyHasher()
	for _, el := range c.Path.Elements() {
		h.u64(uint64(el.Kind))
		for _, p := range el.P {
			h.f64(p.X).f64(p.Y)
		}
	}
	return h.u64(uint64(c.Fill)).sum()
}

// Image is Image in hayro-interpret src/types.rs. It is an image to draw:
// a *StencilImage or a *RasterImage.
//
// Port: upstream is an enum. Go uses an interface that only the two types
// implement. A device tells them apart with a type switch.
type Image interface {
	CacheKey
	// Width is the width of the image that its dictionary states. A device
	// uses the size of the decoded data instead.
	Width() uint32
	// Height is the height of the image that its dictionary states.
	Height() uint32
	isImage()
}

func (*StencilImage) isImage() {}
func (*RasterImage) isImage()  {}

// StencilImage is StencilImage in hayro-interpret src/types.rs. It is a
// mask that is painted with a paint.
type StencilImage struct {
	paint        Paint
	imageXObject *imageXObject
}

// WithStencil is StencilImage::with_stencil in hayro-interpret
// src/types.rs. It calls fn with the decoded mask and the paint. A nonzero
// targetDimension hints at the size that the device needs. fn is not
// called when the mask cannot be decoded.
//
// Port: upstream takes an Option for the size. Go uses the zero size for
// None, as ImageDecodeParams of syntax does.
func (s *StencilImage) WithStencil(fn func(LumaData, Paint), targetDimension [2]uint32) {
	if decoded, ok := s.imageXObject.decodedMask(targetDimension); ok {
		fn(decoded.luma, s.paint)
	}
}

// Stream is StencilImage::stream in hayro-interpret src/types.rs. It
// returns the stream of the image.
func (s *StencilImage) Stream() syntax.Stream {
	return s.imageXObject.stream
}

// Width is StencilImage::width in hayro-interpret src/types.rs.
func (s *StencilImage) Width() uint32 {
	return s.imageXObject.width
}

// Height is StencilImage::height in hayro-interpret src/types.rs.
func (s *StencilImage) Height() uint32 {
	return s.imageXObject.height
}

// CacheKey is the CacheKey implementation of StencilImage in
// hayro-interpret src/types.rs.
func (s *StencilImage) CacheKey() Key128 {
	return s.imageXObject.cacheKey()
}

// RasterImage is RasterImage in hayro-interpret src/types.rs. It is an
// image with colors.
type RasterImage struct {
	imageXObject *imageXObject
}

// WithRGBA is RasterImage::with_rgba in hayro-interpret src/types.rs. It
// calls fn with the decoded colors and alpha channel. The alpha channel is
// nil when the image has none. A nonzero targetDimension hints at the size
// that the device needs. fn is not called when the image cannot be decoded.
func (r *RasterImage) WithRGBA(fn func(ImageData, *LumaData), targetDimension [2]uint32) {
	if decoded, ok := r.imageXObject.decodedImage(targetDimension); ok {
		fn(decoded.image, decoded.alpha)
	}
}

// Stream is RasterImage::stream in hayro-interpret src/types.rs. It returns
// the stream of the image.
func (r *RasterImage) Stream() syntax.Stream {
	return r.imageXObject.stream
}

// Width is RasterImage::width in hayro-interpret src/types.rs.
func (r *RasterImage) Width() uint32 {
	return r.imageXObject.width
}

// Height is RasterImage::height in hayro-interpret src/types.rs.
func (r *RasterImage) Height() uint32 {
	return r.imageXObject.height
}

// CacheKey is the CacheKey implementation of RasterImage in
// hayro-interpret src/types.rs.
func (r *RasterImage) CacheKey() Key128 {
	return r.imageXObject.cacheKey()
}

// RGBData is RgbData in hayro-interpret src/types.rs. It holds the pixels
// of an image with 3 channels.
type RGBData struct {
	// Data holds the pixels, of length Width * Height * 3.
	Data []byte
	// Width is the width.
	Width uint32
	// Height is the height.
	Height uint32
	// Interpolate is true when the image is to be interpolated.
	Interpolate bool
	// ScaleFactors holds the factors in x and y that scale the image so
	// that it covers its area.
	ScaleFactors [2]float32
}

// LumaData is LumaData in hayro-interpret src/types.rs. It holds the
// pixels of an image with 1 channel.
type LumaData struct {
	// Data holds the pixels, of length Width * Height.
	Data []byte
	// Width is the width.
	Width uint32
	// Height is the height.
	Height uint32
	// Interpolate is true when the image is to be interpolated.
	Interpolate bool
	// ScaleFactors holds the factors in x and y that scale the image so
	// that it covers its area.
	ScaleFactors [2]float32
}

// ImageData is ImageData in hayro-interpret src/types.rs. It holds the
// colors of a raster image, with 3 channels or with 1.
//
// Port: upstream is an enum. Go sets one of the two fields.
type ImageData struct {
	// RGB holds the data of ImageData::Rgb.
	RGB *RGBData
	// Luma holds the data of ImageData::Luma.
	Luma *LumaData
}

// Width is ImageData::width in hayro-interpret src/types.rs.
func (d ImageData) Width() uint32 {
	if d.RGB != nil {
		return d.RGB.Width
	}
	return d.Luma.Width
}

// Height is ImageData::height in hayro-interpret src/types.rs.
func (d ImageData) Height() uint32 {
	if d.RGB != nil {
		return d.RGB.Height
	}
	return d.Luma.Height
}

// Interpolate is ImageData::interpolate in hayro-interpret src/types.rs.
func (d ImageData) Interpolate() bool {
	if d.RGB != nil {
		return d.RGB.Interpolate
	}
	return d.Luma.Interpolate
}

// ScaleFactors is ImageData::scale_factors in hayro-interpret
// src/types.rs.
func (d ImageData) ScaleFactors() [2]float32 {
	if d.RGB != nil {
		return d.RGB.ScaleFactors
	}
	return d.Luma.ScaleFactors
}

// Paint is Paint in hayro-interpret src/types.rs. It is a Color, a
// *ShadingPattern or a *TilingPattern.
//
// Port: upstream is an enum with the cases Color and Pattern, and Pattern
// is an enum with the cases Shading and Tiling. Go uses an interface that
// only the three types implement. A device tells them apart with a type
// switch.
type Paint interface {
	CacheKey
	isPaint()
}

func (Color) isPaint()           {}
func (*ShadingPattern) isPaint() {}
func (*TilingPattern) isPaint()  {}

// DrawProps is DrawProps in hayro-interpret src/types.rs. It holds the
// properties of a drawing that paints.
type DrawProps struct {
	// Transform is the transform.
	Transform kurbo.Affine
	// Paint is the paint.
	Paint Paint
	// SoftMask is the soft mask, or nil.
	SoftMask *SoftMask
	// BlendMode is the blend mode.
	BlendMode BlendMode
}

// ImageDrawProps is ImageDrawProps in hayro-interpret src/types.rs. It
// holds the properties of the drawing of an image.
type ImageDrawProps struct {
	// Transform is the transform.
	Transform kurbo.Affine
	// SoftMask is the soft mask, or nil.
	SoftMask *SoftMask
	// BlendMode is the blend mode.
	BlendMode BlendMode
}

// DrawMode is DrawMode in hayro-interpret src/types.rs. It is
// DrawModeFill, DrawModeStroke, DrawModeFillAndStroke or
// DrawModeInvisible.
//
// Port: upstream is an enum. Go uses an interface that only the four types
// implement.
type DrawMode interface {
	isDrawMode()
}

// DrawModeFill is DrawMode::Fill in hayro-interpret src/types.rs. It fills
// with a fill rule.
type DrawModeFill struct {
	// Rule is the fill rule.
	Rule FillRule
}

// DrawModeStroke is DrawMode::Stroke in hayro-interpret src/types.rs. It
// strokes.
type DrawModeStroke struct {
	// Props are the properties of the stroke.
	Props StrokeProps
}

// DrawModeFillAndStroke is DrawMode::FillAndStroke in hayro-interpret
// src/types.rs. It fills and strokes.
type DrawModeFillAndStroke struct {
	// Rule is the fill rule.
	Rule FillRule
	// Props are the properties of the stroke.
	Props StrokeProps
}

// DrawModeInvisible is DrawMode::Invisible in hayro-interpret
// src/types.rs. It marks text that is extracted but not drawn.
type DrawModeInvisible struct{}

func (DrawModeFill) isDrawMode()          {}
func (DrawModeStroke) isDrawMode()        {}
func (DrawModeFillAndStroke) isDrawMode() {}
func (DrawModeInvisible) isDrawMode()     {}

// StrokeProps is StrokeProps in hayro-interpret src/types.rs. It holds the
// properties of a stroke.
type StrokeProps struct {
	// LineWidth is the width of the line.
	LineWidth float32
	// LineCap is the cap of the line.
	LineCap kurboext.Cap
	// LineJoin is the join of the line.
	LineJoin kurboext.Join
	// MiterLimit is the miter limit.
	MiterLimit float32
	// DashArray is the dash array.
	DashArray []float32
	// DashOffset is the dash offset.
	DashOffset float32
	// StrokeAdjust is the automatic stroke adjustment, the entry SA of a
	// graphics state.
	//
	// Port: upstream has no such field, and ignores SA.
	StrokeAdjust bool
}

// DefaultStrokeProps is the Default implementation of StrokeProps in
// hayro-interpret src/types.rs.
func DefaultStrokeProps() StrokeProps {
	return StrokeProps{
		LineWidth:  1,
		LineCap:    kurboext.CapButt,
		LineJoin:   kurboext.JoinMiter,
		MiterLimit: 10,
	}
}

// FillRule is FillRule in hayro-interpret src/types.rs.
type FillRule uint8

// The fill rules.
const (
	// FillRuleNonZero is FillRule::NonZero.
	FillRuleNonZero FillRule = iota
	// FillRuleEvenOdd is FillRule::EvenOdd.
	FillRuleEvenOdd
)

// BlendMode is BlendMode in hayro-interpret src/types.rs. The zero value is
// BlendModeNormal, the default of upstream.
type BlendMode uint8

// The blend modes.
const (
	// BlendModeNormal is BlendMode::Normal.
	BlendModeNormal BlendMode = iota
	// BlendModeMultiply is BlendMode::Multiply.
	BlendModeMultiply
	// BlendModeScreen is BlendMode::Screen.
	BlendModeScreen
	// BlendModeOverlay is BlendMode::Overlay.
	BlendModeOverlay
	// BlendModeDarken is BlendMode::Darken.
	BlendModeDarken
	// BlendModeLighten is BlendMode::Lighten.
	BlendModeLighten
	// BlendModeColorDodge is BlendMode::ColorDodge.
	BlendModeColorDodge
	// BlendModeColorBurn is BlendMode::ColorBurn.
	BlendModeColorBurn
	// BlendModeHardLight is BlendMode::HardLight.
	BlendModeHardLight
	// BlendModeSoftLight is BlendMode::SoftLight.
	BlendModeSoftLight
	// BlendModeDifference is BlendMode::Difference.
	BlendModeDifference
	// BlendModeExclusion is BlendMode::Exclusion.
	BlendModeExclusion
	// BlendModeHue is BlendMode::Hue.
	BlendModeHue
	// BlendModeSaturation is BlendMode::Saturation.
	BlendModeSaturation
	// BlendModeColor is BlendMode::Color.
	BlendModeColor
	// BlendModeLuminosity is BlendMode::Luminosity.
	BlendModeLuminosity
)
