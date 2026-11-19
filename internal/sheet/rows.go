package sheet

import (
	"encoding/binary"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage/svg"
	"github.com/xo/okage/typesetting/font/opentype"

	"github.com/xo/ivy/internal/fontdb"
)

// ExcelRows gives the heights of the rows that Excel prints, for the
// default font of a workbook. Excel keeps the height of a row in pixels of
// the screen, and prints a row of h pixels as trunc(h × P / S) dots of 600
// dots per inch (Sheet.RowDots). A picture across the rows takes h × P / S
// dots for each row, without the rounding. S is the default height of a
// row on the screen, in pixels, and P is the default height of a row on the
// printer, in dots, both for the default font. P is tmHeight +
// tmExternalLeading + 8 of the font at round(size × 600 / 72) pixels per
// em, as GDI gives them.
//
// Excel 2024 prints the default height of Calibri of 11 points, 20 pixels,
// as 121 dots, 14.52 points, so a row of 15 points prints at 0.968 of its
// height. With Century Gothic of 9 points, S is 19 and P is 99, and the
// scale is 0.834.
//
// Excel also fits a row to its cells, unless a person sets its height. The
// height of a line of text on the screen is S of its font, so a row of
// two lines of the default font prints as 2 × P dots.
type ExcelRows struct {
	// print and screen are P and S of the default font.
	print, screen int
	def           Font
	db            *svg.FontDatabase
	cache         map[rowFont]rowMetrics
}

// rowFont is a font of the cache of ExcelRows: its family in lower case,
// its size, and its style.
type rowFont struct {
	family       string
	size         float64
	bold, italic bool
}

// rowMetrics are P and S of a font.
type rowMetrics struct {
	print, screen int
}

// measuredRows holds P and S of the default fonts of the files
// xlsx/testdata/row-heights-*.xlsx, which Excel 2024 printed to PDF on
// Windows on 2026-11-19. The formula of ExcelRows gives P exactly for 10
// of these fonts from the fonts of Windows. It gives 2 dots more for
// Century Gothic and 2 dots less for MS PGothic. The fonts that stand in
// for the fonts of Windows, such as Liberation Mono for Courier New, give
// P up to 6 dots away. S has no formula that fits all the fonts.
var measuredRows = map[rowFont]rowMetrics{
	{family: "arial", size: 10}:           {103, 17},
	{family: "arial", size: 12}:           {123, 20},
	{family: "calibri", size: 8}:          {90, 15},
	{family: "calibri", size: 11}:         {121, 20},
	{family: "calibri", size: 16}:         {171, 28},
	{family: "century gothic", size: 9}:   {99, 19},
	{family: "century gothic", size: 11}:  {119, 22},
	{family: "courier new", size: 10}:     {96, 18},
	{family: "courier new", size: 12}:     {115, 21},
	{family: "ms pgothic", size: 11}:      {102, 18},
	{family: "ｍｓ ｐゴシック", size: 11}:        {102, 18},
	{family: "times new roman", size: 12}: {126, 21},
	{family: "verdana", size: 10}:         {108, 17},
}

// A row that Excel fits keeps the height of its file when that height is
// at most fitSlack pixels or fitSlackRatio of the height away from the
// height that Fit estimates. The fonts of the program can be higher or
// lower than the fonts of Windows: Times New Roman of 20 points takes 34
// or 35 pixels in Excel, and the estimate is 33 pixels.
const (
	fitSlack      = 2
	fitSlackRatio = 0.1
)

// maxRowPixels is the height of the highest row of Excel on the screen, in
// pixels: 409 points.
const maxRowPixels = 545

// NewExcelRows returns the heights of the rows of a workbook whose default
// font is def. A size of 0 is 11 points. It measures a font that measuredRows
// does not hold with the face of db that the layout draws it with, or
// estimates it when db is nil.
func NewExcelRows(db *svg.FontDatabase, def Font) *ExcelRows {
	if !(def.Size > 0) || def.Size > 409 {
		def.Size = 11
	}
	e := &ExcelRows{def: def, db: db, cache: make(map[rowFont]rowMetrics)}
	m := e.metrics(def)
	e.print, e.screen = m.print, m.screen
	return e
}

// Printed returns the height that Excel prints for a row of ht points on
// the screen, in points, before Sheet.RowDots rounds it down to a dot.
func (e *ExcelRows) Printed(ht float64) float64 {
	if !(ht > 0) {
		return 0
	}
	return e.printed(min(ht, 409) * screenDPI / 72)
}

// printed returns the height that Excel prints for a row of px pixels on
// the screen, in points.
func (e *ExcelRows) printed(px float64) float64 {
	return px * float64(e.print) / float64(e.screen) * 72 / printDPI
}

// Default returns the height that Excel prints for a row of the default
// height, in points: P.
func (e *ExcelRows) Default() float64 {
	return float64(e.print) * 72 / printDPI
}

