// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/codestream.rs.

import (
	"math/bits"
	"slices"
)

const (
	// maxLayerCount is MAX_LAYER_COUNT.
	maxLayerCount = 32
	// maxResolutionCount is MAX_RESOLUTION_COUNT.
	maxResolutionCount = 32
	// maxPrecinctExponent is MAX_PRECINCT_EXPONENT.
	maxPrecinctExponent = 31
)

// header is Header in hayro-jpeg2000 src/j2c/codestream.rs. It is the main
// header of a codestream.
type header struct {
	sizeData                sizeData
	globalCodingStyle       codingStyleDefault
	componentInfos          []componentInfo
	ppmPackets              [][]byte
	progressionChanges      []progressionChange
	skippedResolutionLevels uint8
	strict                  bool
}

// ppmMarkerData is PpmMarkerData in hayro-jpeg2000 src/j2c/codestream.rs.
type ppmMarkerData struct {
	sequenceIdx uint8
	data        []byte
}

// readHeader is read_header in hayro-jpeg2000 src/j2c/codestream.rs. It
// reads the main header, from the marker SIZ to the first marker SOT.
func readHeader(reader *bitReader, settings *DecodeSettings) (*header, error) {
	m, err := reader.readMarker()
	if err != nil {
		return nil, err
	}
	if m != markerSIZ {
		return nil, errMarkerExpected("SIZ")
	}

	sd, err := sizeMarker(reader)
	if err != nil {
		return nil, err
	}
	if _, err := sd.numTiles(); err != nil {
		return nil, err
	}

	var cod *codingStyleDefault
	var qcd *quantizationInfo

	numComponents := uint16(len(sd.componentSizes))
	codComponents := make([]*codingStyleComponent, numComponents)
	qcdComponents := make([]*quantizationInfo, numComponents)
	roiShifts := make([]uint8, numComponents)
	var ppmMarkers []ppmMarkerData
	var progressionChanges []progressionChange

loop:
	for {
		m, ok := reader.peekMarker()
		if !ok {
			return nil, errMarkerInvalid
		}
		switch {
		case m == markerSOT:
			break loop
		case m == markerCOD:
			_, _ = reader.readMarker()
			c, ok := codMarker(reader)
			if !ok {
				return nil, errMarkerParseFailure("COD")
			}
			cod = &c
		case m == markerCOC:
			_, _ = reader.readMarker()
			idx, coc, ok := cocMarker(reader, numComponents)
			if !ok || int(idx) >= len(codComponents) {
				return nil, errMarkerParseFailure("COC")
			}
			codComponents[idx] = &coc
		case m == markerQCD:
			_, _ = reader.readMarker()
			q, ok := qcdMarker(reader)
			if !ok {
				return nil, errMarkerParseFailure("QCD")
			}
			qcd = &q
		case m == markerQCC:
			_, _ = reader.readMarker()
			idx, qcc, ok := qccMarker(reader, numComponents)
			if !ok || int(idx) >= len(qcdComponents) {
				return nil, errMarkerParseFailure("QCC")
			}
			qcdComponents[idx] = &qcc
		case m == markerRGN:
			_, _ = reader.readMarker()
			idx, shift, ok := rgnMarker(reader, numComponents)
			if !ok || int(idx) >= len(roiShifts) {
				return nil, errMarkerParseFailure("RGN")
			}
			roiShifts[idx] = shift
		case m == markerTLM:
			_, _ = reader.readMarker()
			if !tlmMarker(reader) {
				return nil, errMarkerParseFailure("TLM")
			}
		case m == markerCOM:
			_, _ = reader.readMarker()
			if !comMarker(reader) {
				return nil, errMarkerParseFailure("COM")
			}
		case m == markerPPM:
			_, _ = reader.readMarker()
			p, ok := ppmMarker(reader)
			if !ok {
				return nil, errMarkerParseFailure("PPM")
			}
			ppmMarkers = append(ppmMarkers, p)
		case m == markerPOC:
			_, _ = reader.readMarker()
			changes, ok := pocMarker(reader, numComponents)
			if !ok {
				return nil, errMarkerParseFailure("POC")
			}
			progressionChanges = append(progressionChanges, changes...)
		case m == markerCRG:
			_, _ = reader.readMarker()
			skipMarkerSegment(reader)
		case m >= 0x30 && m <= 0x3f:
			_, _ = reader.readMarker()
		default:
			return nil, errMarkerUnsupported
		}
	}

	if cod == nil {
		return nil, errMarkerMissing("COD")
	}
	if qcd == nil {
		return nil, errMarkerMissing("QCD")
	}

	infos := make([]componentInfo, len(sd.componentSizes))
	for idx, csi := range sd.componentSizes {
		style := cod.componentParameters
		if c := codComponents[idx]; c != nil {
			style = *c
			style.flags.raw |= cod.componentParameters.flags.raw
		}
		quant := *qcd
		if q := qcdComponents[idx]; q != nil {
			quant = *q
		}
		infos[idx] = componentInfo{
			sizeInfo:         csi,
			codingStyle:      style,
			quantizationInfo: quant,
			roiShift:         roiShifts[idx],
		}
	}

	minNumResolutionLevels := infos[0].numResolutionLevels()
	for _, info := range infos[1:] {
		minNumResolutionLevels = min(minNumResolutionLevels, info.numResolutionLevels())
	}

	var skipped uint8
	if t := settings.TargetResolution; t != nil {
		widthLog := checkedDivIlog2(sd.imageWidth(), t.Width)
		heightLog := checkedDivIlog2(sd.imageHeight(), t.Height)
		skipped = uint8(min(widthLog, heightLog))
	}
	skipped = min(skipped, minNumResolutionLevels-1)

	sd.xResolutionShrinkFactor *= shl32(1, uint32(skipped))
	sd.yResolutionShrinkFactor *= shl32(1, uint32(skipped))

	packets, ok := ppmPackets(ppmMarkers)
	if !ok {
		return nil, errMarkerParseFailure("PPM")
	}

	h := &header{
		sizeData:                sd,
		globalCodingStyle:       *cod,
		componentInfos:          infos,
		ppmPackets:              packets,
		progressionChanges:      progressionChanges,
		skippedResolutionLevels: skipped,
		strict:                  settings.Strict,
	}
	if err := validate(h); err != nil {
		return nil, err
	}
	return h, nil
}

