package msdoc

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/odraw"
)

// This file reads the shapes of Office Drawing that float on the page: the
// pictures, the text boxes, whose text is in the part of the text boxes,
// and the other shapes and the groups of them (MS-DOC 2.9.94 and
// MS-ODRAW). internal/odraw reads the shapes and draws them as figures of
// internal/drawingml (D51). A shape goes into the layout as a box whose
// content is a display list, and the text of a text box goes over that
// box as HTML. A picture without a crop, a turn, a flip, a line, a shadow
// or a transparent color goes into the layout as an image of its file.

// The properties of a shape that the package reads besides those of
// internal/odraw (MS-ODRAW 2.3.4).
const (
	// propPosH and propPosV are the alignment of a shape: 1 at the left
	// or the top, 2 at the center and 3 at the right or the bottom, or 0
	// for its place.
	propPosH = 0x038F
	propPosV = 0x0391
	// propPosRelH and propPosRelV give the origin of the place of a shape:
	// the margin, the page, the column or the paragraph.
	propPosRelH = 0x0390
	propPosRelV = 0x0392
	// behindDocument is the bit fBehindDocument of the property
	// groupBools, and useBehindDocument its bit of use (MS-ODRAW
	// 2.3.4.44).
	behindDocument    = 1 << 5
	useBehindDocument = 1 << 21
)

// fspaSize is the size of an FSPA (MS-DOC 2.9.94).
const fspaSize = 26

// shapeScale is the number of pixels for a point at which the list of a
// shape records its patterns and its pictures in SVG, 144 dots per inch.
// The layout gives no resolution to an object.
const shapeScale = 2

// drawing holds the shapes of the OfficeArtContent of a document by their
// ids, and the pictures of its store.
type drawing struct {
	shapes map[uint32]*odraw.Shape
	pics   *odraw.Pictures
}

// readDrawing reads the OfficeArtContent of the document (MS-DOC
// 2.9.172): the drawing group, with the store of the pictures, then each
// drawing, of the main document or of the headers, after a byte that tells
// which. The pictures of the store are in the stream WordDocument, or in
// their entries.
func (p *parser) readDrawing() *drawing {
	d := &drawing{shapes: map[uint32]*odraw.Shape{}}
	var store []odraw.Entry
	b := p.s.part(fcDggInfo)
	if p.s.word6 {
		b = nil
	}
	for len(b) >= 8 {
		h, _ := odraw.ReadHeader(b)
		if h.Type != odraw.RecDggContainer && h.Type != odraw.RecDgContainer && len(b) >= 9 && le.Uint16(b[3:]) == odraw.RecDgContainer {
			// the byte dgglbl before a drawing
			b = b[1:]
			continue
		}
		if uint64(h.Size) > uint64(len(b)-8) {
			break
		}
		body := b[8 : 8+h.Size]
		switch h.Type {
		case odraw.RecDggContainer:
			if store == nil {
				store = odraw.ReadStore(body, p.lim.MaxObjects)
			}
		case odraw.RecDgContainer:
			if dr := odraw.ReadDrawing(body, p.lim.MaxObjects, p.lim.MaxDepth); dr.Root != nil {
				d.add(dr.Root, p.lim.MaxDepth)
			}
		}
		b = b[8+h.Size:]
	}
	d.pics = odraw.NewPictures(store, p.s.word, p.cfg)
	return d
}

// add adds the shape sh and the shapes of its group to the shapes of d, by
// their ids.
func (d *drawing) add(sh *odraw.Shape, depth int) {
	if _, ok := d.shapes[sh.SPID]; !ok {
		d.shapes[sh.SPID] = sh
	}
	if depth > 0 {
		for _, c := range sh.Children {
			d.add(c, depth-1)
		}
	}
}

// style returns the style that turns the shapes into figures, with the
// pictures pics. A shape of Word has no colors of a scheme.
func (p *parser) style(ctx context.Context, pics *odraw.Pictures) *odraw.Style {
	if p.fontDB == nil {
		p.fontDB = fontdb.Of(p.cfg)
	}
	return &odraw.Style{
		Picture: func(pib int, key uint32, keyed bool) *drawingml.Picture {
			return pics.Get(ctx, pib, key, keyed)
		},
		Background: drawingml.White,
		Log:        p.cfg.Log(),
		Fonts:      p.fontDB,
	}
}

