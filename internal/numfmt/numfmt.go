// Package numfmt formats numbers with the number format codes of Excel
// (ECMA-376 Part 1, 18.8.31), at the quality of a review (D5). The
// spreadsheets of Office Open XML and of the binary format of Excel name a
// format code for each cell, and the file holds the value, not the text
// that Excel shows.
//
// A code has up to four sections, separated by semicolons: the positive
// numbers, the negative numbers, zero, and text. A section can hold a
// condition, such as [>100], and a color, such as [Red]. The package reads
// the digit placeholders 0, # and ?, the decimal point, the thousands
// separator and the scaling by a thousand, percentages, scientific
// notation, fractions, literal text, the dates and the times with the
// elapsed times such as [h]:mm, the text placeholder @, and General.
//
// The package writes the English names of the months and the days, the
// decimal point ".", and the thousands separator ",". It ignores the locale
// of a code, such as [$-409], and the fill of the character after "*". It
// keeps the currency symbol of a code such as [$€-407].
package numfmt

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Result is a value formatted with a code.
type Result struct {
	// Text is the text that Excel shows.
	Text string
	// Color is the color of the section that formats the value, as the
	// number n of [Color n] of Excel, from 1 to 56, or 0 when the section
	// has no color. [Black] is 1, [White] 2, [Red] 3, [Green] 4, [Blue] 5,
	// [Yellow] 6, [Magenta] 7 and [Cyan] 8. Color n is the color of index
	// n+7 of the palette of the workbook.
	Color int
}

// builtin holds the codes of the built-in formats, by their number, as
// Excel shows them in the locale en-US. The formats 5 to 8 and 14 to 22
// depend on the locale.
var builtin = map[int]string{
	0:  "General",
	1:  "0",
	2:  "0.00",
	3:  "#,##0",
	4:  "#,##0.00",
	5:  `"$"#,##0_);\("$"#,##0\)`,
	6:  `"$"#,##0_);[Red]\("$"#,##0\)`,
	7:  `"$"#,##0.00_);\("$"#,##0.00\)`,
	8:  `"$"#,##0.00_);[Red]\("$"#,##0.00\)`,
	9:  "0%",
	10: "0.00%",
	11: "0.00E+00",
	12: "# ?/?",
	13: "# ??/??",
	14: "m/d/yyyy",
	15: "d-mmm-yy",
	16: "d-mmm",
	17: "mmm-yy",
	18: "h:mm AM/PM",
	19: "h:mm:ss AM/PM",
	20: "h:mm",
	21: "h:mm:ss",
	22: "m/d/yyyy h:mm",
	37: "#,##0 ;(#,##0)",
	38: "#,##0 ;[Red](#,##0)",
	39: "#,##0.00;(#,##0.00)",
	40: "#,##0.00;[Red](#,##0.00)",
	41: `_(* #,##0_);_(* \(#,##0\);_(* "-"_);_(@_)`,
	42: `_("$"* #,##0_);_("$"* \(#,##0\);_("$"* "-"_);_(@_)`,
	43: `_(* #,##0.00_);_(* \(#,##0.00\);_(* "-"??_);_(@_)`,
	44: `_("$"* #,##0.00_);_("$"* \(#,##0.00\);_("$"* "-"??_);_(@_)`,
	45: "mm:ss",
	46: "[h]:mm:ss",
	47: "mm:ss.0",
	48: "##0.0E+0",
	49: "@",
}

// Builtin returns the code of the built-in format with the number id, as
// Excel shows it in the locale en-US. It returns false for a number that
// is not a built-in format. A file names a format of its own with a number
// from 164.
func Builtin(id int) (string, bool) {
	code, ok := builtin[id]
	return code, ok
}

// Format is a parsed format code. It is safe for concurrent use.
type Format struct {
	sections []section
}

// cache holds the parsed codes, because a workbook names few codes for
// many cells.
var cache sync.Map

// maxCached is the number of codes that the cache keeps. Past it, Parse
// parses each new code again.
const maxCached = 4096

// cached counts the codes of the cache.
var cached struct {
	sync.Mutex
	n int
}

// Parse parses a format code. It never fails: a part of a code that it
// does not read is literal text.
func Parse(code string) *Format {
	if f, ok := cache.Load(code); ok {
		return f.(*Format)
	}
	f := parse(code)
	cached.Lock()
	if cached.n < maxCached {
		cached.n++
		cache.Store(code, f)
	}
	cached.Unlock()
	return f
}

// Number formats the number v with the code. date1904 selects the date
// system of 1904, whose day 0 is 1 January 1904, instead of the system of
// 1900, whose day 1 is 1 January 1900.
func Number(code string, v float64, date1904 bool) Result {
	return Parse(code).Number(v, date1904)
}

// Text formats the text s with the code: the text section of the code, or
// s as it is.
func Text(code, s string) Result {
	return Parse(code).Text(s)
}

// IsDate reports whether the format shows a number as a date or a time.
func (f *Format) IsDate() bool {
	return len(f.sections) > 0 && f.sections[0].date
}

