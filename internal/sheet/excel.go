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

// digitWidths are the advances of the widest digit of the common fonts, as
// a share of the size of the font, from the fonts of Windows. Excel
// measures the widths of the columns in the digits of the default font. A
// font that is not in the table has digits of 0.55.
var digitWidths = map[string]float64{
	"calibri": 0.5068, "carlito": 0.5068, "aptos": 0.53, "aptos narrow": 0.46,
	"arial": 0.5562, "liberation sans": 0.5562, "helvetica": 0.5562, "microsoft sans serif": 0.5562,
	"times new roman": 0.5, "liberation serif": 0.5, "times": 0.5, "palatino linotype": 0.5,
	"cambria": 0.5537, "caladea": 0.5537, "georgia": 0.6138, "constantia": 0.5464,
	"courier new": 0.6001, "liberation mono": 0.6001, "courier": 0.6001, "consolas": 0.5498,
	"verdana": 0.6357, "tahoma": 0.5459, "segoe ui": 0.5391, "trebuchet ms": 0.5244,
	"candara": 0.5518, "corbel": 0.5244, "franklin gothic medium": 0.5864, "lucida sans unicode": 0.6323,
	// Century Gothic has the widths of ITC Avant Garde Gothic: the digits
	// are 554 units of 1000 in the widths of the font that Excel puts in
	// the PDF files that it exports
	"century gothic": 0.554, "itc avant garde gothic": 0.554,
	// the fonts of East Asia
	"ms gothic": 0.5, "ms pgothic": 0.5, "ms ui gothic": 0.5, "ms mincho": 0.5, "ms pmincho": 0.5,
	"ｍｓ ゴシック": 0.5, "ｍｓ ｐゴシック": 0.5, "ｍｓ 明朝": 0.5, "ｍｓ ｐ明朝": 0.5,
	"yu gothic": 0.5562, "游ゴシック": 0.5562, "meiryo": 0.5, "メイリオ": 0.5,
	"simsun": 0.5, "宋体": 0.5, "nsimsun": 0.5, "新宋体": 0.5,
	"mingliu": 0.5, "細明體": 0.5, "pmingliu": 0.4697, "新細明體": 0.4697,
	"malgun gothic": 0.5508, "맑은 고딕": 0.5508,
	// Windows 10 and later do not install Gulim and Dotum by default, and
	// Excel measures them in Malgun Gothic, the font that replaces them
	"gulim": 0.5508, "굴림": 0.5508, "dotum": 0.5508, "돋움": 0.5508,
	"batang": 0.5, "바탕": 0.5,
}

// The resolutions of the screen, on which Excel measures the widths of the
// columns, and of the printer, on which it prints them, in dots per inch.
const (
	screenDPI = 96
	printDPI  = 600
)

// Digit is the width of a digit of the default font of a workbook, the
// unit of the widths of the columns of Excel.
type Digit struct {
	// Pixels is the width of the widest digit on the screen, in pixels at
	// 96 dots per inch, rounded to the nearest pixel. Calibri of 11
	// points (7.43 pixels), Arial of 10 points (7.41 pixels) and Century
	// Gothic of 9 points (6.65 pixels) all have digits of 7 pixels in
	// Excel.
	Pixels float64
	// Print is the advance of a digit that Excel prints, in points: the
	// advance at the size in whole pixels of 600 dots per inch, rounded to
	// a dot.
	Print float64
}

// DigitOf returns the digit of the font f. A font without a size has the
// size def.
func DigitOf(f Font, def float64) Digit {
	ratio, ok := digitWidths[strings.ToLower(f.Family)]
	if !ok {
		ratio = 0.55
	}
	size := f.Size
	if size <= 0 {
		size = def
	}
	if !(size > 0) || size > 409 {
		size = 11
	}
	px := math.Max(1, math.Round(size*screenDPI/72*ratio))
	dots := math.Max(1, math.Round(math.Round(size*printDPI/72)*ratio))
	return Digit{Pixels: px, Print: dots * 72 / printDPI}
}

// Column returns the width that Excel prints for a column of chars
// characters with their padding, in points (ECMA-376 Part 1, 18.3.1.13).
// Excel rounds the width to pixels of the screen, and prints the pixels in
// digits of the printer. For example, a column of the default width of
// 9.140625 characters of Calibri of 11 points is 64 pixels on the screen,
// 48 points, and Excel prints it 51.6 points wide.
func (d Digit) Column(chars float64) float64 {
	if !(chars > 0) || d.Pixels <= 0 {
		return 0
	}
	chars = math.Min(chars, 255)
	px := math.Trunc((256*chars + math.Trunc(128/d.Pixels)) / 256 * d.Pixels)
	return px / d.Pixels * d.Print
}

// DefaultColumn returns the width that Excel prints for a column of the
// default width, from the base width of base digits without the padding
// (ECMA-376 Part 1, 18.3.1.81): the pixels of the digits and of the
// padding of 5 pixels, rounded up to a multiple of 8 pixels.
func (d Digit) DefaultColumn(base float64) float64 {
	if !(base > 0) || d.Pixels <= 0 {
		return 0
	}
	base = math.Min(base, 255)
	px := math.Ceil((base*d.Pixels+5)/8) * 8
	return px / d.Pixels * d.Print
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
