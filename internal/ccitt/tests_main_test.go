// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt tests/main.rs. The tests decode the CCITT
// streams in testdata/streams and compare them with the images in
// testdata/references.

package ccitt

import (
	"bytes"
	"fmt"
	imagecolor "image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// loadReference is load_reference in hayro-ccitt tests/main.rs. A value
// of true is white.
func loadReference(t *testing.T, name string) [][]bool {
	t.Helper()
	path := filepath.Join("testdata", "references", name+".png")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load %s: %v", path, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to load %s: %v", path, err)
	}

	b := img.Bounds()
	rows := make([][]bool, b.Dy())
	for y := range rows {
		rows[y] = make([]bool, b.Dx())
		for x := range rows[y] {
			g := imagecolor.GrayModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(imagecolor.Gray)
			rows[y][x] = g.Y > 127
		}
	}
	return rows
}

// collector is Collector in hayro-ccitt tests/main.rs. It keeps the
// rows of the decoder, and the first fault of the changes.
type collector struct {
	columns uint32
	rows    [][]bool
	fault   error
}

// PushRow is Decoder::push_row for Collector.
func (c *collector) PushRow(changes []uint32) {
	for i, ch := range changes {
		if (i > 0 && changes[i-1] >= ch) || ch >= c.columns {
			if c.fault == nil {
				c.fault = fmt.Errorf("row %d has the changes %v", len(c.rows), changes)
			}
			break
		}
	}

	row := make([]bool, c.columns)
	for x := range row {
		n := sort.Search(len(changes), func(i int) bool { return changes[i] > uint32(x) })
		row[x] = n%2 == 0
	}
	c.rows = append(c.rows, row)
}

// check is check in hayro-ccitt tests/main.rs.
func check(t *testing.T, stream, reference string, configure func(s *DecodeSettings)) {
	t.Helper()
	checkConsumed(t, stream, reference, -1, configure)
}

// checkConsumed is check_consumed in hayro-ccitt tests/main.rs. A
// negative consumed is the None of upstream, which expects all of the data.
func checkConsumed(t *testing.T, stream, reference string, consumed int, configure func(s *DecodeSettings)) {
	t.Helper()
	data := readStream(t, stream)
	ref := loadReference(t, reference)

	settings := DecodeSettings{
		Columns: uint32(len(ref[0])),
		Rows:    uint32(len(ref)),
	}
	configure(&settings)

	col := &collector{columns: settings.Columns}
	read, err := Decode(data, col, &settings)
	if err != nil {
		t.Fatalf("%s: decoding failed: %v", stream, err)
	}
	if col.fault != nil {
		t.Fatalf("%s: %v", stream, col.fault)
	}

	expectedRows := len(ref)
	if settings.Rows != 0 {
		expectedRows = min(len(ref), int(settings.Rows))
	}
	if len(col.rows) != expectedRows {
		t.Fatalf("%s: wrong number of rows: %d, want %d", stream, len(col.rows), expectedRows)
	}

	for y, actual := range col.rows {
		if len(actual) != len(ref[y]) {
			t.Fatalf("%s: row %d has the wrong length", stream, y)
		}
		for x := range actual {
			if actual[x] != ref[y][x] {
				t.Fatalf("%s: first mismatch at row %d, column %d", stream, y, x)
			}
		}
	}

	if consumed < 0 {
		consumed = len(data)
	}
	if read != consumed {
		t.Fatalf("%s: wrong number of bytes consumed: %d, want %d", stream, read, consumed)
	}
}

// checkMalformed is check_malformed in hayro-ccitt tests/main.rs. A
// malformed stream must decode or fail without a panic.
func checkMalformed(t *testing.T, stream string, columns, rows uint32, configure func(s *DecodeSettings)) {
	t.Helper()
	data := readStream(t, stream)

	settings := DecodeSettings{
		Columns:    columns,
		Rows:       rows,
		EndOfBlock: true,
		Encoding:   Group3_1D,
	}
	configure(&settings)

	_, _ = Decode(data, &collector{columns: columns}, &settings)
}

// readStream reads a stream of testdata/streams.
func readStream(t *testing.T, stream string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "streams", stream+".bin")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return data
}
