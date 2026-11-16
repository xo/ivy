// Package csv opens files of values separated by commas, semicolons, tabs
// or vertical bars: .csv, .tsv and .tab.
//
// A document puts the rows into one table on pages, as a sheet of Excel
// prints them, with the package sheet and the layout of D10. The table has
// light gridlines, because a file of values has no style that separates its
// cells. A field that holds a number is aligned to the right, and the other
// fields to the left. Each column is as wide as its longest field, from 48
// to 300 points, and a longer field breaks into lines.
//
// The package finds the separator in the first records: the one that
// gives the same number of fields, more than one, to the most records. It
// reads the quotes of RFC 4180, and a quote in a field without quotes as
// text. It reads UTF-8 with or without a byte order mark, UTF-16 with a
// byte order mark or with the zero bytes of ASCII text, and windows-1252
// for any other text that is not valid UTF-8.
//
// A file reads at most Limits.MaxMemory bytes and Limits.MaxObjects
// fields. A file with more fields shows the fields up to the limit, and a
// line after the table says so. A file with a zero byte that is not UTF-16
// is not a text file, and Open returns an error that wraps ivy.ErrFormat.
package csv

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/charset"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/sheet"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "csv",
	Mimes: []string{"text/csv", "text/tab-separated-values", "application/csv", "text/x-csv"},
	Exts:  []string{"csv", "tsv", "tab"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// Option is an option of Open.
type Option func(*options)

type options struct {
	cfg ivy.Config
}

// WithConfig sets the configuration that every format shares.
func WithConfig(cfg ivy.Config) Option {
	return func(o *options) {
		o.cfg = cfg
	}
}

// Document is an open file of separated values.
type Document struct {
	cfg ivy.Config
	lim ivy.Limits
	// rows are the records of the file.
	rows [][]string
	// truncated is set when the file holds more fields than the limit.
	truncated bool
	// sep is the separator of the fields.
	sep byte

	mu     sync.Mutex
	src    *layout.Source
	srcErr error
	// pages holds the book of the layout.
	pages ebook.Pages
}

// The capabilities of a document.
var _ ivy.Pager = (*Document)(nil)

// Open reads a file of separated values and its records. It returns an
// error that wraps ivy.ErrFormat for a file that is not text, and one that
// wraps ivy.ErrLimit for a file larger than Limits.MaxMemory.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening csv: %w", err)
	}
	defer ebook.Recover("opening csv", &err)
	lim := o.cfg.Limits.WithDefaults()
	size := src.Size()
	switch {
	case size < 0:
		return nil, fmt.Errorf("csv of %d bytes: %w", size, ivy.ErrFormat)
	case size > lim.MaxMemory:
		return nil, fmt.Errorf("csv of %d bytes: %w", size, ivy.ErrLimit)
	}
	buf := make([]byte, size)
	if size > 0 {
		if _, err := src.ReadAt(buf, 0); err != nil {
			return nil, fmt.Errorf("reading csv: %w", err)
		}
	}
	text := decode(buf)
	if strings.IndexByte(text, 0) >= 0 {
		return nil, fmt.Errorf("csv with a zero byte: %w", ivy.ErrFormat)
	}
	d = &Document{cfg: o.cfg, lim: lim}
	d.sep = detect(text)
	d.rows, d.truncated, err = parse(ctx, text, d.sep, lim.MaxObjects)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// decode returns the text of buf as UTF-8.
func decode(buf []byte) string {
	switch {
	case bytes.HasPrefix(buf, []byte{0xef, 0xbb, 0xbf}):
		return strings.ToValidUTF8(string(buf[3:]), "�")
	case bytes.HasPrefix(buf, []byte{0xff, 0xfe}):
		s, _ := charset.Decode(buf[2:], "utf-16le")
		return s
	case bytes.HasPrefix(buf, []byte{0xfe, 0xff}):
		s, _ := charset.Decode(buf[2:], "utf-16be")
		return s
	case len(buf) >= 4 && buf[0] != 0 && buf[1] == 0 && buf[2] != 0 && buf[3] == 0:
		// ASCII in UTF-16 without a byte order mark
		s, _ := charset.Decode(buf, "utf-16le")
		return s
	case len(buf) >= 4 && buf[0] == 0 && buf[1] != 0 && buf[2] == 0 && buf[3] != 0:
		s, _ := charset.Decode(buf, "utf-16be")
		return s
	case utf8.Valid(buf):
		return string(buf)
	}
	s, ok := charset.Decode(buf, "windows-1252")
	if !ok {
		return strings.ToValidUTF8(string(buf), "�")
	}
	return s
}

// separators are the separators that detect tries, in the order that
// breaks a tie.
var separators = []byte{'\t', ',', ';', '|'}

// sampleRecords is the number of records that detect reads.
const sampleRecords = 64

// detect returns the separator of text: the one whose most frequent count
// of fields, more than one, holds for the most records of the start of the
// text. It returns a comma when no separator splits a record.
func detect(text string) byte {
	if len(text) > 1<<16 {
		cut := strings.LastIndexByte(text[:1<<16], '\n')
		if cut > 0 {
			text = text[:cut]
		} else {
			text = text[:1<<16]
		}
	}
	best, bestScore := byte(','), 0
	for _, sep := range separators {
		rows, _, _ := parse(context.Background(), text, sep, sampleRecords*256)
		if len(rows) > sampleRecords {
			rows = rows[:sampleRecords]
		}
		counts := make(map[int]int)
		for _, r := range rows {
			if len(r) > 1 {
				counts[len(r)]++
			}
		}
		for fields, n := range counts {
			// the agreement of the records first, then the count
			if score := n*1024 + min(fields, 1023); score > bestScore {
				best, bestScore = sep, score
			}
		}
	}
	return best
}

