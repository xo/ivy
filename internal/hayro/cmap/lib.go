// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-cmap src/lib.rs.

// Package cmap is a port of the Rust crate hayro-cmap, a parser of the CMap
// files that PDF files hold.
//
// A CMap maps the character codes of the operators that show text to CIDs.
// A CID (character identifier) is the number of a glyph in a CID font. A
// CMap can also map CIDs or codes to Unicode text.
//
// Parse reads a CMap in its PostScript form, or in the binary form of the
// predefined CMaps. LoadEmbedded returns the binary form of a predefined
// CMap, such as UniGB-UCS2-H, from the data that the package embeds.
//
// Each error wraps ivy.ErrFormat or ivy.ErrLimit.
package cmap

import (
	"slices"
	"unicode/utf16"
)

// CID is Cid in hayro-cmap src/lib.rs. It is a character identifier.
type CID = uint32

// Name is CMapName in hayro-cmap src/lib.rs. It is the name of a CMap.
// The constants are the names of the predefined CMaps.
//
// Port: upstream is an enum with a variant for each predefined CMap and the
// variant Custom for any other name. Go uses the name itself. A name that is
// not a constant of this package is a custom name. The type drops the
// prefix CMap, because cmap.CMapName stutters in Go.
type Name string

// The names of the predefined CMaps. The comment of each one names its
// character collection, its encoding and its writing mode.
const (
	// N83pvRksjH is Adobe-Japan1, Mac Shift-JIS of KanjiTalk 6, horizontal.
	N83pvRksjH Name = "83pv-RKSJ-H"
	// N90msRksjH is Adobe-Japan1, Windows Shift-JIS (code page 932), horizontal.
	N90msRksjH Name = "90ms-RKSJ-H"
	// N90msRksjV is Adobe-Japan1, Windows Shift-JIS (code page 932), vertical.
	N90msRksjV Name = "90ms-RKSJ-V"
	// N90mspRksjH is Adobe-Japan1, Windows Shift-JIS with proportional Roman, horizontal.
	N90mspRksjH Name = "90msp-RKSJ-H"
	// N90mspRksjV is Adobe-Japan1, Windows Shift-JIS with proportional Roman, vertical.
	N90mspRksjV Name = "90msp-RKSJ-V"
	// N90pvRksjH is Adobe-Japan1, Mac Shift-JIS of KanjiTalk 7, horizontal.
	N90pvRksjH Name = "90pv-RKSJ-H"
	// AddRksjH is Adobe-Japan1, Fujitsu FMR Shift-JIS, horizontal.
	AddRksjH Name = "Add-RKSJ-H"
	// AddRksjV is Adobe-Japan1, Fujitsu FMR Shift-JIS, vertical.
	AddRksjV Name = "Add-RKSJ-V"

	// AdobeCns1Ucs2 is Adobe-CNS1, the map from CIDs to Unicode.
	AdobeCns1Ucs2 Name = "Adobe-CNS1-UCS2"
	// AdobeGb1Ucs2 is Adobe-GB1, the map from CIDs to Unicode.
	AdobeGb1Ucs2 Name = "Adobe-GB1-UCS2"
	// AdobeJapan1Ucs2 is Adobe-Japan1, the map from CIDs to Unicode.
	AdobeJapan1Ucs2 Name = "Adobe-Japan1-UCS2"
	// AdobeKorea1Ucs2 is Adobe-Korea1, the map from CIDs to Unicode.
	AdobeKorea1Ucs2 Name = "Adobe-Korea1-UCS2"

	// B5pcH is Adobe-CNS1, Mac Big Five with the ETen extensions, horizontal.
	B5pcH Name = "B5pc-H"
	// B5pcV is Adobe-CNS1, Mac Big Five with the ETen extensions, vertical.
	B5pcV Name = "B5pc-V"
	// CnsEucH is Adobe-CNS1, the EUC encoding of CNS 11643, horizontal.
	CnsEucH Name = "CNS-EUC-H"
	// CnsEucV is Adobe-CNS1, the EUC encoding of CNS 11643, vertical.
	CnsEucV Name = "CNS-EUC-V"
	// ETenB5H is Adobe-CNS1, the ETen extensions of Big Five, horizontal.
	ETenB5H Name = "ETen-B5-H"
	// ETenB5V is Adobe-CNS1, the ETen extensions of Big Five, vertical.
	ETenB5V Name = "ETen-B5-V"
	// ETenmsB5H is Adobe-CNS1, ETen Big Five with the Microsoft symbols, horizontal.
	ETenmsB5H Name = "ETenms-B5-H"
	// ETenmsB5V is Adobe-CNS1, ETen Big Five with the Microsoft symbols, vertical.
	ETenmsB5V Name = "ETenms-B5-V"

	// EucH is Adobe-Japan1, the EUC-JP encoding of JIS X 0208, horizontal.
	EucH Name = "EUC-H"
	// EucV is Adobe-Japan1, the EUC-JP encoding of JIS X 0208, vertical.
	EucV Name = "EUC-V"
	// ExtRksjH is Adobe-Japan1, Shift-JIS with the NEC and IBM extensions, horizontal.
	ExtRksjH Name = "Ext-RKSJ-H"
	// ExtRksjV is Adobe-Japan1, Shift-JIS with the NEC and IBM extensions, vertical.
	ExtRksjV Name = "Ext-RKSJ-V"

	// GbEucH is Adobe-GB1, the EUC encoding of GB 2312-80, horizontal.
	GbEucH Name = "GB-EUC-H"
	// GbEucV is Adobe-GB1, the EUC encoding of GB 2312-80, vertical.
	GbEucV Name = "GB-EUC-V"
	// GbkEucH is Adobe-GB1, GBK (Microsoft code page 936), horizontal.
	GbkEucH Name = "GBK-EUC-H"
	// GbkEucV is Adobe-GB1, GBK (Microsoft code page 936), vertical.
	GbkEucV Name = "GBK-EUC-V"
	// Gbk2kH is Adobe-GB1, GB 18030-2000, horizontal.
	Gbk2kH Name = "GBK2K-H"
	// Gbk2kV is Adobe-GB1, GB 18030-2000, vertical.
	Gbk2kV Name = "GBK2K-V"
	// GbkpEucH is Adobe-GB1, GBK with proportional Roman, horizontal.
	GbkpEucH Name = "GBKp-EUC-H"
	// GbkpEucV is Adobe-GB1, GBK with proportional Roman, vertical.
	GbkpEucV Name = "GBKp-EUC-V"
	// GbpcEucH is Adobe-GB1, the Mac EUC encoding of GB 2312, horizontal.
	GbpcEucH Name = "GBpc-EUC-H"
	// GbpcEucV is Adobe-GB1, the Mac EUC encoding of GB 2312, vertical.
	GbpcEucV Name = "GBpc-EUC-V"

	// H is Adobe-Japan1, the row and cell encoding of JIS X 0208, horizontal.
	H Name = "H"

	// HKscsB5H is Adobe-CNS1, Big Five with the Hong Kong extensions, horizontal.
	HKscsB5H Name = "HKscs-B5-H"
	// HKscsB5V is Adobe-CNS1, Big Five with the Hong Kong extensions, vertical.
	HKscsB5V Name = "HKscs-B5-V"

	// IdentityH is Adobe-Identity, the identity map of two bytes, horizontal.
	IdentityH Name = "Identity-H"
	// IdentityV is Adobe-Identity, the identity map of two bytes, vertical.
	IdentityV Name = "Identity-V"

	// KscEucH is Adobe-Korea1, the EUC-KR encoding of KS X 1001:1992, horizontal.
	KscEucH Name = "KSC-EUC-H"
	// KscEucV is Adobe-Korea1, the EUC-KR encoding of KS X 1001:1992, vertical.
	KscEucV Name = "KSC-EUC-V"
	// KscmsUhcH is Adobe-Korea1, Microsoft UHC (code page 949), horizontal.
	KscmsUhcH Name = "KSCms-UHC-H"
	// KscmsUhcHwH is Adobe-Korea1, Microsoft UHC with half-width Roman, horizontal.
	KscmsUhcHwH Name = "KSCms-UHC-HW-H"
	// KscmsUhcHwV is Adobe-Korea1, Microsoft UHC with half-width Roman, vertical.
	KscmsUhcHwV Name = "KSCms-UHC-HW-V"
	// KscmsUhcV is Adobe-Korea1, Microsoft UHC (code page 949), vertical.
	KscmsUhcV Name = "KSCms-UHC-V"
	// KscpcEucH is Adobe-Korea1, the Mac EUC-KR encoding of KS X 1001:1992, horizontal.
	KscpcEucH Name = "KSCpc-EUC-H"

	// UniCnsUcs2H is Adobe-CNS1, Unicode UCS-2, horizontal.
	UniCnsUcs2H Name = "UniCNS-UCS2-H"
	// UniCnsUcs2V is Adobe-CNS1, Unicode UCS-2, vertical.
	UniCnsUcs2V Name = "UniCNS-UCS2-V"
	// UniCnsUtf16H is Adobe-CNS1, Unicode UTF-16, horizontal.
	UniCnsUtf16H Name = "UniCNS-UTF16-H"
	// UniCnsUtf16V is Adobe-CNS1, Unicode UTF-16, vertical.
	UniCnsUtf16V Name = "UniCNS-UTF16-V"

	// UniGbUcs2H is Adobe-GB1, Unicode UCS-2, horizontal.
	UniGbUcs2H Name = "UniGB-UCS2-H"
	// UniGbUcs2V is Adobe-GB1, Unicode UCS-2, vertical.
	UniGbUcs2V Name = "UniGB-UCS2-V"
	// UniGbUtf16H is Adobe-GB1, Unicode UTF-16, horizontal.
	UniGbUtf16H Name = "UniGB-UTF16-H"
	// UniGbUtf16V is Adobe-GB1, Unicode UTF-16, vertical.
	UniGbUtf16V Name = "UniGB-UTF16-V"

	// UniJisUcs2H is Adobe-Japan1, Unicode UCS-2, horizontal.
	UniJisUcs2H Name = "UniJIS-UCS2-H"
	// UniJisUcs2HwH is Adobe-Japan1, Unicode UCS-2 with half-width Roman, horizontal.
	UniJisUcs2HwH Name = "UniJIS-UCS2-HW-H"
	// UniJisUcs2HwV is Adobe-Japan1, Unicode UCS-2 with half-width Roman, vertical.
	UniJisUcs2HwV Name = "UniJIS-UCS2-HW-V"
	// UniJisUcs2V is Adobe-Japan1, Unicode UCS-2, vertical.
	UniJisUcs2V Name = "UniJIS-UCS2-V"
	// UniJisUtf16H is Adobe-Japan1, Unicode UTF-16, horizontal.
	UniJisUtf16H Name = "UniJIS-UTF16-H"
	// UniJisUtf16V is Adobe-Japan1, Unicode UTF-16, vertical.
	UniJisUtf16V Name = "UniJIS-UTF16-V"

	// UniKsUcs2H is Adobe-Korea1, Unicode UCS-2, horizontal.
	UniKsUcs2H Name = "UniKS-UCS2-H"
	// UniKsUcs2V is Adobe-Korea1, Unicode UCS-2, vertical.
	UniKsUcs2V Name = "UniKS-UCS2-V"
	// UniKsUtf16H is Adobe-Korea1, Unicode UTF-16, horizontal.
	UniKsUtf16H Name = "UniKS-UTF16-H"
	// UniKsUtf16V is Adobe-Korea1, Unicode UTF-16, vertical.
	UniKsUtf16V Name = "UniKS-UTF16-V"

	// V is Adobe-Japan1, the row and cell encoding of JIS X 0208, vertical.
	V Name = "V"
)