// Shape is the form of the numbers that a section of a format shows. A
// program that writes a format in another syntax reads it, such as a chart
// that gives the format of its axes to d3-format.
type Shape struct {
	// General is set for the format General, and for a section without
	// digit placeholders, such as "@" or a text in quotes.
	General bool
	// Date is set for a date or a time.
	Date bool
	// Fraction is set for a fraction, such as "# ?/?".
	Fraction bool
	// Scientific is set for scientific notation, such as "0.00E+00".
	Scientific bool
	// Percent is set when the section multiplies by 100 and shows a
	// percent sign.
	Percent bool
	// Thousands is set when the section separates the thousands.
	Thousands bool
	// Currency is set when the literal text holds a dollar sign.
	Currency bool
	// Decimals is the number of digit placeholders after the decimal
	// point, before the exponent of scientific notation.
	Decimals int
}

// Shape returns the shape of the first section of the format, the section
// of the positive numbers.
func (f *Format) Shape() Shape {
	if len(f.sections) == 0 {
		return Shape{General: true}
	}
	sec := &f.sections[0]
	sh := Shape{
		Date:      sec.date,
		Fraction:  sec.has(tokSlash),
		Percent:   sec.percent > 0,
		Thousands: sec.thousands,
	}
	if sec.date {
		return sh
	}
	point, exp := false, false
	for _, t := range sec.tokens {
		switch t.kind {
		case tokPoint:
			point = true
		case tokExp:
			exp = true
		case tokDigit:
			if point && !exp {
				sh.Decimals++
			}
		case tokLiteral:
			if strings.Contains(t.text, "$") {
				sh.Currency = true
			}
		}
	}
	sh.Scientific = exp
	sh.General = sec.has(tokGeneral) || !sec.has(tokDigit)
	return sh
}

// Number formats the number v. date1904 selects the date system of 1904.
func (f *Format) Number(v float64, date1904 bool) Result {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Result{Text: "#NUM!"}
	}
	sec, neg := f.pick(v)
	if sec == nil {
		return Result{Text: General(v)}
	}
	res := Result{Color: sec.color}
	switch {
	case sec.date:
		if v < 0 {
			// Excel fills the cell with # for a negative date
			res.Text = strings.Repeat("#", 8)
			return res
		}
		res.Text = sec.formatDate(v, date1904)
	default:
		res.Text = sec.formatNumber(math.Abs(v), neg)
	}
	return res
}

// Text formats the text s.
func (f *Format) Text(s string) Result {
	var sec *section
	switch {
	case len(f.sections) >= 4:
		sec = &f.sections[3]
	case len(f.sections) == 1 && f.sections[0].text:
		sec = &f.sections[0]
	}
	if sec == nil {
		return Result{Text: s}
	}
	var b strings.Builder
	for _, t := range sec.tokens {
		switch t.kind {
		case tokLiteral:
			b.WriteString(t.text)
		case tokText:
			b.WriteString(s)
		}
	}
	return Result{Text: b.String(), Color: sec.color}
}

// pick returns the section that formats v, and whether the text of the
// section needs a minus sign.
func (f *Format) pick(v float64) (*section, bool) {
	if len(f.sections) == 1 && f.sections[0].text {
		// a format of text only, such as "@", shows a number as General
		return nil, false
	}
	var num []*section
	for i := range f.sections {
		if i == 3 {
			break
		}
		num = append(num, &f.sections[i])
	}
	if len(num) == 0 {
		return nil, false
	}
	if num[0].cond.op != "" || (len(num) > 1 && num[1].cond.op != "") {
		for i, s := range num {
			if i < 2 && s.cond.op != "" && s.cond.match(v) {
				return s, v < 0 && !s.cond.negative()
			}
		}
		// A value that no condition matches takes the next section, or
		// the last one.
		for i, s := range num {
			if s.cond.op == "" && i > 0 {
				return s, v < 0
			}
		}
		return num[len(num)-1], v < 0
	}
	switch {
	case v < 0 && len(num) >= 2:
		return num[1], false
	case v == 0 && len(num) >= 3:
		return num[2], false
	}
	return num[0], v < 0
}

// condition is the condition of a section, such as [>=100].
type condition struct {
	op  string
	val float64
}

// match reports whether v meets the condition.
func (c condition) match(v float64) bool {
	switch c.op {
	case "<":
		return v < c.val
	case "<=":
		return v <= c.val
	case ">":
		return v > c.val
	case ">=":
		return v >= c.val
	case "=":
		return v == c.val
	case "<>":
		return v != c.val
	}
	return false
}

// negative reports whether the condition holds only for negative numbers,
// so that the section shows the number without its sign.
func (c condition) negative() bool {
	return (c.op == "<" && c.val <= 0) || (c.op == "<=" && c.val < 0)
}

