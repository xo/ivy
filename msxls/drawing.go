package msxls

import (
	"bytes"
	"context"
	"image/color"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/odraw"
	"github.com/xo/ivy/internal/sheet"
)

// This file draws the drawing objects of a sheet: the shapes, the groups,
// the text boxes and the pictures. A worksheet holds them in the records
// MSODRAWING, as Office Drawing (MS-ODRAW), and internal/odraw reads them.
// A record OBJ after the drawing of each shape gives the type of the
// object, and a record TXO the text of a shape (MS-XLS 2.4.181 and
// 2.4.329). Each shape at the top of the drawing, or each group, is an
// object of the layout at its cells. A chart is the chart substream after
// its record OBJ, which chart.go reads. A chart that the package cannot
// read shows as a frame.

// The records of the drawings.
const (
	recMsoDrawingGroup = 0x00EB
	recMsoDrawing      = 0x00EC
	recObj             = 0x005D
	recTxo             = 0x01B6
)

// The types of the objects of the record OBJ, in its structure FtCmo.
const (
	objGroup    = 0x00
	objChart    = 0x05
	objText     = 0x06
	objPicture  = 0x08
	objComboBox = 0x14
	objNote     = 0x19
)

// cmoPrint is the bit fPrint of an FtCmo: the object shows on the printed
// pages.
const cmoPrint = 0x0010

// maxDrawing bounds the bytes of the drawing of one sheet and of the
// pictures of the workbook.
const maxDrawing = 64 << 20

// shapeScale is the number of pixels for a point that a shape takes for its
// patterns and its text. The layout does not give the resolution of the
// page to an object, so a shape takes the resolution of 144 dots per inch.
const shapeScale = 2

// drawObj is a record OBJ of a sheet: the offset of the stream of the
// drawing when it comes, its type and its flags, and the text of its
// record TXO with its runs and its alignment.
type drawObj struct {
	at    int
	ot    int
	flags uint16
	text  string
	runs  []fontRun
	// hAlign and vAlign are the alignments of the text of the TXO: 1 for
	// left or top, 2 for centered, 3 for right or bottom, 4 for justified
	// and 7 for distributed.
	hAlign, vAlign int
	// chart holds the records of the chart substream of a chart.
	chart []record
}

// drawing is the drawing of a sheet, with the objects of its shapes.
type drawing struct {
	w    *workbook
	sh   *sheet.Sheet
	raw  *rawSheet
	book *sheet.Workbook
	ctx  context.Context
	pics *odraw.Pictures
	cfg  ivy.Config
	look *chartLookup
	// objs maps a shape to its record OBJ.
	objs  map[*odraw.Shape]*drawObj
	style *odraw.Style
}

// drawings adds the drawing objects of raw to the images of sh, and their
// objects to book.
func (w *workbook) drawings(ctx context.Context, sh *sheet.Sheet, raw *rawSheet, book *sheet.Workbook, pics *odraw.Pictures, look *chartLookup, cfg ivy.Config) {
	if len(raw.objs) == 0 || len(raw.drawing) < 8 {
		return
	}
	h, _ := odraw.ReadHeader(raw.drawing)
	if h.Type != odraw.RecDgContainer {
		return
	}
	lim := cfg.Limits
	body := raw.drawing[8 : 8+int(min(uint64(h.Size), uint64(len(raw.drawing)-8)))]
	od := odraw.ReadDrawing(body, lim.MaxObjects, lim.MaxDepth)
	if od.Root == nil {
		return
	}
	d := &drawing{w: w, sh: sh, raw: raw, book: book, ctx: ctx, pics: pics, cfg: cfg, look: look, objs: make(map[*odraw.Shape]*drawObj)}
	d.style = &odraw.Style{
		Scheme:     d.paletteColor,
		Picture:    d.picture,
		Background: drawingml.White,
		Log:        cfg.Log(),
		Fonts:      fontdb.Of(cfg),
	}
	d.matchObjs(od.Root, lim.MaxDepth)
	for _, top := range od.Root.Children {
		if len(sh.Images) >= 4096 {
			break
		}
		if top.Flags&odraw.FSPDeleted == 0 {
			d.add(top)
		}
	}
}

