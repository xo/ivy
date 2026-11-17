package msxls

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"image/color"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// odraw returns a record of Office Drawing: a container when ver is 0xF.
func odraw(ver, inst, typ int, data ...[]byte) []byte {
	body := bytes.Join(data, nil)
	out := u16(ver|inst<<4, typ)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(body)))
	return append(out, body...)
}

// anchorShape returns the container of a shape with a cell anchor from
// (col0, row0) to (col1, row1), and the picture pib when it is not 0.
func anchorShape(col0, row0, col1, row1, pib int) []byte {
	var opt []byte
	if pib > 0 {
		opt = odraw(3, 1, odrawOPT, u16(0x4104), u32(pib))
	}
	return odraw(0xF, 0, 0xF004,
		odraw(2, 75, odrawFSP, u32(1025, 0xA00)),
		opt,
		odraw(0, 0, odrawClientAnchor, u16(2, col0, 0, row0, 0, col1, 0, row1, 0)),
		odraw(0, 0, 0xF011),
	)
}

// obj returns a record OBJ of the type ot.
func obj(ot int) []byte {
	return append(u16(0x15, 0x12, ot, 1, 0), make([]byte, 12)...)
}

// drawingWorkbook returns a workbook of a worksheet with a picture, a text
// box, a chart and a comment, and of a chart sheet.
func drawingWorkbook(png []byte) []byte {
	var b biff
	b.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
	b.record(recFont, u16(200, 0, 0x7FFF, 400, 0), []byte{0, 0, 0, 0}, []byte{5, 0}, []byte("Arial"))
	b.record(recXF, u16(0, 0), make([]byte, 16))
	// the store of the pictures: one PNG image, which the record cuts in
	// two, as Excel does for a large group
	blip := odraw(0, 0x6E0, odrawBlipPNG, make([]byte, 16), []byte{0xFF}, png)
	fbse := odraw(2, 6, odrawFBSE, []byte{6, 6}, make([]byte, 16), u16(0xFF), u32(len(blip), 1, 0), []byte{0, 0, 0, 0}, blip)
	group := odraw(0xF, 0, 0xF000, odraw(0xF, 1, odrawBStore, fbse))
	half := len(group) / 2
	b.record(recMsoDrawingGroup, group[:half])
	b.record(recMsoDrawingGroup, group[half:])
	sheet1 := b.record(recBoundSheet, u32(0), []byte{0, 0}, []byte{6, 0}, []byte("Drawn"))
	chart := b.record(recBoundSheet, u32(0), []byte{0, 2}, []byte{5, 0}, []byte("Chart"))
	b.record(recEOF)
	binary.LittleEndian.PutUint32(b.buf[sheet1+4:], uint32(len(b.buf)))
	b.record(recBOF, u16(biff8, bofWorksheet, 0, 0), u32(0, 0))
	b.record(recNumber, u16(0, 0, 0), float(1))
	shapes := [][]byte{
		anchorShape(1, 1, 3, 5, 1),
		anchorShape(4, 1, 6, 3, 0),
		anchorShape(1, 6, 5, 12, 0),
		anchorShape(7, 0, 8, 2, 0),
	}
	// the container of the drawing goes over all the records
	all := bytes.Join(shapes, nil)
	dg := odraw(0xF, 0, 0xF002, odraw(0xF, 0, 0xF003, all))
	head := len(dg) - len(all)
	b.record(recMsoDrawing, dg[:head+len(shapes[0])])
	b.record(recObj, obj(objPicture))
	// the drawing of the next shape in a CONTINUE record after the OBJ
	b.record(recMsoDrawing, shapes[1])
	b.record(recObj, obj(objText))
	b.record(recTxo, u16(0x12, 0, 0, 0, 0, 5, 16), make([]byte, 4))
	b.record(recContinue, []byte{0}, []byte("Hello"))
	b.record(recContinue, make([]byte, 16))
	b.record(recContinue, shapes[2])
	b.record(recObj, obj(objChart))
	b.record(recBOF, u16(biff8, 0x20, 0, 0), u32(0, 0))
	b.record(recEOF)
	b.record(recMsoDrawing, shapes[3])
	b.record(recObj, obj(objNote))
	b.record(recEOF)
	binary.LittleEndian.PutUint32(b.buf[chart+4:], uint32(len(b.buf)))
	b.record(recBOF, u16(biff8, 0x20, 0, 0), u32(0, 0))
	b.record(recEOF)
	return fixture.CFB(fixture.CFBEntry{Name: "Workbook", Data: b.buf})
}

