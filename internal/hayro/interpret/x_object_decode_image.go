// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/x_object/decode/image.rs.

package interpret

import (
	"slices"

	"github.com/xo/ivy/internal/hayro/syntax"
)

// decodedImage is DecodedImage in hayro-interpret
// src/x_object/decode/image.rs.
type decodedImage struct {
	image ImageData
	alpha *LumaData
}

// decodeImage is decode_image in hayro-interpret
// src/x_object/decode/image.rs.
func decodeImage(obj *imageXObject, targetDimension [2]uint32) (decodedImage, bool) {
	d, ok := newImageDecoder(obj, targetDimension)
	if !ok {
		return decodedImage{}, false
	}
	return d.decode()
}

// imageDecoder is ImageDecoder in hayro-interpret
// src/x_object/decode/image.rs.
type imageDecoder struct {
	obj                 *imageXObject
	ctx                 *decodeCtx
	targetDimension     [2]uint32
	colorKeyMask        []uint16
	hasColorKeyMask     bool
	decodedColorKeyMask *LumaData
}

// newImageDecoder is ImageDecoder::new in hayro-interpret
// src/x_object/decode/image.rs.
func newImageDecoder(obj *imageXObject, targetDimension [2]uint32) (*imageDecoder, bool) {
	ctx, ok := decodeContext(obj, targetDimension)
	if !ok {
		return nil, false
	}
	mask, hasMask := obj.stream.Dict().Get[[]uint16](syntax.KeyMask)
	return &imageDecoder{obj: obj, ctx: ctx, targetDimension: targetDimension, colorKeyMask: mask, hasColorKeyMask: hasMask}, true
}

// decode is ImageDecoder::decode in hayro-interpret
// src/x_object/decode/image.rs.
func (d *imageDecoder) decode() (decodedImage, bool) {
	image, ok := d.decodeImage()
	if !ok {
		return decodedImage{}, false
	}
	alpha := d.decodeAlpha(&image)
	return decodedImage{image: image, alpha: alpha}, true
}

// decodeImage is ImageDecoder::decode_image in hayro-interpret
// src/x_object/decode/image.rs. It decodes the components, and returns the
// image as gray when the color space allows it, or as RGB.
func (d *imageDecoder) decodeImage() (ImageData, bool) {
	cs := d.ctx.colorSpace
	numComponents := int(cs.NumComponents())
	// Upstream avoids a panic of the method chunks.
	if numComponents == 0 {
		return ImageData{}, false
	}
	componentRanges := cs.componentRanges()
	defaultDecode := cs.defaultDecodeArr(float32(d.ctx.bitsPerComponent))
	invertedDefaultDecode := cs.invertedDefaultDecodeArr(float32(d.ctx.bitsPerComponent))
	invertedComponentRanges := make([][2]float32, len(componentRanges))
	for i, r := range componentRanges {
		invertedComponentRanges[i] = [2]float32{r[1], r[0]}
	}
	isIndexed := cs.isIndexed()
	bpc8 := d.ctx.bitsPerComponent == 8
	direct, invert := false, false
	switch {
	case bpc8 && (equalRanges(d.ctx.decodeArr, componentRanges) || isIndexed && equalRanges(d.ctx.decodeArr, defaultDecode)):
		direct = true
	case bpc8 && (equalRanges(d.ctx.decodeArr, invertedComponentRanges) || isIndexed && equalRanges(d.ctx.decodeArr, invertedDefaultDecode)):
		direct, invert = true, true
	}

	var components []uint8
	if direct {
		// Port: the decoded data can be the data of the file itself, so
		// the port copies it before it changes it, as Cow::to_mut of
		// upstream does.
		data := slices.Clone(d.ctx.decoded.Data)
		if !fixImageLength(&data, d.ctx.width, &d.ctx.height, 0, numComponents) {
			return ImageData{}, false
		}
		d.ctx.decoded.Data = data
		if d.hasColorKeyMask {
			d.decodedColorKeyMask = d.decodeColorKeyMask()
		}
		if invert {
			for i, v := range data {
				data[i] = 255 - v
			}
		}
		components = data
		d.ctx.decoded.Data = nil
	} else {
		var ok bool
		components, ok = decodeU8Samples(d.ctx.decoded.Data, d.ctx.width, d.ctx.height, cs, d.ctx.bitsPerComponent, d.ctx.decodeArr)
		if !ok {
			return ImageData{}, false
		}
		if !fixImageLength(&components, d.ctx.width, &d.ctx.height, 0, numComponents) {
			return ImageData{}, false
		}
	}

	tf := d.obj.transferFunction
	if (tf == nil || tf.Single != nil) && cs.toLuma(components) {
		if tf != nil {
			tf.Single.ApplyTo(components)
		}
		return ImageData{Luma: &LumaData{
			Data:         components,
			Width:        d.ctx.width,
			Height:       d.ctx.height,
			Interpolate:  d.obj.interpolate,
			ScaleFactors: d.ctx.scaleFactors,
		}}, true
	}
	rgb, ok := d.convertToRGB(components)
	if !ok {
		return ImageData{}, false
	}
	d.applyTransferFunction(rgb)
	return ImageData{RGB: rgb}, true
}

