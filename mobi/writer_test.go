package mobi

import (
	"bytes"
	"encoding/binary"
)

// This file writes MOBI and PalmDOC books for the tests, as the MOBI page
// of the MobileRead wiki describes them.

// testBook is a book of the tests.
type testBook struct {
	// kind is the type and the creator of the database. Empty is BOOKMOBI.
	kind        string
	compression int
	encryption  int
	// encoding is the text encoding. Zero is UTF-8.
	encoding uint32
	// version is the version of the MOBI header. Zero is 6.
	version    uint32
	locale     uint32
	extraFlags uint16
	fullName   string
	// text is the text of the book. The writer compresses it in records
	// of 4096 bytes. records, if it is not nil, holds the text records
	// instead, as they are.
	text    []byte
	records [][]byte
	// huff holds the HUFF record and the CDIC records.
	huff   [][]byte
	images [][]byte
	exth   []exthRecord
}

// trailer is the trailing entries that the writer adds to each text record
// for the extra data flags: for bit 0, one byte of a character that the
// next record continues, and for bit 1, an entry of 3 bytes.
func trailer(flags uint16) []byte {
	var out []byte
	if flags&1 != 0 {
		out = append(out, 'X', 0x01)
	}
	if flags&2 != 0 {
		out = append(out, 0x05, 0x06, 0x83)
	}
	return out
}

// build returns the book as a PalmDB file.
func (b testBook) build() []byte {
	if b.kind == "" {
		b.kind = kindMOBI
	}
	if b.encoding == 0 {
		b.encoding = encodingUTF8
	}
	if b.version == 0 {
		b.version = 6
	}
	if b.compression == 0 {
		b.compression = compressionPalmDoc
	}
	records := b.records
	if records == nil {
		for i := 0; i < len(b.text); i += 4096 {
			chunk := b.text[i:min(i+4096, len(b.text))]
			if b.compression == compressionPalmDoc {
				chunk = palmDocEncode(chunk)
			}
			records = append(records, chunk)
		}
	}
	text := make([][]byte, len(records))
	for i, r := range records {
		text[i] = append(bytes.Clone(r), trailer(b.extraFlags)...)
	}
	all := make([][]byte, 0, 2+len(text)+len(b.huff)+len(b.images))
	all = append(all, nil)
	all = append(all, text...)
	huffRecord := len(all)
	all = append(all, b.huff...)
	firstImage := len(all)
	all = append(all, b.images...)
	all = append(all, []byte("\xe9\x8e\r\n"))

	be16w := binary.BigEndian.AppendUint16
	be32w := binary.BigEndian.AppendUint32
	rec0 := be16w(nil, uint16(b.compression))
	rec0 = be16w(rec0, 0)
	rec0 = be32w(rec0, uint32(len(b.text)))
	rec0 = be16w(rec0, uint16(len(text)))
	rec0 = be16w(rec0, 4096)
	rec0 = be16w(rec0, uint16(b.encryption))
	rec0 = be16w(rec0, 0)
	if b.kind == kindMOBI {
		const length = 232
		h := make([]byte, length)
		for i := 0; i < length; i += 4 {
			binary.BigEndian.PutUint32(h[i:], none)
		}
		put := func(off int, v uint32) { binary.BigEndian.PutUint32(h[off-offMOBI:], v) }
		copy(h, "MOBI")
		put(offHeaderLen, length)
		put(24, 2)
		put(offEncoding, b.encoding)
		put(32, 1234)
		put(offVersion, b.version)
		put(80, uint32(firstImage))
		put(offLocale, b.locale)
		put(104, b.version)
		put(offFirstImage, uint32(firstImage))
		put(offHuffRecord, uint32(huffRecord))
		put(offHuffCount, uint32(len(b.huff)))
		put(120, 0)
		put(124, 0)
		put(offEXTHFlags, 0)
		if len(b.exth) > 0 {
			put(offEXTHFlags, exthFlagPresent)
		}
		put(offExtraFlags-2, uint32(b.extraFlags))
		var exth []byte
		if len(b.exth) > 0 {
			var recs []byte
			for _, r := range b.exth {
				recs = be32w(recs, r.typ)
				recs = be32w(recs, uint32(8+len(r.data)))
				recs = append(recs, r.data...)
			}
			exth = append([]byte("EXTH"), be32w(nil, uint32(12+len(recs)))...)
			exth = be32w(exth, uint32(len(b.exth)))
			exth = append(exth, recs...)
			for len(exth)%4 != 0 {
				exth = append(exth, 0)
			}
		}
		nameOff := len(rec0) + length + len(exth)
		put(offFullName, uint32(nameOff))
		put(offFullNameLen, uint32(len(b.fullName)))
		rec0 = append(rec0, h...)
		rec0 = append(rec0, exth...)
		rec0 = append(rec0, b.fullName...)
		rec0 = append(rec0, 0, 0)
	}
	all[0] = rec0

	head := make([]byte, pdbHeaderSize)
	copy(head, "Test Database")
	copy(head[pdbKindOffset:], b.kind)
	binary.BigEndian.PutUint16(head[pdbCountAt:], uint16(len(all)))
	off := pdbHeaderSize + pdbEntrySize*len(all) + 2
	out := head
	for i, r := range all {
		out = be32w(out, uint32(off))
		out = be32w(out, uint32(i))
		off += len(r)
	}
	out = append(out, 0, 0)
	for _, r := range all {
		out = append(out, r...)
	}
	return out
}

