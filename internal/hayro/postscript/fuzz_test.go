// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the fuzz test of the scanner. Upstream has no such test.

package postscript

import (
	"errors"
	"testing"

	"github.com/xo/ivy"
)

// maxFuzzProcedures is the number of procedures that FuzzScanner scans. It
// keeps the time of one input small.
const maxFuzzProcedures = 1024

// FuzzScanner scans each token of the input, decodes each string and scans
// each procedure. The scanner must not panic or hang, each error must wrap a
// sentinel error of ivy, and each token must move the scanner forward.
func FuzzScanner(f *testing.F) {
	for _, seed := range []string{
		"/CIDInit /ProcSet findresource begin 12 dict begin begincmap",
		"1 begincodespacerange <00> <FF> endcodespacerange",
		"[123 /abc (xyz)] <</Registry (Adobe)>>",
		"{dup 0 lt {neg} if} {//mydict begin}",
		`(a\nb\053\36\3 (x) \
c) <48 65 6C> <~87cURDZ~> <~z~>`,
		"16#FFFE 8#1777 -.002 1e400 9223372036854775808 23A -16#FF",
		"% comment\n42 % another\n/Name",
		"{(}) <7D> <~Ac~>} {1 % comment with }\n2}",
		"<<<41>>> }>x",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		scanners := []*Scanner{NewScanner(data)}
		var buf []byte
		for n := 0; len(scanners) > 0 && n < maxFuzzProcedures; n++ {
			s := scanners[len(scanners)-1]
			scanners = scanners[:len(scanners)-1]
			for !s.AtEnd() {
				before := s.Offset()
				tok, err := s.ParseToken()
				if err != nil {
					if !errors.Is(err, ivy.ErrFormat) && !errors.Is(err, ivy.ErrLimit) {
						t.Fatalf("error %v wraps no sentinel error of ivy", err)
					}
					break
				}
				if s.Offset() <= before {
					t.Fatalf("token %#v did not move the scanner from %d", tok, before)
				}
				switch tok := tok.(type) {
				case String:
					buf, err = tok.DecodeInto(buf)
					if err != nil && !errors.Is(err, ivy.ErrFormat) {
						t.Fatalf("error %v does not wrap ivy.ErrFormat", err)
					}
				case Procedure:
					scanners = append(scanners, tok.Tokens())
				case Number:
					_ = tok.Float64()
				case Name:
					_, _ = tok.AsStr()
				}
			}
		}
	})
}
