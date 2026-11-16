package fixture

import (
	"fmt"
	"strings"
)

// PPTX describes a PowerPoint package that Bytes builds: a theme, a master
// with a title and a body placeholder, one layout, and the slides.
type PPTX struct {
	// Width and Height are the size of the slides in EMU. Zero is 10 by
	// 7.5 inches.
	Width, Height int
	// MasterShapes is XML that the shape tree of the master holds after its
	// placeholders.
	MasterShapes string
	// MasterBackground is the p:bg of the master, or empty.
	MasterBackground string
	// TableStyles is the content of ppt/tableStyles.xml, or empty for an
	// empty list.
	TableStyles string
	// Slides are the slides, in order.
	Slides []PPTXSlide
	// Media are the files of ppt/media. Each slide has the relationship
	// "rIdM" and the index of the file from 1 to each of them, such as
	// rIdM1.
	Media []Entry
	// Charts are the chart parts of ppt/charts. Each slide has the
	// relationship "rIdC" and the index of the part from 1 to each of
	// them, such as rIdC1.
	Charts []Entry
}

// PPTXSlide is a slide of a PPTX.
type PPTXSlide struct {
	// Shapes is XML that the shape tree of the slide holds.
	Shapes string
	// Background is the p:bg of the slide, or empty.
	Background string
	// Hidden hides the slide.
	Hidden bool
}

// The namespaces of PresentationML and DrawingML.
const pptxNS = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`

// pptxTheme is the theme of a PPTX: the colors and the fonts of the
// default theme of Office.
const pptxTheme = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme ` + pptxNS + ` name="Office Theme"><a:themeElements>
<a:clrScheme name="Office">
<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>
<a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2>
<a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2>
<a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4>
<a:accent5><a:srgbClr val="5B9BD5"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6>
<a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink>
</a:clrScheme>
<a:fontScheme name="Office"><a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>
<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme>
<a:fmtScheme name="Office">
<a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill>
<a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"><a:tint val="50000"/></a:schemeClr></a:gs><a:gs pos="100000"><a:schemeClr val="phClr"/></a:gs></a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>
<a:solidFill><a:schemeClr val="phClr"><a:shade val="50000"/></a:schemeClr></a:solidFill></a:fillStyleLst>
<a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="12700"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="19050"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>
<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>
<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst>
</a:fmtScheme></a:themeElements></a:theme>`

