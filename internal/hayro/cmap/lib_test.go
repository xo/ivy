// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-cmap src/lib.rs, the modules tests and
// bcmap_tests.

package cmap

import (
	"testing"
)

// preamble is PREAMBLE in the tests of hayro-cmap src/lib.rs.
const preamble = `/CIDSystemInfo 3 dict dup begin
  /Registry (Adobe) def
  /Ordering (Japan1) def
  /Supplement 0 def
end def
/CMapName /Test def
/WMode 0 def
2 begincodespacerange
<00> <FF>
<0000> <FFFF>
endcodespacerange
`

// noCMap is the resolver |_| None of the tests of upstream.
func noCMap(Name) ([]byte, bool) {
	return nil, false
}

// mustParse is CMap::parse(data, get_cmap).unwrap().
func mustParse(t *testing.T, data string, getCMap Resolver) *CMap {
	t.Helper()
	c, err := Parse([]byte(data), getCMap)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return c
}

// parseWithPreamble is parse_with_preamble in the tests of hayro-cmap
// src/lib.rs.
func parseWithPreamble(t *testing.T, body string) *CMap {
	t.Helper()
	return mustParse(t, preamble+body, noCMap)
}

// wantCID compares the CID of a code with want. A negative want means None.
func wantCID(t *testing.T, c *CMap, code uint32, byteLen uint8, want int64) {
	t.Helper()
	got, ok := c.LookupCIDCode(code, byteLen)
	switch {
	case want < 0 && ok:
		t.Errorf("LookupCIDCode(%#x, %d) = %d, want none", code, byteLen, got)
	case want >= 0 && (!ok || int64(got) != want):
		t.Errorf("LookupCIDCode(%#x, %d) = %d, %v, want %d", code, byteLen, got, ok, want)
	}
}

// wantBf compares the bf string of a code with want.
func wantBf(t *testing.T, c *CMap, code uint32, want BfString) {
	t.Helper()
	got, ok := c.LookupBfString(code)
	if !ok || got != want {
		t.Errorf("LookupBfString(%#x) = %+v, %v, want %+v", code, got, ok, want)
	}
}

// wantNoBf fails the test if a code has a bf string.
func wantNoBf(t *testing.T, c *CMap, code uint32) {
	t.Helper()
	if got, ok := c.LookupBfString(code); ok {
		t.Errorf("LookupBfString(%#x) = %+v, want none", code, got)
	}
}

// wantName compares the name of the metadata with want.
func wantName(t *testing.T, c *CMap, want string) {
	t.Helper()
	got := c.Metadata().Name
	if got == nil || string(got) != want {
		t.Errorf("name = %q, want %q", got, want)
	}
}

// wantMode compares the writing mode of the metadata with want. A nil want
// means None.
func wantMode(t *testing.T, c *CMap, want *WritingMode) {
	t.Helper()
	got := c.Metadata().WritingMode
	switch {
	case want == nil && got != nil:
		t.Errorf("writing mode = %d, want none", *got)
	case want != nil && (got == nil || *got != *want):
		t.Errorf("writing mode = %v, want %d", got, *want)
	}
}

// wantCollection compares the character collection with want. A nil want
// means None.
func wantCollection(t *testing.T, c *CMap, want *CharacterCollection) {
	t.Helper()
	got := c.Metadata().CharacterCollection
	switch {
	case want == nil && got != nil:
		t.Errorf("character collection = %+v, want none", *got)
	case want != nil && (got == nil || *got != *want):
		t.Errorf("character collection = %+v, want %+v", got, *want)
	}
}

// mode returns a pointer to m.
func mode(m WritingMode) *WritingMode {
	return &m
}

// collection returns a pointer to a CharacterCollection.
func collection(f CIDFamily, supplement int32) *CharacterCollection {
	return &CharacterCollection{Family: f, Supplement: supplement}
}

func TestMetadataParsing(t *testing.T) {
	c := mustParse(t, `
/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo 3 dict dup begin
  /Registry (Adobe) def
  /Ordering (Japan1) def
  /Supplement 6 def
end def
/CMapName /Adobe-Japan1-H def
/CMapType 1 def
/WMode 0 def
endcmap`, noCMap)
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeJapan1}, 6))
	wantName(t, c, "Adobe-Japan1-H")
	wantMode(t, c, mode(Horizontal))
}

