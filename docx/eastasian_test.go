package docx

import (
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// TestEastAsianLanguage checks that a run of East Asian text gives the
// layout the East Asian language of w:lang, from the run or from the
// defaults of the document, and that a run of Latin text does not.
func TestEastAsianLanguage(t *testing.T) {
	styles := fixture.WordPart{ID: "rIdS", Type: "styles", Name: "word/styles.xml", Data: fixture.WordXML("w:styles",
		`<w:docDefaults><w:rPrDefault><w:rPr><w:lang w:val="en-GB" w:eastAsia="zh-CN"/></w:rPr></w:rPrDefault></w:docDefaults>`)}
	html, _ := convertHTML(t, fixture.DOCX(`<w:p>`+
		`<w:r><w:t>中文</w:t></w:r>`+
		`<w:r><w:rPr><w:lang w:eastAsia="ja-JP"/></w:rPr><w:t>日本語</w:t></w:r>`+
		`<w:r><w:rPr><w:b/></w:rPr><w:t>Latin</w:t></w:r>`+
		`</w:p>`, styles))
	for _, want := range []string{`<span lang="zh-CN">中文</span>`, `<span lang="ja-JP">日本語</span>`} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the document to hold %q:\n%s", want, html)
		}
	}
	if strings.Count(html, ` lang="`) != 2 {
		t.Errorf("expected the language on the two runs of East Asian text only:\n%s", html)
	}
}

// TestThemeEastAsianFont checks that a run with the East Asian font of the
// theme takes the font of the script of the East Asian language of the
// theme fonts when the theme has no a:ea font: the language of
// w:themeFontLang, or else the language of the default run properties.
func TestThemeEastAsianFont(t *testing.T) {
	theme := fixture.WordPart{ID: "rIdT", Type: "theme", Name: "word/theme/theme1.xml", Data: []byte(`<?xml version="1.0"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office"><a:themeElements>
<a:fontScheme name="Office"><a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:font script="Jpan" typeface="Major Jpan"/><a:font script="Hans" typeface="Major Hans"/></a:majorFont>
<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:font script="Jpan" typeface="Minor Jpan"/><a:font script="Hans" typeface="Minor Hans"/></a:minorFont></a:fontScheme>
</a:themeElements></a:theme>`)}
	styles := func(lang string) fixture.WordPart {
		return fixture.WordPart{ID: "rIdS", Type: "styles", Name: "word/styles.xml", Data: fixture.WordXML("w:styles",
			`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:asciiTheme="minorHAnsi" w:eastAsiaTheme="minorEastAsia" w:hAnsiTheme="minorHAnsi"/>`+
				`<w:lang w:val="en-US" w:eastAsia="`+lang+`"/></w:rPr></w:rPrDefault></w:docDefaults>`)}
	}
	settings := fixture.WordPart{ID: "rIdG", Type: "settings", Name: "word/settings.xml", Data: fixture.WordXML("w:settings",
		`<w:themeFontLang w:val="en-US" w:eastAsia="ja-JP"/>`)}
	body := `<w:p><w:r><w:t>中文</w:t></w:r><w:r><w:rPr><w:rFonts w:eastAsiaTheme="majorEastAsia"/></w:rPr><w:t>見出し</w:t></w:r></w:p>`
	for _, tt := range []struct {
		name  string
		parts []fixture.WordPart
		want  []string
	}{
		{"defaults", []fixture.WordPart{theme, styles("zh-CN")}, []string{`&quot;Minor Hans&quot;`, `&quot;Major Hans&quot;`}},
		{"themeFontLang", []fixture.WordPart{theme, styles("zh-CN"), settings}, []string{`&quot;Minor Jpan&quot;`, `&quot;Major Jpan&quot;`}},
	} {
		html, _ := convertHTML(t, fixture.DOCX(body, tt.parts...))
		for _, want := range tt.want {
			if !strings.Contains(html, want) {
				t.Errorf("%s: expected the document to hold %q:\n%s", tt.name, want, html)
			}
		}
	}
}

// TestAutospace checks the space between East Asian text and Latin text or
// digits: on by default, and off with w:autoSpaceDE and w:autoSpaceDN.
func TestAutospace(t *testing.T) {
	for _, tt := range []struct {
		ppr, want string
	}{
		{``, ""},
		{`<w:autoSpaceDE w:val="0"/>`, "-ivy-text-autospace: ideograph-numeric"},
		{`<w:autoSpaceDN w:val="false"/>`, "-ivy-text-autospace: ideograph-alpha"},
		{`<w:autoSpaceDE w:val="0"/><w:autoSpaceDN w:val="0"/>`, "-ivy-text-autospace: none"},
		{`<w:autoSpaceDE/><w:autoSpaceDN w:val="1"/>`, ""},
	} {
		html, _ := convertHTML(t, fixture.DOCX(`<w:p><w:pPr>`+tt.ppr+`</w:pPr><w:r><w:t>中文A1</w:t></w:r></w:p>`))
		got := strings.Contains(html, "-ivy-text-autospace")
		if tt.want == "" && got || tt.want != "" && !strings.Contains(html, tt.want) {
			t.Errorf("%s: expected %q in the document:\n%s", tt.ppr, tt.want, html)
		}
	}
}
