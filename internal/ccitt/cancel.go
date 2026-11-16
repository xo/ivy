// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-ccitt src/cancel.rs.
//
// Port: upstream wraps the check in a CancelHandle, so that the settings can
// share it. A Go interface value is already shared, so DecodeSettings holds
// the CancelCheck itself, and CancelHandle is not ported.

package ccitt

// CancelCheck is CancelCheck in hayro-ccitt src/cancel.rs. It is a check for
// the cancellation of decoding. The decoder calls it before each row.
type CancelCheck interface {
	// IsCancelled reports whether decoding must stop.
	IsCancelled() bool
}

// CancelFunc is the implementation of CancelCheck for a closure in
// hayro-ccitt src/cancel.rs. For example, the function func() bool { return
// ctx.Err() != nil } stops the decoder when a context ends.
type CancelFunc func() bool

// IsCancelled calls f.
func (f CancelFunc) IsCancelled() bool {
	return f()
}

// isCancelled reports whether the check of settings stops the decoder. It
// is the test settings.cancel.as_ref().is_some_and(|cancel|
// cancel.is_cancelled()) of hayro-ccitt src/group3.rs and src/group4.rs.
func isCancelled(settings *DecodeSettings) bool {
	return settings.Cancel != nil && settings.Cancel.IsCancelled()
}
