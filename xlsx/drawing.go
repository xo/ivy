package xlsx

import (
	"path"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/chart"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/sheet"
)

// emuPerPoint is the number of English Metric Units in a point.
const emuPerPoint = 12700

// maxDrawingObjects bounds the images and the charts of a workbook.
const maxDrawingObjects = 4096

// imageExts are the extensions of the images that the layout draws.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".tif": true, ".tiff": true, ".webp": true, ".svg": true, ".emf": true, ".wmf": true,
}

// anchor is the cell of a corner of a drawing object, with its offset in
// points.
type anchor struct {
	row, col int
	dx, dy   float64
}

// readAnchor reads xdr:from or xdr:to.
func readAnchor(n roxmltree.Node) anchor {
	var a anchor
	for c := range n.Children() {
		v := 0
		if s := strings.TrimSpace(nodeText(c)); s != "" {
			for _, r := range s {
				if r < '0' || r > '9' {
					v = 0
					break
				}
				if v < 1<<40 {
					v = v*10 + int(r-'0')
				}
			}
		}
		switch local(c) {
		case "col":
			a.col = min(v, sheet.MaxCols-1)
		case "row":
			a.row = min(v, sheet.MaxRows-1)
		case "colOff":
			a.dx = float64(v) / emuPerPoint
		case "rowOff":
			a.dy = float64(v) / emuPerPoint
		}
	}
	return a
}

// readDrawing reads the drawing part of a worksheet: its pictures and its
// charts, each anchored at a cell. Shapes and groups are left out.
func (b *book) readDrawing(sh *sheet.Sheet, part string) {
	doc, err := b.parsePart(part)
	if err != nil {
		return
	}
	for n := range doc.RootElement().Children() {
		kind := local(n)
		if kind != "twoCellAnchor" && kind != "oneCellAnchor" && kind != "absoluteAnchor" {
			continue
		}
		if b.objects >= maxDrawingObjects {
			return
		}
		var from, to anchor
		var hasTo bool
		var cx, cy float64
		var obj roxmltree.Node
		hasObj := false
		for c := range n.Children() {
			switch local(c) {
			case "from":
				from = readAnchor(c)
			case "to":
				to, hasTo = readAnchor(c), true
			case "ext":
				cx, cy = number(c, "cx", 0)/emuPerPoint, number(c, "cy", 0)/emuPerPoint
			case "pos":
				from.dx, from.dy = number(c, "x", 0)/emuPerPoint, number(c, "y", 0)/emuPerPoint
			case "pic", "graphicFrame":
				obj, hasObj = c, true
			}
		}
		if !hasObj {
			continue
		}
		img := sheet.Image{Row: from.row, Col: from.col, DX: from.dx, DY: from.dy, Width: cx, Height: cy}
		if hasTo {
			img.Width, img.Height = b.span(sh, from, to)
		}
		if img.Width <= 0 || img.Height <= 0 || img.Width > 14400 || img.Height > 14400 {
			continue
		}
		switch local(obj) {
		case "pic":
			if !b.picture(&img, obj, part) {
				continue
			}
		case "graphicFrame":
			if !b.chart(&img, obj, part) {
				continue
			}
		}
		b.objects++
		sh.Images = append(sh.Images, img)
	}
}

// span returns the width and the height of an object from the anchor from
// to the anchor to. The sheet is sorted.
func (b *book) span(sh *sheet.Sheet, from, to anchor) (float64, float64) {
	w := to.dx - from.dx
	for c := from.col; c < to.col && c-from.col < 1024; c++ {
		w += sh.ColWidth(c)
	}
	h := to.dy - from.dy
	for r := from.row; r < to.row && r-from.row < 4096; r++ {
		h += sh.RowHeight(r)
	}
	return w, h
}

// picture reads the image of xdr:pic into img. It returns false for a
// picture that the layout cannot show.
func (b *book) picture(img *sheet.Image, pic roxmltree.Node, part string) bool {
	var id string
	for d := range pic.Descendants() {
		if local(d) == "blip" {
			id = attr(d, "embed")
			break
		}
	}
	target, ok := b.sheetRel(part, id)
	if !ok || !imageExts[strings.ToLower(path.Ext(target))] {
		return false
	}
	if _, ok := b.files[target]; !ok {
		buf, err := b.doc.ReadPart(target)
		if err != nil {
			return false
		}
		b.files[target] = buf
	}
	img.Path = target
	return true
}

// chart reads the chart of xdr:graphicFrame into img. It returns false for
// a frame that holds no chart. A chart that cannot be read shows as a frame
// with the title of the chart.
func (b *book) chart(img *sheet.Image, frame roxmltree.Node, part string) bool {
	var id string
	for d := range frame.Descendants() {
		if local(d) == "chart" && attr(d, "id") != "" {
			id = attr(d, "id")
			break
		}
	}
	target, ok := b.sheetRel(part, id)
	if !ok {
		return false
	}
	if obj := b.chartObject(target, img.Width, img.Height); obj != nil {
		name := "chart" + strconv.Itoa(b.objects+1)
		if b.chartObjects == nil {
			b.chartObjects = make(map[string]layout.Object)
			b.chartParts = make(map[string]string)
		}
		b.chartObjects[name] = obj
		b.chartParts[name] = target
		img.Object = name
		return true
	}
	img.Label = "Chart"
	if title := b.chartTitle(target); title != "" {
		img.Label = "Chart: " + title
	}
	return true
}

// chartObject returns the object that draws the chart part in a frame of w
// by h points, or nil when the part cannot be read. internal/chart draws it
// with zuzu (D24), with the colors of the theme of the workbook and the
// fonts of the program.
func (b *book) chartObject(part string, w, h float64) layout.Object {
	buf, err := b.doc.ReadPart(part)
	var ch *chart.Chart
	if err == nil {
		ch, err = chart.Parse(buf, chart.Options{
			Theme:  b.theme,
			Config: b.cfg,
			Fonts:  fontdb.Of(b.cfg),
			Width:  w,
			Height: h,
		})
	}
	if err != nil {
		b.cfg.Log().Debug("xlsx: chart left out", "part", part, "err", err)
		return nil
	}
	return ch
}

// chartTitle returns the text of the title of a chart part, or "".
func (b *book) chartTitle(part string) string {
	doc, err := b.parsePart(part)
	if err != nil {
		return ""
	}
	for n := range doc.RootElement().Descendants() {
		if local(n) != "title" {
			continue
		}
		var parts []string
		for t := range n.Descendants() {
			if local(t) == "t" || local(t) == "v" {
				parts = append(parts, nodeText(t))
			}
		}
		return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
	}
	return ""
}
