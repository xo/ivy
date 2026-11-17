package sheet

import (
	"image/color"
	"math"
	"slices"
	"sort"
	"strings"
)

// This file evaluates the conditional formats of a sheet: the rules that
// change the look of a cell from its value, as Excel shows them (ECMA-376
// Part 1, 18.3.1.10 and 18.3.1.18). A format reads its rules into
// Sheet.CondFormats, and Source applies them. A rule of a formula of its
// own, such as the type expression of Excel, needs the formulas, which ivy
// does not compute: the format leaves it out.

// CondFormat is a conditional format: rules over the cells of its ranges.
type CondFormat struct {
	Ranges []Range
	Rules  []CondRule
}

// CondRule is a rule of a conditional format.
type CondRule struct {
	// Kind is the test of the rule, with the names of Excel: cellIs,
	// top10, aboveAverage, duplicateValues, uniqueValues, containsText,
	// notContainsText, beginsWith, endsWith, containsBlanks,
	// notContainsBlanks, containsErrors, notContainsErrors, colorScale,
	// dataBar or iconSet.
	Kind string
	// Priority orders the rules of a sheet: the rule with the lowest
	// number wins over the others. StopIfTrue applies no rule after a
	// rule that matches.
	Priority   int
	StopIfTrue bool
	// Operator is the comparison of cellIs: between, notBetween, equal,
	// notEqual, greaterThan, lessThan, greaterThanOrEqual or
	// lessThanOrEqual. Operands are the values that it compares with.
	Operator string
	Operands []Operand
	// Text is the text of containsText, notContainsText, beginsWith and
	// endsWith.
	Text string
	// Rank is the number of the cells of top10, or their percent with
	// Percent. Bottom takes the lowest values.
	Rank            int
	Percent, Bottom bool
	// Below takes the values below the average for aboveAverage, and
	// EqualAverage the average too. StdDev moves the average by a number
	// of standard deviations.
	Below, EqualAverage bool
	StdDev              int
	// Diff is the change of the style of a cell that the rule matches.
	Diff *Diff
	// Scale holds the two or three points of a colorScale.
	Scale []ScalePoint
	// Bar is the bar of a dataBar, and Icons the icons of an iconSet.
	Bar   *DataBar
	Icons *IconSet
}

// Operand is a value of a rule: a number, or a text.
type Operand struct {
	Number float64
	Text   string
	IsText bool
}

// Threshold is a value of a color scale, a data bar or an icon set: Kind
// is min, max, num, percent or percentile, with Value. GTE takes a value
// equal to the threshold, for an icon set.
type Threshold struct {
	Kind  string
	Value float64
	GTE   bool
}

// ScalePoint is a point of a color scale.
type ScalePoint struct {
	Threshold
	Color color.NRGBA
}

// DataBar is the bar of a data bar.
type DataBar struct {
	Min, Max Threshold
	Color    color.NRGBA
	// Gradient fades the bar to white, as Excel 2007 draws it.
	Gradient bool
	// HideValue hides the text of the cell.
	HideValue bool
}

// IconSet is the icons of an icon set.
type IconSet struct {
	// Name is the name of the set of Excel, such as 3Arrows or
	// 4TrafficLights.
	Name string
	// Thresholds are the thresholds of the icons, the first one for the
	// lowest icon.
	Thresholds []Threshold
	// Reverse puts the icons in the other order, and HideValue hides the
	// text of the cell.
	Reverse, HideValue bool
}

// Toggle is the change of a flag of a style: Keep, On or Off.
type Toggle int8

// The changes of a flag.
const (
	Keep Toggle = iota
	On
	Off
)

// Diff is the change of the style of a cell that a rule makes, the dxf of
// Excel. A transparent color and a border without a width keep the
// style.
type Diff struct {
	Color, Fill          color.NRGBA
	Bold, Italic, Strike Toggle
	// Underline is 0 to keep the style, 1 for a single line, 2 for a
	// double line and -1 for none.
	Underline int
	Borders   [4]Border
}

// condResult is what the rules make of one cell: a style, a bar and an
// icon.
type condResult struct {
	diffs []*Diff
	// fill is the color of a color scale, or transparent.
	fill color.NRGBA
	bar  *barDraw
	icon *iconDraw
}

// barDraw is a data bar in a cell: its length as a share of the cell.
type barDraw struct {
	frac float64
	bar  *DataBar
}

// iconDraw is the icon of a cell.
type iconDraw struct {
	set   *IconSet
	index int
	count int
	hide  bool
}

// maxCondCells is the number of the cells that the rules of a sheet
// test, and maxCondArea the area of a range whose empty cells the rules of
// blanks test.
const (
	maxCondCells = 1 << 18
	maxCondArea  = 1 << 16
)

