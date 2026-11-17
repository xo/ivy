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
