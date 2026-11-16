package mobi

import (
	"errors"
	"fmt"

	"github.com/xo/magic/compress/huffcdic"
	"github.com/xo/magic/compress/palmdoc"

	"github.com/xo/ivy"
)

// maxTrailing is the number of trailing entries that the extra data flags
// can name: one for each of their 16 bits.
const maxTrailing = 16

// text decodes the text records of the book of h, and returns the text, in
// the encoding of the book. The text is at most Limits.MaxMemory bytes.
func (d *Document) text(h *header) ([]byte, error) {
	limit := d.lim.MaxMemory
	if h.textRecords == 0 {
		return nil, fmt.Errorf("book without a text record: %w", ivy.ErrFormat)
	}
	if h.base+h.textRecords >= d.db.count() {
		return nil, fmt.Errorf("book of %d text records and %d records: %w", h.textRecords, d.db.count(), ivy.ErrFormat)
	}
	var decode func(dst, src []byte, limit int) ([]byte, error)
	switch h.compression {
	case compressionNone:
		decode = func(dst, src []byte, limit int) ([]byte, error) {
			if len(src) > limit {
				return append(dst, src[:limit]...), ivy.ErrLimit
			}
			return append(dst, src...), nil
		}
	case compressionPalmDoc:
		decode = palmdoc.Decode
	case compressionHuffCDIC:
		dec, err := d.huffman(h)
		if err != nil {
			return nil, err
		}
		decode = dec.Decode
	default:
		return nil, fmt.Errorf("text of compression %d: %w", h.compression, ivy.ErrUnsupported)
	}
	// the text length of the header is only a hint for the first
	// allocation, because a crafted file can give any length
	out := make([]byte, 0, min(h.textLength, limit, 16<<20))
	for i := 1; i <= h.textRecords; i++ {
		rec, err := d.db.record(h.base + i)
		if err != nil {
			return nil, err
		}
		rec = trimTrailing(rec, h.extraFlags)
		out, err = decode(out, rec, int(limit-int64(len(out))))
		switch {
		case errors.Is(err, ivy.ErrLimit), errors.Is(err, palmdoc.ErrLimit), errors.Is(err, huffcdic.ErrLimit):
			return nil, fmt.Errorf("text of more than %d bytes: %w", limit, ivy.ErrLimit)
		case err != nil:
			// a damaged record keeps the text that it decoded before the
			// damage, as a reader of ebooks does
			d.log.Warn("mobi: damaged text record", "record", h.base+i, "err", err)
		}
	}
	return out, nil
}

// huffman reads the HUFF record and the CDIC records of the book of h.
func (d *Document) huffman(h *header) (*huffcdic.Decoder, error) {
	if h.huffRecord < 0 || h.huffCount < 2 || h.huffRecord+h.huffCount > d.db.count() {
		return nil, fmt.Errorf("huff/cdic book without its huff and cdic records: %w", ivy.ErrFormat)
	}
	huff, err := d.db.record(h.huffRecord)
	if err != nil {
		return nil, err
	}
	cdics := make([][]byte, 0, h.huffCount-1)
	var total int64
	for i := 1; i < h.huffCount; i++ {
		rec, err := d.db.record(h.huffRecord + i)
		if err != nil {
			return nil, err
		}
		if total += int64(len(rec)); total > d.lim.MaxMemory {
			return nil, fmt.Errorf("cdic records of more than %d bytes: %w", d.lim.MaxMemory, ivy.ErrLimit)
		}
		cdics = append(cdics, rec)
	}
	dec, err := huffcdic.New(huff, cdics)
	if err != nil {
		return nil, fmt.Errorf("reading the huff and cdic records: %w: %w", ivy.ErrFormat, err)
	}
	return dec, nil
}

// trimTrailing removes the trailing entries from the end of a text record.
// Each bit of flags from bit 1 names an entry that ends with its size, as a
// number of 7 bits a byte that is read from the end, and the entries follow
// the text in the order of the bits. Bit 0 names the bytes of a character
// that the next record continues: the last byte of the entry gives their
// number in its low 2 bits.
func trimTrailing(rec []byte, flags uint16) []byte {
	for bit := maxTrailing - 1; bit >= 1; bit-- {
		if flags&(1<<bit) == 0 {
			continue
		}
		n := trailingSize(rec)
		if n <= 0 || n > len(rec) {
			return rec[:0]
		}
		rec = rec[:len(rec)-n]
	}
	if flags&1 != 0 && len(rec) > 0 {
		n := int(rec[len(rec)-1]&3) + 1
		if n > len(rec) {
			return rec[:0]
		}
		rec = rec[:len(rec)-n]
	}
	return rec
}

// trailingSize reads the size at the end of a trailing entry. The size is
// a number of 7 bits a byte, the high byte first, and only its first byte
// has the bit 0x80 set. It takes at most 4 bytes.
func trailingSize(rec []byte) int {
	if len(rec) == 0 {
		return 0
	}
	start := len(rec) - 1
	for start > 0 && start > len(rec)-4 && rec[start]&0x80 == 0 {
		start--
	}
	n := 0
	for _, b := range rec[start:] {
		n = n<<7 | int(b&0x7f)
	}
	return n
}
