// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/color/mod.rs.

package interpret

import (
	"sync"

	"github.com/xo/ivy/internal/hayro/syntax"
)

// maxColorSpaceDepth is MAX_COLOR_SPACE_DEPTH in hayro-interpret
// src/color/mod.rs.
const maxColorSpaceDepth = 32

// ColorComponents is ColorComponents in hayro-interpret src/color/mod.rs.
// It holds the components of a color.
type ColorComponents = []float32

// u8Lookup is U8Lookup in hayro-interpret src/color/mod.rs. It is a table
// for the 256 values of a byte, made once on the first use.
//
// Port: upstream shares the table through an Arc. Go keeps the table in
// the value of the color space, which is shared through a pointer.
type u8Lookup[T any] struct {
	once  sync.Once
	table *[256]T
}

// getOrInitWith is U8Lookup::get_or_init_with in hayro-interpret
// src/color/mod.rs. init receives the 256 values of a byte. A nil table is
// a failure, and it is kept.
func (l *u8Lookup[T]) getOrInitWith(init func(input *[256]uint8) *[256]T) *[256]T {
	l.once.Do(func() {
		var input [256]uint8
		for i := range input {
			input[i] = uint8(i)
		}
		l.table = init(&input)
	})
	return l.table
}

// rgbLookupGetOrInit is U8Lookup::<[u8; 3]>::get_or_init in hayro-interpret
// src/color/mod.rs. convert converts the 256 values into colors of RGB.
func rgbLookupGetOrInit(l *u8Lookup[[3]uint8], convert func(input, output []uint8) bool) *[256][3]uint8 {
	return l.getOrInitWith(func(input *[256]uint8) *[256][3]uint8 {
		output := make([]uint8, 256*3)
		if !convert(input[:], output) {
			return nil
		}
		var table [256][3]uint8
		for i := range table {
			copy(table[i][:], output[i*3:])
		}
		return &table
	})
}

// AlphaColor is AlphaColor in hayro-interpret src/color/mod.rs. It is a
// color of RGB with an alpha channel, with components from 0 to 1.
type AlphaColor struct {
	components [4]float32
}

// The colors of AlphaColor in hayro-interpret src/color/mod.rs.
var (
	// AlphaColorBlack is AlphaColor::BLACK.
	AlphaColorBlack = NewAlphaColor([4]float32{0, 0, 0, 1})
	// AlphaColorTransparent is AlphaColor::TRANSPARENT.
	AlphaColorTransparent = NewAlphaColor([4]float32{0, 0, 0, 0})
	// AlphaColorWhite is AlphaColor::WHITE.
	AlphaColorWhite = NewAlphaColor([4]float32{1, 1, 1, 1})
)

// NewAlphaColor is AlphaColor::new in hayro-interpret src/color/mod.rs.
func NewAlphaColor(components [4]float32) AlphaColor {
	return AlphaColor{components: components}
}

// AlphaColorFromRGB8 is AlphaColor::from_rgb8 in hayro-interpret
// src/color/mod.rs.
func AlphaColorFromRGB8(r, g, b uint8) AlphaColor {
	return NewAlphaColor([4]float32{u8ToF32(r), u8ToF32(g), u8ToF32(b), 1})
}

// AlphaColorFromRGBA8 is AlphaColor::from_rgba8 in hayro-interpret
// src/color/mod.rs.
func AlphaColorFromRGBA8(r, g, b, a uint8) AlphaColor {
	return NewAlphaColor([4]float32{u8ToF32(r), u8ToF32(g), u8ToF32(b), u8ToF32(a)})
}

// Premultiplied is AlphaColor::premultiplied in hayro-interpret
// src/color/mod.rs. It returns the color with each channel multiplied by
// the alpha.
func (c AlphaColor) Premultiplied() [4]float32 {
	a := c.components[3]
	return [4]float32{c.components[0] * a, c.components[1] * a, c.components[2] * a, a}
}

// ToRGBA8 is AlphaColor::to_rgba8 in hayro-interpret src/color/mod.rs.
func (c AlphaColor) ToRGBA8() [4]uint8 {
	return [4]uint8{
		asU8(c.components[0]*255 + 0.5),
		asU8(c.components[1]*255 + 0.5),
		asU8(c.components[2]*255 + 0.5),
		asU8(c.components[3]*255 + 0.5),
	}
}

