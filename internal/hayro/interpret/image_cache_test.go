// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the tests of the cache of the decoded images, which
// upstream does not have.

package interpret

import (
	"sync"
	"sync/atomic"
	"testing"
)

// imageKey returns a key for the tests.
func imageKey(n uint64) Key128 {
	return newKeyHasher().u64(n).sum()
}

// sized returns a decode function that counts its calls in n, and returns v
// with a size that, with the overhead, is size bytes.
func sized(n *atomic.Int64, v any, size int64) func() (any, int64, bool) {
	return func() (any, int64, bool) {
		n.Add(1)
		return v, size - imageEntryOverhead, true
	}
}

func TestImageCacheEvicts(t *testing.T) {
	c := newImageCache()
	var n atomic.Int64
	const budget = 3000
	for i := range uint64(2) {
		if v, ok := c.get(imageKey(i), budget, sized(&n, i, 1000), nil); !ok || v != i {
			t.Fatalf("expected %d, got %v and %t", i, v, ok)
		}
	}
	// a hit moves the key 0 to the front, so the key 1 is the least
	// recently used
	c.get(imageKey(0), budget, sized(&n, 0, 1000), nil)
	if got := n.Load(); got != 2 {
		t.Errorf("expected 2 decodings, got %d", got)
	}
	c.get(imageKey(2), budget, sized(&n, 2, 1500), nil)
	if c.used > budget {
		t.Errorf("expected at most %d bytes, got %d", budget, c.used)
	}
	if _, ok := c.entries[imageKey(1)]; ok {
		t.Error("expected the key 1 to be evicted")
	}
	for _, i := range []uint64{0, 2} {
		if _, ok := c.entries[imageKey(i)]; !ok {
			t.Errorf("expected the key %d to stay", i)
		}
	}
	// an entry larger than the budget is not kept
	c.get(imageKey(3), budget, sized(&n, 3, budget+1), nil)
	if _, ok := c.entries[imageKey(3)]; ok {
		t.Error("expected the key 3 not to be kept")
	}
	if c.used != 2500 || c.order.Len() != 2 {
		t.Errorf("expected 2 entries of 2500 bytes, got %d of %d bytes", c.order.Len(), c.used)
	}
}

func TestImageCacheKeep(t *testing.T) {
	c := newImageCache()
	var n atomic.Int64
	// a failed decoding and a decoding that keep refuses are not kept
	c.get(imageKey(0), 1000, func() (any, int64, bool) { return nil, 0, false }, nil)
	c.get(imageKey(1), 1000, sized(&n, 1, 500), func() bool { return false })
	if len(c.entries) != 0 || c.used != 0 {
		t.Errorf("expected an empty cache, got %d entries of %d bytes", len(c.entries), c.used)
	}
}

func TestImageCacheDecodesOnce(t *testing.T) {
	c := newImageCache()
	var n atomic.Int64
	started, release := make(chan struct{}), make(chan struct{})
	decode := func() (any, int64, bool) {
		if n.Add(1) == 1 {
			close(started)
			<-release
		}
		return "image", 100, true
	}
	key := imageKey(0)
	results := make([]any, 8)
	var waiting sync.WaitGroup
	waiting.Add(len(results) - 1)
	c.onWait = waiting.Done
	var wg sync.WaitGroup
	wg.Go(func() {
		results[0], _ = c.get(key, 1000, decode, nil)
	})
	// the first call decodes, so the others find its entry and wait
	<-started
	for i := 1; i < len(results); i++ {
		wg.Go(func() {
			results[i], _ = c.get(key, 1000, decode, nil)
		})
	}
	waiting.Wait()
	close(release)
	wg.Wait()
	if got := n.Load(); got != 1 {
		t.Errorf("expected 1 decoding, got %d", got)
	}
	for i, v := range results {
		if v != "image" {
			t.Errorf("call %d: expected the image, got %v", i, v)
		}
	}
}

func TestImageCacheLeaderFails(t *testing.T) {
	c := newImageCache()
	var n atomic.Int64
	started, release := make(chan struct{}), make(chan struct{})
	key := imageKey(0)
	var wg sync.WaitGroup
	var leader, waiter any
	var leaderOK, waiterOK bool
	var waiting sync.WaitGroup
	waiting.Add(1)
	c.onWait = waiting.Done
	wg.Go(func() {
		defer func() {
			// the panic of the decoding goes up to the caller
			if recover() == nil {
				t.Error("expected a panic")
			}
		}()
		leader, leaderOK = c.get(key, 1000, func() (any, int64, bool) {
			n.Add(1)
			close(started)
			<-release
			panic("decode")
		}, nil)
	})
	<-started
	wg.Go(func() {
		// the call waits, and decodes itself after the first call fails
		waiter, waiterOK = c.get(key, 1000, sized(&n, "image", 500), nil)
	})
	waiting.Wait()
	close(release)
	wg.Wait()
	if leaderOK || leader != nil {
		t.Errorf("expected no image from the panic, got %v", leader)
	}
	if !waiterOK || waiter != "image" {
		t.Errorf("expected the image, got %v and %t", waiter, waiterOK)
	}
	if got := n.Load(); got != 2 {
		t.Errorf("expected 2 decodings, got %d", got)
	}
}
