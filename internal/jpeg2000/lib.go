// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// Package jpeg2000 decodes JPEG 2000 images: raw codestreams (.j2c) and
// images in the JP2 container format. It is a port of the crate
// hayro-jpeg2000 of hayro (D14, D16).
//
// The decoder supports the vast majority of features of the JPEG 2000 core
// coding system (ISO/IEC 15444-1) and some color spaces of the extensions
// (ISO/IEC 15444-2). It yields the decoded components of the image, which a
// program can read directly, or pack into an image of 8 bits with a
// grayscale, RGB, CMYK or ICC-based color space.
//
// A program opens an image with New, and decodes it with Image.Decode:
//
//	img, err := jpeg2000.New(data, nil)
//	if err != nil {
//		return err
//	}
//	decoded, err := img.Decode(nil)
//	if err != nil {
//		return err
//	}
//	bitmap, err := decoded.DataU8()
//
// Every error is a *DecodeError, which wraps ivy.ErrFormat,
// ivy.ErrUnsupported, ivy.ErrLimit or context.Canceled.
package jpeg2000

// This file ports hayro-jpeg2000 src/lib.rs.

import (
	_ "embed"
	"fmt"
	"runtime"
	"runtime/debug"
	"slices"

	"github.com/xo/ivy"
)

// jp2Magic is JP2_MAGIC: the start of a JP2 file.
var jp2Magic = []byte("\x00\x00\x00\x0C\x6A\x50\x20\x20")

// codestreamMagic is CODESTREAM_MAGIC: the start of a raw codestream.
var codestreamMagic = []byte("\xFF\x4F\xFF\x51")

// The ICC profiles of upstream, under CC0 1.0 Universal, as
// assets/README.md says.
var (
	//go:embed assets/ProPhoto-v2-micro.icc
	proPhotoProfile []byte
	//go:embed assets/LAB.icc
	labProfile []byte
)

// Resolution is the size of an image in pixels.
//
// Port: upstream uses the tuple (u32, u32).
type Resolution struct {
	Width  uint32
	Height uint32
}

// DecodeSettings is DecodeSettings in hayro-jpeg2000 src/lib.rs. It holds
// the settings of the decoding.
//
// Port: the zero value does not resolve the indices of a palette, where the
// Default of upstream does. DefaultDecodeSettings returns the default of
// upstream. A nil *DecodeSettings is the default.
type DecodeSettings struct {
	// ResolvePaletteIndices resolves the indices of a palette to colors,
	// instead of returning them as grayscale.
	ResolvePaletteIndices bool
	// Strict rejects malformed input, instead of decoding it leniently.
	Strict bool
	// TargetResolution is a hint for the resolution at which to decode the
	// image. A nil TargetResolution decodes at full resolution.
	TargetResolution *Resolution
	// Cancel cancels the decoding. A nil Cancel never cancels it.
	Cancel *CancelHandle

	// Limits bound the work on the image: MaxPixels bounds the size of the
	// image, MaxMemory bounds each buffer and the memory of all components,
	// and MaxObjects bounds the number of tiles and of precincts. A field
	// that is zero or less takes the default of ivy.
	//
	// Port: upstream has no limits.
	Limits ivy.Limits
}

// DefaultDecodeSettings is Default for DecodeSettings in hayro-jpeg2000
// src/lib.rs.
func DefaultDecodeSettings() DecodeSettings {
	return DecodeSettings{ResolvePaletteIndices: true}
}

// Image is Image in hayro-jpeg2000 src/lib.rs. It is a JPEG 2000 image or
// codestream that New opened, before its decoding.
type Image struct {
	codestream []byte
	header     *header
	boxes      imageBoxes
	settings   DecodeSettings
	hasAlpha   bool
	colorSpace ColorSpace
}

