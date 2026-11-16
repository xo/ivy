package ivy

import (
	"cmp"
	"context"
	"errors"
	"image"
	"image/color"
	"iter"
	"math"
	"slices"

	"github.com/xo/okage"
)

// Format describes a format package for the registry of a program. ivy holds
// no registry (D6). Each format package exports a Format value, and a program
// builds its own registry from the values of the packages that it imports.
type Format struct {
	// Name is the name of the format, such as "psd".
	Name string
	// Mimes are the mime types of the format.
	Mimes []string
	// Exts are the extensions of the format, without the leading dot.
	Exts []string
	// Open opens a document with the default options of the format and the
	// configuration cfg.
	Open func(ctx context.Context, src Source, cfg Config) (Document, error)
}

// Pages iterates over the pages of p, in order, until the last page. It
// stops after the first error.
func Pages(ctx context.Context, p Pager) iter.Seq2[Page, error] {
	return func(yield func(Page, error) bool) {
		for n := 0; ; n++ {
			page, err := p.Page(ctx, n)
			switch {
			case errors.Is(err, ErrPageRange):
				return
			case err != nil:
				yield(nil, err)
				return
			case !yield(page, nil):
				return
			}
		}
	}
}

// BestPreview decodes the preview of d that is closest in size to want, and
// prefers one that is larger. A zero want asks for the largest preview. If
// the decode of a preview fails, BestPreview tries the next one. It returns
// ErrNoPreview if d is not a Previewer or lists no preview.
func BestPreview(ctx context.Context, d Document, want image.Point) (image.Image, error) {
	previewer, ok := d.(Previewer)
	if !ok {
		return nil, ErrNoPreview
	}
	var previews []Preview
	for preview, err := range previewer.Previews(ctx) {
		if err != nil {
			return nil, err
		}
		previews = append(previews, preview)
	}
	if len(previews) == 0 {
		return nil, ErrNoPreview
	}
	sortPreviews(previews, want)
	var last error
	for _, preview := range previews {
		img, err := preview.Decode(ctx)
		if err == nil {
			return img, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		last = err
	}
	return nil, last
}

// sortPreviews sorts the previews in the order that BestPreview tries them:
// first the smallest one that covers want, then the larger ones, then the
// smaller ones from the largest, and last the ones of unknown size.
func sortPreviews(previews []Preview, want image.Point) {
	rank := func(p Preview) (int, int) {
		side := max(p.Size.X, p.Size.Y)
		switch {
		case side == 0:
			return 3, 0
		case want == (image.Point{}):
			return 1, -side
		case p.Size.X >= want.X && p.Size.Y >= want.Y:
			return 0, side
		default:
			return 2, -side
		}
	}
	slices.SortStableFunc(previews, func(a, b Preview) int {
		ag, ak := rank(a)
		bg, bk := rank(b)
		if c := cmp.Compare(ag, bg); c != 0 {
			return c
		}
		return cmp.Compare(ak, bk)
	})
}

// RenderImage renders p at dpi dots per inch over bg, and returns the image
// (D11). A nil bg leaves the background transparent. A page of a raster
// image renders at its own size at 72 dots per inch.
//
// If the page is drawn only in part, RenderImage returns the image and a
// *PartialError.
func RenderImage(ctx context.Context, p Page, dpi float32, bg color.Color) (image.Image, error) {
	if dpi <= 0 || math.IsNaN(float64(dpi)) || math.IsInf(float64(dpi), 0) {
		return nil, ErrUnsupported
	}
	scale := dpi / 72
	w, h := p.Size()
	pw, ph := math.Ceil(float64(w*scale)), math.Ceil(float64(h*scale))
	if pw < 1 || ph < 1 {
		return nil, ErrFormat
	}
	if pw*ph > DefaultMaxPixels || pw > math.MaxInt32 || ph > math.MaxInt32 {
		return nil, ErrLimit
	}
	dst, ok := okage.NewPixmap(uint32(pw), uint32(ph))
	if !ok {
		return nil, ErrLimit
	}
	if bg != nil {
		dst.Fill(okageColor(bg))
	}
	err := p.Render(ctx, dst, okage.TransformFromScale(scale, scale))
	if err != nil {
		if _, partial := errors.AsType[*PartialError](err); !partial {
			return nil, err
		}
	}
	return dst, err
}

// okageColor converts c to a color of okage, which is not premultiplied.
func okageColor(c color.Color) okage.Color {
	n, _ := color.NRGBAModel.Convert(c).(color.NRGBA)
	return okage.ColorFromRGBA8(n.R, n.G, n.B, n.A)
}
