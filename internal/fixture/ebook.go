package fixture

import (
	"encoding/base64"
	"strings"
)

// EPUB returns an EPUB file whose package document is OEBPS/content.opf,
// with the manifest items and the spine items of the package pkg, and the
// files of entries in the folder OEBPS. The metadata names the title
// "Fixture".
func EPUB(pkg EPUBPackage, entries ...Entry) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:identifier id="id">fixture</dc:identifier>
<dc:title>Fixture</dc:title>
<dc:language>en</dc:language>
`)
	b.WriteString(pkg.Metadata)
	b.WriteString("</metadata>\n<manifest>\n")
	b.WriteString(pkg.Manifest)
	b.WriteString("</manifest>\n<spine>\n")
	b.WriteString(pkg.Spine)
	b.WriteString("</spine>\n<guide>\n")
	b.WriteString(pkg.Guide)
	b.WriteString("</guide>\n</package>\n")
	all := make([]Entry, 0, 3+len(entries))
	all = append(all,
		Entry{Name: "mimetype", Data: []byte("application/epub+zip"), Store: true},
		Entry{Name: "META-INF/container.xml", Data: []byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`)},
		Entry{Name: "OEBPS/content.opf", Data: []byte(b.String())},
	)
	for _, e := range entries {
		e.Name = "OEBPS/" + e.Name
		all = append(all, e)
	}
	return Zip(all...)
}

// EPUBPackage is the content of the package document of EPUB, as XML.
type EPUBPackage struct {
	// Metadata are more elements of the metadata, such as a meta element of
	// the cover.
	Metadata string
	// Manifest holds the item elements, and Spine the itemref elements.
	Manifest, Spine string
	// Guide holds the reference elements of EPUB 2.
	Guide string
}

// XHTML returns a document of XHTML with the body body.
func XHTML(body string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Fixture</title></head><body>` + body + `</body></html>`)
}

// FictionBook returns a FictionBook file in UTF-8. attrs are more attributes
// of the root element, such as the declarations of the namespace prefixes.
// titleInfo holds the elements of the title-info, and body the elements of
// the body. The binaries follow the body.
func FictionBook(attrs, titleInfo, body string, binaries ...Binary) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"`)
	if attrs != "" {
		b.WriteString(" " + attrs)
	}
	b.WriteString(">\n<description><title-info>\n<book-title>Fixture</book-title>\n")
	b.WriteString(titleInfo)
	b.WriteString("\n</title-info></description>\n<body>\n")
	b.WriteString(body)
	b.WriteString("\n</body>\n")
	for _, bin := range binaries {
		b.WriteString(`<binary id="` + bin.ID + `" content-type="` + bin.ContentType + `">`)
		b.WriteString(base64.StdEncoding.EncodeToString(bin.Data))
		b.WriteString("</binary>\n")
	}
	b.WriteString("</FictionBook>\n")
	return []byte(b.String())
}

// Binary is a binary element of a FictionBook file.
type Binary struct {
	ID, ContentType string
	Data            []byte
}
