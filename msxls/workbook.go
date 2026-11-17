package msxls

import (
	"context"
	"fmt"
	"image/color"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/odraw"
	"github.com/xo/ivy/internal/sheet"
)

// boundSheet is a sheet of the workbook: the record BOUNDSHEET (MS-XLS
// 2.4.28).
type boundSheet struct {
	name   string
	pos    int
	hidden bool
	// kind is 0 for a worksheet, 1 for a macro sheet, 2 for a chart and 6
	// for a module of Visual Basic.
	kind byte
}

// richString is a shared string with its runs of fonts.
type richString struct {
	text string
	runs []fontRun
}

// fontRun is a run of a rich string: the font of the characters from ich
// (MS-XLS 2.5.130).
type fontRun struct {
	ich, ifnt int
}

// workbook is the globals of a workbook: the strings, the fonts, the
// formats and the sheets.
type workbook struct {
	wb       []byte
	version  uint16
	codePage uint16
	date1904 bool
	palette  [64]color.NRGBA
	fonts    []sheet.Font
	formats  map[int]string
	xfs      [][]byte
	sst      []richString
	sheets   []boundSheet
	// prints are the print areas and the print titles of the sheets, by
	// the index of the sheet from 0.
	prints map[int]*printNames
	// store is the store of the pictures of the drawings, by their index
	// from 1.
	store []odraw.Entry
}

// readWorkbook reads the globals of the workbook stream wb, and decrypts the
// stream if a password protects it.
func readWorkbook(ctx context.Context, wb []byte, cfg ivy.Config) (*workbook, error) {
	lim := cfg.Limits
	if len(wb) < 8 || le.Uint16(wb) != recBOF {
		if len(wb) >= 2 {
			switch le.Uint16(wb) {
			case 0x0009, 0x0209, 0x0409:
				return nil, fmt.Errorf("workbook of biff2 to biff4: %w", ivy.ErrUnsupported)
			}
		}
		return nil, fmt.Errorf("workbook without a bof: %w", ivy.ErrFormat)
	}
	w := &workbook{wb: wb, formats: make(map[int]string), palette: defaultPalette}
	// BIFF7 writes the version of BIFF5, and a version that is not known is
	// taken as BIFF8
	w.version = biff8
	if le.Uint16(wb[4:]) == biff5 {
		w.version = biff5
		w.codePage = 1252
	}
	count := 0
	var group []byte
	for rec := range records(wb, 0) {
		count++
		if count > lim.MaxObjects {
			return nil, fmt.Errorf("workbook globals of more than %d records: %w", lim.MaxObjects, ivy.ErrLimit)
		}
		if count%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("reading excel workbook: %w", err)
			}
		}
		r := newReader(rec)
		switch rec.typ {
		case recFilePass:
			if w.version < biff8 {
				return nil, fmt.Errorf("biff5 workbook with xor obfuscation: %w", ivy.ErrUnsupported)
			}
			if err := decrypt(ctx, wb, rec.data, cfg, rec.off+len(rec.data)); err != nil {
				return nil, err
			}
		case recCodePage:
			if cp := r.u16(); w.version < biff8 && cp != 0 {
				w.codePage = cp
			}
		case recDateMode:
			w.date1904 = r.u16() == 1
		case recPalette:
			n := int(r.u16())
			for i := 0; i < n && 8+i < len(w.palette); i++ {
				c := r.bytes(4)
				if len(c) == 4 {
					w.palette[8+i] = color.NRGBA{c[0], c[1], c[2], 255}
				}
			}
		case recFont:
			w.fonts = append(w.fonts, w.readFont(r))
		case recFormat, recFormat5:
			id := int(r.u16())
			if w.version < biff8 {
				w.formats[id] = r.shortString(w.version, w.codePage)
			} else {
				w.formats[id] = r.longString(w.version, w.codePage)
			}
		case recXF:
			w.xfs = append(w.xfs, rec.data)
		case recBoundSheet:
			bs := boundSheet{pos: int(r.u32())}
			state := r.u8()
			bs.hidden = state&0x03 != 0
			bs.kind = r.u8()
			bs.name = strings.TrimSpace(r.shortString(w.version, w.codePage))
			w.sheets = append(w.sheets, bs)
		case recSST:
			if err := w.readSST(ctx, r, lim); err != nil {
				return nil, err
			}
		case recName:
			w.readName(rec.data)
		case recMsoDrawingGroup:
			// a large group goes on in the records MSODRAWINGGROUP after it
			if len(group) < maxDrawing {
				group = append(group, joined(rec)...)
			}
		case recEOF:
			w.store = readStore(group, lim)
			return w, nil
		}
	}
	w.store = readStore(group, lim)
	return w, nil
}

