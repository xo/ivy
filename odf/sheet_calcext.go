package odf

import (
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/sheet"
)

// This file reads the conditional formats of LibreOffice in a table,
// calcext:conditional-formats: the color scales, the data bars and the
// icon sets. internal/sheet draws them as it draws those of Excel. The
// conditions that give a style, calcext:condition, are also the style:map
// elements of the styles of the cells, which applyConds reads.

// maxCalcextRules bounds the rules of the conditional formats of a table,
// and maxCalcextRanges the ranges of one format.
const (
	maxCalcextRules  = 1024
	maxCalcextRanges = 64
)

// condFormats reads the element calcext:conditional-formats n into sh.
func (r *odsReader) condFormats(sh *sheet.Sheet, n roxmltree.Node) {
	rules := 0
	for f := range n.Children() {
		if odsLocal(f) != "conditional-format" {
			continue
		}
		var cf sheet.CondFormat
		for _, a := range odsAddresses(ebook.Attr(f, "target-range-address")) {
			if rg, ok := odsRange(a); ok && len(cf.Ranges) < maxCalcextRanges {
				cf.Ranges = append(cf.Ranges, rg)
			}
		}
		if len(cf.Ranges) == 0 {
			continue
		}
		for c := range f.Children() {
			if rules >= maxCalcextRules {
				break
			}
			rule, ok := calcextRule(c)
			if !ok {
				continue
			}
			rules++
			// the formats apply in their order
			rule.Priority = rules
			cf.Rules = append(cf.Rules, rule)
		}
		if len(cf.Rules) > 0 {
			sh.CondFormats = append(sh.CondFormats, cf)
		}
	}
}

// odsAddresses splits a list of cell ranges at the spaces outside the
// quotes of the names of the sheets, such as "'My Sheet'.A1:'My Sheet'.B2
// Sheet2.C3".
func odsAddresses(s string) []string {
	var out []string
	quoted, start := false, 0
	for i := 0; i <= len(s); i++ {
		switch {
		case i < len(s) && s[i] == '\'':
			quoted = !quoted
		case i == len(s) || (s[i] == ' ' && !quoted):
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// calcextRule reads a color scale, a data bar or an icon set of LibreOffice
// as a rule of Excel. It returns false for another element.
func calcextRule(n roxmltree.Node) (sheet.CondRule, bool) {
	r := sheet.CondRule{}
	switch odsLocal(n) {
	case "color-scale":
		r.Kind = "colorScale"
		for e := range n.Children() {
			if odsLocal(e) != "color-scale-entry" || len(r.Scale) >= 3 {
				continue
			}
			c, ok := odsColor(ebook.Attr(e, "color"))
			if !ok {
				return r, false
			}
			r.Scale = append(r.Scale, sheet.ScalePoint{Threshold: calcextThreshold(e), Color: c})
		}
		return r, len(r.Scale) >= 2
	case "data-bar":
		r.Kind = "dataBar"
		var ts []sheet.Threshold
		for e := range n.Children() {
			if odsLocal(e) == "formatting-entry" && len(ts) < 2 {
				ts = append(ts, calcextThreshold(e))
			}
		}
		if len(ts) < 2 {
			return r, false
		}
		bar := &sheet.DataBar{
			Min:       ts[0],
			Max:       ts[1],
			Gradient:  ebook.Attr(n, "gradient") != "false",
			HideValue: ebook.Attr(n, "show-value") == "false",
		}
		c, ok := odsColor(ebook.Attr(n, "positive-color"))
		if !ok {
			// the default of LibreOffice
			c, _ = odsColor("#2a6099")
		}
		bar.Color = c
		r.Bar = bar
		return r, true
	case "icon-set":
		r.Kind = "iconSet"
		set := &sheet.IconSet{
			Name:      ebook.Attr(n, "icon-set-type"),
			HideValue: ebook.Attr(n, "show-value") == "false",
		}
		for e := range n.Children() {
			if odsLocal(e) == "formatting-entry" && len(set.Thresholds) < 5 {
				set.Thresholds = append(set.Thresholds, calcextThreshold(e))
			}
		}
		if set.Name == "" {
			set.Name = "3Arrows"
		}
		r.Icons = set
		return r, len(set.Thresholds) >= 2
	}
	return r, false
}

// calcextThreshold reads the type and the value of an entry of a color
// scale, a data bar or an icon set, as a threshold of Excel. A formula
// that is not a number takes the lowest or the highest value.
func calcextThreshold(e roxmltree.Node) sheet.Threshold {
	t := sheet.Threshold{GTE: true}
	v, err := strconv.ParseFloat(strings.TrimSpace(ebook.Attr(e, "value")), 64)
	switch ebook.Attr(e, "type") {
	case "minimum", "auto-minimum":
		t.Kind = "min"
	case "maximum", "auto-maximum":
		t.Kind = "max"
	case "percent":
		t.Kind = "percent"
	case "percentile":
		t.Kind = "percentile"
	case "number", "formula", "":
		t.Kind = "num"
	}
	if t.Kind == "percent" || t.Kind == "percentile" || t.Kind == "num" {
		if err != nil {
			t.Kind = ""
		} else {
			t.Value = v
		}
	}
	return t
}
