package font

import (
	"encoding/binary"
	"math"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

// table returns the table tag of the TrueType font data, or nil.
func table(data []byte, tag string) []byte {
	n := int(binary.BigEndian.Uint16(data[4:]))
	for i := range n {
		rec := data[12+16*i:]
		if string(rec[:4]) == tag {
			off, length := binary.BigEndian.Uint32(rec[8:]), binary.BigEndian.Uint32(rec[12:])
			return data[off : off+length]
		}
	}
	return nil
}

// near reports whether a and b differ by less than 0.001.
func near(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.001
}

func TestVertical(t *testing.T) {
	f, err := Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	h, _ := f.Glyph('H')
	i, _ := f.Glyph('I')
	// Go Regular has no vmtx table, and advances 1 em down the page
	if adv := f.VerticalAdvance(h); !near(adv, 1) {
		t.Errorf("expected a vertical advance of 1 em, got: %v", adv)
	}
	if x, y, ok := f.VerticalOrigin(h); !ok || !near(x, f.Advance(h)/2) || y <= 0 || y > 1.5 {
		t.Errorf("expected the origin above the baseline at half of the advance, got: %v %v %v", x, y, ok)
	}
	// a font with vhea, vmtx and VORG: an advance of 1.5 em for each glyph,
	// and an origin of 0.9 em, or 0.5 em for I
	numGlyphs := int(binary.BigEndian.Uint16(table(goregular.TTF, "maxp")[4:]))
	vhea := make([]byte, 36)
	binary.BigEndian.PutUint32(vhea, 0x00011000)
	binary.BigEndian.PutUint16(vhea[34:], 1)
	vmtx := make([]byte, 4+2*(numGlyphs-1))
	binary.BigEndian.PutUint16(vmtx, 3072)
	vorg := make([]byte, 12)
	binary.BigEndian.PutUint16(vorg, 1)
	binary.BigEndian.PutUint16(vorg[4:], 1843)
	binary.BigEndian.PutUint16(vorg[6:], 1)
	binary.BigEndian.PutUint16(vorg[8:], uint16(i))
	binary.BigEndian.PutUint16(vorg[10:], 1024)
	data := withTables(goregular.TTF, map[string][]byte{"vhea": vhea, "vmtx": vmtx, "VORG": vorg})
	if f, err = Load(data, 0); err != nil {
		t.Fatal(err)
	}
	if adv := f.VerticalAdvance(h); !near(adv, 1.5) {
		t.Errorf("expected a vertical advance of 1.5 em, got: %v", adv)
	}
	if _, y, ok := f.VerticalOrigin(h); !ok || !near(y, 0.9) {
		t.Errorf("expected the default origin of 0.9 em, got: %v %v", y, ok)
	}
	if _, y, ok := f.VerticalOrigin(i); !ok || !near(y, 0.5) {
		t.Errorf("expected the origin of I at 0.5 em, got: %v %v", y, ok)
	}
}
