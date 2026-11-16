// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/mod.rs.

// parsedCodestream is ParsedCodestream in hayro-jpeg2000 src/j2c/mod.rs.
type parsedCodestream struct {
	header *header
	data   []byte
}

// ComponentData is ComponentData in hayro-jpeg2000 src/j2c/mod.rs. It holds
// the decoded data of one component.
type ComponentData struct {
	container simdBuffer
	bitDepth  uint8
}

// BitDepth is ComponentData::bit_depth. It returns the bit depth of the
// component.
func (c *ComponentData) BitDepth() uint8 {
	return c.bitDepth
}

// Samples is ComponentData::samples. It returns the decoded samples of the
// component, one for each pixel, from 0 to 2^BitDepth - 1.
func (c *ComponentData) Samples() []float32 {
	return c.container.truncated()
}

// clone returns a copy of the component with samples of its own, as Clone
// does upstream.
func (c *ComponentData) clone() ComponentData {
	data := make([]float32, len(c.container.data))
	copy(data, c.container.data)
	return ComponentData{
		container: simdBuffer{data: data, originalLen: c.container.originalLen},
		bitDepth:  c.bitDepth,
	}
}

// j2cParse is parse in hayro-jpeg2000 src/j2c/mod.rs. It opens a raw
// codestream. colorChannels is the number of color channels that the
// caller knows, or zero.
//
// Port: the Option<u8> of upstream is a value and the boolean
// hasColorChannels.
func j2cParse(stream []byte, settings *DecodeSettings, colorChannels uint8, hasColorChannels bool) (*Image, error) {
	pc, err := parseRaw(stream, settings)
	if err != nil {
		return nil, err
	}
	h := pc.header
	var boxes imageBoxes

	var cs ColorSpace
	var hasAlpha bool
	if hasColorChannels {
		switch colorChannels {
		case 1:
			cs = ColorSpace{Kind: ColorSpaceGray}
		case 3:
			cs = ColorSpace{Kind: ColorSpaceRGB}
		case 4:
			cs = ColorSpace{Kind: ColorSpaceCMYK}
		default:
			cs = ColorSpace{Kind: ColorSpaceUnknown, Channels: colorChannels}
		}
		hasAlpha = len(h.componentInfos) == int(colorChannels)+1
	} else {
		// Three components with subsampled chroma are practically always
		// YCbCr.
		infos := h.componentInfos
		hasSubsampledChroma := false
		if len(infos) == 3 {
			sub := func(c *componentInfo) [2]uint8 {
				return [2]uint8{c.sizeInfo.horizontalResolution, c.sizeInfo.verticalResolution}
			}
			y, cb, cr := sub(&infos[0]), sub(&infos[1]), sub(&infos[2])
			hasSubsampledChroma = cb == cr && cb != y && cb[0] >= y[0] && cb[1] >= y[1]
		}

		e := enumeratedColorspace{kind: srgb}
		switch {
		case len(infos) < 3:
			e.kind = greyscale
		case hasSubsampledChroma:
			e.kind = sycc
		}
		boxes.colorSpecification = &colorSpecificationBox{colorSpace: colrColorSpace{kind: colrEnumerated, enumerated: e}}

		if cs, hasAlpha, err = resolveAlphaAndColorSpace(&boxes, h, settings); err != nil {
			return nil, err
		}
	}

	return &Image{
		codestream: pc.data,
		header:     h,
		boxes:      boxes,
		settings:   *settings,
		colorSpace: cs,
		hasAlpha:   hasAlpha,
	}, nil
}

// parseRaw is parse_raw in hayro-jpeg2000 src/j2c/mod.rs. It reads the
// marker SOC and the main header.
func parseRaw(stream []byte, settings *DecodeSettings) (parsedCodestream, error) {
	reader := newBitReader(stream)

	m, err := reader.readMarker()
	if err != nil {
		return parsedCodestream{}, err
	}
	if m != markerSOC {
		return parsedCodestream{}, errMarkerExpected("SOC")
	}

	h, err := readHeader(&reader, settings)
	if err != nil {
		return parsedCodestream{}, err
	}
	data, ok := reader.tail()
	if !ok {
		return parsedCodestream{}, errMissingCodestream
	}
	return parsedCodestream{header: h, data: data}, nil
}
