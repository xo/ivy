package pdffont

import (
	"bytes"
	"encoding/binary"
	"testing"

	ot "github.com/xo/okage/typesetting/font/opentype"
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

// TestSfntTablesUnpadded makes sure that a font whose tables have no
// padding reads, and that the reader does not change its data. ot.WriteTTF
// appends the padding of a checksum to each table, and it wrote over the
// version of the table maxp that followed the table name in the Arabic
// fonts of the file arabic-cid-truetype of pdf.js.
func TestSfntTablesUnpadded(t *testing.T) {
	ld, err := ot.NewLoader(bytes.NewReader(goregular.TTF))
	if err != nil {
		t.Fatal(err)
	}
	var list []ot.Table
	odd := false
	for _, tag := range ld.Tables() {
		raw, err := ld.RawTable(tag)
		if err != nil {
			t.Fatal(err)
		}
		if tag == tagCmap {
			continue
		}
		odd = odd || len(raw)%4 != 0
		list = append(list, ot.Table{Tag: tag, Content: raw})
	}
	if !odd {
		t.Fatal("expected a table with a length that is not a multiple of 4")
	}
	// ot.WriteTTF does not pad the tables. The font has no cmap table, so
	// the reader builds the font twice.
	data := ot.WriteTTF(list)
	orig := bytes.Clone(data)
	p, err := newFontProgram(data, 0)
	if err != nil {
		t.Fatalf("expected the font to read, got: %v", err)
	}
	if !bytes.Equal(data, orig) {
		t.Error("the reader changed the data of the font")
	}
	if n := len(p.outline(36).Elements()); n == 0 {
		t.Error("expected the outline of glyph 36")
	}
}
