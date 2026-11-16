package metafile

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/gowemf"
)

// officeFonts returns the folder of the module github.com/xo/ivy/fonts. The
// main module cannot import that module, so the test reads its files.
func officeFonts(t *testing.T) ivy.Config {
	t.Helper()
	dir := filepath.Join("..", "..", "fonts")
	if _, err := os.Stat(filepath.Join(dir, "carlito")); err != nil {
		t.Skip("the fonts of github.com/xo/ivy/fonts are missing:", err)
	}
	return ivy.Config{Fonts: os.DirFS(dir)}
}

func TestFontsSubstitute(t *testing.T) {
	cfg := officeFonts(t)
	for _, c := range []struct {
		name, want string
	}{
		{"Calibri", "Carlito"},
		{"Arial", "Liberation Sans"},
		{"Times New Roman", "Liberation Serif"},
		{"Unknown Face", "Go"},
	} {
		f := newFontSet(nil, cfg).face(gowemf.FontRequest{FaceName: c.name})
		if f == nil || f.family != c.want {
			t.Errorf("%s: expected %s, got %+v", c.name, c.want, f)
		}
	}
	// without the fonts of the program, the Go fonts draw the text
	if f := newFontSet(nil, ivy.Config{}).face(gowemf.FontRequest{FaceName: "Calibri"}); f == nil || f.family != "Go" {
		t.Errorf("expected Go without Config.Fonts, got %+v", f)
	}
	// the pictures share the fonts of the same files
	if first, second := fontdb.Of(cfg), fontdb.Of(cfg); first != second {
		t.Error("expected one database for the same fonts")
	}
}

func TestFontsLimits(t *testing.T) {
	fsys := fstest.MapFS{
		"a/bad.ttf":  {Data: []byte("not a font")},
		"a/note.txt": {Data: []byte("text")},
	}
	// a file that is not a font is left out, and the Go fonts stay
	db := fontdb.Of(ivy.Config{Fonts: fsys})
	if f := (&fontSet{db: db}).face(gowemf.FontRequest{FaceName: "Arial"}); f == nil || f.family != "Go" {
		t.Errorf("expected Go, got %+v", f)
	}
}
