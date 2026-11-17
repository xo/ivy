package fontdb

import (
	"slices"
	"testing"

	"github.com/xo/okage/svg"
)

// The language IDs of the name records of the tests.
const (
	lcidEnglish  = 0x0409
	lcidJapanese = 0x0411
	lcidChina    = 0x0804
	lcidTaiwan   = 0x0404
	lcidKorean   = 0x0412
)

// TestParseLang checks the East Asian language of language tags.
func TestParseLang(t *testing.T) {
	for tag, exp := range map[string]Lang{
		"ja": Japanese, "ja-JP": Japanese, "JA_jp": Japanese,
		"zh": SimplifiedChinese, "zh-CN": SimplifiedChinese, "zh-Hans": SimplifiedChinese, "zh-SG": SimplifiedChinese,
		"zh-TW": TraditionalChinese, "zh-Hant-HK": TraditionalChinese, "zh-HK": TraditionalChinese, "yue": TraditionalChinese,
		"ko": Korean, "ko-KR": Korean,
		"": NoLang, "en-US": NoLang, "x-none": NoLang, "jav": NoLang,
	} {
		if got := ParseLang(tag); got != exp {
			t.Errorf("%q: expected %v, got %v", tag, exp, got)
		}
	}
}

// TestLangOfLCID checks the East Asian language of Windows language IDs.
func TestLangOfLCID(t *testing.T) {
	for id, exp := range map[uint16]Lang{
		lcidJapanese: Japanese, lcidChina: SimplifiedChinese, 0x1004: SimplifiedChinese,
		lcidTaiwan: TraditionalChinese, 0x0c04: TraditionalChinese, 0x1404: TraditionalChinese, 0x7c04: TraditionalChinese,
		lcidKorean: Korean, lcidEnglish: NoLang, 0: NoLang,
	} {
		if got := LangOfLCID(id); got != exp {
			t.Errorf("%#04x: expected %v, got %v", id, exp, got)
		}
	}
}

// TestIsCJK checks the characters of East Asian text.
func TestIsCJK(t *testing.T) {
	for _, r := range "漢あアｱ가ㄅ、（Ａ㐀𠀀" {
		if !IsCJK(r) {
			t.Errorf("%q: expected an East Asian character", r)
		}
	}
	for _, r := range "aé→€\U00010330" {
		if IsCJK(r) {
			t.Errorf("%q: expected no East Asian character", r)
		}
	}
	for r, exp := range map[rune]Lang{'あ': Japanese, 'ア': Japanese, 'ｱ': Japanese, '가': Korean, 'ㄅ': TraditionalChinese, '漢': NoLang, 'a': NoLang} {
		if got := LangOfRune(r); got != exp {
			t.Errorf("%q: expected %v, got %v", r, exp, got)
		}
	}
}

// TestLookupEastAsian checks that the fonts of Office for East Asian text
// match by their English name, their localized name, and their PostScript
// name, in any case and any width, with a word of a style at the end.
func TestLookupEastAsian(t *testing.T) {
	for name, exp := range map[string]string{
		"MS Mincho":       "MS Mincho",
		"ms mincho":       "MS Mincho",
		"ＭＳ 明朝":           "MS Mincho",
		"MS 明朝":           "MS Mincho",
		"MS-Mincho":       "MS Mincho",
		"MSMincho,Bold":   "MS Mincho",
		"ＭＳ Ｐゴシック":        "MS PGothic",
		"MS UI Gothic":    "MS UI Gothic",
		"游ゴシック Light":     "Yu Gothic",
		"Yu Gothic UI":    "Yu Gothic",
		"DengXian Light":  "DengXian",
		"等线":              "DengXian",
		"宋体":              "SimSun",
		"新細明體":            "PMingLiU",
		"細明體":             "MingLiU",
		"맑은 고딕":           "Malgun Gothic",
		"바탕":              "Batang",
		"FangSong_GB2312": "FangSong",
		// The part before a hyphen names the family, as in a PDF file.
		"MingLiU-ExtB": "MingLiU",
	} {
		f, ok := LookupEastAsian(name)
		if !ok || f.Family != exp {
			t.Errorf("%q: expected %q, got %q %v", name, exp, f.Family, ok)
		}
	}
	for _, name := range []string{"Arial", "Century Gothic", "Gothic", "", "MS"} {
		if f, ok := LookupEastAsian(name); ok {
			t.Errorf("%q: expected no font, got %q", name, f.Family)
		}
	}
}