// checkedDivIlog2 is a.checked_div(b).and_then(|v| v.checked_ilog2())
// .unwrap_or(0) of Rust.
func checkedDivIlog2(a, b uint32) uint32 {
	if b == 0 {
		return 0
	}
	v := a / b
	if v == 0 {
		return 0
	}
	return uint32(bits.Len32(v) - 1)
}

// validate is validate in hayro-jpeg2000 src/j2c/codestream.rs.
func validate(h *header) error {
	for i := range h.componentInfos {
		info := &h.componentInfos[i]
		maxResolutionIdx := info.codingStyle.parameters.numResolutionLevels - 1
		style := info.quantizationInfo.quantizationStyle
		numPrecinctExponents := len(info.quantizationInfo.stepSizes)

		switch {
		case numPrecinctExponents == 0:
			return errMissingPrecinctExponents
		case style == noQuantization || style == scalarExpounded:
			// Must cover the largest index accessed in exponentMantissa.
			if maxResolutionIdx != 0 && 1+(int(maxResolutionIdx)-1)*3+2 >= numPrecinctExponents {
				return errInsufficientExponents
			}
		}
	}
	return nil
}

// componentInfo is ComponentInfo in hayro-jpeg2000 src/j2c/codestream.rs.
// A copy of the struct shares the slices of its parameters, which no code
// changes in place.
type componentInfo struct {
	sizeInfo         componentSizeInfo
	codingStyle      codingStyleComponent
	quantizationInfo quantizationInfo
	roiShift         uint8
}

// exponentMantissa is ComponentInfo::exponent_mantissa.
func (c *componentInfo) exponentMantissa(t subBandType, resolution uint8) (exponent, mantissa uint16, err error) {
	nLL := c.codingStyle.parameters.numDecompositionLevels

	var sbIndex int
	switch t {
	case lowLow:
		// TODO: The decoder never reaches this case.
		sbIndex = 0xffff
	case highLow:
		sbIndex = 0
	case lowHigh:
		sbIndex = 1
	case highHigh:
		sbIndex = 2
	}

	stepSizes := c.quantizationInfo.stepSizes
	switch c.quantizationInfo.quantizationStyle {
	case scalarDerived:
		if len(stepSizes) == 0 {
			return 0, 0, errMissingStepSize
		}
		e0, m := stepSizes[0].exponent, stepSizes[0].mantissa
		nB := uint16(nLL)
		if resolution != 0 {
			nB = uint16(nLL) + 1 - uint16(resolution)
		}
		if e0 < uint16(nLL) {
			return 0, 0, errInvalidExponents
		}
		e := uint32(e0-uint16(nLL)) + uint32(nB)
		if e > 0xffff {
			return 0, 0, errInvalidExponents
		}
		return uint16(e), m, nil
	default:
		idx := 0
		if resolution != 0 {
			idx = 1 + (int(resolution)-1)*3 + sbIndex
		}
		if idx >= len(stepSizes) {
			return 0, 0, errMissingStepSize
		}
		return stepSizes[idx].exponent, stepSizes[idx].mantissa, nil
	}
}

