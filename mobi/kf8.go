package mobi

import (
	"bytes"
	"fmt"
	"math/bits"
	"regexp"
	"strconv"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
)

// An index of KF8 is an INDX record with the table of its tags (TAGX), then
// the INDX records that hold its entries. The skeleton index and the
// fragment index of a KF8 book give the parts of the book.
//
// An INDX record starts with "INDX" and the length of its header. The
// header gives, in 4 bytes each, the offset of the IDXT at 20, the number
// of entries or of the records that follow at 24, and the offset of the
// TAGX at 180. The IDXT holds the offset of each entry, in 2 bytes each.
// An entry holds its label, as a length of 1 byte and the bytes, then the
// control bytes, then the values of its tags.
const (
	indxIDXT  = 20
	indxCount = 24
	indxTAGX  = 180
	// tagxHeader is the size of the header of a TAGX: "TAGX", its length
	// and the number of control bytes.
	tagxHeader = 12
	// maxVWI is the number of bytes of a number of variable width.
	maxVWI = 5
)

// The tags of the entries of the skeleton index and of the fragment index.
const (
	tagCount    = 1
	tagFile     = 3
	tagGeometry = 6
)

// indexEntry is an entry of an index: its label and the values of its tags.
type indexEntry struct {
	label string
	tags  map[byte][]uint32
}

// value returns value i of the tag t, or false.
func (e indexEntry) value(t byte, i int) (int, bool) {
	v := e.tags[t]
	if i >= len(v) {
		return 0, false
	}
	return int(v[i]), true
}

// tagDef is an entry of a TAGX table: the tag, the number of values of the
// tag, the mask of its control byte, and the end of a control byte.
type tagDef struct {
	tag, values, mask, end byte
}