// drawn is a shape of Office Drawing that the conversion writes: a text
// box, with the paragraphs of its text, a shape, a group or a picture. It
// floats on the page as a picture does.
type drawn struct {
	floating
	shape *odraw.Shape
	// paras holds the paragraphs of a text box, and text is set for one.
	paras []paragraph
	text  bool
	// obj is the URL of the object of the shape of a text box, which goes
	// under its text, or "" for a text box that draws nothing.
	obj string
	// posH and posV are the alignment of the shape: 1 at the left or the
	// top, 2 at the center and 3 at the right or the bottom, or 0 for
	// its place.
	posH, posV int
	// texts holds the text boxes of the shapes of a group.
	texts []groupText
}

// block reports whether the shape is a block of HTML: a text box, or a
// group with text boxes.
func (d *drawn) block() bool {
	return d.text || len(d.texts) > 0
}

// groupText is a shape of a group that holds text: its box in the object
// of the group, in points, its shape and its paragraphs.
type groupText struct {
	r     odraw.Rect
	shape *odraw.Shape
	paras []paragraph
}

// floating is a picture or a shape that floats on the page: its picture,
// its place in twips, and how the text wraps around it.
type floating struct {
	pic *picture
	// left and top are the place of the picture from the origin of bx
	// and by: 0 the margin, 1 the page, and 2 the column or the paragraph.
	left, top int
	bx, by    int
	// wr is how the text wraps: 1 above and below, 3 not at all, and the
	// others around it.
	wr int
	// wrk is the side of the text that wraps around it: 0 both sides, 1
	// the left side, 2 the right side and 3 the largest side.
	wrk    int
	behind bool
}

// anchors returns the shapes of the main document, or of the headers when
// header is set, by the character of their anchor in their part of the
// text: the FSPAs of the PlcSpaMom or of the PlcSpaHdr.
func (p *parser) anchors(header bool) map[uint32][]byte {
	i := fcPlcSpaMom
	if header {
		i = fcPlcSpaHdr
	}
	out := map[uint32][]byte{}
	plc := p.s.part(i)
	if p.s.word6 || len(plc) < 4+fspaSize {
		return out
	}
	n := (len(plc) - 4) / (4 + fspaSize)
	for k := range min(n, p.lim.MaxObjects) {
		out[u32(plc, 4*k)] = plc[4*(n+1)+fspaSize*k : 4*(n+1)+fspaSize*(k+1)]
	}
	return out
}

// textboxes returns the range of the text of each text box of the main
// document, or of the headers, by the id of its shape: the PlcftxbxTxt or
// the PlcfHdrtxbxTxt, with an FTXBXS for each text box (MS-DOC 2.8.38 and
// 2.9.92).
func (p *parser) textboxes(header bool) map[uint32][2]uint32 {
	i, part := fcPlcftxbxTxt, partTextboxes
	if header {
		i, part = fcPlcfHdrTxbx, partHeaderTextboxes
	}
	const ftxbxsSize = 22
	out := map[uint32][2]uint32{}
	plc := p.s.part(i)
	n := min((len(plc)-4)/(4+ftxbxsSize), p.lim.MaxObjects)
	start, end := p.s.fib.partRange(part)
	for k := range max(n, 0) {
		ftxbxs := plc[4*(n+1)+ftxbxsSize*k:]
		// a text box that Word can reuse holds no text
		if u16(ftxbxs, 8) != 0 {
			continue
		}
		a, b := uint64(start)+uint64(u32(plc, 4*k)), uint64(start)+uint64(u32(plc, 4*k+4))
		if a >= b || b > uint64(end) {
			continue
		}
		out[u32(ftxbxs, 14)] = [2]uint32{uint32(a), uint32(b)}
	}
	return out
}

