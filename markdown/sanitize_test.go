package markdown

import (
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		name, text string
		want       []string
		not        []string
	}{
		{
			"style attribute", `<p style="color: red" align="center">a</p>`,
			[]string{`<p align="center">a</p>`}, []string{"style", "color: red"},
		},
		{
			"script block", "<script>alert(1)</script>\n\ntext\n",
			[]string{"text"}, []string{"script", "alert"},
		},
		{
			"style block", "<style>p { color: red }</style>\n\ntext\n",
			[]string{"text"}, []string{"<style", "color: red"},
		},
		{
			"inline script", "a <script>alert(1)</script> b\n",
			[]string{"a ", " b"}, []string{"script", "alert"},
		},
		{
			"unknown tag keeps content", "a <font color=red>red</font> <center>c</center>\n",
			[]string{"a red c"}, []string{"<font", "<center", "color"},
		},
		{
			"event attribute", `<img src="b.png" onerror="x()" alt="b">`,
			[]string{`<img src="b.png" alt="b">`}, []string{"onerror"},
		},
		{
			"script url", `<a href="javascript:alert(1)">x</a> <a href="jav&#x61;script:x">y</a> <a href="https://a.b/c">z</a>`,
			[]string{`<a>x</a>`, `<a>y</a>`, `<a href="https://a.b/c">z</a>`}, []string{"javascript"},
		},
		{
			"comment", "a <!-- hidden --> b\n",
			[]string{"a  b"}, []string{"hidden", "<!--"},
		},
		{
			"generated html stays", "- [x] done\n",
			[]string{`<input`, `type="checkbox"`}, nil,
		},
		{
			"details", "<details open><summary>s</summary>body</details>\n",
			[]string{"<details open>", "<summary>s</summary>"}, nil,
		},
		{
			"code stays", "`<script>` and\n\n```html\n<script>x</script>\n```\n",
			[]string{"&lt;script&gt;x&lt;/script&gt;", "<code>&lt;script&gt;</code>"}, nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := sourceHTML(t, tt.text)
			if strings.Contains(doc, "\x00") {
				t.Errorf("expected no mark, got %q", doc)
			}
			for _, w := range tt.want {
				if !strings.Contains(doc, w) {
					t.Errorf("expected the html to hold %q, got:\n%s", w, doc)
				}
			}
			for _, n := range tt.not {
				if strings.Contains(doc, n) {
					t.Errorf("expected the html not to hold %q, got:\n%s", n, doc)
				}
			}
		})
	}
}
