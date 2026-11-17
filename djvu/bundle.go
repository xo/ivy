package djvu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/doc"
	"github.com/xo/ivy/djvu/internal/iff"
)

// infoHead is the number of bytes at the start of a page that the bundle
// reads to find its INFO chunk, which is the first chunk of a page.
const infoHead = 64

// bundle is a bundled document, FORM:DJVM, that reads its components from
// the source when a page needs them. It keeps the directory only, so a
// large document does not stay whole in memory.
type bundle struct {
	src  ivy.Source
	lim  ivy.Limits
	dir  *doc.Dir
	byID map[string]doc.Component
}

// openBundle reads the directory of the bundled document in src, whose
// FORM:DJVM chunk starts at off, and the INFO chunk of each page. It
// returns an error that wraps ivy.ErrUnsupported for an indirect document.
func openBundle(src ivy.Source, off int64, lim ivy.Limits) (*bundle, []*doc.Page, error) {
	// the DIRM chunk is the first chunk of FORM:DJVM, after its type
	id, data, err := readChunk(src, off+12, lim)
	if err != nil {
		return nil, nil, err
	}
	if id != "DIRM" {
		return nil, nil, fmt.Errorf("djvu FORM:DJVM without DIRM: %w", ivy.ErrFormat)
	}
	dir, err := doc.ParseDirWithin(data, lim)
	if err != nil {
		return nil, nil, fmt.Errorf("reading djvu directory: %w", format(err))
	}
	if !dir.Bundled {
		return nil, nil, fmt.Errorf("indirect djvu document: %w", ivy.ErrUnsupported)
	}
	if len(dir.Components) > lim.MaxObjects {
		return nil, nil, fmt.Errorf("djvu of %d components: %w", len(dir.Components), ivy.ErrLimit)
	}
	b := &bundle{src: src, lim: lim, dir: dir, byID: make(map[string]doc.Component, len(dir.Components))}
	for _, c := range dir.Components {
		if _, ok := b.byID[c.ID]; !ok {
			b.byID[c.ID] = c
		}
	}
	var pages []*doc.Page
	for _, c := range dir.Components {
		if c.Type != doc.PageComp {
			continue
		}
		in, err := b.info(c)
		pages = append(pages, &doc.Page{ID: c.ID, Info: in, Err: err})
	}
	if len(pages) == 0 {
		return nil, nil, fmt.Errorf("djvu without pages: %w", ivy.ErrFormat)
	}
	return b, pages, nil
}

// readChunk reads the chunk whose header is at off: its id and its data.
// The data of a FORM chunk starts with its type.
func readChunk(src ivy.Source, off int64, lim ivy.Limits) (string, []byte, error) {
	var head [8]byte
	if n, err := src.ReadAt(head[:], off); n < len(head) {
		return "", nil, fmt.Errorf("reading djvu chunk at %d: %w", off, errors.Join(err, ivy.ErrFormat))
	}
	size := int64(binary.BigEndian.Uint32(head[4:]))
	switch {
	case size > lim.MaxMemory:
		return "", nil, fmt.Errorf("djvu chunk of %d bytes: %w", size, ivy.ErrLimit)
	case size > src.Size()-off-8:
		// a chunk that runs past the file is cut, as iff.Parse does
		size = max(0, src.Size()-off-8)
	}
	data := make([]byte, size)
	if n, err := src.ReadAt(data, off+8); int64(n) < size {
		return "", nil, fmt.Errorf("reading djvu chunk at %d: %w", off, errors.Join(err, ivy.ErrFormat))
	}
	return string(head[:4]), data, nil
}

// raw reads the FORM chunk of the component c, with its header.
func (b *bundle) raw(c doc.Component) ([]byte, error) {
	if c.Offset < 0 || int64(c.Offset) > b.src.Size() {
		return nil, fmt.Errorf("djvu component %q at %d: %w", c.ID, c.Offset, ivy.ErrFormat)
	}
	id, data, err := readChunk(b.src, int64(c.Offset), b.lim)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 8, 8+len(data))
	copy(out, id)
	binary.BigEndian.PutUint32(out[4:], uint32(len(data)))
	return append(out, data...), nil
}

