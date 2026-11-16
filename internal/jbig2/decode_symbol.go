// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/decode/symbol.rs, the symbol dictionary
// decoding procedure of T.88 6.5.

import "math"

// maxSymbols is MAX_SYMBOLS, the largest number of symbols of a dictionary
// or of a region.
const maxSymbols uint32 = 1 << 20

// decodeSymbolDictionary is decode in hayro-jbig2 src/decode/symbol.rs.
func decodeSymbolDictionary(header *symbolDictionaryHeader, inputSymbols []*bitmap, referredTables []*huffmanTable,
	standardTables standardHuffmanTables, inputContexts *retainedContexts, budget *memoryBudget, c cancel) (*symbolDictionary, error) {
	numNewSymbols := header.numNewSymbols
	capacity := min(int(numNewSymbols), len(header.data))

	aCtx, err := newSymbolArithmeticContext(header.data, header, inputContexts)
	if err != nil {
		return nil, err
	}
	hCtx, err := newSymbolHuffmanContext(header.data, header, referredTables, standardTables)
	if err != nil {
		return nil, err
	}

	ctx := &symbolDecodeContext{
		header:         header,
		aCtx:           aCtx,
		hCtx:           hCtx,
		symbols:        newSymbols(inputSymbols, capacity),
		standardTables: standardTables,
		budget:         budget,
		cancel:         c,
		symbolWidths:   make([]uint32, 0, capacity),
	}

	useHuffman := header.flags.useHuffman

	readHeightClassDelta := func() (int32, bool, error) {
		if useHuffman {
			return ctx.hCtx.heightClassDeltaTable.decode(&ctx.hCtx.reader)
		}
		v, ok := ctx.aCtx.heightClassDeltaDecoder.decode(ctx.aCtx.decoder)
		return v, ok, nil
	}

	readSymbolWidthDelta := func() (int32, bool, error) {
		if useHuffman {
			return ctx.hCtx.symbolWidthDeltaTable.decode(&ctx.hCtx.reader)
		}
		v, ok := ctx.aCtx.symbolWidthDeltaDecoder.decode(ctx.aCtx.decoder)
		return v, ok, nil
	}

	remainingEmptyHeightClasses := numNewSymbols

	var work workCounter

	for ctx.symbolsDecodedCount < numNewSymbols {
		if err := work.add(64, c); err != nil {
			return nil, err
		}

		heightClassDelta, ok, err := readHeightClassDelta()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrSymbolUnexpectedOOB
		}

		if ctx.heightClassHeight, ok = checkedAddSigned(ctx.heightClassHeight, heightClassDelta); !ok {
			return nil, ErrOverflowBitmapDimension
		}

		var symbolWidth uint32
		ctx.totalWidth = 0
		ctx.heightClassFirstSymbol = ctx.symbolsDecodedCount

		for {
			widthDelta, ok, err := readSymbolWidthDelta()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}

			// The check stops an endless loop for a file that is not valid.
			if ctx.symbolsDecodedCount >= numNewSymbols {
				return nil, ErrTooManySymbols
			}

			if err := work.add(64, c); err != nil {
				return nil, err
			}

			if symbolWidth, ok = checkedAddSigned(symbolWidth, widthDelta); !ok {
				return nil, ErrOverflowBitmapDimension
			}
			if ctx.totalWidth > math.MaxUint32-symbolWidth {
				return nil, ErrOverflowBitmapDimension
			}
			ctx.totalWidth += symbolWidth

			if !useHuffman || header.flags.useRefagg {
				if err := work.add(uint64(symbolWidth)*uint64(ctx.heightClassHeight), c); err != nil {
					return nil, err
				}
			}

			switch {
			case !useHuffman && !header.flags.useRefagg:
				region, err := newBitmap(symbolWidth, ctx.heightClassHeight, budget)
				if err != nil {
					return nil, err
				}
				if err := decodeBitmapArithmeticCoding(region, ctx.aCtx.decoder, ctx.aCtx.genericRegionContexts,
					header.flags.template, false, &header.adaptiveTemplatePixels, c); err != nil {
					return nil, err
				}

				ctx.symbols.new = append(ctx.symbols.new, region)
			case useHuffman && !header.flags.useRefagg:
				ctx.symbolWidths = append(ctx.symbolWidths, symbolWidth)
			default:
				symbol, err := decodeRefinementAggregationBitmap(ctx, symbolWidth)
				if err != nil {
					return nil, err
				}

				ctx.symbols.new = append(ctx.symbols.new, symbol)
			}

			ctx.symbolsDecodedCount++
		}

		if ctx.symbolsDecodedCount == ctx.heightClassFirstSymbol {
			if remainingEmptyHeightClasses == 0 {
				return nil, ErrSymbolInvalid
			}
			remainingEmptyHeightClasses--
		}

		if useHuffman && !header.flags.useRefagg {
			if err := decodeHeightClassCollectiveBitmap(ctx); err != nil {
				return nil, err
			}
		}
	}

	exported, err := exportSymbols(ctx)
	if err != nil {
		return nil, err
	}

	var retained *retainedContexts
	if header.flags.bitmapContextRetained {
		retained = &retainedContexts{
			template:           header.flags.template,
			refinementTemplate: header.flags.refinementTemplate,
			genericRegion:      ctx.aCtx.genericRegionContexts,
			refinementRegion:   ctx.aCtx.refinementRegionContexts,
		}
	}

	return &symbolDictionary{
		exportedSymbols:  exported,
		retainedContexts: retained,
	}, nil
}

