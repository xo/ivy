package drawingml

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"sync"
	"time"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/layout"
)

// Scene is the content of a page whose marks have fixed positions, such as
// a slide: its shapes, its pictures and its text, in the order that they
// draw. A format builds it once, and draws it for each render. A Scene is
// safe for concurrent use after it is built: each render draws with a
// canvas of its own.
//
// The text of a scene goes through the layout of D10, which lays out each
// text body as a small document of HTML and CSS of the size of its box. So
// the text has the line breaks, the shaping and the fonts of the ebooks,
// with ivy.Config.Fonts and the substitutes of internal/font (D24).
type Scene struct {
	cfg ivy.Config
	ops []sceneOp
}

// sceneOp is one drawing operation of a scene.
type sceneOp func(c *Canvas) error

// NewScene returns an empty scene, whose text uses the fonts and the limits
// of cfg.
func NewScene(cfg ivy.Config) *Scene {
	return &Scene{cfg: cfg}
}

// Len returns the number of operations of the scene.
func (s *Scene) Len() int {
	return len(s.ops)
}

// Canvas is where a scene draws: a device, and the transform from the space
// of the scene onto the pixels under it.
type Canvas struct {
	ctx context.Context
	// dev is the device. Its user space is the space of the scene.
	dev display.Device
	// base maps the space of the scene onto the pixels of the device.
	base okage.Transform
	// errs holds the errors of the operations that drew in part.
	errs []error
}

// Render draws the scene onto dst. t maps the space of the scene onto the
// pixels of dst. It returns an *ivy.PartialError when it drew the scene in
// part, and the error of ctx when ctx ends.
func (s *Scene) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error {
	return s.Draw(ctx, display.NewOkage(dst, t), t)
}

// Draw draws the scene onto dev, whose user space is the space of the
// scene. t maps the space of the scene onto the pixels of the device, and
// sets the resolution of the pictures and the text. Every part of the
// scene, the text too, goes to dev as commands of the display list, so a
// device that records the commands holds all of them.
func (s *Scene) Draw(ctx context.Context, dev display.Device, t okage.Transform) (err error) {
	c := &Canvas{ctx: ctx, dev: dev, base: t}
	defer func() {
		if r := recover(); r != nil {
			err = &ivy.PartialError{Err: fmt.Errorf("drawing: panic: %v: %w", r, ivy.ErrFormat)}
		}
	}()
	for _, o := range s.ops {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("drawing: %w", err)
		}
		if err := o(c); err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("drawing: %w", ctx.Err())
			}
			c.errs = append(c.errs, err)
		}
	}
	if len(c.errs) > 0 {
		return &ivy.PartialError{Err: fmt.Errorf("drawing: %d of %d parts drawn in part: %w", len(c.errs), len(s.ops), errors.Join(c.errs...))}
	}
	return nil
}

// scale returns the number of pixels for a unit of the space m, which maps
// onto the space of the scene.
func (c *Canvas) scale(m okage.Transform) float64 {
	t := c.base.PreConcat(m)
	det := math.Abs(float64(t.SX)*float64(t.SY) - float64(t.KX)*float64(t.KY))
	if !(det > 0) || math.IsInf(det, 0) {
		return 1
	}
	return math.Sqrt(det)
}

// Add adds an operation that draws onto the device of the canvas, in the
// space m. f draws in the space m: the canvas sets it before and restores
// it after. scale is the number of pixels for a unit of the space m.
func (s *Scene) Add(m okage.Transform, f func(ctx context.Context, dev display.Device, scale float64) error) {
	s.ops = append(s.ops, func(c *Canvas) error {
		c.dev.Save()
		defer c.dev.Restore()
		c.dev.Concat(m)
		return f(c.ctx, c.dev, c.scale(m))
	})
}

// AddHTML adds a document of HTML, which the layout lays out on a page of w
// by h points and draws with the transform m from the page onto the space
// of the scene. The layout happens once, on the first draw.
func (s *Scene) AddHTML(doc string, w, h float64, m okage.Transform) {
	if !(w > 0 && h > 0) || math.IsInf(w, 0) || math.IsInf(h, 0) {
		return
	}
	b := &htmlBlock{doc: doc, cfg: s.cfg}
	s.ops = append(s.ops, func(c *Canvas) error {
		// the device takes the transform of the page, and the book renders
		// at the pixels of a unit of the scene
		if err := b.book().Draw(c.ctx, 0, c.dev, m, c.scale(okage.IdentityTransform())); err != nil {
			return fmt.Errorf("drawing text: %w", err)
		}
		return nil
	})
}

// htmlBlock is a document of HTML of a scene, with the book that lays it
// out.
type htmlBlock struct {
	doc  string
	cfg  ivy.Config
	once sync.Once
	b    *layout.Book
}

// htmlName is the name of the document of a block in its file system.
const htmlName = "text.html"

// book returns the book of the document. The book lays out the document
// once, on the first draw. A layout that the context stops is done again
// on the next draw.
func (b *htmlBlock) book() *layout.Book {
	b.once.Do(func() {
		b.b = layout.NewBook(layout.Source{
			FS:        htmlFS(b.doc),
			Documents: []string{htmlName},
		}, layout.BookOptions{
			Limits:      b.cfg.Limits,
			Logger:      b.cfg.Logger,
			FontFiles:   b.cfg.Fonts,
			SystemFonts: b.cfg.SystemFonts,
		})
	})
	return b.b
}

// htmlFS is a file system that holds one document of HTML, at htmlName.
type htmlFS string

// The interfaces that an htmlFS implements.
var _ fs.ReadFileFS = htmlFS("")

// Open satisfies the fs.FS interface.
func (f htmlFS) Open(name string) (fs.File, error) {
	if name != htmlName {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &htmlFile{Reader: bytes.NewReader([]byte(f)), size: int64(len(f))}, nil
}

// ReadFile satisfies the fs.ReadFileFS interface.
func (f htmlFS) ReadFile(name string) ([]byte, error) {
	if name != htmlName {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return []byte(f), nil
}

// htmlFile is the open document of an htmlFS.
type htmlFile struct {
	*bytes.Reader
	size int64
}

// Stat satisfies the fs.File interface.
func (f *htmlFile) Stat() (fs.FileInfo, error) {
	return htmlInfo{size: f.size}, nil
}

// Close satisfies the fs.File interface.
func (f *htmlFile) Close() error {
	return nil
}

// htmlInfo is the information of the document of an htmlFS.
type htmlInfo struct {
	size int64
}

// The interfaces that an htmlInfo implements.
var _ fs.FileInfo = htmlInfo{}

// Name satisfies the fs.FileInfo interface.
func (i htmlInfo) Name() string { return htmlName }

// Size satisfies the fs.FileInfo interface.
func (i htmlInfo) Size() int64 { return i.size }

// Mode satisfies the fs.FileInfo interface.
func (i htmlInfo) Mode() fs.FileMode { return 0o444 }

// ModTime satisfies the fs.FileInfo interface.
func (i htmlInfo) ModTime() time.Time { return time.Time{} }

// IsDir satisfies the fs.FileInfo interface.
func (i htmlInfo) IsDir() bool { return false }

// Sys satisfies the fs.FileInfo interface.
func (i htmlInfo) Sys() any { return nil }

// The interfaces that an htmlFile implements.
var _ io.ReaderAt = (*htmlFile)(nil)
