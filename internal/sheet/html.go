package sheet

import (
	"context"
	"fmt"
	"html"
	"image/color"
	"log/slog"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/memfs"
)

// Options are the options of Source.
type Options struct {
	// Limits bound the work. A zero field takes its default.
	Limits ivy.Limits
	// Logger receives the log, or nil.
	Logger *slog.Logger
	// Paper is the paper of the empty page of a workbook without sheets,
	// the paper of ivy.Config. The zero Paper keeps DefaultPage (D32).
	Paper ivy.Paper
}

// maxImages is the number of images and charts that a sheet shows.
const maxImages = 4096

// The padding of a cell at the left and the right, in points, as Excel
// draws it at 100%.
const cellPadding = 1.5

// gridColor is the color of the gridlines: light, so that they do not hide
// the borders of the cells.
const gridColor = "#c8c8c8"

// Source converts the workbook into the source of the layout: one HTML
// document for each sheet that prints something, in order. A workbook that
// prints nothing gives one empty page.
//
// A sheet shows at most Limits.MaxObjects / 4 cells and rows in all, with
// the bands of its columns. At the limit, the sheet stops, its last line
// says so, and the log has a warning. The sheets after it still show.
//
// A sheet of 32 pages or more is also cut into parts of about 16 pages,
// which the book lays out on their own (D55). The Splitter of the source
// gives the parts, and its FS gives their documents.
func (wb *Workbook) Source(ctx context.Context, opts Options) (layout.Source, error) {
	lim := opts.Limits.WithDefaults()
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	var fsys memfs.FS
	src := layout.Source{FS: &fsys, Title: wb.Title, Objects: wb.Objects}
	parts := partPages(ctx)
	sheets := map[string]*parted{}
	for name, data := range wb.Files {
		fsys.Add(name, data)
	}
	for i, sh := range wb.Sheets {
		if err := ctx.Err(); err != nil {
			return layout.Source{}, fmt.Errorf("converting sheet %q: %w", sh.Name, err)
		}
		w := newWriter(ctx, wb, sh, lim)
		doc, ok, err := w.document()
		if err != nil {
			return layout.Source{}, err
		}
		if !ok {
			continue
		}
		if w.stopped {
			log.Warn("sheet: sheet cut at the limit of the cells", "sheet", sh.Name, "max", w.max)
		}
		name := "sheet" + strconv.Itoa(i+1) + ".html"
		fsys.Add(name, []byte(doc))
		if p := w.split(name, parts); p != nil {
			sheets[name] = p
		}
		for path, data := range w.files {
			fsys.Add(path, data)
		}
		src.Documents = append(src.Documents, name)
	}
	if len(src.Documents) == 0 {
		// A workbook that prints nothing gives one empty page, of the
		// page of its first sheet.
		page := DefaultPage
		page.SetPaper(opts.Paper)
		if len(wb.Sheets) > 0 {
			page = wb.Sheets[0].Page
		}
		w := &writer{wb: wb, sh: &Sheet{Page: page}}
		var b strings.Builder
		b.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><style>")
		w.pageRule(&b)
		b.WriteString("</style></head><body></body></html>")
		fsys.Add("empty.html", []byte(b.String()))
		src.Documents = append(src.Documents, "empty.html")
	}
	if len(sheets) > 0 {
		// A long sheet is laid out in parts (D55).
		s := &splitter{fs: &fsys, sheets: sheets}
		src.FS, src.Splitter = s, s
	}
	return src, nil
}

// writer writes the HTML document of one sheet.
type writer struct {
	ctx context.Context
	wb  *Workbook
	sh  *Sheet
	// used marks the styles that the cells use.
	used map[int]bool
	// scale is the scale of the cells.
	scale float64
	// nodes counts the elements and the texts of the document, and max
	// bounds them.
	nodes, max int
	// stopped is set when the sheet stopped at max.
	stopped bool
	// hidden holds the indexes of the hidden rows, sorted.
	hidden []int
	// rowBreaks and colBreaks are the manual breaks.
	rowBreaks, colBreaks map[int]bool
	// images are the images of the sheet, at most maxImages, and ends
	// are the cells of their bottom right corners.
	images []Image
	ends   [][2]int
	// cond holds what the conditional formats make of the cells, by their
	// row and their column. extra holds the styles that they make, after
	// the styles of the workbook, and derived their indexes.
	cond    map[[2]int]*condResult
	extra   []Style
	derived map[derivedKey]int
	// files holds the images that the document adds, such as the icons
	// of the conditional formats.
	files map[string][]byte
	body  strings.Builder
	// tables holds the tables of body, for the parts of a long sheet. rec
	// is the table whose rows the writer records, or nil. top and bottom
	// are the widest borders at the top and at the bottom of the cells of
	// the row that the writer writes (D55).
	tables      []tableSpan
	rec         *tableSpan
	top, bottom float64
	// tall holds the images that show whole in their first cell, because
	// they are taller than maxSliceRows rows.
	tall []int
	// headHTML is the start of the document, up to the start tag of its
	// body.
	headHTML string
}

// derivedKey names a style that conditional formats make: the style of
// the cell and what they change.
type derivedKey struct {
	style int
	diffs string
	fill  color.NRGBA
}

// newWriter returns the writer of sh.
func newWriter(ctx context.Context, wb *Workbook, sh *Sheet, lim ivy.Limits) *writer {
	w := &writer{
		ctx:       ctx,
		wb:        wb,
		sh:        sh,
		used:      make(map[int]bool),
		max:       max(64, lim.MaxObjects/4),
		rowBreaks: make(map[int]bool),
		colBreaks: make(map[int]bool),
	}
	if sh.DefaultColWidth <= 0 {
		sh.DefaultColWidth = 48
	}
	if sh.DefaultRowHeight <= 0 {
		sh.DefaultRowHeight = 15
	}
	for _, r := range sh.Rows {
		if sh.rowHeight(&r) == 0 {
			w.hidden = append(w.hidden, r.Index)
		}
	}
	for _, i := range sh.RowBreaks {
		w.rowBreaks[i] = true
	}
	for _, i := range sh.ColBreaks {
		w.colBreaks[i] = true
	}
	w.cond = sh.evalCond()
	w.images = sh.Images[:min(len(sh.Images), maxImages)]
	w.ends = make([][2]int, len(w.images))
	for i, img := range w.images {
		w.ends[i][0], w.ends[i][1] = w.imageEnd(img)
		if w.ends[i][0]-img.Row >= maxSliceRows {
			w.tall = append(w.tall, i)
		}
	}
	return w
}

// style returns the style of the index i, or the default style.
func (w *writer) style(i int) *Style {
	if i > 0 && i < len(w.wb.Styles) {
		return &w.wb.Styles[i]
	}
	if base := max(1, len(w.wb.Styles)); i >= base && i-base < len(w.extra) {
		return &w.extra[i-base]
	}
	if len(w.wb.Styles) > 0 {
		return &w.wb.Styles[0]
	}
	return &DefaultStyle
}

// usedRange returns the range of the cells that hold data or a visible
// style, the merged cells and the images.
func (w *writer) usedRange() (Range, bool) {
	r := Range{Row0: math.MaxInt, Col0: math.MaxInt, Row1: -1, Col1: -1}
	add := func(row0, col0, row1, col1 int) {
		r.Row0, r.Col0 = min(r.Row0, row0), min(r.Col0, col0)
		r.Row1, r.Col1 = max(r.Row1, row1), max(r.Col1, col1)
	}
	for _, row := range w.sh.Rows {
		for _, c := range row.Cells {
			if c.Text != "" || len(c.Runs) > 0 || w.style(c.Style).visible() {
				add(row.Index, c.Col, row.Index, c.Col)
			}
		}
	}
	for _, m := range w.sh.Merges {
		add(m.Row0, m.Col0, m.Row1, m.Col1)
	}
	for i, img := range w.images {
		add(img.Row, img.Col, w.ends[i][0], w.ends[i][1])
	}
	return r, r.Row1 >= 0
}

// imageEnd returns the cell of the bottom right corner of an image.
func (w *writer) imageEnd(img Image) (int, int) {
	col, x := img.Col, img.DX+img.Width
	for range 1024 {
		cw := w.sh.colWidth(col)
		if x <= cw || col >= MaxCols-1 {
			break
		}
		x -= cw
		col++
	}
	row, y := img.Row, img.DY+img.Height
	for range 4096 {
		rh := w.sh.rowHeight(w.sh.row(row))
		if y <= rh || row >= MaxRows-1 {
			break
		}
		y -= rh
		row++
	}
	return row, col
}

