package odraw

import (
	"context"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
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
		store: store,
		delay: delay,
		cfg:   cfg,
		pics:  make(map[int]*drawingml.Picture),
		raw:   make(map[int][]byte),
		keyed: make(map[pictureKey]*drawingml.Picture),
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