// Scale returns the ratio of the height that Excel prints to the height on
// the screen, for the lines of text in a row (Sheet.RowScale).
func (e *ExcelRows) Scale() float64 {
	return float64(e.print) * screenDPI / (float64(e.screen) * printDPI)
}

// Fit sets the height of each row of sh whose Fit is set to the height of
// its cells, as Excel fits it when it opens the file: the highest cell, in
// lines of the screen height of its font. An empty cell takes one line. A
// row of a small font is lower than the default height. A row without cells
// keeps its height, and so does a row whose height is close to the estimate
// (fitSlack). Fit leaves out a merged cell of more than one row, and takes
// one line of a merged cell of one row. styles are the styles of the
// workbook.
func (e *ExcelRows) Fit(sh *Sheet, styles []Style) {
	// merged holds the first cell of each merged cell, which holds its
	// text, and whether the merged cell spans more than one row
	var merged map[[2]int]bool
	for _, m := range sh.Merges {
		if merged == nil {
			merged = make(map[[2]int]bool)
		}
		merged[[2]int{m.Row0, m.Col0}] = m.Row1 > m.Row0
	}
	for i := range sh.Rows {
		r := &sh.Rows[i]
		if !r.Fit {
			continue
		}
		r.Fit = false
		if r.Hidden {
			continue
		}
		px := 0.0
		for k := range r.Cells {
			c := &r.Cells[k]
			tall, isMerged := merged[[2]int{r.Index, c.Col}]
			if tall {
				continue
			}
			st := &DefaultStyle
			if c.Style >= 0 && c.Style < len(styles) {
				st = &styles[c.Style]
			}
			if isMerged && wraps(st) {
				// Excel does not wrap the text of a merged cell to fit
				// a row
				one := *st
				one.Wrap, one.HAlign = false, ""
				st = &one
			}
			px = max(px, e.cellPixels(sh, c, st))
		}
		if px == 0 {
			// a row without cells keeps its height
			continue
		}
		// a height of the file close to the estimate is the height that
		// Excel fitted with the fonts of Windows when it saved the file
		if r.Height > 0 {
			if file := r.Height / e.printed(1); math.Abs(file-px) <= max(fitSlack, px*fitSlackRatio) {
				continue
			}
		}
		r.Height = e.printed(min(px, maxRowPixels))
	}
}

// cellPixels returns the height of the text of a cell on the screen, in
// pixels.
func (e *ExcelRows) cellPixels(sh *Sheet, c *Cell, st *Style) float64 {
	f := st.Font
	if !(f.Size > 0) {
		f.Size = e.def.Size
	}
	text := c.Text
	if len(c.Runs) > 0 {
		var b strings.Builder
		for _, r := range c.Runs {
			b.WriteString(r.Text)
			if r.Font != nil && r.Font.Size > f.Size {
				f.Family, f.Size = r.Font.Family, r.Font.Size
				if f.Family == "" {
					f.Family = st.Font.Family
				}
			}
		}
		text = b.String()
	}
	f.Size = min(f.Size, 409)
	line := float64(e.metrics(f).screen)
	switch {
	case st.Rotation == 255:
		// the letters stand one under the other
		return line * float64(min(utf8.RuneCountInString(text), maxRowPixels))
	case st.Rotation != 0:
		sin, cos := math.Sincos(float64(st.Rotation) * math.Pi / 180)
		w := TextWidth(longestLine(text), f.Size) * screenDPI / 72
		return math.Abs(sin)*w + math.Abs(cos)*line
	case wraps(st):
		width := sh.colWidth(c.Col) - 2*cellPadding - st.Indent
		return line * float64(wrappedLines(text, f.Size, width))
	}
	return line
}

// longestLine returns the longest line of the text s.
func longestLine(s string) string {
	best := ""
	for l := range strings.SplitSeq(s, "\n") {
		if len(l) > len(best) {
			best = l
		}
	}
	return best
}

// wrappedLines returns the number of lines of the text s that wraps at the
// spaces in a width of width points, in Calibri of the size size
// (TextWidth). A word that is wider than the width breaks between its
// characters.
func wrappedLines(s string, size, width float64) int {
	n := 0
	space := TextWidth(" ", size)
	for para := range strings.SplitSeq(s, "\n") {
		n++
		if width <= 0 {
			continue
		}
		x := 0.0
		for i, word := range strings.Split(para, " ") {
			w := TextWidth(word, size)
			if i > 0 {
				if x+space+w <= width {
					x += space + w
					continue
				}
				n++
			}
			if w > width {
				// a word wider than the line breaks into lines of whole
				// characters
				n += int(min(math.Floor(w/width), maxRowPixels))
				w = math.Mod(w, width)
			}
			x = w
		}
		if n >= maxRowPixels {
			break
		}
	}
	return min(n, maxRowPixels)
}

