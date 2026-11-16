// Package ps opens PostScript and EPS files.
//
// On 2026-11-16 a document gives the previews that an EPS file carries (D11):
// the TIFF image of the binary header of a DOS EPS file, and the bitmap of an
// EPSI file, between %%BeginPreview and %%EndPreview. A WMF preview waits for
// the reader of phase 5, and the pages wait for a PostScript interpreter,
// which is in the long tail (D1). A document also says its title, its author,
// its creator and its date, from its DSC comments.
package ps

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"iter"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/tiff"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name: "ps",
	Mimes: []string{
		"application/postscript",
		"application/eps",
		"application/x-eps",
		"image/eps",
		"image/x-eps",
	},
	Exts: []string{"ps", "eps", "epsi", "epsf", "ept"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// dosSignature starts the binary header of a DOS EPS file.
var dosSignature = []byte{0xc5, 0xd0, 0xd3, 0xc6}

// dosHeaderLen is the length of the binary header of a DOS EPS file.
const dosHeaderLen = 30

// maxLine is the length of the longest line that Open reads. The DSC
// comments are at most 255 bytes long, and a line of a preview is longer.
const maxLine = 64 << 10

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

// Document is an open PostScript or EPS file.
type Document struct {
	src  ivy.Source
	lim  ivy.Limits
	info ivy.Info

	// tiff is the section of the TIFF preview of a DOS EPS file.
	tiff *io.SectionReader
	// epsi is the preview of an EPSI file.
	epsi *epsiPreview
}

// The capabilities of a document.
var (
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a PostScript or an EPS file. It returns an error that wraps
// ivy.ErrFormat if src is neither.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	d := &Document{src: src, lim: o.cfg.Limits.WithDefaults()}
	ps, err := d.readDOSHeader()
	if err != nil {
		return nil, err
	}
	if err := d.readComments(ctx, ps); err != nil {
		return nil, err
	}
	return d, nil
}

// readDOSHeader reads the binary header of a DOS EPS file, if src has one. It
// returns the section of src that holds the PostScript.
func (d *Document) readDOSHeader() (*io.SectionReader, error) {
	size := d.src.Size()
	head := make([]byte, dosHeaderLen)
	n, err := d.src.ReadAt(head, 0)
	if err != nil && n < len(dosSignature) {
		return nil, fmt.Errorf("reading postscript header: %w", ivy.ErrFormat)
	}
	if !bytes.HasPrefix(head, dosSignature) {
		return io.NewSectionReader(d.src, 0, size), nil
	}
	if n < dosHeaderLen {
		return nil, fmt.Errorf("short dos eps header: %w", ivy.ErrFormat)
	}
	section := func(i int) (*io.SectionReader, error) {
		off := int64(binary.LittleEndian.Uint32(head[i:]))
		n := int64(binary.LittleEndian.Uint32(head[i+4:]))
		if off < dosHeaderLen || n < 0 || off > size || n > size-off {
			return nil, fmt.Errorf("dos eps section at %d of %d bytes: %w", off, n, ivy.ErrFormat)
		}
		return io.NewSectionReader(d.src, off, n), nil
	}
	ps, err := section(4)
	if err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint32(head[24:]) != 0 {
		if d.tiff, err = section(20); err != nil {
			return nil, err
		}
	}
	return ps, nil
}

// readComments reads the DSC comments of the header and the EPSI preview. It
// stops at the prolog, at the first page, or at the limit of memory.
func (d *Document) readComments(ctx context.Context, ps *io.SectionReader) error {
	r := bufio.NewReaderSize(io.LimitReader(ps, d.lim.MaxMemory), 4096)
	first, err := readLine(r)
	if err != nil || !strings.HasPrefix(first, "%!PS") {
		return fmt.Errorf("file does not start with %%!PS: %w", ivy.ErrFormat)
	}
	header := true
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("reading postscript: %w", err)
		}
		line, err := readLine(r)
		switch {
		case errors.Is(err, io.EOF):
			// a file that ends in its header still has its comments
			return nil
		case err != nil:
			return err
		}
		switch {
		case strings.HasPrefix(line, "%%EndComments"), !strings.HasPrefix(line, "%"):
			header = false
		case strings.HasPrefix(line, "%%BeginPreview:"):
			if d.epsi, err = readEPSI(r, line, d.lim); err != nil {
				return err
			}
		case strings.HasPrefix(line, "%%BeginProlog"),
			strings.HasPrefix(line, "%%EndProlog"),
			strings.HasPrefix(line, "%%BeginSetup"),
			strings.HasPrefix(line, "%%Page:"):
			return nil
		case header:
			d.readComment(line)
		}
	}
}