// readStore reads the store of pictures of the OfficeArtDggContainer of
// the records MSODRAWINGGROUP.
func readStore(group []byte, lim ivy.Limits) []odraw.Entry {
	h, ok := odraw.ReadHeader(group)
	if !ok || h.Type != odraw.RecDggContainer {
		return nil
	}
	return odraw.ReadStore(group[8:8+int(min(uint64(h.Size), uint64(len(group)-8)))], lim.MaxObjects)
}

// readFont reads a record FONT (MS-XLS 2.4.122).
func (w *workbook) readFont(r *reader) sheet.Font {
	f := sheet.Font{Size: float64(r.u16()) / 20}
	grbit := r.u16()
	f.Italic = grbit&0x02 != 0
	f.Strike = grbit&0x08 != 0
	if c := w.color(int(r.u16())); c != (color.NRGBA{0, 0, 0, 255}) {
		f.Color = c
	}
	f.Bold = r.u16() >= 700
	switch r.u16() {
	case 1:
		f.VertAlign = 1
	case 2:
		f.VertAlign = 2
	}
	switch r.u8() {
	case 0x01, 0x21:
		f.Underline = 1
	case 0x02, 0x22:
		f.Underline = 2
	}
	family := r.u8()
	r.skip(2) // the character set and a reserved byte
	f.Family = r.shortString(w.version, w.codePage)
	if r.bad {
		f.Family = ""
	}
	switch family {
	case 1:
		f.Generic = "serif"
	case 3:
		f.Generic = "monospace"
	}
	if f.Size <= 0 || f.Size > 409 {
		f.Size = 0
	}
	return f
}

// readSST reads the table of shared strings (MS-XLS 2.4.265).
func (w *workbook) readSST(ctx context.Context, r *reader, lim ivy.Limits) error {
	r.u32() // the count of references
	n := int(r.u32())
	if n > lim.MaxObjects {
		return fmt.Errorf("%d shared strings: %w", n, ivy.ErrLimit)
	}
	w.sst = make([]richString, 0, min(n, 1<<16))
	for i := range n {
		if r.bad {
			break
		}
		if i%4096 == 4095 {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("reading the shared strings: %w", err)
			}
		}
		w.sst = append(w.sst, r.richString())
	}
	return nil
}

// font returns the font of the index i of an XF. The index 4 is not used,
// so the index 5 is the fifth font (MS-XLS 2.5.129).
func (w *workbook) font(i int) sheet.Font {
	if i >= 4 {
		i--
	}
	if i < 0 || i >= len(w.fonts) {
		if len(w.fonts) > 0 {
			return w.fonts[0]
		}
		return sheet.Font{Family: "Arial", Size: 10}
	}
	return w.fonts[i]
}

// numberFormat returns the code of the number format of the index i.
func (w *workbook) numberFormat(i int) string {
	if s, ok := w.formats[i]; ok {
		return s
	}
	if s, ok := builtinFormat(i); ok {
		return s
	}
	return "General"
}

// The kinds of a cell.
const (
	cellNumber = iota + 1
	cellText
	cellBool
	cellError
)

// cell is a cell of a sheet with its value.
type cell struct {
	row, col int
	kind     int
	num      float64
	text     string
	runs     []fontRun
	xf       int
}

// rowInfo is the record ROW of a row (MS-XLS 2.4.221).
type rowInfo struct {
	height float64
	hidden bool
	xf     int
}

// colInfo is a record COLINFO (MS-XLS 2.4.53).
type colInfo struct {
	first, last int
	width       float64 // characters
	hidden      bool
	xf          int
}

// rawSheet is a worksheet as the records give it.
type rawSheet struct {
	name      string
	cells     []cell
	rows      map[int]rowInfo
	cols      []colInfo
	defWidth  float64 // characters, from DEFCOLWIDTH
	stdWidth  float64 // characters, from STANDARDWIDTH
	defHeight float64 // points
	merges    []sheet.Range
	page      sheet.Page
	setup     []byte
	fit       bool
	gridlines bool
	header    string
	footer    string
	// hfFlags are the flags of the record HeaderFooter of the sheet, when
	// hasHF is set.
	hfFlags   uint16
	hasHF     bool
	rowBreaks []int
	colBreaks []int
	truncated bool
	// drawing is the stream of Office Drawing of the records MSODRAWING,
	// and objs are the records OBJ.
	drawing []byte
	objs    []drawObj
}