// TestClassifyName checks the kind and the language of the names of fonts
// that a document gives.
func TestClassifyName(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
		lang Lang
		ok   bool
	}{
		{"MS Mincho", Serif, Japanese, true},
		{"Meiryo", Sans, Japanese, true},
		{"SimHei", Sans, SimplifiedChinese, true},
		{"PMingLiU", Serif, TraditionalChinese, true},
		{"Dotum", Sans, Korean, true},
		{"HGS明朝E", Serif, NoLang, true},
		{"HG丸ｺﾞｼｯｸM-PRO", Sans, Japanese, true},
		{"華康楷書體", Serif, NoLang, true},
		{"나눔고딕", Sans, Korean, true},
		{"Arial", NoKind, NoLang, false},
		{"Century Gothic", NoKind, NoLang, false},
	}
	for _, test := range tests {
		kind, lang, ok := ClassifyName(test.name)
		if kind != test.kind || lang != test.lang || ok != test.ok {
			t.Errorf("%q: expected %v %v %v, got %v %v %v", test.name, test.kind, test.lang, test.ok, kind, lang, ok)
		}
	}
}

// fakeFace returns a face with the families, without data.
func fakeFace(families ...svg.FontFamilyName) svg.FaceInfo {
	return svg.FaceInfo{Families: families, Weight: 400, Stretch: svg.DefaultFontStretch()}
}

// TestClassify checks the class of the faces of the fonts of a system.
func TestClassify(t *testing.T) {
	tests := []struct {
		face svg.FaceInfo
		kind Kind
		lang Lang
		ok   bool
	}{
		{fakeFace(svg.FontFamilyName{Name: "MS Gothic", Language: lcidEnglish}, svg.FontFamilyName{Name: "ＭＳ ゴシック", Language: lcidJapanese}), Sans, Japanese, true},
		{fakeFace(svg.FontFamilyName{Name: "Microsoft JhengHei", Language: lcidEnglish}), Sans, TraditionalChinese, true},
		// A font of the table by its localized name only.
		{fakeFace(svg.FontFamilyName{Name: "SomeSun", Language: lcidEnglish}, svg.FontFamilyName{Name: "宋体", Language: lcidChina}), Serif, SimplifiedChinese, true},
		// The language of the name record gives the language.
		{fakeFace(svg.FontFamilyName{Name: "Free Song", Language: lcidEnglish}, svg.FontFamilyName{Name: "自由宋", Language: lcidTaiwan}), Serif, TraditionalChinese, true},
		{fakeFace(svg.FontFamilyName{Name: "IPAexMincho", Language: lcidEnglish}, svg.FontFamilyName{Name: "IPAex明朝", Language: lcidJapanese}), Serif, Japanese, true},
		{fakeFace(svg.FontFamilyName{Name: "Noto Sans CJK JP", Language: lcidEnglish}), Sans, Japanese, true},
		{fakeFace(svg.FontFamilyName{Name: "Noto Serif CJK KR", Language: lcidEnglish}), Serif, Korean, true},
		{fakeFace(svg.FontFamilyName{Name: "Source Han Sans SC", Language: lcidEnglish}, svg.FontFamilyName{Name: "思源黑体", Language: lcidChina}), Sans, SimplifiedChinese, true},
		{fakeFace(svg.FontFamilyName{Name: "MingLiU-ExtB", Language: lcidEnglish}, svg.FontFamilyName{Name: "細明體-ExtB", Language: lcidTaiwan}), Serif, TraditionalChinese, true},
		// The fonts of the module fonts and the Latin fonts are not East
		// Asian fonts.
		{fakeFace(svg.FontFamilyName{Name: "Droid Sans Fallback", Language: lcidEnglish}), NoKind, NoLang, false},
		{fakeFace(svg.FontFamilyName{Name: "Century Gothic", Language: lcidEnglish}), NoKind, NoLang, false},
		{fakeFace(svg.FontFamilyName{Name: "Arial", Language: lcidEnglish}, svg.FontFamilyName{Name: "Arial", Language: lcidJapanese}), NoKind, NoLang, false},
		{fakeFace(), NoKind, NoLang, false},
	}
	for _, test := range tests {
		c, ok := classify(&test.face)
		if ok != test.ok || ok && (c.kind != test.kind || c.lang != test.lang) {
			t.Errorf("%v: expected %v %v %v, got %v %v %v", test.face.Families, test.kind, test.lang, test.ok, c.kind, c.lang, ok)
		}
	}
}