// NameFromBytes is CMapName::from_bytes in hayro-cmap src/lib.rs. It
// returns the name that the bytes hold.
func NameFromBytes(name []byte) Name {
	return Name(name)
}

// ToBytes is CMapName::to_bytes in hayro-cmap src/lib.rs. It returns the raw
// bytes of the name.
func (n Name) ToBytes() []byte {
	return []byte(n)
}

// Resolver is the type of the argument get_cmap of CMap::parse in hayro-cmap
// src/lib.rs. It returns the data of the CMap that a usecmap operator names.
// It returns false if it does not know the name. LoadEmbedded is a Resolver.
type Resolver func(name Name) ([]byte, bool)

// maxNestingDepth is MAX_NESTING_DEPTH in hayro-cmap src/lib.rs. It is the
// largest depth of usecmap.
const maxNestingDepth = 16

// maxNestedParses limits the number of CMaps that one call of Parse reads
// through usecmap, at all depths together.
//
// Port: upstream has no such limit. A CMap can hold a usecmap operator many
// times, and upstream reads the named CMap again for each one. Without the
// limit, a small file can make the parser read a large predefined CMap
// thousands of times.
const maxNestedParses = 64

// CMap is CMap in hayro-cmap src/lib.rs. It is a parsed CMap.
type CMap struct {
	metadata        Metadata
	codespaceRanges []CodespaceRange
	cidRanges       partitionedRanges
	notdefRanges    partitionedRanges
	bfEntries       []bfRange
	base            *CMap
}