// The names of the errors of a cell (MS-XLS 2.5.97).
var errorNames = map[byte]string{
	0x00: "#NULL!", 0x07: "#DIV/0!", 0x0F: "#VALUE!", 0x17: "#REF!",
	0x1D: "#NAME?", 0x24: "#NUM!", 0x2A: "#N/A", 0x2B: "#GETTING_DATA",
}

// The largest row and column of BIFF8.
const (
	maxRow = 65535
	maxCol = 255
)

// readSheet reads the cells and the setup of the worksheet bs.
func (w *workbook) readSheet(ctx context.Context, bs boundSheet, lim ivy.Limits) (*rawSheet, error) {
	sh := &rawSheet{
		name:      bs.name,
		rows:      make(map[int]rowInfo),
		defWidth:  8,
		defHeight: 0,
		page:      sheet.DefaultPage,
	}
	if bs.pos < 0 || bs.pos+4 > len(w.wb) || le.Uint16(w.wb[bs.pos:]) != recBOF {
		return sh, nil
	}
	add := func(c cell) {
		if c.row < 0 || c.row > maxRow || c.col < 0 || c.col > maxCol {
			return
		}
		if len(sh.cells) >= lim.MaxObjects {
			sh.truncated = true
			return
		}
		sh.cells = append(sh.cells, c)
	}
	depth := 0
	pendingString := -1
	count := 0
	for rec := range records(w.wb, bs.pos) {
		count++
		if count%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("reading sheet %q: %w", bs.name, err)
			}
		}
		if count > 4*lim.MaxObjects {
			sh.truncated = true
			return sh, nil
		}
		switch rec.typ {
		case recBOF:
			depth++
			continue
		case recEOF:
			depth--
			if depth <= 0 {
				return sh, nil
			}
			continue
		}
		if depth != 1 {
			// a chart inside the sheet
			continue
		}
		w.sheetRecord(sh, rec, add, &pendingString)
	}
	return sh, nil
}

