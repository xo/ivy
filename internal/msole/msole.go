// Package msole holds the code that the binary formats of Microsoft Office
// share: Word, Excel and PowerPoint before 2007, Visio and Publisher. Each
// of them is a Compound File (MS-CFB), which the package cfb of magic reads
// (D24).
//
// The package opens a Compound File within the limits of an ivy.Config. It
// reads the stream "\x05SummaryInformation" (MS-OLEPS): the title, the
// author, the dates and the thumbnail. It decodes a thumbnail in WMF, EMF,
// DIB, PNG or JPEG, and draws a metafile into an image of a size for a
// preview. It also asks for the password of a file that RC4 encrypts, as D12
// says.
package msole

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"

	"github.com/xo/magic/cfb"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/charset"
	"github.com/xo/ivy/internal/offcrypto"
)

// Signature starts each Compound File.
var Signature = []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}

// SummaryName is the name of the stream of the summary information.
const SummaryName = "\x05SummaryInformation"

// Open opens the Compound File in src, within the limits lim. It returns an
// error that wraps ivy.ErrFormat if src is not a Compound File, and one that
// wraps ivy.ErrLimit if the file has more entries than lim.MaxObjects.
func Open(src ivy.Source, lim ivy.Limits) (*cfb.File, error) {
	lim = lim.WithDefaults()
	f, err := cfb.Open(src, src.Size(), cfb.Limits{
		Entries:    lim.MaxObjects,
		StreamSize: lim.MaxMemory,
	})
	if err != nil {
		return nil, Error(err)
	}
	return f, nil
}

// Error wraps an error of the reader of Compound Files in the error of ivy
// that matches it.
func Error(err error) error {
	switch {
	case errors.Is(err, cfb.ErrLimit):
		return fmt.Errorf("reading the compound file: %w: %w", ivy.ErrLimit, err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("reading the compound file: %w", err)
	}
	return fmt.Errorf("reading the compound file: %w: %w", ivy.ErrFormat, err)
}

// Has reports whether f holds a stream with the name.
func Has(f *cfb.File, name string) bool {
	e, err := f.Lookup(name)
	return err == nil && e.Type == cfb.StreamEntry
}

// ReadStream reads the stream name of f. It returns an error that wraps
// ivy.ErrFormat if f holds no such stream, and one that wraps ivy.ErrLimit if
// the stream is larger than lim.MaxMemory.
func ReadStream(f *cfb.File, name string, lim ivy.Limits) ([]byte, error) {
	lim = lim.WithDefaults()
	e, err := f.Lookup(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("compound file without the stream %q: %w", name, ivy.ErrFormat)
		}
		return nil, Error(err)
	}
	if e.Size > lim.MaxMemory {
		return nil, fmt.Errorf("stream %q of %d bytes: %w", name, e.Size, ivy.ErrLimit)
	}
	buf, err := f.ReadFile(name)
	if err != nil {
		return nil, Error(err)
	}
	return buf, nil
}

// Key derives the key of the encryption info from a password. It tries
// first each password of defaults, such as the password that Excel uses
// when a person gives none, then the password of cfg. While the password is
// wrong, it asks the function PasswordFunc of cfg for another one, as D12
// says. It returns an error that wraps ivy.ErrPassword if no password is
// right.
func Key(ctx context.Context, info *offcrypto.Info, cfg ivy.Config, defaults ...string) (*offcrypto.Key, error) {
	for _, password := range defaults {
		if key, err := info.Key(ctx, password); err == nil {
			return key, nil
		}
	}
	key, err := info.Key(ctx, cfg.Password)
	for attempt := 1; err != nil && errors.Is(err, ivy.ErrPassword) && cfg.PasswordFunc != nil; attempt++ {
		password, perr := cfg.PasswordFunc(ctx, attempt)
		if perr != nil {
			return nil, fmt.Errorf("asking for the password: %w: %w", ivy.ErrPassword, perr)
		}
		key, err = info.Key(ctx, password)
	}
	if err != nil {
		return nil, fmt.Errorf("deriving the key: %w", err)
	}
	return key, nil
}

// CodePageLabel returns the label of the Encoding Standard for the code page
// of Windows cp, such as "windows-1251" for 1251. It returns "" for a code
// page that the Encoding Standard does not name.
func CodePageLabel(cp uint16) string {
	switch {
	case cp == 1200:
		return "utf-16le"
	case cp == 1201:
		return "utf-16be"
	case cp == 65001:
		return "utf-8"
	case cp == 874 || cp >= 1250 && cp <= 1258:
		return "windows-" + strconv.Itoa(int(cp))
	case cp >= 28591 && cp <= 28606:
		return "iso-8859-" + strconv.Itoa(int(cp)-28590)
	}
	switch cp {
	case 437, 850, 20127:
		// the Encoding Standard has no label for these code pages, and their
		// letters of ASCII are the same as in windows-1252
		return "windows-1252"
	case 866:
		return "ibm866"
	case 932:
		return "shift_jis"
	case 936:
		return "gbk"
	case 949:
		return "euc-kr"
	case 950:
		return "big5"
	case 10000:
		return "macintosh"
	case 10007:
		return "x-mac-cyrillic"
	case 20866:
		return "koi8-r"
	case 21866:
		return "koi8-u"
	case 54936:
		return "gb18030"
	}
	return ""
}

// Decode decodes the bytes of a string in the code page cp as UTF-8. It
// takes windows-1252 for a code page that it does not know.
func Decode(buf []byte, cp uint16) string {
	if text, ok := charset.Decode(buf, CodePageLabel(cp)); ok {
		return text
	}
	text, _ := charset.Decode(buf, "windows-1252")
	return text
}