// paletteColor returns the color of the index i of the palette of the
// workbook, which an OfficeArtCOLORREF of a sheet names by fSchemeIndex.
func (d *drawing) paletteColor(i uint32) drawingml.RGBA {
	c := d.w.color(int(min(i, 0xFFFF)))
	return odraw.RGB(c.R, c.G, c.B)
}

// picture returns the picture of the store of the workbook.
func (d *drawing) picture(pib int, key uint32, keyed bool) *drawingml.Picture {
	return d.pics.Get(d.ctx, pib, key, keyed)
}

// matchObjs gives each shape of the group g the record OBJ that follows
// its client data in the stream of the drawing.
func (d *drawing) matchObjs(g *odraw.Shape, depth int) {
	type end struct {
		at int
		sh *odraw.Shape
	}
	var ends []end
	var walk func(sh *odraw.Shape, depth int)
	walk = func(sh *odraw.Shape, depth int) {
		if sh.ClientData != nil {
			// the client data is a slice of the stream of the drawing
			at := cap(d.raw.drawing) - cap(sh.ClientData) + len(sh.ClientData)
			ends = append(ends, end{at: at, sh: sh})
		}
		if depth > 0 {
			for _, c := range sh.Children {
				walk(c, depth-1)
			}
		}
	}
	walk(g, depth)
	slices.SortStableFunc(ends, func(a, b end) int { return a.at - b.at })
	for i := range d.raw.objs {
		o := &d.raw.objs[i]
		// the object takes the last shape before it, unless an object took
		// it
		k, _ := slices.BinarySearchFunc(ends, o.at+1, func(e end, at int) int { return e.at - at })
		if k == 0 {
			continue
		}
		if sh := ends[k-1].sh; d.objs[sh] == nil {
			d.objs[sh] = o
		}
	}
}

// add adds the shape or the group top, which a cell anchor places on the
// sheet, as an image.
func (d *drawing) add(top *odraw.Shape) {
	o := d.objs[top]
	if o == nil {
		return
	}
	switch {
	case o.ot == objNote || o.ot == objComboBox:
		// a comment shows only when a person points at its cell, and a combo
		// box is the button of a filter
		return
	case o.flags&cmoPrint == 0:
		return
	}
	a, ok := clientAnchor(top.ClientAnchor)
	if !ok {
		return
	}
	img, ok := anchorImage(d.sh, a)
	if !ok {
		return
	}
	if o.ot == objChart {
		if img.Width >= 1 && img.Height >= 1 {
			img.Label = "Chart"
			if obj := d.w.chartObject(d.look, o.chart, img.Width, img.Height, d.cfg); obj != nil {
				img.Label, img.Object = "", d.addObject("chart", obj)
			}
			d.sh.Images = append(d.sh.Images, img)
		}
		return
	}
	if d.plainPicture(top, &img) {
		d.sh.Images = append(d.sh.Images, img)
		return
	}
	// The box takes the lines, the arrowheads and the shadows that go past
	// it, since the layout clips an object to its box.
	pad := 0.0
	odraw.Boxes(top, odraw.Rect{Right: img.Width, Bottom: img.Height}, func(sh *odraw.Shape, r odraw.Rect) {
		pad = max(pad, d.style.Bleed(odraw.Chain{&sh.Props}, r))
	})
	pad = math.Ceil(min(pad, 1000))
	obj := &shapeObject{scene: drawingml.NewScene(d.cfg)}
	odraw.Boxes(top, odraw.Rect{Left: pad, Top: pad, Right: pad + img.Width, Bottom: pad + img.Height}, func(sh *odraw.Shape, r odraw.Rect) {
		if len(obj.figures) < d.cfg.Limits.MaxObjects {
			if f := d.figure(sh, r); f != nil {
				obj.figures = append(obj.figures, f)
				obj.scene.AddFigure(f)
			}
		}
	})
	obj.w, obj.h = img.Width+2*pad, img.Height+2*pad
	if len(obj.figures) == 0 || !(obj.w >= 1 && obj.h >= 1) {
		return
	}
	img.DX, img.DY, img.Width, img.Height = img.DX-pad, img.DY-pad, obj.w, obj.h
	img.Object = d.addObject("shape", obj)
	d.sh.Images = append(d.sh.Images, img)
}

// addObject adds an object of the layout to the workbook, and returns its
// name.
func (d *drawing) addObject(prefix string, obj layout.Object) string {
	name := prefix + strconv.Itoa(len(d.book.Objects)+1)
	if d.book.Objects == nil {
		d.book.Objects = make(map[string]layout.Object)
	}
	d.book.Objects[name] = obj
	return name
}