// parse splits text into records of fields separated by sep, with the
// quotes of RFC 4180. It reads at most maxFields fields, and reports
// whether the text holds more.
func parse(ctx context.Context, text string, sep byte, maxFields int) ([][]string, bool, error) {
	var rows [][]string
	var row []string
	var field strings.Builder
	fields := 0
	quoted, inQuotes, started := false, false, false
	endField := func() {
		row = append(row, field.String())
		field.Reset()
		fields++
		quoted, inQuotes, started = false, false, false
	}
	for i := 0; i < len(text); i++ {
		if i&0xffff == 0 {
			if err := ctx.Err(); err != nil {
				return nil, false, fmt.Errorf("reading csv: %w", err)
			}
		}
		if fields >= maxFields {
			return rows, true, nil
		}
		c := text[i]
		if inQuotes {
			if c == '"' {
				if i+1 < len(text) && text[i+1] == '"' {
					field.WriteByte('"')
					i++
				} else {
					inQuotes = false
				}
				continue
			}
			field.WriteByte(c)
			continue
		}
		switch {
		case c == '"' && !started:
			inQuotes, quoted, started = true, true, true
		case c == sep:
			endField()
		case c == '\r' || c == '\n':
			if c == '\r' && i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
			endField()
			rows = append(rows, row)
			row = nil
		default:
			started = true
			field.WriteByte(c)
		}
	}
	if started || len(row) > 0 || field.Len() > 0 || quoted {
		endField()
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return rows, false, nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. It lays out the whole table
// on its first call.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	return d.pages.PageCount(ctx, d.source, d.cfg)
}

// Page satisfies the ivy.Pager interface. It lays out the table only as far
// as page n.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	return d.pages.Page(ctx, d.source, d.cfg, n)
}

// source converts the records into HTML for the layout, once.
func (d *Document) source(ctx context.Context) (src layout.Source, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case d.srcErr != nil:
		return layout.Source{}, d.srcErr
	case d.src != nil:
		return *d.src, nil
	}
	defer func() {
		switch {
		case err == nil:
			d.src = &src
		case ctx.Err() == nil:
			d.srcErr = err
		}
	}()
	defer ebook.Recover("converting csv", &err)
	return d.workbook().Source(ctx, sheet.Options{Limits: d.lim, Logger: d.cfg.Logger})
}

// The sizes of the columns, in points: the width of a character of
// Calibri of 11 points, as a mean, and the bounds of a column.
const (
	charWidth    = 5.6
	minColWidth  = 48
	maxColWidth  = 300
	colPadding   = 6
	maxSizedCols = 4096
)

// workbook returns the sheet of the records.
func (d *Document) workbook() *sheet.Workbook {
	wb := &sheet.Workbook{Styles: []sheet.Style{sheet.DefaultStyle, sheet.DefaultStyle}}
	wb.Styles[1].Wrap = true
	sh := &sheet.Sheet{
		DefaultColWidth:  minColWidth,
		DefaultRowHeight: 15,
		Gridlines:        true,
		Page:             sheet.DefaultPage,
	}
	var widths []float64
	for i, rec := range d.rows {
		if i >= sheet.MaxRows {
			break
		}
		row := sheet.Row{Index: i}
		for c, f := range rec {
			if c >= sheet.MaxCols {
				break
			}
			if c < maxSizedCols {
				for len(widths) <= c {
					widths = append(widths, 0)
				}
				widths[c] = max(widths[c], float64(utf8.RuneCountInString(longestLine(f)))*charWidth+colPadding)
			}
			if f == "" {
				continue
			}
			cell := sheet.Cell{Col: c, Text: f}
			if isNumber(f) {
				cell.Kind = sheet.KindNumber
			}
			row.Cells = append(row.Cells, cell)
		}
		sh.Rows = append(sh.Rows, row)
	}
	wrap := make(map[int]bool)
	for c, w := range widths {
		sh.Cols = append(sh.Cols, sheet.Col{Min: c, Max: c, Width: min(maxColWidth, max(minColWidth, w))})
		if w > maxColWidth {
			// a long field breaks into lines
			wrap[c] = true
		}
	}
	for i := range sh.Rows {
		for k := range sh.Rows[i].Cells {
			c := &sh.Rows[i].Cells[k]
			// a field with a line break breaks into lines too
			if wrap[c.Col] || strings.ContainsAny(c.Text, "\r\n") {
				c.Style = 1
			}
		}
	}
	if d.truncated {
		sh.Note = fmt.Sprintf("The file holds more than %d fields, which ivy does not show.", d.lim.MaxObjects)
	}
	wb.Sheets = []*sheet.Sheet{sh}
	return wb
}

// longestLine returns the longest line of s.
func longestLine(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return s
	}
	longest := ""
	for line := range strings.SplitSeq(strings.ReplaceAll(s, "\r", "\n"), "\n") {
		if len(line) > len(longest) {
			longest = line
		}
	}
	return longest
}

// isNumber reports whether a field holds a number, such as "-1.5",
// "1,234.50", "12%" or "1e3".
func isNumber(s string) bool {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	if s == "" || len(s) > 64 {
		return false
	}
	if strings.Contains(s, ",") {
		// thousands separators in groups of three
		intPart, _, _ := strings.Cut(s, ".")
		groups := strings.Split(intPart, ",")
		for i, g := range groups {
			if (i == 0 && (len(g) == 0 || len(g) > 3)) || (i > 0 && len(g) != 3) {
				return false
			}
		}
		s = strings.ReplaceAll(s, ",", "")
	}
	if c := s[0]; c < '0' || c > '9' {
		if c != '.' {
			return false
		}
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
