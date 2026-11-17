// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/page.rs.

package syntax

import (
	"sync"
)

// pagesContext is PagesContext in hayro-syntax src/page.rs. It holds the
// attributes that a page inherits from the nodes of the page tree.
type pagesContext struct {
	mediaBox  *Rect
	cropBox   *Rect
	rotate    *int32
	resources Dict
}

// Pages is Pages in hayro-syntax src/page.rs. It holds the pages of a PDF
// document.
type Pages struct {
	pages []*Page
	xref  *XRef
}

// newPages is Pages::new in hayro-syntax src/page.rs.
func newPages(pagesDict Dict, ctx *ReaderContext, xref *XRef) (*Pages, bool) {
	var pages []*Page
	if !resolvePages(pagesDict, &pages, pagesContext{}, ctx) {
		return nil, false
	}
	return &Pages{pages: pages, xref: xref}, true
}

// newPagesBruteForce is Pages::new_brute_force in hayro-syntax src/page.rs.
// It takes each dictionary with contents as a page.
func newPagesBruteForce(ctx *ReaderContext, xref *XRef) (*Pages, bool) {
	var pages []*Page
	for o := range xref.objects() {
		dict, ok := ObjectIntoDict(o)
		if !ok {
			continue
		}
		if page, ok := newPage(dict, &pagesContext{}, ctx, true); ok {
			pages = append(pages, page)
		}
	}
	if len(pages) == 0 {
		return nil, false
	}
	return &Pages{pages: pages, xref: xref}, true
}

// XRef is Pages::xref in hayro-syntax src/page.rs. It returns the xref table
// of the document of the pages.
func (p *Pages) XRef() *XRef {
	return p.xref
}

// Slice returns the pages, as the Deref of upstream to a slice does.
func (p *Pages) Slice() []*Page {
	return p.pages
}

// Len returns the number of pages.
func (p *Pages) Len() int {
	return len(p.pages)
}

// pageTreeFrame is PageTreeFrame in hayro-syntax src/page.rs.
type pageTreeFrame struct {
	kids *ArrayIter
	ctx  pagesContext
}

// newPageTreeFrame is PageTreeFrame::new in hayro-syntax src/page.rs.
func newPageTreeFrame(dict Dict, ctx pagesContext) (*pageTreeFrame, bool) {
	if v, ok := dict.Get[Rect](KeyMediaBox); ok {
		ctx.mediaBox = &v
	}
	if v, ok := dict.Get[Rect](KeyCropBox); ok {
		ctx.cropBox = &v
	}
	if v, ok := dict.Get[int32](KeyRotate); ok {
		ctx.rotate = &v
	}
	if resources, ok := dict.Get[Dict](KeyResources); ok {
		ctx.resources = resources
	}
	kids, ok := dict.Get[Array](KeyKids)
	if !ok {
		return nil, false
	}
	return &pageTreeFrame{kids: kids.RawIter(), ctx: ctx}, true
}

// resolvePages is resolve_pages in hayro-syntax src/page.rs. It walks the
// page tree with a stack, and it visits each referenced node once.
func resolvePages(pagesDict Dict, entries *[]*Page, ctx pagesContext, readerCtx *ReaderContext) bool {
	visited := map[ObjectIdentifier]bool{}
	if id, ok := pagesDict.ObjID(); ok {
		visited[id] = true
	}
	first, ok := newPageTreeFrame(pagesDict, ctx)
	if !ok {
		return false
	}
	stack := []*pageTreeFrame{first}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		kid, ok := frame.kids.Next()
		if !ok {
			stack = stack[:len(stack)-1]
			continue
		}
		var dict Dict
		if kid.IsRef {
			id := kid.Ref.ID()
			if visited[id] {
				continue
			}
			visited[id] = true
			dict, ok = readerCtx.XRef().getWith[Dict](id, readerCtx)
		} else {
			dict, ok = ObjectIntoDict(kid.Value)
		}
		if !ok {
			continue
		}
		if t, ok := dict.Get[Name](KeyType); ok && t == KeyPages {
			if child, ok := newPageTreeFrame(dict, frame.ctx); ok {
				stack = append(stack, child)
			}
			continue
		}
		// Lenient: take a missing or unknown Type as a page (corpus
		// 0083781).
		if page, ok := newPage(dict, &frame.ctx, readerCtx, false); ok {
			*entries = append(*entries, page)
		}
	}
	return true
}

