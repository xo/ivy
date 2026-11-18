package odf

import (
	"context"
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/chart"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/numfmt"
	"github.com/xo/ivy/internal/sheet"
	"github.com/xo/ivy/odf/internal/drawpage"
)

// sheetPager gives the pages of a spreadsheet: each table that prints, in
// order, as printing does (D24). It converts the tables into HTML tables
// for the layout of D10 with the package sheet.
//
// The cells show the text of their text:p elements, which the program
// that wrote the file formatted. A cell without a text:p shows its value. No
// formula is computed. The pager reads the widths of the columns and the
// heights of the rows, the hidden rows and columns, the repeated rows and
// columns, the merged cells, the styles of the cells with their parents,
// the page layouts of the master pages with their headers and footers, the
// scale, the print ranges, the rows and the columns of the print titles,
// the manual page breaks, the grid of the page layout, and the images of
// the cells and of the sheet, and the charts of the embedded objects, which
// internal/chart draws (D27). An object that is not a chart shows
// as a frame, and odf/internal/drawpage draws the shapes. The conditions of
// the style of a cell, style:map, give the cell the style of the first
// condition that is true. The color scales, the data bars and the icon
// sets of LibreOffice are left out, and so are the annotations. The pages
// go down, then over, whatever style:print-page-order says.
//
// A run of repeated rows or cells that holds no text and is longer than
// odsMaxEmptyRun is left out, as the trailing runs that LibreOffice writes
// to the end of the sheet. A workbook reads at most Limits.MaxObjects
// cells, and a sheet shows at most Limits.MaxObjects / 4 cells and rows.
type sheetPager struct {
	d *Document

	mu     sync.Mutex
	src    *layout.Source
	srcErr error
	pages  ebook.Pages
}

// odsLineHeight is the height of a line of a header or a footer, as a
// share of the size of its font. It is the height that LibreOffice gives
// the area of a header of one line, measured on its pages.
const odsLineHeight = 1.0

// odsMaxEmptyRun is the longest run of repeated rows or cells without text
// that a sheet keeps.
const odsMaxEmptyRun = 256

// newSheetPager returns the pager of the spreadsheets.
func newSheetPager(d *Document) (pager, error) {
	return &sheetPager{d: d}, nil
}

// count satisfies the pager interface.
func (p *sheetPager) count(ctx context.Context) (int, error) {
	return p.pages.PageCount(ctx, p.source, p.d.cfg)
}

// page satisfies the pager interface.
func (p *sheetPager) page(ctx context.Context, n int) (ivy.Page, error) {
	return p.pages.Page(ctx, p.source, p.d.cfg, n)
}

// source converts the spreadsheet into HTML for the layout, once.
func (p *sheetPager) source(ctx context.Context) (src layout.Source, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case p.srcErr != nil:
		return layout.Source{}, p.srcErr
	case p.src != nil:
		return *p.src, nil
	}
	defer func() {
		switch {
		case err == nil:
			p.src = &src
		case ctx.Err() == nil:
			p.srcErr = err
		}
	}()
	defer ebook.Recover("converting the spreadsheet", &err)
	wb, err := p.d.odsWorkbook(ctx)
	if err != nil {
		return layout.Source{}, err
	}
	return wb.Source(ctx, sheet.Options{Limits: p.d.lim, Logger: p.d.cfg.Logger, Paper: p.d.cfg.Paper})
}

// odsReader is the state of the reader of a spreadsheet.
type odsReader struct {
	ctx    context.Context
	d      *Document
	styles *odsStyles
	wb     *sheet.Workbook
	// index maps the name of a style of a cell to its index in
	// wb.Styles.
	index map[string]int
	// nCells counts the cells of the workbook.
	nCells  int
	objects int
	// cellProps holds the properties of each style of wb.Styles, and maps
	// the conditions of the styles that have some, by their index.
	// condIndex caches the styles of the conditions, by the index of the
	// style of the cell and the name of the style of the condition.
	cellProps []map[string]string
	maps      map[int][]odsMap
	condIndex map[string]int
	// conds is set when a style of a cell has conditions, so that the
	// reader keeps the values of the cells.
	conds bool
	// content and styles are content.xml and styles.xml, and styles can
	// be nil. shapes draws their shapes, from the first shape.
	content, stylesDoc *roxmltree.Document
	shapes             *drawpage.Shapes
}

