package msxls

import (
	"image/color"
	"math"
	"slices"
	"unicode/utf16"

	"github.com/xo/ivy/internal/sheet"
)

// This file reads the conditional formats of a worksheet: the records
// CONDFMT and CF of Excel 97, the records CONDFMT12 and CF12 of Excel 2007,
// and the records CFEX that give a rule of CF the priority and the
// template of Excel 2007 (MS-XLS 2.4.42, 2.4.56, 2.4.57, 2.4.58 and
// 2.4.59). internal/sheet evaluates the rules, as it does for xlsx. A rule
// that needs a formula, which ivy does not compute, is left out, as in xlsx.

// The records of the conditional formats.
const (
	recCondFmt   = 0x01B0
	recCF        = 0x01B1
	recCondFmt12 = 0x0879
	recCF12      = 0x087A
	recCFEx      = 0x087B
)

// The types of a rule, ct of CF and CF12.
const (
	ctCellValue = 1
	ctFormula   = 2
	ctGradient  = 3
	ctDataBar   = 4
	ctFilter    = 5
	ctIconSet   = 6
)

// The templates of a rule, CFTemp (MS-XLS 2.5.38).
const (
	tmplFilter        = 0x05
	tmplUnique        = 0x07
	tmplText          = 0x08
	tmplBlanks        = 0x09
	tmplNoBlanks      = 0x0A
	tmplErrors        = 0x0B
	tmplNoErrors      = 0x0C
	tmplAbove         = 0x19
	tmplBelow         = 0x1A
	tmplDuplicate     = 0x1B
	tmplAboveOrEqual  = 0x1D
	tmplBelowOrEqual  = 0x1E
	maxCondBlocks     = 1024
	maxRulesPerBlock  = 64
	maxRangesPerBlock = 64
)

// The tokens of a formula that an operand of a rule holds (MS-XLS
// 2.5.198).
const (
	ptgUplus  = 0x12
	ptgUminus = 0x13
	ptgStr    = 0x17
	ptgBool   = 0x1D
	ptgInt    = 0x1E
	ptgNum    = 0x1F
	ptgRef    = 0x24
)

// operators are the comparisons of a rule of a cell value, by cp.
var operators = [...]string{"", "between", "notBetween", "equal", "notEqual", "greaterThan", "lessThan", "greaterThanOrEqual", "lessThanOrEqual"}

// iconSets are the names of the icon sets, by iIconSet of CFMultistate
// (MS-XLS 2.5.156, KPISets).
var iconSets = [...]string{
	"3Arrows", "3ArrowsGray", "3Flags", "3TrafficLights1", "3TrafficLights2", "3Signs", "3Symbols", "3Symbols2",
	"4Arrows", "4ArrowsGray", "4RedToBlack", "4Rating", "4TrafficLights",
	"5Arrows", "5ArrowsGray", "5Rating", "5Quarters",
}

// themeColors are the colors of the theme of Office 2007, the theme of a
// workbook of Excel 97, by their index in SpreadsheetML, which swaps the
// dark and the light colors.
var themeColors = [...]uint32{
	0xFFFFFF, 0x000000, 0xEEECE1, 0x1F497D, 0x4F81BD, 0xC0504D,
	0x9BBB59, 0x8064A2, 0x4BACC6, 0xF79646, 0x0000FF, 0x800080,
}

// condBlock is a record CONDFMT or CONDFMT12 with its rules.
type condBlock struct {
	id     int
	ranges []sheet.Range
	rules  []condRec
}

// condRec is a rule of a record CF or CF12, with the fields of its record
// CFEX.
type condRec struct {
	ct, cp       int
	diff         *sheet.Diff
	rgce1, rgce2 []byte
	priority     int
	stop         bool
	template     int
	params       []byte
	// ext is the end of a record CF12: the color scale, the data bar, the
	// filter or the icon set.
	ext []byte
}

