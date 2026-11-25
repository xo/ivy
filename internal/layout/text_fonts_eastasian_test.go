package layout

import (
	"testing"

	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fontdb"
)

// eastAsianConfig returns a configuration of the fonts module and, when
// fake is set, of copies of the face of Droid Sans Fallback with the
// families of East Asian fonts of the system: a Japanese serif, a Japanese
// sans, a Simplified Chinese sans and a Korean sans. The copies draw the
// characters of every language, so the tests do not depend on the fonts of
// the machine.
func eastAsianConfig(t *testing.T, fake bool) *FontConfiguration {
	t.Helper()
	db, first := fontdb.Load(officeFonts(t), false, ivy.Limits{}.WithDefaults(), testLogger())
	db = db.Clone()
	var droid *svg.FaceInfo
	for face := range db.Faces() {
		if face.Families[0].Name == "Droid Sans Fallback" {
			droid = face
		}
	}
	if droid == nil {
		t.Fatal("no Droid Sans Fallback")
	}
	if fake {
		for _, families := range [][]svg.FontFamilyName{
			{{Name: "MS Mincho", Language: 0x0409}, {Name: "ＭＳ 明朝", Language: 0x0411}},
			{{Name: "Yu Gothic", Language: 0x0409}, {Name: "游ゴシック", Language: 0x0411}},
			{{Name: "Microsoft YaHei", Language: 0x0409}, {Name: "微软雅黑", Language: 0x0804}},
			{{Name: "Malgun Gothic", Language: 0x0409}, {Name: "맑은 고딕", Language: 0x0412}},
		} {
			info := *droid
			info.Families = families
			db.PushFaceInfo(info)
		}
	}
	return NewFontConfiguration(db, FontConfigurationOptions{FirstFaces: first})
}

// eastAsianStyleOf returns a style of the configuration with the families
// and the language.
func eastAsianStyleOf(fc *FontConfiguration, lang string, families ...string) *Style {
	style := newInitialStyle(&StyleContext{FontConfig: fc, Cache: map[any]any{}})
	style.Set("font_family", tupleOf(families...))
	if lang != "" {
		style.Set("lang", String(lang))
	}
	return style
}

// familyOf returns the first family of the face id.
func familyOf(t *testing.T, fc *FontConfiguration, id svg.FontID) string {
	t.Helper()
	face, ok := fc.db.Face(id)
	if !ok {
		t.Fatalf("no face %d", id)
	}
	return face.Families[0].Name
}

// TestEastAsianSubstitute checks that an East Asian font that the program
// does not hold takes an East Asian face of the same kind, the language of
// the text first, and that a localized name finds its font.
func TestEastAsianSubstitute(t *testing.T) {
	fc := eastAsianConfig(t, true)
	tests := []struct {
		family, lang, exp string
	}{
		{"ＭＳ 明朝", "", "MS Mincho"},
		// The width of the letters of a name of a font of Office does not
		// matter.
		{"MS 明朝", "", "MS Mincho"},
		{"ｍｓ　ｍｉｎｃｈｏ", "", "MS Mincho"},
		{"MS PMincho", "", "MS Mincho"},
		{"ＭＳ Ｐゴシック", "", "Yu Gothic"},
		{"Meiryo", "", "Yu Gothic"},
		{"SimHei", "", "Microsoft YaHei"},
		{"Batang", "", "Malgun Gothic"},
		// The language of the text comes before the language of the font.
		{"MS PMincho", "ko-KR", "Malgun Gothic"},
		{"SimSun", "ja", "MS Mincho"},
	}
	for _, test := range tests {
		chain := fc.chain(eastAsianStyleOf(fc, test.lang, test.family))
		if got := familyOf(t, fc, chain[0]); got != test.exp {
			t.Errorf("%q in %q: expected %s, got %s", test.family, test.lang, test.exp, got)
		}
	}
	// A Latin font that the program does not hold takes no East Asian face.
	chain := fc.chain(eastAsianStyleOf(fc, "ja", "Comic Sans MS"))
	if got := familyOf(t, fc, chain[0]); got != "Liberation Serif" {
		t.Errorf("Comic Sans MS: expected Liberation Serif, got %s", got)
	}
	// Without East Asian faces, as without the fonts of the system, the
	// chain does not change.
	fc = eastAsianConfig(t, false)
	chain = fc.chain(eastAsianStyleOf(fc, "", "MS Mincho"))
	if got := familyOf(t, fc, chain[0]); got != "Liberation Serif" {
		t.Errorf("MS Mincho without East Asian faces: expected Liberation Serif, got %s", got)
	}
}

