// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/object/string.rs.

package syntax

import (
	"bytes"
	"testing"
)

// readStringBytes reads a string object from data.
func readStringBytes(data string) ([]byte, bool) {
	s, ok := NewReader([]byte(data)).ReadWithoutContext[String]()
	return s, ok
}

func TestStringDisplay(t *testing.T) {
	normal, _ := readStringBytes("(Hello)")
	binary, _ := readStringBytes("<00FF>")
	if got := String(normal).String(); got != "\"Hello\"" {
		t.Errorf("got %q", got)
	}
	if got := String(binary).String(); got != "[0, 255]" {
		t.Errorf("got %q", got)
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		data string
		want []byte
	}{
		{"<>", []byte{}},
		{"<00010203>", []byte{0x00, 0x01, 0x02, 0x03}},
		{"<000102034>", []byte{0x00, 0x01, 0x02, 0x03, 0x40}},
		{"<000102034>dfgfg4", []byte{0x00, 0x01, 0x02, 0x03, 0x40}},
		{"<1  3 4>dfgfg4", []byte{0x13, 0x40}},
		{"<1>dfgfg4", []byte{0x10}},
		{"()", []byte{}},
		{"(Hi there.)", []byte("Hi there.")},
		{"(Hi ) there.)", []byte("Hi ")},
		{"(Hi (()) there)", []byte("Hi (()) there")},
		{"(Hi \\()", []byte("Hi (")},
		{"(Hi \\\nthere)", []byte("Hi there")},
		{"(Hi \\05354)", []byte("Hi +54")},
		{"(\\3)", []byte("\x03")},
		{"(\\36)", []byte("\x1e")},
		{"(\\36ab)", []byte("\x1eab")},
		{"(\\00Y)", []byte("\x00Y")},
		{"(\\0Y)", []byte("\x00Y")},
		{"(a\n\nb)", []byte("a\n\nb")},
		{"(a\r\n\r\nb)", []byte("a\n\nb")},
		{"(a\r\rb)", []byte("a\n\nb")},
		{"(a\\\n\nb)", []byte("a\nb")},
		{"(a\\\r\n\r\nb)", []byte("a\nb")},
		{"(Hi there.)abcde", []byte("Hi there.")},
		{"(Hi \\778)", []byte("Hi \x3F8")},
	}
	for _, test := range tests {
		got, ok := readStringBytes(test.data)
		if !ok || !bytes.Equal(got, test.want) {
			t.Errorf("%q: got %q, %v; want %q", test.data, got, ok, test.want)
		}
	}
	if _, ok := readStringBytes("(Hi \\777)"); !ok {
		t.Error("an octal escape past 255 failed")
	}
	for _, input := range []string{"<", "34AD"} {
		if _, ok := readStringBytes(input); ok {
			t.Errorf("%q: accepted", input)
		}
	}
}