// Rotation is Rotation in hayro-syntax src/page.rs. It is the rotation of a
// page.
type Rotation uint8

// The rotations of hayro-syntax src/page.rs.
const (
	// RotationNone is Rotation::None: no rotation.
	RotationNone Rotation = iota
	// RotationHorizontal is Rotation::Horizontal: a rotation of 90 degrees.
	RotationHorizontal
	// RotationFlipped is Rotation::Flipped: a rotation of 180 degrees.
	RotationFlipped
	// RotationFlippedHorizontal is Rotation::FlippedHorizontal: a rotation
	// of 270 degrees.
	RotationFlippedHorizontal
)

// Page is Page in hayro-syntax src/page.rs. It is a page of a PDF document.
type Page struct {
	inner       Dict
	mediaBox    Rect
	cropBox     Rect
	rotation    Rotation
	streamsOnce sync.Once
	streams     []byte
	hasStreams  bool
	resources   *Resources
	ctx         *ReaderContext
}

// newPage is Page::new in hayro-syntax src/page.rs.
func newPage(dict Dict, ctx *pagesContext, readerCtx *ReaderContext, bruteForce bool) (*Page, bool) {
	if bruteForce && !dict.ContainsKey(KeyContents) {
		return nil, false
	}
	mediaBox, ok := dict.Get[Rect](KeyMediaBox)
	if !ok {
		mediaBox = A4
		if ctx.mediaBox != nil {
			mediaBox = *ctx.mediaBox
		}
	}
	cropBox, ok := dict.Get[Rect](KeyCropBox)
	if !ok {
		cropBox = mediaBox
		if ctx.cropBox != nil {
			cropBox = *ctx.cropBox
		}
	}
	rotate, ok := dict.Get[int32](KeyRotate)
	if !ok {
		rotate = 0
		if ctx.rotate != nil {
			rotate = *ctx.rotate
		}
	}
	var rotation Rotation
	switch ((rotate % 360) + 360) % 360 {
	case 90:
		rotation = RotationHorizontal
	case 180:
		rotation = RotationFlipped
	case 270:
		rotation = RotationFlippedHorizontal
	default:
		rotation = RotationNone
	}
	resources, ok := dict.Get[Dict](KeyResources)
	if !ok {
		resources = ctx.resources
	}
	return &Page{
		inner:     dict,
		mediaBox:  mediaBox,
		cropBox:   cropBox,
		rotation:  rotation,
		resources: NewResources(resources),
		ctx:       readerCtx.clone(),
	}, true
}

// operationsImpl is Page::operations_impl in hayro-syntax src/page.rs.
func (p *Page) operationsImpl() (*UntypedIter, bool) {
	stream, ok := p.PageStream()
	if !ok {
		return nil, false
	}
	return NewUntypedIter(stream), true
}

// PageStream is Page::page_stream in hayro-syntax src/page.rs. It returns
// the decoded content stream of the page. An array of streams becomes one
// stream, with a space after each.
//
// Port: the joined streams stop with a failure past the memory limit of the
// document.
func (p *Page) PageStream() ([]byte, bool) {
	p.streamsOnce.Do(func() {
		if stream, ok := p.inner.Get[Stream](KeyContents); ok {
			if data, err := stream.DecodedData(); err == nil {
				p.streams, p.hasStreams = data, true
			}
			return
		}
		array, ok := p.inner.Get[Array](KeyContents)
		if !ok {
			return
		}
		maxMemory := p.ctx.XRef().decodeEnv().maxMemory
		var collected []byte
		it := array.RawIter()
		for {
			item, ok := it.Next()
			if !ok {
				break
			}
			o, ok := item.Resolve(p.ctx)
			if !ok {
				continue
			}
			stream, ok := ObjectIntoStream(o)
			if !ok {
				continue
			}
			data, err := stream.DecodedData()
			if err != nil {
				continue
			}
			if int64(len(collected))+int64(len(data))+1 > maxMemory {
				return
			}
			collected = append(collected, data...)
			// Streams must have at least one white space between them.
			collected = append(collected, ' ')
		}
		p.streams, p.hasStreams = collected, true
	})
	return p.streams, p.hasStreams
}