func TestDrawings(t *testing.T) {
	png := fixture.PNG(4, 2, color.NRGBA{0, 0, 255, 255})
	d, err := Open(t.Context(), bytes.NewReader(drawingWorkbook(png)))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Documents) != 2 {
		t.Fatalf("expected the worksheet and the chart sheet, got: %q", src.Documents)
	}
	docs := sheets(t, d)
	// the columns of 50.47 points and the rows of 13.07 points
	for _, want := range []string{
		`src="media/image1.png"`,
		`<div class="ph" style="left:0pt;top:0pt;width:100.94pt;height:26.14pt">Hello</div>`,
		`<div class="ph" style="left:0pt;top:0pt;width:201.87pt;height:78.41pt">Chart</div>`,
	} {
		if !strings.Contains(docs[0], want) {
			t.Errorf("expected %q in the sheet:\n%s", want, docs[0])
		}
	}
	if regexp.MustCompile(`class="ph"[^>]*></div>`).MatchString(docs[0]) {
		t.Errorf("expected no frame for the comment:\n%s", docs[0])
	}
	if f, err := src.FS.Open("media/image1.png"); err != nil {
		t.Errorf("expected the picture in the files: %v", err)
	} else {
		_ = f.Close()
	}
	// the chart sheet fills a page of US Letter in landscape
	if !strings.Contains(docs[1], "size:792pt 612pt") || !strings.Contains(docs[1], ">Chart</div>") {
		t.Errorf("expected the chart sheet in landscape:\n%s", docs[1])
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 pages, got: %d", n)
	}
}

func TestBlipData(t *testing.T) {
	file := []byte("a metafile of the test")
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	_, _ = zw.Write(file)
	_ = zw.Close()
	// an OfficeArtMetafileHeader: the size, the bounds, the size in EMU,
	// the saved size, the compression and the filter
	header := func(compression byte, saved int) []byte {
		h := u32(len(file), 0, 0, 100, 100, 1000, 1000, saved)
		return append(h, compression, 0xFE)
	}
	uid := make([]byte, 16)
	for _, test := range []struct {
		name     string
		typ      uint16
		inst     uint16
		data     []byte
		exp, ext string
	}{
		{"emf deflate", odrawBlipEMF, 0x3D4, bytes.Join([][]byte{uid, header(0, z.Len()), z.Bytes()}, nil), string(file), ".emf"},
		{"wmf stored, two ids", odrawBlipWMF, 0x217, bytes.Join([][]byte{uid, uid, header(0xFE, len(file)), file}, nil), string(file), ".wmf"},
		{"png", odrawBlipPNG, 0x6E0, bytes.Join([][]byte{uid, {0xFF}, []byte("\x89PNG...")}, nil), "\x89PNG...", ".png"},
		{"jpeg, two ids", odrawBlipJPEG, 0x46B, bytes.Join([][]byte{uid, uid, {0xFF}, []byte("\xff\xd8")}, nil), "\xff\xd8", ".jpg"},
		{"dib", 0xF01F, 0x7A8, bytes.Join([][]byte{uid, {0xFF}, make([]byte, 40)}, nil), "", ""},
		{"short", odrawBlipEMF, 0x3D4, uid, "", ""},
	} {
		got := blipData(test.typ, test.inst, test.data, 1<<20)
		if string(got.data) != test.exp || got.ext != test.ext {
			t.Errorf("%s: expected %q %q, got: %q %q", test.name, test.exp, test.ext, got.data, got.ext)
		}
	}
}
