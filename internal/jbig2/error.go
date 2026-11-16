// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/error.rs.
//
// Port: DecodeError of upstream is an enum that holds the other kinds of
// errors. Go returns the interface error. Each kind of error is a Go type of
// its own, and each value unwraps to a sentinel error of ivy, so that a caller
// can test it with errors.Is. The macros bail! and err! become return
// statements.

import (
	"context"

	"github.com/xo/ivy"
)

// errorText returns the message of an error of this package. The index 0 is
// not a valid error.
func errorText(messages []string, i uint8) string {
	if i == 0 || int(i) >= len(messages) {
		return "jbig2: unknown error"
	}
	return "jbig2: " + messages[i]
}

// DecodeError is the part of DecodeError in hayro-jbig2 src/error.rs that
// holds no other error.
type DecodeError uint8

const (
	// ErrMemoryLimitExceeded is DecodeError::MemoryLimitExceeded. Decoding
	// needs more memory than the limit in the settings.
	ErrMemoryLimitExceeded DecodeError = iota + 1
	// ErrUnsupported is DecodeError::Unsupported. The data uses a feature that
	// the decoder does not implement.
	ErrUnsupported
	// ErrCancelled is DecodeError::Cancelled and Cancelled. The check for
	// cancellation stopped the decoder.
	ErrCancelled
)

var decodeErrorMessages = []string{
	"",
	"memory limit exceeded",
	"unsupported feature",
	"decoding was cancelled",
}

// Error satisfies the error interface. It is fmt::Display for DecodeError.
func (e DecodeError) Error() string {
	return errorText(decodeErrorMessages, uint8(e))
}

// Unwrap returns the sentinel error of the error. ErrCancelled unwraps to
// context.Canceled.
func (e DecodeError) Unwrap() error {
	switch e {
	case ErrMemoryLimitExceeded:
		return ivy.ErrLimit
	case ErrUnsupported:
		return ivy.ErrUnsupported
	case ErrCancelled:
		return context.Canceled
	}
	return ivy.ErrFormat
}

// ParseError is ParseError in hayro-jbig2 src/error.rs. It is an error of
// reading data.
type ParseError uint8

const (
	// ErrUnexpectedEOF is ParseError::UnexpectedEof. The input ends too early.
	ErrUnexpectedEOF ParseError = iota + 1
)

var parseErrorMessages = []string{
	"",
	"unexpected end of input",
}

// Error satisfies the error interface.
func (e ParseError) Error() string {
	return errorText(parseErrorMessages, uint8(e))
}

// Unwrap returns ivy.ErrFormat.
func (e ParseError) Unwrap() error {
	return ivy.ErrFormat
}

// FormatError is FormatError in hayro-jbig2 src/error.rs. It is an error of
// the structure of the file.
type FormatError uint8

const (
	// ErrInvalidHeader is FormatError::InvalidHeader. The signature of the
	// file header is wrong.
	ErrInvalidHeader FormatError = iota + 1
	// ErrReservedBits is FormatError::ReservedBits. Reserved bits are not
	// zero.
	ErrReservedBits
	// ErrMissingPageInfo is FormatError::MissingPageInfo. The page has no
	// page information segment.
	ErrMissingPageInfo
	// ErrUnknownPageHeight is FormatError::UnknownPageHeight. The height of
	// the page is unknown, and no stripe segment gives it.
	ErrUnknownPageHeight
	// ErrEmptyPage is FormatError::EmptyPage. The page has a width or a
	// height of zero.
	ErrEmptyPage
	// ErrPageCountMismatch is FormatError::PageCountMismatch. The number of
	// pages in the file header is not the number of pages in the file.
	ErrPageCountMismatch
	// ErrInvalidEndOfPage is FormatError::InvalidEndOfPage. A page does not
	// have exactly one end of page segment.
	ErrInvalidEndOfPage
)

var formatErrorMessages = []string{
	"",
	"invalid JBIG2 file header",
	"reserved bits must be zero",
	"missing page information segment",
	"page height unknown with no stripe segments",
	"page has zero width or height",
	"page count in file header does not match",
	"page without exactly one end of page segment",
}

// Error satisfies the error interface.
func (e FormatError) Error() string {
	return errorText(formatErrorMessages, uint8(e))
}

// Unwrap returns ivy.ErrFormat.
func (e FormatError) Unwrap() error {
	return ivy.ErrFormat
}

// SegmentError is SegmentError in hayro-jbig2 src/error.rs. It is an error of
// a segment.
type SegmentError uint8