// condCell is a cell of the range of a conditional format.
type condCell struct {
	row, col int
	cell     *Cell
}

// number returns the number of a cell, and false for a cell that does not
// hold a number.
func (c condCell) number() (float64, bool) {
	if c.cell == nil || c.cell.Kind != KindNumber || math.IsNaN(c.cell.Value) {
		return 0, false
	}
	return c.cell.Value, true
}

// text returns the text of a cell.
func (c condCell) text() string {
	if c.cell == nil {
		return ""
	}
	if len(c.cell.Runs) > 0 {
		var b strings.Builder
		for _, r := range c.cell.Runs {
			b.WriteString(r.Text)
		}
		return b.String()
	}
	return c.cell.Text
}

// condCells returns the cells of the ranges, with the empty cells of a
// small range when blanks is set.
func (sh *Sheet) condCells(ranges []Range, blanks bool, budget *int) []condCell {
	var out []condCell
	seen := map[[2]int]bool{}
	for _, r := range ranges {
		r, ok := clip(r)
		if !ok {
			continue
		}
		area := (r.Row1 - r.Row0 + 1) * (r.Col1 - r.Col0 + 1)
		if blanks && area <= maxCondArea {
			for i := r.Row0; i <= r.Row1; i++ {
				row := sh.row(i)
				for c := r.Col0; c <= r.Col1; c++ {
					if *budget <= 0 {
						return out
					}
					*budget--
					if !seen[[2]int{i, c}] {
						seen[[2]int{i, c}] = true
						out = append(out, condCell{row: i, col: c, cell: row.cell(c)})
					}
				}
			}
			continue
		}
		lo := sort.Search(len(sh.Rows), func(i int) bool { return sh.Rows[i].Index >= r.Row0 })
		for k := lo; k < len(sh.Rows) && sh.Rows[k].Index <= r.Row1; k++ {
			row := &sh.Rows[k]
			for j := range row.Cells {
				c := &row.Cells[j]
				if c.Col < r.Col0 || c.Col > r.Col1 {
					continue
				}
				if *budget <= 0 {
					return out
				}
				*budget--
				if !seen[[2]int{row.Index, c.Col}] {
					seen[[2]int{row.Index, c.Col}] = true
					out = append(out, condCell{row: row.Index, col: c.Col, cell: c})
				}
			}
		}
	}
	return out
}

// condRule is a rule with the cells of its format.
type condRule struct {
	rule  *CondRule
	cells []condCell
}

// evalCond evaluates the conditional formats of the sheet, and returns
// what they make of each cell, by its row and its column.
func (sh *Sheet) evalCond() map[[2]int]*condResult {
	if len(sh.CondFormats) == 0 {
		return nil
	}
	budget := maxCondCells
	var rules []condRule
	for i := range sh.CondFormats {
		f := &sh.CondFormats[i]
		blanks := slices.ContainsFunc(f.Rules, func(r CondRule) bool {
			return r.Kind == "containsBlanks" || r.Kind == "notContainsBlanks"
		})
		cells := sh.condCells(f.Ranges, blanks, &budget)
		for j := range f.Rules {
			rules = append(rules, condRule{rule: &f.Rules[j], cells: cells})
		}
	}
	slices.SortStableFunc(rules, func(a, b condRule) int { return a.rule.Priority - b.rule.Priority })
	out := map[[2]int]*condResult{}
	stopped := map[[2]int]bool{}
	for _, cr := range rules {
		match := cr.rule.matcher(cr.cells)
		limits := cr.rule.limits(cr.cells)
		for _, c := range cr.cells {
			key := [2]int{c.row, c.col}
			if stopped[key] {
				continue
			}
			res := out[key]
			if res == nil {
				res = &condResult{}
			}
			if !cr.rule.apply(c, match, limits, res) {
				continue
			}
			out[key] = res
			if cr.rule.StopIfTrue {
				stopped[key] = true
			}
		}
	}
	return out
}

