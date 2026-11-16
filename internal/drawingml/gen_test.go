package drawingml

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/roxmltree"
)

// update makes TestPresetTable write preset_table.go.
var update = flag.Bool("update", false, "write preset_table.go from testdata/ecma")

// The files of the generator.
const (
	presetFile = "testdata/ecma/presetShapeDefinitions.xml"
	noticeFile = "testdata/ecma/NOTICE"
	tableFile  = "preset_table.go"
)

// upArrowXML is the preset upArrow, which presetShapeDefinitions.xml does not
// hold. ivy wrote it as the mirror image of downArrow, from the top to the
// bottom.
const upArrowXML = `<upArrow xmlns="http://schemas.openxmlformats.org/drawingml/2006/main">
  <avLst>
    <gd name="adj1" fmla="val 50000" />
    <gd name="adj2" fmla="val 50000" />
  </avLst>
  <gdLst>
    <gd name="maxAdj2" fmla="*/ 100000 h ss" />
    <gd name="a1" fmla="pin 0 adj1 100000" />
    <gd name="a2" fmla="pin 0 adj2 maxAdj2" />
    <gd name="dy2" fmla="*/ ss a2 100000" />
    <gd name="y2" fmla="+- t dy2 0" />
    <gd name="dx1" fmla="*/ w a1 200000" />
    <gd name="x1" fmla="+- hc 0 dx1" />
    <gd name="x2" fmla="+- hc dx1 0" />
    <gd name="dy1" fmla="*/ x1 dy2 wd2" />
    <gd name="y1" fmla="+- y2 0 dy1" />
  </gdLst>
  <rect l="x1" t="y1" r="x2" b="b" />
  <pathLst>
    <path>
      <moveTo><pt x="l" y="y2" /></moveTo>
      <lnTo><pt x="x1" y="y2" /></lnTo>
      <lnTo><pt x="x1" y="b" /></lnTo>
      <lnTo><pt x="x2" y="b" /></lnTo>
      <lnTo><pt x="x2" y="y2" /></lnTo>
      <lnTo><pt x="r" y="y2" /></lnTo>
      <lnTo><pt x="hc" y="t" /></lnTo>
      <close />
    </path>
  </pathLst>
</upArrow>`

