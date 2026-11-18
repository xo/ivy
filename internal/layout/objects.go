// This file is not a port. It adds two kinds of images that upstream does not
// have, for the Office formats of phase 5 (D24): the metafiles of Windows,
// EMF and WMF, which internal/metafile draws, and the objects of a Source,
// which a format draws itself, such as a chart. A ListObject is an object
// whose content is a display list (D51).

package layout

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"sync"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/metafile"
)

// pixelsPerPoint converts points to CSS pixels.
const pixelsPerPoint = 4.0 / 3

// MetafileImage is an image in EMF, EMF+ or WMF. The drawing plays its
// records onto the display list, as vectors.
type MetafileImage struct {
	// ID is the digest of the URL of the image.
	ID string
	// Data is the metafile.
	Data []byte

	limits  ivy.Limits
	picture *metafile.Picture

	mu sync.Mutex
	// fonts holds the picture that draws its text with the fonts of a
	// document, for the last font database that drew it.
	fonts struct {
		db      *svg.FontDatabase
		picture *metafile.Picture
	}
}

// metafile returns the image of the metafile data, or false if data is not
// a metafile.
func (il *imageLoader) metafile(url string, data []byte) (Image, bool) {
	pic, err := metafile.Open(data, metafile.Options{Config: ivy.Config{Limits: il.limits}})
	if err != nil {
		return nil, false
	}
	return &MetafileImage{
		ID:      fmt.Sprintf("%x", md5.Sum([]byte(url))),
		Data:    data,
		limits:  il.limits,
		picture: pic,
	}, true
}

// GetIntrinsicSize satisfies Image. The size of a metafile is in points.
func (m *MetafileImage) GetIntrinsicSize(_, _ float64) IntrinsicSize {
	w, h := m.picture.Size()
	return intrinsicPoints(float64(w), float64(h))
}

// intrinsicPoints returns the intrinsic size of a picture of width by height
// points. A size that is zero or less is no intrinsic size.
func intrinsicPoints(width, height float64) IntrinsicSize {
	if !(width > 0) || !(height > 0) {
		return IntrinsicSize{}
	}
	return IntrinsicSize{
		Width: width * pixelsPerPoint, Height: height * pixelsPerPoint, Ratio: width / height,
		HasWidth: true, HasHeight: true, HasRatio: true,
	}
}

// pictureFor returns the picture that draws its text with the fonts db. A
// nil db draws with the default fonts of internal/metafile.
func (m *MetafileImage) pictureFor(db *svg.FontDatabase) *metafile.Picture {
	if db == nil {
		return m.picture
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fonts.db == db {
		return m.fonts.picture
	}
	pic, err := metafile.Open(m.Data, metafile.Options{Config: ivy.Config{Limits: m.limits}, Fonts: db})
	if err != nil {
		return m.picture
	}
	m.fonts.db, m.fonts.picture = db, pic
	return pic
}

// draw draws the metafile in the rectangle (0, 0, concreteWidth,
// concreteHeight) of the current user space.
func (m *MetafileImage) draw(s *stream, concreteWidth, concreteHeight float64) {
	pic := m.pictureFor(s.dc.fonts)
	s.drawPicture("metafile "+m.ID, concreteWidth, concreteHeight, pic.Draw)
}

// ObjectImage is an object of a Source, which the format draws itself.
type ObjectImage struct {
	// Name is the name of the object in Source.Objects.
	Name   string
	Object Object
}

// object returns the image of the object name of the Source.
func (il *imageLoader) object(name string) (Image, error) {
	obj, ok := il.fetcher.Objects[name]
	if !ok || obj == nil {
		return nil, fmt.Errorf("%w: no object %q", ErrURLFetching, name)
	}
	return &ObjectImage{Name: name, Object: obj}, nil
}

// GetIntrinsicSize satisfies Image. The size of an object is in points.
func (o *ObjectImage) GetIntrinsicSize(_, _ float64) IntrinsicSize {
	return intrinsicPoints(o.Object.Size())
}

// draw draws the object in the rectangle (0, 0, concreteWidth,
// concreteHeight) of the current user space.
func (o *ObjectImage) draw(s *stream, concreteWidth, concreteHeight float64) {
	s.drawPicture("object "+o.Name, concreteWidth, concreteHeight, o.Object.Draw)
}

// ListObject is an Object whose content is a display list (D51): a box of
// a size in points, whose list the layout draws at the place of the box.
// The list draws in the space of the box, from (0, 0) to its size, with
// the y axis down. The box records the list once, on its first draw, and
// replays it on each draw, scaled to the rectangle of the box. The page
// that holds the box records the replay as it records the rest of the page
// (D47). A ListObject is safe for concurrent use.
type ListObject struct {
	width, height float64
	record        func(ctx context.Context, dev display.Device) error

	mu   sync.Mutex
	list *display.List
	err  error
}

// NewListObject returns a box of width by height points, whose list record
// draws onto dev. record must draw the same commands on each call. It
// returns an *ivy.PartialError when it draws in part.
func NewListObject(width, height float64, record func(ctx context.Context, dev display.Device) error) *ListObject {
	return &ListObject{width: width, height: height, record: record}
}

// Size satisfies the Object interface.
func (o *ListObject) Size() (float64, float64) {
	return o.width, o.height
}

// Draw satisfies the Object interface. It replays the list, scaled from the
// box onto rect.
func (o *ListObject) Draw(ctx context.Context, dev display.Device, rect okage.Rect) error {
	if !(o.width > 0) || !(o.height > 0) {
		return nil
	}
	list, err := o.recorded(ctx)
	if list == nil {
		return err
	}
	m := okage.TransformFromRow(float32(float64(rect.Width())/o.width), 0, 0, float32(float64(rect.Height())/o.height), rect.X(), rect.Y())
	if !m.IsFinite() {
		return err
	}
	dev.Save()
	defer dev.Restore()
	dev.Concat(m)
	list.Replay(dev)
	return err
}

// recorded returns the list of the box, and records it on the first call.
// A recording that ctx stops is not kept, so that a later draw records the
// list again.
func (o *ListObject) recorded(ctx context.Context) (list *display.List, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.list != nil {
		return o.list, o.err
	}
	defer func() {
		if r := recover(); r != nil {
			list, err = nil, &ivy.PartialError{Err: fmt.Errorf("recording object: panic: %v: %w", r, ivy.ErrFormat)}
		}
	}()
	list = display.NewRecording()
	err = o.record(ctx, list)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("recording object: %w", ctx.Err())
	}
	if _, ok := errors.AsType[*ivy.PartialError](err); err != nil && !ok {
		err = &ivy.PartialError{Err: err}
	}
	o.list, o.err = list, err
	return list, err
}

// drawPicture records the drawing of a picture of vectors in the rectangle
// (0, 0, width, height) of the current user space. The drawing runs when the
// stream replays its commands. A picture drawn in part draws the page in
// part.
func (s *stream) drawPicture(what string, width, height float64, draw func(ctx context.Context, d display.Device, rect okage.Rect) error) {
	if !(width > 0) || !(height > 0) || s.state.fill.alpha <= 0 {
		return
	}
	rect, ok := okage.RectFromXYWH(0, 0, float32(width), float32(height))
	if !ok {
		return
	}
	dc := s.dc
	s.record(func(d display.Device) {
		err := draw(dc.ctx, d, rect)
		if err == nil {
			return
		}
		if pe, ok := errors.AsType[*ivy.PartialError](err); ok {
			err = pe.Err
		}
		dc.log.warning("Failed to draw %s: %v", what, err)
		dc.fail(fmt.Errorf("drawing %s: %w", what, err))
	})
}