// matcher returns the test of a rule that compares a cell with the other
// cells of its range, or a test of the cell alone.
func (r *CondRule) matcher(cells []condCell) func(c condCell) bool {
	var nums []float64
	for _, c := range cells {
		if v, ok := c.number(); ok {
			nums = append(nums, v)
		}
	}
	switch r.Kind {
	case "top10":
		if len(nums) == 0 {
			return func(condCell) bool { return false }
		}
		sorted := slices.Clone(nums)
		slices.Sort(sorted)
		n := r.Rank
		if r.Percent {
			n = int(math.Floor(float64(len(sorted)) * float64(r.Rank) / 100))
		}
		n = min(max(n, 1), len(sorted))
		if r.Bottom {
			limit := sorted[n-1]
			return func(c condCell) bool { v, ok := c.number(); return ok && v <= limit }
		}
		limit := sorted[len(sorted)-n]
		return func(c condCell) bool { v, ok := c.number(); return ok && v >= limit }
	case "aboveAverage":
		if len(nums) == 0 {
			return func(condCell) bool { return false }
		}
		mean, sd, _ := meanSD(nums)
		limit := mean
		if r.StdDev > 0 {
			if r.Below {
				limit -= float64(r.StdDev) * sd
			} else {
				limit += float64(r.StdDev) * sd
			}
		}
		return func(c condCell) bool {
			v, ok := c.number()
			switch {
			case !ok:
				return false
			case r.Below:
				return v < limit || (r.EqualAverage && v == limit)
			}
			return v > limit || (r.EqualAverage && v == limit)
		}
	case "duplicateValues", "uniqueValues":
		counts := map[string]int{}
		for _, c := range cells {
			if t := c.text(); t != "" {
				counts[strings.ToLower(t)]++
			}
		}
		return func(c condCell) bool {
			t := c.text()
			if t == "" {
				return false
			}
			return (counts[strings.ToLower(t)] > 1) == (r.Kind == "duplicateValues")
		}
	}
	return r.test
}

// meanSD returns the mean and the standard deviation of a sample.
func meanSD(nums []float64) (mean, sd float64, n int) {
	for _, v := range nums {
		mean += v
	}
	n = len(nums)
	if n == 0 {
		return 0, 0, 0
	}
	mean /= float64(n)
	if n > 1 {
		for _, v := range nums {
			sd += (v - mean) * (v - mean)
		}
		sd = math.Sqrt(sd / float64(n-1))
	}
	return mean, sd, n
}

// test reports whether a rule that looks at a cell alone matches the cell.
func (r *CondRule) test(c condCell) bool {
	text := strings.ToLower(c.text())
	want := strings.ToLower(r.Text)
	switch r.Kind {
	case "cellIs":
		return r.compare(c)
	case "containsText":
		return want != "" && strings.Contains(text, want)
	case "notContainsText":
		return !strings.Contains(text, want)
	case "beginsWith":
		return want != "" && strings.HasPrefix(text, want)
	case "endsWith":
		return want != "" && strings.HasSuffix(text, want)
	case "containsBlanks":
		return strings.TrimSpace(text) == ""
	case "notContainsBlanks":
		return strings.TrimSpace(text) != ""
	case "containsErrors":
		return c.cell != nil && c.cell.Kind == KindError
	case "notContainsErrors":
		return c.cell == nil || c.cell.Kind != KindError
	}
	return false
}

// compare reports whether a cell matches the comparison of cellIs. A
// number compares with numbers, and a text with texts, without case. A
// text is greater than any number, as in Excel.
func (r *CondRule) compare(c condCell) bool {
	if len(r.Operands) == 0 || c.cell == nil || (c.cell.Text == "" && len(c.cell.Runs) == 0) {
		return false
	}
	cmp := func(o Operand) int {
		v, isNum := c.number()
		switch {
		case isNum && !o.IsText:
			switch {
			case v < o.Number:
				return -1
			case v > o.Number:
				return 1
			}
			return 0
		case isNum:
			return -1
		case !o.IsText:
			return 1
		}
		return strings.Compare(strings.ToLower(c.text()), strings.ToLower(o.Text))
	}
	a := cmp(r.Operands[0])
	switch r.Operator {
	case "between", "notBetween":
		if len(r.Operands) < 2 {
			return false
		}
		b := cmp(r.Operands[1])
		// the operands of between can come in either order
		lo, hi := r.Operands[0], r.Operands[1]
		if !lo.IsText && !hi.IsText && lo.Number > hi.Number {
			a, b = b, a
		}
		in := a >= 0 && b <= 0
		return in == (r.Operator == "between")
	case "equal":
		return a == 0
	case "notEqual":
		return a != 0
	case "greaterThan":
		return a > 0
	case "lessThan":
		return a < 0
	case "greaterThanOrEqual":
		return a >= 0
	case "lessThanOrEqual":
		return a <= 0
	}
	return false
}

// limits returns the values of the thresholds of a color scale, a data
// bar or an icon set over the cells of its range.
func (r *CondRule) limits(cells []condCell) []float64 {
	var nums []float64
	for _, c := range cells {
		if v, ok := c.number(); ok {
			nums = append(nums, v)
		}
	}
	slices.Sort(nums)
	var out []float64
	switch {
	case r.Kind == "colorScale":
		for i, p := range r.Scale {
			out = append(out, threshold(p.Threshold, nums, i == 0))
		}
	case r.Kind == "dataBar" && r.Bar != nil:
		out = append(out, threshold(r.Bar.Min, nums, true), threshold(r.Bar.Max, nums, false))
	case r.Kind == "iconSet" && r.Icons != nil:
		for _, t := range r.Icons.Thresholds {
			out = append(out, threshold(t, nums, true))
		}
	}
	return out
}

