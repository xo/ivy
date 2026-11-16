// Package camera opens the RAW files of digital cameras.
//
// A camera writes a JPEG of each photo into its RAW file, often at full size.
// A document gives these JPEG images as its previews (D11), and the largest
// one is the image that a person reviews. ivy does not decode the raw data of
// the sensor. A document also says the make and the model of the camera, and
// the date of the photo.
//
// The package reads CR2, NEF, NRW, ARW, SRF, SR2, DNG, ORF, RW2, PEF and SRW,
// which are forms of TIFF, RAF of Fujifilm, and CR3 of Canon, which is a form
// of ISO BMFF. It registers CR2, CR3, ORF, RW2 and RAF with the package image,
// so that image.Decode returns the largest preview. The other formats start as
// a plain TIFF file does, so the package does not register them, and a
// program opens them with Open.
package camera

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"iter"
	"slices"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name: "camera",
	Mimes: []string{
		"image/x-canon-cr2",
		"image/x-canon-cr3",
		"image/x-nikon-nef",
		"image/x-nikon-nrw",
		"image/x-sony-arw",
		"image/x-sony-srf",
		"image/x-sony-sr2",
		"image/x-adobe-dng",
		"image/x-olympus-orf",
		"image/x-panasonic-rw2",
		"image/x-pentax-pef",
		"image/x-samsung-srw",
		"image/x-fuji-raf",
	},
	Exts: []string{"cr2", "cr3", "nef", "nrw", "arw", "srf", "sr2", "dng", "orf", "rw2", "pef", "srw", "raf"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

func init() {
	for name, magic := range map[string][]string{
		"cr2": {"II*\x00\x10\x00\x00\x00CR"},
		"cr3": {"????ftypcrx "},
		"orf": {"IIRO", "IIRS", "MMOR"},
		"rw2": {"IIU\x00"},
		"raf": {rafMagic},
	} {
		for _, m := range magic {
			image.RegisterFormat(name, m, Decode, DecodeConfig)
		}
	}
}

// headerLimit is the number of bytes of a JPEG that Open reads to find its
// size. A JPEG can hold its EXIF data before its size.
const headerLimit = 256 << 10

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

// Document is an open RAW file.
type Document struct {
	src      ivy.Source
	lim      ivy.Limits
	info     ivy.Info
	previews []preview
}

// The capabilities of a document.
var (
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// preview is a JPEG in the file.
type preview struct {
	off, n int64
	size   image.Point
}

// region is a part of the file that can hold a JPEG.
type region struct {
	off, n int64
}

// Open opens a RAW file. It returns an error that wraps ivy.ErrFormat if src
// is not a RAW file that the package reads. A file that holds no JPEG that the
// package decodes opens, and lists no preview.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening camera raw: %w", err)
	}
	d := &Document{src: src, lim: o.cfg.Limits.WithDefaults()}
	head := make([]byte, 16)
	if n, _ := src.ReadAt(head, 0); n < len(head) {
		return nil, fmt.Errorf("short camera raw file: %w", ivy.ErrFormat)
	}
	var regions []region
	var err error
	switch {
	case bytes.HasPrefix(head, []byte(rafMagic)):
		regions, err = d.readRAF()
	case string(head[4:12]) == "ftypcrx ":
		regions, err = d.readCR3()
	case string(head[:2]) == "II" || string(head[:2]) == "MM":
		var t *tiffReader
		if t, err = newTIFFReader(src, 0, src.Size(), d.lim); err == nil {
			err = t.walk()
			regions = t.regions
			d.info = t.info
		}
	default:
		return nil, fmt.Errorf("not a camera raw file: %w", ivy.ErrFormat)
	}
	if err != nil {
		return nil, err
	}
	d.addPreviews(regions)
	return d, nil
}

// addPreviews keeps each region that starts a JPEG that the package image/jpeg
// decodes, largest first. A lossless JPEG, which holds the data of the sensor
// in many files, does not decode, so it is left out.
func (d *Document) addPreviews(regions []region) {
	seen := make(map[int64]bool)
	for _, r := range regions {
		if r.n <= 0 || r.off < 0 || r.off > d.src.Size() || r.n > d.src.Size()-r.off || seen[r.off] {
			continue
		}
		seen[r.off] = true
		cfg, err := jpeg.DecodeConfig(io.NewSectionReader(d.src, r.off, min(r.n, headerLimit)))
		if err != nil {
			continue
		}
		d.previews = append(d.previews, preview{off: r.off, n: r.n, size: image.Pt(cfg.Width, cfg.Height)})
	}
	slices.SortStableFunc(d.previews, func(a, b preview) int {
		return cmp.Compare(b.size.X*b.size.Y, a.size.X*a.size.Y)
	})
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Describe satisfies the ivy.Describer interface. Producer is the make and
// the model of the camera, and Created is the date of the photo.
func (d *Document) Describe(context.Context) (ivy.Info, error) {
	return d.info, nil
}

// Previews satisfies the ivy.Previewer interface. It lists the JPEG images of
// the file, largest first.
func (d *Document) Previews(context.Context) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		for _, p := range d.previews {
			if !yield(d.preview(p), nil) {
				return
			}
		}
	}
}

// preview returns the ivy.Preview of p.
func (d *Document) preview(p preview) ivy.Preview {
	return ivy.Preview{
		Kind: ivy.Thumbnail,
		Size: p.size,
		Decode: func(ctx context.Context) (image.Image, error) {
			buf, err := raster.ReadAll(io.NewSectionReader(d.src, p.off, p.n), d.lim.MaxMemory)
			if err != nil {
				return nil, err
			}
			return raster.Decode(ctx, buf, d.lim)
		},
	}
}

// Decode reads a RAW file from r, and returns its largest preview. It reads
// at most ivy.DefaultMaxMemory bytes.
func Decode(r io.Reader) (image.Image, error) {
	d, err := openReader(r)
	if err != nil {
		return nil, err
	}
	return d.preview(d.previews[0]).Decode(context.Background())
}

// DecodeConfig returns the size of the largest preview of the RAW file in r.
func DecodeConfig(r io.Reader) (image.Config, error) {
	d, err := openReader(r)
	if err != nil {
		return image.Config{}, err
	}
	p := d.previews[0]
	cfg, err := jpeg.DecodeConfig(io.NewSectionReader(d.src, p.off, min(p.n, headerLimit)))
	if err != nil {
		return image.Config{}, fmt.Errorf("reading preview: %w: %w", ivy.ErrFormat, err)
	}
	return cfg, nil
}

// openReader reads r to its end, and opens it. It returns ivy.ErrNoPreview if
// the file holds no preview.
func openReader(r io.Reader) (*Document, error) {
	buf, err := raster.ReadAll(r, ivy.DefaultMaxMemory)
	if err != nil {
		return nil, err
	}
	d, err := Open(context.Background(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	if len(d.previews) == 0 {
		return nil, ivy.ErrNoPreview
	}
	return d, nil
}