func TestVerticalWritingMode(t *testing.T) {
	c := mustParse(t, `
/CIDSystemInfo 3 dict dup begin
  /Registry (Adobe) def
  /Ordering (Japan1) def
  /Supplement 6 def
end def
/CMapName /Adobe-Japan1-V def
/WMode 1 def
`, noCMap)
	wantMode(t, c, mode(Vertical))
	wantName(t, c, "Adobe-Japan1-V")
}

func TestCIDRangeLookup(t *testing.T) {
	c := parseWithPreamble(t, `
3 begincidrange
<0000> <00FF> 0
<0100> <01FF> 256
<8140> <817E> 633
endcidrange
`)
	wantCID(t, c, 0x0000, 2, 0)
	wantCID(t, c, 0x0042, 2, 0x42)
	wantCID(t, c, 0x00ff, 2, 0xff)
	wantCID(t, c, 0x0100, 2, 256)
	wantCID(t, c, 0x01ff, 2, 511)
	wantCID(t, c, 0x8140, 2, 633)
	wantCID(t, c, 0x817e, 2, 633+62)
}

func TestCIDCharLookup(t *testing.T) {
	c := parseWithPreamble(t, `
3 begincidchar
<03> 1
<04> 2
<20> 50
endcidchar
`)
	wantCID(t, c, 0x03, 1, 1)
	wantCID(t, c, 0x04, 1, 2)
	wantCID(t, c, 0x20, 1, 50)
}

func TestEmptyBfCharDestinationKeepsTheOtherEntries(t *testing.T) {
	c := parseWithPreamble(t, `
2 beginbfchar
<00> <0041>
<01> <>
endbfchar
`)
	wantBf(t, c, 0x00, NewBfChar('A'))
	wantNoBf(t, c, 0x01)
}

func TestEmptyBfRangeDestinationKeepsTheOtherEntries(t *testing.T) {
	c := parseWithPreamble(t, `
3 beginbfrange
<02> <03> <>
<04> <05> <0042>
<06> <07> [<> <0044>]
endbfrange
`)
	wantNoBf(t, c, 0x02)
	wantNoBf(t, c, 0x03)
	wantBf(t, c, 0x04, NewBfChar('B'))
	wantBf(t, c, 0x05, NewBfChar('C'))
	wantNoBf(t, c, 0x06)
	wantBf(t, c, 0x07, NewBfChar('D'))
}

func TestLookupMiss(t *testing.T) {
	c := parseWithPreamble(t, `
1 begincidrange
<0100> <01FF> 0
endcidrange
`)
	wantCID(t, c, 0x00ff, 2, -1)
	wantCID(t, c, 0x0200, 2, -1)
	wantCID(t, c, 0xffff, 2, -1)
}

func TestMultipleSections(t *testing.T) {
	c := parseWithPreamble(t, `
2 begincidrange
<0000> <00FF> 0
<0100> <01FF> 256
endcidrange
1 begincidchar
<0200> 600
endcidchar
1 begincidrange
<8140> <817E> 633
endcidrange
`)
	wantCID(t, c, 0x0000, 2, 0)
	wantCID(t, c, 0x0100, 2, 256)
	wantCID(t, c, 0x0200, 2, 600)
	wantCID(t, c, 0x8140, 2, 633)
}

func TestSingleByteCodes(t *testing.T) {
	c := parseWithPreamble(t, `
2 begincidrange
<00> <7F> 0
<80> <FF> 200
endcidrange
`)
	wantCID(t, c, 0x00, 1, 0)
	wantCID(t, c, 0x41, 1, 0x41)
	wantCID(t, c, 0x80, 1, 200)
	wantCID(t, c, 0xff, 1, 200+127)
}

func TestDictStyleCIDSystemInfo(t *testing.T) {
	c := mustParse(t, `
/CIDInit /ProcSet findresource begin
10 dict begin
begincmap
/CIDSystemInfo
<< /Registry (Adobe)
/Ordering (UCS)
/Supplement 0
>> def
/CMapName /Adobe-Identity-UCS def
/CMapType 2 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
2 beginbfchar
<001F> <F049>
<002A> <F055>
endbfchar
endcmap
Name currentdict /CMap defineresource pop
end
end
`, noCMap)
	wantCollection(t, c, collection(CIDFamily{Kind: Custom, Registry: "Adobe", Ordering: "UCS"}, 0))
	wantName(t, c, "Adobe-Identity-UCS")
	wantBf(t, c, 0x001f, NewBfChar(''))
	wantBf(t, c, 0x002a, NewBfChar(''))
}

