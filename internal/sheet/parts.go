package sheet

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
)

// A long sheet is laid out in parts (D55). The writer writes the HTML of the
// whole sheet once, and keeps the offset of each row of each table. A part
// is a slice of these rows, which the layout lays out as a document of its
// own. Its document holds the start of the document, the start of each of
// its tables up to the tag tbody, and its rows:
//
//  1. When the part starts inside a table, a copy of the first row of the
//     body of the table without its rowspan attributes, then the lead rows,
//     then the cut row with a page break before it. The table takes the
//     side borders of its first row, and the lead rows give the borders
//     and the merged cells above the cut row. The layout drops the pages
//     before the cut row.
//  2. The rows of the part.
//  3. When the part ends inside a table, the probe row, the first row of
//     the next part, and the tail rows after it. They give the borders and
//     the merged cells below the last row that the part keeps. The layout
//     drops the page that holds the probe row and the pages after it.
//
// A table starts a new page, so a part that starts with a table has no
// lead rows, and a part that ends before a table has no probe row. The last
// part holds the end of the document: the comments and the note.

// defaultPartPages is the number of pages of a part.
const defaultPartPages = 16

// maxEdgeRows is the number of lead rows or tail rows of a cut. The writer
// does not cut where a merged cell needs more.
const maxEdgeRows = 256

// partPagesKey is the key of the number of pages of a part in a context.
type partPagesKey struct{}

// WithPartPages returns a context in which Source cuts a long sheet into
// parts of about n pages, and makes no part for a sheet of fewer than 2n
// pages. A number below 1 makes no part. Without it, a part has about 16
// pages. The tests use it to compare the pages of a whole sheet with the
// pages of its parts.
func WithPartPages(ctx context.Context, n int) context.Context {
	return context.WithValue(ctx, partPagesKey{}, n)
}

// partPages returns the number of pages of a part in ctx.
func partPages(ctx context.Context) int {
	if n, ok := ctx.Value(partPagesKey{}).(int); ok {
		return n
	}
	return defaultPartPages
}

// tableSpan is a table of the body of the document of a sheet.
type tableSpan struct {
	// start, body, rowsEnd and end are offsets in the body: the start of
	// the table, the end of its start tag tbody, the start of its end tag
	// tbody and the end of the table.
	start, body, rowsEnd, end int
	// heads is the number of rows in its head, head their height and
	// headTop the width of the border at their top, in points.
	heads         int
	head, headTop float64
	// rows are the rows of its body.
	rows []rowSpan
}

// rowSpan is a row of the body of a table.
type rowSpan struct {
	// off is the offset of the row in the body of the document.
	off int
	// h is the height of the row in points, as the document gives it.
	h float64
	// top and bottom are the widths of the widest borders at the top and
	// at the bottom of its cells, in points.
	top, bottom float64
	// first and last are the first and the last row of the merged cells
	// that hold the row, or -1 when no merged cell holds it.
	first, last int
	// brk is set when the row starts a page by a manual break.
	brk bool
	// inTall is set when the row is a row of an image that shows whole in
	// its first cell, after its first row.
	inTall bool
}

// holdRows sets the rows of the merged cells that hold each row.
func (t *tableSpan) holdRows(merges []*mergeState) {
	for _, m := range merges {
		if m.first < 0 {
			continue
		}
		for k := m.first; k <= m.last && k < len(t.rows); k++ {
			r := &t.rows[k]
			if r.first < 0 || m.first < r.first {
				r.first = m.first
			}
			r.last = max(r.last, m.last)
		}
	}
}

// round2 rounds v to 2 decimals, as pt does.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// inTall reports whether the row i is a row of an image that shows whole
// in its first cell, after its first row.
func (w *writer) inTall(i int) bool {
	for _, n := range w.tall {
		if w.images[n].Row < i && i <= w.ends[n][0] {
			return true
		}
	}
	return false
}

// borders adds the borders of a cell of the style of the index style to
// the widest borders of the row.
func (w *writer) borders(style int) {
	if w.rec == nil {
		return
	}
	st := w.style(style)
	side := func(k int) float64 {
		if bd := st.Borders[k]; bd.Width > 0 {
			return round2(borderWidth(bd, w.scale))
		}
		if w.sh.Gridlines {
			return round2(max(0.5, 0.5*w.scale))
		}
		return 0
	}
	w.top = max(w.top, side(0))
	w.bottom = max(w.bottom, side(2))
}

