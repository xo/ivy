package docx

import (
	"context"
	"math"
	"strconv"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/ooxml"
)

// diagramScale is the number of pixels for a point that the pictures and
// the text of a diagram take for their resolution, before the transform of
// the page.
const diagramScale = 2

// diagram writes a diagram of SmartArt of w by h points, whose
// dgm:relIds is n, as an object of the layout. Word saves the drawing of
// the diagram in a part dsp:drawing, which the extension dsp:dataModelExt
// of the data part names. internal/drawingml draws its shapes, as for a
// slide. A diagram without a drawing is left out.
func (c *converter) diagram(n roxmltree.Node, w, h float64) bool {
	if !(w > 0 && h > 0) || math.IsInf(w, 0) || math.IsInf(h, 0) {
		return false
	}
	r, ok := c.relationship(ooxml.Attr(n, "dm"))
	if !ok || r.External {
		return false
	}
	data, err := c.readXML(r.Target)
	if err != nil {
		c.log.Debug("docx: diagram left out", "part", r.Target, "err", err)
		return false
	}
	var drawingID string
	for e := range data.RootElement().Descendants() {
		if e.IsElement() && e.TagName().Name() == "dataModelExt" {
			drawingID = ooxml.Attr(e, "relId")
			break
		}
	}
	d, ok := c.relationship(drawingID)
	if drawingID == "" || !ok || d.External {
		c.log.Debug("docx: diagram without a drawing left out", "part", r.Target)
		return false
	}
	drawing, err := c.readXML(d.Target)
	if err != nil {
		c.log.Debug("docx: diagram left out", "part", d.Target, "err", err)
		return false
	}
	tree, ok := child(drawing.RootElement(), "spTree")
	if !ok {
		return false
	}
	c.objects++
	if c.objects > c.lim.MaxObjects {
		return false
	}
	obj := &sceneObject{scene: drawingml.NewScene(c.d.cfg), w: w, h: h}
	t := &drawingml.Tree{
		Ctx:      c.ctx,
		Config:   c.d.cfg,
		Scene:    obj.scene,
		Colors:   &drawingml.Colors{Theme: c.theme},
		Pictures: c.partPictures(d.Target),
	}
	t.Draw(tree, okage.IdentityTransform())
	if obj.scene.Len() == 0 {
		return false
	}
	name := "diagram" + strconv.Itoa(len(c.charts)+1)
	c.charts[name] = obj
	c.w.WriteString(`<img src="` + attrText(layout.ObjectURL(name)) + `" alt="" style="width: ` + pt(w) + `; height: ` + pt(h) + `"/>`)
	return true
}

// partPictures gives the pictures of a part to internal/drawingml.
type partPictures struct {
	c    *converter
	rels map[string]ooxml.Relationship
}

// The interfaces that a partPictures implements.
var _ drawingml.Pictures = (*partPictures)(nil)

// partPictures returns the pictures of the part name.
func (c *converter) partPictures(name string) *partPictures {
	pp := &partPictures{c: c, rels: map[string]ooxml.Relationship{}}
	rels, err := c.d.doc.Relationships(name, c.lim)
	if err != nil {
		c.log.Debug("docx: relationships left out", "part", name, "err", err)
	}
	for _, r := range rels {
		pp.rels[r.ID] = r
	}
	return pp
}

// Picture satisfies the drawingml.Pictures interface. It returns the
// picture of the part that the relationship id targets.
func (pp *partPictures) Picture(id string) *drawingml.Picture {
	r, ok := pp.rels[id]
	if !ok || r.External {
		return nil
	}
	buf, err := pp.c.d.doc.ReadPart(r.Target)
	if err != nil {
		pp.c.log.Debug("docx: picture left out", "part", r.Target, "err", err)
		return nil
	}
	return drawingml.NewPicture(buf, pp.c.d.cfg)
}

// sceneObject is a scene of internal/drawingml of w by h points, such as a
// diagram. It satisfies the interface Object of the layout.
type sceneObject struct {
	scene *drawingml.Scene
	w, h  float64
}

// Size satisfies the interface Object of the layout.
func (o *sceneObject) Size() (float64, float64) {
	return o.w, o.h
}

// Draw satisfies the interface Object of the layout. It draws the scene
// onto rect.
func (o *sceneObject) Draw(ctx context.Context, dev display.Device, rect okage.Rect) error {
	rw, rh := float64(rect.Width()), float64(rect.Height())
	if !(rw > 0) || !(rh > 0) || math.IsInf(rw, 0) || math.IsInf(rh, 0) {
		return nil
	}
	m := okage.TransformFromRow(float32(rw/o.w), 0, 0, float32(rh/o.h), rect.X(), rect.Y())
	dev.Save()
	defer dev.Restore()
	dev.Concat(m)
	return o.scene.Draw(ctx, dev, okage.TransformFromScale(diagramScale, diagramScale).PreConcat(m))
}