// waveletTransform is ComponentInfo::wavelet_transform.
func (c *componentInfo) waveletTransform() waveletTransform {
	return c.codingStyle.parameters.transformation
}

// numResolutionLevels is ComponentInfo::num_resolution_levels.
func (c *componentInfo) numResolutionLevels() uint8 {
	return c.codingStyle.parameters.numResolutionLevels
}

// numDecompositionLevels is ComponentInfo::num_decomposition_levels.
func (c *componentInfo) numDecompositionLevels() uint8 {
	return c.codingStyle.parameters.numDecompositionLevels
}

// codeBlockStyle is ComponentInfo::code_block_style.
func (c *componentInfo) codeBlockStyle() codeBlockStyle {
	return c.codingStyle.parameters.codeBlockStyle
}

// progressionOrder is ProgressionOrder in hayro-jpeg2000
// src/j2c/codestream.rs.
type progressionOrder uint8

// The progression orders.
const (
	layerResolutionComponentPosition progressionOrder = iota
	resolutionLayerComponentPosition
	resolutionPositionComponentLayer
	positionComponentResolutionLayer
	componentPositionResolutionLayer
)

// progressionOrderFromU8 is ProgressionOrder::from_u8.
func progressionOrderFromU8(value uint8) (progressionOrder, error) {
	if value > 4 {
		return 0, errInvalidProgressionOrder
	}
	return progressionOrder(value), nil
}

// progressionChange is ProgressionChange in hayro-jpeg2000
// src/j2c/codestream.rs.
type progressionChange struct {
	resolutionStart  uint8
	componentStart   uint16
	layerEnd         uint16
	resolutionEnd    uint8
	componentEnd     uint16
	progressionOrder progressionOrder
}

// waveletTransform is WaveletTransform in hayro-jpeg2000
// src/j2c/codestream.rs.
type waveletTransform uint8

// The wavelet transforms.
const (
	irreversible97 waveletTransform = iota
	reversible53
)

// waveletTransformFromU8 is WaveletTransform::from_u8.
func waveletTransformFromU8(value uint8) (waveletTransform, error) {
	switch value {
	case 0:
		return irreversible97, nil
	case 1:
		return reversible53, nil
	}
	return 0, errInvalidTransformation
}

// codingStyleFlags is CodingStyleFlags in hayro-jpeg2000
// src/j2c/codestream.rs.
type codingStyleFlags struct {
	raw uint8
}

// hasPrecincts is CodingStyleFlags::has_precincts.
func (f codingStyleFlags) hasPrecincts() bool {
	return f.raw&0x01 != 0
}

// mayUseSopMarkers is CodingStyleFlags::may_use_sop_markers.
func (f codingStyleFlags) mayUseSopMarkers() bool {
	return f.raw&0x02 != 0
}

// usesEphMarker is CodingStyleFlags::uses_eph_marker.
func (f codingStyleFlags) usesEphMarker() bool {
	return f.raw&0x04 != 0
}

// codeBlockStyle is CodeBlockStyle in hayro-jpeg2000
// src/j2c/codestream.rs.
type codeBlockStyle struct {
	selectiveArithmeticCodingBypass bool
	resetContextProbabilities       bool
	terminationOnEachPass           bool
	verticallyCausalContext         bool
	segmentationSymbols             bool
}

// codeBlockStyleFromU8 is CodeBlockStyle::from_u8.
func codeBlockStyleFromU8(value uint8) codeBlockStyle {
	return codeBlockStyle{
		selectiveArithmeticCodingBypass: value&0x01 != 0,
		resetContextProbabilities:       value&0x02 != 0,
		terminationOnEachPass:           value&0x04 != 0,
		verticallyCausalContext:         value&0x08 != 0,
		segmentationSymbols:             value&0x20 != 0,
	}
}