// Parse is CMap::parse in hayro-cmap src/lib.rs. It parses a CMap. It reads
// the CMaps that usecmap names through getCMap. getCMap can be nil. Then a
// usecmap operator is an error.
//
// Port: upstream returns None for each error. Go returns an error that wraps
// ivy.ErrFormat, or ivy.ErrLimit for a limit.
func Parse(data []byte, getCMap Resolver) (*CMap, error) {
	budget := maxNestedParses
	return parseInner(data, getCMap, 0, &budget)
}

// NewIdentityH is CMap::identity_h in hayro-cmap src/lib.rs. It returns the
// Identity-H CMap.
//
// Port: the name IdentityH is the constant of the Name.
func NewIdentityH() *CMap {
	return newIdentity(Horizontal, IdentityH)
}

// NewIdentityV is CMap::identity_v in hayro-cmap src/lib.rs. It returns the
// Identity-V CMap.
//
// Port: the name IdentityV is the constant of the Name.
func NewIdentityV() *CMap {
	return newIdentity(Vertical, IdentityV)
}

// newIdentity is CMap::identity in hayro-cmap src/lib.rs.
func newIdentity(writingMode WritingMode, name Name) *CMap {
	c := &CMap{
		metadata: Metadata{
			CharacterCollection: &CharacterCollection{
				Family:     CIDFamily{Kind: AdobeIdentity},
				Supplement: 0,
			},
			Name:        name.ToBytes(),
			WritingMode: &writingMode,
		},
		codespaceRanges: []CodespaceRange{{numberBytes: 2, low: 0x0000, high: 0xffff}},
	}
	c.cidRanges.push(2, CIDRange{rng: codeRange{start: 0, end: 0xffff}, cidStart: 0})
	return c
}

