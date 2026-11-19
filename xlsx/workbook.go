package xlsx

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/ooxml"
	"github.com/xo/ivy/internal/sheet"
)

// book is the state of the reader of a workbook.
type book struct {
	ctx  context.Context
	doc  *ooxml.Document
	cfg  ivy.Config
	lim  ivy.Limits
	main string
	// rels are the relationships of the workbook part, by their ID, and
	// partRels those of the other parts that the book read.
	rels     map[string]ooxml.Relationship
	partRels map[string]map[string]ooxml.Relationship
	// date1904 selects the date system of 1904.
	date1904 bool
	theme    *drawingml.Theme
	styles   *styles
	strings  []richText
	// names holds the defined names of each sheet, by the index of the
	// sheet in the workbook, for the print areas and the print titles.
	printAreas  map[int]string
	printTitles map[int]string
	// names holds the other defined names, and tables the tables of the
	// sheets that the book read, by their names in lower case. formulas
	// holds the values of the formulas of the thresholds of the
	// conditional formats, by the index of their sheet.
	names    map[nameKey]string
	tables   map[string]table
	formulas map[nameKey]formulaValue
	// files and chartObjects collect the images, and the charts and the
	// shapes, and objects counts them. chartParts holds the part of each
	// chart of chartObjects, by its name.
	files        map[string][]byte
	chartObjects map[string]layout.Object
	chartParts   map[string]string
	objects      int
	// cells counts the cells of the workbook, within lim.MaxObjects.
	cells int
	// sheets holds the sheets that the book read, by their names, for the
	// number formats of the values of the charts.
	sheets map[string]*sheet.Sheet
	// rows gives the heights of the rows that Excel prints, for the
	// default font of the workbook.
	rows *sheet.ExcelRows
}

// excelRows returns the heights of the rows of the workbook, whose default
// font is def, with the fonts of the program.
func (b *book) excelRows(def sheet.Font) *sheet.ExcelRows {
	if b.rows == nil {
		b.rows = sheet.NewExcelRows(fontdb.Of(b.cfg), def)
	}
	return b.rows
}

// richText is a shared string: plain text, or runs of rich text.
type richText struct {
	text string
	runs []sheet.Run
}

// sheetRef is a sheet of the workbook part.
type sheetRef struct {
	name  string
	state string
	part  string
	// chart is set for a chart sheet.
	chart bool
}

// children yields the child elements of n with the local name name.
func children(n roxmltree.Node, name string) func(func(roxmltree.Node) bool) {
	return func(yield func(roxmltree.Node) bool) {
		for c := range n.Children() {
			if c.IsElement() && c.TagName().Name() == name {
				if !yield(c) {
					return
				}
			}
		}
	}
}

// child returns the first child element of n with the local name name.
func child(n roxmltree.Node, name string) (roxmltree.Node, bool) {
	for c := range children(n, name) {
		return c, true
	}
	return roxmltree.Node{}, false
}

// local returns the local name of n.
func local(n roxmltree.Node) string {
	if !n.IsElement() {
		return ""
	}
	return n.TagName().Name()
}

// attr returns the attribute name of n, in any namespace.
func attr(n roxmltree.Node, name string) string {
	return ooxml.Attr(n, name)
}

// flag reads an attribute of the type xsd:boolean, with def when it is
// missing.
func flag(n roxmltree.Node, name string, def bool) bool {
	switch strings.TrimSpace(attr(n, name)) {
	case "1", "true", "on":
		return true
	case "0", "false", "off":
		return false
	}
	return def
}

// number reads a number attribute, with def when it is missing or not
// valid.
func number(n roxmltree.Node, name string, def float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(attr(n, name)), 64)
	if err != nil || v != v {
		return def
	}
	return v
}

// integer reads an integer attribute, with def when it is missing or not
// valid.
func integer(n roxmltree.Node, name string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(attr(n, name)))
	if err != nil {
		return def
	}
	return v
}

// parsePart reads and parses a part of the package.
func (b *book) parsePart(name string) (*roxmltree.Document, error) {
	if err := b.ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	buf, err := b.doc.ReadPart(name)
	if err != nil {
		return nil, err
	}
	doc, err := ooxml.ParseXML(buf, b.lim)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	return doc, nil
}

// relOfType returns the target of the first relationship of the workbook
// of the type name, or "".
func (b *book) relOfType(name string) string {
	for _, r := range b.rels {
		if !r.External && r.TypeIs(name) {
			return r.Target
		}
	}
	return ""
}