// New is Image::new. It opens a JP2 file or a raw codestream. A nil
// settings is DefaultDecodeSettings. The image keeps a reference to data.
func New(data []byte, settings *DecodeSettings) (img *Image, err error) {
	defer recoverPanic(&err)
	s := settingsOrDefault(settings)
	switch {
	case hasPrefix(data, jp2Magic):
		return jp2Parse(data, s)
	case hasPrefix(data, codestreamMagic):
		return j2cParse(data, &s, 0, false)
	}
	return nil, errInvalidSignature
}

// FromCodestream is Image::from_codestream. It opens a raw codestream with
// the number of color channels that the caller knows, as the PDF filter
// JPXDecode does. A nil settings is DefaultDecodeSettings.
func FromCodestream(data []byte, colorChannels uint8, settings *DecodeSettings) (img *Image, err error) {
	defer recoverPanic(&err)
	s := settingsOrDefault(settings)
	if hasPrefix(data, codestreamMagic) {
		return j2cParse(data, &s, colorChannels, true)
	}
	return nil, errInvalidSignature
}

func settingsOrDefault(settings *DecodeSettings) DecodeSettings {
	if settings == nil {
		return DefaultDecodeSettings()
	}
	s := *settings
	if s.TargetResolution != nil {
		t := *s.TargetResolution
		s.TargetResolution = &t
	}
	return s
}

func hasPrefix(data, prefix []byte) bool {
	return len(data) >= len(prefix) && string(data[:len(prefix)]) == string(prefix)
}

// HasAlpha is Image::has_alpha. It returns whether the image has an alpha
// channel.
func (img *Image) HasAlpha() bool {
	return img.hasAlpha
}

// ColorSpace is Image::color_space. It returns the color space of the
// image.
func (img *Image) ColorSpace() ColorSpace {
	return img.colorSpace
}

// Width is Image::width. It returns the width of the image.
func (img *Image) Width() uint32 {
	return img.header.sizeData.imageWidth()
}

// Height is Image::height. It returns the height of the image.
func (img *Image) Height() uint32 {
	return img.header.sizeData.imageHeight()
}

// OriginalBitDepth is Image::original_bit_depth. It returns the bit depth
// of the first component. It assumes that all components share the same
// precision.
func (img *Image) OriginalBitDepth() uint8 {
	return img.header.componentInfos[0].sizeInfo.precision
}

// Decode is Image::decode. It decodes the image, and returns its decoded
// components. A nil ctx decodes with a new context. The DecodedImage holds
// the buffers of ctx until the next decoding with ctx.
//
// Port: upstream takes no nil context.
func (img *Image) Decode(ctx *DecoderContext) (decoded *DecodedImage, err error) {
	defer recoverPanic(&err)
	if ctx == nil {
		ctx = &DecoderContext{}
	}
	settings := &img.settings
	c := cancel{handle: settings.Cancel}
	lim := newLimits(settings.Limits)

	if err := j2cDecode(img.codestream, img.header, ctx, c, lim); err != nil {
		return nil, err
	}
	d := &DecodedImage{
		decodedComponents: ctx.channelData,
		boxes:             img.boxes,
		cancel:            c,
		lim:               lim,
	}

	if settings.ResolvePaletteIndices {
		resolved, err := resolvePaletteIndices(d.decodedComponents, &d.boxes, c, lim)
		if err != nil {
			return nil, err
		}
		d.decodedComponents = resolved
		ctx.channelData = resolved
	}

	if cdef := d.boxes.channelDefinition; cdef != nil {
		// Assumes each component is referenced only once.
		//
		// Port: upstream clones the components, with a TODO to move them.
		// The port moves them.
		type keyed struct {
			c   ComponentData
			key uint32
		}
		n := min(len(d.decodedComponents), len(cdef.channelDefinitions))
		components := make([]keyed, n)
		for i := range n {
			key := uint32(cdef.channelDefinitions[i].association)
			if cdef.channelDefinitions[i].association == associationWholeImage {
				key = 0xffff
			}
			components[i] = keyed{c: d.decodedComponents[i], key: key}
		}
		slices.SortStableFunc(components, func(a, b keyed) int {
			return int(a.key) - int(b.key)
		})
		sorted := make([]ComponentData, n)
		for i := range components {
			sorted[i] = components[i].c
		}
		d.decodedComponents = sorted
		ctx.channelData = sorted
	}

	if len(d.decodedComponents) == 0 {
		// Port: upstream reads the first component, and panics when there
		// is none.
		return nil, errPaletteResolutionFailed
	}

	// Assumes all components share the same bit depth.
	bitDepth := d.decodedComponents[0].bitDepth
	if err := convertColorSpace(d, bitDepth); err != nil {
		return nil, err
	}
	return d, nil
}

