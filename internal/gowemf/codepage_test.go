// Copied from github.com/mgilbir/gowemf, file codepage_test.go, at commit
// 377668cb7b13 (D7, docs/UPSTREAM.md). Its MIT license is in
// internal/gowemf/LICENSE.
// ivy: the tests that compare the tables with the downloaded files of the
// code pages are left out. They need the network and run the Go command.

package gowemf

import (
	"fmt"
	"sync"
	"testing"
	"unicode/utf16"
)

func TestCodePageSpotValues(t *testing.T) {
	// Independent spot checks of well-known assignments.
	for _, c := range []struct {
		cp   uint16
		b    byte
		want uint16
	}{{1252, 0x80, 0x20ac}, {1252, 0xe9, 0x00e9}, {1251, 0xc0, 0x0410}, {1253, 0xc1, 0x0391}, {1250, 0x8a, 0x0160}, {874, 0xa1, 0x0e01}, {1255, 0xe0, 0x05d0}, {1256, 0xc7, 0x0627}} {
		if got := codePageHigh[c.cp][c.b-0x80]; got != c.want {
			t.Errorf("code page %d byte %#x = %#04x, want %#04x", c.cp, c.b, got, c.want)
		}
	}
}

// TestDBCSDecode checks well-known characters, the default character for
// invalid sequences (MS-UCODEREF 3.1.5.1.1) and concurrent first use.
func TestDBCSDecode(t *testing.T) {
	for _, c := range []struct {
		cp    uint16
		in    string
		want  string
		spans []int
	}{
		{950, "\xa4\xa4\xa4\xe5", "中文", []int{2, 2}},
		{950, "\xb7\x73\xb2\xd3\xa9\xfa\xc5\xe9", "新細明體", []int{2, 2, 2, 2}},
		{932, "\x82\x6c\x82\x72\x20\x83\x53\x83\x56\x83\x62\x83\x4e", "ＭＳ ゴシック", []int{2, 2, 1, 2, 2, 2, 2}},
		{932, "A\xb1", "Aｱ", []int{1, 1}}, // a half-width katakana single byte
		{936, "\xcb\xce\xcc\xe5", "宋体", []int{2, 2}},
		{949, "\xb1\xbc\xb8\xb2", "굴림", []int{2, 2}},
		{932, "\x81\x20A", "・A", []int{2, 1}}, // invalid trail: both bytes become the default
		{932, "A\x81", "A・", []int{1, 1}},     // a lead byte at the end
		{936, "\x81\x7f", "?", []int{2}},
		{932, "\x81\x00A", "\u30fb\x00A", []int{1, 1, 1}}, // NUL is not taken as a trail byte
	} {
		got, spans := dbcsCodePages[c.cp].decode([]byte(c.in))
		if string(utf16.Decode(got)) != c.want || fmt.Sprint(spans) != fmt.Sprint(c.spans) {
			t.Errorf("%d %q: %q %v, want %q %v", c.cp, c.in, string(utf16.Decode(got)), spans, c.want, c.spans)
		}
	}
	fresh := &dbcsCodePage{defaultChar: '?', leads: dbcsCodePages[949].leads, single: dbcsCodePages[949].single, pairs: dbcsCodePages[949].pairs}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, _ := fresh.decode([]byte("\xb1\xbc")); got[0] != 0xad74 {
				t.Error("concurrent decode", got)
			}
		}()
	}
	wg.Wait()
}
