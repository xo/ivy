// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro src/image.rs.

package render

import (
	"image"
	"math"

	xdraw "golang.org/x/image/draw"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/pdf/internal/interpret"
)

// maxResizedSide is (u16::MAX / 2) of hayro src/image.rs: the largest
// width or height of a resized image.
const maxResizedSide = math.MaxUint16 / 2

// resampling is RESAMPLING_FUNCTION in hayro src/image.rs: the kernel of
// Hermite. It has no negative weights, so a resampled color that is
// premultiplied never passes its alpha.
//
// Port: upstream resizes with the crate pic_scale. The port resizes with
// golang.org/x/image/draw and the same kernel.
var resampling = &xdraw.Kernel{Support: 1, At: hermite}

// hermite is the cubic of Hermite, a cubic of B 0 and C 0.
func hermite(t float64) float64 {
	t = math.Abs(t)
	if t >= 1 {
		return 0
	}
	return (2*t-3)*t*t + 1
}

// solidColorImage is SolidColorImage in hayro src/image.rs. It is an image
// of one paint, with the size of its alpha channel.
//
// Port: upstream holds a color of RGB, and draws its alpha in a layer. The
// port holds the shader, which the display device fills through the
// alpha channel as a stencil, so that a solid image can also carry a
// pattern.
type solidColorImage struct {
	shader      okage.Shader
	width       uint32
	height      uint32
	interpolate bool
}

// renderImageData is RenderImageData in hayro src/image.rs. One of its
// fields is set.
type renderImageData struct {
	rgb   *interpret.RGBData
	luma  *interpret.LumaData
	solid *solidColorImage
}

// newRenderImageData is the From implementation of RenderImageData for
// ImageData in hayro src/image.rs.
func newRenderImageData(data interpret.ImageData) renderImageData {
	return renderImageData{rgb: data.RGB, luma: data.Luma}
}

// width is RenderImageData::width in hayro src/image.rs.
func (d renderImageData) width() uint32 {
	switch {
	case d.rgb != nil:
		return d.rgb.Width
	case d.luma != nil:
		return d.luma.Width
	case d.solid != nil:
		return d.solid.width
	}
	return 0
}

// height is RenderImageData::height in hayro src/image.rs.
func (d renderImageData) height() uint32 {
	switch {
	case d.rgb != nil:
		return d.rgb.Height
	case d.luma != nil:
		return d.luma.Height
	case d.solid != nil:
		return d.solid.height
	}
	return 0
}

// interpolate is RenderImageData::interpolate in hayro src/image.rs.
func (d renderImageData) interpolate() bool {
	switch {
	case d.rgb != nil:
		return d.rgb.Interpolate
	case d.luma != nil:
		return d.luma.Interpolate
	case d.solid != nil:
		return d.solid.interpolate
	}
	return false
}

// drawImageWithAlphaMask is Renderer::draw_image_with_alpha_mask in hayro
// src/image.rs. It draws an image whose alpha channel has another size or
// another interpolation. t is the transform of the image.
//
// Port: upstream draws the image in a layer, and the alpha channel in a
// second layer that composes with DestIn. The port draws the image in a
// group with a soft mask of type alpha, whose content is the alpha channel.
func (r *renderer) drawImageWithAlphaMask(data renderImageData, alpha interpret.LumaData, t kurbo.Affine) {
	imgWidth, imgHeight := data.width(), data.height()
	maskTransform := t.Mul(kurbo.AffineScaleNonUniform(
		float64(imgWidth)/float64(alpha.Width),
		float64(imgHeight)/float64(alpha.Height),
	))
	maskImage := renderImageData{solid: &solidColorImage{
		shader:      okage.SolidColor(okage.ColorBlack),
		width:       alpha.Width,
		height:      alpha.Height,
		interpolate: alpha.Interpolate,
	}}
	r.dev.BeginGroup(display.Group{
		Opacity:  1,
		Isolated: true,
		Mask: &display.Mask{
			Type: okage.MaskTypeAlpha,
			Content: func(d display.Device) {
				// No recursion: the size and the interpolation of the solid
				// image always match alpha.
				r.child(d).drawImage(maskImage, &alpha, maskTransform)
			},
		},
	})
	r.drawImage(data, nil, t)
	r.dev.EndGroup()
}