// form reads the component c, and parses its FORM chunk.
func (b *bundle) form(c doc.Component) (*iff.Chunk, error) {
	data, err := b.raw(c)
	if err != nil {
		return nil, err
	}
	f, err := iff.ParseAt(data, 0)
	if f == nil {
		return nil, fmt.Errorf("djvu component %q: %w", c.ID, format(err))
	}
	if !f.Composite() {
		return nil, fmt.Errorf("djvu component %q is %s: %w", c.ID, f.Name(), ivy.ErrFormat)
	}
	if depth(f) > b.lim.MaxDepth {
		return nil, fmt.Errorf("djvu chunks nested more than %d deep: %w", b.lim.MaxDepth, ivy.ErrLimit)
	}
	return f, nil
}

// info reads the INFO chunk of the page c. It reads the start of the page,
// and the whole page if the INFO chunk is not at its start.
func (b *bundle) info(c doc.Component) (doc.Info, error) {
	head := make([]byte, infoHead)
	n, _ := b.src.ReadAt(head, int64(c.Offset))
	head = head[:max(n, 0)]
	if len(head) >= 12 && string(head[:4]) == "FORM" && string(head[8:12]) == "DJVU" {
		// the chunks of the page that the head holds whole
		for p := 12; p+8 <= len(head); {
			size := int(binary.BigEndian.Uint32(head[p+4:]))
			if size > len(head)-p-8 {
				break
			}
			if string(head[p:p+4]) == "INFO" {
				return doc.ParseInfo(head[p+8 : p+8+size])
			}
			p += 8 + size + size&1
		}
	}
	f, err := b.form(c)
	if err != nil {
		return doc.Info{}, err
	}
	if f.Name() != "FORM:DJVU" {
		return doc.Info{}, fmt.Errorf("djvu page %q is %s: %w", c.ID, f.Name(), ivy.ErrFormat)
	}
	k := f.Find("INFO")
	if k == nil {
		return doc.Info{}, fmt.Errorf("djvu page %q has no INFO: %w", c.ID, ivy.ErrFormat)
	}
	return doc.ParseInfo(k.Data)
}

// page reads the page c and the components that it includes, as a document
// of one page.
func (b *bundle) page(c doc.Component) (*doc.Document, *doc.Page, error) {
	data, err := b.raw(c)
	if err != nil {
		return nil, nil, err
	}
	if len(data) < 12 || string(data[:4]) != "FORM" || string(data[8:12]) != "DJVU" {
		return nil, nil, fmt.Errorf("djvu page %q is not FORM:DJVU: %w", c.ID, ivy.ErrFormat)
	}
	// a page of one file names the components that it includes by their
	// ids, which the loader finds in the directory
	d, err := doc.ParseWithin(data, b.load, b.lim)
	if err != nil {
		return nil, nil, fmt.Errorf("reading djvu page %q: %w", c.ID, format(err))
	}
	if depth(d.Root) > b.lim.MaxDepth {
		return nil, nil, fmt.Errorf("djvu chunks nested more than %d deep: %w", b.lim.MaxDepth, ivy.ErrLimit)
	}
	if len(d.Pages) != 1 || d.Pages[0].Err != nil {
		return nil, nil, fmt.Errorf("reading djvu page %q: %w", c.ID, ivy.ErrFormat)
	}
	return d, d.Pages[0], nil
}

// load is the loader of the components that a page includes. It reads the
// component with the id name.
func (b *bundle) load(name string) ([]byte, error) {
	c, ok := b.byID[strings.TrimSpace(name)]
	if !ok {
		return nil, fmt.Errorf("djvu component %q not in the directory: %w", name, ivy.ErrFormat)
	}
	return b.raw(c)
}

// forms reads the components of the type typ, and leaves out each one that
// cannot be read.
func (b *bundle) forms(typ doc.ComponentType) []*iff.Chunk {
	var out []*iff.Chunk
	for _, c := range b.dir.Components {
		if c.Type != typ {
			continue
		}
		if f, err := b.form(c); err == nil {
			out = append(out, f)
		}
	}
	return out
}

// pageComponent returns the component of page n.
func (b *bundle) pageComponent(n int) (doc.Component, bool) {
	for _, c := range b.dir.Components {
		if c.Type != doc.PageComp {
			continue
		}
		if n == 0 {
			return c, true
		}
		n--
	}
	return doc.Component{}, false
}
