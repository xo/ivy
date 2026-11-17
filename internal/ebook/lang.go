package ebook

import (
	"strings"
	"unicode"
)

// maxLangScan is the number of bytes at the start of a text that
// TextLanguage reads.
const maxLangScan = 64 << 10

// gutenbergLanguages maps the name of a language in the header of a
// Project Gutenberg file to its tag of BCP 47.
var gutenbergLanguages = map[string]string{
	"arabic": "ar", "bulgarian": "bg", "catalan": "ca", "chinese": "zh",
	"czech": "cs", "danish": "da", "dutch": "nl", "english": "en",
	"esperanto": "eo", "finnish": "fi", "french": "fr", "german": "de",
	"greek": "el", "hebrew": "he", "hungarian": "hu", "icelandic": "is",
	"italian": "it", "japanese": "ja", "korean": "ko", "latin": "la",
	"norwegian": "no", "polish": "pl", "portuguese": "pt", "romanian": "ro",
	"russian": "ru", "serbian": "sr", "spanish": "es", "swedish": "sv",
	"tagalog": "tl", "turkish": "tr", "ukrainian": "uk", "welsh": "cy",
}

// TextLanguage returns the language of a plain text as a tag of BCP 47, or
// the empty string. It reads the start of the text only.
//
// If the text starts with the header of a Project Gutenberg file, the line
// "Language:" of the header gives the language. Otherwise the scripts of
// the letters give an East Asian language: kana gives Japanese, Hangul
// gives Korean, and Han characters alone give Chinese. TextLanguage
// does not find a language that is not East Asian from the letters.
//
// Chinese is zh-Hant when the text holds more characters that only
// Traditional Chinese uses than characters that only Simplified Chinese
// uses, and zh-Hans otherwise.
func TextLanguage(text string) string {
	if len(text) > maxLangScan {
		text = text[:maxLangScan]
	}
	lang := gutenbergLanguage(text)
	if lang != "" && lang != "zh" {
		return lang
	}
	var han, kana, hangul, trad, simp int
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
			kana++
		case unicode.Is(unicode.Hangul, r):
			hangul++
		case unicode.Is(unicode.Han, r):
			han++
			switch {
			case strings.ContainsRune(traditionalOnly, r):
				trad++
			case strings.ContainsRune(simplifiedOnly, r):
				simp++
			}
		}
	}
	switch {
	case lang == "" && han+kana+hangul == 0:
		return ""
	case lang == "" && hangul > han+kana:
		return "ko"
	case lang == "" && kana > 0 && kana*5 >= han:
		// Japanese text holds more kana than a fifth of its kanji
		return "ja"
	case trad > simp:
		return "zh-Hant"
	}
	return "zh-Hans"
}

// gutenbergLanguage returns the language that the header of a Project
// Gutenberg file names, or the empty string.
func gutenbergLanguage(text string) string {
	if !strings.Contains(text[:min(len(text), 1024)], "Project Gutenberg") {
		return ""
	}
	if end := strings.Index(text, "*** START OF"); end >= 0 {
		text = text[:end]
	}
	for line := range strings.Lines(text) {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "Language:")
		if !ok {
			continue
		}
		// a file in more than one language names them as "English and
		// French", or with commas, and the first one is the main one
		name, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(value)), " ")
		name = strings.TrimRight(name, ",;")
		return gutenbergLanguages[name]
	}
	return ""
}

// traditionalOnly and simplifiedOnly are common Chinese characters that only
// Traditional Chinese or only Simplified Chinese writes. The two strings
// pair up, rune by rune.
const (
	traditionalOnly = "這個們來時說國會學對開為與過還後樣麼見長門問間聽頭書東車馬魚鳥風飛從無現發經動種實點應進機當給讓氣話認識寫錢買賣愛歡樂親師處聲讀語謝請員業體關電題錯難邊歲萬夢樓紅寶賈歷隱靈張劉陳筆總結號黃飯漢熱臉腳錄雖島戰爭義衛灣擊華兒媽嗎爺歐齊鐘鄉傳興舊獨爾罷隨頂順須顧顯帶擔據斷際陰陽"
	simplifiedOnly  = "这个们来时说国会学对开为与过还后样么见长门问间听头书东车马鱼鸟风飞从无现发经动种实点应进机当给让气话认识写钱买卖爱欢乐亲师处声读语谢请员业体关电题错难边岁万梦楼红宝贾历隐灵张刘陈笔总结号黄饭汉热脸脚录虽岛战争义卫湾击华儿妈吗爷欧齐钟乡传兴旧独尔罢随顶顺须顾显带担据断际阴阳"
)
