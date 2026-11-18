package xlsx

import (
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/sheet"
)

// This file reads the conditional formats of a worksheet
// (conditionalFormatting, ECMA-376 Part 1, 18.3.1.18) and the differential
// formats that they apply (dxf, 18.8.14). internal/sheet evaluates them.

// maxCondRules bounds the rules of the conditional formats of a sheet.
const maxCondRules = 1024

// dxf reads a differential format: the changes of the font, the fill and
// the borders.
func (s *styles) dxf(n roxmltree.Node) sheet.Diff {
	var d sheet.Diff
	toggle := func(c roxmltree.Node) sheet.Toggle {
		if flag(c, "val", true) {
			return sheet.On
		}
		return sheet.Off
	}
	if f, ok := child(n, "font"); ok {
		for c := range f.Children() {
			switch local(c) {
			case "b":
				d.Bold = toggle(c)
			case "i":
				d.Italic = toggle(c)
			case "strike":
				d.Strike = toggle(c)
			case "u":
				switch attr(c, "val") {
				case "", "single", "singleAccounting":
					d.Underline = 1
				case "double", "doubleAccounting":
					d.Underline = 2
				case "none":
					d.Underline = -1
				}
			case "color":
				if v, ok := s.color(c); ok {
					d.Color = v
				}
			}
		}
	}
	if f, ok := child(n, "fill"); ok {
		// the solid fill of a dxf is its background color, and a pattern
		// without a type is solid
		if p, ok := child(f, "patternFill"); ok && (attr(p, "patternType") == "" || attr(p, "patternType") == "solid") {
			if c, ok := child(p, "bgColor"); ok {
				d.Fill, _ = s.color(c)
			} else if c, ok := child(p, "fgColor"); ok {
				d.Fill, _ = s.color(c)
			}
		} else {
			d.Fill = s.fill(f)
		}
	}
	if b, ok := child(n, "border"); ok {
		d.Borders = s.border(b)
	}
	return d
}

// readCondFormat reads a conditionalFormatting element into sh, and the ids
// of the extensions of its data bars into bars. The rules
// of the type expression and timePeriod need formulas, which ivy does not
// compute, and are left out.
func (b *book) readCondFormat(sh *sheet.Sheet, index int, n roxmltree.Node, rules *int, bars map[string]*sheet.DataBar) {
	var f sheet.CondFormat
	for _, ref := range strings.Fields(attr(n, "sqref")) {
		if r, ok := parseRange(ref); ok && len(f.Ranges) < 64 {
			f.Ranges = append(f.Ranges, r)
		}
	}
	if len(f.Ranges) == 0 {
		return
	}
	for rn := range children(n, "cfRule") {
		if *rules >= maxCondRules {
			break
		}
		if r, ok := b.condRule(sh, index, rn); ok {
			f.Rules = append(f.Rules, r)
			*rules++
			if r.Bar != nil && len(bars) < maxCondRules {
				// the extension of Excel 2010 of the rule
				for e := range rn.Descendants() {
					if local(e) == "id" {
						bars[nodeText(e)] = r.Bar
						break
					}
				}
			}
		}
	}
	if len(f.Rules) > 0 {
		sh.CondFormats = append(sh.CondFormats, f)
	}
}

