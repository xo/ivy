package fixture

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
)

// JPEGDPI returns the JPEG file buf with a JFIF header that gives it the
// resolution of dpi dots per inch. Any header that buf has after its start
// stays.
func JPEGDPI(buf []byte, dpi int) []byte {
	app0 := []byte{0xff, 0xe0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, 1}
	app0 = binary.BigEndian.AppendUint16(app0, uint16(dpi))
	app0 = binary.BigEndian.AppendUint16(app0, uint16(dpi))
	app0 = append(app0, 0, 0)
	out := append([]byte{}, buf[:2]...)
	out = append(out, app0...)
	return append(out, buf[2:]...)
}

// PNGDPI returns the PNG file buf with a pHYs chunk after its IHDR chunk,
// which gives it the resolution of dpi dots per inch.
func PNGDPI(buf []byte, dpi int) []byte {
	ppm := uint32(float64(dpi)/0.0254 + 0.5)
	var body bytes.Buffer
	body.WriteString("pHYs")
	_ = binary.Write(&body, binary.BigEndian, ppm)
	_ = binary.Write(&body, binary.BigEndian, ppm)
	body.WriteByte(1)
	chunk := binary.BigEndian.AppendUint32(nil, 9)
	chunk = append(chunk, body.Bytes()...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(body.Bytes()))
	// the signature is 8 bytes and the IHDR chunk 25
	out := append([]byte{}, buf[:33]...)
	out = append(out, chunk...)
	return append(out, buf[33:]...)
}
