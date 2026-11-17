// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-font src/lib.rs.

package pdffont

import "testing"

// mapGlyphName is map in the tests of hayro-font src/lib.rs.
func mapGlyphName(name string) (string, bool) {
	bf, ok := glyphNameToUnicode(name, false)
	if !ok {
		return "", false
	}
	if bf.IsString {
		return bf.String, true
	}
	return string(bf.Char), true
}

// checkGlyphNames compares each name with the text that it maps to. An
// empty want means None.
func checkGlyphNames(t *testing.T, cases []struct{ name, want string }) {
	t.Helper()
	for _, c := range cases {
		got, ok := mapGlyphName(c.name)
		if c.want == "" {
			if ok {
				t.Errorf("map(%q) = %q, want None", c.name, got)
			}
			continue
		}
		if !ok || got != c.want {
			t.Errorf("map(%q) = %q, %v, want %q", c.name, got, ok, c.want)
		}
	}
}

// TestAGLSpecificationExamples is agl_specification_examples in the tests
// of hayro-font src/lib.rs.
func TestAGLSpecificationExamples(t *testing.T) {
	t.Parallel()
	checkGlyphNames(t, []struct{ name, want string }{
		{"Lcommaaccent", "Ļ"},
		{"uni20AC0308", "€̈"},
		{"u1040C", "\U0001040C"},
		{"uniD801DC0C", ""},
		{"uni20ac", ""},
		{"Lcommaaccent_uni20AC0308_u1040C.alternate", "Ļ€̈\U0001040C"},
		{"foo", ""},
		{".notdef", ""},
	})
}

// TestComponents is components in the tests of hayro-font src/lib.rs.
func TestComponents(t *testing.T) {
	t.Parallel()
	checkGlyphNames(t, []struct{ name, want string }{
		{"dalethatafpatah", "דֲ"},
		{"hyphen.case", "-"},
		{"f_f_l", "ffl"},
		{"f_g102", "f"},
		{"uni000A0020", "\n "},
		{"u41", ""},
		{"u110000", ""},
	})
}