// clip returns r within the size of a sheet, and false if it is empty.
func clip(r Range) (Range, bool) {
	r.Row0, r.Col0 = max(r.Row0, 0), max(r.Col0, 0)
	r.Row1, r.Col1 = min(r.Row1, MaxRows-1), min(r.Col1, MaxCols-1)
	return r, r.Row0 <= r.Row1 && r.Col0 <= r.Col1
}

// document returns the HTML document of the sheet, and false if the sheet
// prints nothing.
func (w *writer) document() (string, bool, error) {
	ranges := w.sh.PrintAreas
	if len(ranges) == 0 {
		used, ok := w.usedRange()
		if !ok {
			return "", false, nil
		}
		ranges = []Range{used}
	}
	first := true
	for _, r := range ranges {
		r, ok := clip(r)
		if !ok {
			continue
		}
		if err := w.printRange(r); err != nil {
			return "", false, err
		}
		first = false
		if w.stopped {
			break
		}
	}
	if first {
		return "", false, nil
	}
	if w.stopped {
		w.body.WriteString(`<p class="note">`)
		w.body.WriteString(html.EscapeString(fmt.Sprintf("The sheet continues past the limit of %d cells, which ivy does not show.", w.max)))
		w.body.WriteString("</p>")
	}
	w.comments()
	if w.sh.Note != "" {
		w.body.WriteString(`<p class="note">`)
		w.body.WriteString(html.EscapeString(w.sh.Note))
		w.body.WriteString("</p>")
	}
	w.headHTML = w.head()
	var b strings.Builder
	b.WriteString(w.headHTML)
	b.WriteString(w.body.String())
	b.WriteString("</body></html>\n")
	return b.String(), true, nil
}

// head returns the start of the document, up to the start tag of its body.
func (w *writer) head() string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(w.sh.Name))
	b.WriteString("</title><style>\n")
	w.css(&b)
	b.WriteString("</style></head><body>")
	return b.String()
}

// comments writes the comments of the cells on pages after the cells, as
// Excel prints them at the end of a sheet: the cell, the author and the
// text of each comment, in the order of the rows.
func (w *writer) comments() {
	sh := w.sh
	if !sh.Page.CommentsAtEnd || len(sh.Comments) == 0 || w.stopped {
		return
	}
	list := slices.Clone(sh.Comments)
	slices.SortStableFunc(list, func(a, b Comment) int {
		if a.Row != b.Row {
			return a.Row - b.Row
		}
		return a.Col - b.Col
	})
	def := w.style(0).Font
	var font strings.Builder
	fontCSS(&font, def, def, 1)
	w.body.WriteString(`<div class="cm" style="` + html.EscapeString(font.String()) + `">`)
	for _, c := range list {
		if !w.spend(2) {
			break
		}
		text := c.Text
		if c.Author != "" {
			text = c.Author + ":\n" + text
		}
		w.body.WriteString("<p>Cell: " + html.EscapeString(CellName(c.Row, c.Col)) + "\nComment: " + html.EscapeString(text) + "</p>")
	}
	w.body.WriteString("</div>")
}

// CellName returns the name of the cell of the row and the column, from 0,
// such as B3.
func CellName(row, col int) string {
	var letters []byte
	for c := col + 1; c > 0; c = (c - 1) / 26 {
		letters = append([]byte{byte('A' + (c-1)%26)}, letters...)
	}
	return string(letters) + strconv.Itoa(row+1)
}

// spend counts n nodes, and reports whether the sheet can still write
// them.
func (w *writer) spend(n int) bool {
	if w.nodes+n > w.max {
		w.stopped = true
		return false
	}
	w.nodes += n
	return true
}

// paper returns the width and the height of the paper, turned for
// landscape.
func (w *writer) paper() (float64, float64) {
	p := w.sh.Page
	pw, ph := p.Width, p.Height
	if pw <= 0 || ph <= 0 {
		pw, ph = DefaultPage.Width, DefaultPage.Height
	}
	if p.Landscape != (pw > ph) {
		pw, ph = ph, pw
	}
	return pw, ph
}

// printable returns the width and the height of the area of the page
// within its margins, at least an inch each, without the areas of the
// header and the footer.
func (w *writer) printable() (float64, float64) {
	pw, ph := w.paper()
	p := w.sh.Page
	return max(72, pw-p.Left-p.Right), max(72, ph-p.Top-p.Bottom-w.areas())
}

// areas returns the height of the areas of the header and the footer, at
// the scale of the cells when they scale with the cells.
func (w *writer) areas() float64 {
	p := w.sh.Page
	return (max(p.HeaderArea, 0) + max(p.FooterArea, 0)) * w.areaScale()
}

// areaScale returns the scale of the areas of the header and the footer.
func (w *writer) areaScale() float64 {
	if w.sh.Page.ScaleHeaderFooter && w.scale > 0 {
		return w.scale
	}
	return 1
}

// printRange writes the tables of the range r: one for each band of
// columns.
func (w *writer) printRange(r Range) error {
	sh := w.sh
	var cols []int
	width := 0.0
	for c := r.Col0; c <= r.Col1; c++ {
		if cw := sh.colWidth(c); cw > 0 {
			cols = append(cols, c)
			width += cw
		}
	}
	if len(cols) == 0 {
		return nil
	}
	var titleCols []int
	if t := sh.TitleCols; t != nil {
		for c := max(0, t.First); c <= min(t.Last, MaxCols-1); c++ {
			if sh.colWidth(c) > 0 {
				titleCols = append(titleCols, c)
			}
		}
	}
	w.scale = w.scaleFor(r, width)
	bands := w.bands(cols, titleCols)
	if w.sh.Page.OverThenDown && len(bands) > 1 {
		return w.overThenDown(r, bands, titleCols)
	}
	for i, band := range bands {
		// Excel prints no page without ink: a band ends at its last row
		// that shows something. A band that shows nothing keeps its rows.
		own := band
		if i > 0 && len(titleCols) > 0 {
			own = slices.DeleteFunc(slices.Clone(band), func(c int) bool { return slices.Contains(titleCols, c) })
		}
		br := r
		if last := w.lastRow(r, own); last >= r.Row0 {
			br.Row1 = last
		}
		if err := w.table(br, band); err != nil {
			return err
		}
		if w.stopped {
			break
		}
	}
	return nil
}

// overThenDown writes the tables of the range r in the order over, then
// down: the rows that fit on a page, in each band, then the next rows. The
// writer finds the rows of each page from their heights, which the rows of
// Excel keep. A table that shows nothing has no page.
func (w *writer) overThenDown(r Range, bands [][]int, titleCols []int) error {
	for _, rows := range w.rowPages(r) {
		for i, band := range bands {
			own := band
			if i > 0 && len(titleCols) > 0 {
				own = slices.DeleteFunc(slices.Clone(band), func(c int) bool { return slices.Contains(titleCols, c) })
			}
			last := w.lastRow(rows, own)
			if last < rows.Row0 {
				continue
			}
			if err := w.table(rows, band); err != nil {
				return err
			}
			if w.stopped {
				return nil
			}
		}
	}
	return nil
}

// rowPages cuts the rows of the range r into the runs of rows that fit on
// a page, below the rows of the print titles. A manual break starts a
// run. A run leaves 2 points free for the borders of the table.
func (w *writer) rowPages(r Range) []Range {
	sh := w.sh
	_, ph := w.printable()
	avail := ph/w.scale - 2
	if t := sh.TitleRows; t != nil {
		for i := max(0, t.First); i <= min(t.Last, MaxRows-1) && i-t.First < 4096; i++ {
			avail -= sh.rowHeight(sh.row(i))
		}
	}
	avail = max(avail, 1)
	var out []Range
	run := r
	used := 0.0
	ri := sort.Search(len(sh.Rows), func(i int) bool { return sh.Rows[i].Index >= r.Row0 })
	for i := r.Row0; i <= r.Row1; i++ {
		var row *Row
		for ri < len(sh.Rows) && sh.Rows[ri].Index < i {
			ri++
		}
		if ri < len(sh.Rows) && sh.Rows[ri].Index == i {
			row = &sh.Rows[ri]
		}
		h := sh.rowHeight(row)
		if w.isTitle(i) {
			h = 0
		}
		if i > run.Row0 && ((used > 0 && used+h > avail) || (w.rowBreaks[i] && h > 0)) {
			run.Row1 = i - 1
			out = append(out, run)
			run.Row0, used = i, 0
		}
		used += h
	}
	run.Row1 = r.Row1
	return append(out, run)
}

