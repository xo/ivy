// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/jp2/cdef.rs.

import "slices"

// cdefParse is parse in hayro-jpeg2000 src/jp2/cdef.rs. It reads the
// channel definition box.
func cdefParse(boxes *imageBoxes, data []byte) error {
	reader := newBitReader(data)
	count, ok := reader.readU16()
	if !ok {
		return errInvalidBox
	}
	if count == 0 {
		return errInvalidBox
	}

	definitions := make([]channelDefinition, 0, min(int(count), len(data)/6))
	for range count {
		b, ok := reader.readBytes(6)
		if !ok {
			return errInvalidBox
		}
		index := uint16(b[0])<<8 | uint16(b[1])
		rawType := uint16(b[2])<<8 | uint16(b[3])
		rawAssociation := uint16(b[4])<<8 | uint16(b[5])

		t, ok := channelTypeFromRaw(rawType)
		if !ok {
			return errInvalidBox
		}
		a, ok := channelAssociationFromRaw(rawAssociation)
		if !ok {
			return errInvalidBox
		}
		definitions = append(definitions, channelDefinition{channelIndex: index, channelType: t, association: a})
	}

	slices.SortStableFunc(definitions, func(a, b channelDefinition) int {
		return int(a.channelIndex) - int(b.channelIndex)
	})

	for idx, def := range definitions {
		if int(def.channelIndex) != idx {
			return errInvalidBox
		}
	}

	boxes.channelDefinition = &channelDefinitionBox{channelDefinitions: definitions}
	return nil
}

// channelDefinitionBox is ChannelDefinitionBox in hayro-jpeg2000
// src/jp2/cdef.rs.
type channelDefinitionBox struct {
	channelDefinitions []channelDefinition
}

// channelDefinition is ChannelDefinition in hayro-jpeg2000 src/jp2/cdef.rs.
type channelDefinition struct {
	channelIndex uint16
	channelType  channelType
	association  channelAssociation
}

// channelType is ChannelType in hayro-jpeg2000 src/jp2/cdef.rs.
type channelType uint8

// The types of channels.
const (
	channelColour channelType = iota
	channelOpacity
)

// channelTypeFromRaw is ChannelType::from_raw.
func channelTypeFromRaw(value uint16) (channelType, bool) {
	switch value {
	case 0:
		return channelColour, true
	case 1:
		return channelOpacity, true
	}
	return 0, false
}

// channelAssociation is ChannelAssociation in hayro-jpeg2000
// src/jp2/cdef.rs. The value 0 is WholeImage, and another value is
// Colour with that value.
type channelAssociation uint16

// associationWholeImage is ChannelAssociation::WholeImage.
const associationWholeImage channelAssociation = 0

// channelAssociationFromRaw is ChannelAssociation::from_raw.
func channelAssociationFromRaw(value uint16) (channelAssociation, bool) {
	if value == 0xffff {
		return 0, false
	}
	return channelAssociation(value), true
}