// checkedAddSigned is u32::checked_add_signed of the Rust core library.
func checkedAddSigned(u uint32, d int32) (uint32, bool) {
	s := int64(u) + int64(d)
	if s < 0 || s > math.MaxUint32 {
		return 0, false
	}
	return uint32(s), true
}

// retainedContexts is RetainedContexts in hayro-jbig2 src/decode/symbol.rs,
// the arithmetic contexts that a later dictionary can reuse.
type retainedContexts struct {
	template           template
	refinementTemplate refinementTemplate
	genericRegion      []arithmeticDecoderContext
	refinementRegion   []arithmeticDecoderContext
}

// symbolDictionary is SymbolDictionary in hayro-jbig2 src/decode/symbol.rs.
// A nil retainedContexts is the None of upstream.
type symbolDictionary struct {
	exportedSymbols  []*bitmap
	retainedContexts *retainedContexts
}

// decodeRefinementAggregationBitmap is decode_refinement_aggregation_bitmap
// in hayro-jbig2 src/decode/symbol.rs.
func decodeRefinementAggregationBitmap(ctx *symbolDecodeContext, symbolWidth uint32) (*bitmap, error) {
	var count int32
	var ok bool
	if ctx.header.flags.useHuffman {
		var err error
		count, ok, err = ctx.hCtx.aggregationInstanceCountTable.decode(&ctx.hCtx.reader)
		if err != nil {
			return nil, err
		}
	} else {
		count, ok = ctx.aCtx.aggregationInstanceCountDecoder.decode(ctx.aCtx.decoder)
	}
	if !ok {
		return nil, ErrSymbolUnexpectedOOB
	}

	switch {
	case count == 1:
		return decodeSymbolRefinementBitmap(ctx, symbolWidth)
	case count > 1:
		return decodeAggregationBitmap(ctx, symbolWidth, uint32(count))
	}
	return nil, ErrSymbolInvalid
}

