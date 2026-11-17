// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/data.rs.

package syntax

import "sync"

// pdfData is Data in hayro-syntax src/data.rs. It holds the bytes of a PDF
// file and the decoded object streams, so that each object stream decodes
// once.
//
// Port: PdfData of upstream is a shared container of the bytes. Go shares a
// slice of bytes, so PdfData falls away.
type pdfData struct {
	data []byte
	// objectStreams has 32 segments, more than enough, as there cannot be
	// more objects than this.
	objectStreams *segmentList[*ObjectStream]
	mu            sync.Mutex
	m             map[ObjectIdentifier]int
	// failed holds the object streams that failed to decode.
	failed map[ObjectIdentifier]bool
	// endstream remembers the last search for endstream in data.
	endstream endstreamMemo
}

// holds reports whether b is the data of the document.
func (d *pdfData) holds(b []byte) bool {
	return len(b) == len(d.data) && (len(b) == 0 || &b[0] == &d.data[0])
}

// endstreamMemo remembers the result of the last search for the keyword
// endstream, so that a later search from an offset in the same gap returns
// at once.
//
// Port: upstream searches each time.
type endstreamMemo struct {
	mu sync.Mutex
	// valid is true after the first search.
	valid bool
	// from is the offset where the last search started. No keyword starts
	// between from and at.
	from int
	// at is the offset of the keyword that the last search found, or -1.
	at int
}

// find returns the offset of the first endstream in data at or after start.
func (m *endstreamMemo) find(data []byte, start int) (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.valid || start < m.from || (m.at >= 0 && start > m.at) {
		m.valid, m.from, m.at = true, start, -1
		if i, ok := findNeedle(data[start:], []byte("endstream")); ok {
			m.at = start + i
		}
	}
	return m.at, m.at >= 0
}

// newPdfData is Data::new in hayro-syntax src/data.rs.
func newPdfData(data []byte) *pdfData {
	return &pdfData{
		data:          data,
		objectStreams: newSegmentList[*ObjectStream](32),
		m:             map[ObjectIdentifier]int{},
		failed:        map[ObjectIdentifier]bool{},
	}
}

// get is Data::get in hayro-syntax src/data.rs.
func (d *pdfData) get() []byte {
	return d.data
}

// objectStream is Data::object_stream in hayro-syntax src/data.rs. It
// returns the decoded object stream with the identifier.
func (d *pdfData) objectStream(id ObjectIdentifier, ctx *ReaderContext) (*ObjectStream, bool) {
	d.mu.Lock()
	idx, ok := d.m[id]
	failed := d.failed[id]
	d.mu.Unlock()
	if ok {
		return d.cached(idx)
	}
	if failed {
		return nil, false
	}
	// The decoding can resolve other objects, so it runs without the lock
	// of the cache.
	stream, ok := ctx.XRef().getWith[Stream](id, ctx)
	if !ok {
		return nil, false
	}
	objectStream, ok := newObjectStream(stream)
	if !ok {
		// Port: upstream decodes the stream again on each access. A stream
		// that fails to decode, such as one past the memory limit, then
		// costs its full time for each member. The failure is kept.
		d.mu.Lock()
		d.failed[id] = true
		d.mu.Unlock()
		return nil, false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if idx, ok := d.m[id]; ok {
		return d.cached(idx)
	}
	idx = len(d.m)
	s := d.objectStreams.getOrInit(idx, func() *ObjectStream { return objectStream })
	if s == nil {
		return objectStream, true
	}
	d.m[id] = idx
	return *s, true
}

// cached returns the object stream at idx of the segment list.
func (d *pdfData) cached(idx int) (*ObjectStream, bool) {
	s, ok := d.objectStreams.get(idx)
	if !ok {
		return nil, false
	}
	return *s, true
}
