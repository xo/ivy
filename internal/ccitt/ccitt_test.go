// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file is not a port. It holds the fuzz test FuzzDecode and the tests
// of the errors.

package ccitt

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
)

// fuzzSink is a Decoder that checks the rows of the decoder. It records the
// first fault.
type fuzzSink struct {
	columns uint32
	rows    int
	fault   string
}

// PushRow makes sure that the changes ascend and that each change is inside
// the row.
func (s *fuzzSink) PushRow(changes []uint32) {
	for i, ch := range changes {
		if (i > 0 && changes[i-1] >= ch) || ch >= s.columns {
			if s.fault == "" {
				s.fault = "the changes of a row do not ascend or are past the row"
			}
			break
		}
	}
	s.rows++
}

// fuzzSettings returns the settings that the fuzz test takes from flags.
// The low 2 bits pick the encoding, and the other bits turn on the flags.
func fuzzSettings(columns, rows uint16, flags uint8) DecodeSettings {
	return DecodeSettings{
		Columns:            uint32(columns),
		Rows:               uint32(rows),
		Encoding:           EncodingMode(flags % 3),
		EndOfBlock:         flags&4 != 0,
		EndOfLine:          flags&8 != 0,
		RowsAreByteAligned: flags&16 != 0,
		Strict:             flags&32 != 0,
	}
}

// FuzzDecode decodes arbitrary data with arbitrary settings. The decoder
// must not panic, it must give valid rows, it must not read past the data,
// and each row must take at least one bit of the data.
func FuzzDecode(f *testing.F) {
	columns := uint16(8)
	if r, err := os.Open(filepath.Join("testdata", "references", "base.png")); err == nil {
		if cfg, err := png.DecodeConfig(r); err == nil {
			columns = uint16(cfg.Width)
		}
		_ = r.Close()
	}
	paths, _ := filepath.Glob(filepath.Join("testdata", "streams", "*.bin"))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, flags := range []uint8{0, 4, 1 | 8, 1 | 8 | 4, 2 | 8, 2 | 8 | 4} {
			f.Add(data, columns, uint16(0), flags)
		}
	}
	f.Add([]byte{0x35, 0x14}, uint16(8), uint16(1), uint8(1))

	f.Fuzz(func(t *testing.T, data []byte, columns, rows uint16, flags uint8) {
		settings := fuzzSettings(columns, rows, flags)
		sink := &fuzzSink{columns: settings.Columns}
		n, err := Decode(data, sink, &settings)
		if sink.fault != "" {
			t.Fatal(sink.fault)
		}
		if err != nil && !errors.Is(err, ivy.ErrFormat) {
			t.Fatalf("Decode() error %v does not unwrap to ivy.ErrFormat", err)
		}
		if n < 0 || n > len(data) {
			t.Fatalf("Decode() read %d bytes of %d", n, len(data))
		}
		if rows != 0 && sink.rows > int(rows) {
			t.Fatalf("Decode() gave %d rows, want at most %d", sink.rows, rows)
		}
		if sink.rows > 8*len(data)+1 {
			t.Fatalf("Decode() gave %d rows for %d bytes", sink.rows, len(data))
		}
	})
}

// TestErrors makes sure that each error has a message of its own, and that
// it unwraps to the sentinel error of ivy or of context.
func TestErrors(t *testing.T) {
	seen := map[string]bool{}
	for _, err := range []DecodeError{ErrUnexpectedEOF, ErrInvalidCode, ErrOverflow, ErrCancelled} {
		msg := err.Error()
		if seen[msg] {
			t.Errorf("the message %q repeats", msg)
		}
		seen[msg] = true
		var want error = ivy.ErrFormat
		if err == ErrCancelled {
			want = context.Canceled
		}
		if !errors.Is(err, want) {
			t.Errorf("%v does not unwrap to %v", err, want)
		}
	}
}

// TestZeroColumns makes sure that an image without columns gives no rows,
// as upstream does.
func TestZeroColumns(t *testing.T) {
	sink := &fuzzSink{}
	n, err := Decode([]byte{0xFF}, sink, &DecodeSettings{})
	if n != 0 || err != nil || sink.rows != 0 {
		t.Fatalf("Decode() = %d, %v with %d rows, want 0, nil with 0 rows", n, err, sink.rows)
	}
}