// convertToRGB is ImageDecoder::convert_to_rgb in hayro-interpret
// src/x_object/decode/image.rs.
func (d *imageDecoder) convertToRGB(decoded []uint8) (*RGBData, bool) {
	if !d.ctx.colorSpace.convertInPlace(decoded) {
		output := make([]uint8, int(d.ctx.width)*int(d.ctx.height)*3)
		if !d.ctx.colorSpace.convert(decoded, output) {
			return nil, false
		}
		decoded = output
	}
	return &RGBData{
		Data:         decoded,
		Width:        d.ctx.width,
		Height:       d.ctx.height,
		Interpolate:  d.obj.interpolate,
		ScaleFactors: d.ctx.scaleFactors,
	}, true
}

// applyTransferFunction is ImageDecoder::apply_transfer_function in
// hayro-interpret src/x_object/decode/image.rs.
func (d *imageDecoder) applyTransferFunction(rgb *RGBData) {
	if d.obj.transferFunction != nil {
		d.obj.transferFunction.applyTo(rgb.Data)
	}
}

// decodeAlpha is ImageDecoder::decode_alpha in hayro-interpret
// src/x_object/decode/image.rs. It returns the alpha channel from a matte,
// from the data of the filter, from SMask or Mask, or from a color key
// mask.
func (d *imageDecoder) decodeAlpha(image *ImageData) *LumaData {
	if alpha, matteRGB, ok := d.resolveMatte(); ok && alpha.Width == d.ctx.width && alpha.Height == d.ctx.height {
		unpremultiply(image, alpha.Data, matteRGB)
		return &alpha
	}
	// An invalid alpha channel is dropped, so that the image itself still
	// draws (PDFJS-19611).
	dict := d.obj.stream.Dict()
	if v, ok := dict.Get[uint8](syntax.KeySMaskInData); ok && v == 1 {
		if d.ctx.decoded.ImageData == nil || d.ctx.decoded.ImageData.Alpha == nil {
			return nil
		}
		data := slices.Clone(d.ctx.decoded.ImageData.Alpha)
		d.ctx.decoded.ImageData.Alpha = nil
		if !fixImageLength(&data, d.ctx.width, &d.ctx.height, 0, 1) {
			return nil
		}
		return &LumaData{
			Data:         data,
			Width:        d.ctx.width,
			Height:       d.ctx.height,
			Interpolate:  d.obj.interpolate,
			ScaleFactors: d.ctx.scaleFactors,
		}
	}
	// SMask takes precedence over Mask.
	sMask, ok := dict.Get[syntax.Stream](syntax.KeySMask)
	if !ok {
		sMask, ok = dict.Get[syntax.Stream](syntax.KeyMask)
	}
	if ok {
		obj, ok := newImageXObjectMask(sMask, d.obj.env, d.obj.cache)
		if !ok {
			return nil
		}
		decoded, ok := decodeMask(obj, d.targetDimension)
		if !ok {
			return nil
		}
		return &decoded.luma
	}
	if d.hasColorKeyMask {
		if d.decodedColorKeyMask != nil {
			m := d.decodedColorKeyMask
			d.decodedColorKeyMask = nil
			return m
		}
		return d.decodeColorKeyMask()
	}
	return nil
}

