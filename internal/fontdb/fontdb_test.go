package fontdb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
)

// testLogger returns a logger that discards each record.
func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// woff returns the font data in a WOFF file whose tables are not
// compressed.
func woff(t *testing.T, sfnt []byte) []byte {
	t.Helper()
	n := int(binary.BigEndian.Uint16(sfnt[4:]))
	var head, body bytes.Buffer
	off := 44 + 20*n
	for i := range n {
		rec := sfnt[12+16*i:]
		tOff, tLen := binary.BigEndian.Uint32(rec[8:]), binary.BigEndian.Uint32(rec[12:])
		head.Write(rec[:4])
		_ = binary.Write(&head, binary.BigEndian, []uint32{uint32(off + body.Len()), tLen, tLen, binary.BigEndian.Uint32(rec[4:])})
		body.Write(sfnt[tOff : tOff+tLen])
		for body.Len()%4 != 0 {
			body.WriteByte(0)
		}
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.BigEndian, []uint32{0x774F4646, binary.BigEndian.Uint32(sfnt), uint32(off + body.Len())})
	_ = binary.Write(&out, binary.BigEndian, []uint16{uint16(n), 0})
	_ = binary.Write(&out, binary.BigEndian, []uint32{uint32(len(sfnt)), 0x00010000, 0, 0, 0, 0, 0})
	out.Write(head.Bytes())
	out.Write(body.Bytes())
	return out.Bytes()
}

// TestLoadLimits checks that the font files are read within the limits.
func TestLoadLimits(t *testing.T) {
	file := func(data []byte) *fstest.MapFile { return &fstest.MapFile{Data: data} }
	fsys := fstest.MapFS{
		"a.ttf":           file(goregular.TTF),
		"b.TTF":           file(gomono.TTF),
		"c.woff":          file(woff(t, goregular.TTF)),
		"notes.txt":       file([]byte("not a font")),
		"bad.otf":         file([]byte("not a font")),
		"sub/d.ttf":       file(goregular.TTF),
		"sub/deep/e.ttf":  file(goregular.TTF),
		"sub/deep/f.ttf":  file(goregular.TTF),
		"sub/deep/g.woff": file([]byte("wOFF not a font")),
	}
	for _, tt := range []struct {
		name string
		lim  ivy.Limits
		want int
	}{
		{"defaults", ivy.Limits{}, 6},
		// The files a.ttf, b.TTF and bad.otf, which has no face.
		{"objects", ivy.Limits{MaxObjects: 3}, 2},
		{"depth", ivy.Limits{MaxDepth: 2}, 4},
		{"memory", ivy.Limits{MaxMemory: int64(len(goregular.TTF)) - 1}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, first := Load(fsys, false, tt.lim.WithDefaults(), testLogger())
			if first != tt.want {
				t.Errorf("%d faces of the font files, expected %d", first, tt.want)
			}
			if db.Len() != first+Default().Len() {
				t.Errorf("%d faces, expected %d", db.Len(), first+Default().Len())
			}
		})
	}
	// A file that grows past MaxMemory after Load listed it is not read.
	f := &files{fsys: fsys, lim: ivy.Limits{MaxMemory: 10}.WithDefaults()}
	if _, err := f.Open("a.ttf"); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("open: %v, expected ivy.ErrLimit", err)
	}
}

// TestLoadShared checks that the callers share the fonts of the same
// comparable fs.FS, and that an fs.FS that is not comparable works.
func TestLoadShared(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.ttf"), goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	lim := ivy.Limits{}.WithDefaults()
	a, first := Load(os.DirFS(dir), false, lim, testLogger())
	b, _ := Load(os.DirFS(dir), false, lim, testLogger())
	if first != 1 || a != b {
		t.Errorf("%d faces, shared %t, expected 1 face, shared", first, a == b)
	}
	if c, _ := Load(os.DirFS(dir), true, lim, testLogger()); c == a {
		t.Error("the fonts of the system share the database without them")
	}
	if c := Of(ivy.Config{Fonts: os.DirFS(dir)}); c != a {
		t.Error("Of does not share the database of Load")
	}
	mapFS := fstest.MapFS{"a.ttf": &fstest.MapFile{Data: goregular.TTF}}
	// A struct is comparable, and the map in it panics the comparison.
	type wrapped struct{ fs.FS }
	for _, fsys := range []fs.FS{mapFS, wrapped{mapFS}} {
		a, first := Load(fsys, false, lim, testLogger())
		b, _ := Load(fsys, false, lim, testLogger())
		if first != 1 || a == b {
			t.Errorf("%T: %d faces, shared %t, expected 1 face, not shared", fsys, first, a == b)
		}
	}
}

// TestDefault checks the databases without font files: the Go fonts as
// every generic family.
func TestDefault(t *testing.T) {
	if db, first := Load(nil, false, ivy.Limits{}.WithDefaults(), testLogger()); db != Default() || first != 0 {
		t.Errorf("expected the default fonts, got %d faces of files", first)
	}
	if Of(ivy.Config{}) != Default() {
		t.Error("Of without fonts does not return the default fonts")
	}
	if Default().Len() != 8 {
		t.Errorf("%d Go fonts, expected 8", Default().Len())
	}
}
