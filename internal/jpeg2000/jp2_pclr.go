// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/jp2/pclr.rs.

// pclrParse is parse in hayro-jpeg2000 src/jp2/pclr.rs. It reads the
// palette box.
func pclrParse(boxes *imageBoxes, data []byte) error {
	reader := newBitReader(data)
	numEntries, ok := reader.readU16()
	if !ok {
		return errInvalidBox
	}
	numComponents, ok := reader.readByte()
	if !ok {
		return errInvalidBox
	}
	if numEntries == 0 || numComponents == 0 {
		return errInvalidBox
	}

	columns := make([]paletteColumn, 0, numComponents)
	for range numComponents {
		descriptor, ok := reader.readByte()
		if !ok {
			return errInvalidBox
		}
		bitDepth := descriptor&0x7f + 1
		if descriptor&0x80 != 0 {
			return errInvalidBox
		}
		columns = append(columns, paletteColumn{bitDepth: bitDepth})
	}

	// Port: upstream holds one Vec for each entry. The port holds the
	// entries in one slice, and grows it as it reads, so that a short box
	// cannot make it allocate.
	entries := make([]uint64, 0, min(int(numEntries)*int(numComponents), len(data)))
	for range numEntries {
		for _, column := range columns {
			numBytes := max(int(divCeil32(uint32(column.bitDepth), 8)), 1)
			raw, ok := reader.readBytes(numBytes)
			if !ok {
				return errInvalidBox
			}
			var value uint64
			for _, b := range raw {
				value = value<<8 | uint64(b)
			}
			entries = append(entries, value)
		}
	}

	boxes.palette = &paletteBox{entries: entries, columns: columns, numEntries: int(numEntries)}
	return nil
}

// paletteBox is PaletteBox in hayro-jpeg2000 src/jp2/pclr.rs.
type paletteBox struct {
	entries    []uint64
	columns    []paletteColumn
	numEntries int
}

// mapEntry is PaletteBox::map. It returns the value of the column of the
// entry.
func (p *paletteBox) mapEntry(entry, column int) (uint64, bool) {
	if entry < 0 || entry >= p.numEntries || column < 0 || column >= len(p.columns) {
		return 0, false
	}
	return p.entries[entry*len(p.columns)+column], true
}

// paletteColumn is PaletteColumn in hayro-jpeg2000 src/jp2/pclr.rs.
type paletteColumn struct {
	bitDepth uint8
}
