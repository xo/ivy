package chart

import (
	"math"

	"github.com/xo/ivy/internal/numfmt"
)

// This file formats the numbers of a chart with the number formats of
// Excel, such as "#,##0.00" and "m/d/yyyy" (ECMA-376 Part 1, 18.8.31), with
// internal/numfmt.

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