// The kinds of the tokens of a section.
const (
	tokLiteral = iota
	// tokDigit is a digit placeholder: 0, # or ?.
	tokDigit
	// tokPoint is the decimal point.
	tokPoint
	// tokExp is the E of scientific notation, with its sign in text.
	tokExp
	// tokSlash is the slash of a fraction.
	tokSlash
	// tokPercent is a percent sign.
	tokPercent
	// tokText is the text placeholder @.
	tokText
	// tokGeneral is the keyword General.
	tokGeneral
	// tokDate is a part of a date or a time, such as yyyy or mm.
	tokDate
)

// token is a token of a section.
type token struct {
	kind int
	// text is the text of a literal, the placeholder of a digit, the sign
	// of an exponent, or the code of a part of a date, such as "yyyy",
	// "M" for the minutes, "[h]" or "AM/PM".
	text string
}

// section is one section of a code.
type section struct {
	tokens []token
	cond   condition
	color  int
	// date is set when the section formats dates and times.
	date bool
	// text is set when the section holds the text placeholder @.
	text bool
	// thousands is set when the section separates the thousands.
	thousands bool
	// scale is the number of times that the section divides by 1000.
	scale int
	// percent counts the percent signs.
	percent int
}

// colors holds the names of the colors of a section, with their number.
var colors = map[string]int{
	"black": 1, "white": 2, "red": 3, "green": 4,
	"blue": 5, "yellow": 6, "magenta": 7, "cyan": 8,
}

// parse parses a code.
func parse(code string) *Format {
	f := &Format{}
	for _, part := range split(code) {
		f.sections = append(f.sections, parseSection(part))
		if len(f.sections) == 4 {
			break
		}
	}
	if len(f.sections) == 0 {
		f.sections = append(f.sections, section{tokens: []token{{kind: tokGeneral}}})
	}
	return f
}

// split splits a code into its sections, at each semicolon outside quotes,
// brackets and escapes.
func split(code string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(code); i++ {
		switch code[i] {
		case '"':
			if j := strings.IndexByte(code[i+1:], '"'); j >= 0 {
				i += j + 1
			} else {
				i = len(code)
			}
		case '\\', '_', '*':
			i++
		case '[':
			if j := strings.IndexByte(code[i:], ']'); j >= 0 {
				i += j
			}
		case ';':
			parts = append(parts, code[start:i])
			start = i + 1
		}
	}
	return append(parts, code[start:])
}

// parseSection parses one section of a code.
func parseSection(s string) section {
	var sec section
	lit := func(text string) {
		if n := len(sec.tokens); n > 0 && sec.tokens[n-1].kind == tokLiteral {
			sec.tokens[n-1].text += text
			return
		}
		sec.tokens = append(sec.tokens, token{kind: tokLiteral, text: text})
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '"':
			j := strings.IndexByte(s[i+1:], '"')
			if j < 0 {
				lit(s[i+1:])
				i = len(s)
				continue
			}
			lit(s[i+1 : i+1+j])
			i += j + 2
		case c == '\\' && i+1 < len(s):
			r := runeAt(s, i+1)
			lit(r)
			i += 1 + len(r)
		case c == '_' && i+1 < len(s):
			// a space of the width of the next character
			lit(" ")
			i += 1 + len(runeAt(s, i+1))
		case c == '*' && i+1 < len(s):
			// the fill with the next character is left out
			i += 1 + len(runeAt(s, i+1))
		case c == '[':
			j := strings.IndexByte(s[i:], ']')
			if j < 0 {
				lit(s[i:])
				i = len(s)
				continue
			}
			sec.bracket(s[i+1:i+j], lit)
			i += j + 1
		case c == '0' || c == '#' || c == '?':
			sec.tokens = append(sec.tokens, token{kind: tokDigit, text: string(c)})
			i++
		case c == '.':
			sec.tokens = append(sec.tokens, token{kind: tokPoint})
			i++
		case c == ',':
			sec.comma(s, i)
			i++
		case c == '%':
			sec.percent++
			sec.tokens = append(sec.tokens, token{kind: tokPercent})
			i++
		case (c == 'E' || c == 'e') && i+1 < len(s) && (s[i+1] == '+' || s[i+1] == '-'):
			sec.tokens = append(sec.tokens, token{kind: tokExp, text: string(s[i+1])})
			i += 2
		case c == '/':
			sec.tokens = append(sec.tokens, token{kind: tokSlash})
			i++
		case c == '@':
			sec.text = true
			sec.tokens = append(sec.tokens, token{kind: tokText})
			i++
		case len(s)-i >= 7 && strings.EqualFold(s[i:i+7], "general"):
			sec.tokens = append(sec.tokens, token{kind: tokGeneral})
			i += 7
		case len(s)-i >= 5 && strings.EqualFold(s[i:i+5], "am/pm"):
			sec.date = true
			sec.tokens = append(sec.tokens, token{kind: tokDate, text: s[i : i+5]})
			i += 5
		case len(s)-i >= 3 && strings.EqualFold(s[i:i+3], "a/p"):
			sec.date = true
			sec.tokens = append(sec.tokens, token{kind: tokDate, text: s[i : i+3]})
			i += 3
		case strings.IndexByte("yYmMdDhHsS", c) >= 0:
			j := i
			for j < len(s) && (s[j]|0x20) == (c|0x20) {
				j++
			}
			sec.date = true
			sec.tokens = append(sec.tokens, token{kind: tokDate, text: strings.ToLower(s[i:j])})
			i = j
		default:
			r := runeAt(s, i)
			lit(r)
			i += len(r)
		}
	}
	if sec.date {
		sec.dateTokens()
	} else {
		sec.fractionTokens()
	}
	return sec
}