// condFmt reads a record CONDFMT or CONDFMT12 (MS-XLS 2.4.56 and 2.4.57).
func (sh *rawSheet) condFmt(b []byte, cf12 bool) {
	if cf12 {
		// an FrtRefHeaderU of 12 bytes
		if len(b) < 12 {
			return
		}
		b = b[12:]
	}
	if len(b) < 14 || len(sh.conds) >= maxCondBlocks {
		sh.condOpen = false
		return
	}
	blk := condBlock{id: int(le.Uint16(b[2:]) >> 1)}
	n := int(le.Uint16(b[12:]))
	for i := range min(n, maxRangesPerBlock) {
		at := 14 + 8*i
		if at+8 > len(b) {
			break
		}
		r0, r1 := int(le.Uint16(b[at:])), int(le.Uint16(b[at+2:]))
		c0, c1 := int(le.Uint16(b[at+4:])), int(le.Uint16(b[at+6:]))
		if r0 <= r1 && c0 <= c1 && r1 <= maxRow && c1 <= maxCol {
			blk.ranges = append(blk.ranges, sheet.Range{Row0: r0, Col0: c0, Row1: r1, Col1: c1})
		}
	}
	sh.conds = append(sh.conds, blk)
	sh.condOpen = true
}

// cf reads a record CF or CF12 into the last block (MS-XLS 2.4.42 and
// 2.4.43).
func (w *workbook) cf(sh *rawSheet, b []byte, cf12 bool) {
	if !sh.condOpen || len(sh.conds) == 0 {
		return
	}
	blk := &sh.conds[len(sh.conds)-1]
	if len(blk.rules) >= maxRulesPerBlock {
		return
	}
	if cf12 {
		if len(b) < 12 {
			return
		}
		b = b[12:]
	}
	if len(b) < 6 {
		return
	}
	rec := condRec{ct: int(b[0]), cp: int(b[1]), priority: math.MaxInt32}
	cce1, cce2 := int(le.Uint16(b[2:])), int(le.Uint16(b[4:]))
	at := 6
	if cf12 {
		var ok bool
		if rec.diff, at, ok = w.dxfn12(b, at); !ok {
			return
		}
	} else {
		d, n, ok := w.dxfn(b[at:])
		if !ok {
			return
		}
		rec.diff, at = d, at+n
	}
	if at+cce1+cce2 > len(b) {
		return
	}
	rec.rgce1 = b[at : at+cce1]
	rec.rgce2 = b[at+cce1 : at+cce1+cce2]
	at += cce1 + cce2
	if cf12 {
		// fmlaActive, the flags, the priority, the template and its
		// parameters
		if at+2 > len(b) {
			return
		}
		at += 2 + int(le.Uint16(b[at:]))
		if at+22 > len(b) {
			return
		}
		rec.stop = b[at]&0x02 != 0
		rec.priority = int(le.Uint16(b[at+1:]))
		rec.template = int(le.Uint16(b[at+3:]))
		rec.params = b[at+6 : at+22]
		rec.ext = b[at+22:]
	}
	blk.rules = append(blk.rules, rec)
}

// cfEx reads a record CFEX, which gives a rule of a record CF its
// priority, its template and its colors (MS-XLS 2.4.44).
func (w *workbook) cfEx(sh *rawSheet, b []byte) {
	if len(b) < 26 || le.Uint32(b[12:]) != 0 {
		// a record CFEX of a rule of CF12 holds nothing more
		return
	}
	id, icf := int(le.Uint16(b[16:])), int(le.Uint16(b[18:]))
	var rec *condRec
	for i := range sh.conds {
		if blk := &sh.conds[i]; blk.id == id && icf < len(blk.rules) {
			rec = &blk.rules[icf]
		}
	}
	if rec == nil {
		return
	}
	rec.template = int(b[21])
	rec.priority = int(le.Uint16(b[22:]))
	rec.stop = b[24]&0x02 != 0
	at := 26
	if b[25] != 0 {
		d, n, ok := w.dxfn12(b, at)
		if !ok {
			return
		}
		if d != nil {
			rec.diff = d
		}
		at = n
	}
	if at+17 <= len(b) {
		rec.params = b[at+1 : at+17]
	}
}

