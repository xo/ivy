package mobi

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/xo/ivy"
)

// The parts of the header of a PalmDB file. The header holds the name of
// the database in 32 bytes, its type and its creator at 60, and the number
// of records at 76. The list of records follows at 78, with 8 bytes for
// each record: its offset in the file, in 4 bytes, then its attributes and
// its unique id.
const (
	pdbHeaderSize = 78
	pdbNameSize   = 32
	pdbKindOffset = 60
	pdbCountAt    = 76
	pdbEntrySize  = 8
)

// The types and creators of the databases that the package reads.
const (
	kindMOBI    = "BOOKMOBI"
	kindPalmDOC = "TEXtREAd"
)

// database is a PalmDB file: a list of records.
type database struct {
	src ivy.Source
	lim ivy.Limits
	// name is the name of the database, without its padding.
	name []byte
	// kind is the type and the creator, such as "BOOKMOBI".
	kind string
	// offsets holds the start of each record, and the size of the file
	// last. Record i is from offsets[i] to offsets[i+1].
	offsets []int64
}

// readDatabase reads the header and the list of records of a PalmDB file.
func readDatabase(src ivy.Source, lim ivy.Limits) (*database, error) {
	size := src.Size()
	if size < pdbHeaderSize {
		return nil, fmt.Errorf("palmdb of %d bytes: %w", size, ivy.ErrFormat)
	}
	var head [pdbHeaderSize]byte
	if _, err := src.ReadAt(head[:], 0); err != nil {
		return nil, fmt.Errorf("reading the header of palmdb: %w", err)
	}
	db := &database{src: src, lim: lim, kind: string(head[pdbKindOffset : pdbKindOffset+8])}
	if db.kind != kindMOBI && db.kind != kindPalmDOC {
		return nil, fmt.Errorf("palmdb of type %q: %w", db.kind, ivy.ErrFormat)
	}
	name := head[:pdbNameSize]
	if i := bytes.IndexByte(name, 0); i >= 0 {
		name = name[:i]
	}
	db.name = bytes.Clone(name)
	count := int(binary.BigEndian.Uint16(head[pdbCountAt:]))
	switch {
	case count == 0:
		return nil, fmt.Errorf("palmdb without a record: %w", ivy.ErrFormat)
	case count > lim.MaxObjects:
		return nil, fmt.Errorf("palmdb of %d records: %w", count, ivy.ErrLimit)
	case pdbHeaderSize+int64(count)*pdbEntrySize > size:
		return nil, fmt.Errorf("palmdb of %d records in %d bytes: %w", count, size, ivy.ErrFormat)
	}
	list := make([]byte, count*pdbEntrySize)
	if _, err := src.ReadAt(list, pdbHeaderSize); err != nil {
		return nil, fmt.Errorf("reading the records of palmdb: %w", err)
	}
	db.offsets = make([]int64, count+1)
	for i := range count {
		off := int64(binary.BigEndian.Uint32(list[i*pdbEntrySize:]))
		if off > size || i > 0 && off < db.offsets[i-1] {
			return nil, fmt.Errorf("record %d of palmdb at offset %d: %w", i, off, ivy.ErrFormat)
		}
		db.offsets[i] = off
	}
	db.offsets[count] = size
	return db, nil
}

// count returns the number of records.
func (db *database) count() int {
	return len(db.offsets) - 1
}

// size returns the size of record i, or 0 if there is no record i.
func (db *database) size(i int) int64 {
	if i < 0 || i >= db.count() {
		return 0
	}
	return db.offsets[i+1] - db.offsets[i]
}

// record reads record i. It returns an error that wraps ivy.ErrFormat if
// there is no record i, and one that wraps ivy.ErrLimit if the record is
// larger than Limits.MaxMemory.
func (db *database) record(i int) ([]byte, error) {
	if i < 0 || i >= db.count() {
		return nil, fmt.Errorf("record %d of %d: %w", i, db.count(), ivy.ErrFormat)
	}
	n := db.size(i)
	if n > db.lim.MaxMemory {
		return nil, fmt.Errorf("record %d of %d bytes: %w", i, n, ivy.ErrLimit)
	}
	buf := make([]byte, n)
	if _, err := db.src.ReadAt(buf, db.offsets[i]); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading record %d: %w", i, err)
	}
	return buf, nil
}

// head returns up to the first n bytes of record i, or nil if there is no
// record i.
func (db *database) head(i, n int) []byte {
	n = int(min(int64(n), db.size(i)))
	if n <= 0 {
		return nil
	}
	buf := make([]byte, n)
	if _, err := db.src.ReadAt(buf, db.offsets[i]); err != nil && !errors.Is(err, io.EOF) {
		return nil
	}
	return buf
}
