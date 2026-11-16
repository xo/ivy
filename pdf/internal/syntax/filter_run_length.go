// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/filter/run_length.rs.

package syntax

// decodeRunLength is decode in hayro-syntax src/filter/run_length.rs.
//
// Port: the output stops with a failure past maxLen bytes.
func decodeRunLength(data []byte, maxLen int64) ([]byte, bool) {
	r := NewReader(data)
	var decoded []byte
	for {
		length, ok := r.ReadU8()
		if !ok {
			return nil, false
		}
		switch {
		case length == 128:
			return decoded, true
		case length <= 127:
			// PDFBOX-3990: stop early if the stream is invalid.
			b, ok := r.ReadBytes(int(length) + 1)
			if !ok {
				return decoded, true
			}
			decoded = append(decoded, b...)
		default:
			n := 257 - int(length)
			b, ok := r.ReadU8()
			if !ok {
				return nil, false
			}
			for range n {
				decoded = append(decoded, b)
			}
		}
		if int64(len(decoded)) > maxLen {
			return nil, false
		}
	}
}