// baseResolver returns a resolver that knows only the CMap Base.
func baseResolver(base string) Resolver {
	return func(name Name) ([]byte, bool) {
		if string(name.ToBytes()) == "Base" {
			return []byte(base), true
		}
		return nil, false
	}
}

func TestUseCMapChaining(t *testing.T) {
	base := `
/CIDSystemInfo 3 dict dup begin
  /Registry (Adobe) def
  /Ordering (Japan1) def
  /Supplement 0 def
end def
/CMapName /Base def
/WMode 0 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 begincidrange
<0000> <00FF> 0
endcidrange
`
	child := `
/Base usecmap
/CIDSystemInfo 3 dict dup begin
  /Registry (Adobe) def
  /Ordering (Japan1) def
  /Supplement 0 def
end def
/CMapName /Child def
/WMode 0 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 begincidrange
<0100> <01FF> 256
endcidrange
`
	c := mustParse(t, child, baseResolver(base))
	wantCID(t, c, 0x0100, 2, 256)
	wantCID(t, c, 0x01ff, 2, 511)
	wantCID(t, c, 0x0000, 2, 0)
	wantCID(t, c, 0x00ff, 2, 0xff)
	wantCID(t, c, 0x0200, 2, -1)
}

func TestUseCMapPartialOverride(t *testing.T) {
	base := `
/CIDSystemInfo 3 dict dup begin
  /Registry (Adobe) def
  /Ordering (Japan1) def
  /Supplement 0 def
end def
/CMapName /Base def
/WMode 0 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 begincidrange
<0000> <00FF> 0
endcidrange
`
	child := `
/Base usecmap
/CIDSystemInfo 3 dict dup begin
  /Registry (Adobe) def
  /Ordering (Japan1) def
  /Supplement 0 def
end def
/CMapName /Child def
/WMode 0 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 begincidrange
<0040> <007F> 500
endcidrange
`
	c := mustParse(t, child, baseResolver(base))
	wantCID(t, c, 0x0000, 2, 0)
	wantCID(t, c, 0x003f, 2, 0x3f)
	wantCID(t, c, 0x0040, 2, 500)
	wantCID(t, c, 0x007f, 2, 563)
	wantCID(t, c, 0x0080, 2, 0x80)
	wantCID(t, c, 0x00ff, 2, 0xff)
}

func TestNotdefCharLookup(t *testing.T) {
	c := parseWithPreamble(t, `
2 beginnotdefchar
<03> 10
<20> 20
endnotdefchar
`)
	wantCID(t, c, 0x03, 1, 10)
	wantCID(t, c, 0x20, 1, 20)
	wantCID(t, c, 0x04, 1, -1)
}

func TestNotdefRangeLookup(t *testing.T) {
	c := parseWithPreamble(t, `
1 beginnotdefrange
<0000> <001F> 100
endnotdefrange
`)
	wantCID(t, c, 0x0000, 2, 100)
	wantCID(t, c, 0x0001, 2, 100)
	wantCID(t, c, 0x001f, 2, 100)
	wantCID(t, c, 0x0020, 2, -1)
}

func TestBfCharLookup(t *testing.T) {
	c := parseWithPreamble(t, `
2 beginbfchar
<0041> <0048>
<0042> <0065>
endbfchar
`)
	wantBf(t, c, 0x0041, NewBfChar('H'))
	wantBf(t, c, 0x0042, NewBfChar('e'))
	wantNoBf(t, c, 0x0043)
}

func TestBfCharLigature(t *testing.T) {
	c := parseWithPreamble(t, `
1 beginbfchar
<005F> <00660066>
endbfchar
`)
	wantBf(t, c, 0x005f, NewBfString("ff"))
}

func TestBfCharSurrogatePair(t *testing.T) {
	c := parseWithPreamble(t, `
1 beginbfchar
<3A51> <D840DC3E>
endbfchar
`)
	wantBf(t, c, 0x3a51, NewBfChar('\U0002003E'))
}

