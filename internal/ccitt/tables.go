// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt src/tables.rs.
//
// Port: upstream builds the lookup tables in a const fn at compile time. Go
// builds them when the package starts.

package ccitt

import "math/bits"

// lookupBits is LOOKUP_BITS, the number of bits that index a lookup
// table.
const lookupBits = 13

// lookupInvalid is LOOKUP_INVALID, the run length of an entry for no
// valid code.
const lookupInvalid uint16 = 0xFFF

// The lookup tables of the runs of each color, WHITE_LOOKUP and BLACK_LOOKUP.
var (
	whiteLookup = buildLookup(whiteTerminating[:], whiteMakeup[:])
	blackLookup = buildLookup(blackTerminating[:], blackMakeup[:])
)

// runCode is the tuple (u16, u8, u16) of hayro-ccitt src/tables.rs: a run
// length, the length of its code in bits, and the code.
type runCode struct {
	runLength uint16
	length    uint8
	code      uint16
}

// buildLookup is build_lookup in hayro-ccitt src/tables.rs. An entry
// holds the length of the code in its upper 4 bits, and the run length, or
// lookupInvalid, in its lower 12 bits. An invalid entry holds the
// length of the longest valid prefix plus 1, so that the decoder skips it.
func buildLookup(terminating, makeup []runCode) [1 << lookupBits]uint16 {
	codeTables := [][]runCode{terminating, makeup, commonMakeup[:]}
	var table [1 << lookupBits]uint16
	for i := range table {
		table[i] = lookupInvalid
	}

	for _, codes := range codeTables {
		for _, c := range codes {
			shift := lookupBits - uint32(c.length)
			start := int(c.code) << shift
			for i := range 1 << shift {
				table[start+i] = c.runLength | uint16(c.length)<<12
			}
		}
	}

	for b := range table {
		if table[b] != lookupInvalid {
			continue
		}
		var longestPrefix uint32
		for _, codes := range codeTables {
			for _, c := range codes {
				diff := (uint32(b) >> (lookupBits - uint32(c.length))) ^ uint32(c.code)
				prefix := uint32(c.length) - uint32(32-bits.LeadingZeros32(diff))
				longestPrefix = max(longestPrefix, prefix)
			}
		}
		table[b] = lookupInvalid | uint16(longestPrefix+1)<<12
	}

	return table
}

// whiteTerminating is WHITE_TERMINATING in hayro-ccitt src/tables.rs, the
// terminating codes of white runs.
var whiteTerminating = [64]runCode{
	{0, 8, 0b00110101},
	{1, 6, 0b000111},
	{2, 4, 0b0111},
	{3, 4, 0b1000},
	{4, 4, 0b1011},
	{5, 4, 0b1100},
	{6, 4, 0b1110},
	{7, 4, 0b1111},
	{8, 5, 0b10011},
	{9, 5, 0b10100},
	{10, 5, 0b00111},
	{11, 5, 0b01000},
	{12, 6, 0b001000},
	{13, 6, 0b000011},
	{14, 6, 0b110100},
	{15, 6, 0b110101},
	{16, 6, 0b101010},
	{17, 6, 0b101011},
	{18, 7, 0b0100111},
	{19, 7, 0b0001100},
	{20, 7, 0b0001000},
	{21, 7, 0b0010111},
	{22, 7, 0b0000011},
	{23, 7, 0b0000100},
	{24, 7, 0b0101000},
	{25, 7, 0b0101011},
	{26, 7, 0b0010011},
	{27, 7, 0b0100100},
	{28, 7, 0b0011000},
	{29, 8, 0b00000010},
	{30, 8, 0b00000011},
	{31, 8, 0b00011010},
	{32, 8, 0b00011011},
	{33, 8, 0b00010010},
	{34, 8, 0b00010011},
	{35, 8, 0b00010100},
	{36, 8, 0b00010101},
	{37, 8, 0b00010110},
	{38, 8, 0b00010111},
	{39, 8, 0b00101000},
	{40, 8, 0b00101001},
	{41, 8, 0b00101010},
	{42, 8, 0b00101011},
	{43, 8, 0b00101100},
	{44, 8, 0b00101101},
	{45, 8, 0b00000100},
	{46, 8, 0b00000101},
	{47, 8, 0b00001010},
	{48, 8, 0b00001011},
	{49, 8, 0b01010010},
	{50, 8, 0b01010011},
	{51, 8, 0b01010100},
	{52, 8, 0b01010101},
	{53, 8, 0b00100100},
	{54, 8, 0b00100101},
	{55, 8, 0b01011000},
	{56, 8, 0b01011001},
	{57, 8, 0b01011010},
	{58, 8, 0b01011011},
	{59, 8, 0b01001010},
	{60, 8, 0b01001011},
	{61, 8, 0b00110010},
	{62, 8, 0b00110011},
	{63, 8, 0b00110100},
}

