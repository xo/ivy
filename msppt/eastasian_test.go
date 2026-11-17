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
