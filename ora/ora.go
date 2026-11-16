// Package ora opens OpenRaster and Krita files.
//
// Both formats are zip archives that hold the layers of an image and the
// merged image, mergedimage.png. A document has one page, the merged image,
// and gives the thumbnail of the file as its preview (D11). The package also
// registers both formats with the package image, so that image.Decode reads
// the merged image.
package ora

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"iter"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/internal/zipdoc"
)

// The mime types that the mimetype entry of a file declares.
const (
	mimeORA   = "image/openraster"
	mimeKrita = "application/x-krita"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "ora",
	Mimes: []string{mimeORA, mimeKrita},
	Exts:  []string{"ora", "kra"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// zipHeader matches the local header of the first entry of a zip archive, up
// to its name, when the entry has no extra field.
const zipHeader = "PK\x03\x04??????????????????????\x08\x00\x00\x00"

func init() {
	image.RegisterFormat("ora", zipHeader+"mimetype"+mimeORA, Decode, DecodeConfig)
	image.RegisterFormat("kra", zipHeader+"mimetype"+mimeKrita, Decode, DecodeConfig)
}

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

// Document is an open OpenRaster or Krita file.
type Document struct {
	archive *zipdoc.Archive
	mime    string
	page    *raster.Page
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
)

// Open opens an OpenRaster or a Krita file. It returns an error that wraps
// ivy.ErrFormat if src is neither.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening ora: %w", err)
	}
	archive, err := zipdoc.Open(src, o.cfg.Limits)
	if err != nil {
		return nil, err
	}
	f := archive.Find("mimetype")
	if f == nil {
		return nil, fmt.Errorf("zip archive without mimetype: %w", ivy.ErrFormat)
	}
	buf, err := archive.Read(f)
	if err != nil {
		return nil, err
	}
	d := &Document{archive: archive, mime: strings.TrimSpace(string(buf))}
	if d.mime != mimeORA && d.mime != mimeKrita {
		return nil, fmt.Errorf("mime type %q: %w", d.mime, ivy.ErrFormat)
	}
	merged := archive.Find("mergedimage.png")
	if merged == nil {
		return nil, fmt.Errorf("%s without mergedimage.png: %w", d.mime, ivy.ErrFormat)
	}
	size, err := archive.Config(merged)
	if err != nil {
		return nil, err
	}
	if err := raster.CheckPixels(size, o.cfg.Limits); err != nil {
		return nil, err
	}
	d.page = raster.NewPage(size, func(ctx context.Context) (image.Image, error) {
		return archive.Decode(ctx, merged)
	})
	return d, nil
}

// Mime returns the mime type that the file declares: "image/openraster" or
// "application/x-krita".
func (d *Document) Mime() string {
	return d.mime
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. A file has one page.
func (d *Document) PageCount(context.Context) (int, error) {
	return 1, nil
}

// Page satisfies the ivy.Pager interface. Page 0 is the merged image.
func (d *Document) Page(_ context.Context, n int) (ivy.Page, error) {
	if n != 0 {
		return nil, ivy.ErrPageRange
	}
	return d.page, nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail of
// an OpenRaster file, or the preview of a Krita file.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.archive.Previews(ctx,
		zipdoc.Candidate{Kind: ivy.Thumbnail, Name: "Thumbnails/thumbnail.png"},
		zipdoc.Candidate{Kind: ivy.Thumbnail, Name: "preview.png"},
	)
}

// Decode reads an OpenRaster or a Krita file from r, and returns its merged
// image. It reads at most ivy.DefaultMaxMemory bytes.
func Decode(r io.Reader) (image.Image, error) {
	d, err := openReader(r)
	if err != nil {
		return nil, err
	}
	return d.archive.Decode(context.Background(), d.archive.Find("mergedimage.png"))
}

// DecodeConfig returns the size of the merged image of the OpenRaster or
// Krita file in r, and the color model of a PNG image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	d, err := openReader(r)
	if err != nil {
		return image.Config{}, err
	}
	w, h := d.page.Size()
	return image.Config{Width: int(w), Height: int(h), ColorModel: color.NRGBAModel}, nil
}

// openReader reads r to its end, and opens it.
func openReader(r io.Reader) (*Document, error) {
	buf, err := raster.ReadAll(r, ivy.DefaultMaxMemory)
	if err != nil {
		return nil, err
	}
	return Open(context.Background(), bytes.NewReader(buf))
}