const (
	// ErrUnknownSegmentType is SegmentError::UnknownType. The type of the
	// segment is unknown or reserved.
	ErrUnknownSegmentType SegmentError = iota + 1
	// ErrInvalidReferredCount is SegmentError::InvalidReferredCount. The
	// number of referred-to segments is not valid.
	ErrInvalidReferredCount
	// ErrInvalidReference is SegmentError::InvalidReference. A segment refers
	// to a segment that it must not refer to.
	ErrInvalidReference
	// ErrInvalidUnknownLength is SegmentError::InvalidUnknownLength. A
	// segment other than an immediate generic region has an unknown length.
	ErrInvalidUnknownLength
	// ErrMissingEndMarker is SegmentError::MissingEndMarker. A region of
	// unknown length has no end marker.
	ErrMissingEndMarker
	// ErrMissingPatternDictionary is SegmentError::MissingPatternDictionary.
	// A halftone region has no pattern dictionary.
	ErrMissingPatternDictionary
	// ErrMissingPageAssociation is SegmentError::MissingPageAssociation. A
	// segment of a page is not associated with a page.
	ErrMissingPageAssociation
)

var segmentErrorMessages = []string{
	"",
	"unknown or reserved segment type",
	"invalid referred-to segment count",
	"segment refers to an invalid segment",
	"unknown segment length for this type",
	"missing end marker for unknown-length region",
	"missing required pattern dictionary",
	"segment not associated with a page",
}

// Error satisfies the error interface.
func (e SegmentError) Error() string {
	return errorText(segmentErrorMessages, uint8(e))
}

// Unwrap returns ivy.ErrFormat.
func (e SegmentError) Unwrap() error {
	return ivy.ErrFormat
}

// HuffmanError is HuffmanError in hayro-jbig2 src/error.rs. It is an error of
// Huffman decoding.
type HuffmanError uint8

const (
	// ErrHuffmanInvalidCode is HuffmanError::InvalidCode. A Huffman code is
	// not valid.
	ErrHuffmanInvalidCode HuffmanError = iota + 1
	// ErrHuffmanInvalidSelection is HuffmanError::InvalidSelection. The
	// selection of a Huffman table is not valid.
	ErrHuffmanInvalidSelection
	// ErrHuffmanMissingTables is HuffmanError::MissingTables. The segment
	// refers to too few Huffman tables.
	ErrHuffmanMissingTables
	// ErrHuffmanUnexpectedOOB is HuffmanError::UnexpectedOob. An
	// out-of-band value occurs where it is not allowed.
	ErrHuffmanUnexpectedOOB
	// ErrHuffmanConflictingCodes is HuffmanError::ConflictingCodes. A Huffman
	// table has codes that conflict.
	ErrHuffmanConflictingCodes
)

var huffmanErrorMessages = []string{
	"",
	"invalid Huffman code",
	"invalid Huffman table selection",
	"not enough referred Huffman tables",
	"unexpected out-of-band value",
	"conflicting Huffman codes",
}

// Error satisfies the error interface.
func (e HuffmanError) Error() string {
	return errorText(huffmanErrorMessages, uint8(e))
}

// Unwrap returns ivy.ErrFormat.
func (e HuffmanError) Unwrap() error {
	return ivy.ErrFormat
}

// RegionError is RegionError in hayro-jbig2 src/error.rs. It is an error of
// the parameters of a region.
type RegionError uint8

const (
	// ErrInvalidCombinationOperator is
	// RegionError::InvalidCombinationOperator. The combination operator is
	// not valid.
	ErrInvalidCombinationOperator RegionError = iota + 1
	// ErrInvalidDimension is RegionError::InvalidDimension. A region has a
	// dimension that is not valid.
	ErrInvalidDimension
	// ErrInvalidMMRData is RegionError::InvalidMmrData. MMR decoding failed.
	ErrInvalidMMRData
	// ErrGrayScaleOutOfRange is RegionError::GrayScaleOutOfRange. A
	// gray-scale value is not less than the number of patterns.
	ErrGrayScaleOutOfRange
)

var regionErrorMessages = []string{
	"",
	"invalid combination operator",
	"invalid dimension value",
	"invalid MMR-encoded data",
	"gray-scale value exceeds pattern count",
}

// Error satisfies the error interface.
func (e RegionError) Error() string {
	return errorText(regionErrorMessages, uint8(e))
}

// Unwrap returns ivy.ErrFormat.
func (e RegionError) Unwrap() error {
	return ivy.ErrFormat
}

