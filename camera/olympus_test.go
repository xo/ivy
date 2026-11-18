package camera

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// olympusNotes returns maker notes of Olympus in the new form: the header
// "OLYMPUS\0II", an IFD whose CameraSettings point to a second IFD, and the
// JPEG jpg that the second IFD names as its preview. The offsets count from
// the start of the notes.
func olympusNotes(jpg []byte) []byte {
	le := binary.LittleEndian
	out := []byte("OLYMPUS\x00II\x03\x00")
	// the main IFD at 12, of one entry, and the camera settings at 30
	out = le.AppendUint16(out, 1)
	out = le.AppendUint16(out, tagOlympusCameraSettings)
	out = le.AppendUint16(out, typeIFD)
	out = le.AppendUint32(out, 1)
	out = le.AppendUint32(out, 30)
	out = le.AppendUint32(out, 0)
	// the camera settings, of two entries, and the JPEG at 60
	out = le.AppendUint16(out, 2)
	for _, e := range [][2]uint32{{tagSettingsPreviewStart, 60}, {tagSettingsPreviewLength, uint32(len(jpg))}} {
		out = le.AppendUint16(out, uint16(e[0]))
		out = le.AppendUint16(out, typeLong)
		out = le.AppendUint32(out, 1)
		out = le.AppendUint32(out, e[1])
	}
	out = le.AppendUint32(out, 0)
	return append(out, jpg...)
}

// oldOlympusNotes returns maker notes of Olympus in the old form, "OLYMP\0",
// with a ThumbnailImage. Their offsets count from the start of the file, and
// the notes start at start.
func oldOlympusNotes(jpg []byte, start uint32) []byte {
	le := binary.LittleEndian
	out := []byte("OLYMP\x00\x01\x00")
	out = le.AppendUint16(out, 1)
	out = le.AppendUint16(out, tagOlympusThumbnail)
	out = le.AppendUint16(out, typeUndefined)
	out = le.AppendUint32(out, uint32(len(jpg)))
	out = le.AppendUint32(out, start+8+2+12+4)
	out = le.AppendUint32(out, 0)
	return append(out, jpg...)
}

// orf returns an ORF file whose Exif IFD holds the maker notes that notes
// makes from the offset of the notes in the file.
func orf(notes func(start uint32) []byte) []byte {
	header := []byte("IIRO\x08\x00\x00\x00")
	blobs := make([][]byte, 1, 2)
	blobs[0] = []byte("OLYMPUS CORPORATION\x00")
	// the header, IFD0 of two entries and the Exif IFD of one, then the
	// make
	start := uint32(len(header)) + 2 + 12*2 + 4 + 2 + 12 + 4 + uint32(len(blobs[0]))
	blobs = append(blobs, notes(start))
	ifd0 := []entry{
		blobRef(tagMake, typeASCII, uint32(len(blobs[0])), 0),
		{tag: tagExifIFD, typ: typeLong, count: 1, blob: -1, ifd: 1},
	}
	exif := []entry{blobRef(tagMakerNote, typeUndefined, uint32(len(blobs[1])), 1)}
	return tiffFile(header, false, [][]entry{ifd0, exif}, blobs)
}

func TestOlympusMakerNotes(t *testing.T) {
	jpg := fixture.JPEG(80, 60, color.White)
	tests := map[string][]byte{
		"new form": orf(func(uint32) []byte { return olympusNotes(jpg) }),
		"old form": orf(func(start uint32) []byte { return oldOlympusNotes(jpg, start) }),
	}
	for name, buf := range tests {
		t.Run(name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			var sizes []image.Point
			for p, err := range d.Previews(t.Context()) {
				if err != nil {
					t.Fatal(err)
				}
				img, err := p.Decode(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				sizes = append(sizes, img.Bounds().Size())
			}
			if len(sizes) != 1 || sizes[0] != image.Pt(80, 60) {
				t.Errorf("expected the preview of the maker notes, got: %v", sizes)
			}
		})
	}
	// maker notes of another maker are left out
	other := orf(func(uint32) []byte { return []byte("Nikon\x00\x02\x10\x00\x00II*\x00") })
	d, err := Open(t.Context(), bytes.NewReader(other))
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range d.Previews(t.Context()) {
		t.Errorf("expected no preview, got one with the error %v", err)
	}
}
