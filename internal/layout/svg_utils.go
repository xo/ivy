// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports part of weasyprint/svg/utils.py: normalize and size, which
// the intrinsic size of an SVG image needs. The rest of the package svg of
// upstream is not ported: okage draws the SVG images (D8).

package layout

import (
	"regexp"
	"strconv"
	"strings"
)

// The regular expressions of normalize in weasyprint/svg/utils.py.
var (
	// svgSpaceRE matches the separators.
	svgSpaceRE = regexp.MustCompile(`[ \n\r\t,]+`)
	// svgDotRE matches a number with a dot that another dot follows.
	svgDotRE = regexp.MustCompile(`(\.[0-9-]+)\.`)
)

// svgNormalize is normalize in weasyprint/svg/utils.py. It gives a canonical
// version of a value.
func svgNormalize(s string) string {
	s = strings.ReplaceAll(s, "E", "e")
	var b strings.Builder
	for i := range len(s) {
		if (s[i] == '+' || s[i] == '-') && (i == 0 || s[i-1] != 'e') {
			b.WriteByte(' ')
		}
		b.WriteByte(s[i])
	}
	s = b.String()
	s = svgSpaceRE.ReplaceAllString(s, " ")
	// (\.[0-9-]+)(?=\.) becomes \1 followed by a space. The lookahead does
	// not consume the dot, so the replacement runs until it changes
	// nothing.
	for range len(s) {
		next := svgDotRE.ReplaceAllString(s, "$1 .")
		if next == s {
			break
		}
		s = next
	}
	return strings.TrimSpace(s)
}

// svgLengthsToPixels are the units of LENGTHS_TO_PIXELS, in the order of
// the dict of upstream.
var svgLengthsToPixels = []string{"px", "pt", "pc", "in", "cm", "mm", "q"}

// svgSize is size in weasyprint/svg/utils.py. It computes a size from a
// string, with its units and its percentages.
func svgSize(s string, fontSize, percentageReference float64) float64 {
	if s == "" {
		return 0
	}

	if f, ok := pyFloat(s); ok {
		return f
	}

	// Not a float, try something else
	s, _, _ = strings.Cut(svgNormalize(s), " ")
	number := func(n string) float64 {
		// Port: upstream raises ValueError for a number that is not
		// valid.
		f, _ := pyFloat(n)
		return f
	}
	switch {
	case strings.HasSuffix(s, "%"):
		return number(s[:len(s)-1]) * percentageReference / 100
	case strings.HasSuffix(s, "rem"):
		return fontSize * number(s[:len(s)-3])
	case strings.HasSuffix(s, "em"):
		return fontSize * number(s[:len(s)-2])
	case strings.HasSuffix(s, "ex"):
		// Assume that 1em == 2ex
		return fontSize * number(s[:len(s)-2]) / 2
	}

	for _, unit := range svgLengthsToPixels {
		if strings.HasSuffix(s, unit) {
			return number(s[:len(s)-len(unit)]) * lengthsToPixels[unit]
		}
	}

	// Unknown size
	return 0
}

// pyFloat is float(string) of Python: a number with optional spaces around
// it.
func pyFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "xXpP_") {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