// decodeSymbolRefinementBitmap is decode_refinement_bitmap in hayro-jbig2
// src/decode/symbol.rs. It decodes a symbol as the refinement of one other
// symbol.
func decodeSymbolRefinementBitmap(ctx *symbolDecodeContext, symbolWidth uint32) (*bitmap, error) {
	useHuffman := ctx.header.flags.useHuffman
	symbolCodeLength := uint32(bitLen32(ctx.totalSymbols() - 1))

	var symbolID int
	var refinementXOffset, refinementYOffset int32
	if useHuffman {
		symbolCodeLength = max(symbolCodeLength, 1)

		id, ok := ctx.hCtx.reader.readBits(uint8(symbolCodeLength))
		if !ok {
			return nil, ErrUnexpectedEOF
		}
		symbolID = int(id)

		var err error
		if refinementXOffset, err = ctx.standardTables.tableO().decodeNoOOB(&ctx.hCtx.reader); err != nil {
			return nil, err
		}
		if refinementYOffset, err = ctx.standardTables.tableO().decodeNoOOB(&ctx.hCtx.reader); err != nil {
			return nil, err
		}
	} else {
		// The contexts are shared across all bitmaps of the dictionary.
		if ctx.aCtx.textRegionContexts == nil {
			ctx.aCtx.textRegionContexts = newTextRegionContexts(symbolCodeLength)
		}
		contexts := ctx.aCtx.textRegionContexts

		symbolID = int(contexts.iaid.decode(ctx.aCtx.decoder))

		var ok bool
		if refinementXOffset, ok = contexts.iardx.decode(ctx.aCtx.decoder); !ok {
			return nil, ErrSymbolUnexpectedOOB
		}
		if refinementYOffset, ok = contexts.iardy.decode(ctx.aCtx.decoder); !ok {
			return nil, ErrSymbolUnexpectedOOB
		}
	}

	referenceRegion := ctx.symbols.get(symbolID)
	if referenceRegion == nil {
		return nil, ErrSymbolOutOfRange
	}
	region, err := newBitmap(symbolWidth, ctx.heightClassHeight, ctx.budget)
	if err != nil {
		return nil, err
	}

	decoder := ctx.aCtx.decoder
	if useHuffman {
		size, err := ctx.standardTables.tableA().decodeNoOOB(&ctx.hCtx.reader)
		if err != nil {
			return nil, err
		}
		ctx.hCtx.reader.align()

		// Upstream casts the size to a usize, so a negative size is too
		// large.
		n := int(size)
		if size < 0 {
			n = math.MaxInt
		}
		data, ok := ctx.hCtx.reader.readBytes(n)
		if !ok {
			return nil, ErrUnexpectedEOF
		}

		decoder = newArithmeticDecoder(data)
	}

	if err := decodeRefinementBitmap(decoder, ctx.aCtx.refinementRegionContexts, region, referenceRegion,
		refinementXOffset, refinementYOffset, ctx.header.flags.refinementTemplate,
		ctx.header.refinementATPixels, false, ctx.cancel); err != nil {
		return nil, err
	}

	return region, nil
}

// decodeAggregationBitmap is decode_aggregation_bitmap in hayro-jbig2
// src/decode/symbol.rs. It decodes a symbol as a text region of other
// symbols.
func decodeAggregationBitmap(ctx *symbolDecodeContext, symbolWidth, aggregationInstanceCount uint32) (*bitmap, error) {
	useHuffman := ctx.header.flags.useHuffman

	symbolCodeLength := uint32(bitLen32(ctx.totalSymbols() - 1))

	var symbolIDTable *huffmanTable
	var huffmanFlags *textRegionHuffmanFlags
	if useHuffman {
		if ctx.hCtx.aggregationSymbolCodes == nil {
			table, err := buildUniformHuffmanTable(ctx.totalSymbols(), max(symbolCodeLength, 1))
			if err != nil {
				return nil, err
			}
			ctx.hCtx.aggregationSymbolCodes = table
		}

		symbolIDTable = ctx.hCtx.aggregationSymbolCodes

		huffmanFlags = &textRegionHuffmanFlags{
			firstSTable:           0,
			deltaSTable:           0,
			deltaTTable:           0,
			refinementWidthTable:  1,
			refinementHeightTable: 1,
			refinementXTable:      1,
			refinementYTable:      1,
			refinementSizeTable:   0,
		}
	}

	header := &textRegionHeader{
		regionInfo: regionSegmentInfo{
			width:               symbolWidth,
			height:              ctx.heightClassHeight,
			combinationOperator: combinationOr,
		},
		flags: textRegionFlags{
			useHuffman:          useHuffman,
			useRefinement:       true,
			logStripSize:        0,
			referenceCorner:     cornerTopLeft,
			transposed:          false,
			combinationOperator: combinationOr,
			defaultPixel:        false,
			deltaSOffset:        0,
			refinementTemplate:  ctx.header.flags.refinementTemplate,
		},
		huffmanFlags:       huffmanFlags,
		refinementATPixels: ctx.header.refinementATPixels,
		numInstances:       aggregationInstanceCount,
		symbolIDTable:      symbolIDTable,
		// The data comes from the decoder of the dictionary, not from the
		// header.
		data:   nil,
		strict: ctx.header.strict,
	}

	var decodeCtx *decodeContext
	if useHuffman {
		var err error
		decodeCtx, err = newHuffmanDecodeContext(&ctx.hCtx.reader, header, nil, ctx.standardTables,
			ctx.aCtx.refinementRegionContexts)
		if err != nil {
			return nil, err
		}
	} else {
		if ctx.aCtx.textRegionContexts == nil {
			ctx.aCtx.textRegionContexts = newTextRegionContexts(symbolCodeLength)
		}
		decodeCtx = newArithmeticDecodeContext(ctx.aCtx.decoder, ctx.aCtx.textRegionContexts,
			ctx.aCtx.refinementRegionContexts)
	}

	info := &header.regionInfo
	bm, err := newBitmapWith(info.width, info.height, info.xLocation, info.yLocation,
		header.flags.defaultPixel, ctx.budget)
	if err != nil {
		return nil, err
	}
	if err := decodeWith(decodeCtx, symbolBitmaps{first: ctx.symbols.input, second: ctx.symbols.new},
		header, bm, ctx.budget, ctx.cancel); err != nil {
		return nil, err
	}
	return bm, nil
}