// fakeDB returns a database of faces with the families, without data.
func fakeDB(faces ...[]svg.FontFamilyName) *svg.FontDatabase {
	db := svg.NewFontDatabase()
	for _, families := range faces {
		db.PushFaceInfo(fakeFace(families...))
	}
	return db
}

// TestEastAsianFamilies checks the order of the East Asian families for a
// kind and a language.
func TestEastAsianFamilies(t *testing.T) {
	db := fakeDB(
		[]svg.FontFamilyName{{Name: "Arial", Language: lcidEnglish}},
		[]svg.FontFamilyName{{Name: "Malgun Gothic", Language: lcidEnglish}},
		[]svg.FontFamilyName{{Name: "SimSun", Language: lcidEnglish}},
		[]svg.FontFamilyName{{Name: "MS Gothic", Language: lcidEnglish}},
		[]svg.FontFamilyName{{Name: "Microsoft YaHei", Language: lcidEnglish}},
		// A second face of a family counts once.
		[]svg.FontFamilyName{{Name: "Microsoft YaHei", Language: lcidEnglish}},
		[]svg.FontFamilyName{{Name: "Yu Gothic", Language: lcidEnglish}},
		[]svg.FontFamilyName{{Name: "Noto Serif CJK JP", Language: lcidEnglish}},
		[]svg.FontFamilyName{{Name: "Droid Sans Fallback", Language: lcidEnglish}},
	)
	e := NewEastAsian(db, 0, db.Len())
	if e.Len() != 6 {
		t.Fatalf("expected 6 families, got %d", e.Len())
	}
	tests := []struct {
		kind Kind
		lang Lang
		exp  []string
	}{
		// The language first, then the kind, then the order of the table.
		{Serif, Japanese, []string{"Noto Serif CJK JP", "Yu Gothic", "MS Gothic", "SimSun", "Microsoft YaHei", "Malgun Gothic"}},
		// A Chinese font comes before a font of the kind for the other
		// Chinese.
		{Serif, TraditionalChinese, []string{"SimSun", "Microsoft YaHei", "Noto Serif CJK JP", "Yu Gothic", "MS Gothic", "Malgun Gothic"}},
		{Sans, Japanese, []string{"Yu Gothic", "MS Gothic", "Noto Serif CJK JP", "Microsoft YaHei", "Malgun Gothic", "SimSun"}},
		{Sans, SimplifiedChinese, []string{"Microsoft YaHei", "SimSun", "Yu Gothic", "MS Gothic", "Malgun Gothic", "Noto Serif CJK JP"}},
		{Serif, Korean, []string{"Malgun Gothic", "SimSun", "Noto Serif CJK JP", "Yu Gothic", "MS Gothic", "Microsoft YaHei"}},
		// Without a language, Japanese comes first, then Simplified
		// Chinese, then Traditional Chinese, then Korean.
		{Sans, NoLang, []string{"Yu Gothic", "MS Gothic", "Microsoft YaHei", "Malgun Gothic", "Noto Serif CJK JP", "SimSun"}},
		{NoKind, NoLang, []string{"Yu Gothic", "MS Gothic", "Noto Serif CJK JP", "SimSun", "Microsoft YaHei", "Malgun Gothic"}},
	}
	for _, test := range tests {
		if got := e.Families(test.kind, test.lang); !slices.Equal(got, test.exp) {
			t.Errorf("%v %v: expected %q, got %q", test.kind, test.lang, test.exp, got)
		}
	}
	// The faces of the fonts module and the Go fonts hold no East Asian
	// family.
	if n := NewEastAsian(Default(), 0, Default().Len()).Len(); n != 0 {
		t.Errorf("the Go fonts: expected no East Asian family, got %d", n)
	}
}

