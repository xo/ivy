package odraw

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"image/color"
	"image/png"
	"testing"
)

// pictBlip returns an OfficeArtBlipPICT record of the picture pic, which
// zlib compresses when compress is set (MS-ODRAW 2.2.28).
func pictBlip(t *testing.T, pic []byte, compress bool) []byte {
	t.Helper()
	data, flag := pic, byte(0xFE)
	if compress {
		var buf bytes.Buffer
		zw := zlib.NewWriter(&buf)
		if _, err := zw.Write(pic); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		data, flag = buf.Bytes(), 0
	}
	body := make([]byte, 16+34, 16+34+len(data))
	binary.LittleEndian.PutUint32(body[16:], uint32(len(pic)))
	body[16+32], body[16+33] = flag, 0xFE
	body = append(body, data...)
	head := make([]byte, 8, 8+len(body))
	binary.LittleEndian.PutUint16(head, 0x542<<4)
	binary.LittleEndian.PutUint16(head[2:], RecBlipPICT)
	binary.LittleEndian.PutUint32(head[4:], uint32(len(body)))
	return append(head, body...)
}

func TestDataPICT(t *testing.T) {
	// a picture of version 1 of 16 by 2 points, with a BitsRect of a
	// BitMap whose first column is black
	pic := []byte{
		0, 0, 0, 0, 0, 0, 0, 2, 0, 16,
		0x11, 0x01, 0x90,
		0, 2, 0, 0, 0, 0, 0, 2, 0, 16,
		0, 0, 0, 0, 0, 2, 0, 16,
		0, 0, 0, 0, 0, 2, 0, 16,
		0, 0,
		0x80, 0, 0x80, 0,
		0xFF,
	}
	for _, compress := range []bool{false, true} {
		data := Data(pictBlip(t, pic, compress), 1<<20)
		if data == nil {
			t.Fatalf("compress %v: no picture", compress)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if got := img.Bounds().Size(); got.X != 16 || got.Y != 2 {
			t.Errorf("compress %v: size %v", compress, got)
		}
		if got := color.NRGBAModel.Convert(img.At(0, 1)); got != (color.NRGBA{0, 0, 0, 255}) {
			t.Errorf("compress %v: pixel 0, 1 is %v", compress, got)
		}
	}
	// a picture that holds no bitmap gives no picture
	if data := Data(pictBlip(t, pic[:13], false), 1<<20); data != nil {
		t.Errorf("picture without a bitmap: %d bytes", len(data))
	}
}
