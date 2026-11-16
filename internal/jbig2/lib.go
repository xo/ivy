// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// Package jbig2 decodes JBIG2 images, as ITU-T T.88 (ISO/IEC 14492) defines
// them. JBIG2 compresses bi-level images, and PDF uses it for scanned text.
// The package is a port of the Rust crate hayro-jbig2.
//
// A PDF reader decodes the data of the filter JBIG2Decode with
// NewEmbeddedPage, and the stream JBIG2Globals with NewGlobals. A
// standalone JBIG2 file opens with NewFile.
//
// The input is untrusted. The decoder never panics on malformed data, and
// DecodeSettings bounds the memory that a page can take.
package jbig2

// This file ports hayro-jbig2 src/lib.rs.

import (
	"cmp"
	"errors"
	"iter"
	"math"
	"slices"
	"sync"
)

// decodeState is DecodeState in hayro-jbig2 src/lib.rs. It holds the
// arithmetic contexts that the regions reuse, the memory budget and the
// check for cancellation.
type decodeState struct {
	contexts []arithmeticDecoderContext
	budget   memoryBudget
	cancel   cancel
}

// newDecodeState is DecodeState::new.
func newDecodeState(memoryLimit int, c cancel) *decodeState {
	return &decodeState{budget: newMemoryBudget(memoryLimit), cancel: c}
}

// resetContexts is the clear and resize of DecodeState::contexts in
// upstream. It gives n contexts in their initial state.
func (s *decodeState) resetContexts(n int) {
	if cap(s.contexts) >= n {
		s.contexts = s.contexts[:n]
		clear(s.contexts)
		return
	}
	s.contexts = make([]arithmeticDecoderContext, n)
}

// DefaultMemoryLimit is the default of DecodeSettings.MemoryLimit: 1 GiB.
const DefaultMemoryLimit = 1 << 30

// DecodeSettings is DecodeSettings in hayro-jbig2 src/lib.rs. The zero
// value is the default of upstream.
type DecodeSettings struct {
	// Strict rejects data that violates the specification. If Strict is
	// false, the decoder decodes malformed data as well as it can.
	Strict bool
	// MemoryLimit is the largest number of bytes that the decoding of a page
	// can hold at one time.
	//
	// Port: upstream holds an Option<usize> with the default Some(1 GiB).
	// A value of 0 is the default DefaultMemoryLimit, and a negative value is
	// None, which means no limit.
	MemoryLimit int
	// Cancel stops the decoder when it reports true. A nil Cancel never
	// stops the decoder.
	Cancel CancelCheck
}

// memoryLimit returns the limit of the settings, or -1 for no limit.
func (s *DecodeSettings) memoryLimit() int {
	switch {
	case s.MemoryLimit == 0:
		return DefaultMemoryLimit
	case s.MemoryLimit < 0:
		return -1
	}
	return s.MemoryLimit
}

// File is File in hayro-jbig2 src/lib.rs, a standalone JBIG2 file.
type File struct {
	pages    []filePage
	globals  *lazyGlobals
	settings DecodeSettings
}

// filePage is FilePage in hayro-jbig2 src/lib.rs.
type filePage struct {
	number   uint32
	segments []segment
	size     pageSize
}

// lazyGlobals is LazyGlobals in hayro-jbig2 src/lib.rs. It decodes the
// global segments of a file at the first use.
//
// Port: upstream keeps the result in a OnceLock. Go keeps it under a mutex.
// As in upstream, a cancelled decode leaves no result, so that a later call
// can try again.
type lazyGlobals struct {
	segments []segment

	mu      sync.Mutex
	done    bool
	decoded *Globals
	err     error
}

// get is LazyGlobals::get.
func (lg *lazyGlobals) get(settings *DecodeSettings) (*Globals, error) {
	lg.mu.Lock()
	defer lg.mu.Unlock()

	if lg.done {
		return lg.decoded, lg.err
	}

	decoded, err := globalsFromSegments(lg.segments, settings)
	if errors.Is(err, ErrCancelled) {
		return nil, err
	}

	lg.done = true
	lg.decoded, lg.err = decoded, err
	return decoded, err
}

