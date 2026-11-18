package mobi

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
)

// The compressions of the text records, at offset 0 of record 0.
const (
	compressionNone     = 1
	compressionPalmDoc  = 2
	compressionHuffCDIC = 17480
)

// The text encodings of the MOBI header.
const (
	encodingCP1252 = 1252
	encodingUTF8   = 65001
)

// The types of EXTH records that the package reads. The MOBI page of the
// MobileRead wiki lists them.
const (
	exthAuthor      = 100
	exthPublisher   = 101
	exthISBN        = 104
	exthSubject     = 105
	exthPublished   = 106
	exthASIN        = 113
	exthBoundary    = 121
	exthCover       = 201
	exthThumbnail   = 202
	exthCreator     = 204
	exthMajor       = 205
	exthMinor       = 206
	exthBuild       = 207
	exthUpdated     = 502
	exthTitle       = 503
	exthLanguage    = 524
	exthCoverURI    = 129
	exthFlagPresent = 0x40
)

// none is the value of an index that a header does not give.
const none = 0xffffffff

// header is record 0 of a book, or the record 0 of the KF8 part of a joint
// file: the header of PalmDOC, the MOBI header and the EXTH records. Each
// record number is the number in the database, with the base added. A
// record number that the header does not give is -1.
type header struct {
	// base is the number of the record of the header.
	base int

	compression int
	textLength  int64
	textRecords int
	encryption  int

	// mobi is true when the record holds a MOBI header. A PalmDOC book has
	// none, and the fields below are zero.
	mobi     bool
	encoding uint32
	version  uint32
	locale   uint32
	// fullName is the full name of the book, in the encoding of the text.
	fullName []byte

	firstImage int
	huffRecord int
	huffCount  int
	extraFlags uint16
	// fdst is the record of the FDST, and fdstCount the number of flows
	// that the header gives. Only a KF8 header gives them.
	fdst      int
	fdstCount int
	skeleton  int
	fragment  int

	exth []exthRecord
}

// exthRecord is one EXTH record.
type exthRecord struct {
	typ  uint32
	data []byte
}

// The offsets of the fields of record 0. The MOBI header starts at 16, and
// the offsets count from the start of the record.
const (
	offCompression = 0
	offTextLength  = 4
	offTextRecords = 8
	offEncryption  = 12
	offMOBI        = 16
	offHeaderLen   = 20
	offEncoding    = 28
	offVersion     = 36
	offFullName    = 84
	offFullNameLen = 88
	offLocale      = 92
	offFirstImage  = 108
	offHuffRecord  = 112
	offHuffCount   = 116
	offEXTHFlags   = 128
	offFDST        = 192
	offFDSTCount   = 196
	offExtraFlags  = 242
	offFragment    = 248
	offSkeleton    = 252
)

// readHeader reads the header in record base of db.
func readHeader(db *database, base int, lim ivy.Limits) (*header, error) {
	rec, err := db.record(base)
	if err != nil {
		return nil, err
	}
	if len(rec) < offMOBI {
		return nil, fmt.Errorf("record %d of %d bytes, too short for a header: %w", base, len(rec), ivy.ErrFormat)
	}
	h := &header{
		base:        base,
		compression: int(be16(rec, offCompression)),
		textLength:  int64(be32(rec, offTextLength)),
		textRecords: int(be16(rec, offTextRecords)),
		encryption:  int(be16(rec, offEncryption)),
		firstImage:  -1,
		huffRecord:  -1,
		fdst:        -1,
		skeleton:    -1,
		fragment:    -1,
	}
	if db.kind == kindPalmDOC {
		// PalmDOC keeps the position of the reader at offset 12, not the
		// encryption
		h.encryption = 0
		return h, nil
	}
	if len(rec) < offHeaderLen+4 || string(rec[offMOBI:offMOBI+4]) != "MOBI" {
		return nil, fmt.Errorf("record %d without a mobi header: %w", base, ivy.ErrFormat)
	}
	h.mobi = true
	length := int(be32(rec, offHeaderLen))
	// end is the end of the MOBI header in the record. A field past it is
	// not given.
	end := min(offMOBI+length, len(rec))
	field := func(off int) uint32 {
		if off+4 > end {
			return none
		}
		return be32(rec, off)
	}
	index := func(off int) int {
		v := field(off)
		if v == none {
			return -1
		}
		return base + int(v)
	}
	h.encoding = field(offEncoding)
	h.version = field(offVersion)
	h.locale = field(offLocale)
	h.firstImage = index(offFirstImage)
	if v := field(offHuffCount); v != none && v > 0 {
		h.huffRecord = index(offHuffRecord)
		h.huffCount = int(min(v, uint32(db.count())))
	}
	if off, n := field(offFullName), field(offFullNameLen); off != none && n != none && uint64(off)+uint64(n) <= uint64(len(rec)) {
		h.fullName = rec[off : off+n]
	}
	// The extra data flags hold for a header of at least 0xE4 bytes.
	if length >= 0xe4 && offExtraFlags+2 <= end {
		h.extraFlags = be16(rec, offExtraFlags)
	}
	if h.version >= 8 && h.version != none {
		h.fdst = index(offFDST)
		if v := field(offFDSTCount); v != none {
			h.fdstCount = int(v)
		}
		h.fragment = index(offFragment)
		h.skeleton = index(offSkeleton)
	}
	if field(offEXTHFlags)&exthFlagPresent != 0 && field(offEXTHFlags) != none {
		h.exth, err = readEXTH(rec, offMOBI+length, lim)
		if err != nil {
			return nil, err
		}
	}
	return h, nil
}

