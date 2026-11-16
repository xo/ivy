// Package charset decodes text in the encodings that the WHATWG Encoding
// Standard names, such as windows-1251, KOI8-R, Shift_JIS and GBK. It is a
// thin layer over golang.org/x/text/encoding/htmlindex (D7). The ebooks use
// it for the encodings that okage does not decode: the HTML parser of okage
// decodes UTF-8, UTF-16 and windows-1252 only.
package charset

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
)

// ErrUnknown is the error of an encoding that the Encoding Standard does not
// name.
var ErrUnknown = errors.New("charset: unknown encoding")

// lookup returns the encoding that label names. The label is any label of
// the Encoding Standard, in any case, such as "cp1251" or "KOI8-R".
func lookup(label string) (encoding.Encoding, error) {
	enc, err := htmlindex.Get(label)
	if err != nil {
		return nil, fmt.Errorf("charset: looking up %q: %w", label, ErrUnknown)
	}
	return enc, nil
}

// Known reports whether the Encoding Standard names label.
func Known(label string) bool {
	_, err := lookup(label)
	return err == nil
}

// Decode decodes buf from the encoding that label names, and returns the
// text as UTF-8. A byte that the encoding does not map becomes U+FFFD. It
// returns false as its second result when the Encoding Standard does not
// name label. Then the text is buf as it is.
func Decode(buf []byte, label string) (string, bool) {
	enc, err := lookup(label)
	if err != nil {
		return string(buf), false
	}
	out, err := enc.NewDecoder().Bytes(buf)
	if err != nil {
		// a decoder of the Encoding Standard replaces each byte that it does
		// not map, so it fails only on a fault of x/text
		return string(bytes.ToValidUTF8(buf, []byte("�"))), true
	}
	return string(out), true
}

// Reader returns a reader of the text of r as UTF-8, from the encoding that
// label names. It has the type CharsetReader of the package html of okage.
// It returns an error that wraps ErrUnknown when the Encoding Standard does
// not name label.
func Reader(label string, r io.Reader) (io.Reader, error) {
	enc, err := lookup(label)
	if err != nil {
		return nil, err
	}
	return enc.NewDecoder().Reader(r), nil
}