// quantizationStyle is QuantizationStyle in hayro-jpeg2000
// src/j2c/codestream.rs.
type quantizationStyle uint8

// The quantization styles.
const (
	noQuantization quantizationStyle = iota
	scalarDerived
	scalarExpounded
)

// quantizationStyleFromU8 is QuantizationStyle::from_u8.
func quantizationStyleFromU8(value uint8) (quantizationStyle, error) {
	switch value & 0x1f {
	case 0:
		return noQuantization, nil
	case 1:
		return scalarDerived, nil
	case 2:
		return scalarExpounded, nil
	}
	return 0, errInvalidQuantizationStyle
}

// stepSize is StepSize in hayro-jpeg2000 src/j2c/codestream.rs.
type stepSize struct {
	mantissa uint16
	exponent uint16
}

// quantizationInfo is QuantizationInfo in hayro-jpeg2000
// src/j2c/codestream.rs.
type quantizationInfo struct {
	quantizationStyle quantizationStyle
	guardBits         uint8
	stepSizes         []stepSize
}

// codingStyleDefault is CodingStyleDefault in hayro-jpeg2000
// src/j2c/codestream.rs.
type codingStyleDefault struct {
	progressionOrder    progressionOrder
	numLayers           uint8
	mct                 bool
	componentParameters codingStyleComponent
}

// codingStyleComponent is CodingStyleComponent in hayro-jpeg2000
// src/j2c/codestream.rs.
type codingStyleComponent struct {
	flags      codingStyleFlags
	parameters codingStyleParameters
}

// codingStyleParameters is CodingStyleParameters in hayro-jpeg2000
// src/j2c/codestream.rs.
type codingStyleParameters struct {
	numDecompositionLevels uint8
	numResolutionLevels    uint8
	codeBlockWidth         uint8
	codeBlockHeight        uint8
	codeBlockStyle         codeBlockStyle
	transformation         waveletTransform
	// precinctExponents holds the exponents of the width and of the
	// height of the precincts of each resolution.
	precinctExponents [][2]uint8
}

// sizeData is SizeData in hayro-jpeg2000 src/j2c/codestream.rs. It is the
// content of the marker SIZ.
type sizeData struct {
	referenceGridWidth      uint32
	referenceGridHeight     uint32
	imageAreaXOffset        uint32
	imageAreaYOffset        uint32
	tileWidth               uint32
	tileHeight              uint32
	tileXOffset             uint32
	tileYOffset             uint32
	componentSizes          []componentSizeInfo
	xShrinkFactor           uint32
	yShrinkFactor           uint32
	xResolutionShrinkFactor uint32
	yResolutionShrinkFactor uint32
}

// tileXCoord is SizeData::tile_x_coord.
func (s *sizeData) tileXCoord(idx uint32) uint32 {
	return idx % s.numXTiles()
}

// tileYCoord is SizeData::tile_y_coord.
func (s *sizeData) tileYCoord(idx uint32) uint32 {
	return idx / s.numXTiles()
}

// componentSizeInfo is ComponentSizeInfo in hayro-jpeg2000
// src/j2c/codestream.rs.
type componentSizeInfo struct {
	precision            uint8
	horizontalResolution uint8
	verticalResolution   uint8
}

// numXTiles is SizeData::num_x_tiles.
func (s *sizeData) numXTiles() uint32 {
	return divCeil32(s.referenceGridWidth-s.tileXOffset, s.tileWidth)
}

// numYTiles is SizeData::num_y_tiles.
func (s *sizeData) numYTiles() uint32 {
	return divCeil32(s.referenceGridHeight-s.tileYOffset, s.tileHeight)
}

// numTiles is SizeData::num_tiles.
func (s *sizeData) numTiles() (uint32, error) {
	n := uint64(s.numXTiles()) * uint64(s.numYTiles())
	if n > 0xffffffff {
		return 0, errImageTooLarge
	}
	return uint32(n), nil
}

// xDivisor is SizeData::x_divisor.
func (s *sizeData) xDivisor() uint64 {
	return uint64(s.xShrinkFactor) * uint64(s.xResolutionShrinkFactor)
}