// TestFontAlias checks that a font that the program does not hold takes the
// font of the system that fontdb.Alias gives, such as URW Gothic for
// Century Gothic.
func TestFontAlias(t *testing.T) {
	db, first := fontdb.Load(officeFonts(t), false, ivy.Limits{}.WithDefaults(), testLogger())
	db = db.Clone()
	var sans *svg.FaceInfo
	for face := range db.Faces() {
		if face.PostScriptName == "LiberationSans" {
			sans = face
		}
	}
	if sans == nil {
		t.Fatal("no Liberation Sans")
	}
	info := *sans
	info.Families = []svg.FontFamilyName{{Name: "URW Gothic", Language: 0x0409}}
	db.PushFaceInfo(info)
	fc := NewFontConfiguration(db, FontConfigurationOptions{FirstFaces: first})
	chain := fc.chain(eastAsianStyleOf(fc, "", "Century Gothic"))
	if got := familyOf(t, fc, chain[0]); got != "URW Gothic" {
		t.Errorf("Century Gothic: expected URW Gothic, got %s", got)
	}
}

// TestEastAsianFallback checks the fallback face of an East Asian
// character: the East Asian faces of the program, for the language and the
// kind of the text, before Droid Sans Fallback.
func TestEastAsianFallback(t *testing.T) {
	fc := eastAsianConfig(t, true)
	tests := []struct {
		r        rune
		lang     string
		families []string
		exp      string
	}{
		{'漢', "zh-CN", []string{"Liberation Sans"}, "Microsoft YaHei"},
		{'漢', "ko", []string{"serif"}, "Malgun Gothic"},
		{'漢', "ja", []string{"serif"}, "MS Mincho"},
		{'漢', "ja", []string{"sans-serif"}, "Yu Gothic"},
		{'漢', "ja-JP", []string{"Liberation Sans"}, "MS Mincho"},
		// Without a language, the first East Asian font of font-family
		// gives it.
		{'漢', "", []string{"Calibri", "宋体", "sans-serif"}, "Microsoft YaHei"},
		// Without one, Japanese comes first, for each character. The
		// copies of Droid Sans Fallback draw the hangul too.
		{'あ', "", []string{"sans-serif"}, "Yu Gothic"},
		{'가', "", []string{"sans-serif"}, "Yu Gothic"},
		{'漢', "", []string{"serif"}, "MS Mincho"},
	}
	for _, test := range tests {
		m := fc.newFontmap(eastAsianStyleOf(fc, test.lang, test.families...))
		f := m.ResolveFace(test.r)
		if got := familyOf(t, fc, m.byFace[f]); got != test.exp {
			t.Errorf("%q in %q %q: expected %s, got %s", test.r, test.lang, test.families, test.exp, got)
		}
	}
	// Without East Asian faces, Droid Sans Fallback draws the character.
	fc = eastAsianConfig(t, false)
	m := fc.newFontmap(eastAsianStyleOf(fc, "ja", "serif"))
	if got := familyOf(t, fc, m.byFace[m.ResolveFace('漢')]); got != "Droid Sans Fallback" {
		t.Errorf("without East Asian faces: expected Droid Sans Fallback, got %s", got)
	}
}
