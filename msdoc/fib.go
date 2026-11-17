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
	flagComplex    = 0x0004
	flagEncrypted  = 0x0100
	flagWhichTable = 0x0200
	flagObfuscated = 0x8000
)

// The first nFib of Word 97. An older file is Word 6 or Word 95, whose
// format is not MS-DOC.
const nFibWord97 = 0x00C1

// The FIB of Word 6 and Word 95 starts with wIdentWord6. It holds every
// structure in the stream WordDocument, and the offsets of its fields are
// not those of MS-DOC. The package reads these fields of it.
const (
	wIdentWord6       = 0xA5DC
	word6ChseOffset   = 0x14
	word6FcMinOffset  = 0x18
	word6FcMacOffset  = 0x1C
	word6CcpOffset    = 0x34
	word6PlcfSed      = 0x88
	word6FcClxOffset  = 0x160
	word6FibSize      = 0x168
	word6CharsetMac   = 256
	word6CodePageMac  = 10000
	word6CodePageANSI = 1252
)

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
	ccpFtnIndex    = 4
	ccpHddIndex    = 5
	ccpHdrTxbxIdx  = 10
	fcPlcffndRef   = 2
	fcPlcffndTxt   = 3
	fcPlcSpaHdr    = 41
	fcPlcfendRef   = 46
	fcPlcfendTxt   = 47
	fcPlcftxbxTxt  = 56
	fcPlcfHdrTxbx  = 58
	fcPlcfHdd      = 11
	fcDop          = 31
	fcPlcSpaMom    = 40
	fcDggInfo      = 50
	fcPlfLst       = 73
	fcPlfLfo       = 74
	cslwCountMin   = 4
	fibBaseSize    = 32
	wIdentWord     = 0xA5EC
	fibRgWOffset   = 32
	cfbWordName    = "WordDocument"
	cfbTable0Name  = "0Table"
	cfbTable1Name  = "1Table"
	cfbDataName    = "Data"
	blockSizeBytes = 512
)

// fib is the part of the File Information Block that the package reads
// (MS-DOC 2.5.1).
type fib struct {
	nFib    uint16
	lid     uint16
	flags   uint16
	ccpText uint32
	// ccpFtn and ccpHdd are the counts of the characters of the footnotes
	// and of the headers, which follow the main document. The macros, the
	// comments, the endnotes, the text boxes and the text boxes of the
	// headers follow them, in that order (MS-DOC 2.4.1).
	ccpFtn, ccpHdd uint32
	ccpMcr, ccpAtn uint32
	ccpEdn         uint32
	ccpTxbx        uint32
	ccpHdrTxbx     uint32
	fcLcb          [][2]uint32
}

// streams is a Word document: its FIB and its two streams, decrypted.
type streams struct {
	fib   fib
	word  []byte
	table []byte
	// data is the stream Data, which holds the pictures, or nil.
	data []byte
	// word6 is set for a document of Word 6 or Word 95, whose table stream
	// is the stream WordDocument. Its text has one byte for each character,
	// in the code page codePage, from the byte fcMin.
	word6    bool
	codePage uint16
	fcMin    uint32
	fcMac    uint32
}

// readStreams reads the streams WordDocument, the table stream and the
// stream Data of f, and decrypts them if a password protects them.
func readStreams(ctx context.Context, f *cfb.File, cfg ivy.Config) (*streams, error) {
	lim := cfg.Limits.WithDefaults()
	word, err := msole.ReadStream(f, cfbWordName, lim)
	if err != nil {
		return nil, err
	}
	if len(word) >= fibBaseSize+2 && le.Uint16(word) == wIdentWord6 {
		return readWord6(word)
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
	// the stream Data holds the pictures, and a document without a picture
	// can have none
	if data, err := msole.ReadStream(f, cfbDataName, lim); err == nil {
		s.data = data
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

// readWord6 reads the FIB of a document of Word 6 or Word 95. The package
// reads only its text: the pieces of a document that Word saved fast, or
// the bytes from fcMin to fcMac. It does not read the properties of the
// characters and the paragraphs, whose modifiers have other codes than in
// Word 97.
func readWord6(word []byte) (*streams, error) {
	if len(word) < word6FibSize {
		return nil, fmt.Errorf("word 6 document with a fib of %d bytes: %w", len(word), ivy.ErrFormat)
	}
	s := &streams{word: word, table: word, word6: true, codePage: word6CodePageANSI}
	s.fib.nFib = le.Uint16(word[2:])
	s.fib.lid = le.Uint16(word[6:])
	s.fib.flags = le.Uint16(word[10:])
	if s.fib.flags&flagEncrypted != 0 {
		return nil, fmt.Errorf("word 6 document with a password: %w", ivy.ErrUnsupported)
	}
	if le.Uint16(word[word6ChseOffset:]) == word6CharsetMac {
		s.codePage = word6CodePageMac
	}
	s.fcMin = le.Uint32(word[word6FcMinOffset:])
	s.fcMac = le.Uint32(word[word6FcMacOffset:])
	s.fib.ccpText = le.Uint32(word[word6CcpOffset:])
	s.fib.fcLcb = make([][2]uint32, fcLcbCountMin)
	s.fib.fcLcb[fcPlcfSed] = [2]uint32{le.Uint32(word[word6PlcfSed:]), le.Uint32(word[word6PlcfSed+4:])}
	if s.fib.flags&flagComplex != 0 {
		s.fib.fcLcb[fcClx] = [2]uint32{le.Uint32(word[word6FcClxOffset:]), le.Uint32(word[word6FcClxOffset+4:])}
	}
	return s, nil
}

// decrypt decrypts the streams with the key of the password. For RC4, the
// table stream starts with the encryption header, of lKey bytes, and the
// stream WordDocument starts with 68 bytes in the clear. Both start a new
// block of RC4 every 512 bytes from their start. The XOR obfuscation
// leaves the same 68 bytes in the clear, and has no header (MS-DOC
// 2.2.6.1).
func (s *streams) decrypt(ctx context.Context, cfg ivy.Config) error {
	if s.fib.flags&flagObfuscated != 0 {
		key, err := msole.Key(ctx, offcrypto.XORMethod2(le.Uint32(s.word[14:])), cfg)
		if err != nil {
			return fmt.Errorf("opening word document: %w", err)
		}
		if len(s.word) > clearBytes {
			if err := key.DecryptAt(s.word[clearBytes:], clearBytes, blockSizeBytes); err != nil {
				return err
			}
		}
		if err := key.DecryptAt(s.table, 0, blockSizeBytes); err != nil {
			return err
		}
		return key.DecryptAt(s.data, 0, blockSizeBytes)
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
	if err := key.DecryptAt(s.table[lKey:], int64(lKey), blockSizeBytes); err != nil {
		return err
	}
	return key.DecryptAt(s.data, 0, blockSizeBytes)
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
	if cslw > ccpHddIndex {
		fb.ccpFtn = le.Uint32(word[off+2+4*ccpFtnIndex:])
		fb.ccpHdd = le.Uint32(word[off+2+4*ccpHddIndex:])
	}
	if cslw > ccpHdrTxbxIdx {
		ccp := func(i int) uint32 {
			return le.Uint32(word[off+2+4*(ccpHddIndex+i):])
		}
		fb.ccpMcr, fb.ccpAtn, fb.ccpEdn, fb.ccpTxbx, fb.ccpHdrTxbx = ccp(1), ccp(2), ccp(3), ccp(4), ccp(5)
	}
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
