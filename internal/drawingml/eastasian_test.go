package drawingml

import (
	"strings"
	"testing"
)

// TestEastAsianRuns checks that a run gives the layout its East Asian font
// after its Latin font, and the language of its East Asian text.
func TestEastAsianRuns(t *testing.T) {
	c := &Colors{Theme: testTheme}
	body := parse(t, `<a:txBody><a:bodyPr/><a:p>`+
		`<a:r><a:rPr lang="en-US" altLang="ja-JP"><a:latin typeface="Calibri"/><a:ea typeface="MS Mincho"/></a:rPr><a:t>日本語</a:t></a:r>`+
		`<a:r><a:rPr lang="ko-KR"><a:ea typeface="+mn-ea"/></a:rPr><a:t>Latin</a:t></a:r>`+
		`<a:r><a:rPr lang="en-US"><a:latin typeface="Arial"/><a:ea typeface="Times New Roman"/></a:rPr><a:t>漢字</a:t></a:r>`+
		`</a:p></a:txBody>`)
	var list ListStyle
	tb := c.ParseTextBody(body, &list, nil, ParseBodyStyle(parse(t, `<a:bodyPr/>`)), Fields{})
	box, ok := tb.Box(testTheme, 300, 200)
	if !ok {
		t.Fatal("expected a box")
	}
	for _, want := range []string{
		// The altLang is the East Asian language of an English run.
		`<span lang="ja-JP" style="`,
		`font-family:&#34;Calibri&#34;, &#34;MS Mincho&#34;, sans-serif;`,
		// A Latin font as the East Asian font changes nothing.
		`<span style="font-size:18pt;font-family:&#34;Arial&#34;, sans-serif;`,
	} {
		if !strings.Contains(box.HTML, want) {
			t.Errorf("expected the document to hold %q:\n%s", want, box.HTML)
		}
	}
	// A run of Latin text keeps the language of the paragraph.
	if strings.Contains(box.HTML, `lang="ko-KR"`) {
		t.Errorf("expected no language on a Latin run:\n%s", box.HTML)
	}
}
