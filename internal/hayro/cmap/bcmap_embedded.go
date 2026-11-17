// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-cmap src/bcmap/embedded.rs.

package cmap

import (
	_ "embed"
	"sync"
)

// bundleData is the bundle of the predefined CMaps in the binary form. It
// holds the Huffman tables, then each CMap in the order of embeddedNames. The
// license of the data is in data/LICENSE.txt.
//
// Port: upstream embeds assets/cmaps.brotli and decompresses it with the
// crate brotli at the first use. ivy has no decoder of brotli, so the port
// embeds the data that the brotli command decompressed from that file. The
// data takes 467,235 bytes, against 250,740 bytes compressed.
//
//go:embed data/cmaps.bin
var bundleData []byte

// bundle is Bundle in hayro-cmap src/bcmap/embedded.rs. It is the parsed
// bundle of the predefined CMaps.
type bundle struct {
	data       []byte
	deltaTable huffmanTable
	countTable huffmanTable
	entries    [][2]int
}

// loadBundle is BUNDLE in hayro-cmap src/bcmap/embedded.rs. It parses the
// bundle once, at its first use.
//
// Port: upstream knows that the data is valid and panics if it is not. Go
// returns an error.
var loadBundle = sync.OnceValues(func() (*bundle, error) {
	data := bundleData
	r := newBCMapReader(data)
	huffSize, ok := r.readU32()
	if !ok {
		return nil, errFormat("the bundle of CMaps has no Huffman tables")
	}
	huffData, ok := r.readBytes(int(huffSize))
	if !ok {
		return nil, errFormat("the bundle of CMaps has no Huffman tables")
	}
	deltaTable, countTable, ok := decodeTables(huffData)
	if !ok {
		return nil, errFormat("the bundle of CMaps has malformed Huffman tables")
	}

	var entries [][2]int
	for !r.atEnd() {
		start := r.position()
		_, ok1 := r.readBytes(6)
		fileLen, ok2 := r.readU32()
		if !ok1 || !ok2 || fileLen < bcmapFileHeaderSize {
			return nil, errFormat("the bundle of CMaps has a malformed entry")
		}
		if _, ok := r.readBytes(int(fileLen) - bcmapFileHeaderSize); !ok {
			return nil, errFormat("the bundle of CMaps has a malformed entry")
		}
		entries = append(entries, [2]int{start, start + int(fileLen)})
	}

	return &bundle{
		data:       data,
		deltaTable: deltaTable,
		countTable: countTable,
		entries:    entries,
	}, nil
})

// embeddedNames holds the names of the predefined CMaps in the order of the
// bundle, which is the alphabetical order. It is the match of load_embedded
// in hayro-cmap src/bcmap/embedded.rs.
var embeddedNames = [...]Name{
	N83pvRksjH, N90msRksjH, N90msRksjV, N90mspRksjH, N90mspRksjV, N90pvRksjH,
	AddRksjH, AddRksjV,
	AdobeCns1Ucs2, AdobeGb1Ucs2, AdobeJapan1Ucs2, AdobeKorea1Ucs2,
	B5pcH, B5pcV, CnsEucH, CnsEucV, ETenB5H, ETenB5V, ETenmsB5H, ETenmsB5V,
	EucH, EucV, ExtRksjH, ExtRksjV,
	GbEucH, GbEucV, GbkEucH, GbkEucV, Gbk2kH, Gbk2kV, GbkpEucH, GbkpEucV,
	GbpcEucH, GbpcEucV,
	H,
	HKscsB5H, HKscsB5V,
	IdentityH, IdentityV,
	KscEucH, KscEucV, KscmsUhcH, KscmsUhcHwH, KscmsUhcHwV, KscmsUhcV, KscpcEucH,
	UniCnsUcs2H, UniCnsUcs2V, UniCnsUtf16H, UniCnsUtf16V,
	UniGbUcs2H, UniGbUcs2V, UniGbUtf16H, UniGbUtf16V,
	UniJisUcs2H, UniJisUcs2HwH, UniJisUcs2HwV, UniJisUcs2V, UniJisUtf16H, UniJisUtf16V,
	UniKsUcs2H, UniKsUcs2V, UniKsUtf16H, UniKsUtf16V,
	V,
}

// embeddedIndex maps the name of a predefined CMap to its index in the
// bundle.
var embeddedIndex = func() map[Name]int {
	m := make(map[Name]int, len(embeddedNames))
	for i, n := range embeddedNames {
		m[n] = i
	}
	return m
}()

// LoadEmbedded is load_embedded in hayro-cmap src/bcmap/embedded.rs. It
// returns the binary form of a predefined CMap. It returns false for a
// custom name. The caller must not change the bytes. LoadEmbedded is a
// Resolver.
func LoadEmbedded(name Name) ([]byte, bool) {
	idx, ok := embeddedIndex[name]
	if !ok {
		return nil, false
	}
	b, err := loadBundle()
	if err != nil || idx >= len(b.entries) {
		return nil, false
	}
	e := b.entries[idx]
	return b.data[e[0]:e[1]:e[1]], true
}
