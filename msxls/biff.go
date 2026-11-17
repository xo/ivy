package msxls

import (
	"context"
	"encoding/binary"
	"fmt"
	"iter"
	"math"
	"unicode/utf16"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/msole"
	"github.com/xo/ivy/internal/offcrypto"
)

var le = binary.LittleEndian

// The types of the records that the package reads (MS-XLS 2.3).
const (
	recFormula              = 0x0006
	recEOF                  = 0x000A
	recDateMode             = 0x0022
	recFilePass             = 0x002F
	recFont                 = 0x0031
	recContinue             = 0x003C
	recCodePage             = 0x0042
	recDefColWidth          = 0x0055
	recColInfo              = 0x007D
	recBoundSheet           = 0x0085
	recMulRK                = 0x00BD
	recRString              = 0x00D6
	recXF                   = 0x00E0
	recInterfaceHdr         = 0x00E1
	recMergeCells           = 0x00E5
	recSST                  = 0x00FC
	recLabelSST             = 0x00FD
	recRRDHead              = 0x0138
	recUsrExcl              = 0x0194
	recFileLock             = 0x0195
	recRRDInfo              = 0x0196
	recNumber               = 0x0203
	recLabel                = 0x0204
	recBoolErr              = 0x0205
	recString               = 0x0207
	recRow                  = 0x0208
	recRK                   = 0x027E
	recFormat               = 0x041E
	recBOF                  = 0x0809
	recFormat5              = 0x001E
	recHeader               = 0x0014
	recFooter               = 0x0015
	recVerticalPageBreaks   = 0x001A
	recHorizontalPageBreaks = 0x001B
	recLeftMargin           = 0x0026
	recRightMargin          = 0x0027
	recTopMargin            = 0x0028
	recBottomMargin         = 0x0029
	recPrintGridlines       = 0x002B
	recWSBool               = 0x0081
	recHCenter              = 0x0083
	recPalette              = 0x0092
	recStandardWidth        = 0x0099
	recSetup                = 0x00A1
	recMulBlank             = 0x00BE
	recBlank                = 0x0201
	recDefaultRowHeight     = 0x0225
	recHeaderFooter         = 0x089C
	recName                 = 0x0018
)

// The versions of the BOF.
const (
	biff5 = 0x0500
	biff8 = 0x0600
)

// The types of the substreams of the BOF.
const (
	bofGlobals   = 0x0005
	bofWorksheet = 0x0010
)

// The block size of RC4 in the workbook stream.
const blockSize = 1024

// defaultPassword is the password that Excel uses to encrypt a workbook
// that a person protects without a password to open it.
const defaultPassword = "VelvetSweatshop"

// record is a record of the workbook stream: its type, the offset of its
// data in the stream, its data, and the data of the CONTINUE records after
// it.
type record struct {
	typ   uint16
	off   int
	data  []byte
	conts [][]byte
}

// records iterates over the records of wb from the offset start. It joins
// each record with the CONTINUE records after it, and stops at a record that
// the stream cuts.
func records(wb []byte, start int) iter.Seq[record] {
	return func(yield func(record) bool) {
		off := start
		next := func() (uint16, int, []byte, bool) {
			if off < 0 || off+4 > len(wb) {
				return 0, 0, nil, false
			}
			typ := le.Uint16(wb[off:])
			n := int(le.Uint16(wb[off+2:]))
			if off+4+n > len(wb) {
				return 0, 0, nil, false
			}
			r := wb[off+4 : off+4+n]
			at := off + 4
			off += 4 + n
			return typ, at, r, true
		}
		typ, at, data, ok := next()
		for ok {
			rec := record{typ: typ, off: at, data: data}
			for {
				typ, at, data, ok = next()
				if !ok || typ != recContinue {
					break
				}
				rec.conts = append(rec.conts, data)
			}
			if !yield(rec) {
				return
			}
		}
	}
}