// symbols is Symbols in hayro-jbig2 src/decode/symbol.rs, the input
// symbols of a dictionary and its new symbols.
type symbols struct {
	input []*bitmap
	new   []*bitmap
}

// newSymbols is Symbols::new.
func newSymbols(input []*bitmap, capacity int) symbols {
	return symbols{input: input, new: make([]*bitmap, 0, capacity)}
}

// inputCount is Symbols::input_count.
func (s *symbols) inputCount() uint32 {
	return uint32(len(s.input))
}

// get is Symbols::get. It returns nil for an index out of range.
func (s *symbols) get(index int) *bitmap {
	if index < 0 {
		return nil
	}
	if index < len(s.input) {
		return s.input[index]
	}
	if i := index - len(s.input); i < len(s.new) {
		return s.new[i]
	}
	return nil
}

// symbolDecodeContext is SymbolDecodeContext in hayro-jbig2
// src/decode/symbol.rs, the state of the decoding of a dictionary.
type symbolDecodeContext struct {
	header                 *symbolDictionaryHeader
	aCtx                   *symbolArithmeticContext
	hCtx                   *symbolHuffmanContext
	symbols                symbols
	standardTables         standardHuffmanTables
	budget                 *memoryBudget
	cancel                 cancel
	symbolWidths           []uint32
	heightClassFirstSymbol uint32
	symbolsDecodedCount    uint32
	totalWidth             uint32
	heightClassHeight      uint32
}

// totalSymbols is SymbolDecodeContext::total_symbols.
func (ctx *symbolDecodeContext) totalSymbols() uint32 {
	return ctx.symbols.inputCount() + ctx.header.numNewSymbols
}

// symbolArithmeticContext is ArithmeticContext in hayro-jbig2
// src/decode/symbol.rs. A nil textRegionContexts is the None of upstream.
type symbolArithmeticContext struct {
	decoder                         *arithmeticDecoder
	heightClassDeltaDecoder         *integerDecoder
	symbolWidthDeltaDecoder         *integerDecoder
	exportRunLengthDecoder          *integerDecoder
	aggregationInstanceCountDecoder *integerDecoder
	genericRegionContexts           []arithmeticDecoderContext
	refinementRegionContexts        []arithmeticDecoderContext
	textRegionContexts              *textRegionContexts
}

// newSymbolArithmeticContext is ArithmeticContext::new. It takes the
// retained contexts of an earlier dictionary when the header asks for them.
func newSymbolArithmeticContext(data []byte, header *symbolDictionaryHeader, inputContexts *retainedContexts) (*symbolArithmeticContext, error) {
	var genericRegion, refinementRegion []arithmeticDecoderContext
	if header.flags.bitmapContextUsed {
		in := inputContexts
		if in == nil {
			return nil, ErrSymbolInvalid
		}

		if in.template != header.flags.template || in.refinementTemplate != header.flags.refinementTemplate {
			return nil, ErrSymbolInvalid
		}

		genericRegion = append([]arithmeticDecoderContext(nil), in.genericRegion...)
		refinementRegion = append([]arithmeticDecoderContext(nil), in.refinementRegion...)
	} else {
		genericRegion = make([]arithmeticDecoderContext, 1<<header.flags.template.contextBits())
		refinementRegion = make([]arithmeticDecoderContext, 1<<header.flags.refinementTemplate.contextBits())
	}

	return &symbolArithmeticContext{
		decoder:                         newArithmeticDecoder(data),
		heightClassDeltaDecoder:         newIntegerDecoder(),
		symbolWidthDeltaDecoder:         newIntegerDecoder(),
		exportRunLengthDecoder:          newIntegerDecoder(),
		aggregationInstanceCountDecoder: newIntegerDecoder(),
		genericRegionContexts:           genericRegion,
		refinementRegionContexts:        refinementRegion,
	}, nil
}

