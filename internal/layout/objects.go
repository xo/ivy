// This file is not a port. It adds two kinds of images that upstream does not
// have, for the Office formats of phase 5 (D24): the metafiles of Windows,
// EMF and WMF, which internal/metafile draws, and the objects of a Source,
// which a format draws itself, such as a chart.

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
