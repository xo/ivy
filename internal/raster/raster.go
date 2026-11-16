// Package raster decodes raster images within the limits of a configuration,
// and turns a raster image into an ivy.Page.
//
// It registers the PNG, JPEG and GIF decoders of the standard library with
// the package image. A format that needs another decoder, such as TIFF or
// WebP, imports that decoder itself, so that only it links the decoder (D6).
package raster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"  // the GIF decoder, for image.Decode
	_ "image/jpeg" // the JPEG decoder, for image.Decode
	_ "image/png"  // the PNG decoder, for image.Decode
	"io"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/okage"
)

// ReadAll reads r to its end. It returns an error that wraps ivy.ErrLimit if
// r holds more than limit bytes.
func ReadAll(r io.Reader, limit int64) ([]byte, error) {
	buf, err := io.ReadAll(io.LimitReader(r, limit+1))
	switch {
	case err != nil:
		return nil, fmt.Errorf("reading: %w", err)
	case int64(len(buf)) > limit:
		return nil, fmt.Errorf("reading more than %d bytes: %w", limit, ivy.ErrLimit)
	}
	return buf, nil
}

// Config returns the size of the encoded image in buf, without a decode of
// its pixels.
func Config(buf []byte) (image.Point, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		return image.Point{}, fmt.Errorf("reading image header: %w: %w", ivy.ErrFormat, err)
	}
	return image.Pt(cfg.Width, cfg.Height), nil
}

// Decode decodes the encoded image in buf. It checks the size of the image
// against lim.MaxPixels before it decodes the pixels.
func Decode(ctx context.Context, buf []byte, lim ivy.Limits) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	size, err := Config(buf)
	if err != nil {
		return nil, err
	}
	if err := CheckPixels(size, lim); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w: %w", ivy.ErrFormat, err)
	}
	return img, nil
}

// CheckPixels returns an error that wraps ivy.ErrLimit if an image of size
// has more pixels than lim.MaxPixels allows, and ivy.ErrFormat if the size
// is not positive.
func CheckPixels(size image.Point, lim ivy.Limits) error {
	lim = lim.WithDefaults()
	switch {
	case size.X <= 0 || size.Y <= 0:
		return fmt.Errorf("image of %dx%d: %w", size.X, size.Y, ivy.ErrFormat)
	case int64(size.X)*int64(size.Y) > lim.MaxPixels:
		return fmt.Errorf("image of %dx%d: %w", size.X, size.Y, ivy.ErrLimit)
	}
	return nil
}

// Page is an ivy.Page for a raster image. It has one point for each pixel.
// It decodes the image on its first render, and keeps it.
type Page struct {
	size image.Point
	load func(context.Context) (image.Image, error)

	mu  sync.Mutex
	pix *okage.Pixmap
}

// NewPage returns a page of the given size in pixels. load decodes the image.
func NewPage(size image.Point, load func(context.Context) (image.Image, error)) *Page {
	return &Page{size: size, load: load}
}

// Size satisfies the ivy.Page interface.
func (p *Page) Size() (float32, float32) {
	return float32(p.size.X), float32(p.size.Y)
}

// Render satisfies the ivy.Page interface. It draws the image with bilinear
// filtering.
func (p *Page) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error {
	pix, err := p.pixmap(ctx)
	if err != nil {
		return err
	}
	paint := okage.DefaultPixmapPaint()
	paint.Quality = okage.FilterQualityBilinear
	dst.DrawPixmap(0, 0, pix.AsRef(), &paint, t, nil)
	return nil
}

// pixmap decodes the image on the first call, and converts it to a pixmap.
// A failed decode is not kept, so that a call with a new context can try
// again.
func (p *Page) pixmap(ctx context.Context) (*okage.Pixmap, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pix != nil {
		return p.pix, nil
	}
	img, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	pix, err := Pixmap(img)
	if err != nil {
		return nil, err
	}
	p.pix = pix
	return pix, nil
}

// Pixmap converts img to a pixmap, with its premultiplied pixels.
func Pixmap(img image.Image) (*okage.Pixmap, error) {
	b := img.Bounds()
	if b.Empty() {
		return nil, fmt.Errorf("empty image: %w", ivy.ErrFormat)
	}
	rgba, ok := img.(*image.RGBA)
	if !ok || rgba.Stride != 4*b.Dx() || b.Min != (image.Point{}) {
		rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	}
	size, ok := okage.IntSizeFromWH(uint32(b.Dx()), uint32(b.Dy()))
	if !ok {
		return nil, fmt.Errorf("image of %v: %w", b.Size(), ivy.ErrLimit)
	}
	pix, ok := okage.PixmapFromVec(rgba.Pix, size)
	if !ok {
		return nil, errors.New("converting image to pixmap")
	}
	return pix, nil
}
