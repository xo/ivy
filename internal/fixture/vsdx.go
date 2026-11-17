package fixture

import (
	"fmt"
	"strings"
)

// VSDX describes a Visio drawing of Office Open XML that Bytes builds: the
// styles of the document, its masters, its pages, and a theme.
type VSDX struct {
	// Styles is XML of StyleSheet elements, which the element StyleSheets
	// of visio/document.xml holds.
	Styles string
	// Colors is XML of ColorEntry elements, the color table of the
	// document.
	Colors string
	// Theme is the content of visio/theme/theme1.xml, or empty for a
	// drawing without a theme.
	Theme string
	// Masters are the masters, and Pages the pages, in order.
	Masters []VSDXMaster
	Pages   []VSDXPage
	// Media holds the files of visio/media, by their names. Each page and
	// each master has a relationship to each of them, with the ID
	// "media:" and the name.
	Media map[string][]byte
}

// VSDXMaster is a master of a VSDX.
type VSDXMaster struct {
	ID int
	// Shapes is XML of the Shape elements of the master.
	Shapes string
}

// VSDXPage is a page of a VSDX.
type VSDXPage struct {
	ID int
	// Width and Height are the size of the page in inches. Zero is 8.5 by
	// 11 inches.
	Width, Height float64
	// Background marks a background page, and BackPage is the ID of the
	// background page of a page, or 0 for none.
	Background bool
	BackPage   int
	// Sheet is XML of the Cell and Section elements of the PageSheet,
	// after the size of the page.
	Sheet string
	// Shapes is XML of the Shape elements of the page.
	Shapes string
}

// visioNS declares the namespaces of the parts of Visio.
const visioNS = `xmlns="http://schemas.microsoft.com/office/visio/2012/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xml:space="preserve"`

