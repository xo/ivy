package drawingml

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"math"
	"sync"

	_ "golang.org/x/image/bmp"  // the BMP decoder, for raster.Decode
	_ "golang.org/x/image/tiff" // the TIFF decoder, for raster.Decode
	_ "golang.org/x/image/webp" // the WebP decoder, for raster.Decode

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/metafile"
	"github.com/xo/ivy/internal/raster"
)

// pictureKind is the kind of the data of a picture.
type pictureKind uint8

// The kinds of pictures.
const (
	pictureRaster pictureKind = iota
	pictureMetafile
	pictureSVG
)

// Picture is an image that a document embeds: a raster image, such as PNG,
// JPEG, GIF, BMP, TIFF or WebP, a metafile of Windows, or SVG. It decodes
// its data once, on the first draw. It is safe for concurrent use.
type Picture struct {
	data []byte
	cfg  ivy.Config
	lim  ivy.Limits

	once sync.Once
	kind pictureKind
	img  image.Image
	pix  *okage.Pixmap
	meta *metafile.Picture
	tree *svg.Tree
	err  error
}

// NewPicture returns the picture of data. It decodes nothing yet. The
// picture keeps data, so the caller must not change it.
func NewPicture(data []byte, cfg ivy.Config) *Picture {
	return &Picture{data: data, cfg: cfg, lim: cfg.Limits.WithDefaults()}
}

// decode decodes the data of the picture once.
func (p *Picture) decode(ctx context.Context) error {
	p.once.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				p.err = fmt.Errorf("decoding picture: panic: %v: %w", r, ivy.ErrFormat)
			}
		}()
		data := p.data
		if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
			// a picture that gzip compresses, such as EMZ, WMZ or SVGZ
			zr, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				p.err = fmt.Errorf("decoding compressed picture: %w: %w", ivy.ErrFormat, err)
				return
			}
			data, err = raster.ReadAll(zr, p.lim.MaxMemory)
			if err != nil {
				p.err = fmt.Errorf("decoding compressed picture: %w", err)
				return
			}
		}
		switch {
		case isMetafile(data):
			p.kind = pictureMetafile
			p.meta, p.err = metafile.Open(data, metafile.Options{Config: p.cfg})
		case isSVG(data):
			p.kind = pictureSVG
			p.tree, p.err = svg.FromData(data, svg.WithMaxDepth(uint32(min(p.lim.MaxDepth, 1024))))
			if p.err != nil {
				p.err = fmt.Errorf("decoding svg picture: %w: %w", ivy.ErrFormat, p.err)
			}
		default:
			p.kind = pictureRaster
			p.img, p.err = raster.Decode(ctx, data, p.lim)
		}
	})
	return p.err
}

// isMetafile reports whether data starts as a WMF or an EMF file.
func isMetafile(data []byte) bool {
	if len(data) < 44 {
		return false
	}
	switch {
	case binary.LittleEndian.Uint32(data) == 0x9ac6cdd7:
		// the placeable header of WMF
		return true
	case binary.LittleEndian.Uint32(data) == 1 && string(data[40:44]) == " EMF":
		return true
	case (data[0] == 1 || data[0] == 2) && data[1] == 0 && data[2] == 9 && data[3] == 0:
		// the header of WMF: in memory or on disk, of 9 words
		return true
	}
	return false
}

// isSVG reports whether data starts as an XML document that can be SVG.
func isSVG(data []byte) bool {
	head := bytes.TrimPrefix(data[:min(len(data), 512)], []byte("\xef\xbb\xbf"))
	head = bytes.TrimLeft(head, " \t\r\n")
	return bytes.HasPrefix(head, []byte("<")) && bytes.Contains(data[:min(len(data), 4096)], []byte("<svg"))
}

// Resolution returns the resolution of a raster picture in dots per inch,
// from the JFIF header of a JPEG file or the pHYs chunk of a PNG file. It
// returns 96 for each axis that the picture does not give, as Office does.
func (p *Picture) Resolution() (float64, float64) {
	x, y, ok := rasterResolution(p.data)
	if !ok || !(x >= 1 && x <= 10000) || !(y >= 1 && y <= 10000) {
		return 96, 96
	}
	return x, y
}

