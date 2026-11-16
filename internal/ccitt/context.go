// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt src/context.rs.

package ccitt

// decoderContext is DecoderContext in hayro-ccitt src/context.rs. It holds
// the state of the decoder between the codes. In refChanges and
// codingChanges, the even indices are changes to black, and the odd indices
// are changes to white.
type decoderContext struct {
	refChanges    []uint32
	refPos        uint32
	b1Idx         uint32
	codingChanges []uint32
	pixelsDecoded uint32
	lineWidth     uint32
	color         color
	decodedRows   uint32
	settings      *DecodeSettings
	hasEOLs       bool
}

// newDecoderContext is DecoderContext::new in hayro-ccitt src/context.rs.
func newDecoderContext(settings *DecodeSettings) *decoderContext {
	return &decoderContext{
		lineWidth: settings.Columns,
		color:     white,
		settings:  settings,
	}
}

// pushPixels is DecoderContext::push_pixels in hayro-ccitt src/context.rs.
// It adds count pixels of the current color to the line.
func (c *decoderContext) pushPixels(count uint32) error {
	remaining := c.lineWidth - c.pixelsDecoded

	if count > remaining && c.settings.Strict {
		return ErrInvalidCode
	}

	count = min(count, remaining)

	if count > 0 {
		lastIsWhite := len(c.codingChanges)%2 == 0

		if lastIsWhite != c.color.isWhite() {
			c.codingChanges = append(c.codingChanges, c.pixelsDecoded)
		}

		c.pixelsDecoded += count
	}

	return nil
}

// nextLine is DecoderContext::next_line in hayro-ccitt src/context.rs. It
// gives the line to the decoder, and it makes the line the reference line.
func (c *decoderContext) nextLine(decoder Decoder) {
	decoder.PushRow(c.codingChanges)
	c.refChanges, c.codingChanges = c.codingChanges, c.refChanges
	c.codingChanges = c.codingChanges[:0]
	c.pixelsDecoded = 0
	c.refPos = 0
	c.b1Idx = 0
	c.color = white
	c.decodedRows++

	c.updateB()
}

// a0 is DecoderContext::a0 in hayro-ccitt src/context.rs. It returns false
// before the first pixel of the line.
func (c *decoderContext) a0() (uint32, bool) {
	if c.pixelsDecoded == 0 {
		return 0, false
	}
	return c.pixelsDecoded, true
}

// b1 is DecoderContext::b1 in hayro-ccitt src/context.rs.
func (c *decoderContext) b1() uint32 {
	if int(c.b1Idx) < len(c.refChanges) {
		return c.refChanges[c.b1Idx]
	}
	return c.lineWidth
}

// b2 is DecoderContext::b2 in hayro-ccitt src/context.rs.
func (c *decoderContext) b2() uint32 {
	if i := int(c.b1Idx) + 1; i < len(c.refChanges) {
		return c.refChanges[i]
	}
	return c.lineWidth
}

// updateB is DecoderContext::update_b in hayro-ccitt src/context.rs. It
// finds b1, the first change of the reference line right of a0 to the
// opposite of the current color.
func (c *decoderContext) updateB() {
	var minIdx uint32
	if a, ok := c.a0(); ok {
		minIdx = a + 1
	}
	idx := int(c.refPos)

	for idx < len(c.refChanges) && c.refChanges[idx] < minIdx {
		idx++
	}

	c.refPos = uint32(idx)

	if (idx%2 == 0) != c.color.isWhite() {
		idx++
	}

	c.b1Idx = uint32(idx)
}

// atEOL is DecoderContext::at_eol in hayro-ccitt src/context.rs.
func (c *decoderContext) atEOL() bool {
	a, _ := c.a0()
	return a == c.lineWidth
}

// decode1DLine is decode_1d_line in hayro-ccitt src/context.rs. It decodes
// a line of runs of Group 3.
func decode1DLine(c *decoderContext, r *bitReader) error {
	for !c.atEOL() {
		runLength, err := r.decodeRun(c.color)
		if err != nil {
			return err
		}
		if err := c.pushPixels(runLength); err != nil {
			return err
		}
		c.color = c.color.opposite()
	}

	return nil
}

// decode2DLine is decode_2d_line in hayro-ccitt src/context.rs. It decodes
// a line of modes of Group 3 or Group 4.
func decode2DLine(c *decoderContext, r *bitReader) error {
	for !c.atEOL() {
		m, err := r.decodeMode()
		if err != nil {
			return err
		}

		switch m.kind {
		case modePass:
			a0, _ := c.a0()
			if err := c.pushPixels(c.b2() - a0); err != nil {
				return err
			}
			c.updateB()
		case modeVertical:
			b1 := c.b1()
			var a1 uint32
			if m.offset >= 0 {
				d := uint32(m.offset)
				if b1 > ^uint32(0)-d {
					return ErrOverflow
				}
				a1 = b1 + d
			} else {
				d := uint32(-int32(m.offset))
				if b1 < d {
					return ErrOverflow
				}
				a1 = b1 - d
			}

			a0, _ := c.a0()
			if a1 < a0 {
				return ErrOverflow
			}
			if err := c.pushPixels(a1 - a0); err != nil {
				return err
			}
			c.color = c.color.opposite()

			c.updateB()
		case modeHorizontal:
			a0a1, err := r.decodeRun(c.color)
			if err != nil {
				return err
			}
			if err := c.pushPixels(a0a1); err != nil {
				return err
			}
			c.color = c.color.opposite()

			a1a2, err := r.decodeRun(c.color)
			if err != nil {
				return err
			}
			if err := c.pushPixels(a1a2); err != nil {
				return err
			}
			c.color = c.color.opposite()

			c.updateB()
		}
	}

	return nil
}

// color is Color in hayro-ccitt src/context.rs.
type color uint8

// The values of color.
const (
	white color = iota
	black
)

// opposite is Color::opposite in hayro-ccitt src/context.rs.
func (c color) opposite() color {
	if c == white {
		return black
	}
	return white
}

// isWhite is Color::is_white in hayro-ccitt src/context.rs.
func (c color) isWhite() bool {
	return c == white
}