// odsParse reads and parses the entry name.
func (d *Document) odsParse(ctx context.Context, name string) (*roxmltree.Document, error) {
	buf, err := d.readEntry(ctx, name)
	if err != nil {
		return nil, err
	}
	doc, err := ebook.ParseXML(buf, d.lim)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	return doc, nil
}

// odsWorkbook reads the tables of the spreadsheet.
func (d *Document) odsWorkbook(ctx context.Context) (*sheet.Workbook, error) {
	r := &odsReader{ctx: ctx, d: d, styles: newODSStyles(), index: make(map[string]int), maps: map[int][]odsMap{}, condIndex: map[string]int{}}
	if doc, err := d.odsParse(ctx, "styles.xml"); err == nil {
		r.styles.read(doc)
		r.stylesDoc = doc
	} else if ctx.Err() != nil {
		return nil, err
	}
	content, err := d.odsParse(ctx, "content.xml")
	if err != nil {
		return nil, err
	}
	r.content = content
	r.styles.read(content)
	for _, st := range r.styles.styles {
		r.conds = r.conds || st.family == "table-cell" && len(st.maps) > 0
	}
	r.wb = &sheet.Workbook{Files: make(map[string][]byte)}
	// The default style of the cells: the default style of the family
	// with the style Default.
	defProps := r.styles.props("table-cell", "Default")
	def := r.styles.cellStyle(defProps)
	if def.Font.Size <= 0 {
		def.Font.Size = 10
	}
	if def.Font.Family == "" {
		def.Font.Family = "Liberation Sans"
	}
	r.wb.Styles = []sheet.Style{def}
	r.cellProps = []map[string]string{defProps}
	r.index["Default"] = 0
	var spreadsheet roxmltree.Node
	found := false
	for n := range content.RootElement().Descendants() {
		if odsLocal(n) == "spreadsheet" {
			spreadsheet, found = n, true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("content.xml without office:spreadsheet: %w", ivy.ErrFormat)
	}
	for t := range spreadsheet.Children() {
		if odsLocal(t) != "table" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("reading the spreadsheet: %w", err)
		}
		sh, ok := r.table(t)
		if ok {
			r.wb.Sheets = append(r.wb.Sheets, sh)
		}
	}
	return r.wb, nil
}

// styleIndex returns the index in wb.Styles of the style of a cell, and
// adds it on its first use.
func (r *odsReader) styleIndex(name string) int {
	if name == "" {
		return 0
	}
	if i, ok := r.index[name]; ok {
		return i
	}
	if r.styles.style("table-cell", name) == nil || len(r.wb.Styles) >= r.d.lim.MaxObjects {
		r.index[name] = 0
		return 0
	}
	p := r.styles.props("table-cell", name)
	i := len(r.wb.Styles)
	r.wb.Styles = append(r.wb.Styles, r.styles.cellStyle(p))
	r.cellProps = append(r.cellProps, p)
	if maps := r.styles.style("table-cell", name).maps; len(maps) > 0 {
		r.maps[i] = maps
	}
	r.index[name] = i
	return i
}

// odsTable is the state of the reader of one table.
type odsTable struct {
	sh   *sheet.Sheet
	row  int
	col  int
	stop bool
	// colStyles are the default styles of the cells of the columns.
	colStyles []sheet.Col
	// titleRows and titleCols are the rows and the columns of the
	// header elements.
	titleRows, titleCols *sheet.Span
	// conds are the cells whose style has conditions, and values the
	// values of the cells, by row and column, for the references of the
	// conditions.
	conds  []odsCond
	values map[[2]int]odsValue
}

// table reads a table:table. It returns false for a table that does not
// print: a hidden table, or one with table:print="false".
func (r *odsReader) table(t roxmltree.Node) (*sheet.Sheet, bool) {
	if ebook.Attr(t, "print") == "false" {
		return nil, false
	}
	st := r.styles.style("table", ebook.Attr(t, "style-name"))
	if st != nil && r.styles.props("table", ebook.Attr(t, "style-name"))["display"] == "false" {
		return nil, false
	}
	sh := &sheet.Sheet{
		Name:             ebook.Attr(t, "name"),
		DefaultColWidth:  64,
		DefaultRowHeight: 12.8,
	}
	master := "Default"
	if st != nil && st.master != "" {
		master = st.master
	}
	r.pageSetup(sh, master)
	ts := &odsTable{sh: sh, values: map[[2]int]odsValue{}}
	r.walk(ts, t, 0)
	r.applyConds(sh, ts.conds, ts.values)
	sh.Cols = append(sh.Cols, ts.colStyles...)
	for i := range sh.Images {
		if img := &sh.Images[i]; img.Row < 0 {
			img.Row, img.Col, img.DX, img.DY = odsAnchor(sh, img.DX, img.DY)
		}
	}
	sh.TitleRows, sh.TitleCols = ts.titleRows, ts.titleCols
	for _, p := range strings.Fields(ebook.Attr(t, "print-ranges")) {
		if rg, ok := odsRange(p); ok {
			sh.PrintAreas = append(sh.PrintAreas, rg)
		}
	}
	sh.Sort()
	return sh, true
}