func TestBfRangeIncrementing(t *testing.T) {
	c := parseWithPreamble(t, `
1 beginbfrange
<0000> <0004> <0041>
endbfrange
`)
	wantBf(t, c, 0x0000, NewBfChar('A'))
	wantBf(t, c, 0x0001, NewBfChar('B'))
	wantBf(t, c, 0x0004, NewBfChar('E'))
	wantNoBf(t, c, 0x0005)
}

func TestBfRangeArray(t *testing.T) {
	c := parseWithPreamble(t, `
1 beginbfrange
<005F> <0061> [<00660066> <00660069> <0066006C>]
endbfrange
`)
	wantBf(t, c, 0x005f, NewBfString("ff"))
	wantBf(t, c, 0x0060, NewBfString("fi"))
	wantBf(t, c, 0x0061, NewBfString("fl"))
}

func TestUnicodeLookupMiss(t *testing.T) {
	c := parseWithPreamble(t, `
1 beginbfchar
<0041> <0048>
endbfchar
`)
	wantNoBf(t, c, 0x0000)
	wantNoBf(t, c, 0x0042)
}

func TestIdentityH(t *testing.T) {
	c := NewIdentityH()
	wantName(t, c, "Identity-H")
	wantMode(t, c, mode(Horizontal))
	wantCID(t, c, 0x0041, 2, 0x0041)
	wantCID(t, c, 0x1234, 2, 0x1234)
	wantCID(t, c, 0xffff, 2, 0xffff)
	wantCID(t, c, 0x0041, 1, -1)
	wantCID(t, c, 0x0041, 3, -1)
}

func TestIdentityV(t *testing.T) {
	c := NewIdentityV()
	wantName(t, c, "Identity-V")
	wantMode(t, c, mode(Vertical))
	wantCID(t, c, 0x0041, 2, 0x0041)
	wantCID(t, c, 0xffff, 2, 0xffff)
}

func TestMinimalCMapNoNameNoWMode(t *testing.T) {
	// Corpus PDF 0500013.
	c := mustParse(t, `
/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapType 2 def
1 begincodespacerange
<00><ff>
endcodespacerange
1 beginbfrange
<01><01><0020>
endbfrange
endcmap
`, noCMap)
	if c.Metadata().Name != nil {
		t.Errorf("name = %q, want none", c.Metadata().Name)
	}
	wantCollection(t, c, nil)
	wantMode(t, c, nil)
	wantBf(t, c, 0x01, NewBfChar(' '))
}

func TestRegistryAsName(t *testing.T) {
	// Corpus PDF 0875241.
	c := mustParse(t, `
/CIDSystemInfo
<< /Registry /ABCDEF+SimSun
/Ordering (pdfbeaninc)
/Supplement 0
>> def
/CMapName /ABCDEF+SimSun def
/CMapType 2 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<0000> <0000> <6881>
endbfrange
`, noCMap)
	wantCollection(t, c, collection(CIDFamily{Kind: Custom, Registry: "ABCDEF+SimSun", Ordering: "pdfbeaninc"}, 0))
	wantBf(t, c, 0x0000, NewBfChar('梁'))
}

func TestSingleByteBfRangeDestination(t *testing.T) {
	// Corpus PDF 0625248.
	c := parseWithPreamble(t, `
1 beginbfrange
<00> <7f> <00>
endbfrange
`)
	wantBf(t, c, 0x00, NewBfChar('\u0000'))
	wantBf(t, c, 0x41, NewBfChar('A'))
	wantBf(t, c, 0x7f, NewBfChar('\u007F'))
}

// rangeTriple returns the start, the end and the CID of a range.
func rangeTriple(r CIDRange) [3]uint32 {
	return [3]uint32{r.Start(), r.End(), r.CIDStart()}
}

