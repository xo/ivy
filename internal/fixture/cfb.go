package fixture

import (
	"encoding/binary"
	"slices"
	"strings"
	"time"
	"unicode/utf16"
)

// CFBEntry is a stream or a storage of a Compound File. A storage has
// children and no data.
type CFBEntry struct {
	Name     string
	Data     []byte
	Storage  bool
	Children []CFBEntry
}

// The special numbers of the sectors of MS-CFB.
const (
	cfbFATSect    = 0xFFFFFFFD
	cfbEndOfChain = 0xFFFFFFFE
	cfbFree       = 0xFFFFFFFF
	cfbNoStream   = 0xFFFFFFFF
	cfbCutoff     = 4096
)

// cfbNode is an entry of the directory that CFB writes.
type cfbNode struct {
	entry    CFBEntry
	typ      byte
	children []int
	left     uint32
	right    uint32
	child    uint32
	start    uint32
	size     uint64
}

// CFB returns a Compound File of version 3 that holds the entries in its
// root storage (MS-CFB). A stream of less than 4096 bytes goes into the mini
// stream, as the specification says.
func CFB(entries ...CFBEntry) []byte {
	nodes := []*cfbNode{{entry: CFBEntry{Name: "Root Entry", Storage: true, Children: entries}, typ: 5}}
	for i := 0; i < len(nodes); i++ {
		for _, c := range nodes[i].entry.Children {
			typ := byte(2)
			if c.Storage {
				typ = 1
			}
			nodes[i].children = append(nodes[i].children, len(nodes))
			nodes = append(nodes, &cfbNode{entry: c, typ: typ})
		}
	}
	// The mini stream and its table.
	var mini []byte
	var miniFAT []uint32
	for _, n := range nodes[1:] {
		if n.typ != 2 || len(n.entry.Data) >= cfbCutoff {
			continue
		}
		n.size = uint64(len(n.entry.Data))
		if n.size == 0 {
			n.start = cfbEndOfChain
			continue
		}
		n.start = uint32(len(miniFAT))
		count := (len(n.entry.Data) + 63) / 64
		for i := range count {
			next := uint32(len(miniFAT) + 1)
			if i == count-1 {
				next = cfbEndOfChain
			}
			miniFAT = append(miniFAT, next)
		}
		mini = append(mini, n.entry.Data...)
		mini = append(mini, make([]byte, count*64-len(n.entry.Data))...)
	}
	// The sectors of the file, before the table of sectors.
	var sectors [][]byte
	var fat []uint32
	chain := func(data []byte) uint32 {
		if len(data) == 0 {
			return cfbEndOfChain
		}
		start := uint32(len(sectors))
		for off := 0; off < len(data); off += 512 {
			s := make([]byte, 512)
			copy(s, data[off:])
			sectors = append(sectors, s)
			fat = append(fat, uint32(len(sectors)))
		}
		fat[len(fat)-1] = cfbEndOfChain
		return start
	}
	for _, n := range nodes[1:] {
		if n.typ == 2 && len(n.entry.Data) >= cfbCutoff {
			n.size = uint64(len(n.entry.Data))
			n.start = chain(n.entry.Data)
		}
	}
	nodes[0].start = chain(mini)
	nodes[0].size = uint64(len(mini))
	miniFATBytes := make([]byte, 0, 4*len(miniFAT))
	for _, v := range miniFAT {
		miniFATBytes = binary.LittleEndian.AppendUint32(miniFATBytes, v)
	}
	miniFATStart := chain(miniFATBytes)
	miniFATSectors := (len(miniFATBytes) + 511) / 512
	// The directory, with each storage as a balanced tree of its children.
	for _, n := range nodes {
		n.left, n.right, n.child = cfbNoStream, cfbNoStream, cfbNoStream
	}
	for _, n := range nodes {
		kids := slices.Clone(n.children)
		slices.SortFunc(kids, func(a, b int) int {
			x, y := nodes[a].entry.Name, nodes[b].entry.Name
			if len(x) != len(y) {
				return len(x) - len(y)
			}
			return strings.Compare(strings.ToUpper(x), strings.ToUpper(y))
		})
		n.child = cfbTree(nodes, kids)
	}
	dir := make([]byte, 0, 128*len(nodes))
	for _, n := range nodes {
		dir = append(dir, cfbDirEntry(n)...)
	}
	for len(dir)%512 != 0 {
		dir = append(dir, cfbDirEntry(&cfbNode{left: cfbNoStream, right: cfbNoStream, child: cfbNoStream})...)
	}
	dirStart := chain(dir)
	// The table of sectors covers itself.
	nfat := 1
	for (len(sectors)+nfat+127)/128 > nfat {
		nfat++
	}
	if nfat > 109 {
		panic("fixture: compound file too large")
	}
	fatStart := uint32(len(sectors))
	for range nfat {
		sectors = append(sectors, nil)
		fat = append(fat, cfbFATSect)
	}
	table := make([]byte, 0, 512*nfat)
	for _, v := range fat {
		table = binary.LittleEndian.AppendUint32(table, v)
	}
	for len(table) < 512*nfat {
		table = binary.LittleEndian.AppendUint32(table, cfbFree)
	}
	for i := range nfat {
		sectors[int(fatStart)+i] = table[512*i : 512*(i+1)]
	}
	h := make([]byte, 512)
	binary.LittleEndian.PutUint64(h, 0xE11AB1A1E011CFD0)
	binary.LittleEndian.PutUint16(h[24:], 0x003E)
	binary.LittleEndian.PutUint16(h[26:], 3)
	binary.LittleEndian.PutUint16(h[28:], 0xFFFE)
	binary.LittleEndian.PutUint16(h[30:], 9)
	binary.LittleEndian.PutUint16(h[32:], 6)
	binary.LittleEndian.PutUint32(h[44:], uint32(nfat))
	binary.LittleEndian.PutUint32(h[48:], dirStart)
	binary.LittleEndian.PutUint32(h[56:], cfbCutoff)
	binary.LittleEndian.PutUint32(h[60:], miniFATStart)
	binary.LittleEndian.PutUint32(h[64:], uint32(miniFATSectors))
	binary.LittleEndian.PutUint32(h[68:], cfbEndOfChain)
	for i := range 109 {
		v := uint32(cfbFree)
		if i < nfat {
			v = fatStart + uint32(i)
		}
		binary.LittleEndian.PutUint32(h[76+4*i:], v)
	}
	out := h
	for _, s := range sectors {
		out = append(out, s...)
	}
	return out
}