// runeAt returns the character of s at the byte i, as a string.
func runeAt(s string, i int) string {
	for j := i + 1; j <= len(s); j++ {
		if j == len(s) || s[j]&0xc0 != 0x80 {
			return s[i:j]
		}
	}
	return s[i:]
}

// bracket reads the text in brackets of a section: a color, a condition,
// an elapsed time, or a currency and a locale.
func (sec *section) bracket(b string, lit func(string)) {
	lower := strings.ToLower(b)
	switch {
	case colors[lower] != 0:
		sec.color = colors[lower]
	case strings.HasPrefix(lower, "color"):
		if n, err := strconv.Atoi(strings.TrimSpace(lower[5:])); err == nil && n >= 1 && n <= 56 {
			sec.color = n
		}
	case strings.HasPrefix(b, "$"):
		// [$€-407]: the currency symbol, then the locale
		sym, _, _ := strings.Cut(b[1:], "-")
		lit(sym)
	case len(lower) > 0 && strings.Trim(lower, "h") == "",
		len(lower) > 0 && strings.Trim(lower, "m") == "",
		len(lower) > 0 && strings.Trim(lower, "s") == "":
		sec.date = true
		sec.tokens = append(sec.tokens, token{kind: tokDate, text: "[" + lower[:1] + "]" + lower[1:]})
	case strings.HasPrefix(b, "<") || strings.HasPrefix(b, ">") || strings.HasPrefix(b, "="):
		op := b[:1]
		rest := b[1:]
		if strings.HasPrefix(rest, "=") || (op == "<" && strings.HasPrefix(rest, ">")) {
			op += rest[:1]
			rest = rest[1:]
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64); err == nil {
			sec.cond = condition{op: op, val: v}
		}
	}
	// Any other text in brackets, such as [DBNum1], is left out.
}

// comma reads a comma at the byte i of s: a thousands separator between
// digit placeholders, or else a scaling by a thousand.
func (sec *section) comma(s string, i int) {
	before := false
	for _, t := range sec.tokens {
		if t.kind == tokDigit {
			before = true
		}
		if t.kind == tokPoint {
			before = false
		}
	}
	if before && i+1 < len(s) && strings.IndexByte("0#?", s[i+1]) >= 0 {
		sec.thousands = true
		return
	}
	if before {
		sec.scale++
		return
	}
	sec.tokens = append(sec.tokens, token{kind: tokLiteral, text: ","})
}

// dateTokens turns the tokens of a date section: an m after an h or before
// an s is the minutes, a point after the seconds starts the fractions of a
// second, and the other number tokens are literal text.
func (sec *section) dateTokens() {
	toks := sec.tokens
	for i := range toks {
		t := &toks[i]
		if t.kind != tokDate || (t.text != "m" && t.text != "mm") {
			continue
		}
		minute := false
		for j := i - 1; j >= 0; j-- {
			if toks[j].kind == tokDate {
				minute = strings.HasPrefix(toks[j].text, "h") || strings.HasPrefix(toks[j].text, "[h")
				break
			}
		}
		for j := i + 1; j < len(toks) && !minute; j++ {
			if toks[j].kind == tokDate {
				minute = strings.HasPrefix(toks[j].text, "s") || strings.HasPrefix(toks[j].text, "[s")
				break
			}
		}
		if minute {
			t.text = strings.ToUpper(t.text)
		}
	}
	var out []token
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch t.kind {
		case tokDate, tokLiteral, tokText:
			out = append(out, t)
		case tokPoint:
			// .0, .00 or .000 after the seconds
			j := i + 1
			for j < len(toks) && toks[j].kind == tokDigit && toks[j].text == "0" && j-i <= 3 {
				j++
			}
			if j > i+1 {
				out = append(out, token{kind: tokDate, text: "." + strings.Repeat("0", j-i-1)})
				i = j - 1
				continue
			}
			out = append(out, token{kind: tokLiteral, text: "."})
		case tokDigit:
			out = append(out, token{kind: tokLiteral, text: t.text})
		case tokSlash:
			out = append(out, token{kind: tokLiteral, text: "/"})
		case tokPercent:
			out = append(out, token{kind: tokLiteral, text: "%"})
		case tokExp:
			out = append(out, token{kind: tokLiteral, text: "E" + t.text})
		case tokGeneral:
			out = append(out, token{kind: tokLiteral, text: "General"})
		}
	}
	sec.tokens = out
}

