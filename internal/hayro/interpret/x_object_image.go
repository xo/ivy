// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/x_object/image.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// imageXObject is ImageXObject in hayro-interpret src/x_object/image.rs.
//
// Port: upstream holds the warning sink. Go holds it in env, with the
// decoders of JBIG2 and JPX and the limits of the settings.
type imageXObject struct {
	width            uint32
	height           uint32
	colorSpace       ColorSpace
	hasColorSpace    bool
	cache            *Cache
	interpolate      bool
	kind             imageKind
	stream           syntax.Stream
	transferFunction *ActiveTransferFunction
	env              imageEnv
}

// imageEnv holds the settings that the decoding of an image uses.
type imageEnv struct {
	warningSink WarningSinkFn
	decoders    *syntax.ImageDecoders
	limits      ivy.Limits
}

// imageEnvOf returns the environment of the images of the settings.
func imageEnvOf(settings *InterpreterSettings) imageEnv {
	return imageEnv{warningSink: settings.WarningSink, decoders: settings.Decoders, limits: settings.Limits.WithDefaults()}
}

// warn sends a warning to the sink.
func (e *imageEnv) warn(w InterpreterWarning) {
	if e.warningSink != nil {
		e.warningSink(w)
	}
}

// imageKind is ImageKind in hayro-interpret src/x_object/image.rs.
type imageKind uint8

// The cases of ImageKind.
const (
	imageKindImage imageKind = iota
	imageKindMask
	imageKindStencilMask
)

// isMask is ImageKind::is_mask in hayro-interpret src/x_object/image.rs.
func (k imageKind) isMask() bool {
	return k != imageKindImage
}

// newImageXObject is ImageXObject::new in hayro-interpret
// src/x_object/image.rs.
func newImageXObject(stream syntax.Stream, resolveCS func(syntax.Name) (ColorSpace, bool), settings *InterpreterSettings, cache *Cache, transferFunction *ActiveTransferFunction) (*imageXObject, bool) {
	return newImageXObjectInner(stream, resolveCS, imageEnvOf(settings), cache, imageKindImage, transferFunction)
}

// newImageXObjectMask is ImageXObject::new_mask in hayro-interpret
// src/x_object/image.rs.
func newImageXObjectMask(stream syntax.Stream, env imageEnv, cache *Cache) (*imageXObject, bool) {
	return newImageXObjectInner(stream, nil, env, cache, imageKindMask, nil)
}

// newImageXObjectInner is ImageXObject::new_inner in hayro-interpret
// src/x_object/image.rs.
func newImageXObjectInner(stream syntax.Stream, resolveCS func(syntax.Name) (ColorSpace, bool), env imageEnv, cache *Cache, kind imageKind, transferFunction *ActiveTransferFunction) (*imageXObject, bool) {
	dict := stream.Dict()
	isStencilMask, ok := dict.Get[bool](syntax.KeyIM)
	if !ok {
		isStencilMask, _ = dict.Get[bool](syntax.KeyImageMask)
	}
	if isStencilMask {
		kind = imageKindStencilMask
	}
	x := &imageXObject{
		cache:            cache,
		env:              env,
		transferFunction: transferFunction,
		stream:           stream,
		kind:             kind,
	}
	if kind.isMask() {
		x.colorSpace, x.hasColorSpace = DeviceGray(), true
	} else {
		csObj, ok := dict.Get[syntax.Object](syntax.KeyCS)
		if !ok {
			csObj, ok = dict.Get[syntax.Object](syntax.KeyColorSpace)
		}
		if ok {
			x.colorSpace, x.hasColorSpace = newColorSpace(csObj, cache)
			// Against the specification, some producers name the color
			// space of an image XObject (#1311).
			if name, isName := syntax.ObjectIntoName(csObj); !x.hasColorSpace && isName && resolveCS != nil {
				x.colorSpace, x.hasColorSpace = resolveCS(name)
			}
		}
	}
	interpolate, ok := dict.Get[bool](syntax.KeyI)
	if !ok {
		interpolate, _ = dict.Get[bool](syntax.KeyInterpolate)
	}
	x.interpolate = interpolate
	width, ok := dict.Get[uint32](syntax.KeyW)
	if !ok {
		if width, ok = dict.Get[uint32](syntax.KeyWidth); !ok {
			return nil, false
		}
	}
	height, ok := dict.Get[uint32](syntax.KeyH)
	if !ok {
		if height, ok = dict.Get[uint32](syntax.KeyHeight); !ok {
			return nil, false
		}
	}
	if width == 0 || height == 0 {
		return nil, false
	}
	x.width, x.height = width, height
	return x, true
}

// draw is ImageXObject::draw in hayro-interpret src/x_object/image.rs. It
// draws the image in a transparency group that applies the alpha of the
// fill and the soft mask.
func (x *imageXObject) draw(ctx *Context, device Device) {
	if !ctx.ocgState.isVisible() {
		return
	}
	hasOC := xobjectOC(x.stream.Dict(), ctx)
	if !ctx.ocgState.isVisible() {
		if hasOC {
			ctx.ocgState.endMarkedContent()
		}
		return
	}
	width, height := float64(x.width), float64(x.height)
	ctx.saveState()
	ctx.preConcatAffine(kurbo.NewAffine([6]float64{1 / width, 0, 0, -1 / height, 0, 1}))
	transform := ctx.ctm()
	hasAlpha := x.hasMask()
	comp := ctx.compositingMut()
	softMask, blendMode := comp.softMask, comp.blendMode
	comp.softMask, comp.blendMode = nil, BlendModeNormal
	if hasAlpha {
		softMask = nil
	}
	device.PushTransparencyGroup(ctx.fillPaint().alpha, softMask, blendMode, false, false)
	// The transparency group already applies the alpha of the fill.
	ctx.fillPaintMut().alpha = 1
	var image Image
	if x.kind.isMask() {
		image = &StencilImage{paint: ctx.getPaint(false), imageXObject: x}
	} else {
		image = &RasterImage{imageXObject: x}
	}
	device.DrawImage(image, ImageDrawProps{Transform: transform, BlendMode: BlendModeNormal})
	device.PopTransparencyGroup()
	ctx.restoreState(device)
	if hasOC {
		ctx.ocgState.endMarkedContent()
	}
}

// decodedMask is ImageXObject::decoded_mask in hayro-interpret
// src/x_object/image.rs.
func (x *imageXObject) decodedMask(targetDimension [2]uint32) (decodedMask, bool) {
	if !x.kind.isMask() {
		return decodedMask{}, false
	}
	return decodeMask(x, targetDimension)
}

// decodedImage is ImageXObject::decoded_image in hayro-interpret
// src/x_object/image.rs.
func (x *imageXObject) decodedImage(targetDimension [2]uint32) (decodedImage, bool) {
	if x.kind.isMask() {
		return decodedImage{}, false
	}
	return decodeImage(x, targetDimension)
}

// hasMask is ImageXObject::has_mask in hayro-interpret
// src/x_object/image.rs.
func (x *imageXObject) hasMask() bool {
	dict := x.stream.Dict()
	return dict.ContainsKey(syntax.KeySMaskInData) || dict.ContainsKey(syntax.KeySMask) || dict.ContainsKey(syntax.KeyMask)
}

// cacheKey is the CacheKey implementation of ImageXObject in
// hayro-interpret src/x_object/image.rs.
func (x *imageXObject) cacheKey() Key128 {
	return streamCacheKey(x.stream)
}
