package msppt

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/zipdoc"
)

// This file reads the margins of the paragraphs from the metroBlob of a
// shape. PowerPoint 2007 and later write the shape in DrawingML into the
// property metroBlob of MS-ODRAW, as a zip archive. The StyleTextPropAtom
// of the same shape can leave out the left margins and the indents of its
// paragraphs, and PowerPoint and LibreOffice then draw the margins of the
// metroBlob. Pages 38, 42 and 48 of custom-geometry.ppt of Apache POI show
// it.

// propMetroBlob is the id of the property metroBlob of a shape.
const propMetroBlob = 0x03A9

// metroShapeXML is the entry of a metroBlob that holds the shape.
const metroShapeXML = "drs/shapexml.xml"

// metroPara is the left margin and the indent of a paragraph of the text of
// a metroBlob, in points, as marL and indent of a:pPr give them.
type metroPara struct {
	marL, indent opt[float64]
}

// metroParagraphs returns the left margins and the indents of the
// paragraphs of the text of the metroBlob b, in order. It returns nil when
// b holds no text that the package reads.
func metroParagraphs(b []byte, lim ivy.Limits) []metroPara {
	if len(b) == 0 {
		return nil
	}
	a, err := zipdoc.Open(bytes.NewReader(b), lim)
	if err != nil {
		return nil
	}
	f := a.Find(metroShapeXML)
	if f == nil {
		return nil
	}
	doc, err := a.ParseXML(f)
	if err != nil {
		return nil
	}
	var body roxmltree.Node
	var found bool
	for n := range doc.RootElement().Descendants() {
		if n.IsElement() && n.TagName().Name() == "txBody" {
			body, found = n, true
			break
		}
	}
	if !found {
		return nil
	}
	var paras []metroPara
	for p := range body.Children() {
		if !p.IsElement() || p.TagName().Name() != "p" {
			continue
		}
		var m metroPara
		for c := range p.Children() {
			if !c.IsElement() || c.TagName().Name() != "pPr" {
				continue
			}
			if v, ok := emuAttr(c, "marL"); ok {
				m.marL.set(v)
			}
			if v, ok := emuAttr(c, "indent"); ok {
				m.indent.set(v)
			}
			break
		}
		paras = append(paras, m)
	}
	return paras
}

// emuAttr returns the attribute name of the element n, a length in EMUs,
// in points. It returns false when n has no such attribute, or when the
// value is not an integer within the range of a:pPr.
func emuAttr(n roxmltree.Node, name string) (float64, bool) {
	s, ok := n.Attribute(roxmltree.ExpandedNameFromStr(name))
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || v < -51206400 || v > 51206400 {
		return 0, false
	}
	return float64(v) / emuPerPoint, true
}

// setMetroMargins gives the paragraphs of tb the left margins and the
// indents of the paragraphs of the metroBlob paras, when both have the same
// number of paragraphs.
func setMetroMargins(tb *drawingml.TextBody, paras []metroPara) {
	if tb == nil || len(paras) != len(tb.Paragraphs) {
		return
	}
	for i, m := range paras {
		st := &tb.Paragraphs[i].Style
		if m.marL.ok {
			st.MarL.Set(m.marL.v)
		}
		if m.indent.ok {
			st.Indent.Set(m.indent.v)
		}
	}
}
