package djvu

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/doc"
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

// indirect converts the bundled document data into an indirect document:
// its index, and a folder with a file for each component, as djvmcvt of
// DjVuLibre writes them.
func indirect(t testing.TB, data []byte) ([]byte, fstest.MapFS) {
	t.Helper()
	// the DIRM chunk follows AT&T, the header of FORM:DJVM and its type
	if string(data[16:20]) != "DIRM" {
		t.Fatal("expected a DIRM chunk")
	}
	dirm := data[24 : 24+binary.BigEndian.Uint32(data[20:])]
	dir, err := doc.ParseDir(dirm)
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{}
	for _, c := range dir.Components {
		size := binary.BigEndian.Uint32(data[c.Offset+4:])
		files[c.Name] = &fstest.MapFile{Data: append([]byte("AT&T"), data[c.Offset:c.Offset+8+int(size)]...)}
	}
	// an indirect DIRM has the bit of a bundle clear, and no offsets
	n := len(dir.Components)
	ind := append([]byte{dirm[0] &^ 0x80}, dirm[1:3]...)
	ind = append(ind, dirm[3+4*n:]...)
	return form("DJVM", chunk("DIRM", ind)), files
}

func TestIndirect(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"bundled.djvu", "shared-dict.djvu", "thumbnails.djvu"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data := testFile(t, name)
			index, files := indirect(t, data)
			if _, err := Open(t.Context(), bytes.NewReader(index)); !errors.Is(err, ivy.ErrUnsupported) {
				t.Errorf("expected ErrUnsupported without a folder, got: %v", err)
			}
			want := open(t, name, ivy.Config{})
			d, err := Open(t.Context(), bytes.NewReader(index), WithDir(files))
			if err != nil {
				t.Fatal(err)
			}
			got, _ := d.PageCount(t.Context())
			count, _ := want.PageCount(t.Context())
			if got != count {
				t.Fatalf("expected %d pages, got: %d", count, got)
			}
			// the pages are those of the bundled document
			for i := range count {
				a, b := render(t, d, i, 30), render(t, want, i, 30)
				if !samePixels(a, b) {
					t.Errorf("page %d: expected the page of the bundled document", i)
				}
			}
			wantInfo, _ := want.Describe(t.Context())
			if info, err := d.Describe(t.Context()); err != nil || info.Title != wantInfo.Title {
				t.Errorf("expected the title %q, got: %+v, %v", wantInfo.Title, info, err)
			}
			var previews, wantPreviews int
			for _, err := range d.Previews(t.Context()) {
				if err != nil {
					t.Fatal(err)
				}
				previews++
			}
			for _, err := range want.Previews(t.Context()) {
				if err != nil {
					t.Fatal(err)
				}
				wantPreviews++
			}
			if previews != wantPreviews {
				t.Errorf("expected %d previews, got: %d", wantPreviews, previews)
			}
		})
	}
}

// samePixels reports whether a and b hold the same pixels.
func samePixels(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}

func TestIndirectErrors(t *testing.T) {
	t.Parallel()
	index, files := indirect(t, testFile(t, "bundled.djvu"))
	// a missing page draws nothing, and the other pages draw
	delete(files, "photo.djvu")
	d, err := Open(t.Context(), bytes.NewReader(index), WithDir(files))
	if err != nil {
		t.Fatal(err)
	}
	render(t, d, 0, 20)
	page, _ := d.Page(t.Context(), 1)
	if _, err := ivy.RenderImage(t.Context(), page, 20, color.White); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat for a missing page, got: %v", err)
	}
	// a page larger than the limit is not read
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 8 << 10}}
	d, err = Open(t.Context(), bytes.NewReader(index), WithDir(files), WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	page, _ = d.Page(t.Context(), 3)
	if _, err := ivy.RenderImage(t.Context(), page, 20, color.White); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit for a page of 27170 bytes, got: %v", err)
	}
	// a name out of the folder is not read
	files["x/a.djvu"] = files["bilevel.djvu"]
	b := &bundle{files: files, lim: ivy.Limits{}.WithDefaults()}
	for _, name := range []string{"../bilevel.djvu", "/bilevel.djvu", "x/../bilevel.djvu", ".", ""} {
		if _, err := b.raw(doc.Component{ID: name, Name: name}); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%q: expected ErrFormat, got: %v", name, err)
		}
	}
	if _, err := b.raw(doc.Component{Name: "x/a.djvu"}); err != nil {
		t.Errorf("expected a file of a subfolder, got: %v", err)
	}
	if _, err := b.raw(doc.Component{Name: "x"}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat for a folder, got: %v", err)
	}
}