// walk reads the columns and the rows of the element n of a table, and of
// its groups, at the depth depth.
func (r *odsReader) walk(ts *odsTable, n roxmltree.Node, depth int) {
	if depth > r.d.lim.MaxDepth {
		return
	}
	for c := range n.Children() {
		if ts.stop {
			return
		}
		switch odsLocal(c) {
		case "table-column":
			r.column(ts, c)
		case "table-header-columns":
			first := ts.col
			r.walk(ts, c, depth+1)
			if ts.col > first {
				ts.titleCols = &sheet.Span{First: first, Last: ts.col - 1}
			}
		case "table-columns", "table-column-group":
			r.walk(ts, c, depth+1)
		case "table-row":
			r.row(ts, c)
		case "table-header-rows":
			first := ts.row
			r.walk(ts, c, depth+1)
			if ts.row > first {
				ts.titleRows = &sheet.Span{First: first, Last: ts.row - 1}
			}
		case "table-rows", "table-row-group":
			r.walk(ts, c, depth+1)
		case "shapes":
			for f := range c.Children() {
				if odsLocal(f) == "frame" {
					r.frame(ts.sh, f, -1, -1)
				} else if odtIsShape(f) {
					r.shape(ts.sh, f, -1, -1)
				}
			}
		}
	}
}

// odsRepeat reads the attribute of a repeat, at least 1.
func odsRepeat(n roxmltree.Node, name string) int {
	v, err := strconv.Atoi(strings.TrimSpace(ebook.Attr(n, name)))
	if err != nil || v < 1 {
		return 1
	}
	return v
}

// column reads a table:table-column.
func (r *odsReader) column(ts *odsTable, c roxmltree.Node) {
	n := odsRepeat(c, "number-columns-repeated")
	if ts.col >= sheet.MaxCols {
		return
	}
	col := sheet.Col{Min: ts.col, Max: min(ts.col+n, sheet.MaxCols) - 1}
	ts.col = col.Max + 1
	p := r.styles.props("table-column", ebook.Attr(c, "style-name"))
	if w, ok := odsLength(p["column-width"]); ok && w > 0 {
		col.Width = math.Min(w, 2000)
	}
	switch ebook.Attr(c, "visibility") {
	case "collapse", "filter":
		col.Hidden = true
	}
	if name := ebook.Attr(c, "default-cell-style-name"); name != "" && name != "Default" {
		col.Style = r.styleIndex(name)
	}
	if p["break-before"] == "page" {
		ts.sh.ColBreaks = append(ts.sh.ColBreaks, col.Min)
	}
	if col.Width > 0 || col.Hidden || col.Style != 0 {
		ts.colStyles = append(ts.colStyles, col)
	}
}

// colStyle returns the default style of the cells of the column c.
func (ts *odsTable) colStyle(c int) int {
	for _, col := range ts.colStyles {
		if c >= col.Min && c <= col.Max {
			return col.Style
		}
	}
	return 0
}

