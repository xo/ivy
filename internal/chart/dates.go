package chart

import (
	"image/color"
	"math"
	"time"
)

// This file places the points of a date axis (c:dateAx) by their dates, as
// Excel does: each day, month or year of the axis has a category of its
// own, and a date without a value leaves a gap.

// excelEpoch is the day before the day 1 of Excel, 1 January 1900, for the
// days after the false 29 February 1900 of Excel.
var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// dateOf returns the date of a serial number of a day of Excel.
func dateOf(serial float64) time.Time {
	days := math.Floor(serial)
	if days < 61 {
		// Excel counts a 29 February 1900 that did not exist
		days++
	}
	return excelEpoch.AddDate(0, 0, int(days))
}

// serialOf returns the serial number of Excel of a date.
func serialOf(t time.Time) float64 {
	days := math.Round(t.Sub(excelEpoch).Hours() / 24)
	if days < 61 {
		days--
	}
	return days
}

// unitIndex returns the number of the day, the month or the year of a date
// of Excel.
func unitIndex(serial float64, unit string) int {
	switch unit {
	case "months":
		t := dateOf(serial)
		return t.Year()*12 + int(t.Month()) - 1
	case "years":
		return dateOf(serial).Year()
	}
	return int(math.Floor(serial))
}

// unitSerial returns the serial number of the first day of the day, the
// month or the year with the number i.
func unitSerial(i int, unit string) float64 {
	switch unit {
	case "months":
		return serialOf(time.Date(i/12, time.Month(i%12+1), 1, 0, 0, 0, 0, time.UTC))
	case "years":
		return serialOf(time.Date(i, 1, 1, 0, 0, 0, 0, time.UTC))
	}
	return float64(i)
}

// placeDates places the points of the series of a date axis at their
// dates: it gives each series one value for each day, month or year from
// the first date to the last, and the categories the dates in the format
// of the axis. An axis of more than maxCategories units, or dates out of
// the range of Excel, keep the categories in their order.
func (c *Chart) placeDates() {
	if !c.cat.date {
		return
	}
	var list []*series
	lo, hi := math.Inf(1), math.Inf(-1)
	format := ""
	for _, p := range c.plots {
		if p.kind == kindScatter || p.kind == kindPie || p.kind == kindBox || p.secondary {
			continue
		}
		for _, s := range p.series {
			if len(s.dates) == 0 {
				continue
			}
			list = append(list, s)
			for _, d := range s.dates {
				if !math.IsNaN(d) {
					lo, hi = min(lo, d), max(hi, d)
				}
			}
			if format == "" {
				format = s.dateFormat
			}
		}
	}
	// the dates of Excel go from 1900 to 9999
	if len(list) == 0 || lo < 1 || hi > 2958465 {
		return
	}
	unit := c.cat.baseUnit
	if unit != "days" && unit != "months" && unit != "years" {
		unit = "days"
		if hi-lo >= maxCategories {
			unit = "months"
		}
	}
	first, last := unitIndex(lo, unit), unitIndex(hi, unit)
	n := last - first + 1
	if n <= 0 || n > maxCategories {
		return
	}
	if c.cat.format != "" {
		format = c.cat.format
	}
	if !isDate(format) {
		format = "m/d/yyyy"
	}
	cats := make([]string, n)
	for k := range cats {
		cats[k] = formatValue(unitSerial(first+k, unit), format)
	}
	for _, s := range list {
		vals := make([]float64, n)
		for k := range vals {
			vals[k] = math.NaN()
		}
		// the colors and the labels of single points move with their
		// points
		var points map[int]color.NRGBA
		var labels map[int]*pointLabel
		for i, d := range s.dates {
			if math.IsNaN(d) || i >= len(s.vals) {
				continue
			}
			k := unitIndex(d, unit) - first
			vals[k] = s.vals[i]
			if pc, ok := s.points[i]; ok {
				if points == nil {
					points = map[int]color.NRGBA{}
				}
				points[k] = pc
			}
			if pl, ok := s.labels.points[i]; ok {
				if labels == nil {
					labels = map[int]*pointLabel{}
				}
				labels[k] = pl
			}
		}
		s.vals, s.cats, s.span = vals, cats, true
		s.points, s.labels.points = points, labels
	}
}