// drawnAt returns the shape whose anchor is the character cp of the main
// document, or of the headers when header is set, or nil for a shape that
// draws nothing. A text box reads its text.
func (p *parser) drawnAt(ctx context.Context, cp uint32, header bool) (*drawn, error) {
	if p.drawing == nil {
		p.drawing = p.readDrawing()
		p.spas = [2]map[uint32][]byte{p.anchors(false), p.anchors(true)}
		p.txbxs = [2]map[uint32][2]uint32{p.textboxes(false), p.textboxes(true)}
	}
	k := 0
	if header {
		start, _ := p.s.fib.partRange(partHeaders)
		cp -= start
		k = 1
	}
	fspa, ok := p.spas[k][cp]
	if !ok {
		return nil, nil
	}
	s := p.drawing.shapes[u32(fspa, 0)]
	if s == nil {
		return nil, nil
	}
	c := odraw.Chain{&s.Props}
	left, top := int(int32(u32(fspa, 4))), int(int32(u32(fspa, 8)))
	right, bottom := int(int32(u32(fspa, 12))), int(int32(u32(fspa, 16)))
	w, h := right-left, bottom-top
	if w < 0 || h < 0 || w > 31680 || h > 31680 || w+h == 0 || odraw.Hidden(c) {
		return nil, nil
	}
	flags := u16(fspa, 20)
	d := &drawn{
		floating: floating{
			left: left, top: top,
			bx: int(flags >> 1 & 3), by: int(flags >> 3 & 3), wr: int(flags >> 5 & 0xF), wrk: int(flags >> 9 & 0xF),
			behind: flags&0x4000 != 0 || c.Flag(odraw.PropGroupBools, behindDocument, useBehindDocument, false),
		},
		shape: s,
		posH:  int(c.Value(propPosH, 0)),
		posV:  int(c.Value(propPosV, 0)),
	}
	// the origins of the shape, which Word 2000 and later write in the
	// shape and not in the FSPA: the margin, the page, the column and the
	// character, or the margin, the page, the paragraph and the line
	if v, ok := c.Get(propPosRelH); ok && v <= 3 {
		d.bx = [...]int{0, 1, 2, 2}[v]
	}
	if v, ok := c.Get(propPosRelV); ok && v <= 3 {
		d.by = [...]int{0, 1, 2, 2}[v]
	}
	wp, hp := float64(w)/20, float64(h)/20
	if pic := p.plainPicture(ctx, s, p.drawing.pics, wp, hp); pic != nil {
		// a picture, such as that of an OLE object, shows and not its text
		d.pic = pic
		return d, nil
	}
	if r, ok := p.txbxs[k][s.SPID]; ok && !hasPicture(s) {
		paras, err := p.boxParagraphs(ctx, r)
		if err != nil {
			return nil, err
		}
		d.paras, d.text = paras, true
		d.pic = &picture{width: wp, height: hp}
		if obj, ok := p.shapeObject(ctx, s, wp, hp, p.drawing.pics); ok {
			d.obj, d.pic.pad = obj.name, obj.pad
		}
		return d, nil
	}
	obj, ok := p.shapeObject(ctx, s, wp, hp, p.drawing.pics)
	if !ok {
		return nil, nil
	}
	d.pic = &picture{name: obj.name, width: wp, height: hp, pad: obj.pad}
	if s.IsGroup() {
		box := odraw.Rect{Left: obj.pad, Top: obj.pad, Right: obj.pad + wp, Bottom: obj.pad + hp}
		if err := p.groupTexts(ctx, d, s, box, k); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// hasPicture reports whether the shape sh is a picture frame with a
// picture.
func hasPicture(sh *odraw.Shape) bool {
	pib, _ := sh.Props.Get(odraw.PropPib)
	return sh.Type == odraw.SptPictureFrame && pib > 0
}

// boxParagraphs returns the paragraphs of the text of a text box, in the
// range r of the characters. The mark at the end of the text shows
// nothing.
func (p *parser) boxParagraphs(ctx context.Context, r [2]uint32) ([]paragraph, error) {
	paras, err := p.paragraphsIn(ctx, r[0], r[1])
	if err != nil {
		return nil, err
	}
	for len(paras) > 1 && len(paras[len(paras)-1].runs) == 0 && !paras[len(paras)-1].pap.inTable {
		paras = paras[:len(paras)-1]
	}
	return paras, nil
}

// groupTexts adds the shapes of the group g that hold text to d, with
// their boxes from the box r of the group, as the object of the group
// places them. k is 0 for the main document and 1 for the headers.
func (p *parser) groupTexts(ctx context.Context, d *drawn, g *odraw.Shape, r odraw.Rect, k int) error {
	var err error
	odraw.Boxes(g, r, func(sh *odraw.Shape, cr odraw.Rect) {
		t, ok := p.txbxs[k][sh.SPID]
		if !ok || err != nil || len(d.texts) >= p.lim.MaxObjects {
			return
		}
		var paras []paragraph
		if paras, err = p.boxParagraphs(ctx, t); err == nil {
			d.texts = append(d.texts, groupText{r: normalized(cr), shape: sh, paras: paras})
		}
	})
	return err
}

// normalized returns r with its left before its right and its top above
// its bottom.
func normalized(r odraw.Rect) odraw.Rect {
	return odraw.Rect{
		Left: min(r.Left, r.Right), Top: min(r.Top, r.Bottom),
		Right: max(r.Left, r.Right), Bottom: max(r.Top, r.Bottom),
	}
}

// shapeObj is the object of the layout of a shape: its URL, and the bleed
// around the box of the shape in points.
type shapeObj struct {
	name string
	pad  float64
}

// shapeObject adds the object of the layout that draws the shape or the
// group top, in a box of w by h points, with the pictures pics. The object
// is larger than the box by its bleed on each side, which holds the lines,
// the arrowheads, the corners of a turn and the shadows that pass the box.
// It returns false when the shape draws nothing.
func (p *parser) shapeObject(ctx context.Context, top *odraw.Shape, w, h float64, pics *odraw.Pictures) (shapeObj, bool) {
	if p.objects == nil {
		p.objects = map[string]layout.Object{}
	}
	if len(p.objects) >= p.lim.MaxObjects {
		return shapeObj{}, false
	}
	st := p.style(ctx, pics)
	pad := 0.0
	odraw.Boxes(top, odraw.Rect{Right: w, Bottom: h}, func(sh *odraw.Shape, r odraw.Rect) {
		pad = max(pad, st.Bleed(odraw.Chain{&sh.Props}, r))
	})
	pad = math.Ceil(min(pad, 1000))
	scene := drawingml.NewScene(p.cfg)
	n := 0
	odraw.Boxes(top, odraw.Rect{Left: pad, Top: pad, Right: pad + w, Bottom: pad + h}, func(sh *odraw.Shape, r odraw.Rect) {
		c := odraw.Chain{&sh.Props}
		if n >= p.lim.MaxObjects || odraw.Hidden(c) {
			return
		}
		if f := st.Figure(sh, c, r); f != nil && len(f.Paths) > 0 && (f.Fill.Visible() || f.Line.Visible() || f.Shadow != nil) {
			scene.AddFigure(f)
			n++
		}
	})
	if n == 0 {
		return shapeObj{}, false
	}
	obj := layout.NewListObject(w+2*pad, h+2*pad, func(ctx context.Context, dev display.Device) error {
		return scene.Draw(ctx, dev, okage.TransformFromScale(shapeScale, shapeScale))
	})
	name := "shape" + strconv.Itoa(len(p.objects)+1)
	p.objects[name] = obj
	return shapeObj{name: layout.ObjectURL(name), pad: pad}, true
}

// plainPicture returns the picture of a picture frame of w by h points
// whose file the layout can draw as an image: a picture without a crop, a
// turn, a flip, a line, a shadow or a transparent color. A raster picture
// whose colors change takes a file of its own, which it decodes with ctx.
// It adds the file to the media. It returns nil for another shape.
func (p *parser) plainPicture(ctx context.Context, sh *odraw.Shape, pics *odraw.Pictures, w, h float64) *picture {
	c := odraw.Chain{&sh.Props}
	pib := int(c.Value(odraw.PropPib, 0))
	if sh.IsGroup() || sh.Type != odraw.SptPictureFrame || pib <= 0 || sh.Flags&(odraw.FSPFlipH|odraw.FSPFlipV) != 0 {
		return nil
	}
	for _, id := range []uint16{odraw.PropRotation, odraw.PropCropTop, odraw.PropCropBottom, odraw.PropCropLeft, odraw.PropCropRight, odraw.PropPictureTransparent} {
		if v, ok := c.Get(id); ok && v != 0 {
			return nil
		}
	}
	st := odraw.Style{}
	if l := st.Line(c, false); l.Visible() || st.Shadow(c) != nil {
		return nil
	}
	data := pics.Data(pib)
	if r := odraw.PictureRecolor(c); !r.Zero() {
		// a raster picture with its colors changed is a file of its own
		data = pics.Recolored(ctx, pib, r)
	}
	if data == nil {
		return nil
	}
	name := "media/" + strconv.Itoa(len(p.media)+1) + mediaExt(data)
	p.media = append(p.media, media{name: name, data: data})
	return &picture{name: name, width: w, height: h}
}

// mediaExt returns the extension of the file of a picture.
func mediaExt(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		return ".png"
	case bytes.HasPrefix(data, []byte("\xff\xd8")):
		return ".jpg"
	case bytes.HasPrefix(data, []byte("BM")):
		return ".bmp"
	case bytes.HasPrefix(data, []byte("II*\x00")), bytes.HasPrefix(data, []byte("MM\x00*")):
		return ".tif"
	case len(data) >= 44 && le.Uint32(data) == 1 && string(data[40:44]) == " EMF":
		return ".emf"
	}
	return ".wmf"
}

// writeDrawn writes a shape that floats on the page: a text box as a block
// with its text over the object of its shape, or the object or the image
// of a shape.
func (p *parser) writeDrawn(b *strings.Builder, d *drawn) {
	f := d.floating
	pg := p.page
	if pg.width == 0 {
		pg = letter
	}
	w, h := f.pic.width, f.pic.height
	// an aligned shape takes its place from the box of its origin
	refW, refH := float64(pg.width-pg.left-pg.right)/20, float64(pg.height-pg.top-pg.bottom)/20
	if f.bx == 1 {
		refW = float64(pg.width) / 20
	}
	if f.by == 1 {
		refH = float64(pg.height) / 20
	}
	switch d.posH {
	case 1:
		f.left = 0
	case 2:
		f.left = int((refW - w) / 2 * 20)
	case 3:
		f.left = int((refW - w) * 20)
	}
	if f.by != 2 {
		switch d.posV {
		case 1:
			f.top = 0
		case 2:
			f.top = int((refH - h) / 2 * 20)
		case 3:
			f.top = int((refH - h) * 20)
		}
	}
	if d.text {
		p.writeTextbox(b, d, &f)
		return
	}
	// the object of a shape is larger than the shape by its bleed
	pad := f.pic.pad
	f.left, f.top = f.left-int(math.Round(pad*20)), f.top-int(math.Round(pad*20))
	w, h = w+2*pad, h+2*pad
	style := p.floatStyle(&f, w, h)
	if len(d.texts) > 0 {
		// the text boxes of a group are on the object of the group
		if !slices.Contains(style, "position: absolute") {
			style = append(style, "position: relative")
		}
		b.WriteString(`<div style="` + attr(strings.Join(style, "; ")) + `">`)
		b.WriteString(`<img src="` + attr(f.pic.name) + `" alt="" style="display: block; width: ` + num(w) + `pt; height: ` + num(h) + `pt"/>`)
		for _, t := range d.texts {
			box := fmt.Sprintf("position: absolute; left: %spt; top: %spt; width: %spt; height: %spt; ",
				num(t.r.Left), num(t.r.Top), num(t.r.Right-t.r.Left), num(t.r.Bottom-t.r.Top))
			b.WriteString(`<div style="` + attr(box+strings.Join(textInsets(t.shape, 0), "; ")) + `">`)
			b.WriteString(p.body(t.paras, nil))
			b.WriteString("</div>")
		}
		b.WriteString("</div>")
		return
	}
	b.WriteString(`<img src="` + attr(f.pic.name) + `" alt="" style="` + attr(strings.Join(style, "; ")) + `"/>`)
}

// writeTextbox writes a text box that floats at f: a block with its text,
// whose background is the object of its shape. The block is larger than
// the text box by the bleed of the object, and its padding holds the
// bleed.
func (p *parser) writeTextbox(b *strings.Builder, d *drawn, f *floating) {
	pad := 0.0
	if d.obj != "" {
		pad = f.pic.pad
	}
	f.left, f.top = f.left-int(math.Round(pad*20)), f.top-int(math.Round(pad*20))
	style := p.floatStyle(f, f.pic.width+2*pad, f.pic.height+2*pad)
	style = append(style, textInsets(d.shape, pad)...)
	if d.obj != "" {
		style = append(style, "background: url("+d.obj+") no-repeat", "background-size: 100% 100%")
	}
	b.WriteString(`<div style="` + attr(strings.Join(style, "; ")) + `">`)
	b.WriteString(p.body(d.paras, nil))
	b.WriteString("</div>")
}

// textInsets returns the style of the box of the text of the shape s: the
// insets of its text, and pad more, as the padding of a box that holds
// them.
func textInsets(s *odraw.Shape, pad float64) []string {
	c := odraw.Chain{&s.Props}
	inset := func(id uint16, def uint32) string {
		return num(float64(int32(c.Value(id, def)))/odraw.EMUPerPoint + pad)
	}
	return []string{"box-sizing: border-box", "overflow: visible",
		"padding: " + inset(odraw.PropTextTop, 45720) + "pt " + inset(odraw.PropTextRight, 91440) + "pt " +
			inset(odraw.PropTextBottom, 45720) + "pt " + inset(odraw.PropTextLeft, 91440) + "pt"}
}

// attr returns s escaped for an attribute.
func attr(s string) string {
	return strings.NewReplacer("&", "&amp;", `"`, "&#34;", "<", "&lt;", ">", "&gt;").Replace(s)
}