func TestRawRanges(t *testing.T) {
	base := `
1 begincodespacerange
<00> <80>
endcodespacerange
1 begincidrange
<20> <7E> 1
endcidrange
`
	child := `
/Base usecmap
1 begincodespacerange
<8140> <9FFC>
endcodespacerange
1 begincidrange
<8140> <817E> 633
endcidrange
1 beginnotdefrange
<8180> <81FC> 7
endnotdefrange
`
	c := mustParse(t, child, baseResolver(base))

	codespace := c.CodespaceRanges()
	if len(codespace) != 1 || codespace[0].ByteLen() != 2 || codespace[0].Low() != 0x8140 || codespace[0].High() != 0x9ffc {
		t.Fatalf("codespace ranges = %+v", codespace)
	}

	cid := c.CIDRanges(2)
	if len(cid) != 1 || rangeTriple(cid[0]) != [3]uint32{0x8140, 0x817e, 633} {
		t.Fatalf("CID ranges = %+v", cid)
	}
	if len(c.CIDRanges(1)) != 0 || len(c.CIDRanges(5)) != 0 {
		t.Fatal("CID ranges of 1 or 5 bytes are not empty")
	}

	notdef := c.NotdefRanges(2)
	if len(notdef) != 1 || rangeTriple(notdef[0]) != [3]uint32{0x8180, 0x81fc, 7} {
		t.Fatalf("notdef ranges = %+v", notdef)
	}

	b := c.Base()
	if b == nil || b.Base() != nil {
		t.Fatal("the chain of base CMaps is wrong")
	}
	if b.CodespaceRanges()[0].ByteLen() != 1 {
		t.Fatalf("base codespace = %+v", b.CodespaceRanges())
	}
	if bc := b.CIDRanges(1); len(bc) == 0 || rangeTriple(bc[0]) != [3]uint32{0x20, 0x7e, 1} {
		t.Fatalf("base CID ranges = %+v", bc)
	}

	for _, identity := range []*CMap{NewIdentityH(), NewIdentityV()} {
		codespace := identity.CodespaceRanges()
		if len(codespace) != 1 || codespace[0].ByteLen() != 2 || codespace[0].Low() != 0 || codespace[0].High() != 0xffff {
			t.Fatalf("identity codespace ranges = %+v", codespace)
		}
	}
}

// mustParseEmbedded is the start of each test of bcmap_tests in hayro-cmap
// src/lib.rs.
func mustParseEmbedded(t *testing.T, name Name) *CMap {
	t.Helper()
	data, ok := LoadEmbedded(name)
	if !ok {
		t.Fatalf("embedded %s CMap not found", name)
	}
	c, err := Parse(data, LoadEmbedded)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	return c
}

func TestEmbeddedH(t *testing.T) {
	c := mustParseEmbedded(t, H)
	wantMode(t, c, nil)
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeJapan1}, 1))
	wantCID(t, c, 0x2121, 2, 633)
	wantCID(t, c, 0x2221, 2, 727)
}

func TestEmbedded90msRksjH(t *testing.T) {
	c := mustParseEmbedded(t, N90msRksjH)
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeJapan1}, 2))
	wantCID(t, c, 0x20, 1, 231)
	wantCID(t, c, 0x8140, 2, 633)
}

func TestEmbeddedV(t *testing.T) {
	c := mustParseEmbedded(t, V)
	wantMode(t, c, mode(Vertical))
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeJapan1}, 1))
	wantCID(t, c, 0x2121, 2, 633)
}

func TestEmbeddedGbkEucH(t *testing.T) {
	c := mustParseEmbedded(t, GbkEucH)
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeGB1}, 2))
	wantCID(t, c, 0x20, 1, 7716)
	wantCID(t, c, 0x21, 1, 814)
	wantCID(t, c, 0x7e, 1, 814+0x7e-0x21)
	wantCID(t, c, 0xfe80, 2, 22094)
	wantCID(t, c, 0xfea0, 2, 22094+0xa0-0x80)
}

func TestEmbeddedKscEucH(t *testing.T) {
	c := mustParseEmbedded(t, KscEucH)
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeKorea1}, 0))
	wantCID(t, c, 0x20, 1, 8094)
	wantCID(t, c, 0x41, 1, 8094+0x41-0x20)
	wantCID(t, c, 0xa1a1, 2, 101)
	wantCID(t, c, 0xa1fe, 2, 101+0xfe-0xa1)
	wantCID(t, c, 0xfda1, 2, 7962)
}

func TestEmbeddedB5pcH(t *testing.T) {
	c := mustParseEmbedded(t, B5pcH)
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeCNS1}, 0))
	wantCID(t, c, 0x20, 1, 1)
	wantCID(t, c, 0x41, 1, 1+0x41-0x20)
	wantCID(t, c, 0x80, 1, 61)
	wantCID(t, c, 0xd140, 2, 7251)
	wantCID(t, c, 0xd17e, 2, 7251+0x7e-0x40)
	wantCID(t, c, 0xf9d5, 2, 13642+3)
}