// row reads a table:table-row and its cells.
func (r *odsReader) row(ts *odsTable, rn roxmltree.Node) {
	n := odsRepeat(rn, "number-rows-repeated")
	if ts.row >= sheet.MaxRows {
		ts.stop = true
		return
	}
	row := sheet.Row{Index: ts.row}
	p := r.styles.props("table-row", ebook.Attr(rn, "style-name"))
	if h, ok := odsLength(p["row-height"]); ok && h > 0 {
		row.Height = math.Min(h, 1000)
	}
	switch ebook.Attr(rn, "visibility") {
	case "collapse", "filter":
		row.Hidden = true
	}
	rowStyle := ebook.Attr(rn, "default-cell-style-name")
	if rowStyle != "" {
		row.Style = r.styleIndex(rowStyle)
	}
	if p["break-before"] == "page" {
		ts.sh.RowBreaks = append(ts.sh.RowBreaks, ts.row)
	}
	merges := len(ts.sh.Merges)
	var values []odsValue
	row.Cells, values = r.cells(ts, rn, rowStyle)
	rowMerges := ts.sh.Merges[merges:]
	hasText := false
	for _, c := range row.Cells {
		if c.Text != "" || len(c.Runs) > 0 {
			hasText = true
			break
		}
	}
	if !hasText && n > odsMaxEmptyRun {
		// a long run of empty rows, such as the rows to the end of the
		// sheet, keeps no row
		ts.row = min(ts.row+n, sheet.MaxRows)
		return
	}
	keep := len(row.Cells) > 0 || row.Height > 0 || row.Hidden || row.Style != 0
	for k := range n {
		if ts.row >= sheet.MaxRows {
			ts.stop = true
			return
		}
		if keep {
			if k > 0 {
				if r.nCells+len(row.Cells) > r.d.lim.MaxObjects {
					ts.sh.Note = "The workbook holds more cells than the limit of ivy. The sheet stops at the limit."
					ts.stop = true
					return
				}
				r.nCells += len(row.Cells)
			}
			rr := row
			rr.Index = ts.row
			if k > 0 {
				rr.Cells = append([]sheet.Cell(nil), row.Cells...)
				for _, m := range rowMerges {
					m.Row1 += ts.row - m.Row0
					m.Row0 = ts.row
					ts.sh.Merges = append(ts.sh.Merges, m)
				}
			}
			ts.sh.Rows = append(ts.sh.Rows, rr)
			r.keepValues(ts, rr, values)
		}
		ts.row++
	}
}

// keepValues keeps the values of the cells of the row rr, the last row of
// the sheet, for the conditions of the styles, and the cells whose style
// has conditions. values holds the value of each cell of rr.
func (r *odsReader) keepValues(ts *odsTable, rr sheet.Row, values []odsValue) {
	if !r.conds {
		return
	}
	for i, c := range rr.Cells {
		if i >= len(values) {
			break
		}
		if !values[i].empty && len(ts.values) < r.d.lim.MaxObjects {
			ts.values[[2]int{rr.Index, c.Col}] = values[i]
		}
		if len(r.maps[c.Style]) > 0 {
			ts.conds = append(ts.conds, odsCond{row: len(ts.sh.Rows) - 1, cell: i, value: values[i]})
		}
	}
}

// cells reads the cells of a row, and their values. rowStyle is the
// default style of the cells of the row.
func (r *odsReader) cells(ts *odsTable, rn roxmltree.Node, rowStyle string) ([]sheet.Cell, []odsValue) {
	var cells []sheet.Cell
	var values []odsValue
	col := 0
	for cn := range rn.Children() {
		kind := odsLocal(cn)
		if kind != "table-cell" && kind != "covered-table-cell" {
			continue
		}
		n := odsRepeat(cn, "number-columns-repeated")
		if col >= sheet.MaxCols {
			break
		}
		cell, hasText := r.cell(cn)
		var value odsValue
		if r.conds {
			value = odsCellValue(cell, func(name string) string { return ebook.Attr(cn, name) })
		}
		name := ebook.Attr(cn, "style-name")
		switch {
		case name != "":
			cell.Style = r.styleIndex(name)
		case rowStyle != "":
			cell.Style = r.styleIndex(rowStyle)
		default:
			cell.Style = ts.colStyle(col)
		}
		if len(cell.Runs) > 0 {
			// a span keeps the weight and the slant of its cell
			base := r.wb.Styles[cell.Style].Font
			for i := range cell.Runs {
				if f := cell.Runs[i].Font; f != nil {
					f.Bold = f.Bold || base.Bold
					f.Italic = f.Italic || base.Italic
				}
			}
		}
		if kind == "covered-table-cell" || (!hasText && n > odsMaxEmptyRun) {
			col += n
			continue
		}
		if spanC, spanR := odsRepeat(cn, "number-columns-spanned"), odsRepeat(cn, "number-rows-spanned"); spanC > 1 || spanR > 1 {
			ts.sh.Merges = append(ts.sh.Merges, sheet.Range{
				Row0: ts.row, Col0: col,
				Row1: min(ts.row+spanR, sheet.MaxRows) - 1, Col1: min(col+spanC, sheet.MaxCols) - 1,
			})
		}
		for f := range cn.Children() {
			if odsLocal(f) == "frame" {
				r.frame(ts.sh, f, ts.row, col)
			} else if odtIsShape(f) {
				r.shape(ts.sh, f, ts.row, col)
			}
		}
		for range n {
			if col >= sheet.MaxCols {
				break
			}
			if hasText || cell.Style != 0 {
				if r.nCells >= r.d.lim.MaxObjects {
					ts.sh.Note = "The workbook holds more cells than the limit of ivy. The sheet stops at the limit."
					ts.stop = true
					return cells, values
				}
				r.nCells++
				c := cell
				c.Col = col
				cells = append(cells, c)
				if r.conds {
					values = append(values, value)
				}
			}
			col++
		}
	}
	return cells, values
}