// yDivisor is SizeData::y_divisor.
func (s *sizeData) yDivisor() uint64 {
	return uint64(s.yShrinkFactor) * uint64(s.yResolutionShrinkFactor)
}

// imageXOffset is SizeData::image_x_offset.
func (s *sizeData) imageXOffset() uint32 {
	return uint32(divCeil64(uint64(s.imageAreaXOffset), s.xDivisor()))
}

// imageYOffset is SizeData::image_y_offset.
func (s *sizeData) imageYOffset() uint32 {
	return uint32(divCeil64(uint64(s.imageAreaYOffset), s.yDivisor()))
}

// imageWidth is SizeData::image_width.
func (s *sizeData) imageWidth() uint32 {
	return uint32(divCeil64(uint64(s.referenceGridWidth), s.xDivisor())) - s.imageXOffset()
}

// imageHeight is SizeData::image_height.
func (s *sizeData) imageHeight() uint32 {
	return uint32(divCeil64(uint64(s.referenceGridHeight), s.yDivisor())) - s.imageYOffset()
}

// sizeMarker is size_marker in hayro-jpeg2000 src/j2c/codestream.rs.
func sizeMarker(reader *bitReader) (sizeData, error) {
	sd, ok := sizeMarkerInner(reader)
	if !ok {
		return sd, errMarkerParseFailure("SIZ")
	}

	if sd.tileWidth == 0 || sd.tileHeight == 0 || sd.referenceGridWidth == 0 || sd.referenceGridHeight == 0 {
		return sd, errInvalidDimensions
	}
	if sd.tileXOffset >= sd.referenceGridWidth || sd.tileYOffset >= sd.referenceGridHeight {
		return sd, errInvalidDimensions
	}
	if sd.tileXOffset > sd.imageAreaXOffset || sd.tileYOffset > sd.imageAreaYOffset {
		return sd, errTileInvalidOffsets
	}
	xEnd := uint64(sd.tileXOffset) + uint64(sd.tileWidth)
	yEnd := uint64(sd.tileYOffset) + uint64(sd.tileHeight)
	if xEnd > 0xffffffff || yEnd > 0xffffffff {
		return sd, errTileInvalidOffsets
	}
	if xEnd <= uint64(sd.imageAreaXOffset) || yEnd <= uint64(sd.imageAreaYOffset) {
		return sd, errTileInvalidOffsets
	}
	for _, comp := range sd.componentSizes {
		if comp.precision == 0 || comp.verticalResolution == 0 || comp.horizontalResolution == 0 {
			return sd, errInvalidComponentMetadata
		}
	}
	return sd, nil
}

// sizeMarkerInner is size_marker_inner in hayro-jpeg2000
// src/j2c/codestream.rs.
func sizeMarkerInner(reader *bitReader) (sizeData, bool) {
	var sd sizeData
	if _, ok := reader.readU16(); !ok {
		return sd, false
	}
	if _, ok := reader.readU16(); !ok {
		return sd, false
	}
	var vals [8]uint32
	for i := range vals {
		v, ok := reader.readU32()
		if !ok {
			return sd, false
		}
		vals[i] = v
	}
	xsiz, ysiz, xOsiz, yOsiz := vals[0], vals[1], vals[2], vals[3]
	xtSiz, ytSiz, xtoSiz, ytoSiz := vals[4], vals[5], vals[6], vals[7]
	csiz, ok := reader.readU16()
	if !ok {
		return sd, false
	}

	if xOsiz >= xsiz || yOsiz >= ysiz {
		return sd, false
	}
	if csiz == 0 {
		return sd, false
	}

	// Port: upstream reserves csiz entries first. The port grows the slice
	// as it reads them, so that a short marker cannot make it allocate.
	var components []componentSizeInfo
	for range csiz {
		b, ok := reader.readBytes(3)
		if !ok {
			return sd, false
		}
		ssiz, xRsiz, yRsiz := b[0], b[1], b[2]

		precision := ssiz&0x7f + 1
		// Signed samples are treated as unsigned, like other decoders do.

		// The standard allows up to 38 bits. The decoder supports 31.
		if uint32(precision) > bitplaneBitSize {
			return sd, false
		}

		components = append(components, componentSizeInfo{
			precision:            precision,
			horizontalResolution: xRsiz,
			verticalResolution:   yRsiz,
		})
	}

	// Uniformly subsampled images are decoded at component, not
	// reference-grid, resolution.
	xShrinkFactor, yShrinkFactor := uint32(1), uint32(1)
	hr, vr := components[0].horizontalResolution, components[0].verticalResolution
	sameResolution := true
	for _, c := range components[1:] {
		sameResolution = sameResolution && c.horizontalResolution == hr && c.verticalResolution == vr
	}
	if sameResolution {
		xShrinkFactor = uint32(hr)
		yShrinkFactor = uint32(vr)
	}

	return sizeData{
		referenceGridWidth:      xsiz,
		referenceGridHeight:     ysiz,
		imageAreaXOffset:        xOsiz,
		imageAreaYOffset:        yOsiz,
		tileWidth:               xtSiz,
		tileHeight:              ytSiz,
		tileXOffset:             xtoSiz,
		tileYOffset:             ytoSiz,
		componentSizes:          components,
		xShrinkFactor:           xShrinkFactor,
		yShrinkFactor:           yShrinkFactor,
		xResolutionShrinkFactor: 1,
		yResolutionShrinkFactor: 1,
	}, true
}