// cfbTree links the nodes kids, in their order, as a balanced binary tree,
// and returns its root.
func cfbTree(nodes []*cfbNode, kids []int) uint32 {
	if len(kids) == 0 {
		return cfbNoStream
	}
	mid := len(kids) / 2
	n := nodes[kids[mid]]
	n.left = cfbTree(nodes, kids[:mid])
	n.right = cfbTree(nodes, kids[mid+1:])
	return uint32(kids[mid])
}

// cfbDirEntry returns the 128 bytes of the directory entry of n.
func cfbDirEntry(n *cfbNode) []byte {
	b := make([]byte, 128)
	if n.typ != 0 {
		u := utf16.Encode([]rune(n.entry.Name))
		u = u[:min(len(u), 31)]
		for i, c := range u {
			binary.LittleEndian.PutUint16(b[2*i:], c)
		}
		binary.LittleEndian.PutUint16(b[64:], uint16(2*len(u)+2))
	}
	b[66] = n.typ
	b[67] = 1
	binary.LittleEndian.PutUint32(b[68:], n.left)
	binary.LittleEndian.PutUint32(b[72:], n.right)
	binary.LittleEndian.PutUint32(b[76:], n.child)
	binary.LittleEndian.PutUint32(b[116:], n.start)
	binary.LittleEndian.PutUint64(b[120:], n.size)
	return b
}

