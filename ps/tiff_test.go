package ps

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// packedTIFF2 returns a big-endian TIFF file of 5 by 2 pixels of 2 bits,
// white is zero, without compression. The first row holds the levels 0, 1,
// 2, 3 and 0, and the second row the level 3. extra are more entries of the
// IFD, as tag, type and value.
func packedTIFF2(extra ...[3]uint32) []byte {
	pixels := []byte{0x1b, 0x00, 0xff, 0xc0}
	entries := make([][3]uint32, 0, 7+len(extra))
	entries = append(entries,
		[3]uint32{tagImageWidth, 3, 5},
		[3]uint32{tagImageLength, 3, 2},
		[3]uint32{tagBitsPerSample, 3, 2},
		[3]uint32{259, 3, 1},
		[3]uint32{tagPhotometric, 3, photoWhiteIsZero},
		[3]uint32{273, 4, 0},
		[3]uint32{279, 4, uint32(len(pixels))},
	)
	entries = append(entries, extra...)
	buf := []byte("MM\x00*\x00\x00\x00\x08")
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(entries)))
	data := uint32(8 + 2 + 12*len(entries) + 4)
	for _, e := range entries {
		if e[0] == 273 {
			e[2] = data
		}
		buf = binary.BigEndian.AppendUint16(buf, uint16(e[0]))
		buf = binary.BigEndian.AppendUint16(buf, uint16(e[1]))
		buf = binary.BigEndian.AppendUint32(buf, 1)
		if e[1] == 3 {
			buf = binary.BigEndian.AppendUint16(buf, uint16(e[2]))
			buf = append(buf, 0, 0)
		} else {
			buf = binary.BigEndian.AppendUint32(buf, e[2])
		}
	}
	buf = append(buf, 0, 0, 0, 0)
	return append(buf, pixels...)
}

func TestPackedTIFF(t *testing.T) {
	buf := packedTIFF2()
	p, ok := readPackedTIFF(buf)
	if !ok {
		t.Fatal("expected a packed image")
	}
	img, err := p.decode(buf)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != image.Rect(0, 0, 5, 2) {
		t.Fatalf("expected 5x2, got: %v", img.Bounds())
	}
	want := []uint8{255, 170, 85, 0, 255}
	for x, v := range want {
		if got := color.GrayModel.Convert(img.At(x, 0)).(color.Gray).Y; got != v {
			t.Errorf("at %d,0: expected %d, got: %d", x, v, got)
		}
		if got := color.GrayModel.Convert(img.At(x, 1)).(color.Gray).Y; got != 0 {
			t.Errorf("at %d,1: expected 0, got: %d", x, got)
		}
	}
	// a predictor and tiles change the layout of the bytes, so the reader
	// leaves them to golang.org/x/image/tiff
	for _, extra := range [][3]uint32{{tagPredictor, 3, 2}, {tagTileWidth, 3, 16}, {tagSamplesPerPixel, 3, 2}} {
		if _, ok := readPackedTIFF(packedTIFF2(extra)); ok {
			t.Errorf("tag %d: expected no packed image", extra[0])
		}
	}
	if _, ok := readPackedTIFF(buf[:20]); ok {
		t.Error("expected no packed image of a short file")
	}
}