// symbolHuffmanContext is HuffmanContext in hayro-jbig2 src/decode/symbol.rs.
// A nil aggregationSymbolCodes is the None of upstream.
type symbolHuffmanContext struct {
	heightClassDeltaTable         *huffmanTable
	symbolWidthDeltaTable         *huffmanTable
	collectiveBitmapSizeTable     *huffmanTable
	aggregationInstanceCountTable *huffmanTable
	exportRunLengthTable          *huffmanTable
	aggregationSymbolCodes        *huffmanTable
	reader                        reader
}

// newSymbolHuffmanContext is HuffmanContext::new. It takes the
// user-supplied tables in order.
func newSymbolHuffmanContext(data []byte, header *symbolDictionaryHeader, referredTables []*huffmanTable,
	standardTables standardHuffmanTables) (*symbolHuffmanContext, error) {
	customCount := 0
	for _, s := range []huffmanTableSelection{
		header.flags.deltaHeightTable,
		header.flags.deltaWidthTable,
		header.flags.collectiveBitmapSizeTable,
		header.flags.aggregateInstanceTable,
	} {
		if s == tableUserSupplied {
			customCount++
		}
	}

	if len(referredTables) < customCount {
		return nil, ErrHuffmanMissingTables
	}

	customTableIdx := 0
	getTable := func(selection huffmanTableSelection) *huffmanTable {
		switch selection {
		case tableB1:
			return standardTables.tableA()
		case tableB2:
			return standardTables.tableB()
		case tableB3:
			return standardTables.tableC()
		case tableB4:
			return standardTables.tableD()
		case tableB5:
			return standardTables.tableE()
		}
		table := referredTables[customTableIdx]
		customTableIdx++
		return table
	}

	h := &symbolHuffmanContext{reader: newReader(data)}
	h.heightClassDeltaTable = getTable(header.flags.deltaHeightTable)
	h.symbolWidthDeltaTable = getTable(header.flags.deltaWidthTable)
	h.collectiveBitmapSizeTable = getTable(header.flags.collectiveBitmapSizeTable)
	h.aggregationInstanceCountTable = getTable(header.flags.aggregateInstanceTable)
	h.exportRunLengthTable = getTable(tableB1)
	return h, nil
}

// decodeHeightClassCollectiveBitmap is decode_height_class_collective_bitmap
// in hayro-jbig2 src/decode/symbol.rs. It decodes the collective bitmap of a
// height class, and it cuts the symbols of the class from it.
func decodeHeightClassCollectiveBitmap(ctx *symbolDecodeContext) error {
	bitmapSize, err := ctx.hCtx.collectiveBitmapSizeTable.decodeNoOOB(&ctx.hCtx.reader)
	if err != nil {
		return err
	}
	if bitmapSize < 0 {
		return ErrOverflowBitmapDimension
	}

	ctx.hCtx.reader.align()

	var collective *bitmap
	if bitmapSize == 0 {
		rowBytes := int(divCeil(ctx.totalWidth, 8))
		data, ok := ctx.hCtx.reader.readBytes(rowBytes * int(ctx.heightClassHeight))
		if !ok {
			return ErrUnexpectedEOF
		}

		collective, err = newBitmap(ctx.totalWidth, ctx.heightClassHeight, ctx.budget)
		if err != nil {
			return err
		}
		checkMask := rowMask(ctx.totalWidth)

		for y := range ctx.heightClassHeight {
			if y&checkMask == checkMask {
				if err := ctx.cancel.check(); err != nil {
					return err
				}
			}

			row := data[int(y)*rowBytes:][:rowBytes]
			for byteX, b := range row {
				for bit := range uint32(8) {
					x := uint32(byteX)*8 + bit
					if x < ctx.totalWidth {
						collective.setPixel(x, y, (b>>(7-bit))&1)
					}
				}
			}
		}
	} else {
		data, ok := ctx.hCtx.reader.readBytes(int(bitmapSize))
		if !ok {
			return ErrUnexpectedEOF
		}

		collective, err = newBitmap(ctx.totalWidth, ctx.heightClassHeight, ctx.budget)
		if err != nil {
			return err
		}
		if _, err := decodeBitmapMMR(collective, data, ctx.header.strict, ctx.cancel); err != nil {
			return err
		}
	}

	var xOffset uint32
	var work workCounter

	for symbolIdx := ctx.heightClassFirstSymbol; symbolIdx < ctx.symbolsDecodedCount; symbolIdx++ {
		symbolWidth := ctx.symbolWidths[symbolIdx]
		if err := work.add(uint64(max(copyWork(symbolWidth), 1))*uint64(ctx.heightClassHeight), ctx.cancel); err != nil {
			return err
		}
		symbol, err := newBitmap(symbolWidth, ctx.heightClassHeight, ctx.budget)
		if err != nil {
			return err
		}

		for y := range ctx.heightClassHeight {
			srcBitOffset := xOffset % wordBits
			srcWordStart := xOffset / wordBits
			destRow := int(y * symbol.stride)

			for w := range symbol.stride {
				sw := srcWordStart + w
				src1 := collective.getWord(y, sw)
				wd := src1
				if srcBitOffset != 0 {
					src2 := collective.getWord(y, sw+1)
					wd = src1<<srcBitOffset | src2>>(wordBits-srcBitOffset)
				}
				symbol.data[destRow+int(w)] = wd
			}

			if trailing := symbolWidth % wordBits; trailing != 0 && symbol.stride > 0 {
				last := destRow + int(symbol.stride) - 1
				symbol.data[last] &= ^word(0) << (wordBits - trailing)
			}
		}

		ctx.symbols.new = append(ctx.symbols.new, symbol)
		if xOffset > math.MaxUint32-symbolWidth {
			return ErrOverflowIndex
		}
		xOffset += symbolWidth
	}

	return nil
}