// resolveMatte is ImageDecoder::resolve_matte in hayro-interpret
// src/x_object/decode/image.rs. It returns the soft mask with its matte
// color in RGB.
func (d *imageDecoder) resolveMatte() (LumaData, [3]uint8, bool) {
	sMask, ok := d.obj.stream.Dict().Get[syntax.Stream](syntax.KeySMask)
	if !ok {
		return LumaData{}, [3]uint8{}, false
	}
	matte, ok := sMask.Dict().Get[[]float32](syntax.KeyMatte)
	if !ok || len(matte) != int(d.ctx.colorSpace.NumComponents()) {
		return LumaData{}, [3]uint8{}, false
	}
	// The matte applies in RGB rather than in the color space of the
	// image.
	var matteRGB [3]uint8
	d.ctx.colorSpace.convertValues(matte, matteRGB[:])
	maskObj, ok := newImageXObjectMask(sMask, d.obj.env, d.obj.cache)
	if !ok {
		return LumaData{}, [3]uint8{}, false
	}
	decoded, ok := decodeMask(maskObj, d.targetDimension)
	if !ok {
		return LumaData{}, [3]uint8{}, false
	}
	return decoded.luma, matteRGB, true
}

// decodeColorKeyMask is ImageDecoder::decode_color_key_mask in
// hayro-interpret src/x_object/decode/image.rs. A pixel with a component
// outside its range of the mask is opaque.
func (d *imageDecoder) decodeColorKeyMask() *LumaData {
	numComponents := int(d.ctx.colorSpace.NumComponents())
	components, ok := unpackSamples(d.ctx.decoded.Data, d.ctx.width, d.ctx.height, numComponents, d.ctx.bitsPerComponent)
	if !ok {
		return nil
	}
	data := make([]uint8, 0, int(d.ctx.width)*int(d.ctx.height))
	for i := 0; i+numComponents <= len(components); i += numComponents {
		var value uint8
		for j, c := range components[i : i+numComponents] {
			if 2*j+1 >= len(d.colorKeyMask) {
				break
			}
			if c > d.colorKeyMask[2*j+1] || c < d.colorKeyMask[2*j] {
				value = 255
			}
		}
		data = append(data, value)
	}
	height := d.ctx.height
	if !fixImageLength(&data, d.ctx.width, &height, 0, 1) {
		return nil
	}
	return &LumaData{
		Data:         data,
		Width:        d.ctx.width,
		Height:       height,
		Interpolate:  d.obj.interpolate,
		ScaleFactors: d.ctx.scaleFactors,
	}
}

// unpremultiply is unpremultiply in hayro-interpret
// src/x_object/decode/image.rs. It removes the matte color from the
// colors.
func unpremultiply(image *ImageData, alpha []uint8, matteRGB [3]uint8) {
	switch {
	case image.RGB != nil:
		data := image.RGB.Data
		for p := 0; p*3+3 <= len(data) && p < len(alpha); p++ {
			a := alpha[p]
			if a == 0 {
				continue
			}
			invAlpha := 255 / float32(a)
			for c := range 3 {
				m := float32(matteRGB[c])
				data[p*3+c] = asU8(m + (float32(data[p*3+c])-m)*invAlpha)
			}
		}
	case image.Luma != nil:
		m := float32(matteRGB[0])
		data := image.Luma.Data
		for i := 0; i < len(data) && i < len(alpha); i++ {
			a := alpha[i]
			if a == 0 {
				continue
			}
			invAlpha := 255 / float32(a)
			data[i] = asU8(m + (float32(data[i])-m)*invAlpha)
		}
	}
}
