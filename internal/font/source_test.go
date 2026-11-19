package font

import (
	"bytes"
	"encoding/binary"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

// withFSType returns a copy of the font data whose field fsType of the
// table OS/2 is v.
func withFSType(t *testing.T, data []byte, v uint16) []byte {
	t.Helper()
	data = bytes.Clone(data)
	os2, ok := Source{Data: data}.table("OS/2")
	if !ok || len(os2) < 10 {
		t.Fatal("expected the table OS/2")
	}
	binary.BigEndian.PutUint16(os2[8:], v)
	return data
}

func TestSourceEmbeddable(t *testing.T) {
	for _, tt := range []struct {
		fsType uint16
		want   bool
	}{
		{0x0000, true},
		{0x0002, false},
		{0x0004, true},
		{0x0008, true},
		// the least restrictive bit wins, so bit 1 with bit 3 is editable
		{0x000A, true},
		{0x0100, false},
		{0x0200, false},
		{0x0008 | 0x0100, false},
	} {
		src := Source{Data: withFSType(t, goregular.TTF, tt.fsType)}
		if got := src.Embeddable(); got != tt.want {
			t.Errorf("fsType %#04x: expected %t, got %t", tt.fsType, tt.want, got)
		}
	}
	if !(Source{Data: []byte("not a font")}).Embeddable() {
		t.Error("expected a font without the table OS/2 to be embeddable")
	}
}

func TestSourceKey(t *testing.T) {
	f, err := Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	src, ok := f.Source()
	if !ok || len(src.Data) != len(goregular.TTF) || src.Index != 0 {
		t.Fatalf("expected the source of the font, got: %d bytes, %d, %t", len(src.Data), src.Index, ok)
	}
	// Go Regular has no table fvar, so the variations are not in the key
	bold := src
	bold.Variations = []Variation{{Tag: [4]byte{'w', 'g', 'h', 't'}, Value: 700}}
	if src.Key() != bold.Key() || bold.Key().HasVariations() {
		t.Error("expected the variations of a font without axes to be out of the key")
	}
	other := Source{Data: bytes.Clone(goregular.TTF)}
	if src.Key() == other.Key() {
		t.Error("expected two copies of the bytes to have two keys")
	}
	if _, ok := FromFace(f.face).Source(); ok {
		t.Error("expected a face of FromFace to have no source")
	}
	if got, ok := FromFaceSource(f.face, src).Source(); !ok || got.Key() != src.Key() {
		t.Error("expected the source of FromFaceSource")
	}
}

func TestSourceTable(t *testing.T) {
	for _, tt := range []struct {
		name  string
		data  []byte
		index int
	}{
		{"short", []byte("ttcf"), 0},
		{"collection without fonts", append([]byte("ttcf\x00\x01\x00\x00"), 0, 0, 0, 0), 0},
		{"index of a single font", goregular.TTF, 1},
		{"negative index", goregular.TTF, -1},
		{"directory past the end", []byte("\x00\x01\x00\x00\x00\x09\x00\x00\x00\x00\x00\x00"), 0},
	} {
		if _, ok := (Source{Data: tt.data, Index: tt.index}).table("OS/2"); ok {
			t.Errorf("%s: expected no table", tt.name)
		}
	}
}
