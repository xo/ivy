package xlsx

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/numfmt"
	"github.com/xo/ivy/internal/ooxml"
	"github.com/xo/ivy/internal/sheet"
)

// readWorksheet reads a worksheet part into sh.
func (b *book) readWorksheet(sh *sheet.Sheet, part string, index int) error {
	doc, err := b.parsePart(part)
	if err != nil {
		return err
	}
	root := doc.RootElement()
	def := b.styles.defaultFont()
	digit := sheet.DigitOf(def, 11)
	sh.DefaultColWidth = digit.DefaultColumn(8)
	sh.DefaultRowHeight = 15 * sheet.RowScale
	sh.FixedRows = true
	sh.Page = sheet.Page{
		Width: sheet.LetterWidth, Height: sheet.LetterHeight,
		Left: 0.7 * 72, Right: 0.7 * 72, Top: 0.75 * 72, Bottom: 0.75 * 72,
		Header: 0.3 * 72, Footer: 0.3 * 72,
		ScaleHeaderFooter: true,
	}
	// ECMA-376 Part 1, 18.3.1.63, gives paperSize the default 1, US Letter,
	// and orientation the default "default", portrait. The paper of the
	// program replaces that default, in its orientation (D32).
	sh.Page.SetPaper(b.cfg.Paper)
	fit := false
	var drawing string
	for n := range root.Children() {
		switch local(n) {
		case "sheetPr":
			if p, ok := child(n, "pageSetUpPr"); ok {
				fit = flag(p, "fitToPage", false)
			}
		case "sheetFormatPr":
			if v := number(n, "defaultRowHeight", 0); v > 0 && v < 410 {
				sh.DefaultRowHeight = v * sheet.RowScale
			}
			if v := number(n, "defaultColWidth", 0); v > 0 {
				sh.DefaultColWidth = digit.Column(v)
			} else if v := number(n, "baseColWidth", 0); v > 0 && v < 256 {
				sh.DefaultColWidth = digit.DefaultColumn(v)
			}
		case "cols":
			b.readCols(sh, n, digit)
		case "sheetData":
			if err := b.readSheetData(sh, n); err != nil {
				return err
			}
		case "mergeCells":
			for m := range children(n, "mergeCell") {
				if r, ok := parseRange(attr(m, "ref")); ok && (r.Row0 != r.Row1 || r.Col0 != r.Col1) {
					sh.Merges = append(sh.Merges, r)
				}
				if len(sh.Merges) >= b.lim.MaxObjects {
					break
				}
			}
		case "printOptions":
			sh.Gridlines = flag(n, "gridLines", false)
			sh.Page.CenterH = flag(n, "horizontalCentered", false)
		case "pageMargins":
			p := &sh.Page
			p.Left = margin(n, "left", p.Left)
			p.Right = margin(n, "right", p.Right)
			p.Top = margin(n, "top", p.Top)
			p.Bottom = margin(n, "bottom", p.Bottom)
			p.Header = margin(n, "header", p.Header)
			p.Footer = margin(n, "footer", p.Footer)
		case "pageSetup":
			p := &sh.Page
			own := false
			if w, h, ok := sheet.PaperSize(integer(n, "paperSize", 0)); ok {
				p.Width, p.Height, own = w, h, true
			}
			// a size of its own, such as "210mm", which wins over paperSize
			if pw, ok := paperLength(attr(n, "paperWidth")); ok {
				if ph, ok := paperLength(attr(n, "paperHeight")); ok {
					p.Width, p.Height, own = pw, ph, true
				}
			}
			p.Landscape = orientation(attr(n, "orientation"), p.Landscape && !own)
			p.Scale = math.Max(10, math.Min(400, number(n, "scale", 100))) / 100
			p.FitWidth = max(0, integer(n, "fitToWidth", 1))
			p.FitHeight = max(0, integer(n, "fitToHeight", 1))
		case "headerFooter":
			// scaleWithDoc is true by default (ECMA-376 Part 1, 18.3.1.46)
			sh.Page.ScaleHeaderFooter = flag(n, "scaleWithDoc", true)
			if h, ok := child(n, "oddHeader"); ok {
				sh.Header = sheet.ParseHeaderFooter(nodeText(h))
			}
			if f, ok := child(n, "oddFooter"); ok {
				sh.Footer = sheet.ParseHeaderFooter(nodeText(f))
			}
		case "rowBreaks":
			for brk := range children(n, "brk") {
				if id := integer(brk, "id", -1); id > 0 && id < sheet.MaxRows {
					sh.RowBreaks = append(sh.RowBreaks, id)
				}
			}
		case "colBreaks":
			for brk := range children(n, "brk") {
				if id := integer(brk, "id", -1); id > 0 && id < sheet.MaxCols {
					sh.ColBreaks = append(sh.ColBreaks, id)
				}
			}
		case "drawing":
			if r, ok := b.sheetRel(part, attr(n, "id")); ok {
				drawing = r
			}
		}
	}
	sh.Page.Fit = fit
	sh.Sort()
	b.printSetup(sh, index)
	if drawing != "" {
		b.readDrawing(sh, drawing)
	}
	return nil
}