// dxfn12 reads a DXFN12 at the offset at of b: the size, the DXFN and its
// extension of full colors. It returns the change of the style, or nil,
// and the offset after it.
func (w *workbook) dxfn12(b []byte, at int) (*sheet.Diff, int, bool) {
	if at+4 > len(b) {
		return nil, 0, false
	}
	cb := int(le.Uint32(b[at:]))
	at += 4
	if cb == 0 {
		// 2 reserved bytes
		return nil, at + 2, at+2 <= len(b)
	}
	if cb < 0 || at+cb > len(b) {
		return nil, 0, false
	}
	body := b[at : at+cb]
	d, n, ok := w.dxfn(body)
	if !ok {
		return nil, 0, false
	}
	xfExt(d, body[n:])
	return d, at + cb, true
}

// dxfn reads a DXFN (MS-XLS 2.5.79): the changes of the number format, the
// font, the alignment, the borders, the fill and the protection. It
// returns the change of the style and the size of the DXFN.
func (w *workbook) dxfn(b []byte) (*sheet.Diff, int, bool) {
	if len(b) < 6 {
		return nil, 0, false
	}
	flags := le.Uint32(b)
	flags2 := le.Uint16(b[4:])
	at := 6
	d := &sheet.Diff{}
	if flags&(1<<25) != 0 {
		// the number format: an index, or a format of its own
		if flags2&0x0001 != 0 {
			if at+2 > len(b) {
				return nil, 0, false
			}
			at += int(le.Uint16(b[at:]))
		} else {
			at += 2
		}
	}
	if flags&(1<<26) != 0 {
		// DXFFntD (MS-XLS 2.5.80)
		if at+118 > len(b) {
			return nil, 0, false
		}
		f := b[at:]
		ts, bls := le.Uint32(f[68:]), le.Uint16(f[72:])
		uls := f[76]
		icv := le.Uint32(f[80:])
		tsNinch, ulsNinch, blsNinch := le.Uint32(f[88:]), le.Uint32(f[96:]), le.Uint32(f[100:])
		if tsNinch&0x02 == 0 {
			d.Italic = toggle(ts&0x02 != 0)
		}
		if tsNinch&0x80 == 0 {
			d.Strike = toggle(ts&0x80 != 0)
		}
		if blsNinch == 0 && bls != 0 {
			d.Bold = toggle(bls >= 700)
		}
		if ulsNinch == 0 {
			switch uls {
			case 0:
				d.Underline = -1
			case 1, 0x21:
				d.Underline = 1
			case 2, 0x22:
				d.Underline = 2
			}
		}
		if icv != 0xFFFFFFFF && icv < 0x7FFF {
			d.Color = w.color(int(icv))
		}
		at += 118
	}
	if flags&(1<<27) != 0 {
		at += 8
	}
	if flags&(1<<28) != 0 {
		// DXFBdr (MS-XLS 2.5.82): the styles and the colors of the
		// left, the right, the top and the bottom
		if at+8 > len(b) {
			return nil, 0, false
		}
		v1, v2 := le.Uint32(b[at:]), le.Uint32(b[at+4:])
		sides := [4]struct {
			ninch    uint32
			dg, icv  int
			borderAt int
		}{
			{1 << 12, int(v1 >> 8 & 0x0F), int(v2 & 0x7F), 0},
			{1 << 11, int(v1 >> 4 & 0x0F), int(v1 >> 23 & 0x7F), 1},
			{1 << 13, int(v1 >> 12 & 0x0F), int(v2 >> 7 & 0x7F), 2},
			{1 << 10, int(v1 & 0x0F), int(v1 >> 16 & 0x7F), 3},
		}
		for _, s := range sides {
			if flags&s.ninch == 0 && s.dg > 0 {
				d.Borders[s.borderAt] = w.border(s.dg, s.icv)
			}
		}
		at += 8
	}
	if flags&(1<<29) != 0 {
		// DXFPat (MS-XLS 2.5.84): a solid fill takes the color in back
		if at+4 > len(b) {
			return nil, 0, false
		}
		fls := le.Uint16(b[at:]) >> 10
		icv := le.Uint16(b[at+2:])
		fore, back := int(icv&0x7F), int(icv>>7&0x7F)
		solid := flags&(1<<16) != 0 || fls <= 1
		switch {
		case solid && flags&(1<<18) == 0 && back != 64 && back != 65:
			d.Fill = w.color(back)
		case solid && flags&(1<<17) == 0 && fore != 64 && fore != 65:
			d.Fill = w.color(fore)
		case !solid && flags&(1<<17) == 0 && fore != 64:
			d.Fill = w.color(fore)
		}
		at += 4
	}
	if flags&(1<<30) != 0 {
		at += 2
	}
	if at > len(b) {
		return nil, 0, false
	}
	return d, at, true
}

