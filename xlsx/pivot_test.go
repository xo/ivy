package xlsx

import (
	"regexp"
	"slices"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// TestPivotBold checks that the header rows and the row of the grand totals
// of a pivot table with a style are bold, as Excel prints the pivot tables
// of poi/xlsx/pivot-table.xlsx, and that the other cells keep their style.
func TestPivotBold(t *testing.T) {
	const pivot = `<?xml version="1.0" encoding="UTF-8"?>` +
		`<pivotTableDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" name="P" cacheId="0">` +
		`<location ref="A1:B4" firstHeaderRow="1" firstDataRow="1" firstDataCol="1"/>` +
		`<rowItems count="3"><i><x/></i><i><x v="1"/></i><i t="grand"><x/></i></rowItems>` +
		`<pivotTableStyleInfo name="PivotStyleLight16" showRowHeaders="1" showColHeaders="1"/></pivotTableDefinition>`
	cell := func(ref, text string) string {
		return `<c r="` + ref + `" t="inlineStr"><is><t>` + text + `</t></is></c>`
	}
	book := fixture.XLSXBook{
		Sheets: []fixture.XLSXSheet{{
			Name: "Pivot",
			Body: `<sheetData>` +
				`<row r="1">` + cell("A1", "Labels") + cell("B1", "Sum") + `</row>` +
				`<row r="2">` + cell("A2", "x") + `<c r="B2"><v>1</v></c></row>` +
				`<row r="3">` + cell("A3", "y") + `<c r="B3"><v>2</v></c></row>` +
				`<row r="4">` + cell("A4", "Total") + `<c r="B4"><v>3</v></c></row>` +
				`<row r="5">` + cell("A5", "after") + `</row>` +
				`</sheetData>`,
			Rels: `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotTable" Target="../pivotTables/pivotTable1.xml"/>`,
		}},
		Extra: []fixture.Entry{{Name: "xl/pivotTables/pivotTable1.xml", Data: []byte(pivot)}},
	}
	d := open(t, fixture.XLSX(book), ivy.Config{})
	_, docs := sourceOf(t, d)
	doc := docs[0]
	// the class of the bold style, which the workbook adds after its own
	m := regexp.MustCompile(`td\.(s\d+)\{[^}]*font-weight:bold`).FindStringSubmatch(doc)
	if m == nil {
		t.Fatalf("expected a bold style in the sheet:\n%s", doc)
	}
	bold := regexp.MustCompile(`<td class="` + m[1] + `[" ][^>]*>([^<]*)<`)
	cells := bold.FindAllStringSubmatch(doc, -1)
	got := make([]string, 0, len(cells))
	for _, c := range cells {
		got = append(got, c[1])
	}
	if exp := []string{"Labels", "Sum", "Total", "3"}; !slices.Equal(got, exp) {
		t.Errorf("expected the bold cells %q, got: %q", exp, got)
	}
}
