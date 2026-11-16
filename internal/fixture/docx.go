package fixture

import (
	"fmt"
	"strings"
)

// WordNamespaces declares the prefixes of the namespaces of
// WordprocessingML and DrawingML that the parts of a Word file use.
const WordNamespaces = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
	`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" ` +
	`xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" ` +
	`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
	`xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" ` +
	`xmlns:v="urn:schemas-microsoft-com:vml" ` +
	`xmlns:o="urn:schemas-microsoft-com:office:office"`

// WordPart is a part of a Word file that the main part relates to.
type WordPart struct {
	// ID is the id of the relationship, such as "rId5".
	ID string
	// Type is the last name of the type of the relationship, such as
	// "styles", "numbering", "header" or "image".
	Type string
	// Name is the name of the part, such as "word/styles.xml".
	Name string
	Data []byte
	// External makes the relationship external, such as a hyperlink. Name
	// is then its target.
	External bool
}

// DOCX returns a Word file whose body holds the XML body, such as
// `<w:p><w:r><w:t>Text</w:t></w:r></w:p>`, with the parts. The main part
// declares the prefixes of WordNamespaces.
func DOCX(body string, parts ...WordPart) []byte {
	var rels strings.Builder
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	entries := []Entry{
		{Name: "[Content_Types].xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)},
		{Name: "_rels/.rels", Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)},
		{Name: "word/document.xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document ` + WordNamespaces + `><w:body>` + body + `</w:body></w:document>`)},
	}
	for _, p := range parts {
		target, mode := strings.TrimPrefix(p.Name, "word/"), ""
		if p.External {
			target, mode = p.Name, ` TargetMode="External"`
		}
		typ := "http://schemas.openxmlformats.org/officeDocument/2006/relationships/" + p.Type
		fmt.Fprintf(&rels, `<Relationship Id="%s" Type="%s" Target="%s"%s/>`, p.ID, typ, xmlAttr(target), mode)
		if !p.External {
			entries = append(entries, Entry{Name: p.Name, Data: p.Data})
		}
	}
	rels.WriteString(`</Relationships>`)
	entries = append(entries, Entry{Name: "word/_rels/document.xml.rels", Data: []byte(rels.String())})
	return Zip(entries...)
}

// WordXML returns a part of a Word file of the root element root, such as
// "w:styles", with the content and the prefixes of WordNamespaces.
func WordXML(root, content string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><` + root + ` ` + WordNamespaces + `>` + content + `</` + root + `>`)
}

// xmlAttr escapes s for an attribute of XML.
func xmlAttr(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", `"`, "&quot;").Replace(s)
}