// pos is a place in the tables of a sheet: the row r of the table t. The
// row 0 is the start of the table, and the table len(tables) is the end of
// the document.
type pos struct {
	t, r int
}

// parted is a sheet that the layout lays out in parts.
type parted struct {
	// name is the path of the document of the whole sheet, and base the
	// start of the paths of its parts.
	name, base string
	// head is the start of the document, up to the start tag of its body.
	// body is the body, and tail the offset of the end of its last table.
	head, body string
	tail       int
	tables     []tableSpan
	// parts are the parts of the sheet, and pages the number of pages that
	// the writer expects each one to keep.
	parts []pos
	pages []int
}

// split returns the parts of the document of the sheet, which the writer
// wrote, or nil when the sheet has fewer than 2n pages.
func (w *writer) split(name string, n int) *parted {
	if n < 1 || len(w.tables) == 0 {
		return nil
	}
	p := &parted{
		name:   name,
		base:   strings.TrimSuffix(name, ".html"),
		head:   w.headHTML,
		body:   w.body.String(),
		tail:   w.tables[len(w.tables)-1].end,
		tables: w.tables,
	}
	avail := w.pageHeight()
	since, total := 0, 0
	for t := range p.tables {
		tb := &p.tables[t]
		used := 0.0
		top := tb.headTop
		for r := range tb.rows {
			row := &tb.rows[r]
			if r > 0 && !tb.brkOrFull(r, used, top, avail) {
				used += row.h
				continue
			}
			// The row r starts a page.
			if since >= n && (r == 0 || p.cuttable(pos{t, r})) {
				p.parts = append(p.parts, pos{t, r})
				p.pages = append(p.pages, since)
				since = 0
			}
			if len(p.parts) == 0 {
				p.parts = append(p.parts, pos{t, 0})
			}
			since++
			total++
			used = row.h
			top = tb.headTop
			if tb.heads == 0 {
				top = max(row.top, tb.above(r))
			}
		}
		if len(tb.rows) == 0 {
			// A table without rows shows its head on a page.
			if len(p.parts) == 0 {
				p.parts = append(p.parts, pos{t, 0})
			}
			since++
			total++
		}
	}
	p.pages = append(p.pages, since)
	if total < 2*n || len(p.parts) < 2 {
		return nil
	}
	return p
}

// above returns the width of the border at the bottom of the row before
// the row r, or 0.
func (t *tableSpan) above(r int) float64 {
	if r == 0 {
		return 0
	}
	return t.rows[r-1].bottom
}

// brkOrFull reports whether the row r starts a page: by a manual break, or
// because it does not fit on the page whose rows take used points, with
// a border of the width top at the top of the page. The page holds the
// head, and the halves of the borders at the top and at the bottom of the
// table. avail is the height of the area of the page.
func (t *tableSpan) brkOrFull(r int, used, top, avail float64) bool {
	row := &t.rows[r]
	if row.brk {
		return true
	}
	bottom := t.rows[len(t.rows)-1].bottom
	need := top/2 + t.head + used + row.h + bottom/2
	return need > avail*(1+1e-9)+1e-6
}

// pageHeight returns the height of the area of a page, in points, as the
// rule @page gives it.
func (w *writer) pageHeight() float64 {
	p := w.sh.Page
	_, ph := w.paper()
	top := max(0, p.Top) + max(p.HeaderArea, 0)*w.areaScale()
	bottom := max(0, p.Bottom) + max(p.FooterArea, 0)*w.areaScale()
	return round2(ph) - round2(top) - round2(bottom)
}

// cuttable reports whether a part can start at the row c.r of the table
// c.t, which is not its first row: the lead rows and the tail rows of the
// cut are at most maxEdgeRows rows, and the row is not a row of an image
// that shows whole in its first cell, after its first row.
func (p *parted) cuttable(c pos) bool {
	tb := &p.tables[c.t]
	if tb.rows[c.r].inTall {
		return false
	}
	lead, tail := tb.edges(c.r)
	return c.r-lead <= maxEdgeRows && tail-c.r <= maxEdgeRows
}

