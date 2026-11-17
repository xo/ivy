package msppt

import (
	"encoding/binary"
	"testing"
)

// TestReadCFEastAsianFont checks that a TextCFException gives the font of
// the East Asian text, from its oldEATypeface.
func TestReadCFEastAsianFont(t *testing.T) {
	b := binary.LittleEndian.AppendUint32(nil, cfTypeface|cfOldEATypeface)
	b = binary.LittleEndian.AppendUint16(b, 0)
	b = binary.LittleEndian.AppendUint16(b, 1)
	c, n, ok := readCF(b)
	if !ok || n != len(b) {
		t.Fatalf("expected %d bytes, got %d %v", len(b), n, ok)
	}
	if !c.font.ok || c.font.v != 0 || !c.eaFont.ok || c.eaFont.v != 1 {
		t.Errorf("expected the fonts 0 and 1, got %+v and %+v", c.font, c.eaFont)
	}
}

// TestSpecialInfo checks the languages of the runs of a
// TextSpecialInfoAtom: the lang of a run when it is East Asian, or else its
// altLang, and the spell and smart tags before and after them.
func TestSpecialInfo(t *testing.T) {
	le := binary.LittleEndian
	var b []byte
	// 3 characters in English with the altLang Japanese, after the spell
	b = le.AppendUint32(b, 3)
	b = le.AppendUint32(b, siSpell|siLang|siAltLang)
	b = le.AppendUint16(b, 0)
	b = le.AppendUint16(b, 0x0409)
	b = le.AppendUint16(b, 0x0411)
	// 2 characters in Traditional Chinese, with a smart tag
	b = le.AppendUint32(b, 2)
	b = le.AppendUint32(b, siLang|siSmartTag)
	b = le.AppendUint16(b, 0x0404)
	b = le.AppendUint32(b, 1)
	b = le.AppendUint32(b, 7)
	// 1 character in English
	b = le.AppendUint32(b, 1)
	b = le.AppendUint32(b, siLang)
	b = le.AppendUint16(b, 0x0409)
	tx := &text{units: make([]uint16, 5)}
	tx.si = readSIRuns(b, 5)
	for _, tt := range []struct {
		i    int
		lang string
		end  int
	}{{0, "ja", 3}, {3, "zh-Hant", 5}, {5, "", 6}} {
		if lang, end := tx.langAt(tt.i, "ko"); lang != tt.lang || end != tt.end {
			t.Errorf("character %d: expected %q to %d, got: %q to %d", tt.i, tt.lang, tt.end, lang, end)
		}
	}
	// a character without a run takes the default language
	if lang, _ := tx.langAt(9, "ko"); lang != "ko" {
		t.Errorf("expected the default language ko, got: %q", lang)
	}
	if _, _, ok := readSI(le.AppendUint32(nil, siLang)); ok {
		t.Error("expected a cut exception to fail")
	}
}
