// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/trivia.rs.

package syntax

// regularCharacterTable is REGULAR_CHARACTER_TABLE in hayro-syntax
// src/trivia.rs. A byte is regular when it is neither white space nor a
// delimiter.
var regularCharacterTable = buildRegularCharacterTable()

// buildRegularCharacterTable is build_regular_character_table in hayro-syntax
// src/trivia.rs.
func buildRegularCharacterTable() [256]bool {
	var table [256]bool
	for i := range table {
		table[i] = true
	}
	for _, b := range []byte{0x00, 0x09, 0x0a, 0x0c, 0x0d, 0x20, '(', ')', '<', '>', '[', ']', '{', '}', '/', '%'} {
		table[b] = false
	}
	return table
}

// whiteSpaceCharacterTable is WHITE_SPACE_CHARACTER_TABLE in hayro-syntax
// src/trivia.rs.
var whiteSpaceCharacterTable = buildWhiteSpaceTable()

// buildWhiteSpaceTable is build_white_space_table in hayro-syntax
// src/trivia.rs.
func buildWhiteSpaceTable() [256]bool {
	var table [256]bool
	for _, b := range []byte{0x00, 0x09, 0x0a, 0x0c, 0x0d, 0x20} {
		table[b] = true
	}
	return table
}

// isWhiteSpaceCharacter is is_white_space_character in hayro-syntax
// src/trivia.rs.
func isWhiteSpaceCharacter(c byte) bool {
	return whiteSpaceCharacterTable[c]
}

// isRegularCharacter is is_regular_character in hayro-syntax src/trivia.rs.
func isRegularCharacter(c byte) bool {
	return regularCharacterTable[c]
}

// isEOLCharacter is is_eol_character in hayro-syntax src/trivia.rs.
func isEOLCharacter(c byte) bool {
	return c == 0x0a || c == 0x0d
}

// Comment is Comment in hayro-syntax src/trivia.rs. It holds the text of a
// comment, without the leading percent sign.
type Comment []byte

// skipComment is the Skippable implementation of Comment in hayro-syntax
// src/trivia.rs.
func skipComment(r *Reader) bool {
	if !r.ForwardTag("%") {
		return false
	}
	r.ForwardWhile(func(b byte) bool { return !isEOLCharacter(b) })
	return true
}

// readComment is the Readable implementation of Comment in hayro-syntax
// src/trivia.rs.
func readComment(r *Reader) (Comment, bool) {
	start := r.offset
	if !skipComment(r) {
		r.offset = start
		return nil, false
	}
	return Comment(r.Data[start+1 : r.offset]), true
}