// sheetRel returns the target of the relationship id of the part. It
// reads the relationships of each part once.
func (b *book) sheetRel(part, id string) (string, bool) {
	rels, ok := b.partRels[part]
	if !ok {
		list, err := b.doc.Relationships(part, b.lim)
		if err != nil {
			list = nil
		}
		rels = make(map[string]ooxml.Relationship, len(list))
		for _, r := range list {
			rels[r.ID] = r
		}
		if b.partRels == nil {
			b.partRels = make(map[string]map[string]ooxml.Relationship)
		}
		b.partRels[part] = rels
	}
	r, ok := rels[id]
	if !ok || r.External {
		return "", false
	}
	return r.Target, true
}

// margin reads a margin of pageMargins, in inches, as points.
func margin(n roxmltree.Node, name string, def float64) float64 {
	v := number(n, name, -1)
	if v < 0 || v > 50 {
		return def
	}
	return v * 72
}

// paperLength parses a length of a paper, such as "210mm" or "8.5in", in
// points: an ST_PositiveUniversalMeasure of ECMA-376 Part 1.
func paperLength(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	for unit, scale := range map[string]float64{"mm": 72 / 25.4, "cm": 72 / 2.54, "in": 72, "pt": 1, "pc": 12, "pi": 12} {
		if v, ok := strings.CutSuffix(s, unit); ok {
			// a paper smaller than 1 inch or larger than 200 inches is
			// damaged
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil || !(f*scale >= 72) || !(f*scale <= 14400) {
				return 0, false
			}
			return f * scale, true
		}
	}
	return 0, false
}

// orientation reports whether the attribute orientation of pageSetup, v,
// turns the paper. The value "default", or no value, gives def.
func orientation(v string, def bool) bool {
	switch v {
	case "landscape":
		return true
	case "portrait":
		return false
	}
	return def
}

// readCols reads the widths of the columns, as Excel prints them in the
// digits of the default font.
func (b *book) readCols(sh *sheet.Sheet, n roxmltree.Node, digit sheet.Digit) {
	for c := range children(n, "col") {
		lo, hi := integer(c, "min", 0)-1, integer(c, "max", 0)-1
		if lo < 0 || hi < lo || lo >= sheet.MaxCols {
			continue
		}
		col := sheet.Col{Min: lo, Max: min(hi, sheet.MaxCols-1), Style: integer(c, "style", 0)}
		col.Hidden = flag(c, "hidden", false)
		if w := number(c, "width", -1); w >= 0 {
			col.Width = digit.Column(w)
			if col.Width == 0 {
				col.Hidden = true
			}
		}
		if col.Style < 0 || col.Style >= len(b.styles.cells) {
			col.Style = 0
		}
		sh.Cols = append(sh.Cols, col)
		if len(sh.Cols) >= b.lim.MaxObjects {
			return
		}
	}
}