// xfExt reads the full colors of an XFExtNoFRT, which a DXFN12 ends with
// (MS-XLS 2.5.283): they replace the colors of the palette.
func xfExt(d *sheet.Diff, b []byte) {
	if d == nil || len(b) < 8 {
		return
	}
	n := int(le.Uint16(b[6:]))
	at := 8
	for range min(n, 32) {
		if at+4 > len(b) {
			return
		}
		typ, cb := le.Uint16(b[at:]), int(le.Uint16(b[at+2:]))
		if cb < 4 || at+cb > len(b) {
			return
		}
		data := b[at+4 : at+cb]
		if c, ok := fullColor(data); ok {
			switch typ {
			case 0x05:
				d.Fill = c
			case 0x04:
				if d.Fill.A == 0 {
					d.Fill = c
				}
			case 0x0D:
				d.Color = c
			}
		}
		at += cb
	}
}

// fullColor reads a FullColorExt (MS-XLS 2.5.155): an RGB color or a color
// of the theme, with a tint.
func fullColor(b []byte) (color.NRGBA, bool) {
	if len(b) < 8 {
		return color.NRGBA{}, false
	}
	tint := float64(int16(le.Uint16(b[2:]))) / 32767
	return xclr(int(le.Uint16(b)), b[4:8], tint)
}

// cfColor reads a CFColor (MS-XLS 2.5.32): the type, the value and the
// tint of a color.
func (w *workbook) cfColor(b []byte) (color.NRGBA, bool) {
	if len(b) < 16 {
		return color.NRGBA{}, false
	}
	typ := int(le.Uint32(b))
	if typ == 1 {
		return w.color(int(le.Uint32(b[4:]))), true
	}
	return xclr(typ, b[4:8], f64(b[8:]))
}

// xclr returns the color of the type typ and of the value v: 2 for RGB and
// 3 for a color of the theme.
func xclr(typ int, v []byte, tint float64) (color.NRGBA, bool) {
	var c color.NRGBA
	switch typ {
	case 2:
		c = color.NRGBA{v[0], v[1], v[2], 255}
	case 3:
		i := int(le.Uint32(v))
		if i < 0 || i >= len(themeColors) {
			return color.NRGBA{}, false
		}
		t := themeColors[i]
		c = color.NRGBA{uint8(t >> 16), uint8(t >> 8), uint8(t), 255}
	default:
		return color.NRGBA{}, false
	}
	if tint != 0 && !math.IsNaN(tint) {
		c = sheet.Tint(c, tint)
	}
	return c, true
}

// toggle returns the change of a flag of a style.
func toggle(on bool) sheet.Toggle {
	if on {
		return sheet.On
	}
	return sheet.Off
}