// exthText returns an EXTH record of text.
func exthText(typ uint32, s string) exthRecord {
	return exthRecord{typ: typ, data: []byte(s)}
}

// exthNumber returns an EXTH record of a number of 4 bytes.
func exthNumber(typ, v uint32) exthRecord {
	return exthRecord{typ: typ, data: binary.BigEndian.AppendUint32(nil, v)}
}

// palmDocEncode compresses data with PalmDoc: a copy of 3 to 10 bytes from
// up to 2047 bytes back, a space and a letter in one byte, or bytes as they
// are.
func palmDocEncode(data []byte) []byte {
	var out []byte
	for i := 0; i < len(data); {
		best, dist := 0, 0
		for d := 1; d <= min(i, 2047); d++ {
			n := 0
			for n < 10 && i+n < len(data) && data[i+n] == data[i-d+n] {
				n++
			}
			if n > best {
				best, dist = n, d
			}
		}
		switch c := data[i]; {
		case best >= 3:
			v := 0x8000 | dist<<3 | (best - 3)
			out = append(out, byte(v>>8), byte(v))
			i += best
		case c == ' ' && i+1 < len(data) && data[i+1] >= 0x40 && data[i+1] < 0x80:
			out = append(out, data[i+1]^0x80)
			i += 2
		case c == 0 || c >= 0x09 && c < 0x80:
			out = append(out, c)
			i++
		default:
			out = append(out, 1, c)
			i++
		}
	}
	return out
}

// huffBook returns the HUFF record and the CDIC record of a book whose
// codes are all of 8 bits. Code c stands for phrase 255-c, and phrases maps
// a code to its phrase. A code that phrases leaves out stands for its own
// byte. compressed lists the codes whose phrases are themselves codes.
func huffBook(phrases map[byte][]byte, compressed map[byte]bool) (huff, cdic []byte) {
	be16w := binary.BigEndian.AppendUint16
	be32w := binary.BigEndian.AppendUint32
	huff = append([]byte("HUFF"), be32w(nil, 24)...)
	huff = be32w(huff, 24)
	huff = be32w(huff, 24+256*4)
	huff = be32w(huff, 0)
	huff = be32w(huff, 0)
	for range 256 {
		// a code of 8 bits, final, and 255 the highest code
		huff = be32w(huff, 255<<8|0x80|8)
	}
	huff = append(huff, make([]byte, 64*4)...)
	var offsets, data []byte
	for i := range 256 {
		c := byte(255 - i)
		p, ok := phrases[c]
		if !ok {
			p = []byte{c}
		}
		offsets = be16w(offsets, uint16(2*256+len(data)))
		flag := uint16(0x8000)
		if compressed[c] {
			flag = 0
		}
		data = be16w(data, uint16(len(p))|flag)
		data = append(data, p...)
	}
	cdic = append([]byte("CDIC"), be32w(nil, 16)...)
	cdic = be32w(cdic, 256)
	cdic = be32w(cdic, 8)
	cdic = append(cdic, offsets...)
	return huff, append(cdic, data...)
}
