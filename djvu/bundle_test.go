package djvu

import (
	"bytes"
	"sync/atomic"
	"testing"
)

// countingSource counts the bytes that a document reads from its source.
type countingSource struct {
	*bytes.Reader
	n atomic.Int64
}

// ReadAt satisfies the io.ReaderAt interface.
func (s *countingSource) ReadAt(p []byte, off int64) (int, error) {
	n, err := s.Reader.ReadAt(p, off)
	s.n.Add(int64(n))
	return n, err //nolint:wrapcheck // the error of bytes.Reader
}

func TestBundleOnDemand(t *testing.T) {
	t.Parallel()
	data := testFile(t, "bundled.djvu")
	src := &countingSource{Reader: bytes.NewReader(data)}
	d, err := Open(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	// Open reads the directory and the start of each page
	if n := src.n.Load(); n > int64(len(data))/4 {
		t.Errorf("expected Open to read a part of the %d bytes, got: %d", len(data), n)
	}
	count, _ := d.PageCount(t.Context())
	if count != 4 {
		t.Fatalf("expected 4 pages, got: %d", count)
	}
	// a page reads its component, and not the whole file
	before := src.n.Load()
	img := render(t, d, 1, 30)
	if img.Bounds().Empty() {
		t.Fatal("expected a page")
	}
	if n := src.n.Load() - before; n <= 0 || n >= int64(len(data)) {
		t.Errorf("expected the page to read a part of the %d bytes, got: %d", len(data), n)
	}
}