// NewFile is File::new in hayro-jbig2 src/lib.rs. It parses a JBIG2 file,
// and it groups the segments by page. It decodes no region.
//
// The file keeps data, so the caller must not change data while it uses the
// file.
func NewFile(data []byte, settings DecodeSettings) (*File, error) {
	strict := settings.Strict
	file, err := parseFile(data, strict)
	if err != nil {
		return nil, err
	}

	var globals []segment
	var pages []filePage
	pageIndices := make(map[uint32]int)

	for _, seg := range file.segments {
		number := seg.header.pageAssociation
		if number == 0 {
			if strict && seg.header.segmentType.requiresPage() {
				return nil, ErrMissingPageAssociation
			}

			globals = append(globals, seg)
			continue
		}

		index, ok := pageIndices[number]
		if !ok {
			pages = append(pages, filePage{number: number})
			index = len(pages) - 1
			pageIndices[number] = index
		}
		pages[index].segments = append(pages[index].segments, seg)
	}

	if strict && file.header.hasNumberOfPages && int(file.header.numberOfPages) != len(pages) {
		return nil, ErrPageCountMismatch
	}

	for i := range pages {
		page := &pages[i]
		if strict {
			endOfPageCount := 0
			for _, seg := range page.segments {
				if seg.header.segmentType == segmentEndOfPage {
					endOfPageCount++
				}
			}

			if endOfPageCount != 1 {
				return nil, ErrInvalidEndOfPage
			}
		}

		if page.size, err = newPageSize(page.segments); err != nil {
			return nil, err
		}
	}

	f := &File{pages: pages, settings: settings}
	if len(globals) > 0 {
		f.globals = &lazyGlobals{segments: globals}
	}
	return f, nil
}

// PageCount is File::page_count. It returns the number of pages in the
// file.
func (f *File) PageCount() int {
	return len(f.pages)
}

// Page is File::page. It returns the page at index, in the order of the
// file, or nil for an index out of range.
func (f *File) Page(index int) *Page {
	if index < 0 || index >= len(f.pages) {
		return nil
	}
	page := &f.pages[index]

	return &Page{
		number:   page.number,
		segments: page.segments,
		lazy:     f.globals,
		size:     page.size,
		settings: f.settings,
	}
}

// Pages is File::pages. It returns the pages of the file, in the order of
// the file.
func (f *File) Pages() iter.Seq[*Page] {
	return func(yield func(*Page) bool) {
		for i := range f.PageCount() {
			if !yield(f.Page(i)) {
				return
			}
		}
	}
}

// Globals is Globals in hayro-jbig2 src/lib.rs. It holds the decoded global
// segments that the embedded pages share, such as the data of the stream
// JBIG2Globals of a PDF file.
type Globals struct {
	dictionaries dictionaries
}

// NewGlobals is Globals::new in hayro-jbig2 src/lib.rs. It parses and
// decodes embedded global segments. The data has no file header.
func NewGlobals(data []byte, settings DecodeSettings) (*Globals, error) {
	r := newReader(data)
	segments, err := parseSegmentsSequential(&r, nil, settings.Strict)
	if err != nil {
		return nil, err
	}
	sortSegments(segments)

	return globalsFromSegments(segments, &settings)
}

// globalsFromSegments is Globals::from_segments.
func globalsFromSegments(segments []segment, settings *DecodeSettings) (*Globals, error) {
	strict := settings.Strict
	var dicts dictionaries
	c := cancel{handle: settings.Cancel}
	state := newDecodeState(settings.memoryLimit(), c)

	for i := range segments {
		if err := c.check(); err != nil {
			return nil, err
		}
		if err := decodeDictionarySegment(&segments[i], &dicts, nil, standardHuffmanTables{}, state, strict); err != nil {
			return nil, err
		}
	}

	return &Globals{dictionaries: dicts}, nil
}

// Page is Page in hayro-jbig2 src/lib.rs, a JBIG2 page of a File or
// embedded in another format.
//
// Port: upstream holds the segments either owned or borrowed, and the
// globals either decoded or lazy. Go holds a slice of segments, and either
// globals or lazy.
type Page struct {
	number   uint32
	segments []segment
	globals  *Globals
	lazy     *lazyGlobals
	size     pageSize
	settings DecodeSettings
}