// exportSymbols is export_symbols in hayro-jbig2 src/decode/symbol.rs. It
// reads the runs of the export flags (T.88 6.5.10).
func exportSymbols(ctx *symbolDecodeContext) ([]*bitmap, error) {
	totalSymbols := ctx.totalSymbols()

	readRunLength := func() (uint32, error) {
		var value int32
		var ok bool
		if ctx.header.flags.useHuffman {
			var err error
			value, ok, err = ctx.hCtx.exportRunLengthTable.decode(&ctx.hCtx.reader)
			if err != nil {
				return 0, err
			}
		} else {
			value, ok = ctx.aCtx.exportRunLengthDecoder.decode(ctx.aCtx.decoder)
		}
		if !ok {
			return 0, ErrHuffmanUnexpectedOOB
		}
		if value < 0 {
			return 0, ErrHuffmanInvalidCode
		}
		return uint32(value), nil
	}

	exported := make([]*bitmap, 0, min(ctx.header.numExportedSymbols, totalSymbols))
	var index uint32
	shouldExport := false
	remainingRuns := uint64(totalSymbols)*2 + 2
	if remainingRuns > math.MaxUint32 {
		remainingRuns = math.MaxUint32
	}

	for index < totalSymbols {
		if remainingRuns == 0 {
			return nil, ErrSymbolInvalid
		}
		remainingRuns--
		runLength, err := readRunLength()
		if err != nil {
			return nil, err
		}

		if index > math.MaxUint32-runLength {
			return nil, ErrOverflowIndex
		}
		endIndex := index + runLength
		if endIndex > totalSymbols {
			return nil, ErrSymbolOutOfRange
		}

		if shouldExport {
			for symbolIdx := index; symbolIdx < endIndex; symbolIdx++ {
				symbol := ctx.symbols.get(int(symbolIdx))
				if symbol == nil {
					return nil, ErrSymbolOutOfRange
				}
				if symbolIdx >= ctx.symbols.inputCount() {
					ctx.budget.retain(symbol.memorySize())
				}

				exported = append(exported, symbol)
			}
		}

		index = endIndex
		shouldExport = !shouldExport
	}

	if len(exported) != int(ctx.header.numExportedSymbols) {
		return nil, ErrNoSymbols
	}

	return exported, nil
}

// huffmanTableSelection is HuffmanTableSelection in hayro-jbig2
// src/decode/symbol.rs.
type huffmanTableSelection uint8

// The values of huffmanTableSelection.
const (
	tableB1 huffmanTableSelection = iota
	tableB2
	tableB3
	tableB4
	tableB5
	tableUserSupplied
)

