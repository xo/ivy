package camera

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/xo/ivy"
)

// The boxes of a CR3 file that hold a JPEG: THMB holds a thumbnail of 160 by
// 120, and PRVW a preview of 1620 by 1080. CMT1 holds a TIFF file with the
// make, the model and the date.
const (
	boxThumbnail = "THMB"
	boxPreview   = "PRVW"
	boxMetadata  = "CMT1"
)

// jpegSOI starts a JPEG.
var jpegSOI = []byte{0xff, 0xd8, 0xff}

// maxJPEGSearch is the number of bytes at the start of a THMB or a PRVW box in
// which readCR3 looks for the start of the JPEG, past the fields of the box.
const maxJPEGSearch = 64

// readCR3 walks the boxes of a CR3 file.
func (d *Document) readCR3() ([]region, error) {
	w := &boxWalker{d: d}
	if err := w.walk(0, d.src.Size(), 0); err != nil {
		return nil, err
	}
	return w.regions, nil
}

// boxWalker walks the boxes of ISO BMFF.
type boxWalker struct {
	d       *Document
	count   int
	regions []region
}

// walk walks the boxes between off and end.
func (w *boxWalker) walk(off, end int64, depth int) error {
	if depth > w.d.lim.MaxDepth {
		return fmt.Errorf("boxes nested deeper than %d: %w", w.d.lim.MaxDepth, ivy.ErrLimit)
	}
	for off+8 <= end {
		w.count++
		if w.count > w.d.lim.MaxObjects {
			return fmt.Errorf("more than %d boxes: %w", w.d.lim.MaxObjects, ivy.ErrLimit)
		}
		head := make([]byte, 16)
		if n, _ := w.d.src.ReadAt(head, off); n < 8 {
			return nil
		}
		size, typ, hdr := int64(binary.BigEndian.Uint32(head)), string(head[4:8]), int64(8)
		switch size {
		case 0:
			size = end - off
		case 1:
			size, hdr = int64(binary.BigEndian.Uint64(head[8:])), 16
		}
		if size < hdr || size > end-off {
			// a malformed box ends the walk, and keeps what it found
			return nil
		}
		body, bodyEnd := off+hdr, off+size
		switch typ {
		case "moov":
			if err := w.walk(body, bodyEnd, depth+1); err != nil {
				return err
			}
		case "uuid":
			// the boxes of Canon follow the 16 bytes of the uuid, and the
			// box of the preview has 8 more bytes before its boxes
			if err := w.walk(body+16, bodyEnd, depth+1); err != nil {
				return err
			}
			if err := w.walk(body+24, bodyEnd, depth+1); err != nil {
				return err
			}
		case boxThumbnail, boxPreview:
			w.findJPEG(body, bodyEnd)
		case boxMetadata:
			if t, err := newTIFFReader(w.d.src, body, bodyEnd-body, w.d.lim); err == nil {
				if err := t.walk(); err != nil {
					return err
				}
				w.d.info = t.info
			}
		}
		off = bodyEnd
	}
	return nil
}

// findJPEG keeps the JPEG that starts in the first bytes of the body of a box
// and ends with the box.
func (w *boxWalker) findJPEG(body, end int64) {
	buf := make([]byte, min(maxJPEGSearch, end-body))
	if n, _ := w.d.src.ReadAt(buf, body); n < len(buf) {
		return
	}
	if i := bytes.Index(buf, jpegSOI); i >= 0 {
		w.regions = append(w.regions, region{off: body + int64(i), n: end - body - int64(i)})
	}
}