// readWorkbook reads the workbook part: its sheets, its date system and
// its defined names.
func (b *book) readWorkbook() ([]sheetRef, error) {
	main, err := b.doc.MainPart(b.lim)
	if err != nil {
		return nil, err
	}
	b.main = main
	rels, err := b.doc.Relationships(main, b.lim)
	if err != nil {
		return nil, err
	}
	b.rels = make(map[string]ooxml.Relationship, len(rels))
	for _, r := range rels {
		b.rels[r.ID] = r
	}
	doc, err := b.parsePart(main)
	if err != nil {
		return nil, err
	}
	root := doc.RootElement()
	if local(root) != "workbook" {
		return nil, fmt.Errorf("main part %s with the root %q: %w", main, local(root), ivy.ErrFormat)
	}
	var refs []sheetRef
	b.printAreas = make(map[int]string)
	b.printTitles = make(map[int]string)
	for n := range root.Children() {
		switch local(n) {
		case "workbookPr":
			b.date1904 = flag(n, "date1904", false)
		case "sheets":
			for s := range children(n, "sheet") {
				ref := sheetRef{name: attr(s, "name"), state: attr(s, "state")}
				if r, ok := b.rels[attr(s, "id")]; ok && !r.External {
					ref.part = r.Target
					ref.chart = r.TypeIs("chartsheet")
				}
				refs = append(refs, ref)
				if len(refs) > b.lim.MaxObjects {
					return nil, fmt.Errorf("workbook of more than %d sheets: %w", b.lim.MaxObjects, ivy.ErrLimit)
				}
			}
		case "definedNames":
			for d := range children(n, "definedName") {
				id := integer(d, "localSheetId", -1)
				text, _ := d.Text()
				switch attr(d, "name") {
				case "_xlnm.Print_Area":
					b.printAreas[id] = text
				case "_xlnm.Print_Titles":
					b.printTitles[id] = text
				default:
					if len(b.names) < b.lim.MaxObjects {
						if b.names == nil {
							b.names = make(map[nameKey]string)
						}
						b.names[nameKey{id, strings.ToLower(attr(d, "name"))}] = text
					}
				}
			}
		}
	}
	return refs, nil
}

// readTheme reads the theme of the workbook, if it has one.
func (b *book) readTheme() {
	part := b.relOfType("theme")
	if part == "" {
		return
	}
	doc, err := b.parsePart(part)
	if err != nil {
		return
	}
	b.theme, _ = drawingml.ParseTheme(doc)
}

// readStrings reads the shared strings.
func (b *book) readStrings() error {
	part := b.relOfType("sharedStrings")
	if part == "" {
		return nil
	}
	doc, err := b.parsePart(part)
	if err != nil {
		return err
	}
	for si := range children(doc.RootElement(), "si") {
		b.strings = append(b.strings, b.richText(si))
	}
	return nil
}

// richText reads the text of an si or an is element: a t element, or runs
// of r elements. The phonetic runs rPh are left out.
func (b *book) richText(n roxmltree.Node) richText {
	var rt richText
	var plain strings.Builder
	rich := false
	for c := range n.Children() {
		switch local(c) {
		case "t":
			s := nodeText(c)
			plain.WriteString(s)
			rt.runs = append(rt.runs, sheet.Run{Text: s})
		case "r":
			var run sheet.Run
			for rc := range c.Children() {
				switch local(rc) {
				case "t":
					run.Text += nodeText(rc)
				case "rPr":
					f := b.styles.font(rc)
					run.Font = &f
					rich = true
				}
			}
			plain.WriteString(run.Text)
			rt.runs = append(rt.runs, run)
		}
	}
	rt.text = plain.String()
	if !rich {
		rt.runs = nil
	}
	return rt
}

// nodeText returns the text of the element n and of its descendants, as
// the file has it.
func nodeText(n roxmltree.Node) string {
	var b strings.Builder
	for d := range n.Descendants() {
		if d.IsText() {
			s, _ := d.Text()
			b.WriteString(s)
		}
	}
	return decodeEscapes(b.String())
}