// codingStyleParametersRead is coding_style_parameters in hayro-jpeg2000
// src/j2c/codestream.rs.
func codingStyleParametersRead(reader *bitReader, flags codingStyleFlags) (codingStyleParameters, bool) {
	var p codingStyleParameters
	numDecompositionLevels, ok := reader.readByte()
	if !ok || numDecompositionLevels > maxResolutionCount {
		return p, false
	}
	numResolutionLevels := numDecompositionLevels + 1

	b, ok := reader.readBytes(4)
	if !ok || b[0] > 0xff-2 || b[1] > 0xff-2 {
		return p, false
	}
	codeBlockWidth, codeBlockHeight := b[0]+2, b[1]+2
	if codeBlockWidth > 10 || codeBlockHeight > 10 || codeBlockWidth+codeBlockHeight > 12 {
		return p, false
	}
	style := codeBlockStyleFromU8(b[2])
	transformation, err := waveletTransformFromU8(b[3])
	if err != nil {
		return p, false
	}

	exponents := make([][2]uint8, 0, numResolutionLevels)
	if flags.hasPrecincts() {
		for range numResolutionLevels {
			size, ok := reader.readByte()
			if !ok {
				return p, false
			}
			widthExp, heightExp := size&0xf, size>>4
			if widthExp > maxPrecinctExponent || heightExp > maxPrecinctExponent {
				return p, false
			}
			exponents = append(exponents, [2]uint8{widthExp, heightExp})
		}
	} else {
		for range numResolutionLevels {
			exponents = append(exponents, [2]uint8{15, 15})
		}
	}

	return codingStyleParameters{
		numDecompositionLevels: numDecompositionLevels,
		numResolutionLevels:    numResolutionLevels,
		codeBlockWidth:         codeBlockWidth,
		codeBlockHeight:        codeBlockHeight,
		codeBlockStyle:         style,
		transformation:         transformation,
		precinctExponents:      exponents,
	}, true
}

// comMarker is com_marker in hayro-jpeg2000 src/j2c/codestream.rs.
func comMarker(reader *bitReader) bool {
	return skipMarkerSegment(reader)
}

// tlmMarker is tlm_marker in hayro-jpeg2000 src/j2c/codestream.rs.
func tlmMarker(reader *bitReader) bool {
	return skipMarkerSegment(reader)
}

// ppmMarker is ppm_marker in hayro-jpeg2000 src/j2c/codestream.rs.
func ppmMarker(reader *bitReader) (ppmMarkerData, bool) {
	length, ok := reader.readU16()
	if !ok || length < 2 {
		return ppmMarkerData{}, false
	}
	data, ok := reader.readBytes(int(length - 2))
	if !ok || len(data) == 0 {
		return ppmMarkerData{}, false
	}
	return ppmMarkerData{sequenceIdx: data[0], data: data[1:]}, true
}