// lastRow returns the last row of the range r that shows something in the
// columns of band, or r.Row0-1 when no row does: a cell with text or with
// a style that draws, a merged cell, or an image. The columns of band are
// sorted.
func (w *writer) lastRow(r Range, band []int) int {
	if len(band) == 0 {
		return r.Row0 - 1
	}
	in := func(c0, c1 int) bool {
		k := sort.SearchInts(band, c0)
		return k < len(band) && band[k] <= c1
	}
	last := r.Row0 - 1
	for _, m := range w.sh.Merges {
		if m.Row0 <= r.Row1 && m.Row1 >= r.Row0 && in(m.Col0, m.Col1) {
			last = max(last, min(m.Row1, r.Row1))
		}
	}
	for i, img := range w.images {
		if img.Row <= r.Row1 && w.ends[i][0] >= r.Row0 && in(img.Col, w.ends[i][1]) {
			last = max(last, min(w.ends[i][0], r.Row1))
		}
	}
	hi := sort.Search(len(w.sh.Rows), func(i int) bool { return w.sh.Rows[i].Index > r.Row1 })
	for i := hi - 1; i >= 0 && w.sh.Rows[i].Index > last && w.sh.Rows[i].Index >= r.Row0; i-- {
		row := &w.sh.Rows[i]
		if row.Hidden {
			continue
		}
		if row.Style != 0 && w.style(row.Style).visible() {
			return row.Index
		}
		for _, c := range row.Cells {
			if in(c.Col, c.Col) && (c.Text != "" || len(c.Runs) > 0 || w.style(c.Style).visible()) {
				return row.Index
			}
		}
	}
	return last
}

// scaleFor returns the scale of the cells of the range r, whose printed
// columns are width points wide.
func (w *writer) scaleFor(r Range, width float64) (s float64) {
	p := w.sh.Page
	if !p.Fit {
		s := p.Scale
		if s <= 0 {
			s = 1
		}
		return min(4, max(0.1, s))
	}
	pw, ph := w.paper()
	pw, ph = max(72, pw-p.Left-p.Right), max(72, ph-p.Top-p.Bottom)
	defer func() {
		// Excel fits the cells at a whole percent
		s = max(0.1, math.Floor(s*100+1e-9)/100)
	}()
	s = 1.0
	if p.FitWidth > 0 && width > 0 {
		s = min(s, float64(p.FitWidth)*pw/width)
	}
	if p.FitHeight > 0 {
		// Each page holds the areas of the header and the footer, at the
		// scale of the cells when they scale with the cells.
		n := float64(p.FitHeight)
		areas := max(p.HeaderArea, 0) + max(p.FooterArea, 0)
		if h := w.height(r); h > 0 && p.ScaleHeaderFooter {
			s = min(s, n*ph/(h+n*areas))
		} else if h > 0 {
			s = min(s, n*max(ph-areas, 1)/h)
		}
	}
	return s
}

// height returns the height of the rows of r in points.
func (w *writer) height(r Range) float64 {
	sh := w.sh
	n := r.Row1 - r.Row0 + 1
	h := 0.0
	for i := range sh.Rows {
		row := &sh.Rows[i]
		if row.Index < r.Row0 || row.Index > r.Row1 {
			continue
		}
		n--
		h += sh.rowHeight(row)
	}
	return h + float64(n)*sh.grid(sh.DefaultRowHeight)
}

// bands cuts the columns into the bands that fit the width of the page,
// with the title columns in each band after the first. A manual break of
// a column starts a band.
func (w *writer) bands(cols, titleCols []int) [][]int {
	sh := w.sh
	pw, _ := w.printable()
	avail := pw / w.scale
	titleWidth := 0.0
	for _, c := range titleCols {
		titleWidth += sh.colWidth(c)
	}
	var bands [][]int
	var band []int
	used := 0.0
	for _, c := range cols {
		cw := sh.colWidth(c)
		if len(band) > 0 && (used+cw > avail+0.01 || w.colBreaks[c]) {
			bands = append(bands, band)
			band, used = nil, 0
		}
		if len(band) == 0 && len(bands) > 0 && len(titleCols) > 0 && !slices.Contains(titleCols, c) {
			band = append(band, titleCols...)
			used = titleWidth
		}
		if !slices.Contains(band, c) {
			band = append(band, c)
			used += cw
		}
	}
	if len(band) > 0 {
		bands = append(bands, band)
	}
	for _, b := range bands {
		slices.Sort(b)
	}
	return bands
}

// pt formats a length in points for CSS.
func pt(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64) + "pt"
}

