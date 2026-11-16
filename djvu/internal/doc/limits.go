// ivy: this file is not in dejapdf. It holds the code that the copy adds
// for the limits of ivy (D5, D26).

package doc

import (
	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/bzz"
)

// unbzz decompresses a BZZ stream, up to lim.MaxMemory bytes.
func unbzz(b []byte, lim ivy.Limits) ([]byte, error) {
	lim = lim.WithDefaults()
	return bzz.DecodeMax(b, int(min(lim.MaxMemory, 1<<30)))
}

// u24 reads a 24-bit big-endian number.
//
// ivy: it is u24 of text.go, which the copy leaves out.
func u24(b []byte) int { return int(b[0])<<16 | int(b[1])<<8 | int(b[2]) }