// NewEmbeddedPage is Page::new_embedded in hayro-jbig2 src/lib.rs. It
// parses an embedded page, such as the data of the filter JBIG2Decode of a
// PDF file. The data has no file header. The globals can be nil.
//
// The page keeps data, so the caller must not change data while it uses the
// page.
func NewEmbeddedPage(data []byte, globals *Globals, settings DecodeSettings) (*Page, error) {
	r := newReader(data)
	segments, err := parseSegmentsSequential(&r, nil, settings.Strict)
	if err != nil {
		return nil, err
	}

	sortSegments(segments)

	size, err := newPageSize(segments)
	if err != nil {
		return nil, err
	}
	number := uint32(1)
	for _, seg := range segments {
		if seg.header.segmentType == segmentPageInformation {
			number = seg.header.pageAssociation
			break
		}
	}

	return &Page{
		number:   number,
		segments: segments,
		globals:  globals,
		size:     size,
		settings: settings,
	}, nil
}

// Number is Page::number. It returns the page number of the segment headers
// of the page.
func (p *Page) Number() uint32 {
	return p.number
}

// Width is Page::width. It returns the width of the page in pixels.
func (p *Page) Width() uint32 {
	return p.size.width
}

// Height is Page::height. It returns the height of the page in pixels.
func (p *Page) Height() uint32 {
	return p.size.height
}

// Decode is Page::decode. It decodes the page.
func (p *Page) Decode() (*PageBitmap, error) {
	globals := p.globals
	if p.lazy != nil {
		var err error
		if globals, err = p.lazy.get(&p.settings); err != nil {
			return nil, err
		}
	}

	bm, err := decodeSegments(p.segments, globals, p.size, &p.settings)
	if err != nil {
		return nil, err
	}

	return &PageBitmap{bitmap: bm, settings: p.settings}, nil
}

// PageBitmap is PageBitmap in hayro-jbig2 src/lib.rs, the decoded pixels of
// a page. Each row is a slice of uint32 words. The first pixel is the most
// significant bit of the first word, and black is 1.
type PageBitmap struct {
	bitmap *bitmap
	// Port: upstream keeps only the cancel handle. Go keeps the settings,
	// so that DataU8 can respect the memory limit.
	settings DecodeSettings
}

// Width is PageBitmap::width. It returns the width of the bitmap in pixels.
func (pb *PageBitmap) Width() uint32 {
	return pb.bitmap.width
}

// Height is PageBitmap::height. It returns the height of the bitmap in
// pixels.
func (pb *PageBitmap) Height() uint32 {
	return pb.bitmap.height
}

// Stride returns the number of words of each row.
//
// Port: upstream gives the rows only through rows. Go also gives the
// stride, so that a caller can index Data.
func (pb *PageBitmap) Stride() int {
	return int(pb.bitmap.stride)
}

// Data returns the words of all rows, row after row. The caller must not
// change them.
//
// Port: this method is not in upstream. It gives the words without a copy.
func (pb *PageBitmap) Data() []uint32 {
	return pb.bitmap.data
}

// Rows is PageBitmap::rows. It returns the rows of the bitmap. The bits past
// the width of the bitmap are unspecified.
func (pb *PageBitmap) Rows() iter.Seq[[]uint32] {
	return func(yield func([]uint32) bool) {
		stride := int(pb.bitmap.stride)
		if stride == 0 {
			return
		}
		data := pb.bitmap.data
		for i := 0; i+stride <= len(data); i += stride {
			if !yield(data[i : i+stride]) {
				return
			}
		}
	}
}

