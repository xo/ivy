package mobi

import (
	"fmt"
	"slices"

	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
)

// The NCX index of a MOBI or KF8 book is its table of contents. Each entry
// gives the offset of its target in the text, the offset of its label in
// the CNCX records, its depth from 0 and, in KF8, its fragment and its
// offset in the fragment. An entry can also give its parent, as the number
// of the parent entry. The CNCX records follow the records of the index.
// The header of the first INDX record gives their number at 52. A string of
// a CNCX record is its length, as a number of variable width, then its
// bytes. The offset of a string is the number of its record times 0x10000,
// plus its offset in the record.
const (
	tagNCXOffset = 1
	tagNCXLabel  = 3
	tagNCXDepth  = 4
	tagNCXPosFID = 6
	tagNCXParent = 21
	indxCNCX     = 52
	// cncxStride is the step of the offsets of a string from one CNCX
	// record to the next.
	cncxStride = 0x10000
)

// ncxEntry is an entry of the NCX index.
type ncxEntry struct {
	label string
	// level is the level of the entry, from 1.
	level int
	// pos is the offset of the target in the text of a MOBI book, or -1.
	pos int
	// fid and off are the fragment and the offset of the target in a KF8
	// book, or -1.
	fid, off int
	// parent is the number of the parent entry, or -1.
	parent int
}

// readNCX returns the entries of the NCX index of the book, in the order of
// the table of contents. A book without an index, or whose index cannot be
// read, gives no entry.
func (d *Document) readNCX(h *header) []ncxEntry {
	if h.ncx < 0 {
		return nil
	}
	budget := d.lim.MaxObjects
	index, err := d.readIndex(h.ncx, &budget)
	if err != nil {
		d.log.Debug("mobi: table of contents left out", "err", err)
		return nil
	}
	cncx := d.readCNCX(h)
	entries := make([]ncxEntry, len(index))
	for i, e := range index {
		ne := ncxEntry{level: 1, pos: -1, fid: -1, off: -1, parent: -1}
		if v, ok := e.value(tagNCXLabel, 0); ok {
			ne.label = cncx[v]
		}
		if v, ok := e.value(tagNCXDepth, 0); ok {
			ne.level = v + 1
		}
		if v, ok := e.value(tagNCXOffset, 0); ok {
			ne.pos = v
		}
		fid, ok1 := e.value(tagNCXPosFID, 0)
		off, ok2 := e.value(tagNCXPosFID, 1)
		if ok1 && ok2 {
			ne.fid, ne.off = fid, off
		}
		if v, ok := e.value(tagNCXParent, 0); ok && v != i && v < len(index) {
			ne.parent = v
		}
		entries[i] = ne
	}
	return ncxOrder(entries, d.lim.MaxDepth)
}

// ncxOrder returns the entries in the order of the table of contents. A KF8
// book lists the entries of each depth after those of the depth above, and
// gives the parent of each entry. The order is then the order of a walk of
// the tree: each entry, then its children in the order of the index. An
// entry whose parent does not lead to a root keeps its place after the
// tree. Without parents, the order of the index is the order of the table.
func ncxOrder(entries []ncxEntry, maxDepth int) []ncxEntry {
	children := make([][]int, len(entries))
	var roots []int
	parents := false
	for i, e := range entries {
		if e.parent >= 0 {
			children[e.parent] = append(children[e.parent], i)
			parents = true
		} else {
			roots = append(roots, i)
		}
	}
	if !parents {
		return entries
	}
	out := make([]ncxEntry, 0, len(entries))
	visited := make([]bool, len(entries))
	var walk func(i, depth int)
	walk = func(i, depth int) {
		if visited[i] || depth > maxDepth {
			return
		}
		visited[i] = true
		out = append(out, entries[i])
		for _, c := range children[i] {
			walk(c, depth+1)
		}
	}
	for _, r := range roots {
		walk(r, 1)
	}
	for i := range entries {
		if !visited[i] {
			out = append(out, entries[i])
		}
	}
	return slices.Clip(out)
}

// readCNCX returns the strings of the CNCX records of the NCX index, by
// their offsets. A record that cannot be read ends the strings.
func (d *Document) readCNCX(h *header) map[int]string {
	out := make(map[int]string)
	head, err := d.db.record(h.ncx)
	if err != nil || len(head) < indxCNCX+4 {
		return out
	}
	count, n := int(be32(head, indxCount)), int(be32(head, indxCNCX))
	first := h.ncx + count + 1
	if count < 0 || n < 0 || first < 0 || first+n > d.db.count() {
		return out
	}
	for i := range n {
		rec, err := d.db.record(first + i)
		if err != nil {
			return out
		}
		for pos := 0; pos < len(rec); {
			length, k := vwi(rec[pos:])
			if k == 0 || int64(pos)+int64(k)+int64(length) > int64(len(rec)) {
				break
			}
			if length > 0 {
				if len(out) >= d.lim.MaxObjects {
					return out
				}
				out[i*cncxStride+pos] = h.decode(rec[pos+k : pos+k+int(length)])
			}
			pos += k + int(length)
		}
	}
	return out
}

// ncxTargets returns the offsets of the text that the entries of a MOBI
// book target, for the anchors of the text.
func ncxTargets(entries []ncxEntry) []int {
	var out []int
	for _, e := range entries {
		if e.pos >= 0 {
			out = append(out, e.pos)
		}
	}
	return out
}

// mobiTOC returns the table of contents of a MOBI book whose text is the
// document name. Each target is an anchor that mobiHTML adds.
func (d *Document) mobiTOC(entries []ncxEntry, name string) []layout.TOCEntry {
	t := ebook.NewTOC(d.lim)
	for _, e := range entries {
		if t.Full() {
			break
		}
		if e.pos >= 0 {
			t.Add(e.label, e.level, fmt.Sprintf("%s#filepos%d", name, e.pos))
		}
	}
	return t.Entries
}

// kf8TOC returns the table of contents of a KF8 book: the part of each
// target, with the id of its element when the element has one.
func (d *Document) kf8TOC(entries []ncxEntry, l *links) []layout.TOCEntry {
	t := ebook.NewTOC(d.lim)
	for _, e := range entries {
		if t.Full() {
			break
		}
		if e.fid < 0 {
			continue
		}
		if target := l.position(e.fid, e.off); target != "#" {
			t.Add(e.label, e.level, target)
		}
	}
	return t.Entries
}