func TestEmbeddedUniJisUtf16H(t *testing.T) {
	c := mustParseEmbedded(t, UniJisUtf16H)
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeJapan1}, 7))
	wantCID(t, c, 0x20, 2, 1)
	wantCID(t, c, 0x41, 2, 1+0x41-0x20)
	wantCID(t, c, 0x5c, 2, 97)
	wantCID(t, c, 0x5bcc, 2, 3531)
	wantCID(t, c, 0xd884df50, 4, 19130)
}

func TestEmbeddedIdentityH(t *testing.T) {
	c := mustParseEmbedded(t, IdentityH)
	wantMode(t, c, nil)
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeIdentity}, 0))
	wantCID(t, c, 0x0000, 2, 0)
	wantCID(t, c, 0x0041, 2, 0x41)
	wantCID(t, c, 0xffff, 2, 0xffff)
}

func TestEmbeddedIdentityV(t *testing.T) {
	c := mustParseEmbedded(t, IdentityV)
	wantMode(t, c, mode(Vertical))
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeIdentity}, 0))
	wantCID(t, c, 0x0000, 2, 0)
	wantCID(t, c, 0x0041, 2, 0x41)
	wantCID(t, c, 0xffff, 2, 0xffff)
}

func TestEmbeddedAdobeGb1Ucs2(t *testing.T) {
	c := mustParseEmbedded(t, AdobeGb1Ucs2)
	wantCollection(t, c, collection(CIDFamily{Kind: Custom, Registry: "Adobe", Ordering: "Adobe_GB1_UCS2"}, 5))
	wantBf(t, c, 0x0063, NewBfChar('·'))
	wantBf(t, c, 0x0001, NewBfChar(' '))
	wantBf(t, c, 0x0022, NewBfChar('A'))
	wantBf(t, c, 0x005f, NewBfChar('~'))
	wantBf(t, c, 0x560f, NewBfString("郎︀"))
}

func TestEmbeddedAdobeJapan1Ucs2(t *testing.T) {
	c := mustParseEmbedded(t, AdobeJapan1Ucs2)
	wantCollection(t, c, collection(CIDFamily{Kind: Custom, Registry: "Adobe", Ordering: "Adobe_Japan1_UCS2"}, 7))
	wantBf(t, c, 0x003d, NewBfChar('¥'))
	wantBf(t, c, 0x0001, NewBfChar(' '))
	wantBf(t, c, 0x003c, NewBfChar('['))
	wantBf(t, c, 0x0476, NewBfString("芦\U000E0100"))
	wantBf(t, c, 0x265b, NewBfString("10/11"))
	wantBf(t, c, 0x2e6b, NewBfString("オングストローム"))
}

func TestEmbeddedAdobeKorea1Ucs2(t *testing.T) {
	c := mustParseEmbedded(t, AdobeKorea1Ucs2)
	wantCollection(t, c, collection(CIDFamily{Kind: Custom, Registry: "Adobe", Ordering: "Adobe_Korea1_UCS2"}, 2))
	wantBf(t, c, 0x0060, NewBfChar('₩'))
	wantBf(t, c, 0x2072, NewBfString("[10]"))
	wantBf(t, c, 0x2073, NewBfString("[11]"))
}

func TestEmbeddedAdobeCns1Ucs2(t *testing.T) {
	c := mustParseEmbedded(t, AdobeCns1Ucs2)
	wantCollection(t, c, collection(CIDFamily{Kind: Custom, Registry: "Adobe", Ordering: "Adobe_CNS1_UCS2"}, 7))
	wantBf(t, c, 0x0060, NewBfChar('©'))
	wantBf(t, c, 0x0001, NewBfChar(' '))
	wantBf(t, c, 0x005f, NewBfChar('~'))
	wantBf(t, c, 0x36b0, NewBfChar('\U000200CC'))
}

func TestEmbeddedETenB5H(t *testing.T) {
	c := mustParseEmbedded(t, ETenB5H)
	wantCollection(t, c, collection(CIDFamily{Kind: AdobeCNS1}, 0))
	wantCID(t, c, 0x20, 1, 13648)
	wantCID(t, c, 0x7e, 1, 13648+0x7e-0x20)
	wantCID(t, c, 0xa140, 2, 99)
	wantCID(t, c, 0xa158, 2, 99+0x58-0x40)
	wantCID(t, c, 0xd040, 2, 7094)
	wantCID(t, c, 0xf9fe, 2, 14056+0xfe-0xd6)
}