// cell reads the value and the text of a table:table-cell. It reports
// whether the cell holds text.
func (r *odsReader) cell(cn roxmltree.Node) (sheet.Cell, bool) {
	var c sheet.Cell
	var paras []string
	var runs []sheet.Run
	rich := false
	for p := range cn.Children() {
		if odsLocal(p) != "p" && odsLocal(p) != "h" {
			continue
		}
		if len(paras) > 0 {
			runs = append(runs, sheet.Run{Text: "\n"})
		}
		var b strings.Builder
		r.text(p, &b, &runs, &rich, nil, 0)
		paras = append(paras, b.String())
	}
	c.Text = strings.Join(paras, "\n")
	if rich {
		c.Runs = runs
	}
	valueType := ebook.Attr(cn, "value-type")
	switch valueType {
	case "float", "percentage", "currency", "date", "time", "boolean":
		// LibreOffice aligns a boolean to the right, as a number
		c.Kind = sheet.KindNumber
	}
	if len(paras) == 0 && valueType != "" && valueType != "string" {
		// a cell without a paragraph shows its value. An empty paragraph
		// is the text of a value whose format shows nothing, such as a
		// zero in the format #,###
		switch valueType {
		case "date":
			c.Text = ebook.Attr(cn, "date-value")
		case "time":
			c.Text = ebook.Attr(cn, "time-value")
		case "boolean":
			c.Text = strings.ToUpper(ebook.Attr(cn, "boolean-value"))
		default:
			if v, err := strconv.ParseFloat(ebook.Attr(cn, "value"), 64); err == nil {
				c.Text = numfmt.General(v)
			}
		}
	}
	for a := range cn.Attributes() {
		if a.Name() == "value-type" && a.Value() == "error" {
			// LibreOffice marks an error with calcext:value-type
			c.Kind = sheet.KindError
		}
	}
	return c, c.Text != ""
}

// text writes the text of a paragraph and of its spans into b, and into
// runs with the font of each span. It sets rich when a span has a font of
// its own.
func (r *odsReader) text(n roxmltree.Node, b *strings.Builder, runs *[]sheet.Run, rich *bool, font *sheet.Font, depth int) {
	if depth > 32 {
		return
	}
	add := func(s string) {
		b.WriteString(s)
		if k := len(*runs); k > 0 && (*runs)[k-1].Font == font && (*runs)[k-1].Text != "\n" {
			(*runs)[k-1].Text += s
			return
		}
		*runs = append(*runs, sheet.Run{Text: s, Font: font})
	}
	for c := range n.Children() {
		if c.IsText() {
			s, _ := c.Text()
			add(s)
			continue
		}
		switch odsLocal(c) {
		case "s":
			add(strings.Repeat(" ", min(odsRepeat(c, "c"), 1024)))
		case "tab":
			add("\t")
		case "line-break":
			add("\n")
		case "span", "a":
			f := font
			if name := ebook.Attr(c, "style-name"); name != "" && r.styles.style("text", name) != nil {
				sf := r.styles.font(r.styles.props("text", name))
				f = &sf
				*rich = true
			}
			r.text(c, b, runs, rich, f, depth+1)
		case "annotation", "note":
			// the comments are left out
		default:
			r.text(c, b, runs, rich, font, depth+1)
		}
	}
}

// odsRange parses a cell range of OpenDocument, such as "Sheet1.A1:Sheet1.D20"
// or "$'My Sheet'.$A$1:.$D$20".
func odsRange(s string) (sheet.Range, bool) {
	a, z, found := strings.Cut(s, ":")
	if !found {
		z = a
	}
	r0, c0, ok := odsCell(a)
	if !ok {
		return sheet.Range{}, false
	}
	r1, c1, ok := odsCell(z)
	if !ok {
		return sheet.Range{}, false
	}
	return sheet.Range{Row0: min(r0, r1), Col0: min(c0, c1), Row1: max(r0, r1), Col1: max(c0, c1)}, true
}

