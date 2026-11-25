package fontdb

import (
	"bytes"
	"encoding/binary"
	"os"
	"slices"
	"testing"

	"github.com/xo/okage/svg"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
)

// repoFonts returns the fonts of the module github.com/xo/ivy/fonts, from
// its folder in the repository. It skips the test when the folder is not
// there, such as in the module cache.
func repoFonts(t *testing.T) (*svg.FontDatabase, int) {
	t.Helper()
	const dir = "../../fonts"
	if _, err := os.Stat(dir + "/droid/DroidSansFallback.ttf"); err != nil {
		t.Skipf("no fonts module: %v", err)
	}
	return Load(os.DirFS(dir), false, ivy.Limits{}.WithDefaults(), testLogger())
}

// postScriptName returns the PostScript name of the face id of db.
func postScriptName(t *testing.T, db *svg.FontDatabase, id svg.FontID) string {
	t.Helper()
	face, ok := db.Face(id)
	if !ok {
		t.Fatalf("no face %d", id)
	}
	return face.PostScriptName
}

// TestFamilyCandidates checks the families that a font name can name.
func TestFamilyCandidates(t *testing.T) {
	tests := []struct {
		name string
		exp  []string
	}{
		{"Arial", []string{"Arial"}},
		{"ArialMT", []string{"ArialMT", "Arial"}},
		{"Arial,Bold", []string{"Arial,Bold", "Arial"}},
		{"Arial-BoldMT", []string{"Arial-BoldMT", "Arial"}},
		{"TimesNewRomanPSMT", []string{"TimesNewRomanPSMT", "TimesNewRoman", "Times New Roman"}},
		{"TimesNewRomanPS-BoldItalicMT", []string{"TimesNewRomanPS-BoldItalicMT", "TimesNewRomanPS", "TimesNewRoman", "Times New Roman"}},
		{"CourierNewPSMT", []string{"CourierNewPSMT", "CourierNew", "Courier New"}},
		{"MS-Mincho", []string{"MS-Mincho", "MS"}},
		{"Times New Roman", []string{"Times New Roman"}},
	}
	for _, test := range tests {
		if got := familyCandidates(test.name); !slices.Equal(got, test.exp) {
			t.Errorf("%s: expected %q, got: %q", test.name, test.exp, got)
		}
	}
}

// TestFind checks that Find finds a font by the name that a document gives,
// or its substitute, among the fonts of the module github.com/xo/ivy/fonts.
func TestFind(t *testing.T) {
	db, _ := repoFonts(t)
	tests := []struct {
		name   string
		weight uint16
		italic bool
		exp    string
	}{
		{"ArialMT", 400, false, "LiberationSans"},
		{"Arial,Bold", 700, false, "LiberationSans-Bold"},
		{"Helvetica-Oblique", 400, true, "LiberationSans-Italic"},
		{"TimesNewRomanPS-BoldItalicMT", 700, true, "LiberationSerif-BoldItalic"},
		{"CourierNewPSMT", 400, false, "LiberationMono"},
		{"Calibri", 400, false, "Carlito-Regular"},
		{"Cambria-Bold", 700, false, "Caladea-Bold"},
		{"Carlito-Italic", 400, true, "Carlito-Italic"},
		{"LiberationSerif", 400, false, "LiberationSerif"},
		{"Droid Sans Fallback", 400, false, "DroidSansFallback"},
	}
	for _, test := range tests {
		id, ok := Find(db, test.name, test.weight, test.italic)
		if !ok {
			t.Errorf("%s: expected %s, found nothing", test.name, test.exp)
			continue
		}
		if got := postScriptName(t, db, id); got != test.exp {
			t.Errorf("%s: expected %s, got: %s", test.name, test.exp, got)
		}
	}
	for _, name := range []string{"", "SegoeUI", "MS-Mincho", "KozMinPr6N-Regular"} {
		if id, ok := Find(db, name, 400, false); ok {
			t.Errorf("%q: expected nothing, got: %s", name, postScriptName(t, db, id))
		}
	}
}

