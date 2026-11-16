package fixture

import (
	"fmt"
	"html"
	"strings"
)

// XLSXSheet is a sheet of an Excel file that XLSX builds.
type XLSXSheet struct {
	Name string
	// State is "", "hidden" or "veryHidden".
	State string
	// Body holds the children of the element worksheet, such as
	// <sheetData>, in their order.
	Body string
	// Rels holds the Relationship elements of the sheet, such as the one
	// of its drawing.
	Rels string
	// Chart makes the sheet a chart sheet, whose Body holds the children
	// of the element chartsheet.
	Chart bool
}

// XLSXBook is an Excel file that XLSX builds.
type XLSXBook struct {
	Sheets []XLSXSheet
	// Styles holds the children of the element styleSheet, or "" for no
	// styles part.
	Styles string
	// Strings are the shared strings, as plain text.
	Strings []string
	// Workbook holds the children of the element workbook that come
	// before its sheets, such as <workbookPr date1904="1"/>.
	Workbook string
	// Names holds the defined names, the children of definedNames.
	Names string
	// Extra holds more parts, such as drawings and images.
	Extra []Entry
}

// The namespaces of SpreadsheetML.
const (
	nsMain = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	nsRel  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	relPkg = "http://schemas.openxmlformats.org/package/2006/relationships"
)

// XLSX returns the Excel file of b.
func XLSX(b XLSXBook) []byte {
	var sheets, rels strings.Builder
	entries := []Entry{
		{Name: "[Content_Types].xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`)},
		{Name: "_rels/.rels", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="` + relPkg + `">` +
			`<Relationship Id="rId1" Type="` + nsRel + `/officeDocument" Target="xl/workbook.xml"/></Relationships>`)},
	}
	for i, s := range b.Sheets {
		id := fmt.Sprintf("rId%d", i+1)
		fmt.Fprintf(&sheets, `<sheet name="%s" sheetId="%d" r:id="%s"`, html.EscapeString(s.Name), i+1, id)
		if s.State != "" {
			fmt.Fprintf(&sheets, ` state="%s"`, s.State)
		}
		sheets.WriteString("/>")
		kind, folder := "worksheet", "worksheets"
		if s.Chart {
			kind, folder = "chartsheet", "chartsheets"
		}
		fmt.Fprintf(&rels, `<Relationship Id="%s" Type="%s/%s" Target="%s/sheet%d.xml"/>`, id, nsRel, kind, folder, i+1)
		entries = append(entries, Entry{
			Name: fmt.Sprintf("xl/%s/sheet%d.xml", folder, i+1),
			Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><` + kind + ` xmlns="` + nsMain + `" xmlns:r="` + nsRel + `">` + s.Body + `</` + kind + `>`),
		})
		if s.Rels != "" {
			entries = append(entries, Entry{
				Name: fmt.Sprintf("xl/%s/_rels/sheet%d.xml.rels", folder, i+1),
				Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="` + relPkg + `">` + s.Rels + `</Relationships>`),
			})
		}
	}
	if b.Styles != "" {
		rels.WriteString(`<Relationship Id="rIdS" Type="` + nsRel + `/styles" Target="styles.xml"/>`)
		entries = append(entries, Entry{Name: "xl/styles.xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="` + nsMain + `">` + b.Styles + `</styleSheet>`)})
	}
	if len(b.Strings) > 0 {
		rels.WriteString(`<Relationship Id="rIdT" Type="` + nsRel + `/sharedStrings" Target="sharedStrings.xml"/>`)
		var sst strings.Builder
		for _, s := range b.Strings {
			sst.WriteString(`<si><t xml:space="preserve">` + html.EscapeString(s) + `</t></si>`)
		}
		entries = append(entries, Entry{Name: "xl/sharedStrings.xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><sst xmlns="` + nsMain + `">` + sst.String() + `</sst>`)})
	}
	names := ""
	if b.Names != "" {
		names = "<definedNames>" + b.Names + "</definedNames>"
	}
	entries = append(entries,
		Entry{Name: "xl/workbook.xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="` + nsMain + `" xmlns:r="` + nsRel + `">` +
			b.Workbook + "<sheets>" + sheets.String() + "</sheets>" + names + "</workbook>")},
		Entry{Name: "xl/_rels/workbook.xml.rels", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="` + relPkg + `">` + rels.String() + `</Relationships>`)},
	)
	return Zip(append(entries, b.Extra...)...)
}

// odfNamespaces declares the namespaces of OpenDocument.
const odfNamespaces = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" ` +
	`xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" ` +
	`xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" ` +
	`xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" ` +
	`xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" ` +
	`xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" ` +
	`xmlns:xlink="http://www.w3.org/1999/xlink" ` +
	`xmlns:number="urn:oasis:names:tc:opendocument:xmlns:datastyle:1.0" ` +
	`xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" office:version="1.3"`

// ODSBook is an OpenDocument spreadsheet that ODS builds.
type ODSBook struct {
	// Styles holds the children of office:styles of styles.xml, such as
	// the default style of the cells.
	Styles string
	// AutoStyles holds the children of office:automatic-styles of
	// styles.xml, such as the page layouts.
	AutoStyles string
	// MasterStyles holds the children of office:master-styles.
	MasterStyles string
	// ContentStyles holds the children of office:automatic-styles of
	// content.xml, such as the styles of the columns, the rows and the
	// cells.
	ContentStyles string
	// Body holds the children of office:spreadsheet: the tables.
	Body string
	// Extra holds more entries, such as images.
	Extra []Entry
}

// ODS returns the OpenDocument spreadsheet of b.
func ODS(b ODSBook) []byte {
	const mime = "application/vnd.oasis.opendocument.spreadsheet"
	content := `<?xml version="1.0" encoding="UTF-8"?><office:document-content ` + odfNamespaces + `>` +
		`<office:automatic-styles>` + b.ContentStyles + `</office:automatic-styles>` +
		`<office:body><office:spreadsheet>` + b.Body + `</office:spreadsheet></office:body></office:document-content>`
	styles := `<?xml version="1.0" encoding="UTF-8"?><office:document-styles ` + odfNamespaces + `>` +
		`<office:styles>` + b.Styles + `</office:styles>` +
		`<office:automatic-styles>` + b.AutoStyles + `</office:automatic-styles>` +
		`<office:master-styles>` + b.MasterStyles + `</office:master-styles></office:document-styles>`
	entries := make([]Entry, 0, 4+len(b.Extra))
	entries = append(entries,
		Entry{Name: "mimetype", Data: []byte(mime), Store: true},
		Entry{Name: "META-INF/manifest.xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.3">` +
			`<manifest:file-entry manifest:full-path="/" manifest:media-type="` + mime + `"/>` +
			`<manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/>` +
			`<manifest:file-entry manifest:full-path="styles.xml" manifest:media-type="text/xml"/></manifest:manifest>`)},
		Entry{Name: "content.xml", Data: []byte(content)},
		Entry{Name: "styles.xml", Data: []byte(styles)},
	)
	return Zip(append(entries, b.Extra...)...)
}