// Components is AlphaColor::components in hayro-interpret
// src/color/mod.rs.
func (c AlphaColor) Components() [4]float32 {
	return c.components
}

// u8ToF32 is u8_to_f32 in hayro-interpret src/color/mod.rs.
func u8ToF32(x uint8) float32 {
	return float32(x) * (1.0 / 255.0)
}

// colorSpaceType is ColorSpaceType in hayro-interpret src/color/mod.rs,
// with the trait ToRgb of the same file. Each color space implements it.
//
// Port: upstream is an enum. Go uses an interface, and a type switch where
// upstream matches a case.
type colorSpaceType interface {
	// convert is ToRgb::convert. It converts the components of 8 bits in
	// input into RGB in output.
	convert(input, output []uint8) bool
	// convertInPlace is ToRgb::convert_in_place. It converts the components
	// into RGB in place. The default of upstream fails.
	convertInPlace(input []uint8) bool
	// isNone is ToRgb::is_none. The default of upstream is false.
	isNone() bool
}

// lumaConverter is the trait ToLuma in hayro-interpret src/color/mod.rs.
type lumaConverter interface {
	// toLuma converts the components into gray in place.
	toLuma(input []uint8) bool
}

// newColorSpaceType is ColorSpaceType::new in hayro-interpret
// src/color/mod.rs.
func newColorSpaceType(object syntax.Object, cache *Cache, depth uint8) (colorSpaceType, bool) {
	if depth >= maxColorSpaceDepth {
		return nil, false
	}
	return newColorSpaceTypeInner(object, cache, depth)
}

// newColorSpaceTypeInner is ColorSpaceType::new_inner in hayro-interpret
// src/color/mod.rs.
func newColorSpaceTypeInner(object syntax.Object, cache *Cache, depth uint8) (colorSpaceType, bool) {
	switch o := object.(type) {
	case syntax.Name:
		return colorSpaceTypeFromName(o)
	case syntax.Array:
		iter := o.FlexIter()
		name, ok := iter.Next[syntax.Name]()
		if !ok {
			return nil, false
		}
		switch name {
		case syntax.KeyICCBased:
			iccStream, ok := iter.Next[syntax.Stream]()
			if !ok {
				return nil, false
			}
			dict := iccStream.Dict()
			numComponents, ok := dict.Get[int](syntax.KeyN)
			if !ok {
				return nil, false
			}
			return cacheGetOrInsertWith(cache, streamCacheKey(iccStream), func() (colorSpaceType, bool) {
				decoded, err := iccStream.DecodedData()
				if err != nil {
					return nil, false
				}
				if icc, ok := newICCProfile(decoded, numComponents); ok {
					// Upstream notes that the output is sRGB for SVG and
					// PNG, so a profile of sRGB is DeviceRGB.
					if icc.isSRGB() {
						return deviceRGBSpace, true
					}
					return icc, true
				}
				if alt, ok := dict.Get[syntax.Object](syntax.KeyAlternate); ok {
					if cs, ok := newColorSpaceType(alt, cache, depth+1); ok {
						return cs, true
					}
				}
				if n, ok := dict.Get[uint8](syntax.KeyN); ok {
					switch n {
					case 1:
						return deviceGraySpace, true
					case 3:
						return deviceRGBSpace, true
					case 4:
						return deviceCMYKSpace, true
					}
				}
				return nil, false
			})
		case syntax.KeyCalCMYK:
			return deviceCMYKSpace, true
		case syntax.KeyCalGray:
			calDict, ok := iter.Next[syntax.Dict]()
			if !ok {
				return nil, false
			}
			return newCalGray(calDict), true
		case syntax.KeyCalRGB:
			calDict, ok := iter.Next[syntax.Dict]()
			if !ok {
				return nil, false
			}
			return newCalRGB(calDict), true
		case syntax.KeyDeviceRGB, syntax.KeyRGB:
			return deviceRGBSpace, true
		case syntax.KeyDeviceGray, syntax.KeyG:
			return deviceGraySpace, true
		case syntax.KeyDeviceCMYK, syntax.KeyCMYK:
			return deviceCMYKSpace, true
		case syntax.KeyLab:
			labDict, ok := iter.Next[syntax.Dict]()
			if !ok {
				return nil, false
			}
			l, ok := newLab(labDict)
			if !ok {
				return nil, false
			}
			return l, true
		case syntax.KeyIndexed, syntax.KeyI:
			idx, ok := newIndexed(o, cache, depth+1)
			if !ok {
				return nil, false
			}
			return idx, true
		case syntax.KeySeparation:
			sep, ok := newSeparation(o, cache, depth+1)
			if !ok {
				return nil, false
			}
			return sep, true
		case syntax.KeyDeviceN:
			dn, ok := newDeviceN(o, cache, depth+1)
			if !ok {
				return nil, false
			}
			return dn, true
		case syntax.KeyPattern:
			cs := DeviceRGB()
			if obj, ok := iter.Next[syntax.Object](); ok {
				if nested, ok := newColorSpaceNested(obj, cache, depth+1); ok {
					cs = nested
				}
			}
			return &patternSpace{cs: cs}, true
		}
	}
	return nil, false
}

