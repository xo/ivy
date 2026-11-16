// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/decode/text.rs, the text region decoding
// procedure of T.88 6.4.

import (
	"math"
	"math/bits"
)

// decodeTextRegion is decode in hayro-jbig2 src/decode/text.rs.
func decodeTextRegion(header *textRegionHeader, symbols []*bitmap, referredTables []*huffmanTable,
	standardTables standardHuffmanTables, scratch *decodeState) (regionBitmap, error) {
	info := &header.regionInfo
	bm, err := newBitmapWith(info.width, info.height, info.xLocation, info.yLocation,
		header.flags.defaultPixel, &scratch.budget)
	if err != nil {
		return regionBitmap{}, err
	}

	if err := decodeTextRegionInto(header, symbols, referredTables, standardTables, bm, scratch); err != nil {
		return regionBitmap{}, err
	}

	return regionBitmap{bitmap: bm, combinationOperator: info.combinationOperator}, nil
}

// decodeTextRegionInto is decode_into in hayro-jbig2 src/decode/text.rs.
func decodeTextRegionInto(header *textRegionHeader, symbols []*bitmap, referredTables []*huffmanTable,
	standardTables standardHuffmanTables, bm *bitmap, scratch *decodeState) error {
	scratch.resetContexts(1 << header.flags.refinementTemplate.contextBits())

	if header.flags.useHuffman {
		r := newReader(header.data)
		ctx, err := newHuffmanDecodeContext(&r, header, referredTables, standardTables, scratch.contexts)
		if err != nil {
			return err
		}
		return decodeWith(ctx, symbolBitmaps{first: symbols}, header, bm, &scratch.budget, scratch.cancel)
	}

	decoder := newArithmeticDecoder(header.data)

	numSymbols := uint32(len(symbols))
	symbolCodeLength := uint32(bitLen32(max(numSymbols, 1) - 1))
	contexts := newTextRegionContexts(symbolCodeLength)

	ctx := newArithmeticDecodeContext(decoder, contexts, scratch.contexts)
	return decodeWith(ctx, symbolBitmaps{first: symbols}, header, bm, &scratch.budget, scratch.cancel)
}

// symbolBitmaps is SymbolBitmaps in hayro-jbig2 src/decode/text.rs. It joins
// two lists of symbols without a copy.
type symbolBitmaps struct {
	first  []*bitmap
	second []*bitmap
}

// get is SymbolBitmaps::get. It returns nil for an index out of range.
func (s symbolBitmaps) get(index int) *bitmap {
	if index < 0 {
		return nil
	}
	if index < len(s.first) {
		return s.first[index]
	}
	if i := index - len(s.first); i < len(s.second) {
		return s.second[i]
	}
	return nil
}