// Resources is Page::resources in hayro-syntax src/page.rs.
func (p *Page) Resources() *Resources {
	return p.resources
}

// MediaBox is Page::media_box in hayro-syntax src/page.rs.
func (p *Page) MediaBox() Rect {
	return p.mediaBox
}

// Rotation is Page::rotation in hayro-syntax src/page.rs.
func (p *Page) Rotation() Rotation {
	return p.rotation
}

// CropBox is Page::crop_box in hayro-syntax src/page.rs.
func (p *Page) CropBox() Rect {
	return p.cropBox
}

// IntersectedCropBox is Page::intersected_crop_box in hayro-syntax
// src/page.rs. It returns the intersection of the crop box and the media
// box.
func (p *Page) IntersectedCropBox() Rect {
	return p.CropBox().Intersect(p.MediaBox())
}

// BaseDimensions is Page::base_dimensions in hayro-syntax src/page.rs. It
// returns the size of the intersected crop box, or A4 for a page without
// area.
func (p *Page) BaseDimensions() (float32, float32) {
	cropBox := p.IntersectedCropBox()
	if isNearlyZero(float32(cropBox.Width())) || isNearlyZero(float32(cropBox.Height())) {
		return float32(A4.Width()), float32(A4.Height())
	}
	return float32(rustMax(cropBox.Width(), 1)), float32(rustMax(cropBox.Height(), 1))
}

// RenderDimensions is Page::render_dimensions in hayro-syntax src/page.rs.
// It returns the width and the height after the rotation.
func (p *Page) RenderDimensions() (float32, float32) {
	w, h := p.BaseDimensions()
	if p.rotation == RotationHorizontal || p.rotation == RotationFlippedHorizontal {
		w, h = h, w
	}
	return w, h
}

// Operations is Page::operations in hayro-syntax src/page.rs. It returns an
// untyped iterator over the operators of the content stream.
func (p *Page) Operations() *UntypedIter {
	if it, ok := p.operationsImpl(); ok {
		return it
	}
	return EmptyUntypedIter()
}

// Raw is Page::raw in hayro-syntax src/page.rs. It returns the dictionary of
// the page.
func (p *Page) Raw() Dict {
	return p.inner
}

// XRef is Page::xref in hayro-syntax src/page.rs.
func (p *Page) XRef() *XRef {
	return p.ctx.XRef()
}

// TypedOperations is Page::typed_operations in hayro-syntax src/page.rs. It
// returns a typed iterator over the operators of the content stream.
func (p *Page) TypedOperations() *TypedIter {
	return typedIterFromUntyped(p.Operations())
}

// InitialTransform is Page::initial_transform in hayro-syntax src/page.rs.
// It returns the transform to start the rendering of the page with.
func (p *Page) InitialTransform(invertY bool) Transform {
	cropBox := p.IntersectedCropBox()
	_, baseHeight := p.BaseDimensions()
	width, height := p.RenderDimensions()
	horizontal := transformRotateCW90.Mul(transformTranslate(0, -float64(width)))
	flippedHorizontal := transformTranslate(0, float64(height)).Mul(transformRotateCCW90)
	var rotation Transform
	switch p.rotation {
	case RotationHorizontal:
		rotation = flippedHorizontal
		if invertY {
			rotation = horizontal
		}
	case RotationFlipped:
		rotation = transformScale(-1).Mul(transformTranslate(-float64(width), -float64(height)))
	case RotationFlippedHorizontal:
		rotation = horizontal
		if invertY {
			rotation = flippedHorizontal
		}
	default:
		rotation = TransformIdentity
	}
	inversion := TransformIdentity
	if invertY {
		inversion = Transform{1, 0, 0, -1, 0, float64(baseHeight)}
	}
	return rotation.Mul(inversion).Mul(transformTranslate(-cropBox.X0, -cropBox.Y0))
}