// Metadata is CMap::metadata in hayro-cmap src/lib.rs. It returns the
// metadata of c.
func (c *CMap) Metadata() Metadata {
	return c.metadata
}

// Base is CMap::base in hayro-cmap src/lib.rs. It returns the CMap that
// usecmap names, or nil.
func (c *CMap) Base() *CMap {
	return c.base
}

// CodespaceRanges is CMap::codespace_ranges in hayro-cmap src/lib.rs. It
// returns the codespace ranges that c declares. The caller must not change
// them.
func (c *CMap) CodespaceRanges() []CodespaceRange {
	return c.codespaceRanges
}

// CIDRanges is CMap::cid_ranges in hayro-cmap src/lib.rs. It returns the CID
// ranges that c declares for codes of byteLen bytes. The caller must not
// change them.
func (c *CMap) CIDRanges(byteLen uint8) []CIDRange {
	r, _ := c.cidRanges.get(byteLen)
	return r
}

// NotdefRanges is CMap::notdef_ranges in hayro-cmap src/lib.rs. It returns
// the .notdef ranges that c declares for codes of byteLen bytes. The caller
// must not change them.
func (c *CMap) NotdefRanges(byteLen uint8) []CIDRange {
	r, _ := c.notdefRanges.get(byteLen)
	return r
}

