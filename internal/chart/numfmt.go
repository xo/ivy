package chart

import (
	"math"
	"strconv"
	"strings"

	"github.com/xo/ivy/internal/numfmt"
)

// This file formats the numbers of a chart with the number formats of
// Excel, such as "#,##0.00" and "m/d/yyyy" (ECMA-376 Part 1, 18.8.31), with
// internal/numfmt. It also converts a format to d3-format, for the labels
// that zuzu writes on an axis.

// formatValue formats v with the format code of Excel. An empty code is
// General. A value that is not finite is the empty string.
func formatValue(v float64, code string) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	if code == "" {
		code = "General"
	}
	return numfmt.Number(code, v, false).Text
}

// isDate reports whether the format code shows a number as a date or a
// time.
func isDate(code string) bool {
	return numfmt.Parse(code).IsDate()
}

// d3Format converts a number format of Excel to a format of d3-format for
// the labels of an axis. It returns the empty string for General, for a
// date, for a fraction, and for a format that d3-format cannot write,
// which keeps the format of zuzu.
func d3Format(code string) string {
	sh := numfmt.Parse(code).Shape()
	if sh.General || sh.Date || sh.Fraction {
		return ""
	}
	var b strings.Builder
	if sh.Currency {
		b.WriteByte('$')
	}
	if sh.Thousands {
		b.WriteByte(',')
	}
	b.WriteString("." + strconv.Itoa(sh.Decimals))
	switch {
	case sh.Percent:
		b.WriteByte('%')
	case sh.Scientific:
		b.WriteByte('e')
	default:
		b.WriteByte('f')
	}
	return b.String()
}
