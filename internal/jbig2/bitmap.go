// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/bitmap.rs.

// maxDimension is MAX_DIMENSION. It keeps the pixel coordinates and their
// offsets within an int32.
const maxDimension uint32 = 1 << 22

// maxArea is MAX_AREA, the largest number of pixels of a bitmap.
const maxArea uint64 = 0xFFFF * 0xFFFF

// word is Word in hayro-jbig2 src/bitmap.rs. A word holds 32 pixels, with the
// first pixel in the most significant bit.
type word = uint32

// The sizes of a word.
const (
	// wordBits is WORD_BITS.
	wordBits uint32 = 32
	// wordBytes is WORD_BYTES.
	wordBytes uint32 = wordBits / 8
	// wordShift is WORD_SHIFT.
	wordShift uint32 = wordBits - 1
)

// bitmapHeaderSize is size_of::<Bitmap>() of upstream on a 64-bit target.
//
// Port: the memory budget counts it for each bitmap. Go uses the number of
// upstream, so that the limits of the tests of upstream hold.
const bitmapHeaderSize = 48

// bitmap is Bitmap in hayro-jbig2 src/bitmap.rs. Each row holds stride
// words. A bit of 1 is black. The bits past the width are zero.
type bitmap struct {
	width     uint32
	height    uint32
	stride    uint32
	data      []word
	xLocation uint32
	yLocation uint32
}

// checkSize is check_size in hayro-jbig2 src/bitmap.rs.
func checkSize(width, height uint32) error {
	if width > maxDimension || height > maxDimension || uint64(width)*uint64(height) > maxArea {
		return ErrOverflowBitmapDimension
	}
	return nil
}

// newBitmap is Bitmap::new.
func newBitmap(width, height uint32, budget *memoryBudget) (*bitmap, error) {
	return newBitmapWith(width, height, 0, 0, false, budget)
}

// newBitmapWith is Bitmap::new_with. It charges the memory of the bitmap to
// the budget before it allocates the memory.
func newBitmapWith(width, height, xLocation, yLocation uint32, defaultPixel bool, budget *memoryBudget) (*bitmap, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}

	stride := divCeil(width, wordBits)
	n := int(stride) * int(height)
	if err := budget.charge(bitmapMemorySizeFor(n)); err != nil {
		return nil, err
	}

	data := make([]word, n)
	b := &bitmap{
		width:     width,
		height:    height,
		stride:    stride,
		data:      data,
		xLocation: xLocation,
		yLocation: yLocation,
	}

	if defaultPixel {
		for i := range data {
			data[i] = ^word(0)
		}
		b.clearPadding()
	}

	return b, nil
}

// memorySize is Bitmap::memory_size.
func (b *bitmap) memorySize() int {
	return bitmapMemorySizeFor(len(b.data))
}

// bitmapMemorySizeFor is Bitmap::memory_size_for.
func bitmapMemorySizeFor(words int) int {
	return words*int(wordBytes) + bitmapHeaderSize
}

// clearPadding is Bitmap::clear_padding. It clears the bits past the width.
func (b *bitmap) clearPadding() {
	trailing := b.width % wordBits
	if trailing == 0 || b.stride == 0 {
		return
	}

	mask := ^word(0) << (wordBits - trailing)
	for i := int(b.stride) - 1; i < len(b.data); i += int(b.stride) {
		b.data[i] &= mask
	}
}

// getPixel is Bitmap::get_pixel. It returns 0 outside the bitmap.
func (b *bitmap) getPixel(x, y uint32) uint8 {
	if x >= b.width || y >= b.height {
		return 0
	}

	i := int(y)*int(b.stride) + int(x/wordBits)
	bitPos := wordShift - x%wordBits
	return uint8((b.data[i] >> bitPos) & 1)
}

// setPixel is Bitmap::set_pixel. It ORs the value into the pixel, and it does
// nothing outside the bitmap.
func (b *bitmap) setPixel(x, y uint32, value uint8) {
	if x >= b.width || y >= b.height {
		return
	}

	i := int(y)*int(b.stride) + int(x/wordBits)
	bitPos := wordShift - x%wordBits
	b.data[i] |= word(value) << bitPos
}

// combine is Bitmap::combine. It combines other into b, with the top left
// corner of other at x, y.
func (b *bitmap) combine(other *bitmap, x, y int32, operator combinationOperator, c cancel) error {
	destXStart := max(x, 0)
	destXEnd := int32(min(int64(x)+int64(other.width), int64(maxInt32), int64(b.width)))

	if destXStart >= destXEnd {
		return nil
	}

	srcXStart := uint32(destXStart - x)
	dxStart := uint32(destXStart)
	dxEnd := uint32(destXEnd)

	srcHeight := int64(other.height)
	srcYStart := uint32(min(max(-int64(y), 0), srcHeight))
	srcYEnd := uint32(min(max(int64(b.height)-int64(y), 0), srcHeight))
	checkMask := rowMask(copyWork(other.width))

	for srcY := srcYStart; srcY < srcYEnd; srcY++ {
		if srcY&checkMask == checkMask {
			if err := c.check(); err != nil {
				return err
			}
		}

		destY := uint32(int64(y) + int64(srcY))

		firstWord := dxStart / wordBits
		lastWord := (dxEnd - 1) / wordBits

		for wordIdx := firstWord; wordIdx <= lastWord; wordIdx++ {
			wordStartX := wordIdx * wordBits
			wordEndX := wordStartX + wordBits

			pxStart := max(dxStart, wordStartX)
			pxEnd := min(dxEnd, wordEndX)

			bitStart := pxStart - wordStartX
			bitEnd := pxEnd - wordStartX

			var mask word
			if bitEnd == wordBits {
				mask = ^word(0) >> bitStart
			} else {
				mask = (^word(0) >> bitStart) &^ (^word(0) >> bitEnd)
			}

			srcXForRange := srcXStart + (pxStart - dxStart)
			srcWordIdx := srcXForRange / wordBits
			srcBitOffset := srcXForRange % wordBits

			srcWord1 := other.getWord(srcY, srcWordIdx)
			srcWord2 := other.getWord(srcY, srcWordIdx+1)

			srcRaw := srcWord1
			if srcBitOffset != 0 {
				srcRaw = srcWord1<<srcBitOffset | srcWord2>>(wordBits-srcBitOffset)
			}
			srcAligned := srcRaw >> bitStart

			destIdx := int(destY)*int(b.stride) + int(wordIdx)
			destWord := b.data[destIdx]

			var result word
			switch operator {
			case combinationOr:
				result = destWord | srcAligned
			case combinationAnd:
				result = destWord & srcAligned
			case combinationXor:
				result = destWord ^ srcAligned
			case combinationXnor:
				result = ^(destWord ^ srcAligned)
			case combinationReplace:
				result = srcAligned
			}

			b.data[destIdx] = (destWord &^ mask) | (result & mask)
		}
	}

	return nil
}

// getWord is Bitmap::get_word. It returns 0 outside the bitmap.
func (b *bitmap) getWord(row, wordIdx uint32) word {
	if row >= b.height || wordIdx >= b.stride {
		return 0
	}
	return b.data[int(row)*int(b.stride)+int(wordIdx)]
}

// maxInt32 is i32::MAX.
const maxInt32 = 1<<31 - 1
