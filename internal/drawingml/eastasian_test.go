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

// TestScriptFonts checks that a theme without an a:ea font gives an East
// Asian run the font of the script of its language.
func TestScriptFonts(t *testing.T) {
	doc := parse(t, `<a:fontScheme><a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/>`+
		`<a:font script="Jpan" typeface="游明朝"/><a:font script="Hang" typeface="맑은 고딕"/>`+
		`<a:font script="Hans" typeface="等线"/><a:font script="Hant" typeface="新細明體"/><a:font script="Arab" typeface="Arial"/>`+
		`</a:minorFont></a:fontScheme>`)
	var minor Fonts
	for n := range doc.Children() {
		if n.IsElement() {
			minor = parseFonts(n)
		}
	}
	for lang, want := range map[string]string{"ja-JP": "游明朝", "zh-CN": "等线", "zh-TW": "新細明體", "ko-KR": "맑은 고딕", "en-US": "", "": ""} {
		if got := minor.EastAsianFor(lang); got != want {
			t.Errorf("%q: expected %q, got: %q", lang, want, got)
		}
	}
	// a:ea comes before the fonts of the scripts
	if got := (Fonts{EastAsian: "MS Mincho", Scripts: minor.Scripts}).EastAsianFor("zh-CN"); got != "MS Mincho" {
		t.Errorf("expected MS Mincho, got: %q", got)
	}
	theme := &Theme{Minor: minor}
	c := &Colors{Theme: theme}
	body := parse(t, `<a:txBody><a:bodyPr/><a:p>`+
		`<a:r><a:rPr lang="ja-JP"><a:latin typeface="+mn-lt"/><a:ea typeface="+mn-ea"/></a:rPr><a:t>日本語</a:t></a:r>`+
		`</a:p></a:txBody>`)
	var list ListStyle
	tb := c.ParseTextBody(body, &list, nil, ParseBodyStyle(parse(t, `<a:bodyPr/>`)), Fields{})
	box, ok := tb.Box(theme, 300, 200)
	if !ok {
		t.Fatal("expected a box")
	}
	if want := `font-family:&#34;Calibri&#34;, &#34;游明朝&#34;, sans-serif;`; !strings.Contains(box.HTML, want) {
		t.Errorf("expected the document to hold %q:\n%s", want, box.HTML)
	}
}

// TestKerning checks that a run kerns at the sizes of its attribute kern
// and above, as PowerPoint kerns the runs of custom-geometry.pptx, and
// that kern="0" does not kern.
func TestKerning(t *testing.T) {
	c := &Colors{Theme: testTheme}
	body := parse(t, `<a:txBody><a:bodyPr/><a:lstStyle><a:lvl1pPr><a:defRPr kern="1200"/></a:lvl1pPr></a:lstStyle>`+
		`<a:p><a:r><a:rPr sz="1800"/><a:t>AVA</a:t></a:r><a:r><a:rPr sz="1100"/><a:t>To</a:t></a:r>`+
		`<a:r><a:rPr sz="2000" kern="0"/><a:t>We</a:t></a:r></a:p></a:txBody>`)
	var list ListStyle
	tb := c.ParseTextBody(body, &list, nil, ParseBodyStyle(parse(t, `<a:bodyPr/>`)), Fields{})
	box, ok := tb.Box(testTheme, 300, 200)
	if !ok {
		t.Fatal("expected a box")
	}
	if !strings.Contains(box.HTML, "font-kerning: none") {
		t.Errorf("expected no kerning by default:\n%s", box.HTML)
	}
	if n := strings.Count(box.HTML, "font-kerning:normal"); n != 1 || !strings.Contains(box.HTML, "font-size:18pt;font-kerning:normal;") {
		t.Errorf("expected the run of 18 points only to kern, got %d runs:\n%s", n, box.HTML)
	}
}