// readIndex reads the entries of the index whose INDX record is rec.
func (d *Document) readIndex(rec int, budget *int) ([]indexEntry, error) {
	head, err := d.db.record(rec)
	if err != nil {
		return nil, err
	}
	if len(head) < indxTAGX+4 || string(head[:4]) != "INDX" {
		return nil, fmt.Errorf("index %d without its INDX record: %w", rec, ivy.ErrFormat)
	}
	tags, controls, err := readTAGX(head, int(be32(head, indxTAGX)))
	if err != nil {
		return nil, fmt.Errorf("index %d: %w", rec, err)
	}
	count := int(be32(head, indxCount))
	if count < 0 || rec+count >= d.db.count() {
		return nil, fmt.Errorf("index %d of %d records: %w", rec, count, ivy.ErrFormat)
	}
	var entries []indexEntry
	for i := 1; i <= count; i++ {
		buf, err := d.db.record(rec + i)
		if err != nil {
			return nil, err
		}
		if len(buf) < indxCount+4 || string(buf[:4]) != "INDX" {
			return nil, fmt.Errorf("record %d of index %d is not an INDX record: %w", rec+i, rec, ivy.ErrFormat)
		}
		idxt, n := int(be32(buf, indxIDXT)), int(be32(buf, indxCount))
		if idxt < 0 || idxt+4 > len(buf) || string(buf[idxt:idxt+4]) != "IDXT" || n < 0 || idxt+4+2*n > len(buf) {
			return nil, fmt.Errorf("record %d of index %d without its IDXT: %w", rec+i, rec, ivy.ErrFormat)
		}
		if *budget -= n; *budget < 0 {
			return nil, fmt.Errorf("indexes of more than %d entries: %w", d.lim.MaxObjects, ivy.ErrLimit)
		}
		for j := range n {
			start := int(be16(buf, idxt+4+2*j))
			end := idxt
			if j+1 < n {
				end = int(be16(buf, idxt+4+2*j+2))
			}
			if start >= end || end > len(buf) {
				return nil, fmt.Errorf("entry %d of record %d of index %d: %w", j, rec+i, rec, ivy.ErrFormat)
			}
			e, err := readEntry(buf[start:end], tags, controls)
			if err != nil {
				return nil, fmt.Errorf("entry %d of record %d of index %d: %w", j, rec+i, rec, err)
			}
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// readTAGX reads the TAGX table at off in rec, and returns its entries and
// the number of control bytes of an entry.
func readTAGX(rec []byte, off int) ([]tagDef, int, error) {
	if off < 0 || off+tagxHeader > len(rec) || string(rec[off:off+4]) != "TAGX" {
		return nil, 0, fmt.Errorf("index without its TAGX: %w", ivy.ErrFormat)
	}
	length, controls := int(be32(rec, off+4)), int(be32(rec, off+8))
	if length < tagxHeader || off+length > len(rec) || controls < 1 || controls > 8 {
		return nil, 0, fmt.Errorf("TAGX of %d bytes and %d control bytes: %w", length, controls, ivy.ErrFormat)
	}
	tags := make([]tagDef, 0, (length-tagxHeader)/4)
	for p := off + tagxHeader; p+4 <= off+length; p += 4 {
		tags = append(tags, tagDef{tag: rec[p], values: rec[p+1], mask: rec[p+2], end: rec[p+3]})
	}
	return tags, controls, nil
}

// readEntry reads an entry of an index.
//
// Each tag of the TAGX has a mask in a control byte. The bits of the mask
// in the control byte give the number of the values of the tag, in groups
// of the number of values of the TAGX. If the bits are all set and the
// mask has more than one bit, a number of variable width follows instead,
// which gives the number of bytes of the values. The values of the tags
// then follow in the order of the TAGX, as numbers of variable width.
func readEntry(buf []byte, tags []tagDef, controls int) (indexEntry, error) {
	n := int(buf[0])
	if 1+n+controls > len(buf) {
		return indexEntry{}, fmt.Errorf("entry of %d bytes with a label of %d: %w", len(buf), n, ivy.ErrFormat)
	}
	e := indexEntry{label: string(buf[1 : 1+n]), tags: make(map[byte][]uint32)}
	control := buf[1+n : 1+n+controls]
	pos := 1 + n + controls
	type pending struct {
		def tagDef
		// count is the number of groups of values, or -1 when size gives
		// the number of bytes of the values
		count, size int
	}
	var todo []pending
	cb := 0
	for _, t := range tags {
		if t.end&1 != 0 {
			cb++
			continue
		}
		if cb >= len(control) || t.mask == 0 {
			continue
		}
		v := control[cb] & t.mask
		if v == 0 {
			continue
		}
		if v == t.mask && bits.OnesCount8(t.mask) > 1 {
			size, k := vwi(buf[pos:])
			if k == 0 {
				return indexEntry{}, fmt.Errorf("entry without the size of tag %d: %w", t.tag, ivy.ErrFormat)
			}
			pos += k
			todo = append(todo, pending{def: t, count: -1, size: int(size)})
			continue
		}
		todo = append(todo, pending{def: t, count: int(v >> bits.TrailingZeros8(t.mask))})
	}
	for _, p := range todo {
		var values []uint32
		if p.count >= 0 {
			for range p.count * int(p.def.values) {
				v, k := vwi(buf[pos:])
				if k == 0 {
					break
				}
				values = append(values, v)
				pos += k
			}
		} else {
			end := min(pos+p.size, len(buf))
			for pos < end {
				v, k := vwi(buf[pos:end])
				if k == 0 {
					break
				}
				values = append(values, v)
				pos += k
			}
		}
		e.tags[p.def.tag] = values
	}
	return e, nil
}

// vwi reads a number of variable width at the start of b: 7 bits a byte,
// the high byte first, and the last byte has the bit 0x80 set. It returns
// the number and its length, or a length of 0 if b does not start with one.
func vwi(b []byte) (uint32, int) {
	var v uint32
	for i := 0; i < len(b) && i < maxVWI; i++ {
		v = v<<7 | uint32(b[i]&0x7f)
		if b[i]&0x80 != 0 {
			return v, i + 1
		}
	}
	return 0, 0
}

// kf8Book is the content of a KF8 book: its parts of HTML and its flows,
// such as the style sheets.
type kf8Book struct {
	// parts are the parts of the book, in the order of reading.
	parts [][]byte
	// flows are the flows of the text. Flow 0 holds the parts, and the
	// others hold style sheets and images of SVG.
	flows [][]byte
	// fragments gives the part of each fragment, and its offset in the
	// part.
	fragments []fragmentPlace
}

// fragmentPlace is the place of a fragment in the parts.
type fragmentPlace struct {
	part, offset int
}

// readKF8 splits the text of a KF8 book into its flows, and the first flow
// into its parts.
func (d *Document) readKF8(h *header, text []byte) (*kf8Book, error) {
	book := &kf8Book{flows: d.flows(h, text)}
	if h.skeleton < 0 || h.fragment < 0 {
		book.parts = [][]byte{book.flows[0]}
		return book, nil
	}
	budget := d.lim.MaxObjects
	skeletons, err := d.readIndex(h.skeleton, &budget)
	if err != nil {
		return nil, fmt.Errorf("reading the skeleton index: %w", err)
	}
	fragments, err := d.readIndex(h.fragment, &budget)
	if err != nil {
		return nil, fmt.Errorf("reading the fragment index: %w", err)
	}
	if err := book.assemble(skeletons, fragments, d.lim); err != nil {
		return nil, err
	}
	return book, nil
}

// flows returns the flows of the text, from the FDST record. The FDST holds
// "FDST", the offset of its table, the number of flows, and then the start
// and the end of each flow, in 4 bytes each. Without an FDST, the text is
// one flow.
func (d *Document) flows(h *header, text []byte) [][]byte {
	whole := [][]byte{text}
	if h.fdst < 0 {
		return whole
	}
	rec, err := d.db.record(h.fdst)
	if err != nil || len(rec) < 12 || string(rec[:4]) != "FDST" {
		return whole
	}
	off, n := int64(be32(rec, 4)), int64(be32(rec, 8))
	if n < 1 || off < 12 || off+8*n > int64(len(rec)) {
		return whole
	}
	flows := make([][]byte, 0, n)
	for i := range n {
		start, end := int64(be32(rec, int(off+8*i))), int64(be32(rec, int(off+8*i+4)))
		if start > end || end > int64(len(text)) {
			d.log.Debug("mobi: flow out of the text", "flow", i, "start", start, "end", end)
			flows = append(flows, nil)
			continue
		}
		flows = append(flows, text[start:end])
	}
	return flows
}

// assemble builds the parts from the skeletons and the fragments. Flow 0
// holds each skeleton and then its fragments. Each fragment goes into its
// skeleton at the position that its label gives, which counts from the
// start of flow 0.
func (b *kf8Book) assemble(skeletons, fragments []indexEntry, lim ivy.Limits) error {
	text := b.flows[0]
	// total bounds the bytes of the parts, and work the bytes that the
	// insertions move, because a crafted book can use a part of the text
	// many times
	var total, work int64
	next := 0
	for i, s := range skeletons {
		count, _ := s.value(tagCount, 0)
		start, ok1 := s.value(tagGeometry, 0)
		length, ok2 := s.value(tagGeometry, 1)
		if !ok1 || !ok2 || start < 0 || length < 0 || start+length > len(text) {
			return fmt.Errorf("skeleton %d out of the text: %w", i, ivy.ErrFormat)
		}
		part := bytes.Clone(text[start : start+length])
		pos := start + length
		for range count {
			if next >= len(fragments) {
				break
			}
			f := fragments[next]
			insert, err := strconv.Atoi(strings.TrimSpace(f.label))
			flen, ok := f.value(tagGeometry, 1)
			if err != nil || !ok || flen < 0 || pos+flen > len(text) {
				return fmt.Errorf("fragment %d out of the text: %w", next, ivy.ErrFormat)
			}
			at := min(max(insert-start, 0), len(part))
			if work += int64(len(part) + flen); work > 4*lim.MaxMemory {
				return fmt.Errorf("kf8 parts that move more than %d bytes: %w", 4*lim.MaxMemory, ivy.ErrLimit)
			}
			part = append(part[:at], append(bytes.Clone(text[pos:pos+flen]), part[at:]...)...)
			b.fragments = append(b.fragments, fragmentPlace{part: i, offset: at})
			pos += flen
			next++
		}
		if total += int64(len(part)); total > lim.MaxMemory {
			return fmt.Errorf("kf8 parts of more than %d bytes: %w", lim.MaxMemory, ivy.ErrLimit)
		}
		b.parts = append(b.parts, part)
	}
	if len(b.parts) == 0 {
		b.parts = [][]byte{text}
	}
	return nil
}

// kindleLink is a link of KF8: kindle:embed:XXXX to a resource,
// kindle:flow:XXXX to a flow, both with an optional mime type, and
// kindle:pos:fid:XXXX:off:YYYYYYYYYY to an offset in a fragment. The
// numbers are in base 32, with the digits 0 to 9 and A to V.
var kindleLink = regexp.MustCompile(`kindle:(?:(embed|flow):([0-9A-Va-v]{1,8})(?:\?mime=[A-Za-z0-9.+/-]*)?|pos:fid:([0-9A-Va-v]{1,8}):off:([0-9A-Va-v]{1,10}))`)

// idAttr is the attribute id of a tag.
var idAttr = regexp.MustCompile(`\sid\s*=\s*["']([^"'<>]*)["']`)

// links names the files of a KF8 book in its FS.
type links struct {
	book  *kf8Book
	res   *resources
	parts []string
	flows []string
}

// rewrite replaces each link of KF8 in s with the name of its file.
func (l *links) rewrite(s string) string {
	return kindleLink.ReplaceAllStringFunc(s, func(m string) string {
		g := kindleLink.FindStringSubmatch(m)
		switch g[1] {
		case "embed":
			if name, ok := l.res.name(base32(g[2])); ok {
				return name
			}
		case "flow":
			if n := base32(g[2]); n > 0 && n < len(l.flows) && l.flows[n] != "" {
				return l.flows[n]
			}
		default:
			return l.position(base32(g[3]), base32(g[4]))
		}
		return m
	})
}

// position returns the link to the offset off in the fragment fid: the
// name of its part, and the id of the element at the offset, if it has
// one.
func (l *links) position(fid, off int) string {
	if fid < 0 || fid >= len(l.book.fragments) {
		return "#"
	}
	f := l.book.fragments[fid]
	name := l.parts[f.part]
	part := l.book.parts[f.part]
	at := f.offset + off
	if at < 0 || at >= len(part) {
		return name
	}
	// the scan is short, because a crafted part can hold many links
	near := part[at:min(at+maxTagScan, len(part))]
	lt := bytes.IndexByte(near, '<')
	if lt < 0 {
		return name
	}
	tag := near[lt:]
	if gt := bytes.IndexByte(tag, '>'); gt >= 0 {
		tag = tag[:gt]
	}
	if m := idAttr.FindSubmatch(tag); m != nil {
		return name + "#" + string(m[1])
	}
	return name
}

// base32 reads a number of KF8 in base 32. It returns -1 for a number that
// is not valid.
func base32(s string) int {
	v, err := strconv.ParseInt(s, 32, 64)
	if err != nil || v > 1<<31 {
		return -1
	}
	return int(v)
}

// isXHTML reports whether a part of a KF8 book is XHTML that the XML parser
// reads. A part that is not is read as HTML, which recovers from errors.
func isXHTML(part []byte, lim ivy.Limits) bool {
	if !bytes.HasPrefix(bytes.TrimLeft(part, " \t\r\n\uFEFF"), []byte("<?xml")) {
		return false
	}
	_, err := ebook.ParseXML(part, lim)
	return err == nil
}

// svgRoot is the start tag of the root of an SVG document, and svgSize is
// its attribute width or height in percent.
var (
	svgRoot = regexp.MustCompile(`(?is)<svg\b[^>]*>`)
	svgSize = regexp.MustCompile(`(?i)(\s(width|height)\s*=\s*["'])\s*([0-9]{1,3}(?:\.[0-9]{1,3})?)\s*%`)
)

// svgPageSize returns the SVG document doc with a width or a height in
// percent of its root as a part of the page area of an ebook on the paper
// p. An SVG flow is often the cover of a book, which a part shows with an img
// element and a size of 100%. calibre draws such an image as wide as the
// column and as high as the page area, with the picture in the middle. The
// layout would give it the height of its width and of its viewBox instead.
func svgPageSize(doc string, p ivy.Paper) string {
	loc := svgRoot.FindStringIndex(doc)
	if loc == nil {
		return doc
	}
	tag := svgSize.ReplaceAllStringFunc(doc[loc[0]:loc[1]], func(m string) string {
		g := svgSize.FindStringSubmatch(m)
		pct, err := strconv.ParseFloat(g[3], 64)
		if err != nil {
			return m
		}
		size, height := ebook.BookArea(p)
		if strings.EqualFold(g[2], "height") {
			size = height
		}
		return g[1] + strconv.FormatFloat(size*pct/100, 'f', 2, 64) + "pt"
	})
	return doc[:loc[0]] + tag + doc[loc[1]:]
}

// flowType returns the extension of a flow from its content: an image of
// SVG, or a style sheet.
func flowType(flow []byte) string {
	head := bytes.TrimLeft(flow[:min(len(flow), 512)], " \t\r\n\uFEFF")
	if bytes.HasPrefix(head, []byte("<svg")) || bytes.HasPrefix(head, []byte("<?xml")) && bytes.Contains(head, []byte("<svg")) {
		return "svg"
	}
	return "css"
}
