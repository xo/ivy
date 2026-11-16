package chm

import (
	"encoding/binary"
	"slices"
	"strings"
	"unicode/utf16"
)

// This file writes CHM files for the tests. It follows the writer of the
// tests of github.com/xo/magic/compress/lzx, and adds files of HTML, #SYSTEM
// and #WINDOWS, more than one chunk of the directory, and LZX with blocks
// that are not compressed. 7z and extract_chmLib read the files that it
// writes.

// chmFile is a file of a CHM file of the tests.
type chmFile struct {
	name string
	data []byte
	// plain puts the file in section 0. The other files go in section 1.
	plain bool
}

// chmOptions are the options of a CHM file of the tests.
type chmOptions struct {
	// chunkSize is the size of a chunk of the directory. Zero is 4096.
	chunkSize int
	// interval is the reset interval of LZX, in frames. Zero is 2.
	interval int
	// lzxcVersion is the version of the control data. Zero is 2.
	lzxcVersion int
	// lcid is the LCID of the ITSF header.
	lcid uint32
	// version is the version of the ITSF header. Zero is 3.
	version int
}

// frameSize is the size of a frame of LZX.
const frameSize = 32768

// The writers of little endian numbers.
var (
	le16  = binary.LittleEndian.AppendUint16
	le32w = binary.LittleEndian.AppendUint32
	le64w = binary.LittleEndian.AppendUint64
)

// bitWriter writes the bits of LZX: 16-bit words, little endian, from the
// highest bit of each word.
type bitWriter struct {
	out []byte
	acc uint32
	n   uint
}

// write writes the low k bits of v, the highest first.
func (w *bitWriter) write(v uint32, k uint) {
	for i := int(k) - 1; i >= 0; i-- {
		w.acc = w.acc<<1 | v>>uint(i)&1
		if w.n++; w.n == 16 {
			w.out = append(w.out, byte(w.acc), byte(w.acc>>8))
			w.acc, w.n = 0, 0
		}
	}
}

// lzxStore compresses data with LZX into blocks that are not compressed,
// one block for each frame, with a reset every interval frames. It returns
// the compressed bytes of each frame.
func lzxStore(data []byte, interval int) [][]byte {
	var frames [][]byte
	for i := 0; i < len(data); i += frameSize {
		frame := data[i:min(i+frameSize, len(data))]
		var w bitWriter
		if (i/frameSize)%interval == 0 {
			// no E8 translation, after each reset
			w.write(0, 1)
		}
		w.write(3, 3)
		w.write(uint32(len(frame)), 24)
		w.write(0, 16-w.n)
		for range 3 {
			w.out = le32w(w.out, 1)
		}
		w.out = append(w.out, frame...)
		if len(w.out)%2 == 1 {
			w.out = append(w.out, 0)
		}
		frames = append(frames, w.out)
	}
	return frames
}

// utf16le returns s in UTF-16LE.
func utf16le(s string) []byte {
	var out []byte
	for _, u := range utf16.Encode([]rune(s)) {
		out = le16(out, u)
	}
	return out
}

// guid appends the GUID s, as {01234567-89AB-CDEF-0123-456789ABCDEF}, in the
// byte order of Windows.
func guid(b []byte, s string) []byte {
	hex := func(s string) []byte {
		var out []byte
		for i := 0; i < len(s); i += 2 {
			var v byte
			for _, c := range s[i : i+2] {
				v <<= 4
				if c >= '0' && c <= '9' {
					v |= byte(c - '0')
				} else {
					v |= byte(c - 'A' + 10)
				}
			}
			out = append(out, v)
		}
		return out
	}
	d1, d2, d3 := hex(s[1:9]), hex(s[10:14]), hex(s[15:19])
	b = append(b, d1[3], d1[2], d1[1], d1[0], d2[1], d2[0], d3[1], d3[0])
	b = append(b, hex(s[20:24])...)
	return append(b, hex(s[25:37])...)
}

