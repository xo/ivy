package odfcrypt

import (
	"sync"
	"unicode/utf8"

	"github.com/xo/ivy/internal/charset"
)

// cp1252 maps a character to its byte in windows-1252, for the bytes from
// 0x80 to 0xff.
var cp1252 = sync.OnceValue(func() map[rune]byte {
	m := make(map[rune]byte, 128)
	for b := 0x80; b <= 0xff; b++ {
		s, ok := charset.Decode([]byte{byte(b)}, "windows-1252")
		if r, _ := utf8.DecodeRuneInString(s); ok && r != utf8.RuneError {
			m[r] = byte(b)
		}
	}
	return m
})

// Windows1252 returns the bytes of password in windows-1252, as a string,
// which old versions of StarOffice hashed in place of its UTF-8 bytes. It
// returns false if the password is ASCII, whose bytes are the same, or if
// windows-1252 does not hold one of its characters.
func Windows1252(password string) (string, bool) {
	m := cp1252()
	out := make([]byte, 0, len(password))
	ascii := true
	for _, r := range password {
		switch b, ok := m[r]; {
		case r < 0x80:
			out = append(out, byte(r))
		case ok:
			out = append(out, b)
			ascii = false
		default:
			return "", false
		}
	}
	if ascii {
		return "", false
	}
	return string(out), true
}
