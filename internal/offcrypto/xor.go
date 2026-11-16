package offcrypto

import (
	"fmt"
	"math/bits"

	"github.com/xo/ivy"
)

// xorInfo holds the values that a binary format stores for the XOR
// obfuscation (MS-OFFCRYPTO 2.3.7).
type xorInfo struct {
	// method is 1 for Excel and 2 for Word.
	method int
	// xorKey is the XOR key of method 1. Excel stores it beside the
	// verifier.
	xorKey uint16
	// verifier is the password verifier: 16 bits for method 1, and 32 bits
	// for method 2, with the XOR key in the upper 16 bits.
	verifier uint32
}

// XORMethod1 returns the Info of the XOR obfuscation of Excel, from the key
// and the verifier of the record FILEPASS (MS-XLS 2.4.117). The key of the
// Info decrypts with the data transformation of method 1.
func XORMethod1(key, verifier uint16) *Info {
	return &Info{Scheme: XOR, xor: &xorInfo{method: 1, xorKey: key, verifier: uint32(verifier)}}
}

// XORMethod2 returns the Info of the XOR obfuscation of Word, from the
// verifier in FibBase.lKey (MS-DOC 2.2.6.1). The key of the Info decrypts
// with the data transformation of method 2.
func XORMethod2(verifier uint32) *Info {
	return &Info{Scheme: XOR, xor: &xorInfo{method: 2, verifier: verifier}}
}

// The arrays of MS-OFFCRYPTO 2.3.7.2.
var (
	xorPad = [15]byte{
		0xBB, 0xFF, 0xFF, 0xBA, 0xFF, 0xFF, 0xB9, 0x80,
		0x00, 0xBE, 0x0F, 0x00, 0xBF, 0x0F, 0x00,
	}
	xorInitialCode = [15]uint16{
		0xE1F0, 0x1D0F, 0xCC9C, 0x84C0, 0x110C,
		0x0E10, 0xF1CE, 0x313E, 0x1872, 0xE139,
		0xD40F, 0x84F9, 0x280C, 0xA96A, 0x4EC3,
	}
	xorMatrix = [105]uint16{
		0xAEFC, 0x4DD9, 0x9BB2, 0x2745, 0x4E8A, 0x9D14, 0x2A09,
		0x7B61, 0xF6C2, 0xFDA5, 0xEB6B, 0xC6F7, 0x9DCF, 0x2BBF,
		0x4563, 0x8AC6, 0x05AD, 0x0B5A, 0x16B4, 0x2D68, 0x5AD0,
		0x0375, 0x06EA, 0x0DD4, 0x1BA8, 0x3750, 0x6EA0, 0xDD40,
		0xD849, 0xA0B3, 0x5147, 0xA28E, 0x553D, 0xAA7A, 0x44D5,
		0x6F45, 0xDE8A, 0xAD35, 0x4A4B, 0x9496, 0x390D, 0x721A,
		0xEB23, 0xC667, 0x9CEF, 0x29FF, 0x53FE, 0xA7FC, 0x5FD9,
		0x47D3, 0x8FA6, 0x0F6D, 0x1EDA, 0x3DB4, 0x7B68, 0xF6D0,
		0xB861, 0x60E3, 0xC1C6, 0x93AD, 0x377B, 0x6EF6, 0xDDEC,
		0x45A0, 0x8B40, 0x06A1, 0x0D42, 0x1A84, 0x3508, 0x6A10,
		0xAA51, 0x4483, 0x8906, 0x022D, 0x045A, 0x08B4, 0x1168,
		0x76B4, 0xED68, 0xCAF1, 0x85C3, 0x1BA7, 0x374E, 0x6E9C,
		0x3730, 0x6E60, 0xDCC0, 0xA9A1, 0x4363, 0x86C6, 0x1DAD,
		0x3331, 0x6662, 0xCCC4, 0x89A9, 0x0373, 0x06E6, 0x0DCC,
		0x1021, 0x2042, 0x4084, 0x8108, 0x1231, 0x2462, 0x48C4,
	}
)

// xorPassword returns the password in single bytes, as MS-OFFCRYPTO 2.3.7.4
// says: the low byte of each character, or its high byte if the low byte
// is zero. It keeps the first 15 bytes.
func xorPassword(password string) []byte {
	var out []byte
	for _, r := range password {
		if len(out) == 15 {
			break
		}
		if r > 0xFFFF {
			// a character outside the BMP is two units of UTF-16
			r1, r2 := 0xD800+(r-0x10000)>>10, 0xDC00+(r-0x10000)&0x3FF
			out = append(out, lowOrHigh(uint16(r1)))
			if len(out) < 15 {
				out = append(out, lowOrHigh(uint16(r2)))
			}
			continue
		}
		out = append(out, lowOrHigh(uint16(r)))
	}
	return out
}

// lowOrHigh returns the low byte of u, or its high byte if the low byte is
// zero.
func lowOrHigh(u uint16) byte {
	if u&0xFF != 0 {
		return byte(u)
	}
	return byte(u >> 8)
}