// fractionTokens turns a slash that does not stand between digits into
// literal text. A fraction has digit placeholders before the slash and
// placeholders or a number after it.
func (sec *section) fractionTokens() {
	toks := sec.tokens
	for i, t := range toks {
		if t.kind != tokSlash {
			continue
		}
		left := i > 0 && toks[i-1].kind == tokDigit
		right := i+1 < len(toks) && (toks[i+1].kind == tokDigit || isNumberLiteral(toks[i+1]))
		if !left || !right {
			toks[i] = token{kind: tokLiteral, text: "/"}
		}
	}
}

// isNumberLiteral reports whether t is a literal that starts with a digit,
// the fixed denominator of a fraction, such as 8 in "# ?/8".
func isNumberLiteral(t token) bool {
	return t.kind == tokLiteral && t.text != "" && t.text[0] >= '1' && t.text[0] <= '9'
}

// formatNumber formats v, which is not negative. neg adds a minus sign.
func (sec *section) formatNumber(v float64, neg bool) string {
	if len(sec.tokens) == 0 {
		return ""
	}
	for range sec.percent {
		v *= 100
	}
	for range sec.scale {
		v /= 1000
	}
	var body string
	switch {
	case sec.has(tokExp):
		body = sec.scientific(v)
	case sec.has(tokSlash):
		body = sec.fraction(v)
	case sec.has(tokDigit) || sec.has(tokPoint):
		body = sec.decimal(v)
	default:
		body = sec.literal(v)
	}
	if neg && strings.ContainsAny(body, "123456789") {
		return "-" + body
	}
	return body
}

// has reports whether the section holds a token of the kind.
func (sec *section) has(kind int) bool {
	for _, t := range sec.tokens {
		if t.kind == kind {
			return true
		}
	}
	return false
}

// literal formats a section without digit placeholders: its literal text,
// with General in the place of the keyword.
func (sec *section) literal(v float64) string {
	var b strings.Builder
	for _, t := range sec.tokens {
		switch t.kind {
		case tokLiteral:
			b.WriteString(t.text)
		case tokPercent:
			b.WriteByte('%')
		case tokGeneral:
			b.WriteString(General(v))
		}
	}
	return b.String()
}

// decimal formats v with digit placeholders and a decimal point.
func (sec *section) decimal(v float64) string {
	point := -1
	for i, t := range sec.tokens {
		if t.kind == tokPoint {
			point = i
			break
		}
	}
	intEnd := len(sec.tokens)
	if point >= 0 {
		intEnd = point
	}
	var frac []string
	if point >= 0 {
		for _, t := range sec.tokens[point+1:] {
			if t.kind == tokDigit {
				frac = append(frac, t.text)
			}
		}
	}
	digits := formatFixed(v, len(frac))
	intPart, fracPart, _ := strings.Cut(digits, ".")
	if intPart == "0" {
		intPart = ""
	}
	out := sec.fillInt(sec.tokens[:intEnd], intPart)
	if point < 0 {
		return out
	}
	return out + "." + fillFrac(sec.tokens[point+1:], fracPart)
}

// fillInt fills the digit placeholders of the integer part toks with the
// digits, from the right. The first placeholder takes the digits that are
// left over. It inserts the thousands separators.
func (sec *section) fillInt(toks []token, digits string) string {
	n := 0
	for _, t := range toks {
		if t.kind == tokDigit {
			n++
		}
	}
	// pieces holds the text of each token, from the left.
	pieces := make([]string, len(toks))
	pos := len(digits)
	k := n
	for i := len(toks) - 1; i >= 0; i-- {
		t := toks[i]
		switch t.kind {
		case tokDigit:
			k--
			switch {
			case k == 0 && pos > 0:
				pieces[i] = digits[:pos]
				pos = 0
			case pos > 0:
				pieces[i] = digits[pos-1 : pos]
				pos--
			case t.text == "0":
				pieces[i] = "0"
			case t.text == "?":
				pieces[i] = " "
			}
		case tokLiteral:
			pieces[i] = t.text
		case tokPercent:
			pieces[i] = "%"
		case tokGeneral:
			pieces[i] = ""
		}
	}
	if n == 0 {
		return strings.Join(pieces, "") + digits
	}
	if !sec.thousands {
		return strings.Join(pieces, "")
	}
	// The digits of the integer part, from the right, get a comma before
	// each group of three.
	count := 0
	for i := len(toks) - 1; i >= 0; i-- {
		if toks[i].kind != tokDigit {
			continue
		}
		p := pieces[i]
		var b []byte
		for j := len(p) - 1; j >= 0; j-- {
			if p[j] >= '0' && p[j] <= '9' {
				if count > 0 && count%3 == 0 {
					b = append(b, ',')
				}
				count++
			}
			b = append(b, p[j])
		}
		for l, r := 0, len(b)-1; l < r; l, r = l+1, r-1 {
			b[l], b[r] = b[r], b[l]
		}
		pieces[i] = string(b)
	}
	return strings.Join(pieces, "")
}

