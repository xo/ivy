// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/iff/iff_test.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package iff

import (
	"encoding/binary"
	"strings"
	"testing"
)

func chunk(id string, data []byte) []byte {
	b := []byte(id)
	b = binary.BigEndian.AppendUint32(b, uint32(len(data)))
	b = append(b, data...)
	if len(data)%2 == 1 {
		b = append(b, 0)
	}
	return b
}

func form(typ string, kids ...[]byte) []byte {
	d := []byte(typ)
	for _, k := range kids {
		d = append(d, k...)
	}
	return chunk("FORM", d)
}

func TestParse(t *testing.T) {
	body := form("DJVU",
		chunk("INFO", []byte("0123456789")),
		chunk("ODD1", []byte("abc")), // padded
		form("DJVI", chunk("ANTa", []byte("x"))),
		chunk("LAST", []byte("q")), // odd, last: pad may be absent
	)
	body = body[:len(body)-1] // drop final pad
	binary.BigEndian.PutUint32(body[4:], uint32(len(body)-8))

	for _, magic := range []string{"AT&T", ""} {
		c, err := Parse(append([]byte(magic), body...))
		if err != nil {
			t.Fatal(err)
		}
		if c.Name() != "FORM:DJVU" || len(c.Children) != 4 {
			t.Fatalf("got %s with %d children", c.Name(), len(c.Children))
		}
		if got := c.Find("ODD1"); got == nil || string(got.Data) != "abc" {
			t.Fatalf("ODD1 = %v", got)
		}
		in := c.Find("FORM:DJVI")
		if in == nil || string(in.Find("ANTa").Data) != "x" {
			t.Fatal("nested form not parsed")
		}
		if got := string(c.Find("LAST").Data); got != "q" {
			t.Fatalf("LAST = %q", got)
		}
		if c.Offset != len(magic) || c.Children[0].Offset != len(magic)+12 {
			t.Fatalf("offsets %d %d", c.Offset, c.Children[0].Offset)
		}
		if c.Size() != len(body)-8 {
			t.Fatalf("size %d", c.Size())
		}
	}
}

func TestParseErrors(t *testing.T) {
	good := form("DJVU", chunk("INFO", []byte("0123456789")))
	cases := map[string][]byte{
		"truncated header": good[:6],
		"exceeds":          good[:len(good)-2],
		"child exceeds":    form("DJVU", chunk("INFO", []byte("01"))[:9]),
		"missing type":     chunk("FORM", []byte("DJ")),
	}
	for name, b := range cases {
		if _, err := Parse(b); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	deep := chunk("INFO", nil)
	for i := 0; i < maxDepth+2; i++ {
		deep = form("DJVU", deep)
	}
	if _, err := Parse(deep); err == nil || !strings.Contains(err.Error(), "deep") {
		t.Errorf("deep nesting: %v", err)
	}
}

func FuzzParse(f *testing.F) {
	f.Add(form("DJVU", chunk("INFO", []byte("0123456789"))))
	f.Fuzz(func(t *testing.T, b []byte) {
		Parse(b)
	})
}