// xorVerifier1 returns the password verifier of method 1 (MS-OFFCRYPTO
// 2.3.7.1). The empty password has the verifier 0.
func xorVerifier1(password []byte) uint16 {
	if len(password) == 0 {
		return 0
	}
	var v uint16
	for i := len(password); i >= 0; i-- {
		b := byte(len(password))
		if i > 0 {
			b = password[i-1]
		}
		v = (v>>14)&1 | (v<<1)&0x7FFF
		v ^= uint16(b)
	}
	return v ^ 0xCE4B
}

// xorKey1 returns the XOR key of method 1 (MS-OFFCRYPTO 2.3.7.2). The empty
// password has the key 0.
func xorKey1(password []byte) uint16 {
	if len(password) == 0 {
		return 0
	}
	key := xorInitialCode[len(password)-1]
	el := len(xorMatrix) - 1
	for i := len(password) - 1; i >= 0; i-- {
		c := password[i]
		for range 7 {
			if c&0x40 != 0 {
				key ^= xorMatrix[el]
			}
			c <<= 1
			el--
		}
	}
	return key
}

// xorRor returns the byte a XOR b, rotated right by one bit.
func xorRor(a, b byte) byte {
	return bits.RotateLeft8(a^b, -1)
}

// xorArray1 returns the obfuscation array of method 1 (MS-OFFCRYPTO
// 2.3.7.2).
func xorArray1(password []byte) [16]byte {
	key := xorKey1(password)
	hi, lo := byte(key>>8), byte(key)
	var a [16]byte
	i := len(password)
	if i%2 == 1 {
		a[i] = xorRor(xorPad[0], hi)
		i--
		a[i] = xorRor(password[len(password)-1], lo)
	}
	for i > 0 {
		i--
		a[i] = xorRor(password[i], hi)
		i--
		a[i] = xorRor(password[i], lo)
	}
	i = 15
	for pad := 15 - len(password); pad > 0; {
		// the empty password has no byte for the last element
		if pad < len(xorPad) {
			a[i] = xorRor(xorPad[pad], hi)
		}
		i--
		pad--
		a[i] = xorRor(xorPad[pad], lo)
		i--
		pad--
	}
	return a
}

// xorArray2 returns the obfuscation array of method 2 (MS-OFFCRYPTO
// 2.3.7.5).
func xorArray2(password []byte) [16]byte {
	key := xorKey1(password)
	hi, lo := byte(key>>8), byte(key)
	var a [16]byte
	n := copy(a[:], password)
	copy(a[n:], xorPad[:])
	for i := 0; i < 16; i += 2 {
		a[i] = xorRor(a[i], lo)
		a[i+1] = xorRor(a[i+1], hi)
	}
	return a
}

// XORVerifier2 returns the password verifier of method 2 that Word stores
// in FibBase.lKey (MS-OFFCRYPTO 2.3.7.4): the XOR key in the upper 16 bits,
// and the verifier of method 1 in the lower 16 bits.
func XORVerifier2(password string) uint32 {
	pw := xorPassword(password)
	return uint32(xorKey1(pw))<<16 | uint32(xorVerifier1(pw))
}

// key derives the obfuscation array of the password and makes sure that
// the password matches the verifier. The empty password never matches,
// because a file without a password is not obfuscated.
func (xi *xorInfo) key(password string) (*Key, error) {
	pw := xorPassword(password)
	if len(pw) == 0 {
		return nil, fmt.Errorf("xor obfuscation without a password: %w", ivy.ErrPassword)
	}
	key, verifier := xorKey1(pw), xorVerifier1(pw)
	k := &Key{scheme: XOR, xorMethod: xi.method}
	switch xi.method {
	case 1:
		if verifier != uint16(xi.verifier) || key != xi.xorKey {
			return nil, fmt.Errorf("xor obfuscation: %w", ivy.ErrPassword)
		}
		k.xorArray = xorArray1(pw)
	default:
		if uint32(key)<<16|uint32(verifier) != xi.verifier {
			return nil, fmt.Errorf("xor obfuscation: %w", ivy.ErrPassword)
		}
		k.xorArray = xorArray2(pw)
	}
	return k, nil
}

// deobfuscate reverses the XOR obfuscation of buf in place. The first byte
// of buf takes the element index of the obfuscation array.
//
// Method 1 (MS-OFFCRYPTO 2.3.7.3) applies the array, then rotates each byte
// right by 5 bits. Method 2 (MS-OFFCRYPTO 2.3.7.6) applies the array, and
// leaves a byte that is 0, or that is equal to its element, as it is.
func (k *Key) deobfuscate(buf []byte, index int64) {
	i := int(index & 15)
	for j, b := range buf {
		x := k.xorArray[i]
		switch {
		case k.xorMethod == 1:
			buf[j] = bits.RotateLeft8(b^x, -5)
		case b != 0 && b != x:
			buf[j] = b ^ x
		}
		i = (i + 1) & 15
	}
}
