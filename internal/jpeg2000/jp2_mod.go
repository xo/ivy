// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/jp2/mod.rs.

// imageBoxes is ImageBoxes in hayro-jpeg2000 src/jp2/mod.rs. A nil field is
// a box that the file does not hold.
type imageBoxes struct {
	colorSpecification *colorSpecificationBox
	channelDefinition  *channelDefinitionBox
	palette            *paletteBox
	componentMapping   *componentMappingBox
}

// DecodedImage is DecodedImage in hayro-jpeg2000 src/jp2/mod.rs. It is a
// decoded image. Its components belong to the DecoderContext of the
// decoding, and the next decoding with that context reuses them.
type DecodedImage struct {
	decodedComponents []ComponentData
	boxes             imageBoxes
	cancel            cancel
	lim               *limits
}

// jp2Parse is parse in hayro-jpeg2000 src/jp2/mod.rs. It opens a JP2 file.
func jp2Parse(data []byte, settings DecodeSettings) (*Image, error) {
	reader := newBitReader(data)
	signature, ok := boxRead(&reader, settings.Strict)
	if !ok {
		return nil, errInvalidBox
	}
	if signature.boxType != boxJP2Signature {
		return nil, errInvalidSignature
	}

	fileType, ok := boxRead(&reader, settings.Strict)
	if !ok {
		return nil, errInvalidBox
	}
	if fileType.boxType != boxFileType {
		return nil, errInvalidFileType
	}

	var boxes *imageBoxes
	var pc *parsedCodestream

	for !reader.atEnd() {
		current, ok := boxRead(&reader, settings.Strict)
		if !ok {
			if settings.Strict {
				return nil, errInvalidBox
			}
			break
		}

		switch current.boxType {
		case boxJP2Header:
			b := &imageBoxes{}
			jp2h := newBitReader(current.data)
			for !jp2h.atEnd() {
				child, ok := boxRead(&jp2h, settings.Strict)
				if !ok {
					return nil, errInvalidBox
				}
				switch child.boxType {
				case boxChannelDefinition:
					if err := cdefParse(b, child.data); err != nil && settings.Strict {
						return nil, errInvalidBox
					}
				case boxColourSpecification:
					if err := colrParse(b, child.data); err != nil {
						return nil, err
					}
				case boxPalette:
					if err := pclrParse(b, child.data); err != nil && settings.Strict {
						return nil, errInvalidBox
					}
					// A decoding at a reduced resolution corrupts the
					// indices of a palette.
					settings.TargetResolution = nil
				case boxComponentMapping:
					if err := cmapParse(b, child.data); err != nil {
						return nil, err
					}
				}
			}
			boxes = b
		case boxContiguousCodestream:
			p, err := parseRaw(current.data, &settings)
			if err != nil {
				return nil, err
			}
			pc = &p
		}
	}

	if boxes == nil {
		return nil, errInvalidBox
	}
	if pc == nil {
		return nil, errMissingCodestream
	}

	if boxes.palette != nil && boxes.componentMapping == nil {
		// Some files have a pclr box without cmap. Map every channel
		// through the palette.
		entries := make([]componentMappingEntry, len(boxes.palette.columns))
		for i := range entries {
			entries[i] = componentMappingEntry{palette: true, column: uint8(i)}
		}
		boxes.componentMapping = &componentMappingBox{entries: entries}
	}

	cs, hasAlpha, err := resolveAlphaAndColorSpace(boxes, pc.header, &settings)
	if err != nil {
		return nil, err
	}

	return &Image{
		codestream: pc.data,
		header:     pc.header,
		boxes:      *boxes,
		settings:   settings,
		colorSpace: cs,
		hasAlpha:   hasAlpha,
	}, nil
}
