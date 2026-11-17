package pptx

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/msole"
)

// readEntry returns the entry with the name of the zip archive buf.
func readEntry(t *testing.T, buf []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := zr.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return data
}

// smartArt returns a presentation with one slide that holds a diagram of
// SmartArt in a frame at (100, 100) of 200 by 100 points. The drawing of
// the diagram has a red rectangle in the left half of the frame.
func smartArt(t *testing.T) []byte {
	t.Helper()
	frame := `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="2" name="Diagram"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
		`<p:xfrm><a:off x="1270000" y="1270000"/><a:ext cx="2540000" cy="1270000"/></p:xfrm>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram">` +
		`<dgm:relIds xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" r:dm="rIdD1" r:lo="rIdD3" r:qs="rIdD4" r:cs="rIdD5"/>` +
		`</a:graphicData></a:graphic></p:graphicFrame>`
	buf := (&fixture.PPTX{Slides: []fixture.PPTXSlide{{Shapes: frame}}}).Bytes()
	data := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<dgm:ptLst><dgm:pt modelId="1" type="doc"/></dgm:ptLst><dgm:extLst><a:ext uri="http://schemas.microsoft.com/office/drawing/2008/diagram">` +
		`<dsp:dataModelExt xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" relId="rIdD2" minVer="http://schemas.openxmlformats.org/drawingml/2006/diagram"/>` +
		`</a:ext></dgm:extLst></dgm:dataModel>`
	drawing := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<dsp:spTree><dsp:nvGrpSpPr><dsp:cNvPr id="0" name=""/><dsp:cNvGrpSpPr/></dsp:nvGrpSpPr><dsp:grpSpPr/>` +
		`<dsp:sp modelId="{1}"><dsp:nvSpPr><dsp:cNvPr id="0" name=""/><dsp:cNvSpPr/></dsp:nvSpPr>` +
		`<dsp:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="1270000" cy="1270000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>` +
		`<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill><a:ln><a:noFill/></a:ln></dsp:spPr></dsp:sp>` +
		`</dsp:spTree></dsp:drawing>`
	rels := string(readEntry(t, buf, "ppt/slides/_rels/slide1.xml.rels"))
	add := `<Relationship Id="rIdD1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="../diagrams/data1.xml"/>` +
		`<Relationship Id="rIdD2" Type="http://schemas.microsoft.com/office/2007/relationships/diagramDrawing" Target="../diagrams/drawing1.xml"/>`
	rels = strings.Replace(rels, "</Relationships>", add+"</Relationships>", 1)
	return fixture.Rezip(buf,
		fixture.Entry{Name: "ppt/slides/_rels/slide1.xml.rels", Data: []byte(rels)},
		fixture.Entry{Name: "ppt/diagrams/data1.xml", Data: []byte(data)},
		fixture.Entry{Name: "ppt/diagrams/drawing1.xml", Data: []byte(drawing)},
	)
}

func TestSmartArt(t *testing.T) {
	img := render(t, smartArt(t), 0)
	// the drawing is in the space of the frame
	expect(t, img, 150, 150, red)
	expect(t, img, 250, 150, white)
	expect(t, img, 95, 150, white)
}

// standardEncrypt encrypts the package pkg with the Standard encryption of
// MS-OFFCRYPTO (2.3.4.5 to 2.3.4.9), with the EncryptionInfo stream info,
// whose verifier is for the password, and returns the Compound File.
func standardEncrypt(t *testing.T, info []byte, password string, pkg []byte) []byte {
	t.Helper()
	headerSize := binary.LittleEndian.Uint32(info[8:])
	header := info[12 : 12+headerSize]
	keyBits := binary.LittleEndian.Uint32(header[16:])
	verifier := info[12+headerSize:]
	salt := verifier[4 : 4+binary.LittleEndian.Uint32(verifier)]
	// the hash of the salt and the password, 50000 times, and of block 0
	pw := utf16.Encode([]rune(password))
	h := sha1.New()
	h.Write(salt)
	for _, u := range pw {
		h.Write([]byte{byte(u), byte(u >> 8)})
	}
	sum := h.Sum(nil)
	for i := range uint32(50000) {
		h.Reset()
		h.Write(binary.LittleEndian.AppendUint32(nil, i))
		h.Write(sum)
		sum = h.Sum(nil)
	}
	h.Reset()
	h.Write(sum)
	h.Write([]byte{0, 0, 0, 0})
	sum = h.Sum(nil)
	buf := bytes.Repeat([]byte{0x36}, 64)
	for i, b := range sum {
		buf[i] ^= b
	}
	x1 := sha1.Sum(buf)
	block, err := aes.NewCipher(x1[:keyBits/8])
	if err != nil {
		t.Fatal(err)
	}
	plain := make([]byte, (len(pkg)+15)/16*16)
	copy(plain, pkg)
	for i := 0; i < len(plain); i += 16 {
		block.Encrypt(plain[i:i+16], plain[i:i+16])
	}
	stream := append(binary.LittleEndian.AppendUint64(nil, uint64(len(pkg))), plain...)
	return fixture.CFB(
		fixture.CFBEntry{Name: "EncryptionInfo", Data: info},
		fixture.CFBEntry{Name: "EncryptedPackage", Data: stream},
	)
}

func TestPassword(t *testing.T) {
	// the EncryptionInfo of a file of msoffcrypto-tool, whose password is
	// Password1234_
	const password = "Password1234_"
	buf, err := os.ReadFile(filepath.Join("..", "internal", "ooxml", "testdata", "ecma376standard_password.docx"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := msole.Open(bytes.NewReader(buf), ivy.Limits{}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	info, err := msole.ReadStream(f, "EncryptionInfo", ivy.Limits{}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	enc := standardEncrypt(t, info, password, smartArt(t))
	if _, err := Format.Open(t.Context(), bytes.NewReader(enc), ivy.Config{}); !errors.Is(err, ivy.ErrPassword) {
		t.Fatalf("expected %v without the password, got: %v", ivy.ErrPassword, err)
	}
	if _, err := Format.Open(t.Context(), bytes.NewReader(enc), ivy.Config{Password: "wrong"}); !errors.Is(err, ivy.ErrPassword) {
		t.Fatalf("expected %v with a wrong password, got: %v", ivy.ErrPassword, err)
	}
	doc, err := Format.Open(t.Context(), bytes.NewReader(enc), ivy.Config{Password: password})
	if err != nil {
		t.Fatal(err)
	}
	p, err := doc.(ivy.Pager).Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if !near(img, 150, 150, red, 12) || img.Bounds() != image.Rect(0, 0, 720, 540) {
		t.Errorf("expected the red slide of 720 by 540, got %v at (150, 150) on %v", img.At(150, 150), img.Bounds())
	}
}
