package fixture

import (
	"archive/zip"
	"bytes"
	"fmt"
	"hash/crc32"
	"io"
	"strings"
)

// ODP describes an OpenDocument presentation or drawing that Bytes builds:
// one master page with a page layout, and the pages.
type ODP struct {
	// Mime is the mime type. Empty is a presentation.
	Mime string
	// Width and Height are the size of the pages, such as "28cm". Empty is
	// 28 by 15.75 cm.
	Width, Height string
	// Styles is XML that office:styles of styles.xml holds, such as
	// draw:gradient and the common styles.
	Styles string
	// MasterStyle is the style:drawing-page-properties of the master page,
	// such as its background, or empty.
	MasterStyle string
	// MasterShapes is XML that the master page holds.
	MasterShapes string
	// AutomaticStyles is XML that office:automatic-styles of content.xml
	// holds.
	AutomaticStyles string
	// Pages holds the XML of each draw:page: its attributes, then ">", then
	// its shapes, such as `draw:style-name="dp1"><draw:rect .../>`. A page
	// without attributes starts with ">".
	Pages []string
	// Entries are more entries of the package, such as pictures in
	// Pictures/.
	Entries []Entry
}

// The namespaces of OpenDocument.
const odpNS = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" ` +
	`xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" ` +
	`xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" ` +
	`xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" ` +
	`xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" ` +
	`xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" ` +
	`xmlns:xlink="http://www.w3.org/1999/xlink" ` +
	`xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" ` +
	`xmlns:presentation="urn:oasis:names:tc:opendocument:xmlns:presentation:1.0" ` +
	`xmlns:loext="urn:org:documentfoundation:names:experimental:office:xmlns:loext:1.0" ` +
	`office:version="1.3"`

// Parts returns the entries of the package without the mimetype and the
// manifest, for a test that encrypts them with EncryptODF.
func (o *ODP) Parts() []Entry {
	w, h := o.Width, o.Height
	if w == "" || h == "" {
		w, h = "28cm", "15.75cm"
	}
	styles := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles ` + odpNS + `>
<office:font-face-decls><style:font-face style:name="Liberation Sans" svg:font-family="'Liberation Sans'"/></office:font-face-decls>
<office:styles>
<style:default-style style:family="graphic"><style:graphic-properties svg:stroke-color="#3465a4" draw:fill-color="#729fcf"/>
<style:text-properties style:font-name="Liberation Sans" fo:font-size="18pt"/></style:default-style>
<style:style style:name="standard" style:family="graphic"><style:graphic-properties draw:stroke="solid" svg:stroke-width="0cm" svg:stroke-color="#3465a4" draw:fill="solid" draw:fill-color="#729fcf" fo:padding-top="0.125cm" fo:padding-bottom="0.125cm" fo:padding-left="0.25cm" fo:padding-right="0.25cm"/>
<style:text-properties style:font-name="Liberation Sans" fo:font-size="18pt" fo:color="#000000"/></style:style>
` + o.Styles + `
</office:styles>
<office:automatic-styles>
<style:page-layout style:name="PM1"><style:page-layout-properties fo:margin-top="0cm" fo:margin-bottom="0cm" fo:margin-left="0cm" fo:margin-right="0cm" fo:page-width="` + w + `" fo:page-height="` + h + `"/></style:page-layout>
<style:style style:name="Mdp1" style:family="drawing-page"><style:drawing-page-properties ` + o.MasterStyle + `/></style:style>
</office:automatic-styles>
<office:master-styles><style:master-page style:name="Default" style:page-layout-name="PM1" draw:style-name="Mdp1">` + o.MasterShapes + `</style:master-page></office:master-styles>
</office:document-styles>`
	var body strings.Builder
	kind := "presentation"
	if strings.HasSuffix(o.mime(), "graphics") {
		kind = "drawing"
	}
	for i, p := range o.Pages {
		attrs, shapes, _ := strings.Cut(p, ">")
		fmt.Fprintf(&body, `<draw:page draw:name="page%d" draw:master-page-name="Default" %s>%s</draw:page>`, i+1, attrs, shapes)
	}
	content := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content ` + odpNS + `>
<office:automatic-styles>` + o.AutomaticStyles + `</office:automatic-styles>
<office:body><office:` + kind + `>` + body.String() + `</office:` + kind + `></office:body></office:document-content>`
	entries := make([]Entry, 0, 2+len(o.Entries))
	entries = append(entries,
		Entry{Name: "content.xml", Data: []byte(content)},
		Entry{Name: "styles.xml", Data: []byte(styles)},
	)
	return append(entries, o.Entries...)
}

// mime returns the mime type of the package.
func (o *ODP) mime() string {
	if o.Mime == "" {
		return "application/vnd.oasis.opendocument.presentation"
	}
	return o.Mime
}

// Bytes returns the package.
func (o *ODP) Bytes() []byte {
	var manifest strings.Builder
	manifest.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.3">
<manifest:file-entry manifest:full-path="/" manifest:media-type="` + o.mime() + `"/>
<manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/>
<manifest:file-entry manifest:full-path="styles.xml" manifest:media-type="text/xml"/>
`)
	for _, e := range o.Entries {
		fmt.Fprintf(&manifest, `<manifest:file-entry manifest:full-path="%s" manifest:media-type=""/>`, e.Name)
	}
	manifest.WriteString(`</manifest:manifest>`)
	parts := o.Parts()
	entries := make([]Entry, 0, 2+len(parts))
	entries = append(entries,
		Entry{Name: "mimetype", Data: []byte(o.mime()), Store: true},
		Entry{Name: "META-INF/manifest.xml", Data: []byte(manifest.String())},
	)
	return odfZip(append(entries, parts...)...)
}

// odfZip returns a zip archive of the entries, as OpenDocument needs it:
// an entry that is stored has its sizes in its header, without a data
// descriptor, so that LibreOffice reads the mimetype.
func odfZip(entries ...Entry) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		var w io.Writer
		var err error
		if e.Store {
			w, err = zw.CreateRaw(&zip.FileHeader{
				Name:               e.Name,
				Method:             zip.Store,
				CRC32:              crc32.ChecksumIEEE(e.Data),
				CompressedSize64:   uint64(len(e.Data)),
				UncompressedSize64: uint64(len(e.Data)),
			})
		} else {
			w, err = zw.CreateHeader(&zip.FileHeader{Name: e.Name, Method: zip.Deflate})
		}
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(e.Data); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