// fillFrac fills the digit placeholders after the decimal point with the
// digits, from the left. A trailing zero of a # is left out, and a
// trailing zero of a ? is a space.
func fillFrac(toks []token, digits string) string {
	var places []int
	for i, t := range toks {
		if t.kind == tokDigit {
			places = append(places, i)
		}
	}
	pieces := make([]string, len(toks))
	for i, t := range toks {
		switch t.kind {
		case tokLiteral:
			pieces[i] = t.text
		case tokPercent:
			pieces[i] = "%"
		}
	}
	trailing := true
	for k := len(places) - 1; k >= 0; k-- {
		i := places[k]
		d := byte('0')
		if k < len(digits) {
			d = digits[k]
		}
		switch {
		case trailing && d == '0' && toks[i].text == "#":
			pieces[i] = ""
		case trailing && d == '0' && toks[i].text == "?":
			pieces[i] = " "
		default:
			trailing = false
			pieces[i] = string(d)
		}
	}
	return strings.Join(pieces, "")
}

// scientific formats v in scientific notation, such as 0.00E+00.
func (sec *section) scientific(v float64) string {
	exp := -1
	for i, t := range sec.tokens {
		if t.kind == tokExp {
			exp = i
			break
		}
	}
	mant := sec.tokens[:exp]
	nInt, nFrac, point := 0, 0, false
	for _, t := range mant {
		switch {
		case t.kind == tokPoint:
			point = true
		case t.kind == tokDigit && point:
			nFrac++
		case t.kind == tokDigit:
			nInt++
		}
	}
	e := 0
	if v != 0 {
		e = int(math.Floor(math.Log10(v)))
		if nInt > 1 {
			// engineering notation: the exponent is a multiple of the
			// number of the integer placeholders
			e = int(math.Floor(float64(e)/float64(nInt))) * nInt
		}
		m := roundHalfUp(v/math.Pow(10, float64(e)), nFrac)
		limit := 10.0
		if nInt > 1 {
			limit = math.Pow(10, float64(nInt))
		}
		if m >= limit {
			if nInt > 1 {
				e += nInt
			} else {
				e++
			}
		}
	}
	m := v / math.Pow(10, float64(e))
	var b strings.Builder
	sub := section{tokens: mant, thousands: sec.thousands}
	b.WriteString(sub.decimal(m))
	b.WriteByte('E')
	switch {
	case e < 0:
		b.WriteByte('-')
	case sec.tokens[exp].text == "+":
		b.WriteByte('+')
	}
	rest := sec.tokens[exp+1:]
	ae := e
	if ae < 0 {
		ae = -ae
	}
	es := strconv.Itoa(ae)
	expSec := section{}
	b.WriteString(expSec.fillInt(rest, es))
	return b.String()
}

// fraction formats v as a fraction, such as # ?/? or ?/8.
func (sec *section) fraction(v float64) string {
	slash := -1
	for i, t := range sec.tokens {
		if t.kind == tokSlash {
			slash = i
			break
		}
	}
	// The numerator is the run of placeholders before the slash. A run of
	// placeholders before it, after a literal, is the integer part.
	numStart := slash
	for numStart > 0 && sec.tokens[numStart-1].kind == tokDigit {
		numStart--
	}
	intEnd := numStart
	for intEnd > 0 && sec.tokens[intEnd-1].kind == tokLiteral {
		intEnd--
	}
	hasInt := false
	for _, t := range sec.tokens[:intEnd] {
		if t.kind == tokDigit {
			hasInt = true
		}
	}
	whole := 0.0
	frac := v
	if hasInt {
		whole = math.Floor(v)
		frac = v - whole
	}
	// The denominator: a fixed number, or the most digits of its
	// placeholders.
	denTok := sec.tokens[slash+1:]
	fixed := 0
	denDigits := 0
	denEnd := 0
	for i, t := range denTok {
		if t.kind == tokDigit {
			denDigits++
			denEnd = i + 1
			continue
		}
		if i == 0 && isNumberLiteral(t) {
			j := 0
			for j < len(t.text) && t.text[j] >= '0' && t.text[j] <= '9' {
				j++
			}
			fixed, _ = strconv.Atoi(t.text[:j])
			denTok = append([]token{{kind: tokLiteral, text: t.text[:j]}, {kind: tokLiteral, text: t.text[j:]}}, denTok[1:]...)
			denEnd = 1
		}
		break
	}
	var num, den int
	if fixed > 0 {
		den = fixed
		num = int(math.Round(frac * float64(den)))
	} else {
		maxDen := int(math.Pow(10, float64(max(1, denDigits)))) - 1
		num, den = approximate(frac, maxDen)
	}
	if hasInt && num == den && den > 0 {
		whole++
		num = 0
	}
	var b strings.Builder
	if hasInt {
		ip := strconv.FormatFloat(whole, 'f', 0, 64)
		if num == 0 {
			// Excel shows the whole number, and spaces for the fraction
			return sec.fillInt(sec.tokens[:intEnd], ip) + strings.Repeat(" ", max(0, slash-intEnd+1+denEnd))
		}
		if ip == "0" {
			ip = ""
		}
		b.WriteString(sec.fillInt(sec.tokens[:intEnd], ip))
		for _, t := range sec.tokens[intEnd:numStart] {
			b.WriteString(t.text)
		}
	} else {
		num += int(whole) * den
	}
	numToks := sec.tokens[numStart:slash]
	b.WriteString(fillPadded(numToks, strconv.Itoa(num)))
	b.WriteByte('/')
	if fixed > 0 {
		b.WriteString(strconv.Itoa(den))
		for _, t := range denTok[1:] {
			if t.kind == tokLiteral {
				b.WriteString(t.text)
			}
		}
		return b.String()
	}
	ds := strconv.Itoa(den)
	for len(ds) < denDigits {
		ds += " "
	}
	b.WriteString(ds)
	for _, t := range denTok[denEnd:] {
		if t.kind == tokLiteral {
			b.WriteString(t.text)
		}
	}
	return b.String()
}