// drawImage is Renderer::draw_image in hayro src/image.rs. t is the
// transform of the image: it maps its pixels to the space of the target.
//
// Port: upstream builds a pixmap of RGBA, which it resizes in the formats
// of luma, RGB or RGBA. The port builds an image of RGBA, premultiplied, and
// resizes it, which gives the same pixels. A solid image becomes the stencil
// of FillMask. The data of an image shorter than its size draws nothing.
func (r *renderer) drawImage(data renderImageData, alpha *interpret.LumaData, t kurbo.Affine) {
	imgWidth, imgHeight := data.width(), data.height()
	if imgWidth == 0 || imgHeight == 0 {
		return
	}
	xAdvance, yAdvance := interpret.XYAdvances(t)
	xScale, yScale := float32(xAdvance.Hypot()), float32(yAdvance.Hypot())
	interpolate := data.interpolate() || r.global.forceImageInterpolation
	if alpha != nil && (alpha.Width != imgWidth || alpha.Height != imgHeight ||
		(alpha.Interpolate || r.global.forceImageInterpolation) != interpolate) {
		if alpha.Width == 0 || alpha.Height == 0 {
			return
		}
		r.drawImageWithAlphaMask(data, *alpha, t)
		return
	}
	smooth := interpolate
	needsResize := xScale < 1 || yScale < 1
	newWidth, newHeight := imgWidth, imgHeight
	if needsResize {
		newWidth = resizedSide(imgWidth, xScale)
		newHeight = resizedSide(imgHeight, yScale)
		if r.inType3Glyph {
			smooth = true
		}
	}
	n := int(imgWidth) * int(imgHeight)
	if alpha != nil && len(alpha.Data) < n {
		return
	}
	var rgba *image.RGBA
	if data.solid != nil {
		if alpha == nil {
			return
		}
		rgba = alphaImage(alpha.Data[:n], imgWidth, imgHeight)
	} else {
		rgba = premultipliedImage(data, alpha, imgWidth, imgHeight)
		if rgba == nil {
			return
		}
	}
	additional := kurbo.IdentityAffine
	if needsResize && (newWidth != imgWidth || newHeight != imgHeight) {
		resized := image.NewRGBA(image.Rect(0, 0, int(newWidth), int(newHeight)))
		resampling.Scale(resized, resized.Rect, rgba, rgba.Rect, xdraw.Src, nil)
		additional = kurbo.AffineScaleNonUniform(
			float64(imgWidth)/float64(newWidth),
			float64(imgHeight)/float64(newHeight),
		)
		rgba = resized
	}
	// Pad the images of glyphs of Type 3 with transparent pixels, so that
	// the sampling of the edges does not make them look bold.
	if r.inType3Glyph {
		padded := image.NewRGBA(image.Rect(0, 0, rgba.Rect.Dx()+4, rgba.Rect.Dy()+4))
		xdraw.Copy(padded, image.Pt(2, 2), rgba, rgba.Rect, xdraw.Src, nil)
		rgba = padded
		additional = additional.Mul(kurbo.AffineTranslate(kurbo.NewVec2(-2, -2)))
	}
	opts := display.ImageOptions{Opacity: 1, Smooth: smooth}
	r.dev.Save()
	defer r.dev.Restore()
	r.dev.Concat(toTransform(t))
	if data.solid != nil {
		mask := &image.Alpha{Pix: make([]byte, len(rgba.Pix)/4), Stride: rgba.Rect.Dx(), Rect: rgba.Rect}
		for i := range mask.Pix {
			mask.Pix[i] = rgba.Pix[4*i+3]
		}
		r.dev.FillMask(mask, toTransform(additional), opts, data.solid.shader)
		return
	}
	r.dev.DrawImage(rgba, toTransform(additional), opts)
}

// resizedSide returns the side of an image of side pixels at the scale,
// as Renderer::draw_image of hayro src/image.rs computes it.
func resizedSide(side uint32, scale float32) uint32 {
	v := float64(side) * float64(scale)
	v = math.Ceil(v)
	switch {
	case !(v >= 1):
		return 1
	case v > maxResizedSide:
		return maxResizedSide
	}
	return uint32(v)
}

// alphaImage returns an image of RGBA, premultiplied, of white with the
// alpha of a, so that a resize of its 4 channels resizes the alpha.
func alphaImage(a []byte, width, height uint32) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for i, v := range a {
		img.Pix[4*i] = v
		img.Pix[4*i+1] = v
		img.Pix[4*i+2] = v
		img.Pix[4*i+3] = v
	}
	return img
}