// metrics returns P and S of the font f: the measures of measuredRows, or
// else P of the face of the font and S of P.
func (e *ExcelRows) metrics(f Font) rowMetrics {
	key := rowFont{family: strings.ToLower(strings.TrimSpace(f.Family)), size: f.Size}
	if m, ok := measuredRows[key]; ok {
		return m
	}
	key.bold, key.italic = f.Bold, f.Italic
	if m, ok := e.cache[key]; ok {
		return m
	}
	ppem := math.Round(f.Size * printDPI / 72)
	p, ok := printHeight(e.db, f, ppem)
	if !ok {
		// a line of 1.22 times the size of the font, as Excel lays out
		// Calibri
		p = int(math.Round(ppem*1.22)) + 8
	}
	// S is about P at the resolution of the screen, rounded up: exact for
	// Arial, Calibri and Times New Roman of the sizes of measuredRows
	m := rowMetrics{print: max(p, 1), screen: max(1, int(math.Ceil(float64(p)*screenDPI/printDPI-1e-9)))}
	e.cache[key] = m
	return m
}

// printHeight returns tmHeight + tmExternalLeading + 8 of the face of db
// that draws the font f, at ppem pixels per em, as GDI gives them. tmHeight
// comes from the table VDMX, or from usWinAscent and usWinDescent of OS/2.
// tmExternalLeading is the line gap of hhea, less the part of the height
// of OS/2 that is more than the height of hhea.
func printHeight(db *svg.FontDatabase, f Font, ppem float64) (int, bool) {
	if db == nil {
		return 0, false
	}
	weight := uint16(400)
	if f.Bold {
		weight = 700
	}
	id, ok := fontdb.Find(db, f.Family, weight, f.Italic)
	if !ok {
		kind := svg.FontFamilySansSerif
		switch f.Generic {
		case "serif":
			kind = svg.FontFamilySerif
		case "monospace":
			kind = svg.FontFamilyMonospace
		}
		id, ok = db.Query(svg.FontQuery{
			Families: []svg.FontFamily{{Kind: kind}},
			Weight:   weight,
			Stretch:  svg.DefaultFontStretch(),
		})
		if !ok {
			return 0, false
		}
	}
	data, index, ok := db.FaceData(id)
	if !ok {
		return 0, false
	}
	loaders, err := opentype.NewLoaders(opentype.NewSlicedResource(data))
	if err != nil || int(index) >= len(loaders) {
		return 0, false
	}
	ld := loaders[index]
	head, err1 := ld.RawTable(opentype.MustNewTag("head"))
	os2, err2 := ld.RawTable(opentype.MustNewTag("OS/2"))
	hhea, err3 := ld.RawTable(opentype.MustNewTag("hhea"))
	if err1 != nil || err2 != nil || err3 != nil || len(head) < 20 || len(os2) < 78 || len(hhea) < 10 {
		return 0, false
	}
	upm := float64(binary.BigEndian.Uint16(head[18:]))
	if upm < 16 || upm > 16384 {
		return 0, false
	}
	winAscent := float64(binary.BigEndian.Uint16(os2[74:]))
	winDescent := float64(binary.BigEndian.Uint16(os2[76:]))
	ascent := float64(int16(binary.BigEndian.Uint16(hhea[4:])))
	descent := float64(int16(binary.BigEndian.Uint16(hhea[6:])))
	gap := float64(int16(binary.BigEndian.Uint16(hhea[8:])))
	s := ppem / upm
	height := int(math.Round(winAscent*s) + math.Round(winDescent*s))
	vdmx, err := ld.RawTable(opentype.MustNewTag("VDMX"))
	if err == nil {
		if h, ok := vdmxHeight(vdmx, int(ppem)); ok {
			height = h
		}
	}
	leading := int(math.Round(max(0, gap-((winAscent+winDescent)-(ascent-descent))) * s))
	return height + leading + 8, true
}

// vdmxHeight returns yMax - yMin of the table VDMX for the ratio 1:1 at
// ppem pixels per em, if the table gives it.
func vdmxHeight(b []byte, ppem int) (int, bool) {
	u16 := func(o int) (int, bool) {
		if o < 0 || o+2 > len(b) {
			return 0, false
		}
		return int(binary.BigEndian.Uint16(b[o:])), true
	}
	n, ok := u16(4)
	if !ok {
		return 0, false
	}
	for i := range n {
		r := 6 + 4*i
		if r+4 > len(b) {
			return 0, false
		}
		x, y0, y1 := b[r+1], b[r+2], b[r+3]
		// the ratio 0:0 matches any ratio
		if x != 0 && (x != 1 || y0 > 1 || y1 < 1) || x == 0 && (y0 != 0 || y1 != 0) {
			continue
		}
		off, ok := u16(6 + 4*n + 2*i)
		if !ok {
			return 0, false
		}
		recs, ok := u16(off)
		if !ok {
			return 0, false
		}
		for k := range recs {
			e := off + 4 + 6*k
			h, ok1 := u16(e)
			yMax, ok2 := u16(e + 2)
			yMin, ok3 := u16(e + 4)
			if !ok1 || !ok2 || !ok3 {
				return 0, false
			}
			if h == ppem {
				return int(int16(yMax)) - int(int16(yMin)), true
			}
		}
		return 0, false
	}
	return 0, false
}