// edges returns the first lead row of a cut at the row r of the table, and
// the last tail row: the row before r, from one row before the first row
// of each merged cell that holds it, and the row r, up to one row after
// the last row of each merged cell that holds it.
func (t *tableSpan) edges(r int) (lead, tail int) {
	lead = r - 1
	if f := t.rows[r-1].first; f >= 0 {
		lead = max(0, min(lead, f-1))
	}
	tail = r
	if l := t.rows[r].last; l >= 0 {
		tail = min(len(t.rows)-1, max(tail, l+1))
	}
	return lead, tail
}

// row returns the HTML of the row r of the table t.
func (p *parted) row(t, r int) string {
	tb := &p.tables[t]
	end := tb.rowsEnd
	if r+1 < len(tb.rows) {
		end = tb.rows[r+1].off
	}
	return p.body[tb.rows[r].off:end]
}

// path returns the path of the part from the place from to the place to.
func (p *parted) path(from, to pos) string {
	return p.base + ".part." + strconv.Itoa(from.t) + "." + strconv.Itoa(from.r) + "." +
		strconv.Itoa(to.t) + "." + strconv.Itoa(to.r) + ".html"
}

// places returns the places of the part of the path name, and false when
// name is not the path of a part of the sheet.
func (p *parted) places(name string) (from, to pos, ok bool) {
	rest, ok := strings.CutPrefix(name, p.base+".part.")
	if !ok {
		return pos{}, pos{}, false
	}
	rest, ok = strings.CutSuffix(rest, ".html")
	if !ok {
		return pos{}, pos{}, false
	}
	fields := strings.Split(rest, ".")
	if len(fields) != 4 {
		return pos{}, pos{}, false
	}
	var v [4]int
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return pos{}, pos{}, false
		}
		v[i] = n
	}
	from, to = pos{v[0], v[1]}, pos{v[2], v[3]}
	if !p.valid(from) || !p.valid(to) || !before(from, to) {
		return pos{}, pos{}, false
	}
	return from, to, true
}

// valid reports whether c is a place in the tables of the sheet.
func (p *parted) valid(c pos) bool {
	if c.t == len(p.tables) {
		return c.r == 0
	}
	return c.t < len(p.tables) && (c.r == 0 || c.r < len(p.tables[c.t].rows))
}

// before reports whether the place a comes before the place b.
func before(a, b pos) bool {
	return a.t < b.t || (a.t == b.t && a.r < b.r)
}

// part returns the part from the place from to the place to, which keeps
// about pages pages.
func (p *parted) part(from, to pos, pages int) layout.Part {
	cut, probe := p.write(nil, from, to, nil)
	return layout.Part{Path: p.path(from, to), Cut: cut, Probe: probe, Pages: pages}
}

// write writes the document of the part from the place from to the place
// to into b, when b is not nil. It returns the indexes of its cut row and
// of its probe row among its rows, or -1. When places is not nil, it holds
// the place of each row of the document, or a place with a negative table
// for a row that the part does not keep.
func (p *parted) write(b *strings.Builder, from, to pos, places *[]pos) (cut, probe int) {
	cut, probe = -1, -1
	rows := 0
	add := func(s string, at pos) {
		if b != nil {
			b.WriteString(s)
		}
		if places != nil {
			*places = append(*places, at)
		}
		rows++
	}
	none := pos{-1, -1}
	if b != nil {
		b.WriteString(p.head)
	}
	for t := from.t; t < len(p.tables) && before(pos{t, 0}, to); t++ {
		tb := &p.tables[t]
		r0, r1 := 0, len(tb.rows)
		if t == from.t {
			r0 = from.r
		}
		if t == to.t {
			r1 = to.r
		}
		if b != nil {
			b.WriteString(p.body[tb.start:tb.body])
		}
		for range tb.heads {
			if places != nil {
				*places = append(*places, none)
			}
			rows++
		}
		if r0 > 0 {
			add(noRowspan(p.row(t, 0)), none)
			lead, _ := tb.edges(r0)
			for r := lead; r < r0; r++ {
				add(p.row(t, r), none)
			}
			cut = rows
			add(breakBefore(p.row(t, r0)), pos{t, r0})
			r0++
		}
		for r := r0; r < r1; r++ {
			add(p.row(t, r), pos{t, r})
		}
		if r1 < len(tb.rows) {
			probe = rows
			_, tail := tb.edges(r1)
			for r := r1; r <= tail; r++ {
				add(p.row(t, r), none)
			}
		}
		if b != nil {
			b.WriteString(p.body[tb.rowsEnd:tb.end])
		}
	}
	if b != nil {
		if to.t >= len(p.tables) {
			b.WriteString(p.body[p.tail:])
		}
		b.WriteString("</body></html>\n")
	}
	return cut, probe
}

