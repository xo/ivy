package msxls

import (
	"context"
	"image/color"
	"slices"
	"strconv"
	"unicode/utf16"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/numfmt"
	"github.com/xo/ivy/internal/odraw"
	"github.com/xo/ivy/internal/sheet"
)

// This file converts the records of a workbook into the model of
// internal/sheet, which puts the sheets on pages as xlsx and odf do.

// defaultPalette is the palette of a workbook without the record PALETTE
// (MS-XLS 2.5.161). The colors 0 to 7 repeat the colors 8 to 15.
var defaultPalette = func() [64]color.NRGBA {
	rgb := [...]uint32{
		0x000000, 0xFFFFFF, 0xFF0000, 0x00FF00, 0x0000FF, 0xFFFF00, 0xFF00FF, 0x00FFFF,
		0x000000, 0xFFFFFF, 0xFF0000, 0x00FF00, 0x0000FF, 0xFFFF00, 0xFF00FF, 0x00FFFF,
		0x800000, 0x008000, 0x000080, 0x808000, 0x800080, 0x008080, 0xC0C0C0, 0x808080,
		0x9999FF, 0x993366, 0xFFFFCC, 0xCCFFFF, 0x660066, 0xFF8080, 0x0066CC, 0xCCCCFF,
		0x000080, 0xFF00FF, 0xFFFF00, 0x00FFFF, 0x800080, 0x800000, 0x008080, 0x0000FF,
		0x00CCFF, 0xCCFFFF, 0xCCFFCC, 0xFFFF99, 0x99CCFF, 0xFF99CC, 0xCC99FF, 0xFFCC99,
		0x3366FF, 0x33CCCC, 0x99CC00, 0xFFCC00, 0xFF9900, 0xFF6600, 0x666699, 0x969696,
		0x003366, 0x339966, 0x003300, 0x333300, 0x993300, 0x993366, 0x333399, 0x333333,
	}
	var p [64]color.NRGBA
	for i, v := range rgb {
		p[i] = color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
	}
	return p
}()

// color returns the color of the index icv of the palette. The system
// colors 64 and 0x7FFF are black, the color of the text, and 65 is white,
// the color of the window.
func (w *workbook) color(icv int) color.NRGBA {
	switch {
	case icv >= 0 && icv < len(w.palette):
		return w.palette[icv]
	case icv == 65:
		return color.NRGBA{255, 255, 255, 255}
	}
	return color.NRGBA{0, 0, 0, 255}
}

// builtinFormat returns the code of a number format that a workbook does
// not write.
func builtinFormat(i int) (string, bool) {
	return numfmt.Builtin(i)
}

// The horizontal and the vertical alignments of an XF.
var (
	hAligns = [8]string{"", "left", "center", "right", "fill", "justify", "centerContinuous", "distributed"}
	vAligns = [8]string{"top", "center", "bottom", "justify", "distributed", "bottom", "bottom", "bottom"}
)

// borderStyles are the widths in points and the lines of the styles of a
// border (MS-XLS 2.5.11).
var borderStyles = [14]sheet.Border{
	{},
	{Width: 0.75, Line: "solid"},
	{Width: 1.5, Line: "solid"},
	{Width: 0.75, Line: "dashed"},
	{Width: 0.75, Line: "dotted"},
	{Width: 2.25, Line: "solid"},
	{Width: 2.25, Line: "double"},
	{Width: 0.5, Line: "solid"},
	{Width: 1.5, Line: "dashed"},
	{Width: 0.75, Line: "dashed"},
	{Width: 1.5, Line: "dashed"},
	{Width: 0.75, Line: "dotted"},
	{Width: 1.5, Line: "dotted"},
	{Width: 1.5, Line: "dashed"},
}

// border returns the border of the style dg and the color icv.
func (w *workbook) border(dg, icv int) sheet.Border {
	if dg <= 0 || dg >= len(borderStyles) {
		return sheet.Border{}
	}
	b := borderStyles[dg]
	if c := w.color(icv); c != (color.NRGBA{0, 0, 0, 255}) {
		b.Color = c
	}
	return b
}

