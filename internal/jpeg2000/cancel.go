// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/cancel.rs.

// CancelCheck is CancelCheck in hayro-jpeg2000 src/cancel.rs. It is a check
// that cancels the decoding.
type CancelCheck interface {
	// IsCancelled returns whether the decoding must stop.
	IsCancelled() bool
}

// CancelFunc is the implementation of CancelCheck for a closure in
// hayro-jpeg2000 src/cancel.rs. A caller with a context can pass
// CancelFunc(func() bool { return ctx.Err() != nil }).
type CancelFunc func() bool

// IsCancelled calls the function.
func (f CancelFunc) IsCancelled() bool {
	return f()
}

// CancelHandle is CancelHandle in hayro-jpeg2000 src/cancel.rs. It is a
// shared CancelCheck that the decode settings can hold.
//
// Port: upstream shares the check with an Arc. Go shares the pointer.
type CancelHandle struct {
	check CancelCheck
}

// NewCancelHandle is CancelHandle::new. It returns a handle for the check.
func NewCancelHandle(check CancelCheck) *CancelHandle {
	return &CancelHandle{check: check}
}

// IsCancelled is CancelHandle::is_cancelled. It returns whether the decoding
// must stop.
func (h *CancelHandle) IsCancelled() bool {
	return h != nil && h.check != nil && h.check.IsCancelled()
}

// chunk is CHUNK: the number of samples that the decoder processes between
// two checks.
const chunk = 1 << 16

// cancel is Cancel in hayro-jpeg2000 src/cancel.rs.
type cancel struct {
	handle *CancelHandle
}

// isCancelled is Cancel::is_cancelled.
func (c cancel) isCancelled() bool {
	return c.handle.IsCancelled()
}

// check is Cancel::check. It returns errCancelled when the decoding must
// stop.
func (c cancel) check() error {
	if c.isCancelled() {
		return errCancelled
	}
	return nil
}

// resize is Cancel::resize. It returns buf with the length n. The samples
// after the old length are zero.
//
// Port: Go cannot resize a slice in place, so resize returns the slice.
func (c cancel) resize(buf []float32, n int) ([]float32, error) {
	if len(buf) > n {
		buf = buf[:n]
	}
	if cap(buf) < n {
		grown := make([]float32, len(buf), n)
		copy(grown, buf)
		buf = grown
	}
	for len(buf) < n {
		if err := c.check(); err != nil {
			return buf, err
		}
		next := min(n, len(buf)+chunk)
		old := len(buf)
		buf = buf[:next]
		clear(buf[old:next])
	}
	return buf, nil
}
