// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/jp2/cmap.rs.

// cmapParse is parse in hayro-jpeg2000 src/jp2/cmap.rs. It reads the
// component mapping box.
func cmapParse(boxes *imageBoxes, data []byte) error {
	reader := newBitReader(data)
	entries := make([]componentMappingEntry, 0, len(data)/4)

	for !reader.atEnd() {
		idx, ok := reader.readU16()
		if !ok {
			return errInvalidBox
		}
		mappingType, ok := reader.readByte()
		if !ok {
			return errInvalidBox
		}
		column, ok := reader.readByte()
		if !ok {
			return errInvalidBox
		}

		e := componentMappingEntry{componentIndex: idx}
		switch mappingType {
		case 0:
		case 1:
			e.palette = true
			e.column = column
		default:
			return errInvalidBox
		}
		entries = append(entries, e)
	}

	boxes.componentMapping = &componentMappingBox{entries: entries}
	return nil
}

// componentMappingBox is ComponentMappingBox in hayro-jpeg2000
// src/jp2/cmap.rs.
type componentMappingBox struct {
	entries []componentMappingEntry
}

// componentMappingEntry is ComponentMappingEntry in hayro-jpeg2000
// src/jp2/cmap.rs.
//
// Port: the enum ComponentMappingType becomes the fields palette and
// column. When palette is false, the mapping is Direct.
type componentMappingEntry struct {
	componentIndex uint16
	palette        bool
	column         uint8
}