// odsCell parses a cell address, such as "Sheet1.$B$12", into its row and
// its column.
func odsCell(s string) (int, int, bool) {
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		s = s[i+1:]
	}
	s = strings.ReplaceAll(s, "$", "")
	i, col := 0, 0
	for i < len(s) && i < 3 {
		ch := s[i] | 0x20
		if ch < 'a' || ch > 'z' {
			break
		}
		col = col*26 + int(ch-'a'+1)
		i++
	}
	row, err := strconv.Atoi(s[i:])
	if i == 0 || err != nil || row < 1 || row > sheet.MaxRows || col > sheet.MaxCols {
		return 0, 0, false
	}
	return row - 1, col - 1, true
}

// pageSetup sets the page of the sheet from the master page name: its page
// layout and its header and footer.
func (r *odsReader) pageSetup(sh *sheet.Sheet, name string) {
	sh.Page = sheet.Page{
		Width: sheet.LetterWidth, Height: sheet.LetterHeight,
		Top: 56.69, Bottom: 56.69, Left: 56.69, Right: 56.69,
	}
	// ODF 1.3 gives no default to fo:page-width and fo:page-height. A
	// sheet without them takes the paper of the program, or US Letter, and
	// keeps its margins (D32).
	sh.Page.SetPaper(r.d.cfg.Paper)
	m, ok := r.styles.masters[name]
	if !ok {
		return
	}
	layout := r.styles.layouts[ebook.Attr(m, "page-layout-name")]
	if layout == nil {
		layout = &odsStyle{props: map[string]string{}}
	}
	p := layout.props
	pg := &sh.Page
	own := false
	if w, ok := odsLength(p["page-width"]); ok && w > 72 {
		if h, ok := odsLength(p["page-height"]); ok && h > 72 {
			pg.Width, pg.Height, own = w, h, true
		}
	}
	switch p["print-orientation"] {
	case "landscape":
		pg.Landscape = true
	case "portrait":
		pg.Landscape = false
	default:
		// the paper of the program keeps its orientation
		pg.Landscape = pg.Landscape && !own
	}
	if pg.Landscape != (pg.Width > pg.Height) {
		// the size of the paper is in the orientation of the page
		pg.Width, pg.Height = pg.Height, pg.Width
	}
	if pg.Width > pg.Height {
		pg.Width, pg.Height = pg.Height, pg.Width
	}
	marginOf := func(key string, def float64) float64 {
		if v, ok := odsLength(p[key]); ok && v >= 0 {
			return v
		}
		if v, ok := odsLength(p["margin"]); ok && v >= 0 {
			return v
		}
		return def
	}
	pg.Top, pg.Bottom = marginOf("margin-top", pg.Top), marginOf("margin-bottom", pg.Bottom)
	pg.Left, pg.Right = marginOf("margin-left", pg.Left), marginOf("margin-right", pg.Right)
	if v := strings.TrimSuffix(p["scale-to"], "%"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			pg.Scale = f / 100
		}
	}
	if n, err := strconv.Atoi(p["scale-to-pages"]); err == nil && n > 0 {
		pg.Fit = true
		pg.FitWidth, pg.FitHeight = n, n
	}
	if x, err := strconv.Atoi(p["scale-to-X"]); err == nil && x >= 0 {
		pg.Fit, pg.FitWidth = true, x
	}
	if y, err := strconv.Atoi(p["scale-to-Y"]); err == nil && y >= 0 {
		pg.Fit, pg.FitHeight = true, y
	}
	sh.Gridlines = strings.Contains(" "+p["print"]+" ", " grid ")
	pg.CenterH = p["table-centering"] == "horizontal" || p["table-centering"] == "both"
	pg.CenterV = p["table-centering"] == "vertical" || p["table-centering"] == "both"
	// The header and the footer take their height from the area of the
	// page: the cells start under them.
	pg.Header, pg.Footer = pg.Top, pg.Bottom
	for c := range m.Children() {
		kind := odsLocal(c)
		if (kind != "header" && kind != "footer") || ebook.Attr(c, "display") == "false" {
			continue
		}
		hf := r.headerFooter(c)
		if hf.Empty() {
			continue
		}
		// The area is as high as its lines of text and its spacing from
		// the cells, and at least its min-height, which holds the
		// spacing, as in LibreOffice.
		lines := 1
		for _, part := range [][]sheet.Field{hf.Left, hf.Center, hf.Right} {
			n := 1
			for _, f := range part {
				n += strings.Count(f.Text, "\n")
			}
			lines = max(lines, n)
		}
		size := r.wb.Styles[0].Font.Size
		if size <= 0 {
			size = 10
		}
		height := float64(min(lines, 16)) * size * odsLineHeight
		spacing := "margin-bottom"
		if kind == "footer" {
			spacing = "margin-top"
		}
		if v, ok := odsLength(p[kind+"-"+spacing]); ok {
			height += v
		}
		if v, ok := odsLength(p[kind+"-min-height"]); ok {
			height = math.Max(height, v)
		}
		if kind == "header" {
			sh.Header = hf
			pg.Top += height
		} else {
			sh.Footer = hf
			pg.Bottom += height
		}
	}
}