// decodeWith is decode_with in hayro-jbig2 src/decode/text.rs. It decodes
// the strips of symbol instances, and it draws each instance into region.
func decodeWith(ctx *decodeContext, symbols symbolBitmaps, header *textRegionHeader, region *bitmap,
	budget *memoryBudget, c cancel) error {
	stripSize := header.stripSize()

	stripDeltaT, err := ctx.readStripDeltaT(stripSize)
	if err != nil {
		return err
	}
	if stripDeltaT == math.MinInt32 {
		return ErrOverflowPlacementCoordinate
	}
	stripT := -stripDeltaT
	var firstS int32
	var instanceCount uint32
	var work workCounter

	for instanceCount < header.numInstances {
		deltaT, err := ctx.readStripDeltaT(stripSize)
		if err != nil {
			return err
		}
		var ok bool
		if stripT, ok = checkedAddInt32(stripT, deltaT); !ok {
			return ErrOverflowPlacementCoordinate
		}

		firstSymbolInStrip := true
		var currentS int32

	strip:
		for {
			if ctx.isExhausted() {
				return ErrUnexpectedEOF
			}

			if err := work.add(128, c); err != nil {
				return err
			}

			switch {
			case firstSymbolInStrip:
				deltaFirstS, err := ctx.readFirstS()
				if err != nil {
					return err
				}
				if firstS, ok = checkedAddInt32(firstS, deltaFirstS); !ok {
					return ErrOverflowPlacementCoordinate
				}
				currentS = firstS
				firstSymbolInStrip = false
			case instanceCount == header.numInstances:
				_, hasValue, err := ctx.readDeltaS()

				// Port: upstream evaluates the result only in strict mode, so a
				// lenient decode ignores the error.
				if header.strict {
					if err != nil {
						return err
					}
					if hasValue {
						return ErrTooManyInstances
					}
				}

				break strip
			default:
				deltaS, hasValue, err := ctx.readDeltaS()
				if err != nil {
					return err
				}
				if !hasValue {
					break strip
				}

				if currentS, ok = checkedAddInt32(currentS, deltaS); ok {
					currentS, ok = checkedAddInt32(currentS, int32(header.flags.deltaSOffset))
				}
				if !ok {
					return ErrOverflowPlacementCoordinate
				}
			}

			currentT, err := ctx.readSymbolT(stripSize, header.flags.logStripSize)
			if err != nil {
				return err
			}
			symbolT, ok := checkedAddInt32(stripT, currentT)
			if !ok {
				return ErrOverflowPlacementCoordinate
			}

			symbolID, err := ctx.readSymbolID()
			if err != nil {
				return err
			}

			sb, err := decodeSymbolInstanceBitmap(ctx, symbols, header, symbolID, budget, c)
			if err != nil {
				return err
			}

			symbolBitmapRef := sb.owned
			if symbolBitmapRef == nil {
				symbolBitmapRef = symbols.get(sb.reference)
				if symbolBitmapRef == nil {
					if header.strict {
						return ErrSymbolOutOfRange
					}
					instanceCount++
					continue
				}
			}
			symbolWidth := int32(symbolBitmapRef.width)
			symbolHeight := int32(symbolBitmapRef.height)

			corner := header.flags.referenceCorner
			transposed := header.flags.transposed
			switch {
			case !transposed && (corner == cornerTopRight || corner == cornerBottomRight):
				if currentS, ok = checkedAddInt32(currentS, symbolWidth-1); !ok {
					return ErrOverflowPlacementCoordinate
				}
			case transposed && (corner == cornerBottomLeft || corner == cornerBottomRight):
				if currentS, ok = checkedAddInt32(currentS, symbolHeight-1); !ok {
					return ErrOverflowPlacementCoordinate
				}
			}

			symbolS := currentS

			x, y, err := placeSymbol(corner, transposed, symbolS, symbolT, symbolWidth, symbolHeight)
			if err != nil {
				return err
			}

			if err := region.combine(symbolBitmapRef, x, y, header.flags.combinationOperator, c); err != nil {
				return err
			}
			if err := work.add(uint64(wordWork)*uint64(len(symbolBitmapRef.data)), c); err != nil {
				return err
			}

			if sb.owned != nil {
				budget.release(sb.owned.memorySize())
			}

			switch {
			case !transposed && (corner == cornerTopLeft || corner == cornerBottomLeft):
				if currentS, ok = checkedAddInt32(currentS, symbolWidth-1); !ok {
					return ErrOverflowPlacementCoordinate
				}
			case transposed && (corner == cornerTopLeft || corner == cornerTopRight):
				if currentS, ok = checkedAddInt32(currentS, symbolHeight-1); !ok {
					return ErrOverflowPlacementCoordinate
				}
			}

			instanceCount++
		}
	}

	return nil
}

// placeSymbol is the computation of the top left corner of an instance in
// decode_with in hayro-jbig2 src/decode/text.rs. It takes the closure start
// of upstream, which gives the first pixel of a span from its last pixel.
func placeSymbol(corner referenceCorner, transposed bool, symbolS, symbolT, symbolWidth, symbolHeight int32) (int32, int32, error) {
	start := func(end, size int32) (int32, error) {
		v, ok := checkedSubInt32(end, size-1)
		if !ok {
			return 0, ErrOverflowPlacementCoordinate
		}
		return v, nil
	}

	s, t := symbolS, symbolT
	w, h := symbolWidth, symbolHeight
	if transposed {
		s, t = symbolT, symbolS
	}

	var err error
	x, y := s, t
	switch corner {
	case cornerTopRight:
		x, err = start(s, w)
	case cornerBottomLeft:
		y, err = start(t, h)
	case cornerBottomRight:
		if x, err = start(s, w); err == nil {
			y, err = start(t, h)
		}
	}
	return x, y, err
}