// condFormats converts the conditional formats of raw into sh, whose rows
// give the values of the cells that a rule names.
func (w *workbook) condFormats(sh *sheet.Sheet, raw *rawSheet) {
	order := 0
	for _, blk := range raw.conds {
		if len(blk.ranges) == 0 {
			continue
		}
		f := sheet.CondFormat{Ranges: blk.ranges}
		for _, rec := range blk.rules {
			order++
			if r, ok := w.condRule(sh, rec, order); ok {
				f.Rules = append(f.Rules, r)
			}
		}
		if len(f.Rules) > 0 {
			sh.CondFormats = append(sh.CondFormats, f)
		}
	}
}

// condRule converts a rule. It returns false for a rule that ivy cannot
// evaluate. A rule without a priority of Excel 2007 takes its order.
func (w *workbook) condRule(sh *sheet.Sheet, rec condRec, order int) (sheet.CondRule, bool) {
	r := sheet.CondRule{Priority: rec.priority, StopIfTrue: rec.stop, Diff: rec.diff}
	if rec.priority == math.MaxInt32 {
		r.Priority = order
	}
	param := func(i int) int {
		if i+2 > len(rec.params) {
			return 0
		}
		return int(int16(le.Uint16(rec.params[i:])))
	}
	switch rec.ct {
	case ctCellValue:
		if rec.cp <= 0 || rec.cp >= len(operators) || rec.diff == nil {
			return r, false
		}
		r.Kind, r.Operator = "cellIs", operators[rec.cp]
		for i, rgce := range [][]byte{rec.rgce1, rec.rgce2} {
			if i == 1 && rec.cp > 2 {
				break
			}
			o, ok := operand(sh, rgce)
			if !ok {
				return r, false
			}
			r.Operands = append(r.Operands, o)
		}
		return r, true
	case ctFormula:
		if rec.diff == nil {
			return r, false
		}
		switch rec.template {
		case tmplFilter:
			if len(rec.params) < 3 {
				return r, false
			}
			r.Kind = "top10"
			r.Bottom, r.Percent = rec.params[0]&0x01 == 0, rec.params[0]&0x02 != 0
			r.Rank = int(le.Uint16(rec.params[1:]))
		case tmplUnique:
			r.Kind = "uniqueValues"
		case tmplDuplicate:
			r.Kind = "duplicateValues"
		case tmplText:
			kinds := [...]string{"containsText", "notContainsText", "beginsWith", "endsWith"}
			k := param(0)
			text, ok := formulaText(rec.rgce1)
			if k < 0 || k >= len(kinds) || !ok {
				return r, false
			}
			r.Kind, r.Text = kinds[k], text
		case tmplBlanks:
			r.Kind = "containsBlanks"
		case tmplNoBlanks:
			r.Kind = "notContainsBlanks"
		case tmplErrors:
			r.Kind = "containsErrors"
		case tmplNoErrors:
			r.Kind = "notContainsErrors"
		case tmplAbove, tmplBelow, tmplAboveOrEqual, tmplBelowOrEqual:
			r.Kind = "aboveAverage"
			r.Below = rec.template == tmplBelow || rec.template == tmplBelowOrEqual
			r.EqualAverage = rec.template == tmplAboveOrEqual || rec.template == tmplBelowOrEqual
			r.StdDev = max(0, min(3, param(0)))
		default:
			return r, false
		}
		return r, true
	case ctFilter:
		// CFFilter: its size, a reserved byte, the flags and the rank
		if len(rec.ext) < 6 || rec.diff == nil {
			return r, false
		}
		r.Kind = "top10"
		r.Bottom, r.Percent = rec.ext[3]&0x01 == 0, rec.ext[3]&0x02 != 0
		r.Rank = int(le.Uint16(rec.ext[4:]))
		return r, true
	case ctGradient:
		return w.colorScale(r, rec.ext)
	case ctDataBar:
		return w.dataBar(r, rec.ext)
	case ctIconSet:
		return iconSet(r, rec.ext)
	}
	return r, false
}