// headerFooter reads a style:header or a style:footer: its regions, or
// its paragraphs, which are its center.
func (r *odsReader) headerFooter(n roxmltree.Node) sheet.HeaderFooter {
	var hf sheet.HeaderFooter
	for c := range n.Children() {
		var part *[]sheet.Field
		switch odsLocal(c) {
		case "region-left":
			part = &hf.Left
		case "region-center", "p":
			part = &hf.Center
		case "region-right":
			part = &hf.Right
		default:
			continue
		}
		if odsLocal(c) == "p" {
			*part = append(*part, r.fields(c, 0)...)
			continue
		}
		for i, p := range odsChildren(c, "p") {
			if i > 0 {
				*part = append(*part, sheet.Field{Kind: sheet.FieldText, Text: "\n"})
			}
			*part = append(*part, r.fields(p, 0)...)
		}
	}
	return hf
}

// odsChildren returns the child elements of n with the local name name.
func odsChildren(n roxmltree.Node, name string) []roxmltree.Node {
	var out []roxmltree.Node
	for c := range n.Children() {
		if odsLocal(c) == name {
			out = append(out, c)
		}
	}
	return out
}

// fields reads the text and the fields of a paragraph of a header or a
// footer. The date and the time of the printing are left out.
func (r *odsReader) fields(n roxmltree.Node, depth int) []sheet.Field {
	var out []sheet.Field
	if depth > 16 {
		return nil
	}
	for c := range n.Children() {
		if c.IsText() {
			s, _ := c.Text()
			out = append(out, sheet.Field{Kind: sheet.FieldText, Text: s})
			continue
		}
		switch odsLocal(c) {
		case "sheet-name":
			out = append(out, sheet.Field{Kind: sheet.FieldSheet})
		case "page-number":
			out = append(out, sheet.Field{Kind: sheet.FieldPage})
		case "page-count":
			out = append(out, sheet.Field{Kind: sheet.FieldPages})
		case "title", "file-name":
			out = append(out, sheet.Field{Kind: sheet.FieldTitle})
		case "date", "time":
		case "s":
			out = append(out, sheet.Field{Kind: sheet.FieldText, Text: strings.Repeat(" ", min(odsRepeat(c, "c"), 64))})
		case "tab":
			out = append(out, sheet.Field{Kind: sheet.FieldText, Text: " "})
		default:
			out = append(out, r.fields(c, depth+1)...)
		}
	}
	return out
}

// odsImageExts are the extensions of the images that the layout draws.
var odsImageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".tif": true, ".tiff": true, ".webp": true, ".svg": true, ".emf": true, ".wmf": true,
}

// frame reads a draw:frame: an image or a chart. A frame of a cell is
// anchored at the cell of row and col. A frame of the sheet has a row of
// -1, and table anchors it later at the cell that holds its position.
func (r *odsReader) frame(sh *sheet.Sheet, f roxmltree.Node, row, col int) {
	if r.objects >= 4096 {
		return
	}
	w, _ := odsLength(ebook.Attr(f, "width"))
	h, _ := odsLength(ebook.Attr(f, "height"))
	x, _ := odsLength(ebook.Attr(f, "x"))
	y, _ := odsLength(ebook.Attr(f, "y"))
	if w <= 0 || h <= 0 || w > 14400 || h > 14400 {
		return
	}
	// a frame of the sheet keeps its position until the table has read
	// its columns and its rows
	img := sheet.Image{Row: row, Col: col, DX: math.Max(0, x), DY: math.Max(0, y), Width: w, Height: h}
	for c := range f.Children() {
		href := strings.TrimPrefix(ebook.Attr(c, "href"), "./")
		switch odsLocal(c) {
		case "image":
			if href == "" || !odsImageExts[strings.ToLower(path.Ext(href))] {
				continue
			}
			if _, ok := r.wb.Files[href]; !ok {
				buf, err := r.d.readEntry(r.ctx, href)
				if err != nil {
					continue
				}
				r.wb.Files[href] = buf
			}
			img.Path = href
		case "object":
			if obj := r.chartObject(href, w, h); obj != nil {
				name := "chart" + strconv.Itoa(r.objects+1)
				if r.wb.Objects == nil {
					r.wb.Objects = make(map[string]layout.Object)
				}
				r.wb.Objects[name] = obj
				img.Object = name
				break
			}
			img.Label = "Chart"
			if title := r.chartTitle(href); title != "" {
				img.Label = "Chart: " + title
			}
		default:
			continue
		}
		r.objects++
		sh.Images = append(sh.Images, img)
		return
	}
}

