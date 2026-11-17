// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/sync.rs.
//
// Port: upstream picks the types of std or of core for Arc, OnceLock, Mutex
// and RwLock. Go uses the package sync, and onceCell stands for OnceLock.

package syntax

import (
	"sync"
	"sync/atomic"
)

// onceCell is OnceLock in hayro-syntax src/sync.rs. It holds a value that is
// set once.
type onceCell[T any] struct {
	mu   sync.Mutex
	done atomic.Bool
	v    T
}

// get returns the value when it is set.
func (c *onceCell[T]) get() (*T, bool) {
	if !c.done.Load() {
		return nil, false
	}
	return &c.v, true
}

// getOrInit returns the value, and sets it with f when it is not set.
func (c *onceCell[T]) getOrInit(f func() T) *T {
	if c.done.Load() {
		return &c.v
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.done.Load() {
		c.v = f()
		c.done.Store(true)
	}
	return &c.v
}