// readComment reads one DSC comment of the header.
func (d *Document) readComment(line string) {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return
	}
	value = dscText(value)
	switch key {
	case "%%Title":
		d.info.Title = value
	case "%%For":
		if value != "" {
			d.info.Authors = []string{value}
		}
	case "%%Creator":
		d.info.Producer = value
	case "%%CreationDate":
		d.info.Created = dscTime(value)
	}
}

// dscText returns the text of a DSC comment, without the parentheses of a
// PostScript string.
func dscText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		s = s[1 : len(s)-1]
	}
	return strings.TrimSpace(s)
}

// dscTime parses the date of %%CreationDate. The comment has no fixed form,
// so dscTime tries the forms that programs write, and returns the zero time
// for any other.
func dscTime(s string) time.Time {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"Mon Jan _2 15:04:05 2006",
		"1/2/06 15:04",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	// a date of PDF, D:YYYYMMDDHHmmSS, which some programs write
	if digits, ok := strings.CutPrefix(s, "D:"); ok && len(digits) >= 14 {
		if t, err := time.Parse("20060102150405", digits[:14]); err == nil {
			return t
		}
	}
	return time.Time{}
}

// readLine reads one line, which ends at a carriage return, a line feed, or
// both. It returns an error that wraps ivy.ErrLimit for a line longer than
// maxLine.
func readLine(r *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		c, err := r.ReadByte()
		switch {
		case err != nil:
			if b.Len() > 0 {
				return b.String(), nil
			}
			return "", fmt.Errorf("reading line: %w", err)
		case c == '\n':
			return b.String(), nil
		case c == '\r':
			if next, err := r.Peek(1); err == nil && next[0] == '\n' {
				_, _ = r.ReadByte()
			}
			return b.String(), nil
		case b.Len() >= maxLine:
			return "", fmt.Errorf("line longer than %d bytes: %w", maxLine, ivy.ErrLimit)
		}
		b.WriteByte(c)
	}
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Describe satisfies the ivy.Describer interface. It gives the title, the
// author, the creator and the date of the DSC comments.
func (d *Document) Describe(context.Context) (ivy.Info, error) {
	return d.info, nil
}

// Previews satisfies the ivy.Previewer interface. It lists the TIFF preview
// and the EPSI preview, where the file holds them.
func (d *Document) Previews(context.Context) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		if d.tiff != nil && !yield(d.tiffPreview(), nil) {
			return
		}
		if d.epsi != nil {
			yield(d.epsi.preview(d.lim), nil)
		}
	}
}

// tiffPreview returns the TIFF preview of a DOS EPS file.
func (d *Document) tiffPreview() ivy.Preview {
	var size image.Point
	if cfg, err := tiff.DecodeConfig(io.NewSectionReader(d.tiff, 0, d.tiff.Size())); err == nil {
		size = image.Pt(cfg.Width, cfg.Height)
	}
	return ivy.Preview{
		Kind: ivy.Thumbnail,
		Size: size,
		Decode: func(ctx context.Context) (image.Image, error) {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("decoding tiff preview: %w", err)
			}
			cfg, err := tiff.DecodeConfig(io.NewSectionReader(d.tiff, 0, d.tiff.Size()))
			if err != nil {
				return nil, tiffError(err)
			}
			if err := raster.CheckPixels(image.Pt(cfg.Width, cfg.Height), d.lim); err != nil {
				return nil, err
			}
			img, err := tiff.Decode(io.NewSectionReader(d.tiff, 0, d.tiff.Size()))
			if err != nil {
				return nil, tiffError(err)
			}
			return img, nil
		},
	}
}