// LookupCIDCode is CMap::lookup_cid_code in hayro-cmap src/lib.rs. It
// returns the CID of a character code of byteLen bytes.
func (c *CMap) LookupCIDCode(code uint32, byteLen uint8) (CID, bool) {
	// Upstream does not test the codespace ranges first, as Acrobat does not.
	cidRanges, ok := c.cidRanges.get(byteLen)
	if !ok {
		return 0, false
	}
	notdefRanges, ok := c.notdefRanges.get(byteLen)
	if !ok {
		return 0, false
	}

	if i, ok := findInRanges(cidRanges, code); ok {
		entry := &cidRanges[i]
		if code < entry.rng.start {
			return 0, false
		}
		offset := code - entry.rng.start
		if entry.cidStart > ^uint32(0)-offset {
			return 0, false
		}
		return entry.cidStart + offset, true
	} else if i, ok := findInRanges(notdefRanges, code); ok {
		return notdefRanges[i].cidStart, true
	}

	// Some files map the codes of text with bf chars (pdfjs_bug920426).
	// Upstream looks at this level only.
	if bf, ok := c.lookupBfStringInner(code, false); ok && !bf.IsString {
		return CID(bf.Char), true
	}

	if c.base == nil {
		return 0, false
	}
	return c.base.LookupCIDCode(code, byteLen)
}

// LookupBfString is CMap::lookup_bf_string in hayro-cmap src/lib.rs. It
// returns the Unicode text of a character code.
func (c *CMap) LookupBfString(code uint32) (BfString, bool) {
	return c.lookupBfStringInner(code, true)
}

// lookupBfStringInner is CMap::lookup_bf_string_inner in hayro-cmap
// src/lib.rs. If recurse is true, it also looks in the base CMap.
func (c *CMap) lookupBfStringInner(code uint32, recurse bool) (BfString, bool) {
	if i, ok := findInRanges(c.bfEntries, code); ok {
		entry := &c.bfEntries[i]
		diff := code - entry.rng.start
		if diff > 0xffff {
			return BfString{}, false
		}
		offset := uint16(diff)

		if offset == 0 {
			return decodeUTF16(entry.dstBase)
		}
		if len(entry.dstBase) == 0 {
			return BfString{}, false
		}
		units := slices.Clone(entry.dstBase)
		last := units[len(units)-1]
		if last > 0xffff-offset {
			return BfString{}, false
		}
		units[len(units)-1] = last + offset
		return decodeUTF16(units)
	}

	if recurse && c.base != nil {
		return c.base.LookupBfString(code)
	}
	return BfString{}, false
}

// decodeUTF16 is decode_utf16 in CMap::lookup_bf_string_inner in hayro-cmap
// src/lib.rs. It returns BfString.Char for one character, and
// BfString.String for more. It returns false for an unpaired surrogate.
func decodeUTF16(units []uint16) (BfString, bool) {
	first, n, ok := nextUTF16(units)
	if !ok {
		return BfString{}, false
	}
	if n == len(units) {
		return NewBfChar(first), true
	}

	runes := make([]rune, 0, len(units))
	for i := 0; i < len(units); {
		r, n, ok := nextUTF16(units[i:])
		if !ok {
			return BfString{}, false
		}
		runes = append(runes, r)
		i += n
	}
	return NewBfString(string(runes)), true
}