// condRule reads a cfRule element. It returns false for a rule that ivy
// cannot evaluate.
func (b *book) condRule(sh *sheet.Sheet, index int, n roxmltree.Node) (sheet.CondRule, bool) {
	r := sheet.CondRule{
		Kind:         attr(n, "type"),
		Priority:     integer(n, "priority", 0),
		StopIfTrue:   flag(n, "stopIfTrue", false),
		Operator:     attr(n, "operator"),
		Text:         attr(n, "text"),
		Rank:         integer(n, "rank", 10),
		Percent:      flag(n, "percent", false),
		Bottom:       flag(n, "bottom", false),
		Below:        !flag(n, "aboveAverage", true),
		EqualAverage: flag(n, "equalAverage", false),
		StdDev:       max(0, min(3, integer(n, "stdDev", 0))),
	}
	if id := integer(n, "dxfId", -1); id >= 0 && id < len(b.styles.dxfs) {
		r.Diff = &b.styles.dxfs[id]
	}
	switch r.Kind {
	case "cellIs":
		for fn := range children(n, "formula") {
			o, ok := b.operand(sh, nodeText(fn))
			if !ok {
				return r, false
			}
			r.Operands = append(r.Operands, o)
		}
		return r, len(r.Operands) > 0 && r.Diff != nil
	case "top10", "aboveAverage", "duplicateValues", "uniqueValues", "containsText", "notContainsText",
		"beginsWith", "endsWith", "containsBlanks", "notContainsBlanks", "containsErrors", "notContainsErrors":
		return r, r.Diff != nil
	case "colorScale":
		cs, ok := child(n, "colorScale")
		if !ok {
			return r, false
		}
		thresholds := b.thresholds(sh, index, cs)
		i := 0
		for c := range children(cs, "color") {
			if i < len(thresholds) {
				col, _ := b.styles.color(c)
				r.Scale = append(r.Scale, sheet.ScalePoint{Threshold: thresholds[i], Color: col})
			}
			i++
		}
		return r, len(r.Scale) >= 2
	case "dataBar":
		db, ok := child(n, "dataBar")
		if !ok {
			return r, false
		}
		thresholds := b.thresholds(sh, index, db)
		if len(thresholds) < 2 {
			return r, false
		}
		bar := &sheet.DataBar{Min: thresholds[0], Max: thresholds[1], Gradient: true, HideValue: !flag(db, "showValue", true)}
		if c, ok := child(db, "color"); ok {
			bar.Color, _ = b.styles.color(c)
		}
		if bar.Color.A == 0 {
			bar.Color = rgb(0x638ec6)
		}
		r.Bar = bar
		return r, true
	case "iconSet":
		is, ok := child(n, "iconSet")
		if !ok {
			return r, false
		}
		set := &sheet.IconSet{
			Name:       attr(is, "iconSet"),
			Reverse:    flag(is, "reverse", false),
			HideValue:  !flag(is, "showValue", true),
			Thresholds: b.thresholds(sh, index, is),
		}
		if set.Name == "" {
			set.Name = "3TrafficLights1"
		}
		r.Icons = set
		return r, len(set.Thresholds) >= 2
	}
	return r, false
}

// thresholds reads the cfvo elements of a color scale, a data bar or an
// icon set of the sheet sh of the index index.
func (b *book) thresholds(sh *sheet.Sheet, index int, n roxmltree.Node) []sheet.Threshold {
	var out []sheet.Threshold
	for c := range children(n, "cfvo") {
		t := sheet.Threshold{Kind: attr(c, "type"), GTE: flag(c, "gte", true)}
		switch t.Kind {
		case "num", "percent", "percentile":
			v, ok := b.thresholdValue(sh, index, attr(c, "val"))
			t.Value = v
			if !ok {
				// a formula that ivy does not compute: the lowest or the
				// highest value
				t.Kind = ""
			}
		case "min", "max":
		case "autoMin":
			t.Kind = "min"
		case "autoMax":
			t.Kind = "max"
		default:
			t.Kind = ""
		}
		out = append(out, t)
		if len(out) >= 5 {
			break
		}
	}
	return out
}

// thresholdValue returns the value of the formula f of a threshold of the
// sheet sh of the index index. It computes each formula of a sheet once.
func (b *book) thresholdValue(sh *sheet.Sheet, index int, f string) (float64, bool) {
	f = strings.TrimSpace(f)
	if v, err := strconv.ParseFloat(f, 64); err == nil {
		return v, true
	}
	key := nameKey{index, f}
	if v, ok := b.formulas[key]; ok {
		return v.value, v.ok
	}
	v, ok := b.evalNumber(sh, index, f, 0)
	if b.formulas == nil {
		b.formulas = make(map[nameKey]formulaValue)
	}
	b.formulas[key] = formulaValue{v, ok}
	return v, ok
}

// operand reads the formula of a rule of cellIs: a number, a text in
// quotes, or an absolute reference to a cell of the sheet, such as $B$1.
func (b *book) operand(sh *sheet.Sheet, s string) (sheet.Operand, bool) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "=") {
		s = strings.TrimSpace(s[1:])
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return sheet.Operand{Number: v}, true
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return sheet.Operand{Text: strings.ReplaceAll(s[1:len(s)-1], `""`, `"`), IsText: true}, true
	}
	if strings.Count(s, "$") == 2 && strings.HasPrefix(s, "$") {
		row, col, ok := parseRef(s)
		if !ok {
			return sheet.Operand{}, false
		}
		for i := range sh.Rows {
			if sh.Rows[i].Index != row {
				continue
			}
			for _, c := range sh.Rows[i].Cells {
				if c.Col != col {
					continue
				}
				if c.Kind == sheet.KindNumber {
					return sheet.Operand{Number: c.Value}, true
				}
				return sheet.Operand{Text: c.Text, IsText: true}, true
			}
		}
		// an empty cell is 0
		return sheet.Operand{}, true
	}
	return sheet.Operand{}, false
}
