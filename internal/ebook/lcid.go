package ebook

// An LCID is a locale identifier of Windows. Its low 10 bits are the primary
// language, and the next 6 bits are the sublanguage, such as the country.
// MOBI and CHM files give their language as an LCID.

// lcidLanguages maps a primary language of an LCID to its code of ISO 639-1.
var lcidLanguages = map[uint32]string{
	0x01: "ar", 0x02: "bg", 0x03: "ca", 0x04: "zh", 0x05: "cs", 0x06: "da",
	0x07: "de", 0x08: "el", 0x09: "en", 0x0a: "es", 0x0b: "fi", 0x0c: "fr",
	0x0d: "he", 0x0e: "hu", 0x0f: "is", 0x10: "it", 0x11: "ja", 0x12: "ko",
	0x13: "nl", 0x14: "nb", 0x15: "pl", 0x16: "pt", 0x17: "rm", 0x18: "ro",
	0x19: "ru", 0x1a: "hr", 0x1b: "sk", 0x1c: "sq", 0x1d: "sv", 0x1e: "th",
	0x1f: "tr", 0x20: "ur", 0x21: "id", 0x22: "uk", 0x23: "be", 0x24: "sl",
	0x25: "et", 0x26: "lv", 0x27: "lt", 0x29: "fa", 0x2a: "vi", 0x2b: "hy",
	0x2c: "az", 0x2d: "eu", 0x2f: "mk", 0x36: "af", 0x37: "ka", 0x38: "fo",
	0x39: "hi", 0x3e: "ms", 0x3f: "kk", 0x41: "sw", 0x43: "uz", 0x44: "tt",
	0x45: "bn", 0x46: "pa", 0x47: "gu", 0x49: "ta", 0x4a: "te", 0x4b: "kn",
	0x4c: "ml", 0x4e: "mr", 0x56: "gl",
}

// lcidRegions maps the LCID of a language whose country changes its script
// or its spelling to a tag of BCP 47. Serbian and Bosnian share the primary
// language of Croatian.
var lcidRegions = map[uint32]string{
	0x0404: "zh-TW", 0x0804: "zh-CN", 0x0c04: "zh-HK", 0x1004: "zh-SG", 0x1404: "zh-MO",
	0x0409: "en-US", 0x0809: "en-GB",
	0x0416: "pt-BR", 0x0816: "pt-PT",
	0x081a: "sr-Latn", 0x0c1a: "sr-Cyrl", 0x141a: "bs", 0x201a: "bs-Cyrl",
}

// LCIDLanguage returns the language of the LCID lcid as a tag of BCP 47,
// such as "en-US" or "de". It returns "" for an LCID that it does not know.
func LCIDLanguage(lcid uint32) string {
	if tag, ok := lcidRegions[lcid&0xffff]; ok {
		return tag
	}
	return lcidLanguages[lcid&0x3ff]
}

// LCIDCharset returns the name of the code page of Windows that the LCID
// lcid uses for its text, as the WHATWG Encoding Standard names it, such as
// "windows-1251" or "shift_jis". It returns "windows-1252" for an LCID that
// it does not know, because that is the code page of most of them.
func LCIDCharset(lcid uint32) string {
	switch lcid & 0xffff {
	case 0x0404, 0x0c04, 0x1404:
		return "big5"
	case 0x0804, 0x1004:
		return "gbk"
	case 0x081a, 0x141a:
		// Serbian and Bosnian in the Latin script
		return "windows-1250"
	case 0x0c1a, 0x201a:
		// Serbian and Bosnian in the Cyrillic script
		return "windows-1251"
	}
	switch lcid & 0x3ff {
	case 0x11:
		return "shift_jis"
	case 0x12:
		return "euc-kr"
	case 0x04:
		return "gbk"
	case 0x05, 0x0e, 0x15, 0x18, 0x1a, 0x1b, 0x1c, 0x24:
		return "windows-1250"
	case 0x02, 0x19, 0x22, 0x23, 0x2f, 0x3f, 0x44:
		return "windows-1251"
	case 0x08:
		return "windows-1253"
	case 0x1f, 0x2c, 0x43:
		return "windows-1254"
	case 0x0d:
		return "windows-1255"
	case 0x01, 0x20, 0x29:
		return "windows-1256"
	case 0x25, 0x26, 0x27:
		return "windows-1257"
	case 0x2a:
		return "windows-1258"
	case 0x1e:
		return "windows-874"
	}
	return "windows-1252"
}
