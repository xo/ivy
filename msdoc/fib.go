package msdoc

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/xo/magic/cfb"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/msole"
	"github.com/xo/ivy/internal/offcrypto"
)

var le = binary.LittleEndian

// The flags of the FibBase (MS-DOC 2.5.2).
const (
	flagDot        = 0x0001
	flagEncrypted  = 0x0100
	flagWhichTable = 0x0200
	flagObfuscated = 0x8000
)

// The first nFib of Word 97. An older file is Word 6 or Word 95, whose
// format is not MS-DOC.
const nFibWord97 = 0x00C1

// clearBytes is the number of bytes at the start of the stream WordDocument
// that the encryption leaves in the clear: the FibBase and a part of the
// FibRgW97 (MS-DOC 2.2.6.2).
const clearBytes = 68

// The indexes of the pairs of FibRgFcLcb97 (MS-DOC 2.5.6) that the package
// reads.
const (
	fcStshf        = 1
	fcPlcfSed      = 6
	fcPlcfBteChpx  = 12
	fcPlcfBtePapx  = 13
	fcSttbfFfn     = 15
	fcClx          = 33
	fcLcbCountMin  = 34
	ccpTextIndex   = 3
	cslwCountMin   = 4
	fibBaseSize    = 32
	wIdentWord     = 0xA5EC
	fibRgWOffset   = 32
	cfbWordName    = "WordDocument"
	cfbTable0Name  = "0Table"
	cfbTable1Name  = "1Table"
	blockSizeBytes = 512
)

// fib is the part of the File Information Block that the package reads
// (MS-DOC 2.5.1).
type fib struct {
	nFib    uint16
	lid     uint16
	flags   uint16
	ccpText uint32
	fcLcb   [][2]uint32
}

// streams is a Word document: its FIB and its two streams, decrypted.
type streams struct {
	fib   fib
	word  []byte
	table []byte
}

// readStreams reads the streams WordDocument and the table stream of f, and
// decrypts them if a password protects them.
func readStreams(ctx context.Context, f *cfb.File, cfg ivy.Config) (*streams, error) {
	lim := cfg.Limits.WithDefaults()
	word, err := msole.ReadStream(f, cfbWordName, lim)
	if err != nil {
		return nil, err
	}
	if len(word) < fibBaseSize+2 || le.Uint16(word) != wIdentWord {
		return nil, fmt.Errorf("word document without a fib: %w", ivy.ErrFormat)
	}
	s := &streams{word: word}
	s.fib.nFib = le.Uint16(word[2:])
	s.fib.lid = le.Uint16(word[6:])
	s.fib.flags = le.Uint16(word[10:])
	if s.fib.nFib < nFibWord97 {
		return nil, fmt.Errorf("word document of nfib %#x, before word 97: %w", s.fib.nFib, ivy.ErrUnsupported)
	}
	name := cfbTable0Name
	if s.fib.flags&flagWhichTable != 0 {
		name = cfbTable1Name
	}
	if s.table, err = msole.ReadStream(f, name, lim); err != nil {
		return nil, err
	}
	if s.fib.flags&flagEncrypted != 0 {
		if err := s.decrypt(ctx, cfg); err != nil {
			return nil, err
		}
	}
	if err := s.fib.parse(word); err != nil {
		return nil, err
	}
	return s, nil
}

// decrypt decrypts the streams with the key of the password. The table
// stream starts with the encryption header, of lKey bytes, and the stream
// WordDocument starts with 68 bytes in the clear. Both start a new block of
// RC4 every 512 bytes from their start.
func (s *streams) decrypt(ctx context.Context, cfg ivy.Config) error {
	if s.fib.flags&flagObfuscated != 0 {
		return fmt.Errorf("word document with xor obfuscation: %w", ivy.ErrUnsupported)
	}
	lKey := int(le.Uint32(s.word[14:]))
	if lKey <= 0 || lKey > len(s.table) {
		return fmt.Errorf("encryption header of %d bytes: %w", lKey, ivy.ErrFormat)
	}
	info, err := offcrypto.ParseEncryptionInfo(s.table[:lKey])
	if err != nil {
		return err
	}
	if info.Scheme != offcrypto.RC4 && info.Scheme != offcrypto.RC4CryptoAPI {
		return fmt.Errorf("word document with %v encryption: %w", info.Scheme, ivy.ErrUnsupported)
	}
	key, err := msole.Key(ctx, info, cfg)
	if err != nil {
		return fmt.Errorf("opening word document: %w", err)
	}
	if len(s.word) > clearBytes {
		if err := key.DecryptAt(s.word[clearBytes:], clearBytes, blockSizeBytes); err != nil {
			return err
		}
	}
	return key.DecryptAt(s.table[lKey:], int64(lKey), blockSizeBytes)
}

// parse reads the parts of the FIB after the FibBase: the counts of
// characters and the pairs of offsets and sizes in the table stream.
func (fb *fib) parse(word []byte) error {
	off := fibRgWOffset
	csw := int(u16(word, off))
	off += 2 + 2*csw
	cslw := int(u16(word, off))
	if cslw < cslwCountMin || off+2+4*cslw > len(word) {
		return fmt.Errorf("fib with %d longs: %w", cslw, ivy.ErrFormat)
	}
	fb.ccpText = le.Uint32(word[off+2+4*ccpTextIndex:])
	off += 2 + 4*cslw
	n := int(u16(word, off))
	off += 2
	n = min(n, (len(word)-off)/8)
	if n < fcLcbCountMin {
		return fmt.Errorf("fib with %d pairs of offsets: %w", n, ivy.ErrFormat)
	}
	fb.fcLcb = make([][2]uint32, n)
	for i := range n {
		fb.fcLcb[i] = [2]uint32{le.Uint32(word[off+8*i:]), le.Uint32(word[off+8*i+4:])}
	}
	return nil
}

// part returns the bytes of the structure at the pair i of the FIB in the
// table stream, or nil if the file holds none or the pair is out of the
// stream.
func (s *streams) part(i int) []byte {
	if i >= len(s.fib.fcLcb) {
		return nil
	}
	fc, lcb := uint64(s.fib.fcLcb[i][0]), uint64(s.fib.fcLcb[i][1])
	if lcb == 0 || fc+lcb > uint64(len(s.table)) {
		return nil
	}
	return s.table[fc : fc+lcb]
}

// u16 returns the uint16 at off in b, or 0 past the end.
func u16(b []byte, off int) uint16 {
	if off < 0 || off+2 > len(b) {
		return 0
	}
	return le.Uint16(b[off:])
}

// u32 returns the uint32 at off in b, or 0 past the end.
func u32(b []byte, off int) uint32 {
	if off < 0 || off+4 > len(b) {
		return 0
	}
	return le.Uint32(b[off:])
}