// whiteMakeup is WHITE_MAKEUP in hayro-ccitt src/tables.rs, the make-up
// codes of white runs.
var whiteMakeup = [27]runCode{
	{64, 5, 0b11011},
	{128, 5, 0b10010},
	{192, 6, 0b010111},
	{256, 7, 0b0110111},
	{320, 8, 0b00110110},
	{384, 8, 0b00110111},
	{448, 8, 0b01100100},
	{512, 8, 0b01100101},
	{576, 8, 0b01101000},
	{640, 8, 0b01100111},
	{704, 9, 0b011001100},
	{768, 9, 0b011001101},
	{832, 9, 0b011010010},
	{896, 9, 0b011010011},
	{960, 9, 0b011010100},
	{1024, 9, 0b011010101},
	{1088, 9, 0b011010110},
	{1152, 9, 0b011010111},
	{1216, 9, 0b011011000},
	{1280, 9, 0b011011001},
	{1344, 9, 0b011011010},
	{1408, 9, 0b011011011},
	{1472, 9, 0b010011000},
	{1536, 9, 0b010011001},
	{1600, 9, 0b010011010},
	{1664, 6, 0b011000},
	{1728, 9, 0b010011011},
}

// blackTerminating is BLACK_TERMINATING in hayro-ccitt src/tables.rs, the
// terminating codes of black runs.
var blackTerminating = [64]runCode{
	{0, 10, 0b0000110111},
	{1, 3, 0b010},
	{2, 2, 0b11},
	{3, 2, 0b10},
	{4, 3, 0b011},
	{5, 4, 0b0011},
	{6, 4, 0b0010},
	{7, 5, 0b00011},
	{8, 6, 0b000101},
	{9, 6, 0b000100},
	{10, 7, 0b0000100},
	{11, 7, 0b0000101},
	{12, 7, 0b0000111},
	{13, 8, 0b00000100},
	{14, 8, 0b00000111},
	{15, 9, 0b000011000},
	{16, 10, 0b0000010111},
	{17, 10, 0b0000011000},
	{18, 10, 0b0000001000},
	{19, 11, 0b00001100111},
	{20, 11, 0b00001101000},
	{21, 11, 0b00001101100},
	{22, 11, 0b00000110111},
	{23, 11, 0b00000101000},
	{24, 11, 0b00000010111},
	{25, 11, 0b00000011000},
	{26, 12, 0b000011001010},
	{27, 12, 0b000011001011},
	{28, 12, 0b000011001100},
	{29, 12, 0b000011001101},
	{30, 12, 0b000001101000},
	{31, 12, 0b000001101001},
	{32, 12, 0b000001101010},
	{33, 12, 0b000001101011},
	{34, 12, 0b000011010010},
	{35, 12, 0b000011010011},
	{36, 12, 0b000011010100},
	{37, 12, 0b000011010101},
	{38, 12, 0b000011010110},
	{39, 12, 0b000011010111},
	{40, 12, 0b000001101100},
	{41, 12, 0b000001101101},
	{42, 12, 0b000011011010},
	{43, 12, 0b000011011011},
	{44, 12, 0b000001010100},
	{45, 12, 0b000001010101},
	{46, 12, 0b000001010110},
	{47, 12, 0b000001010111},
	{48, 12, 0b000001100100},
	{49, 12, 0b000001100101},
	{50, 12, 0b000001010010},
	{51, 12, 0b000001010011},
	{52, 12, 0b000000100100},
	{53, 12, 0b000000110111},
	{54, 12, 0b000000111000},
	{55, 12, 0b000000100111},
	{56, 12, 0b000000101000},
	{57, 12, 0b000001011000},
	{58, 12, 0b000001011001},
	{59, 12, 0b000000101011},
	{60, 12, 0b000000101100},
	{61, 12, 0b000001011010},
	{62, 12, 0b000001100110},
	{63, 12, 0b000001100111},
}

// blackMakeup is BLACK_MAKEUP in hayro-ccitt src/tables.rs, the make-up
// codes of black runs.
var blackMakeup = [27]runCode{
	{64, 10, 0b0000001111},
	{128, 12, 0b000011001000},
	{192, 12, 0b000011001001},
	{256, 12, 0b000001011011},
	{320, 12, 0b000000110011},
	{384, 12, 0b000000110100},
	{448, 12, 0b000000110101},
	{512, 13, 0b0000001101100},
	{576, 13, 0b0000001101101},
	{640, 13, 0b0000001001010},
	{704, 13, 0b0000001001011},
	{768, 13, 0b0000001001100},
	{832, 13, 0b0000001001101},
	{896, 13, 0b0000001110010},
	{960, 13, 0b0000001110011},
	{1024, 13, 0b0000001110100},
	{1088, 13, 0b0000001110101},
	{1152, 13, 0b0000001110110},
	{1216, 13, 0b0000001110111},
	{1280, 13, 0b0000001010010},
	{1344, 13, 0b0000001010011},
	{1408, 13, 0b0000001010100},
	{1472, 13, 0b0000001010101},
	{1536, 13, 0b0000001011010},
	{1600, 13, 0b0000001011011},
	{1664, 13, 0b0000001100100},
	{1728, 13, 0b0000001100101},
}

// commonMakeup is COMMON_MAKEUP in hayro-ccitt src/tables.rs, the make-up
// codes of both colors for long runs.
var commonMakeup = [13]runCode{
	{1792, 11, 0b00000001000},
	{1856, 11, 0b00000001100},
	{1920, 11, 0b00000001101},
	{1984, 12, 0b000000010010},
	{2048, 12, 0b000000010011},
	{2112, 12, 0b000000010100},
	{2176, 12, 0b000000010101},
	{2240, 12, 0b000000010110},
	{2304, 12, 0b000000010111},
	{2368, 12, 0b000000011100},
	{2432, 12, 0b000000011101},
	{2496, 12, 0b000000011110},
	{2560, 12, 0b000000011111},
}
