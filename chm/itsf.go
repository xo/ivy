package chm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/xo/magic/compress/lzx"

	"github.com/xo/ivy"
)

// The parts of the ITSF header. It starts with "ITSF" and its version, 2 or
// 3, then the length of the header at 8, and the LCID at 0x14. The offset
// and the length of the directory are at 0x48 and 0x50, in 8 bytes each.
// Version 3 gives the offset of the content at 0x58. In version 2, the
// content follows the directory.
const (
	itsfMinSize   = 0x58
	itsfV3Size    = 0x60
	itsfLength    = 8
	itsfLCID      = 0x14
	itsfDirOffset = 0x48
	itsfDirLength = 0x50
	itsfContent   = 0x58
)

// The parts of the ITSP header of the directory. It starts with "ITSP",
// then the length of the header at 8, the size of a chunk at 0x10 and the
// number of chunks at 0x2C. The chunks follow the header.
const (
	itspMinSize   = 0x30
	itspLength    = 8
	itspChunkSize = 0x10
	itspChunks    = 0x2c
	// minChunkSize and maxChunkSize bound the size of a chunk. A CHM file
	// of Microsoft has chunks of 4096 bytes.
	minChunkSize = 0x20
	maxChunkSize = 1 << 16
	// pmglHeader is the size of the header of a PMGL chunk: "PMGL", the
	// length of the free space at its end, and three more fields.
	pmglHeader = 0x14
)

// The sections of the content. Section 0 is not compressed, and section 1,
// which the ITSF format names MSCompressed, LZX compresses.
const (
	sectionPlain      = 0
	sectionCompressed = 1
)

// The names of the files of section 0 that describe section 1.
const (
	storage     = "::DataSpace/Storage/MSCompressed/"
	controlName = storage + "ControlData"
	contentName = storage + "Content"
	resetName   = storage + "Transform/{7FC28940-9D31-11D0-9B27-00A0C91E9C7C}/InstanceData/ResetTable"
)

// maxResetInterval is the largest number of frames from one reset of LZX to
// the next. A read of a file decodes up to that many frames that it does
// not return, so the limit bounds the work of a crafted file. A CHM file of
// Microsoft resets every 2 frames.
const maxResetInterval = 256

// entry is a file of the directory.
type entry struct {
	section        int
	offset, length int64
}

// archive is the content of a CHM file: its directory, and the reader of
// its two sections.
type archive struct {
	src ivy.Source
	lim ivy.Limits
	// lcid is the LCID of the ITSF header.
	lcid uint32
	// content is the offset of section 0 in the file.
	content int64
	// names are the names of the files, in the order of the directory, and
	// entries holds the files by name.
	names   []string
	entries map[string]entry

	// mu guards section 1, which decodes one read at a time.
	mu     sync.Mutex
	lzx    *lzx.Section
	lzxErr error
}

// readArchive reads the ITSF header and the directory of a CHM file.
func readArchive(src ivy.Source, lim ivy.Limits) (*archive, error) {
	size := src.Size()
	if size < itsfMinSize {
		return nil, fmt.Errorf("chm of %d bytes: %w", size, ivy.ErrFormat)
	}
	head := make([]byte, min(size, itsfV3Size))
	if _, err := src.ReadAt(head, 0); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading the header of chm: %w", err)
	}
	if string(head[:4]) != "ITSF" {
		return nil, fmt.Errorf("chm without its ITSF header: %w", ivy.ErrFormat)
	}
	version := le32(head, 4)
	if version < 2 || version > 3 {
		return nil, fmt.Errorf("chm of version %d: %w", version, ivy.ErrUnsupported)
	}
	a := &archive{src: src, lim: lim, lcid: le32(head, itsfLCID), entries: make(map[string]entry)}
	dirOff, dirLen := le64(head, itsfDirOffset), le64(head, itsfDirLength)
	if dirOff < 0 || dirLen < itspMinSize || dirOff > size || dirLen > size-dirOff {
		return nil, fmt.Errorf("chm with its directory at %d and %d bytes: %w", dirOff, dirLen, ivy.ErrFormat)
	}
	a.content = dirOff + dirLen
	if version >= 3 && le32(head, itsfLength) >= itsfV3Size && len(head) >= itsfV3Size {
		a.content = le64(head, itsfContent)
	}
	if a.content < 0 || a.content > size {
		return nil, fmt.Errorf("chm with its content at %d: %w", a.content, ivy.ErrFormat)
	}
	if dirLen > lim.MaxMemory {
		return nil, fmt.Errorf("chm directory of %d bytes: %w", dirLen, ivy.ErrLimit)
	}
	dir := make([]byte, dirLen)
	if _, err := src.ReadAt(dir, dirOff); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading the directory of chm: %w", err)
	}
	if err := a.readDirectory(dir); err != nil {
		return nil, err
	}
	return a, nil
}