// ppmPackets is ppm_packets in hayro-jpeg2000 src/j2c/codestream.rs. It
// joins the PPM markers and splits them into the packet headers of each
// tile-part.
func ppmPackets(markers []ppmMarkerData) ([][]byte, bool) {
	// The packet headers of a tile-part can continue in the next PPM
	// segment.
	slices.SortStableFunc(markers, func(a, b ppmMarkerData) int {
		return int(a.sequenceIdx) - int(b.sequenceIdx)
	})
	var data []byte
	for _, m := range markers {
		data = append(data, m.data...)
	}

	reader := newBitReader(data)
	var packets [][]byte
	for !reader.atEnd() {
		length, ok := reader.readU32()
		if !ok {
			return nil, false
		}
		b, ok := reader.readBytes(int(length))
		if !ok {
			return nil, false
		}
		packets = append(packets, b)
	}
	return packets, true
}

// readComponentIndex reads the index of a component: one byte when there
// are fewer than 257 components, and two bytes otherwise.
func readComponentIndex(reader *bitReader, csiz uint16) (uint16, bool) {
	if csiz < 257 {
		b, ok := reader.readByte()
		return uint16(b), ok
	}
	return reader.readU16()
}

// rgnMarker is rgn_marker in hayro-jpeg2000 src/j2c/codestream.rs.
func rgnMarker(reader *bitReader, csiz uint16) (componentIndex uint16, roiShift uint8, ok bool) {
	if _, ok := reader.readU16(); !ok {
		return 0, 0, false
	}
	componentIndex, ok = readComponentIndex(reader, csiz)
	if !ok {
		return 0, 0, false
	}
	b, ok := reader.readBytes(2)
	if !ok {
		return 0, 0, false
	}
	if b[0] == 0 {
		roiShift = b[1]
	}
	return componentIndex, roiShift, true
}

// skipMarkerSegment is skip_marker_segment in hayro-jpeg2000
// src/j2c/codestream.rs.
func skipMarkerSegment(reader *bitReader) bool {
	length, ok := reader.readU16()
	if !ok || length < 2 {
		return false
	}
	return reader.skipBytes(int(length - 2))
}

// codMarker is cod_marker in hayro-jpeg2000 src/j2c/codestream.rs.
func codMarker(reader *bitReader) (codingStyleDefault, bool) {
	var c codingStyleDefault
	if _, ok := reader.readU16(); !ok {
		return c, false
	}
	b, ok := reader.readBytes(2)
	if !ok {
		return c, false
	}
	flags := codingStyleFlags{raw: b[0]}
	order, err := progressionOrderFromU8(b[1])
	if err != nil {
		return c, false
	}
	numLayers, ok := reader.readU16()
	if !ok || numLayers == 0 || numLayers > maxLayerCount {
		return c, false
	}
	mct, ok := reader.readByte()
	if !ok {
		return c, false
	}
	params, ok := codingStyleParametersRead(reader, flags)
	if !ok {
		return c, false
	}
	return codingStyleDefault{
		progressionOrder: order,
		numLayers:        uint8(numLayers),
		mct:              mct == 1,
		componentParameters: codingStyleComponent{
			flags:      flags,
			parameters: params,
		},
	}, true
}

// cocMarker is coc_marker in hayro-jpeg2000 src/j2c/codestream.rs.
func cocMarker(reader *bitReader, csiz uint16) (uint16, codingStyleComponent, bool) {
	if _, ok := reader.readU16(); !ok {
		return 0, codingStyleComponent{}, false
	}
	idx, ok := readComponentIndex(reader, csiz)
	if !ok {
		return 0, codingStyleComponent{}, false
	}
	raw, ok := reader.readByte()
	if !ok {
		return 0, codingStyleComponent{}, false
	}
	flags := codingStyleFlags{raw: raw}
	params, ok := codingStyleParametersRead(reader, flags)
	if !ok {
		return 0, codingStyleComponent{}, false
	}
	return idx, codingStyleComponent{flags: flags, parameters: params}, true
}

// pocMarker is poc_marker in hayro-jpeg2000 src/j2c/codestream.rs.
func pocMarker(outer *bitReader, csiz uint16) ([]progressionChange, bool) {
	length, ok := outer.readU16()
	if !ok || length < 2 {
		return nil, false
	}
	length -= 2
	data, ok := outer.readBytes(int(length))
	if !ok {
		return nil, false
	}
	reader := newBitReader(data)

	entrySize := 7
	if csiz >= 257 {
		entrySize = 9
	}

	n := int(length) / entrySize
	changes := make([]progressionChange, 0, n)
	for range n {
		var c progressionChange
		if c.resolutionStart, ok = reader.readByte(); !ok {
			return nil, false
		}
		if c.componentStart, ok = readComponentIndex(&reader, csiz); !ok {
			return nil, false
		}
		if c.layerEnd, ok = reader.readU16(); !ok {
			return nil, false
		}
		if c.resolutionEnd, ok = reader.readByte(); !ok {
			return nil, false
		}
		if c.componentEnd, ok = readComponentIndex(&reader, csiz); !ok {
			return nil, false
		}
		if c.componentEnd == 0 {
			c.componentEnd = 256
		}
		order, ok := reader.readByte()
		if !ok {
			return nil, false
		}
		var err error
		if c.progressionOrder, err = progressionOrderFromU8(order); err != nil {
			return nil, false
		}
		changes = append(changes, c)
	}
	return changes, true
}