// plainPicture sets the path of the image img to the file of the picture
// of the shape sh, when sh is a picture without a crop, a turn, a line, a
// color key, a shadow or a change of its colors, in a raster format. The
// layout then draws the picture as an image of the sheet, as it draws the
// pictures of xlsx.
func (d *drawing) plainPicture(sh *odraw.Shape, img *sheet.Image) bool {
	c := odraw.Chain{&sh.Props}
	pib := int(c.Value(odraw.PropPib, 0))
	if sh.IsGroup() || sh.Type != odraw.SptPictureFrame || pib <= 0 || !(img.Width >= 1 && img.Height >= 1) {
		return false
	}
	for _, id := range []uint16{odraw.PropRotation, odraw.PropCropTop, odraw.PropCropBottom, odraw.PropCropLeft, odraw.PropCropRight, odraw.PropPictureTransparent} {
		if v, ok := c.Get(id); ok && v != 0 {
			return false
		}
	}
	if l := d.style.Line(c, false); l.Visible() || d.style.Shadow(c) != nil || !odraw.PictureRecolor(c).Zero() || sh.Flags&(odraw.FSPFlipH|odraw.FSPFlipV) != 0 {
		return false
	}
	data := d.pics.Data(pib)
	ext := pictureExt(data)
	if ext == "" {
		return false
	}
	img.Path = "media/image" + strconv.Itoa(pib) + ext
	d.book.Files[img.Path] = data
	return true
}

// pictureExt returns the extension of the file of a raster picture, or "".
// A metafile draws in the scene of a shape: the layout draws a WMF file
// without a placeable header blank as an image of a sheet.
func pictureExt(data []byte) string {
	switch {
	case len(data) < 4:
		return ""
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		return ".png"
	case bytes.HasPrefix(data, []byte("\xff\xd8")):
		return ".jpg"
	case bytes.HasPrefix(data, []byte("BM")):
		return ".bmp"
	}
	return ""
}

// figure returns the figure of the shape sh in the box r, with the text of
// its record TXO, or nil.
func (d *drawing) figure(sh *odraw.Shape, r odraw.Rect) *drawingml.Figure {
	c := odraw.Chain{&sh.Props}
	if odraw.Hidden(c) {
		return nil
	}
	o := d.objs[sh]
	if o != nil && (o.ot == objNote || o.ot == objComboBox || o.ot == objChart || o.flags&cmoPrint == 0) {
		return nil
	}
	f := d.style.Figure(sh, c, r)
	if f == nil {
		return nil
	}
	if o != nil && o.text != "" {
		f.Text = d.textBody(o, c)
	}
	return f
}

// textBody returns the text of the record TXO o, in a shape with the
// properties c. The runs give the fonts of the characters, and a
// character without a run takes the default font of the workbook.
func (d *drawing) textBody(o *drawObj, c odraw.Chain) *drawingml.TextBody {
	tb := &drawingml.TextBody{Body: odraw.BodyStyle(c), WinBaseline: true}
	switch o.vAlign {
	case 2:
		tb.Body.Anchor.Set("ctr")
	case 3:
		tb.Body.Anchor.Set("b")
	case 1, 4, 7:
		tb.Body.Anchor.Set("t")
	}
	var align string
	switch o.hAlign {
	case 2:
		align = "ctr"
	case 3:
		align = "r"
	case 4:
		align = "just"
	case 7:
		align = "dist"
	default:
		align = "l"
	}
	units := utf16.Encode([]rune(o.text))
	font := d.w.font(0)
	runs := o.runs
	para := drawingml.Paragraph{}
	para.Style.Align.Set(align)
	flush := func(rs drawingml.RunStyle, text []uint16) {
		if s := strings.ReplaceAll(string(utf16.Decode(text)), "\r", ""); s != "" {
			para.Runs = append(para.Runs, drawingml.Run{Text: s, Style: rs})
		}
	}
	start := 0
	rs := d.runStyle(font)
	for i := 0; i <= len(units); i++ {
		if len(runs) > 0 && runs[0].ich <= i {
			flush(rs, units[start:i])
			start = i
			rs = d.runStyle(d.w.font(runs[0].ifnt))
			runs = runs[1:]
			i--
			continue
		}
		if i == len(units) || units[i] == '\n' {
			flush(rs, units[start:i])
			para.End = rs
			tb.Paragraphs = append(tb.Paragraphs, para)
			para = drawingml.Paragraph{}
			para.Style.Align.Set(align)
			start = i + 1
		}
	}
	return tb
}