// TestFindAlias checks that Find takes the alias of a font that the
// database does not hold, and the font itself when the database holds it.
func TestFindAlias(t *testing.T) {
	db := fakeDB(
		[]svg.FontFamilyName{{Name: "URW Gothic", Language: lcidEnglish}},
	)
	id, ok := Find(db, "Century Gothic", 400, false)
	if !ok || id != 0 {
		t.Errorf("expected URW Gothic for Century Gothic, got: %d %t", id, ok)
	}
	db.PushFaceInfo(fakeFace(svg.FontFamilyName{Name: "Century Gothic", Language: lcidEnglish}))
	id, ok = Find(db, "Century Gothic", 400, false)
	if !ok || id != 1 {
		t.Errorf("expected Century Gothic, got: %d %t", id, ok)
	}
	if alias, ok := Alias(" CENTURY GOTHIC "); !ok || alias != "URW Gothic" {
		t.Errorf("expected URW Gothic, got: %q %t", alias, ok)
	}
	if alias, ok := Alias("Arial"); ok {
		t.Errorf("expected no alias for Arial, got: %q", alias)
	}
}

// TestCovering checks that Covering finds Droid Sans Fallback of the module
// github.com/xo/ivy/fonts for a character of each CJK collection, and
// nothing in the Go fonts.
func TestCovering(t *testing.T) {
	db, first := repoFonts(t)
	for _, r := range []rune{'あ', '这', '這', '한'} {
		id, ok := Covering(db, first, r)
		if !ok {
			t.Errorf("%q: expected Droid Sans Fallback, found nothing", r)
			continue
		}
		if got := postScriptName(t, db, id); got != "DroidSansFallback" {
			t.Errorf("%q: expected DroidSansFallback, got: %s", r, got)
		}
	}
	if id, ok := Covering(db, first, 'A'); !ok || id >= svg.FontID(first) {
		t.Errorf("'A': expected a face of the font files, got: %d, %v", id, ok)
	}
	// The layout looks for the fallback of a character with the selector
	// of okage, after the faces of the style, such as Carlito for Calibri.
	carlito, ok := Find(db, "Calibri", 700, false)
	if !ok {
		t.Fatal("expected Carlito")
	}
	if id, ok := svg.DefaultFallbackSelector(nil)('漢', []svg.FontID{carlito}, db); !ok ||
		postScriptName(t, db, id) != "DroidSansFallback" {
		t.Errorf("expected the selector of the layout to find DroidSansFallback, got: %d, %v", id, ok)
	}
	if id, ok := Covering(Default(), 0, 'あ'); ok {
		t.Errorf("expected no Go font to draw 'あ', got: %d", id)
	}
}

// TestDraws checks that Draws reads the character map of a face, and that
// a table cmap that ends past the data draws nothing.
func TestDraws(t *testing.T) {
	db := svg.NewFontDatabase()
	ids := db.LoadFontData(bytes.Clone(goregular.TTF))
	if len(ids) != 1 {
		t.Fatalf("expected one face of Go Regular, got: %d", len(ids))
	}
	if !Draws(db, ids[0], 'a') {
		t.Error("expected Go Regular to draw 'a'")
	}
	if Draws(db, ids[0], 'あ') {
		t.Error("expected Go Regular not to draw 'あ'")
	}
	if Draws(db, ids[0]+1, 'a') {
		t.Error("expected no face after Go Regular")
	}
	data := bytes.Clone(goregular.TTF)
	n := int(binary.BigEndian.Uint16(data[4:]))
	for i := range n {
		rec := 12 + 16*i
		if string(data[rec:rec+4]) == "cmap" {
			binary.BigEndian.PutUint32(data[rec+12:], 0xFFFFFFF0)
		}
	}
	db = svg.NewFontDatabase()
	ids = db.LoadFontData(data)
	if len(ids) == 1 && Draws(db, ids[0], 'a') {
		t.Error("expected no character in a table cmap that ends past the data")
	}
}

// TestGenericFamilies checks that the fonts module gives the generic
// families, as LibreOffice and calibre use them, and that the Go fonts stay
// the generic families without it.
func TestGenericFamilies(t *testing.T) {
	db, _ := repoFonts(t)
	for kind, exp := range map[svg.FontFamilyKind]string{
		svg.FontFamilySerif:     "Liberation Serif",
		svg.FontFamilySansSerif: "Liberation Sans",
		svg.FontFamilyMonospace: "Liberation Mono",
		svg.FontFamilyCursive:   "Go",
	} {
		if got := db.FamilyName(svg.FontFamily{Kind: kind}); got != exp {
			t.Errorf("generic family %v: expected %q, got: %q", kind, exp, got)
		}
	}
	if got := Default().FamilyName(svg.FontFamily{Kind: svg.FontFamilySerif}); got != "Go" {
		t.Errorf("default serif: expected %q, got: %q", "Go", got)
	}
}