// resolveAlphaAndColorSpace is resolve_alpha_and_color_space in
// hayro-jpeg2000 src/lib.rs.
func resolveAlphaAndColorSpace(boxes *imageBoxes, h *header, settings *DecodeSettings) (ColorSpace, bool, error) {
	numComponents := len(h.componentInfos)
	if settings.ResolvePaletteIndices && boxes.palette != nil {
		numComponents = len(boxes.palette.columns)
	}

	hasAlpha := false
	if cdef := boxes.channelDefinition; cdef != nil && len(cdef.channelDefinitions) != 0 {
		last := cdef.channelDefinitions[len(cdef.channelDefinitions)-1]
		hasAlpha = last.channelType == channelOpacity
	}

	cs, err := getColorSpace(boxes, numComponents)
	if err != nil {
		return ColorSpace{}, false, err
	}

	if !settings.ResolvePaletteIndices && boxes.palette != nil {
		hasAlpha = false
		cs = ColorSpace{Kind: ColorSpaceGray}
	}

	actual := len(h.componentInfos)
	alpha := 0
	if hasAlpha {
		alpha = 1
	}

	if boxes.palette == nil && actual != int(cs.NumChannels())+alpha {
		switch {
		case !settings.Strict && actual == int(cs.NumChannels())+1 && !hasAlpha:
			// Treat one extra component as alpha (OpenJPEG test
			// orb-blue10-lin-j2k).
			hasAlpha = true
		case actual == 1 || (actual == 2 && hasAlpha):
			cs = ColorSpace{Kind: ColorSpaceGray}
		case actual == 3:
			cs = ColorSpace{Kind: ColorSpaceRGB}
		case actual == 4 && hasAlpha:
			cs = ColorSpace{Kind: ColorSpaceRGB}
		case actual == 4:
			cs = ColorSpace{Kind: ColorSpaceCMYK}
		default:
			n := actual - alpha
			if n > 0xff {
				return ColorSpace{}, false, errTooManyChannels
			}
			cs = ColorSpace{Kind: ColorSpaceUnknown, Channels: uint8(n)}
		}
	}
	return cs, hasAlpha, nil
}

// ColorSpaceKind is the case of ColorSpace in hayro-jpeg2000 src/lib.rs.
type ColorSpaceKind uint8

// The kinds of color spaces.
const (
	// ColorSpaceGray is ColorSpace::Gray: a grayscale image.
	ColorSpaceGray ColorSpaceKind = iota
	// ColorSpaceRGB is ColorSpace::RGB: an RGB image.
	ColorSpaceRGB
	// ColorSpaceCMYK is ColorSpace::CMYK: a CMYK image.
	ColorSpaceCMYK
	// ColorSpaceUnknown is ColorSpace::Unknown: an unknown color space.
	ColorSpaceUnknown
	// ColorSpaceICC is ColorSpace::Icc: an image based on an ICC profile.
	ColorSpaceICC
)

// ColorSpace is ColorSpace in hayro-jpeg2000 src/lib.rs. It is the color
// space of the image.
//
// Port: the Rust enum becomes a struct. Kind names the case. Channels holds
// num_channels of Unknown and of Icc, and Profile holds the profile of Icc.
type ColorSpace struct {
	Kind ColorSpaceKind
	// Channels is the number of channels of an unknown color space, or of
	// the ICC profile.
	Channels uint8
	// Profile is the raw data of the ICC profile.
	Profile []byte
}