// premultipliedImage returns the colors of data and the alpha channel as an
// image of RGBA, premultiplied, or nil when the data is shorter than the
// size.
func premultipliedImage(data renderImageData, alpha *interpret.LumaData, width, height uint32) *image.RGBA {
	n := int(width) * int(height)
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	pix := img.Pix
	switch {
	case data.rgb != nil:
		if len(data.rgb.Data) < 3*n {
			return nil
		}
		src := data.rgb.Data
		for i := range n {
			pix[4*i] = src[3*i]
			pix[4*i+1] = src[3*i+1]
			pix[4*i+2] = src[3*i+2]
			pix[4*i+3] = 255
		}
	case data.luma != nil:
		if len(data.luma.Data) < n {
			return nil
		}
		for i, g := range data.luma.Data[:n] {
			pix[4*i] = g
			pix[4*i+1] = g
			pix[4*i+2] = g
			pix[4*i+3] = 255
		}
	default:
		return nil
	}
	if alpha != nil {
		for i, a := range alpha.Data[:n] {
			pix[4*i] = div255(uint16(pix[4*i]) * uint16(a))
			pix[4*i+1] = div255(uint16(pix[4*i+1]) * uint16(a))
			pix[4*i+2] = div255(uint16(pix[4*i+2]) * uint16(a))
			pix[4*i+3] = a
		}
	}
	return img
}

// div255 is div_255 in hayro src/image.rs: an approximation of v / 255.
func div255(v uint16) uint8 {
	return uint8((uint32(v) + 255) >> 8)
}

// drawPDFImage is Renderer::draw_pdf_image in hayro src/image.rs.
func (r *renderer) drawPDFImage(img interpret.Image, props interpret.ImageDrawProps) {
	local := kurbo.NewRect(0, 0, float64(img.Width()), float64(img.Height()))
	if r.missesTarget(props.Transform.TransformRectBbox(local)) {
		return
	}
	end := r.applyDrawProps(props.SoftMask, props.BlendMode)
	defer end()
	transform := props.Transform
	x, y := interpret.XYAdvances(transform)
	target := [2]uint32{
		asU32(math.Ceil(x.Hypot() * float64(img.Width()))),
		asU32(math.Ceil(y.Hypot() * float64(img.Height()))),
	}
	switch im := img.(type) {
	case *interpret.StencilImage:
		im.WithStencil(func(stencil interpret.LumaData, paint interpret.Paint) {
			transform := transform.Mul(kurbo.AffineScaleNonUniform(float64(stencil.ScaleFactors[0]), float64(stencil.ScaleFactors[1])))
			if c, ok := paint.(interpret.Color); ok {
				r.drawImage(renderImageData{solid: &solidColorImage{
					shader:      colorShader(c),
					width:       stencil.Width,
					height:      stencil.Height,
					interpolate: stencil.Interpolate,
				}}, &stencil, transform)
				return
			}
			stencilRect := kurbo.NewRect(0, 0, float64(stencil.Width), float64(stencil.Height))
			shader, clipPath := r.setPaint(paint, transform, func() kurbo.Rect { return stencilRect }, false, 0)
			if shader == nil {
				return
			}
			if clipPath != nil {
				r.pushClipPathInner(clipPath, interpret.FillRuleNonZero)
				defer r.popClip()
			}
			r.drawImage(renderImageData{solid: &solidColorImage{
				shader:      shader,
				width:       stencil.Width,
				height:      stencil.Height,
				interpolate: stencil.Interpolate,
			}}, &stencil, transform)
		}, target)
	case *interpret.RasterImage:
		im.WithRGBA(func(data interpret.ImageData, alpha *interpret.LumaData) {
			sf := data.ScaleFactors()
			transform := transform.Mul(kurbo.AffineScaleNonUniform(float64(sf[0]), float64(sf[1])))
			r.drawImage(newRenderImageData(data), alpha, transform)
		}, target)
	}
}

// asU32 is the cast `as u32` of Rust: it saturates, and NaN becomes 0.
func asU32(x float64) uint32 {
	switch {
	case math.IsNaN(x) || x <= 0:
		return 0
	case x >= math.MaxUint32:
		return math.MaxUint32
	}
	return uint32(x)
}