// The formats of the clipboard of a thumbnail (MS-OLEPS 2.11).
const (
	ClipMetafilePict = 3
	ClipDIB          = 8
	ClipEnhMetafile  = 14
)

// Summary is the content of a stream "\x05SummaryInformation" (MS-OLEPS
// 2.25.1). A field that is empty is left out.
type Summary struct {
	// CodePage is the code page of the strings, such as 1252. Zero writes
	// 1252.
	CodePage uint16
	// Title and Author are the bytes of the strings in the code page.
	Title, Author    []byte
	Created, Changed time.Time
	// ThumbnailTag is the tag of the clipboard data: -1 for a format of
	// Windows. ThumbnailFormat is that format, such as ClipEnhMetafile.
	ThumbnailTag    int32
	ThumbnailFormat uint32
	Thumbnail       []byte
}

// Bytes returns the property set of the summary.
func (s Summary) Bytes() []byte {
	type prop struct {
		id    uint32
		value []byte
	}
	cp := s.CodePage
	if cp == 0 {
		cp = 1252
	}
	var props []prop
	props = append(props, prop{1, cfbLE(nil, uint32(2), cp, uint16(0))})
	str := func(id uint32, b []byte) {
		if b == nil {
			return
		}
		v := cfbLE(nil, uint32(0x1E), uint32(len(b)+1), b, []byte{0})
		props = append(props, prop{id, v})
	}
	str(0x02, s.Title)
	str(0x04, s.Author)
	date := func(id uint32, t time.Time) {
		if t.IsZero() {
			return
		}
		ft := uint64(t.Unix()+11644473600)*1e7 + uint64(t.Nanosecond()/100)
		props = append(props, prop{id, cfbLE(nil, uint32(0x40), uint32(ft), uint32(ft>>32))})
	}
	date(0x0C, s.Created)
	date(0x0D, s.Changed)
	if s.Thumbnail != nil {
		data := cfbLE(nil, uint32(s.ThumbnailTag))
		if s.ThumbnailTag == -1 || s.ThumbnailTag == -2 {
			data = cfbLE(data, s.ThumbnailFormat)
		}
		data = append(data, s.Thumbnail...)
		props = append(props, prop{0x11, cfbLE(nil, uint32(0x47), uint32(len(data)), data)})
	}
	// The section: its size, the count, the table of offsets and the values.
	off := 8 + 8*len(props)
	var table, values []byte
	for _, p := range props {
		for len(p.value)%4 != 0 {
			p.value = append(p.value, 0)
		}
		table = cfbLE(table, p.id, uint32(off+len(values)))
		values = append(values, p.value...)
	}
	section := cfbLE(nil, uint32(8+len(table)+len(values)), uint32(len(props)), table, values)
	fmtid := []byte{
		0xE0, 0x85, 0x9F, 0xF2, 0xF9, 0x4F, 0x68, 0x10,
		0xAB, 0x91, 0x08, 0x00, 0x2B, 0x27, 0xB3, 0xD9,
	}
	head := cfbLE(nil, uint16(0xFFFE), uint16(0), uint32(0x00020006), make([]byte, 16), uint32(1), fmtid, uint32(48))
	return append(head, section...)
}

// cfbLE appends the values to b in little-endian order: a uint16 or a
// uint32 in its size, and a []byte as it is.
func cfbLE(b []byte, values ...any) []byte {
	for _, v := range values {
		switch v := v.(type) {
		case uint16:
			b = binary.LittleEndian.AppendUint16(b, v)
		case uint32:
			b = binary.LittleEndian.AppendUint32(b, v)
		case []byte:
			b = append(b, v...)
		default:
			panic("fixture: bad value")
		}
	}
	return b
}
