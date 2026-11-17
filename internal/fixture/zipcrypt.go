package fixture

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha1" //nolint:gosec // the AES of WinZip authenticates with HMAC-SHA1
	"encoding/binary"
	"hash/crc32"
)

// ZipCipher is a method of encryption of the entries of a zip archive.
type ZipCipher int

// The methods of encryption of EncryptedZip.
const (
	// ZipCrypto is the traditional encryption of PKWARE.
	ZipCrypto ZipCipher = iota
	// WinZipAES1 is AE-1 of WinZip with a key of 256 bits, which keeps the
	// CRC of the entry.
	WinZipAES1
	// WinZipAES2 is AE-2 of WinZip with a key of 256 bits, which sets the
	// CRC to 0.
	WinZipAES2
)

// EncryptedZip returns a zip archive that holds the entries, in order, each
// compressed and encrypted with the password.
func EncryptedZip(c ZipCipher, password string, entries ...Entry) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, e := range entries {
		method := zip.Deflate
		packed := e.Data
		if e.Store {
			method = zip.Store
		} else {
			var b bytes.Buffer
			fw, err := flate.NewWriter(&b, flate.DefaultCompression)
			if err != nil {
				panic(err)
			}
			if _, err := fw.Write(e.Data); err != nil {
				panic(err)
			}
			if err := fw.Close(); err != nil {
				panic(err)
			}
			packed = b.Bytes()
		}
		sum := crc32.ChecksumIEEE(e.Data)
		hdr := &zip.FileHeader{
			Name:               e.Name,
			Method:             method,
			Flags:              0x1,
			CRC32:              sum,
			UncompressedSize64: uint64(len(e.Data)),
		}
		var data []byte
		switch c {
		case ZipCrypto:
			data = zipCrypt(packed, password, byte(sum>>24), byte(i))
		default:
			version := uint16(1)
			if c == WinZipAES2 {
				version, hdr.CRC32 = 2, 0
			}
			hdr.Extra = binary.LittleEndian.AppendUint16(nil, 0x9901)
			hdr.Extra = binary.LittleEndian.AppendUint16(hdr.Extra, 7)
			hdr.Extra = binary.LittleEndian.AppendUint16(hdr.Extra, version)
			hdr.Extra = append(hdr.Extra, 'A', 'E', 3)
			hdr.Extra = binary.LittleEndian.AppendUint16(hdr.Extra, method)
			hdr.Method = 99
			data = winZipAES(packed, password, byte(i))
		}
		hdr.CompressedSize64 = uint64(len(data))
		w, err := zw.CreateRaw(hdr)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(data); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// zipCrypt encrypts data with ZipCrypto, after a header of 12 bytes whose
// last byte is check. seed makes the other bytes of the header.
func zipCrypt(data []byte, password string, check, seed byte) []byte {
	k := [3]uint32{0x12345678, 0x23456789, 0x34567890}
	crc := func(c uint32, b byte) uint32 { return crc32.IEEETable[byte(c)^b] ^ c>>8 }
	update := func(b byte) {
		k[0] = crc(k[0], b)
		k[1] = (k[1]+k[0]&0xff)*134775813 + 1
		k[2] = crc(k[2], byte(k[1]>>24))
	}
	for i := range len(password) {
		update(password[i])
	}
	plain := make([]byte, 12, 12+len(data))
	for i := range 11 {
		plain[i] = seed + byte(i)*37
	}
	plain[11] = check
	plain = append(plain, data...)
	out := make([]byte, len(plain))
	for i, p := range plain {
		t := k[2] | 2
		out[i] = p ^ byte((t*(t^1))>>8)
		update(p)
	}
	return out
}

// winZipAES encrypts data with the AES of WinZip and a key of 256 bits: the
// salt, the password verifier, the data in the mode CTR with a counter that
// is little-endian, and the authentication code. seed makes the salt.
func winZipAES(data []byte, password string, seed byte) []byte {
	salt := make([]byte, 16, 16+2+len(data)+10)
	for i := range salt {
		salt[i] = seed + byte(i)*11
	}
	keys, err := pbkdf2.Key(sha1.New, password, salt, 1000, 66)
	if err != nil {
		panic(err)
	}
	block, err := aes.NewCipher(keys[:32])
	if err != nil {
		panic(err)
	}
	enc := make([]byte, len(data))
	var counter, stream [16]byte
	for i := range data {
		if i%16 == 0 {
			for j := range counter {
				counter[j]++
				if counter[j] != 0 {
					break
				}
			}
			block.Encrypt(stream[:], counter[:])
		}
		enc[i] = data[i] ^ stream[i%16]
	}
	mac := hmac.New(sha1.New, keys[32:64])
	mac.Write(enc)
	out := salt
	out = append(out, keys[64:66]...)
	out = append(out, enc...)
	return append(out, mac.Sum(nil)[:10]...)
}