// style converts an XF into a style (MS-XLS 2.4.353). It reads the font,
// the alignment, the wrap, the indent, the rotation, the borders and the
// fill of BIFF8, and the font, the alignment and the wrap of BIFF5.
func (w *workbook) style(xf []byte) sheet.Style {
	var st sheet.Style
	if len(xf) < 7 {
		return st
	}
	st.Font = w.font(int(le.Uint16(xf)))
	st.HAlign = hAligns[xf[6]&0x07]
	st.Wrap = xf[6]&0x08 != 0
	if v := vAligns[xf[6]>>4&0x07]; v != "bottom" {
		st.VAlign = v
	}
	if w.version < biff8 || len(xf) < 20 {
		return st
	}
	switch rot := int(xf[7]); {
	case rot == 255:
		st.Rotation = 255
	case rot > 90 && rot <= 180:
		st.Rotation = 90 - rot
	case rot <= 90:
		st.Rotation = rot
	}
	st.Indent = float64(xf[8]&0x0F) * sheet.IndentStep
	st.Shrink = xf[8]&0x10 != 0
	dg := le.Uint16(xf[10:])
	icv := le.Uint16(xf[12:])
	more := le.Uint32(xf[14:])
	st.Borders = [4]sheet.Border{
		w.border(int(dg>>8&0x0F), int(more&0x7F)),
		w.border(int(dg>>4&0x0F), int(icv>>7&0x7F)),
		w.border(int(dg>>12&0x0F), int(more>>7&0x7F)),
		w.border(int(dg&0x0F), int(icv&0x7F)),
	}
	if fls := more >> 26; fls != 0 {
		// a pattern is drawn as its color in front
		fore := int(le.Uint16(xf[18:]) & 0x7F)
		if fore != 64 {
			st.Fill = w.color(fore)
		}
	}
	return st
}

// numberFormatOf returns the code of the number format of the XF xf.
func (w *workbook) numberFormatOf(xf int) string {
	if xf < 0 || xf >= len(w.xfs) || len(w.xfs[xf]) < 4 {
		return "General"
	}
	return w.numberFormat(int(le.Uint16(w.xfs[xf][2:])))
}

// book converts the workbook into the model of internal/sheet: each
// worksheet that is not hidden, in order. paper is the paper of the program
// for a sheet whose file gives none.
func (w *workbook) book(ctx context.Context, title string, cfg ivy.Config) (*sheet.Workbook, error) {
	lim, paper := cfg.Limits, cfg.Paper
	pics := odraw.NewPictures(w.store, nil, cfg)
	book := &sheet.Workbook{Title: title, Excel: true}
	if len(w.xfs) > 0 {
		book.Styles = make([]sheet.Style, len(w.xfs))
		for i, xf := range w.xfs {
			book.Styles[i] = w.style(xf)
		}
	} else if len(w.fonts) > 0 {
		book.Styles = []sheet.Style{{Font: w.fonts[0]}}
	}
	def := w.font(0)
	digit := sheet.DigitOf(def, 10)
	if !(def.Size > 0) {
		def.Size = 10
	}
	rows := sheet.NewExcelRows(fontdb.Of(cfg), def)
	cells := 0
	book.Files = make(map[string][]byte)
	look := &chartLookup{ctx: ctx, lim: lim, w: w}
	for i, bs := range w.sheets {
		if (bs.kind != 0 && bs.kind != 2) || bs.hidden {
			continue
		}
		raw, err := w.readSheet(ctx, bs, lim)
		if err != nil {
			return nil, err
		}
		if bs.kind == 2 {
			// a chart sheet of Excel is in landscape by default
			raw.page.Landscape = true
		}
		// The paper of the program replaces US Letter, the paper of a sheet
		// without the record SETUP, and the margins stay. The record can
		// give its own paper (D32).
		raw.page.SetPaper(paper)
		if bs.kind == 2 {
			sh := w.chartSheet(raw)
			if obj := w.chartObject(look, raw.chart, sh.Images[0].Width, sh.Images[0].Height, cfg); obj != nil {
				name := "chart" + strconv.Itoa(len(book.Objects)+1)
				if book.Objects == nil {
					book.Objects = make(map[string]layout.Object)
				}
				book.Objects[name] = obj
				sh.Images[0].Label, sh.Images[0].Object = "", name
			}
			book.Sheets = append(book.Sheets, sh)
			continue
		}
		cells += len(raw.cells)
		sh := w.toSheet(raw, rows, digit, book.Styles)
		if p := w.prints[i]; p != nil {
			sh.PrintAreas, sh.TitleRows, sh.TitleCols = p.areas, p.rows, p.cols
		}
		w.drawings(ctx, sh, raw, book, pics, look, cfg)
		book.Sheets = append(book.Sheets, sh)
		if cells > lim.MaxObjects {
			break
		}
	}
	return book, nil
}

