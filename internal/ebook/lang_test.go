package ebook

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTextLanguage(t *testing.T) {
	const gutenberg = "The Project Gutenberg eBook of A Book\n\nTitle: A Book\n\nLanguage: %s\n\n*** START OF THE PROJECT GUTENBERG EBOOK ***\n\n%s"
	for _, tt := range []struct {
		name, text, want string
	}{
		{"empty", "", ""},
		{"latin", "Just some English text.", ""},
		{"japanese", "吾輩は猫である。名前はまだ無い。", "ja"},
		{"korean", "대한민국의 수도는 서울이다. 漢字", "ko"},
		{"simplified", "这是一个关于中国的故事。", "zh-Hans"},
		{"traditional", "這是一個關於中國的故事。", "zh-Hant"},
		{"han only", "天地人山水", "zh-Hans"},
		{"gutenberg chinese", fmtGutenberg(gutenberg, "Chinese", "此開卷第一回也．作者自云：因曾歷過一番夢幻之后，故將真事隱去"), "zh-Hant"},
		{"gutenberg french", fmtGutenberg(gutenberg, "French", "Il était une fois."), "fr"},
		{"gutenberg two", fmtGutenberg(gutenberg, "English and French", "Text."), "en"},
		{"gutenberg unknown", fmtGutenberg(gutenberg, "Klingon", "Text."), ""},
		{"line in body", "Language: French\n", ""},
	} {
		if got := TextLanguage(tt.text); got != tt.want {
			t.Errorf("%s: expected %q, got: %q", tt.name, tt.want, got)
		}
	}
	// only the start of a long text counts
	long := strings.Repeat("a", maxLangScan) + "日本語のテキスト"
	if got := TextLanguage(long); got != "" {
		t.Errorf("expected no language from the end of a long text, got: %q", got)
	}
}

func fmtGutenberg(format, lang, body string) string {
	return strings.Replace(strings.Replace(format, "%s", lang, 1), "%s", body, 1)
}

func TestChineseVariants(t *testing.T) {
	if utf8.RuneCountInString(traditionalOnly) != utf8.RuneCountInString(simplifiedOnly) {
		t.Fatal("expected the two lists of characters to pair up")
	}
	seen := make(map[rune]bool)
	for _, r := range traditionalOnly + simplifiedOnly {
		if seen[r] {
			t.Errorf("expected each character once, got %q twice", r)
		}
		seen[r] = true
	}
}