// TestPresetTable generates preset_table.go from presetShapeDefinitions.xml.
// With the flag -update, it writes the file. Without it, it fails if the file
// is not what it generates.
func TestPresetTable(t *testing.T) {
	list, err := readPresets()
	if err != nil {
		t.Fatal(err)
	}
	src, err := genTable(list)
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(tableFile, src, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	have, err := os.ReadFile(tableFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(have, src) {
		t.Fatalf("%s is not what the generator writes. Run go generate ./internal/drawingml", tableFile)
	}
}

// readPresets reads the presets of presetShapeDefinitions.xml, with upArrow,
// sorted by name.
func readPresets() ([]Geometry, error) {
	data, err := os.ReadFile(presetFile)
	if err != nil {
		return nil, fmt.Errorf("reading the presets: %w", err)
	}
	doc, err := roxmltree.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", presetFile, err)
	}
	var list []Geometry
	add := func(n roxmltree.Node) error {
		g, err := ParseCustom(n)
		if err != nil {
			return fmt.Errorf("preset %s: %w", n.TagName().Name(), err)
		}
		g.name = n.TagName().Name()
		i := slices.IndexFunc(list, func(o Geometry) bool { return o.name == g.name })
		switch {
		case i < 0:
			list = append(list, *g)
		case !reflect.DeepEqual(list[i], *g):
			return fmt.Errorf("preset %s: two different definitions", g.name)
		}
		return nil
	}
	for n := range doc.RootElement().Children() {
		if !n.IsElement() {
			continue
		}
		if err := add(n); err != nil {
			return nil, err
		}
	}
	if !slices.ContainsFunc(list, func(g Geometry) bool { return g.name == "upArrow" }) {
		up, err := roxmltree.Parse(upArrowXML)
		if err != nil {
			return nil, fmt.Errorf("parsing upArrow: %w", err)
		}
		if err := add(up.RootElement()); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(list, func(a, b Geometry) int { return strings.Compare(a.name, b.name) })
	// The text rectangle of pie swaps its top and its right side.
	for i := range list {
		if list[i].name == "pie" && list[i].text == (textRect{"il", "ir", "it", "ib"}) {
			list[i].text = textRect{"il", "it", "ir", "ib"}
		}
	}
	return list, nil
}

// The Go names of the constants that the table uses.
var (
	opNames = [...]string{
		opVal: "opVal", opMulDiv: "opMulDiv", opAddSub: "opAddSub",
		opAddDiv: "opAddDiv", opIfElse: "opIfElse", opAbs: "opAbs",
		opAt2: "opAt2", opCat2: "opCat2", opCos: "opCos", opMax: "opMax",
		opMin: "opMin", opMod: "opMod", opPin: "opPin", opSat2: "opSat2",
		opSin: "opSin", opSqrt: "opSqrt", opTan: "opTan",
	}
	verbNames = [...]string{
		verbMoveTo: "verbMoveTo", verbLineTo: "verbLineTo",
		verbArcTo: "verbArcTo", verbQuadTo: "verbQuadTo",
		verbCubicTo: "verbCubicTo", verbClose: "verbClose",
	}
	fillNames = [...]string{
		FillNorm: "FillNorm", FillNone: "FillNone", FillLighten: "FillLighten",
		FillLightenLess: "FillLightenLess", FillDarken: "FillDarken",
		FillDarkenLess: "FillDarkenLess",
	}
)

// genTable returns the source of preset_table.go for the presets.
func genTable(list []Geometry) ([]byte, error) {
	notice, err := os.ReadFile(noticeFile)
	if err != nil {
		return nil, fmt.Errorf("reading the notice: %w", err)
	}
	i := bytes.Index(notice, []byte("COPYRIGHT NOTICE"))
	if i < 0 {
		return nil, fmt.Errorf("%s has no copyright notice", noticeFile)
	}
	var b bytes.Buffer
	b.WriteString("// Code generated by TestPresetTable from " + presetFile + ". DO NOT EDIT.\n\n")
	b.WriteString("// The tables of this file come from presetShapeDefinitions.xml of ECMA-376\n")
	b.WriteString("// Part 1, 5th edition. ivy wrote the preset upArrow, which the file does not\n")
	b.WriteString("// hold, and it corrects the text rectangle of pie. " + noticeFile + " has\n")
	b.WriteString("// the source and the hashes of the file.\n")
	b.WriteString("// This is the copyright notice of the file:\n//\n")
	for line := range strings.Lines(string(notice[i:])) {
		line = strings.TrimRight(line, "\n")
		if line == "" {
			b.WriteString("//\n")
			continue
		}
		b.WriteString("// " + line + "\n")
	}
	b.WriteString("\npackage drawingml\n\n")
	b.WriteString("// presets holds the preset shapes, sorted by name.\n")
	b.WriteString("var presets = [...]Geometry{\n")
	for _, g := range list {
		fmt.Fprintf(&b, "{\nname: %q,\n", g.name)
		writeGuides(&b, "adjust", g.adjust)
		writeGuides(&b, "guides", g.guides)
		if len(g.paths) > 0 {
			b.WriteString("paths: []pathDef{\n")
			for _, p := range g.paths {
				b.WriteString("{")
				if p.w != 0 || p.h != 0 {
					fmt.Fprintf(&b, "w: %s, h: %s, ", float(p.w), float(p.h))
				}
				fmt.Fprintf(&b, "fill: %s, stroke: %t, extrusion: %t, commands: []command{\n", fillNames[p.fill], p.stroke, p.extrusion)
				for _, c := range p.commands {
					fmt.Fprintf(&b, "{%s, %s},\n", verbNames[c.verb], strArray(c.args[:]))
				}
				b.WriteString("}},\n")
			}
			b.WriteString("},\n")
		}
		if g.text != (textRect{}) {
			fmt.Fprintf(&b, "text: textRect{%q, %q, %q, %q},\n", g.text.l, g.text.t, g.text.r, g.text.b)
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting %s: %w", tableFile, err)
	}
	return src, nil
}

// writeGuides writes the field with the name and the guides of list.
func writeGuides(b *bytes.Buffer, name string, list []Guide) {
	if len(list) == 0 {
		return
	}
	b.WriteString(name + ": []Guide{\n")
	for _, g := range list {
		fmt.Fprintf(b, "{%q, %s, %s},\n", g.name, opNames[g.op], strArray(g.args[:]))
	}
	b.WriteString("},\n")
}

// strArray returns the Go literal of an array of strings, without the empty
// strings at its end.
func strArray(s []string) string {
	n := len(s)
	for n > 0 && s[n-1] == "" {
		n--
	}
	q := make([]string, n)
	for i := range n {
		q[i] = strconv.Quote(s[i])
	}
	return fmt.Sprintf("[%d]string{%s}", len(s), strings.Join(q, ", "))
}

// float returns the Go literal of v.
func float(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