// colorSpaceTypeFromName is ColorSpaceType::new_from_name in
// hayro-interpret src/color/mod.rs.
func colorSpaceTypeFromName(name syntax.Name) (colorSpaceType, bool) {
	switch name {
	case syntax.KeyDeviceRGB, syntax.KeyRGB:
		return deviceRGBSpace, true
	case syntax.KeyDeviceGray, syntax.KeyG:
		return deviceGraySpace, true
	case syntax.KeyDeviceCMYK, syntax.KeyCMYK, syntax.KeyCalCMYK:
		return deviceCMYKSpace, true
	case syntax.KeyPattern:
		return &patternSpace{cs: DeviceRGB()}, true
	}
	return nil, false
}

// ColorSpace is ColorSpace in hayro-interpret src/color/mod.rs. It is a
// color space of PDF.
//
// Port: upstream shares the color space through an Arc. Go copies the
// value, which holds an interface to the shared state.
type ColorSpace struct {
	t colorSpaceType
}

// newColorSpace is ColorSpace::new in hayro-interpret src/color/mod.rs.
func newColorSpace(object syntax.Object, cache *Cache) (ColorSpace, bool) {
	return newColorSpaceNested(object, cache, 0)
}

// newColorSpaceNested is ColorSpace::new_nested in hayro-interpret
// src/color/mod.rs.
func newColorSpaceNested(object syntax.Object, cache *Cache, depth uint8) (ColorSpace, bool) {
	t, ok := newColorSpaceType(object, cache, depth)
	if !ok {
		return ColorSpace{}, false
	}
	return ColorSpace{t: t}, true
}

// colorSpaceFromName is ColorSpace::new_from_name in hayro-interpret
// src/color/mod.rs.
func colorSpaceFromName(name syntax.Name) (ColorSpace, bool) {
	t, ok := colorSpaceTypeFromName(name)
	if !ok {
		return ColorSpace{}, false
	}
	return ColorSpace{t: t}, true
}

// DeviceGray is ColorSpace::device_gray in hayro-interpret
// src/color/mod.rs.
func DeviceGray() ColorSpace {
	return ColorSpace{t: deviceGraySpace}
}

// DeviceRGB is ColorSpace::device_rgb in hayro-interpret src/color/mod.rs.
func DeviceRGB() ColorSpace {
	return ColorSpace{t: deviceRGBSpace}
}

// DeviceCMYK is ColorSpace::device_cmyk in hayro-interpret
// src/color/mod.rs.
func DeviceCMYK() ColorSpace {
	return ColorSpace{t: deviceCMYKSpace}
}

// patternColorSpace is ColorSpace::pattern in hayro-interpret
// src/color/mod.rs.
func patternColorSpace() ColorSpace {
	return ColorSpace{t: &patternSpace{cs: DeviceGray()}}
}

// typ returns the type of the color space.
//
// Port: the zero ColorSpace of Go has no type. It acts as DeviceGray, so
// that a zero value cannot make a panic.
func (c ColorSpace) typ() colorSpaceType {
	if c.t == nil {
		return deviceGraySpace
	}
	return c.t
}

