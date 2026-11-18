// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports no upstream file. It keeps the decoded images of a
// document between the renders, which upstream does not do.

package interpret

import (
	"container/list"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// imageCacheShare is the share of Limits.MaxMemory that the decoded images
// of a document can keep: a quarter. The default is 128 MiB.
const imageCacheShare = 4

// imageEntryOverhead is the number of bytes that an entry of imageCache
// counts on top of its pixels.
const imageEntryOverhead = 256

// imageCache keeps the decoded images of a document between the renders, in
// the order of their last use. It is safe for concurrent use. Two calls that
// miss on the same key at the same time decode the image once: the second
// call waits for the first.
//
// Port: upstream decodes each image again at each draw. A page of ivy
// renders in bands (D47), so each band decoded each image of the page.
type imageCache struct {
	mu      sync.Mutex
	used    int64
	entries map[Key128]*imageEntry
	// order holds the stored entries, the most recently used first.
	order *list.List
	// onWait, if it is set, runs when a call starts to wait for another
	// call. The tests set it.
	onWait func()
}

// imageEntry is an entry of imageCache.
type imageEntry struct {
	key Key128
	// done is closed when the decoding of the entry ends.
	done chan struct{}
	val  any
	size int64
	ok   bool
	// elem is the element of the entry in order, or nil while the entry
	// decodes.
	elem *list.Element
}

// newImageCache returns an empty imageCache.
func newImageCache() *imageCache {
	return &imageCache{entries: make(map[Key128]*imageEntry), order: list.New()}
}

// get returns the value of the key. If the cache does not hold the key, get
// calls decode, which returns the value, its size in bytes, and whether the
// value is valid. get keeps a valid value of a size up to budget, unless
// keep returns false. It then removes the values that were used least
// recently, until the values fit in budget.
//
// If another call decodes the same key, get waits for it. If that call
// fails, get calls decode itself.
func (c *imageCache) get(key Key128, budget int64, decode func() (any, int64, bool), keep func() bool) (any, bool) {
	c.mu.Lock()
	if e, found := c.entries[key]; found {
		if e.elem != nil {
			c.order.MoveToFront(e.elem)
			c.mu.Unlock()
			return e.val, e.ok
		}
		onWait := c.onWait
		c.mu.Unlock()
		if onWait != nil {
			onWait()
		}
		<-e.done
		if e.ok {
			return e.val, true
		}
		v, _, ok := decode()
		return v, ok
	}
	e := &imageEntry{key: key, done: make(chan struct{})}
	c.entries[key] = e
	c.mu.Unlock()
	// The deferred function also runs when decode panics, so that the calls
	// that wait do not wait forever.
	defer c.finish(e, budget, keep)
	e.val, e.size, e.ok = decode()
	return e.val, e.ok
}

// finish stores the entry that a call of get decoded, or removes it, and
// wakes the calls that wait for it.
func (c *imageCache) finish(e *imageEntry, budget int64, keep func() bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer close(e.done)
	e.size += imageEntryOverhead
	if !e.ok || e.size > budget || (keep != nil && !keep()) {
		delete(c.entries, e.key)
		return
	}
	e.elem = c.order.PushFront(e)
	c.used += e.size
	for c.used > budget {
		last := c.order.Back()
		old := last.Value.(*imageEntry)
		c.order.Remove(last)
		delete(c.entries, old.key)
		c.used -= old.size
	}
}

// ImageBudget returns the number of bytes that the cache of the decoded
// images of a document can keep with the limits.
func ImageBudget(lim ivy.Limits) int64 {
	return lim.WithDefaults().MaxMemory / imageCacheShare
}

// imageBudget returns the number of bytes that the decoded images can keep
// with the limits of the environment.
func (e *imageEnv) imageBudget() int64 {
	return ImageBudget(e.limits)
}

// DerivedImage returns the value that build returns for the decoded image of
// key and the numbers of params, such as the size of a resized copy. The
// cache keeps the value with the decoded images, up to budget bytes, unless
// keep returns false, such as for an interpretation that Cancel stopped.
// build returns the value, its size in bytes, and whether the value is
// valid. The caller must not change the value.
func (c *InterpreterCache) DerivedImage(key Key128, params []uint64, budget int64, build func() (any, int64, bool), keep func() bool) (any, bool) {
	h := newKeyHasher().str("derived image").key(key)
	for _, p := range params {
		h.u64(p)
	}
	return c.objectCache.images.get(h.sum(), budget, build, keep)
}

// DecodedKey returns the key of the decoded mask for the size, and false
// when the cache does not keep the mask.
func (s *StencilImage) DecodedKey(targetDimension [2]uint32) (Key128, bool) {
	return s.imageXObject.decodedCacheKey(targetDimension)
}

// DecodedKey returns the key of the decoded image for the size, and false
// when the cache does not keep the image.
func (r *RasterImage) DecodedKey(targetDimension [2]uint32) (Key128, bool) {
	return r.imageXObject.decodedCacheKey(targetDimension)
}

// canceled reports whether the interpretation that decodes the image
// stopped. A decoder that stops can return a part of the image.
func (e *imageEnv) canceled() bool {
	return e.cancel != nil && e.cancel()
}

// decodedCacheKey returns the key of the decoded image, and false if the
// image must not be cached. The key holds the stream of the image, the kind
// of the image and the size that the decoder of JPX aims at. The
// dictionary of the stream fixes the rest: the color space, the decode
// array, the soft mask and the interpolation. An inline image, an image
// with a transfer function and an image whose color space is a name of the
// resources are not cached.
func (x *imageXObject) decodedCacheKey(targetDimension [2]uint32) (Key128, bool) {
	if x.cache == nil || x.cache.images == nil || x.transferFunction != nil || x.namedColorSpace {
		return Key128{}, false
	}
	if _, ok := x.stream.Dict().ObjID(); !ok {
		return Key128{}, false
	}
	h := newKeyHasher().str("decoded image").key(streamCacheKey(x.stream)).u64(uint64(x.kind))
	if usesTargetDimension(x.stream) {
		h.u64(uint64(targetDimension[0])).u64(uint64(targetDimension[1]))
	}
	return h.sum(), true
}

// usesTargetDimension reports whether the decoding of the image or of its
// mask depends on the size that the device needs. Only the decoder of JPX
// reads it.
func usesTargetDimension(s syntax.Stream) bool {
	if hasJPXFilter(s) {
		return true
	}
	dict := s.Dict()
	for _, key := range []syntax.Name{syntax.KeySMask, syntax.KeyMask} {
		if m, ok := dict.Get[syntax.Stream](key); ok && hasJPXFilter(m) {
			return true
		}
	}
	return false
}

// hasJPXFilter reports whether a filter of the stream is JPXDecode.
func hasJPXFilter(s syntax.Stream) bool {
	for _, f := range s.Filters() {
		if f == syntax.FilterJPXDecode {
			return true
		}
	}
	return false
}

// lumaSize returns the number of bytes of the data of l.
func lumaSize(l *LumaData) int64 {
	if l == nil {
		return 0
	}
	return int64(len(l.Data))
}

// imageDataSize returns the number of bytes of the data of d.
func imageDataSize(d ImageData) int64 {
	n := lumaSize(d.Luma)
	if d.RGB != nil {
		n += int64(len(d.RGB.Data))
	}
	return n
}
