// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/ccitt.rs. The crate hayro-ccitt
// that it calls is the package internal/ccitt (D17).

package syntax

import (
	"errors"
	"fmt"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ccitt"
)

// decodeCcitt is decode in hayro-syntax src/filter/ccitt.rs. Where upstream
// returns None, it returns an error that wraps ivy.ErrFormat.
func decodeCcitt(data []byte, params Dict, imageParams *ImageDecodeParams, env *decodeEnv) (FilterResult, error) {
	k, ok := params.Get[int32](KeyK)
	if !ok {
		k = 0
	}

	columnsParam, ok := params.Get[int](KeyColumns)
	if !ok {
		columnsParam = 1728
	}
	columns := uint32(columnsParam)
	// The height of the image wins over Rows.
	var rows uint32
	if imageParams.Height > 0 {
		rows = imageParams.Height
	} else {
		rows, _ = params.Get[uint32](KeyRows)
	}

	endOfBlock, ok := params.Get[bool](KeyEndOfBlock)
	if !ok {
		endOfBlock = true
	}
	endOfLine, _ := params.Get[bool](KeyEndOfLine)
	rowsAreByteAligned, _ := params.Get[bool](KeyEncodedByteAlign)
	encoding := ccitt.Group3_1D
	switch {
	case k < 0:
		encoding = ccitt.Group4
	case k > 0:
		encoding = ccitt.Group3_2D
	}
	settings := ccitt.DecodeSettings{
		Columns:            columns,
		Rows:               rows,
		EndOfBlock:         endOfBlock,
		EndOfLine:          endOfLine,
		RowsAreByteAligned: rowsAreByteAligned,
		Encoding:           encoding,
		Strict:             false,
	}
	white := byte(0xFF)
	if blackIs1, _ := params.Get[bool](KeyBlackIs1); blackIs1 {
		white = 0x00
	}

	// Expand to 8 bits per component unless the image is indexed, for an
	// easier conversion to RGBA8.
	var (
		decoded     []byte
		bpc         uint8
		decodedRows uint32
	)
	if imageParams.IsIndexed {
		stride := (int64(columns) + 7) / 8
		// Port: the image must fit in env.maxMemory. Upstream does not bound
		// it.
		if stride*int64(rows) > env.maxMemory {
			return FilterResult{}, errCcittLimit(columns, rows)
		}
		decoder := &bitPackDecoder{
			output:       make([]byte, stride*int64(rows)),
			columns:      columns,
			stride:       int(stride),
			white:        white,
			lastByteMask: byte(0xFF) << ((8 - columns%8) % 8),
			maxMemory:    env.maxMemory,
		}
		fillBytes(decoder.output, white)
		settings.Cancel = ccitt.CancelFunc(func() bool { return decoder.overLimit })
		_, err := ccitt.Decode(data, decoder, &settings)
		if decoder.overLimit {
			return FilterResult{}, errCcittLimit(columns, rows)
		}

		// Lenient: return the partial output when at least one row decoded
		// (0001763.pdf).
		if err != nil && decoder.decodedRows == 0 {
			return FilterResult{}, errCcitt(err)
		}

		decoder.output = decoder.output[:int(decoder.decodedRows)*decoder.stride]

		decoded, bpc, decodedRows = decoder.output, 1, decoder.decodedRows
	} else {
		// Port: the image must fit in env.maxMemory. Upstream does not bound
		// it.
		if int64(columns)*int64(rows) > env.maxMemory {
			return FilterResult{}, errCcittLimit(columns, rows)
		}
		decoder := &luma8Decoder{
			output:    make([]byte, int64(columns)*int64(rows)),
			columns:   columns,
			white:     white,
			maxMemory: env.maxMemory,
		}
		fillBytes(decoder.output, white)
		settings.Cancel = ccitt.CancelFunc(func() bool { return decoder.overLimit })
		_, err := ccitt.Decode(data, decoder, &settings)
		if decoder.overLimit {
			return FilterResult{}, errCcittLimit(columns, rows)
		}

		if err != nil && decoder.decodedRows == 0 {
			return FilterResult{}, errCcitt(err)
		}

		if err != nil || rows == 0 {
			decoder.output = decoder.output[:int(decoder.decodedRows)*int(columns)]
		}

		decoded, bpc, decodedRows = decoder.output, 8, decoder.decodedRows
	}

	height := rows
	if rows == 0 {
		height = decodedRows
	}
	return FilterResult{
		Data: decoded,
		ImageData: &ImageData{
			ColorSpace:       &ImageColorSpace{Kind: ImageColorSpaceGray},
			BitsPerComponent: bpc,
			Width:            columns,
			Height:           height,
		},
	}, nil
}