// patternCS is ColorSpace::pattern_cs in hayro-interpret src/color/mod.rs.
func (c ColorSpace) patternCS() (ColorSpace, bool) {
	if p, ok := c.typ().(*patternSpace); ok {
		return p.colorSpace(), true
	}
	return ColorSpace{}, false
}

// isPattern is ColorSpace::is_pattern in hayro-interpret src/color/mod.rs.
func (c ColorSpace) isPattern() bool {
	_, ok := c.typ().(*patternSpace)
	return ok
}

// isIndexed is ColorSpace::is_indexed in hayro-interpret src/color/mod.rs.
func (c ColorSpace) isIndexed() bool {
	_, ok := c.typ().(*indexedSpace)
	return ok
}

// isDeviceGrayOrRGB is ColorSpace::is_device_gray_or_rgb in
// hayro-interpret src/color/mod.rs.
func (c ColorSpace) isDeviceGrayOrRGB() bool {
	switch c.typ().(type) {
	case *deviceGray, *deviceRGB:
		return true
	}
	return false
}

// indexedHival is ColorSpace::indexed_hival in hayro-interpret
// src/color/mod.rs.
func (c ColorSpace) indexedHival() (uint8, bool) {
	switch t := c.typ().(type) {
	case *indexedSpace:
		return t.hival, true
	case *patternSpace:
		return t.colorSpace().indexedHival()
	}
	return 0, false
}

// repeatRange returns n ranges from 0 to 1.
func repeatRange(n int, r [2]float32) [][2]float32 {
	out := make([][2]float32, n)
	for i := range out {
		out[i] = r
	}
	return out
}

// defaultDecodeArr is ColorSpace::default_decode_arr in hayro-interpret
// src/color/mod.rs.
func (c ColorSpace) defaultDecodeArr(n float32) [][2]float32 {
	unit := [2]float32{0, 1}
	switch t := c.typ().(type) {
	case *deviceCMYK:
		return repeatRange(4, unit)
	case *deviceRGB, *calRGB:
		return repeatRange(3, unit)
	case *iccProfile:
		return repeatRange(t.numberComponents(), unit)
	case *labSpace:
		return [][2]float32{{0, 100}, {t.rng[0], t.rng[1]}, {t.rng[2], t.rng[3]}}
	case *indexedSpace:
		return [][2]float32{{0, pow2f32(n) - 1}}
	case *deviceNSpace:
		return repeatRange(int(t.numComponents), unit)
	}
	// DeviceGray, CalGray, Separation and Pattern.
	return repeatRange(1, unit)
}

// invertedDefaultDecodeArr is ColorSpace::inverted_default_decode_arr in
// hayro-interpret src/color/mod.rs.
func (c ColorSpace) invertedDefaultDecodeArr(n float32) [][2]float32 {
	arr := c.defaultDecodeArr(n)
	for i, r := range arr {
		arr[i] = [2]float32{r[1], r[0]}
	}
	return arr
}

// componentRanges is ColorSpace::component_ranges in hayro-interpret
// src/color/mod.rs.
func (c ColorSpace) componentRanges() [][2]float32 {
	unit := [2]float32{0, 1}
	switch t := c.typ().(type) {
	case *deviceCMYK:
		return repeatRange(4, unit)
	case *deviceRGB, *calRGB:
		return repeatRange(3, unit)
	case *iccProfile:
		return repeatRange(t.numberComponents(), unit)
	case *labSpace:
		return [][2]float32{{0, 100}, {-128, 127}, {-128, 127}}
	case *indexedSpace:
		return [][2]float32{{0, float32(t.hival)}}
	case *patternSpace:
		return t.colorSpace().componentRanges()
	case *deviceNSpace:
		return repeatRange(int(t.numComponents), unit)
	}
	// DeviceGray, CalGray and Separation.
	return repeatRange(1, unit)
}

// convertValues is ColorSpace::convert_values in hayro-interpret
// src/color/mod.rs.
func (c ColorSpace) convertValues(input []float32, output []uint8) bool {
	return c.convert(c.encodeValues(input), output)
}

