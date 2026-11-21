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
// ivy: it is u24 of text.go of dejapdf. It is in this file because the
// palettes used it before the copy held text.go.
func u24(b []byte) int { return int(b[0])<<16 | int(b[1])<<8 | int(b[2]) }