// readEXTH reads the EXTH records at off in rec. The block starts with
// "EXTH", its length and the number of records. Each record holds its type
// and its length, which counts the 8 bytes of the two, then its data.
func readEXTH(rec []byte, off int, lim ivy.Limits) ([]exthRecord, error) {
	if off < 0 || off+12 > len(rec) || string(rec[off:off+4]) != "EXTH" {
		// a missing EXTH block is not fatal, because the book can still be
		// read
		return nil, nil
	}
	count := int(be32(rec, off+8))
	if count > lim.MaxObjects {
		return nil, fmt.Errorf("exth of %d records: %w", count, ivy.ErrLimit)
	}
	var records []exthRecord
	p := off + 12
	for range count {
		if p+8 > len(rec) {
			break
		}
		typ, n := be32(rec, p), int64(be32(rec, p+4))
		if n < 8 || int64(p)+n > int64(len(rec)) {
			break
		}
		records = append(records, exthRecord{typ: typ, data: rec[p+8 : p+int(n)]})
		p += int(n)
	}
	return records, nil
}

// exthData returns the data of the first EXTH record of the type typ.
func (h *header) exthData(typ uint32) ([]byte, bool) {
	for _, r := range h.exth {
		if r.typ == typ {
			return r.data, true
		}
	}
	return nil, false
}

// exthInt returns the number in the first EXTH record of the type typ.
func (h *header) exthInt(typ uint32) (int, bool) {
	data, ok := h.exthData(typ)
	if !ok || len(data) == 0 || len(data) > 4 {
		return 0, false
	}
	var v uint32
	for _, b := range data {
		v = v<<8 | uint32(b)
	}
	return int(v), true
}

// exthStrings returns the text of each EXTH record of the type typ.
func (h *header) exthStrings(typ uint32) []string {
	var out []string
	for _, r := range h.exth {
		if r.typ == typ {
			if s := strings.TrimSpace(h.decode(r.data)); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// exthString returns the text of the first EXTH record of the type typ.
func (h *header) exthString(typ uint32) string {
	if s := h.exthStrings(typ); len(s) > 0 {
		return s[0]
	}
	return ""
}

// decode returns text of the book as UTF-8. A book of UTF-8 gives
// U+FFFD for a byte that is not valid, and any other book is windows-1252.
func (h *header) decode(buf []byte) string {
	if h.encoding == encodingUTF8 {
		if utf8.Valid(buf) {
			return string(buf)
		}
		return strings.ToValidUTF8(string(buf), "\uFFFD")
	}
	return ebook.Windows1252(buf)
}

// info returns the metadata of the header.
func (h *header) info(db *database) ivy.Info {
	info := ivy.Info{
		Title:      h.exthString(exthTitle),
		Authors:    h.exthStrings(exthAuthor),
		Subject:    strings.Join(h.exthStrings(exthSubject), ", "),
		Language:   h.exthString(exthLanguage),
		Created:    ebook.ParseDate(h.exthString(exthPublished)),
		Modified:   ebook.ParseDate(h.exthString(exthUpdated)),
		Producer:   h.producer(),
		Publisher:  h.exthString(exthPublisher),
		Identifier: h.exthString(exthISBN),
	}
	if info.Identifier == "" {
		info.Identifier = h.exthString(exthASIN)
	}
	if info.Title == "" {
		info.Title = strings.TrimSpace(h.decode(h.fullName))
	}
	if info.Title == "" {
		info.Title = strings.TrimSpace(ebook.Windows1252(db.name))
	}
	if info.Language == "" && h.locale != none {
		info.Language = ebook.LCIDLanguage(h.locale)
	}
	return info
}

// creators are the programs that EXTH record 204 names.
var creators = map[int]string{
	1:   "mobigen",
	2:   "Mobipocket Creator",
	200: "kindlegen (Windows)",
	201: "kindlegen (Linux)",
	202: "kindlegen (Mac)",
}

// producer returns the program that made the book, from the EXTH records
// 204 to 207, such as "kindlegen (Linux) 2.9.0".
func (h *header) producer() string {
	id, ok := h.exthInt(exthCreator)
	if !ok {
		return ""
	}
	name, ok := creators[id]
	if !ok {
		name = "creator " + strconv.Itoa(id)
	}
	var parts []string
	for _, typ := range []uint32{exthMajor, exthMinor, exthBuild} {
		v, ok := h.exthInt(typ)
		if !ok {
			break
		}
		parts = append(parts, strconv.Itoa(v))
	}
	if len(parts) == 0 {
		return name
	}
	return name + " " + strings.Join(parts, ".")
}

// be16 returns the big endian number of 2 bytes at off in b.
func be16(b []byte, off int) uint16 {
	return binary.BigEndian.Uint16(b[off:])
}

// be32 returns the big endian number of 4 bytes at off in b.
func be32(b []byte, off int) uint32 {
	return binary.BigEndian.Uint32(b[off:])
}