// readDirectory reads the PMGL chunks of the directory, which list the
// files. It skips the PMGI chunks, which index the PMGL chunks, because it
// reads each PMGL chunk.
func (a *archive) readDirectory(dir []byte) error {
	if string(dir[:4]) != "ITSP" {
		return fmt.Errorf("chm directory without its ITSP header: %w", ivy.ErrFormat)
	}
	start, chunkSize := int64(le32(dir, itspLength)), int64(le32(dir, itspChunkSize))
	if start < itspMinSize || start > int64(len(dir)) || chunkSize < minChunkSize || chunkSize > maxChunkSize {
		return fmt.Errorf("chm directory with a header of %d bytes and chunks of %d: %w", start, chunkSize, ivy.ErrFormat)
	}
	count := min(int64(le32(dir, itspChunks)), (int64(len(dir))-start)/chunkSize)
	for i := range count {
		chunk := dir[start+i*chunkSize : start+(i+1)*chunkSize]
		if string(chunk[:4]) != "PMGL" {
			continue
		}
		if err := a.readChunk(chunk); err != nil {
			return fmt.Errorf("chunk %d of the chm directory: %w", i, err)
		}
	}
	if len(a.names) == 0 {
		return fmt.Errorf("chm directory without a file: %w", ivy.ErrFormat)
	}
	return nil
}

// readChunk reads the entries of a PMGL chunk. Each entry gives the length
// of the name, the name in UTF-8, the section, the offset in the section
// and the length, each number as an ENCINT. The entries end at the free
// space, whose length the header gives.
func (a *archive) readChunk(chunk []byte) error {
	free := int(le32(chunk, 4))
	end := len(chunk) - free
	if free < 0 || end < pmglHeader {
		end = len(chunk)
	}
	p := pmglHeader
	for p < end {
		var n, section, offset, length int64
		var ok bool
		if n, p, ok = encint(chunk, p, end); !ok || n <= 0 || n > int64(end-p) {
			return nil
		}
		name := string(chunk[p : p+int(n)])
		p += int(n)
		if section, p, ok = encint(chunk, p, end); !ok {
			return nil
		}
		if offset, p, ok = encint(chunk, p, end); !ok {
			return nil
		}
		if length, p, ok = encint(chunk, p, end); !ok {
			return nil
		}
		if _, dup := a.entries[name]; dup {
			continue
		}
		if len(a.names) >= a.lim.MaxObjects {
			return fmt.Errorf("chm of more than %d files: %w", a.lim.MaxObjects, ivy.ErrLimit)
		}
		a.names = append(a.names, name)
		a.entries[name] = entry{section: int(min(section, 1<<16)), offset: offset, length: length}
	}
	return nil
}

// encint reads an ENCINT at p in b, before end: 7 bits a byte, the high
// byte first, and each byte but the last has the bit 0x80 set. It returns
// the number, the position after it, and false if there is no number or if
// it does not fit in 62 bits.
func encint(b []byte, p, end int) (int64, int, bool) {
	var v int64
	for p < end {
		c := b[p]
		p++
		v = v<<7 | int64(c&0x7f)
		if v >= 1<<55 {
			return 0, p, false
		}
		if c&0x80 == 0 {
			return v, p, true
		}
	}
	return 0, p, false
}

// read reads the file name. It returns an error that wraps ivy.ErrLimit if
// the file is larger than Limits.MaxMemory.
func (a *archive) read(name string) ([]byte, error) {
	e, ok := a.entries[name]
	if !ok {
		return nil, fmt.Errorf("chm without the file %s: %w", name, ivy.ErrFormat)
	}
	if e.length > a.lim.MaxMemory {
		return nil, fmt.Errorf("chm file %s of %d bytes: %w", name, e.length, ivy.ErrLimit)
	}
	switch e.section {
	case sectionPlain:
		return a.readPlain(name, e)
	case sectionCompressed:
		return a.readCompressed(name, e)
	}
	return nil, fmt.Errorf("chm file %s in section %d: %w", name, e.section, ivy.ErrUnsupported)
}

// readPlain reads a file of section 0.
func (a *archive) readPlain(name string, e entry) ([]byte, error) {
	size := a.src.Size()
	if e.offset > size-a.content || e.length > size-a.content-e.offset {
		return nil, fmt.Errorf("chm file %s at %d of %d bytes, past the end: %w", name, e.offset, e.length, ivy.ErrFormat)
	}
	buf := make([]byte, e.length)
	if _, err := a.src.ReadAt(buf, a.content+e.offset); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading chm file %s: %w", name, err)
	}
	return buf, nil
}