// noRowspan returns the HTML of a row without its rowspan attributes. The
// text of a cell cannot hold them, because the writer escapes the quotes.
func noRowspan(row string) string {
	const attr = ` rowspan="`
	var b strings.Builder
	for {
		i := strings.Index(row, attr)
		if i < 0 {
			break
		}
		b.WriteString(row[:i])
		row = row[i+len(attr):]
		if j := strings.IndexByte(row, '"'); j >= 0 {
			row = row[j+1:]
		}
	}
	b.WriteString(row)
	return b.String()
}

// breakBefore returns the HTML of a row with a page break before it.
func breakBefore(row string) string {
	if strings.HasPrefix(row, `<tr class="pb"`) {
		return row
	}
	return `<tr class="pb"` + strings.TrimPrefix(row, "<tr")
}

// splitter holds the long sheets of a workbook. It is the Splitter of the
// source, and the file system of the source, which gives the documents of
// the parts. It is safe for concurrent use.
type splitter struct {
	fs     *memfs.FS
	sheets map[string]*parted
	// recuts counts the calls of Recut, for the tests.
	recuts atomic.Int64
}

// The interfaces that splitter satisfies.
var (
	_ layout.Splitter = (*splitter)(nil)
	_ fs.FS           = (*splitter)(nil)
)

// Split satisfies the layout.Splitter interface.
func (s *splitter) Split(path string) []layout.Part {
	p := s.sheets[path]
	if p == nil {
		return nil
	}
	parts := make([]layout.Part, len(p.parts))
	for i, from := range p.parts {
		to := pos{len(p.tables), 0}
		if i+1 < len(p.parts) {
			to = p.parts[i+1]
		}
		parts[i] = p.part(from, to, p.pages[i])
	}
	return parts
}

// Recut satisfies the layout.Splitter interface.
func (s *splitter) Recut(part, next layout.Part, row int) layout.Part {
	s.recuts.Add(1)
	p := s.sheetOf(part.Path)
	if p == nil {
		return next
	}
	from, _, ok := p.places(part.Path)
	_, to, nextOK := p.places(next.Path)
	if !ok || !nextOK {
		return next
	}
	if row >= 0 {
		var places []pos
		_, end, _ := p.places(part.Path)
		p.write(nil, from, end, &places)
		if row < len(places) && places[row].t >= 0 && before(from, places[row]) && before(places[row], to) {
			from = places[row]
		}
	}
	return p.part(from, to, next.Pages+1)
}

// sheetOf returns the sheet of the path of a part, or nil.
func (s *splitter) sheetOf(name string) *parted {
	base, _, ok := strings.Cut(name, ".part.")
	if !ok {
		return nil
	}
	return s.sheets[base+".html"]
}

// Open satisfies the fs.FS interface. It gives the document of a part, and
// the files of the workbook.
func (s *splitter) Open(name string) (fs.File, error) {
	if p := s.sheetOf(name); p != nil {
		if from, to, ok := p.places(name); ok {
			var b strings.Builder
			p.write(&b, from, to, nil)
			return &partFile{Reader: bytes.NewReader([]byte(b.String())), name: name}, nil
		}
	}
	f, err := s.fs.Open(name)
	if err != nil {
		return nil, fmt.Errorf("opening %s of the workbook: %w", name, err)
	}
	return f, nil
}

// partFile is the document of a part.
type partFile struct {
	*bytes.Reader
	name string
}

// Stat satisfies the fs.File interface.
func (f *partFile) Stat() (fs.FileInfo, error) { return f, nil }

// Close satisfies the fs.File interface.
func (f *partFile) Close() error { return nil }

// Name satisfies the fs.FileInfo interface.
func (f *partFile) Name() string { return f.name[strings.LastIndexByte(f.name, '/')+1:] }

// Mode satisfies the fs.FileInfo interface.
func (f *partFile) Mode() fs.FileMode { return 0o444 }

// ModTime satisfies the fs.FileInfo interface.
func (f *partFile) ModTime() time.Time { return time.Time{} }

// IsDir satisfies the fs.FileInfo interface.
func (f *partFile) IsDir() bool { return false }

// Sys satisfies the fs.FileInfo interface.
func (f *partFile) Sys() any { return nil }
