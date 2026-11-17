// This file is not in github.com/mgilbir/gowemf. It holds the tests of the
// changes that ivy made to its copy (internal/gowemf/README.md).

package gowemf

import (
	"errors"
	"image/color"
	"math"
	"testing"
	"unicode/utf16"
)

// TestTolerantFraming checks that Walk reads the files that break the
// framing rules of MS-WMF and MS-EMF in the ways that Windows accepts.
func TestTolerantFraming(t *testing.T) {
	wmf := func(edit func([]byte) []byte) []byte { return edit(wmfFixture(false)) }
	emf := func(edit func([]byte) []byte) []byte { return edit(emfFixture()) }
	for name, b := range map[string][]byte{
		"wmf max record": wmf(func(b []byte) []byte { put32(b, 12, 4); return b }),
		"wmf bytes after EOF": wmf(func(b []byte) []byte {
			b = append(b, 0, 0)
			put32(b, 6, uint32(len(b)/2))
			return b
		}),
		"wmf without EOF": wmf(func(b []byte) []byte {
			b = b[:len(b)-6]
			put32(b, 6, uint32(len(b)/2))
			return b
		}),
		"wmf size too small": wmf(func(b []byte) []byte { put32(b, 6, 9); return b }),
		// A WMF of the store of Office Drawing counts the placeable header
		// that the store removed.
		"wmf size with a placeable header": wmf(func(b []byte) []byte { put32(b, 6, uint32(len(b)/2+11)); return b }),
		"emf record count":                 emf(func(b []byte) []byte { put32(b, 52, 3); return b }),
		"emf bytes after EOF": emf(func(b []byte) []byte {
			b = append(b, 0, 0, 0, 0)
			put32(b, 48, uint32(len(b)))
			return b
		}),
		"emf without EOF": emf(func(b []byte) []byte {
			b = b[:len(b)-20]
			put32(b, 48, uint32(len(b)))
			return b
		}),
		"emf size too small": emf(func(b []byte) []byte { put32(b, 48, 88); return b }),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Walk(b, Limits{}, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
	// A size larger than the data means that the file is cut short.
	for name, b := range map[string][]byte{
		"wmf":          wmf(func(b []byte) []byte { put32(b, 6, 18); return b }),
		"wmf 22 bytes": wmf(func(b []byte) []byte { put32(b, 6, uint32(len(b)/2+12)); return b }),
		"placeable wmf 22 bytes": func() []byte {
			b := wmfFixture(true)
			put32(b, 22+6, uint32((len(b)-22)/2+11))
			return b
		}(),
		"emf": emf(func(b []byte) []byte { put32(b, 48, uint32(len(b)+4)); return b }),
	} {
		t.Run(name+" size too large", func(t *testing.T) {
			if _, err := Walk(b, Limits{}, nil); !errors.Is(err, ErrMalformed) {
				t.Fatal(err)
			}
		})
	}
}

// TestPlayIgnoredReferences checks that Play draws the records of a file
// that refers to a missing object, or to an object of another type, and
// leaves out the records that Windows ignores.
func TestPlayIgnoredReferences(t *testing.T) {
	place := &PlaceableHeader{Bounds: Rect{0, 0, 10, 10}, UnitsPerInch: 96}
	clip := wmfBox(MetaIntersectClipRect, 0, 0, 5, 5)
	box := wmfBox(MetaRectangle, 0, 0, 10, 10)
	for name, tt := range map[string]struct {
		records []Record
		clipped bool
	}{
		// SelectClipRegion with an empty slot removes the clip.
		"empty region slot": {[]Record{clip, wmfRec(MetaSelectClipRegion, 0), box}, false},
		// SelectClipRegion with a brush fails, and the clip stays.
		"brush as region": {[]Record{wmfBrush(0, 0xff, 0), clip, wmfRec(MetaSelectClipRegion, 0), box}, true},
		// The selection and the deletion of a missing object do nothing.
		"missing object": {[]Record{wmfRec(MetaSelectObject, 1), wmfRec(MetaDeleteObject, 1), clip, box}, true},
		// A region paint that names a brush as its region does nothing.
		"region paint": {[]Record{wmfBrush(0, 0xff, 0), wmfRec(MetaFillRegion, 0, 0), wmfRec(MetaPaintRegion, 0), box}, false},
	} {
		t.Run(name, func(t *testing.T) {
			b := record(t, wmfDocument(2, tt.records...), PlayOptions{Destination: Box{Width: 10, Height: 10}, Placeable: place})
			if len(b.fills) != 1 {
				t.Fatal("fills", b.fills)
			}
			if clipped := len(b.fills[0].clip) != 0; clipped != tt.clipped {
				t.Fatal("clip", b.fills[0].clip)
			}
		})
	}
}

// TestWMFShortTextAdvances checks that an ExtTextOut record too short for
// all its advances decodes without them.
func TestWMFShortTextAdvances(t *testing.T) {
	// y, x, the length 3, no options, "abc" with a pad byte, and only two
	// advances for the three characters
	body := cat(words(10, 20, 3, 0), []byte{'a', 'b', 'c', 0}, words(7, 7))
	v, err := Decode(testRecord(WMF, MetaExtTextOut, 0, body), DecodeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	text := v.(Text)
	if string(text.Bytes) != "abc" || text.Advances.Len() != 0 {
		t.Fatal(text)
	}
}

// TestWMFPatternBrushDIB checks that a BS_PATTERN brush that holds a DIB in
// place of a Bitmap16 paints its pattern, with the colors of the device
// context for a monochrome DIB.
func TestWMFPatternBrushDIB(t *testing.T) {
	// a BITMAPINFOHEADER of 8 by 8 pixels of 1 bit, a color table of black
	// and white, and the bits, all 0
	dib := cat(longs(40, 8, 8), words(1, 1), longs(0, 32, 0, 0, 0, 0), longs(0, 0xffffff), make([]byte, 32))
	brush := testRecord(WMF, MetaDIBCreatePatternBrush, 0, cat(words(3, 0), dib))
	place := &PlaceableHeader{Bounds: Rect{0, 0, 10, 10}, UnitsPerInch: 96}
	file := wmfDocument(1, brush, wmfRec(MetaSelectObject, 0), testRecord(WMF, MetaSetTextColor, 0, longs(0x0000ff)), wmfBox(MetaRectangle, 0, 0, 10, 10))
	b := record(t, file, PlayOptions{Destination: Box{Width: 10, Height: 10}, Placeable: place})
	if len(b.fills) != 1 || b.fills[0].paint.Kind != PaintPattern {
		t.Fatal(b.fills)
	}
	if c := color.NRGBAModel.Convert(b.fills[0].paint.Pattern.At(0, 0)); c != (color.NRGBA{255, 0, 0, 255}) {
		t.Fatal("pattern color", c)
	}
}

// TestPlayEMFPlusDrawString checks the layout of EmfPlusDrawString. The
// font of fakeText has an em of 10 pixels, an ascent of 8, a descent of 2,
// and advances of 5, or 2.5 for a space.
func TestPlayEMFPlusDrawString(t *testing.T) {
	font := plusFontObj(1, 10, 2, 0, "F")
	str := func(format uint32, box []float64, s string) []byte {
		units := utf16.Encode([]rune(s))
		text := make([]byte, 2*len(units))
		for i, u := range units {
			put16(text, 2*i, u)
		}
		return plusRec(PlusDrawStringRecord, 0x8001, dwords(0xff000000, format, uint32(len(units))), fl(box...), text)
	}
	// flags, the language, the alignment, the line alignment, the digits, a
	// tab offset, the hotkey prefix, the margins, the tracking, the
	// trimming, and no tab stops or ranges
	format := func(flags, align, lineAlign uint32) []byte {
		return plusObj(2, 7, dwords(plusVersion, flags, 0, align, lineAlign, 0, 0), fl(0), dwords(0), fl(0, 0, 1), dwords(0, 0, 0))
	}
	type line struct {
		text string
		x, y float64
	}
	for _, tt := range []struct {
		name    string
		records [][]byte
		want    []line
	}{
		// the default format: margins of 1/6 em and a tracking of 1.03
		{"default", [][]byte{font, str(0xffffffff, []float64{0, 0, 100, 40}, "AB CD")}, []line{{"AB CD", 10.0 / 6, 8}}},
		{"wrap", [][]byte{font, str(0xffffffff, []float64{0, 0, 20, 40}, "AB CD")}, []line{{"AB", 10.0 / 6, 8}, {"CD", 10.0 / 6, 18}}},
		{"line feed", [][]byte{font, str(0xffffffff, []float64{0, 0, 0, 0}, "AB\r\nC")}, []line{{"AB", 10.0 / 6, 8}, {"C", 10.0 / 6, 18}}},
		// centered in both directions: the line is 10 wide and 10 high
		{"center", [][]byte{font, format(0, 1, 1), str(2, []float64{0, 0, 30, 30}, "AB")}, []line{{"AB", 10, 18}}},
		{"far", [][]byte{font, format(0, 2, 2), str(2, []float64{0, 0, 30, 30}, "AB")}, []line{{"AB", 20, 28}}},
		{"no wrap", [][]byte{font, format(plusStringNoWrap, 0, 0), str(2, []float64{0, 0, 5, 30}, "AB CD")}, []line{{"AB CD", 0, 8}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var skipped []string
			b := &fakeText{}
			o := PlayOptions{Destination: Box{Width: 96, Height: 64}, Unsupported: func(u UnsupportedOperation) error {
				skipped = append(skipped, u.Reason)
				return nil
			}}
			if _, err := Play(plusScene(96, 64, tt.records...), o, b); err != nil {
				t.Fatal(err)
			}
			if b.recordingBackend.bad != nil || len(skipped) != 0 || len(b.drawn) != len(tt.want) {
				t.Fatal(b.recordingBackend.bad, skipped, b.drawn)
			}
			for i, w := range tt.want {
				run := b.drawn[i]
				if got := string(utf16.Decode(run.Text)); got != w.text {
					t.Errorf("line %d: %q, want %q", i, got, w.text)
				}
				if o := run.Origins[0]; math.Abs(o.X-w.x) > 1e-6 || math.Abs(o.Y-w.y) > 1e-6 {
					t.Errorf("line %d: origin %v, want (%g, %g)", i, o, w.x, w.y)
				}
			}
		})
	}
}

// TestROP2Raster checks the ternary raster operations of the mix modes.
func TestROP2Raster(t *testing.T) {
	for rop2, want := range map[uint32]RasterOperation{1: 0x00, 6: 0x55, 7: 0x5a, 9: 0xa0, 11: 0xaa, 13: 0xf0, 15: 0xfa, 16: 0xff} {
		if got := rop2Raster(rop2); got != want {
			t.Errorf("R2 %d: 0x%02x, want 0x%02x", rop2, got, want)
		}
	}
	for rop2, want := range map[uint32]bool{0: false, 1: false, 4: false, 6: true, 9: true, 11: false, 13: false, 17: false} {
		if got := rop2ReadsDestination(rop2); got != want {
			t.Errorf("R2 %d reads the destination: %v", rop2, got)
		}
	}
}

// TestEMFRegionBufferSize checks that the region data of an EMF record
// takes its rectangles from nCount, and not from nRgnSize, which can be 0
// or the size of a larger buffer.
func TestEMFRegionBufferSize(t *testing.T) {
	for _, size := range []int32{0, 16, 168} {
		// the size of the region data, the mode RGN_COPY, the header of 32
		// bytes with one rectangle, the bounds, and the rectangle
		body := cat(longs(48, 5, 32, 1, 1, size), longs(100, 43, 695, 513), longs(100, 43, 695, 513))
		v, err := Decode(testRecord(EMF, EMRExtSelectClipRgn, 0, body), DecodeLimits{})
		if err != nil {
			t.Fatalf("nRgnSize %d: %v", size, err)
		}
		if r := v.(Region); r.Count != 1 || len(r.Rectangles) != 16 {
			t.Fatalf("nRgnSize %d: %+v", size, r)
		}
	}
	// A count of rectangles that do not fit in the data is still an error.
	body := cat(longs(48, 5, 32, 1, 2, 32), longs(0, 0, 1, 1), longs(0, 0, 1, 1))
	if _, err := Decode(testRecord(EMF, EMRExtSelectClipRgn, 0, body), DecodeLimits{}); !errors.Is(err, ErrMalformed) {
		t.Fatal(err)
	}
}

// TestPlayEMFPlusMetafileImageOrigin checks that the source rectangle of a
// metafile image is in the coordinates of its frame, or of its placeable
// bounds, which need not start at 0.
func TestPlayEMFPlusMetafileImageOrigin(t *testing.T) {
	half := plusRec(PlusSetPixelOffsetModeRecord, 4)
	// innerEMF moved by (100, 50) pixels: the frame, the bounds and the
	// red rectangle start there.
	emf := emfScene(40, 20, 2, emfSelect(nullPen), emfBrush(1, 0, red), emfSelect(1), emfBox(EMRRectangle, 100, 50, 120, 60))
	copy(emf[8:], longs(100, 50, 139, 69, 2500, 1250, 3500, 1750))
	img := plusObj(1, 5, metafileImageObj(3, emf))
	b, skipped := plusPlay(t, plusScene(96, 64, half, img, drawImage(1, [4]float64{100, 50, 41, 21}, [4]float64{10, 10, 82, 42})), PlayOptions{})
	if len(skipped) != 0 || len(b.fills) != 1 || !pointsNear(b.fills[0].path.Points, Point{10, 10}, Point{10, 31}, Point{51, 31}, Point{51, 10}) {
		t.Fatal("EMF", skipped, b.fills)
	}
	// The frame with the rounding errors of float32 is still inside.
	b, skipped = plusPlay(t, plusScene(96, 64, half, img, drawImage(1, [4]float64{100.0001, 49.9999, 41.0001, 21.0002}, [4]float64{10, 10, 82, 42})), PlayOptions{})
	if len(skipped) != 0 || len(b.fills) != 1 {
		t.Fatal("EMF with rounding", skipped, b.fills)
	}
	// A placeable WMF with the bounds (-10, -20) to (38, 12).
	wmf := wmfScene(38, 12, 2, wmfPen(5, 0, 0), wmfRec(MetaSelectObject, 0), wmfBrush(0, blue, 0), wmfRec(MetaSelectObject, 1), wmfBox(MetaRectangle, -10, -20, 14, -4))
	put16(wmf, 6, uint16(0xfff6))
	put16(wmf, 8, uint16(0xffec))
	var sum uint16
	for i := 0; i < 20; i += 2 {
		sum ^= u16(wmf[i:])
	}
	put16(wmf, 20, sum)
	points := plusRec(PlusDrawImagePointsRecord, 1, dwords(0xffffffff, 2), fl(-10, -20, 48, 32), dwords(3), fl(10, 10, 58, 10, 10, 42))
	b, skipped = plusPlay(t, plusScene(96, 64, half, plusObj(1, 5, metafileImageObj(2, wmf)), points), PlayOptions{})
	if len(skipped) != 0 || len(b.fills) != 1 || !pointsNear(b.fills[0].path.Points, Point{10, 10}, Point{10, 25}, Point{33, 25}, Point{33, 10}) {
		t.Fatal("WMF", skipped, b.fills)
	}
}

// TestWMFShortFaceName checks that a WMF CreateFontIndirect record with a
// face name of fewer than 32 bytes decodes, as MathType writes it.
func TestWMFShortFaceName(t *testing.T) {
	// the height, the width, the escapement, the orientation, the weight,
	// the flags, and "Symbol" with its NUL and one byte more
	body := cat(words(-224, 0, 0, 0, 400), []byte{0, 0, 0, 2, 0, 0, 0, 0}, []byte("Symbol\x00\x15"))
	v, err := Decode(testRecord(WMF, MetaCreateFontIndirect, 0, body), DecodeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if f := v.(Font); string(f.FaceName) != "Symbol\x00\x15" || f.CharSet != 2 {
		t.Fatalf("%+v", f)
	}
}
