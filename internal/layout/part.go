// This file is not a port. It lays out a part of a long document, such as
// a part of a long sheet, which drops the pages before its cut row and from
// its probe row (D55). Upstream lays out each document whole.

package layout

import "context"

// partState is the state of a Document that is a part of a long document.
type partState struct {
	// cut and probe are Part.Cut and Part.Probe.
	cut, probe int
	// index holds the index of each element tr of the document, in tree
	// order.
	index map[*Element]int
	// pages holds the rows of the pages of the current pass that span
	// read.
	pages []pageRows
}

// pageRows are the rows of the body of a table on a page: the rows that
// are not rows of a header or of a footer. first and last are the indexes
// of the first row and of the last row, or -1 when the page has none.
// continued is set when the first row of the page is the last row of the
// page before: the page starts in the middle of the row.
type pageRows struct {
	first, last int
	continued   bool
}

// starts reports whether the page starts at the row of the index row.
func (r pageRows) starts(row int) bool {
	return r.first == row && r.first >= 0 && !r.continued
}

// newPartState returns the state of the part p of the document whose root
// element is root.
func newPartState(p *Part, root *Element) *partState {
	s := &partState{cut: p.Cut, probe: p.Probe, index: map[*Element]int{}}
	var walk func(e *Element)
	walk = func(e *Element) {
		if e.Tag == "tr" {
			s.index[e] = len(s.index)
		}
		for _, child := range e.Children {
			walk(child)
		}
	}
	if root != nil {
		walk(root)
	}
	return s
}

// rowsOf returns the rows of the body of the tables of page.
func (s *partState) rowsOf(page *Box) pageRows {
	r := pageRows{first: -1, last: -1}
	var walk func(b *Box)
	walk = func(b *Box) {
		for _, child := range b.Children {
			switch child.Kind {
			case KindTableRowGroup:
				if child.table().IsHeader || child.table().IsFooter {
					continue
				}
			case KindTableRow:
				if i, ok := s.index[child.Element]; ok && child.Element != nil {
					if r.first < 0 {
						r.first = i
					}
					r.last = i
				}
				continue
			case KindTableCell:
				continue
			}
			walk(child)
		}
	}
	walk(page)
	return r
}

// span returns the pages of the part among the first n pages of the
// current pass, without a new layout: the index of the first page that it
// keeps, and the index after the last one. end is set when the n pages
// hold the end of the part: the page that holds the probe row. Before the
// end, last is n. ok is set when the probe row starts the page that holds
// it. row is the first row of the page after the last page that the part
// keeps, or -1 for the cut row.
//
// When the page that holds the probe row starts in the middle of a row,
// the part ends at the last page before it that starts at a row, so that
// the next part can start at that row.
func (d *Document) span(n int) (first, last int, end, ok bool, row int) {
	s := d.part
	for k := len(s.pages); k < n; k++ {
		r := s.rowsOf(d.pages[k])
		r.continued = k > 0 && r.first >= 0 && s.pages[k-1].last == r.first
		s.pages = append(s.pages, r)
	}
	first = -1
	for k := range n {
		r := s.pages[k]
		if first < 0 {
			if s.cut >= 0 && !r.starts(s.cut) {
				continue
			}
			first = k
		}
		if s.probe < 0 || r.first < 0 || r.last < s.probe {
			continue
		}
		// The page k holds the probe row.
		if r.starts(s.probe) {
			return first, k, true, true, s.probe
		}
		for q := k; q > first; q-- {
			if s.pages[q].first >= 0 && !s.pages[q].continued {
				return first, q, true, false, s.pages[q].first
			}
		}
		return first, first, true, false, -1
	}
	if first < 0 {
		// No page starts at the cut row yet.
		return n, n, false, false, -1
	}
	return first, n, false, false, -1
}

// partEnd lays out the pages of the part until it knows whether it keeps
// its page of index want, or the whole part when want is negative. It
// returns what span returns for the pages that it laid out.
//
// A page is kept only when the end of the part is known, or when a page
// after it starts at a row: the end of the part can only move back to
// such a page.
func (d *Document) partEnd(want int) (first, last int, ok bool, row int) {
	for {
		pass := d.loop
		complete := d.complete
		n := len(d.pages)
		if n > 0 || complete {
			first, last, end, ok, row := d.span(n)
			switch {
			case end:
				return first, last, ok, row
			case complete && d.part.probe < 0:
				// The last part keeps its pages to the end.
				return first, last, true, -1
			case complete:
				// No page holds the probe row: the part keeps no page, and
				// the next part starts at the cut row.
				return first, first, false, -1
			case want >= 0 && d.part.probe < 0 && first+want < n:
				// The last part has no end that can move back.
				return first, last, ok, row
			case want >= 0 && first+want < n-1 && d.startsAfter(first+want, n):
				return first, last, ok, row
			}
		}
		d.layoutTo(n)
		if d.loop != pass || d.complete != complete {
			// A new pass can replace the pages.
			continue
		}
		if len(d.pages) == n && !d.complete {
			// The pass gives no more pages: finish the layout.
			d.completeLayout()
		}
	}
}

// startsAfter reports whether a page after the page of index k, among the
// first n pages that span read, starts at a row.
func (d *Document) startsAfter(k, n int) bool {
	for _, r := range d.part.pages[k+1 : n] {
		if r.first >= 0 && !r.continued {
			return true
		}
	}
	return false
}

// partPage returns the finished page of index i among the pages that the
// part keeps, or nil when the part keeps fewer pages.
func (d *Document) partPage(i int) *Box {
	first, last, _, _ := d.partEnd(i)
	if first+i >= last {
		return nil
	}
	return d.finish(first + i)
}

// partFirst returns the index of the first page that the part keeps, which
// partEnd found before.
func (d *Document) partFirst() int {
	first, _, _, _, _ := d.span(len(d.pages))
	return first
}

// Probe lays out the whole part and reports whether its probe row starts a
// page. When it does not, row is the first row of the page after the last
// page that the part keeps, or -1 for the cut row (D55). A document that is
// not a part reports true.
func (d *Document) Probe(ctx context.Context) (ok bool, row int, err error) {
	if d.part == nil {
		return true, -1, nil
	}
	err = d.run(ctx, func() {
		_, _, ok, row = d.partEnd(-1)
	})
	return ok, row, err
}

// totalPages returns the value of the counter pages for the pages of the
// current pass: the pages before the document, the pages of the document
// and the pages after it. A part of a long document counts only the pages
// that it keeps.
func (d *Document) totalPages() int {
	kept := len(d.pages)
	if d.part != nil {
		first, last, end, _, _ := d.span(len(d.pages))
		kept = last - first
		if !end && d.part.probe >= 0 {
			kept = 0
		}
	}
	return d.pagesBefore + kept + d.pagesAfter
}

// setPagesAfter sets the number of pages after the document, which the
// counter pages counts. The document must use the counter pages only in
// its margin boxes: the number of pages does not change the layout of its
// content, so the pages stay, and only their margin boxes change (D55).
func (d *Document) setPagesAfter(n int) {
	if d.pagesAfter == n {
		return
	}
	d.pagesAfter = n
	if !d.complete {
		// The end of the pass sets the counter.
		return
	}
	total := d.totalPages()
	for _, pageData := range d.lc.pageMaker {
		pageData.counterValues["pages"] = []int{total}
	}
	clear(d.finished)
	d.collected = 0
	d.watch = [3][]*Element{}
	d.lc.stringSet = map[string]map[int][]string{}
}