// hex formats a color for CSS.
func hex(c color.NRGBA) string {
	if c.A == 0 {
		return "#000000"
	}
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// quote returns s as a string of CSS.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\A `)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\%x `, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// fontFamily returns the value of font-family for f.
func fontFamily(f Font, def Font) string {
	// the generic family follows the family: a font with a family of its
	// own and no generic family is sans-serif
	family, generic := f.Family, f.Generic
	if family == "" {
		family = def.Family
		if generic == "" {
			generic = def.Generic
		}
	}
	if generic == "" {
		generic = "sans-serif"
	}
	if family == "" {
		return generic
	}
	return quote(family) + ", " + generic
}

// fontCSS writes the declarations of the font f, at the scale s. def is
// the font of the default style.
func fontCSS(b *strings.Builder, f, def Font, s float64) {
	size := f.Size
	if size <= 0 {
		size = def.Size
	}
	if size <= 0 {
		size = 11
	}
	b.WriteString("font-family:" + fontFamily(f, def) + ";font-size:" + pt(size*s) + ";")
	if f.Bold {
		b.WriteString("font-weight:bold;")
	}
	if f.Italic {
		b.WriteString("font-style:italic;")
	}
	var deco []string
	switch f.Underline {
	case 1:
		deco = append(deco, "underline")
	case 2:
		deco = append(deco, "underline")
	}
	if f.Strike {
		deco = append(deco, "line-through")
	}
	if len(deco) > 0 {
		b.WriteString("text-decoration:" + strings.Join(deco, " ") + ";")
		if f.Underline == 2 {
			b.WriteString("text-decoration-style:double;")
		}
	}
	if f.Color.A != 0 {
		b.WriteString("color:" + hex(f.Color) + ";")
	}
	switch f.VertAlign {
	case 1:
		b.WriteString("vertical-align:super;font-size:" + pt(size*s*0.7) + ";")
	case 2:
		b.WriteString("vertical-align:sub;font-size:" + pt(size*s*0.7) + ";")
	}
}

// css writes the style sheet of the document.
func (w *writer) css(b *strings.Builder) {
	w.pageRule(b)
	def := w.style(0)
	s := w.scale
	b.WriteString("html,body{margin:0;padding:0}\n")
	b.WriteString("table.sh{table-layout:fixed;border-collapse:collapse;border-spacing:0;")
	fontCSS(b, def.Font, def.Font, s)
	if w.sh.Page.CenterH {
		b.WriteString("margin-left:auto;margin-right:auto;")
	}
	b.WriteString("}\n")
	b.WriteString("table.sh+table.sh{break-before:page}\n")
	b.WriteString("tr{break-inside:avoid;height:" + pt(w.sh.grid(w.sh.DefaultRowHeight)*s) + "}\n")
	b.WriteString("tr.pb{break-before:page}\n")
	b.WriteString("td{padding:0 " + pt(cellPadding*s) + ";vertical-align:bottom;white-space:pre;text-align:left;overflow:visible;line-height:normal;")
	if w.sh.Gridlines {
		b.WriteString("border:" + pt(max(0.5, 0.5*s)) + " solid " + gridColor + ";")
	}
	// the cells of the default style take its fill, its borders and its
	// alignment
	w.styleCSS(b, def, false)
	b.WriteString("}\n")
	b.WriteString("td.c{overflow:hidden}\ntd.a{position:relative}\n")
	b.WriteString("div.cl{position:absolute;left:0;top:0;overflow:hidden}\ndiv.wc{overflow:hidden}\n")
	b.WriteString("div.db,img.ic{position:absolute}\nspan.t{position:relative}\n")
	b.WriteString("span.r{display:inline-block}\n")
	b.WriteString("span.sl,span.sb{display:inline-block;width:0}\nspan.sl>span,span.sb>span{display:inline-block}\n")
	b.WriteString("span.sl>span{transform:translateX(-100%)}\nspan.sb>span{transform:translateX(-50%)}\n")
	b.WriteString("div.ph{position:absolute;border:0.75pt solid #808080;background:#f2f2f2;color:#404040;font-size:9pt;white-space:normal;text-align:center;overflow:hidden}\n")
	b.WriteString("p.note{margin:6pt 0 0;font-family:sans-serif;font-size:9pt;color:#606060}\n")
	b.WriteString("div.cm{break-before:page;white-space:pre-wrap}\ndiv.cm p{margin:0 0 10pt}\n")
	ids := make([]int, 0, len(w.used))
	for i := range w.used {
		ids = append(ids, i)
	}
	slices.Sort(ids)
	for _, i := range ids {
		b.WriteString("td.s" + strconv.Itoa(i) + "{")
		w.styleCSS(b, w.style(i), true)
		b.WriteString("}\n")
	}
	// the alignment General of a number and of a boolean, after the
	// styles, which can undo the alignment of the default style
	b.WriteString("td.n{text-align:right}\ntd.x{text-align:center}\n")
}

// styleCSS writes the declarations of the style st. own is set for a style
// other than the default style: it writes the font too, and it undoes what
// the default style sets on each cell and st does not. The cells of the
// default style take the font of the table.
func (w *writer) styleCSS(b *strings.Builder, st *Style, own bool) {
	s := w.scale
	def := w.style(0)
	if own {
		fontCSS(b, st.Font, def.Font, s)
		w.resetCSS(b, st, def)
	}
	if st.Fill.A != 0 {
		b.WriteString("background-color:" + hex(st.Fill) + ";")
	}
	for k, side := range [4]string{"top", "right", "bottom", "left"} {
		bd := st.Borders[k]
		if bd.Width <= 0 {
			continue
		}
		line := bd.Line
		if line == "" {
			line = "solid"
		}
		b.WriteString("border-" + side + ":" + pt(borderWidth(bd, s)) + " " + line + " " + hex(bd.Color) + ";")
	}
	switch st.HAlign {
	case "center", "centerContinuous", "distributed":
		b.WriteString("text-align:center;")
	case "right":
		b.WriteString("text-align:right;")
	case "left", "fill":
		b.WriteString("text-align:left;")
	case "justify":
		b.WriteString("text-align:justify;")
	}
	switch st.VAlign {
	case "top":
		b.WriteString("vertical-align:top;")
	case "center", "justify", "distributed":
		b.WriteString("vertical-align:middle;")
	}
	if wraps(st) {
		b.WriteString("white-space:pre-wrap;overflow-wrap:break-word;")
	}
	if st.Indent > 0 {
		side := "left"
		if st.HAlign == "right" {
			side = "right"
		}
		b.WriteString("padding-" + side + ":" + pt((cellPadding+st.Indent)*s) + ";")
	}
}

// resetCSS writes the declarations that undo on a cell of the style st what
// the default style def sets on each cell, when st does not set it.
func (w *writer) resetCSS(b *strings.Builder, st, def *Style) {
	if def.Fill.A != 0 && st.Fill.A == 0 {
		b.WriteString("background-color:transparent;")
	}
	grid := "none"
	if w.sh.Gridlines {
		grid = pt(max(0.5, 0.5*w.scale)) + " solid " + gridColor
	}
	for k, side := range [4]string{"top", "right", "bottom", "left"} {
		if def.Borders[k].Width > 0 && st.Borders[k].Width <= 0 {
			b.WriteString("border-" + side + ":" + grid + ";")
		}
	}
	if def.HAlign != "" && st.HAlign == "" {
		// a number of the alignment General goes to the right with the
		// class n, which comes after the styles
		b.WriteString("text-align:left;")
	}
	if def.VAlign != "" && def.VAlign != "bottom" && (st.VAlign == "" || st.VAlign == "bottom") {
		b.WriteString("vertical-align:bottom;")
	}
	if wraps(def) && !wraps(st) {
		b.WriteString("white-space:pre;overflow-wrap:normal;")
	}
	if def.Indent > 0 && st.Indent <= 0 {
		b.WriteString("padding:0 " + pt(cellPadding*w.scale) + ";")
	}
}

// wraps reports whether the text of a cell of the style st breaks into
// lines.
func wraps(st *Style) bool {
	return st.Wrap || st.VAlign == "justify" || st.VAlign == "distributed" || st.HAlign == "justify" || st.HAlign == "distributed"
}

// borderWidth returns the width in points of the border bd at the scale
// s: at least half a point, so that a border shows at a small scale.
func borderWidth(bd Border, s float64) float64 {
	width := max(bd.Width*s, 0.5)
	if bd.Line == "double" {
		width = max(width, 2.25*s)
	}
	return width
}

// pageRule writes the rule @page of the sheet, with its header and its
// footer.
func (w *writer) pageRule(b *strings.Builder) {
	p := w.sh.Page
	pw, ph := w.paper()
	top := max(0, p.Top) + max(p.HeaderArea, 0)*w.areaScale()
	bottom := max(0, p.Bottom) + max(p.FooterArea, 0)*w.areaScale()
	b.WriteString("@page{size:" + pt(pw) + " " + pt(ph) + ";margin:" +
		pt(top) + " " + pt(max(0, p.Right)) + " " + pt(bottom) + " " + pt(max(0, p.Left)) + ";")
	header, footer := w.sh.Header, w.sh.Footer
	if header.Empty() && footer.Empty() && w.sh.Name != "" {
		// The name of the sheet, small and light, tells a reader which
		// sheet a page shows. It is not ink that a printout has.
		b.WriteString("@top-left{content:" + quote(w.sh.Name) +
			";vertical-align:top;padding-top:" + pt(max(0, min(p.Header, top-10))) +
			";font-family:sans-serif;font-size:8pt;color:#909090}")
	}
	w.marginBoxes(b, header, "top", p.Header, top)
	w.marginBoxes(b, footer, "bottom", p.Footer, bottom)
	b.WriteString("}\n")
}

// marginBoxes writes the margin boxes of a header or a footer. edge is the
// distance of the text from the edge of the paper, and margin the margin
// of the page on that side.
func (w *writer) marginBoxes(b *strings.Builder, hf HeaderFooter, side string, edge, margin float64) {
	if hf.Empty() {
		return
	}
	def := w.style(0).Font
	font := def
	if hf.Font != nil {
		font = *hf.Font
	}
	align := "top"
	if side == "bottom" {
		align = "bottom"
	}
	for _, part := range []struct {
		name   string
		fields []Field
	}{{"left", hf.Left}, {"center", hf.Center}, {"right", hf.Right}} {
		if len(part.fields) == 0 {
			continue
		}
		var content []string
		for _, f := range part.fields {
			switch f.Kind {
			case FieldText:
				content = append(content, quote(f.Text))
			case FieldPage:
				content = append(content, "counter(page)")
			case FieldPages:
				content = append(content, "counter(pages)")
			case FieldSheet:
				content = append(content, quote(w.sh.Name))
			case FieldTitle:
				content = append(content, quote(w.wb.Title))
			}
		}
		b.WriteString("@" + side + "-" + part.name + "{content:" + strings.Join(content, " ") +
			";vertical-align:" + align + ";padding-" + side + ":" + pt(max(0, min(edge, margin-10))) +
			";white-space:pre;")
		s := 1.0
		if w.sh.Page.ScaleHeaderFooter && w.scale > 0 {
			s = w.scale
		}
		fontCSS(b, font, def, s)
		b.WriteString("}")
	}
}

// table writes the table of the range r for the columns of band. The rows
// of the print titles go in its head.
func (w *writer) table(r Range, band []int) error {
	sh := w.sh
	if !w.spend(2 + len(band)) {
		return nil
	}
	b := &w.body
	span := tableSpan{start: b.Len()}
	b.WriteString(`<table class="sh"`)
	total := 0.0
	for _, c := range band {
		total += sh.colWidth(c)
	}
	b.WriteString(` style="width:` + pt(total*w.scale))
	if top := w.centerTop(r); top > 0 {
		b.WriteString(";margin-top:" + pt(top))
	}
	b.WriteString(`"`)
	b.WriteString("><colgroup>")
	for _, c := range band {
		b.WriteString(`<col style="width:` + pt(sh.colWidth(c)*w.scale) + `">`)
	}
	b.WriteString("</colgroup>")
	if t := sh.TitleRows; t != nil {
		head := Range{Row0: max(0, t.First), Row1: min(t.Last, MaxRows-1), Col0: r.Col0, Col1: r.Col1}
		if head.Row0 <= head.Row1 {
			b.WriteString("<thead>")
			var titles tableSpan
			w.rec = &titles
			err := w.rows(head, band, false)
			w.rec = nil
			if err != nil {
				return err
			}
			b.WriteString("</thead>")
			span.heads = len(titles.rows)
			for _, row := range titles.rows {
				span.head += row.h
			}
			if len(titles.rows) > 0 {
				span.headTop = titles.rows[0].top
			}
		}
	}
	b.WriteString("<tbody>")
	span.body = b.Len()
	w.rec = &span
	err := w.rows(r, band, sh.TitleRows != nil)
	w.rec = nil
	span.rowsEnd = b.Len()
	b.WriteString("</tbody></table>")
	span.end = b.Len()
	w.tables = append(w.tables, span)
	return err
}

// centerTop returns the space above the table of the rows of r, in points,
// that centers the rows down the page, or 0. Only the rows that fit on one
// page, without a manual break, are centered.
func (w *writer) centerTop(r Range) float64 {
	if !w.sh.Page.CenterV {
		return 0
	}
	h := w.height(r)
	if t := w.sh.TitleRows; t != nil && (t.First < r.Row0 || t.Last > r.Row1) {
		h += w.height(Range{Row0: max(0, t.First), Row1: min(t.Last, MaxRows-1)})
	}
	for i := r.Row0 + 1; i <= r.Row1; i++ {
		if w.rowBreaks[i] {
			return 0
		}
	}
	_, ph := w.printable()
	// the rows leave 2 points for the borders of the table, as rowPages
	// does
	free := ph - 2 - h*w.scale
	if free <= 0 {
		return 0
	}
	return free / 2
}

// isTitle reports whether the row i is a row of the print titles.
func (w *writer) isTitle(i int) bool {
	t := w.sh.TitleRows
	return t != nil && i >= t.First && i <= t.Last
}

// printed counts the rows from row0 to row1 that the table shows: the rows
// that are not hidden, and not titles when skipTitles is set.
func (w *writer) printed(row0, row1 int, skipTitles bool) int {
	if row1 < row0 {
		return 0
	}
	n := row1 - row0 + 1
	lo := sort.SearchInts(w.hidden, row0)
	hi := sort.SearchInts(w.hidden, row1+1)
	n -= hi - lo
	if t := w.sh.TitleRows; skipTitles && t != nil {
		a, z := max(row0, t.First), min(row1, t.Last)
		if a <= z {
			n -= z - a + 1
			lo := sort.SearchInts(w.hidden, a)
			hi := sort.SearchInts(w.hidden, z+1)
			n += hi - lo
		}
	}
	return max(n, 0)
}

// mergeState is a merged cell in a table.
type mergeState struct {
	m Range
	// started is set when the table wrote the cell of the merge.
	started bool
	// cols are the indexes in the band of its columns.
	lo, hi int
	// first and last are the first and the last row of the merge among
	// the rows that the writer records, or -1.
	first, last int
}

// rows writes the rows of the range r for the columns of band.
// skipTitles leaves out the rows of the print titles.
func (w *writer) rows(r Range, band []int, skipTitles bool) error {
	sh := w.sh
	// The merges that cross the range and the band.
	var merges []*mergeState
	for _, m := range sh.Merges {
		if m.Row1 < r.Row0 || m.Row0 > r.Row1 || m.Row0 > m.Row1 || m.Col0 > m.Col1 {
			continue
		}
		lo := sort.SearchInts(band, m.Col0)
		hi := sort.SearchInts(band, m.Col1+1)
		if lo >= hi {
			continue
		}
		merges = append(merges, &mergeState{m: m, lo: lo, hi: hi, first: -1, last: -1})
	}
	slices.SortFunc(merges, func(a, b *mergeState) int { return a.m.Row0 - b.m.Row0 })
	if rec := w.rec; rec != nil {
		defer rec.holdRows(merges)
	}
	var active []*mergeState
	next := 0
	cover := make([]*mergeState, len(band))
	text := make([]bool, len(band))
	// The rows of the sheet from r.Row0.
	ri := sort.Search(len(sh.Rows), func(i int) bool { return sh.Rows[i].Index >= r.Row0 })
	firstRow := true
	for i := r.Row0; i <= r.Row1; i++ {
		if (i-r.Row0)%1024 == 0 {
			if err := w.ctx.Err(); err != nil {
				return fmt.Errorf("converting sheet %q: %w", sh.Name, err)
			}
		}
		var row *Row
		for ri < len(sh.Rows) && sh.Rows[ri].Index < i {
			ri++
		}
		if ri < len(sh.Rows) && sh.Rows[ri].Index == i {
			row = &sh.Rows[ri]
		}
		h := sh.rowHeight(row)
		if h == 0 || (skipTitles && w.isTitle(i)) {
			continue
		}
		// The merges of the row.
		for next < len(merges) && merges[next].m.Row0 <= i {
			active = append(active, merges[next])
			next++
		}
		active = slices.DeleteFunc(active, func(m *mergeState) bool { return m.m.Row1 < i })
		clear(cover)
		for _, m := range active {
			for k := m.lo; k < m.hi; k++ {
				if cover[k] == nil {
					cover[k] = m
				}
			}
		}
		if !w.spend(1) {
			return nil
		}
		if rec := w.rec; rec != nil {
			k := len(rec.rows)
			for _, m := range active {
				if m.first < 0 {
					m.first = k
				}
				m.last = k
			}
			rec.rows = append(rec.rows, rowSpan{
				off: w.body.Len(), h: round2(h * w.scale), brk: w.rowBreaks[i] && !firstRow,
				first: -1, last: -1, inTall: w.inTall(i),
			})
			w.top, w.bottom = 0, 0
		}
		w.body.WriteString("<tr")
		if w.rowBreaks[i] && !firstRow {
			w.body.WriteString(` class="pb"`)
		}
		if h != sh.grid(sh.DefaultRowHeight) {
			w.body.WriteString(` style="height:` + pt(h*w.scale) + `"`)
		}
		w.body.WriteString(">")
		firstRow = false
		for k, c := range band {
			text[k] = cover[k] != nil
			if cell := row.cell(c); cell != nil && (cell.Text != "" || len(cell.Runs) > 0) {
				text[k] = true
			}
		}
		ok := w.cells(i, row, band, cover, text, skipTitles)
		w.body.WriteString("</tr>")
		if rec := w.rec; rec != nil {
			last := &rec.rows[len(rec.rows)-1]
			last.top, last.bottom = w.top, w.bottom
		}
		if !ok {
			return nil
		}
	}
	return nil
}

// cellStyle returns the style of the cell of the column c in row: its own
// style, or else the style of the row or of the column.
func (w *writer) cellStyle(row *Row, cell *Cell, c int) int {
	if cell != nil {
		return cell.Style
	}
	if row != nil && row.Style != 0 {
		return row.Style
	}
	if col := w.sh.col(c); col != nil {
		return col.Style
	}
	return 0
}

// cells writes the cells of the row i. It returns false when the sheet
// stops at its limit.
func (w *writer) cells(i int, row *Row, band []int, cover []*mergeState, text []bool, skipTitles bool) bool {
	b := &w.body
	// empty counts the empty cells that the row has not written yet. gap
	// is set when a merged cell of an earlier row is among their columns.
	// wrote is set when the row wrote a cell.
	empty := 0
	gap, wrote := false, false
	flush := func() bool {
		if empty == 0 {
			return true
		}
		if !w.spend(1) {
			return false
		}
		if empty == 1 {
			b.WriteString("<td></td>")
		} else {
			b.WriteString(`<td colspan="` + strconv.Itoa(empty) + `"></td>`)
		}
		w.borders(0)
		empty, gap = 0, false
		return true
	}
	for k := 0; k < len(band); k++ {
		c := band[k]
		m := cover[k]
		srcRow, srcCol := i, c
		colspan, rowspan := 1, 1
		if m != nil {
			if m.started || k != m.lo {
				gap = gap || empty > 0
				continue
			}
			m.started = true
			colspan = m.hi - m.lo
			rowspan = w.printed(i, min(m.m.Row1, MaxRows-1), skipTitles)
			srcRow, srcCol = m.m.Row0, m.m.Col0
		}
		var cell *Cell
		srcRowp := row
		if srcRow != i {
			srcRowp = w.sh.row(srcRow)
		}
		cell = srcRowp.cell(srcCol)
		style := w.cellStyle(srcRowp, cell, srcCol)
		res := w.cond[[2]int{srcRow, srcCol}]
		if res != nil {
			style = w.derive(style, res)
		}
		imgs := w.imagesAt(i, k, band, m)
		hasText := cell != nil && (cell.Text != "" || len(cell.Runs) > 0)
		if m != nil && w.firstCol(m.m) != c {
			// the text of a merge that spans bands shows in its first band
			hasText = false
		}
		if m == nil && !hasText && len(imgs) == 0 && !w.sh.Gridlines && !w.style(style).visible() {
			empty++
			continue
		}
		if !flush() {
			return false
		}
		// a cell counts once, with its text
		if !w.spend(1) {
			return false
		}
		wrote = true
		if style != 0 {
			w.used[style] = true
		}
		w.borders(style)
		st := w.style(style)
		var class []string
		if style != 0 {
			class = append(class, "s"+strconv.Itoa(style))
		}
		if hasText && st.HAlign == "" {
			switch {
			case cell.Kind == KindNumber && !cell.AsText:
				class = append(class, "n")
			case cell.Kind == KindBool, cell.Kind == KindError:
				class = append(class, "x")
			}
		}
		spill := w.spill(cell, st, m, k, text)
		clipped := m != nil || st.Wrap || (hasText && cell.Kind != KindText) ||
			(k+1 < len(band) && text[k+1] && spill != "left") ||
			(hasText && st.HAlign != "" && st.HAlign != "left" && st.HAlign != "general" && spill == "")
		marks := res != nil && hasText && (res.bar != nil || res.icon != nil)
		if len(imgs) > 0 || marks {
			class = append(class, "a")
		}
		if clipped && len(imgs) == 0 {
			class = append(class, "c")
		}
		b.WriteString("<td")
		if len(class) > 0 {
			b.WriteString(` class="` + strings.Join(class, " ") + `"`)
		}
		if colspan > 1 {
			b.WriteString(` colspan="` + strconv.Itoa(colspan) + `"`)
		}
		if rowspan > 1 {
			b.WriteString(` rowspan="` + strconv.Itoa(rowspan) + `"`)
		}
		var inline []string
		if hasText && cell.Color.A != 0 {
			inline = append(inline, "color:"+hex(cell.Color))
		}
		if hasText && st.Shrink && !st.Wrap {
			if f := w.shrink(cell, st, band[k:k+colspan]); f > 0 {
				inline = append(inline, "font-size:"+pt(f))
			}
		}
		wrap := wraps(st)
		// clipText is the height of the block that holds a text that
		// wraps in rows that keep their heights, or 0
		clipText := 0.0
		if hasText && w.sh.FixedRows {
			h := w.sh.rowHeight(srcRowp)
			if rowspan > 1 {
				// the rows of a merged cell keep their heights in the
				// layout, so a line can take the height of them all
				h = w.spanHeight(i, m.m.Row1, skipTitles)
			}
			if wrap && st.Rotation == 0 && w.lines(cell, st, h) < 2 {
				// Excel shows the first line of a text that wraps in a row
				// of one line, and the row keeps its height
				wrap = false
				inline = append(inline, "white-space:pre", "overflow-wrap:normal")
			}
			if lh := w.lineHeight(cell, st, h, wrap); lh > 0 {
				inline = append(inline, "line-height:"+pt(lh))
			}
			if wrap && st.Rotation == 0 {
				// Excel keeps the heights of the rows, and cuts the lines
				// of a text that wraps that they do not hold: the text
				// goes in a block of their height, with lines that fit
				size, avail := w.room(cell, st, h)
				clipText = avail
				normal := 1.22
				if eastAsian(cell) {
					normal = 1.5
				}
				if n := w.lines(cell, st, h); n >= 2 && size*w.scale*normal*float64(n) > avail {
					inline = append(inline, "line-height:"+pt(max(0.1, avail/float64(n))))
				}
			}
		}
		if len(inline) > 0 {
			b.WriteString(` style="` + strings.Join(inline, ";") + `"`)
		}
		b.WriteString(">")
		over := false
		if marks {
			markHeight := w.sh.rowHeight(srcRowp)
			if rowspan > 1 {
				markHeight = w.spanHeight(i, m.m.Row1, skipTitles)
			}
			if w.marks(res, markHeight, band[k:k+colspan]) {
				hasText = false
			} else {
				// the text goes over the bar, and after the icon
				b.WriteString(`<span class="t"`)
				if res.icon != nil {
					b.WriteString(` style="padding-left:` + pt(iconRoom(markHeight)*w.scale) + `"`)
				}
				b.WriteString(">")
				over = true
			}
		}
		if hasText {
			switch hashes := w.overflow(cell, st, band[k:k+colspan]); {
			case hashes != "":
				b.WriteString(hashes)
			case anchored(cell, st, m) && w.mayOverflow(cell, st, band[k:k+colspan]):
				// an inline block of no width takes the place of the end
				// or the middle of the text, and its text moves back by
				// the whole or the half of its width, so that it goes
				// past the cell on the left or on both sides, or is cut
				// on both sides as Excel cuts it
				side := "l"
				if st.HAlign != "right" {
					side = "b"
				}
				b.WriteString(`<span class="s` + side + `"><span>`)
				w.content(cell, st, wrap)
				b.WriteString("</span></span>")
			case clipText > 0:
				b.WriteString(`<div class="wc" style="max-height:` + pt(clipText) + `">`)
				w.content(cell, st, wrap)
				b.WriteString("</div>")
			default:
				w.content(cell, st, wrap)
			}
		}
		if over {
			b.WriteString("</span>")
		}
		for _, img := range imgs {
			w.image(img)
		}
		b.WriteString("</td>")
		k += colspan - 1
	}
	if empty > 0 && wrote && !gap && !w.style(0).visible() {
		// The empty cells at the end of a row draw nothing, so the row
		// leaves them out, which saves their styles and their boxes. They
		// still count, so that the sheet stops at the same cell.
		return w.spend(1)
	}
	return flush()
}

// spill returns the side to which the text of the cell of the column
// band[k] goes past the cell, into the empty cells next to it, as Excel
// shows it: "right" for text that is aligned left, "left" for text that is
// aligned right, "both" for centered text, or "" for none. text says which
// cells of the band hold text or a merged cell. Text that is aligned right
// or centered stays in the band.
func (w *writer) spill(cell *Cell, st *Style, m *mergeState, k int, text []bool) string {
	if cell == nil || m != nil || cell.Kind != KindText || (cell.Text == "" && len(cell.Runs) == 0) ||
		wraps(st) || st.Shrink || st.Rotation != 0 {
		return ""
	}
	prev := k > 0 && !text[k-1]
	next := k+1 < len(text) && !text[k+1]
	switch st.HAlign {
	case "", "left", "general":
		if next {
			return "right"
		}
	case "right":
		if prev {
			return "left"
		}
	case "center", "centerContinuous":
		if prev && next {
			return "both"
		}
	}
	return ""
}

// derive returns the index of the style that the conditional formats of
// res make of the style of the index style.
func (w *writer) derive(style int, res *condResult) int {
	if len(res.diffs) == 0 && res.fill.A == 0 {
		return style
	}
	var key strings.Builder
	for _, d := range res.diffs {
		fmt.Fprintf(&key, "%p,", d)
	}
	k := derivedKey{style: style, diffs: key.String(), fill: res.fill}
	if i, ok := w.derived[k]; ok {
		return i
	}
	if len(w.extra) >= maxDerived {
		return style
	}
	if w.derived == nil {
		w.derived = map[derivedKey]int{}
	}
	w.extra = append(w.extra, applyDiffs(*w.style(style), res.diffs, res.fill))
	i := max(1, len(w.wb.Styles)) + len(w.extra) - 1
	w.derived[k] = i
	return i
}

// iconSize returns the size of the icon of a conditional format in a row
// of the height h, and iconRoom the room that the text leaves for it.
func iconSize(h float64) float64 { return max(1, min(h-2, 12)) }

func iconRoom(h float64) float64 { return iconSize(h) + 1.5 }

// maxDerived is the number of the styles that the conditional formats of
// a sheet make.
const maxDerived = 4096

// marks writes the data bar and the icon of the conditional formats of a
// cell of the height h in the columns cols, such as a merged cell. It
// reports whether they hide the text of the cell.
func (w *writer) marks(res *condResult, h float64, cols []int) bool {
	s := w.scale
	width := 0.0
	for _, c := range cols {
		width += w.sh.colWidth(c)
	}
	b := &w.body
	hide := false
	left := 1.5
	if ic := res.icon; ic != nil && w.spend(1) {
		size := iconSize(h)
		path := iconPath(ic.set.Name, ic.index, ic.count)
		if w.files == nil {
			w.files = map[string][]byte{}
		}
		if _, ok := w.files[path]; !ok {
			w.files[path] = iconSVG(ic.set.Name, ic.index, ic.count)
		}
		b.WriteString(`<img class="ic" src="` + html.EscapeString(path) + `" alt="" style="left:` + pt(left*s) +
			";top:" + pt((h-size-1)*s) + ";width:" + pt(size*s) + ";height:" + pt(size*s) + `">`)
		hide = ic.hide
	}
	if bar := res.bar; bar != nil && w.spend(1) {
		// Excel draws a bar from a tenth of the cell to nine tenths
		length := (width - 3) * (0.1 + 0.8*min(max(bar.frac, 0), 1))
		fill := "background-color:" + hex(bar.bar.Color)
		if bar.bar.Gradient {
			fill = "background-image:linear-gradient(to right," + hex(bar.bar.Color) + ",#ffffff)"
		}
		b.WriteString(`<div class="db" style="left:` + pt(1.5*s) + ";top:" + pt(1.5*s) + ";width:" + pt(max(0, length)*s) +
			";height:" + pt(max(0, h-3)*s) + ";" + fill + `"></div>`)
		hide = hide || bar.bar.HideValue
	}
	return hide
}

// anchored reports whether the text of a cell is aligned right or
// centered on one line, and so holds its place at its end or at its middle
// when it is wider than its cell.
func anchored(cell *Cell, st *Style, m *mergeState) bool {
	if cell == nil || m != nil || cell.Kind != KindText || wraps(st) || st.Shrink || st.Rotation != 0 {
		return false
	}
	return st.HAlign == "right" || st.HAlign == "center" || st.HAlign == "centerContinuous"
}

// mayOverflow reports whether the text of a cell can be wider than its
// columns. It measures the text in Calibri, with a margin for the fonts
// that are wider.
func (w *writer) mayOverflow(cell *Cell, st *Style, cols []int) bool {
	size := st.Font.Size
	if size <= 0 {
		size = w.style(0).Font.Size
	}
	if size <= 0 {
		size = 11
	}
	text := cell.Text
	if len(cell.Runs) > 0 {
		var b strings.Builder
		for _, r := range cell.Runs {
			b.WriteString(r.Text)
		}
		text = b.String()
	}
	width := -2 * cellPadding
	for _, c := range cols {
		width += w.sh.colWidth(c)
	}
	return TextWidth(text, size)*1.25 > width
}

// lineHeight returns the height of the line of the text of a cell in rows
// of the height h, or 0 for the height of its font. Excel keeps the height
// of a row and lets the text cross its edges (Sheet.FixedRows). The layout
// makes a row taller for a line of text, so a line of text that is higher
// than its row gets the height of the row, less its borders. The lines of a
// text that wraps, and a rotated text, keep their heights.
func (w *writer) lineHeight(cell *Cell, st *Style, h float64, wrap bool) float64 {
	if wrap || st.Rotation != 0 || h <= 0 {
		return 0
	}
	size, avail := w.room(cell, st, h)
	s := w.scale
	// the normal height of a line is from 1.15 to 1.22 times the size of
	// the font, such as with Arial and Calibri, and up to 1.5 times with
	// the fonts of East Asia
	normal := 1.22
	if eastAsian(cell) {
		normal = 1.5
	}
	if size*s*normal <= avail {
		return 0
	}
	return max(0.1, min(avail, size*s*1.15))
}

// room returns the largest size of the font of the text of a cell, and the
// height within the borders of rows of the height h, at the scale of the
// cells, in points.
func (w *writer) room(cell *Cell, st *Style, h float64) (size, avail float64) {
	size = st.Font.Size
	for _, r := range cell.Runs {
		if r.Font != nil {
			size = max(size, r.Font.Size)
		}
	}
	if size <= 0 {
		size = w.style(0).Font.Size
	}
	if size <= 0 {
		size = 11
	}
	s := w.scale
	borders := 0.0
	for _, k := range []int{0, 2} {
		if bd := st.Borders[k]; bd.Width > 0 {
			borders += borderWidth(bd, s) / 2
		} else if w.sh.Gridlines {
			borders += max(0.5, 0.5*s) / 2
		}
	}
	return size, h*s - borders
}

// lines returns the number of lines of the text of a cell that rows of the
// printed height h hold, with lines of 1.22 times the size of the font, as
// Excel lays out Calibri and MS PGothic. The lines are printed at the scale
// of the rows (Sheet.RowScale) too, and the borders of the rows do not
// count: Excel fits 2 lines of MS PGothic of 11 points in a row of 27
// points, twice its default height.
func (w *writer) lines(cell *Cell, st *Style, h float64) int {
	size, _ := w.room(cell, st, h)
	scale := w.sh.RowScale
	if !(scale > 0) {
		scale = calibriRowScale
	}
	return int(h/(size*1.22*scale) + 0.01)
}

// calibriRowScale is the scale of the rows of a workbook whose default font
// is Calibri of 11 points: 121 dots for 20 pixels (ExcelRows).
const calibriRowScale = 0.968

// eastAsian reports whether the text of a cell holds a character of the
// scripts of East Asia, whose fonts have higher lines.
func eastAsian(cell *Cell) bool {
	has := func(s string) bool {
		for _, r := range s {
			if r >= 0x2E80 && r <= 0xFFEF {
				return true
			}
		}
		return false
	}
	if has(cell.Text) {
		return true
	}
	for _, r := range cell.Runs {
		if has(r.Text) {
			return true
		}
	}
	return false
}

// spanHeight returns the height of the rows from row0 to row1 that the
// table shows, or 0 for a span of more than 4096 rows.
func (w *writer) spanHeight(row0, row1 int, skipTitles bool) float64 {
	if row1-row0 > 4096 {
		return 0
	}
	h := 0.0
	for i := row0; i <= row1; i++ {
		if skipTitles && w.isTitle(i) {
			continue
		}
		h += w.sh.rowHeight(w.sh.row(i))
	}
	return h
}

// firstCol returns the first column of the merge m that is not hidden.
func (w *writer) firstCol(m Range) int {
	for c := m.Col0; c <= m.Col1 && c < MaxCols; c++ {
		if w.sh.colWidth(c) > 0 {
			return c
		}
	}
	return m.Col0
}

// placed is an image in a cell, with its offset from the top left corner
// of the cell. A slice of an image shows only the part of the image in the
// box of clipW by clipH points at the corner of the cell.
type placed struct {
	img          Image
	dx, dy       float64
	slice        bool
	clipW, clipH float64
}

// maxSliceRows is the number of rows of an image that each show the slice
// of the image in their row. A taller image shows in its first cell only.
const maxSliceRows = 1024

// imagesAt returns the images that the cell of the row i and the column
// band[k] holds. A merged cell m holds the images of its cells.
//
// An image that covers more than one row, or more than one band of
// columns, shows as slices, one in each row in the first column of each
// band that it covers. So each page shows the part of the image on it, as
// Excel and LibreOffice print it.
func (w *writer) imagesAt(i, k int, band []int, m *mergeState) []placed {
	var out []placed
	c := band[k]
	for n, img := range w.images {
		row1, col1 := w.ends[n][0], w.ends[n][1]
		switch {
		case m == nil && img.Row == i && img.Col == c && row1 == i && w.inBand(band[k:], col1):
			// the image fits in the row and in the band
			out = append(out, placed{img: img, dx: img.DX, dy: img.DY})
		case m == nil && i >= img.Row && i <= row1 && row1-img.Row < maxSliceRows &&
			c >= img.Col && c <= col1 && (k == 0 || band[k-1] < img.Col):
			p := placed{img: img, dx: img.DX, dy: img.DY, slice: true, clipH: w.sh.rowHeight(w.sh.row(i))}
			for cc := img.Col; cc < c && cc-img.Col < 1024; cc++ {
				p.dx -= w.sh.colWidth(cc)
			}
			for rr := img.Row; rr < i; rr++ {
				p.dy -= w.sh.rowHeight(w.sh.row(rr))
			}
			for j := k; j < len(band) && band[j] <= col1; j++ {
				p.clipW += w.sh.colWidth(band[j])
			}
			out = append(out, p)
		case m == nil && img.Row == i && img.Col == c:
			// a taller image shows whole in its first cell
			out = append(out, placed{img: img, dx: img.DX, dy: img.DY})
		case m != nil && img.Row >= m.m.Row0 && img.Row <= m.m.Row1 && img.Col >= m.m.Col0 && img.Col <= m.m.Col1 &&
			img.Row >= i && img.Col >= c:
			dx, dy := img.DX, img.DY
			for cc := c; cc < img.Col; cc++ {
				dx += w.sh.colWidth(cc)
			}
			for rr := i; rr < img.Row && rr-i < 4096; rr++ {
				dy += w.sh.rowHeight(w.sh.row(rr))
			}
			out = append(out, placed{img: img, dx: dx, dy: dy})
		}
	}
	return out
}

// inBand reports whether the columns of band hold every column up to col
// without a gap.
func (w *writer) inBand(band []int, col int) bool {
	for j, c := range band {
		if c == col {
			return true
		}
		if j > 0 && c != band[j-1]+1 {
			return false
		}
	}
	return false
}

// image writes an image in its cell.
func (w *writer) image(p placed) {
	if !w.spend(2) {
		return
	}
	s := w.scale
	img := p.img
	pos := "left:" + pt(p.dx*s) + ";top:" + pt(p.dy*s) + ";width:" + pt(img.Width*s) + ";height:" + pt(img.Height*s)
	b := &w.body
	if p.slice {
		if p.clipW <= 0 || p.clipH <= 0 {
			return
		}
		if img.Object != "" || img.Path == "" {
			// the slice of a chart or of a frame reaches 1 point into the
			// next row, which draws its own slice over it, so that no
			// seam shows between the rows
			b.WriteString(`<div class="cl" style="width:` + pt(p.clipW*s) + ";height:" + pt((p.clipH+1)*s) + `">`)
			w.imageTag(img, pos)
			b.WriteString("</div>")
			return
		}
		b.WriteString(`<div class="cl" style="width:` + pt(p.clipW*s) + ";height:" + pt(p.clipH*s) + `">`)
		w.imageTag(img, pos)
		b.WriteString("</div>")
		w.strip(p)
		return
	}
	w.imageTag(img, pos)
}

// strip draws the picture of the slice p again in a strip of 2 points
// across the bottom edge of its row, when the picture goes on in the next
// row. The two rows clip the picture at the same line, and each one draws
// a part of a pixel on the line, which leaves a light seam. The strip
// draws the whole pixel. It leaves out stripInset at each side of the
// picture, whose smoothed pixels would draw twice there and leave a dark
// mark.
func (w *writer) strip(p placed) {
	img := p.img
	if p.dy+img.Height <= p.clipH+0.01 || !w.spend(2) {
		return
	}
	left := max(0, p.dx) + stripInset
	right := min(p.clipW, p.dx+img.Width) - stripInset
	if right <= left {
		return
	}
	s := w.scale
	top := p.clipH - stripHalf
	pos := "left:" + pt((p.dx-left)*s) + ";top:" + pt((p.dy-top)*s) + ";width:" + pt(img.Width*s) + ";height:" + pt(img.Height*s)
	b := &w.body
	b.WriteString(`<div class="cl" style="left:` + pt(left*s) + ";top:" + pt(top*s) + ";width:" + pt((right-left)*s) +
		";height:" + pt(2*stripHalf*s) + `">`)
	w.imageTag(img, pos)
	b.WriteString("</div>")
}

// stripHalf is the half height of the strip of a slice of a picture, and
// stripInset the width that it leaves out at each side of the picture, in
// points. The inset is a pixel at 96 dots per inch.
const (
	stripHalf  = 1
	stripInset = 0.75
)

// imageTag writes the element of the image img at the position pos.
func (w *writer) imageTag(img Image, pos string) {
	b := &w.body
	switch {
	case img.Object != "":
		b.WriteString(`<img style="position:absolute;` + pos + `" src="` + html.EscapeString(layout.ObjectURL(img.Object)) + `" alt="">`)
	case img.Path != "":
		b.WriteString(`<img style="position:absolute;` + pos + `" src="` + html.EscapeString(img.Path) + `" alt="">`)
	default:
		b.WriteString(`<div class="ph" style="` + pos + `">` + html.EscapeString(img.Label) + `</div>`)
	}
}

// shrink returns the size of the font of a cell whose style shrinks its
// text to fit its columns, or 0 when the text fits. It guesses the width of
// the text from the size of the font.
func (w *writer) shrink(cell *Cell, st *Style, cols []int) float64 {
	size := st.Font.Size
	if size <= 0 {
		size = w.style(0).Font.Size
	}
	if size <= 0 {
		size = 11
	}
	width := 0.0
	for _, c := range cols {
		width += w.sh.colWidth(c)
	}
	width -= 2 * cellPadding
	n := utf8.RuneCountInString(cell.Text)
	need := float64(n) * size * 0.5
	if need <= width || need <= 0 || width <= 0 {
		return 0
	}
	return max(1, size*width/need) * w.scale
}

// overflow returns the signs # that a number shows when it is wider than
// its cell, as Excel shows it, or "" when it fits. It guesses the width of
// the text from the size of the font: a digit is about half as wide as the
// font is high.
func (w *writer) overflow(cell *Cell, st *Style, cols []int) string {
	if cell.Kind != KindNumber || st.Wrap || st.Shrink || st.Rotation != 0 || len(cell.Runs) > 0 {
		return ""
	}
	size := st.Font.Size
	if size <= 0 {
		size = w.style(0).Font.Size
	}
	if size <= 0 {
		size = 11
	}
	width := -2 * cellPadding
	for _, c := range cols {
		width += w.sh.colWidth(c)
	}
	need := 0.0
	for _, r := range cell.Text {
		switch {
		case r >= '0' && r <= '9', r == '$', r == '#', r == '%':
			need += 0.53
		case r == '.' || r == ',' || r == ' ' || r == ':' || r == '/' || r == '-':
			need += 0.28
		default:
			need += 0.6
		}
	}
	need *= size
	if need <= width*1.05 || width <= 0 {
		return ""
	}
	return strings.Repeat("#", max(1, min(256, int(width/(size*0.53)))))
}

// content writes the text of a cell. wrap keeps the breaks of the lines of
// a cell that wraps its text.
func (w *writer) content(cell *Cell, st *Style, wrap bool) {
	b := &w.body
	text := func(s string) string {
		if !st.Wrap || !wrap {
			s = strings.ReplaceAll(s, "\r\n", " ")
			s = strings.ReplaceAll(s, "\n", " ")
		}
		return html.EscapeString(s)
	}
	rot := st.Rotation
	switch {
	case rot == 255:
		// the letters stacked from top to bottom
		parts := make([]string, 0, len(cell.Text))
		for _, r := range cell.Text {
			parts = append(parts, html.EscapeString(string(r)))
		}
		b.WriteString(strings.Join(parts, "\n"))
		return
	case rot != 0 && rot >= -90 && rot <= 90:
		b.WriteString(`<span class="r" style="transform:rotate(` + strconv.Itoa(-rot) + `deg)">`)
		defer b.WriteString("</span>")
	}
	if len(cell.Runs) == 0 {
		b.WriteString(text(cell.Text))
		return
	}
	def := w.style(0).Font
	for _, r := range cell.Runs {
		if r.Font == nil {
			b.WriteString(text(r.Text))
			continue
		}
		// A run takes the family, the size and the color of the cell
		// that it does not set.
		f := *r.Font
		if f.Family == "" {
			f.Family, f.Generic = st.Font.Family, st.Font.Generic
		}
		if f.Size <= 0 {
			f.Size = st.Font.Size
		}
		if f.Color.A == 0 {
			f.Color = st.Font.Color
		}
		var css strings.Builder
		fontCSS(&css, f, def, w.scale)
		b.WriteString(`<span style="` + html.EscapeString(css.String()) + `">` + text(r.Text) + "</span>")
	}
}