// sheetRecord reads one record of a worksheet into sh.
func (w *workbook) sheetRecord(sh *rawSheet, rec record, add func(cell), pendingString *int) {
	r := newReader(rec)
	switch rec.typ {
	case recNumber:
		row, col, ixfe := int(r.u16()), int(r.u16()), int(r.u16())
		if v := r.bytes(8); len(v) == 8 {
			add(cell{row: row, col: col, xf: ixfe, kind: cellNumber, num: f64(v)})
		}
	case recRK:
		row, col, ixfe := int(r.u16()), int(r.u16()), int(r.u16())
		add(cell{row: row, col: col, xf: ixfe, kind: cellNumber, num: rk(r.u32())})
	case recMulRK:
		row, col := int(r.u16()), int(r.u16())
		n := (len(rec.data) - 6) / 6
		for i := range max(n, 0) {
			ixfe := int(r.u16())
			add(cell{row: row, col: col + i, xf: ixfe, kind: cellNumber, num: rk(r.u32())})
		}
	case recLabelSST:
		row, col, ixfe := int(r.u16()), int(r.u16()), int(r.u16())
		if i := int(r.u32()); i < len(w.sst) {
			add(cell{row: row, col: col, xf: ixfe, kind: cellText, text: w.sst[i].text, runs: w.sst[i].runs})
		}
	case recLabel, recRString:
		row, col, ixfe := int(r.u16()), int(r.u16()), int(r.u16())
		add(cell{row: row, col: col, xf: ixfe, kind: cellText, text: r.longString(w.version, w.codePage)})
	case recBoolErr:
		row, col, ixfe := int(r.u16()), int(r.u16()), int(r.u16())
		v, isErr := r.u8(), r.u8()
		c := cell{row: row, col: col, xf: ixfe, kind: cellBool, num: float64(v)}
		if isErr != 0 {
			c.kind, c.text = cellError, errorName(v)
		}
		add(c)
	case recBlank:
		row, col, ixfe := int(r.u16()), int(r.u16()), int(r.u16())
		add(cell{row: row, col: col, xf: ixfe})
	case recMulBlank:
		row, col := int(r.u16()), int(r.u16())
		n := (len(rec.data) - 6) / 2
		for i := range max(n, 0) {
			add(cell{row: row, col: col + i, xf: int(r.u16())})
		}
	case recFormula:
		*pendingString = -1
		row, col, ixfe := int(r.u16()), int(r.u16()), int(r.u16())
		v := r.bytes(8)
		if len(v) < 8 {
			return
		}
		c := cell{row: row, col: col, xf: ixfe}
		if v[6] == 0xFF && v[7] == 0xFF {
			switch v[0] {
			case 0:
				// the text comes in the record STRING after it
				c.kind = cellText
				*pendingString = len(sh.cells)
			case 1:
				c.kind, c.num = cellBool, float64(v[2])
			case 2:
				c.kind, c.text = cellError, errorName(v[2])
			case 3:
				c.kind = cellText
			}
		} else {
			c.kind, c.num = cellNumber, f64(v)
		}
		add(c)
	case recString:
		if i := *pendingString; i >= 0 && i < len(sh.cells) {
			sh.cells[i].text = r.longString(w.version, w.codePage)
		}
		*pendingString = -1
	case recColInfo:
		ci := colInfo{first: int(r.u16()), last: int(r.u16()), width: float64(r.u16()) / 256}
		ci.xf = int(r.u16())
		ci.hidden = r.u16()&0x0001 != 0
		if !r.bad && ci.first <= ci.last && ci.first <= maxCol {
			ci.last = min(ci.last, maxCol)
			sh.cols = append(sh.cols, ci)
		}
	case recDefColWidth:
		if v := float64(r.u16()); v > 0 && v < 256 {
			sh.defWidth = v
		}
	case recStandardWidth:
		if v := float64(r.u16()) / 256; v > 0 && v < 256 {
			sh.stdWidth = v
		}
	case recDefaultRowHeight:
		r.u16()
		if v := float64(int16(r.u16())) / 20; v > 0 && v < 410 {
			sh.defHeight = v
		}
	case recRow:
		row := int(r.u16())
		r.skip(4)
		height := float64(r.u16()&0x7FFF) / 20
		r.skip(4)
		flags := r.u16()
		xf := int(r.u16() & 0x0FFF)
		if r.bad || row > maxRow {
			return
		}
		ri := rowInfo{hidden: flags&0x0020 != 0}
		// miyRw is the height of the row that Excel shows, whether a person
		// set it (fUnsynced) or Excel fitted it to the cells
		if height > 0 && height < 410 {
			ri.height = height
		}
		if flags&0x0080 != 0 {
			ri.xf = xf
		}
		sh.rows[row] = ri
	case recMergeCells:
		n := int(r.u16())
		for range n {
			r1, r2, c1, c2 := int(r.u16()), int(r.u16()), int(r.u16()), int(r.u16())
			if r.bad {
				break
			}
			if r2 >= r1 && c2 >= c1 && r2 <= maxRow && c2 <= maxCol && (r1 != r2 || c1 != c2) {
				sh.merges = append(sh.merges, sheet.Range{Row0: r1, Col0: c1, Row1: r2, Col1: c2})
			}
		}
	case recSetup:
		sh.setup = rec.data
	case recMsoDrawing:
		if len(sh.drawing) < maxDrawing {
			sh.drawing = append(sh.drawing, joined(rec)...)
		}
	case recObj:
		// the structure FtCmo comes first: ft 0x15, cb, the type, the id
		// and the flags
		if ft := r.u16(); ft == 0x15 && len(sh.objs) < 4096 {
			r.skip(2)
			o := drawObj{at: len(sh.drawing), ot: int(r.u16())}
			r.skip(2)
			o.flags = r.u16()
			sh.objs = append(sh.objs, o)
		}
		// Excel can write the drawing of the next shape in CONTINUE
		// records after the record OBJ
		sh.addDrawing(rec.conts)
	case recTxo:
		// the text of the last object and its runs, in the CONTINUE records
		// after it, and then the drawing of the next shape
		if len(rec.data) < 14 {
			sh.addDrawing(rec.conts)
			return
		}
		n, runs := int(le.Uint16(rec.data[10:])), int(le.Uint16(rec.data[12:]))
		if n == 0 || len(rec.conts) == 0 {
			sh.addDrawing(rec.conts)
			return
		}
		grbit := le.Uint16(rec.data)
		r.skip(len(rec.data))
		high := r.u8()&1 != 0
		text := r.chars(min(n, 32767), high, w.codePage)
		// the runs of the fonts: the first character, the font and 4
		// bytes each, and a last run at the end of the text
		var fr []fontRun
		for range runs / 8 {
			ich, ifnt := int(r.u16()), int(r.u16())
			r.skip(4)
			if r.bad {
				break
			}
			if ich < n && len(fr) < 1024 {
				fr = append(fr, fontRun{ich: ich, ifnt: ifnt})
			}
		}
		r.skip(runs % 8)
		if len(sh.objs) > 0 {
			o := &sh.objs[len(sh.objs)-1]
			o.text, o.runs = text, fr
			o.hAlign, o.vAlign = int(grbit>>1&7), int(grbit>>4&7)
		}
		if r.bad {
			return
		}
		r.avail()
		sh.addDrawing(r.segs[min(r.seg, len(r.segs)):])
	case recWSBool:
		sh.fit = r.u16()&0x0100 != 0
	case recPrintGridlines:
		sh.gridlines = r.u16() != 0
	case recHCenter:
		sh.page.CenterH = r.u16() != 0
	case recLeftMargin, recRightMargin, recTopMargin, recBottomMargin:
		v := r.bytes(8)
		if len(v) < 8 {
			return
		}
		m := f64(v) * 72
		if !(m >= 0 && m < 1000) {
			return
		}
		switch rec.typ {
		case recLeftMargin:
			sh.page.Left = m
		case recRightMargin:
			sh.page.Right = m
		case recTopMargin:
			sh.page.Top = m
		case recBottomMargin:
			sh.page.Bottom = m
		}
	case recHeader, recFooter:
		if len(rec.data) == 0 {
			return
		}
		var s string
		if w.version < biff8 {
			s = r.shortString(w.version, w.codePage)
		} else {
			s = r.longString(w.version, w.codePage)
		}
		if rec.typ == recHeader {
			sh.header = s
		} else {
			sh.footer = s
		}
	case recHeaderFooter:
		// an FrtHeader of 12 bytes, the GUID of the custom view, which is
		// zero for the sheet itself, and the flags (MS-XLS 2.4.137)
		if len(rec.data) >= 30 && !sh.hasHF && allZero(rec.data[12:28]) {
			sh.hfFlags, sh.hasHF = le.Uint16(rec.data[28:]), true
		}
	case recHorizontalPageBreaks, recVerticalPageBreaks:
		n := int(r.u16())
		size := 6
		if w.version < biff8 {
			size = 2
		}
		for range n {
			v := int(r.u16())
			r.skip(size - 2)
			if r.bad {
				break
			}
			if rec.typ == recHorizontalPageBreaks {
				sh.rowBreaks = append(sh.rowBreaks, v)
			} else {
				sh.colBreaks = append(sh.colBreaks, v)
			}
		}
	}
}