// rasterResolution reads the resolution of the JPEG or PNG file data, in
// dots per inch.
func rasterResolution(data []byte) (float64, float64, bool) {
	switch {
	case len(data) >= 18 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff && data[3] == 0xe0 &&
		string(data[6:11]) == "JFIF\x00":
		x, y := float64(binary.BigEndian.Uint16(data[14:])), float64(binary.BigEndian.Uint16(data[16:]))
		switch data[13] {
		case 1:
			return x, y, true
		case 2:
			return x * 2.54, y * 2.54, true
		}
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		for i := 8; i+12 <= len(data); {
			n := int(binary.BigEndian.Uint32(data[i:]))
			typ := string(data[i+4 : i+8])
			if n < 0 || n > len(data)-i-12 || typ == "IDAT" {
				break
			}
			if typ == "pHYs" && n >= 9 && data[i+16] == 1 {
				// pixels per meter
				x, y := float64(binary.BigEndian.Uint32(data[i+8:])), float64(binary.BigEndian.Uint32(data[i+12:]))
				return x * 0.0254, y * 0.0254, true
			}
			i += n + 12
		}
	}
	return 0, 0, false
}

// Size returns the size of the picture in pixels, or in points for a
// metafile and SVG. It returns false if the picture cannot be decoded.
func (p *Picture) Size(ctx context.Context) (float64, float64, bool) {
	if p.decode(ctx) != nil {
		return 0, 0, false
	}
	switch p.kind {
	case pictureMetafile:
		w, h := p.meta.Size()
		return float64(w), float64(h), true
	case pictureSVG:
		s := p.tree.Size()
		return float64(s.Width()), float64(s.Height()), true
	}
	b := p.img.Bounds()
	return float64(b.Dx()), float64(b.Dy()), true
}

// Insets are the four insets of a rectangle, each a fraction of its width or
// its height, such as the a:srcRect and a:fillRect of a picture. A negative
// inset grows the rectangle.
type Insets struct {
	L, T, R, B float64
}

// ParseInsets reads the attributes l, t, r and b of n, in 1000ths of a
// percent.
func ParseInsets(n roxmltree.Node) Insets {
	return Insets{
		L: pct(numberAttr(n, "l", 0)),
		T: pct(numberAttr(n, "t", 0)),
		R: pct(numberAttr(n, "r", 0)),
		B: pct(numberAttr(n, "b", 0)),
	}
}

// Zero reports whether each inset is zero.
func (in Insets) Zero() bool {
	return in == Insets{}
}

// apply returns the rectangle r with the insets.
func (in Insets) apply(r okage.Rect) (okage.Rect, bool) {
	w, h := float64(r.Width()), float64(r.Height())
	x0 := float64(r.X()) + in.L*w
	y0 := float64(r.Y()) + in.T*h
	x1 := float64(r.Right()) - in.R*w
	y1 := float64(r.Bottom()) - in.B*h
	return rectLTRB(x0, y0, x1, y1)
}

// rectLTRB returns the rectangle of the edges, or false if it is empty or
// not finite.
func rectLTRB(x0, y0, x1, y1 float64) (okage.Rect, bool) {
	for _, v := range [...]float64{x0, y0, x1, y1} {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e9 {
			return okage.Rect{}, false
		}
	}
	if x1 <= x0 || y1 <= y0 {
		return okage.Rect{}, false
	}
	return okage.RectFromLTRB(float32(x0), float32(y0), float32(x1), float32(y1))
}