// toSheet converts a worksheet into a sheet of internal/sheet. The widths
// of the columns and the heights of the rows are those that Excel prints:
// the widths in the digits of the default font, and the heights with the
// scale of the default font (sheet.ExcelRows). Excel fits each row without
// fUnsynced to its cells.
func (w *workbook) toSheet(raw *rawSheet, rows *sheet.ExcelRows, digit sheet.Digit, bookStyles []sheet.Style) *sheet.Sheet {
	styles := len(bookStyles)
	sh := &sheet.Sheet{
		Name:             raw.name,
		DefaultRowHeight: raw.defHeight,
		Merges:           raw.merges,
		Page:             raw.page,
		Gridlines:        raw.gridlines,
		FixedRows:        true,
		RowScale:         rows.Scale(),
		Lines:            rows,
		RowDots:          true,
		Header:           sheet.ParseHeaderFooter(raw.header),
		Footer:           sheet.ParseHeaderFooter(raw.footer),
	}
	sh.Page.ScaleHeaderFooter = raw.scaleHeaderFooter()
	// the height of a row without a record ROW, or the default height of
	// the default font
	if raw.defHeight > 0 {
		sh.DefaultRowHeight = rows.Printed(raw.defHeight)
	} else {
		sh.DefaultRowHeight = rows.Default()
	}
	if raw.stdWidth > 0 {
		sh.DefaultColWidth = digit.Column(raw.stdWidth)
	} else {
		sh.DefaultColWidth = digit.DefaultColumn(raw.defWidth)
	}
	styleOf := func(xf int) int {
		if xf < 0 || xf >= styles {
			return 0
		}
		return xf
	}
	// the columns, sorted and without overlaps
	slices.SortStableFunc(raw.cols, func(a, b colInfo) int { return a.first - b.first })
	next := 0
	for _, c := range raw.cols {
		if c.first < next {
			continue
		}
		sh.Cols = append(sh.Cols, sheet.Col{
			Min: c.first, Max: c.last,
			Width:  digit.Column(c.width),
			Hidden: c.hidden,
			Style:  styleOf(c.xf),
		})
		next = c.last + 1
	}
	// the rows with their cells, sorted, each column once
	byRow := make(map[int][]cell)
	for _, c := range raw.cells {
		byRow[c.row] = append(byRow[c.row], c)
	}
	indexes := make([]int, 0, len(byRow)+len(raw.rows))
	for r := range byRow {
		indexes = append(indexes, r)
	}
	for r := range raw.rows {
		if _, ok := byRow[r]; !ok {
			indexes = append(indexes, r)
		}
	}
	slices.Sort(indexes)
	sh.Rows = make([]sheet.Row, 0, len(indexes))
	for _, r := range indexes {
		info := raw.rows[r]
		row := sheet.Row{Index: r, Height: rows.Printed(info.height), Hidden: info.hidden, Fit: info.fit, Style: styleOf(info.xf)}
		cs := byRow[r]
		slices.SortStableFunc(cs, func(a, b cell) int { return a.col - b.col })
		for i, c := range cs {
			if i+1 < len(cs) && cs[i+1].col == c.col {
				// a later record of the same cell wins
				continue
			}
			row.Cells = append(row.Cells, w.toCell(c, styleOf(c.xf)))
		}
		sh.Rows = append(sh.Rows, row)
	}
	rows.Fit(sh, bookStyles)
	w.setup(sh, raw)
	w.condFormats(sh, raw)
	if raw.truncated {
		sh.Note = "The workbook holds more cells than the limit of ivy. The sheet stops at the limit."
	}
	return sh
}