// withFakeCJK returns the database of the fonts module with copies of the
// face of Droid Sans Fallback that have the families of East Asian fonts:
// a Japanese serif, a Simplified Chinese sans and a Korean sans. They draw
// the characters of the three languages.
func withFakeCJK(t *testing.T) *svg.FontDatabase {
	t.Helper()
	base, _ := repoFonts(t)
	db := base.Clone()
	var droid *svg.FaceInfo
	for face := range db.Faces() {
		if face.Families[0].Name == "Droid Sans Fallback" {
			droid = face
		}
	}
	if droid == nil {
		t.Fatal("no Droid Sans Fallback")
	}
	for _, families := range [][]svg.FontFamilyName{
		{{Name: "MS Mincho", Language: lcidEnglish}, {Name: "ＭＳ 明朝", Language: lcidJapanese}},
		{{Name: "Microsoft YaHei", Language: lcidEnglish}, {Name: "微软雅黑", Language: lcidChina}},
		{{Name: "Malgun Gothic", Language: lcidEnglish}, {Name: "맑은 고딕", Language: lcidKorean}},
	} {
		info := *droid
		info.Families = families
		info.PostScriptName = families[0].Name
		db.PushFaceInfo(info)
	}
	return db
}

// TestFindEastAsian checks that Find matches an East Asian font by its
// localized name, and that it takes an East Asian face of the same kind
// and language for a font that the database does not hold.
func TestFindEastAsian(t *testing.T) {
	db := withFakeCJK(t)
	for name, exp := range map[string]string{
		// The names of the font.
		"ＭＳ 明朝":     "MS Mincho",
		"MS 明朝":     "MS Mincho",
		"MS-Mincho": "MS Mincho",
		"微软雅黑":      "Microsoft YaHei",
		"맑은 고딕":     "Malgun Gothic",
		// The substitutes: the language first, then the kind.
		"MS PMincho":    "MS Mincho",
		"ＭＳ Ｐゴシック":      "MS Mincho",
		"SimSun":        "Microsoft YaHei",
		"新細明體":          "Microsoft YaHei",
		"Batang":        "Malgun Gothic",
		"HG丸ｺﾞｼｯｸM-PRO": "MS Mincho",
	} {
		id, ok := Find(db, name, 400, false)
		if !ok {
			t.Errorf("%q: no face", name)
			continue
		}
		if got := postScriptName(t, db, id); got != exp {
			t.Errorf("%q: expected %s, got %s", name, exp, got)
		}
	}
	// A Latin font has no East Asian substitute.
	if id, ok := Find(db, "Comic Sans MS", 400, false); ok {
		t.Errorf("Comic Sans MS: expected no face, got %s", postScriptName(t, db, id))
	}
	// Without East Asian faces, as without the fonts of the system, Find
	// gives no substitute.
	base, _ := repoFonts(t)
	if id, ok := Find(base, "MS Mincho", 400, false); ok {
		t.Errorf("MS Mincho without East Asian faces: expected no face, got %s", postScriptName(t, base, id))
	}
}