// readCompressed reads a file of section 1.
func (a *archive) readCompressed(name string, e entry) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.section()
	if err != nil {
		return nil, err
	}
	if e.offset > s.Length || e.length > s.Length-e.offset {
		return nil, fmt.Errorf("chm file %s at %d of %d bytes, past the end of section 1: %w", name, e.offset, e.length, ivy.ErrFormat)
	}
	buf, err := s.Decode(make([]byte, 0, e.length), e.offset, int(e.length))
	if err != nil {
		return nil, fmt.Errorf("decoding chm file %s: %w: %w", name, ivy.ErrFormat, err)
	}
	return buf, nil
}

// section returns the reader of section 1. It reads the control data, the
// reset table and the compressed data the first time. The caller holds mu.
//
// The control data holds the number of its DWORDs, "LZXC", the version, the
// reset interval and the size of the window, in 4 bytes each. Version 2
// gives the two in units of 32768 bytes, and version 1 gives them in bytes.
//
// The reset table holds its version, the number of its entries, the size of
// an entry, the length of its header, the length of the section, the length
// of the compressed data and the size of a frame. The entries follow the
// header. Each one is the offset of a frame in the compressed data, in 8
// bytes.
func (a *archive) section() (*lzx.Section, error) {
	if a.lzx != nil || a.lzxErr != nil {
		return a.lzx, a.lzxErr
	}
	a.lzx, a.lzxErr = a.readSection()
	return a.lzx, a.lzxErr
}

// readSection reads the reader of section 1.
func (a *archive) readSection() (*lzx.Section, error) {
	for _, name := range []string{controlName, resetName, contentName} {
		if e, ok := a.entries[name]; !ok || e.section != sectionPlain {
			return nil, fmt.Errorf("chm without %s in section 0: %w", name, ivy.ErrFormat)
		}
	}
	control, err := a.read(controlName)
	if err != nil {
		return nil, err
	}
	if len(control) < 24 || string(control[4:8]) != "LZXC" {
		return nil, fmt.Errorf("chm with its control data of %d bytes: %w", len(control), ivy.ErrFormat)
	}
	version, interval, window := le32(control, 8), int64(le32(control, 12)), int64(le32(control, 16))
	switch version {
	case 1:
		interval /= lzx.FrameSize
	case 2:
		window *= lzx.FrameSize
	default:
		return nil, fmt.Errorf("chm with lzxc of version %d: %w", version, ivy.ErrUnsupported)
	}
	switch {
	case interval < 1:
		return nil, fmt.Errorf("chm with an lzx reset interval of %d frames: %w", interval, ivy.ErrFormat)
	case interval > maxResetInterval:
		return nil, fmt.Errorf("chm with an lzx reset interval of %d frames: %w", interval, ivy.ErrLimit)
	}
	table, err := a.read(resetName)
	if err != nil {
		return nil, err
	}
	if len(table) < 0x28 {
		return nil, fmt.Errorf("chm with a reset table of %d bytes: %w", len(table), ivy.ErrFormat)
	}
	n, size, hdr := int64(le32(table, 4)), int64(le32(table, 8)), int64(le32(table, 12))
	if size != 8 || hdr < 0x28 || hdr > int64(len(table)) || n > (int64(len(table))-hdr)/8 {
		return nil, fmt.Errorf("chm with a reset table of %d entries of %d bytes: %w", n, size, ivy.ErrFormat)
	}
	s := &lzx.Section{
		WindowSize:    int(min(window, 1<<30)),
		ResetInterval: int(interval),
		Length:        le64(table, 16),
		ResetTable:    make([]uint64, n),
	}
	if s.Length < 0 {
		return nil, fmt.Errorf("chm with section 1 of %d bytes: %w", s.Length, ivy.ErrFormat)
	}
	for i := range s.ResetTable {
		s.ResetTable[i] = binary.LittleEndian.Uint64(table[hdr+8*int64(i):])
	}
	if s.Data, err = a.read(contentName); err != nil {
		return nil, err
	}
	return s, nil
}

// files returns the names of the files of the archive that a document can
// link to, in the order of the directory. It leaves out the folders, the
// files of the system, whose names start with "#" or "$", and the files of
// the storage, whose names start with "::".
func (a *archive) files() []string {
	var out []string
	for _, name := range a.names {
		rest, ok := strings.CutPrefix(name, "/")
		if !ok || rest == "" || strings.HasSuffix(rest, "/") || rest[0] == '#' || rest[0] == '$' {
			continue
		}
		out = append(out, name)
	}
	return out
}

// le32 returns the little endian number of 4 bytes at off in b.
func le32(b []byte, off int) uint32 {
	return binary.LittleEndian.Uint32(b[off:])
}

// le64 returns the little endian number of 8 bytes at off in b, as an
// int64. A number past the range of int64 is negative.
func le64(b []byte, off int) int64 {
	return int64(binary.LittleEndian.Uint64(b[off:]))
}