// visioRels returns a part of relationships of Visio. Each element of list
// is the ID, the type and the target of a relationship. A type without a
// slash is a type of Visio 2010.
func visioRels(list ...[3]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, r := range list {
		typ := r[1]
		if !strings.Contains(typ, "/") {
			typ = "http://schemas.microsoft.com/visio/2010/relationships/" + typ
		}
		fmt.Fprintf(&b, `<Relationship Id="%s" Type="%s" Target="%s"/>`, r[0], typ, r[2])
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

// mediaRels returns the relationships of a part to the files of v.Media.
func (v *VSDX) mediaRels() [][3]string {
	list := make([][3]string, 0, len(v.Media))
	for name := range v.Media {
		list = append(list, [3]string{"media:" + name, "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image", "../media/" + name})
	}
	return list
}

// Bytes returns the package.
func (v *VSDX) Bytes() []byte {
	entries := []Entry{
		{Name: "[Content_Types].xml", Data: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/visio/document.xml" ContentType="application/vnd.ms-visio.drawing.main+xml"/>
</Types>`)},
		{Name: "_rels/.rels", Data: []byte(visioRels([3]string{"rId1", "document", "visio/document.xml"}))},
	}
	docRels := [][3]string{{"rId1", "pages", "pages/pages.xml"}, {"rId2", "masters", "masters/masters.xml"}}
	if v.Theme != "" {
		docRels = append(docRels, [3]string{"rId3", "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme", "theme/theme1.xml"})
		entries = append(entries, Entry{Name: "visio/theme/theme1.xml", Data: []byte(v.Theme)})
	}
	doc := `<?xml version="1.0" encoding="UTF-8"?><VisioDocument ` + visioNS + `>` +
		`<DocumentSettings DefaultLineStyle="0" DefaultFillStyle="0" DefaultTextStyle="0"/>` +
		`<Colors>` + v.Colors + `</Colors>` +
		`<StyleSheets><StyleSheet ID="0" NameU="No Style">` +
		`<Cell N="LineWeight" V="0.01041666666666667"/><Cell N="LineColor" V="0"/><Cell N="LinePattern" V="1"/>` +
		`<Cell N="FillForegnd" V="1"/><Cell N="FillBkgnd" V="0"/><Cell N="FillPattern" V="1"/>` +
		`<Cell N="LeftMargin" V="0"/><Cell N="RightMargin" V="0"/><Cell N="TopMargin" V="0"/><Cell N="BottomMargin" V="0"/>` +
		`<Cell N="VerticalAlign" V="1"/>` +
		`<Section N="Character"><Row IX="0"><Cell N="Font" V="Calibri"/><Cell N="Color" V="0"/><Cell N="Style" V="0"/><Cell N="Size" V="0.1666666666666667"/></Row></Section>` +
		`<Section N="Paragraph"><Row IX="0"><Cell N="HorzAlign" V="1"/><Cell N="SpLine" V="-1.2"/></Row></Section>` +
		`</StyleSheet>` + v.Styles + `</StyleSheets></VisioDocument>`
	entries = append(entries,
		Entry{Name: "visio/document.xml", Data: []byte(doc)},
		Entry{Name: "visio/_rels/document.xml.rels", Data: []byte(visioRels(docRels...))},
	)
	var masters strings.Builder
	masterRels := make([][3]string, 0, len(v.Masters))
	masters.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Masters ` + visioNS + `>`)
	for i, m := range v.Masters {
		name := fmt.Sprintf("master%d.xml", i+1)
		id := fmt.Sprintf("rId%d", i+1)
		fmt.Fprintf(&masters, `<Master ID="%d" NameU="Master %d"><Rel r:id="%s"/></Master>`, m.ID, m.ID, id)
		masterRels = append(masterRels, [3]string{id, "master", name})
		entries = append(entries,
			Entry{Name: "visio/masters/" + name, Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><MasterContents ` + visioNS + `><Shapes>` + m.Shapes + `</Shapes></MasterContents>`)},
			Entry{Name: "visio/masters/_rels/" + name + ".rels", Data: []byte(visioRels(v.mediaRels()...))},
		)
	}
	masters.WriteString(`</Masters>`)
	entries = append(entries,
		Entry{Name: "visio/masters/masters.xml", Data: []byte(masters.String())},
		Entry{Name: "visio/masters/_rels/masters.xml.rels", Data: []byte(visioRels(masterRels...))},
	)
	var pages strings.Builder
	pageRels := make([][3]string, 0, len(v.Pages))
	pages.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Pages ` + visioNS + `>`)
	for i, p := range v.Pages {
		name := fmt.Sprintf("page%d.xml", i+1)
		id := fmt.Sprintf("rId%d", i+1)
		w, h := p.Width, p.Height
		if w == 0 || h == 0 {
			w, h = 8.5, 11
		}
		fmt.Fprintf(&pages, `<Page ID="%d" NameU="Page-%d"`, p.ID, p.ID)
		if p.Background {
			pages.WriteString(` Background="1"`)
		}
		if p.BackPage != 0 {
			fmt.Fprintf(&pages, ` BackPage="%d"`, p.BackPage)
		}
		fmt.Fprintf(&pages, `><PageSheet LineStyle="0" FillStyle="0" TextStyle="0"><Cell N="PageWidth" V="%g"/><Cell N="PageHeight" V="%g"/>%s</PageSheet><Rel r:id="%s"/></Page>`,
			w, h, p.Sheet, id)
		pageRels = append(pageRels, [3]string{id, "page", name})
		entries = append(entries,
			Entry{Name: "visio/pages/" + name, Data: []byte(`<?xml version="1.0" encoding="UTF-8"?><PageContents ` + visioNS + `><Shapes>` + p.Shapes + `</Shapes></PageContents>`)},
			Entry{Name: "visio/pages/_rels/" + name + ".rels", Data: []byte(visioRels(v.mediaRels()...))},
		)
	}
	pages.WriteString(`</Pages>`)
	entries = append(entries,
		Entry{Name: "visio/pages/pages.xml", Data: []byte(pages.String())},
		Entry{Name: "visio/pages/_rels/pages.xml.rels", Data: []byte(visioRels(pageRels...))},
	)
	for name, data := range v.Media {
		entries = append(entries, Entry{Name: "visio/media/" + name, Data: data})
	}
	return Zip(entries...)
}

// VisioCells returns XML of Cell elements, from pairs of names and values,
// such as VisioCells("PinX", "1", "PinY", "2").
func VisioCells(pairs ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		fmt.Fprintf(&b, `<Cell N="%s" V="%s"/>`, pairs[i], pairs[i+1])
	}
	return b.String()
}

// VisioRect returns XML of a Geometry section with the index ix that draws
// the rectangle of the box of a shape of the size w by h.
func VisioRect(ix int, w, h float64) string {
	return fmt.Sprintf(`<Section N="Geometry" IX="%d"><Row T="MoveTo" IX="1"><Cell N="X" V="0"/><Cell N="Y" V="0"/></Row>`+
		`<Row T="LineTo" IX="2"><Cell N="X" V="%g"/><Cell N="Y" V="0"/></Row>`+
		`<Row T="LineTo" IX="3"><Cell N="X" V="%g"/><Cell N="Y" V="%g"/></Row>`+
		`<Row T="LineTo" IX="4"><Cell N="X" V="0"/><Cell N="Y" V="%g"/></Row>`+
		`<Row T="LineTo" IX="5"><Cell N="X" V="0"/><Cell N="Y" V="0"/></Row></Section>`, ix, w, w, h, h)
}
