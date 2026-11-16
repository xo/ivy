package pdffont

import (
	"bytes"
	"encoding/binary"
	"testing"

	ot "github.com/go-text/typesetting/font/opentype"
	"golang.org/x/image/font/gofont/goregular"
)

// This file ports no upstream file. It tests the change of sfntTables from
// skrifa.

func TestSfntTablesCut(t *testing.T) {
	data := bytes.Clone(goregular.TTF)
	numTables := int(binary.BigEndian.Uint16(data[4:]))
	// the record of the table that ends last, and that of the first table
	last, lastEnd := 0, uint32(0)
	for i := range numTables {
		rec := 12 + 16*i
		end := binary.BigEndian.Uint32(data[rec+8:]) + binary.BigEndian.Uint32(data[rec+12:])
		if end >= lastEnd {
			last, lastEnd = rec, end
		}
	}
	lastTag := ot.Tag(binary.BigEndian.Uint32(data[last:]))
	lastStart := binary.BigEndian.Uint32(data[last+8:])
	binary.BigEndian.PutUint32(data[last+12:], 0xffffffff)
	first := 12
	if first == last {
		first += 16
	}
	firstTag := ot.Tag(binary.BigEndian.Uint32(data[first:]))
	binary.BigEndian.PutUint32(data[first+8:], uint32(len(data)+1))
	tabs, err := sfntTables(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, exp := len(tabs[lastTag]), len(data)-int(lastStart); got != exp {
		t.Errorf("expected the table %v cut to %d bytes, got: %d", lastTag, exp, got)
	}
	if _, ok := tabs[firstTag]; ok {
		t.Errorf("expected the table %v, which starts past the end, skipped", firstTag)
	}
	// the font still reads with the cut table
	if _, err := newSfntProgram(bytes.Clone(goregular.TTF)[:len(goregular.TTF)-2], 0); err != nil {
		t.Errorf("expected a font with a short last table to read, got: %v", err)
	}
}
