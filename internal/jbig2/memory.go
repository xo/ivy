// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/memory.rs.

// memoryBudget is MemoryBudget in hayro-jbig2 src/memory.rs. It counts the
// bytes that a decode holds. The bytes of the current segment are free at the
// end of the segment, unless the decoder retains them.
type memoryBudget struct {
	limit    int
	retained int
	segment  int
}

// newMemoryBudget is MemoryBudget::new. A negative limit is the None of
// upstream, which means no limit.
func newMemoryBudget(limit int) memoryBudget {
	if limit < 0 {
		limit = int(^uint(0) >> 1)
	}
	return memoryBudget{limit: limit}
}

// charge is MemoryBudget::charge. It returns ErrMemoryLimitExceeded when the
// bytes do not fit in the limit.
func (b *memoryBudget) charge(bytes int) error {
	used := b.retained + b.segment
	if bytes < 0 || bytes > b.limit-used {
		return ErrMemoryLimitExceeded
	}
	b.segment += bytes
	return nil
}

// release is MemoryBudget::release.
func (b *memoryBudget) release(bytes int) {
	b.segment = max(b.segment-bytes, 0)
}

// retain is MemoryBudget::retain. It moves bytes of the current segment to
// the bytes that the decoder keeps.
func (b *memoryBudget) retain(bytes int) {
	bytes = min(bytes, b.segment)
	b.segment -= bytes
	b.retained += bytes
}

// endSegment is MemoryBudget::end_segment.
func (b *memoryBudget) endSegment() {
	b.segment = 0
}