// colorScale reads a CFGradient (MS-XLS 2.5.36).
func (w *workbook) colorScale(r sheet.CondRule, b []byte) (sheet.CondRule, bool) {
	if len(b) < 6 {
		return r, false
	}
	nInterp, nCurve := int(b[3]), int(b[4])
	if nInterp < 2 || nInterp > 3 || nCurve != nInterp {
		return r, false
	}
	at := 6
	var ts []sheet.Threshold
	for range nInterp {
		t, n, ok := cfvo(b[at:])
		if !ok || at+n+8 > len(b) {
			return r, false
		}
		ts = append(ts, t)
		at += n + 8
	}
	for i := range nCurve {
		if at+24 > len(b) {
			return r, false
		}
		c, ok := w.cfColor(b[at+8:])
		if !ok {
			return r, false
		}
		r.Scale = append(r.Scale, sheet.ScalePoint{Threshold: ts[i], Color: c})
		at += 24
	}
	r.Kind = "colorScale"
	return r, true
}

// dataBar reads a CFDatabar (MS-XLS 2.5.35).
func (w *workbook) dataBar(r sheet.CondRule, b []byte) (sheet.CondRule, bool) {
	if len(b) < 22 {
		return r, false
	}
	c, ok := w.cfColor(b[6:])
	if !ok {
		c = color.NRGBA{0x63, 0x8e, 0xc6, 255}
	}
	lo, n1, ok1 := cfvo(b[22:])
	if !ok1 {
		return r, false
	}
	hi, _, ok2 := cfvo(b[22+n1:])
	if !ok2 {
		return r, false
	}
	r.Kind = "dataBar"
	r.Bar = &sheet.DataBar{Min: lo, Max: hi, Color: c, Gradient: true, HideValue: b[3]&0x02 != 0}
	return r, true
}

// iconSet reads a CFMultistate (MS-XLS 2.5.39).
func iconSet(r sheet.CondRule, b []byte) (sheet.CondRule, bool) {
	if len(b) < 6 {
		return r, false
	}
	n, set, flags := int(b[3]), int(b[4]), b[5]
	if set >= len(iconSets) || n < 2 || n > 5 {
		return r, false
	}
	is := &sheet.IconSet{Name: iconSets[set], HideValue: flags&0x01 != 0, Reverse: flags&0x04 != 0}
	at := 6
	for range n {
		t, k, ok := cfvo(b[at:])
		if !ok || at+k+5 > len(b) {
			return r, false
		}
		t.GTE = b[at+k] != 0
		is.Thresholds = append(is.Thresholds, t)
		at += k + 5
	}
	r.Kind, r.Icons = "iconSet", is
	return r, true
}

// cfvo reads a CFVO (MS-XLS 2.5.37): the kind of a threshold, its formula
// and its value. It returns the threshold and its size. A threshold of a
// formula, which ivy does not compute, is the lowest or the highest value.
func cfvo(b []byte) (sheet.Threshold, int, bool) {
	if len(b) < 3 {
		return sheet.Threshold{}, 0, false
	}
	typ := b[0]
	cce := int(le.Uint16(b[1:]))
	n := 3 + cce
	if n > len(b) {
		return sheet.Threshold{}, 0, false
	}
	var t sheet.Threshold
	if cce == 0 && typ != 2 && typ != 3 {
		if n+8 > len(b) {
			return sheet.Threshold{}, 0, false
		}
		t.Value = f64(b[n:])
		n += 8
	}
	switch typ {
	case 1:
		t.Kind = "num"
	case 2:
		t.Kind = "min"
	case 3:
		t.Kind = "max"
	case 4:
		t.Kind = "percent"
	case 5:
		t.Kind = "percentile"
	}
	if cce > 0 {
		// a formula: a constant, or else no value of its own
		t.Kind = ""
		if typ != 7 {
			if o, ok := operand(nil, b[3:3+cce]); ok && !o.IsText {
				t.Kind, t.Value = [...]string{1: "num", 4: "percent", 5: "percentile"}[min(int(typ), 5)], o.Number
			}
		}
	}
	return t, n, true
}