// Resources is Resources in hayro-syntax src/page.rs. It holds the
// dictionaries of the resources of a page.
type Resources struct {
	// ExtGStates is the raw dictionary of the external graphics states.
	ExtGStates Dict
	// Fonts is the raw dictionary of the fonts.
	Fonts Dict
	// Properties is the raw dictionary of the properties.
	Properties Dict
	// ColorSpaces is the raw dictionary of the color spaces.
	ColorSpaces Dict
	// XObjects is the raw dictionary of the external objects.
	XObjects Dict
	// Patterns is the raw dictionary of the patterns.
	Patterns Dict
	// Shadings is the raw dictionary of the shadings.
	Shadings Dict
}

// NewResources is Resources::new in hayro-syntax src/page.rs.
func NewResources(resources Dict) *Resources {
	get := func(key Name) Dict {
		d, _ := resources.Get[Dict](key)
		return d
	}
	return &Resources{
		ExtGStates:  get(KeyExtGState),
		Fonts:       get(KeyFont),
		ColorSpaces: get(KeyColorSpace),
		Properties:  get(KeyProperties),
		XObjects:    get(KeyXObject),
		Patterns:    get(KeyPattern),
		Shadings:    get(KeyShading),
	}
}

// GetExtGState is Resources::get_ext_g_state in hayro-syntax src/page.rs.
func (r *Resources) GetExtGState(name Name) (Dict, bool) {
	return r.ExtGStates.Get[Dict](name)
}

// GetColorSpace is Resources::get_color_space in hayro-syntax src/page.rs.
func (r *Resources) GetColorSpace(name Name) (Object, bool) {
	return r.ColorSpaces.Get[Object](name)
}

// GetFont is Resources::get_font in hayro-syntax src/page.rs.
func (r *Resources) GetFont(name Name) (Dict, bool) {
	return r.Fonts.Get[Dict](name)
}

// GetPattern is Resources::get_pattern in hayro-syntax src/page.rs.
func (r *Resources) GetPattern(name Name) (Object, bool) {
	return r.Patterns.Get[Object](name)
}

// GetXObject is Resources::get_x_object in hayro-syntax src/page.rs.
func (r *Resources) GetXObject(name Name) (Stream, bool) {
	return r.XObjects.Get[Stream](name)
}

// GetShading is Resources::get_shading in hayro-syntax src/page.rs.
func (r *Resources) GetShading(name Name) (Object, bool) {
	return r.Shadings.Get[Object](name)
}

// pointsPerInch is POINTS_PER_INCH in hayro-syntax src/page.rs.
const pointsPerInch = 72.0

// pointsPerMM is POINTS_PER_MM in hayro-syntax src/page.rs.
const pointsPerMM = 1.0 / (10.0 * 2.54) * pointsPerInch

// A4 is A4 in hayro-syntax src/page.rs. It is the size of an A4 page.
var A4 = Rect{X0: 0, Y0: 0, X1: 210 * pointsPerMM, Y1: 297 * pointsPerMM}

// cachedPages is CachedPages in hayro-syntax src/page.rs, in the module
// cached. It reads the pages of the document once.
//
// Port: upstream ties the lifetime of the pages to the xref table with a
// transmute. Go needs nothing of the kind.
func cachedPages(xref *XRef) (*Pages, bool) {
	ctx := newReaderContext(xref, false)
	if p, ok := xref.getWith[Dict](xref.trailerData.pagesRef, ctx); ok {
		if pages, ok := newPages(p, ctx, xref); ok {
			return pages, true
		}
	}
	return newPagesBruteForce(ctx, xref)
}
