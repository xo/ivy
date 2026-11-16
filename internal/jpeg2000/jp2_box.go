// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/jp2/box.rs.

// The types of the boxes of a JP2 file. Some exist only for completeness.
const (
	boxJP2Signature         = 0x6A502020
	boxFileType             = 0x66747970
	boxJP2Header            = 0x6A703268
	boxImageHeader          = 0x69686472
	boxBitsPerComponent     = 0x62706363
	boxColourSpecification  = 0x636F6C72
	boxPalette              = 0x70636C72
	boxComponentMapping     = 0x636D6170
	boxChannelDefinition    = 0x63646566
	boxResolution           = 0x72657320
	boxCaptureResolution    = 0x72657363
	boxDisplayResolution    = 0x72657364
	boxContiguousCodestream = 0x6A703263
	boxIntellectualProperty = 0x6A703269
	boxXML                  = 0x786D6C20
	boxUUID                 = 0x75756964
	boxUUIDInfo             = 0x75696E66
	boxUUIDList             = 0x756C7374
	boxURL                  = 0x75726C20
)

// jp2Box is Jp2Box in hayro-jpeg2000 src/jp2/box.rs.
type jp2Box struct {
	data    []byte
	boxType uint32
}

// boxRead is read in hayro-jpeg2000 src/jp2/box.rs. It reads the next box.
func boxRead(reader *bitReader, strict bool) (jp2Box, bool) {
	lBox, ok := reader.readU32()
	if !ok {
		return jp2Box{}, false
	}
	tBox, ok := reader.readU32()
	if !ok {
		return jp2Box{}, false
	}

	var data []byte
	switch lBox {
	case 0:
		if data, ok = reader.tail(); !ok {
			return jp2Box{}, false
		}
		reader.jumpToEnd()
	case 1:
		xlBox, ok := reader.readU64()
		if !ok || xlBox < 16 {
			return jp2Box{}, false
		}
		n := xlBox - 16
		if n > uint64(len(reader.data)) {
			// Port: the length does not fit an int on every platform. Any
			// length past the data reads the rest of the data.
			n = uint64(len(reader.data)) + 1
		}
		if data, ok = readData(reader, int(n), strict); !ok {
			return jp2Box{}, false
		}
	default:
		if lBox < 8 {
			return jp2Box{}, false
		}
		if data, ok = readData(reader, int(lBox-8), strict); !ok {
			return jp2Box{}, false
		}
	}
	return jp2Box{data: data, boxType: tBox}, true
}

// readData is read_data in hayro-jpeg2000 src/jp2/box.rs.
func readData(reader *bitReader, length int, strict bool) ([]byte, bool) {
	if data, ok := reader.readBytes(length); ok {
		return data, true
	}

	// Some files declare a box length that exceeds the file, in which case
	// we use the remaining data instead.
	if strict {
		return nil, false
	}
	data, ok := reader.tail()
	if !ok {
		return nil, false
	}
	reader.jumpToEnd()
	return data, true
}