// checkedAddInt32 is i32::checked_add of the Rust core library.
func checkedAddInt32(a, b int32) (int32, bool) {
	s := int64(a) + int64(b)
	if s < math.MinInt32 || s > math.MaxInt32 {
		return 0, false
	}
	return int32(s), true
}

// checkedSubInt32 is i32::checked_sub of the Rust core library.
func checkedSubInt32(a, b int32) (int32, bool) {
	s := int64(a) - int64(b)
	if s < math.MinInt32 || s > math.MaxInt32 {
		return 0, false
	}
	return int32(s), true
}

// checkedMulInt32 is i32::checked_mul of the Rust core library.
func checkedMulInt32(a, b int32) (int32, bool) {
	p := int64(a) * int64(b)
	if p < math.MinInt32 || p > math.MaxInt32 {
		return 0, false
	}
	return int32(p), true
}

// textRegionContexts is TextRegionContexts in hayro-jbig2
// src/decode/text.rs, the decoders of the integers of T.88 Table 31.
type textRegionContexts struct {
	iadt  *integerDecoder
	iafs  *integerDecoder
	iads  *integerDecoder
	iait  *integerDecoder
	iaid  *symbolIDDecoder
	iari  *integerDecoder
	iardw *integerDecoder
	iardh *integerDecoder
	iardx *integerDecoder
	iardy *integerDecoder
}

// newTextRegionContexts is TextRegionContexts::new.
func newTextRegionContexts(symbolCodeLength uint32) *textRegionContexts {
	return &textRegionContexts{
		iadt:  newIntegerDecoder(),
		iafs:  newIntegerDecoder(),
		iads:  newIntegerDecoder(),
		iait:  newIntegerDecoder(),
		iaid:  newSymbolIDDecoder(symbolCodeLength),
		iari:  newIntegerDecoder(),
		iardw: newIntegerDecoder(),
		iardh: newIntegerDecoder(),
		iardx: newIntegerDecoder(),
		iardy: newIntegerDecoder(),
	}
}

// decodeContext is DecodeContext in hayro-jbig2 src/decode/text.rs.
//
// Port: the enum of upstream is a struct. If huffman is true, the variant is
// Huffman, with reader, tables and symbolCodes. Otherwise the variant is
// Arithmetic, with decoder and contexts. Both variants have grContexts.
type decodeContext struct {
	huffman bool

	reader      *reader
	tables      textRegionHuffmanTables
	symbolCodes *huffmanTable

	decoder  *arithmeticDecoder
	contexts *textRegionContexts

	grContexts []arithmeticDecoderContext
}

// newHuffmanDecodeContext is DecodeContext::new_huffman.
func newHuffmanDecodeContext(r *reader, header *textRegionHeader, referredTables []*huffmanTable,
	standardTables standardHuffmanTables, grContexts []arithmeticDecoderContext) (*decodeContext, error) {
	if header.huffmanFlags == nil {
		return nil, ErrHuffmanInvalidSelection
	}
	tables, err := selectHuffmanTables(header.huffmanFlags, referredTables, standardTables)
	if err != nil {
		return nil, err
	}
	if header.symbolIDTable == nil {
		return nil, ErrHuffmanMissingTables
	}

	return &decodeContext{
		huffman:     true,
		reader:      r,
		tables:      tables,
		symbolCodes: header.symbolIDTable,
		grContexts:  grContexts,
	}, nil
}

// newArithmeticDecodeContext is DecodeContext::new_arithmetic.
func newArithmeticDecodeContext(decoder *arithmeticDecoder, contexts *textRegionContexts,
	grContexts []arithmeticDecoderContext) *decodeContext {
	return &decodeContext{
		decoder:    decoder,
		contexts:   contexts,
		grContexts: grContexts,
	}
}

// isExhausted is DecodeContext::is_exhausted.
func (ctx *decodeContext) isExhausted() bool {
	if ctx.huffman {
		return false
	}
	return ctx.decoder.isExhausted()
}