// TemplateError is TemplateError in hayro-jbig2 src/error.rs. It is an error
// of the configuration of a template.
type TemplateError uint8

const (
	// ErrTemplateInvalid is TemplateError::Invalid. The template value is not
	// valid.
	ErrTemplateInvalid TemplateError = iota + 1
	// ErrInvalidATPixel is TemplateError::InvalidAtPixel. The location of an
	// adaptive template pixel is not valid.
	ErrInvalidATPixel
)

var templateErrorMessages = []string{
	"",
	"invalid template value",
	"invalid adaptive template pixel location",
}

// Error satisfies the error interface.
func (e TemplateError) Error() string {
	return errorText(templateErrorMessages, uint8(e))
}

// Unwrap returns ivy.ErrFormat.
func (e TemplateError) Unwrap() error {
	return ivy.ErrFormat
}

// SymbolError is SymbolError in hayro-jbig2 src/error.rs. It is an error of
// the symbols.
type SymbolError uint8

const (
	// ErrNoSymbols is SymbolError::NoSymbols. No symbols are available.
	ErrNoSymbols SymbolError = iota + 1
	// ErrTooManySymbols is SymbolError::TooManySymbols. The symbol
	// dictionary holds more symbols than it declares.
	ErrTooManySymbols
	// ErrSymbolOutOfRange is SymbolError::OutOfRange. A symbol ID is out of
	// range.
	ErrSymbolOutOfRange
	// ErrSymbolUnexpectedOOB is SymbolError::UnexpectedOob. An out-of-band
	// value occurs where it is not allowed.
	ErrSymbolUnexpectedOOB
	// ErrSymbolInvalid is SymbolError::Invalid. A symbol is not valid.
	ErrSymbolInvalid
	// ErrInvalidStripDelta is SymbolError::InvalidStripDelta. The
	// multiplication of a strip delta overflowed.
	ErrInvalidStripDelta
	// ErrTooManyInstances is SymbolError::TooManyInstances. A text region
	// holds too many symbol instances.
	ErrTooManyInstances
)

var symbolErrorMessages = []string{
	"",
	"no symbols available",
	"symbol dictionary contains too many symbols",
	"symbol ID out of range",
	"unexpected out-of-band value",
	"invalid symbol encountered",
	"strip delta overflow",
	"too many symbol instances",
}

// Error satisfies the error interface.
func (e SymbolError) Error() string {
	return errorText(symbolErrorMessages, uint8(e))
}

// Unwrap returns ivy.ErrFormat.
func (e SymbolError) Unwrap() error {
	return ivy.ErrFormat
}

// OverflowError is OverflowError in hayro-jbig2 src/error.rs. It is an
// arithmetic overflow, or a value past a fixed limit of the decoder.
type OverflowError uint8

const (
	// ErrOverflowBitmapDimension is OverflowError::BitmapDimension. A
	// dimension of a bitmap is too large.
	ErrOverflowBitmapDimension OverflowError = iota + 1
	// ErrOverflowGridDimension is OverflowError::GridDimension. A dimension
	// of a halftone grid is too large.
	ErrOverflowGridDimension
	// ErrOverflowPlacementCoordinate is OverflowError::PlacementCoordinate.
	// The computation of a placement coordinate overflowed.
	ErrOverflowPlacementCoordinate
	// ErrOverflowReferenceOffset is OverflowError::ReferenceOffset. The
	// computation of a reference offset overflowed.
	ErrOverflowReferenceOffset
	// ErrOverflowIndex is OverflowError::Index. The computation of an index
	// overflowed.
	ErrOverflowIndex
	// ErrOverflowSymbolCount is OverflowError::SymbolCount. A symbol
	// dictionary or a region has too many symbols.
	ErrOverflowSymbolCount
)

var overflowErrorMessages = []string{
	"",
	"overflow in bitmap dimension computation",
	"overflow in grid dimension computation",
	"overflow in placement coordinate computation",
	"overflow in reference offset computation",
	"overflow in index computation",
	"too many symbols",
}

// Error satisfies the error interface.
func (e OverflowError) Error() string {
	return errorText(overflowErrorMessages, uint8(e))
}

// Unwrap returns ivy.ErrLimit for a value past a fixed limit of the decoder,
// and ivy.ErrFormat for the other errors.
func (e OverflowError) Unwrap() error {
	switch e {
	case ErrOverflowBitmapDimension, ErrOverflowGridDimension, ErrOverflowSymbolCount:
		return ivy.ErrLimit
	}
	return ivy.ErrFormat
}
