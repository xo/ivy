package mobi

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/zipdoc"
)

// The headers of the records of resources that wrap their content. A CRES
// record holds an image of high resolution after a header of 12 bytes. A
// FONT record holds a font that zlib compresses and that a key can XOR.
const (
	cresHeader = 12
	fontHeader = 24
	// fontXORBytes is the number of bytes at the start of a font that the
	// key of a FONT record XORs.
	fontXORBytes = 1040
)

// The flags of a FONT record.
const (
	fontZlib = 1
	fontXOR  = 2
)

// resources gives the names in the FS of a book to its resource records.
// Resource n is record first+n-1: a MOBI book counts its images from 1 in
// the attribute recindex, and a KF8 book counts its resources from 1 in its
// links kindle:embed.
type resources struct {
	d     *Document
	fsys  *zipdoc.FS
	first int
	names map[int]string
}

// newResources returns the resources of the book, from the record first.
func newResources(d *Document, fsys *zipdoc.FS, first int) *resources {
	return &resources{d: d, fsys: fsys, first: first, names: make(map[int]string)}
}

// name returns the name of resource n in the FS, and adds the resource to
// the FS the first time. It returns false if resource n is not an image or
// a font.
func (r *resources) name(n int) (string, bool) {
	if name, ok := r.names[n]; ok {
		return name, name != ""
	}
	rec := r.first + n - 1
	if r.first < 0 || n < 1 || rec >= r.d.db.count() {
		return "", false
	}
	ext, ok := resourceType(r.d.db.head(rec, 32))
	if !ok {
		r.names[n] = ""
		return "", false
	}
	name := fmt.Sprintf("resource%05d.%s", n, ext)
	r.fsys.AddFunc(name, r.d.db.size(rec), func() (buf []byte, err error) {
		defer ebook.Recover("reading mobi resource "+name, &err)
		return r.d.resource(rec)
	})
	r.names[n] = name
	return name, true
}

// resourceType returns the extension of a resource from its first bytes.
func resourceType(head []byte) (string, bool) {
	if bytes.HasPrefix(head, []byte("CRES")) && len(head) > cresHeader {
		return resourceType(head[cresHeader:])
	}
	switch {
	case bytes.HasPrefix(head, []byte{0xff, 0xd8, 0xff}):
		return "jpg", true
	case bytes.HasPrefix(head, []byte("\x89PNG")):
		return "png", true
	case bytes.HasPrefix(head, []byte("GIF8")):
		return "gif", true
	case bytes.HasPrefix(head, []byte("BM")):
		return "bmp", true
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return "webp", true
	case bytes.HasPrefix(head, []byte("FONT")):
		// the type of the font is only known after the decode, and the
		// layout reads the type from the content
		return "ttf", true
	}
	return "", false
}

// resource reads the content of the resource record rec: an image, or a
// font that it decodes.
func (d *Document) resource(rec int) ([]byte, error) {
	buf, err := d.db.record(rec)
	if err != nil {
		return nil, err
	}
	switch {
	case bytes.HasPrefix(buf, []byte("CRES")) && len(buf) > cresHeader:
		return buf[cresHeader:], nil
	case bytes.HasPrefix(buf, []byte("FONT")):
		return decodeFont(buf, d.lim)
	}
	return buf, nil
}

// decodeFont decodes a FONT record. Its header gives, in 4 bytes each after
// "FONT": the size of the font, the flags, the offset of the data, and the
// length and the offset of the key of the XOR. The XOR comes first, then
// zlib.
func decodeFont(buf []byte, lim ivy.Limits) ([]byte, error) {
	if len(buf) < fontHeader {
		return nil, fmt.Errorf("font record of %d bytes: %w", len(buf), ivy.ErrFormat)
	}
	size, flags, off := int64(be32(buf, 4)), be32(buf, 8), int64(be32(buf, 12))
	keyLen, keyOff := int64(be32(buf, 16)), int64(be32(buf, 20))
	if off > int64(len(buf)) {
		return nil, fmt.Errorf("font record with its data at %d of %d bytes: %w", off, len(buf), ivy.ErrFormat)
	}
	if size > lim.MaxMemory {
		return nil, fmt.Errorf("font of %d bytes: %w", size, ivy.ErrLimit)
	}
	data := bytes.Clone(buf[off:])
	if flags&fontXOR != 0 {
		if keyLen <= 0 || keyOff+keyLen > int64(len(buf)) {
			return nil, fmt.Errorf("font record with its key out of the record: %w", ivy.ErrFormat)
		}
		key := buf[keyOff : keyOff+keyLen]
		for i := range min(len(data), fontXORBytes) {
			data[i] ^= key[i%len(key)]
		}
	}
	if flags&fontZlib == 0 {
		return data, nil
	}
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding the font: %w: %w", ivy.ErrFormat, err)
	}
	out, err := io.ReadAll(io.LimitReader(zr, lim.MaxMemory+1))
	if cerr := zr.Close(); err == nil && int64(len(out)) <= lim.MaxMemory {
		err = cerr
	}
	switch {
	case err != nil:
		return nil, fmt.Errorf("decoding the font: %w: %w", ivy.ErrFormat, err)
	case int64(len(out)) > lim.MaxMemory:
		return nil, fmt.Errorf("font of more than %d bytes: %w", lim.MaxMemory, ivy.ErrLimit)
	}
	return out, nil
}