// readStripDeltaT is DecodeContext::read_strip_delta_t. It returns the
// value times the strip size.
func (ctx *decodeContext) readStripDeltaT(stripSize uint32) (int32, error) {
	var value int32
	if ctx.huffman {
		v, err := ctx.tables.deltaT.decodeNoOOB(ctx.reader)
		if err != nil {
			return 0, err
		}
		value = v
	} else {
		v, ok := ctx.contexts.iadt.decode(ctx.decoder)
		if !ok {
			return 0, ErrSymbolUnexpectedOOB
		}
		value = v
	}

	p, ok := checkedMulInt32(value, int32(stripSize))
	if !ok {
		return 0, ErrInvalidStripDelta
	}
	return p, nil
}

// readFirstS is DecodeContext::read_first_s.
func (ctx *decodeContext) readFirstS() (int32, error) {
	if ctx.huffman {
		return ctx.tables.firstS.decodeNoOOB(ctx.reader)
	}
	v, ok := ctx.contexts.iafs.decode(ctx.decoder)
	if !ok {
		return 0, ErrSymbolUnexpectedOOB
	}
	return v, nil
}

// readDeltaS is DecodeContext::read_delta_s. It returns false for the
// out-of-band value, which ends the strip.
func (ctx *decodeContext) readDeltaS() (int32, bool, error) {
	if ctx.huffman {
		return ctx.tables.deltaS.decode(ctx.reader)
	}
	v, ok := ctx.contexts.iads.decode(ctx.decoder)
	return v, ok, nil
}

// readSymbolT is DecodeContext::read_symbol_t.
func (ctx *decodeContext) readSymbolT(stripSize uint32, logStripSize uint8) (int32, error) {
	if stripSize == 1 {
		return 0, nil
	}

	if ctx.huffman {
		v, ok := ctx.reader.readBits(logStripSize)
		if !ok {
			return 0, ErrHuffmanInvalidCode
		}
		return int32(v), nil
	}
	v, ok := ctx.contexts.iait.decode(ctx.decoder)
	if !ok {
		return 0, ErrSymbolUnexpectedOOB
	}
	return v, nil
}

// readSymbolID is DecodeContext::read_symbol_id.
func (ctx *decodeContext) readSymbolID() (int, error) {
	if ctx.huffman {
		v, err := ctx.symbolCodes.decodeNoOOB(ctx.reader)
		if err != nil {
			return 0, err
		}
		// Port: upstream casts the i32 to a usize, so a negative value
		// becomes an index past every symbol.
		if v < 0 {
			return math.MaxInt, nil
		}
		return int(v), nil
	}
	return int(ctx.contexts.iaid.decode(ctx.decoder)), nil
}

// readRefinementFlag is DecodeContext::read_refinement_flag.
func (ctx *decodeContext) readRefinementFlag() (uint8, error) {
	if ctx.huffman {
		b, ok := ctx.reader.readBit()
		if !ok {
			return 0, ErrUnexpectedEOF
		}
		return b, nil
	}
	v, ok := ctx.contexts.iari.decode(ctx.decoder)
	if !ok {
		return 0, ErrSymbolUnexpectedOOB
	}
	return uint8(v), nil
}

// readRefinementValue reads a value with the Huffman table, or with the
// arithmetic integer decoder. It is the common part of the four methods
// that read the refinement values.
func (ctx *decodeContext) readRefinementValue(table *huffmanTable, id *integerDecoder) (int32, error) {
	if ctx.huffman {
		return table.decodeNoOOB(ctx.reader)
	}
	v, ok := id.decode(ctx.decoder)
	if !ok {
		return 0, ErrSymbolUnexpectedOOB
	}
	return v, nil
}

// readRefinementDeltaWidth is DecodeContext::read_refinement_delta_width.
func (ctx *decodeContext) readRefinementDeltaWidth() (int32, error) {
	if ctx.huffman {
		return ctx.readRefinementValue(ctx.tables.refinementWidth, nil)
	}
	return ctx.readRefinementValue(nil, ctx.contexts.iardw)
}