// nextUTF16 is one step of char::decode_utf16 of the Rust core library. It
// returns the first character of units and the number of units that it
// takes. It returns false for an unpaired surrogate or no units.
//
// Port: utf16.Decode of Go replaces an unpaired surrogate with U+FFFD.
// Upstream fails instead.
func nextUTF16(units []uint16) (rune, int, bool) {
	if len(units) == 0 {
		return 0, 0, false
	}
	u := rune(units[0])
	if !utf16.IsSurrogate(u) {
		return u, 1, true
	}
	if len(units) < 2 {
		return 0, 0, false
	}
	r := utf16.DecodeRune(u, rune(units[1]))
	if r == '�' {
		return 0, 0, false
	}
	return r, 2, true
}

// rangeHolder is HasRange in hayro-cmap src/lib.rs.
type rangeHolder interface {
	rangeOf() codeRange
}

// findInRanges is find_in_ranges in hayro-cmap src/lib.rs. It returns the
// index of an entry whose range holds code. The entries are in the order of
// their starts.
//
// Port: the search is slice::binary_search_by of the Rust core library, step
// by step. If two ranges overlap and both hold code, Go finds the same entry
// as upstream.
func findInRanges[T rangeHolder](entries []T, code uint32) (int, bool) {
	// cmpEntry is the closure of upstream. It returns -1 for Less, 0 for
	// Equal and 1 for Greater.
	cmpEntry := func(e T) int {
		r := e.rangeOf()
		switch {
		case code < r.start:
			return 1
		case code > r.end:
			return -1
		}
		return 0
	}

	size := len(entries)
	if size == 0 {
		return 0, false
	}
	base := 0
	for size > 1 {
		half := size / 2
		mid := base + half
		if cmpEntry(entries[mid]) != 1 {
			base = mid
		}
		size -= half
	}
	if cmpEntry(entries[base]) == 0 {
		return base, true
	}
	return 0, false
}

// codeRange is Range in hayro-cmap src/lib.rs. It holds the codes from start
// to end, with end included.
type codeRange struct {
	start uint32
	end   uint32
}

// CIDRange is CidRange in hayro-cmap src/lib.rs. It is a range of character
// codes that maps to CIDs.
type CIDRange struct {
	rng      codeRange
	cidStart CID
}

// Start is CidRange::start in hayro-cmap src/lib.rs. It returns the first
// character code of the range.
func (r CIDRange) Start() uint32 {
	return r.rng.start
}

// End is CidRange::end in hayro-cmap src/lib.rs. It returns the last
// character code of the range. The range includes it.
func (r CIDRange) End() uint32 {
	return r.rng.end
}

// CIDStart is CidRange::cid_start in hayro-cmap src/lib.rs. It returns the
// CID of the first character code.
func (r CIDRange) CIDStart() CID {
	return r.cidStart
}

// rangeOf is HasRange for CidRange in hayro-cmap src/lib.rs.
func (r CIDRange) rangeOf() codeRange {
	return r.rng
}

// partitionedRanges is PartitionedRanges in hayro-cmap src/lib.rs. It holds
// the CID ranges in four buckets, one for each length of a code from one to
// four bytes.
type partitionedRanges struct {
	buckets [4][]CIDRange
}

// push is PartitionedRanges::push in hayro-cmap src/lib.rs. It drops a range
// whose length of code is not from one to four bytes.
func (p *partitionedRanges) push(byteLen int, r CIDRange) {
	if byteLen < 1 || byteLen > len(p.buckets) {
		return
	}
	p.buckets[byteLen-1] = append(p.buckets[byteLen-1], r)
}

