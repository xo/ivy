// Package psd opens Photoshop files, PSD and PSB.
//
// A document has one page: the merged image that Photoshop writes next to the
// layers. A file that Photoshop saved without the option "Maximize
// Compatibility" holds no merged image, and then the page is a composite of
// the visible layers, with their opacity, in the normal blend mode. The blend
// modes, the masks and the effects of the layers are left out (D5). The spot
// channels of the merged image draw over it in the color of their ink, as
// Photoshop shows them, and the alpha channels, which hold a selection, do
// not draw. A document
// gives the JPEG thumbnail of the file as its preview (D11).
//
// The decoder is a copy of github.com/oov/psd, in internal/oovpsd, with a fix
// for the files of 16 and 32 bits. The package registers PSD and PSB with the
// package image, so that image.Decode returns the page as an *image.NRGBA.
package psd

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"iter"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/psd/internal/oovpsd"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "psd",
	Mimes: []string{"image/vnd.adobe.photoshop", "image/x-photoshop", "application/x-photoshop"},
	Exts:  []string{"psd", "psb"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

func init() {
	image.RegisterFormat("psd", "8BPS\x00\x01", Decode, DecodeConfig)
	image.RegisterFormat("psb", "8BPS\x00\x02", Decode, DecodeConfig)
}

// The image resources that the package reads.
const (
	// resThumbnail holds the JPEG thumbnail of Photoshop 5 and later.
	resThumbnail = 0x040c
	// resVersion holds whether the file holds a merged image.
	resVersion = 0x0421
)

// thumbnailHeader is the length of the header of the thumbnail resource,
// before its JPEG.
const thumbnailHeader = 28

// Option is an option of Open.
type Option func(*options)

type options struct {
	cfg ivy.Config
}

// WithConfig sets the configuration that every format shares.
func WithConfig(cfg ivy.Config) Option {
	return func(o *options) {
		o.cfg = cfg
	}
}

// Document is an open Photoshop file.
type Document struct {
	src  ivy.Source
	lim  ivy.Limits
	cfg  oovpsd.Config
	page *raster.Page
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
)

// Open opens a PSD or a PSB file. It reads the header and the image resources,
// and decodes the image when the page renders. It returns an error that wraps
// ivy.ErrFormat if src is not a Photoshop file, and ivy.ErrLimit if the image
// exceeds the limits.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening psd: %w", err)
	}
	d := &Document{src: src, lim: o.cfg.Limits.WithDefaults()}
	cfg, _, err := oovpsd.DecodeConfig(io.NewSectionReader(src, 0, src.Size()))
	if err != nil {
		return nil, fmt.Errorf("reading psd header: %w: %w", ivy.ErrFormat, err)
	}
	if err := checkConfig(cfg, d.lim); err != nil {
		return nil, err
	}
	d.cfg = cfg
	open := func() io.Reader { return io.NewSectionReader(src, 0, src.Size()) }
	d.page = raster.NewPage(cfg.Rect.Size(), func(ctx context.Context) (image.Image, error) {
		return decode(ctx, open, d.lim)
	})
	return d, nil
}

// checkConfig checks the number of channels against the color mode, and the
// size of the image against the limits. The merged image takes a plane of
// bytes for each channel.
func checkConfig(cfg oovpsd.Config, lim ivy.Limits) error {
	if n := cfg.ColorMode.Channels(); cfg.Channels < n {
		return fmt.Errorf("psd of %d channels in a color mode of %d: %w", cfg.Channels, n, ivy.ErrFormat)
	}
	if err := raster.CheckPixels(cfg.Rect.Size(), lim); err != nil {
		return err
	}
	plane := (int64(cfg.Rect.Dx())*int64(cfg.Depth) + 7) / 8 * int64(cfg.Rect.Dy())
	if n := plane * int64(cfg.Channels); n > lim.MaxMemory {
		return fmt.Errorf("psd image of %d bytes: %w", n, ivy.ErrLimit)
	}
	return nil
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. A file has one page.
func (d *Document) PageCount(context.Context) (int, error) {
	return 1, nil
}

// Page satisfies the ivy.Pager interface. Page 0 is the image.
func (d *Document) Page(_ context.Context, n int) (ivy.Page, error) {
	if n != 0 {
		return nil, ivy.ErrPageRange
	}
	return d.page, nil
}

// Previews satisfies the ivy.Previewer interface. It lists the JPEG thumbnail
// of the file, where the file holds one.
func (d *Document) Previews(context.Context) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		res, ok := d.cfg.Res[resThumbnail]
		// format 1 is a JPEG of RGB
		if !ok || len(res.Data) <= thumbnailHeader || binary.BigEndian.Uint32(res.Data) != 1 {
			return
		}
		jpg := res.Data[thumbnailHeader:]
		size, _ := raster.Config(jpg)
		yield(ivy.Preview{
			Kind: ivy.Thumbnail,
			Size: size,
			Decode: func(ctx context.Context) (image.Image, error) {
				return raster.Decode(ctx, jpg, d.lim)
			},
		}, nil)
	}
}