// readRefinementDeltaHeight is DecodeContext::read_refinement_delta_height.
func (ctx *decodeContext) readRefinementDeltaHeight() (int32, error) {
	if ctx.huffman {
		return ctx.readRefinementValue(ctx.tables.refinementHeight, nil)
	}
	return ctx.readRefinementValue(nil, ctx.contexts.iardh)
}

// readRefinementXOffset is DecodeContext::read_refinement_x_offset.
func (ctx *decodeContext) readRefinementXOffset() (int32, error) {
	if ctx.huffman {
		return ctx.readRefinementValue(ctx.tables.refinementX, nil)
	}
	return ctx.readRefinementValue(nil, ctx.contexts.iardx)
}

// readRefinementYOffset is DecodeContext::read_refinement_y_offset.
func (ctx *decodeContext) readRefinementYOffset() (int32, error) {
	if ctx.huffman {
		return ctx.readRefinementValue(ctx.tables.refinementY, nil)
	}
	return ctx.readRefinementValue(nil, ctx.contexts.iardy)
}

// decodeRefinementBitmap is DecodeContext::decode_refinement_bitmap.
func (ctx *decodeContext) decodeRefinementBitmap(refined, referenceBitmap *bitmap, referenceXOffset, referenceYOffset int32,
	tmpl refinementTemplate, at []adaptiveTemplatePixel, c cancel) error {
	if !ctx.huffman {
		return decodeRefinementBitmap(ctx.decoder, ctx.grContexts, refined, referenceBitmap,
			referenceXOffset, referenceYOffset, tmpl, at, false, c)
	}

	size, err := ctx.tables.refinementSize.decodeNoOOB(ctx.reader)
	if err != nil {
		return err
	}
	ctx.reader.align()

	// Upstream casts the size to a u32, so a negative size is too large.
	data, ok := ctx.reader.readBytes(int(uint32(size)))
	if !ok {
		return ErrUnexpectedEOF
	}

	decoder := newArithmeticDecoder(data)
	return decodeRefinementBitmap(decoder, ctx.grContexts, refined, referenceBitmap,
		referenceXOffset, referenceYOffset, tmpl, at, false, c)
}

// symbolBitmap is SymbolBitmap in hayro-jbig2 src/decode/text.rs.
//
// Port: the enum of upstream is a struct. If owned is nil, the variant is
// Reference with the index reference. Otherwise the variant is Owned.
type symbolBitmap struct {
	reference int
	owned     *bitmap
}

// decodeSymbolInstanceBitmap is decode_symbol_instance_bitmap in hayro-jbig2
// src/decode/text.rs. It refines the symbol of an instance when the
// instance asks for it.
func decodeSymbolInstanceBitmap(ctx *decodeContext, symbols symbolBitmaps, header *textRegionHeader,
	symbolID int, budget *memoryBudget, c cancel) (symbolBitmap, error) {
	if !header.flags.useRefinement {
		return symbolBitmap{reference: symbolID}, nil
	}
	flag, err := ctx.readRefinementFlag()
	if err != nil {
		return symbolBitmap{}, err
	}
	if flag == 0 {
		return symbolBitmap{reference: symbolID}, nil
	}

	referenceBitmap := symbols.get(symbolID)
	if referenceBitmap == nil {
		return symbolBitmap{}, ErrSymbolOutOfRange
	}

	rdw, err := ctx.readRefinementDeltaWidth()
	if err != nil {
		return symbolBitmap{}, err
	}
	rdh, err := ctx.readRefinementDeltaHeight()
	if err != nil {
		return symbolBitmap{}, err
	}
	rdx, err := ctx.readRefinementXOffset()
	if err != nil {
		return symbolBitmap{}, err
	}
	rdy, err := ctx.readRefinementYOffset()
	if err != nil {
		return symbolBitmap{}, err
	}

	refinedWidth, ok := checkedAddInt32(int32(referenceBitmap.width), rdw)
	if !ok || refinedWidth < 0 {
		return symbolBitmap{}, ErrOverflowBitmapDimension
	}
	refinedHeight, ok := checkedAddInt32(int32(referenceBitmap.height), rdh)
	if !ok || refinedHeight < 0 {
		return symbolBitmap{}, ErrOverflowBitmapDimension
	}
	referenceXOffset, ok := checkedAddInt32(divEuclid2(rdw), rdx)
	if !ok {
		return symbolBitmap{}, ErrOverflowReferenceOffset
	}
	referenceYOffset, ok := checkedAddInt32(divEuclid2(rdh), rdy)
	if !ok {
		return symbolBitmap{}, ErrOverflowReferenceOffset
	}

	refined, err := newBitmap(uint32(refinedWidth), uint32(refinedHeight), budget)
	if err != nil {
		return symbolBitmap{}, err
	}

	if err := ctx.decodeRefinementBitmap(refined, referenceBitmap, referenceXOffset, referenceYOffset,
		header.flags.refinementTemplate, header.refinementATPixels, c); err != nil {
		return symbolBitmap{}, err
	}

	return symbolBitmap{owned: refined}, nil
}