// tiffError wraps an error of the TIFF decoder with ivy.ErrUnsupported for a
// feature that the decoder does not support, and with ivy.ErrFormat for any
// other.
func tiffError(err error) error {
	if _, ok := errors.AsType[tiff.UnsupportedError](err); ok {
		return fmt.Errorf("decoding tiff preview: %w: %w", ivy.ErrUnsupported, err)
	}
	return fmt.Errorf("decoding tiff preview: %w: %w", ivy.ErrFormat, err)
}

// epsiPreview is the bitmap of an EPSI file.
type epsiPreview struct {
	width, height, depth int
	data                 []byte
}

// readEPSI reads the preview that starts with the line
// "%%BeginPreview: width height depth lines", up to %%EndPreview.
func readEPSI(r *bufio.Reader, line string, lim ivy.Limits) (*epsiPreview, error) {
	fields := strings.Fields(strings.TrimPrefix(line, "%%BeginPreview:"))
	if len(fields) < 3 {
		return nil, fmt.Errorf("epsi preview %q: %w", line, ivy.ErrFormat)
	}
	var v [3]int
	for i := range v {
		n, err := strconv.Atoi(fields[i])
		if err != nil {
			return nil, fmt.Errorf("epsi preview %q: %w", line, ivy.ErrFormat)
		}
		v[i] = n
	}
	p := &epsiPreview{width: v[0], height: v[1], depth: v[2]}
	switch p.depth {
	case 1, 2, 4, 8:
	default:
		return nil, fmt.Errorf("epsi preview of depth %d: %w", p.depth, ivy.ErrFormat)
	}
	if err := raster.CheckPixels(image.Pt(p.width, p.height), lim); err != nil {
		return nil, err
	}
	want := p.stride() * p.height
	var hexText strings.Builder
	for {
		line, err := readLine(r)
		if err != nil {
			return nil, fmt.Errorf("epsi preview without %%%%EndPreview: %w", ivy.ErrFormat)
		}
		if strings.HasPrefix(line, "%%EndPreview") {
			break
		}
		if hexText.Len() < 2*want {
			hexText.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "%")))
		}
	}
	data, err := hex.DecodeString(hexText.String())
	if err != nil || len(data) < want {
		return nil, fmt.Errorf("epsi preview data: %w", ivy.ErrFormat)
	}
	p.data = data[:want]
	return p, nil
}

// stride returns the number of bytes of one row. Each row starts on a byte.
func (p *epsiPreview) stride() int {
	return (p.width*p.depth + 7) / 8
}

// preview returns the preview.
func (p *epsiPreview) preview(lim ivy.Limits) ivy.Preview {
	return ivy.Preview{
		Kind: ivy.Thumbnail,
		Size: image.Pt(p.width, p.height),
		Decode: func(ctx context.Context) (image.Image, error) {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("decoding epsi preview: %w", err)
			}
			if err := raster.CheckPixels(image.Pt(p.width, p.height), lim); err != nil {
				return nil, err
			}
			return p.image(), nil
		},
	}
}

// image returns the bitmap as a gray image. In EPSI, 0 is white and the
// largest value is black, the reverse of the image operator of PostScript
// (EPSF 3.0, and ps2epsi of Ghostscript).
func (p *epsiPreview) image() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, p.width, p.height))
	largest := 1<<p.depth - 1
	perByte := 8 / p.depth
	for y := range p.height {
		row := p.data[y*p.stride() : (y+1)*p.stride()]
		for x := range p.width {
			b := row[x/perByte]
			shift := 8 - p.depth*(x%perByte+1)
			v := int(b>>shift) & largest
			img.SetGray(x, y, color.Gray{Y: uint8(255 - v*255/largest)})
		}
	}
	return img
}