// decodeEscapes decodes the escapes _xHHHH_ of ECMA-376, which write a
// character that XML cannot hold, such as a control character.
func decodeEscapes(s string) string {
	if !strings.Contains(s, "_x") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '_' && i+6 < len(s) && s[i+1] == 'x' && s[i+6] == '_' {
			if v, err := strconv.ParseUint(s[i+2:i+6], 16, 16); err == nil {
				switch {
				case v == 0x0a || v == 0x09:
					b.WriteRune(rune(v))
				case v >= 0x20:
					b.WriteRune(rune(v))
				}
				i += 6
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// parseRef parses a reference to a cell, such as "B12" or "$B$12", into its
// row and its column, from 0.
func parseRef(s string) (row, col int, ok bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), "$", "")
	i := 0
	col = 0
	for i < len(s) && i < 4 {
		c := s[i] | 0x20
		if c < 'a' || c > 'z' {
			break
		}
		col = col*26 + int(c-'a'+1)
		i++
	}
	if i == 0 || i == len(s) {
		return 0, 0, false
	}
	row, err := strconv.Atoi(s[i:])
	if err != nil || row < 1 || row > sheet.MaxRows || col > sheet.MaxCols {
		return 0, 0, false
	}
	return row - 1, col - 1, true
}

// parseCol parses the letters of a column, such as "B" or "$B", into its
// index, from 0.
func parseCol(s string) (int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "$")
	if s == "" || len(s) > 3 {
		return 0, false
	}
	col := 0
	for i := 0; i < len(s); i++ {
		c := s[i] | 0x20
		if c < 'a' || c > 'z' {
			return 0, false
		}
		col = col*26 + int(c-'a'+1)
	}
	if col > sheet.MaxCols {
		return 0, false
	}
	return col - 1, true
}

// parseRowNum parses the number of a row, such as "12" or "$12", into its
// index, from 0.
func parseRowNum(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(s), "$"))
	if err != nil || n < 1 || n > sheet.MaxRows {
		return 0, false
	}
	return n - 1, true
}

// parseRange parses a range, such as "A1:C3", "A1", "A:C" or "1:3". A
// range of whole columns or rows spans the whole sheet the other way.
func parseRange(s string) (sheet.Range, bool) {
	a, z, found := strings.Cut(strings.TrimSpace(s), ":")
	if !found {
		z = a
	}
	if r0, c0, ok := parseRef(a); ok {
		r1, c1, ok := parseRef(z)
		if !ok {
			return sheet.Range{}, false
		}
		return sheet.Range{Row0: min(r0, r1), Col0: min(c0, c1), Row1: max(r0, r1), Col1: max(c0, c1)}, true
	}
	if c0, ok := parseCol(a); ok {
		c1, ok := parseCol(z)
		if !ok {
			return sheet.Range{}, false
		}
		return sheet.Range{Row0: 0, Col0: min(c0, c1), Row1: sheet.MaxRows - 1, Col1: max(c0, c1)}, true
	}
	if r0, ok := parseRowNum(a); ok {
		r1, ok := parseRowNum(z)
		if !ok {
			return sheet.Range{}, false
		}
		return sheet.Range{Row0: min(r0, r1), Col0: 0, Row1: max(r0, r1), Col1: sheet.MaxCols - 1}, true
	}
	return sheet.Range{}, false
}

// splitNames splits the text of a defined name at each comma outside
// quotes, and drops the name of the sheet of each part, such as
// 'My Sheet'!$A$1:$B$2.
func splitNames(s string) []string {
	var parts []string
	quoted := false
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			quoted = !quoted
		case ',':
			if !quoted {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	for i, p := range parts {
		if j := strings.LastIndexByte(p, '!'); j >= 0 {
			p = p[j+1:]
		}
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// printSetup sets the print areas and the print titles of the sheet of
// the index i of the workbook.
func (b *book) printSetup(sh *sheet.Sheet, i int) {
	if text, ok := b.printAreas[i]; ok {
		for _, p := range splitNames(text) {
			if r, ok := parseRange(p); ok {
				sh.PrintAreas = append(sh.PrintAreas, r)
			}
		}
	}
	if text, ok := b.printTitles[i]; ok {
		for _, p := range splitNames(text) {
			r, ok := parseRange(p)
			if !ok {
				continue
			}
			switch {
			case r.Col0 == 0 && r.Col1 == sheet.MaxCols-1:
				sh.TitleRows = &sheet.Span{First: r.Row0, Last: r.Row1}
			case r.Row0 == 0 && r.Row1 == sheet.MaxRows-1:
				sh.TitleCols = &sheet.Span{First: r.Col0, Last: r.Col1}
			}
		}
	}
}