// divEuclid2 is i32::div_euclid(v, 2) of the Rust core library. It rounds
// toward negative infinity.
func divEuclid2(v int32) int32 {
	return v >> 1
}

// selectHuffmanTables is select_huffman_tables in hayro-jbig2
// src/decode/text.rs. It takes the user-supplied tables in order.
func selectHuffmanTables(flags *textRegionHuffmanFlags, customTables []*huffmanTable,
	standardTables standardHuffmanTables) (textRegionHuffmanTables, error) {
	var t textRegionHuffmanTables
	customTableIdx := 0

	getCustom := func() (*huffmanTable, error) {
		if customTableIdx >= len(customTables) {
			return nil, ErrHuffmanMissingTables
		}
		table := customTables[customTableIdx]
		customTableIdx++
		return table, nil
	}

	pick := func(selection uint8, standard ...*huffmanTable) (*huffmanTable, error) {
		// The last selection, 3, is a user-supplied table. A value between
		// the standard tables and 3 is not valid.
		if int(selection) < len(standard) {
			return standard[selection], nil
		}
		if selection == 3 {
			return getCustom()
		}
		return nil, ErrHuffmanInvalidSelection
	}

	var err error
	if t.firstS, err = pick(flags.firstSTable, standardTables.tableF(), standardTables.tableG()); err != nil {
		return t, err
	}
	if t.deltaS, err = pick(flags.deltaSTable, standardTables.tableH(), standardTables.tableI(), standardTables.tableJ()); err != nil {
		return t, err
	}
	if t.deltaT, err = pick(flags.deltaTTable, standardTables.tableK(), standardTables.tableL(), standardTables.tableM()); err != nil {
		return t, err
	}
	if t.refinementWidth, err = pick(flags.refinementWidthTable, standardTables.tableN(), standardTables.tableO()); err != nil {
		return t, err
	}
	if t.refinementHeight, err = pick(flags.refinementHeightTable, standardTables.tableN(), standardTables.tableO()); err != nil {
		return t, err
	}
	if t.refinementX, err = pick(flags.refinementXTable, standardTables.tableN(), standardTables.tableO()); err != nil {
		return t, err
	}
	if t.refinementY, err = pick(flags.refinementYTable, standardTables.tableN(), standardTables.tableO()); err != nil {
		return t, err
	}

	switch flags.refinementSizeTable {
	case 0:
		t.refinementSize = standardTables.tableA()
	case 1:
		if t.refinementSize, err = getCustom(); err != nil {
			return t, err
		}
	default:
		return t, ErrHuffmanInvalidSelection
	}

	return t, nil
}

