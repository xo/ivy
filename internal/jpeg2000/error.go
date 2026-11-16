// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/error.rs.

import (
	"context"

	"github.com/xo/ivy"
)

// ErrorKind is the case of DecodeError in hayro-jpeg2000 src/error.rs. Each
// case of upstream holds an enum of its own, such as FormatError.
//
// Port: the nested enums of upstream become one flat set of errors, and
// ErrorKind names the outer case.
type ErrorKind uint8

// The kinds of errors.
const (
	// KindFormat is DecodeError::Format: errors of the JP2 file format and
	// of its boxes.
	KindFormat ErrorKind = iota
	// KindMarker is DecodeError::Marker: errors of the markers of the
	// codestream.
	KindMarker
	// KindTile is DecodeError::Tile: errors of the tiles.
	KindTile
	// KindValidation is DecodeError::Validation: errors of the size and of
	// the parameters of the image.
	KindValidation
	// KindDecoding is DecodeError::Decoding: errors of the decoding.
	KindDecoding
	// KindColor is DecodeError::Color: errors of the color spaces and of
	// the components.
	KindColor
	// KindCancelled is DecodeError::Cancelled: the caller cancelled the
	// decoding.
	KindCancelled
)

// DecodeError is DecodeError in hayro-jpeg2000 src/error.rs. It is the
// error of each function of the package.
//
// Port: DecodeError wraps a sentinel error, so that errors.Is finds the
// cause. A malformed file wraps ivy.ErrFormat, an unsupported feature wraps
// ivy.ErrUnsupported, an image that is too large wraps ivy.ErrLimit, and a
// cancelled decoding wraps context.Canceled.
type DecodeError struct {
	kind  ErrorKind
	text  string
	cause error
	// stack holds the stack of a panic that recoverPanic turned into the
	// error.
	stack []byte
}

// Error satisfies the error interface. The text is the Display text of
// upstream.
func (err *DecodeError) Error() string {
	return "jpeg2000: " + err.text
}

// Unwrap returns the sentinel error.
func (err *DecodeError) Unwrap() error {
	return err.cause
}

// Kind returns the kind of the error.
func (err *DecodeError) Kind() ErrorKind {
	return err.kind
}

func newError(kind ErrorKind, text string, cause error) *DecodeError {
	return &DecodeError{kind: kind, text: text, cause: cause}
}

// errCancelled is Cancelled and DecodeError::Cancelled.
var errCancelled = newError(KindCancelled, "decoding was cancelled", context.Canceled)

// The cases of FormatError: errors of the JP2 file format and of its boxes.
var (
	// errInvalidSignature is FormatError::InvalidSignature.
	errInvalidSignature = newError(KindFormat, "invalid JP2 signature", ivy.ErrFormat)
	// errInvalidFileType is FormatError::InvalidFileType.
	errInvalidFileType = newError(KindFormat, "invalid JP2 file type", ivy.ErrFormat)
	// errInvalidBox is FormatError::InvalidBox.
	errInvalidBox = newError(KindFormat, "invalid JP2 box", ivy.ErrFormat)
	// errMissingCodestream is FormatError::MissingCodestream.
	errMissingCodestream = newError(KindFormat, "missing codestream data", ivy.ErrFormat)
	// errFormatUnsupported is FormatError::Unsupported.
	errFormatUnsupported = newError(KindFormat, "unsupported JP2 image", ivy.ErrUnsupported)
)

// The cases of MarkerError: errors of the markers of the codestream.
var (
	// errMarkerInvalid is MarkerError::Invalid.
	errMarkerInvalid = newError(KindMarker, "invalid marker", ivy.ErrFormat)
	// errMarkerUnsupported is MarkerError::Unsupported.
	errMarkerUnsupported = newError(KindMarker, "unsupported marker", ivy.ErrUnsupported)
)

// errMarkerExpected is MarkerError::Expected.
func errMarkerExpected(marker string) error {
	return newError(KindMarker, "expected "+marker+" marker", ivy.ErrFormat)
}

// errMarkerMissing is MarkerError::Missing.
func errMarkerMissing(marker string) error {
	return newError(KindMarker, "missing "+marker+" marker", ivy.ErrFormat)
}

// errMarkerParseFailure is MarkerError::ParseFailure.
func errMarkerParseFailure(marker string) error {
	return newError(KindMarker, "failed to parse "+marker+" marker", ivy.ErrFormat)
}

// The cases of TileError: errors of the tiles.
var (
	// errTileInvalid is TileError::Invalid.
	errTileInvalid = newError(KindTile, "image contains no tiles", ivy.ErrFormat)
	// errTileInvalidIndex is TileError::InvalidIndex.
	errTileInvalidIndex = newError(KindTile, "invalid tile index in tile-part header", ivy.ErrFormat)
	// errTileInvalidOffsets is TileError::InvalidOffsets.
	errTileInvalidOffsets = newError(KindTile, "invalid tile offsets", ivy.ErrFormat)
	// errTilePpmPptConflict is TileError::PpmPptConflict.
	errTilePpmPptConflict = newError(KindTile, "PPT marker present when PPM marker exists in main header", ivy.ErrFormat)
)