// allZero reports whether each byte of b is zero.
func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// hfScaleWithDoc is the flag fHFScaleWithDoc of the record HeaderFooter:
// the header and the footer scale with the cells.
const hfScaleWithDoc = 0x0004

// scaleHeaderFooter reports whether the header and the footer of the sheet
// scale with the cells. Excel scales them unless the record HeaderFooter
// clears fHFScaleWithDoc.
func (sh *rawSheet) scaleHeaderFooter() bool {
	return !sh.hasHF || sh.hfFlags&hfScaleWithDoc != 0
}

// addDrawing adds the segments segs to the stream of the drawing.
func (sh *rawSheet) addDrawing(segs [][]byte) {
	for _, b := range segs {
		if len(sh.drawing) >= maxDrawing {
			return
		}
		sh.drawing = append(sh.drawing, b...)
	}
}

// joined returns the data of rec and of its CONTINUE records.
func joined(rec record) []byte {
	if len(rec.conts) == 0 {
		return rec.data
	}
	out := append([]byte(nil), rec.data...)
	for _, c := range rec.conts {
		if len(out) >= maxDrawing {
			break
		}
		out = append(out, c...)
	}
	return out
}

// errorName returns the text of the error v of a cell.
func errorName(v byte) string {
	if s, ok := errorNames[v]; ok {
		return s
	}
	return "#ERR!"
}
