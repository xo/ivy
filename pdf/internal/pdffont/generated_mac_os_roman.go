// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/generated/mac_os_roman.rs. A script made the
// tables from the Rust file. README.md holds the script.
//
// The table follows
// <https://github.com/apache/pdfbox/blob/4438b8fdc67a3a9ebfb194595d0e81f88b708a37/pdfbox/src/main/java/org/apache/pdfbox/pdmodel/font/encoding/MacOSRomanEncoding.java.java>.

package pdffont

// macOSRomanGet is get in hayro-font src/generated/mac_os_roman.rs. It
// returns the glyph name of a code that Mac OS Roman adds to MacRomanEncoding.
func macOSRomanGet(code uint8) (string, bool) {
	s := macOSRomanTable[code]
	return s, s != ""
}

// macOSRomanGetInverse is get_inverse in hayro-font
// src/generated/mac_os_roman.rs. It returns the code of a glyph name.
func macOSRomanGetInverse(name string) (uint8, bool) {
	c, ok := macOSRomanInverseTable[name]
	return c, ok
}

// macOSRomanTable is MAC_OS_ROMAN in hayro-font src/generated/mac_os_roman.rs.
var macOSRomanTable = [256]string{
	173: "notequal",
	176: "infinity",
	178: "lessequal",
	179: "greaterequal",
	182: "partialdiff",
	183: "summation",
	184: "product",
	185: "pi",
	186: "integral",
	189: "Omega",
	195: "radical",
	197: "approxequal",
	198: "Delta",
	215: "lozenge",
	219: "Euro",
	240: "apple",
}

// macOSRomanInverseTable is MAC_OS_ROMAN_INVERSE in hayro-font src/generated/mac_os_roman.rs.
var macOSRomanInverseTable = map[string]uint8{
	"notequal":     173,
	"infinity":     176,
	"lessequal":    178,
	"greaterequal": 179,
	"partialdiff":  182,
	"summation":    183,
	"product":      184,
	"pi":           185,
	"integral":     186,
	"Omega":        189,
	"radical":      195,
	"approxequal":  197,
	"Delta":        198,
	"lozenge":      215,
	"Euro":         219,
	"apple":        240,
}