// Draw draws the picture into the rectangle dst of the user space of dev.
// crop is the part of the picture that dst shows, as a:srcRect gives it:
// the picture is cut by a positive inset and grows by a negative one. scale
// is the number of pixels of the device for a unit of the user space, for
// the pictures that okage draws into pixels, such as SVG. opacity is from 0
// to 1.
func (p *Picture) Draw(ctx context.Context, dev display.Device, dst okage.Rect, crop Insets, opacity, scale float64) error {
	if err := p.decode(ctx); err != nil {
		return err
	}
	if !(crop.L+crop.R < 1 && crop.T+crop.B < 1) {
		return nil
	}
	// full is the rectangle of the whole picture, so that its crop falls on
	// dst
	sw, sh := 1-crop.L-crop.R, 1-crop.T-crop.B
	fw, fh := float64(dst.Width())/sw, float64(dst.Height())/sh
	fx, fy := float64(dst.X())-crop.L*fw, float64(dst.Y())-crop.T*fh
	full, ok := rectLTRB(fx, fy, fx+fw, fy+fh)
	if !ok {
		return nil
	}
	dev.Save()
	defer dev.Restore()
	if !crop.Zero() {
		dev.ClipPath(okage.PathFromRect(dst), okage.FillRuleWinding)
	}
	switch p.kind {
	case pictureMetafile:
		if opacity < 1 {
			dev.BeginGroup(display.Group{Opacity: float32(unit(opacity)), Isolated: true})
			defer dev.EndGroup()
		}
		return p.meta.Draw(ctx, dev, full)
	case pictureSVG:
		return p.drawSVG(dev, full, opacity, scale)
	}
	dev.DrawImage(p.img, display.RectTransform(p.img.Bounds().Size(), full),
		display.ImageOptions{Opacity: float32(unit(opacity)), Smooth: true})
	return nil
}

// drawSVG renders the SVG picture into pixels at the scale of the device,
// and draws them into full.
func (p *Picture) drawSVG(dev display.Device, full okage.Rect, opacity, scale float64) error {
	size := p.tree.Size()
	if !(size.Width() > 0 && size.Height() > 0) {
		return nil
	}
	if !(scale > 0) || math.IsInf(scale, 0) {
		scale = 1
	}
	w := math.Ceil(float64(full.Width()) * scale)
	h := math.Ceil(float64(full.Height()) * scale)
	// the pixels are bounded by the limits and by a size that a screen
	// shows
	if limit := float64(min(p.lim.MaxPixels, 1<<24)); w*h > limit {
		f := math.Sqrt(limit / (w * h))
		w, h = math.Floor(w*f), math.Floor(h*f)
	}
	if w < 1 || h < 1 {
		return nil
	}
	pix, ok := okage.NewPixmap(uint32(w), uint32(h))
	if !ok {
		return fmt.Errorf("svg picture of %gx%g pixels: %w", w, h, ivy.ErrLimit)
	}
	t := okage.TransformFromScale(float32(w)/size.Width(), float32(h)/size.Height())
	m := pix.AsMut()
	svg.Render(p.tree, t, &m)
	img := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	copy(img.Pix, pix.TakeDemultiplied())
	dev.DrawImage(img, display.RectTransform(img.Bounds().Size(), full),
		display.ImageOptions{Opacity: float32(unit(opacity)), Smooth: true})
	return nil
}

// pattern returns the pixmap of a raster picture, for a tile. It returns
// false for a metafile or SVG.
func (p *Picture) pattern(ctx context.Context) (*okage.Pixmap, image.Point, bool) {
	if p.decode(ctx) != nil || p.kind != pictureRaster {
		return nil, image.Point{}, false
	}
	pix, err := p.pixmap()
	if err != nil {
		return nil, image.Point{}, false
	}
	return pix, p.img.Bounds().Size(), true
}

// pixmapMu guards the pixmap of the pictures.
var pixmapMu sync.Mutex

// pixmap returns the pixmap of a raster picture, which it makes once.
func (p *Picture) pixmap() (*okage.Pixmap, error) {
	pixmapMu.Lock()
	defer pixmapMu.Unlock()
	if p.pix != nil {
		return p.pix, nil
	}
	pix, err := raster.Pixmap(p.img)
	if err != nil {
		return nil, err
	}
	p.pix = pix
	return pix, nil
}
