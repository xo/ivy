// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/lzw_flate.rs.
//
// Port: the inflate of miniz_oxide and zlib-rs becomes compress/flate, and
// the LZW decoder of upstream becomes golang.org/x/image/tiff/lzw for the
// early change of PDF and compress/lzw without it (D16).

package syntax

import (
	"bytes"
	"compress/flate"
	"compress/lzw"
	"errors"
	"fmt"
	"io"

	tifflzw "golang.org/x/image/tiff/lzw"

	"github.com/xo/ivy"
)

// windowSize is WINDOW_SIZE in hayro-syntax src/filter/lzw_flate.rs.
const windowSize = 32 * 1024

// zeroWindow is the preset dictionary of zeros that the second attempt of
// inflate uses.
var zeroWindow = make([]byte, windowSize)

// decodeFlate is flate::decode in hayro-syntax src/filter/lzw_flate.rs. It
// inflates the data and undoes the predictor.
//
// Port: the output stops with ivy.ErrLimit past maxMemory bytes.
func decodeFlate(data []byte, params Dict, maxMemory int64) ([]byte, error) {
	if body, ok := zlibBody(data); ok {
		data = body
	}
	// Broken encoders can refer to data before the start, which reads as
	// zeros.
	decoded, err := inflate(data, false, maxMemory)
	if err != nil {
		if errors.Is(err, ivy.ErrLimit) {
			return nil, err
		}
		decoded, err = inflate(data, true, maxMemory)
		if errors.Is(err, ivy.ErrLimit) {
			return nil, err
		}
		// Lenient: keep the partial output of a corrupt stream.
	}
	out, ok := applyPredictor(decoded, params)
	if !ok {
		return nil, fmt.Errorf("applying predictor: %w", ivy.ErrFormat)
	}
	return out, nil
}

// zlibBody is zlib_body in hayro-syntax src/filter/lzw_flate.rs. It returns
// the data after a valid zlib header.
func zlibBody(data []byte) ([]byte, bool) {
	if len(data) < 2 {
		return nil, false
	}
	cmf, flg := data[0], data[1]
	if cmf&0x0f == 0x08 && (uint16(cmf)<<8|uint16(flg))%31 == 0 && flg&0x20 == 0 {
		return data[2:], true
	}
	return nil, false
}

// inflate is inflate in hayro-syntax src/filter/lzw_flate.rs. It returns the
// output and a nil error when the stream ends or the input runs out, and the
// partial output and an error when the stream is corrupt.
func inflate(data []byte, zero bool, maxMemory int64) ([]byte, error) {
	var r io.ReadCloser
	if zero {
		r = flate.NewReaderDict(bytes.NewReader(data), zeroWindow)
	} else {
		r = flate.NewReader(bytes.NewReader(data))
	}
	return readLimited(r, maxMemory)
}

// readLimited reads r to its end. A truncated input is no error, as for
// upstream. It fails with ivy.ErrLimit past maxMemory bytes.
func readLimited(r io.Reader, maxMemory int64) ([]byte, error) {
	out, err := io.ReadAll(io.LimitReader(r, maxMemory+1))
	if int64(len(out)) > maxMemory {
		return nil, errLimit(maxMemory)
	}
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return out, fmt.Errorf("inflating stream: %w: %w", ivy.ErrFormat, err)
	}
	return out, nil
}

// decodeLzw is lzw::decode in hayro-syntax src/filter/lzw_flate.rs. It
// decodes the data and undoes the predictor.
//
// Port: decode_impl of upstream becomes the decoders of Go. A stream
// without the code EOD decodes, as for upstream.
func decodeLzw(data []byte, params Dict, maxMemory int64) ([]byte, error) {
	earlyChange := true
	if e, ok := params.Get[uint8](KeyEarlyChange); ok {
		earlyChange = e != 0
	}
	var r io.ReadCloser
	if earlyChange {
		r = tifflzw.NewReader(bytes.NewReader(data), tifflzw.MSB, 8)
	} else {
		r = lzw.NewReader(bytes.NewReader(data), lzw.MSB, 8)
	}
	decoded, err := readLimited(r, maxMemory)
	if err != nil {
		return nil, err
	}
	out, ok := applyPredictor(decoded, params)
	if !ok {
		return nil, fmt.Errorf("applying predictor: %w", ivy.ErrFormat)
	}
	return out, nil
}