// decodeSymbolIDHuffmanTable is decode_symbol_id_huffman_table in
// hayro-jbig2 src/decode/text.rs. It reads the symbol ID Huffman table of
// T.88 7.4.3.1.7.
func decodeSymbolIDHuffmanTable(r *reader, numSymbols uint32) (*huffmanTable, error) {
	runcodeLines := make([]tableLine, 0, 35)
	for runcodeIdx := range int32(35) {
		prefixLength, ok := r.readBits(4)
		if !ok {
			return nil, ErrHuffmanInvalidCode
		}
		runcodeLines = append(runcodeLines, newTableLine(runcodeIdx, uint8(prefixLength), 0))
	}

	runcodeTable, err := buildHuffmanTable(runcodeLines)
	if err != nil {
		return nil, err
	}
	symbolCodeLengths := make([]uint8, 0, numSymbols)

	repeat := func(v uint8, n int) {
		for range n {
			symbolCodeLengths = append(symbolCodeLengths, v)
		}
	}

	for uint32(len(symbolCodeLengths)) < numSymbols {
		runcode, err := runcodeTable.decodeNoOOB(r)
		if err != nil {
			return nil, err
		}

		switch rc := uint32(runcode); {
		case rc <= 31:
			symbolCodeLengths = append(symbolCodeLengths, uint8(rc))
		case rc == 32:
			extraBits, ok := r.readBits(2)
			if !ok {
				return nil, ErrHuffmanInvalidCode
			}
			if len(symbolCodeLengths) == 0 {
				return nil, ErrHuffmanInvalidCode
			}
			repeat(symbolCodeLengths[len(symbolCodeLengths)-1], int(extraBits)+3)
		case rc == 33:
			extraBits, ok := r.readBits(3)
			if !ok {
				return nil, ErrHuffmanInvalidCode
			}
			repeat(0, int(extraBits)+3)
		case rc == 34:
			extraBits, ok := r.readBits(7)
			if !ok {
				return nil, ErrHuffmanInvalidCode
			}
			repeat(0, int(extraBits)+11)
		default:
			return nil, ErrHuffmanInvalidCode
		}
	}

	if uint32(len(symbolCodeLengths)) != numSymbols {
		return nil, ErrHuffmanInvalidCode
	}

	r.align()

	symbolLines := make([]tableLine, len(symbolCodeLengths))
	for i, prefixLength := range symbolCodeLengths {
		symbolLines[i] = newTableLine(int32(i), prefixLength, 0)
	}
	return buildHuffmanTable(symbolLines)
}

// textRegionHuffmanTables is TextRegionHuffmanTables in hayro-jbig2
// src/decode/text.rs.
type textRegionHuffmanTables struct {
	firstS           *huffmanTable
	deltaS           *huffmanTable
	deltaT           *huffmanTable
	refinementWidth  *huffmanTable
	refinementHeight *huffmanTable
	refinementX      *huffmanTable
	refinementY      *huffmanTable
	refinementSize   *huffmanTable
}

// referenceCorner is ReferenceCorner in hayro-jbig2 src/decode/text.rs.
type referenceCorner uint8

// The values of referenceCorner, in the order of the two bits of REFCORNER.
const (
	cornerBottomLeft referenceCorner = iota
	cornerTopLeft
	cornerBottomRight
	cornerTopRight
)

// textRegionFlags is TextRegionFlags in hayro-jbig2 src/decode/text.rs.
type textRegionFlags struct {
	useHuffman          bool
	useRefinement       bool
	logStripSize        uint8
	referenceCorner     referenceCorner
	transposed          bool
	combinationOperator combinationOperator
	defaultPixel        bool
	deltaSOffset        int8
	refinementTemplate  refinementTemplate
}

// textRegionHuffmanFlags is TextRegionHuffmanFlags in hayro-jbig2
// src/decode/text.rs.
type textRegionHuffmanFlags struct {
	firstSTable           uint8
	deltaSTable           uint8
	deltaTTable           uint8
	refinementWidthTable  uint8
	refinementHeightTable uint8
	refinementXTable      uint8
	refinementYTable      uint8
	refinementSizeTable   uint8
}

// textRegionHeader is TextRegionHeader in hayro-jbig2 src/decode/text.rs. A
// nil huffmanFlags or symbolIDTable is the None of upstream.
type textRegionHeader struct {
	regionInfo         regionSegmentInfo
	flags              textRegionFlags
	huffmanFlags       *textRegionHuffmanFlags
	refinementATPixels []adaptiveTemplatePixel
	numInstances       uint32
	symbolIDTable      *huffmanTable
	data               []byte
	strict             bool
}

// stripSize is TextRegionHeader::strip_size.
func (h *textRegionHeader) stripSize() uint32 {
	return 1 << h.flags.logStripSize
}