// runStyle returns the style of a run of the font f.
func (d *drawing) runStyle(f sheet.Font) drawingml.RunStyle {
	var rs drawingml.RunStyle
	if f.Family != "" {
		rs.Latin.Set(f.Family)
	}
	if f.Size > 0 {
		rs.Size.Set(f.Size)
	}
	rs.Bold.Set(f.Bold)
	rs.Italic.Set(f.Italic)
	rs.Underline.Set(f.Underline > 0)
	rs.Strike.Set(f.Strike)
	col := f.Color
	if col.A == 0 {
		col = color.NRGBA{0, 0, 0, 255}
	}
	rs.Paint.Set(drawingml.Solid(odraw.RGB(col.R, col.G, col.B)))
	switch f.VertAlign {
	case 1:
		rs.Baseline.Set(0.3)
	case 2:
		rs.Baseline.Set(-0.25)
	}
	return rs
}

// clientAnchor reads the cell anchor of a shape, an
// OfficeArtClientAnchorSheet of MS-XLS: the flags, then the column and the
// offset in 1/1024 of its width, and the row and the offset in 1/256 of its
// height, of the top left corner and of the bottom right corner.
func clientAnchor(b []byte) ([8]int, bool) {
	var a [8]int
	if len(b) < 18 {
		return a, false
	}
	for i := range a {
		a[i] = int(le.Uint16(b[2+2*i:]))
	}
	return a, true
}

// anchorImage returns the image of a cell anchor a. A line across or down
// has a box of no height or no width.
func anchorImage(sh *sheet.Sheet, a [8]int) (sheet.Image, bool) {
	col0, dx0, row0, dy0 := a[0], a[1], a[2], a[3]
	col1, dx1, row1, dy1 := a[4], a[5], a[6], a[7]
	if col0 > maxCol || col1 > maxCol || row0 > maxRow || row1 > maxRow || col1 < col0 || row1 < row0 || row1-row0 > 4096 {
		return sheet.Image{}, false
	}
	img := sheet.Image{Row: row0, Col: col0}
	img.DX = float64(min(dx0, 1024)) / 1024 * sh.ColWidth(col0)
	img.DY = float64(min(dy0, 256)) / 256 * sh.RowHeight(row0)
	x1 := float64(min(dx1, 1024)) / 1024 * sh.ColWidth(col1)
	y1 := float64(min(dy1, 256)) / 256 * sh.RowHeight(row1)
	for c := col0; c < col1; c++ {
		x1 += sh.ColWidth(c)
	}
	for r := row0; r < row1; r++ {
		y1 += sh.RowHeight(r)
	}
	img.Width, img.Height = x1-img.DX, y1-img.DY
	if !(img.Width >= 0) || !(img.Height >= 0) || img.Width > 14400 || img.Height > 14400 {
		return sheet.Image{}, false
	}
	return img, true
}

// shapeObject is a shape of a sheet, or a group of shapes, in its box. It
// satisfies the interface Object of the layout.
type shapeObject struct {
	scene *drawingml.Scene
	w, h  float64
	// figures are the figures of the scene, for the tests.
	figures []*drawingml.Figure
}

// Size satisfies the interface Object of the layout.
func (o *shapeObject) Size() (float64, float64) {
	return o.w, o.h
}

// Draw satisfies the interface Object of the layout. It draws the box of
// the shape onto rect.
func (o *shapeObject) Draw(ctx context.Context, dev display.Device, rect okage.Rect) error {
	rw, rh := float64(rect.Width()), float64(rect.Height())
	if !(rw > 0) || !(rh > 0) || math.IsInf(rw, 0) || math.IsInf(rh, 0) {
		return nil
	}
	m := okage.TransformFromRow(float32(rw/o.w), 0, 0, float32(rh/o.h), rect.X(), rect.Y())
	dev.Save()
	defer dev.Restore()
	dev.Concat(m)
	return o.scene.Draw(ctx, dev, okage.TransformFromScale(shapeScale, shapeScale).PreConcat(m))
}
