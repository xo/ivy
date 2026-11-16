// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the fuzz test of the parser. Upstream has no such test.

package cmap

import (
	"errors"
	"testing"

	"github.com/xo/ivy"
)

// FuzzParse parses the input as a CMap, with the predefined CMaps as the
// resolver, and looks up some codes. The parser must not panic or hang, and
// each error must wrap a sentinel error of ivy.
func FuzzParse(f *testing.F) {
	f.Add([]byte(preamble + "3 begincidrange <0000> <00FF> 0 <0100> <01FF> 256 <8140> <817E> 633 endcidrange"))
	f.Add([]byte(preamble + "2 beginbfchar <0041> <0048> <005F> <00660066> endbfchar"))
	f.Add([]byte(preamble + "1 beginbfrange <005F> <0061> [<00660066> <00660069> <0066006C>] endbfrange"))
	f.Add([]byte(preamble + "2 beginnotdefchar <03> 10 <20> 20 endnotdefchar"))
	f.Add([]byte("/UniJIS-UCS2-HW-H usecmap 1 begincidchar <20> 5 endcidchar"))
	f.Add([]byte("<< /Registry /ABCDEF+SimSun /Ordering (pdfbeaninc) /Supplement 0 >> def"))
	for _, name := range []Name{H, UniJisUcs2HwH, AdobeJapan1Ucs2, GbkEucH} {
		data, ok := LoadEmbedded(name)
		if !ok {
			f.Fatalf("embedded %s CMap not found", name)
		}
		// A short prefix keeps the corpus small and still reaches each
		// segment parser.
		f.Add(data[:min(len(data), 512)])
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		c, err := Parse(data, LoadEmbedded)
		if err != nil {
			if !errors.Is(err, ivy.ErrFormat) && !errors.Is(err, ivy.ErrLimit) {
				t.Fatalf("error %v wraps no sentinel error of ivy", err)
			}
			return
		}
		for _, code := range []uint32{0, 0x20, 0x41, 0xff, 0x2121, 0x8140, 0xffff, 0xffffffff} {
			for byteLen := uint8(0); byteLen <= 5; byteLen++ {
				_, _ = c.LookupCIDCode(code, byteLen)
			}
			_, _ = c.LookupBfString(code)
		}
		for _, r := range c.CodespaceRanges() {
			_ = r.ByteLen()
		}
	})
}