// symbolDictionaryFlags is SymbolDictionaryFlags in hayro-jbig2
// src/decode/symbol.rs.
type symbolDictionaryFlags struct {
	useHuffman                bool
	useRefagg                 bool
	deltaHeightTable          huffmanTableSelection
	deltaWidthTable           huffmanTableSelection
	collectiveBitmapSizeTable huffmanTableSelection
	aggregateInstanceTable    huffmanTableSelection
	bitmapContextUsed         bool
	bitmapContextRetained     bool
	template                  template
	refinementTemplate        refinementTemplate
}

// symbolDictionaryHeader is SymbolDictionaryHeader in hayro-jbig2
// src/decode/symbol.rs.
type symbolDictionaryHeader struct {
	flags                  symbolDictionaryFlags
	adaptiveTemplatePixels [4]adaptiveTemplatePixel
	refinementATPixels     []adaptiveTemplatePixel
	numExportedSymbols     uint32
	numNewSymbols          uint32
	data                   []byte
	strict                 bool
}

// parseSymbolDictionary is parse in hayro-jbig2 src/decode/symbol.rs.
func parseSymbolDictionary(r *reader, strict bool) (*symbolDictionaryHeader, error) {
	flagsWord, ok := r.readU16()
	if !ok {
		return nil, ErrUnexpectedEOF
	}
	useHuffman := flagsWord&0x0001 != 0
	useRefagg := flagsWord&0x0002 != 0

	var deltaHeightTable huffmanTableSelection
	switch (flagsWord >> 2) & 0x03 {
	case 0:
		deltaHeightTable = tableB4
	case 1:
		deltaHeightTable = tableB5
	case 3:
		deltaHeightTable = tableUserSupplied
	default:
		return nil, ErrHuffmanInvalidSelection
	}

	var deltaWidthTable huffmanTableSelection
	switch (flagsWord >> 4) & 0x03 {
	case 0:
		deltaWidthTable = tableB2
	case 1:
		deltaWidthTable = tableB3
	case 3:
		deltaWidthTable = tableUserSupplied
	default:
		return nil, ErrHuffmanInvalidSelection
	}

	collectiveBitmapSizeTable := tableB1
	if flagsWord&0x0040 != 0 {
		collectiveBitmapSizeTable = tableUserSupplied
	}

	aggregateInstanceTable := tableB1
	if flagsWord&0x0080 != 0 {
		aggregateInstanceTable = tableUserSupplied
	}

	flags := symbolDictionaryFlags{
		useHuffman:                useHuffman,
		useRefagg:                 useRefagg,
		deltaHeightTable:          deltaHeightTable,
		deltaWidthTable:           deltaWidthTable,
		collectiveBitmapSizeTable: collectiveBitmapSizeTable,
		aggregateInstanceTable:    aggregateInstanceTable,
		bitmapContextUsed:         flagsWord&0x0100 != 0,
		bitmapContextRetained:     flagsWord&0x0200 != 0,
		template:                  templateFromByte(uint8(flagsWord >> 10)),
		refinementTemplate:        refinementTemplateFromByte(uint8(flagsWord >> 12)),
	}

	var at [4]adaptiveTemplatePixel
	if !useHuffman {
		var err error
		at, err = parseAdaptiveTemplatePixels(r, flags.template)
		if err != nil {
			return nil, err
		}
	}

	var refinementATPixels []adaptiveTemplatePixel
	if useRefagg && flags.refinementTemplate == refinementTemplate0 {
		var err error
		refinementATPixels, err = parseRefinementATPixels(r)
		if err != nil {
			return nil, err
		}
	}
	numExportedSymbols, ok1 := r.readU32()
	numNewSymbols, ok2 := r.readU32()
	if !ok1 || !ok2 {
		return nil, ErrUnexpectedEOF
	}

	if numExportedSymbols > maxSymbols || numNewSymbols > maxSymbols {
		return nil, ErrOverflowSymbolCount
	}

	data, ok := r.tail()
	if !ok {
		return nil, ErrUnexpectedEOF
	}

	return &symbolDictionaryHeader{
		flags:                  flags,
		adaptiveTemplatePixels: at,
		refinementATPixels:     refinementATPixels,
		numExportedSymbols:     numExportedSymbols,
		numNewSymbols:          numNewSymbols,
		data:                   data,
		strict:                 strict,
	}, nil
}