// chartSheet converts a chart sheet into a sheet whose page shows its
// chart, or a frame when the package cannot read the chart.
func (w *workbook) chartSheet(raw *rawSheet) *sheet.Sheet {
	sh := &sheet.Sheet{
		Name:   raw.name,
		Page:   raw.page,
		Header: sheet.ParseHeaderFooter(raw.header),
		Footer: sheet.ParseHeaderFooter(raw.footer),
	}
	sh.Page.ScaleHeaderFooter = raw.scaleHeaderFooter()
	w.setup(sh, raw)
	// The chart fills the page: one cell of the size of the page holds it.
	p := sh.Page
	pw, ph := p.Width, p.Height
	if p.Landscape {
		pw, ph = ph, pw
	}
	cw, rh := max(72, pw-p.Left-p.Right-1), max(72, ph-p.Top-p.Bottom-1)
	sh.DefaultColWidth, sh.DefaultRowHeight = cw, rh
	sh.Page.Scale, sh.Page.Fit = 1, false
	sh.Images = []sheet.Image{{Width: cw - 2, Height: rh - 2, Label: "Chart"}}
	return sh
}

// toCell converts a cell, with the text that its number format gives.
func (w *workbook) toCell(c cell, style int) sheet.Cell {
	out := sheet.Cell{Col: c.col, Style: style}
	code := w.numberFormatOf(c.xf)
	switch c.kind {
	case cellNumber:
		out.Kind, out.Value = sheet.KindNumber, c.num
		res := numfmt.Number(code, c.num, w.date1904)
		out.Text, out.AsText = res.Text, res.AsText
		if res.Color > 0 && res.Color+7 < len(w.palette) {
			out.Color = w.palette[res.Color+7]
		}
	case cellBool:
		out.Kind = sheet.KindBool
		out.Text = "FALSE"
		if c.num != 0 {
			out.Text = "TRUE"
		}
	case cellError:
		out.Kind = sheet.KindError
		out.Text = c.text
	case cellText:
		out.Kind = sheet.KindText
		out.Text = numfmt.Text(code, c.text).Text
		if len(c.runs) > 0 && out.Text == c.text {
			out.Runs = w.runs(c.text, c.runs)
		}
	}
	return out
}

// runs splits the text of a rich string at its runs of fonts. The runs
// count the characters in UTF-16.
func (w *workbook) runs(text string, runs []fontRun) []sheet.Run {
	units := utf16.Encode([]rune(text))
	var out []sheet.Run
	pos := 0
	var font *sheet.Font
	for _, r := range runs {
		ich := min(max(r.ich, pos), len(units))
		if ich > pos {
			out = append(out, sheet.Run{Text: string(utf16.Decode(units[pos:ich])), Font: font})
		}
		f := w.font(r.ifnt)
		font, pos = &f, ich
	}
	if pos < len(units) {
		out = append(out, sheet.Run{Text: string(utf16.Decode(units[pos:])), Font: font})
	}
	return out
}

// setup reads the record SETUP into the page of sh (MS-XLS 2.4.257).
func (w *workbook) setup(sh *sheet.Sheet, raw *rawSheet) {
	slices.Sort(raw.rowBreaks)
	slices.Sort(raw.colBreaks)
	sh.RowBreaks = slices.Compact(raw.rowBreaks)
	sh.ColBreaks = slices.Compact(raw.colBreaks)
	b := raw.setup
	if len(b) < 12 {
		return
	}
	flags := le.Uint16(b[10:])
	// fLeftToRight: the pages go over, then down
	sh.Page.OverThenDown = flags&0x0001 != 0
	if flags&0x0004 != 0 {
		// fNoPls: the paper, the scale and the orientation are not set,
		// and the sheet keeps the paper of the program, or US Letter
		return
	}
	own := false
	if w, h, ok := sheet.PaperSize(int(le.Uint16(b))); ok {
		sh.Page.Width, sh.Page.Height, own = w, h, true
	}
	if scale := int(le.Uint16(b[2:])); scale >= 10 && scale <= 400 {
		sh.Page.Scale = float64(scale) / 100
	}
	switch {
	case flags&0x0040 == 0:
		// fPortrait
		sh.Page.Landscape = flags&0x0002 == 0
	case own:
		// fNoOrient: fPortrait is not valid, and the paper of the file is
		// in portrait
		sh.Page.Landscape = false
	}
	if raw.fit {
		sh.Page.Fit = true
		sh.Page.FitWidth = int(le.Uint16(b[6:]))
		sh.Page.FitHeight = int(le.Uint16(b[8:]))
	}
	if len(b) >= 32 {
		if v := f64(b[16:]) * 72; v >= 0 && v < 1000 {
			sh.Page.Header = v
		}
		if v := f64(b[24:]) * 72; v >= 0 && v < 1000 {
			sh.Page.Footer = v
		}
	}
}
