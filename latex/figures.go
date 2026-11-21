package latex

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"sync"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/latex/internal/engine"
	"github.com/xo/ivy/pdf"
	"github.com/xo/ivy/ps"
)

// figures draws the figures of a document with the decoders of ivy: a
// raster image as pixels, and a PDF or an EPS figure as vectors, with the
// first page of the packages pdf and ps (D62). It keeps each figure that it
// decoded, so that the bands of a page decode it once.
type figures struct {
	cfg  ivy.Config
	mu   sync.Mutex
	figs map[*byte]*figure
}

// figure is a decoded figure.
type figure struct {
	once sync.Once
	img  image.Image
	page ivy.Page
	err  error
}

// newFigures returns the figures of a document.
func newFigures(cfg ivy.Config) *figures {
	return &figures{cfg: cfg, figs: make(map[*byte]*figure)}
}

// The interface that draws the figures of the engine.
var _ engine.Figures = (*figures)(nil)

// DrawFigure satisfies the engine.Figures interface.
func (fs *figures) DrawFigure(ctx context.Context, d display.Device, data []byte, kind engine.FigureKind, w, h, res float32) error {
	if len(data) == 0 {
		return nil
	}
	fs.mu.Lock()
	fig, ok := fs.figs[&data[0]]
	if !ok {
		fig = new(figure)
		fs.figs[&data[0]] = fig
	}
	fs.mu.Unlock()
	fig.once.Do(func() {
		fig.img, fig.page, fig.err = fs.decode(ctx, data, kind)
	})
	if fig.err != nil {
		return fig.err
	}
	dst, ok := okage.RectFromXYWH(0, 0, w, h)
	if !ok {
		return nil
	}
	if fig.img != nil {
		d.DrawImage(fig.img, display.RectTransform(fig.img.Bounds().Size(), dst), display.ImageOptions{Opacity: 1, Smooth: true})
		return nil
	}
	pw, ph := fig.page.Size()
	if !(pw > 0 && ph > 0) {
		return nil
	}
	drawer, ok := fig.page.(display.Drawer)
	if !ok {
		return fmt.Errorf("latex: figure without a display list: %w", ivy.ErrUnsupported)
	}
	d.Save()
	defer d.Restore()
	d.ClipPath(okage.PathFromRect(dst), okage.FillRuleWinding)
	d.Concat(okage.TransformFromScale(w/pw, h/ph))
	return drawer.Draw(ctx, d, res*max(w/pw, h/ph))
}

// decode decodes a figure.
func (fs *figures) decode(ctx context.Context, data []byte, kind engine.FigureKind) (image.Image, ivy.Page, error) {
	lim := fs.cfg.Limits.WithDefaults()
	switch kind {
	case engine.FigurePDF:
		doc, err := pdf.Open(ctx, bytes.NewReader(data), pdf.WithConfig(fs.cfg))
		if err != nil {
			return nil, nil, fmt.Errorf("latex: PDF figure: %w", err)
		}
		page, err := doc.Page(ctx, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("latex: PDF figure: %w", err)
		}
		return nil, page, nil
	case engine.FigureEPS:
		doc, err := ps.Open(ctx, bytes.NewReader(data), ps.WithConfig(fs.cfg))
		if err != nil {
			return nil, nil, fmt.Errorf("latex: EPS figure: %w", err)
		}
		page, err := doc.Page(ctx, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("latex: EPS figure: %w", err)
		}
		return nil, page, nil
	}
	img, err := raster.Decode(ctx, data, lim)
	if err != nil {
		return nil, nil, fmt.Errorf("latex: figure: %w", err)
	}
	return img, nil, nil
}