// errCcitt returns the error of a CCITT image that did not decode. It wraps
// ivy.ErrFormat.
func errCcitt(err error) error {
	if errors.Is(err, ivy.ErrFormat) {
		return fmt.Errorf("decoding ccitt: %w", err)
	}
	return fmt.Errorf("decoding ccitt: %w: %w", ivy.ErrFormat, err)
}

// errCcittLimit returns the error of a CCITT image past the memory limit. It
// wraps ivy.ErrLimit.
func errCcittLimit(columns, rows uint32) error {
	return fmt.Errorf("decoding ccitt: image of %dx%d: %w", columns, rows, ivy.ErrLimit)
}

// bitPackDecoder is BitPackDecoder in decode in hayro-syntax
// src/filter/ccitt.rs. It packs the rows at 1 bit per pixel.
type bitPackDecoder struct {
	output       []byte
	columns      uint32
	stride       int
	white        byte
	lastByteMask byte
	decodedRows  uint32

	// Port: maxMemory bounds the output when Rows is 0, and overLimit stops
	// the decoder past it. Upstream does not bound the output.
	maxMemory int64
	overLimit bool
}

// PushRow is the implementation of hayro_ccitt::Decoder for BitPackDecoder.
func (d *bitPackDecoder) PushRow(changes []uint32) {
	if d.overLimit {
		return
	}
	start := int(d.decodedRows) * d.stride
	end := start + d.stride

	if len(d.output) < end {
		if int64(end) > d.maxMemory {
			d.overLimit = true
			return
		}
		d.output = growBytes(d.output, end, d.white)
	}

	row := d.output[start:end]

	// The rows start white, so a flip of a span makes it black.
	for span := range blackSpans(changes, d.columns) {
		first, last := span[0]/8, (span[1]-1)/8
		firstMask := byte(0xFF) >> (span[0] % 8)
		lastMask := byte(0xFF) << (7 - (span[1]-1)%8)

		if first == last {
			row[first] ^= firstMask & lastMask
		} else {
			row[first] ^= firstMask
			fillBytes(row[first+1:last], ^d.white)
			row[last] ^= lastMask
		}
	}

	if len(row) > 0 {
		row[len(row)-1] &= d.lastByteMask
	}

	d.decodedRows++
}

// luma8Decoder is Luma8Decoder in decode in hayro-syntax
// src/filter/ccitt.rs. It writes the rows at 8 bits per pixel.
type luma8Decoder struct {
	output      []byte
	columns     uint32
	white       byte
	decodedRows uint32

	// Port: maxMemory bounds the output when Rows is 0, and overLimit stops
	// the decoder past it. Upstream does not bound the output.
	maxMemory int64
	overLimit bool
}

// PushRow is the implementation of hayro_ccitt::Decoder for Luma8Decoder.
func (d *luma8Decoder) PushRow(changes []uint32) {
	if d.overLimit {
		return
	}
	start := int(d.decodedRows) * int(d.columns)
	end := start + int(d.columns)

	if len(d.output) < end {
		if int64(end) > d.maxMemory {
			d.overLimit = true
			return
		}
		d.output = growBytes(d.output, end, d.white)
	}

	row := d.output[start:end]

	for span := range blackSpans(changes, d.columns) {
		fillBytes(row[span[0]:span[1]], ^d.white)
	}

	d.decodedRows++
}

// blackSpans is black_spans in hayro-syntax src/filter/ccitt.rs. It yields
// each span of black pixels as its start and its end.
//
// Port: a span that is empty or past the row is left out, so that a fault
// of the decoder cannot make a panic. internal/ccitt never gives such a
// span.
func blackSpans(changes []uint32, columns uint32) func(yield func([2]int) bool) {
	return func(yield func([2]int) bool) {
		for i := 0; i < len(changes); i += 2 {
			end := columns
			if i+1 < len(changes) {
				end = changes[i+1]
			}
			if changes[i] >= end || end > columns {
				continue
			}
			if !yield([2]int{int(changes[i]), int(end)}) {
				return
			}
		}
	}
}

// fillBytes is fill_short in hayro-syntax src/filter/ccitt.rs. It sets each
// byte of s to value.
//
// Port: upstream uses stores of a fixed size for short spans, which beat a
// call of memset in Rust. Go fills the slice with a loop, which the compiler
// turns into a memset.
func fillBytes(s []byte, value byte) {
	for i := range s {
		s[i] = value
	}
}

// growBytes is Vec::resize in hayro-syntax src/filter/ccitt.rs. It extends
// s to n bytes, and it fills the new bytes with value.
func growBytes(s []byte, n int, value byte) []byte {
	old := len(s)
	s = append(s, make([]byte, n-old)...)
	fillBytes(s[old:], value)
	return s
}
