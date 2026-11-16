// Package comic opens comic book archives: CBZ, a zip archive of images, and
// CBT, a tar archive of images.
//
// Each image is a page, in the natural order of the names, so that page2.jpg
// comes before page10.jpg. The first page is the cover (D11). CBR and CB7 need
// outside packages for RAR and 7z, and they come as modules of their own after
// the maintainers approve the packages (D6, D7).
package comic

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"iter"
	"path"
	"slices"
	"strings"
	"sync"

	_ "golang.org/x/image/webp" // the WebP decoder, for image.Decode

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/internal/zipdoc"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "comic",
	Mimes: []string{"application/vnd.comicbook+zip", "application/x-cbz", "application/x-cbt"},
	Exts:  []string{"cbz", "cbt"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// headerLimit is the number of bytes of an entry that a page reads to find
// the size of its image.
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

// Document is an open comic book archive.
type Document struct {
	lim     ivy.Limits
	entries []entry

	mu    sync.Mutex
	pages map[int]*raster.Page
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
)

// entry is one image of the archive.
type entry struct {
	name string
	// open opens the encoded image.
	open func() (io.ReadCloser, error)
}

// Open opens a CBZ or a CBT file. It returns an error that wraps ivy.ErrFormat
// if src is neither a zip nor a tar archive.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening comic book archive: %w", err)
	}
	d := &Document{lim: o.cfg.Limits.WithDefaults(), pages: make(map[int]*raster.Page)}
	var err error
	if d.entries, err = zipEntries(src, d.lim); errors.Is(err, ivy.ErrFormat) {
		d.entries, err = tarEntries(src, d.lim)
	}
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(d.entries, func(a, b entry) int {
		return naturalCompare(a.name, b.name)
	})
	return d, nil
}

// zipEntries lists the images of a zip archive.
func zipEntries(src ivy.Source, lim ivy.Limits) ([]entry, error) {
	archive, err := zipdoc.Open(src, lim)
	if err != nil {
		return nil, err
	}
	var entries []entry
	for _, f := range archive.Files() {
		if f.FileInfo().IsDir() || !isImage(f.Name) {
			continue
		}
		entries = append(entries, entry{name: f.Name, open: func() (io.ReadCloser, error) {
			buf, err := archive.Read(f)
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(buf)), nil
		}})
	}
	return entries, nil
}

// tarEntries lists the images of a tar archive. A tar archive has no index,
// so tarEntries reads each header once, and keeps the offset of its data.
func tarEntries(src ivy.Source, lim ivy.Limits) ([]entry, error) {
	counter := &countingReader{r: io.NewSectionReader(src, 0, src.Size())}
	tr := tar.NewReader(counter)
	var entries []entry
	for n := 0; ; n++ {
		hdr, err := tr.Next()
		switch {
		case errors.Is(err, io.EOF) && n > 0:
			return entries, nil
		case err != nil:
			return nil, fmt.Errorf("reading archive: %w: %w", ivy.ErrFormat, err)
		case n >= lim.MaxObjects:
			return nil, fmt.Errorf("archive of more than %d entries: %w", lim.MaxObjects, ivy.ErrLimit)
		}
		if hdr.Typeflag != tar.TypeReg || !isImage(hdr.Name) {
			continue
		}
		off, size := counter.n, hdr.Size
		if size > lim.MaxMemory {
			return nil, fmt.Errorf("archive entry %s of %d bytes: %w", hdr.Name, size, ivy.ErrLimit)
		}
		entries = append(entries, entry{name: hdr.Name, open: func() (io.ReadCloser, error) {
			return io.NopCloser(io.NewSectionReader(src, off, size)), nil
		}})
	}
}

// countingReader counts the bytes that it reads.
type countingReader struct {
	r io.Reader
	n int64
}

// Read satisfies the io.Reader interface.
func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	// a reader returns io.EOF as it is, so that its caller sees the end
	return n, err //nolint:wrapcheck
}

// isImage reports whether name is an image that a page decodes. The files of
// macOS, such as those under __MACOSX/, are left out.
func isImage(name string) bool {
	base := path.Base(name)
	if strings.HasPrefix(base, ".") || strings.Contains(name, "__MACOSX/") {
		return false
	}
	switch strings.ToLower(path.Ext(base)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	}
	return false
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. It is the number of images.
func (d *Document) PageCount(context.Context) (int, error) {
	return len(d.entries), nil
}

// Page satisfies the ivy.Pager interface. It reads the size of the image, and
// decodes the image when the page renders.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	if n < 0 || n >= len(d.entries) {
		return nil, ivy.ErrPageRange
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if p, ok := d.pages[n]; ok {
		return p, nil
	}
	e := d.entries[n]
	size, err := d.config(ctx, e)
	if err != nil {
		return nil, err
	}
	p := raster.NewPage(size, func(ctx context.Context) (image.Image, error) {
		return d.decode(ctx, e)
	})
	d.pages[n] = p
	return p, nil
}

// Previews satisfies the ivy.Previewer interface. The cover is the first page.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		if len(d.entries) == 0 {
			return
		}
		e := d.entries[0]
		size, err := d.config(ctx, e)
		if err != nil {
			yield(ivy.Preview{}, err)
			return
		}
		yield(ivy.Preview{
			Kind: ivy.Cover,
			Size: size,
			Decode: func(ctx context.Context) (image.Image, error) {
				return d.decode(ctx, e)
			},
		}, nil)
	}
}

// config reads the size of the image of e, and checks it against the limits.
func (d *Document) config(ctx context.Context, e entry) (image.Point, error) {
	if err := ctx.Err(); err != nil {
		return image.Point{}, fmt.Errorf("reading %s: %w", e.name, err)
	}
	rc, err := e.open()
	if err != nil {
		return image.Point{}, err
	}
	defer func() { _ = rc.Close() }()
	buf, err := io.ReadAll(io.LimitReader(rc, headerLimit))
	if err != nil {
		return image.Point{}, fmt.Errorf("reading %s: %w: %w", e.name, ivy.ErrFormat, err)
	}
	size, err := raster.Config(buf)
	if err != nil {
		return image.Point{}, fmt.Errorf("page %s: %w", e.name, err)
	}
	if err := raster.CheckPixels(size, d.lim); err != nil {
		return image.Point{}, err
	}
	return size, nil
}

// decode decodes the image of e.
func (d *Document) decode(ctx context.Context, e entry) (image.Image, error) {
	rc, err := e.open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	buf, err := raster.ReadAll(rc, d.lim.MaxMemory)
	if err != nil {
		return nil, err
	}
	return raster.Decode(ctx, buf, d.lim)
}

// naturalCompare compares two names, and compares each run of digits as a
// number, so that "page2" comes before "page10".
func naturalCompare(a, b string) int {
	for a != "" && b != "" {
		da, db := digits(a), digits(b)
		if da > 0 && db > 0 {
			na, nb := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
			if c := compareNumbers(na, nb); c != 0 {
				return c
			}
			a, b = a[da:], b[db:]
			continue
		}
		ca, cb := strings.ToLower(a[:1]), strings.ToLower(b[:1])
		if ca != cb {
			return strings.Compare(ca, cb)
		}
		a, b = a[1:], b[1:]
	}
	return len(a) - len(b)
}

// digits returns the length of the run of digits at the start of s.
func digits(s string) int {
	n := 0
	for n < len(s) && '0' <= s[n] && s[n] <= '9' {
		n++
	}
	return n
}

// compareNumbers compares two numbers written in digits without leading
// zeros.
func compareNumbers(a, b string) int {
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}