// get is PartitionedRanges::get in hayro-cmap src/lib.rs.
func (p *partitionedRanges) get(byteLen uint8) ([]CIDRange, bool) {
	if byteLen < 1 || int(byteLen) > len(p.buckets) {
		return nil, false
	}
	return p.buckets[byteLen-1], true
}

// sort is PartitionedRanges::sort in hayro-cmap src/lib.rs. It sorts each
// bucket by the start of its ranges, and it keeps the order of equal starts.
func (p *partitionedRanges) sort() {
	for i := range p.buckets {
		slices.SortStableFunc(p.buckets[i], func(a, b CIDRange) int {
			return cmpUint32(a.rng.start, b.rng.start)
		})
	}
}

// cmpUint32 compares a and b.
func cmpUint32(a, b uint32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// bfRange is BfRange in hayro-cmap src/lib.rs. It maps a range of codes to
// Unicode text. The last unit of dstBase grows with the code.
type bfRange struct {
	rng     codeRange
	dstBase []uint16
}

// rangeOf is HasRange for BfRange in hayro-cmap src/lib.rs.
func (r bfRange) rangeOf() codeRange {
	return r.rng
}

// sortBfEntries is the sort of the bf entries in hayro-cmap src/parse.rs and
// src/bcmap/mod.rs. It keeps the order of equal starts.
func sortBfEntries(entries []bfRange) {
	slices.SortStableFunc(entries, func(a, b bfRange) int {
		return cmpUint32(a.rng.start, b.rng.start)
	})
}

// CodespaceRange is CodespaceRange in hayro-cmap src/lib.rs. It is a range of
// the valid byte sequences of the character codes.
type CodespaceRange struct {
	numberBytes uint8
	low         uint32
	high        uint32
}

// ByteLen is CodespaceRange::byte_len in hayro-cmap src/lib.rs. It returns
// the number of bytes of the codes in the range.
func (r CodespaceRange) ByteLen() uint8 {
	return r.numberBytes
}

// Low is CodespaceRange::low in hayro-cmap src/lib.rs. It returns the lower
// bound of the range.
func (r CodespaceRange) Low() uint32 {
	return r.low
}

// High is CodespaceRange::high in hayro-cmap src/lib.rs. It returns the upper
// bound of the range.
func (r CodespaceRange) High() uint32 {
	return r.high
}

// BfString is BfString in hayro-cmap src/lib.rs. It is the Unicode value of a
// code in a CMap.
//
// Port: upstream is an enum with the variants Char and String. Go holds both
// values in one struct. IsString tells which value is set.
type BfString struct {
	// Char is the value of BfString::Char, one character.
	Char rune
	// String is the value of BfString::String, a UTF-8 string.
	String string
	// IsString is true for BfString::String and false for BfString::Char.
	IsString bool
}

// NewBfChar is BfString::Char in hayro-cmap src/lib.rs.
func NewBfChar(r rune) BfString {
	return BfString{Char: r}
}

// NewBfString is BfString::String in hayro-cmap src/lib.rs.
func NewBfString(s string) BfString {
	return BfString{String: s, IsString: true}
}

// Metadata is Metadata in hayro-cmap src/lib.rs. It is the metadata of a CMap
// file.
//
// Port: upstream uses Option for each field. Go uses nil for None. A name
// that is present and empty is a non-nil empty slice.
type Metadata struct {
	// CharacterCollection is the character collection that the CMap names.
	CharacterCollection *CharacterCollection
	// Name is the name of the CMap.
	Name []byte
	// WritingMode is the writing mode.
	WritingMode *WritingMode
}

// CIDFamilyKind tells which variant of CidFamily in hayro-cmap src/lib.rs a
// CIDFamily is.
type CIDFamilyKind uint8

// The variants of CidFamily in hayro-cmap src/lib.rs.
const (
	// AdobeJapan1 is CidFamily::AdobeJapan1, Adobe-Japan1.
	AdobeJapan1 CIDFamilyKind = iota
	// AdobeGB1 is CidFamily::AdobeGB1, Adobe-GB1.
	AdobeGB1
	// AdobeCNS1 is CidFamily::AdobeCNS1, Adobe-CNS1.
	AdobeCNS1
	// AdobeKorea1 is CidFamily::AdobeKorea1, Adobe-Korea1.
	AdobeKorea1
	// AdobeIdentity is CidFamily::AdobeIdentity, Adobe-Identity.
	AdobeIdentity
	// Custom is CidFamily::Custom, a pair of a registry and an ordering that
	// is not predefined.
	Custom
)

// CIDFamily is CidFamily in hayro-cmap src/lib.rs. It is the family of a CID
// character collection: its registry and its ordering.
//
// Port: upstream is an enum, and only the variant Custom holds data. Go
// holds Registry and Ordering only for the kind Custom, so that == compares
// two families as upstream does.
type CIDFamily struct {
	// Kind is the variant.
	Kind CIDFamilyKind
	// Registry is the name of the registry of the kind Custom.
	Registry string
	// Ordering is the name of the ordering of the kind Custom.
	Ordering string
}

// CIDFamilyFromRegistryOrdering is CidFamily::from_registry_ordering in
// hayro-cmap src/lib.rs. It returns the family of a registry and an
// ordering.
func CIDFamilyFromRegistryOrdering(registry, ordering []byte) CIDFamily {
	if string(registry) == "Adobe" {
		switch string(ordering) {
		case "Japan1":
			return CIDFamily{Kind: AdobeJapan1}
		case "GB1":
			return CIDFamily{Kind: AdobeGB1}
		case "CNS1":
			return CIDFamily{Kind: AdobeCNS1}
		case "Korea1":
			return CIDFamily{Kind: AdobeKorea1}
		case "Identity":
			return CIDFamily{Kind: AdobeIdentity}
		}
	}
	return CIDFamily{Kind: Custom, Registry: string(registry), Ordering: string(ordering)}
}

// UnicodeCMap is CidFamily::unicode_cmap in hayro-cmap src/lib.rs. It
// returns the predefined UTF-16 CMap that maps Unicode to the CIDs of the
// family.
func (f CIDFamily) UnicodeCMap() (Name, bool) {
	switch f.Kind {
	case AdobeJapan1:
		return UniJisUtf16H, true
	case AdobeGB1:
		return UniGbUtf16H, true
	case AdobeCNS1:
		return UniCnsUtf16H, true
	case AdobeKorea1:
		return UniKsUtf16H, true
	}
	return "", false
}

// UCS2CMap is CidFamily::ucs2_cmap in hayro-cmap src/lib.rs. It returns the
// UCS2 CMap of the family.
func (f CIDFamily) UCS2CMap() (Name, bool) {
	switch f.Kind {
	case AdobeJapan1:
		return AdobeJapan1Ucs2, true
	case AdobeGB1:
		return AdobeGb1Ucs2, true
	case AdobeCNS1:
		return AdobeCns1Ucs2, true
	case AdobeKorea1:
		return AdobeKorea1Ucs2, true
	}
	return "", false
}

// CharacterCollection is CharacterCollection in hayro-cmap src/lib.rs. It
// names the character set and the order of the CIDs.
type CharacterCollection struct {
	// Family is the registry and the ordering.
	Family CIDFamily
	// Supplement is the number of the supplement.
	Supplement int32
}

// WritingMode is WritingMode in hayro-cmap src/lib.rs. It is the writing
// mode of a CMap. The zero value is Horizontal, the default of upstream.
type WritingMode uint8

// The writing modes.
const (
	// Horizontal is WritingMode::Horizontal.
	Horizontal WritingMode = iota
	// Vertical is WritingMode::Vertical.
	Vertical
)