// qcdMarker is qcd_marker in hayro-jpeg2000 src/j2c/codestream.rs.
func qcdMarker(reader *bitReader) (quantizationInfo, bool) {
	length, ok := reader.readU16()
	if !ok {
		return quantizationInfo{}, false
	}
	sqcd, ok := reader.readByte()
	if !ok {
		return quantizationInfo{}, false
	}
	style, err := quantizationStyleFromU8(sqcd & 0x1f)
	if err != nil || length < 3 {
		return quantizationInfo{}, false
	}
	params, ok := quantizationParameters(reader, style, int(length-3))
	if !ok {
		return quantizationInfo{}, false
	}
	params.guardBits = sqcd >> 5 & 0x07
	return params, true
}

// qccMarker is qcc_marker in hayro-jpeg2000 src/j2c/codestream.rs.
func qccMarker(reader *bitReader, csiz uint16) (uint16, quantizationInfo, bool) {
	length, ok := reader.readU16()
	if !ok {
		return 0, quantizationInfo{}, false
	}
	idx, ok := readComponentIndex(reader, csiz)
	if !ok {
		return 0, quantizationInfo{}, false
	}
	sqcc, ok := reader.readByte()
	if !ok {
		return 0, quantizationInfo{}, false
	}
	style, err := quantizationStyleFromU8(sqcc & 0x1f)
	if err != nil {
		return 0, quantizationInfo{}, false
	}
	indexSize := uint16(1)
	if csiz >= 257 {
		indexSize = 2
	}
	if length < 2+indexSize+1 {
		return 0, quantizationInfo{}, false
	}
	params, ok := quantizationParameters(reader, style, int(length-2-indexSize-1))
	if !ok {
		return 0, quantizationInfo{}, false
	}
	params.guardBits = sqcc >> 5 & 0x07
	return idx, params, true
}

// quantizationParameters is quantization_parameters in hayro-jpeg2000
// src/j2c/codestream.rs.
func quantizationParameters(reader *bitReader, style quantizationStyle, remainingBytes int) (quantizationInfo, bool) {
	var steps []stepSize
	irreversible := func(val uint16) stepSize {
		return stepSize{exponent: val >> 11, mantissa: val & (1<<11 - 1)}
	}

	switch style {
	case noQuantization:
		for range remainingBytes {
			value, ok := reader.readByte()
			if !ok {
				return quantizationInfo{}, false
			}
			steps = append(steps, stepSize{exponent: uint16(value >> 3)})
		}
	case scalarDerived:
		value, ok := reader.readU16()
		if !ok {
			return quantizationInfo{}, false
		}
		steps = append(steps, irreversible(value))
	case scalarExpounded:
		for range remainingBytes / 2 {
			value, ok := reader.readU16()
			if !ok {
				return quantizationInfo{}, false
			}
			steps = append(steps, irreversible(value))
		}
	}
	return quantizationInfo{quantizationStyle: style, stepSizes: steps}, true
}

// The codes of the markers, from the module markers in hayro-jpeg2000
// src/j2c/codestream.rs.
const (
	markerSOC = 0x4f
	markerSOT = 0x90
	markerSOD = 0x93
	markerEOC = 0xd9

	markerSIZ = 0x51

	markerCOD = 0x52
	markerCOC = 0x53
	markerRGN = 0x5e
	markerQCD = 0x5c
	markerQCC = 0x5d
	markerPOC = 0x5f

	markerTLM = 0x55
	markerPLM = 0x57
	markerPLT = 0x58
	markerPPM = 0x60
	markerPPT = 0x61

	markerSOP = 0x91
	markerEPH = 0x92

	markerCRG = 0x63
	markerCOM = 0x64
)