// fillPadded writes digits right aligned in the placeholders toks: a ? is
// a space and a 0 a zero where no digit falls.
func fillPadded(toks []token, digits string) string {
	n := len(toks)
	if len(digits) >= n {
		return digits
	}
	var b strings.Builder
	for _, t := range toks[:n-len(digits)] {
		switch t.text {
		case "?":
			b.WriteByte(' ')
		case "0":
			b.WriteByte('0')
		}
	}
	return b.String() + digits
}

// approximate returns the fraction num/den nearest to x, which is in
// [0, 1], with den at most maxDen. It walks the Stern-Brocot tree with the
// continued fraction of x.
func approximate(x float64, maxDen int) (int, int) {
	if x <= 0 {
		return 0, 1
	}
	if maxDen < 1 {
		maxDen = 1
	}
	// the convergents p/q of the continued fraction
	p0, q0, p1, q1 := 0, 1, 1, 0
	r := x
	for range 64 {
		a := math.Floor(r)
		if a > float64(math.MaxInt32) {
			break
		}
		ai := int(a)
		p2, q2 := ai*p1+p0, ai*q1+q0
		if q2 > maxDen {
			// the best semiconvergent within maxDen
			k := (maxDen - q0) / q1
			ps, qs := k*p1+p0, k*q1+q0
			if math.Abs(x-float64(ps)/float64(qs)) < math.Abs(x-float64(p1)/float64(q1)) {
				return ps, qs
			}
			return p1, q1
		}
		p0, q0, p1, q1 = p1, q1, p2, q2
		f := r - a
		if f < 1e-12 {
			break
		}
		r = 1 / f
	}
	return p1, q1
}

// roundHalfUp rounds v, which is not negative, to n decimals, with a half
// away from zero, as Excel rounds.
func roundHalfUp(v float64, n int) float64 {
	if n > 15 {
		return v
	}
	p := math.Pow(10, float64(n))
	if v*p > 1e17 {
		return v
	}
	// the nearest decimal text of v avoids the error of the binary value,
	// such as 1.005, which is 1.00499... in binary
	s := strconv.FormatFloat(v*p, 'g', 15, 64)
	r, err := strconv.ParseFloat(s, 64)
	if err != nil {
		r = v * p
	}
	return math.Floor(r+0.5+1e-9) / p
}

// formatFixed formats v, which is not negative, with n decimals, rounded
// as Excel rounds. Excel keeps 15 significant digits.
func formatFixed(v float64, n int) string {
	v = significant(v)
	if n > 30 {
		n = 30
	}
	r := roundHalfUp(v, n)
	return strconv.FormatFloat(r, 'f', n, 64)
}

// significant rounds v to the 15 significant digits that Excel keeps.
func significant(v float64) float64 {
	if v == 0 {
		return 0
	}
	r, err := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 15, 64), 64)
	if err != nil {
		return v
	}
	return r
}

// General formats v as the format General does: at most 11 characters, in
// scientific notation for a very large or very small number.
func General(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "#NUM!"
	}
	if v == 0 {
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	v = significant(v)
	if v >= 1e11 || v < 1e-9 {
		return sign + generalScientific(v)
	}
	// the most decimals that fit in 11 characters, with the sign
	for prec := 10; prec >= 0; prec-- {
		s := strconv.FormatFloat(roundHalfUp(v, prec), 'f', prec, 64)
		if strings.Contains(s, ".") {
			s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		}
		if len(s) <= 11 {
			if s == "0" {
				return sign + generalScientific(v)
			}
			return sign + s
		}
	}
	return sign + generalScientific(v)
}

