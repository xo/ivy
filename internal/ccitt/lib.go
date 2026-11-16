// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt src/lib.rs.

// Package ccitt decodes images of CCITT fax data. It is a port of the crate
// hayro-ccitt of hayro (D17).
//
// The package decodes Group 3 and Group 4 data, as ITU-T T.4 and T.6 define
// them. PDF files and fax files use these codings for images of black and
// white pixels. Decode takes the data and the DecodeSettings. It gives each
// row to a Decoder as a list of the positions where the color changes.
package ccitt

// Decode is decode in hayro-ccitt src/lib.rs. It decodes data into decoder,
// and it returns the number of bytes that it read. Each error unwraps to
// ivy.ErrFormat, or to context.Canceled for ErrCancelled.
func Decode(data []byte, decoder Decoder, settings *DecodeSettings) (int, error) {
	if settings.Columns == 0 {
		return 0, nil
	}

	r := newBitReader(data)
	ctx := newDecoderContext(settings)

	var err error
	switch settings.Encoding {
	case Group3_1D:
		err = group3Decode1D(ctx, &r, decoder)
	case Group3_2D:
		err = group3Decode2D(ctx, &r, decoder)
	default:
		err = group4Decode(ctx, &r, decoder)
	}
	if err != nil {
		return 0, err
	}

	r.align()
	return r.bytePos(), nil
}

// Decoder is Decoder in hayro-ccitt src/lib.rs. It receives the rows of a
// CCITT image.
type Decoder interface {
	// PushRow receives a row as the ascending positions where its color
	// changes. The row starts white. The slice is valid only during the
	// call, because the decoder uses it again for a later row.
	PushRow(changes []uint32)
}

// DecodeSettings is DecodeSettings in hayro-ccitt src/lib.rs. It holds the
// settings of a call of Decode.
type DecodeSettings struct {
	// Columns is the width of the image.
	Columns uint32
	// Rows is the height of the image. A value of 0 decodes until an
	// end-of-block marker or the end of the data.
	Rows uint32
	// EndOfBlock is true if the data can hold an end-of-block marker.
	EndOfBlock bool
	// EndOfLine is true if the data holds end-of-line markers.
	EndOfLine bool
	// RowsAreByteAligned is true if each row starts at a byte boundary.
	RowsAreByteAligned bool
	// Encoding is the coding of the data. The zero value is Group4.
	//
	// Port: a value that is not a constant of EncodingMode decodes as
	// Group 4. Rust cannot make such a value.
	Encoding EncodingMode
	// Strict is true to reject malformed data. If Strict is false, the
	// decoder decodes malformed data as well as it can.
	Strict bool
	// Cancel is the check for cancellation, or nil.
	//
	// Port: upstream holds an Option of CancelHandle. A nil interface value
	// is the None of upstream.
	Cancel CancelCheck
}

// EncodingMode is EncodingMode in hayro-ccitt src/lib.rs. It is a coding of
// CCITT data.
type EncodingMode uint8

// The values of EncodingMode. The names keep the underscore of upstream,
// because Group31D does not read as Group 3 1D.
const (
	// Group4 is EncodingMode::Group4, Group 4 (MMR).
	Group4 EncodingMode = iota
	// Group3_1D is EncodingMode::Group3_1D, Group 3 with one dimension (MH).
	Group3_1D
	// Group3_2D is EncodingMode::Group3_2D, Group 3 with two dimensions
	// (MR).
	Group3_2D
)
