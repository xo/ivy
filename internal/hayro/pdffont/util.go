// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/util.rs.

package pdffont

import (
	"encoding/binary"
	"hash"
	"hash/fnv"

	"github.com/xo/ivy/internal/hayro/syntax"
)

// Key128 is the type u128 of the keys of hayro-font. A key identifies a
// font program, a glyph or a dictionary in a cache.
//
// Port: Go has no 128-bit integer. A key is 16 bytes, and Go can compare
// two keys with ==.
type Key128 [16]byte

// keyHasher is the state of hash128 in hayro-font src/util.rs. It hashes a
// sequence of values into a Key128.
//
// Port: upstream hashes with SipHash-1-3 through the trait Hash of Rust. Go
// hashes with FNV-1a of 128 bits from hash/fnv. The keys differ from the keys
// of upstream, but they play the same role. Each slice and each string
// starts with its length, as the trait Hash of Rust does, so that two
// sequences of values never give the same input.
type keyHasher struct {
	h   hash.Hash
	buf [8]byte
}

// newKeyHasher returns a hasher with no input.
func newKeyHasher() *keyHasher {
	return &keyHasher{h: fnv.New128a()}
}

// bytes adds a slice of bytes and its length.
func (k *keyHasher) bytes(b []byte) *keyHasher {
	k.u64(uint64(len(b)))
	k.h.Write(b)
	return k
}

// str adds a string and its length.
func (k *keyHasher) str(s string) *keyHasher {
	k.u64(uint64(len(s)))
	k.h.Write([]byte(s))
	return k
}

// u64 adds a number of 64 bits.
func (k *keyHasher) u64(v uint64) *keyHasher {
	binary.LittleEndian.PutUint64(k.buf[:], v)
	k.h.Write(k.buf[:])
	return k
}

// boolean adds a bool.
func (k *keyHasher) boolean(v bool) *keyHasher {
	if v {
		return k.u64(1)
	}
	return k.u64(0)
}

// key adds a key.
func (k *keyHasher) key(v Key128) *keyHasher {
	k.h.Write(v[:])
	return k
}

// sum returns the key of the values that the hasher took.
func (k *keyHasher) sum() Key128 {
	var out Key128
	k.h.Sum(out[:0])
	return out
}

// dictCacheKey is the CacheKey implementation of Dict in hayro-font
// src/util.rs. It hashes the raw bytes of the dictionary.
func dictCacheKey(d syntax.Dict) Key128 {
	return newKeyHasher().bytes(d.Data()).sum()
}

// streamCacheKey is the CacheKey implementation of Stream in hayro-font
// src/util.rs. It hashes the identifier of the stream, or its dictionary and
// its raw data if it has no identifier.
func streamCacheKey(s syntax.Stream) Key128 {
	if id, ok := s.Dict().ObjID(); ok {
		return newKeyHasher().str("obj").u64(uint64(id.ObjNumber)).u64(uint64(uint32(id.GenNumber))).sum()
	}
	return newKeyHasher().str("data").bytes(s.Dict().Data()).bytes(s.RawData()).sum()
}
