package odraw

import (
	"bytes"
	"context"
	"image/png"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/raster"
)

// Pictures holds the pictures of a store, each made once. It is not safe
// for concurrent use: the format guards it.
type Pictures struct {
	store []Entry
	delay []byte
	cfg   ivy.Config
	pics  map[int]*drawingml.Picture
	raw   map[int][]byte
	keyed map[pictureKey]*drawingml.Picture
	// recolored holds the files of the pictures with their colors
	// changed, or nil for a picture that Recolored cannot change.
	recolored map[recolorKey][]byte
}

// recolorKey is a picture of the store with a change of its colors.
type recolorKey struct {
	pib int
	r   drawingml.Recolor
}

// pictureKey is a picture of the store with a color that it leaves
// transparent.
type pictureKey struct {
	pib int
	key uint32
}

// NewPictures returns the pictures of the entries store, whose records are
// in the entries or in the stream delay.
func NewPictures(store []Entry, delay []byte, cfg ivy.Config) *Pictures {
	return &Pictures{
		store:     store,
		delay:     delay,
		cfg:       cfg,
		pics:      make(map[int]*drawingml.Picture),
		raw:       make(map[int][]byte),
		keyed:     make(map[pictureKey]*drawingml.Picture),
		recolored: make(map[recolorKey][]byte),
	}
}

// Data returns the file of the picture of the entry pib, from 1, or nil.
func (p *Pictures) Data(pib int) []byte {
	if p.load(pib) == nil {
		return nil
	}
	return p.raw[pib]
}

// load returns the picture of the entry pib, made once, or nil.
func (p *Pictures) load(pib int) *drawingml.Picture {
	if pic, ok := p.pics[pib]; ok {
		return pic
	}
	p.pics[pib] = nil
	if pib < 1 || pib > len(p.store) {
		return nil
	}
	data := Data(p.store[pib-1].Record(p.delay), p.cfg.Limits.WithDefaults().MaxMemory)
	if data == nil {
		return nil
	}
	pic := drawingml.NewPicture(data, p.cfg)
	p.pics[pib] = pic
	p.raw[pib] = data
	return pic
}

// Get returns the picture of the entry pib, from 1, or nil. When keyed is
// set, the pixels of the color of the OfficeArtCOLORREF key of a raster
// picture are transparent, which needs ctx to decode the picture.
func (p *Pictures) Get(ctx context.Context, pib int, key uint32, keyed bool) *drawingml.Picture {
	pic := p.load(pib)
	if pic == nil || !keyed || key&0xFF000000 != 0 {
		return pic
	}
	k := pictureKey{pib: pib, key: key}
	if q, ok := p.keyed[k]; ok {
		return q
	}
	p.keyed[k] = pic
	data, ok := ColorKey(ctx, p.raw[pib], uint8(key), uint8(key>>8), uint8(key>>16), p.cfg.Limits)
	if !ok {
		return pic
	}
	q := drawingml.NewPicture(data, p.cfg)
	p.keyed[k] = q
	return q
}

// Recolored returns the file of the raster picture of the entry pib, from
// 1, with its colors changed by r, as a PNG file, or nil for a picture that
// it cannot decode, such as a metafile. It decodes the picture with ctx.
func (p *Pictures) Recolored(ctx context.Context, pib int, r drawingml.Recolor) []byte {
	k := recolorKey{pib: pib, r: r}
	if data, ok := p.recolored[k]; ok {
		return data
	}
	p.recolored[k] = nil
	data := p.Data(pib)
	if ctx == nil || len(data) == 0 || data[0] == 0x01 || data[0] == 0xD7 {
		return nil
	}
	img, err := raster.Decode(ctx, data, p.cfg.Limits)
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, r.Apply(img)); err != nil {
		return nil
	}
	p.recolored[k] = buf.Bytes()
	return p.recolored[k]
}
