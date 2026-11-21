package djvu

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/xo/okage"

	"github.com/xo/ivy/djvu/internal/doc"
	"github.com/xo/ivy/internal/display"
)

// navigation is the outline of a document and the targets of its links,
// which a document reads once, when a writer first draws a page with its
// links (D58).
type navigation struct {
	targets *doc.Targets
	// marks holds the bookmarks of each page, in the preorder of the
	// outline.
	marks [][]mark
}

// mark is a bookmark of the outline, at the top of its page.
type mark struct {
	level int
	label string
}

// navigation returns the outline of the document and the targets of its
// links. It reads them on its first call.
func (d *Document) navigation() *navigation {
	d.navOnce.Do(func() {
		n := &navigation{marks: make([][]mark, len(d.pages))}
		var bookmarks []*doc.Bookmark
		var err error
		switch {
		case d.bundle != nil:
			n.targets = doc.NewTargets(d.bundle.dir, len(d.pages))
			var data []byte
			if data, err = d.bundle.navm(); err == nil && data != nil {
				bookmarks, err = doc.OutlineOf(data, d.lim)
			}
		case d.doc != nil && d.doc.Root != nil:
			n.targets = doc.NewTargets(nil, len(d.pages))
			bookmarks, err = d.doc.Outline()
		default:
			n.targets = doc.NewTargets(nil, len(d.pages))
		}
		if err != nil {
			d.log.Debug("djvu: damaged outline", "err", err)
		}
		count := 0
		var walk func(bs []*doc.Bookmark, level int)
		walk = func(bs []*doc.Bookmark, level int) {
			if level > d.lim.MaxDepth {
				return
			}
			for _, b := range bs {
				if count >= d.lim.MaxObjects {
					return
				}
				// an entry that leads to no page is left out, and its
				// children stay
				if i, _ := n.targets.Resolve(b.URL, 0); i >= 0 {
					n.marks[i] = append(n.marks[i], mark{level: level, label: b.Title})
					count++
				}
				walk(b.Children, level+1)
			}
		}
		walk(bookmarks, 1)
		d.nav = n
	})
	return d.nav
}

// navm reads the data of the NAVM chunk of the document, or returns nil
// when the document has none. The chunk is a chunk of the FORM:DJVM chunk,
// next to DIRM.
func (b *bundle) navm() ([]byte, error) {
	var head [8]byte
	if n, _ := b.src.ReadAt(head[:], b.off); n < len(head) {
		return nil, nil
	}
	end := min(b.src.Size(), b.off+8+int64(binary.BigEndian.Uint32(head[4:])))
	// the chunks of FORM:DJVM, after its type, one for each component and
	// a few more
	for p, i := b.off+12, 0; p+8 <= end && i <= b.lim.MaxObjects+8; i++ {
		if n, _ := b.src.ReadAt(head[:], p); n < len(head) {
			return nil, nil
		}
		size := int64(binary.BigEndian.Uint32(head[4:]))
		if string(head[:4]) == "NAVM" {
			_, data, err := readChunk(b.src, p, b.lim)
			return data, err
		}
		p += 8 + size + size&1
	}
	return nil, nil
}

// drawLinks gives d the bookmarks, the anchor and the links of the page,
// when d is a display.Linker or a display.Bookmarker. Each page has an
// anchor at its top, display.PageAnchor(n, 0, 0), which a link to the page
// targets. Only a writer, such as pdfenc, takes them, so the pixels of
// Render do not change. A damaged outline or annotation is logged and left
// out. drawLinks returns an error only for a panic.
func (p *Page) drawLinks(ctx context.Context, d display.Device) (err error) {
	_, linker := d.(display.Linker)
	_, marker := d.(display.Bookmarker)
	if !linker && !marker {
		return nil
	}
	defer recoverTo(fmt.Sprintf("drawing the links of djvu page %d", p.n+1), &err)
	nav := p.d.navigation()
	if marker {
		for _, m := range nav.marks[p.n] {
			display.Bookmark(d, m.level, m.label, 0, 0, true)
		}
	}
	if !linker {
		return nil
	}
	display.Anchor(d, display.PageAnchor(p.n, 0, 0), 0, 0)
	if p.p.Err != nil || ctx.Err() != nil {
		return nil
	}
	_, pp, err := p.load()
	if err != nil {
		p.d.log.Debug("djvu: page without links", "page", p.n+1, "err", err)
		return nil
	}
	exprs, err := pp.Annotations()
	if err != nil {
		p.d.log.Debug("djvu: damaged annotations", "page", p.n+1, "err", err)
	}
	in := pp.Info
	if in.Width <= 0 || in.Height <= 0 || in.DPI <= 0 {
		return nil
	}
	// a maparea is in the pixels of the page with y up, and the user space
	// of the page before its rotation is in points with y down
	s := 72 / float64(in.DPI)
	h := float64(in.Height)
	w, hp := float32(float64(in.Width)*s), float32(h*s)
	d.Save()
	defer d.Restore()
	d.Concat(rotation(in.Rotate, w, hp))
	links := 0
	for _, e := range exprs {
		if links >= p.d.lim.MaxObjects {
			p.d.log.Debug("djvu: too many links", "page", p.n+1, "max", p.d.lim.MaxObjects)
			return nil
		}
		a, ok := doc.MapAreaOf(e)
		if !ok {
			continue
		}
		r, ok := okage.RectFromLTRB(float32(a.Rect[0]*s), float32((h-a.Rect[3])*s),
			float32(a.Rect[2]*s), float32((h-a.Rect[1])*s))
		if !ok {
			continue
		}
		i, uri := nav.targets.Resolve(a.URL, p.n)
		switch {
		case i >= 0:
			display.LinkAnchor(d, r, display.PageAnchor(i, 0, 0))
		case uri != "":
			display.LinkURI(d, r, uri)
		default:
			continue
		}
		links++
	}
	return nil
}
