package layout

import (
	"context"
	"image"
	"image/color"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/xo/okage/svg"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fontdb"
)

// officeFonts returns the folder of the module github.com/xo/ivy/fonts. The
// main module cannot import it, so the test reads its files. It skips the
// test when the folder is missing, as in the cache of the modules.
func officeFonts(t *testing.T) fs.FS {
	t.Helper()
	dir := filepath.Join("..", "..", "fonts")
	if _, err := os.Stat(filepath.Join(dir, "carlito", "Carlito-Regular.ttf")); err != nil {
		t.Skip("the fonts of github.com/xo/ivy/fonts are missing:", err)
	}
	return os.DirFS(dir)
}

// testLogger returns a logger that discards each record.
func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// renderFamily renders the first page of a book of one line of text in the
// family of font-family.
func renderFamily(t *testing.T, family string, opts BookOptions) image.Image {
	t.Helper()
	fsys := fstest.MapFS{"a.html": &fstest.MapFile{Data: []byte(`<p>Office documents name Calibri</p>`)}}
	style := `@page { size: 400px 40px; margin: 5px } body { margin: 0; font: 20px ` + family + ` }`
	b := NewBook(Source{FS: fsys, Documents: []string{"a.html"}, StyleSheets: []string{style}}, opts)
	p, err := b.Page(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(context.Background(), p, 96, color.White)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// samePixels reports whether two images have the same pixels.
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

// TestBookFontFilesSubstitute checks that a book with the fonts of
// github.com/xo/ivy/fonts draws the family Calibri with Carlito, and Arial
// with Liberation Sans, and that a book without them draws Calibri with the
// Go fonts.
func TestBookFontFilesSubstitute(t *testing.T) {
	fonts := officeFonts(t)
	with := BookOptions{FontFiles: fonts}
	calibri := renderFamily(t, "Calibri", with)
	if !samePixels(calibri, renderFamily(t, "Carlito", with)) {
		t.Error("Calibri is not drawn with Carlito")
	}
	if samePixels(calibri, renderFamily(t, "Calibri", BookOptions{})) {
		t.Error("Calibri is drawn with the Go fonts")
	}
	if !samePixels(renderFamily(t, "Arial", with), renderFamily(t, "'Liberation Sans'", with)) {
		t.Error("Arial is not drawn with Liberation Sans")
	}
	// A family without a substitute takes the default font, Go.
	if !samePixels(renderFamily(t, "Unknown", with), renderFamily(t, "Unknown", BookOptions{})) {
		t.Error("a family without a substitute is not drawn with the Go fonts")
	}
}

// TestBookFontFilesOrder checks the order in which the families of the
// faces are looked up: the faces of the document, then the font files, then
// the substitute, then the Go fonts.
func TestBookFontFilesOrder(t *testing.T) {
	fonts := officeFonts(t)
	lim := ivy.Limits{}.WithDefaults()
	db, first := fontdb.Load(fonts, false, lim, testLogger())
	if first != 20 {
		t.Fatalf("%d faces of the font files, expected 20", first)
	}
	fc := NewFontConfiguration(db, FontConfigurationOptions{FirstFaces: first})
	family := func(name string) string {
		t.Helper()
		id, ok := fc.query(svg.FontFamily{Kind: svg.FontFamilyNamed, Name: name}, svg.FontQuery{Weight: 400})
		if !ok {
			return ""
		}
		face, _ := fc.db.Face(id)
		return face.Families[0].Name
	}
	for name, want := range map[string]string{
		"Calibri":          "Carlito",
		"calibri":          "Carlito",
		"Cambria":          "Caladea",
		"Times New Roman":  "Liberation Serif",
		"Courier New":      "Liberation Mono",
		"liberation sans":  "Liberation Sans",
		"Go":               "Go",
		"Comic Sans MS":    "",
		"Times New Roman ": "Liberation Serif",
	} {
		if got := family(name); got != want {
			t.Errorf("%q: family %q, expected %q", name, got, want)
		}
	}
	// A face of the document hides the substitute.
	ids := fc.db.LoadFontData(goregular.TTF)
	info, _ := fc.db.Face(ids[0])
	info.Families = append([]svg.FontFamilyName{{Name: "Calibri"}}, info.Families...)
	if got := family("Calibri"); got != "Calibri" {
		t.Errorf("Calibri of the document: family %q, expected Calibri", got)
	}
	// Without FirstFaces, no face gives a substitute.
	fc = NewFontConfiguration(db, FontConfigurationOptions{})
	if got := family("Calibri"); got != "" {
		t.Errorf("Calibri without FirstFaces: family %q, expected none", got)
	}
}

// TestBookZeroFontFiles checks that a book without font files uses the
// default fonts.
func TestBookZeroFontFiles(t *testing.T) {
	b := NewBook(testBook(1, nil), BookOptions{})
	if b.fc.first != 0 || b.fc.base != fontdb.Default().Len() {
		t.Errorf("first %d, base %d, expected 0 and %d", b.fc.first, b.fc.base, fontdb.Default().Len())
	}
	p, err := b.Page(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !inked(t, p) {
		t.Error("expected ink on the page")
	}
}