// encodeValues is ColorSpace::encode_values in hayro-interpret
// src/color/mod.rs. It converts the components into bytes, and it fills
// missing components from the initial color.
func (c ColorSpace) encodeValues(input []float32) []uint8 {
	numComponents := int(c.NumComponents())
	values := make([]float32, 0, numComponents)
	values = append(values, input[:min(len(input), numComponents)]...)
	if len(values) < numComponents {
		initial := c.initialColor()
		if len(values) < len(initial) {
			values = append(values, initial[len(values):]...)
		}
	}
	if hival, ok := c.indexedHival(); ok {
		out := make([]uint8, len(values))
		for i, v := range values {
			out[i] = asU8(rustClamp(v+0.5, 0, float32(hival)))
		}
		return out
	}
	var ranges [][2]float32
	if icc, ok := c.typ().(*iccProfile); ok && icc.isLab() {
		ranges = [][2]float32{{0, 100}, {-128, 127}, {-128, 127}}
	} else {
		ranges = c.componentRanges()
	}
	return encodeComponents(values, ranges)
}

// initialColor is ColorSpace::initial_color in hayro-interpret
// src/color/mod.rs.
func (c ColorSpace) initialColor() ColorComponents {
	switch t := c.typ().(type) {
	case *deviceCMYK:
		return ColorComponents{0, 0, 0, 1}
	case *deviceRGB, *calRGB, *labSpace:
		return ColorComponents{0, 0, 0}
	case *iccProfile:
		switch t.numberComponents() {
		case 3:
			return ColorComponents{0, 0, 0}
		case 4:
			return ColorComponents{0, 0, 0, 1}
		}
		return ColorComponents{0}
	case *separationSpace:
		return ColorComponents{1}
	case *patternSpace:
		return t.initialColor()
	case *deviceNSpace:
		out := make(ColorComponents, t.numComponents)
		for i := range out {
			out[i] = 1
		}
		return out
	}
	// DeviceGray, CalGray and Indexed.
	return ColorComponents{0}
}

// NumComponents is ColorSpace::num_components in hayro-interpret
// src/color/mod.rs. It returns the number of components of the color
// space.
func (c ColorSpace) NumComponents() uint8 {
	switch t := c.typ().(type) {
	case *deviceCMYK:
		return 4
	case *deviceRGB, *calRGB, *labSpace:
		return 3
	case *iccProfile:
		return uint8(t.numberComponents())
	case *patternSpace:
		return t.numComponents()
	case *deviceNSpace:
		return t.numComponents
	}
	// DeviceGray, CalGray, Indexed and Separation.
	return 1
}

// ToRGBA is ColorSpace::to_rgba in hayro-interpret src/color/mod.rs. It
// turns the components and the opacity into a color of RGBA.
func (c ColorSpace) ToRGBA(comps []float32, opacity float32) AlphaColor {
	alpha := f32ToU8(opacity)
	at := func(i int) uint8 {
		if i < len(comps) {
			return f32ToU8(comps[i])
		}
		return 0
	}
	switch t := c.typ().(type) {
	case *deviceGray:
		gray := at(0)
		return AlphaColorFromRGBA8(gray, gray, gray, alpha)
	case *deviceRGB:
		return AlphaColorFromRGBA8(at(0), at(1), at(2), alpha)
	case *deviceCMYK:
		if len(comps) == 4 {
			input := []uint8{at(0), at(1), at(2), at(3)}
			output := make([]uint8, 3)
			if t.convert(input, output) {
				return AlphaColorFromRGBA8(output[0], output[1], output[2], alpha)
			}
			return AlphaColorBlack
		}
	}
	if col, ok := c.toAlphaColor(comps, opacity); ok {
		return col
	}
	return AlphaColorBlack
}

// toAlphaColor is ColorSpace::to_alpha_color in hayro-interpret
// src/color/mod.rs.
func (c ColorSpace) toAlphaColor(input []float32, opacity float32) (AlphaColor, bool) {
	output := make([]uint8, 3)
	if !c.convertValues(input, output) {
		return AlphaColor{}, false
	}
	if c.isNone() {
		opacity = 0
	}
	return AlphaColorFromRGBA8(output[0], output[1], output[2], asU8(opacity*255+0.5)), true
}

