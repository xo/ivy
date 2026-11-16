package sheet

import (
	"math"
	"strconv"
	"strings"
)

// This file holds what the two formats of Excel share, the Office Open XML
// of xlsx and the binary format of msxls: the sizes of the papers, the
// widths of the columns, and the codes of the headers and the footers.

// paperSizes are the sizes of the papers of Excel, by their number, in
// points. Office Open XML (ECMA-376 Part 1, 18.3.1.63) and the binary
// format number them the same.
var paperSizes = map[int][2]float64{
	1:  {612, 792},         // Letter
	2:  {612, 792},         // Letter small
	3:  {792, 1224},        // Tabloid
	4:  {1224, 792},        // Ledger
	5:  {612, 1008},        // Legal
	6:  {396, 612},         // Statement
	7:  {522, 756},         // Executive
	8:  {841.89, 1190.55},  // A3
	9:  {595.28, 841.89},   // A4
	10: {595.28, 841.89},   // A4 small
	11: {419.53, 595.28},   // A5
	12: {728.5, 1031.81},   // B4 (JIS)
	13: {515.91, 728.5},    // B5 (JIS)
	14: {612, 936},         // Folio
	15: {609.45, 779.53},   // Quarto
	16: {720, 1008},        // 10 by 14 inches
	17: {792, 1224},        // 11 by 17 inches
	18: {612, 792},         // Note
	19: {279, 639},         // envelope 9
	20: {297, 684},         // envelope 10
	27: {311.81, 623.62},   // envelope DL
	28: {459.21, 649.13},   // envelope C5
	34: {498.9, 708.66},    // envelope B5
	37: {279, 540},         // envelope Monarch
	66: {1190.55, 1683.78}, // A2
	70: {297.64, 419.53},   // A6
}

// PaperSize returns the width and the height in points of the paper of
// Excel with the number n, in portrait, and true. It returns false for a
// number that it does not know.
func PaperSize(n int) (float64, float64, bool) {
	size, ok := paperSizes[n]
	return size[0], size[1], ok
}

// digitWidths are the widths of a digit of the common fonts, as a share of
// the size of the font. Excel measures the widths of the columns in the
// digits of the default font.
var digitWidths = map[string]float64{
	"calibri": 0.5068, "carlito": 0.5068, "aptos": 0.53, "aptos narrow": 0.46,
	"arial": 0.556, "liberation sans": 0.556, "helvetica": 0.556,
	"times new roman": 0.5, "liberation serif": 0.5, "times": 0.5,
	"cambria": 0.5552, "caladea": 0.5552,
	"courier new": 0.6, "liberation mono": 0.6, "courier": 0.6,
	"verdana": 0.636, "tahoma": 0.546, "segoe ui": 0.559, "georgia": 0.614,
}

// MaxDigitWidth returns the width of the widest digit of the font f, in
// pixels at 96 dots per inch, as Excel rounds it. A font without a size
// has the size def.
func MaxDigitWidth(f Font, def float64) float64 {
	ratio, ok := digitWidths[strings.ToLower(f.Family)]
	if !ok {
		ratio = 0.55
	}
	size := f.Size
	if size <= 0 {
		size = def
	}
	return math.Max(1, math.Floor(size*96/72*ratio))
}

// WidthToPoints converts a width of a column of Excel, in characters with
// their padding, to points (ECMA-376 Part 1, 18.3.1.13). mdw is the width
// of the widest digit of the default font, from MaxDigitWidth.
func WidthToPoints(chars, mdw float64) float64 {
	if chars <= 0 {
		return 0
	}
	chars = math.Min(chars, 255)
	px := math.Trunc((256*chars + math.Trunc(128/mdw)) / 256 * mdw)
	return px * 0.75
}

// ParseHeaderFooter parses the text of a header or a footer of Excel, with
// its codes, such as "&LPage &P of &N&R&A" (ECMA-376 Part 1, 18.3.1.46). It
// reads the sections &L, &C and &R, the page &P, the pages &N, the sheet &A
// and the file &F. It leaves out the date &D, the time &T, the path &Z and
// the picture &G, and the codes of the fonts, except the first font and the
// first size, which become the font of the whole header.
func ParseHeaderFooter(s string) HeaderFooter {
	var hf HeaderFooter
	part := &hf.Center
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			*part = append(*part, Field{Kind: FieldText, Text: text.String()})
			text.Reset()
		}
	}
	field := func(k FieldKind) {
		flush()
		*part = append(*part, Field{Kind: k})
	}
	var font Font
	hasFont := false
	for i := 0; i < len(s); i++ {
		if s[i] != '&' || i+1 >= len(s) {
			text.WriteByte(s[i])
			continue
		}
		i++
		switch c := s[i]; c {
		case '&':
			text.WriteByte('&')
		case 'L':
			flush()
			part = &hf.Left
		case 'C':
			flush()
			part = &hf.Center
		case 'R':
			flush()
			part = &hf.Right
		case 'P':
			field(FieldPage)
		case 'N':
			field(FieldPages)
		case 'A':
			field(FieldSheet)
		case 'F':
			field(FieldTitle)
		case '"':
			j := strings.IndexByte(s[i+1:], '"')
			if j < 0 {
				i = len(s)
				continue
			}
			name, style, _ := strings.Cut(s[i+1:i+1+j], ",")
			if !hasFont && name != "-" {
				font.Family = name
				font.Bold = strings.Contains(strings.ToLower(style), "bold")
				font.Italic = strings.Contains(strings.ToLower(style), "italic")
				hasFont = true
			}
			i += j + 1
		case 'K':
			// a color: six digits, or a color of the theme
			i += min(6, len(s)-i-1)
		default:
			if c >= '0' && c <= '9' {
				j := i
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
				if size, err := strconv.Atoi(s[i:j]); err == nil && font.Size == 0 && size > 0 && size < 410 {
					font.Size = float64(size)
					hasFont = true
				}
				i = j - 1
			}
			// &D, &T, &Z, &G and the codes of the style are left out
		}
	}
	flush()
	if hasFont {
		hf.Font = &font
	}
	return hf
}
