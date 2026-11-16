// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/object/stream.rs.

package syntax

import (
	"bytes"
	"testing"
)

// streamImpl reads an indirect stream from data.
func streamImpl(t *testing.T, data string) Stream {
	t.Helper()
	s, ok := NewReader([]byte(data)).ReadIndirect[Stream](DummyReaderContext())
	if !ok {
		t.Fatalf("%q: no stream", data)
	}
	return s
}

func TestStreamDisplay(t *testing.T) {
	if got := streamImpl(t, "<< /Length 3 >> stream\nabc\nendstream").String(); got != "Stream (len: 3)" {
		t.Errorf("got %q", got)
	}
}

func TestStream(t *testing.T) {
	if s := streamImpl(t, "<< /Length 10 >> stream\nabcdefghij\nendstream"); string(s.data) != "abcdefghij" {
		t.Errorf("got %q", s.data)
	}
}

func TestStreamFallback(t *testing.T) {
	if s := streamImpl(t, "<< /Length 999 >> stream\nabcdefghij\nendstream"); string(s.data) != "abcdefghij" {
		t.Errorf("got %q", s.data)
	}
}

func TestStreamWithExternalFile(t *testing.T) {
	s := streamImpl(t, "<< /Length 3 /F (data.bin) >> stream\nabc\nendstream")
	if string(s.data) != "abc" || !s.Dict().ContainsKey(KeyF) {
		t.Fatal("stream")
	}
	if _, err := s.Decoded(); err == nil {
		t.Fatal("decoded a stream with an external file")
	}
}

func TestInlineFilterAbbreviationStillDecodes(t *testing.T) {
	dict, ok := NewReader([]byte("<< /F /AHx >>")).ReadWithContext[Dict](DummyReaderContext())
	if !ok {
		t.Fatal("no dict")
	}
	got, err := newStream([]byte("41>"), dict).Decoded()
	if err != nil || !bytes.Equal(got, []byte("A")) {
		t.Fatalf("got %q, %v", got, err)
	}
}