// pptxMaster returns the master of a PPTX, with the shapes and the
// background.
func pptxMaster(p *PPTX) string {
	bg := p.MasterBackground
	if bg == "" {
		bg = `<p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg>`
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster ` + pptxNS + `><p:cSld>` + bg + `<p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>
<p:spPr><a:xfrm><a:off x="457200" y="274638"/><a:ext cx="8229600" cy="1143000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
<p:txBody><a:bodyPr anchor="ctr"/><a:lstStyle/><a:p><a:r><a:t>Title</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
<p:spPr><a:xfrm><a:off x="457200" y="1600200"/><a:ext cx="8229600" cy="4525963"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Body</a:t></a:r></a:p></p:txBody></p:sp>
` + p.MasterShapes + `
</p:spTree></p:cSld>
<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>
<p:txStyles>
<p:titleStyle><a:lvl1pPr algn="ctr"><a:defRPr sz="4400"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mj-lt"/></a:defRPr></a:lvl1pPr></p:titleStyle>
<p:bodyStyle>
<a:lvl1pPr marL="342900" indent="-342900"><a:spcBef><a:spcPct val="20000"/></a:spcBef><a:buFont typeface="Arial"/><a:buChar char="&#8226;"/><a:defRPr sz="3200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl1pPr>
<a:lvl2pPr marL="742950" indent="-285750"><a:spcBef><a:spcPct val="20000"/></a:spcBef><a:buFont typeface="Arial"/><a:buChar char="&#8211;"/><a:defRPr sz="2800"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl2pPr>
</p:bodyStyle>
<p:otherStyle><a:lvl1pPr><a:defRPr sz="1800"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl1pPr></p:otherStyle>
</p:txStyles></p:sldMaster>`
}

// pptxLayout is the layout of a PPTX, with the placeholders of the master.
const pptxLayout = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout ` + pptxNS + ` type="obj"><p:cSld name="Title and Content"><p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/>
<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Title</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="3" name="Content"/><p:cNvSpPr/><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr><p:spPr/>
<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Content</a:t></a:r></a:p></p:txBody></p:sp>
</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`

// rels returns a part of relationships of the targets, which have the IDs
// of the keys.
func rels(list ...[3]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, r := range list {
		fmt.Fprintf(&b, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/%s" Target="%s"/>`, r[0], r[1], r[2])
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

// Bytes returns the package.
func (p *PPTX) Bytes() []byte {
	w, h := p.Width, p.Height
	if w == 0 || h == 0 {
		w, h = 9144000, 6858000
	}
	var types strings.Builder
	types.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Default Extension="png" ContentType="image/png"/>
<Default Extension="jpeg" ContentType="image/jpeg"/>
<Default Extension="emf" ContentType="image/x-emf"/>
<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
<Override PartName="/ppt/tableStyles.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.tableStyles+xml"/>
`)
	var ids, presRels strings.Builder
	presList := make([][3]string, 0, 3+len(p.Slides))
	presList = append(presList,
		[3]string{"rId1", "slideMaster", "slideMasters/slideMaster1.xml"},
		[3]string{"rId2", "theme", "theme/theme1.xml"},
		[3]string{"rId3", "tableStyles", "tableStyles.xml"},
	)
	slides := make([]Entry, 0, 2*len(p.Slides))
	for i, s := range p.Slides {
		n := i + 1
		fmt.Fprintf(&types, `<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, n)
		fmt.Fprintf(&ids, `<p:sldId id="%d" r:id="rIdS%d"/>`, 255+n, n)
		presList = append(presList, [3]string{fmt.Sprintf("rIdS%d", n), "slide", fmt.Sprintf("slides/slide%d.xml", n)})
		show := ""
		if s.Hidden {
			show = ` show="0"`
		}
		xml := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld ` + pptxNS + show + `><p:cSld>` + s.Background + `<p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>
` + s.Shapes + `
</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`
		slideRels := make([][3]string, 0, 1+len(p.Media)+len(p.Charts))
		slideRels = append(slideRels, [3]string{"rId1", "slideLayout", "../slideLayouts/slideLayout1.xml"})
		for j, m := range p.Media {
			slideRels = append(slideRels, [3]string{fmt.Sprintf("rIdM%d", j+1), "image", "../media/" + m.Name})
		}
		for j, c := range p.Charts {
			slideRels = append(slideRels, [3]string{fmt.Sprintf("rIdC%d", j+1), "chart", "../charts/" + c.Name})
		}
		slides = append(slides,
			Entry{Name: fmt.Sprintf("ppt/slides/slide%d.xml", n), Data: []byte(xml)},
			Entry{Name: fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", n), Data: []byte(rels(slideRels...))},
		)
	}
	types.WriteString(`</Types>`)
	presRels.WriteString(rels(presList...))
	pres := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation ` + pptxNS + `><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
<p:sldIdLst>` + ids.String() + `</p:sldIdLst>` +
		fmt.Sprintf(`<p:sldSz cx="%d" cy="%d"/><p:notesSz cx="6858000" cy="9144000"/>`, w, h) +
		`<p:defaultTextStyle><a:lvl1pPr><a:defRPr sz="1800"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl1pPr></p:defaultTextStyle></p:presentation>`
	tableStyles := p.TableStyles
	if tableStyles == "" {
		tableStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"/>`
	}
	entries := make([]Entry, 0, 10+len(slides)+len(p.Media)+len(p.Charts))
	entries = append(entries, []Entry{
		{Name: "[Content_Types].xml", Data: []byte(types.String())},
		{Name: "_rels/.rels", Data: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`)},
		{Name: "ppt/presentation.xml", Data: []byte(pres)},
		{Name: "ppt/_rels/presentation.xml.rels", Data: []byte(presRels.String())},
		{Name: "ppt/theme/theme1.xml", Data: []byte(pptxTheme)},
		{Name: "ppt/tableStyles.xml", Data: []byte(tableStyles)},
		{Name: "ppt/slideMasters/slideMaster1.xml", Data: []byte(pptxMaster(p))},
		{Name: "ppt/slideMasters/_rels/slideMaster1.xml.rels", Data: []byte(rels(
			[3]string{"rId1", "slideLayout", "../slideLayouts/slideLayout1.xml"},
			[3]string{"rId2", "theme", "../theme/theme1.xml"},
		))},
		{Name: "ppt/slideLayouts/slideLayout1.xml", Data: []byte(pptxLayout)},
		{Name: "ppt/slideLayouts/_rels/slideLayout1.xml.rels", Data: []byte(rels(
			[3]string{"rId1", "slideMaster", "../slideMasters/slideMaster1.xml"},
		))},
	}...)
	entries = append(entries, slides...)
	for _, m := range p.Media {
		entries = append(entries, Entry{Name: "ppt/media/" + m.Name, Data: m.Data})
	}
	for _, c := range p.Charts {
		entries = append(entries, Entry{Name: "ppt/charts/" + c.Name, Data: c.Data})
	}
	return Zip(entries...)
}