// operand returns the value of a formula of one constant, such as 10, -2.5
// or "text", or of one cell of the sheet sh with a reference that does not
// move.
func operand(sh *sheet.Sheet, rgce []byte) (sheet.Operand, bool) {
	var o sheet.Operand
	neg := false
	have := false
	for i := 0; i < len(rgce); {
		ptg := rgce[i]
		base := ptg
		if ptg >= 0x20 && ptg < 0x80 {
			base = ptg&0x1F | 0x20
		}
		switch {
		case have && ptg == ptgUminus:
			neg = !neg
			i++
		case have && ptg == ptgUplus, have && ptg == ptgParen:
			i++
		case !have && ptg == ptgInt && i+3 <= len(rgce):
			o.Number, have = float64(le.Uint16(rgce[i+1:])), true
			i += 3
		case !have && ptg == ptgNum && i+9 <= len(rgce):
			o.Number, have = f64(rgce[i+1:]), true
			i += 9
		case !have && ptg == ptgBool && i+2 <= len(rgce):
			o.Number, have = float64(rgce[i+1]), true
			i += 2
		case !have && ptg == ptgStr:
			s, n, ok := ptgString(rgce[i:])
			if !ok {
				return o, false
			}
			o.Text, o.IsText, have = s, true, true
			i += n
		case !have && base == ptgRef && sh != nil && i+5 <= len(rgce):
			row, col := int(le.Uint16(rgce[i+1:])), le.Uint16(rgce[i+3:])
			if col&0xC000 != 0 {
				// a reference that moves with the cell
				return o, false
			}
			c := cellAt(sh, row, int(col&0x3FFF))
			switch {
			case c == nil:
			case c.Kind == sheet.KindNumber:
				o.Number = c.Value
			default:
				o.Text, o.IsText = c.Text, true
			}
			have = true
			i += 5
		default:
			return o, false
		}
	}
	if neg {
		if o.IsText {
			return o, false
		}
		o.Number = -o.Number
	}
	return o, have
}

// formulaText returns the first text of a formula, such as the text that a
// rule of the template "contains text" searches for.
func formulaText(rgce []byte) (string, bool) {
	for i := 0; i < len(rgce); i++ {
		if rgce[i] != ptgStr {
			continue
		}
		if s, _, ok := ptgString(rgce[i:]); ok {
			return s, true
		}
	}
	return "", false
}

// ptgString reads the token PtgStr at the start of b: its text and its
// size.
func ptgString(b []byte) (string, int, bool) {
	if len(b) < 3 {
		return "", 0, false
	}
	n := int(b[1])
	if b[2]&1 == 0 {
		if 3+n > len(b) {
			return "", 0, false
		}
		units := make([]uint16, n)
		for i := range n {
			units[i] = uint16(b[3+i])
		}
		return string(utf16.Decode(units)), 3 + n, true
	}
	if 3+2*n > len(b) {
		return "", 0, false
	}
	units := make([]uint16, n)
	for i := range n {
		units[i] = le.Uint16(b[3+2*i:])
	}
	return string(utf16.Decode(units)), 3 + 2*n, true
}

// cellAt returns the cell of sh at the row and the column, or nil.
func cellAt(sh *sheet.Sheet, row, col int) *sheet.Cell {
	i, ok := slices.BinarySearchFunc(sh.Rows, row, func(r sheet.Row, v int) int { return r.Index - v })
	if !ok {
		return nil
	}
	cells := sh.Rows[i].Cells
	k, ok := slices.BinarySearchFunc(cells, col, func(c sheet.Cell, v int) int { return c.Col - v })
	if !ok {
		return nil
	}
	return &cells[k]
}