// apply applies the rule to a cell, and reports whether it matched.
// limits are the values of its thresholds.
func (r *CondRule) apply(c condCell, match func(condCell) bool, limits []float64, res *condResult) bool {
	switch r.Kind {
	case "colorScale":
		v, ok := c.number()
		if !ok || len(r.Scale) < 2 || len(limits) != len(r.Scale) || res.fill.A != 0 {
			return false
		}
		res.fill = r.scaleColor(v, limits)
		return true
	case "dataBar":
		v, ok := c.number()
		if !ok || r.Bar == nil || len(limits) != 2 || res.bar != nil {
			return false
		}
		lo, hi := limits[0], limits[1]
		frac := 1.0
		if hi > lo {
			frac = (min(max(v, lo), hi) - lo) / (hi - lo)
		}
		res.bar = &barDraw{frac: frac, bar: r.Bar}
		return true
	case "iconSet":
		v, ok := c.number()
		if !ok || r.Icons == nil || res.icon != nil || len(r.Icons.Thresholds) == 0 || len(limits) != len(r.Icons.Thresholds) {
			return false
		}
		index := 0
		for i, t := range r.Icons.Thresholds {
			if limit := limits[i]; i > 0 && (v > limit || (t.GTE && v == limit)) {
				index = i
			}
		}
		count := len(r.Icons.Thresholds)
		if r.Icons.Reverse {
			index = count - 1 - index
		}
		res.icon = &iconDraw{set: r.Icons, index: index, count: count, hide: r.Icons.HideValue}
		return true
	}
	if r.Diff == nil || !match(c) {
		return false
	}
	res.diffs = append(res.diffs, r.Diff)
	return true
}

// threshold returns the value of a threshold over the sorted numbers of
// a range. low says that a threshold without a value of its own is the
// lowest value.
func threshold(t Threshold, nums []float64, low bool) float64 {
	if len(nums) == 0 {
		return t.Value
	}
	lo, hi := nums[0], nums[len(nums)-1]
	switch t.Kind {
	case "min":
		return lo
	case "max":
		return hi
	case "num":
		return t.Value
	case "percent":
		return lo + (hi-lo)*t.Value/100
	case "percentile":
		pos := min(max(t.Value/100, 0), 1) * float64(len(nums)-1)
		i := int(math.Floor(pos))
		if i+1 >= len(nums) {
			return nums[len(nums)-1]
		}
		return nums[i] + (pos-float64(i))*(nums[i+1]-nums[i])
	}
	if low {
		return lo
	}
	return hi
}

// scaleColor returns the color of the value v on a color scale whose
// points have the values points.
func (r *CondRule) scaleColor(v float64, points []float64) color.NRGBA {
	if v <= points[0] {
		return r.Scale[0].Color
	}
	for i := 1; i < len(points); i++ {
		if v <= points[i] {
			lo, hi := points[i-1], points[i]
			f := 1.0
			if hi > lo {
				f = (v - lo) / (hi - lo)
			}
			return mix(r.Scale[i-1].Color, r.Scale[i].Color, f)
		}
	}
	return r.Scale[len(r.Scale)-1].Color
}

// mix returns the color between a and b at the fraction f, in RGB, as
// Excel mixes the colors of a scale.
func mix(a, b color.NRGBA, f float64) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*f)) }
	return color.NRGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 255}
}

// applyDiffs returns the style st with the changes of the diffs, the one
// of the highest priority first, and the fill of a color scale.
func applyDiffs(st Style, diffs []*Diff, fill color.NRGBA) Style {
	if fill.A != 0 {
		st.Fill = fill
	}
	toggle := func(v *bool, t Toggle) {
		switch t {
		case On:
			*v = true
		case Off:
			*v = false
		}
	}
	// the rule of the highest priority comes first, and wins
	for i := len(diffs) - 1; i >= 0; i-- {
		d := diffs[i]
		if d.Color.A != 0 {
			st.Font.Color = d.Color
		}
		if d.Fill.A != 0 {
			st.Fill = d.Fill
		}
		toggle(&st.Font.Bold, d.Bold)
		toggle(&st.Font.Italic, d.Italic)
		toggle(&st.Font.Strike, d.Strike)
		switch d.Underline {
		case 1, 2:
			st.Font.Underline = d.Underline
		case -1:
			st.Font.Underline = 0
		}
		for k, b := range d.Borders {
			if b.Width > 0 {
				st.Borders[k] = b
			}
		}
	}
	return st
}