// decrypt decrypts the workbook stream in place, from the record FILEPASS
// at the offset filePass. It leaves in the clear the headers of the
// records, the records that MS-XLS 2.2.10 names, and the first 4 bytes of
// BOUNDSHEET.
//
// The XOR obfuscation starts each record at the element of the
// obfuscation array of the end of the record: the byte at the position p of
// a record of n bytes takes the element p+n modulo 16 (MS-XLS 2.2.10).
func decrypt(ctx context.Context, wb []byte, filePass []byte, cfg ivy.Config, end int) error {
	if len(filePass) < 2 {
		return fmt.Errorf("filepass of %d bytes: %w", len(filePass), ivy.ErrFormat)
	}
	var info *offcrypto.Info
	switch le.Uint16(filePass) {
	case 0:
		if len(filePass) < 6 {
			return fmt.Errorf("filepass of the xor obfuscation of %d bytes: %w", len(filePass), ivy.ErrFormat)
		}
		info = offcrypto.XORMethod1(le.Uint16(filePass[2:]), le.Uint16(filePass[4:]))
	case 1:
		var err error
		if info, err = offcrypto.ParseEncryptionInfo(filePass[2:]); err != nil {
			return err
		}
		if info.Scheme != offcrypto.RC4 && info.Scheme != offcrypto.RC4CryptoAPI {
			return fmt.Errorf("workbook with %v encryption: %w", info.Scheme, ivy.ErrUnsupported)
		}
	default:
		return fmt.Errorf("workbook with the encryption type %d: %w", le.Uint16(filePass), ivy.ErrUnsupported)
	}
	key, err := msole.Key(ctx, info, cfg, defaultPassword)
	if err != nil {
		return fmt.Errorf("opening excel workbook: %w", err)
	}
	xor := info.Scheme == offcrypto.XOR
	for off := end; off+4 <= len(wb); {
		typ := le.Uint16(wb[off:])
		n := int(le.Uint16(wb[off+2:]))
		data := wb[off+4 : min(off+4+n, len(wb))]
		shift := 0
		if xor {
			shift = n
		}
		switch typ {
		case recBOF, recFilePass, recUsrExcl, recFileLock, recInterfaceHdr, recRRDInfo, recRRDHead:
		case recBoundSheet:
			if len(data) > 4 {
				if err := key.DecryptAt(data[4:], int64(off+8+shift), blockSize); err != nil {
					return err
				}
			}
		default:
			if err := key.DecryptAt(data, int64(off+4+shift), blockSize); err != nil {
				return err
			}
		}
		off += 4 + n
	}
	return nil
}

// reader reads the data of a record and of its CONTINUE records. A string
// whose characters go on in a CONTINUE record starts that record with a
// byte of flags (MS-XLS 2.5.293).
type reader struct {
	segs [][]byte
	seg  int
	pos  int
	bad  bool
}

// newReader returns a reader of the data of rec.
func newReader(rec record) *reader {
	segs := make([][]byte, 0, 1+len(rec.conts))
	segs = append(segs, rec.data)
	segs = append(segs, rec.conts...)
	return &reader{segs: segs}
}

// avail moves to the next segment when the current one ends, and returns
// the bytes left in the current one.
func (r *reader) avail() int {
	for r.seg < len(r.segs) && r.pos >= len(r.segs[r.seg]) {
		r.seg++
		r.pos = 0
	}
	if r.seg >= len(r.segs) {
		return 0
	}
	return len(r.segs[r.seg]) - r.pos
}

// bytes reads n bytes, across the segments.
func (r *reader) bytes(n int) []byte {
	out := make([]byte, 0, min(n, 1<<16))
	for n > 0 {
		k := min(n, r.avail())
		if k == 0 {
			r.bad = true
			return out
		}
		out = append(out, r.segs[r.seg][r.pos:r.pos+k]...)
		r.pos += k
		n -= k
	}
	return out
}

// skip skips n bytes, across the segments.
func (r *reader) skip(n int) {
	for n > 0 {
		k := min(n, r.avail())
		if k == 0 {
			r.bad = true
			return
		}
		r.pos += k
		n -= k
	}
}

func (r *reader) u8() byte {
	b := r.bytes(1)
	if len(b) < 1 {
		return 0
	}
	return b[0]
}