// The cases of ValidationError: errors of the size and of the parameters of
// the image.
var (
	// errInvalidDimensions is ValidationError::InvalidDimensions.
	errInvalidDimensions = newError(KindValidation, "invalid image dimensions", ivy.ErrFormat)
	// errImageTooLarge is ValidationError::ImageTooLarge. The limits of
	// DecodeSettings also return it.
	errImageTooLarge = newError(KindValidation, "image is too large", ivy.ErrLimit)
	// errTooManyChannels is ValidationError::TooManyChannels.
	errTooManyChannels = newError(KindValidation, "image has too many channels", ivy.ErrLimit)
	// errInvalidComponentMetadata is
	// ValidationError::InvalidComponentMetadata.
	errInvalidComponentMetadata = newError(KindValidation, "invalid component metadata", ivy.ErrFormat)
	// errInvalidProgressionOrder is ValidationError::InvalidProgressionOrder.
	errInvalidProgressionOrder = newError(KindValidation, "invalid progression order", ivy.ErrFormat)
	// errInvalidTransformation is ValidationError::InvalidTransformation.
	errInvalidTransformation = newError(KindValidation, "invalid transformation type", ivy.ErrFormat)
	// errInvalidQuantizationStyle is
	// ValidationError::InvalidQuantizationStyle.
	errInvalidQuantizationStyle = newError(KindValidation, "invalid quantization style", ivy.ErrFormat)
	// errMissingPrecinctExponents is
	// ValidationError::MissingPrecinctExponents.
	errMissingPrecinctExponents = newError(KindValidation, "missing exponents for precinct sizes", ivy.ErrFormat)
	// errInsufficientExponents is ValidationError::InsufficientExponents.
	errInsufficientExponents = newError(KindValidation, "not enough exponents provided in header", ivy.ErrFormat)
	// errMissingStepSize is ValidationError::MissingStepSize.
	errMissingStepSize = newError(KindValidation, "missing exponent step size", ivy.ErrFormat)
	// errInvalidExponents is ValidationError::InvalidExponents.
	errInvalidExponents = newError(KindValidation, "invalid quantization exponents", ivy.ErrFormat)
)

// The cases of DecodingError: errors of the decoding.
var (
	// errCodeBlockDecodeFailure is DecodingError::CodeBlockDecodeFailure.
	errCodeBlockDecodeFailure = newError(KindDecoding, "failed to decode code-block", ivy.ErrFormat)
	// errTooManyBitplanes is DecodingError::TooManyBitplanes.
	errTooManyBitplanes = newError(KindDecoding, "number of bitplanes is too large", ivy.ErrUnsupported)
	// errTooManyCodingPasses is DecodingError::TooManyCodingPasses.
	errTooManyCodingPasses = newError(KindDecoding, "code-block contains too many coding passes", ivy.ErrFormat)
	// errInvalidBitplaneCount is DecodingError::InvalidBitplaneCount.
	errInvalidBitplaneCount = newError(KindDecoding, "invalid number of bitplanes", ivy.ErrFormat)
	// errInvalidPrecinct is DecodingError::InvalidPrecinct.
	errInvalidPrecinct = newError(KindDecoding, "a precinct was invalid", ivy.ErrFormat)
	// errInvalidProgressionIterator is
	// DecodingError::InvalidProgressionIterator.
	errInvalidProgressionIterator = newError(KindDecoding, "a progression iterator was invalid", ivy.ErrFormat)
)

// The cases of ColorError: errors of the color spaces and of the
// components.
var (
	// errMct is ColorError::Mct.
	errMct = newError(KindColor, "multi-component transform failed", ivy.ErrFormat)
	// errPaletteResolutionFailed is ColorError::PaletteResolutionFailed.
	errPaletteResolutionFailed = newError(KindColor, "failed to resolve palette indices", ivy.ErrFormat)
	// errSyccConversionFailed is ColorError::SyccConversionFailed.
	errSyccConversionFailed = newError(KindColor, "failed to convert from sYCC to RGB", ivy.ErrFormat)
	// errLabConversionFailed is ColorError::LabConversionFailed.
	errLabConversionFailed = newError(KindColor, "failed to convert from LAB to RGB", ivy.ErrFormat)
)

// errInternal reports a state that an assertion of upstream rejects with a
// panic.
//
// Port: upstream panics on such a state. The port must never panic on its
// input, so it returns this error. It wraps ivy.ErrFormat, because only a
// malformed file can cause the state.
var errInternal = newError(KindDecoding, "inconsistent internal state", ivy.ErrFormat)