// DataU8 is PageBitmap::data_u8. It returns the pixels as 8-bit samples,
// with black as 0 and white as 255.
//
// Port: upstream can only fail with Cancelled. Go also returns
// ErrMemoryLimitExceeded when the samples do not fit in the memory limit of
// the settings, because a page of the largest size takes 4 GiB of samples.
func (pb *PageBitmap) DataU8() ([]byte, error) {
	n := int(pb.Width()) * int(pb.Height())
	if limit := pb.settings.memoryLimit(); limit >= 0 && n > limit {
		return nil, ErrMemoryLimitExceeded
	}

	buf := make([]byte, n)
	if err := pb.StoreU8Into(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// StoreU8Into is PageBitmap::store_u8_into. It stores the pixels as 8-bit
// samples into buf, with black as 0 and white as 255. It panics if buf does
// not hold width times height bytes, as upstream does.
func (pb *PageBitmap) StoreU8Into(buf []byte) error {
	if len(buf) != int(pb.Width())*int(pb.Height()) {
		panic("jbig2: buffer has the wrong size")
	}

	c := cancel{handle: pb.settings.Cancel}

	for i := 0; i < len(buf); i += fillChunk {
		if err := c.check(); err != nil {
			return err
		}
		chunk := buf[i:min(i+fillChunk, len(buf))]
		for j := range chunk {
			chunk[j] = 0xFF
		}
	}

	return pb.storeBlackU8(buf, c)
}

// storeBlackU8 is PageBitmap::store_black_u8. It writes 0 for each black
// pixel.
func (pb *PageBitmap) storeBlackU8(buf []byte, c cancel) error {
	mask := rowMask(copyWork(pb.Width()))
	width := int(pb.Width())
	if width == 0 {
		return nil
	}

	y := uint32(0)
	for row := range pb.Rows() {
		if int(y)*width >= len(buf) {
			break
		}
		if y&mask == mask {
			if err := c.check(); err != nil {
				return err
			}
		}

		out := buf[int(y)*width : int(y+1)*width]
		for wi, w := range row {
			if w == 0 {
				continue
			}
			start := wi * 32
			if start >= len(out) {
				break
			}
			for bi := range 4 {
				b := byte(w >> (24 - 8*bi))
				if b == 0 {
					continue
				}
				o := start + bi*8
				if o >= len(out) {
					break
				}
				end := min(o+8, len(out))
				copy(out[o:end], u8Pixels[b][:end-o])
			}
		}
		y++
	}

	return nil
}

// u8Pixels is U8_PIXELS in hayro-jbig2 src/lib.rs. It maps a byte of pixels
// to its 8 samples.
var u8Pixels = func() (table [256][8]byte) {
	for b := range 256 {
		for bit := range 8 {
			table[b][bit] = 0xFF
			if b&(0x80>>bit) != 0 {
				table[b][bit] = 0x00
			}
		}
	}
	return table
}()

// pageSize is PageSize in hayro-jbig2 src/lib.rs.
type pageSize struct {
	width        uint32
	height       uint32
	defaultPixel bool
}

// newPageSize is PageSize::new. It reads the size of the page from its page
// information segment. An unknown height comes from the end of stripe
// segments.
func newPageSize(segments []segment) (pageSize, error) {
	var heightFromStripes uint32
	hasHeightFromStripes := false
	for _, seg := range segments {
		if seg.header.segmentType != segmentEndOfStripe || len(seg.data) != 4 {
			continue
		}
		row := uint32(seg.data[0])<<24 | uint32(seg.data[1])<<16 | uint32(seg.data[2])<<8 | uint32(seg.data[3])
		if row == math.MaxUint32 {
			continue
		}
		if !hasHeightFromStripes || row+1 > heightFromStripes {
			heightFromStripes = row + 1
			hasHeightFromStripes = true
		}
	}

	var pageInfoSeg *segment
	for i := range segments {
		if segments[i].header.segmentType == segmentPageInformation {
			pageInfoSeg = &segments[i]
			break
		}
	}
	if pageInfoSeg == nil {
		return pageSize{}, ErrMissingPageInfo
	}

	r := newReader(pageInfoSeg.data)
	pageInfo, err := parsePageInformation(&r)
	if err != nil {
		return pageSize{}, err
	}

	height := pageInfo.height
	if height == 0xFFFF_FFFF {
		if !hasHeightFromStripes {
			return pageSize{}, ErrUnknownPageHeight
		}
		height = heightFromStripes
	}

	if pageInfo.width == 0 || height == 0 {
		return pageSize{}, ErrEmptyPage
	}

	if err := checkSize(pageInfo.width, height); err != nil {
		return pageSize{}, err
	}

	return pageSize{
		width:        pageInfo.width,
		height:       height,
		defaultPixel: pageInfo.flags.defaultPixel != 0,
	}, nil
}

// decodeSegments is decode_segments in hayro-jbig2 src/lib.rs. It decodes
// the segments of a page in order.
func decodeSegments(segments []segment, globals *Globals, size pageSize, settings *DecodeSettings) (*bitmap, error) {
	strict := settings.Strict
	c := cancel{handle: settings.Cancel}
	state := newDecodeState(settings.memoryLimit(), c)
	pageBitmap, err := newBitmapWith(size.width, size.height, 0, 0, size.defaultPixel, &state.budget)
	if err != nil {
		return nil, err
	}
	state.budget.retain(pageBitmap.memorySize())
	ps := newPageState(size.defaultPixel)
	standardTables := standardHuffmanTables{}

	for i := range segments {
		seg := &segments[i]
		if err := c.check(); err != nil {
			return nil, err
		}
		stop, err := decodeSegment(seg, pageBitmap, ps, globals, standardTables, state, strict)
		state.budget.endSegment()

		if err != nil {
			// Nothing refers to an immediate region, so the page does not
			// depend on a region that failed.
			if !strict &&
				!errors.Is(err, ErrMemoryLimitExceeded) &&
				!errors.Is(err, ErrCancelled) &&
				seg.header.segmentType.isImmediateRegion() {
				// The region can be in the page already.
				ps.pagePristine = false
				continue
			}
			return nil, err
		}
		if stop {
			break
		}
	}

	return pageBitmap, nil
}

// decodeSegment is decode_segment in hayro-jbig2 src/lib.rs. It returns true
// at the end of the page or of the file, the Break of upstream.
func decodeSegment(seg *segment, pageBitmap *bitmap, ps *pageState, globals *Globals,
	standardTables standardHuffmanTables, state *decodeState, strict bool) (bool, error) {
	r := newReader(seg.data)

	switch seg.header.segmentType {
	case segmentPageInformation:
	case segmentImmediateGenericRegion, segmentImmediateLosslessGenericRegion:
		hadUnknownLength := !seg.header.hasDataLength
		header, err := parseGenericRegion(&r, hadUnknownLength, strict)
		if err != nil {
			return false, err
		}

		if ps.canDecodeDirectly(pageBitmap, &header.regionInfo, false) {
			if err := decodeGenericRegionInto(header, pageBitmap, state); err != nil {
				return false, err
			}
		} else {
			region, err := decodeGenericRegion(header, state)
			if err != nil {
				return false, err
			}
			if err := combineRegion(pageBitmap, region, state.cancel); err != nil {
				return false, err
			}
		}
		ps.pagePristine = false
	case segmentIntermediateGenericRegion:
		header, err := parseGenericRegion(&r, false, strict)
		if err != nil {
			return false, err
		}
		region, err := decodeGenericRegion(header, state)
		if err != nil {
			return false, err
		}
		ps.storeRegion(seg.header.segmentNumber, region.bitmap, &state.budget)
	case segmentPatternDictionary, segmentSymbolDictionary, segmentTables, segmentExtension:
		if err := decodeDictionarySegment(seg, &ps.dictionaries, globals, standardTables, state, strict); err != nil {
			return false, err
		}
	case segmentImmediateTextRegion, segmentImmediateLosslessTextRegion:
		symbols, referredTables, err := ps.dictionaries.getSymbolsAndTables(seg.header.referredToSegments, globals, strict)
		if err != nil {
			return false, err
		}

		header, err := parseTextRegion(&r, uint32(len(symbols)), strict)
		if err != nil {
			return false, err
		}

		if ps.canDecodeDirectly(pageBitmap, &header.regionInfo, header.flags.defaultPixel) {
			if err := decodeTextRegionInto(header, symbols, referredTables, standardTables, pageBitmap, state); err != nil {
				return false, err
			}
		} else {
			region, err := decodeTextRegion(header, symbols, referredTables, standardTables, state)
			if err != nil {
				return false, err
			}
			if err := combineRegion(pageBitmap, region, state.cancel); err != nil {
				return false, err
			}
		}
		ps.pagePristine = false
	case segmentIntermediateTextRegion:
		symbols, referredTables, err := ps.dictionaries.getSymbolsAndTables(seg.header.referredToSegments, globals, strict)
		if err != nil {
			return false, err
		}

		header, err := parseTextRegion(&r, uint32(len(symbols)), strict)
		if err != nil {
			return false, err
		}
		region, err := decodeTextRegion(header, symbols, referredTables, standardTables, state)
		if err != nil {
			return false, err
		}
		ps.storeRegion(seg.header.segmentNumber, region.bitmap, &state.budget)
	case segmentImmediateHalftoneRegion, segmentImmediateLosslessHalftoneRegion:
		patternDict := ps.firstPatternDictionary(seg.header.referredToSegments, globals)
		if patternDict == nil {
			return false, ErrMissingPatternDictionary
		}

		header, err := parseHalftoneRegion(&r, strict)
		if err != nil {
			return false, err
		}

		if ps.canDecodeDirectly(pageBitmap, &header.regionInfo, header.flags.initialPixelColor) {
			if err := decodeHalftoneRegionInto(header, patternDict, pageBitmap, state); err != nil {
				return false, err
			}
		} else {
			region, err := decodeHalftoneRegion(header, patternDict, state)
			if err != nil {
				return false, err
			}
			if err := combineRegion(pageBitmap, region, state.cancel); err != nil {
				return false, err
			}
		}
		ps.pagePristine = false
	case segmentIntermediateHalftoneRegion:
		patternDict := ps.firstPatternDictionary(seg.header.referredToSegments, globals)
		if patternDict == nil {
			return false, ErrMissingPatternDictionary
		}

		header, err := parseHalftoneRegion(&r, strict)
		if err != nil {
			return false, err
		}
		region, err := decodeHalftoneRegion(header, patternDict, state)
		if err != nil {
			return false, err
		}
		ps.storeRegion(seg.header.segmentNumber, region.bitmap, &state.budget)
	case segmentIntermediateGenericRefinementRegion:
		referred, err := ps.getRefinedRegion(seg.header.referredToSegments, strict)
		if err != nil {
			return false, err
		}

		if strict && referred == nil {
			return false, ErrInvalidReference
		}

		header, err := parseGenericRefinementRegion(&r)
		if err != nil {
			return false, err
		}

		reference := referred
		if reference == nil {
			if reference, err = pageBitmapArea(pageBitmap, &header.regionInfo, &state.budget, state.cancel); err != nil {
				return false, err
			}
		}
		region, err := decodeGenericRefinementRegion(header, reference, state)
		if err != nil {
			return false, err
		}

		ps.storeRegion(seg.header.segmentNumber, region.bitmap, &state.budget)
	case segmentImmediateGenericRefinementRegion, segmentImmediateLosslessGenericRefinementRegion:
		referred, err := ps.getRefinedRegion(seg.header.referredToSegments, strict)
		if err != nil {
			return false, err
		}

		header, err := parseGenericRefinementRegion(&r)
		if err != nil {
			return false, err
		}

		if referred != nil && ps.canDecodeDirectly(pageBitmap, &header.regionInfo, false) {
			if err := decodeGenericRefinementRegionInto(header, referred, pageBitmap, state); err != nil {
				return false, err
			}
		} else {
			reference := referred
			if reference == nil {
				if reference, err = pageBitmapArea(pageBitmap, &header.regionInfo, &state.budget, state.cancel); err != nil {
					return false, err
				}
			}
			region, err := decodeGenericRefinementRegion(header, reference, state)
			if err != nil {
				return false, err
			}
			if err := combineRegion(pageBitmap, region, state.cancel); err != nil {
				return false, err
			}
		}

		ps.pagePristine = false
	case segmentEndOfPage, segmentEndOfFile:
		return true, nil
	}

	return false, nil
}

// combineRegion is the call page_bitmap.combine(&region.bitmap, ...) that
// decode_segment of upstream repeats for each kind of region. It draws the
// region into the page at its location.
func combineRegion(pageBitmap *bitmap, region regionBitmap, c cancel) error {
	return pageBitmap.combine(region.bitmap, int32(region.bitmap.xLocation), int32(region.bitmap.yLocation),
		region.combinationOperator, c)
}

// decodeDictionarySegment is decode_dictionary_segment in hayro-jbig2
// src/lib.rs.
func decodeDictionarySegment(seg *segment, dicts *dictionaries, globals *Globals,
	standardTables standardHuffmanTables, state *decodeState, strict bool) error {
	r := newReader(seg.data)

	switch seg.header.segmentType {
	case segmentPatternDictionary:
		header, err := parsePatternDictionary(&r, strict)
		if err != nil {
			return err
		}
		dictionary, err := decodePatternDictionary(header, state)
		if err != nil {
			return err
		}
		total := 0
		for _, p := range dictionary.patterns {
			total += p.memorySize()
		}
		state.budget.retain(total)
		dicts.storePatternDictionary(seg.header.segmentNumber, dictionary)
	case segmentSymbolDictionary:
		inputSymbols, referredTables, err := dicts.getSymbolsAndTables(seg.header.referredToSegments, globals, strict)
		if err != nil {
			return err
		}

		var retained *retainedContexts
		refs := seg.header.referredToSegments
		for i := len(refs) - 1; i >= 0; i-- {
			if dict := dicts.getSymbolDictionary(refs[i], globals); dict != nil {
				retained = dict.retainedContexts
				break
			}
		}

		header, err := parseSymbolDictionary(&r, strict)
		if err != nil {
			return err
		}
		dictionary, err := decodeSymbolDictionary(header, inputSymbols, referredTables, standardTables,
			retained, &state.budget, state.cancel)
		if err != nil {
			return err
		}
		dicts.storeSymbolDictionary(seg.header.segmentNumber, dictionary)
	case segmentTables:
		table, err := readCustomHuffmanTable(&r)
		if err != nil {
			return err
		}
		dicts.storeHuffmanTable(seg.header.segmentNumber, table)
	case segmentExtension:
		if !strict {
			break
		}
		extensionType, ok := r.readU32()
		if !ok {
			return ErrUnexpectedEOF
		}
		if extensionType&0x8000_0000 != 0 {
			return ErrUnsupported
		}
	}

	return nil
}

// pageState is PageState in hayro-jbig2 src/lib.rs.
type pageState struct {
	defaultPixel     bool
	pagePristine     bool
	referredSegments []numbered[*bitmap]
	dictionaries     dictionaries
}

// numbered is the tuple (u32, T) of upstream: a value and the number of its
// segment.
type numbered[T any] struct {
	segmentNumber uint32
	value         T
}

// newPageState is PageState::new.
func newPageState(defaultPixel bool) *pageState {
	return &pageState{defaultPixel: defaultPixel, pagePristine: true}
}

// canDecodeDirectly is PageState::can_decode_directly. It reports whether a
// region can decode into the page itself: the page is untouched, the region
// covers it, and the operator gives the pixels of the region.
func (ps *pageState) canDecodeDirectly(pageBitmap *bitmap, info *regionSegmentInfo, regionDefaultPixel bool) bool {
	if !ps.pagePristine {
		return false
	}

	coversPage := info.xLocation == 0 &&
		info.yLocation == 0 &&
		info.width == pageBitmap.width &&
		info.height == pageBitmap.height

	if !coversPage {
		return false
	}

	pageDefaultIsZero := !ps.defaultPixel

	if regionDefaultPixel == pageDefaultIsZero {
		return false
	}

	switch info.combinationOperator {
	case combinationReplace:
		return true
	case combinationOr, combinationXor:
		return pageDefaultIsZero
	}
	return !pageDefaultIsZero
}

// storeRegion is PageState::store_region.
func (ps *pageState) storeRegion(segmentNumber uint32, region *bitmap, budget *memoryBudget) {
	budget.retain(region.memorySize())
	ps.referredSegments = append(ps.referredSegments, numbered[*bitmap]{segmentNumber, region})
}

// getReferredSegment is PageState::get_referred_segment.
func (ps *pageState) getReferredSegment(segmentNumber uint32) *bitmap {
	return find(ps.referredSegments, segmentNumber)
}

// getRefinedRegion is PageState::get_refined_region. It returns the region
// of the first referred segment, or nil.
func (ps *pageState) getRefinedRegion(referredSegments []uint32, strict bool) (*bitmap, error) {
	if len(referredSegments) == 0 {
		return nil, nil
	}

	region := ps.getReferredSegment(referredSegments[0])
	if region == nil && strict {
		return nil, ErrInvalidReference
	}
	return region, nil
}

// firstPatternDictionary is the lookup of the pattern dictionary of the
// first referred segment in decode_segment of upstream.
func (ps *pageState) firstPatternDictionary(referredSegments []uint32, globals *Globals) *patternDictionary {
	if len(referredSegments) == 0 {
		return nil
	}
	return ps.dictionaries.getPatternDictionary(referredSegments[0], globals)
}

// dictionaries is Dictionaries in hayro-jbig2 src/lib.rs.
type dictionaries struct {
	patternDictionaries []numbered[*patternDictionary]
	symbolDictionaries  []numbered[*symbolDictionary]
	huffmanTables       []numbered[*huffmanTable]
}

// storePatternDictionary is Dictionaries::store_pattern_dictionary.
func (d *dictionaries) storePatternDictionary(segmentNumber uint32, dictionary *patternDictionary) {
	d.patternDictionaries = append(d.patternDictionaries, numbered[*patternDictionary]{segmentNumber, dictionary})
}

// storeSymbolDictionary is Dictionaries::store_symbol_dictionary.
func (d *dictionaries) storeSymbolDictionary(segmentNumber uint32, dictionary *symbolDictionary) {
	d.symbolDictionaries = append(d.symbolDictionaries, numbered[*symbolDictionary]{segmentNumber, dictionary})
}

// storeHuffmanTable is Dictionaries::store_huffman_table.
func (d *dictionaries) storeHuffmanTable(segmentNumber uint32, table *huffmanTable) {
	d.huffmanTables = append(d.huffmanTables, numbered[*huffmanTable]{segmentNumber, table})
}

// getPatternDictionary is Dictionaries::get_pattern_dictionary. It looks in
// the page first, then in the globals.
func (d *dictionaries) getPatternDictionary(segmentNumber uint32, globals *Globals) *patternDictionary {
	if v := find(d.patternDictionaries, segmentNumber); v != nil {
		return v
	}
	if globals == nil {
		return nil
	}
	return find(globals.dictionaries.patternDictionaries, segmentNumber)
}

// getSymbolDictionary is Dictionaries::get_symbol_dictionary. It looks in
// the page first, then in the globals.
func (d *dictionaries) getSymbolDictionary(segmentNumber uint32, globals *Globals) *symbolDictionary {
	if v := find(d.symbolDictionaries, segmentNumber); v != nil {
		return v
	}
	if globals == nil {
		return nil
	}
	return find(globals.dictionaries.symbolDictionaries, segmentNumber)
}

// getHuffmanTable is Dictionaries::get_huffman_table. It looks in the page
// first, then in the globals.
func (d *dictionaries) getHuffmanTable(segmentNumber uint32, globals *Globals) *huffmanTable {
	if v := find(d.huffmanTables, segmentNumber); v != nil {
		return v
	}
	if globals == nil {
		return nil
	}
	return find(globals.dictionaries.huffmanTables, segmentNumber)
}

// getSymbolsAndTables is Dictionaries::get_symbols_and_tables. It collects
// the exported symbols and the Huffman tables of the referred segments, in
// order.
func (d *dictionaries) getSymbolsAndTables(referredSegments []uint32, globals *Globals, strict bool) ([]*bitmap, []*huffmanTable, error) {
	var symbols []*bitmap
	var tables []*huffmanTable

	for _, num := range referredSegments {
		if dict := d.getSymbolDictionary(num, globals); dict != nil {
			if len(symbols)+len(dict.exportedSymbols) > int(maxSymbols) {
				return nil, nil, ErrOverflowSymbolCount
			}

			symbols = append(symbols, dict.exportedSymbols...)
		} else if table := d.getHuffmanTable(num, globals); table != nil {
			tables = append(tables, table)
		} else if strict {
			return nil, nil, ErrInvalidReference
		}
	}

	return symbols, tables, nil
}

// find is find in hayro-jbig2 src/lib.rs. It searches the entries, sorted by
// segment number, with a binary search. It returns the zero value of T when
// no entry has the number.
func find[T any](entries []numbered[T], segmentNumber uint32) T {
	i, ok := slices.BinarySearchFunc(entries, segmentNumber, func(e numbered[T], n uint32) int {
		return cmp.Compare(e.segmentNumber, n)
	})
	if !ok {
		var zero T
		return zero
	}
	return entries[i].value
}

// pageBitmapArea is page_bitmap_area in hayro-jbig2 src/lib.rs. It copies
// the area of the page under a region, as the reference of a refinement.
func pageBitmapArea(page *bitmap, info *regionSegmentInfo, budget *memoryBudget, c cancel) (*bitmap, error) {
	area, err := newBitmapWith(info.width, info.height, info.xLocation, info.yLocation, false, budget)
	if err != nil {
		return nil, err
	}
	if err := area.combine(page, negateLocation(info.xLocation), negateLocation(info.yLocation), combinationReplace, c); err != nil {
		return nil, err
	}

	return area, nil
}

// negateLocation is -i32::try_from(v).unwrap_or(i32::MAX) of upstream.
func negateLocation(v uint32) int32 {
	if v > math.MaxInt32 {
		return -math.MaxInt32
	}
	return -int32(v)
}