func (r *reader) u16() uint16 {
	b := r.bytes(2)
	if len(b) < 2 {
		return 0
	}
	return le.Uint16(b)
}

func (r *reader) u32() uint32 {
	b := r.bytes(4)
	if len(b) < 4 {
		return 0
	}
	return le.Uint32(b)
}

// chars reads n characters, of one byte or of two. A segment that starts in
// the middle of the characters starts with a byte of flags, whose low bit
// says the size of the characters after it.
func (r *reader) chars(n int, high bool, cp uint16) string {
	var units []uint16
	var narrow []byte
	for n > 0 {
		if r.pos >= len(r.segs[min(r.seg, len(r.segs)-1)]) {
			if r.avail() == 0 {
				r.bad = true
				break
			}
			high = r.segs[r.seg][r.pos]&1 != 0
			r.pos++
		}
		size := 1
		if high {
			size = 2
		}
		k := min(n, r.avail()/size)
		if k == 0 {
			r.bad = true
			break
		}
		b := r.segs[r.seg][r.pos : r.pos+k*size]
		if high {
			if len(narrow) > 0 {
				units = appendNarrow(units, narrow, cp)
				narrow = narrow[:0]
			}
			for i := 0; i < len(b); i += 2 {
				units = append(units, le.Uint16(b[i:]))
			}
		} else {
			narrow = append(narrow, b...)
		}
		r.pos += k * size
		n -= k
	}
	if len(narrow) > 0 {
		units = appendNarrow(units, narrow, cp)
	}
	return string(utf16.Decode(units))
}

// appendNarrow appends the characters of one byte each to units. In BIFF8
// they are the low bytes of UTF-16. In BIFF5 they are in the code page cp.
func appendNarrow(units []uint16, b []byte, cp uint16) []uint16 {
	if cp == 0 || cp == 1200 {
		for _, c := range b {
			units = append(units, uint16(c))
		}
		return units
	}
	return append(units, utf16.Encode([]rune(msole.Decode(b, cp)))...)
}

// richString reads an XLUnicodeRichExtendedString (MS-XLS 2.5.293): the
// text and its runs of fonts.
func (r *reader) richString() richString {
	cch := int(r.u16())
	flags := r.u8()
	runs, ext := 0, 0
	if flags&0x08 != 0 {
		runs = int(r.u16())
	}
	if flags&0x04 != 0 {
		ext = int(int32(r.u32()))
	}
	s := richString{text: r.chars(cch, flags&0x01 != 0, 0)}
	for range runs {
		ich, ifnt := int(r.u16()), int(r.u16())
		if r.bad {
			break
		}
		s.runs = append(s.runs, fontRun{ich: ich, ifnt: ifnt})
	}
	if ext > 0 {
		r.skip(ext)
	}
	return s
}

// shortString reads a ShortXLUnicodeString of BIFF8, or the string of BIFF5
// with a count of one byte and the characters of the code page cp.
func (r *reader) shortString(v uint16, cp uint16) string {
	cch := int(r.u8())
	if v < biff8 {
		return r.chars(cch, false, cp)
	}
	flags := r.u8()
	return r.chars(cch, flags&0x01 != 0, 0)
}

// longString reads an XLUnicodeString of BIFF8, or the string of BIFF5 with
// a count of two bytes and the characters of the code page cp.
func (r *reader) longString(v uint16, cp uint16) string {
	cch := int(r.u16())
	if v < biff8 {
		return r.chars(cch, false, cp)
	}
	flags := r.u8()
	return r.chars(cch, flags&0x01 != 0, 0)
}

// f64 decodes a float of 8 bytes, little endian.
func f64(b []byte) float64 {
	return math.Float64frombits(le.Uint64(b))
}

// rk decodes an RkNumber (MS-XLS 2.5.217).
func rk(v uint32) float64 {
	var f float64
	if v&0x02 != 0 {
		f = float64(int32(v) >> 2)
	} else {
		f = math.Float64frombits(uint64(v&0xFFFFFFFC) << 32)
	}
	if v&0x01 != 0 {
		f /= 100
	}
	return f
}
