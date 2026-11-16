// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/cancel.rs.

import "math/bits"

// CancelCheck is CancelCheck in hayro-jbig2 src/cancel.rs. It is a check for
// the cancellation of decoding. The decoder calls it from time to time.
//
// Port: upstream wraps the check in a CancelHandle, so that the settings can
// share it. A Go interface value is already shared, so DecodeSettings holds
// the CancelCheck itself.
type CancelCheck interface {
	// IsCancelled reports whether decoding must stop.
	IsCancelled() bool
}

// CancelFunc is the implementation of CancelCheck for a closure in
// hayro-jbig2 src/cancel.rs. For example, the function func() bool { return
// ctx.Err() != nil } stops the decoder when a context ends.
type CancelFunc func() bool

// IsCancelled calls f.
func (f CancelFunc) IsCancelled() bool {
	return f()
}

// cancel is Cancel in hayro-jbig2 src/cancel.rs. A nil handle never cancels.
type cancel struct {
	handle CancelCheck
}

// check is Cancel::check. It returns ErrCancelled when decoding must stop.
func (c cancel) check() error {
	if c.handle != nil && c.handle.IsCancelled() {
		return ErrCancelled
	}
	return nil
}

// checkInterval is CHECK_INTERVAL, the amount of work between two checks, in
// units of about one decoded pixel.
const checkInterval uint32 = 1 << 16

// wordWork is WORD_WORK. To copy or combine a word of pixels costs about as
// much as to decode four pixels.
const wordWork uint32 = 4

// fillChunk is FILL_CHUNK. To fill a byte costs about 1/64 of the decoding of
// a pixel.
const fillChunk = 64 * int(checkInterval)

// workCounter is WorkCounter in hayro-jbig2 src/cancel.rs.
type workCounter uint64

// add is WorkCounter::add. It checks for cancellation after each
// checkInterval units of work.
func (w *workCounter) add(work uint64, c cancel) error {
	*w += workCounter(work)
	if *w >= workCounter(checkInterval) {
		*w = 0
		return c.check()
	}
	return nil
}

// copyWork is copy_work in hayro-jbig2 src/cancel.rs.
func copyWork(width uint32) uint32 {
	return divCeil(width, wordBits) * wordWork
}

// rowMask is row_mask in hayro-jbig2 src/cancel.rs. A caller checks for
// cancellation on each row y where y&mask == mask. Each row has a fixed cost
// too, which matters for a very narrow bitmap.
func rowMask(rowWork uint32) uint32 {
	rows := min(max(checkInterval/max(rowWork, 1), 1), 1024)
	return 1<<(bits.Len32(rows)-1) - 1
}

// divCeil is u32::div_ceil of the Rust core library.
func divCeil(a, b uint32) uint32 {
	q := a / b
	if a%b != 0 {
		q++
	}
	return q
}
