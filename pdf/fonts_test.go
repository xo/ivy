package pdf

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/pdf/internal/cmap"
	"github.com/xo/ivy/pdf/internal/pdffont"
)

// fontsDir is the folder of the module github.com/xo/ivy/fonts in the
// repository.
const fontsDir = "../fonts"

// repoFonts returns the configuration with the fonts of the module
// github.com/xo/ivy/fonts. It skips the test when the folder is not there,
// such as in the module cache.
func repoFonts(t *testing.T) ivy.Config {
	t.Helper()
	if _, err := os.Stat(filepath.Join(fontsDir, "droid")); err != nil {
		t.Skipf("no fonts module: %v", err)
	}
	return ivy.Config{Fonts: os.DirFS(fontsDir)}
}

// fontFile returns the bytes of a file of the fonts module.
func fontFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fontsDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestFontResolver checks the fonts that the resolver gives for the fonts
// that a file does not embed.
func TestFontResolver(t *testing.T) {
	cfg := repoFonts(t)
	r := newFontResolver(cfg, cfg.Limits.WithDefaults())
	named := func(name string, bold, italic bool) pdffont.FontQuery {
		q := pdffont.DefaultFallbackFontQuery()
		q.PostScriptName, q.IsBold, q.IsItalic = name, bold, italic
		return pdffont.FontQuery{Named: true, Fallback: q}
	}
	cjk := func(kind cmap.CIDFamilyKind) pdffont.FontQuery {
		q := pdffont.DefaultFallbackFontQuery()
		q.PostScriptName = "KozMinPr6N-Regular"
		q.CharacterCollection = &cmap.CharacterCollection{Family: cmap.CIDFamily{Kind: kind}}
		return pdffont.FontQuery{Fallback: q}
	}
	helvetica, _ := pdffont.Helvetica.GetFontData()
	times, _ := pdffont.TimesRoman.GetFontData()
	for _, c := range []struct {
		name  string
		query pdffont.FontQuery
		file  string
		std   []byte
	}{
		{"ArialMT", named("ArialMT", false, false), "liberation/LiberationSans-Regular.ttf", nil},
		{"Arial,Bold", named("Arial,Bold", true, false), "liberation/LiberationSans-Bold.ttf", nil},
		{"TimesNewRomanPS-ItalicMT", named("TimesNewRomanPS-ItalicMT", false, true), "liberation/LiberationSerif-Italic.ttf", nil},
		{"Calibri", named("Calibri", false, false), "carlito/Carlito-Regular.ttf", nil},
		{"Japan1", cjk(cmap.AdobeJapan1), "droid/DroidSansFallback.ttf", nil},
		{"GB1", cjk(cmap.AdobeGB1), "droid/DroidSansFallback.ttf", nil},
		{"CNS1", cjk(cmap.AdobeCNS1), "droid/DroidSansFallback.ttf", nil},
		{"Korea1", cjk(cmap.AdobeKorea1), "droid/DroidSansFallback.ttf", nil},
		{"Identity", cjk(cmap.AdobeIdentity), "", helvetica},
		{"Times-Roman", pdffont.FontQuery{IsStandard: true, Standard: pdffont.TimesRoman}, "", times},
	} {
		data, _, ok := r.resolve(&c.query)
		switch {
		case !ok:
			t.Errorf("%s: expected a font", c.name)
		case c.file != "" && !bytes.Equal(data, fontFile(t, c.file)):
			t.Errorf("%s: expected %s", c.name, c.file)
		case c.std != nil && !bytes.Equal(data, c.std):
			t.Errorf("%s: expected the substitute of a standard font", c.name)
		}
	}
	for _, name := range []string{"SegoeUI", "MS-Mincho"} {
		q := named(name, false, false)
		if _, _, ok := r.resolve(&q); ok {
			t.Errorf("%s: expected no font", name)
		}
	}
	// Without fonts, the resolver gives the substitutes of the standard
	// fonts, and nothing for a Named query.
	r = newFontResolver(ivy.Config{}, ivy.Limits{}.WithDefaults())
	q := named("ArialMT", false, false)
	if _, _, ok := r.resolve(&q); ok {
		t.Error("ArialMT: expected no font without fonts")
	}
	q = cjk(cmap.AdobeJapan1)
	if data, _, ok := r.resolve(&q); !ok || !bytes.Equal(data, helvetica) {
		t.Error("Japan1: expected Helvetica without fonts")
	}
}

// cjkPage is a page with the text あ漢A of a Japanese font that the file
// does not embed.
var cjkPage = build(
	"<< /Type /Catalog /Pages 2 0 R >>",
	"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
	"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 40] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
	stream("", "BT /F1 20 Tf 10 12 Td <30426F220041> Tj ET"),
	"<< /Type /Font /Subtype /Type0 /BaseFont /KozMinPr6N-Regular /Encoding /UniJIS-UCS2-H /DescendantFonts [6 0 R] >>",
	"<< /Type /Font /Subtype /CIDFontType0 /BaseFont /KozMinPr6N-Regular /CIDSystemInfo << /Registry (Adobe) /Ordering (Japan1) /Supplement 6 >> /DW 1000 /W [1 95 500] >>",
	"<< >>",
)

// TestCJKFont checks that the CJK text of a font that the file does not
// embed draws with the fonts of the configuration.
func TestCJKFont(t *testing.T) {
	render := func(cfg ivy.Config) int {
		d := open(t, cjkPage, WithConfig(cfg))
		img, err := ivy.RenderImage(t.Context(), page(t, d, 0), 72, color.White)
		if err != nil {
			t.Fatal(err)
		}
		return ink(img)
	}
	// Without fonts, only the letter A draws.
	latin := render(ivy.Config{})
	if latin == 0 {
		t.Error("expected the letter A without fonts")
	}
	if all := render(repoFonts(t)); all < 3*latin {
		t.Errorf("expected the CJK characters to draw, got %d pixels, %d of A", all, latin)
	}
}
