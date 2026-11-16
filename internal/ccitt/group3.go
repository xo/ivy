// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt src/group3.rs. Each function name takes the
// name of its module, as okage D10 says, because decode and recover collide
// with other names.

package ccitt

// group3Decode1D is decode_1d in hayro-ccitt src/group3.rs. It decodes the
// lines of Group 3 data with one dimension.
func group3Decode1D(c *decoderContext, r *bitReader, decoder Decoder) error {
	// PDF producers often set EndOfLine wrongly, so the decoder always looks
	// for an EOL.
	c.hasEOLs = r.readEOLIfAvailable() > 0 || c.settings.EndOfLine

	for {
		if isCancelled(c.settings) {
			return ErrCancelled
		}

		lineStart := *r

		if err := decode1DLine(c, r); err != nil {
			if err := group3Recover(c, r, lineStart, err); err != nil {
				return err
			}
		}

		c.nextLine(decoder)

		done, err := group3CheckEOB(c, r)
		if err != nil {
			return err
		}
		if done {
			break
		}
	}

	return nil
}

// group3Decode2D is decode_2d in hayro-ccitt src/group3.rs. It decodes the
// lines of Group 3 data with two dimensions. A tag bit before each line
// picks the coding of the line.
func group3Decode2D(c *decoderContext, r *bitReader, decoder Decoder) error {
	// PDF producers often set EndOfLine wrongly, so the decoder always looks
	// for an EOL.
	c.hasEOLs = r.readEOLIfAvailable() > 0 || c.settings.EndOfLine

	for {
		if isCancelled(c.settings) {
			return ErrCancelled
		}

		lineStart := *r

		tagBit, err := r.readBit()
		if err == nil {
			if tagBit == 1 {
				err = decode1DLine(c, r)
			} else {
				err = decode2DLine(c, r)
			}
		}

		if err != nil {
			if err := group3Recover(c, r, lineStart, err); err != nil {
				return err
			}
		}

		c.nextLine(decoder)

		done, err := group3CheckEOB(c, r)
		if err != nil {
			return err
		}
		if done {
			break
		}
	}

	return nil
}

// group3Recover is recover in hayro-ccitt src/group3.rs. It fills the rest
// of a line that failed with white, when the data has EOL codes. Otherwise
// it returns err.
func group3Recover(c *decoderContext, r *bitReader, lineStart bitReader, err error) error {
	if !c.hasEOLs || c.settings.Strict {
		return err
	}

	// The first EOL after the line start ends the line, and group3CheckEOB
	// skips to it.
	*r = lineStart

	c.color = white
	return c.pushPixels(c.lineWidth - c.pixelsDecoded)
}

// group3CheckEOB is check_eob in hayro-ccitt src/group3.rs. It reads the
// EOL codes after a line, and it reports whether the image ends.
func group3CheckEOB(c *decoderContext, r *bitReader) (bool, error) {
	// Without EOLs, the padding of the alignment and the leading zeros of
	// the next line can look like an EOL.
	if c.settings.RowsAreByteAligned && !c.hasEOLs {
		r.align()
	}

	eolCount := group3ReadEOLs(c, r)
	rowsDone := c.decodedRows == c.settings.Rows

	if eolCount == 0 && c.hasEOLs && !rowsDone && !r.atEnd() {
		if c.settings.Strict {
			return false, ErrInvalidCode
		}

		if r.skipToEOL() {
			eolCount = group3ReadEOLs(c, r)
		} else {
			c.hasEOLs = false
		}
	}

	// PDFBOX-2778 has 7 EOLs instead of 6.
	if c.settings.EndOfBlock && eolCount >= 6 {
		return true, nil
	}

	return rowsDone || r.atEnd(), nil
}

// group3ReadEOLs is read_eols in hayro-ccitt src/group3.rs. In data with two
// dimensions, an RTC is six EOL codes that each have the tag bit 1, so the
// function also reads the tag bits between the EOL codes.
func group3ReadEOLs(c *decoderContext, r *bitReader) int {
	eolCount := r.readEOLIfAvailable()

	if c.settings.Encoding != Group3_2D {
		return eolCount
	}

	for eolCount > 0 {
		next := *r

		if bit, err := next.readBit(); err != nil || bit != 1 {
			break
		}

		count := next.readEOLIfAvailable()
		if count == 0 {
			// The tag bit belongs to the next line, unless it is the last
			// tag bit of an RTC.
			if eolCount >= 6 {
				*r = next
			}

			break
		}

		*r = next
		eolCount += count
	}

	return eolCount
}
