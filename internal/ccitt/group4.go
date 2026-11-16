// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt src/group4.rs.

package ccitt

import "errors"

// group4Decode is decode in hayro-ccitt src/group4.rs. It decodes the lines
// of Group 4 data until the rows, an end-of-block marker or the data end.
// The name takes the name of its module, as okage D10 says.
func group4Decode(c *decoderContext, r *bitReader, decoder Decoder) error {
	for {
		if isCancelled(c.settings) {
			return ErrCancelled
		}

		// PDF allows EOLs. The decoder reads at most the two EOLs of an EOFB,
		// because data after an EOFB can look like an EOL.
		var eolCount int
		if c.settings.EndOfBlock {
			eolCount = r.readEOLs(2)
		} else {
			eolCount = r.readEOLIfAvailable()
		}

		if c.settings.EndOfBlock && eolCount >= 2 {
			break
		}

		if (c.settings.Rows != 0 && c.decodedRows == c.settings.Rows) || r.atEnd() {
			break
		}

		if err := decode2DLine(c, r); err != nil {
			if c.settings.Strict {
				return err
			}

			if c.pixelsDecoded > 0 {
				c.color = white
				if err := c.pushPixels(c.lineWidth - c.pixelsDecoded); err != nil {
					return err
				}
				c.nextLine(decoder)
			}

			if errors.Is(err, ErrUnexpectedEOF) {
				break
			}

			return err
		}

		c.nextLine(decoder)

		if c.settings.RowsAreByteAligned {
			r.align()
		}
	}

	return nil
}