// generalScientific formats v, which is positive, in the scientific
// notation of General, such as 1.23457E+11.
func generalScientific(v float64) string {
	e := int(math.Floor(math.Log10(v)))
	m := v / math.Pow(10, float64(e))
	m = roundHalfUp(m, 5)
	if m >= 10 {
		m /= 10
		e++
	}
	s := strconv.FormatFloat(m, 'f', 5, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	sign := "+"
	if e < 0 {
		sign = "-"
		e = -e
	}
	es := strconv.Itoa(e)
	if len(es) < 2 {
		es = "0" + es
	}
	return s + "E" + sign + es
}

// The days of the epochs of the date systems: the day before day 1 of
// 1900, and day 0 of 1904.
var (
	epoch1900 = time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	epoch1904 = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
)

// maxSerial is the last day that Excel shows: 31 December 9999.
const maxSerial = 2958465

// formatDate formats the serial number v, which is not negative, as a date
// and a time.
func (sec *section) formatDate(v float64, date1904 bool) string {
	if v > maxSerial+1 {
		return strings.Repeat("#", 8)
	}
	// The seconds are rounded to the fractions that the section shows.
	sub := 0
	for _, t := range sec.tokens {
		if t.kind == tokDate && strings.HasPrefix(t.text, ".") {
			sub = max(sub, len(t.text)-1)
		}
	}
	scale := math.Pow(10, float64(sub))
	ticks := math.Floor(v*86400*scale + 0.5)
	days := math.Floor(ticks / (86400 * scale))
	secTicks := ticks - days*86400*scale
	totalSec := math.Floor(ticks / scale)
	secs := int(math.Floor(secTicks / scale))
	fracTicks := int(secTicks - float64(secs)*scale)
	hour, minute, second := secs/3600, secs/60%60, secs%60

	var year, day, weekday int
	var month time.Month
	switch {
	case date1904:
		t := epoch1904.AddDate(0, 0, int(days))
		year, month, day, weekday = t.Year(), t.Month(), t.Day(), int(t.Weekday())
	case days == 0:
		// Excel shows day 0 as 0 January 1900
		year, month, day, weekday = 1900, time.January, 0, 6
	case days == 60:
		// day 60 is 29 February 1900, a day that Excel keeps from Lotus
		// 1-2-3, which took 1900 for a leap year
		year, month, day, weekday = 1900, time.February, 29, 3
	default:
		d := int(days)
		if d > 60 {
			d--
		}
		t := epoch1900.AddDate(0, 0, d)
		year, month, day, weekday = t.Year(), t.Month(), t.Day(), int(t.Weekday())
	}
	ampm := false
	for _, t := range sec.tokens {
		if t.kind == tokDate && (strings.EqualFold(t.text, "am/pm") || strings.EqualFold(t.text, "a/p")) {
			ampm = true
		}
	}
	var b strings.Builder
	for _, t := range sec.tokens {
		switch t.kind {
		case tokLiteral:
			b.WriteString(t.text)
			continue
		case tokText:
			continue
		}
		switch x := t.text; {
		case x == "yy" || x == "y":
			b.WriteString(pad2(year % 100))
		case strings.HasPrefix(x, "y"):
			b.WriteString(strconv.Itoa(year))
		case x == "m":
			b.WriteString(strconv.Itoa(int(month)))
		case x == "mm":
			b.WriteString(pad2(int(month)))
		case x == "mmm":
			b.WriteString(month.String()[:3])
		case x == "mmmmm":
			b.WriteString(month.String()[:1])
		case strings.HasPrefix(x, "mmmm"):
			b.WriteString(month.String())
		case x == "d":
			b.WriteString(strconv.Itoa(day))
		case x == "dd":
			b.WriteString(pad2(day))
		case x == "ddd":
			b.WriteString(time.Weekday(weekday).String()[:3])
		case strings.HasPrefix(x, "dddd"):
			b.WriteString(time.Weekday(weekday).String())
		case x == "h" || x == "hh":
			h := hour
			if ampm {
				h %= 12
				if h == 0 {
					h = 12
				}
			}
			if x == "hh" {
				b.WriteString(pad2(h))
			} else {
				b.WriteString(strconv.Itoa(h))
			}
		case x == "M":
			b.WriteString(strconv.Itoa(minute))
		case strings.HasPrefix(x, "MM"):
			b.WriteString(pad2(minute))
		case x == "s":
			b.WriteString(strconv.Itoa(second))
		case strings.HasPrefix(x, "ss"):
			b.WriteString(pad2(second))
		case strings.HasPrefix(x, "."):
			f := strconv.Itoa(fracTicks)
			for len(f) < sub {
				f = "0" + f
			}
			b.WriteString(".")
			b.WriteString(f[:len(x)-1])
		case strings.HasPrefix(x, "[h]"):
			b.WriteString(padN(int(totalSec)/3600, len(x)-2))
		case strings.HasPrefix(x, "[m]"):
			b.WriteString(padN(int(totalSec)/60, len(x)-2))
		case strings.HasPrefix(x, "[s]"):
			b.WriteString(padN(int(totalSec), len(x)-2))
		case strings.EqualFold(x, "am/pm"):
			s := "AM"
			if hour >= 12 {
				s = "PM"
			}
			if x == "am/pm" {
				s = strings.ToLower(s)
			}
			b.WriteString(s)
		case strings.EqualFold(x, "a/p"):
			s := "A"
			if hour >= 12 {
				s = "P"
			}
			if x[0] == 'a' {
				s = strings.ToLower(s)
			}
			b.WriteString(s)
		default:
			// hours, minutes or seconds of more than two letters
			b.WriteString(x)
		}
	}
	return b.String()
}

// pad2 writes n with two digits at least.
func pad2(n int) string {
	return padN(n, 2)
}

// padN writes n with width digits at least.
func padN(n, width int) string {
	s := strconv.Itoa(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}