// convert is the ToRgb implementation of ColorSpace in hayro-interpret
// src/color/mod.rs.
func (c ColorSpace) convert(input, output []uint8) bool {
	return c.typ().convert(input, output)
}

// convertInPlace is the ToRgb implementation of ColorSpace in
// hayro-interpret src/color/mod.rs.
func (c ColorSpace) convertInPlace(input []uint8) bool {
	return c.typ().convertInPlace(input)
}

// isNone is the ToRgb implementation of ColorSpace in hayro-interpret
// src/color/mod.rs.
func (c ColorSpace) isNone() bool {
	return c.typ().isNone()
}

// toLuma is the ToLuma implementation of ColorSpace in hayro-interpret
// src/color/mod.rs. The color spaces of RGB, CMYK, Lab, Separation and
// DeviceN fail.
func (c ColorSpace) toLuma(input []uint8) bool {
	if l, ok := c.typ().(lumaConverter); ok {
		return l.toLuma(input)
	}
	return false
}

// f32ToU8 is f32_to_u8 in hayro-interpret src/color/mod.rs.
func f32ToU8(val float32) uint8 {
	return asU8(val*255 + 0.5)
}

// lerpColor is lerp_color in hayro-interpret src/color/mod.rs.
func lerpColor(c0, c1 [4]float32, t float32) [4]float32 {
	var out [4]float32
	for i := range out {
		out[i] = c0[i] + (c1[i]-c0[i])*t
	}
	return out
}

// colorError is color_error in hayro-interpret src/color/mod.rs. It
// returns the largest difference of a channel.
func colorError(c0, c1 [4]float32) float32 {
	var e float32
	for i := range c0 {
		d := c0[i] - c1[i]
		if d < 0 {
			d = -d
		}
		e = rustMax(e, d)
	}
	return e
}

// Color is Color in hayro-interpret src/color/mod.rs. It is a color with
// its color space and its opacity.
type Color struct {
	colorSpace ColorSpace
	components ColorComponents
	opacity    float32
}

// newColor is Color::new in hayro-interpret src/color/mod.rs.
func newColor(colorSpace ColorSpace, components ColorComponents, opacity float32) Color {
	return Color{colorSpace: colorSpace, components: components, opacity: opacity}
}

// ColorSpace is Color::color_space in hayro-interpret src/color/mod.rs. It
// returns the color space of the color, which is DeviceRGB after a transfer
// function.
func (c Color) ColorSpace() ColorSpace {
	return c.colorSpace
}

// Components is Color::components in hayro-interpret src/color/mod.rs. It
// returns the components of the color as the PDF file gives them. The
// caller must not change them.
func (c Color) Components() []float32 {
	return c.components
}

// ToRGBA is Color::to_rgba in hayro-interpret src/color/mod.rs.
func (c Color) ToRGBA() AlphaColor {
	return c.colorSpace.ToRGBA(c.components, c.opacity)
}

// ColorFromRGBA is Color::from_rgba in hayro-interpret src/color/mod.rs.
func ColorFromRGBA(rgba AlphaColor) Color {
	comps := rgba.Components()
	return Color{colorSpace: DeviceRGB(), components: ColorComponents{comps[0], comps[1], comps[2]}, opacity: comps[3]}
}

// CacheKey is the CacheKey implementation of Paint::Color in
// hayro-interpret src/types.rs. It hashes the color as RGBA8, as upstream
// does.
func (c Color) CacheKey() Key128 {
	rgba := c.ToRGBA().ToRGBA8()
	return newKeyHasher().str("color").bytes(rgba[:]).sum()
}

// encodeComponents is encode_components in hayro-interpret
// src/color/mod.rs.
func encodeComponents(input []float32, ranges [][2]float32) []uint8 {
	out := make([]uint8, len(input))
	if len(ranges) == 0 {
		// Port: upstream divides by the length of ranges, which is never 0.
		return out
	}
	for i, v := range input {
		r := ranges[i%len(ranges)]
		out[i] = asU8((v-r[0])/(r[1]-r[0])*255 + 0.5)
	}
	return out
}

// pow2f32 is 2.0_f32.powf(n).
func pow2f32(n float32) float32 {
	return powf32(2, n)
}
