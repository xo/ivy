// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/cache.rs.

package interpret

import (
	"sync"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/pdf/internal/syntax"
)

// cacheEntry is an entry of Cache: a value, or the None of upstream.
type cacheEntry struct {
	val any
	ok  bool
}

// Cache is Cache in hayro-interpret src/cache.rs. It holds the objects that
// the interpreter reads once and uses many times, such as color spaces and
// shadings. It is safe for concurrent use.
//
// Port: upstream shares the map through an Arc. Go shares the pointer.
type Cache struct {
	mu sync.Mutex
	m  map[Key128]cacheEntry
}

// newCache is Cache::new in hayro-interpret src/cache.rs.
func newCache() *Cache {
	return &Cache{m: make(map[Key128]cacheEntry)}
}

// getOrInsertWith is Cache::get_or_insert_with in hayro-interpret
// src/cache.rs. It returns the value of the key, and calls f for the value
// when the cache does not hold the key. A failure of f is cached too.
//
// Port: upstream downcasts the value to T, and a value of another type
// gives None. Go does the same with a type assertion.
func cacheGetOrInsertWith[T any](c *Cache, id Key128, f func() (T, bool)) (T, bool) {
	c.mu.Lock()
	e, found := c.m[id]
	c.mu.Unlock()
	if found {
		v, ok := e.val.(T)
		return v, ok && e.ok
	}
	// The lock is free while f runs, because f can use the cache.
	v, ok := f()
	c.mu.Lock()
	c.m[id] = cacheEntry{val: v, ok: ok}
	c.mu.Unlock()
	return v, ok
}

// CacheKey is CacheKey in hayro-interpret src/cache.rs. A type that
// implements it returns a key that identifies its value.
type CacheKey interface {
	// CacheKey returns the key of the value.
	CacheKey() Key128
}

// pairCacheKey is the CacheKey implementation of (T, U) in hayro-interpret
// src/cache.rs.
func pairCacheKey(a, b Key128) Key128 {
	return newKeyHasher().key(a).key(b).sum()
}

// dictCacheKey is the CacheKey implementation of Dict in hayro-interpret
// src/cache.rs. It hashes the raw data of the dictionary.
func dictCacheKey(d syntax.Dict) Key128 {
	return newKeyHasher().str("dict").bytes(d.Data()).sum()
}

// streamCacheKey is the CacheKey implementation of Stream in
// hayro-interpret src/cache.rs. It hashes the identifier of the stream, or
// its dictionary and its raw data if it has no identifier.
func streamCacheKey(s syntax.Stream) Key128 {
	if id, ok := s.Dict().ObjID(); ok {
		return newKeyHasher().str("obj").u64(uint64(id.ObjNumber)).u64(uint64(uint32(id.GenNumber))).sum()
	}
	return newKeyHasher().str("stream").bytes(s.Dict().Data()).bytes(s.RawData()).sum()
}

// resourcesCacheKey is the CacheKey implementation of Resources in
// hayro-interpret src/cache.rs.
func resourcesCacheKey(r *syntax.Resources) Key128 {
	h := newKeyHasher()
	if r == nil {
		return h.sum()
	}
	for _, d := range []syntax.Dict{r.ExtGStates, r.Fonts, r.Properties, r.ColorSpaces, r.XObjects, r.Patterns, r.Shadings} {
		h.key(dictCacheKey(d))
	}
	return h.sum()
}

// objectCacheKey is the CacheKey implementation of Object in
// hayro-interpret src/cache.rs.
func objectCacheKey(o syntax.Object) Key128 {
	h := newKeyHasher()
	switch v := o.(type) {
	case syntax.Null:
		return h.str("null").sum()
	case syntax.Bool:
		return h.str("bool").boolean(bool(v)).sum()
	case syntax.Number:
		return h.str("number").f64(v.AsF64()).sum()
	case syntax.String:
		return h.str("string").bytes(v.AsBytes()).sum()
	case syntax.Name:
		return h.str("name").str(string(v)).sum()
	case syntax.Dict:
		return dictCacheKey(v)
	case syntax.Array:
		return h.str("array").bytes(v.Data()).sum()
	case syntax.Stream:
		return streamCacheKey(v)
	}
	return h.sum()
}

// affineCacheKey is the CacheKey implementation of Affine in
// hayro-interpret src/cache.rs.
func affineCacheKey(a kurbo.Affine) Key128 {
	h := newKeyHasher()
	for _, c := range a {
		h.f64(c)
	}
	return h.sum()
}

// rectCacheKey is the CacheKey implementation of Rect in hayro-interpret
// src/cache.rs.
func rectCacheKey(r kurbo.Rect) Key128 {
	return newKeyHasher().f64(r.X0).f64(r.Y0).f64(r.X1).f64(r.Y1).sum()
}