// shape reads a shape of the namespace draw, such as draw:custom-shape or
// a group draw:g, which odf/internal/drawpage draws. A shape of a cell is
// anchored at the cell of row and col, as a frame. A shape of the sheet has
// a row of -1, and table anchors it later at the cell that holds its
// position.
func (r *odsReader) shape(sh *sheet.Sheet, n roxmltree.Node, row, col int) {
	if r.objects >= 4096 {
		return
	}
	if r.shapes == nil {
		cfg := r.d.cfg
		cfg.Limits = r.d.lim
		r.shapes = drawpage.NewShapes(drawpage.Options{Config: cfg, Read: r.d.readEntry}, r.content, r.stylesDoc)
	}
	s, ok := r.shapes.Shape(r.ctx, n, true)
	if !ok || s.W > 14400 || s.H > 14400 {
		return
	}
	r.objects++
	name := "shape" + strconv.Itoa(r.objects)
	if r.wb.Objects == nil {
		r.wb.Objects = make(map[string]layout.Object)
	}
	r.wb.Objects[name] = s
	sh.Images = append(sh.Images, sheet.Image{
		Row: row, Col: col, DX: math.Max(0, s.X), DY: math.Max(0, s.Y),
		Width: s.W, Height: s.H, Object: name,
	})
}

// odsAnchor returns the cell that holds the point (x, y) of a sheet, and
// the offset of the point in the cell. It walks the columns and the rows
// that the sheet has read so far, which are in order.
func odsAnchor(sh *sheet.Sheet, x, y float64) (int, int, float64, float64) {
	col, ci := 0, 0
	for ; col < sheet.MaxCols-1 && col < 4096; col++ {
		for ci < len(sh.Cols) && sh.Cols[ci].Max < col {
			ci++
		}
		w := sh.DefaultColWidth
		if ci < len(sh.Cols) && sh.Cols[ci].Min <= col && sh.Cols[ci].Width > 0 {
			w = sh.Cols[ci].Width
		}
		if x < w {
			break
		}
		x -= w
	}
	row, ri := 0, 0
	for ; row < sheet.MaxRows-1 && row < 65536; row++ {
		for ri < len(sh.Rows) && sh.Rows[ri].Index < row {
			ri++
		}
		h := sh.DefaultRowHeight
		if ri < len(sh.Rows) && sh.Rows[ri].Index == row && sh.Rows[ri].Height > 0 {
			h = sh.Rows[ri].Height
		}
		if y < h {
			break
		}
		y -= h
	}
	return row, col, math.Max(0, x), math.Max(0, y)
}

// chartObject returns the object that draws the chart of the embedded
// object dir in a frame of w by h points, or nil when the object is not a
// chart. internal/chart draws it (D27), with the fonts of the
// program.
func (r *odsReader) chartObject(dir string, w, h float64) layout.Object {
	name := path.Clean(dir)
	if name == "." || strings.HasPrefix(name, "..") || strings.HasPrefix(name, "/") || strings.Contains(dir, "://") {
		return nil
	}
	buf, err := r.d.readEntry(r.ctx, name+"/content.xml")
	if err != nil {
		return nil
	}
	cfg := r.d.cfg
	ch, err := chart.ParseODF(buf, chart.Options{Config: cfg, Fonts: fontdb.Of(cfg), Width: w, Height: h})
	if err != nil {
		cfg.Log().Debug("odf: object that is not a chart", "object", name, "err", err)
		return nil
	}
	return ch
}

// chartTitle returns the title of the chart of the object in the folder
// dir, or "".
func (r *odsReader) chartTitle(dir string) string {
	doc, err := r.d.odsParse(r.ctx, path.Join(dir, "content.xml"))
	if err != nil {
		return ""
	}
	for n := range doc.RootElement().Descendants() {
		if odsLocal(n) == "title" {
			return ebook.NodeText(n)
		}
	}
	return ""
}