// Decode reads a PSD or a PSB file from r, and returns its image as an
// *image.NRGBA. It reads at most ivy.DefaultMaxMemory bytes.
func Decode(r io.Reader) (image.Image, error) {
	buf, err := raster.ReadAll(r, ivy.DefaultMaxMemory)
	if err != nil {
		return nil, err
	}
	open := func() io.Reader { return bytes.NewReader(buf) }
	return decode(context.Background(), open, ivy.Limits{}.WithDefaults())
}

// DecodeConfig returns the size of the image of the PSD or PSB file in r.
func DecodeConfig(r io.Reader) (image.Config, error) {
	cfg, _, err := oovpsd.DecodeConfig(r)
	if err != nil {
		return image.Config{}, fmt.Errorf("reading psd header: %w: %w", ivy.ErrFormat, err)
	}
	return image.Config{Width: cfg.Rect.Dx(), Height: cfg.Rect.Dy(), ColorModel: color.NRGBAModel}, nil
}

// decode decodes the image of the file that open reads: the merged image, or
// a composite of the layers if the file holds no merged image. open returns a
// reader from the start of the file each time.
func decode(ctx context.Context, open func() io.Reader, lim ivy.Limits) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("decoding psd: %w", err)
	}
	merged := true
	p, _, err := oovpsd.Decode(open(), &oovpsd.DecodeOptions{
		ConfigLoaded: func(cfg oovpsd.Config) error {
			merged = hasMergedImage(cfg)
			return checkConfig(cfg, lim)
		},
		SkipLayerImage: true,
	})
	if err != nil {
		return nil, decodeError(err)
	}
	if merged {
		img := toNRGBA(p.Picker)
		if err := drawSpots(ctx, img, p); err != nil {
			return nil, err
		}
		return img, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("decoding psd layers: %w", err)
	}
	p, _, err = oovpsd.Decode(open(), &oovpsd.DecodeOptions{SkipMergedImage: true, MaxMemory: lim.MaxMemory})
	if err != nil {
		return nil, decodeError(err)
	}
	return composite(p), nil
}

// hasMergedImage reports whether the file holds a merged image. The version
// resource of Photoshop says so in its fifth byte, and a file without the
// resource holds one.
func hasMergedImage(cfg oovpsd.Config) bool {
	res, ok := cfg.Res[resVersion]
	return !ok || len(res.Data) < 5 || res.Data[4] != 0
}

// decodeError wraps an error of the decoder. An error that already wraps a
// sentinel error of ivy, such as one of the limits, keeps it, and any other
// error is a malformed file.
func decodeError(err error) error {
	if errors.Is(err, oovpsd.ErrLayerMemory) {
		return fmt.Errorf("decoding psd layers: %w: %w", ivy.ErrLimit, err)
	}
	for _, sentinel := range []error{ivy.ErrLimit, ivy.ErrFormat, ivy.ErrUnsupported} {
		if errors.Is(err, sentinel) {
			return err
		}
	}
	return fmt.Errorf("decoding psd: %w: %w", ivy.ErrFormat, err)
}

// composite draws the visible layers of p, from the bottom up, with their
// opacity, in the normal blend mode.
func composite(p *oovpsd.PSD) *image.NRGBA {
	dst := image.NewNRGBA(p.Config.Rect)
	var drawLayers func(layers []oovpsd.Layer)
	drawLayers = func(layers []oovpsd.Layer) {
		for i := range layers {
			l := &layers[i]
			switch {
			case !l.Visible():
			case l.Folder():
				drawLayers(l.Layer)
			case l.Picker != nil && !l.Rect.Empty():
				mask := image.NewUniform(color.Alpha{A: l.Opacity})
				draw.DrawMask(dst, l.Rect, l.Picker, l.Rect.Min, mask, image.Point{}, draw.Over)
			}
		}
	}
	drawLayers(p.Layer)
	return dst
}

// toNRGBA converts img to an *image.NRGBA. The pickers of the decoder convert
// each pixel when it is asked for, so the conversion runs once, here.
func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}
	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}