// parseTextRegionFlags is parse_text_region_flags in hayro-jbig2
// src/decode/text.rs.
func parseTextRegionFlags(r *reader) (textRegionFlags, error) {
	flagsWord, ok := r.readU16()
	if !ok {
		return textRegionFlags{}, ErrUnexpectedEOF
	}
	op, err := combinationOperatorFromValue(uint8((flagsWord >> 7) & 0x03))
	if err != nil {
		return textRegionFlags{}, err
	}

	// The 5 bits of SBDSOFFSET are a signed value.
	deltaSOffset := int8(uint8((flagsWord>>10)&0x1F)<<3) >> 3

	return textRegionFlags{
		useHuffman:          flagsWord&0x0001 != 0,
		useRefinement:       flagsWord&0x0002 != 0,
		logStripSize:        uint8((flagsWord >> 2) & 0x03),
		referenceCorner:     referenceCorner((flagsWord >> 4) & 0x03),
		transposed:          flagsWord&0x0040 != 0,
		combinationOperator: op,
		defaultPixel:        flagsWord&0x0200 != 0,
		deltaSOffset:        deltaSOffset,
		refinementTemplate:  refinementTemplateFromByte(uint8(flagsWord >> 15)),
	}, nil
}

// parseTextRegionHuffmanFlags is parse_text_region_huffman_flags in
// hayro-jbig2 src/decode/text.rs.
func parseTextRegionHuffmanFlags(r *reader) (*textRegionHuffmanFlags, error) {
	flagsWord, ok := r.readU16()
	if !ok {
		return nil, ErrUnexpectedEOF
	}
	return &textRegionHuffmanFlags{
		firstSTable:           uint8(flagsWord & 0x03),
		deltaSTable:           uint8((flagsWord >> 2) & 0x03),
		deltaTTable:           uint8((flagsWord >> 4) & 0x03),
		refinementWidthTable:  uint8((flagsWord >> 6) & 0x03),
		refinementHeightTable: uint8((flagsWord >> 8) & 0x03),
		refinementXTable:      uint8((flagsWord >> 10) & 0x03),
		refinementYTable:      uint8((flagsWord >> 12) & 0x03),
		refinementSizeTable:   uint8((flagsWord >> 14) & 0x01),
	}, nil
}

// parseTextRegion is parse in hayro-jbig2 src/decode/text.rs.
func parseTextRegion(r *reader, numSymbols uint32, strict bool) (*textRegionHeader, error) {
	regionInfo, err := parseRegionSegmentInfo(r)
	if err != nil {
		return nil, err
	}
	flags, err := parseTextRegionFlags(r)
	if err != nil {
		return nil, err
	}
	var huffmanFlags *textRegionHuffmanFlags
	if flags.useHuffman {
		huffmanFlags, err = parseTextRegionHuffmanFlags(r)
		if err != nil {
			return nil, err
		}
	}

	var refinementATPixels []adaptiveTemplatePixel
	if flags.useRefinement && flags.refinementTemplate == refinementTemplate0 {
		refinementATPixels, err = parseRefinementATPixels(r)
		if err != nil {
			return nil, err
		}
	}

	numInstances, ok := r.readU32()
	if !ok {
		return nil, ErrUnexpectedEOF
	}

	var symbolIDTable *huffmanTable
	if flags.useHuffman {
		symbolIDTable, err = decodeSymbolIDHuffmanTable(r, numSymbols)
		if err != nil {
			return nil, err
		}
	}

	data, ok := r.tail()
	if !ok {
		return nil, ErrUnexpectedEOF
	}

	return &textRegionHeader{
		regionInfo:         regionInfo,
		flags:              flags,
		huffmanFlags:       huffmanFlags,
		refinementATPixels: refinementATPixels,
		numInstances:       numInstances,
		symbolIDTable:      symbolIDTable,
		data:               data,
		strict:             strict,
	}, nil
}

// bitLen32 is 32 - u32::leading_zeros of the Rust core library, the number
// of bits of v.
func bitLen32(v uint32) int {
	return bits.Len32(v)
}