// NumChannels is ColorSpace::num_channels. It returns the number of
// channels that the color space expects.
func (cs ColorSpace) NumChannels() uint8 {
	switch cs.Kind {
	case ColorSpaceGray:
		return 1
	case ColorSpaceRGB:
		return 3
	case ColorSpaceCMYK:
		return 4
	}
	return cs.Channels
}

// Bitmap is Bitmap in hayro-jpeg2000 src/lib.rs. It holds the decoded
// result of the image.
type Bitmap struct {
	// ColorSpace is the color space of the image.
	ColorSpace ColorSpace
	// Data holds the interleaved samples of 8 bits, with the alpha channel
	// last.
	Data []byte
	// HasAlpha is whether the image has an alpha channel.
	HasAlpha bool
	// Width is the width of the image.
	Width uint32
	// Height is the height of the image.
	Height uint32
	// OriginalBitDepth is the original bit depth of the image.
	OriginalBitDepth uint8
}

// Components is DecodedImage::components. It returns the decoded
// components of the image.
func (d *DecodedImage) Components() []ComponentData {
	return d.decodedComponents
}

// DataU8 is DecodedImage::data_u8. It returns the decoded image as
// interleaved unsigned samples of 8 bits.
func (d *DecodedImage) DataU8() (buf []byte, err error) {
	defer recoverPanic(&err)
	components := d.Components()
	if len(components) == 0 {
		return nil, nil
	}
	size := uint64(len(components[0].Samples())) * uint64(len(components))
	if err := d.lim.memory(size); err != nil {
		return nil, err
	}
	buf = make([]byte, size)
	if err := d.StoreU8Into(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// StoreU8Into is DecodedImage::store_u8_into. It stores the decoded image
// as interleaved samples of 8 bits into buf, which must fit exactly.
//
// Port: upstream panics when buf is too short. The port stops at the end
// of buf.
func (d *DecodedImage) StoreU8Into(buf []byte) (err error) {
	defer recoverPanic(&err)
	return interleaveAndConvert(d, buf)
}

// toU8Scale is the function to_u8_scale of interleave_and_convert.
func toU8Scale(bitDepth uint8) float32 {
	return 255.0 / float32(shl32(1, uint32(bitDepth))-1)
}

// interleaveAndConvert is interleave_and_convert in hayro-jpeg2000
// src/lib.rs.
func interleaveAndConvert(d *DecodedImage, buf []byte) error {
	c := d.cancel
	components := d.decodedComponents
	num := len(components)
	if num == 0 {
		return nil
	}

	allSame := true
	for _, comp := range components[1:] {
		if comp.bitDepth != components[0].bitDepth {
			allSame = false
		}
	}

	maxLen := len(components[0].container.truncated())

	if allSame && num <= 4 {
		scale := toU8Scale(components[0].bitDepth)
		maxLen = min(maxLen, len(buf)/num)
		planes := make([][]float32, num)
		for start := 0; start < maxLen; {
			if err := c.check(); err != nil {
				return err
			}
			end := min(start+chunk, maxLen)
			for i := range components {
				data := components[i].container.data
				planes[i] = data[min(start, len(data)):min(end, len(data))]
			}
			interleaveToU8(planes, scale, buf[start*num:end*num])
			start = end
		}
		return nil
	}

	scales := make([]float32, num)
	for i := range components {
		scales[i] = toU8Scale(components[i].bitDepth)
	}
	out := 0
	for sample := range maxLen {
		if sample%chunk == 0 {
			if err := c.check(); err != nil {
				return err
			}
		}
		for i := range components {
			if out >= len(buf) {
				return nil
			}
			var v float32
			if data := components[i].container.data; sample < len(data) {
				v = data[sample]
			}
			buf[out] = roundToU8(v * scales[i])
			out++
		}
	}
	return nil
}

// interleaveToU8 is interleave_to_u8 in hayro-jpeg2000 src/lib.rs, with
// interleave_to_u8_impl in its scalar version.
func interleaveToU8(planes [][]float32, scale float32, out []byte) {
	n := len(planes)
	pixels := len(out) / n
	for _, p := range planes {
		pixels = min(pixels, len(p))
	}
	truncated := make([][]float32, n)
	for i, p := range planes {
		truncated[i] = p[:pixels]
	}
	roundAndInterleave(truncated, scale, out[:pixels*n])
}

// convertColorSpace is convert_color_space in hayro-jpeg2000 src/lib.rs.
func convertColorSpace(d *DecodedImage, bitDepth uint8) error {
	spec := d.boxes.colorSpecification
	if spec == nil || spec.colorSpace.kind != colrEnumerated {
		return nil
	}
	switch e := spec.colorSpace.enumerated; e.kind {
	case enumSycc:
		return syccToRGB(d.decodedComponents, bitDepth, d.cancel)
	case enumCieLab:
		return cielabToRGB(d.decodedComponents, bitDepth, &e.cieLab, d.cancel)
	case enumYcck:
		return ycckToCMYK(d.decodedComponents, bitDepth, d.cancel)
	}
	return nil
}

// getColorSpace is get_color_space in hayro-jpeg2000 src/lib.rs.
func getColorSpace(boxes *imageBoxes, numComponents int) (ColorSpace, error) {
	cs := colrColorSpace{kind: colrUnknown}
	if boxes.colorSpecification != nil {
		cs = boxes.colorSpecification.colorSpace
	}

	switch cs.kind {
	case colrEnumerated:
		switch cs.enumerated.kind {
		case enumCmyk, enumYcck:
			return ColorSpace{Kind: ColorSpaceCMYK}, nil
		case enumSrgb, enumEsRgb, enumSycc:
			return ColorSpace{Kind: ColorSpaceRGB}, nil
		case enumRommRgb:
			return ColorSpace{Kind: ColorSpaceICC, Profile: slices.Clone(proPhotoProfile), Channels: 3}, nil
		case enumGreyscale:
			return ColorSpace{Kind: ColorSpaceGray}, nil
		case enumCieLab:
			return ColorSpace{Kind: ColorSpaceICC, Profile: slices.Clone(labProfile), Channels: 3}, nil
		}
		return ColorSpace{}, errFormatUnsupported
	case colrICC:
		if m, ok := iccMetadataFromData(cs.icc); ok {
			return ColorSpace{Kind: ColorSpaceICC, Profile: slices.Clone(cs.icc), Channels: m.colorSpace.numComponents()}, nil
		}
		// Fall back to RGB for unparseable profiles (OpenJPEG test
		// orb-blue10-lin-jp2).
		return ColorSpace{Kind: ColorSpaceRGB}, nil
	}

	switch numComponents {
	case 1:
		return ColorSpace{Kind: ColorSpaceGray}, nil
	case 3:
		return ColorSpace{Kind: ColorSpaceRGB}, nil
	case 4:
		return ColorSpace{Kind: ColorSpaceCMYK}, nil
	}
	if numComponents > 0xff {
		return ColorSpace{}, errTooManyChannels
	}
	return ColorSpace{Kind: ColorSpaceUnknown, Channels: uint8(numComponents)}, nil
}

// resolvePaletteIndices is resolve_palette_indices in hayro-jpeg2000
// src/lib.rs. It maps the components through the palette, as the component
// mapping box says.
func resolvePaletteIndices(components []ComponentData, boxes *imageBoxes, c cancel, lim *limits) ([]ComponentData, error) {
	palette := boxes.palette
	if palette == nil {
		return components, nil
	}
	mapping := boxes.componentMapping
	if mapping == nil {
		// Port: upstream unwraps the mapping, which the parser of the JP2
		// file always sets with a palette.
		return nil, errPaletteResolutionFailed
	}

	maxIndex := int64(palette.numEntries) - 1
	resolved := make([]ComponentData, 0, len(mapping.entries))
	var used uint64

	for _, entry := range mapping.entries {
		if err := c.check(); err != nil {
			return nil, err
		}
		idx := int(entry.componentIndex)
		if idx >= len(components) {
			return nil, errPaletteResolutionFailed
		}
		component := &components[idx]

		// Port: the limits bound the memory of the resolved components.
		used += uint64(len(component.container.data)) * 4
		if err := lim.memory(used); err != nil {
			return nil, err
		}

		if !entry.palette {
			resolved = append(resolved, component.clone())
			continue
		}

		column := int(entry.column)
		if column >= len(palette.columns) {
			return nil, errPaletteResolutionFailed
		}
		info := palette.columns[column]

		samples := component.container.truncated()
		mapped := make([]float32, len(samples), len(samples)+simdWidth)
		for i, sample := range samples {
			if i%chunk == 0 {
				if err := c.check(); err != nil {
					return nil, err
				}
			}
			// Lossy decoding can reconstruct indices outside the palette.
			index := min(max(f32ToI64(sample+0.5), 0), maxIndex)
			value, ok := palette.mapEntry(int(index), column)
			if !ok {
				return nil, errPaletteResolutionFailed
			}
			mapped[i] = float32(value)
		}
		resolved = append(resolved, ComponentData{container: newSimdBuffer(mapped), bitDepth: info.bitDepth})
	}
	return resolved, nil
}

// cielabToRGB is cielab_to_rgb in hayro-jpeg2000 src/lib.rs. It only
// normalizes the Lab values. The conversion of the color happens through
// the Lab ICC profile.
func cielabToRGB(components []ComponentData, bitDepth uint8, lab *cieLab, c cancel) error {
	if len(components) < 3 {
		return errLabConversionFailed
	}
	l, a, b := &components[0], &components[1], &components[2]

	prec0, prec1, prec2 := l.bitDepth, a.bitDepth, b.bitDepth
	// Prevent underflows/divisions by zero further below.
	if prec0 < 4 || prec1 < 4 || prec2 < 4 {
		return errLabConversionFailed
	}

	rl := lab.rl.unwrapOr(100)
	ra := lab.ra.unwrapOr(170)
	rb := lab.rb.unwrapOr(200)
	ol := lab.ol.unwrapOr(0)
	oa := lab.oa.unwrapOr(shl32(1, uint32(bitDepth-1)))
	ob := lab.ob.unwrapOr(shl32(1, uint32(bitDepth-2)) + shl32(1, uint32(bitDepth-3)))

	divisor := func(prec uint8) float32 {
		return float32(shlI32(1, uint32(prec)) - 1)
	}

	// Copied from OpenJPEG.
	minL := -(float32(rl) * float32(ol)) / divisor(prec0)
	maxL := minL + float32(rl)
	minA := -(float32(ra) * float32(oa)) / divisor(prec1)
	maxA := minA + float32(ra)
	minB := -(float32(rb) * float32(ob)) / divisor(prec2)
	maxB := minB + float32(rb)

	bitMax := shl32(1, uint32(bitDepth)) - 1

	divisorL := divisor(prec0)
	divisorA := divisor(prec1)
	divisorB := divisor(prec2)

	scaleLFinal := float32(bitMax) / 100.0
	scaleABFinal := float32(bitMax) / 255.0

	lOffset := minL * scaleLFinal
	lScale := (maxL - minL) / divisorL * scaleLFinal
	aOffset := (minA + 128.0) * scaleABFinal
	aScale := (maxA - minA) / divisorA * scaleABFinal
	bOffset := (minB + 128.0) * scaleABFinal
	bScale := (maxB - minB) / divisorB * scaleABFinal

	ld, ad, bd := l.container.data, a.container.data, b.container.data
	n := min(len(ld), len(ad), len(bd)) / simdWidth * simdWidth
	for i := 0; i < n; i += simdWidth {
		if (i/simdWidth)%(chunk/simdWidth) == 0 {
			if err := c.check(); err != nil {
				return err
			}
		}
		for j := i; j < i+simdWidth; j++ {
			ld[j] = mulAdd(ld[j], lScale, lOffset)
			ad[j] = mulAdd(ad[j], aScale, aOffset)
			bd[j] = mulAdd(bd[j], bScale, bOffset)
		}
	}
	return nil
}

// ycckToCMYK is ycck_to_cmyk in hayro-jpeg2000 src/lib.rs.
func ycckToCMYK(components []ComponentData, bitDepth uint8, c cancel) error {
	if err := syccToRGB(components, bitDepth, c); err != nil {
		return err
	}

	maxValue := float32(shl32(1, uint32(bitDepth)) - 1)
	for i := range min(len(components), 3) {
		if err := c.check(); err != nil {
			return err
		}
		data := components[i].container.data
		for j := range data {
			data[j] = maxValue - data[j]
		}
	}
	return nil
}

// syccToRGB is sycc_to_rgb in hayro-jpeg2000 src/lib.rs.
func syccToRGB(components []ComponentData, bitDepth uint8, c cancel) error {
	offset := float32(shl32(1, uint32(bitDepth)-1))
	maxValue := float32(shl32(1, uint32(bitDepth)) - 1)

	if len(components) < 3 {
		return errSyccConversionFailed
	}
	yd, cbd, crd := components[0].container.data, components[1].container.data, components[2].container.data

	n := min(len(yd), len(cbd), len(crd)) / simdWidth * simdWidth
	for i := 0; i < n; i += simdWidth {
		if (i/simdWidth)%(chunk/simdWidth) == 0 {
			if err := c.check(); err != nil {
				return err
			}
		}
		for j := i; j < i+simdWidth; j++ {
			y := yd[j]
			cb := cbd[j] - offset
			cr := crd[j] - offset

			r := mulAdd(cr, 1.402, y)
			g := mulAdd(cr, -0.714136, mulAdd(cb, -0.344136, y))
			b := mulAdd(cb, 1.772, y)

			yd[j] = maxF32(minF32(r, maxValue), 0)
			cbd[j] = maxF32(minF32(g, maxValue), 0)
			crd[j] = maxF32(minF32(b, maxValue), 0)
		}
	}
	return nil
}

// limits holds the limits of DecodeSettings.
//
// Port: upstream has no limits. A nil *limits has none either.
type limits struct {
	maxMemory  uint64
	maxPixels  uint64
	maxObjects uint64
}

// newLimits returns the limits, with the defaults of ivy.
func newLimits(l ivy.Limits) *limits {
	l = l.WithDefaults()
	return &limits{
		maxMemory:  uint64(l.MaxMemory),
		maxPixels:  uint64(l.MaxPixels),
		maxObjects: uint64(l.MaxObjects),
	}
}

// memory returns an error when a buffer of n bytes exceeds the limits.
func (l *limits) memory(n uint64) error {
	if l != nil && n > l.maxMemory {
		return errImageTooLarge
	}
	return nil
}

// pixels returns an error when an image of n pixels exceeds the limits.
func (l *limits) pixels(n uint64) error {
	if l != nil && n > l.maxPixels {
		return errImageTooLarge
	}
	return nil
}

// objects returns an error when n objects exceed the limits.
func (l *limits) objects(n uint64) error {
	if l != nil && n > l.maxObjects {
		return errImageTooLarge
	}
	return nil
}

// recoverPanic turns a runtime panic into an error.
//
// Port: upstream relies on its own checks and on fuzzing to never panic.
// The port keeps the checks, and it adds this net, so that a check that the
// port missed cannot crash the program that reads a malformed file. The
// error wraps ivy.ErrFormat. The fuzz test fails on it.
func recoverPanic(err *error) {
	r := recover()
	if r == nil {
		return
	}
	if _, ok := r.(runtime.Error); !ok {
		panic(r)
	}
	e := newError(KindDecoding, fmt.Sprint("internal error: ", r), ivy.ErrFormat)
	e.stack = debug.Stack()
	*err = e
}
