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