// readSheetData reads the rows and the cells of a worksheet.
func (b *book) readSheetData(sh *sheet.Sheet, n roxmltree.Node) error {
	next := 0
	for rn := range children(n, "row") {
		if err := b.ctx.Err(); err != nil {
			return err
		}
		idx := integer(rn, "r", next+1) - 1
		if idx < 0 || idx >= sheet.MaxRows {
			continue
		}
		next = idx + 1
		row := sheet.Row{Index: idx}
		if ht := number(rn, "ht", -1); ht >= 0 && ht < 410 {
			row.Height = ht * sheet.RowScale
			if ht == 0 {
				row.Hidden = true
			}
		}
		row.Hidden = row.Hidden || flag(rn, "hidden", false)
		if flag(rn, "customFormat", false) {
			if s := integer(rn, "s", 0); s > 0 && s < len(b.styles.cells) {
				row.Style = s
			}
		}
		col := 0
		for cn := range children(rn, "c") {
			c, ok := b.readCell(cn, &col)
			if !ok {
				continue
			}
			if b.cells >= b.lim.MaxObjects {
				sh.Note = "The workbook holds more cells than the limit of ivy. The sheet stops at the limit."
				sh.Rows = append(sh.Rows, row)
				return nil
			}
			b.cells++
			row.Cells = append(row.Cells, c)
		}
		sh.Rows = append(sh.Rows, row)
	}
	return nil
}

// readCell reads a cell element. col is the column of the cell after the
// previous one, for a cell without a reference.
func (b *book) readCell(n roxmltree.Node, col *int) (sheet.Cell, bool) {
	c := sheet.Cell{Col: *col}
	if ref := attr(n, "r"); ref != "" {
		_, cc, ok := parseRef(ref)
		if !ok {
			return sheet.Cell{}, false
		}
		c.Col = cc
	}
	if c.Col >= sheet.MaxCols {
		return sheet.Cell{}, false
	}
	*col = c.Col + 1
	if s := integer(n, "s", 0); s > 0 && s < len(b.styles.cells) {
		c.Style = s
	}
	var v string
	hasV := false
	var inline richText
	hasInline := false
	for e := range n.Children() {
		switch local(e) {
		case "v":
			v, hasV = nodeText(e), true
		case "is":
			inline, hasInline = b.richText(e), true
		}
	}
	format := b.styles.format(c.Style)
	switch attr(n, "t") {
	case "s":
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if !hasV || err != nil || i < 0 || i >= len(b.strings) {
			return c, true
		}
		b.setText(&c, b.strings[i], format)
	case "inlineStr":
		if hasInline {
			b.setText(&c, inline, format)
		}
	case "str":
		b.setText(&c, richText{text: v}, format)
	case "b":
		c.Kind = sheet.KindBool
		switch strings.TrimSpace(v) {
		case "1", "true":
			c.Text = "TRUE"
		case "0", "false":
			c.Text = "FALSE"
		}
	case "e":
		c.Kind = sheet.KindError
		c.Text = v
	case "d":
		t, ok := parseDate(v)
		if !ok {
			c.Text = v
			return c, true
		}
		b.setNumber(&c, b.serial(t), format)
	default:
		if !hasV {
			return c, true
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			c.Text = v
			return c, true
		}
		b.setNumber(&c, f, format)
	}
	return c, true
}

// setText sets the text of a cell, with the text section of its format.
func (b *book) setText(c *sheet.Cell, rt richText, format *numfmt.Format) {
	c.Kind = sheet.KindText
	res := format.Text(rt.text)
	if res.Text != rt.text {
		c.Text = res.Text
	} else {
		c.Text = rt.text
		c.Runs = rt.runs
	}
	c.Color = b.styles.paletteColor(res.Color)
}

// setNumber sets the text of a cell that holds a number.
func (b *book) setNumber(c *sheet.Cell, v float64, format *numfmt.Format) {
	c.Kind = sheet.KindNumber
	res := format.Number(v, b.date1904)
	c.Text = res.Text
	c.Color = b.styles.paletteColor(res.Color)
}

// parseDate parses a date of the type d, in ISO 8601.
func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04", "2006-01-02", "15:04:05.999999999"} {
		if t, err := time.Parse(layout, s); err == nil {
			if layout == "15:04:05.999999999" {
				t = t.AddDate(1899, 11, 30)
			}
			return t, true
		}
	}
	return time.Time{}, false
}

// serial returns the serial number of Excel of the time t.
func (b *book) serial(t time.Time) float64 {
	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	if b.date1904 {
		return t.Sub(time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24
	}
	d := t.Sub(time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)).Hours() / 24
	if d < 61 {
		// the days before 1 March 1900 come before the day 29 February
		// 1900 that Excel counts
		d--
	}
	return d
}
