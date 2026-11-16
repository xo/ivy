package drawingml

import (
	"math"
	"testing"

	"github.com/xo/okage/roxmltree"
)

// FuzzCustom parses a custom geometry and evaluates it. Each point and the
// text rectangle must be finite and near the box.
func FuzzCustom(f *testing.F) {
	const ns = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
	f.Add(customXML)
	f.Add(`<a:custGeom ` + ns + `><a:pathLst><a:path><a:moveTo><a:pt x="0" y="0"/></a:moveTo><a:lnTo><a:pt x="w" y="h"/></a:lnTo></a:path></a:pathLst></a:custGeom>`)
	f.Add(`<a:custGeom ` + ns + `><a:gdLst><a:gd name="a" fmla="at2 w h"/><a:gd name="r" fmla="mod w h 0"/></a:gdLst>` +
		`<a:pathLst><a:path w="21600" h="21600"><a:moveTo><a:pt x="10800" y="0"/></a:moveTo><a:arcTo wR="r" hR="10800" stAng="a" swAng="-cd2"/><a:close/></a:path></a:pathLst></a:custGeom>`)
	f.Add(`<a:custGeom ` + ns + `><a:avLst><a:gd name="adj" fmla="pin 0 adj 100000"/></a:avLst><a:gdLst><a:gd name="x" fmla="?: adj 1 -1"/><a:gd name="y" fmla="tan x 5400000"/></a:gdLst>` +
		`<a:rect l="y" t="x" r="0" b="b"/><a:pathLst><a:path fill="lighten"><a:cubicBezTo><a:pt x="y" y="y"/><a:pt x="x" y="1e309"/><a:pt x="-9e18" y="NaN"/></a:cubicBezTo></a:path></a:pathLst></a:custGeom>`)
	f.Fuzz(func(t *testing.T, s string) {
		opts := roxmltree.DefaultParsingOptions()
		opts.NodesLimit = 1 << 14
		doc, err := roxmltree.ParseWithOptions(s, opts)
		if err != nil {
			return
		}
		g, err := ParseCustom(doc.RootElement())
		if err != nil {
			return
		}
		for _, size := range [][2]float64{{100, 50}, {0, 0}, {1e7, 3}} {
			sh, err := g.Evaluate(size[0], size[1], g.adjust)
			if err != nil {
				t.Fatal(err)
			}
			limit := boxes * max(size[0], size[1], 1) * (1 + 1e-6)
			for _, p := range sh.Paths {
				for _, pt := range p.Path.Points() {
					x, y := float64(pt.X), float64(pt.Y)
					if math.IsNaN(x) || math.IsNaN(y) || math.Abs(x) > limit || math.Abs(y) > limit {
						t.Fatalf("point %v for the size %v", pt, size)
					}
				}
			}
			checkFinite(t, "text", sh)
		}
	})
}