// encintw appends v as an ENCINT.
func encintw(b []byte, v int) []byte {
	var tmp []byte
	for {
		tmp = append([]byte{byte(v & 0x7f)}, tmp...)
		if v >>= 7; v == 0 {
			break
		}
	}
	for i := range len(tmp) - 1 {
		tmp[i] |= 0x80
	}
	return append(b, tmp...)
}

// systemFile returns a #SYSTEM file with the entries of the codes.
func systemFile(entries map[uint16][]byte) []byte {
	b := le32w(nil, 3)
	codes := make([]uint16, 0, len(entries))
	for code := range entries {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	for _, code := range codes {
		b = le16(b, code)
		b = le16(b, uint16(len(entries[code])))
		b = append(b, entries[code]...)
	}
	return b
}

// cstr returns s with a NUL.
func cstr(s string) []byte {
	return append([]byte(s), 0)
}

// lcidEntry returns the data of code 4 of #SYSTEM.
func lcidEntry(lcid uint32) []byte {
	return append(le32w(nil, lcid), make([]byte, 32)...)
}

// buildCHM returns a CHM file that holds files.
func buildCHM(files []chmFile, o chmOptions) []byte {
	if o.chunkSize == 0 {
		o.chunkSize = 0x1000
	}
	if o.interval == 0 {
		o.interval = 2
	}
	if o.lzxcVersion == 0 {
		o.lzxcVersion = 2
	}
	if o.version == 0 {
		o.version = 3
	}
	type dirEntry struct {
		name      string
		section   int
		off, size int
	}
	var entries []dirEntry
	var content []byte
	for _, f := range files {
		if f.plain {
			continue
		}
		entries = append(entries, dirEntry{f.name, 1, len(content), len(f.data)})
		content = append(content, f.data...)
	}
	// 7z and extract_chmLib read section 1 only when its content fills its
	// last frame
	content = append(content, make([]byte, (frameSize-len(content)%frameSize)%frameSize)...)
	frames := lzxStore(content, o.interval)
	var compressed []byte
	offsets := make([]uint64, 0, len(frames)+1)
	for _, f := range frames {
		offsets = append(offsets, uint64(len(compressed)))
		compressed = append(compressed, f...)
	}
	offsets = append(offsets, uint64(len(compressed)))
	interval, window := uint32(o.interval), uint32(1)<<16/frameSize
	if o.lzxcVersion == 1 {
		interval, window = uint32(o.interval*frameSize), 1<<16
	}
	lzxc := le32w(nil, 6)
	lzxc = append(lzxc, "LZXC"...)
	lzxc = le32w(lzxc, uint32(o.lzxcVersion))
	lzxc = le32w(lzxc, interval)
	lzxc = le32w(lzxc, window)
	lzxc = le32w(lzxc, 1)
	lzxc = le32w(lzxc, 0)
	table := le32w(nil, 2)
	table = le32w(table, uint32(len(offsets)))
	table = le32w(table, 8)
	table = le32w(table, 0x28)
	table = le64w(table, uint64(len(content)))
	table = le64w(table, uint64(len(compressed)))
	table = le64w(table, frameSize)
	for _, off := range offsets {
		table = le64w(table, off)
	}
	names := []string{"Uncompressed", "MSCompressed"}
	nameList := le16(nil, 0)
	nameList = le16(nameList, uint16(len(names)))
	for _, n := range names {
		nameList = le16(nameList, uint16(len(n)))
		nameList = append(nameList, utf16le(n)...)
		nameList = le16(nameList, 0)
	}
	binary.LittleEndian.PutUint16(nameList, uint16(len(nameList)/2))
	var section0 []byte
	add := func(name string, data []byte) {
		entries = append(entries, dirEntry{name, 0, len(section0), len(data)})
		section0 = append(section0, data...)
	}
	add("::DataSpace/Storage/MSCompressed/ControlData", lzxc)
	add("::DataSpace/Storage/MSCompressed/Transform/{7FC28940-9D31-11D0-9B27-00A0C91E9C7C}/InstanceData/ResetTable", table)
	add("::DataSpace/Storage/MSCompressed/Content", compressed)
	add("::DataSpace/NameList", nameList)
	add("::DataSpace/Storage/MSCompressed/SpanInfo", le64w(nil, uint64(len(content))))
	add("::DataSpace/Storage/MSCompressed/Transform/List", utf16le("{7FC28940-9D31-11D0-9B27-00A0C91E9C7C}"))
	for _, f := range files {
		if f.plain {
			add(f.name, f.data)
		}
	}
	// A directory chunk lists its entries in the order of their names,
	// without case.
	slices.SortStableFunc(entries, func(a, b dirEntry) int {
		return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})

	// The directory: the header ITSP, then the PMGL chunks, each with its
	// entries and the count of its entries at its end.
	var chunks [][]byte
	chunk := append([]byte("PMGL"), make([]byte, 16)...)
	count := 0
	finish := func() {
		free := o.chunkSize - len(chunk)
		binary.LittleEndian.PutUint32(chunk[4:], uint32(free))
		chunk = append(chunk, make([]byte, free)...)
		binary.LittleEndian.PutUint16(chunk[o.chunkSize-2:], uint16(count))
		chunks = append(chunks, chunk)
		chunk = append([]byte("PMGL"), make([]byte, 16)...)
		count = 0
	}
	for _, e := range entries {
		var enc []byte
		enc = encintw(enc, len(e.name))
		enc = append(enc, e.name...)
		enc = encintw(enc, e.section)
		enc = encintw(enc, e.off)
		enc = encintw(enc, e.size)
		if len(chunk)+len(enc) > o.chunkSize-2 {
			finish()
		}
		chunk = append(chunk, enc...)
		count++
	}
	finish()
	for i, c := range chunks {
		prev, next := uint32(i-1), uint32(i+1)
		if i == 0 {
			prev = 0xffffffff
		}
		if i == len(chunks)-1 {
			next = 0xffffffff
		}
		binary.LittleEndian.PutUint32(c[12:], prev)
		binary.LittleEndian.PutUint32(c[16:], next)
	}
	itsp := []byte("ITSP")
	for _, v := range []uint32{1, 0x54, 0x0A, uint32(o.chunkSize), 2, 1, 0xffffffff, 0, uint32(len(chunks) - 1), 0xffffffff, uint32(len(chunks)), 0x409} {
		itsp = le32w(itsp, v)
	}
	itsp = guid(itsp, "{5D02926A-212E-11D0-9DF9-00A0C922E6EC}")
	itsp = le32w(itsp, 0x54)
	for range 3 {
		itsp = le32w(itsp, 0xffffffff)
	}
	directory := itsp
	for _, c := range chunks {
		directory = append(directory, c...)
	}

	// The header ITSF, then header section 0, the directory and section 0.
	itsfSize := 0x60
	if o.version == 2 {
		itsfSize = 0x58
	}
	const hs0Size = 0x18
	dirOff := itsfSize + hs0Size
	contentOff := dirOff + len(directory)
	total := contentOff + len(section0)
	b := []byte("ITSF")
	b = le32w(b, uint32(o.version))
	b = le32w(b, uint32(itsfSize))
	b = le32w(b, 1)
	b = binary.BigEndian.AppendUint32(b, 0x12345678)
	b = le32w(b, o.lcid)
	b = guid(b, "{7C01FD10-7BAA-11D0-9E0C-00A0C922E6EC}")
	b = guid(b, "{7C01FD11-7BAA-11D0-9E0C-00A0C922E6EC}")
	b = le64w(b, uint64(itsfSize))
	b = le64w(b, hs0Size)
	b = le64w(b, uint64(dirOff))
	b = le64w(b, uint64(len(directory)))
	if o.version == 3 {
		b = le64w(b, uint64(contentOff))
	}
	b = le32w(b, 0x01FE)
	b = le32w(b, 0)
	b = le64w(b, uint64(total))
	b = le32w(b, 0)
	b = le32w(b, 0)
	b = append(b, directory...)
	return append(b, section0...)
}
